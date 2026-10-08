package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func settledTaskFixture(t *testing.T, prepare ...func(*swarmSessionFixture)) (*swarmSessionFixture, registry.Session, executionLeaseLedger) {
	t.Helper()
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ReadyTimeout = 5 * time.Second
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	for _, setup := range prepare {
		setup(f)
	}
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, err := json.Marshal(f.requests[0])
	if err != nil {
		t.Fatal(err)
	}
	sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	row, err := f.store.Get(f.ops[0].ChildID)
	if err != nil || row.Status != registry.StatusRunning || row.SandboxRef == "" {
		t.Fatal("actual task launch did not settle", row, err)
	}
	data, err := os.ReadFile(a.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := decodeExecutionLedger(data)
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].State != "backend-settled" || ledger.Leases[0].SessionID != row.ID {
		t.Fatal("actual launch did not leave exact settled receipt", ledger, err)
	}
	return f, row, ledger
}

func TestConfirmStoppedTaskCleanupRequiresExactActualLaunchReceipt(t *testing.T) {
	f, row, baseline := settledTaskFixture(t)
	a := f.env.api
	if err := a.ConfirmStoppedTaskCleanup(context.Background(), row); err != nil {
		t.Fatal("actual settled task denied", err)
	}
	original, err := json.Marshal(baseline)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"missing ledger", "corrupt ledger", "no lease", "legacy guard", "prebackend", "pending create", "foreign scope", "foreign sandbox", "conflicting pending lease", "duplicate settled lease", "wrong operation", "wrong worktree", "changed request", "no ref", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			if err := writeExecutionLedger(a.execution.path, original); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := writeExecutionLedger(a.execution.path, original); err != nil {
					t.Fatal(err)
				}
			}()
			ledger, err := decodeExecutionLedger(original)
			if err != nil {
				t.Fatal(err)
			}
			candidate := row.Clone()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "no lease":
				ledger.Leases = nil
			case "legacy guard":
				ledger.Leases[0].LaunchGuard = 0
			case "prebackend":
				ledger.Leases[0].State = "allocating"
			case "pending create":
				ledger.Leases[0].State = "backend-dispatching"
			case "foreign scope":
				ledger.Leases[0].Scope = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			case "foreign sandbox":
				ledger.Leases[0].SandboxID = "sb-other"
			case "conflicting pending lease", "duplicate settled lease":
				other := ledger.Leases[0]
				other.ID += "-conflict"
				if scenario == "conflicting pending lease" {
					other.State = "backend-dispatching"
				}
				ledger.Leases = append(ledger.Leases, other)
			case "wrong operation":
				candidate.ProjectRunOperationID = "op-other"
			case "wrong worktree":
				candidate.WorktreeID = "w-other"
			case "changed request":
				candidate.Model += "-changed"
			case "no ref":
				candidate.SandboxRef = ""
			case "canceled":
				cancel()
			}
			data, err := json.Marshal(ledger)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "corrupt ledger" {
				data = []byte("{broken")
			}
			if err := writeExecutionLedger(a.execution.path, data); err != nil {
				t.Fatal(err)
			}
			if scenario == "missing ledger" {
				if err := os.Remove(a.execution.path); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.ConfirmStoppedTaskCleanup(ctx, candidate); err == nil || errors.Is(err, backend.ErrUnsupported) {
				t.Fatal("uncertain task received cleanup/fallback permission", err)
			}
		})
	}
	// Read-only cleanup observation must not queue behind the admission mutex.
	a.execution.mu.Lock()
	result := make(chan error, 1)
	go func() { result <- a.ConfirmStoppedTaskCleanup(context.Background(), row) }()
	select {
	case err := <-result:
		a.execution.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		a.execution.mu.Unlock()
		<-result
		t.Fatal("cleanup proof blocked on the admission mutex")
	}
}

func TestConfirmStoppedTaskCleanupOnlyIdentifiesExactSerialRunForLegacyFallback(t *testing.T) {
	env, _, op, _, candidate := reservedProjectRunSession(t, false)
	candidate.ID, candidate.SandboxID = op.ChildID, op.ChildID
	if err := env.api.ConfirmStoppedTaskCleanup(context.Background(), candidate); err != backend.ErrUnsupported {
		t.Fatal("exact non-task operation lost legacy cleanup classification", err)
	}
	candidate.Model += "-changed"
	if err := env.api.ConfirmStoppedTaskCleanup(context.Background(), candidate); err == nil || errors.Is(err, backend.ErrUnsupported) {
		t.Fatal("mismatched operation received legacy cleanup permission", err)
	}
}

type cleanupSnapshotStore struct {
	registry.Store
	captured, proceed chan struct{}
	getOnce           sync.Once
}

func (s *cleanupSnapshotStore) List() ([]registry.Session, error) {
	rows, err := s.Store.List()
	s.getOnce.Do(func() { close(s.captured); <-s.proceed })
	return rows, err
}

// The stale sweep and a real restart HTTP request overlap. The restart really
// destroys/recreates through the existing allocator and publishes a new token,
// container reference and readiness transition before the old snapshot resumes.
func TestObservedExitCannotOverwriteActualConcurrentHTTPRestart(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "stopped", true: "missing"}[missing], func(t *testing.T) {
			env := sessionsNewEnv(t)
			sessionsStubReady(t, nil)
			done := sessionsWatchDone(t)
			before := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
			oldBackend := &taskCleanupLifecycleBackend{Backend: env.be, absent: missing, sandbox: backend.Sandbox{ID: before.SandboxID, Ref: before.SandboxRef, Kind: backend.KindDocker, State: backend.StateStopped}}
			wrapped := &cleanupSnapshotStore{Store: env.st, captured: make(chan struct{}), proceed: make(chan struct{})}
			var release sync.Once
			unblock := func() { release.Do(func() { close(wrapped.proceed) }) }
			t.Cleanup(unblock)
			r := reaper.New(reaper.Config{Store: wrapped, Backend: oldBackend, IdleAfter: time.Hour, Now: func() time.Time { return time.Now().Add(time.Hour) }})
			sweep := make(chan error, 1)
			go func() { sweep <- r.Sweep(context.Background()) }()
			select {
			case <-wrapped.captured:
			case <-time.After(5 * time.Second):
				t.Fatal("sweep did not capture old allocation")
			}
			sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions/"+before.ID+"/restart", ""), 200)
			sessionsAwaitWatch(t, done)
			after, err := env.st.Get(before.ID)
			if err != nil || after.SandboxID == before.SandboxID || after.SandboxRef == before.SandboxRef || after.AttachToken == before.AttachToken || after.Status != registry.StatusRunning {
				t.Fatal("real HTTP restart failed to produce a new allocation", after, err)
			}
			unblock()
			select {
			case err := <-sweep:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("sweep did not finish")
			}
			current, err := env.st.Get(before.ID)
			if err != nil || current.Status == registry.StatusExited || current.SandboxID != after.SandboxID || current.SandboxRef != after.SandboxRef || current.AttachToken != after.AttachToken || current.Endpoint != after.Endpoint {
				t.Fatal("stale sweep changed restarted HTTP allocation", current, err)
			}
			if oldBackend.forceCalls != 0 || oldBackend.removes != 0 || env.be.destroyCount() != 1 || len(env.be.specs) != 1 {
				t.Fatal("sweep touched the restarted worker or repeated allocation")
			}
		})
	}
}

type taskCleanupLifecycleBackend struct {
	backend.Backend
	sandbox             backend.Sandbox
	removeErr           error
	absent              bool
	removes, forceCalls int
}

func (b *taskCleanupLifecycleBackend) List(context.Context) ([]backend.Sandbox, error) {
	if b.absent {
		return nil, nil
	}
	return []backend.Sandbox{b.sandbox}, nil
}
func (b *taskCleanupLifecycleBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	if b.absent || id != b.sandbox.ID {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	return b.sandbox, nil
}
func (b *taskCleanupLifecycleBackend) DestroyStopped(_ context.Context, observed backend.Sandbox) error {
	b.removes++
	if observed.Ref != b.sandbox.Ref || observed.ID != b.sandbox.ID {
		return errors.New("wrong immutable cleanup target")
	}
	if b.removeErr != nil {
		return b.removeErr
	}
	b.absent = true
	return nil
}
func (b *taskCleanupLifecycleBackend) Destroy(context.Context, string) error {
	b.forceCalls++
	return errors.New("unexpected force cleanup")
}

func TestStoppedTaskCleanupRetriesAfterFailedRemoveWithoutLosingSettledLease(t *testing.T) {
	var next *swarmSessionFixture
	policy := executionFixturePolicy()
	policy.MaxActive, policy.MaxPerProject = 1, 1
	f, row, ledger := settledTaskFixture(t, func(f *swarmSessionFixture) {
		next = f.anotherProject(t, "cleanup-next-project", "api")
		var settings struct {
			Revision int `json:"revision"`
		}
		w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
			t.Fatal("read shared scheduling policy", w.Code)
		}
		body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": policy, "dispatchPaused": true})
		if w := f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
			t.Fatal("save single shared slot", w.Code, w.Body.String())
		}
	})
	a := f.env.api
	const instance = "11111111-1111-4111-8111-111111111111"
	b := &taskCleanupLifecycleBackend{Backend: a.deps.Backend, sandbox: backend.Sandbox{
		ID: row.SandboxID, Ref: row.SandboxRef, Kind: backend.KindDocker, State: backend.StateStopped, Instance: instance,
		Labels: map[string]string{backend.LabelManaged: "true", backend.LabelInstance: instance, backend.LabelSandboxID: row.SandboxID, backend.LabelSessionID: row.ID},
	}, removeErr: errors.New("controlled transient engine remove failure")}
	a.deps.Backend = b
	newReaper := func() *reaper.Reaper {
		return reaper.New(reaper.Config{Store: f.store, Backend: b, Instance: instance, ConfirmStoppedTaskCleanup: a.ConfirmStoppedTaskCleanup})
	}
	if err := newReaper().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.removes != 1 || b.absent || b.forceCalls != 0 {
		t.Fatal("first exact removal was not attempted", b)
	}
	exited, err := f.store.Get(row.ID)
	if err != nil || exited.Status != registry.StatusExited {
		t.Fatal("confirmed stopped process was not recorded", exited, err)
	}
	if busy, err := a.observeSwarmWorktree(context.Background(), row.ProjectID, row.WorktreeID); err != nil || busy {
		t.Fatal("stopped task should still be reviewable", busy, err)
	}
	if err := a.execution.release(context.Background(), ledger.Leases[0].ID); !errors.Is(err, ErrExecutionLeaseHeld) {
		t.Fatal("stopped-but-present task discarded cleanup proof", err)
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: next.owner.ID}
	decision, err := a.execution.reserve(context.Background(), "probe-next-project", next.owner.ID, "controlled-capacity-probe", 1, actor, management.ExecutionAuthority{}, policy)
	if err != nil || decision.State != "waiting" || decision.Code != "global_slot_limit" {
		t.Fatal("failed cleanup allowed another project's allocation", decision, err)
	}
	// A new observer and newly read on-disk admission ledger must preserve the
	// retry; success does not depend on an in-memory proof from the first sweep.
	a.execution = newExecutionAdmission(a)
	if err := a.ConfirmStoppedTaskCleanup(context.Background(), exited); err != nil {
		t.Fatal("retained proof could not be recovered", err)
	}
	b.removeErr = nil
	if err := newReaper().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.removes != 2 || !b.absent || b.forceCalls != 0 {
		t.Fatal("retry did not remove exact stopped allocation", b)
	}
	// Manual task launches do not have a scheduler Release callback. The next
	// actual reviewed HTTP launch must reclaim proven-absent capacity during
	// admission, without a stop, Forget or explicit lease release in between.
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(next.requests[0])
	sessionsDecodeOne(t, next.env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	if len(f.env.be.specs) != 2 {
		t.Fatal("next project did not allocate exactly once after removal")
	}
	if err := a.ConfirmStoppedTaskCleanup(context.Background(), exited); err == nil {
		t.Fatal("missing lease became positive proof")
	}
}
