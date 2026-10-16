package api

import (
 "context"
 "errors"
 "math"
 "path/filepath"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/management"
 "github.com/lcfrai/luci/control-plane/internal/product"
 "github.com/lcfrai/luci/control-plane/internal/registry"
 "github.com/lcfrai/luci/control-plane/internal/resourcebudget"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

type debianInputProofChildIdentity struct {Ordinal int `json:"ordinal"`;PID uint32 `json:"pid"`;CreationTime uint64 `json:"creationTime"`}
type debianInputProofExecutionLease struct {
 Scope string `json:"scope"`;ID string `json:"id"`
 Identity runtimeinstall.DebianInputProofIdentity `json:"identity"`
 Actor management.ExecutionActorRef `json:"actor"`
 Resources backend.Resources `json:"resources"`
 Children []debianInputProofChildIdentity `json:"children,omitempty"`
 State string `json:"state"`
 DiskBytes uint64 `json:"diskBytes"`
 CreatedAt time.Time `json:"createdAt"`;WholeDeadline time.Time `json:"wholeDeadline"`
}
func validDebianInputProofLease(v debianInputProofExecutionLease) bool {
 if !checkHex(v.Scope,32) || !checkLogicalID(v.ID) || v.ID!=v.Identity.AllocationID || !runtimeinstall.ValidInputProofIdentity(v.Identity) || v.Identity.OwnerPID==0 || v.Resources.MemoryMiB!=192 || v.Resources.CPUMillis!=1000 || v.Actor.TeamID!="" || v.Actor.ProjectID!="" || len(v.Actor.UserID)>200 || v.Actor.Local&&v.Actor.UserID!="" || !v.Actor.Local&&v.Actor.UserID=="" || len(v.Children)>10 || v.DiskBytes==0 || v.DiskBytes>70<<20 || v.CreatedAt.IsZero() || !v.WholeDeadline.After(v.CreatedAt) || v.WholeDeadline.Sub(v.CreatedAt)>60*time.Second {return false}
 switch v.State{case "reserved","preparing","creating-suspended","running","settling","retiring","unknown","finished":default:return false}
 seen:=map[uint32]bool{};for i,c:=range v.Children{if c.Ordinal!=i || c.PID==0 || c.CreationTime==0 || seen[c.PID]{return false};seen[c.PID]=true};return true
}
func addDebianInputProofReservationView(v *executionReservationView,rows []debianInputProofExecutionLease){for _,r:=range rows{if r.State!="finished"{v.Count++;v.MemoryMiB+=r.Resources.MemoryMiB;v.CPUMillis+=r.Resources.CPUMillis}}}
func addDebianInputProofHostDemand(rows []debianInputProofExecutionLease,scope,id string,d *executionCapacityDemand)error{
 for _,r:=range rows {if !validDebianInputProofLease(r){return runtimeinstall.ErrInputProofOwnership};if r.State=="finished" || r.Scope==scope && r.ID==id{continue}
  memory:=uint64(r.Resources.MemoryMiB)<<20
  if d.NativeHostMemoryBytes>math.MaxUint64-memory || d.NativeHostCPUMillis>math.MaxInt64-int64(r.Resources.CPUMillis) || d.NativeHostDiskBytes>math.MaxUint64-r.DiskBytes{return errors.New("native proof demand overflow")}
  d.NativeHostMemoryBytes+=memory;d.NativeHostCPUMillis+=int64(r.Resources.CPUMillis);d.NativeHostDiskBytes+=r.DiskBytes
 };return nil
}
func (g *executionAdmission) inputProofDiskCapacity(ctx context.Context,d executionCapacityDemand,reserveMiB int)error{
 if d.NativeHostDiskBytes==0{return nil};if ctx.Err()!=nil{return ctx.Err()}
 // Every proof output/copy is confined to this SAME shared execution root.
 // This measures its actual volume; the sampler's sessions/state role is not
 // assumed to be the volume of an independently configured execution root.
 probe:=resourcebudget.ObserveDisk;if g.assemblyDiskProbe!=nil{probe=g.assemblyDiskProbe}
 disk,err:=probe(resourcebudget.DiskPath{Role:"debian-input-proof",Path:filepath.Dir(g.path)})
 required:=d.NativeHostDiskBytes+(uint64(reserveMiB)<<20)
 if err!=nil || disk.AvailableBytes==nil || disk.ObservedAt.IsZero() || time.Since(disk.ObservedAt)>5*time.Second || *disk.AvailableBytes<required{return errors.New("native proof volume capacity unavailable")};return ctx.Err()
}
func (g *executionAdmission) addInputProofAdmissionCapacity(ctx context.Context,rows []debianInputProofExecutionLease,scope,id string,d *executionCapacityDemand,reserve int)error{
 if err:=addDebianInputProofHostDemand(rows,scope,id,d);err!=nil{return err};return g.inputProofDiskCapacity(ctx,*d,reserve)
}
func evaluateDebianInputProofHostCapacity(s resourcebudget.Snapshot,p product.SwarmSchedulerPolicy,d executionCapacityDemand,now time.Time)error{
 h:=s.Host
 if !h.Fresh(now,5*time.Second) || h.Platform!="windows" || h.Memory.PhysicalAvailableBytes==nil || h.Memory.CommitAvailableBytes==nil || h.Memory.EffectiveTotalBytes==nil || h.CPU.UtilizationPercent==nil || h.CPU.EffectiveMillis==nil || *h.CPU.UtilizationPercent<0 || *h.CPU.UtilizationPercent>=50 || math.IsNaN(*h.CPU.UtilizationPercent) || math.IsInf(*h.CPU.UtilizationPercent,0){return errors.New("native proof host observation unavailable")}
 needed:=p.MemoryHeadroomBytes(*h.Memory.EffectiveTotalBytes)+d.PendingMemory+d.PendingHostLaunchBytes+d.NativeHostMemoryBytes
 if needed<8<<30{needed=8<<30}
 if *h.Memory.PhysicalAvailableBytes<needed || *h.Memory.CommitAvailableBytes<needed{return errors.New("native proof requires physical and commit reserve")}
 if d.CPU>math.MaxInt64-d.NativeHostCPUMillis || d.CPU+d.NativeHostCPUMillis>*h.CPU.EffectiveMillis*int64(100-p.HeadroomPercent)/100{return errors.New("native proof CPU capacity unavailable")};return nil
}
func (g *executionAdmission) reserveDebianInputProof(ctx context.Context,row debianInputProofExecutionLease,p product.SwarmSchedulerPolicy)error{
 if !validDebianInputProofLease(row){return runtimeinstall.ErrInputProofOwnership}
 return g.mutate(ctx,func(l *executionLeaseLedger)error{
  authority,err:=g.api.authorizeRuntimeInstaller(ctx,row.Actor);if err!=nil{return err};if !row.Actor.Local&&(authority.Policy.CheckMemoryMiB<192||authority.Policy.CheckCPUMillis<1000){return errCheckResourcePolicy};if err=g.api.deps.Product.CheckRuntimeInputProofCurrent(ctx,row.Identity);err!=nil{return err}
  for _,r:=range l.InputProofs{if r.Scope==row.Scope && r.ID==row.ID{return runtimeinstall.ErrInputProofOwnership}}
  for _,r:=range l.ImageAssemblies{if r.State!="finished"{return errors.New("retained exclusive image assembly blocks native proof admission")}}
  if executionLeaseCount(*l)>=4096{return errors.New("execution capacity history full")}
  s,now:=g.api.deps.ResourceSnapshot(),time.Now().UTC();d:=executionCapacityDemand{}
  nonnative:=len(l.Leases)>0
  for _,r:=range l.Checks{nonnative=nonnative||r.State!="finished"};for _,r:=range l.Previews{nonnative=nonnative||r.State!="finished"};for _,r:=range l.RuntimeImages{nonnative=nonnative||r.State!="finished"};for _,r:=range l.Documents{nonnative=nonnative||r.State!="finished"};for _,r:=range l.ImageAssemblies{nonnative=nonnative||r.State!="finished"}
  sessions,err:=g.api.deps.Store.List();if err!=nil{return err};for _,r:=range sessions{if r.Status!=registry.StatusExited{nonnative=true}}
  if nonnative {
   if !freshExecutionCapacity(s,now){return errors.New("retained execution demand needs real daemon evidence")}
   observed:=map[string]bool{}
   for _,c:=range s.Execution.Containers{m,cpu,ok:=executionWorkloadDemand(c,now);if !ok{return errors.New("retained workload demand unknown")};d.AllMemory+=m;d.CPU+=cpu;observed[c.SandboxID]=true}
   covered:=map[string]bool{}
   for _,r:=range l.Leases{covered[r.SessionID]=true;if r.SessionID==""||!observed[r.sandboxID()]{m:=uint64(r.Resources.MemoryMiB)<<20;d.PendingMemory+=m;d.AllMemory+=m;d.CPU+=int64(r.Resources.CPUMillis);d.PendingAllocations++}}
   for _,r:=range sessions{if r.Status!=registry.StatusExited && !observed[r.SandboxID] && !covered[r.ID]{return errors.New("legacy execution demand unknown")}}
   addWorkerHostLaunchCapacity(s,l.Leases,"","",g.api.deps.Backend.Kind(),&d)
   if err=addCheckCapacity(s,l.Checks,"","",&d);err!=nil{return err};if err=addPreviewCapacity(s,l.Previews,"","",&d);err!=nil{return err};if err=addRuntimeImageCapacity(l.RuntimeImages,"","",&d);err!=nil{return err};if err=addDocumentCapacity(s,l.Documents,"","",&d);err!=nil{return err};if err=g.addAssemblyAdmissionCapacity(ctx,s,l.ImageAssemblies,"","",&d,p.DiskReserveMiB);err!=nil{return err}
   // This operation creates NO daemon allocation. Validate the actual retained
   // guest demand without charging a phantom new sandbox/image copy.
   if err=validateInputProofRetainedDaemonCapacity(s,p,d,now);err!=nil{return err}
  }
  if err=addDebianInputProofHostDemand(append(append([]debianInputProofExecutionLease{},l.InputProofs...),row),"","",&d);err!=nil{return err}
  if err=evaluateDebianInputProofHostCapacity(s,p,d,now);err!=nil{return err};if err=g.inputProofDiskCapacity(ctx,d,p.DiskReserveMiB);err!=nil{return err}
  l.InputProofs=append(l.InputProofs,row);return nil
 })
}
func validateInputProofRetainedDaemonCapacity(s resourcebudget.Snapshot,p product.SwarmSchedulerPolicy,d executionCapacityDemand,now time.Time)error{
 if !freshExecutionCapacity(s,now){return errors.New("retained daemon observation unavailable")}
 if d.AllMemory>math.MaxUint64-p.MemoryHeadroomBytes(*s.Execution.TotalMemoryBytes)||d.AllMemory+p.MemoryHeadroomBytes(*s.Execution.TotalMemoryBytes)>*s.Execution.TotalMemoryBytes||d.CPU>*s.Execution.CPUMillis*int64(100-p.HeadroomPercent)/100{return errors.New("retained daemon demand exceeds capacity")}
 disk:=s.Execution.DaemonDisk
 if disk==nil||disk.AvailableBytes==nil||disk.ObservedAt.IsZero()||disk.ObservedAt.After(now)||now.Sub(disk.ObservedAt)>5*time.Second{return errors.New("retained daemon disk observation unavailable")}
 required:=uint64(p.DiskReserveMiB)<<20
 if d.PendingAllocations>0{estimate,err:=estimateExecutionCreateDisk(s.Execution);if err!=nil{return err};if estimate.Bytes==0||uint64(d.PendingAllocations)>(math.MaxUint64-required)/estimate.Bytes{return errors.New("retained disk overflow")};required+=uint64(d.PendingAllocations)*estimate.Bytes}
 if required>math.MaxUint64-d.PendingDiskBytes{return errors.New("retained disk overflow")};required+=d.PendingDiskBytes
 if *disk.AvailableBytes<required{return errors.New("retained daemon disk pressure")}
 return resourcebudget.ValidateHostStorage(s.Execution.HostStorage,true,required,now,5*time.Second)
}
func (g *executionAdmission) advanceDebianInputProof(ctx context.Context,id runtimeinstall.DebianInputProofIdentity,next string,child *debianInputProofChildIdentity)error{
 return g.mutate(ctx,func(l *executionLeaseLedger)error{for i:=range l.InputProofs{r:=&l.InputProofs[i];if r.Scope==g.scope && r.ID==id.AllocationID{if r.Identity!=id || r.State=="finished"{return runtimeinstall.ErrInputProofOwnership};r.State=next;if child!=nil{if child.Ordinal!=len(r.Children){return runtimeinstall.ErrInputProofOwnership};r.Children=append(r.Children,*child)};if !validDebianInputProofLease(*r){return runtimeinstall.ErrInputProofOwnership};return nil}};return runtimeinstall.ErrInputProofOwnership})
}
func(g *executionAdmission)requireDebianInputProofReservation(ctx context.Context,id runtimeinstall.DebianInputProofIdentity,ordinal int)error{
 l,e:=g.readReservationLedger(ctx);if e!=nil{return e};found:=false
 for _,r:=range l.InputProofs{if r.Scope==g.scope&&r.ID==id.AllocationID{if r.Identity!=id||r.State!="creating-suspended"||len(r.Children)!=ordinal+1||!time.Now().Before(r.WholeDeadline){return runtimeinstall.ErrInputProofOwnership};found=true}}
 if !found{return runtimeinstall.ErrInputProofOwnership}
 p,e:=g.api.deps.Product.SwarmSchedulerPolicy();if e!=nil{return e};d:=executionCapacityDemand{}
 if e=addDebianInputProofHostDemand(l.InputProofs,"","",&d);e!=nil{return e}
 // A recheck conservatively keeps the complete configured ceilings of every
 // other claim on the host, including observed containers. It never assumes
 // that a changing observed workload has released its configured reservation.
 nonnative:=false
 charge:=func(r backend.Resources){nonnative=true;d.PendingMemory+=uint64(r.MemoryMiB)<<20;d.CPU+=int64(r.CPUMillis)}
 snapshot:=g.api.deps.ResourceSnapshot()
 for _,r:=range l.Leases{charge(r.Resources);allowance:=r.HostLaunchBytes;if allowance==0{allowance=windowsDockerHostLaunchBytes(snapshot,g.api.deps.Backend.Kind())};d.PendingHostLaunchBytes+=allowance}
 for _,r:=range l.Checks{if r.State!="finished"{charge(r.Resources);allowance:=r.HostLaunchBytes;if allowance==0&&snapshot.Host.Platform=="windows"{allowance=2048<<20};d.PendingHostLaunchBytes+=allowance}}
 for _,r:=range l.Previews{if r.State!="finished"{charge(r.Resources);allowance:=r.HostLaunchBytes;if allowance==0&&snapshot.Host.Platform=="windows"{allowance=2048<<20};d.PendingHostLaunchBytes+=allowance}}
 for _,r:=range l.RuntimeImages{if r.State!="finished"{res:=r.Resources;res.MemoryMiB=max(res.MemoryMiB,r.AdmissionMemoryMiB);charge(res)}}
 for _,r:=range l.Documents{if r.State!="finished"{charge(r.Resources)}}
 for _,r:=range l.ImageAssemblies{if r.State!="finished"{charge(r.Resources);d.PendingHostLaunchBytes+=r.HostLaunchBytes}}
 if nonnative&&!freshExecutionCapacity(snapshot,time.Now().UTC()){return errors.New("retained demand needs real current daemon observation")}
 if e=evaluateDebianInputProofHostCapacity(snapshot,p,d,time.Now().UTC());e!=nil{return e};return g.inputProofDiskCapacity(ctx,d,p.DiskReserveMiB)
}
