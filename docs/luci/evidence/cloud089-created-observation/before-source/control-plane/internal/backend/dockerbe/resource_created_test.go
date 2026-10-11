package dockerbe_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
)

func createdResourceInspect(id string) container.InspectResponse {
	return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
		ID: id, State: &container.State{Status: "created", StartedAt: "0001-01-01T00:00:00Z"}, HostConfig: &container.HostConfig{},
	}}
}

func TestResourceObserverConfirmedNeverStartedContainersHaveNoLiveDemand(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "owned-pending"}[owned], func(t *testing.T) {
			id := strings.Repeat("a", 64)
			labels := map[string]string{}
			if owned {
				labels = labelsFor("s-pending", "session-pending")
			}
			f := &resourceDocker{fakeDocker: &fakeDocker{
				list:     []container.Summary{{ID: id, State: "created", Labels: labels}},
				inspects: []inspectResult{{resp: createdResourceInspect(id)}},
			}}
			b, err := dockerbe.New(dockerbe.Config{Client: f, Instance: testInstance})
			if err != nil {
				t.Fatal(err)
			}
			got := b.ObserveResources(context.Background())
			if !got.InventoryComplete || len(got.Containers) != 0 || f.closedBodies != 0 {
				t.Fatalf("never-started container treated as live demand: %+v; stats=%d", got, f.closedBodies)
			}
			if len(f.inspectNames) != 1 || f.inspectNames[0] != id || len(f.creates)+len(f.starts)+len(f.removes) != 0 {
				t.Fatalf("observation did not inspect exact identity without mutation: %+v", f.fakeDocker)
			}
		})
	}
}

func TestResourceObserverCreatedExemptionRequiresCurrentExactNeverStartedState(t *testing.T) {
	for _, scenario := range []string{"inspect-error", "missing-base", "wrong-id", "missing-state", "started", "invalid-start-time", "running", "paused", "restarting", "dead", "pid", "changed-status", "launch-fence-label"} {
		t.Run(scenario, func(t *testing.T) {
			id := strings.Repeat("a", 64)
			row := createdResourceInspect(id)
			var inspectErr error
			labels := map[string]string{}
			switch scenario {
			case "inspect-error":
				inspectErr = errors.New("unavailable")
			case "missing-base":
				row.ContainerJSONBase = nil
			case "wrong-id":
				row.ID = strings.Repeat("b", 64)
			case "missing-state":
				row.State = nil
			case "started":
				row.State.StartedAt = "2026-10-01T00:00:00Z"
			case "invalid-start-time":
				row.State.StartedAt = "unknown"
			case "running":
				row.State.Running = true
			case "paused":
				row.State.Paused = true
			case "restarting":
				row.State.Restarting = true
			case "dead":
				row.State.Dead = true
			case "pid":
				row.State.Pid = 123
			case "changed-status":
				row.State.Status = "running"
			case "launch-fence-label":
				labels[backend.LabelLaunchFence] = "1"
			}
			f := &resourceDocker{fakeDocker: &fakeDocker{
				list:     []container.Summary{{ID: id, State: "created", Labels: labels}},
				inspects: []inspectResult{{resp: row, err: inspectErr}},
			}}
			b, err := dockerbe.New(dockerbe.Config{Client: f, Instance: testInstance})
			if err != nil {
				t.Fatal(err)
			}
			got := b.ObserveResources(context.Background())
			if len(got.Containers) != 1 || f.closedBodies != 1 || got.Containers[0].MemoryUsageBytes != nil {
				t.Fatalf("uncertain or changed state bypassed resource demand: %+v; stats=%d", got, f.closedBodies)
			}
		})
	}
}

func TestResourceObserverCreatedInspectionRemainsBounded(t *testing.T) {
	rows := make([]container.Summary, 129)
	id := strings.Repeat("a", 64)
	for i := range rows {
		rows[i] = container.Summary{ID: id, State: "created"}
	}
	f := &resourceDocker{fakeDocker: &fakeDocker{list: rows, inspects: []inspectResult{{resp: createdResourceInspect(id)}}}}
	b, err := dockerbe.New(dockerbe.Config{Client: f, Instance: testInstance})
	if err != nil {
		t.Fatal(err)
	}
	got := b.ObserveResources(context.Background())
	if got.InventoryComplete || got.Status != "unavailable" || len(f.inspectNames) != 128 {
		t.Fatalf("created inventory evaded observation budget: %+v; inspect=%d", got, len(f.inspectNames))
	}
}
