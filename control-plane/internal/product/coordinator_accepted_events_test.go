package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestAcceptedAmendmentSelectsTransitiveDependentsAndPreservesIndependentTask(t *testing.T) {
	tasks := []ProjectTask{
		{ID: "schema"},
		{ID: "api", DependsOn: []string{"schema"}},
		{ID: "screen", DependsOn: []string{"api"}},
		{ID: "unrelated"},
	}
	events := []CoordinationEventObservation{{Intent: CoordinationIntent{TaskIDs: []string{"schema"}}}}
	if got := coordinatorAffectedTaskIDs(tasks, events); !reflect.DeepEqual(got, []string{"api", "schema", "screen"}) {
		t.Fatal("dependent old-version work was not selected for replan", got)
	}
	events[0].Intent.TaskIDs = []string{"removed"}
	if got := coordinatorAffectedTaskIDs(tasks, events); got != nil {
		t.Fatal("missing task granted an implicit new execution scope", got)
	}
}

func TestCoordinatorAcceptedPlanEventStartsOnlyAffectedReviewedWorkOnce(t *testing.T) {
	s, p, h, ctx, req, starts := coordinatorContinuationFixture(t)
	anchor := coordinatorCreate(t, h, p, req, 201)
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		rows.Rows[0].Status = "blocked" // An older build must not be replayed.
		rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: anchor.CreatedAt}
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Improve the saved task acceptance", "")
	plan := snapshot.Plan
	plan.Tasks[0].Acceptance = "The changed result is independently checked."
	answer := OrchestratorAnswer{Summary: "Changed task acceptance", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Revise one task", Plan: &plan}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	// A second person's still-pending proposal advances the event head but has
	// not changed canonical context. It must not hold the already accepted plan
	// event or force unrelated work to wait for that person's decision.
	peerContext := orchestratorActor(t, "bob")
	peer, peerBase := reserveOrchestratorTest(t, s, p.ID, peerContext, "Consider a clearer home screen", "")
	design := peerBase.DesignPlan
	design.Status, design.Digest = "draft", ""
	design.Screens[0].Purpose = "Clarify the saved-work entry point"
	peerAnswer := OrchestratorAnswer{Summary: "A possible design change", Options: []string{}, Impact: []string{"Design review"}, Proposal: &OrchestratorProposal{Target: "design-plan", Summary: "Home screen", DesignPlan: &design}}
	if err = s.CompleteOrchestratorTurn(p.ID, peer.ID, peer.JobID, "succeeded", &peerAnswer); err != nil {
		t.Fatal(err)
	}
	coordination, err := s.CoordinationViewForProject(ctx, p.ID)
	if err != nil || len(coordination.Events) < 2 || coordination.Events[len(coordination.Events)-1].Kind != "proposed" {
		t.Fatal("later independent proposal was not retained", err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.ContextCommits) != 1 || rows.ContextCommits[0].Status != "confirmed" {
		t.Fatal("accepted context did not reach exact reviewed Git commit", err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 2 || len(rows.Rows[1].AcceptedEvents) != 1 || rows.Rows[1].OriginTurnID != turn.ID || rows.Rows[1].Status != "queued" || len(rows.Rows[1].Run.ImplementationContext.TaskIDs) != 1 || rows.Rows[1].Run.ImplementationContext.TaskIDs[0] != "task-save" {
		t.Fatalf("accepted event did not reserve exact changed task: %+v %v", rows, err)
	}
	queue, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	oldBlocked := false
	for _, item := range queue.Items {
		if item.ID == anchor.QueueIDs[0] {
			oldBlocked = item.State == "blocked" && item.ReasonCode == "context_superseded"
		}
	}
	if !oldBlocked || queue.Counts.Queued != 1 || *starts != 0 {
		t.Fatal("amendment left old-version task runnable or blocked the new one", queue)
	}
	actual, err := s.projectRuns.Get(p.ID, p.Path, rows.Rows[1].RunID)
	if err != nil || actual.Input.Implementation.PlanRevision != snapshot.Plan.Revision+1 || *starts != 0 {
		t.Fatalf("changed reviewed input was not retained: %+v %v", actual, err)
	}
	// The same event after a process restart cannot reserve another run.
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.SetCoordinatorAuthorizer(s.coordinatorAuthorize)
	reopened.SetCoordinatorExecutionAuthorizer(s.coordinatorExecuteAuthorize)
	reopened.SetProjectRunSelectionResolver(s.projectRunSelectionResolver)
	reopened.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = reopened.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	retained, err := reopened.readContinuations(p, "alice")
	if err != nil || len(retained.Rows) != 2 || retained.Rows[1].ID != rows.Rows[1].ID {
		t.Fatal("restart replayed accepted event", err)
	}
	// Revoking current execution authority cannot create a second continuation.
	reopened.coordinatorBackground.authorize = func(context.Context, management.ExecutionActorRef, string, bool) (context.Context, error) {
		return nil, errCoordinatorAuthority
	}
	if err = reopened.RunCoordinatorOnce(context.Background()); err != nil && !errors.Is(err, errCoordinatorAuthority) {
		t.Fatal(err)
	}
	retained, err = reopened.readContinuations(p, "alice")
	if err != nil || len(retained.Rows) != 2 {
		t.Fatal("revoked authority changed durable work", err)
	}
}

func TestSupersededContextLeavesActiveTaskAndCurrentVersionAlone(t *testing.T) {
	s, p, h, _, req, _ := coordinatorContinuationFixture(t)
	old := coordinatorCreate(t, h, p, req, 201)
	current := req.Run.ImplementationContext
	if err := s.blockSupersededQueuedTasks(p.ID, current); err != nil {
		t.Fatal(err)
	}
	queue, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || len(queue.Items) != 1 || queue.Items[0].ID != old.QueueIDs[0] || queue.Items[0].State != "queued" {
		t.Fatal("same-version waiting task was changed", queue, err)
	}
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err == nil {
		ledger.Items[0].State = "running"
		ledger.Items[0].Revision++
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	changed := current
	changed.PlanRevision++
	if err = s.blockSupersededQueuedTasks(p.ID, changed); err != nil {
		t.Fatal(err)
	}
	queue, err = s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || len(queue.Items) != 1 || queue.Items[0].State != "running" || queue.Items[0].ReasonCode == "context_superseded" {
		t.Fatal("active owned task lost its observation path", queue, err)
	}
}

func TestAcceptedAmendmentStopsExactActiveAttemptBeforeReplanAfterRestart(t *testing.T) {
	s, p, h, ctx, req, starts := coordinatorContinuationFixture(t)
	anchor := coordinatorCreate(t, h, p, req, 201)
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("original worker did not start", err, *starts)
	}
	old, err := s.GetProjectSwarmRun(p.ID, anchor.RunID)
	if err != nil || len(old.Operations) != 1 {
		t.Fatal("original operation missing", err)
	}
	op := old.Operations[0]
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: anchor.CreatedAt}
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Change the running task's acceptance", "")
	plan := snapshot.Plan
	plan.Tasks[0].Acceptance = "The amended task has its own evidence."
	answer := OrchestratorAnswer{Summary: "Change running task", Proposal: &OrchestratorProposal{Target: "plan", Summary: "New acceptance", Plan: &plan}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	stops, settled := 0, false
	runtime := s.swarmScheduler.runtime
	runtime.StopOwned = func(_ context.Context, got SwarmSupersededRequest) (bool, error) {
		stops++
		if got.ProjectID != p.ID || got.RunID != anchor.RunID || got.QueueID != anchor.QueueIDs[0] || got.OperationID != op.ID || got.ChildID != op.ChildID || got.TaskID != op.TaskID {
			t.Fatalf("stop escaped exact old attempt: %+v", got)
		}
		return settled, nil // first lost ACK: charge and child remain uncertain
	}
	s.swarmScheduler.runtime = runtime
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	for i := 0; i < 3; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	queue, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || len(queue.Items) != 1 || queue.Items[0].State != "unknown" || queue.Items[0].ReasonCode != "context_superseded_stop_pending" || queue.Items[0].MemoryMiB != 1024 || stops == 0 {
		t.Fatal("lost stop ACK erased exact active charge or intent", queue, stops, err)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("scheduler relaunched superseded work", err, *starts)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 {
		t.Fatal("revised work reserved before exact stop", err)
	}
	s.coordinatorBackground.authorize = func(context.Context, management.ExecutionActorRef, string, bool) (context.Context, error) {
		return nil, errCoordinatorAuthority
	}
	before := stops
	if err = s.RunCoordinatorOnce(context.Background()); err != nil || stops != before {
		t.Fatal("revoked actor changed stop or admission", err, stops)
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.SetCoordinatorAuthorizer(s.coordinatorAuthorize)
	reopened.SetCoordinatorExecutionAuthorizer(s.coordinatorExecuteAuthorize)
	reopened.SetProjectRunSelectionResolver(s.projectRunSelectionResolver)
	reopened.swarmScheduler.runtime = runtime
	reopened.branchSessions = s.branchSessions
	reopened.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	reopened.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	settled = true
	for i := 0; i < 4; i++ {
		if err = reopened.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	queue, err = reopened.ProjectSwarmQueueSnapshot(p.ID)
	rows, readErr := reopened.readContinuations(p, "alice")
	if err != nil || readErr != nil || len(rows.Rows) != 2 || rows.Rows[1].OriginTurnID != turn.ID || len(queue.Items) != 2 || queue.Items[0].State != "cancelled" || queue.Items[0].MemoryMiB != 0 || queue.Items[1].State != "queued" || *starts != 1 {
		t.Fatal("confirmed stop did not admit exactly one revised run", err, readErr, queue, rows, *starts)
	}
}

func TestSupersededStopDoesNotTouchConcurrentOtherProject(t *testing.T) {
	s, p, h, _, req, _ := coordinatorContinuationFixture(t)
	anchor := coordinatorCreate(t, h, p, req, 201)
	other := createJourneyProject(t, h, "independent-project")
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err == nil {
		ledger.Items[0].State = "running"
		ledger.Items[0].MemoryMiB = 1024
		ledger.Items[0].Revision++
		peer := ledger.Items[0]
		peer.ID, peer.ProjectID, peer.Actor.ProjectID = "queue-independent-123", other.ID, other.ID
		peer.KeyDigest = reviewDigest("independent-key")
		peer.BatchKeyDigest = reviewDigest("independent-batch")
		peer.RequestDigest = reviewDigest("independent-request")
		ledger.Items = append(ledger.Items, peer)
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.ProjectSwarmQueueSnapshot(other.ID)
	if err != nil {
		t.Fatal(err)
	}
	selection := req.Run.ImplementationContext
	selection.PlanRevision++
	// Without a stop runtime this remains charged, while the other project's
	// private queue receipt is byte-for-byte unchanged.
	settled, err := s.stopSupersededActiveTasks(context.Background(), p.ID, selection)
	if err != nil || settled {
		t.Fatal("old attempt should remain pending", err, settled)
	}
	after, err := s.ProjectSwarmQueueSnapshot(other.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated project changed", err, before, after)
	}
	queue, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || len(queue.Items) != 1 || queue.Items[0].ID != anchor.QueueIDs[0] || queue.Items[0].ReasonCode != "context_superseded_identity_pending" || queue.Items[0].MemoryMiB != 1024 {
		t.Fatal("affected stop intent was not isolated", queue, err)
	}
}

func TestSupersededUnknownWithoutExactOperationCannotReleaseOrReplan(t *testing.T) {
	s, p, h, _, req, _ := coordinatorContinuationFixture(t)
	coordinatorCreate(t, h, p, req, 201)
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err == nil {
		ledger.Items[0].State = "unknown"
		ledger.Items[0].MemoryMiB = 1024
		ledger.Items[0].ReasonCode = "allocation_unknown"
		ledger.Items[0].Revision++
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	runtime := s.swarmScheduler.runtime
	runtime.StopOwned = func(context.Context, SwarmSupersededRequest) (bool, error) { calls++; return true, nil }
	s.swarmScheduler.runtime = runtime
	selection := req.Run.ImplementationContext
	selection.PlanRevision++
	settled, err := s.stopSupersededActiveTasks(context.Background(), p.ID, selection)
	queue, readErr := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || readErr != nil || settled || calls != 0 || queue.Items[0].State != "unknown" || queue.Items[0].ReasonCode != "context_superseded_identity_pending" || queue.Items[0].MemoryMiB != 1024 {
		t.Fatal("missing identity was incorrectly treated as an absent worker", err, readErr, queue, calls)
	}
}

func TestAmendmentWaitsForTerminalOldVersionLeaseProof(t *testing.T) {
	s, p, h, _, req, _ := coordinatorContinuationFixture(t)
	anchor := coordinatorCreate(t, h, p, req, 201)
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err == nil {
		ledger.Items[0].State = "completed"
		ledger.Items[0].MemoryMiB = 1024
		ledger.Items[0].CPUMillis = 1000
		ledger.Items[0].Revision++
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	releaseAllowed := false
	releases := 0
	runtime := s.swarmScheduler.runtime
	runtime.Release = func(_ context.Context, id string) error {
		if id != anchor.QueueIDs[0] {
			t.Fatalf("released a different reservation: %s", id)
		}
		releases++
		if !releaseAllowed {
			return errors.New("exact cleanup remains unconfirmed")
		}
		return nil
	}
	s.swarmScheduler.runtime = runtime
	selection := req.Run.ImplementationContext
	selection.PlanRevision++
	settled, err := s.stopSupersededActiveTasks(context.Background(), p.ID, selection)
	queue, readErr := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || readErr != nil || settled || releases != 1 || queue.Items[0].State != "completed" || queue.Items[0].MemoryMiB != 1024 {
		t.Fatal("unconfirmed terminal reservation was released or its outcome changed", err, readErr, queue, releases)
	}
	releaseAllowed = true
	settled, err = s.stopSupersededActiveTasks(context.Background(), p.ID, selection)
	queue, readErr = s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || readErr != nil || !settled || releases != 2 || queue.Items[0].State != "completed" || queue.Items[0].MemoryMiB != 0 || queue.Items[0].CPUMillis != 0 {
		t.Fatal("exact terminal lease did not settle before replanning", err, readErr, queue, releases)
	}
}

func TestStaleCoordinatorKeepsUnknownExternalMutationReceipt(t *testing.T) {
	s, p, h, _, req, _ := coordinatorContinuationFixture(t)
	anchor := coordinatorCreate(t, h, p, req, 201)
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		row := &rows.Rows[findContinuation(rows, anchor.ID)]
		row.Actions[len(row.Actions)-1].Status = "unknown"
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/plan", map[string]any{
		"expectedRevision":              req.Run.ImplementationContext.PlanRevision,
		"expectedSpecificationRevision": req.Run.ImplementationContext.SpecificationRevision,
		"tasks":                         []ProjectTask{{ID: "task-save", Title: "Implement saving", Status: "done", RequirementIDs: []string{"AC-save"}, Acceptance: "Amended independent check"}},
	}, 200)
	before, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	paused, err := s.blockStaleCoordinatorRow(p, before.Rows[0])
	after, readErr := s.readContinuations(p, "alice")
	if err != nil || readErr != nil || !paused || !reflect.DeepEqual(before, after) {
		t.Fatal("an uncertain old external mutation was overwritten or replayed", err, readErr, before, after)
	}
}

func TestCoordinatorSavedDraftContinuesWithoutBrowserAndDoesNotReplayAfterRestart(t *testing.T) {
	s, p, h, ctx, req, starts := coordinatorContinuationFixture(t)
	anchor := coordinatorReadyFixture(t, s, p, ctx, req)
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		rows.Rows[0].Status = "blocked"
		rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: anchor.CreatedAt}
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	policy := coordinatorSavePolicy(t, h, p.ID, req.ExpectedPolicyRevision, "auto-drafts", []string{"plan"})
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Improve one task from the conversation", "")
	plan := snapshot.Plan
	plan.Tasks[0].Acceptance = "The amended result has its own check."
	answer := OrchestratorAnswer{Summary: "Changed task acceptance", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Revise one task", Plan: &plan}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || view.Context.Plan.Tasks[0].Acceptance != plan.Tasks[0].Acceptance || view.Turns[len(view.Turns)-1].AppliedRevision == 0 {
		t.Fatal("saved draft policy did not continue the exact completed turn", err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 2 || rows.Rows[1].OriginTurnID != turn.ID || rows.Rows[1].PolicyRevision != policy.Revision || *starts != 0 {
		t.Fatal("accepted draft did not bind one affected reviewed continuation", err)
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.SetCoordinatorAuthorizer(s.coordinatorAuthorize)
	reopened.SetCoordinatorExecutionAuthorizer(s.coordinatorExecuteAuthorize)
	reopened.SetProjectRunSelectionResolver(s.projectRunSelectionResolver)
	reopened.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = reopened.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := reopened.readContinuations(p, "alice")
	if err != nil || len(after.Rows) != 2 || after.Rows[1].ID != rows.Rows[1].ID {
		t.Fatal("restart replayed the saved context amendment", err)
	}
}

func TestCoordinatorTwoPersonalLucisApplyCompatiblePlanIntentsWithoutBrowser(t *testing.T) {
	s, p, h, aliceContext, req, _ := coordinatorContinuationFixture(t)
	anchor := coordinatorReadyFixture(t, s, p, aliceContext, req)
	s.mu.Lock()
	alice, err := s.readContinuations(p, "alice")
	if err == nil {
		alice.Rows[0].Status = "blocked"
		alice.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: anchor.CreatedAt}
		err = s.saveContinuations(alice)
	}
	if err == nil {
		bob, readErr := s.readContinuations(p, "bob")
		err = readErr
		if err == nil {
			seed := anchor
			seed.Actor.ID, seed.Actor.DisplayName = "bob", "Bob"
			seed.ExecutionActor.UserID = "bob"
			seed.Status = "blocked"
			bob.Rows = append(bob.Rows, seed)
			bob.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: anchor.CreatedAt}
			err = s.saveContinuations(bob)
		}
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	coordinatorSavePolicy(t, h, p.ID, req.ExpectedPolicyRevision, "auto-drafts", []string{"plan"})
	bobContext := orchestratorActor(t, "bob")
	a, base := reserveOrchestratorTest(t, s, p.ID, aliceContext, "Improve the task acceptance", "")
	first := base.Plan
	first.Tasks[0].Acceptance = "The revised result is independently checked."
	firstAnswer := OrchestratorAnswer{Summary: "Compatible task change", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Revise one task field", Plan: &first}}
	if err := s.CompleteOrchestratorTurn(p.ID, a.ID, a.JobID, "succeeded", &firstAnswer); err != nil {
		t.Fatal(err)
	}
	b, peerBase := reserveOrchestratorTest(t, s, p.ID, bobContext, "Add a separate implementation task", "")
	second := peerBase.Plan
	second.Tasks = append(second.Tasks, ProjectTask{ID: "task-new", Title: "Verify a second saved-work path", Status: "todo", RequirementIDs: []string{"AC-save"}})
	secondAnswer := OrchestratorAnswer{Summary: "Compatible task change", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Revise one task field", Plan: &second}}
	if err := s.CompleteOrchestratorTurn(p.ID, b.ID, b.JobID, "succeeded", &secondAnswer); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = func(ctx context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
		if (ref.UserID != "alice" && ref.UserID != "bob") || harness != "codex" || !secrets {
			return nil, errCoordinatorAuthority
		}
		return management.WithExecutionAuthority(ctx, management.ExecutionAuthority{Actor: management.User{ID: ref.UserID, Role: "member"}}), nil
	}
	for i := 0; i < 6; i++ {
		if err := s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	view, err := s.OrchestratorView(aliceContext, p.ID, 0)
	if err != nil || view.Context.Plan.Tasks[0].Acceptance != first.Tasks[0].Acceptance || len(view.Context.Plan.Tasks) != 2 || view.Context.Plan.Tasks[1].ID != "task-new" {
		t.Fatalf("compatible teammate edits did not reconcile: tasks=%v err=%v", view.Context.Plan.Tasks, err)
	}
	for _, id := range []string{a.ID, b.ID} {
		turn, err := s.OrchestratorTurn(p.ID, id)
		if err != nil || turn.AppliedRevision == 0 {
			t.Fatalf("teammate intent was not accepted: %s %v", id, err)
		}
	}
	accepted := 0
	for _, actor := range []string{"alice", "bob"} {
		rows, err := s.readContinuations(p, actor)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows.Rows {
			if len(row.AcceptedEvents) > 0 {
				accepted++
				if len(row.AcceptedEvents) != 2 || len(row.Run.ImplementationContext.TaskIDs) != 2 {
					t.Fatalf("team changes did not select both affected tasks: %+v", row)
				}
			}
		}
	}
	if accepted != 1 {
		t.Fatalf("expected one shared continuation for both accepted intents, got %d", accepted)
	}
}

func TestCoordinatorAcceptedEventPreservesUnrelatedDirtySource(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	anchor := coordinatorReadyFixture(t, s, p, ctx, req)
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		rows.Rows[0].Status = "blocked"
		rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: anchor.CreatedAt}
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Amend one reviewed task", "")
	plan := snapshot.Plan
	plan.Tasks[0].Acceptance = "The revised task has independent evidence."
	answer := OrchestratorAnswer{Summary: "Revise task", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Revise task", Plan: &plan}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	before, err := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	unexpected := filepath.Join(p.Path, "unrelated.txt")
	if err = os.WriteFile(unexpected, []byte("user work must remain\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(after) != strings.TrimSpace(before) {
		t.Fatal("unreviewed source was committed", err)
	}
	bytes, err := os.ReadFile(unexpected)
	if err != nil || string(bytes) != "user work must remain\n" {
		t.Fatal("unreviewed source changed", err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 || len(rows.ContextCommits) != 0 {
		t.Fatal("unreviewed source reserved accepted work", err)
	}
}

func TestCoordinatorTeammateAcceptedEventUsesOneCurrentRoute(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	anchor := coordinatorReadyFixture(t, s, p, ctx, req)
	s.mu.Lock()
	alice, err := s.readContinuations(p, "alice")
	if err == nil {
		alice.Rows[0].Status = "blocked"
		err = s.saveContinuations(alice)
	}
	if err == nil {
		var bob coordinatorContinuations
		bob, err = s.readContinuations(p, "bob")
		if err == nil {
			seed := anchor
			seed.Actor.ID, seed.Actor.DisplayName = "bob", "Bob"
			seed.ExecutionActor.UserID = "bob"
			seed.Status = "blocked"
			bob.Rows = append(bob.Rows, seed)
			bob.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: anchor.CreatedAt}
			err = s.saveContinuations(bob)
		}
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Change a shared task for the team", "")
	plan := snapshot.Plan
	plan.Tasks[0].Acceptance = "A teammate's reviewed change is implemented."
	answer := OrchestratorAnswer{Summary: "Change shared task", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Change shared task", Plan: &plan}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = func(ctx context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
		if ref.UserID != "bob" || harness != "codex" || !secrets {
			return nil, errCoordinatorAuthority
		}
		return management.WithExecutionAuthority(ctx, management.ExecutionAuthority{Actor: management.User{ID: "bob", Role: "member"}}), nil
	}
	for i := 0; i < 2; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	bob, err := s.readContinuations(p, "bob")
	if err != nil || len(bob.Rows) != 2 || len(bob.Rows[1].AcceptedEvents) != 1 || bob.Rows[1].OriginTurnID != turn.ID || bob.Rows[1].Actor.ID != "bob" {
		t.Fatalf("accepted team event was not bound to one executing Luci: %+v %v", bob, err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	alice, err = s.readContinuations(p, "alice")
	if err != nil || len(alice.Rows) != 1 {
		t.Fatal("teammate event duplicated work", err)
	}
}
