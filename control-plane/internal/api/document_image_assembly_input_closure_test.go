package api

import (
	"context"
	"encoding/base64"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestAssemblyDerivedLockRetainsOriginalBytesAndSeparateInventory(t *testing.T) {
	original:=[]byte("{\n  \"state\": \"unresolved\", \"sbomClosed\": false\n}\n")
	source:=assemblyCatalogSource{DependencyLockDigest:assemblyCatalogDigest(original)}
	files:=[]assemblyObservedFile{{Path:"usr/bin/example",Owner:"debian/example",Kind:"regular",Bytes:1,SHA256:strings.Repeat("a",64)}}
	verification,err:=deriveAssemblyVerificationFiles(files);if err!=nil||len(verification)!=1{t.Fatal(err)}
	dependency:=assemblyDependencyClosureV1{Schema:1,Packages:[]assemblyDependencyPackage{{Name:"example",Version:"1",Architecture:"amd64"}},Edges:[]assemblyDependencyEdge{}}
	data,err:=deriveAssemblyClosedLock(original,source,dependency,&assemblyObservedDomains{},verification,[]backend.DocumentImageAssemblyInput{})
	if err!=nil{t.Fatal(err)}
	var lock assemblyClosedLockV1
	if decodeAssemblyCatalog(data,2<<20,&lock)!=nil||lock.State!="closed-reviewed"||lock.Unresolved==nil||len(lock.Unresolved)!=0||lock.Observed.SourceLockSHA256!=source.DependencyLockDigest{t.Fatal("derived lock omitted exact origin")}
	retained,err:=base64.StdEncoding.Strict().DecodeString(lock.SourceMetadata)
	if err!=nil||string(retained)!=string(original)||len(lock.Observed.InstalledInputs)!=1||lock.Observed.InstalledInputs[0].Path!="verification/installed-0000.json"{t.Fatal("source metadata rewritten or inventory disguised as notice")}
	if _,err=deriveAssemblyClosedLock(append(original,' '),source,dependency,&assemblyObservedDomains{},verification,nil);err==nil{t.Fatal("different original accepted")}
}

func TestAssemblyVerificationPagesAreCanonicalBoundedAndDetached(t *testing.T) {
	files:=make([]assemblyObservedFile,4000)
	for i:=range files {files[i]=assemblyObservedFile{Path:strings.Repeat("a",900)+string(rune(0x1000+i)),Owner:"luci",Kind:"regular",Bytes:0,SHA256:assemblyCatalogDigest(nil)}}
	pages,err:=deriveAssemblyVerificationFiles(files);if err!=nil||len(pages)<2{t.Fatal("large inventory not paged",err)}
	count:=0
	for _,page:=range pages {
		if len(page.data)>1<<20||page.input.Bytes!=int64(len(page.data))||page.input.Digest!=assemblyCatalogDigest(page.data){t.Fatal("incorrect page bound/pin")}
		var decoded []assemblyObservedFile
		if decodeAssemblyCatalog(page.data,1<<20,&decoded)!=nil{t.Fatal("noncanonical page")};count+=len(decoded)
	}
	if count!=len(files){t.Fatal("lost input inventory entry")}
	before:=string(pages[0].data);files[0].Path="changed";if string(pages[0].data)!=before{t.Fatal("retained caller backing")}
}

func TestAssemblyAcceptedCandidateCannotBeMintedFromSyntaxOrMissingOwner(t *testing.T) {
	ctx:=context.Background()
	for _,candidate:=range []*assemblyAcceptedInputCandidate{nil,{}, {retiring:true}, {closure:assemblyInputClosureRef{1,strings.Repeat("a",64)}}} {
		if candidate.current(ctx)==nil{t.Fatal("metadata without actual original owners accepted")}
	}
	if candidate,err:=buildDocumentAssemblyClosure(ctx,nil,nil,nil);err==nil||candidate!=nil{t.Fatal("absent domains created acceptance")}
	data:=[]byte("one");candidate:=&assemblyAcceptedInputCandidate{records:[]assemblyCatalogObject{{assemblyCatalogRef{},data}}}
	copy:=candidate.canonicalCopy();copy[0].data[0]='X'
	if string(candidate.records[0].data)!="one"{t.Fatal("canonicalCopy leaked mutable retained bytes")}
}

// Synthetic handles and fake current-record callbacks test binding only. There
// is no real closure resolver, cryptographic proof, Docker or native execution.
type assemblyClosureExecutor struct {
	*assemblyRuntimeExecutor
	prepares, creates int
}

func (f *assemblyClosureExecutor) PrepareDocumentImageAssembly(ctx context.Context, spec backend.DocumentImageAssemblySpec) (backend.PreparedDocumentImageAssembly, error) {
	f.prepares++
	return f.assemblyRuntimeExecutor.PrepareDocumentImageAssembly(ctx, spec)
}
func (f *assemblyClosureExecutor) CreateDocumentImageAssembly(ctx context.Context, prepared backend.PreparedDocumentImageAssembly) (string, error) {
	f.creates++
	return f.assemblyRuntimeExecutor.CreateDocumentImageAssembly(ctx, prepared)
}

func TestAssemblyClosureRefSyntaxIsNotEligibilityAndLegacyWireIsStable(t *testing.T) {
	good := assemblyInputClosureRef{Schema: 1, Digest: strings.Repeat("a", 64)}
	if !validAssemblyInputClosureRef(good) || validAssemblyInputClosureRef(assemblyInputClosureRef{}) || !retainedAssemblyInputClosureRef(assemblyInputClosureRef{}) {
		t.Fatal("new-work and retained syntax policies conflated")
	}
	for _, bad := range []assemblyInputClosureRef{{Schema: 2, Digest: good.Digest}, {Schema: 0, Digest: good.Digest}, {Schema: 1}, {Schema: 1, Digest: strings.Repeat("A", 64)}, {Schema: 1, Digest: strings.Repeat("g", 64)}, {Schema: 1, Digest: good.Digest + "\n"}} {
		if validAssemblyInputClosureRef(bad) || retainedAssemblyInputClosureRef(bad) {
			t.Fatal("noncanonical/unsupported handle accepted", bad)
		}
	}
	legacy := documentImageAssemblyOutcome{}
	identity, _ := json.Marshal(legacy.Identity)
	// Exact pre-extension field order/bytes. The absent handle must not change
	// historical OutcomeDigest or require silently rewriting durable history.
	want := `{"identity":` + string(identity) + `,"stage":"","state":"","oomKilled":false,"readerSettled":false,"cleanupConfirmed":false,"resourcesHeld":false,"watchdogSettled":false,"watchdogTriggered":false,"watchdogRetirementConfirmed":false}`
	data, err := json.Marshal(legacy)
	sum := sha256.Sum256([]byte(want))
	if err != nil || string(data) != want || assemblyOutcomeDigest(legacy) != hex.EncodeToString(sum[:]) {
		t.Fatal("legacy zero wire/digest changed", string(data), err)
	}
	for _, bound := range []bool{false, true} {
		req, h, base, _ := assemblyRuntimeFixture(t)
		if !bound {
			req.InputClosureRef = assemblyInputClosureRef{}
		}
		req.ValidateCurrent = func(context.Context) error { return errors.New("authoritative record unavailable") }
		f := &assemblyClosureExecutor{assemblyRuntimeExecutor: base}
		if out, err := runDocumentImageAssembly(context.Background(), f, h, req); err == nil || f.prepares != 0 || f.creates != 0 || out.ResourcesHeld {
			t.Fatal("syntax/zero handle enabled effects without eligibility", out, err)
		}
	}
}

func TestAssemblyClosureRefAtomicAdmissionAndImmutableReservedReplay(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	l, err := g.readReservationLedger(context.Background())
	if err != nil || l.ImageAssemblies[0].InputClosureRef != req.InputClosureRef || lease.InputClosureRef != req.InputClosureRef {
		t.Fatal("atomic durable admission lost exact evidence handle", l, err)
	}
	p, _ := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	if replay, err := h.Admit(context.Background(), p, req); err != nil || replay != lease {
		t.Fatal("same reserved invocation did not retain immutable intent", replay, err)
	}
	changed := req
	changed.InputClosureRef.Digest = strings.Repeat("8", 64)
	h2, err := g.api.documentImageAssemblySharedHooks(f, changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Admit(context.Background(), p, changed); err == nil {
		t.Fatal("original adapter accepted a different request handle")
	}
	if _, err = h2.Admit(context.Background(), p, changed); err == nil {
		t.Fatal("same inputs/allocation silently rebound to newer evidence")
	}
	if err = g.mutate(context.Background(), func(l *executionLeaseLedger) error {
		l.ImageAssemblies[0].InputClosureRef = assemblyInputClosureRef{} // Retained pre-extension row fixture.
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Admit(context.Background(), p, req); err == nil {
		t.Fatal("legacy reserved row upgraded into new-work authority")
	}
	l, err = g.readReservationLedger(context.Background())
	if err != nil || len(l.ImageAssemblies) != 1 || l.ImageAssemblies[0].InputClosureRef != (assemblyInputClosureRef{}) || l.ImageAssemblies[0].State != "reserved" {
		t.Fatal("legacy history unreadable/pruned/rebound", l, err)
	}
}

func TestAssemblyClosureRefCurrentEligibilityAtEveryDispatchBoundary(t *testing.T) {
	for _, boundary := range []int{1, 2, 3, 4} {
		t.Run(string(rune('0'+boundary)), func(t *testing.T) {
			req, h, base, _ := assemblyRuntimeFixture(t)
			calls := 0
			req.ValidateCurrent = func(context.Context) error {
				calls++
				if calls == boundary {
					return errors.New("record/policy/source binding changed")
				}
				return nil
			}
			f := &assemblyClosureExecutor{assemblyRuntimeExecutor: base}
			out, err := runDocumentImageAssembly(context.Background(), f, h, req)
			if err == nil || base.starts != 0 || calls != boundary || (boundary < 4 && f.creates != 0) {
				t.Fatal("revoked binding dispatched at wrong boundary", out, calls, f.creates, err)
			}
		})
	}
}

func TestAssemblyClosureRefRevocationRecheckedUnderAdmissionLock(t *testing.T) {
	g, req, f, _ := assemblySharedFixture(t)
	if err := g.mutate(context.Background(), func(*executionLeaseLedger) error { return nil }); err != nil {
		t.Fatal(err)
	}
	revoked := false
	req.ValidateCurrent = func(context.Context) error {
		if revoked {
			return errors.New("record revoked while waiting for admission")
		}
		return nil
	}
	h, err := g.api.documentImageAssemblySharedHooks(f, req)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	if err = h.Authorize(context.Background(), req.Actor, p.Spec.Identity); err != nil {
		t.Fatal(err)
	}
	revoked = true
	if _, err = h.Admit(context.Background(), p, req); err == nil {
		t.Fatal("previous proof snapshot survived current locked validation")
	}
	l, err := g.readReservationLedger(context.Background())
	if err != nil || len(l.ImageAssemblies) != 0 || f.starts != 0 {
		t.Fatal("revoked proof allocated or lost readable empty history", l, err)
	}
}

func TestAssemblyClosureRefPublicationAndFinishCannotRelabelEvidence(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	out := documentImageAssemblyOutcome{Identity: lease.Identity, InputClosureRef: lease.InputClosureRef, Stage: "reserved", State: "failed_owned_resources_retired", ReaderSettled: true, WatchdogSettled: true, CleanupConfirmed: true, ResourcesHeld: true}
	wrong := out
	wrong.InputClosureRef.Digest = strings.Repeat("8", 64)
	if err := h.Publish(context.Background(), wrong); err == nil {
		t.Fatal("publication substituted a different evidence record")
	}
	if err := h.Publish(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	store, err := g.assemblyPrivateCapture(lease.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.publish(context.Background(), wrong, false, false); err == nil {
		t.Fatal("private durable receipt rebound to another handle")
	}
	wrong.ResourcesHeld = false
	if err = h.Finish(context.Background(), lease.Identity, wrong); err == nil {
		t.Fatal("mismatched evidence released the lease")
	}
	receipt, err := store.read()
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(store.receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Outcome.InputClosureRef = wrong.InputClosureRef
	altered, _ := json.Marshal(receipt)
	if err = writeExecutionLedger(store.receipt, altered); err != nil {
		t.Fatal(err)
	}
	out.ResourcesHeld = false
	if err = h.Finish(context.Background(), lease.Identity, out); err == nil {
		t.Fatal("mismatched private evidence passed outcome digest comparison")
	}
	l, err := g.readReservationLedger(context.Background())
	if err != nil || l.ImageAssemblies[0].State != "reserved" || l.ImageAssemblies[0].InputClosureRef != lease.InputClosureRef {
		t.Fatal("uncertain publication released or changed intent", l, err)
	}
	if err = writeExecutionLedger(store.receipt, original); err != nil {
		t.Fatal(err)
	}
	if err = h.Finish(context.Background(), lease.Identity, out); err != nil {
		t.Fatal("exact failed/unallocated receipt could not settle", err)
	}
}

func TestAssemblyClosureRefRevokedAndLegacyRecordsAllowOnlyExactRecovery(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "bound_revoked_missing_receipt", true: "legacy_unbound"}[legacy], func(t *testing.T) {
			g, req, f, h := assemblySharedFixture(t)
			lease := assemblySharedAdmit(t, g, req, f, h)
			ref := strings.Repeat("b", 64)
			if err := h.Transition(context.Background(), lease.Identity, "reserved", "creating", ""); err != nil {
				t.Fatal(err)
			}
			if err := h.Transition(context.Background(), lease.Identity, "creating", "created", ref); err != nil {
				t.Fatal(err)
			}
			if legacy {
				if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
					l.ImageAssemblies[0].InputClosureRef = assemblyInputClosureRef{}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			l, err := g.readReservationLedger(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			lease = l.ImageAssemblies[0]
			req.InputClosureRef = lease.InputClosureRef
			proofCalls := 0
			req.ValidateCurrent = func(context.Context) error { proofCalls++; return errors.New("old evidence revoked/unavailable") }
			verified := &assemblySharedFenceExecutor{assemblyRuntimeExecutor: f}
			recovery, err := g.api.documentImageAssemblyRecoveryHooks(verified, req, lease)
			if err != nil {
				t.Fatal(err)
			}
			if err = recovery.Authorize(context.Background(), req.Actor, lease.Identity); err != nil || proofCalls != 0 {
				t.Fatal("old trust eligibility stranded exact cleanup", proofCalls, err)
			}
			g.api.deps.AuthorizeRuntimeInstaller = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
				return management.ExecutionAuthority{}, management.ErrExecutionAuthority
			}
			if err = recovery.Authorize(context.Background(), req.Actor, lease.Identity); err == nil {
				t.Fatal("recovery bypassed current installation cleanup authority")
			}
			g.api.deps.AuthorizeRuntimeInstaller = nil
			counted := &assemblyClosureExecutor{assemblyRuntimeExecutor: f}
			if _, err = runDocumentImageAssembly(context.Background(), counted, recovery, req); err == nil || counted.prepares != 0 || counted.creates != 0 || f.starts != 0 {
				t.Fatal("retirement adapter enabled new-work preparation")
			}
			p, _ := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
			if _, err = recovery.Admit(context.Background(), p, req); err == nil {
				t.Fatal("recovery admitted work")
			}
			if err = recovery.Transition(context.Background(), lease.Identity, "created", "starting", ref); err == nil {
				t.Fatal("recovery published a start intent")
			}
			if writer, err := recovery.CaptureSink(context.Background(), lease.Identity); err == nil || writer != nil {
				t.Fatal("recovery opened a new capture")
			}
			changed := req
			changed.InputClosureRef = assemblyInputClosureRef{Schema: 1, Digest: strings.Repeat("8", 64)}
			if _, err = g.api.documentImageAssemblyRecoveryHooks(verified, changed, lease); err == nil {
				t.Fatal("different historical handle selected for recovery")
			}
			fence := backend.DocumentImageAssemblyFenceRequest{Identity: lease.Identity, ExpectedRef: lease.Ref, IntentID: strings.Repeat("a", 32)}
			out, err := recoverDocumentImageAssembly(context.Background(), verified, recovery, lease, fence, nil)
			if err != nil || out.ResourcesHeld || out.InputClosureRef != lease.InputClosureRef || out.Capture != (backend.DocumentImageAssemblyCapture{}) || proofCalls != 0 || f.starts != 0 {
				t.Fatal("historical ref lost or trust/new-work implied by cleanup", out, proofCalls, err)
			}
			l, err = g.readReservationLedger(context.Background())
			if err != nil || l.ImageAssemblies[0].State != "finished" || l.ImageAssemblies[0].InputClosureRef != lease.InputClosureRef {
				t.Fatal("exact cleanup failed to settle retained intent", l, err)
			}
		})
	}
}

func TestAssemblyClosureRefUnknownRecoveryKeepsCapacityAndBinding(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	if err := h.Transition(context.Background(), lease.Identity, "reserved", "creating", ""); err != nil {
		t.Fatal(err)
	}
	l, err := g.readReservationLedger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	lease = l.ImageAssemblies[0]
	verified := &assemblySharedFenceExecutor{assemblyRuntimeExecutor: f, verifyErr: backend.ErrLaunchFencePending}
	req.ValidateCurrent = nil // No trust resolver needed for exact cleanup.
	recovery, err := g.api.documentImageAssemblyRecoveryHooks(verified, req, lease)
	if err != nil {
		t.Fatal(err)
	}
	fence := backend.DocumentImageAssemblyFenceRequest{Identity: lease.Identity, IntentID: strings.Repeat("a", 32)}
	out, err := recoverDocumentImageAssembly(context.Background(), verified, recovery, lease, fence, nil)
	if err == nil || !out.ResourcesHeld || out.CleanupConfirmed || out.InputClosureRef != lease.InputClosureRef {
		t.Fatal("uncertain ACK released or laundered historical binding", out, err)
	}
	l, err = g.readReservationLedger(context.Background())
	if err != nil || l.ImageAssemblies[0].State != "fencing" || l.ImageAssemblies[0].InputClosureRef != lease.InputClosureRef || l.ImageAssemblyFences[0].State != "intent" {
		t.Fatal("unknown resources or original record charge lost", l, err)
	}
}

func TestAssemblyClosureRefBlockedEligibilityCannotDelayExactRetirement(t *testing.T) {
	req, h, f, finished := assemblyRuntimeFixture(t)
	proofEntered, releaseProof, releaseReader := make(chan struct{}), make(chan struct{}), make(chan struct{})
	proofReleased, readerReleased := false, false
	defer func() {
		if !proofReleased {
			close(releaseProof)
		}
		if !readerReleased {
			close(releaseReader)
		}
	}()
	calls := 0
	req.ValidateCurrent = func(ctx context.Context) error {
		calls++
		if calls == 4 { // Positively created/attached, immediately before Start.
			close(proofEntered)
			<-releaseProof
			return ctx.Err()
		}
		return nil
	}
	f.stream = &assemblyRuntimeStream{release: releaseReader, ignoreClose: true, closed: make(chan struct{})}
	result := make(chan documentImageAssemblyOutcome, 1)
	go func() { out, _ := runDocumentImageAssembly(context.Background(), f, h, req); result <- out }()
	select {
	case <-proofEntered:
	case <-time.After(time.Second):
		t.Fatal("pre-Start evidence hook not reached")
	}
	select {
	case <-f.retired:
	case <-time.After(8 * time.Second):
		t.Fatal("blocked eligibility/reader defeated independent exact stop")
	}
	if f.retiredIdentity.AllocationID != req.Spec.Identity.AllocationID || f.retiredRef != strings.Repeat("b", 64) || f.starts != 0 || *finished != 0 {
		t.Fatal("unknown foreground released capacity or stopped another allocation")
	}
	select {
	case out := <-result:
		t.Fatal("blocked proof/reader falsely settled", out)
	default:
	}
	close(releaseProof)
	proofReleased = true
	close(releaseReader)
	readerReleased = true
	select {
	case out := <-result:
		if out.InputClosureRef != req.InputClosureRef || out.Capture != (backend.DocumentImageAssemblyCapture{}) || !out.CleanupConfirmed || !out.ReaderSettled || !out.WatchdogSettled || out.ResourcesHeld || *finished != 1 {
			t.Fatal("late settlement lost binding or accepted interrupted output", out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("exact owned inert cleanup did not finish after blockers released")
	}
}
