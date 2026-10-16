package api

import (
 "context"
 "errors"
 "sync"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
)

var errDebianProofCensus=errors.New("original producer resource census is unavailable or unsettled")
type debianProofOriginalKind uint8
const (
 debianProofOriginalJob debianProofOriginalKind=iota+1
 debianProofOriginalPipe
 debianProofOriginalPI
 debianProofOriginalFile
 debianProofOriginalReOpen
 debianProofOriginalQuery
 debianProofOriginalInheritedStdio
 debianProofOriginalBinaryAlias
 debianProofOriginalInheritedBinary
)
// Fixed native primitive sizes only; no caller/profile count or HANDLE issuer.
func debianProofOriginalSlots(kind debianProofOriginalKind)int{switch kind{case debianProofOriginalPipe,debianProofOriginalPI:return 2;case debianProofOriginalInheritedStdio:return 3;case debianProofOriginalInheritedBinary:return 184;case debianProofOriginalJob,debianProofOriginalFile,debianProofOriginalReOpen,debianProofOriginalQuery,debianProofOriginalBinaryAlias:return 1};return 0}
type debianProofOriginalSlot struct {value uintptr;closing,unknown bool}
type debianProofOriginalTicket struct {owner *debianProofNativeOwner;kind debianProofOriginalKind;pending,attempted bool;slots []debianProofOriginalSlot;childOf *debianProofOriginalTicket}
type debianProofSourceAdmission struct {
 owner *debianProofNativeOwner
 lease *dockerbe.DocumentImageAssemblyOriginalRegistryLease
 charge dockerbe.DocumentImageAssemblyOriginalRegistryCharge
 ceiling int
 evidence *dockerbe.DocumentImageAssemblyEvidenceAcquisition
 packages *dockerbe.DocumentImageAssemblyPackageAcquisition
 consumed,acquiring,retiring,released bool
 guard backend.DocumentImageAssemblyEvidenceGuard
 packageGuard backend.DocumentImageAssemblyPackageInputGuard
 binaryTransport *debianBinaryNativeTransport
}
type debianProofEvidenceAdmission struct {source *debianProofSourceAdmission}
type debianProofPackageAdmission struct {source *debianProofSourceAdmission}
type debianProofQueryAdmission struct {owner *debianProofNativeOwner;ticket *debianProofOriginalTicket;consumed bool}
// Census metadata refers to SAME original owners. It does not own replacement
// handles, reconstruct a process, validate trust, or authorize work itself.
type debianProofNativeCensus struct {mu sync.Mutex;owner *debianProofNativeOwner;retiring bool;reserved int;native []*debianProofOriginalTicket;sources []*debianProofSourceAdmission;query *debianProofOriginalTicket;transports []*debianBinaryNativeTransport}
func newDebianProofNativeCensus(o *debianProofNativeOwner)*debianProofNativeCensus{return &debianProofNativeCensus{owner:o}}
func debianProofChildDerivative(kind debianProofOriginalKind)bool{return kind==debianProofOriginalInheritedStdio||kind==debianProofOriginalInheritedBinary}
func(c *debianProofNativeCensus)hasNativeLocked(t *debianProofOriginalTicket)bool{for _,issued:=range c.native{if issued==t{return true}};return false}
func(c *debianProofNativeCensus)hasSourceLocked(s *debianProofSourceAdmission)bool{for _,issued:=range c.sources{if issued==s{return true}};return false}
func(o *debianProofNativeOwner)reserveNativeOriginal(kind debianProofOriginalKind)(*debianProofOriginalTicket,error){
 c:=o.census;if c==nil||c.owner!=o{return nil,errDebianProofCensus};slots:=debianProofOriginalSlots(kind);c.mu.Lock();defer c.mu.Unlock()
 if slots==0||debianProofChildDerivative(kind)||kind==debianProofOriginalBinaryAlias||kind==debianProofOriginalQuery||c.retiring||o.nativeRetired()||!time.Now().Before(o.workLimit())||c.reserved>1024-slots||len(c.native)>=1024{return nil,errDebianProofCensus};t:=&debianProofOriginalTicket{owner:o,kind:kind,pending:true,slots:make([]debianProofOriginalSlot,slots)};c.native=append(c.native,t);c.reserved+=slots;return t,nil
}
// One fixed Create attempt reserves PI2 plus its known inherited stdio3 as a
// single metadata transaction. No partly reserved pre-Create capacity leak.
func(o *debianProofNativeOwner)reserveOriginalChild()(*debianProofOriginalTicket,*debianProofOriginalTicket,error){
 c:=o.census;if c==nil||c.owner!=o{return nil,nil,errDebianProofCensus};c.mu.Lock();defer c.mu.Unlock();if c.retiring||o.nativeRetired()||!time.Now().Before(o.workLimit())||c.reserved>1019||len(c.native)>1022{return nil,nil,errDebianProofCensus}
 pi:=&debianProofOriginalTicket{owner:o,kind:debianProofOriginalPI,pending:true,slots:make([]debianProofOriginalSlot,2)};child:=&debianProofOriginalTicket{owner:o,kind:debianProofOriginalInheritedStdio,pending:true,slots:make([]debianProofOriginalSlot,3),childOf:pi};c.native=append(c.native,pi,child);c.reserved+=5;return pi,child,nil
}
func(o *debianProofNativeOwner)recordNativeReturn(t *debianProofOriginalTicket,values ...uintptr)error{
 return o.recordOriginalReturn(t,values,true)
}
// knownZero is derived only at the concrete API return edge. Missing outputs
// on success/partial allocation remain charged; no transient refund then repair.
func(o *debianProofNativeOwner)recordOriginalReturn(t *debianProofOriginalTicket,values []uintptr,knownZero bool)error{
 c:=o.census;if c==nil||t==nil||t.owner!=o{return errDebianProofCensus};c.mu.Lock();defer c.mu.Unlock();if !c.hasNativeLocked(t)||!t.pending||len(values)!=len(t.slots){return errDebianProofCensus};t.pending=false;var collision,unknown bool
 for i,v:=range values{t.slots[i].value=v;if v==0{if knownZero{c.reserved--}else{t.slots[i].unknown=true;unknown=true};continue};for j:=0;j<i;j++{if values[j]==v{collision=true;t.slots[i].unknown=true;t.slots[j].unknown=true}};for _,prior:=range c.native{if prior==t||debianProofChildDerivative(prior.kind)||debianProofChildDerivative(t.kind)||(prior.kind==debianProofOriginalQuery)!=(t.kind==debianProofOriginalQuery){continue};for _,s:=range prior.slots{if s.value==v{collision=true;t.slots[i].unknown=true}}}}
 if collision||unknown{return errDebianProofCensus};return nil
}
func(o *debianProofNativeOwner)beginNativeClose(value uintptr,query bool)(*debianProofOriginalTicket,int,error){
 c:=o.census;if c==nil||value==0{return nil,0,errDebianProofCensus};c.mu.Lock();defer c.mu.Unlock();var found *debianProofOriginalTicket;index:=0
 for _,t:=range c.native{if debianProofChildDerivative(t.kind)||(t.kind==debianProofOriginalQuery)!=query||t.pending{continue};for i,s:=range t.slots{if s.value==value{if found!=nil||s.closing{return nil,0,errDebianProofCensus};found,index=t,i}}};if found==nil{return nil,0,errDebianProofCensus};found.slots[index].closing=true;return found,index,nil
}
func(o *debianProofNativeOwner)finishNativeClose(t *debianProofOriginalTicket,index int,positive bool)error{
 c:=o.census;if c==nil||t==nil||t.owner!=o{return errDebianProofCensus};c.mu.Lock();defer c.mu.Unlock();if !c.hasNativeLocked(t)||index<0||index>=len(t.slots)||!t.slots[index].closing{return errDebianProofCensus};slot:=&t.slots[index];slot.closing=false;if positive{slot.value=0;slot.unknown=false;c.reserved--}else{slot.unknown=true};return nil
}
func(o *debianProofNativeOwner)registerSource(ctx context.Context,lease *dockerbe.DocumentImageAssemblyOriginalRegistryLease,evidence *dockerbe.DocumentImageAssemblyEvidenceAcquisition,packages *dockerbe.DocumentImageAssemblyPackageAcquisition,guard backend.DocumentImageAssemblyEvidenceGuard)(*debianProofSourceAdmission,error){
 if lease==nil||ctx.Err()!=nil||o.census==nil{return nil,errDebianProofCensus};charge,err:=lease.Charge();if err!=nil{return nil,err};ceiling,err:=charge.Slots();if err!=nil||ceiling<=0||ceiling>1024{return nil,errDebianProofCensus}
 c:=o.census;c.mu.Lock();defer c.mu.Unlock();if c.owner!=o||c.retiring||o.nativeRetired()||!time.Now().Before(o.workLimit())||ctx.Err()!=nil{return nil,errDebianProofCensus}
 for _,old:=range c.sources{if old.charge.SameRegistry(charge){if old.released||evidence!=nil||packages!=nil{return nil,errDebianProofCensus};return old,nil}}
 if c.reserved>1024-ceiling||len(c.sources)>=256{return nil,errDebianProofCensus};s:=&debianProofSourceAdmission{owner:o,lease:lease,charge:charge,ceiling:ceiling,evidence:evidence,packages:packages,guard:guard,consumed:guard!=nil};c.sources=append(c.sources,s);c.reserved+=ceiling;return s,nil
}
func(o *debianProofNativeOwner)registerEvidenceSource(ctx context.Context,a *dockerbe.DocumentImageAssemblyEvidenceAcquisition)(*debianProofEvidenceAdmission,error){
 if a==nil{return nil,errDebianProofCensus};lease,err:=a.RegistryLease();if err!=nil{return nil,err};s,err:=o.registerSource(ctx,lease,a,nil,nil);if err!=nil{a.RequestRetirement();return nil,err};return &debianProofEvidenceAdmission{s},nil
}
func(o *debianProofNativeOwner)registerExistingEvidenceSource(ctx context.Context,g backend.DocumentImageAssemblyEvidenceGuard)(*debianProofEvidenceAdmission,error){
 lease,err:=dockerbe.BorrowDocumentImageAssemblyEvidenceRegistry(ctx,g);if err!=nil{return nil,err};s,err:=o.registerSource(ctx,lease,nil,nil,g);if err!=nil{return nil,err};return &debianProofEvidenceAdmission{s},nil
}
func(o *debianProofNativeOwner)registerPackageSource(ctx context.Context,a *dockerbe.DocumentImageAssemblyPackageAcquisition)(*debianProofPackageAdmission,error){
 if a==nil{return nil,errDebianProofCensus};lease,err:=a.RegistryLease();if err!=nil{return nil,err};s,err:=o.registerSource(ctx,lease,nil,a,nil);if err!=nil{a.RequestRetirement();return nil,err};return &debianProofPackageAdmission{s},nil
}
func(o *debianProofNativeOwner)registerExistingPackageSource(ctx context.Context,g backend.DocumentImageAssemblyPackageInputGuard)(*debianProofPackageAdmission,error){
 lease,err:=dockerbe.BorrowDocumentImageAssemblyPackageRegistry(ctx,g);if err!=nil{return nil,err};s,err:=o.registerSource(ctx,lease,nil,nil,nil);if err!=nil{return nil,err};o.census.mu.Lock();s.packageGuard=g;s.consumed=true;o.census.mu.Unlock();return &debianProofPackageAdmission{s},nil
}
func(o *debianProofNativeOwner)consumeSource(ctx context.Context,s *debianProofSourceAdmission)error{
 c:=o.census;if c==nil||s==nil||s.owner!=o{return errDebianProofCensus};c.mu.Lock();defer c.mu.Unlock();if !c.hasSourceLocked(s)||c.retiring||o.nativeRetired()||!time.Now().Before(o.workLimit())||s.retiring||s.released||s.consumed||ctx.Err()!=nil{return errDebianProofCensus};s.consumed,s.acquiring=true,true;return nil
}
func(o *debianProofNativeOwner)acquireEvidenceSource(ctx context.Context,p *debianProofEvidenceAdmission)(backend.DocumentImageAssemblyEvidenceGuard,error){
 if p==nil||p.source==nil||p.source.evidence==nil{return nil,errDebianProofCensus};s:=p.source;if err:=o.consumeSource(ctx,s);err!=nil{return nil,err};g,err:=s.evidence.Acquire(ctx)
 err=o.finishEvidenceAcquire(ctx,s,g,err);return g,err
}
func(o *debianProofNativeOwner)finishEvidenceAcquire(ctx context.Context,s *debianProofSourceAdmission,g backend.DocumentImageAssemblyEvidenceGuard,err error)error{
 c:=o.census;if c==nil||s==nil||s.owner!=o{return errDebianProofCensus};c.mu.Lock();if !c.hasSourceLocked(s)||!s.acquiring{c.mu.Unlock();return errDebianProofCensus};s.guard=g;s.acquiring=false;retired:=c.retiring||s.retiring||o.nativeRetired()||ctx.Err()!=nil||!time.Now().Before(o.workLimit());c.mu.Unlock();if retired||g==nil&&err==nil{return errDebianProofCensus};return err
}
func(o *debianProofNativeOwner)acquirePackageSource(ctx context.Context,p *debianProofPackageAdmission)(backend.DocumentImageAssemblyPackageInputGuard,error){
 if p==nil||p.source==nil||p.source.packages==nil{return nil,errDebianProofCensus};s:=p.source;if err:=o.consumeSource(ctx,s);err!=nil{return nil,err};g,err:=s.packages.Acquire(ctx)
 c:=o.census;c.mu.Lock();s.packageGuard=g;s.acquiring=false;retired:=c.retiring||s.retiring||o.nativeRetired()||ctx.Err()!=nil||!time.Now().Before(o.workLimit());c.mu.Unlock();if retired||g==nil&&err==nil{return g,errDebianProofCensus};return g,err
}
func(o *debianProofNativeOwner)acquireEvidenceObjects(ctx context.Context,objects []backend.DocumentImageAssemblyEvidenceObject)(backend.DocumentImageAssemblyEvidenceGuard,error){
 a,err:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(objects);if err!=nil{return nil,err};p,err:=o.registerEvidenceSource(ctx,a);if err!=nil{return nil,err};return o.acquireEvidenceSource(ctx,p)
}
func(o *debianProofNativeOwner)retireSourceAdmissions(){
 c:=o.census;if c==nil{return};c.mu.Lock();c.retiring=true;sources:=append([]*debianProofSourceAdmission(nil),c.sources...);for _,s:=range sources{s.retiring=true};c.mu.Unlock()
 for _,s:=range sources{if s.evidence!=nil{s.evidence.RequestRetirement()};if s.packages!=nil{s.packages.RequestRetirement()}}
}
// Called after real source/candidate owners Close. A transferred stale token
// is not a cleanup capability; consult the SAME backend lifetime directly.
func(o *debianProofNativeOwner)settleSourceAdmissions(ctx context.Context)bool{
 c:=o.census;if c==nil{return false};c.mu.Lock();sources:=append([]*debianProofSourceAdmission(nil),c.sources...);c.mu.Unlock();all:=true
 for _,s:=range sources{state,err:=s.lease.RetirementState();if err!=nil||ctx.Err()!=nil||!state.CollectorStopped||!state.AllSlotsClosed||state.CloseInFlight{all=false;continue};c.mu.Lock();if s.acquiring{all=false}else if !s.released{s.released=true;c.reserved-=s.ceiling};c.mu.Unlock()};return all
}
func(o *debianProofNativeOwner)reserveObserverQuery()(*debianProofQueryAdmission,error){
 c:=o.census;if c==nil||c.owner!=o{return nil,errDebianProofCensus};c.mu.Lock();defer c.mu.Unlock();if len(c.sources)==0||c.query!=nil||c.retiring||o.nativeRetired()||!time.Now().Before(o.workLimit())||c.reserved>=1024||len(c.native)>=1024{return nil,errDebianProofCensus};t:=&debianProofOriginalTicket{owner:o,kind:debianProofOriginalQuery,pending:true,slots:make([]debianProofOriginalSlot,1)};c.query=t;c.native=append(c.native,t);c.reserved++;return &debianProofQueryAdmission{owner:o,ticket:t},nil
}
func(o *debianProofNativeOwner)originalCensusSettled()bool{
 c:=o.census;if c==nil{return false};c.mu.Lock();defer c.mu.Unlock();if c.reserved!=0{return false};for _,s:=range c.sources{if !s.released||s.acquiring{return false}};for _,t:=range c.native{if t.pending{return false};for _,s:=range t.slots{if s.value!=0||s.closing||s.unknown{return false}}};return true
}
func(o *debianProofNativeOwner)processCensusSettled()bool{
 c:=o.census;if c==nil{return false};c.mu.Lock();defer c.mu.Unlock();for _,t:=range c.native{if t.kind==debianProofOriginalFile||t.kind==debianProofOriginalReOpen{continue};if t.pending{return false};for _,s:=range t.slots{if s.value!=0||s.closing||s.unknown{return false}}};return true
}
