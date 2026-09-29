package project

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Review 4, I-1. A git that exited on its own while a child it left behind
// still held its pipes: no context ended it, so os/exec started its WaitDelay
// timer the moment Process.Wait returned (os/exec/exec.go:991-993, go1.26.2)
// and gave up exactly gitWaitDelay later. Elapsed is measured across cmd.Run,
// so it was git's runtime PLUS that wait, and the message printed the sum as
// "git ran": 5.642s for a 3.642 s git, beside the 2 s drain it had just named.
// This is the decision and the arithmetic, with no git and no host timing.
func TestRecordCauseTakesTheDrainOutOfAGitNoContextEnded(t *testing.T) {
	const ran = 1986 * time.Millisecond
	start := time.Now().Add(-(ran + gitWaitDelay))
	ge := &GitError{Args: []string{"config", "--global", "--edit"}, Dir: `C:\tmp\p`,
		Err: exec.ErrWaitDelay, Timeout: HostConfigTimeout, Elapsed: ran + gitWaitDelay, Exited: true}
	ge.recordCause(context.Background(), context.Background(), start)

	if ge.TimedOut || ge.Canceled {
		t.Fatalf("TimedOut=%v Canceled=%v with no context done: git's own exit ended it", ge.TimedOut, ge.Canceled)
	}
	if ge.Drain != gitWaitDelay {
		t.Errorf("Drain = %s, want the %s os/exec waited after git exited", ge.Drain, gitWaitDelay)
	}
	if ge.Elapsed != ran {
		t.Errorf("Elapsed = %s, want git's own %s: the drain is still counted as git's runtime", ge.Elapsed, ran)
	}
	// The arm names the drain once, and it is the struct's number, not a
	// constant the printf carries on its own (review 4, I-2's MY-1).
	want := fmt.Sprintf("its pipes outlived it (git ran 1.986s, then %s waiting for its pipes): ", gitWaitDelay)
	if msg := ge.Error(); !strings.Contains(msg, want) {
		t.Errorf("error = %q, want %q", msg, want)
	}

	// os/exec never gives up before it was told to, so Elapsed is never below
	// the drain on this path; if a clock ever says otherwise, the runtime is
	// none rather than negative.
	short := &GitError{Err: exec.ErrWaitDelay, Timeout: GitTimeout, Elapsed: gitWaitDelay / 2, Exited: true}
	short.recordCause(context.Background(), context.Background(), time.Now())
	if short.Elapsed != 0 || short.Drain != gitWaitDelay {
		t.Errorf("Elapsed=%s Drain=%s from %s, want 0 and %s", short.Elapsed, short.Drain, gitWaitDelay/2, gitWaitDelay)
	}
}

// The case is as narrow as the arm it feeds: a git that failed on its own
// keeps its runtime whole, and a context that ended the git still wins, so
// nothing a caller classifies with errors.Is changes.
func TestRecordCauseLeavesEveryOtherEndAsItWas(t *testing.T) {
	failed := &GitError{Err: errors.New("exit status 128"), Timeout: GitTimeout,
		Elapsed: 40 * time.Millisecond, Exited: true, ExitedNonZero: true}
	failed.recordCause(context.Background(), context.Background(), time.Now())
	if failed.Drain != 0 || failed.Elapsed != 40*time.Millisecond || failed.TimedOut || failed.Canceled {
		t.Errorf("a git that failed on its own became Elapsed=%s Drain=%s TimedOut=%v Canceled=%v",
			failed.Elapsed, failed.Drain, failed.TimedOut, failed.Canceled)
	}

	parent, cancel := context.WithCancel(context.Background())
	cancel()
	superseded := &GitError{Err: exec.ErrWaitDelay, Timeout: GitTimeout, Elapsed: 3 * time.Second, Exited: true}
	superseded.recordCause(parent, parent, time.Now())
	if !superseded.Canceled || !errors.Is(superseded, context.Canceled) || superseded.Drain != 0 {
		t.Errorf("a caller that gave up read Canceled=%v Drain=%s: the caller's context is asked first",
			superseded.Canceled, superseded.Drain)
	}
}
