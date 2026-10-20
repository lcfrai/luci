package api

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/product"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
 "github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

// Actual API lease/publication/registration paths, with inert typed Docker
// outcomes. This does not verify an archive, engine, parser or real image.
type documentInstallerFixture struct{*installerFixture;pin string;restores int;unknown bool}
func(f *documentInstallerFixture)PreparePackagedRuntimeImage(ctx context.Context,s backend.RuntimeImageSpec)(backend.PreparedRuntimeImage,error){
 s.Identity.EngineID,s.Identity.InstanceID,s.Identity.SourceDigest=httpCheckEngine,httpCheckInstance,f.pin;s.ImageTag="docker.io/luci/document-extractor:packaged"
 b:=&runtimeoci.DocumentBundle{ManifestSHA256:f.pin};b.Manifest.Image.ID="sha256:"+strings.Repeat("a",64)
 return backend.PreparedRuntimeImage{Spec:s,RequiredDiskBytes:backend.RuntimeImageRequiredDiskBytes,ContextDigest:f.pin,DocumentBundle:b},ctx.Err()
}
func(f *documentInstallerFixture)LoadPackagedRuntimeImage(ctx context.Context,p backend.PreparedRuntimeImage)(backend.RuntimeImageBuildResult,error){f.imports++;if f.unknown{return backend.RuntimeImageBuildResult{},backend.ErrUnavailable};return backend.RuntimeImageBuildResult{Settled:true,ImageID:p.DocumentBundle.Manifest.Image.ID},ctx.Err()}
func(f *documentInstallerFixture)DerivePackagedRuntimeImage(ctx context.Context,p backend.PreparedRuntimeImage)(backend.RuntimeImageBuildResult,error){f.derives++;return backend.RuntimeImageBuildResult{Settled:true,ImageID:p.DocumentBundle.Manifest.Image.ID},ctx.Err()}
func(f *documentInstallerFixture)DocumentImageID(ctx context.Context,p backend.PreparedRuntimeImage,id string)(string,error){if f.imports!=1||f.derives!=1||f.publishes!=1||id!=p.DocumentBundle.Manifest.Image.ID{return "",backend.ErrLaunchFenceOwnership};return id,ctx.Err()}
func(f *documentInstallerFixture)RestoreDocumentExtractor(ctx context.Context,_ string,r backend.DocumentRuntimePublishedReceipt)(backend.DocumentExtractionExecutor,error){
 f.restores++;if r.ManifestSHA256!=f.pin||r.Identity.SourceDigest!=f.pin||r.ImageID!="sha256:"+strings.Repeat("a",64)||r.ImageTag!="docker.io/luci/document-extractor:packaged"{return nil,backend.ErrLaunchFenceOwnership};return &inertDocumentExecutor{},ctx.Err()
}

func documentInstallAPIFixture(t *testing.T)(*API,*documentInstallerFixture,runtimeinstall.Request){
 t.Helper();a,ordinary,req:=installerRuntimeFixture(t);root:=t.TempDir();source:=filepath.Join(root,"document-image")
 if err:=os.Mkdir(source,0700);err!=nil{t.Fatal(err)}
 for _,name:=range []string{"document-runtime-manifest-v1.json","document-image-linux-amd64.tar"}{if err:=os.WriteFile(filepath.Join(source,name),[]byte("inert fixture, not a verified package"),0600);err!=nil{t.Fatal(err)}}
 pin:=strings.Repeat("b",64);p,err:=product.New(product.Options{StateDir:t.TempDir(),RuntimeDir:root,DocumentRuntime:runtimeinstall.DocumentRuntimeConfiguration{ImageManifestSHA256:pin}});if err!=nil{t.Fatal(err)};a.deps.Product=p
 f:=&documentInstallerFixture{installerFixture:ordinary,pin:pin};a.deps.DocumentRuntimeInstaller=f
 req.Mode,req.SourceDirectory,req.ImageTag=runtimeinstall.DocumentRuntimeInstallMode,source,runtimeinstall.DocumentRuntimeImageTag
 return a,f,req
}

func TestDocumentPackagedRuntimeUsesSharedPhasesThenPrivateBindingAndRestore(t *testing.T){
 a,f,req:=documentInstallAPIFixture(t)
 out,err:=a.installRuntimeImage(context.Background(),req,installerPublish)
 if err!=nil||out.Phase!="ready"||out.ResourcesHeld||!out.CleanupConfirmed||f.imports!=1||f.derives!=1||f.publishes!=1||f.builds!=0||f.restores!=1||a.currentDocumentExtractor()==nil{t.Fatal("document import did not reach exact registered extractor",out,err)}
 if _,err=a.execution.runtimeImageLease(context.Background(),out.AllocationID);!errors.Is(err,errRuntimeImageLeaseAbsent){t.Fatal("positive publication did not acknowledge its exact lease",err)}
 a.documentExtractorMu.Lock();a.documentRuntimeExtractor=nil;a.documentExtractorMu.Unlock()
 got,err:=a.resolveDocumentExtractor(context.Background());if err!=nil||got==nil||got.DocumentImageID()!=out.ImageID||f.restores!=2||f.imports!=1||f.publishes!=1{t.Fatal("restart restore mutated Docker or lost the pinned image",err)}
}

func TestDocumentPackagedUnknownLoadRetainsLeaseAndCannotMintReceipt(t *testing.T){
 a,f,req:=documentInstallAPIFixture(t);f.unknown=true
 out,err:=a.installRuntimeImage(context.Background(),req,installerPublish)
 if err==nil||out.Phase!="unknown"||!out.ResourcesHeld||f.imports!=1||f.derives!=0||f.publishes!=0||f.restores!=0||a.currentDocumentExtractor()!=nil{t.Fatal("unknown import acquired extraction authority",out,err)}
 if _,err=os.Stat(filepath.Join(filepath.Dir(a.execution.path),"document-runtime-publication.json"));!errors.Is(err,os.ErrNotExist){t.Fatal("unknown import published a private receipt",err)}
 lease,err:=a.execution.runtimeImageLease(context.Background(),out.AllocationID);if err!=nil||a.restoreDocumentRuntimeLease(context.Background(),lease)==nil||f.restores!=0{t.Fatal("active lease was adopted as completed",err)}
}

func TestDocumentPackagedRejectsForeignSourceBeforePrepareOrDockerEffects(t *testing.T){
 a,f,req:=documentInstallAPIFixture(t);req.SourceDirectory=t.TempDir()
 if _,err:=a.installRuntimeImage(context.Background(),req,installerPublish);err==nil||f.imports!=0||f.publishes!=0||f.restores!=0{t.Fatal("request selected a foreign document package",err)}
}

func TestDocumentReservedRecoveryDoesNotRequireCurrentImageOrPackage(t *testing.T){
 a,_,req:=inertDocumentFixture(t);a.deps.DocumentExtractor=nil;a.deps.DocumentRuntimeInstaller=nil
 lease:=documentCapacityFixture();lease.Scope=a.execution.scope;lease.Identity.ScopeDigest=a.execution.scope;lease.ID=req.AllocationID;lease.Identity.AllocationID=req.AllocationID;lease.Identity.ProjectID=req.ProjectID;lease.Identity.DocumentID=req.DocumentID;lease.Identity.AttemptID=req.AttemptID;lease.Actor=req.Actor;lease.State="reserved"
 if err:=a.execution.mutate(context.Background(),func(l *executionLeaseLedger)error{l.Documents=append(l.Documents,lease);return nil});err!=nil{t.Fatal(err)}
 out,err:=a.recoverDocumentExtraction(context.Background(),product.DiscoveryExtractionRecoveryRequest{ProjectID:req.ProjectID,DocumentID:req.DocumentID,AttemptID:req.AttemptID,AllocationID:req.AllocationID,Actor:req.Actor,Action:"retire",Outcome:product.DiscoveryExtractionOutcome{State:"claimed"}},func(product.DiscoveryExtractionOutcome)error{return nil})
 if err!=nil||out.ResourcesHeld||!out.CleanupConfirmed||!out.ReaderSettled{t.Fatal("never-started reservation depended on current parser readiness",out,err)}
 saved,err:=a.execution.documentLease(context.Background(),req.ProjectID,req.DocumentID,req.AttemptID);if err!=nil||saved.State!="finished"{t.Fatal("exact reserved identity was not retired",err)}
}
