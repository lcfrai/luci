//go:build windows

package api

import (
 "context"
 "errors"
 "path/filepath"
 "sync"
 "testing"
 "time"

 "golang.org/x/sys/windows"
 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
)

// Fixed metadata/fake kernel scheduling only: no original native acquisition,
// PDH, DLL lookup, process launch, signature or eligible profile is established.
func(o *debianProofNativeOwner)recordKernelReturn(t *debianProofOriginalTicket,handles ...windows.Handle)error{
 // Synthetic known ACK helper only in tests. Production always classifies the
 // actual primitive status using recordKernelCallReturn, never this shortcut.
 values:=make([]uintptr,len(handles));for i,h:=range handles{if validDebianProducedHandle(h){values[i]=uintptr(h)}};return o.recordNativeReturn(t,values...)
}
func censusPreparedSource(t *testing.T)(*debianProofNativeOwner,*debianProofEvidenceAdmission){
 t.Helper();n:=newDebianProofNativeOwner(context.Background());a,err:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition([]backend.DocumentImageAssemblyEvidenceObject{{Role:"source",Path:filepath.Join(t.TempDir(),"never-opened.fixture"),Bytes:1,Digest:assemblyCatalogDigest([]byte("x"))}});if err!=nil{t.Fatal(err)};p,err:=n.registerEvidenceSource(context.Background(),a);if err!=nil{t.Fatal(err)};return n,p
}
func TestDebianCensusFixedCapacityBlocksActualFileHelperBeforeAllocation(t *testing.T){
 n,f:=projectionInertNative();for i:=0;i<1019;i++{if _,err:=n.reserveNativeOriginal(debianProofOriginalJob);err!=nil{t.Fatal(err)}};pi,child,err:=n.reserveOriginalChild();if err!=nil||pi==nil||child.childOf!=pi||n.census.reserved!=1024{t.Fatal("fixed PI2+inherited3 did not fit exact boundary",err)}
 if n.writeEvidenceBytes(`C:\fixed-owner\refused.json`,[]byte("x"),16)==nil||f.next!=100||len(f.paths)!=0{t.Fatal("1025th possible original reached fake CreateFile")}
 other,_:=projectionInertNative();for i:=0;i<1020;i++{if _,err:=other.reserveNativeOriginal(debianProofOriginalJob);err!=nil{t.Fatal(err)}};before:=len(other.census.native);if _,_,err:=other.reserveOriginalChild();err==nil||other.census.reserved!=1020||len(other.census.native)!=before{t.Fatal("refused five-slot Create partially reserved capacity")}
}
func TestDebianCensusNamespacesReuseAndForeignTicketsRemainDistinct(t *testing.T){
 n,_:=censusPreparedSource(t);file,err:=n.reserveNativeOriginal(debianProofOriginalFile);if err!=nil{t.Fatal(err)};if err=n.recordNativeReturn(file,55);err!=nil{t.Fatal(err)};admission,err:=n.reserveObserverQuery();if err!=nil{t.Fatal(err)};query:=admission.ticket;if err=n.recordNativeReturn(query,55);err!=nil{t.Fatal("equal PDH/kernel values collided",err)};if _,err=n.reserveObserverQuery();err==nil{t.Fatal("same owner issued a replacement observer query")}
 ticket,index,err:=n.beginNativeClose(55,false);if err!=nil||ticket!=file{t.Fatal("wrong native namespace selected")};if n.finishNativeClose(ticket,index,true)!=nil||query.slots[0].value!=55{t.Fatal("kernel close refunded PDH original")};ticket,index,err=n.beginNativeClose(55,true);if err!=nil||ticket!=query{t.Fatal("query owner unavailable")};_ = n.finishNativeClose(ticket,index,false);if !query.slots[0].unknown||n.originalCensusSettled(){t.Fatal("failed query Close labelled settled")};ticket,index,err=n.beginNativeClose(55,true);if err!=nil{t.Fatal(err)};_ = n.finishNativeClose(ticket,index,true)
 reused,err:=n.reserveNativeOriginal(debianProofOriginalFile);if err!=nil{t.Fatal(err)};if err=n.recordNativeReturn(reused,55);err!=nil||reused==file{t.Fatal("numeric reuse adopted an old entry")};if n.finishNativeClose(file,0,true)==nil{t.Fatal("old close receipt settled reused original")}
 forged:=&debianProofOriginalTicket{owner:n,kind:debianProofOriginalFile,pending:true,slots:make([]debianProofOriginalSlot,1)};if n.recordNativeReturn(forged,77)==nil{t.Fatal("unissued same-owner ticket accepted")};other,_:=projectionInertNative();if other.recordNativeReturn(reused,55)==nil{t.Fatal("foreign owner accepted original ticket")}
 invalid,err:=n.reserveNativeOriginal(debianProofOriginalFile);if err!=nil{t.Fatal(err)};before:=n.census.reserved;if err=n.recordKernelCallReturn(invalid,errors.New("synthetic direct false"),windows.InvalidHandle);err!=nil||n.census.reserved!=before-1||invalid.slots[0].value!=0{t.Fatal("INVALID_HANDLE_VALUE fabricated ownership")}
 partial,err:=n.reserveNativeOriginal(debianProofOriginalPI);if err!=nil{t.Fatal(err)};before=n.census.reserved;if n.recordKernelCallReturn(partial,errors.New("synthetic partial PI"),201,0)==nil||n.census.reserved!=before||!partial.slots[1].unknown{t.Fatal("partial missing native output transiently refunded capacity")}
}
func TestDebianCensusPreparedSourceSingleUseRetainsLatePartialGuard(t *testing.T){
 n,p:=censusPreparedSource(t);if err:=n.consumeSource(context.Background(),p.source);err!=nil{t.Fatal(err)};if n.consumeSource(context.Background(),p.source)==nil{t.Fatal("same prepared admission consumed twice")};foreign,_:=projectionInertNative();if foreign.consumeSource(context.Background(),p.source)==nil{t.Fatal("foreign receiver consumed admission")}
 n.retireSourceAdmissions();late:=&assemblyDebianFakeGuard{valid:true};if n.finishEvidenceAcquire(context.Background(),p.source,late,errors.New("synthetic late partial acquisition"))==nil||p.source.guard!=late||p.source.acquiring||!p.source.retiring{t.Fatal("late partial token lost or became dispatch authority")};if n.originalCensusSettled(){t.Fatal("partial synthetic owner fabricated closure")};_ = late.Close()
 // This lower-edge fake return deliberately is not a genuine Acquire receipt;
 // the production constructor/adopter is never called by this fixture.
}
func TestDebianCensusDeduplicatesSealedLifetimeRatherThanPathOrCopiedCount(t *testing.T){
 n:=newDebianProofNativeOwner(context.Background());objects:=[]backend.DocumentImageAssemblyEvidenceObject{{Role:"source",Path:filepath.Join(t.TempDir(),"never-opened.fixture"),Bytes:1,Digest:assemblyCatalogDigest([]byte("x"))}}
 a,err:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(objects);if err!=nil{t.Fatal(err)};p,err:=n.registerEvidenceSource(context.Background(),a);if err!=nil{t.Fatal(err)};before:=n.census.reserved;same,err:=n.registerSource(context.Background(),p.source.lease,nil,nil,nil);if err!=nil||same!=p.source||n.census.reserved!=before{t.Fatal("same original lifetime counted twice")}
 b,err:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(objects);if err!=nil{t.Fatal(err)};other,err:=n.registerEvidenceSource(context.Background(),b);if err!=nil||other.source.charge.SameRegistry(p.source.charge)||n.census.reserved!=2*before{t.Fatal("same path reused a distinct original lifetime")};n.retireSourceAdmissions();if !n.settleSourceAdmissions(context.Background())||!n.originalCensusSettled(){t.Fatal("concretely unstarted prepared owners failed known no-allocation retirement")}
}
func TestDebianCensusObserverHasNoEffectsBeforeRealSourceAndQueryAdmission(t *testing.T){
 n:=newDebianProofNativeOwner(context.Background());opens:=0;n.observer.calls=&debianProofObserverCalls{Open:func(*uintptr)uint32{opens++;return 1},Close:func(uintptr)uint32{return 0}}
 if opens!=0||n.observerStarted.Load(){t.Fatal("constructor started native observer")};if _,err:=n.reserveObserverQuery();err==nil{t.Fatal("query issued without sealed source")};forged:=&debianProofQueryAdmission{owner:n,ticket:&debianProofOriginalTicket{owner:n,kind:debianProofOriginalQuery,pending:true,slots:make([]debianProofOriginalSlot,1)}};if n.startObserver(context.Background(),forged)==nil||opens!=0{t.Fatal("unissued query reached fake Open")}
}
func TestDebianCensusPartialPDHQueryRequiresItsExactCloseAcknowledgement(t *testing.T){
 for _,closeStatus:=range []uint32{0,1}{t.Run(string(rune('0'+closeStatus)),func(t *testing.T){n,_:=censusPreparedSource(t);closed:=make(chan uintptr,1);n.observer.calls=&debianProofObserverCalls{Open:func(q *uintptr)uint32{*q=55;return 1},Close:func(q uintptr)uint32{closed<-q;return closeStatus}}
  p,err:=n.reserveObserverQuery();if err!=nil{t.Fatal(err)};if err=n.startObserver(context.Background(),p);err!=nil{t.Fatal(err)};select{case<-n.observer.done:case<-time.After(time.Second):t.Fatal("fake partial observer failed to return")};select{case q:=<-closed:if q!=55{t.Fatal("wrong original PDH query closed")};default:t.Fatal("partial PDH error discarded original")}
  n.retire();settled:=n.settle();sources:=n.settleSourceAdmissions(context.Background());if !sources{t.Fatal("unstarted prepared source did not retire")};if closeStatus==0{if !settled||!n.originalCensusSettled()||n.observer.query!=0{t.Fatal("positively closed partial query remained charged")}}else{if settled||n.originalCensusSettled()||n.observer.query!=55||p.ticket.slots[0].value!=55{t.Fatal("failed query closure discarded original or capacity")}}
 })}
}
func TestDebianCensusBlockedPDHOpenCannotDelayOriginalJobRetirement(t *testing.T){
 n,_:=censusPreparedSource(t);entered,release,terminated:=make(chan struct{}),make(chan struct{}),make(chan struct{},1);var once sync.Once;unblock:=func(){once.Do(func(){close(release)})};defer unblock()
 n.observer.calls=&debianProofObserverCalls{Open:func(q *uintptr)uint32{*q=55;close(entered);<-release;return 1},Close:func(q uintptr)uint32{if q!=55{t.Error("wrong fake original query")};return 0}}
 k:=&inertDebianKernel{closed:map[windows.Handle]bool{}};n.calls=k.calls();n.calls.TerminateJob=func(h windows.Handle,_ uint32)error{if h!=77{t.Error("wrong fake original Job")};terminated<-struct{}{};return nil}
 ticket,err:=n.reserveNativeOriginal(debianProofOriginalJob);if err!=nil{t.Fatal(err)};if err=n.recordKernelReturn(ticket,77);err!=nil{t.Fatal(err)};n.jobs=[]*debianProofJob{{handle:77}}
 p,err:=n.reserveObserverQuery();if err!=nil{t.Fatal(err)};if err=n.startObserver(context.Background(),p);err!=nil{t.Fatal(err)};select{case<-entered:case<-time.After(time.Second):t.Fatal("fake query not entered")}
 retired:=make(chan struct{});go func(){n.retire();close(retired)}();select{case<-retired:case<-time.After(time.Second):t.Fatal("pending query blocked retirement latch")};select{case<-terminated:case<-time.After(time.Second):t.Fatal("original Job retirement waited for query/registry IO")};if !p.ticket.pending||n.originalCensusSettled(){t.Fatal("pending query lost its original capacity")};unblock()
 select{case<-n.observer.done:case<-time.After(time.Second):t.Fatal("late partial query failed to join")};if p.ticket.pending||p.ticket.slots[0].value!=0{t.Fatal("late nonzero query did not receive exact query Close")}
 if !settleInertDebianOwner(n)||!n.settleSourceAdmissions(context.Background())||!n.originalCensusSettled(){t.Fatal("changed fake original completion failed exact final settlement")}
}
func TestDebianCensusSuccessWithoutOriginalQueryRemainsUnknown(t *testing.T){
 n,_:=censusPreparedSource(t);n.observer.calls=&debianProofObserverCalls{Open:func(*uintptr)uint32{return 0},Close:func(uintptr)uint32{t.Error("zero original query must not be closed");return 0}};p,err:=n.reserveObserverQuery();if err!=nil{t.Fatal(err)};if err=n.startObserver(context.Background(),p);err!=nil{t.Fatal(err)};select{case<-n.observer.done:case<-time.After(time.Second):t.Fatal("fake missing query did not return")};n.retire();if n.settle(){t.Fatal("success status without original query fabricated cleanup")};_ = n.settleSourceAdmissions(context.Background());if n.originalCensusSettled()||p.ticket.pending||!p.ticket.slots[0].unknown||n.census.reserved!=1{t.Fatal("ambiguous query output lost original charge")}
}
func TestDebianCensusInheritedStdioCannotSettleFromParentCloseOrUnknownCreate(t *testing.T){
 n,_:=projectionInertNative();pi,child,err:=n.reserveOriginalChild();if err!=nil{t.Fatal(err)};job:=&debianProofJob{generation:1};life:=&debianProofLifetime{job:job,piTicket:pi,inherited:child};if err=n.recordKernelReturn(pi,201,202);err!=nil{t.Fatal(err)};if err=n.recordInheritedCreateReturn(life,windows.ProcessInformation{Process:201,Thread:202},errors.New("synthetic partial Create"));err!=nil{t.Fatal(err)}
 // Parent PI/stdio closure is not child derivative closure. These are exact
 // fake Close acknowledgements; no original process/Job observation exists.
 for _,h:=range []uintptr{201,202}{ticket,index,e:=n.beginNativeClose(h,false);if e!=nil{t.Fatal(e)};_ = n.finishNativeClose(ticket,index,true)};if n.census.reserved!=3||n.settleInheritedStdio(life){t.Fatal("parent Close refunded child copies")}
 life.processSettled=true;if n.settleInheritedStdio(life){t.Fatal("process exit alone settled exact Job copies")};job.settled=true;job.borrowers=1;if n.settleInheritedStdio(life){t.Fatal("LOCAL132 borrower erased child copies")};job.borrowers=0;if !n.settleInheritedStdio(life)||n.census.reserved!=0{t.Fatal("joint original process/Job fake settlement failed")}
 absentPI,absentChild,e:=n.reserveOriginalChild();if e!=nil{t.Fatal(e)};absent:=&debianProofLifetime{piTicket:absentPI,inherited:absentChild};_ = n.recordKernelCallReturn(absentPI,errors.New("synthetic direct false/noPI"),0,0);if n.recordInheritedCreateReturn(absent,windows.ProcessInformation{},errors.New("synthetic direct false/noPI"))!=nil||!n.settleInheritedStdio(absent){t.Fatal("known unallocated Create did not settle copies")}
 unknownPI,unknownChild,e:=n.reserveOriginalChild();if e!=nil{t.Fatal(e)};unknown:=&debianProofLifetime{piTicket:unknownPI,inherited:unknownChild};if n.recordKernelCallReturn(unknownPI,nil,0,0)==nil{t.Fatal("success without original PI became known absence")};if n.recordInheritedCreateReturn(unknown,windows.ProcessInformation{},nil)!=nil||n.settleInheritedStdio(unknown)||n.census.reserved!=5{t.Fatal("missing identity with success status became known absence")}
}
