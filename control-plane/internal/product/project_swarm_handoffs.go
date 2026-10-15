package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func (s *Server) registerTaskHandoffs(mux Registrar) {
	const base = "/v1/product/projects/{id}/runs/{runId}/swarm/operations/{operationId}/handoff"
	mux.HandleFunc("GET "+base, s.getTaskHandoff)
	mux.HandleFunc("POST "+base+"/collect", s.collectProjectTaskHandoff)
	mux.HandleFunc("POST "+base+"/questions/{questionId}/answer", s.answerTaskHandoff)
}

func (s *Server) taskHandoffRequest(run ProjectRun, op ProjectRunOperation) (TaskHandoffCaptureRequest, error) {
	queueID, err := s.SwarmQueueIDForOperation(run.ProjectID, run.ID, op.ID)
	if err != nil {
		return TaskHandoffCaptureRequest{}, err
	}
	return TaskHandoffCaptureRequest{ProjectSwarmRecoveryRequest: ProjectSwarmRecoveryRequest{ProjectID: run.ProjectID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, QueueID: queueID, UpdatedAt: op.UpdatedAt}, TaskID: op.TaskID, WorktreeID: op.WorktreeID, RequestDigest: op.RequestDigest, ImplementationDigest: implementationDigest(op.Implementation)}, nil
}

func (s *Server) captureTaskHandoff(ctx context.Context, projectPath string, run ProjectRun, operationID string, refresh bool, authorize bool) (ProjectRun, error) {
	op := swarmOperation(&run, operationID)
	if op == nil || op.TaskID == "" || op.Allocation != "ready" || op.Implementation == nil {
		return run, ErrProjectRunInvalid
	}
	if op.StartFailure.Valid() {
		return run, errors.New("This attempt was refused before sandbox allocation. Its recorded start diagnostic is retained; there is no worker report to capture.")
	}
	if op.Handoff != nil && !refresh {
		return run, nil
	}
	if taskHandoffIntegration(run, *op) != nil {
		if refresh {
			return run, errors.New("This integrated or reviewed integration attempt retains its exact captured report. Start a separate attempt to change its output.")
		}
		return run, nil
	}
	if op.Status != "review-required" && op.Status != "failed" && op.Status != "succeeded" && op.Status != "cancelled" {
		return run, errors.New("Stop and observe this task before capturing its report.")
	}
	if s.taskHandoffCollector == nil {
		if authorize {
			return run, errors.New("Task report collection is unavailable on this execution service.")
		}
		return run, nil
	}
	req, err := s.taskHandoffRequest(run, *op)
	if err != nil {
		return run, err
	}
	if authorize {
		if s.taskHandoffAuthorize == nil {
			return run, errors.New("Current report authority is unavailable.")
		}
		if err = s.taskHandoffAuthorize(ctx, req.ProjectSwarmRecoveryRequest); err != nil {
			return run, err
		}
	}
	capture, err := s.taskHandoffCollector(ctx, req)
	if err != nil {
		return run, err
	}
	if capture.Release == nil {
		return run, errors.New("The task output guard was not retained.")
	}
	defer capture.Release()
	if capture.Status != "captured" && capture.Status != "missing" && capture.Status != "invalid" && capture.Status != "unavailable" || capture.Status == "captured" && (capture.Report == nil || !hexDigest(capture.ContextDigest)) {
		return run, ErrProjectRunInvalid
	}
	if capture.Status != "captured" && capture.Report != nil {
		return run, ErrProjectRunInvalid
	}
	if capture.Report != nil {
		raw, marshalErr := json.Marshal(capture.Report)
		if marshalErr != nil {
			return run, marshalErr
		}
		normalized, err := DecodeTaskHandoffReport(raw)
		if err != nil {
			return run, err
		}
		capture.Report = normalized
	}
	path, err := s.swarmPath(run.ProjectID, run.ID, op.ID)
	if err != nil {
		return run, err
	}
	if err = s.validateSwarmTaskGit(ctx, path); err != nil {
		return run, err
	}
	sourceDigest, commit, dirty, err := s.checkSource(ctx, path)
	if err != nil {
		return run, err
	}
	receipt := TaskHandoffReceipt{Version: 1, ProjectID: run.ProjectID, RunID: run.ID, TaskID: op.TaskID, OperationID: op.ID, ChildID: op.ChildID, WorktreeID: op.WorktreeID, RequestDigest: op.RequestDigest, ImplementationDigest: req.ImplementationDigest, ContextDigest: capture.ContextDigest, SourceCommit: commit, SourceDigest: sourceDigest, SourceDirty: dirty, Status: capture.Status, ReasonCode: capture.ReasonCode, CapturedAt: time.Now().UTC().Format(time.RFC3339Nano), Reported: capture.Report}
	if op.Handoff != nil {
		prior, err := s.projectRuns.taskHandoffView(run, *op)
		if err != nil {
			return run, err
		}
		compare := receipt
		compare.CapturedAt = prior.Receipt.CapturedAt
		if implementationDigest(compare) == op.Handoff.Digest {
			if authorize {
				if err := s.taskHandoffAuthorize(ctx, req.ProjectSwarmRecoveryRequest); err != nil {
					return run, err
				}
			}
			return run, nil
		}
	}
	digest, err := s.projectRuns.writeHandoffObject(run.ProjectID, receipt, TaskHandoffReportLimit+16384)
	if err != nil {
		return run, err
	}
	ref := taskHandoffReference(receipt, digest)
	if authorize {
		if err = s.taskHandoffAuthorize(ctx, req.ProjectSwarmRecoveryRequest); err != nil {
			return run, err
		}
	}
	return s.projectRuns.mutateSwarm(run.ProjectID, projectPath, run.ID, run.Revision, func(current *ProjectRun) error {
		selected := swarmOperation(current, op.ID)
		if selected == nil || selected.ChildID != op.ChildID || selected.RequestDigest != op.RequestDigest || selected.Integration != nil {
			return ErrProjectRunConflict
		}
		selected.Handoff = ref
		if current.Swarm != nil && selected.Implementation != nil {
			for _, task := range current.Swarm.Tasks {
				depends := false
				for _, dependency := range task.DependsOn {
					if dependency == selected.TaskID {
						depends = true
						break
					}
				}
				if !depends {
					continue
				}
				summary := ref.Summary
				if summary == "" {
					summary = "An owned task report was captured with status " + ref.Status + ". Inspect its complete retained receipt before relying on the result."
				}
				key := implementationDigest(struct{ Sender, Recipient, Digest string }{selected.ID, task.ID, ref.Digest})
				message := ProjectSwarmMessage{ID: newID("msg-"), Kind: "dependency-result", Sender: ProjectSwarmMessageSender{TaskID: selected.TaskID, OperationID: selected.ID, ChildID: selected.ChildID, WorktreeID: selected.WorktreeID, ContextDigest: implementationDigest(selected.Implementation)}, RecipientTaskID: task.ID, Evidence: ProjectSwarmMessageEvidence{Kind: "handoff", ID: selected.ID, Digest: ref.Digest}, Summary: summary, Attribution: "owned-worker-report", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), KeyDigest: key, RequestDigest: key}
				if _, err := appendSwarmMessage(current, message); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func taskHandoffReference(receipt TaskHandoffReceipt, digest string) *TaskHandoffReference {
	ref := &TaskHandoffReference{Digest: digest, Status: receipt.Status, CapturedAt: receipt.CapturedAt}
	if receipt.Reported != nil {
		ref.Questions = len(receipt.Reported.Questions)
		ref.Unanswered = ref.Questions
		ref.Summary = receipt.Reported.Summary
		for len(ref.Summary) > 1000 {
			ref.Summary = ref.Summary[:len(ref.Summary)-1]
		}
		for !handoffText(ref.Summary, 1000) {
			ref.Summary = ref.Summary[:len(ref.Summary)-1]
		}
	}
	return ref
}

func (s *Server) getTaskHandoff(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	op := swarmOperation(&run, r.PathValue("operationId"))
	if op == nil || op.TaskID == "" {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	view, err := s.projectRuns.taskHandoffView(run, *op)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"handoff": view})
}

func (s *Server) collectProjectTaskHandoff(w http.ResponseWriter, r *http.Request) {
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
	if !s.projectRunDispatchMu.TryLock() {
		projectRunProblem(w, ErrProjectRunPending)
		return
	}
	defer s.projectRunDispatchMu.Unlock()
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if run.Revision != req.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	run, err = s.captureTaskHandoff(r.Context(), p.Path, run, r.PathValue("operationId"), true, true)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	view, err := s.projectRuns.taskHandoffView(run, *swarmOperation(&run, r.PathValue("operationId")))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"run": s.projectRunView(run), "handoff": view})
}

func (s *Server) answerTaskHandoff(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		HandoffDigest    string `json:"handoffDigest"`
		Answer           string `json:"answer"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Answer) == "" || !handoffText(req.Answer, 4000) {
		projectRunProblem(w, errors.New("Write an answer of at most 4,000 bytes."))
		return
	}
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	op := swarmOperation(&run, r.PathValue("operationId"))
	if run.Revision != req.ExpectedRevision || op == nil || op.Handoff == nil || op.Handoff.Digest != req.HandoffDigest {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	bound, err := s.taskHandoffRequest(run, *op)
	if err != nil || s.taskHandoffAuthorize == nil {
		projectRunProblem(w, ErrProjectRunInvalid)
		return
	}
	if err = s.taskHandoffAuthorize(r.Context(), bound.ProjectSwarmRecoveryRequest); err != nil {
		projectRunProblem(w, err)
		return
	}
	view, err := s.projectRuns.taskHandoffView(run, *op)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	found := false
	for _, id := range view.QuestionIDs {
		found = found || id == r.PathValue("questionId")
	}
	if !found {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	answers := taskHandoffAnswers{Version: 1, ReportDigest: req.HandoffDigest, Answers: []TaskHandoffAnswer{}, Actors: []string{}}
	if op.Handoff.AnswersDigest != "" {
		if err = s.projectRuns.readHandoffObject(p.ID, op.Handoff.AnswersDigest, &answers, 64<<10); err != nil {
			projectRunProblem(w, err)
			return
		}
	}
	actor, err := management.CaptureExecutionControlActor(r.Context(), p.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	actorID := actor.UserID
	if actor.Local {
		actorID = "local-owner"
	}
	answer := TaskHandoffAnswer{QuestionID: r.PathValue("questionId"), Answer: req.Answer, AnsweredAt: time.Now().UTC().Format(time.RFC3339Nano), Attribution: "project-editor"}
	at := -1
	for i, value := range answers.Answers {
		if value.QuestionID == answer.QuestionID {
			at = i
			if value.Answer == answer.Answer {
				if err = s.taskHandoffAuthorize(r.Context(), bound.ProjectSwarmRecoveryRequest); err != nil {
					projectRunProblem(w, err)
					return
				}
				journeyJSON(w, 200, map[string]any{"run": s.projectRunView(run), "handoff": view})
				return
			}
		}
	}
	if at < 0 {
		answers.Answers = append(answers.Answers, answer)
		answers.Actors = append(answers.Actors, actorID)
	} else {
		answers.Answers[at], answers.Actors[at] = answer, actorID
	}
	digest, err := s.projectRuns.writeHandoffObject(p.ID, answers, 64<<10)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if err = s.taskHandoffAuthorize(r.Context(), bound.ProjectSwarmRecoveryRequest); err != nil {
		projectRunProblem(w, err)
		return
	}
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
		selected := swarmOperation(current, op.ID)
		if selected == nil || selected.Handoff == nil || selected.Handoff.Digest != req.HandoffDigest {
			return ErrProjectRunConflict
		}
		copy := *selected.Handoff
		copy.AnswersDigest = digest
		copy.Unanswered = copy.Questions - len(answers.Answers)
		selected.Handoff = &copy
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	view, err = s.projectRuns.taskHandoffView(run, *swarmOperation(&run, op.ID))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"run": s.projectRunView(run), "handoff": view})
}
