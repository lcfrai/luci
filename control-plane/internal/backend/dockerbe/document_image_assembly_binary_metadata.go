package dockerbe

import (
 "context"
 "sort"

 "github.com/lcfrai/luci/control-plane/internal/backend"
)

// Snapshot the fixed180 non-selection originals BEFORE selection.json exists.
// Existing guard borrows prevent Close/Transfer throughout synchronous queries.
// This consumes one metadata attempt on all three SAME owners; it opens no
// source, reads/seeks no payload and does not reserve the later793 byte loan.
func SnapshotDocumentImageAssemblyBinaryOriginals(ctx context.Context,packages backend.DocumentImageAssemblyPackageInputGuard,indexes,sources backend.DocumentImageAssemblyEvidenceGuard)(out [180]backend.DocumentImageAssemblyBinaryFingerprint,err error){
 p,pok:=packages.(*assemblyPackageInputToken);i,iok:=indexes.(*assemblyEvidenceToken);s,sok:=sources.(*assemblyEvidenceToken)
 if ctx==nil||ctx.Err()!=nil||!pok||!iok||!sok||p==nil||i==nil||s==nil||i.owner==s.owner{return out,backend.ErrUnsupported}
 po,err:=p.enter(ctx,true);if err!=nil{return out,err};defer po.leave()
 io,err:=i.enter(ctx,true);if err!=nil{return out,err};defer io.leave()
 so,err:=s.enter(ctx,true);if err!=nil{return out,err};defer so.leave()
 po.mu.Lock();used:=po.binaryMetadataUsed||len(po.objects)!=174;po.binaryMetadataUsed=true;po.mu.Unlock()
 io.mu.Lock();used=used||io.binaryMetadataUsed||len(io.objects)!=4;io.binaryMetadataUsed=true;io.mu.Unlock()
 so.mu.Lock();used=used||so.binaryMetadataUsed||len(so.objects)!=2;so.binaryMetadataUsed=true;so.mu.Unlock()
 if used{return out,backend.ErrUnavailable}
 var bindings [180]assemblyBinaryOriginalBinding
 for n,object:=range po.objects{bindings[n]=assemblyBinaryOriginalBinding{guard:po.guard,object:object}}
 keys:=make([]string,0,4);for role:=range io.objects{keys=append(keys,role)};sort.Strings(keys)
 for n,role:=range keys{object:=io.objects[role];if object.Role!="xz-index:"+object.Digest{return out,backend.ErrUnsupported};bindings[174+n]=assemblyBinaryOriginalBinding{guard:io.guard,object:object}}
 roles:=[2]string{"binary-binding-reader-source","pinned-inspector-source"}
 for n,role:=range roles{object,ok:=so.objects[role];if !ok{return out,backend.ErrUnsupported};if n==0&&(object.Bytes!=22824||object.Digest!="2f52054fc43e926389180ba038ae40650318b940ef3efce5f767492f918f451b")||n==1&&(object.Bytes!=39189||object.Digest!="06b77ce9d7f2d6d06ec5b532cfc965acccd4619322aaf5e17cfa138e6a90acba"){return out,backend.ErrUnsupported};bindings[178+n]=assemblyBinaryOriginalBinding{guard:so.guard,object:object}}
 for n:=range bindings{
  if ctx.Err()!=nil{return out,ctx.Err()};if err=bindAssemblyBinaryOriginal(&bindings[n]);err!=nil{return out,err}
  identity,e:=assemblyBinaryOriginalIdentity(ctx,bindings[n]);if e!=nil{return out,e}
  slot:=n;if n>=178{slot++};out[n]=backend.DocumentImageAssemblyBinaryFingerprint{Slot:uint16(slot),Identity:identity}
 }
 if err=po.guard.CheckCurrent(ctx);err!=nil{return out,err};if err=io.guard.CheckCurrent(ctx);err!=nil{return out,err};if err=so.guard.CheckCurrent(ctx);err!=nil{return out,err}
 return out,ctx.Err()
}
