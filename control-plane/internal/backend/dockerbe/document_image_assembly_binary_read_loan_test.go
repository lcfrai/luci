package dockerbe

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Private fake owner metadata only. No actual file/handle/registry/census/child
// is minted or checked. The nonnil dummy SourceGuard is never asked for payload
// or native identity; no public caller can construct these unexported tokens.
func assemblyBinaryReservationFixture() (backend.DocumentImageAssemblyBinaryReadRequest, *assemblyPackageInputToken, *assemblyEvidenceToken, *assemblyEvidenceToken) {
	r := backend.DocumentImageAssemblyBinaryReadRequest{Purpose: backend.DocumentImageAssemblyBinaryReadPurpose, LoanID: "synthetic-loan", ScopeDigest: strings.Repeat("c", 64)}
	p := &assemblyPackageInputToken{owner: &assemblyPackageInputOwner{guard: &assemblySourceGuard{}, ready: true, generation: 1}, generation: 1}
	base := backend.DocumentImageAssemblyPackageInputBytes / 174
	for ordinal := range r.Packages {
		size := base
		if ordinal == 173 {
			size = backend.DocumentImageAssemblyPackageInputBytes - base*173
		}
		subject := backend.DocumentImageAssemblyBinaryReadSubject{Role: fmt.Sprintf("package.%03d", ordinal), Bytes: size, Digest: strings.Repeat("a", 64)}
		r.Packages[ordinal] = subject
		p.owner.objects = append(p.owner.objects, backend.DocumentImageAssemblyEvidenceObject{Role: subject.Role, Path: fmt.Sprintf("synthetic-original-%d", ordinal), Bytes: size, Digest: subject.Digest})
	}
	r.Indexes = [4]backend.DocumentImageAssemblyBinaryReadSubject{
		{Digest: "82871e4e9839feba2ad17969d5987b2317242fd429572d8321a7f91a9c702c8c", Bytes: 352940},
		{Digest: "87e7e94047fb7fb6f4ceecc7022d4bee55b66031cc2a7666d3196f3e0aabb846", Bytes: 6924},
		{Digest: "9e0b5aabb2465b3d2e7a7fe27f9913846277833f7a2826e7767acccff5b588c5", Bytes: 8790396},
		{Digest: "e99504fa4249fdf10e73fc6352c3a42d47b1a70b29a94190190242e249a6a911", Bytes: 339172},
	}
	for ordinal := range r.Indexes {
		r.Indexes[ordinal].Role = "xz-index:" + r.Indexes[ordinal].Digest
	}
	r.SmallSources = [3]backend.DocumentImageAssemblyBinaryReadSubject{
		{Role: "binary-binding-selection", Bytes: 100, Digest: strings.Repeat("d", 64)},
		{Role: "binary-binding-reader-source", Bytes: 22824, Digest: "2f52054fc43e926389180ba038ae40650318b940ef3efce5f767492f918f451b"},
		{Role: "pinned-inspector-source", Bytes: 39189, Digest: "06b77ce9d7f2d6d06ec5b532cfc965acccd4619322aaf5e17cfa138e6a90acba"},
	}
	evidence := func(subjects []backend.DocumentImageAssemblyBinaryReadSubject) *assemblyEvidenceToken {
		o := &assemblyEvidenceOwner{guard: &assemblySourceGuard{}, ready: true, generation: 1, objects: map[string]backend.DocumentImageAssemblyEvidenceObject{}}
		for _, s := range subjects {
			o.objects[s.Role] = backend.DocumentImageAssemblyEvidenceObject{Role: s.Role, Path: "synthetic-no-path-lookup:" + s.Role, Bytes: s.Bytes, Digest: s.Digest}
		}
		return &assemblyEvidenceToken{o, 1}
	}
	i, s := evidence(r.Indexes[:]), evidence(r.SmallSources[:])
	i.owner.readBytes = 39014888
	i.owner.readCalls = 16
	s.owner.readBytes = 10
	s.owner.readCalls = 2
	return r, p, i, s
}

func TestAssemblyBinaryLoanFixedReservationNoNativeCensusOrRefund(t *testing.T) {
	request, p, i, s := assemblyBinaryReservationFixture()
	r, err := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
	if err != nil {
		if r != nil {
			_ = r.ReleaseUnallocated()
		}
		t.Fatal(err)
	}
	defer r.ReleaseUnallocated()
	state := r.State()
	if state.Phase != "reserved-no-native-census" || state.ActiveGuardBorrows != 3 || state.NativeCensusState != "not-collected" || state.NativeAliasState != "not-created" {
		t.Fatal("metadata reservation invented native readiness", state)
	}
	if i.owner.readBytes != 58059296 || i.owner.readCalls != 784 || s.owner.readBytes != 393226 || s.owner.readCalls != 18 {
		t.Fatal("same lifetime quota was not charged", i.owner.readBytes, i.owner.readCalls, s.owner.readBytes, s.owner.readCalls)
	}
	if r.ReleaseUnallocated() != nil || r.ReleaseUnallocated() != nil {
		t.Fatal("unallocated metadata release failed")
	}
	if i.owner.readBytes != 58059296 || i.owner.readCalls != 784 || s.owner.readBytes != 393226 || s.owner.readCalls != 18 {
		t.Fatal("release refunded quota")
	}
	request.LoanID = "synthetic-second-attempt"
	next, err := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
	if next != nil {
		defer next.ReleaseUnallocated()
	}
	if err == nil || next == nil || next.State().IndexBytesCharged != 0 || i.owner.readCalls != 784 {
		t.Fatal("reprepare reset1024read lifetime", err)
	}
}

func TestAssemblyBinaryLoanByteAndCallBoundaryBeforeAnyNativeEffect(t *testing.T) {
	for _, which := range []string{"exact-boundary", "index-bytes", "index-calls", "source-bytes", "source-calls", "negative-counter"} {
		t.Run(which, func(t *testing.T) {
			request, p, i, s := assemblyBinaryReservationFixture()
			i.owner.readBytes = backend.DocumentImageAssemblyEvidenceMaxBytes - backend.DocumentImageAssemblyBinaryIndexLoanBytes
			i.owner.readCalls = backend.DocumentImageAssemblyEvidenceMaxReads - backend.DocumentImageAssemblyBinaryIndexLoanCalls
			s.owner.readBytes = backend.DocumentImageAssemblyEvidenceMaxBytes - backend.DocumentImageAssemblyBinarySourceLoanBytes
			s.owner.readCalls = backend.DocumentImageAssemblyEvidenceMaxReads - backend.DocumentImageAssemblyBinarySourceLoanCalls
			switch which {
			case "index-bytes":
				i.owner.readBytes++
			case "index-calls":
				i.owner.readCalls++
			case "source-bytes":
				s.owner.readBytes++
			case "source-calls":
				s.owner.readCalls++
			case "negative-counter":
				i.owner.readBytes = -1
			}
			indexBytes, indexCalls := i.owner.readBytes, i.owner.readCalls
			sourceBytes, sourceCalls := s.owner.readBytes, s.owner.readCalls
			r, err := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
			if r != nil {
				defer r.ReleaseUnallocated()
			}
			if which == "exact-boundary" {
				if err != nil || i.owner.readBytes != backend.DocumentImageAssemblyEvidenceMaxBytes || s.owner.readCalls != backend.DocumentImageAssemblyEvidenceMaxReads {
					t.Fatal("exact boundary refused", err)
				}
				return
			}
			if err == nil || r == nil {
				t.Fatal("bad quota did not retain exact partial borrow", err)
			}
			if strings.HasPrefix(which, "source-") {
				if i.owner.readBytes != indexBytes+backend.DocumentImageAssemblyBinaryIndexLoanBytes || i.owner.readCalls != indexCalls+backend.DocumentImageAssemblyBinaryIndexLoanCalls {
					t.Fatal("prior atomic reservation disappeared")
				}
			} else if i.owner.readBytes != indexBytes || i.owner.readCalls != indexCalls {
				t.Fatal("refused quota was changed")
			}
			if s.owner.readBytes != sourceBytes || s.owner.readCalls != sourceCalls {
				t.Fatal("failed source reserve mutated lifetime")
			}
		})
	}
}

func TestAssemblyBinaryLoanExclusiveBorrowRetirementAndIdentityOnlyEntry(t *testing.T) {
	request, p, i, s := assemblyBinaryReservationFixture()
	r, err := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
	if err != nil {
		t.Fatal(err)
	}
	defer r.ReleaseUnallocated()
	beforeBytes, beforeCalls := i.owner.readBytes, i.owner.readCalls
	if data, err := i.ReadBounded(context.Background(), request.Indexes[0].Role, 16<<20); data != nil || err == nil {
		t.Fatal("borrowed source read payload")
	}
	if next, err := i.Transfer(context.Background()); next != nil || err == nil {
		t.Fatal("borrowed source moved")
	}
	if next, err := p.Transfer(context.Background()); next != nil || err == nil {
		t.Fatal("borrowed package moved")
	}
	if i.owner.readBytes != beforeBytes || i.owner.readCalls != beforeCalls {
		t.Fatal("refused borrower read charged again")
	}
	// Exercise only the state admission edge; no dummy native method is called.
	identityOwner, err := i.enter(context.Background(), false)
	if err != nil {
		t.Fatal("identity-only checks disallowed during borrow", err)
	}
	identityOwner.leave()
	if i.Close() == nil || p.Close() == nil || i.owner.closed || p.owner.closed {
		t.Fatal("borrowed originals falsely settled")
	}
	if r.State().Phase != "partial-reservation-retirement-only" {
		t.Fatal("guard close failed to latch reservation")
	}
	if r.ReleaseUnallocated() != nil {
		t.Fatal("exact unallocated borrow release failed")
	}
	if i.owner.ready || p.owner.ready || !i.owner.closing || !p.owner.retiring {
		t.Fatal("release revived retired original owner")
	}
}

func TestAssemblyBinaryLoanPartialReservationReleaseAndIndependentLatch(t *testing.T) {
	request, p, i, s := assemblyBinaryReservationFixture()
	s.owner.readCalls = backend.DocumentImageAssemblyEvidenceMaxReads
	r, err := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, p, i, s)
	if err == nil || r == nil {
		t.Fatal("expected partial reservation")
	}
	defer r.ReleaseUnallocated()
	if r.State().ActiveGuardBorrows != 2 || r.State().SourceCallsCharged != 0 || !r.retiring.Load() {
		t.Fatal("partial borrow/charge lost", r.State())
	}
	// A retained owner state lock cannot delay the reservation's own latch.
	i.owner.mu.Lock()
	r.retiring.Store(false)
	done := make(chan error, 1)
	go func() { done <- r.Retire() }()
	select {
	case err := <-done:
		if err != nil || !r.retiring.Load() {
			i.owner.mu.Unlock()
			t.Fatal("independent latch failed", err)
		}
	case <-time.After(time.Second):
		i.owner.mu.Unlock()
		t.Fatal("latch waited on owner state")
	}
	if r.ReleaseUnallocated() == nil {
		i.owner.mu.Unlock()
		t.Fatal("busy metadata borrow falsely released")
	}
	i.owner.mu.Unlock()
	if r.State().ActiveGuardBorrows != 1 || r.ReleaseUnallocated() != nil || r.State().ActiveGuardBorrows != 0 {
		t.Fatal("exact remaining partial borrow lost", r.State())
	}
	if i.owner.readCalls != 784 {
		t.Fatal("partial release refunded index quota")
	}
}

type assemblyForeignPackageGuard struct{}

func (assemblyForeignPackageGuard) CheckCurrent(context.Context) error {
	panic("foreign guard invoked")
}
func (assemblyForeignPackageGuard) Transfer(context.Context) (backend.DocumentImageAssemblyPackageInputGuard, error) {
	panic("foreign guard invoked")
}
func (assemblyForeignPackageGuard) Close() error { panic("foreign guard invoked") }

func TestAssemblyBinaryLoanRefusesForeignStaleOrMismatchedOwners(t *testing.T) {
	for _, which := range []string{"foreign", "purpose", "role", "scope", "stale-package", "stale-index", "wrong-package-subject", "same-evidence-owner", "busy-index"} {
		t.Run(which, func(t *testing.T) {
			request, p, i, s := assemblyBinaryReservationFixture()
			var packages backend.DocumentImageAssemblyPackageInputGuard = p
			switch which {
			case "foreign":
				packages = assemblyForeignPackageGuard{}
			case "purpose":
				request.Purpose = "ordinary-check"
			case "role":
				request.Packages[0].Role = "script"
			case "scope":
				request.ScopeDigest = "not-scope"
			case "stale-package":
				p.generation++
			case "stale-index":
				i.generation++
			case "wrong-package-subject":
				request.Packages[0].Digest = strings.Repeat("b", 64)
			case "same-evidence-owner":
				s = i
			case "busy-index":
				i.owner.busy = true
			}
			r, err := ReserveDocumentImageAssemblyBinaryReadLoan(context.Background(), request, packages, i, s)
			if r != nil {
				defer r.ReleaseUnallocated()
			}
			if err == nil || i.owner.readBytes != 39014888 || i.owner.readCalls != 16 {
				t.Fatal("foreign/stale state charged or accepted", err)
			}
		})
	}
}
