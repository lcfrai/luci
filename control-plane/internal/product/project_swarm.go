package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func (s *Server) registerProjectSwarm(mux Registrar) {
	const base = "/v1/product/projects/{id}/runs/{runId}/swarm"
	mux.HandleFunc("POST "+base, s.initializeProjectSwarm)
	for _, action := range []string{"pause", "resume", "cancel", "reconcile"} {
		mux.HandleFunc("POST "+base+"/"+action, s.controlProjectSwarm)
	}
	mux.HandleFunc("POST "+base+"/tasks/{taskId}/reserve", s.reserveProjectSwarmTask)
	mux.HandleFunc("GET "+base+"/tasks/{taskId}/review", s.reviewProjectSwarmTask)
	mux.HandleFunc("POST "+base+"/tasks/{taskId}/integrate", s.integrateProjectSwarmTask)
	mux.HandleFunc("POST "+base+"/tasks/{taskId}/abandon", s.abandonProjectSwarmTask)
	mux.HandleFunc("GET "+base+"/operations/{operationId}/workspace", s.inspectProjectSwarmWorkspace)
	mux.HandleFunc("GET "+base+"/operations/{operationId}/workspace/file", s.readProjectSwarmWorkspaceFile)
}
func (s *Server) swarmPath(projectID, runID, operationID string) (string, error) {
	if !prototypeReleaseID.MatchString(projectID) || !validRunID(runID) || !strings.HasPrefix(operationID, "op-") || !prototypeReleaseID.MatchString(operationID) {
		return "", ErrProjectRunInvalid
	}
	root := s.projectRoot()
	parts := []string{".luci-swarms", projectID, runID, operationID}
	relative := ""
	for _, part := range parts {
		relative = filepath.Join(relative, part)
		_, err := guardedProjectPath(root, relative, true)
		if err != nil {
			return "", err
		}
		if _, err := os.Lstat(filepath.Join(root, relative)); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, filepath.Join(parts...)), nil
}
func (s *Server) initializeProjectSwarm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int `json:"expectedRevision"`
		MaxParallel      int `json:"maxParallel"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.MaxParallel == 0 {
		req.MaxParallel = 2
	}
	if req.MaxParallel < 1 || req.MaxParallel > 4 {
		projectRunProblem(w, errors.New("Choose between one and four parallel task slots."))
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if swarmSubscription(run) && req.MaxParallel > 1 {
		projectRunProblem(w, errors.New("This saved ChatGPT subscription permits one active coding session. Choose one slot, or create a run using an API model connection for parallel workers."))
		return
	}
	if _, err = s.resolveProjectRunSelection(r.Context(), run); err != nil {
		projectRunProblem(w, err)
		return
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	selected := map[string]bool{}
	for _, id := range run.Input.Implementation.TaskIDs {
		selected[id] = true
	}
	tasks := []ProjectSwarmTask{}
	for _, task := range currentPlan(state, p.ID).Tasks {
		if !selected[task.ID] {
			continue
		}
		for _, dep := range task.DependsOn {
			if !selected[dep] {
				projectRunProblem(w, errors.New("Include every dependency in this reviewed run's task selection before starting a swarm."))
				return
			}
		}
		tasks = append(tasks, ProjectSwarmTask{ID: task.ID, Title: task.Title, DependsOn: append([]string{}, task.DependsOn...), WritePaths: append([]string{}, task.WritePaths...), Acceptance: task.Acceptance})
	}
	if len(tasks) != len(selected) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	if err = s.validateSwarmGit(r.Context(), p.Path); err != nil {
		projectRunProblem(w, err)
		return
	}
	digest, commit, dirty, err := s.checkSource(r.Context(), p.Path)
	if err != nil || dirty || !deliverySHA.MatchString(commit) || digest != run.Input.Implementation.SourceDigest || commit != run.Input.Implementation.SourceCommit {
		projectRunProblem(w, errors.New("Commit the reviewed starting source and create a fresh run before preparing isolated tasks."))
		return
	}
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, req.ExpectedRevision, func(run *ProjectRun) error {
		if run.Swarm != nil || len(run.Operations) > 0 || len(run.Checks) > 0 || run.State != "ready" {
			return ErrProjectRunConflict
		}
		run.Swarm = &ProjectSwarm{State: "ready", MaxParallel: req.MaxParallel, Tasks: tasks}
		run.Reason = "Reviewed tasks are prepared for isolated execution. Dependencies require reviewed integration."
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.writeProjectRun(w, 201, run)
}
func (s *Server) controlProjectSwarm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	action := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil || run.Revision != req.ExpectedRevision || run.Swarm == nil {
		if err == nil {
			err = ErrProjectRunConflict
		}
		projectRunProblem(w, err)
		return
	}
	if action == "reconcile" {
		run, err = s.observeProjectSwarm(r.Context(), p.Path, run)
	} else {
		run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, req.ExpectedRevision, func(run *ProjectRun) error {
			if run.Swarm.State == "cancelled" || run.State == "complete" {
				return ErrProjectRunConflict
			}
			switch action {
			case "pause":
				run.Swarm.State = "paused"
				run.Reason = "Future dispatch is paused. Running agents continue; use each session's stop control to stop its process."
			case "resume":
				run.Swarm.State = "ready"
				run.Reason = "Dispatch is enabled. Reserve ready tasks deliberately; no work was replayed."
			case "cancel":
				run.Swarm.State = "cancelled"
				run.State = "cancelled"
				run.Reason = "Future dispatch is cancelled. Running agents and retained changes require explicit stop and review."
				for i := range run.Operations {
					if run.Operations[i].Status == "reserved" && run.Operations[i].Allocation == "ready" {
						run.Operations[i].Status = "cancelled"
					}
				}
			}
			return nil
		})
	}
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.writeProjectRun(w, 200, run)
}
func (s *Server) reserveProjectSwarmTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		IdempotencyKey   string `json:"idempotencyKey"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	// Recover immutable reservations before any mutable provider or source read.
	run, op, found, err := s.projectRuns.FindOperationByKey(p.ID, p.Path, r.PathValue("runId"), req.IdempotencyKey, "session")
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if found {
		if op.TaskID != r.PathValue("taskId") {
			projectRunProblem(w, ErrProjectRunKeyConflict)
			return
		}
		journeyJSON(w, 200, map[string]any{"run": s.projectRunView(run), "operation": op})
		return
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	selected, err := s.resolveProjectRunSelection(r.Context(), run)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	guard := s.branchGuard(p.ID)
	if !guard.TryLock() {
		projectRunProblem(w, errors.New("Project source is busy. Wait before reserving this task."))
		return
	}
	defer guard.Unlock()
	snapshot, err := s.swarmSourceSnapshot(r.Context(), p)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if !snapshot.CanSwitch {
		projectRunProblem(w, errors.New(snapshot.Message))
		return
	}
	if err = s.validateSwarmGit(r.Context(), p.Path); err != nil {
		projectRunProblem(w, err)
		return
	}
	// Every worker starts from a clean current integration checkout. Prior
	// integrations are ancestors; arbitrary source movement requires a new run.
	_, head, dirty, err := s.checkSource(r.Context(), p.Path)
	if err != nil || dirty {
		projectRunProblem(w, errors.New("Commit or preserve the main checkout's pending edits before reserving an isolated task."))
		return
	}
	if run.Swarm == nil {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	expected := swarmExpectedHead(run)
	if head != expected {
		projectRunProblem(w, errors.New("The main checkout moved outside this swarm's reviewed integrations. Prepare a fresh run for the changed source."))
		return
	}
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	run, op, created, err := s.projectRuns.ReserveSwarmTask(p.ID, p.Path, run.ID, req.ExpectedRevision, req.IdempotencyKey, r.PathValue("taskId"), head, func(id string) string { path, _ := s.swarmPath(p.ID, run.ID, id); return project.WorktreeID(path) })
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if !created {
		journeyJSON(w, 200, map[string]any{"run": s.projectRunView(run), "operation": op})
		return
	}
	path, allocationErr := s.swarmPath(p.ID, run.ID, op.ID)
	if allocationErr == nil {
		parent := filepath.Dir(path)
		allocationErr = os.MkdirAll(parent, 0700)
		if allocationErr == nil {
			_, allocationErr = hostpath.Confine(s.projectRoot(), parent)
		}
	}
	if allocationErr == nil {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			allocationErr = errors.New("The reserved worktree path already exists; it was not adopted.")
		}
	}
	if allocationErr == nil {
		allocationErr = s.createSwarmCheckout(r.Context(), p.Path, path, head)
	}
	var binding *registry.ImplementationContext
	if allocationErr == nil {
		binding, allocationErr = s.swarmTaskBinding(r.Context(), run, op, path)
	}
	selected.WorktreeID = op.WorktreeID
	candidate := projectRunCandidate(selected, binding)
	run, err = s.projectRuns.CompleteSwarmAllocation(p.ID, p.Path, run.ID, run.Revision, op.ID, binding, candidate, allocationErr)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	op = *swarmOperation(&run, op.ID)
	journeyJSON(w, 201, map[string]any{"run": s.projectRunView(run), "operation": op})
}
func (s *Server) swarmTaskBinding(ctx context.Context, run ProjectRun, op ProjectRunOperation, path string) (*registry.ImplementationContext, error) {
	capture, err := s.swarmTaskCapture(ctx, run, op, path)
	if err != nil {
		return nil, err
	}
	return capture.Binding, nil
}
func (s *Server) swarmTaskCapture(ctx context.Context, run ProjectRun, op ProjectRunOperation, path string) (*ReviewedContextCapture, error) {
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	selection := *ImplementationContextSelection(&run.Input.Implementation)
	selection.TaskIDs = []string{op.TaskID}
	binding, err := implementationSnapshot(state, selection, op.WorktreeID)
	if err != nil {
		return nil, err
	}
	if err = s.validateSwarmTaskGit(ctx, path); err != nil {
		return nil, err
	}
	binding.SourceDigest, binding.SourceCommit, binding.SourceDirty, err = s.checkSource(ctx, path)
	if err != nil || binding.SourceDirty || binding.SourceCommit != op.BaseCommit {
		return nil, errors.New("The isolated starting source no longer matches its reserved base.")
	}
	return captureReviewedContext(state, binding)
}

// Called only by a reserved run operation. Ordinary reviewed sessions retain
// their main-checkout rule, and clients cannot nominate another linked path.
func (s *Server) CaptureProjectRunTaskContext(ctx context.Context, projectID, runID, operationID, worktreeID, path string, selection ImplementationContextRequest) (*ReviewedContextCapture, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return nil, err
	}
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return nil, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.TaskID == "" {
		return s.CaptureImplementationContext(ctx, selection, worktreeID, path)
	}
	expected, err := s.swarmPath(projectID, runID, operationID)
	if err != nil || filepath.Clean(path) != expected || worktreeID != op.WorktreeID || op.Allocation != "ready" || op.Implementation == nil || !reflect.DeepEqual(*ImplementationContextSelection(op.Implementation), selection) {
		return nil, errors.New("The selected task does not match its owned isolated worktree.")
	}
	if run.Swarm.State != "ready" {
		return nil, errors.New("This swarm is paused or cancelled. No new child was started.")
	}
	if err = s.OrchestratorExecutionGate(projectID); err != nil {
		return nil, err
	}
	if err = s.validateProjectRunApprovals(projectID, run.Input.Approvals); err != nil {
		return nil, err
	}
	capture, err := s.swarmTaskCapture(ctx, run, *op, path)
	if err != nil || !reflect.DeepEqual(capture.Binding, op.Implementation) {
		return nil, errors.New("The reserved task source or reviewed context changed. Inspect this attempt before continuing.")
	}
	return capture, nil
}
func (s *Server) observeProjectSwarm(ctx context.Context, path string, run ProjectRun) (ProjectRun, error) {
	if s.projectRunSessionObserver == nil {
		return run, errors.New("The session observer is unavailable.")
	}
	for _, op := range run.Operations {
		if op.Allocation != "ready" || op.Status == "reserved" || op.Status == "succeeded" || op.Status == "failed" || op.Status == "cancelled" {
			continue
		}
		children, err := s.projectRunSessionObserver(ctx, run.ProjectID, run.ID, op.ID)
		if err != nil {
			return run, err
		}
		status, message := "unknown", "The reserved child cannot be identified. Its outcome is unknown; no replacement was launched."
		if len(children) == 1 && children[0].ID == op.ChildID && projectRunSessionBindingMatches(run, op, children[0]) {
			child := children[0]
			status, message = "running", "The existing agent is running in its isolated checkout."
			if child.Status == registry.StatusExited {
				status, message = "succeeded", "The agent exited. Review committed changes and check the result before integration; exit does not establish correctness."
				if child.ExitReason == "project-run-outcome-unknown" {
					status = "unknown"
				} else if child.ExitReason != "exited" && child.ExitReason != "" {
					status = "failed"
					message = "The agent stopped without a confirmed successful result. Retain and inspect its checkout before retrying."
				}
			}
		}
		run, err = s.projectRuns.ObserveOperation(run.ProjectID, path, run.ID, run.Revision, op.ID, op.ChildID, status, message)
		if err != nil {
			return run, err
		}
	}
	for _, task := range run.Swarm.Tasks {
		if task.Integration == nil || task.Integration.State == "integrated" || task.Integration.Commit == "" {
			continue
		}
		_, head, dirty, err := s.checkSource(ctx, path)
		if err != nil {
			return run, err
		}
		if head != task.Integration.Commit || dirty {
			continue
		}
		run, err = s.projectRuns.mutateSwarm(run.ProjectID, path, run.ID, run.Revision, func(current *ProjectRun) error {
			receipt := swarmTask(current, task.ID).Integration
			receipt.State = "integrated"
			receipt.IntegratedAt = time.Now().UTC().Format(time.RFC3339Nano)
			current.Reason = "The retained integration is exactly the current clean source. Its receipt was recovered without rerunning Git."
			current.State = "ready"
			if swarmAllIntegrated(*current) {
				current.State = "awaiting-checks"
			}
			return nil
		})
		if err != nil {
			return run, err
		}
	}
	return run, nil
}

// Each task owns complete Git metadata and copied objects: no shared writable
// refs, host .git pointer, alternates or hardlinks enter the worker sandbox.
func (s *Server) createSwarmCheckout(ctx context.Context, source, destination, commit string) error {
	if err := os.Mkdir(destination, 0700); err != nil {
		return err
	}
	format, err := s.swarmGit(ctx, source, "rev-parse", "--show-object-format")
	format = strings.TrimSpace(format)
	if err != nil || (format != "sha1" && format != "sha256") {
		return errors.New("The source Git object format could not be identified.")
	}
	if _, err := s.swarmGit(ctx, destination, "init", "--initial-branch=luci-task", "--object-format="+format); err != nil {
		return err
	}
	if _, err := s.swarmGit(ctx, destination, "-c", "protocol.file.allow=always", "fetch", "--no-tags", "--no-write-fetch-head", "--", source, commit); err != nil {
		return err
	}
	_, err = s.swarmGit(ctx, destination, "checkout", "--detach", commit)
	return err
}

// The private ledger, not repository-controlled metadata, owns these extra
// checkouts. Wrappers can expose them under their original project authority.
func (s *Server) ProjectSwarmWorktrees(ctx context.Context, projectID string) ([]project.Worktree, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return nil, err
	}
	runs, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil {
		return nil, err
	}
	rows := []project.Worktree{}
	for _, run := range runs {
		if run.Swarm == nil {
			continue
		}
		for _, op := range run.Operations {
			if op.TaskID == "" || op.Allocation != "ready" || op.Status == "cancelled" {
				continue
			}
			task := swarmTask(&run, op.TaskID)
			archived := task != nil && task.OperationID == op.ID && task.Integration != nil && task.Integration.State == "integrated"
			path, err := s.swarmPath(p.ID, run.ID, op.ID)
			if err != nil {
				if archived {
					continue
				}
				return nil, err
			}
			if op.WorktreeID != project.WorktreeID(path) {
				return nil, ErrProjectRunInvalid
			}
			info, err := os.Lstat(filepath.Join(path, ".git"))
			if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				if archived {
					continue
				}
				return nil, ErrProjectRunInvalid
			}
			if _, err := journeyGitBoundary(ctx, s.projectRoot(), path, []string{"status"}); err != nil {
				if archived {
					continue
				}
				return nil, err
			}
			rows = append(rows, project.Worktree{ID: op.WorktreeID, ProjectID: p.ID, Name: "Task " + op.TaskID, Path: path, IsPrimary: false})
		}
	}
	return rows, nil
}

func (s *Server) validateSwarmImplementation(projectID, worktreeID, path string, saved *registry.ImplementationContext) error {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return err
	}
	runs, err := s.projectRuns.List(projectID, p.Path)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Swarm == nil {
			continue
		}
		for _, op := range run.Operations {
			if op.WorktreeID != worktreeID {
				continue
			}
			expected, err := s.swarmPath(projectID, run.ID, op.ID)
			if err != nil || expected != filepath.Clean(path) || op.Allocation != "ready" || !sameRunReviewedInputs(op.Implementation, saved) {
				return errors.New("The task checkout does not match its private run receipt.")
			}
			if run.Swarm.State != "ready" {
				return errors.New("The swarm is paused or cancelled. No further execution was started.")
			}
			return nil
		}
	}
	return errors.New("This linked checkout is not an owned isolated task. Start a task from its reviewed swarm.")
}

func (s *Server) abandonProjectSwarmTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	task := swarmTask(&run, r.PathValue("taskId"))
	if task == nil {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	op := swarmOperation(&run, task.OperationID)
	if op == nil {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	if task.Integration != nil && task.Integration.State == "integrated" {
		projectRunProblem(w, errors.New("An integrated task cannot be discarded. Amend the plan to change delivered work."))
		return
	}
	if op.Status == "dispatching" || op.Status == "running" || op.Status == "unknown" && op.Allocation == "ready" {
		projectRunProblem(w, errors.New("Stop and observe the existing session before retiring its attempt. Unknown launches cannot be replayed."))
		return
	}
	if s.projectRunSessionObserver == nil {
		projectRunProblem(w, errors.New("Session ownership must be available before retiring this attempt."))
		return
	}
	children, err := s.projectRunSessionObserver(r.Context(), p.ID, run.ID, op.ID)
	if err != nil || len(children) > 1 || len(children) == 1 && (children[0].ID != op.ChildID || children[0].Status != registry.StatusExited) {
		projectRunProblem(w, errors.New("The existing child is active or cannot be identified safely. Observe it before retiring this attempt."))
		return
	}
	// Publication may have succeeded before the receipt write was interrupted.
	// Never permit a fresh attempt to duplicate those already delivered changes.
	if task.Integration != nil && task.Integration.Commit != "" {
		head, err := s.swarmGit(r.Context(), p.Path, "rev-parse", "HEAD")
		if err != nil || strings.TrimSpace(head) == task.Integration.Commit {
			projectRunProblem(w, errors.New("Check swarm status first: the retained integration may already be the current source."))
			return
		}
	}
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, req.ExpectedRevision, func(run *ProjectRun) error {
		task := swarmTask(run, r.PathValue("taskId"))
		op := swarmOperation(run, task.OperationID)
		if task.Integration != nil {
			op.Integration = task.Integration
			task.Integration = nil
		}
		op.Status = "cancelled"
		if op.Allocation != "ready" {
			op.Allocation = "abandoned"
		}
		op.Message = "The operator retired this attempt. All task and integration files were retained; a new attempt needs a fresh reservation."
		run.Reason = op.Message
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.writeProjectRun(w, 200, run)
}

func swarmSubscription(run ProjectRun) bool {
	return run.Input.Route.Connection.Kind == "codex-subscription" || run.Input.Route.Harness == "codex" && run.Input.Route.Connection.AuthMode == "chatgpt"
}
func swarmExpectedHead(run ProjectRun) string {
	expected := run.Input.Implementation.SourceCommit
	if run.Swarm == nil {
		return expected
	}
	for pass := 0; pass < len(run.Swarm.Tasks); pass++ {
		for _, task := range run.Swarm.Tasks {
			if task.Integration != nil && task.Integration.State == "integrated" && task.Integration.TargetCommit == expected {
				expected = task.Integration.Commit
			}
		}
	}
	return expected
}

func (s *Server) validateSwarmTaskGit(ctx context.Context, path string) error {
	info, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return errors.New("An isolated task must retain its own ordinary Git directory; linked or redirected metadata cannot be adopted.")
	}
	return s.validateSwarmGit(ctx, path)
}

// Branch switching conservatively considers every project session. Isolated
// dispatch/integration only needs the actual primary writer to be idle; an
// unrelated worker must not prevent a second independent task from starting.
func (s *Server) swarmWorktreeActive(ctx context.Context, projectID, worktreeID string) (bool, error) {
	if s.projectSwarmWorktreeObserver != nil {
		return s.projectSwarmWorktreeObserver(ctx, projectID, worktreeID)
	}
	if s.branchSessions == nil {
		return true, errors.New("Session state is unavailable. Observe project sessions before changing source.")
	}
	return s.branchSessions(ctx, projectID, worktreeID)
}
func (s *Server) swarmSourceSnapshot(ctx context.Context, p project.Project) (BranchCatalogue, error) {
	snapshot, err := s.branchSnapshot(ctx, s.projectRoot(), p)
	if err != nil {
		return snapshot, err
	}
	if snapshot.Commit == "" || snapshot.CurrentBranch == "" || !snapshot.Clean {
		return snapshot, nil
	}
	s.mu.RLock()
	busy := s.deliveryBusy[p.ID]
	for _, job := range s.projectChecks {
		if job.run.ProjectID == p.ID && job.cancel != nil {
			busy = true
			break
		}
	}
	s.mu.RUnlock()
	if busy {
		snapshot.CanSwitch = false
		snapshot.Message = "Wait for this project's checks or delivery operation before changing its source."
		return snapshot, nil
	}
	active, err := s.swarmWorktreeActive(ctx, p.ID, project.WorktreeID(p.Path))
	if err != nil {
		snapshot.CanSwitch = false
		snapshot.Message = "The primary checkout's active sessions could not be checked."
		return snapshot, nil
	}
	snapshot.CanSwitch = !active
	if active {
		snapshot.Message = "Stop the primary checkout's coding session before creating or integrating isolated tasks."
	}
	return snapshot, nil
}
