package product

import (
	"context"
	"net/http"
	"testing"
)

// An accepted amendment changes the original approved contract. The saved
// route remains readable, but cannot approve the replacement run or integrate
// its work by the legacy nil-contract fallback.
func TestAcceptedAmendmentWaitsForFreshExecutionContract(t *testing.T) {
	s, p, h, ctx, req, _ := coordinatorContinuationFixture(t)
	base := "/v1/product/projects/" + p.ID
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	tasks := currentPlan(state, p.ID).Tasks
	tasks[0].WritePaths = []string{"result.txt"}
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, h, http.MethodPost, base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, http.StatusOK).Body.Bytes(), &plan)
	commitRoute := base + "/git/commit"
	journeyRequest(t, h, http.MethodPost, commitRoute, reviewedCommitRequest(t, h, commitRoute, "Review task write scope"), http.StatusOK)
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, http.MethodPost, base+"/orchestrator/policy", map[string]any{"expectedRevision": req.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "integrationMode": "reviewed-scopes"}, http.StatusOK).Body.Bytes(), &policy)
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedContextDigest, req.ExpectedPolicyRevision = view.Context.Digest, policy.Revision
	req.Run.ImplementationContext.PlanRevision = plan.Revision
	anchor := coordinatorReadyFixture(t, s, p, ctx, req)
	priorRuns, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	selection := req.Run.ImplementationContext
	binding, err := bindProjectRequirements(state, p.ID, []string{"AC-save"}, &selection.SpecificationRevision, &selection.DesignPlanRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	check := &CoordinatorCheckPlan{Commands: []CheckCommand{{Label: "Combined result", Executable: "node", Args: []string{"-e", "console.log('checked')"}}}, RequirementIDs: binding.RequirementIDs, RequirementSnapshots: binding.RequirementSnapshots, AcknowledgeProjectCommands: true}
	installControlledProjectCheckRuntime(t, s, func(context.Context, ProjectCheckCommandRequest) (string, error) { return "checked", nil })
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		rows.Rows[0].Status = "blocked"
		rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, CheckPlan: check, UpdatedAt: anchor.CreatedAt}
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Improve this implementation while building", "")
	amended := snapshot.Plan
	amended.Tasks[0].Acceptance = "The amended implementation passes combined checks."
	answer := OrchestratorAnswer{Summary: "Amend task", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Amend task", Plan: &amended}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err = s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	for i := 0; i < 2; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 || rows.Rows[0].ID != anchor.ID {
		t.Fatalf("changed plan reused old contract or minted work: %+v %v", rows, err)
	}
	runs, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil || len(runs) != len(priorRuns) {
		t.Fatal("accepted event created a new run without approval", runs, err)
	}
	for i := range runs {
		if runs[i].ID != priorRuns[i].ID {
			t.Fatal("accepted event replaced a run without approval", runs, priorRuns)
		}
	}
}
