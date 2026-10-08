package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestStoppedTaskCleanupWiringDeniesUntilProofPublication(t *testing.T) {
	g := &stoppedTaskCleanupGate{}
	row := registry.Session{ID: "controlled-child", ProjectID: "controlled-project", SandboxID: "controlled-sandbox", SandboxRef: "immutable-controlled-ref"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := g.confirm(ctx, row); err == nil {
		t.Fatal("startup without a validated API granted immediate cleanup")
	}
	denied := errors.New("exact launch settlement was not proven")
	var calls int
	g.publish(func(got context.Context, observed registry.Session) error {
		calls++
		if got != ctx || observed.ID != row.ID || observed.ProjectID != row.ProjectID || observed.SandboxID != row.SandboxID || observed.SandboxRef != row.SandboxRef {
			t.Fatal("wiring changed proof identity or context")
		}
		return denied
	})
	if err := g.confirm(ctx, row); !errors.Is(err, denied) || calls != 1 {
		t.Fatal("wiring dropped the API denial", err, calls)
	}
	g.publish(func(context.Context, registry.Session) error { calls++; return nil })
	if err := g.confirm(ctx, row); err != nil || calls != 2 {
		t.Fatal("published positive proof was not delegated", err, calls)
	}
	g.publish(nil)
	if err := g.confirm(ctx, row); err == nil || calls != 2 {
		t.Fatal("cleared proof retained cleanup permission", err, calls)
	}
}

func TestStoppedTaskCleanupWiringPublicationIsRaceSafe(t *testing.T) {
	g := &stoppedTaskCleanupGate{}
	var calls atomic.Int32
	confirm := func(context.Context, registry.Session) error { calls.Add(1); return nil }
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 100 {
				g.publish(confirm)
				_ = g.confirm(context.Background(), registry.Session{})
				g.publish(nil)
			}
		})
	}
	workers.Wait()
	g.publish(confirm)
	if err := g.confirm(context.Background(), registry.Session{}); err != nil || calls.Load() == 0 {
		t.Fatal("atomic proof publication did not converge", err)
	}
	g.publish(nil)
	if err := g.confirm(context.Background(), registry.Session{}); err == nil {
		t.Fatal("shutdown enabled cleanup")
	}
}
