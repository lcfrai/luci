package backend

import "context"

// DocumentRuntimeInstaller is the selected-engine, document-specific packaged
// installer. Its effects are deliberately the same individually journaled
// phases as the ordinary RuntimeImageImportExecutor. A document extractor may
// only be obtained after a positive publication readback of a settled import.
type DocumentRuntimeInstaller interface {
	RuntimeImageImportExecutor
	DocumentImageID(context.Context, PreparedRuntimeImage, string) (string, error)
}
