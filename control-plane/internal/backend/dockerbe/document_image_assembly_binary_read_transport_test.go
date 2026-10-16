package dockerbe

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Private METADATA fixture only. Bypasses the production registry-binding
// constructor explicitly; it cannot establish a real original handle, native
// lookup, alias, census or child. No fake NativeReadHandle success is supplied.
func assemblyBinaryTransportMetadataFixture(t *testing.T) (*DocumentImageAssemblyBinaryReadTransport, *DocumentImageAssemblyBinaryReadReservation, *assemblyPackageInputToken, *assemblyEvidenceToken, *assemblyEvidenceToken) {
	t.Helper()
	request, p, i, s := assemblyBinaryReservationFixture()
	r, e := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
	if e != nil {
		t.Fatal(e)
	}
	state, e := consumeAssemblyBinaryReadReservation(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	bindings, e := assemblyBinaryOriginalBindings(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	state.bindings = bindings
	state.ready = true
	return &DocumentImageAssemblyBinaryReadTransport{state}, r, p, i, s
}

func TestAssemblyBinaryTransportOnceConsumedAndAPIOwnershipUnobserved(t *testing.T) {
	token, r, _, i, s := assemblyBinaryTransportMetadataFixture(t)
	if duplicate, e := PrepareDocumentImageAssemblyBinaryReadTransport(context.Background(), r); duplicate != nil || e == nil {
		t.Fatal("consumed reservation replayed")
	}
	if r.ReleaseUnallocated() == nil {
		t.Fatal("consumed reservation released as unallocated")
	}
	if r.State().NativeAliasState != "unobserved-api-owned-after-consumption" || r.State().NativeCensusState != "unobserved-api-owned-after-consumption" {
		t.Fatal("absence invented after consumption", r.State())
	}
	token.Retire()
	if token.ReleaseRetiredOriginalBorrows() != nil {
		t.Fatal("unallocated metadata borrows did not retire")
	}
	state := token.State()
	if !state.OriginalBorrowsReleased || state.APIResourceState != "unobserved-api-owned" {
		t.Fatal("metadata release invented API settlement", state)
	}
	if i.owner.readBytes != 58059296 || i.owner.readCalls != 784 || s.owner.readBytes != 393226 || s.owner.readCalls != 18 {
		t.Fatal("transport reset/refunded lifetime quota")
	}
	if r.ReleaseUnallocated() == nil {
		t.Fatal("released consumed reservation became unallocated again")
	}
}

func TestAssemblyBinaryTransportPartialBindingRetainsExactReservation(t *testing.T) {
	request, p, i, s := assemblyBinaryReservationFixture()
	r, e := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
	if e != nil {
		t.Fatal(e)
	}
	// Dummy SourceGuards have NO real registry. Public constructor must refuse
	// after consuming, retaining exact original scope and quota, not fake success.
	transport, e := PrepareDocumentImageAssemblyBinaryReadTransport(context.Background(), r)
	if transport == nil || e == nil || !r.consumed || r.transport != transport.state || !r.retiring.Load() {
		t.Fatal("partial constructor ownership lost", e)
	}
	if b, e := transport.BorrowOriginalForDuplicate(context.Background(), 0); b != nil || e == nil {
		t.Fatal("missing actual registry became readable original")
	}
	if r.ReleaseUnallocated() == nil {
		t.Fatal("partial consumed borrower escaped ownership")
	}
	if transport.ReleaseRetiredOriginalBorrows() != nil || !transport.State().OriginalBorrowsReleased {
		t.Fatal("exact metadata partial failed release")
	}
	if i.owner.readCalls != 784 {
		t.Fatal("partial binding refunded quota")
	}
}

func TestAssemblyBinaryTransportBorrowIsFixedOnceAndCopySafe(t *testing.T) {
	transport, _, _, _, _ := assemblyBinaryTransportMetadataFixture(t)
	defer func() { transport.Retire(); _ = transport.ReleaseRetiredOriginalBorrows() }()
	copyShell := *transport
	b, e := copyShell.BorrowOriginalForDuplicate(context.Background(), 180)
	if e != nil {
		t.Fatal(e)
	}
	if duplicate, e := transport.BorrowOriginalForDuplicate(context.Background(), 180); duplicate != nil || e == nil {
		t.Fatal("copied shell duplicated original-operation borrower")
	}
	if out, e := transport.BorrowOriginalForDuplicate(context.Background(), 181); out != nil || e == nil {
		t.Fatal("182nd slot admitted")
	}
	if state := transport.State(); state.IssuedOriginalBorrows != 1 || state.ActiveOriginalBorrows != 1 {
		t.Fatal(state)
	}
	copyBorrow := *b
	if b.ReturnFromDuplicate() != nil || copyBorrow.ReturnFromDuplicate() == nil {
		t.Fatal("borrow return was not once-only")
	}
	if _, _, e := copyBorrow.beginLookup(); e == nil {
		t.Fatal("returned borrower restarted native lookup")
	}
	if transport.State().ActiveOriginalBorrows != 0 {
		t.Fatal("declined original operation retained active borrower")
	}
}

func TestAssemblyBinaryTransportLateOriginalLookupAndRetirementStayOwned(t *testing.T) {
	transport, _, _, _, _ := assemblyBinaryTransportMetadataFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b, e := transport.BorrowOriginalForDuplicate(ctx, 0)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = b.beginLookup(); e != nil {
		t.Fatal(e)
	} // private in-flight state, NO native call
	transport.Retire()
	cancel()
	if b.ReturnFromDuplicate() == nil || transport.ReleaseRetiredOriginalBorrows() == nil {
		t.Fatal("in-flight lookup falsely returned/released")
	}
	native := &assemblyBinaryOriginalNative{} // private inert placeholder, no HANDLE proof
	if e = b.finishLookup(ctx, native, errors.New("synthetic late lookup error")); e == nil || transport.state.operations[0].native != native {
		t.Fatal("late original reference lost", e)
	}
	if state := transport.State(); state.NativeLookupsInFlight != 0 || state.ActiveOriginalBorrows != 1 {
		t.Fatal(state)
	}
	if b.ReturnFromDuplicate() != nil || transport.ReleaseRetiredOriginalBorrows() != nil {
		t.Fatal("exact returned metadata borrower failed release")
	}
	if transport.State().APIResourceState != "unobserved-api-owned" {
		t.Fatal("backend metadata invented native alias retirement")
	}
}

func TestAssemblyBinaryTransportOldOwnerRetirementAndPartialReleaseForward(t *testing.T) {
	transport, r, p, i, _ := assemblyBinaryTransportMetadataFixture(t)
	b, e := transport.BorrowOriginalForDuplicate(context.Background(), 173)
	if e != nil {
		t.Fatal(e)
	}
	if p.Close() == nil || !r.retiring.Load() {
		t.Fatal("old owner falsely closed during original borrow")
	}
	if next, e := transport.BorrowOriginalForDuplicate(context.Background(), 0); next != nil || e == nil {
		t.Fatal("old retirement failed forwarding")
	}
	transport.Retire()
	if transport.ReleaseRetiredOriginalBorrows() == nil {
		t.Fatal("unreturned source borrower released")
	}
	if b.ReturnFromDuplicate() != nil {
		t.Fatal("declined operation failed return")
	}
	i.owner.mu.Lock()
	e = transport.ReleaseRetiredOriginalBorrows()
	i.owner.mu.Unlock()
	if e == nil || transport.State().OriginalBorrowsReleased || !r.indexActive || r.packageActive {
		t.Fatal("busy original metadata owner falsely settled", e)
	}
	if transport.ReleaseRetiredOriginalBorrows() != nil || !transport.State().OriginalBorrowsReleased {
		t.Fatal("exact partial remainder failed release")
	}
	if p.owner.ready || !p.owner.retiring {
		t.Fatal("source metadata release revived retired owner")
	}
}

func TestAssemblyBinaryTransportZeroAndMissingCurrentOwnerRefuse(t *testing.T) {
	if x, e := PrepareDocumentImageAssemblyBinaryReadTransport(context.Background(), nil); x != nil || e == nil {
		t.Fatal("nil reservation minted transport")
	}
	zero := &DocumentImageAssemblyBinaryReadTransport{}
	zero.Retire()
	if b, e := zero.BorrowOriginalForDuplicate(context.Background(), 0); b != nil || e == nil {
		t.Fatal("zero transport minted source")
	}
	if zero.ReleaseRetiredOriginalBorrows() == nil {
		t.Fatal("zero transport settled API resources")
	}
	request, p, i, s := assemblyBinaryReservationFixture()
	r, e := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
	if e != nil {
		t.Fatal(e)
	}
	p.owner.objects[0].Digest = request.Indexes[0].Digest
	x, e := PrepareDocumentImageAssemblyBinaryReadTransport(context.Background(), r)
	if x == nil || e != backend.ErrLaunchFenceOwnership {
		t.Fatal("changed original ref accepted", e)
	}
	if !x.state.retiring.Load() || x.ReleaseRetiredOriginalBorrows() != nil {
		t.Fatal("changed ref partial was lost")
	}
}
