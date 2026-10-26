package api

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"strings"
	"testing"
	"time"
)

type managedModelTestBackend struct {
	*sessionsBackend
	live             map[string]bool
	keepAfterDestroy bool
}

func (b *managedModelTestBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	if b.live[id] {
		return backend.Sandbox{ID: id, Kind: backend.KindDocker, State: backend.StateRunning}, nil
	}
	return backend.Sandbox{}, backend.ErrNotFound
}
func (b *managedModelTestBackend) Destroy(ctx context.Context, id string) error {
	if e := b.sessionsBackend.Destroy(ctx, id); e != nil {
		return e
	}
	if !b.keepAfterDestroy {
		delete(b.live, id)
	}
	return nil
}
func managedSessionFixture(t *testing.T, status registry.Status) (*sessionsEnv, *managedModelTestBackend, registry.Session) {
	t.Helper()
	env := sessionsNewEnv(t)
	local044Product(t, env.api)
	be := &managedModelTestBackend{sessionsBackend: env.be, live: map[string]bool{}}
	env.api.deps.Backend = be
	id := "s-managed-model"
	binding := &managedmodel.Binding{Boot: strings.Repeat("a", 64), Generation: "7", Revision: 2, RuntimeScope: env.api.execution.scope, LeaseID: "lease-" + id, Endpoint: "http://127.0.0.1:24567/v1", Model: "installed:latest"}
	row := registry.Session{ID: id, ProjectID: "p-managed-model", Harness: "opencode", MachineID: LocalMachineID, Backend: "docker", SandboxID: id, SandboxRef: "exact-managed-model", Status: status, CreatedAt: time.Now(), ManagedLocalModel: binding}
	if e := env.st.Create(row); e != nil {
		t.Fatal(e)
	}
	local044Lease(t, env.api, row)
	if e := env.api.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		for i := range l.Leases {
			if l.Leases[i].ID == binding.LeaseID {
				copy := *binding
				l.Leases[i].ManagedLocalModel = &copy
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	be.live[id] = true
	row, _ = env.st.Get(id)
	return env, be, row
}
func TestManagedModelCloseRetiresOnlyExactGenerationAndLease(t *testing.T) {
	env, be, row := managedSessionFixture(t, registry.StatusRunning)
	unrelated := registry.Session{ID: "s-unrelated-byo", ProjectID: "p-byo", MachineID: LocalMachineID, Backend: "docker", SandboxID: "s-unrelated-byo", SandboxRef: "byo-ref", Status: registry.StatusRunning}
	if e := env.st.Create(unrelated); e != nil {
		t.Fatal(e)
	}
	be.live[unrelated.ID] = true
	if e := env.api.ManagedModelDependents(context.Background(), row.ManagedLocalModel.Boot, "7", false); e == nil || len(be.destroys) != 0 {
		t.Fatal("manual stop killed dependent work")
	}
	if e := env.api.ManagedModelDependents(context.Background(), row.ManagedLocalModel.Boot, "7", true); e != nil {
		t.Fatal(e)
	}
	if len(be.destroys) != 1 || be.destroys[0] != row.ID || be.live[row.ID] || !be.live[unrelated.ID] {
		t.Fatal("exact dependent identity or unrelated BYO isolation lost", be.destroys)
	}
	current, _ := env.st.Get(row.ID)
	if current.Status != registry.StatusExited {
		t.Fatal("verified removal not recorded")
	}
	ledger, e := env.api.execution.readFenceLedger()
	if e != nil || len(ledger.Leases) != 0 {
		t.Fatal("verified dependent lease not settled", e)
	}
}
func TestManagedModelCloseDoesNotPromoteDestroyAcknowledgmentToAbsence(t *testing.T) {
	for _, status := range []registry.Status{registry.StatusRunning, registry.StatusExited} {
		t.Run(string(status), func(t *testing.T) {
			env, be, row := managedSessionFixture(t, status)
			be.keepAfterDestroy = true
			if e := env.api.ManagedModelDependents(context.Background(), row.ManagedLocalModel.Boot, "7", true); e == nil {
				t.Fatal("present exact CID was treated as removed")
			}
			current, _ := env.st.Get(row.ID)
			if current.Status != status {
				t.Fatal("unconfirmed removal changed terminal state")
			}
			ledger, e := env.api.execution.readFenceLedger()
			if e != nil || len(ledger.Leases) != 1 {
				t.Fatal("unknown cleanup released lease", e)
			}
		})
	}
}
func TestManagedModelCloseRefusesChangedLeaseGeneration(t *testing.T) {
	env, be, row := managedSessionFixture(t, registry.StatusRunning)
	if e := env.api.execution.mutate(context.Background(), func(l *executionLeaseLedger) error { l.Leases[0].ManagedLocalModel.Generation = "8"; return nil }); e != nil {
		t.Fatal(e)
	}
	if e := env.api.ManagedModelDependents(context.Background(), row.ManagedLocalModel.Boot, "7", true); e == nil || len(be.destroys) != 0 {
		t.Fatal("changed allocation lease was stopped")
	}
}
