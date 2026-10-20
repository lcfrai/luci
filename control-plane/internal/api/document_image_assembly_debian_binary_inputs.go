package api

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "path/filepath"
 "strings"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// The profile pin selects paths, not acquisition/authentication facts. Native
// original guards and the fixed child independently check every selected byte.
type debianBinaryInputPath struct{Role string `json:"role"`;RelativePath string `json:"relativePath"`;Bytes int64 `json:"bytes"`;SHA256 string `json:"sha256"`}
type debianBinaryInputPaths struct{Schema int `json:"schema"`;Files []debianBinaryInputPath `json:"files"`}
type debianBinaryProducerInputs struct{
 packages backend.DocumentImageAssemblyPackageInputGuard
 indexes,sourceIndexes,small,selectionGuard backend.DocumentImageAssemblyEvidenceGuard
 packageAdmission *debianProofPackageAdmission
 indexAdmission,sourceIndexAdmission,smallAdmission,selectionAdmission *debianProofEvidenceAdmission
 first3 []assemblyCatalogObject
 policy assemblyDebianRootPolicyV1
 keys assemblyDebianKeyObservationsV1
 signatures assemblyDebianReleaseSignaturesV1
 pins assemblyDebianProducerPins
 selection assemblyDebianBinarySelectionV1
 indexSelection assemblyDebianIndexSelectionV1
 indexRecord assemblyDebianSignedIndexBindingsV1
 indexConfig,indexReceipt,indexManifest []byte
 indexManifestRef assemblyCatalogRef
 transport *debianBinaryNativeTransport
 spool *debianBinarySpoolSet
}
func(b *debianBinaryProducerInputs)current(ctx context.Context)error{
 if b==nil{return errAssemblyDebianCapture}
 for _,g:=range []backend.DocumentImageAssemblyEvidenceGuard{b.indexes,b.sourceIndexes,b.small,b.selectionGuard}{if g!=nil{if e:=g.CheckCurrent(ctx);e!=nil{return e}}}
 if b.packages!=nil{if e:=b.packages.CheckCurrent(ctx);e!=nil{return e}}
 if len(b.first3)!=3||checkAssemblyDebianKeys(b.keys,b.policy,b.pins,time.Now().Unix())!=nil||checkAssemblyDebianSignatures(b.signatures,b.policy,b.keys,b.pins,time.Now().Unix())!=nil{return errAssemblyDebianEvidence}
 return ctx.Err()
}
func readDebianPlanInput(ctx context.Context,p *debianProducerPlan,role string)([]byte,error){
 if p==nil||p.inputs==nil{return nil,errAssemblyDebianCapture};f,ok:=p.files[role]
 if !ok||f.Bytes<=0||f.Bytes>1<<20{return nil,errAssemblyDebianCapture}
 return readDebianProductGuard(ctx,p.readBudget,p.inputs,role,assemblyDebianObjectRef{role,f.SHA256,f.Bytes})
}

// Resolve only previously adopted exact child-profile facts. No durable JSON
// is reconstructed into a native completion, run, acquisition or loan token.
func resolveDebianBinaryFirst3(ctx context.Context,p *debianProducerPlan,snap assemblyInputCatalogSnapshot)(*debianBinaryProducerInputs,error){
 raw,e:=readDebianPlanInput(ctx,p,runtimeinstall.DebianSignatureProfileRole);if e!=nil{return nil,e}
 child,e:=runtimeinstall.DecodeDebianInputProofProfile(raw,p.files[runtimeinstall.DebianSignatureProfileRole].SHA256)
 if e!=nil||child.Purpose!=""||child.ID!=p.profile.ID||child.Protocol!=p.profile.Protocol||child.SourceSHA256!=p.profile.SourceSHA256{return nil,errAssemblyDebianEvidence}
 child.Root=p.profile.Root;child.DescriptorRelativePath=runtimeinstall.DebianSignatureProfilePath
 childFiles,e:=validateDebianProductProfile(child);if e!=nil{return nil,e}
 for role,f:=range childFiles{if p.files[role]!=f{return nil,errAssemblyDebianEvidence}}
 selected,e:=debianProductProfilePins(child,childFiles);if e!=nil{return nil,e}
 config,e:=readDebianPlanInput(ctx,p,"producer-config");if e!=nil{return nil,e};policyBytes,e:=readDebianPlanInput(ctx,p,"policy-source");if e!=nil{return nil,e}
 policy,e:=normalizeAssemblyDebianProductPolicy(policyBytes,config,selected,snap.State.Policy.Revision);if e!=nil{return nil,e}
 pins:=assemblyDebianProducerPins{Product:&selected,SourceReceipt:selected.SourceReceipt,Config:selected.Config,ProjectionSHA:selected.Projection.SHA256,GPG:selected.GPG.SHA256,GPGV:selected.GPGV.SHA256}
 for i:=0;i<5;i++{f:=childFiles[fmt.Sprintf("signed-release-%d",i)];pins.SignedObjects=append(pins.SignedObjects,assemblyDebianObjectRef{"signed-release",f.SHA256,f.Bytes})}
 policyRef,e:=assemblyDebianRecordRef("official-root-policy",policy);if e!=nil{return nil,e}
 // Multiple historical rows are legal. Select the most recent locally
 // authenticated key observation, then exact linked signatures; same-epoch
 // ambiguity is refused rather than choosing by catalog array order.
 var keys assemblyDebianKeyObservationsV1;var signatures assemblyDebianReleaseSignaturesV1;var keyRef,sigRef assemblyCatalogRef
 for _,ref:=range snap.Index.Records{if ref.Kind!="key-observations"{continue};data,re:=p.owner.readMetadata(ctx,ref);var v assemblyDebianKeyObservationsV1
  if re==nil&&decodeAssemblyCatalog(data,assemblyCatalogRecordLimit,&v)==nil&&v.RootPolicy==policyRef&&checkAssemblyDebianKeys(v,policy,pins,time.Now().Unix())==nil{if keyRef!=(assemblyCatalogRef{})&&v.ObservedAt==keys.ObservedAt{return nil,errAssemblyDebianEvidence};if keyRef==(assemblyCatalogRef{})||v.ObservedAt>keys.ObservedAt{keys,keyRef=v,ref}}}
 if keyRef==(assemblyCatalogRef{}){return nil,errAssemblyDebianEvidence}
 for _,ref:=range snap.Index.Records{if ref.Kind!="release-signatures"{continue};data,re:=p.owner.readMetadata(ctx,ref);var v assemblyDebianReleaseSignaturesV1
  if re==nil&&decodeAssemblyCatalog(data,assemblyCatalogRecordLimit,&v)==nil&&v.RootPolicy==policyRef&&v.KeyObservations==keyRef&&checkAssemblyDebianSignatures(v,policy,keys,pins,time.Now().Unix())==nil{if sigRef!=(assemblyCatalogRef{}){return nil,errAssemblyDebianEvidence};signatures,sigRef=v,ref}}
 if sigRef==(assemblyCatalogRef{}){return nil,errAssemblyDebianEvidence}
 records:=[]assemblyCatalogObject{}
 for _,ref:=range []assemblyCatalogRef{policyRef,keyRef,sigRef}{found:=false;for _,r:=range snap.Index.Records{found=found||r==ref};if !found{return nil,errAssemblyCatalogConflict};data,re:=p.owner.readMetadata(ctx,ref);if re!=nil{return nil,re};records=append(records,assemblyCatalogObject{ref,data})}
 var productConfig assemblyDebianProductConfigV1
 if decodeAssemblyDebianRaw(config,&productConfig)!=nil{return nil,errAssemblyDebianEvidence}
 indexes,e:=normalizeAssemblyDebianIndexSelectionFields(productConfig.ProjectionConfig,policy);if e!=nil{return nil,e}
 lock,e:=readDebianPlanInput(ctx,p,"dependency-lock");if e!=nil{return nil,e};manifest,e:=readDebianPlanInput(ctx,p,"acquisition-manifest");if e!=nil{return nil,e}
 selection,e:=normalizeAssemblyDebianBinarySelection(lock,manifest,policyBytes);if e!=nil{return nil,e}
 return &debianBinaryProducerInputs{first3:records,policy:policy,keys:keys,signatures:signatures,pins:pins,selection:selection,indexSelection:indexes},nil
}

func debianBinaryDeclaredObjects(raw []byte,root string,b *debianBinaryProducerInputs)([]backend.DocumentImageAssemblyEvidenceObject,[]backend.DocumentImageAssemblyEvidenceObject,error){
 var paths debianBinaryInputPaths
 if b==nil||checkAssemblyDebianBinarySelection(b.selection)!=nil||checkAssemblyDebianIndexSelection(b.indexSelection,b.policy)!=nil||len(raw)>64<<10||decodeAssemblyDebianRaw(raw,&paths)!=nil||paths.Schema!=1||len(paths.Files)!=182||!filepath.IsAbs(root){return nil,nil,errAssemblyDebianEvidence}
 canonical,e:=json.Marshal(paths);if e!=nil||!bytes.Equal(canonical,raw){return nil,nil,errAssemblyDebianEvidence}
 expected:=map[string]assemblyDebianObjectRef{}
 for _,p:=range b.selection.Packages{role:=fmt.Sprintf("package.%03d",p.Ordinal);expected[role]=assemblyDebianObjectRef{role,p.SHA256,p.Bytes}}
 for _,v:=range b.indexSelection.Objects{role:="xz-index:"+v.SHA256;expected[role]=assemblyDebianObjectRef{role,v.SHA256,v.Bytes}}
 packages:=make([]backend.DocumentImageAssemblyEvidenceObject,174);indexes:=make([]backend.DocumentImageAssemblyEvidenceObject,0,8);seen:=map[string]bool{};previous:=""
 for _,v:=range paths.Files{ref,ok:=expected[v.Role];relative:=filepath.FromSlash(v.RelativePath)
  if !ok||v.Role<=previous||v.SHA256!=ref.SHA256||v.Bytes!=ref.Bytes||relative==""||relative=="."||strings.ContainsAny(v.RelativePath,"\\:\x00\r\n")||filepath.IsAbs(relative)||filepath.VolumeName(relative)!=""||filepath.Clean(relative)!=relative||relative==".."||strings.HasPrefix(relative,".."+string(filepath.Separator)){return nil,nil,errAssemblyDebianEvidence}
  for _,part:=range strings.Split(v.RelativePath,"/"){device:=strings.ToUpper(strings.SplitN(part,".",2)[0]);if strings.TrimSpace(part)!=part||strings.HasSuffix(part,".")||device=="CON"||device=="PRN"||device=="AUX"||device=="NUL"||len(device)==4&&(strings.HasPrefix(device,"COM")||strings.HasPrefix(device,"LPT"))&&device[3]>='1'&&device[3]<='9'{return nil,nil,errAssemblyDebianEvidence}}
  full:=filepath.Join(root,relative);key:=strings.ToLower(full);if seen[key]{return nil,nil,errAssemblyDebianEvidence};seen[key]=true;previous=v.Role
  object:=backend.DocumentImageAssemblyEvidenceObject{Role:v.Role,Path:full,Bytes:v.Bytes,Digest:v.SHA256}
  if strings.HasPrefix(v.Role,"package."){for ordinal:=range packages{if v.Role==fmt.Sprintf("package.%03d",ordinal){packages[ordinal]=object;break}}}else{indexes=append(indexes,object)}
 }
 if len(indexes)!=8||len(seen)!=182{return nil,nil,errAssemblyDebianEvidence};return packages,indexes,nil
}

func acquireDebianBinaryInputs(ctx context.Context,o *debianProofNativeOwner,p *debianProducerPlan,b *debianBinaryProducerInputs)error{
 raw,e:=readDebianPlanInput(ctx,p,"binary-input-paths");if e!=nil{return e}
 packages,indexes,e:=debianBinaryDeclaredObjects(raw,p.profile.Root,b);if e!=nil{return e}
 pa,e:=dockerbe.PrepareDocumentImageAssemblyPackageAcquisition(packages);if e!=nil{return e};b.packageAdmission,e=o.registerPackageSource(ctx,pa);if e!=nil{return e}
 binarySHA:=map[string]bool{};for _,binding:=range b.indexSelection.Bindings{if binding.IndexPath=="main/binary-amd64/Packages.xz"{binarySHA[binding.CompressedObject.SHA256]=true}}
 var binaryObjects,sourceObjects []backend.DocumentImageAssemblyEvidenceObject
 for _,object:=range indexes{if binarySHA[object.Digest]{binaryObjects=append(binaryObjects,object)}else{sourceObjects=append(sourceObjects,object)}}
 if len(binaryObjects)!=4||len(sourceObjects)!=4{return errAssemblyDebianEvidence}
 ia,e:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(binaryObjects);if e!=nil{return e};b.indexAdmission,e=o.registerEvidenceSource(ctx,ia);if e!=nil{return e}
 sa,e:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(sourceObjects);if e!=nil{return e};b.sourceIndexAdmission,e=o.registerEvidenceSource(ctx,sa);if e!=nil{return e}
 // Every real original collector charge is sealed before either Acquire.
 b.packages,e=o.acquirePackageSource(ctx,b.packageAdmission);if e!=nil{return e};b.indexes,e=o.acquireEvidenceSource(ctx,b.indexAdmission);if e!=nil{return e};b.sourceIndexes,e=o.acquireEvidenceSource(ctx,b.sourceIndexAdmission);if e!=nil{return e}
 for _,object:=range indexes{g:=b.sourceIndexes;if binarySHA[object.Digest]{g=b.indexes};data,re:=g.ReadBounded(ctx,object.Role,object.Bytes);if re!=nil||int64(len(data))!=object.Bytes||assemblyCatalogDigest(data)!=object.Digest{return errAssemblyDebianEvidence};if re=p.owner.writeImmutableAssemblyObject(ctx,assemblyCatalogRef{1,"debian-index-raw",object.Digest,object.Bytes},data,true);re!=nil{return re}}
 now:=time.Now().Unix();b.indexConfig,e=json.Marshal(b.indexSelection);if e!=nil{return e}
 b.indexReceipt,e=json.Marshal(struct{Schema int `json:"schema"`;RunID string `json:"runId"`;Method string `json:"method"`;Objects []assemblyDebianObjectRef `json:"objects"`}{1,p.identity.RunID,"original-held-file-size-sha256",b.indexSelection.Objects});if e!=nil{return e}
 receipt:=assemblyDebianObjectRef{"index-acquisition-receipt",assemblyCatalogDigest(b.indexReceipt),int64(len(b.indexReceipt))}
 config:=assemblyDebianObjectRef{"index-profile",assemblyCatalogDigest(b.indexConfig),int64(len(b.indexConfig))}
 refs:=[]assemblyCatalogRef{};for _,r:=range b.first3{refs=append(refs,r.ref)}
 snap,e:=p.owner.readCurrent(ctx);if e!=nil{return e}
 b.indexManifest,e=json.Marshal(assemblyDebianIndexRetainedManifestV1{1,p.identity.RunID,snap.CAS,snap.Source,snap.State.Policy,refs,config,receipt,b.indexSelection.Objects});if e!=nil{return e}
 b.indexManifestRef=assemblyCatalogRef{1,"producer-raw-manifest",assemblyCatalogDigest(b.indexManifest),int64(len(b.indexManifest))}
 b.indexRecord=assemblyDebianSignedIndexBindingsV1{Schema:1,RecordType:"luci-debian-signed-index-bindings-v1",RootPolicy:refs[0],KeyObservations:refs[1],ReleaseSignatures:refs[2],DependencyLockSHA256:b.policy.DependencyLockSHA256,AcquisitionManifestSHA256:b.policy.AcquisitionManifestSHA256,Capture:assemblyDebianIndexCaptureV1{assemblyDebianIndexProfile,config,receipt,b.indexManifestRef,now,"original-held-file-size-sha256"},Objects:b.indexSelection.Objects,Bindings:b.indexSelection.Bindings,PayloadDecode:"not-performed",ParagraphCoverage:"not-evaluated",ServerEligibility:"not-evaluated-server-owned"}
 return checkAssemblyDebianIndexBindings(b.indexRecord,b.policy,b.keys,b.signatures,b.indexSelection)
}

func prepareDebianBinarySelection(ctx context.Context,o *debianProofNativeOwner,p *debianProducerPlan,b *debianBinaryProducerInputs)(debianBinaryRawExpected,error){
 var expected debianBinaryRawExpected
 sources:=[]backend.DocumentImageAssemblyEvidenceObject{}
 for _,role:=range []string{"binary-reader","binary-inspector"}{f:=p.files[role];logical:="binary-binding-reader-source";if role=="binary-inspector"{logical="pinned-inspector-source"};sources=append(sources,backend.DocumentImageAssemblyEvidenceObject{Role:logical,Path:p.paths[role],Bytes:f.Bytes,Digest:f.SHA256})}
 a,e:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition(sources);if e!=nil{return expected,e};b.smallAdmission,e=o.registerEvidenceSource(ctx,a);if e!=nil{return expected,e};b.small,e=o.acquireEvidenceSource(ctx,b.smallAdmission);if e!=nil{return expected,e}
 fingerprints,e:=dockerbe.SnapshotDocumentImageAssemblyBinaryOriginals(ctx,b.packages,b.indexes,b.small);if e!=nil{return expected,e}
 indexRef,e:=assemblyDebianRecordRef("signed-index-bindings",b.indexRecord);if e!=nil{return expected,e}
 bindingBytes,e:=json.Marshal(struct{Identity runtimeinstall.DebianInputProofIdentity `json:"identity"`;Policy assemblyCatalogRef `json:"policy"`;Indexes assemblyCatalogRef `json:"indexes"`}{p.identity,b.first3[0].ref,indexRef});if e!=nil{return expected,e}
 digest:=assemblyCatalogDigest(bindingBytes);loanID:="binary-"+digest[:32]
 type logicalBinding struct{Ordinal int `json:"ordinal"`;CandidateID string `json:"candidateId"`}
 type indexBinding struct{Object assemblyDebianObjectRef `json:"object"`;Bindings []logicalBinding `json:"bindings"`}
 indexes:=[]indexBinding{}
 request:=backend.DocumentImageAssemblyBinaryReadRequest{Purpose:backend.DocumentImageAssemblyBinaryReadPurpose,LoanID:loanID,ScopeDigest:digest}
 for ordinal,tuple:=range b.selection.Packages{request.Packages[ordinal]=backend.DocumentImageAssemblyBinaryReadSubject{Role:fmt.Sprintf("package.%03d",ordinal),Bytes:tuple.Bytes,Digest:tuple.SHA256}}
 for _,object:=range b.indexSelection.Objects{
  row:=indexBinding{Object:object};for ordinal,logical:=range b.indexSelection.Bindings{if logical.IndexPath=="main/binary-amd64/Packages.xz"&&logical.CompressedObject==object{row.Bindings=append(row.Bindings,logicalBinding{ordinal,logical.CandidateID})}}
  if len(row.Bindings)>0{if len(indexes)>=4{return expected,errAssemblyDebianEvidence};n:=len(indexes);indexes=append(indexes,row);expected.Indexes[n]=object;request.Indexes[n]=backend.DocumentImageAssemblyBinaryReadSubject{Role:"xz-index:"+object.SHA256,Bytes:object.Bytes,Digest:object.SHA256}}
 }
 if len(indexes)!=4{return expected,errAssemblyDebianEvidence}
 value:=struct{Schema int `json:"schema"`;ABI string `json:"abi"`;OperationID string `json:"operationId"`;LoanID string `json:"loanId"`;SourceBinding string `json:"sourceBindingSha256"`;SignedIndexes assemblyCatalogRef `json:"signedIndexes"`;Selection assemblyDebianBinarySelectionV1 `json:"selection"`;Indexes []indexBinding `json:"indexes"`;Fingerprints [180]backend.DocumentImageAssemblyBinaryFingerprint `json:"fingerprints"`}{1,"luci-debian-binary-entry-v1",p.identity.RunID,loanID,digest,indexRef,b.selection,indexes,fingerprints}
 raw,e:=json.Marshal(value);if e!=nil||len(raw)>256<<10{return expected,errAssemblyDebianEvidence}
 path:=filepath.Join(p.directory,"binary-selection.json");if e=o.writeEvidenceBytes(path,raw,256<<10);e!=nil{return expected,e}
 object:=backend.DocumentImageAssemblyEvidenceObject{Role:"binary-binding-selection",Path:path,Bytes:int64(len(raw)),Digest:assemblyCatalogDigest(raw)}
 sa,e:=dockerbe.PrepareDocumentImageAssemblyEvidenceAcquisition([]backend.DocumentImageAssemblyEvidenceObject{object});if e!=nil{return expected,e};b.selectionAdmission,e=o.registerEvidenceSource(ctx,sa);if e!=nil{return expected,e};b.selectionGuard,e=o.acquireEvidenceSource(ctx,b.selectionAdmission);if e!=nil{return expected,e}
 observed,e:=b.selectionGuard.ReadBounded(ctx,object.Role,object.Bytes);if e!=nil||!bytes.Equal(observed,raw){return expected,errAssemblyDebianEvidence}
 request.SmallSources[0]=backend.DocumentImageAssemblyBinaryReadSubject{Role:object.Role,Bytes:object.Bytes,Digest:object.Digest}
 for i,source:=range sources{request.SmallSources[i+1]=backend.DocumentImageAssemblyBinaryReadSubject{Role:source.Role,Bytes:source.Bytes,Digest:source.Digest}}
 expected.OperationID=p.identity.RunID;expected.LoanID=loanID;expected.SourceBindingSHA256=digest;expected.SelectionSHA256=object.Digest;expected.SignedIndexes=indexRef;expected.Selection=b.selection
 b.transport,e=o.prepareBinaryReadTransportSelection(ctx,copyDebianBinaryTransportSelection(request),b.packageAdmission,b.indexAdmission,b.smallAdmission,b.selectionAdmission)
 if e!=nil{return expected,e}
 for slot:=uint16(0);slot<181;slot++{if e=o.duplicateBinaryOriginalAlias(ctx,b.transport,slot);e!=nil{return expected,e}}
 return expected,nil
}
