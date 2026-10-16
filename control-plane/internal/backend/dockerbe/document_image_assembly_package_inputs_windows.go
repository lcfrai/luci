//go:build windows

package dockerbe

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func lockAssemblyPackageInputSources(ctx context.Context, objects []backend.DocumentImageAssemblyEvidenceObject) (*assemblySourceGuard, error) {
	// This existing function has no 16MiB wrapper policy or payload I/O. It
	// acquires ancestors + each original before ordinary type/link/size checks,
	// returning the exact native collector even on partial/cancelled failure.
	guard, err := lockAssemblyEvidenceSources(ctx, objects)
	if guard != nil && len(guard.handles) > backend.DocumentImageAssemblyPackageInputMaxNativeHandles {
		return guard, backend.ErrUnavailable
	}
	return guard, err
}
