package api

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// A binary purpose has its own one-child owner, resource reservation and
// original deadlines. The signature worker is never reused or enlarged.
func(r *debianProducerLiveRun)executeBinary(ctx context.Context,profile runtimeinstall.BundledDebianInputProofProfile,files map[string]runtimeinstall.DebianInputProofFile,publish runtimeinstall.PublishDebianInputProof){
 out:=runtimeinstall.DebianInputProofOutcome{Identity:r.identity,Phase:"preparing",Message:"Validating the current signature facts and pinned binary inputs."}
 send:=func()error{r.mu.Lock();r.outcome=out;r.identity=out.Identity;r.mu.Unlock();return publish(out)}
 native:=newDebianBinaryProofNativeOwner(ctx)
 selected,e:=debianProductProfilePins(profile,files)
 var runErr error;var reserved bool;var accepted *assemblyAcceptedInputCandidate
 plan:=&debianProducerPlan{identity:r.identity,profile:profile,files:files,paths:map[string]string{},productPins:selected,readBudget:&debianProductReadBudget{deadline:native.workLimit()}}
 r.mu.Lock();r.native=native;r.plan=plan;r.outcome=out;r.mu.Unlock()
 workCtx,cancel:=context.WithDeadline(ctx,native.workLimit());defer cancel();ctx=workCtx
 defer func(){
  // Latch once; all original collectors/tickets stay reachable on failed Close
  // or a late synchronous return. Trust/config freshness is not cleanup proof.
  native.retire();settled:=native.settle()
  if !native.closeProduced(){settled=false}
  cleanupCtx,stop:=context.WithDeadline(context.Background(),native.wholeDeadline);defer stop()
  if b:=plan.binary;b!=nil{
   if b.transport!=nil{if ce:=native.settleBinaryReadTransport(cleanupCtx,b.transport);ce!=nil{settled=false}}
   if b.spool!=nil{if ce:=b.spool.closeRetiredRaw(cleanupCtx);ce!=nil{settled=false}}
   for _,g:=range []backend.DocumentImageAssemblyEvidenceGuard{b.selectionGuard,b.small,b.indexes,b.sourceIndexes}{if g!=nil{if ce:=g.Close();ce!=nil{settled=false}}}
   if b.packages!=nil{if ce:=b.packages.Close();ce!=nil{settled=false}}
  }
  if plan.inputs!=nil{if ce:=plan.inputs.Close();ce!=nil{settled=false}}
  if !plan.retireDocumentDomains(){settled=false}
  if !native.settleSourceAdmissions(cleanupCtx)||!native.originalCensusSettled(){settled=false}
  out.Identity=plan.identity;out.CleanupConfirmed=settled;out.ResourcesHeld=reserved&&!settled
  r.mu.Lock();r.resourceSettlementKnown=settled;r.mu.Unlock()
  if reserved&&settled{if ce:=r.api.execution.advanceDebianInputProof(cleanupCtx,plan.identity,"finished",nil);ce!=nil{out.CleanupConfirmed=false;out.ResourcesHeld=true;runErr=ce}}
  if accepted!=nil{accepted.mu.Lock();out.PublicationUnknown=out.PublicationUnknown||accepted.publicationUnknown||accepted.publicationRetired;accepted.retiring=true;accepted.mu.Unlock()}
  if runErr!=nil{out.Phase="failed";out.ReasonCode="input_proof_incomplete";out.Message="Binary input proof did not complete. Retained originals and any uncertain publication require exact reconciliation."}
  if !out.CleanupConfirmed||out.PublicationUnknown{out.Phase="unknown"}
  if ce:=send();ce!=nil{out.Phase="unknown";out.PublicationUnknown=true;runErr=ce}
  r.mu.Lock();r.outcome=out;r.err=runErr;r.mu.Unlock();close(r.done)
  if out.CleanupConfirmed&&!out.ResourcesHeld&&!out.PublicationUnknown{liveDebianProducerRuns.Delete(plan.identity.RunID)}
 }()
 if e!=nil||profile.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose||plan.identity.Purpose!=profile.Purpose{runErr=runtimeinstall.ErrInputProofOwnership;return}
 owner,e:=r.api.documentImageAssemblyInputOwner();if e!=nil{runErr=e;return};plan.owner=owner
 root,e:=r.api.execution.prepareExecutionDirectory();if e!=nil{runErr=e;return};plan.directory=filepath.Join(root,"debian-proof-"+plan.identity.RunID)
 if e=os.Mkdir(plan.directory,0700);e!=nil{runErr=e;return}
 canonical,e:=canonicalDebianProducerProfile(profile);if e!=nil{runErr=e;return}
 descriptor:=profile.DescriptorRelativePath;if descriptor==""{descriptor="profile.json"}
 objects:=[]backend.DocumentImageAssemblyEvidenceObject{{Role:"profile-descriptor",Path:filepath.Join(profile.Root,filepath.FromSlash(descriptor)),Bytes:int64(len(canonical)),Digest:profile.SHA256}}
 for _,f:=range profile.Files{path:=filepath.Join(profile.Root,filepath.FromSlash(f.RelativePath));plan.paths[f.Role]=path;objects=append(objects,backend.DocumentImageAssemblyEvidenceObject{Role:debianProfileEvidenceRole(f.Role),Path:path,Bytes:f.Bytes,Digest:f.SHA256})}
 acquisition,e:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(objects);if e!=nil{runErr=e;return}
 admission,e:=native.registerEvidenceSource(ctx,acquisition);if e!=nil{runErr=e;return}
 query,e:=native.reserveObserverQuery();if e!=nil{runErr=e;return};if e=native.startObserver(ctx,query);e!=nil{runErr=e;return};if e=native.initialAdmission(ctx);e!=nil{runErr=e;return}
 if e=plan.readBudget.acquisition(ctx,objects);e!=nil{runErr=e;return};plan.inputs,e=native.acquireEvidenceSource(ctx,admission);if e!=nil{runErr=e;return}
 snap,e:=plan.currentOrInitializeCatalog(ctx,r.actor);if e!=nil{runErr=e;return}
 source,e:=readDebianPlanInput(ctx,plan,"catalog-source-descriptor");if e!=nil||checkDebianProducerSourceDescriptor(source,files["catalog-source-descriptor"],snap)!=nil{runErr=errAssemblyCatalogConflict;return}
 plan.identity.ExpectedCAS=runtimeinstall.InputProofCAS{Revision:snap.CAS.Revision,Digest:snap.CAS.Digest};out.Identity=plan.identity
 b,e:=resolveDebianBinaryFirst3(ctx,plan,snap);if e!=nil{runErr=e;return};plan.binary=b
 plan.toolDomain,e=r.api.currentConfiguredDebianToolDomain(ctx,plan);if e!=nil{runErr=e;return}
 requirements,e:=debianHostRequirementsFromDomain(plan.toolDomain);if e!=nil{runErr=e;return};if _,e=observeDebianHostPrerequisites(ctx,native,requirements);e!=nil{runErr=e;return}
 if e=send();e!=nil{runErr=e;return};if e=plan.current(ctx,r.api);e!=nil{runErr=e;return}
 if e=os.Mkdir(filepath.Join(plan.directory,"home"),0700);e!=nil{runErr=e;return};plan.environment,e=debianProofEnvironment(filepath.Join(plan.directory,"home"));if e!=nil{runErr=e;return}
 scheduler,e:=r.api.deps.Product.SwarmSchedulerPolicy();if e!=nil{runErr=e;return}
 row:=debianInputProofExecutionLease{Scope:r.api.execution.scope,ID:plan.identity.AllocationID,Identity:plan.identity,Actor:r.actor,Resources:backend.Resources{MemoryMiB:2048,CPUMillis:1000},State:"reserved",DiskBytes:128<<20,CreatedAt:native.began,WholeDeadline:native.wholeDeadline}
 reserved=true
 if e=r.api.execution.reserveDebianInputProof(ctx,row,scheduler);e!=nil{
  checkCtx,stop:=context.WithTimeout(context.Background(),100*time.Millisecond);ledger,re:=r.api.execution.readReservationLedger(checkCtx);stop()
  if re==nil{reserved=false;for _,retained:=range ledger.InputProofs{if retained.Scope==row.Scope&&retained.ID==row.ID{reserved=true;if retained.Identity!=row.Identity{out.PublicationUnknown=true}}}}else{out.PublicationUnknown=true};runErr=e;return
 }
 out.Phase="reserved";out.ResourcesHeld=true;if e=send();e!=nil{runErr=e;return}
 if e=acquireDebianBinaryInputs(ctx,native,plan,b);e!=nil{runErr=e;return}
 expected,e:=prepareDebianBinarySelection(ctx,native,plan,b);if e!=nil{runErr=e;return}
 decoder,e:=newDebianBinaryRawDecoder(expected,native.began,native.workLimit());if e!=nil{runErr=e;return}
 spoolDirectory:=filepath.Join(plan.directory,"binary-raw");if e=os.Mkdir(spoolDirectory,0700);e!=nil{runErr=e;return}
 b.spool,e=newDebianBinarySpoolSet(ctx,native,spoolDirectory,decoder);if e!=nil{runErr=e;return}
 result,e:=native.binaryChild(ctx,r.api,plan,b.transport,b.spool);if e!=nil{runErr=e;return}
 captured,e:=captureDebianBinarySemantics(ctx,r.api,plan,b.spool,result,b.indexRecord);if e!=nil{runErr=e;return}
 accepted,e=buildDocumentAssemblyClosure(ctx,r.api,plan,captured);if e!=nil{runErr=e;return}
 r.mu.Lock();r.accepted=accepted;r.mu.Unlock()
 adopted,e:=owner.adoptDocumentAssemblyClosure(ctx,accepted);if e!=nil{runErr=e;return}
 out.PublishedCAS=runtimeinstall.InputProofCAS{Revision:adopted.Revision,Digest:adopted.Digest};out.Phase="facts-recorded";out.Message="The current authenticated document input closure was retained; image assembly remains a separate contained operation."
 for _,kind:=range []string{"signed-index-bindings","debian-binaries","input-closure","producer-raw-manifest"}{found:=false;for _,record:=range accepted.canonicalCopy(){if record.ref.Kind==kind{if found{runErr=errors.New("ambiguous closure outcome domain");return};found=true;out.Domains=append(out.Domains,runtimeinstall.InputProofDomainRef{Kind:kind,SHA256:record.ref.Digest,Bytes:record.ref.Bytes})}};if !found{runErr=errAssemblyDebianCapture;return}}
}
