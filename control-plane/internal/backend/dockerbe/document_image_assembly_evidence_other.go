//go:build !windows

package dockerbe

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func lockAssemblyEvidenceSources(context.Context, []backend.DocumentImageAssemblyEvidenceObject) (*assemblySourceGuard, error) {
	return nil, backend.ErrUnsupported
}
func (*assemblySourceGuard) readEvidence(context.Context, backend.DocumentImageAssemblyEvidenceObject, int64) ([]byte, error) {
	return nil, backend.ErrUnsupported
}
