package product

import (
	"net/http"
	"strings"
)

// An explicit retirement advances the same run CAS used by the original
// request. Either the original write wins first, or this fence wins first; an
// absence/read timeout alone never proves rejection. Lost retirement replies
// are settled by the existing exact disposition/readback endpoints.
func (s *Server) retireProjectSwarmMessageRequest(w http.ResponseWriter, r *http.Request) {
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
	var req swarmMessagePostRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !validRunKey(req.IdempotencyKey) || !swarmMessageKind(req.Kind) || !handoffText(req.Summary, 1000) || strings.TrimSpace(req.Summary) == "" || !validSwarmMessageEvidenceShape(req.Evidence) || !swarmQuestionResolutionShape(req.Kind, req.CausalMessageID, req.ResolvesQuestionID) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if run.Swarm == nil || swarmOperation(run, r.PathValue("operationId")) == nil {
			return ErrProjectRunConflict
		}
		for _, msg := range run.Swarm.Messages {
			if msg.Sender.OperationID == r.PathValue("operationId") && msg.KeyDigest == implementationDigest(req.IdempotencyKey) {
				return ErrProjectRunConflict
			}
		}
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "operationId": r.PathValue("operationId"), "state": "rejected", "runRevision": run.Revision, "message": nil})
}

func (s *Server) retireProjectSwarmAckRequest(w http.ResponseWriter, r *http.Request) {
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
		RecipientOperationID string `json:"recipientOperationId"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !prototypeReleaseID.MatchString(req.RecipientOperationID) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if run.Swarm == nil {
			return ErrProjectRunNotFound
		}
		for _, msg := range run.Swarm.Messages {
			if msg.ID == r.PathValue("messageId") {
				if msg.Ack != nil {
					return ErrProjectRunConflict
				}
				return nil
			}
		}
		return ErrProjectRunNotFound
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "messageId": r.PathValue("messageId"), "state": "rejected", "runRevision": run.Revision})
}

func (s *Server) retireProjectSwarmReviewRequest(w http.ResponseWriter, r *http.Request) {
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
	if req.ExpectedRevision < 1 || !validRunKey(req.IdempotencyKey) || !hexDigest(req.ReviewDigest) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if run.Swarm == nil || swarmTask(run, r.PathValue("taskId")) == nil {
			return ErrProjectRunConflict
		}
		for _, item := range run.Swarm.ReviewObligations {
			if item.KeyDigest == implementationDigest(req.IdempotencyKey) {
				return ErrProjectRunConflict
			}
		}
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "taskId": r.PathValue("taskId"), "state": "rejected", "runRevision": run.Revision, "obligation": nil})
}

func (s *Server) retireProjectSwarmReviewResolution(w http.ResponseWriter, r *http.Request) {
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
	if req.ExpectedRevision < 1 || req.Outcome != "passed" && req.Outcome != "repair-required" || !hexDigest(req.ReviewerHandoffDigest) || !prototypeReleaseID.MatchString(req.FindingMessageID) {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if run.Swarm == nil {
			return ErrProjectRunNotFound
		}
		for _, item := range run.Swarm.ReviewObligations {
			if item.ID == r.PathValue("obligationId") {
				if item.State != "pending" {
					return ErrProjectRunConflict
				}
				return nil
			}
		}
		return ErrProjectRunNotFound
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "obligationId": r.PathValue("obligationId"), "state": "rejected", "runRevision": run.Revision})
}
