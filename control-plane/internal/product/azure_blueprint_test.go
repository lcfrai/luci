package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func azureBlueprintFixture(t *testing.T) (*Server, project.Project, CloudRequest, context.Context) {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "foundation-binding")
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	spec, design := currentSpecification(state), currentDesignPlan(state)
	b := deploymentplan.Blueprint{Version: 1, ProjectID: p.ID, Source: deploymentplan.Source{Commit: strings.Repeat("a", 40), Context: &deploymentplan.ContextReference{BriefRevision: state.Brief.Revision, SpecificationRevision: spec.Revision, SpecificationDigest: spec.Digest, DesignRevision: design.Revision, DesignDigest: design.Digest}}, Infrastructure: deploymentplan.Infrastructure{ID: "cluster", Kind: "kubernetes", Reference: "existing", Namespace: "apps"}, Services: []deploymentplan.Service{{ID: "app", Kind: "app", Image: "example/app@sha256:" + strings.Repeat("b", 64), Port: 8080, Replicas: 1, Resources: deploymentplan.ResourceBudget{RequestMilliCPU: 250, LimitMilliCPU: 500, RequestMemoryMiB: 256, LimitMemoryMiB: 512}}}}
	compiled, err := deploymentplan.Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	ctx := homeActor("alice")
	actor, policy, _ := management.ActorFromContext(ctx)
	policy.AllowCloudProvisioning = true
	policy.AllowPublicPublishing = true
	policy.MonthlyBudgetUSD = 100
	s.coordinatorDeploymentAuthorize = func(_ context.Context, ref management.ExecutionActorRef, _ bool) (management.ExecutionAuthority, error) {
		if ref.UserID != "alice" || ref.ProjectID != p.ID || ref.Local {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{Actor: actor, Policy: policy}, nil
	}
	request := CloudRequest{Provider: "azure", Name: "foundation-one", Region: "australiaeast", Domain: "example.com", Size: "small", MonthlyBudgetUSD: 50, Foundation: &deploymentplan.AzureFoundationRequest{Blueprint: b, ExpectedBlueprintDigest: compiled.Digest}}
	return s, p, request, ctx
}

func foundationPlanForTest(t *testing.T, s *Server, ctx context.Context, request CloudRequest) CloudPlan {
	t.Helper()
	b, err := s.prepareFoundationBinding(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	p := CloudPlan{ID: "cp-owned", Provider: "azure", Name: request.Name, Region: request.Region, MonthlyBudgetUSD: request.MonthlyBudgetUSD, Foundation: b}
	b.PlanDigest = foundationPlanDigest(p)
	if !validFoundationPlan(p) {
		t.Fatal("server binding invalid")
	}
	return p
}

func TestAzureFoundationBindingCannotSubstituteActorProjectOrClientDigest(t *testing.T) {
	s, _, request, ctx := azureBlueprintFixture(t)
	plan := foundationPlanForTest(t, s, ctx, request)
	if _, err := s.authorizeFoundationPlan(homeActor("bob"), plan, true, true); err == nil {
		t.Fatal("another actor reused the stored plan")
	}
	request.Foundation.ExpectedBlueprintDigest = strings.Repeat("0", 64)
	if _, err := s.prepareFoundationBinding(ctx, request); err == nil {
		t.Fatal("client digest stood in for compilation")
	}
	plan.Foundation.Actor.ProjectID = "foreign"
	if _, err := s.authorizeFoundationPlan(ctx, plan, true, true); err == nil {
		t.Fatal("forged plan identity gained authority")
	}
}

func TestAzureFoundationCurrentContextAndCurrentBudgetRecheckedBeforeProviderUse(t *testing.T) {
	s, p, request, ctx := azureBlueprintFixture(t)
	plan := foundationPlanForTest(t, s, ctx, request)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Brief.Revision++
	state.Brief.Purpose = "A later project edit"
	if err = writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	s.mu.RLock()
	err = s.recheckFoundationContextLocked(p, plan)
	s.mu.RUnlock()
	if err == nil {
		t.Fatal("source drift reused a reviewed foundation")
	}
	actor, policy, _ := management.ActorFromContext(ctx)
	policy.AllowCloudProvisioning = true
	policy.AllowPublicPublishing = true
	policy.MonthlyBudgetUSD = 1
	s.coordinatorDeploymentAuthorize = func(context.Context, management.ExecutionActorRef, bool) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{Actor: actor, Policy: policy}, nil
	}
	if _, err = s.authorizeFoundationPlan(ctx, plan, true, true); err == nil {
		t.Fatal("current budget did not fence the allocation")
	}
	if _, err = s.authorizeFoundationPlan(ctx, plan, false, true); err != nil {
		t.Fatal("retained observation incorrectly demanded new-allocation budget", err)
	}
}

func TestAzureFoundationUnknownJobNeverAdmitsAnotherProviderAttempt(t *testing.T) {
	s, _, request, ctx := azureBlueprintFixture(t)
	plan := foundationPlanForTest(t, s, ctx, request)
	job := CloudJob{ID: "cj-owned", PlanID: plan.ID, Provider: "azure", Plan: plan, Status: "unknown"}
	s.jobs[job.ID] = job
	if err := s.admitFoundationProvider(ctx, job); err == nil {
		t.Fatal("unknown operation was admitted for replay")
	}
	if !s.foundationJobReadable(ctx, job) {
		t.Fatal("unknown evidence should remain readable to its current deployer")
	}
}

func TestAzureFoundationOptionalBindingPreservesExactLegacyPlanEncoding(t *testing.T) {
	raw, err := json.Marshal(CloudPlan{})
	expected := `{"monthlyBudgetUsd":0,"id":"","provider":"","name":"","region":"","domain":"","resources":null,"estimate":{"currency":"","monthly":null,"verified":false,"explanation":""},"steps":null,"files":null,"commands":null,"warnings":null,"createdAt":"0001-01-01T00:00:00Z"}`
	if err != nil || string(raw) != expected {
		t.Fatal("optional binding changed legacy plan bytes", string(raw), err)
	}
	raw, err = json.Marshal(CloudRequest{})
	if err != nil || string(raw) != `{"provider":"","name":"","region":"","domain":"","size":"","monthlyBudgetUsd":0,"sshPublicKey":""}` {
		t.Fatal("legacy request encoding changed", string(raw), err)
	}
}

func foundationRetainedJobForTest(t *testing.T) (*Server, CloudJob, context.Context, *bool) {
	t.Helper()
	s, _, request, ctx := azureBlueprintFixture(t)
	plan := foundationPlanForTest(t, s, ctx, request)
	job := CloudJob{ID: "cj-owned", PlanID: plan.ID, Provider: "azure", Name: "foundation-one", Domain: "private.example.com", Plan: plan, Status: "provisioning", Owned: true, ResourceID: "/subscriptions/test/resourceGroups/foundation-one"}
	s.jobs[job.ID] = job
	revoked := false
	original := s.coordinatorDeploymentAuthorize
	s.coordinatorDeploymentAuthorize = func(c context.Context, actor management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		if revoked {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return original(c, actor, allocate)
	}
	return s, job, ctx, &revoked
}

func TestAzureFoundationDeleteReauthorizesAfterProviderOwnershipRead(t *testing.T) {
	s, job, ctx, revoked := foundationRetainedJobForTest(t)
	deletes := 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if exe == "az" && len(args) > 1 && args[0] == "group" && args[1] == "show" {
			*revoked = true
			return `{"id":"/subscriptions/test/resourceGroups/foundation-one","tags":{"luci-job":"cj-owned"}}`, nil
		}
		if len(args) > 1 && args[1] == "delete" {
			deletes++
		}
		t.Fatal("unexpected provider command", exe, args)
		return "", nil
	}
	r := httptest.NewRequest("POST", "/v1/setup/cloud/jobs/cj-owned/delete", strings.NewReader(`{"confirmName":"foundation-one"}`)).WithContext(ctx)
	r.SetPathValue("jobId", job.ID)
	w := httptest.NewRecorder()
	s.deleteCloudJob(w, r)
	if w.Code != 403 || deletes != 0 || s.jobsSnapshot(job.ID).Status != "provisioning" {
		t.Fatal("late grant loss reached a destructive command", w.Code, deletes)
	}
}

func TestAzureFoundationRefreshWithholdsRevokedResponseButRetainsObservation(t *testing.T) {
	s, job, ctx, revoked := foundationRetainedJobForTest(t)
	s.runner = func(_ context.Context, _ string, args []string, _ string) (string, error) {
		if len(args) > 1 && args[0] == "group" && args[1] == "exists" {
			return "true", nil
		}
		if len(args) > 1 && args[0] == "group" && args[1] == "show" {
			return `{"id":"/subscriptions/test/resourceGroups/foundation-one","tags":{"luci-job":"cj-owned"}}`, nil
		}
		if len(args) > 2 && args[0] == "deployment" && args[1] == "group" && args[2] == "show" {
			*revoked = true
			return `{"properties":{"provisioningState":"Succeeded","outputs":{}}}`, nil
		}
		t.Fatal("unexpected provider command", args)
		return "", nil
	}
	r := httptest.NewRequest("POST", "/v1/setup/cloud/jobs/cj-owned/refresh", nil).WithContext(ctx)
	r.SetPathValue("jobId", job.ID)
	w := httptest.NewRecorder()
	s.refreshCloudJob(w, r)
	if w.Code != 403 || strings.Contains(w.Body.String(), job.Domain) || s.jobsSnapshot(job.ID).Status != "resources_ready" || !strings.Contains(w.Body.String(), "not rolled back") {
		t.Fatal("revoked response leaked or discarded real observation", w.Code, w.Body.String())
	}
}

func TestAzureFoundationLateDNSResponseUsesCurrentGrantWithoutDNSLookup(t *testing.T) {
	s, job, ctx, revoked := foundationRetainedJobForTest(t)
	if !s.foundationJobReadable(ctx, job) {
		t.Fatal("initial grant absent")
	}
	*revoked = true
	r := httptest.NewRequest("POST", "/v1/setup/cloud/jobs/cj-owned/dns", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	s.writeFoundationJobResponse(w, r, job, 200, map[string]any{"domain": job.Domain, "ready": true})
	if w.Code != 403 || strings.Contains(w.Body.String(), job.Domain) || !strings.Contains(w.Body.String(), "not rolled back") {
		t.Fatal("late DNS response bypassed the shared grant check", w.Code, w.Body.String())
	}
}

func TestAzureFoundationAdmissionRefusalIsDistinctFromProviderLoginFailure(t *testing.T) {
	for _, kind := range []string{"admission", "provider"} {
		t.Run(kind, func(t *testing.T) {
			s, job, _, _ := foundationRetainedJobForTest(t)
			job.Status = "creating"
			s.jobs[job.ID] = job
			calls := 0
			if kind == "admission" {
				s.coordinatorDeploymentAuthorize = func(context.Context, management.ExecutionActorRef, bool) (management.ExecutionAuthority, error) {
					return management.ExecutionAuthority{}, management.ErrExecutionAuthority
				}
			}
			s.runner = func(context.Context, string, []string, string) (string, error) {
				calls++
				return "", errors.New("mock provider login failure")
			}
			s.provisionCloud(job)
			saved := s.jobsSnapshot(job.ID)
			if saved.ResourceID != job.ResourceID || saved.Owned != job.Owned {
				t.Fatal("lost retained ownership during refusal")
			}
			if kind == "admission" {
				if calls != 0 || saved.Status != "unknown" || !strings.Contains(saved.Message, "current foundation plan or deployment access changed") || strings.Contains(saved.Message, "not signed in") || strings.Contains(saved.Message, "nothing was created") {
					t.Fatal("local admission refusal was mislabeled or replayed", saved, calls)
				}
			} else if calls != 1 || saved.Status != "failed" || !strings.Contains(saved.Message, "Azure is not signed in") {
				t.Fatal("actual provider failure lost legacy diagnosis", saved, calls)
			}
		})
	}
}
