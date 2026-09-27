package project_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// F2. On Windows the deadline kill is TerminateProcess(…, 1) and Go returns
// the killed process's OWN status, so a timeout arrived in the log as a bare
// `exit status 1: ` with nothing after the colon — indistinguishable from a
// git that answered "no". The 2026-09-27 quiet-host run lost o2.resume, the
// only end-to-end proof that Resume forks a real conversation, to one such
// line. The runner must say that it killed the git, and name the bound.
func TestExecRunnerNamesTheTimeoutAndTheBoundItExceeded(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()

	// 1 ms is shorter than a process start, so this takes whichever of the two
	// paths the machine gives it — the git that starts and is killed, or the
	// git that never starts because the deadline had already passed. Both are
	// the runner's own bound and both must read the same.
	r := &project.ExecRunner{Timeout: time.Millisecond}
	_, err := r.Run(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatal("Run with a 1ms bound returned no error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded, so no caller can classify it", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "timed out") {
		t.Errorf("error = %q, want it to say the git timed out", msg)
	}
	if !strings.Contains(msg, "1ms") {
		t.Errorf("error = %q, want the bound it exceeded in it", msg)
	}
}

// The other path: the context handed in has already expired, so no git is
// started at all. Three of the four F2 errors in the Go suite read "context
// deadline exceeded: " — that is this path, and it must name a bound too.
func TestExecRunnerNamesTheDeadlineWhenTheContextHasAlreadyExpired(t *testing.T) {
	requireGit(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := project.NewExecRunner().Run(ctx, t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatal("Run with an already-expired context returned no error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "timed out") {
		t.Errorf("error = %q, want it to say the git timed out", msg)
	}
}

// A caller that gives up is not a timeout and must not read as one: the rail
// cancels enumeration whenever a poll is superseded.
func TestExecRunnerSaysACancelledCallerIsNotATimeout(t *testing.T) {
	requireGit(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := project.NewExecRunner().Run(ctx, t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatal("Run with a cancelled context returned no error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %q does not wrap context.Canceled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q reads as a timeout, but the caller cancelled", err)
	}
}

// The other half of F2: a git that FAILED must still report git's own
// sentence, unchanged, and must not read as a timeout.
func TestExecRunnerKeepsGitsOwnFailureDistinctFromATimeout(t *testing.T) {
	requireGit(t)

	_, err := project.NewExecRunner().Run(context.Background(), t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatal("Run in a directory that is not a repository returned no error")
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		t.Errorf("error %q reads as a timeout or a cancellation, but git answered", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "not a git repository") {
		t.Errorf("error = %q, want git's own message in it", msg)
	}
	if strings.Contains(msg, "timed out") {
		t.Errorf("error = %q, want no timeout claim in it", msg)
	}
	var ge *project.GitError
	if !errors.As(err, &ge) {
		t.Fatalf("error %q is not a *project.GitError, so a caller cannot read its streams", err)
	}
	if ge.TimedOut {
		t.Error("GitError.TimedOut on a git that answered")
	}
	if !strings.Contains(ge.Stderr, "not a git repository") {
		t.Errorf("GitError.Stderr = %q, want git's own message", ge.Stderr)
	}
}
