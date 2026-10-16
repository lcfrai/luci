//go:build !windows

package dockerbe

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func lockAssemblyPackageInputSources(context.Context, []backend.DocumentImageAssemblyEvidenceObject) (*assemblySourceGuard, error) {
	return nil, backend.ErrUnsupported
}
