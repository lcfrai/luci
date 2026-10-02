package project_test

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Review 2, I-1, the other half. head() tolerates ONE silence: git's own, from
// a `rev-parse --verify --quiet HEAD` that RAN and answered non-zero with
// nothing on either stream. A git whose PIPES were closed under it by
// cmd.WaitDelay is silent for a different reason — the process ran, so it is
// not the never-started case either, and its empty streams are os/exec's doing
// and not git's answer. Tolerating it restores F4: the branch list comes back
// with nothing marked IsHead, no error and no log line saying why.
//
// The fixture sets no exit status of its own, which is exactly the point: a
// GitError that does not carry git answering for itself says nothing about a
// repository, however silent it is.
func TestBranchesDoesNotCallAConfirmationUnbornWhenGitNeverAnsweredItself(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")

	// The first git: F2's own case, killed at this runner's bound.
	killed := &project.GitError{
		Args:     []string{"rev-parse", "--abbrev-ref", "HEAD"},
		Dir:      dir,
		Err:      errors.New("exit status 1"),
		Timeout:  10 * time.Second,
		Elapsed:  10 * time.Second,
		TimedOut: true,
		Exited:   true,
	}
	// The confirmation: a process that RAN and exited 0 while a child held its
	// pipes, so os/exec gave up on them. Both streams empty, neither context
	// done — git said nothing because nobody could hear it.
	unheard := &project.GitError{
		Args:    []string{"rev-parse", "--verify", "--quiet", "HEAD"},
		Dir:     dir,
		Err:     exec.ErrWaitDelay,
		Timeout: 10 * time.Second,
		Elapsed: 2 * time.Second,
		Exited:  true,
	}

	g := newFakeGit()
	g.reply(dir, "main\nfeat/ready-flag\n", headRefs, "--format=%(refname:short)", "refs/heads")
	g.fail(dir, killed, "rev-parse", "--abbrev-ref", "HEAD")
	g.fail(dir, unheard, "rev-parse", "--verify", "--quiet", "HEAD")

	got, err := branchesOf(t, root, dir, g)
	if err == nil {
		t.Fatalf("Branches whose confirmation was silenced by its own pipes = %#v, nil; "+
			"want the killed rev-parse's error, not a guessed unborn HEAD", got)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded, so the rail cannot say the read was killed", err)
	}
}
