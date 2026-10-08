package dockerbe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type fenceMissing struct{}

func (fenceMissing) Error() string { return "missing" }
func (fenceMissing) NotFound()     {}

type fenceDockerFixture struct {
	mu                            sync.Mutex
	rows                          map[string]container.InspectResponse
	engine, endpoint              string
	next                          int
	creates, starts               int
	removes                       []string
	force                         []bool
	lostAck, invisibleReservation bool
	inspectErr, listErr           error
	beforeRemove                  func(string)
}

func newFenceFixture(t *testing.T) (*Backend, *fenceDockerFixture, backend.LaunchFenceRequest) {
	t.Helper()
	f := &fenceDockerFixture{rows: map[string]container.InspectResponse{}, engine: "engine-owned", endpoint: "unix:///var/run/docker.sock"}
	b, err := New(Config{Instance: "owned-instance", Client: f})
	if err != nil {
		t.Fatal(err)
	}
	return b, f, backend.LaunchFenceRequest{EngineID: f.engine, InstanceID: "owned-instance", SandboxID: "s-owned", SessionID: "s-owned",
		IntentID: strings.Repeat("a", 32), ScopeDigest: strings.Repeat("b", 64), OperationDigest: strings.Repeat("c", 64)}
}
func (f *fenceDockerFixture) DaemonHost() string { return f.endpoint }
func (f *fenceDockerFixture) Info(context.Context) (system.Info, error) {
	return system.Info{ID: f.engine, OSType: "linux"}, nil
}
func (f *fenceDockerFixture) Ping(context.Context) (types.Ping, error) { return types.Ping{}, nil }
func (f *fenceDockerFixture) Close() error                             { return nil }
func cloneFenceInspect(row container.InspectResponse) container.InspectResponse {
	data, _ := json.Marshal(row)
	var copy container.InspectResponse
	_ = json.Unmarshal(data, &copy)
	return copy
}
func (f *fenceDockerFixture) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inspectErr != nil {
		return container.InspectResponse{}, f.inspectErr
	}
	for ref, row := range f.rows {
		if ref == id || row.Name == "/"+id {
			return cloneFenceInspect(row), nil
		}
	}
	return container.InspectResponse{}, fenceMissing{}
}
func (f *fenceDockerFixture) ContainerCreate(_ context.Context, cfg *container.Config, host *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, name string) (container.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	if f.invisibleReservation {
		return container.CreateResponse{}, errors.New("name reserved by a pending invisible Create")
	}
	for _, row := range f.rows {
		if row.Name == "/"+name {
			return container.CreateResponse{}, errors.New("name conflict")
		}
	}
	f.next++
	id := fmt.Sprintf("%064x", f.next)
	f.rows[id] = cloneFenceInspect(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{ID: id, Name: "/" + name, Path: cfg.Cmd[0], HostConfig: host, State: &container.State{Status: "created"}},
		Config:            cfg,
	})
	if f.lostAck {
		f.lostAck = false
		return container.CreateResponse{}, context.DeadlineExceeded
	}
	return container.CreateResponse{ID: id}, nil
}
func (f *fenceDockerFixture) ContainerStart(_ context.Context, id string, _ container.StartOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	row, ok := f.rows[id]
	if !ok {
		return fenceMissing{}
	}
	if row.Path == "/" {
		return errors.New("directory cannot execute")
	}
	row.State.Status, row.State.Running = "running", true
	f.rows[id] = row
	return nil
}
func (f *fenceDockerFixture) ContainerList(_ context.Context, opts container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []container.Summary
	for id, row := range f.rows {
		match := true
		for _, filter := range opts.Filters.Get("label") {
			parts := strings.SplitN(filter, "=", 2)
			v, ok := row.Config.Labels[parts[0]]
			if !ok || len(parts) == 2 && v != parts[1] {
				match = false
			}
		}
		if match {
			out = append(out, container.Summary{ID: id, Names: []string{row.Name}, State: row.State.Status, Labels: row.Config.Labels})
		}
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}
func (f *fenceDockerFixture) ContainerRemove(_ context.Context, id string, opts container.RemoveOptions) error {
	if f.beforeRemove != nil {
		f.beforeRemove(id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removes, f.force = append(f.removes, id), append(f.force, opts.Force)
	if _, ok := f.rows[id]; !ok {
		return fenceMissing{}
	}
	delete(f.rows, id)
	return nil
}
func (f *fenceDockerFixture) original(req backend.LaunchFenceRequest) string {
	id := strings.Repeat("f", 64)
	f.rows[id] = container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: id, Name: "/" + namePrefix + req.SandboxID, Path: "/sandboxd", State: &container.State{Status: "created"}},
		Config: &container.Config{Labels: map[string]string{backend.LabelManaged: "true", backend.LabelInstance: req.InstanceID,
			backend.LabelSandboxID: req.SandboxID, backend.LabelSessionID: req.SessionID}}}
	return id
}

func TestLaunchFenceAcknowledgesInertPermanentNameAndPreservesOutputBoundary(t *testing.T) {
	b, f, req := newFenceFixture(t)
	original := f.original(req)
	req.ExpectedOriginalRef = original
	proof, err := b.FenceLaunch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if proof.FenceRef == original || proof.EngineID != req.EngineID || len(f.removes) != 1 || f.removes[0] != original || !f.force[0] || f.starts != 0 {
		t.Fatalf("wrong mutation or receipt: %+v %+v", proof, f.removes)
	}
	if err = b.VerifyLaunchFence(context.Background(), req, proof); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Get(context.Background(), req.SandboxID); !errors.Is(err, backend.ErrLaunchFenced) {
		t.Fatal("fence became a worker", err)
	}
	if err = b.Destroy(context.Background(), req.SandboxID); !errors.Is(err, backend.ErrLaunchFenced) || len(f.removes) != 1 {
		t.Fatal("ordinary Stop removed fence", err)
	}
	if rows, e := b.List(context.Background()); e != nil || len(rows) != 0 {
		t.Fatal("fence leaked into reaper workers", rows, e)
	}
	cfg, host := fenceConfiguration(req)
	if _, err = f.ContainerCreate(context.Background(), cfg, host, nil, nil, namePrefix+req.SandboxID); err == nil {
		t.Fatal("late Create won fenced name")
	}
	if err = f.ContainerStart(context.Background(), original, container.StartOptions{}); err == nil {
		t.Fatal("removed original started")
	}
	if err = f.ContainerStart(context.Background(), proof.FenceRef, container.StartOptions{}); err == nil {
		t.Fatal("scratch directory executed")
	}
}

func TestLaunchFenceLostAcknowledgmentRequiresDifferentAcknowledgedObject(t *testing.T) {
	b, f, req := newFenceFixture(t)
	f.lostAck = true
	if proof, err := b.FenceLaunch(context.Background(), req); !errors.Is(err, backend.ErrLaunchFencePending) || proof.FenceRef != "" {
		t.Fatal("lost acknowledgment became proof", proof, err)
	}
	unacknowledged := fmt.Sprintf("%064x", 1)
	proof, err := b.FenceLaunch(context.Background(), req)
	if err != nil || proof.FenceRef == unacknowledged || len(f.removes) != 1 || f.removes[0] != unacknowledged || f.force[0] {
		t.Fatal("retry adopted unsafe object", proof, f.removes, err)
	}
	// A late daemon cleanup for the old failed Create cannot erase the new ID.
	_ = f.ContainerRemove(context.Background(), unacknowledged, container.RemoveOptions{Force: true})
	if err = b.VerifyLaunchFence(context.Background(), req, proof); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchFenceInvisibleReservationRemainsPendingWithoutDestruction(t *testing.T) {
	b, f, req := newFenceFixture(t)
	f.invisibleReservation = true
	if proof, err := b.FenceLaunch(context.Background(), req); !errors.Is(err, backend.ErrLaunchFencePending) || proof.FenceRef != "" || len(f.removes) != 0 {
		t.Fatal("invisible reservation was treated as absence proof", proof, err)
	}
	f.invisibleReservation = false
	original := f.original(req)
	proof, err := b.FenceLaunch(context.Background(), req)
	if err != nil || len(f.removes) != 1 || f.removes[0] != original || proof.FenceRef == "" {
		t.Fatal(proof, err)
	}
}

func TestLaunchFenceRejectsChangedIdentityForeignObjectsAndUncertainInventory(t *testing.T) {
	for _, scenario := range []string{"engine", "instance", "session", "known-ref", "foreign-fence", "inspect-uncertain", "inventory-uncertain", "active-create"} {
		t.Run(scenario, func(t *testing.T) {
			b, f, req := newFenceFixture(t)
			id := f.original(req)
			switch scenario {
			case "engine":
				req.EngineID = "other-engine"
			case "instance":
				req.InstanceID = "other-instance"
			case "session":
				req.SessionID = "other-session"
			case "known-ref":
				req.ExpectedOriginalRef = strings.Repeat("e", 64)
			case "foreign-fence":
				row := f.rows[id]
				row.Config.Labels[backend.LabelLaunchFence] = "1"
				f.rows[id] = row
			case "inspect-uncertain":
				f.inspectErr = errors.Join(fenceMissing{}, errors.New("connection reset"))
			case "inventory-uncertain":
				f.listErr = errors.New("inventory unavailable")
			case "active-create":
				b.creates.Store(req.SandboxID, struct{}{})
			}
			if proof, err := b.FenceLaunch(context.Background(), req); err == nil || proof.FenceRef != "" || len(f.removes) != 0 || f.creates != 0 {
				t.Fatal("refusal crossed destructive edge", proof, err, f.removes)
			}
		})
	}
}

func TestLaunchFenceIdentityRequiresLocalSocketAndLinuxDaemon(t *testing.T) {
	for _, endpoint := range []string{"tcp://127.0.0.1:2375", "tcp://engine:2375", "npipe:////server/pipe/docker_engine", "unix://foreign/var/run/docker.sock", "unix:///socket?other=1", "unix:///tmp/../socket", "npipe:////./pipe/../engine"} {
		t.Run(endpoint, func(t *testing.T) {
			b, f, _ := newFenceFixture(t)
			f.endpoint = endpoint
			if _, err := b.LaunchFenceIdentity(context.Background()); !errors.Is(err, backend.ErrUnsupported) {
				t.Fatal(endpoint, err)
			}
		})
	}
	for _, endpoint := range []string{"unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine", "npipe:////./pipe/dockerDesktopLinuxEngine"} {
		b, f, _ := newFenceFixture(t)
		f.endpoint = endpoint
		if identity, err := b.LaunchFenceIdentity(context.Background()); err != nil || identity.Protocol != 1 {
			t.Fatal(endpoint, identity, err)
		}
	}
}

func TestLaunchFencePermanentInventoryBoundPrecedesOriginalRemoval(t *testing.T) {
	b, f, req := newFenceFixture(t)
	original := f.original(req)
	for i := 0; i < backend.MaxLaunchFences; i++ {
		id := fmt.Sprintf("%064x", i+100)
		f.rows[id] = container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: id, Name: fmt.Sprintf("/foreign-%d", i), State: &container.State{Status: "created"}}, Config: &container.Config{Labels: map[string]string{backend.LabelLaunchFence: "1"}}}
	}
	if _, err := b.FenceLaunch(context.Background(), req); !errors.Is(err, backend.ErrLaunchFencePending) || len(f.removes) != 0 || f.creates != 0 {
		t.Fatal("quota consumed original before refusing", err)
	}
	if _, exists := f.rows[original]; !exists {
		t.Fatal("original lost")
	}
}

func TestLaunchFenceProofRejectsMutationAndRetainsMarker(t *testing.T) {
	for _, scenario := range []string{"proof-engine", "proof-binding", "proof-ref", "proof-time", "name", "mount", "command", "state", "label", "extra-label", "image", "engine-changed"} {
		t.Run(scenario, func(t *testing.T) {
			b, f, req := newFenceFixture(t)
			proof, err := b.FenceLaunch(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			row := f.rows[proof.FenceRef]
			switch scenario {
			case "proof-engine":
				proof.EngineID = "wrong"
			case "proof-binding":
				proof.OperationDigest = strings.Repeat("d", 64)
			case "proof-ref":
				proof.FenceRef = strings.Repeat("e", 64)
			case "proof-time":
				proof.AcknowledgedAt = ""
			case "name":
				row.Name = "/renamed"
			case "mount":
				row.HostConfig.Binds = []string{"/outside:/inside"}
			case "command":
				row.Config.Cmd = []string{"/bin/sh"}
			case "state":
				row.State.Status = "running"
				row.State.Running = true
			case "label":
				row.Config.Labels[fenceIntentLabel] = strings.Repeat("d", 32)
			case "extra-label":
				row.Config.Labels[backend.LabelManaged] = "true"
			case "image":
				row.Config.Image = "worker-image"
			case "engine-changed":
				f.engine = "replacement-engine"
			}
			f.rows[row.ID] = row
			if err = b.VerifyLaunchFence(context.Background(), req, proof); err == nil || len(f.removes) != 0 {
				t.Fatal("changed proof accepted or mutated", err)
			}
		})
	}
}

func TestLaunchFenceLateOrdinaryDestroyOnlyTargetsOriginalImmutableID(t *testing.T) {
	b, f, req := newFenceFixture(t)
	original := f.original(req)
	f.beforeRemove = func(id string) {
		if id != original {
			t.Fatalf("mutable remove target %q", id)
		}
		// Simulate recovery winning after ordinary Destroy's inspection but
		// before its Remove RPC arrives. The stale RPC must retain the fence.
		f.mu.Lock()
		delete(f.rows, original)
		f.mu.Unlock()
		cfg, host := fenceConfiguration(req)
		_, _ = f.ContainerCreate(context.Background(), cfg, host, nil, nil, namePrefix+req.SandboxID)
	}
	if err := b.Destroy(context.Background(), req.SandboxID); err != nil {
		t.Fatal(err)
	}
	f.beforeRemove = nil
	if len(f.rows) != 1 || len(f.removes) != 1 || f.removes[0] != original {
		t.Fatal("late cleanup erased fence", f.removes)
	}
	if _, err := b.Get(context.Background(), req.SandboxID); !errors.Is(err, backend.ErrLaunchFenced) {
		t.Fatal(err)
	}
}

type fenceResourceDockerFixture struct {
	*fenceDockerFixture
	stats int
}

func (f *fenceResourceDockerFixture) ContainerStatsOneShot(context.Context, string) (container.StatsResponseReader, error) {
	f.stats++
	return container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(`{}`))}, nil
}

func TestLaunchFenceResourceObservationRequiresActualInertShape(t *testing.T) {
	for _, scenario := range []string{"inert", "running", "worker-image", "mount", "wrong-command", "incomplete-labels", "inspect-unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			b, f, req := newFenceFixture(t)
			proof, err := b.FenceLaunch(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			row := f.rows[proof.FenceRef]
			switch scenario {
			case "running":
				row.State.Status = "running"
				row.State.Running = true
			case "worker-image":
				row.Config.Image = "large-worker-image"
			case "mount":
				row.HostConfig.Binds = []string{"/work:/work"}
			case "wrong-command":
				row.Config.Cmd = []string{"/bin/sh"}
			case "incomplete-labels":
				delete(row.Config.Labels, fenceBindingLabel)
			case "inspect-unavailable":
				f.inspectErr = errors.New("Docker unavailable")
			}
			f.rows[proof.FenceRef] = row
			resourceAPI := &fenceResourceDockerFixture{fenceDockerFixture: f}
			b.api = resourceAPI
			snapshot := b.ObserveResources(context.Background())
			if scenario == "inert" {
				if len(snapshot.Containers) != 0 || resourceAPI.stats != 0 {
					t.Fatal("inert marker reserved worker resources", snapshot, resourceAPI.stats)
				}
			} else if len(snapshot.Containers) != 1 || resourceAPI.stats != 1 {
				t.Fatal("label alone bypassed capacity observation", snapshot, resourceAPI.stats)
			}
		})
	}
}

func TestLaunchFenceStoppedCleanupCannotRemoveMarkedObjects(t *testing.T) {
	for _, alreadyMarked := range []bool{true, false} {
		t.Run(fmt.Sprint(alreadyMarked), func(t *testing.T) {
			b, f, req := newFenceFixture(t)
			ref := f.original(req)
			row := f.rows[ref]
			row.State.Status = "exited"
			labels := map[string]string{}
			for key, value := range row.Config.Labels {
				labels[key] = value
			}
			if alreadyMarked {
				labels[backend.LabelLaunchFence] = "1"
			}
			row.Config.Labels[backend.LabelLaunchFence] = "1"
			f.rows[ref] = row
			observed := backend.Sandbox{ID: req.SandboxID, Ref: ref, Kind: backend.KindDocker, Instance: req.InstanceID,
				State: backend.StateStopped, Labels: labels}
			if err := b.DestroyStopped(context.Background(), observed); !errors.Is(err, backend.ErrLaunchFenced) || len(f.removes) != 0 {
				t.Fatal("stopped cleanup removed marker", err, f.removes)
			}
		})
	}
}

func TestLaunchFenceNewlyLearnedOriginalPinStrengthensSameIntent(t *testing.T) {
	b, f, req := newFenceFixture(t)
	original := f.original(req)
	saved := req
	req.ExpectedOriginalRef = original // learned after the intent was persisted
	if fenceConfigurationDigest(req) != fenceConfigurationDigest(saved) {
		t.Fatal("learning an exact removal pin changed retirement identity")
	}
	proof, err := b.FenceLaunch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.VerifyLaunchFence(context.Background(), saved, proof); err != nil {
		t.Fatal("same saved intent could not verify acknowledged fence", err)
	}
	if err = b.VerifyLaunchFence(context.Background(), req, proof); err != nil {
		t.Fatal("stronger original absence proof failed", err)
	}
}

func TestLaunchFenceRenamedOriginalAndDuplicateIdentityCannotBeReleased(t *testing.T) {
	for _, knownRef := range []bool{false, true} {
		t.Run(fmt.Sprint(knownRef), func(t *testing.T) {
			b, f, req := newFenceFixture(t)
			ref := f.original(req)
			row := f.rows[ref]
			row.Name = "/renamed-original"
			f.rows[ref] = row
			if knownRef {
				req.ExpectedOriginalRef = ref
			}
			if _, err := b.FenceLaunch(context.Background(), req); !errors.Is(err, backend.ErrLaunchFenceOwnership) || f.creates != 0 || len(f.removes) != 0 {
				t.Fatal("renamed original was ignored or removed", err, f.removes)
			}
		})
	}
	b, f, req := newFenceFixture(t)
	proof, err := b.FenceLaunch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	ref := f.original(req)
	row := f.rows[ref]
	row.Name = "/unexpected-other-original"
	f.rows[ref] = row
	if err = b.VerifyLaunchFence(context.Background(), req, proof); !errors.Is(err, backend.ErrLaunchFenceOwnership) || len(f.removes) != 0 {
		t.Fatal("fence alone hid an extra original allocation", err)
	}
}
