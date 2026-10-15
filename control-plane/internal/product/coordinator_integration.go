package product

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type coordinatorIntegrationContextKey struct{}
type coordinatorIntegrationGate struct {
	Row    coordinatorContinuationRecord
	TaskID string
}

// Called again inside the integration handler's source/dispatch exclusion,
// immediately before its durable integrating receipt. A revoked delegation
// cannot rely on an earlier background read.
func (s *Server) checkCoordinatorIntegrationGate(ctx context.Context, p project.Project, run ProjectRun) error {
	gate, ok := ctx.Value(coordinatorIntegrationContextKey{}).(coordinatorIntegrationGate)
	if !ok {
		return nil // The ordinary explicit review route retains its own authority.
	}
	if gate.Row.ProjectID != p.ID || gate.Row.RunID != run.ID || gate.Row.Status != "queued" || gate.Row.CheckPlan == nil {
		return errCoordinatorAuthority
	}
	if err := s.continuationAuthority(ctx, p, gate.Row); err != nil {
		return err
	}
	s.mu.RLock()
	policy, err := s.readCoordinatorPolicy(p)
	if err == nil && (policy.Revision != gate.Row.PolicyRevision || policy.ExecutionMode != "reviewed-tasks" || policy.IntegrationMode != "reviewed-scopes") {
		err = ErrOrchestratorConflict
	}
	if err == nil {
		var digest string
		digest, err = s.continuationContextLocked(p, gate.Row.Run.ImplementationContext)
		if err == nil && digest != gate.Row.CanonicalDigest {
			err = ErrOrchestratorConflict
		}
	}
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	task := swarmTask(&run, gate.TaskID)
	if task == nil || len(task.WritePaths) == 0 {
		return ErrOrchestratorConflict
	}
	return nil
}

// This optional project policy delegates the decision to integrate one exact
// stopped candidate inside its reviewed write paths. It does not delegate
// semantic acceptance: the combined result still needs independent checks,
// preview and the separately authorized delivery plan. Any pending/unknown
// integration receipt is retained and never invoked twice.
func (s *Server) continueCoordinatorCandidateIntegration(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (bool, error) {
	if row.RunID == "" || row.CheckPlan == nil || len(row.Actions) != 3 || row.Actions[2].Status != "confirmed" || row.Status != "queued" {
		return false, nil
	}
	s.mu.RLock()
	policy, err := s.readCoordinatorPolicy(p)
	if err == nil && policy.Revision == row.PolicyRevision && policy.ExecutionMode == "reviewed-tasks" && policy.IntegrationMode == "reviewed-scopes" {
		var digest string
		digest, err = s.continuationContextLocked(p, row.Run.ImplementationContext)
		if err == nil && digest != row.CanonicalDigest {
			err = ErrOrchestratorConflict
		}
	} else if err == nil {
		s.mu.RUnlock()
		return false, nil
	}
	s.mu.RUnlock()
	if err != nil || s.continuationAuthority(ctx, p, row) != nil {
		return false, nil
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || run.Swarm == nil || run.State == "cancelled" || run.State == "blocked" {
		return false, err
	}
	if err = s.validateProjectSwarmReuseCurrent(ctx, p, run); err != nil {
		return false, nil
	}
	for _, task := range run.Swarm.Tasks {
		if task.Integration != nil || len(task.WritePaths) == 0 {
			continue
		}
		op := swarmOperation(&run, task.OperationID)
		if op == nil || op.Allocation != "ready" || op.Status != "succeeded" && (op.Status != "review-required" || op.Handoff == nil || op.Handoff.Unanswered != 0) {
			continue
		}
		review, reviewErr := s.swarmReview(ctx, p, run, task.ID)
		if reviewErr != nil || !review.CanIntegrate || !swarmIndependentReviewReady(run, task.ID, review) || len(review.Paths) == 0 || !hexDigest(review.ReviewDigest) {
			return false, nil
		}
		for _, path := range review.Paths {
			if !swarmPathAllowed(path, task.WritePaths) {
				return false, nil
			}
		}
		s.mu.RLock()
		currentPolicy, currentErr := s.readCoordinatorPolicy(p)
		if currentErr == nil {
			var digest string
			digest, currentErr = s.continuationContextLocked(p, row.Run.ImplementationContext)
			if currentErr == nil && digest != row.CanonicalDigest {
				currentErr = ErrOrchestratorConflict
			}
		}
		s.mu.RUnlock()
		if currentErr != nil || currentPolicy.Revision != row.PolicyRevision || currentPolicy.IntegrationMode != "reviewed-scopes" || s.continuationAuthority(ctx, p, row) != nil {
			return false, nil
		}
		body, _ := json.Marshal(map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest})
		gated := context.WithValue(ctx, coordinatorIntegrationContextKey{}, coordinatorIntegrationGate{Row: row, TaskID: task.ID})
		req, requestErr := http.NewRequestWithContext(gated, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(body))
		if requestErr != nil {
			return false, requestErr
		}
		req.SetPathValue("id", p.ID)
		req.SetPathValue("runId", run.ID)
		req.SetPathValue("taskId", task.ID)
		req.Header.Set("Content-Type", "application/json")
		capture := &coordinatorResponse{header: make(http.Header)}
		s.integrateProjectSwarmTask(capture, req)
		if capture.status >= 200 && capture.status < 300 && !capture.overflow {
			return true, nil
		}
		// A response can be lost after the integration reservation. The
		// authoritative task receipt, not this response, governs any retry.
		return false, nil
	}
	return false, nil
}
