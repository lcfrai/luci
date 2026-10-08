package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type swarmFixtureTiming struct {
	Stage, Task string
	Elapsed     time.Duration
}

type swarmSessionFixture struct {
	timings   []swarmFixtureTiming
	env       *sessionsEnv
	options   product.Options
	storePath string
	store     *registry.FileStore
	owner     project.Project
	run       product.ProjectRun
	ops       []product.ProjectRunOperation
	requests  []sessionCreateRequest
}

func newSwarmSessionFixture(t *testing.T) *swarmSessionFixture {
	t.Helper()
	return newSwarmSessionFixtureTasks(t, "api", "ui")
}

func newSwarmSessionFixtureTasks(t *testing.T, tasksToReserve ...string) *swarmSessionFixture {
	t.Helper()
	f := &swarmSessionFixture{storePath: filepath.Join(t.TempDir(), "sessions.json")}
	var err error
	f.store, err = registry.NewFileStore(f.storePath)
	if err != nil {
		t.Fatal(err)
	}
	f.options = product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir(), BranchSessions: func(ctx context.Context, projectID, worktreeID string) (bool, error) {
		rows, err := f.store.List()
		for _, row := range rows {
			if (row.ProjectID == projectID || row.WorktreeID == worktreeID) && row.Status != registry.StatusExited {
				return true, err
			}
		}
		return false, err
	}}
	pd, err := product.New(f.options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pd.Close() })
	scanner := project.NewScanner(f.options.ProjectsDir, project.NewExecRunner()).WithProjectBoundary()
	f.env = sessionsNewEnvOverWith(t, t.TempDir(), f.store, f.store, func(d *Deps) {
		d.Product, d.Projects, d.Worktrees = pd, scanner, scanner
		d.ProjectsDir = f.options.ProjectsDir
	})
	f.env.mux = f.env.api.guarded
	w := f.env.do(t, "POST", "/v1/product/projects", `{"name":"swarm-session-app"}`)
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &f.owner) != nil {
		t.Fatal("project setup", w.Code, w.Body.String())
	}
	f.env.dir = f.owner.Path
	if err := os.WriteFile(filepath.Join(f.owner.Path, "README.md"), []byte("Controlled swarm mechanics fixture; no model output.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	selection := reviewedSessionPlan(t, f.env, f.owner.ID)
	tasks := []product.ProjectTask{
		{ID: "api", Title: "Implement isolated API", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"api-result.txt"}, Acceptance: "The API task records its own result."},
		{ID: "ui", Title: "Implement isolated interface", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"ui-result.txt"}, Acceptance: "The interface task records its own result."},
		{ID: "join", Title: "Verify combined behavior", Status: "todo", RequirementIDs: []string{"AC-save"}, DependsOn: []string{"api", "ui"}, WritePaths: []string{"joined-result.txt"}, Acceptance: "The integrated API and interface pass combined checks."},
	}
	body, _ := json.Marshal(map[string]any{"expectedRevision": selection.PlanRevision, "expectedSpecificationRevision": selection.SpecificationRevision, "tasks": tasks})
	w = f.env.do(t, "POST", "/v1/product/projects/"+f.owner.ID+"/plan", string(body))
	var plan product.ProjectPlan
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &plan) != nil {
		t.Fatal("task graph setup", w.Code, w.Body.String())
	}
	selection.PlanRevision, selection.TaskIDs = plan.Revision, []string{"api", "ui", "join"}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Swarm mechanics fixture", "-c", "user.email=swarm@example.com", "commit", "-m", "Reviewed swarm fixture inputs"}} {
		if _, err := product.RunCommand(context.Background(), "git", args, f.owner.Path); err != nil {
			t.Fatal(err)
		}
	}
	connection := sessionSaveConnection(t, f.env, 0, "", "codex", true)
	body, _ = json.Marshal(map[string]any{"idempotencyKey": "create-swarm-session-fixture", "worktreeId": project.WorktreeID(f.owner.Path), "harness": "codex", "permissionMode": "ask", "agentConnectionId": connection.ID, "outcome": "preview", "implementationContext": selection})
	started := time.Now()
	f.readRun(t, f.env.do(t, "POST", f.base(), string(body)), 201)
	f.timings = append(f.timings, swarmFixtureTiming{Stage: "create-run", Elapsed: time.Since(started)})
	body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "maxParallel": 2})
	started = time.Now()
	f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm", string(body)), 201)
	f.timings = append(f.timings, swarmFixtureTiming{Stage: "initialize-swarm", Elapsed: time.Since(started)})
	for _, task := range tasksToReserve {
		f.reserve(t, task)
	}

	if len(f.env.be.specs) != 0 {
		t.Fatal("planning or reservation dispatched provider work")
	}
	return f
}

func (f *swarmSessionFixture) reserve(t *testing.T, task string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "idempotencyKey": "reserve-swarm-session-" + task})
	started := time.Now()
	w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/"+task+"/reserve", string(body))
	f.timings = append(f.timings, swarmFixtureTiming{Stage: "reserve", Task: task, Elapsed: time.Since(started)})
	var result struct {
		Run       product.ProjectRun          `json:"run"`
		Operation product.ProjectRunOperation `json:"operation"`
	}
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Operation.Implementation == nil {
		t.Fatal("task reserve", w.Code, w.Body.String())
	}
	f.run = result.Run
	f.ops = append(f.ops, result.Operation)
	route := f.run.Input.Route
	connectionID := route.Connection.ID
	f.requests = append(f.requests, sessionCreateRequest{WorktreeID: result.Operation.WorktreeID, Harness: route.Harness, Model: route.Model, Effort: route.Effort, PermissionMode: route.PermissionMode, AgentConnectionID: &connectionID, ImplementationContext: product.ImplementationContextSelection(result.Operation.Implementation), ProjectRunID: f.run.ID, ProjectRunOperationID: result.Operation.ID})
}

func (f *swarmSessionFixture) base() string { return "/v1/product/projects/" + f.owner.ID + "/runs" }

func (f *swarmSessionFixture) readRun(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	var result struct {
		Run product.ProjectRun `json:"run"`
	}
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Run.ID == "" {
		t.Fatal("run response", w.Code, w.Body.String())
	}
	f.run = result.Run
}

func (f *swarmSessionFixture) reopen(t *testing.T) {
	t.Helper()
	f.env.api.deps.Product.Close()
	var err error
	f.store, err = registry.NewFileStore(f.storePath)
	if err != nil {
		t.Fatal(err)
	}
	pd, err := product.New(f.options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pd.Close() })
	scanner := project.NewScanner(f.options.ProjectsDir, project.NewExecRunner()).WithProjectBoundary()
	d := f.env.api.deps
	d.Product, d.Projects, d.Worktrees, d.Store, d.Flags = pd, scanner, scanner, f.store, f.store
	reopened, err := New(d)
	if err != nil {
		t.Fatal(err)
	}
	f.env.api, f.env.mux = reopened, reopened.guarded
}

// This subprocess is a deterministic harness witness, not a coding model or
// Docker isolation test. It reads the delivered task bundle, writes only its
// assigned fixture result and stays alive until the parent releases stdin.
func TestSwarmControlledWorkerProcess(t *testing.T) {
	if os.Getenv("LUCI_SWARM_TEST_PROCESS") != "1" {
		return
	}
	task := os.Getenv("LUCI_SWARM_TEST_TASK")
	data, err := os.ReadFile(filepath.Join(os.Getenv("LUCI_SWARM_TEST_CONTEXT"), "LUCI.md"))
	if err != nil {
		t.Fatal(err)
	}
	marker := bytes.Index(data, []byte("## Assigned task contract"))
	if marker < 0 || !bytes.Contains(data[marker:], []byte(`"id": "`+task+`"`)) {
		t.Fatal("assigned task is absent from delivered context")
	}
	if task != "api" && task != "ui" {
		t.Fatal("unexpected controlled task")
	}
	if err := os.WriteFile(task+"-result.txt", []byte("Controlled "+task+" fixture result\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fmt.Println("READY")
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

func TestProjectSwarmTwoReviewedSessionsRunIsolatedAndRecoverAfterRestart(t *testing.T) {
	f := newSwarmSessionFixture(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	type worker struct {
		cmd                     *exec.Cmd
		input                   io.WriteCloser
		contextPath, path, task string
		captured                []byte
	}
	var mu sync.Mutex
	workers := map[string]*worker{}
	f.env.be.onCreate = func(spec backend.Spec) {
		child, err := f.store.Get(spec.SessionID)
		if err != nil || child.ProjectID != f.owner.ID || child.ProjectRunID != f.run.ID || child.ImplementationContext == nil || len(child.ImplementationContext.TaskIDs) != 1 || child.ReviewedContext == nil {
			t.Error("backend allocation preceded its task-specific durable registry record", err)
			return
		}
		path, capturedPath := "", ""
		for _, mount := range spec.Mounts {
			if mount.ContainerPath == sessionWorkDir {
				path = mount.HostPath
			}
			if mount.ContainerPath == "/home/dev/.luci-reviewed-context" && mount.ReadOnly {
				capturedPath = mount.HostPath
			}
			if mount.HostPath == f.owner.Path || mount.HostPath == filepath.Join(f.owner.Path, ".git") {
				t.Error("worker received the primary source")
			}
		}
		if path == "" || capturedPath == "" || path == capturedPath {
			t.Error("missing distinct source/read-only context mounts")
			return
		}
		if info, err := os.Stat(filepath.Join(path, ".git")); err != nil || !info.IsDir() {
			t.Error("worker did not receive independent Git metadata")
			return
		}
		for _, suffix := range []string{"commondir", "objects/info/alternates"} {
			if _, err := os.Stat(filepath.Join(path, ".git", filepath.FromSlash(suffix))); !os.IsNotExist(err) {
				t.Error("worker shares Git storage", suffix)
				return
			}
		}
		exe, err := os.Executable()
		if err != nil {
			t.Error(err)
			return
		}
		task := child.ImplementationContext.TaskIDs[0]
		cmd := exec.Command(exe, "-test.run=^TestSwarmControlledWorkerProcess$")
		cmd.Dir, cmd.Env = path, append(os.Environ(), "LUCI_SWARM_TEST_PROCESS=1", "LUCI_SWARM_TEST_TASK="+task, "LUCI_SWARM_TEST_CONTEXT="+capturedPath)
		input, err := cmd.StdinPipe()
		if err != nil {
			t.Error(err)
			return
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			t.Error(err)
			return
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		captured, err := os.ReadFile(filepath.Join(capturedPath, "LUCI.md"))
		if err != nil {
			t.Error(err)
			return
		}
		if err := cmd.Start(); err != nil {
			t.Error(err)
			return
		}
		w := &worker{cmd: cmd, input: input, path: path, contextPath: capturedPath, task: task, captured: captured}
		mu.Lock()
		workers[spec.SessionID] = w
		mu.Unlock()
		ready := make(chan string, 1)
		go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
		select {
		case line := <-ready:
			if line != "READY\n" {
				t.Errorf("controlled worker did not start: %q", line)
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Error("controlled worker readiness timed out")
		}
	}
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, w := range workers {
			_ = w.input.Close()
			_ = w.cmd.Process.Kill()
			_ = w.cmd.Wait()
		}
	})
	responses := make(chan *httptest.ResponseRecorder, 2)
	for _, req := range f.requests {
		body, _ := json.Marshal(req)
		go func(body string) { responses <- f.env.do(t, "POST", "/v1/sessions", body) }(string(body))
	}
	seen := map[string]bool{}
	for range f.requests {
		w := <-responses
		child := sessionsDecodeOne(t, w, 201)
		seen[child.ID] = true
		sessionsAwaitWatch(t, done)
	}
	if len(seen) != 2 || len(workers) != 2 || len(f.env.be.specs) != 2 {
		t.Fatal("two tasks did not produce distinct overlapping controlled workers")
	}
	paths, captures := map[string]bool{}, map[string]bool{}
	for _, op := range f.ops {
		w := workers[op.ChildID]
		if w == nil || w.cmd.ProcessState != nil || paths[w.path] || captures[w.contextPath] {
			t.Fatal("workers share a checkout, context copy or completed process")
		}
		paths[w.path], captures[w.contextPath] = true, true
		other := "api"
		if w.task == "api" {
			other = "ui"
		}
		if _, err := os.Stat(filepath.Join(w.path, other+"-result.txt")); !os.IsNotExist(err) {
			t.Fatal("parallel worker edits leaked across checkouts")
		}
		if _, err := os.Stat(filepath.Join(f.owner.Path, w.task+"-result.txt")); !os.IsNotExist(err) {
			t.Fatal("worker edit changed the main checkout")
		}
		current, err := os.ReadFile(filepath.Join(w.contextPath, "LUCI.md"))
		if err != nil || !bytes.Equal(current, w.captured) {
			t.Fatal("worker edit changed delivered context")
		}
		stored, err := f.store.Get(op.ChildID)
		if err != nil || stored.Status != registry.StatusRunning || stored.ProjectRunOperationID != op.ID {
			t.Fatal("parallel registry ownership/liveness was lost", err)
		}
	}
	f.reopen(t)
	for i, req := range f.requests {
		body, _ := json.Marshal(req)
		child := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 200)
		if child.ID != f.ops[i].ChildID {
			t.Fatal("restart retry substituted a child")
		}
	}
	if len(f.env.be.specs) != 2 {
		t.Fatal("restart/retry dispatched provider work again")
	}
	f.readRun(t, f.env.do(t, "GET", f.base()+"/"+f.run.ID, ""), 200)
	body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "idempotencyKey": "reserve-blocked-dependent"})
	w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/join/reserve", string(body))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "dependencies") {
		t.Fatal("dependent task allocated before prerequisites integrated", w.Code, w.Body.String())
	}
	for _, worker := range workers {
		_ = worker.input.Close()
		if err := worker.cmd.Wait(); err != nil {
			t.Fatal("controlled task process did not finish successfully", err)
		}
	}
	t.Log("Two controlled worker processes remained concurrently alive across ledger and registry restart; task files and frozen context stayed isolated. This does not measure model throughput or Docker enforcement.")
}

func TestProjectSwarmInterruptedClaimRemainsUnconfirmedAfterRestart(t *testing.T) {
	f := newSwarmSessionFixture(t)
	op, req := f.ops[0], f.requests[0]
	route := f.run.Input.Route
	candidate := registry.Session{ProjectID: f.owner.ID, WorktreeID: op.WorktreeID, Harness: route.Harness, Model: route.Model, Effort: route.Effort, PermissionMode: route.PermissionMode, AgentConnectionID: route.Connection.ID, AgentConnectionRevision: route.Connection.Revision, AgentConnectionKind: route.Connection.Kind, AgentConnectionProtocol: route.Connection.Protocol, AuthMode: route.Connection.AuthMode, ImplementationContext: op.Implementation, ProjectRunID: f.run.ID, ProjectRunOperationID: op.ID}
	admission, err := f.env.api.deps.Product.BeginProjectRunSession(f.owner.ID, f.run.ID, op.ID, candidate, f.store.Get)
	if err != nil {
		t.Fatal(err)
	}
	admission.Release() // Simulate loss after durable claim, before registry/backend.
	f.reopen(t)
	body, _ := json.Marshal(req)
	w := f.env.do(t, "POST", "/v1/sessions", string(body))
	if w.Code != 409 || len(f.env.be.specs) != 0 || !strings.Contains(w.Body.String(), "unknown") {
		t.Fatal("uncertain claimed child was replayed after restart", w.Code, w.Body.String())
	}
	rows, err := f.store.List()
	if err != nil || len(rows) != 0 {
		t.Fatal("uncertain dispatch created a replacement registry child", err)
	}
}

func TestProjectSwarmCheckoutRefusesUnboundAndForeignSessionAllocation(t *testing.T) {
	f := newSwarmSessionFixture(t)
	for name, change := range map[string]func(*sessionCreateRequest){
		"manual terminal": func(r *sessionCreateRequest) {
			r.ProjectRunID, r.ProjectRunOperationID, r.Harness = "", "", "shell"
			r.ImplementationContext = nil
		},
		"manual coding": func(r *sessionCreateRequest) {
			r.ProjectRunID, r.ProjectRunOperationID = "", ""
			r.ImplementationContext = nil
		},
		"foreign task operation": func(r *sessionCreateRequest) { r.ProjectRunOperationID = f.ops[1].ID },
		"unbound fork":           func(r *sessionCreateRequest) { r.ProjectRunID, r.ProjectRunOperationID, r.Resume = "", "", "s-foreign" },
	} {
		t.Run(name, func(t *testing.T) {
			req := f.requests[0]
			change(&req)
			body, _ := json.Marshal(req)
			w := f.env.do(t, "POST", "/v1/sessions", string(body))
			if w.Code != 409 || !strings.Contains(w.Body.String(), "reserved run operation") {
				t.Fatal("unbound task checkout reached session setup", w.Code, w.Body.String())
			}
		})
	}
	if len(f.env.be.specs) != 0 {
		t.Fatal("refused task allocation started a process")
	}
	rows, err := f.store.List()
	if err != nil || len(rows) != 0 {
		t.Fatal("refused task allocation created registry state", err)
	}
	body, _ := json.Marshal(map[string]int{"expectedRevision": f.run.Revision})
	f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/cancel", string(body)), 200)
	runID, operationID, err := f.env.api.deps.Product.ProjectSwarmWorktreeOwner(context.Background(), f.owner.ID, f.ops[0].WorktreeID)
	if err != nil || runID != f.run.ID || operationID != f.ops[0].ID {
		t.Fatal("cancelling released private checkout ownership", err)
	}
	for _, req := range []sessionCreateRequest{f.requests[0], {WorktreeID: f.ops[0].WorktreeID, Harness: "shell"}} {
		body, _ := json.Marshal(req)
		w := f.env.do(t, "POST", "/v1/sessions", string(body))
		if w.Code != 404 || !strings.Contains(w.Body.String(), `"error":"worktree_not_found"`) {
			t.Fatal("cancelled task checkout did not remain hidden from new dispatch", w.Code, w.Body.String())
		}
	}
	if len(f.env.be.specs) != 0 {
		t.Fatal("cancelled task checkout started a process")
	}
}

func TestProjectSwarmPrepareSecondTaskWhileFirstIsRunning(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(f.requests[0])
	first := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	stored, err := f.store.Get(first.ID)
	if err != nil || stored.Status != registry.StatusRunning {
		t.Fatal("first task did not become active", err)
	}
	f.readRun(t, f.env.do(t, "GET", f.base()+"/"+f.run.ID, ""), 200)
	f.reserve(t, "ui")
	body, _ = json.Marshal(f.requests[1])
	second := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	if first.ID == second.ID || first.WorktreeID == second.WorktreeID || len(f.env.be.specs) != 2 {
		t.Fatal("parallel task admission did not preserve distinct children")
	}
	// A running isolated sibling also must not make a stopped worker look busy.
	_, err = f.store.Update(first.ID, func(s *registry.Session) error { s.Status = registry.StatusExited; return nil })
	if err != nil {
		t.Fatal(err)
	}
	w := f.env.do(t, "GET", f.base()+"/"+f.run.ID+"/swarm/tasks/api/review", "")
	if strings.Contains(w.Body.String(), "still active") {
		t.Fatal("isolated sibling incorrectly blocks this worker's review", w.Code, w.Body.String())
	}
}

func TestProjectSwarmControllerIntegrationWorktreeNeverBecomesSessionSource(t *testing.T) {
	f := newSwarmSessionFixture(t)
	privatePath, err := f.env.api.deps.Worktrees.Path(context.Background(), f.ops[0].WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	integration := privatePath + "-integration"
	if _, err := product.RunCommand(context.Background(), "git", []string{"worktree", "add", "--detach", integration, "HEAD"}, f.owner.Path); err != nil {
		t.Fatal(err)
	}
	controllerID := project.WorktreeID(integration)
	base := project.NewScanner(f.options.ProjectsDir, project.NewExecRunner()).WithProjectBoundary()
	if got, err := base.Path(context.Background(), controllerID); err != nil || got != integration {
		t.Fatal("fixture did not register a discoverable controller worktree", got, err)
	}
	rows, err := f.env.api.deps.Projects.Worktrees(context.Background(), f.owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == controllerID {
			t.Fatal("controller checkout leaked into project catalog")
		}
	}
	if _, err := f.env.api.deps.Worktrees.Path(context.Background(), controllerID); err == nil {
		t.Fatal("controller checkout resolved without ledger-owned task entry")
	}
	body, _ := json.Marshal(sessionCreateRequest{WorktreeID: controllerID, Harness: "shell"})
	w := f.env.do(t, "POST", "/v1/sessions", string(body))
	if w.Code < 400 || len(f.env.be.specs) != 0 {
		t.Fatal("controller checkout became a writable session source", w.Code, w.Body.String())
	}
}
