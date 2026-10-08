package product

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type swarmLifecycleBackend struct {
	backend.Backend
	rows map[string]backend.Sandbox
}

func (b *swarmLifecycleBackend) Kind() backend.Kind { return backend.KindDocker }
func (b *swarmLifecycleBackend) List(context.Context) ([]backend.Sandbox, error) {
	rows := []backend.Sandbox{}
	for _, row := range b.rows {
		rows = append(rows, row)
	}
	return rows, nil
}
func (b *swarmLifecycleBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	if row, ok := b.rows[id]; ok {
		return row, nil
	}
	return backend.Sandbox{}, backend.ErrNotFound
}
func (b *swarmLifecycleBackend) Destroy(_ context.Context, id string) error {
	delete(b.rows, id)
	return nil
}

func TestSwarmReaperExitRequiresReviewReleasesConfirmedCapacityAndPreservesDependencies(t *testing.T) {
	s, mux, run, base, admitted, starts, children := schedulerFixture(t)
	*admitted = true
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err == nil {
		ledger.Policy.MaxActive, ledger.Policy.MaxPerProject = 1, 1
		err = s.saveSwarmScheduler(&ledger)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b", "task-c"})
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil || len(current.Operations) != 1 || *starts != 1 {
		t.Fatal("first task did not start alone", current, err)
	}
	op := current.Operations[0]
	workerPath := mustSwarmPathFixture(t, s, run.ProjectID, run.ID, op.ID)
	commit := swarmCommitFixture(t, workerPath, "alpha.txt", "review this retained change\n")
	store, err := registry.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	child := children[op.ID]
	child.SandboxID = child.ID
	if err = store.Create(child); err != nil {
		t.Fatal(err)
	}
	s.projectRunSessionObserver = func(_ context.Context, _, _, id string) ([]registry.Session, error) {
		if id == op.ID {
			row, err := store.Get(child.ID)
			return []registry.Session{row}, err
		}
		if row, ok := children[id]; ok {
			return []registry.Session{row}, nil
		}
		return nil, nil
	}
	be := &swarmLifecycleBackend{rows: map[string]backend.Sandbox{child.ID: {ID: child.ID, State: backend.StateStopped, Instance: "swarm-lifecycle"}}}
	collector := reaper.New(reaper.Config{Store: store, Backend: be, Instance: "swarm-lifecycle", Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err = collector.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	exited, err := store.Get(child.ID)
	if err != nil || exited.Status != registry.StatusExited || exited.ExitReason != "process exited" {
		t.Fatal("production reaper did not observe stopped process", exited, err)
	}
	absenceConfirmed := false
	s.swarmScheduler.runtime.Release = func(context.Context, string) error {
		if !absenceConfirmed {
			return errors.New("backend lifecycle is not yet confirmed")
		}
		return nil
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err = s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil || current.Operations[0].Status != "review-required" || current.Acceptance != nil || len(current.Checks) != 0 || len(current.Releases) != 0 || swarmTask(&current, "task-a").Integration != nil || *starts != 1 {
		t.Fatal("process exit bypassed review or unconfirmed capacity", current, err)
	}
	queue, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || queue.Counts.Unknown != 1 || queue.Items[0].MemoryMiB != 1024 {
		t.Fatal("unconfirmed stopped child lost its reservation", queue, err)
	}
	// A fresh backend absence releases capacity. The next independent task
	// starts from the reviewed main source, never the unintegrated worker tip.
	if err = collector.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = be.Get(context.Background(), child.ID); !errors.Is(err, backend.ErrNotFound) {
		t.Fatal("terminal sandbox still present", err)
	}
	absenceConfirmed = true
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err = s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil || len(current.Operations) != 2 || *starts != 2 || current.Operations[1].TaskID != "task-b" || current.Operations[1].BaseCommit != run.Input.Implementation.SourceCommit || current.Operations[1].BaseCommit == commit || swarmTask(&current, "task-c").OperationID != "" {
		t.Fatal("independent admission lost source/dependency boundary", current, err)
	}
	queue, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || queue.Counts.Completed != 1 || queue.Items[0].ReasonCode != "review_required" || queue.Items[0].MemoryMiB != 0 {
		t.Fatal("stopped task is not awaiting review with released capacity", queue, err)
	}
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	review, err := s.swarmReview(context.Background(), p, current, "task-a")
	if err != nil || !review.CanIntegrate || review.SourceCommit != commit {
		t.Fatal("normally exited worker cannot enter explicit integration review", review, err)
	}
	s.projectRuns, err = NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 2 {
		t.Fatal("restart replayed terminal native task", *starts, err)
	}
	current, _ = s.GetProjectSwarmRun(run.ProjectID, run.ID)
	journeyRequest(t, mux, "POST", base+"/tasks/task-c/reserve", map[string]any{"expectedRevision": current.Revision, "idempotencyKey": "dependent-after-process-exit"}, 409)
	journeyRequest(t, mux, "POST", strings.TrimSuffix(base, "/swarm")+"/accept", map[string]any{"expectedRevision": current.Revision}, 409)
}

func TestSwarmTaskPermissionsRefuseBeforePreparationAllocationOrQueue(t *testing.T) {
	for _, mode := range []string{"ask", "plan"} {
		t.Run(mode, func(t *testing.T) {
			s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
			req.PermissionMode, selected.PermissionMode = mode, mode
			base := "/v1/product/projects/" + p.ID + "/runs"
			run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
			base += "/" + run.ID + "/swarm"
			before := auditTree(t, s.stateDir)
			w := journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": run.Revision, "maxParallel": 1}, 409)
			if !strings.Contains(w.Body.String(), `"task_permission_requires_interactive"`) || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("unsupported permission prepared state or lost actionable code", w.Body.String())
			}
			// An existing pre-one-shot swarm retains its saved permissions; it
			// cannot silently inherit Bypass when resuming in the newer runtime.
			run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(row *ProjectRun) error {
				row.Swarm = &ProjectSwarm{State: "ready", MaxParallel: 1, Tasks: []ProjectSwarmTask{{ID: "task-save", Title: "Implement saving", DependsOn: []string{}, WritePaths: []string{}}}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			s.registerSwarmScheduler(mux)
			s.swarmScheduler.runtime = SwarmSchedulerRuntime{CaptureActor: management.CaptureExecutionActor,
				Admit: func(context.Context, SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
					t.Fatal("unsupported task admitted resources")
					return SwarmAdmissionResult{}, nil
				},
				Dispatch: func(context.Context, SwarmDispatchRequest) (SwarmDispatchResult, error) {
					t.Fatal("unsupported task dispatched")
					return SwarmDispatchResult{}, nil
				},
				Release: func(context.Context, string) error { t.Fatal("unsupported task allocated resources"); return nil },
			}
			before, sourceBefore := auditTree(t, s.stateDir), auditTree(t, p.Path)
			for _, action := range []string{"reserve", "queue"} {
				w = journeyRequest(t, mux, "POST", base+"/tasks/task-save/"+action, map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "permission-refusal-" + action}, 409)
				if !strings.Contains(w.Body.String(), `"task_permission_requires_interactive"`) {
					t.Fatal("unsupported permission lost actionable reason", w.Body.String())
				}
			}
			if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) || !reflect.DeepEqual(sourceBefore, auditTree(t, p.Path)) {
				t.Fatal("permission refusal changed stored consent or source")
			}
			retained, _ := s.GetProjectSwarmRun(run.ProjectID, run.ID)
			if retained.Input.Route.PermissionMode != mode || len(retained.Operations) != 0 {
				t.Fatal("permission refusal altered exact saved route", retained)
			}
		})
	}
}

func TestSwarmKnownFailureAndDisappearanceKeepDistinctUnverifiedOutcomes(t *testing.T) {
	for _, tc := range []struct{ reason, status, queueState string }{
		{"exit code 1", "failed", "completed"},
		{"killed", "failed", "completed"},
		{"sandbox disappeared", "unknown", "unknown"},
		{"project-run-outcome-unknown", "unknown", "unknown"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			s, mux, run, base, admitted, starts, children := schedulerFixture(t)
			*admitted = true
			enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
			if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
			if err != nil || len(current.Operations) != 1 {
				t.Fatal(current, err)
			}
			op := current.Operations[0]
			child := children[op.ID]
			child.Status, child.ExitReason = registry.StatusExited, tc.reason
			children[op.ID] = child
			if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			current, err = s.GetProjectSwarmRun(run.ProjectID, run.ID)
			queue, queueErr := s.ProjectSwarmQueueSnapshot(run.ProjectID)
			if err != nil || queueErr != nil || current.Operations[0].Status != tc.status || queue.Items[0].State != tc.queueState || *starts != 1 || current.Acceptance != nil || len(current.Checks) != 0 || swarmTask(&current, "task-a").Integration != nil {
				t.Fatal("lifecycle inferred success or replayed uncertain work", current, queue, err, queueErr)
			}
			if tc.status == "unknown" && queue.Items[0].MemoryMiB != 1024 {
				t.Fatal("uncertain process lost capacity reservation")
			}
		})
	}
}

func TestSwarmResumeRetainsUnsupportedSavedPermissionWithoutRequeue(t *testing.T) {
	s, mux, run, base, _, starts, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	control := "/v1/product/projects/" + run.ProjectID + "/swarm-queue/" + items[0].ID
	journeyRequest(t, mux, "POST", control+"/pause", map[string]int{"expectedRevision": items[0].Revision}, 200)
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	// Persist a legacy Ask route through the store's full validator. New
	// capabilities must not rewrite its immutable permission or reauthorize it.
	s.projectRuns.mu.Lock()
	run, err = s.projectRuns.mutate(p.ID, p.Path, run.ID, run.Revision, func(row *storedProjectRun) (bool, error) {
		row.Run.Input.Route.PermissionMode = "ask"
		row.Run.Swarm.State = "paused"
		row.InputDigest = reviewDigest(struct {
			Input             ProjectRunInput
			CodexConnectionID string
		}{row.Run.Input, row.CodexConnectionID})
		return true, nil
	})
	s.projectRuns.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, s.stateDir)
	for _, route := range []struct {
		path     string
		revision int
	}{
		{control + "/resume", items[0].Revision + 1},
		{base + "/resume", run.Revision},
	} {
		w := journeyRequest(t, mux, "POST", route.path, map[string]int{"expectedRevision": route.revision}, 409)
		if !strings.Contains(w.Body.String(), `"task_permission_requires_interactive"`) {
			t.Fatal("resume lost the unsupported permission explanation", w.Body.String())
		}
	}
	if *starts != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("unsupported resume altered consent or started a process")
	}
}
