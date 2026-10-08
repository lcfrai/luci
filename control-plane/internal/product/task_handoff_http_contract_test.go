package product

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This fixture tests actual handler serialization, including independently stored
// controller-check metadata. The check metadata is controlled test data; this
// test does not claim to have run the named command or verified an application.
func TestTaskHandoffHTTPSharedContract(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.registerTaskHandoffs(mux)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "handoff-wire-1234567890")
	op := run.Operations[0]
	route := base + "/operations/" + op.ID + "/handoff"
	type identity struct {
		ID            string `json:"id"`
		TaskID        string `json:"taskId"`
		ChildID       string `json:"childId"`
		WorktreeID    string `json:"worktreeId"`
		RequestDigest string `json:"requestDigest"`
	}
	type sample struct {
		Operation identity        `json:"operation"`
		Response  json.RawMessage `json:"response"`
	}
	cases := map[string]sample{}
	read := func(name string) {
		body := journeyRequest(t, mux, "GET", route, nil, 200).Body.Bytes()
		var value struct {
			Handoff TaskHandoffView `json:"handoff"`
		}
		decodeEvidenceResponse(t, body, &value)
		if value.Handoff.ProjectID != p.ID || value.Handoff.RunID != run.ID || value.Handoff.OperationID != op.ID || value.Handoff.TaskID != op.TaskID {
			t.Fatal("handler changed the exact requested identity", value)
		}
		cases[name] = sample{identity{op.ID, op.TaskID, op.ChildID, op.WorktreeID, op.RequestDigest}, append(json.RawMessage(nil), body...)}
	}
	read("notRecorded")
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, "alpha.txt", "wire fixture output\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	capture := TaskHandoffCapture{Status: "missing", ReasonCode: "task_report_missing", ContextDigest: strings.Repeat("a", 64), Release: func() {}}
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) { return capture, nil })
	collect := func() {
		var response struct {
			Run ProjectRun `json:"run"`
		}
		decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", route+"/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &response)
		run = response.Run
	}
	collect()
	read("missing")
	report, err := DecodeTaskHandoffReport([]byte(`{"version":1,"summary":"Worker reports the requested component","changedFiles":["alpha.txt"],"decisions":[{"decision":"Use one component","rationale":"Avoid duplicate state"}],"reportedChecks":[{"command":"worker-check","outcome":"unknown","detail":"Worker could not establish a result"}],"questions":[{"question":"Which retention period?","impact":"Data lifecycle","blocking":true}],"limitations":["Worker claims require review"]}`))
	if err != nil {
		t.Fatal(err)
	}
	capture.Status, capture.ReasonCode, capture.Report = "captured", "", report
	collect()
	view, err := s.projectRuns.taskHandoffView(run, *swarmOperation(&run, op.ID))
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Run ProjectRun `json:"run"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", route+"/questions/"+view.QuestionIDs[0]+"/answer", map[string]any{"expectedRevision": run.Revision, "handoffDigest": view.Digest, "answer": "Keep 30 days; the reviewed assignment still applies."}, 200).Body.Bytes(), &response)
	run = response.Run
	evidence := ProjectRunCheckEvidence{CheckID: "check-wire-contract", ProjectID: p.ID, Commit: view.Receipt.SourceCommit, SourceDigest: view.Receipt.SourceDigest, Approvals: run.Input.Approvals, Requirements: run.Input.Requirements, Commands: []ProjectRunCommandEvidence{{CheckCommand: CheckCommand{Label: "Controlled recorded command", Executable: "fixture-check", Args: nil}, Status: "passed", ExitCode: 0}}, Execution: "host", CreatedAt: "2026-01-01T00:00:00Z", OriginalAvailable: true}
	run, err = s.projectRuns.JoinCheck(p.ID, p.Path, run.ID, run.Revision, evidence)
	if err != nil {
		t.Fatal(err)
	}
	read("captured")
	raw, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	var actual any
	if err = json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	replacements := map[string]string{p.ID: "project-fixture", run.ID: "run-fixture", op.ID: "op-fixture", op.ChildID: "s-12345678", op.WorktreeID: "worktree-fixture", run.Input.Approvals.SpecificationApprovalID: "approval-specification-fixture", run.Input.Approvals.DesignApprovalID: "approval-design-fixture"}
	actual = normalizeTaskHandoffWire(actual, replacements)
	rendered, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Optional external artifact capture never changes checked-in test data.
	if output := os.Getenv("LUCI_HANDOFF_HTTP_CAPTURE"); output != "" {
		if !filepath.IsAbs(output) {
			t.Fatal("capture path must be absolute")
		}
		if err = os.WriteFile(output, append(rendered, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	expectedBytes, err := os.ReadFile("testdata/task-handoff-http.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected any
	if err = json.Unmarshal(expectedBytes, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("actual handler contract differs from frontend shared fixture:\n%s", rendered)
	}
}

var handoffWireDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var handoffWireCommit = regexp.MustCompile(`^[a-f0-9]{40}$`)

func normalizeTaskHandoffWire(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeTaskHandoffWire(item, replacements)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = normalizeTaskHandoffWire(item, replacements)
		}
		return v
	case string:
		if replacement, ok := replacements[v]; ok {
			return replacement
		}
		if handoffWireDigest.MatchString(v) {
			return strings.Repeat("a", 64)
		}
		if handoffWireCommit.MatchString(v) {
			return strings.Repeat("b", 40)
		}
		if _, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return "2026-01-01T00:00:00Z"
		}
	}
	return value
}
