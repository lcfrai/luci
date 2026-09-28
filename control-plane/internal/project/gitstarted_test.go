package project_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// Review I-1. head() tolerates ONE silence — `rev-parse --verify --quiet HEAD`
// exiting non-zero with nothing on either stream, which is git confirming that
// HEAD resolves to nothing. A git that NEVER STARTED produces the same three
// facts: no output, no timeout, no cancellation. So a confirmation that was
// never spawned was read as a confirmed unborn HEAD, the first rev-parse's
// error was discarded whatever it had been, and Branches answered with the
// whole branch list and nothing marked IsHead — F4's own defect, a wrong fact
// drawn confidently with no error and no log, restored on the transient
// per-process stall this change exists for.
//
// The shape below is not invented; TestExecRunnerSaysWhenNoGitProcessEverStarted
// measures it from a real ExecRunner.
func TestBranchesReportsAConfirmationThatNeverStartedRatherThanCallingItUnborn(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")

	// F2's own case for the first git: killed at this runner's bound.
	killed := &project.GitError{
		Args:     []string{"rev-parse", "--abbrev-ref", "HEAD"},
		Dir:      dir,
		Err:      errors.New("exit status 1"),
		Timeout:  10 * time.Second,
		Elapsed:  10 * time.Second,
		TimedOut: true,
		Exited:   true,
	}
	// And a confirmation that never ran: os/exec's start error, both streams
	// empty, neither context done, no process.
	unstarted := &project.GitError{
		Args:    []string{"rev-parse", "--verify", "--quiet", "HEAD"},
		Dir:     dir,
		Err:     errors.New("chdir " + dir + ": The system cannot find the file specified."),
		Timeout: 10 * time.Second,
	}

	g := newFakeGit()
	g.reply(dir, "main\nfeat/ready-flag\n", headRefs, "--format=%(refname:short)", "refs/heads")
	g.fail(dir, killed, "rev-parse", "--abbrev-ref", "HEAD")
	g.fail(dir, unstarted, "rev-parse", "--verify", "--quiet", "HEAD")

	got, err := branchesOf(t, root, dir, g)
	if err == nil {
		t.Fatalf("Branches whose confirmation never started = %#v, nil; want the killed rev-parse's error", got)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded, so the rail cannot say the read was killed", err)
	}
}

// The measurement the test above leans on: what a git that could not start
// actually looks like. If os/exec ever fills a stream or sets a context on
// this path, the reasoning above changes and this goes red first.
func TestExecRunnerSaysWhenNoGitProcessEverStarted(t *testing.T) {
	requireGit(t)
	missing := filepath.Join(t.TempDir(), "no-such-directory")

	_, err := project.NewExecRunner().Run(context.Background(), missing, "rev-parse", "--verify", "--quiet", "HEAD")
	var ge *project.GitError
	if !errors.As(err, &ge) {
		t.Fatalf("Run in a directory that does not exist = %v, want a *project.GitError", err)
	}
	if ge.TimedOut || ge.Canceled || ge.Output() != "" {
		t.Fatalf("a git that cannot start now reports TimedOut=%v Canceled=%v Output=%q; "+
			"head()'s tolerance is reasoned from all three being empty", ge.TimedOut, ge.Canceled, ge.Output())
	}
	if ge.Exited {
		t.Error("GitError.Exited is true, but no git process was created")
	}
	// Review 2, I-1. The two facts must BOTH be false here, and this is the
	// one shape where they agree: nothing ran, so nothing answered.
	if ge.ExitedNonZero {
		t.Error("GitError.ExitedNonZero is true, but git never ran to answer anything")
	}
	if msg := ge.Error(); !strings.Contains(msg, "no git process was started") {
		t.Errorf("error = %q, want it to say no process was started; "+
			"otherwise the reader sees git's silence and reads it as git's answer", msg)
	}
}
