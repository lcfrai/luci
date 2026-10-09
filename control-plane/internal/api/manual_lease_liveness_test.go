package api

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Structural optional capability keeps this exact regression source compatible
// with original production, which does not call AllocationAbsent yet.
type manualAbsenceBackend struct {
	backend.Backend
	get func(context.Context, string) (backend.Sandbox, error)
}

func (b manualAbsenceBackend) AllocationAbsent(ctx context.Context, id, ref string) (bool, error) {
	if ref == "" {
		return false, backend.ErrUnsupported
	}
	_, err := b.get(ctx, id)
	if err == nil {
		return false, nil
	}
	if unambiguousBackendNotFound(err) {
		return true, nil
	}
	return false, err
}

// Durable controlled manual allocation: no real process or Docker resources.
// The same fixture/test body runs against both original and fixed production.
func manualLeaseFixture(t *testing.T) (*sessionsEnv, registry.Session, executionLease) {
	t.Helper()
	root := t.TempDir()
	store, err := registry.NewFileStore(filepath.Join(root, "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	options := product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()}
	pd, err := product.New(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pd.Close() })
	env := sessionsNewEnvOverWith(t, root, store, store, func(d *Deps) {
		d.Product, d.ProjectsDir = pd, options.ProjectsDir
		d.ResourceSnapshot = executionFixtureSnapshot
		d.ReadyTimeout = 5 * time.Second
		d.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	})
	row := sessionsFullRecord()
	row.Harness, row.Command, row.Args = "shell", "/bin/bash", []string{"-l"}
	row.Status, row.Endpoint, row.AttachToken, row.AttachCount = registry.StatusExited, "", "", 0
	row = sessionsSeed(t, store, row)
	g := env.api.execution
	now := time.Now().UTC()
	lease := executionLease{ID: "manual-c070", Scope: g.scope, Actor: management.ExecutionActorRef{Local: true, ProjectID: row.ProjectID}, ProjectID: row.ProjectID, Provider: "harness:shell", ProviderLimit: 4, SessionID: row.ID, SandboxID: row.SandboxID, State: "backend-settled", LaunchGuard: 1, Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}, CreatedAt: now, UpdatedAt: now}
	if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error { l.Leases = append(l.Leases, lease); return nil }); err != nil {
		t.Fatal(err)
	}
	return env, row, lease
}

func manualLeaseBytes(t *testing.T, a *API) []byte {
	t.Helper()
	data, err := os.ReadFile(a.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Regression: a service restart with an empty scheduler queue must settle
// confirmed-absent manual capacity without another admission, Stop, or Forget.
// This does not claim the original admission-triggered reconciliation leaked.
func TestManualLeaseBackgroundReopenRetiresConfirmedAbsenceWithoutAdmission(t *testing.T) {
	env, row, own := manualLeaseFixture(t)
	a := env.api
	foreign := own
	foreign.ID, foreign.Scope, foreign.SessionID, foreign.SandboxID = "manual-foreign", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "s-foreign", "s-foreign"
	if err := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error { l.Leases = append(l.Leases, foreign); return nil }); err != nil {
		t.Fatal(err)
	}
	var reads, authority atomic.Int64
	a.deps.Backend = manualAbsenceBackend{Backend: env.be, get: func(ctx context.Context, id string) (backend.Sandbox, error) {
		reads.Add(1)
		if id != row.SandboxID {
			t.Error("observer read a foreign allocation")
		}
		return backend.Sandbox{}, backend.ErrNotFound
	}}
	a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
		authority.Add(1)
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	a.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
		authority.Add(1)
		return management.ErrExecutionAuthority
	}
	d := a.deps
	store, err := registry.NewFileStore(filepath.Join(filepath.Dir(env.dir), "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	d.Store, d.Flags, d.ObserveTaskCompletions = store, store, true
	reopened, err := New(d)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		ledger, err := reopened.execution.readFenceLedger()
		if err != nil {
			t.Fatal(err)
		}
		if len(ledger.Leases) == 1 && ledger.Leases[0] == foreign {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("confirmed-absent manual reservation stayed charged without a new admission")
		case <-poll.C:
		}
	}
	view := reopened.execution.reservationObservation(context.Background())
	if view.Status != "ready" || view.Count != 1 || view.MemoryMiB != 1024 || view.CPUMillis != 1000 {
		t.Fatal("reservation view did not preserve only foreign demand", view)
	}
	if reads.Load() != 1 || authority.Load() != 0 || len(env.be.specs) != 0 || len(env.be.destroys) != 0 {
		t.Fatal("observation allocated, destroyed, reused authority or repeated a retired lookup")
	}
	after := manualLeaseBytes(t, reopened)
	if err := d.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, manualLeaseBytes(t, reopened)) {
		t.Fatal("idle repeated turn rewrote durable ledger")
	}
}
