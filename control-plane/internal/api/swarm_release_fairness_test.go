package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// A controlled Docker-shaped backend: creates are real typed dispatcher calls,
// while worker execution and telemetry are deterministic, not Docker/model or
// native-platform evidence. The stopped original remains a named immutable
// object; active-only resource inventory intentionally excludes it.
type releaseFairnessBackend struct {
	*controlledLaunchFencer
	mu        sync.Mutex
	rows      map[string]backend.Sandbox
	destroyed []string
}

func (b *releaseFairnessBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	row, err := b.controlledLaunchFencer.Backend.Create(ctx, spec)
	if err != nil {
		return row, err
	}
	row.State, row.Instance = backend.StateRunning, b.identity.InstanceID
	row.Labels = map[string]string{backend.LabelManaged: "true", backend.LabelInstance: row.Instance, backend.LabelSandboxID: row.ID, backend.LabelSessionID: spec.SessionID}
	b.mu.Lock()
	b.rows[row.ID] = row
	b.mu.Unlock()
	return row, nil
}
func (b *releaseFairnessBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	if err := ctx.Err(); err != nil {
		return backend.Sandbox{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[id]
	if !ok {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	return row, nil
}
func (b *releaseFairnessBackend) Destroy(ctx context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.destroyed = append(b.destroyed, id)
	return errors.New("controlled retained-container removal failure")
}
func (b *releaseFairnessBackend) snapshot() resourcebudget.Snapshot {
	snapshot := executionFixtureSnapshot()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, row := range b.rows {
		if row.State == backend.StateRunning {
			snapshot.Execution.Containers = append(snapshot.Execution.Containers, resourcebudget.ContainerUsage{
				ID: row.Ref, SandboxID: row.ID, Owned: true, Kind: "agent", ObservedAt: snapshot.Execution.ObservedAt,
				MemoryLimitBytes: executionPointer(uint64(1024) << 20), CPULimitMillis: executionPointer(int64(1000)),
			})
		}
	}
	return snapshot
}

func TestSwarmTerminalCleanupDoesNotStarveOtherProjects(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	projects := []*swarmSessionFixture{f}
	for _, name := range []string{"release-project-b", "release-project-c", "release-project-d"} {
		projects = append(projects, f.anotherProject(t, name))
	}
	a := f.env.api
	setPaused := func(paused bool) {
		t.Helper()
		var settings struct {
			Revision int                          `json:"revision"`
			Policy   product.SwarmSchedulerPolicy `json:"policy"`
		}
		w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
			t.Fatal("read scheduler pause", w.Code, w.Body.String())
		}
		body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": settings.Policy, "dispatchPaused": paused})
		if w := f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
			t.Fatal("set scheduler pause", w.Code, w.Body.String())
		}
	}
	installRuntime := func(a *API) {
		a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{
			CaptureActor: a.captureQueuedActor, Admit: a.admitQueuedSwarm, Dispatch: a.dispatchQueuedSwarm,
			Release: a.execution.release, Resources: a.queuedResourceView,
		})
	}
	quiesce := func(a *API) {
		t.Helper()
		setPaused(true)
		installRuntime(a)
		if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil && !errors.Is(err, ErrExecutionLeaseHeld) {
			t.Fatal(err)
		}
	}
	resume := func(a *API) {
		installRuntime(a)
		setPaused(false)
	}
	quiesce(a)
	be := &releaseFairnessBackend{controlledLaunchFencer: &controlledLaunchFencer{
		Backend: f.env.be, identity: backend.LaunchFenceIdentity{Protocol: 1, EngineID: "controlled-release-engine", InstanceID: "11111111-1111-4111-8111-111111111111"}, proofs: map[string]backend.LaunchFenceProof{},
	}, rows: map[string]backend.Sandbox{}}
	a.deps.Backend, a.deps.ResourceSnapshot = be, be.snapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	policy := product.SwarmSchedulerPolicy{MaxActive: 4, MaxPerProject: 1, HeadroomPercent: 25, MinHeadroomMiB: 2048, DiskReserveMiB: 2048}
	var settings struct {
		Revision int `json:"revision"`
	}
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
		t.Fatal("scheduler settings", w.Code, w.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": policy, "dispatchPaused": true})
	if w := f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
		t.Fatal("four-agent policy", w.Code, w.Body.String())
	}
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	queueTask := func(p *swarmSessionFixture) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"expectedRevision": p.run.Revision, "idempotencyKey": "retained-release-" + p.owner.ID, "taskIds": []string{"api"}, "priority": "normal"})
		w := p.env.do(t, "POST", p.base()+"/"+p.run.ID+"/swarm/queue", string(body))
		var response struct {
			Items []product.SwarmQueueItem `json:"items"`
		}
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Items) != 1 {
			t.Fatal("queue consent", w.Code, w.Body.String())
		}
		return response.Items[0].ID
	}
	queueA := queueTask(f)
	resume(a)
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal("initial project A launch", err)
	}
	childA := sessionsAwaitWatch(t, done)
	quiesce(a)
	rowA, err := f.store.Get(childA)
	if err != nil || rowA.Status != registry.StatusRunning || rowA.SandboxRef == "" {
		t.Fatal("initial exact child not acknowledged", err)
	}
	be.mu.Lock()
	stopped := be.rows[childA]
	stopped.State = backend.StateStopped
	be.rows[childA] = stopped
	be.mu.Unlock()
	// Controlled process observation, not a fabricated passing check. The
	// exact immutable container is stopped but its cleanup has not completed.
	if _, err := f.store.Update(childA, func(row *registry.Session) error {
		row.Status, row.ExitReason = registry.StatusExited, "controlled task process failed"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	resume(a)
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil && !errors.Is(err, ErrExecutionLeaseHeld) {
		t.Fatal("record A failure with retained capacity", err)
	}
	quiesce(a)
	readQueue := func(p *swarmSessionFixture) product.ProjectSwarmQueue {
		t.Helper()
		var queue product.ProjectSwarmQueue
		w := p.env.do(t, "GET", "/v1/product/projects/"+p.owner.ID+"/swarm-queue", "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &queue) != nil || len(queue.Items) != 1 {
			t.Fatal("queue view", w.Code, w.Body.String())
		}
		return queue
	}
	if q := readQueue(f); q.Items[0].ID != queueA || q.Items[0].State != "failed" || q.Items[0].MemoryMiB != 1024 || q.Items[0].CPUMillis != 1000 {
		t.Fatal("A is not the required terminal failure with retained capacity", q)
	}
	for _, p := range projects[1:] {
		queueTask(p)
	}
	resume(a)
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatalf("terminal cleanup must not block the three independently admissible projects: %v", err)
	}
	for range 3 {
		sessionsAwaitWatch(t, done)
	}
	assertRetainedAndProgressed := func(a *API) {
		t.Helper()
		if controlledAllocationCount(f.env.be) != 4 {
			t.Fatal("four projects did not retain exactly four attempts", controlledAllocationCount(f.env.be))
		}
		ledger, err := a.execution.readFenceLedger()
		if err != nil || len(ledger.Leases) != 4 {
			t.Fatal("A's retained reservation was freed/duplicated", len(ledger.Leases), err)
		}
		foundA, totalMemory := false, 0
		for _, lease := range ledger.Leases {
			totalMemory += lease.Resources.MemoryMiB
			if lease.ID == queueA {
				foundA = lease.SessionID == childA && lease.SandboxID == rowA.SandboxID && lease.State == "backend-settled" && lease.Resources.MemoryMiB == 1024 && lease.FenceEngineID == be.identity.EngineID
			}
		}
		if !foundA || totalMemory != 4096 {
			t.Fatal("retained A is not charged one unchanged GiB", foundA, totalMemory)
		}
		for i, p := range projects {
			q := readQueue(p)
			if i == 0 {
				if q.Items[0].State != "failed" || q.Items[0].MemoryMiB != 1024 || !strings.Contains(q.Items[0].Reason, "reservation remains held") {
					t.Fatal("A failure/held cleanup disappeared", q)
				}
			} else {
				row, err := f.store.Get(q.Items[0].ChildID)
				if err != nil || q.Items[0].State != "running" || row.ProjectID != p.owner.ID || row.Status != registry.StatusRunning || row.SandboxRef == "" {
					t.Fatalf("project %d did not progress with its own child: queue=%s err=%v", i, q.Items[0].State, err)
				}
			}
		}
		// One stopped, retained reservation plus three live workers exhaust the
		// four-slot budget; skipping cleanup must not silently free a fifth slot.
		actor := management.ExecutionActorRef{Local: true, ProjectID: projects[1].owner.ID}
		decision, err := a.execution.reserve(context.Background(), "capacity-probe-five", actor.ProjectID, "separate-capacity-probe", 4, actor, management.ExecutionAuthority{}, policy)
		if err != nil || decision.State != "waiting" || decision.Code != "global_slot_limit" {
			t.Fatal("held cleanup reservation no longer consumes its global slot", decision, err)
		}
		be.mu.Lock()
		defer be.mu.Unlock()
		if len(be.destroyed) != 0 || len(be.rows) != 4 || be.rows[childA].Ref != rowA.SandboxRef || be.rows[childA].State != backend.StateStopped {
			t.Fatal("scheduler removed or replaced retained output/runtime")
		}
	}
	for range 2 {
		if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal("repeated scheduling turn blocked on item-local cleanup", err)
		}
	}
	assertRetainedAndProgressed(a)
	quiesce(a)
	f.reopen(t)
	a = f.env.api
	for range 2 {
		if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal("reopened controller starved on persistent cleanup", err)
		}
	}
	assertRetainedAndProgressed(a)
	path, err := a.deps.Worktrees.Path(context.Background(), rowA.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatal(fmt.Sprintf("retained worktree unavailable: %v", err))
	}
}
