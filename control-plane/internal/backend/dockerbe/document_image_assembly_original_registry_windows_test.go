//go:build windows

package dockerbe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func assemblyRegistryObject(path string) backend.DocumentImageAssemblyEvidenceObject {
	return backend.DocumentImageAssemblyEvidenceObject{Role: "sample", Path: path, Bytes: 1, Digest: strings.Repeat("a", 64)}
}

// This deferred fixture uses real Windows TEMP-file ordinary handles only.
// Digest is intentionally declared metadata, not authenticated payload proof.
func TestAssemblyRegistryPreparedNoOpenAndLifetimeSurvivesTokenTransfer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	objects := []backend.DocumentImageAssemblyEvidenceObject{assemblyRegistryObject(path)}
	a, err := PrepareDocumentImageAssemblyEvidenceAcquisition(objects)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.RegistryLease()
	if err != nil {
		t.Fatal(err)
	}
	charge, err := lease.Charge()
	if err != nil {
		t.Fatal(err)
	}
	state, _ := lease.RetirementState()
	if state.CollectorStarted || state.PendingOpen != 0 || state.ValidACK != 0 || state.Planned < 2 {
		t.Fatal("pure prepare performed original opens", state)
	}
	objects[0].Path = "not-the-held-path"
	objects[0].Digest = strings.Repeat("b", 64)
	guard, err := a.Acquire(context.Background())
	if guard != nil {
		defer func() { _ = guard.Close() }()
	}
	if err != nil {
		t.Fatal(err)
	}
	// Copying the exported shell cannot clone the private one-shot claim.
	copyAcquisition := *a
	if duplicate, e := copyAcquisition.Acquire(context.Background()); duplicate != nil || e == nil {
		t.Fatal("copied shell replayed acquisition")
	}
	borrowed, err := BorrowDocumentImageAssemblyEvidenceRegistry(context.Background(), guard)
	if err != nil {
		t.Fatal(err)
	}
	borrowCharge, _ := borrowed.Charge()
	if !charge.SameRegistry(borrowCharge) {
		t.Fatal("actual guard changed prepared registry")
	}
	old := guard
	next, err := old.Transfer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	guard = next
	if stale, e := BorrowDocumentImageAssemblyEvidenceRegistry(context.Background(), old); stale != nil || e == nil {
		t.Fatal("stale generation borrowed registry")
	}
	current, err := BorrowDocumentImageAssemblyEvidenceRegistry(context.Background(), guard)
	if err != nil {
		t.Fatal(err)
	}
	currentCharge, _ := current.Charge()
	if !charge.SameRegistry(currentCharge) {
		t.Fatal("token move changed stable registry lifetime")
	}
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	state, _ = lease.RetirementState()
	if !state.AllSlotsClosed || !state.CollectorStopped {
		t.Fatal("exact original close did not settle", state)
	}
	other, err := PrepareDocumentImageAssemblyEvidenceAcquisition([]backend.DocumentImageAssemblyEvidenceObject{assemblyRegistryObject(path)})
	if err != nil {
		t.Fatal(err)
	}
	other.RequestRetirement()
	otherLease, _ := other.RegistryLease()
	otherCharge, _ := otherLease.Charge()
	if charge.SameRegistry(otherCharge) {
		t.Fatal("same path restart reused old lifetime")
	}
}

func TestAssemblyRegistryCloseCompactionRetainsStableFailedSlot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original")
	if e := os.WriteFile(path, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	a, e := PrepareDocumentImageAssemblyEvidenceAcquisition([]backend.DocumentImageAssemblyEvidenceObject{assemblyRegistryObject(path)})
	if e != nil {
		t.Fatal(e)
	}
	guard, e := a.Acquire(context.Background())
	if guard != nil {
		defer guard.Close()
	}
	if e != nil {
		t.Fatal(e)
	}
	g := a.state.guard
	lease, _ := a.RegistryLease()
	charge, _ := lease.Charge()
	retained := g.handles[0]
	g.closeHandle = func(h syscall.Handle) error {
		if h == retained.handle {
			return errors.New("synthetic exact close refusal")
		}
		return syscall.CloseHandle(h)
	}
	if guard.Close() == nil || len(g.handles) != 1 || g.handles[0].slotID != retained.slotID {
		t.Fatal("compaction lost stable failed slot")
	}
	state, _ := lease.RetirementState()
	if state.UnknownClose != 1 || state.AllSlotsClosed {
		t.Fatal("failed close invented closure", state)
	}
	g.closeHandle = nil
	if guard.Close() != nil {
		t.Fatal("exact retained original failed closure")
	}
	state, _ = lease.RetirementState()
	still, _ := lease.Charge()
	if !state.AllSlotsClosed || !charge.SameRegistry(still) {
		t.Fatal("close lost lifetime", state)
	}
	before, _ := charge.Slots()
	after, _ := still.Slots()
	if before != after {
		t.Fatal("compaction refunded capacity")
	}
}

// Actual prepared/collector code with ONLY a fake CreateFile and fake Close
// outcome. No native file/API is called by this fixture. Valid ACK+error must
// survive an independently requested retirement while Create is blocked.
func TestAssemblyRegistryBlockedOriginalACKCannotCloseOrSettleEarly(t *testing.T) {
	a, e := PrepareDocumentImageAssemblyEvidenceAcquisition([]backend.DocumentImageAssemblyEvidenceObject{assemblyRegistryObject(`X:\unopened-original`)})
	if e != nil {
		t.Fatal(e)
	}
	g := a.state.guard
	lease, _ := a.RegistryLease()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	var nativeClose atomic.Int32
	g.openFile = func(*uint16, uint32, uint32, *syscall.SecurityAttributes, uint32, uint32, int32) (syscall.Handle, error) {
		close(entered)
		<-release
		return 700, errors.New("synthetic valid ACK plus error")
	}
	g.closeHandle = func(h syscall.Handle) error {
		if h != 700 {
			t.Error("foreign original close")
		}
		nativeClose.Add(1)
		return nil
	}
	type response struct {
		guard backend.DocumentImageAssemblyEvidenceGuard
		err   error
	}
	done := make(chan response, 1)
	go func() { guard, err := a.Acquire(context.Background()); done <- response{guard, err} }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("actual collector did not enter fake native Create")
	}
	a.RequestRetirement()
	// Even a separately held original IO mutex cannot block metadata snapshots.
	g.mu.Lock()
	snapshotDone := make(chan DocumentImageAssemblyOriginalRegistryRetirement, 1)
	go func() { state, _ := lease.RetirementState(); snapshotDone <- state }()
	select {
	case state := <-snapshotDone:
		if !state.Retiring || state.CollectorStopped || state.PendingOpen != 1 {
			g.mu.Unlock()
			t.Fatal("pending registry state lost", state)
		}
	case <-time.After(time.Second):
		g.mu.Unlock()
		t.Fatal("snapshot waited on original IO mutex")
	}
	g.mu.Unlock()
	if g.Close() == nil || nativeClose.Load() != 0 {
		t.Fatal("running collector allowed original close")
	}
	once.Do(func() { close(release) })
	var result response
	select {
	case result = <-done:
	case <-time.After(time.Second):
		t.Fatal("actual returning collector did not settle")
	}
	if result.guard == nil || result.err == nil {
		t.Fatal("late partial ownership lost")
	}
	defer result.guard.Close()
	state, _ := lease.RetirementState()
	if !state.CollectorStopped || state.ValidACK != 1 || state.AllSlotsClosed {
		t.Fatal("late valid ACK not retained", state)
	}
	if result.guard.Close() != nil || nativeClose.Load() != 1 {
		t.Fatal("exact partial original did not close once")
	}
	state, _ = lease.RetirementState()
	if !state.AllSlotsClosed {
		t.Fatal(state)
	}
}

func TestAssemblyRegistryInvalidACKAndUnknownZeroAreDistinct(t *testing.T) {
	for _, which := range []string{"invalid", "unknown-zero"} {
		t.Run(which, func(t *testing.T) {
			a, e := PrepareDocumentImageAssemblyEvidenceAcquisition([]backend.DocumentImageAssemblyEvidenceObject{assemblyRegistryObject(`X:\unopened-original`)})
			if e != nil {
				t.Fatal(e)
			}
			g := a.state.guard
			g.openFile = func(*uint16, uint32, uint32, *syscall.SecurityAttributes, uint32, uint32, int32) (syscall.Handle, error) {
				if which == "invalid" {
					return syscall.InvalidHandle, errors.New("synthetic absent ACK")
				}
				return 0, nil
			}
			g.closeHandle = func(syscall.Handle) error { t.Fatal("invalid/zero native value was closed"); return nil }
			guard, e := a.Acquire(context.Background())
			if guard == nil || e == nil {
				t.Fatal("invalid ACK accepted")
			}
			lease, _ := a.RegistryLease()
			state, _ := lease.RetirementState()
			if which == "invalid" {
				if state.UnknownOpen != 0 || !state.AllSlotsClosed || guard.Close() != nil {
					t.Fatal("actual invalid ACK not known absent", state)
				}
			} else {
				if state.UnknownOpen != 1 || state.AllSlotsClosed || guard.Close() == nil {
					t.Fatal("unknown zero became positive closure", state)
				}
			}
		})
	}
}

func TestAssemblyRegistryPackageFactoryPreservesReadFreeFixedLimits(t *testing.T) {
	objects := assemblyPackageInputFixture(t)
	a, e := PrepareDocumentImageAssemblyPackageAcquisition(objects)
	if e != nil {
		t.Fatal(e)
	}
	lease, _ := a.RegistryLease()
	state, _ := lease.RetirementState()
	if state.ValidACK != 0 || state.Planned < 174 {
		t.Fatal("package prepare opened originals", state)
	}
	guard, e := a.Acquire(context.Background())
	if guard != nil {
		defer guard.Close()
	}
	if e != nil {
		t.Fatal(e)
	}
	borrow, e := BorrowDocumentImageAssemblyPackageRegistry(context.Background(), guard)
	if e != nil {
		t.Fatal(e)
	}
	x, _ := lease.Charge()
	y, _ := borrow.Charge()
	if !x.SameRegistry(y) {
		t.Fatal("package factory lost registry")
	}
	if e = guard.Close(); e != nil {
		t.Fatal(e)
	}
	state, _ = lease.RetirementState()
	if !state.AllSlotsClosed {
		t.Fatal(state)
	}
}
