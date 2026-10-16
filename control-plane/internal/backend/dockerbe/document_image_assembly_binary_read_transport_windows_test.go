//go:build windows

package dockerbe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Deferred real TEMP original-file/metadata fixture. Declared SHA identities
// intentionally are NOT hashes of these sparse zero files. No payload is read,
// authenticated, parsed, inherited or duplicated; no Job/child/census exists.
func assemblyBinaryTransportWindowsFixture(t *testing.T) *DocumentImageAssemblyBinaryReadTransport {
	t.Helper()
	request, _, _, _ := assemblyBinaryReservationFixture()
	objects := assemblyPackageInputFixture(t)
	packages, e := AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
	if packages != nil {
		t.Cleanup(func() { _ = packages.Close() })
	}
	if e != nil {
		t.Fatal(e)
	}
	for i, o := range objects {
		request.Packages[i] = backend.DocumentImageAssemblyBinaryReadSubject{Role: o.Role, Bytes: o.Bytes, Digest: o.Digest}
	}
	directory := t.TempDir()
	makeObjects := func(subjects []backend.DocumentImageAssemblyBinaryReadSubject, prefix string) []backend.DocumentImageAssemblyEvidenceObject {
		var out []backend.DocumentImageAssemblyEvidenceObject
		for i, s := range subjects {
			path := filepath.Join(directory, fmt.Sprintf("%s-%d", prefix, i))
			f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				t.Fatal(e)
			}
			e = f.Truncate(s.Bytes)
			closeErr := f.Close()
			if e != nil || closeErr != nil {
				t.Fatal(e, closeErr)
			}
			out = append(out, backend.DocumentImageAssemblyEvidenceObject{Role: s.Role, Path: path, Bytes: s.Bytes, Digest: s.Digest})
		}
		return out
	}
	indexes, e := AcquireDocumentImageAssemblyEvidence(context.Background(), makeObjects(request.Indexes[:], "index"))
	if indexes != nil {
		t.Cleanup(func() { _ = indexes.Close() })
	}
	if e != nil {
		t.Fatal(e)
	}
	sources, e := AcquireDocumentImageAssemblyEvidence(context.Background(), makeObjects(request.SmallSources[:], "source"))
	if sources != nil {
		t.Cleanup(func() { _ = sources.Close() })
	}
	if e != nil {
		t.Fatal(e)
	}
	r, e := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, packages, indexes, sources)
	if e != nil {
		if r != nil {
			_ = r.ReleaseUnallocated()
		}
		t.Fatal(e)
	}
	transport, e := PrepareDocumentImageAssemblyBinaryReadTransport(context.Background(), r)
	if transport != nil {
		t.Cleanup(func() { transport.Retire(); _ = transport.ReleaseRetiredOriginalBorrows() })
	}
	if e != nil {
		t.Fatal(e)
	}
	return transport
}

func TestAssemblyBinaryTransportActualOriginalGetterPreservesCursor(t *testing.T) {
	transport := assemblyBinaryTransportWindowsFixture(t)
	binding := transport.state.bindings[0]
	var original syscall.Handle
	for _, held := range binding.guard.handles {
		if held.slotID == binding.slotID {
			original = held.handle
			break
		}
	}
	if original == 0 {
		t.Fatal("fixture lost original slot")
	}
	// Parent evidence reads may leave EOF. Getter must preserve, not silently
	// reset, that cursor; future entry's2086 initial-seek contract is separate.
	if position, e := syscall.Seek(original, binding.object.Bytes, 0); e != nil || position != binding.object.Bytes {
		t.Fatal(position, e)
	}
	borrow, e := transport.BorrowOriginalForDuplicate(context.Background(), 0)
	if e != nil {
		t.Fatal(e)
	}
	defer borrow.ReturnFromDuplicate()
	before := len(binding.guard.handles)
	h, e := borrow.NativeReadHandle()
	if e != nil || h != original || len(binding.guard.handles) != before {
		t.Fatal("getter opened/substituted original", h, e)
	}
	if position, e := syscall.Seek(h, 0, 1); e != nil || position != binding.object.Bytes {
		t.Fatal("getter moved source cursor", position, e)
	}
	if duplicate, e := borrow.NativeReadHandle(); duplicate != 0 || e == nil {
		t.Fatal("same borrower exposed native source twice")
	}
	if borrow.ReturnFromDuplicate() != nil {
		t.Fatal("original operation failed return")
	}
	transport.Retire()
	if transport.ReleaseRetiredOriginalBorrows() != nil {
		t.Fatal("metadata original borrow failed release")
	}
	if transport.State().APIResourceState != "unobserved-api-owned" {
		t.Fatal("getter/metadata release invented alias closure")
	}
}

func TestAssemblyBinaryTransportLookupFrameRetirementRetainsBorrow(t *testing.T) {
	transport := assemblyBinaryTransportWindowsFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	borrow, e := transport.BorrowOriginalForDuplicate(ctx, 174)
	if e != nil {
		t.Fatal(e)
	}
	defer borrow.ReturnFromDuplicate()
	// Drive the real lower lookup/finalization edge with its begin event made
	// explicit before scheduling. This models a blocked original IO mutex,
	// not OS syscall cancellation or an original DuplicateHandle completion.
	binding, lookupCtx, e := borrow.beginLookup()
	if e != nil {
		t.Fatal(e)
	}
	binding.guard.mu.Lock()
	done := make(chan error, 1)
	go func() {
		native, err := lookupAssemblyBinaryOriginal(lookupCtx, binding)
		done <- borrow.finishLookup(lookupCtx, native, err)
	}()
	transport.Retire()
	if state := transport.State(); state.NativeLookupsInFlight != 1 || state.ActiveOriginalBorrows != 1 {
		binding.guard.mu.Unlock()
		t.Fatal("lookup did not remain owned", state)
	}
	if borrow.ReturnFromDuplicate() == nil || transport.ReleaseRetiredOriginalBorrows() == nil {
		binding.guard.mu.Unlock()
		t.Fatal("blocked lookup falsely settled")
	}
	cancel()
	binding.guard.mu.Unlock()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled/retired lookup delivered original")
		}
	case <-time.After(time.Second):
		t.Fatal("returning lookup frame did not join")
	}
	if transport.State().ActiveOriginalBorrows != 1 || borrow.ReturnFromDuplicate() != nil {
		t.Fatal("actual returned operation ownership lost")
	}
	if transport.ReleaseRetiredOriginalBorrows() != nil {
		t.Fatal("exact metadata release failed")
	}
}
