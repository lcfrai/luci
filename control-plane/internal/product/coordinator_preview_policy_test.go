package product

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestCoordinatorPreviewPolicyIsExplicitPreservedAndCurrentlyAuthorized(t *testing.T) {
	s, p, h, _ := coordinatorPolicyFixture(t)
	base := "/v1/product/projects/" + p.ID + "/orchestrator/policy"
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", base, nil, 200).Body.Bytes(), &policy)
	if policy.PreviewMode != "manual" || policy.DeploymentMode != "manual" {
		t.Fatal("default delegated delivery", policy)
	}
	body := map[string]any{"expectedRevision": policy.Revision, "mode": "review", "allowedTargets": []string{"plan"}, "previewMode": "checked-source"}
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base, body, 200).Body.Bytes(), &policy)
	policy = coordinatorSavePolicy(t, h, p.ID, policy.Revision, "auto-drafts", []string{"plan"})
	if policy.PreviewMode != "checked-source" || policy.DeploymentMode != "manual" || policy.ExecutionMode != "manual" {
		t.Fatal("draft update reset explicit preview policy or granted unrelated permission", policy)
	}
	path, _ := s.coordinatorPolicyPath(p.ID, false)
	before, _ := os.ReadFile(path)
	body["expectedRevision"] = policy.Revision
	body["previewMode"] = "automatic-deploy"
	journeyRequest(t, h, "POST", base, body, 400)
	body["previewMode"] = "manual"
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("current Edit revoked") })
	journeyRequest(t, h, "POST", base, body, 403)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid or unauthorized policy changed delegation")
	}
}

func TestCoordinatorSavedPreviewPlanCannotBeSubstitutedOrDropped(t *testing.T) {
	s, p, h, _, req, starts := coordinatorContinuationFixture(t)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bindProjectRequirements(state, p.ID, []string{"AC-save"}, &req.Run.ImplementationContext.SpecificationRevision, &req.Run.ImplementationContext.DesignPlanRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	req.CheckPlan = &CoordinatorCheckPlan{Commands: []CheckCommand{{Label: "Reviewed check", Executable: "node", Args: []string{"-e", "process.exit(0)"}}}, RequirementIDs: binding.RequirementIDs, RequirementSnapshots: binding.RequirementSnapshots, AcknowledgeProjectCommands: true}
	req.PreviewPlan = &CoordinatorPreviewPlan{Dockerfile: "Dockerfile", Port: 8080, ExpectedConfigurationRevision: 1, AcknowledgePreviewBuild: true}
	req.RememberRoute = true
	first := coordinatorCreate(t, h, p, req, 201)
	path, _ := s.continuationPath(p.ID, "alice", false)
	before, _ := os.ReadFile(path)
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if rows.SavedRoute == nil || reviewDigest(rows.SavedRoute.PreviewPlan) != reviewDigest(req.PreviewPlan) || reviewDigest(rows.Rows[0].PreviewPlan) != reviewDigest(req.PreviewPlan) {
		t.Fatal("exact preview plan not retained")
	}
	replay := coordinatorCreate(t, h, p, req, 200)
	if replay.ID != first.ID {
		t.Fatal("lost-response replay duplicated continuation")
	}
	base := "/v1/product/projects/" + p.ID + "/orchestrator/continuations"
	version := rows.SavedRoute.Revision
	req.ExpectedRouteRevision = &version
	plan := *req.PreviewPlan
	for _, kind := range []string{"port", "configuration", "omitted"} {
		t.Run(kind, func(t *testing.T) {
			changed := req
			copy := plan
			changed.PreviewPlan = &copy
			changed.IdempotencyKey = "preview-substitution-" + kind
			switch kind {
			case "port":
				copy.Port++
			case "configuration":
				copy.ExpectedConfigurationRevision++
			case "omitted":
				changed.PreviewPlan = nil
			}
			journeyRequest(t, h, "POST", base, changed, 409)
		})
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || *starts != 0 {
		t.Fatal("route refusal/readback changed state or dispatched work")
	}
}

func TestCoordinatorBackgroundFinishesCheckedPreviewWithoutBrowserAndReopenDoesNotReplay(t *testing.T) {
	s, p, h, ctx, out, starts := coordinatorPreviewFixture(t, "preview", false)
	row := coordinatorPreviewCheckedFixture(t, s, p, h, ctx, out)
	if row.Status != "checks-verified" || row.Delivery.Preview != nil || starts.Load() != 0 {
		t.Fatal("fixture already previewed", row)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	// No continuation POST follows the check receipt. The service tick resolves
	// the saved actor and advances the same action after the UI stops driving it.
	if err := s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitCoordinatorPreviewFixture(t, s, row.ID)
	if err := s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	ready := rows.Rows[findContinuation(rows, row.ID)]
	if ready.Status != "preview-ready" || ready.Delivery.Preview == nil || ready.Delivery.Preview.Evidence == nil || starts.Load() != 1 {
		t.Fatal("background did not join exact owned health", ready, starts.Load())
	}
	releaseID := ready.Delivery.Preview.ReleaseID
	reopened, _ := coordinatorDeliveryReopen(t, s)
	reopened.SetPreviewRuntime(s.PreviewRuntime())
	reopened.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	before := auditTree(t, s.stateDir)
	for range 2 {
		if err := reopened.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	retained, err := reopened.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	got := retained.Rows[findContinuation(retained, row.ID)]
	if got.Status != "preview-ready" || got.Delivery.Preview.ReleaseID != releaseID || starts.Load() != 1 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("restart replayed or rewrote exact terminal preview")
	}
}
