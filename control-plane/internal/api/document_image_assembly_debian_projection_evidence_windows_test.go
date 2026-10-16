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

// Inert file-table outcomes only. Actual production methods run against a
// per-owner fake table; no file/DLL/Job/process/PDH/native call is executed.
type projectionInertFile struct {data []byte;id uint32}
type projectionInertHandle struct{file *projectionInertFile;cursor int;write bool;closed bool}
type projectionInertFiles struct {
 mu sync.Mutex
 next windows.Handle
 paths map[string]*projectionInertFile
 handles map[windows.Handle]*projectionInertHandle
 failClose windows.Handle
 partialAlias,partialCreate bool
}
func(f *projectionInertFiles)handle(file *projectionInertFile,write bool)windows.Handle{f.next++;f.handles[f.next]=&projectionInertHandle{file:file,write:write};return f.next}
func(f *projectionInertFiles)calls()*debianProofWindowsCalls{return &debianProofWindowsCalls{
 FileCreate:func(name *uint16,access,share uint32,_ *windows.SecurityAttributes,creation,flags uint32,_ windows.Handle)(windows.Handle,error){f.mu.Lock();defer f.mu.Unlock();path:=windows.UTF16PtrToString(name);if share!=windows.FILE_SHARE_READ{return 0,errors.New("fake original share mismatch")};if creation==windows.CREATE_NEW{if f.paths[path]!=nil{return 0,errors.New("fake exclusive create collision")};file:=&projectionInertFile{id:uint32(len(f.paths)+1)};f.paths[path]=file;h:=f.handle(file,true);if f.partialCreate{return h,errors.New("fake partial Create error")};return h,nil};if access!=windows.GENERIC_READ||flags&windows.FILE_FLAG_OPEN_REPARSE_POINT==0||f.paths[path]==nil{return 0,errors.New("fake original open mismatch")};return f.handle(f.paths[path],false),nil},
 FileInfo:func(h windows.Handle,info *windows.ByHandleFileInformation)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed{return errors.New("fake original absent")};*info=windows.ByHandleFileInformation{FileAttributes:windows.FILE_ATTRIBUTE_NORMAL,VolumeSerialNumber:7,FileIndexLow:v.file.id,FileSizeLow:uint32(len(v.file.data)),CreationTime:windows.Filetime{LowDateTime:1},LastWriteTime:windows.Filetime{LowDateTime:2}};return nil},
 FileWrite:func(h windows.Handle,data []byte,n *uint32,_ *windows.Overlapped)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed||!v.write{return errors.New("fake writable original absent")};v.file.data=append(v.file.data,data...);*n=uint32(len(data));return nil},
 FileFlush:func(windows.Handle)error{return nil},
 FileReadAlias:func(h windows.Handle,alias *windows.Handle)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed{return errors.New("fake original absent")};*alias=f.handle(v.file,false);if f.partialAlias{return errors.New("fake alias ACK plus error")};return nil},
 Read:func(h windows.Handle,data []byte,n *uint32,_ *windows.Overlapped)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed||v.write{return errors.New("fake readable original absent")};*n=uint32(copy(data,v.file.data[v.cursor:]));v.cursor+=int(*n);return nil},
 Close:func(h windows.Handle)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed{return errors.New("fake stale close")};if h==f.failClose{return errors.New("fake positive close unavailable")};v.closed=true;return nil},
}}
func projectionInertNative()(*debianProofNativeOwner,*projectionInertFiles){f:=&projectionInertFiles{next:100,paths:map[string]*projectionInertFile{},handles:map[windows.Handle]*projectionInertHandle{}};return &debianProofNativeOwner{calls:f.calls(),workDeadline:time.Now().Add(time.Minute)},f}
func TestDebianProjectionCreatedAliasSurvivesWriterCloseUntilGuardedIdentityJoin(t *testing.T){
 n,f:=projectionInertNative();data:=[]byte("actual fake created bytes");path:=`C:\fixed-owner\copy.json`;if e:=n.writeEvidenceBytes(path,data,64<<10);e!=nil{t.Fatal(e)}
 if len(n.produced)!=2||n.produced[0].handle!=0||n.produced[1].handle==0||!n.produced[1].readOnly||!n.produced[1].identityKnown{t.Fatal("created write-to-read ownership gap")};alias:=n.produced[1].handle
 guard:=&assemblyDebianFakeGuard{valid:true};objects:=[]backend.DocumentImageAssemblyEvidenceObject{{Role:"projection-copy:config",Path:path,Bytes:int64(len(data)),Digest:assemblyCatalogDigest(data)}}
 if e:=n.adoptProduced(context.Background(),guard,objects);e!=nil{t.Fatal(e)};if !f.handles[alias].closed||!n.closeProduced(){t.Fatal("guarded adoption failed positive alias closure")};if guard.CheckCurrent(context.Background())!=nil{t.Fatal("native settlement closed retained evidence guard")}
}
func TestDebianProjectionGuardedNamespaceSubstitutionCannotReplaceCreatedOriginal(t *testing.T){
 n,f:=projectionInertNative();data:=[]byte("identical bytes");path:=`C:\fixed-owner\copy.json`;if e:=n.writeEvidenceBytes(path,data,64<<10);e!=nil{t.Fatal(e)};alias:=n.produced[1].handle
 // Deliberate impossible-under-real-share fake race: equal content but a new
 // namespace FileID. The actual join still must reject it independently.
 f.paths[path]=&projectionInertFile{id:99,data:append([]byte(nil),data...)};objects:=[]backend.DocumentImageAssemblyEvidenceObject{{Role:"projection-copy:config",Path:path,Bytes:int64(len(data)),Digest:assemblyCatalogDigest(data)}}
 if n.adoptProduced(context.Background(),&assemblyDebianFakeGuard{valid:true},objects)==nil{t.Fatal("content hash substituted namespace identity")};if f.handles[alias].closed{t.Fatal("refused join discarded original alias")};if !n.closeProduced(){t.Fatal("refused join original aliases were not retained")}
}
func TestDebianProjectionPartialOriginalACKAndCloseFailureRemainOwned(t *testing.T){
 for _,mode:=range []string{"create","alias","writer-close"}{t.Run(mode,func(t *testing.T){n,f:=projectionInertNative();switch mode{case "create":f.partialCreate=true;case "alias":f.partialAlias=true;case "writer-close":f.failClose=101};if n.writeEvidenceBytes(`C:\fixed-owner\copy.json`,[]byte("x"),16)==nil{t.Fatal("partial original acknowledgement labelled success")};if len(n.produced)==0||n.produced[0].handle!=101{t.Fatal("nonzero original on error lost")};f.failClose=101;if n.closeProduced(){t.Fatal("failed original Close labelled positive")};if n.produced[0].handle!=101{t.Fatal("failed Close dropped exact original")};f.failClose=0;if !n.closeProduced(){t.Fatal("retained originals did not settle after changed fake outcome")}})}
}
func TestDebianProjectionNativeCompletionKeepsRetainedEvidenceLifetimeDistinct(t *testing.T){
 n,_:=projectionInertNative();done:=make(chan struct{});close(done);n.watchDone=done;n.observer=&debianProofObserver{closed:true};results:=make([]debianProofChildResult,10)
 for i:=range results{n.jobs=append(n.jobs,&debianProofJob{ordinal:i,settled:true});n.lifetimes=append(n.lifetimes,&debianProofLifetime{pid:uint32(i+1),creation:uint64(i+1),processSettled:true,readersSettled:true});results[i]=debianProofChildResult{Ordinal:i,PID:uint32(i+1),CreationTime:uint64(i+1),OriginalHandleOwned:true,ExitConfirmed:true,JobEmpty:true,HandlesClosed:true,CaptureSettled:true}}
 n.produced=[]*debianProofProducedFile{{handle:55,readOnly:true,identityKnown:true}};if !n.positiveCompletion(results){t.Fatal("process completion incorrectly demanded evidence retirement")};n.lifetimes[2].handles=[]windows.Handle{56};if n.positiveCompletion(results){t.Fatal("evidence separation erased original process handle ownership")}
}
