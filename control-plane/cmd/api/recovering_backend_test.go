package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type availableBackend struct {
	mu                                     sync.Mutex
	created                                backend.Spec
	attachedID, attachedToken, destroyedID string
	closed                                 int
}

func (*availableBackend) Kind() backend.Kind { return backend.KindDocker }
func (b *availableBackend) Create(_ context.Context, spec backend.Spec) (backend.Sandbox, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.created = spec
	return backend.Sandbox{ID: spec.ID, State: backend.StateRunning}, nil
}
func (*availableBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	if id == "absent" {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	return backend.Sandbox{ID: id, State: backend.StateRunning}, nil
}
func (*availableBackend) List(context.Context) ([]backend.Sandbox, error) {
	return []backend.Sandbox{{ID: "existing", State: backend.StateRunning}}, nil
}
func (b *availableBackend) Attach(_ context.Context, id, token string) (*websocket.Conn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attachedID, b.attachedToken = id, token
	return nil, nil
}
func (b *availableBackend) Destroy(_ context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.destroyedID = id
	return nil
}
func (b *availableBackend) Close() error { b.mu.Lock(); defer b.mu.Unlock(); b.closed++; return nil }

func TestRecoveringBackendRemainsOfflineThenDelegatesWhenFactoryRecovers(t *testing.T) {
	var attempts atomic.Int32
	ready := &availableBackend{}
	b := &recoveringBackend{factory: func() (backend.Backend, error) {
		if attempts.Add(1) == 1 {
			return nil, backend.ErrUnavailable
		}
		return ready, nil
	}}
	ctx := context.Background()
	if _, err := b.List(ctx); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("offline List: %v", err)
	}
	if _, err := b.Create(ctx, backend.Spec{ID: "too-early"}); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("offline Create: %v", err)
	}
	if attempts.Load() != 1 {
		t.Fatal("offline requests bypassed reconnect backoff")
	}
	b.mu.Lock()
	if !b.nextAttempt.After(time.Now()) {
		t.Fatal("offline factory did not schedule bounded retry")
	}
	// Expire the backoff without sleeping for a daemon that does not exist.
	b.nextAttempt = time.Now().Add(-time.Second)
	b.mu.Unlock()
	var group sync.WaitGroup
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			rows, err := b.List(ctx)
			if err != nil || len(rows) != 1 || rows[0].ID != "existing" {
				t.Errorf("recovered List: %#v, %v", rows, err)
			}
		}()
	}
	group.Wait()
	if attempts.Load() != 2 {
		t.Fatal("concurrent recovery created more than one backend")
	}
	spec := backend.Spec{ID: "new-session", Command: "claude", Args: []string{"one prompt"}, AttachToken: "fixture-attach-token"}
	if row, err := b.Create(ctx, spec); err != nil || row.ID != spec.ID {
		t.Fatalf("recovered Create: %#v, %v", row, err)
	}
	if _, err := b.Get(ctx, "absent"); !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("backend error was replaced: %v", err)
	}
	if _, err := b.Attach(ctx, spec.ID, spec.AttachToken); err != nil {
		t.Fatal(err)
	}
	if err := b.Destroy(ctx, spec.ID); err != nil {
		t.Fatal(err)
	}
	if ready.created.Command != spec.Command || len(ready.created.Args) != 1 || ready.created.Args[0] != "one prompt" || ready.attachedID != spec.ID || ready.attachedToken != spec.AttachToken || ready.destroyedID != spec.ID {
		t.Fatal("recovery altered a delegated session operation")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if ready.closed != 1 {
		t.Fatal("recovered backend was not closed")
	}
	if _, err := b.List(ctx); !errors.Is(err, backend.ErrUnavailable) || attempts.Load() != 2 {
		t.Fatal("closed backend reconnected")
	}
}

func TestClosingAnOfflineRecoveringBackendDoesNotInitializeDocker(t *testing.T) {
	b := &recoveringBackend{factory: func() (backend.Backend, error) {
		t.Fatal("close initialized a backend")
		return nil, backend.ErrUnavailable
	}}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Get(context.Background(), "unused"); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("closed backend Get: %v", err)
	}
}
