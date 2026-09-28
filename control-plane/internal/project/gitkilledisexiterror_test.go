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
	// Nil streams are os.DevNull, so os/exec opens no pipe and starts no
	// copying goroutine: nothing here can reach the ErrWaitDelay path, and
	// the only error under test is the one Process.Wait reported.
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

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
	time.Sleep(90 * time.Second)
}
