package product

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "github.com/lcfrai/luci/control-plane/internal/hostpath"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// The request selects only a closed operation kind. Source and expected trust
// pin always come from the owning installation, never a browser path or hash.
func(s *Server)documentRuntimeSource()(string,error){
 if !runtimeinstall.InputProofHex(s.documentRuntime.ImageManifestSHA256)||!filepath.IsAbs(s.runtimeDir){return "",errors.New("the independently verified document runtime package is not configured in this installation")}
 root,err:=hostpath.ResolveOutside(s.projectRoot(),s.runtimeDir);if err!=nil{return "",err}
 root=filepath.Join(root,"document-image")
 for _,name:=range []string{"document-runtime-manifest-v1.json","document-image-linux-amd64.tar"}{
  file:=filepath.Join(root,name);resolved,e:=hostpath.Confine(root,file);if e!=nil||resolved!=file{return "",errors.New("document runtime package path changed")}
  info,e:=os.Lstat(file);if e!=nil||!info.Mode().IsRegular()||info.Size()<1{return "",errors.New("the verified document runtime companion is missing; reinstall a complete package")}
 }
 return root,nil
}

func(s *Server)DocumentRuntimeSource()(string,error){return s.documentRuntimeSource()}

func(s *Server)documentAssemblySelection(prior RuntimeJob)(string,*runtimeinstall.DocumentAssemblyArtifact,error){
 proof:=prior.InputProof
 if proof==nil||prior.OperationKind!=runtimeinstall.DebianInputProofKind||proof.Identity.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose||proof.Phase!="facts-recorded"||!runtimeinstall.ValidInputProofOutcome(*proof)||!proof.CleanupConfirmed||proof.ResourcesHeld||proof.PublicationUnknown{return "",nil,errors.New("Complete the current document input proof before explicitly assembling its image")}
 p,e:=s.DocumentAssemblyProfile(context.Background(),proof.Identity.ProfileSHA256,proof.Identity.SourceSHA256);if e!=nil{return "",nil,e}
 ref:="";for _,domain:=range proof.Domains{if domain.Kind=="input-closure"{if ref!=""{return "",nil,runtimeinstall.ErrInputProofUnavailable};ref=domain.SHA256}}
 if !runtimeinstall.InputProofHex(ref){return "",nil,runtimeinstall.ErrInputProofUnavailable}
 return p.Root,&runtimeinstall.DocumentAssemblyArtifact{InputClosureSHA256:ref,ProfileSHA256:p.SHA256,SourceSHA256:p.SourceSHA256},nil
}

// Re-resolve exact startup authority; a prior job/closure supplies no current
// package readiness. The caller still needs actual input and selected-engine
// preparation under the independent assembly lease before model-free compute.
func(s *Server)DocumentAssemblyProfile(ctx context.Context,profileSHA,sourceSHA string)(runtimeinstall.BundledDebianInputProofProfile,error){
 var zero runtimeinstall.BundledDebianInputProofProfile
 if ctx==nil||ctx.Err()!=nil||!s.documentRuntime.Configured()||s.documentRuntime.ProfileSHA256!=profileSHA{return zero,runtimeinstall.ErrInputProofUnavailable}
 profiles,e:=s.runtimeInputProofProfiles();if e!=nil{return zero,e}
 for _,p:=range profiles{if p.Purpose==runtimeinstall.DebianBinaryInputProofPurpose&&p.SHA256==profileSHA&&p.SourceSHA256==sourceSHA{return p,nil}}
 return zero,runtimeinstall.ErrInputProofUnavailable
}

// Current captured job authority is independent from package/profile validity.
// A cancellation or replacement cannot acquire staging or start new compute.
func(s *Server)CheckDocumentAssemblyCurrent(ctx context.Context,jobID string,selection runtimeinstall.DocumentAssemblyArtifact)error{
 if ctx==nil||ctx.Err()!=nil{return runtimeinstall.ErrInputProofOwnership}
 s.mu.RLock();defer s.mu.RUnlock()
 job:=s.runtimeJob
 if s.runtimeInstall.closed||s.runtimeInstall.sealed||job.ID!=jobID||job.OperationKind!=runtimeinstall.DocumentRuntimeAssemblyMode||job.CancelRequested||job.Status!="installing"||job.Execution==nil||job.Execution.DocumentAssembly==nil{return runtimeinstall.ErrInputProofOwnership}
 captured:=job.Execution.DocumentAssembly
 if captured.InputClosureSHA256!=selection.InputClosureSHA256||captured.ProfileSHA256!=selection.ProfileSHA256||captured.SourceSHA256!=selection.SourceSHA256||selection.Ref!=""||selection.CaptureSHA256!=""||selection.CaptureBytes!=0||selection.ReceiptSHA256!=""{return runtimeinstall.ErrInputProofOwnership}
 return nil
}

func(s *Server)DocumentAssemblyRecoverySelection(jobID string)(runtimeinstall.DocumentAssemblyArtifact,error){
 s.mu.RLock();defer s.mu.RUnlock()
 if s.runtimeJob.ID!=jobID||s.runtimeJob.OperationKind!=runtimeinstall.DocumentRuntimeAssemblyMode||s.runtimeJob.Execution==nil||s.runtimeJob.Execution.DocumentAssembly==nil{return runtimeinstall.DocumentAssemblyArtifact{},runtimeinstall.ErrInputProofOwnership}
 selection:=*s.runtimeJob.Execution.DocumentAssembly
 selection.Ref="";selection.CaptureSHA256="";selection.CaptureBytes=0;selection.ReceiptSHA256=""
 return selection,nil
}
