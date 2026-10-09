package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// These tests exercise the typed allocator against reviewed Git/context inputs.
// The backend is controlled; they do not claim provider/model or Docker proof.
func TestQueuedDispatchRechecksFreshAuthorityAfterDurableClaim(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	calls := 0
	a.deps.AuthorizeExecution = func(ctx context.Context, got management.ExecutionActorRef, harness string, secrets bool) (management.ExecutionAuthority, error) {
		if got != actor || harness != "codex" || !secrets {
			t.Error("queued identity/route authority changed", got, harness, secrets)
		}
		calls++
		if calls >= 3 {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{}, nil
	}
	policy, e := a.deps.Product.SwarmSchedulerPolicy()
	if e != nil {
		t.Fatal(e)
	}
	admission, e := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-authority", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
	if e != nil || admission.State != "admitted" {
		t.Fatal(admission, e)
	}
	got, e := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-authority", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
	if e != nil || got.State != "failed" || got.ReasonCode != "execution_authority_changed" || got.SessionID == "" {
		t.Fatal("pre-allocation refusal lost its exact diagnostic", got, e)
	}
	if controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("revoked authority allocated a backend")
	}
	row, e := f.store.Get(got.SessionID)
	if e != nil || row.Status != registry.StatusExited || row.StartFailure != registry.NewSessionStartFailure("admission", "execution_authority_changed") {
		t.Fatal("failed final authorization left live registry state", row, e)
	}
	// A later retry must not recreate the exact claimed operation.
	calls = 0
	repeated, e := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-authority", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
	if e != nil || repeated != got {
		t.Fatal("duplicate dispatch changed the recorded failure", repeated, e)
	}
	if controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("failed claimed operation replayed a backend")
	}
}

func TestQueuedAdmissionRefusesRevokedActorBeforeCapacityOrAllocation(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	a := f.env.api
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, errors.New("controlled-secret/private/authority")
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	policy, e := a.deps.Product.SwarmSchedulerPolicy()
	if e != nil {
		t.Fatal(e)
	}
	got, e := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-revoked", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
	if e != nil || got.State != "blocked" || got.ReasonCode != "access_changed" || strings.Contains(got.Message, "controlled-secret") || strings.Contains(got.Message, "/private/") {
		t.Fatal(got, e)
	}
	if a.execution.snapshot.Load() != nil || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("revoked queue actor acquired capacity or allocated backend")
	}
}

func controlledAllocationCount(b *sessionsBackend) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.specs)
}
