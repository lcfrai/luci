package project

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitTimeout bounds one git invocation. Enumeration runs behind an HTTP
// handler that the rail polls, so a git that blocks — a network remote, a
// credential prompt, an index.lock held by another process — must fail rather
// than hold a request open.
const GitTimeout = 10 * time.Second

// ExecRunner is the production Runner: the only place in the product that
// starts a git process.
type ExecRunner struct {
	// Timeout overrides GitTimeout when non-zero. Tests use it; nothing else
	// needs to.
	Timeout time.Duration
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
// On FAILURE it returns both streams in the error, stderr first. Without that
// the rail shows "exit status 128" and the user has nothing to act on; with it
// the error carries git's own sentence, e.g. "fatal: not a git repository".
func (r *ExecRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = GitTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Headless. A credential helper, an editor or a pager would block until
	// the timeout, and on Windows the Git Credential Manager pops a window.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_PAGER=cat",
		"GCM_INTERACTIVE=never",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// stderr first: on a failure git's own sentence is there, and stdout
		// is usually empty. Both are included because a few subcommands
		// diagnose on stdout, and an error naming nothing is useless.
		detail := strings.TrimSpace(stderr.String() + "\n" + stdout.String())
		return nil, fmt.Errorf("git %s in %s: %v: %s",
			strings.Join(args, " "), dir, err, detail)
	}
	return stdout.Bytes(), nil
}
