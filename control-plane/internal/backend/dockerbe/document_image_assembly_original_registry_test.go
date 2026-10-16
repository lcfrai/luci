package dockerbe

import (
	"context"
	"fmt"
	"testing"
)

// Pure private registry-state fixtures. No native guard/API/census receiver is
// fabricated or called, and these observations do not authenticate inputs.
func TestAssemblyRegistryPreparedChargeDoesNotRefundOrChangeLifetime(t *testing.T) {
	r := newAssemblyOriginalRegistry(true, []assemblyOriginalSlot{{path: "original", bytes: 1, state: "planned"}, {path: "other", bytes: 2, state: "planned"}})
	charge, err := r.lease.Charge()
	if err != nil {
		t.Fatal(err)
	}
	if n, e := charge.Slots(); e != nil || n != 2 {
		t.Fatal(n, e)
	}
	if err := r.start(); err != nil {
		t.Fatal(err)
	}
	id, err := r.open("original", false, 1)
	if err != nil || id != 1 {
		t.Fatal(id, err)
	}
	r.acknowledgeOpen(id, true, false)
	r.retire()
	state, _ := r.lease.RetirementState()
	if state.CollectorStopped || state.AllSlotsClosed || state.Planned != 1 || state.ValidACK != 1 {
		t.Fatal("retirement invented stop", state)
	}
	if r.beginClose() == nil {
		t.Fatal("active collector allowed original close")
	}
	r.stop()
	if r.beginClose() != nil || r.beginSlotClose(id) != nil {
		t.Fatal("actual returned collector could not retire")
	}
	r.acknowledgeClose(id, false)
	r.finishClose()
	state, _ = r.lease.RetirementState()
	if state.UnknownClose != 1 || state.AllSlotsClosed || state.Closed != 1 {
		t.Fatal("failed close lost exact slot", state)
	}
	if r.beginClose() != nil || r.beginSlotClose(id) != nil {
		t.Fatal("exact retained slot could not retry close")
	}
	r.acknowledgeClose(id, true)
	r.finishClose()
	state, _ = r.lease.RetirementState()
	if !state.AllSlotsClosed {
		t.Fatal(state)
	}
	still, e := r.lease.Charge()
	if e != nil || !charge.SameRegistry(still) {
		t.Fatal("compaction/retry changed lifetime", e)
	}
	if n, _ := still.Slots(); n != 2 {
		t.Fatal("metadata close refunded reserved ceiling", n)
	}
	other := newAssemblyOriginalRegistry(true, []assemblyOriginalSlot{{path: "original", bytes: 1, state: "planned"}})
	otherCharge, _ := other.lease.Charge()
	if charge.SameRegistry(otherCharge) {
		t.Fatal("same-path restart reused old registry")
	}
}

func TestAssemblyRegistryPendingUnknownAndNeverAttemptedRequireRealStop(t *testing.T) {
	r := newAssemblyOriginalRegistry(true, []assemblyOriginalSlot{{path: "pending", state: "planned"}, {path: "untouched", state: "planned"}})
	if r.start() != nil {
		t.Fatal("start")
	}
	id, e := r.open("pending", false, 0)
	if e != nil {
		t.Fatal(e)
	}
	r.retire()
	r.stopUnstarted()
	state, _ := r.lease.RetirementState()
	if state.CollectorStopped || state.Planned != 1 || state.PendingOpen != 1 {
		t.Fatal("refusal/timeout released running collector", state)
	}
	r.acknowledgeOpen(id, false, false)
	r.stop()
	state, _ = r.lease.RetirementState()
	if state.UnknownOpen != 1 || state.Closed != 1 || state.AllSlotsClosed {
		t.Fatal("unknown readback became absence", state)
	}
	if r.beginClose() != nil {
		t.Fatal("metadata cleanup refusal")
	}
	if r.beginSlotClose(id) == nil {
		t.Fatal("unknown original acquired invented handle")
	}
	r.finishClose()
}

func TestAssemblyRegistryDynamicLimitAndZeroForeignChargeRefuse(t *testing.T) {
	r := newAssemblyOriginalRegistry(false, nil)
	if r.start() != nil {
		t.Fatal("start")
	}
	if _, e := r.lease.Charge(); e == nil {
		t.Fatal("dynamic in-flight count treated as complete")
	}
	for i := 0; i < 1024; i++ {
		id, e := r.open(fmt.Sprintf("slot-%d", i), false, 1)
		if e != nil {
			t.Fatal(e)
		}
		r.acknowledgeOpen(id, false, true)
	}
	if id, e := r.open("overflow", false, 1); id != 0 || e == nil {
		t.Fatal("1025th slot admitted", id, e)
	}
	r.stop()
	charge, e := r.lease.Charge()
	if e != nil {
		t.Fatal(e)
	}
	if n, _ := charge.Slots(); n != 1024 {
		t.Fatal(n)
	}
	if _, e := (DocumentImageAssemblyOriginalRegistryCharge{}).Slots(); e == nil {
		t.Fatal("zero opaque value accepted")
	}
	if charge.SameRegistry(DocumentImageAssemblyOriginalRegistryCharge{}) {
		t.Fatal("zero registry equal")
	}
}

func TestAssemblyRegistryRetiredCollectorCannotStartOrOpenAgain(t *testing.T) {
	r := newAssemblyOriginalRegistry(true, []assemblyOriginalSlot{{path: "held", bytes: 1, state: "planned"}})
	r.retire()
	r.stopUnstarted()
	if r.start() == nil {
		t.Fatal("retired one-shot started")
	}
	if _, e := r.open("held", false, 1); e == nil {
		t.Fatal("retired owner dispatched open")
	}
	state, _ := r.lease.RetirementState()
	if !state.CollectorStopped || state.CollectorStarted || !state.AllSlotsClosed {
		t.Fatal(state)
	}
}

func TestAssemblyRegistryForeignCurrentTokenCannotMintLease(t *testing.T) {
	if lease, e := BorrowDocumentImageAssemblyPackageRegistry(context.Background(), assemblyForeignPackageGuard{}); lease != nil || e == nil {
		t.Fatal("foreign interface minted registry")
	}
	if lease, e := BorrowDocumentImageAssemblyEvidenceRegistry(context.Background(), nil); lease != nil || e == nil {
		t.Fatal("nil interface minted registry")
	}
	if lease, e := assemblyRegistryLease(nil); lease != nil || e == nil {
		t.Fatal("nil guard minted registry")
	}
	var absent *DocumentImageAssemblyOriginalRegistryLease
	if _, e := absent.RetirementState(); e == nil {
		t.Fatal("nil lease became known closure")
	}
	if _, e := absent.Charge(); e == nil {
		t.Fatal("nil lease became zero census")
	}
}

func TestAssemblyRegistryZeroAcquisitionAndCopiedClaimRefuseReplay(t *testing.T) {
	evidence := &DocumentImageAssemblyEvidenceAcquisition{}
	packages := &DocumentImageAssemblyPackageAcquisition{}
	evidence.RequestRetirement()
	packages.RequestRetirement()
	if g, e := evidence.Acquire(context.Background()); g != nil || e == nil {
		t.Fatal("zero evidence shell minted original owner")
	}
	if g, e := packages.Acquire(context.Background()); g != nil || e == nil {
		t.Fatal("zero package shell minted original owner")
	}
	if l, e := evidence.RegistryLease(); l != nil || e == nil {
		t.Fatal("zero shell minted registry")
	}
	// Pure state edge, no native collector. Public shell copies retain SAME
	// private claim pointer; one consumed start prevents a second admission.
	state := &assemblyOriginalAcquisition{}
	first := DocumentImageAssemblyEvidenceAcquisition{state: state}
	copyShell := first
	if first.state.begin(context.Background()) != nil || copyShell.state.begin(context.Background()) == nil {
		t.Fatal("copied claim reset one-shot start")
	}
}
