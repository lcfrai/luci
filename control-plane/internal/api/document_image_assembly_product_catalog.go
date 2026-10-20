package api

import (
 "context"
 "errors"
 "os"

 "github.com/lcfrai/luci/control-plane/internal/management"
)

// A first configured package needs a source/policy selection, not preexisting
// fabricated fact rows. This may create ONLY the empty metadata head from a
// real held profile input. It never resets/replaces a corrupt or foreign head.
func(p *debianProducerPlan)currentOrInitializeCatalog(ctx context.Context,actor management.ExecutionActorRef)(assemblyInputCatalogSnapshot,error){
 if p==nil||p.owner==nil||p.inputs==nil{return assemblyInputCatalogSnapshot{},errAssemblyCatalogInvalid}
 data,e:=readDebianPlanInput(ctx,p,"catalog-source-descriptor");if e!=nil{return assemblyInputCatalogSnapshot{},e}
 var source assemblyCatalogSource
 if decodeAssemblyCatalog(data,assemblyCatalogRecordLimit,&source)!=nil||!validAssemblyCatalogSource(source)||source.DependencyLockDigest!=assemblyDebianLockSHA||source.InputManifestDigest!=assemblyDebianManifestSHA{return assemblyInputCatalogSnapshot{},errAssemblyCatalogInvalid}
 current,e:=p.owner.readCurrent(ctx)
 if errors.Is(e,os.ErrNotExist){
  if e=p.inputs.CheckCurrent(ctx);e!=nil{return assemblyInputCatalogSnapshot{},e}
  _,e=p.owner.publishMetadata(ctx,actor,assemblyCatalogCAS{},source,assemblyCatalogPolicy{"product-profile-v1",assemblyDebianPolicySourceSHA},assemblyCatalogIndex{Schema:1,Records:[]assemblyCatalogRef{}})
  if e!=nil{return assemblyInputCatalogSnapshot{},e};current,e=p.owner.readCurrent(ctx)
 }
 if e!=nil{return assemblyInputCatalogSnapshot{},e}
 if checkDebianProducerSourceDescriptor(data,p.files["catalog-source-descriptor"],current)!=nil||p.inputs.CheckCurrent(ctx)!=nil{return assemblyInputCatalogSnapshot{},errAssemblyCatalogConflict}
 return current,nil
}
