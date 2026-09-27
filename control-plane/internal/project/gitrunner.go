package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitTimeout bounds one git invocation behind the POLLED rail. Enumeration
// runs behind an HTTP handler the rail polls every few seconds, so a git that
// blocks — a network remote, a credential prompt, an index.lock held by
// another process — must fail rather than hold a request open: the next poll
// asks again two seconds later and costs the user nothing.
//
// It is deliberately NOT the bound for a git on the create path. See
// HostConfigTimeout.
const GitTimeout = 10 * time.Second

// HostConfigTimeout bounds ONE git that reads the HOST's own config while a
// create is in flight (cmd/api's hostGit). It is one git's bound and nothing
// else: how long a READ of several gits may take is a different decision with
// a different reason, and cmd/api's hostConfigReadBudget makes it. Sizing a
// read from this number was review C-1 — four gits sharing 30 s give each 7.5 s,
// below the 10 s F2 proved too short.
//
// That git is not polled: it runs once, inside POST /v1/sessions, with the
// user waiting, and its failure destroys the whole create — the sandbox, the
// worktree and the intent. On 2026-09-27 one such read exceeded 10 s on a
// quiet host and cost the phase 2 proof o2.resume, its only end-to-end proof
// that Resume forks a real conversation. `git config --get` does no I/O worth
// 10 s; a read that slow is a process that has not started yet. WHY it has not
// is UNVERIFIED — on-access virus scanning of a fresh git.exe is the
// hypothesis, and nothing in the phase 2 evidence measures it. What the
// evidence does show is a host-git that exceeded 10 s and a second one that
// did not, seconds apart, so the stall is transient. So this bound is sized to
// outlast a transient stall rather than to keep a poll cheap.
const HostConfigTimeout = 30 * time.Second

// gitWaitDelay is how long cmd.Wait waits after the deadline kill before it
// gives up on git's pipes. Without it the bound is a promise this runner
// cannot keep: the 2026-09-27 evidence includes `exec: canceling Cmd:
// TerminateProcess: Access is denied.`, and a kill that fails leaves Wait
// blocked on a stdout/stderr pipe that no one will ever close, holding the
// handler open forever — the exact failure GitTimeout exists to prevent.
const gitWaitDelay = 2 * time.Second

// GitError is what a git that did not succeed returns. It exists because the
// flattened string it replaced could not tell a git that FAILED from a git
// this runner KILLED: on Windows the deadline kill is TerminateProcess(…, 1)
// and Go returns the killed process's own status (os/exec, go1.26.2), so a
// timeout reached the log as `exit status 1: ` with nothing after the colon —
// the same text a git that answered "no" would produce. Callers classify with
// errors.Is against context.DeadlineExceeded and context.Canceled, and read
// git's own streams from Stderr and Stdout.
type GitError struct {
	Args []string
	Dir  string
	// Err is what os/exec returned: the killed process's own exit status, or
	// context's error when no process was started at all.
	Err error
	// Stderr and Stdout are git's, trimmed. On a kill they are usually empty.
	Stderr string
	Stdout string
	// Timeout is the bound that was in force, named in the message so the
	// reader knows which one to argue with.
	Timeout time.Duration
	// Elapsed is how long the git actually ran.
	Elapsed time.Duration
	// TimedOut: a deadline ended this git, not git itself.
	TimedOut bool
	// Canceled: the caller gave up.
	Canceled bool
	// Caller: the deadline or cancellation was the CALLER's, not this
	// runner's bound — the rail supersedes a poll this way.
	Caller bool
}

// Output is both of git's streams, stderr first, empty when git said nothing.
// A git that fails quietly is a real signal: `rev-parse --verify --quiet HEAD`
// says nothing when HEAD is unborn and prints on every other failure.
func (e *GitError) Output() string {
	return strings.TrimSpace(e.Stderr + "\n" + e.Stdout)
}

func (e *GitError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "git %s in %s: ", strings.Join(e.Args, " "), e.Dir)
	switch {
	case e.TimedOut && e.Caller:
		fmt.Fprintf(&b, "timed out: the caller's deadline passed after %s, inside the %s bound: ", e.Elapsed.Round(time.Millisecond), e.Timeout)
	case e.TimedOut:
		fmt.Fprintf(&b, "timed out after the %s bound (git ran %s): ", e.Timeout, e.Elapsed.Round(time.Millisecond))
	case e.Canceled:
		fmt.Fprintf(&b, "cancelled by the caller after %s: ", e.Elapsed.Round(time.Millisecond))
	}
	if e.Err != nil {
		fmt.Fprintf(&b, "%v", e.Err)
	}
	// stderr first: on a failure git's own sentence is there, and stdout is
	// usually empty. Both are included because a few subcommands diagnose on
	// stdout, and an error naming nothing is useless.
	return b.String() + ": " + e.Output()
}

func (e *GitError) Unwrap() error { return e.Err }

// Is answers for the cause, which Err cannot: a killed git returns its own
// exit status, so only this runner knows the deadline is what ended it.
func (e *GitError) Is(target error) bool {
	switch target {
	case context.DeadlineExceeded:
		return e.TimedOut
	case context.Canceled:
		return e.Canceled
	}
	return false
}

// ExecRunner is the production Runner: the only place in the product that
// starts a git process.
type ExecRunner struct {
	// Timeout overrides GitTimeout when non-zero. Tests use it; nothing else
	// needs to.
	Timeout time.Duration
	// Env is appended to every git's environment after the headless
	// variables, so an entry here wins over this process's own. cmd/api's
	// host-config runner sets GIT_CEILING_DIRECTORIES with it; nil adds none.
	Env []string
}

var _ Runner = (*ExecRunner)(nil)

// NewExecRunner returns a Runner that shells out to git on PATH.
func NewExecRunner() *ExecRunner { return &ExecRunner{Timeout: GitTimeout} }

// Run executes `git <args...>` in dir and returns its STANDARD OUTPUT ONLY.
//
// The two streams are captured separately and that separation is the whole
// point of this function. git writes non-fatal warnings to stderr and still
// exits 0 — "warning: ignoring broken ref refs/heads/x" is the common one —
// and every caller here parses the result line by line. CombinedOutput on the
// success path would hand that warning to the parser, and the rail would grow
// a branch literally named "warning: ignoring broken ref refs/heads/x" with a
// derived id to match, while rev-parse --abbrev-ref HEAD would return
// "warning: …\nmain" and match no branch, so nothing would be flagged IsHead.
//
// On FAILURE it returns a *GitError carrying both streams, stderr first, and
// what ended the git. Without that the rail shows "exit status 128" and the
// user has nothing to act on; with it the error carries git's own sentence,
// e.g. "fatal: not a git repository", or says the bound killed it.
func (r *ExecRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = GitTimeout
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// The bound has to be real even when the kill is refused; see gitWaitDelay.
	cmd.WaitDelay = gitWaitDelay
	// Headless. A credential helper, an editor or a pager would block until
	// the timeout, and on Windows the Git Credential Manager pops a window.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_PAGER=cat",
		"GCM_INTERACTIVE=never",
	)
	cmd.Env = append(cmd.Env, r.Env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		ge := &GitError{
			Args:    args,
			Dir:     dir,
			Err:     err,
			Stderr:  strings.TrimSpace(stderr.String()),
			Stdout:  strings.TrimSpace(stdout.String()),
			Timeout: timeout,
			Elapsed: time.Since(start),
		}
		// What ended this git, asked of the contexts rather than of the exit
		// status, which on Windows is the killed process's own. The caller's
		// context is asked FIRST: the rail cancels a superseded poll, and
		// that is not this runner's bound being exceeded.
		switch {
		case errors.Is(parent.Err(), context.Canceled):
			ge.Canceled, ge.Caller = true, true
		case errors.Is(parent.Err(), context.DeadlineExceeded):
			ge.TimedOut, ge.Caller = true, true
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			ge.TimedOut = true
		}
		return nil, ge
	}
	return stdout.Bytes(), nil
}
