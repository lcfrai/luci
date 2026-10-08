package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// This controlled adapter makes the exact stopped-removal boundary visible to
// the real reaper. No Docker/process/model execution is claimed by this test.
type restartReleaseBackend struct {
	*releaseFairnessBackend
	cleanupID, cleanupRef string
	allowCleanup          bool
	stoppedCleanups       []backend.Sandbox
}

func (b *restartReleaseBackend) List(ctx context.Context) ([]backend.Sandbox, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	rows := make([]backend.Sandbox, 0, len(b.rows))
	for _, row := range b.rows {
		rows = append(rows, row)
	}
	return rows, nil
}
func (b *restartReleaseBackend) DestroyStopped(ctx context.Context, observed backend.Sandbox) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current, ok := b.rows[observed.ID]
	if !ok || observed.ID != b.cleanupID || observed.Ref != b.cleanupRef || current.Ref != observed.Ref || current.State != backend.StateStopped || observed.State != backend.StateStopped || current.Instance != observed.Instance {
		return errors.New("controlled exact cleanup identity mismatch")
	}
	b.stoppedCleanups = append(b.stoppedCleanups, observed)
	if !b.allowCleanup {
		return errors.New("controlled stopped-object removal remains pending")
	}
	delete(b.rows, observed.ID)
	return nil
}

func TestSwarmTerminalCleanupRestartBeforeWaitingProjectAdmission(t *testing.T) {
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
	quiesce := func(a *API) {
		t.Helper()
		setPaused(true)
		if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil && !errors.Is(err, ErrExecutionLeaseHeld) {
			t.Fatal(err)
		}
	}
	resume := func(a *API) {
		setPaused(false)
	}
	quiesce(a)
	baseBackend := &releaseFairnessBackend{controlledLaunchFencer: &controlledLaunchFencer{
		Backend: f.env.be, identity: backend.LaunchFenceIdentity{Protocol: 1, EngineID: "controlled-release-engine", InstanceID: "11111111-1111-4111-8111-111111111111"}, proofs: map[string]backend.LaunchFenceProof{},
	}, rows: map[string]backend.Sandbox{}}
	be := &restartReleaseBackend{releaseFairnessBackend: baseBackend}
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
		body, _ := json.Marshal(map[string]any{"expectedRevision": p.run.Revision, "idempotencyKey": "restart-retained-release-" + p.owner.ID, "taskIds": []string{"api"}, "priority": "normal"})
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
	waitingIDs := map[string]string{}
	for _, p := range projects[1:] {
		waitingIDs[p.owner.ID] = queueTask(p)
	}
	// Persist the important restart boundary: A is stopped and still charged;
	// B/C/D have consented queue records but no operation, child or allocation.
	assertWaiting := func() executionLease {
		t.Helper()
		if controlledAllocationCount(f.env.be) != 1 {
			t.Fatal("waiting projects launched before the restart/resume boundary")
		}
		ledger, err := a.execution.readFenceLedger()
		if err != nil || len(ledger.Leases) != 1 {
			t.Fatal("pre-resume ledger lost exact retained A", err)
		}
		lease := ledger.Leases[0]
		if lease.ID != queueA || lease.SessionID != childA || lease.SandboxID != rowA.SandboxID || lease.State != "backend-settled" || lease.Resources.MemoryMiB != 1024 || lease.Resources.CPUMillis != 1000 || lease.FenceEngineID != be.identity.EngineID {
			t.Fatal("pre-resume A ownership or charge changed")
		}
		if q := readQueue(f); q.Items[0].State != "failed" || q.Items[0].MemoryMiB != 1024 || !strings.Contains(q.Items[0].Reason, "reservation remains held") {
			t.Fatal("pre-resume A lost failed/held status")
		}
		for _, p := range projects[1:] {
			q := readQueue(p)
			item := q.Items[0]
			if item.ID != waitingIDs[p.owner.ID] || item.State != "queued" || item.OperationID != "" || item.ChildID != "" || item.MemoryMiB != 0 || item.CPUMillis != 0 {
				t.Fatal("waiting project was allocated before ordinary resume", item.State)
			}
			run, err := a.deps.Product.GetProjectSwarmRun(p.owner.ID, p.run.ID)
			if err != nil || len(run.Operations) != 0 {
				t.Fatal("waiting project acquired a private operation", err)
			}
		}
		return lease
	}
	retainedBefore := assertWaiting()
	priorAPI, priorStore := a, f.store
	f.reopen(t)
	a = f.env.api
	if a == priorAPI || f.store == priorStore {
		t.Fatal("fixture did not reopen both service and registry")
	}
	if enabled, err := a.deps.Product.SwarmSchedulerDispatchEnabled(); err != nil || enabled {
		t.Fatal("restart did not retain the ordinary global pause", enabled, err)
	}
	for range 2 {
		if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal("paused restarted cleanup turn failed", err)
		}
	}
	if reopened := assertWaiting(); reopened != retainedBefore {
		t.Fatal("restart changed A's durable reservation")
	}
	// These are the actual API.New-installed callbacks. No post-restart
	// callback replacement, direct child launch or private queue edit occurs.
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
	assertRetainedAndProgressed(a)
	path, err := a.deps.Worktrees.Path(context.Background(), rowA.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("retained worktree unavailable: %v", err)
	}
	beforeRemoval, err := a.execution.readFenceLedger()
	if err != nil {
		t.Fatal(err)
	}
	peersBefore := map[string]executionLease{}
	for _, lease := range beforeRemoval.Leases {
		if lease.ID != queueA {
			peersBefore[lease.ID] = lease
		}
	}
	if len(peersBefore) != 3 {
		t.Fatal("missing peer claims before exact cleanup")
	}
	be.mu.Lock()
	be.cleanupID, be.cleanupRef = rowA.SandboxID, rowA.SandboxRef
	be.mu.Unlock()
	observer := reaper.New(reaper.Config{Store: f.store, Backend: be, Instance: be.identity.InstanceID, ConfirmStoppedTaskCleanup: a.ConfirmStoppedTaskCleanup})
	// First a real sweep sees a failed exact Remove. A remains stopped,
	// charged, reviewable and distinct from all three running peer objects.
	if err := observer.Sweep(context.Background()); err != nil {
		t.Fatal("controlled failed-cleanup sweep", err)
	}
	assertRetainedAndProgressed(a)
	be.mu.Lock()
	if len(be.stoppedCleanups) != 1 || be.stoppedCleanups[0].ID != rowA.SandboxID || be.stoppedCleanups[0].Ref != rowA.SandboxRef {
		be.mu.Unlock()
		t.Fatal("sweep did not target only the exact stopped original")
	}
	be.allowCleanup = true
	be.mu.Unlock()
	if err := observer.Sweep(context.Background()); err != nil {
		t.Fatal("controlled exact cleanup retry", err)
	}
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal("confirmed absence did not release retained A", err)
	}
	afterRemoval, err := a.execution.readFenceLedger()
	if err != nil || len(afterRemoval.Leases) != 3 {
		t.Fatal("confirmed cleanup released the wrong number of claims", err)
	}
	totalMemory := 0
	for _, lease := range afterRemoval.Leases {
		want, ok := peersBefore[lease.ID]
		if !ok || lease != want {
			t.Fatal("exact A cleanup changed another project's claim")
		}
		totalMemory += lease.Resources.MemoryMiB
	}
	if totalMemory != 3072 {
		t.Fatal("only A's one-GiB charge should be released", totalMemory)
	}
	q := readQueue(f)
	if q.Items[0].State != "failed" || q.Items[0].MemoryMiB != 0 || q.Items[0].CPUMillis != 0 || strings.Contains(q.Items[0].Reason, "reservation remains held") {
		t.Fatal("A's durable failure/release status is inaccurate", q.Items[0])
	}
	retainedRow, err := f.store.Get(childA)
	if err != nil || retainedRow.Status != registry.StatusExited || retainedRow.SandboxRef != rowA.SandboxRef {
		t.Fatal("cleanup discarded the retained session receipt", err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatal("runtime removal discarded retained source", err)
	}
	be.mu.Lock()
	if len(be.stoppedCleanups) != 2 || len(be.destroyed) != 0 || len(be.rows) != 3 {
		be.mu.Unlock()
		t.Fatal("cleanup was not exact, bounded, or stopped-only")
	}
	for _, observed := range be.stoppedCleanups {
		if observed.ID != rowA.SandboxID || observed.Ref != rowA.SandboxRef {
			be.mu.Unlock()
			t.Fatal("cleanup targeted a peer or replacement")
		}
	}
	for _, sb := range be.rows {
		if sb.State != backend.StateRunning {
			be.mu.Unlock()
			t.Fatal("cleanup affected a running peer")
		}
	}
	be.mu.Unlock()
	if controlledAllocationCount(f.env.be) != 4 {
		t.Fatal("cleanup or reopen replayed an attempt")
	}

}
