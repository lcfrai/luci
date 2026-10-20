//go:build windows

package dockerbe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Entirely inert originals. These tests exercise ownership transitions, not
// Windows filesystem/driver cancellation, real profile proof, or native IO.
type domainFake struct{
	mu sync.Mutex
	next syscall.Handle
	data []byte
	infos map[syscall.Handle]syscall.ByHandleFileInformation
	events map[syscall.Handle]bool
	closed map[syscall.Handle]bool
	cancels int
	reads int
}

func newDomainFake(data []byte)(*domainFake,domainNative){
	f:=&domainFake{next:20,data:append([]byte(nil),data...),infos:map[syscall.Handle]syscall.ByHandleFileInformation{},events:map[syscall.Handle]bool{},closed:map[syscall.Handle]bool{}}
	n:=domainNative{
		drive:func(string)(uint32,error){return 3,nil},
		open:func(_ string,directory bool)(syscall.Handle,error){f.mu.Lock();defer f.mu.Unlock();f.next++;h:=f.next;v:=syscall.ByHandleFileInformation{VolumeSerialNumber:7,FileIndexLow:uint32(h),NumberOfLinks:1};if directory{v.FileAttributes=syscall.FILE_ATTRIBUTE_DIRECTORY}else{v.FileSizeLow=uint32(len(f.data))};f.infos[h]=v;return h,nil},
		info:func(h syscall.Handle)(syscall.ByHandleFileInformation,error){f.mu.Lock();defer f.mu.Unlock();if f.closed[h]{return syscall.ByHandleFileInformation{},syscall.Errno(6)};return f.infos[h],nil},
		kind:func(syscall.Handle)(uint32,error){return syscall.FILE_TYPE_DISK,nil},
		event:func()(syscall.Handle,error){f.mu.Lock();defer f.mu.Unlock();f.next++;f.events[f.next]=true;return f.next,nil},
		read:func(h syscall.Handle,dst []byte,n *uint32,ov *syscall.Overlapped)error{f.mu.Lock();defer f.mu.Unlock();if f.closed[h]{return syscall.Errno(6)};f.reads++;off:=uint64(ov.OffsetHigh)<<32|uint64(ov.Offset);if off>=uint64(len(f.data)){*n=0;return nil};*n=uint32(copy(dst,f.data[int(off):]));return nil},
		result:func(syscall.Handle,*syscall.Overlapped)(uint32,error){return 0,domainIOIncomplete},
		cancel:func(syscall.Handle,*syscall.Overlapped)error{f.mu.Lock();defer f.mu.Unlock();f.cancels++;return nil},
		close:func(h syscall.Handle)error{f.mu.Lock();defer f.mu.Unlock();if f.closed[h]{return syscall.Errno(6)};f.closed[h]=true;return nil},
	}
	return f,n
}

func domainWindowsPlan(t *testing.T,data []byte)domainInputPlan{
	t.Helper();digest:=sha256.Sum256(data)
	p,err:=planDocumentDomainInputs(`E:\owned\domain`,[]backend.DomainInput{{Role:"base",RelativePath:"inputs/base.tar",Bytes:int64(len(data)),SHA256:hex.EncodeToString(digest[:])}})
	if err!=nil{t.Fatal(err)};return p
}

func domainOpenFake(t *testing.T,data []byte)(*documentDomainInputSet,*domainFake,context.Context){
	t.Helper();ctx,cancel:=context.WithTimeout(context.Background(),time.Second*5);t.Cleanup(cancel)
	f,n:=newDomainFake(data);s,err:=openDomainInputsWithNative(ctx,domainWindowsPlan(t,data),n)
	if err!=nil{t.Fatal(err)};t.Cleanup(func(){if err:=s.Close();err!=nil{t.Error(err)}})
	return s,f,ctx
}

func TestDocumentDomainInputsReadOnlyExactOriginalCopies(t *testing.T){
	data:=bytes.Repeat([]byte("input"),18000) // two actual bounded blocks
	s,f,ctx:=domainOpenFake(t,data)
	r,err:=s.ReaderAt(ctx,"base");if err!=nil{t.Fatal(err)}
	if _,err=s.ReaderAt(ctx,"base");err==nil{t.Fatal("second reader context silently substituted")}
	got:=make([]byte,len(data)+2);n,err:=r.ReadAt(got,0)
	if n!=len(data)||!errors.Is(err,io.EOF)||!bytes.Equal(got[:n],data){t.Fatalf("read %d: %v",n,err)}
	got[0]^=255
	check:=make([]byte,4);if n,err=r.ReadAt(check,0);err!=nil||n!=4||!bytes.Equal(check,data[:4]){t.Fatal("caller buffer aliased retained original")}
	if err=s.CheckCurrent(ctx);err!=nil{t.Fatal(err)}
	if s.readBytes!=uint64(len(data)*2+4)||s.readCalls!=5||f.reads!=5{t.Fatalf("hash+inspection debits not shared: %d/%d",s.readBytes,s.readCalls)}
}

func TestDocumentDomainInputsValidShortKeepsFullRequestedDebit(t *testing.T){
	s,_,ctx:=domainOpenFake(t,[]byte("original"))
	s.native.read=func(_ syscall.Handle,dst []byte,n *uint32,_ *syscall.Overlapped)error{dst[0]='o';*n=1;return nil}
	r,err:=s.ReaderAt(ctx,"base");if err!=nil{t.Fatal(err)}
	before:=s.readBytes;got:=make([]byte,4)
	n,err:=r.ReadAt(got,0)
	if n!=1||err==nil||s.readBytes!=before+4||s.failure==nil{t.Fatal("short read refunded debit or lost sticky refusal")}
	if _,err=r.ReadAt(got,0);err==nil{t.Fatal("failed original reused")}
}

func TestDocumentDomainInputsPendingCancellationRetainsActualMemory(t *testing.T){
	s,f,ctx:=domainOpenFake(t,[]byte("original"))
	readCtx,cancel:=context.WithCancel(ctx)
	s.native.read=func(_ syscall.Handle,_ []byte,_ *uint32,_ *syscall.Overlapped)error{cancel();return syscall.ERROR_IO_PENDING}
	r,err:=s.ReaderAt(readCtx,"base");if err!=nil{t.Fatal(err)}
	if _,err=r.ReadAt(make([]byte,4),0);err==nil{t.Fatal("cancel request became read success")}
	p:=s.pending
	if p==nil||p.complete||len(p.buffer)!=4||p.cancelDone==nil{t.Fatal("pending original backing or cancel custody missing")}
	<-p.cancelDone
	if f.cancels!=1{t.Fatal("original cancellation was not issued exactly once")}
	if err=s.Close();err==nil||s.closed||s.pending!=p||len(p.buffer)!=4{t.Fatal("unknown IO falsely retired")}
	s.mu.Lock();attempt:=s.closeAttempt;s.mu.Unlock();if attempt!=nil{<-attempt.done}
	for _,h:=range s.held{if f.closed[h.handle]{t.Fatal("original closed beneath live overlapped read")}}
	s.native.result=func(syscall.Handle,*syscall.Overlapped)(uint32,error){return 0,syscall.ERROR_OPERATION_ABORTED}
	if err=s.Close();err!=nil||!s.closed||s.pending!=nil||p.buffer!=nil{t.Fatal("actual aborted completion did not settle retained originals",err)}
}

func TestDocumentDomainInputsCompletedReadRequiresEventCloseACK(t *testing.T){
	s,_,ctx:=domainOpenFake(t,[]byte("original"))
	closeOriginal:=s.native.close;failed:=false
	s.native.close=func(h syscall.Handle)error{if s.pending!=nil&&h==s.pending.event&&!failed{failed=true;return backend.ErrUnavailable};return closeOriginal(h)}
	r,err:=s.ReaderAt(ctx,"base");if err!=nil{t.Fatal(err)}
	if _,err=r.ReadAt(make([]byte,4),0);err==nil{t.Fatal("unacknowledged event close became read success")}
	p:=s.pending
	if p==nil||!p.complete||len(p.buffer)!=4{t.Fatal("event and backing lost after close failure")}
	if err=s.Close();err!=nil||p.buffer!=nil{t.Fatal("known completed read event did not retire",err)}
}

func TestDocumentDomainInputsCancellationBorrowMustActuallyReturn(t *testing.T){
	s,f,ctx:=domainOpenFake(t,[]byte("original"))
	readCtx,cancel:=context.WithCancel(ctx)
	entered,release:=make(chan struct{}),make(chan struct{})
	s.native.cancel=func(syscall.Handle,*syscall.Overlapped)error{close(entered);<-release;return nil}
	s.native.read=func(syscall.Handle,[]byte,*uint32,*syscall.Overlapped)error{cancel();return syscall.ERROR_IO_PENDING}
	s.native.result=func(syscall.Handle,*syscall.Overlapped)(uint32,error){select{case <-entered:return 0,syscall.ERROR_OPERATION_ABORTED;default:return 0,domainIOIncomplete}}
	r,err:=s.ReaderAt(readCtx,"base");if err!=nil{t.Fatal(err)}
	if _,err=r.ReadAt(make([]byte,4),0);err==nil{t.Fatal("cancellation became read success")}
	<-entered;p:=s.pending
	if p==nil||!p.complete||!p.cancelInFlight||len(p.buffer)!=4{t.Fatal("actual cancellation borrow was dropped")}
	if err=s.Close();err==nil{t.Fatal("completion alone retired a still-borrowed original")}
	s.mu.Lock();attempt:=s.closeAttempt;s.mu.Unlock();if attempt!=nil{<-attempt.done}
	for _,h:=range s.held{if f.closed[h.handle]{t.Fatal("live cancellation borrower lost its original")}}
	close(release);<-p.cancelDone
	if err=s.Close();err!=nil{t.Fatal(err)}
}

func TestDocumentDomainInputsLateSubmissionDoesNotBlockIndependentRetirement(t *testing.T){
	s,_,ctx:=domainOpenFake(t,[]byte("original"))
	entered,release,returned:=make(chan struct{}),make(chan struct{}),make(chan struct{})
	s.native.read=func(syscall.Handle,[]byte,*uint32,*syscall.Overlapped)error{close(entered);<-release;return syscall.ERROR_IO_PENDING}
	s.native.result=func(syscall.Handle,*syscall.Overlapped)(uint32,error){return 0,syscall.ERROR_OPERATION_ABORTED}
	r,err:=s.ReaderAt(ctx,"base");if err!=nil{t.Fatal(err)}
	go func(){defer close(returned);if _,err:=r.ReadAt(make([]byte,4),0);err==nil{t.Error("late cancelled submission succeeded")}}()
	<-entered
	if err=s.Close();err==nil{t.Fatal("blocked native submission was falsely retired")}
	close(release);<-returned
	p:=s.pending;if p!=nil&&p.cancelDone!=nil{<-p.cancelDone}
	if err=s.Close();err!=nil{t.Fatal(err)}
}

func TestDocumentDomainInputsPartialAcquisitionReturnsOwnedWorker(t *testing.T){
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
	f,n:=newDomainFake([]byte("original"));openOriginal:=n.open
	entered,release:=make(chan struct{}),make(chan struct{})
	n.open=func(path string,directory bool)(syscall.Handle,error){select{case <-entered:default:close(entered);<-release};return openOriginal(path,directory)}
	go func(){<-entered;cancel()}()
	s,err:=openDomainInputsWithNative(ctx,domainWindowsPlan(t,[]byte("original")),n)
	if s==nil||err==nil{t.Fatal("partial owner not returned on acquisition deadline")}
	if err=s.Close();err==nil||len(f.closed)!=0{t.Fatal("inflight open incorrectly claimed retired")}
	close(release);<-s.acquired
	if len(s.held)!=1{t.Fatal("late valid original handle was not retained")}
	if err=s.Close();err!=nil{t.Fatal(err)}
}

func TestDocumentDomainInputsRejectsDigestLinksAndChangedOriginal(t *testing.T){
	for _,name:=range []string{"digest","hardlink","reparse"}{t.Run(name,func(t *testing.T){
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
		_,n:=newDomainFake([]byte("original"));p:=domainWindowsPlan(t,[]byte("original"));infoOriginal:=n.info
		if name=="digest"{p.inputs[0].SHA256=hex.EncodeToString(make([]byte,32))}else{n.info=func(h syscall.Handle)(syscall.ByHandleFileInformation,error){v,err:=infoOriginal(h);if v.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY==0{if name=="hardlink"{v.NumberOfLinks=2}else{v.FileAttributes|=syscall.FILE_ATTRIBUTE_REPARSE_POINT}};return v,err}}
		s,err:=openDomainInputsWithNative(ctx,p,n);if s==nil||err==nil{t.Fatal("unverified input published")};if err=s.Close();err!=nil{t.Fatal(err)}
	})}
	s,f,ctx:=domainOpenFake(t,[]byte("original"));file:=s.files["base"]
	f.mu.Lock();v:=f.infos[file.handle];v.LastWriteTime.LowDateTime++;f.infos[file.handle]=v;f.mu.Unlock()
	if err:=s.CheckCurrent(ctx);err==nil||s.failure==nil{t.Fatal("changed original remained eligible")}
}

func TestDocumentDomainInputsQuotaRefusalDoesNotIssueAnotherRead(t *testing.T){
	s,f,ctx:=domainOpenFake(t,[]byte("original"));r,err:=s.ReaderAt(ctx,"base");if err!=nil{t.Fatal(err)}
	before:=f.reads;s.readCalls=backend.DocumentDomainInputReadCalls
	if _,err=r.ReadAt(make([]byte,1),0);err==nil||f.reads!=before{t.Fatal("call exhaustion issued native read")}
}
