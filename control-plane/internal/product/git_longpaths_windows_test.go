package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Real Git, not an argv mirror: the managed Windows account root reproduced
// an init failure at .git/hooks/fsmonitor-watchman.sample (260 characters).
// Exercise all three process boundaries with a longer path and no existing
// long-path setting, without modifying any real person's Git configuration.
func TestWindowsLongProjectPathsAcrossJourneyScannerAndArchive(t *testing.T) {
	s, _ := journeyFixture(t)
	global := filepath.Join(t.TempDir(), "gitconfig")
	globalBytes := []byte("[user]\n\tname = Long Path Fixture\n")
	if err := os.WriteFile(global, globalBytes, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	// Match the live failure: Git's working directory fits MAX_PATH, but its
	// generated hooks and ordinary application descendants do not. Windows'
	// process-start limit on the working directory is a separate constraint.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(root) >= 210 {
		t.Fatalf("temporary root is too long for this process-start fixture: %d", len(root))
	}
	root = filepath.Join(root, strings.Repeat("x", 210-len(root)-1))
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	s.config.ProjectsDir = root
	mux := http.NewServeMux()
	s.registerProjects(mux)
	p := createJourneyProject(t, mux, "long-path-app")
	if len(filepath.Join(p.Path, ".git", "hooks", "fsmonitor-watchman.sample")) <= 260 {
		t.Fatal("fixture does not reproduce the Windows path boundary")
	}
	content := []byte("A real committed application file.\n")
	fileName := "application-" + strings.Repeat("x", 40) + ".html"
	if len(filepath.Join(p.Path, fileName)) <= 260 {
		t.Fatal("fixture application file does not exceed MAX_PATH")
	}
	if err := os.WriteFile(filepath.Join(p.Path, fileName), content, 0600); err != nil {
		t.Fatal(err)
	}
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Commit under the managed long path"), 200)
	status := journeyRequest(t, mux, "GET", base+"/git", nil, 200)
	var summary GitSummary
	if err := json.Unmarshal(status.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if !summary.Clean || len(summary.Log) != 1 {
		t.Fatalf("commit/status did not complete: %#v", summary)
	}
	ctx := context.Background()
	scanner := project.NewScanner(root, project.NewExecRunner()).WithProjectBoundary()
	rows, err := scanner.Worktrees(ctx, p.ID)
	if err != nil || len(rows) != 1 || rows[0].ID != project.WorktreeID(p.Path) {
		t.Fatalf("bounded worktree scan: %#v, %v", rows, err)
	}
	resolved, err := scanner.Path(ctx, rows[0].ID)
	if err != nil || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(p.Path)) {
		t.Fatalf("bounded worktree resolution: %q, %v", resolved, err)
	}
	extracted := filepath.Join(root, "archive-output")
	if err := os.Mkdir(extracted, 0700); err != nil {
		t.Fatal(err)
	}
	if err := archiveCommittedProject(ctx, p.Path, summary.Log[0].Hash, extracted); err != nil {
		t.Fatal(err)
	}
	archived, err := os.ReadFile(filepath.Join(extracted, fileName))
	if err != nil || string(archived) != string(content) {
		t.Fatalf("committed content did not survive archive: %q, %v", archived, err)
	}
	got, err := os.ReadFile(global)
	if err != nil || string(got) != string(globalBytes) {
		t.Fatalf("managed Git modified global configuration: %q, %v", got, err)
	}
	// Ensure the same endpoint still refuses untrusted transport input; the
	// platform setting must not create a new route around the clone allowlist.
	r := httptest.NewRequest("POST", "/v1/product/projects", strings.NewReader(`{"name":"refused","remoteUrl":"file:///outside"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unsupported clone transport accepted: %d", w.Code)
	}
	t.Logf("Windows Git init, commit, status, bounded scan and archive passed; project path %d characters, application path %d characters", len(p.Path), len(filepath.Join(p.Path, fileName)))
}
