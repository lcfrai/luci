package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type stoppedCleanupBackendFixture struct {
	*availableBackend
	observed backend.Sandbox
	ctx      context.Context
	result   error
}

func (b *stoppedCleanupBackendFixture) DestroyStopped(ctx context.Context, observed backend.Sandbox) error {
	b.ctx, b.observed = ctx, observed
	return b.result
}
func TestRecoveringBackendStoppedCleanupKeepsImmutableProofAndNeverForces(t *testing.T) {
	observed := backend.Sandbox{ID: "owned", Ref: "immutable-ref", Instance: "owned-instance", State: backend.StateStopped, Labels: map[string]string{backend.LabelSessionID: "owned-session"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	refused := errors.New("container resumed; stopped-only removal refused")
	ready := &stoppedCleanupBackendFixture{availableBackend: &availableBackend{}, result: refused}
	b := &recoveringBackend{factory: func() (backend.Backend, error) { return ready, nil }}
	if err := b.DestroyStopped(ctx, observed); !errors.Is(err, refused) || ready.ctx != ctx || !reflect.DeepEqual(ready.observed, observed) || ready.destroyedID != "" {
		t.Fatal("recovery altered immutable cleanup or fell back to force", err)
	}
	ready.result = nil
	if err := b.DestroyStopped(ctx, observed); err != nil {
		t.Fatal(err)
	}
	legacy := &availableBackend{}
	b = &recoveringBackend{factory: func() (backend.Backend, error) { return legacy, nil }}
	if err := b.DestroyStopped(ctx, observed); !errors.Is(err, backend.ErrUnsupported) || legacy.destroyedID != "" {
		t.Fatal("missing optional capability fell back to force", err)
	}
	b = &recoveringBackend{factory: func() (backend.Backend, error) { return nil, backend.ErrUnavailable }}
	if err := b.DestroyStopped(ctx, observed); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatal("unavailable cleanup was reported complete", err)
	}
}
