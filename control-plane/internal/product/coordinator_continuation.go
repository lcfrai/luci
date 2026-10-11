package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const coordinatorContinuationLimit = 40
const coordinatorContinuationBytes = 2 << 20

type CoordinatorAction struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	ReferenceID string `json:"referenceId,omitempty"`
	Message     string `json:"message,omitempty"`
	StartedAt   string `json:"startedAt"`
	FinishedAt  string `json:"finishedAt,omitempty"`
}
type CoordinatorContinuation struct {
	ID             string              `json:"id"`
	IdempotencyKey string              `json:"idempotencyKey"`
	ProjectID      string              `json:"projectId"`
	Actor          ContextActor        `json:"actor"`
	OriginTurnID   string              `json:"originTurnId"`
	ContextDigest  string              `json:"contextDigest"`
	PolicyRevision int                 `json:"policyRevision"`
	Revision       int                 `json:"revision"`
	Status         string              `json:"status"`
	Message        string              `json:"message"`
	RunID          string              `json:"runId,omitempty"`
	QueueIDs       []string            `json:"queueIds"`
	Actions        []CoordinatorAction `json:"actions"`
	CreatedAt      string              `json:"createdAt"`
	UpdatedAt      string              `json:"updatedAt"`
	NextAction     *OrchestratorAction `json:"nextAction,omitempty"`
}
type coordinatorContinuationRecord struct {
	CoordinatorContinuation
	KeyDigest       string                  `json:"keyDigest"`
	RequestDigest   string                  `json:"requestDigest"`
	CanonicalDigest string                  `json:"canonicalDigest"`
	Run             projectRunCreateRequest `json:"run"`
	MaxParallel     int                     `json:"maxParallel"`
}
type coordinatorContinuations struct {
	Version      int                             `json:"version"`
	AccountScope string                          `json:"accountScope"`
	ProjectScope string                          `json:"projectScope"`
	ActorID      string                          `json:"actorId"`
	ProjectID    string                          `json:"projectId"`
	Rows         []coordinatorContinuationRecord `json:"rows"`
}
type coordinatorContinuationRequest struct {
	IdempotencyKey         string                  `json:"idempotencyKey"`
	OriginTurnID           string                  `json:"originTurnId"`
	ExpectedContextDigest  string                  `json:"expectedContextDigest"`
	ExpectedPolicyRevision int                     `json:"expectedPolicyRevision"`
	Run                    projectRunCreateRequest `json:"run"`
	MaxParallel            int                     `json:"maxParallel"`
}

func (s *Server) SetCoordinatorExecutionAuthorizer(fn func(context.Context, string, string, bool) error) {
	s.coordinatorExecuteAuthorize = fn
}
func (s *Server) registerCoordinatorContinuations(mux Registrar) {
	const base = "/v1/product/projects/{id}/orchestrator/continuations"
	mux.HandleFunc("GET "+base, s.listCoordinatorContinuations)
	mux.HandleFunc("POST "+base, s.createCoordinatorContinuation)
	mux.HandleFunc("GET "+base+"/{continuationId}", s.getCoordinatorContinuation)
	mux.HandleFunc("POST "+base+"/{continuationId}/advance", s.advanceCoordinatorContinuation)
}
func (s *Server) continuationPath(id, actor string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) || !validConnectionToken(actor, 128, false) {
		return "", ErrOrchestratorInvalid
	}
	root, err := guardedProjectPath(s.stateDir, "coordinator-continuations", true)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		if runtime.GOOS == "windows" {
			err = restrictPrivateFile(root)
		} else {
			err = os.Chmod(root, 0700)
		}
		if err != nil {
			return "", err
		}
	}
	root, err = guardedProjectPath(s.stateDir, "coordinator-continuations", false)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", ErrOrchestratorInvalid
	}
	path, err := guardedProjectPath(root, id+"-"+reviewDigest(actor)+".json", create)
	if err != nil {
		return "", err
	}
	if err = hostpath.OneName(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}
func (s *Server) readContinuations(p project.Project, actor string) (coordinatorContinuations, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	out := coordinatorContinuations{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ActorID: actor, ProjectID: p.ID, Rows: []coordinatorContinuationRecord{}}
	if err != nil {
		return out, err
	}
	path, err := s.continuationPath(p.ID, actor, false)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	var stored coordinatorContinuations
	if err = externalReadJSON(path, coordinatorContinuationBytes, &stored); errors.Is(err, os.ErrNotExist) {
		return out, nil
	} else if err != nil {
		return out, err
	}
	if stored.Version != 1 || stored.AccountScope != out.AccountScope || stored.ProjectScope != out.ProjectScope || stored.ProjectID != p.ID || stored.ActorID != actor || len(stored.Rows) > coordinatorContinuationLimit {
		return out, ErrOrchestratorInvalid
	}
	seen := map[string]bool{}
	for _, r := range stored.Rows {
		if r.ProjectID != p.ID || r.Actor.ID != actor || !strings.HasPrefix(r.ID, "continue-") || !prototypeReleaseID.MatchString(r.ID) || seen[r.ID] || r.Revision < 1 || !validRunKey(r.IdempotencyKey) || reviewDigest(r.IdempotencyKey) != r.KeyDigest || !hexDigest(r.KeyDigest) || !hexDigest(r.RequestDigest) || !hexDigest(r.CanonicalDigest) || !hexDigest(r.ContextDigest) || r.Run.ImplementationContext.ProjectID != p.ID || r.MaxParallel < 1 || r.MaxParallel > MaxSwarmParallelTasks || len(r.Actions) > 3 {
			return out, ErrOrchestratorInvalid
		}
		seen[r.ID] = true
		for i, a := range r.Actions {
			if a.ID != r.ID+"-"+a.Kind || i >= 3 || a.Kind != []string{"create-run", "prepare-tasks", "queue-tasks"}[i] || (a.Status != "pending" && a.Status != "confirmed" && a.Status != "unknown" && a.Status != "refused") {
				return out, ErrOrchestratorInvalid
			}
		}
	}
	return stored, nil
}
func (s *Server) saveContinuations(row coordinatorContinuations) error {
	raw, err := json.MarshalIndent(row, "", "  ")
	if err != nil || len(raw)+1 > coordinatorContinuationBytes {
		return ErrOrchestratorCapacity
	}
	path, err := s.continuationPath(row.ProjectID, row.ActorID, true)
	if err != nil {
		return err
	}
	return durableRunJSON(path, row)
}
func findContinuation(rows coordinatorContinuations, id string) int {
	for i, r := range rows.Rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}
func (s *Server) continuationPublic(ctx context.Context, row coordinatorContinuationRecord) CoordinatorContinuation {
	out := row.CoordinatorContinuation
	// The durable receipt never pretends that queued tasks were verified or deployed.
	// Present the same authoritative next step used by the project conversation.
	if view, err := s.OrchestratorView(ctx, row.ProjectID, 0); err == nil {
		next := view.NextAction
		if row.RunID != "" && next.RunID != "" && next.RunID != row.RunID {
			next = OrchestratorAction{ID: "review-run", RunID: row.RunID, Label: "Review this build", Reason: "Inspect the exact run retained by this continuation; another build has separate progress.", Lens: "develop"}
		}
		out.NextAction = &next
	}
	return out
}
func (s *Server) listCoordinatorContinuations(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, ok := authenticatedContextActor(r.Context())
	if !ok {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	s.mu.RLock()
	rows, err := s.readContinuations(p, actor.ID)
	s.mu.RUnlock()
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	out := []CoordinatorContinuation{}
	for _, row := range rows.Rows {
		out = append(out, row.CoordinatorContinuation)
	}
	if r.Context().Err() != nil {
		return
	}
	journeyJSON(w, 200, map[string]any{"continuations": out})
}
func (s *Server) getCoordinatorContinuation(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, ok := authenticatedContextActor(r.Context())
	if !ok {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	s.mu.RLock()
	rows, err := s.readContinuations(p, actor.ID)
	s.mu.RUnlock()
	index := findContinuation(rows, r.PathValue("continuationId"))
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	if index < 0 {
		journeyError(w, 404, "continuation_missing", "This continuation is not part of your project conversation.")
		return
	}
	out := s.continuationPublic(r.Context(), rows.Rows[index])
	if r.Context().Err() != nil {
		return
	}
	journeyJSON(w, 200, out)
}
func (s *Server) continuationAuthority(ctx context.Context, p project.Project, row coordinatorContinuationRecord) error {
	if err := s.coordinatorAuthority(ctx, p.ID); err != nil {
		return err
	}
	actor, ok := authenticatedContextActor(ctx)
	if !ok || actor.ID != row.Actor.ID || s.coordinatorExecuteAuthorize == nil {
		return errCoordinatorAuthority
	}
	if err := s.coordinatorExecuteAuthorize(ctx, p.ID, row.Run.Harness, row.Run.AgentConnectionID != nil); err != nil {
		return errCoordinatorAuthority
	}
	return nil
}
func (s *Server) continuationContextLocked(p project.Project, selection ImplementationContextRequest) (string, error) {
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return "", err
	}
	approved, err := projectRunApprovalSelection(state)
	if err != nil {
		return "", err
	}
	snapshot, err := implementationSnapshot(state, selection, "")
	if err != nil {
		return "", err
	}
	return reviewDigest(struct {
		Context   any
		Approvals ProjectRunApprovalSelection
	}{snapshot, approved}), nil
}
func (s *Server) createCoordinatorContinuation(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req coordinatorContinuationRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validRunKey(req.IdempotencyKey) || !hexDigest(req.ExpectedContextDigest) || req.ExpectedPolicyRevision < 1 || !prototypeReleaseID.MatchString(req.OriginTurnID) || req.Run.IdempotencyKey != "" || req.Run.ImplementationContext.ProjectID != p.ID || req.MaxParallel < 1 || req.MaxParallel > MaxSwarmParallelTasks || (req.Run.Outcome != "preview" && req.Run.Outcome != "publish") || req.Run.WorktreeID == "" || req.Run.Harness == "" || req.Run.PermissionMode == "" {
		journeyError(w, 400, "invalid_continuation", "Choose an exact completed turn, current reviewed tasks, saved upstream route and intended outcome.")
		return
	}
	sort.Strings(req.Run.ImplementationContext.TaskIDs)
	actor, ok := authenticatedContextActor(r.Context())
	if !ok {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	requestKey := req.IdempotencyKey
	key := reviewDigest(requestKey)
	req.IdempotencyKey = ""
	requestDigest := reviewDigest(req)
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ID: newID("continue-"), IdempotencyKey: requestKey, ProjectID: p.ID, Actor: actor, OriginTurnID: req.OriginTurnID, ContextDigest: req.ExpectedContextDigest, PolicyRevision: req.ExpectedPolicyRevision, Revision: 1, Status: "ready", Message: "Continuing the reviewed task selection with its saved upstream route.", QueueIDs: []string{}, Actions: []CoordinatorAction{}}, KeyDigest: key, RequestDigest: requestDigest, Run: req.Run, MaxParallel: req.MaxParallel}
	if err := s.continuationAuthority(r.Context(), p, row); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	release, err := s.AcquireGitSessionAdmission(r.Context(), p.ID)
	if err != nil {
		coordinatorPolicyProblem(w, errCoordinatorSourceBusy)
		return
	}
	s.mu.Lock()
	rows, err := s.readContinuations(p, actor.ID)
	if err == nil {
		for _, prior := range rows.Rows {
			if prior.KeyDigest == key {
				if prior.RequestDigest != requestDigest {
					err = ErrOrchestratorConflict
					break
				}
				s.mu.Unlock()
				release()
				out := s.continuationPublic(r.Context(), prior)
				if r.Context().Err() == nil {
					journeyJSON(w, 200, out)
				}
				return
			}
		}
	}
	if err == nil && len(rows.Rows) >= coordinatorContinuationLimit {
		err = ErrOrchestratorCapacity
	}
	if err == nil {
		var policy CoordinatorPolicy
		policy, err = s.readCoordinatorPolicy(p)
		if err == nil && (policy.Revision != req.ExpectedPolicyRevision || policy.ExecutionMode != "reviewed-tasks") {
			err = errCoordinatorPolicy
		}
	}
	if err == nil {
		var saved orchestratorLedger
		saved, err = s.readOrchestrator(p)
		if err == nil {
			found := false
			for _, turn := range saved.Turns {
				if turn.ID == req.OriginTurnID && turn.Actor.ID == actor.ID && turn.Status == "succeeded" && turn.Answer != nil && !turn.Answer.Blocking && (turn.Answer.Proposal == nil || turn.AppliedRevision > 0) {
					found = true
				}
			}
			if !found || len(orchestratorQuestions(saved)) > 0 {
				err = ErrOrchestratorQuestion
			}
		}
	}
	if err == nil {
		var snapshot OrchestratorContext
		snapshot, err = s.orchestratorSnapshotLocked(r.Context(), p)
		if err == nil && snapshot.Digest != req.ExpectedContextDigest {
			err = ErrOrchestratorConflict
		}
	}
	if err == nil {
		row.CanonicalDigest, err = s.continuationContextLocked(p, req.Run.ImplementationContext)
	}
	if err == nil {
		err = s.continuationAuthority(r.Context(), p, row)
	}
	if err == nil {
		row.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		row.UpdatedAt = row.CreatedAt
		rows.Rows = append(rows.Rows, row)
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	release()
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	out, err := s.continueCoordinator(r.Context(), p, row.ID, actor.ID, 0)
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	if r.Context().Err() == nil {
		journeyJSON(w, 201, s.continuationPublic(r.Context(), out))
	}
}
func (s *Server) advanceCoordinatorContinuation(w http.ResponseWriter, r *http.Request) {
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
	actor, ok := authenticatedContextActor(r.Context())
	if !ok || req.ExpectedRevision < 1 {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	out, err := s.continueCoordinator(r.Context(), p, r.PathValue("continuationId"), actor.ID, req.ExpectedRevision)
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	if r.Context().Err() == nil {
		journeyJSON(w, 200, s.continuationPublic(r.Context(), out))
	}
}

// Fixed typed adapters reuse the production handlers and all their current
// validation/admission. Neither a model nor a caller supplies a method or URL.
// The bounded writer is private in-process response capture, not another server.
type coordinatorResponse struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (w *coordinatorResponse) Header() http.Header { return w.header }
func (w *coordinatorResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *coordinatorResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	if w.body.Len()+len(p) > 1<<20 {
		w.overflow = true
		return 0, io.ErrShortBuffer
	}
	return w.body.Write(p)
}
func (s *Server) invokeCoordinatorStep(ctx context.Context, p project.Project, row coordinatorContinuationRecord, kind string) (string, []string, error) {
	var body any
	var handler http.HandlerFunc
	switch kind {
	case "create-run":
		req := row.Run
		req.IdempotencyKey = row.ID + "-create-run"
		body = req
		handler = s.createProjectRun
	case "prepare-tasks":
		run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
		if err != nil {
			return "", nil, err
		}
		body = struct {
			ExpectedRevision int `json:"expectedRevision"`
			MaxParallel      int `json:"maxParallel"`
		}{run.Revision, row.MaxParallel}
		handler = s.initializeProjectSwarm
	case "queue-tasks":
		run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
		if err != nil {
			return "", nil, err
		}
		body = struct {
			ExpectedRevision int      `json:"expectedRevision"`
			IdempotencyKey   string   `json:"idempotencyKey"`
			Priority         string   `json:"priority"`
			TaskIDs          []string `json:"taskIds"`
		}{run.Revision, row.ID + "-queue-tasks", "normal", row.Run.ImplementationContext.TaskIDs}
		handler = s.enqueueProjectSwarmTask
	default:
		return "", nil, ErrOrchestratorInvalid
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	req.SetPathValue("id", p.ID)
	req.SetPathValue("runId", row.RunID)
	req.Header.Set("Content-Type", "application/json")
	release, err := s.AcquireGitSessionAdmission(ctx, p.ID)
	if err != nil {
		return "", nil, &coordinatorStepError{Status: 409, Message: errCoordinatorSourceBusy.Error()}
	}
	defer release()
	if err = s.continuationAuthority(ctx, p, row); err != nil {
		return "", nil, err
	}
	capture := &coordinatorResponse{header: make(http.Header)}
	handler(capture, req)
	if capture.overflow {
		return "", nil, errors.New("The operation response exceeded its capture bound; reconcile the retained operation before continuing.")
	}
	if capture.status < 200 || capture.status >= 300 {
		var failure struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(capture.body.Bytes(), &failure)
		if len(failure.Message) > 4000 {
			failure.Message = failure.Message[:4000]
		}
		return "", nil, &coordinatorStepError{Status: capture.status, Message: failure.Message}
	}
	if kind == "queue-tasks" {
		var result struct {
			Items []SwarmQueueItem `json:"items"`
		}
		if json.Unmarshal(capture.body.Bytes(), &result) != nil || len(result.Items) != len(row.Run.ImplementationContext.TaskIDs) {
			return "", nil, ErrOrchestratorInvalid
		}
		ids := []string{}
		for _, item := range result.Items {
			if item.ProjectID != p.ID || item.RunID != row.RunID {
				return "", nil, ErrOrchestratorInvalid
			}
			ids = append(ids, item.ID)
		}
		return row.RunID, ids, nil
	}
	var result struct {
		Run ProjectRunView `json:"run"`
	}
	if json.Unmarshal(capture.body.Bytes(), &result) != nil || result.Run.ProjectID != p.ID || result.Run.ID == "" || (row.RunID != "" && result.Run.ID != row.RunID) {
		return "", nil, ErrOrchestratorInvalid
	}
	return result.Run.ID, nil, nil
}

type coordinatorStepError struct {
	Status  int
	Message string
}

func (e *coordinatorStepError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "The existing execution operation refused this continuation; inspect its saved run before continuing."
}

func (s *Server) observeCoordinatorStep(p project.Project, row coordinatorContinuationRecord, kind string) (string, []string, bool, error) {
	switch kind {
	case "create-run":
		req := row.Run
		req.IdempotencyKey = ""
		req.WorktreeID = strings.TrimSpace(req.WorktreeID)
		req.Harness = strings.TrimSpace(req.Harness)
		req.Model = strings.TrimSpace(req.Model)
		req.PermissionMode = strings.TrimSpace(req.PermissionMode)
		req.Effort = strings.TrimSpace(req.Effort)
		sort.Strings(req.ImplementationContext.TaskIDs)
		run, found, err := s.projectRuns.FindByKey(p.ID, p.Path, row.ID+"-create-run", reviewDigest(req))
		return run.ID, nil, found, err
	case "prepare-tasks":
		run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
		if err != nil {
			return "", nil, false, err
		}
		if run.Swarm == nil {
			return "", nil, false, nil
		}
		if run.Swarm.MaxParallel != row.MaxParallel || len(run.Swarm.Tasks) != len(row.Run.ImplementationContext.TaskIDs) {
			return "", nil, false, ErrOrchestratorConflict
		}
		wanted := map[string]bool{}
		for _, id := range row.Run.ImplementationContext.TaskIDs {
			wanted[id] = true
		}
		for _, task := range run.Swarm.Tasks {
			if !wanted[task.ID] {
				return "", nil, false, ErrOrchestratorConflict
			}
		}
		return run.ID, nil, true, nil
	case "queue-tasks":
		s.swarmScheduler.mu.Lock()
		defer s.swarmScheduler.mu.Unlock()
		ledger, err := s.readSwarmScheduler()
		if err != nil {
			return "", nil, false, err
		}
		ids := []string{}
		seen := map[string]bool{}
		wanted := map[string]bool{}
		for _, id := range row.Run.ImplementationContext.TaskIDs {
			wanted[id] = true
		}
		for _, item := range ledger.Items {
			if item.BatchKeyDigest != reviewDigest(row.ID+"-queue-tasks") {
				continue
			}
			if item.ProjectID != p.ID || item.RunID != row.RunID || item.Actor.UserID != row.Actor.ID || seen[item.TaskID] || !wanted[item.TaskID] {
				return "", nil, false, ErrOrchestratorConflict
			}
			seen[item.TaskID] = true
			ids = append(ids, item.ID)
		}
		if len(ids) == 0 {
			return "", nil, false, nil
		}
		if len(ids) != len(wanted) {
			return "", nil, false, ErrOrchestratorConflict
		}
		return row.RunID, ids, true, nil
	}
	return "", nil, false, ErrOrchestratorInvalid
}
func (s *Server) continueCoordinator(ctx context.Context, p project.Project, id, actor string, expected int) (coordinatorContinuationRecord, error) {
	var out coordinatorContinuationRecord
	for step := 0; step < 4; step++ {
		s.mu.Lock()
		rows, err := s.readContinuations(p, actor)
		index := findContinuation(rows, id)
		if err != nil || index < 0 {
			s.mu.Unlock()
			if err == nil {
				err = ErrOrchestratorInvalid
			}
			return out, err
		}
		out = rows.Rows[index]
		if expected > 0 && out.Revision != expected {
			s.mu.Unlock()
			return out, ErrOrchestratorConflict
		}
		expected = 0
		if err = s.continuationAuthority(ctx, p, out); err != nil {
			s.mu.Unlock()
			return out, err
		}
		var policy CoordinatorPolicy
		policy, err = s.readCoordinatorPolicy(p)
		if err == nil && (policy.Revision != out.PolicyRevision || policy.ExecutionMode != "reviewed-tasks") {
			err = errCoordinatorPolicy
		}
		if err == nil {
			saved, e := s.readOrchestrator(p)
			err = e
			if err == nil && len(orchestratorQuestions(saved)) > 0 {
				err = ErrOrchestratorQuestion
			}
		}
		if err == nil {
			current, e := s.continuationContextLocked(p, out.Run.ImplementationContext)
			err = e
			if err == nil && current != out.CanonicalDigest {
				err = ErrOrchestratorConflict
			}
		}
		if err != nil {
			s.mu.Unlock()
			return out, err
		}
		next := len(out.Actions)
		if next > 0 && out.Actions[next-1].Status != "confirmed" {
			last := out.Actions[next-1]
			if last.Status == "refused" || s.coordinatorActions[last.ID] {
				s.mu.Unlock()
				return out, nil
			}
			s.mu.Unlock()
			reference, ids, found, observeErr := s.observeCoordinatorStep(p, out, last.Kind)
			if observeErr != nil {
				return out, observeErr
			}
			if !found {
				return out, nil
			} // Absence is not permission to replay a possibly interrupted mutation.
			out, err = s.finishCoordinatorStep(p, actor, id, last.ID, reference, ids, nil)
			if err != nil {
				return out, err
			}
			continue
		}
		if next == 3 {
			s.mu.Unlock()
			return out, nil
		}
		kind := []string{"create-run", "prepare-tasks", "queue-tasks"}[next]
		now := time.Now().UTC().Format(time.RFC3339Nano)
		action := CoordinatorAction{ID: out.ID + "-" + kind, Kind: kind, Status: "pending", StartedAt: now, Message: "This exact operation is reserved. An interrupted response must be reconciled before another step."}
		out.Actions = append(out.Actions, action)
		out.Status = "continuing"
		out.Message = action.Message
		out.Revision++
		out.UpdatedAt = now
		rows.Rows[index] = out
		err = s.saveContinuations(rows)
		if err == nil {
			if s.coordinatorActions == nil {
				s.coordinatorActions = map[string]bool{}
			}
			s.coordinatorActions[action.ID] = true
		}
		s.mu.Unlock()
		if err != nil {
			return out, err
		}
		reference, ids, invokeErr := s.invokeCoordinatorStep(ctx, p, out, kind)
		out, err = s.finishCoordinatorStep(p, actor, id, action.ID, reference, ids, invokeErr)
		s.mu.Lock()
		delete(s.coordinatorActions, action.ID)
		s.mu.Unlock()
		if err != nil {
			return out, err
		}
		if invokeErr != nil {
			return out, nil
		}
	}
	return out, nil
}
func (s *Server) finishCoordinatorStep(p project.Project, actor, id, actionID, reference string, queueIDs []string, callErr error) (coordinatorContinuationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out coordinatorContinuationRecord
	rows, err := s.readContinuations(p, actor)
	index := findContinuation(rows, id)
	if err != nil || index < 0 {
		if err == nil {
			err = ErrOrchestratorInvalid
		}
		return out, err
	}
	out = rows.Rows[index]
	if len(out.Actions) == 0 || out.Actions[len(out.Actions)-1].ID != actionID {
		return out, ErrOrchestratorConflict
	}
	action := &out.Actions[len(out.Actions)-1]
	if action.Status == "confirmed" {
		if reference != action.ReferenceID {
			return out, ErrOrchestratorConflict
		}
		return out, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	action.FinishedAt = now
	if callErr != nil {
		action.Status = "unknown"
		var refusal *coordinatorStepError
		if errors.As(callErr, &refusal) && refusal.Status >= 400 && refusal.Status < 500 {
			action.Status = "refused"
		}
		action.Message = callErr.Error()
		if len(action.Message) > 4000 {
			action.Message = action.Message[:4000]
		}
		out.Status = action.Status
		out.Message = action.Message
	} else {
		if reference == "" || (out.RunID != "" && reference != out.RunID) {
			return out, ErrOrchestratorInvalid
		}
		action.Status = "confirmed"
		action.ReferenceID = reference
		action.Message = "The existing operation's exact durable result was observed."
		out.RunID = reference
		out.Status = "ready"
		out.Message = "The preceding step is confirmed; continuing under the current policy."
		if action.Kind == "queue-tasks" {
			out.QueueIDs = append([]string{}, queueIDs...)
			out.Status = "queued"
			out.Message = "Reviewed tasks are queued under existing provider, dependency and resource admission. Integration, independent checks and delivery still require their authoritative results."
		}
	}
	out.Revision++
	out.UpdatedAt = now
	rows.Rows[index] = out
	return out, s.saveContinuations(rows)
}
