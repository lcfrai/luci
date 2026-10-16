package api

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strconv"
 "strings"
 "sync"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
 "github.com/lcfrai/luci/control-plane/internal/management"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

type debianProducerPlan struct {
 identity runtimeinstall.DebianInputProofIdentity
 profile runtimeinstall.BundledDebianInputProofProfile
 directory string
 environment []uint16
 inputs backend.DocumentImageAssemblyEvidenceGuard
 paths map[string]string
 files map[string]runtimeinstall.DebianInputProofFile
 commands [10][]string
 owner *assemblyInputOwner
 productPins assemblyDebianProductProfilePins
}
// Closed contract for the separately allocated fixed-file product entry. The
// held-original copy/guard lifetime seam is not yet implemented by the existing
// paths-map writer below; pre-reservation eligibility therefore remains refused.
type debianProductProjectionChild struct {Ordinal int `json:"ordinal"`;Exit uint32 `json:"exit"`}
type debianProductProjectionRequest struct {
 Schema string `json:"schema"`
 Protocol string `json:"protocol"`
 ObservedUnix int64 `json:"observedUnix"`
 Children [9]debianProductProjectionChild `json:"children"`
 Files []assemblyDebianObjectRef `json:"files"`
}
type debianProducerLiveRun struct {
 mu sync.Mutex
 identity runtimeinstall.DebianInputProofIdentity
 api *API
 actor management.ExecutionActorRef
 plan *debianProducerPlan
 native *debianProofNativeOwner
 outcome runtimeinstall.DebianInputProofOutcome
	 capturedOwner *assemblyDebianOwnedRun
	 candidate *assemblyDebianDomainCandidate
	 resourceSettlementKnown bool
 err error
 done chan struct{}
}
type debianProductProofBinding struct {api *API;identity runtimeinstall.DebianInputProofIdentity;actor management.ExecutionActorRef}
func(b *debianProductProofBinding)current(ctx context.Context)error{if b==nil||b.api==nil||b.api.deps.Product==nil{return runtimeinstall.ErrInputProofOwnership};if err:=b.api.deps.Product.CheckRuntimeInputProofCurrent(ctx,b.identity);err!=nil{return err};authority,err:=b.api.authorizeRuntimeInstaller(ctx,b.actor);if err!=nil{return err};if !b.actor.Local&&(authority.Policy.CheckMemoryMiB<192||authority.Policy.CheckCPUMillis<1000){return errCheckResourcePolicy};return nil}
// Process-local ownership only. Durable metadata cannot populate this map or
// recreate a native capability. Unknown live creators/guards remain reachable.
var liveDebianProducerRuns sync.Map

func canonicalDebianProducerProfile(p runtimeinstall.BundledDebianInputProofProfile)([]byte,error){
 return json.Marshal(struct{Schema int `json:"schema"`;ID string `json:"id"`;Protocol string `json:"protocol"`;Source string `json:"sourceSHA256"`;Files []runtimeinstall.DebianInputProofFile `json:"files"`}{1,p.ID,p.Protocol,p.SourceSHA256,p.Files})
}
func validateDebianProducerProfile(p runtimeinstall.BundledDebianInputProofProfile)(map[string]runtimeinstall.DebianInputProofFile,error){
 return validateDebianProducerProfileShape(p,false)
}
func validateDebianProductProfile(p runtimeinstall.BundledDebianInputProofProfile)(map[string]runtimeinstall.DebianInputProofFile,error){
 files,e:=validateDebianProducerProfileShape(p,true);if e!=nil{return nil,e};if _,e=debianProductProfilePins(p,files);e!=nil{return nil,e};return files,nil
}
func validateDebianProducerProfileShape(p runtimeinstall.BundledDebianInputProofProfile,product bool)(map[string]runtimeinstall.DebianInputProofFile,error){
 if !checkLogicalID(p.ID) || p.Protocol!=runtimeinstall.DebianInputProofProtocol || !filepath.IsAbs(p.Root) || len(p.Files)==0 || len(p.Files)>255 || !assemblyCatalogHex(p.SHA256) || !assemblyCatalogHex(p.SourceSHA256){return nil,runtimeinstall.ErrInputProofUnavailable}
 canonical,err:=canonicalDebianProducerProfile(p);if err!=nil || len(canonical)>64<<10 || assemblyCatalogDigest(canonical)!=p.SHA256{return nil,runtimeinstall.ErrInputProofUnavailable}
 roles,paths:=map[string]runtimeinstall.DebianInputProofFile{},map[string]bool{};total:=int64(len(canonical))
 required:=[]string{"cp-python","gpg","gpgv","projection-entry","projection-source","producer-source-receipt","producer-config","policy-source","dependency-lock","acquisition-manifest","snapshot-source","catalog-source-descriptor"}
 for i:=0;i<3;i++{required=append(required,"public-key-"+strconv.Itoa(i))};for i:=0;i<5;i++{required=append(required,"signed-release-"+strconv.Itoa(i))}
 requiredRoles:=map[string]bool{};for _,role:=range required{requiredRoles[role]=true}
 previous:=""
 for _,f:=range p.Files{
  relative:=filepath.FromSlash(f.RelativePath)
  if f.Role<=previous || len(f.Role)>200 || !requiredRoles[f.Role]&&f.Role!="support:"+f.RelativePath || relative=="" || relative=="." || strings.Contains(f.RelativePath,"\\") || filepath.IsAbs(relative) || filepath.VolumeName(relative)!="" || strings.Contains(relative,":") || filepath.Clean(relative)!=relative || relative==".." || strings.HasPrefix(relative,".."+string(filepath.Separator)) || f.Bytes<=0 || f.Bytes>16<<20 || !assemblyCatalogHex(f.SHA256) || paths[strings.ToLower(relative)]{return nil,runtimeinstall.ErrInputProofUnavailable}
  roles[f.Role]=f;paths[strings.ToLower(relative)]=true;previous=f.Role;total+=f.Bytes;if total>64<<20{return nil,runtimeinstall.ErrInputProofUnavailable}
 }
 for _,role:=range required{if _,ok:=roles[role];!ok{return nil,runtimeinstall.ErrInputProofUnavailable}}
 if roles["projection-source"].SHA256!=assemblyDebianProjectionSHA || roles["producer-source-receipt"].SHA256!=p.SourceSHA256{return nil,runtimeinstall.ErrInputProofUnavailable}
 if !product&&(roles["gpg"].SHA256!="a5140c85353e8399da8d6bd7e3741524cd76a4e69be88af12042b1c9ef022984" || roles["gpgv"].SHA256!="f4d13204d77fdf63c02b0e6742230f83a833128c28f7b715709c2c63a96c427b" || roles["producer-config"].SHA256!=assemblyDebianConfigSHA){return nil,runtimeinstall.ErrInputProofUnavailable}
 if product&&(p.SourceSHA256==assemblyDebianSourceReceiptSHA||roles["producer-config"].SHA256==assemblyDebianConfigSHA){return nil,runtimeinstall.ErrInputProofUnavailable}
 return roles,nil
}
func (p *debianProducerPlan) current(ctx context.Context,a *API)error{
 if p==nil || p.inputs==nil || a.deps.Product==nil || p.owner==nil{return runtimeinstall.ErrInputProofOwnership}
 if err:=p.inputs.CheckCurrent(ctx);err!=nil{return err};if err:=a.deps.Product.CheckRuntimeInputProofCurrent(ctx,p.identity);err!=nil{return err}
 // Actor comes from the exact live request, never the profile manifest.
 value,ok:=liveDebianProducerRuns.Load(p.identity.RunID);if !ok{return runtimeinstall.ErrInputProofOwnership};run:=value.(*debianProducerLiveRun);run.mu.Lock();actor:=run.actor;run.mu.Unlock()
 if err:=(&debianProductProofBinding{api:a,identity:p.identity,actor:actor}).current(ctx);err!=nil{return err}
 snap,err:=p.owner.readCurrent(ctx);sourceFile:=p.files["catalog-source-descriptor"];if err!=nil || snap.CAS.Revision!=p.identity.ExpectedCAS.Revision || snap.CAS.Digest!=p.identity.ExpectedCAS.Digest || snap.State.SourceDescriptor.Digest!=sourceFile.SHA256 || snap.State.SourceDescriptor.Bytes!=sourceFile.Bytes{return errAssemblyCatalogConflict}
 return p.inputs.CheckCurrent(ctx)
}
func (a *API) proveBundledDebianInputs(ctx context.Context,req runtimeinstall.DebianInputProofRequest,publish runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){
 out:=runtimeinstall.DebianInputProofOutcome{Identity:req.Identity,Phase:"blocked",CleanupConfirmed:true,ReasonCode:"input_proof_unavailable",Message:runtimeinstall.ErrInputProofUnavailable.Error()}
 if publish==nil || a.deps.Product==nil || a.execution==nil || a.deps.ResourceSnapshot==nil || a.deps.Store==nil || !runtimeinstall.ValidInputProofIdentity(req.Identity){return out,runtimeinstall.ErrInputProofOwnership}
 authority,err:=a.authorizeRuntimeInstaller(ctx,req.Actor);if err!=nil{return out,err};if !req.Actor.Local&&(authority.Policy.CheckMemoryMiB<192||authority.Policy.CheckCPUMillis<1000){return out,errCheckResourcePolicy}
 profile,err:=a.deps.Product.RuntimeInputProofProfile(ctx,req.Identity);if err!=nil{return out,err}
 files,err:=debianProducerProfileBeforeReservation(profile);if err!=nil{return out,err}
 if !debianProducerPlatformSupported(){return out,runtimeinstall.ErrInputProofUnavailable}
 pid,creation,err:=debianProducerHostIdentity();if err!=nil{return out,err};req.Identity.OwnerPID,req.Identity.OwnerCreationTime=pid,creation
 run:=&debianProducerLiveRun{api:a,identity:req.Identity,actor:req.Actor,done:make(chan struct{}),outcome:out}
 if _,loaded:=liveDebianProducerRuns.LoadOrStore(req.Identity.RunID,run);loaded{return out,runtimeinstall.ErrInputProofOwnership}
 // The retained worker owns partial acquisition and late native returns. A
 // request deadline cannot discard its original handles or trigger a retry.
 go run.execute(ctx,profile,files,publish)
 timer:=time.NewTimer(60*time.Second);defer timer.Stop()
 select{case<-run.done:run.mu.Lock();out,err=run.outcome,run.err;run.mu.Unlock();return out,err
 case<-ctx.Done():run.retire();case<-timer.C:run.retire()}
 run.mu.Lock();out=run.outcome;candidate:=run.candidate;out.Phase="unknown";out.CleanupConfirmed=false;out.Message="The exact native producer or its original byte owners remain unsettled; no retry was started.";run.mu.Unlock()
 if candidate!=nil{candidate.mu.Lock();out.PublicationUnknown=out.PublicationUnknown||candidate.publicationUnknown||candidate.publicationRetired||candidate.publication!=nil&&candidate.publication.inFlight;candidate.mu.Unlock()}
 return out,errors.New("native input proof settlement unknown")
}
func(r *debianProducerLiveRun) retire(){r.mu.Lock();native:=r.native;r.mu.Unlock();if native!=nil{native.retire()}}
func(r *debianProducerLiveRun) execute(ctx context.Context,profile runtimeinstall.BundledDebianInputProofProfile,files map[string]runtimeinstall.DebianInputProofFile,publish runtimeinstall.PublishDebianInputProof){
 send:=func(out runtimeinstall.DebianInputProofOutcome)error{r.mu.Lock();r.outcome,r.identity=out,out.Identity;r.mu.Unlock();return publish(out)}
 var runErr error;var reserved bool
 out:=runtimeinstall.DebianInputProofOutcome{Identity:r.identity,Phase:"preparing",Message:"Validating the application-pinned producer profile."}
 selected,e:=debianProductProfilePins(profile,files);if e!=nil{out.Phase="blocked";out.CleanupConfirmed=true;_ = send(out);r.mu.Lock();r.outcome,r.err=out,e;r.mu.Unlock();close(r.done);liveDebianProducerRuns.Delete(r.identity.RunID);return}
 native:=newDebianProofNativeOwner(ctx);r.mu.Lock();r.native=native;r.outcome=out;r.mu.Unlock()
 workCtx,cancelWork:=context.WithDeadline(ctx,native.workLimit());defer cancelWork();ctx=workCtx
 var candidate *assemblyDebianDomainCandidate
 var ownedRun *assemblyDebianOwnedRun
 plan:=&debianProducerPlan{identity:r.identity,profile:profile,files:files,paths:map[string]string{},productPins:selected}
 r.mu.Lock();r.plan=plan;r.mu.Unlock()
 defer func(){
  if runErr!=nil{native.retire()}
  settled:=native.settle()
  if candidate!=nil{if e:=candidate.retire();e!=nil{settled=false}}else if ownedRun!=nil{if e:=ownedRun.retire();e!=nil{settled=false}}else if plan.inputs!=nil{if e:=plan.inputs.Close();e!=nil{settled=false}}
  out.Identity=plan.identity;out.CleanupConfirmed=settled;out.ResourcesHeld=reserved&&!settled
  r.mu.Lock();r.resourceSettlementKnown=settled;r.mu.Unlock()
  if reserved&&settled{cleanupCtx,cancel:=context.WithTimeout(context.Background(),2*time.Second);if e:=r.api.execution.advanceDebianInputProof(cleanupCtx,plan.identity,"finished",nil);e!=nil{out.ResourcesHeld=true;out.CleanupConfirmed=false;runErr=e};cancel()}
  if runErr!=nil{out.Phase="failed";out.ReasonCode="input_proof_incomplete";out.Message="Input proof did not complete; retained resources and uncertain publication must be reconciled."}
  if !out.CleanupConfirmed || out.PublicationUnknown{out.Phase="unknown"}
  if e:=send(out);e!=nil{out.Phase="unknown";out.PublicationUnknown=true;runErr=e}
  r.mu.Lock();r.outcome,r.err=out,runErr;r.mu.Unlock();close(r.done)
  if out.CleanupConfirmed&&!out.ResourcesHeld&&!out.PublicationUnknown{liveDebianProducerRuns.Delete(plan.identity.RunID)}
 }()
 owner,e:=r.api.documentImageAssemblyInputOwner();if e!=nil{runErr=e;return};plan.owner=owner
 root,e:=r.api.execution.prepareExecutionDirectory();if e!=nil{runErr=e;return}
 plan.directory=filepath.Join(root,"debian-proof-"+plan.identity.RunID)
 if e=os.Mkdir(plan.directory,0700);e!=nil{runErr=e;return}
 objects:=make([]backend.DocumentImageAssemblyEvidenceObject,0,len(files)+1)
 canonical,_:=canonicalDebianProducerProfile(profile)
 objects=append(objects,backend.DocumentImageAssemblyEvidenceObject{Role:"profile-descriptor",Path:filepath.Join(profile.Root,"profile.json"),Bytes:int64(len(canonical)),Digest:profile.SHA256})
 for _,f:=range profile.Files{path:=filepath.Join(profile.Root,filepath.FromSlash(f.RelativePath));plan.paths[f.Role]=path;objects=append(objects,backend.DocumentImageAssemblyEvidenceObject{Role:f.Role,Path:path,Bytes:f.Bytes,Digest:f.SHA256})}
 plan.inputs,e=dockerbe.AcquireDocumentImageAssemblyEvidence(ctx,objects);if e!=nil{runErr=e;return}
 read:=func(role string)([]byte,error){f:=files[role];if f.Bytes<=0||f.Bytes>1<<20{return nil,runtimeinstall.ErrInputProofUnavailable};return plan.inputs.ReadBounded(ctx,role,f.Bytes)}
 config,e:=read("producer-config");if e!=nil{runErr=e;return};policy,e:=read("policy-source");if e!=nil{runErr=e;return}
 // Existing domain pins are mandatory. A future packaged profile needs an
 // independently reviewed closed pin seam; digest-shaped files do not bypass it.
 if _,e=normalizeAssemblyDebianProductPolicy(policy,config,plan.productPins,"product-profile-v1");e!=nil{runErr=runtimeinstall.ErrInputProofUnavailable;return}
 snap,e:=owner.readCurrent(ctx);if e!=nil{runErr=e;return}
 sourceBytes,e:=read("catalog-source-descriptor");if e!=nil{runErr=e;return}
 if checkDebianProducerSourceDescriptor(sourceBytes,files["catalog-source-descriptor"],snap)!=nil{runErr=errAssemblyCatalogConflict;return}
 plan.identity.ExpectedCAS=runtimeinstall.InputProofCAS{Revision:snap.CAS.Revision,Digest:snap.CAS.Digest}
 out.Identity=plan.identity;out.Phase="preparing";if e=send(out);e!=nil{runErr=e;return}
 if e=plan.current(ctx,r.api);e!=nil{runErr=e;return}
 plan.environment,e=debianProofEnvironment(filepath.Join(plan.directory,"home"));if e!=nil{runErr=e;return};if e=os.Mkdir(filepath.Join(plan.directory,"home"),0700);e!=nil{runErr=e;return}
 plan.commands=debianProducerCommands(plan)
 scheduler,e:=r.api.deps.Product.SwarmSchedulerPolicy();if e!=nil{runErr=e;return}
 row:=debianInputProofExecutionLease{Scope:r.api.execution.scope,ID:plan.identity.AllocationID,Identity:plan.identity,Actor:r.actor,Resources:backend.Resources{MemoryMiB:192,CPUMillis:1000},State:"reserved",DiskBytes:70<<20,CreatedAt:native.began,WholeDeadline:native.wholeDeadline}
 reserved=true
 if e=r.api.execution.reserveDebianInputProof(ctx,row,scheduler);e!=nil{
  // No native Create follows an uncertain reservation ACK. Exact readback
  // distinguishes a retained claim from positively absent metadata; no retry.
  checkCtx,cancel:=context.WithTimeout(context.Background(),100*time.Millisecond);ledger,re:=r.api.execution.readReservationLedger(checkCtx);cancel()
  if re==nil{reserved=false;for _,retained:=range ledger.InputProofs{if retained.Scope==row.Scope&&retained.ID==row.ID{if retained.Identity!=row.Identity{out.PublicationUnknown=true};reserved=true}}}else{out.PublicationUnknown=true}
  runErr=e;return
 }
 out.Phase="reserved";out.ResourcesHeld=true;if e=send(out);e!=nil{runErr=e;return}
 results:=make([]debianProofChildResult,0,10);logBytes:=0
 for i,argv:=range plan.commands{
  if i==9{if e=writeDebianProjectionRequest(native,plan,results);e!=nil{runErr=e;return}}
  child,ce:=native.child(ctx,r.api,plan,i,argv);results=append(results,child);logBytes+=len(child.Stdout)+len(child.Stderr)
  if ce!=nil||!child.OriginalHandleOwned||!child.ExitConfirmed||!child.JobEmpty||!child.HandlesClosed||!child.CaptureSettled||logBytes>256<<10||i<4&&child.Exit!=0||i>=4&&i<9&&child.Exit!=0&&child.Exit!=2||i==9&&child.Exit!=0{runErr=errors.New("fixed native stage incomplete");return}
  if e=native.writeBytes(filepath.Join(plan.directory,fmt.Sprintf("stdout-%d",i)),child.Stdout,128<<10);e!=nil{runErr=e;return};if e=native.writeBytes(filepath.Join(plan.directory,fmt.Sprintf("stderr-%d",i)),child.Stderr,128<<10);e!=nil{runErr=e;return}
 }
 // Original process/Job/PDH/capture completion precedes the private capability.
 if !native.settle(){runErr=errors.New("native owner did not positively settle");return}
 ownedRun,e=mintDebianProductRun(ctx,r.api,plan,results,native);if e!=nil{runErr=e;return}
 captured,e:=captureDebianProductRun(ctx,ownedRun);if e!=nil{runErr=e;return}
 candidate,e=newAssemblyDebianProductDomains(ctx,owner,captured,time.Now().Unix());if e!=nil{runErr=e;return}
 r.mu.Lock();r.candidate=candidate;r.mu.Unlock()
 adopted,e:=owner.adoptDebianDomains(ctx,candidate);if e!=nil{candidate.mu.Lock();out.PublicationUnknown=candidate.publicationUnknown||candidate.publicationRetired;candidate.mu.Unlock();runErr=e;return}
 out.Phase="facts-recorded";out.PublishedCAS=runtimeinstall.InputProofCAS{Revision:adopted.Revision,Digest:adopted.Digest};out.Message="Three Debian input fact domains were retained. Full dependency closure and document runtime are still unavailable."
 for _,object:=range candidate.records{out.Domains=append(out.Domains,runtimeinstall.InputProofDomainRef{Kind:object.ref.Kind,SHA256:object.ref.Digest,Bytes:object.ref.Bytes})}
 candidate.mu.Lock();rawManifest:=candidate.rawManifestRef;candidate.mu.Unlock();if rawManifest.Kind!="producer-raw-manifest"{runErr=errAssemblyDebianCapture;out.PublicationUnknown=true;return};out.Domains=append(out.Domains,runtimeinstall.InputProofDomainRef{Kind:rawManifest.Kind,SHA256:rawManifest.Digest,Bytes:rawManifest.Bytes})
}
// A diagnostic profile cannot be relabelled as this original-Go-owner protocol.
// This runs before live ownership, native observation, or ledger reservation.
// New packaged pins need an explicit versioned core transition; structural
// validity or an application registry entry alone cannot supply eligibility.
func debianProducerProfileBeforeReservation(p runtimeinstall.BundledDebianInputProofProfile)(map[string]runtimeinstall.DebianInputProofFile,error){
 if p.SourceSHA256=="13a80135f8b3b6392d8dc69df5f441a34eae0aa9592cc94dc6710ef2e061d1e0"{return nil,runtimeinstall.ErrInputProofUnavailable}
 for _,f:=range p.Files{if f.Role=="producer-config"&&f.SHA256==assemblyDebianConfigSHA||f.Role=="producer-source-receipt"&&f.SHA256=="13a80135f8b3b6392d8dc69df5f441a34eae0aa9592cc94dc6710ef2e061d1e0"{return nil,runtimeinstall.ErrInputProofUnavailable}}
 if _,e:=validateDebianProductProfile(p);e!=nil{return nil,e}
 // A closed metadata family is not a live eligibility capability. Product
 // packaged expected objects, actual UCRT/WOW64/current-host proof and the
 // separately reviewed held-original projection-copy protocol are absent.
 // Refuse before PID/observer/owner registration or resource reservation even
 // if a future source edit populates Product's registry with shape-valid data.
 return nil,runtimeinstall.ErrInputProofUnavailable
}
func debianProductProfilePins(p runtimeinstall.BundledDebianInputProofProfile,files map[string]runtimeinstall.DebianInputProofFile)(assemblyDebianProductProfilePins,error){
 ref:=func(role string)assemblyDebianObjectRef{f:=files[role];return assemblyDebianObjectRef{role,f.SHA256,f.Bytes}}
 raw,e:=canonicalDebianProducerProfile(p);if e!=nil{return assemblyDebianProductProfilePins{},e}
 pins:=assemblyDebianProductProfilePins{Profile:assemblyDebianObjectRef{"product-profile",p.SHA256,int64(len(raw))},SourceReceipt:ref("producer-source-receipt"),Config:ref("producer-config"),Entry:ref("projection-entry"),Projection:ref("projection-source"),Python:ref("cp-python"),GPG:ref("gpg"),GPGV:ref("gpgv")}
 if !validAssemblyDebianProductPins(pins)||pins.SourceReceipt.SHA256!=p.SourceSHA256{return assemblyDebianProductProfilePins{},runtimeinstall.ErrInputProofUnavailable};return pins,nil
}
func checkDebianProducerSourceDescriptor(data []byte,file runtimeinstall.DebianInputProofFile,snap assemblyInputCatalogSnapshot)error{
 var source assemblyCatalogSource
 if file.Role!="catalog-source-descriptor"||assemblyCatalogDigest(data)!=file.SHA256||int64(len(data))!=file.Bytes||decodeAssemblyCatalog(data,assemblyCatalogRecordLimit,&source)!=nil||!validAssemblyCatalogSource(source)||source!=snap.Source||snap.State.SourceDescriptor.Digest!=file.SHA256||snap.State.SourceDescriptor.Bytes!=file.Bytes||snap.State.Policy.Digest!=assemblyDebianPolicySourceSHA{return errAssemblyCatalogConflict};return nil
}
func debianProducerCommands(p *debianProducerPlan)(commands [10][]string){
 home:=filepath.Join(p.directory,"home");gpg,gpgv:=p.paths["gpg"],p.paths["gpgv"]
 base:=[]string{gpg,"--no-options","--homedir",home,"--batch","--no-tty"}
 for i:=0;i<3;i++{commands[i]=append(append([]string{},base...),"--yes","--output",filepath.Join(p.directory,fmt.Sprintf("keyring-%d",i)),"--dearmor",p.paths[fmt.Sprintf("public-key-%d",i)])}
 commands[3]=append(append([]string{},base...),"--with-colons","--fixed-list-mode","--with-fingerprint","--with-subkey-fingerprint","--no-default-keyring")
 for i:=0;i<3;i++{commands[3]=append(commands[3],"--keyring",filepath.Join(p.directory,fmt.Sprintf("keyring-%d",i)))};commands[3]=append(commands[3],"--list-keys")
 for i:=0;i<5;i++{commands[4+i]=[]string{gpgv,"--homedir",home,"--status-fd","1"};for k:=0;k<3;k++{commands[4+i]=append(commands[4+i],"--keyring",filepath.Join(p.directory,fmt.Sprintf("keyring-%d",k)))};commands[4+i]=append(commands[4+i],"--output",filepath.Join(p.directory,fmt.Sprintf("plaintext-%d",i)),p.paths[fmt.Sprintf("signed-release-%d",i)])}
 commands[9]=[]string{p.paths["cp-python"],"-I","-S","-B",p.paths["projection-entry"],p.paths["projection-source"],filepath.Join(p.directory,"projection-request.json"),filepath.Join(p.directory,"producer-output")};return
}
func writeDebianProjectionRequest(native *debianProofNativeOwner,p *debianProducerPlan,results []debianProofChildResult)error{
 input:=struct{Schema int `json:"schema"`;Config string `json:"config"`;Snapshot string `json:"snapshot"`;Paths map[string]string `json:"paths"`;Children []debianProofChildResult `json:"children"`}{1,p.paths["producer-config"],p.paths["snapshot-source"],p.paths,results}
 data,e:=json.Marshal(input);if e!=nil{return e};return native.writeBytes(filepath.Join(p.directory,"projection-request.json"),data,64<<10)
}
