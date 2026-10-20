package api

import (
 "context"
 "encoding/json"
 "math"
 "sort"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// This third closed permit accepts only the genuine current full constructor's
// sealed candidate. Neither membership in an index nor arbitrary JSON can call
// publishCatalogHeadLocked without that final Product adoption permit.
func(p *assemblyDebianPublicationPermit)beginClosure(ctx context.Context,owner *assemblyInputOwner,next assemblyCatalogCAS)error{
 c:=p.accepted
 if c==nil||c.owner!=owner||c.plan==nil||c.current(ctx)!=nil{return errAssemblyDebianCapture}
 c.mu.Lock();valid:=c.adopting&&!c.retiring&&!c.publicationRetired&&c.targetCAS==next;c.mu.Unlock();if !valid{return errAssemblyDebianCapture}
 permit,e:=c.plan.owner.api.deps.Product.BeginRuntimeInputProofAdoption(ctx,c.plan.identity,runtimeinstall.InputProofCAS{Revision:next.Revision,Digest:next.Digest});if e!=nil||permit==nil||!permit.Current(){return errAssemblyDebianCapture};p.product=permit
 c.mu.Lock();defer c.mu.Unlock()
 if c.retiring||!c.adopting||c.targetCAS!=next||ctx.Err()!=nil||!closureNativeCurrent(c){_ = permit.Finish(runtimeinstall.InputProofCAS{},errAssemblyDebianCapture);return errAssemblyDebianCapture}
 p.inFlight=true;c.publication=p;return nil
}
func(p *assemblyDebianPublicationPermit)finishClosure(result assemblyCatalogCAS,err error)(assemblyCatalogCAS,error){
 c:=p.accepted
 if p.product!=nil{if e:=p.product.Finish(runtimeinstall.InputProofCAS{Revision:result.Revision,Digest:result.Digest},err);e!=nil{err=errAssemblyDebianPublicationUncertain}}
 c.mu.Lock();defer c.mu.Unlock();p.inFlight=false
 if p.retired||c.retiring||c.publication!=p||result!=c.targetCAS||err!=nil||!closureNativeCurrent(c){c.publicationUnknown=true;if p.retired||c.retiring{c.publicationRetired=true};if err!=nil{return assemblyCatalogCAS{},err};return assemblyCatalogCAS{},errAssemblyDebianPublicationUncertain}
 c.publicationUnknown=false;return result,nil
}
func closureNativeCurrent(c *assemblyAcceptedInputCandidate)bool{return c!=nil&&c.binary!=nil&&c.binary.spool!=nil&&c.binary.spool.owner!=nil&&!c.binary.spool.owner.nativeRetired()&&time.Now().Before(c.binary.spool.owner.workLimit())}

func(o *assemblyInputOwner)adoptDocumentAssemblyClosure(ctx context.Context,c *assemblyAcceptedInputCandidate)(assemblyCatalogCAS,error){
 if c==nil||c.owner!=o||c.plan==nil||c.binary==nil||c.current(ctx)!=nil{return assemblyCatalogCAS{},errAssemblyDebianCapture}
 ctx,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel()
 c.mu.Lock();if c.retiring||c.adopting||c.publicationRetired{c.mu.Unlock();return assemblyCatalogCAS{},errAssemblyDebianCapture};c.adopting=true;target,targetIndex,unknown:=c.targetCAS,c.targetIndex,c.publicationUnknown;c.mu.Unlock()
 defer func(){c.mu.Lock();c.adopting=false;c.mu.Unlock()}()
 records:=c.canonicalCopy();if len(records)<4||len(records)>assemblyCatalogRefLimit{return assemblyCatalogCAS{},errAssemblyDebianCapture}
 byKind:=map[string]int{};seen:=map[assemblyCatalogRef]bool{}
 for _,record:=range records{
  if !validAssemblyCatalogRef(record.ref)||record.ref.Kind=="evidence-index"||record.ref.Kind=="source-descriptor"||record.ref.Digest!=assemblyCatalogDigest(record.data)||record.ref.Bytes!=int64(len(record.data))||seen[record.ref]{return assemblyCatalogCAS{},errAssemblyDebianCapture}
  seen[record.ref]=true;byKind[record.ref.Kind]++
 }
 for _,kind:=range []string{"signed-index-bindings","debian-binaries","input-closure","producer-raw-manifest"}{if byKind[kind]!=1{return assemblyCatalogCAS{},errAssemblyDebianCapture}}
 // Raw stores are not head-selectable. Their existing no-replace writer must
 // confirm exact bytes before any catalog domain is eligible for publication.
 b:=c.plan.binary;if b==nil||b.spool==nil||b.current(ctx)!=nil{return assemblyCatalogCAS{},errAssemblyDebianCapture}
 for _,raw:=range [][]byte{b.indexConfig,b.indexReceipt,c.binary.receipt}{if len(raw)==0||len(raw)>1<<20{return assemblyCatalogCAS{},errAssemblyDebianCapture};if e:=o.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"producer-raw",assemblyCatalogDigest(raw),int64(len(raw))},raw,true);e!=nil{return assemblyCatalogCAS{},e}}
 if e:=o.writeImmutableAssemblyObject(ctx,b.indexManifestRef,b.indexManifest,false);e!=nil{return assemblyCatalogCAS{},e}
 unlock,e:=o.lock(ctx);if e!=nil{return assemblyCatalogCAS{},e};defer unlock()
 old,e:=o.readCurrent(ctx);if e!=nil{return assemblyCatalogCAS{},e}
 if target!=(assemblyCatalogCAS{})&&old.CAS==target&&old.Source==c.source&&old.State.Policy==c.policy&&equalAssemblyDebianJSON(old.Index,targetIndex){return target,nil}
 if unknown{return assemblyCatalogCAS{},errAssemblyDebianPublicationUncertain}
 if old.CAS!=c.expected||old.Source!=c.source||old.State.Policy!=c.policy||old.State.Revision==math.MaxUint64||c.current(ctx)!=nil{return assemblyCatalogCAS{},errAssemblyCatalogConflict}
 index:=assemblyCatalogIndex{Schema:1,Records:append([]assemblyCatalogRef(nil),old.Index.Records...)}
 for _,record:=range records{
  if e=o.writeImmutableAssemblyObject(ctx,record.ref,record.data,false);e!=nil{return assemblyCatalogCAS{},e}
  present:=false;for _,existing:=range index.Records{present=present||existing==record.ref};if !present{index.Records=append(index.Records,record.ref)}
 }
 sort.Slice(index.Records,func(i,j int)bool{return index.Records[i].Kind+"/"+index.Records[i].Digest<index.Records[j].Kind+"/"+index.Records[j].Digest})
 if !validAssemblyCatalogIndex(index)||c.current(ctx)!=nil{return assemblyCatalogCAS{},errAssemblyDebianCapture}
 sourceRaw,_:=json.Marshal(c.source);indexRaw,_:=json.Marshal(index)
 state:=assemblyInputCatalogState{1,old.State.Revision+1,assemblyCatalogRef{1,"source-descriptor",assemblyCatalogDigest(sourceRaw),int64(len(sourceRaw))},c.policy,assemblyCatalogRef{1,"evidence-index",assemblyCatalogDigest(indexRaw),int64(len(indexRaw))}}
 head,_:=json.Marshal(state);c.mu.Lock();c.targetCAS=assemblyCatalogCAS{state.Revision,assemblyCatalogDigest(head)};c.targetIndex=index;c.mu.Unlock()
 value,err:=liveDebianProducerRuns.Load(c.plan.identity.RunID);if !err||value.(*debianProducerLiveRun).plan!=c.plan{return assemblyCatalogCAS{},errAssemblyDebianCapture}
 run:=value.(*debianProducerLiveRun);run.mu.Lock();actor:=run.actor;run.mu.Unlock()
 return o.publishCatalogHeadLocked(ctx,actor,old,c.source,c.policy,index,&assemblyDebianPublicationPermit{accepted:c})
}
