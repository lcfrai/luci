package product

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Real HTTP, durable contracts/operations and independent Git checkouts. Only
// the execution backend/admission is controlled; actual resource accounting
// and cross-project fairness are exercised by the API companion regression.
func TestSwarmConfiguredSixInOneProjectRetainsSeventhAndReplayProtection(t *testing.T) {
	s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
	s.registerSwarmScheduler(mux)
	s.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	req.PermissionMode, selected.PermissionMode = "bypass", "bypass"
	base := "/v1/product/projects/" + p.ID
	var tasks []ProjectTask
	var ids []string
	for i := 1; i <= 7; i++ {
		id := fmt.Sprintf("task-%d", i)
		ids = append(ids, id)
		tasks = append(tasks, ProjectTask{ID: id, Title: id, Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{id + ".txt"}})
	}
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": req.ImplementationContext.PlanRevision, "expectedSpecificationRevision": req.ImplementationContext.SpecificationRevision, "tasks": tasks}, 200).Body.Bytes(), &plan)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Save seven independent tasks"), 200)
	req.ImplementationContext.PlanRevision = plan.Revision
	req.ImplementationContext.TaskIDs = ids
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", req, 201)).ProjectRun
	swarmBase := base + "/runs/" + run.ID + "/swarm"
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", swarmBase, map[string]int{"expectedRevision": run.Revision, "maxParallel": 6}, 201)).ProjectRun
	policy := defaultSwarmSchedulerPolicy()
	policy.MaxActive = 6
	policy.MaxPerProject = 6
	journeyRequest(t, mux, "PUT", "/v1/product/swarm-scheduler", map[string]any{"expectedRevision": 1, "policy": policy, "dispatchPaused": false}, 200)
	starts := 0
	children := map[string]registry.Session{}
	s.swarmScheduler.runtime = SwarmSchedulerRuntime{
		CaptureActor: management.CaptureExecutionActor,
		Admit: func(_ context.Context, r SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
			if r.ProjectID != p.ID || r.RunID != run.ID || r.Actor.ProjectID != p.ID {
				t.Fatal("admission scope lost")
			}
			return SwarmAdmissionResult{State: "admitted", MemoryMiB: 1024, CPUMillis: 1000}, nil
		},
		Dispatch: func(_ context.Context, r SwarmDispatchRequest) (SwarmDispatchResult, error) {
			current, err := s.GetProjectSwarmRun(p.ID, run.ID)
			if err != nil {
				return SwarmDispatchResult{}, err
			}
			current, op, claimed, err := s.projectRuns.ClaimOperation(p.ID, p.Path, current.ID, current.Revision, r.OperationID)
			if err != nil || !claimed {
				t.Fatal("claim", claimed, err)
			}
			pick := *selected
			pick.WorktreeID = op.WorktreeID
			child := projectRunCandidate(pick, op.Implementation)
			child.ID = op.ChildID
			child.ProjectRunID = current.ID
			child.ProjectRunOperationID = op.ID
			child.Status = registry.StatusRunning
			children[op.ID] = child
			starts++
			return SwarmDispatchResult{State: "running", SessionID: op.ChildID, ReasonCode: "running", Message: "Running."}, nil
		}, Release: func(context.Context, string) error { return nil },
	}
	s.projectRunSessionObserver = func(_ context.Context, _, _, opID string) ([]registry.Session, error) {
		if child, ok := children[opID]; ok {
			return []registry.Session{child}, nil
		}
		return nil, nil
	}
	enqueueSchedulerFixture(t, mux, run, swarmBase, ids)
	for range 9 {
		if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if starts != 6 {
		t.Fatalf("configured six should start exactly six independent tasks, got %d", starts)
	}
	current, err := s.GetProjectSwarmRun(p.ID, run.ID)
	if err != nil || len(current.Operations) != 6 {
		t.Fatal(current, err)
	}
	paths := map[string]bool{}
	taskIDs := map[string]bool{}
	for _, op := range current.Operations {
		path, e := s.swarmPath(p.ID, run.ID, op.ID)
		if e != nil {
			t.Fatal(e)
		}
		if paths[path] {
			t.Fatal("worktree reused")
		}
		paths[path] = true
		info, e := os.Stat(filepath.Join(path, ".git"))
		if e != nil || !info.IsDir() {
			t.Fatal("independent Git metadata absent", e)
		}
		if op.Implementation == nil || len(op.Implementation.TaskIDs) != 1 || op.Implementation.TaskIDs[0] != op.TaskID || taskIDs[op.TaskID] {
			t.Fatal("task context widened or repeated", op)
		}
		taskIDs[op.TaskID] = true
	}
	before, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || before.Counts.Running != 6 || before.Counts.Queued != 1 {
		t.Fatal(before, err)
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.swarmScheduler.runtime = s.swarmScheduler.runtime
	reopened.projectRunSessionObserver = s.projectRunSessionObserver
	reopened.projectRunSelectionResolver = s.projectRunSelectionResolver
	for range 2 {
		if err = reopened.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	after, err := reopened.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || after.Counts.Running != 6 || after.Counts.Queued != 1 || starts != 6 {
		t.Fatal("reopen lost concurrency or replayed work", after, starts, err)
	}
	saved, err := reopened.SwarmSchedulerPolicy()
	if err != nil || saved != policy {
		t.Fatal("policy not durable", saved, err)
	}
}

func TestSwarmConfiguredLimitsUseBoundedTaskAndQueueContracts(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSwarmScheduler(mux)
	original, err := s.SwarmSchedulerPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if original.MaxActive != 4 || original.MaxPerProject != 2 {
		t.Fatal("default throughput changed", original)
	}
	valid := original
	valid.MaxActive = 500
	valid.MaxPerProject = 200
	journeyRequest(t, mux, "PUT", "/v1/product/swarm-scheduler", map[string]any{"expectedRevision": 1, "policy": valid, "dispatchPaused": true}, 200)
	before := auditTree(t, s.stateDir)
	for _, limits := range [][2]int{{501, 200}, {500, 201}, {0, 1}, {500, 0}, {6, 7}} {
		t.Run(fmt.Sprintf("global-%d-project-%d", limits[0], limits[1]), func(t *testing.T) {
			bad := valid
			bad.MaxActive = limits[0]
			bad.MaxPerProject = limits[1]
			journeyRequest(t, mux, "PUT", "/v1/product/swarm-scheduler", map[string]any{"expectedRevision": 2, "policy": bad, "dispatchPaused": true}, 400)
			if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("invalid policy changed durable state")
			}
		})
	}
	saved, err := s.SwarmSchedulerPolicy()
	if err != nil || saved != valid {
		t.Fatal(saved, err)
	}
}
