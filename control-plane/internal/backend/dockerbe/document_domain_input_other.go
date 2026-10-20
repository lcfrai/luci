//go:build !windows

package dockerbe

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func openDocumentDomainInputs(context.Context, domainInputPlan) (backend.DocumentDomainInputs, error) {
	// This custody profile is Windows-specific; path metadata alone cannot
	// substitute for held originals with cancellable original reads.
	return nil, backend.ErrUnsupported
}
