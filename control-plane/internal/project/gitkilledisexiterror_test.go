package project_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Review 3, I-1. The contract and the ledger said the old derivation reported
// "no git process was started" for "a git killed at its bound or one whose
// pipes outlived it". Only the second half is true, and this is why: Cmd.Wait
// PREFERS Process.Wait's error over the context's (os/exec/exec.go:931-941,
// go1.26.2), and a process killed at its deadline has a non-zero state — on
// Windows TerminateProcess(…, 1) — so os/exec builds an *exec.ExitError from
// it. An ordinary killed git was therefore ALWAYS reported correctly, under
// both derivations, and the reviewer measured exactly that twice.
//
// This pins the fact the corrected sentence rests on, with no git, no editor
// and no host timing: the test binary re-executes itself as a process that
// sleeps past a 1 s deadline. If a future Go returns the context's error here
// instead, this fails, and §2.11's sentence has to be re-derived rather than
// quietly becoming false again.
func TestAProcessKilledAtItsDeadlineCarriesItsOwnExitError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperSleepsPastItsDeadline", "-test.timeout=2m")
	cmd.Env = append(os.Environ(), helperSleepEnv+"=1")
	// Review 4, m-1. os/exec's own Cancel, with its answer kept. A host that
	// refuses the kill (`exec: canceling Cmd: TerminateProcess: Access is
	// denied.` is in the 2026-09-27 evidence) makes os/exec report ITS error
	// instead of the process's: that is a kill this host did not stage, not a
	// fact §2.11 got wrong. watchCtx calls this before it hands its result to
	// Wait, so reading both after Run is not a race.
	var killErr error
	killed := false
	cmd.Cancel = func() error {
		killErr = cmd.Process.Kill()
		killed = killErr == nil
		return killErr
	}
	// Nil streams are os.DevNull, so os/exec opens no pipe and starts no
	// copying goroutine: nothing here can reach the ErrWaitDelay path, and
	// the only error under test is the one Process.Wait reported.
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		t.Skipf("this host refused the kill at the deadline (%v), so after %s there is no killed "+
			"process to classify: os/exec reported its own error instead, %v", killErr, elapsed, err)
	}
	// And a helper that ended before its deadline was never killed: a fresh
	// process the host kills at start (0xc0000142) exits non-zero too, and
	// would pass the assertion below without a kill having been staged.
	if !killed {
		t.Fatalf("the helper ended on its own after %s (%v), before its 1 s deadline's kill: no kill "+
			"was staged, so nothing here says what a killed process carries", elapsed, err)
	}
	if err == nil {
		t.Fatalf("the helper returned nil after %s: it outlived its deadline unkilled, so there "+
			"is no kill here to classify", elapsed)
	}
	if cmd.ProcessState == nil {
		t.Fatalf("ProcessState is nil after %s (%v), so the start itself was refused and the "+
			"fixture staged nothing", elapsed, err)
	}
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		t.Fatalf("a process killed at its deadline gave %#v, not an *exec.ExitError: §2.11 says "+
			"an ordinary killed git was always reported correctly, and that rests on this", err)
	}
	t.Logf("killed after %s: ProcessState=%v, ExitError=%v", elapsed, cmd.ProcessState, exited)
}

const helperSleepEnv = "PANACEA_TEST_HELPER_SLEEP"

// TestHelperSleepsPastItsDeadline is not a test: it is the child process the
// test above kills. It does nothing at all unless that test started it.
func TestHelperSleepsPastItsDeadline(t *testing.T) {
	if os.Getenv(helperSleepEnv) == "" {
		t.Skip("helper process; run only by TestAProcessKilledAtItsDeadlineCarriesItsOwnExitError")
	}
	// Review 4, m-1: long enough to outlive its 1 s deadline many times over,
	// short enough that a host refusing the kill costs seconds, not 90.
	time.Sleep(15 * time.Second)
}
