package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type stagedInitializationFixture struct{ stage string }

func (e stagedInitializationFixture) Error() string {
	return "private socket/config/credential fixture"
}
func (e stagedInitializationFixture) Unwrap() error                     { return backend.ErrUnavailable }
func (e stagedInitializationFixture) DockerInitializationStage() string { return e.stage }

type observingBackendFixture struct {
	*availableBackend
	snapshot    resourcebudget.ExecutionSnapshot
	seenContext context.Context
}

func (b *observingBackendFixture) ObserveResources(ctx context.Context) resourcebudget.ExecutionSnapshot {
	b.seenContext = ctx
	return b.snapshot
}

func TestRecoveringResourceDiagnosticsPreserveStageThroughCooldown(t *testing.T) {
	for _, tc := range []struct{ name, stage, code string }{
		{"configuration", "docker_client_configuration_unavailable", "docker_client_configuration_unavailable"},
		{"ping", "docker_initial_ping_unavailable", "docker_initial_ping_unavailable"},
		{"unknown", "", "execution_runtime_initialization_unavailable"},
		{"untrusted stage", "private socket/config/credential fixture", "execution_runtime_initialization_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			b := &recoveringBackend{factory: func() (backend.Backend, error) {
				attempts++
				return nil, fmt.Errorf("private wrapper: %w", stagedInitializationFixture{tc.stage})
			}}
			first := b.ObserveResources(context.Background())
			if attempts != 1 || first.Status != "unavailable" || !first.ObservedAt.IsZero() || first.InventoryComplete || first.TotalMemoryBytes != nil || first.CPUMillis != nil || len(first.Unavailable) != 1 || !strings.Contains(first.Unavailable[0].Reason, tc.code) || strings.Contains(first.Unavailable[0].Reason, "private") {
				t.Fatal("public failure was unbounded, leaked private errors or invented capacity", first)
			}
			if tc.name != "ping" && strings.Contains(first.Unavailable[0].Reason, "Start Docker Desktop") {
				t.Fatal("non-Ping failure promised that starting Desktop would fix configuration")
			}
			b.mu.Lock()
			cooldownScheduled := b.nextAttempt.After(time.Now())
			b.nextAttempt = time.Now().Add(time.Hour) // Deterministic cooldown, no real waiting.
			cutoff := b.nextAttempt
			b.mu.Unlock()
			if !cooldownScheduled {
				t.Fatal("existing reconnect cooldown missing")
			}
			second := b.ObserveResources(context.Background())
			if attempts != 1 || !reflect.DeepEqual(first, second) || !b.nextAttempt.Equal(cutoff) {
				t.Fatal("cooldown lost the fixed cause, retried or renewed its deadline")
			}
		})
	}
}

func TestRecoveringResourceDiagnosticsDelegateAfterRecovery(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "fixture")
	want := resourcebudget.ExecutionSnapshot{Status: "partial", Scope: "execution-daemon", ObservedAt: time.Now(), InventoryComplete: true, Unavailable: []resourcebudget.Issue{{Metric: "execution.daemonDisk", Reason: "fixture disk missing"}}}
	ready := &observingBackendFixture{availableBackend: &availableBackend{}, snapshot: want}
	attempts := 0
	b := &recoveringBackend{factory: func() (backend.Backend, error) {
		attempts++
		if attempts == 1 {
			return nil, stagedInitializationFixture{"docker_initial_ping_unavailable"}
		}
		return ready, nil
	}}
	_ = b.ObserveResources(ctx)
	b.mu.Lock()
	b.nextAttempt = time.Now().Add(-time.Second)
	b.mu.Unlock()
	got := b.ObserveResources(ctx)
	if attempts != 2 || !reflect.DeepEqual(got, want) || ready.seenContext != ctx || b.initializationReason != "" {
		t.Fatal("recovery changed the context/snapshot or retained stale initialization failure")
	}
	if !reflect.DeepEqual(b.ObserveResources(ctx), want) || attempts != 2 {
		t.Fatal("a recovered backend was constructed again")
	}
}

func TestRecoveringResourceDiagnosticsKeepUnknownClosedAndUnsupportedDistinct(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		closed     bool
		value      backend.Backend
		err        error
	}{
		{"unknown", "execution_runtime_initialization_unavailable", false, nil, errors.New("private socket/config/credential fixture")},
		{"closed", "execution_runtime_closed", true, nil, nil},
		{"unsupported", "execution_resource_observer_unsupported", false, &availableBackend{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			b := &recoveringBackend{closed: tc.closed, initializationReason: safeBackendInitializationReason(stagedInitializationFixture{"docker_initial_ping_unavailable"}), factory: func() (backend.Backend, error) { attempts++; return tc.value, tc.err }}
			got := b.ObserveResources(context.Background())
			if len(got.Unavailable) != 1 || !strings.Contains(got.Unavailable[0].Reason, tc.code) || strings.Contains(got.Unavailable[0].Reason, "private") || got.InventoryComplete || !got.ObservedAt.IsZero() {
				t.Fatal("distinct refusal was lost or leaked a private error", got)
			}
			if tc.closed && attempts != 0 {
				t.Fatal("closed runtime called its factory")
			}
		})
	}
}
