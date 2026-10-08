package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func swarmStoreFixture(t *testing.T, tasks []ProjectSwarmTask, limit int) (*ProjectRunStore, string, string, ProjectRun) {
	t.Helper()
	s, id, path, input := projectRunStoreFixture(t)
	input.Route.Connection = AgentConnectionReceipt{ID: "connection-api", Revision: 1, Kind: "api", Protocol: "openai-responses", AuthMode: "api-key"}
	input.Implementation.TaskIDs, input.Implementation.TaskTitles = []string{}, []string{}
	for _, task := range tasks {
		input.Implementation.TaskIDs = append(input.Implementation.TaskIDs, task.ID)
		input.Implementation.TaskTitles = append(input.Implementation.TaskTitles, task.Title)
	}
	run := createStoredRun(t, s, id, path, input)
	run, err := s.mutateSwarm(id, path, run.ID, run.Revision, func(run *ProjectRun) error {
		run.Swarm = &ProjectSwarm{State: "ready", MaxParallel: limit, Tasks: tasks}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, id, path, run
}

func TestProjectSwarmStoreSubscriptionConnectionKeepsOneActiveCredentialLease(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	input.Implementation.TaskIDs = []string{"a", "b"}
	input.Implementation.TaskTitles = []string{"Implement a", "Implement b"}
	run := createStoredRun(t, s, id, path, input)
	run, err := s.mutateSwarm(id, path, run.ID, run.Revision, func(run *ProjectRun) error {
		run.Swarm = &ProjectSwarm{State: "ready", MaxParallel: 2, Tasks: []ProjectSwarmTask{storedSwarmTask("a", "src/a"), storedSwarmTask("b", "src/b")}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	run, _ = reserveStoredSwarmTask(t, s, path, run, "a")
	if _, _, created, err := s.ReserveSwarmTask(id, path, run.ID, run.Revision, "subscription-second-123456789", "b", run.Input.Implementation.SourceCommit, func(string) string { t.Fatal("second subscription lease allocated"); return "" }); err == nil || created {
		t.Fatal("parallel task bypassed single subscription credential lease", created, err)
	}
}

func storedSwarmTask(id string, paths ...string) ProjectSwarmTask {
	return ProjectSwarmTask{ID: id, Title: "Implement " + id, DependsOn: []string{}, WritePaths: paths, Acceptance: "The intended change behaves correctly"}
}

func reserveStoredSwarmTask(t *testing.T, s *ProjectRunStore, path string, run ProjectRun, task string) (ProjectRun, ProjectRunOperation) {
	t.Helper()
	updated, op, created, err := s.ReserveSwarmTask(run.ProjectID, path, run.ID, run.Revision, "reserve-task-"+task+"-123456789", task, run.Input.Implementation.SourceCommit, func(id string) string { return project.WorktreeID(filepath.Join(path, id)) })
	if err != nil || !created {
		t.Fatalf("reserve %s: created=%v err=%v", task, created, err)
	}
	return updated, op
}

func allocateStoredSwarmTask(t *testing.T, s *ProjectRunStore, path string, run ProjectRun, op ProjectRunOperation) ProjectRun {
	t.Helper()
	binding := run.Input.Implementation.Clone()
	binding.WorktreeID = op.WorktreeID
	binding.TaskIDs = []string{op.TaskID}
	binding.TaskTitles = []string{swarmTask(&run, op.TaskID).Title}
	binding.SourceCommit = op.BaseCommit
	if op.DependencyContext != nil {
		binding.DependencyContextDigest = op.DependencyContext.Digest
	}
	route := run.Input.Route
	candidate := registry.Session{ProjectID: run.ProjectID, WorktreeID: op.WorktreeID, ImplementationContext: binding, Harness: route.Harness, Model: route.Model, PermissionMode: route.PermissionMode, AgentConnectionID: route.Connection.ID, AgentConnectionRevision: route.Connection.Revision, AgentConnectionKind: route.Connection.Kind, AuthMode: route.Connection.AuthMode}
	updated, err := s.CompleteSwarmAllocation(run.ProjectID, path, run.ID, run.Revision, op.ID, binding, candidate, nil)
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func TestProjectSwarmStoreConcurrentReservationRestartAndUnknownAllocationNeverReplay(t *testing.T) {
	s, id, path, run := swarmStoreFixture(t, []ProjectSwarmTask{storedSwarmTask("a", "src/a"), storedSwarmTask("b", "src/b")}, 2)
	var allocations, fresh atomic.Int32
	var wg sync.WaitGroup
	const key = "concurrent-reservation-123456789"
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, created, err := s.ReserveSwarmTask(id, path, run.ID, run.Revision, key, "a", run.Input.Implementation.SourceCommit, func(operation string) string {
				allocations.Add(1)
				return project.WorktreeID(filepath.Join(path, operation))
			})
			if err != nil {
				t.Error(err)
			}
			if created {
				fresh.Add(1)
			}
		}()
	}
	wg.Wait()
	if allocations.Load() != 1 || fresh.Load() != 1 {
		t.Fatalf("concurrent requests produced %d reservations and %d path allocations", fresh.Load(), allocations.Load())
	}
	run, err := s.Get(id, path, run.ID)
	if err != nil || len(run.Operations) != 1 {
		t.Fatal(run, err)
	}
	op := run.Operations[0]
	if op.Allocation != "allocating" || op.Status != "reserved" || swarmTask(&run, "a").OperationID != op.ID {
		t.Fatalf("reservation did not retain its exact task child: %#v", run)
	}
	if _, _, claimed, err := s.ClaimOperation(id, path, run.ID, run.Revision, op.ID); err == nil || claimed {
		t.Fatal("allocation without a confirmed receipt granted dispatch", claimed, err)
	}
	file := filepath.Join(s.stateDir, "project-runs", id, run.ID+".json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, duplicate, created, err := reopened.ReserveSwarmTask(id, path, run.ID, 1, key, "a", run.Input.Implementation.SourceCommit, func(string) string { t.Fatal("retry reallocated a worktree identity"); return "" })
	if err != nil || created || duplicate.ID != op.ID || duplicate.ChildID != op.ChildID || duplicate.WorktreeID != op.WorktreeID || recovered.Revision != run.Revision {
		t.Fatal(recovered, duplicate, created, err)
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("restart or duplicate request rewrote the persisted attempt")
	}
	if _, _, _, err := reopened.ReserveSwarmTask(id, path, run.ID, run.Revision, key, "b", op.BaseCommit, func(string) string { return "" }); !errors.Is(err, ErrProjectRunKeyConflict) {
		t.Fatalf("same key rebound to another task: %v", err)
	}
	if _, _, _, err := reopened.ReserveSwarmTask(id, path, run.ID, run.Revision, "another-attempt-123456789", "a", op.BaseCommit, func(string) string { return "" }); err == nil {
		t.Fatal("new key replayed an unresolved allocation")
	}
	run, err = reopened.CompleteSwarmAllocation(id, path, run.ID, run.Revision, op.ID, nil, registry.Session{}, errors.New("simulated interruption after creating checkout"))
	if err != nil || run.Operations[0].Allocation != "unknown" || run.Operations[0].Status != "unknown" {
		t.Fatal(run, err)
	}
	if _, _, _, err := reopened.ReserveSwarmTask(id, path, run.ID, run.Revision, "unknown-retry-123456789", "a", op.BaseCommit, func(string) string { return "" }); err == nil {
		t.Fatal("unknown allocation permitted a replacement child")
	}
}

func TestProjectSwarmStoreCapacityAndWindowsOwnershipConflicts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		a, b     []string
		conflict bool
	}{
		{"disjoint", []string{"src/a"}, []string{"src/b"}, false},
		{"component boundary", []string{"src/api"}, []string{"src/api2"}, false},
		{"directory ancestor", []string{"src"}, []string{"src/a/file.go"}, true},
		{"windows case", []string{"src/App.tsx"}, []string{"SRC/app.tsx"}, true},
		{"unicode case", []string{"src/ſervice"}, []string{"src/Service"}, true},
		{"first unscoped", nil, []string{"src/b"}, true},
		{"second unscoped", []string{"src/a"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, id, path, run := swarmStoreFixture(t, []ProjectSwarmTask{storedSwarmTask("a", tc.a...), storedSwarmTask("b", tc.b...), storedSwarmTask("c", "other")}, 2)
			run, _ = reserveStoredSwarmTask(t, s, path, run, "a")
			next, _, created, err := s.ReserveSwarmTask(id, path, run.ID, run.Revision, "reserve-second-123456789", "b", run.Input.Implementation.SourceCommit, func(op string) string { return project.WorktreeID(filepath.Join(path, op)) })
			if tc.conflict {
				if err == nil || created || !strings.Contains(err.Error(), "scope") {
					t.Fatalf("overlapping/unscoped task was not refused: created=%v err=%v", created, err)
				}
				return
			}
			if err != nil || !created || len(next.Operations) != 2 || next.Operations[0].WorktreeID == next.Operations[1].WorktreeID || next.Operations[0].ChildID == next.Operations[1].ChildID {
				t.Fatal("independent tasks did not receive separate slots", next, created, err)
			}
			if _, _, created, err := s.ReserveSwarmTask(id, path, next.ID, next.Revision, "over-capacity-123456789", "c", run.Input.Implementation.SourceCommit, func(string) string { t.Fatal("over-capacity request allocated identity"); return "" }); err == nil || created || !strings.Contains(err.Error(), "slots") {
				t.Fatal("parallel limit did not count unresolved reservations", created, err)
			}
		})
	}
}

func TestProjectSwarmStoreDependenciesRequireEveryIntegratedReceipt(t *testing.T) {
	a, b, c := storedSwarmTask("a", "src/a"), storedSwarmTask("b", "src/b"), storedSwarmTask("c", "tests")
	c.DependsOn = []string{"a", "b"}
	s, id, path, run := swarmStoreFixture(t, []ProjectSwarmTask{a, b, c}, 2)
	for _, task := range []string{"a", "b"} {
		var op ProjectRunOperation
		run, op = reserveStoredSwarmTask(t, s, path, run, task)
		run = allocateStoredSwarmTask(t, s, path, run, op)
		var err error
		run, err = s.ObserveOperation(id, path, run.ID, run.Revision, op.ID, op.ChildID, "succeeded", "Recorded coding exit; not correctness evidence")
		if err != nil {
			t.Fatal(err)
		}
	}
	assertBlocked := func(label string) {
		t.Helper()
		if _, _, created, err := s.ReserveSwarmTask(id, path, run.ID, run.Revision, "dependent-task-123456789", "c", run.Input.Implementation.SourceCommit, func(string) string { t.Fatal("blocked dependency allocated a checkout identity"); return "" }); err == nil || created || !strings.Contains(err.Error(), "dependencies") {
			t.Fatalf("%s unlocked dependent task: created=%v err=%v", label, created, err)
		}
	}
	assertBlocked("successful coding exits")
	receipt := func(state string) *ProjectSwarmIntegration {
		return &ProjectSwarmIntegration{State: state, SourceCommit: strings.Repeat("2", 40), TargetCommit: run.Input.Implementation.SourceCommit, Commit: strings.Repeat("3", 40), ReviewDigest: strings.Repeat("b", 64), IntegratedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	setReceipt := func(task, state string) {
		t.Helper()
		var err error
		run, err = s.mutateSwarm(id, path, run.ID, run.Revision, func(run *ProjectRun) error { swarmTask(run, task).Integration = receipt(state); return nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	setReceipt("a", "integrating")
	assertBlocked("integration intent")
	setReceipt("a", "unknown")
	assertBlocked("unconfirmed integration")
	setReceipt("a", "integrated")
	assertBlocked("only one integrated dependency")
	setReceipt("b", "integrated")
	run, op := reserveStoredSwarmTask(t, s, path, run, "c")
	if op.TaskID != "c" || len(run.Checks) != 0 || run.Acceptance != nil || run.State == "complete" {
		t.Fatalf("dependency admission manufactured whole-build evidence: %#v", run)
	}
}

func TestProjectSwarmStorePauseCancelAndUncertainDispatchRetainExactChild(t *testing.T) {
	s, id, path, run := swarmStoreFixture(t, []ProjectSwarmTask{storedSwarmTask("a", "src/a")}, 1)
	run, op := reserveStoredSwarmTask(t, s, path, run, "a")
	run = allocateStoredSwarmTask(t, s, path, run, op)
	var err error
	run, err = s.mutateSwarm(id, path, run.ID, run.Revision, func(run *ProjectRun) error { run.Swarm.State = "paused"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, _, claimed, err := s.ClaimOperation(id, path, run.ID, run.Revision, op.ID); err == nil || claimed {
		t.Fatal("paused run granted new dispatch", claimed, err)
	}
	run, err = s.mutateSwarm(id, path, run.ID, run.Revision, func(run *ProjectRun) error { run.Swarm.State = "ready"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	var claimed bool
	run, _, claimed, err = s.ClaimOperation(id, path, run.ID, run.Revision, op.ID)
	if err != nil || !claimed {
		t.Fatal(claimed, err)
	}
	reopened, err := NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(id, path, run.ID)
	if err != nil || loaded.Operations[0].Status != "dispatching" || loaded.Operations[0].ChildID != op.ChildID {
		t.Fatal(loaded, err)
	}
	if _, recovered, claimed, err := reopened.ClaimOperation(id, path, run.ID, loaded.Revision, op.ID); err != nil || claimed || recovered.ChildID != op.ChildID {
		t.Fatal("restart replayed a claimed dispatch", recovered, claimed, err)
	}
	run, err = reopened.Cancel(id, path, run.ID, loaded.Revision)
	if err != nil || run.Operations[0].Status != "dispatching" || run.Operations[0].ChildID != op.ChildID {
		t.Fatal("cancelling future work erased an in-flight child", run, err)
	}
	if _, _, claimed, err := reopened.ClaimOperation(id, path, run.ID, run.Revision, op.ID); !errors.Is(err, ErrProjectRunCancelled) || claimed {
		t.Fatal("cancelled run granted dispatch", claimed, err)
	}
}

func TestProjectSwarmStoreCorruptCoordinationFailsClosedWithoutRepair(t *testing.T) {
	for _, tc := range []struct {
		name    string
		corrupt func(*ProjectSwarm)
	}{
		{"cycle", func(s *ProjectSwarm) { s.Tasks[0].DependsOn = []string{"b"}; s.Tasks[1].DependsOn = []string{"a"} }},
		{"duplicate dependency", func(s *ProjectSwarm) { s.Tasks[1].DependsOn = []string{"a", "a"} }},
		{"unsafe write path", func(s *ProjectSwarm) { s.Tasks[0].WritePaths = []string{"../outside"} }},
		{"windows duplicate", func(s *ProjectSwarm) { s.Tasks[0].WritePaths = []string{"src/App", "SRC/app"} }},
		{"oversized acceptance", func(s *ProjectSwarm) { s.Tasks[0].Acceptance = strings.Repeat("x", 4001) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, id, path, run := swarmStoreFixture(t, []ProjectSwarmTask{storedSwarmTask("a", "src/a"), storedSwarmTask("b", "src/b")}, 2)
			file := filepath.Join(s.stateDir, "project-runs", id, run.ID+".json")
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var record storedProjectRun
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			tc.corrupt(record.Run.Swarm)
			data, err = json.Marshal(record)
			if err != nil || os.WriteFile(file, data, 0600) != nil {
				t.Fatal("could not prepare corrupted record", err)
			}
			if _, err := s.Get(id, path, run.ID); err == nil {
				t.Fatal("corrupted coordination record accepted")
			}
			after, _ := os.ReadFile(file)
			if !reflect.DeepEqual(data, after) {
				t.Fatal("read silently repaired corrupted coordination")
			}
		})
	}
}

func TestProjectSwarmStoreFailedAttemptRequiresExplicitDistinctRetry(t *testing.T) {
	s, id, path, run := swarmStoreFixture(t, []ProjectSwarmTask{storedSwarmTask("a", "src/a")}, 1)
	run, first := reserveStoredSwarmTask(t, s, path, run, "a")
	run = allocateStoredSwarmTask(t, s, path, run, first)
	var err error
	run, err = s.ObserveOperation(id, path, run.ID, run.Revision, first.ID, first.ChildID, "failed", "Observed process failure")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		loaded, err := s.Get(id, path, run.ID)
		if err != nil || len(loaded.Operations) != 1 || loaded.Operations[0].Status != "failed" {
			t.Fatal("read allocated a retry", loaded, err)
		}
	}
	run, second, created, err := s.ReserveSwarmTask(id, path, run.ID, run.Revision, fmt.Sprintf("explicit-retry-%s-123456789", first.ID), "a", first.BaseCommit, func(id string) string { return project.WorktreeID(filepath.Join(path, id)) })
	if err != nil || !created || first.ID == second.ID || first.ChildID == second.ChildID || first.WorktreeID == second.WorktreeID || len(run.Operations) != 2 || run.Operations[0].Status != "failed" || swarmTask(&run, "a").OperationID != second.ID {
		t.Fatal("explicit retry erased or reused previous attempt", run, second, created, err)
	}
}
