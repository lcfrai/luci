package dockerbe

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

// DocumentRuntimeInstaller is an exact selected-engine consumer of a separately
// pinned document Docker-save distribution. It does not select an engine,
// build an image, obtain credentials or adopt a sandbox harness package.
type DocumentRuntimeInstaller struct {
	selected *Backend
	manifestSHA256 string
	attach DocumentAttachConfiguration
	mu sync.Mutex
	pinnedImageID string // established only by the held, separately pinned package
	loaded map[backend.RuntimeImageIdentity]string // exact identity -> positively settled immutable image
	published map[backend.RuntimeImageIdentity]string // exact identity -> positively acknowledged image/tag
}

var _ backend.DocumentRuntimeInstaller = (*DocumentRuntimeInstaller)(nil)

func NewDocumentRuntimeInstaller(selected *Backend, pinnedManifestSHA256 string, attach DocumentAttachConfiguration) (*DocumentRuntimeInstaller,error) {
	if selected == nil || !fenceHex(pinnedManifestSHA256,32) { return nil,backend.ErrUnsupported }
	// Reuse the pure selected-client attach-header validation; it has no SDK or
	// image effects and copies the caller's exact trusted profile.
	validated,err := NewDocumentExtractor(selected,"sha256:"+pinnedManifestSHA256,attach)
	if err != nil { return nil,err }
	return &DocumentRuntimeInstaller{selected:selected,manifestSHA256:pinnedManifestSHA256,attach:DocumentAttachConfiguration{Headers:validated.attachHeaders},loaded:map[backend.RuntimeImageIdentity]string{},published:map[backend.RuntimeImageIdentity]string{}},nil
}

func (d *DocumentRuntimeInstaller) PrepareRuntimeImage(ctx context.Context,spec backend.RuntimeImageSpec)(backend.PreparedRuntimeImage,error){return d.PreparePackagedRuntimeImage(ctx,spec)}
func (d *DocumentRuntimeInstaller) BuildRuntimeImage(context.Context,backend.PreparedRuntimeImage)(backend.RuntimeImageBuildResult,error){return backend.RuntimeImageBuildResult{Settled:true},backend.ErrUnsupported}

func (d *DocumentRuntimeInstaller) PreparePackagedRuntimeImage(ctx context.Context,spec backend.RuntimeImageSpec)(backend.PreparedRuntimeImage,error){
	if !validRuntimeIdentity(spec.Identity,false) || spec.Resources.MemoryMiB <= 0 || spec.Resources.CPUMillis <= 0 { return backend.PreparedRuntimeImage{},backend.ErrUnsupported }
	tag,err := runtimeImageTag(spec.ImageTag);if err != nil{return backend.PreparedRuntimeImage{},err};spec.ImageTag=tag
	bundle,raw,err:=runtimeoci.OpenDocument(spec.SourceDirectory,d.manifestSHA256)
	if err != nil{return backend.PreparedRuntimeImage{},err}
	closeBundle:=true;defer func(){if closeBundle{_ =bundle.Close()}}()
	probe,cancel:=context.WithTimeout(ctx,10*time.Second);defer cancel()
	fence,err:=d.selected.LaunchFenceIdentity(probe);if err!=nil{return backend.PreparedRuntimeImage{},err}
	if spec.Identity.EngineID!="" && spec.Identity.EngineID!=fence.EngineID || spec.Identity.InstanceID!="" && spec.Identity.InstanceID!=fence.InstanceID{return backend.PreparedRuntimeImage{},backend.ErrLaunchFenceOwnership}
	spec.Identity.EngineID,spec.Identity.InstanceID,spec.Identity.SourceDigest=fence.EngineID,fence.InstanceID,d.manifestSHA256
	limits,ok:=d.selected.api.(resourceLimitClient);if !ok{return backend.PreparedRuntimeImage{},backend.ErrUnsupported}
	info,err:=limits.Info(probe);if err!=nil||probe.Err()!=nil{return backend.PreparedRuntimeImage{},backend.ErrUnavailable}
	if info.ID!=fence.EngineID||info.OSType!="linux"{return backend.PreparedRuntimeImage{},backend.ErrLaunchFenceOwnership}
	if !requestedLimitsSupported(info,spec.Resources){return backend.PreparedRuntimeImage{},backend.ErrResourceLimitsUnsupported}
	if info.Driver!="vfs"&&info.Driver!="overlay2"&&info.Driver!="overlayfs"{return backend.PreparedRuntimeImage{},backend.ErrUnsupported}
	if _,ok=d.selected.api.(runtimeImageLoadClient);!ok{return backend.PreparedRuntimeImage{},backend.ErrUnsupported}
	if _,ok=d.selected.api.(runtimeImageClient);!ok{return backend.PreparedRuntimeImage{},backend.ErrUnsupported}
	admission,err:=packagedLoadAdmissionMiB(bundle.Manifest,spec.Resources.MemoryMiB);if err!=nil{return backend.PreparedRuntimeImage{},err}
	d.mu.Lock()
	if d.pinnedImageID!=""&&d.pinnedImageID!=bundle.Manifest.Image.ID{d.mu.Unlock();return backend.PreparedRuntimeImage{},backend.ErrLaunchFenceOwnership}
	d.pinnedImageID=bundle.Manifest.Image.ID
	d.mu.Unlock()
	closeBundle=false
	return backend.PreparedRuntimeImage{Spec:spec,PreparedContext:raw,ContextDigest:d.manifestSHA256,RequiredDiskBytes:backend.RuntimeImageRequiredDiskBytes,AdmissionMemoryMiB:admission,StorageDriver:info.Driver,Bundle:bundle},nil
}

func(d *DocumentRuntimeInstaller) validPrepared(p backend.PreparedRuntimeImage) bool {
	return p.Bundle!=nil && p.PullBundle==nil && p.Bundle.Manifest.Kind=="luci-document-prebuilt-docker-save" && p.Bundle.ManifestSHA256==d.manifestSHA256 && p.ContextDigest==d.manifestSHA256 && p.Spec.Identity.SourceDigest==d.manifestSHA256 && validRuntimeIdentity(p.Spec.Identity,true) && runtimeImageID(p.Bundle.Manifest.Image.ID) && runtimeoci.VerifyDocumentArchive(p.Bundle,p.PreparedContext)==nil
}

func(d *DocumentRuntimeInstaller) inspectDocument(ctx context.Context,p backend.PreparedRuntimeImage) error {
	if err:=d.selected.runtimeImageIdentity(ctx,p.Spec.Identity);err!=nil{return err}
	api,ok:=d.selected.api.(runtimeImageLoadClient);if !ok{return backend.ErrUnsupported}
	row,err:=api.ImageInspect(ctx,p.Bundle.Manifest.Image.ID)
	if err!=nil {if errdefs.IsNotFound(err){return err};return backend.ErrUnavailable}
	if row.ID!=p.Bundle.Manifest.Image.ID||!runtimeBaseValid(row)||row.Architecture!="amd64"||len(row.Config.ExposedPorts)!=0||len(row.Config.Volumes)!=0{return backend.ErrLaunchFenceOwnership}
	return d.selected.runtimeImageIdentity(ctx,p.Spec.Identity)
}

func(d *DocumentRuntimeInstaller) LoadPackagedRuntimeImage(ctx context.Context,p backend.PreparedRuntimeImage)(backend.RuntimeImageBuildResult,error){
	if !d.validPrepared(p){return backend.RuntimeImageBuildResult{Settled:true},backend.ErrLaunchFenceOwnership}
	if err:=d.inspectDocument(ctx,p);err==nil {
		d.mu.Lock();d.loaded[p.Spec.Identity]=p.Bundle.Manifest.Image.ID;d.mu.Unlock()
		return backend.RuntimeImageBuildResult{ImageID:p.Bundle.Manifest.Image.ID,Settled:true},nil
	}else if !errdefs.IsNotFound(err){return backend.RuntimeImageBuildResult{Settled:true},err}
	if err:=d.selected.runtimeImageIdentity(ctx,p.Spec.Identity);err!=nil{return backend.RuntimeImageBuildResult{Settled:true},err}
	api:=d.selected.api.(runtimeImageLoadClient)
	response,err:=api.ImageLoad(ctx,io.LimitReader(p.Bundle.Archive,p.Bundle.Manifest.Archive.Bytes))
	if err!=nil||response.Body==nil {if response.Body!=nil{_ =response.Body.Close()};return backend.RuntimeImageBuildResult{},backend.ErrUnavailable}
	reader:=&io.LimitedReader{R:response.Body,N:runtimeImageStreamLimit+1};decoder:=json.NewDecoder(reader)
	failed,complete:="",false
	for {var msg runtimeStreamMessage;e:=decoder.Decode(&msg);if e==io.EOF{complete=true;break};if e!=nil{break};if detail:=runtimeMessageError(msg);detail!=""{failed=detail}}
	closeErr:=response.Body.Close()
	if !complete||reader.N==0||closeErr!=nil||ctx.Err()!=nil{return backend.RuntimeImageBuildResult{},backend.ErrUnavailable}
	if failed!=""{return backend.RuntimeImageBuildResult{Settled:true},backend.ErrUnavailable}
	probe,cancel:=context.WithTimeout(context.WithoutCancel(ctx),10*time.Second);defer cancel()
	if err:=d.inspectDocument(probe,p);err!=nil{return backend.RuntimeImageBuildResult{},err}
	d.mu.Lock();d.loaded[p.Spec.Identity]=p.Bundle.Manifest.Image.ID;d.mu.Unlock()
	return backend.RuntimeImageBuildResult{ImageID:p.Bundle.Manifest.Image.ID,Settled:true},nil
}

// The prebuilt image is already immutable and usable. This phase deliberately
// has no Docker mutation; it requires a prior positive load/readback in this
// owner lifetime. A lost load response is never converted into settlement.
func(d *DocumentRuntimeInstaller) DerivePackagedRuntimeImage(ctx context.Context,p backend.PreparedRuntimeImage)(backend.RuntimeImageBuildResult,error){
	if !d.validPrepared(p){return backend.RuntimeImageBuildResult{Settled:true},backend.ErrLaunchFenceOwnership}
	d.mu.Lock();loaded:=d.loaded[p.Spec.Identity];d.mu.Unlock()
	if loaded!=p.Bundle.Manifest.Image.ID{return backend.RuntimeImageBuildResult{},backend.ErrUnavailable}
	if err:=d.inspectDocument(ctx,p);err!=nil{return backend.RuntimeImageBuildResult{},err}
	return backend.RuntimeImageBuildResult{ImageID:loaded,Settled:true},nil
}

func(d *DocumentRuntimeInstaller) PublishRuntimeImage(ctx context.Context,p backend.PreparedRuntimeImage,imageID string)(backend.RuntimeImagePublishResult,error){
	if !d.validPrepared(p)||imageID!=p.Bundle.Manifest.Image.ID{return backend.RuntimeImagePublishResult{},backend.ErrLaunchFenceOwnership}
	d.mu.Lock();loaded:=d.loaded[p.Spec.Identity];d.mu.Unlock()
	if loaded!=imageID{return backend.RuntimeImagePublishResult{},backend.ErrUnavailable}
	if err:=d.inspectDocument(ctx,p);err!=nil{return backend.RuntimeImagePublishResult{},err}
	api:=d.selected.api.(runtimeImageClient)
	tag,err:=runtimeImageTag(p.Spec.ImageTag);if err!=nil||tag!=p.Spec.ImageTag{return backend.RuntimeImagePublishResult{},backend.ErrLaunchFenceOwnership}
	current,err:=api.ImageInspect(ctx,tag)
	if err==nil&&current.ID!=imageID{return backend.RuntimeImagePublishResult{},backend.ErrLaunchFenceOwnership}
	if err!=nil&&!errdefs.IsNotFound(err){return backend.RuntimeImagePublishResult{},backend.ErrUnavailable}
	if errdefs.IsNotFound(err){if err=d.selected.runtimeImageIdentity(ctx,p.Spec.Identity);err!=nil{return backend.RuntimeImagePublishResult{},err};if err=api.ImageTag(ctx,imageID,tag);err!=nil{return backend.RuntimeImagePublishResult{},backend.ErrUnavailable}}
	current,err=api.ImageInspect(ctx,tag)
	if err!=nil||current.ID!=imageID||ctx.Err()!=nil{return backend.RuntimeImagePublishResult{},backend.ErrUnavailable}
	if err=d.inspectDocument(ctx,p);err!=nil{return backend.RuntimeImagePublishResult{},err}
	d.mu.Lock();d.published[p.Spec.Identity]=imageID+"\x00"+tag;d.mu.Unlock()
	return backend.RuntimeImagePublishResult{ImageID:imageID,ImageTag:tag,Acknowledged:true},nil
}

func(d *DocumentRuntimeInstaller) ObserveRuntimeImage(ctx context.Context,id backend.RuntimeImageIdentity,imageID,tag string)(backend.RuntimeImageObservation,error){
	d.mu.Lock();pinned:=d.pinnedImageID;d.mu.Unlock()
	if !validRuntimeIdentity(id,true)||id.SourceDigest!=d.manifestSHA256||pinned==""||imageID!=pinned{return backend.RuntimeImageObservation{},backend.ErrLaunchFenceOwnership}
	normalized,err:=runtimeImageTag(tag);if err!=nil||normalized!=tag{return backend.RuntimeImageObservation{},backend.ErrLaunchFenceOwnership}
	if err=d.selected.runtimeImageIdentity(ctx,id);err!=nil{return backend.RuntimeImageObservation{},err}
	api,ok:=d.selected.api.(runtimeImageLoadClient);if !ok{return backend.RuntimeImageObservation{},backend.ErrUnsupported}
	out:=backend.RuntimeImageObservation{ImageID:imageID,ImageTag:tag}
	row,err:=api.ImageInspect(ctx,imageID)
	if errdefs.IsNotFound(err){out.Publication="image-absent";return out,nil}
	if err!=nil{return backend.RuntimeImageObservation{},backend.ErrUnavailable}
	if row.ID!=imageID||!runtimeBaseValid(row){return backend.RuntimeImageObservation{},backend.ErrLaunchFenceOwnership}
	row,err=api.ImageInspect(ctx,tag)
	switch {case errdefs.IsNotFound(err):out.Publication="tag-absent";case err!=nil:return backend.RuntimeImageObservation{},backend.ErrUnavailable;case row.ID!=imageID:out.Publication="tag-different";default:out.Publication="published"}
	return out,d.selected.runtimeImageIdentity(ctx,id)
}

// DocumentImageID is not an observation-only adoption path. It requires this
// owner's positive load and publication receipts, then fresh exact readback.
// After restart an uncertain prior import remains unusable until the trusted
// coordinator explicitly reconciles and performs a positive no-write load.
func(d *DocumentRuntimeInstaller) DocumentImageID(ctx context.Context,p backend.PreparedRuntimeImage,settledImageID string)(string,error){
	if !d.validPrepared(p)||settledImageID!=p.Bundle.Manifest.Image.ID{return "",backend.ErrLaunchFenceOwnership}
	d.mu.Lock();loaded,published:=d.loaded[p.Spec.Identity],d.published[p.Spec.Identity];d.mu.Unlock()
	if loaded!=settledImageID||published!=settledImageID+"\x00"+p.Spec.ImageTag{return "",backend.ErrUnavailable}
	if err:=d.inspectDocument(ctx,p);err!=nil{return "",err}
	obs,err:=d.ObserveRuntimeImage(ctx,p.Spec.Identity,settledImageID,p.Spec.ImageTag)
	if err!=nil||obs.Publication!="published"{return "",backend.ErrUnavailable}
	return settledImageID,nil
}

func(d *DocumentRuntimeInstaller) DocumentExtractor(ctx context.Context,p backend.PreparedRuntimeImage,settledImageID string)(*DocumentExtractor,error){
	id,err:=d.DocumentImageID(ctx,p,settledImageID);if err!=nil{return nil,err}
	return NewDocumentExtractor(d.selected,id,d.attach)
}

