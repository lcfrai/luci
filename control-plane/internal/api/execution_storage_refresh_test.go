package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func storageRefreshSnapshot(age time.Duration) resourcebudget.Snapshot {
	s := executionFixtureSnapshot()
	s.Host.Platform = "windows"
	s.Execution.EngineID = "controlled-engine"
	s.Execution.DaemonDisk.ObservedAt = time.Now().Add(-age)
	return s
}

func storageRefreshGate(t *testing.T) *executionAdmission {
	t.Helper()
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	pd, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pd.Close)
	g.api.deps.Product = pd
	return g
}

func TestWorkerStorageRefreshRequiresNewSampleAndPreflightMargin(t *testing.T) {
	g := storageRefreshGate(t)
	initial := storageRefreshSnapshot(4900 * time.Millisecond)
	initialAt := initial.Execution.DaemonDisk.ObservedAt
	nearExpiry := storageRefreshSnapshot(3 * time.Second)
	fresh := storageRefreshSnapshot(0)
	reads := 0
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		reads++
		switch reads {
		case 1, 2:
			return initial // The same cached timestamp is never a refresh.
		case 3:
			return nearExpiry // Newer, but lacks the required preparation margin.
		default:
			fresh = storageRefreshSnapshot(0)
			return fresh
		}
	}
	if err := g.api.awaitWorkerStorageRefresh(context.Background()); err != nil || reads != 4 {
		t.Fatal("near-expiry sample bypassed the bounded fresh publication wait", reads, err)
	}
	if initial.Execution.DaemonDisk.ObservedAt != initialAt || workerStorageRefreshActive.Load() {
		t.Fatal("readiness rewrote evidence or retained its wait slot")
	}
	policy, err := g.api.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		t.Fatal(err)
	}
	// Model two seconds of normal preparation without sleeping or changing
	// sample timestamps. Only the unchanged five-second evaluator admits it.
	at := time.Now().Add(2 * time.Second)
	if code, _ := evaluateExecutionCapacity(fresh, policy, g.api.sessionResources(context.Background()), executionCapacityDemand{}, at); code != "" {
		t.Fatal("fresh sample lost its actual preparation margin", code)
	}
	if !fresh.Execution.DaemonDisk.ObservedAt.Before(at) {
		t.Fatal("test did not preserve original observation time")
	}
	if controlledAllocationCount(g.api.deps.Backend.(*sessionsBackend)) != 0 || g.snapshot.Load() != nil {
		t.Fatal("readiness wait reserved or allocated work")
	}
}

func TestWorkerStorageRefreshStopsForPressureAndAdmissionRechecks(t *testing.T) {
	for _, pressureDuringWait := range []bool{true, false} {
		t.Run(map[bool]string{true: "pressure during wait", false: "pressure after fresh publication"}[pressureDuringWait], func(t *testing.T) {
			g := storageRefreshGate(t)
			reads := 0
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				reads++
				if reads == 1 {
					return storageRefreshSnapshot(6 * time.Second)
				}
				s := storageRefreshSnapshot(0)
				if pressureDuringWait || reads > 2 {
					s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(1 << 20))
				}
				return s
			}
			if err := g.api.awaitWorkerStorageRefresh(context.Background()); err != nil || reads != 2 {
				t.Fatal("readiness concealed current memory pressure", reads, err)
			}
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
			d, err := g.reserve(context.Background(), "fresh-but-pressured", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
			if err != nil || d.State != "waiting" || d.Code != "memory_pressure" {
				t.Fatal("freshness wait replaced atomic capacity admission", d, err)
			}
			assertExecutionLeaseCount(t, g, 0)
			if controlledAllocationCount(g.api.deps.Backend.(*sessionsBackend)) != 0 || workerStorageRefreshActive.Load() {
				t.Fatal("pressure created work or retained the wait slot")
			}
		})
	}
}

func TestWorkerStorageRefreshUnknownConfirmationAndFutureRemainImmediate(t *testing.T) {
	for _, kind := range []string{"missing", "future", "zero", "unconfirmed", "expired-host", "shared-unknown", "memory-pressure", "full-disk", "other-engine"} {
		t.Run(kind, func(t *testing.T) {
			g := storageRefreshGate(t)
			reads := 0
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				reads++
				s := storageRefreshSnapshot(6 * time.Second)
				switch kind {
				case "missing":
					s.Execution.DaemonDisk = nil
				case "future":
					s.Execution.DaemonDisk.ObservedAt = time.Now().Add(time.Minute)
				case "zero":
					s.Execution.DaemonDisk.ObservedAt = time.Time{}
				case "unconfirmed":
					s.Execution.HostStorage.State = "confirmation-required"
				case "expired-host":
					s.Execution.HostStorage.Disk.ObservedAt = time.Now().Add(-time.Minute)
				case "shared-unknown":
					s.Execution.DaemonDisk = nil
					s.Execution.HostStorage = &resourcebudget.HostStorageBound{Required: true, State: "unavailable", ReasonCode: "shared_observation_unavailable"}
				case "memory-pressure":
					s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(1))
				case "full-disk":
					s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(1))
				case "other-engine":
					s.Execution.EngineID = ""
				}
				return s
			}
			if err := g.api.awaitWorkerStorageRefresh(context.Background()); err != nil || reads != 1 || workerStorageRefreshActive.Load() {
				t.Fatal("unverified or pressured capacity entered a refresh wait", reads, err)
			}
			if g.snapshot.Load() != nil || controlledAllocationCount(g.api.deps.Backend.(*sessionsBackend)) != 0 {
				t.Fatal("unverified readiness reserved or created work")
			}
		})
	}
}

func TestWorkerStorageRefreshCancelledAndOccupiedSlotDoNotAccumulateWaiters(t *testing.T) {
	g := storageRefreshGate(t)
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return storageRefreshSnapshot(6 * time.Second) }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := g.api.awaitWorkerStorageRefresh(ctx); !errors.Is(err, context.DeadlineExceeded) || workerStorageRefreshActive.Load() {
		t.Fatal("cancelled wait retained its slot", err)
	}
	if !workerStorageRefreshActive.CompareAndSwap(false, true) {
		t.Fatal("cancelled wait blocked the next caller")
	}
	defer workerStorageRefreshActive.Store(false)
	reads := 0
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { reads++; return storageRefreshSnapshot(6 * time.Second) }
	if err := g.api.awaitWorkerStorageRefresh(context.Background()); err != nil || reads != 1 {
		t.Fatal("occupied slot queued another waiter", reads, err)
	}
	if g.snapshot.Load() != nil || controlledAllocationCount(g.api.deps.Backend.(*sessionsBackend)) != 0 {
		t.Fatal("cancelled or occupied wait admitted work")
	}
}

func TestWorkerStorageRefreshOrdinaryCreateCancellationPrecedesAdmission(t *testing.T) {
	g := storageRefreshGate(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { cancel(); return storageRefreshSnapshot(6 * time.Second) }
	r := httptest.NewRequest("POST", "/v1/sessions", strings.NewReader(`{"worktreeId":"w-4a77b1c9","harness":"shell"}`)).WithContext(ctx)
	w := httptest.NewRecorder()
	g.api.handleSessionCreate(w, r)
	rows, err := g.api.deps.Store.List()
	if w.Code != 409 || !strings.Contains(w.Body.String(), "cancelled before capacity admission") || err != nil || len(rows) != 0 || g.snapshot.Load() != nil || controlledAllocationCount(g.api.deps.Backend.(*sessionsBackend)) != 0 || workerStorageRefreshActive.Load() {
		t.Fatal("cancelled ordinary create reached admission or allocation", w.Code, err)
	}
}

func TestWorkerStorageRefreshOrdinaryAskCreateWaitsThenAllocatesOnce(t *testing.T) {
	f := newSwarmSessionFixtureRoute(t, "opencode", func(env *sessionsEnv) string {
		return sessionSaveConnection(t, env, 0, "", "opencode", true).ID
	})
	a := f.env.api
	reads := 0
	a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		reads++
		if reads == 1 {
			return storageRefreshSnapshot(6 * time.Second)
		}
		return storageRefreshSnapshot(0)
	}
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	f.env.be.onCreate = func(backendSpec backend.Spec) {
		if reads < 2 || workerStorageRefreshActive.Load() {
			t.Error("ordinary create allocated before fresh background publication")
		}
	}
	body, err := json.Marshal(map[string]any{"worktreeId": f.run.WorktreeID, "harness": "opencode", "permissionMode": "ask", "agentConnectionId": f.connectionID})
	if err != nil {
		t.Fatal(err)
	}
	created := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	row, err := f.store.Get(created.ID)
	if err != nil || row.PermissionMode != "ask" || row.AgentConnectionID != f.connectionID || reads < 2 || controlledAllocationCount(f.env.be) != 1 || workerStorageRefreshActive.Load() {
		t.Fatal("ordinary Ask launch lost its route/permission or duplicated allocation", reads, err)
	}
	assertExecutionLeaseCount(t, a.execution, 1)
}

func TestWorkerStorageRefreshRestartTimeoutPreservesOriginal(t *testing.T) {
	f, original := restartAdmissionFixture(t)
	f.env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return storageRefreshSnapshot(6 * time.Second) }
	started := time.Now()
	w := f.env.do(t, "POST", "/v1/sessions/"+original.ID+"/restart", "")
	elapsed := time.Since(started)
	after, err := f.store.Get(original.ID)
	if w.Code != 409 || elapsed < workerStorageRefreshBudget || elapsed > workerStorageRefreshBudget+3*time.Second || err != nil || after.SandboxID != original.SandboxID || after.AttachToken != original.AttachToken || after.Status != original.Status || f.env.be.destroyCount() != 0 || controlledAllocationCount(f.env.be) != 1 || workerStorageRefreshActive.Load() {
		t.Fatal("bounded unavailable restart changed the original or kept waiting", w.Code, elapsed, err)
	}
}

func TestWorkerStorageRefreshQueuedCancellationPrecedesReservation(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	a := f.env.api
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.deps.ResourceSnapshot = func() resourcebudget.Snapshot { cancel(); return storageRefreshSnapshot(6 * time.Second) }
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		t.Fatal(err)
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	_, err = a.admitQueuedSwarm(ctx, product.SwarmAdmissionRequest{QueueID: "cancelled-storage-wait", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
	current, readErr := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if !errors.Is(err, context.Canceled) || readErr != nil || current.Revision != f.run.Revision || a.execution.snapshot.Load() != nil || controlledAllocationCount(f.env.be) != 0 || workerStorageRefreshActive.Load() {
		t.Fatal("cancelled queued readiness reached reservation or allocation", err, readErr)
	}
}
