//go:build windows

package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const domainIOIncomplete syscall.Errno = 996
const domainHandleEOF syscall.Errno = 38

var domainKernel = syscall.NewLazyDLL("kernel32.dll")
var domainCreateEvent = domainKernel.NewProc("CreateEventW")
var domainOverlappedResult = domainKernel.NewProc("GetOverlappedResult")
var domainDriveType = domainKernel.NewProc("GetDriveTypeW")
var domainReadFile = domainKernel.NewProc("ReadFile")

func originalDomainResult(h syscall.Handle,ov *syscall.Overlapped)(uint32,error){
	var n uint32
	ok,_,err:=domainOverlappedResult.Call(uintptr(h),uintptr(unsafe.Pointer(ov)),uintptr(unsafe.Pointer(&n)),0)
	runtime.KeepAlive(ov)
	if ok==0{return n,err};return n,nil
}

// Fixed private native calls, replaceable only by inert Windows regression
// sources. No raw handle, buffer, callback, or native issuer escapes the owner.
type domainNative struct {
	drive func(string) (uint32, error)
	open func(string, bool) (syscall.Handle, error)
	info func(syscall.Handle) (syscall.ByHandleFileInformation, error)
	kind func(syscall.Handle) (uint32, error)
	event func() (syscall.Handle, error)
	read func(syscall.Handle, []byte, *uint32, *syscall.Overlapped) error
	result func(syscall.Handle, *syscall.Overlapped) (uint32, error)
	cancel func(syscall.Handle, *syscall.Overlapped) error
	close func(syscall.Handle) error
}

func originalDomainNative() domainNative {
	return domainNative{
		drive:func(volume string)(uint32,error){name,err:=syscall.UTF16PtrFromString(volume+`\`);if err!=nil{return 0,err};kind,_,_:=domainDriveType.Call(uintptr(unsafe.Pointer(name)));runtime.KeepAlive(name);return uint32(kind),nil},
		open:func(path string, directory bool)(syscall.Handle,error){
			name, err := syscall.UTF16PtrFromString(path); if err != nil { return syscall.InvalidHandle, err }
			flags := uint32(syscall.FILE_FLAG_OPEN_REPARSE_POINT | syscall.FILE_FLAG_OVERLAPPED)
			if directory { flags |= syscall.FILE_FLAG_BACKUP_SEMANTICS }
			// Deny existing/new writers and deletion/rename of every original.
			return syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, flags, 0)
		},
		info:func(h syscall.Handle)(syscall.ByHandleFileInformation,error){ var v syscall.ByHandleFileInformation; err := syscall.GetFileInformationByHandle(h,&v); return v,err },
		kind:syscall.GetFileType,
		event:func()(syscall.Handle,error){
			h,_,err := domainCreateEvent.Call(0,1,0,0)
			if h == 0 { return 0,err }; return syscall.Handle(h),nil
		},
		read:func(h syscall.Handle,buffer []byte,n *uint32,ov *syscall.Overlapped)error{
			// Asynchronous ReadFile receives NULL for lpNumberOfBytesRead. No
			// caller-stack count pointer is lent to an outstanding operation.
			ok,_,err:=domainReadFile.Call(uintptr(h),uintptr(unsafe.Pointer(&buffer[0])),uintptr(len(buffer)),0,uintptr(unsafe.Pointer(ov)))
			runtime.KeepAlive(buffer);runtime.KeepAlive(ov)
			if ok==0{return err}
			actual,resultErr:=originalDomainResult(h,ov)
			if resultErr!=nil{return syscall.ERROR_IO_PENDING} // retain unknown count/completion
			*n=actual;return nil
		},
		result:originalDomainResult,
		cancel:syscall.CancelIoEx,
		close:syscall.CloseHandle,
	}
}

type domainOriginal struct {
	handle syscall.Handle
	path string
	directory bool
	info syscall.ByHandleFileInformation
	validated bool
}

type domainPendingRead struct {
	handle syscall.Handle
	event syscall.Handle
	ov syscall.Overlapped
	buffer []byte // heap backing remains reachable through actual completion
	complete bool
	n uint32
	err error
	submitted bool
	cancelIssued bool
	cancelInFlight bool
	cancelDone chan struct{}
}

type domainCloseAttempt struct {done chan struct{};err error}

type documentDomainInputSet struct {
	mu sync.Mutex // short state transitions only; never held for read wait/Close
	gate chan struct{} // one original operation, including acquisition worker
	ctx context.Context
	stopContext func() bool
	retiring atomic.Bool
	plan domainInputPlan
	native domainNative
	held []*domainOriginal
	files map[string]*domainOriginal
	readers map[string]*domainInputReader
	pending *domainPendingRead
	ready bool
	closed bool
	failure error
	readCalls uint64
	readBytes uint64
	acquired chan struct{}
	acquisitionError error
	closeAttempt *domainCloseAttempt
}

type domainInputReader struct {
	owner *documentDomainInputSet
	file *domainOriginal
	ctx context.Context
	stopContext func() bool
}

func openDocumentDomainInputs(ctx context.Context, p domainInputPlan)(backend.DocumentDomainInputs,error){
	// Only ordinary absolute drive paths are eligible for this Windows proof.
	volume := filepath.VolumeName(p.root)
	if len(volume)!=2 || volume[1]!=':' || strings.Contains(p.root[len(volume):],":") || len(utf16.Encode([]rune(p.root)))>32760 { return nil,backend.ErrUnsupported }
	components:=strings.TrimPrefix(p.root[len(volume):],`\`)
	if components!=""&&!domainRelativePath(strings.ReplaceAll(components,`\`,"/")){return nil,backend.ErrUnsupported}
	return openDomainInputsWithNative(ctx,p,originalDomainNative())
}

func openDomainInputsWithNative(ctx context.Context,p domainInputPlan,native domainNative)(*documentDomainInputSet,error){
	s := &documentDomainInputSet{ctx:ctx,plan:p,native:native,gate:make(chan struct{},1),files:map[string]*domainOriginal{},readers:map[string]*domainInputReader{},acquired:make(chan struct{})}
	s.gate <- struct{}{} // acquisition owns the gate before the worker is started
	s.stopContext = context.AfterFunc(ctx,s.requestRetirement)
	// CreateFile/metadata have no Windows overlapped variant. The explicitly
	// owned worker retains their original arguments/results through actual return.
	// A deadline can return the partial set; Close never waits for that worker.
	go func(){
		err := s.acquire()
		s.mu.Lock();s.acquisitionError=err;if err!=nil{s.failLocked(err)}else if !s.retiring.Load(){s.ready=true};s.mu.Unlock()
		<-s.gate
		close(s.acquired)
	}()
	select {
	case <-s.acquired:
		s.mu.Lock();err:=s.acquisitionError;s.mu.Unlock()
		if ctx.Err()!=nil { s.requestRetirement();return s,ctx.Err() }
		return s,err
	case <-ctx.Done():
		s.requestRetirement();return s,ctx.Err()
	}
}

func(s *documentDomainInputSet)Demand()backend.DocumentDomainInputDemand{return s.plan.demand}

func(s *documentDomainInputSet)failLocked(err error){ if s.failure==nil { s.failure=err };s.ready=false }
func(s *documentDomainInputSet)fail(err error)error{s.mu.Lock();s.failLocked(err);s.mu.Unlock();return err}

func(s *documentDomainInputSet)requestRetirement(){
	s.retiring.Store(true)
	// Independent retirement never waits behind a native call. Submission and
	// completion paths recheck the latch after their actual native return.
	if !s.mu.TryLock(){return}
	p:=s.pending
	if p!=nil&&p.submitted&&!p.complete&&!p.cancelIssued{
		p.cancelIssued,p.cancelInFlight=true,true
		p.cancelDone=make(chan struct{})
		// This bounded single worker owns the cancellation borrow through actual
		// return; a CancelIoEx request/ERROR_NOT_FOUND is not completion proof.
		go func(){_=s.native.cancel(p.handle,&p.ov);s.mu.Lock();p.cancelInFlight=false;s.mu.Unlock();close(p.cancelDone)}()
	}
	s.mu.Unlock()
}

func(s *documentDomainInputSet)live(ctx context.Context)error{
	if ctx.Err()!=nil {s.requestRetirement();return ctx.Err()}
	if s.ctx.Err()!=nil{s.requestRetirement();return s.ctx.Err()}
	if s.retiring.Load(){return backend.ErrUnavailable}
	return nil
}

func(s *documentDomainInputSet)acquire()error{
	if err:=s.live(s.ctx);err!=nil{return err}
	// No UNC/mapped remote volume. This issues no network API or subprocess;
	// arbitrary filesystem filter behavior is not claimed as a network proof.
	drive,err:=s.native.drive(filepath.VolumeName(s.plan.root))
	if err!=nil||drive!=3{return backend.ErrUnsupported} // DRIVE_FIXED only
	seen:=map[string]*domainOriginal{}
	identities:=map[[3]uint32]bool{}
	open:=func(path string,directory bool,bytes int64)(*domainOriginal,error){
		if err:=s.live(s.ctx);err!=nil{return nil,err}
		if len(utf16.Encode([]rune(path)))>32760{return nil,backend.ErrUnsupported}
		key:=strings.ToLower(path)
		if prior:=seen[key];prior!=nil {if prior.directory!=directory{return nil,backend.ErrUnsupported};return prior,nil}
		if len(s.held)+1>=backend.DocumentDomainInputHandleLimit{return nil,backend.ErrUnsupported}
		h,err:=s.native.open(path,directory)
		valid:=h!=0&&h!=syscall.InvalidHandle
		var held *domainOriginal
		if valid {held=&domainOriginal{handle:h,path:path,directory:directory};s.held=append(s.held,held)}
		if err!=nil{return held,err};if !valid{return nil,backend.ErrUnavailable}
		if err=s.live(s.ctx);err!=nil{return held,err}
		info,err:=s.native.info(h);if err!=nil{return held,err}
		kind,err:=s.native.kind(h)
		if err!=nil||kind!=syscall.FILE_TYPE_DISK||info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT!=0||(info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY!=0)!=directory||!directory&&(info.NumberOfLinks!=1||uint64(info.FileSizeHigh)<<32|uint64(info.FileSizeLow)!=uint64(bytes)){return held,backend.ErrUnsupported}
		identity:=[3]uint32{info.VolumeSerialNumber,info.FileIndexHigh,info.FileIndexLow}
		if identities[identity]{return held,backend.ErrUnsupported}
		identities[identity]=true
		held.info,held.validated=info,true;seen[key]=held
		return held,s.live(s.ctx)
	}
	for _,input:=range s.plan.inputs{
		full:=filepath.Join(s.plan.root,filepath.FromSlash(input.RelativePath))
		var dirs []string
		for dir:=filepath.Dir(full);;dir=filepath.Dir(dir){dirs=append(dirs,dir);if filepath.Dir(dir)==dir{break}}
		for i:=len(dirs)-1;i>=0;i--{if _,err:=open(dirs[i],true,0);err!=nil{return err}}
		file,err:=open(full,false,input.Bytes);if err!=nil{return err}
		s.files[input.Role]=file
		hash:=sha256.New();var offset int64
		for offset<input.Bytes{
			size:=int64(backend.DocumentDomainInputBlockBytes);if size>input.Bytes-offset{size=input.Bytes-offset}
			data,readErr:=s.readBlock(s.ctx,file,offset,int(size))
			if readErr!=nil{return readErr};if len(data)!=int(size){return backend.ErrUnavailable}
			_,_=hash.Write(data);offset+=size
		}
		if hex.EncodeToString(hash.Sum(nil))!=input.SHA256{return backend.ErrLaunchFenceOwnership}
	}
	return s.checkOriginals(s.ctx)
}

func(s *documentDomainInputSet)checkOriginals(ctx context.Context)error{
	if err:=s.live(ctx);err!=nil{return err}
	for _,h:=range s.held{
		if err:=s.live(ctx);err!=nil{return err}
		current,err:=s.native.info(h.handle)
		if err!=nil||!h.validated||!assemblySourceIdentityEqual(h.info,current){return backend.ErrLaunchFenceOwnership}
	}
	return s.live(ctx)
}

func(s *documentDomainInputSet)enter(ctx context.Context)error{
	if ctx==nil{return backend.ErrUnsupported}
	if _,ok:=ctx.Deadline();!ok{return backend.ErrUnsupported}
	if err:=s.live(ctx);err!=nil{return err}
	select{case s.gate<-struct{}{}:case <-ctx.Done():s.requestRetirement();return ctx.Err();case <-s.ctx.Done():s.requestRetirement();return s.ctx.Err()}
	s.mu.Lock();valid:=s.ready&&!s.closed&&s.failure==nil;s.mu.Unlock()
	if !valid||s.live(ctx)!=nil{<-s.gate;return backend.ErrUnavailable};return nil
}

func(s *documentDomainInputSet)ReaderAt(ctx context.Context,role string)(io.ReaderAt,error){
	if err:=s.enter(ctx);err!=nil{return nil,err};defer func(){<-s.gate}()
	file:=s.files[role];if file==nil{return nil,backend.ErrUnsupported}
	// One captured reader context per declared role bounds retained cancellation
	// hooks. Do not silently substitute an old context for a new caller context.
	if s.readers[role]!=nil{return nil,backend.ErrUnsupported}
	r:=&domainInputReader{owner:s,file:file,ctx:ctx}
	r.stopContext=context.AfterFunc(ctx,s.requestRetirement)
	s.readers[role]=r
	return r,nil
}

func(s *documentDomainInputSet)CheckCurrent(ctx context.Context)error{
	if err:=s.enter(ctx);err!=nil{return err};defer func(){<-s.gate}()
	stop:=context.AfterFunc(ctx,s.requestRetirement);defer stop()
	if err:=s.checkOriginals(ctx);err!=nil{return s.fail(err)};return nil
}

func(r *domainInputReader)ReadAt(dst []byte,offset int64)(int,error){
	s:=r.owner
	if err:=s.enter(r.ctx);err!=nil{return 0,err};defer func(){<-s.gate}()
	size:=int64(uint64(r.file.info.FileSizeHigh)<<32|uint64(r.file.info.FileSizeLow))
	if offset<0||!validDomainReadAtBytes(int64(len(dst))){return 0,backend.ErrUnsupported}
	if len(dst)==0{return 0,nil};if offset>=size{return 0,io.EOF}
	limit:=len(dst);if int64(limit)>size-offset{limit=int(size-offset)}
	n:=0
	for n<limit{
		count:=limit-n;if count>backend.DocumentDomainInputBlockBytes{count=backend.DocumentDomainInputBlockBytes}
		data,err:=s.readBlock(r.ctx,r.file,offset+int64(n),count)
		if err!=nil{return n,s.fail(err)}
		copy(dst[n:],data);n+=len(data)
		if len(data)!=count{return n,s.fail(backend.ErrUnavailable)}
	}
	if n<len(dst){return n,io.EOF};return n,nil
}

// Called only with the gate. Requested debit is permanent even for a valid
// short return, error, cancellation, or uncertain completion; no quota refund.
func(s *documentDomainInputSet)readBlock(ctx context.Context,file *domainOriginal,offset int64,count int)([]byte,error){
	if err:=s.live(ctx);err!=nil{return nil,err}
	if count<1||count>backend.DocumentDomainInputBlockBytes||offset<0{return nil,backend.ErrUnsupported}
	s.mu.Lock()
	if s.pending!=nil||s.readCalls>=backend.DocumentDomainInputReadCalls||s.readBytes>backend.DocumentDomainInputReadBytes-uint64(count){s.failLocked(backend.ErrUnavailable);s.mu.Unlock();return nil,backend.ErrUnavailable}
	s.readCalls++;s.readBytes+=uint64(count);s.mu.Unlock()
	event,err:=s.native.event()
	p:=&domainPendingRead{handle:file.handle,event:event,buffer:make([]byte,count),ov:syscall.Overlapped{Offset:uint32(uint64(offset)),OffsetHigh:uint32(uint64(offset)>>32),HEvent:event}}
	if event==0||event==syscall.InvalidHandle{return nil,backend.ErrUnavailable}
	s.mu.Lock();s.pending=p;s.mu.Unlock()
	if err!=nil {s.mu.Lock();p.complete=true;p.err=err;s.mu.Unlock();return nil,errors.Join(err,s.releaseRead(p))}
	if err=s.live(ctx);err!=nil{s.mu.Lock();p.complete=true;p.err=err;s.mu.Unlock();return nil,errors.Join(err,s.releaseRead(p))}
	// Admit the sole original submission under the latch. Never hold the state
	// mutex across ReadFile: even an overlapped API call can itself return late.
	s.mu.Lock()
	if s.retiring.Load(){p.complete=true;s.mu.Unlock();return nil,errors.Join(backend.ErrUnavailable,s.releaseRead(p))}
	s.mu.Unlock()
	var actual uint32
	err=s.native.read(file.handle,p.buffer,&actual,&p.ov)
	s.mu.Lock()
	p.submitted=true
	if err==nil||err!=syscall.ERROR_IO_PENDING {p.complete=true;p.n=actual;p.err=err}
	s.mu.Unlock()
	if s.retiring.Load(){s.requestRetirement()}
	if err==syscall.ERROR_IO_PENDING{
		for {
			if s.pollRead(p){break}
			if liveErr:=s.live(ctx);liveErr!=nil{
				s.requestRetirement()
				if !s.retirePending(p){return nil,errors.Join(liveErr,backend.ErrUnavailable)}
				break
			}
			timer:=time.NewTimer(2*time.Millisecond)
			select{case <-timer.C:case <-ctx.Done():case <-s.ctx.Done():};timer.Stop()
		}
	}
	if !p.complete{return nil,backend.ErrUnavailable}
	var result []byte
	if p.err==nil&&p.n<=uint32(count)&&s.live(ctx)==nil{result=append([]byte(nil),p.buffer[:p.n]...)}
	readErr:=p.err
	if p.n>uint32(count){readErr=backend.ErrUnavailable}
	if liveErr:=s.live(ctx);liveErr!=nil{readErr=errors.Join(readErr,liveErr)}
	closeErr:=s.releaseRead(p)
	if readErr!=nil||closeErr!=nil{return nil,errors.Join(readErr,closeErr)}
	return result,nil
}

func(s *documentDomainInputSet)pollRead(p *domainPendingRead)bool{
	s.mu.Lock();defer s.mu.Unlock()
	if p.complete{return true}
	n,err:=s.native.result(p.handle,&p.ov)
	if err==nil||err==syscall.ERROR_OPERATION_ABORTED||err==domainHandleEOF{p.complete=true;p.n=n;p.err=err;return true}
	// IO_INCOMPLETE and all unclassified failures retain original memory. A
	// cancellation request or a signalled event alone is never an IO close ACK.
	return false
}

func(s *documentDomainInputSet)retirePending(p *domainPendingRead)bool{
	// Separate finite cleanup grace, no extension/reset of inspection authority.
	until:=time.Now().Add(200*time.Millisecond)
	for attempts:=0;attempts<100;attempts++{
		if s.pollRead(p){return true}
		if !time.Now().Before(until){break}
		time.Sleep(2*time.Millisecond)
	}
	return false
}

func(s *documentDomainInputSet)releaseRead(p *domainPendingRead)error{
	s.mu.Lock();defer s.mu.Unlock()
	if s.pending!=p||!p.complete||p.cancelInFlight{return backend.ErrUnavailable}
	if err:=s.native.close(p.event);err!=nil{return err}
	s.pending=nil
	p.buffer=nil // only after actual completion AND original event-close ACK
	return nil
}

func(s *documentDomainInputSet)Close()error{
	s.requestRetirement()
	if !s.mu.TryLock(){return backend.ErrUnavailable}
	if s.closed{
		attempt:=s.closeAttempt;s.mu.Unlock()
		if attempt==nil{return backend.ErrUnavailable}
		select{case <-attempt.done:return attempt.err;default:return backend.ErrUnavailable}
	}
	if previous:=s.closeAttempt;previous!=nil{
		select{case <-previous.done:s.closeAttempt=nil;default:s.mu.Unlock();return backend.ErrUnavailable}
	}
	// Never wait behind acquisition, metadata or a reader. Native CloseHandle
	// also belongs to an explicitly retained worker, not an interruptibility claim.
	select{case s.gate<-struct{}{}:default:s.mu.Unlock();return backend.ErrUnavailable}
	attempt:=&domainCloseAttempt{done:make(chan struct{})};s.closeAttempt=attempt;s.mu.Unlock()
	go func(){attempt.err=s.closeOriginals();<-s.gate;close(attempt.done)}()
	timer:=time.NewTimer(200*time.Millisecond);defer timer.Stop()
	select{case <-attempt.done:return attempt.err;case <-timer.C:return backend.ErrUnavailable}
}

// Sole retirement worker, holding the gate until every native return. No caller
// may forget this owner because a cleanup request's bounded wait expired.
func(s *documentDomainInputSet)closeOriginals()error{
	s.mu.Lock();p:=s.pending;s.mu.Unlock()
	if p!=nil{
		if !s.retirePending(p){return backend.ErrUnavailable}
		if err:=s.releaseRead(p);err!=nil{return err}
	}
	var first error
	remaining:=make([]*domainOriginal,0,len(s.held))
	for i:=len(s.held)-1;i>=0;i--{
		h:=s.held[i]
		if err:=s.native.close(h.handle);err!=nil{remaining=append(remaining,h);if first==nil{first=err}}
	}
	s.held=remaining
	if len(remaining)!=0{return errors.Join(first,backend.ErrUnavailable)}
	s.mu.Lock();s.closed=true;s.ready=false;s.mu.Unlock()
	if s.stopContext!=nil{s.stopContext()}
	for _,r:=range s.readers{if r.stopContext!=nil{r.stopContext()}}
	return nil
}
