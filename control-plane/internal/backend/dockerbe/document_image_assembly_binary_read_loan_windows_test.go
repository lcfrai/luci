//go:build windows

package dockerbe

import (
	"context"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Deferred actual Windows TEMP original-file fixture, not real packages or
// producer/census proof. The existing fixture creates sparse/truncated files;
// neither acquisition nor this reservation reads/hashes their payload.
func TestAssemblyBinaryLoanPackageDeclarationsCopiedBeforeBorrow(t *testing.T) {
	objects := assemblyPackageInputFixture(t)
	guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
	if guard != nil {
		defer guard.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	request, _, indexes, sources := assemblyBinaryReservationFixture()
	for ordinal, object := range objects {
		request.Packages[ordinal] = backend.DocumentImageAssemblyBinaryReadSubject{Role: object.Role, Bytes: object.Bytes, Digest: object.Digest}
	}
	// Alter the caller's original slice after acquisition. It must not redefine
	// the collector's later exact role/reference lookup or native ownership.
	objects[0].Digest = strings.Repeat("b", 64)
	objects[0].Path = "not-the-original"
	reservation, err := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, guard, indexes, sources)
	if reservation != nil {
		defer reservation.ReleaseUnallocated()
	}
	if err != nil {
		t.Fatal("caller slice changed the retained declaration", err)
	}
	if reservation.State().ActiveGuardBorrows != 3 {
		t.Fatal("exact original borrow missing")
	}
	if err := guard.CheckCurrent(context.Background()); err != nil {
		t.Fatal("identity-only original check during borrow", err)
	}
	if err := reservation.ReleaseUnallocated(); err != nil {
		t.Fatal(err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal("released original ownership failed close", err)
	}
}
