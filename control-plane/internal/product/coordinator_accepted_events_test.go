package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

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
