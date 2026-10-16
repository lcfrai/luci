package dockerbe

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const assemblyBinaryOriginalSlots = 181

type assemblyBinaryOriginalBinding struct {
	guard    *assemblySourceGuard
	registry *assemblyOriginalRegistry
	slotID   uint64
	object   backend.DocumentImageAssemblyEvidenceObject
}
type assemblyBinaryOriginalOperation struct {
	ctx                                   context.Context
	issued, lookupStarted, busy, returned bool
	native                                *assemblyBinaryOriginalNative
}
type assemblyBinaryTransportState struct {
	mu          sync.Mutex // metadata only, never native/guard/registry/owner IO
	retiring    atomic.Bool
	released    atomic.Bool
	reservation *DocumentImageAssemblyBinaryReadReservation
	ready       bool
	bindings    [assemblyBinaryOriginalSlots]assemblyBinaryOriginalBinding
	operations  [assemblyBinaryOriginalSlots]assemblyBinaryOriginalOperation
}

// Opaque shells share one private state, including after a Go value copy. They
// are original-operation borrows ONLY, not alias/native census/child authority.
type DocumentImageAssemblyBinaryReadTransport struct{ state *assemblyBinaryTransportState }
type DocumentImageAssemblyBinaryOriginalBorrow struct {
	state *assemblyBinaryTransportState
	slot  uint16
}
type DocumentImageAssemblyBinaryReadTransportState struct {
	Phase, APIResourceState                                             string
	IssuedOriginalBorrows, ActiveOriginalBorrows, NativeLookupsInFlight int
	OriginalBorrowsReleased                                             bool
}

func consumeAssemblyBinaryReadReservation(ctx context.Context, r *DocumentImageAssemblyBinaryReadReservation) (*assemblyBinaryTransportState, error) {
	if ctx == nil || ctx.Err() != nil || r == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.consumed || r.transport != nil || r.released || !r.complete || r.retiring.Load() || !r.packageActive || !r.indexActive || !r.sourceActive || !r.indexCharged || !r.sourceCharged {
		return nil, backend.ErrUnavailable
	}
	s := &assemblyBinaryTransportState{reservation: r}
	r.consumed = true
	r.transport = s // BEFORE any later binding/lookup/native effect
	return s, nil
}

// Pure binding after once-only consumption. A later error returns a nonnil
// partial transport and retains SAME793 quota/original borrows for retirement.
// No alias is created and no native census/current installer proof is minted.
func PrepareDocumentImageAssemblyBinaryReadTransport(ctx context.Context, r *DocumentImageAssemblyBinaryReadReservation) (*DocumentImageAssemblyBinaryReadTransport, error) {
	s, err := consumeAssemblyBinaryReadReservation(ctx, r)
	if err != nil {
		return nil, err
	}
	t := &DocumentImageAssemblyBinaryReadTransport{state: s}
	bindings, err := assemblyBinaryOriginalBindings(ctx, r)
	if err == nil {
		for i := range bindings {
			if ctx.Err() != nil || r.retiring.Load() {
				err = backend.ErrUnavailable
				break
			}
			if err = bindAssemblyBinaryOriginal(&bindings[i]); err != nil {
				break
			}
		}
	}
	s.mu.Lock()
	s.bindings = bindings // retain even partial metadata, never a new native owner
	if err == nil && (ctx.Err() != nil || s.retiring.Load() || r.retiring.Load()) {
		err = backend.ErrUnavailable
	}
	s.ready = err == nil
	s.mu.Unlock()
	if err != nil {
		t.Retire()
	}
	return t, err
}

func assemblyBinaryOriginalBindings(ctx context.Context, r *DocumentImageAssemblyBinaryReadReservation) ([assemblyBinaryOriginalSlots]assemblyBinaryOriginalBinding, error) {
	var out [assemblyBinaryOriginalSlots]assemblyBinaryOriginalBinding
	if r.packages == nil || r.packages.owner == nil || r.indexes == nil || r.indexes.owner == nil || r.sources == nil || r.sources.owner == nil {
		return out, backend.ErrLaunchFenceOwnership
	}
	p := r.packages.owner
	if !p.mu.TryLock() {
		return out, backend.ErrUnavailable
	}
	if ctx.Err() != nil || r.retiring.Load() || p.loan != r || p.generation != r.packages.generation || !p.ready || p.retiring || p.closed || p.busy || len(p.objects) != 174 {
		p.mu.Unlock()
		return out, backend.ErrUnavailable
	}
	for i, subject := range r.request.Packages {
		if !assemblyBinaryReadSubjectMatches(subject, p.objects[i]) {
			p.mu.Unlock()
			return out, backend.ErrLaunchFenceOwnership
		}
		out[i] = assemblyBinaryOriginalBinding{guard: p.guard, object: p.objects[i]}
	}
	p.mu.Unlock()
	for _, group := range []struct {
		token    *assemblyEvidenceToken
		subjects []backend.DocumentImageAssemblyBinaryReadSubject
		start    int
	}{{r.indexes, r.request.Indexes[:], 174}, {r.sources, r.request.SmallSources[:], 178}} {
		o := group.token.owner
		if !o.mu.TryLock() {
			return out, backend.ErrUnavailable
		}
		if ctx.Err() != nil || r.retiring.Load() || o.loan != r || o.generation != group.token.generation || !o.ready || o.closing || o.closed || o.poisoned || o.busy {
			o.mu.Unlock()
			return out, backend.ErrUnavailable
		}
		for i, subject := range group.subjects {
			object, ok := o.objects[subject.Role]
			if !ok || !assemblyBinaryReadSubjectMatches(subject, object) {
				o.mu.Unlock()
				return out, backend.ErrLaunchFenceOwnership
			}
			out[group.start+i] = assemblyBinaryOriginalBinding{guard: o.guard, object: object}
		}
		o.mu.Unlock()
	}
	return out, nil
}

func (t *DocumentImageAssemblyBinaryReadTransport) BorrowOriginalForDuplicate(ctx context.Context, slot uint16) (*DocumentImageAssemblyBinaryOriginalBorrow, error) {
	if t == nil || t.state == nil || ctx == nil || slot >= assemblyBinaryOriginalSlots {
		return nil, backend.ErrLaunchFenceOwnership
	}
	s := t.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready || s.retiring.Load() || s.reservation.retiring.Load() || s.released.Load() || ctx.Err() != nil || s.operations[slot].issued {
		return nil, backend.ErrUnavailable
	}
	op := &s.operations[slot]
	op.ctx = ctx
	op.issued = true
	return &DocumentImageAssemblyBinaryOriginalBorrow{state: s, slot: slot}, nil
}

func (b *DocumentImageAssemblyBinaryOriginalBorrow) beginLookup() (assemblyBinaryOriginalBinding, context.Context, error) {
	if b == nil || b.state == nil || b.slot >= assemblyBinaryOriginalSlots {
		return assemblyBinaryOriginalBinding{}, nil, backend.ErrLaunchFenceOwnership
	}
	s := b.state
	s.mu.Lock()
	defer s.mu.Unlock()
	op := &s.operations[b.slot]
	if !op.issued || op.returned || op.lookupStarted || !s.ready || s.retiring.Load() || s.reservation.retiring.Load() || s.released.Load() || op.ctx == nil || op.ctx.Err() != nil {
		return assemblyBinaryOriginalBinding{}, nil, backend.ErrUnavailable
	}
	op.lookupStarted = true
	op.busy = true
	return s.bindings[b.slot], op.ctx, nil
}
func (b *DocumentImageAssemblyBinaryOriginalBorrow) finishLookup(ctx context.Context, native *assemblyBinaryOriginalNative, err error) error {
	s := b.state
	s.mu.Lock()
	defer s.mu.Unlock()
	op := &s.operations[b.slot]
	if native != nil {
		op.native = native
	} // every late/partial original remains reachable
	op.busy = false
	if err == nil && (ctx.Err() != nil || s.retiring.Load() || s.reservation.retiring.Load() || op.returned || s.released.Load()) {
		err = backend.ErrUnavailable
	}
	if err != nil {
		s.retiring.Store(true)
		s.reservation.retiring.Store(true)
	}
	return err
}

// This is a trusted original callsite return event, not native success proof.
// The API must retain it until its SAME original DuplicateHandle call returns,
// including late/partial/error return. Backend owns no such API alias call yet.
func (b *DocumentImageAssemblyBinaryOriginalBorrow) ReturnFromDuplicate() error {
	if b == nil || b.state == nil || b.slot >= assemblyBinaryOriginalSlots {
		return backend.ErrLaunchFenceOwnership
	}
	s := b.state
	s.mu.Lock()
	defer s.mu.Unlock()
	op := &s.operations[b.slot]
	if !op.issued || op.returned {
		return backend.ErrLaunchFenceOwnership
	}
	if op.busy {
		return backend.ErrUnavailable
	}
	op.returned = true
	return nil
}

func (t *DocumentImageAssemblyBinaryReadTransport) Retire() {
	if t != nil && t.state != nil {
		t.state.retiring.Store(true)
		t.state.reservation.retiring.Store(true)
	}
}
func (t *DocumentImageAssemblyBinaryReadTransport) State() DocumentImageAssemblyBinaryReadTransportState {
	out := DocumentImageAssemblyBinaryReadTransportState{Phase: "absent", APIResourceState: "unobserved-api-owned"}
	if t == nil || t.state == nil {
		return out
	}
	s := t.state
	s.mu.Lock()
	defer s.mu.Unlock()
	out.Phase = "partial-original-binding-retirement-only"
	if s.ready {
		out.Phase = "original-slots-bound-api-unobserved"
	}
	if s.retiring.Load() || s.reservation.retiring.Load() {
		out.Phase = "retirement-only-api-unobserved"
	}
	for _, op := range s.operations {
		if op.issued {
			out.IssuedOriginalBorrows++
		}
		if op.issued && !op.returned {
			out.ActiveOriginalBorrows++
		}
		if op.busy {
			out.NativeLookupsInFlight++
		}
	}
	out.OriginalBorrowsReleased = s.released.Load()
	if out.OriginalBorrowsReleased {
		out.Phase = "original-borrows-released-api-unobserved"
	}
	return out
}

// Exact metadata borrower release ONLY. Real later private API integration
// must FIRST observe its own original alias/child/Job/reader/observer settlement.
// No success/callback/count argument can substitute for that distinct gate.
func (t *DocumentImageAssemblyBinaryReadTransport) ReleaseRetiredOriginalBorrows() error {
	if t == nil || t.state == nil {
		return backend.ErrLaunchFenceOwnership
	}
	s := t.state
	r := s.reservation
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.consumed || r.transport != s || !s.retiring.Load() {
		return backend.ErrUnavailable
	}
	s.mu.Lock()
	for _, op := range s.operations {
		if op.issued && !op.returned || op.busy {
			s.mu.Unlock()
			return backend.ErrUnavailable
		}
	}
	s.mu.Unlock()
	err := r.releaseOriginalBorrowsLocked()
	if r.released {
		s.released.Store(true)
	}
	return err
}
