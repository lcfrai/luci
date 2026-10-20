package dockerbe

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// This reservation alone is metadata ONLY. It may be consumed by the sealed
// original-slot transport, but supplies no alias/child/census or captured-run
// completion/adoption capability and cannot enable a producer.
type DocumentImageAssemblyBinaryReadReservation struct {
	mu                                       sync.Mutex // metadata only, never native I/O; owner locks use TryLock
	retiring                                 atomic.Bool
	request                                  backend.DocumentImageAssemblyBinaryReadRequest
	packages                                 *assemblyPackageInputToken
	indexes, sources                         *assemblyEvidenceToken
	selection                                *assemblyEvidenceToken // fixed late selection, never replacement source originals
	selectionActive bool
	packageActive, indexActive, sourceActive bool
	indexCharged, sourceCharged, selectionCharged bool
	complete, released                       bool
	consumed                                 bool
	transport                                *assemblyBinaryTransportState
}

// Reserve exact existing owners/generations, not a fresh budget or copied guard.
// Per-owner quota+exclusive-borrow updates are atomic. A later-stage refusal
// returns a nonnil partial reservation; every charge survives release/error.
// Caller must retire/release this unallocated reservation and separately close
// its original guards. No function in this source can start native effects.
func ReserveDocumentImageAssemblyBinaryReadLoan(ctx context.Context, request backend.DocumentImageAssemblyBinaryReadRequest, packages backend.DocumentImageAssemblyPackageInputGuard, indexes, smallSources backend.DocumentImageAssemblyEvidenceGuard) (*DocumentImageAssemblyBinaryReadReservation, error) {
	return reserveDocumentBinaryReadLoan(ctx,request,packages,indexes,smallSources,nil)
}

// The selection depends on metadata from the SAME held reader/inspector
// originals. Keep their two-object collector intact and borrow a separately
// registered single selection collector; no reopen/replace of source handles.
func ReserveDocumentImageAssemblyBinaryReadLoanWithSelection(ctx context.Context,request backend.DocumentImageAssemblyBinaryReadRequest,packages backend.DocumentImageAssemblyPackageInputGuard,indexes,smallSources,selection backend.DocumentImageAssemblyEvidenceGuard)(*DocumentImageAssemblyBinaryReadReservation,error){
 if selection==nil{return nil,backend.ErrUnsupported}
 return reserveDocumentBinaryReadLoan(ctx,request,packages,indexes,smallSources,selection)
}
func reserveDocumentBinaryReadLoan(ctx context.Context, request backend.DocumentImageAssemblyBinaryReadRequest, packages backend.DocumentImageAssemblyPackageInputGuard, indexes, smallSources,selection backend.DocumentImageAssemblyEvidenceGuard) (*DocumentImageAssemblyBinaryReadReservation, error) {
	if ctx.Err() != nil || validateAssemblyBinaryReadRequest(request) != nil {
		return nil, backend.ErrUnsupported
	}
	p, pok := packages.(*assemblyPackageInputToken)
	i, iok := indexes.(*assemblyEvidenceToken)
	s, sok := smallSources.(*assemblyEvidenceToken)
	if !pok || !iok || !sok || p == nil || p.owner == nil || i == nil || i.owner == nil || s == nil || s.owner == nil || i.owner == s.owner {
		return nil, backend.ErrLaunchFenceOwnership
	}
	r := &DocumentImageAssemblyBinaryReadReservation{request: request, packages: p, indexes: i, sources: s}
	if selection!=nil{token,ok:=selection.(*assemblyEvidenceToken);if !ok||token==nil||token.owner==nil||token.owner==i.owner||token.owner==s.owner{return nil,backend.ErrLaunchFenceOwnership};r.selection=token}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.reservePackage(ctx); err != nil {
		return nil, err
	}
	if err := r.reserveEvidence(ctx, i, request.Indexes[:], true); err != nil {
		r.retiring.Store(true)
		return r, err
	}
	subjects:=request.SmallSources[:];if r.selection!=nil{subjects=subjects[1:]}
	if err := r.reserveEvidence(ctx, s, subjects, false); err != nil {
		r.retiring.Store(true)
		return r, err
	}
	if r.selection!=nil{
	 if err:=r.reserveEvidence(ctx,r.selection,request.SmallSources[:1],false);err!=nil{r.retiring.Store(true);return r,err}
	}
	if ctx.Err() != nil || r.retiring.Load() {
		r.retiring.Store(true)
		return r, backend.ErrUnavailable
	}
	r.complete = true
	return r, nil
}

func validateAssemblyBinaryReadRequest(r backend.DocumentImageAssemblyBinaryReadRequest) error {
	if r.Purpose != backend.DocumentImageAssemblyBinaryReadPurpose || len(r.LoanID) == 0 || len(r.LoanID) > 128 || strings.Trim(r.LoanID, "abcdefghijklmnopqrstuvwxyz0123456789._-") != "" || !fenceHex(r.ScopeDigest, 32) {
		return backend.ErrUnsupported
	}
	total := int64(0)
	for ordinal, subject := range r.Packages {
		if subject.Role != fmt.Sprintf("package.%03d", ordinal) || subject.Bytes <= 0 || subject.Bytes > backend.DocumentImageAssemblyPackageInputMaxObjectBytes || subject.Bytes > backend.DocumentImageAssemblyPackageInputBytes-total || !fenceHex(subject.Digest, 32) {
			return backend.ErrUnsupported
		}
		total += subject.Bytes
	}
	if total != backend.DocumentImageAssemblyPackageInputBytes {
		return backend.ErrUnsupported
	}
	total = 0
	previous := ""
	for _, subject := range r.Indexes {
		if !fenceHex(subject.Digest, 32) || subject.Digest <= previous || subject.Role != "xz-index:"+subject.Digest || subject.Bytes <= 0 || subject.Bytes > backend.DocumentImageAssemblyEvidenceMaxObjectBytes || subject.Bytes > 9489432-total {
			return backend.ErrUnsupported
		}
		total += subject.Bytes
		previous = subject.Digest
	}
	if total != 9489432 {
		return backend.ErrUnsupported
	}
	selection, reader, inspector := r.SmallSources[0], r.SmallSources[1], r.SmallSources[2]
	if selection.Role != "binary-binding-selection" || selection.Bytes <= 0 || selection.Bytes > 256<<10 || !fenceHex(selection.Digest, 32) || reader.Role != "binary-binding-reader-source" || reader.Bytes != 22824 || reader.Digest != "2f52054fc43e926389180ba038ae40650318b940ef3efce5f767492f918f451b" || inspector.Role != "pinned-inspector-source" || inspector.Bytes != 39189 || inspector.Digest != "06b77ce9d7f2d6d06ec5b532cfc965acccd4619322aaf5e17cfa138e6a90acba" {
		return backend.ErrUnsupported
	}
	return nil
}

func assemblyBinaryReadSubjectMatches(s backend.DocumentImageAssemblyBinaryReadSubject, o backend.DocumentImageAssemblyEvidenceObject) bool {
	return s.Role == o.Role && s.Bytes == o.Bytes && s.Digest == o.Digest
}

func (r *DocumentImageAssemblyBinaryReadReservation) reservePackage(ctx context.Context) error {
	o := r.packages.owner
	if !o.mu.TryLock() {
		return backend.ErrUnavailable
	}
	defer o.mu.Unlock()
	if ctx.Err() != nil || r.retiring.Load() || r.packages.generation == 0 || r.packages.generation != o.generation || o.guard == nil {
		return backend.ErrLaunchFenceOwnership
	}
	if !o.ready || o.retiring || o.closed || o.busy || o.loan != nil || o.loanEpoch >= backend.DocumentImageAssemblyBinaryLoanMaxReservations || len(o.objects) != 174 {
		return backend.ErrUnavailable
	}
	for ordinal, subject := range r.request.Packages {
		if !assemblyBinaryReadSubjectMatches(subject, o.objects[ordinal]) {
			return backend.ErrLaunchFenceOwnership
		}
	}
	o.loanEpoch++
	o.loan = r
	r.packageActive = true
	return nil
}

func (r *DocumentImageAssemblyBinaryReadReservation) reserveEvidence(ctx context.Context, token *assemblyEvidenceToken, subjects []backend.DocumentImageAssemblyBinaryReadSubject, index bool) error {
	o := token.owner
	if !o.mu.TryLock() {
		return backend.ErrUnavailable
	}
	defer o.mu.Unlock()
	if ctx.Err() != nil || r.retiring.Load() || token.generation == 0 || token.generation != o.generation || o.guard == nil {
		return backend.ErrLaunchFenceOwnership
	}
	if !o.ready || o.closing || o.closed || o.poisoned || o.busy || o.loan != nil || o.loanEpoch >= backend.DocumentImageAssemblyBinaryLoanMaxReservations {
		return backend.ErrUnavailable
	}
	for _, subject := range subjects {
		object, ok := o.objects[subject.Role]
		if !ok || !assemblyBinaryReadSubjectMatches(subject, object) {
			return backend.ErrLaunchFenceOwnership
		}
	}
	bytes, calls := backend.DocumentImageAssemblyBinarySourceLoanBytes, backend.DocumentImageAssemblyBinarySourceLoanCalls
	if index {
		bytes, calls = backend.DocumentImageAssemblyBinaryIndexLoanBytes, backend.DocumentImageAssemblyBinaryIndexLoanCalls
	}
	if o.readBytes < 0 || o.readBytes > backend.DocumentImageAssemblyEvidenceMaxBytes || o.readCalls < 0 || o.readCalls > backend.DocumentImageAssemblyEvidenceMaxReads || bytes > backend.DocumentImageAssemblyEvidenceMaxBytes-o.readBytes || calls > backend.DocumentImageAssemblyEvidenceMaxReads-o.readCalls {
		return backend.ErrUnsupported
	}
	// Fixed purpose grant, no caller override. Charge before a possible future
	// native loan; never refund even if the next owner reservation fails.
	o.readBytes += bytes
	o.readCalls += calls
	o.loanEpoch++
	o.loan = r
	if index {
		r.indexActive = true
		r.indexCharged = true
	} else if token==r.selection{
		r.selectionActive=true;r.selectionCharged=true
	} else {
		r.sourceActive = true
		r.sourceCharged = true
	}
	return nil
}

// No mutex or I/O before the retirement latch. Existing current guard owners
// independently refuse borrowed read/transfer/positive close until release.
func (r *DocumentImageAssemblyBinaryReadReservation) Retire() error {
	if r == nil {
		return backend.ErrLaunchFenceOwnership
	}
	r.retiring.Store(true)
	return nil
}

func (r *DocumentImageAssemblyBinaryReadReservation) State() backend.DocumentImageAssemblyBinaryReadReservationState {
	if r == nil {
		return backend.DocumentImageAssemblyBinaryReadReservationState{Phase: "absent", NativeCensusState: "not-collected", NativeAliasState: "not-created"}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	phase := "partial-reservation-retirement-only"
	if r.complete && !r.retiring.Load() {
		phase = "reserved-no-native-census"
	}
	if r.released {
		phase = "unallocated-borrows-released"
	}
	census, aliases := "not-collected", "not-created"
	if r.consumed {
		phase = "consumed-original-transport"
		if r.retiring.Load() {
			phase = "consumed-retirement-only"
		}
		if r.released {
			phase = "retired-original-borrows-released-api-unobserved"
		}
		census, aliases = "unobserved-api-owned-after-consumption", "unobserved-api-owned-after-consumption"
	}
	out := backend.DocumentImageAssemblyBinaryReadReservationState{Purpose: r.request.Purpose, LoanID: r.request.LoanID, ScopeDigest: r.request.ScopeDigest, Phase: phase, NativeCensusState: census, NativeAliasState: aliases}
	if r.indexCharged {
		out.IndexBytesCharged = backend.DocumentImageAssemblyBinaryIndexLoanBytes
		out.IndexCallsCharged = backend.DocumentImageAssemblyBinaryIndexLoanCalls
	}
	if r.sourceCharged {
		out.SourceBytesCharged = backend.DocumentImageAssemblyBinarySourceLoanBytes
		out.SourceCallsCharged = backend.DocumentImageAssemblyBinarySourceLoanCalls
	}
	if r.selectionCharged{out.SourceBytesCharged+=backend.DocumentImageAssemblyBinarySourceLoanBytes;out.SourceCallsCharged+=backend.DocumentImageAssemblyBinarySourceLoanCalls}
	for _, active := range []bool{r.packageActive, r.indexActive, r.sourceActive,r.selectionActive} {
		if active {
			out.ActiveGuardBorrows++
		}
	}
	return out
}

// Valid only before sealed original-transport consumption. Consumption disables
// this unallocated release path permanently, before any possible API effect.
// Releasing borrows neither refunds quota nor closes caller-owned originals nor
// proves process/resource settlement. Busy metadata owners remain explicitly held.
func (r *DocumentImageAssemblyBinaryReadReservation) ReleaseUnallocated() error {
	if r == nil {
		return backend.ErrLaunchFenceOwnership
	}
	r.retiring.Store(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.consumed {
		return backend.ErrUnavailable
	}
	return r.releaseOriginalBorrowsLocked()
}

// Metadata-only exact borrower release, with r.mu held. Its caller proves only
// original-operation bookkeeping; it NEVER proves API alias/child settlement.
func (r *DocumentImageAssemblyBinaryReadReservation) releaseOriginalBorrowsLocked() error {
	if r.released {
		return nil
	}
	var first error
	if r.packageActive {
		o := r.packages.owner
		if !o.mu.TryLock() {
			first = backend.ErrUnavailable
		} else {
			if o.loan != r || o.generation != r.packages.generation {
				first = backend.ErrLaunchFenceOwnership
			} else {
				o.loan = nil
				r.packageActive = false
			}
			o.mu.Unlock()
		}
	}
	for _, item := range []struct {
		token  *assemblyEvidenceToken
		active *bool
	}{{r.indexes, &r.indexActive}, {r.sources, &r.sourceActive},{r.selection,&r.selectionActive}} {
		if !*item.active {
			continue
		}
		o := item.token.owner
		if !o.mu.TryLock() {
			if first == nil {
				first = backend.ErrUnavailable
			}
			continue
		}
		if o.loan != r || o.generation != item.token.generation {
			if first == nil {
				first = backend.ErrLaunchFenceOwnership
			}
		} else {
			o.loan = nil
			*item.active = false
		}
		o.mu.Unlock()
	}
	if !r.packageActive && !r.indexActive && !r.sourceActive && !r.selectionActive {
		r.released = true
	}
	return first
}
