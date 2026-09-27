package project_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

const headRefs = "for-each-ref"

func branchesOf(t *testing.T, root, dir string, g project.Runner) ([]project.Branch, error) {
	t.Helper()
	return project.NewScanner(root, g).Branches(context.Background(), project.ProjectID(dir))
}

// F4. Scanner.Branches tolerated EVERY failure of rev-parse --abbrev-ref HEAD,
// though its comment gave one reason: a repository with no commits. So a
// rev-parse killed by F2's bound returned the branch list with nothing marked
// IsHead — a wrong fact, drawn confidently, with no error and no log line to
// say why. Only an unborn HEAD may be tolerated.
func TestBranchesReportsATimedOutRevParseInsteadOfDroppingTheHeadMarker(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")

	killed := &project.GitError{
		Args:     []string{"rev-parse", "--abbrev-ref", "HEAD"},
		Dir:      dir,
		Err:      errors.New("exit status 1"),
		Timeout:  10 * time.Second,
		TimedOut: true,
	}
	g := newFakeGit()
	g.reply(dir, "main\nfeat/ready-flag\n", headRefs, "--format=%(refname:short)", "refs/heads")
	g.fail(dir, killed, "rev-parse", "--abbrev-ref", "HEAD")
	g.fail(dir, killed, "rev-parse", "--verify", "--quiet", "HEAD")

	got, err := branchesOf(t, root, dir, g)
	if err == nil {
		t.Fatalf("Branches with a killed rev-parse = %#v, nil; want the error", got)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded", err)
	}
}

// A git that answered with its own failure is not an unborn HEAD either.
func TestBranchesReportsARevParseThatGitItselfRefused(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")

	broke := &project.GitError{
		Args:   []string{"rev-parse", "--abbrev-ref", "HEAD"},
		Dir:    dir,
		Err:    errors.New("exit status 128"),
		Stderr: "fatal: not a git repository: .git",
	}
	g := newFakeGit()
	g.reply(dir, "main\n", headRefs, "--format=%(refname:short)", "refs/heads")
	g.fail(dir, broke, "rev-parse", "--abbrev-ref", "HEAD")
	g.fail(dir, broke, "rev-parse", "--verify", "--quiet", "HEAD")

	got, err := branchesOf(t, root, dir, g)
	if err == nil {
		t.Fatalf("Branches with a refused rev-parse = %#v, nil; want the error", got)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want git's own sentence in it", err)
	}
}

// The one tolerated reason, and it stays tolerated: HEAD names a branch that
// has no commit yet. `rev-parse --verify --quiet HEAD` is how that is
// confirmed rather than guessed from git's prose — it exits non-zero and says
// NOTHING, where every other failure prints.
func TestBranchesStillListsBranchesWhenHeadIsUnborn(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")

	g := newFakeGit()
	g.reply(dir, "main\n", headRefs, "--format=%(refname:short)", "refs/heads")
	g.fail(dir, &project.GitError{
		Args: []string{"rev-parse", "--abbrev-ref", "HEAD"},
		Dir:  dir,
		Err:  errors.New("exit status 128"),
		// git prints its ambiguous-argument prose here; the confirmation
		// below is what decides, not this text.
		Stderr: "fatal: ambiguous argument 'HEAD': unknown revision",
	}, "rev-parse", "--abbrev-ref", "HEAD")
	g.fail(dir, &project.GitError{
		Args: []string{"rev-parse", "--verify", "--quiet", "HEAD"},
		Dir:  dir,
		Err:  errors.New("exit status 1"),
	}, "rev-parse", "--verify", "--quiet", "HEAD")

	got, err := branchesOf(t, root, dir, g)
	if err != nil {
		t.Fatalf("Branches on an unborn HEAD: %v", err)
	}
	if len(got) != 1 || got[0].Name != "main" {
		t.Fatalf("Branches = %#v, want main alone", got)
	}
	if got[0].IsHead {
		t.Errorf("main is marked IsHead, but no commit is checked out")
	}
}

// The same unborn HEAD against real git, so the confirmation is pinned to
// git's behaviour and not to this package's belief about it: an orphan branch
// alongside a branch that does have commits.
func TestBranchesAgainstARealRepositoryWhoseHeadIsUnborn(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	dir := filepath.Join(root, "panacea")
	makeGitRepo(t, dir)

	cmd := exec.Command("git", "checkout", "--orphan", "blank")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout --orphan blank: %v\n%s", err, out)
	}

	got, err := branchesOf(t, root, dir, project.NewExecRunner())
	if err != nil {
		t.Fatalf("Branches on a real unborn HEAD: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("Branches on a real unborn HEAD returned no branches; main has commits")
	}
	for _, b := range got {
		if b.IsHead {
			t.Errorf("%s is marked IsHead, but HEAD is unborn", b.Name)
		}
	}
}
