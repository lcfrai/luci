package project_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
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

	// Exercise the runner's own deadline already expiring before process
	// startup. A real git can finish inside 1 ms on a fast host, so that bound
	// does not prove a timeout. The separate running-process fixture verifies
	// cancellation after startup; here the expired bound must still be named.
	r := &project.ExecRunner{Timeout: time.Nanosecond}
	_, err := r.Run(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatal("Run with a 1ns bound returned no error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded, so no caller can classify it", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "timed out") {
		t.Errorf("error = %q, want it to say the git timed out", msg)
	}
	if !strings.Contains(msg, "1ns") {
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

// Review M-1. WHOSE deadline ended the git is the other half of ruling (a),
// and nothing read it: deleting GitError.Caller left the package green, and
// with it dies the one sentence that tells an operator the read was not slow —
// the request was already over before git started. A bound that ended a git
// this runner started is a different fact and reads differently.
func TestExecRunnerSaysWhoseDeadlineEndedTheGit(t *testing.T) {
	requireGit(t)

	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	gaveUp, cancelNow := context.WithCancel(context.Background())
	cancelNow()

	// A bound this runner would never reach, so the caller's context is the
	// only thing that can end these two gits.
	caller := &project.ExecRunner{Timeout: 42 * time.Second}
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want string
		// alsoWant is review M-5: naming the bound on EVERY timeout path is
		// ruling (a), and deleting ", inside the %s bound" from the caller's
		// message left both suites green. A cancellation is not a timeout and
		// names no bound, so it wants nothing here.
		alsoWant string
	}{
		{"the caller's deadline passed", expired, "the caller's deadline passed", "inside the 42s bound"},
		{"the caller gave up", gaveUp, "cancelled by the caller", ""},
	} {
		_, err := caller.Run(tc.ctx, t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
		var ge *project.GitError
		if !errors.As(err, &ge) {
			t.Fatalf("%s: error %v is not a *project.GitError", tc.name, err)
		}
		if !ge.Caller {
			t.Errorf("%s: GitError.Caller is false, so the message cannot say whose deadline it was", tc.name)
		}
		msg := ge.Error()
		if !strings.Contains(msg, tc.want) {
			t.Errorf("%s: error = %q, want %q in it", tc.name, msg, tc.want)
		}
		if tc.alsoWant != "" && !strings.Contains(msg, tc.alsoWant) {
			t.Errorf("%s: error = %q, want %q in it: a timeout that names no bound leaves the "+
				"reader nothing to argue with", tc.name, msg, tc.alsoWant)
		}
	}

	// Our own bound is not the caller's, and must not say it was.
	ours := &project.ExecRunner{Timeout: time.Nanosecond}
	_, err := ours.Run(context.Background(), t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
	var ge *project.GitError
	if !errors.As(err, &ge) {
		t.Fatalf("our own bound: error %v is not a *project.GitError", err)
	}
	if ge.Caller {
		t.Errorf("our own bound: GitError.Caller is true, but the caller handed in a context that never ends")
	}
	if msg := ge.Error(); !strings.Contains(msg, "timed out after the 1ns bound") {
		t.Errorf("our own bound: error = %q, want the bound this runner set named", msg)
	}
}

// Review M-4. The re-decided bounds were pinned only against each other —
// HostConfigTimeout is asserted to be greater than GitTimeout and to be the
// host runner's Timeout, both by reference to the constant itself — so 11 s
// passes every assertion in both packages while the reason the number exists
// evaporates. This pins it to what it is FOR.
func TestHostConfigTimeoutOutlastsAStallThatTheGitTimeoutAlreadyLostTo(t *testing.T) {
	// What the 2026-09-27 evidence shows: a host-git read that had 10 s and
	// did not finish, seconds away from another that did. A bound sized to
	// outlast that stall has to cover the stall AND the git's own work after
	// it, and the only measured number for either is GitTimeout: the stall
	// beat one of them, so the bound carries a second for the stall to grow
	// into and a third for the work. Anything less re-runs the create into
	// the failure this bound was separated from GitTimeout to escape.
	if want := 3 * project.GitTimeout; project.HostConfigTimeout < want {
		t.Errorf("HostConfigTimeout = %s, want at least %s (3 x GitTimeout = %s): a bound that only "+
			"just exceeds the one a stall already beat is not a margin, and F2 cost the phase 2 proof "+
			"its only end-to-end evidence that Resume forks a conversation",
			project.HostConfigTimeout, want, project.GitTimeout)
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
