//go:build windows

package api

import (
 "context"
 "errors"
 "path/filepath"
 "sync"
 "sync/atomic"
 "testing"
 "time"
 "unsafe"

 "golang.org/x/sys/windows"
)

// INERT kernel outcomes only: this fixture never calls the production table,
// opens a DLL/query/process/Job or establishes actual Windows containment.
type inertDebianKernel struct {
 mu sync.Mutex
 closed map[windows.Handle]bool
 pipe int
 limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
 created,resumed,terminated atomic.Int32
 deleted atomic.Bool
 enter,release chan struct{}
 failClose windows.Handle
 partialPipe bool
}
func(k *inertDebianKernel)calls()*debianProofWindowsCalls{return &debianProofWindowsCalls{
 CreateJob:func(*windows.SecurityAttributes,*uint16)(windows.Handle,error){return 77,nil},
 SetJob:func(_ windows.Handle,_ uint32,p uintptr,_ uint32)(int,error){k.limits=*(*windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION)(unsafe.Pointer(p));return 1,nil},
 QueryJob:func(_ windows.Handle,class int32,p uintptr,_ uint32,_ *uint32)error{if class==windows.JobObjectExtendedLimitInformation{*(*windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION)(unsafe.Pointer(p))=k.limits};return nil},
 TerminateJob:func(windows.Handle,uint32)error{k.terminated.Add(1);return nil},
 Close:func(h windows.Handle)error{k.mu.Lock();defer k.mu.Unlock();if h==k.failClose{return errors.New("synthetic close refusal")};if k.closed[h]{return errors.New("synthetic double close")};k.closed[h]=true;return nil},
 Pipe:func(r,w *windows.Handle,_ *windows.SecurityAttributes,_ uint32)error{k.pipe++;*r=windows.Handle(99+2*k.pipe);*w=*r+1;if k.partialPipe{return errors.New("synthetic partial pipe")};return nil},
 HandleInfo:func(windows.Handle,uint32,uint32)error{return nil},
 CreateProcess:func(_ *uint16,_ *uint16,_ *windows.SecurityAttributes,_ *windows.SecurityAttributes,inherit bool,flags uint32,_ *uint16,_ *uint16,_ *windows.StartupInfo,pi *windows.ProcessInformation)error{
  k.created.Add(1);if !inherit||flags&windows.CREATE_SUSPENDED==0||flags&windows.EXTENDED_STARTUPINFO_PRESENT==0||k.deleted.Load(){return errors.New("synthetic startup ownership rejected")}
  if k.enter!=nil{close(k.enter);<-k.release}
  if k.deleted.Load(){return errors.New("backing attributes freed before original ACK")}
  pi.Process,pi.Thread,pi.ProcessId=201,202,203;return nil
 },
 Times:func(_ windows.Handle,create,_,_,_ *windows.Filetime)error{create.LowDateTime=204;return nil},
 Read:func(h windows.Handle,_ []byte,n *uint32,_ *windows.Overlapped)error{k.mu.Lock();defer k.mu.Unlock();writer:=h+1;if !k.closed[writer]{return errors.New("parent writer retained at reader EOF")};*n=0;return nil},
 Wait:func(windows.Handle,uint32)(uint32,error){return windows.WAIT_OBJECT_0,nil},Exit:func(_ windows.Handle,v *uint32)error{*v=0;return nil},Resume:func(windows.Handle)(uint32,error){k.resumed.Add(1);return 1,nil},
 Affinity:func(a,b *uintptr)error{*a,*b=3,3;return nil},InJob:func(windows.Handle,windows.Handle)(bool,error){return true,nil},
 InitAttributes:func(_ unsafe.Pointer,n *uintptr)error{*n=64;return nil},UpdateAttributes:func(unsafe.Pointer,uintptr,unsafe.Pointer,uintptr)error{return nil},DeleteAttributes:func(unsafe.Pointer){k.deleted.Store(true)},
}}
func inertDebianOwner(k *inertDebianKernel)*debianProofNativeOwner{
 now:=time.Now();done:=make(chan struct{});close(done)
 o:=&debianProofNativeOwner{calls:k.calls(),began:now,workDeadline:now.Add(40*time.Second),wholeDeadline:now.Add(60*time.Second),cancel:func(){},stopWatch:make(chan struct{}),watchDone:make(chan struct{}),observer:&debianProofObserver{sample:debianNativeSample{At:now,Physical:9<<30,Commit:9<<30,CPU:10,Known:true},stop:make(chan struct{}),done:done,closed:true}}
 o.census=newDebianProofNativeCensus(o);o.observerStarted.Store(true);return o
}
func TestDebianNativeActualChildLateCreateNeverResumesAndClosesParentWrites(t *testing.T){
 k:=&inertDebianKernel{closed:map[windows.Handle]bool{},enter:make(chan struct{}),release:make(chan struct{})};o:=inertDebianOwner(k)
 g:=executionTestGate(t,filepath.Join(t.TempDir(),"shared"));row:=nativeLeaseFixture();row.Scope=g.scope
 if e:=g.mutate(context.Background(),func(l *executionLeaseLedger)error{l.InputProofs=append(l.InputProofs,row);return nil});e!=nil{t.Fatal(e)}
 p:=&debianProducerPlan{identity:row.Identity,directory:t.TempDir(),environment:[]uint16{0,0}};p.commands[0]=[]string{"synthetic-held-gpg"}
 type response struct{result debianProofChildResult;err error};finished:=make(chan response,1)
 go func(){r,e:=o.child(context.Background(),g.api,p,0,p.commands[0]);finished<-response{r,e}}()
 select{case<-k.enter:case<-time.After(time.Second):t.Fatal("actual child method did not reach synthetic Create")}
 o.retire();close(o.watchDone) // explicitly acknowledged fake watchdog settlement
 if k.deleted.Load(){t.Fatal("startup values freed while Create borrowed them")};close(k.release)
 var got response;select{case got=<-finished:case<-time.After(time.Second):t.Fatal("late original ACK did not join cleanup")}
 if got.err==nil||k.resumed.Load()!=0||!got.result.OriginalHandleOwned||!got.result.ExitConfirmed||!got.result.CaptureSettled||!got.result.HandlesClosed{t.Fatalf("late-ACK ownership/result mismatch: %#v %v",got.result,got.err)}
 inherited:=o.lifetimes[0].inherited;if inherited==nil||inherited.pending||inherited.slots[0].value==0||inherited.childOf!=o.lifetimes[0].piTicket{t.Fatal("parent stdio/PI Close prematurely settled child copies")}
 k.mu.Lock();jobClosed:=k.closed[77];writers:=k.closed[102]&&k.closed[104]&&k.closed[105];k.mu.Unlock()
 if jobClosed||!writers||!k.deleted.Load(){t.Fatal("Job closed too early or parent writes/attributes not settled")}
 if !settleInertDebianOwner(o){t.Fatal("synthetic original handles failed positive settlement")}
 if inherited.slots[0].value!=0||!o.processCensusSettled(){t.Fatal("joint original process/Job settlement did not retire inherited copies")}
}
func TestDebianNativeActualChildRetainsPartialPipeAndFailedClose(t *testing.T){
 k:=&inertDebianKernel{closed:map[windows.Handle]bool{},partialPipe:true,failClose:102};o:=inertDebianOwner(k)
 p:=&debianProducerPlan{directory:t.TempDir()};p.commands[0]=[]string{"synthetic-held-gpg"}
 result,e:=o.child(context.Background(),&API{},p,0,p.commands[0])
 if e==nil||result.HandlesClosed||k.created.Load()!=0||len(o.lifetimes)!=1||o.lifetimes[0].handles[1]!=102{t.Fatal("partial original handle was lost or a child was created")}
 o.retire();close(o.watchDone);if o.settle(){t.Fatal("failed Close was labelled positive")}
 k.mu.Lock();k.failClose=0;k.mu.Unlock();if !settleInertDebianOwner(o){t.Fatal("exact retained handle could not settle after changed fake Close outcome")}
}
func TestDebianNativeRetirementBorrowPreventsConcurrentOriginalJobClose(t *testing.T){
 k:=&inertDebianKernel{closed:map[windows.Handle]bool{}};o:=inertDebianOwner(k);entered,release:=make(chan struct{}),make(chan struct{})
 o.calls.TerminateJob=func(windows.Handle,uint32)error{close(entered);<-release;return nil}
 ticket,e:=o.reserveNativeOriginal(debianProofOriginalJob);if e!=nil{t.Fatal(e)};if e=o.recordKernelReturn(ticket,77);e!=nil{t.Fatal(e)};o.jobs=[]*debianProofJob{{handle:77}};close(o.watchDone);retired:=make(chan struct{});go func(){o.retire();close(retired)}();<-entered
 select{case<-retired:case<-time.After(time.Second):t.Fatal("blocked native termination blocked emergency latch")}
 if o.settle(){t.Fatal("original Job closed while retirement borrowed its handle")};k.mu.Lock();closed:=k.closed[77];k.mu.Unlock();if closed{t.Fatal("borrowed handle was closed")}
 close(release);if !settleInertDebianOwner(o){t.Fatal("original handle did not settle after termination borrower returned")}
}
func settleInertDebianOwner(o *debianProofNativeOwner)bool{until:=time.Now().Add(time.Second);for time.Now().Before(until){if o.settle(){return true};time.Sleep(time.Millisecond)};return false}
func TestDebianNativePendingGenerationConsumesLateCreateAfterWholeDeadline(t *testing.T){testDebianNativePendingGeneration(t,false)}
func TestDebianNativePendingGenerationConsumesPartialCreateError(t *testing.T){testDebianNativePendingGeneration(t,true)}
func TestDebianNativeActualChildKnownUnallocatedCreateSettlesOnlyItsOwnDerivatives(t *testing.T){
 k:=&inertDebianKernel{closed:map[windows.Handle]bool{}};o:=inertDebianOwner(k)
 o.calls.CreateProcess=func(_ *uint16,_ *uint16,_ *windows.SecurityAttributes,_ *windows.SecurityAttributes,_ bool,_ uint32,_ *uint16,_ *uint16,_ *windows.StartupInfo,_ *windows.ProcessInformation)error{k.created.Add(1);return errors.New("synthetic direct Win32 false with zero original PI")}
 g:=executionTestGate(t,filepath.Join(t.TempDir(),"shared"));row:=nativeLeaseFixture();row.Scope=g.scope;if e:=g.mutate(context.Background(),func(l *executionLeaseLedger)error{l.InputProofs=append(l.InputProofs,row);return nil});e!=nil{t.Fatal(e)}
 p:=&debianProducerPlan{identity:row.Identity,directory:t.TempDir(),environment:[]uint16{0,0}};p.commands[0]=[]string{"synthetic-held-gpg"};result,e:=o.child(context.Background(),g.api,p,0,p.commands[0]);if e==nil||result.OriginalHandleOwned||k.created.Load()!=1||k.resumed.Load()!=0||len(o.lifetimes)!=1{t.Fatal("known unallocated Create acquired native identity")};life:=o.lifetimes[0]
 if life.inherited==nil||life.inherited.pending||life.inherited.slots[0].value!=0||o.jobs[0].generation!=0{t.Fatal("direct false/noPI became a late allocated generation")};o.retire();close(o.watchDone);if !settleInertDebianOwner(o)||!o.originalCensusSettled(){t.Fatal("known no-allocation plus actual fake Job cleanup did not settle")}
}
func testDebianNativePendingGeneration(t *testing.T,partialError bool){
 t.Helper();k:=&inertDebianKernel{closed:map[windows.Handle]bool{},enter:make(chan struct{}),release:make(chan struct{})};o:=inertDebianOwner(k)
 originalCreate:=o.calls.CreateProcess
 if partialError{o.calls.CreateProcess=func(app,command *uint16,processSA,threadSA *windows.SecurityAttributes,inherit bool,flags uint32,env,dir *uint16,start *windows.StartupInfo,pi *windows.ProcessInformation)error{if e:=originalCreate(app,command,processSA,threadSA,inherit,flags,env,dir,start,pi);e!=nil{return e};return errors.New("synthetic partial original Create ACK plus error")}}
 firstEntered,secondEntered,releaseFirst,releaseSecond:=make(chan struct{}),make(chan struct{}),make(chan struct{}),make(chan struct{})
 var childOnce,firstOnce,secondOnce sync.Once
 releaseChild:=func(){childOnce.Do(func(){close(k.release)})};releaseOld:=func(){firstOnce.Do(func(){close(releaseFirst)})};releaseNew:=func(){secondOnce.Do(func(){close(releaseSecond)})}
 defer releaseChild();defer releaseOld();defer releaseNew()
 o.calls.TerminateJob=func(windows.Handle,uint32)error{switch k.terminated.Add(1){case 1:close(firstEntered);<-releaseFirst;case 2:close(secondEntered);<-releaseSecond;default:t.Error("same generation received a replacement termination")};return nil}
 g:=executionTestGate(t,filepath.Join(t.TempDir(),"shared"));row:=nativeLeaseFixture();row.Scope=g.scope
 if e:=g.mutate(context.Background(),func(l *executionLeaseLedger)error{l.InputProofs=append(l.InputProofs,row);return nil});e!=nil{t.Fatal(e)}
 p:=&debianProducerPlan{identity:row.Identity,directory:t.TempDir(),environment:[]uint16{0,0}};p.commands[0]=[]string{"synthetic-held-gpg"}
 type response struct{result debianProofChildResult;err error};finished:=make(chan response,1)
 go func(){r,e:=o.child(context.Background(),g.api,p,0,p.commands[0]);finished<-response{r,e}}()
 select{case<-k.enter:case<-time.After(time.Second):t.Fatal("synthetic original Create not entered")}
 o.retire();select{case<-firstEntered:case<-time.After(time.Second):t.Fatal("original pre-ACK termination not entered")}
 // Explicit fake scheduling boundary: whole deadline and watchdog have already
 // settled while BOTH original calls remain borrowed. No real native effect.
 o.wholeDeadline=time.Now().Add(-time.Second);close(o.watchDone);releaseChild()
 var got response;select{case got=<-finished:case<-time.After(time.Second):t.Fatal("late original Create did not return")}
 if got.err==nil||k.resumed.Load()!=0||len(o.lifetimes)!=1||len(o.lifetimes[0].handles)!=8||o.lifetimes[0].handles[6]!=201||o.lifetimes[0].handles[7]!=202{t.Fatal("late/partial original handles or refusal were lost")}
 inherited:=o.lifetimes[0].inherited;if inherited==nil||inherited.pending||inherited.childOf!=o.lifetimes[0].piTicket||inherited.slots[0].value==0{t.Fatal("late/partial Create lost possible inherited stdio generation")}
 j:=o.jobs[0];j.mu.Lock();pending:=j.generation==1&&j.lastTerminationGeneration==0&&j.terminating&&j.borrowers==1;j.mu.Unlock();if !pending{t.Fatal("new original generation was not pending under exact old borrower")}
 releaseOld();select{case<-secondEntered:case<-time.After(time.Second):t.Fatal("returning original borrower stranded the late generation")}
 j.mu.Lock();held:=j.handle==77&&j.borrowers==1&&j.lastTerminationGeneration==1;j.mu.Unlock();if !held{t.Fatal("pending handoff exposed a closable original Job")}
 if o.settle(){t.Fatal("expired whole deadline labelled native settlement positive")};k.mu.Lock();closed:=k.closed[77];k.mu.Unlock();if closed{t.Fatal("original Job closed during pending termination")}
 releaseNew();until:=time.Now().Add(time.Second)
 for time.Now().Before(until){j.mu.Lock();settled:=!j.terminating&&j.borrowers==0;j.mu.Unlock();if settled{o.retire();if k.terminated.Load()!=2{t.Fatal("duplicate generation termination")};return};time.Sleep(time.Millisecond)}
 t.Fatal("exact pending borrower did not return")
}
