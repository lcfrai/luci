package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestLocal044PressureResponseIsSustainedAndOneOwnedStopPerEpisode(t *testing.T) {
	available := uint64(900 << 20)
	stops := 0
	g := localHostPressureGuard{
		sample: func(context.Context) resourcebudget.HostSnapshot {
			total := uint64(8 << 30)
			return resourcebudget.HostSnapshot{ObservedAt: time.Now().UTC(), Memory: resourcebudget.Memory{EffectiveTotalBytes: &total, EffectiveAvailableBytes: &available}}
		},
		policy: func() (product.SwarmSchedulerPolicy, error) {
			return product.SwarmSchedulerPolicy{HeadroomPercent: 10, MinHeadroomMiB: 1024}, nil
		},
		stop:   func(context.Context) (bool, error) { stops++; return true, nil },
		log:    func(error) {},
		record: func(uint64, uint64) {},
	}
	ctx := context.Background()
	g.step(ctx)
	if stops != 0 {
		t.Fatal("one transient sample stopped work")
	}
	g.step(ctx)
	for range 9 {
		g.step(ctx)
	}
	if stops != 1 {
		t.Fatal("persistent low host memory caused repeated workload kills", stops)
	}
	available = 2 << 30
	for range 3 {
		g.step(ctx)
	}
	available = 900 << 20
	g.step(ctx)
	if stops != 1 {
		t.Fatal("one recovered sample reset the guard", stops)
	}
	available = 2 << 30
	for range 8 {
		g.step(ctx)
	}
	available = 900 << 20
	g.step(ctx)
	g.step(ctx)
	if stops != 2 {
		t.Fatal("new measured pressure episode was ignored", stops)
	}
}

func TestLocal044PressureStopsOnlyNewestExactLocalContainer(t *testing.T) {
	env := sessionsNewEnv(t)
	old := registry.Session{ID: "s-local044-old", MachineID: LocalMachineID, Backend: "docker", SandboxID: "s-local044-old", SandboxRef: "exact-old", Status: registry.StatusRunning, CreatedAt: time.Now().Add(-time.Minute)}
	newer := registry.Session{ID: "s-local044-new", MachineID: LocalMachineID, Backend: "docker", SandboxID: "s-local044-new", SandboxRef: "exact-new", Status: registry.StatusRunning, CreatedAt: time.Now()}
	foreign := registry.Session{ID: "s-local044-foreign", MachineID: "remote", Backend: "docker", SandboxID: "s-local044-foreign", SandboxRef: "foreign", Status: registry.StatusRunning, CreatedAt: time.Now().Add(time.Minute)}
	for _, row := range []registry.Session{old, newer, foreign} {
		if err := env.st.Create(row); err != nil {
			t.Fatal(err)
		}
	}
	acted, err := env.api.stopNewestOwnedSessionForPressure(context.Background())
	if err != nil || !acted {
		t.Fatal("exact pressure stop failed", acted, err)
	}
	env.be.mu.Lock()
	destroyed := append([]string(nil), env.be.destroys...)
	env.be.mu.Unlock()
	if len(destroyed) != 1 || destroyed[0] != newer.SandboxID {
		t.Fatal("an unrelated or older sandbox was destroyed", destroyed)
	}
	for _, check := range []struct {
		id, status, reason string
	}{
		{old.ID, string(registry.StatusRunning), ""},
		{newer.ID, string(registry.StatusExited), "host_memory_pressure"},
		{foreign.ID, string(registry.StatusRunning), ""},
	} {
		row, err := env.st.Get(check.id)
		if err != nil || string(row.Status) != check.status || row.ExitReason != check.reason {
			t.Fatal("pressure response changed the wrong record", check.id, row, err)
		}
	}
}

func TestLocal044UnknownDestroyRetainsSessionAndCannotSettleLease(t *testing.T) {
	env := sessionsNewEnv(t)
	row := registry.Session{ID: "s-local044-unknown", ProjectID: "p-local044", MachineID: LocalMachineID, Backend: "docker", SandboxID: "s-local044-unknown", SandboxRef: "exact-unknown", Status: registry.StatusRunning}
	if err := env.st.Create(row); err != nil {
		t.Fatal(err)
	}
	if err := env.api.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		l.Leases = append(l.Leases, executionLease{ID: "local044-held", Scope: env.api.execution.scope, Actor: management.ExecutionActorRef{Local: true, ProjectID: row.ProjectID}, ProjectID: row.ProjectID, Provider: "harness:codex", ProviderLimit: 1, SessionID: row.ID, SandboxID: row.SandboxID, State: "backend-settled", LaunchGuard: 1, Resources: backend.Resources{MemoryMiB: 512, CPUMillis: 1000}, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	env.be.destroyErr = errors.New("Docker stop acknowledgment lost")
	acted, err := env.api.stopNewestOwnedSessionForPressure(context.Background())
	if !acted || err == nil {
		t.Fatal("unknown stop was recorded as clean", acted, err)
	}
	retained, err := env.st.Get(row.ID)
	if err != nil || retained.Status != registry.StatusRunning || retained.ExitReason != "" {
		t.Fatal("unknown external effect fabricated terminal state", retained, err)
	}
	if err := env.api.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 1 || l.Leases[0].ID != "local044-held" {
			t.Fatal("unknown stop released the original capacity", l.Leases)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
