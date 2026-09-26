package gitx_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// The Exec tests run against a helper process, the standard os/exec pattern: the test binary
// re-executes itself as a fake git when GITX_FAKE_GIT=1, and prints its environment or argv,
// sleeps, echoes stdin, or exits with a chosen code. That is the only way to observe the
// child's environment without trusting the code under test. TestMain sets GITX_FAKE_GIT in
// this process, so every child inherits it (H3's config read carries no Call.Env); only a
// child whose executable is this binary acts on it.
func TestMain(m *testing.M) {
	if os.Getenv("GITX_FAKE_GIT") == "1" {
		os.Exit(fakeGit(os.Args[1:]))
	}
	os.Setenv("GITX_FAKE_GIT", "1")
	os.Exit(m.Run())
}

func fakeGit(argv []string) int {
	i := 0
	for i < len(argv) {
		switch {
		case argv[i] == "-c" && i+1 < len(argv):
			i += 2
			continue
		case argv[i] == "--literal-pathspecs":
			i++
			continue
		}
		break
	}
	if strings.Join(argv[i:], " ") == "config --local --no-includes --list -z" {
		fmt.Print(os.Getenv("GITX_FAKE_CONFIG"))
		return 0
	}
	code, _ := strconv.Atoi(os.Getenv("GITX_FAKE_CODE"))
	switch os.Getenv("GITX_FAKE_MODE") {
	case "env":
		for _, kv := range os.Environ() {
			fmt.Println(kv)
		}
	case "argv":
		for _, a := range argv {
			fmt.Println(a)
		}
	case "warn":
		fmt.Fprintln(os.Stderr, "warning: x")
		fmt.Println("ok")
	case "stdin":
		io.Copy(os.Stdout, os.Stdin)
	case "stderr":
		fmt.Fprint(os.Stderr, os.Getenv("GITX_FAKE_STDERR"))
	case "sleep":
		d, _ := time.ParseDuration(os.Getenv("GITX_FAKE_SLEEP"))
		time.Sleep(d)
		fmt.Println("done")
	case "loader-once": // Windows' loader failure on the first start only
		f := os.Getenv("GITX_FAKE_COUNTER")
		if _, err := os.Stat(f); err != nil {
			os.WriteFile(f, nil, 0o600)
			return signed(statusDLLInitFailed)
		}
		fmt.Println("ok")
	}
	return code
}

const statusDLLInitFailed uint32 = 0xC0000142

// signed is an NTSTATUS as the signed 32-bit number os.Exit and cmd /c exit take.
func signed(u uint32) int { return int(int32(u)) }

// fake is an Exec whose git is this test binary, with OwnedRoot a temp directory, so a Scope
// None call there runs no H3 check.
func fake(t *testing.T) (*gitx.Exec, string) {
	t.Helper()
	x := gitxtest.NewExec(t)
	x.Git = os.Args[0]
	x.Hardening.OwnedRoot = t.TempDir()
	return x, x.Hardening.OwnedRoot
}

func fakeCall(dir string, class gitx.Class, env map[string]string) gitx.Call {
	c := gitx.Call{Dir: dir, Args: []string{"status"}, Class: class, Scope: gitx.None, Env: env}
	if class == gitx.Remote {
		c.RemoteURL = "https://example.invalid/r.git"
	}
	return c
}

var classes = []gitx.Class{gitx.Read, gitx.LocalWrite, gitx.Remote}

func childEnv(t *testing.T, x *gitx.Exec, dir string, class gitx.Class, env map[string]string) map[string]string {
	t.Helper()
	if env == nil {
		env = map[string]string{}
	}
	env["GITX_FAKE_MODE"] = "env"
	res, err := x.Run(context.Background(), fakeCall(dir, class, env))
	if err != nil {
		t.Fatalf("class %d: %v", class, err)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(res.Stdout), "\r\n", "\n"), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && k != "" {
			got[strings.ToUpper(k)] = v
		}
	}
	return got
}

func TestExecReturnsStdoutOnlyOnSuccess(t *testing.T) {
	x, dir := fake(t)
	res, err := x.Run(context.Background(), fakeCall(dir, gitx.Read, map[string]string{"GITX_FAKE_MODE": "warn"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.ReplaceAll(string(res.Stdout), "\r\n", "\n"); got != "ok\n" {
		t.Fatalf("Stdout = %q, want %q (stderr must not reach it)", got, "ok\n")
	}
	if !strings.Contains(string(res.Stderr), "warning: x") {
		t.Fatalf("Stderr = %q, want git's warning kept apart", res.Stderr)
	}
}

func TestExecReadClassSetsNoOptionalLocks(t *testing.T) {
	x, dir := fake(t)
	if v := childEnv(t, x, dir, gitx.Read, nil)["GIT_OPTIONAL_LOCKS"]; v != "0" {
		t.Fatalf("Read child GIT_OPTIONAL_LOCKS = %q, want 0", v)
	}
}

func TestExecLocalAndRemoteTakeTheirLocks(t *testing.T) {
	x, dir := fake(t)
	t.Setenv("GIT_OPTIONAL_LOCKS", "0") // an inherited value must not reach a write either
	for _, c := range []gitx.Class{gitx.LocalWrite, gitx.Remote} {
		if v, ok := childEnv(t, x, dir, c, nil)["GIT_OPTIONAL_LOCKS"]; ok {
			t.Errorf("class %d child sees GIT_OPTIONAL_LOCKS=%q; a write must take its locks", c, v)
		}
	}
}

func TestExecIsHeadlessForEveryClass(t *testing.T) {
	x, dir := fake(t)
	want := map[string]string{"GIT_TERMINAL_PROMPT": "0", "GCM_INTERACTIVE": "never", "GIT_PAGER": "cat", "GIT_EDITOR": "false"}
	for _, c := range classes {
		got := childEnv(t, x, dir, c, nil)
		for k, v := range want {
			if got[k] != v {
				t.Errorf("class %d: %s = %q, want %q", c, k, got[k], v)
			}
		}
	}
}

func TestExecCallEnvWins(t *testing.T) {
	x, dir := fake(t)
	if v := childEnv(t, x, dir, gitx.Read, map[string]string{"GIT_PAGER": "less"})["GIT_PAGER"]; v != "less" {
		t.Fatalf("GIT_PAGER = %q, want the call's less", v)
	}
}

func exitCall(dir string, code int, okExit ...int) gitx.Call {
	c := fakeCall(dir, gitx.LocalWrite, map[string]string{"GITX_FAKE_CODE": strconv.Itoa(code)})
	c.OKExit = okExit
	return c
}

func TestExecOKExitIsAnAnswer(t *testing.T) {
	x, dir := fake(t)
	res, err := x.Run(context.Background(), exitCall(dir, 1, 1))
	if err != nil || res.ExitCode != 1 {
		t.Fatalf("exit 1 with OKExit [1] = (%+v, %v), want ExitCode 1 and no error", res, err)
	}
	_, err = x.Run(context.Background(), exitCall(dir, 2, 1))
	if ge, ok := gitx.AsError(err); !ok || ge.ExitCode != 2 {
		t.Fatalf("exit 2 with OKExit [1] = %v, want *Error with ExitCode 2", err)
	}
}

func TestExecTimeoutIsPerClass(t *testing.T) {
	x, dir := fake(t)
	x.Timeouts = map[gitx.Class]time.Duration{gitx.Read: 50 * time.Millisecond}
	sleep := map[string]string{"GITX_FAKE_MODE": "sleep", "GITX_FAKE_SLEEP": "1s"}
	_, err := x.Run(context.Background(), fakeCall(dir, gitx.Read, sleep))
	if ge, ok := gitx.AsError(err); !ok || !ge.TimedOut || ge.ExitCode != -1 {
		t.Fatalf("Read past its 50ms = %v, want *Error{TimedOut, ExitCode -1}", err)
	}
	if _, err := x.Run(context.Background(), fakeCall(dir, gitx.LocalWrite, sleep)); err != nil {
		t.Fatalf("the same call as LocalWrite = %v, want its own (2 min) timeout", err)
	}
	// Carried t01-D16: Windows kills with TerminateProcess(h, 1), so a runner that reads the
	// exit code before the context would return this timeout as OKExit's answer 1.
	c := fakeCall(dir, gitx.Read, sleep)
	c.OKExit = []int{1}
	res, err := x.Run(context.Background(), c)
	if ge, ok := gitx.AsError(err); !ok || !ge.TimedOut || ge.ExitCode != -1 {
		t.Fatalf("timed-out call with OKExit [1] = (%+v, %v), want *Error{TimedOut, ExitCode -1}", res, err)
	}
}

func TestExecPassesStdin(t *testing.T) {
	x, dir := fake(t)
	c := fakeCall(dir, gitx.LocalWrite, map[string]string{"GITX_FAKE_MODE": "stdin"})
	c.Stdin = []byte("subject\n\nbody\n")
	res, err := x.Run(context.Background(), c)
	if err != nil || string(res.Stdout) != string(c.Stdin) {
		t.Fatalf("stdin echo = (%q, %v), want %q", res.Stdout, err, c.Stdin)
	}
}

func TestErrorNeverContainsEnv(t *testing.T) {
	x, dir := fake(t)
	c := exitCall(dir, 128)
	c.Env["X"] = "s3cr3t"
	c.Env["GITX_FAKE_MODE"] = "stderr"
	c.Env["GITX_FAKE_STDERR"] = "fatal: no\n"
	_, err := x.Run(context.Background(), c)
	ge, ok := gitx.AsError(err)
	if !ok {
		t.Fatalf("err = %v, want *Error", err)
	}
	if strings.Contains(ge.Error(), "s3cr3t") || strings.Contains(ge.Sentence(), "s3cr3t") {
		t.Fatalf("the call's Env reached the error: %q / %q", ge.Error(), ge.Sentence())
	}
}

func TestSentencePrefersTheLastFatalLine(t *testing.T) {
	e := &gitx.Error{Stderr: "hint: a\nfatal: b\nhint: c", ExitCode: 128}
	if got := e.Sentence(); got != "fatal: b" {
		t.Fatalf("Sentence = %q, want %q", got, "fatal: b")
	}
}

func TestSentenceIsCappedOnARuneBoundary(t *testing.T) {
	e := &gitx.Error{Stderr: strings.Repeat("é", 1500), ExitCode: 1} // 3000 bytes
	got := e.Sentence()
	if len(got) > gitx.MaxSentence || !utf8Valid(got) || len(got) < gitx.MaxSentence-1 {
		t.Fatalf("Sentence is %d bytes (valid UTF-8 %v), want <= %d on a rune boundary", len(got), utf8Valid(got), gitx.MaxSentence)
	}
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "�") == s }

// TestExecNamesAGitThatNeverStarted is carried t01-D14: exit 3221225794 (0xC0000142,
// STATUS_DLL_INIT_FAILED) means Windows' loader gave up before git's main, so it is never git's
// answer. It is retried once (no git code ran) and then named NotStarted. Every other code,
// including a git that started and crashed either side of it, passes through unretried.
func TestExecNamesAGitThatNeverStarted(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("0xC0000142 is a Windows loader status; no other OS returns it as an exit code")
	}
	x, dir := fake(t)
	spawns := gitxtest.Spy(t, x)
	n := func() int { k := len(*spawns); *spawns = (*spawns)[:0]; return k }

	c := exitCall(dir, signed(statusDLLInitFailed), 1)
	_, err := x.Run(context.Background(), c)
	ge, ok := gitx.AsError(err)
	if !ok || !ge.NotStarted || ge.ExitCode != -1 || !strings.Contains(ge.Error(), "could not start") {
		t.Fatalf("0xC0000142 = %v, want *Error{NotStarted, ExitCode -1} saying it could not start", err)
	}
	if k := n(); k != 2 {
		t.Fatalf("0xC0000142 started %d processes, want 2 (one retry)", k)
	}

	c = fakeCall(dir, gitx.LocalWrite, map[string]string{"GITX_FAKE_MODE": "loader-once",
		"GITX_FAKE_COUNTER": filepath.Join(t.TempDir(), "started")})
	if res, err := x.Run(context.Background(), c); err != nil || !strings.HasPrefix(string(res.Stdout), "ok") {
		t.Fatalf("a loader failure then a start = (%q, %v), want the retry's ok", res.Stdout, err)
	}
	if k := n(); k != 2 {
		t.Fatalf("loader-once started %d processes, want 2", k)
	}

	for _, code := range []uint32{0, 1, 128, 255, 0xC0000005, 0xC0000135, 0xC0000409} {
		_, err := x.Run(context.Background(), exitCall(dir, signed(code)))
		if k := n(); k != 1 {
			t.Errorf("exit %#x started %d processes, want 1: a git that ran is never retried", code, k)
		}
		if code == 0 {
			if err != nil {
				t.Errorf("exit 0 = %v", err)
			}
			continue
		}
		ge, ok := gitx.AsError(err)
		if !ok || ge.NotStarted || uint32(ge.ExitCode) != code {
			t.Errorf("exit %#x = %v, want *Error with that ExitCode and NotStarted false", code, err)
		}
	}
}

// TestExecCancelIsNotAnExitCode: a request the human abandoned kills git; on Windows that kill
// is exit 1, so the call reads as cancelled, never as OKExit's answer (task 02 addition).
func TestExecCancelIsNotAnExitCode(t *testing.T) {
	x, dir := fake(t)
	c := fakeCall(dir, gitx.Read, map[string]string{"GITX_FAKE_MODE": "sleep", "GITX_FAKE_SLEEP": "5s"})
	c.OKExit = []int{1}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)
	_, err := x.Run(ctx, c)
	ge, ok := gitx.AsError(err)
	if !ok || ge.TimedOut || ge.ExitCode != -1 || !errors.Is(err, context.Canceled) || !strings.Contains(ge.Error(), "cancelled") {
		t.Fatalf("cancelled call = %v, want *Error{ExitCode -1, TimedOut false} wrapping context.Canceled, saying cancelled", err)
	}
}

// TestExecNamesAMissingGit: an executable that does not exist is NotStarted, never an exit code.
func TestExecNamesAMissingGit(t *testing.T) {
	x, dir := fake(t)
	x.Git = filepath.Join(dir, "no-such-git")
	_, err := x.Run(context.Background(), fakeCall(dir, gitx.Read, nil))
	if ge, ok := gitx.AsError(err); !ok || !ge.NotStarted || ge.ExitCode != -1 {
		t.Fatalf("missing git = %v, want *Error{NotStarted, ExitCode -1}", err)
	}
	if errors.Is(err, gitx.ErrCallShape) {
		t.Fatalf("a missing git is not a call shape")
	}
}
