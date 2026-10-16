package dockerbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const previewPrefix = "luci.preview-"

var _ backend.PreviewExecutor = (*Backend)(nil)

type previewBuildClient interface {
	ImageBuild(context.Context, io.Reader, build.ImageBuildOptions) (build.ImageBuildResponse, error)
}
type previewStopClient interface {
	ContainerStop(context.Context, string, container.StopOptions) error
}
type previewNetworkClient interface {
	NetworkInspect(context.Context, string, network.InspectOptions) (network.Inspect, error)
}

func previewImageID(v string) bool {
	return strings.HasPrefix(v, "sha256:") && fenceHex(strings.TrimPrefix(v, "sha256:"), 32)
}
func validPreviewIdentity(v backend.PreviewIdentity, prepared bool) bool {
	if v.Kind != "" && v.Kind != backend.ReviewedSourcePreview || v.Kind == "" && (v.SnapshotDigest != "" || v.CommandDigest != "" || v.EnvironmentDigest != "" || v.RuntimeImageID != "") || v.Kind == backend.ReviewedSourcePreview && ((v.SnapshotDigest != "" || prepared) && !fenceHex(v.SnapshotDigest, 32) || (v.CommandDigest != "" || prepared) && !fenceHex(v.CommandDigest, 32) || (v.EnvironmentDigest != "" || prepared) && !fenceHex(v.EnvironmentDigest, 32) || (v.RuntimeImageID != "" || prepared) && !previewImageID(v.RuntimeImageID)) {
		return false
	}
	if v.Protocol != backend.PreviewExecutionProtocol || !fenceLogicalID(v.AllocationID) || !fenceLogicalID(v.ReleaseID) || !fenceLogicalID(v.ProjectID) || !fenceHex(v.ScopeDigest, 32) || !fenceHex(v.SourceDigest, 32) || (!fenceHex(v.Commit, 20) && !fenceHex(v.Commit, 32)) {
		return false
	}
	if !prepared && v.EngineID == "" && v.InstanceID == "" {
		return true
	}
	return v.EngineID != "" && len(v.EngineID) <= 256 && strings.TrimSpace(v.EngineID) == v.EngineID && !strings.ContainsAny(v.EngineID, "\x00\r\n") && backend.ValidateInstance(v.InstanceID) == nil
}
func previewIdentityLabels(v backend.PreviewIdentity) map[string]string {
	labels := map[string]string{
		"luci.preview": "true", backend.LabelInstance: v.InstanceID, previewPrefix + "protocol": strconv.Itoa(v.Protocol), previewPrefix + "engine": v.EngineID, previewPrefix + "allocation": v.AllocationID, previewPrefix + "release": v.ReleaseID, previewPrefix + "scope": v.ScopeDigest, previewPrefix + "project": v.ProjectID, previewPrefix + "source": v.SourceDigest, previewPrefix + "commit": v.Commit,
	}
	if v.Kind != "" {
		labels[previewPrefix+"kind"], labels[previewPrefix+"snapshot"], labels[previewPrefix+"command"], labels[previewPrefix+"image"] = v.Kind, v.SnapshotDigest, v.CommandDigest, v.RuntimeImageID
		labels[previewPrefix+"environment"] = v.EnvironmentDigest
	}
	return labels
}
func previewName(v backend.PreviewIdentity) string { return "luci-preview-" + v.AllocationID }
func validPreviewSpec(s backend.PreviewSpec) bool {
	if !validPreviewIdentity(s.Identity, false) || (s.Kind != "preview-build" && s.Kind != "preview" && s.Kind != backend.ReviewedSourcePreview) || (s.Kind == backend.ReviewedSourcePreview) != (s.Identity.Kind == backend.ReviewedSourcePreview) || s.Resources.MemoryMiB <= 0 || int64(s.Resources.MemoryMiB) > math.MaxInt64>>20 || s.Resources.CPUMillis <= 0 || int64(s.Resources.CPUMillis) > math.MaxInt64/1000000 || s.Port < 1 || s.Port > 65535 {
		return false
	}
	if s.Kind == "preview" && (!previewImageID(s.ImageID) || s.SourceDirectory != "" || len(s.SourceArchive) != 0) {
		return false
	}
	if len(s.Environment) > 256 || len(s.Labels) > 32 {
		return false
	}
	total := 0
	for k, v := range s.Environment {
		total += len(k) + len(v)
		if k == "" || strings.ContainsAny(k, "=\x00\r\n") || strings.ContainsRune(v, 0) || total > 64<<10 {
			return false
		}
	}
	for k, v := range s.Labels {
		if k == "" || len(k) > 200 || len(v) > 1000 || strings.ContainsAny(k+v, "\x00\r\n") || k == backend.LabelManaged || k == backend.LabelInstance || k == backend.LabelSandboxID || k == backend.LabelSessionID || k == "luci.preview" || strings.HasPrefix(k, previewPrefix) {
			return false
		}
	}
	return true
}
func (b *Backend) previewIdentity(ctx context.Context, v backend.PreviewIdentity) error {
	if !validPreviewIdentity(v, true) || v.InstanceID != b.instance {
		return backend.ErrLaunchFenceOwnership
	}
	got, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return err
	}
	if got.EngineID != v.EngineID || got.InstanceID != v.InstanceID {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}
func (b *Backend) previewNetwork(ctx context.Context, s backend.PreviewSpec) error {
	if s.Network == "" || s.Network == "bridge" {
		return nil
	}
	api, ok := b.api.(previewNetworkClient)
	if !ok {
		return backend.ErrUnsupported
	}
	r, err := api.NetworkInspect(ctx, s.Network, network.InspectOptions{})
	if err != nil {
		return backend.ErrUnavailable
	}
	// Platform derives a private instance namespace from its state directory
	// and control-plane instance. It is distinct from the backend instance.
	// Spec.Labels comes only from the trusted product/platform adapters; it is
	// never a browser-supplied network ownership assertion.
	platformInstance := s.Labels["ai.lcfr.luci.platform.instance"]
	if !fenceHex(platformInstance, 8) || s.Labels["ai.lcfr.luci.platform.project"] != s.Identity.ProjectID || r.ID != s.Network || !fenceHex(r.ID, 32) || r.Driver != "bridge" || r.Labels["ai.lcfr.luci.platform.instance"] != platformInstance || r.Labels["ai.lcfr.luci.platform.project"] != s.Identity.ProjectID {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}
func (b *Backend) PreparePreview(ctx context.Context, s backend.PreviewSpec) (backend.PreparedPreview, error) {
	if !validPreviewSpec(s) {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	identity, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return backend.PreparedPreview{}, err
	}
	if s.Identity.EngineID != "" && s.Identity.EngineID != identity.EngineID || s.Identity.InstanceID != "" && s.Identity.InstanceID != identity.InstanceID {
		return backend.PreparedPreview{}, backend.ErrLaunchFenceOwnership
	}
	s.Identity.EngineID, s.Identity.InstanceID = identity.EngineID, identity.InstanceID
	api, ok := b.api.(resourceLimitClient)
	if !ok {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	info, err := api.Info(ctx)
	if err != nil {
		return backend.PreparedPreview{}, backend.ErrUnavailable
	}
	if info.ID != identity.EngineID || info.OSType != "linux" {
		return backend.PreparedPreview{}, backend.ErrLaunchFenceOwnership
	}
	if !requestedLimitsSupported(info, s.Resources) {
		return backend.PreparedPreview{}, backend.ErrResourceLimitsUnsupported
	}
	if info.Driver != "vfs" && info.Driver != "overlay2" && info.Driver != "overlayfs" {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	if err = b.previewNetwork(ctx, s); err != nil {
		return backend.PreparedPreview{}, err
	}
	p := backend.PreparedPreview{Spec: s, StorageDriver: info.Driver}
	if s.Kind == backend.ReviewedSourcePreview {
		return b.prepareReviewedSource(ctx, p)
	} else if s.Kind == "preview-build" {
		plan, e := b.planPreviewContext(ctx, s, nil)
		if e != nil {
			return backend.PreparedPreview{}, e
		}
		p.PreparedContext, p.ContextDigest, p.BaseImages, p.RequiredDiskBytes = plan.prepared, plan.digest, plan.images, plan.demand
		p.BasePlan = plan.bases
		p.Spec.ContextDigest = p.ContextDigest
		p.BasePlanDigest = previewBasePlanDigest(p)
	} else {
		images, ok := b.api.(imageResourceClient)
		if !ok {
			return backend.PreparedPreview{}, backend.ErrUnsupported
		}
		row, e := images.ImageInspect(ctx, s.ImageID)
		if e != nil {
			return backend.PreparedPreview{}, backend.ErrUnavailable
		}
		if row.ID != s.ImageID || row.Config == nil || row.Size <= 0 || row.Os != "linux" || len(row.Config.Volumes) != 0 || len(row.Config.OnBuild) != 0 {
			return backend.PreparedPreview{}, backend.ErrUnsupported
		}
		labels := previewIdentityLabels(s.Identity)
		delete(labels, previewPrefix+"allocation")
		for k, v := range labels {
			if row.Config.Labels[k] != v {
				return backend.PreparedPreview{}, backend.ErrLaunchFenceOwnership
			}
		}
		p.RequiredDiskBytes = uint64(row.Size) + (256 << 20)
	}
	if ctx.Err() != nil {
		return backend.PreparedPreview{}, ctx.Err()
	}
	return p, nil
}
func (b *Backend) BuildPreview(ctx context.Context, p backend.PreparedPreview, beforeMutation func(context.Context) error) (backend.PreviewBuildResult, error) {
	unstarted := backend.PreviewBuildResult{Settled: true}
	if p.Spec.Kind != "preview-build" || beforeMutation == nil {
		return unstarted, backend.ErrUnsupported
	}
	fresh, err := b.PreparePreview(ctx, p.Spec)
	if err != nil {
		return unstarted, err
	}
	if !reflect.DeepEqual(fresh, p) {
		return unstarted, backend.ErrLaunchFenceOwnership
	}
	api, ok := b.api.(previewBuildClient)
	if !ok {
		return unstarted, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	plan, settled, err := b.provisionPreviewBases(ctx, p, beforeMutation)
	if err != nil {
		return backend.PreviewBuildResult{Settled: settled}, err
	}
	if err = b.previewIdentity(ctx, p.Spec.Identity); err != nil {
		return unstarted, err
	}
	// Explicit classic builder: BuildKit ignores the per-build memory/CPU flags.
	// nproc is a process ulimit, not a cgroup PID bound; serving uses PidsLimit.
	opts := build.ImageBuildOptions{Version: build.BuilderV1, Dockerfile: p.Spec.Dockerfile, Remove: true, ForceRemove: true, PullParent: false, NoCache: true, Memory: p.Spec.Resources.MemoryBytes(), MemorySwap: p.Spec.Resources.MemoryBytes(), CPUPeriod: 100000, CPUQuota: int64(p.Spec.Resources.CPUMillis) * 100, NetworkMode: "default", Labels: previewIdentityLabels(p.Spec.Identity), Ulimits: []*container.Ulimit{{Name: "nproc", Soft: 256, Hard: 256}}}
	if opts.Dockerfile == "" {
		opts.Dockerfile = "Dockerfile"
	}
	if err = ctx.Err(); err != nil {
		return unstarted, err
	}
	if err = beforeMutation(ctx); err != nil {
		return unstarted, err
	}
	if err = ctx.Err(); err != nil {
		return unstarted, err
	}
	response, err := api.ImageBuild(ctx, bytes.NewReader(plan.prepared), opts)
	if err != nil || response.Body == nil {
		return backend.PreviewBuildResult{}, backend.ErrUnavailable
	}
	defer response.Body.Close()
	// A complete terminal JSON message AND complete transport AND settled owned
	// intermediates are required. A context error/truncated stream is uncertain.
	reader := &io.LimitedReader{R: response.Body, N: (4 << 20) + 1}
	decoder := json.NewDecoder(reader)
	completedFinalImage := false
	imageID := ""
	buildFailed := false
	for {
		var message struct {
			Stream      string `json:"stream"`
			Error       string `json:"error"`
			ErrorDetail *struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
			Aux *struct {
				ID string `json:"ID"`
			} `json:"aux"`
		}
		err = decoder.Decode(&message)
		if err == io.EOF {
			break
		}
		if err != nil || ctx.Err() != nil {
			return backend.PreviewBuildResult{}, backend.ErrUnavailable
		}
		if message.Error != "" || message.ErrorDetail != nil {
			buildFailed = true
		}
		if message.Aux != nil {
			if !previewImageID(message.Aux.ID) {
				return backend.PreviewBuildResult{}, backend.ErrUnavailable
			}
			// Classic Docker emits aux.ID after each stage, not just the final
			// build. A later stage invalidates every earlier success marker.
			imageID = message.Aux.ID
			completedFinalImage = false
		}
		if strings.HasPrefix(message.Stream, "Successfully built ") {
			// The pinned daemon emits this exact outer-backend line only after
			// all stages. Require its short ID to match the last full aux ID,
			// then still consume bounded EOF and prove intermediates absent.
			completedFinalImage = imageID != "" && message.Stream == "Successfully built "+strings.TrimPrefix(imageID, "sha256:")[:12]+"\n"
		}
	}
	if (!buildFailed && !completedFinalImage) || ctx.Err() != nil || reader.N == 0 {
		return backend.PreviewBuildResult{}, backend.ErrUnavailable
	}
	if err = b.previewIdentity(ctx, p.Spec.Identity); err != nil {
		return backend.PreviewBuildResult{}, err
	}
	rows, err := b.api.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(filters.Arg("label", previewPrefix+"allocation="+p.Spec.Identity.AllocationID))})
	if err != nil || len(rows) != 0 || ctx.Err() != nil {
		return backend.PreviewBuildResult{}, backend.ErrUnavailable
	}
	result := backend.PreviewBuildResult{Settled: true}
	if buildFailed {
		return result, fmt.Errorf("the bounded preview image build failed: %w", backend.ErrUnavailable)
	}
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return backend.PreviewBuildResult{}, backend.ErrUnsupported
	}
	row, err := images.ImageInspect(ctx, imageID)
	if err != nil || row.ID != imageID || row.Size <= 0 || row.Config == nil {
		return backend.PreviewBuildResult{}, backend.ErrUnavailable
	}
	for k, v := range previewIdentityLabels(p.Spec.Identity) {
		if row.Config.Labels[k] != v {
			return backend.PreviewBuildResult{}, backend.ErrLaunchFenceOwnership
		}
	}
	result.ImageID = imageID
	result.ImageVirtualSizeBytes = uint64(row.Size)
	return result, nil
}
func previewConfiguration(p backend.PreparedPreview) (*container.Config, *container.HostConfig) {
	s := p.Spec
	labels := previewIdentityLabels(s.Identity)
	for k, v := range s.Labels {
		labels[k] = v
	}
	labels[previewPrefix+"memory"] = strconv.Itoa(s.Resources.MemoryMiB)
	labels[previewPrefix+"cpu"] = strconv.Itoa(s.Resources.CPUMillis)
	labels[previewPrefix+"image"] = s.ImageID
	labels[previewPrefix+"port"] = strconv.Itoa(s.Port)
	labels[previewPrefix+"network"] = s.Network
	port := nat.Port(strconv.Itoa(s.Port) + "/tcp")
	init, pids := true, int64(256)
	keys := make([]string, 0, len(s.Environment))
	for k := range s.Environment {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+s.Environment[k])
	}
	config := &container.Config{Image: s.ImageID, Env: env, Labels: labels, ExposedPorts: nat.PortSet{port: {}}}
	host := &container.HostConfig{Init: &init, AutoRemove: false, RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled}, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, NetworkMode: container.NetworkMode(s.Network), PortBindings: nat.PortMap{port: {{HostIP: loopbackIPv4, HostPort: ""}}}, Resources: container.Resources{Memory: s.Resources.MemoryBytes(), MemorySwap: s.Resources.MemoryBytes(), NanoCPUs: s.Resources.NanoCPUs(), PidsLimit: &pids}, LogConfig: container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "10m", "max-file": "2"}}}
	if s.Kind == backend.ReviewedSourcePreview {
		configureReviewedSource(p, config, host)
	}
	return config, host
}
func (b *Backend) CreatePreview(ctx context.Context, p backend.PreparedPreview) (string, error) {
	if p.Spec.Kind != "preview" && p.Spec.Kind != backend.ReviewedSourcePreview {
		return "", backend.ErrUnsupported
	}
	fresh, err := b.PreparePreview(ctx, p.Spec)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(fresh, p) {
		return "", backend.ErrLaunchFenceOwnership
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	config, host := previewConfiguration(p)
	row, err := b.api.ContainerCreate(ctx, config, host, nil, nil, previewName(p.Spec.Identity))
	if err != nil || ctx.Err() != nil || !fenceHex(row.ID, 32) {
		return "", backend.ErrUnavailable
	}
	return row.ID, nil
}
func previewOwned(v backend.PreviewIdentity, ref string, r container.InspectResponse, strict bool) bool {
	if r.ContainerJSONBase == nil || !fenceHex(ref, 32) || r.ID != ref || r.Name != "/"+previewName(v) || r.Config == nil || r.State == nil || fenceMarked(r) {
		return false
	}
	for k, x := range previewIdentityLabels(v) {
		if r.Config.Labels[k] != x {
			return false
		}
	}
	if !strict {
		return true
	}
	cfg, h := r.Config, r.HostConfig
	if h == nil {
		return false
	}
	if v.Kind == "" && (cfg.Labels[previewPrefix+"kind"] != "" || cfg.Labels[previewPrefix+"snapshot"] != "") {
		return false
	}
	m, e1 := strconv.Atoi(cfg.Labels[previewPrefix+"memory"])
	cpu, e2 := strconv.Atoi(cfg.Labels[previewPrefix+"cpu"])
	port, e3 := strconv.Atoi(cfg.Labels[previewPrefix+"port"])
	if e1 != nil || e2 != nil || e3 != nil || m <= 0 || cpu <= 0 || port < 1 || port > 65535 || int64(m) > math.MaxInt64>>20 || int64(cpu) > math.MaxInt64/1000000 {
		return false
	}
	if v.Kind == backend.ReviewedSourcePreview && !ownedReviewedSource(v, cfg, h, port) {
		return false
	}
	if !previewImageID(cfg.Image) || r.Image != cfg.Image || cfg.Labels[previewPrefix+"image"] != cfg.Image || len(cfg.Volumes) != 0 || cfg.Tty || cfg.OpenStdin || h.Privileged || h.AutoRemove || h.Init == nil || !*h.Init || len(h.Binds) != 0 || len(h.Mounts) != 0 || len(r.Mounts) != 0 || len(h.VolumesFrom) != 0 || len(h.Devices) != 0 || len(h.DeviceRequests) != 0 || len(h.CapAdd) != 0 || len(h.CapDrop) != 1 || h.CapDrop[0] != "ALL" || !reflect.DeepEqual(h.SecurityOpt, []string{"no-new-privileges:true"}) || h.PidMode != "" || h.UTSMode != "" || h.UsernsMode != "" || h.IpcMode != "" && h.IpcMode != "private" || h.PublishAllPorts || h.RestartPolicy.Name != container.RestartPolicyDisabled || h.RestartPolicy.MaximumRetryCount != 0 || h.Memory != int64(m)<<20 || h.MemorySwap != h.Memory || h.NanoCPUs != int64(cpu)*1000000 || h.PidsLimit == nil || *h.PidsLimit != 256 {
		return false
	}
	expectedNetwork := cfg.Labels[previewPrefix+"network"]
	if expectedNetwork == "" {
		if h.NetworkMode != "" && h.NetworkMode != "default" && h.NetworkMode != "bridge" {
			return false
		}
	} else if string(h.NetworkMode) != expectedNetwork {
		return false
	}
	p := nat.Port(strconv.Itoa(port) + "/tcp")
	bindings := h.PortBindings[p]
	if len(h.PortBindings) != 1 || len(bindings) != 1 || bindings[0].HostIP != loopbackIPv4 {
		return false
	}
	return true
}
func (b *Backend) inspectPreview(ctx context.Context, v backend.PreviewIdentity, ref string, strict bool) (container.InspectResponse, bool, error) {
	if !fenceHex(ref, 32) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err := b.previewIdentity(ctx, v); err != nil {
		return container.InspectResponse{}, false, err
	}
	row, err := b.api.ContainerInspect(ctx, ref)
	if err != nil {
		if ctx.Err() == nil && stoppedCleanupNotFound(err) {
			if e := b.previewIdentity(ctx, v); e != nil {
				return container.InspectResponse{}, false, e
			}
			return container.InspectResponse{}, true, nil
		}
		return container.InspectResponse{}, false, backend.ErrUnavailable
	}
	if ctx.Err() != nil || !previewOwned(v, ref, row, strict) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err := b.previewIdentity(ctx, v); err != nil {
		return container.InspectResponse{}, false, err
	}
	return row, false, nil
}
func (b *Backend) StartPreview(ctx context.Context, v backend.PreviewIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	row, absent, err := b.inspectPreview(ctx, v, ref, true)
	if err != nil {
		return err
	}
	if absent || row.State.Running || row.State.Paused || row.State.Restarting || row.State.Dead || row.State.Pid != 0 || (row.State.Status != "created" && row.State.Status != "exited") {
		return backend.ErrNotRunning
	}
	resources := backend.Resources{MemoryMiB: int(row.HostConfig.Memory >> 20), CPUMillis: int(row.HostConfig.NanoCPUs / 1000000)}
	engine, err := b.memoryLimitIdentity(ctx, resources)
	if err != nil {
		return err
	}
	if engine != v.EngineID {
		return backend.ErrLaunchFenceOwnership
	}
	if err = b.api.ContainerStart(ctx, ref, container.StartOptions{}); err != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	return nil
}
func (b *Backend) InspectPreview(ctx context.Context, v backend.PreviewIdentity, ref string) (backend.PreviewObservation, error) {
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	row, absent, err := b.inspectPreview(ctx, v, ref, true)
	if err != nil {
		return backend.PreviewObservation{}, err
	}
	out := backend.PreviewObservation{Ref: ref}
	if absent {
		out.State = "absent"
		return out, nil
	}
	if row.State.Paused || row.State.Restarting || row.State.Dead {
		return out, backend.ErrUnavailable
	}
	switch {
	case row.State.Status == "created" && !row.State.Running && row.State.Pid == 0:
		out.State = "created"
	case row.State.Status == "exited" && !row.State.Running && row.State.Pid == 0:
		out.State = "exited"
	case row.State.Status == "running" && row.State.Running:
		out.State = "running"
	default:
		return out, backend.ErrUnavailable
	}
	if row.NetworkSettings != nil {
		port := nat.Port(row.Config.Labels[previewPrefix+"port"] + "/tcp")
		bind := row.NetworkSettings.Ports[port]
		if len(bind) > 0 {
			if len(bind) != 1 || bind[0].HostIP != loopbackIPv4 {
				return out, backend.ErrNonLoopbackPublish
			}
			n, err := strconv.Atoi(bind[0].HostPort)
			if err != nil || n < 1 || n > 65535 {
				return out, backend.ErrUnavailable
			}
			out.URL = "http://" + net.JoinHostPort(loopbackIPv4, bind[0].HostPort)
		}
	}
	return out, nil
}
func (b *Backend) StopPreview(ctx context.Context, v backend.PreviewIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	row, absent, err := b.inspectPreview(ctx, v, ref, false)
	if err != nil || absent {
		return err
	}
	if row.State.Running || row.State.Restarting || row.State.Paused {
		api, ok := b.api.(previewStopClient)
		if !ok {
			return backend.ErrUnsupported
		}
		timeout := 10
		if err = api.ContainerStop(ctx, ref, container.StopOptions{Timeout: &timeout}); err != nil {
			return backend.ErrUnavailable
		}
	}
	row, absent, err = b.inspectPreview(ctx, v, ref, false)
	if err != nil || absent {
		return err
	}
	if row.State.Running || row.State.Restarting || row.State.Paused || row.State.Dead || row.State.Pid != 0 || (row.State.Status != "exited" && row.State.Status != "created") {
		return backend.ErrUnavailable
	}
	return nil
}
func (b *Backend) RemovePreview(ctx context.Context, v backend.PreviewIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := b.StopPreview(ctx, v, ref); err != nil {
		return err
	}
	_, absent, err := b.inspectPreview(ctx, v, ref, false)
	if err != nil || absent {
		return err
	}
	if err = b.api.ContainerRemove(ctx, ref, container.RemoveOptions{RemoveVolumes: true}); err != nil && !errors.Is(err, backend.ErrNotFound) && !stoppedCleanupNotFound(err) {
		return backend.ErrUnavailable
	}
	_, absent, err = b.inspectPreview(ctx, v, ref, false)
	if err != nil {
		return err
	}
	if !absent {
		return backend.ErrUnavailable
	}
	return nil
}
