package product

import (
	"context"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestCoordinatorReviewedScopeIntegratesStoppedCandidateBeforeIndependentChecks(t *testing.T) {
	s, p, h, ctx, request, _ := coordinatorContinuationFixture(t)
	base := "/v1/product/projects/" + p.ID
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	tasks := currentPlan(state, p.ID).Tasks
	tasks[0].WritePaths = []string{"result.txt"}
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, 200).Body.Bytes(), &plan)
	commitRoute := base + "/git/commit"
	journeyRequest(t, h, "POST", commitRoute, reviewedCommitRequest(t, h, commitRoute, "Review candidate write scope"), 200)
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/orchestrator/policy", map[string]any{"expectedRevision": request.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "integrationMode": "reviewed-scopes"}, 200).Body.Bytes(), &policy)
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedContextDigest, request.ExpectedPolicyRevision = view.Context.Digest, policy.Revision
	request.Run.ImplementationContext.PlanRevision = plan.Revision
	s, p, h, ctx, continuation, _ := coordinatorDeliveryRequestFixture(t, s, p, h, ctx, request)
	run, err := s.projectRuns.Get(p.ID, p.Path, continuation.RunID)
	if err != nil {
		t.Fatal(err)
	}
	swarmBase := base + "/runs/" + run.ID + "/swarm"
	run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", swarmBase+"/tasks/task-save/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "candidate-integration-123456"}, 201)).ProjectRun
	op := run.Operations[len(run.Operations)-1]
	workerPath := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, workerPath, "result.txt", "independently integrated implementation\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || run.Swarm.Tasks[0].Integration == nil || run.Swarm.Tasks[0].Integration.State != "integrated" {
		t.Fatalf("reviewed candidate did not integrate: %+v %v", run, err)
	}
	commit := run.Swarm.Tasks[0].Integration.Commit
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || run.Swarm.Tasks[0].Integration.Commit != commit {
		t.Fatal("replayed reviewed integration", err)
	}
	rows, err := s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 || rows.Rows[0].Delivery == nil || rows.Rows[0].Delivery.CheckID == "" {
		t.Fatal("integrated source did not reserve independent check", err)
	}
}
