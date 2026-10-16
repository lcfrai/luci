//go:build windows

package api

import (
	"context"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
)

type debianBinaryOriginalAlias struct {
	borrow *dockerbe.DocumentImageAssemblyBinaryOriginalBorrow
	handle, source windows.Handle
	issued, borrowStarted, borrowAcquireReturned, lookupStarted, lookupReturned, duplicateStarted, duplicateReturned bool
	borrowReturnStarted, borrowReturned, closing bool
}

type debianBinaryTransportNative struct {
	aliases [181]debianBinaryOriginalAlias
	startup *debianBinaryStartupBorrow
}

type debianBinaryStartupBorrow struct {
	transport *debianBinaryNativeTransport
	life *debianProofLifetime
	handles [184]windows.Handle
	jobs [1]windows.Handle
	attributes []byte
	attributePointer unsafe.Pointer
	preparing, prepared, initialized, deleting, deleted bool
	createStarted, createReturning, createReturned bool
}

func newDebianBinaryTransportNative(*debianBinaryNativeTransport) *debianBinaryTransportNative { return &debianBinaryTransportNative{} }

func (o *debianProofNativeOwner) ownsBinaryTransport(t *debianBinaryNativeTransport) bool {
	if o == nil || o.census == nil || t == nil || t.owner != o { return false }
	c := o.census; c.mu.Lock(); defer c.mu.Unlock()
	for _, issued := range c.transports { if issued == t { return true } }
	return false
}

func (o *debianProofNativeOwner) duplicateBinaryOriginalAlias(ctx context.Context, t *debianBinaryNativeTransport, slot uint16) error {
	if ctx == nil || !o.ownsBinaryTransport(t) || slot >= 181 { return errDebianProofCensus }
	t.mu.Lock()
	if t.preparing || !t.consumeReturned || t.transport == nil || t.native == nil || t.claim == nil || t.retiring.Load() || o.nativeRetired() || ctx.Err() != nil || !time.Now().Before(o.workLimit()) || t.native.aliases[slot].issued {
		t.mu.Unlock(); return errDebianProofCensus
	}
	alias := &t.native.aliases[slot]; alias.issued = true; alias.borrowStarted = true
	transport := t.transport
	t.mu.Unlock()
	borrow, err := transport.BorrowOriginalForDuplicate(ctx, slot)
	t.mu.Lock(); alias.borrow = borrow; alias.borrowAcquireReturned = true; t.mu.Unlock() // retain partial output before classification
	if err != nil || borrow == nil {
		if borrow != nil { t.mu.Lock(); alias.lookupReturned = true; t.mu.Unlock(); _ = o.returnBinaryOriginalBorrow(t, slot) }
		t.retire(); if err == nil { err = errDebianProofCensus }; return err
	}
	t.mu.Lock(); alias.lookupStarted = true; t.mu.Unlock()
	source, err := borrow.NativeReadHandle()
	t.mu.Lock(); alias.source = windows.Handle(source); alias.lookupReturned = true; retired := t.retiring.Load() || o.nativeRetired() || ctx.Err() != nil; t.mu.Unlock()
	if err == nil && !retired { err = o.duplicateBinaryOriginalAtReturn(ctx, t, slot) }
	returnErr := o.returnBinaryOriginalBorrow(t, slot)
	if err != nil || retired || returnErr != nil { t.retire(); if err == nil { err = errDebianProofCensus } }
	return err
}

// The sole production writer of alias.source is the sealed original getter
// above. Tests may fabricate this lower edge and explicitly cannot mint a run.
func (o *debianProofNativeOwner) duplicateBinaryOriginalAtReturn(ctx context.Context, t *debianBinaryNativeTransport, slot uint16) error {
	if ctx == nil || !o.ownsBinaryTransport(t) || slot >= 181 { return errDebianProofCensus }
	t.mu.Lock(); if t.native == nil || t.claim == nil { t.mu.Unlock(); return errDebianProofCensus }; alias := &t.native.aliases[slot]
	if !alias.issued || !alias.lookupReturned || !validDebianProducedHandle(alias.source) || alias.duplicateStarted || t.retiring.Load() || ctx.Err() != nil { t.mu.Unlock(); return errDebianProofCensus }
	source := alias.source; t.mu.Unlock()
	if err := o.beginBinaryOriginal(t.claim, t.claim.aliases[slot]); err != nil { return err }
	t.mu.Lock(); alias.duplicateStarted = true; t.mu.Unlock()
	var handle windows.Handle
	err := o.calls.Duplicate(windows.CurrentProcess(), source, windows.CurrentProcess(), &handle, windows.GENERIC_READ, true, 0)
	t.mu.Lock(); alias.handle = debianProofOwnedKernel(handle); alias.duplicateReturned = true; t.mu.Unlock()
	ack := o.recordKernelCallReturn(t.claim.aliases[slot], err, handle)
	if ack != nil { err = ack }
	if err == nil && (!validDebianProducedHandle(handle) || t.retiring.Load() || o.nativeRetired() || ctx.Err() != nil || !time.Now().Before(o.workLimit())) { err = errDebianProofCensus }
	if err != nil { t.retire() }
	return err
}

func (o *debianProofNativeOwner) returnBinaryOriginalBorrow(t *debianBinaryNativeTransport, slot uint16) error {
	t.mu.Lock(); alias := &t.native.aliases[slot]
	if alias.borrow == nil { t.mu.Unlock(); return errDebianProofCensus }
	if !alias.lookupReturned || alias.duplicateStarted && !alias.duplicateReturned || alias.borrowReturnStarted { t.mu.Unlock(); return errDebianProofCensus }
	alias.borrowReturnStarted = true; borrow := alias.borrow; t.mu.Unlock()
	err := borrow.ReturnFromDuplicate()
	t.mu.Lock(); alias.borrowReturned = err == nil; t.mu.Unlock()
	return err
}

// Derive the list from this SAME owner's registered life/job/pipe/alias values.
// No HANDLE, argv, arbitrary list or count is accepted. This prepares borrowing
// only; no binary Create/Resume/purpose/resource arm is implemented or enabled.
func (o *debianProofNativeOwner) prepareBinaryInheritedHandleList(ctx context.Context, t *debianBinaryNativeTransport, life *debianProofLifetime) (*debianBinaryStartupBorrow, error) {
	if ctx == nil || ctx.Err() != nil || !o.ownsBinaryTransport(t) || life == nil { return nil, errDebianProofCensus }
	o.mu.Lock(); ownedLife, ownedJob := false, false
	for _, old := range o.lifetimes { ownedLife = ownedLife || old == life }
	for _, old := range o.jobs { ownedJob = ownedJob || old == life.job }
	if !ownedLife || !ownedJob || len(life.handles) != 6 { o.mu.Unlock(); return nil, errDebianProofCensus }
	stdio := [3]windows.Handle{life.handles[4], life.handles[1], life.handles[3]}; o.mu.Unlock()
	if t.claim == nil || life.piTicket != t.claim.pi || life.inherited != t.claim.inherited { return nil, errDebianProofCensus }
	c := o.census; c.mu.Lock()
	valid := !c.retiring && !o.nativeRetired() && time.Now().Before(o.workLimit())
	for _, ticket := range append([]*debianProofOriginalTicket{t.claim.job}, t.claim.pipes[:]...) { valid = valid && c.hasNativeLocked(ticket) && !ticket.pending }
	for _, ticket := range append([]*debianProofOriginalTicket{t.claim.job}, t.claim.pipes[:]...) { for _, slot := range ticket.slots { valid = valid && !slot.unknown && !slot.closing } }
	valid = valid && t.claim.pi.pending && !t.claim.pi.attempted && t.claim.inherited.pending && !t.claim.inherited.attempted
	valid = valid && t.claim.job.slots[0].value != 0 && t.claim.pipes[2].slots[0].value == uintptr(stdio[0]) && t.claim.pipes[0].slots[1].value == uintptr(stdio[1]) && t.claim.pipes[1].slots[1].value == uintptr(stdio[2])
	job := windows.Handle(t.claim.job.slots[0].value); c.mu.Unlock()
	if !valid { return nil, errDebianProofCensus }
	life.job.mu.Lock(); valid = life.job.handle == job && !life.job.closing && !life.job.retirementRequested; life.job.mu.Unlock()
	if !valid { return nil, errDebianProofCensus }
	t.mu.Lock()
	if t.native == nil || t.native.startup != nil || t.retiring.Load() || t.preparing || !t.consumeReturned { t.mu.Unlock(); return nil, errDebianProofCensus }
	b := &debianBinaryStartupBorrow{transport: t, life: life, jobs: [1]windows.Handle{job}, preparing: true}
	copy(b.handles[:3], stdio[:])
	for i := range t.native.aliases {
		alias := &t.native.aliases[i]
		if !alias.duplicateReturned || !alias.borrowReturned || alias.closing || !validDebianProducedHandle(alias.handle) { t.mu.Unlock(); return nil, errDebianProofCensus }
		b.handles[3+i] = alias.handle
	}
	t.native.startup = b // original backing retained BEFORE attribute/native calls
	t.mu.Unlock()
	defer func() { t.mu.Lock(); b.preparing = false; t.mu.Unlock() }()
	var bytes uintptr
	_ = o.calls.BinaryInitAttributes(nil, &bytes)
	if bytes == 0 || bytes > 64<<10 { t.retire(); return b, errDebianProofCensus }
	b.attributes = make([]byte, bytes); b.attributePointer = unsafe.Pointer(&b.attributes[0])
	if err := o.calls.BinaryInitAttributes(b.attributePointer, &bytes); err != nil { t.retire(); return b, err }
	t.mu.Lock(); b.initialized = true; t.mu.Unlock()
	if err := o.calls.BinaryUpdateAttributes(b.attributePointer, windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&b.handles[0]), unsafe.Sizeof(b.handles)); err != nil { t.retire(); return b, err }
	if err := o.calls.BinaryUpdateAttributes(b.attributePointer, 0x2000D, unsafe.Pointer(&b.jobs[0]), unsafe.Sizeof(b.jobs)); err != nil { t.retire(); return b, err }
	t.mu.Lock(); b.prepared = !t.retiring.Load() && !o.nativeRetired() && ctx.Err() == nil && time.Now().Before(o.workLimit()); prepared := b.prepared; t.mu.Unlock()
	if !prepared { t.retire(); return b, errDebianProofCensus }
	return b, nil
}

// Future exact original Create callsite must borrow these fixed backings once.
// No Create is called here; this does not authorize a binary purpose or Resume.
func (o *debianProofNativeOwner) beginBinaryStartupCreate(ctx context.Context, t *debianBinaryNativeTransport, b *debianBinaryStartupBorrow) error {
	if ctx == nil || !o.ownsBinaryTransport(t) || b == nil || b.transport != t { return errDebianProofCensus }
	t.mu.Lock()
	if b.preparing || !b.prepared || b.createStarted || b.deleting || b.deleted || t.retiring.Load() || ctx.Err() != nil { t.mu.Unlock(); return errDebianProofCensus }
	// Mark the startup borrower before touching census; retirement cannot free
	// attributes/backings while this exact callsite is between the two locks.
	b.createStarted = true; t.mu.Unlock()
	c := o.census; c.mu.Lock()
	valid := !c.retiring && !o.nativeRetired() && time.Now().Before(o.workLimit()) && t.claim.pi.pending && !t.claim.pi.attempted && t.claim.inherited.pending && !t.claim.inherited.attempted
	if valid { t.claim.pi.attempted = true; t.claim.inherited.attempted = true }
	c.mu.Unlock()
	if !valid { t.mu.Lock(); b.createReturned = true; t.mu.Unlock(); t.retire(); return errDebianProofCensus }
	return nil
}

// Original-return bookkeeping only, not an external PI receipt/adopter. The
// future fixed Create callsite is absent; tests exercise deterministic outputs.
func (o *debianProofNativeOwner) recordBinaryOriginalCreateReturn(t *debianBinaryNativeTransport, b *debianBinaryStartupBorrow, pi windows.ProcessInformation, callErr error) error {
	if !o.ownsBinaryTransport(t) || b == nil || b.transport != t { return errDebianProofCensus }
	t.mu.Lock(); if !b.createStarted || b.createReturning || b.createReturned { t.mu.Unlock(); return errDebianProofCensus }; b.createReturning = true; t.mu.Unlock()
	pi.Process, pi.Thread = debianProofOwnedKernel(pi.Process), debianProofOwnedKernel(pi.Thread)
	o.mu.Lock(); b.life.handles = append(b.life.handles, pi.Process, pi.Thread); o.mu.Unlock()
	if pi.Process != 0 || pi.Thread != 0 {
		j := b.life.job; j.mu.Lock(); j.generation = 1; borrower := takeDebianProofTerminationLocked(j); j.mu.Unlock()
		if borrower != nil { go o.terminateOriginalJob(borrower) }
	}
	ack := o.recordKernelCallReturn(t.claim.pi, callErr, pi.Process, pi.Thread)
	values := make([]uintptr, 184)
	if callErr == nil || pi.Process != 0 || pi.Thread != 0 { for i := range values { values[i] = uintptr(i+1) } }
	inherited := o.recordNativeReturn(t.claim.inherited, values...)
	runtime.KeepAlive(b.attributes); runtime.KeepAlive(b.handles); runtime.KeepAlive(b.jobs)
	t.mu.Lock(); b.createReturning = false; b.createReturned = true; t.mu.Unlock()
	if ack != nil { return ack }; if inherited != nil { return inherited }; if callErr != nil { return callErr }
	if t.retiring.Load() || o.nativeRetired() { return errDebianProofCensus }; return nil
}

func (o *debianProofNativeOwner) settleBinaryNativeTransport(t *debianBinaryNativeTransport) bool {
	if !o.ownsBinaryTransport(t) { return false }
	t.mu.Lock(); native := t.native
	if native == nil { t.mu.Unlock(); return true }
	b := native.startup
	if b != nil && (b.preparing || b.createStarted && !b.createReturned || b.deleting) { t.mu.Unlock(); return false }
	if b != nil && b.initialized && !b.deleted {
		b.deleting = true; t.mu.Unlock(); err := o.calls.BinaryDeleteAttributes(b.attributePointer); t.mu.Lock(); b.deleting = false
		if err != nil { t.mu.Unlock(); return false }; b.deleted = true
	}
	for i := range native.aliases {
		alias := &native.aliases[i]
		if alias.borrowStarted && !alias.borrowAcquireReturned || alias.lookupStarted && !alias.lookupReturned || alias.duplicateStarted && !alias.duplicateReturned || alias.borrow != nil && !alias.borrowReturned || alias.closing { t.mu.Unlock(); return false }
		if alias.handle == 0 { continue }
		alias.closing = true; handle := alias.handle; t.mu.Unlock()
		err := o.closeKernelOriginal(handle)
		t.mu.Lock(); alias.closing = false; if err != nil { t.mu.Unlock(); return false }; alias.handle = 0
	}
	t.mu.Unlock()
	return true
}

func (o *debianProofNativeOwner) settleBinaryReadTransport(ctx context.Context, t *debianBinaryNativeTransport) error {
	if ctx == nil || !o.ownsBinaryTransport(t) { return errDebianProofCensus }
	// Existing owner schedules original Job retirement first, independently of
	// any blocked getter/Duplicate/Close/backend release. No replacement loop.
	o.retire(); t.retire()
	t.mu.Lock()
	if t.released { t.mu.Unlock(); return nil }
	if t.preparing || t.reserveStarted && !t.reserveReturned || t.consumeStarted && !t.consumeReturned || t.releaseInFlight { t.mu.Unlock(); return errDebianProofCensus }
	t.releaseInFlight = true; reservation, transport, consumed := t.reservation, t.transport, t.consumeStarted; t.mu.Unlock()
	defer func() { t.mu.Lock(); t.releaseInFlight = false; t.mu.Unlock() }()
	_ = o.releaseUnattemptedBinaryClaim(t.claim)
	if !o.settleBinaryNativeTransport(t) || !o.settle() || !o.releaseUnattemptedBinaryClaim(t.claim) || !o.processCensusSettled() { return errDebianProofCensus }
	var err error
	if consumed {
		if transport == nil { return errDebianProofCensus } // uncertain consumption never uses unallocated release
		err = transport.ReleaseRetiredOriginalBorrows()
	} else if reservation != nil { err = reservation.ReleaseUnallocated() }
	if err != nil { return err }
	t.mu.Lock(); t.released = true; t.mu.Unlock()
	return nil // borrower bookkeeping ONLY; source guards/full closure still live
}
