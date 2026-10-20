package api

import (
 "context"
 "encoding/json"
 "errors"
 "path/filepath"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func preparedPackagedRuntimeImageID(p backend.PreparedRuntimeImage)(string,bool){
 if p.PullBundle!=nil{return "",false}
 if p.Bundle!=nil&&p.DocumentBundle==nil{return p.Bundle.Manifest.Image.ID,true}
 if p.Bundle==nil&&p.DocumentBundle!=nil{return p.DocumentBundle.Manifest.Image.ID,true}
 return "",false
}

func(a *API)documentRuntimePin()string{if a==nil||a.deps.Product==nil{return ""};return a.deps.Product.DocumentRuntimeConfiguration().ImageManifestSHA256}

func(a *API)currentDocumentExtractor()backend.DocumentExtractionExecutor{
 a.documentExtractorMu.RLock();current:=a.documentRuntimeExtractor;a.documentExtractorMu.RUnlock()
 if current!=nil{return current};return a.deps.DocumentExtractor
}

func(a *API)documentRetirementExecutor()(backend.DocumentExtractionRetirementExecutor,error){
 if current:=a.currentDocumentExtractor();current!=nil{return current,nil}
 selected,ok:=a.deps.Backend.(*dockerbe.Backend);if !ok{return nil,backend.ErrUnsupported}
 return dockerbe.NewDocumentRetirement(selected)
}

func(a *API)documentRuntimeDirectory()(string,error){
 if a==nil||a.deps.Product==nil||!checkHex(a.documentRuntimePin(),32){return "",backend.ErrUnsupported}
 return a.deps.Product.DocumentRuntimeSource()
}

func(a *API)retainDocumentRuntime(ctx context.Context,p backend.PreparedRuntimeImage,id string)error{
 if a.deps.DocumentRuntimeInstaller==nil||p.DocumentBundle==nil||p.Bundle!=nil||p.PullBundle!=nil||p.DocumentBundle.ManifestSHA256!=a.documentRuntimePin()||p.Spec.ImageTag!=runtimeinstall.DocumentRuntimeImageTag&&p.Spec.ImageTag!="docker.io/luci/document-extractor:packaged"{return backend.ErrLaunchFenceOwnership}
 observed,err:=a.deps.DocumentRuntimeInstaller.DocumentImageID(ctx,p,id)
 if err!=nil||observed!=id{return errors.Join(err,backend.ErrLaunchFenceOwnership)}
 receipt:=backend.DocumentRuntimePublishedReceipt{Identity:p.Spec.Identity,ImageID:id,ImageTag:p.Spec.ImageTag,ManifestSHA256:p.DocumentBundle.ManifestSHA256}
 executor,err:=a.deps.DocumentRuntimeInstaller.RestoreDocumentExtractor(ctx,p.Spec.SourceDirectory,receipt)
 if err!=nil||executor==nil||executor.DocumentImageID()!=id||ctx.Err()!=nil{return errors.Join(err,ctx.Err(),backend.ErrUnavailable)}
 if err=a.saveDocumentRuntimeReceipt(ctx,receipt);err!=nil{return err}
 a.documentExtractorMu.Lock();a.documentRuntimeExtractor=executor;a.documentExtractorMu.Unlock()
 return nil
}

func(a *API)saveDocumentRuntimeReceipt(ctx context.Context,r backend.DocumentRuntimePublishedReceipt)error{
 if ctx.Err()!=nil||r.Identity.ScopeDigest!=a.execution.scope||r.ManifestSHA256!=a.documentRuntimePin()||r.Identity.SourceDigest!=r.ManifestSHA256||!checkHex(r.ManifestSHA256,32)||r.ImageID==""{return backend.ErrLaunchFenceOwnership}
 root,err:=a.execution.prepareExecutionDirectory();if err!=nil{return err}
 data,err:=json.Marshal(r);if err!=nil||len(data)>16<<10{return backend.ErrLaunchFenceOwnership}
 path:=filepath.Join(root,"document-runtime-publication.json")
 writeErr:=writeExecutionLedger(path,data)
 actual,readErr:=codexRead(filepath.Dir(root),path,16<<10)
 if readErr!=nil||string(actual)!=string(data){return errors.Join(writeErr,readErr,backend.ErrUnavailable)}
 return ctx.Err()
}

// Called only by an already authorized extraction/recovery POST, never by a
// display GET. A receipt is a continuity binding, not proof: the pinned package
// and exact selected image/tag must be reverified without Load/Tag/Publish.
func(a *API)resolveDocumentExtractor(ctx context.Context)(backend.DocumentExtractionExecutor,error){
 if current:=a.currentDocumentExtractor();current!=nil{return current,nil}
 if a.deps.DocumentRuntimeInstaller==nil||a.execution==nil{return nil,backend.ErrUnsupported}
 a.documentRestoreMu.Lock();defer a.documentRestoreMu.Unlock()
 if current:=a.currentDocumentExtractor();current!=nil{return current,nil}
 root:=filepath.Dir(a.execution.path)
 raw,err:=codexRead(filepath.Dir(root),filepath.Join(root,"document-runtime-publication.json"),16<<10);if err!=nil{return nil,err}
 var receipt backend.DocumentRuntimePublishedReceipt
 if json.Unmarshal(raw,&receipt)!=nil{return nil,backend.ErrLaunchFenceOwnership}
 canonical,err:=json.Marshal(receipt)
 if err!=nil||string(canonical)!=string(raw)||receipt.Identity.ScopeDigest!=a.execution.scope||receipt.ManifestSHA256!=a.documentRuntimePin()||receipt.Identity.SourceDigest!=receipt.ManifestSHA256{return nil,backend.ErrLaunchFenceOwnership}
 source,err:=a.documentRuntimeDirectory();if err!=nil{return nil,err}
 executor,err:=a.deps.DocumentRuntimeInstaller.RestoreDocumentExtractor(ctx,source,receipt)
 if err!=nil||executor==nil||executor.DocumentImageID()!=receipt.ImageID||ctx.Err()!=nil{return nil,errors.Join(err,ctx.Err(),backend.ErrUnavailable)}
 a.documentExtractorMu.Lock();a.documentRuntimeExtractor=executor;a.documentExtractorMu.Unlock()
 return executor,nil
}

func(a *API)restoreDocumentRuntimeLease(ctx context.Context,v runtimeImageExecutionLease)error{
 if v.State!="finished"||v.DistributionMode!="packaged"||v.ImportStage!="deriving"||v.Image==""||v.DocumentManifestSHA256==""||v.DocumentManifestSHA256!=a.documentRuntimePin()||v.Scope!=a.execution.scope||!validRuntimeImageExecutionLease(v)||a.deps.DocumentRuntimeInstaller==nil{return backend.ErrLaunchFenceOwnership}
 receipt:=backend.DocumentRuntimePublishedReceipt{Identity:v.Identity,ImageID:v.Image,ImageTag:v.ImageTag,ManifestSHA256:v.DocumentManifestSHA256}
 source,err:=a.documentRuntimeDirectory();if err!=nil{return err}
 executor,err:=a.deps.DocumentRuntimeInstaller.RestoreDocumentExtractor(ctx,source,receipt)
 if err!=nil||executor==nil||executor.DocumentImageID()!=receipt.ImageID||ctx.Err()!=nil{return errors.Join(err,ctx.Err(),backend.ErrUnavailable)}
 if err=a.saveDocumentRuntimeReceipt(ctx,receipt);err!=nil{return err}
 a.documentExtractorMu.Lock();a.documentRuntimeExtractor=executor;a.documentExtractorMu.Unlock()
 return nil
}

