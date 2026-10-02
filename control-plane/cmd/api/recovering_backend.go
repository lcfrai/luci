package main

import (
	"context"
	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"sync"
	"time"
)

// Setup, briefs and designs remain usable when Docker has not been installed.
// The session backend reconnects on demand when the daemon becomes available.
type recoveringBackend struct {
	mu          sync.Mutex
	current     backend.Backend
	factory     func() (backend.Backend, error)
	nextAttempt time.Time
	closed      bool
}

func (b *recoveringBackend) Kind() backend.Kind { return backend.KindDocker }
func (b *recoveringBackend) get() (backend.Backend, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, backend.ErrUnavailable
	}
	if b.current != nil {
		return b.current, nil
	}
	if time.Now().Before(b.nextAttempt) {
		return nil, backend.ErrUnavailable
	}
	value, err := b.factory()
	if err != nil {
		b.nextAttempt = time.Now().Add(5 * time.Second)
		return nil, err
	}
	b.current = value
	return value, nil
}
func (b *recoveringBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	value, err := b.get()
	if err != nil {
		return backend.Sandbox{}, err
	}
	return value.Create(ctx, spec)
}
func (b *recoveringBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	value, err := b.get()
	if err != nil {
		return backend.Sandbox{}, err
	}
	return value.Get(ctx, id)
}
func (b *recoveringBackend) List(ctx context.Context) ([]backend.Sandbox, error) {
	value, err := b.get()
	if err != nil {
		return nil, err
	}
	return value.List(ctx)
}
func (b *recoveringBackend) Attach(ctx context.Context, id, token string) (*websocket.Conn, error) {
	value, err := b.get()
	if err != nil {
		return nil, err
	}
	return value.Attach(ctx, id, token)
}
func (b *recoveringBackend) Destroy(ctx context.Context, id string) error {
	value, err := b.get()
	if err != nil {
		return err
	}
	return value.Destroy(ctx, id)
}
func (b *recoveringBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.current != nil {
		return b.current.Close()
	}
	return nil
}
