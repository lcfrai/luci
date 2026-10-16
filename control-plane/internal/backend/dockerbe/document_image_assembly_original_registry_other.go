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
func prepareAssemblyOriginalNamespaceGuard(string) (*assemblySourceGuard, error) {
	return nil, backend.ErrUnsupported
}
func acquireAssemblyPreparedNamespace(context.Context, *assemblySourceGuard, string) error {
	return backend.ErrUnsupported
}
