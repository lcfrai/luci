package product

import (
	"context"
	"encoding/base64"
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

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Only explicit review routes in fixtures receive the same validated worker
// actor context as production. Raw handler tests remain unauthenticated.
type fixtureReviewRegistrar struct{ Registrar }

func fixtureReviewHandler(next http.Handler) http.Handler {
	token := strings.Repeat("f", 43)
	user, _ := json.Marshal(management.User{ID: "fixture-reviewer", Username: "reviewer", Role: "super-admin"})
	policy, _ := json.Marshal(management.Policy{AllowHostCommands: true, AllowPublicPublishing: true, MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, MonthlyBudgetUSD: 30, AllowedHarnesses: []string{"claude-code", "codex", "opencode", "shell"}, BackingMode: "docker", InstanceName: "Fixture"})
	worker := management.WorkerHandler(token, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clone := r.Clone(r.Context())
		clone.Header.Set("Authorization", "Bearer "+token)
		clone.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(user))
		clone.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
		worker.ServeHTTP(w, clone)
	})
}

func (mux fixtureReviewRegistrar) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if strings.HasPrefix(pattern, "POST ") && strings.HasSuffix(pattern, "/approve") {
		mux.Registrar.HandleFunc(pattern, fixtureReviewHandler(http.HandlerFunc(handler)).ServeHTTP)
	} else {
		mux.Registrar.HandleFunc(pattern, handler)
	}
}

func journeyFixture(t *testing.T) (*Server, *http.ServeMux) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is required for the local project journey")
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "no-global-config"))
	t.Setenv("LUCI_GIT_NAME", "Luci Test Person")
	t.Setenv("LUCI_GIT_EMAIL", "person@example.com")
	s, err := New(Options{StateDir: t.TempDir(), ProjectsDir: ownedFixtureTempDir(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	mux := http.NewServeMux()
	s.registerProjects(fixtureReviewRegistrar{mux})
	return s, mux
}

func journeyRequest(t *testing.T, handler http.Handler, method, path string, body any, expected int) *httptest.ResponseRecorder {
	t.Helper()
	var text string
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		text = string(data)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(text))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != expected {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, w.Code, expected, w.Body.String())
	}
	return w
}

func createJourneyProject(t *testing.T, handler http.Handler, name string) project.Project {
	t.Helper()
	w := journeyRequest(t, handler, "POST", "/v1/product/projects", map[string]string{"name": name}, 201)
	var p project.Project
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != project.ProjectID(p.Path) {
		t.Fatalf("project ID %q disagrees with catalogue for %q", p.ID, p.Path)
	}
	return p
}

func TestSlowProjectCloneKeepsSetupResponsiveAndReservesItsDirectory(t *testing.T) {
	s, mux := journeyFixture(t)
	mux.HandleFunc("GET /v1/setup/status", s.handleStatus)
	started, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	s.runner = func(ctx context.Context, _ string, _ []string, dir string) (string, error) {
		if _, err := RunCommand(ctx, "git", []string{"init", "--initial-branch=main", "."}, dir); err != nil {
			return "", err
		}
		close(started)
		select {
		case <-release:
			return "", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/v1/product/projects", strings.NewReader(`{"name":"slow-clone","remoteUrl":"https://example.com/project.git"}`)))
		done <- w
	}()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("clone did not start")
	}
	statusDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/v1/setup/status", nil))
		statusDone <- w
	}()
	select {
	case w := <-statusDone:
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("setup status waited for the clone's global lock")
	}
	assertNoImportedProject(t, s, "slow-clone")
	journeyRequest(t, mux, "POST", "/v1/product/projects", map[string]string{"name": "slow-clone"}, 409)
	journeyRequest(t, mux, "POST", "/v1/product/projects", map[string]string{"name": "slow-clone", "remoteUrl": "https://example.com/other.git", "importAttemptId": importTestID}, 409)
	unblock.Do(func() { close(release) })
	select {
	case w := <-done:
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("clone did not finish")
	}
}

func TestProjectJourneyPersistsRevisionConflictsAndSafeDesignExport(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "first-app")
	base := "/v1/product/projects/" + p.ID
	w := journeyRequest(t, mux, "GET", base+"/brief", nil, 200)
	var brief Brief
	_ = json.Unmarshal(w.Body.Bytes(), &brief)
	if brief.Revision != 0 || brief.ProjectID != p.ID || brief.Criteria == nil {
		t.Fatalf("empty brief: %#v", brief)
	}
	if _, err := os.Stat(filepath.Join(p.Path, ".luci")); !os.IsNotExist(err) {
		t.Fatalf("reading an empty brief created state: %v", err)
	}
	body := map[string]any{"revision": 0, "purpose": "A useful small app", "audience": "A person", "constraints": "Local first", "criteria": []string{"Works offline"}}
	journeyRequest(t, mux, "POST", base+"/brief", body, 200)
	journeyRequest(t, mux, "POST", base+"/brief", body, 409)
	text, err := os.ReadFile(filepath.Join(p.Path, "LUCI.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "Revision: 1") || !strings.Contains(string(text), "untrusted project context") {
		t.Fatalf("missing canonical context/provenance: %s", text)
	}
	frames := []Frame{{ID: "home", Name: "Home <script>", Width: 400, Height: 300, Background: "#ffffff", Elements: []Element{{ID: "title", Kind: "text", X: 10, Y: 10, Width: 300, Height: 100, Text: "<script>alert('x')</script>", Color: "#111111"}}}}
	journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 0, "frames": frames}, 200)
	body["revision"] = 1
	body["purpose"] = "Second revision"
	journeyRequest(t, mux, "POST", base+"/brief", body, 200)
	exported := journeyRequest(t, mux, "GET", base+"/design/export", nil, 200)
	if strings.Contains(exported.Body.String(), "<script>") || !strings.Contains(exported.Body.String(), "&lt;script&gt;") || !strings.Contains(exported.Body.String(), "Brief revision 1") || strings.Contains(exported.Body.String(), "ZgotmplZ") {
		t.Fatalf("unsafe or unbound export: %s", exported.Body.String())
	}
	if !strings.Contains(exported.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("export lacks sandbox CSP")
	}
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.BriefRevisions) != 2 || len(state.DesignRevisions) != 1 || state.BriefRevisions[0].Purpose != "A useful small app" {
		t.Fatalf("revision history lost: %#v", state)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	restartedMux := http.NewServeMux()
	restarted.registerProjects(restartedMux)
	w = journeyRequest(t, restartedMux, "GET", base+"/brief", nil, 200)
	_ = json.Unmarshal(w.Body.Bytes(), &brief)
	if brief.Revision != 2 || brief.Purpose != "Second revision" {
		t.Fatalf("restart lost brief: %#v", brief)
	}
	scene := json.RawMessage(`{"elements":[],"appState":{"viewBackgroundColor":"#ffffff"},"files":{}}`)
	w = journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 1, "frames": []Frame{}, "scene": scene}, 200)
	var design Design
	_ = json.Unmarshal(w.Body.Bytes(), &design)
	if design.BriefRevision != 2 || len(design.Scene) == 0 {
		t.Fatalf("drawing provenance missing: %#v", design)
	}
	journeyRequest(t, mux, "GET", base+"/design/export", nil, 422)
	frames[0].Background = "url(https://example.com/tracker)"
	journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 2, "frames": frames}, 400)
}

func TestProjectFilesContainTraversalSecretsAndLinks(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "file-app")
	base := "/v1/product/projects/" + p.ID
	if err := os.WriteFile(filepath.Join(p.Path, "hello.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".env"), []byte("SECRET=private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "binary"), []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	listing := journeyRequest(t, mux, "GET", base+"/files", nil, 200)
	if strings.Contains(listing.Body.String(), ".env") || strings.Contains(listing.Body.String(), ".git") {
		t.Fatalf("private entries leaked: %s", listing.Body.String())
	}
	journeyRequest(t, mux, "GET", base+"/files?path=hello.txt", nil, 200)
	for _, path := range []string{"../hello.txt", ".env", ".git/config", "C:%5Csecret.txt", "..%5Csecret.txt"} {
		journeyRequest(t, mux, "GET", base+"/files?path="+path, nil, 403)
	}
	journeyRequest(t, mux, "GET", base+"/files?path=binary", nil, 415)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	_ = os.WriteFile(outside, []byte("outside secret"), 0600)
	if err := os.Symlink(outside, filepath.Join(p.Path, "escape.txt")); err == nil {
		journeyRequest(t, mux, "GET", base+"/files?path=escape.txt", nil, 403)
		if err := os.Symlink(outside, filepath.Join(p.Path, "LUCI.md")); err != nil {
			t.Fatal(err)
		}
		journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "change", "criteria": []string{}}, 403)
		data, _ := os.ReadFile(outside)
		if string(data) != "outside secret" {
			t.Fatal("brief save overwrote the symlink target")
		}
	} else {
		t.Logf("host cannot create symlinks; traversal/credential checks still ran: %v", err)
	}
}

func TestProjectCreationRefusesOverwriteAndNonHTTPSCredentials(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "existing")
	_ = os.WriteFile(filepath.Join(p.Path, "keep.txt"), []byte("keep"), 0600)
	journeyRequest(t, mux, "POST", "/v1/product/projects", map[string]string{"name": "existing"}, 409)
	for _, remote := range []string{"ssh://git@example.com/app", "https://user:secret@example.com/app", "file:///tmp/app", "https://example.com/app?token=secret", "https://example.com/app?", "https://example.com/app#", "https://example.com/" + strings.Repeat("x", 2048)} {
		journeyRequest(t, mux, "POST", "/v1/product/projects", map[string]string{"name": "new-app", "remoteUrl": remote}, 400)
	}
	for _, name := range []string{"../outside", "CON", "bad/name", "name."} {
		journeyRequest(t, mux, "POST", "/v1/product/projects", map[string]string{"name": name}, 400)
	}
	if _, err := os.Stat(filepath.Join(s.projectRoot(), "new-app")); !os.IsNotExist(err) {
		t.Fatalf("invalid remote created a directory: %v", err)
	}
}

func TestGitJourneyCommitsAsThePersonAndSuppressesRepoPrograms(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "git-app")
	base := "/v1/product/projects/" + p.ID
	git := func(args ...string) string {
		t.Helper()
		out, err := RunCommand(context.Background(), "git", args, p.Path)
		if err != nil {
			t.Fatalf("git %v: %v / %s", args, err, out)
		}
		return out
	}
	marker := filepath.Join(p.Path, "hook-ran")
	hook := "#!/bin/sh\n: > '" + filepath.ToSlash(marker) + "'\n"
	if err := os.MkdirAll(filepath.Join(p.Path, ".git", "hooks"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".git", "hooks", "pre-commit"), []byte(hook), 0700); err != nil {
		t.Fatal(err)
	}
	filterMarker := filepath.Join(p.Path, "filter-ran")
	git("config", "filter.danger.clean", ": > '"+filepath.ToSlash(filterMarker)+"'; cat")
	git("config", "filter.danger.required", "true")
	_ = os.WriteFile(filepath.Join(p.Path, ".gitattributes"), []byte("*.txt filter=danger\n"), 0600)
	_ = os.WriteFile(filepath.Join(p.Path, "hello.txt"), []byte("first\n"), 0600)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Create the app"), 200)
	identity := git("show", "-s", "--format=%an <%ae>", "HEAD")
	if strings.TrimSpace(identity) != "Luci Test Person <person@example.com>" {
		t.Fatalf("wrong identity: %s", identity)
	}
	_ = os.WriteFile(filepath.Join(p.Path, "hello.txt"), []byte("second\n"), 0600)
	w := journeyRequest(t, mux, "GET", base+"/git", nil, 200)
	var summary GitSummary
	_ = json.Unmarshal(w.Body.Bytes(), &summary)
	if summary.Clean || len(summary.Changes) != 1 || summary.Changes[0].Path != "hello.txt" || !strings.Contains(summary.Diff, "+second") {
		t.Fatalf("unexpected change summary: %s", w.Body.String())
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Improve the app"), 200)
	for _, path := range []string{marker, filterMarker} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("repository program executed at %s: %v", path, err)
		}
	}
	_ = os.WriteFile(filepath.Join(p.Path, ".env"), []byte("TOKEN=secret"), 0600)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Should refuse credentials"), 409)
	if _, err := s.resolveProject(p.ID); err != nil {
		t.Fatal(err)
	}
}
