package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const documentLabel = "luci.document-extractor"
const documentPrefix = "luci-document-"

// The wrapper shares the already selected local Docker client. It cannot
// select an engine or use the worker/check image as an extraction fallback.
type DocumentExtractor struct {
	*Backend
	imageID       string
	attachHeaders map[string]string
	attached      sync.Map // exact immutable CID -> owned pre-Start stream; never recovered.
}

// Exact headers from the trusted owner of the selected local SDK client.
// A nonnil empty map explicitly declares the normal no-custom-header profile;
// an unspecified opaque client is unsupported, never silently reconstructed.
type DocumentAttachConfiguration struct{ Headers map[string]string }

var _ backend.DocumentExtractionExecutor = (*DocumentExtractor)(nil)

func NewDocumentExtractor(b *Backend, imageID string, attach DocumentAttachConfiguration) (*DocumentExtractor, error) {
	if b == nil || !strings.HasPrefix(imageID, "sha256:") || !fenceHex(strings.TrimPrefix(imageID, "sha256:"), 32) {
		return nil, backend.ErrUnsupported
	}
	if attach.Headers == nil || len(attach.Headers) > 32 {
		return nil, backend.ErrUnsupported
	}
	headers := map[string]string{}
	total := 0
	for name, value := range attach.Headers {
		total += len(name) + len(value)
		if name == "" || strings.ContainsAny(name+value, "\x00\r\n") || total > 16<<10 {
			return nil, backend.ErrUnsupported
		}
		switch strings.ToLower(name) {
		case "host", "connection", "upgrade", "content-length", "content-type":
			return nil, backend.ErrUnsupported
		}
		headers[name] = value
	}
	return &DocumentExtractor{Backend: b, imageID: imageID, attachHeaders: headers}, nil
}
func documentName(identity backend.DocumentExtractionIdentity) string {
	return documentPrefix + identity.AllocationID
}
func documentLabels(identity backend.DocumentExtractionIdentity) map[string]string {
	return map[string]string{documentLabel: "true", "luci.document-protocol": strconv.Itoa(identity.Protocol), "luci.document-profile": identity.Profile, backend.LabelInstance: identity.InstanceID, "luci.document-engine": identity.EngineID, "luci.document-allocation": identity.AllocationID, "luci.document-scope": identity.ScopeDigest, "luci.document-actor": identity.ActorDigest, "luci.document-project": identity.ProjectID, "luci.document-id": identity.DocumentID, "luci.document-attempt": identity.AttemptID, "luci.document-original": identity.OriginalDigest, "luci.document-command": identity.CommandDigest}
}
func (b *DocumentExtractor) documentIdentity(ctx context.Context, identity backend.DocumentExtractionIdentity) error {
	if !backend.ValidateDocumentExtractionIdentity(identity, true) || identity.InstanceID != b.instance {
		return backend.ErrLaunchFenceOwnership
	}
	actual, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return err
	}
	if actual.EngineID != identity.EngineID || actual.InstanceID != identity.InstanceID {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}
func verifyDocumentOriginal(spec backend.DocumentExtractionSpec) bool {
	resolved, err := filepath.EvalSymlinks(spec.OriginalPath)
	if err != nil || resolved != spec.OriginalPath || hostpath.OneName(spec.OriginalPath) != nil {
		return false
	}
	before, err := os.Lstat(spec.OriginalPath)
	if err != nil || !before.Mode().IsRegular() || before.Size() != int64(spec.OriginalBytes) {
		return false
	}
	file, err := os.Open(spec.OriginalPath)
	if err != nil {
		return false
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return false
	}
	digest := sha256.New()
	bytes, err := io.Copy(digest, io.LimitReader(file, backend.DocumentOriginalMaxBytes+1))
	return err == nil && bytes == int64(spec.OriginalBytes) && hex.EncodeToString(digest.Sum(nil)) == spec.Identity.OriginalDigest
}
func documentEnvironment() []string {
	return []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/tmp", "TMPDIR=/tmp", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "OMP_THREAD_LIMIT=1", "OMP_NUM_THREADS=1", "TESSDATA_PREFIX=/opt/luci/tessdata", "PYTHONDONTWRITEBYTECODE=1"}
}
func documentConfiguration(prepared backend.PreparedDocumentExtraction) (*container.Config, *container.HostConfig) {
	spec := prepared.Spec
	labels := documentLabels(spec.Identity)
	labels["luci.document-image"] = spec.ImageID
	labels["luci.document-mime"] = spec.MimeType
	labels["luci.document-bytes"] = strconv.Itoa(spec.OriginalBytes)
	init, pids := true, int64(32)
	return &container.Config{Image: spec.ImageID, Entrypoint: []string{"/usr/bin/python3"}, Cmd: []string{"-I", "/opt/luci/document_extractor.py", "--original", "/input/original", "--digest", spec.Identity.OriginalDigest, "--mime", spec.MimeType, "--bytes", strconv.Itoa(spec.OriginalBytes), "--attempt", spec.Identity.AttemptID}, User: strconv.Itoa(spec.UserID) + ":" + strconv.Itoa(spec.GroupID), WorkingDir: "/tmp", Env: documentEnvironment(), NetworkDisabled: true, AttachStdout: true, Labels: labels, Healthcheck: &container.HealthConfig{Test: []string{"NONE"}}}, &container.HostConfig{Init: &init, ReadonlyRootfs: true, AutoRemove: false, RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled}, NetworkMode: "none", IpcMode: "none", CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, Tmpfs: map[string]string{"/tmp": "rw,nosuid,nodev,noexec,size=128m,mode=1777"}, Mounts: []mount.Mount{{Type: mount.TypeBind, Source: spec.OriginalPath, Target: "/input/original", ReadOnly: true}}, LogConfig: container.LogConfig{Type: "none"}, Resources: container.Resources{Memory: 512 << 20, MemorySwap: 512 << 20, NanoCPUs: 1_000_000_000, PidsLimit: &pids}}
}
func (b *DocumentExtractor) PrepareDocument(ctx context.Context, spec backend.DocumentExtractionSpec) (backend.PreparedDocumentExtraction, error) {
	if !backend.ValidateDocumentExtractionSpec(spec) || spec.ImageID != b.imageID || !verifyDocumentOriginal(spec) {
		return backend.PreparedDocumentExtraction{}, backend.ErrUnsupported
	}
	if _, ok := b.api.(documentAttachSource); !ok {
		return backend.PreparedDocumentExtraction{}, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	identity, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return backend.PreparedDocumentExtraction{}, err
	}
	if spec.Identity.EngineID != "" && spec.Identity.EngineID != identity.EngineID || spec.Identity.InstanceID != "" && spec.Identity.InstanceID != identity.InstanceID {
		return backend.PreparedDocumentExtraction{}, backend.ErrLaunchFenceOwnership
	}
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return backend.PreparedDocumentExtraction{}, backend.ErrUnsupported
	}
	image, err := images.ImageInspect(ctx, b.imageID)
	if err != nil || ctx.Err() != nil {
		return backend.PreparedDocumentExtraction{}, backend.ErrUnavailable
	}
	if image.ID != b.imageID || image.Size <= 0 || image.Os != "linux" || image.Config == nil || len(image.Config.Volumes) != 0 || len(image.Config.ExposedPorts) != 0 {
		return backend.PreparedDocumentExtraction{}, backend.ErrUnsupported
	}
	allowed := map[string]bool{}
	for _, entry := range documentEnvironment() {
		allowed[entry] = true
	}
	for _, entry := range image.Config.Env {
		if !allowed[entry] {
			return backend.PreparedDocumentExtraction{}, backend.ErrUnsupported
		}
	}
	info, err := b.api.(fenceIdentityClient).Info(ctx)
	if err != nil || ctx.Err() != nil {
		return backend.PreparedDocumentExtraction{}, backend.ErrUnavailable
	}
	if info.ID != identity.EngineID || info.OSType != "linux" {
		return backend.PreparedDocumentExtraction{}, backend.ErrLaunchFenceOwnership
	}
	if info.Driver != "vfs" && info.Driver != "overlay2" && info.Driver != "overlayfs" {
		return backend.PreparedDocumentExtraction{}, backend.ErrUnsupported
	}
	if !requestedLimitsSupported(info, backend.DocumentExtractionResources()) {
		return backend.PreparedDocumentExtraction{}, backend.ErrResourceLimitsUnsupported
	}
	spec.Identity.EngineID, spec.Identity.InstanceID = identity.EngineID, identity.InstanceID
	return backend.PreparedDocumentExtraction{Spec: spec, ImageVirtualSizeBytes: uint64(image.Size), StorageDriver: info.Driver}, nil
}
func (b *DocumentExtractor) CreateDocument(ctx context.Context, prepared backend.PreparedDocumentExtraction) (string, error) {
	fresh, err := b.PrepareDocument(ctx, prepared.Spec)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(fresh, prepared) {
		return "", backend.ErrLaunchFenceOwnership
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	config, host := documentConfiguration(prepared)
	created, err := b.api.ContainerCreate(ctx, config, host, nil, nil, documentName(prepared.Spec.Identity))
	if err != nil || ctx.Err() != nil || !fenceHex(created.ID, 32) {
		return "", backend.ErrUnavailable
	}
	return created.ID, nil
}
func documentOwnedReference(identity backend.DocumentExtractionIdentity, ref string, row container.InspectResponse) bool {
	if row.ContainerJSONBase == nil || !fenceHex(ref, 32) || row.ID != ref || row.Name != "/"+documentName(identity) || row.Config == nil || row.State == nil || fenceMarked(row) {
		return false
	}
	for key, value := range documentLabels(identity) {
		if row.Config.Labels[key] != value {
			return false
		}
	}
	for _, key := range []string{backend.LabelManaged, backend.LabelSandboxID, backend.LabelSessionID, checkLabel, "luci.preview"} {
		if _, present := row.Config.Labels[key]; present {
			return false
		}
	}
	return true
}
func documentOwnedConfiguration(identity backend.DocumentExtractionIdentity, ref string, row container.InspectResponse) bool {
	if !documentOwnedReference(identity, ref, row) || row.HostConfig == nil {
		return false
	}
	cfg, host := row.Config, row.HostConfig
	if len(host.Mounts) != 1 || len(row.Mounts) != 1 {
		return false
	}
	bytes, err := strconv.Atoi(cfg.Labels["luci.document-bytes"])
	if err != nil {
		return false
	}
	ids := strings.Split(cfg.User, ":")
	if len(ids) != 2 {
		return false
	}
	uid, e1 := strconv.Atoi(ids[0])
	gid, e2 := strconv.Atoi(ids[1])
	if e1 != nil || e2 != nil {
		return false
	}
	spec := backend.DocumentExtractionSpec{UserID: uid, GroupID: gid, Identity: identity, ImageID: cfg.Labels["luci.document-image"], OriginalPath: host.Mounts[0].Source, OriginalBytes: bytes, MimeType: cfg.Labels["luci.document-mime"]}
	if !backend.ValidateDocumentExtractionSpec(spec) {
		return false
	}
	expected, limits := documentConfiguration(backend.PreparedDocumentExtraction{Spec: spec})
	if cfg.Image != spec.ImageID || row.Image != spec.ImageID || !reflect.DeepEqual(cfg.Labels, expected.Labels) || !reflect.DeepEqual(cfg.Entrypoint, expected.Entrypoint) || !reflect.DeepEqual(cfg.Cmd, expected.Cmd) || cfg.User != expected.User || cfg.WorkingDir != expected.WorkingDir || !reflect.DeepEqual(cfg.Env, expected.Env) || !cfg.NetworkDisabled || cfg.Tty || cfg.OpenStdin || cfg.StdinOnce || cfg.AttachStdin || !cfg.AttachStdout || cfg.AttachStderr || len(cfg.ExposedPorts) != 0 || len(cfg.Volumes) != 0 || cfg.Healthcheck == nil || !reflect.DeepEqual(cfg.Healthcheck.Test, expected.Healthcheck.Test) {
		return false
	}
	if host.Init == nil || !*host.Init || !host.ReadonlyRootfs || host.AutoRemove || host.Privileged || host.NetworkMode != "none" || host.IpcMode != "none" || host.PidMode != "" || host.UTSMode != "" || host.UsernsMode != "" || host.PublishAllPorts || len(host.PortBindings) != 0 || len(host.Binds) != 0 || len(host.VolumesFrom) != 0 || len(host.Devices) != 0 || len(host.DeviceRequests) != 0 || len(host.Links) != 0 || len(host.ExtraHosts) != 0 || len(host.CapAdd) != 0 || !reflect.DeepEqual(host.CapDrop, limits.CapDrop) || !reflect.DeepEqual(host.SecurityOpt, limits.SecurityOpt) || !reflect.DeepEqual(host.Tmpfs, limits.Tmpfs) || !reflect.DeepEqual(host.Mounts, limits.Mounts) || host.RestartPolicy.Name != container.RestartPolicyDisabled || host.RestartPolicy.MaximumRetryCount != 0 || host.LogConfig.Type != "none" || host.Memory != limits.Memory || host.MemorySwap != limits.MemorySwap || host.NanoCPUs != limits.NanoCPUs || host.PidsLimit == nil || *host.PidsLimit != 32 {
		return false
	}
	return row.Mounts[0].Type == mount.TypeBind && row.Mounts[0].Source == spec.OriginalPath && row.Mounts[0].Destination == "/input/original" && !row.Mounts[0].RW
}
func (b *DocumentExtractor) inspectDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string, configuration bool) (container.InspectResponse, bool, error) {
	if !fenceHex(ref, 32) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err := b.documentIdentity(ctx, identity); err != nil {
		return container.InspectResponse{}, false, err
	}
	row, err := b.api.ContainerInspect(ctx, ref)
	if err != nil {
		if ctx.Err() == nil && stoppedCleanupNotFound(err) {
			if e := b.documentIdentity(ctx, identity); e != nil {
				return container.InspectResponse{}, false, e
			}
			return container.InspectResponse{}, true, nil
		}
		return container.InspectResponse{}, false, backend.ErrUnavailable
	}
	if ctx.Err() != nil || !documentOwnedReference(identity, ref, row) || configuration && !documentOwnedConfiguration(identity, ref, row) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err = b.documentIdentity(ctx, identity); err != nil {
		return container.InspectResponse{}, false, err
	}
	return row, false, nil
}
func (b *DocumentExtractor) StartDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row, absent, err := b.inspectDocument(ctx, identity, ref, true)
	if err != nil {
		return err
	}
	if absent || row.State.Status != "created" || row.State.Running || row.State.Restarting || row.State.Paused || row.State.Dead || row.State.Pid != 0 {
		return backend.ErrNotRunning
	}
	owned, ok := b.attached.Load(ref)
	if !ok {
		return backend.ErrUnavailable
	}
	stream, ok := owned.(*documentResultStream)
	if !ok {
		return backend.ErrLaunchFenceOwnership
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed || stream.identity != identity {
		return backend.ErrUnavailable
	}
	if b.api.ContainerStart(ctx, ref, container.StartOptions{}) != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	return nil
}
func (b *DocumentExtractor) InspectDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string) (backend.DocumentExtractionObservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row, absent, err := b.inspectDocument(ctx, identity, ref, true)
	if err != nil {
		return backend.DocumentExtractionObservation{}, err
	}
	out := backend.DocumentExtractionObservation{Ref: ref}
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
	case row.State.Status == "running" && row.State.Running:
		out.State = "running"
	case row.State.Status == "exited" && !row.State.Running && row.State.Pid == 0:
		out.State = "exited"
		code := row.State.ExitCode
		out.ExitCode = &code
		out.OOMKilled = row.State.OOMKilled
	default:
		return out, backend.ErrUnavailable
	}
	return out, nil
}
func (b *DocumentExtractor) RetireDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, absent, err := b.inspectDocument(ctx, identity, ref, false)
	if err != nil || absent {
		return err
	}
	if err = b.api.ContainerRemove(ctx, ref, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil && (ctx.Err() != nil || !stoppedCleanupNotFound(err)) {
		return backend.ErrUnavailable
	}
	_, absent, err = b.inspectDocument(ctx, identity, ref, false)
	if err != nil {
		return err
	}
	if !absent {
		return backend.ErrUnavailable
	}
	return nil
}
