package product

import (
 "context"
 "errors"
 "net/http"
 "path/filepath"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func validateRuntimeJobKind(j RuntimeJob) error {
 switch j.OperationKind {
 case "","image-install": if j.InputProof!=nil{return runtimeinstall.ErrInputProofOwnership}
 case runtimeinstall.DebianInputProofKind:
  if j.Execution!=nil || j.InputProof==nil || j.ID!=j.InputProof.Identity.JobID || !runtimeinstall.ValidInputProofOutcome(*j.InputProof){return runtimeinstall.ErrInputProofOwnership}
 default:return runtimeinstall.ErrInputProofOwnership
 };return nil
}
func (s *Server) installRuntimeInputProof(w http.ResponseWriter,r *http.Request,p runtimeinstall.BundledDebianInputProofProfile){
 s.mu.RLock();rt:=s.runtimeInstall.runtime;closed:=s.runtimeInstall.closed;s.mu.RUnlock()
 if closed || rt.CaptureActor==nil || rt.ProveDebianInputs==nil {problem(w,503,"runtime_input_proof_unavailable",runtimeinstall.ErrInputProofUnavailable.Error());return}
 limits,valid:=runtimeinstall.DebianInputProofLimits(p.Purpose);if !valid{problem(w,503,"runtime_input_proof_unavailable",runtimeinstall.ErrInputProofUnavailable.Error());return}
 actor,err:=rt.CaptureActor(r.Context());if err!=nil{problem(w,403,"runtime_forbidden",err.Error());return}
 // Serialize the initial durable binding before exposing its cancellable job.
 // No Product state lock is held while waiting for persistence or doing IO.
 s.runtimeInstall.proofIO.Lock()
 s.mu.Lock()
 if s.runtimeInstall.closed || s.runtimeInstall.done!=nil || runtimeInstallActive(s.runtimeJob.Status) || runtimeInstallUnresolved(s.runtimeJob){j:=s.runtimeJob;s.mu.Unlock();s.runtimeInstall.proofIO.Unlock();writeJSON(w,202,j);return}
 now:=time.Now().UTC();id:=runtimeinstall.DebianInputProofIdentity{Purpose:p.Purpose,JobID:newID("runtime-"),JobGeneration:newID("generation-"),RunID:newID("proof-"),AllocationID:newID("native-"),ProfileID:p.ID,ProfileSHA256:p.SHA256,SourceSHA256:p.SourceSHA256}
 out:=runtimeinstall.DebianInputProofOutcome{Identity:id,Phase:"queued",Message:"Checking the bundled input proof profile before reserving native resources."}
 j:=RuntimeJob{ID:id.JobID,OperationKind:runtimeinstall.DebianInputProofKind,InputProof:&out,Status:"installing",Message:out.Message,CreatedAt:now,UpdatedAt:now}
 ctx,cancel:=context.WithTimeout(context.Background(),limits.Whole);done:=make(chan struct{})
 s.runtimeJob=j;s.runtimeInstall.cancel,s.runtimeInstall.done=cancel,done;s.runtimeInstall.proof=&runtimeInputProofOwnership{identity:id,deadline:now.Add(limits.Work),projectsRoot:s.config.ProjectsDir,runtimeRoot:s.runtimeDir};s.mu.Unlock()
 err=atomicJSON(filepath.Join(s.stateDir,"runtime-install.json"),j)
 s.runtimeInstall.proofIO.Unlock()
 if err!=nil {cancel();s.mu.Lock();s.runtimeInstall.proof.invalid.Store(true);s.runtimeJob.Status="unknown";s.runtimeInstall.cancel,s.runtimeInstall.done=nil,nil;close(done);s.mu.Unlock();problem(w,500,"runtime_save_failed",err.Error());return}
 go s.runRuntimeInputProof(ctx,cancel,done,id.JobID,func(publish runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){return rt.ProveDebianInputs(ctx,runtimeinstall.DebianInputProofRequest{Identity:id,Actor:actor},publish)})
 writeJSON(w,202,j)
}
func (s *Server) publishRuntimeInputProof(id string,out runtimeinstall.DebianInputProofOutcome) error {
 if !runtimeinstall.ValidInputProofOutcome(out){return runtimeinstall.ErrInputProofOwnership}
 s.runtimeInstall.proofIO.Lock();defer s.runtimeInstall.proofIO.Unlock()
 s.mu.Lock()
 j:=s.runtimeJob
 if s.runtimeInstall.sealed || j.ID!=id || j.OperationKind!=runtimeinstall.DebianInputProofKind || j.InputProof==nil || !runtimeinstall.SameInputProofOperation(j.InputProof.Identity,out.Identity){s.mu.Unlock();return runtimeinstall.ErrInputProofOwnership}
 if out.Phase=="facts-recorded" && (j.CancelRequested || s.runtimeInstall.proof==nil || s.runtimeInstall.proof.invalid.Load()){s.mu.Unlock();return runtimeinstall.ErrInputProofOwnership}
 if j.InputProof.PublicationUnknown&&!out.PublicationUnknown{s.mu.Unlock();return runtimeinstall.ErrInputProofOwnership}
 switch out.Phase{case "preparing","reserved","creating-suspended","running":if !s.proofCurrentLocked(out.Identity){s.mu.Unlock();return runtimeinstall.ErrInputProofOwnership}}
 if s.runtimeInstall.proof!=nil {s.runtimeInstall.proof.identity=out.Identity}
 j.InputProof=&out;j.Message,j.UpdatedAt=out.Message,time.Now().UTC()
 switch out.Phase{case "facts-recorded","blocked","failed","cancelled","unknown":j.Status=out.Phase;default:j.Status="installing"}
 if j.CancelRequested && runtimeInstallActive(j.Status){j.Status="cancelling"}
 s.runtimeJob=j;s.mu.Unlock()
 err:=atomicJSON(filepath.Join(s.stateDir,"runtime-install.json"),j)
 s.mu.Lock();defer s.mu.Unlock()
 if err!=nil{if s.runtimeJob.ID==id{s.runtimeJob.Status="unknown"};return err}
 if s.runtimeInstall.sealed || s.runtimeJob.ID!=id || s.runtimeJob.CancelRequested && out.Phase=="facts-recorded"{return runtimeinstall.ErrInputProofOwnership};return nil
}
func (s *Server) runRuntimeInputProof(ctx context.Context,cancel context.CancelFunc,done chan struct{},id string,run func(runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error)){
 defer func(){cancel();s.mu.Lock();if s.runtimeInstall.done==done{s.runtimeInstall.done,s.runtimeInstall.cancel=nil,nil};close(done);s.mu.Unlock()}()
 out,err:=run(func(v runtimeinstall.DebianInputProofOutcome)error{return s.publishRuntimeInputProof(id,v)})
 if out.Identity.JobID!=""{if e:=s.publishRuntimeInputProof(id,out);e!=nil{err=e}}
 s.mu.Lock();defer s.mu.Unlock()
 if s.runtimeInstall.sealed || s.runtimeJob.ID!=id{return}
 j:=s.runtimeJob
 if err!=nil || runtimeInstallActive(j.Status){
  if runtimeInstallRetained(j) || j.InputProof==nil || j.InputProof.PublicationUnknown{j.Status="unknown";j.Message="The exact native input proof has not settled; no replacement was started."}else if errors.Is(ctx.Err(),context.Canceled)||j.CancelRequested{j.Status="cancelled"}else{j.Status="failed"}
  // A failed acknowledgement retains the last durable active/unknown record;
  // restart already maps active to unknown. Never overwrite cancellation from
  // a fallback writer or block the emergency latch on filesystem publication.
  j.UpdatedAt=time.Now().UTC();s.runtimeJob=j
 }
}
func (s *Server) reconcileRuntimeInputProof(w http.ResponseWriter,r *http.Request,id string,rt runtimeinstall.Runtime){
 if rt.ObserveDebianInputProof==nil{problem(w,503,"runtime_input_proof_observer_unavailable","Native proof recovery is unavailable; no image reconciliation or replacement was started.");return}
 actor,err:=rt.CaptureActor(r.Context());if err!=nil{problem(w,403,"runtime_forbidden",err.Error());return}
 s.mu.Lock();j:=s.runtimeJob
 if s.runtimeInstall.closed || j.ID!=id || j.InputProof==nil || j.OperationKind!=runtimeinstall.DebianInputProofKind{s.mu.Unlock();problem(w,409,"runtime_job_changed","The retained native proof is no longer current.");return}
 if s.runtimeInstall.done!=nil || !runtimeInstallUnresolved(j){s.mu.Unlock();writeJSON(w,200,j);return}
 j.Status="reconciling";j.UpdatedAt=time.Now().UTC()
 ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);done:=make(chan struct{});s.runtimeJob=j;s.runtimeInstall.cancel,s.runtimeInstall.done=cancel,done;s.mu.Unlock()
 go s.runRuntimeInputProof(ctx,cancel,done,id,func(publish runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){return rt.ObserveDebianInputProof(ctx,j.InputProof.Identity,actor,publish)})
 writeJSON(w,202,j)
}
func(s *Server)cancelRuntimeInputProof(w http.ResponseWriter,r *http.Request,id string){
 s.mu.Lock();if s.runtimeJob.ID!=id||s.runtimeJob.OperationKind!=runtimeinstall.DebianInputProofKind{s.mu.Unlock();problem(w,409,"runtime_job_changed","The selected proof changed.");return}
 cancel:=s.runtimeInstall.cancel
 if cancel==nil||s.runtimeJob.CancelRequested||!runtimeInstallActive(s.runtimeJob.Status){j:=s.runtimeJob;s.mu.Unlock();writeJSON(w,200,j);return}
 if s.runtimeInstall.proof!=nil{s.runtimeInstall.proof.invalid.Store(true)}
 s.runtimeJob.CancelRequested=true;s.runtimeJob.Status="cancelling";s.runtimeJob.UpdatedAt=time.Now().UTC();s.mu.Unlock()
 cancel() // native emergency retirement is independent of publication IO
 s.runtimeInstall.proofIO.Lock();s.mu.RLock();j:=s.runtimeJob;s.mu.RUnlock()
 err:=atomicJSON(filepath.Join(s.stateDir,"runtime-install.json"),j);s.runtimeInstall.proofIO.Unlock()
 if err!=nil{problem(w,500,"runtime_save_failed",err.Error());return};writeJSON(w,202,j)
}
