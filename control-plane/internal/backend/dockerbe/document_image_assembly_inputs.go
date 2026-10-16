package dockerbe

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const assemblySourceHandleLimit = 1024

type assemblySourceClaim struct {
	owner           *DocumentImageAssembler
	spec            backend.DocumentImageAssemblySpec
	guard           *assemblySourceGuard
	identity        backend.DocumentImageAssemblyIdentity
	createAttempted bool
	closed          bool
}

// This view has no Close or transferable handle. The exact owning assembler
// remains responsible for partial acquisition, failed close and unknown Create.
type assemblyBorrowedSourceGuard struct{ claim *assemblySourceClaim }

func (view assemblyBorrowedSourceGuard) CheckCurrent(ctx context.Context) error {
	if view.claim == nil || view.claim.owner == nil {
		return backend.ErrUnavailable
	}
	b := view.claim.owner
	b.sourceMu.Lock()
	defer b.sourceMu.Unlock()
	if b.sourceClaim != view.claim || view.claim.closed || view.claim.guard == nil {
		return backend.ErrUnavailable
	}
	return view.claim.guard.CheckCurrent(ctx)
}

// Acquire before the resolver and Prepare, not at the allocation edge. These
// bounded opens do NOT hash 2GiB or confer domain acceptance. No daemon call,
// fallback platform guard, automatic replacement or restart adoption occurs.
// A nonnil view returned with error still identifies an owned partial claim.
func (b *DocumentImageAssembler) AcquireDocumentImageAssemblyInputs(ctx context.Context, spec backend.DocumentImageAssemblySpec, objects []backend.DocumentImageAssemblyRetainedObject) (backend.DocumentImageAssemblyInputGuard, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if !b.pinnedSpec(spec) || spec.Identity.EngineID != "" || spec.Identity.InstanceID != "" || len(objects) > assemblySourceHandleLimit || !assemblySourceGuardSupported() || ctx.Err() != nil {
		return nil, backend.ErrUnsupported
	}
	seen := make(map[string]bool, len(objects))
	for _, object := range objects {
		key := strings.ToLower(filepath.Clean(object.Path))
		if !filepath.IsAbs(object.Path) || filepath.Clean(object.Path) != object.Path || len(object.Path) > 32760 || strings.ContainsAny(object.Path, "\x00\r\n") || object.Bytes <= 0 || object.Bytes > 2<<30 || !fenceHex(object.Digest, 32) || seen[key] {
			return nil, backend.ErrUnsupported
		}
		seen[key] = true
	}
	b.sourceMu.Lock()
	defer b.sourceMu.Unlock()
	if b.sourceClaim != nil {
		// Even a positively closed claim cannot be reused for this allocation.
		return nil, backend.ErrUnavailable
	}
	guard, err := lockAssemblySources(ctx, spec, objects...)
	if guard == nil {
		return nil, err
	}
	claim := &assemblySourceClaim{owner: b, spec: spec, guard: guard, identity: spec.Identity}
	b.sourceClaim = claim
	b.guards.Store(claim.identity, guard)
	return assemblyBorrowedSourceGuard{claim}, err
}

// Only a positively unallocated claim may be released without the normal
// exact engine retirement/fence protocol. A failed/lost Create ACK forbids this.
func (b *DocumentImageAssembler) ReleaseUnallocatedDocumentImageAssemblyInputs(ctx context.Context, spec backend.DocumentImageAssemblySpec) error {
	b.sourceMu.Lock()
	defer b.sourceMu.Unlock()
	claim := b.sourceClaim
	if ctx.Err() != nil || claim == nil || claim.spec != spec || claim.createAttempted {
		return backend.ErrLaunchFenceOwnership
	}
	if claim.closed {
		return nil
	}
	if err := claim.guard.Close(); err != nil {
		return err // Failed exact handles remain owned and reachable.
	}
	claim.closed = true
	b.guards.CompareAndDelete(claim.identity, claim.guard)
	return nil
}

func (b *DocumentImageAssembler) checkAssemblyInputs(ctx context.Context, spec backend.DocumentImageAssemblySpec) error {
	b.sourceMu.Lock()
	defer b.sourceMu.Unlock()
	claim := b.sourceClaim
	plain := spec
	plain.Identity.EngineID, plain.Identity.InstanceID = "", ""
	if claim == nil || claim.closed || claim.spec != plain || claim.guard == nil {
		return backend.ErrUnavailable
	}
	return claim.guard.CheckCurrent(ctx)
}

// Single exact ownership transfer after read-only Prepare, before Create. The
// original held handles are preserved; neither paths nor handles are reopened.
func (b *DocumentImageAssembler) bindAssemblyInputs(ctx context.Context, spec backend.DocumentImageAssemblySpec) error {
	b.sourceMu.Lock()
	defer b.sourceMu.Unlock()
	claim := b.sourceClaim
	plain := spec
	plain.Identity.EngineID, plain.Identity.InstanceID = "", ""
	if claim == nil || claim.closed || claim.spec != plain || !backend.ValidateDocumentImageAssemblyIdentity(spec.Identity, true) || claim.identity != plain.Identity && claim.identity != spec.Identity {
		return backend.ErrLaunchFenceOwnership
	}
	if err := claim.guard.CheckCurrent(ctx); err != nil {
		return err
	}
	if claim.identity != spec.Identity {
		b.guards.CompareAndDelete(claim.identity, claim.guard)
		claim.identity = spec.Identity
		b.guards.Store(claim.identity, claim.guard)
	}
	return nil
}
