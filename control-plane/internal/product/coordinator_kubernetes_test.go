package product

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func coordinatorKubernetesPlanFixture() *CoordinatorKubernetesPlan {
	return &CoordinatorKubernetesPlan{TargetID: "kube-main", TargetFingerprint: strings.Repeat("a", 64), NamespaceUID: "namespace-original", BlueprintRevision: 3, BlueprintDigest: strings.Repeat("b", 64), BlueprintContextDigest: strings.Repeat("c", 64), PlanDigest: strings.Repeat("d", 64), ServiceImages: []CoordinatorKubernetesImage{{ServiceID: "app", Image: "registry.example/project/app@sha256:" + strings.Repeat("e", 64)}}, MonthlyBudgetUSD: 50, AcknowledgeCosts: true, AcknowledgeImages: true}
}

func TestCoordinatorKubernetesSelectionIsSeparateExplicitAndLegacyCompatible(t *testing.T) {
	plan := coordinatorKubernetesPlanFixture()
	checks := &CoordinatorCheckPlan{}
	if err := validateCoordinatorKubernetesSelection(plan, nil, checks, "publish"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*CoordinatorKubernetesPlan){func(p *CoordinatorKubernetesPlan) { p.AcknowledgeCosts = false }, func(p *CoordinatorKubernetesPlan) { p.ServiceImages[0].Image = "app:latest" }, func(p *CoordinatorKubernetesPlan) { p.RequiresTelemetry = true }} {
		copy := *plan
		copy.ServiceImages = append([]CoordinatorKubernetesImage{}, plan.ServiceImages...)
		mutate(&copy)
		if validateCoordinatorKubernetesPlan(&copy) == nil {
			t.Fatal("unreviewed mutation accepted")
		}
	}
	if validateCoordinatorKubernetesSelection(plan, &CoordinatorDeploymentPlan{}, checks, "publish") == nil || validateCoordinatorKubernetesSelection(plan, nil, nil, "publish") == nil || validateCoordinatorKubernetesSelection(plan, nil, checks, "preview") == nil {
		t.Fatal("mixed deployment or missing independent checks accepted")
	}
	raw, _ := json.Marshal(CoordinatorSavedRoute{})
	if strings.Contains(string(raw), "kubernetesPlan") {
		t.Fatal("legacy nil route encoding changed")
	}
	raw, _ = json.Marshal(CoordinatorDelivery{})
	if strings.Contains(string(raw), "kubernetes") {
		t.Fatal("legacy nil delivery encoding changed")
	}
}

func TestCoordinatorKubernetesIdentityBindsCandidateWithoutPublicHealthClaim(t *testing.T) {
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ID: "continue-original", ProjectID: "project-one", RunID: "run-original", Delivery: &CoordinatorDelivery{CheckID: "check-original", Status: "verified", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64)}}, Run: projectRunCreateRequest{Outcome: "publish"}, CheckPlan: &CoordinatorCheckPlan{}, KubernetesPlan: coordinatorKubernetesPlanFixture()}
	before := coordinatorKubernetesBinding(row)
	original := coordinatorKubernetesReviewID(row)
	row.Delivery.Kubernetes = &CoordinatorKubernetesAction{ActionID: row.ID + "-kubernetes", ReviewID: original, BindingDigest: before, Status: "unknown", StartedAt: "2026-10-31T00:00:00Z", UpdatedAt: "2026-10-31T00:00:00Z", Resources: []CoordinatorKubernetesResource{}}
	if err := validateCoordinatorKubernetesContinuation(row); err != nil {
		t.Fatal(err)
	}
	row.Delivery.Kubernetes.ApplicationHealthVerified = true
	if validateCoordinatorKubernetesContinuation(row) == nil {
		t.Fatal("workload receipt became app acceptance")
	}
	row.Delivery.Kubernetes.ApplicationHealthVerified = false
	row.Delivery.CheckID = "check-new"
	if coordinatorKubernetesReviewID(row) == original || coordinatorKubernetesBinding(row) == before || validateCoordinatorKubernetesContinuation(row) == nil {
		t.Fatal("a replacement candidate adopted the original review")
	}
}

func TestCoordinatorKubernetesOriginalApplyMarkerSurvivesMissingReviewWithoutReplay(t *testing.T) {
	s, p, h, ctx, out, _ := coordinatorDeliveryFixture(t)
	coordinatorIntegrateFixture(t, s, p, h, out)
	row, _ := coordinatorReserveCheckFixture(t, s, p, ctx, out.ID)
	row.Run.Outcome = "publish"
	row.KubernetesPlan = coordinatorKubernetesPlanFixture()
	row.DeploymentPlan = nil
	row.PreviewPlan = nil
	row.Delivery.Status = "verified"
	now := coordinatorPreviewNow()
	row.Delivery.Kubernetes = &CoordinatorKubernetesAction{ActionID: row.ID + "-kubernetes", ReviewID: coordinatorKubernetesReviewID(row), BindingDigest: coordinatorKubernetesBinding(row), Status: "pending", StartedAt: now, UpdatedAt: now, Resources: []CoordinatorKubernetesResource{}}
	actor, policy, _ := management.ActorFromContext(ctx)
	policy.AllowPublicPublishing = false
	policy.AllowCloudProvisioning = true
	policy.MonthlyBudgetUSD = 100
	calls := 0
	s.coordinatorDeploymentAuthorize = func(_ context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		calls++
		if allocate {
			t.Fatal("an already attempted original requested new allocation")
		}
		if ref != row.ExecutionActor {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{Actor: actor, Policy: policy}, nil
	}
	s.mu.Lock()
	rows, err := s.readContinuations(p, row.Actor.ID)
	index := findContinuation(rows, row.ID)
	if err == nil && index >= 0 {
		rows.Rows[index] = row
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil || index < 0 {
		t.Fatal(err)
	}
	row.Delivery.Kubernetes.ApplyAttempted = true
	saved, err := s.persistCoordinatorKubernetes(ctx, p, row, "running", "Original Apply intent retained.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.persistCoordinatorKubernetes(ctx, p, row, "running", "old revision", nil); err == nil {
		t.Fatal("stale CAS repeated original admission")
	}
	s.invokeCoordinatorKubernetes(ctx, p, saved)
	s.mu.RLock()
	rows, err = s.readContinuations(p, row.Actor.ID)
	s.mu.RUnlock()
	if err != nil {
		t.Fatal(err)
	}
	latest := rows.Rows[findContinuation(rows, row.ID)]
	if latest.Delivery.Kubernetes.Status != "unknown" || !latest.Delivery.Kubernetes.ApplyAttempted || latest.Delivery.Kubernetes.ReviewID != saved.Delivery.Kubernetes.ReviewID || calls == 0 {
		t.Fatal("missing review was retried or adopted")
	}
	_, reviews, err := s.readKubernetesReviews(p.ID)
	if err != nil || len(reviews.Reviews) != 0 {
		t.Fatal("recovery minted a new review instead of observing original", err)
	}
}
