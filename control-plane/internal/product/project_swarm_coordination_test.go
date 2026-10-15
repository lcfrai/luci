package product

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSwarmAddressedMessageReplayAckAndReadOnlyProjection(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	actor := coordinationHandler(s, "alice")
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "coordination-task-a-123456")
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "coordination-task-b-123456")
	a, b := run.Operations[0], run.Operations[1]
	request := map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "message-request-1234567890", "recipientTaskId": "task-b", "kind": "question", "summary": "Please check the saved dependency interface.", "evidence": map[string]string{"kind": "none"}}
	var created struct {
		Message     ProjectSwarmMessage `json:"message"`
		RunRevision int                 `json:"runRevision"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/operations/"+a.ID+"/messages", request, 200).Body.Bytes(), &created)
	if created.Message.Sender.OperationID != a.ID || created.Message.Sender.ChildID != a.ChildID || created.Message.RecipientTaskID != "task-b" || created.Message.Sequence != 1 {
		t.Fatalf("wrong scope: %+v", created)
	}
	stored, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || len(stored.Swarm.Messages) != 1 {
		t.Fatal(err, stored)
	}
	before := auditTree(t, s.stateDir)
	var replay struct {
		Message     ProjectSwarmMessage `json:"message"`
		RunRevision int                 `json:"runRevision"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/operations/"+a.ID+"/messages", request, 200).Body.Bytes(), &replay)
	if replay.Message.ID != created.Message.ID || replay.RunRevision != created.RunRevision || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("message replay changed durable state")
	}
	request["summary"] = "Different payload"
	journeyRequest(t, actor, "POST", base+"/operations/"+a.ID+"/messages", request, 409)
	var supervision struct {
		ProjectID  string                `json:"projectId"`
		ObservedAt string                `json:"observedAt"`
		Messages   []ProjectSwarmMessage `json:"messages"`
		Attention  []struct {
			Action struct {
				Kind     string `json:"kind"`
				TargetID string `json:"targetId"`
			} `json:"action"`
		} `json:"attention"`
		Page struct {
			Omitted    int `json:"omitted"`
			NextCursor int `json:"nextCursor"`
		} `json:"page"`
		Metrics map[string]any `json:"metrics"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/supervision?after=0&limit=1", nil, 200).Body.Bytes(), &supervision)
	if len(supervision.Messages) != 1 || supervision.Messages[0].Ack != nil || supervision.ProjectID != p.ID || supervision.ObservedAt == "" || len(supervision.Attention) != 1 || supervision.Attention[0].Action.Kind != "open-message" || supervision.Attention[0].Action.TargetID != created.Message.ID || supervision.Page.NextCursor != 1 || supervision.Page.Omitted != 0 || supervision.Metrics["quality"] != nil || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("supervision mutated or fabricated measured progress", supervision)
	}
	revoked := httptest.NewRequest("GET", base+"/supervision", nil)
	revoked = revoked.WithContext(management.WithTeamAccess(revoked.Context(), management.TeamAccess{ProjectID: "p-foreign", Permissions: management.TeamPermissions{Read: true, Edit: true}}))
	denied := httptest.NewRecorder()
	mux.ServeHTTP(denied, revoked)
	if denied.Code != 403 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("revoked project grant read private swarm projection", denied.Code)
	}
	journeyRequest(t, actor, "POST", base+"/messages/"+created.Message.ID+"/ack", map[string]any{"expectedRevision": created.RunRevision, "recipientOperationId": a.ID}, 409)
	var ack struct {
		Message     ProjectSwarmMessage `json:"message"`
		RunRevision int                 `json:"runRevision"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/messages/"+created.Message.ID+"/ack", map[string]any{"expectedRevision": created.RunRevision, "recipientOperationId": b.ID}, 200).Body.Bytes(), &ack)
	if ack.Message.Ack == nil || ack.Message.Ack.OperationID != b.ID || ack.Message.Ack.ChildID != b.ChildID {
		t.Fatalf("ack lacked exact recipient: %+v", ack)
	}
	before = auditTree(t, s.stateDir)
	journeyRequest(t, actor, "POST", base+"/messages/"+created.Message.ID+"/ack", map[string]any{"expectedRevision": created.RunRevision, "recipientOperationId": b.ID}, 200)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("ack replay changed durable state")
	}
}

func TestSwarmIndependentReviewerCaptureBindsCandidatePatch(t *testing.T) {
	s, mux, p, run, base, selected := swarmHTTPFixture(t,
		ProjectTask{ID: "task-a", Title: "Build component", Status: "done", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Component works"},
		ProjectTask{ID: "task-b", Title: "Independently review component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{}, Acceptance: "Inspect candidate source and report findings"},
		ProjectTask{ID: "task-c", Title: "Use component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}},
	)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "review-candidate-123456")
	candidate := run.Operations[0]
	path, err := s.swarmPath(p.ID, run.ID, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	commit := swarmCommitFixture(t, path, "alpha.txt", "candidate content\n")
	if _, err := os.Stat(filepath.Join(p.Path, "alpha.txt")); !os.IsNotExist(err) {
		t.Fatal("candidate leaked to main")
	}
	route := *selected
	route.WorktreeID = candidate.WorktreeID
	child := projectRunCandidate(route, candidate.Implementation)
	child.ID, child.ProjectRunID, child.ProjectRunOperationID, child.Status, child.ExitReason = candidate.ChildID, run.ID, candidate.ID, registry.StatusExited, "exited"
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.SetProjectRunSessionObserver(func(_ context.Context, _, _, operationID string) ([]registry.Session, error) {
		if operationID == candidate.ID {
			return []registry.Session{child}, nil
		}
		return nil, nil
	})
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/reconcile", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if !review.CanIntegrate || review.SourceCommit != commit || !strings.Contains(review.Patch, "+candidate content") {
		t.Fatalf("candidate not reviewable: %+v", review)
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "reviewer-reserved-123456")
	reviewer := run.Operations[1]
	actor := coordinationHandler(s, "alice")
	var created struct {
		Obligation  ProjectSwarmReviewObligationView `json:"obligation"`
		RunRevision int                              `json:"runRevision"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/tasks/task-a/review-obligations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "independent-review-123456789", "candidateOperationId": candidate.ID, "reviewerOperationId": reviewer.ID, "reviewDigest": review.ReviewDigest}, 201).Body.Bytes(), &created)
	if created.Obligation.CandidateCommit != commit || created.Obligation.ReviewerOperationID != reviewer.ID || created.Obligation.State != "pending" || created.Obligation.RepairLimit != 2 || created.Obligation.RepairsConsumed != 0 {
		t.Fatalf("review receipt mismatch: %+v", created)
	}
	stored, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !swarmTask(&stored, "task-b").ReviewOnly || swarmAllIntegrated(stored) {
		t.Fatal("review-only task bypassed its independent receipt")
	}
	var projected struct {
		ProjectID         string                             `json:"projectId"`
		ReviewObligations []ProjectSwarmReviewObligationView `json:"reviewObligations"`
		Tasks             []struct {
			TaskID string `json:"taskId"`
			Role   string `json:"role"`
		} `json:"tasks"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "GET", base+"/supervision", nil, 200).Body.Bytes(), &projected)
	if projected.ProjectID != p.ID || len(projected.ReviewObligations) != 1 || projected.ReviewObligations[0].ID != created.Obligation.ID || projected.ReviewObligations[0].RepairLimit != 2 || len(projected.Tasks) != 3 || projected.Tasks[1].TaskID != "task-b" || projected.Tasks[1].Role != "review-only" {
		t.Fatal("supervision lost typed review and reviewer role", projected)
	}
	bound, err := s.swarmTaskCapture(context.Background(), stored, *swarmOperation(&stored, reviewer.ID), mustSwarmPathFixture(t, s, p.ID, run.ID, reviewer.ID))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range bound.Files {
		if file.Path == "LUCI.md" {
			found = strings.Contains(string(file.Data), "+candidate content") && strings.Contains(string(file.Data), review.ReviewDigest)
		}
	}
	if !found {
		t.Fatal("reviewer did not receive exact candidate patch and receipt")
	}
	// No read integrates, dispatches, or mutates the saved run.
	before := auditTree(t, s.stateDir)
	journeyRequest(t, actor, "GET", base+"/supervision", nil, 200)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("supervision read mutated run")
	}
	run = swarmFinishFixture(t, s, p, stored, reviewer.ID)
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		if req.OperationID != reviewer.ID {
			t.Fatal("foreign reviewer capture", req)
		}
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("reviewer captured source"), Report: &TaskHandoffReport{Version: 1, Summary: "Independent review found a missing verification case", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect candidate diff", Outcome: "failed", Detail: "Acceptance case missing"}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	var handoff struct {
		Run     ProjectRun      `json:"run"`
		Handoff TaskHandoffView `json:"handoff"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/operations/"+reviewer.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &handoff)
	run = handoff.Run
	var finding struct {
		Message     ProjectSwarmMessage `json:"message"`
		RunRevision int                 `json:"runRevision"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/operations/"+reviewer.ID+"/messages", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "review-finding-123456789", "recipientTaskId": "task-a", "kind": "review-finding", "evidence": map[string]any{"kind": "handoff", "id": reviewer.ID, "digest": handoff.Handoff.Digest}, "summary": "Independent review found a missing acceptance case."}, 200).Body.Bytes(), &finding)
	var resolved struct {
		Obligation  ProjectSwarmReviewObligationView `json:"obligation"`
		RunRevision int                              `json:"runRevision"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/review-obligations/"+created.Obligation.ID+"/resolve", map[string]any{"expectedRevision": finding.RunRevision, "outcome": "repair-required", "reviewerHandoffDigest": handoff.Handoff.Digest, "findingMessageId": finding.Message.ID}, 200).Body.Bytes(), &resolved)
	if resolved.Obligation.State != "repair-required" || resolved.Obligation.ReviewerHandoffDigest != handoff.Handoff.Digest || resolved.Obligation.RepairsConsumed != 1 {
		t.Fatal("independent failure was lost", resolved)
	}
	stored, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	contracted := stored
	contracted.Input.OrchestrationContract = &OrchestrationContract{ID: "approved-contract-fixture"}
	if swarmIndependentReviewReady(contracted, "task-a", review) {
		t.Fatal("failed independent review authorized integration", err)
	}
	checkpoint := &ProjectSwarmCheckpoint{SourceOperationID: candidate.ID, SourceChildID: candidate.ChildID, SourceCommit: commit, SourceBaseCommit: candidate.BaseCommit, TargetCommit: candidate.BaseCommit, ReviewDigest: review.ReviewDigest, ContextDigest: implementationDigest(candidate.Implementation), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	repaired, next, newAttempt, err := s.projectRuns.reserveSwarmTask(p.ID, p.Path, run.ID, stored.Revision, "repair-attempt-123456789", "task-a", candidate.BaseCommit, func(id string) string {
		return project.WorktreeID(filepath.Join(s.projectRoot(), ".luci-swarms", p.ID, run.ID, id))
	}, checkpoint)
	if err != nil || !newAttempt || repaired.Swarm.ReviewObligations[0].RepairOperationID != next.ID || next.Checkpoint == nil || next.Checkpoint.SourceOperationID != candidate.ID {
		t.Fatal("repair lost exact candidate and attempt identity", err, repaired.Swarm.ReviewObligations, next)
	}
}

func TestSwarmAddressedCaptureIsScopedAndImmutable(t *testing.T) {
	s := &Server{}
	op := ProjectRunOperation{ID: "op-000000000000000000000002", TaskID: "task-b"}
	run := ProjectRun{ID: "run-000000000000000000000001", Swarm: &ProjectSwarm{Messages: []ProjectSwarmMessage{
		{ID: "msg-000000000000000000000001", RecipientTaskID: "task-b", Summary: "Review the exact dependency result."},
		{ID: "msg-000000000000000000000002", RecipientTaskID: "task-c", Summary: "Private other task note."},
	}}}
	prior := &ReviewedContextCapture{Files: []ReviewedContextFile{{Path: "LUCI.md", Data: []byte("Original reviewed task\n")}}}
	captured, err := s.appendSwarmCoordinationCapture(context.Background(), project.Project{}, run, op, prior)
	if err != nil {
		t.Fatal(err)
	}
	text := string(captured.Files[0].Data)
	if !strings.Contains(text, "Review the exact dependency result.") || strings.Contains(text, "Private other task note.") || !strings.Contains(text, "untrusted") {
		t.Fatal("addressed scope or authority wording lost", text)
	}
	if string(prior.Files[0].Data) != text {
		t.Fatal("returned capture diverged")
	}
	if len(run.Swarm.Messages) != 2 || run.Swarm.Messages[0].Ack != nil {
		t.Fatal("capture mutated retained run")
	}
}

func TestSwarmReviewGateRetainsLegacyAndRequiresExactIndependentReceipt(t *testing.T) {
	run := ProjectRun{Swarm: &ProjectSwarm{}}
	review := ProjectSwarmReview{TaskID: "task-a", OperationID: "op-000000000000000000000001", SourceCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ReviewDigest: implementationDigest("review")}
	if !swarmIndependentReviewReady(run, "task-a", review) {
		t.Fatal("legacy run changed authority without an approved contract")
	}
	run.Input.OrchestrationContract = &OrchestrationContract{ID: "contract-fixture"}
	if swarmIndependentReviewReady(run, "task-a", review) {
		t.Fatal("new contract integrated without independent review")
	}
	run.Swarm.ReviewObligations = []ProjectSwarmReviewObligation{{TaskID: "task-a", CandidateOperationID: review.OperationID, CandidateCommit: review.SourceCommit, ReviewDigest: review.ReviewDigest, State: "passed", ReviewerHandoffDigest: implementationDigest("handoff"), FindingMessageID: "msg-000000000000000000000001"}}
	run.Swarm.ReviewObligations[0].ReviewerOperationID = "op-000000000000000000000002"
	run.Swarm.ReviewObligations[0].ReviewerChildID = "s-000000000000000000000002"
	run.Operations = []ProjectRunOperation{{ID: run.Swarm.ReviewObligations[0].ReviewerOperationID, ChildID: run.Swarm.ReviewObligations[0].ReviewerChildID, Handoff: &TaskHandoffReference{Digest: run.Swarm.ReviewObligations[0].ReviewerHandoffDigest}}}
	run.Swarm.Messages = []ProjectSwarmMessage{{ID: run.Swarm.ReviewObligations[0].FindingMessageID, Kind: "review-finding", Sender: ProjectSwarmMessageSender{OperationID: run.Swarm.ReviewObligations[0].ReviewerOperationID}, RecipientTaskID: "task-a", Evidence: ProjectSwarmMessageEvidence{Kind: "handoff", Digest: run.Swarm.ReviewObligations[0].ReviewerHandoffDigest}}}
	if !swarmIndependentReviewReady(run, "task-a", review) {
		t.Fatal("matching independent review was ignored")
	}
	review.ReviewDigest = implementationDigest("amended source")
	if swarmIndependentReviewReady(run, "task-a", review) {
		t.Fatal("old review authorized changed candidate")
	}
}

func TestSwarmReviewOnlyTaskDoesNotFabricateCheckedCompletion(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	run := ProjectRun{Swarm: &ProjectSwarm{Tasks: []ProjectSwarmTask{{ID: "task-a", OperationID: "op-candidate", Integration: &ProjectSwarmIntegration{State: "integrated"}}, {ID: "task-review", OperationID: "op-reviewer", ReviewOnly: true}}, ReviewObligations: []ProjectSwarmReviewObligation{{TaskID: "task-a", ReviewerOperationID: "op-reviewer", State: "repair-required", ReviewerHandoffDigest: digest}}}, Operations: []ProjectRunOperation{{ID: "op-reviewer", Status: "succeeded", Handoff: &TaskHandoffReference{Digest: digest}}}}
	if swarmAllIntegrated(run) {
		t.Fatal("failed independent review was counted as checked completion")
	}
	run.Swarm.ReviewObligations[0].State = "passed"
	if !swarmAllIntegrated(run) {
		t.Fatal("passed reviewer and integrated code were not ready for combined checks")
	}
	run.Operations[0].Handoff.Digest = implementationDigest("different handoff")
	if swarmAllIntegrated(run) {
		t.Fatal("changed reviewer evidence still counted")
	}
}
