package product

import (
 "context"
 "crypto/sha256"
 "fmt"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/management"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func TestConfiguredDocumentProfileUsesStartupPinsAndRejectsChangedDescriptor(t *testing.T){
 s,_:=proofProductFixture(t);root,err:=s.runtimeInputProofRoot();if err!=nil{t.Fatal(err)}
 if err=os.MkdirAll(root,0700);err!=nil{t.Fatal(err)}
 p:=runtimeinstall.BundledDebianInputProofProfile{ID:"configured",Protocol:runtimeinstall.DebianInputProofProtocol,SourceSHA256:strings.Repeat("b",64),Files:[]runtimeinstall.DebianInputProofFile{{Role:"support:manifests/windows-input-proof-tool-closure-v1.json",RelativePath:"manifests/windows-input-proof-tool-closure-v1.json",Bytes:1,SHA256:strings.Repeat("c",64)}}}
 raw,err:=runtimeinstall.CanonicalDebianInputProofProfile(p);if err!=nil{t.Fatal(err)}
 s.documentRuntime=runtimeinstall.DocumentRuntimeConfiguration{ProfileSHA256:fmt.Sprintf("%x",sha256.Sum256(raw)),ToolDomainSHA256:strings.Repeat("c",64)}
 if err=os.WriteFile(filepath.Join(root,"profile.json"),raw,0600);err!=nil{t.Fatal(err)}
 selected,exists,err:=s.selectedRuntimeInputProofProfile();if err!=nil||!exists||selected.ID!="configured"||selected.Root!=root{t.Fatal("configured profile was not selected",err)}
 // A retained legacy application entry cannot silently replace bad configured
 // input, and the file cannot repin itself by changing its declared fields.
 if err=os.WriteFile(filepath.Join(root,"profile.json"),append(raw,'\n'),0600);err!=nil{t.Fatal(err)}
 if _,_,err=s.selectedRuntimeInputProofProfile();err==nil{t.Fatal("changed configured descriptor fell back to legacy registry")}
 s.documentRuntime.ToolDomainSHA256=strings.Repeat("d",64)
 if err=os.WriteFile(filepath.Join(root,"profile.json"),raw,0600);err!=nil{t.Fatal(err)}
 if _,_,err=s.selectedRuntimeInputProofProfile();err==nil{t.Fatal("different independently selected tool domain admitted")}
}

func TestConfiguredBinaryProfileSelectsOnlyPinnedSignaturePrerequisite(t *testing.T){
 s,_:=proofProductFixture(t);root,err:=s.runtimeInputProofRoot();if err!=nil{t.Fatal(err)}
 if err=os.MkdirAll(filepath.Join(root,"profiles"),0700);err!=nil{t.Fatal(err)}
 tool:=runtimeinstall.DebianInputProofFile{Role:"support:manifests/windows-input-proof-tool-closure-v1.json",RelativePath:"manifests/windows-input-proof-tool-closure-v1.json",Bytes:1,SHA256:strings.Repeat("c",64)}
 child:=runtimeinstall.BundledDebianInputProofProfile{ID:"staged",Protocol:runtimeinstall.DebianInputProofProtocol,SourceSHA256:strings.Repeat("b",64),Files:[]runtimeinstall.DebianInputProofFile{tool}}
 childRaw,_:=runtimeinstall.CanonicalDebianInputProofProfile(child);childSHA:=fmt.Sprintf("%x",sha256.Sum256(childRaw))
 primary:=child;primary.Purpose=runtimeinstall.DebianBinaryInputProofPurpose;primary.Files=[]runtimeinstall.DebianInputProofFile{{Role:runtimeinstall.DebianSignatureProfileRole,RelativePath:runtimeinstall.DebianSignatureProfilePath,Bytes:int64(len(childRaw)),SHA256:childSHA},tool}
 primaryRaw,_:=runtimeinstall.CanonicalDebianInputProofProfile(primary);primarySHA:=fmt.Sprintf("%x",sha256.Sum256(primaryRaw))
 s.documentRuntime=runtimeinstall.DocumentRuntimeConfiguration{ProfileSHA256:primarySHA,ToolDomainSHA256:tool.SHA256}
 if err=os.WriteFile(filepath.Join(root,"profile.json"),primaryRaw,0600);err!=nil{t.Fatal(err)}
 if err=os.WriteFile(filepath.Join(root,filepath.FromSlash(runtimeinstall.DebianSignatureProfilePath)),childRaw,0600);err!=nil{t.Fatal(err)}
 selected,exists,err:=s.selectedRuntimeInputProofProfile()
 if err!=nil||!exists||selected.Purpose!=""||selected.SHA256!=childSHA||selected.DescriptorRelativePath!=runtimeinstall.DebianSignatureProfilePath||s.ValidateDocumentRuntimeProfile(context.Background(),selected)!=nil{t.Fatal("first stage did not select the exact pinned descriptor",err)}
 wrong:=selected;wrong.DescriptorRelativePath="profile.json";if s.ValidateDocumentRuntimeProfile(context.Background(),wrong)==nil{t.Fatal("selected stage adopted a different descriptor path")}
 s.runtimeJob.InputProof=&runtimeinstall.DebianInputProofOutcome{Identity:runtimeinstall.DebianInputProofIdentity{ProfileSHA256:childSHA},Phase:"facts-recorded",CleanupConfirmed:true}
 selected,_,err=s.selectedRuntimeInputProofProfile();if err!=nil||selected.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose||selected.SHA256!=primarySHA{t.Fatal("settled prerequisite did not select a separate binary stage",err)}
 if err=os.WriteFile(filepath.Join(root,filepath.FromSlash(runtimeinstall.DebianSignatureProfilePath)),append(childRaw,'\n'),0600);err!=nil{t.Fatal(err)}
 if _,_,err=s.selectedRuntimeInputProofProfile();err==nil{t.Fatal("changed transitive descriptor gained native eligibility")}
}

// The selected registry entry and all positive owner facts below are ONLY
// synthetic metadata in this test. No packaged tools or native proof is minted.
func proofProductFixture(t *testing.T)(*Server,*http.ServeMux){
 t.Helper();s,mux:=journeyFixture(t);s.registerSetupTools(mux);s.runtimeDir=runtimeSourceFixture(t)
 old:=bundledDebianProofProfiles;bundledDebianProofProfiles=[]runtimeinstall.BundledDebianInputProofProfile{{ID:"fixture-profile",Protocol:runtimeinstall.DebianInputProofProtocol,SHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64),Files:[]runtimeinstall.DebianInputProofFile{{Role:"fixture",RelativePath:"fixture",Bytes:1,SHA256:strings.Repeat("c",64)}}}}
 t.Cleanup(func(){s.closeRuntimeInstaller();bundledDebianProofProfiles=old});return s,mux
}
func TestRuntimeInputProofActualHandlerUsesTypedStageAndCancellation(t *testing.T){
 s,mux:=proofProductFixture(t);started,cancelled:=make(chan runtimeinstall.DebianInputProofRequest,1),make(chan struct{})
 s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor:runtimeActorFixture,Run:func(context.Context,runtimeinstall.Request,runtimeinstall.Publish)(runtimeinstall.Outcome,error){t.Error("proof was routed to image allocation");return runtimeinstall.Outcome{},nil},ProveDebianInputs:func(ctx context.Context,req runtimeinstall.DebianInputProofRequest,publish runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){out:=runtimeinstall.DebianInputProofOutcome{Identity:req.Identity,Phase:"reserved",ResourcesHeld:true,Message:"synthetic reservation"};if e:=publish(out);e!=nil{return out,e};started<-req;<-ctx.Done();close(cancelled);out.Phase="unknown";return out,ctx.Err()}})
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install",map[string]any{},202)
 var req runtimeinstall.DebianInputProofRequest;select{case req=<-started:case<-time.After(time.Second):t.Fatal("typed stage not called")}
 permit,e:=s.BeginRuntimeInputProofResume(context.Background(),req.Identity,0);if e!=nil{t.Fatal(e)}
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install/cancel",map[string]any{"jobId":req.Identity.JobID},202)
 select{case<-cancelled:case<-time.After(time.Second):t.Fatal("cancel did not reach operation")}
 if permit.Current()||permit.Finish(true)==nil{t.Fatal("late resume survived cancel")}
 j:=readRuntimeJob(t,mux);if j.Execution!=nil||j.InputProof==nil||!runtimeInstallUnresolved(j){t.Fatal("native ownership disappeared or became image state")}
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install",map[string]any{},202)
 if readRuntimeJob(t,mux).ID!=req.Identity.JobID{t.Fatal("unknown proof allocated a successor")}
}
func TestRuntimeInputProofCurrentGenerationAndOnceOnlyPermits(t *testing.T){
 s,mux:=proofProductFixture(t);started,release:=make(chan runtimeinstall.DebianInputProofRequest,1),make(chan struct{})
 s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor:runtimeActorFixture,ProveDebianInputs:func(ctx context.Context,req runtimeinstall.DebianInputProofRequest,_ runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){started<-req;select{case<-release:case<-ctx.Done():};return runtimeinstall.DebianInputProofOutcome{Identity:req.Identity,Phase:"cancelled",CleanupConfirmed:true},nil}})
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install",map[string]any{},202);req:=<-started;defer close(release)
 stale:=req.Identity;stale.JobGeneration="other";if s.CheckRuntimeInputProofCurrent(context.Background(),stale)==nil{t.Fatal("stale generation accepted")}
 stale=req.Identity;stale.ProfileSHA256=strings.Repeat("f",64);if s.CheckRuntimeInputProofCurrent(context.Background(),stale)==nil{t.Fatal("stale profile accepted")}
 for i:=0;i<10;i++{p,e:=s.BeginRuntimeInputProofResume(context.Background(),req.Identity,i);if e!=nil||!p.Current()||p.Finish(true)!=nil{t.Fatalf("ordinal%d: %v",i,e)};if _,e=s.BeginRuntimeInputProofResume(context.Background(),req.Identity,i);e==nil{t.Fatal("ordinal replay accepted")}}
 if _,e:=s.BeginRuntimeInputProofResume(context.Background(),req.Identity,10);e==nil{t.Fatal("eleventh child accepted")}
 target:=runtimeinstall.InputProofCAS{Revision:2,Digest:strings.Repeat("d",64)};p,e:=s.BeginRuntimeInputProofAdoption(context.Background(),req.Identity,target);if e!=nil{t.Fatal(e)}
 s.mu.Lock();s.runtimeInstall.proof.invalid.Store(true);s.mu.Unlock();if p.Current()||p.Finish(target,nil)==nil{t.Fatal("retirement became publication success")}
}

func TestRuntimeBinaryProofCapturesPurposeAndSingleResume(t *testing.T){
 s,mux:=proofProductFixture(t);bundledDebianProofProfiles[0].Purpose=runtimeinstall.DebianBinaryInputProofPurpose
 started,release:=make(chan runtimeinstall.DebianInputProofRequest,1),make(chan struct{})
 s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor:runtimeActorFixture,ProveDebianInputs:func(ctx context.Context,req runtimeinstall.DebianInputProofRequest,_ runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){started<-req;select{case<-release:case<-ctx.Done():};return runtimeinstall.DebianInputProofOutcome{Identity:req.Identity,Phase:"cancelled",CleanupConfirmed:true},nil}})
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install",map[string]any{},202);req:=<-started;defer close(release)
 if req.Identity.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose{t.Fatal("purpose was not captured")}
 s.mu.RLock();deadline:=s.runtimeInstall.proof.deadline;s.mu.RUnlock()
 if time.Until(deadline)<290*time.Second||time.Until(deadline)>300*time.Second{t.Fatal("binary work reused signature clock")}
 wrong:=req.Identity;wrong.Purpose="";if s.CheckRuntimeInputProofCurrent(context.Background(),wrong)==nil{t.Fatal("purpose substituted on current operation")}
 target:=runtimeinstall.InputProofCAS{Revision:2,Digest:strings.Repeat("d",64)}
 if _,e:=s.BeginRuntimeInputProofAdoption(context.Background(),req.Identity,target);e==nil{t.Fatal("adoption before original resume")}
 permit,e:=s.BeginRuntimeInputProofResume(context.Background(),req.Identity,0);if e!=nil||permit.Finish(true)!=nil{t.Fatal("single binary resume refused",e)}
 if _,e=s.BeginRuntimeInputProofResume(context.Background(),req.Identity,1);e==nil{t.Fatal("second binary child admitted")}
 if _,e=s.BeginRuntimeInputProofAdoption(context.Background(),req.Identity,target);e!=nil{t.Fatal("completed single stage did not permit exact adoption",e)}
}
func TestRuntimeInputProofPersistenceCannotBlockCancelLatch(t *testing.T){
 s,mux:=proofProductFixture(t);started,cancelled:=make(chan runtimeinstall.DebianInputProofRequest,1),make(chan struct{})
 s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor:runtimeActorFixture,ProveDebianInputs:func(ctx context.Context,req runtimeinstall.DebianInputProofRequest,_ runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){started<-req;<-ctx.Done();close(cancelled);return runtimeinstall.DebianInputProofOutcome{Identity:req.Identity,Phase:"cancelled",CleanupConfirmed:true},nil}})
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install",map[string]any{},202);req:=<-started
 s.runtimeInstall.proofIO.Lock();finished:=make(chan struct{});go func(){defer close(finished);w:=httptest.NewRecorder();request:=httptest.NewRequest("POST","/v1/setup/runtime/install/cancel",strings.NewReader(`{"jobId":"`+req.Identity.JobID+`"}`));s.cancelRuntimeInstall(w,request)}()
 select{case<-cancelled:case<-time.After(time.Second):s.runtimeInstall.proofIO.Unlock();t.Fatal("persistence lock blocked emergency cancel")}
 if s.CheckRuntimeInputProofCurrent(context.Background(),req.Identity)==nil{s.runtimeInstall.proofIO.Unlock();t.Fatal("cancelled generation remained eligible")};s.runtimeInstall.proofIO.Unlock();select{case<-finished:case<-time.After(time.Second):t.Fatal("cancel publication did not settle")}
}
func TestRuntimeInputProofReloadClosedKindAndLegacyImage(t *testing.T){
 id:=runtimeinstall.DebianInputProofIdentity{JobID:"j",JobGeneration:"g",RunID:"r",AllocationID:"a",ProfileID:"p",ProfileSHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64)}
 value:=RuntimeJob{ID:"j",OperationKind:runtimeinstall.DebianInputProofKind,Status:"installing",InputProof:&runtimeinstall.DebianInputProofOutcome{Identity:id,Phase:"reserved",ResourcesHeld:true}}
 data,_:=json.Marshal(value);var decoded RuntimeJob;if decodeRuntimeJobState(data,&decoded)!=nil||!runtimeInstallRetained(decoded){t.Fatal("native receipt did not decode/retain")}
 decoded.Execution=&runtimeinstall.Outcome{};bad,_:=json.Marshal(decoded);if decodeRuntimeJobState(bad,&decoded)==nil{t.Fatal("mixed image/native arms accepted")}
 if decodeRuntimeJobState([]byte(`{"id":"j","operationKind":"document-input-proof","operationKind":"","inputProof":null}`),&decoded)==nil{t.Fatal("duplicate kind bypass")}
 if decodeRuntimeJobState([]byte(`{"id":"old","status":"failed","futureLegacyField":1}`),&decoded)!=nil{t.Fatal("legacy optional read changed")}
 s,_:=journeyFixture(t);s.runtimeJob=value;s.runtimeInstall.proof=nil;if s.CheckRuntimeInputProofCurrent(context.Background(),id)==nil{t.Fatal("decoded record minted live generation")}
}
func TestRuntimeInputProofFirstBindingRejectsInitialWildcardReplayAndStickyPublication(t *testing.T){
 s,mux:=proofProductFixture(t);started,release:=make(chan runtimeinstall.DebianInputProofRequest,1),make(chan struct{})
 s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor:runtimeActorFixture,ProveDebianInputs:func(ctx context.Context,req runtimeinstall.DebianInputProofRequest,_ runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){started<-req;select{case<-release:case<-ctx.Done():};return runtimeinstall.DebianInputProofOutcome{Identity:req.Identity,Phase:"cancelled",CleanupConfirmed:true},nil}})
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install",map[string]any{},202);initial:=<-started;defer close(release)
 bound:=initial.Identity;bound.OwnerPID=17;bound.OwnerCreationTime=19;bound.ExpectedCAS=runtimeinstall.InputProofCAS{Revision:1,Digest:strings.Repeat("c",64)}
 out:=runtimeinstall.DebianInputProofOutcome{Identity:bound,Phase:"reserved",ResourcesHeld:true}
 if e:=s.publishRuntimeInputProof(bound.JobID,out);e!=nil{t.Fatal(e)}
 replay:=out;replay.Identity=initial.Identity;if s.publishRuntimeInputProof(bound.JobID,replay)==nil{t.Fatal("initial zero binding was reusable after durable normalization")}
 out.Phase="unknown";out.PublicationUnknown=true;if e:=s.publishRuntimeInputProof(bound.JobID,out);e!=nil{t.Fatal(e)}
 cleanup:=out;cleanup.Phase="failed";cleanup.ResourcesHeld=false;cleanup.CleanupConfirmed=true;cleanup.PublicationUnknown=false
 if s.publishRuntimeInputProof(bound.JobID,cleanup)==nil{t.Fatal("native cleanup alone terminalized catalog uncertainty")}
 if !runtimeInstallUnresolved(readRuntimeJob(t,mux)){t.Fatal("publication uncertainty released the job")}
}
func TestRuntimeInputProofQueuedPublicationPrecedesVisibleCancellationIdentity(t *testing.T){
 s,mux:=proofProductFixture(t);captured:=make(chan struct{},1)
 s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor:func(ctx context.Context)(management.ExecutionActorRef,error){captured<-struct{}{};return runtimeActorFixture(ctx)},ProveDebianInputs:func(_ context.Context,req runtimeinstall.DebianInputProofRequest,_ runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){return runtimeinstall.DebianInputProofOutcome{Identity:req.Identity,Phase:"blocked",CleanupConfirmed:true},nil}})
 s.runtimeInstall.proofIO.Lock();finished:=make(chan struct{});go func(){defer close(finished);w:=httptest.NewRecorder();mux.ServeHTTP(w,httptest.NewRequest("POST","/v1/setup/runtime/install",strings.NewReader(`{}`)))}()
 <-captured;s.mu.RLock();visible:=s.runtimeJob.OperationKind==runtimeinstall.DebianInputProofKind||s.runtimeInstall.done!=nil;s.mu.RUnlock()
 if visible{s.runtimeInstall.proofIO.Unlock();t.Fatal("cancellable job was visible before it owned initial persistence")}
 s.runtimeInstall.proofIO.Unlock();select{case<-finished:case<-time.After(time.Second):t.Fatal("initial publication did not settle")}
}
func TestRuntimeInputProofActualReconcileUsesBoundNativeObserverAndKeepsPublicationUnknown(t *testing.T){
 s,mux:=proofProductFixture(t);id:=runtimeinstall.DebianInputProofIdentity{JobID:"native-retained",JobGeneration:"g",RunID:"r",AllocationID:"a",ProfileID:"p",ProfileSHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64),OwnerPID:17,OwnerCreationTime:19,ExpectedCAS:runtimeinstall.InputProofCAS{Revision:1,Digest:strings.Repeat("c",64)}}
 s.runtimeJob=RuntimeJob{ID:id.JobID,OperationKind:runtimeinstall.DebianInputProofKind,Status:"unknown",InputProof:&runtimeinstall.DebianInputProofOutcome{Identity:id,Phase:"unknown",PublicationUnknown:true,ResourcesHeld:true}}
 observed:=make(chan runtimeinstall.DebianInputProofIdentity,1)
 s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor:runtimeActorFixture,Reconcile:func(context.Context,runtimeinstall.Identity,management.ExecutionActorRef,runtimeinstall.Publish)(runtimeinstall.Outcome,error){t.Error("native recovery called image reconciler");return runtimeinstall.Outcome{},nil},ObserveDebianInputProof:func(_ context.Context,bound runtimeinstall.DebianInputProofIdentity,_ management.ExecutionActorRef,publish runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){observed<-bound;out:=runtimeinstall.DebianInputProofOutcome{Identity:bound,Phase:"unknown",PublicationUnknown:true,CleanupConfirmed:true};return out,publish(out)}})
 journeyRequest(t,mux,"POST","/v1/setup/runtime/install/reconcile",map[string]any{"jobId":id.JobID},202)
 select{case got:=<-observed:if got!=id{t.Fatal("recovery used initial wildcard identity")};case<-time.After(time.Second):t.Fatal("typed observer not called")}
 until:=time.Now().Add(time.Second);for time.Now().Before(until){j:=readRuntimeJob(t,mux);if j.InputProof!=nil&&j.InputProof.CleanupConfirmed{if !j.InputProof.PublicationUnknown||!runtimeInstallUnresolved(j){t.Fatal("native exit released publication uncertainty")};return};time.Sleep(time.Millisecond)};t.Fatal("observer publication did not settle")
}
