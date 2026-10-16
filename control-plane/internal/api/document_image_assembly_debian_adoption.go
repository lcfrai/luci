package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"
)

type assemblyCatalogObject struct { ref assemblyCatalogRef; data []byte }

var errAssemblyDebianPublicationUncertain = errors.New("Debian domain publication uncertain; no retry or overwrite")

// A permit is bound to one sealed candidate, owner and exact next head. It is
// not a caller callback. Native resource retirement never holds this lock over
// filesystem/authority IO; an intersecting retirement invalidates success.
type assemblyDebianPublicationPermit struct {
	candidate *assemblyDebianDomainCandidate
	indexes *assemblyDebianIndexCandidate // exactly one concrete candidate arm
	inFlight bool // protected by candidate.mu
	retired bool
}

func (p *assemblyDebianPublicationPermit) begin(ctx context.Context, owner *assemblyInputOwner, next assemblyCatalogCAS) error {
	if p==nil || (p.candidate==nil)==(p.indexes==nil) { return errAssemblyDebianCapture }
	if p.indexes!=nil{return p.beginIndexes(ctx,owner,next)}
	c:=p.candidate
	c.mu.Lock()
	if c.retiring || !c.adopting || c.capture==nil || c.capture.run==nil || c.capture.run.owner!=owner || c.targetCAS!=next || c.guard==nil || c.inputs==nil || c.publicationRetired { c.mu.Unlock();return errAssemblyDebianCapture }
	output,inputs:=c.guard,c.inputs
	c.mu.Unlock()
	if err:=output.CheckCurrent(ctx);err!=nil{return err};if err:=inputs.CheckCurrent(ctx);err!=nil{return err}
	c.mu.Lock();defer c.mu.Unlock()
	if c.retiring || c.guard!=output || c.inputs!=inputs || c.targetCAS!=next || ctx.Err()!=nil { return errAssemblyDebianCapture }
	p.inFlight=true;c.publication=p
	return nil
}

func (p *assemblyDebianPublicationPermit) finish(result assemblyCatalogCAS, err error)(assemblyCatalogCAS,error){
	if p==nil || (p.candidate==nil)==(p.indexes==nil){return assemblyCatalogCAS{},errAssemblyDebianCapture}
	if p.indexes!=nil{return p.finishIndexes(result,err)}
	c:=p.candidate;c.mu.Lock();defer c.mu.Unlock()
	p.inFlight=false
	if p.retired || c.retiring || c.publication!=p {
		c.publicationRetired,c.publicationUnknown=true,true
		return assemblyCatalogCAS{},errAssemblyDebianPublicationUncertain
	}
	if err!=nil { c.publicationUnknown=true;return assemblyCatalogCAS{},err }
	if result!=c.targetCAS {c.publicationUnknown=true;return assemblyCatalogCAS{},errAssemblyDebianPublicationUncertain}
	c.publicationUnknown=false
	return result,nil
}
type assemblyDebianRawManifestEntry struct {
	Slot string `json:"slot"`
	Object assemblyDebianObjectRef `json:"object"`
}
type assemblyDebianRawManifest struct {
	Schema int `json:"schema"`
	RunID string `json:"runId"`
	ExpectedCAS assemblyCatalogCAS `json:"expectedCas"`
	Source assemblyCatalogSource `json:"source"`
	Policy assemblyCatalogPolicy `json:"policy"`
	Entries []assemblyDebianRawManifestEntry `json:"entries"`
	CompleteDomains []string `json:"completeDomains"`
	MissingDomains []string `json:"missingDomains"`
}

func (c *assemblyDebianDomainCandidate) current(ctx context.Context) error {
	if c == nil || c.capture == nil || c.capture.run == nil { return errAssemblyDebianCapture }
	c.mu.Lock(); retiring, output, inputs := c.retiring, c.guard, c.inputs; c.mu.Unlock()
	if retiring || output == nil || inputs == nil || ctx.Err() != nil { return errAssemblyDebianCapture }
	if err := output.CheckCurrent(ctx); err != nil { return err }
	if err := inputs.CheckCurrent(ctx); err != nil { return err }
	if err:=c.capture.run.owner.authorize(ctx,c.capture.run.actor);err!=nil{return err}
	c.mu.Lock();retiring=c.retiring;c.mu.Unlock();if retiring{return errAssemblyDebianCapture}
	if err:=inputs.CheckCurrent(ctx);err!=nil{return err};return output.CheckCurrent(ctx)
}

// Cleanup is independent of current installer eligibility. Failed closes remain
// owned and retire-only, including after a head was published; three fact rows
// never authorize a native allocation or stand in for full input closure.
func (c *assemblyDebianDomainCandidate) retire() error {
	if c == nil { return errAssemblyDebianCapture }
	c.mu.Lock(); c.retiring=true; if c.publication!=nil && c.publication.inFlight {c.publication.retired=true;c.publicationRetired=true;c.publicationUnknown=true}; output,inputs:=c.guard,c.inputs; c.mu.Unlock()
	var first error
	if output != nil { first=output.Close() }
	if inputs != nil { if err:=inputs.Close(); first==nil { first=err } }
	return first
}

// adoptDebianDomains selects only three actual captured fact domains and their
// raw manifest. No full InputClosureRef/ResolveCurrent/boot/public route exists.
// The owner has no production caller until its real Windows producer bridge
// can supply the live completion and original byte guards required by capture.
func (o *assemblyInputOwner) adoptDebianDomains(ctx context.Context, c *assemblyDebianDomainCandidate) (assemblyCatalogCAS,error) {
	return o.adoptDebianDomainsAt(ctx,c,time.Now().Unix())
}

// Internal body accepts the single server clock observation for deterministic
// metadata tests; the only production entry above always obtains time.Now.
// This is not a public clock override or native execution/eligibility route.
func (o *assemblyInputOwner) adoptDebianDomainsAt(ctx context.Context, c *assemblyDebianDomainCandidate, now int64) (assemblyCatalogCAS,error) {
	began:=time.Now()
	ctx,cancel:=context.WithTimeout(ctx,2*time.Second); defer cancel()
	if c!=nil {c.mu.Lock();retiredPublication:=c.publicationRetired;c.mu.Unlock();if retiredPublication{return assemblyCatalogCAS{},errAssemblyDebianPublicationUncertain}}
	if c==nil || c.capture==nil || c.capture.run==nil || c.capture.run.owner!=o || c.current(ctx)!=nil { return assemblyCatalogCAS{},errAssemblyDebianCapture }
	r:=c.capture.run
	c.mu.Lock()
	if c.adopting || c.retiring { c.mu.Unlock(); return assemblyCatalogCAS{},errAssemblyDebianCapture }
	c.adopting=true
	target,targetIndex,publicationUnknown:=c.targetCAS,c.targetIndex,c.publicationUnknown
	c.mu.Unlock()
	defer func(){c.mu.Lock();c.adopting=false;c.mu.Unlock()}()
	if !validAssemblyCatalogSource(r.source) || !validAssemblyCatalogPolicy(r.policy) || r.policy.Digest!=assemblyDebianPolicySourceSHA || r.source.DependencyLockDigest!=assemblyDebianLockSHA || r.source.InputManifestDigest!=assemblyDebianManifestSHA || checkAssemblyDebianSignatures(c.signatures,c.policy,c.keys,c.pins,now)!=nil { return assemblyCatalogCAS{},errAssemblyDebianCapture }
	unlock,err:=o.lock(ctx); if err!=nil { return assemblyCatalogCAS{},err }; defer unlock()
	if err=c.current(ctx); err!=nil { return assemblyCatalogCAS{},err }
	old,err:=o.readCurrent(ctx)
	// A capture needs a previously installer-selected exact source/policy head.
	// It cannot bootstrap source authority by supplying digest-shaped arguments.
	if err!=nil { return assemblyCatalogCAS{},err }
	if target!=(assemblyCatalogCAS{}) && old.CAS==target && old.Source==r.source && old.State.Policy==r.policy && equalAssemblyDebianJSON(old.Index,targetIndex) { return target,nil }
	if publicationUnknown {return assemblyCatalogCAS{},errAssemblyDebianPublicationUncertain}
	if old.CAS!=r.expected || old.Source!=r.source || old.State.Policy!=r.policy || old.State.Revision==math.MaxUint64 { return assemblyCatalogCAS{},errAssemblyCatalogConflict }
	manifest:=assemblyDebianRawManifest{Schema:1,RunID:r.id,ExpectedCAS:r.expected,Source:r.source,Policy:r.policy,CompleteDomains:[]string{"official-root-policy","key-observations","release-signatures"},MissingDomains:[]string{"signed-index-bindings","debian-binaries","debian-sources","base-rootfs","wheels","dependency-closure","expected-installed-files","notices-and-sources","luci-source-helper"}}
	slots:=make([]string,0,len(c.capture.raw)); for slot:=range c.capture.raw { slots=append(slots,slot) }; sort.Strings(slots)
	for _,slot:=range slots {
		data:=c.capture.raw[slot]
		object:=assemblyDebianObjectRef{Role:slot,SHA256:assemblyCatalogDigest(data),Bytes:int64(len(data))}
		manifest.Entries=append(manifest.Entries,assemblyDebianRawManifestEntry{slot,object})
		if err=o.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"producer-raw",object.SHA256,object.Bytes},data,true); err!=nil { return assemblyCatalogCAS{},err }
	}
	data,err:=json.Marshal(manifest); if err!=nil || len(data)>assemblyCatalogRecordLimit { return assemblyCatalogCAS{},errAssemblyDebianCapture }
	manifestRef:=assemblyCatalogRef{1,"producer-raw-manifest",assemblyCatalogDigest(data),int64(len(data))}
	objects:=append(append([]assemblyCatalogObject{},c.records...),assemblyCatalogObject{manifestRef,data})
	if len(c.records)!=3 { return assemblyCatalogCAS{},errAssemblyDebianCapture }
	// Recompute the canonical domain bytes, not mutable cached candidate fields.
	for i,item:=range []struct{kind string; value any}{{"official-root-policy",c.policy},{"key-observations",c.keys},{"release-signatures",c.signatures}} {
		ref,e:=assemblyDebianRecordRef(item.kind,item.value); canonical,_:=json.Marshal(item.value)
		if e!=nil || c.records[i].ref!=ref || string(c.records[i].data)!=string(canonical) { return assemblyCatalogCAS{},errAssemblyDebianCapture }
	}
	index:=assemblyCatalogIndex{Schema:1,Records:append([]assemblyCatalogRef{},old.Index.Records...)}
	for _,object:=range objects {
		if err=o.writeImmutableAssemblyObject(ctx,object.ref,object.data,false); err!=nil { return assemblyCatalogCAS{},err }
		present:=false
		for _,existing:=range index.Records { if existing==object.ref { present=true; break } }
		if !present { index.Records=append(index.Records,object.ref) }
	}
	sort.Slice(index.Records,func(i,j int)bool{return index.Records[i].Kind+"/"+index.Records[i].Digest<index.Records[j].Kind+"/"+index.Records[j].Digest})
	// Advance the captured server clock conservatively across bounded IO. Tests
	// use the same monotonic advance, not a frozen eligibility time at publication.
	finalNow:=now+int64(math.Ceil(time.Since(began).Seconds()))
	if !validAssemblyCatalogIndex(index) || c.current(ctx)!=nil || checkAssemblyDebianSignatures(c.signatures,c.policy,c.keys,c.pins,finalNow)!=nil { return assemblyCatalogCAS{},errAssemblyDebianCapture }
	sourceData,_:=json.Marshal(r.source); indexData,_:=json.Marshal(index)
	state:=assemblyInputCatalogState{1,old.State.Revision+1,assemblyCatalogRef{1,"source-descriptor",assemblyCatalogDigest(sourceData),int64(len(sourceData))},r.policy,assemblyCatalogRef{1,"evidence-index",assemblyCatalogDigest(indexData),int64(len(indexData))}}
	head,_:=json.Marshal(state)
	c.mu.Lock();c.targetCAS=assemblyCatalogCAS{state.Revision,assemblyCatalogDigest(head)};c.targetIndex=index;c.mu.Unlock()
	return o.publishCatalogHeadLocked(ctx,r.actor,old,r.source,r.policy,index,&assemblyDebianPublicationPermit{candidate:c})
}
