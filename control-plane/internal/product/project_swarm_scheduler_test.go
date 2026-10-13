package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func schedulerFixture(t *testing.T, cloud ...bool) (*Server, *http.ServeMux, ProjectRun, string, *bool, *int, map[string]registry.Session) {
	t.Helper()
	location := ""
	if len(cloud) > 0 && cloud[0] {
		location = "cloud-microvm"
	}
	s, mux, p, run, base, selection := swarmHTTPFixtureWithLocation(t, location)
	s.registerSwarmScheduler(mux)
	admitted := false
	starts := 0
	children := map[string]registry.Session{}
	s.swarmScheduler.runtime = SwarmSchedulerRuntime{
		CaptureActor: management.CaptureExecutionActor,
		Admit: func(_ context.Context, req SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
			if req.ProjectID != p.ID || req.RunID != run.ID || req.Actor.ProjectID != p.ID {
				t.Fatal("admission lost reviewed scope")
			}
			if !admitted {
				return SwarmAdmissionResult{State: "waiting", ReasonCode: "memory_pressure", Message: "Waiting for memory headroom."}, nil
			}
			return SwarmAdmissionResult{State: "admitted", MemoryMiB: 1024, CPUMillis: 1000}, nil
		},
		Dispatch: func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
			current, err := s.GetProjectSwarmRun(req.ProjectID, req.RunID)
			if err != nil {
				return SwarmDispatchResult{}, err
			}
			current, op, claimed, err := s.projectRuns.ClaimOperation(p.ID, p.Path, current.ID, current.Revision, req.OperationID)
			if err != nil || !claimed {
				t.Fatalf("claim: %v %v", claimed, err)
			}
			starts++
			picked := *selection
			picked.WorktreeID = op.WorktreeID
			child := projectRunCandidate(picked, op.Implementation)
			child.ID = op.ChildID
			child.ProjectRunID = current.ID
			child.ProjectRunOperationID = op.ID
			child.Status = registry.StatusRunning
			children[op.ID] = child
			return SwarmDispatchResult{State: "running", SessionID: op.ChildID, ReasonCode: "running", Message: "Running."}, nil
		},
		Release: func(context.Context, string) error { return nil },
	}
	s.projectRunSessionObserver = func(_ context.Context, projectID, runID, opID string) ([]registry.Session, error) {
		if child, ok := children[opID]; ok {
			return []registry.Session{child}, nil
		}
		return nil, nil
	}
	return s, mux, run, base, &admitted, &starts, children
}
func enqueueSchedulerFixture(t *testing.T, mux *http.ServeMux, run ProjectRun, base string, tasks []string) []SwarmQueueItem {
	t.Helper()
	var response struct {
		Items []SwarmQueueItem `json:"items"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/queue", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "explicit-reviewed-batch", "priority": "normal", "taskIds": tasks}, 201).Body.Bytes(), &response)
	return response.Items
}

func TestAcceptedAmendmentFencesOldRunDependentAfterCurrentGraphChanges(t *testing.T) {
	s, mux, run, base, _, _, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b", "task-c"})
	if len(items) != 3 {
		t.Fatal("old graph fixture did not retain all tasks")
	}
	current := ImplementationContextRequest{ProjectID: run.ProjectID, BriefRevision: run.Input.Implementation.BriefRevision,
		PlanRevision:          run.Input.Implementation.PlanRevision + 1,
		SpecificationRevision: run.Input.Implementation.SpecificationRevision,
		DesignPlanRevision:    run.Input.Implementation.DesignPlanRevision, TaskIDs: []string{"task-a"}}
	// task-c depends on task-a in this saved run. A revised plan may remove
	// that edge, but the old task-c still consumed old task-a's contract.
	if err := s.blockSupersededQueuedTasks(run.ProjectID, current); err != nil {
		t.Fatal(err)
	}
	queue, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || len(queue.Items) != 3 {
		t.Fatal(err, queue)
	}
	states := map[string]string{}
	for _, item := range queue.Items {
		states[item.TaskID] = item.State
	}
	if states["task-a"] != "blocked" || states["task-c"] != "blocked" || states["task-b"] != "queued" {
		t.Fatal("old dependent or independent task changed incorrectly", states)
	}
}

func TestExactActiveQueueStopRetainsChargeAfterLostACKAndSettlesAfterRestart(t *testing.T) {
	s, mux, run, base, admitted, starts, _ := schedulerFixture(t, true)
	*admitted = true
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("owned task did not start once", err, *starts)
	}
	before, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || len(before.Items) != 1 || before.Items[0].OperationID == "" || before.Items[0].ChildID == "" {
		t.Fatal("active attempt lacks exact identity", before, err)
	}
	stops, settled := 0, false
	runtime := s.swarmScheduler.runtime
	runtime.StopOwned = func(_ context.Context, req SwarmSupersededRequest) (bool, error) {
		stops++
		if req.QueueID != before.Items[0].ID || req.ProjectID != run.ProjectID || req.RunID != run.ID || req.OperationID != before.Items[0].OperationID || req.ChildID != before.Items[0].ChildID {
			t.Fatalf("stop lost immutable attempt identity: %+v", req)
		}
		return settled, nil
	}
	s.swarmScheduler.runtime = runtime
	path := "/v1/product/projects/" + run.ProjectID + "/swarm-queue/" + before.Items[0].ID + "/stop"
	journeyRequest(t, mux, "POST", path, map[string]int{"expectedRevision": before.Items[0].Revision}, 202)
	pending, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || pending.Items[0].ReasonCode != "operator_stop_pending" || pending.Items[0].MemoryMiB != 1024 || pending.Items[0].State != "unknown" || stops != 1 {
		t.Fatal("lost stop acknowledgement released capacity", pending, err, stops)
	}
	journeyRequest(t, mux, "POST", path, map[string]int{"expectedRevision": before.Items[0].Revision}, 409)
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("stop pending resumed a worker", err, *starts)
	}
	settled = true
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.swarmScheduler.runtime = runtime
	if err = reopened.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	final, err := reopened.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || final.Items[0].State != "cancelled" || final.Items[0].ReasonCode != "operator_stopped" || final.Items[0].MemoryMiB != 0 || *starts != 1 {
		t.Fatal("exact stop did not settle once after restart", final, err, *starts)
	}
}
func TestSwarmSchedulerPressurePrecedesCloneAndBatchConsentIsAtomic(t *testing.T) {
	s, mux, run, base, admitted, starts, children := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b", "task-c"})
	if len(items) != 3 {
		t.Fatal("batch was partial")
	}
	before := auditTree(t, s.stateDir)
	journeyRequest(t, mux, "GET", "/v1/product/projects/"+run.ProjectID+"/swarm-queue", nil, 200)
	journeyRequest(t, mux, "HEAD", "/v1/product/swarm-scheduler", nil, 200)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("read changed queue")
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 0 {
		t.Fatal("started under pressure")
	}
	if _, err := os.Stat(filepath.Join(s.projectRoot(), ".luci-swarms")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pressure allocated clone: %v", err)
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Queued != 3 {
		t.Fatalf("waiting projection: %+v %v", snapshot, err)
	}
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	if err == nil {
		row.Policy.MaxActive = 1
		row.Policy.MaxPerProject = 1
		err = s.saveSwarmScheduler(&row)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	*admitted = true
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 1 {
		t.Fatalf("want one agent, got %d", *starts)
	}
	current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Operations) != 1 {
		t.Fatalf("allocated waiting children: %d", len(current.Operations))
	}
	// Repeating the full consent survives revision changes and returns the same receipts.
	var repeated struct {
		Items []SwarmQueueItem `json:"items"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/queue", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "explicit-reviewed-batch", "priority": "normal", "taskIds": []string{"task-c", "task-b", "task-a"}}, 200).Body.Bytes(), &repeated)
	for i, item := range items {
		if repeated.Items[i].ID != item.ID {
			t.Fatal("duplicate consent allocated another queue item")
		}
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 1 {
		t.Fatal("replayed active child")
	}
	op := current.Operations[0]
	child := children[op.ID]
	child.Status = registry.StatusExited
	child.ExitReason = "exited"
	children[op.ID] = child
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 2 {
		t.Fatalf("known exit did not release capacity: %d", *starts)
	}
	snapshot, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Counts.Completed != 1 || snapshot.Counts.Running != 1 || snapshot.Counts.Queued != 1 {
		t.Fatalf("wrong lifecycle %+v", snapshot)
	}
	for _, item := range snapshot.Items {
		if item.TaskID == "task-c" && item.State != "queued" {
			t.Fatal("process exit released dependency without integration")
		}
	}
}

func TestCloudSwarmKeepsSchedulerSlotUntilExactMicroVMRetirement(t *testing.T) {
	s, mux, run, base, admitted, starts, _ := schedulerFixture(t, true)
	*admitted = true
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err == nil {
		ledger.Policy.MaxActive, ledger.Policy.MaxPerProject = 1, 1
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	retired := false
	s.swarmScheduler.runtime.CloudRetired = func(_ context.Context, projectID, runID, operationID, childID string) (bool, error) {
		if projectID != run.ProjectID || runID != run.ID || operationID == "" || childID == "" {
			t.Fatal("retirement checked another cloud allocation")
		}
		return retired, nil
	}
	s.projectRunSessionObserver = func(context.Context, string, string, string) ([]registry.Session, error) {
		t.Fatal("cloud task was observed through the local Docker registry")
		return nil, nil
	}
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("first cloud task was not dispatched exactly once", err, *starts)
	}
	current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil || len(current.Operations) != 1 {
		t.Fatal(err, current.Operations)
	}
	op := current.Operations[0]
	_, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, current.Revision, op.ID, op.ChildID, "review-required", "exact cloud native exit retained")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("uncertain cloud retirement released the next slot", err, *starts)
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || snapshot.Counts.Unknown != 1 || snapshot.Counts.Queued != 1 {
		t.Fatal("retirement uncertainty was not durable", snapshot, err)
	}
	retired = true
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 2 {
		t.Fatal("confirmed cloud absence did not release the next task", err, *starts)
	}
}
func TestCancelledCloudAttemptKeepsSchedulerSlotUntilVMRetirement(t *testing.T) {
	s, mux, run, base, admitted, starts, _ := schedulerFixture(t, true)
	*admitted = true
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	retired := false
	s.swarmScheduler.runtime.CloudRetired = func(_ context.Context, projectID, runID, operationID, childID string) (bool, error) {
		if projectID != run.ProjectID || runID != run.ID || operationID == "" || childID == "" {
			t.Fatal("cloud cancellation checked a different attempt")
		}
		return retired, nil
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("cloud attempt did not start once", err, *starts)
	}
	current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	op := current.Operations[0]
	current, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, current.Revision, op.ID, op.ChildID, "cancelled", "exact retirement requested")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || queue.Counts.Unknown != 1 || *starts != 1 {
		t.Fatal("cancelled task released an unretired VM", queue, err)
	}
	retired = true
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || queue.Counts.Unknown != 0 || queue.Items[0].State != "cancelled" || *starts != 1 {
		t.Fatal("exact retired VM did not settle once", queue, err)
	}
}
func TestCloudLostLaunchResumesOnlyOwnedAttemptAfterDispatchUnpauses(t *testing.T) {
	s, mux, run, base, admitted, starts, _ := schedulerFixture(t, true)
	*admitted = true
	first := s.swarmScheduler.runtime.Dispatch
	s.swarmScheduler.runtime.Dispatch = func(ctx context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
		if _, err := first(ctx, req); err != nil {
			return SwarmDispatchResult{}, err
		}
		return SwarmDispatchResult{State: "unknown", ReasonCode: "cloud_create_unconfirmed"}, nil
	}
	resumes := 0
	s.swarmScheduler.runtime.ResumeCloud = func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
		resumes++
		current, err := s.GetProjectSwarmRun(req.ProjectID, req.RunID)
		if err != nil || len(current.Operations) != 1 || current.Operations[0].ID != req.OperationID {
			t.Fatal("cloud resume lost its exact retained operation", err)
		}
		return SwarmDispatchResult{State: "running", SessionID: current.Operations[0].ChildID}, nil
	}
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("initial cloud attempt was not retained", err, *starts)
	}
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err == nil {
		ledger.Paused = true
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || resumes != 0 || *starts != 1 {
		t.Fatal("paused dispatch resumed an upstream cloud process", err, resumes, *starts)
	}
	s.swarmScheduler.mu.Lock()
	ledger, err = s.readSwarmScheduler()
	if err == nil {
		ledger.Paused = false
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || resumes != 1 || *starts != 1 {
		t.Fatal("exact cloud attempt did not resume without new allocation", err, resumes, *starts)
	}
}
func TestSwarmSchedulerInterruptedLaunchReopensWithoutReplay(t *testing.T) {
	s, mux, run, base, admitted, starts, _ := schedulerFixture(t)
	*admitted = true
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	s.swarmScheduler.runtime.Dispatch = func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
		current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		p, err := s.resolveProject(run.ProjectID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		_, _, claimed, err := s.projectRuns.ClaimOperation(p.ID, p.Path, current.ID, current.Revision, req.OperationID)
		if !claimed || err != nil {
			t.Fatalf("claim failed: %v", err)
		}
		*starts++
		return SwarmDispatchResult{}, errors.New("interrupted after durable launch claim")
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Unknown != 1 {
		t.Fatalf("lost unknown: %+v %v", snapshot, err)
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.swarmScheduler.runtime = s.swarmScheduler.runtime
	reopened.projectRunSessionObserver = s.projectRunSessionObserver
	reopened.projectRunSelectionResolver = s.projectRunSelectionResolver
	for i := 0; i < 2; i++ {
		if err = reopened.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if *starts != 1 {
		t.Fatalf("replayed unknown launch %d times", *starts)
	}
	after, err := reopened.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || after.Items[0].ChildID != snapshot.Items[0].ChildID || after.Counts.Unknown != 1 {
		t.Fatalf("lost durable child: %+v %v", after, err)
	}
}
func TestSwarmSchedulerProjectFairnessAgingAndScopedProjection(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	makeItem := func(id, project, priority string, age time.Duration) swarmQueueRecord {
		return swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{ID: id, ProjectID: project, Priority: priority, State: "queued", Revision: 1, QueuedAt: now.Add(-age).Format(time.RFC3339Nano), UpdatedAt: now.Add(-age).Format(time.RFC3339Nano)}}
	}
	row := swarmSchedulerLedger{Revision: 18, PolicyRevision: 2, LastServed: map[string]uint64{"project-a": 4, "project-b": 1}, Items: []swarmQueueRecord{makeItem("a-new", "project-a", "high", time.Minute), makeItem("b-new", "project-b", "high", time.Minute), makeItem("b-old", "project-b", "low", 20*time.Minute), makeItem("c-first", "project-c", "low", time.Minute)}}
	ordered := orderedSwarmQueue(row, now)
	want := []string{"c-first", "b-old", "b-new", "a-new"}
	for i, item := range ordered {
		if item.ID != want[i] {
			t.Fatalf("fair order %+v", ordered)
		}
	}
	snapshot := queueProjectSnapshot(row, "project-b")
	if len(snapshot.Items) != 2 || snapshot.Counts.Queued != 2 {
		t.Fatalf("scoped projection %+v", snapshot)
	}
	other := row
	other.Revision++
	other.Items = append(append([]swarmQueueRecord{}, row.Items...), makeItem("secret", "private-project", "high", 0))
	if !reflect.DeepEqual(snapshot, queueProjectSnapshot(other, "project-b")) {
		t.Fatal("another project's activity leaked into scoped queue revision/timestamp")
	}
}
func TestSwarmSchedulerItemControlsDoNotConflictWithOtherItems(t *testing.T) {
	s, mux, run, base, _, _, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	controls := "/v1/product/projects/" + run.ProjectID + "/swarm-queue/"
	journeyRequest(t, mux, "POST", controls+items[0].ID+"/pause", map[string]int{"expectedRevision": items[0].Revision}, 200)
	journeyRequest(t, mux, "POST", controls+items[1].ID+"/cancel", map[string]int{"expectedRevision": items[1].Revision}, 200)
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Paused != 1 || snapshot.Counts.Cancelled != 1 {
		t.Fatalf("controls %+v %v", snapshot, err)
	}
	journeyRequest(t, mux, "POST", controls+items[0].ID+"/resume", map[string]int{"expectedRevision": snapshot.Items[0].Revision}, 200)
	journeyRequest(t, mux, "POST", base+"/queue", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "explicit-reviewed-batch", "priority": "high", "taskIds": []string{"task-a", "task-b"}}, 409)
}

func TestSwarmSchedulerExitRetainsUnconfirmedResourcesAndRetriesRelease(t *testing.T) {
	s, mux, run, base, admitted, starts, children := schedulerFixture(t)
	*admitted = true
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	op := current.Operations[0]
	child := children[op.ID]
	child.Status = registry.StatusExited
	child.ExitReason = "exited"
	children[op.ID] = child
	s.swarmScheduler.runtime.Release = func(context.Context, string) error { return errors.New("backend still reports running") }
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Unknown != 1 || snapshot.Items[0].MemoryMiB != 1024 || snapshot.Items[0].ReasonCode != "resource_release_pending" {
		t.Fatalf("freed unconfirmed capacity: %+v %v", snapshot, err)
	}
	s.swarmScheduler.runtime.Release = func(context.Context, string) error { return nil }
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Completed != 1 || snapshot.Items[0].MemoryMiB != 0 || *starts != 1 {
		t.Fatalf("release recovery: %+v %v starts=%d", snapshot, err, *starts)
	}
}
func TestSwarmSchedulerRetiredAllocationCanBeExplicitlyQueuedAgain(t *testing.T) {
	s, mux, run, base, _, _, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	current, op, _, err := s.projectRuns.ReserveSwarmTask(p.ID, p.Path, run.ID, run.Revision, "scheduler-"+items[0].ID, "task-a", run.Input.Implementation.SourceCommit, func(string) string { return "wt-controlled-interruption" })
	if err != nil {
		t.Fatal(err)
	}
	if err = s.schedulerUpdate(items[0].ID, func(item *swarmQueueRecord) error {
		item.OperationID = op.ID
		item.ChildID = op.ChildID
		item.State = "claiming"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Unknown != 1 {
		t.Fatalf("allocation replayed %+v %v", snapshot, err)
	}
	retired := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", map[string]int{"expectedRevision": current.Revision}, 200)).ProjectRun
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Cancelled != 1 {
		t.Fatalf("retirement did not recover queue: %+v %v", snapshot, err)
	}
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/queue", map[string]any{"expectedRevision": retired.Revision, "idempotencyKey": "fresh-explicit-consent", "priority": "normal"}, 201)
}
func TestSwarmSchedulerCorruptOrCopiedLedgerNeverRepairsOrDispatches(t *testing.T) {
	for _, variant := range []string{"truncated", "foreign-account", "duplicate-task"} {
		t.Run(variant, func(t *testing.T) {
			s, mux, run, base, admitted, starts, _ := schedulerFixture(t)
			*admitted = true
			enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
			path := filepath.Join(s.stateDir, "swarm-scheduler.json")
			if variant == "truncated" {
				if err := os.WriteFile(path, []byte(`{"version":1`), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				s.swarmScheduler.mu.Lock()
				row, err := s.readSwarmScheduler()
				s.swarmScheduler.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				if variant == "foreign-account" {
					row.AccountScope = reviewDigest("another-account")
				} else {
					duplicate := row.Items[0]
					duplicate.ID = "queue-other"
					duplicate.KeyDigest = reviewDigest("another-key")
					row.Items = append(row.Items, duplicate)
				}
				if err = atomicJSON(path, row); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ProjectSwarmQueueSnapshot(run.ProjectID); err == nil {
				t.Fatal("accepted corrupt queue")
			}
			if err = s.RunSwarmSchedulerOnce(context.Background()); err == nil {
				t.Fatal("dispatched corrupt queue")
			}
			after, err := os.ReadFile(path)
			if err != nil || !reflect.DeepEqual(before, after) || *starts != 0 {
				t.Fatal("corrupt queue was repaired or dispatched", err)
			}
		})
	}
}

func TestSwarmSchedulerRefusalAfterClaimRemainsUnknown(t *testing.T) {
	s, mux, run, base, admitted, _, _ := schedulerFixture(t)
	*admitted = true
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	released := 0
	s.swarmScheduler.runtime.Release = func(context.Context, string) error { released++; return nil }
	s.swarmScheduler.runtime.Dispatch = func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
		current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		p, err := s.resolveProject(run.ProjectID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		_, op, claimed, err := s.projectRuns.ClaimOperation(p.ID, p.Path, current.ID, current.Revision, req.OperationID)
		if !claimed || err != nil {
			t.Fatal("missing durable claim", err)
		}
		return SwarmDispatchResult{State: "blocked", SessionID: op.ChildID, ReasonCode: "context_storage", Message: "Context storage failed after claim."}, nil
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Unknown != 1 || released != 0 {
		t.Fatalf("claimed refusal lost reservation %+v %v released%d", snapshot, err, released)
	}
	if err = s.schedulerSetReason(items[0].ID, "dispatching", "dispatching", "Starting the exact child."); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Items[0].State != "reserved" || snapshot.Counts.Reserved != 1 {
		t.Fatalf("public phase leaked internal claim: %+v %v", snapshot, err)
	}
}

func TestSwarmSchedulerRunCancellationUpdatesQueueWhileGloballyPaused(t *testing.T) {
	s, mux, run, base, _, starts, _ := schedulerFixture(t)
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	journeyRequest(t, mux, "PUT", "/v1/product/swarm-scheduler", map[string]any{"expectedRevision": 1, "policy": defaultSwarmSchedulerPolicy(), "dispatchPaused": true}, 200)
	journeyRequest(t, mux, "POST", base+"/cancel", map[string]int{"expectedRevision": run.Revision}, 200)
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Cancelled != 2 || !snapshot.DispatchPaused || *starts != 0 {
		t.Fatalf("cancelled work remained queued %+v %v", snapshot, err)
	}
}

func TestSwarmSchedulerFourBlockedProjectsBoundWritesAndWakePromptly(t *testing.T) {
	s, mux, first, base, _, _, _ := schedulerFixture(t)
	originalResolver := s.projectRunSelectionResolver
	routes := map[string]ProjectRunSelection{}
	s.projectRunSelectionResolver = func(ctx context.Context, req ProjectRunSelectionRequest) (ProjectRunSelection, error) {
		if route, ok := routes[req.ProjectID]; ok {
			return route, nil
		}
		return originalResolver(ctx, req)
	}
	runs := []ProjectRun{first}
	bases := []string{base}
	for _, name := range []string{"pressure-two", "pressure-three", "pressure-four"} {
		p := createJourneyProject(t, mux, name)
		spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
		path := "/v1/product/projects/" + p.ID
		var plan ProjectPlan
		decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", path+"/plan", map[string]any{"expectedRevision": 0, "expectedSpecificationRevision": spec.Revision, "tasks": []ProjectTask{{ID: "task-a", Title: "Build the reviewed change", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"result.txt"}}}}, 200).Body.Bytes(), &plan)
		journeyRequest(t, mux, "POST", path+"/git/commit", reviewedCommitRequest(t, mux, path+"/git/commit", "Save the isolated pressure fixture"), 200)
		route := first.Input.Route
		routes[p.ID] = ProjectRunSelection{ProjectID: p.ID, WorktreeID: project.WorktreeID(p.Path), Path: p.Path, Harness: route.Harness, Model: route.Model, PermissionMode: route.PermissionMode, AgentConnectionID: route.Connection.ID, AgentConnectionRevision: route.Connection.Revision, AgentConnectionKind: route.Connection.Kind, AgentConnectionProtocol: route.Connection.Protocol, AuthMode: route.Connection.AuthMode, Connection: route.Connection}
		id := route.Connection.ID
		req := projectRunCreateRequest{IdempotencyKey: "create-pressure-" + name, WorktreeID: project.WorktreeID(p.Path), Harness: route.Harness, Model: route.Model, PermissionMode: route.PermissionMode, AgentConnectionID: &id, Outcome: "preview", ImplementationContext: ImplementationContextRequest{ProjectID: p.ID, BriefRevision: spec.BriefRevision, PlanRevision: plan.Revision, SpecificationRevision: spec.Revision, DesignPlanRevision: design.Revision, TaskIDs: []string{"task-a"}}}
		run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", path+"/runs", req, 201)).ProjectRun
		childBase := path + "/runs/" + run.ID + "/swarm"
		run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", childBase, map[string]int{"expectedRevision": run.Revision, "maxParallel": 2}, 201)).ProjectRun
		runs = append(runs, run)
		bases = append(bases, childBase)
	}
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	s.swarmScheduler.now = func() time.Time { return now }
	probes, starts := 0, 0
	admitted := false
	startedProjects := map[string]bool{}
	s.swarmScheduler.runtime.Admit = func(context.Context, SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
		probes++
		if !admitted {
			return SwarmAdmissionResult{State: "waiting", ReasonCode: "memory_pressure", Message: "Waiting for memory headroom."}, nil
		}
		return SwarmAdmissionResult{State: "admitted", MemoryMiB: 1024, CPUMillis: 1000}, nil
	}
	s.swarmScheduler.runtime.Dispatch = func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
		p, err := s.resolveProject(req.ProjectID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		run, err := s.GetProjectSwarmRun(req.ProjectID, req.RunID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		_, op, claimed, err := s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, req.OperationID)
		if err != nil || !claimed {
			t.Fatal("missing exact claim", err)
		}
		starts++
		startedProjects[p.ID] = true
		return SwarmDispatchResult{State: "running", SessionID: op.ChildID}, nil
	}
	for i, run := range runs {
		enqueueSchedulerFixture(t, mux, run, bases[i], []string{"task-a"})
	}
	s.swarmScheduler.mu.Lock()
	initial, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	// 200 explicit polls at100ms represent20 seconds of persistent pressure.
	// Without cooldown these would write1,600 claim/wait transitions.
	for turn := 0; turn < 200; turn++ {
		if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		now = now.Add(100 * time.Millisecond)
	}
	s.swarmScheduler.mu.Lock()
	after, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if probes != 20 || after.Revision-initial.Revision != 40 || starts != 0 {
		t.Fatalf("pressure caused unbounded churn: probes%d revisions%d starts%d", probes, after.Revision-initial.Revision, starts)
	}
	if _, err = os.Stat(filepath.Join(s.projectRoot(), ".luci-swarms")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("pressure allocated checkout", err)
	}
	// Refresh pressure at20s to enter cooldown, then clear pressure and wake;
	// explicit operator intent must not wait out that cooldown.
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	admitted = true
	s.WakeSwarmScheduler()
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if starts != 4 || len(startedProjects) != 4 {
		t.Fatalf("wake did not promptly advance four distinct projects: starts%d projects%d", starts, len(startedProjects))
	}
}
