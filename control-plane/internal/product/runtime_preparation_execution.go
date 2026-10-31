package product

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// The batch is the same reviewed preparation, not a new deployment engine.
// Every service intent is persisted before the request is acknowledged. Only
// in-memory permits created by that write may call the existing publisher.
type RuntimePreparationExecution struct {
	RunID          string `json:"runId"`
	CheckID        string `json:"checkId"`
	Commit         string `json:"commit"`
	SourceDigest   string `json:"sourceDigest"`
	RunInputDigest string `json:"runInputDigest"`
	CheckDigest    string `json:"checkDigest"`
	StartedAt      string `json:"startedAt"`
	FinishedAt     string `json:"finishedAt,omitempty"`
	State          string `json:"state"`
}
type RuntimePreparationStart struct {
	ExpectedRevision  int    `json:"expectedRevision"`
	PreparationDigest string `json:"preparationDigest"`
	RunID             string `json:"runId"`
	CheckID           string `json:"checkId"`
}

func runtimeExecutionSource(e *RuntimePreparationExecution) RuntimeBuildAttempt {
	if e == nil {
		return RuntimeBuildAttempt{}
	}
	return RuntimeBuildAttempt{RunID: e.RunID, CheckID: e.CheckID, Commit: e.Commit, SourceDigest: e.SourceDigest, RunInputDigest: e.RunInputDigest, CheckDigest: e.CheckDigest, Attempted: true}
}
func validRuntimePreparationExecution(row RuntimePreparation) bool {
	e := row.Execution
	if e == nil {
		return true
	}
	if !validConnectionToken(e.RunID, 128, false) || !validConnectionToken(e.CheckID, 128, false) || !deliverySHA.MatchString(e.Commit) || !hexDigest(e.SourceDigest) || !hexDigest(e.RunInputDigest) || !hexDigest(e.CheckDigest) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, e.StartedAt); err != nil {
		return false
	}
	if e.State != "running" && e.State != "attention" && e.State != "complete" {
		return false
	}
	if e.State == "running" && e.FinishedAt != "" || e.State != "running" && e.FinishedAt == "" {
		return false
	}
	if e.FinishedAt != "" {
		finished, err := time.Parse(time.RFC3339Nano, e.FinishedAt)
		started, _ := time.Parse(time.RFC3339Nano, e.StartedAt)
		if err != nil || finished.Before(started) {
			return false
		}
	}
	if e.State == "complete" && row.SavedBlueprintRevision <= row.Plan.BlueprintRevision {
		return false
	}
	if len(row.Builds) != len(row.Plan.Builds) {
		return false
	}
	for _, planned := range row.Plan.Builds {
		found := false
		for _, b := range row.Builds {
			if b.ServiceID == planned.Selection.ServiceID && sameRuntimeBuildSource(b, runtimeExecutionSource(e)) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func (s *Server) runtimePreparationActive(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.coordinatorActions["preparation-"+id]
}
func (s *Server) finishRuntimePreparationExecution(actor management.ExecutionActorRef, id string, source RuntimeBuildAttempt, complete bool) {
	s.runtimePreparationMu.Lock()
	defer s.runtimePreparationMu.Unlock()
	file, row, err := s.readRuntimePreparation(actor, id)
	if err != nil || row.Execution == nil || !sameRuntimeBuildSource(runtimeExecutionSource(row.Execution), source) {
		return
	}
	row.Execution.State = "attention"
	if complete && row.SavedBlueprintRevision > row.Plan.BlueprintRevision {
		row.Execution.State = "complete"
	}
	row.Execution.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	row.Revision++
	if !validRuntimePreparationExecution(row.RuntimePreparation) || runtimePreparationBytes(row) != nil {
		return
	}
	_ = durableRunJSON(file, row)
}

func (s *Server) StartRuntimePreparation(ctx context.Context, actor management.ExecutionActorRef, id string, input RuntimePreparationStart) (RuntimePreparation, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	if input.PreparationDigest != row.Plan.Digest {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	// Response loss or process restart can only read the same original batch.
	if row.Execution != nil {
		if row.Execution.RunID != input.RunID || row.Execution.CheckID != input.CheckID {
			return row.RuntimePreparation, ErrRuntimePreparation
		}
		return s.ReadRuntimePreparation(ctx, actor, id)
	}
	if len(row.Builds) != 0 || row.SavedBlueprintRevision != 0 || row.Revision != input.ExpectedRevision {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	source, err := s.runtimeBuildSource(ctx, row, input.RunID, input.CheckID)
	if err != nil {
		return row.RuntimePreparation, err
	}
	if len(row.Plan.Builds) > 0 && (s.PreviewRuntime().BuildAndPublish == nil || s.PreviewRuntime().Observe == nil) {
		return row.RuntimePreparation, errors.New("This engine does not support image publication")
	}
	if err = ctx.Err(); err != nil {
		return row.RuntimePreparation, err
	}
	permits := make([]*runtimeImagePermit, 0, len(row.Plan.Builds))
	s.runtimePreparationMu.Lock()
	file, current, err := s.readRuntimePreparation(actor, id)
	if err == nil && (current.Revision != row.Revision || reviewDigest(current) != reviewDigest(row)) {
		err = ErrRuntimePreparation
	}
	if err == nil {
		err = s.runtimePreparationAuthority(ctx, current, true)
	}
	if err == nil {
		for _, build := range current.Plan.Builds {
			attempt := source
			attempt.ServiceID = build.Selection.ServiceID
			current.Builds = append(current.Builds, attempt)
			permits = append(permits, &runtimeImagePermit{planDigest: current.Plan.Digest, source: attempt})
		}
		current.Execution = &RuntimePreparationExecution{RunID: source.RunID, CheckID: source.CheckID, Commit: source.Commit, SourceDigest: source.SourceDigest, RunInputDigest: source.RunInputDigest, CheckDigest: source.CheckDigest, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), State: "running"}
		current.Revision++
		if !validRuntimePreparationExecution(current.RuntimePreparation) {
			err = ErrRuntimePreparation
		} else {
			err = runtimePreparationBytes(current)
		}
	}
	if err == nil {
		err = durableRunJSON(file, current)
	}
	s.runtimePreparationMu.Unlock()
	if err != nil {
		return row.RuntimePreparation, err
	}
	row = current
	// Persistence failure never reaches this point. A crash after the write has
	// no surviving permits and therefore cannot create replacement builds.
	key := "preparation-" + id
	s.mu.Lock()
	if s.coordinatorActions == nil {
		s.coordinatorActions = map[string]bool{}
	}
	s.coordinatorActions[key] = true
	s.mu.Unlock()
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Minute)
	stop := context.AfterFunc(s.workspaceContext, cancel)
	go func() {
		complete := false
		defer cancel()
		defer stop()
		defer func() {
			s.finishRuntimePreparationExecution(actor, id, source, complete)
			s.mu.Lock()
			delete(s.coordinatorActions, key)
			s.mu.Unlock()
			s.WakeCoordinator()
		}()
		for _, permit := range permits {
			result, err := s.executeRuntimePreparationService(bounded, actor, id, permit.source.ServiceID, source.RunID, source.CheckID, permit)
			if err != nil {
				return
			}
			published := false
			for _, build := range result.Builds {
				if build.ServiceID == permit.source.ServiceID && build.Runtime != nil && build.Runtime.Phase == "published" && !build.Runtime.ResourcesHeld && build.Runtime.CleanupConfirmed {
					published = true
				}
			}
			if !published {
				return
			}
		}
		if _, err := s.RunRuntimeResourcePreparation(bounded, actor, id, source.RunID, source.CheckID); err != nil {
			return
		}
		result, err := s.FinalizeRuntimePreparation(bounded, actor, id)
		complete = err == nil && result.SavedBlueprintRevision > row.Plan.BlueprintRevision
	}()
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	return row.RuntimePreparation, nil
}

// Recovery is explicit and observational. It cannot create a missing service,
// resource, upload or build. Successful original receipts may finish the same
// blueprint CAS that was already authorized by this batch.
func (s *Server) ObserveRuntimePreparation(ctx context.Context, actor management.ExecutionActorRef, id string) (RuntimePreparation, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	if row.Execution == nil || s.runtimePreparationActive(id) || row.Execution.State == "complete" {
		return s.ReadRuntimePreparation(ctx, actor, id)
	}
	source := runtimeExecutionSource(row.Execution)
	failed := func(cause error) (RuntimePreparation, error) {
		s.finishRuntimePreparationExecution(actor, id, source, false)
		return row.RuntimePreparation, cause
	}
	for _, build := range row.Builds {
		if build.Runtime == nil {
			return failed(ErrRuntimePreparation)
		}
		if _, err = s.ObserveRuntimePreparationService(ctx, actor, id, build.ServiceID); err != nil {
			return failed(err)
		}
	}
	if _, err = s.ObserveRuntimeResourcePreparation(ctx, actor, id); err != nil {
		return failed(err)
	}
	result, err := s.FinalizeRuntimePreparation(ctx, actor, id)
	s.finishRuntimePreparationExecution(actor, id, source, err == nil && result.SavedBlueprintRevision > row.Plan.BlueprintRevision)
	if err != nil {
		return row.RuntimePreparation, err
	}
	return s.ReadRuntimePreparation(ctx, actor, id)
}
func (s *Server) runtimePreparationExecutionHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, err := management.CaptureDeploymentActor(r.Context(), p.ID)
	if err != nil || actor.Local {
		problem(w, 403, "runtime_forbidden", "Current project deployment and Secrets access is required.")
		return
	}
	var row RuntimePreparation
	if r.PathValue("action") == "observe" {
		row, err = s.ObserveRuntimePreparation(r.Context(), actor, r.PathValue("preparationId"))
	} else if r.PathValue("action") == "build" {
		var input RuntimePreparationStart
		if !journeyDecode(w, r, &input) {
			return
		}
		row, err = s.StartRuntimePreparation(r.Context(), actor, r.PathValue("preparationId"), input)
	} else {
		problem(w, 404, "runtime_action_unknown", "Choose Build reviewed images or Refresh original outcome.")
		return
	}
	if err != nil {
		problem(w, 409, "runtime_preparation_pending", "This preparation could not advance. Read its original outcome and current checked run before continuing; do not create a replacement for an unknown build.")
		return
	}
	writeJSON(w, 202, row)
}
