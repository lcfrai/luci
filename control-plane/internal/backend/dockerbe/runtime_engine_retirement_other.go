//go:build !windows

package dockerbe

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// The shared Linux or remote Docker backend has no portable daemon-generation
// readback. A failed connection or absent intermediate is not a stop receipt.
func (b *Backend) ObserveRuntimeImageEngine(context.Context) (backend.RuntimeImageEngineState, error) {
	return backend.RuntimeImageEngineState{}, backend.ErrUnsupported
}
