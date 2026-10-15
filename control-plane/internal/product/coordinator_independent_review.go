package product

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type coordinatorIndependentReviewContextKey struct{}
type coordinatorIndependentReviewGate struct {
	Row  coordinatorContinuationRecord
	Kind string
	// Test-only barrier for a policy or actor change after handler preflight.
	BeforeMutation func()
}

func coordinatorIndependentReviewBeforeMutation(ctx context.Context) {
	if gate, ok := ctx.Value(coordinatorIndependentReviewContextKey{}).(coordinatorIndependentReviewGate); ok && gate.BeforeMutation != nil {
		gate.BeforeMutation()
	}
}

// Only background review effects carry this gate. Explicit authorized routes
// retain their own rules. The handler checks it inside the durable run mutation,
// after any concurrent policy, grant, context or contract change.
func (s *Server) checkCoordinatorIndependentReviewGate(ctx context.Context, p project.Project, run ProjectRun, kind string) error {
	gate, ok := ctx.Value(coordinatorIndependentReviewContextKey{}).(coordinatorIndependentReviewGate)
	if !ok {
		return nil
	}
	row := gate.Row
	if gate.Kind != kind || row.ProjectID != p.ID || row.RunID != run.ID || row.Status != "queued" || row.CheckPlan == nil || run.Swarm == nil || run.State == "cancelled" || run.State == "blocked" || run.Input.OrchestrationContract == nil ||
		!sameImplementationSelection(*ImplementationContextSelection(&run.Input.Implementation), row.Run.ImplementationContext) || !s.coordinatorIndependentReviewCurrent(ctx, p, row) || s.contractCurrentForRun(p, run) != nil {
		return ErrProjectRunConflict
	}
	return nil
}

// The coordinator may assign only an already reserved, approved reviewer to
// an exact stopped candidate. The existing review handler owns the CAS and
// independent source checks; the scheduler never invents an assignment.
func (s *Server) continueCoordinatorIndependentReview(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (bool, error) {
	if row.Status != "queued" || row.RunID == "" || row.CheckPlan == nil || len(row.Actions) != 3 || row.Actions[2].Status != "confirmed" {
		return false, nil
	}
	if !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
		return false, nil
	}
	var err error
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || run.Swarm == nil || run.Input.OrchestrationContract == nil || run.State == "cancelled" || run.State == "blocked" {
		return false, err
	}
	if s.contractCurrentForRun(p, run) != nil || s.validateProjectSwarmReuseCurrent(ctx, p, run) != nil {
		return false, nil
	}
	for _, reviewerTask := range run.Swarm.Tasks {
		if reviewerTask.Role != "reviewer" || !swarmContractReviewerRole(run, reviewerTask, reviewerTask.ReviewOfTaskID) {
			continue
		}
		reviewer := swarmOperation(&run, reviewerTask.OperationID)
		candidateTask := swarmTask(&run, reviewerTask.ReviewOfTaskID)
		if reviewer == nil || candidateTask == nil || reviewer.Allocation != "ready" || reviewer.Status != "reserved" || reviewer.ChildID == "" || reviewer.Handoff != nil || reviewer.Implementation == nil {
			continue
		}
		candidate := swarmOperation(&run, candidateTask.OperationID)
		if candidate == nil || candidate.Implementation == nil || candidate.ChildID == "" {
			continue
		}
		// A saved assignment, including one whose response was lost, is the
		// authoritative readback. Its stable request key is never replayed with
		// a different candidate or reviewer generation.
		assigned := false
		for _, obligation := range run.Swarm.ReviewObligations {
			if obligation.ReviewerOperationID == reviewer.ID || obligation.CandidateOperationID == candidate.ID {
				assigned = true
				break
			}
		}
		if assigned {
			continue
		}
		review, reviewErr := s.swarmReview(ctx, p, run, candidateTask.ID)
		if reviewErr != nil || !review.CanIntegrate || review.OperationID != candidate.ID || !hexDigest(review.ReviewDigest) {
			continue
		}
		if !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
			return false, nil
		}
		request := swarmReviewCreateRequest{
			ExpectedRevision:     run.Revision,
			IdempotencyKey:       "coordinator-review-" + implementationDigest(struct{ Run, Candidate, Reviewer string }{run.ID, candidate.ID, reviewer.ID}),
			CandidateOperationID: candidate.ID,
			ReviewerOperationID:  reviewer.ID,
			ReviewDigest:         review.ReviewDigest,
		}
		body, err := json.Marshal(request)
		if err != nil {
			return false, err
		}
		gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "create"})
		httpRequest, err := http.NewRequestWithContext(gated, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(body))
		if err != nil {
			return false, err
		}
		httpRequest.SetPathValue("id", p.ID)
		httpRequest.SetPathValue("runId", run.ID)
		httpRequest.SetPathValue("taskId", candidateTask.ID)
		httpRequest.Header.Set("Content-Type", "application/json")
		capture := &coordinatorResponse{header: make(http.Header)}
		s.createProjectSwarmReviewObligation(capture, httpRequest)
		if capture.status >= 200 && capture.status < 300 && !capture.overflow {
			s.WakeSwarmScheduler()
			return true, nil
		}
		// A lost response does not mean a missing assignment. The next turn
		// inspects its exact durable obligation before considering another POST.
		return false, nil
	}
	return false, nil
}

// An observed reviewer handoff is still a worker claim. The coordinator may
// record that claim and resolve the exact obligation under reviewed-tasks
// policy; integration and combined checks remain separate authoritative gates.
func (s *Server) continueCoordinatorReviewOutcome(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (bool, error) {
	if row.RunID == "" || row.Status != "queued" || !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
		return false, nil
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || run.Swarm == nil || run.Input.OrchestrationContract == nil || run.State == "cancelled" || run.State == "blocked" {
		return false, err
	}
	if s.contractCurrentForRun(p, run) != nil {
		return false, nil
	}
	for _, obligation := range run.Swarm.ReviewObligations {
		if obligation.State != "pending" {
			continue
		}
		reviewer := swarmOperation(&run, obligation.ReviewerOperationID)
		if reviewer == nil || reviewer.ChildID != obligation.ReviewerChildID || reviewer.Handoff == nil || reviewer.ReviewAssignmentDigest != swarmReviewAssignmentDigest(run, reviewer.ID) || reviewer.Status != "succeeded" && reviewer.Status != "review-required" {
			continue
		}
		view, viewErr := s.projectRuns.taskHandoffView(run, *reviewer)
		if viewErr != nil || view.Receipt == nil || view.Receipt.Reported == nil || view.Receipt.ContextDigest == "" || view.Receipt.OperationID != reviewer.ID || view.Receipt.ChildID != reviewer.ChildID || view.Receipt.Reported.Summary == "" {
			continue
		}
		report := view.Receipt.Reported
		if !handoffText(report.Summary, 1000) || strings.TrimSpace(report.Summary) == "" || len(report.ReportedChecks) == 0 || len(report.Questions) > 0 {
			// A consequential question or an oversized/uncheckable finding needs
			// a person. Never coerce it into an automatic pass or repair.
			continue
		}
		outcome := "passed"
		for _, check := range report.ReportedChecks {
			if check.Outcome == "failed" {
				outcome = "repair-required"
			} else if check.Outcome != "passed" {
				outcome = ""
				break
			}
		}
		if outcome == "" {
			continue
		}
		key := "coordinator-finding-" + implementationDigest(struct{ Run, Obligation, Handoff string }{run.ID, obligation.ID, reviewer.Handoff.Digest})
		request := swarmMessagePostRequest{ExpectedRevision: run.Revision, IdempotencyKey: key, RecipientTaskID: obligation.TaskID, Kind: "review-finding", Evidence: ProjectSwarmMessageEvidence{Kind: "handoff", ID: reviewer.ID, Digest: reviewer.Handoff.Digest}, Summary: report.Summary}
		messageID := ""
		for _, message := range run.Swarm.Messages {
			if message.Sender.OperationID == reviewer.ID && message.KeyDigest == implementationDigest(key) {
				if message.RequestDigest != swarmMessageRequestDigest(request) || message.Kind != "review-finding" || message.RecipientTaskID != obligation.TaskID || message.Evidence != request.Evidence {
					return false, nil
				}
				messageID = message.ID
				break
			}
		}
		if messageID == "" {
			if !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
				return false, nil
			}
			body, marshalErr := json.Marshal(request)
			if marshalErr != nil {
				return false, marshalErr
			}
			gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "finding"})
			httpRequest, requestErr := http.NewRequestWithContext(gated, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(body))
			if requestErr != nil {
				return false, requestErr
			}
			httpRequest.SetPathValue("id", p.ID)
			httpRequest.SetPathValue("runId", run.ID)
			httpRequest.SetPathValue("operationId", reviewer.ID)
			httpRequest.Header.Set("Content-Type", "application/json")
			capture := &coordinatorResponse{header: make(http.Header)}
			s.postProjectSwarmMessage(capture, httpRequest)
			if capture.status < 200 || capture.status >= 300 || capture.overflow {
				return false, nil // exact key readback on next turn
			}
			return true, nil // one durable effect per coordinator turn
		}
		if !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
			return false, nil
		}
		resolveRequest := struct {
			ExpectedRevision      int    `json:"expectedRevision"`
			Outcome               string `json:"outcome"`
			ReviewerHandoffDigest string `json:"reviewerHandoffDigest"`
			FindingMessageID      string `json:"findingMessageId"`
		}{run.Revision, outcome, reviewer.Handoff.Digest, messageID}
		body, marshalErr := json.Marshal(resolveRequest)
		if marshalErr != nil {
			return false, marshalErr
		}
		gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "resolve"})
		httpRequest, requestErr := http.NewRequestWithContext(gated, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(body))
		if requestErr != nil {
			return false, requestErr
		}
		httpRequest.SetPathValue("id", p.ID)
		httpRequest.SetPathValue("runId", run.ID)
		httpRequest.SetPathValue("obligationId", obligation.ID)
		httpRequest.Header.Set("Content-Type", "application/json")
		capture := &coordinatorResponse{header: make(http.Header)}
		s.resolveProjectSwarmReviewObligation(capture, httpRequest)
		if capture.status >= 200 && capture.status < 300 && !capture.overflow {
			s.WakeCoordinator()
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func (s *Server) coordinatorIndependentReviewCurrent(ctx context.Context, p project.Project, row coordinatorContinuationRecord) bool {
	if s.continuationAuthority(ctx, p, row) != nil {
		return false
	}
	s.mu.RLock()
	policy, err := s.readCoordinatorPolicy(p)
	if err == nil {
		var digest string
		digest, err = s.continuationContextLocked(p, row.Run.ImplementationContext)
		if err == nil && digest != row.CanonicalDigest {
			err = ErrOrchestratorConflict
		}
	}
	s.mu.RUnlock()
	return err == nil && policy.Revision == row.PolicyRevision && policy.ExecutionMode == "reviewed-tasks" && policy.IntegrationMode == "reviewed-scopes"
}
