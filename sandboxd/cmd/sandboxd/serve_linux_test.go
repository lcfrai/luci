package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const procProbeDirEnv = "SANDBOXD_TEST_PROC_PROBE_DIR"

// hasCapSysPtrace reports whether this process may read any process's
// /proc/<pid>/environ regardless of dumpability: CAP_SYS_PTRACE (bit 19).
func hasCapSysPtrace(t *testing.T) bool {
	t.Helper()
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatalf("read /proc/self/status: %v", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if hex, ok := strings.CutPrefix(line, "CapEff:"); ok {
			caps, err := strconv.ParseUint(strings.TrimSpace(hex), 16, 64)
			if err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			return caps&(1<<19) != 0
		}
	}
	t.Fatalf("no CapEff line in /proc/self/status")
	return false
}

// Stripping the token from the session's environment is not enough on its
// own: the session runs as sandboxd's user, and /proc/<pid>/environ keeps the
// environment sandboxd was started with whatever the Go runtime's copy says.
// sandboxd makes itself non-dumpable, so its session cannot read that file
// (review t18 M3; the review's remedy stopped at the environment).
//
// The token must be in a process's initial environment for the file to hold
// it, so the test runs itself again as that process: the helper starts a
// session that reads /proc/$PPID/environ, the helper's own.
func TestNewServer_sessionCannotReadTheTokenFromProc(t *testing.T) {
	if dir := os.Getenv(procProbeDirEnv); dir != "" {
		procProbeHelper(t, dir)
		return
	}
	if hasCapSysPtrace(t) {
		t.Skip("CAP_SYS_PTRACE reads a non-dumpable process's environ; the property needs a session without it")
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestNewServer_sessionCannotReadTheTokenFromProc$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), attachTokenEnv+"="+probeToken, procProbeDirEnv+"="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the helper failed: %v\n%s", err, out)
	}
	self, err := os.ReadFile(filepath.Join(dir, "self.txt"))
	if err != nil || !strings.Contains(string(self), probeToken) {
		t.Fatalf("the helper's /proc/self/environ lacks the token (%v), so the probe proves nothing", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "proc.txt"))
	if err != nil {
		t.Fatalf("read the session's probe: %v", err)
	}
	if strings.Contains(string(got), probeToken) {
		t.Errorf("the session read the attach token from sandboxd's /proc/<pid>/environ")
	}
	why, _ := os.ReadFile(filepath.Join(dir, "proc.err"))
	t.Logf("the session's read of /proc/$PPID/environ: %q", strings.TrimSpace(string(why)))
}

// procProbeHelper is the test's second run: a sandboxd whose initial
// environment holds the token.
func procProbeHelper(t *testing.T, dir string) {
	script := fmt.Sprintf("cat /proc/$PPID/environ > %[1]s/proc.txt 2> %[1]s/proc.err; printf done > %[1]s/done", dir)
	srv, err := newServer("/bin/sh", []string{"-c", script}, probeToken, false, "off")
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	closeServer(t, srv)
	waitForFile(t, filepath.Join(dir, "done"), "done")
	self, err := os.ReadFile("/proc/self/environ")
	if err != nil {
		t.Fatalf("read /proc/self/environ: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "self.txt"), self, 0o600); err != nil {
		t.Fatalf("write self.txt: %v", err)
	}
}
