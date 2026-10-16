package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Selection is configuration attribution only, never authenticated evidence.
// It is derived from the already frozen policy/config, not a caller URL list.
func normalizeAssemblyDebianIndexSelection(source, config []byte) (assemblyDebianIndexSelectionV1, error) {
	var out assemblyDebianIndexSelectionV1
	policy, err := normalizeAssemblyDebianPolicy(source, config, "index-selection-v1")
	if err != nil {
		return out, err
	}
	var input struct {
		Releases []struct {
			SHA        string `json:"sha256"`
			Candidates []struct {
				ID      string                   `json:"id"`
				Era     string                   `json:"era"`
				Indexes []assemblyDebianRawIndex `json:"indexCandidates"`
			} `json:"candidates"`
		} `json:"releases"`
	}
	if json.Unmarshal(config, &input) != nil || len(input.Releases) != 5 {
		return out, errAssemblyDebianEvidence
	}
	selections := map[string]assemblyDebianReleaseSelectionV1{}
	for _, s := range policy.Releases {
		selections[s.CandidateID] = s
	}
	objects, seen := map[string]assemblyDebianObjectRef{}, map[string]bool{}
	out = assemblyDebianIndexSelectionV1{Schema: 1, Profile: assemblyDebianIndexProfile, SourcePolicy: policy.SourcePolicy, ProducerConfig: policy.ProducerConfig}
	for ordinal, release := range input.Releases {
		for _, c := range release.Candidates {
			s, ok := selections[c.ID]
			if !ok || seen[c.ID] || s.SignedRelease.SHA256 != release.SHA || len(c.Indexes) != 2 {
				return assemblyDebianIndexSelectionV1{}, errAssemblyDebianEvidence
			}
			seen[c.ID] = true
			era := "current"
			if s.HistoricalTimestamp != nil {
				era = "historical-base-anchor"
			}
			if c.Era != era {
				return assemblyDebianIndexSelectionV1{}, errAssemblyDebianEvidence
			}
			for _, index := range c.Indexes {
				var advertised *assemblyDebianSignedIndexClaimV1
				for i := range s.SignedIndexes {
					if s.SignedIndexes[i].Path == index.Path {
						advertised = &s.SignedIndexes[i]
					}
				}
				if advertised == nil || index.Read || index.SignatureVerified || index.SHA256 != advertised.SHA256 || index.Bytes != advertised.Bytes {
					return assemblyDebianIndexSelectionV1{}, errAssemblyDebianEvidence
				}
				object := assemblyDebianObjectRef{"xz-index", index.SHA256, index.Bytes}
				if !checkAssemblyDebianObject(object, "xz-index", assemblyDebianIndexObjectLimit) {
					return assemblyDebianIndexSelectionV1{}, errAssemblyDebianEvidence
				}
				if old, exists := objects[object.SHA256]; exists && old != object {
					return assemblyDebianIndexSelectionV1{}, errAssemblyDebianEvidence
				}
				objects[object.SHA256] = object
				binding := assemblyDebianIndexBindingV1{CandidateID: c.ID, Era: era, ReleaseOrdinal: ordinal, IndexPath: index.Path, CompressedObject: object, Location: assemblyDebianIndexLocationV1{index.URL, index.ByHashURL}}
				if era == "historical-base-anchor" {
					anchor := policy.HistoricalAnchor
					binding.HistoricalAnchor = &anchor
				}
				out.Bindings = append(out.Bindings, binding)
			}
		}
	}
	for _, object := range objects {
		out.Objects = append(out.Objects, object)
	}
	sort.Slice(out.Objects, func(i, j int) bool { return out.Objects[i].SHA256 < out.Objects[j].SHA256 })
	sort.Slice(out.Bindings, func(i, j int) bool {
		return out.Bindings[i].CandidateID+"/"+out.Bindings[i].IndexPath < out.Bindings[j].CandidateID+"/"+out.Bindings[j].IndexPath
	})
	if checkAssemblyDebianIndexSelection(out, policy) != nil {
		return assemblyDebianIndexSelectionV1{}, errAssemblyDebianEvidence
	}
	return out, nil
}

func checkAssemblyDebianIndexSelection(s assemblyDebianIndexSelectionV1, policy assemblyDebianRootPolicyV1) error {
	if s.Schema != 1 || s.Profile != assemblyDebianIndexProfile || s.SourcePolicy != policy.SourcePolicy || s.ProducerConfig != policy.ProducerConfig || len(s.Objects) != 8 || len(s.Bindings) != 12 || len(policy.Releases) != 6 {
		return errAssemblyDebianEvidence
	}
	objects := map[string]assemblyDebianObjectRef{}
	total, previous := int64(0), ""
	for _, o := range s.Objects {
		if !checkAssemblyDebianObject(o, "xz-index", assemblyDebianIndexObjectLimit) || o.SHA256 <= previous || o.Bytes > assemblyDebianIndexBytes-total {
			return errAssemblyDebianEvidence
		}
		total += o.Bytes
		previous = o.SHA256
		objects[o.SHA256] = o
	}
	if total != assemblyDebianIndexBytes {
		return errAssemblyDebianEvidence
	}
	selections := map[string]assemblyDebianReleaseSelectionV1{}
	for _, p := range policy.Releases {
		selections[p.CandidateID] = p
	}
	used := map[string]bool{}
	counts := map[string]int{}
	previous = ""
	for _, b := range s.Bindings {
		p, exists := selections[b.CandidateID]
		key := b.CandidateID + "/" + b.IndexPath
		if !exists || key <= previous || b.ReleaseOrdinal < 0 || b.ReleaseOrdinal >= 5 || objects[b.CompressedObject.SHA256] != b.CompressedObject {
			return errAssemblyDebianEvidence
		}
		previous = key
		era := "current"
		if p.HistoricalTimestamp != nil {
			era = "historical-base-anchor"
		}
		if b.Era != era || era == "current" && b.HistoricalAnchor != nil || era == "historical-base-anchor" && (b.HistoricalAnchor == nil || *b.HistoricalAnchor != policy.HistoricalAnchor) {
			return errAssemblyDebianEvidence
		}
		matched := false
		for _, claim := range p.SignedIndexes {
			if claim.Path == b.IndexPath && claim.Bytes == b.CompressedObject.Bytes && claim.SHA256 == b.CompressedObject.SHA256 {
				matched = true
			}
		}
		if !matched || b.IndexPath != "main/binary-amd64/Packages.xz" && b.IndexPath != "main/source/Sources.xz" {
			return errAssemblyDebianEvidence
		}
		// URI location is a configured attribution, never proof of acquisition.
		base := "https://deb.debian.org/debian/"
		if p.Codename == "bookworm-security" {
			base = "https://security.debian.org/debian-security/"
		}
		if era == "historical-base-anchor" {
			archive := "debian"
			if p.Codename == "bookworm-security" {
				archive = "debian-security"
			}
			base = "https://snapshot.debian.org/archive/" + archive + "/" + assemblyDebianSnapshotTimestamp + "/"
		}
		prefix := base + "dists/" + p.Codename + "/"
		section := "main/source/"
		if b.IndexPath == "main/binary-amd64/Packages.xz" {
			section = "main/binary-amd64/"
		}
		if len(b.Location.ConfiguredURL) > 512 || len(b.Location.ConfiguredByHashURL) > 512 || b.Location.ConfiguredURL != prefix+b.IndexPath || b.Location.ConfiguredByHashURL != prefix+section+"by-hash/SHA256/"+b.CompressedObject.SHA256 {
			return errAssemblyDebianEvidence
		}
		used[b.CompressedObject.SHA256] = true
		counts[b.CandidateID]++
	}
	if len(used) != 8 || len(counts) != 6 {
		return errAssemblyDebianEvidence
	}
	for _, count := range counts {
		if count != 2 {
			return errAssemblyDebianEvidence
		}
	}
	return nil
}

func checkAssemblyDebianIndexBindings(value assemblyDebianSignedIndexBindingsV1, policy assemblyDebianRootPolicyV1, keys assemblyDebianKeyObservationsV1, signatures assemblyDebianReleaseSignaturesV1, selection assemblyDebianIndexSelectionV1) error {
	if checkAssemblyDebianIndexSelection(selection, policy) != nil || value.Schema != 1 || value.RecordType != "luci-debian-signed-index-bindings-v1" || value.DependencyLockSHA256 != policy.DependencyLockSHA256 || value.AcquisitionManifestSHA256 != policy.AcquisitionManifestSHA256 || value.PayloadDecode != "not-performed" || value.ParagraphCoverage != "not-evaluated" || value.ServerEligibility != "not-evaluated-server-owned" {
		return errAssemblyDebianEvidence
	}
	policyRef, pe := assemblyDebianRecordRef("official-root-policy", policy)
	keyRef, ke := assemblyDebianRecordRef("key-observations", keys)
	sigRef, se := assemblyDebianRecordRef("release-signatures", signatures)
	config, _ := json.Marshal(selection)
	if pe != nil || ke != nil || se != nil || value.RootPolicy != policyRef || value.KeyObservations != keyRef || value.ReleaseSignatures != sigRef || !equalAssemblyDebianJSON(value.Objects, selection.Objects) || !equalAssemblyDebianJSON(value.Bindings, selection.Bindings) || value.Capture.Profile != assemblyDebianIndexProfile || value.Capture.Method != "original-held-file-size-sha256" || value.Capture.Config != (assemblyDebianObjectRef{"index-profile", assemblyCatalogDigest(config), int64(len(config))}) || !checkAssemblyDebianObject(value.Capture.SourceReceipt, "index-acquisition-receipt", assemblyCatalogRecordLimit) || !validAssemblyCatalogRef(value.Capture.RetainedManifest) || value.Capture.RetainedManifest.Kind != "producer-raw-manifest" || !assemblyDebianEpoch(value.Capture.ObservedAt) {
		return errAssemblyDebianEvidence
	}
	// Ordinals bind to actual first3 authenticated outputs, not policy SHA alone.
	for _, binding := range value.Bindings {
		if len(signatures.Releases) != 5 {
			return errAssemblyDebianEvidence
		}
		release := signatures.Releases[binding.ReleaseOrdinal]
		found := false
		for _, logical := range release.Logical {
			if logical.CandidateID == binding.CandidateID {
				for _, claim := range logical.SignedIndexes {
					if claim.Path == binding.IndexPath && claim.SHA256 == binding.CompressedObject.SHA256 && claim.Bytes == binding.CompressedObject.Bytes {
						found = true
					}
				}
			}
		}
		if !found {
			return errAssemblyDebianEvidence
		}
	}
	_, err := assemblyDebianRecordRef("signed-index-bindings", value)
	return err
}

func (c *assemblyDebianCapturedIndexes) retire() error {
	if c == nil {
		return errAssemblyDebianCapture
	}
	c.mu.Lock()
	if c.phase == "candidate" {
		handoff := c.handoff
		c.mu.Unlock()
		if handoff == nil {
			return errAssemblyDebianCapture
		}
		return handoff.retire()
	}
	c.phase = "retiring"
	guard := c.guard
	c.mu.Unlock()
	if guard == nil {
		return errAssemblyDebianCapture
	}
	if err := guard.Close(); err != nil {
		return err
	}
	c.mu.Lock()
	c.phase = "closed"
	c.mu.Unlock()
	return nil
}

// No caller creates this capability today: the actual acquisition/producer
// bridge is separately missing. The constructor never launches or adopts paths.
func newAssemblyDebianIndexCandidate(ctx context.Context, owner *assemblyInputOwner, first3 *assemblyDebianDomainCandidate, captured *assemblyDebianCapturedIndexes, now int64) (*assemblyDebianIndexCandidate, error) {
	began := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if first3 == nil || captured == nil || captured.owner != owner || captured.first3 != first3 || first3.capture == nil || first3.capture.run == nil || first3.capture.run.owner != owner || first3.current(ctx) != nil {
		return nil, errAssemblyDebianCapture
	}
	r := first3.capture.run
	selection, err := normalizeAssemblyDebianIndexSelection(first3.capture.raw["policy-source"], first3.capture.raw["producer-config"])
	if err != nil {
		return nil, err
	}
	if r.completion == nil || !equalAssemblyDebianJSON(selection, captured.selection) || !assemblyIndexFirst3Canonical(first3) || checkAssemblyIndexFirst3Time(first3, now) != nil {
		return nil, errAssemblyDebianCapture
	}
	captured.mu.Lock()
	completion, guard, phase := captured.completion, captured.guard, captured.phase
	captured.mu.Unlock()
	if phase != "settled" || guard == nil || completion == nil || completion.runID != captured.id || !checkLogicalID(captured.id) || completion.exitCode != 0 || !completion.originalHandleOwned || !completion.exitConfirmed || !completion.capturesSettled || !completion.watchdogSettled || !completion.observerSettled || !completion.publicationSettled || !completion.handlesSettled || completion.unknown || !finiteAssemblyDebianEpoch(completion.started) || !finiteAssemblyDebianEpoch(completion.completed) || completion.started < r.completion.completed || completion.completed < completion.started || completion.completed-completion.started > 60 || completion.completed > float64(now+first3.policy.FutureSkewSeconds) || !assemblyCatalogHex(completion.outerSHA) || captured.receipt.SHA256 != completion.outerReceiptSHA || !checkAssemblyDebianObject(captured.receipt, "index-acquisition-receipt", assemblyCatalogRecordLimit) || guard.CheckCurrent(ctx) != nil {
		return nil, errAssemblyDebianCapture
	}
	captured.mu.Lock()
	if captured.phase != "settled" {
		captured.mu.Unlock()
		return nil, errAssemblyDebianCapture
	}
	captured.phase = "capturing"
	captured.mu.Unlock()
	moved := false
	defer func() {
		if !moved {
			captured.mu.Lock()
			if captured.phase == "capturing" {
				captured.phase = "retiring"
			}
			captured.mu.Unlock()
		}
	}()
	first3.mu.Lock()
	expected, retiring, adopting, unknown := first3.targetCAS, first3.retiring, first3.adopting, first3.publicationUnknown || first3.publicationRetired
	first3.mu.Unlock()
	if expected == (assemblyCatalogCAS{}) || retiring || adopting || unknown {
		return nil, errAssemblyDebianCapture
	}
	current, err := owner.Current(ctx, r.actor)
	if err != nil || current.CAS != expected || current.Source != r.source || current.State.Policy != r.policy || !assemblyIndexContains(current.Index, first3.records) {
		return nil, errAssemblyCatalogConflict
	}
	receipt, err := guard.ReadBounded(ctx, "index-acquisition-receipt", captured.receipt.Bytes)
	if err != nil || int64(len(receipt)) != captured.receipt.Bytes || assemblyCatalogDigest(receipt) != captured.receipt.SHA256 {
		return nil, errAssemblyDebianCapture
	}
	for _, object := range selection.Objects {
		if _, e := readAssemblyDebianIndex(ctx, guard, object); e != nil {
			return nil, e
		}
	}
	profile, _ := json.Marshal(selection)
	profileRef := assemblyDebianObjectRef{"index-profile", assemblyCatalogDigest(profile), int64(len(profile))}
	refs := make([]assemblyCatalogRef, 0, 3)
	for _, item := range first3.records {
		refs = append(refs, item.ref)
	}
	manifest := assemblyDebianIndexRetainedManifestV1{1, captured.id, expected, r.source, r.policy, refs, profileRef, captured.receipt, selection.Objects}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil || len(manifestBytes) > assemblyCatalogRecordLimit {
		return nil, errAssemblyDebianCapture
	}
	manifestRef := assemblyCatalogRef{1, "producer-raw-manifest", assemblyCatalogDigest(manifestBytes), int64(len(manifestBytes))}
	pr, _ := assemblyDebianRecordRef("official-root-policy", first3.policy)
	kr, _ := assemblyDebianRecordRef("key-observations", first3.keys)
	sr, _ := assemblyDebianRecordRef("release-signatures", first3.signatures)
	observed := assemblyIndexFinalNow(now, began)
	value := assemblyDebianSignedIndexBindingsV1{1, "luci-debian-signed-index-bindings-v1", pr, kr, sr, first3.policy.DependencyLockSHA256, first3.policy.AcquisitionManifestSHA256, assemblyDebianIndexCaptureV1{assemblyDebianIndexProfile, profileRef, captured.receipt, manifestRef, observed, "original-held-file-size-sha256"}, selection.Objects, selection.Bindings, "not-performed", "not-evaluated", "not-evaluated-server-owned"}
	if err = checkAssemblyDebianIndexBindings(value, first3.policy, first3.keys, first3.signatures, selection); err != nil {
		return nil, err
	}
	canonical, _ := json.Marshal(value)
	ref, err := assemblyDebianRecordRef("signed-index-bindings", value)
	if err != nil {
		return nil, err
	}
	if first3.current(ctx) != nil || guard.CheckCurrent(ctx) != nil || checkAssemblyIndexFirst3Time(first3, assemblyIndexFinalNow(now, began)) != nil {
		return nil, errAssemblyDebianCapture
	}
	candidate := &assemblyDebianIndexCandidate{owner: owner, first3: first3, capture: captured, expected: expected, record: value, manifest: manifest, config: profile, receipt: receipt, canonical: canonical, manifestBytes: manifestBytes, ref: ref, manifestRef: manifestRef}
	if err := transferAssemblyDebianIndexOwners(ctx, candidate); err != nil {
		return nil, err
	}
	moved = true
	return candidate, nil
}

// This lower-level edge is shared by the real byte constructor and inert
// ownership fixtures. It cannot mint metadata or prove any index bytes. Every
// native Transfer/CheckCurrent/Close and authority/CAS read occurs OUTSIDE the
// prior-owner mutexes, so retirement can latch while synchronous IO is pending.
func transferAssemblyDebianIndexOwners(ctx context.Context, c *assemblyDebianIndexCandidate) error {
	if c == nil || c.capture == nil || c.first3 == nil || c.first3.capture == nil || c.first3.capture.run == nil || c.owner == nil || c.capture.owner != c.owner || c.capture.first3 != c.first3 || c.first3.capture.run.owner != c.owner {
		return errAssemblyDebianCapture
	}
	captured, first3 := c.capture, c.first3
	captured.mu.Lock()
	first3.mu.Lock()
	if ctx.Err() != nil || captured.phase != "capturing" || captured.handoff != nil || captured.guard == nil || first3.retiring || first3.adopting || first3.indexTransfer || first3.indexHandoff != nil || first3.guard == nil || first3.inputs == nil || first3.targetCAS != c.expected {
		first3.mu.Unlock()
		captured.mu.Unlock()
		return errAssemblyDebianCapture
	}
	captured.phase = "transferring"
	first3.indexTransfer = true
	first3.mu.Unlock()
	captured.mu.Unlock()
	promoted := false
	defer func() {
		if !promoted {
			captured.mu.Lock()
			if captured.phase == "transferring" {
				captured.phase = "retiring"
			}
			captured.mu.Unlock()
			first3.mu.Lock()
			first3.indexTransfer = false
			first3.retiring = true
			first3.mu.Unlock()
		}
	}()
	indexes, err := moveAssemblyDebianIndexGuard(ctx, c, 0)
	if err != nil {
		return err
	}
	output, err := moveAssemblyDebianIndexGuard(ctx, c, 1)
	if err != nil {
		return err
	}
	inputs, err := moveAssemblyDebianIndexGuard(ctx, c, 2)
	if err != nil {
		return err
	}
	for _, g := range []backend.DocumentImageAssemblyEvidenceGuard{indexes, output, inputs} {
		if err := g.CheckCurrent(ctx); err != nil {
			return err
		}
	}
	r := first3.capture.run
	current, err := c.owner.Current(ctx, r.actor)
	if err != nil || current.CAS != c.expected || current.Source != r.source || current.State.Policy != r.policy || !assemblyIndexContains(current.Index, first3.records) {
		return errAssemblyCatalogConflict
	}
	// CheckCurrent may itself wait in synchronous native IO. Recheck context and
	// BOTH retirement latches afterwards, while holding only short state locks.
	captured.mu.Lock()
	first3.mu.Lock()
	defer captured.mu.Unlock()
	defer first3.mu.Unlock()
	if ctx.Err() != nil || captured.phase != "transferring" || captured.handoff != nil || captured.guard != indexes || first3.retiring || first3.adopting || !first3.indexTransfer || first3.indexHandoff != nil || first3.guard != output || first3.inputs != inputs || first3.targetCAS != c.expected {
		return errAssemblyDebianCapture
	}
	c.indexes, c.output, c.inputs = indexes, output, inputs
	captured.guard = nil
	captured.phase = "candidate"
	captured.handoff = c
	first3.guard, first3.inputs = nil, nil
	first3.indexTransfer = false
	first3.indexHandoff = c
	promoted = true
	return nil
}

func assemblyIndexHandoffLatched(ctx context.Context, c *assemblyDebianIndexCandidate) bool {
	if ctx.Err() != nil {
		return true
	}
	c.capture.mu.Lock()
	stopped := c.capture.phase != "transferring" || c.capture.handoff != nil
	c.capture.mu.Unlock()
	c.first3.mu.Lock()
	stopped = stopped || c.first3.retiring || !c.first3.indexTransfer || c.first3.indexHandoff != nil
	c.first3.mu.Unlock()
	return stopped
}

// Three closed ownership slots, not a callback or arbitrary owner map. A token
// returned together with an error is still registered in its ORIGINAL owner
// before refusal. An intersecting retirement closes the exact late token there;
// failed Close remains retained and retire-only, never silently abandoned.
func moveAssemblyDebianIndexGuard(ctx context.Context, c *assemblyDebianIndexCandidate, slot int) (backend.DocumentImageAssemblyEvidenceGuard, error) {
	var old backend.DocumentImageAssemblyEvidenceGuard
	switch slot {
	case 0:
		c.capture.mu.Lock()
		old = c.capture.guard
		c.capture.mu.Unlock()
	case 1:
		c.first3.mu.Lock()
		old = c.first3.guard
		c.first3.mu.Unlock()
	case 2:
		c.first3.mu.Lock()
		old = c.first3.inputs
		c.first3.mu.Unlock()
	default:
		return nil, errAssemblyDebianCapture
	}
	if old == nil || assemblyIndexHandoffLatched(ctx, c) {
		return nil, errAssemblyDebianCapture
	}
	next, err := old.Transfer(ctx)
	if next != nil {
		switch slot {
		case 0:
			c.capture.mu.Lock()
			c.capture.guard = next
			c.capture.mu.Unlock()
		case 1:
			c.first3.mu.Lock()
			c.first3.guard = next
			c.first3.mu.Unlock()
		case 2:
			c.first3.mu.Lock()
			c.first3.inputs = next
			c.first3.mu.Unlock()
		}
	}
	if err != nil || next == nil || next == old || assemblyIndexHandoffLatched(ctx, c) {
		var closeErr error
		if next != nil {
			closeErr = next.Close()
		}
		return nil, errors.Join(err, errAssemblyDebianCapture, closeErr)
	}
	return next, nil
}

func assemblyIndexFirst3Canonical(c *assemblyDebianDomainCandidate) bool {
	if c == nil || len(c.records) != 3 {
		return false
	}
	for i, item := range []struct {
		kind  string
		value any
	}{{"official-root-policy", c.policy}, {"key-observations", c.keys}, {"release-signatures", c.signatures}} {
		ref, err := assemblyDebianRecordRef(item.kind, item.value)
		data, e := json.Marshal(item.value)
		if err != nil || e != nil || c.records[i].ref != ref || !bytes.Equal(c.records[i].data, data) {
			return false
		}
	}
	return true
}

func checkAssemblyIndexFirst3Time(c *assemblyDebianDomainCandidate, now int64) error {
	if c == nil || checkAssemblyDebianSignatures(c.signatures, c.policy, c.keys, c.pins, now) != nil {
		return errAssemblyDebianEvidence
	}
	for _, selection := range c.policy.Releases {
		if err := checkAssemblyDebianReleaseTimeUse(selection, c.policy, now); err != nil {
			return err
		}
	}
	return nil
}

func assemblyIndexContains(index assemblyCatalogIndex, objects []assemblyCatalogObject) bool {
	if len(objects) != 3 {
		return false
	}
	for _, object := range objects {
		found := false
		for _, ref := range index.Records {
			if ref == object.ref {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func readAssemblyDebianIndex(ctx context.Context, guard backend.DocumentImageAssemblyEvidenceGuard, object assemblyDebianObjectRef) ([]byte, error) {
	if guard == nil || !checkAssemblyDebianObject(object, "xz-index", assemblyDebianIndexObjectLimit) {
		return nil, errAssemblyDebianEvidence
	}
	data, err := guard.ReadBounded(ctx, "xz-index:"+object.SHA256, object.Bytes)
	if err != nil || int64(len(data)) != object.Bytes || assemblyCatalogDigest(data) != object.SHA256 {
		return nil, errAssemblyDebianCapture
	}
	return data, nil
}

// Keep time arithmetic the same server-clock/monotonic rule as first3 adoption.
func assemblyIndexFinalNow(now int64, began time.Time) int64 {
	return now + int64(math.Ceil(time.Since(began).Seconds()))
}
