package api

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func (c *assemblyDebianIndexCandidate) current(ctx context.Context) error {
	if c == nil || c.owner == nil || c.first3 == nil || c.first3.capture == nil || c.first3.capture.run == nil {
		return errAssemblyDebianCapture
	}
	c.mu.Lock()
	retiring, output, inputs, indexes := c.retiring, c.output, c.inputs, c.indexes
	c.mu.Unlock()
	if retiring || output == nil || inputs == nil || indexes == nil || ctx.Err() != nil {
		return errAssemblyDebianCapture
	}
	for _, guard := range []backend.DocumentImageAssemblyEvidenceGuard{output, inputs, indexes} {
		if err := guard.CheckCurrent(ctx); err != nil {
			return err
		}
	}
	if err := c.owner.authorize(ctx, c.first3.capture.run.actor); err != nil {
		return err
	}
	c.mu.Lock()
	retiring = c.retiring
	c.mu.Unlock()
	if retiring {
		return errAssemblyDebianCapture
	}
	for _, guard := range []backend.DocumentImageAssemblyEvidenceGuard{output, inputs, indexes} {
		if err := guard.CheckCurrent(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Original handles remain owned on partial/failed Close; trust is irrelevant to
// retirement. A concurrent commit can remain observable, but never report pass.
func (c *assemblyDebianIndexCandidate) retire() error {
	if c == nil {
		return errAssemblyDebianCapture
	}
	c.mu.Lock()
	c.retiring = true
	if c.publication != nil && c.publication.inFlight {
		c.publication.retired = true
		c.publicationRetired = true
		c.publicationUnknown = true
	}
	guards := []backend.DocumentImageAssemblyEvidenceGuard{c.output, c.inputs, c.indexes}
	c.mu.Unlock()
	var first error
	for _, guard := range guards {
		if guard != nil {
			if err := guard.Close(); first == nil {
				first = err
			}
		}
	}
	return first
}

// This is the second and only other closed arm of the existing permit. It does
// not accept arbitrary callbacks, records, authority flags or raw byte callers.
func (p *assemblyDebianPublicationPermit) beginIndexes(ctx context.Context, owner *assemblyInputOwner, next assemblyCatalogCAS) error {
	c := p.indexes
	c.mu.Lock()
	if c.retiring || !c.adopting || c.owner != owner || c.targetCAS != next || c.publicationRetired || c.output == nil || c.inputs == nil || c.indexes == nil {
		c.mu.Unlock()
		return errAssemblyDebianCapture
	}
	output, inputs, indexes := c.output, c.inputs, c.indexes
	c.mu.Unlock()
	for _, guard := range []backend.DocumentImageAssemblyEvidenceGuard{output, inputs, indexes} {
		if err := guard.CheckCurrent(ctx); err != nil {
			return err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retiring || c.output != output || c.inputs != inputs || c.indexes != indexes || c.targetCAS != next || ctx.Err() != nil {
		return errAssemblyDebianCapture
	}
	p.inFlight = true
	c.publication = p
	return nil
}

func (p *assemblyDebianPublicationPermit) finishIndexes(result assemblyCatalogCAS, err error) (assemblyCatalogCAS, error) {
	c := p.indexes
	c.mu.Lock()
	defer c.mu.Unlock()
	p.inFlight = false
	if p.retired || c.retiring || c.publication != p {
		c.publicationRetired, c.publicationUnknown = true, true
		return assemblyCatalogCAS{}, errAssemblyDebianPublicationUncertain
	}
	if err != nil {
		c.publicationUnknown = true
		return assemblyCatalogCAS{}, err
	}
	if result != c.targetCAS {
		c.publicationUnknown = true
		return assemblyCatalogCAS{}, errAssemblyDebianPublicationUncertain
	}
	c.publicationUnknown = false
	return result, nil
}

func (c *assemblyDebianIndexCandidate) check(now int64) error {
	if c == nil || c.first3 == nil || c.first3.capture == nil || c.first3.capture.run == nil || c.capture == nil || c.capture.owner != c.owner || !assemblyIndexFirst3Canonical(c.first3) {
		return errAssemblyDebianCapture
	}
	selection, err := normalizeAssemblyDebianIndexSelection(c.first3.capture.raw["policy-source"], c.first3.capture.raw["producer-config"])
	if err != nil {
		return err
	}
	if checkAssemblyIndexFirst3Time(c.first3, now) != nil || checkAssemblyDebianIndexBindings(c.record, c.first3.policy, c.first3.keys, c.first3.signatures, selection) != nil {
		return errAssemblyDebianEvidence
	}
	canonical, err := json.Marshal(c.record)
	ref, e := assemblyDebianRecordRef("signed-index-bindings", c.record)
	manifest, me := json.Marshal(c.manifest)
	config, ce := json.Marshal(selection)
	if err != nil || e != nil || me != nil || ce != nil || ref != c.ref || !bytes.Equal(canonical, c.canonical) || !bytes.Equal(manifest, c.manifestBytes) || !bytes.Equal(config, c.config) || c.record.Capture.RetainedManifest != c.manifestRef || c.manifestRef != (assemblyCatalogRef{1, "producer-raw-manifest", assemblyCatalogDigest(manifest), int64(len(manifest))}) || c.record.Capture.SourceReceipt.SHA256 != assemblyCatalogDigest(c.receipt) || c.record.Capture.SourceReceipt.Bytes != int64(len(c.receipt)) {
		return errAssemblyDebianCapture
	}
	r := c.first3.capture.run
	refs := make([]assemblyCatalogRef, 0, 3)
	for _, item := range c.first3.records {
		refs = append(refs, item.ref)
	}
	if c.record.Capture.ObservedAt > now+c.first3.policy.FutureSkewSeconds || c.record.Capture.ObservedAt < c.first3.keys.ObservedAt-1 || c.manifest.Schema != 1 || c.manifest.RunID != c.capture.id || c.manifest.ExpectedCAS != c.expected || c.manifest.Source != r.source || c.manifest.Policy != r.policy || !equalAssemblyDebianJSON(c.manifest.First3, refs) || c.manifest.Config != c.record.Capture.Config || c.manifest.Receipt != c.record.Capture.SourceReceipt || !equalAssemblyDebianJSON(c.manifest.Objects, c.record.Objects) {
		return errAssemblyDebianCapture
	}
	return nil
}

// No production caller/mint/acquisition is wired. The existing installer alone
// may eventually supply a sealed actual index capture; a catalog digest is not
// a substitute for it and this fourth fact never returns full InputClosureRef.
func (o *assemblyInputOwner) adoptDebianIndexBindings(ctx context.Context, c *assemblyDebianIndexCandidate) (assemblyCatalogCAS, error) {
	return o.adoptDebianIndexBindingsAt(ctx, c, time.Now().Unix())
}

func (o *assemblyInputOwner) adoptDebianIndexBindingsAt(ctx context.Context, c *assemblyDebianIndexCandidate, now int64) (assemblyCatalogCAS, error) {
	began := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if c == nil || c.owner != o {
		return assemblyCatalogCAS{}, errAssemblyDebianCapture
	}
	c.mu.Lock()
	retired := c.publicationRetired
	c.mu.Unlock()
	if retired {
		return assemblyCatalogCAS{}, errAssemblyDebianPublicationUncertain
	}
	if c.current(ctx) != nil || c.check(now) != nil {
		return assemblyCatalogCAS{}, errAssemblyDebianCapture
	}
	c.mu.Lock()
	if c.retiring || c.adopting {
		c.mu.Unlock()
		return assemblyCatalogCAS{}, errAssemblyDebianCapture
	}
	c.adopting = true
	target, targetIndex, unknown := c.targetCAS, c.targetIndex, c.publicationUnknown
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.adopting = false; c.mu.Unlock() }()
	r := c.first3.capture.run
	unlock, err := o.lock(ctx)
	if err != nil {
		return assemblyCatalogCAS{}, err
	}
	defer unlock()
	if c.current(ctx) != nil {
		return assemblyCatalogCAS{}, errAssemblyDebianCapture
	}
	old, err := o.readCurrent(ctx)
	if err != nil {
		return assemblyCatalogCAS{}, err
	}
	if target != (assemblyCatalogCAS{}) && old.CAS == target && old.Source == r.source && old.State.Policy == r.policy && equalAssemblyDebianJSON(old.Index, targetIndex) {
		return target, nil
	}
	if unknown {
		return assemblyCatalogCAS{}, errAssemblyDebianPublicationUncertain
	}
	if old.CAS != c.expected || old.Source != r.source || old.State.Policy != r.policy || old.State.Revision == math.MaxUint64 || !assemblyIndexContains(old.Index, c.first3.records) {
		return assemblyCatalogCAS{}, errAssemblyCatalogConflict
	}
	// A fresh original-handle read, not cached payloads. Both rounds are charged
	// against one unchanged <=64MiB lifetime budget. No giant payload map exists.
	for _, object := range c.record.Objects {
		data, e := readAssemblyDebianIndex(ctx, c.indexes, object)
		if e != nil {
			return assemblyCatalogCAS{}, e
		}
		if e = o.writeImmutableAssemblyObject(ctx, assemblyCatalogRef{1, "debian-index-raw", object.SHA256, object.Bytes}, data, true); e != nil {
			return assemblyCatalogCAS{}, e
		}
		data = nil
	}
	// Keep the profile/owned receipt as small raw objects under existing bounds.
	for _, data := range [][]byte{c.config, c.receipt} {
		if err = o.writeImmutableAssemblyObject(ctx, assemblyCatalogRef{1, "producer-raw", assemblyCatalogDigest(data), int64(len(data))}, data, true); err != nil {
			return assemblyCatalogCAS{}, err
		}
	}
	index := assemblyCatalogIndex{Schema: 1, Records: append([]assemblyCatalogRef{}, old.Index.Records...)}
	for _, object := range []assemblyCatalogObject{{c.manifestRef, c.manifestBytes}, {c.ref, c.canonical}} {
		if err = o.writeImmutableAssemblyObject(ctx, object.ref, object.data, false); err != nil {
			return assemblyCatalogCAS{}, err
		}
		present := false
		for _, existing := range index.Records {
			if existing == object.ref {
				present = true
			}
		}
		if !present {
			index.Records = append(index.Records, object.ref)
		}
	}
	sort.Slice(index.Records, func(i, j int) bool {
		return index.Records[i].Kind+"/"+index.Records[i].Digest < index.Records[j].Kind+"/"+index.Records[j].Digest
	})
	if !validAssemblyCatalogIndex(index) || c.current(ctx) != nil || c.check(assemblyIndexFinalNow(now, began)) != nil {
		return assemblyCatalogCAS{}, errAssemblyDebianCapture
	}
	sourceData, _ := json.Marshal(r.source)
	indexData, _ := json.Marshal(index)
	state := assemblyInputCatalogState{1, old.State.Revision + 1, assemblyCatalogRef{1, "source-descriptor", assemblyCatalogDigest(sourceData), int64(len(sourceData))}, r.policy, assemblyCatalogRef{1, "evidence-index", assemblyCatalogDigest(indexData), int64(len(indexData))}}
	head, _ := json.Marshal(state)
	c.mu.Lock()
	c.targetCAS = assemblyCatalogCAS{state.Revision, assemblyCatalogDigest(head)}
	c.targetIndex = index
	c.mu.Unlock()
	return o.publishCatalogHeadLocked(ctx, r.actor, old, r.source, r.policy, index, &assemblyDebianPublicationPermit{indexes: c})
}
