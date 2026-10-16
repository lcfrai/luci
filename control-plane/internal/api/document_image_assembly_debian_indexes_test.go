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
	"testing"
	"time"

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
