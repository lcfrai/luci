//go:build windows

package api

import (
 "context"
 "errors"
 "math"
 "sync"
 "time"
 "unsafe"

 "golang.org/x/sys/windows"
)

var proofKernel=windows.NewLazySystemDLL("kernel32.dll")
var proofMemoryStatus=proofKernel.NewProc("GlobalMemoryStatusEx")
var proofPDH=windows.NewLazySystemDLL("pdh.dll")
var proofPDHOpen=proofPDH.NewProc("PdhOpenQueryW")
var proofPDHAdd=proofPDH.NewProc("PdhAddEnglishCounterW")
var proofPDHCollect=proofPDH.NewProc("PdhCollectQueryData")
var proofPDHValue=proofPDH.NewProc("PdhGetFormattedCounterValue")
var proofPDHClose=proofPDH.NewProc("PdhCloseQuery")

type proofMemory struct {Length,Load uint32;TotalPhys,AvailPhys,TotalCommit,AvailCommit,TotalVirtual,AvailVirtual,Extended uint64}
type proofPDHDouble struct {Status uint32;_ uint32;Value float64}
type debianNativeSample struct {At time.Time `json:"at"`;Physical,Commit uint64;CPU float64;Known bool}
type debianProofObserverCalls struct {
 Open func(*uintptr)uint32
 Add func(uintptr,*uintptr)uint32
 Collect func(uintptr)uint32
 Value func(uintptr,*proofPDHDouble)uint32
 Memory func(*proofMemory)bool
 Close func(uintptr)uint32
}
func newDebianProofObserverCalls()*debianProofObserverCalls{return &debianProofObserverCalls{
 Open:func(q *uintptr)uint32{if proofPDHOpen.Find()!=nil{return 1};s,_,_:=proofPDHOpen.Call(0,0,uintptr(unsafe.Pointer(q)));return uint32(s)},
 Add:func(q uintptr,c *uintptr)uint32{if proofPDHAdd.Find()!=nil{return 1};name,_:=windows.UTF16PtrFromString(`\Processor(_Total)\% Processor Time`);s,_,_:=proofPDHAdd.Call(q,uintptr(unsafe.Pointer(name)),0,uintptr(unsafe.Pointer(c)));return uint32(s)},
 Collect:func(q uintptr)uint32{if proofPDHCollect.Find()!=nil{return 1};s,_,_:=proofPDHCollect.Call(q);return uint32(s)},
 Value:func(c uintptr,v *proofPDHDouble)uint32{if proofPDHValue.Find()!=nil{return 1};s,_,_:=proofPDHValue.Call(c,0x200,0,uintptr(unsafe.Pointer(v)));return uint32(s)},
 Memory:func(m *proofMemory)bool{if proofMemoryStatus.Find()!=nil{return false};s,_,_:=proofMemoryStatus.Call(uintptr(unsafe.Pointer(m)));return s!=0},
 Close:func(q uintptr)uint32{if proofPDHClose.Find()!=nil{return 1};s,_,_:=proofPDHClose.Call(q);return uint32(s)},
}}
type debianProofObserver struct {
 owner *debianProofNativeOwner
 ticket *debianProofOriginalTicket
 calls *debianProofObserverCalls
 mu sync.Mutex
 sample debianNativeSample
 query,counter uintptr
 stop chan struct{}
 done chan struct{}
 stopOnce sync.Once
 closed bool
 err error
 cleanupErr error
}
func newDebianProofObserver()*debianProofObserver{return &debianProofObserver{calls:newDebianProofObserverCalls(),closed:true,stop:make(chan struct{}),done:make(chan struct{})}}
func (o *debianProofObserver) run(ctx context.Context){
 defer close(o.done)
 defer func(){if o.query!=0{ticket,index,e:=o.owner.beginNativeClose(o.query,true);status:=uint32(1);if e==nil{status=o.calls.Close(o.query);e=o.owner.finishNativeClose(ticket,index,status==0)};o.mu.Lock();o.closed=e==nil&&status==0;if !o.closed{o.cleanupErr=errors.New("PDH query close not acknowledged")};o.mu.Unlock();if e==nil&&status==0{o.query,o.counter=0,0}}}()
 if o.owner==nil||o.ticket==nil||o.calls==nil{o.err=errDebianProofCensus;return}
 if ctx.Err()!=nil||o.owner.stopped.Load()||!time.Now().Before(o.owner.workDeadline){_ = o.owner.recordNativeReturn(o.ticket,0);o.err=errDebianProofCensus;return}
 status:=o.calls.Open(&o.query)
 // A partial nonzero query is recorded BEFORE status classification and is
 // closed only through PdhCloseQuery. Counter is subordinate, never HANDLE.
 if e:=o.owner.recordOriginalReturn(o.ticket,[]uintptr{o.query},status!=0&&o.query==0);e!=nil{o.err=e;return}
 if o.query!=0{o.closed=false};if status!=0||o.query==0{o.err=errors.New("PDH query unavailable");return}
 if ctx.Err()!=nil||o.owner.stopped.Load()||!time.Now().Before(o.owner.workDeadline){o.err=errDebianProofCensus;return}
 if status=o.calls.Add(o.query,&o.counter);status!=0{o.err=errors.New("PDH CPU counter unavailable");return}
 primed:=false;ticker:=time.NewTicker(250*time.Millisecond);defer ticker.Stop()
 for {
  select{case <-o.stop:return;case<-ctx.Done():return;case <-ticker.C:}
  if o.owner.stopped.Load()||!time.Now().Before(o.owner.workDeadline){return}
  sample:=debianNativeSample{At:time.Now().UTC()};var m proofMemory;m.Length=uint32(unsafe.Sizeof(m));var value proofPDHDouble
  collected:=o.calls.Collect(o.query)
  formatted:=uint32(1);if primed && collected==0{formatted=o.calls.Value(o.counter,&value)}
  memory:=o.calls.Memory(&m)
  if memory && m.TotalPhys>0 && m.AvailPhys<=m.TotalPhys && m.TotalCommit>0 && m.AvailCommit<=m.TotalCommit && formatted==0 && (value.Status==0 || value.Status==1) && !math.IsNaN(value.Value) && !math.IsInf(value.Value,0) && value.Value>=0 && value.Value<=100{sample.Physical,sample.Commit,sample.CPU,sample.Known=m.AvailPhys,m.AvailCommit,value.Value,true}
  primed=collected==0;o.mu.Lock();o.sample=sample;o.mu.Unlock()
 }
}
func(o *debianProofObserver) latest()debianNativeSample{o.mu.Lock();defer o.mu.Unlock();return o.sample}
func(o *debianProofObserver) awaitFresh(deadline time.Time)(debianNativeSample,error){for time.Now().Before(deadline){s:=o.latest();if s.Known && !s.At.After(time.Now()) && time.Since(s.At)<=time.Second{return s,nil};select{case<-o.done:return s,errors.New("native observer stopped");case<-time.After(25*time.Millisecond):}};return o.latest(),errors.New("native observer did not produce a fresh sample")}
func(o *debianProofObserver) settle(deadline time.Time)bool{o.stopOnce.Do(func(){close(o.stop)});left:=time.Until(deadline);if left<=0{return false};select{case<-o.done:o.mu.Lock();defer o.mu.Unlock();return o.closed&&o.cleanupErr==nil;case<-time.After(left):return false}}
