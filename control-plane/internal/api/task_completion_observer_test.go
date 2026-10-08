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
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func completionReport(t *testing.T, f *swarmSessionFixture, row registry.Session) {
	t.Helper()
	handoffCommit(t, f, f.env.be.spec(t, 0), "api-result.txt")
	dir, err := f.env.api.taskHandoffOutputPath(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	report := product.TaskHandoffReport{Version: 1, Summary: "Controlled natural task result", ChangedFiles: []string{"api-result.txt"}, Decisions: []product.TaskHandoffDecision{}, ReportedChecks: []product.TaskHandoffReportedCheck{{Command: "worker-claimed-check", Outcome: "passed", Detail: "Not controller verification"}}, Questions: []product.TaskHandoffQuestion{{Question: "Which retention period?", Impact: "Data lifecycle", Blocking: true}}, Limitations: []string{"Controlled backend process states; no model-quality claim"}}
	data, _ := json.Marshal(report)
	if err = os.WriteFile(filepath.Join(dir, "handoff.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func completionReaper(t *testing.T, f *swarmSessionFixture, row registry.Session, removalErr error) *taskCleanupLifecycleBackend {
	t.Helper()
	const instance = "11111111-1111-4111-8111-111111111111"
	be := &taskCleanupLifecycleBackend{Backend: f.env.api.deps.Backend, sandbox: backend.Sandbox{ID: row.SandboxID, Ref: row.SandboxRef, Kind: backend.KindDocker, State: backend.StateStopped, Instance: instance, Labels: map[string]string{backend.LabelManaged: "true", backend.LabelInstance: instance, backend.LabelSandboxID: row.SandboxID, backend.LabelSessionID: row.ID}}, removeErr: removalErr}
	f.env.api.deps.Backend = be
	r := reaper.New(reaper.Config{Store: f.store, Backend: be, Instance: instance, ConfirmStoppedTaskCleanup: f.env.api.ConfirmStoppedTaskCleanup})
	if err := r.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	return be
}

func completionCandidate(t *testing.T, f *swarmSessionFixture) registry.TaskObservationCandidate {
	t.Helper()
	rows, _, err := f.store.TaskObservationPage(0, 128, 8)
	if err != nil || len(rows) != 1 {
		t.Fatal("expected one durable observation", len(rows), err)
	}
	return rows[0]
}

// Actual HTTP manual launch, real private Git/context/registry, reaper exit CAS
// and exact stopped cleanup, then the production timer observes after restart.
// No task queue entry, POST reconcile, collect, stop or completion fabrication.
func TestTaskCompletionObserverManualExitAfterCleanupRestartUpdatesReportAndHome(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	completionReport(t, f, row)
	var settings struct {
		Revision int `json:"revision"`
	}
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": executionFixturePolicy(), "dispatchPaused": true})
	if w = f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	be := completionReaper(t, f, row, nil)
	if !be.absent || be.removes != 1 || be.forceCalls != 0 {
		t.Fatal("exact stopped allocation not removed", be)
	}
	handoffReloadRun(t, f)
	if f.run.Operations[0].Status != "dispatching" || f.run.Operations[0].Handoff != nil {
		t.Fatal("fixture was already product-observed")
	}
	pending, err := f.store.Get(row.ID)
	if err != nil || pending.Status != registry.StatusExited || pending.TaskObservationComplete {
		t.Fatal("missing durable pending terminal row", err)
	}

	// Observation has service-owned read provenance, not the former user's
	// execution grant. A revoked grant remains revoked and is never impersonated.
	var authCalls atomic.Int64
	f.env.api.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
		authCalls.Add(1)
		return errors.New("current grant revoked")
	}
	f.env.api.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
		authCalls.Add(1)
		return management.ExecutionAuthority{}, errors.New("current grant revoked")
	}
	f.env.api.deps.ObserveTaskCompletions = true
	f.reopen(t)
	deadline := time.After(15 * time.Second)
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		current, err := f.store.Get(row.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.TaskObservationComplete {
			break
		}
		select {
		case <-deadline:
			t.Fatal("background observation did not finish without a queue or reconcile")
		case <-poll.C:
		}
	}
	handoffReloadRun(t, f)
	op := f.run.Operations[0]
	if op.Status != "review-required" || op.Handoff == nil || op.Handoff.Status != "captured" || op.Handoff.Unanswered != 1 || len(f.run.Checks) != 0 || f.run.Acceptance != nil || f.run.Swarm.Tasks[0].Integration != nil {
		t.Fatal("recorded completion lost claims or inferred verification", op)
	}
	var report struct {
		Handoff product.TaskHandoffView `json:"handoff"`
	}
	w = f.env.do(t, "GET", f.base()+"/"+f.run.ID+"/swarm/operations/"+op.ID+"/handoff", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil || report.Handoff.Receipt == nil || report.Handoff.Receipt.Reported == nil || len(report.Handoff.QuestionIDs) != 1 || len(report.Handoff.RecordedChecks) != 0 {
		t.Fatal("report not readable", w.Code, w.Body.String())
	}
	var home struct {
		Attention product.ProjectOverviewAttention `json:"attention"`
	}
	w = f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/overview", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &home) != nil || home.Attention.Count == nil || *home.Attention.Count != 1 || len(home.Attention.Items) != 1 || home.Attention.Items[0].Kind != "task-question" {
		t.Fatal("Home lost manual completion attention", w.Code, w.Body.String())
	}
	var queue product.ProjectSwarmQueue
	w = f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/swarm-queue", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &queue) != nil || len(queue.Items) != 0 || !queue.DispatchPaused {
		t.Fatal("observation depended on/changed queue", w.Code, w.Body.String())
	}
	if authCalls.Load() != 0 || controlledAllocationCount(f.env.be) != 1 || be.forceCalls != 0 {
		t.Fatal("observation impersonated a user, allocated or forced cleanup")
	}
	before := f.run
	if err := f.env.api.ObserveTaskSessionsOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	handoffReloadRun(t, f)
	if !reflect.DeepEqual(before, f.run) {
		t.Fatal("completed observation rewrote immutable results")
	}
}

func TestTaskCompletionObserverRetriesBusyAndCleanupFailureWithoutDispatch(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	completionReport(t, f, row)
	be := completionReaper(t, f, row, errors.New("controlled cleanup failure"))
	if be.absent || be.removes != 1 {
		t.Fatal("cleanup failure fixture invalid")
	}
	if !sessionRestarts.acquire(row.ID) {
		t.Fatal("could not hold child lifecycle")
	}
	err := f.env.api.ObserveTaskSessionsOnce(context.Background())
	sessionRestarts.release(row.ID)
	if err == nil {
		t.Fatal("busy output was treated as captured")
	}
	pending, _ := f.store.Get(row.ID)
	handoffReloadRun(t, f)
	if pending.TaskObservationComplete || f.run.Operations[0].Handoff != nil {
		t.Fatal("failed capture lost durable retry")
	}
	f.reopen(t)
	if err = f.env.api.ObserveTaskSessionsOnce(context.Background()); err != nil {
		t.Fatal("stopped output retry after restart", err)
	}
	observed, _ := f.store.Get(row.ID)
	handoffReloadRun(t, f)
	if !observed.TaskObservationComplete || f.run.Operations[0].Handoff == nil || f.run.Operations[0].Status != "review-required" {
		t.Fatal("retry did not retain canonical output")
	}
	if be.removes != 1 || be.forceCalls != 0 || be.absent || controlledAllocationCount(f.env.be) != 1 {
		t.Fatal("observer mutated cleanup or allocation")
	}
	if err = f.env.api.execution.releaseSession(context.Background(), row.ID); !errors.Is(err, ErrExecutionLeaseHeld) {
		t.Fatal("report observation released still-stored task capacity", err)
	}
}

func TestTaskCompletionObserverQueueReadFailureRetainsCaptureUntilOwnershipRecovers(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	completionReport(t, f, row)
	// Persist a valid empty, paused queue before introducing the read failure.
	var settings struct {
		Revision int `json:"revision"`
	}
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": executionFixturePolicy(), "dispatchPaused": true})
	if w = f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	queuePath := filepath.Join(f.options.StateDir, "swarm-scheduler.json")
	valid, err := os.ReadFile(queuePath)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{"version":1`)
	if err = os.WriteFile(queuePath, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	be := completionReaper(t, f, row, nil)
	f.env.api.deps.ObserveTaskCompletions = true
	f.reopen(t)
	if err = f.env.api.deps.Product.RunSwarmSchedulerOnce(context.Background()); err == nil {
		t.Fatal("corrupt queue was accepted")
	}
	handoffReloadRun(t, f)
	current, err := f.store.Get(row.ID)
	if err != nil || f.run.Operations[0].Status != "review-required" || f.run.Operations[0].Handoff != nil || current.TaskObservationComplete {
		t.Fatal("queue read failure suppressed the terminal fact or fabricated ownership", err)
	}
	after, err := os.ReadFile(queuePath)
	if err != nil || !reflect.DeepEqual(after, invalid) || controlledAllocationCount(f.env.be) != 1 || be.removes != 1 {
		t.Fatal("observer repaired the queue, allocated, or repeated cleanup", err)
	}
	// Restore the exact original bytes; no run/registry edits or reconcile POST.
	if err = os.WriteFile(queuePath, valid, 0600); err != nil {
		t.Fatal(err)
	}
	if err = f.env.api.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	handoffReloadRun(t, f)
	current, err = f.store.Get(row.ID)
	if err != nil || !current.TaskObservationComplete || f.run.Operations[0].Handoff == nil || f.run.Operations[0].Handoff.Status != "captured" {
		t.Fatal("restored ownership did not permit the retained report retry", err)
	}
	after, err = os.ReadFile(queuePath)
	if err != nil || !reflect.DeepEqual(after, valid) || controlledAllocationCount(f.env.be) != 1 || be.removes != 1 {
		t.Fatal("completion mutated queue consent or process lifecycle", err)
	}
}

func TestTaskCompletionObserverRejectsStaleAndForeignGenerations(t *testing.T) {
	for _, change := range []string{"restarted", "ref", "project", "operation", "request"} {
		t.Run(change, func(t *testing.T) {
			f, row, _ := settledTaskFixture(t)
			completionReport(t, f, row)
			completionReaper(t, f, row, nil)
			candidate := completionCandidate(t, f)
			before, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.Update(row.ID, func(current *registry.Session) error {
				switch change {
				case "restarted":
					current.Status = registry.StatusRunning
				case "ref":
					current.SandboxRef += "different"
				case "project":
					current.ProjectID = "p-foreign"
				case "operation":
					current.ProjectRunOperationID = "op-foreign"
				case "request":
					current.Model += "different"
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err = f.env.api.observeTaskCompletionCandidate(context.Background(), candidate); err == nil {
				t.Fatal("stale snapshot was published")
			}
			current, _ := f.store.Get(row.ID)
			if current.TaskObservationComplete {
				t.Fatal("replacement was marked observed")
			}
			after, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("stale snapshot rewrote the original operation", err)
			}
			// A fresh foreign/mismatched row is also rejected by the canonical run
			// binding, rather than relying only on its stale metadata timestamp.
			if change != "restarted" && change != "ref" {
				if err = f.env.api.ObserveTaskSessionsOnce(context.Background()); err == nil {
					t.Fatal("foreign binding was accepted")
				}
			}
		})
	}
}

// The private run can be published just before the session generation moves.
// Its immutable report remains valid history; only the old watermark is denied.
type completionMovingStore struct {
	*registry.FileStore
	beforeWatermark func()
}

func (s *completionMovingStore) Update(id string, change func(*registry.Session) error) (registry.Session, error) {
	if s.beforeWatermark != nil {
		move := s.beforeWatermark
		s.beforeWatermark = nil
		move()
	}
	return s.FileStore.Update(id, change)
}
func TestTaskCompletionObserverCASAfterReportPublicationDoesNotMarkReplacement(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	completionReport(t, f, row)
	completionReaper(t, f, row, nil)
	candidate := completionCandidate(t, f)
	moved := &completionMovingStore{FileStore: f.store}
	moved.beforeWatermark = func() {
		current, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
		if err != nil || current.Operations[0].Handoff == nil {
			t.Fatal("race did not occur after durable publication", err)
		}
		if _, err = f.store.Update(row.ID, func(saved *registry.Session) error {
			saved.Status, saved.SandboxRef = registry.StatusRunning, saved.SandboxRef+"-replacement"
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	f.env.api.deps.Store = moved
	if err := f.env.api.observeTaskCompletionCandidate(context.Background(), candidate); !errors.Is(err, product.ErrProjectRunConflict) {
		t.Fatal("late watermark did not compare generation", err)
	}
	current, err := f.store.Get(row.ID)
	if err != nil || current.TaskObservationComplete || current.Status != registry.StatusRunning {
		t.Fatal("replacement was marked complete", current, err)
	}
	handoffReloadRun(t, f)
	if f.run.Operations[0].Handoff == nil || f.run.Operations[0].Status != "review-required" || controlledAllocationCount(f.env.be) != 1 {
		t.Fatal("exact old report history lost or new child created")
	}
}
