package product

import (
	"context"
	"net/http"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// The upstream coding exit is controlled; the amended context commit, changed
// task reservation, candidate Git integration and independent check reservation
// use the production product paths. This is not a model or Docker witness.
func TestAcceptedAmendmentReachesReviewedIntegrationAndIndependentCheck(t *testing.T) {
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
	if err != nil || len(rows.Rows) != 2 || len(rows.Rows[1].AcceptedEvents) != 1 || rows.Rows[1].Status != "queued" {
		t.Fatalf("amendment did not reserve affected work: %+v %v", rows, err)
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, rows.Rows[1].RunID)
	if err != nil || run.Input.Implementation.PlanRevision != snapshot.Plan.Revision+1 {
		t.Fatal("amendment did not change exact upstream input", err)
	}
	swarmBase := base + "/runs/" + run.ID + "/swarm"
	run = projectRunHTTPResponse(t, journeyRequest(t, h, http.MethodPost, swarmBase+"/tasks/task-save/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "amended-candidate-123456"}, http.StatusCreated)).ProjectRun
	op := run.Operations[len(run.Operations)-1]
	workerPath := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, workerPath, "result.txt", "independently integrated implementation\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || run.Swarm.Tasks[0].Integration == nil || run.Swarm.Tasks[0].Integration.State != "integrated" {
		t.Fatal("amended candidate did not integrate", err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || rows.Rows[1].Delivery == nil || rows.Rows[1].Delivery.CheckID == "" {
		t.Fatal("amended combined result did not reserve independent check", err)
	}
}
