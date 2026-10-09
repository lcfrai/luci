package dockerbe

import (
	"context"
	"fmt"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

var _ backend.AllocationAbsenceObserver = (*Backend)(nil)

// AllocationAbsent performs no ownership filtering: even a foreign object at
// the logical name retains the old charge. An original container renamed away
// from that name is still present at its immutable reference and also retains.
func (b *Backend) AllocationAbsent(ctx context.Context, id, immutableRef string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !fenceLogicalID(id) || !fenceHex(immutableRef, 32) {
		return false, backend.ErrUnsupported
	}
	if _, creating := b.creates.Load(id); creating {
		return false, nil
	}
	for _, target := range []string{namePrefix + id, immutableRef} {
		_, err := b.api.ContainerInspect(ctx, target)
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err == nil {
			return false, nil
		}
		if !stoppedCleanupNotFound(err) {
			return false, fmt.Errorf("dockerbe: exact allocation absence is unconfirmed: %w", backend.ErrUnavailable)
		}
	}
	if _, creating := b.creates.Load(id); creating {
		return false, nil
	}
	return true, nil
}
