//go:build windows

package api

import (
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
type debianProofObserver struct {
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
func newDebianProofObserver()*debianProofObserver{o:=&debianProofObserver{closed:true,stop:make(chan struct{}),done:make(chan struct{})};go o.run();return o}
func (o *debianProofObserver) run(){
 defer close(o.done)
 defer func(){if o.query!=0{status,_,_:=proofPDHClose.Call(o.query);o.mu.Lock();o.closed=status==0;if status!=0{o.cleanupErr=errors.New("PDH query close not acknowledged")};o.mu.Unlock();if status==0{o.query,o.counter=0,0}}}()
 if status,_,_:=proofPDHOpen.Call(0,0,uintptr(unsafe.Pointer(&o.query)));status!=0{o.err=errors.New("PDH query unavailable");return}
 o.closed=false
 name,_:=windows.UTF16PtrFromString(`\Processor(_Total)\% Processor Time`)
 if status,_,_:=proofPDHAdd.Call(o.query,uintptr(unsafe.Pointer(name)),0,uintptr(unsafe.Pointer(&o.counter)));status!=0{o.err=errors.New("PDH CPU counter unavailable");return}
 primed:=false;ticker:=time.NewTicker(250*time.Millisecond);defer ticker.Stop()
 for {
  select{case <-o.stop:return;case <-ticker.C:}
  sample:=debianNativeSample{At:time.Now().UTC()};var m proofMemory;m.Length=uint32(unsafe.Sizeof(m));var value proofPDHDouble
  collected,_,_:=proofPDHCollect.Call(o.query)
  formatted:=uintptr(1);if primed && collected==0{formatted,_,_=proofPDHValue.Call(o.counter,0x200,0,uintptr(unsafe.Pointer(&value)))}
  memory,_,_:=proofMemoryStatus.Call(uintptr(unsafe.Pointer(&m)))
  if memory!=0 && m.TotalPhys>0 && m.AvailPhys<=m.TotalPhys && m.TotalCommit>0 && m.AvailCommit<=m.TotalCommit && formatted==0 && (value.Status==0 || value.Status==1) && !math.IsNaN(value.Value) && !math.IsInf(value.Value,0) && value.Value>=0 && value.Value<=100{sample.Physical,sample.Commit,sample.CPU,sample.Known=m.AvailPhys,m.AvailCommit,value.Value,true}
  primed=collected==0;o.mu.Lock();o.sample=sample;o.mu.Unlock()
 }
}
func(o *debianProofObserver) latest()debianNativeSample{o.mu.Lock();defer o.mu.Unlock();return o.sample}
func(o *debianProofObserver) awaitFresh(deadline time.Time)(debianNativeSample,error){for time.Now().Before(deadline){s:=o.latest();if s.Known && !s.At.After(time.Now()) && time.Since(s.At)<=time.Second{return s,nil};select{case<-o.done:return s,errors.New("native observer stopped");case<-time.After(25*time.Millisecond):}};return o.latest(),errors.New("native observer did not produce a fresh sample")}
func(o *debianProofObserver) settle(deadline time.Time)bool{o.stopOnce.Do(func(){close(o.stop)});left:=time.Until(deadline);if left<=0{return false};select{case<-o.done:o.mu.Lock();defer o.mu.Unlock();return o.closed&&o.cleanupErr==nil;case<-time.After(left):return false}}
