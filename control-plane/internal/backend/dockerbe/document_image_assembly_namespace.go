package dockerbe

import (
	"context"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type assemblyNamespaceOwner struct {
	mu sync.Mutex // metadata only; never collector/native IO
	io sync.Mutex // TryLock only, so retirement never waits for native IO
	guard *assemblySourceGuard // the SAME sole original-handle owner
	generation uint64
	ready, closing, closed, poisoned bool
	busy bool
}

type assemblyNamespaceToken struct {
	owner *assemblyNamespaceOwner
	generation uint64
}

var _ backend.DocumentImageAssemblyNamespaceGuard = (*assemblyNamespaceToken)(nil)

// Prepare performs bounded lexical planning only. The future receiver must
// register the actual sealed RegistryLease charge BEFORE calling Acquire.
// An arbitrary directory string is not current private-workspace authority.
func PrepareDocumentImageAssemblyNamespaceAcquisition(directory string) (*DocumentImageAssemblyNamespaceAcquisition, error) {
	if err := validateAssemblyNamespaceDirectory(directory); err != nil {
		return nil, err
	}
	g, err := prepareAssemblyOriginalNamespaceGuard(directory)
	if err != nil {
		return nil, err
	}
	o := &assemblyNamespaceOwner{guard: g, generation: 1}
	return &DocumentImageAssemblyNamespaceAcquisition{state: &assemblyOriginalAcquisition{guard: g, namespace: o, directory: directory}}, nil
}

func (t *assemblyNamespaceToken) current() bool {
	return t != nil && t.owner != nil && t.generation > 0 && t.generation == t.owner.generation
}

func (t *assemblyNamespaceToken) enter(ctx context.Context) (*assemblyNamespaceOwner, error) {
	if t == nil || t.owner == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	o := t.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !t.current() || o.guard == nil || o.guard.originalRegistry() == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if !o.ready || o.closing || o.closed || o.poisoned || ctx.Err() != nil || !o.io.TryLock() {
		return nil, backend.ErrUnavailable
	}
	o.busy = true
	return o, nil
}

func (o *assemblyNamespaceOwner) leave() {
	o.mu.Lock()
	o.busy = false
	o.mu.Unlock()
	o.io.Unlock()
}

func (t *assemblyNamespaceToken) CheckCurrent(ctx context.Context) error {
	o, err := t.enter(ctx)
	if err != nil {
		return err
	}
	defer o.leave()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = o.guard.CheckCurrent(ctx)
	o.mu.Lock()
	defer o.mu.Unlock()
	if err != nil || ctx.Err() != nil {
		o.poisoned = true
		if err != nil { return err }
		return ctx.Err()
	}
	if !t.current() || !o.ready || o.closing || o.closed || o.poisoned || o.guard.originalRegistry().retiring.Load() {
		return backend.ErrUnavailable
	}
	return nil
}

func (t *assemblyNamespaceToken) Transfer(ctx context.Context) (backend.DocumentImageAssemblyNamespaceGuard, error) {
	o, err := t.enter(ctx)
	if err != nil { return nil, err }
	defer o.leave()
	o.mu.Lock()
	exhausted := o.generation >= 257
	o.mu.Unlock()
	if exhausted { return nil, backend.ErrUnavailable }
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = o.guard.CheckCurrent(ctx) // no owner/registry metadata lock across IO
	o.mu.Lock()
	defer o.mu.Unlock()
	if err != nil || ctx.Err() != nil {
		o.poisoned = true
		if err == nil { err = ctx.Err() }
		return nil, err
	}
	if !t.current() || !o.ready || o.closing || o.closed || o.poisoned || o.guard.originalRegistry().retiring.Load() {
		return nil, backend.ErrUnavailable
	}
	o.generation++
	return &assemblyNamespaceToken{o, o.generation}, nil
}

func (t *assemblyNamespaceToken) Close() error {
	if t == nil || t.owner == nil { return backend.ErrLaunchFenceOwnership }
	o := t.owner
	o.mu.Lock()
	if !t.current() || o.guard == nil {
		o.mu.Unlock()
		return backend.ErrLaunchFenceOwnership
	}
	if o.closed { o.mu.Unlock(); return nil }
	o.closing = true
	o.ready = false
	guard := o.guard
	o.mu.Unlock()
	// Latch before taking native IO ownership. Collector-stop is necessary,
	// never proof that a future API spool borrower has returned.
	r := guard.originalRegistry()
	if r == nil { return backend.ErrUnsupported }
	r.retire()
	state, err := r.lease.RetirementState()
	if err != nil { return err }
	if !state.CollectorStopped { return backend.ErrUnavailable }
	if !o.io.TryLock() { return backend.ErrUnavailable }
	defer o.io.Unlock()
	o.mu.Lock()
	if !t.current() { o.mu.Unlock(); return backend.ErrLaunchFenceOwnership }
	if o.closed { o.mu.Unlock(); return nil }
	o.busy = true
	o.mu.Unlock()
	err = guard.Close() // original collector owns and retains partial closes
	o.mu.Lock()
	o.busy = false
	if err == nil { o.closed = true }
	o.mu.Unlock()
	return err
}
