package project_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH; skipping the ExecRunner tests")
	}
}

// makeGitRepo builds a real repository from nothing. The developer's own git
// config is neutralised so init.defaultBranch, templates, hooks and signing
// cannot change what the test observes. Requires git >= 2.28 for `init -b`.
func makeGitRepo(t *testing.T, dir string) {
	t.Helper()
	requireGit(t)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	cfg := t.TempDir()
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(cfg, "gitconfig"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(cfg, "gitconfig-system"),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	run := func(args ...string) {
		t.Helper()
		args = append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial")
	run("branch", "evidence-upload")
}

func TestExecRunnerReturnsGitsOutput(t *testing.T) {
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "luci")
	makeGitRepo(t, dir)

	out, err := project.NewExecRunner().Run(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "main" {
		t.Fatalf("rev-parse --abbrev-ref HEAD = %q, want %q", got, "main")
	}
}

// "exit status 128" in the rail tells the user nothing. On FAILURE — and only
// on failure — the runner folds both streams into the error so it carries
// git's own sentence. On success the caller gets stdout alone; see
// TestExecRunnerDoesNotLetAGitWarningOnStderrBecomeABranch.
func TestExecRunnerErrorCarriesGitsOwnMessage(t *testing.T) {
	requireGit(t)

	_, err := project.NewExecRunner().Run(context.Background(), t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatal("Run in a directory that is not a repository returned no error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %q, want git's own message in it", err)
	}
}

func TestExecRunnerHonoursACancelledContext(t *testing.T) {
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "luci")
	makeGitRepo(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if _, err := project.NewExecRunner().Run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		t.Fatal("Run with a cancelled context returned no error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run took %s to notice a cancelled context", elapsed)
	}
}

func TestScannerAgainstARealRepositoryAndARealLinkedWorktree(t *testing.T) {
	requireGit(t)

	// Normalise the temp root before a single path is built from it. On the
	// Windows development machine os.TempDir() is the 8.3 short form
	// C:\Users\AU-DEV~1\AppData\Local\Temp, while git normalises every path it
	// prints and emits the long form C:/Users/au-dev-dmackie1/...; on macOS the
	// same call resolves /var to /private/var. parseWorktrees only Cleans and
	// worktree.Verify only Abs's, and neither expands a short component or a
	// symlink, so without this the string this test builds and the string git
	// reports are two spellings of one directory and every path comparison
	// below fails.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(root): %v", err)
	}
	repo := filepath.Join(root, "luci")
	makeGitRepo(t, repo)

	// A real linked worktree, created OUTSIDE root so it is not also
	// enumerated as a project. The env neutralisation repeats makeGitRepo's on
	// purpose, so this test reads standalone.
	linked := filepath.Join(t.TempDir(), "evidence-upload")
	cfg := t.TempDir()
	add := exec.Command("git", "worktree", "add", linked, "evidence-upload")
	add.Dir = repo
	add.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(cfg, "gitconfig"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(cfg, "gitconfig-system"),
	)
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	// The same normalisation for the worktree, and it has to come after the
	// directory exists: EvalSymlinks fails on a path that is not there yet.
	// This is the value the Path assertion at the bottom compares against.
	linked, err = filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatalf("EvalSymlinks(linked): %v", err)
	}

	s := project.NewScanner(root, project.NewExecRunner())
	ctx := context.Background()

	projects, err := s.Projects(ctx)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "luci" {
		t.Fatalf("Projects() = %#v, want exactly the luci repository", projects)
	}
	pid := projects[0].ID

	branches, err := s.Branches(ctx, pid)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	seen := map[string]bool{}
	for _, b := range branches {
		seen[b.Name] = true
		if b.Name == "main" && !b.IsHead {
			t.Error("main is not marked IsHead in the primary worktree's repository")
		}
	}
	if !seen["main"] || !seen["evidence-upload"] {
		t.Fatalf("Branches() = %#v, want main and evidence-upload", branches)
	}

	worktrees, err := s.Worktrees(ctx, pid)
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("Worktrees() = %#v, want the primary and the linked one", worktrees)
	}
	if !worktrees[0].IsPrimary || worktrees[1].IsPrimary {
		t.Errorf("IsPrimary = %v, %v; want only the first record", worktrees[0].IsPrimary, worktrees[1].IsPrimary)
	}
	if worktrees[1].Name != "evidence-upload" {
		t.Errorf("Worktrees()[1].Name = %q, want evidence-upload", worktrees[1].Name)
	}
	if worktrees[1].BranchID != project.BranchID(pid, "evidence-upload") {
		t.Errorf("Worktrees()[1].BranchID = %q, want the derived id", worktrees[1].BranchID)
	}

	got, err := s.Path(ctx, worktrees[1].ID)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != linked {
		t.Fatalf("Path = %q, want %q — both sides are EvalSymlinks-normalised, so a "+
			"difference here is a real resolution bug and not a short-name or "+
			"separator artefact: filepath.Clean in parseWorktrees already turns "+
			"git's forward slashes into the platform separator", got, linked)
	}
	if name := s.ProjectName(pid); name != "luci" {
		t.Errorf("ProjectName after enumeration = %q, want luci", name)
	}
}

// Every other test in this file runs a git that either succeeds silently or
// fails loudly, and neither shape can catch the runner folding stderr into
// stdout. This one scripts the third shape — git exits 0 and still writes to
// stderr — because that is the one a line-by-line parser gets wrong. A corrupt
// loose ref makes `for-each-ref` print
// "warning: ignoring broken ref refs/heads/garbage" on stderr, the real
// branches on stdout, and exit 0; a runner returning combined output would put
// that sentence through Scanner.Branches and the rail would grow a branch
// literally named "warning: ignoring broken ref refs/heads/garbage" with a
// matching derived id, while rev-parse --abbrev-ref HEAD would return
// "warning: …\nmain" and match nothing, so no branch would be marked IsHead.
// Without this test that defect ships green.
func TestExecRunnerDoesNotLetAGitWarningOnStderrBecomeABranch(t *testing.T) {
	requireGit(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(root): %v", err)
	}
	repo := filepath.Join(root, "luci")
	makeGitRepo(t, repo)

	// A corrupt loose ref. git reports it and carries on: the complaint goes
	// to stderr, the good refs to stdout, and the exit status is 0.
	broken := filepath.Join(repo, ".git", "refs", "heads", "garbage")
	if err := os.WriteFile(broken, []byte("not-a-sha\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Confirm the premise before asserting on it. git's diagnostics are not a
	// stable interface; if this build refuses rather than warns, or says
	// nothing at all, the test has nothing to prove and skips rather than
	// failing for a reason that is not the product's fault.
	probe := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	probe.Dir = repo
	var pout, perr bytes.Buffer
	probe.Stdout, probe.Stderr = &pout, &perr
	if err := probe.Run(); err != nil {
		t.Skipf("this git fails rather than warns on a broken loose ref: %v", err)
	}
	if !strings.Contains(perr.String(), "warning") {
		t.Skip("this git emits no stderr warning for a broken loose ref")
	}
	if strings.Contains(pout.String(), "warning") {
		t.Fatalf("premise broken: git put its warning on stdout:\n%s", pout.String())
	}

	// The runner's own output must be stdout alone.
	out, err := project.NewExecRunner().Run(context.Background(), repo,
		"for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(string(out), "warning") {
		t.Fatalf("Run folded git's stderr into its result:\n%s", out)
	}

	// And the same through the parser, which is where a user would see it.
	s := project.NewScanner(root, project.NewExecRunner())
	ctx := context.Background()
	projects, err := s.Projects(ctx)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("Projects() = %#v, want exactly the luci repository", projects)
	}
	branches, err := s.Branches(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	names := map[string]bool{}
	head := ""
	for _, b := range branches {
		names[b.Name] = true
		if b.IsHead {
			head = b.Name
		}
	}
	if len(names) != 2 || !names["main"] || !names["evidence-upload"] {
		t.Fatalf("Branches() = %#v, want exactly main and evidence-upload; a stderr warning leaked into the parse", branches)
	}
	if head != "main" {
		t.Fatalf("IsHead branch = %q, want main; rev-parse's stdout carried a stderr warning", head)
	}
}
