package api

import (
 "context"
 "errors"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
 "github.com/lcfrai/luci/control-plane/internal/management"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Reconciliation never reopens source inputs, prepares a new image, reattaches
// capture, or replays compute. Historical cleanup does not require the current
// package/profile to remain installed.
func(a *API)reconcileDocumentAssembly(ctx context.Context,id runtimeinstall.Identity,actor management.ExecutionActorRef,publish runtimeinstall.Publish)(runtimeinstall.Outcome,error){
 out:=runtimeinstall.Outcome{Identity:id,Phase:"unknown",ResourcesHeld:true,Message:"The exact document assembly still requires settlement."}
 if a==nil||a.execution==nil||a.deps.Product==nil||publish==nil||!checkLogicalID(id.JobID)||id.ImageTag!=documentAssemblyImageTag{return out,backend.ErrUnsupported}
 selection,e:=a.deps.Product.DocumentAssemblyRecoverySelection(id.JobID);if e!=nil{return out,e};out.DocumentAssembly=&selection
 if _,e=a.authorizeRuntimeInstaller(ctx,actor);e!=nil{return out,e}
 unlock,e:=a.execution.lockChildLifecycle(ctx,"runtime-install-"+id.JobID);if e!=nil{return out,e};defer unlock()
 var row documentImageAssemblyLease
 e=a.execution.mutate(ctx,func(l *executionLeaseLedger)error{
  for _,v:=range l.ImageAssemblies{if v.Scope==a.execution.scope&&v.DocumentJobID==id.JobID{if row.ID!=""||v.Actor!=actor||v.DocumentSelection==nil||*v.DocumentSelection!=selection{return backend.ErrLaunchFenceOwnership};row=v}}
  return nil
 });if e!=nil{return out,e}
 if row.ID==""{if id.AllocationID!=""||id.EngineID!=""||id.InstanceID!=""||id.SourceDigest!=""{return out,backend.ErrLaunchFenceOwnership};out.Phase="cancelled";out.ResourcesHeld=false;out.CleanupConfirmed=true;out.Message="No assembly allocation was started; nothing was replayed.";return out,publish(out)}
 if id.AllocationID!=""&&(id.AllocationID!=row.ID||id.EngineID!=row.Identity.EngineID||id.InstanceID!=row.Identity.InstanceID||id.SourceDigest!=row.Identity.SourceDigest){return out,backend.ErrLaunchFenceOwnership}
 out.AllocationID,out.EngineID,out.InstanceID,out.SourceDigest=row.ID,row.Identity.EngineID,row.Identity.InstanceID,row.Identity.SourceDigest
 if row.Stage!=nil&&row.Stage.State!="closed"{
  var live *liveDocumentAssemblyStage
  if actual,ok:=liveDocumentAssemblyStages.Load(a.execution.assemblyStageKey(row.ID));ok{live=actual.(*liveDocumentAssemblyStage)}else if row.State=="staging"&&row.Stage.State=="reserved"{live=&liveDocumentAssemblyStage{}}
  if live==nil{return out,backend.ErrUnavailable}
  if e=a.execution.closeDocumentAssemblyStage(ctx,row,live);e!=nil{return out,e}
  ledger,readErr:=a.execution.readReservationLedger(ctx);if readErr!=nil{return out,readErr};i,readErr:=assemblyLeaseIndex(&ledger,row.Identity);if readErr!=nil{return out,readErr};row=ledger.ImageAssemblies[i]
 }
 if row.State=="staging"{if e=a.execution.finishUnstartedDocumentAssembly(ctx,row);e!=nil{return out,e};out.Phase="cancelled";out.ResourcesHeld=false;out.CleanupConfirmed=true;out.Message="Original staging inputs retired; assembly was never started and was not replayed.";return out,publish(out)}
 if row.State=="finished"{
  artifact,ae:=a.execution.documentAssemblyArtifact(ctx,row.Identity,selection)
  if ae==nil{out.DocumentAssembly=&artifact;out.Phase="assembled";out.ResourcesHeld=false;out.CleanupConfirmed=true;out.Message="The exact settled maintainer artifact is retained; no image was registered.";return out,publish(out)}
  // A finished row is positive retirement. Missing/corrupt capture cannot be
  // promoted to an artifact and does not authorize a new invocation.
  out.Phase="failed";out.ResourcesHeld=false;out.CleanupConfirmed=true;out.ReasonCode="document_assembly_artifact_unavailable";out.Message="Assembly resources retired, but its exact artifact is unavailable. No compute was replayed."
  return out,errors.Join(ae,publish(out))
 }
 if row.Spec==nil{return out,backend.ErrLaunchFenceOwnership}
 selected,ok:=a.deps.Backend.(*dockerbe.Backend);if !ok{return out,backend.ErrUnsupported}
 spec:=*row.Spec;spec.Identity.EngineID,spec.Identity.InstanceID="",""
 executor,e:=dockerbe.NewDocumentImageAssembler(selected,spec,dockerbe.DocumentAttachConfiguration{Headers:map[string]string{}});if e!=nil{return out,e}
 request:=documentImageAssemblyRequest{Spec:spec,Actor:actor,InputClosureRef:row.InputClosureRef,CreatedAt:row.CreatedAt,ComputeDeadline:row.ComputeDeadline,WholeDeadline:row.WholeDeadline,DocumentSelection:&selection,DocumentJobID:id.JobID}
 hooks,e:=a.documentImageAssemblyRecoveryHooks(executor,request,row);if e!=nil{return out,e}
 cleanup,stop:=context.WithTimeout(ctx,backend.DocumentImageAssemblyCleanupBudget);defer stop()
 if row.State=="reserved"{
  // Reservation precedes every possible Create. It needs no source/profile or
  // image readback to retire an exact never-started allocation.
  settled:=documentImageAssemblyOutcome{Identity:row.Identity,InputClosureRef:row.InputClosureRef,Stage:"recovery",State:"interrupted_capture_unavailable_owned_resources_retired",ReaderSettled:true,CleanupConfirmed:true,WatchdogSettled:true}
  if e=hooks.Publish(cleanup,settled);e!=nil{return out,e};if e=hooks.Finish(cleanup,row.Identity,settled);e!=nil{return out,e}
 }else{
  intent,e:=assemblyIntentID();if e!=nil{return out,e}
  result,re:=recoverDocumentImageAssembly(cleanup,executor,hooks,row,backend.DocumentImageAssemblyFenceRequest{Identity:row.Identity,IntentID:intent,ExpectedRef:row.Ref},nil)
  if re!=nil||result.ResourcesHeld||!result.CleanupConfirmed{return out,errors.Join(re,backend.ErrUnavailable)}
 }
 out.Phase="cancelled";out.ResourcesHeld=false;out.CleanupConfirmed=true;out.Message="The exact historical assembly retired. Capture is unavailable; no build or import was replayed."
 return out,publish(out)
}

