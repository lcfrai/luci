package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
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

const assemblyLabel = "luci.document-image-assembly"
const assemblyPrefix = "luci-document-assembly-"
const assemblyManifestName = "assembly-manifest.json"

// No route, default image, engine selection, pull, build, import or registration.
// The trusted installation owner must supply a separately reviewed closed spec.
type DocumentImageAssembler struct {
	*Backend
	spec          backend.DocumentImageAssemblySpec
	attachHeaders map[string]string
	attached      sync.Map
	handshakes    sync.Map // exact CID -> owned native connections, including failed handoff.
	guards        sync.Map // immutable source read handles held until positive retirement/fence.
	sourceMu      sync.Mutex
}

var _ backend.DocumentImageAssemblyExecutor = (*DocumentImageAssembler)(nil)

func NewDocumentImageAssembler(b *Backend, spec backend.DocumentImageAssemblySpec, attach DocumentAttachConfiguration) (*DocumentImageAssembler, error) {
	if !backend.ValidateDocumentImageAssemblySpec(spec) || spec.Identity.EngineID != "" || spec.Identity.InstanceID != "" {
		return nil, backend.ErrUnsupported
	}
	// Reuse the pure selected-client header validator, not parser operations.
	validated, err := NewDocumentExtractor(b, spec.BaseImageID, attach)
	if err != nil {
		return nil, err
	}
	return &DocumentImageAssembler{Backend: b, spec: spec, attachHeaders: validated.attachHeaders}, nil
}
func assemblyName(identity backend.DocumentImageAssemblyIdentity) string {
	return assemblyPrefix + identity.AllocationID
}
func assemblyLabels(identity backend.DocumentImageAssemblyIdentity) map[string]string {
	return map[string]string{assemblyLabel: "true", "luci.assembly-protocol": strconv.Itoa(identity.Protocol), "luci.assembly-profile": identity.Profile, backend.LabelInstance: identity.InstanceID, "luci.assembly-engine": identity.EngineID, "luci.assembly-allocation": identity.AllocationID, "luci.assembly-scope": identity.ScopeDigest, "luci.assembly-actor": identity.ActorDigest, "luci.assembly-project": identity.ProjectID, "luci.assembly-source-commit": identity.SourceCommit, "luci.assembly-source": identity.SourceDigest, "luci.assembly-input": identity.InputDigest, "luci.assembly-helper": identity.HelperDigest, "luci.assembly-recipe": identity.RecipeDigest, "luci.assembly-command": identity.CommandDigest}
}
func (b *DocumentImageAssembler) assemblyIdentity(ctx context.Context, identity backend.DocumentImageAssemblyIdentity) error {
	if !backend.ValidateDocumentImageAssemblyIdentity(identity, true) || identity.InstanceID != b.instance {
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
func (b *DocumentImageAssembler) pinnedSpec(spec backend.DocumentImageAssemblySpec) bool {
	copy := spec
	copy.Identity.EngineID, copy.Identity.InstanceID = "", ""
	return backend.ValidateDocumentImageAssemblySpec(spec) && copy == b.spec
}
func assemblyEnvironment() []string {
	return []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/tmp", "TMPDIR=/tmp", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "OMP_THREAD_LIMIT=1", "OMP_NUM_THREADS=1"}
}
func assemblyConfiguration(prepared backend.PreparedDocumentImageAssembly) (*container.Config, *container.HostConfig) {
	spec := prepared.Spec
	labels := assemblyLabels(spec.Identity)
	init, pids := true, int64(64)
	return &container.Config{Image: spec.BaseImageID, Entrypoint: []string{"/tool/luci-document-image-assembler"}, Cmd: []string{"--manifest", "/input/" + assemblyManifestName, "--input-digest", spec.Identity.InputDigest, "--source-digest", spec.Identity.SourceDigest, "--helper-digest", spec.Identity.HelperDigest, "--recipe-digest", spec.Identity.RecipeDigest}, User: "0:0", WorkingDir: "/work", Env: assemblyEnvironment(), NetworkDisabled: true, AttachStdout: true, Labels: labels, Healthcheck: &container.HealthConfig{Test: []string{"NONE"}}}, &container.HostConfig{Init: &init, ReadonlyRootfs: true, RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled}, NetworkMode: "none", IpcMode: "none", CapDrop: []string{"ALL"}, CapAdd: []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "SETUID", "SETGID", "SYS_CHROOT", "MKNOD"}, SecurityOpt: []string{"no-new-privileges:true"}, Tmpfs: map[string]string{"/work": "rw,nosuid,exec,dev,size=2g,mode=0700", "/tmp": "rw,nosuid,nodev,noexec,size=128m,mode=1777"}, Mounts: []mount.Mount{{Type: mount.TypeBind, Source: spec.InputPath, Target: "/input", ReadOnly: true}, {Type: mount.TypeBind, Source: spec.HelperPath, Target: "/tool/luci-document-image-assembler", ReadOnly: true}}, LogConfig: container.LogConfig{Type: "none"}, Resources: container.Resources{Memory: 2 << 30, MemorySwap: 2 << 30, NanoCPUs: 1_000_000_000, PidsLimit: &pids}}
}

// Directory contents are a finite closed set. Every regular input is rehashed
// before Create and Start, and the helper must independently copy+hash all data
// into its owned tmpfs before executing any package script. No symlink/hardlink,
// current-path substitution, extra directory payload or incomplete hash passes.
func hashAssemblyFile(ctx context.Context, path string, bytes int64, digest string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path || hostpath.OneName(path) != nil {
		return false
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() != bytes {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return false
	}
	h := sha256.New()
	var buffer [64 << 10]byte
	var count int64
	for {
		if ctx.Err() != nil {
			return false
		}
		n, e := f.Read(buffer[:])
		count += int64(n)
		if count > bytes {
			return false
		}
		_, _ = h.Write(buffer[:n])
		if e == io.EOF {
			break
		}
		if e != nil || n == 0 {
			return false
		}
	}
	after, err := f.Stat()
	return err == nil && ctx.Err() == nil && os.SameFile(before, after) && after.Size() == bytes && count == bytes && hex.EncodeToString(h.Sum(nil)) == digest
}
func verifyAssemblyInputs(ctx context.Context, spec backend.DocumentImageAssemblySpec) bool {
	resolved, err := filepath.EvalSymlinks(spec.InputPath)
	if err != nil || resolved != spec.InputPath {
		return false
	}
	manifestPath := filepath.Join(spec.InputPath, assemblyManifestName)
	info, err := os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > backend.DocumentImageAssemblyManifestMaxBytes || !hashAssemblyFile(ctx, manifestPath, info.Size(), spec.Identity.InputDigest) {
		return false
	}
	f, err := os.Open(manifestPath)
	if err != nil {
		return false
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, backend.DocumentImageAssemblyManifestMaxBytes+1))
	decoder.DisallowUnknownFields()
	var manifest backend.DocumentImageAssemblyManifest
	if decoder.Decode(&manifest) != nil {
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || !backend.ValidateDocumentImageAssemblyManifest(manifest) || manifest.SourceCommit != spec.Identity.SourceCommit || manifest.SourceDigest != spec.Identity.SourceDigest || manifest.HelperDigest != spec.Identity.HelperDigest || manifest.RecipeDigest != spec.Identity.RecipeDigest {
		return false
	}
	expected := map[string]backend.DocumentImageAssemblyInput{}
	var total int64
	for _, item := range backend.DocumentImageAssemblyInputs(manifest) {
		expected[item.Path] = item
		total += item.Bytes
	}
	if total != spec.InputBytes {
		return false
	}
	seen := 0
	err = walkAssemblySources(ctx, spec.InputPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || ctx.Err() != nil {
			return backend.ErrUnavailable
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return backend.ErrUnsupported
		}
		if path == spec.InputPath {
			return nil
		}
		rel, e := filepath.Rel(spec.InputPath, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			for key := range expected {
				if strings.HasPrefix(key, rel+"/") {
					return nil
				}
			}
			return backend.ErrUnsupported
		}
		if rel == assemblyManifestName {
			return nil
		}
		item, ok := expected[rel]
		if !ok || !hashAssemblyFile(ctx, path, item.Bytes, item.Digest) {
			return backend.ErrUnsupported
		}
		seen++
		return nil
	})
	return err == nil && seen == len(expected) && hashAssemblyFile(ctx, manifestPath, info.Size(), spec.Identity.InputDigest) && hashAssemblyFile(ctx, spec.HelperPath, spec.HelperBytes, spec.Identity.HelperDigest) && staticAssemblyHelper(spec.HelperPath, spec.HelperBytes)
}

// Pinning is not a substitute for checking the fixed native-entry profile
// before the kernel can invoke a loader. Parse only bounded ELF header tables;
// do not import/load/execute the helper or any downloaded native library.
func staticAssemblyHelper(name string, size int64) bool {
	if size < 64 || size > backend.DocumentImageAssemblyHelperMaxBytes {
		return false
	}
	file, err := os.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	var header [64]byte
	if _, err = file.ReadAt(header[:], 0); err != nil || string(header[:7]) != "\x7fELF\x02\x01\x01" || binary.LittleEndian.Uint16(header[16:18]) != 2 || binary.LittleEndian.Uint16(header[18:20]) != 62 || binary.LittleEndian.Uint32(header[20:24]) != 1 || binary.LittleEndian.Uint16(header[52:54]) != 64 || binary.LittleEndian.Uint16(header[54:56]) != 56 {
		return false
	}
	offset := binary.LittleEndian.Uint64(header[32:40])
	count := uint64(binary.LittleEndian.Uint16(header[56:58]))
	entry := binary.LittleEndian.Uint64(header[24:32])
	if count < 1 || count > 128 || offset > uint64(size) || count*56 > uint64(size)-offset {
		return false
	}
	executable := false
	for n := uint64(0); n < count; n++ {
		var program [56]byte
		if _, err = file.ReadAt(program[:], int64(offset+n*56)); err != nil {
			return false
		}
		kind := binary.LittleEndian.Uint32(program[:4])
		flags := binary.LittleEndian.Uint32(program[4:8])
		start := binary.LittleEndian.Uint64(program[8:16])
		vaddr := binary.LittleEndian.Uint64(program[16:24])
		filesz := binary.LittleEndian.Uint64(program[32:40])
		memsz := binary.LittleEndian.Uint64(program[40:48])
		if kind == 2 || kind == 3 || start > uint64(size) || filesz > uint64(size)-start {
			return false
		}
		if kind == 1 && flags&1 != 0 && entry >= vaddr && entry-vaddr < memsz && filesz <= memsz {
			executable = true
		}
	}
	return executable
}

// Unlike filepath.WalkDir/ReadDir(path), this never allocates an entire hostile
// directory before checking the finite entry/depth budget. No link is descended.
func walkAssemblySources(ctx context.Context, root string, visit fs.WalkDirFunc) error {
	entries := 0
	var walk func(string, fs.DirEntry, int) error
	walk = func(name string, entry fs.DirEntry, depth int) error {
		entries++
		if ctx.Err() != nil || entries > 1024 || depth > 16 {
			return backend.ErrUnavailable
		}
		if err := visit(name, entry, nil); err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		dir, err := os.Open(name)
		if err != nil {
			return err
		}
		defer dir.Close()
		for {
			children, readErr := dir.ReadDir(64)
			for _, child := range children {
				if err = walk(filepath.Join(name, child.Name()), child, depth+1); err != nil {
					return err
				}
			}
			if readErr == io.EOF {
				return nil
			}
			if readErr != nil || len(children) == 0 {
				return backend.ErrUnavailable
			}
		}
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return backend.ErrUnsupported
	}
	return walk(root, fs.FileInfoToDirEntry(info), 0)
}

func (b *DocumentImageAssembler) PrepareDocumentImageAssembly(ctx context.Context, spec backend.DocumentImageAssemblySpec) (backend.PreparedDocumentImageAssembly, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !assemblySourceGuardSupported() || !b.pinnedSpec(spec) || !verifyAssemblyInputs(ctx, spec) {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrUnsupported
	}
	if _, ok := b.api.(documentAttachSource); !ok {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrUnsupported
	}
	identity, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return backend.PreparedDocumentImageAssembly{}, err
	}
	if spec.Identity.EngineID != "" && spec.Identity.EngineID != identity.EngineID || spec.Identity.InstanceID != "" && spec.Identity.InstanceID != identity.InstanceID {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrLaunchFenceOwnership
	}
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrUnsupported
	}
	image, err := images.ImageInspect(ctx, spec.BaseImageID)
	if err != nil || ctx.Err() != nil {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrUnavailable
	}
	if image.ID != backend.DocumentImageAssemblyBaseID || image.Size <= 0 || image.Os != "linux" || image.Architecture != "amd64" || image.Config == nil || len(image.Config.Volumes) != 0 || len(image.Config.ExposedPorts) != 0 {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrUnsupported
	}
	// The fixed base's environment must not introduce executable hooks/secrets.
	for _, entry := range image.Config.Env {
		if entry != "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" {
			return backend.PreparedDocumentImageAssembly{}, backend.ErrUnsupported
		}
	}
	info, err := b.api.(fenceIdentityClient).Info(ctx)
	if err != nil || ctx.Err() != nil {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrUnavailable
	}
	if info.ID != identity.EngineID || info.OSType != "linux" {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrLaunchFenceOwnership
	}
	if info.Driver != "vfs" && info.Driver != "overlay2" && info.Driver != "overlayfs" {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrUnsupported
	}
	if !requestedLimitsSupported(info, backend.DocumentImageAssemblyResources()) {
		return backend.PreparedDocumentImageAssembly{}, backend.ErrResourceLimitsUnsupported
	}
	spec.Identity.EngineID, spec.Identity.InstanceID = identity.EngineID, identity.InstanceID
	return backend.PreparedDocumentImageAssembly{Spec: spec, BaseVirtualSizeBytes: uint64(image.Size), StorageDriver: info.Driver}, nil
}
func (b *DocumentImageAssembler) CreateDocumentImageAssembly(ctx context.Context, prepared backend.PreparedDocumentImageAssembly) (string, error) {
	fresh, err := b.PrepareDocumentImageAssembly(ctx, prepared.Spec)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(fresh, prepared) {
		return "", backend.ErrLaunchFenceOwnership
	}
	b.sourceMu.Lock()
	defer b.sourceMu.Unlock()
	if _, exists := b.guards.Load(prepared.Spec.Identity); exists {
		return "", backend.ErrUnavailable
	}
	// A readonly bind does not prevent the host from changing executable bytes.
	// Deny host write/delete through positively owned Windows handles before the
	// final verification, retaining them through uncertain Create/Start outcomes.
	guard, err := lockAssemblySources(ctx, prepared.Spec)
	if guard != nil {
		// The lifecycle's exact allocation lock forbids a concurrent duplicate.
		// Keep a failed partial guard reachable until every close is acknowledged.
		b.guards.Store(prepared.Spec.Identity, guard)
	}
	if err != nil {
		if guard != nil {
			_ = b.releaseAssemblySources(prepared.Spec.Identity)
		}
		return "", err
	}
	if !verifyAssemblyInputs(ctx, prepared.Spec) {
		if b.releaseAssemblySources(prepared.Spec.Identity) != nil {
			return "", backend.ErrUnavailable
		}
		return "", backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	config, host := assemblyConfiguration(prepared)
	created, err := b.api.ContainerCreate(ctx, config, host, nil, nil, assemblyName(prepared.Spec.Identity))
	if err != nil || ctx.Err() != nil || !fenceHex(created.ID, 32) {
		return "", backend.ErrUnavailable
	}
	return created.ID, nil
}
func (b *DocumentImageAssembler) releaseAssemblySources(identity backend.DocumentImageAssemblyIdentity) error {
	var handshakeErr error
	b.handshakes.Range(func(key, value any) bool {
		owner, ok := value.(*assemblyAttachOwner)
		if !ok {
			handshakeErr = backend.ErrLaunchFenceOwnership
			return false
		}
		if owner.identity == identity {
			if owner.Close() != nil {
				handshakeErr = backend.ErrUnavailable
			} else {
				b.handshakes.CompareAndDelete(key, value)
			}
		}
		return true
	})
	if handshakeErr != nil {
		return handshakeErr
	}
	var streamErr error
	b.attached.Range(func(_, value any) bool {
		stream, ok := value.(*assemblyResultStream)
		if !ok {
			streamErr = backend.ErrLaunchFenceOwnership
			return false
		}
		if stream.identity == identity && stream.Close() != nil {
			streamErr = backend.ErrUnavailable
		}
		return true
	})
	if streamErr != nil {
		return streamErr
	}
	if value, ok := b.guards.Load(identity); ok {
		if value.(*assemblySourceGuard).Close() != nil {
			return backend.ErrUnavailable
		}
		b.guards.CompareAndDelete(identity, value)
	}
	return nil
}
func assemblyOwnedReference(identity backend.DocumentImageAssemblyIdentity, ref string, row container.InspectResponse) bool {
	if row.ContainerJSONBase == nil || !fenceHex(ref, 32) || row.ID != ref || row.Name != "/"+assemblyName(identity) || row.Config == nil || row.State == nil || fenceMarked(row) {
		return false
	}
	if !reflect.DeepEqual(row.Config.Labels, assemblyLabels(identity)) {
		return false
	}
	return true // exact labels exclude agent/check/parser/preview and foreign purposes.
}
func (b *DocumentImageAssembler) assemblyOwnedConfiguration(identity backend.DocumentImageAssemblyIdentity, ref string, row container.InspectResponse) bool {
	if !assemblyOwnedReference(identity, ref, row) || row.HostConfig == nil {
		return false
	}
	spec := b.spec
	spec.Identity = identity
	if !b.pinnedSpec(spec) {
		return false
	}
	expected, limits := assemblyConfiguration(backend.PreparedDocumentImageAssembly{Spec: spec})
	cfg, host := row.Config, row.HostConfig
	if cfg.Image != spec.BaseImageID || row.Image != spec.BaseImageID || !reflect.DeepEqual(cfg.Entrypoint, expected.Entrypoint) || !reflect.DeepEqual(cfg.Cmd, expected.Cmd) || cfg.User != expected.User || cfg.WorkingDir != expected.WorkingDir || !reflect.DeepEqual(cfg.Env, expected.Env) || !cfg.NetworkDisabled || cfg.Tty || cfg.OpenStdin || cfg.StdinOnce || cfg.AttachStdin || !cfg.AttachStdout || cfg.AttachStderr || len(cfg.ExposedPorts) != 0 || len(cfg.Volumes) != 0 || cfg.Healthcheck == nil || !reflect.DeepEqual(cfg.Healthcheck.Test, expected.Healthcheck.Test) {
		return false
	}
	if host.Init == nil || !*host.Init || !host.ReadonlyRootfs || host.AutoRemove || host.Privileged || host.NetworkMode != "none" || host.IpcMode != "none" || host.PidMode != "" || host.UTSMode != "" || host.UsernsMode != "" || host.PublishAllPorts || len(host.PortBindings) != 0 || len(host.Binds) != 0 || len(host.VolumesFrom) != 0 || len(host.Devices) != 0 || len(host.DeviceRequests) != 0 || len(host.Links) != 0 || len(host.ExtraHosts) != 0 || !reflect.DeepEqual(host.CapAdd, limits.CapAdd) || !reflect.DeepEqual(host.CapDrop, limits.CapDrop) || !reflect.DeepEqual(host.SecurityOpt, limits.SecurityOpt) || !reflect.DeepEqual(host.Tmpfs, limits.Tmpfs) || !reflect.DeepEqual(host.Mounts, limits.Mounts) || host.RestartPolicy.Name != container.RestartPolicyDisabled || host.RestartPolicy.MaximumRetryCount != 0 || host.LogConfig.Type != "none" || host.Memory != limits.Memory || host.MemorySwap != limits.MemorySwap || host.NanoCPUs != limits.NanoCPUs || host.PidsLimit == nil || *host.PidsLimit != 64 {
		return false
	}
	if len(row.Mounts) != 2 {
		return false
	}
	for _, item := range limits.Mounts {
		found := false
		for _, actual := range row.Mounts {
			if actual.Type == mount.TypeBind && actual.Source == item.Source && actual.Destination == item.Target && !actual.RW {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func (b *DocumentImageAssembler) inspectAssembly(ctx context.Context, identity backend.DocumentImageAssemblyIdentity, ref string, configuration bool) (container.InspectResponse, bool, error) {
	if !fenceHex(ref, 32) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err := b.assemblyIdentity(ctx, identity); err != nil {
		return container.InspectResponse{}, false, err
	}
	row, err := b.api.ContainerInspect(ctx, ref)
	if err != nil {
		if ctx.Err() == nil && stoppedCleanupNotFound(err) {
			if e := b.assemblyIdentity(ctx, identity); e != nil {
				return container.InspectResponse{}, false, e
			}
			return container.InspectResponse{}, true, nil
		}
		return container.InspectResponse{}, false, backend.ErrUnavailable
	}
	if ctx.Err() != nil || !assemblyOwnedReference(identity, ref, row) || configuration && !b.assemblyOwnedConfiguration(identity, ref, row) {
		return container.InspectResponse{}, false, backend.ErrLaunchFenceOwnership
	}
	if err = b.assemblyIdentity(ctx, identity); err != nil {
		return container.InspectResponse{}, false, err
	}
	return row, false, nil
}
func (b *DocumentImageAssembler) StartDocumentImageAssembly(ctx context.Context, identity backend.DocumentImageAssemblyIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row, absent, err := b.inspectAssembly(ctx, identity, ref, true)
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
	stream, ok := owned.(*assemblyResultStream)
	if !ok {
		return backend.ErrLaunchFenceOwnership
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed || stream.identity != identity {
		return backend.ErrUnavailable
	}
	spec := b.spec
	spec.Identity = identity
	if !b.pinnedSpec(spec) || !verifyAssemblyInputs(ctx, spec) {
		return backend.ErrUnsupported
	}
	if b.api.ContainerStart(ctx, ref, container.StartOptions{}) != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	return nil
}
func (b *DocumentImageAssembler) InspectDocumentImageAssembly(ctx context.Context, identity backend.DocumentImageAssemblyIdentity, ref string) (backend.DocumentImageAssemblyObservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row, absent, err := b.inspectAssembly(ctx, identity, ref, true)
	if err != nil {
		return backend.DocumentImageAssemblyObservation{}, err
	}
	out := backend.DocumentImageAssemblyObservation{Ref: ref}
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
func (b *DocumentImageAssembler) RetireDocumentImageAssembly(ctx context.Context, identity backend.DocumentImageAssemblyIdentity, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, absent, err := b.inspectAssembly(ctx, identity, ref, false)
	if err != nil || absent {
		if err == nil && absent {
			return b.releaseAssemblySources(identity)
		}
		return err
	}
	if err = b.api.ContainerRemove(ctx, ref, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil && (ctx.Err() != nil || !stoppedCleanupNotFound(err)) {
		return backend.ErrUnavailable
	}
	_, absent, err = b.inspectAssembly(ctx, identity, ref, false)
	if err != nil {
		return err
	}
	if !absent {
		return backend.ErrUnavailable
	}
	return b.releaseAssemblySources(identity)
}
