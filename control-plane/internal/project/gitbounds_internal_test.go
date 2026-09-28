package project

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Review 2, m-1. gitWaitDelay was pinned only to itself: the killed-path test
// measured the kill against bound + gitWaitDelay + slack, so the constant
// appeared on both sides of its own assertion and 2 s → 30 s survived the whole
// package, exit 0. It is not a free number. Every bound this runner promises —
// GitTimeout behind the polled rail, HostConfigTimeout inside a create — is
// really that bound PLUS this delay, because Wait may spend it on pipes a
// killed git's children still hold. A handler told 10 s and held 40 s has been
// given a bound that is not a promise, which is the whole of F2.
//
// So it is pinned to what it is FOR: a drain, a small share of the smallest
// bound it is added to, and never a second bound in its own right.
func TestGitWaitDelayIsASmallShareOfTheSmallestBoundItIsAddedTo(t *testing.T) {
	if gitWaitDelay <= 0 {
		t.Fatalf("gitWaitDelay = %s: os/exec reads zero as WAIT FOREVER, so a kill that "+
			"is refused holds the handler open — the failure the constant exists to prevent", gitWaitDelay)
	}
	if share := GitTimeout / 4; gitWaitDelay > share {
		t.Errorf("gitWaitDelay = %s, more than the %s a drain may add to the %s bound it sits on: "+
			"a poll promised %s would return after %s", gitWaitDelay, share, GitTimeout, GitTimeout, GitTimeout+gitWaitDelay)
	}
}

// Review 2, m-3. M-1's split had exactly one assertion and it lived inside the
// killed-path fixture, which SKIPS on a host where git never reached its
// editor — so on such a run nothing pinned it at all. The split needs no git:
// it is arithmetic over a deadline, and this is that arithmetic.
func TestSplitAtDeadlineMovesTheDrainOutOfElapsed(t *testing.T) {
	const bound, drain = 6 * time.Second, 2 * time.Second
	start := time.Now().Add(-(bound + drain))
	ctx, cancel := context.WithDeadline(context.Background(), start.Add(bound))
	defer cancel()

	ge := &GitError{Timeout: bound, Elapsed: bound + drain, Exited: true}
	ge.splitAtDeadline(ctx, start)
	if ge.Elapsed > bound {
		t.Errorf("Elapsed = %s under a %s bound: the number beside the bound contradicts it", ge.Elapsed, bound)
	}
	if ge.Drain < drain-10*time.Millisecond || ge.Drain > drain+10*time.Millisecond {
		t.Errorf("Drain = %s, want the %s that was spent after the deadline", ge.Drain, drain)
	}
	ge.TimedOut = true
	if msg := ge.Error(); !strings.Contains(msg, "git ran 6s, then 2s waiting for its pipes") {
		t.Errorf("error = %q, want git's own runtime and the drain named separately", msg)
	}

	// Scheduling noise is not a kill that was refused, and naming it would be a
	// false signal: below the threshold the split does not happen at all.
	quiet := &GitError{Timeout: bound, Elapsed: bound + time.Millisecond, Exited: true}
	quiet.splitAtDeadline(ctx, start)
	if quiet.Drain != 0 {
		t.Errorf("Drain = %s for a millisecond of noise, want none", quiet.Drain)
	}

	// And a git that never started has nothing to split: its Elapsed is the
	// time os/exec spent failing to create a process, not a git's runtime.
	never := &GitError{Timeout: bound, Elapsed: bound + drain}
	never.splitAtDeadline(ctx, start)
	if never.Drain != 0 || never.Elapsed != bound+drain {
		t.Errorf("a git that never started split into Elapsed=%s Drain=%s", never.Elapsed, never.Drain)
	}
}
