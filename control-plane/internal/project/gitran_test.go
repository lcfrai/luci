package project_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// Review 2, I-1. "A git process RAN" and "git exited NON-ZERO" are two facts,
// and GitError.Exited was derived from the second: an *exec.ExitError, which
// os/exec returns only when the process ran AND answered non-zero. A git that
// ran and exited ZERO is not one — so the message said `no git process was
// started` about a git that had run its whole bound, pointing the operator at
// a process create that never happened (AV, a missing binary, a dead
// directory) when git had in fact run. F2 exists BECAUSE the error did not say
// why; a false cause in the same sentence is the same defect.
//
// Staged the way the reviewer staged it, and with the same mechanism the
// killed-path test already uses on this host: git runs its editor through its
// own sh, so an editor that BACKGROUNDS a sleeper and returns 0 leaves git
// exiting 0 with a child still holding its pipes. cmd.Wait then has no EOF to
// read, cmd.WaitDelay closes the pipes, and os/exec returns ErrWaitDelay —
// a process that ran, with no *exec.ExitError anywhere.
func TestExecRunnerSaysGitRanWhenItExitedZeroWithItsPipesStillHeld(t *testing.T) {
	requireGit(t)
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "editor-ran")
	// The child holds git's pipes for a minute, so the only thing that can free
	// cmd.Wait inside holdLimit is cmd.WaitDelay. Measured here: 15.3 s with
	// the delay in place (13.3 s of it git's own start and edit on this host,
	// the reviewer measured 14.6 s for the same fixture), against the 60 s-plus
	// a run that waits the child out takes. The limit sits between the two
	// populations and nearer the one it must not clip. Measured twice here,
	// 15.3 s and 19.4 s, both with the delay in place.
	const holdLimit = 30 * time.Second

	r := &project.ExecRunner{Timeout: 30 * time.Second, Env: []string{
		// git hands this to its own sh: the marker proves the editor ran, `&`
		// backgrounds the sleeper with git's pipes in hand, and sh then exits 0
		// so GIT ITSELF exits 0. The trailing # drops the path git appends.
		"GIT_EDITOR=: > " + filepath.ToSlash(marker) + "; sleep 60 & #",
		// Edited config is this test's own file; ~/.gitconfig is never opened.
		"GIT_CONFIG_GLOBAL=" + filepath.Join(tmp, "gitconfig"),
	}}

	start := time.Now()
	_, err := r.Run(context.Background(), tmp, "config", "--global", "--edit")
	elapsed := time.Since(start)

	if _, statErr := os.Stat(marker); statErr != nil {
		t.Skipf("git never reached its editor on this host (%s, %v), so no child held its pipes", elapsed, err)
	}
	// This assertion runs whether or not the fixture staged its error: if
	// cmd.WaitDelay is gone, Run waits the child out instead and there is no
	// skip to hide behind.
	if elapsed >= holdLimit {
		t.Errorf("Run returned after %s for a git that exited 0 with a child holding its pipes: "+
			"cmd.WaitDelay is what keeps a bound a promise this runner can keep", elapsed)
	}
	var ge *project.GitError
	if !errors.As(err, &ge) {
		t.Skipf("the editor's child did not hold git's pipes on this host (%s, %v)", elapsed, err)
	}

	t.Logf("Run returned after %s: %v", elapsed, err)
	if ge.TimedOut || ge.Canceled {
		t.Fatalf("TimedOut=%v Canceled=%v: this git was ended by its pipes, not by a deadline; "+
			"the shape under test is gone", ge.TimedOut, ge.Canceled)
	}
	if !ge.Exited {
		t.Errorf("Exited is false for a git that ran %s and exited 0", elapsed)
	}
	// And the narrow fact stays narrow: git exited ZERO here, so it answered
	// nothing, and head()'s tolerance must not read this silence as an answer.
	if ge.ExitedNonZero {
		t.Errorf("ExitedNonZero is true for a git that exited 0: %v", err)
	}
	if msg := ge.Error(); strings.Contains(msg, "no git process was started") {
		t.Errorf("error = %q, but git ran for %s: the operator is sent to look at a process "+
			"create that did happen", msg, elapsed)
	}
}
