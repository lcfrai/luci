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
type projectionInertOpen struct{file *projectionInertFile;cursor int;read,write bool;share uint32}
type projectionInertHandle struct{open *projectionInertOpen;read,write,closed bool}
type projectionInertFiles struct {
 mu sync.Mutex
 next windows.Handle
 paths map[string]*projectionInertFile
 handles map[windows.Handle]*projectionInertHandle
 failClose windows.Handle
 partialAlias,partialCreate bool
}
// Sharing is per open FILE_OBJECT, not the reduced access on a duplicate.
func(f *projectionInertFiles)open(file *projectionInertFile,read,write bool,share uint32)(windows.Handle,error){
 seen:=map[*projectionInertOpen]bool{};for _,h:=range f.handles{v:=h.open;if h.closed||v.file!=file||seen[v]{continue};seen[v]=true;if read&&v.share&windows.FILE_SHARE_READ==0||write&&v.share&windows.FILE_SHARE_WRITE==0||v.read&&share&windows.FILE_SHARE_READ==0||v.write&&share&windows.FILE_SHARE_WRITE==0{return windows.InvalidHandle,windows.ERROR_SHARING_VIOLATION}}
 f.next++;f.handles[f.next]=&projectionInertHandle{open:&projectionInertOpen{file:file,read:read,write:write,share:share},read:read,write:write};return f.next,nil
}
func(f *projectionInertFiles)duplicateRead(h windows.Handle)windows.Handle{f.next++;f.handles[f.next]=&projectionInertHandle{open:f.handles[h].open,read:true};return f.next}
func(f *projectionInertFiles)calls()*debianProofWindowsCalls{return &debianProofWindowsCalls{
 FileCreate:func(name *uint16,access,share uint32,_ *windows.SecurityAttributes,creation,flags uint32,_ windows.Handle)(windows.Handle,error){f.mu.Lock();defer f.mu.Unlock();path:=windows.UTF16PtrToString(name);if share!=windows.FILE_SHARE_READ{return 0,errors.New("fake original share mismatch")};if creation==windows.CREATE_NEW{if f.paths[path]!=nil{return 0,errors.New("fake exclusive create collision")};file:=&projectionInertFile{id:uint32(len(f.paths)+1)};f.paths[path]=file;h,e:=f.open(file,true,true,share);if f.partialCreate{return h,errors.New("fake partial Create error")};return h,e};if access!=windows.GENERIC_READ||flags&windows.FILE_FLAG_OPEN_REPARSE_POINT==0||f.paths[path]==nil{return 0,errors.New("fake original open mismatch")};return f.open(f.paths[path],true,false,share)},
 FileInfo:func(h windows.Handle,info *windows.ByHandleFileInformation)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed{return errors.New("fake original absent")};file:=v.open.file;*info=windows.ByHandleFileInformation{FileAttributes:windows.FILE_ATTRIBUTE_NORMAL,VolumeSerialNumber:7,FileIndexLow:file.id,FileSizeLow:uint32(len(file.data)),CreationTime:windows.Filetime{LowDateTime:1},LastWriteTime:windows.Filetime{LowDateTime:2}};return nil},
 FileWrite:func(h windows.Handle,data []byte,n *uint32,_ *windows.Overlapped)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed||!v.write{return errors.New("fake writable original absent")};v.open.file.data=append(v.open.file.data,data...);*n=uint32(len(data));return nil},
 FileFlush:func(windows.Handle)error{return nil},
 FileReOpen:func(h windows.Handle)(windows.Handle,error){f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed{return 0,errors.New("fake original absent")};read,e:=f.open(v.open.file,true,false,windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE);if f.partialAlias{return read,errors.New("fake ReOpen original ACK plus error")};return read,e},
 Read:func(h windows.Handle,data []byte,n *uint32,_ *windows.Overlapped)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed||!v.read{return errors.New("fake readable original absent")};*n=uint32(copy(data,v.open.file.data[v.open.cursor:]));v.open.cursor+=int(*n);return nil},
 Close:func(h windows.Handle)error{f.mu.Lock();defer f.mu.Unlock();v:=f.handles[h];if v==nil||v.closed{return errors.New("fake stale close")};if h==f.failClose{return errors.New("fake positive close unavailable")};v.closed=true;return nil},
}}
func projectionInertNative()(*debianProofNativeOwner,*projectionInertFiles){f:=&projectionInertFiles{next:100,paths:map[string]*projectionInertFile{},handles:map[windows.Handle]*projectionInertHandle{}};return &debianProofNativeOwner{calls:f.calls(),workDeadline:time.Now().Add(time.Minute)},f}
func TestDebianProjectionCreatedAliasSurvivesWriterCloseUntilGuardedIdentityJoin(t *testing.T){
 n,f:=projectionInertNative();data:=[]byte("actual fake created bytes");path:=`C:\fixed-owner\copy.json`;if e:=n.writeEvidenceBytes(path,data,64<<10);e!=nil{t.Fatal(e)}
 if len(n.produced)!=2||n.produced[0].handle!=0||n.produced[1].handle==0||!n.produced[1].readOnly||!n.produced[1].identityKnown{t.Fatal("created write-to-read ownership gap")};alias:=n.produced[1].handle
 if f.handles[alias].open==f.handles[101].open||f.handles[alias].open.write||f.handles[alias].open.share!=windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE{t.Fatal("fake reopened original still models a reduced duplicate")}
 guard:=&assemblyDebianFakeGuard{valid:true};objects:=[]backend.DocumentImageAssemblyEvidenceObject{{Role:"projection-copy:config",Path:path,Bytes:int64(len(data)),Digest:assemblyCatalogDigest(data)}}
 if e:=n.adoptProduced(context.Background(),guard,objects);e!=nil{t.Fatal(e)};if !f.handles[alias].closed||!n.closeProduced(){t.Fatal("guarded adoption failed positive alias closure")};if guard.CheckCurrent(context.Background())!=nil{t.Fatal("native settlement closed retained evidence guard")}
}
func TestDebianProjectionReducedDuplicateRetainsWriteOpenSharing(t *testing.T){
 _,f:=projectionInertNative();file:=&projectionInertFile{id:1};f.paths[`C:\fixed-owner\old.json`]=file;writer,e:=f.open(file,true,true,windows.FILE_SHARE_READ);if e!=nil{t.Fatal(e)};duplicate:=f.duplicateRead(writer)
 if e=f.calls().Close(writer);e!=nil{t.Fatal(e)};if f.handles[duplicate].write||!f.handles[duplicate].open.write{t.Fatal("fake did not distinguish handle access from FILE_OBJECT WriteAccess")}
 if _,e=f.open(file,true,false,windows.FILE_SHARE_READ);e!=windows.ERROR_SHARING_VIOLATION{t.Fatal("old Duplicate downgrade did not reproduce restrictive open sharing failure")};if e=f.calls().Close(duplicate);e!=nil{t.Fatal(e)};if _,e=f.open(file,true,false,windows.FILE_SHARE_READ);e!=nil{t.Fatal("fully retired old FILE_OBJECT still charged share access")}
}
type projectionInertExpectedGuard struct {assemblyDebianFakeGuard;files *projectionInertFiles;path,sha string}
func(g *projectionInertExpectedGuard)CheckCurrent(ctx context.Context)error{if e:=g.assemblyDebianFakeGuard.CheckCurrent(ctx);e!=nil{return e};g.files.mu.Lock();defer g.files.mu.Unlock();if assemblyCatalogDigest(g.files.paths[g.path].data)!=g.sha{return errors.New("fake560 expected original bytes mismatch")};return nil}
func TestDebianProjectionPermissiveHandoffRejectsCompetingWriterAndChangedBytes(t *testing.T){
 for _,mode:=range []string{"writer","changed-bytes"}{t.Run(mode,func(t *testing.T){n,f:=projectionInertNative();data:=[]byte("abc");path:=`C:\fixed-owner\copy.json`;if e:=n.writeEvidenceBytes(path,data,16);e!=nil{t.Fatal(e)};read:=n.produced[1].handle;objects:=[]backend.DocumentImageAssemblyEvidenceObject{{Role:"projection-copy:config",Path:path,Bytes:3,Digest:assemblyCatalogDigest(data)}}
  guard:=&projectionInertExpectedGuard{assemblyDebianFakeGuard:assemblyDebianFakeGuard{valid:true},files:f,path:path,sha:assemblyCatalogDigest(data)}
  var writer windows.Handle;if mode=="writer"{var e error;writer,e=f.open(f.paths[path],true,true,windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE);if e!=nil{t.Fatal("permissive handoff fake incorrectly excludes all writes",e)}}else{f.paths[path].data=[]byte("xyz")}
  if n.adoptProduced(context.Background(),guard,objects)==nil{t.Fatal("handoff admitted competing writer or changed expected bytes")};if f.handles[read].closed{t.Fatal("refusal discarded retained original identity")};if writer!=0{if e:=f.calls().Close(writer);e!=nil{t.Fatal(e)}};if !n.closeProduced(){t.Fatal("exact refused originals failed cleanup")}
 })}
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
