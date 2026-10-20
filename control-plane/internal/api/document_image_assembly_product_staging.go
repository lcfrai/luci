package api

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "sort"
 "strings"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

type assemblyStageFile struct{destination string;input backend.DomainInput;generated []byte}
type assemblyStageSelection struct{files []assemblyStageFile;inputs []backend.DomainInput;manifest []byte;helper runtimeinstall.DebianInputProofFile;directoryBytes uint64;inputBytes int64}

// Only the current resolved closure nominates build bytes. The small path map
// is independently profile-pinned; it can map package paths, never change a
// selected size/digest or nominate a replacement artifact/source.
func selectDocumentAssemblyStage(ctx context.Context,profile runtimeinstall.BundledDebianInputProofProfile,resolved *assemblyResolvedInputClosure)(assemblyStageSelection,error){
 var selected assemblyStageSelection
 if resolved==nil||resolved.current(ctx)!=nil{return selected,errAssemblyClosureDomain}
 files,e:=validateDebianProductProfile(profile);if e!=nil{return selected,e}
 mapFile:=files["binary-input-paths"];mapPath:=filepath.Join(profile.Root,filepath.FromSlash(mapFile.RelativePath))
 if mapFile.Bytes<1||mapFile.Bytes>64<<10{return selected,errAssemblyClosureDomain}
 raw,e:=codexRead(profile.Root,mapPath,mapFile.Bytes);if e!=nil||int64(len(raw))!=mapFile.Bytes||assemblyCatalogDigest(raw)!=mapFile.SHA256{return selected,errAssemblyClosureDomain}
 var paths debianBinaryInputPaths;if decodeAssemblyCatalog(raw,64<<10,&paths)!=nil||paths.Schema!=1||len(paths.Files)!=182{return selected,errAssemblyClosureDomain}
 packages:=map[string]debianBinaryInputPath{};for _,v:=range paths.Files{if strings.HasPrefix(v.Role,"package."){if _,ok:=packages[v.Role];ok{return selected,errAssemblyClosureDomain};packages[v.Role]=v}}
 manifest:=resolved.manifest;selected.manifest,e=json.Marshal(manifest);if e!=nil||len(selected.manifest)>backend.DocumentImageAssemblyManifestMaxBytes{return selected,errAssemblyClosureDomain}
 generated:=map[string][]byte{manifest.DependencyLock.Path:resolved.closedLock}
 for _,file:=range resolved.verification{if _,ok:=generated[file.input.Path];ok{return selected,errAssemblyClosureDomain};generated[file.input.Path]=file.data}
 expected:=[]backend.DocumentImageAssemblyInput{manifest.BaseLayer,manifest.Extractor,manifest.Verifier,manifest.Requirements,manifest.DependencyLock}
 for _,p:=range manifest.Packages{expected=append(expected,p.DocumentImageAssemblyInput)};expected=append(expected,manifest.Wheels...);expected=append(expected,manifest.Notices...);expected=append(expected,manifest.VerificationInputs...)
 seenDestination,seenSource:=map[string]bool{},map[string]backend.DomainInput{}
 for _,input:=range expected{
  key:=strings.ToLower(input.Path);if seenDestination[key]||!assemblyClosureInputValid(input){return selected,errAssemblyClosureDomain};seenDestination[key]=true
  entry:=assemblyStageFile{destination:input.Path}
  if data,ok:=generated[input.Path];ok{if int64(len(data))!=input.Bytes||assemblyCatalogDigest(data)!=input.Digest{return selected,errAssemblyClosureDomain};entry.generated=append([]byte(nil),data...)}else{
   source:=input.Path
   if strings.HasPrefix(input.Path,"debs/"){
    found:=false;for ordinal,tuple:=range manifest.Packages{if tuple.DocumentImageAssemblyInput==input{candidate,ok:=packages[fmt.Sprintf("package.%03d",ordinal)];if !ok||candidate.Bytes!=input.Bytes||candidate.SHA256!=input.Digest{return selected,errAssemblyClosureDomain};source=candidate.RelativePath;found=true;break}};if !found{return selected,errAssemblyClosureDomain}
   }
   role:=fmt.Sprintf("assembly-input.%03d",len(selected.inputs));entry.input=backend.DomainInput{Role:role,RelativePath:source,Bytes:input.Bytes,SHA256:input.Digest}
   fold:=strings.ToLower(source);if previous,ok:=seenSource[fold];ok{if previous.RelativePath!=source||previous.Bytes!=input.Bytes||previous.SHA256!=input.Digest{return selected,errAssemblyClosureDomain};entry.input=previous}else{seenSource[fold]=entry.input;selected.inputs=append(selected.inputs,entry.input)}
  }
  selected.files=append(selected.files,entry);selected.inputBytes+=input.Bytes
 }
 selected.helper=files["assembly-helper-executable"]
 if selected.helper.Bytes<1||selected.helper.Bytes>backend.DocumentImageAssemblyHelperMaxBytes||selected.helper.SHA256!=manifest.HelperDigest{return selected,errAssemblyClosureDomain}
 helper:=backend.DomainInput{Role:"assembly-helper",RelativePath:selected.helper.RelativePath,Bytes:selected.helper.Bytes,SHA256:selected.helper.SHA256}
 if _,ok:=seenSource[strings.ToLower(helper.RelativePath)];ok{return selected,errAssemblyClosureDomain};selected.inputs=append(selected.inputs,helper)
 if selected.inputBytes<=0||selected.inputBytes>backend.DocumentImageAssemblyInputMaxBytes{return selected,errAssemblyClosureDomain}
 selected.directoryBytes=uint64(selected.inputBytes+selected.helper.Bytes+int64(len(selected.manifest)))
 sort.Slice(selected.inputs,func(i,j int)bool{return selected.inputs[i].Role<selected.inputs[j].Role})
 return selected,nil
}

func writeAssemblyStageFile(ctx context.Context,live *liveDocumentAssemblyStage,root,name string,reader io.ReaderAt,expected int64,digest string,generated []byte)(result error){
 if live==nil{return backend.ErrUnsupported}
 live.mu.Lock();if live.retiring||live.writing||live.writerUnknown{live.mu.Unlock();return backend.ErrUnavailable};live.writing=true;live.mu.Unlock()
 defer func(){live.mu.Lock();live.writing=false;live.mu.Unlock()}()
 if !assemblyCatalogHex(digest)||expected<1||expected>backend.DocumentImageAssemblyInputMaxBytes||filepath.IsAbs(name)||strings.ContainsAny(name,"\\:\x00\r\n")||filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))!=name||name==".."||strings.HasPrefix(name,"../"){return backend.ErrUnsupported}
 full:=filepath.Join(root,filepath.FromSlash(name));parent:=filepath.Dir(full)
 if e:=assemblyStageParents(root,parent);e!=nil{return e}
 file,e:=os.OpenFile(full,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600);if e!=nil{return e}
 live.mu.Lock();live.writer=file;live.mu.Unlock()
 defer func(){closeErr:=file.Close();live.mu.Lock();if closeErr!=nil{live.writerUnknown=true}else{live.writer=nil};live.mu.Unlock();result=errors.Join(result,closeErr)}()
 hash:=sha256.New();buffer:=make([]byte,64<<10)
 for offset:=int64(0);offset<expected;{
  if e=ctx.Err();e!=nil{return e};count:=int(min(int64(len(buffer)),expected-offset))
  if generated!=nil{if int64(len(generated))!=expected{return backend.ErrUnsupported};copy(buffer[:count],generated[offset:offset+int64(count)])}else{if reader==nil{return backend.ErrUnsupported};n,re:=reader.ReadAt(buffer[:count],offset);if n!=count||re!=nil{return errors.Join(backend.ErrUnavailable,re)}}
  n,we:=file.Write(buffer[:count]);if we!=nil||n!=count{return errors.Join(io.ErrShortWrite,we)};_,_=hash.Write(buffer[:count]);offset+=int64(count)
 }
 if hex.EncodeToString(hash.Sum(nil))!=digest{return backend.ErrLaunchFenceOwnership};if e=file.Sync();e!=nil{return e};return ctx.Err()
}

func assemblyStageParents(root,parent string)error{
 relative,e:=filepath.Rel(root,parent);if e!=nil||relative==".."||strings.HasPrefix(relative,".."+string(filepath.Separator)){return backend.ErrUnsupported}
 if e=codexRegularPath(filepath.Dir(root),root);e!=nil{return e};if e=codexPrivateDir(root);e!=nil{return e}
 current:=root
 if relative=="."{return nil}
 for _,part:=range strings.Split(relative,string(filepath.Separator)){
  current=filepath.Join(current,part)
  if e=os.Mkdir(current,0700);e!=nil&&!errors.Is(e,os.ErrExist){return e}
  if e=codexRegularPath(root,current);e!=nil{return e};if e=codexPrivateDir(current);e!=nil{return e}
 }
 return nil
}
