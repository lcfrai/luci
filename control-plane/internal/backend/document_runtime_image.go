package backend

import "context"

// DocumentRuntimeInstaller is the selected-engine, document-specific packaged
// installer. Its effects are deliberately the same individually journaled
// phases as the ordinary RuntimeImageImportExecutor. A document extractor may
// only be obtained after a positive publication readback of a settled import.
type DocumentRuntimeInstaller interface {
	RuntimeImageImportExecutor
	DocumentImageID(context.Context, PreparedRuntimeImage, string) (string, error)
	RestoreDocumentExtractor(context.Context, string, DocumentRuntimePublishedReceipt) (DocumentExtractionExecutor, error)
}

// DocumentRuntimePublishedReceipt is supplied only after the API rereads an
// exact, durable finished packaged-publication lease. It names the historical
// receipt; it does not grant an installer permission to settle unknown work.
type DocumentRuntimePublishedReceipt struct {
	Identity RuntimeImageIdentity
	ImageID string
	ImageTag string
	ManifestSHA256 string
}
