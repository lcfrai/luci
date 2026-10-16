package product

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

type swarmReviewCreateRequest struct {
	ExpectedRevision     int    `json:"expectedRevision"`
	IdempotencyKey       string `json:"idempotencyKey"`
	CandidateOperationID string `json:"candidateOperationId"`
	ReviewerOperationID  string `json:"reviewerOperationId"`
	ReviewDigest         string `json:"reviewDigest"`
}

func swarmReviewCreateDigest(taskID string, req swarmReviewCreateRequest) string {
	return implementationDigest(struct{ Task, Candidate, Reviewer, Review string }{taskID, req.CandidateOperationID, req.ReviewerOperationID, req.ReviewDigest})
}

// A zero write scope is not a reviewer designation. The role and its exact
// candidate must have been approved in the immutable execution contract.
func swarmContractReviewerRole(run ProjectRun, reviewer ProjectSwarmTask, candidateTaskID string) bool {
	if reviewer.Role != "reviewer" || reviewer.ReviewOfTaskID != candidateTaskID || len(reviewer.WritePaths) != 0 || run.Input.OrchestrationContract == nil {
		return false
	}
	codingCandidate := false
	for _, task := range run.Input.OrchestrationContract.Input.Tasks {
		if task.ID == candidateTaskID && task.Role != "reviewer" {
			codingCandidate = true
		}
	}
	if !codingCandidate {
		return false
	}
	for _, task := range run.Input.OrchestrationContract.Input.Tasks {
		if task.ID == reviewer.ID {
			return task.Role == "reviewer" && task.ReviewOfTaskID == candidateTaskID && len(task.WritePaths) == 0 && task.Acceptance == reviewer.Acceptance
		}
	}
	return false
}

// An unresolved failed finding is discharged only by its already reserved
// repair attempt producing a different candidate commit. A second reviewer
// of the same bytes cannot erase the finding.
func swarmReviewRepairSuccessor(run ProjectRun, taskID string, candidate ProjectRunOperation, candidateCommit string) error {
	if run.Swarm == nil {
		return ErrProjectRunConflict
	}
	for i := len(run.Swarm.ReviewObligations) - 1; i >= 0; i-- {
		prior := run.Swarm.ReviewObligations[i]
		if prior.TaskID != taskID || prior.State != "repair-required" {
			continue
		}
		if prior.RepairOperationID == "" || candidate.ID != prior.RepairOperationID || candidate.Checkpoint == nil || candidate.Checkpoint.SourceOperationID != prior.CandidateOperationID || candidate.Checkpoint.SourceCommit != prior.CandidateCommit || candidateCommit != "" && candidateCommit == prior.CandidateCommit {
			return errors.New("The failed independent finding requires its exact reserved repair attempt and a changed candidate before another review can pass.")
		}
		return nil
	}
	return nil
}

func swarmFailedReviewCount(run ProjectRun, taskID string) int {
	count := 0
	if run.Swarm != nil {
		for _, item := range run.Swarm.ReviewObligations {
			if item.TaskID == taskID && item.State == "repair-required" {
				count++
			}
		}
	}
	return count
}

func (s *Server) getProjectSwarmReviewObligation(w http.ResponseWriter, r *http.Request) {
	if !swarmCoordinationAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
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
	for _, item := range run.Swarm.ReviewObligations {
		if item.ID == r.PathValue("obligationId") {
			journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "obligation": swarmReviewObligationView(run, item), "runRevision": run.Revision})
			return
		}
	}
	projectRunProblem(w, ErrProjectRunNotFound)
}

// Exact no-effect create disposition: a missing key is rejected only after
// its CAS revision has advanced. The original request stays client-retained
// while the same revision could still admit an in-flight create.
func (s *Server) projectSwarmReviewObligationDisposition(w http.ResponseWriter, r *http.Request) {
	if !swarmCoordinationAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req swarmReviewCreateRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !validRunKey(req.IdempotencyKey) || !hexDigest(req.ReviewDigest) {
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
	if run.Revision < req.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	state := "unconfirmed"
	var found *ProjectSwarmReviewObligationView
	for _, item := range run.Swarm.ReviewObligations {
		if item.KeyDigest != implementationDigest(req.IdempotencyKey) {
			continue
		}
		if item.RequestDigest == swarmReviewCreateDigest(r.PathValue("taskId"), req) {
			state = "recorded"
			view := swarmReviewObligationView(run, item)
			found = &view
		} else {
			state = "key-conflict"
		}
		break
	}
	if state == "unconfirmed" && run.Revision > req.ExpectedRevision {
		state = "rejected"
	}
	journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "taskId": r.PathValue("taskId"), "state": state, "runRevision": run.Revision, "obligation": found})
}

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
	var req swarmReviewCreateRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validRunKey(req.IdempotencyKey) || !hexDigest(req.ReviewDigest) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	// Serialize new assignments with the final owned-session claim. Capture
	// itself may precede this lock; its digest is checked again at admission.
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil || run.Swarm == nil {
		if err == nil {
			err = ErrProjectRunNotFound
		}
		projectRunProblem(w, err)
		return
	}
	digest := swarmReviewCreateDigest(r.PathValue("taskId"), req)
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
	// Two repairs are permitted, and the second repaired candidate still needs
	// its own independent review. A failed final review can be retained as a
	// finding, but it cannot authorize a third repair under this contract.
	if swarmFailedReviewCount(run, r.PathValue("taskId")) > maxSwarmReviewRepairs {
		projectRunProblem(w, errors.New("The independent review repair limit is exhausted. Replan this task before another review or repair."))
		return
	}
	candidate := swarmOperation(&run, req.CandidateOperationID)
	reviewer := swarmOperation(&run, req.ReviewerOperationID)
	if candidate == nil || reviewer == nil || candidate.TaskID != r.PathValue("taskId") || swarmTask(&run, candidate.TaskID) == nil || swarmTask(&run, candidate.TaskID).OperationID != candidate.ID || swarmTask(&run, reviewer.TaskID) == nil || swarmTask(&run, reviewer.TaskID).OperationID != reviewer.ID || reviewer.TaskID == candidate.TaskID || reviewer.ID == candidate.ID || reviewer.ChildID == "" || reviewer.Allocation != "ready" || reviewer.Status != "reserved" || reviewer.Handoff != nil || reviewer.Implementation == nil || candidate.Implementation == nil || implementationDigest(reviewer.Implementation) == implementationDigest(candidate.Implementation) || !swarmContractReviewerRole(run, *swarmTask(&run, reviewer.TaskID), candidate.TaskID) {
		projectRunProblem(w, errors.New("Reserve a distinct, not-yet-started zero-write-scope task attempt for the independent review. Preserve implementation tasks and their write scope."))
		return
	}
	if err := swarmReviewRepairSuccessor(run, candidate.TaskID, *candidate, ""); err != nil {
		projectRunProblem(w, err)
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
	coordinatorIndependentReviewBeforeMutation(r.Context())
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, req.ExpectedRevision, func(current *ProjectRun) error {
		if err := s.checkCoordinatorIndependentReviewGate(r.Context(), p, *current, "create"); err != nil {
			return err
		}
		if current.Swarm == nil || len(current.Swarm.ReviewObligations) >= maxSwarmReviewObligations {
			return ErrProjectRunLimit
		}
		if op := swarmOperation(current, reviewer.ID); op == nil || op.Status != "reserved" || op.ChildID != reviewer.ChildID || swarmTask(current, reviewer.TaskID) == nil || swarmTask(current, reviewer.TaskID).OperationID != reviewer.ID {
			return ErrProjectRunConflict
		}
		swarmTask(current, reviewer.TaskID).ReviewOnly = true
		if err := swarmReviewRepairSuccessor(*current, candidate.TaskID, *candidate, item.CandidateCommit); err != nil {
			return err
		}
		for _, prior := range current.Swarm.ReviewObligations {
			if prior.CandidateOperationID == candidate.ID {
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
	coordinatorIndependentReviewBeforeMutation(r.Context())
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if err := s.checkCoordinatorIndependentReviewGate(r.Context(), p, *run, "resolve"); err != nil {
			return err
		}
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
			if selected.ReviewerOperationID != reviewer.ID || reviewer.ReviewAssignmentDigest != swarmReviewAssignmentDigest(*run, reviewer.ID) {
				return errors.New("This reviewer did not start with the exact saved review assignment. Preserve its handoff and reserve a new reviewed attempt.")
			}
			if req.Outcome == "passed" {
				if len(view.Receipt.Reported.ReportedChecks) == 0 {
					return errors.New("An independent pass requires at least one reported candidate check and a finding linked to the exact delivered assignment.")
				}
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
	candidate := swarmOperation(&run, review.OperationID)
	if candidate == nil || swarmReviewRepairSuccessor(run, taskID, *candidate, review.SourceCommit) != nil {
		return false
	}
	for _, item := range run.Swarm.ReviewObligations {
		if item.TaskID == taskID && item.CandidateOperationID == review.OperationID && item.ReviewDigest == review.ReviewDigest && item.CandidateCommit == review.SourceCommit && item.State == "passed" && hexDigest(item.ReviewerHandoffDigest) && item.FindingMessageID != "" {
			reviewer := swarmOperation(&run, item.ReviewerOperationID)
			var reviewerTask *ProjectSwarmTask
			if reviewer != nil {
				reviewerTask = swarmTask(&run, reviewer.TaskID)
			}
			if reviewer == nil || reviewerTask == nil || reviewerTask.OperationID != reviewer.ID || !reviewerTask.ReviewOnly || !swarmContractReviewerRole(run, *reviewerTask, taskID) || reviewer.ReviewAssignmentDigest != swarmResolvedReviewAssignmentDigest(item) || reviewer.ChildID != item.ReviewerChildID || reviewer.Handoff == nil || reviewer.Handoff.Digest != item.ReviewerHandoffDigest {
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
