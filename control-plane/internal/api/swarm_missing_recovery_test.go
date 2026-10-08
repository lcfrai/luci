package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type missingChildBackend struct {
	backend.Backend
	getError error
	getCalls int
	lastID   string
}

func (b *missingChildBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	b.getCalls++
	b.lastID = id
	return backend.Sandbox{ID: id, State: backend.StateRunning}, b.getError
}

func missingChildFixture(t *testing.T) (*swarmSessionFixture, string, *missingChildBackend) {
	t.Helper()
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	probe := &missingChildBackend{Backend: f.env.be, getError: backend.ErrNotFound}
	a.deps.Backend = probe
	op := f.ops[0]
	route := f.run.Input.Route
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	policy, e := a.deps.Product.SwarmSchedulerPolicy()
	if e != nil {
		t.Fatal(e)
	}
	leaseID := "manual-missing-claim"
	if d, e := a.execution.reserve(context.Background(), leaseID, f.owner.ID, route.Connection.ID, policy.MaxActive, actor, management.ExecutionAuthority{}, policy); e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	candidate := registry.Session{ProjectID: f.owner.ID, WorktreeID: op.WorktreeID, Harness: route.Harness, Model: route.Model, Effort: route.Effort, PermissionMode: route.PermissionMode, AgentConnectionID: route.Connection.ID, AgentConnectionRevision: route.Connection.Revision, AgentConnectionKind: route.Connection.Kind, AgentConnectionProtocol: route.Connection.Protocol, AuthMode: route.Connection.AuthMode, ImplementationContext: op.Implementation, ProjectRunID: f.run.ID, ProjectRunOperationID: op.ID}
	admission, e := a.deps.Product.BeginProjectRunSession(f.owner.ID, f.run.ID, op.ID, candidate, f.store.Get)
	if e != nil {
		t.Fatal(e)
	}
	if e := a.execution.bind(context.Background(), leaseID, op.ChildID); e != nil {
		admission.Release()
		t.Fatal(e)
	}
	admission.Release() // Lost controller after durable claim/bind, before Store.Create.
	f.run, e = a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if e != nil {
		t.Fatal(e)
	}
	return f, leaseID, probe
}
func missingChildRequest(f *swarmSessionFixture) product.ProjectSwarmRecoveryRequest {
	op := f.run.Operations[0]
	return product.ProjectSwarmRecoveryRequest{ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: op.ID, ChildID: op.ChildID, UpdatedAt: op.UpdatedAt}
}
func missingChildRecover(t *testing.T, f *swarmSessionFixture, expected int) {
	t.Helper()
	op := f.ops[0]
	body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "operationId": op.ID, "childId": op.ChildID, "recoverMissingChild": true})
	w := f.env.do(t, http.MethodPost, f.base()+"/"+f.run.ID+"/swarm/tasks/api/abandon", string(body))
	if w.Code != expected {
		t.Fatalf("retire missing claim %d want%d: %s", w.Code, expected, w.Body.String())
	}
	var e error
	f.run, e = f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if e != nil {
		t.Fatal(e)
	}
}
func TestSwarmMissingChildRetirementPreservesOutputAndFencesExactLateAllocator(t *testing.T) {
	f, leaseID, probe := missingChildFixture(t)
	op := f.ops[0]
	a := f.env.api
	path, e := a.deps.Worktrees.Path(context.Background(), op.WorktreeID)
	if e != nil {
		t.Fatal(e)
	}
	retained := []byte("Retained uncommitted work from before the interrupted launch.\n")
	if e = os.WriteFile(filepath.Join(path, "retained.txt"), retained, 0600); e != nil {
		t.Fatal(e)
	}
	missingChildRecover(t, f, 200)
	if probe.getCalls < 2 || probe.lastID != op.ChildID || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("recovery did not independently check exact backend absence", probe.getCalls, probe.lastID)
	}
	if data, e := os.ReadFile(filepath.Join(path, "retained.txt")); e != nil || !bytes.Equal(data, retained) {
		t.Fatal("recovery removed retained task work", e)
	}
	if e = a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 0 || len(l.Retirements) != 1 || l.Retirements[0].ChildID != op.ChildID {
			t.Fatal("missing durable exact retirement", l)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if e = a.execution.markBackendStarting(context.Background(), leaseID, op.ChildID, op.ChildID); e == nil {
		t.Fatal("late allocator crossed retired launch edge")
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	policy, _ := a.deps.Product.SwarmSchedulerPolicy()
	if _, e = a.execution.reserve(context.Background(), leaseID, f.owner.ID, f.run.Input.Route.Connection.ID, policy.MaxActive, actor, management.ExecutionAuthority{}, policy); e == nil {
		t.Fatal("same retired lease reacquired capacity")
	}
	// Explicit retry and process reopen recover the receipt; neither starts work.
	missingChildRecover(t, f, 200)
	f.reopen(t)
	missingChildRecover(t, f, 200)
	body, _ := json.Marshal(f.requests[0])
	w := f.env.do(t, http.MethodPost, "/v1/sessions", string(body))
	if w.Code < 400 || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("retired operation replayed a child", w.Code, w.Body.String())
	}
}
func TestSwarmMissingChildRecoveryRefusesAmbiguousProofs(t *testing.T) {
	for _, name := range []string{"legacy guard", "backend already attempted", "registry child exists", "backend transport unavailable", "backend child exists", "inventory sees exact child", "stale inventory", "incomplete inventory", "current editor revoked"} {
		t.Run(name, func(t *testing.T) {
			f, leaseID, probe := missingChildFixture(t)
			a := f.env.api
			op := f.ops[0]
			switch name {
			case "legacy guard":
				if e := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error { l.Leases[0].LaunchGuard = 0; return nil }); e != nil {
					t.Fatal(e)
				}
			case "backend already attempted":
				if e := a.execution.markBackendStarting(context.Background(), leaseID, op.ChildID, op.ChildID); e != nil {
					t.Fatal(e)
				}
			case "registry child exists":
				if e := f.store.Create(registry.Session{ID: op.ChildID, SandboxID: op.ChildID, Status: registry.StatusStarting}); e != nil {
					t.Fatal(e)
				}
			case "backend transport unavailable":
				probe.getError = errors.Join(backend.ErrNotFound, backend.ErrUnavailable)
			case "backend child exists":
				probe.getError = nil
			case "inventory sees exact child":
				a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
					s := executionFixtureSnapshot()
					s.Execution.Containers = []resourcebudget.ContainerUsage{{ID: "actual-container", SandboxID: op.ChildID, Owned: true, Kind: "agent"}}
					return s
				}
			case "stale inventory":
				a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
					s := executionFixtureSnapshot()
					s.Execution.ObservedAt = time.Now().Add(-time.Minute)
					return s
				}
			case "incomplete inventory":
				a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
					s := executionFixtureSnapshot()
					s.Execution.InventoryComplete = false
					return s
				}
			case "current editor revoked":
				a.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error { return management.ErrExecutionAuthority }
			}
			missingChildRecover(t, f, 409)
			if f.run.Operations[0].AbsenceRetirement != nil || f.run.Operations[0].Status == "cancelled" {
				t.Fatal("ambiguous launch was tombstoned")
			}
			if e := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
				if len(l.Leases) != 1 || len(l.Retirements) != 0 {
					t.Fatal("unproved absence released capacity", l)
				}
				return nil
			}); e != nil {
				t.Fatal(e)
			}
			if controlledAllocationCount(f.env.be) != 0 {
				t.Fatal("recovery allocated replacement")
			}
		})
	}
}
func TestSwarmMissingChildFinalizationRequiresProductTombstone(t *testing.T) {
	f, _, _ := missingChildFixture(t)
	req := missingChildRequest(f)
	proof, e := f.env.api.proveSwarmMissingChild(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.env.api.finalizeSwarmMissingChild(context.Background(), req, proof); e == nil {
		t.Fatal("released capacity before durable operation retirement")
	}
}

func TestSwarmMissingChildLatePreparedAllocatorCannotReachBackend(t *testing.T) {
	f, leaseID, _ := missingChildFixture(t)
	op := f.ops[0]
	missingChildRecover(t, f, 200)
	candidate := registry.Session{ID: op.ChildID, SandboxID: op.ChildID, Status: registry.StatusStarting, ProjectID: f.owner.ID}
	_, e := f.env.api.allocatePreparedSession(context.Background(), preparedSessionAllocation{Record: candidate, Spec: backend.Spec{ID: op.ChildID, SessionID: op.ChildID}, BeforeBackendStart: func(ctx context.Context) error {
		return f.env.api.execution.markBackendStarting(ctx, leaseID, op.ChildID, op.ChildID)
	}})
	if e == nil || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("delayed prepared allocator reached backend after retirement", e)
	}
	row, e := f.store.Get(op.ChildID)
	if e != nil || row.Status != registry.StatusExited {
		t.Fatal("fenced late allocator left a running registry witness", row, e)
	}
}
func TestSwarmMissingChildRetirementRetryCompletesAfterFinalizationRevocation(t *testing.T) {
	f, _, _ := missingChildFixture(t)
	a := f.env.api
	calls := 0
	revoked := false
	a.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
		calls++
		if calls > 1 && revoked {
			return management.ErrExecutionAuthority
		}
		return nil
	}
	revoked = true
	missingChildRecover(t, f, 409)
	if f.run.Operations[0].AbsenceRetirement == nil || f.run.Operations[0].Status != "cancelled" {
		t.Fatal("failed finalize lost durable no-replay tombstone")
	}
	if e := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 1 || len(l.Retirements) != 0 {
			t.Fatal("revoked finalization released capacity")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	revoked = false
	missingChildRecover(t, f, 200)
	if e := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 0 || len(l.Retirements) != 1 {
			t.Fatal("explicit retry did not finalize exact prior proof")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("retirement retry allocated")
	}
}
func TestExecutionBackendStartMarkerCannotBeDowngraded(t *testing.T) {
	f, leaseID, _ := missingChildFixture(t)
	op := f.ops[0]
	g := f.env.api.execution
	if e := g.markBackendStarting(context.Background(), leaseID, op.ChildID, op.ChildID); e != nil {
		t.Fatal(e)
	}
	if e := g.bindSandbox(context.Background(), leaseID, op.ChildID, op.ChildID); e == nil {
		t.Fatal("backend marker downgraded to pre-launch proof")
	}
	if _, e := f.env.api.proveSwarmMissingChild(context.Background(), missingChildRequest(f)); e == nil {
		t.Fatal("attempted backend launch became recoverable as never-started")
	}
}

func TestSwarmMissingChildRecoveryWaitsForActualAllocatorThenRefusesLiveChild(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	sessionsStubReady(t, nil)
	watched := sessionsWatchDone(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	f.env.be.onCreate = func(spec backend.Spec) { close(entered); <-release }
	var proofs atomic.Int32
	a.deps.Product.SetProjectSwarmRecovery(func(ctx context.Context, req product.ProjectSwarmRecoveryRequest) (product.ProjectSwarmRecoveryProof, error) {
		proofs.Add(1)
		return a.proveSwarmMissingChild(ctx, req)
	}, a.finalizeSwarmMissingChild)
	creation := make(chan *httptest.ResponseRecorder, 1)
	body, _ := json.Marshal(f.requests[0])
	go func() { creation <- f.env.do(t, http.MethodPost, "/v1/sessions", string(body)) }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("controlled allocator did not reach Backend.Create")
	}
	run, e := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if e != nil {
		t.Fatal(e)
	}
	op := f.ops[0]
	request, _ := json.Marshal(map[string]any{"expectedRevision": run.Revision, "operationId": op.ID, "childId": op.ChildID, "recoverMissingChild": true})
	recovery := make(chan *httptest.ResponseRecorder, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		recovery <- f.env.do(t, http.MethodPost, f.base()+"/"+f.run.ID+"/swarm/tasks/api/abandon", string(request))
	}()
	<-started
	select {
	case got := <-recovery:
		t.Fatalf("recovery passed an in-flight allocator: %d %s", got.Code, got.Body.String())
	case <-time.After(100 * time.Millisecond):
	}
	if proofs.Load() != 0 {
		t.Fatal("backend absence was probed while allocator owned dispatch admission")
	}
	unblock()
	select {
	case got := <-creation:
		if got.Code != 201 {
			t.Fatal("original allocator failed", got.Code, got.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("allocator did not complete")
	}
	select {
	case got := <-recovery:
		if got.Code != 409 {
			t.Fatal("registered live child was retired", got.Code, got.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("recovery did not finish after allocator")
	}
	sessionsAwaitWatch(t, watched)
	row, e := f.store.Get(op.ChildID)
	if e != nil || row.Status != registry.StatusRunning {
		t.Fatal("recovery changed original running session", row, e)
	}
	if controlledAllocationCount(f.env.be) != 1 || f.env.be.destroyCount() != 0 {
		t.Fatal("recovery replayed or destroyed original allocation")
	}
	if e := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 1 || l.Leases[0].SessionID != op.ChildID || l.Leases[0].State != "backend-dispatching" || len(l.Retirements) != 0 {
			t.Fatal("active child capacity was released", l)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
