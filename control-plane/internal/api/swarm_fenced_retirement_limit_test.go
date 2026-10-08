package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Minimal valid historical pre-backend receipts exercise the independent
// retirement bound without consuming any permanent Docker marker slots.
func fillFenceFixtureRetirementHistory(t *testing.T, a *API) {
	t.Helper()
	if err := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Retirements) != 0 {
			return fmt.Errorf("fixture requires initially empty retirement history")
		}
		at := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 4096; i++ {
			id := fmt.Sprintf("history-%d", i)
			proof := product.ProjectSwarmRecoveryProof{ChildID: "historical-child", LeaseID: id, Backend: "docker", ObservedAt: at.Format(time.RFC3339), InventoryObservedAt: at.Format(time.RFC3339)}
			l.Retirements = append(l.Retirements, executionRetirement{Scope: a.execution.scope, LeaseID: id, ProjectID: "historical-project", RunID: "historical-run", OperationID: id, ChildID: proof.ChildID, Proof: proof, RetiredAt: at})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if ledger, err := a.execution.readFenceLedger(); err != nil || len(ledger.Retirements) != 4096 {
		t.Fatal("full valid historical ledger did not reload", err)
	}
}

func TestSwarmFencedRetirementHistoryLimitReportsActualBoundBeforeEffects(t *testing.T) {
	f, probe, _ := fencedTaskFixture(t)
	a := f.env.api
	fillFenceFixtureRetirementHistory(t, a)
	before, err := os.ReadFile(a.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	w := f.env.do(t, http.MethodGet, f.base()+"/"+f.run.ID+"/swarm/operations/"+f.ops[0].ID+"/recovery", "")
	var view struct {
		Availability product.ProjectSwarmRecoveryAvailability `json:"availability"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &view) != nil || view.Availability.Supported || view.Availability.ReasonCode != "recovery_retirement_history_full" || view.Availability.Message != errLaunchRetirementLimit.Error() {
		t.Fatal("capability reported the wrong exhausted bound", w.Code, w.Body.String())
	}
	w = fencedRecover(t, f, http.StatusConflict)
	if !strings.Contains(w.Body.String(), errLaunchRetirementLimit.Error()) || strings.Contains(w.Body.String(), "64 permanent") {
		t.Fatal("recovery action reported a marker limit instead of retirement history", w.Body.String())
	}
	after, err := os.ReadFile(a.execution.path)
	if err != nil || !bytes.Equal(before, after) || len(probe.requests) != 0 || probe.verifyCalls != 0 {
		t.Fatal("full retirement history changed launch state or reached backend effects", err)
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil || len(ledger.Fences) != 0 || len(ledger.Leases) != 1 || ledger.Leases[0].State != "backend-dispatching" || f.run.Operations[0].LaunchRecovery != nil {
		t.Fatal("quota refusal stopped or retired the unresolved launch", err)
	}
}

func TestSwarmFencedRetirementHistoryFillingBeforeFinalizeRetainsProofAndCapacity(t *testing.T) {
	f, probe, _ := fencedTaskFixture(t)
	a := f.env.api
	a.deps.Product.SetProjectSwarmFenceRecovery(a.projectSwarmFenceRecoveryAvailability, a.authorizeSwarmFenceRecovery, a.acquireSwarmFenceRecovery, a.proveSwarmFencedLaunch, func(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest, intent product.ProjectSwarmRecoveryIntent, proof product.ProjectSwarmRecoveryProof) error {
		fillFenceFixtureRetirementHistory(t, a)
		return a.finalizeSwarmFencedLaunch(ctx, req, intent, proof)
	})
	w := fencedRecover(t, f, http.StatusConflict)
	if !strings.Contains(w.Body.String(), errLaunchRetirementLimit.Error()) || strings.Contains(w.Body.String(), "64 permanent") || strings.Contains(w.Body.String(), "No worker was stopped") {
		t.Fatal("partial recovery misreported the exhausted bound or earlier effects", w.Body.String())
	}
	ledger := assertFencedLedger(t, f, 1, "proved")
	op := f.run.Operations[0]
	if len(ledger.Retirements) != 4096 || ledger.Leases[0].State != "fencing" || op.LaunchRecoveryProof == nil || op.AbsenceRetirement == nil || op.AbsenceRetirement.FinalizedAt != "" || len(probe.requests) != 1 || probe.verifyCalls != 1 {
		t.Fatal("failed finalization discarded proof, released capacity, or repeated backend effects")
	}
}
