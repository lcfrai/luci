package api

import (
 "context"
 "encoding/json"
 "errors"
 "io"
 "os"
 "path/filepath"
 "reflect"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

const documentAssemblyImageTag="luci/document-extractor:assembly"

// This explicit administrator operation produces a maintainer artifact. The
// standard finalizer/package/verified installer remains its only publication
// path; no assembly result is an image-registration or extraction-ready claim.
func(a *API)assembleCurrentDocumentImage(ctx context.Context,r runtimeinstall.Request,publish runtimeinstall.Publish)(out runtimeinstall.Outcome,retErr error){
 out=runtimeinstall.Outcome{Identity:r.Identity,Phase:"blocked",CleanupConfirmed:true,Message:"Document assembly has not started."}
 if r.DocumentAssembly!=nil{selection:=*r.DocumentAssembly;out.DocumentAssembly=&selection}
 fail:=func(e error)(runtimeinstall.Outcome,error){out.ReasonCode="document_assembly_unavailable";if out.ResourcesHeld{out.Phase="unknown";out.CleanupConfirmed=false;out.Message="The exact document assembly retains its allocation; reconcile it before new work."};if publish!=nil{if pe:=publish(out);pe!=nil{return out,pe}};return out,e}
 if a==nil||a.execution==nil||a.deps.Product==nil||publish==nil||r.DocumentAssembly==nil||r.Mode!=runtimeinstall.DocumentRuntimeAssemblyMode||r.ImageTag!=documentAssemblyImageTag||r.Retained()||!checkLogicalID(r.JobID){return fail(backend.ErrUnsupported)}
 selection:=*r.DocumentAssembly
 current:=func(c context.Context)error{if _,e:=a.authorizeRuntimeInstaller(c,r.Actor);e!=nil{return e};return a.deps.Product.CheckDocumentAssemblyCurrent(c,r.JobID,selection)}
 if e:=current(ctx);e!=nil{return fail(e)}
 unlock,e:=a.execution.lockChildLifecycle(ctx,"runtime-install-"+r.JobID);if e!=nil{return fail(e)};defer unlock()
 started:=time.Now().UTC();computeDeadline:=started.Add(backend.DocumentImageAssemblyComputeBudget);wholeDeadline:=computeDeadline.Add(backend.DocumentImageAssemblyCleanupBudget)
 compute,cancel:=context.WithDeadline(ctx,computeDeadline);defer cancel()
 profile,e:=a.deps.Product.DocumentAssemblyProfile(compute,selection.ProfileSHA256,selection.SourceSHA256);if e!=nil||profile.Root!=r.SourceDirectory{return fail(errors.Join(backend.ErrLaunchFenceOwnership,e))}
 owner,e:=a.documentImageAssemblyInputOwner();if e!=nil{return fail(e)}
 closure,e:=owner.resolveCurrentAssemblyClosure(compute,r.Actor,assemblyInputClosureRef{Schema:1,Digest:selection.InputClosureSHA256});if e!=nil{return fail(e)}
 validate:=func(c context.Context)error{if e:=current(c);e!=nil{return e};if _,e:=a.deps.Product.DocumentAssemblyProfile(c,selection.ProfileSHA256,selection.SourceSHA256);e!=nil{return e};return closure.current(c)}
 stage,e:=selectDocumentAssemblyStage(compute,profile,closure);if e!=nil{return fail(e)}
 selected,ok:=a.deps.Backend.(*dockerbe.Backend);if !ok{return fail(backend.ErrUnsupported)}
 engine,e:=selected.LaunchFenceIdentity(compute);if e!=nil{return fail(e)}
 allocation:="document-assembly-"+assemblyCatalogDigest([]byte(r.JobID))[:32]
 private,e:=a.execution.prepareExecutionDirectory();if e!=nil{return fail(e)}
 directory:=filepath.Join(private,"document-assembly-staging",allocation)
 spec:=backend.DocumentImageAssemblySpec{BaseImageID:backend.DocumentImageAssemblyBaseID,InputPath:filepath.Join(directory,"input"),InputBytes:stage.inputBytes,HelperPath:filepath.Join(directory,"helper"),HelperBytes:stage.helper.Bytes,Identity:backend.DocumentImageAssemblyIdentity{Protocol:backend.DocumentImageAssemblyProtocol,Profile:backend.DocumentImageAssemblyProfile,AllocationID:allocation,ScopeDigest:a.execution.scope,ActorDigest:backend.DocumentExtractionActorDigest(r.Actor.UserID,r.Actor.TeamID,r.Actor.ProjectID,r.Actor.Local),SourceCommit:closure.manifest.SourceCommit,SourceDigest:closure.manifest.SourceDigest,InputDigest:assemblyCatalogDigest(stage.manifest),HelperDigest:closure.manifest.HelperDigest,RecipeDigest:closure.manifest.RecipeDigest}}
 spec.Identity.CommandDigest=backend.DocumentImageAssemblyCommandDigest(spec)
 executor,e:=dockerbe.NewDocumentImageAssembler(selected,spec,dockerbe.DocumentAttachConfiguration{Headers:map[string]string{}});if e!=nil{return fail(e)}
 identity:=spec.Identity;identity.EngineID,identity.InstanceID=engine.EngineID,engine.InstanceID
 preparedSpec:=spec;preparedSpec.Identity=identity
 demand,e:=dockerbe.PlanDocumentDomainInputs(profile.Root,stage.inputs);if e!=nil{return fail(e)}
 inputsJSON,e:=json.Marshal(stage.inputs);if e!=nil{return fail(e)}
 if a.deps.ResourceSnapshot==nil{return fail(backend.ErrUnavailable)}
 snapshot:=a.deps.ResourceSnapshot()
 binding:=documentAssemblyStageBinding{InputsDigest:assemblyCatalogDigest(inputsJSON),Demand:demand,DirectoryBytes:stage.directoryBytes,State:"reserved",WriterHandles:1,WriterBufferBytes:64<<10}
 captured:=selection
 row:=documentImageAssemblyLease{ID:allocation,Scope:a.execution.scope,Actor:r.Actor,Identity:identity,InputClosureRef:closure.closure,Resources:backend.DocumentImageAssemblyResources(),BaseImageID:backend.DocumentImageAssemblyBaseID,CreateDiskBytes:stage.directoryBytes,CaptureDiskBytes:backend.DocumentImageAssemblyOutputMaxBytes,HostLaunchBytes:windowsDockerHostLaunchBytes(snapshot,backend.KindDocker),State:"staging",CreatedAt:started,UpdatedAt:started,ComputeDeadline:computeDeadline,WholeDeadline:wholeDeadline,Spec:&preparedSpec,DocumentSelection:&captured,DocumentJobID:r.JobID,Stage:&binding}
 policy,e:=a.deps.Product.SwarmSchedulerPolicy();if e!=nil{return fail(e)}
 if e=a.execution.reserveDocumentAssemblyStage(compute,row,policy,validate);e!=nil{return fail(e)}
 out.AllocationID,out.EngineID,out.InstanceID,out.SourceDigest=allocation,identity.EngineID,identity.InstanceID,identity.SourceDigest
 out.Phase,out.ResourcesHeld,out.CleanupConfirmed="staging",true,false
 out.Message="Staging the exact verified document inputs under the assembly allocation."
 live:=&liveDocumentAssemblyStage{}
 if _,loaded:=liveDocumentAssemblyStages.LoadOrStore(a.execution.assemblyStageKey(allocation),live);loaded{return fail(backend.ErrLaunchFenceOwnership)}
 // Any failure before container admission settles ONLY this original stage.
 // After admission, the existing assembly runtime owns container retirement.
 admitted:=false
 defer func(){
  if admitted{return}
  cleanup,stop:=context.WithDeadline(context.Background(),wholeDeadline);defer stop()
  if ce:=a.execution.closeDocumentAssemblyStage(cleanup,row,live);ce!=nil{out.ResourcesHeld=true;out.CleanupConfirmed=false;out.Phase="unknown";retErr=errors.Join(retErr,ce);return}
  if ce:=a.execution.finishUnstartedDocumentAssembly(cleanup,row);ce!=nil{out.ResourcesHeld=true;out.CleanupConfirmed=false;out.Phase="unknown";retErr=errors.Join(retErr,ce);return}
  out.ResourcesHeld=false;out.CleanupConfirmed=true;if out.Phase!="assembled"{out.Phase="failed"};if publish!=nil{retErr=errors.Join(retErr,publish(out))}
 }()
 if e=publish(out);e!=nil{return out,e}
 if e=validate(compute);e!=nil{return out,e}
 if e=a.execution.stageDocumentAssembly(compute,row,live,profile.Root,directory,stage,validate);e!=nil{return out,e}
 if e=a.execution.closeDocumentAssemblyStage(compute,row,live);e!=nil{return out,e}
 request:=documentImageAssemblyRequest{Spec:spec,Actor:r.Actor,InputClosureRef:closure.closure,CreatedAt:started,ComputeDeadline:computeDeadline,WholeDeadline:wholeDeadline,ValidateCurrent:validate,DocumentSelection:&captured,DocumentJobID:r.JobID}
 result,e:=a.executeDocumentImageAssembly(compute,executor,request)
 settlement,settledCancel:=context.WithDeadline(context.Background(),wholeDeadline);defer settledCancel()
 ledger,le:=a.execution.readReservationLedger(settlement);retainedState:="";if le==nil{if i,ie:=assemblyLeaseIndex(&ledger,identity);ie==nil{retainedState=ledger.ImageAssemblies[i].State;admitted=retainedState!="staging"}else{le=ie}}
 out.ResourcesHeld,out.CleanupConfirmed=result.ResourcesHeld,result.CleanupConfirmed
 if admitted&&retainedState!="finished"{out.ResourcesHeld=true;out.CleanupConfirmed=false}
 if le!=nil{out.ResourcesHeld=true;out.CleanupConfirmed=false;return out,errors.Join(e,le)}
 if e!=nil||result.State!="assembled_owned_resources_retired_not_registered"{out.Phase="failed";if out.ResourcesHeld{out.Phase="unknown"};out.Message="Document assembly did not produce a positively settled artifact; no runtime was registered.";return out,e}
 artifact,ae:=a.execution.documentAssemblyArtifact(settlement,identity,selection);if ae!=nil{out.Phase="unknown";out.Message="Assembly settled but its exact artifact receipt could not be confirmed; reconcile without replay.";return out,ae}
 out.DocumentAssembly=&artifact;out.Phase="assembled";out.Message="Document image artifact assembled and retired. Finalize and package it before preparing extraction; no runtime image was registered."
 return out,publish(out)
}

func(g *executionAdmission)stageDocumentAssembly(ctx context.Context,row documentImageAssemblyLease,live *liveDocumentAssemblyStage,source,directory string,stage assemblyStageSelection,current func(context.Context)error)error{
 if e:=current(ctx);e!=nil{return e}
 if e:=g.advanceDocumentAssemblyStage(ctx,row.Identity,"reserved","opening");e!=nil{return e}
 live.mu.Lock();if live.retiring{live.mu.Unlock();return backend.ErrUnavailable};live.opening=true;live.mu.Unlock()
 original,e:=dockerbe.OpenDocumentDomainInputs(ctx,source,stage.inputs)
 live.mu.Lock();live.owner=original;live.opening=false;retiring:=live.retiring;live.mu.Unlock()
 if e!=nil||original==nil||retiring{return errors.Join(backend.ErrUnavailable,e)}
 if original.Demand()!=row.Stage.Demand{return backend.ErrLaunchFenceOwnership}
 if e=g.advanceDocumentAssemblyStage(ctx,row.Identity,"opening","open");e!=nil{return e}
 if e=current(ctx);e!=nil{return e}
 parent:=filepath.Dir(directory);if e=os.Mkdir(parent,0700);e!=nil&&!errors.Is(e,os.ErrExist){return e};if e=codexRegularPath(filepath.Dir(parent),parent);e!=nil{return e};if e=codexPrivateDir(parent);e!=nil{return e}
 if e=os.Mkdir(directory,0700);e!=nil{return e};if e=codexPrivateDir(directory);e!=nil{return e}
 input:=filepath.Join(directory,"input");if e=os.Mkdir(input,0700);e!=nil{return e}
 readers:=map[string]io.ReaderAt{}
 for _,entry:=range stage.files{
  if e=current(ctx);e!=nil{return e};if e=original.CheckCurrent(ctx);e!=nil{return e}
  var reader io.ReaderAt
  if entry.generated==nil{reader=readers[entry.input.Role];if reader==nil{reader,e=original.ReaderAt(ctx,entry.input.Role);if e!=nil{return e};readers[entry.input.Role]=reader}}
  var bytes int64;var digest string
  if entry.generated!=nil{bytes=int64(len(entry.generated));digest=assemblyCatalogDigest(entry.generated)}else{bytes=entry.input.Bytes;digest=entry.input.SHA256}
  if e=writeAssemblyStageFile(ctx,live,input,entry.destination,reader,bytes,digest,entry.generated);e!=nil{return e}
 }
 helper,e:=original.ReaderAt(ctx,"assembly-helper");if e!=nil{return e}
 if e=writeAssemblyStageFile(ctx,live,directory,"helper",helper,stage.helper.Bytes,stage.helper.SHA256,nil);e!=nil{return e}
 if e=writeAssemblyStageFile(ctx,live,input,"assembly-manifest.json",nil,int64(len(stage.manifest)),assemblyCatalogDigest(stage.manifest),stage.manifest);e!=nil{return e}
 if e=original.CheckCurrent(ctx);e!=nil{return e};return current(ctx)
}

func(g *executionAdmission)finishUnstartedDocumentAssembly(ctx context.Context,before documentImageAssemblyLease)error{
 ledger,e:=g.readReservationLedger(ctx);if e!=nil{return e};i,e:=assemblyLeaseIndex(&ledger,before.Identity);if e!=nil{return e};row:=ledger.ImageAssemblies[i]
 if row.State!="staging"||row.Stage==nil||row.Stage.State!="closed"||row.Ref!=""{return backend.ErrLaunchFenceOwnership}
 out:=documentImageAssemblyOutcome{Identity:row.Identity,InputClosureRef:row.InputClosureRef,Stage:"staging",State:"failed_owned_resources_retired",ReaderSettled:true,CleanupConfirmed:true,WatchdogSettled:true}
 capture,e:=g.assemblyPrivateCapture(row.Identity);if e!=nil{return e};if e=capture.publish(ctx,out,false,false);e!=nil{return e}
 return g.mutate(ctx,func(l *executionLeaseLedger)error{i,e:=assemblyLeaseIndex(l,row.Identity);if e!=nil{return e};if !reflect.DeepEqual(l.ImageAssemblies[i],row){return backend.ErrLaunchFenceOwnership};l.ImageAssemblies[i].State="finished";l.ImageAssemblies[i].OutcomeDigest=assemblyOutcomeDigest(out);l.ImageAssemblies[i].UpdatedAt=time.Now().UTC();return nil})
}

func(g *executionAdmission)documentAssemblyArtifact(ctx context.Context,identity backend.DocumentImageAssemblyIdentity,selection runtimeinstall.DocumentAssemblyArtifact)(runtimeinstall.DocumentAssemblyArtifact,error){
 result:=selection;ledger,e:=g.readReservationLedger(ctx);if e!=nil{return result,e};i,e:=assemblyLeaseIndex(&ledger,identity);if e!=nil{return result,e};row:=ledger.ImageAssemblies[i]
 if row.State!="finished"||row.DocumentSelection==nil||*row.DocumentSelection!=selection{return result,backend.ErrLaunchFenceOwnership}
 capture,e:=g.assemblyPrivateCapture(identity);if e!=nil{return result,e};receipt,e:=capture.read();if e!=nil{return result,e};out:=receipt.Outcome
 if out.State!="assembled_owned_resources_retired_not_registered"||!receipt.CaptureExitVerified||out.ResourcesHeld||!out.CleanupConfirmed||!out.ReaderSettled||!out.WatchdogSettled||out.ExitCode==nil||*out.ExitCode!=0||out.OOMKilled||row.OutcomeDigest!=assemblyOutcomeDigest(out){return result,backend.ErrLaunchFenceOwnership}
 if e=capture.verify(ctx,out.Capture);e!=nil{return result,e};raw,e:=codexRead(capture.dir,capture.receipt,16<<10);if e!=nil{return result,e}
 result.Ref=assemblyIdentityDigest(identity);result.CaptureSHA256=out.Capture.Digest;result.CaptureBytes=out.Capture.Bytes;result.ReceiptSHA256=assemblyCatalogDigest(raw);return result,nil
}
