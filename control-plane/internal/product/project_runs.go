package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// ProjectRunView is a projection of private history. Reading it never observes
// providers, repairs state, starts work, or changes an earlier acceptance.
type ProjectRunView struct {
	ProjectRun
	AllowedActions   []string                   `json:"allowedActions"`
	ContextFreshness ProjectRunContextFreshness `json:"contextFreshness"`
	CodingBlocker    *ProjectRunCodingBlocker   `json:"codingBlocker,omitempty"`
}

// Context freshness describes only the saved reviewed inputs. It does not
// observe the source, model route, children or release, or rewrite history.
type ProjectRunContextFreshness struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// A separate conversation decision can block further coding even when the
// saved reviewed documents still match. Absence is not full coding admission.
type ProjectRunCodingBlocker struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	TurnID string `json:"turnId,omitempty"`
}

func projectRunCodingBlocker(ledger orchestratorLedger, readErr error) *ProjectRunCodingBlocker {
	if readErr != nil {
		return &ProjectRunCodingBlocker{Kind: "unavailable", Reason: "Project decisions could not be read. Open the project conversation and refresh it before starting more coding."}
	}
	if questions := orchestratorQuestions(ledger); len(questions) > 0 {
		return &ProjectRunCodingBlocker{Kind: "question", TurnID: questions[0].TurnID, Reason: "Resolve this decision before starting more coding: " + questions[0].Question}
	}
	return nil
}

func (s *Server) SetProjectRunSelectionResolver(resolve func(context.Context, ProjectRunSelectionRequest) (ProjectRunSelection, error)) {
	s.projectRunSelectionResolver = resolve
}

func (s *Server) registerProjectRuns(mux Registrar) {
	s.registerProjectSwarm(mux)
	s.registerOrchestrationContracts(mux)
	const base = "/v1/product/projects/{id}/runs"
	mux.HandleFunc("GET "+base, s.listProjectRuns)
	mux.HandleFunc("POST "+base, s.createProjectRun)
	mux.HandleFunc("GET "+base+"/{runId}", s.getProjectRun)
	mux.HandleFunc("POST "+base+"/{runId}/operations", s.reserveProjectRunOperation)
	for _, action := range []string{"reconcile", "cancel", "checks", "releases", "accept"} {
		mux.HandleFunc("POST "+base+"/{runId}/"+action, s.mutateProjectRun)
	}
}

func projectRunProblem(w http.ResponseWriter, err error) {
	status, code := http.StatusConflict, "project_run_conflict"
	message := err.Error()
	var pathError *os.PathError
	var linkError *os.LinkError
	if errors.Is(err, harness.ErrTaskPermission) {
		code = "task_permission_requires_interactive"
	} else if errors.Is(err, ErrProjectRunNotFound) {
		status, code = http.StatusNotFound, "project_run_not_found"
	} else if errors.Is(err, ErrProjectRunInvalid) || errors.As(err, &pathError) || errors.As(err, &linkError) {
		status, code = http.StatusInternalServerError, "project_run_unavailable"
		message = "Private project run history is unavailable. Check the workspace's storage, then refresh run history before continuing. Do not repeat an unconfirmed action."
	}
	journeyError(w, status, code, message)
}

func (s *Server) projectRunView(run ProjectRun) ProjectRunView {
	var state journeyState
	var ledger orchestratorLedger
	p, err := s.resolveProject(run.ProjectID)
	ledgerErr := err
	if err == nil {
		s.mu.RLock()
		state, err = s.readApprovedJourney(p.Path, p.ID)
		ledger, ledgerErr = s.readOrchestrator(p)
		s.mu.RUnlock()
	}
	return projectRunViewWithOriginals(run, s.projectRunOriginalChecks(run.ProjectID), projectRunContextFreshness(run, state, err), projectRunCodingBlocker(ledger, ledgerErr))
}

func projectRunContextFreshness(run ProjectRun, state journeyState, readErr error) ProjectRunContextFreshness {
	if readErr != nil {
		return ProjectRunContextFreshness{Status: "unavailable", Reason: "The current reviewed context could not be read. Check the project's context storage, then refresh history before starting more coding."}
	}
	saved := &run.Input.Implementation
	if saved.ProjectID != run.ProjectID || saved.WorktreeID != run.WorktreeID {
		return ProjectRunContextFreshness{Status: "changed", Reason: "This run's reviewed inputs belong to a different project or worktree. Review the current plan and create a new run."}
	}
	current, err := implementationSnapshot(state, *ImplementationContextSelection(saved), run.WorktreeID)
	if err != nil {
		return ProjectRunContextFreshness{Status: "changed", Reason: err.Error()}
	}
	previous := saved.Clone()
	previous.SourceCommit, previous.SourceDigest, previous.SourceDirty = "", "", false
	if implementationDigest(current) != implementationDigest(previous) {
		return ProjectRunContextFreshness{Status: "changed", Reason: "The saved requirements, design or implementation plan changed. Review the current plan and create a new run for more coding."}
	}
	approvals, err := projectRunApprovalSelection(state)
	if err != nil {
		return ProjectRunContextFreshness{Status: "changed", Reason: err.Error()}
	}
	if approvals != run.Input.Approvals {
		return ProjectRunContextFreshness{Status: "changed", Reason: "The definition or design was reviewed again after this run was prepared. Review the current plan and create a new run for more coding."}
	}
	return ProjectRunContextFreshness{Status: "current", Reason: "The saved reviewed context still matches. Source, model availability and running operations are checked separately."}
}

func (s *Server) projectRunOriginalChecks(projectID string) map[string]bool {
	s.mu.RLock()
	rows, err := s.readCheckHistory(projectID)
	s.mu.RUnlock()
	available := make(map[string]bool, len(rows))
	if err == nil {
		for _, row := range rows {
			available[row.ID] = true
		}
	}
	return available
}

func projectRunViewWithOriginals(run ProjectRun, originals map[string]bool, freshness ProjectRunContextFreshness, blocker *ProjectRunCodingBlocker) ProjectRunView {
	run.Operations = append([]ProjectRunOperation{}, run.Operations...)
	for i := range run.Operations {
		run.Operations[i] = publicSwarmOperation(run.Operations[i])
	}
	actions := []string{"reconcile"}
	currentInputs := freshness.Status == "current" && blocker == nil
	if run.Swarm != nil && !swarmAllIntegrated(run) {
		return ProjectRunView{ProjectRun: run, AllowedActions: []string{}, ContextFreshness: freshness, CodingBlocker: blocker}
	}
	if run.State != "complete" && run.State != "cancelled" {
		actions = append(actions, "cancel")
		pending := false
		for _, op := range run.Operations {
			if op.Status == "reserved" || op.Status == "dispatching" || op.Status == "running" || op.Status == "unknown" {
				pending = true
			}
			if op.Status == "reserved" && currentInputs && run.Swarm == nil {
				actions = append(actions, "continue-coding")
			}
		}
		if !pending && currentInputs {
			if run.Swarm == nil {
				actions = append(actions, "start-coding")
			}
			actions = append(actions, "join-check")
		}
		if !pending && currentInputs && len(run.Checks) > 0 {
			actions = append(actions, "join-release")
		}
		if !pending && currentInputs && len(run.Checks) > 0 && len(run.Releases) > 0 && run.Observation != nil && run.Observation.Valid {
			actions = append(actions, "accept")
		}
	}
	for i := range run.Checks {
		run.Checks[i].OriginalAvailable = originals[run.Checks[i].CheckID]
	}
	return ProjectRunView{ProjectRun: run, AllowedActions: actions, ContextFreshness: freshness, CodingBlocker: blocker}
}

func (s *Server) writeProjectRun(w http.ResponseWriter, status int, run ProjectRun) {
	journeyJSON(w, status, map[string]any{"run": s.projectRunView(run)})
}

func (s *Server) listProjectRuns(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	runs, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	views := make([]ProjectRunView, 0, len(runs))
	originals := s.projectRunOriginalChecks(p.ID)
	// Read one owned context snapshot for the entire list, not source or model
	// state per historical run. An unreadable snapshot must not hide history.
	s.mu.RLock()
	state, contextErr := s.readApprovedJourney(p.Path, p.ID)
	ledger, ledgerErr := s.readOrchestrator(p)
	s.mu.RUnlock()
	blocker := projectRunCodingBlocker(ledger, ledgerErr)
	for _, run := range runs {
		views = append(views, projectRunViewWithOriginals(run, originals, projectRunContextFreshness(run, state, contextErr), blocker))
	}
	journeyJSON(w, http.StatusOK, map[string]any{"runs": views})
}

func (s *Server) getProjectRun(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.writeProjectRun(w, http.StatusOK, run)
}

type projectRunCreateRequest struct {
	NativeDelegation string `json:"nativeDelegation,omitempty"`
	IdempotencyKey        string                          `json:"idempotencyKey"`
	WorktreeID            string                          `json:"worktreeId"`
	Harness               string                          `json:"harness"`
	Model                 string                          `json:"model"`
	PermissionMode        string                          `json:"permissionMode"`
	Effort                string                          `json:"effort,omitempty"`
	ExecutionLocation     string                          `json:"executionLocation,omitempty"`
	AgentConnectionID     *string                         `json:"agentConnectionId,omitempty"`
	Outcome               string                          `json:"outcome"`
	ImplementationContext ImplementationContextRequest    `json:"implementationContext"`
	OrchestrationContract *OrchestrationContractSelection `json:"orchestrationContract,omitempty"`
}

func (s *Server) createProjectRun(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req projectRunCreateRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	req.WorktreeID, req.Harness, req.Model = strings.TrimSpace(req.WorktreeID), strings.TrimSpace(req.Harness), strings.TrimSpace(req.Model)
	req.PermissionMode, req.Effort = strings.TrimSpace(req.PermissionMode), strings.TrimSpace(req.Effort)
	req.ExecutionLocation = strings.TrimSpace(req.ExecutionLocation)
	if req.ExecutionLocation == "local-docker" {
		req.ExecutionLocation = "" // preserve the historical local route digest
	}
	if req.Harness == "" {
		req.Harness = "claude-code"
	}
	if req.PermissionMode == "" {
		req.PermissionMode = "ask"
	}
	if !validRunKey(req.IdempotencyKey) || req.WorktreeID == "" || req.ImplementationContext.ProjectID != p.ID || (req.Outcome != "preview" && req.Outcome != "publish") {
		journeyError(w, http.StatusBadRequest, "invalid_project_run", "Choose this project's saved tasks, worktree, intended outcome and a fresh request key.")
		return
	}
	if req.ExecutionLocation != "" && req.ExecutionLocation != "cloud-microvm" {
		journeyError(w, http.StatusBadRequest, "invalid_execution_location", "Choose local Docker or a configured cloud microVM for this reviewed run.")
		return
	}
	if req.ExecutionLocation == "cloud-microvm" {
		if req.PermissionMode != "bypass" || req.Harness != "codex" && req.Harness != "claude-code" {
			journeyError(w, http.StatusBadRequest, "unsupported_cloud_harness", "Cloud microVM tasks currently require an explicit noninteractive Codex or Claude Code run.")
			return
		}
		if s.cloudMicroVMAvailable == nil || !s.cloudMicroVMAvailable() {
			journeyError(w, http.StatusServiceUnavailable, "cloud_microvm_unavailable", "A verified cloud microVM execution service is not configured for this workspace.")
			return
		}
	}
	// Normalize unordered task selection, never trust a digest from the client.
	sort.Strings(req.ImplementationContext.TaskIDs)
	key := req.IdempotencyKey
	req.IdempotencyKey = ""
	digest := reviewDigest(req)
	if run, found, err := s.projectRuns.FindByKey(p.ID, p.Path, key, digest); err != nil {
		projectRunProblem(w, err)
		return
	} else if found {
		s.writeProjectRun(w, http.StatusOK, run)
		return
	}
	if err := s.OrchestratorExecutionGate(p.ID); err != nil {
		projectRunProblem(w, err)
		return
	}
	if s.projectRunSelectionResolver == nil {
		projectRunProblem(w, errors.New("The coding session service is unavailable."))
		return
	}
	selected, err := s.projectRunSelectionResolver(r.Context(), ProjectRunSelectionRequest{ProjectID: p.ID, WorktreeID: req.WorktreeID, Harness: req.Harness, Model: req.Model, PermissionMode: req.PermissionMode, Effort: req.Effort, AgentConnectionID: req.AgentConnectionID})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if selected.Connection.Kind == "legacy" && !(selected.AuthMode == "chatgpt" && selected.CodexConnectionID != "") {
		projectRunProblem(w, errors.New("Save a model connection in Settings before creating a durable run. Its account and revision must remain identifiable."))
		return
	}
	release, err := s.AcquireGitSessionAdmission(r.Context(), p.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	defer release()
	implementation, err := s.BindImplementationContext(r.Context(), req.ImplementationContext, selected.WorktreeID, selected.Path)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if len(implementation.RequirementIDs) == 0 {
		projectRunProblem(w, errors.New("Link the selected tasks to reviewed requirements before creating a run."))
		return
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	var approvals ProjectRunApprovalSelection
	var binding projectRequirementBinding
	if err == nil {
		approvals, err = projectRunApprovalSelection(state)
	}
	if err == nil {
		binding, err = bindProjectRequirements(state, p.ID, implementation.RequirementIDs, &implementation.SpecificationRevision, &implementation.DesignPlanRevision, true)
	}
	s.mu.RUnlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	input := ProjectRunInput{Implementation: *implementation, Approvals: approvals, Route: ProjectRunRoute{Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, Effort: selected.Effort, ExecutionLocation: req.ExecutionLocation, Connection: selected.Connection}, Outcome: req.Outcome, Requirements: binding.RequirementSnapshots}
	if err := s.captureNativeDelegation(req.NativeDelegation, &input.Route); err != nil { projectRunProblem(w, err); return }
	if req.NativeDelegation != "" && req.OrchestrationContract == nil { projectRunProblem(w, ErrNativeDelegation); return }
	if req.OrchestrationContract != nil {
		contract, err := s.contractApprovedForRun(r.Context(), p, *req.OrchestrationContract, input)
		if err != nil {
			projectRunProblem(w, err)
			return
		}
		input.OrchestrationContract = contract
	}
	run, created, err := s.projectRuns.Create(p.ID, p.Path, key, input, selected.CodexConnectionID, digest)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	s.writeProjectRun(w, status, run)
}

func (s *Server) resolveProjectRunSelection(ctx context.Context, run ProjectRun) (ProjectRunSelection, error) {
	var zero ProjectRunSelection
	if err := ValidateNativeDelegationCurrent(run.Input.Route, s.sandboxImage); err != nil { return zero, err }
	if err := s.OrchestratorExecutionGate(run.ProjectID); err != nil {
		return zero, err
	}
	if s.projectRunSelectionResolver == nil {
		return zero, errors.New("The coding session service is unavailable.")
	}
	route := run.Input.Route
	id := route.Connection.ID
	selected, err := s.projectRunSelectionResolver(ctx, ProjectRunSelectionRequest{ProjectID: run.ProjectID, WorktreeID: run.WorktreeID, Harness: route.Harness, Model: route.Model, PermissionMode: route.PermissionMode, Effort: route.Effort, AgentConnectionID: &id})
	if err != nil {
		return zero, err
	}
	if selected.ProjectID != run.ProjectID || selected.WorktreeID != run.WorktreeID || selected.Harness != route.Harness || selected.Model != route.Model || selected.PermissionMode != route.PermissionMode || selected.Effort != route.Effort || selected.Connection != route.Connection {
		return zero, errors.New("The selected model connection or worktree changed. Restore its saved selection or start a new run.")
	}
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		return zero, err
	}
	generation, err := s.projectRuns.PrivateCodexConnectionID(p.ID, p.Path, run.ID)
	if err != nil {
		return zero, err
	}
	if generation != selected.CodexConnectionID {
		return zero, errors.New("The coding account changed. Create a new run using the current connection.")
	}
	if err := s.ValidateImplementationContext(p.ID, run.WorktreeID, selected.Path, &run.Input.Implementation); err != nil {
		return zero, err
	}
	if err := s.validateProjectRunApprovals(p.ID, run.Input.Approvals); err != nil {
		return zero, err
	}
	if err := s.contractCurrentForRun(p, run); err != nil {
		return zero, err
	}
	return selected, nil
}

func projectRunCandidate(selected ProjectRunSelection, implementation *registry.ImplementationContext) registry.Session {
	return registry.Session{ProjectID: selected.ProjectID, WorktreeID: selected.WorktreeID, Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, Effort: selected.Effort, AuthMode: selected.AuthMode, CodexConnectionID: selected.CodexConnectionID, AgentConnectionID: selected.AgentConnectionID, AgentConnectionRevision: selected.AgentConnectionRevision, AgentConnectionKind: selected.AgentConnectionKind, AgentConnectionProtocol: selected.AgentConnectionProtocol, ImplementationContext: implementation}
}

func (s *Server) reserveProjectRunOperation(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		IdempotencyKey   string `json:"idempotencyKey"`
		Kind             string `json:"kind"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !validRunKey(req.IdempotencyKey) || req.Kind != "session" {
		journeyError(w, http.StatusBadRequest, "invalid_project_run_operation", "Send the current revision, a fresh request key and session operation kind.")
		return
	}
	if run, op, found, err := s.projectRuns.FindOperationByKey(p.ID, p.Path, r.PathValue("runId"), req.IdempotencyKey, req.Kind); err != nil {
		projectRunProblem(w, err)
		return
	} else if found {
		journeyJSON(w, http.StatusOK, map[string]any{"run": s.projectRunView(run), "operation": publicSwarmOperation(op)})
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if run.Revision != req.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	if run.Input.Route.NativeDelegation != "" {
		projectRunProblem(w, errors.New("Native-helper runs require their reviewed isolated swarm task dispatcher, not a direct session operation."))
		return
	}
	selected, err := s.resolveProjectRunSelection(r.Context(), run)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	release, err := s.AcquireGitSessionAdmission(r.Context(), p.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	defer release()
	implementation, err := s.BindImplementationContext(r.Context(), *ImplementationContextSelection(&run.Input.Implementation), selected.WorktreeID, selected.Path)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	candidate := projectRunCandidate(selected, implementation)
	s.projectRunDispatchMu.Lock()
	run, op, created, err := s.projectRuns.ReserveOperation(p.ID, p.Path, run.ID, req.ExpectedRevision, req.IdempotencyKey, req.Kind, ProjectRunSessionRequestDigest(candidate))
	s.projectRunDispatchMu.Unlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	journeyJSON(w, status, map[string]any{"run": s.projectRunView(run), "operation": publicSwarmOperation(op)})
}

func projectRunPending(run ProjectRun) bool {
	for _, op := range run.Operations {
		if op.Status == "reserved" || op.Status == "dispatching" || op.Status == "running" || op.Status == "unknown" {
			return true
		}
	}
	return false
}

func (s *Server) mutateProjectRun(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	action := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		CheckID          string `json:"checkId,omitempty"`
		ReleaseID        string `json:"releaseId,omitempty"`
		Target           string `json:"target,omitempty"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || (action != "checks" && req.CheckID != "") || (action != "releases" && (req.ReleaseID != "" || req.Target != "")) {
		journeyError(w, http.StatusBadRequest, "invalid_project_run_action", "Send the current revision and only this action's selected evidence identity.")
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if run.Revision != req.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	if run.Swarm != nil && !swarmAllIntegrated(run) && action != "cancel" {
		projectRunProblem(w, errors.New("Observe and integrate every swarm task before joining combined verification evidence."))
		return
	}
	if action == "cancel" {
		s.projectRunDispatchMu.Lock()
		run, err = s.projectRuns.Cancel(p.ID, p.Path, run.ID, req.ExpectedRevision)
		s.projectRunDispatchMu.Unlock()
	} else if action == "reconcile" || action == "accept" {
		run, err = s.reconcileProjectRun(r.Context(), p, run)
		if err == nil && action == "accept" {
			if run.Observation == nil || !run.Observation.Valid {
				err = errors.New("The current run evidence needs attention before acceptance.")
			} else {
				run, err = s.projectRuns.Accept(p.ID, p.Path, run.ID, run.Revision)
			}
		}
	} else {
		_, err = s.resolveProjectRunSelection(r.Context(), run)
		if err == nil && projectRunPending(run) {
			err = ErrProjectRunPending
		}
		if err == nil {
			switch action {
			case "checks":
				var check ProjectRunCheckEvidence
				check, err = s.SnapshotProjectRunCheck(r.Context(), p.ID, req.CheckID, run.Input.Approvals, run.Input.Requirements)
				if err == nil {
					run, err = s.projectRuns.JoinCheck(p.ID, p.Path, run.ID, run.Revision, check)
				}
			case "releases":
				if len(run.Checks) == 0 {
					err = errors.New("Join passing checks before selecting their release.")
					break
				}
				var release ProjectRunReleaseEvidence
				release, err = s.SnapshotProjectRunRelease(r.Context(), p.ID, req.ReleaseID, req.Target, run.Input.Outcome, run.Input.Approvals, run.Checks[len(run.Checks)-1])
				if err == nil {
					run, err = s.projectRuns.JoinRelease(p.ID, p.Path, run.ID, run.Revision, release)
				}
			}
		}
	}
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.writeProjectRun(w, http.StatusOK, run)
}

func (s *Server) reconcileProjectRun(ctx context.Context, p project.Project, run ProjectRun) (ProjectRun, error) {
	observation := ProjectRunObservation{State: "ready", Reason: "Reviewed inputs and model selection are current. Start coding when ready.", Valid: true}
	_, validationErr := s.resolveProjectRunSelection(ctx, run)
	// Serialize with actual session dispatch. A missing claimed child is unknown,
	// never a license to invoke the executor a second time.
	s.projectRunDispatchMu.Lock()
	locked := true
	defer func() {
		if locked {
			s.projectRunDispatchMu.Unlock()
		}
	}()
	current, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		return ProjectRun{}, err
	}
	if current.Revision != run.Revision {
		return ProjectRun{}, ErrProjectRunConflict
	}
	for _, op := range run.Operations {
		if op.Status == "reserved" {
			observation.State, observation.Reason = "ready", "A coding session is reserved but has not been dispatched. Continue that reserved operation or cancel this run."
			continue
		}
		if projectRunOperationTerminal(op.Status) {
			continue
		}
		status, message := "unknown", "The coding child's outcome is unknown. Inspect its retained session; this operation will not be replayed."
		if s.projectRunSessionObserver != nil {
			children, observeErr := s.projectRunSessionObserver(ctx, run.ProjectID, run.ID, op.ID)
			if observeErr != nil {
				message = "The coding session could not be observed. Use Check run status again when its workspace is available."
			} else if len(children) == 1 {
				child := children[0]
				if child.ID == op.ChildID && child.ProjectID == run.ProjectID && child.WorktreeID == run.WorktreeID && child.ProjectRunID == run.ID && child.ProjectRunOperationID == op.ID && ProjectRunSessionRequestDigest(child) == op.RequestDigest {
					switch child.Status {
					case registry.StatusStarting:
						status, message = "running", "The coding sandbox is still starting; agent execution has not been confirmed. Wait for startup recovery or explicitly stop this session before trying again."
					case registry.StatusRunning, registry.StatusIdle:
						status, message = "running", "The coding session is active. Its output is not verification of the selected requirements."
					case registry.StatusExited:
						status, message = "cancelled", "The coding session ended. Its outcome remains unverified; run and join checks for the candidate source."
						if child.ExitReason == "project-run-outcome-unknown" || child.ExitReason == "sandbox disappeared" {
							status, message = "unknown", "The coding runtime is missing or its allocation was interrupted. Its outcome is unknown; inspect the retained session before continuing."
						} else if child.ExitReason != "" && child.ExitReason != "killed" && child.ExitReason != "exited" && child.ExitReason != "process exited" && child.ExitReason != "idle timeout" {
							status, message = "failed", "The coding session ended with a recorded failure. Inspect its session before starting a new attempt."
						}
					}
				} else {
					message = "The recorded child does not match this operation. Resolve its conflicting identity before continuing."
				}
			} else if len(children) > 1 {
				message = "Multiple sessions claim this operation. Resolve their conflicting identities before continuing."
			}
		}
		run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, status, message)
		if err != nil {
			return ProjectRun{}, err
		}
		observation.OperationID, observation.ChildID, observation.ChildStatus = op.ID, op.ChildID, status
		observation.State, observation.Reason = run.State, message
		if status == "unknown" || status == "failed" {
			observation.Valid = false
		}
	}
	// Evidence inspection takes Git admission. Release the dispatch lock first
	// to preserve the session handler's Git -> dispatch lock order. The store's
	// revision check below rejects any intervening mutation.
	s.projectRunDispatchMu.Unlock()
	locked = false
	if !projectRunPending(run) {
		observation.State, observation.Reason = "awaiting-checks", "Join passing checks for the current clean candidate and selected requirements."
		if len(run.Operations) == 0 && len(run.Checks) == 0 {
			observation.State, observation.Reason = "ready", "Reviewed inputs and model selection are current. Start coding or join existing verified work."
		}
		if len(run.Checks) > 0 {
			check := run.Checks[len(run.Checks)-1]
			if err := s.validateProjectRunCandidate(ctx, p.ID, check); err != nil {
				validationErr = err
			} else {
				observation.State, observation.Reason = "awaiting-release", "Checks match the current candidate. Join its observed release."
				if len(run.Releases) > 0 {
					release := run.Releases[len(run.Releases)-1]
					if _, err := s.SnapshotProjectRunRelease(ctx, p.ID, release.ReleaseID, release.Target, run.Input.Outcome, run.Input.Approvals, check); err != nil {
						validationErr = err
					} else {
						observation.Reason = "Checks and the freshly observed release match. Review the application and accept the selected outcome."
					}
				}
			}
		}
	}
	if len(run.Operations) > 0 {
		last := run.Operations[len(run.Operations)-1]
		if last.Status == "failed" {
			newEvidence := false
			if len(run.Checks) > 0 {
				joined, _ := time.Parse(time.RFC3339Nano, run.Checks[len(run.Checks)-1].JoinedAt)
				failed, _ := time.Parse(time.RFC3339Nano, last.UpdatedAt)
				newEvidence = joined.After(failed)
			}
			if !newEvidence {
				observation.State, observation.Valid, observation.Reason = "blocked", false, "The latest coding attempt failed. Inspect its session, explicitly retry, or join newly verified candidate checks."
			}
		}
	}
	// Settings and private review authority may change while a release health
	// observation is in flight. Revalidate after that bounded external work.
	if _, err := s.resolveProjectRunSelection(ctx, run); err != nil {
		validationErr = err
	}
	if validationErr != nil {
		observation.State, observation.Valid, observation.Reason = "blocked", false, validationErr.Error()
	}
	if observation.State == "complete" || observation.State == "cancelled" {
		observation.State = "awaiting-checks"
	}
	return s.projectRuns.Observe(p.ID, p.Path, run.ID, run.Revision, observation)
}
