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
	mu                               sync.Mutex
	guard                            *assemblySourceGuard // sole existing native-handle owner
	objects                          map[string]backend.DocumentImageAssemblyEvidenceObject
	generation                       uint64
	ready, closing, closed, poisoned bool
	readBytes                        int64
}

type assemblyEvidenceToken struct {
	owner      *assemblyEvidenceOwner
	generation uint64
}

var _ backend.DocumentImageAssemblyEvidenceGuard = (*assemblyEvidenceToken)(nil)

// Evidence-only reuse of the existing Windows ordinary-file guard. No Spec,
// Backend client, engine/Job selection, Prepare/Create, allocation or new store.
func AcquireDocumentImageAssemblyEvidence(ctx context.Context, objects []backend.DocumentImageAssemblyEvidenceObject) (backend.DocumentImageAssemblyEvidenceGuard, error) {
	if !assemblySourceGuardSupported() || len(objects) == 0 || len(objects) > backend.DocumentImageAssemblyEvidenceMaxObjects || ctx.Err() != nil {
		return nil, backend.ErrUnsupported
	}
	owned := make(map[string]backend.DocumentImageAssemblyEvidenceObject, len(objects))
	paths := make(map[string]bool, len(objects))
	var total int64
	for _, object := range objects {
		key := strings.ToLower(object.Path)
		if len(object.Role) == 0 || len(object.Role) > 128 || strings.Trim(object.Role[:1], "abcdefghijklmnopqrstuvwxyz0123456789") != "" || strings.Trim(object.Role, "abcdefghijklmnopqrstuvwxyz0123456789._:-") != "" || owned[object.Role].Role != "" || !filepath.IsAbs(object.Path) || filepath.Clean(object.Path) != object.Path || len(object.Path) > 32760 || strings.ContainsAny(object.Path, "\x00\r\n") || paths[key] || object.Bytes <= 0 || object.Bytes > backend.DocumentImageAssemblyEvidenceMaxObjectBytes || object.Bytes > backend.DocumentImageAssemblyEvidenceMaxBytes-total || !fenceHex(object.Digest, 32) {
			return nil, backend.ErrUnsupported
		}
		owned[object.Role], paths[key] = object, true
		total += object.Bytes
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	guard, err := lockAssemblyEvidenceSources(ctx, objects)
	if guard == nil {
		return nil, err
	}
	owner := &assemblyEvidenceOwner{guard: guard, objects: owned, generation: 1, ready: err == nil}
	return &assemblyEvidenceToken{owner, 1}, err
}

func (token *assemblyEvidenceToken) current() bool {
	return token != nil && token.owner != nil && token.owner.generation == token.generation
}

func (token *assemblyEvidenceToken) CheckCurrent(ctx context.Context) error {
	if token == nil || token.owner == nil {
		return backend.ErrLaunchFenceOwnership
	}
	o := token.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !token.current() {
		return backend.ErrLaunchFenceOwnership
	}
	if !o.ready || o.closing || o.closed || o.poisoned {
		return backend.ErrUnavailable
	}
	return o.guard.CheckCurrent(ctx)
}

func (token *assemblyEvidenceToken) ReadBounded(ctx context.Context, role string, maximum int64) ([]byte, error) {
	if token == nil || token.owner == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	o := token.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !token.current() {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if !o.ready || o.closing || o.closed || o.poisoned || ctx.Err() != nil {
		return nil, backend.ErrUnavailable
	}
	object, exists := o.objects[role]
	if !exists || maximum <= 0 || maximum > backend.DocumentImageAssemblyEvidenceMaxObjectBytes || object.Bytes > maximum || object.Bytes > backend.DocumentImageAssemblyEvidenceMaxBytes-o.readBytes {
		return nil, backend.ErrUnsupported
	}
	// Charge before allocation/I/O, including failed reads; no reset or replay.
	o.readBytes += object.Bytes
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, err := o.guard.readEvidence(ctx, object, maximum)
	if err != nil {
		o.poisoned = true
		return nil, err
	}
	return data, nil
}

func (token *assemblyEvidenceToken) Transfer(ctx context.Context) (backend.DocumentImageAssemblyEvidenceGuard, error) {
	if token == nil || token.owner == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	o := token.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !token.current() {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if !o.ready || o.closing || o.closed || o.poisoned || o.generation >= 257 || ctx.Err() != nil {
		return nil, backend.ErrUnavailable
	}
	if err := o.guard.CheckCurrent(ctx); err != nil {
		o.poisoned = true
		return nil, err
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
	defer o.mu.Unlock()
	if !token.current() {
		return backend.ErrLaunchFenceOwnership
	}
	if o.closed {
		return nil
	}
	o.closing = true
	o.ready = false
	if err := o.guard.Close(); err != nil {
		return err
	}
	o.closed = true
	return nil
}
