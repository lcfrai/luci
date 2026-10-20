package product

import (
 "context"
 "io"
 "os"
 "path/filepath"
 "sync/atomic"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
 "github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// Legacy build-selected profiles remain available. Configured distributions
// instead supply an independently pinned descriptor through startup Options.
// Neither route falls back to PATH or a descriptor's own digest declaration.
var bundledDebianProofProfiles = []runtimeinstall.BundledDebianInputProofProfile{}

func(s *Server)DocumentRuntimeConfiguration()runtimeinstall.DocumentRuntimeConfiguration{return s.documentRuntime}
func(s *Server)runtimeInputProofProfiles()([]runtimeinstall.BundledDebianInputProofProfile,error){
 if !s.documentRuntime.Configured(){return bundledDebianProofProfiles,nil}
 if !s.documentRuntime.Valid(){return nil,runtimeinstall.ErrInputProofUnavailable}
 root,err:=s.runtimeInputProofRoot();if err!=nil{return nil,err}
 p,err:=readRuntimeInputProofDescriptor(root,"profile.json",s.documentRuntime.ProfileSHA256);if err!=nil{return nil,err}
 // The independently configured tool domain must be among the exact profile
 // bytes held again by the native owner's evidence acquisition before use.
 found:=false;for _,f:=range p.Files{if f.Role=="support:manifests/windows-input-proof-tool-closure-v1.json"{found=f.SHA256==s.documentRuntime.ToolDomainSHA256&&f.Bytes>0&&f.Bytes<=64<<10}}
 if !found{return nil,runtimeinstall.ErrInputProofUnavailable}
 profiles:=[]runtimeinstall.BundledDebianInputProofProfile{p}
 for _,f:=range p.Files{if f.Role==runtimeinstall.DebianSignatureProfileRole{
  if p.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose||f.RelativePath!=runtimeinstall.DebianSignatureProfilePath||f.Bytes<1||f.Bytes>64<<10{return nil,runtimeinstall.ErrInputProofUnavailable}
  child,e:=readRuntimeInputProofDescriptor(root,f.RelativePath,f.SHA256);if e!=nil{return nil,e}
  raw,e:=runtimeinstall.CanonicalDebianInputProofProfile(child)
  if e!=nil||int64(len(raw))!=f.Bytes||child.Purpose!=""||child.ID!=p.ID||child.Protocol!=p.Protocol||child.SourceSHA256!=p.SourceSHA256{return nil,runtimeinstall.ErrInputProofUnavailable}
  for _,nested:=range child.Files{if nested.Role==runtimeinstall.DebianSignatureProfileRole{return nil,runtimeinstall.ErrInputProofUnavailable}}
  profiles=append(profiles,child)
 }}
 return profiles,nil
}

// A selected stage is re-resolved through the independently pinned primary
// descriptor. Matching user-supplied profile fields never establish authority.
func(s *Server)ValidateDocumentRuntimeProfile(ctx context.Context,p runtimeinstall.BundledDebianInputProofProfile)error{
 if ctx==nil||ctx.Err()!=nil{return runtimeinstall.ErrInputProofUnavailable}
 profiles,err:=s.runtimeInputProofProfiles();if err!=nil{return err}
 expected,e:=runtimeinstall.CanonicalDebianInputProofProfile(p);if e!=nil{return e}
 for _,current:=range profiles{raw,re:=runtimeinstall.CanonicalDebianInputProofProfile(current);if re==nil&&current.SHA256==p.SHA256&&current.Root==p.Root&&current.DescriptorRelativePath==p.DescriptorRelativePath&&string(raw)==string(expected){return nil}}
 return runtimeinstall.ErrInputProofUnavailable
}

func readRuntimeInputProofDescriptor(root,relative,digest string)(runtimeinstall.BundledDebianInputProofProfile,error){
 var empty runtimeinstall.BundledDebianInputProofProfile
 if relative!="profile.json"&&relative!=runtimeinstall.DebianSignatureProfilePath{return empty,runtimeinstall.ErrInputProofUnavailable}
 name:=filepath.Join(root,filepath.FromSlash(relative))
 if _,err:=hostpath.Confine(root,name);err!=nil{return empty,runtimeinstall.ErrInputProofUnavailable}
 before,err:=os.Lstat(name);if err!=nil||!before.Mode().IsRegular()||before.Size()<1||before.Size()>64<<10{return empty,runtimeinstall.ErrInputProofUnavailable}
 file,err:=os.Open(name);if err!=nil{return empty,err};closed:=false;defer func(){if !closed{_ = file.Close()}}()
 opened,err:=file.Stat();if err!=nil||!os.SameFile(before,opened){return empty,runtimeinstall.ErrInputProofUnavailable}
 data,err:=io.ReadAll(io.LimitReader(file,(64<<10)+1));if err!=nil||int64(len(data))!=opened.Size(){return empty,runtimeinstall.ErrInputProofUnavailable}
 after,err:=file.Stat();if err!=nil||!os.SameFile(opened,after)||opened.Size()!=after.Size()||opened.ModTime()!=after.ModTime(){return empty,runtimeinstall.ErrInputProofUnavailable}
 closed=true;if err=file.Close();err!=nil{return empty,err}
 p,err:=runtimeinstall.DecodeDebianInputProofProfile(data,digest);if err!=nil{return empty,err}
 p.Root,p.DescriptorRelativePath=root,relative
 return p,nil
}

type runtimeInputProofOwnership struct {
 identity runtimeinstall.DebianInputProofIdentity
 invalid atomic.Bool
 next int
 adoption bool
 deadline time.Time
 projectsRoot,runtimeRoot string
}
type RuntimeInputProofResumePermit struct { owner *runtimeInputProofOwnership; ordinal int; finished atomic.Bool }
type RuntimeInputProofAdoptionPermit struct { owner *runtimeInputProofOwnership; target runtimeinstall.InputProofCAS; finished atomic.Bool }

func (p *RuntimeInputProofResumePermit) Current() bool { return p!=nil && p.owner!=nil && !p.finished.Load() && !p.owner.invalid.Load() && time.Now().Before(p.owner.deadline) }
func (p *RuntimeInputProofAdoptionPermit) Current() bool {return p!=nil&&p.owner!=nil&&!p.finished.Load()&&!p.owner.invalid.Load()&&time.Now().Before(p.owner.deadline)}
func (p *RuntimeInputProofResumePermit) Finish(acknowledged bool) error {
 if p==nil || p.owner==nil || p.finished.Swap(true) {return runtimeinstall.ErrInputProofOwnership}
 if !acknowledged || p.owner.invalid.Load() || !time.Now().Before(p.owner.deadline) {p.owner.invalid.Store(true);return runtimeinstall.ErrInputProofOwnership};return nil
}
func (p *RuntimeInputProofAdoptionPermit) Finish(observed runtimeinstall.InputProofCAS, err error) error {
 if p==nil || p.owner==nil || p.finished.Swap(true) {return runtimeinstall.ErrInputProofOwnership}
 if err!=nil || p.owner.invalid.Load() || !time.Now().Before(p.owner.deadline) || observed!=p.target { p.owner.invalid.Store(true);return runtimeinstall.ErrInputProofOwnership };return nil
}
func (s *Server) proofCurrentLocked(id runtimeinstall.DebianInputProofIdentity) bool {
 p:=s.runtimeInstall.proof
 return p!=nil && !p.invalid.Load() && p.projectsRoot==s.config.ProjectsDir && p.runtimeRoot==s.runtimeDir && time.Now().Before(p.deadline) && s.runtimeInstall.done!=nil && runtimeInstallActive(s.runtimeJob.Status) && !s.runtimeInstall.closed && !s.runtimeInstall.sealed && !s.runtimeJob.CancelRequested && s.runtimeJob.OperationKind==runtimeinstall.DebianInputProofKind && s.runtimeJob.ID==id.JobID && s.runtimeJob.InputProof!=nil && runtimeinstall.SameInputProofOperation(p.identity,id) && runtimeinstall.SameInputProofOperation(s.runtimeJob.InputProof.Identity,id)
}
func (s *Server) CheckRuntimeInputProofCurrent(ctx context.Context,id runtimeinstall.DebianInputProofIdentity) error {
 if ctx.Err()!=nil{return ctx.Err()};s.mu.RLock();defer s.mu.RUnlock();if !s.proofCurrentLocked(id){return runtimeinstall.ErrInputProofOwnership};return nil
}
func (s *Server) RuntimeInputProofProfile(ctx context.Context,id runtimeinstall.DebianInputProofIdentity)(runtimeinstall.BundledDebianInputProofProfile,error){
 if err:=s.CheckRuntimeInputProofCurrent(ctx,id);err!=nil{return runtimeinstall.BundledDebianInputProofProfile{},err}
 profiles,err:=s.runtimeInputProofProfiles();if err!=nil{return runtimeinstall.BundledDebianInputProofProfile{},err}
 for _,p:=range profiles { if p.Purpose==id.Purpose && p.ID==id.ProfileID && p.SHA256==id.ProfileSHA256 && p.SourceSHA256==id.SourceSHA256 {root,err:=s.runtimeInputProofRoot();if err!=nil{return p,err};p.Root=root;p.Files=append([]runtimeinstall.DebianInputProofFile(nil),p.Files...);return p,nil} }
 return runtimeinstall.BundledDebianInputProofProfile{},runtimeinstall.ErrInputProofUnavailable
}
func (s *Server) BeginRuntimeInputProofResume(ctx context.Context,id runtimeinstall.DebianInputProofIdentity,ordinal int)(*RuntimeInputProofResumePermit,error){
 if ctx.Err()!=nil{return nil,ctx.Err()};s.mu.Lock();defer s.mu.Unlock()
 limits,ok:=runtimeinstall.DebianInputProofLimits(id.Purpose)
 if !ok || !s.proofCurrentLocked(id) || ordinal<0 || ordinal>=limits.Children || s.runtimeInstall.proof.next!=ordinal {return nil,runtimeinstall.ErrInputProofOwnership}
 p:=s.runtimeInstall.proof;p.next++;return &RuntimeInputProofResumePermit{owner:p,ordinal:ordinal},nil
}
func (s *Server) BeginRuntimeInputProofAdoption(ctx context.Context,id runtimeinstall.DebianInputProofIdentity,target runtimeinstall.InputProofCAS)(*RuntimeInputProofAdoptionPermit,error){
 if ctx.Err()!=nil{return nil,ctx.Err()};s.mu.Lock();defer s.mu.Unlock()
 limits,ok:=runtimeinstall.DebianInputProofLimits(id.Purpose)
 if !ok || !s.proofCurrentLocked(id) || s.runtimeInstall.proof.next!=limits.Children || s.runtimeInstall.proof.adoption || target.Revision==0 || !runtimeinstall.InputProofHex(target.Digest){return nil,runtimeinstall.ErrInputProofOwnership}
 p:=s.runtimeInstall.proof;p.adoption=true;return &RuntimeInputProofAdoptionPermit{owner:p,target:target},nil
}

// The server's closed bundled plan, never request JSON, selects this component.
func (s *Server) selectedRuntimeInputProofProfile()(runtimeinstall.BundledDebianInputProofProfile,bool,error){
 profiles,err:=s.runtimeInputProofProfiles();if err!=nil{return runtimeinstall.BundledDebianInputProofProfile{},true,err}
 if len(profiles)==0{return runtimeinstall.BundledDebianInputProofProfile{},false,nil}
 if len(profiles)>2{return runtimeinstall.BundledDebianInputProofProfile{},true,runtimeinstall.ErrInputProofUnavailable}
 p:=profiles[0]
 if p.Purpose==runtimeinstall.DebianBinaryInputProofPurpose&&s.documentRuntime.Configured(){
  if len(profiles)!=2||profiles[1].Purpose!=""{return p,true,runtimeinstall.ErrInputProofUnavailable}
  // This chooses a requested stage only. The API must independently resolve
  // current fact records, guards, source/CAS, host and native authority.
  s.mu.RLock();prior:=s.runtimeJob.InputProof;ready:=prior!=nil&&prior.Phase=="facts-recorded"&&prior.CleanupConfirmed&&!prior.ResourcesHeld&&!prior.PublicationUnknown&&prior.Identity.ProfileSHA256==profiles[1].SHA256;s.mu.RUnlock()
  if !ready{p=profiles[1]}
 }
 if _,ok:=runtimeinstall.DebianInputProofLimits(p.Purpose);!ok{return p,true,runtimeinstall.ErrInputProofUnavailable}
 if p.ID=="" || p.Protocol!=runtimeinstall.DebianInputProofProtocol || !runtimeinstall.InputProofHex(p.SHA256) || !runtimeinstall.InputProofHex(p.SourceSHA256) || len(p.Files)==0 || len(p.Files)>256 {return runtimeinstall.BundledDebianInputProofProfile{},true,runtimeinstall.ErrInputProofUnavailable}
 root,err:=s.runtimeInputProofRoot();if err!=nil{return p,true,err};p.Root=root;return p,true,nil
}
func(s *Server)runtimeInputProofRoot()(string,error){if !filepath.IsAbs(s.runtimeDir){return "",runtimeinstall.ErrInputProofUnavailable};root,err:=hostpath.ResolveOutside(s.projectRoot(),s.runtimeDir);if err!=nil{return "",err};return filepath.Join(root,"windows","debian-signature"),nil}
