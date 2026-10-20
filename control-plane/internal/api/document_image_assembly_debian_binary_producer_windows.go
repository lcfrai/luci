//go:build windows

package api

import (
 "context"
 "errors"
 "io"
 "runtime"
 "strconv"
 "strings"
 "time"
 "unsafe"

 "golang.org/x/sys/windows"
 "github.com/lcfrai/luci/control-plane/internal/product"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

var binaryPeekPipe=proofKernel.NewProc("PeekNamedPipe")
var binaryTickCount=proofKernel.NewProc("GetTickCount64")

// Construct a NEW purpose owner. Never extend/reset an existing first3 clock.
func newDebianBinaryProofNativeOwner(ctx context.Context)*debianProofNativeOwner{
 o:=newDebianProofNativeOwner(ctx);o.workDeadline=o.began.Add(300*time.Second);o.wholeDeadline=o.began.Add(420*time.Second);return o
}

func(o *debianProofNativeOwner)positiveBinaryCompletion(result debianProofChildResult)bool{
 if o==nil||o.stopped.Load()||!o.observerStarted.Load()||!o.processCensusSettled()||len(o.jobs)!=1||len(o.lifetimes)!=1||!time.Now().Before(o.workLimit()){return false}
 select{case<-o.watchDone:default:return false}
 o.observer.mu.Lock();closed:=o.observer.closed&&o.observer.err==nil;o.observer.mu.Unlock();if !closed{return false}
 job:=o.jobs[0];job.mu.Lock();closed=job.settled&&job.handle==0&&job.borrowers==0&&job.ordinal==0;job.mu.Unlock();if !closed{return false}
 life:=o.lifetimes[0]
 if !life.processSettled||!life.readersSettled||life.pid!=result.PID||life.creation!=result.CreationTime{return false}
 for _,h:=range life.handles{if h!=0{return false}}
 return result.Ordinal==0&&result.PID!=0&&result.CreationTime!=0&&result.Exit==0&&result.OriginalHandleOwned&&result.ExitConfirmed&&result.JobEmpty&&result.HandlesClosed&&result.CaptureSettled&&!result.Started.IsZero()&&!result.Ended.Before(result.Started)&&result.Ended.Sub(result.Started)<=300*time.Second
}

func binaryOriginalIdentity(v windows.ByHandleFileInformation)string{
 values:=[]uint64{1,uint64(v.FileAttributes),uint64(v.VolumeSerialNumber),uint64(v.FileIndexHigh)<<32|uint64(v.FileIndexLow),uint64(v.FileSizeHigh)<<32|uint64(v.FileSizeLow),uint64(v.CreationTime.HighDateTime)<<32|uint64(v.CreationTime.LowDateTime),uint64(v.LastWriteTime.HighDateTime)<<32|uint64(v.LastWriteTime.LowDateTime),uint64(v.NumberOfLinks)}
 parts:=make([]string,len(values));for i,v:=range values{parts[i]=strconv.FormatUint(v,10)};return strings.Join(parts,",")
}

func(o *debianProofNativeOwner)binaryEntryArguments(ctx context.Context,p *debianProducerPlan,t *debianBinaryNativeTransport)([]string,error){
 if ctx==nil||ctx.Err()!=nil||p==nil||p.identity.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose||!o.ownsBinaryTransport(t)||t.native==nil||o.nativeRetired()||o.calls==nil||o.calls.BinaryTick==nil||o.calls.FileInfo==nil{return nil,errDebianProofCensus}
 entry:=p.files[debianBinaryEntryRole]
 if entry.Bytes!=debianBinaryEntryBytes||entry.SHA256!=debianBinaryEntrySHA||p.paths[debianBinaryEntryRole]==""||p.paths["cp-python"]==""{return nil,runtimeinstall.ErrInputProofUnavailable}
 handles:=make([]string,181)
 t.mu.Lock();seen:=map[windows.Handle]bool{};for i,alias:=range t.native.aliases{if !alias.duplicateReturned||!alias.borrowReturned||alias.closing||!validDebianProducedHandle(alias.handle)||seen[alias.handle]{t.mu.Unlock();return nil,errDebianProofCensus};seen[alias.handle]=true;handles[i]=strconv.FormatUint(uint64(alias.handle),16)};selectionHandle:=t.native.aliases[178].handle;t.mu.Unlock()
 var info windows.ByHandleFileInformation;if err:=o.calls.FileInfo(selectionHandle,&info);err!=nil{return nil,err}
 selected:=t.selection.request.SmallSources[0]
 if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT)!=0||info.NumberOfLinks!=1||uint64(info.FileSizeHigh)<<32|uint64(info.FileSizeLow)!=uint64(selected.Bytes){return nil,errDebianProofCensus}
 // Tick FIRST, Go-now SECOND: only remaining original work is delegated.
 tick,err:=o.calls.BinaryTick();now:=time.Now();if err!=nil{return nil,err};deadline,err:=debianBinaryRawAbsoluteTick(tick,now,o.workLimit());if err!=nil{return nil,err}
 return []string{p.paths["cp-python"],"-I","-S","-B",p.paths[debianBinaryEntryRole],"--abi","luci-debian-binary-entry-v1","--operation-id",p.identity.RunID,"--loan-id",t.selection.request.LoanID,"--source-binding-sha256",t.selection.request.ScopeDigest,"--selection-sha256",selected.Digest,"--selection-bytes",strconv.FormatInt(selected.Bytes,10),"--selection-identity",binaryOriginalIdentity(info),"--work-deadline-tick-ms",strconv.FormatUint(deadline,10),"--handles",strings.Join(handles,",")},nil
}

// Both streams use the SAME decoder/read tickets on this retained goroutine.
// Peek avoids blocking on one empty pipe while the other fills. Every actual
// Read still has its own permanently charged ticket and original backing.
func(s *debianBinarySpoolSet)captureOriginalPipes(ctx context.Context,life *debianProofLifetime)error{
 if s==nil||s.native==nil||s.owner==nil||s.owner.calls==nil||s.decoder.state==nil||life==nil||len(life.handles)<8||s.native.life!=life||s.owner.calls.BinaryPeek==nil{return errDebianBinarySpool}
 defer func(){life.readersSettled=true}() // this goroutine has returned from every synchronous read
 // Borrow the decoder's existing scratch at the sole native read boundary;
 // never allocate another frame arena or additional stdout scratch buffer.
 stdout:=s.decoder.state.scratch[:]
 attempts:=0
 for{
  if !s.current(ctx)||attempts>=1000000{return errDebianBinarySpool};attempts++
  s.decoder.state.mu.Lock();ready:=s.decoder.state.ready;outEOF:=s.decoder.state.stdoutEOF;errEOF:=s.decoder.state.stderrEOF;s.decoder.state.mu.Unlock()
  if ready{view,err:=s.decoder.nextFrame();if err!=nil{return err};if err=s.persistFrame(ctx,view);err!=nil{return err};continue}
  if outEOF&&errEOF{return s.finishRawSpool(ctx)}
  progressed:=false
  for _,stream:=range []debianBinaryRawStream{debianBinaryRawStdout,debianBinaryRawStderr}{
   if stream==debianBinaryRawStdout&&outEOF||stream==debianBinaryRawStderr&&errEOF{continue}
   h:=life.handles[0];if stream==debianBinaryRawStderr{h=life.handles[2]}
   var available uint32
   peekErr:=s.owner.calls.BinaryPeek(h,&available)
   broken:=peekErr==windows.ERROR_BROKEN_PIPE
   if peekErr!=nil&&!broken{return errDebianBinarySpool};if available==0&&!broken{continue};progressed=true
   if stream==debianBinaryRawStderr{if err:=s.readStderrOnce(ctx);err!=nil{return err};continue}
   request:=min(int(available),len(stdout));if broken{request=1}
   s.decoder.state.mu.Lock();last:=s.decoder.state.frames==23;s.decoder.state.mu.Unlock();if last{request=1}
   ticket,err:=s.decoder.beginRead(stream,request,time.Now());if err!=nil{return err}
   attempt:=&debianBinarySpoolReadAttempt{ticket:ticket,original:h};s.mu.Lock();s.native.stdoutAttempt=attempt;s.mu.Unlock()
   var n uint32;readErr:=s.owner.calls.Read(h,stdout[:request],&n,nil)
   s.mu.Lock();attempt.count=n;attempt.nativeErr=readErr;attempt.returned=true;s.mu.Unlock()
   if n>uint32(request){return errDebianBinaryRaw}
   if n==0&&readErr==windows.ERROR_BROKEN_PIPE{readErr=io.EOF}
   if err=s.decoder.observeRead(ticket,stdout[:n],readErr,time.Now());err!=nil{return err}
   // A frame can leave admitted bytes in scratch; process its view before a
   // second stdout read. Stderr is serviced on the next bounded loop turn.
   break
  }
  if !progressed{select{case<-ctx.Done():return ctx.Err();case<-time.After(time.Millisecond):}}
 }
}

func(o *debianProofNativeOwner)binaryChild(ctx context.Context,a *API,p *debianProducerPlan,t *debianBinaryNativeTransport,s *debianBinarySpoolSet)(result debianProofChildResult,err error){
 result.Ordinal=0;result.Started=time.Now().UTC()
 if p==nil||p.identity.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose||!debianProducerPlatformSupported()||!o.ownsBinaryTransport(t)||t.claim==nil||s==nil||s.owner!=o||!o.observerStarted.Load()||o.nativeRetired(){return result,runtimeinstall.ErrInputProofOwnership}
 defer func(){if err!=nil{o.retire()}}()
 if err=p.current(ctx,a);err!=nil{return result,err}
 if err=o.beginBinaryOriginal(t.claim,t.claim.job);err!=nil{return result,err}
 job,e:=o.calls.CreateJob(nil,nil);owned:=&debianProofJob{handle:debianProofOwnedKernel(job),ordinal:0}
 if validDebianProducedHandle(job){o.mu.Lock();o.jobs=append(o.jobs,owned);o.mu.Unlock()}
 if ack:=o.recordKernelCallReturn(t.claim.job,e,job);ack!=nil{return result,ack};if e!=nil{return result,e};if !validDebianProducedHandle(job){return result,errDebianProofCensus}
 var affinity,system uintptr;if e=o.calls.Affinity(&affinity,&system);e!=nil||affinity==0{return result,errDebianProofCensus};oneCPU:=affinity&(^affinity+1)
 limits:=windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
 limits.BasicLimitInformation.LimitFlags=windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE|windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS|windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY|windows.JOB_OBJECT_LIMIT_JOB_MEMORY|windows.JOB_OBJECT_LIMIT_AFFINITY
 limits.BasicLimitInformation.ActiveProcessLimit=1;limits.BasicLimitInformation.Affinity=oneCPU;limits.ProcessMemoryLimit=2<<30;limits.JobMemoryLimit=2<<30
 if _,e=o.calls.SetJob(job,windows.JobObjectExtendedLimitInformation,uintptr(unsafe.Pointer(&limits)),uint32(unsafe.Sizeof(limits)));e!=nil{return result,e}
 var actual windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION;var returned uint32
 if e=o.calls.QueryJob(job,windows.JobObjectExtendedLimitInformation,uintptr(unsafe.Pointer(&actual)),uint32(unsafe.Sizeof(actual)),&returned);e!=nil||actual.BasicLimitInformation.LimitFlags!=limits.BasicLimitInformation.LimitFlags||actual.BasicLimitInformation.ActiveProcessLimit!=1||actual.BasicLimitInformation.Affinity!=oneCPU||actual.ProcessMemoryLimit!=2<<30||actual.JobMemoryLimit!=2<<30{return result,errDebianProofCensus}
 life:=&debianProofLifetime{job:owned,piTicket:t.claim.pi,inherited:t.claim.inherited};o.mu.Lock();o.lifetimes=append(o.lifetimes,life);o.mu.Unlock()
 sa:=windows.SecurityAttributes{Length:uint32(unsafe.Sizeof(windows.SecurityAttributes{})),InheritHandle:1}
 for i:=0;i<3;i++{
  if e=o.beginBinaryOriginal(t.claim,t.claim.pipes[i]);e!=nil{return result,e}
  var read,write windows.Handle;e=o.calls.Pipe(&read,&write,&sa,0);read,write=debianProofOwnedKernel(read),debianProofOwnedKernel(write)
  life.handles=append(life.handles,read,write)
  if ack:=o.recordKernelCallReturn(t.claim.pipes[i],e,read,write);ack!=nil{return result,ack};if e!=nil{return result,e};if read==0||write==0{return result,errDebianProofCensus}
  if i<2{if e=o.calls.HandleInfo(read,windows.HANDLE_FLAG_INHERIT,0);e!=nil{return result,e}}
 }
 closeAt:=func(i int)error{h:=life.handles[i];if h==0{return nil};if e:=o.closeKernelOriginal(h);e!=nil{return e};life.handles[i]=0;return nil}
 if e=closeAt(5);e!=nil{return result,e}
 startup,e:=o.prepareBinaryInheritedHandleList(ctx,t,life);if e!=nil{return result,e}
 argv,e:=o.binaryEntryArguments(ctx,p,t);if e!=nil{return result,e}
 executable,e:=windows.UTF16PtrFromString(argv[0]);if e!=nil{return result,e};quoted:=make([]string,len(argv));for i,v:=range argv{quoted[i]=windows.EscapeArg(v)};command,e:=windows.UTF16FromString(strings.Join(quoted," "));if e!=nil{return result,e}
 working,e:=windows.UTF16PtrFromString(p.directory);if e!=nil{return result,e};environment:=append([]uint16(nil),p.environment...);if len(environment)==0{return result,errDebianProofCensus}
 si:=windows.StartupInfoEx{};si.Cb=uint32(unsafe.Sizeof(si));si.Flags=windows.STARTF_USESTDHANDLES|windows.STARTF_USESHOWWINDOW;si.ShowWindow=windows.SW_HIDE;si.StdInput,si.StdOutput,si.StdErr=startup.handles[0],startup.handles[1],startup.handles[2];si.ProcThreadAttributeList=(*windows.ProcThreadAttributeList)(startup.attributePointer)
 if e=a.execution.advanceDebianInputProof(ctx,p.identity,"creating-suspended",nil);e!=nil{return result,e};if e=o.beginBinaryStartupCreate(ctx,t,startup);e!=nil{return result,e}
 var pi windows.ProcessInformation
 e=o.calls.CreateProcess(executable,&command[0],nil,nil,true,windows.CREATE_SUSPENDED|windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_NO_WINDOW,&environment[0],working,&si.StartupInfo,&pi)
 runtime.KeepAlive(command);runtime.KeepAlive(environment);runtime.KeepAlive(startup)
 ack:=o.recordBinaryOriginalCreateReturn(t,startup,pi,e);if ack!=nil{return result,ack}
 if e!=nil{return result,e};if pi.Process==0||pi.Thread==0||pi.ProcessId==0{return result,errDebianProofCensus}
 for _,i:=range []int{1,3,4}{if e=closeAt(i);e!=nil{return result,e}}
 result.PID,result.OriginalHandleOwned=pi.ProcessId,true
 var creation,exit,kernel,user windows.Filetime;if e=o.calls.Times(pi.Process,&creation,&exit,&kernel,&user);e!=nil{return result,e};result.CreationTime=uint64(creation.HighDateTime)<<32|uint64(creation.LowDateTime);life.pid,life.creation=result.PID,result.CreationTime
 if result.CreationTime==0{return result,errDebianProofCensus};if inJob,ie:=o.calls.InJob(pi.Process,job);ie!=nil||!inJob{return result,errDebianProofCensus}
 if e=s.bindStderrLife(life);e!=nil{return result,e}
 child:=debianInputProofChildIdentity{0,result.PID,result.CreationTime};if e=a.execution.advanceDebianInputProof(ctx,p.identity,"creating-suspended",&child);e!=nil{return result,e}
 if e=p.current(ctx,a);e!=nil{return result,e};if e=a.execution.requireDebianInputProofReservation(ctx,p.identity,0);e!=nil{return result,e}
 sample,e:=o.observer.awaitFresh(minProofTime(o.workLimit(),time.Now().Add(time.Second)));if e!=nil||sample.Physical<8<<30||sample.Commit<8<<30||sample.CPU>=50{return result,errDebianProofCensus};result.Admission=sample
 var permit *product.RuntimeInputProofResumePermit;permit,e=a.deps.Product.BeginRuntimeInputProofResume(ctx,p.identity,0);if e!=nil||permit==nil||!permit.Current()||o.nativeRetired(){return result,runtimeinstall.ErrInputProofOwnership}
 _,e=o.calls.Resume(pi.Thread);if pe:=permit.Finish(e==nil);pe!=nil{e=pe};if e!=nil{return result,e}
 if e=a.execution.advanceDebianInputProof(ctx,p.identity,"running",nil);e!=nil{return result,e}
 if e=s.captureOriginalPipes(ctx,life);e!=nil{return result,e};result.CaptureSettled=true
 for time.Now().Before(o.wholeDeadline){state,we:=o.calls.Wait(pi.Process,25);if we!=nil{return result,we};if state==windows.WAIT_OBJECT_0{result.ExitConfirmed=true;break};if o.nativeRetired()||ctx.Err()!=nil||!time.Now().Before(o.workLimit()){return result,runtimeinstall.ErrInputProofOwnership}}
 if !result.ExitConfirmed{return result,errors.New("binary original process did not settle")}
 if e=o.calls.Exit(pi.Process,&result.Exit);e!=nil{return result,e};result.JobEmpty=proofJobEmpty(o.calls,job);life.processSettled=result.JobEmpty;result.Ended=time.Now().UTC()
 if result.Exit!=0||!result.JobEmpty{return result,errors.New("binary original exit or Job refused")}
 // Retire only this transport's borrowed alias capability. Successful source
 // guards/output remain current until canonical validation/adoption finishes.
 t.retire();if !o.settleBinaryNativeTransport(t)||!o.settle(){return result,errDebianProofCensus}
 if e=t.transport.ReleaseRetiredOriginalBorrows();e!=nil{return result,e};t.mu.Lock();t.released=true;t.mu.Unlock()
 if e=p.current(ctx,a);e!=nil{return result,e};if o.nativeRetired()||!time.Now().Before(o.workLimit()){return result,runtimeinstall.ErrInputProofOwnership}
 result.HandlesClosed=true;return result,nil
}
