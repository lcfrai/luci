package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// These are actual router, registry, context and receipt operations with a
// controlled stopped backend. They do not establish native CLI/model behavior.
func TestTaskHandoffPurgeKeepsUncapturedOwnershipThenRetainsCapturedQuestions(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	a := f.env.api
	be := &handoffLifecycleBackend{Backend: a.deps.Backend, rows: map[string]backend.Sandbox{row.SandboxID: {ID: row.SandboxID, Ref: row.SandboxRef, State: backend.StateStopped}}}
	a.deps.Backend = be
	handoffCommit(t, f, f.env.be.spec(t, 0), "api-result.txt")
	reportDir, err := a.taskHandoffOutputPath(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	report := product.TaskHandoffReport{Version: 1, Summary: "Preserve the original task findings", ChangedFiles: []string{"api-result.txt"}, Decisions: []product.TaskHandoffDecision{}, ReportedChecks: []product.TaskHandoffReportedCheck{}, Questions: []product.TaskHandoffQuestion{{Question: "Which retention period?", Impact: "Data lifecycle", Blocking: true}}, Limitations: []string{"Controlled process state"}}
	raw, _ := json.Marshal(report)
	if err = os.WriteFile(filepath.Join(reportDir, "handoff.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	row, err = f.store.Update(row.ID, func(saved *registry.Session) error {
		saved.Status, saved.ExitReason = registry.StatusExited, "process exited"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	contextDir, err := a.reviewedContextPath(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	contextBefore := inspectFixtureFiles(t, contextDir)
	runBefore, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil || runBefore.Operations[0].Handoff != nil {
		t.Fatal("fixture already captured", err)
	}
	beforeDestroy := f.env.be.destroyCount()
	purgeURL := "/v1/sessions/" + row.ID + "?purge=true"
	w := f.env.do(t, "DELETE", purgeURL, "")
	if sessionsProblemCode(t, w, 409) != "task_handoff_required" || !bytes.Contains(w.Body.Bytes(), []byte("Check team status")) {
		t.Fatal("uncaptured task purge lacked recovery guidance", w.Code, w.Body.String())
	}
	after, err := f.store.Get(row.ID)
	if err != nil || !reflect.DeepEqual(row, after) || !reflect.DeepEqual(contextBefore, inspectFixtureFiles(t, contextDir)) || f.env.be.destroyCount() != beforeDestroy {
		t.Fatal("refused Forget changed ownership, context or backend", err)
	}
	runAfter, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil || !reflect.DeepEqual(runBefore, runAfter) {
		t.Fatal("Forget implicitly collected or changed run", err)
	}

	// Check team observes the stopped child and captures the fixed report using
	// its held output guard. Forget never tries to collect under its own lock.
	handoffReloadRun(t, f)
	body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision})
	f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/reconcile", string(body)), 200)
	route := f.base() + "/" + f.run.ID + "/swarm/operations/" + f.ops[0].ID + "/handoff"
	read := func() product.TaskHandoffView {
		t.Helper()
		var response struct {
			Handoff product.TaskHandoffView `json:"handoff"`
		}
		w := f.env.do(t, "GET", route, "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		return response.Handoff
	}
	captured := read()
	if captured.Status != "captured" || captured.Receipt == nil || captured.Receipt.Reported == nil || len(captured.QuestionIDs) != 1 || captured.Receipt.Reported.Questions[0].Question != report.Questions[0].Question {
		t.Fatal("question not durably captured", captured)
	}
	body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "handoffDigest": captured.Digest, "answer": "Thirty days, with a separate reviewed change if scope changes."})
	f.readRun(t, f.env.do(t, "POST", route+"/questions/"+captured.QuestionIDs[0]+"/answer", string(body)), 200)
	captured = read()
	if len(captured.Answers) != 1 {
		t.Fatal("answer missing")
	}

	// A dangling/corrupt canonical object cannot authorize deleting the last
	// ownership proof merely because a small reference remains in the run.
	receiptPath := filepath.Join(f.options.StateDir, "task-handoffs", f.owner.ID, captured.Digest+".json")
	receiptBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(receiptPath, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if w = f.env.do(t, "DELETE", purgeURL, ""); sessionsProblemCode(t, w, 409) != "task_handoff_required" {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = os.WriteFile(receiptPath, receiptBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if f.env.be.destroyCount() != beforeDestroy {
		t.Fatal("corrupt receipt refusal touched backend")
	}

	be.remove(row.SandboxID) // Exact controlled absence, separately from claims.
	if w = f.env.do(t, "DELETE", purgeURL, ""); w.Code != 204 {
		t.Fatal("captured task could not be forgotten", w.Code, w.Body.String())
	}
	if _, err = f.store.Get(row.ID); !errors.Is(err, registry.ErrNotFound) {
		t.Fatal("session was not purged", err)
	}
	if _, err = os.Lstat(contextDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("per-session context was not purged", err)
	}
	if retained := read(); !reflect.DeepEqual(retained, captured) {
		t.Fatal("Forget lost or rewrote canonical claims/questions/answers", retained)
	}
	if got, err := os.ReadFile(filepath.Join(reportDir, "handoff.json")); err != nil || !bytes.Equal(got, raw) {
		t.Fatal("Forget removed retained raw task output", err)
	}

	// An attributed report is not lifecycle proof. The source can still be
	// inspected, but no replacement checkout may be prepared from missing
	// ownership state without a separately finalized recovery receipt.
	review := apiCheckpointRead(t, f)
	allocations := controlledAllocationCount(f.env.be)
	w = f.env.do(t, "POST", apiCheckpointURL(f), apiCheckpointBody(review, "forgotten-task-is-not-launch-proof"))
	if w.Code != 409 || controlledAllocationCount(f.env.be) != allocations {
		t.Fatal("report was promoted into stopped-worker proof", w.Code, w.Body.String())
	}
}

func TestTaskHandoffPurgeAcceptsExplicitNonCapturedReceipts(t *testing.T) {
	for _, status := range []string{"missing", "invalid", "unavailable"} {
		t.Run(status, func(t *testing.T) {
			f, row, _ := settledTaskFixture(t)
			a := f.env.api
			a.deps.Backend = &handoffLifecycleBackend{Backend: a.deps.Backend, rows: map[string]backend.Sandbox{}}
			if status == "invalid" {
				dir, err := a.taskHandoffOutputPath(row.ID)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "handoff.json"), []byte("not a valid report"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.store.Update(row.ID, func(saved *registry.Session) error {
				saved.Status, saved.ExitReason = registry.StatusExited, "process exited"
				if status == "unavailable" {
					saved.ReviewedContext = nil
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			handoffReloadRun(t, f)
			body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision})
			f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/reconcile", string(body)), 200)
			view, err := a.deps.Product.TaskHandoffForOperation(f.owner.ID, f.run.ID, f.ops[0].ID)
			if err != nil || view.Status != status || view.Receipt == nil || view.Receipt.Reported != nil {
				t.Fatal("absence not explicit", view, err)
			}
			w := f.env.do(t, "DELETE", "/v1/sessions/"+row.ID+"?purge=true", "")
			if w.Code != 204 {
				t.Fatal(status, w.Code, w.Body.String())
			}
			after, err := a.deps.Product.TaskHandoffForOperation(f.owner.ID, f.run.ID, f.ops[0].ID)
			if err != nil || !reflect.DeepEqual(view, after) {
				t.Fatal("explicit absence receipt lost", err)
			}
		})
	}
}
