package dockerbe

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const (
	checkLabel             = "luci.check"
	checkProtocolLabel     = "luci.check-protocol"
	checkEngineLabel       = "luci.check-engine"
	checkAllocationLabel   = "luci.check-allocation"
	checkIDLabel           = "luci.check-id"
	checkIndexLabel        = "luci.check-command-index"
	checkScopeLabel        = "luci.check-scope"
	checkProjectLabel      = "luci.check-project"
	checkCommandLabel      = "luci.check-command-digest"
	checkSourceLabel       = "luci.check-source-digest"
	checkImageLabel        = "luci.check-image"
	maxCheckLogBytes       = 256 << 10
	checkObservationBudget = 5 * time.Second
)

var checkExecutablePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.+-]{0,79}$`)
var _ backend.CheckExecutor = (*Backend)(nil)

// Optional SDK capabilities keep existing agent-only fakes honest. Production
// uses the same configured client and engine for both agents and checks.
type checkLogClient interface {
	ContainerLogs(context.Context, string, container.LogsOptions) (io.ReadCloser, error)
}

func checkName(identity backend.CheckIdentity) string { return "luci-check-" + identity.AllocationID }

func validCheckIdentity(identity backend.CheckIdentity, prepared bool) bool {
	if identity.Protocol != backend.CheckExecutionProtocol || !fenceLogicalID(identity.AllocationID) ||
		!fenceLogicalID(identity.CheckID) || !fenceLogicalID(identity.ProjectID) ||
		identity.CommandIndex < 0 || identity.CommandIndex >= 8 ||
		!fenceHex(identity.ScopeDigest, 32) || !fenceHex(identity.CommandDigest, 32) {
		return false
	}
	if !prepared && identity.EngineID == "" && identity.InstanceID == "" {
		return true
	}
	return identity.EngineID != "" && len(identity.EngineID) <= 256 && strings.TrimSpace(identity.EngineID) == identity.EngineID &&
		!strings.ContainsAny(identity.EngineID, "\x00\r\n") && backend.ValidateInstance(identity.InstanceID) == nil
}

func validCheckSpec(spec backend.CheckSpec) bool {
	if !validCheckIdentity(spec.Identity, false) || !checkExecutablePattern.MatchString(spec.Executable) ||
		len(spec.Args) > 100 || !fenceHex(spec.SourceDigest, 32) ||
		!filepath.IsAbs(spec.SourceDirectory) || filepath.Clean(spec.SourceDirectory) != spec.SourceDirectory ||
		strings.ContainsRune(spec.SourceDirectory, 0) ||
		spec.UserID < 0 || spec.GroupID < 0 || int64(spec.UserID) > math.MaxInt32 || int64(spec.GroupID) > math.MaxInt32 ||
		spec.Resources.MemoryMiB <= 0 || int64(spec.Resources.MemoryMiB) > math.MaxInt64>>20 ||
		spec.Resources.CPUMillis <= 0 || int64(spec.Resources.CPUMillis) > math.MaxInt64/1_000_000 {
		return false
	}
	bytes := 0
	for _, arg := range spec.Args {
		bytes += len(arg)
		if strings.ContainsRune(arg, 0) || bytes > 32<<10 {
			return false
		}
	}
	return backend.CheckCommandDigest(spec) == spec.Identity.CommandDigest
}

func checkSourceDirectory(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && resolved == path
}

func (b *Backend) checkIdentity(ctx context.Context, expected backend.CheckIdentity) error {
	if !validCheckIdentity(expected, true) || expected.InstanceID != b.instance {
		return backend.ErrLaunchFenceOwnership
	}
	actual, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return err
	}
	if actual.EngineID != expected.EngineID || actual.InstanceID != expected.InstanceID {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}

func (b *Backend) PrepareCheck(ctx context.Context, spec backend.CheckSpec) (backend.PreparedCheck, error) {
	if !validCheckSpec(spec) || !checkSourceDirectory(spec.SourceDirectory) || strings.TrimSpace(b.resourceImage) == "" {
		return backend.PreparedCheck{}, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	identity, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return backend.PreparedCheck{}, err
	}
	if spec.Identity.EngineID != "" && spec.Identity.EngineID != identity.EngineID ||
		spec.Identity.InstanceID != "" && spec.Identity.InstanceID != identity.InstanceID {
		return backend.PreparedCheck{}, backend.ErrLaunchFenceOwnership
	}
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return backend.PreparedCheck{}, backend.ErrUnsupported
	}
	image, err := images.ImageInspect(ctx, b.resourceImage)
	if err != nil || ctx.Err() != nil {
		return backend.PreparedCheck{}, backend.ErrUnavailable
	}
	if !strings.HasPrefix(image.ID, "sha256:") || !fenceHex(strings.TrimPrefix(image.ID, "sha256:"), 32) || image.Size <= 0 || image.Os != "linux" ||
		image.Config == nil || len(image.Config.Volumes) != 0 {
		return backend.PreparedCheck{}, backend.ErrUnsupported
	}
	info, err := b.api.(fenceIdentityClient).Info(ctx)
	if err != nil || ctx.Err() != nil {
		return backend.PreparedCheck{}, backend.ErrUnavailable
	}
	if info.ID != identity.EngineID || info.OSType != "linux" {
		return backend.PreparedCheck{}, backend.ErrLaunchFenceOwnership
	}
	if info.Driver != "vfs" && info.Driver != "overlay2" && info.Driver != "overlayfs" {
		return backend.PreparedCheck{}, backend.ErrUnsupported
	}
	if !requestedLimitsSupported(info, spec.Resources) {
		// Docker otherwise discards these requested limits and may return a
		// successful Create with warnings. Do not allocate under that ambiguity.
		return backend.PreparedCheck{}, backend.ErrResourceLimitsUnsupported
	}
	spec.Identity.EngineID, spec.Identity.InstanceID = identity.EngineID, identity.InstanceID
	spec.Args = append([]string(nil), spec.Args...)
	return backend.PreparedCheck{Spec: spec, ImageID: image.ID, ImageVirtualSizeBytes: uint64(image.Size), StorageDriver: info.Driver}, nil
}

func checkIdentityLabels(identity backend.CheckIdentity) map[string]string {
	return map[string]string{
		checkLabel: "true", checkProtocolLabel: strconv.Itoa(identity.Protocol),
		backend.LabelInstance: identity.InstanceID, checkEngineLabel: identity.EngineID,
		checkAllocationLabel: identity.AllocationID, checkIDLabel: identity.CheckID,
		checkIndexLabel: strconv.Itoa(identity.CommandIndex), checkScopeLabel: identity.ScopeDigest,
		checkProjectLabel: identity.ProjectID, checkCommandLabel: identity.CommandDigest,
	}
}

func checkConfiguration(prepared backend.PreparedCheck) (*container.Config, *container.HostConfig) {
	spec := prepared.Spec
	labels := checkIdentityLabels(spec.Identity)
	labels[checkSourceLabel], labels[checkImageLabel] = spec.SourceDigest, prepared.ImageID
	init, pids := true, int64(256)
	return &container.Config{
			Image: prepared.ImageID, Entrypoint: []string{spec.Executable}, Cmd: append([]string(nil), spec.Args...),
			User: strconv.Itoa(spec.UserID) + ":" + strconv.Itoa(spec.GroupID), WorkingDir: "/workspace",
			Env:    []string{"HOME=/tmp", "GOCACHE=/tmp/go-cache", "GOMODCACHE=/tmp/go-mod", "npm_config_cache=/tmp/npm-cache"},
			Labels: labels, AttachStdout: true, AttachStderr: true,
			Healthcheck: &container.HealthConfig{Test: []string{"NONE"}},
		}, &container.HostConfig{
			Init: &init, ReadonlyRootfs: true, AutoRemove: false,
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
			CapDrop:       []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"},
			Tmpfs:  map[string]string{"/tmp": "rw,nosuid,nodev,size=1g"},
			Mounts: []mount.Mount{{Type: mount.TypeBind, Source: spec.SourceDirectory, Target: "/workspace"}},
			// Equal memory and combined memory+swap limits prohibit extra swap.
			Resources: container.Resources{Memory: spec.Resources.MemoryBytes(), MemorySwap: spec.Resources.MemoryBytes(), NanoCPUs: spec.Resources.NanoCPUs(), PidsLimit: &pids},
		}
}

func (b *Backend) CreateCheck(ctx context.Context, prepared backend.PreparedCheck) (string, error) {
	// An admission receipt identifies these exact resolved image bytes. A tag,
	// daemon, driver or source-path change refuses the allocation, never pulls.
	fresh, err := b.PrepareCheck(ctx, prepared.Spec)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(fresh, prepared) {
		return "", backend.ErrLaunchFenceOwnership
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	config, host := checkConfiguration(prepared)
	created, err := b.api.ContainerCreate(ctx, config, host, nil, nil, checkName(prepared.Spec.Identity))
	if err != nil || ctx.Err() != nil || !fenceHex(created.ID, 32) {
		// No implicit cleanup, name adoption, retry, or Start. The coordinator
		// retains its durable creating phase until an acknowledged fence proves
		// that a delayed Create cannot later acquire this logical name.
		return "", backend.ErrUnavailable
	}
	return created.ID, nil
}

// Ownership of an acknowledged immutable allocation is sufficient for safe
// cleanup even when its execution configuration is invalid. Starting it or
// treating it as verified output requires the stronger check below.
func checkOwnedReference(identity backend.CheckIdentity, ref string, row container.InspectResponse) bool {
	if row.ContainerJSONBase == nil || !fenceHex(ref, 32) || row.ID != ref || row.Name != "/"+checkName(identity) ||
		row.Config == nil || row.State == nil || fenceMarked(row) {
		return false
	}
	cfg := row.Config
	for key, value := range checkIdentityLabels(identity) {
		if cfg.Labels[key] != value {
			return false
		}
	}
	return true
}

// checkOwnedContainer validates the configuration bound by the controller's
// command digest. Extra image environment/port metadata is harmless; publishing
// ports, extra mounts or changed hard limits cannot authorize a Start/evidence.
func checkOwnedContainer(identity backend.CheckIdentity, ref string, row container.InspectResponse) bool {
	if !checkOwnedReference(identity, ref, row) || row.HostConfig == nil {
		return false
	}
	cfg, host := row.Config, row.HostConfig
	for _, key := range []string{backend.LabelManaged, backend.LabelSandboxID, backend.LabelSessionID} {
		if _, present := cfg.Labels[key]; present {
			return false
		}
	}
	if !strings.HasPrefix(cfg.Image, "sha256:") || !fenceHex(strings.TrimPrefix(cfg.Image, "sha256:"), 32) ||
		cfg.Image != cfg.Labels[checkImageLabel] || row.Image != cfg.Image ||
		len(cfg.Entrypoint) != 1 || cfg.WorkingDir != "/workspace" || cfg.Tty || cfg.OpenStdin || cfg.StdinOnce || cfg.AttachStdin ||
		cfg.Healthcheck == nil || !reflect.DeepEqual(cfg.Healthcheck.Test, []string{"NONE"}) ||
		len(cfg.Volumes) != 0 || host.AutoRemove || host.Init == nil || !*host.Init || !host.ReadonlyRootfs || host.Privileged ||
		(host.NetworkMode != "" && host.NetworkMode != "default" && host.NetworkMode != "bridge") ||
		(host.IpcMode != "" && host.IpcMode != "private") || host.PidMode != "" || host.UTSMode != "" || host.UsernsMode != "" ||
		host.PublishAllPorts || len(host.PortBindings) != 0 || len(host.CapAdd) != 0 || len(host.CapDrop) != 1 || host.CapDrop[0] != "ALL" ||
		!reflect.DeepEqual(host.SecurityOpt, []string{"no-new-privileges:true"}) ||
		host.RestartPolicy.Name != container.RestartPolicyDisabled || host.RestartPolicy.MaximumRetryCount != 0 ||
		len(host.Binds) != 0 || len(host.VolumesFrom) != 0 || len(host.Devices) != 0 || len(host.DeviceRequests) != 0 || len(host.Links) != 0 ||
		len(host.ExtraHosts) != 0 || len(host.Mounts) != 1 || host.Mounts[0].Type != mount.TypeBind || host.Mounts[0].Target != "/workspace" || host.Mounts[0].ReadOnly ||
		!reflect.DeepEqual(host.Tmpfs, map[string]string{"/tmp": "rw,nosuid,nodev,size=1g"}) ||
		host.PidsLimit == nil || *host.PidsLimit != 256 || host.Memory <= 0 || host.MemorySwap != host.Memory || host.Memory%(1<<20) != 0 || host.NanoCPUs <= 0 || host.NanoCPUs%1_000_000 != 0 {
		return false
	}
	if len(row.Mounts) != 1 || row.Mounts[0].Type != mount.TypeBind || row.Mounts[0].Source != host.Mounts[0].Source || row.Mounts[0].Destination != "/workspace" || !row.Mounts[0].RW {
		return false
	}
	ids := strings.Split(cfg.User, ":")
	if len(ids) != 2 {
		return false
	}
	uid, err1 := strconv.Atoi(ids[0])
	gid, err2 := strconv.Atoi(ids[1])
	if err1 != nil || err2 != nil {
		return false
	}
	env := map[string]string{}
	for _, entry := range cfg.Env {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			return false
		}
		if _, duplicate := env[key]; duplicate {
			return false
		}
		env[key] = value
	}
	if env["HOME"] != "/tmp" || env["GOCACHE"] != "/tmp/go-cache" || env["GOMODCACHE"] != "/tmp/go-mod" || env["npm_config_cache"] != "/tmp/npm-cache" {
		return false
	}
	spec := backend.CheckSpec{Identity: identity, Executable: cfg.Entrypoint[0], Args: cfg.Cmd,
		SourceDirectory: host.Mounts[0].Source, SourceDigest: cfg.Labels[checkSourceLabel],
		Resources: backend.Resources{MemoryMiB: int(host.Memory >> 20), CPUMillis: int(host.NanoCPUs / 1_000_000)}, UserID: uid, GroupID: gid}
	return validCheckSpec(spec)
}

func (b *Backend) inspectCheck(ctx context.Context, identity backend.CheckIdentity, ref string, requireConfiguration bool) (container.InspectResponse, bool, error) {
	if !fenceHex(ref, 32) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err := b.checkIdentity(ctx, identity); err != nil {
		return container.InspectResponse{}, false, err
	}
	row, err := b.api.ContainerInspect(ctx, ref)
	if err != nil {
		if ctx.Err() == nil && stoppedCleanupNotFound(err) {
			if err = b.checkIdentity(ctx, identity); err != nil {
				return container.InspectResponse{}, false, err
			}
			return container.InspectResponse{}, true, nil
		}
		return container.InspectResponse{}, false, backend.ErrUnavailable
	}
	if ctx.Err() != nil || !checkOwnedReference(identity, ref, row) || requireConfiguration && !checkOwnedContainer(identity, ref, row) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err = b.checkIdentity(ctx, identity); err != nil {
		return container.InspectResponse{}, false, err
	}
	return row, false, nil
}

func (b *Backend) StartCheck(ctx context.Context, identity backend.CheckIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	row, absent, err := b.inspectCheck(ctx, identity, ref, true)
	if err != nil {
		return err
	}
	if absent || row.State.Status != "created" || row.State.Running || row.State.Restarting || row.State.Paused || row.State.Dead || row.State.Pid != 0 {
		return backend.ErrNotRunning
	}
	if err := b.api.ContainerStart(ctx, ref, container.StartOptions{}); err != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	return nil
}

func (b *Backend) InspectCheck(ctx context.Context, identity backend.CheckIdentity, ref string) (backend.CheckObservation, error) {
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	row, absent, err := b.inspectCheck(ctx, identity, ref, true)
	if err != nil {
		return backend.CheckObservation{}, err
	}
	result := backend.CheckObservation{Ref: ref}
	if absent {
		result.State = "absent"
		return result, nil
	}
	if row.State.Paused || row.State.Restarting || row.State.Dead {
		return backend.CheckObservation{}, backend.ErrUnavailable
	}
	switch {
	case row.State.Status == "created" && !row.State.Running && row.State.Pid == 0:
		result.State = "created"
	case row.State.Status == "running" && row.State.Running:
		result.State = "running"
	case row.State.Status == "exited" && !row.State.Running && row.State.Pid == 0:
		result.State = "exited"
		code := row.State.ExitCode
		result.ExitCode = &code
	default:
		return backend.CheckObservation{}, backend.ErrUnavailable
	}
	return result, nil
}

func (b *Backend) RemoveCheck(ctx context.Context, identity backend.CheckIdentity, ref string, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	row, absent, err := b.inspectCheck(ctx, identity, ref, false)
	if err != nil || absent {
		return err
	}
	if !force && (row.State.Running || row.State.Restarting || row.State.Paused || row.State.Dead ||
		(row.State.Status != "created" && row.State.Status != "exited")) {
		return backend.ErrNotRunning
	}
	if err = b.api.ContainerRemove(ctx, ref, container.RemoveOptions{Force: force, RemoveVolumes: true}); err != nil {
		if ctx.Err() != nil || !stoppedCleanupNotFound(err) {
			return backend.ErrUnavailable
		}
	}
	// A successful remove response alone is not the coordinator's release
	// witness. Confirm this exact immutable reference is now absent.
	_, absent, err = b.inspectCheck(ctx, identity, ref, false)
	if err != nil {
		return err
	}
	if !absent {
		return backend.ErrUnavailable
	}
	return nil
}

func (b *Backend) CheckLogs(ctx context.Context, identity backend.CheckIdentity, ref string, limit int) (backend.CheckOutput, error) {
	if limit < 1 || limit > maxCheckLogBytes {
		return backend.CheckOutput{}, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, absent, err := b.inspectCheck(ctx, identity, ref, true)
	if err != nil {
		return backend.CheckOutput{}, err
	}
	if absent {
		return backend.CheckOutput{}, backend.ErrNotFound
	}
	api, ok := b.api.(checkLogClient)
	if !ok {
		return backend.CheckOutput{}, backend.ErrUnsupported
	}
	reader, err := api.ContainerLogs(ctx, ref, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: false, Tail: "all"})
	if err != nil || reader == nil {
		return backend.CheckOutput{}, backend.ErrUnavailable
	}
	defer reader.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = reader.Close() })
	defer stopClose()
	return readCheckLogs(ctx, reader, limit)
}

// Parse Docker's non-TTY framing ourselves: stdcopy may allocate the declared
// frame size before enforcing a caller's output limit. Memory here stays at
// limit+1 even for a malicious length header, and a truncated frame is failure.
func readCheckLogs(ctx context.Context, reader io.Reader, limit int) (backend.CheckOutput, error) {
	if limit < 1 || limit > maxCheckLogBytes {
		return backend.CheckOutput{}, backend.ErrUnsupported
	}
	output := make([]byte, 0, limit+1)
	// Even empty/micro frames consume a finite parsing budget. Ordinary Docker
	// log frames are much larger; an exhausted wire budget is an explicit error.
	reader = io.LimitReader(reader, 4<<20)
	for frames := 0; frames < 65_536; frames++ {
		if ctx.Err() != nil {
			return backend.CheckOutput{}, backend.ErrUnavailable
		}
		var header [8]byte
		_, err := io.ReadFull(reader, header[:])
		if ctx.Err() != nil {
			return backend.CheckOutput{}, backend.ErrUnavailable
		}
		if errors.Is(err, io.EOF) {
			return backend.CheckOutput{Text: string(output)}, nil
		}
		if err != nil || (header[0] != 1 && header[0] != 2) || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return backend.CheckOutput{}, backend.ErrUnavailable
		}
		size := uint64(binary.BigEndian.Uint32(header[4:]))
		remaining := limit + 1 - len(output)
		read := int(size)
		if size > uint64(remaining) {
			read = remaining
		}
		start := len(output)
		output = append(output, make([]byte, read)...)
		if _, err = io.ReadFull(reader, output[start:]); err != nil || ctx.Err() != nil {
			return backend.CheckOutput{}, backend.ErrUnavailable
		}
		if len(output) > limit {
			return backend.CheckOutput{Text: string(output[:limit]), Truncated: true}, nil
		}
	}
	return backend.CheckOutput{}, backend.ErrUnavailable
}
