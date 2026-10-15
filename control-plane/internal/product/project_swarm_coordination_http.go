package product

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func swarmCoordinationAccess(w http.ResponseWriter, r *http.Request, edit bool) bool {
	if r.Context().Err() != nil {
		return false
	}
	if access, scoped := management.TeamAccessFromContext(r.Context()); scoped &&
		(access.ProjectID != r.PathValue("id") || !access.Permissions.Read || edit && !access.Permissions.Edit) {
		journeyError(w, http.StatusForbidden, "project_unavailable", "This project is not available in the current workspace.")
		return false
	}
	return true
}

func (s *Server) getProjectSwarmSupervision(w http.ResponseWriter, r *http.Request) {
	if !swarmCoordinationAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	after, err := strconv.Atoi(r.URL.Query().Get("after"))
	if r.URL.Query().Get("after") == "" {
		after = 0
		err = nil
	}
	limit, limitErr := strconv.Atoi(r.URL.Query().Get("limit"))
	if r.URL.Query().Get("limit") == "" {
		limit = 25
		limitErr = nil
	}
	if err != nil || limitErr != nil || after < 0 || limit < 1 || limit > 50 {
		journeyError(w, http.StatusBadRequest, "invalid_cursor", "Choose a nonnegative message cursor and a page size from 1 to 50.")
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
	queue, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.mu.RLock()
	state, stateErr := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	freshness := projectRunContextFreshness(run, state, stateErr)
	messages := []ProjectSwarmMessage{}
	for _, item := range run.Swarm.Messages {
		if item.Sequence <= after {
			continue
		}
		if len(messages) == limit {
			break
		}
		messages = append(messages, item)
	}
	next := after
	if len(messages) > 0 {
		next = messages[len(messages)-1].Sequence
	}
	omitted := len(run.Swarm.Messages) - next
	if omitted < 0 {
		omitted = 0
	}
	type taskView struct {
		TaskID           string `json:"taskId"`
		OperationID      string `json:"operationId"`
		Role             string `json:"role"`
		State            string `json:"state"`
		IntegrationState string `json:"integrationState"`
	}
	tasks := []taskView{}
	activity, quality, settlement := "ready", "not-started", "confirmed"
	for _, task := range run.Swarm.Tasks {
		view := taskView{TaskID: task.ID, OperationID: task.OperationID, Role: "coding", State: "queued", IntegrationState: "none"}
		if task.ReviewOnly {
			view.Role = "review-only"
		}
		if task.Integration != nil {
			view.IntegrationState = task.Integration.State
			if task.Integration.State == "integrated" {
				view.State = "integrated"
				quality = "needs-review"
			} else {
				view.State = "unknown"
				settlement = "uncertain"
			}
		}
		if op := swarmOperation(&run, task.OperationID); op != nil && task.Integration == nil {
			switch op.Status {
			case "reserved":
				view.State = "reserved"
				if activity == "ready" {
					activity = "queued"
				}
			case "dispatching":
				view.State = "unknown"
				settlement = "uncertain"
				if activity != "working" {
					activity = "unknown"
				}
			case "running":
				view.State = "working"
				activity = "working"
			case "review-required", "succeeded":
				view.State = "review-required"
				quality = "needs-review"
			case "failed", "cancelled":
				view.State = "blocked"
				if activity != "working" {
					activity = "blocked"
				}
			case "unknown":
				view.State = "unknown"
			default:
				view.State = "unknown"
			}
			if op.Status == "unknown" || op.Allocation == "unknown" || swarmRetirementPending(*op) {
				settlement = "uncertain"
				view.State = "unknown"
			}
			if task.ReviewOnly && op.Handoff != nil && view.State == "review-required" {
				for _, item := range run.Swarm.ReviewObligations {
					if item.ReviewerOperationID == op.ID && item.State == "passed" && item.ReviewerHandoffDigest == op.Handoff.Digest {
						view.State = "review-passed"
						break
					}
				}
			}
		}
		tasks = append(tasks, view)
	}
	if run.Swarm.State == "paused" {
		activity = "paused"
	}
	if run.Swarm.State == "cancelled" || run.State == "cancelled" {
		activity = "stopped"
	}
	if run.State == "blocked" {
		activity = "blocked"
	}
	if settlement == "uncertain" && activity == "ready" {
		activity = "unknown"
	}
	for _, item := range run.Swarm.ReviewObligations {
		if item.State == "repair-required" {
			quality = "repair-required"
		} else if item.State == "unknown" {
			settlement = "uncertain"
		} else if (item.State == "pending" || item.State == "passed") && quality != "repair-required" {
			quality = "needs-review"
		}
	}
	if run.State == "awaiting-checks" {
		quality = "awaiting-checks"
	}
	if run.State == "complete" {
		quality = "checked"
	}
	type attentionView struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		TaskID string `json:"taskId"`
		Action struct {
			Kind     string `json:"kind"`
			TargetID string `json:"targetId"`
		} `json:"action"`
	}
	attention := []attentionView{}
	addAttention := func(id, kind, taskID, actionKind, targetID string) {
		item := attentionView{ID: id, Kind: kind, TaskID: taskID}
		item.Action.Kind, item.Action.TargetID = actionKind, targetID
		attention = append(attention, item)
	}
	for _, msg := range run.Swarm.Messages {
		if msg.Kind == "question" {
			addAttention(msg.ID, "question", msg.RecipientTaskID, "open-message", msg.ID)
		}
	}
	for _, item := range run.Swarm.ReviewObligations {
		if item.State == "pending" || item.State == "repair-required" || item.State == "unknown" {
			kind := "review"
			if item.State == "repair-required" {
				kind = "repair"
			}
			if item.State == "unknown" {
				kind = "unknown"
			}
			addAttention(item.ID, kind, item.TaskID, "open-review-obligation", item.ID)
		}
	}
	for _, op := range run.Operations {
		if op.Status == "unknown" || op.Allocation == "unknown" || swarmRetirementPending(op) {
			addAttention(op.ID, "unknown", op.TaskID, "inspect-recovery", op.ID)
		}
	}
	contractDigest := ""
	if run.Input.OrchestrationContract != nil {
		contractDigest = run.Input.OrchestrationContract.Digest
	}
	journeyJSON(w, 200, map[string]any{
		"version": 1, "projectId": p.ID, "runId": run.ID, "runRevision": run.Revision, "queueRevision": queue.Revision,
		"observedAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"context":       map[string]any{"briefRevision": run.Input.Implementation.BriefRevision, "planRevision": run.Input.Implementation.PlanRevision, "contractDigest": contractDigest, "freshness": freshness.Status},
		"activityPhase": activity, "qualityPhase": quality, "settlement": settlement,
		"tasks": tasks, "messages": messages, "reviewObligations": swarmReviewObligationViews(run),
		"page":      map[string]any{"after": after, "nextCursor": next, "hasMore": omitted > 0, "omitted": omitted},
		"attention": attention, "metrics": map[string]any{"routineDecisions": nil, "latency": nil, "quality": nil, "throughput": nil, "cost": nil},
	})
}

// An exact scoped read bypasses the bounded display window. Missing remains
// missing; it is not evidence that an in-flight acknowledgement failed.
func (s *Server) getProjectSwarmMessage(w http.ResponseWriter, r *http.Request) {
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
	for _, msg := range run.Swarm.Messages {
		if msg.ID == r.PathValue("messageId") {
			journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "message": msg, "runRevision": run.Revision})
			return
		}
	}
	projectRunProblem(w, ErrProjectRunNotFound)
}

type swarmMessagePostRequest struct {
	ExpectedRevision int                         `json:"expectedRevision"`
	IdempotencyKey   string                      `json:"idempotencyKey"`
	RecipientTaskID  string                      `json:"recipientTaskId"`
	Kind             string                      `json:"kind"`
	CausalMessageID  string                      `json:"causalMessageId"`
	Evidence         ProjectSwarmMessageEvidence `json:"evidence"`
	Summary          string                      `json:"summary"`
}

func swarmMessageRequestDigest(req swarmMessagePostRequest) string {
	return implementationDigest(struct {
		Recipient, Kind, Causal string
		Evidence                ProjectSwarmMessageEvidence
		Summary                 string
	}{req.RecipientTaskID, req.Kind, req.CausalMessageID, req.Evidence, req.Summary})
}

// This POST is effect-free: the body contains the original claim, so no
// private summary or stable key appears in a URL or intermediary access log.
// An absent key is definitively rejected only after the saved run revision
// passes its CAS revision. At the same revision, an in-flight write may win.
func (s *Server) projectSwarmMessageDisposition(w http.ResponseWriter, r *http.Request) {
	if !swarmCoordinationAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req swarmMessagePostRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !validRunKey(req.IdempotencyKey) || !swarmMessageKind(req.Kind) || !handoffText(req.Summary, 1000) || strings.TrimSpace(req.Summary) == "" || !validSwarmMessageEvidenceShape(req.Evidence) {
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
	state := "unconfirmed"
	var found *ProjectSwarmMessage
	for i := range run.Swarm.Messages {
		msg := &run.Swarm.Messages[i]
		if msg.Sender.OperationID != r.PathValue("operationId") || msg.KeyDigest != implementationDigest(req.IdempotencyKey) {
			continue
		}
		if msg.RequestDigest == swarmMessageRequestDigest(req) {
			state = "recorded"
			found = msg
		} else {
			state = "key-conflict"
		}
		break
	}
	if state == "unconfirmed" && run.Revision > req.ExpectedRevision {
		state = "rejected"
	}
	if run.Revision < req.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	journeyJSON(w, 200, map[string]any{"projectId": p.ID, "runId": run.ID, "operationId": r.PathValue("operationId"), "state": state, "runRevision": run.Revision, "message": found})
}

func (s *Server) postProjectSwarmMessage(w http.ResponseWriter, r *http.Request) {
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
	if !validRunKey(req.IdempotencyKey) || !swarmMessageKind(req.Kind) || !handoffText(req.Summary, 1000) || strings.TrimSpace(req.Summary) == "" {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	digest := swarmMessageRequestDigest(req)
	previous, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if previous.Swarm == nil {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	for _, prior := range previous.Swarm.Messages {
		if prior.Sender.OperationID != r.PathValue("operationId") || prior.KeyDigest != implementationDigest(req.IdempotencyKey) {
			continue
		}
		if prior.RequestDigest != digest {
			projectRunProblem(w, ErrProjectRunKeyConflict)
			return
		}
		journeyJSON(w, 200, map[string]any{"message": prior, "runRevision": previous.Revision})
		return
	}
	if previous.Input.OrchestrationContract != nil {
		if err := s.contractCurrentForRun(p, previous); err != nil {
			projectRunProblem(w, err)
			return
		}
	}
	var msg ProjectSwarmMessage
	coordinatorIndependentReviewBeforeMutation(r.Context())
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if err := s.checkCoordinatorIndependentReviewGate(r.Context(), p, *run, "finding"); err != nil {
			return err
		}
		if run.Swarm == nil || run.State == "cancelled" || swarmTask(run, req.RecipientTaskID) == nil {
			return ErrProjectRunConflict
		}
		op := swarmOperation(run, r.PathValue("operationId"))
		if op == nil || op.Allocation != "ready" || op.Implementation == nil || op.ChildID == "" || swarmTask(run, op.TaskID) == nil || swarmTask(run, op.TaskID).OperationID != op.ID {
			return ErrProjectRunConflict
		}
		if err := validateSwarmMessageEvidence(*op, req.Evidence); err != nil {
			return err
		}
		for _, prior := range run.Swarm.Messages {
			if prior.Sender.OperationID == op.ID && prior.KeyDigest == implementationDigest(req.IdempotencyKey) {
				return ErrProjectRunConflict
			}
		}
		msg = ProjectSwarmMessage{ID: newID("msg-"), Kind: req.Kind, Sender: ProjectSwarmMessageSender{TaskID: op.TaskID, OperationID: op.ID, ChildID: op.ChildID, WorktreeID: op.WorktreeID, ContextDigest: implementationDigest(op.Implementation)}, RecipientTaskID: req.RecipientTaskID, CausalMessageID: req.CausalMessageID, Evidence: req.Evidence, Summary: req.Summary, Attribution: "coordinator-recorded", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), KeyDigest: implementationDigest(req.IdempotencyKey), RequestDigest: digest}
		var appendErr error
		msg, appendErr = appendSwarmMessage(run, msg)
		return appendErr
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"message": msg, "runRevision": run.Revision})
}

func (s *Server) ackProjectSwarmMessage(w http.ResponseWriter, r *http.Request) {
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
	actor, ok := authenticatedContextActor(r.Context())
	if !ok {
		projectRunProblem(w, errCoordinatorAuthority)
		return
	}
	var req struct {
		ExpectedRevision     int    `json:"expectedRevision"`
		RecipientOperationID string `json:"recipientOperationId"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	previous, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if previous.Swarm == nil {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	for _, prior := range previous.Swarm.Messages {
		if prior.ID != r.PathValue("messageId") {
			continue
		}
		if prior.Ack != nil && prior.Ack.OperationID == req.RecipientOperationID && prior.Ack.ActorID == actor.ID {
			journeyJSON(w, 200, map[string]any{"message": prior, "runRevision": previous.Revision})
			return
		}
	}
	var message ProjectSwarmMessage
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, r.PathValue("runId"), req.ExpectedRevision, func(run *ProjectRun) error {
		if run.Swarm == nil {
			return ErrProjectRunNotFound
		}
		op := swarmOperation(run, req.RecipientOperationID)
		if op == nil || op.ChildID == "" || op.Allocation != "ready" || swarmTask(run, op.TaskID) == nil || swarmTask(run, op.TaskID).OperationID != op.ID {
			return ErrProjectRunConflict
		}
		for i := range run.Swarm.Messages {
			item := &run.Swarm.Messages[i]
			if item.ID != r.PathValue("messageId") {
				continue
			}
			if op.TaskID != item.RecipientTaskID {
				return ErrProjectRunConflict
			}
			if item.Ack == nil {
				item.Ack = &ProjectSwarmMessageAck{OperationID: op.ID, ChildID: op.ChildID, ActorID: actor.ID, At: time.Now().UTC().Format(time.RFC3339Nano)}
			} else if item.Ack.OperationID != op.ID || item.Ack.ChildID != op.ChildID || item.Ack.ActorID != actor.ID {
				return ErrProjectRunConflict
			}
			message = *item
			return nil
		}
		return ErrProjectRunNotFound
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"message": message, "runRevision": run.Revision})
}
