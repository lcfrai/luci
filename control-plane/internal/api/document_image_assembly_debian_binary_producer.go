package api

import (
 "context"
 "encoding/json"
 "strings"

 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

const debianBinaryEntryRole="binary-entry"
const debianBinaryEntryBytes=int64(34087)
const debianBinaryEntrySHA="94f196ca6f9f49aad943e03395b03027890baa8a46bffb07b849dab64f595483"

// A semantic support role may contain its distribution-relative path. Backend
// evidence roles deliberately cannot. Map the opaque role, never its path, to
// a collision-resistant bounded ID while retaining the original profile refs.
func debianProfileEvidenceRole(role string)string{
 if strings.HasPrefix(role,"support:"){return "support."+assemblyCatalogDigest([]byte(role))}
 return role
}

// A descriptor's own hash is never a launch gate. Only startup configuration
// owned by Product selects these exact bytes; retained operation identity then
// binds them again before each Resume. Omitted configuration keeps the old
// diagnostic/profile refusal, rather than discovering tools on the host.
func(a *API)configuredDebianProducerProfileBeforeReservation(p runtimeinstall.BundledDebianInputProofProfile)(map[string]runtimeinstall.DebianInputProofFile,error){
 if a==nil||a.deps.Product==nil{return nil,runtimeinstall.ErrInputProofUnavailable}
 configured:=a.deps.Product.DocumentRuntimeConfiguration()
 if !configured.Configured(){return debianProducerProfileBeforeReservation(p)}
 if !configured.Valid()||a.deps.Product.ValidateDocumentRuntimeProfile(context.Background(),p)!=nil{return nil,runtimeinstall.ErrInputProofUnavailable}
 // The binary arm has a distinct closed source set/entry and is dispatched
 // separately; it must never enter the original ten-command signature arm.
 if _,ok:=runtimeinstall.DebianInputProofLimits(p.Purpose);!ok{return nil,runtimeinstall.ErrInputProofUnavailable}
 files,err:=validateDebianProductProfile(p);if err!=nil{return nil,err}
 domain,ok:=files[assemblyToolDomainRole]
 if !ok||p.SHA256==configured.ProfileSHA256&&domain.SHA256!=configured.ToolDomainSHA256||domain.Bytes<=0||domain.Bytes>assemblyToolDomainLimit{return nil,runtimeinstall.ErrInputProofUnavailable}
 return files,nil
}

// The expected object family is selected by TWO independent startup pins, not
// by a producer JSON claim. Existing domain validation checks every declared
// reference against the exact independently pinned profile and its closure.
func(a *API)currentConfiguredDebianToolDomain(ctx context.Context,p *debianProducerPlan)(*assemblyDebianToolDomainV1,error){
 if ctx==nil||ctx.Err()!=nil||p==nil||p.inputs==nil||a==nil||a.deps.Product==nil{return nil,errAssemblyToolDomain}
 configured:=a.deps.Product.DocumentRuntimeConfiguration()
 f,ok:=p.files[assemblyToolDomainRole]
 if !configured.Valid()||!configured.Configured()||a.deps.Product.ValidateDocumentRuntimeProfile(ctx,p.profile)!=nil||!ok||p.profile.SHA256==configured.ProfileSHA256&&f.SHA256!=configured.ToolDomainSHA256||f.Bytes<=0||f.Bytes>assemblyToolDomainLimit{return nil,errAssemblyToolDomain}
 data,err:=readDebianProductGuard(ctx,p.readBudget,p.inputs,debianProfileEvidenceRole(assemblyToolDomainRole),assemblyDebianObjectRef{assemblyToolDomainRole,f.SHA256,f.Bytes});if err!=nil{return nil,err}
 if assemblyToolJSONUnique(data)!=nil{return nil,errAssemblyToolDomain}
 var value assemblyToolDomainRecordV1
 if json.Unmarshal(data,&value)!=nil||len(value.Packages)!=2{return nil,errAssemblyToolDomain}
 abi,err:=json.Marshal(value.SystemABI);if err!=nil{return nil,err}
 expected:=applicationPinnedToolDomainV1{ProducerFamily:assemblyToolProductProducerFamily,ProfileID:p.profile.ID,ProfileSHA256:p.profile.SHA256,ProducerSourceSHA256:p.profile.SourceSHA256,CatalogSourceSHA256:p.files["catalog-source-descriptor"].SHA256,Manifest:assemblyToolFileRef{assemblyToolDomainRole,f.Bytes,f.SHA256},SystemABISHA256:assemblyCatalogDigest(abi),LicenceInventory:value.LicenceInventory,SourceInventory:value.SourceInventory,InspectionReceipt:value.SystemABI.InspectionReceipt}
 for i:=range value.Packages{expected.AuthenticationReceipts[i]=value.Packages[i].AuthenticationReceipt}
 domain,err:=newAssemblyDebianToolDomainV1(data,p.profile,expected);if err!=nil{return nil,err}
 if err=p.inputs.CheckCurrent(ctx);err!=nil{return nil,err}
 if a.deps.Product.DocumentRuntimeConfiguration()!=configured{return nil,errAssemblyToolDomain}
 return domain,nil
}
