//go:build windows

package api

import (
 "context"
 "errors"
 "io"
 "runtime"
 "strings"
 "sync"
 "sync/atomic"
 "time"
 "unsafe"

 "golang.org/x/sys/windows"
 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/product"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

var proofInitializeAttributes=proofKernel.NewProc("InitializeProcThreadAttributeList")
var proofUpdateAttributes=proofKernel.NewProc("UpdateProcThreadAttribute")
var proofDeleteAttributes=proofKernel.NewProc("DeleteProcThreadAttributeList")
var proofIsInJob=proofKernel.NewProc("IsProcessInJob")
var proofGetAffinity=proofKernel.NewProc("GetProcessAffinityMask")
var proofReOpenFile=proofKernel.NewProc("ReOpenFile")

// Captured once by each concrete production owner. Only *_windows_test.go
// supplies inert kernel scheduling outcomes; no profile/request/table setter.
type debianProofWindowsCalls struct {
 CreateJob func(*windows.SecurityAttributes,*uint16)(windows.Handle,error)
 SetJob func(windows.Handle,uint32,uintptr,uint32)(int,error)
 QueryJob func(windows.Handle,int32,uintptr,uint32,*uint32)error
 TerminateJob func(windows.Handle,uint32)error
 Close func(windows.Handle)error
 Pipe func(*windows.Handle,*windows.Handle,*windows.SecurityAttributes,uint32)error
 HandleInfo func(windows.Handle,uint32,uint32)error
 CreateProcess func(*uint16,*uint16,*windows.SecurityAttributes,*windows.SecurityAttributes,bool,uint32,*uint16,*uint16,*windows.StartupInfo,*windows.ProcessInformation)error
 Times func(windows.Handle,*windows.Filetime,*windows.Filetime,*windows.Filetime,*windows.Filetime)error
 Read func(windows.Handle,[]byte,*uint32,*windows.Overlapped)error
 Wait func(windows.Handle,uint32)(uint32,error)
 Exit func(windows.Handle,*uint32)error
 Resume func(windows.Handle)(uint32,error)
 Affinity func(*uintptr,*uintptr)error
 InJob func(windows.Handle,windows.Handle)(bool,error)
 InitAttributes func(unsafe.Pointer,*uintptr)error
 UpdateAttributes func(unsafe.Pointer,uintptr,unsafe.Pointer,uintptr)error
 DeleteAttributes func(unsafe.Pointer)
 FileCreate func(*uint16,uint32,uint32,*windows.SecurityAttributes,uint32,uint32,windows.Handle)(windows.Handle,error)
 FileInfo func(windows.Handle,*windows.ByHandleFileInformation)error
 FileWrite func(windows.Handle,[]byte,*uint32,*windows.Overlapped)error
 FileFlush func(windows.Handle)error
 FileReOpen func(windows.Handle)(windows.Handle,error)
}
func newDebianProofWindowsCalls()*debianProofWindowsCalls{return &debianProofWindowsCalls{
 CreateJob:windows.CreateJobObject,SetJob:windows.SetInformationJobObject,QueryJob:windows.QueryInformationJobObject,TerminateJob:windows.TerminateJobObject,Close:windows.CloseHandle,Pipe:windows.CreatePipe,HandleInfo:windows.SetHandleInformation,CreateProcess:windows.CreateProcess,Times:windows.GetProcessTimes,Read:windows.ReadFile,Wait:windows.WaitForSingleObject,Exit:windows.GetExitCodeProcess,Resume:windows.ResumeThread,
 Affinity:func(a,b *uintptr)error{r,_,_:=proofGetAffinity.Call(uintptr(windows.CurrentProcess()),uintptr(unsafe.Pointer(a)),uintptr(unsafe.Pointer(b)));if r==0{return errors.New("original affinity unavailable")};return nil},
 InJob:func(p,j windows.Handle)(bool,error){var in int32;r,_,_:=proofIsInJob.Call(uintptr(p),uintptr(j),uintptr(unsafe.Pointer(&in)));if r==0{return false,errors.New("original membership unavailable")};return in!=0,nil},
 InitAttributes:func(p unsafe.Pointer,n *uintptr)error{r,_,_:=proofInitializeAttributes.Call(uintptr(p),2,0,uintptr(unsafe.Pointer(n)));if r==0{return errors.New("startup attributes unavailable")};return nil},
 UpdateAttributes:func(p unsafe.Pointer,k uintptr,v unsafe.Pointer,n uintptr)error{r,_,_:=proofUpdateAttributes.Call(uintptr(p),0,k,uintptr(v),n,0,0);if r==0{return errors.New("startup attribute unavailable")};return nil},
 DeleteAttributes:func(p unsafe.Pointer){proofDeleteAttributes.Call(uintptr(p))},
 FileCreate:windows.CreateFile,FileInfo:windows.GetFileInformationByHandle,FileWrite:windows.WriteFile,FileFlush:windows.FlushFileBuffers,
 FileReOpen:func(source windows.Handle)(windows.Handle,error){h,_,e:=proofReOpenFile.Call(uintptr(source),uintptr(windows.GENERIC_READ),uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE),uintptr(windows.FILE_FLAG_OPEN_REPARSE_POINT));handle:=windows.Handle(h);if validDebianProducedHandle(handle){return handle,nil};if e!=windows.ERROR_SUCCESS{return handle,e};return handle,errors.New("original file reopen unavailable")},
}}

type debianProofJob struct {mu sync.Mutex;handle windows.Handle;settled,closing,terminating,terminationStarted,retirementRequested bool;borrowers int;ordinal int;generation,lastTerminationGeneration uint8}
type debianProofTerminationBorrow struct {job *debianProofJob;handle windows.Handle;generation uint8}
type debianProofProducedFile struct {handle windows.Handle;path string;info windows.ByHandleFileInformation;readOnly,identityKnown bool}
type debianProofLifetime struct {handles []windows.Handle; readersSettled bool;processSettled bool;stdout,stderr chan proofPipeRead;outResult,errResult *proofPipeRead;pid uint32;creation uint64}
type debianProofNativeOwner struct {
 calls *debianProofWindowsCalls
 mu sync.Mutex
 jobs []*debianProofJob
 lifetimes []*debianProofLifetime
 produced []*debianProofProducedFile
 stopped atomic.Bool
 stopOnce sync.Once
 stopWatch chan struct{}
 watchDone chan struct{}
 observer *debianProofObserver
 began,workDeadline,wholeDeadline time.Time
 cancel context.CancelFunc
}
func debianProducerPlatformSupported()bool{return runtime.GOARCH=="amd64"}
func(o *debianProofNativeOwner)workLimit()time.Time{return o.workDeadline}
func(o *debianProofNativeOwner)positiveCompletion(results []debianProofChildResult)bool{
 if o==nil || o.stopped.Load() || len(results)!=10 || len(o.jobs)!=10 || len(o.lifetimes)!=10 || !time.Now().Before(o.workDeadline){return false}
 select{case<-o.watchDone:default:return false}
 o.observer.mu.Lock();closed:=o.observer.closed&&o.observer.err==nil;o.observer.mu.Unlock();if !closed{return false}
 for i,j:=range o.jobs{j.mu.Lock();closed:=j.settled&&j.handle==0&&j.borrowers==0&&j.ordinal==i;j.mu.Unlock();if !closed{return false}}
 for _,l:=range o.lifetimes{if !l.processSettled||!l.readersSettled{return false};for _,h:=range l.handles{if h!=0{return false}}}
 for i,r:=range results{if r.Ordinal!=i||r.PID==0||r.CreationTime==0||o.lifetimes[i].pid!=r.PID||o.lifetimes[i].creation!=r.CreationTime||!r.OriginalHandleOwned||!r.ExitConfirmed||!r.JobEmpty||!r.HandlesClosed||!r.CaptureSettled{return false}}
 return true
}
type debianProofChildResult struct {
 Ordinal int `json:"ordinal"`
 PID uint32 `json:"pid"`;CreationTime uint64 `json:"creationTime"`
 Started,Ended time.Time
 Exit uint32 `json:"exit"`
 OriginalHandleOwned,ExitConfirmed,JobEmpty,HandlesClosed,CaptureSettled bool
 Admission debianNativeSample
 Stdout,Stderr []byte `json:"-"`
}
func debianProducerHostIdentity()(uint32,uint64,error){
 var create,exit,kernel,user windows.Filetime
 if err:=windows.GetProcessTimes(windows.CurrentProcess(),&create,&exit,&kernel,&user);err!=nil{return 0,0,err}
 return windows.GetCurrentProcessId(),uint64(create.HighDateTime)<<32|uint64(create.LowDateTime),nil
}
func newDebianProofNativeOwner(ctx context.Context)*debianProofNativeOwner{
 began:=time.Now();_,cancel:=context.WithCancel(ctx)
 o:=&debianProofNativeOwner{calls:newDebianProofWindowsCalls(),began:began,workDeadline:began.Add(40*time.Second),wholeDeadline:began.Add(60*time.Second),cancel:cancel,stopWatch:make(chan struct{}),watchDone:make(chan struct{}),observer:newDebianProofObserver()}
 go func(){defer close(o.watchDone);ticker:=time.NewTicker(100*time.Millisecond);defer ticker.Stop();primingUntil:=began.Add(2*time.Second)
  for{select{case<-o.stopWatch:return;case<-ctx.Done():o.retire();return;case<-ticker.C:
   s:=o.observer.latest();if !time.Now().Before(o.workDeadline) || time.Now().After(primingUntil) && (!s.Known || time.Since(s.At)>time.Second || s.Physical<4<<30 || s.Commit<4<<30 || s.CPU>=50){o.retire();return}
  }}
 }();return o
}
func(o *debianProofNativeOwner) retire(){
 o.stopped.Store(true);o.cancel();o.mu.Lock();jobs:=append([]*debianProofJob(nil),o.jobs...);o.mu.Unlock()
 // Jobs remain original and open until watchDone; never PID reopen or close
 // while a concurrent emergency retirement can still use the handle.
 for _,j:=range jobs{o.requestJobRetirement(j)}
}
// Called under this exact original Job's mutex. Each Job has at most two
// generations: before Create acknowledgement and one nonzero original PI ACK.
// Repeated watchdog/wait/cancel observations cannot schedule the same generation.
func takeDebianProofTerminationLocked(j *debianProofJob)*debianProofTerminationBorrow{
 if !j.retirementRequested||j.handle==0||j.closing||j.terminating||j.terminationStarted&&j.lastTerminationGeneration==j.generation{return nil}
 j.borrowers++;j.terminating=true;j.terminationStarted=true;j.lastTerminationGeneration=j.generation
 return &debianProofTerminationBorrow{job:j,handle:j.handle,generation:j.generation}
}
func(o *debianProofNativeOwner)requestJobRetirement(j *debianProofJob){
 j.mu.Lock();j.retirementRequested=true;borrow:=takeDebianProofTerminationLocked(j);j.mu.Unlock()
 if borrow!=nil{go o.terminateOriginalJob(borrow)}
}
func(o *debianProofNativeOwner)terminateOriginalJob(borrow *debianProofTerminationBorrow){
 _ = o.calls.TerminateJob(borrow.handle,1)
 j:=borrow.job;j.mu.Lock();j.borrowers--;j.terminating=false
 // Consume a late original ACK atomically with relinquishing the old borrower.
 // No closer can see zero borrowers between these exact two generations. This
 // is a finite handoff, not polling, a new Job, or replacement termination loop.
 next:=takeDebianProofTerminationLocked(j);j.mu.Unlock()
 if next!=nil{o.terminateOriginalJob(next)}
}
func(o *debianProofNativeOwner)acknowledgeOriginalCreate(ordinal int,pi windows.ProcessInformation){
 if pi.Process==0&&pi.Thread==0{return}
 o.mu.Lock();var owned *debianProofJob;for _,j:=range o.jobs{if j.ordinal==ordinal{owned=j;break}};o.mu.Unlock()
 if owned==nil{return}
 owned.mu.Lock();owned.generation=1;borrow:=takeDebianProofTerminationLocked(owned);owned.mu.Unlock()
 if borrow!=nil{go o.terminateOriginalJob(borrow)}
}
func(o *debianProofNativeOwner) settle()bool{
 o.stopOnce.Do(func(){close(o.stopWatch)});left:=time.Until(o.wholeDeadline);if left<=0{return false}
 select{case<-o.watchDone:case<-time.After(left):return false}
 observer:=o.observer.settle(o.wholeDeadline);ok:=observer
 o.mu.Lock();jobs:=append([]*debianProofJob(nil),o.jobs...);o.mu.Unlock()
 for _,life:=range o.lifetimes{
  if len(life.handles)>=8 && life.handles[6]!=0 {wait:=max(time.Duration(0),time.Until(minProofTime(o.wholeDeadline,time.Now().Add(5*time.Second))));state,e:=o.calls.Wait(life.handles[6],uint32(wait/time.Millisecond));life.processSettled=e==nil&&state==windows.WAIT_OBJECT_0}
  if life.stdout!=nil{_ = life.finishReaders(o.wholeDeadline)}
  for i,h:=range life.handles{if h==0{continue};if (i==0||i==2)&&!life.readersSettled||i>=6&&!life.processSettled{ok=false;continue};if o.calls.Close(h)!=nil{ok=false}else{life.handles[i]=0}}
 }
 if ok{for _,j:=range jobs{j.mu.Lock();h:=j.handle;if h==0{j.mu.Unlock();continue};if j.borrowers!=0||j.closing{j.mu.Unlock();ok=false;continue};j.closing=true;j.mu.Unlock()
  empty:=proofJobEmpty(o.calls,h);var e error;if empty{e=o.calls.Close(h)}else{e=errors.New("Job is not empty")};j.mu.Lock();if e!=nil{ok=false;j.closing=false}else{j.handle=0;j.settled=true};j.mu.Unlock()
 }}
 return ok&&time.Now().Before(o.wholeDeadline)
}
func(o *debianProofNativeOwner)readProduced(path string,maximum int64)([]byte,error){
 name,e:=windows.UTF16PtrFromString(path);if e!=nil{return nil,e}
 handle,e:=o.calls.FileCreate(name,windows.GENERIC_READ,windows.FILE_SHARE_READ,nil,windows.OPEN_EXISTING,windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_NORMAL,0)
 original:=&debianProofProducedFile{handle:handle,path:path,readOnly:true};if validDebianProducedHandle(handle){o.produced=append(o.produced,original)};if e!=nil{return nil,e};if !validDebianProducedHandle(handle){return nil,errors.New("produced original handle absent")}
 var before,after windows.ByHandleFileInformation
 if e=o.calls.FileInfo(handle,&before);e!=nil{return nil,e}
 size:=int64(uint64(before.FileSizeHigh)<<32|uint64(before.FileSizeLow))
 if maximum<=0||maximum>1<<20||size<0||size>maximum||before.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY)!=0{return nil,errors.New("produced file type or bound refused")}
 data:=make([]byte,0,size);buffer:=make([]byte,4096);remaining:=maximum+1
 for{if remaining<=0{return nil,errors.New("produced original read debit exhausted")};request:=min(int64(len(buffer)),remaining);remaining-=request;var n uint32;e=o.calls.Read(handle,buffer[:int(request)],&n,nil);if int64(n)>request{return nil,errors.New("produced original read acknowledgement invalid")};if n>0{if int64(len(data))+int64(n)>maximum{return nil,errors.New("produced file grew beyond bound")};data=append(data,buffer[:n]...)};if e==windows.ERROR_HANDLE_EOF{break};if e!=nil{return nil,e};if n==0{break}}
 if e=o.calls.FileInfo(handle,&after);e!=nil||!equalDebianProducedIdentity(before,after)||int64(len(data))!=size{return nil,errors.New("produced original file identity changed")};original.info,original.identityKnown=after,true;return data,nil
}
func equalDebianProducedIdentity(a,b windows.ByHandleFileInformation)bool{return a.VolumeSerialNumber==b.VolumeSerialNumber&&a.FileIndexHigh==b.FileIndexHigh&&a.FileIndexLow==b.FileIndexLow&&a.FileAttributes==b.FileAttributes&&a.FileSizeHigh==b.FileSizeHigh&&a.FileSizeLow==b.FileSizeLow&&a.CreationTime==b.CreationTime&&a.LastWriteTime==b.LastWriteTime}
func validDebianProducedHandle(h windows.Handle)bool{return h!=0&&h!=windows.InvalidHandle}
// ReOpenFile makes a distinct read-open of the CREATED original before WRITE
// Close. Unlike DuplicateHandle, its open-object WriteAccess is false. It keeps
// identity and DELETE exclusion, but permits writes until560 acquires its
// restrictive guard and validates exact expected bytes. No path-only handoff.
func(o *debianProofNativeOwner)writeEvidenceBytes(path string,data []byte,maximum int)error{
 if maximum<=0||maximum>256<<10||len(data)>maximum||!time.Now().Before(o.workDeadline){return errors.New("native evidence output bound")};name,e:=windows.UTF16PtrFromString(path);if e!=nil{return e}
 h,e:=o.calls.FileCreate(name,windows.GENERIC_READ|windows.GENERIC_WRITE,windows.FILE_SHARE_READ,nil,windows.CREATE_NEW,windows.FILE_ATTRIBUTE_NORMAL,0);writer:=&debianProofProducedFile{handle:h,path:path};if validDebianProducedHandle(h){o.produced=append(o.produced,writer)};if e!=nil{return e};if !validDebianProducedHandle(h){return errors.New("created original file handle absent")}
 for offset:=0;offset<len(data);{if !time.Now().Before(o.workDeadline){return errors.New("evidence write deadline")};var n uint32;e=o.calls.FileWrite(h,data[offset:],&n,nil);if e!=nil{return e};if n==0||uint64(n)>uint64(len(data)-offset){return errors.New("evidence write unacknowledged")};offset+=int(n)}
 if e=o.calls.FileFlush(h);e!=nil{return e};var info windows.ByHandleFileInformation;if e=o.calls.FileInfo(h,&info);e!=nil{return e};if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY)!=0||uint64(info.FileSizeHigh)<<32|uint64(info.FileSizeLow)!=uint64(len(data)){return errors.New("created original file identity unavailable")}
 alias,e:=o.calls.FileReOpen(h);read:=&debianProofProducedFile{handle:alias,path:path,readOnly:true};if validDebianProducedHandle(alias){o.produced=append(o.produced,read)};if e!=nil{return e};if !validDebianProducedHandle(alias){return errors.New("independent original read open absent")}
 var actual windows.ByHandleFileInformation;if e=o.calls.FileInfo(alias,&actual);e!=nil||!equalDebianProducedIdentity(info,actual){return errors.New("original read alias identity mismatch")};read.info,read.identityKnown=actual,true
 if e=o.calls.Close(h);e!=nil{return e};writer.handle=0;return nil
}
// Backend originals already hold this fixed namespace. Compare its FileID to
// still-held creation/read aliases; exact content hashes alone cannot do this.
func(o *debianProofNativeOwner)adoptProduced(ctx context.Context,guard backend.DocumentImageAssemblyEvidenceGuard,objects []backend.DocumentImageAssemblyEvidenceObject)error{
 if guard==nil||guard.CheckCurrent(ctx)!=nil{return errors.New("produced original guard unavailable")};expected:=map[string]backend.DocumentImageAssemblyEvidenceObject{};for _,v:=range objects{key:=strings.ToLower(v.Path);if _,exists:=expected[key];exists{return errors.New("duplicate produced original purpose")};expected[key]=v}
 originals:=append([]*debianProofProducedFile(nil),o.produced...);checked:=map[string]windows.ByHandleFileInformation{}
 for _,original:=range originals{if original.handle==0{continue};v,exists:=expected[strings.ToLower(original.path)];if !exists||!original.readOnly||!original.identityKnown||ctx.Err()!=nil||!time.Now().Before(o.workDeadline){return errors.New("produced original ownership unavailable")}
  var current windows.ByHandleFileInformation;if o.calls.FileInfo(original.handle,&current)!=nil||!equalDebianProducedIdentity(original.info,current)||uint64(current.FileSizeHigh)<<32|uint64(current.FileSizeLow)!=uint64(v.Bytes){return errors.New("produced original identity changed")};key:=strings.ToLower(original.path);if previous,exists:=checked[key];exists{if !equalDebianProducedIdentity(previous,current){return errors.New("conflicting original aliases for one guarded path")};continue};checked[key]=current
  name,e:=windows.UTF16PtrFromString(original.path);if e!=nil{return e};h,e:=o.calls.FileCreate(name,windows.GENERIC_READ,windows.FILE_SHARE_READ,nil,windows.OPEN_EXISTING,windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_NORMAL,0);joined:=&debianProofProducedFile{handle:h,path:original.path,readOnly:true};if validDebianProducedHandle(h){o.produced=append(o.produced,joined)};if e!=nil{return e};if !validDebianProducedHandle(h){return errors.New("guarded namespace original handle absent")}
  var info windows.ByHandleFileInformation;if o.calls.FileInfo(h,&info)!=nil||!equalDebianProducedIdentity(current,info){return errors.New("guarded namespace is not the created original")};joined.info,joined.identityKnown=info,true
 }
 if len(checked)!=len(expected){return errors.New("guarded file lacks an original produced alias")};if e:=guard.CheckCurrent(ctx);e!=nil{return e};if !o.closeProduced(){return errors.New("produced original alias close unknown")};return nil
}
func(o *debianProofNativeOwner)closeProduced()bool{ok:=true;for _,f:=range o.produced{if f.handle!=0{if o.calls.Close(f.handle)!=nil{ok=false}else{f.handle=0}}};return ok}
func(o *debianProofNativeOwner)writeBytes(path string,data []byte,maximum int)error{
 if maximum<=0||maximum>256<<10||len(data)>maximum{return errors.New("native proof output bound")};name,e:=windows.UTF16PtrFromString(path);if e!=nil{return e}
 h,e:=windows.CreateFile(name,windows.GENERIC_WRITE,0,nil,windows.CREATE_NEW,windows.FILE_ATTRIBUTE_NORMAL,0);if e!=nil{return e};file:=&debianProofProducedFile{handle:h};o.produced=append(o.produced,file)
 for offset:=0;offset<len(data);{var n uint32;e=windows.WriteFile(h,data[offset:],&n,nil);if e!=nil{return e};if n==0{return errors.New("native output write not acknowledged")};offset+=int(n)}
 if e=windows.FlushFileBuffers(h);e!=nil{return e};if e=windows.CloseHandle(h);e!=nil{return e};file.handle=0;return nil
}
func proofJobEmpty(calls *debianProofWindowsCalls,job windows.Handle)bool{
 var ids struct{Assigned,Count uint32;PIDs [2]uintptr};var returned uint32
 if err:=calls.QueryJob(job,windows.JobObjectBasicProcessIdList,uintptr(unsafe.Pointer(&ids)),uint32(unsafe.Sizeof(ids)),&returned);err!=nil{return false};return ids.Assigned==0&&ids.Count==0
}
func proofReadPipe(owner *debianProofNativeOwner,handle windows.Handle,maximum int,done chan<- proofPipeRead){
 result:=proofPipeRead{};defer func(){done<-result}();buffer:=make([]byte,4096)
 for{var n uint32;err:=owner.calls.Read(handle,buffer,&n,nil);if n>0{if len(result.bytes)+int(n)>maximum{result.err=errors.New("native output exceeded its fixed bound");owner.retire();return};result.bytes=append(result.bytes,buffer[:n]...)};if err==windows.ERROR_BROKEN_PIPE || err==io.EOF{return};if err!=nil{result.err=err;owner.retire();return};if n==0{return}}
}
type proofPipeRead struct{bytes []byte;err error}
func(l *debianProofLifetime)finishReaders(deadline time.Time)bool{
 if l.outResult==nil{select{case r:=<-l.stdout:l.outResult=&r;case<-time.After(max(time.Duration(0),time.Until(deadline))):return false}}
 if l.errResult==nil{select{case r:=<-l.stderr:l.errResult=&r;case<-time.After(max(time.Duration(0),time.Until(deadline))):return false}}
 l.readersSettled=true;return l.outResult.err==nil&&l.errResult.err==nil
}

// Fixed-purpose original-handle launch. This function is never an exec.Cmd
// wrapper and never accepts HTTP commands. Caller derives its closed argv from
// an application-selected, held profile and fixed ordinal0..9.
func(o *debianProofNativeOwner) child(ctx context.Context,a *API,p *debianProducerPlan,ordinal int,argv []string)(result debianProofChildResult,err error){
 result.Ordinal=ordinal;result.Started=time.Now().UTC()
 if o.stopped.Load() || ordinal<0 || ordinal>=10 || len(argv)==0 || !equalAssemblyDebianJSON(argv,p.commands[ordinal]) || !time.Now().Before(o.workDeadline){return result,runtimeinstall.ErrInputProofOwnership}
 sample,e:=o.observer.awaitFresh(minProofTime(o.workDeadline,time.Now().Add(2*time.Second)));if e!=nil||sample.Physical<8<<30||sample.Commit<8<<30||sample.CPU>=50{return result,errors.New("native pre-resume reserve unavailable")};result.Admission=sample
 job,e:=o.calls.CreateJob(nil,nil);if job!=0{owned:=&debianProofJob{handle:job,ordinal:ordinal};o.mu.Lock();o.jobs=append(o.jobs,owned);o.mu.Unlock()};if e!=nil{return result,e};if job==0{return result,errors.New("original Job handle absent")}
 if o.stopped.Load(){o.retire();return result,runtimeinstall.ErrInputProofOwnership}
 var affinity,system uintptr
 if e=o.calls.Affinity(&affinity,&system);e!=nil || affinity==0{return result,errors.New("original process affinity unavailable")}
 oneCPU:=affinity & (^affinity+1)
 limits:=windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
 limits.BasicLimitInformation.LimitFlags=windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE|windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS|windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY|windows.JOB_OBJECT_LIMIT_JOB_MEMORY|windows.JOB_OBJECT_LIMIT_AFFINITY
 limits.BasicLimitInformation.ActiveProcessLimit=1;limits.BasicLimitInformation.Affinity=oneCPU;limits.ProcessMemoryLimit=128<<20;limits.JobMemoryLimit=128<<20
 if _,e=o.calls.SetJob(job,windows.JobObjectExtendedLimitInformation,uintptr(unsafe.Pointer(&limits)),uint32(unsafe.Sizeof(limits)));e!=nil{return result,e}
 var actual windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION;var returned uint32
 if e=o.calls.QueryJob(job,windows.JobObjectExtendedLimitInformation,uintptr(unsafe.Pointer(&actual)),uint32(unsafe.Sizeof(actual)),&returned);e!=nil||actual.BasicLimitInformation.LimitFlags!=limits.BasicLimitInformation.LimitFlags||actual.BasicLimitInformation.ActiveProcessLimit!=1||actual.BasicLimitInformation.Affinity!=oneCPU||actual.ProcessMemoryLimit!=128<<20||actual.JobMemoryLimit!=128<<20{return result,errors.New("original Job limits not acknowledged")}
 // Register partial stdio before every fallible step. A failed Close keeps its
 // handle in this creator's backing state until a later exact retirement.
 life:=&debianProofLifetime{};o.mu.Lock();o.lifetimes=append(o.lifetimes,life);o.mu.Unlock()
 handles:=&life.handles;closeAt:=func(i int)bool{if (*handles)[i]==0{return true};if o.calls.Close((*handles)[i])!=nil{return false};(*handles)[i]=0;return true}
 allClosed:=true
 defer func(){for i:=range *handles{if (i==0||i==2)&&!life.readersSettled || i>=6&&!life.processSettled{allClosed=false;continue};if !closeAt(i){allClosed=false}};result.HandlesClosed=allClosed}()
 sa:=windows.SecurityAttributes{Length:uint32(unsafe.Sizeof(windows.SecurityAttributes{})),InheritHandle:1}
 var outRead,outWrite,errRead,errWrite,inRead,inWrite windows.Handle
 e=o.calls.Pipe(&outRead,&outWrite,&sa,0);*handles=append(*handles,outRead,outWrite);life.readersSettled=true;if e!=nil{return result,e}
 if e=o.calls.HandleInfo(outRead,windows.HANDLE_FLAG_INHERIT,0);e!=nil{return result,e}
 e=o.calls.Pipe(&errRead,&errWrite,&sa,0);*handles=append(*handles,errRead,errWrite);if e!=nil{return result,e}
 if e=o.calls.HandleInfo(errRead,windows.HANDLE_FLAG_INHERIT,0);e!=nil{return result,e}
 e=o.calls.Pipe(&inRead,&inWrite,&sa,0);*handles=append(*handles,inRead,inWrite);if e!=nil{return result,e}
 if !closeAt(5){return result,errors.New("stdin writer close unconfirmed")}
 var attributeBytes uintptr
 _ = o.calls.InitAttributes(nil,&attributeBytes)
 if attributeBytes==0 || attributeBytes>64<<10{return result,errors.New("startup attribute size unavailable")}
 attributes:=make([]byte,attributeBytes);attributePtr:=unsafe.Pointer(&attributes[0])
 if e=o.calls.InitAttributes(attributePtr,&attributeBytes);e!=nil{return result,e}
 deleted:=false;deleteAttributes:=func(){if !deleted{o.calls.DeleteAttributes(attributePtr);deleted=true}}
 defer deleteAttributes()
 childHandles:=[3]windows.Handle{inRead,outWrite,errWrite};jobs:=[1]windows.Handle{job}
 if e=o.calls.UpdateAttributes(attributePtr,windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST,unsafe.Pointer(&childHandles[0]),unsafe.Sizeof(childHandles));e!=nil{return result,e}
 if e=o.calls.UpdateAttributes(attributePtr,0x2000D,unsafe.Pointer(&jobs[0]),unsafe.Sizeof(jobs));e!=nil{return result,e}
 executable,e:=windows.UTF16PtrFromString(argv[0]);if e!=nil{return result,e}
 quoted:=make([]string,len(argv));for i,arg:=range argv{quoted[i]=windows.EscapeArg(arg)};command,e:=windows.UTF16FromString(strings.Join(quoted," "));if e!=nil{return result,e}
 working,e:=windows.UTF16PtrFromString(p.directory);if e!=nil{return result,e}
 environment:=append([]uint16{},p.environment...)
 si:=windows.StartupInfoEx{};si.Cb=uint32(unsafe.Sizeof(si));si.Flags=windows.STARTF_USESTDHANDLES|windows.STARTF_USESHOWWINDOW;si.ShowWindow=windows.SW_HIDE;si.StdInput,si.StdOutput,si.StdErr=inRead,outWrite,errWrite;si.ProcThreadAttributeList=(*windows.ProcThreadAttributeList)(attributePtr)
 if e=a.execution.advanceDebianInputProof(ctx,p.identity,"creating-suspended",nil);e!=nil{return result,e}
 pi:=windows.ProcessInformation{}
 // Called within a retained purpose-specific owner goroutine. The original
 // backing values remain alive through even a late Create ACK after deadline.
 e=o.calls.CreateProcess(executable,&command[0],nil,nil,true,windows.CREATE_SUSPENDED|windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_NO_WINDOW,&environment[0],working,&si.StartupInfo,&pi)
 runtime.KeepAlive(attributes);runtime.KeepAlive(childHandles);runtime.KeepAlive(jobs);runtime.KeepAlive(command);runtime.KeepAlive(environment)
 deleteAttributes()
 *handles=append(*handles,pi.Process,pi.Thread)
 // Any nonzero original PI return creates the owned generation even when the
 // SDK also reports an error. No exit or creation-time identity is inferred.
 o.acknowledgeOriginalCreate(ordinal,pi)
 // Parent WRITE copies MUST close before any wait for reader EOF.
 write1,write2,stdinClosed:=closeAt(1),closeAt(3),closeAt(4);writesClosed:=write1&&write2&&stdinClosed
 if e!=nil{return result,e};result.OriginalHandleOwned=true;result.PID=pi.ProcessId
 var create,exit,kernel,user windows.Filetime;if e=o.calls.Times(pi.Process,&create,&exit,&kernel,&user);e!=nil{return result,e};result.CreationTime=uint64(create.HighDateTime)<<32|uint64(create.LowDateTime)
 life.pid,life.creation=result.PID,result.CreationTime
 stdout,stderr:=make(chan proofPipeRead,1),make(chan proofPipeRead,1);life.stdout,life.stderr=stdout,stderr;go proofReadPipe(o,outRead,128<<10,stdout);go proofReadPipe(o,errRead,128<<10,stderr)
 life.readersSettled=false
 finishReaders:=func()bool{until:=minProofTime(o.wholeDeadline,time.Now().Add(5*time.Second));ok:=life.finishReaders(until)
  if life.outResult!=nil{result.Stdout=life.outResult.bytes};if life.errResult!=nil{result.Stderr=life.errResult.bytes};return ok
 }
 // A reader which does not settle retains its handles and creator on the live
 // owner; no deferred Close races an outstanding original ReadFile.
 defer func(){result.CaptureSettled=finishReaders();if !life.readersSettled{allClosed=false}}()
 if inJob,ie:=o.calls.InJob(pi.Process,job);ie!=nil||!inJob{e=errors.New("original suspended child is not in its Job")}
 childIdentity:=debianInputProofChildIdentity{ordinal,pi.ProcessId,result.CreationTime}
 if e==nil{e=a.execution.advanceDebianInputProof(ctx,p.identity,"creating-suspended",&childIdentity)}
 var permit *product.RuntimeInputProofResumePermit
 if e==nil&&!o.stopped.Load()&&writesClosed{e=p.current(ctx,a)}
 if e==nil&&!o.stopped.Load()&&writesClosed{e=a.execution.requireDebianInputProofReservation(ctx,p.identity,ordinal)}
 if e==nil&&!o.stopped.Load()&&writesClosed{
  sample,e=o.observer.awaitFresh(minProofTime(o.workDeadline,time.Now().Add(time.Second)));if e==nil&&(sample.Physical<8<<30||sample.Commit<8<<30||sample.CPU>=50){e=errors.New("fresh pre-resume capacity refused")};result.Admission=sample
 }
 if e==nil&&!o.stopped.Load()&&writesClosed{permit,e=a.deps.Product.BeginRuntimeInputProofResume(ctx,p.identity,ordinal)}
 if e!=nil||o.stopped.Load()||!writesClosed||permit==nil||!permit.Current(){o.retire();e=runtimeinstall.ErrInputProofOwnership}else{
  _,e=o.calls.Resume(pi.Thread);if pe:=permit.Finish(e==nil);pe!=nil{e=pe;o.retire()}
 }
 if e==nil{e=a.execution.advanceDebianInputProof(ctx,p.identity,"running",nil)}
 if e!=nil{o.retire()}
 childUntil:=minProofTime(o.workDeadline,result.Started.Add(5*time.Second))
 for time.Now().Before(o.wholeDeadline){
  state,we:=o.calls.Wait(pi.Process,50);if we!=nil{e=we;o.retire();break};if state==windows.WAIT_OBJECT_0{result.ExitConfirmed=true;break}
  if !time.Now().Before(childUntil)||o.stopped.Load()||ctx.Err()!=nil{o.retire()}
 }
 if result.ExitConfirmed {if ee:=o.calls.Exit(pi.Process,&result.Exit);ee!=nil{e=ee;result.ExitConfirmed=false}}
 result.JobEmpty=proofJobEmpty(o.calls,job);result.Ended=time.Now().UTC()
 life.processSettled=result.ExitConfirmed&&result.JobEmpty
 if !result.ExitConfirmed||!result.JobEmpty{e=errors.New("original child or Job settlement unknown")}
 return result,e
}
func minProofTime(a,b time.Time)time.Time{if a.Before(b){return a};return b}

// Only OS identity is obtained here. No installed diagnostic executable is
// discovered or selected. Product proof payloads come solely from the profile.
func debianProofEnvironment(home string)([]uint16,error){
 root,err:=windows.GetWindowsDirectory();if err!=nil{return nil,err}
 entries:=[]string{"GNUPGHOME="+home,"SystemRoot="+root,"TEMP="+home,"TMP="+home,"WINDIR="+root}
 var out []uint16;for _,part:=range entries{v,e:=windows.UTF16FromString(part);if e!=nil{return nil,e};out=append(out,v...)};out=append(out,0);return out,nil
}
