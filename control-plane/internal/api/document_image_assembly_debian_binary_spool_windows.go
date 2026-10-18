//go:build windows

package api

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "io"
 "time"

 "golang.org/x/sys/windows"
 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
)

type debianBinarySpoolFile struct {
 writer,bridge,joined windows.Handle
 info windows.ByHandleFileInformation
 writerClose,bridgeClose,joinClose bool
 complete bool
 writeAttempt *debianBinarySpoolWriteAttempt
}
type debianBinarySpoolWriteAttempt struct{requested,count uint32;returned bool;nativeErr error}
type debianBinarySpoolReadAttempt struct{ticket debianBinaryRawRead;original windows.Handle;count uint32;returned bool;nativeErr error}
type debianBinarySpoolNative struct {
 files [25]debianBinarySpoolFile
 life *debianProofLifetime
 stderrClosing bool
 stderrAttempt *debianBinarySpoolReadAttempt
}

// Mechanically guards an already-existing private directory. It is NOT the
// absent Product binary purpose/profile/current-generation/lease issuer.
// Non-nil partial owners are returned even when collection fails or returns late.
func newDebianBinarySpoolSet(ctx context.Context,o *debianProofNativeOwner,directory string,d debianBinaryRawDecoder)(*debianBinarySpoolSet,error){
 if ctx==nil||o==nil||o.calls==nil||d.state==nil||!debianProducerPlatformSupported()||ctx.Err()!=nil||o.nativeRetired(){return nil,errDebianBinarySpool}
 s:=&debianBinarySpoolSet{owner:o,decoder:d,native:&debianBinarySpoolNative{},busy:true}
 defer func(){s.mu.Lock();s.busy=false;s.mu.Unlock()}()
 o.mu.Lock();if len(o.binarySpools)!=0{o.mu.Unlock();return nil,errDebianBinarySpool};o.binarySpools=append(o.binarySpools,s);o.mu.Unlock()
 ns,err:=o.registerNamespaceSource(ctx,directory);s.mu.Lock();s.namespace=ns;retired:=s.retiring;s.mu.Unlock()
 if err!=nil||ns==nil||retired{s.requestRetirement();if err==nil{err=errDebianBinarySpool};return s,err}
 pool,err:=o.reserveBinarySpoolPool();s.mu.Lock();s.pool=pool;retired=s.retiring;s.mu.Unlock()
 if err!=nil||retired{s.requestRetirement();if err==nil{err=errDebianBinarySpool};return s,err}
 guard,err:=o.acquireNamespaceSource(ctx,ns);s.mu.Lock();s.namespaceGuard=guard;retired=s.retiring;s.mu.Unlock()
 if err!=nil||guard==nil||retired||!s.current(ctx){s.requestRetirement();if err==nil{err=errDebianBinarySpool};return s,err}
 return s,nil
}

func(s *debianBinarySpoolSet)nativeCurrent(ctx context.Context)error{
 s.mu.Lock();bad:=s.retiring||s.failed||!s.busy;g:=s.namespaceGuard;s.mu.Unlock()
 if bad||!s.current(ctx)||g==nil{return errDebianBinarySpool}
 if err:=g.CheckCurrent(ctx);err!=nil{return err};if !s.current(ctx){return errDebianBinarySpool}
 s.mu.Lock();bad=s.retiring||s.failed;s.mu.Unlock();if bad{return errDebianBinarySpool};return nil
}
func(s *debianBinarySpoolSet)allowed(ctx context.Context)bool{s.mu.Lock();bad:=s.retiring||s.failed;s.mu.Unlock();return !bad&&s.current(ctx)}
// A claimed slot can be settled as unallocated only at this exact wrapper
// boundary when no native API was called. seal() never guesses this for it.
func(s *debianBinarySpoolSet)skipOriginal(t *debianProofOriginalTicket)error{return s.owner.recordOriginalReturn(t,[]uintptr{0},true)}
func(s *debianBinarySpoolSet)createFile(ctx context.Context,slot int)error{
 if err:=s.nativeCurrent(ctx);err!=nil{return err};name,err:=windows.UTF16PtrFromString(s.path(slot));if err!=nil{return err}
 t,err:=s.pool.claim(slot,0);if err!=nil{return err}
 if !s.allowed(ctx){_ = s.skipOriginal(t);return errDebianBinarySpool}
 h,callErr:=s.owner.calls.FileCreate(name,windows.GENERIC_READ|windows.GENERIC_WRITE,windows.FILE_SHARE_READ,nil,windows.CREATE_NEW,windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_NORMAL,0)
 f:=&s.native.files[slot];if validDebianProducedHandle(h){f.writer=h} // partial ACK before error/clock
 ack:=s.owner.recordKernelCallReturn(t,callErr,h)
 if ack!=nil{return ack};if callErr!=nil{return callErr};if !validDebianProducedHandle(h)||!s.allowed(ctx){return errDebianBinarySpool};return nil
}
func(s *debianBinarySpoolSet)writeChunk(ctx context.Context,slot int,length int)(int,error){
 if length<1||length>len(s.writeBacking)||!s.allowed(ctx){return 0,errDebianBinarySpool}
 if err:=s.debitWrite(length);err!=nil{return 0,err}
 attempt:=&debianBinarySpoolWriteAttempt{requested:uint32(length)};s.mu.Lock();s.native.files[slot].writeAttempt=attempt;s.mu.Unlock()
 var n uint32;err:=s.owner.calls.FileWrite(s.native.files[slot].writer,s.writeBacking[:length],&n,nil)
 s.mu.Lock();attempt.count=n;attempt.nativeErr=err;attempt.returned=true;s.mu.Unlock()
 // Full requested length remains charged even on a valid short success.
 if err!=nil{return int(n),err};if n==0||n>uint32(length)||!s.allowed(ctx){return int(n),errDebianBinarySpool};return int(n),nil
}
func(s *debianBinarySpoolSet)finishFile(ctx context.Context,slot,bytes int,digest string)error{
 f:=&s.native.files[slot];if !s.allowed(ctx){return errDebianBinarySpool}
 if err:=s.owner.calls.FileFlush(f.writer);err!=nil{return err};if !s.allowed(ctx){return errDebianBinarySpool}
 if err:=s.owner.calls.FileInfo(f.writer,&f.info);err!=nil{return err}
 if !s.allowed(ctx)||f.info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT)!=0||f.info.NumberOfLinks!=1||uint64(f.info.FileSizeHigh)<<32|uint64(f.info.FileSizeLow)!=uint64(bytes){return errDebianBinarySpool}
 t,err:=s.pool.claim(slot,1);if err!=nil{return err};if !s.allowed(ctx){_ = s.skipOriginal(t);return errDebianBinarySpool}
 h,callErr:=s.owner.calls.FileReOpen(f.writer);if validDebianProducedHandle(h){f.bridge=h}
 ack:=s.owner.recordKernelCallReturn(t,callErr,h);if ack!=nil{return ack};if callErr!=nil{return callErr};if !validDebianProducedHandle(h)||!s.allowed(ctx){return errDebianBinarySpool}
 var info windows.ByHandleFileInformation;if err=s.owner.calls.FileInfo(h,&info);err!=nil{return err};if !equalDebianProducedIdentity(f.info,info)||!s.allowed(ctx){return errDebianBinarySpool}
 f.writerClose=true;if err=s.owner.closeKernelOriginal(f.writer);err!=nil{return err};f.writer=0
 if !s.allowed(ctx){return errDebianBinarySpool};if err=s.storeObject(slot,bytes,digest);err!=nil{return err};f.complete=true;return nil
}

func(s *debianBinarySpoolSet)persistFrame(ctx context.Context,v debianBinaryRawView)error{
 if err:=s.begin(ctx);err!=nil{return err};err:=s.persistCurrentFrame(ctx,v);return s.end(err)
}
func(s *debianBinarySpoolSet)persistCurrentFrame(ctx context.Context,v debianBinaryRawView)error{
 ref,err:=v.descriptor();if err!=nil||v.state!=s.decoder.state{return errDebianBinarySpool};slot:=s.nextFrame
 if slot>=23||ref.Bytes<=0{return errDebianBinarySpool};kind,part:=uint8(1+slot%2),uint8(slot/2);if slot==22{kind,part=3,255};if ref.Kind!=kind||ref.Part!=part{return errDebianBinarySpool}
 if err=s.createFile(ctx,slot);err!=nil{return err};hash:=sha256.New()
 for offset:=0;offset<ref.Bytes;{
  request:=min(len(s.writeBacking),ref.Bytes-offset);n,e:=v.copyPayload(offset,s.writeBacking[:request]);if e!=nil||n!=request{return errDebianBinarySpool}
  written,e:=s.writeChunk(ctx,slot,request);if e!=nil{return e};_,_ = hash.Write(s.writeBacking[:written]);offset+=written
 }
 if hex.EncodeToString(hash.Sum(nil))!=ref.SHA256{return errDebianBinarySpool}
 if err=s.finishFile(ctx,slot,ref.Bytes,ref.SHA256);err!=nil{return err}
 if err=s.decoder.advance(v,time.Now());err!=nil{return err};s.nextFrame++;return nil
}

func(s *debianBinarySpoolSet)bindStderrLife(life *debianProofLifetime)error{
 if s==nil||s.owner==nil||life==nil{return errDebianBinarySpool};o:=s.owner;o.mu.Lock()
 owned:=false;for _,l:=range o.lifetimes{if l==life{owned=true}};o.mu.Unlock()
 // Existing first3 reader goroutines must never share this original pipe.
 if !owned||o.nativeRetired()||!time.Now().Before(o.workLimit())||len(life.handles)<3||!validDebianProducedHandle(life.handles[2])||life.stderr!=nil||life.stdout!=nil||life.readersSettled||life.binarySpool.Load()!=nil{return errDebianBinarySpool}
 c:=o.census;c.mu.Lock();found:=false;for _,t:=range c.native{if t.kind!=debianProofOriginalPipe||t.pending{continue};for _,slot:=range t.slots{if slot.value==uintptr(life.handles[2])&&!slot.closing&&!slot.unknown{found=true}}};c.mu.Unlock();if !found{return errDebianProofCensus}
 s.mu.Lock();defer s.mu.Unlock();if s.busy||s.retiring||s.failed||s.native.life!=nil||!life.binarySpool.CompareAndSwap(nil,s){return errDebianBinarySpool};s.native.life=life;return nil
}
func(s *debianBinarySpoolSet)stderrBorrowed()bool{if s==nil{return false};s.mu.Lock();defer s.mu.Unlock();return s.stderrPending}
func(s *debianBinarySpoolSet)transferRetainedNamespace(ctx context.Context)error{
 if s==nil||ctx==nil{return errDebianBinarySpool};s.mu.Lock()
 if s.busy||s.stderrPending||s.retiring||s.failed||s.namespaceGuard==nil||!s.current(ctx){s.mu.Unlock();return errDebianBinarySpool};s.busy=true;old:=s.namespaceGuard;s.mu.Unlock()
 moved,err:=old.Transfer(ctx)
 s.mu.Lock();if moved!=nil{s.namespaceGuard=moved};retired:=s.retiring;s.mu.Unlock()
 if moved!=nil&&s.namespace!=nil&&s.namespace.source!=nil{c:=s.owner.census;c.mu.Lock();s.namespace.source.namespaceGuard=moved;c.mu.Unlock()}
 if err!=nil||moved==nil||retired||!s.current(ctx){err=errDebianBinarySpool};return s.end(err)
}
// Seal this original read end before owner Close, so a new read cannot race
// a positive EOF/retirement close. Existing first3 channel readers are excluded.
func(s *debianBinarySpoolSet)claimStderrClose()bool{
 s.mu.Lock();defer s.mu.Unlock();if s.stderrPending||s.busy||s.native.stderrClosing||!s.stderrEOF&&!s.retiring&&!s.failed{return false};s.native.stderrClosing=true;return true
}
// Exact actual original read only; never accepts a supplied byte observation.
// ONE-byte requests preserve513 EOF1 semantics;65536 shared calls may refuse
// well before64KiB stderr. This is not the absent concurrent reader scheduler.
func(s *debianBinarySpoolSet)readStderrOnce(ctx context.Context)error{
 if err:=s.begin(ctx);err!=nil{return err};err:=s.readStderrOriginal(ctx);return s.end(err)
}
func(s *debianBinarySpoolSet)readStderrOriginal(ctx context.Context)error{
 life:=s.native.life;if life==nil||life.binarySpool.Load()!=s||len(life.handles)<3{return errDebianBinarySpool}
 s.mu.Lock();if s.stderrEOF||s.native.stderrClosing||s.stderrBytes>=len(s.stderr){s.mu.Unlock();return errDebianBinarySpool};s.stderrPending=true;s.mu.Unlock()
 defer func(){s.mu.Lock();s.stderrPending=false;s.mu.Unlock()}() // only after actual native return
 r,err:=s.decoder.beginRead(debianBinaryRawStderr,1,time.Now());if err!=nil{return err}
 attempt:=&debianBinarySpoolReadAttempt{ticket:r,original:life.handles[2]};s.mu.Lock();s.native.stderrAttempt=attempt;s.mu.Unlock()
 var n uint32;err=s.owner.calls.Read(life.handles[2],s.readBacking[:],&n,nil)
 s.mu.Lock();attempt.count=n;attempt.nativeErr=err;attempt.returned=true;s.mu.Unlock()
 if !s.allowed(ctx){return errDebianBinarySpool} // retain raw pending/full debit; actual backing returned
 if n>1{return errDebianBinarySpool}
 if n==0&&err==windows.ERROR_BROKEN_PIPE{
  if e:=s.decoder.observeRead(r,nil,io.EOF,time.Now());e!=nil{return e};s.mu.Lock();s.stderrEOF=true;s.mu.Unlock();return nil
 }
 if err!=nil||n!=1{_ = s.decoder.observeRead(r,s.readBacking[:min(n,1)],err,time.Now());return errDebianBinarySpool}
 // Copy exact original bytes BEFORE513 discards stderr; commit length only
 // after matching raw observation accepts. No counter-derived content/hash.
 s.stderr[s.stderrBytes]=s.readBacking[0]
 if err=s.decoder.observeRead(r,s.readBacking[:],nil,time.Now());err!=nil{return err};s.stderrBytes++;return nil
}

func(s *debianBinarySpoolSet)persistSmallFixed(ctx context.Context,slot int,data []byte)error{
 if slot!=23&&slot!=24||len(data)>64<<10{return errDebianBinarySpool};if err:=s.createFile(ctx,slot);err!=nil{return err}
 for offset:=0;offset<len(data);{request:=min(len(data)-offset,len(s.writeBacking));copy(s.writeBacking[:request],data[offset:offset+request]);n,err:=s.writeChunk(ctx,slot,request);if err!=nil{return err};offset+=n}
 return s.finishFile(ctx,slot,len(data),binarySpoolSHA(data))
}
func(s *debianBinarySpoolSet)finishRawSpool(ctx context.Context)error{
 if err:=s.begin(ctx);err!=nil{return err};err:=s.finishRawOriginals(ctx);return s.end(err)
}
func(s *debianBinarySpoolSet)finishRawOriginals(ctx context.Context)error{
 s.mu.Lock();ready:=s.stderrEOF&&!s.stderrPending&&s.nextFrame==23;s.mu.Unlock();if !ready{return errDebianBinarySpool}
 if err:=s.persistSmallFixed(ctx,23,s.stderr[:s.stderrBytes]);err!=nil{return err}
 manifest,err:=s.manifestBytes();if err!=nil{return err};if err=s.persistSmallFixed(ctx,24,manifest);err!=nil{return err}
 s.decoder.retire() // no live raw view while existing bounded verifier allocates
 objects,err:=s.evidenceObjects();if err!=nil{return err}
 acquisition,err:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(objects);if err!=nil{return err}
 admission,err:=s.owner.registerEvidenceSource(ctx,acquisition);s.mu.Lock();s.output=admission;retired:=s.retiring;s.mu.Unlock()
 if err!=nil||admission==nil||retired{acquisition.RequestRetirement();return errDebianBinarySpool}
 guard,err:=s.owner.acquireEvidenceSource(ctx,admission);s.mu.Lock();s.outputGuard=guard;retired=s.retiring;s.mu.Unlock()
 if err!=nil||guard==nil||retired{return errDebianBinarySpool}
 if err=guard.CheckCurrent(ctx);err!=nil{return err}
 for i,object:=range objects{
  if err=s.joinAndHash(ctx,i,guard,object);err!=nil{return err}
 }
 if err=guard.CheckCurrent(ctx);err!=nil{return err};if !s.allowed(ctx){return errDebianBinarySpool}
 // Every returned token is retained before post-call classification; no
 // stale token may Close current evidence after the transfer.
 moved,err:=guard.Transfer(ctx);if moved!=nil{s.mu.Lock();s.outputGuard=moved;s.mu.Unlock()}
 if err!=nil||moved==nil||!s.allowed(ctx){return errDebianBinarySpool}
 s.mu.Lock();retired=s.retiring;if !retired{s.finished=true};s.mu.Unlock();if retired{return errDebianBinarySpool};s.pool.seal();return nil
}
func(s *debianBinarySpoolSet)joinAndHash(ctx context.Context,slot int,g backend.DocumentImageAssemblyEvidenceGuard,object backend.DocumentImageAssemblyEvidenceObject)error{
 f:=&s.native.files[slot];if !f.complete||!validDebianProducedHandle(f.bridge)||!s.allowed(ctx){return errDebianBinarySpool}
 var current windows.ByHandleFileInformation;if err:=s.owner.calls.FileInfo(f.bridge,&current);err!=nil{return err};if !equalDebianProducedIdentity(f.info,current){return errDebianBinarySpool}
 name,err:=windows.UTF16PtrFromString(object.Path);if err!=nil{return err};t,err:=s.pool.claim(slot,2);if err!=nil{return err}
 if !s.allowed(ctx){_ = s.skipOriginal(t);return errDebianBinarySpool}
 h,callErr:=s.owner.calls.FileCreate(name,windows.GENERIC_READ,windows.FILE_SHARE_READ,nil,windows.OPEN_EXISTING,windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_NORMAL,0)
 if validDebianProducedHandle(h){f.joined=h};ack:=s.owner.recordKernelCallReturn(t,callErr,h);if ack!=nil{return ack};if callErr!=nil{return callErr};if !validDebianProducedHandle(h)||!s.allowed(ctx){return errDebianBinarySpool}
 var joined windows.ByHandleFileInformation;if err=s.owner.calls.FileInfo(h,&joined);err!=nil{return err};if !equalDebianProducedIdentity(current,joined)||!s.allowed(ctx){return errDebianBinarySpool}
 bytes,err:=g.ReadBounded(ctx,object.Role,max(int64(1),object.Bytes));if err!=nil{return err}
 if int64(len(bytes))!=object.Bytes||binarySpoolSHA(bytes)!=object.Digest||!s.allowed(ctx){return errDebianBinarySpool}
 if err=g.CheckCurrent(ctx);err!=nil{return err}
 f.bridgeClose=true;if err=s.owner.closeKernelOriginal(f.bridge);err!=nil{return err};f.bridge=0
 f.joinClose=true;if err=s.owner.closeKernelOriginal(f.joined);err!=nil{return err};f.joined=0
 return nil
}

// Explicit retirement only. No waiting for busy IO, no automatic repeated
// Close on a failed original, and no native process/lease/completion assertion.
func(s *debianBinarySpoolSet)closeRetiredRaw(ctx context.Context)error{
 if s==nil||ctx==nil||s.owner==nil{return errDebianBinarySpool};s.owner.retire();s.mu.Lock()
 if s.busy||s.stderrPending||s.closing{s.mu.Unlock();return errDebianBinarySpool};s.busy=true;s.closing=true;s.mu.Unlock()
 defer func(){s.mu.Lock();s.busy=false;s.closing=false;s.mu.Unlock()}()
 for i:=range s.native.files{f:=&s.native.files[i];for _,v:=range []struct{handle *windows.Handle;attempted *bool}{{&f.writer,&f.writerClose},{&f.bridge,&f.bridgeClose},{&f.joined,&f.joinClose}}{
  if *v.handle==0{continue};if *v.attempted||ctx.Err()!=nil||!time.Now().Before(s.owner.wholeDeadline){return errDebianBinarySpool};*v.attempted=true
  if err:=s.owner.closeKernelOriginal(*v.handle);err!=nil{return err};*v.handle=0
 }}
 s.mu.Lock();out,ns:=s.outputGuard,s.namespaceGuard;s.mu.Unlock()
 if out!=nil{if ctx.Err()!=nil||!time.Now().Before(s.owner.wholeDeadline){return errDebianBinarySpool};if err:=out.Close();err!=nil{return err};s.mu.Lock();s.outputGuard=nil;s.mu.Unlock()}
 if ns!=nil{if ctx.Err()!=nil||!time.Now().Before(s.owner.wholeDeadline){return errDebianBinarySpool};if err:=ns.Close();err!=nil{return err};s.mu.Lock();s.namespaceGuard=nil;s.mu.Unlock()}
 // A missing/partial token is not proof the acquisition's actual originals
 // stopped or closed. Consult exactly the retained sealed lifetimes.
 var sources [2]*debianProofSourceAdmission;if s.namespace!=nil{sources[0]=s.namespace.source};if s.output!=nil{sources[1]=s.output.source}
 for _,a:=range sources{
  if a==nil{continue};if a.lease==nil{return errDebianBinarySpool};state,err:=a.lease.RetirementState();if err!=nil||!state.CollectorStopped||!state.AllSlotsClosed||state.CloseInFlight{return errDebianBinarySpool}
 }
 if s.pool!=nil&&!s.pool.settled(){return errDebianBinarySpool};if ctx.Err()!=nil||!time.Now().Before(s.owner.wholeDeadline){return errDebianBinarySpool};return nil
}
