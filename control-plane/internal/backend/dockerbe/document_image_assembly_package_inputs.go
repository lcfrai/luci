package dockerbe

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type assemblyPackageInputOwner struct {
	mu                            sync.Mutex           // short state checks only; NEVER held across native I/O
	io                            sync.Mutex           // TryLock only: retirement never waits behind native checks
	guard                         *assemblySourceGuard // sole existing native original-handle owner
	generation                    uint64
	objects                       []backend.DocumentImageAssemblyEvidenceObject // immutable request copy, never a public path accessor
	loan                          *DocumentImageAssemblyBinaryReadReservation
	loanEpoch                     uint64
	ready, retiring, closed, busy bool
}

type assemblyPackageInputToken struct {
	owner      *assemblyPackageInputOwner
	generation uint64
}

var _ backend.DocumentImageAssemblyPackageInputGuard = (*assemblyPackageInputToken)(nil)

// No hash/read, fake Spec, assembly claim, SDK, process launch, read-handle loan
// or store. A returned nonnil guard plus error owns exact partial originals and
// is retirement-only. Native calls are synchronous; context does not interrupt
// an in-flight Windows syscall. Independent process retirement must not wait on
// this guard. Busy/failed Close is explicitly NOT positive settlement.
func AcquireDocumentImageAssemblyPackageInputs(ctx context.Context, objects []backend.DocumentImageAssemblyEvidenceObject) (backend.DocumentImageAssemblyPackageInputGuard, error) {
	if !assemblySourceGuardSupported() || ctx.Err() != nil || len(objects) != backend.DocumentImageAssemblyPackageInputCount {
		return nil, backend.ErrUnsupported
	}
	paths := map[string]bool{}
	var total int64
	for ordinal, object := range objects {
		key := strings.ToLower(object.Path)
		volume := filepath.VolumeName(object.Path)
		if object.Role != fmt.Sprintf("package.%03d", ordinal) || !filepath.IsAbs(object.Path) || filepath.Clean(object.Path) != object.Path || len(volume) != 2 || volume[1] != ':' || strings.Contains(object.Path[len(volume):], ":") || len(object.Path) > 32760 || strings.ContainsAny(object.Path, "\x00\r\n") || paths[key] || object.Bytes <= 0 || object.Bytes > backend.DocumentImageAssemblyPackageInputMaxObjectBytes || object.Bytes > backend.DocumentImageAssemblyPackageInputBytes-total || !fenceHex(object.Digest, 32) {
			return nil, backend.ErrUnsupported
		}
		paths[key] = true
		total += object.Bytes
	}
	if total != backend.DocumentImageAssemblyPackageInputBytes {
		return nil, backend.ErrUnsupported
	}
	// The caller's slice cannot redefine a later role/ref lookup. Admission
	// remains read-free: this is a copy of validated declarations, not a hash.
	owned := append([]backend.DocumentImageAssemblyEvidenceObject(nil), objects...)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	guard, err := lockAssemblyPackageInputSources(ctx, owned)
	if guard == nil {
		if err == nil {
			err = backend.ErrUnavailable
		}
		return nil, err
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	owner := &assemblyPackageInputOwner{guard: guard, generation: 1, ready: err == nil, objects: owned}
	return &assemblyPackageInputToken{owner, 1}, err
}

func (t *assemblyPackageInputToken) enter(ctx context.Context, exclusive bool) (*assemblyPackageInputOwner, error) {
	if t == nil || t.owner == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	o := t.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if t.generation != o.generation {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if ctx.Err() != nil {
		o.retiring = true
		if o.loan != nil {
			o.loan.retiring.Store(true)
		}
		return nil, ctx.Err()
	}
	if !o.ready || o.retiring || o.closed {
		return nil, backend.ErrUnavailable
	}
	if exclusive && o.loan != nil {
		return nil, backend.ErrUnavailable
	}
	if !o.io.TryLock() {
		return nil, backend.ErrUnavailable
	}
	o.busy = true
	return o, nil
}

func (o *assemblyPackageInputOwner) leave() {
	o.mu.Lock()
	o.busy = false
	o.mu.Unlock()
	o.io.Unlock()
}

func (t *assemblyPackageInputToken) CheckCurrent(ctx context.Context) error {
	o, err := t.enter(ctx, false)
	if err != nil {
		return err
	}
	defer o.leave()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = o.guard.CheckCurrent(ctx)
	o.mu.Lock()
	defer o.mu.Unlock()
	if t.generation != o.generation {
		return backend.ErrLaunchFenceOwnership
	}
	if err != nil || ctx.Err() != nil {
		o.retiring = true
		if o.loan != nil {
			o.loan.retiring.Store(true)
		}
		if err != nil {
			return err
		}
		return ctx.Err()
	}
	if o.retiring || o.closed || !o.ready {
		return backend.ErrUnavailable
	}
	return nil
}

func (t *assemblyPackageInputToken) Transfer(ctx context.Context) (backend.DocumentImageAssemblyPackageInputGuard, error) {
	o, err := t.enter(ctx, true)
	if err != nil {
		return nil, err
	}
	defer o.leave()
	o.mu.Lock()
	exhausted := o.generation > backend.DocumentImageAssemblyPackageInputMaxTransfers
	o.mu.Unlock()
	if exhausted {
		return nil, backend.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = o.guard.CheckCurrent(ctx) // original handles; no payload, no state mutex
	o.mu.Lock()
	defer o.mu.Unlock()
	if t.generation != o.generation {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if err != nil || ctx.Err() != nil {
		o.retiring = true
		if err != nil {
			return nil, err
		}
		return nil, ctx.Err()
	}
	// Close/cancel can latch while CheckCurrent blocks. Never promote a late
	// token after that latch; the unchanged original generation still owns all.
	if o.retiring || o.closed || !o.ready {
		return nil, backend.ErrUnavailable
	}
	o.generation++
	return &assemblyPackageInputToken{o, o.generation}, nil
}

func (t *assemblyPackageInputToken) Close() error {
	if t == nil || t.owner == nil {
		return backend.ErrLaunchFenceOwnership
	}
	o := t.owner
	o.mu.Lock()
	if t.generation != o.generation {
		o.mu.Unlock()
		return backend.ErrLaunchFenceOwnership
	}
	if o.closed {
		o.mu.Unlock()
		return nil
	}
	// Independent of native I/O: no observer/transfer can stay eligible even if
	// the exact native close cannot start or has an unknown in-flight outcome.
	o.retiring = true
	o.ready = false
	if o.loan != nil {
		o.loan.retiring.Store(true)
		o.mu.Unlock()
		return backend.ErrUnavailable
	}
	if !o.io.TryLock() {
		o.mu.Unlock()
		return backend.ErrUnavailable
	}
	o.busy = true
	o.mu.Unlock()
	err := o.guard.Close() // existing exact remaining-handle/partialClose owner
	o.mu.Lock()
	o.busy = false
	if err == nil {
		o.closed = true
	}
	o.mu.Unlock()
	o.io.Unlock()
	return err
}
