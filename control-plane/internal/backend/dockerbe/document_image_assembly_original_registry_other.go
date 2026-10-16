//go:build !windows

package dockerbe

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func (*assemblySourceGuard) originalRegistry() *assemblyOriginalRegistry { return nil }
func prepareAssemblyOriginalGuard([]backend.DocumentImageAssemblyEvidenceObject) (*assemblySourceGuard, error) {
	return nil, backend.ErrUnsupported
}
func acquireAssemblyPreparedSources(context.Context, *assemblySourceGuard, []backend.DocumentImageAssemblyEvidenceObject) error {
	return backend.ErrUnsupported
}
