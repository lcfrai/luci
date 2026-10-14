package dockerbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const runtimeImagePrefix = "luci.runtime-image-"
const runtimeImageBuildBudget = 30 * time.Minute
const runtimeImageStreamLimit = 8 << 20
const runtimeColdBaseAdmissionMiB = 1024

var _ backend.RuntimeImageExecutor = (*Backend)(nil)

type runtimeImageClient interface {
	imageResourceClient
	ImageBuild(context.Context, io.Reader, build.ImageBuildOptions) (build.ImageBuildResponse, error)
	ImagePull(context.Context, string, image.PullOptions) (io.ReadCloser, error)
	ImageTag(context.Context, string, string) error
}

type runtimeImageKillClient interface {
	ContainerKill(context.Context, string, string) error
}

// Classic ImageBuild has no durable build-status/readback API. Keep its
// transport open briefly after caller cancellation so an exact owned RUN
// container can be interrupted and the daemon's terminal response drained.
// A missing container or a cancelled transport alone never proves settlement.
func (b *Backend) cancelOwnedRuntimeBuild(ctx, monitorCtx context.Context, identity backend.RuntimeImageIdentity, done <-chan struct{}, cancelBuild context.CancelFunc) {
	select {
	case <-ctx.Done():
	case <-done:
		return
	}
	killer, ok := b.api.(runtimeImageKillClient)
	if !ok {
		cancelBuild()
		return
	}
	probe, cancel := context.WithTimeout(monitorCtx, 20*time.Second)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	killed := map[string]bool{}
	for {
		select {
		case <-done:
			return
		default:
		}
		if err := b.runtimeImageIdentity(probe, identity); err != nil {
			cancelBuild()
			return
		}
		rows, err := b.api.ContainerList(probe, container.ListOptions{All: true,
			Filters: filters.NewArgs(filters.Arg("label", runtimeImagePrefix+"allocation="+identity.AllocationID))})
		if err != nil || len(rows) > 128 {
			cancelBuild()
			return
		}
		for _, row := range rows {
			if len(row.ID) != 64 || !fenceHex(row.ID, 32) || row.Labels["luci.runtime-image"] != "true" {
				cancelBuild()
				return
			}
			for key, value := range runtimeIdentityLabels(identity) {
				if row.Labels[key] != value {
					cancelBuild()
					return
				}
			}
			if row.State == "running" && !killed[row.ID] {
				if err := killer.ContainerKill(probe, row.ID, "SIGKILL"); err != nil {
					cancelBuild()
					return
				}
				killed[row.ID] = true
			}
		}
		select {
		case <-done:
			return
		case <-probe.Done():
			cancelBuild() // outcome stays unknown unless a terminal response was read
			return
		case <-ticker.C:
		}
	}
}

func runtimeImageID(id string) bool {
	return strings.HasPrefix(id, "sha256:") && fenceHex(strings.TrimPrefix(id, "sha256:"), 32)
}

func runtimeImageTag(tag string) (string, error) {
	if tag == "" || len(tag) > 256 || strings.TrimSpace(tag) != tag {
		return "", backend.ErrUnsupported
	}
	ref, err := reference.ParseNormalizedNamed(tag)
	if err != nil {
		return "", backend.ErrUnsupported
	}
	if _, digested := ref.(reference.Canonical); digested {
		return "", backend.ErrUnsupported
	}
	return reference.TagNameOnly(ref).String(), nil
}

func validRuntimeIdentity(v backend.RuntimeImageIdentity, prepared bool) bool {
	if v.Protocol != backend.RuntimeImageExecutionProtocol || !fenceLogicalID(v.AllocationID) || !fenceLogicalID(v.JobID) || !fenceHex(v.ScopeDigest, 32) {
		return false
	}
	if v.SourceDigest != "" && !fenceHex(v.SourceDigest, 32) || prepared && v.SourceDigest == "" {
		return false
	}
	if !prepared && v.EngineID == "" && v.InstanceID == "" {
		return true
	}
	return v.EngineID != "" && len(v.EngineID) <= 256 && strings.TrimSpace(v.EngineID) == v.EngineID && !strings.ContainsAny(v.EngineID, "\x00\r\n") && backend.ValidateInstance(v.InstanceID) == nil
}

func runtimeIdentityLabels(v backend.RuntimeImageIdentity) map[string]string {
	return map[string]string{
		"luci.runtime-image": "true", backend.LabelInstance: v.InstanceID,
		runtimeImagePrefix + "protocol":   strconv.Itoa(v.Protocol),
		runtimeImagePrefix + "engine":     v.EngineID,
		runtimeImagePrefix + "allocation": v.AllocationID,
		runtimeImagePrefix + "job":        v.JobID,
		runtimeImagePrefix + "scope":      v.ScopeDigest,
		runtimeImagePrefix + "source":     v.SourceDigest,
	}
}

func (b *Backend) runtimeImageIdentity(ctx context.Context, expected backend.RuntimeImageIdentity) error {
	if !validRuntimeIdentity(expected, true) || expected.InstanceID != b.instance {
		return backend.ErrLaunchFenceOwnership
	}
	current, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return err
	}
	if current.EngineID != expected.EngineID || current.InstanceID != expected.InstanceID {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}

func runtimeBaseValid(row image.InspectResponse) bool {
	return runtimeImageID(row.ID) && row.Os == "linux" && row.Size > 0 && row.Config != nil && len(row.Config.OnBuild) == 0 && len(row.Config.Volumes) == 0
}

// PrepareRuntimeImage never pulls an image or invokes Docker build. Cold base
// absence is supported; their fixed allowance is admitted by the coordinator.
func (b *Backend) PrepareRuntimeImage(ctx context.Context, spec backend.RuntimeImageSpec) (backend.PreparedRuntimeImage, error) {
	if !validRuntimeIdentity(spec.Identity, false) || spec.Resources.MemoryMiB <= 0 || int64(spec.Resources.MemoryMiB) > math.MaxInt64>>20 || spec.Resources.CPUMillis <= 0 || int64(spec.Resources.CPUMillis) > math.MaxInt64/1000000 {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	tag, err := runtimeImageTag(spec.ImageTag)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	spec.ImageTag = tag
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	identity, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	if spec.Identity.EngineID != "" && spec.Identity.EngineID != identity.EngineID || spec.Identity.InstanceID != "" && spec.Identity.InstanceID != identity.InstanceID {
		return backend.PreparedRuntimeImage{}, backend.ErrLaunchFenceOwnership
	}
	spec.Identity.EngineID, spec.Identity.InstanceID = identity.EngineID, identity.InstanceID
	limits, ok := b.api.(resourceLimitClient)
	if !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	api, ok := b.api.(runtimeImageClient)
	if !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	info, err := limits.Info(ctx)
	if err != nil || ctx.Err() != nil {
		return backend.PreparedRuntimeImage{}, backend.ErrUnavailable
	}
	if info.ID != identity.EngineID || info.OSType != "linux" {
		return backend.PreparedRuntimeImage{}, backend.ErrLaunchFenceOwnership
	}
	if !requestedLimitsSupported(info, spec.Resources) {
		return backend.PreparedRuntimeImage{}, backend.ErrResourceLimitsUnsupported
	}
	if info.Driver != "vfs" && info.Driver != "overlay2" && info.Driver != "overlayfs" {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	files, digest, err := runtimeContext(spec)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	spec.Identity.SourceDigest = digest
	prepared, bases, err := runtimePreparedContext(files, spec.Identity, nil)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	coldBases := 0
	for _, base := range bases {
		row, err := api.ImageInspect(ctx, base)
		if errdefs.IsNotFound(err) {
			coldBases++
			continue
		}
		if err != nil {
			return backend.PreparedRuntimeImage{}, backend.ErrUnavailable
		}
		if !runtimeBaseValid(row) {
			return backend.PreparedRuntimeImage{}, fmt.Errorf("runtime base %q has unsupported image metadata: %w", base, backend.ErrUnsupported)
		}
	}
	if ctx.Err() != nil {
		return backend.PreparedRuntimeImage{}, ctx.Err()
	}
	// The observed cold Go and Node pulls consume daemon/host memory before a
	// capped RUN exists. Charge those separately instead of raising the RUN cap.
	admission := spec.Resources.MemoryMiB + coldBases*runtimeColdBaseAdmissionMiB
	if admission > 65536 {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	return backend.PreparedRuntimeImage{Spec: spec, PreparedContext: prepared, ContextDigest: digest,
		RequiredDiskBytes: backend.RuntimeImageRequiredDiskBytes, AdmissionMemoryMiB: admission,
		StorageDriver: info.Driver, BaseImages: bases}, nil
}

type runtimeStreamMessage struct {
	Stream      string `json:"stream"`
	Status      string `json:"status"`
	Error       string `json:"error"`
	ErrorDetail *struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
	Aux *struct {
		ID string `json:"ID"`
	} `json:"aux"`
}

func runtimeMessageError(message runtimeStreamMessage) string {
	detail := message.Error
	if message.ErrorDetail != nil && message.ErrorDetail.Message != "" {
		detail = message.ErrorDetail.Message
	}
	if detail == "" && message.ErrorDetail != nil {
		detail = "Docker reported an unsuccessful operation"
	}
	detail = strings.ToValidUTF8(detail, "?")
	if len(detail) > 4096 {
		detail = strings.ToValidUTF8(detail[:1024], "") + " [earlier output omitted] " + strings.ToValidUTF8(detail[len(detail)-2048:], "")
	}
	return detail
}

// A successful pull is an admitted mutation, with no auth callback or retry.
// Inspect alone cannot turn an interrupted pull into a complete response.
func consumeRuntimePull(ctx context.Context, body io.Reader, base string) error {
	reader := &io.LimitedReader{R: body, N: runtimeImageStreamLimit + 1}
	decoder := json.NewDecoder(reader)
	terminal, failure := false, ""
	for {
		var message runtimeStreamMessage
		err := decoder.Decode(&message)
		if err == io.EOF {
			break
		}
		if err != nil || ctx.Err() != nil {
			return backend.ErrUnavailable
		}
		if detail := runtimeMessageError(message); detail != "" {
			failure = detail
		}
		if message.Status == "Status: Downloaded newer image for "+base || message.Status == "Status: Image is up to date for "+base {
			terminal = true
		}
	}
	if ctx.Err() != nil || reader.N == 0 {
		return backend.ErrUnavailable
	}
	if failure != "" {
		return fmt.Errorf("runtime base pull failed: %s: %w", failure, backend.ErrUnavailable)
	}
	if !terminal {
		return backend.ErrUnavailable
	}
	return nil
}

func (b *Backend) runtimeBases(ctx, requestCtx context.Context, api runtimeImageClient, bases []string) (map[string]string, error) {
	ids := make(map[string]string, len(bases))
	for _, base := range bases {
		if err := requestCtx.Err(); err != nil {
			return nil, err
		}
		// Context preparation establishes exactly this fixed allowlist.
		if base != "golang:1.26-bookworm" && base != "node:24-bookworm-slim" {
			return nil, backend.ErrUnsupported
		}
		row, err := api.ImageInspect(ctx, base)
		if errdefs.IsNotFound(err) {
			body, pullErr := api.ImagePull(ctx, base, image.PullOptions{})
			if pullErr != nil || body == nil {
				return nil, backend.ErrUnavailable
			}
			pullErr = consumeRuntimePull(ctx, body, base)
			closeErr := body.Close()
			if pullErr != nil {
				return nil, pullErr
			}
			if closeErr != nil {
				return nil, backend.ErrUnavailable
			}
			row, err = api.ImageInspect(ctx, base)
		}
		if err != nil || ctx.Err() != nil {
			return nil, backend.ErrUnavailable
		}
		if !runtimeBaseValid(row) {
			return nil, backend.ErrUnsupported
		}
		ids[base] = row.ID
	}
	return ids, nil
}

type runtimeBuildStream struct {
	imageID, failure string
	success          bool
}

func consumeRuntimeBuild(ctx context.Context, body io.Reader) (runtimeBuildStream, error) {
	reader := &io.LimitedReader{R: body, N: runtimeImageStreamLimit + 1}
	decoder := json.NewDecoder(reader)
	var result runtimeBuildStream
	for {
		var message runtimeStreamMessage
		err := decoder.Decode(&message)
		if err == io.EOF {
			break
		}
		if err != nil || ctx.Err() != nil {
			return runtimeBuildStream{}, backend.ErrUnavailable
		}
		if detail := runtimeMessageError(message); detail != "" {
			result.failure = detail
		}
		if message.Aux != nil {
			if !runtimeImageID(message.Aux.ID) {
				return runtimeBuildStream{}, backend.ErrUnavailable
			}
			// Classic Docker emits aux.ID after every stage, not only the last.
			// Any later stage invalidates a preceding success-looking marker.
			result.imageID, result.success = message.Aux.ID, false
		}
		if strings.HasPrefix(message.Stream, "Successfully built ") {
			if result.imageID == "" || message.Stream != "Successfully built "+strings.TrimPrefix(result.imageID, "sha256:")[:12]+"\n" {
				return runtimeBuildStream{}, backend.ErrUnavailable
			}
			result.success = true
		}
	}
	if ctx.Err() != nil || reader.N == 0 || !result.success && result.failure == "" {
		return runtimeBuildStream{}, backend.ErrUnavailable
	}
	return result, nil
}

func (b *Backend) runtimeIntermediatesAbsent(ctx context.Context, identity backend.RuntimeImageIdentity) error {
	if err := b.runtimeImageIdentity(ctx, identity); err != nil {
		return err
	}
	// Do not hide a colliding/altered identity by filtering on all its other
	// labels. Any container with this allocation label prevents settlement.
	rows, err := b.api.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(filters.Arg("label", runtimeImagePrefix+"allocation="+identity.AllocationID))})
	if err != nil || len(rows) != 0 || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	return b.runtimeImageIdentity(ctx, identity)
}

func (b *Backend) verifyRuntimeImage(ctx context.Context, api runtimeImageClient, identity backend.RuntimeImageIdentity, id string) error {
	if !runtimeImageID(id) {
		return backend.ErrLaunchFenceOwnership
	}
	if err := b.runtimeImageIdentity(ctx, identity); err != nil {
		return err
	}
	row, err := api.ImageInspect(ctx, id)
	if err != nil || row.ID != id || !runtimeBaseValid(row) || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	for key, expected := range runtimeIdentityLabels(identity) {
		if row.Config.Labels[key] != expected {
			return backend.ErrLaunchFenceOwnership
		}
	}
	return b.runtimeImageIdentity(ctx, identity)
}

func (b *Backend) BuildRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
	// The caller has already persisted this exact allocation and its capacity
	// claim. Even a pre-dispatch refusal returns no settlement claim here.
	requestCtx := ctx
	if err := requestCtx.Err(); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	fresh, err := b.PrepareRuntimeImage(ctx, prepared.Spec)
	if err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	if !reflect.DeepEqual(fresh, prepared) {
		return backend.RuntimeImageBuildResult{}, backend.ErrLaunchFenceOwnership
	}
	if err := requestCtx.Err(); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	api, ok := b.api.(runtimeImageClient)
	if !ok {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnsupported
	}
	deadline := time.Now().Add(runtimeImageBuildBudget)
	if callerDeadline, okay := ctx.Deadline(); okay && callerDeadline.Before(deadline) {
		deadline = callerDeadline
	}
	// An acknowledged base pull is independent of the job's cancellation.
	// Drain its terminal transport before deciding whether the build started.
	ctx, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	defer cancel()
	files, digest, err := runtimeContext(prepared.Spec)
	if err != nil || digest != prepared.ContextDigest {
		return backend.RuntimeImageBuildResult{}, backend.ErrLaunchFenceOwnership
	}
	ids, err := b.runtimeBases(ctx, requestCtx, api, prepared.BaseImages)
	if err != nil {
		if requestCtx.Err() != nil && errors.Is(err, requestCtx.Err()) && ctx.Err() == nil {
			// Every earlier pull was fully consumed and closed. No build request
			// has been issued, so this allocation has a positive no-build result.
			return backend.RuntimeImageBuildResult{Settled: true}, err
		}
		return backend.RuntimeImageBuildResult{}, err
	}
	if err = b.runtimeImageIdentity(ctx, prepared.Spec.Identity); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	archive, _, err := runtimePreparedContext(files, prepared.Spec.Identity, ids)
	if err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	if err := requestCtx.Err(); err != nil {
		// The only daemon mutations before this point are fully consumed base
		// pulls. No image build has been issued for this allocation.
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	// The classic builder enforces these RUN-container limits. Pull/download and
	// daemon bookkeeping share the admitted allowance, not a separate cgroup.
	options := build.ImageBuildOptions{Version: build.BuilderV1, Dockerfile: "images/sandbox/Dockerfile", Remove: true, ForceRemove: true, PullParent: false, NoCache: true,
		Memory: prepared.Spec.Resources.MemoryBytes(), MemorySwap: prepared.Spec.Resources.MemoryBytes(), CPUPeriod: 100000, CPUQuota: int64(prepared.Spec.Resources.CPUMillis) * 100,
		NetworkMode: "default", Labels: runtimeIdentityLabels(prepared.Spec.Identity), Ulimits: []*container.Ulimit{{Name: "nproc", Soft: 256, Hard: 256}}}
	// No Tags: publication is a distinct, reauthorized operation.
	buildCtx, buildCancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	monitorCtx, stopMonitor := context.WithCancel(context.Background())
	done := make(chan struct{})
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		b.cancelOwnedRuntimeBuild(requestCtx, monitorCtx, prepared.Spec.Identity, done, buildCancel)
	}()
	defer func() { close(done); stopMonitor(); <-watchDone; buildCancel() }()
	response, err := api.ImageBuild(buildCtx, bytes.NewReader(archive), options)
	if err != nil || response.Body == nil {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	stream, streamErr := consumeRuntimeBuild(buildCtx, response.Body)
	closeErr := response.Body.Close()
	if streamErr != nil {
		return backend.RuntimeImageBuildResult{}, streamErr
	}
	if closeErr != nil || buildCtx.Err() != nil {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	if err = b.runtimeIntermediatesAbsent(buildCtx, prepared.Spec.Identity); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	if stream.failure != "" {
		return backend.RuntimeImageBuildResult{Settled: true}, fmt.Errorf("runtime image build failed: %s: %w", stream.failure, backend.ErrUnavailable)
	}
	if !stream.success {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	if err = b.verifyRuntimeImage(buildCtx, api, prepared.Spec.Identity, stream.imageID); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	return backend.RuntimeImageBuildResult{ImageID: stream.imageID, Settled: true}, nil
}

func (b *Backend) PublishRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage, imageID string) (backend.RuntimeImagePublishResult, error) {
	if prepared.ContextDigest != prepared.Spec.Identity.SourceDigest || !fenceHex(prepared.ContextDigest, 32) {
		return backend.RuntimeImagePublishResult{}, backend.ErrLaunchFenceOwnership
	}
	tag, err := runtimeImageTag(prepared.Spec.ImageTag)
	if err != nil || tag != prepared.Spec.ImageTag {
		return backend.RuntimeImagePublishResult{}, backend.ErrLaunchFenceOwnership
	}
	api, ok := b.api.(runtimeImageClient)
	if !ok {
		return backend.RuntimeImagePublishResult{}, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err = b.verifyRuntimeImage(ctx, api, prepared.Spec.Identity, imageID); err != nil {
		return backend.RuntimeImagePublishResult{}, err
	}
	current, err := api.ImageInspect(ctx, tag)
	if err == nil && current.ID != imageID {
		return backend.RuntimeImagePublishResult{}, fmt.Errorf("configured runtime tag already names a different image; existing image retained: %w", backend.ErrLaunchFenceOwnership)
	}
	if err != nil && !errdefs.IsNotFound(err) {
		return backend.RuntimeImagePublishResult{}, backend.ErrUnavailable
	}
	if errdefs.IsNotFound(err) {
		if err = b.runtimeImageIdentity(ctx, prepared.Spec.Identity); err != nil {
			return backend.RuntimeImagePublishResult{}, err
		}
		// The coordinator holds the engine/canonical-tag publication lock. Docker
		// has no tag CAS: this refuses an observed differing tag but cannot promise
		// atomic protection from independent privileged daemon writers.
		if err = api.ImageTag(ctx, imageID, tag); err != nil {
			return backend.RuntimeImagePublishResult{}, backend.ErrUnavailable
		}
	}
	current, err = api.ImageInspect(ctx, tag)
	if err != nil || current.ID != imageID || ctx.Err() != nil {
		return backend.RuntimeImagePublishResult{}, backend.ErrUnavailable
	}
	if err = b.verifyRuntimeImage(ctx, api, prepared.Spec.Identity, imageID); err != nil {
		return backend.RuntimeImagePublishResult{}, err
	}
	return backend.RuntimeImagePublishResult{ImageID: imageID, ImageTag: tag, Acknowledged: true}, nil
}

// ObserveRuntimeImage reconciles only publication. The coordinator must already
// possess a retained positively settled build receipt; even image/container
// absence is not evidence that a cancelled or disconnected build has ended.
func (b *Backend) ObserveRuntimeImage(ctx context.Context, identity backend.RuntimeImageIdentity, imageID, imageTag string) (backend.RuntimeImageObservation, error) {
	tag, err := runtimeImageTag(imageTag)
	if err != nil || !runtimeImageID(imageID) {
		return backend.RuntimeImageObservation{}, backend.ErrLaunchFenceOwnership
	}
	api, ok := b.api.(runtimeImageClient)
	if !ok {
		return backend.RuntimeImageObservation{}, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err = b.runtimeImageIdentity(ctx, identity); err != nil {
		return backend.RuntimeImageObservation{}, err
	}
	observation := backend.RuntimeImageObservation{ImageID: imageID, ImageTag: tag}
	_, err = api.ImageInspect(ctx, imageID)
	if errdefs.IsNotFound(err) {
		if err = b.runtimeImageIdentity(ctx, identity); err != nil {
			return backend.RuntimeImageObservation{}, err
		}
		observation.Publication = "image-absent"
		return observation, nil
	}
	if err != nil {
		return backend.RuntimeImageObservation{}, backend.ErrUnavailable
	}
	if err = b.verifyRuntimeImage(ctx, api, identity, imageID); err != nil {
		return backend.RuntimeImageObservation{}, err
	}
	current, err := api.ImageInspect(ctx, tag)
	switch {
	case errdefs.IsNotFound(err):
		observation.Publication = "tag-absent"
	case err != nil:
		return backend.RuntimeImageObservation{}, backend.ErrUnavailable
	case current.ID != imageID:
		observation.Publication = "tag-different"
	default:
		observation.Publication = "published"
	}
	if err = b.verifyRuntimeImage(ctx, api, identity, imageID); err != nil {
		return backend.RuntimeImageObservation{}, err
	}
	return observation, nil
}
