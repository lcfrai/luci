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

	"github.com/lcfrai/luci/control-plane/internal/project"
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
	dir := makeRepo(t, root, "luci")

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
	dir := makeRepo(t, root, "luci")

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

// Review I-1. A CANCELLED confirmation confirms nothing either, and it is the
// ordinary path rather than an exotic one: the rail cancels a superseded poll
// as a matter of course, and both gits of the read then die the same way with
// nothing on either stream. Without head()'s `&& !ge.Canceled` guard the
// silence of a cancelled git reads as "HEAD is unborn", so Branches answers
// with the full branch list and nothing marked IsHead — F4 exactly, restored,
// on a request whose reader has already gone. Deleting that guard left the
// whole package green before this test existed.
func TestBranchesReportsACancelledConfirmationInsteadOfCallingItAnUnbornHead(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")

	// Both gits die the way a cancelled git dies on Windows: the corpse's own
	// exit status, and nothing on either stream.
	cancelled := func(args ...string) *project.GitError {
		return &project.GitError{
			Args:     args,
			Dir:      dir,
			Err:      errors.New("exit status 1"),
			Timeout:  10 * time.Second,
			Elapsed:  120 * time.Millisecond,
			Canceled: true,
			Caller:   true,
		}
	}
	g := newFakeGit()
	g.reply(dir, "main\nfeat/ready-flag\n", headRefs, "--format=%(refname:short)", "refs/heads")
	g.fail(dir, cancelled("rev-parse", "--abbrev-ref", "HEAD"), "rev-parse", "--abbrev-ref", "HEAD")
	g.fail(dir, cancelled("rev-parse", "--verify", "--quiet", "HEAD"), "rev-parse", "--verify", "--quiet", "HEAD")

	got, err := branchesOf(t, root, dir, g)
	if err == nil {
		t.Fatalf("Branches with a cancelled confirmation = %#v, nil; want the error", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %q does not wrap context.Canceled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q reads as a timeout, but the caller cancelled", err)
	}
}

// HEAD can name a current branch before its first commit. A silent Git-owned
// nonzero verification is followed by a successful symbolic-ref and ref-format
// check; missing object output alone must never invent a branch.
func TestBranchesStillListsBranchesWhenHeadIsUnborn(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")

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
		// A git that RAN and ANSWERED, and whose answer was nothing.
		// Silence alone is not the confirmation: a git that never started
		// is silent too (review I-1), and so is one whose pipes were closed
		// under it while it ran (review 2, I-1) — hence both facts here.
		// The real-git test below is what keeps this fake honest.
		Exited:        true,
		ExitedNonZero: true,
	}, "rev-parse", "--verify", "--quiet", "HEAD")
	g.reply(dir, "refs/heads/main\n", "symbolic-ref", "--quiet", "HEAD")
	g.reply(dir, "", "check-ref-format", "refs/heads/main")

	got, err := branchesOf(t, root, dir, g)
	if err != nil {
		t.Fatalf("Branches on an unborn HEAD: %v", err)
	}
	if len(got) != 1 || got[0].Name != "main" {
		t.Fatalf("Branches = %#v, want main alone", got)
	}
	if !got[0].IsHead {
		t.Error("the verified current initial branch was not marked IsHead")
	}
}

// The same unborn HEAD against real git, so the confirmation is pinned to
// git's behaviour and not to this package's belief about it: an orphan branch
// alongside a branch that does have commits.
func TestBranchesAgainstARealRepositoryWhoseHeadIsUnborn(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	dir := filepath.Join(root, "luci")
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
	initial := 0
	for _, b := range got {
		if b.Name == "blank" && b.IsHead {
			initial++
		} else if b.IsHead {
			t.Errorf("%s is marked IsHead; the current initial branch is blank", b.Name)
		}
	}
	if initial != 1 {
		t.Fatal("the verified initial branch was not exposed alongside existing branches")
	}
}
