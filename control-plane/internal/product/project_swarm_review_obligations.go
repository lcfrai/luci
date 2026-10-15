package product

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// A review obligation is an exact, retained relationship between two owned
// attempts. It never substitutes for checked combined source or grant consent.
func (s *Server) createProjectSwarmReviewObligation(w http.ResponseWriter, r *http.Request) {
	if !swarmCoordinationAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if err := s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		projectRunProblem(w, err)
		return
	}
	var req struct {
		ExpectedRevision     int    `json:"expectedRevision"`
		IdempotencyKey       string `json:"idempotencyKey"`
		CandidateOperationID string `json:"candidateOperationId"`
		ReviewerOperationID  string `json:"reviewerOperationId"`
		ReviewDigest         string `json:"reviewDigest"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validRunKey(req.IdempotencyKey) || !hexDigest(req.ReviewDigest) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil || run.Swarm == nil {
		if err == nil {
			err = ErrProjectRunNotFound
		}
		projectRunProblem(w, err)
		return
	}
	digest := implementationDigest(struct{ Task, Candidate, Reviewer, Review string }{r.PathValue("taskId"), req.CandidateOperationID, req.ReviewerOperationID, req.ReviewDigest})
	for _, prior := range run.Swarm.ReviewObligations {
		if prior.KeyDigest == implementationDigest(req.IdempotencyKey) {
			if prior.RequestDigest != digest {
				projectRunProblem(w, ErrProjectRunKeyConflict)
				return
			}
			journeyJSON(w, 200, map[string]any{"obligation": swarmReviewObligationView(run, prior), "runRevision": run.Revision})
			return
		}
	}
	if run.Revision != req.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	repairs := 0
	for _, prior := range run.Swarm.ReviewObligations {
		if prior.TaskID == r.PathValue("taskId") && prior.State == "repair-required" {
			repairs++
		}
	}
	if repairs >= maxSwarmReviewRepairs {
		projectRunProblem(w, errors.New("Two independent findings already require repair for this task. Replan it before another autonomous review cycle."))
		return
	}
	candidate := swarmOperation(&run, req.CandidateOperationID)
	reviewer := swarmOperation(&run, req.ReviewerOperationID)
	if candidate == nil || reviewer == nil || candidate.TaskID != r.PathValue("taskId") || swarmTask(&run, candidate.TaskID) == nil || swarmTask(&run, candidate.TaskID).OperationID != candidate.ID || swarmTask(&run, reviewer.TaskID) == nil || swarmTask(&run, reviewer.TaskID).OperationID != reviewer.ID || reviewer.TaskID == candidate.TaskID || reviewer.ID == candidate.ID || reviewer.ChildID == "" || reviewer.Allocation != "ready" || reviewer.Status != "reserved" || reviewer.Handoff != nil || reviewer.Implementation == nil || candidate.Implementation == nil || implementationDigest(reviewer.Implementation) == implementationDigest(candidate.Implementation) || len(swarmTask(&run, reviewer.TaskID).WritePaths) != 0 {
		projectRunProblem(w, errors.New("Reserve a distinct, not-yet-started zero-write-scope task attempt for the independent review. Preserve implementation tasks and their write scope."))
		return
	}
	for _, task := range run.Swarm.Tasks {
		for _, dep := range task.DependsOn {
			if dep == reviewer.TaskID {
				projectRunProblem(w, errors.New("A dependent implementation task requires code integration from this task, so it cannot be used as the review-only task."))
				return
			}
		}
	}
	review, err := s.swarmReview(r.Context(), p, run, candidate.TaskID)
	if err != nil || !review.CanIntegrate || review.OperationID != candidate.ID || review.ReviewDigest != req.ReviewDigest {
		if err == nil {
			err = ErrProjectRunConflict
		}
		projectRunProblem(w, err)
		return
	}
	item := ProjectSwarmReviewObligation{ID: newID("review-"), TaskID: candidate.TaskID, CandidateOperationID: candidate.ID, CandidateChildID: candidate.ChildID, CandidateCommit: review.SourceCommit, ContextDigest: implementationDigest(candidate.Implementation), ReviewDigest: review.ReviewDigest, ReviewerOperationID: reviewer.ID, ReviewerChildID: reviewer.ChildID, State: "pending", Attempt: candidate.Attempt, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), KeyDigest: implementationDigest(req.IdempotencyKey), RequestDigest: digest}
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, req.ExpectedRevision, func(current *ProjectRun) error {
		if current.Swarm == nil || len(current.Swarm.ReviewObligations) >= maxSwarmReviewObligations {
			return ErrProjectRunLimit
		}
		if op := swarmOperation(current, reviewer.ID); op == nil || op.Status != "reserved" || op.ChildID != reviewer.ChildID || swarmTask(current, reviewer.TaskID) == nil || swarmTask(current, reviewer.TaskID).OperationID != reviewer.ID {
			return ErrProjectRunConflict
		}
		swarmTask(current, reviewer.TaskID).ReviewOnly = true
		for _, prior := range current.Swarm.ReviewObligations {
			if prior.CandidateOperationID == candidate.ID && prior.State != "repair-required" {
				return ErrProjectRunConflict
			}
			if prior.ReviewerOperationID == reviewer.ID {
				return ErrProjectRunConflict
			}
		}
		current.Swarm.ReviewObligations = append(current.Swarm.ReviewObligations, item)
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 201, map[string]any{"obligation": swarmReviewObligationView(run, item), "runRevision": run.Revision})
}

func (s *Server) resolveProjectSwarmReviewObligation(w http.ResponseWriter, r *http.Request) {
	if !swarmCoordinationAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if err := s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		projectRunProblem(w, err)
		return
	}
	var req struct {
		ExpectedRevision      int    `json:"expectedRevision"`
		Outcome               string `json:"outcome"`
		ReviewerHandoffDigest string `json:"reviewerHandoffDigest"`
		FindingMessageID      string `json:"findingMessageId"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if (req.Outcome != "passed" && req.Outcome != "repair-required") || !hexDigest(req.ReviewerHandoffDigest) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	var item ProjectSwarmReviewObligation
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if run.Swarm == nil {
			return ErrProjectRunNotFound
		}
		for i := range run.Swarm.ReviewObligations {
			selected := &run.Swarm.ReviewObligations[i]
			if selected.ID != r.PathValue("obligationId") {
				continue
			}
			if selected.State != "pending" {
				return ErrProjectRunConflict
			}
			reviewer := swarmOperation(run, selected.ReviewerOperationID)
			candidate := swarmOperation(run, selected.CandidateOperationID)
			if reviewer == nil || candidate == nil || reviewer.ChildID != selected.ReviewerChildID || reviewer.Handoff == nil || reviewer.Handoff.Digest != req.ReviewerHandoffDigest || reviewer.Status != "review-required" && reviewer.Status != "succeeded" || candidate.ChildID != selected.CandidateChildID {
				return ErrProjectRunConflict
			}
			view, err := s.projectRuns.taskHandoffView(*run, *reviewer)
			if err != nil || view.Receipt == nil || view.Receipt.Reported == nil || view.Receipt.ContextDigest == "" {
				return ErrProjectRunConflict
			}
			if req.Outcome == "passed" {
				if strings.TrimSpace(view.Receipt.Reported.Summary) == "" {
					return ErrProjectRunConflict
				}
				for _, check := range view.Receipt.Reported.ReportedChecks {
					if check.Outcome != "passed" {
						return ErrProjectRunConflict
					}
				}
				for _, question := range view.Receipt.Reported.Questions {
					if question.Blocking {
						return ErrProjectRunConflict
					}
				}
			}
			if req.FindingMessageID != "" {
				found := false
				for _, msg := range run.Swarm.Messages {
					if msg.ID == req.FindingMessageID && msg.Kind == "review-finding" && msg.Sender.OperationID == reviewer.ID && msg.RecipientTaskID == candidate.TaskID && msg.Evidence.Kind == "handoff" && msg.Evidence.Digest == reviewer.Handoff.Digest {
						found = true
						break
					}
				}
				if !found {
					return ErrProjectRunConflict
				}
			} else {
				return ErrProjectRunConflict
			}
			selected.State, selected.ReviewerHandoffDigest, selected.FindingMessageID, selected.ResolvedAt = req.Outcome, req.ReviewerHandoffDigest, req.FindingMessageID, time.Now().UTC().Format(time.RFC3339Nano)
			item = *selected
			return nil
		}
		return ErrProjectRunNotFound
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"obligation": swarmReviewObligationView(run, item), "runRevision": run.Revision})
}

func swarmIndependentReviewReady(run ProjectRun, taskID string, review ProjectSwarmReview) bool {
	if run.Input.OrchestrationContract == nil {
		return true
	} // older runs remain explicitly unassessed
	for _, item := range run.Swarm.ReviewObligations {
		if item.TaskID == taskID && item.CandidateOperationID == review.OperationID && item.ReviewDigest == review.ReviewDigest && item.CandidateCommit == review.SourceCommit && item.State == "passed" && hexDigest(item.ReviewerHandoffDigest) && item.FindingMessageID != "" {
			reviewer := swarmOperation(&run, item.ReviewerOperationID)
			if reviewer == nil || reviewer.ChildID != item.ReviewerChildID || reviewer.Handoff == nil || reviewer.Handoff.Digest != item.ReviewerHandoffDigest {
				continue
			}
			for _, msg := range run.Swarm.Messages {
				if msg.ID == item.FindingMessageID && msg.Kind == "review-finding" && msg.Sender.OperationID == reviewer.ID && msg.RecipientTaskID == taskID && msg.Evidence.Kind == "handoff" && msg.Evidence.Digest == item.ReviewerHandoffDigest {
					return true
				}
			}
		}
	}
	return false
}
