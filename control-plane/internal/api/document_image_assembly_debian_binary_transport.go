package api

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
)

// Immutable metadata only. The existing backend Reserve validates the complete
// finite request. Neither this value nor a populated request authorizes a binary
// purpose, current Product generation, host, command, profile or producer.
type debianBinaryTransportSelection struct {
	request backend.DocumentImageAssemblyBinaryReadRequest
}

func copyDebianBinaryTransportSelection(request backend.DocumentImageAssemblyBinaryReadRequest) debianBinaryTransportSelection {
	return debianBinaryTransportSelection{request: request} // fixed arrays/string values
}

type debianProofBinaryOriginalClaim struct {
	owner *debianProofNativeOwner
	aliases [181]*debianProofOriginalTicket
	job *debianProofOriginalTicket
	pipes [3]*debianProofOriginalTicket
	pi, inherited *debianProofOriginalTicket
}

type debianBinaryNativeTransport struct {
	mu sync.Mutex // metadata only; no backend, guard, native or authority I/O
	owner *debianProofNativeOwner
	selection debianBinaryTransportSelection
	sources [3]*debianProofSourceAdmission
	packages backend.DocumentImageAssemblyPackageInputGuard
	indexes, smallSources backend.DocumentImageAssemblyEvidenceGuard
	claim *debianProofBinaryOriginalClaim
	reservation *dockerbe.DocumentImageAssemblyBinaryReadReservation
	transport *dockerbe.DocumentImageAssemblyBinaryReadTransport
	native *debianBinaryTransportNative
	retiring atomic.Bool
	preparing, prepareReturned, reserveStarted, reserveReturned bool
	consumeStarted, consumeReturned, releaseInFlight, released bool
}

// The reservation is created HERE from exact registered current guard tokens.
// No caller-supplied reservation, raw HANDLE/path or copied ref can substitute.
// There is deliberately no production binary-purpose caller yet: the actual
// Product/source/host/profile gate stays closed and the first3 child is unchanged.
func (o *debianProofNativeOwner) prepareBinaryReadTransport(ctx context.Context, selection debianBinaryTransportSelection, packages *debianProofPackageAdmission, indexes, smallSources *debianProofEvidenceAdmission) (*debianBinaryNativeTransport, error) {
	if ctx == nil || ctx.Err() != nil || o == nil || o.census == nil || !debianProducerPlatformSupported() || packages == nil || indexes == nil || smallSources == nil {
		return nil, errDebianProofCensus
	}
	c := o.census
	t := &debianBinaryNativeTransport{owner: o, selection: selection, sources: [3]*debianProofSourceAdmission{packages.source, indexes.source, smallSources.source}, preparing: true}
	c.mu.Lock()
	if c.owner != o || c.retiring || o.nativeRetired() || !time.Now().Before(o.workLimit()) || indexes.source == smallSources.source {
		c.mu.Unlock(); return nil, errDebianProofCensus
	}
	for _, s := range t.sources {
		if s == nil || s.owner != o || !c.hasSourceLocked(s) || !s.consumed || s.acquiring || s.retiring || s.released || s.binaryTransport != nil {
			c.mu.Unlock(); return nil, errDebianProofCensus
		}
	}
	t.packages, t.indexes, t.smallSources = packages.source.packageGuard, indexes.source.guard, smallSources.source.guard
	if t.packages == nil || t.indexes == nil || t.smallSources == nil {
		c.mu.Unlock(); return nil, errDebianProofCensus
	}
	for _, s := range t.sources { s.binaryTransport = t }
	c.transports = append(c.transports, t)
	c.mu.Unlock()
	defer func() { t.mu.Lock(); t.preparing = false; t.prepareReturned = true; t.mu.Unlock() }()
	// Derive SAME stable registry seals from actual concrete guard tokens outside
	// receiver locks. Backend generation validation remains authoritative too.
	packageLease, err := dockerbe.BorrowDocumentImageAssemblyPackageRegistry(ctx, t.packages)
	if err != nil { t.retire(); return t, err }
	indexLease, err := dockerbe.BorrowDocumentImageAssemblyEvidenceRegistry(ctx, t.indexes)
	if err != nil { t.retire(); return t, err }
	sourceLease, err := dockerbe.BorrowDocumentImageAssemblyEvidenceRegistry(ctx, t.smallSources)
	if err != nil { t.retire(); return t, err }
	for i, lease := range []*dockerbe.DocumentImageAssemblyOriginalRegistryLease{packageLease, indexLease, sourceLease} {
		charge, e := lease.Charge()
		if e != nil || !charge.SameRegistry(t.sources[i].charge) { t.retire(); return t, errDebianProofCensus }
	}
	if t.sources[1].charge.SameRegistry(t.sources[2].charge) { t.retire(); return t, errDebianProofCensus }
	c.mu.Lock()
	valid := !c.retiring && !o.nativeRetired() && !t.retiring.Load() && ctx.Err() == nil && time.Now().Before(o.workLimit())
	for _, s := range t.sources { valid = valid && c.hasSourceLocked(s) && s.binaryTransport == t && !s.retiring && !s.released && !s.acquiring }
	// Each compared snapshot has just passed the concrete backend token getter;
	// its dynamic type is the backend's pointer token, not a caller equality hook.
	valid = valid && t.sources[0].packageGuard == t.packages && t.sources[1].guard == t.indexes && t.sources[2].guard == t.smallSources
	if !valid { c.mu.Unlock(); t.retire(); return t, errDebianProofCensus }
	claim, err := o.reserveBinaryOriginalTransportLocked()
	if err == nil { t.claim = claim }
	c.mu.Unlock()
	if err != nil { t.retire(); return t, err }
	t.mu.Lock(); t.reserveStarted = true; t.mu.Unlock()
	reservation, err := dockerbe.ReserveDocumentImageAssemblyBinaryReadLoan(ctx, t.selection.request, t.packages, t.indexes, t.smallSources)
	err = t.finishReservation(ctx, reservation, err)
	if err != nil { return t, err }
	t.mu.Lock()
	if t.retiring.Load() || ctx.Err() != nil || o.nativeRetired() || !time.Now().Before(o.workLimit()) { t.mu.Unlock(); t.retire(); return t, errDebianProofCensus }
	t.consumeStarted = true
	t.mu.Unlock()
	transport, err := dockerbe.PrepareDocumentImageAssemblyBinaryReadTransport(ctx, reservation)
	t.mu.Lock(); t.transport = transport; t.consumeReturned = true; retired := t.retiring.Load() || ctx.Err() != nil || o.nativeRetired() || !time.Now().Before(o.workLimit()); t.mu.Unlock()
	if err != nil || transport == nil || retired { t.retire(); if err == nil { err = errDebianProofCensus }; return t, err }
	t.mu.Lock(); t.native = newDebianBinaryTransportNative(t); retired = t.retiring.Load() || o.nativeRetired() || ctx.Err() != nil; t.mu.Unlock()
	if retired { t.retire(); return t, errDebianProofCensus }
	return t, nil
}

// Lower actual-return edge also used by narrowly labelled inert regression
// sources. A nonnil partial reservation is retained before error classification.
func (t *debianBinaryNativeTransport) finishReservation(ctx context.Context, reservation *dockerbe.DocumentImageAssemblyBinaryReadReservation, err error) error {
	t.mu.Lock(); t.reservation = reservation; t.reserveReturned = true
	retired := t.retiring.Load() || ctx.Err() != nil || t.owner.nativeRetired() || !time.Now().Before(t.owner.workLimit())
	t.mu.Unlock()
	if err != nil || reservation == nil || retired { t.retire(); if err == nil { return errDebianProofCensus } }
	return err
}

func (t *debianBinaryNativeTransport) retire() {
	if t == nil { return }
	t.retiring.Store(true)
	t.mu.Lock(); reservation, transport := t.reservation, t.transport; t.mu.Unlock()
	if transport != nil { transport.Retire() }
	if reservation != nil { _ = reservation.Retire() }
}

// These issued tickets never reached a native primitive. Derive absence from
// the private call latch, never a timeout/status DTO or the transport's State.
func (o *debianProofNativeOwner) releaseUnattemptedBinaryClaim(claim *debianProofBinaryOriginalClaim) bool {
	if claim == nil { return true }
	c := o.census; if c == nil || claim.owner != o { return false }
	c.mu.Lock(); defer c.mu.Unlock()
	all := true
	for _, ticket := range claim.tickets() {
		if !c.hasNativeLocked(ticket) { all = false; continue }
		if ticket.pending && !ticket.attempted { ticket.pending = false; c.reserved -= len(ticket.slots) }
		if ticket.pending { all = false }
		for _, slot := range ticket.slots { if slot.value != 0 || slot.unknown || slot.closing { all = false } }
	}
	return all
}

func (claim *debianProofBinaryOriginalClaim) tickets() []*debianProofOriginalTicket {
	result := make([]*debianProofOriginalTicket, 0, 187)
	result = append(result, claim.aliases[:]...); result = append(result, claim.job); result = append(result, claim.pipes[:]...)
	return append(result, claim.pi, claim.inherited)
}

// Called ONLY with this owner's census lock; fixed374, no caller count.
func (o *debianProofNativeOwner) reserveBinaryOriginalTransportLocked() (*debianProofBinaryOriginalClaim, error) {
	c := o.census
	if c == nil || c.owner != o || c.retiring || o.nativeRetired() || !time.Now().Before(o.workLimit()) || c.reserved > 1024-374 || len(c.native) > 1024-187 { return nil, errDebianProofCensus }
	claim := &debianProofBinaryOriginalClaim{owner: o}
	issue := func(kind debianProofOriginalKind) *debianProofOriginalTicket { ticket := &debianProofOriginalTicket{owner: o, kind: kind, pending: true, slots: make([]debianProofOriginalSlot, debianProofOriginalSlots(kind))}; c.native = append(c.native, ticket); c.reserved += len(ticket.slots); return ticket }
	for i := range claim.aliases { claim.aliases[i] = issue(debianProofOriginalBinaryAlias) }
	claim.job = issue(debianProofOriginalJob)
	for i := range claim.pipes { claim.pipes[i] = issue(debianProofOriginalPipe) }
	claim.pi = issue(debianProofOriginalPI); claim.inherited = issue(debianProofOriginalInheritedBinary); claim.inherited.childOf = claim.pi
	return claim, nil
}

func (o *debianProofNativeOwner) beginBinaryOriginal(claim *debianProofBinaryOriginalClaim, ticket *debianProofOriginalTicket) error {
	if o == nil || o.census == nil || claim == nil || claim.owner != o { return errDebianProofCensus }
	c := o.census; c.mu.Lock(); defer c.mu.Unlock()
	member := false; for _, issued := range claim.tickets() { if issued == ticket { member = true } }
	if !member || ticket == nil || !c.hasNativeLocked(ticket) || !ticket.pending || ticket.attempted || c.retiring || o.nativeRetired() || !time.Now().Before(o.workLimit()) { return errDebianProofCensus }
	ticket.attempted = true; return nil
}

func (o *debianProofNativeOwner) retireBinaryTransports() {
	if o == nil || o.census == nil { return }
	c := o.census; c.mu.Lock(); transports := append([]*debianBinaryNativeTransport(nil), c.transports...); c.mu.Unlock()
	for _, transport := range transports { transport.retire() }
}
