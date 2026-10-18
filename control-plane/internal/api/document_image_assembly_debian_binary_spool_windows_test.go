//go:build windows

package api

import (
 "context"
 "errors"
 "sync"
 "testing"
 "time"

 "golang.org/x/sys/windows"
 "github.com/lcfrai/luci/control-plane/internal/backend"
)

// Lower original kernel schedules only: no DLL/native filesystem/process,
// namespace acquisition, producer/lease/caller construction or174 preimages.
type binarySpoolFakeOpen struct{path string;write,shareWrite,closed bool;id uint32}
type binarySpoolFakeKernel struct{
 mu sync.Mutex
 next windows.Handle
 opens map[windows.Handle]*binarySpoolFakeOpen
 data map[string][]byte
 creates,reopens,writes,closes int
 short bool
 partialCreate,partialReopen,failClose bool
}
func(k *binarySpoolFakeKernel)calls()*debianProofWindowsCalls{return &debianProofWindowsCalls{
 FileCreate:func(name *uint16,access,share uint32,_ *windows.SecurityAttributes,creation,flags uint32,_ windows.Handle)(windows.Handle,error){
  k.mu.Lock();defer k.mu.Unlock();path:=windows.UTF16PtrToString(name);k.creates++
  if creation==windows.CREATE_NEW{if _,exists:=k.data[path];exists{return windows.InvalidHandle,windows.ERROR_FILE_EXISTS};k.data[path]=nil}
  var id uint32;for _,prior:=range k.opens{if prior.path!=path{continue};id=prior.id;if !prior.closed&&prior.write&&share&windows.FILE_SHARE_WRITE==0{return windows.InvalidHandle,windows.ERROR_SHARING_VIOLATION}}
  if id==0{id=uint32(len(k.data))};k.next++;h:=k.next;k.opens[h]=&binarySpoolFakeOpen{path:path,write:access&windows.GENERIC_WRITE!=0,shareWrite:share&windows.FILE_SHARE_WRITE!=0,id:id}
  if k.partialCreate{return h,errors.New("fake valid original plus error")};return h,nil
 },
 FileWrite:func(h windows.Handle,b []byte,n *uint32,_ *windows.Overlapped)error{k.mu.Lock();defer k.mu.Unlock();k.writes++;f:=k.opens[h];if f==nil||f.closed||!f.write{return errors.New("fake write ownership")};count:=len(b);if k.short&&k.writes==1&&count>3{count=3};*n=uint32(count);k.data[f.path]=append(k.data[f.path],b[:count]...);return nil},
 FileFlush:func(windows.Handle)error{return nil},
 FileInfo:func(h windows.Handle,info *windows.ByHandleFileInformation)error{k.mu.Lock();defer k.mu.Unlock();f:=k.opens[h];if f==nil||f.closed{return errors.New("fake original not open")};*info=windows.ByHandleFileInformation{VolumeSerialNumber:1,FileIndexLow:f.id,NumberOfLinks:1,FileSizeLow:uint32(len(k.data[f.path]))};return nil},
 FileReOpen:func(h windows.Handle)(windows.Handle,error){k.mu.Lock();defer k.mu.Unlock();k.reopens++;source:=k.opens[h];if source==nil||source.closed{return 0,errors.New("fake reopen original absent")};k.next++;read:=k.next;k.opens[read]=&binarySpoolFakeOpen{path:source.path,shareWrite:true,id:source.id};if k.partialReopen{return read,errors.New("fake partial ReOpen ACK")};return read,nil},
 Close:func(h windows.Handle)error{k.mu.Lock();defer k.mu.Unlock();k.closes++;f:=k.opens[h];if f==nil||f.closed{return errors.New("fake duplicate Close")};if k.failClose{return errors.New("fake Close failure")};f.closed=true;return nil},
}}
type binarySpoolFakeNamespace struct{mu sync.Mutex;closed bool;closes int}
func(g *binarySpoolFakeNamespace)CheckCurrent(context.Context)error{g.mu.Lock();defer g.mu.Unlock();if g.closed{return errDebianBinarySpool};return nil}
func(g *binarySpoolFakeNamespace)Transfer(context.Context)(backend.DocumentImageAssemblyNamespaceGuard,error){return nil,errDebianBinarySpool}
func(g *binarySpoolFakeNamespace)Close()error{g.mu.Lock();defer g.mu.Unlock();g.closed=true;g.closes++;return nil}
func binarySpoolLowerFixture(t *testing.T)(*debianBinarySpoolSet,*binarySpoolFakeKernel){
 t.Helper();now:=time.Now();o:=&debianProofNativeOwner{began:now,workDeadline:now.Add(time.Minute),wholeDeadline:now.Add(2*time.Minute),cancel:func(){}};o.census=newDebianProofNativeCensus(o)
 kernel:=&binarySpoolFakeKernel{next:100,opens:map[windows.Handle]*binarySpoolFakeOpen{},data:map[string][]byte{}};o.calls=kernel.calls()
 pool,err:=o.reserveBinarySpoolPool();if err!=nil{t.Fatal(err)}
 expected,_:=binaryRawExpectedFixture();decoder,err:=newDebianBinaryRawDecoder(expected,now,now.Add(time.Minute));if err!=nil{t.Fatal(err)}
 // Fabricated lower token/path only. This bypasses real namespace constructor
 // intentionally and cannot establish sealed-source authority or a live run.
 s:=&debianBinarySpoolSet{owner:o,decoder:decoder,pool:pool,native:&debianBinarySpoolNative{},namespace:&debianProofNamespaceAdmission{source:&debianProofSourceAdmission{owner:o,namespacePath:`E:\fake-spool`}},namespaceGuard:&binarySpoolFakeNamespace{}}
 o.binarySpools=[]*debianBinarySpoolSet{s};return s,kernel
}
func binarySpoolTinyView(t *testing.T,s *debianBinarySpoolSet,payload []byte)debianBinaryRawView{
 t.Helper();frame,_:=binaryRawTestFrame(1,0,payload);input:=append([]byte(debianBinaryRawMagic),frame...);r,e:=s.decoder.beginRead(debianBinaryRawStdout,len(input),time.Now());if e!=nil{t.Fatal(e)};if e=s.decoder.observeRead(r,input,nil,time.Now());e!=nil{t.Fatal(e)};v,e:=s.decoder.nextFrame();if e!=nil{t.Fatal(e)};return v
}
func TestDebianBinarySpoolFixed75PoolCountsOnceAndSealsUnattempted(t *testing.T){
 for _,before:=range []int{949,950}{t.Run(map[int]string{949:"exact-boundary",950:"over-capacity"}[before],func(t *testing.T){
  now:=time.Now();o:=&debianProofNativeOwner{began:now,workDeadline:now.Add(time.Minute),wholeDeadline:now.Add(2*time.Minute),cancel:func(){}};o.census=newDebianProofNativeCensus(o);for i:=0;i<before;i++{if _,e:=o.reserveNativeOriginal(debianProofOriginalJob);e!=nil{t.Fatal(e)}}
  count:=len(o.census.native);p,e:=o.reserveBinarySpoolPool()
  if before==950{if e==nil||p!=nil||o.census.reserved!=950||len(o.census.native)!=count{t.Fatal("partial pool on refusal")};return}
  if e!=nil||o.census.reserved!=1024{t.Fatal("fixed75 boundary",e)};ticket,e:=p.claim(0,0);if e!=nil||o.census.reserved!=1024{t.Fatal("per-call double reservation")}
  if _,e=p.claim(0,0);e==nil{t.Fatal("reused pool slot")};p.seal();if o.census.reserved!=950||!ticket.pending{t.Fatal("claimed pending refunded")}
  if e=o.recordOriginalReturn(ticket,[]uintptr{0},false);e==nil||o.census.reserved!=950{t.Fatal("unknown zero became unallocated")}
 })}
}
func TestDebianBinarySpoolPoolCopiesAndUnknownOriginalCannotSettle(t *testing.T){
 s,_:=binarySpoolLowerFixture(t);o:=s.owner;copyPool:=*s.pool
 if _,e:=copyPool.claim(0,0);e==nil{t.Fatal("value copy became issuer")};copyPool.seal();if o.census.reserved!=75{t.Fatal("copied pool settled original claims")}
 if _,e:=o.reserveBinarySpoolPool();e==nil||o.census.reserved!=75{t.Fatal("second pool allocated")}
 ticket,e:=s.pool.claim(0,0);if e!=nil{t.Fatal(e)};s.pool.seal()
 if e=o.recordOriginalReturn(ticket,[]uintptr{0},false);e==nil||s.pool.settled()||o.census.reserved!=1{t.Fatal("unknown allocation disappeared on zero handle")}
}
func TestDebianBinarySpoolWritesShortWithoutRefundAndKeepsReadBridge(t *testing.T){
 s,k:=binarySpoolLowerFixture(t);k.short=true;v:=binarySpoolTinyView(t,s,[]byte("abcdefghij"))
 if e:=s.persistFrame(context.Background(),v);e!=nil{t.Fatal(e)}
 if s.writeDebit!=17||s.writeCalls!=2||s.nextFrame!=1||s.native.files[0].writer!=0||s.native.files[0].bridge==0{t.Fatal("short debit/original bridge lost")}
 if string(k.data[s.path(0)])!="abcdefghij"||k.reopens!=1{t.Fatal("raw original bytes")}
 if _,e:=v.descriptor();e==nil{t.Fatal("frame view survived advance")}
 // ReOpen is a distinct read-open, not a reduced Duplicate of the writer.
 bridge:=k.opens[s.native.files[0].bridge];if bridge.write||!bridge.shareWrite{t.Fatal("fake open-instance sharing model drift")}
}
func TestDebianBinarySpoolPartialOriginalACKsAndCloseFailureRemainOwned(t *testing.T){
 for _,mode:=range []string{"create","reopen","close"}{t.Run(mode,func(t *testing.T){
  s,k:=binarySpoolLowerFixture(t);k.partialCreate=mode=="create";k.partialReopen=mode=="reopen";k.failClose=mode=="close"
  v:=binarySpoolTinyView(t,s,[]byte("x"));if e:=s.persistFrame(context.Background(),v);e==nil{t.Fatal("partial failure accepted")};f:=s.native.files[0]
  if f.writer==0||mode!="create"&&f.bridge==0||!s.failed||!s.retiring{t.Fatal("exact partial original lost")}
  if mode=="close"{before:=k.closes;if s.closeRetiredRaw(context.Background())==nil||k.closes!=before{t.Fatal("failed Close automatically retried")}}
  for h,open:=range k.opens{if !open.closed&&h!=f.writer&&h!=f.bridge{t.Fatal("unretained allocated fake original",h)}}
 })}
}
func binarySpoolStderrLife(t *testing.T,s *debianBinarySpoolSet,h windows.Handle){
 t.Helper();ticket,e:=s.owner.reserveNativeOriginal(debianProofOriginalPipe);if e!=nil{t.Fatal(e)};if e=s.owner.recordNativeReturn(ticket,uintptr(h),uintptr(h+1));e!=nil{t.Fatal(e)}
 life:=&debianProofLifetime{handles:[]windows.Handle{0,0,h}};s.owner.lifetimes=append(s.owner.lifetimes,life);if e=s.bindStderrLife(life);e!=nil{t.Fatal(e)}
}
func TestDebianBinarySpoolStderrCopiesActualBytesAndRequiresEOF(t *testing.T){
 s,_:=binarySpoolLowerFixture(t);binarySpoolStderrLife(t,s,900)
 source:=[]byte("ab");offset:=0;s.owner.calls.Read=func(h windows.Handle,b []byte,n *uint32,_ *windows.Overlapped)error{if h!=900||len(b)!=1{t.Fatal("not exact owned EOF1 reader")};if offset==len(source){return windows.ERROR_BROKEN_PIPE};b[0]=source[offset];offset++;*n=1;return nil}
 if e:=s.readStderrOnce(context.Background());e!=nil{t.Fatal(e)};source[0]='z'
 if s.stderrEOF||s.stderrBytes!=1||s.stderr[0]!='a'{t.Fatal("stderr reconstructed or input alias retained")}
 if e:=s.readStderrOnce(context.Background());e!=nil{t.Fatal(e)};if e:=s.readStderrOnce(context.Background());e!=nil{t.Fatal(e)}
 if !s.stderrEOF||string(s.stderr[:s.stderrBytes])!="ab"||s.decoder.observation().StderrDebit!=3{t.Fatal("actual stderr bytes/EOF not coupled")}
 if s.readStderrOnce(context.Background())==nil{t.Fatal("read after EOF")}
}
func TestDebianBinarySpoolEmptyStderrNeedsActualZeroEOF(t *testing.T){
 s,_:=binarySpoolLowerFixture(t);binarySpoolStderrLife(t,s,900);s.owner.calls.Read=func(windows.Handle,[]byte,*uint32,*windows.Overlapped)error{return windows.ERROR_BROKEN_PIPE}
 if e:=s.readStderrOnce(context.Background());e!=nil||!s.stderrEOF||s.stderrBytes!=0||s.decoder.observation().StderrDebit!=1{t.Fatal("empty EOF was not actually observed",e)}
 if e:=s.begin(context.Background());e!=nil{t.Fatal(e)};if e:=s.persistSmallFixed(context.Background(),23,s.stderr[:s.stderrBytes]);e!=nil{t.Fatal(e)};_ = s.end(nil)
 if s.objects[23].Bytes!=0||s.objects[23].SHA256!=binarySpoolSHA(nil)||s.writeCalls!=0{t.Fatal("empty output had invented write bytes")}
}
func TestDebianBinarySpoolBlockedStderrRetiresBeforeActualReturn(t *testing.T){
 s,_:=binarySpoolLowerFixture(t);binarySpoolStderrLife(t,s,900);entered,release,done:=make(chan struct{}),make(chan struct{}),make(chan error,1)
 s.owner.calls.Read=func(windows.Handle,[]byte,*uint32,*windows.Overlapped)error{close(entered);<-release;return windows.ERROR_BROKEN_PIPE}
 go func(){done<-s.readStderrOnce(context.Background())}();<-entered
 s.owner.retire();if !s.retiring||!s.stderrBorrowed()||s.claimStderrClose()||s.closeRetiredRaw(context.Background())==nil{t.Fatal("blocked actual read lost borrower/latch")}
 if s.namespaceGuard.(*binarySpoolFakeNamespace).closes!=0{t.Fatal("collector state used as API borrower settlement")}
 close(release);if <-done==nil||s.stderrEOF||s.stderrBorrowed()||!s.decoder.observation().Pending{t.Fatal("late EOF became accepted or borrowed backing escaped")}
}
func TestDebianBinarySpoolStderrFailuresAreStickyAndFullyCharged(t *testing.T){
 for _,mode:=range []string{"zero-success","partial-error","invalid-count","shared-call-cap"}{t.Run(mode,func(t *testing.T){
  s,_:=binarySpoolLowerFixture(t);binarySpoolStderrLife(t,s,900);called:=0
  s.owner.calls.Read=func(_ windows.Handle,b []byte,n *uint32,_ *windows.Overlapped)error{called++;switch mode{case "partial-error":b[0]=9;*n=1;return errors.New("fake partial native read");case "invalid-count":*n=2};return nil}
  if mode=="shared-call-cap"{s.decoder.state.calls=debianBinaryRawCallsMax}
  if s.readStderrOnce(context.Background())==nil||!s.failed||s.stderrEOF||s.stderrBytes!=0{t.Fatal("stderr failure fabricated complete bytes")}
  if mode=="shared-call-cap"{if called!=0{t.Fatal("effect after combined cap")}}else if s.decoder.observation().StderrDebit!=1{t.Fatal("failed pending read refunded")}
 })}
}

type binarySpoolFakeEvidence struct {
 kernel *binarySpoolFakeKernel
 objects map[string]backend.DocumentImageAssemblyEvidenceObject
 readCalls int
 maximum int64
 corrupt bool
}
func(g *binarySpoolFakeEvidence)CheckCurrent(context.Context)error{return nil}
func(g *binarySpoolFakeEvidence)ReadBounded(_ context.Context,role string,maximum int64)([]byte,error){
 g.readCalls++;g.maximum=maximum;object,ok:=g.objects[role];if !ok||maximum<1||maximum<object.Bytes{return nil,errDebianBinarySpool}
 g.kernel.mu.Lock();b:=append([]byte{},g.kernel.data[object.Path]...);g.kernel.mu.Unlock();if g.corrupt{b=append(b,99)};return b,nil
}
func(g *binarySpoolFakeEvidence)Transfer(context.Context)(backend.DocumentImageAssemblyEvidenceGuard,error){return nil,errDebianBinarySpool}
func(g *binarySpoolFakeEvidence)Close()error{return nil}
func TestDebianBinarySpoolRestrictiveIdentityJoinRequiresActualReadHash(t *testing.T){
 for _,mode:=range []string{"raw-byte-match","changed-bytes","competing-writer","foreign-file-id","empty-stderr"}{t.Run(mode,func(t *testing.T){
  s,k:=binarySpoolLowerFixture(t);slot:=0
  if mode=="empty-stderr"{slot=23;if e:=s.begin(context.Background());e!=nil{t.Fatal(e)};e:=s.persistSmallFixed(context.Background(),23,nil);if e!=nil{t.Fatal(e)};_ = s.end(nil)}else{v:=binarySpoolTinyView(t,s,[]byte("raw"));if e:=s.persistFrame(context.Background(),v);e!=nil{t.Fatal(e)}}
  v:=s.objects[slot];object:=backend.DocumentImageAssemblyEvidenceObject{Role:v.Role,Path:s.path(slot),Bytes:v.Bytes,Digest:v.SHA256};guard:=&binarySpoolFakeEvidence{kernel:k,objects:map[string]backend.DocumentImageAssemblyEvidenceObject{v.Role:object},corrupt:mode=="changed-bytes"}
  if mode=="competing-writer"{bridge:=k.opens[s.native.files[slot].bridge];k.opens[5000]=&binarySpoolFakeOpen{path:bridge.path,write:true,id:bridge.id}}
  if mode=="foreign-file-id"{s.native.files[slot].info.FileIndexLow++}
  if e:=s.begin(context.Background());e!=nil{t.Fatal(e)};e:=s.joinAndHash(context.Background(),slot,guard,object);_ = s.end(e)
  positive:=mode=="raw-byte-match"||mode=="empty-stderr"
  if positive{if e!=nil||guard.readCalls!=1||s.native.files[slot].bridge!=0||s.native.files[slot].joined!=0{t.Fatal("actual hash/identity handoff",e)};if mode=="empty-stderr"&&guard.maximum!=1{t.Fatal("empty verifier maximum was not positive")}}else if e==nil{t.Fatal("unverified/changed original accepted")}
  if mode=="changed-bytes"&&guard.readCalls!=1{t.Fatal("identity/CheckCurrent substituted for raw bytes")}
  // This fake guard/call table is NOT a positive actual namespace/evidence
  // acquisition or a producer completion; full native proof is outstanding.
 })}
}
func TestDebianBinarySpoolBlockedOutputKeepsBackingAndLateOriginal(t *testing.T){
 for _,mode:=range []string{"create","write","flush","reopen"}{t.Run(mode,func(t *testing.T){
  s,k:=binarySpoolLowerFixture(t);v:=binarySpoolTinyView(t,s,[]byte("original"));entered,release,done:=make(chan struct{}),make(chan struct{}),make(chan error,1)
  switch mode{
  case "create":call:=s.owner.calls.FileCreate;s.owner.calls.FileCreate=func(a *uint16,b,c uint32,d *windows.SecurityAttributes,e,f uint32,g windows.Handle)(windows.Handle,error){h,err:=call(a,b,c,d,e,f,g);close(entered);<-release;return h,err}
  case "write":call:=s.owner.calls.FileWrite;s.owner.calls.FileWrite=func(h windows.Handle,b []byte,n *uint32,o *windows.Overlapped)error{close(entered);<-release;if string(b)!="original"{return errors.New("fake borrowed write backing changed")};return call(h,b,n,o)}
  case "flush":s.owner.calls.FileFlush=func(windows.Handle)error{close(entered);<-release;return nil}
  case "reopen":call:=s.owner.calls.FileReOpen;s.owner.calls.FileReOpen=func(h windows.Handle)(windows.Handle,error){read,err:=call(h);close(entered);<-release;return read,err}
  }
  go func(){done<-s.persistFrame(context.Background(),v)}();<-entered;s.owner.retire()
  if !s.retiring||s.closeRetiredRaw(context.Background())==nil||k.closes!=0{t.Fatal("blocked output retired/closed as if no borrower")}
  close(release);if <-done==nil||s.nextFrame!=0||!s.failed{t.Fatal("late original revived capture")}
  f:=s.native.files[0];if f.writer==0||mode=="reopen"&&f.bridge==0{t.Fatal("late nonzero original lost")}
  for h,open:=range k.opens{if !open.closed&&h!=f.writer&&h!=f.bridge{t.Fatal("allocated original untracked",h)}}
 })}
}
type binarySpoolTransferNamespace struct {
 next backend.DocumentImageAssemblyNamespaceGuard
 entered,release chan struct{}
 closes int
}
func(g *binarySpoolTransferNamespace)CheckCurrent(context.Context)error{return nil}
func(g *binarySpoolTransferNamespace)Transfer(context.Context)(backend.DocumentImageAssemblyNamespaceGuard,error){close(g.entered);<-g.release;return g.next,errors.New("fake late partial transfer")}
func(g *binarySpoolTransferNamespace)Close()error{g.closes++;return nil}
func TestDebianBinarySpoolNamespaceMoveKeepsLateCurrentToken(t *testing.T){
 s,_:=binarySpoolLowerFixture(t);current:=&binarySpoolFakeNamespace{};old:=&binarySpoolTransferNamespace{next:current,entered:make(chan struct{}),release:make(chan struct{})};s.namespaceGuard=old
 acquisition:=s.namespace.source // fabricated source metadata remains same pointer, not real lease proof
 done:=make(chan error,1);go func(){done<-s.transferRetainedNamespace(context.Background())}();<-old.entered;s.owner.retire()
 if s.closeRetiredRaw(context.Background())==nil||old.closes!=0||current.closes!=0{t.Fatal("namespace close while transfer borrower pending")}
 close(old.release);if <-done==nil||s.namespaceGuard!=current||s.namespace.source!=acquisition||s.namespace.source.namespaceGuard!=current{t.Fatal("partial current token/acquisition custody lost")}
 if s.closeRetiredRaw(context.Background())==nil{t.Fatal("fake lease metadata became actual closure")}
 if old.closes!=0||current.closes!=1{t.Fatal("stale namespace token closed")}
}
func TestDebianBinarySpoolExpiredFinalClockCannotReturnFinished(t *testing.T){
 s,_:=binarySpoolLowerFixture(t);s.busy=true;s.finished=true;s.owner.workDeadline=time.Now().Add(-time.Second)
 if s.end(nil)==nil||!s.failed||s.finished||s.busy||!s.retiring{t.Fatal("late final work returned positive raw retention")}
}
