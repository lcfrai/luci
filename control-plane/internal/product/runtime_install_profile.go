package product

import (
 "context"
 "path/filepath"
 "sync/atomic"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
 "github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// Populated only by a reviewed application-source packaging change. There is
// currently no redistributable tool/profile closure in this repository. Never
// fall back to installed diagnostic tools, PATH or a profile's self-hash.
var bundledDebianProofProfiles = []runtimeinstall.BundledDebianInputProofProfile{}

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
 for _,p:=range bundledDebianProofProfiles { if p.Purpose==id.Purpose && p.ID==id.ProfileID && p.SHA256==id.ProfileSHA256 && p.SourceSHA256==id.SourceSHA256 {root,err:=s.runtimeInputProofRoot();if err!=nil{return p,err};p.Root=root;p.Files=append([]runtimeinstall.DebianInputProofFile(nil),p.Files...);return p,nil} }
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
 if len(bundledDebianProofProfiles)==0{return runtimeinstall.BundledDebianInputProofProfile{},false,nil}
 if len(bundledDebianProofProfiles)!=1{return runtimeinstall.BundledDebianInputProofProfile{},true,runtimeinstall.ErrInputProofUnavailable}
 p:=bundledDebianProofProfiles[0]
 if _,ok:=runtimeinstall.DebianInputProofLimits(p.Purpose);!ok{return p,true,runtimeinstall.ErrInputProofUnavailable}
 if p.ID=="" || p.Protocol!=runtimeinstall.DebianInputProofProtocol || !runtimeinstall.InputProofHex(p.SHA256) || !runtimeinstall.InputProofHex(p.SourceSHA256) || len(p.Files)==0 || len(p.Files)>256 {return runtimeinstall.BundledDebianInputProofProfile{},true,runtimeinstall.ErrInputProofUnavailable}
 root,err:=s.runtimeInputProofRoot();if err!=nil{return p,true,err};p.Root=root;return p,true,nil
}
func(s *Server)runtimeInputProofRoot()(string,error){if !filepath.IsAbs(s.runtimeDir){return "",runtimeinstall.ErrInputProofUnavailable};root,err:=hostpath.ResolveOutside(s.projectRoot(),s.runtimeDir);if err!=nil{return "",err};return filepath.Join(root,"windows","debian-signature"),nil}
