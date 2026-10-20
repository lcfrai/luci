package dockerbe

import (
 "context"
 "github.com/lcfrai/luci/control-plane/internal/backend"
)

// Historical cleanup is independent of current parser package/image readiness.
// This type deliberately exposes no image selection, prepare/create/start or
// attach method. Existing exact original identity/config/fence checks remain
// authoritative on every forwarded retirement operation.
type documentRetirement struct{original *DocumentExtractor}
func NewDocumentRetirement(b *Backend)(backend.DocumentExtractionRetirementExecutor,error){if b==nil{return nil,backend.ErrUnsupported};return &documentRetirement{original:&DocumentExtractor{Backend:b}},nil}
func(d *documentRetirement)InspectDocument(ctx context.Context,id backend.DocumentExtractionIdentity,ref string)(backend.DocumentExtractionObservation,error){return d.original.InspectDocument(ctx,id,ref)}
func(d *documentRetirement)RetireDocument(ctx context.Context,id backend.DocumentExtractionIdentity,ref string)error{return d.original.RetireDocument(ctx,id,ref)}
func(d *documentRetirement)FenceDocumentLaunch(ctx context.Context,r backend.DocumentExtractionFenceRequest)(backend.DocumentExtractionFenceProof,error){return d.original.FenceDocumentLaunch(ctx,r)}
func(d *documentRetirement)VerifyDocumentLaunchFence(ctx context.Context,r backend.DocumentExtractionFenceRequest,p backend.DocumentExtractionFenceProof)error{return d.original.VerifyDocumentLaunchFence(ctx,r,p)}
