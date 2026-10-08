package product

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTaskHandoffReportStrictClaimSchema(t *testing.T) {
	valid := `{"version":1,"summary":"Actual worker claim","changedFiles":["src/main.go"],"decisions":[],"reportedChecks":[{"command":"go test ./...","outcome":"passed","detail":"Worker reported exit zero"}],"questions":[{"question":"Which retention period?","impact":"Data lifecycle","blocking":true}],"limitations":[]}`
	for name, raw := range map[string]string{"valid": valid, "absent questions": strings.Replace(valid, `"questions":[{"question":"Which retention period?","impact":"Data lifecycle","blocking":true}],`, "", 1), "null questions": strings.Replace(valid, `"questions":[{"question":"Which retention period?","impact":"Data lifecycle","blocking":true}]`, `"questions":null`, 1), "duplicate field": strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1), "unknown authority": strings.Replace(valid, `"version":1`, `"version":1,"verified":true`, 1), "trailing": valid + ` {}`, "too large": strings.Replace(valid, "Actual worker claim", strings.Repeat("x", TaskHandoffReportLimit), 1), "traversal": strings.Replace(valid, "src/main.go", "../private", 1), "unknown check": strings.Replace(valid, `"outcome":"passed"`, `"outcome":"certified"`, 1), "invalid utf8": strings.Replace(valid, "Actual worker claim", string([]byte{0xff}), 1), "duplicate nested": strings.Replace(valid, `"blocking":true`, `"blocking":true,"blocking":false`, 1)} {
		t.Run(name, func(t *testing.T) {
			report, err := DecodeTaskHandoffReport([]byte(raw))
			if name == "valid" {
				if err != nil || report.ReportedChecks[0].Outcome != "passed" || !report.Questions[0].Blocking {
					t.Fatal(report, err)
				}
			} else if err == nil {
				t.Fatal("unsafe report accepted", report)
			}
		})
	}
}

func TestTaskHandoffRecordedClaimsQuestionsAndFrozenDependency(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.registerTaskHandoffs(mux)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "handoff-first-123456789")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, "alpha.txt", "actual retained source\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	report := &TaskHandoffReport{Version: 1, Summary: "A claims a complete component", ChangedFiles: []string{"alpha.txt"}, Decisions: []TaskHandoffDecision{{Decision: "Keep one component", Rationale: "Avoid duplicate state"}}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "fixture check", Outcome: "passed", Detail: "A claim, not independently joined evidence"}}, Questions: []TaskHandoffQuestion{{Question: "Which retention period?", Impact: "User data", Blocking: true}}, Limitations: []string{}}
	collected, released := 0, 0
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		collected++
		if req.ProjectID != p.ID || req.OperationID != op.ID || req.ChildID != op.ChildID {
			t.Fatal("capture widened", req)
		}
		return TaskHandoffCapture{Release: func() { released++ }, Status: "captured", ContextDigest: strings.Repeat("a", 64), Report: report}, nil
	})
	route := base + "/operations/" + op.ID + "/handoff"
	var response struct {
		Run     ProjectRun      `json:"run"`
		Handoff TaskHandoffView `json:"handoff"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", route+"/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &response)
	run = response.Run
	if collected != 1 || released != 1 || response.Handoff.Receipt.Reported == nil || len(response.Handoff.RecordedChecks) != 0 || len(response.Handoff.QuestionIDs) != 1 {
		t.Fatal("claims became checks or question disappeared", response.Handoff)
	}
	digest := response.Handoff.Digest
	question := response.Handoff.QuestionIDs[0]
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", route+"/questions/"+question+"/answer", map[string]any{"expectedRevision": run.Revision, "handoffDigest": digest, "answer": "Keep 30 days; reviewed product constraints still apply."}, 200).Body.Bytes(), &response)
	run = response.Run
	if response.Handoff.Digest != digest || len(response.Handoff.Answers) != 1 || response.Handoff.Receipt.Reported.Questions[0].Question != report.Questions[0].Question {
		t.Fatal("answer rewrote worker report", response.Handoff)
	}
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if !review.CanIntegrate {
		t.Fatal(review)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	journeyRequest(t, mux, "POST", route+"/collect", map[string]any{"expectedRevision": run.Revision}, 409)
	run = swarmReserveHTTP(t, mux, base, run, "task-c", "handoff-dependent-1234567")
	child := *swarmOperation(&run, swarmTask(&run, "task-c").OperationID)
	packet, err := s.projectRuns.taskDependencyPacket(run, child)
	if err != nil || len(packet.Dependencies) != 1 {
		t.Fatal(packet, err)
	}
	prior := packet.Dependencies[0]
	if prior.Digest != digest || prior.OperationID != op.ID || prior.Integration == nil || prior.Integration.State != "integrated" || len(prior.RecordedChecks) != 0 || len(prior.Answers) != 1 || prior.Answers[0].Answer != response.Handoff.Answers[0].Answer {
		t.Fatal("dependent received incorrect authority", prior)
	}
	before, _ := json.Marshal(packet)
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", route+"/questions/"+question+"/answer", map[string]any{"expectedRevision": run.Revision, "handoffDigest": digest, "answer": "A later amendment must not rewrite an already reserved child's inputs."}, 200).Body.Bytes(), &response)
	after, err := s.projectRuns.taskDependencyPacket(response.Run, child)
	encoded, _ := json.Marshal(after)
	if err != nil || !bytes.Equal(before, encoded) {
		t.Fatal("later answer changed frozen dependency", err)
	}
	// A normal listing still works after the separate handoff object store grew.
	if _, err = s.projectRuns.List(p.ID, p.Path); err != nil {
		t.Fatal("handoff directory poisoned run history", err)
	}
	reopened, err := NewProjectRunStore(s.projectRuns.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.taskDependencyPacket(loaded, child); err != nil {
		t.Fatal("frozen packet unavailable after restart", err)
	}
}

func TestTaskHandoffObjectRefusesSymlinkAndForeignDigest(t *testing.T) {
	s, id, _, _ := projectRunStoreFixture(t)
	value := map[string]string{"claim": "retained"}
	digest, err := s.writeHandoffObject(id, value, 4096)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := s.handoffDirectory(id, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, digest+".json")
	if err = os.WriteFile(path, []byte(`{"claim":"changed"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	if err = s.readHandoffObject(id, digest, &out, 4096); err == nil {
		t.Fatal("changed immutable receipt accepted")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "private.json")
	if err = os.WriteFile(target, []byte(`{"claim":"retained"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, path); err != nil {
		t.Skip("host cannot create symlink")
	}
	if err = s.readHandoffObject(id, digest, &out, 4096); err == nil {
		t.Fatal("foreign symlink report read")
	}
}

func TestTaskHandoffReadAudit(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.registerTaskHandoffs(mux)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "read-handoff-task-12345")
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error {
		t.Fatal("GET requested mutation authority")
		return nil
	}, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		t.Fatal("GET collected task output")
		return TaskHandoffCapture{}, nil
	})
	beforeState, beforeProject := auditTree(t, s.stateDir), auditTree(t, p.Path)
	for _, method := range []string{"GET", "HEAD"} {
		journeyRequest(t, mux, method, base+"/operations/"+run.Operations[0].ID+"/handoff", nil, 200)
		journeyRequest(t, mux, method, base+"/operations/op-missing/handoff", nil, 404)
	}
	if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, p.Path)) {
		t.Fatal("handoff read wrote product or source bytes")
	}
}

func TestTaskHandoffMissingReportDoesNotHideIndependentChecks(t *testing.T) {
	run := ProjectRun{ProjectID: "project-test", Checks: []ProjectRunCheckEvidence{{CheckID: "check-accepted", ProjectID: "project-test", Commit: strings.Repeat("a", 40)}, {CheckID: "check-other", ProjectID: "project-test", Commit: strings.Repeat("b", 40)}}}
	op := ProjectRunOperation{ID: "op-test", TaskID: "task-test", Integration: &ProjectSwarmIntegration{State: "integrated", Commit: strings.Repeat("a", 40)}}
	view, err := (&ProjectRunStore{}).taskHandoffView(run, op)
	if err != nil || view.Status != "not-recorded" || view.Receipt != nil || len(view.QuestionIDs) != 0 || len(view.RecordedChecks) != 1 || view.RecordedChecks[0].CheckID != "check-accepted" {
		t.Fatal("report availability changed independently recorded verification", view, err)
	}
	op.Integration = nil
	view, err = (&ProjectRunStore{}).taskHandoffView(run, op)
	if err != nil || len(view.RecordedChecks) != 0 {
		t.Fatal("unbound source acquired check authority", view, err)
	}
}

func TestTaskHandoffCompactIndexPreservesUnknownsAndVisibleOmission(t *testing.T) {
	prior := TaskHandoffView{TaskID: "prior-task", OperationID: "prior-operation", Status: "missing"}
	packet := &TaskDependencyPacket{PreviousAttempt: &prior, Dependencies: []TaskHandoffView{}}
	for i := 0; i < 100; i++ {
		packet.Dependencies = append(packet.Dependencies, TaskHandoffView{TaskID: "task-known", OperationID: "op-known", Status: "captured", QuestionIDs: []string{strings.Repeat("a", 64)}, Receipt: &TaskHandoffReceipt{Reported: &TaskHandoffReport{Summary: strings.Repeat("界", 1000)}}})
	}
	packet.Dependencies = append(packet.Dependencies, TaskHandoffView{TaskID: "task-unknown", Status: "not-recorded"})
	raw, err := taskHandoffNavigation(packet)
	if err != nil || len(raw) > 64<<10 {
		t.Fatal(len(raw), err)
	}
	var index struct {
		Dependencies []struct {
			Summary   string `json:"reportedSummary"`
			Truncated bool   `json:"reportedSummaryTruncated"`
			Known     bool   `json:"questionReportAvailable"`
		} `json:"dependencies"`
		PreviousAttempt struct {
			Status string `json:"reportStatus"`
			Known  bool   `json:"questionReportAvailable"`
		} `json:"previousAttempt"`
		Omitted         int    `json:"dependenciesOmitted"`
		KnownUnanswered int    `json:"knownUnansweredQuestions"`
		Unavailable     int    `json:"unavailableReportCount"`
		Complete        bool   `json:"questionReportsComplete"`
		Path            string `json:"completePacket"`
	}
	if err = json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	if index.Complete || index.Unavailable != 2 || index.KnownUnanswered != 100 || index.Omitted == 0 || len(index.Dependencies)+index.Omitted != len(packet.Dependencies) || index.PreviousAttempt.Status != "missing" || index.PreviousAttempt.Known || index.Path != ".luci/task-handoffs.json" {
		t.Fatal("compact index silently invented known context", string(raw))
	}
	if !index.Dependencies[0].Truncated || !index.Dependencies[0].Known || !handoffText(index.Dependencies[0].Summary, 1000) {
		t.Fatal("summary truncated without notice", index.Dependencies[0])
	}
}
