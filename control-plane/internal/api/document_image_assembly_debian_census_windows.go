//go:build windows

package api

import (
 "context"
 "time"

 "golang.org/x/sys/windows"
)

func(o *debianProofNativeOwner)nativeRetired()bool{return o==nil||o.stopped.Load()}
func(o *debianProofNativeOwner)initialAdmission(ctx context.Context)error{
 if ctx.Err()!=nil||o.stopped.Load()||!o.observerStarted.Load(){return errDebianProofCensus};s,err:=o.observer.awaitFresh(minProofTime(o.workDeadline,time.Now().Add(2*time.Second)));if err!=nil||s.Physical<8<<30||s.Commit<8<<30||s.CPU>=50||ctx.Err()!=nil||o.stopped.Load(){return errDebianProofCensus};return nil
}
// Same original owner performs each close; metadata cannot manufacture an ACK.
func(o *debianProofNativeOwner)closeKernelOriginal(h windows.Handle)error{
 ticket,index,err:=o.beginNativeClose(uintptr(h),false);if err!=nil{return err};err=o.calls.Close(h);if ack:=o.finishNativeClose(ticket,index,err==nil);ack!=nil{return ack};return err
}
func(o *debianProofNativeOwner)recordKernelCallReturn(t *debianProofOriginalTicket,callErr error,handles ...windows.Handle)error{
 values:=make([]uintptr,len(handles));allZero:=true;for i,h:=range handles{if validDebianProducedHandle(h){values[i]=uintptr(h);allZero=false}};return o.recordOriginalReturn(t,values,callErr!=nil&&allZero)
}
func debianProofOwnedKernel(h windows.Handle)windows.Handle{if validDebianProducedHandle(h){return h};return 0}
func(o *debianProofNativeOwner)recordInheritedCreateReturn(life *debianProofLifetime,pi windows.ProcessInformation,createErr error)error{
 if life==nil||life.inherited==nil||life.piTicket==nil||life.inherited.childOf!=life.piTicket{return errDebianProofCensus}
 // Direct Win32 false + no original PI is positively unallocated. Every
 // partial/valid/ambiguous acknowledgement retains all three possible copies.
 if createErr!=nil&&pi.Process==0&&pi.Thread==0{return o.recordNativeReturn(life.inherited,0,0,0)}
 return o.recordNativeReturn(life.inherited,1,2,3)
}
func(o *debianProofNativeOwner)settleInheritedStdio(life *debianProofLifetime)bool{
 if life==nil{return false};if life.inherited==nil{return true} // Create not attempted.
 c:=o.census;if c==nil{return false};t:=life.inherited
 c.mu.Lock();if !c.hasNativeLocked(t)||t.childOf!=life.piTicket||t.pending{c.mu.Unlock();return false};empty:=true;for _,slot:=range t.slots{if slot.value!=0{empty=false}};c.mu.Unlock();if empty{return true}
 if !life.processSettled||life.job==nil{return false};life.job.mu.Lock();jobSettled:=life.job.settled&&life.job.handle==0&&life.job.borrowers==0&&life.job.generation==1;life.job.mu.Unlock();if !jobSettled{return false}
 c.mu.Lock();defer c.mu.Unlock();if t.pending||t.childOf!=life.piTicket{return false};for i:=range t.slots{if t.slots[i].value!=0{t.slots[i]=debianProofOriginalSlot{};c.reserved--}};return true
}
func(o *debianProofNativeOwner)startObserver(ctx context.Context,p *debianProofQueryAdmission)error{
 c:=o.census;if c==nil||p==nil||p.owner!=o||p.ticket==nil||p.ticket.owner!=o||p.ticket.kind!=debianProofOriginalQuery{return errDebianProofCensus}
 c.mu.Lock();if c.query!=p.ticket||!c.hasNativeLocked(p.ticket)||p.consumed||!p.ticket.pending||o.observerStarted.Load()||len(c.sources)==0{c.mu.Unlock();return errDebianProofCensus};p.consumed=true
 refused:=c.retiring||o.stopped.Load()||ctx.Err()!=nil||!time.Now().Before(o.workDeadline);if !refused{o.observerStarted.Store(true)};c.mu.Unlock()
 if refused{_ = o.recordNativeReturn(p.ticket,0);return errDebianProofCensus}
 // All backing/query ownership is established BEFORE either goroutine. A
 // cancellation between this point and run is observed before its PdhOpen.
 o.observer.owner,o.observer.ticket=o,p.ticket
 go o.observer.run(ctx)
 go func(){defer close(o.watchDone);ticker:=time.NewTicker(100*time.Millisecond);defer ticker.Stop();primingUntil:=o.began.Add(2*time.Second)
  for{select{case<-o.stopWatch:return;case<-ctx.Done():o.retire();return;case<-ticker.C:
   s:=o.observer.latest();if !time.Now().Before(o.workDeadline)||time.Now().After(primingUntil)&&(!s.Known||time.Since(s.At)>time.Second||s.Physical<4<<30||s.Commit<4<<30||s.CPU>=50){o.retire();return}
  }}
 }();return nil
}
