package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func local044Lease(t *testing.T, a *API, row registry.Session) {
	t.Helper()
	if err := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		l.Leases = append(l.Leases, executionLease{ID: "lease-" + row.ID, Scope: a.execution.scope, Actor: management.ExecutionActorRef{Local: true, ProjectID: row.ProjectID}, ProjectID: row.ProjectID, Provider: "harness:codex", ProviderLimit: 2, SessionID: row.ID, SandboxID: row.SandboxID, State: "backend-settled", LaunchGuard: 1, Resources: backend.Resources{MemoryMiB: 512, CPUMillis: 1000}, CreatedAt: time.Now().Add(-time.Minute), UpdatedAt: time.Now()})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func local044Product(t *testing.T, a *API) {
	t.Helper()
	p, err := product.New(product.Options{StateDir: filepath.Join(t.TempDir(), "product")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.deps.Product = p
}

func TestLocal044WorkerObservationAndExactStopUsePrivateRegistryAndLease(t *testing.T) {
	env := sessionsNewEnv(t)
	local044Product(t, env.api)
	old := registry.Session{ID: "s-local044-old", ProjectID: "p-old", MachineID: LocalMachineID, Backend: "docker", SandboxID: "s-local044-old", SandboxRef: "exact-old", Status: registry.StatusRunning, CreatedAt: time.Now().Add(-time.Minute)}
	newer := registry.Session{ID: "s-local044-new", ProjectID: "p-new", MachineID: LocalMachineID, Backend: "docker", SandboxID: "s-local044-new", SandboxRef: "exact-new", Status: registry.StatusRunning, CreatedAt: time.Now()}
	foreign := registry.Session{ID: "s-local044-foreign", ProjectID: "p-foreign", MachineID: "remote", Backend: "docker", SandboxID: "s-local044-foreign", SandboxRef: "foreign", Status: registry.StatusRunning, CreatedAt: time.Now().Add(time.Minute)}
	for _, row := range []registry.Session{old, newer, foreign} {
		if err := env.st.Create(row); err != nil {
			t.Fatal(err)
		}
	}
	local044Lease(t, env.api, old)
	local044Lease(t, env.api, newer)
	observation, err := env.api.HostPressureObservation(context.Background(), 8<<30)
	if err != nil || observation.Candidate == nil || observation.Candidate.SessionID != newer.ID || observation.Candidate.LeaseID != "lease-"+newer.ID {
		t.Fatal("newest exact local owned lease was not selected", observation, err)
	}
	stale := *observation.Candidate
	stale.SandboxRef = "another-generation"
	if acted, err := env.api.StopHostPressureCandidate(context.Background(), stale); acted || err != nil {
		t.Fatal("stale selected generation stopped work", acted, err)
	}
	acted, err := env.api.StopHostPressureCandidate(context.Background(), *observation.Candidate)
	if err != nil || !acted {
		t.Fatal("exact pressure stop failed", acted, err)
	}
	env.be.mu.Lock()
	destroyed := append([]string(nil), env.be.destroys...)
	env.be.mu.Unlock()
	if len(destroyed) != 1 || destroyed[0] != newer.SandboxID {
		t.Fatal("another actor's or older sandbox was destroyed", destroyed)
	}
	for _, row := range []registry.Session{old, foreign} {
		got, err := env.st.Get(row.ID)
		if err != nil || got.Status != registry.StatusRunning {
			t.Fatal("unselected work changed", got, err)
		}
	}
	got, err := env.st.Get(newer.ID)
	if err != nil || got.Status != registry.StatusExited || !strings.Contains(got.ExitReason, "protect your computer") {
		t.Fatal("stopped session did not explain the recovery to its owner", got, err)
	}
}

func TestLocal044UnknownDestroyKeepsExactRegistryAndLease(t *testing.T) {
	env := sessionsNewEnv(t)
	local044Product(t, env.api)
	row := registry.Session{ID: "s-local044-unknown", ProjectID: "p-local044", MachineID: LocalMachineID, Backend: "docker", SandboxID: "s-local044-unknown", SandboxRef: "exact-unknown", Status: registry.StatusRunning}
	if err := env.st.Create(row); err != nil {
		t.Fatal(err)
	}
	local044Lease(t, env.api, row)
	observation, err := env.api.HostPressureObservation(context.Background(), 8<<30)
	if err != nil || observation.Candidate == nil {
		t.Fatal(observation, err)
	}
	env.be.destroyErr = errors.New("Docker stop acknowledgment lost")
	acted, err := env.api.StopHostPressureCandidate(context.Background(), *observation.Candidate)
	if !acted || err == nil {
		t.Fatal("unknown stop was recorded as clean", acted, err)
	}
	retained, err := env.st.Get(row.ID)
	if err != nil || retained.Status != registry.StatusRunning || retained.ExitReason != "" {
		t.Fatal("unknown external effect fabricated terminal state", retained, err)
	}
	if err := env.api.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 1 || l.Leases[0].ID != "lease-"+row.ID {
			t.Fatal("unknown stop released the original capacity", l.Leases)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
