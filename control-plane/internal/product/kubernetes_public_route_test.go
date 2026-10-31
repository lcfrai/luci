package product

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"testing"
)

func TestPublicKubernetesRouteRequiresCurrentPublishingPolicyWithoutWideningPrivateDeploy(t *testing.T) {
	s, p, _, ctx := azureBlueprintFixture(t)
	actor, policy, _ := management.ActorFromContext(ctx)
	policy.AllowCloudProvisioning = true
	policy.AllowPublicPublishing = false
	policy.MonthlyBudgetUSD = 100
	s.coordinatorDeploymentAuthorize = func(_ context.Context, ref management.ExecutionActorRef, _ bool) (management.ExecutionAuthority, error) {
		if ref.UserID != actor.ID || ref.ProjectID != p.ID {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{Actor: actor, Policy: policy}, nil
	}
	ref, err := management.CaptureDeploymentActor(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.kubernetesPublicAuthority(ctx, ref) == nil {
		t.Fatal("public routing ignored publishing policy")
	}
	if err = s.kubernetesAuthority(ctx, ref, true, true, 50); err != nil {
		t.Fatal("private workload was incorrectly made public-policy dependent", err)
	}
	policy.AllowPublicPublishing = true
	if err = s.kubernetesPublicAuthority(ctx, ref); err != nil {
		t.Fatal(err)
	}
	ref.UserID = "someone-else"
	if s.kubernetesPublicAuthority(ctx, ref) == nil {
		t.Fatal("public route inherited another caller's grant")
	}
}
