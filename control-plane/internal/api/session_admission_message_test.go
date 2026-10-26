package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestManualSessionAdmissionDoesNotClaimQueuedWork(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return resourcebudget.Snapshot{} }
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	decision, err := g.reserve(context.Background(), "manual-fixture", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if err != nil || decision.State != "waiting" || decision.Code != "resource_observation_unavailable" {
		t.Fatal("fixture did not reach the real unknown-capacity refusal", decision, err)
	}
	queued := decision.Message
	manual := manualSessionAdmissionMessage(decision)
	if strings.Contains(manual, "queued") || !strings.Contains(manual, "No new worker was started") || !strings.Contains(manual, "Refresh workspace capacity") {
		t.Fatal("manual refusal claimed queued work or omitted the next action", manual)
	}
	if decision.Message != queued || !strings.Contains(queued, "The task remains queued.") {
		t.Fatal("manual response changed the shared queued-scheduler decision")
	}
}

func TestManualSessionAdmissionPreservesOtherRefusals(t *testing.T) {
	for _, code := range []string{"memory_pressure", "global_slot_limit"} {
		t.Run(code, func(t *testing.T) {
			decision := executionDecision{State: "waiting", Code: code, Message: "Keep this exact refusal."}
			if manualSessionAdmissionMessage(decision) != decision.Message {
				t.Fatal("manual response rewrote another admission refusal")
			}
		})
	}
}

func TestManualSessionRevalidationKeepsSharedQueuedDecision(t *testing.T) {
	decision := executionDecision{State: "waiting", Code: "resource_observation_unavailable", Message: "Current machine capacity is not available. The task remains queued."}
	original := decision
	err := manualSessionAdmissionFailure(decision)
	var admission *executionAdmissionError
	if !errors.As(err, &admission) || admission.code != decision.Code || admission.evidence != decision.Evidence || err.Error() != manualSessionAdmissionMessage(decision) || strings.Contains(err.Error(), "queued") {
		t.Fatal("manual revalidation lost the typed refusal or retained false queue wording")
	}
	if decision != original || admissionDecisionFailure(decision).Error() != original.Message {
		t.Fatal("manual revalidation changed durable queued-dispatch semantics")
	}
}
