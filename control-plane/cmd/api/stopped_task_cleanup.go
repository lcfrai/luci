package main

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type stoppedTaskCleanupProof struct {
	confirm func(context.Context, registry.Session) error
}

// Reaping starts before API construction. Publishing an immutable callback
// avoids both granting early cleanup and racing an ordinary function assignment.
type stoppedTaskCleanupGate struct {
	proof atomic.Pointer[stoppedTaskCleanupProof]
}

func (g *stoppedTaskCleanupGate) confirm(ctx context.Context, row registry.Session) error {
	proof := g.proof.Load()
	if proof == nil || proof.confirm == nil {
		return errors.New("confirmed task cleanup is unavailable until the controller is ready")
	}
	return proof.confirm(ctx, row)
}

func (g *stoppedTaskCleanupGate) publish(confirm func(context.Context, registry.Session) error) {
	if confirm == nil {
		g.proof.Store(nil)
		return
	}
	g.proof.Store(&stoppedTaskCleanupProof{confirm: confirm})
}
