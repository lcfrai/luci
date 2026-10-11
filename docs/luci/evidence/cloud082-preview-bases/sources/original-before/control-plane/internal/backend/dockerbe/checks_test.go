package dockerbe

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"io"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// This models acknowledged objects and protocol errors, not a real Docker
// daemon. No test shells out, opens Docker, or uses the source checkout as data.
type checkProbe struct {
	info                                               system.Info
	image                                              image.InspectResponse
	endpoint                                           string
	rows                                               map[string]container.InspectResponse
	creates, starts, removes, inspects                 []string
	removeOptions                                      []container.RemoveOptions
	configs                                            []*container.Config
	hosts                                              []*container.HostConfig
	infoCalls, imageCalls, logCalls                    int
	infoHook                                           func(int, *system.Info)
	inspectHook                                        func(string, container.InspectResponse) (container.InspectResponse, error)
	removeHook                                         func(string)
	createError, startError, inspectError, removeError error
	loseCreateACK, loseStartACK, removeNoEffect        bool
	reader                                             io.ReadCloser
	logOptions                                         container.LogsOptions
}

type checkMissing struct{}

func (checkMissing) Error() string { return "controlled missing object" }
func (checkMissing) NotFound()     {}

func copyCheckRow(row container.InspectResponse) container.InspectResponse {
	raw, _ := json.Marshal(row)
	var result container.InspectResponse
	_ = json.Unmarshal(raw, &result)
	return result
}
func newCheckProbe(t *testing.T) (*Backend, *checkProbe, backend.CheckSpec) {
	t.Helper()
	f := &checkProbe{
		info:     system.Info{ID: "check-engine", OSType: "linux", Driver: "vfs", MemoryLimit: true, SwapLimit: true, CPUCfsPeriod: true, CPUCfsQuota: true, PidsLimit: true},
		image:    image.InspectResponse{ID: "sha256:" + strings.Repeat("a", 64), Size: 2048, Os: "linux", Config: &dockerspec.DockerOCIImageConfig{}},
		endpoint: "unix:///fixture-only/docker.sock", rows: map[string]container.InspectResponse{},
	}
	if runtime.GOOS == "windows" {
		f.endpoint = "npipe:////./pipe/docker_engine"
	}
	b, err := New(Config{Instance: "check-instance", ResourceImage: "fixture:image", Client: f})
	if err != nil {
		t.Fatal(err)
	}
	spec := backend.CheckSpec{
		Identity:   backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, AllocationID: "allocation-one", CheckID: "check-one", CommandIndex: 0, ScopeDigest: strings.Repeat("b", 64), ProjectID: "project-one"},
		Executable: "node", Args: []string{"verify.mjs", "final"}, SourceDirectory: t.TempDir(), SourceDigest: strings.Repeat("c", 64),
		Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}, UserID: 1000, GroupID: 1000,
	}
	spec.SourceDirectory, err = filepath.EvalSymlinks(spec.SourceDirectory)
	if err != nil {
		t.Fatal(err)
	}
	spec.Identity.CommandDigest = backend.CheckCommandDigest(spec)
	return b, f, spec
}
func (f *checkProbe) DaemonHost() string                       { return f.endpoint }
func (f *checkProbe) Ping(context.Context) (types.Ping, error) { return types.Ping{}, nil }
func (f *checkProbe) Close() error                             { return nil }
func (f *checkProbe) Info(context.Context) (system.Info, error) {
	f.infoCalls++
	value := f.info
	if f.infoHook != nil {
		f.infoHook(f.infoCalls, &value)
	}
	return value, nil
}
func (f *checkProbe) ImageInspect(context.Context, string, ...client.ImageInspectOption) (image.InspectResponse, error) {
	f.imageCalls++
	return f.image, nil
}
func (f *checkProbe) ContainerCreate(_ context.Context, cfg *container.Config, host *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, name string) (container.CreateResponse, error) {
	f.creates = append(f.creates, name)
	if f.createError != nil {
		return container.CreateResponse{}, f.createError
	}
	for _, row := range f.rows {
		if row.Name == "/"+name {
			return container.CreateResponse{}, errors.New("controlled name conflict")
		}
	}
	ref := fmt.Sprintf("%064x", len(f.creates))
	path := "/"
	if len(cfg.Entrypoint) > 0 {
		path = cfg.Entrypoint[0]
	} else if len(cfg.Cmd) > 0 {
		path = cfg.Cmd[0]
	}
	row := container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: ref, Name: "/" + name, Image: cfg.Image, Path: path, HostConfig: host, State: &container.State{Status: "created"}}, Config: cfg}
	for _, m := range host.Mounts {
		row.Mounts = append(row.Mounts, container.MountPoint{Type: m.Type, Source: m.Source, Destination: m.Target, RW: !m.ReadOnly})
	}
	row = copyCheckRow(row)
	f.rows[ref] = row
	f.configs = append(f.configs, row.Config)
	f.hosts = append(f.hosts, row.HostConfig)
	if f.loseCreateACK {
		f.loseCreateACK = false
		return container.CreateResponse{}, context.DeadlineExceeded
	}
	return container.CreateResponse{ID: ref}, nil
}
func (f *checkProbe) ContainerStart(_ context.Context, ref string, _ container.StartOptions) error {
	f.starts = append(f.starts, ref)
	if f.startError != nil {
		return f.startError
	}
	row, ok := f.rows[ref]
	if !ok {
		return checkMissing{}
	}
	if row.Path == "/" {
		return errors.New("inert directory cannot execute")
	}
	row.State.Status, row.State.Running, row.State.Pid = "running", true, 123
	f.rows[ref] = row
	if f.loseStartACK {
		f.loseStartACK = false
		return context.DeadlineExceeded
	}
	return nil
}
func (f *checkProbe) ContainerInspect(_ context.Context, ref string) (container.InspectResponse, error) {
	f.inspects = append(f.inspects, ref)
	if f.inspectError != nil {
		return container.InspectResponse{}, f.inspectError
	}
	for id, row := range f.rows {
		if id == ref || row.Name == "/"+ref {
			row = copyCheckRow(row)
			if f.inspectHook != nil {
				return f.inspectHook(ref, row)
			}
			return row, nil
		}
	}
	return container.InspectResponse{}, checkMissing{}
}
func (f *checkProbe) ContainerList(_ context.Context, options container.ListOptions) ([]container.Summary, error) {
	var result []container.Summary
	for ref, row := range f.rows {
		matches := true
		for _, filter := range options.Filters.Get("label") {
			key, value, equal := strings.Cut(filter, "=")
			actual, exists := row.Config.Labels[key]
			if !exists || equal && actual != value {
				matches = false
			}
		}
		if matches {
			result = append(result, container.Summary{ID: ref, Names: []string{row.Name}, State: row.State.Status, Labels: row.Config.Labels})
		}
		if options.Limit > 0 && len(result) >= options.Limit {
			break
		}
	}
	return result, nil
}
func (f *checkProbe) ContainerRemove(_ context.Context, ref string, options container.RemoveOptions) error {
	f.removes = append(f.removes, ref)
	f.removeOptions = append(f.removeOptions, options)
	if f.removeHook != nil {
		f.removeHook(ref)
	}
	if f.removeError != nil {
		return f.removeError
	}
	if _, exists := f.rows[ref]; !exists {
		return checkMissing{}
	}
	if !f.removeNoEffect {
		delete(f.rows, ref)
	}
	return nil
}
func (f *checkProbe) ContainerLogs(_ context.Context, ref string, options container.LogsOptions) (io.ReadCloser, error) {
	f.logCalls++
	f.logOptions = options
	if _, exists := f.rows[ref]; !exists {
		return nil, checkMissing{}
	}
	return f.reader, nil
}
func prepareAndCreateCheck(t *testing.T, b *Backend, spec backend.CheckSpec) (backend.PreparedCheck, string) {
	t.Helper()
	p, err := b.PrepareCheck(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := b.CreateCheck(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return p, ref
}

func TestCheckPreparationRefusesChangedCapabilityOrIdentityBeforeAllocation(t *testing.T) {
	cases := map[string]func(*checkProbe, *backend.CheckSpec){
		"no-memory":       func(f *checkProbe, _ *backend.CheckSpec) { f.info.MemoryLimit = false },
		"no-swap":         func(f *checkProbe, _ *backend.CheckSpec) { f.info.SwapLimit = false },
		"no-cpu-period":   func(f *checkProbe, _ *backend.CheckSpec) { f.info.CPUCfsPeriod = false },
		"no-cpu-quota":    func(f *checkProbe, _ *backend.CheckSpec) { f.info.CPUCfsQuota = false },
		"no-pids":         func(f *checkProbe, _ *backend.CheckSpec) { f.info.PidsLimit = false },
		"remote-endpoint": func(f *checkProbe, _ *backend.CheckSpec) { f.endpoint = "tcp://127.0.0.1:2375" },
		"windows-engine":  func(f *checkProbe, _ *backend.CheckSpec) { f.info.OSType = "windows" },
		"image-volume":    func(f *checkProbe, _ *backend.CheckSpec) { f.image.Config.Volumes = map[string]struct{}{"/hidden": {}} },
		"mutable-image":   func(f *checkProbe, _ *backend.CheckSpec) { f.image.ID = "fixture:latest" },
		"engine-switch": func(f *checkProbe, _ *backend.CheckSpec) {
			f.infoHook = func(n int, info *system.Info) {
				if n >= 2 {
					info.ID = "other-engine"
				}
			}
		},
		"argv-amendment":   func(_ *checkProbe, s *backend.CheckSpec) { s.Args = []string{"different.mjs"} },
		"source-amendment": func(_ *checkProbe, s *backend.CheckSpec) { s.SourceDigest = strings.Repeat("d", 64) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b, f, spec := newCheckProbe(t)
			mutate(f, &spec)
			_, err := b.PrepareCheck(context.Background(), spec)
			if err == nil || len(f.creates)+len(f.starts)+len(f.removes) != 0 {
				t.Fatalf("refusal had effects: %v %+v", err, f.creates)
			}
			if strings.HasPrefix(name, "no-") && !errors.Is(err, backend.ErrResourceLimitsUnsupported) {
				t.Fatalf("wrong capability error: %v", err)
			}
		})
	}
}

func TestCheckPreparedSnapshotCannotAllocateAfterChange(t *testing.T) {
	for _, name := range []string{"engine", "image", "driver", "memory-capability", "args"} {
		t.Run(name, func(t *testing.T) {
			b, f, spec := newCheckProbe(t)
			p, err := b.PrepareCheck(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "engine":
				f.info.ID = "replacement"
			case "image":
				f.image.ID = "sha256:" + strings.Repeat("f", 64)
			case "driver":
				f.info.Driver = "overlay2"
			case "memory-capability":
				f.info.MemoryLimit = false
			case "args":
				p.Spec.Args[0] = "changed.mjs"
			}
			if _, err = b.CreateCheck(context.Background(), p); err == nil || len(f.creates) != 0 {
				t.Fatal("stale preparation allocated", err, f.creates)
			}
		})
	}
}

func TestCheckCreatedLimitMismatchNeverStartsButOwnedObjectCanBeRemoved(t *testing.T) {
	cases := map[string]func(*container.InspectResponse){
		"memory-discarded":   func(r *container.InspectResponse) { r.HostConfig.Memory = 0 },
		"swap-unbounded":     func(r *container.InspectResponse) { r.HostConfig.MemorySwap = -1 },
		"cpu-discarded":      func(r *container.InspectResponse) { r.HostConfig.NanoCPUs = 0 },
		"pid-discarded":      func(r *container.InspectResponse) { r.HostConfig.PidsLimit = nil },
		"readonly-discarded": func(r *container.InspectResponse) { r.HostConfig.ReadonlyRootfs = false },
		"cap-drop-discarded": func(r *container.InspectResponse) { r.HostConfig.CapDrop = nil },
		"cap-drop-changed":   func(r *container.InspectResponse) { r.HostConfig.CapDrop = []string{"NET_RAW"} },
		"cap-drop-extra":     func(r *container.InspectResponse) { r.HostConfig.CapDrop = []string{"ALL", "NET_RAW"} },
		"cap-add-changed":    func(r *container.InspectResponse) { r.HostConfig.CapAdd = []string{"NET_RAW"} },
		"security-discarded": func(r *container.InspectResponse) { r.HostConfig.SecurityOpt = nil },
		"security-changed":   func(r *container.InspectResponse) { r.HostConfig.SecurityOpt = []string{"no-new-privileges:false"} },
		"argv-changed":       func(r *container.InspectResponse) { r.Config.Cmd = []string{"foreign.mjs"} },
		"extra-mount": func(r *container.InspectResponse) {
			r.HostConfig.Mounts = append(r.HostConfig.Mounts, mount.Mount{Type: mount.TypeBind, Source: "/foreign", Target: "/foreign"})
		},
		"source-replaced": func(r *container.InspectResponse) {
			r.HostConfig.Mounts[0].Source = "/different"
			r.Mounts[0].Source = "/different"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b, f, spec := newCheckProbe(t)
			p, ref := prepareAndCreateCheck(t, b, spec)
			row := f.rows[ref]
			mutate(&row)
			f.rows[ref] = row
			if err := b.StartCheck(context.Background(), p.Spec.Identity, ref); !errors.Is(err, backend.ErrLaunchFenceOwnership) || len(f.starts) != 0 {
				t.Fatal("invalid configuration started", err)
			}
			if _, err := b.InspectCheck(context.Background(), p.Spec.Identity, ref); !errors.Is(err, backend.ErrLaunchFenceOwnership) {
				t.Fatal("invalid configuration became verified execution evidence", err)
			}
			if _, err := b.CheckLogs(context.Background(), p.Spec.Identity, ref, 128); !errors.Is(err, backend.ErrLaunchFenceOwnership) || f.logCalls != 0 {
				t.Fatal("invalid configuration became verified command output", err)
			}
			if err := b.RemoveCheck(context.Background(), p.Spec.Identity, ref, false); err != nil {
				t.Fatal("exact owned invalid object could not be cleaned", err)
			}
			if !reflect.DeepEqual(f.removes, []string{ref}) || f.removeOptions[0].Force || len(f.rows) != 0 {
				t.Fatal("incorrect cleanup", f.removes)
			}
		})
	}
}

func TestCheckExactOwnershipAndEngineProtectStartLogsAndRemoval(t *testing.T) {
	for _, kind := range []string{"ref", "name", "allocation", "scope", "engine"} {
		t.Run(kind, func(t *testing.T) {
			b, f, spec := newCheckProbe(t)
			p, ref := prepareAndCreateCheck(t, b, spec)
			row := f.rows[ref]
			switch kind {
			case "ref":
				row.ID = strings.Repeat("f", 64)
			case "name":
				row.Name = "/foreign"
			case "allocation":
				row.Config.Labels[checkAllocationLabel] = "foreign"
			case "scope":
				row.Config.Labels[checkScopeLabel] = strings.Repeat("f", 64)
			case "engine":
				f.info.ID = "different-engine"
			}
			f.rows[ref] = row
			if err := b.StartCheck(context.Background(), p.Spec.Identity, ref); err == nil {
				t.Fatal("foreign start permitted")
			}
			if _, err := b.CheckLogs(context.Background(), p.Spec.Identity, ref, 128); err == nil {
				t.Fatal("foreign logs permitted")
			}
			if err := b.RemoveCheck(context.Background(), p.Spec.Identity, ref, true); err == nil {
				t.Fatal("foreign remove permitted")
			}
			if len(f.starts)+len(f.removes)+f.logCalls != 0 {
				t.Fatal("ownership refusal emitted an effect")
			}
		})
	}
}

func TestCheckRemovalNeverFollowsReplacementNameAndRequiresAbsence(t *testing.T) {
	b, f, spec := newCheckProbe(t)
	p, ref := prepareAndCreateCheck(t, b, spec)
	replacement := strings.Repeat("f", 64)
	f.removeHook = func(id string) { row := copyCheckRow(f.rows[id]); row.ID = replacement; f.rows[replacement] = row }
	if err := b.RemoveCheck(context.Background(), p.Spec.Identity, ref, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.rows[replacement]; !ok || !reflect.DeepEqual(f.removes, []string{ref}) {
		t.Fatal("mutable name replacement was removed")
	}
	if err := b.RemoveCheck(context.Background(), p.Spec.Identity, checkName(p.Spec.Identity), true); err == nil || len(f.removes) != 1 {
		t.Fatal("name was accepted as immutable reference")
	}
	for _, mode := range []string{"remove-error", "success-but-still-present", "mixed-notfound"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newCheckProbe(t)
			p, ref := prepareAndCreateCheck(t, b, spec)
			switch mode {
			case "remove-error":
				f.removeError = errors.New("private daemon diagnostic")
			case "success-but-still-present":
				f.removeNoEffect = true
			case "mixed-notfound":
				f.inspectError = errors.Join(checkMissing{}, errors.New("transport failed"))
			}
			if err := b.RemoveCheck(context.Background(), p.Spec.Identity, ref, true); err == nil {
				t.Fatal("uncertainty became successful cleanup")
			}
			if _, ok := f.rows[ref]; !ok {
				t.Fatal("controlled uncertain fixture unexpectedly disappeared")
			}
		})
	}
}

func TestCheckAmbiguousCreateAndStartNeverRetryOrCleanImplicitly(t *testing.T) {
	b, f, spec := newCheckProbe(t)
	p, err := b.PrepareCheck(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	f.loseCreateACK = true
	ref, err := b.CreateCheck(context.Background(), p)
	if ref != "" || !errors.Is(err, backend.ErrUnavailable) || len(f.creates) != 1 || len(f.rows) != 1 || len(f.starts)+len(f.removes) != 0 {
		t.Fatal("lost Create ACK was adopted/replayed/cleaned", ref, err)
	}
	b, f, spec = newCheckProbe(t)
	p, ref = prepareAndCreateCheck(t, b, spec)
	f.loseStartACK = true
	if err = b.StartCheck(context.Background(), p.Spec.Identity, ref); !errors.Is(err, backend.ErrUnavailable) || len(f.starts) != 1 || len(f.removes) != 0 || !f.rows[ref].State.Running {
		t.Fatal("lost Start ACK was converted to absence", err)
	}
}

func TestCheckFenceLostACKRequiresNewAcknowledgedMarker(t *testing.T) {
	b, f, spec := newCheckProbe(t)
	p, original := prepareAndCreateCheck(t, b, spec)
	req := backend.CheckFenceRequest{Identity: p.Spec.Identity, IntentID: strings.Repeat("d", 32), ExpectedOriginalRef: original}
	f.loseCreateACK = true
	if proof, err := b.FenceCheckLaunch(context.Background(), req); !errors.Is(err, backend.ErrLaunchFencePending) || proof.FenceRef != "" {
		t.Fatal("lost fence ACK manufactured proof", proof, err)
	}
	if len(f.rows) != 1 || len(f.removes) != 1 || f.removes[0] != original || !f.removeOptions[0].Force {
		t.Fatal("wrong original cleanup")
	}
	var unacknowledged string
	for ref := range f.rows {
		unacknowledged = ref
	}
	proof, err := b.FenceCheckLaunch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if proof.FenceRef == unacknowledged || len(f.removes) != 2 || f.removes[1] != unacknowledged || f.removeOptions[1].Force || len(f.starts) != 0 {
		t.Fatal("unacknowledged marker adopted or started", proof, f.removes)
	}
	if err = b.VerifyCheckLaunchFence(context.Background(), req, proof); err != nil {
		t.Fatal(err)
	}
	if _, err = b.CreateCheck(context.Background(), p); err == nil {
		t.Fatal("late Create acquired fenced name")
	}
	if err = b.StartCheck(context.Background(), p.Spec.Identity, original); err == nil {
		t.Fatal("late immutable original Start succeeded")
	}
	if err = b.RemoveCheck(context.Background(), p.Spec.Identity, proof.FenceRef, true); err == nil {
		t.Fatal("ordinary check cleanup removed permanent fence")
	}
	if _, ok := f.rows[proof.FenceRef]; !ok {
		t.Fatal("fence disappeared")
	}
	for _, kind := range []string{"identity", "ref", "config", "time"} {
		t.Run(kind, func(t *testing.T) {
			bad := proof
			switch kind {
			case "identity":
				bad.Identity.CheckID = "foreign"
			case "ref":
				bad.FenceRef = original
			case "config":
				bad.ConfigDigest = strings.Repeat("f", 64)
			case "time":
				bad.AcknowledgedAt = "not-a-time"
			}
			before := len(f.removes)
			if err := b.VerifyCheckLaunchFence(context.Background(), req, bad); err == nil || len(f.removes) != before {
				t.Fatal("changed proof authorized effects", err)
			}
		})
	}
}

func TestCheckFenceRejectsMalformedInspectAndDuplicateAttempt(t *testing.T) {
	for _, kind := range []string{"nil-original", "nil-fresh", "duplicate-attempt"} {
		t.Run(kind, func(t *testing.T) {
			b, f, spec := newCheckProbe(t)
			p, ref := prepareAndCreateCheck(t, b, spec)
			req := backend.CheckFenceRequest{Identity: p.Spec.Identity, IntentID: strings.Repeat("d", 32), ExpectedOriginalRef: ref}
			if kind == "duplicate-attempt" {
				row := copyCheckRow(f.rows[ref])
				row.ID = strings.Repeat("f", 64)
				row.Name = "/renamed-old-attempt"
				f.rows[row.ID] = row
			} else {
				f.inspectHook = func(id string, row container.InspectResponse) (container.InspectResponse, error) {
					if kind == "nil-original" && id == checkName(p.Spec.Identity) || kind == "nil-fresh" && id == ref {
						return container.InspectResponse{}, nil
					}
					return row, nil
				}
			}
			if proof, err := b.FenceCheckLaunch(context.Background(), req); err == nil || proof.FenceRef != "" {
				t.Fatal("malformed/duplicate inventory authorized recovery", proof, err)
			}
			if len(f.creates) != 1 || len(f.removes)+len(f.starts) != 0 {
				t.Fatal("recovery emitted effects before ownership was proved")
			}
		})
	}
}

func checkLogFrame(stream byte, payload []byte) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}

type observedLogReader struct {
	*bytes.Reader
	closed bool
}

func (r *observedLogReader) Close() error { r.closed = true; return nil }
func TestCheckLogsPreserveStreamsAndBoundHugeDeclaredFrames(t *testing.T) {
	b, f, spec := newCheckProbe(t)
	p, ref := prepareAndCreateCheck(t, b, spec)
	r := &observedLogReader{Reader: bytes.NewReader(append(checkLogFrame(1, []byte("out\n")), checkLogFrame(2, []byte("err\n"))...))}
	f.reader = r
	value, err := b.CheckLogs(context.Background(), p.Spec.Identity, ref, 8)
	if err != nil || value.Text != "out\nerr\n" || value.Truncated || !r.closed || f.logOptions.Follow || !f.logOptions.ShowStdout || !f.logOptions.ShowStderr {
		t.Fatal("wrong bounded output", value, err)
	}
	for _, limit := range []int{0, maxCheckLogBytes + 1} {
		if _, err := b.CheckLogs(context.Background(), p.Spec.Identity, ref, limit); !errors.Is(err, backend.ErrUnsupported) || f.logCalls != 1 {
			t.Fatal("invalid output bound issued logs RPC", err)
		}
	}
	header := make([]byte, 8)
	header[0] = 1
	binary.BigEndian.PutUint32(header[4:], ^uint32(0))
	input := append(header, []byte("123456789")...)
	value, err = readCheckLogs(context.Background(), bytes.NewReader(input), 8)
	if err != nil || value.Text != "12345678" || !value.Truncated {
		t.Fatal("declared 4GiB frame was not capped", value, err)
	}
	for name, data := range map[string][]byte{"short-header": {1, 0, 0}, "short-body": checkLogFrame(1, []byte("abc"))[:10], "stdin-frame": checkLogFrame(0, nil), "reserved-header": {1, 1, 0, 0, 0, 0, 0, 0}, "empty-frame-flood": bytes.Repeat(checkLogFrame(1, nil), 65536)} {
		t.Run(name, func(t *testing.T) {
			if _, err := readCheckLogs(context.Background(), bytes.NewReader(data), 8); !errors.Is(err, backend.ErrUnavailable) {
				t.Fatal("malformed/unbounded framing accepted", err)
			}
		})
	}
}

type blockingCheckReader struct {
	started   chan struct{}
	closed    chan struct{}
	once      sync.Once
	closeOnce sync.Once
}

func (r *blockingCheckReader) Read([]byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.closed
	return 0, io.ErrClosedPipe
}
func (r *blockingCheckReader) Close() error { r.closeOnce.Do(func() { close(r.closed) }); return nil }
func TestCheckLogsCancellationClosesOwnedReader(t *testing.T) {
	b, f, spec := newCheckProbe(t)
	p, ref := prepareAndCreateCheck(t, b, spec)
	r := &blockingCheckReader{started: make(chan struct{}), closed: make(chan struct{})}
	f.reader = r
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer r.Close()
	result := make(chan error, 1)
	go func() { _, err := b.CheckLogs(ctx, p.Spec.Identity, ref, 128); result <- err }()
	select {
	case <-r.started:
	case <-time.After(3 * time.Second):
		t.Fatal("log reader was not reached")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, backend.ErrUnavailable) {
			t.Fatal("cancelled log read became success", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not close the owned reader")
	}
}
