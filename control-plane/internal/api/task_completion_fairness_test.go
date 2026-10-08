package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type completionFairBackend struct {
	*handoffLifecycleBackend
	removes atomic.Int64
}

func (b *completionFairBackend) List(context.Context) ([]backend.Sandbox, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]backend.Sandbox, 0, len(b.rows))
	for _, row := range b.rows {
		out = append(out, row)
	}
	return out, nil
}

func (b *completionFairBackend) DestroyStopped(_ context.Context, observed backend.Sandbox) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[observed.ID]
	if !ok || row.Ref != observed.Ref || row.State != backend.StateStopped {
		return errors.New("controlled cleanup identity or state changed")
	}
	delete(b.rows, row.ID)
	b.removes.Add(1)
	return nil
}

// Four real HTTP manual task launches share one registry, queue, private state
// and execution ledger. Only backend process states and the first collector's
// delay are controlled. The three-second turn budget matches the scheduler;
// this verifies progress, not host speed or model performance.
func TestTaskCompletionObserverSlowProjectDoesNotStarveOtherProjectsAfterRestart(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	projects := []*swarmSessionFixture{f}
	for _, name := range []string{"completion-b", "completion-c", "completion-d"} {
		projects = append(projects, f.anotherProject(t, name, "api"))
	}
	a := f.env.api
	a.deps.ReadyTimeout = 5 * time.Second
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	be := &completionFairBackend{handoffLifecycleBackend: &handoffLifecycleBackend{Backend: a.deps.Backend, rows: map[string]backend.Sandbox{}}}
	a.deps.Backend = be
	policy := executionFixturePolicy()
	policy.MaxActive = 4
	var settings struct {
		Revision int `json:"revision"`
	}
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": policy, "dispatchPaused": true})
	if w = f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	const instance = "11111111-1111-4111-8111-111111111111"
	for i, project := range projects {
		body, _ = json.Marshal(project.requests[0])
		sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
		sessionsAwaitWatch(t, done)
		row, err := f.store.Get(project.ops[0].ChildID)
		if err != nil || row.Status != registry.StatusRunning || row.SandboxRef == "" {
			t.Fatal("launch not settled", err)
		}
		handoffCommit(t, project, f.env.be.spec(t, i), "api-result.txt")
		dir, err := a.taskHandoffOutputPath(row.ID)
		if err != nil {
			t.Fatal(err)
		}
		report := product.TaskHandoffReport{Version: 1, Summary: "Controlled result for " + project.owner.ID, ChangedFiles: []string{"api-result.txt"}, Decisions: []product.TaskHandoffDecision{}, ReportedChecks: []product.TaskHandoffReportedCheck{}, Questions: []product.TaskHandoffQuestion{}, Limitations: []string{"Controlled completion fairness, not model quality"}}
		data, _ := json.Marshal(report)
		if err = os.WriteFile(filepath.Join(dir, "handoff.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		be.mu.Lock()
		sandbox := be.rows[row.ID]
		sandbox.State, sandbox.Instance = backend.StateStopped, instance
		sandbox.Labels = map[string]string{backend.LabelManaged: "true", backend.LabelInstance: instance, backend.LabelSandboxID: row.SandboxID, backend.LabelSessionID: row.ID}
		be.rows[row.ID] = sandbox
		be.mu.Unlock()
	}
	r := reaper.New(reaper.Config{Store: f.store, Backend: be, Instance: instance, ConfirmStoppedTaskCleanup: a.ConfirmStoppedTaskCleanup})
	if err := r.Sweep(context.Background()); err != nil || be.removes.Load() != 4 {
		t.Fatal("real reaper did not retain four exact exits", err)
	}
	leaseBefore, err := os.ReadFile(a.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	queuePath := filepath.Join(f.options.StateDir, "swarm-scheduler.json")
	queueBefore, err := os.ReadFile(queuePath)
	if err != nil {
		t.Fatal(err)
	}
	var slowCalls atomic.Int64
	installCollector := func() {
		a = f.env.api
		a.deps.Product.SetTaskHandoffCollector(a.authorizeTaskOutput, func(ctx context.Context, req product.TaskHandoffCaptureRequest) (product.TaskHandoffCapture, error) {
			if req.ProjectID == f.owner.ID {
				slowCalls.Add(1)
				<-ctx.Done()
				return product.TaskHandoffCapture{}, ctx.Err()
			}
			return a.collectTaskHandoff(ctx, req)
		})
	}
	turn := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := a.ObserveTaskSessionsOnce(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
	marked := func(project *swarmSessionFixture) bool {
		row, err := f.store.Get(project.ops[0].ChildID)
		if err != nil {
			t.Fatal(err)
		}
		return row.TaskObservationComplete
	}
	installCollector()
	turn()
	if slowCalls.Load() != 1 || marked(f) {
		t.Fatal("first collector did not consume the shared budget without completing")
	}
	// Losing the in-memory page on restart may retry A once. It must not make
	// B/C/D permanently unreachable on every later bounded turn.
	f.reopen(t)
	installCollector()
	for attempt := 0; attempt < 4; attempt++ {
		turn()
		if marked(projects[1]) && marked(projects[2]) && marked(projects[3]) {
			break
		}
	}
	for _, project := range projects[1:] {
		if !marked(project) {
			t.Fatalf("fast project %s starved behind the slow first collector after restart", project.owner.Name)
		}
		handoffReloadRun(t, project)
		op := project.run.Operations[0]
		if op.Status != "review-required" || op.Handoff == nil || op.Handoff.Status != "captured" || project.run.Acceptance != nil || len(project.run.Checks) != 0 || project.run.Swarm.Tasks[0].Integration != nil {
			t.Fatal("fair observation inferred execution or verification authority")
		}
		var queue product.ProjectSwarmQueue
		w = f.env.do(t, "GET", "/v1/product/projects/"+project.owner.ID+"/swarm-queue", "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &queue) != nil || len(queue.Items) != 0 || !queue.DispatchPaused {
			t.Fatal("observation depended on queue dispatch", w.Code, w.Body.String())
		}
	}
	if marked(f) {
		t.Fatal("slow incomplete project was marked complete")
	}
	handoffReloadRun(t, f)
	if f.run.Operations[0].Handoff != nil {
		t.Fatal("slow report was fabricated")
	}
	leaseAfter, err := os.ReadFile(a.execution.path)
	if err != nil || !reflect.DeepEqual(leaseBefore, leaseAfter) {
		t.Fatal("observation released or changed capacity", err)
	}
	queueAfter, err := os.ReadFile(queuePath)
	if err != nil || !reflect.DeepEqual(queueBefore, queueAfter) || controlledAllocationCount(f.env.be) != 4 || f.env.be.destroyCount() != 0 || be.removes.Load() != 4 {
		t.Fatal("observation changed consent, allocation, or cleanup", err)
	}
}
