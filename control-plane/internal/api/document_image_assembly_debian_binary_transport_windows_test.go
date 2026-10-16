//go:build windows

package api

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
)

// Lower-edge metadata and captured kernel outcomes only. These fixtures never
// call a DLL, actual guard acquisition, backend loan success, native process,
// crypto/parser, producer constructor/adopter or eligible product profile.
// They intentionally do NOT establish a full positive Reserve/consume journey;
// that requires the real retained package/index/small-source original owners.
func binaryInertTransport(t *testing.T) (*debianProofNativeOwner, *debianBinaryNativeTransport, *inertDebianKernel) {
	t.Helper()
	kernel := &inertDebianKernel{closed: map[windows.Handle]bool{}}
	owner := newDebianProofNativeOwner(context.Background()); owner.calls = kernel.calls()
	owner.calls.BinaryInitAttributes = owner.calls.InitAttributes
	owner.calls.BinaryUpdateAttributes = owner.calls.UpdateAttributes
	owner.calls.BinaryDeleteAttributes = func(p unsafe.Pointer) error { owner.calls.DeleteAttributes(p); return nil }
	owner.census.mu.Lock(); claim, err := owner.reserveBinaryOriginalTransportLocked(); owner.census.mu.Unlock()
	if err != nil { t.Fatal(err) }
	transport := &debianBinaryNativeTransport{owner: owner, claim: claim, consumeReturned: true, native: &debianBinaryTransportNative{}}
	owner.census.mu.Lock(); owner.census.transports = append(owner.census.transports, transport); owner.census.mu.Unlock()
	return owner, transport, kernel
}

func binaryInertAlias(t *testing.T, owner *debianProofNativeOwner, transport *debianBinaryNativeTransport, slot uint16, source windows.Handle) {
	t.Helper()
	transport.native.aliases[slot] = debianBinaryOriginalAlias{issued: true, lookupStarted: true, lookupReturned: true, borrowReturned: true, source: source}
	// Explicit fabricated lower-edge source value, never accepted by production
	// prepare/opaque original getter; not a backend borrow or source proof.
}

func TestDebianBinaryTransportFixed374IsAtomicAndCountsExistingClaims(t *testing.T) {
	for _, before := range []int{650, 651} {
		owner := newDebianProofNativeOwner(context.Background())
		for i := 0; i < before; i++ { if _, err := owner.reserveNativeOriginal(debianProofOriginalJob); err != nil { t.Fatal(err) } }
		owner.census.mu.Lock(); count := len(owner.census.native); claim, err := owner.reserveBinaryOriginalTransportLocked(); owner.census.mu.Unlock()
		if before == 650 {
			if err != nil || claim == nil || owner.census.reserved != 1024 || len(claim.inherited.slots) != 184 || claim.inherited.childOf != claim.pi { t.Fatal("exact fixed374 boundary failed", err) }
		} else if err == nil || claim != nil || owner.census.reserved != before || len(owner.census.native) != count { t.Fatal("over-capacity claim partially allocated") }
	}
}

func TestDebianBinaryTransportForeignAdmissionsRefuseBeforeReserve(t *testing.T) {
	owner := newDebianProofNativeOwner(context.Background()); foreign := newDebianProofNativeOwner(context.Background())
	// Equal metadata is immaterial: exact receiver membership rejects the
	// foreign pointer before a guard getter, quota reservation or native call.
	packageSource := &debianProofSourceAdmission{owner: foreign, consumed: true}
	indexSource := &debianProofSourceAdmission{owner: owner, consumed: true}
	smallSource := &debianProofSourceAdmission{owner: owner, consumed: true}
	owner.census.sources = []*debianProofSourceAdmission{indexSource, smallSource}
	got, err := owner.prepareBinaryReadTransport(context.Background(), debianBinaryTransportSelection{}, &debianProofPackageAdmission{packageSource}, &debianProofEvidenceAdmission{indexSource}, &debianProofEvidenceAdmission{smallSource})
	if err == nil || got != nil || len(owner.census.transports) != 0 || owner.census.reserved != 0 { t.Fatal("foreign same-shape metadata entered Reserve") }
	packageSource.owner = owner; owner.census.sources = append(owner.census.sources, packageSource)
	got, err = owner.prepareBinaryReadTransport(context.Background(), debianBinaryTransportSelection{}, &debianProofPackageAdmission{packageSource}, &debianProofEvidenceAdmission{indexSource}, &debianProofEvidenceAdmission{indexSource})
	if err == nil || got != nil { t.Fatal("same evidence admission filled both exclusive source purposes") }
}

func TestDebianBinaryTransportRetainsPartialReservationBeforeRefusal(t *testing.T) {
	owner, transport, _ := binaryInertTransport(t)
	partial := &dockerbe.DocumentImageAssemblyBinaryReadReservation{}
	transport.reserveStarted = true; transport.preparing = true; transport.retiring.Store(true)
	if transport.finishReservation(context.Background(), partial, errors.New("synthetic partial original return")) == nil || transport.reservation != partial || !transport.reserveReturned || !transport.retiring.Load() { t.Fatal("partial reservation lost before refusal") }
	if owner.settleBinaryReadTransport(context.Background(), transport) == nil || transport.released { t.Fatal("in-flight prepare was treated as unallocated") }
	// This is ONLY the actual-return metadata edge, not a successful real loan.
}

func TestDebianBinaryTransportPendingGetterAndUncertainConsumeStayOwned(t *testing.T) {
	owner, transport, _ := binaryInertTransport(t)
	// Explicit lower metadata latch only, not an actual backend NativeReadHandle
	// success or simulation of cryptographic/native input ownership.
	transport.native.aliases[0] = debianBinaryOriginalAlias{issued: true, borrowStarted: true}
	transport.retire()
	if owner.settleBinaryNativeTransport(transport) { t.Fatal("pending original borrow acquisition became settled") }
	transport.native.aliases[0].borrowAcquireReturned = true
	transport.native.aliases[0].lookupStarted = true
	if owner.settleBinaryNativeTransport(transport) || transport.released { t.Fatal("pending getter became settled on retirement") }
	transport.native.aliases[0].lookupReturned = true
	transport.reservation = &dockerbe.DocumentImageAssemblyBinaryReadReservation{}
	transport.reserveStarted, transport.reserveReturned = true, true
	transport.consumeStarted, transport.consumeReturned = true, true
	if owner.settleBinaryReadTransport(context.Background(), transport) == nil || transport.released || transport.reservation.State().Phase == "unallocated-borrows-released" { t.Fatal("uncertain consume used ReleaseUnallocated") }
}

func TestDebianBinaryTransportChildNamespaceCannotRefundEqualParentValue(t *testing.T) {
	owner, transport, kernel := binaryInertTransport(t)
	if owner.recordKernelReturn(transport.claim.aliases[0], 1) != nil { t.Fatal("fake parent original registration failed") }
	values := make([]uintptr, 184); for i := range values { values[i] = uintptr(i+1) }
	if owner.recordNativeReturn(transport.claim.inherited, values...) != nil { t.Fatal("child slot namespace collided with parent numeric value") }
	if owner.closeKernelOriginal(1) != nil || !kernel.closed[1] || transport.claim.inherited.slots[0].value != 1 || transport.claim.inherited.slots[183].value != 184 { t.Fatal("parent Close refunded equal-valued child entries") }
}

func TestDebianBinaryTransportActualDuplicateFlagsAndPartialACKOwnership(t *testing.T) {
	for _, outcome := range []string{"positive", "partial-error", "missing-success"} { t.Run(outcome, func(t *testing.T) {
		owner, transport, kernel := binaryInertTransport(t); binaryInertAlias(t, owner, transport, 0, 7000)
		calls := 0
		owner.calls.Duplicate = func(from, source, into windows.Handle, out *windows.Handle, access uint32, inherit bool, options uint32) error {
			calls++; if from != windows.CurrentProcess() || into != windows.CurrentProcess() || source != 7000 || access != windows.GENERIC_READ || !inherit || options != 0 { t.Error("fixed readonly duplicate arguments changed") }
			if outcome != "missing-success" { *out = 8000 }
			if outcome == "partial-error" { return errors.New("synthetic BOOL failure with valid original ACK") }; return nil
		}
		err := owner.duplicateBinaryOriginalAtReturn(context.Background(), transport, 0)
		if calls != 1 || (err == nil) != (outcome == "positive") { t.Fatal("actual fixed native edge classified incorrectly", err) }
		if owner.duplicateBinaryOriginalAtReturn(context.Background(), transport, 0) == nil || calls != 1 { t.Fatal("same slot duplicated twice") }
		foreign := newDebianProofNativeOwner(context.Background())
		if foreign.duplicateBinaryOriginalAtReturn(context.Background(), transport, 0) == nil || calls != 1 { t.Fatal("foreign receiver consumed issued original alias state") }
		alias := &transport.native.aliases[0]
		if outcome == "missing-success" {
			if !transport.claim.aliases[0].slots[0].unknown || owner.releaseUnattemptedBinaryClaim(transport.claim) || owner.originalCensusSettled() { t.Fatal("missing native output refunded original charge") }
			return
		}
		if alias.handle != 8000 || transport.claim.aliases[0].slots[0].value != 8000 { t.Fatal("valid partial alias disappeared before cleanup") }
		transport.retire(); _ = owner.releaseUnattemptedBinaryClaim(transport.claim)
		if !owner.settleBinaryNativeTransport(transport) || !kernel.closed[8000] || alias.handle != 0 || !owner.releaseUnattemptedBinaryClaim(transport.claim) { t.Fatal("exact fake original alias did not close") }
	}) }
}

func TestDebianBinaryTransportBlockedDuplicateRetirementRetainsLateACK(t *testing.T) {
	owner, transport, kernel := binaryInertTransport(t); binaryInertAlias(t, owner, transport, 0, 7000)
	entered, release := make(chan struct{}), make(chan struct{}); var once sync.Once; unblock := func() { once.Do(func() { close(release) }) }; defer unblock()
	owner.calls.Duplicate = func(_, _ windows.Handle, _ windows.Handle, out *windows.Handle, _ uint32, _ bool, _ uint32) error { *out = 8000; close(entered); <-release; return nil }
	finished := make(chan error, 1); go func() { finished <- owner.duplicateBinaryOriginalAtReturn(context.Background(), transport, 0) }()
	select { case <-entered: case <-time.After(time.Second): t.Fatal("fake Duplicate not reached") }
	retired := make(chan struct{}); go func() { owner.retire(); close(retired) }()
	select { case <-retired: case <-time.After(time.Second): t.Fatal("original Duplicate blocked independent retirement") }
	if !transport.claim.aliases[0].pending || owner.settleBinaryNativeTransport(transport) || kernel.closed[8000] { t.Fatal("pending original output was closed or declared absent") }
	unblock(); select { case err := <-finished: if err == nil { t.Fatal("late ACK became new work") }; case <-time.After(time.Second): t.Fatal("late original did not return") }
	if transport.native.aliases[0].handle != 8000 || transport.claim.aliases[0].pending { t.Fatal("late valid original ACK was not retained") }
	_ = owner.releaseUnattemptedBinaryClaim(transport.claim)
	if !owner.settleBinaryNativeTransport(transport) || !kernel.closed[8000] { t.Fatal("late exact fake original failed positive Close") }
}

func TestDebianBinaryTransportFailedAliasCloseRetainsOriginal(t *testing.T) {
	owner, transport, kernel := binaryInertTransport(t); binaryInertAlias(t, owner, transport, 0, 7000)
	owner.calls.Duplicate = func(_, _ windows.Handle, _ windows.Handle, out *windows.Handle, _ uint32, _ bool, _ uint32) error { *out = 8000; return nil }
	if owner.duplicateBinaryOriginalAtReturn(context.Background(), transport, 0) != nil { t.Fatal("fake duplicate failed") }
	kernel.failClose = 8000; transport.retire(); _ = owner.releaseUnattemptedBinaryClaim(transport.claim)
	if owner.settleBinaryNativeTransport(transport) || transport.native.aliases[0].handle != 8000 || !transport.claim.aliases[0].slots[0].unknown || kernel.closed[8000] { t.Fatal("failed Close discarded original or fabricated positive settlement") }
}

func TestDebianBinaryTransportBlockedAliasCloseCannotBlockRetireLatch(t *testing.T) {
	owner, transport, _ := binaryInertTransport(t); binaryInertAlias(t, owner, transport, 0, 7000)
	owner.calls.Duplicate = func(_, _ windows.Handle, _ windows.Handle, out *windows.Handle, _ uint32, _ bool, _ uint32) error { *out = 8000; return nil }
	if owner.duplicateBinaryOriginalAtReturn(context.Background(), transport, 0) != nil { t.Fatal("fake duplicate failed") }
	entered, release := make(chan struct{}), make(chan struct{}); var once sync.Once; unblock := func() { once.Do(func() { close(release) }) }; defer unblock()
	owner.calls.Close = func(handle windows.Handle) error { if handle != 8000 { t.Error("wrong fake original Close") }; close(entered); <-release; return nil }
	finished := make(chan bool, 1); go func() { finished <- owner.settleBinaryNativeTransport(transport) }()
	select { case <-entered: case <-time.After(time.Second): t.Fatal("fake Close not entered") }
	retired := make(chan struct{}); go func() { owner.retire(); close(retired) }()
	select { case <-retired: case <-time.After(time.Second): t.Fatal("blocked original Close held retirement lock") }
	if transport.native.aliases[0].handle != 8000 || !transport.claim.aliases[0].slots[0].closing { t.Fatal("blocked Close discarded original backing") }
	unblock(); select { case positive := <-finished: if !positive { t.Fatal("actual fake Close return failed settlement") }; case <-time.After(time.Second): t.Fatal("fake Close failed to join") }
}

func binaryInertStartup(t *testing.T) (*debianProofNativeOwner, *debianBinaryNativeTransport, *debianBinaryStartupBorrow, *inertDebianKernel) {
	t.Helper(); owner, transport, kernel := binaryInertTransport(t)
	for i := range transport.native.aliases {
		handle := windows.Handle(1000+i); transport.native.aliases[i] = debianBinaryOriginalAlias{handle: handle, issued: true, lookupReturned: true, duplicateStarted: true, duplicateReturned: true, borrowReturned: true}
		if owner.recordKernelReturn(transport.claim.aliases[i], handle) != nil { t.Fatal("fake alias ACK failed") }
	}
	job := &debianProofJob{handle: 77}; owner.jobs = append(owner.jobs, job)
	life := &debianProofLifetime{job: job, piTicket: transport.claim.pi, inherited: transport.claim.inherited, handles: []windows.Handle{101,102,103,104,105,106}, readersSettled: true}
	owner.lifetimes = append(owner.lifetimes, life)
	_ = owner.recordKernelReturn(transport.claim.job, 77)
	for i, pair := range [][2]windows.Handle{{101,102},{103,104},{105,106}} { _ = owner.recordKernelReturn(transport.claim.pipes[i], pair[0], pair[1]) }
	updates := 0
	owner.calls.BinaryUpdateAttributes = func(_ unsafe.Pointer, key uintptr, value unsafe.Pointer, bytes uintptr) error {
		updates++
		if key == windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST {
			if bytes != unsafe.Sizeof([184]windows.Handle{}) { t.Error("variable inherited list length") }
			list := (*[184]windows.Handle)(value)
			if list[0] != 105 || list[1] != 102 || list[2] != 104 { t.Error("stdio list included wrong parent ends") }
			for i := 0; i < 181; i++ { if list[3+i] != windows.Handle(1000+i) { t.Error("fixed original alias ordinal lost") } }
		} else if key != 0x2000D || *(*windows.Handle)(value) != 77 { t.Error("wrong original Job attribute") }
		return nil
	}
	b, err := owner.prepareBinaryInheritedHandleList(context.Background(), transport, life)
	if err != nil || b == nil || !b.prepared || updates != 2 || kernel.created.Load() != 0 || kernel.resumed.Load() != 0 { t.Fatal("fixed startup borrowing failed or launched native work", err) }
	return owner, transport, b, kernel
}

func TestDebianBinaryTransportFixed184StartupBorrowsUntilOriginalCreateReturn(t *testing.T) {
	owner, transport, startup, kernel := binaryInertStartup(t)
	if owner.beginBinaryStartupCreate(context.Background(), transport, startup) != nil { t.Fatal("fake original Create borrow failed") }
	transport.retire()
	if owner.settleBinaryNativeTransport(transport) || kernel.deleted.Load() || kernel.closed[1000] { t.Fatal("startup borrow released aliases/attributes before original return") }
	if owner.recordBinaryOriginalCreateReturn(transport, startup, windows.ProcessInformation{Process: 201,Thread: 202}, errors.New("synthetic late valid PI plus error")) == nil { t.Fatal("late original PI error was hidden") }
	if !startup.createReturned || startup.life.job.generation != 1 || transport.claim.inherited.slots[183].value == 0 { t.Fatal("partial late ACK lost generation/child184") }
	if !owner.settleBinaryNativeTransport(transport) || !kernel.deleted.Load() || !kernel.closed[1000] || owner.settleInheritedStdio(startup.life) { t.Fatal("parent alias closure settled live child derivatives") }
	for i := range transport.native.aliases { if !kernel.closed[windows.Handle(1000+i)] { t.Fatal("fake allocated alias lost") } }
}

func TestDebianBinaryTransportChild184NeedsJointOriginalProcessJobSettlement(t *testing.T) {
	owner, transport, startup, kernel := binaryInertStartup(t)
	if owner.beginBinaryStartupCreate(context.Background(), transport, startup) != nil || owner.recordBinaryOriginalCreateReturn(transport, startup, windows.ProcessInformation{Process:201,Thread:202}, nil) != nil { t.Fatal("fake original return failed") }
	transport.retire(); if !owner.settleBinaryNativeTransport(transport) { t.Fatal("parent fake aliases failed Close") }
	if owner.settleInheritedStdio(startup.life) { t.Fatal("parent Close counted as child exit") }
	startup.life.processSettled = true
	if owner.settleInheritedStdio(startup.life) { t.Fatal("process-only fact counted as Job settlement") }
	if owner.closeKernelOriginal(77) != nil || !kernel.closed[77] { t.Fatal("fake original Job failed positive Close") }
	startup.life.job.handle = 0; startup.life.job.settled = true; startup.life.job.borrowers = 1
	if owner.settleInheritedStdio(startup.life) { t.Fatal("LOCAL132 borrower lost child copies") }
	startup.life.job.borrowers = 0
	if !owner.settleInheritedStdio(startup.life) || transport.claim.inherited.slots[183].value != 0 { t.Fatal("joint exact fake original settlement failed") }
}

func TestDebianBinaryTransportFailedAttributeDeleteRetainsStartupAndAliases(t *testing.T) {
	owner, transport, startup, kernel := binaryInertStartup(t)
	owner.calls.BinaryDeleteAttributes = func(unsafe.Pointer) error { return errors.New("synthetic missing delete export") }
	transport.retire()
	if owner.settleBinaryNativeTransport(transport) || startup.deleted || kernel.closed[1000] || transport.native.aliases[0].handle != 1000 { t.Fatal("unacknowledged attribute deletion released startup/aliases") }
}

func TestDebianBinaryTransportCreateKnownAbsenceDiffersFromMissingSuccess(t *testing.T) {
	for _, knownAbsent := range []bool{true,false} {
		owner, transport, startup, _ := binaryInertStartup(t)
		if owner.beginBinaryStartupCreate(context.Background(), transport, startup) != nil { t.Fatal("fake startup borrow failed") }
		var callErr error; if knownAbsent { callErr = errors.New("synthetic original false/no PI") }
		err := owner.recordBinaryOriginalCreateReturn(transport, startup, windows.ProcessInformation{}, callErr)
		if knownAbsent {
			if err == nil || !owner.settleInheritedStdio(startup.life) || transport.claim.inherited.slots[183].value != 0 { t.Fatal("known unallocated child copies held as allocated or error hidden") }
		} else if err == nil || owner.settleInheritedStdio(startup.life) || !transport.claim.pi.slots[0].unknown || transport.claim.inherited.slots[183].value == 0 { t.Fatal("success without original identity fabricated absence") }
	}
}
