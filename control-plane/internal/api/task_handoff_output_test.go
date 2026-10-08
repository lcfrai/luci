package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type handoffLifecycleBackend struct {
	backend.Backend
	mu   sync.Mutex
	rows map[string]backend.Sandbox
}

func (b *handoffLifecycleBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	row, err := b.Backend.Create(ctx, spec)
	if err == nil {
		b.mu.Lock()
		b.rows[row.ID] = row
		b.mu.Unlock()
	}
	return row, err
}
func (b *handoffLifecycleBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[id]
	if !ok {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	return row, nil
}
func (b *handoffLifecycleBackend) stop(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	row := b.rows[id]
	row.State = backend.StateStopped
	b.rows[id] = row
}
func (b *handoffLifecycleBackend) remove(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.rows, id)
}

func handoffReloadRun(t *testing.T, f *swarmSessionFixture) {
	t.Helper()
	var err error
	f.run, err = f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil {
		t.Fatal(err)
	}
}
func handoffCommit(t *testing.T, f *swarmSessionFixture, spec backend.Spec, name string) {
	t.Helper()
	dir := ""
	for _, mount := range spec.Mounts {
		if mount.ContainerPath == sessionWorkDir {
			dir = mount.HostPath
		}
	}
	rel, err := filepath.Rel(f.options.ProjectsDir, dir)
	if err != nil || dir == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatal("refuse fixture source outside owned project storage")
	}
	if info, err := os.Stat(filepath.Join(dir, ".git")); err != nil || !info.IsDir() {
		t.Fatal("fixture needs exact standalone Git checkout", err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), []byte("Controlled handoff result\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "--", name}, {"-c", "user.name=Handoff fixture", "-c", "user.email=handoff@example.test", "commit", "-m", "Record controlled task result"}} {
		if _, err = product.RunCommand(context.Background(), "git", args, dir); err != nil {
			t.Fatal(err)
		}
	}
}

// Actual HTTP task reservations/dispatches, owned Git checkouts, output mounts,
// capture, answers, integration and next-worker context. Backend process states
// are controlled mechanics; this test does not claim native CLI/model behavior.
func TestTaskHandoffActualAPIDependencyContextKeepsClaimsAndEvidenceSeparate(t *testing.T) {
	f := newSwarmSessionFixture(t)
	a := f.env.api
	a.deps.ReadyTimeout = 5 * time.Second
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	be := &handoffLifecycleBackend{Backend: a.deps.Backend, rows: map[string]backend.Sandbox{}}
	a.deps.Backend = be
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	firstDigest, firstQuestion := "", ""
	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(f.requests[i])
		created := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
		sessionsAwaitWatch(t, done)
		spec := f.env.be.spec(t, i)
		reportRoot := ""
		for _, mount := range spec.Mounts {
			if mount.ContainerPath == taskHandoffContainerDirectory {
				if mount.ReadOnly {
					t.Fatal("output mount is readonly")
				}
				reportRoot = mount.HostPath
			}
		}
		if reportRoot == "" {
			t.Fatal("actual task has no owned report mount")
		}
		handoffCommit(t, f, spec, f.ops[i].TaskID+"-result.txt")
		report := product.TaskHandoffReport{Version: 1, Summary: "Controlled " + f.ops[i].TaskID + " result", ChangedFiles: []string{f.ops[i].TaskID + "-result.txt"}, Decisions: []product.TaskHandoffDecision{{Decision: "Keep explicit dependency inputs", Rationale: "Reliable handoffs"}}, ReportedChecks: []product.TaskHandoffReportedCheck{{Command: "claimed-check", Outcome: "passed", Detail: "No controller check was run"}}, Questions: []product.TaskHandoffQuestion{}, Limitations: []string{"Controlled backend; no model claim"}}
		if i == 0 {
			report.Questions = append(report.Questions, product.TaskHandoffQuestion{Question: "Which retention period?", Impact: "Data lifecycle", Blocking: true})
		}
		raw, _ := json.Marshal(report)
		if err := os.WriteFile(filepath.Join(reportRoot, "handoff.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		be.stop(created.ID)
		if _, err := f.store.Update(created.ID, func(row *registry.Session) error {
			row.Status = registry.StatusExited
			row.ExitReason = "process exited"
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		handoffReloadRun(t, f)
		body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision})
		f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/reconcile", string(body)), 200)
		route := f.base() + "/" + f.run.ID + "/swarm/operations/" + f.ops[i].ID + "/handoff"
		w := f.env.do(t, "GET", route, "")
		var response struct {
			Run     product.ProjectRun      `json:"run"`
			Handoff product.TaskHandoffView `json:"handoff"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Handoff.Status != "captured" || response.Handoff.Receipt == nil {
			t.Fatal("actual report not captured", w.Code, w.Body.String())
		}
		if len(response.Handoff.RecordedChecks) != 0 || response.Handoff.Receipt.Reported.ReportedChecks[0].Outcome != "passed" {
			t.Fatal("worker claim acquired controller authority", response.Handoff)
		}
		if i == 0 {
			firstDigest = response.Handoff.Digest
			firstQuestion = response.Handoff.QuestionIDs[0]
			body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "handoffDigest": firstDigest, "answer": "Thirty days; this answer grants no extra execution permission."})
			w = f.env.do(t, "POST", route+"/questions/"+firstQuestion+"/answer", string(body))
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
				t.Fatal(w.Code, w.Body.String())
			}
			f.run = response.Run
			if bytes.Contains(w.Body.Bytes(), []byte(`"actors"`)) {
				t.Fatal("private answer actor receipt leaked")
			}
		}
		var reviewed struct {
			Review product.ProjectSwarmReview `json:"review"`
		}
		w = f.env.do(t, "GET", f.base()+"/"+f.run.ID+"/swarm/tasks/"+f.ops[i].TaskID+"/review", "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &reviewed) != nil || !reviewed.Review.CanIntegrate {
			t.Fatal("captured task not reviewable", w.Code, w.Body.String())
		}
		body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "reviewDigest": reviewed.Review.ReviewDigest})
		f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/"+f.ops[i].TaskID+"/integrate", string(body)), 200)
		be.remove(created.ID)
	}
	f.reserve(t, "join")
	body, _ := json.Marshal(f.requests[2])
	created := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	row, err := f.store.Get(created.ID)
	if err != nil || row.ReviewedContext == nil || row.ReviewedContext.Version != 2 {
		t.Fatal(row, err)
	}
	captured, err := a.readReviewedContext(row.ID, row.ImplementationContext, row.ReviewedContext)
	if err != nil {
		t.Fatal(err)
	}
	var packet product.TaskDependencyPacket
	var original []byte
	for _, file := range captured.Files {
		if file.Path == ".luci/task-handoffs.json" {
			original = append([]byte{}, file.Data...)
			if err = json.Unmarshal(file.Data, &packet); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(packet.Dependencies) != 2 || packet.TaskID != "join" || packet.OperationID != f.ops[2].ID || packet.ProjectID != f.owner.ID {
		t.Fatal("wrong frozen dependencies", packet)
	}
	for _, dep := range packet.Dependencies {
		if dep.Integration == nil || dep.Integration.State != "integrated" || len(dep.RecordedChecks) != 0 || dep.Receipt == nil || dep.Receipt.ContextDigest == "" {
			t.Fatal("missing provenance or false verification", dep)
		}
	}
	if packet.Dependencies[0].Digest != firstDigest || len(packet.Dependencies[0].Answers) != 1 || packet.Dependencies[0].QuestionIDs[0] != firstQuestion {
		t.Fatal("task decision/question handoff lost", packet.Dependencies[0])
	}
	handoffReloadRun(t, f)
	body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "handoffDigest": firstDigest, "answer": "Later answer affects future context only."})
	w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/operations/"+f.ops[0].ID+"/handoff/questions/"+firstQuestion+"/answer", string(body))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	again, err := a.readReviewedContext(row.ID, row.ImplementationContext, row.ReviewedContext)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range again.Files {
		if file.Path == ".luci/task-handoffs.json" && !bytes.Equal(file.Data, original) {
			t.Fatal("later answer mutated already-dispatched task context")
		}
	}
}

func TestTaskOutputGuardRefusesLiveAndUnsettledAndRevokedAuthority(t *testing.T) {
	f, row, ledger := settledTaskFixture(t)
	a := f.env.api
	req := product.ProjectSwarmRecoveryRequest{ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, ChildID: row.ID}
	if release, err := a.acquireStoppedTaskOutput(context.Background(), req); err == nil {
		release()
		t.Fatal("running task output acquired")
	}
	be := &taskCleanupLifecycleBackend{Backend: a.deps.Backend, sandbox: backend.Sandbox{ID: row.SandboxID, Ref: row.SandboxRef, State: backend.StateStopped}}
	a.deps.Backend = be
	if _, err := f.store.Update(row.ID, func(value *registry.Session) error { value.Status = registry.StatusExited; return nil }); err != nil {
		t.Fatal(err)
	}
	release, err := a.acquireStoppedTaskOutput(context.Background(), req)
	if err != nil {
		t.Fatal("exact acknowledged stopped task refused", err)
	}
	release()
	ledger.Leases[0].State = "backend-dispatching"
	raw, _ := json.Marshal(ledger)
	if err = writeExecutionLedger(a.execution.path, raw); err != nil {
		t.Fatal(err)
	}
	be.absent = true
	if release, err := a.acquireStoppedTaskOutput(context.Background(), req); err == nil {
		release()
		t.Fatal("absence bypassed unsettled launch")
	}
	ledger.Leases[0].State = "backend-settled"
	raw, _ = json.Marshal(ledger)
	if err = writeExecutionLedger(a.execution.path, raw); err != nil {
		t.Fatal(err)
	}
	a.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
		return errors.New("current editor access revoked")
	}
	if release, err := a.acquireStoppedTaskOutput(context.Background(), req); err == nil {
		release()
		t.Fatal("revoked editor captured output")
	}
}

func TestTaskHandoffOutputCollectorRequiresBoundSafeFile(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	a := f.env.api
	be := &taskCleanupLifecycleBackend{Backend: a.deps.Backend, sandbox: backend.Sandbox{ID: row.SandboxID, Ref: row.SandboxRef, State: backend.StateStopped}}
	a.deps.Backend = be
	if _, err := f.store.Update(row.ID, func(value *registry.Session) error { value.Status = registry.StatusExited; return nil }); err != nil {
		t.Fatal(err)
	}
	op := f.ops[0]
	req := product.TaskHandoffCaptureRequest{ProjectSwarmRecoveryRequest: product.ProjectSwarmRecoveryRequest{ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: op.ID, ChildID: row.ID}, TaskID: op.TaskID, WorktreeID: op.WorktreeID, RequestDigest: op.RequestDigest, ImplementationDigest: taskOutputImplementationDigest(op.Implementation)}
	dir, err := a.taskHandoffOutputPath(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "handoff.json")
	collect := func(status, reason string) {
		t.Helper()
		captured, err := a.collectTaskHandoff(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if captured.Release == nil {
			t.Fatal("collector lost guard")
		}
		defer captured.Release()
		if captured.Status != status || captured.ReasonCode != reason {
			t.Fatal(captured)
		}
		if status == "captured" && (captured.Report == nil || captured.Report.ChangedFiles == nil || captured.Report.Questions == nil || captured.ContextDigest != row.ReviewedContext.Digest) {
			t.Fatal("report lost bounded schema or context", captured)
		}
	}
	collect("missing", "task_report_missing")
	valid := []byte(`{"version":1,"summary":"Retained claim","changedFiles":[],"decisions":[],"reportedChecks":[],"questions":[],"limitations":[]}`)
	if err = os.WriteFile(path, valid, 0600); err != nil {
		t.Fatal(err)
	}
	collect("captured", "")
	original := req.TaskID
	req.TaskID = "foreign-task"
	if captured, err := a.collectTaskHandoff(context.Background(), req); err == nil {
		captured.Release()
		t.Fatal("foreign task read report")
	}
	req.TaskID = original
	for _, raw := range [][]byte{[]byte(`{"version":1,"summary":"x","summary":"override"}`), []byte(strings.Repeat("x", product.TaskHandoffReportLimit+1))} {
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		reason := "task_report_schema_invalid"
		if len(raw) > product.TaskHandoffReportLimit {
			reason = "task_report_unsafe_or_oversized"
		}
		collect("invalid", reason)
	}
	foreign := filepath.Join(t.TempDir(), "foreign-report.json")
	if err = os.WriteFile(foreign, valid, 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if kind == "symlink" {
				err = os.Symlink(foreign, path)
			} else {
				err = os.Link(foreign, path)
			}
			if err != nil {
				t.Skip("host cannot make test link", err)
			}
			collect("invalid", "task_report_unsafe_or_oversized")
		})
	}
}
