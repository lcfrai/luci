package api

import (
 "context"
 "encoding/json"
 "path/filepath"
 "strconv"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

type debianProductOwnerReceipt struct {
 Schema string `json:"schema"`
 Identity runtimeinstall.DebianInputProofIdentity `json:"identity"`
 Profile runtimeinstall.BundledDebianInputProofProfile `json:"profile"`
 Started,Completed time.Time
 Children []debianProofChildResult `json:"children"`
 ProjectionRequest assemblyDebianObjectRef `json:"projectionRequest"`
 ProjectionInputs []assemblyDebianObjectRef `json:"projectionInputs"`
}
type debianProductOriginalCompletion struct {
 native *debianProofNativeOwner
 identity runtimeinstall.DebianInputProofIdentity
 profileSHA,receiptSHA string
 children []debianProofChildResult
 pins assemblyDebianProductProfilePins
 projection *debianProductProjectionEvidence
}

// Only the actual concrete Windows owner's original-handle settlement can
// reach this constructor. Paths/JSON alone never construct a completion.
func mintDebianProductRun(ctx context.Context,a *API,p *debianProducerPlan,results []debianProofChildResult,native *debianProofNativeOwner)(*assemblyDebianOwnedRun,error){
 projection:=p.projection
 if p.current(ctx,a)!=nil || !native.positiveCompletion(results)||projection==nil||projection.identity!=p.identity.RunID||projection.checkCurrent(ctx,false)!=nil{return nil,errAssemblyDebianCapture}
 projection.mu.Lock();requestRef:=projection.request;copies:=append([]assemblyDebianObjectRef(nil),projection.copies...);projection.mu.Unlock()
 receipt:=debianProductOwnerReceipt{Schema:runtimeinstall.DebianInputProofProtocol,Identity:p.identity,Profile:p.profile,Started:native.began,Completed:time.Now().UTC(),Children:results,ProjectionRequest:requestRef,ProjectionInputs:copies}
 rawReceipt,e:=json.Marshal(receipt);if e!=nil||len(rawReceipt)>64<<10{return nil,errAssemblyDebianCapture}
 if e=native.writeEvidenceBytes(filepath.Join(p.directory,"native-owner-receipt"),rawReceipt,64<<10);e!=nil{return nil,e}
 // Publish/readback is bounded and no-replace. The live completion binds these
 // exact bytes; a decoded owner receipt cannot be used to mint a capability.
 receiptBytes,e:=readDebianProductProduced(ctx,p.readBudget,native,filepath.Join(p.directory,"native-owner-receipt"),64<<10);if e!=nil||string(receiptBytes)!=string(rawReceipt){return nil,errAssemblyDebianCapture}
 slots:=map[string]assemblyDebianObjectRef{};outputObjects:=[]backend.DocumentImageAssemblyEvidenceObject{}
 inputRoles:=[]string{"policy-source","producer-source-receipt","producer-config","projection-source","dependency-lock","acquisition-manifest","snapshot-source","catalog-source-descriptor"}
 for i:=0;i<3;i++{inputRoles=append(inputRoles,"public-key-"+strconv.Itoa(i))};for i:=0;i<5;i++{inputRoles=append(inputRoles,"signed-release-"+strconv.Itoa(i))}
 for _,role:=range inputRoles{f:=p.files[role];slots[role]=assemblyDebianObjectRef{Role:role,SHA256:f.SHA256,Bytes:f.Bytes}}
 // Reuse the 26 early originals and exact request already owned before child9.
 // Copies stay held by that guard; their refs are reconciled against retained
 // original bytes during capture rather than opening those early paths again.
 projection.mu.Lock();for role,ref:=range projection.refs{if role=="projection-request"||len(role)<16||role[:16]!="projection-copy:"{slots[role]=ref}};projection.mu.Unlock()
 run:=&assemblyDebianOwnedRun{product:&debianProductProofBinding{api:a,identity:p.identity,actor:liveDebianProducerActor(p.identity.RunID)},owner:p.owner,id:p.identity.RunID,actor:liveDebianProducerActor(p.identity.RunID),expected:assemblyCatalogCAS{Revision:p.identity.ExpectedCAS.Revision,Digest:p.identity.ExpectedCAS.Digest},guard:p.projectionToken,inputGuard:p.inputs,objects:slots,phase:"settling-product-evidence"}
 // Retain partial output ownership on failure in the actual live owner.
 live,_:=liveDebianProducerRuns.Load(p.identity.RunID);if live!=nil{lr:=live.(*debianProducerLiveRun);lr.mu.Lock();lr.capturedOwner=run;lr.mu.Unlock()}
 for _,role:=range []string{"stdout-9","stderr-9","producer-output","native-owner-receipt"}{
  path:=filepath.Join(p.directory,role);limit:=int64(128<<10);if role=="producer-output"{limit=256<<10};if role=="native-owner-receipt"{limit=64<<10}
  data,err:=readDebianProductProduced(ctx,p.readBudget,native,path,limit);if err!=nil{return run,err}
  ref:=assemblyDebianObjectRef{Role:role,SHA256:assemblyCatalogDigest(data),Bytes:int64(len(data))};slots[role]=ref;outputObjects=append(outputObjects,backend.DocumentImageAssemblyEvidenceObject{Role:role,Path:path,Bytes:ref.Bytes,Digest:ref.SHA256})
  projection.mu.Lock();projection.refs[role]=ref;projection.lateRoles[role]=true;projection.mu.Unlock()
 }
 if e=projection.acquire(ctx,true,outputObjects);e!=nil{return run,e}
 if e=native.adoptProduced(ctx,projection.late,outputObjects);e!=nil{return run,e}
 if projection.checkCurrent(ctx,true)!=nil||!native.positiveCompletion(results){return run,errAssemblyDebianCapture}
 snapshot,e:=p.owner.readCurrent(ctx);if e!=nil{return run,e};run.source,run.policy=snapshot.Source,snapshot.State.Policy
 for i:=0;i<9;i++{run.commands[i]=append([]string{},p.commands[i]...)}
 run.mu.Lock();if run.phase!="settling-product-evidence"{run.mu.Unlock();return run,errAssemblyDebianCapture};run.phase="settled";run.completion=&assemblyDebianOriginalCompletion{runID:run.id,started:float64(native.began.UnixNano())/1e9,completed:float64(receipt.Completed.UnixNano())/1e9,product:&debianProductOriginalCompletion{native:native,identity:p.identity,profileSHA:p.profile.SHA256,receiptSHA:assemblyCatalogDigest(rawReceipt),children:append([]debianProofChildResult(nil),results...),pins:p.productPins,projection:projection}};run.mu.Unlock()
 return run,nil
}
func captureDebianProductRun(ctx context.Context,run *assemblyDebianOwnedRun)(*assemblyDebianCapturedRun,error){
 if run==nil||run.completion==nil||run.completion.product==nil||run.product==nil||run.current(ctx)!=nil{return nil,errAssemblyDebianCapture}
 completion:=run.completion.product
 if completion.identity!=run.product.identity||!completion.native.positiveCompletion(completion.children)||completion.projection==nil||completion.projection.identity!=run.id||completion.projection.checkCurrent(ctx,true)!=nil{return nil,errAssemblyDebianCapture}
 run.mu.Lock();if run.phase!="settled"{run.mu.Unlock();return nil,errAssemblyDebianCapture};run.phase="capturing";run.mu.Unlock()
 raw:=map[string][]byte{};total:=int64(0)
 for role,ref:=range run.objects{var data []byte;var e error;if assemblyDebianInputSlot(role)||role=="catalog-source-descriptor"{data,e=readDebianProductGuard(ctx,completion.projection.budget,run.inputGuard,role,ref)}else{data,e=run.guard.ReadBounded(ctx,role,max(int64(1),ref.Bytes))};if e!=nil||int64(len(data))!=ref.Bytes||assemblyCatalogDigest(data)!=ref.SHA256{return nil,errAssemblyDebianCapture};total+=ref.Bytes;if total>4<<20{return nil,errAssemblyDebianCapture};raw[role]=data}
 if checkDebianProductNormativeRaw(raw)!=nil||run.source.DependencyLockDigest!=assemblyDebianLockSHA||run.source.InputManifestDigest!=assemblyDebianManifestSHA{return nil,errAssemblyDebianCapture}
 var receipt debianProductOwnerReceipt;var output assemblyDebianRawOutput
 if decodeAssemblyDebianRaw(raw["native-owner-receipt"],&receipt)!=nil||receipt.Schema!=runtimeinstall.DebianInputProofProtocol||receipt.Identity!=completion.identity||receipt.Profile.SHA256!=completion.profileSHA||len(receipt.Children)!=10||assemblyCatalogDigest(raw["native-owner-receipt"])!=completion.receiptSHA||decodeAssemblyDebianRaw(raw["producer-output"],&output)!=nil{return nil,errAssemblyDebianCapture}
 if checkDebianProductProjectionCapture(raw,receipt,completion)!=nil{return nil,errAssemblyDebianCapture}
 // One Go owner receipt remains one receipt. It cannot supply two external
 // diagnostic ownership layers or an exit claim for this API process.
 inner:=assemblyDebianNativeReceipt{}
 for i,c:=range receipt.Children[:9]{
  if c.Ordinal!=i||!c.OriginalHandleOwned||!c.ExitConfirmed||!c.JobEmpty||!c.HandlesClosed||!c.CaptureSettled||!c.Admission.Known||c.Admission.Physical<8<<30||c.Admission.Commit<8<<30||c.Admission.CPU>=50{return nil,errAssemblyDebianCapture}
  child:=assemblyDebianNativeChild{Index:i,PID:int(c.PID),Began:float64(c.Started.UnixNano())/1e9,Ended:float64(c.Ended.UnixNano())/1e9,Exit:int(c.Exit)}
  if i<4&&c.Exit!=0||i>=4&&c.Exit!=0&&c.Exit!=2{return nil,errAssemblyDebianCapture};inner.Children=append(inner.Children,child)
 }
 if output.Schema!=assemblyDebianProductRawSchema||output.TrustPolicySHA!=assemblyDebianPolicySourceSHA||len(output.Releases)!=5||output.MonotonicPolicyState!="not-updated-by-this-read-only-batch"||output.Consumer!="server-captured AssemblyInputClosureRef; mandatory ValidateCurrent"||!equalAssemblyDebianJSON(output.Missing,[]string{"index-paragraphs","package-source-bindings","installed-manifest","source-retention","notice-closure","output-proof"}){return nil,errAssemblyDebianCapture}
 if run.current(ctx)!=nil{return nil,errAssemblyDebianCapture};run.mu.Lock();defer run.mu.Unlock();if run.phase!="capturing"{return nil,errAssemblyDebianCapture};run.phase="captured";return &assemblyDebianCapturedRun{run:run,raw:raw,inner:inner,output:output},nil
}
func liveDebianProducerActor(runID string)management.ExecutionActorRef{v,ok:=liveDebianProducerRuns.Load(runID);if !ok{return management.ExecutionActorRef{}};r:=v.(*debianProducerLiveRun);r.mu.Lock();defer r.mu.Unlock();return r.actor}
func checkDebianProductNormativeRaw(raw map[string][]byte)error{
 snapshot:=expectedAssemblyDebianHistoricalAnchor().SnapshotSourceRef
 if assemblyCatalogDigest(raw["dependency-lock"])!=assemblyDebianLockSHA||assemblyCatalogDigest(raw["acquisition-manifest"])!=assemblyDebianManifestSHA||int64(len(raw["snapshot-source"]))!=snapshot.Bytes||assemblyCatalogDigest(raw["snapshot-source"])!=snapshot.SHA256{return errAssemblyDebianCapture};return nil
}

// Reconcile copies to the retained original facts, exact request and live owner
// clock. Matching digest-shaped JSON alone cannot create this completion.
func checkDebianProductProjectionCapture(raw map[string][]byte,receipt debianProductOwnerReceipt,completion *debianProductOriginalCompletion)error{
 if completion==nil||completion.native==nil||completion.projection==nil||len(receipt.Children)!=10||len(completion.children)!=10||!equalAssemblyDebianJSON(receipt.Children,completion.children){return errAssemblyDebianCapture}
 e:=completion.projection;e.mu.Lock();requestRef:=e.request;copies:=append([]assemblyDebianObjectRef(nil),e.copies...);valid:=e.phase=="complete"&&!e.retiring&&!e.moving&&!e.closing;e.mu.Unlock()
 if !valid||receipt.ProjectionRequest!=requestRef||!equalAssemblyDebianJSON(receipt.ProjectionInputs,copies)||len(copies)!=13||requestRef.Role!="projection-request"||requestRef.Bytes<=0||requestRef.Bytes>64<<10||int64(len(raw["projection-request"]))!=requestRef.Bytes||assemblyCatalogDigest(raw["projection-request"])!=requestRef.SHA256{return errAssemblyDebianCapture}
 var request debianProductProjectionRequest;if decodeAssemblyDebianRaw(raw["projection-request"],&request)!=nil{return errAssemblyDebianCapture}
 expected,err:=buildDebianProductProjectionRequest(receipt.Children[:9],copies,request.ObservedUnix)
 if err!=nil||!equalAssemblyDebianJSON(request,expected)||!receipt.Started.Equal(completion.native.began)||request.ObservedUnix<receipt.Started.Unix()||request.ObservedUnix>receipt.Completed.Unix()||receipt.Children[9].Started.After(receipt.Completed){return errAssemblyDebianCapture}
 sourceRoles:=[]string{"producer-config","snapshot-source","stdout-3"};for i:=0;i<5;i++{sourceRoles=append(sourceRoles,"stdout-"+strconv.Itoa(4+i))};for i:=0;i<5;i++{sourceRoles=append(sourceRoles,"plaintext-"+strconv.Itoa(i))}
 for i,role:=range sourceRoles{data,exists:=raw[role];if !exists||copies[i].Role!=debianProjectionCopyNames()[i].role||int64(len(data))!=copies[i].Bytes||assemblyCatalogDigest(data)!=copies[i].SHA256{return errAssemblyDebianCapture}}
 return nil
}
