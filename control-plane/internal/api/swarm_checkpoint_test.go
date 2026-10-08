package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// These exercise the real HTTP/router, private execution ledger, lifecycle
// guard, Git workspaces and captured contexts. Process states are controlled;
// they are not a Docker, native Windows or model-quality witness.
func apiCheckpointURL(f *swarmSessionFixture) string {
	return f.base() + "/" + f.run.ID + "/swarm/operations/" + f.ops[0].ID + "/checkpoint"
}
func apiCheckpointRead(t *testing.T, f *swarmSessionFixture) product.ProjectSwarmCheckpointReview {
	t.Helper()
	w := f.env.do(t, "GET", apiCheckpointURL(f), "")
	var body struct {
		Review product.ProjectSwarmCheckpointReview `json:"review"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Review.ProjectID != f.owner.ID || body.Review.RunID != f.run.ID || body.Review.OperationID != f.ops[0].ID || body.Review.ChildID != f.ops[0].ChildID {
		t.Fatal("checkpoint review identity", w.Code, w.Body.String())
	}
	return body.Review
}
func apiCheckpointBody(review product.ProjectSwarmCheckpointReview, key string) string {
	data, _ := json.Marshal(map[string]any{"expectedRevision": review.ExpectedRevision, "idempotencyKey": key, "reviewDigest": review.ReviewDigest, "childId": review.ChildID})
	return string(data)
}
func stoppedCheckpointFixture(t *testing.T) (*swarmSessionFixture, *handoffLifecycleBackend, executionLeaseLedger, string) {
	t.Helper()
	f, row, ledger := settledTaskFixture(t)
	a := f.env.api
	be := &handoffLifecycleBackend{Backend: a.deps.Backend, rows: map[string]backend.Sandbox{row.SandboxID: {ID: row.SandboxID, Ref: row.SandboxRef, State: backend.StateStopped}}}
	a.deps.Backend = be
	path, err := a.deps.Worktrees.Path(context.Background(), row.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path, "api-result.txt"), []byte("retained API work\nfinish verification next\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reportRoot, err := a.taskHandoffOutputPath(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	report := product.TaskHandoffReport{Version: 1, Summary: "Retained implementation needs a verification decision", ChangedFiles: []string{"api-result.txt"}, Decisions: []product.TaskHandoffDecision{{Decision: "Keep the retained API implementation", Rationale: "Continue in a separate reviewed attempt"}}, ReportedChecks: []product.TaskHandoffReportedCheck{{Command: "worker-claimed-check", Outcome: "passed", Detail: "A worker claim, not recorded controller evidence"}}, Questions: []product.TaskHandoffQuestion{{Question: "Which API compatibility target?", Impact: "Remaining verification", Blocking: true}}, Limitations: []string{"Controlled worker; no live model quality claim"}}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(reportRoot, "handoff.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Update(row.ID, func(saved *registry.Session) error {
		saved.Status = registry.StatusExited
		saved.ExitReason = "controlled failed worker"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	handoffReloadRun(t, f)
	body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision})
	f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/reconcile", string(body)), 200)
	return f, be, ledger, path
}

func assertCheckpointStillUnaccepted(t *testing.T, run product.ProjectRun) {
	t.Helper()
	if run.Acceptance != nil || len(run.Checks) != 0 || len(run.Releases) != 0 {
		t.Fatal("checkpoint became accepted evidence", run)
	}
	for _, task := range run.Swarm.Tasks {
		if task.Integration != nil || task.ID == "join" && task.OperationID != "" {
			t.Fatal("checkpoint released dependencies/integration", task)
		}
	}
}

func TestSwarmCheckpointActualHTTPSettledLaunchPreparesThenExplicitlyStartsDistinctWorker(t *testing.T) {
	f, _, _, oldPath := stoppedCheckpointFixture(t)
	a := f.env.api
	handoffRoute := f.base() + "/" + f.run.ID + "/swarm/operations/" + f.ops[0].ID + "/handoff"
	var handoff struct {
		Handoff product.TaskHandoffView `json:"handoff"`
	}
	handoffRead := f.env.do(t, "GET", handoffRoute, "")
	if handoffRead.Code != 200 || json.Unmarshal(handoffRead.Body.Bytes(), &handoff) != nil || handoff.Handoff.Status != "captured" || handoff.Handoff.Receipt == nil || handoff.Handoff.Receipt.Reported == nil || len(handoff.Handoff.QuestionIDs) != 1 {
		t.Fatal("retained report not captured", handoffRead.Code, handoffRead.Body.String())
	}
	answer := func(text string) {
		t.Helper()
		handoffReloadRun(t, f)
		body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "handoffDigest": handoff.Handoff.Digest, "answer": text})
		f.readRun(t, f.env.do(t, "POST", handoffRoute+"/questions/"+handoff.Handoff.QuestionIDs[0]+"/answer", string(body)), 200)
	}
	answer("Verify the stable v1 contract; no extra execution permission is granted.")
	before, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	ledgerBefore, err := os.ReadFile(a.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	readmeBefore, err := os.ReadFile(filepath.Join(f.owner.Path, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	review := apiCheckpointRead(t, f)
	if !review.CanContinue || len(review.Files) != 1 || review.Files[0].After != "retained API work\nfinish verification next\n" {
		t.Fatal("actual stopped source not reviewable", review)
	}
	body := apiCheckpointBody(review, "http-reviewed-retained-checkpoint")
	w := f.env.do(t, "POST", apiCheckpointURL(f), body)
	var result struct {
		Run       product.ProjectRun          `json:"run"`
		Operation product.ProjectRunOperation `json:"operation"`
	}
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Operation.Allocation != "ready" || result.Operation.Status != "reserved" || result.Operation.Checkpoint == nil || result.Operation.Checkpoint.SeedCommit == "" {
		t.Fatal("actual checkpoint POST failed", w.Code, w.Body.String())
	}
	if controlledAllocationCount(f.env.be) != 1 || len(result.Run.Operations) != 2 {
		t.Fatal("preparing checkpoint launched a worker")
	}
	assertCheckpointStillUnaccepted(t, result.Run)
	private, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil || !reflect.DeepEqual(before.Operations[0], private.Operations[0]) {
		t.Fatal("new attempt rewrote old receipt", err)
	}
	ledgerAfter, err := os.ReadFile(a.execution.path)
	if err != nil || !bytes.Equal(ledgerBefore, ledgerAfter) {
		t.Fatal("preparation changed old resource lease", err)
	}
	readmeAfter, err := os.ReadFile(filepath.Join(f.owner.Path, "README.md"))
	if err != nil || !bytes.Equal(readmeBefore, readmeAfter) {
		t.Fatal("checkpoint changed primary", err)
	}
	newPath, err := a.deps.Worktrees.Path(context.Background(), result.Operation.WorktreeID)
	if err != nil || newPath == oldPath {
		t.Fatal("new workspace not independent", err)
	}
	for _, dir := range []string{oldPath, newPath} {
		if data, err := os.ReadFile(filepath.Join(dir, "api-result.txt")); err != nil || string(data) != review.Files[0].After {
			t.Fatal("retained bytes changed", err)
		}
	}
	f.run = result.Run
	f.reopen(t)
	a = f.env.api
	duplicate := f.env.do(t, "POST", apiCheckpointURL(f), body)
	var reopened struct {
		Operation product.ProjectRunOperation `json:"operation"`
	}
	if duplicate.Code != 200 || json.Unmarshal(duplicate.Body.Bytes(), &reopened) != nil || reopened.Operation.ID != result.Operation.ID || controlledAllocationCount(f.env.be) != 1 {
		t.Fatal("reopen/lost response replayed worker or allocation", duplicate.Code, duplicate.Body.String())
	}
	// A later answer changes future reservations, not the previously reviewed
	// checkpoint's frozen inputs, even when its worker has not started yet.
	answer("A later editor changes the target to v2; do not rewrite the prepared attempt.")
	// Starting the new attempt is a separate authenticated request after the
	// prepared checkpoint survived service/registry reload.
	request := f.requests[0]
	request.WorktreeID = result.Operation.WorktreeID
	request.ProjectRunOperationID = result.Operation.ID
	request.ImplementationContext = product.ImplementationContextSelection(result.Operation.Implementation)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	requestJSON, _ := json.Marshal(request)
	created := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(requestJSON)), 201)
	sessionsAwaitWatch(t, done)
	if created.ID != result.Operation.ChildID || created.ID == f.ops[0].ChildID || controlledAllocationCount(f.env.be) != 2 {
		t.Fatal("explicit start reused old child", created)
	}
	row, err := f.store.Get(created.ID)
	if err != nil || row.ImplementationContext.SourceCommit != result.Operation.Checkpoint.SeedCommit {
		t.Fatal("new worker lost exact checkpoint source", err)
	}
	captured, err := a.readReviewedContext(row.ID, row.ImplementationContext, row.ReviewedContext)
	if err != nil {
		t.Fatal(err)
	}
	var packet product.TaskDependencyPacket
	for _, file := range captured.Files {
		if file.Path == ".luci/task-handoffs.json" {
			if err = json.Unmarshal(file.Data, &packet); err != nil {
				t.Fatal(err)
			}
		}
	}
	if packet.PreviousAttempt == nil || packet.PreviousAttempt.OperationID != f.ops[0].ID || packet.PreviousAttempt.TaskID != "api" || len(packet.Dependencies) != 0 || len(packet.PreviousAttempt.RecordedChecks) != 0 {
		t.Fatal("previous attempt context lost or promoted into check evidence", packet)
	}
	previous := packet.PreviousAttempt
	if previous.Status != "captured" || previous.Receipt == nil || previous.Receipt.Reported == nil || previous.Digest != handoff.Handoff.Digest || previous.Integration != nil || len(previous.Answers) != 1 || previous.Answers[0].Answer != "Verify the stable v1 contract; no extra execution permission is granted." || previous.QuestionIDs[0] != handoff.Handoff.QuestionIDs[0] {
		t.Fatal("checkpoint report/question/answer was lost, promoted or changed after preparation", previous)
	}
	reported := previous.Receipt.Reported
	if reported.Summary != handoff.Handoff.Receipt.Reported.Summary || len(reported.ReportedChecks) != 1 || reported.ReportedChecks[0].Outcome != "passed" || len(reported.Decisions) != 1 || len(reported.Limitations) != 1 || len(reported.Questions) != 1 {
		t.Fatal("previous worker claims or limitations missing", reported)
	}
	handoffReloadRun(t, f)
	assertCheckpointStillUnaccepted(t, f.run)
}

func TestSwarmCheckpointActualHTTPFinalizedFenceRequiresCurrentExactMarker(t *testing.T) {
	for _, markerValid := range []bool{true, false} {
		t.Run(map[bool]string{true: "valid-finalized-fence", false: "fence-no-longer-verifies"}[markerValid], func(t *testing.T) {
			f, probe, _ := fencedTaskFixture(t)
			a := f.env.api
			path, err := a.deps.Worktrees.Path(context.Background(), f.ops[0].WorktreeID)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(path, "api-result.txt"), []byte("retained uncertain-launch output\n"), 0600); err != nil {
				t.Fatal(err)
			}
			fencedRecover(t, f, 200)
			probe.Backend = &taskCleanupLifecycleBackend{Backend: f.env.be, absent: true}
			review := apiCheckpointRead(t, f)
			if !review.CanContinue {
				t.Fatal(review)
			}
			before, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil {
				t.Fatal(err)
			}
			ledgerBefore, err := os.ReadFile(a.execution.path)
			if err != nil {
				t.Fatal(err)
			}
			probe.mu.Lock()
			verifies := probe.verifyCalls
			if !markerValid {
				probe.verifyErr = backend.ErrLaunchFenceOwnership
			}
			probe.mu.Unlock()
			w := f.env.do(t, "POST", apiCheckpointURL(f), apiCheckpointBody(review, "checkpoint-after-exact-finalized-fence"))
			probe.mu.Lock()
			afterVerifies := probe.verifyCalls
			probe.mu.Unlock()
			if afterVerifies <= verifies {
				t.Fatal("checkpoint trusted old fence receipt without live verification")
			}
			current, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !markerValid {
				if w.Code < 400 || !reflect.DeepEqual(before, current) {
					t.Fatal("invalid current fence allowed a new workspace", w.Code, w.Body.String())
				}
				return
			}
			if w.Code != 201 || len(current.Operations) != 2 || current.Operations[1].Allocation != "ready" || !reflect.DeepEqual(before.Operations[0], current.Operations[0]) {
				t.Fatal("finalized launch could not continue safely", w.Code, w.Body.String())
			}
			assertCheckpointStillUnaccepted(t, current)
			ledgerAfter, err := os.ReadFile(a.execution.path)
			if err != nil || !bytes.Equal(ledgerBefore, ledgerAfter) || controlledAllocationCount(f.env.be) != 1 {
				t.Fatal("continuation removed fence/changed old lease or launched worker", err)
			}
		})
	}
}

func TestSwarmCheckpointActualHTTPRechecksUnsettledAndRevokedAuthorityBeforeCopy(t *testing.T) {
	for _, scenario := range []string{"pending-backend", "editor-revoked", "execution-revoked"} {
		t.Run(scenario, func(t *testing.T) {
			f, _, ledger, _ := stoppedCheckpointFixture(t)
			a := f.env.api
			review := apiCheckpointRead(t, f)
			if !review.CanContinue {
				t.Fatal(review)
			}
			switch scenario {
			case "pending-backend":
				ledger.Leases[0].State = "backend-dispatching"
				raw, _ := json.Marshal(ledger)
				if err := writeExecutionLedger(a.execution.path, raw); err != nil {
					t.Fatal(err)
				}
			case "editor-revoked":
				a.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
					return errors.New("current editor access revoked")
				}
			case "execution-revoked":
				a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
					return management.ExecutionAuthority{}, errors.New("current execution grant revoked")
				}
			}
			before, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil {
				t.Fatal(err)
			}
			ledgerBefore, err := os.ReadFile(a.execution.path)
			if err != nil {
				t.Fatal(err)
			}
			w := f.env.do(t, "POST", apiCheckpointURL(f), apiCheckpointBody(review, "refuse-new-copy-after-current-authority-change"))
			if w.Code < 400 {
				t.Fatal("stale consent permitted checkpoint", w.Code, w.Body.String())
			}
			current, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil || !reflect.DeepEqual(before, current) {
				t.Fatal("refusal replaced old operation", err)
			}
			ledgerAfter, err := os.ReadFile(a.execution.path)
			if err != nil || !bytes.Equal(ledgerBefore, ledgerAfter) || controlledAllocationCount(f.env.be) != 1 {
				t.Fatal("refusal changed execution ledger or launched worker", err)
			}
			if strings.Contains(w.Body.String(), "seedCommit") {
				t.Fatal("refused response advertised checkpoint")
			}
		})
	}
}
