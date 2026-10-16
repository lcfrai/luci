package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Metadata and fake owners only. Real compressed index byte objects have not
// been acquired, so no synthetic SHA preimage/positive full capture is claimed.
func assemblyIndexMetadataFixture(t *testing.T) (assemblyDebianSignedIndexBindingsV1, assemblyDebianIndexSelectionV1, assemblyDebianRootPolicyV1, assemblyDebianKeyObservationsV1, assemblyDebianReleaseSignaturesV1) {
	t.Helper()
	policy, keys, signatures, _, now := assemblyDebianSummaryFixture(t)
	source, config := assemblyDebianSourceFixture(t)
	selection, err := normalizeAssemblyDebianIndexSelection(source, config)
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := json.Marshal(selection)
	pr, _ := assemblyDebianRecordRef("official-root-policy", policy)
	kr, _ := assemblyDebianRecordRef("key-observations", keys)
	sr, _ := assemblyDebianRecordRef("release-signatures", signatures)
	value := assemblyDebianSignedIndexBindingsV1{1, "luci-debian-signed-index-bindings-v1", pr, kr, sr, policy.DependencyLockSHA256, policy.AcquisitionManifestSHA256, assemblyDebianIndexCaptureV1{assemblyDebianIndexProfile, assemblyDebianObjectRef{"index-profile", assemblyCatalogDigest(profile), int64(len(profile))}, assemblyDebianObjectRef{"index-acquisition-receipt", assemblyCatalogDigest([]byte("synthetic owned receipt")), 23}, assemblyCatalogRef{1, "producer-raw-manifest", assemblyCatalogDigest([]byte("synthetic manifest")), 18}, now, "original-held-file-size-sha256"}, selection.Objects, selection.Bindings, "not-performed", "not-evaluated", "not-evaluated-server-owned"}
	return value, selection, policy, keys, signatures
}

func TestAssemblyDebianIndexSelectionExactTwelveBindingsEightByteSubjects(t *testing.T) {
	value, selection, policy, keys, signatures := assemblyIndexMetadataFixture(t)
	if err := checkAssemblyDebianIndexBindings(value, policy, keys, signatures, selection); err != nil {
		t.Fatal(err)
	}
	if len(selection.Bindings) != 12 || len(selection.Objects) != 8 {
		t.Fatal("logical/content dedup conflated")
	}
	total := int64(0)
	for _, object := range selection.Objects {
		total += object.Bytes
	}
	if total != assemblyDebianIndexBytes {
		t.Fatal(total)
	}
	byHash := map[string][]assemblyDebianIndexBindingV1{}
	for _, b := range selection.Bindings {
		byHash[b.CompressedObject.SHA256] = append(byHash[b.CompressedObject.SHA256], b)
	}
	shared := 0
	for _, relations := range byHash {
		if len(relations) == 2 {
			shared++
			if relations[0].Era == relations[1].Era || relations[0].Location == relations[1].Location {
				t.Fatal("same bytes promoted snapshot/current role")
			}
		}
	}
	if shared != 4 {
		t.Fatal("expected four shared current/historical byte subjects", shared)
	}
	source, config := assemblyDebianSourceFixture(t)
	config[0] ^= 1
	if _, err := normalizeAssemblyDebianIndexSelection(source, config); err == nil {
		t.Fatal("unpinned configuration accepted")
	}
}

func TestAssemblyDebianIndexMetadataRejectsOriginSnapshotOrdinalAndCoverageClaims(t *testing.T) {
	for _, which := range []string{"foreign-origin", "snapshot-date", "ordinal", "missing-logical", "extra-object", "wrong-size", "full-closure", "decoded", "wrong-signatures"} {
		t.Run(which, func(t *testing.T) {
			original, selection, policy, keys, signatures := assemblyIndexMetadataFixture(t)
			data, _ := json.Marshal(original)
			var value assemblyDebianSignedIndexBindingsV1
			if json.Unmarshal(data, &value) != nil {
				t.Fatal("fixture")
			}
			switch which {
			case "foreign-origin":
				value.Bindings[0].Location.ConfiguredURL = "https://foreign.invalid/index"
			case "snapshot-date":
				for i := range value.Bindings {
					if value.Bindings[i].HistoricalAnchor != nil {
						value.Bindings[i].HistoricalAnchor.SnapshotTimestamp = "20260919T000000Z"
						break
					}
				}
			case "ordinal":
				value.Bindings[0].ReleaseOrdinal = (value.Bindings[0].ReleaseOrdinal + 1) % 5
			case "missing-logical":
				value.Bindings = value.Bindings[:11]
			case "extra-object":
				value.Objects = append(value.Objects, value.Objects[0])
			case "wrong-size":
				value.Objects[0].Bytes++
			case "full-closure":
				value.ServerEligibility = "verified"
			case "decoded":
				value.PayloadDecode = "complete"
			case "wrong-signatures":
				value.ReleaseSignatures.Digest = strings.Repeat("9", 64)
			}
			if checkAssemblyDebianIndexBindings(value, policy, keys, signatures, selection) == nil {
				t.Fatal("unsupported authenticated/coverage claim accepted", which)
			}
		})
	}
}

func TestAssemblyDebianIndexOriginalReadChecksBoundDigestAndExactCount(t *testing.T) {
	data := []byte("synthetic opaque bytes, not an XZ fixture")
	object := assemblyDebianObjectRef{"xz-index", assemblyCatalogDigest(data), int64(len(data))}
	guard := &assemblyDebianFakeGuard{valid: true, data: map[string][]byte{"xz-index:" + object.SHA256: data}}
	read, err := readAssemblyDebianIndex(context.Background(), guard, object)
	if err != nil || !bytes.Equal(read, data) {
		t.Fatal(read, err)
	}
	for _, which := range []string{"short", "extra", "wrong-digest", "oversize", "empty"} {
		t.Run(which, func(t *testing.T) {
			bad := object
			payload := append([]byte{}, data...)
			switch which {
			case "short":
				payload = payload[:len(payload)-1]
			case "extra":
				payload = append(payload, 0)
			case "wrong-digest":
				payload[0] ^= 1
			case "oversize":
				bad.Bytes = assemblyDebianIndexObjectLimit + 1
			case "empty":
				bad.Bytes = 0
			}
			g := &assemblyDebianFakeGuard{valid: true, data: map[string][]byte{"xz-index:" + bad.SHA256: payload}}
			if got, e := readAssemblyDebianIndex(context.Background(), g, bad); e == nil || got != nil {
				t.Fatal("partial/foreign compressed bytes accepted")
			}
			if (which == "oversize" || which == "empty") && g.reads != 0 {
				t.Fatal("bound checked after read")
			}
		})
	}
}

func TestAssemblyDebianIndexConstructorCannotPromoteSyntheticMissingBytes(t *testing.T) {
	r, now := assemblyDebianOwnedFixture(t)
	captured, err := captureDebianSignatureRun(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	first3, err := newAssemblyDebianDomains(context.Background(), r.owner, captured, now)
	if err != nil {
		t.Fatal(err)
	}
	cas, err := r.owner.adoptDebianDomainsAt(context.Background(), first3, now)
	if err != nil {
		t.Fatal(err)
	}
	source, config := assemblyDebianSourceFixture(t)
	selection, err := normalizeAssemblyDebianIndexSelection(source, config)
	if err != nil {
		t.Fatal(err)
	}
	receipt := []byte("synthetic original receipt, not native proof")
	receiptRef := assemblyDebianObjectRef{"index-acquisition-receipt", assemblyCatalogDigest(receipt), int64(len(receipt))}
	g := &assemblyDebianFakeGuard{valid: true, data: map[string][]byte{"index-acquisition-receipt": receipt}}
	for _, object := range selection.Objects {
		g.data["xz-index:"+object.SHA256] = []byte("wrong compressed bytes")
	}
	completion := &assemblyDebianOriginalCompletion{runID: "index-fixture", outerSHA: strings.Repeat("1", 64), outerReceiptSHA: receiptRef.SHA256, started: float64(now + 1), completed: float64(now + 2), originalHandleOwned: true, exitConfirmed: true, capturesSettled: true, watchdogSettled: true, observerSettled: true, publicationSettled: true, handlesSettled: true}
	indexes := &assemblyDebianCapturedIndexes{owner: r.owner, first3: first3, id: completion.runID, guard: g, completion: completion, receipt: receiptRef, selection: selection, phase: "settled"}
	if candidate, e := newAssemblyDebianIndexCandidate(context.Background(), r.owner, first3, indexes, now+2); e == nil || candidate != nil {
		t.Fatal("known SHA preimage was replaced by a synthetic receipt")
	}
	if indexes.phase != "retiring" || !g.valid || first3.guard == nil || first3.inputs == nil {
		t.Fatal("failed capture lost ownership or transferred first3")
	}
	current, e := r.owner.Current(context.Background(), r.actor)
	if e != nil || current.CAS != cas {
		t.Fatal("failed index capture selected a head", current, e)
	}
	if indexes.retire() != nil || first3.retire() != nil {
		t.Fatal("exact fake guard cleanup failed")
	}
}

func TestAssemblyDebianIndexRawRetentionUsesSharedBoundsWithoutSelectableNamespace(t *testing.T) {
	r, _ := assemblyDebianOwnedFixture(t)
	data := bytes.Repeat([]byte{7}, (1<<20)+1)
	ref := assemblyCatalogRef{1, "debian-index-raw", assemblyCatalogDigest(data), int64(len(data))}
	if err := r.owner.writeImmutableAssemblyObject(context.Background(), ref, data, true); err != nil {
		t.Fatal("index raw still capped at first3's1MiB", err)
	}
	if assemblyCatalogKind(ref.Kind) || validAssemblyCatalogRef(ref) {
		t.Fatal("raw index became selectable domain")
	}
	root, err := r.owner.root()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "records", ref.Kind, ref.Digest+".xz")
	read, err := codexRead(filepath.Dir(root), path, ref.Bytes)
	if err != nil || !bytes.Equal(read, data) {
		t.Fatal("no-replace retained bytes mismatch", err)
	}
	if err := boundedAssemblyCatalogHistory(context.Background(), filepath.Join(root, "records"), 0); err != nil {
		t.Fatal("raw namespace rejected by retained history", err)
	}
	if err := r.owner.writeImmutableAssemblyObject(context.Background(), ref, data, true); err != nil {
		t.Fatal("exact immutable readback failed", err)
	}
	if err := r.owner.writeImmutableAssemblyObject(context.Background(), ref, data, false); err == nil {
		t.Fatal("typed metadata writer accepted raw namespace")
	}
	oversize := make([]byte, int(assemblyDebianIndexObjectLimit)+1)
	if err := r.owner.writeImmutableAssemblyObject(context.Background(), assemblyCatalogRef{1, ref.Kind, assemblyCatalogDigest(oversize), int64(len(oversize))}, oversize, true); err == nil {
		t.Fatal("raw index limit ignored")
	}
	if err := os.WriteFile(filepath.Join(root, "records", ref.Kind, "foreign.xz"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if boundedAssemblyCatalogHistory(context.Background(), filepath.Join(root, "records"), 0) == nil {
		t.Fatal("foreign raw inventory ignored")
	}
}

func TestAssemblyDebianIndexHistoryIncludesLargeRetainedByteClaims(t *testing.T) {
	r, _ := assemblyDebianOwnedFixture(t)
	root, err := r.owner.root()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "records", "debian-index-raw")
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// Sparse temporary files test accounting only; never selected/accepted bytes.
	for i := 0; i < 4; i++ {
		f, e := os.Create(filepath.Join(dir, fmt.Sprintf("%064x.xz", i+1)))
		if e != nil {
			t.Fatal(e)
		}
		e = f.Truncate(assemblyDebianIndexObjectLimit)
		closeErr := f.Close()
		if e != nil || closeErr != nil {
			t.Fatal(e, closeErr)
		}
	}
	if boundedAssemblyCatalogHistory(context.Background(), filepath.Join(root, "records"), 1) == nil {
		t.Fatal("large index namespace escaped global64MiB history")
	}
}

func TestAssemblyDebianIndexPermitHasOneArmAndRetirementBlocksFinalAuthority(t *testing.T) {
	r, _ := assemblyDebianOwnedFixture(t)
	old, err := r.owner.Current(context.Background(), r.actor)
	if err != nil {
		t.Fatal(err)
	}
	sd, _ := json.Marshal(old.Source)
	id, _ := json.Marshal(old.Index)
	state := assemblyInputCatalogState{1, old.State.Revision + 1, assemblyCatalogRef{1, "source-descriptor", assemblyCatalogDigest(sd), int64(len(sd))}, old.State.Policy, assemblyCatalogRef{1, "evidence-index", assemblyCatalogDigest(id), int64(len(id))}}
	head, _ := json.Marshal(state)
	g := func() *assemblyDebianFakeGuard {
		return &assemblyDebianFakeGuard{valid: true, data: map[string][]byte{}}
	}
	c := &assemblyDebianIndexCandidate{owner: r.owner, output: g(), inputs: g(), indexes: g(), adopting: true, targetCAS: assemblyCatalogCAS{state.Revision, assemblyCatalogDigest(head)}}
	for _, p := range []*assemblyDebianPublicationPermit{{}, {candidate: &assemblyDebianDomainCandidate{}, indexes: c}} {
		if p.begin(context.Background(), r.owner, c.targetCAS) == nil {
			t.Fatal("permit accepted zero/two arms")
		}
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	r.owner.api.deps.AuthorizeRuntimeInstaller = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		close(entered)
		<-release
		return management.ExecutionAuthority{}, nil
	}
	go func() {
		unlock, e := r.owner.lock(context.Background())
		if e != nil {
			done <- e
			return
		}
		defer unlock()
		_, e = r.owner.publishCatalogHeadLocked(context.Background(), r.actor, old, old.Source, old.State.Policy, old.Index, &assemblyDebianPublicationPermit{indexes: c})
		done <- e
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("final installer hook not reached")
	}
	if err = c.retire(); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("late authority revived retired index permit")
		}
	case <-time.After(time.Second):
		t.Fatal("publication fake not settled")
	}
	current, e := r.owner.readCurrent(context.Background())
	if e != nil || current.CAS != old.CAS {
		t.Fatal("precommit retirement changed head", current, e)
	}
}

func TestAssemblyDebianIndexPermitLostACKAndUnknownCloseRetainOwnership(t *testing.T) {
	for _, retire := range []bool{false, true} {
		t.Run(fmt.Sprint(retire), func(t *testing.T) {
			r, _ := assemblyDebianOwnedFixture(t)
			old, e := r.owner.readCurrent(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			g := func() *assemblyDebianFakeGuard {
				return &assemblyDebianFakeGuard{valid: true, data: map[string][]byte{}}
			}
			indexes := g()
			c := &assemblyDebianIndexCandidate{owner: r.owner, output: g(), inputs: g(), indexes: indexes, adopting: true, targetCAS: old.CAS}
			p := &assemblyDebianPublicationPermit{indexes: c}
			if e = p.begin(context.Background(), r.owner, old.CAS); e != nil {
				t.Fatal(e)
			}
			if retire {
				indexes.failClose = true
				if e = c.retire(); e == nil || c.indexes != indexes || indexes.closed {
					t.Fatal("unknown close lost ownership")
				}
			}
			observed, e := r.owner.reconcilePublication(context.Background(), old.CAS, errors.New("synthetic lost existing head ACK"))
			settled, e := p.finish(observed, e)
			if retire {
				if !errors.Is(e, errAssemblyDebianPublicationUncertain) || settled != (assemblyCatalogCAS{}) || !c.publicationRetired || !c.publicationUnknown {
					t.Fatal("intersecting retirement became success", settled, e)
				}
				indexes.failClose = false
			}
			if !retire && (e != nil || settled != old.CAS) {
				t.Fatal("exact unretired readback refused", settled, e)
			}
			if e = c.retire(); e != nil {
				t.Fatal("exact remaining guard not settled", e)
			}
		})
	}
}

// Fake synchronous guard boundary with the real helper's Transfer/CheckCurrent/
// Close calls. Like the Windows guard, one native-owner mutex is held through
// Transfer IO; a competing Close can wait, but prior-owner latches must not.
// No actual files, handles, payload reads, processes or index SHA preimages.
type assemblyIndexTransferState struct {
	mu                          sync.Mutex
	generation                  int
	closed                      bool
	entered, release            chan struct{}
	cancelCheck                 context.CancelFunc
	returnTokenError, failClose bool
}
type assemblyIndexTransferToken struct {
	state      *assemblyIndexTransferState
	generation int
}

func (g *assemblyIndexTransferToken) CheckCurrent(ctx context.Context) error {
	g.state.mu.Lock()
	defer g.state.mu.Unlock()
	if g.state.cancelCheck != nil && g.generation > 0 {
		cancel := g.state.cancelCheck
		g.state.cancelCheck = nil
		cancel()
	}
	if ctx.Err() != nil || g.state.closed || g.generation != g.state.generation {
		return errAssemblyDebianCapture
	}
	return nil
}
func (g *assemblyIndexTransferToken) ReadBounded(context.Context, string, int64) ([]byte, error) {
	return nil, errors.New("ownership fixture must never read payload")
}
func (g *assemblyIndexTransferToken) Transfer(ctx context.Context) (backend.DocumentImageAssemblyEvidenceGuard, error) {
	g.state.mu.Lock()
	defer g.state.mu.Unlock()
	if ctx.Err() != nil || g.state.closed || g.generation != g.state.generation {
		return nil, errAssemblyDebianCapture
	}
	if g.state.entered != nil {
		close(g.state.entered)
		<-g.state.release
		g.state.entered = nil
	}
	// Actual guard CheckCurrent checks context at end; this fake does too. It
	// never models a successful transfer after its context deadline/cancel.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	g.state.generation++
	next := &assemblyIndexTransferToken{g.state, g.state.generation}
	if g.state.returnTokenError {
		return next, errors.New("synthetic partial token return")
	}
	return next, nil
}
func (g *assemblyIndexTransferToken) Close() error {
	g.state.mu.Lock()
	defer g.state.mu.Unlock()
	if g.generation != g.state.generation {
		return errAssemblyDebianCapture
	}
	if g.state.failClose {
		return errors.New("synthetic exact Close unknown")
	}
	g.state.closed = true
	return nil
}

func assemblyIndexTransferFixture(t *testing.T) (*assemblyDebianIndexCandidate, [3]*assemblyIndexTransferState) {
	t.Helper()
	r, now := assemblyDebianOwnedFixture(t)
	raw, err := captureDebianSignatureRun(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	first3, err := newAssemblyDebianDomains(context.Background(), r.owner, raw, now)
	if err != nil {
		t.Fatal(err)
	}
	cas, err := r.owner.adoptDebianDomainsAt(context.Background(), first3, now)
	if err != nil {
		t.Fatal(err)
	}
	// Settle the old fake tokens explicitly before replacing them for this
	// lower-level handoff fixture. Canonical first3 metadata is not index proof.
	if first3.guard.Close() != nil || first3.inputs.Close() != nil {
		t.Fatal("fixture original token cleanup")
	}
	states := [3]*assemblyIndexTransferState{{}, {}, {}}
	token := func(i int) *assemblyIndexTransferToken { return &assemblyIndexTransferToken{states[i], 0} }
	first3.guard, first3.inputs = token(1), token(2)
	captured := &assemblyDebianCapturedIndexes{owner: r.owner, first3: first3, guard: token(0), phase: "capturing"}
	return &assemblyDebianIndexCandidate{owner: r.owner, first3: first3, capture: captured, expected: cas}, states
}

func TestAssemblyDebianIndexTransferRetirementLatchesWhileNativeTransferIsBlocked(t *testing.T) {
	for _, slot := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(slot), func(t *testing.T) {
			c, states := assemblyIndexTransferFixture(t)
			entered, release := make(chan struct{}), make(chan struct{})
			states[slot].entered, states[slot].release = entered, release
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			transferred, retired, latched := make(chan error, 1), make(chan error, 1), make(chan bool, 1)
			transferDone, retireDone, readerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			retireLaunched, readerLaunched := false, false
			go func() {
				defer close(transferDone)
				transferred <- transferAssemblyDebianIndexOwners(context.Background(), c)
			}()
			defer func() {
				unblock()
				doneSet := []chan struct{}{transferDone}
				if retireLaunched {
					doneSet = append(doneSet, retireDone)
				}
				if readerLaunched {
					doneSet = append(doneSet, readerDone)
				}
				for _, done := range doneSet {
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Error("ownership fixture goroutine not settled")
					}
				}
				_ = c.capture.retire()
				_ = c.first3.retire()
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				unblock()
				t.Fatal("actual helper did not reach blocked Transfer")
			}
			retireLaunched = true
			go func() {
				defer close(retireDone)
				if slot == 0 {
					retired <- c.capture.retire()
				} else {
					retired <- c.first3.retire()
				}
			}()
			readerLaunched = true
			go func() {
				defer close(readerDone)
				until := time.Now().Add(time.Second)
				for time.Now().Before(until) {
					var stopped bool
					if slot == 0 {
						c.capture.mu.Lock()
						stopped = c.capture.phase == "retiring"
						c.capture.mu.Unlock()
					} else {
						c.first3.mu.Lock()
						stopped = c.first3.retiring
						c.first3.mu.Unlock()
					}
					if stopped {
						latched <- true
						return
					}
					time.Sleep(time.Millisecond)
				}
				latched <- false
			}()
			select {
			case yes := <-latched:
				if !yes {
					t.Fatal("retirement did not latch while Transfer blocked")
				}
			case <-time.After(time.Second):
				t.Fatal("prior owner mutex held across native Transfer")
			}
			unblock()
			select {
			case err := <-transferred:
				if err == nil {
					t.Fatal("retirement intersection promoted candidate")
				}
			case <-time.After(time.Second):
				t.Fatal("late Transfer did not settle")
			}
			select {
			case <-retired:
			case <-time.After(time.Second):
				t.Fatal("original retirement did not settle")
			}
			if c.capture.handoff != nil || c.first3.indexHandoff != nil || c.indexes != nil || c.output != nil || c.inputs != nil {
				t.Fatal("candidate installed after prior retirement")
			}
			// The late successful token was registered in its original owner and
			// closed there; other partial tokens stay exactly owned until cleanup.
			states[slot].mu.Lock()
			generation, closed := states[slot].generation, states[slot].closed
			states[slot].mu.Unlock()
			if generation != 1 || !closed {
				t.Fatal("late exact token was lost", generation, closed)
			}
		})
	}
}

func TestAssemblyDebianIndexTransferCancellationBeforePromotionRetainsEveryMovedToken(t *testing.T) {
	c, states := assemblyIndexTransferFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	states[2].cancelCheck = cancel
	if err := transferAssemblyDebianIndexOwners(ctx, c); err == nil {
		t.Fatal("cancelled final current check promoted candidate")
	}
	if c.capture.handoff != nil || c.first3.indexHandoff != nil || c.capture.guard == nil || c.first3.guard == nil || c.first3.inputs == nil {
		t.Fatal("cancellation lost prior owner tokens")
	}
	for _, state := range states {
		state.mu.Lock()
		generation := state.generation
		state.mu.Unlock()
		if generation != 1 {
			t.Fatal("did not drive all three actual Transfer edges")
		}
	}
	if c.capture.retire() != nil || c.first3.retire() != nil {
		t.Fatal("exact transferred tokens not closeable by their retained owners")
	}
}

func TestAssemblyDebianIndexTransferPartialReturnedTokenAndUnknownCloseStayOwned(t *testing.T) {
	c, states := assemblyIndexTransferFixture(t)
	states[0].returnTokenError, states[0].failClose = true, true
	if err := transferAssemblyDebianIndexOwners(context.Background(), c); err == nil {
		t.Fatal("partial token return promoted")
	}
	current, ok := c.capture.guard.(*assemblyIndexTransferToken)
	if !ok || current.generation != 1 || c.capture.phase != "retiring" || c.capture.handoff != nil {
		t.Fatal("returned-on-error token lost exact prior owner")
	}
	if c.capture.retire() == nil {
		t.Fatal("unknown Close claimed settled")
	}
	states[0].failClose = false
	if c.capture.retire() != nil || c.first3.retire() != nil {
		t.Fatal("retained exact tokens not settleable")
	}
}

func TestAssemblyDebianIndexPromotedRetirementFollowsEitherOriginalOwner(t *testing.T) {
	for _, prior := range []string{"captured", "first3"} {
		t.Run(prior, func(t *testing.T) {
			c, states := assemblyIndexTransferFixture(t)
			if err := transferAssemblyDebianIndexOwners(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			if c.capture.handoff != c || c.first3.indexHandoff != c || c.capture.guard != nil || c.first3.guard != nil || c.first3.inputs != nil {
				t.Fatal("ownership not atomically routed")
			}
			var err error
			if prior == "captured" {
				err = c.capture.retire()
			} else {
				err = c.first3.retire()
			}
			if err != nil || !c.retiring {
				t.Fatal("old owner retirement ignored promoted token owner", err)
			}
			for _, state := range states {
				state.mu.Lock()
				closed := state.closed
				state.mu.Unlock()
				if !closed {
					t.Fatal("old retirement skipped promoted native owner")
				}
			}
			if c.capture.retire() != nil || c.first3.retire() != nil {
				t.Fatal("routing exact already-closed ownership failed")
			}
		})
	}
}
