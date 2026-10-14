package dockerbe

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

var _ backend.RuntimeImageImportExecutor = (*Backend)(nil)

type runtimeImageLoadClient interface {
	ImageLoad(context.Context, io.Reader, ...client.ImageLoadOption) (image.LoadResponse, error)
	ImageInspect(context.Context, string, ...client.ImageInspectOption) (image.InspectResponse, error)
}

// PreparePackagedRuntimeImage performs all package reads and engine checks
// before any daemon mutation. The caller retains and closes the held bundle.
func (b *Backend) PreparePackagedRuntimeImage(ctx context.Context, spec backend.RuntimeImageSpec) (backend.PreparedRuntimeImage, error) {
	if !validRuntimeIdentity(spec.Identity, false) || spec.Resources.MemoryMiB <= 0 || int64(spec.Resources.MemoryMiB) > math.MaxInt64>>20 || spec.Resources.CPUMillis <= 0 {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	tag, err := runtimeImageTag(spec.ImageTag)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	spec.ImageTag = tag
	bundle, err := runtimeoci.Open(spec.SourceDirectory)
	if err != nil {
		return backend.PreparedRuntimeImage{}, fmt.Errorf("installed runtime distribution is unavailable: %w", err)
	}
	closeBundle := true
	defer func() {
		if closeBundle {
			bundle.Close()
		}
	}()
	probe, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	fence, err := b.LaunchFenceIdentity(probe)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	if spec.Identity.EngineID != "" && spec.Identity.EngineID != fence.EngineID || spec.Identity.InstanceID != "" && spec.Identity.InstanceID != fence.InstanceID {
		return backend.PreparedRuntimeImage{}, backend.ErrLaunchFenceOwnership
	}
	spec.Identity.EngineID, spec.Identity.InstanceID, spec.Identity.SourceDigest = fence.EngineID, fence.InstanceID, bundle.Manifest.CanonicalInputDigest
	limits, ok := b.api.(resourceLimitClient)
	if !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	info, err := limits.Info(probe)
	if err != nil || probe.Err() != nil {
		return backend.PreparedRuntimeImage{}, backend.ErrUnavailable
	}
	if info.ID != fence.EngineID || info.OSType != "linux" {
		return backend.PreparedRuntimeImage{}, backend.ErrLaunchFenceOwnership
	}
	if !requestedLimitsSupported(info, spec.Resources) {
		return backend.PreparedRuntimeImage{}, backend.ErrResourceLimitsUnsupported
	}
	if info.Driver != "vfs" && info.Driver != "overlay2" && info.Driver != "overlayfs" {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	if _, ok = b.api.(runtimeImageLoadClient); !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	if _, ok = b.api.(runtimeImageClient); !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	// Preserve the existing 32 GiB admission until installed import measurements
	// justify a smaller exact disk allowance. No RUN memory or cold pull is used.
	closeBundle = false
	return backend.PreparedRuntimeImage{Spec: spec, ContextDigest: bundle.Manifest.CanonicalInputDigest, RequiredDiskBytes: backend.RuntimeImageRequiredDiskBytes, AdmissionMemoryMiB: spec.Resources.MemoryMiB, StorageDriver: info.Driver, Bundle: bundle}, nil
}

func (b *Backend) inspectSharedRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage) error {
	if prepared.Bundle == nil || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return backend.ErrLaunchFenceOwnership
	}
	images, ok := b.api.(runtimeImageLoadClient)
	if !ok {
		return backend.ErrUnsupported
	}
	row, err := images.ImageInspect(ctx, prepared.Bundle.Manifest.Image.ID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return err
		}
		return backend.ErrUnavailable
	}
	if row.ID != prepared.Bundle.Manifest.Image.ID || !runtimeBaseValid(row) || row.Architecture != "amd64" || row.Config.User != "dev" || row.Config.WorkingDir != "/home/dev" || len(row.Config.Entrypoint) != 1 || row.Config.Entrypoint[0] != "/usr/local/bin/entrypoint.sh" {
		return backend.ErrLaunchFenceOwnership
	}
	for key := range row.Config.Labels {
		if strings.HasPrefix(key, "luci.runtime-image") || key == backend.LabelInstance {
			return backend.ErrLaunchFenceOwnership
		}
	}
	return b.runtimeImageIdentity(ctx, prepared.Spec.Identity)
}

// LoadPackagedRuntimeImage forwards only the held, reverified tagless tar.
// Stream interruption keeps the coordinator's charged import phase uncertain;
// a missing image, endpoint timeout or caller cancellation is not settlement.
func (b *Backend) LoadPackagedRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
	if prepared.Bundle == nil || prepared.ContextDigest != prepared.Bundle.Manifest.CanonicalInputDigest || prepared.Spec.Identity.SourceDigest != prepared.ContextDigest {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrLaunchFenceOwnership
	}
	if err := b.runtimeImageIdentity(ctx, prepared.Spec.Identity); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	if err := prepared.Bundle.VerifyArchive(); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	// A prior package import may be shared by a later exact owned installation.
	// Verify its immutable ID/config without issuing a duplicate daemon write.
	if err := b.inspectSharedRuntimeImage(ctx, prepared); err == nil {
		return backend.RuntimeImageBuildResult{ImageID: prepared.Bundle.Manifest.Image.ID, Settled: true}, nil
	} else if !errdefs.IsNotFound(err) {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	images, ok := b.api.(runtimeImageLoadClient)
	if !ok {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrUnsupported
	}
	response, err := images.ImageLoad(ctx, io.LimitReader(prepared.Bundle.Archive, prepared.Bundle.Manifest.Archive.Bytes))
	if err != nil || response.Body == nil {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	reader := &io.LimitedReader{R: response.Body, N: runtimeImageStreamLimit + 1}
	decoder := json.NewDecoder(reader)
	failure := ""
	complete := false
	for {
		var msg runtimeStreamMessage
		e := decoder.Decode(&msg)
		if e == io.EOF {
			complete = true
			break
		}
		if e != nil {
			break
		}
		if detail := runtimeMessageError(msg); detail != "" {
			failure = detail
		}
	}
	closeErr := response.Body.Close()
	if !complete || reader.N == 0 || closeErr != nil || ctx.Err() != nil {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	probe, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if failure != "" {
		return backend.RuntimeImageBuildResult{Settled: true}, fmt.Errorf("runtime import failed: %s: %w", failure, backend.ErrUnavailable)
	}
	if err = b.inspectSharedRuntimeImage(probe, prepared); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	return backend.RuntimeImageBuildResult{ImageID: prepared.Bundle.Manifest.Image.ID, Settled: true}, nil
}

// DerivePackagedRuntimeImage builds a metadata-only FROM/LABEL Dockerfile from
// the exact imported immutable ID. It contains no RUN, pull, npm, apt or Go.
func (b *Backend) DerivePackagedRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
	if prepared.Bundle == nil {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrUnsupported
	}
	if err := b.inspectSharedRuntimeImage(ctx, prepared); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	identity := prepared.Spec.Identity
	labels := runtimeIdentityLabels(identity)
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var dockerfile strings.Builder
	dockerfile.WriteString("FROM " + prepared.Bundle.Manifest.Image.ID + "\nLABEL")
	for _, key := range keys {
		dockerfile.WriteString(" " + key + "=" + fmt.Sprintf("%q", labels[key]))
	}
	dockerfile.WriteByte('\n')
	content := []byte(dockerfile.String())
	var contextTar bytes.Buffer
	tarWriter := tar.NewWriter(&contextTar)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "Dockerfile", Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(content))}); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	if _, err := tarWriter.Write(content); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	if err := tarWriter.Close(); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	images, ok := b.api.(runtimeImageClient)
	if !ok {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrUnsupported
	}
	options := build.ImageBuildOptions{Version: build.BuilderV1, Dockerfile: "Dockerfile", Remove: true, ForceRemove: true, PullParent: false, NoCache: true, Memory: prepared.Spec.Resources.MemoryBytes(), MemorySwap: prepared.Spec.Resources.MemoryBytes(), CPUPeriod: 100000, CPUQuota: int64(prepared.Spec.Resources.CPUMillis) * 100, NetworkMode: "none", Labels: labels}
	buildCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	response, err := images.ImageBuild(buildCtx, bytes.NewReader(contextTar.Bytes()), options)
	if err != nil || response.Body == nil {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	stream, streamErr := consumeRuntimeBuild(buildCtx, response.Body)
	closeErr := response.Body.Close()
	if streamErr != nil || closeErr != nil || buildCtx.Err() != nil {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	if err = b.runtimeIntermediatesAbsent(buildCtx, identity); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	if stream.failure != "" {
		return backend.RuntimeImageBuildResult{Settled: true}, fmt.Errorf("runtime derivative failed: %s: %w", stream.failure, backend.ErrUnavailable)
	}
	if !stream.success || !runtimeImageID(stream.imageID) {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	if err = b.verifyRuntimeImage(buildCtx, images, identity, stream.imageID); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	return backend.RuntimeImageBuildResult{ImageID: stream.imageID, Settled: true}, nil
}
