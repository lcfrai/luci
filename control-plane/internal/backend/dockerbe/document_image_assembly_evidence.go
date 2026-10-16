package dockerbe

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type assemblyEvidenceOwner struct {
	mu                               sync.Mutex           // state only, never native I/O
	io                               sync.Mutex           // TryLock, no wait before retirement latch
	guard                            *assemblySourceGuard // sole existing native-handle owner
	objects                          map[string]backend.DocumentImageAssemblyEvidenceObject
	generation                       uint64
	ready, closing, closed, poisoned bool
	readBytes                        int64
	readCalls                        int
	busy                             bool
	loan                             *DocumentImageAssemblyBinaryReadReservation
	loanEpoch                        uint64
}

type assemblyEvidenceToken struct {
	owner      *assemblyEvidenceOwner
	generation uint64
}

var _ backend.DocumentImageAssemblyEvidenceGuard = (*assemblyEvidenceToken)(nil)

// Evidence-only reuse of the existing Windows ordinary-file guard. No Spec,
// Backend client, engine/Job selection, Prepare/Create, allocation or new store.
func AcquireDocumentImageAssemblyEvidence(ctx context.Context, objects []backend.DocumentImageAssemblyEvidenceObject) (backend.DocumentImageAssemblyEvidenceGuard, error) {
	if ctx.Err() != nil {
		return nil, backend.ErrUnsupported
	}
	a, err := PrepareDocumentImageAssemblyEvidenceAcquisition(objects)
	if err != nil {
		return nil, err
	}
	return a.Acquire(ctx)
}

func PrepareDocumentImageAssemblyEvidenceAcquisition(objects []backend.DocumentImageAssemblyEvidenceObject) (*DocumentImageAssemblyEvidenceAcquisition, error) {
	if !assemblySourceGuardSupported() || len(objects) == 0 || len(objects) > backend.DocumentImageAssemblyEvidenceMaxObjects {
		return nil, backend.ErrUnsupported
	}
	owned := make(map[string]backend.DocumentImageAssemblyEvidenceObject, len(objects))
	paths := make(map[string]bool, len(objects))
	var total int64
	for _, object := range objects {
		key := strings.ToLower(object.Path)
		if len(object.Role) == 0 || len(object.Role) > 128 || strings.Trim(object.Role[:1], "abcdefghijklmnopqrstuvwxyz0123456789") != "" || strings.Trim(object.Role, "abcdefghijklmnopqrstuvwxyz0123456789._:-") != "" || owned[object.Role].Role != "" || !filepath.IsAbs(object.Path) || filepath.Clean(object.Path) != object.Path || len(object.Path) > 32760 || strings.ContainsAny(object.Path, "\x00\r\n") || paths[key] || object.Bytes < 0 || object.Bytes > backend.DocumentImageAssemblyEvidenceMaxObjectBytes || object.Bytes > backend.DocumentImageAssemblyEvidenceMaxBytes-total || !fenceHex(object.Digest, 32) || object.Bytes == 0 && object.Digest != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
			return nil, backend.ErrUnsupported
		}
		owned[object.Role], paths[key] = object, true
		total += object.Bytes
	}
	copyObjects := append([]backend.DocumentImageAssemblyEvidenceObject(nil), objects...)
	guard, err := prepareAssemblyOriginalGuard(copyObjects)
	if err != nil {
		return nil, err
	}
	owner := &assemblyEvidenceOwner{guard: guard, objects: owned, generation: 1}
	return &DocumentImageAssemblyEvidenceAcquisition{state: &assemblyOriginalAcquisition{guard: guard, evidence: owner, objects: copyObjects}}, nil
}

func (token *assemblyEvidenceToken) current() bool {
	return token != nil && token.owner != nil && token.owner.generation == token.generation
}

func (token *assemblyEvidenceToken) enter(ctx context.Context, exclusive bool) (*assemblyEvidenceOwner, error) {
	if token == nil || token.owner == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	o := token.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !token.current() {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if !o.ready || o.closing || o.closed || o.poisoned || ctx.Err() != nil || exclusive && o.loan != nil {
		return nil, backend.ErrUnavailable
	}
	if !o.io.TryLock() {
		return nil, backend.ErrUnavailable
	}
	o.busy = true
	return o, nil
}

func (o *assemblyEvidenceOwner) leave() {
	o.mu.Lock()
	o.busy = false
	o.mu.Unlock()
	o.io.Unlock()
}

func (token *assemblyEvidenceToken) CheckCurrent(ctx context.Context) error {
	o, err := token.enter(ctx, false)
	if err != nil {
		return err
	}
	defer o.leave()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = o.guard.CheckCurrent(ctx)
	o.mu.Lock()
	defer o.mu.Unlock()
	if !token.current() {
		return backend.ErrLaunchFenceOwnership
	}
	if err != nil || ctx.Err() != nil {
		o.poisoned = true
		if o.loan != nil {
			o.loan.retiring.Store(true)
		}
		if err != nil {
			return err
		}
		return ctx.Err()
	}
	if o.closing || o.closed || o.poisoned || !o.ready {
		return backend.ErrUnavailable
	}
	return nil
}

func (token *assemblyEvidenceToken) ReadBounded(ctx context.Context, role string, maximum int64) ([]byte, error) {
	o, err := token.enter(ctx, true)
	if err != nil {
		return nil, err
	}
	defer o.leave()
	o.mu.Lock()
	if !token.current() || !o.ready || o.closing || o.closed || o.poisoned || o.loan != nil || ctx.Err() != nil {
		o.mu.Unlock()
		return nil, backend.ErrUnavailable
	}
	object, exists := o.objects[role]
	charge := max(int64(1), object.Bytes)
	if !exists || maximum <= 0 || maximum > backend.DocumentImageAssemblyEvidenceMaxObjectBytes || object.Bytes > maximum || charge > backend.DocumentImageAssemblyEvidenceMaxBytes-o.readBytes || o.readCalls >= backend.DocumentImageAssemblyEvidenceMaxReads {
		o.mu.Unlock()
		return nil, backend.ErrUnsupported
	}
	// Charge before allocation/I/O, including failed reads; no reset or replay.
	o.readBytes += charge
	o.readCalls++
	o.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, err := o.guard.readEvidence(ctx, object, maximum)
	o.mu.Lock()
	defer o.mu.Unlock()
	if err != nil {
		o.poisoned = true
		if o.loan != nil {
			o.loan.retiring.Store(true)
		}
		return nil, err
	}
	if !token.current() || o.closing || o.closed || o.poisoned || !o.ready || ctx.Err() != nil {
		return nil, backend.ErrUnavailable
	}
	return data, nil
}

func (token *assemblyEvidenceToken) Transfer(ctx context.Context) (backend.DocumentImageAssemblyEvidenceGuard, error) {
	o, err := token.enter(ctx, true)
	if err != nil {
		return nil, err
	}
	defer o.leave()
	o.mu.Lock()
	exhausted := o.generation >= 257
	o.mu.Unlock()
	if exhausted {
		return nil, backend.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = o.guard.CheckCurrent(ctx)
	o.mu.Lock()
	defer o.mu.Unlock()
	if err != nil || ctx.Err() != nil {
		o.poisoned = true
		if err == nil {
			err = ctx.Err()
		}
		return nil, err
	}
	if !token.current() || o.closing || o.closed || o.poisoned || !o.ready {
		return nil, backend.ErrUnavailable
	}
	o.generation++
	return &assemblyEvidenceToken{o, o.generation}, nil
}

func (token *assemblyEvidenceToken) Close() error {
	if token == nil || token.owner == nil {
		return backend.ErrLaunchFenceOwnership
	}
	o := token.owner
	o.mu.Lock()
	if !token.current() {
		o.mu.Unlock()
		return backend.ErrLaunchFenceOwnership
	}
	if o.closed {
		o.mu.Unlock()
		return nil
	}
	o.closing = true
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
	err := o.guard.Close()
	o.mu.Lock()
	o.busy = false
	if err == nil {
		o.closed = true
	}
	o.mu.Unlock()
	o.io.Unlock()
	return err
}
