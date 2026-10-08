package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// The wrapper changes only the external admission observation, after the real
// registry has durably recorded the exact child. It does not fabricate task
// status, change a private lease, or bypass any allocator callback.
type queuedStartFailureStore struct {
	*registry.FileStore
	afterCreate func()
}

type queuedNeverAllocatedBackend struct {
	backend.Backend
	controlled *sessionsBackend
	probes     atomic.Int64
}

func (b *queuedNeverAllocatedBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	b.probes.Add(1)
	if err := ctx.Err(); err != nil {
		return backend.Sandbox{}, err
	}
	if controlledAllocationCount(b.controlled) == 0 {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	return b.Backend.Get(ctx, id)
}

func (s *queuedStartFailureStore) Create(row registry.Session) error {
	if err := s.FileStore.Create(row); err != nil {
		return err
	}
	s.afterCreate()
	return nil
}

func TestQueuedStartFailurePreservesFinalAdmissionReasonAcrossRestart(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		held       bool
	}{
		{"authority-revoked", "execution_authority_changed", false},
		{"observation-expired", "resource_observation_unavailable", false},
		{"memory-pressure", "memory_pressure", false},
		{"memory-pressure-backend-not-confirmed-absent", "memory_pressure", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			a := f.env.api
			// Quiesce the initially empty controller before changing fixture
			// dependencies; the actual queued runtime is restored below.
			a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{})
			if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			var created atomic.Bool
			var createCalls, initialAuthority, finalAuthority, initialSnapshot, finalSnapshot atomic.Int64
			a.deps.Store = &queuedStartFailureStore{FileStore: f.store, afterCreate: func() {
				createCalls.Add(1)
				created.Store(true)
			}}
			a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
			absence := &queuedNeverAllocatedBackend{Backend: f.env.be, controlled: f.env.be}
			if !tc.held {
				a.deps.Backend = absence
			}
			const privateCause = "controlled-private-credential at /private/controlled-authority-file"
			a.deps.AuthorizeExecution = func(ctx context.Context, actor management.ExecutionActorRef, harness string, secrets bool) (management.ExecutionAuthority, error) {
				if actor.ProjectID != f.owner.ID || harness != "codex" || !secrets {
					t.Error("queued authority scope changed", actor.ProjectID, harness, secrets)
				}
				if created.Load() {
					finalAuthority.Add(1)
					if tc.name == "authority-revoked" {
						return management.ExecutionAuthority{}, errors.New(privateCause)
					}
				} else {
					initialAuthority.Add(1)
				}
				return management.ExecutionAuthority{}, nil
			}
			a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				snapshot := executionFixtureSnapshot()
				if created.Load() {
					finalSnapshot.Add(1)
					switch tc.name {
					case "observation-expired":
						snapshot.Host.ObservedAt = time.Now().Add(-10 * time.Second)
					case "memory-pressure", "memory-pressure-backend-not-confirmed-absent":
						snapshot.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(512) << 20)
					}
				} else {
					initialSnapshot.Add(1)
				}
				return snapshot
			}
			startQueuedRuntime := func(a *API) {
				a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{
					CaptureActor: a.captureQueuedActor, Admit: a.admitQueuedSwarm,
					Dispatch: a.dispatchQueuedSwarm, Release: a.execution.release, Resources: a.queuedResourceView,
				})
			}
			startQueuedRuntime(a)
			turn := func(a *API) {
				t.Helper()
				err := a.deps.Product.RunSwarmSchedulerOnce(context.Background())
				if err != nil && !(tc.held && errors.Is(err, ErrExecutionLeaseHeld)) {
					t.Fatal(err)
				}
			}
			body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "idempotencyKey": "controlled-final-admission-" + tc.name, "taskIds": []string{"api"}, "priority": "normal"})
			w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/queue", string(body))
			var queued struct {
				Items []product.SwarmQueueItem `json:"items"`
			}
			if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &queued) != nil || len(queued.Items) != 1 {
				t.Fatal("queue consent failed", w.Code, w.Body.String())
			}
			// The same serialized production turn is used by the background wake.
			// A second turn records terminal presentation after dispatch refusal;
			// neither turn calls a private test-only reconciliation shortcut.
			for range 2 {
				turn(a)
			}
			a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{})
			if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if createCalls.Load() != 1 || initialAuthority.Load() < 2 || finalAuthority.Load() < 1 || initialSnapshot.Load() < 1 {
				t.Fatal("fixture did not reach final admission after earlier success", createCalls.Load(), initialAuthority.Load(), finalAuthority.Load(), initialSnapshot.Load())
			}
			if tc.name != "authority-revoked" && finalSnapshot.Load() < 1 {
				t.Fatal("final capacity observation was not consulted")
			}
			if controlledAllocationCount(f.env.be) != 0 {
				t.Fatal("refused final admission invoked Backend.Create")
			}
			rows, err := f.store.List()
			if err != nil || len(rows) != 1 {
				t.Fatal("exact failed child not retained", len(rows), err)
			}
			child := rows[0]
			want := registry.NewSessionStartFailure("admission", tc.code)
			if child.Status != registry.StatusExited || child.SandboxRef != "" || child.StartFailure != want || !child.StartFailure.Valid() {
				t.Fatalf("failed child lost safe stage/reason: status=%s ref=%s diagnostic=%+v", child.Status, child.SandboxRef, child.StartFailure)
			}
			assertReads := func() {
				t.Helper()
				var publicRun struct {
					Run product.ProjectRun `json:"run"`
				}
				w := f.env.do(t, "GET", f.base()+"/"+f.run.ID, "")
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &publicRun) != nil || len(publicRun.Run.Operations) != 1 {
					t.Fatal("retained run unreadable", w.Code, w.Body.String())
				}
				op := publicRun.Run.Operations[0]
				if op.ChildID != child.ID || op.Status != "failed" || op.StartFailure != want || op.Handoff != nil || len(publicRun.Run.Checks) != 0 || publicRun.Run.Acceptance != nil {
					t.Fatal("failed start inferred an agent result or lost exact diagnosis", op.Status, op.StartFailure)
				}
				path, err := f.env.api.deps.Worktrees.Path(context.Background(), op.WorktreeID)
				if err != nil {
					t.Fatal("failed checkout not retained", err)
				}
				if info, err := os.Stat(path); err != nil || !info.IsDir() {
					t.Fatal("retained failed checkout disappeared", err)
				}
				var queue product.ProjectSwarmQueue
				w = f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/swarm-queue", "")
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &queue) != nil || len(queue.Items) != 1 {
					t.Fatal("failed queue unreadable", w.Code, w.Body.String())
				}
				item := queue.Items[0]
				message := want.Message
				if tc.held {
					message += " Its execution reservation remains held pending confirmed release."
				}
				if item.ID != queued.Items[0].ID || item.ChildID != child.ID || item.OperationID != op.ID || item.State != "failed" || item.ReasonCode != tc.code || item.Reason != message || queue.Counts.Failed != 1 || queue.Counts.Completed != 0 {
					t.Fatal("queue called a failed start completed/result-ready", item, queue.Counts)
				}
				var home product.ProjectOverview
				w = f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/overview", "")
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &home) != nil || home.Execution.Scope != "swarm-queue" || home.Execution.Failed == nil || *home.Execution.Failed != 1 || home.Execution.Running == nil || *home.Execution.Running != 0 || home.LatestRun.ID != f.run.ID || home.LatestRun.State != "blocked" || home.Attention.Count == nil || *home.Attention.Count != 1 || len(home.Attention.Items) != 1 || home.Attention.Items[0].Kind != "blocked" {
					t.Fatal("Home hid/misclassified the failed-start blocker", w.Code, w.Body.String())
				}
				ledger, err := f.env.api.execution.readFenceLedger()
				if err != nil {
					t.Fatal(err)
				}
				if tc.held {
					if item.MemoryMiB == 0 || item.CPUMillis == 0 || len(ledger.Leases) != 1 || ledger.Leases[0].ID != item.ID || ledger.Leases[0].SessionID != child.ID || ledger.Leases[0].State != "allocating" {
						t.Fatal("diagnostic was used to release unconfirmed execution capacity", item.MemoryMiB, len(ledger.Leases))
					}
				} else if item.MemoryMiB != 0 || item.CPUMillis != 0 || len(ledger.Leases) != 0 || absence.probes.Load() == 0 {
					t.Fatal("exact absent backend was not independently confirmed before release", item.MemoryMiB, len(ledger.Leases), absence.probes.Load())
				}
				var publicSession struct {
					ID           string                       `json:"id"`
					StartFailure registry.SessionStartFailure `json:"startFailure"`
				}
				w = f.env.do(t, "GET", "/v1/sessions/"+child.ID, "")
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &publicSession) != nil || publicSession.ID != child.ID || publicSession.StartFailure != want {
					t.Fatal("session did not expose safe diagnostic", w.Code, w.Body.String())
				}
				for _, route := range []string{"/v1/sessions", f.base(), f.base() + "/" + f.run.ID, "/v1/product/projects/" + f.owner.ID + "/swarm-queue"} {
					response := f.env.do(t, "GET", route, "")
					if response.Code != 200 || strings.Contains(response.Body.String(), "controlled-private-credential") || strings.Contains(response.Body.String(), "/private/controlled-authority-file") {
						t.Fatal("public failure projection leaked the underlying error or became unreadable", route, response.Code)
					}
				}
			}
			assertReads()
			f.reopen(t)
			assertReads()
			// Recovery of machine capacity/access does not authorize replay of
			// this claimed attempt. The queue stays terminal with its original
			// diagnostic; the operator must explicitly review a new attempt.
			created.Store(false)
			for range 2 {
				turn(f.env.api)
			}
			if controlledAllocationCount(f.env.be) != 0 {
				t.Fatal("reopening/restored admission replayed the failed attempt")
			}
			assertReads()
		})
	}
}

func TestManualTaskStartFailureIsObservedWithoutQueueOrReconcile(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{})
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var created atomic.Bool
	var checks atomic.Int64
	a.deps.Store = &queuedStartFailureStore{FileStore: f.store, afterCreate: func() { created.Store(true) }}
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Backend = &queuedNeverAllocatedBackend{Backend: f.env.be, controlled: f.env.be}
	a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
		checks.Add(1)
		if created.Load() {
			return management.ExecutionAuthority{}, errors.New("controlled-manual-private-cause")
		}
		return management.ExecutionAuthority{}, nil
	}
	body, _ := json.Marshal(f.requests[0])
	w := f.env.do(t, "POST", "/v1/sessions", string(body))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "execution_authority_changed") || strings.Contains(w.Body.String(), "controlled-manual-private-cause") || !created.Load() || checks.Load() < 2 || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("manual final admission was not safely refused", w.Code, w.Body.String(), checks.Load())
	}
	want := registry.NewSessionStartFailure("admission", "execution_authority_changed")
	child, err := f.store.Get(f.ops[0].ChildID)
	if err != nil || child.Status != registry.StatusExited || child.StartFailure != want || child.TaskObservationComplete {
		t.Fatal("manual diagnostic was missing/already observed", child.StartFailure, child.TaskObservationComplete, err)
	}
	// Reopen the real registry and product to make the exited child, rather than
	// this HTTP response or a queued item, the source for service observation.
	f.reopen(t)
	a = f.env.api
	authorityBefore := checks.Load()
	if err := a.ObserveTaskSessionsOnce(context.Background()); err != nil {
		t.Fatal("manual diagnostic observation", err)
	}
	marked, err := f.store.Get(child.ID)
	if err != nil || !marked.TaskObservationComplete || marked.StartFailure != want || marked.ID != child.ID || marked.SandboxRef != "" || checks.Load() != authorityBefore {
		t.Fatal("observation lost exact generation or impersonated execution authority", marked.StartFailure, marked.TaskObservationComplete, err)
	}
	var runView struct {
		Run product.ProjectRun `json:"run"`
	}
	w = f.env.do(t, "GET", f.base()+"/"+f.run.ID, "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &runView) != nil || len(runView.Run.Operations) != 1 {
		t.Fatal("manual retained run unreadable", w.Code, w.Body.String())
	}
	op := runView.Run.Operations[0]
	if op.ID != f.ops[0].ID || op.ChildID != child.ID || op.Status != "failed" || op.StartFailure != want || op.Handoff != nil || runView.Run.State != "blocked" || len(runView.Run.Checks) != 0 || runView.Run.Acceptance != nil || runView.Run.Swarm.Tasks[0].Integration != nil {
		t.Fatal("manual refused start became output/verification", op.Status, op.StartFailure, runView.Run.State)
	}
	var home product.ProjectOverview
	w = f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/overview", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &home) != nil || home.LatestRun.ID != f.run.ID || home.LatestRun.State != "blocked" || home.Attention.Count == nil || *home.Attention.Count != 1 || len(home.Attention.Items) != 1 || home.Attention.Items[0].ID != f.run.ID || home.Attention.Items[0].Kind != "blocked" || home.NextAction.RunID != f.run.ID || home.NextAction.ID != "inspect-run" {
		t.Fatal("Home lost the exact manual blocked attempt", w.Code, w.Body.String())
	}
	// These counts describe the queue, not every manual session. The blocked
	// run attention above carries the manual failure without inventing a queue.
	if home.Execution.Scope != "swarm-queue" || home.Execution.Failed == nil || *home.Execution.Failed != 0 || home.Execution.Running == nil || *home.Execution.Running != 0 {
		t.Fatal("manual failure fabricated queued-work counts", home.Execution)
	}
	var queue product.ProjectSwarmQueue
	w = f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/swarm-queue", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &queue) != nil || len(queue.Items) != 0 {
		t.Fatal("manual observation invented a queued entry", w.Code, w.Body.String())
	}
	f.reopen(t)
	if err := f.env.api.ObserveTaskSessionsOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded, err := f.store.Get(child.ID)
	if err != nil || !reloaded.TaskObservationComplete || reloaded.StartFailure != want || controlledAllocationCount(f.env.be) != 0 || checks.Load() != authorityBefore {
		t.Fatal("reopened manual diagnostic replayed/lost observation", err)
	}
}
