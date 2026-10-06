package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/platform"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func branchFixture(t *testing.T) (*Server, *http.ServeMux, project.Project, BranchCatalogue) {
	t.Helper()
	s, mux := journeyFixture(t)
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	p := createJourneyProject(t, mux, "branch-app")
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("main source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"brief", map[string]any{"revision": 0, "purpose": "Main purpose", "audience": "fixture users", "constraints": "", "criteria": []string{"actual source"}}, 200)
	journeyRequest(t, mux, "POST", branchBase(p)+"git/commit", map[string]string{"message": "Initial main context"}, 200)
	return s, mux, p, branchCatalogue(t, mux, p)
}

func branchBase(p project.Project) string { return "/v1/product/projects/" + p.ID + "/" }

func branchCatalogue(t *testing.T, handler http.Handler, p project.Project) BranchCatalogue {
	t.Helper()
	w := journeyRequest(t, handler, "GET", branchBase(p)+"git/branches", nil, 200)
	var result BranchCatalogue
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func branchCreateRequest(snapshot BranchCatalogue, name, id string) branchMutation {
	return branchMutation{Name: name, ExpectedBranchID: snapshot.CurrentBranchID, ExpectedCommit: snapshot.Commit, OperationID: id}
}

func decodeBranchResult(t *testing.T, w *httptest.ResponseRecorder) BranchResult {
	t.Helper()
	var result BranchResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func branchNative(ctx context.Context, exe string, args []string, dir string) (string, error) {
	return runCommandEnvironment(ctx, exe, args, dir, project.ManagedGitEnvironment(os.Environ()))
}

func branchSwitchRequest(snapshot BranchCatalogue, name, id string) branchMutation {
	for _, b := range snapshot.Branches {
		if b.Name == name {
			return branchMutation{BranchID: b.ID, ExpectedBranchID: snapshot.CurrentBranchID, ExpectedCommit: snapshot.Commit, ExpectedTargetCommit: b.Commit, OperationID: id}
		}
	}
	panic("fixture target branch missing")
}

func TestPrimaryBranchCreateSwitchConnectsFilesGitContextAndDeploymentSource(t *testing.T) {
	s, mux, p, original := branchFixture(t)
	if !original.Clean || !original.CanCreate || original.CurrentBranch != "main" {
		t.Fatal(original)
	}
	initialGit, err := os.ReadFile(filepath.Join(p.Path, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	s.registerApplicationConfiguration(mux)
	journeyRequest(t, mux, "POST", branchBase(p)+"configuration", map[string]any{"expectedRevision": 0, "updates": []map[string]string{{"name": "API_SECRET", "value": "retained-private-value"}}}, 200)
	privateRevision, err := s.applicationConfigurationSnapshot(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(original, "feat/real-branch", "create-real-branch"), 200))
	if result.Operation.Status != "succeeded" || result.Commit != original.Commit || result.Branch == nil || result.Worktree == nil || !result.Worktree.IsPrimary {
		t.Fatal(result)
	}
	if result.Branch.ID != project.BranchID(p.ID, "feat/real-branch") || result.Worktree.BranchID != result.Branch.ID {
		t.Fatal(result)
	}
	if got := branchCatalogue(t, mux, p); got.CurrentBranch != "feat/real-branch" || !got.Clean || got.Operation == nil || got.Operation.Status != "succeeded" {
		t.Fatal(got)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("feature source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	brief := journeyRequest(t, mux, "GET", branchBase(p)+"brief", nil, 200)
	var current Brief
	if err := json.Unmarshal(brief.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"brief", map[string]any{"revision": current.Revision, "purpose": "Feature purpose", "audience": "fixture users", "constraints": "", "criteria": []string{"actual source"}}, 200)
	reviewDeploymentFixture(t, mux, p.ID)
	journeyRequest(t, mux, "POST", branchBase(p)+"git/commit", map[string]string{"message": "Actual feature implementation"}, 200)
	feature := branchCatalogue(t, mux, p)
	if feature.Commit == original.Commit {
		t.Fatal("feature commit was not created")
	}
	if err := s.ValidateDeploymentSource(context.Background(), p.ID, feature.Commit); err != nil {
		t.Fatal("new branch deployment source disconnected", err)
	}
	journeyRequest(t, mux, "GET", branchBase(p)+"files?path=README.md", nil, 200)
	main := decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/switch", branchSwitchRequest(feature, "main", "switch-main-real"), 200))
	if main.Operation.Status != "succeeded" || main.Commit != original.Commit {
		t.Fatal(main)
	}
	text, err := os.ReadFile(filepath.Join(p.Path, "README.md"))
	if err != nil || string(text) != "main source\n" {
		t.Fatal("primary Files did not follow switch", string(text), err)
	}
	brief = journeyRequest(t, mux, "GET", branchBase(p)+"brief", nil, 200)
	if json.Unmarshal(brief.Body.Bytes(), &current) != nil || current.Purpose != "Main purpose" {
		t.Fatal("versioned journey did not follow switch", brief.Body.String())
	}
	if err := s.ValidateDeploymentSource(context.Background(), p.ID, feature.Commit); err == nil {
		t.Fatal("wrong branch deployment commit accepted")
	}
	if err := s.ValidateDeploymentSource(context.Background(), p.ID, original.Commit); err == nil || !strings.Contains(strings.ToLower(err.Error()), "approve") {
		t.Fatal("returning to unreviewed legacy main bypassed explicit Define/Design review", err)
	}
	configuration, err := s.applicationConfigurationSnapshot(p.ID)
	if err != nil || configuration.Revision != privateRevision.Revision || configuration.Values["API_SECRET"] != "retained-private-value" {
		t.Fatal("branch switch changed runtime secrets", err)
	}
	scanner := project.NewScanner(s.projectRoot(), project.NewExecRunner()).WithProjectBoundary()
	worktrees, err := scanner.Worktrees(context.Background(), p.ID)
	if err != nil || len(worktrees) != 1 || worktrees[0].BranchID != original.CurrentBranchID || worktrees[0].ID != main.Worktree.ID {
		t.Fatal("rail did not see native checkout", worktrees, err)
	}
	resolved, err := scanner.Path(context.Background(), main.Worktree.ID)
	resolvedInfo, statErr := os.Stat(resolved)
	primaryInfo, primaryErr := os.Stat(p.Path)
	if err != nil || statErr != nil || primaryErr != nil || !os.SameFile(resolvedInfo, primaryInfo) {
		t.Fatal("primary session path changed", resolved, err)
	}
	if err := s.ValidateSessionBranch(context.Background(), p.ID, project.WorktreeID(p.Path), feature.CurrentBranchID); err == nil {
		t.Fatal("old branch session could restart on main")
	}
	if err := s.ValidateSessionBranch(context.Background(), p.ID, project.WorktreeID(p.Path), original.CurrentBranchID); err != nil {
		t.Fatal(err)
	}
	configAfter, _ := os.ReadFile(filepath.Join(p.Path, ".git", "config"))
	if string(configAfter) != string(initialGit) {
		t.Fatal("branch controls rewrote repository configuration")
	}
}

func TestPrimaryBranchPreconditionsAndInvalidRequestsDoNotChangeGit(t *testing.T) {
	s, mux, p, snapshot := branchFixture(t)
	before := auditTree(t, p.Path)
	for _, name := range []string{"-unsafe", "../outside", "feat/../outside", "bad..name", "bad@{name", "bad.lock", "bad/", "bad\nname"} {
		journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, name, "invalid-branch-name"), 400)
	}
	stale := branchCreateRequest(snapshot, "valid-name", "stale-source-commit")
	stale.ExpectedCommit = strings.Repeat("a", 40)
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", stale, 409)
	stale = branchCreateRequest(snapshot, "valid-name", "stale-source-branch")
	stale.ExpectedBranchID = project.BranchID(p.ID, "missing")
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", stale, 409)
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "main", "duplicate-main-name"), 409)
	if fmt.Sprint(before) != fmt.Sprint(auditTree(t, p.Path)) {
		t.Fatal("preflight changed source or Git metadata")
	}
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("unsaved work"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "new-name", "dirty-source-gate"), 409)
	if got := branchCatalogue(t, mux, p); got.Clean || got.CanCreate {
		t.Fatal(got)
	}
	newProject := createJourneyProject(t, mux, "unborn-branch")
	unborn := branchCatalogue(t, mux, newProject)
	if unborn.Commit != "" || unborn.CanCreate || unborn.CurrentBranch != "main" {
		t.Fatal(unborn)
	}
	unborn.Commit = strings.Repeat("a", 40)
	journeyRequest(t, mux, "POST", branchBase(newProject)+"git/branches", branchCreateRequest(unborn, "first-branch", "unborn-source-gate"), 409)
	s.branchSessions = nil
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("main source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := branchCatalogue(t, mux, p); got.CanCreate {
		t.Fatal("unchecked session state allowed branch change")
	}
}

func TestPrimaryBranchSessionStateAndSharedAdmissionPreventCheckoutRaces(t *testing.T) {
	s, mux, p, snapshot := branchFixture(t)
	other := createJourneyProject(t, mux, "available-during-checkout")
	s.registerApplicationConfiguration(mux)
	active := true
	primary, err := s.branchPrimaryWorktree(context.Background(), s.projectRoot(), p)
	if err != nil {
		t.Fatal(err)
	}
	s.branchSessions = func(_ context.Context, projectID, worktreeID string) (bool, error) {
		if projectID != p.ID || worktreeID != primary.ID {
			t.Fatal("session check escaped project")
		}
		return active, nil
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "blocked", "active-session-block"), 409)
	active = false
	releaseAdmission, err := s.AcquireGitSessionAdmission(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "blocked", "pending-admission-block"), 409)
	releaseAdmission()
	releaseAdmission() // release is safe for abandoned requests too.
	started, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(unblock) })
	s.branchRunner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		for _, arg := range args {
			if arg == "switch" {
				close(started)
				select {
				case <-unblock:
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
		}
		return branchNative(ctx, exe, args, dir)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		data, _ := json.Marshal(branchCreateRequest(snapshot, "held-branch", "checkout-admission-block"))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", branchBase(p)+"git/branches", strings.NewReader(string(data))))
		done <- w
	}()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("native checkout did not start")
	}
	if release, err := s.AcquireGitSessionAdmission(context.Background(), p.ID); err == nil {
		release()
		t.Fatal("session admitted while checkout was in flight")
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "other", "concurrent-checkout-block"), 409)
	journeyRequest(t, mux, "GET", branchBase(p)+"git/branches", nil, 409)
	journeyRequest(t, mux, "GET", branchBase(p)+"files?path=README.md", nil, 409)
	journeyRequest(t, mux, "POST", branchBase(p)+"brief", map[string]any{}, 409)
	journeyRequest(t, mux, "GET", branchBase(other)+"files", nil, 200)
	journeyRequest(t, mux, "GET", branchBase(p)+"configuration", nil, 200)
	operation := decodeBranchResult(t, journeyRequest(t, mux, "GET", branchBase(p)+"git/branch-operations/checkout-admission-block", nil, 200))
	if operation.Operation.Status != "running" {
		t.Fatal("a running checkout was reconciled while still holding source admission", operation)
	}
	if _, err := s.FunctionSources().Save(context.Background(), p.ID, "blocked-function", platform.DataFunctionSave{}); err == nil {
		t.Fatal("Functions wrote source during checkout")
	}
	if _, err := os.Stat(filepath.Join(p.Path, "supabase")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("contended Functions source allocated a project directory", err)
	}
	if err := s.ValidateDeploymentSource(context.Background(), p.ID, snapshot.Commit); err == nil {
		t.Fatal("deployment validation inspected changing source")
	}
	once.Do(func() { close(unblock) })
	select {
	case w := <-done:
		if w.Code != 200 || decodeBranchResult(t, w).Operation.Status != "succeeded" {
			t.Fatal(w.Code, w.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("checkout did not finish")
	}
}

func TestPrimaryBranchCannotSwitchARefCheckedOutInAnotherNativeWorktree(t *testing.T) {
	s, mux, p, snapshot := branchFixture(t)
	decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "linked-owner", "create-linked-owner"), 200))
	feature := branchCatalogue(t, mux, p)
	decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/switch", branchSwitchRequest(feature, "main", "return-main-linked"), 200))
	linkedPath := filepath.Join(s.projectRoot(), ".owned-worktrees", "linked-owner")
	if _, err := s.journeyGit(context.Background(), p.Path, "worktree", "add", linkedPath, "linked-owner"); err != nil {
		t.Fatal(err)
	}
	catalogue := branchCatalogue(t, mux, p)
	var found bool
	for _, row := range catalogue.Branches {
		if row.Name == "main" && (!row.CheckedOut || row.CanSwitch || !row.IsHead) {
			t.Fatal("current primary branch advertises another switch", row)
		}
		if row.Name == "linked-owner" {
			found = true
			if !row.CheckedOut || row.CanSwitch || row.IsHead || !strings.Contains(row.Message, "another worktree") {
				t.Fatal("linked checkout ownership was lost", row)
			}
		}
	}
	if !found {
		t.Fatal("native linked branch missing from catalogue")
	}
	beforeSource, beforeLinked := auditTree(t, p.Path), auditTree(t, linkedPath)
	journeyRequest(t, mux, "POST", branchBase(p)+"git/switch", branchSwitchRequest(catalogue, "linked-owner", "refuse-linked-owner"), 409)
	if fmt.Sprint(beforeSource) != fmt.Sprint(auditTree(t, p.Path)) || fmt.Sprint(beforeLinked) != fmt.Sprint(auditTree(t, linkedPath)) {
		t.Fatal("a refused switch changed either checkout")
	}
}

func TestPrimaryBranchDisablesControllerHooksFiltersAndInheritedGitRedirection(t *testing.T) {
	s, mux, p, original := branchFixture(t)
	marker := filepath.Join(t.TempDir(), "must-not-run")
	script := "#!/bin/sh\nprintf unsafe > '" + filepath.ToSlash(marker) + "'\n"
	if err := os.MkdirAll(filepath.Join(p.Path, ".git", "hooks"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".git", "hooks", "post-checkout"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".gitattributes"), []byte("README.md filter=controllertrap\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"filter.controllertrap.clean", "filter.controllertrap.smudge", "filter.controllertrap.process", "core.fsmonitor"} {
		if _, err := s.journeyGit(context.Background(), p.Path, "config", key, "sh -c \"printf unsafe > '"+filepath.ToSlash(marker)+"'\""); err != nil {
			t.Fatal(err)
		}
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/commit", map[string]string{"message": "Controlled filtered fixture"}, 200)
	original = branchCatalogue(t, mux, p)
	foreign := createJourneyProject(t, mux, "foreign-git")
	t.Setenv("GIT_DIR", filepath.Join(foreign.Path, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign.Path)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", "sh -c \"printf inherited > '"+filepath.ToSlash(marker)+"'\"")
	result := decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(original, "safe-branch", "safe-controller-flags"), 200))
	if result.Operation.Status != "succeeded" {
		t.Fatal(result)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("repository code executed on controller", err)
	}
	if got := branchCatalogue(t, mux, p); got.CurrentBranch != "safe-branch" || got.Commit != original.Commit {
		t.Fatal("inherited Git location redirected checkout", got)
	}
}

func TestPrimaryBranchRejectsGitConfigAndMetadataEscapesWithoutLeakingErrors(t *testing.T) {
	for _, attack := range []string{"include", "commondir", "alternates", "hardlink"} {
		t.Run(attack, func(t *testing.T) {
			s, mux, p, snapshot := branchFixture(t)
			private := filepath.Join(t.TempDir(), "private-do-not-disclose")
			if err := os.WriteFile(private, []byte("private-token-must-not-disclose"), 0600); err != nil {
				t.Fatal(err)
			}
			switch attack {
			case "include":
				f, err := os.OpenFile(filepath.Join(p.Path, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.WriteString("\n[includeIf \"gitdir:**\"]\n path = " + filepath.ToSlash(private) + "\n")
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			case "commondir":
				if err := os.WriteFile(filepath.Join(p.Path, ".git", "commondir"), []byte(filepath.ToSlash(filepath.Dir(private))), 0600); err != nil {
					t.Fatal(err)
				}
			case "alternates":
				if err := os.WriteFile(filepath.Join(p.Path, ".git", "objects", "info", "alternates"), []byte(filepath.ToSlash(filepath.Dir(private))), 0600); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(private, filepath.Join(p.Path, ".git", "packed-refs")); err != nil {
					t.Skip("hard links are unavailable", err)
				}
			}
			w := journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "denied", "unsafe-config-block"), 403)
			if strings.Contains(w.Body.String(), private) || strings.Contains(w.Body.String(), "private-token") {
				t.Fatal("private Git detail leaked", w.Body.String())
			}
			journeyRequest(t, mux, "GET", branchBase(p)+"git/branches", nil, 403)
			_ = s
		})
	}
}

func TestPrimaryBranchReceiptsReconcileInterruptedSuccessReadOnlyAndNeverRetry(t *testing.T) {
	s, mux, p, snapshot := branchFixture(t)
	configPath := filepath.Join(p.Path, ".git", "config")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var switchCount atomic.Int32
	s.branchRunner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		out, err := branchNative(ctx, exe, args, dir)
		for _, arg := range args {
			if arg == "switch" && err == nil {
				switchCount.Add(1)
				if err := os.WriteFile(configPath, append(append([]byte{}, config...), []byte("\n[include]\n path = hidden-config\n")...), 0600); err != nil {
					return "", err
				}
				return "", errors.New("private-token-must-not-disclose")
			}
		}
		return out, err
	}
	req := branchCreateRequest(snapshot, "interrupted", "interrupted-checkout")
	result := decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", req, 200))
	if result.Operation.Status != "uncertain" || result.Branch != nil {
		t.Fatal("unverified branch reported ready", result)
	}
	if strings.Contains(result.Operation.Message, "private-token") {
		t.Fatal("Git error leaked")
	}
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		t.Fatal(err)
	}
	s.branchRunner = branchNative
	beforeSource, beforeState := auditTree(t, p.Path), auditTree(t, s.stateDir)
	op := branchBase(p) + "git/branch-operations/" + req.OperationID
	verified := decodeBranchResult(t, journeyRequest(t, mux, "GET", op, nil, 200))
	if verified.Operation.Status != "succeeded" || verified.Branch == nil || verified.Commit != snapshot.Commit {
		t.Fatal(verified)
	}
	if fmt.Sprint(beforeSource) != fmt.Sprint(auditTree(t, p.Path)) || fmt.Sprint(beforeState) != fmt.Sprint(auditTree(t, s.stateDir)) {
		t.Fatal("read-only reconciliation changed source or receipts")
	}
	decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", req, 200))
	if switchCount.Load() != 1 {
		t.Fatal("receipt retry re-executed checkout")
	}
	conflict := req
	conflict.Name = "different"
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", conflict, 409)
}

func TestPrimaryBranchForeignBranchOperationAndIgnoredFileStayProtected(t *testing.T) {
	s, mux, p, snapshot := branchFixture(t)
	foreign := createJourneyProject(t, mux, "other-owned-project")
	if err := os.WriteFile(filepath.Join(foreign.Path, "README.md"), []byte("other project"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", branchBase(foreign)+"git/commit", map[string]string{"message": "Other source"}, 200)
	foreignSnapshot := branchCatalogue(t, mux, foreign)
	foreignBefore := auditTree(t, foreign.Path)
	journeyRequest(t, mux, "POST", branchBase(p)+"git/switch", branchMutation{BranchID: foreignSnapshot.CurrentBranchID, ExpectedBranchID: snapshot.CurrentBranchID, ExpectedCommit: snapshot.Commit, ExpectedTargetCommit: foreignSnapshot.Commit, OperationID: "foreign-branch-refusal"}, 404)
	if fmt.Sprint(foreignBefore) != fmt.Sprint(auditTree(t, foreign.Path)) {
		t.Fatal("another project was changed")
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".gitignore"), []byte("ignored.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/commit", map[string]string{"message": "Ignore private local file"}, 200)
	snapshot = branchCatalogue(t, mux, p)
	decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "file-owner", "create-file-owner"), 200))
	if err := os.WriteFile(filepath.Join(p.Path, "ignored.txt"), []byte("branch version"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "add", "--force", "ignored.txt"); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/commit", map[string]string{"message": "Track branch file"}, 200)
	feature := branchCatalogue(t, mux, p)
	decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/switch", branchSwitchRequest(feature, "main", "back-main-ignored"), 200))
	if err := os.WriteFile(filepath.Join(p.Path, "ignored.txt"), []byte("private local content"), 0600); err != nil {
		t.Fatal(err)
	}
	main := branchCatalogue(t, mux, p)
	refused := decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/switch", branchSwitchRequest(main, "file-owner", "no-overwrite-ignored"), 200))
	if refused.Operation.Status != "failed" {
		t.Fatal("unchanged refused checkout was not recorded as failed", refused)
	}
	text, err := os.ReadFile(filepath.Join(p.Path, "ignored.txt"))
	if err != nil || string(text) != "private local content" {
		t.Fatal("ignored local contents lost", string(text), err)
	}
	journeyRequest(t, mux, "GET", branchBase(foreign)+"git/branch-operations/create-file-owner", nil, 404)
}

func TestPrimaryBranchActiveChecksAndDeliveryKeepTheirSource(t *testing.T) {
	s, mux, p, snapshot := branchFixture(t)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.mu.Lock()
	s.projectChecks = map[string]*projectCheckJob{"held-source-check": {run: ProjectCheckRun{ID: "held-source-check", ProjectID: p.ID, Status: "running"}, cancel: cancel}}
	s.mu.Unlock()
	if catalogue := branchCatalogue(t, mux, p); catalogue.CanSwitch || catalogue.CanCreate || !strings.Contains(catalogue.Message, "checks or delivery") {
		t.Fatal("an active check released its primary source", catalogue)
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "denied-check", "check-source-admission"), 409)
	s.mu.Lock()
	delete(s.projectChecks, "held-source-check")
	s.mu.Unlock()
	if !s.reserveDelivery(p.ID) {
		t.Fatal("fixture delivery was already active")
	}
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(snapshot, "denied-delivery", "delivery-source-admission"), 409)
	s.releaseDelivery(p.ID)
	if after := branchCatalogue(t, mux, p); after.Commit != snapshot.Commit || after.CurrentBranchID != snapshot.CurrentBranchID || !after.CanCreate {
		t.Fatal("released source admission remained unavailable", after)
	}
}

func TestPrimaryBranchReadinessStorageFailureNeverStartsCheckout(t *testing.T) {
	s, mux, p, snapshot := branchFixture(t)
	var readyChecks, checkouts int
	s.clearBranchReady = func(ctx context.Context, id string) error {
		primary, err := s.branchPrimaryWorktree(ctx, s.projectRoot(), p)
		if err != nil || primary.ID != id {
			t.Fatal("readiness clear escaped the native primary checkout", id, err)
		}
		readyChecks++
		return errors.New("private-readiness-location-must-not-disclose")
	}
	s.branchRunner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		for _, arg := range args {
			if arg == "switch" {
				checkouts++
			}
		}
		return branchNative(ctx, exe, args, dir)
	}
	before := auditTree(t, p.Path)
	req := branchCreateRequest(snapshot, "readiness-refused", "readiness-storage-failure")
	failed := decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", req, 200))
	if failed.Operation.Status != "failed" || failed.Branch != nil || readyChecks != 1 || checkouts != 0 || strings.Contains(failed.Operation.Message, "private-readiness") {
		t.Fatal("readiness failure was not confined before Git", failed, readyChecks, checkouts)
	}
	journeyRequest(t, mux, "GET", branchBase(p)+"git/branch-operations/"+req.OperationID, nil, 200)
	journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", req, 200)
	if readyChecks != 1 || checkouts != 0 || fmt.Sprint(before) != fmt.Sprint(auditTree(t, p.Path)) {
		t.Fatal("a failed readiness receipt retried its mutation")
	}
}
