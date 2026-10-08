//go:build linux || darwin || windows

package statelease

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Use real processes: an in-memory mutex, PID file or unlink-on-close lock can
// appear sound in one process while allowing stale writers after a crash.
func TestLeaseProcessContentionCrashAndReopen(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprintf("crash=%t", crash), func(t *testing.T) {
			dir := processLeaseDirectory(t)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			preserved := filepath.Join(dir, "identity.json")
			if err := os.WriteFile(preserved, []byte("retain existing identity\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := leaseProcessCommand(t, "hold", dir)
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var diagnostics bytes.Buffer
			cmd.Stderr = &diagnostics
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			t.Cleanup(func() {
				input.Close()
				if !waited {
					cmd.Process.Kill()
					cmd.Wait()
				}
			})
			ready := make(chan string, 1)
			go func() {
				line, _ := bufio.NewReader(output).ReadString('\n')
				ready <- line
			}()
			select {
			case line := <-ready:
				if line != "HELD\n" {
					t.Fatalf("holder did not acquire lease: %q", line)
				}
			case <-time.After(15 * time.Second):
				t.Fatal("holder did not become ready")
			}
			before, err := os.Stat(filepath.Join(dir, FileName))
			if err != nil {
				t.Fatal(err)
			}
			contender := leaseProcessCommand(t, "contend", dir)
			if output, err := contender.CombinedOutput(); err != nil || !strings.Contains(string(output), "BLOCKED\n") {
				t.Fatalf("second process was not clearly refused: %v\n%s", err, output)
			}
			// An isolated root is independent while the original remains held.
			isolated := leaseProcessCommand(t, "once", processLeaseDirectory(t))
			if output, err := isolated.CombinedOutput(); err != nil || !strings.Contains(string(output), "ACQUIRED\n") {
				t.Fatalf("separate root was blocked: %v\n%s", err, output)
			}
			if crash {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				input.Close()
			}
			waitErr := cmd.Wait()
			waited = true
			if !crash && waitErr != nil {
				t.Fatalf("normal close failed: %v\n%s", waitErr, diagnostics.String())
			}
			input.Close()
			after, err := os.Stat(filepath.Join(dir, FileName))
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("lease was deleted/replaced on process exit: %v", err)
			}
			metadata, err := os.ReadFile(filepath.Join(dir, FileName))
			if err != nil || string(metadata) != `{"version":1,"releaseVersion":"holder-release"}` {
				t.Fatalf("refused contender changed lock metadata: %s (%v)", metadata, err)
			}
			next := leaseProcessCommand(t, "once", dir)
			if output, err := next.CombinedOutput(); err != nil || !strings.Contains(string(output), "ACQUIRED\n") {
				t.Fatalf("process exit did not release ownership: %v\n%s", err, output)
			}
			last, err := os.Stat(filepath.Join(dir, FileName))
			if err != nil || !os.SameFile(before, last) {
				t.Fatalf("restart replaced persistent lock identity: %v", err)
			}
			data, err := os.ReadFile(preserved)
			if err != nil || string(data) != "retain existing identity\n" {
				t.Fatalf("existing state changed across contention/restart: %q (%v)", data, err)
			}
		})
	}
}

func TestLeaseProcessHelper(t *testing.T) {
	mode := os.Getenv("LUCI_TEST_STATE_LEASE_PROCESS")
	if mode == "" {
		return
	}
	dir := os.Getenv("LUCI_TEST_STATE_LEASE_DIRECTORY")
	release := "holder-release"
	if mode == "contend" {
		release = "must-not-overwrite"
	}
	lease, err := Acquire(dir, release)
	if mode == "contend" {
		if lease != nil {
			lease.Close()
		}
		if !errors.Is(err, ErrInUse) {
			t.Fatalf("expected state-in-use error, got %v", err)
		}
		fmt.Println("BLOCKED")
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if mode == "once" {
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
		fmt.Println("ACQUIRED")
		return
	}
	fmt.Println("HELD")
	io.Copy(io.Discard, os.Stdin)
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func leaseProcessCommand(t *testing.T, mode, dir string) *exec.Cmd {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestLeaseProcessHelper$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "LUCI_TEST_STATE_LEASE_PROCESS="+mode, "LUCI_TEST_STATE_LEASE_DIRECTORY="+dir)
	return cmd
}

func processLeaseDirectory(t *testing.T) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Let Acquire create this directory with the OS-specific private policy.
	return filepath.Join(parent, "state")
}
