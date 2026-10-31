package product

import (
	"context"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestTelemetryObservationRequiresCurrentOriginalDeployAuthorityWithoutAllocating(t *testing.T) {
	s, p, _, ctx := azureBlueprintFixture(t)
	actor, _, _ := management.ActorFromContext(ctx)
	ref, err := management.CaptureDeploymentActor(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeJobs, beforePlans := len(s.jobs), len(s.plans)
	calls := 0
	s.coordinatorDeploymentAuthorize = func(_ context.Context, original management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		calls++
		if original.UserID != actor.ID || original.ProjectID != p.ID || allocate {
			t.Fatal("read requested different authority or allocation", original, allocate)
		}
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if _, err := s.ReadKubernetesTelemetry(ctx, ref, "workload-"+strings.Repeat("a", 32)); err == nil || calls != 1 {
		t.Fatal("current denied authority reached target reads", err, calls)
	}
	if len(s.jobs) != beforeJobs || len(s.plans) != beforePlans {
		t.Fatal("telemetry read created provider work")
	}
}
