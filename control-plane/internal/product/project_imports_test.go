package product

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

const importTestID = "imp-000000000000000000000001"

func gitImportFixture(t *testing.T) (string, *atomic.Bool) {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		if _, err := RunCommand(context.Background(), "git", args, dir); err != nil {
			t.Fatal(err)
		}
	}
	git(work, "init", "--initial-branch=main", ".")
	if err := os.WriteFile(filepath.Join(work, "owned.txt"), []byte("Existing owned application\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(work, "add", ".")
	git(work, "-c", "user.name=Import Fixture", "-c", "user.email=fixture@example.com", "commit", "-m", "Existing application")
	git(root, "clone", "--bare", work, "repo.git")
	git(filepath.Join(root, "repo.git"), "update-server-info")
	fail := &atomic.Bool{}
	files := http.FileServer(http.Dir(root))
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "fixture unavailable", 503)
			return
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	cert := filepath.Join(t.TempDir(), "fixture-ca.pem")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSL_CAINFO", cert)
	return server.URL + "/repo.git", fail
}
func importRequest(t *testing.T, handler http.Handler, name, remote, id string, status int) *httptest.ResponseRecorder {
	t.Helper()
	return journeyRequest(t, fixtureReviewHandler(handler), "POST", "/v1/product/projects", map[string]string{"name": name, "remoteUrl": remote, "importAttemptId": id}, status)
}
func importStatus(t *testing.T, handler http.Handler, id string) projectImportView {
	t.Helper()
	w := journeyRequest(t, fixtureReviewHandler(handler), "GET", "/v1/product/project-imports/"+id, nil, 200)
	var view projectImportView
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	return view
}
func assertNoImportedProject(t *testing.T, s *Server, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(s.projectRoot(), name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final project exists: %v", err)
	}
	scanner := project.NewScanner(s.projectRoot(), nil)
	projects, err := scanner.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("partial clone entered catalogue: %+v", projects)
	}
}
func TestProjectImportRealGitFailureRetryAndOwnedRecovery(t *testing.T) {
	s, mux := journeyFixture(t)
	remote, fail := gitImportFixture(t)
	fail.Store(true)
	w := importRequest(t, mux, "owned-app", remote, importTestID, 422)
	if strings.Contains(w.Body.String(), remote) || strings.Contains(w.Body.String(), "fixture unavailable") {
		t.Fatal("raw Git diagnostics exposed")
	}
	assertNoImportedProject(t, s, "owned-app")
	failed := importStatus(t, mux, importTestID)
	if failed.Status != "failed" || failed.RetainedPath == "" {
		t.Fatalf("missing actionable retained failure: %+v", failed)
	}
	if _, err := os.Stat(filepath.Join(s.projectRoot(), failed.RetainedPath)); err != nil {
		t.Fatal(err)
	}
	fail.Store(false)
	// Same attempt means the same known failure, not a hidden second Git call.
	importRequest(t, mux, "owned-app", remote, importTestID, 422)
	assertNoImportedProject(t, s, "owned-app")
	next := "imp-000000000000000000000002"
	w = importRequest(t, mux, "owned-app", remote, next, 201)
	var created project.Project
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(filepath.Join(created.Path, "owned.txt"))
	if err != nil || string(bytes) != "Existing owned application\n" {
		t.Fatalf("real repository not restored: %q %v", bytes, err)
	}
	// A fresh server instance recovers from private state, not an in-memory result.
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restartedMux := http.NewServeMux()
	restarted.registerProjects(restartedMux)
	if view := importStatus(t, restartedMux, next); view.Status != "complete" || view.Project == nil || view.Project.ID != created.ID {
		t.Fatalf("restart lost owned completion: %+v", view)
	}
	original := s.runner
	s.runner = func(context.Context, string, []string, string) (string, error) {
		t.Fatal("recovery must not run Git")
		return "", nil
	}
	complete := importStatus(t, mux, next)
	if complete.Status != "complete" || complete.Project == nil || complete.Project.ID != created.ID {
		t.Fatalf("recovery: %+v", complete)
	}
	importRequest(t, mux, "owned-app", remote, next, 200)
	journeyRequest(t, mux, "GET", "/v1/product/project-imports/"+next, nil, 403)
	importRequest(t, mux, "different", remote, next, 409)
	// Lost final receipt is observed without rewriting it or re-cloning.
	receipt, err := s.readImportReceipt(next)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Status = "running"
	receipt.Project = nil
	if err = s.writeImportReceipt(&receipt); err != nil {
		t.Fatal(err)
	}
	if view := importStatus(t, mux, next); view.Status != "complete" {
		t.Fatalf("published result lost: %+v", view)
	}
	retained, err := s.readImportReceipt(next)
	if err != nil || retained.Status != "running" {
		t.Fatal("GET rewrote receipt")
	}
	s.runner = original
	// A different same-name repository must never be adopted as that result.
	if err = os.Rename(created.Path, created.Path+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(created.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if view := importStatus(t, mux, next); view.Status != "unconfirmed" || view.Project != nil {
		t.Fatalf("adopted replacement: %+v", view)
	}
}
func TestProjectImportNoReplaceAndNoRecursiveCleanup(t *testing.T) {
	for _, kind := range []string{"empty-directory", "file", "symlink", "stage-replaced", "root-changed", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			s, mux := journeyFixture(t)
			root := s.projectRoot()
			final := filepath.Join(root, "safe")
			foreign := filepath.Join(t.TempDir(), "foreign")
			if err := os.Mkdir(foreign, 0700); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(foreign, "keep.txt")
			if err := os.WriteFile(sentinel, []byte("Never erase"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var retained string
			s.runner = func(ctx context.Context, _ string, _ []string, dir string) (string, error) {
				if _, err := RunCommand(ctx, "git", []string{"init", "--initial-branch=main", "."}, dir); err != nil {
					return "", err
				}
				switch kind {
				case "empty-directory":
					return "", os.Mkdir(final, 0700)
				case "file":
					return "", os.WriteFile(final, []byte("Existing user work"), 0600)
				case "symlink":
					if err := os.Symlink(foreign, final); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
				case "stage-replaced":
					retained = dir + "-original"
					if err := os.Rename(dir, retained); err != nil {
						return "", err
					}
					return "", os.Symlink(foreign, dir)
				case "root-changed":
					s.mu.Lock()
					s.config.ProjectsDir = t.TempDir()
					s.mu.Unlock()
				case "cancelled":
					cancel()
				}
				return "", nil
			}
			request := httptest.NewRequest("POST", "/v1/product/projects", strings.NewReader(`{"name":"safe","remoteUrl":"https://example.com/repo.git","importAttemptId":"`+importTestID+`"}`)).WithContext(ctx)
			response := httptest.NewRecorder()
			fixtureReviewHandler(mux).ServeHTTP(response, request)
			if response.Code != 422 {
				t.Fatalf("unsafe import accepted: %d %s", response.Code, response.Body.String())
			}
			if bytes, err := os.ReadFile(sentinel); err != nil || string(bytes) != "Never erase" {
				t.Fatal("foreign work was removed")
			}
			if kind == "file" {
				bytes, err := os.ReadFile(final)
				if err != nil || string(bytes) != "Existing user work" {
					t.Fatal("existing file overwritten")
				}
			}
			if kind == "empty-directory" {
				entries, err := os.ReadDir(final)
				if err != nil || len(entries) != 0 {
					t.Fatal("existing empty directory overwritten")
				}
			}
			if kind == "stage-replaced" {
				if _, err := os.Stat(filepath.Join(retained, ".git")); err != nil {
					t.Fatal("owned staged repository deleted")
				}
			}
			if kind == "cancelled" {
				assertNoImportedProject(t, s, "safe")
			}
		})
	}
}
func TestProjectImportInterruptedAndForeignActorAreReadOnly(t *testing.T) {
	s, mux := journeyFixture(t)
	s.runner = func(context.Context, string, []string, string) (string, error) {
		return "", errors.New("private credential helper value")
	}
	importRequest(t, mux, "pending", "https://example.com/repo.git", importTestID, 422)
	receipt, err := s.readImportReceipt(importTestID)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Status = "running"
	if err = s.writeImportReceipt(&receipt); err != nil {
		t.Fatal(err)
	}
	if view := importStatus(t, mux, importTestID); view.Status != "interrupted" {
		t.Fatalf("interrupted attempt: %+v", view)
	}
	importRequest(t, mux, "pending", "https://example.com/repo.git", importTestID, 409)
	receipt.ActorID = "another-account"
	if err = s.writeImportReceipt(&receipt); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, fixtureReviewHandler(mux), "GET", "/v1/product/project-imports/"+importTestID, nil, 404)
	importRequest(t, mux, "pending", "https://example.com/repo.git", importTestID, 409)
}
func TestProjectImportNativeNoReplaceIdentity(t *testing.T) {
	root := t.TempDir()
	from, to := filepath.Join(root, "from"), filepath.Join(root, "to")
	if err := os.Mkdir(from, 0700); err != nil {
		t.Fatal(err)
	}
	identity, err := projectImportDirectoryIdentity(from)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(to, 0700); err != nil {
		t.Fatal(err)
	}
	if err = projectImportRenameNoReplace(from, to); !errors.Is(err, os.ErrExist) {
		t.Fatalf("must preserve occupied destination: %v", err)
	}
	if err = os.Remove(to); err != nil {
		t.Fatal(err)
	}
	if err = projectImportRenameNoReplace(from, to); err != nil {
		t.Fatal(err)
	}
	after, err := projectImportDirectoryIdentity(to)
	if err != nil || after != identity {
		t.Fatalf("move changed identity: %q %q %v", identity, after, err)
	}
	if err = os.Symlink(to, from); err == nil {
		if _, err = projectImportDirectoryIdentity(from + string(os.PathSeparator)); err == nil {
			t.Fatal("followed symlink identity")
		}
	}
}

func TestProjectImportPostPublicationRefusesForeignDirectory(t *testing.T) {
	s, mux := journeyFixture(t)
	s.runner = func(context.Context, string, []string, string) (string, error) {
		return "", errors.New("fixture interruption")
	}
	importRequest(t, mux, "swapped", "https://example.com/repo.git", importTestID, 422)
	receipt, err := s.readImportReceipt(importTestID)
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(receipt.Root, receipt.RetainedPath)
	if _, err = RunCommand(context.Background(), "git", []string{"init", "--initial-branch=main", "."}, stage); err != nil {
		t.Fatal(err)
	}
	receipt.GitIdentity, err = projectImportDirectoryIdentity(filepath.Join(stage, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	receipt.Status = "running"
	receipt.Phase = "publishing"
	if err = s.writeImportReceipt(&receipt); err != nil {
		t.Fatal(err)
	}
	// Model a swap after the last pre-move check: the real no-replace primitive
	// can publish another directory, but the success boundary must not adopt it.
	retained := stage + "-original"
	if err = os.Rename(stage, retained); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(stage, "keep.txt"), []byte("Foreign retained work"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = RunCommand(context.Background(), "git", []string{"init", "--initial-branch=main", "."}, stage); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(receipt.Root, receipt.Name)
	if err = projectImportRenameNoReplace(stage, final); err != nil {
		t.Fatal(err)
	}
	view, err := s.finishImportPublication(&receipt)
	if err == nil || view.Status != "unconfirmed" || view.Project != nil {
		t.Fatalf("foreign publication reported success: %+v %v", view, err)
	}
	result := httptest.NewRecorder()
	importResponse(result, view, 503, "import_unconfirmed")
	if result.Code != 503 || strings.Contains(result.Body.String(), `"project":`) {
		t.Fatal("foreign project exposed as completed")
	}
	persisted, err := s.readImportReceipt(importTestID)
	if err != nil || persisted.Status != "running" || persisted.Phase != "publishing" {
		t.Fatalf("original publication intent changed: %+v %v", persisted, err)
	}
	if bytes, err := os.ReadFile(filepath.Join(final, "keep.txt")); err != nil || string(bytes) != "Foreign retained work" {
		t.Fatal("foreign folder modified or removed")
	}
	if _, err = os.Stat(filepath.Join(retained, ".git")); err != nil {
		t.Fatal("original staging evidence removed")
	}
}
