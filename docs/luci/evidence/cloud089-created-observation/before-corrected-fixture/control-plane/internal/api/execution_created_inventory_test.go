package api

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// Only observer reads are implemented. Any attempt to allocate or mutate a
// Docker object through this fixture fails rather than silently succeeding.
type createdInventoryDocker struct {
	dockerbe.APIClient
	state string
	owned bool
	stats int
}

func (f *createdInventoryDocker) Ping(context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (f *createdInventoryDocker) Info(context.Context) (system.Info, error) {
	return system.Info{ID: httpCheckEngine, OSType: "linux", NCPU: 8, MemTotal: 16 << 30, Driver: "overlay2"}, nil
}
func (f *createdInventoryDocker) ContainerList(context.Context, container.ListOptions) ([]container.Summary, error) {
	labels := map[string]string{}
	if f.owned {
		labels = map[string]string{backend.LabelManaged: "true", backend.LabelInstance: httpCheckInstance, backend.LabelSandboxID: "s-held"}
	}
	return []container.Summary{{ID: strings.Repeat("a", 64), State: f.state, Labels: labels}}, nil
}
func (f *createdInventoryDocker) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
	return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: id, HostConfig: &container.HostConfig{}, State: &container.State{Status: f.state, Running: f.state == "running", Paused: f.state == "paused", Restarting: f.state == "restarting", StartedAt: "0001-01-01T00:00:00Z"}}}, nil
}
func (f *createdInventoryDocker) ContainerStatsOneShot(context.Context, string) (container.StatsResponseReader, error) {
	f.stats++
	return container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(`{}`)), OSType: "linux"}, nil
}

func createdInventorySnapshot(t *testing.T, f *createdInventoryDocker) resourcebudget.Snapshot {
	t.Helper()
	b, err := dockerbe.New(dockerbe.Config{Client: f, Instance: httpCheckInstance})
	if err != nil {
		t.Fatal(err)
	}
	observed := b.ObserveResources(context.Background())
	if !observed.InventoryComplete {
		t.Fatal("controlled inventory incomplete", observed)
	}
	// Capacity and storage are deterministic fixture inputs. Container demand
	// and its completeness are the output of the actual production observer.
	snapshot := executionFixtureSnapshot()
	snapshot.Execution.EngineID = observed.EngineID
	snapshot.Execution.Containers = observed.Containers
	snapshot.Execution.InventoryComplete = observed.InventoryComplete
	return snapshot
}

func TestCreatedContainerInventoryDoesNotBlockButLiveUnknownStillRefuses(t *testing.T) {
	for _, state := range []string{"created", "running", "paused", "restarting"} {
		t.Run(state, func(t *testing.T) {
			g := previewTestGate(t)
			f := &createdInventoryDocker{state: state}
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return createdInventorySnapshot(t, f) }
			d, _, err := g.reserveRuntimeImage(context.Background(), runtimeImagePrepared(g, "runtime-probe"), management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
			if err != nil {
				t.Fatal(err)
			}
			if state == "created" {
				if d.State != "admitted" || f.stats != 0 {
					t.Fatal("never-started foreign container blocked admission", d, f.stats)
				}
			} else if d.State != "waiting" || d.Code != "execution_limits_unknown" || f.stats == 0 {
				t.Fatal("live unknown workload was treated as zero", d, f.stats)
			}
		})
	}
}

func TestCreatedOwnedWorkerChargeSurvivesObservationAndReopenAcrossAdmissions(t *testing.T) {
	g := previewTestGate(t)
	g.api.deps.Resources = backend.Resources{MemoryMiB: 6144, CPUMillis: 1000}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-held"}
	if d, err := g.reserve(context.Background(), "queue-held", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil || d.State != "admitted" {
		t.Fatal(d, err)
	}
	if err := g.bind(context.Background(), "queue-held", "s-held"); err != nil {
		t.Fatal(err)
	}
	f := &createdInventoryDocker{state: "created", owned: true}
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return createdInventorySnapshot(t, f) }
	g = newExecutionAdmission(g.api)
	before, err := g.readReservationLedger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	g.api.deps.Resources = backend.Resources{MemoryMiB: 3072, CPUMillis: 1000}
	actor.ProjectID = "p-other"
	assertHeld := func(d executionDecision, err error) {
		t.Helper()
		if err != nil || d.State != "waiting" || d.Code != "memory_pressure" {
			t.Fatal("pending allocation lost its held memory charge", d, err)
		}
		after, err := g.readReservationLedger(context.Background())
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("admission changed retained allocation", before, after, err)
		}
	}
	assertHeld(g.reserve(context.Background(), "queue-other", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy()))
	check := backend.PreparedCheck{Spec: backend.CheckSpec{Identity: backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: "check-other", CheckID: "check-other", ProjectID: actor.ProjectID, ScopeDigest: g.scope, CommandDigest: strings.Repeat("b", 64)}, Resources: g.api.deps.Resources}, StorageDriver: "overlay2", ImageVirtualSizeBytes: 1 << 30}
	assertHeld(g.reserveCheck(context.Background(), check, actor, management.ExecutionAuthority{}, executionFixturePolicy()))
	d, _, err := g.reservePreview(context.Background(), previewTestPrepared(g, actor.ProjectID, "preview-other", "preview", 3072, 1000), actor, management.ExecutionAuthority{}, executionFixturePolicy())
	assertHeld(d, err)
	runtime := runtimeImagePrepared(g, "runtime-other")
	runtime.Spec.Resources = g.api.deps.Resources
	d, _, err = g.reserveRuntimeImage(context.Background(), runtime, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
	assertHeld(d, err)
	if f.stats != 0 {
		t.Fatal("created worker requested meaningless stats", f.stats)
	}
}
