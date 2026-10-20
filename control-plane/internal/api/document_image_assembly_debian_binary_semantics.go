package api

import (
 "context"
 "encoding/json"
 "time"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// This is the semantic handoff of the SAME retained raw spool. It contains no
// caller-selected reader, path, native completion flag or mutable payload map.
// A parsed frame is never enough: the original child and all of its aliases
// must have positively settled before this constructor reads any raw evidence.
type debianBinarySemanticCapture struct {
 spool *debianBinarySpoolSet
 plan *debianProducerPlan
 child debianProofChildResult
 expected debianBinaryRawExpected
 indexes assemblyDebianSignedIndexBindingsV1
 root assemblyDebianBinariesV1
 parts [11]assemblyDebianBinaryPartV1
 controls [174]assemblyDebianBinaryFieldsV1
 claimBytes [174][]byte
 receipt, manifest []byte
}

func(s *debianBinarySpoolSet)readSemanticObject(ctx context.Context,slot int)([]byte,error){
 if s==nil||slot<0||slot>=25{return nil,errAssemblyDebianCapture}
 s.mu.Lock();guard:=s.outputGuard;object:=s.objects[slot];ready:=s.finished&&!s.busy&&!s.retiring&&!s.failed;if ready{s.busy=true};s.mu.Unlock()
 if ready{defer func(){s.mu.Lock();s.busy=false;s.mu.Unlock()}()}
 if !ready||guard==nil||!s.current(ctx)||object.Role!=debianBinarySpoolRole(slot)||object.Bytes<0{return nil,errAssemblyDebianCapture}
 limit:=int64(64<<10);if slot<22&&slot%2==0{limit=4<<20}
 if object.Bytes>limit||object.Bytes==0&&slot!=23{return nil,errAssemblyDebianCapture}
 data,err:=guard.ReadBounded(ctx,object.Role,max(int64(1),object.Bytes))
 if err!=nil||int64(len(data))!=object.Bytes||assemblyCatalogDigest(data)!=object.SHA256||guard.CheckCurrent(ctx)!=nil||!s.current(ctx){return nil,errAssemblyDebianCapture}
 return data,nil
}

func captureDebianBinarySemantics(ctx context.Context,a *API,p *debianProducerPlan,s *debianBinarySpoolSet,child debianProofChildResult,indexes assemblyDebianSignedIndexBindingsV1)(*debianBinarySemanticCapture,error){
 if ctx==nil||ctx.Err()!=nil||a==nil||p==nil||s==nil||s.owner==nil||p.current(ctx,a)!=nil||!s.owner.positiveBinaryCompletion(child)||s.decoder.state==nil{return nil,errAssemblyDebianCapture}
 // Retirement invalidates views but preserves this immutable expected copy.
 state:=s.decoder.state;state.mu.Lock();expected:=state.expected;rawRefs:=state.refs;readCalls,stdoutDebit,stderrDebit:=state.calls,state.stdoutDebit,state.stderrDebit;complete:=state.retired&&!state.failed&&state.frames==23&&state.stdoutEOF&&state.stderrEOF&&state.pending==nil;state.mu.Unlock()
 indexRef,err:=assemblyDebianRecordRef("signed-index-bindings",indexes)
 if err!=nil||!complete||expected.OperationID!=p.identity.RunID||expected.SignedIndexes!=indexRef{return nil,errAssemblyDebianCapture}
 capture:=&debianBinarySemanticCapture{spool:s,plan:p,child:child,expected:expected,indexes:indexes}
 rawManifest,err:=s.readSemanticObject(ctx,24);if err!=nil{return nil,err}
 var manifest debianBinarySpoolManifest
 if decodeAssemblyDebianRaw(rawManifest,&manifest)!=nil||manifest.Schema!="luci-binary-raw-spool-v1"||manifest.Meaning!="raw-only-not-native-semantic-or-adoption-proof"||manifest.Stderr!=s.objects[23]||manifest.Frames!=rawRefs||manifest.ReadAttempts!=readCalls||manifest.StdoutDebit!=stdoutDebit||manifest.StderrDebit!=stderrDebit{return nil,errAssemblyDebianCapture}
 if e:=p.owner.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"binary-raw",assemblyCatalogDigest(rawManifest),int64(len(rawManifest))},rawManifest,true);e!=nil{return nil,e}
 receipt,err:=json.Marshal(struct{Schema string `json:"schema"`;Identity runtimeinstall.DebianInputProofIdentity `json:"identity"`;Child debianProofChildResult `json:"child"`;RawManifest assemblyDebianObjectRef `json:"rawManifest"`}{"luci-binary-original-owner-v1",p.identity,child,assemblyDebianObjectRef{"binary-raw-manifest",assemblyCatalogDigest(rawManifest),int64(len(rawManifest))}})
 if err!=nil||len(receipt)>64<<10{return nil,errAssemblyDebianCapture};capture.receipt=receipt
 var summary debianBinaryRawSummary
 data,err:=s.readSemanticObject(ctx,22);summaryCheck:=debianBinaryRawState{expected:expected,refs:rawRefs};if err!=nil||summaryCheck.checkSummary(data)!=nil||decodeAssemblyDebianRaw(data,&summary)!=nil{return nil,errAssemblyDebianCapture}
 if summary.OperationID!=expected.OperationID||summary.LoanID!=expected.LoanID||summary.SourceBindingSHA256!=expected.SourceBindingSHA256||summary.SelectionSHA256!=expected.SelectionSHA256||summary.SignedIndexes!=indexRef||summary.SeekCalls!=359||len(summary.RawOutputs)!=22{return nil,errAssemblyDebianCapture}
 if e:=p.owner.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"binary-raw",assemblyCatalogDigest(data),int64(len(data))},data,true);e!=nil{return nil,e}
 // All eleven bundles share the existing finite16MiB aggregate. Only one
 // byte bundle exists at once; decoded bounded fields are retained for the
 // existing cross-part coverage validator, never a second archive decoder.
 bundles:=make([]assemblyDebianBinaryBundle,11)
 refs:=make([]assemblyDebianBinaryPartRefV1,11)
 total:=int64(0)
 controls:=0
 for part:=0;part<11;part++{
  slot:=part*2
  raw,readErr:=s.readSemanticObject(ctx,slot);if readErr!=nil{return nil,readErr}
  ref:=assemblyDebianObjectRef{"binary-binding-bundle",assemblyCatalogDigest(raw),int64(len(raw))}
  if ref.Bytes>assemblyDebianBinaryEvidenceLimit-total{return nil,errAssemblyDebianCapture};total+=ref.Bytes
  bundle,decodeErr:=decodeAssemblyDebianBinaryBundle(raw,ref,part);if decodeErr!=nil{return nil,decodeErr};bundles[part]=bundle
  for _,frame:=range bundle.frames{if frame.kind==3{if frame.claims==nil||controls>=174||len(frame.rawClaims)==0||int64(len(frame.rawClaims))!=frame.span.Bytes||assemblyCatalogDigest(frame.rawClaims)!=frame.span.SHA256{return nil,errAssemblyDebianCapture};capture.controls[controls]=frame.claims.Control;capture.claimBytes[controls]=frame.rawClaims;controls++}}
  // Retain the exact raw evidence during its existing semantic read. A later
  // adoption never rereads all bundles and silently exhausts/reset the same
  // output guard's finite lifetime read allowance. Raw storage is not adoption.
  if e:=p.owner.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"binary-raw",ref.SHA256,ref.Bytes},raw,true);e!=nil{return nil,e}
  raw=nil
  body,readErr:=s.readSemanticObject(ctx,slot+1);if readErr!=nil||decodeAssemblyDebianRaw(body,&capture.parts[part])!=nil{return nil,errAssemblyDebianCapture}
  value:=capture.parts[part];canonical,e:=json.Marshal(value)
  if e!=nil||string(body)!=string(canonical)||value.OperationID!=p.identity.RunID{return nil,errAssemblyDebianCapture}
  refs[part]=assemblyDebianBinaryPartRefV1{part,value.FirstLockOrdinal,len(value.Bindings),assemblyDebianObjectRef{"binary-binding-part",assemblyCatalogDigest(canonical),int64(len(canonical))}}
  if e:=p.owner.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"binary-raw",assemblyCatalogDigest(body),int64(len(body))},body,true);e!=nil{return nil,e}
  for _,n:=range []int{slot,slot+1}{o:=s.objects[n];r:=summary.RawOutputs[n];if manifest.Frames[n]!=r||r.Bytes!=int(o.Bytes)||r.SHA256!=o.SHA256||int(r.Part)!=part||int(r.Kind)!=1+n%2{return nil,errAssemblyDebianCapture}}
 }
 // The manifest binds actual raw refs and owner receipt; it cannot assert
 // dependency, source archive, installed inventory or distribution closure.
 capture.manifest,err=json.Marshal(struct{Schema int `json:"schema"`;Receipt assemblyDebianObjectRef `json:"receipt"`;Raw assemblyDebianObjectRef `json:"raw"`;Parts []assemblyDebianBinaryPartRefV1 `json:"parts"`}{1,assemblyDebianObjectRef{"binary-binding-receipt",assemblyCatalogDigest(receipt),int64(len(receipt))},assemblyDebianObjectRef{"binary-raw-manifest",assemblyCatalogDigest(rawManifest),int64(len(rawManifest))},refs})
 if err!=nil||len(capture.manifest)>assemblyCatalogMetadataLimit{return nil,errAssemblyDebianCapture}
 if e:=p.owner.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"evidence-index",assemblyCatalogDigest(capture.manifest),int64(len(capture.manifest))},capture.manifest,false);e!=nil{return nil,e}
 capture.root=assemblyDebianBinariesV1{Schema:1,RecordType:"luci-debian-binary-bindings-v1",ObservationState:"complete-observations-not-adopted",SignedIndexes:indexRef,DependencyLockSHA256:expected.Selection.DependencyLockSHA256,AcquisitionManifestSHA256:expected.Selection.AcquisitionManifestSHA256,Producer:assemblyDebianObjectRef{"binary-binding-receipt",assemblyCatalogDigest(receipt),int64(len(receipt))},RetainedManifest:assemblyCatalogRef{1,"evidence-index",assemblyCatalogDigest(capture.manifest),int64(len(capture.manifest))},Parts:refs,DecodedIndexes:summary.DecodedIndexes,Counters:assemblyDebianBinaryCountersV1{summary.Counters.CompressedReadBytes,summary.Counters.ReadCalls,summary.Counters.ExpandedBytes,summary.Counters.Paragraphs},PackageCount:summary.PackageCount,PackageBytes:summary.PackageBytes,SourceArchiveBinding:summary.SourceArchiveBinding,DependencyClosure:summary.DependencyClosure,ExpectedInstalledFiles:summary.ExpectedInstalledFiles,ObservedInstalledOutput:summary.ObservedInstalledOutput,ServerEligibility:summary.ServerEligibility}
 if controls!=174||checkAssemblyDebianBinaryCoverage(expected.Selection,indexes,capture.root,capture.parts[:],bundles)!=nil||p.current(ctx,a)!=nil||!s.owner.positiveBinaryCompletion(child)||!time.Now().Before(s.owner.workLimit()){return nil,errAssemblyDebianCapture}
 return capture,nil
}
