package product

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func coordinatorPolicyDeploymentPlan() *CoordinatorDeploymentPlan {
	return &CoordinatorDeploymentPlan{TargetID: "target-one", TargetFingerprint: strings.Repeat("a", 64), ServerUUID: "server-one", Repository: "https://github.com/fixture/application", Branch: "main", Domain: "https://app.example.com", Port: 3000, HealthPath: "/", Dockerfile: "Dockerfile", Source: "public", DataClientFramework: "none", ExpectedConfigurationRevision: 1, AcknowledgePublic: true, AcknowledgeDeploymentCosts: true, AcknowledgeRepositoryPush: true}
}

func TestCoordinatorDeploymentPolicyIsExplicitAndPartialDraftUpdatePreservesIt(t *testing.T) {
	s, p, h, _ := coordinatorPolicyFixture(t)
	base := "/v1/product/projects/" + p.ID + "/orchestrator/policy"
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", base, nil, 200).Body.Bytes(), &policy)
	if policy.DeploymentMode != "manual" {
		t.Fatal("public delivery silently delegated", policy)
	}
	body := map[string]any{"expectedRevision": policy.Revision, "mode": "review", "allowedTargets": []string{"plan"}, "deploymentMode": "existing-target"}
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base, body, 200).Body.Bytes(), &policy)
	policy = coordinatorSavePolicy(t, h, p.ID, policy.Revision, "auto-drafts", []string{"plan"})
	if policy.DeploymentMode != "existing-target" || policy.ExecutionMode != "manual" || policy.PreviewMode != "manual" {
		t.Fatal("draft update altered independent delegation", policy)
	}
	path, _ := s.coordinatorPolicyPath(p.ID, false)
	before, _ := os.ReadFile(path)
	body["expectedRevision"] = policy.Revision
	body["deploymentMode"] = "provision-any-host"
	journeyRequest(t, h, "POST", base, body, 400)
	body["deploymentMode"] = "manual"
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("Edit revoked") })
	journeyRequest(t, h, "POST", base, body, 403)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("refused policy changed deployment delegation")
	}
}

func TestCoordinatorSavedDeploymentPlanCannotBeSubstitutedOrExpanded(t *testing.T) {
	s, p, h, _, req, starts := coordinatorContinuationFixture(t)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bindProjectRequirements(state, p.ID, []string{"AC-save"}, &req.Run.ImplementationContext.SpecificationRevision, &req.Run.ImplementationContext.DesignPlanRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	req.Run.Outcome = "publish"
	req.CheckPlan = &CoordinatorCheckPlan{Commands: []CheckCommand{{Label: "Reviewed check", Executable: "node", Args: []string{"-e", "process.exit(0)"}}}, RequirementIDs: binding.RequirementIDs, RequirementSnapshots: binding.RequirementSnapshots, AcknowledgeProjectCommands: true}
	req.PreviewPlan = &CoordinatorPreviewPlan{Dockerfile: "Dockerfile", Port: 3000, ExpectedConfigurationRevision: 1, AcknowledgePreviewBuild: true}
	req.DeploymentPlan = coordinatorPolicyDeploymentPlan()
	req.RememberRoute = true
	first := coordinatorCreate(t, h, p, req, 201)
	path, _ := s.continuationPath(p.ID, "alice", false)
	before, _ := os.ReadFile(path)
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if rows.SavedRoute == nil || reviewDigest(rows.SavedRoute.DeploymentPlan) != reviewDigest(req.DeploymentPlan) || reviewDigest(rows.Rows[0].DeploymentPlan) != reviewDigest(req.DeploymentPlan) {
		t.Fatal("exact public plan not retained")
	}
	replay := coordinatorCreate(t, h, p, req, 200)
	if replay.ID != first.ID {
		t.Fatal("duplicate public delivery continuation")
	}
	version := rows.SavedRoute.Revision
	req.ExpectedRouteRevision = &version
	for _, kind := range []string{"target", "repository", "branch", "public-url", "configuration", "data", "omitted", "consent", "repository-consent", "preview-outcome"} {
		t.Run(kind, func(t *testing.T) {
			changed := req
			copy := *req.DeploymentPlan
			changed.DeploymentPlan = &copy
			changed.IdempotencyKey = "deployment-substitution-" + kind
			status := 409
			switch kind {
			case "target":
				copy.TargetID = "target-two"
			case "repository":
				copy.Repository = "https://github.com/fixture/other"
			case "branch":
				copy.Branch = "other"
			case "public-url":
				copy.Domain = "https://other.example.com"
			case "configuration":
				copy.ExpectedConfigurationRevision++
			case "data":
				copy.ExpectedDataDomainRevision = 1
				copy.ExpectedDataBindingFingerprint = strings.Repeat("b", 64)
			case "omitted":
				changed.DeploymentPlan = nil
			case "consent":
				copy.AcknowledgePublic = false
				status = 400
			case "repository-consent":
				copy.AcknowledgeRepositoryPush = false
				status = 400
			case "preview-outcome":
				changed.Run.Outcome = "preview"
				status = 400
			}
			journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", changed, status)
		})
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || *starts != 0 {
		t.Fatal("route refusal mutated or dispatched")
	}
}

func TestCoordinatorBackgroundFinishesDelegatedDeploymentAndReopenDoesNotReplay(t *testing.T) {
	s, p, h, ctx, out, provider := coordinatorDeploymentFixture(t, false)
	row := coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out)
	if row.Status != "preview-ready" || row.Delivery.Deployment != nil {
		t.Fatal("fixture already deployed", row)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err := s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitCoordinatorDeploymentFixture(t, s, row.ID)
	if err := s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	ready := rows.Rows[findContinuation(rows, row.ID)]
	projects, apps, deployments := provider.counts()
	if ready.Status != "deployed" || ready.Delivery.Deployment == nil || ready.Delivery.Deployment.Evidence == nil || projects != 1 || apps != 1 || deployments != 1 {
		t.Fatal("background did not establish exact public health", ready, projects, apps, deployments)
	}
	reopened, _ := coordinatorDeliveryReopen(t, s)
	reopened.SetPreviewRuntime(s.PreviewRuntime())
	reopened.SetCoordinatorDeploymentAuthorizer(s.coordinatorDeploymentAuthorize)
	reopened.cloudReleaseHealthObserver = s.cloudReleaseHealthObserver
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
	p2, a2, d2 := provider.counts()
	if got.Status != "deployed" || got.Delivery.Deployment.ReleaseID != ready.Delivery.Deployment.ReleaseID || p2 != projects || a2 != apps || d2 != deployments || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("restart replayed or rewrote terminal delivery")
	}
}

func TestCoordinatorBackgroundRetainedDeploymentPublicationObservesWithoutNewExecutionOrAllocation(t *testing.T) {
	s, p, h, ctx, out, provider := coordinatorDeploymentFixture(t, false)
	row := coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out)
	provider.git.loseACK.Store(true)
	row, err := s.continueCoordinatorGitPublication(ctx, p, row)
	if err != nil || row.Delivery.GitPublication == nil || row.Delivery.GitPublication.Status != "unknown" || provider.git.pushes.Load() != 1 {
		t.Fatal("fixture did not retain the actually published commit", err)
	}

	// Reopen without retaining the originating browser. New workers and new
	// provider allocations are refused, but current Edit/Deploy permits exact
	// observation of the already issued source publication.
	reopened, _ := coordinatorDeliveryReopen(t, s)
	reopened.runner = s.runner
	reopened.SetPreviewRuntime(s.PreviewRuntime())
	reopened.cloudReleaseHealthObserver = s.cloudReleaseHealthObserver
	executeCalls, allocationCalls := 0, 0
	reopened.SetCoordinatorExecutionAuthorizer(func(context.Context, string, string, bool) error {
		executeCalls++
		return errors.New("new worker execution suspended")
	})
	reopened.coordinatorBackground.authorize = func(context.Context, management.ExecutionActorRef, string, bool) (context.Context, error) {
		executeCalls++
		return nil, errors.New("new worker execution suspended")
	}
	reopened.SetCoordinatorDeploymentAuthorizer(func(ctx context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		if allocate {
			allocationCalls++
			return management.ExecutionAuthority{}, errors.New("new provider allocations suspended")
		}
		return s.coordinatorDeploymentAuthorize(ctx, ref, false)
	})
	if err := reopened.RunCoordinatorOnce(context.Background()); err == nil {
		t.Fatal("new provider allocation unexpectedly permitted")
	}
	retained, err := reopened.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	got := retained.Rows[findContinuation(retained, row.ID)]
	projects, apps, deployments := provider.counts()
	if !coordinatorGitPublished(got) || got.Delivery.Deployment != nil || executeCalls != 0 || allocationCalls != 1 || provider.git.pushes.Load() != 1 || provider.git.observations.Load() != 1 || projects+apps+deployments != 0 {
		t.Fatal("background did not distinguish retained observation from new effects", got.Status, executeCalls, allocationCalls, provider.git.pushes.Load(), provider.git.observations.Load(), projects, apps, deployments)
	}
	for _, kind := range []string{"edit-revoked", "deploy-revoked", "foreign-current-actor", "public-policy-revoked"} {
		t.Run(kind, func(t *testing.T) {
			before := auditTree(t, reopened.stateDir)
			originalEdit := reopened.coordinatorAuthorize
			originalDeploy := reopened.coordinatorDeploymentAuthorize
			defer func() {
				reopened.coordinatorAuthorize = originalEdit
				reopened.coordinatorDeploymentAuthorize = originalDeploy
			}()
			if kind == "edit-revoked" {
				reopened.coordinatorAuthorize = func(context.Context, string) error { return errors.New("Edit revoked") }
			} else {
				reopened.coordinatorDeploymentAuthorize = func(ctx context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
					authority, err := originalDeploy(ctx, ref, allocate)
					switch kind {
					case "deploy-revoked":
						return management.ExecutionAuthority{}, errors.New("Deploy revoked")
					case "foreign-current-actor":
						authority.Actor.ID = "bob"
					case "public-policy-revoked":
						authority.Policy.AllowPublicPublishing = false
					}
					return authority, err
				}
			}
			if err := reopened.RunCoordinatorOnce(context.Background()); err == nil {
				t.Fatal("current authority refusal was bypassed")
			}
			if !reflect.DeepEqual(before, auditTree(t, reopened.stateDir)) || provider.git.observations.Load() != 1 || provider.git.pushes.Load() != 1 || allocationCalls != 1 {
				t.Fatal("revoked observation mutated or replayed work")
			}
		})
	}
}
