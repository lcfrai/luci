package product

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func findingTestAuthority(ctx context.Context, ref management.ExecutionActorRef, harness string, _ bool) (context.Context, error) {
	if ref.UserID != "alice" && ref.UserID != "bob" || ref.Local || harness != "codex" {
		return nil, errCoordinatorAuthority
	}
	return management.WithExecutionAuthority(ctx, management.ExecutionAuthority{Actor: management.User{ID: ref.UserID, Role: "member"}}), nil
}

func findingTestSource(t *testing.T, s *Server, p project.Project, row coordinatorContinuationRecord, withBob bool) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	alice, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	alice.Rows[0].Status, alice.Rows[0].Message = "blocked", "The exact independent check needs a reviewed design clarification."
	alice.Rows[0].Revision++
	if err = s.saveContinuations(alice); err != nil {
		t.Fatal(err)
	}
	if !withBob {
		return
	}
	bob, err := s.readContinuations(p, "bob")
	if err != nil {
		t.Fatal(err)
	}
	peer := row
	peer.ID, peer.IdempotencyKey = newID("continue-"), "peer-finding-context-12345678"
	peer.KeyDigest, peer.RequestDigest = reviewDigest(peer.IdempotencyKey), reviewDigest("peer-request")
	peer.Actor = ContextActor{ID: "bob", DisplayName: "Bob"}
	peer.ExecutionActor.UserID = "bob"
	peer.Status, peer.Message, peer.Actions = "refused", "Prior peer work was refused.", nil
	bob.Rows = append(bob.Rows, peer)
	if err = s.saveContinuations(bob); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinatorFindingTwoRecipientsSurviveRestartAndRevocation(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	findingTestSource(t, s, p, row, true)
	s.coordinatorBackground.authorize = findingTestAuthority
	s.coordinatorBackground.dispatch = func(context.Context, management.ExecutionActorRef, CoordinatorFindingDispatch) (string, error) {
		t.Fatal("automatic model discussion was not opted in")
		return "", nil
	}
	for i := 0; i < 8; i++ {
		if err := s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for _, actorID := range []string{"alice", "bob"} {
		rows, err := s.readContinuations(p, actorID)
		if err != nil || len(rows.Findings) != 1 || rows.Findings[0].RecipientID != actorID || rows.Findings[0].ContinuationID != row.ID {
			t.Fatalf("finding was not bound to %s: %+v %v", actorID, rows.Findings, err)
		}
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, actorID := range []string{"alice", "bob"} {
		fresh := management.WithExecutionAuthority(context.Background(), management.ExecutionAuthority{Actor: management.User{ID: actorID, Role: "member"}})
		view, err := reopened.OrchestratorView(fresh, p.ID, 0)
		if err != nil || view.Activity == nil || len(view.Activity.Findings) != 1 || view.Activity.Findings[0].RecipientID != actorID {
			t.Fatalf("restart lost recipient %s: %+v %v", actorID, view.Activity, err)
		}
	}
	// A later source revision is not delivered to a removed member. The
	// gateway also refuses their project conversation read at consumption.
	s.mu.Lock()
	alice, _ := s.readContinuations(p, "alice")
	alice.Rows[0].Revision++
	alice.Rows[0].Message = "A further exact check needs review."
	if err = s.saveContinuations(alice); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	s.coordinatorBackground.authorize = func(ctx context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
		if ref.UserID == "bob" {
			return nil, management.ErrExecutionAuthority
		}
		return findingTestAuthority(ctx, ref, harness, secrets)
	}
	for i := 0; i < 8; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	bob, err := s.readContinuations(p, "bob")
	if err != nil || len(bob.Findings) != 1 {
		t.Fatal("revoked recipient received a later private finding", err)
	}
}

func TestCoordinatorVerifiedStagesReachBothConversationsWithoutModelReplay(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	findingTestSource(t, s, p, row, true)
	path, err := s.continuationPath(p.ID, "alice", false)
	if err != nil {
		t.Fatal(err)
	}
	bobPath, err := s.continuationPath(p.ID, "bob", false)
	if err != nil {
		t.Fatal(err)
	}
	root, names := filepath.Dir(path), []string{filepath.Base(path), filepath.Base(bobPath)}
	for index, stage := range []string{"checks-verified", "preview-ready"} {
		source, readErr := s.readContinuations(p, "alice")
		if readErr != nil {
			t.Fatal(readErr)
		}
		source.Rows[0].Status, source.Rows[0].Message = stage, "Exact reviewed stage "+stage+" is available for this run."
		source.Rows[0].Revision += index + 1
		for _, actorID := range []string{"alice", "bob"} {
			changed, deliverErr := s.deliverCoordinatorFindings(context.Background(), p, source, root, names, findingTestAuthority, nil)
			if deliverErr != nil || !changed {
				t.Fatal("stage was not delivered to current reader", stage, actorID, changed, deliverErr)
			}
			found, readErr := s.readContinuations(p, actorID)
			if readErr != nil || len(found.Findings) != index+1 || found.Findings[index].SourceStatus != stage || found.Findings[index].Status != "observed" || !validCoordinatorFinding(found.Findings[index], p.ID, actorID) {
				t.Fatal("stage was not an exact read-only conversation observation", stage, actorID, found.Findings, readErr)
			}
		}
		if changed, deliverErr := s.deliverCoordinatorFindings(context.Background(), p, source, root, names, findingTestAuthority, nil); deliverErr != nil || changed {
			t.Fatal("stage was duplicated after durable readback", stage, changed, deliverErr)
		}
	}
}

func TestCoordinatorFindingSeedsCurrentReaderWithoutCodingRoute(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	findingTestSource(t, s, p, row, false)
	s.coordinatorBackground.authorize = findingTestAuthority
	s.coordinatorBackground.dispatch = func(context.Context, management.ExecutionActorRef, CoordinatorFindingDispatch) (string, error) {
		t.Fatal("read-only teammate cannot authorize model spending")
		return "", nil
	}
	bob := management.ExecutionActorRef{UserID: "bob", ProjectID: p.ID}
	s.coordinatorBackground.recipients = func(_ context.Context, source management.ExecutionActorRef) ([]management.ExecutionActorRef, error) {
		if source.UserID != "alice" {
			return nil, management.ErrExecutionAuthority
		}
		return []management.ExecutionActorRef{source, bob}, nil
	}
	revoked := false
	s.coordinatorBackground.readFinding = func(_ context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if ref.UserID == "bob" && revoked {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{Actor: management.User{ID: ref.UserID, Role: "member"}}, nil
	}
	for i := 0; i < 12; i++ {
		if err := s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.readContinuations(p, "bob")
	if err != nil || len(rows.Rows) != 0 || rows.RecipientRef != bob || len(rows.Findings) != 1 || rows.Findings[0].ContinuationID != row.ID {
		t.Fatalf("current reader without coding route missed finding: %+v %v", rows, err)
	}
	revoked = true
	s.mu.Lock()
	alice, _ := s.readContinuations(p, "alice")
	alice.Rows[0].Revision++
	alice.Rows[0].Message = "A later exact finding needs review."
	if err = s.saveContinuations(alice); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	for i := 0; i < 12; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = s.readContinuations(p, "bob")
	if err != nil || len(rows.Findings) != 1 {
		t.Fatal("revoked reader received later content", err)
	}
}

func TestCoordinatorFindingOneModelReservationAndUnknownNoReplay(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "question", true: "lost-ack"}[failure], func(t *testing.T) {
			s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
			row := coordinatorReadyFixture(t, s, p, ctx, req)
			findingTestSource(t, s, p, row, false)
			s.mu.Lock()
			rows, err := s.readContinuations(p, "alice")
			if err != nil {
				t.Fatal(err)
			}
			rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, AutoDiscussFindings: true, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: row.CreatedAt}
			source := rows.Rows[0]
			rows.Findings = []CoordinatorFinding{{ID: findingIdentity(source, "alice"), ProjectID: p.ID, RecipientID: "alice", ContinuationID: source.ID, OriginTurnID: source.OriginTurnID, ContextDigest: source.ContextDigest, SourceRevision: source.Revision, SourceStatus: source.Status, Status: "pending", Message: source.Message, UpdatedAt: source.UpdatedAt}}
			if err = s.saveContinuations(rows); err != nil {
				t.Fatal(err)
			}
			policy, err := s.readCoordinatorPolicy(p)
			if err != nil {
				t.Fatal(err)
			}
			policy.Revision, policy.Mode = 1, "auto-drafts"
			scope, err := s.approvalScopes(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			path, err := s.coordinatorPolicyPath(p.ID, true)
			if err != nil {
				t.Fatal(err)
			}
			if err = durableRunJSON(path, storedCoordinatorPolicy{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Policy: policy}); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			calls := 0
			dispatch := func(fresh context.Context, ref management.ExecutionActorRef, input CoordinatorFindingDispatch) (string, error) {
				calls++
				if ref.UserID != "alice" || input.ExpectedContextDigest == "" || input.RequestKey == "" || !strings.Contains(input.Message, source.ID) {
					t.Fatal("model request lost exact authority or source binding")
				}
				if failure {
					return "", errors.New("response lost after uncertain allocation")
				}
				turn, _, _, err := s.ReserveOrchestratorTurn(fresh, p.ID, OrchestratorTurnRequest{ExpectedRevision: input.ExpectedRevision, ExpectedContextDigest: input.ExpectedContextDigest, IdempotencyKey: input.RequestKey, Message: input.Message, RouteDigest: strings.Repeat("a", 64)}, newID("assist-"))
				if err != nil {
					t.Fatal(err)
				}
				return turn.ID, s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &OrchestratorAnswer{Summary: "A design decision is needed.", Question: "Keep the existing layout?", Rationale: "The observed check found an ambiguity.", Blocking: true, Options: []string{"Yes", "No"}, Impact: []string{"Design"}})
			}
			if followed, err := s.continueCoordinatorFinding(context.Background(), p, rows, findingTestAuthority, dispatch); !followed || err != nil || calls != 1 {
				t.Fatalf("one exact follow-up was not reserved: %v %v %d", followed, err, calls)
			}
			reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			retained, err := reopened.readContinuations(p, "alice")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = reopened.continueCoordinatorFinding(context.Background(), p, retained, findingTestAuthority, dispatch); err != nil || calls != 1 {
				t.Fatal("restart repeated a model call", err)
			}
			retained, _ = reopened.readContinuations(p, "alice")
			want := "question"
			if failure {
				want = "unknown"
			}
			if retained.Findings[0].Status != want {
				t.Fatalf("retained status %s, want %s", retained.Findings[0].Status, want)
			}
			if retained.Findings[0].RequestKey != "finding-"+reviewDigest([]string{retained.Findings[0].ID, "alice"})[:32] {
				t.Fatal("the interrupted model reservation lost its exact request key")
			}
			if !failure {
				question := retained.Findings[0].TurnID
				reply, _ := reserveOrchestratorTest(t, reopened, p.ID, ctx, "Yes; keep the approved layout and revise only the affected check.", question)
				if err = reopened.CompleteOrchestratorTurn(p.ID, reply.ID, reply.JobID, "succeeded", &OrchestratorAnswer{Summary: "The authorized clarification is recorded.", Options: []string{}, Impact: []string{}}); err != nil {
					t.Fatal(err)
				}
				if _, err = reopened.continueCoordinatorFinding(context.Background(), p, retained, findingTestAuthority, dispatch); err != nil || calls != 1 {
					t.Fatal("clarification caused a second model call", err)
				}
				retained, err = reopened.readContinuations(p, "alice")
				if err != nil || retained.Findings[0].Status != "resumed" {
					t.Fatalf("clarification was not bound to the saved finding: %+v %v", retained.Findings, err)
				}
			}
		})
	}
}

func TestCoordinatorAcceptedEventReplaysToTwoSavedPeopleAfterRestart(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	findingTestSource(t, s, p, row, true)
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Clarify the approved implementation", "")
	plan := snapshot.Plan
	plan.Tasks[0].Acceptance = "The revised intent is checked before delivery."
	answer := OrchestratorAnswer{Summary: "Revise acceptance", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Revise acceptance", Plan: &plan}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	for _, actorID := range []string{"alice", "bob"} {
		rows, err := s.readContinuations(p, actorID)
		if err != nil {
			t.Fatal(err)
		}
		if changed, err := s.deliverAcceptedFinding(context.Background(), p, rows, findingTestAuthority, nil); err != nil || !changed {
			t.Fatalf("accepted event did not reach %s: %v %v", actorID, changed, err)
		}
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, actorID := range []string{"alice", "bob"} {
		rows, err := reopened.readContinuations(p, actorID)
		if err != nil || len(rows.Findings) != 1 || rows.Findings[0].EventID == "" || rows.Findings[0].OriginTurnID != turn.ID || rows.FindingEventCursor < 1 {
			t.Fatalf("saved accepted event missing for %s: %+v %v", actorID, rows.Findings, err)
		}
		if changed, err := reopened.deliverAcceptedFinding(context.Background(), p, rows, findingTestAuthority, nil); err != nil || changed {
			t.Fatalf("accepted event replayed for %s: %v %v", actorID, changed, err)
		}
	}
}

func TestCoordinatorFindingRevokedAuthorityAndPolicyNeverDispatch(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	findingTestSource(t, s, p, row, false)
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	source := rows.Rows[0]
	rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, AutoDiscussFindings: true, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: row.CreatedAt}
	rows.Findings = []CoordinatorFinding{{ID: findingIdentity(source, "alice"), ProjectID: p.ID, RecipientID: "alice", ContinuationID: source.ID, OriginTurnID: source.OriginTurnID, ContextDigest: source.ContextDigest, SourceRevision: source.Revision, SourceStatus: source.Status, Status: "pending", Message: source.Message, UpdatedAt: source.UpdatedAt}}
	if err = s.saveContinuations(rows); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	calls := 0
	dispatch := func(context.Context, management.ExecutionActorRef, CoordinatorFindingDispatch) (string, error) {
		calls++
		return "", nil
	}
	if advanced, err := s.continueCoordinatorFinding(context.Background(), p, rows, findingTestAuthority, dispatch); err != nil || advanced || calls != 0 {
		t.Fatal("manual project policy admitted model spending", err)
	}
	s.mu.Lock()
	policy, _ := s.readCoordinatorPolicy(p)
	policy.Revision, policy.Mode = 1, "auto-drafts"
	scope, _ := s.approvalScopes(p.Path, p.ID)
	path, _ := s.coordinatorPolicyPath(p.ID, true)
	err = durableRunJSON(path, storedCoordinatorPolicy{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Policy: policy})
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	revoked := func(context.Context, management.ExecutionActorRef, string, bool) (context.Context, error) {
		return nil, management.ErrExecutionAuthority
	}
	if advanced, err := s.continueCoordinatorFinding(context.Background(), p, rows, revoked, dispatch); err != nil || advanced || calls != 0 {
		t.Fatal("revoked member admitted model spending", err)
	}
	if !s.findingRouteCurrent(p, "alice", 1) || s.CoordinatorFindingDispatchCurrent(p.ID, "alice", rows.Findings[0].ID, 1) {
		t.Fatal("unreserved finding passed the exact dispatch fence")
	}
	retained, err := s.readContinuations(p, "alice")
	if err != nil || retained.Findings[0].Status != "pending" {
		t.Fatal("refusal altered a retained finding", err)
	}
}

func TestCoordinatorFindingPreferenceCanBePausedWithCurrentActorAndCAS(t *testing.T) {
	s, p, handler, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: row.CreatedAt}
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/product/projects/" + p.ID + "/orchestrator/findings/preferences"
	var updated CoordinatorSavedRoute
	decodeEvidenceResponse(t, journeyRequest(t, handler, http.MethodPost, path, map[string]any{"expectedRouteRevision": 1, "autoDiscussFindings": true}, http.StatusOK).Body.Bytes(), &updated)
	if updated.Revision != 2 || !updated.AutoDiscussFindings {
		t.Fatal("current preference was not saved", updated)
	}
	journeyRequest(t, handler, http.MethodPost, path, map[string]any{"expectedRouteRevision": 1, "autoDiscussFindings": false}, http.StatusConflict)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("revoked edit") })
	journeyRequest(t, handler, http.MethodPost, path, map[string]any{"expectedRouteRevision": 2, "autoDiscussFindings": false}, http.StatusForbidden)
	retained, err := s.readContinuations(p, "alice")
	if err != nil || retained.SavedRoute == nil || retained.SavedRoute.Revision != 2 || !retained.SavedRoute.AutoDiscussFindings {
		t.Fatal("stale or revoked preference changed saved model consent", err)
	}
}
