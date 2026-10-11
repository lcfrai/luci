package e2bbe

import "context"

// beginIO registers before any accepted client operation can have an effect.
// Controller.Close keeps its journal/checkpoint lease until all such operations
// return, including body verification/storage after HTTP headers arrive. This
// proves client drain only: cancelling a request is not remote mutation
// settlement and never releases the VM charge. Returned WebSockets remain
// owned by the attach caller after the registered handshake has completed.
func (g *GuestTransport) beginIO(ctx context.Context) (context.Context, func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil, nil, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	owner := g.owner
	if owner != nil {
		owner.mu.Lock()
		if owner.closed {
			owner.mu.Unlock()
			return nil, nil, ErrClosed
		}
		owner.workers.Add(1)
		owner.mu.Unlock()
	}
	child, cancel := context.WithCancel(ctx)
	stopGuest := context.AfterFunc(g.ctx, cancel)
	stopOwner := func() bool { return true }
	if owner != nil {
		stopOwner = context.AfterFunc(owner.ctx, cancel)
	}
	g.operations.Add(1)
	return child, func() {
		stopGuest()
		stopOwner()
		cancel()
		g.operations.Done()
		if owner != nil {
			owner.workers.Done()
		}
	}, nil
}
