package product

import (
	"context"
	"errors"
	"testing"
)

func TestCoordinatorDelegatedReviewRenewsOnlyExactPreviouslyApprovedAmendment(t *testing.T) {
	s, p, h, ctx, req, _ := coordinatorContinuationFixture(t)
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
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{
		"expectedRevision": req.ExpectedPolicyRevision, "mode": "auto-drafts", "reviewMode": "approved-amendments", "allowedTargets": []string{"specification"},
	}, 200).Body.Bytes(), &policy)
	if policy.ReviewMode != "approved-amendments" {
		t.Fatal("delegated review was not saved")
	}
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Improve the approved definition", "")
	spec := snapshot.Specification
	spec.Purpose = "Keep saved work with a clear review history"
	spec.Status, spec.Digest = "draft", ""
	answer := OrchestratorAnswer{Summary: "Refine the approved purpose", Options: []string{}, Impact: []string{"Definition review"}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Purpose amendment", Specification: &spec}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || state.Specification == nil || state.Specification.Status != "draft" {
		t.Fatal("draft was not saved before review", err)
	}
	currentAuthorizer := s.coordinatorAuthorize
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("revoked") })
	rows, err = s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := s.continueSavedCoordinatorReview(context.Background(), p, rows, coordinatorFixtureReauthorize); changed || err != nil {
		t.Fatal("revoked edit authority reviewed an amendment", err)
	}
	state, err = s.readApprovedJourney(p.Path, p.ID)
	if err != nil || state.Specification.Status != "draft" {
		t.Fatal("revoked review mutated context", err)
	}
	s.SetCoordinatorAuthorizer(currentAuthorizer)
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err = s.readApprovedJourney(p.Path, p.ID)
	if err != nil || !specificationApprovalMatches(state) || state.Specification.Purpose != spec.Purpose || state.approvalAuthority.Specification.DelegatedPolicyRevision != policy.Revision {
		t.Fatal("exact amended definition was not approved under saved policy", err)
	}
	revision, receipt := state.Specification.Revision, state.approvalAuthority.Specification.ID
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || after.Specification.Revision != revision || after.approvalAuthority.Specification.ID != receipt {
		t.Fatal("review replayed", err)
	}
}

func TestCoordinatorAnsweredQuestionResumesSavedApprovedRouteOnce(t *testing.T) {
	s, p, h, ctx, req, _ := coordinatorContinuationFixture(t)
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
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{
		"expectedRevision": req.ExpectedPolicyRevision, "mode": "auto-drafts", "reviewMode": "approved-amendments", "allowedTargets": []string{"specification"},
	}, 200).Body.Bytes(), &CoordinatorPolicy{})
	question, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Should private work be easier to find?", "")
	if err = s.CompleteOrchestratorTurn(p.ID, question.ID, question.JobID, "succeeded", &OrchestratorAnswer{
		Summary: "The navigation choice needs a decision.", Question: "Show private work first?", Rationale: "This affects the entry point.",
		Options: []string{"Yes", "No"}, Blocking: true, Impact: []string{"Definition"},
	}); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	blocked, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || len(blocked.BlockingQuestions) != 1 || blocked.Turns[len(blocked.Turns)-1].AppliedRevision != 0 {
		t.Fatalf("unanswered decision changed saved context: %+v %v", blocked, err)
	}
	reply, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Yes. Show my saved work first.", question.ID)
	spec := snapshot.Specification
	spec.Purpose = "Find private saved work before starting something new"
	spec.Status, spec.Digest = "draft", ""
	if err = s.CompleteOrchestratorTurn(p.ID, reply.ID, reply.JobID, "succeeded", &OrchestratorAnswer{
		Summary: "Put saved work first.", Options: []string{}, Impact: []string{"Definition"},
		Proposal: &OrchestratorProposal{Target: "specification", Summary: "Saved work entry", Specification: &spec},
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	resumed, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || len(resumed.BlockingQuestions) != 0 || resumed.Activity == nil || len(resumed.Activity.Accepted) != 1 ||
		resumed.Activity.Accepted[0].OriginTurnID != reply.ID || resumed.Activity.Accepted[0].AppliedTargetDigest == "" {
		t.Fatalf("answered decision did not resume exact reviewed change: %+v %v", resumed.Activity, err)
	}
	acceptedID := resumed.Activity.Accepted[0].EventID
	for i := 0; i < 2; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	replayed, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || len(replayed.Activity.Accepted) != 1 || replayed.Activity.Accepted[0].EventID != acceptedID {
		t.Fatalf("clarification replay created a second accepted event: %+v %v", replayed.Activity, err)
	}
}

func TestCoordinatorDelegatedReviewRejectsUnreviewedBaseAndPolicyChange(t *testing.T) {
	s, p, h, ctx := coordinatorPolicyFixture(t)
	policy := coordinatorSavePolicy(t, h, p.ID, 0, "auto-drafts", []string{"specification"})
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	gate := coordinatorDelegatedReviewGate{ActorID: "alice", TurnID: "turn-missing", PolicyRevision: policy.Revision, TargetRevision: 1, TargetDigest: reviewDigest(state.Specification)}
	s.mu.RLock()
	err = s.validateCoordinatorDelegatedReviewLocked(ctx, p, state, gate, "specification", gate.TargetRevision, gate.TargetDigest)
	s.mu.RUnlock()
	if err == nil {
		t.Fatal("manual policy or missing turn conferred review")
	}
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{
		"expectedRevision": policy.Revision, "mode": "review", "reviewMode": "approved-amendments", "allowedTargets": []string{"specification"},
	}, 409)
}

func TestCoordinatorDelegatedDesignReviewPreservesDefinitionDependency(t *testing.T) {
	s, p, h, ctx, req, _ := coordinatorContinuationFixture(t)
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
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{
		"expectedRevision": req.ExpectedPolicyRevision, "mode": "auto-drafts", "reviewMode": "approved-amendments", "allowedTargets": []string{"design-plan"},
	}, 200).Body.Bytes(), &policy)
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Improve the reviewed design", "")
	design := snapshot.DesignPlan
	design.Status, design.Digest = "draft", ""
	design.Screens[0].Purpose = "Make saved work easier to reopen"
	answer := OrchestratorAnswer{Summary: "Improve the existing home screen", Options: []string{}, Impact: []string{"Design review"}, Proposal: &OrchestratorProposal{Target: "design-plan", Summary: "Home amendment", DesignPlan: &design}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || state.DesignPlan.Status != "draft" || !specificationApprovalMatches(state) {
		t.Fatal("design draft changed approved definition", err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err = s.readApprovedJourney(p.Path, p.ID)
	if err != nil || !designApprovalMatches(state) || state.DesignPlan.Screens[0].Purpose != design.Screens[0].Purpose || state.approvalAuthority.Design.DelegatedPolicyRevision != policy.Revision {
		t.Fatal("design review lost its exact definition and prior design binding", err)
	}
}
