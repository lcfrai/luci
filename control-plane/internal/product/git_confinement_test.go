package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestManagedCommitDoesNotStartDeferredMaintenance(t *testing.T) {
	s, mux := journeyFixture(t)
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	p := createJourneyProject(t, mux, "owned-maintenance")
	for key, value := range map[string]string{"maintenance.auto": "true", "maintenance.autoDetach": "true", "gc.auto": "1", "gc.autoDetach": "true"} {
		confinedFixtureGit(t, p.Path, "config", key, value)
	}
	configPath := filepath.Join(p.Path, ".git", "config")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(t.TempDir(), "managed-git-trace.jsonl")
	traced := func(ctx context.Context, executable string, args []string, dir string) (string, error) {
		// Inject only this test's private trace after production environment
		// sanitation; controller commands never inherit a caller's trace target.
		env := append(project.ManagedGitEnvironment(os.Environ()), "GIT_TRACE2_EVENT="+filepath.ToSlash(tracePath))
		return runCommandEnvironment(ctx, executable, args, dir, env)
	}
	s.runner, s.branchRunner = traced, traced
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Committed without a detached metadata writer\n"), 0600); err != nil {
		t.Fatal(err)
	}
	committed := journeyRequest(t, mux, "POST", branchBase(p)+"git/commit", map[string]string{"message": "Explicit controller commit"}, 200)
	var result struct{ Commit string }
	if json.Unmarshal(committed.Body.Bytes(), &result) != nil || !deliverySHA.MatchString(result.Commit) {
		t.Fatal("managed commit did not return its verified revision")
	}
	catalogue := branchCatalogue(t, mux, p)
	if catalogue.Commit != result.Commit {
		t.Fatal("immediate metadata verification lost the committed revision")
	}
	decodeBranchResult(t, journeyRequest(t, mux, "POST", branchBase(p)+"git/branches", branchCreateRequest(catalogue, "managed-feature", "managed-no-background-writer"), 200))
	assertNoGitMaintenance(t, tracePath)
	for _, name := range []string{"objects/maintenance.lock", "gc.pid", "gc.log.lock"} {
		if _, err := os.Lstat(filepath.Join(p.Path, ".git", filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatal("deferred maintenance metadata survived the controller command", name, err)
		}
	}
	if err := validateBranchMetadata(context.Background(), s.projectRoot(), p.Path); err != nil {
		t.Fatal("strict metadata guard rejected the completed command", err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("controller policy rewrote the user's native Git configuration", err)
	}
}

func assertNoGitMaintenance(t *testing.T, tracePath string) {
	t.Helper()
	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal("real Git trace was not recorded", err)
	}
	starts := 0
	for _, line := range strings.Split(strings.TrimSpace(string(trace)), "\n") {
		var event struct {
			Event string
			Argv  []string
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event == "start" {
			starts++
		}
		if event.Event == "child_start" && len(event.Argv) >= 2 && (event.Argv[1] == "maintenance" || event.Argv[1] == "gc") {
			t.Fatalf("native command launched deferred Git maintenance: %v", event.Argv)
		}
	}
	if starts == 0 {
		t.Fatal("no real Git execution observed")
	}
}

func TestNativePreviewFixtureCommitsWithoutDeferredMaintenance(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "native-preview-maintenance")
	for key, value := range map[string]string{"maintenance.auto": "true", "maintenance.autoDetach": "true", "gc.auto": "1", "gc.autoDetach": "true"} {
		confinedFixtureGit(t, p.Path, "config", key, value)
	}
	tracePath := filepath.Join(t.TempDir(), "native-fixture-git-trace.jsonl")
	t.Setenv("GIT_TRACE2_EVENT", filepath.ToSlash(tracePath))
	// These bytes intentionally bypass public commit/review rules. Keep using
	// actual native Git so deployment's later secret/review checks stay real.
	if err := os.WriteFile(filepath.Join(p.Path, ".env"), []byte("FIXTURE_PRIVATE_VALUE=must-not-enter-release\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	assertNoGitMaintenance(t, tracePath)
	head, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil || !deliverySHA.MatchString(strings.TrimSpace(head)) {
		t.Fatal("native fixture did not leave an immediately verifiable commit", err)
	}
	contents, err := s.journeyGit(context.Background(), p.Path, "show", "HEAD:.env")
	if err != nil || !strings.Contains(contents, "FIXTURE_PRIVATE_VALUE") {
		t.Fatal("fixture lost its intentionally adversarial committed bytes", err)
	}
}

func confinedFixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	// Keep adversarial native configuration intact, but never leave fixture
	// maintenance mutating metadata after this synchronous helper returns.
	args = append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)
	out, err := runCommandEnvironment(context.Background(), "git", args, dir, project.ManagedGitEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("scratch Git %v failed: %v", args, err)
	}
	return strings.TrimSpace(out)
}

func commitConfinedFixture(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	confinedFixtureGit(t, dir, "add", "--", "README.md")
	confinedFixtureGit(t, dir, "-c", "user.name=Boundary fixture", "-c", "user.email=fixture@example.test", "commit", "--no-gpg-sign", "-m", content)
	return confinedFixtureGit(t, dir, "rev-parse", "HEAD")
}

func TestJourneyGitAndArchiveRejectForeignMetadataBeforeStartingGit(t *testing.T) {
	for _, attack := range []string{"commondir", "alternates", "include", "config-symlink", "nested-ref-symlink", "index-hardlink"} {
		t.Run(attack, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "owned")
			commitConfinedFixture(t, p.Path, "owned project")
			foreign := t.TempDir()
			confinedFixtureGit(t, foreign, "init", "--initial-branch=main", ".")
			const private = "OTHER_ACCOUNT_PRIVATE_COMMIT"
			foreignHead := commitConfinedFixture(t, foreign, private)
			gitDir, foreignGit := filepath.Join(p.Path, ".git"), filepath.Join(foreign, ".git")
			write := func(path, value string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch attack {
			case "commondir":
				write(filepath.Join(gitDir, "commondir"), filepath.ToSlash(foreignGit)+"\n")
				// Prove the exact metadata bypass even with explicit directory
				// pins, using scratch repositories only.
				out := confinedFixtureGit(t, p.Path, "--git-dir="+gitDir, "--work-tree="+p.Path, "log", "-1", "--format=%s")
				if out != private {
					t.Fatal("scratch common-directory attack did not reproduce")
				}
			case "alternates":
				write(filepath.Join(gitDir, "objects", "info", "alternates"), filepath.ToSlash(filepath.Join(foreignGit, "objects"))+"\n")
			case "include":
				confinedFixtureGit(t, p.Path, "config", "include.path", filepath.ToSlash(filepath.Join(foreignGit, "config")))
			case "config-symlink":
				if err := os.Remove(filepath.Join(gitDir, "config")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(foreignGit, "config"), filepath.Join(gitDir, "config")); err != nil {
					t.Skipf("symlink fixture unavailable: %v", err)
				}
			case "nested-ref-symlink":
				if err := os.Symlink(filepath.Join(foreignGit, "refs", "heads", "main"), filepath.Join(gitDir, "refs", "heads", "foreign")); err != nil {
					t.Skipf("symlink fixture unavailable: %v", err)
				}
			case "index-hardlink":
				if err := os.Remove(filepath.Join(gitDir, "index")); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(filepath.Join(foreignGit, "index"), filepath.Join(gitDir, "index")); err != nil {
					t.Skipf("hard-link fixture unavailable: %v", err)
				}
			}
			before := auditTree(t, foreign)
			calls := 0
			s.runner = func(ctx context.Context, executable string, args []string, dir string) (string, error) {
				calls++
				return RunCommand(ctx, executable, args, dir)
			}
			for _, args := range [][]string{{"log", "-1", "--format=%s"}, {"config", "--get", "remote.origin.url"}, {"add", "--all", "--", "."}, {"update-ref", "refs/heads/main", foreignHead}} {
				out, err := s.journeyGit(context.Background(), p.Path, args...)
				if err == nil || out != "" || strings.Contains(err.Error(), private) {
					t.Fatalf("unsafe metadata reached Git or leaked foreign content: output=%q err=%v", out, err)
				}
			}
			base := "/v1/product/projects/" + p.ID + "/git"
			journeyRequest(t, mux, "GET", base, nil, 422)
			journeyRequest(t, mux, "POST", base+"/commit", map[string]string{"message": "must not mutate another account"}, 422)
			if calls != 0 {
				t.Fatalf("unsafe metadata started %d Git commands", calls)
			}
			destination := t.TempDir()
			if err := archiveCommittedProject(context.Background(), p.Path, foreignHead, destination); err == nil {
				t.Fatal("unsafe metadata was accepted as a release build context")
			}
			entries, err := os.ReadDir(destination)
			if err != nil || len(entries) != 0 {
				t.Fatal("refused archive left foreign content in its build directory")
			}
			if !reflect.DeepEqual(before, auditTree(t, foreign)) {
				t.Fatal("refused project operation changed another account's repository")
			}
		})
	}
}

func TestJourneyGitPreservesInternalLinkedWorktreesAndRejectsForeignOnes(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "owned")
	commit := commitConfinedFixture(t, p.Path, "owned project")
	linked := filepath.Join(p.Path, ".luci", "worktrees", "feature")
	confinedFixtureGit(t, p.Path, "worktree", "add", "-b", "feature", linked)
	out, err := s.journeyGit(context.Background(), linked, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(out) != commit {
		t.Fatalf("owned linked-worktree inspection failed: %q %v", out, err)
	}
	if err := os.WriteFile(filepath.Join(linked, "README.md"), []byte("feature branch edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.journeyGit(context.Background(), linked, "add", "--", "README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.journeyGit(context.Background(), linked, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--no-gpg-sign", "-m", "Feature edit"); err != nil {
		t.Fatal(err)
	}
	if primary := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD"); primary != commit {
		t.Fatal("linked-worktree commit changed the primary branch")
	}
	foreign := t.TempDir()
	confinedFixtureGit(t, foreign, "init", "--initial-branch=main", ".")
	commitConfinedFixture(t, foreign, "foreign project")
	if err := os.WriteFile(filepath.Join(linked, ".git"), []byte("gitdir: "+filepath.ToSlash(filepath.Join(foreign, ".git"))+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := s.journeyGit(context.Background(), linked, "log", "-1", "--format=%s"); err == nil || out != "" {
		t.Fatal("foreign linked-worktree metadata reached Git")
	}
}

func TestJourneyGitAndArchiveIgnoreInheritedRepositoryRedirection(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "owned")
	commit := commitConfinedFixture(t, p.Path, "owned project")
	foreign := t.TempDir()
	confinedFixtureGit(t, foreign, "init", "--initial-branch=main", ".")
	foreignCommit := commitConfinedFixture(t, foreign, "foreign project")
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_COMMON_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(foreign, ".git", "index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.worktree")
	t.Setenv("GIT_CONFIG_VALUE_0", foreign)
	// Exercise the same context-preserving wrappers used by existing adapters.
	s.runner = func(ctx context.Context, executable string, args []string, dir string) (string, error) {
		return RunCommand(ctx, executable, args, dir)
	}
	out, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(out) != commit || strings.TrimSpace(out) == foreignCommit {
		t.Fatalf("inherited Git redirection escaped confinement: %q %v", out, err)
	}
	destination := filepath.Join(t.TempDir(), "private-archive")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	if err := archiveCommittedProject(context.Background(), p.Path, commit, destination); err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(destination, "README.md"))
	if err != nil || string(readme) != "owned project" {
		t.Fatal("archive used an inherited foreign repository")
	}
}

func TestJourneyGitKeepsTrustedIdentityAndCredentialHelperDiscovery(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "owned")
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\nname = Connected User\nemail = connected@example.test\n[credential]\nhelper = test-connected-helper\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	identity, err := s.journeyGit(context.Background(), t.TempDir(), "config", "--get", "user.name")
	if err != nil || strings.TrimSpace(identity) != "Connected User" {
		t.Fatalf("trusted host identity unavailable: %q %v", identity, err)
	}
	helper, err := s.journeyGit(context.Background(), p.Path, "config", "--global", "--get-urlmatch", "credential.helper", "https://example.test/repository.git")
	if err != nil || strings.TrimSpace(helper) != "test-connected-helper" {
		t.Fatalf("trusted Git helper unavailable: %q %v", helper, err)
	}
	if out, err := s.journeyGit(context.Background(), p.Path, "config", "--get", "user.name"); err == nil || out != "" {
		t.Fatal("managed repository inspection inherited host configuration")
	}
}

func TestJourneyGitStopsWhenFilterInspectionFails(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "owned")
	failed := errors.New("filter inspection unavailable")
	calls := 0
	s.runner = func(context.Context, string, []string, string) (string, error) {
		calls++
		return "", failed
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "add", "--all", "--", "."); !errors.Is(err, failed) || calls != 1 {
		t.Fatal("failed filter discovery proceeded to a worktree operation")
	}
}
