package main

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Recovery must retain the optional immutable, non-force cleanup capability.
// A backend without it is refused; Destroy is never a substitute.
func (b *recoveringBackend) DestroyStopped(ctx context.Context, observed backend.Sandbox) error {
	value, err := b.get()
	if err != nil {
		return err
	}
	if cleaner, ok := value.(interface {
		DestroyStopped(context.Context, backend.Sandbox) error
	}); ok {
		return cleaner.DestroyStopped(ctx, observed)
	}
	return backend.ErrUnsupported
}
