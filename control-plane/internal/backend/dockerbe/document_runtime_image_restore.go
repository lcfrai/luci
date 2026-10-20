package dockerbe

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

// RestoreDocumentExtractor is a read-only restart path for an API-validated
// completed publication receipt. A current Docker observation alone cannot
// turn an unknown, pending, or cancelled installer lease into completion.
// This path never calls ImageLoad, ImageTag, or Publish and does not populate
// the in-memory load/publication maps used by a new installation attempt.
func (d *DocumentRuntimeInstaller) RestoreDocumentExtractor(ctx context.Context, sourceDirectory string, receipt backend.DocumentRuntimePublishedReceipt) (backend.DocumentExtractionExecutor, error) {
	if d == nil || !validRuntimeIdentity(receipt.Identity, true) ||
		receipt.Identity.SourceDigest != d.manifestSHA256 || receipt.ManifestSHA256 != d.manifestSHA256 ||
		!runtimeImageID(receipt.ImageID) {
		return nil, backend.ErrLaunchFenceOwnership
	}
	tag, err := runtimeImageTag(receipt.ImageTag)
	if err != nil || tag != receipt.ImageTag {
		return nil, backend.ErrLaunchFenceOwnership
	}
	bundle, err := runtimeoci.OpenDocument(sourceDirectory, d.manifestSHA256)
	if err != nil { return nil, err }
	defer bundle.Close()
	if bundle.Manifest.Image.ID != receipt.ImageID || ctx.Err() != nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	p := backend.PreparedRuntimeImage{Spec:backend.RuntimeImageSpec{Identity:receipt.Identity, ImageTag:tag}, DocumentBundle:bundle}
	if err = d.inspectDocument(ctx,p); err != nil { return nil, err }
	api, ok := d.selected.api.(runtimeImageClient)
	if !ok { return nil, backend.ErrUnsupported }
	row, err := api.ImageInspect(ctx,tag)
	if err != nil { return nil, backend.ErrUnavailable }
	if row.ID != receipt.ImageID || !runtimeBaseValid(row) || row.Architecture != "amd64" || ctx.Err() != nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if err = bundle.VerifyArchive(); err != nil { return nil, err }
	if err = d.selected.runtimeImageIdentity(ctx,receipt.Identity); err != nil { return nil, err }
	return NewDocumentExtractor(d.selected,receipt.ImageID,d.attach)
}
