package project_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// Review 3, m-2 and m-1. The only pin on the new derivation's MESSAGE lived
// behind two t.Skipf calls: a host without sh, without sleep, or one that
// ignores GIT_EDITOR runs neither assertion, and the mutation survives there.
// Every fact the message states about a git that RAN is a fact about the
// struct, so it is pinned here from a GitError built by hand — no git, no
// host timing, no fixture that can fail to stage.
//
// Review 3, m-1 is the second half: the shape this round exists to fix — a
// git that ran, was ended by neither deadline nor cancellation, and whose
// pipes os/exec gave up on — took no arm of the switch at all, so what
// reached the operator was `exec: WaitDelay expired before I/O complete` with
// no runtime in it. The mechanism was named; the duration was not.
func TestGitErrorMessageNamesTheRuntimeOfAGitWhosePipesOutlivedIt(t *testing.T) {
	ge := &project.GitError{
		Args:    []string{"config", "--global", "--edit"},
		Dir:     `C:\tmp\p`,
		Err:     exec.ErrWaitDelay,
		Timeout: 30 * time.Second,
		Elapsed: 3986 * time.Millisecond,
		// The fact the round split out: a process RAN. It exited 0, so git
		// itself answered nothing and ExitedNonZero stays false.
		Exited: true,
	}

	msg := ge.Error()
	if strings.Contains(msg, "no git process was started") {
		t.Errorf("error = %q, but this git ran 3.986s: the operator is sent to look at a "+
			"process create that did happen", msg)
	}
	if !strings.Contains(msg, "git ran 3.986s") {
		t.Errorf("error = %q, want git's own runtime in it: an operator deciding whether to "+
			"argue with the bound cannot read a duration that is not there", msg)
	}
	if !strings.Contains(msg, "WaitDelay expired") {
		t.Errorf("error = %q, want os/exec's own sentence kept", msg)
	}
}

// The other two shapes, same way: the message that must still say a process
// never started, and the killed git whose runtime and drain are two numbers.
func TestGitErrorMessageTellsAStartThatWasRefusedFromAGitThatRan(t *testing.T) {
	never := &project.GitError{
		Args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, Dir: `C:\gone`,
		Err: exec.ErrNotFound, Timeout: 10 * time.Second, Elapsed: 2 * time.Millisecond,
	}
	if msg := never.Error(); !strings.Contains(msg, "no git process was started") {
		t.Errorf("error = %q for a start that was refused, want it said in those words", msg)
	}

	killed := &project.GitError{
		Args: []string{"config", "--global", "--edit"}, Dir: `C:\tmp\p`,
		Err: exec.ErrWaitDelay, Timeout: 6 * time.Second,
		Elapsed: 5872 * time.Millisecond, Drain: 2167 * time.Millisecond,
		// A killed git on Windows exits 1, so os/exec builds an
		// *exec.ExitError from Process.Wait and Cmd.Wait prefers it: this
		// shape carries BOTH facts, which is review 3's I-1.
		Exited: true, ExitedNonZero: true, TimedOut: true,
	}
	msg := killed.Error()
	if !strings.Contains(msg, "timed out after the 6s bound (git ran 5.872s, then 2.167s waiting for its pipes)") {
		t.Errorf("error = %q, want the bound, git's own runtime and the drain as three separate numbers", msg)
	}
	if strings.Contains(msg, "no git process was started") {
		t.Errorf("error = %q, but this git ran 5.872s before the bound killed it", msg)
	}
}
