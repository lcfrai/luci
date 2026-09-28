package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Review I-2. F2 IS the killed path: a git that STARTED, ran past its bound
// and was killed by TerminateProcess. Every timeout test in this package took
// the OTHER path — a bound of 1 ms or 1 ns, or a context already expired — so
// no process ever existed to kill, and the message was proved only where git
// never ran. The line that makes this path's bound real, cmd.WaitDelay, was
// left unpinned on the claim that nothing portable can stage a git that
// outlives its own kill. It can: git runs its editor through its own sh, so an
// editor that sleeps is a git that blocks with a CHILD holding git's pipes,
// and a killed git whose pipes are still held leaves cmd.Wait with nothing to
// read and no EOF — the bound becomes a hang. Measured on this host at 25 s
// against this 6 s bound with the line deleted.
//
// The child is what makes the mutant die, so this test proves the child ran
// rather than assuming it: the editor touches a marker before it sleeps. On a
// host where git had not started its editor by the time the bound fired — and
// a slow process start is F2's own condition, measured here at over 2.8 s —
// nothing held the pipes, there is no hang to catch, and this skips rather
// than passing on evidence it does not have.
//
// It is an internal test because the kill it measures is this package's own.
// Amended for review 2, m-1: it used to read gitWaitDelay into its limit rather
// than restate it, which is the right instinct for a restatement (section 3D)
// and the wrong one for an assertion — the constant then passed its own test at
// any value. gitbounds_internal_test.go pins the constant itself.
func TestExecRunnerKillsAGitThatWasStillRunning(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH; skipping the ExecRunner tests")
	}
	const bound = 6 * time.Second
	for attempt := 1; attempt <= 2; attempt++ {
		if staged := killedGitAttempt(t, bound, attempt); staged {
			return
		}
	}
	t.Skipf("git never reached its editor within %s on this host, so no child held its pipes "+
		"and there is no kill here to measure", bound)
}

// killedGitAttempt runs one blocking git under bound and reports whether the
// fixture staged — whether git's editor was running when the bound killed it.
// Every assertion of the killed path lives here, and none of them runs on an
// attempt that did not stage.
func killedGitAttempt(t *testing.T, bound time.Duration, attempt int) bool {
	t.Helper()
	tmp := t.TempDir()
	marker := filepath.Join(tmp, fmt.Sprintf("editor-ran-%d", attempt))
	r := &ExecRunner{Timeout: bound, Env: []string{
		// git hands this to its own sh, so `;` runs both halves: the marker
		// is the proof the editor started, and the sleep outlives the kill
		// with git's pipes in hand. The trailing # drops the path git
		// appends. Forward slashes because sh reads `\` as an escape.
		"GIT_EDITOR=: > " + filepath.ToSlash(marker) + "; sleep 25 #",
		// The config edited is a file in this test's own directory, so the
		// developer's ~/.gitconfig is never opened.
		"GIT_CONFIG_GLOBAL=" + filepath.Join(tmp, "gitconfig"),
	}}

	start := time.Now()
	_, err := r.Run(context.Background(), tmp, "config", "--global", "--edit")
	elapsed := time.Since(start)

	var ge *GitError
	if !errors.As(err, &ge) || !ge.TimedOut {
		t.Logf("attempt %d: git config --edit finished in %s without being killed: %v", attempt, elapsed, err)
		return false
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Logf("attempt %d: the %s bound killed git after %s, but its editor had not started",
			attempt, bound, elapsed)
		return false
	}
	t.Logf("attempt %d: Run returned after %s (bound %s, drain %s): %v", attempt, elapsed, bound, ge.Drain, err)

	if ge.Caller {
		t.Errorf("Caller is true, but the caller's context never ends: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded, so no caller can classify it", err)
	}
	if !ge.Exited {
		t.Errorf("Exited is false, but this git ran for %s before it was killed", ge.Elapsed)
	}
	// The promise F2 is about: the handler is free at the bound plus the
	// drain, even when git's pipes are held by a child the kill did not
	// reach. Without cmd.WaitDelay this same call measured 25 s.
	//
	// Review 2, m-1: the limit used to be written as bound + gitWaitDelay +
	// 4 s, so the constant sat on both sides of its own assertion and 2 s →
	// 30 s passed here. The number is unchanged and now belongs to the test: 6 s
	// is the 2 s drain plus scheduling slack on a host whose process starts F2
	// measured at over 2.8 s. Raising the constant past it is what this is for.
	if limit := bound + 6*time.Second; elapsed > limit {
		t.Errorf("Run returned after %s under a %s bound (limit %s): the bound is not a promise "+
			"this runner can keep", elapsed, bound, limit)
	}
	msg := ge.Error()
	if !strings.Contains(msg, "timed out after the 6s bound") {
		t.Errorf("error = %q, want the bound it exceeded named", msg)
	}
	if strings.Contains(msg, "no git process was started") {
		t.Errorf("error = %q, but this git ran %s before the bound killed it", msg, ge.Elapsed)
	}
	// Review M-1: the git cannot have run LONGER than the bound that killed
	// it. The drain belongs to Drain, and the message names it separately.
	if ge.Elapsed > bound {
		t.Errorf("Elapsed = %s under a %s bound: the number beside the bound contradicts it", ge.Elapsed, bound)
	}
	return true
}
