### Task 27: The `panacea-orchestrator` runner, and the pod-kill test

`sandboxd` starts exactly one PTY command for a pod's life (`sandboxd/cmd/sandboxd/main.go:41`,
`-command`; `sandboxd/internal/hub/hub.go:50`, the one `ptysession.Start` call) and phase 8 bans
`pods/exec` (P8-D7). So an Orchestrator cannot be "a Claude session Fleet types into". Its PTY
command is a product-owned runner, `panacea-orchestrator`, that does nothing on its own: it waits
for task 26's `turn.begin`, runs **one headless Claude Code child** for that turn, reports the
turn's end, and waits again. The runner is the only process in the pod that knows a turn's nonce,
and the only one that can end a turn.

Five properties, each pinned by a test.

1. **Never a fork.** The first turn on a session is `claude -p --session-id <uuid>`, every later
   one `claude -p --resume <uuid>`. `--fork-session` never appears. Phase 2's Resume forks on
   purpose (`control-plane/internal/harness/argv.go:91-100`, and the comment there explains why:
   two cockpit sessions must not share a transcript). The runner is the opposite case, one
   conversation continued turn after turn, so it has its own argv builder and never imports or
   copies phase 2's. The two modules could not share it anyway: `sandboxd` imports nothing from
   `control-plane` (the file map's rule).
2. **The runner accepts only the product's sentence as input.** `turn.begin`'s `input` must match
   task 26's `Input(n)` exactly, by regular expression, or the turn is refused. A compromised or
   buggy Fleet still cannot put text into the Orchestrator's prompt through this path; peer text
   arrives only as `panacea inbox`'s tool output (§7.1).
3. **Nothing reads meaning from the child's output** (OD-8). The runner picks `--output-format
   text`, not the design's `stream-json` (§4.2 says `stream-json`; the skeleton's scope for this
   task says `text`, and the scope wins), because `text` has no fields to be tempted by. For a
   human-origin turn the stdout is kept **verbatim and bounded** (8,000 bytes, cut on a rune
   boundary) as the turn's `text·A` reply; for a Fleet-origin turn it is drained and discarded.
   Nothing in the runner or Fleet parses it.
4. **The child cannot see the turn's secrets or the pod's.** Its environment is an allowlist
   (`HOME`, `PATH`, `LANG`, `TZ`, `TERM=dumb`), so `PANACEA_APP_CONTROL_TOKEN`, the attach token
   and anything `ANTHROPIC_*` never reach it; the nonce is never in its argv or environment; its
   stdin is `/dev/null`; it runs in its own process group, which the runner kills whole when the
   turn ends, so no background process outlives a turn and waits for the next `turn.begin`.
5. **The working directory is an empty folder**, `/run/panacea/orch`, the same path every turn,
   so no project settings or `CLAUDE.md` load and `--resume` finds the transcript under the same
   project key. The runner checks, before every turn, that the folder is empty and that no
   `CLAUDE.md`, `CLAUDE.local.md` or `.claude/` exists in it or its ancestors, nor
   `$HOME/.claude/CLAUDE.md`; any hit refuses the turn with `exit: failed`, code `-2`. A
   `--setting-sources`-style flag that would make this unnecessary is **UNVERIFIED** at the pinned
   version (task 33 checks); the empty folder does not depend on it.

Three decisions the scope left open.

**(a) The runner talks to `sandboxd` through task 16's socket, not a private descriptor.** Task
16's part (decision 10) already answers `events.wait` from `sandboxd` itself, only to the PTY's own
pid (`SO_PEERCRED`), queueing every pushed event but `snapshot`. The runner re-dials after each
50-second empty answer. This task requires one more line of task 16's code: `turn.end` on the
socket is accepted **only from the same pid**, else `forbidden`, so the harness (a child of the
runner, with a different pid) cannot end its own turn. Task 26's nonce is the second lock, in
Fleet.

**(b) The runner kills the child before Fleet's deadline, not at it.** The local kill time is
`min(deadline, now + 1 h) − 15 s`: `SIGTERM` to the process group, `SIGKILL` 10 s later, then
`turn.end` with `exit: timeout`, `code: -1`. Fleet's claim therefore ends by a runner's report,
not by the sweep, in every case but a dead pod. The one-hour cap bounds a clock-skewed or hostile
deadline.

**(c) `turn.end` is retried until it is answered.** While the stream is down the socket answers
`unavailable` (task 16, decision 5). The runner retries with backoff (1 s doubling to 30 s) for up
to 15 minutes, then gives up and waits for the next event. `turn_mismatch` (Fleet already recorded
the turn `lost`, task 26 (d)) is logged and treated as done. A `stop` event makes the runner exit
0, which ends the PTY and so the pod; `nudge` is ignored (worker-only, task 38); an unknown event
is logged by kind, never by content.

**Files:**
- `sandboxd/cmd/panacea-orchestrator/main.go` — reads its environment once, builds a `Runner`,
  runs it; exit 0 on `stop`, 2 on bad configuration (new)
- `sandboxd/internal/runner/runner.go` — `Config`, `Runner`, `Run`: the wait → turn → end loop (new)
- `sandboxd/internal/runner/argv.go` — `Argv`, `ChildEnv`, `ValidInput`: the whole command line (new)
- `sandboxd/internal/runner/preflight.go` — the empty-folder and no-instructions check (new)
- `sandboxd/internal/runner/client.go` — `Client`: `Wait` and `End` over the socket, one request
  per connection (task 16's rule 2) (new)
- `sandboxd/internal/runner/child_linux.go`, `child_other.go` — start in a new process group, kill
  the group; the non-Linux build returns `ErrUnsupported` (new)
- `sandboxd/internal/runner/{runner,argv,preflight,client}_test.go`, `child_linux_test.go` (new)
- `sandboxd/internal/control/socket.go` — `turn.end` accepted only from `RunnerPid()`, the check
  task 16 applies to `events.wait` (modified; task 16's file, one condition)
- `images/sandbox/Dockerfile` — builds `panacea-orchestrator` beside `sandboxd` (`:8`) and copies
  it to `/usr/local/bin` (`:20`) (modified)
- `control-plane/internal/fleet/turns/podkill_test.go` — the pod-kill test, Fleet side (new)

The file map's row for this task lists `runner.go`, `rotate.go` and `dive.go`. `rotate.go` and
`dive.go` are task 40's. `argv.go`, `preflight.go`, `client.go` and the two `child_*.go` files are
added so each property above has one small file and one test file.

**Interfaces:**
- Produces:
  - the binary `/usr/local/bin/panacea-orchestrator`, the `orchestrator` class's PTY command
    (task 15 sets `-command` to it; task 15's scope names it)
  - `runner.Argv(claude string, b control.TurnBegin) ([]string, error)`, `runner.ChildEnv(parent
    []string) []string`, `runner.ValidInput(s string) bool` — consumed by task 40 (rotation starts
    a `new` session through the same `Argv`; DIVE's interactive `claude --resume` is task 40's own
    builder and must not be this one, since this one always adds `-p`)
  - `runner.Runner.Parked() bool` — true between turns; task 40's DIVE attaches only while parked
- Consumes:
  - task 26 (ST.5): `control.TurnBegin`, `control.TurnSession`, `control.TurnEnd`,
    `control.VerbTurnEnd`, `control.Exit*` in `sandboxd/internal/control/turn.go`; the exact
    `Input(n)` sentences
  - task 16: `control.DefaultSocketPath`, `control.VerbEventsWait`, `control.CodeForbidden`, the
    ST.1 `Request`/`Response`/`Error` shapes, `CodeUnavailable`, and decision 10's pid check
  - task 15: the orchestrator pod's `emptyDir` `HOME` and `/run/panacea`, no machine disk, and
    `sandboxd`'s policy digest check before the PTY starts (so a turn never runs under an
    unverified managed file)
  - task 18: the `panacea` CLI on `PATH` (the child's only tool besides `Read`)
- Requires of **task 16** (code, not plan): `socket.go` answers `turn.end` with `forbidden`
  unless the peer pid equals `RunnerPid()`. One condition beside the `events.wait` one.
- Requires of **task 26**: `turn_mismatch` is the answer to a stale `turn.end`, never
  `unavailable`, so the runner can tell "give up" from "retry".

**`argv.go`, complete.**

```go
// Package runner is the orchestrator class's PTY command: one headless
// Claude Code child per turn, never a fork, never reading its output.
package runner

import (
	"errors"
	"regexp"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/control"
)

var (
	uuidRE  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	inputRE = regexp.MustCompile("^(1 team event\\. Run `panacea inbox` to read it\\." +
		"|([2-9]|[1-4][0-9]|50) team events\\. Run `panacea inbox` to read them\\.)$")

	ErrBadSession = errors.New("runner: turn.begin session is not a v4 uuid with mode new|resume")
	ErrBadInput   = errors.New("runner: turn.begin input is not the product template")
)

// ValidInput is true only for task 26's Input(n), 1 <= n <= MaxBatch (50).
func ValidInput(s string) bool { return inputRE.MatchString(s) }

// Argv is the child's whole command line. Flags are those `claude --help`
// 2.1.274 lists (control-plane/internal/harness/argv.go:91-97 quotes the two
// session flags); that -p honours them this way at the image's pinned
// version is UNVERIFIED, and the OD-8 canary covers it (task 33).
func Argv(claude string, b control.TurnBegin) ([]string, error) {
	if !uuidRE.MatchString(b.Session.ID) {
		return nil, ErrBadSession
	}
	if !ValidInput(b.Input) {
		return nil, ErrBadInput
	}
	argv := []string{claude, "-p", "--output-format", "text"}
	switch b.Session.Mode {
	case "new":
		argv = append(argv, "--session-id", b.Session.ID)
	case "resume":
		argv = append(argv, "--resume", b.Session.ID)
	default:
		return nil, ErrBadSession
	}
	return append(argv, b.Input), nil
}

// childEnvKeys is the whole allowlist. Policy (model, gateway base URL,
// apiKeyHelper) reaches the child through the managed-settings file task
// 15 mounts, never through the environment (Global constraint 10).
var childEnvKeys = []string{"HOME", "PATH", "LANG", "TZ"}

// ChildEnv filters the runner's own environment down to the allowlist and
// forces TERM=dumb: the child writes to a pipe, not a terminal.
func ChildEnv(parent []string) []string {
	out := make([]string, 0, len(childEnvKeys)+1)
	for _, kv := range parent {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		for _, allow := range childEnvKeys {
			if k == allow {
				out = append(out, kv)
			}
		}
	}
	return append(out, "TERM=dumb")
}
```

`Argv` adds no `--permission-mode`, `--model`, `--allowedTools`, `--dangerously-skip-permissions`,
`--continue` or `--fork-session`. Each is either policy, which is the managed file's job
(task 13's invariants pin `permissions.defaultMode` and the Orchestrator's allowed tools), or
exactly the thing this runner must not do. The test asserts the full argv, element by element,
for both modes, so adding any flag fails it.

**`preflight.go`, complete.**

```go
package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrPreflight wraps every reason a turn is refused before the child starts.
var ErrPreflight = errors.New("runner: preflight")

// instructionNames are the files and folders Claude Code reads instructions
// or settings from, walking up from the working directory. The list is the
// documented set at 2.1.27x; which of them the pinned CLI reads is
// UNVERIFIED (task 33's /status check).
var instructionNames = []string{"CLAUDE.md", "CLAUDE.local.md", ".claude"}

// Preflight refuses a turn unless dir is an empty 0700 directory, and no
// instruction file exists in dir, its ancestors or home's ~/.claude.
// Transcripts under ~/.claude/projects are expected and not checked.
func Preflight(dir, home string) error {
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm() != 0o700 {
		return fmt.Errorf("%w: %s is not a 0700 directory", ErrPreflight, dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		return fmt.Errorf("%w: %s is not empty", ErrPreflight, dir)
	}
	for d := filepath.Dir(dir); ; d = filepath.Dir(d) {
		for _, n := range instructionNames {
			if _, err := os.Lstat(filepath.Join(d, n)); err == nil {
				return fmt.Errorf("%w: %s exists", ErrPreflight, filepath.Join(d, n))
			}
		}
		if d == filepath.Dir(d) {
			break
		}
	}
	for _, n := range []string{"CLAUDE.md", "CLAUDE.local.md"} {
		p := filepath.Join(home, ".claude", n)
		if _, err := os.Lstat(p); err == nil {
			return fmt.Errorf("%w: %s exists", ErrPreflight, p)
		}
	}
	return nil
}
```

The ancestor walk skips `dir` itself, which the emptiness check already covers. `$HOME` is the
pod's `emptyDir` (task 15), which is not an ancestor of `/run/panacea/orch`, so `~/.claude/`
(where the CLI keeps transcripts) is checked only for the two instruction files, not for its
existence. The error names the path it found, which is a product path, never peer text.

**`client.go`, the surface.** One request per connection (task 16, rule 2):

```go
package runner

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/control"
)

var (
	ErrUnavailable  = errors.New("runner: control unavailable")    // retry
	ErrTurnMismatch = errors.New("runner: fleet refused turn.end") // give up
	ErrForbidden    = errors.New("runner: sandboxd refused: not the runner pid")
)

type Client struct {
	Socket string // control.DefaultSocketPath
	dial   func(ctx context.Context, path string) (conn, error) // net.Dial in production
}

// Wait sends events.wait and returns the next pushed event, or nil after
// sandboxd's 50 s empty answer.
func (c *Client) Wait(ctx context.Context) (json.RawMessage, error)

// End sends turn.end; ok, ErrUnavailable, ErrTurnMismatch or ErrForbidden.
func (c *Client) End(ctx context.Context, e control.TurnEnd) error
```

`End` maps the ST.1 error codes: `unavailable` → `ErrUnavailable`; `turn_mismatch` →
`ErrTurnMismatch`; `forbidden` → `ErrForbidden` (a bug: the runner is not the PTY's pid, and the
runner exits 1 so the pod restarts rather than spin); anything else → an error carrying the code.

**`runner.go`, the loop, complete in its control flow.**

```go
package runner

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/control"
)

const (
	MaxReplyBytes = 8000             // task 26's MaxReplyBytes
	killMargin    = 15 * time.Second // kill this long before Fleet's deadline
	maxTurn       = time.Hour        // cap on a deadline, whatever Fleet sent
	grace         = 10 * time.Second // SIGTERM → SIGKILL
	endRetryFor   = 15 * time.Minute
)

type Config struct {
	Claude  string   // absolute path, resolved once at start
	Workdir string   // /run/panacea/orch
	Home    string   // $HOME
	Env     []string // os.Environ(); filtered by ChildEnv per turn
	Client  *Client
	Start   func(argv, env []string, dir string) (Child, error) // startChild in production
	Now     func() time.Time
	Log     *slog.Logger // to the PTY: status lines only, never child output
}

// Child is one running harness process in its own process group.
type Child interface {
	Stdout() io.Reader
	Wait() (code int, err error)
	KillGroup(sig Signal) error // the whole group, stragglers included
}

type Runner struct {
	cfg    Config
	parked atomic.Bool
}

func New(cfg Config) *Runner { r := &Runner{cfg: cfg}; r.parked.Store(true); return r }

// Parked is true between turns (task 40's DIVE attaches only then).
func (r *Runner) Parked() bool { return r.parked.Load() }

// Run returns nil on a stop event, an error on ErrForbidden or ctx end.
func (r *Runner) Run(ctx context.Context) error {
	for {
		raw, err := r.cfg.Client.Wait(ctx)
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			r.cfg.Log.Warn("events.wait failed; retrying", "err", err)
			sleep(ctx, time.Second)
			continue
		case raw == nil:
			continue // 50 s empty answer; re-dial
		}
		var head struct{ Event string `json:"event"` }
		if json.Unmarshal(raw, &head) != nil {
			r.cfg.Log.Warn("undecodable event dropped")
			continue
		}
		switch head.Event {
		case control.EventStop:
			return nil
		case control.EventTurnBegin:
			var b control.TurnBegin
			if err := strictDecode(raw, &b); err != nil {
				r.cfg.Log.Warn("turn.begin refused", "err", err)
				continue // no turn id we trust; Fleet's sweep records it lost
			}
			if err := r.deliver(ctx, r.turn(ctx, b)); err != nil {
				return err // only ErrForbidden escapes
			}
		default:
			r.cfg.Log.Info("event ignored", "kind", head.Event) // nudge, snapshot, …
		}
	}
}

// turn runs one child and returns the turn.end to send. It never fails:
// every failure is an exit value Fleet records.
func (r *Runner) turn(ctx context.Context, b control.TurnBegin) control.TurnEnd {
	r.parked.Store(false)
	defer r.parked.Store(true)
	start := r.cfg.Now()
	end := control.TurnEnd{Turn: b.Turn, Nonce: b.Nonce, Exit: control.ExitFailed, Code: -2}
	argv, err := Argv(r.cfg.Claude, b)
	if err == nil {
		err = Preflight(r.cfg.Workdir, r.cfg.Home)
	}
	if err != nil {
		r.cfg.Log.Warn("turn refused before start", "turn", b.Turn, "err", err)
		end.DurationMs = r.cfg.Now().Sub(start).Milliseconds()
		return end
	}
	child, err := r.cfg.Start(argv, ChildEnv(r.cfg.Env), r.cfg.Workdir)
	if err != nil {
		r.cfg.Log.Warn("child did not start", "turn", b.Turn, "err", err)
		return end
	}
	r.cfg.Log.Info("turn started", "turn", b.Turn, "mode", b.Session.Mode)

	killAt := b.Deadline
	if cap := start.Add(maxTurn); killAt.After(cap) {
		killAt = cap
	}
	timer := time.AfterFunc(killAt.Sub(r.cfg.Now())-killMargin, func() {
		_ = child.KillGroup(SIGTERM)
		time.AfterFunc(grace, func() { _ = child.KillGroup(SIGKILL) })
	})
	type out struct {
		s string
		t bool
	}
	got := make(chan out, 1)
	go func() { s, t := collect(child.Stdout(), b.KeepReply); got <- out{s, t} }()
	code, _ := child.Wait()      // the group leader only, not the pipe
	killed := !timer.Stop()
	_ = child.KillGroup(SIGKILL) // stragglers: nothing outlives a turn
	o := <-got                   // the pipe closes once the whole group is gone
	reply, truncated := o.s, o.t

	end.Code, end.DurationMs = code, r.cfg.Now().Sub(start).Milliseconds()
	switch {
	case killed:
		end.Exit, end.Code = control.ExitTimeout, -1
	case code == 0:
		end.Exit = control.ExitOK
	default:
		end.Exit = control.ExitFailed
	}
	if b.KeepReply {
		end.Reply, end.ReplyTruncated = reply, truncated
	}
	r.cfg.Log.Info("turn ended", "turn", b.Turn, "exit", end.Exit, "ms", end.DurationMs)
	return end
}

// collect reads stdout to EOF. With keep, it keeps the first MaxReplyBytes,
// cut back to a rune boundary; without, it discards. It never inspects the
// bytes (OD-8): no field, no line, no marker is looked for.
func collect(r io.Reader, keep bool) (string, bool) {
	if !keep {
		_, _ = io.Copy(io.Discard, r)
		return "", false
	}
	buf := make([]byte, MaxReplyBytes+1)
	n, _ := io.ReadFull(r, buf)
	_, _ = io.Copy(io.Discard, r)
	if n <= MaxReplyBytes {
		return string(buf[:n]), false
	}
	cut := MaxReplyBytes
	for cut > 0 && !utf8.RuneStart(buf[cut]) {
		cut--
	}
	return string(buf[:cut]), true
}

// deliver sends turn.end until it is answered. Only ErrForbidden escapes.
func (r *Runner) deliver(ctx context.Context, e control.TurnEnd) error {
	wait, until := time.Second, r.cfg.Now().Add(endRetryFor)
	for {
		err := r.cfg.Client.End(ctx, e)
		switch {
		case err == nil, errors.Is(err, ErrTurnMismatch):
			return nil
		case errors.Is(err, ErrForbidden):
			return err
		case ctx.Err() != nil || r.cfg.Now().After(until):
			r.cfg.Log.Warn("turn.end not delivered; fleet's sweep will record it", "turn", e.Turn)
			return nil
		}
		sleep(ctx, wait)
		if wait *= 2; wait > 30*time.Second {
			wait = 30 * time.Second
		}
	}
}
```

`strictDecode` is `json.Decoder` with `DisallowUnknownFields` and a check for trailing bytes, the
same as task 09's decoders; `sleep` is a `select` on `ctx.Done()` and a timer. `runner.go` imports
`errors` besides those shown. `control.EventStop` is task 09's constant; `control.EventTurnBegin`
is task 26's sandbox copy. The kill timer's duration can be negative (a deadline already inside
the margin), in which case `AfterFunc` fires at once and the turn ends `timeout`, which is the
honest record.
