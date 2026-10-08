package product

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Actual product HTTP, retained Git changes and immutable private receipts;
// worker output is controlled data, not a live model or independent check.
func TestCloud011OrchestratorRetainsExactHandoffAfterIntegration(t *testing.T) {
	s, mux, p, run, base, selected := swarmHTTPFixture(t)
	s.registerTaskHandoffs(mux)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "orchestrator-handoff-attempt")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, "alpha.txt", "retained implementation\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	report := &TaskHandoffReport{Version: 1, Summary: "Worker reports implemented retention settings", ChangedFiles: []string{"alpha.txt"}, Decisions: []TaskHandoffDecision{{Decision: "Use project settings", Rationale: "Keep policy explicit"}}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "worker check", Outcome: "passed", Detail: "Unverified worker assertion"}}, Questions: []TaskHandoffQuestion{{Question: "How many days should records remain?", Impact: "Changes deletion behavior", Blocking: true}}, Limitations: []string{"Deletion has not been independently verified"}}
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		if req.ProjectID != p.ID || req.OperationID != op.ID || req.ChildID != op.ChildID {
			t.Fatal("wrong attempt captured", req)
		}
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: strings.Repeat("a", 64), Report: report}, nil
	})
	var captured struct {
		Run     ProjectRun      `json:"run"`
		Handoff TaskHandoffView `json:"handoff"`
	}
	route := base + "/operations/" + op.ID + "/handoff"
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", route+"/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &captured)
	run = captured.Run
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if !review.CanIntegrate {
		t.Fatal(review)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	connection := selected.AgentConnectionID
	req := projectRunCreateRequest{IdempotencyKey: "newer-plan-retains-worker-question", WorktreeID: run.WorktreeID, Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, AgentConnectionID: &connection, Outcome: run.Input.Outcome, ImplementationContext: *ImplementationContextSelection(&run.Input.Implementation)}
	newer := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/runs", req, 201)).ProjectRun
	before := auditTree(t, s.stateDir)
	var view OrchestratorView
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/orchestrator", nil, 200).Body.Bytes(), &view)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("snapshot read mutated private state")
	}
	h := view.Context.TaskHandoffs
	if h == nil || len(h.Entries) != 1 || h.AttemptCount != 1 || h.OmittedCount != 0 || view.Context.CurrentRun.ID != newer.ID {
		t.Fatal("newer run hid retained handoff", view.Context)
	}
	e := h.Entries[0]
	if e.RunID != run.ID || e.OperationID != op.ID || e.ChildID != op.ChildID || e.TaskID != "task-a" || e.Digest != captured.Handoff.Digest || e.ContextDigest != strings.Repeat("a", 64) || e.SourceCommit != captured.Handoff.Receipt.SourceCommit || e.WorkerSummary != report.Summary {
		t.Fatal("attempt/context provenance changed", e)
	}
	if !e.QuestionsKnown || e.PendingQuestionCount != 1 || len(e.WorkerQuestions) != 1 || e.WorkerQuestions[0].QuestionID != captured.Handoff.QuestionIDs[0] || !e.WorkerQuestions[0].WorkerReportedBlocking || e.WorkerReportedCheckCount != 1 || !e.RecordedChecksKnown || e.RecordedCheckCount != 0 || e.Integration == nil || e.Integration.State != "integrated" {
		t.Fatal("worker claims became checks or integration resolved ambiguity", e)
	}
	if len(view.BlockingQuestions) != 0 || len(view.Context.ActiveRuns) != 0 || len(view.Context.NeedsAttention) != 1 || view.Context.NeedsAttention[0].Kind != "task-question" || view.NextAction.RunID != run.ID {
		t.Fatal("worker question became global gate/capacity or lost attention", view)
	}
	overview := overviewGet(t, mux, p.ID)
	if len(overview.Attention.Items) != 1 || overview.Attention.Items[0].Kind != "task-question" {
		t.Fatal("overview omitted integrated question", overview.Attention)
	}
	// Answering is a separate human action bound to the original report.
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", route+"/questions/"+captured.Handoff.QuestionIDs[0]+"/answer", map[string]any{"expectedRevision": run.Revision, "handoffDigest": captured.Handoff.Digest, "answer": "30 days"}, 200).Body.Bytes(), &captured)
	// Each HTTP response is a fresh JSON object; an omitted now-empty optional
	// slice must not inherit data from the previous Go decoding destination.
	view = OrchestratorView{}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/orchestrator", nil, 200).Body.Bytes(), &view)
	e = view.Context.TaskHandoffs.Entries[0]
	if e.Digest != captured.Handoff.Digest || e.PendingQuestionCount != 0 || len(e.Answers) != 1 || e.Answers[0].Answer != captured.Handoff.Answers[0].Answer || e.Answers[0].Question != report.Questions[0].Question || e.Answers[0].Impact != report.Questions[0].Impact || e.RecordedCheckCount != 0 || len(view.Context.NeedsAttention) != 0 {
		t.Fatal("answer changed original provenance or granted verification", view.Context)
	}
	// Reusing another attempt's valid digest must yield unavailable, never its
	// worker questions or a known-zero count. No private receipt is changed.
	forged := captured.Run
	forged.Operations = append([]ProjectRunOperation{}, forged.Operations...)
	forged.Operations[0].ChildID = "different-attempt-child"
	wrong := s.projectRuns.orchestratorHandoffs([]ProjectRun{forged}).Entries[0]
	if wrong.DetailsStatus != "unavailable" || wrong.QuestionsKnown || wrong.RecordedChecksKnown || wrong.WorkerSummary != "" || len(wrong.Answers) != 0 {
		t.Fatal("foreign attempt receipt was attributed", wrong)
	}
}

func TestCloud011OrchestratorHandoffBoundsUnknownAndLegacy(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, s, id, path, input)
	run.Operations = nil
	for i := 0; i < 40; i++ {
		op := ProjectRunOperation{ID: fmt.Sprintf("op-%024d", i), TaskID: fmt.Sprintf("task-%d", i), ChildID: fmt.Sprintf("child-%d", i), WorktreeID: "worktree", Implementation: &input.Implementation}
		report := &TaskHandoffReport{Version: 1, Summary: strings.Repeat("s", 4000), ChangedFiles: []string{}, ReportedChecks: []TaskHandoffReportedCheck{}, Questions: []TaskHandoffQuestion{}, Decisions: []TaskHandoffDecision{}, Limitations: []string{}}
		for j := 0; j < 8; j++ {
			report.Questions = append(report.Questions, TaskHandoffQuestion{Question: strings.Repeat("q", 2000), Impact: strings.Repeat("i", 1000), Blocking: true})
			report.Decisions = append(report.Decisions, TaskHandoffDecision{Decision: strings.Repeat("d", 1000), Rationale: strings.Repeat("r", 1000)})
		}
		receipt := TaskHandoffReceipt{Version: 1, ProjectID: id, RunID: run.ID, TaskID: op.TaskID, OperationID: op.ID, ChildID: op.ChildID, WorktreeID: op.WorktreeID, ImplementationDigest: implementationDigest(op.Implementation), SourceCommit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64), Status: "captured", CapturedAt: "2026-10-08T12:00:00Z", Reported: report}
		digest, err := s.writeHandoffObject(id, receipt, TaskHandoffReportLimit+16384)
		if err != nil {
			t.Fatal(err)
		}
		op.Handoff = &TaskHandoffReference{Digest: digest, Status: "captured", Summary: "Compact reference", Questions: 8, Unanswered: 8, CapturedAt: receipt.CapturedAt}
		run.Operations = append(run.Operations, op)
	}
	projection := s.orchestratorHandoffs([]ProjectRun{run})
	encoded, err := json.MarshalIndent(projection, "", "  ")
	if err != nil || len(encoded) > orchestratorHandoffLimit || len(projection.Entries) != orchestratorHandoffReadLimit || projection.OmittedCount != 24 {
		t.Fatal("unbounded or silent projection omission", len(encoded), projection, err)
	}
	omitted := 0
	for _, e := range projection.Entries {
		if !e.QuestionsKnown || e.PendingQuestionCount != 8 || e.QuestionsOmitted+len(e.WorkerQuestions) != 8 || e.DecisionsOmitted+len(e.WorkerDecisions) != 8 {
			t.Fatal("truncated count", e)
		}
		omitted += e.QuestionsOmitted
	}
	if omitted == 0 {
		t.Fatal("fixture did not exercise explicit detail omission")
	}
	// Handoff detail shares the existing assistant run budget, including a
	// legal dense history whose active identities may never be discarded.
	history := make([]ProjectRun, projectRunHistoryLimit)
	for i := range history {
		history[i] = run
		history[i].ID = fmt.Sprintf("run-%024d", i)
		history[i].State = "blocked"
		history[i].Operations = append([]ProjectRunOperation{}, run.Operations...)
		for j := range history[i].Operations {
			history[i].Operations[j].Status = "running"
		}
	}
	latest, active := orchestratorRunProjectionWithHandoffs(history, journeyState{}, len(encoded))
	combined, err := json.MarshalIndent(OrchestratorContext{CurrentRun: latest, ActiveRuns: active, NeedsAttention: projectRunAttentionProjection(history, journeyState{}), TaskHandoffs: projection}, "", "  ")
	if err != nil || len(combined) >= orchestratorContextLimit-8*1024 || len(active) != projectRunHistoryLimit {
		t.Fatal("handoff consumed assistant identity headroom", len(combined), err)
	}
	run.Operations[0].Handoff = nil
	missing := s.orchestratorHandoffs([]ProjectRun{{ID: run.ID, ProjectID: id, Operations: run.Operations[:1]}})
	if missing.UnreportedCount != 1 || missing.Entries[0].Status != "not-recorded" || missing.Entries[0].QuestionsKnown || missing.Entries[0].RecordedChecksKnown {
		t.Fatal("missing report became known-zero", missing)
	}
	legacy, _ := json.Marshal(OrchestratorContext{})
	if strings.Contains(string(legacy), "taskHandoffs") {
		t.Fatal("legacy snapshots gained a new zero field")
	}
	for _, state := range []string{"complete", "cancelled", "ready"} {
		run.State = state
		if projectRunAttentionKind(run) != "task-question" {
			t.Fatal("unanswered old attempt hidden by parent state", state)
		}
	}
}
