package api

import (
 "context"
 "errors"
 "os"
 "sync"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/product"
 "github.com/lcfrai/luci/control-plane/internal/registry"
)

// Pre-staging is the SAME allocation/serial lease, not an uncharged helper.
// It reserves host originals before Open and is atomically converted to the
// normal container reservation only after positive original-set Close.
type documentAssemblyStageBinding struct{
 InputsDigest string `json:"inputsDigest"`
 Demand backend.DocumentDomainInputDemand `json:"demand"`
 DirectoryBytes uint64 `json:"directoryBytes"`
 State string `json:"state"` // reserved, opening, open, unknown, closed
 WriterHandles int `json:"writerHandles"`
 WriterBufferBytes int `json:"writerBufferBytes"`
}
func validDocumentAssemblyStage(s documentAssemblyStageBinding)bool{
 d:=s.Demand
 if s.WriterHandles!=1||s.WriterBufferBytes!=64<<10||d.DeclaredBytes<=0||d.OriginalHandles<=d.Inputs||s.DirectoryBytes<uint64(d.DeclaredBytes){return false}
 if !assemblyCatalogHex(s.InputsDigest)||d.Inputs<1||d.Inputs>backend.DocumentDomainInputLimit||d.OriginalHandles<2||d.OriginalHandles>backend.DocumentDomainInputHandleLimit||d.DeclaredBytes<0||d.DeclaredBytes>backend.DocumentDomainInputTotalBytes||d.ReadCalls!=backend.DocumentDomainInputReadCalls||d.ReadBytes!=backend.DocumentDomainInputReadBytes||d.BlockBytes!=backend.DocumentDomainInputBlockBytes||d.ReadBufferBytes!=2*backend.DocumentDomainInputBlockBytes||d.AcquisitionWorkers!=1||d.RetirementWorkers!=1||d.CancellationWorkers!=1||d.CancellationCallbacks!=d.Inputs+2||s.DirectoryBytes<1{return false}
 switch s.State{case "reserved","opening","open","unknown","closed":return true};return false
}
type liveDocumentAssemblyStage struct{mu sync.Mutex;owner backend.DocumentDomainInputs;writer *os.File;opening,closing,retiring,writing,writerUnknown bool}
var liveDocumentAssemblyStages sync.Map
func(g *executionAdmission)assemblyStageKey(id string)string{return g.path+"\x00"+g.scope+"\x00"+id}

func(g *executionAdmission)reserveDocumentAssemblyStage(ctx context.Context,row documentImageAssemblyLease,policy product.SwarmSchedulerPolicy,current func(context.Context)error)error{
 if row.Stage==nil||row.Stage.State!="reserved"||row.State!="staging"||!validDocumentImageAssemblyLease(row)||current==nil||g.api.deps.ResourceSnapshot==nil||g.api.deps.Store==nil{return backend.ErrUnsupported}
 return g.mutate(ctx,func(l *executionLeaseLedger)error{
  if e:=current(ctx);e!=nil{return e};authority,e:=g.api.authorizeRuntimeInstaller(ctx,row.Actor);if e!=nil{return e}
  if !row.Actor.Local&&(authority.Policy.CheckMemoryMiB<2048||authority.Policy.CheckCPUMillis<1000){return errCheckResourcePolicy}
  if assemblyOtherClaims(*l,g.scope,row.ID){return errDocumentImageAssemblySerial}
  for _,old:=range l.ImageAssemblies{if old.Scope==g.scope&&old.ID==row.ID{return backend.ErrLaunchFenceOwnership}}
  snap:=g.api.deps.ResourceSnapshot();now:=time.Now().UTC()
  if !freshExecutionCapacity(snap,now)||snap.Execution.EngineID!=row.Identity.EngineID||len(snap.Execution.Containers)!=0||!now.Before(row.ComputeDeadline){return backend.ErrUnavailable}
  sessions,e:=g.api.deps.Store.List();if e!=nil{return e};for _,session:=range sessions{if session.Status!=registry.StatusExited{return errDocumentImageAssemblySerial}}
  var demand executionCapacityDemand
  rows:=append(append([]documentImageAssemblyLease(nil),l.ImageAssemblies...),row)
  if e=g.addAssemblyAdmissionCapacity(ctx,snap,rows,g.scope,row.ID,&demand,policy.DiskReserveMiB);e!=nil{return e}
  // The prospective stage is skipped above only to avoid duplicate resource
  // counting. Its actual host staging+capture disk remains explicitly charged.
  demand.PendingDiskBytes+=row.Stage.DirectoryBytes+row.CaptureDiskBytes;demand.HostLaunchBytes=row.HostLaunchBytes
  if code,message:=evaluateExecutionCapacity(snap,policy,row.Resources,demand,now);code!=""{return errors.New(message)}
  if executionLeaseCount(*l)>=4096{return backend.ErrUnavailable}
  l.ImageAssemblies=append(l.ImageAssemblies,row);return nil
 })
}
func(g *executionAdmission)advanceDocumentAssemblyStage(ctx context.Context,id backend.DocumentImageAssemblyIdentity,from,to string)error{
 return g.mutate(ctx,func(l *executionLeaseLedger)error{
  i,e:=assemblyLeaseIndex(l,id);if e!=nil{return e};row:=&l.ImageAssemblies[i]
  if row.State!="staging"||row.Stage==nil||row.Stage.State!=from{return backend.ErrLaunchFenceOwnership}
  next:=*row.Stage;next.State=to;if !validDocumentAssemblyStage(next){return backend.ErrUnsupported};row.Stage=&next;row.UpdatedAt=time.Now().UTC();return nil
 })
}

func(g *executionAdmission)closeDocumentAssemblyStage(ctx context.Context,row documentImageAssemblyLease,live *liveDocumentAssemblyStage)error{
 if live==nil{return backend.ErrUnavailable}
 live.mu.Lock();live.retiring=true;if live.opening||live.closing||live.writing||live.writerUnknown{live.mu.Unlock();return backend.ErrUnavailable};owner:=live.owner;live.closing=true;live.mu.Unlock()
 var e error;if owner!=nil{e=owner.Close()}
 live.mu.Lock();live.closing=false;if e==nil{live.owner=nil};live.mu.Unlock();if e!=nil{return e}
 ledger,e:=g.readReservationLedger(ctx);if e!=nil{return e};i,e:=assemblyLeaseIndex(&ledger,row.Identity);if e!=nil{return e};actual:=ledger.ImageAssemblies[i]
 if actual.Stage==nil{return backend.ErrLaunchFenceOwnership};if actual.Stage.State=="closed"{return nil}
 if e=g.advanceDocumentAssemblyStage(ctx,row.Identity,actual.Stage.State,"closed");e!=nil{return e}
 liveDocumentAssemblyStages.Delete(g.assemblyStageKey(row.ID));return nil
}
