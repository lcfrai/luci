package api

import (
	"context"
	"encoding/json"
	"net/http"
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

// This fixture exercises real scanner IDs and native Git through product HTTP,
// while the existing session backend double makes admission observable without
// allocating customer or installed Docker resources.
func sessionsPrimaryBranchFixture(t *testing.T) (*sessionsEnv, project.Project, project.Worktree, product.BranchCatalogue) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("native Git required")
	}
	env := sessionsNewEnv(t)
	root := t.TempDir()
	pd, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: root, ClearBranchReady: func(ctx context.Context, worktreeID string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := env.st.SetWorktreeReady(worktreeID, false)
		return err
	}, BranchSessions: func(ctx context.Context, projectID, worktreeID string) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		rows, err := env.st.List()
		for _, row := range rows {
			if (row.ProjectID == projectID || row.WorktreeID == worktreeID) && row.Status != registry.StatusExited {
				return true, err
			}
		}
		return false, err
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pd.Close() })
	pd.Register(env.mux)
	env.api.registerCatalog(env.mux)
	created := env.do(t, "POST", "/v1/product/projects", `{"name":"primary-session-app"}`)
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var p project.Project
	if err := json.Unmarshal(created.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("real primary source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Luci Branch Fixture", "-c", "user.email=branch@example.com", "commit", "-m", "Initialized primary source"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := product.RunCommand(ctx, "git", args, p.Path)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	scanner := project.NewScanner(root, project.NewExecRunner()).WithProjectBoundary()
	worktrees, err := scanner.Worktrees(context.Background(), p.ID)
	if err != nil || len(worktrees) != 1 {
		t.Fatal(worktrees, err)
	}
	env.api.deps.Product, env.api.deps.Projects, env.api.deps.Worktrees = pd, scanner, scanner
	env.dir = p.Path
	return env, p, worktrees[0], sessionsBranchCatalogue(t, env, p)
}

func sessionsBranchCatalogue(t *testing.T, env *sessionsEnv, p project.Project) product.BranchCatalogue {
	t.Helper()
	w := env.do(t, "GET", "/v1/product/projects/"+p.ID+"/git/branches", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result product.BranchCatalogue
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func sessionsBranchRequest(t *testing.T, env *sessionsEnv, p project.Project, route string, body map[string]string, status int) product.BranchResult {
	t.Helper()
	raw, _ := json.Marshal(body)
	w := env.do(t, "POST", "/v1/product/projects/"+p.ID+"/git/"+route, string(raw))
	if w.Code != status {
		t.Fatal(w.Code, w.Body.String())
	}
	var result product.BranchResult
	if status == 200 {
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Operation.Status != "succeeded" {
			t.Fatal(result)
		}
	}
	return result
}

func TestPrimaryBranchSessionCreateAndRestartHoldSharedAdmission(t *testing.T) {
	env, p, wt, source := sessionsPrimaryBranchFixture(t)
	sessionsStubReady(t, nil)
	doneWatch := sessionsWatchDone(t)
	entered, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(unblock) })
	env.be.onCreate = func(spec backend.Spec) {
		close(entered)
		<-unblock
	}
	finished := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		env.mux.ServeHTTP(w, httptest.NewRequest("POST", sessionsTarget("/v1/sessions"), strings.NewReader(`{"worktreeId":"`+wt.ID+`","harness":"claude-code"}`)))
		finished <- w
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("create did not enter its admitted backend")
	}
	sessionsBranchRequest(t, env, p, "branches", map[string]string{"name": "cannot-race-start", "expectedBranchId": source.CurrentBranchID, "expectedCommit": source.Commit, "operationId": "create-admission-held"}, 409)
	if snapshot := sessionsBranchCatalogue(t, env, p); snapshot.CanCreate || snapshot.CanSwitch || !strings.Contains(snapshot.Message, "active sessions") {
		t.Fatal("starting session was not authoritative", snapshot)
	}
	once.Do(func() { close(unblock) })
	var created sessionJSON
	select {
	case w := <-finished:
		created = sessionsDecodeOne(t, w, 201)
	case <-time.After(10 * time.Second):
		t.Fatal("admitted create did not finish")
	}
	sessionsAwaitWatch(t, doneWatch)
	if created.ProjectID != p.ID || created.BranchID != source.CurrentBranchID || created.WorktreeID != wt.ID {
		t.Fatal("session recorded stale primary ownership", created)
	}
	env.be.mu.Lock()
	env.be.onCreate = nil
	env.be.mu.Unlock()
	enteredRestart, unblockRestart := make(chan struct{}), make(chan struct{})
	var restartOnce sync.Once
	defer restartOnce.Do(func() { close(unblockRestart) })
	env.be.onDestroy = func(string) {
		close(enteredRestart)
		<-unblockRestart
	}
	doneRestartWatch := sessionsWatchDone(t)
	go func() {
		w := httptest.NewRecorder()
		env.mux.ServeHTTP(w, httptest.NewRequest("POST", sessionsTarget("/v1/sessions/"+created.ID+"/restart"), nil))
		finished <- w
	}()
	select {
	case <-enteredRestart:
	case <-time.After(10 * time.Second):
		t.Fatal("restart did not reach its admitted backend")
	}
	sessionsBranchRequest(t, env, p, "branches", map[string]string{"name": "cannot-race-restart", "expectedBranchId": source.CurrentBranchID, "expectedCommit": source.Commit, "operationId": "restart-admission-held"}, 409)
	restartOnce.Do(func() { close(unblockRestart) })
	select {
	case w := <-finished:
		sessionsDecodeOne(t, w, 200)
	case <-time.After(10 * time.Second):
		t.Fatal("admitted restart did not finish")
	}
	sessionsAwaitWatch(t, doneRestartWatch)
}

func TestPrimaryBranchOldSessionCannotRestartOnAnotherCheckout(t *testing.T) {
	env, p, wt, source := sessionsPrimaryBranchFixture(t)
	readyRoute := "/v1/projects/" + p.ID + "/worktrees/" + wt.ID
	if w := env.do(t, "PATCH", readyRoute, `{"ready":true}`); w.Code != 200 {
		t.Fatal("could not set previous branch readiness", w.Code, w.Body.String())
	}
	feature := sessionsBranchRequest(t, env, p, "branches", map[string]string{"name": "feature-source", "expectedBranchId": source.CurrentBranchID, "expectedCommit": source.Commit, "operationId": "create-feature-session"}, 200)
	var listed struct {
		Worktrees []worktreeJSON `json:"worktrees"`
	}
	listedResponse := env.do(t, "GET", "/v1/projects/"+p.ID+"/worktrees", "")
	if listedResponse.Code != 200 || json.Unmarshal(listedResponse.Body.Bytes(), &listed) != nil || len(listed.Worktrees) != 1 || listed.Worktrees[0].Ready || env.st.WorktreeFlagsSnapshot()[wt.ID].Ready {
		t.Fatal("previous branch readiness reappeared on catalogue refresh", listedResponse.Code, listedResponse.Body.String())
	}
	sessionsStubReady(t, nil)
	doneWatch := sessionsWatchDone(t)
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", `{"worktreeId":"`+wt.ID+`","harness":"claude-code"}`), 201)
	sessionsAwaitWatch(t, doneWatch)
	if created.BranchID != feature.Branch.ID || created.WorktreeID != feature.Worktree.ID {
		t.Fatal("new primary session did not follow switched branch", created, feature)
	}
	_, err := env.st.Update(created.ID, func(row *registry.Session) error { row.Status = registry.StatusExited; return nil })
	if err != nil {
		t.Fatal(err)
	}
	current := sessionsBranchCatalogue(t, env, p)
	sessionsBranchRequest(t, env, p, "switch", map[string]string{"branchId": source.CurrentBranchID, "expectedBranchId": current.CurrentBranchID, "expectedCommit": current.Commit, "expectedTargetCommit": source.Commit, "operationId": "switch-before-restart"}, 200)
	beforeDestroys := env.be.destroyCount()
	w := env.do(t, http.MethodPost, "/v1/sessions/"+created.ID+"/restart", "")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "different branch") || env.be.destroyCount() != beforeDestroys {
		t.Fatal("wrong branch restart touched the backend", w.Code, w.Body.String())
	}
	_, err = env.st.Update(created.ID, func(row *registry.Session) error { row.BranchID = ""; return nil })
	if err != nil {
		t.Fatal(err)
	}
	w = env.do(t, http.MethodPost, "/v1/sessions/"+created.ID+"/restart", "")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "no saved branch") || env.be.destroyCount() != beforeDestroys {
		t.Fatal("unverified legacy restart touched the backend", w.Code, w.Body.String())
	}
}
