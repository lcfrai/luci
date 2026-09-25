### Task 38: The template-only PTY writer and the delivery ladder

**Slice:** TM1. **Focused time:** 2.5 h. **dependsOn:** 19 (the control stream and dispatch), 31
(directives, piggyback as rung 1); 17 (the broker's attach relay under a delegation) and 20 (the
session's stop and resume) through them.

A directive is a `dv-…` Hub record before it is delivered (§4.3, task 31). Task 31 built rung 1:
every `panacea` call a worker makes returns its pending directives first. That reaches a **busy**
worker, which calls `panacea check` before every edit. It does not reach an **idle** one, which is
sitting at its prompt and will call nothing until something types into it. This task is the rest
of §4.3's ladder, and it owns the most dangerous line of code in the phase: the only Fleet code
that writes bytes into an agent's terminal.

The Claude Code prompt is a command line as well as a chat box: `!` runs a shell command, `/` a
slash command, `#` writes a memory, `Shift+Tab` (`ESC [ Z`) cycles permission modes, and text
typed while the agent works queues as the next user turn (§4.3, citing the interactive-mode docs
read 2026-09-25, `07` C3). A PTY write is a user keystroke with the human's authority. So:

1. **The writer's API takes a template id, never bytes.** There is no function in the package
   that accepts a `[]byte` or a `string` to write. The bytes live in one unexported table.
2. **A unit test proves it refuses every other byte sequence**, including `!`, `/`, `#`, `ESC [ Z`
   and every control byte (`07` C3), by construction and by fuzzing.
3. **No directive text, no peer text and no agent text ever crosses a PTY.** A nudge says only
   *"go and look"*; the facts arrive through the pull (§5.4). Task 31's piggyback remains the
   **only** way directive text reaches a worker (`task-31.md:18-21`).

Six decisions the scope left open, made here so nobody re-derives them.

**(a) The writer is in Fleet and writes one `Stdin` frame through the broker's attach relay.**
The skeleton puts it in `cp/internal/fleet/ptywriter`; task 09's `nudge` event says instead that
"the pod's delivery rung decides" (`task-09.md:402-406`). Task 16 queues pushed events for
`events.wait` only, and answers that verb only to the PTY's own pid (`task-16.md:137-143`), which
for a worker is the harness itself, never a caller; so a worker pod has no consumer for `nudge`.
This task follows the skeleton: Fleet dials the worker's `/attach` through the broker (task 17's
attach relay under the owner's delegation), sends **one** phase 2 `Stdin` frame (kind byte
`0x01`, `sandboxd/internal/wire/frame.go:16`) whose payload is a template, and closes. Task 09's
`nudge` stays the Orchestrator runner's signal. Listed in the gaps for task 09 to reword.

**(b) The writer never reads what the terminal says.** Attaching makes `sandboxd` send the
scrollback replay first (`sandboxd/internal/server/server.go:191`). `coder/websocket`'s
`CloseRead` would close the socket on that first data message, so the writer runs one goroutine
that reads each message into `io.Discard` and never decodes it, not even the kind byte, then
closes after its write. Nothing is logged but the byte count. OD-8 holds in the strongest form:
the bytes carry no meaning to Fleet because Fleet never looks at them.

**(c) A template ends in one carriage return, and that is the only control byte allowed.**
Typing without Enter leaves the text in the input box and the idle agent sees nothing. So each
text template is printable ASCII followed by exactly one `\r` (0x0D), and the interrupt template
is the single byte `0x03` (Ctrl+C). The lint test pins the shape: printable `0x20`–`0x7E` only,
no leading `!`, `/` or `#`, no `!` anywhere, one trailing `\r`. The design's directive template
begins with `[`, which after an `ESC` would read as a CSI sequence; no template contains `ESC`, and
decision (e) keeps the writer away from a terminal someone else is typing into, so nothing can
precede it. **UNVERIFIED:** that `\r` submits and
`0x03` interrupts in the pinned Claude Code, mid-generation and mid-tool-call; task 47's probes
settle both, and a failure changes a constant in this file, not the design.

**(d) Idle is a byte count the pod reports, never content.** Phase 2's idle is derived from bytes
crossing the relay (`control-plane/internal/registry/session.go:22-23`, and the reaper's
`LastOutputAt`, `control-plane/internal/reaper/reaper.go:650-658`). In the cloud nobody relays a
worker's terminal unless its owner is attached, so `sandboxd` itself reports
`{"event":"pty.stats","bytesOut":…,"lastOutputAt":…}` on its control connection every 10 s while
the count moves and every 60 s when it does not. Fleet keeps the latest per agent **in memory**
(activity-class state; global constraint 1). A worker is idle when `now - lastOutputAt >=
IdleAfter` (15 s). A worker with no report yet is **not** idle: the ladder fails quiet.

**(e) No nudge while the owner is attached.** A template typed into a terminal where a person is
half-way through a sentence would corrupt their input and submit it. Task 39's relay knows every
live attach; the ladder asks `Attached(agent)` and skips rung 3 while it is true. The owner sees
the directive on their Desk anyway.

**(f) One nudge per worker per 3 minutes, and a nudge is owed, not queued.** §5.4's cap
(each nudge costs the worker's owner a turn). The ladder keeps, per worker, `lastNudge` and an
`owed` flag. A directive sets `owed`; the next time the worker is idle, not attached, and 3
minutes have passed since `lastNudge`, the ladder writes **one** template and clears `owed`. Two
directives in one minute therefore cause one nudge; the worker's single `panacea check` then
drains both through rung 1. A worker that calls `panacea` for any reason clears `owed` too,
because piggyback has just delivered everything.

**Files:**
- `control-plane/internal/fleet/ptywriter/ptywriter.go` — `Template`, the byte table, `Writer`,
  `Allowed` (new)
- `control-plane/internal/fleet/ptywriter/{ptywriter_test.go,fuzz_test.go}` — the refusal table,
  the template lint, the fuzz target (new)
- `control-plane/internal/fleet/delivery/ladder.go` — `Ladder`: `Directive`, `Notify`, `Pause`,
  `Resume`, `OnCall`, `OnStats`, the sweep (new)
- `control-plane/internal/fleet/delivery/idle.go` — the in-memory `pty.stats` table (new)
- `control-plane/internal/fleet/delivery/ladder_test.go` (new)
- `control-plane/internal/fleet/wire/ptystats.go`, `sandboxd/internal/control/ptystats.go` —
  `PTYStats`, one copy per module (new, beside task 09's files, not an edit of them)
- `testdata/panacea-control/v1/pty-stats.json` — joins task 09's fixture set (new)
- `sandboxd/internal/control/stats.go` — the 10 s / 60 s reporter over `sandboxd`'s own control
  connection, reading only the hub's byte counter (new)
- `sandboxd/internal/hub/hub.go` — `BytesOut() (n uint64, last time.Time)`, an atomic counter
  bumped where output is fanned out (modify, phase 2's file, one method and two fields)
- `control-plane/internal/fleet/control/dispatch.go` — routes the `pty.stats` frame to
  `Ladder.OnStats` and every worker call to `Ladder.OnCall` (modify, task 19's file, two lines)
- `control-plane/cmd/fleetd/main.go` — builds the writer and the ladder; hands the ladder to task
  31's `Service` and task 37's `Pauser` (modify, one anchored hunk)

**Interfaces:**
- Produces:
  - `ptywriter.Template` and its three values; `(*ptywriter.Writer).Write(ctx, agent, Template)
    error`; `ptywriter.Allowed([]byte) bool`, the test oracle;
  - `delivery.Ladder` with `Directive(ctx, worker, dv, kind)`, `Notify(ctx, worker)`,
    `Pause(ctx, worker, dv)`, `Resume(ctx, worker, dv)` (task 37's `budget.Deliverer`),
    `OnCall(worker)`, `OnStats(agent, PTYStats)`;
  - `wire.PTYStats` / `control.PTYStats` (ST.1 addition, one fixture).
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 31: `directive.Service.Issue`'s post-write hook (the ladder is called after the `dv-…`
    is in the outbox, `task-31.md:121-125`), `Drain` (rung 1, unchanged);
  - task 19: `control.Caller` and dispatch; the upstream frame path (`task-19.md`);
  - task 17: the broker's attach relay under a delegation, reached through a one-method
    `AttachDialer` that `cmd/fleetd` implements with the same dialer task 39 uses;
  - task 20: `sessions.Stop(ctx, agent, keepDisk bool)` and `sessions.Resume(ctx, agent)`
    (**not yet written; required of task 20**, see the gaps);
  - task 39: `relay.Attached(agent) bool` (this writer's own, `task-39.md`);
  - phase 2: `sandboxd/internal/wire` frame kinds (`frame.go:14-19`); `github.com/coder/websocket`.

#### `ptywriter.go` (complete)

```go
// Package ptywriter is the ONLY Fleet code that writes to an agent's PTY.
// Its API takes a Template, never bytes: there is no function here that
// accepts a []byte or string to write (05-design.md §4.3, 07 C3, OD-8).
package ptywriter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// Template is a product-written byte sequence. The zero value is invalid.
type Template uint8

const (
	TemplateDirective  Template = iota + 1 // a directive from the worker's own Orchestrator
	TemplateTeamUpdate                     // §5.4: a fact changed; pull it
	TemplateInterrupt                      // rung 4's keystroke, Ctrl+C
)

// frameStdin is phase 2's attach frame kind for keystrokes
// (sandboxd/internal/wire/frame.go:16). Written here because control-plane
// never imports sandboxd; pinned by TestFrameKindMatchesSandboxd.
const frameStdin = 0x01

// table is every byte sequence this package can ever write. Unexported, and
// the only composite literal of []byte in the package (TestOneTable).
var table = map[Template][]byte{
	TemplateDirective:  []byte("[Panacea] Your Orchestrator sent you a directive. Run `panacea check`.\r"),
	TemplateTeamUpdate: []byte("Team update for you. Run `panacea check`.\r"),
	TemplateInterrupt:  {0x03},
}

var ErrUnknownTemplate = errors.New("ptywriter: unknown template")

// Allowed is the refusal oracle the tests drive: true iff b is exactly one
// template's bytes. Write never calls it on caller input, because there is
// none; it exists so the refusal table and the fuzzer test one function.
func Allowed(b []byte) bool {
	for _, t := range table {
		if bytes.Equal(b, t) {
			return true
		}
	}
	return false
}

// AttachDialer opens the agent's /attach through the broker's relay, under
// the owner's delegation (task 17). cmd/fleetd implements it with the dialer
// task 39 uses for the owner's own attaches.
type AttachDialer interface {
	DialAttach(ctx context.Context, agent model.AgentID) (*websocket.Conn, error)
}

type Writer struct {
	Dial    AttachDialer
	Timeout time.Duration // whole write, dial to close; default 10 s
	// Wrote is called after a successful write with the template and the byte
	// count only, for the log and the metric. Never the bytes.
	Wrote func(agent model.AgentID, t Template, n int)
}

// Write sends exactly one Stdin frame holding t's bytes, discarding every
// message the sandbox sends (the scrollback replay first) unread.
func (w *Writer) Write(ctx context.Context, agent model.AgentID, t Template) error {
	b, ok := table[t]
	if !ok {
		return ErrUnknownTemplate
	}
	timeout := w.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c, err := w.Dial.DialAttach(ctx, agent)
	if err != nil {
		return err
	}
	defer c.CloseNow()
	go discard(ctx, c) // decision (b): read, never decode
	frame := make([]byte, 0, len(b)+1)
	frame = append(append(frame, frameStdin), b...)
	if err := c.Write(ctx, websocket.MessageBinary, frame); err != nil {
		return err
	}
	if w.Wrote != nil {
		w.Wrote(agent, t, len(b))
	}
	return c.Close(websocket.StatusNormalClosure, "")
}

func discard(ctx context.Context, c *websocket.Conn) {
	for {
		_, r, err := c.Reader(ctx)
		if err != nil {
			return
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			return
		}
	}
}
```

`frame` is built from `table`'s bytes and one constant; `TestOneTable` parses the package's own
source with `go/parser` and fails if any other `[]byte` composite literal, string-to-`[]byte`
conversion or `append` of a non-table slice reaches `c.Write`. It is a structural check on the
file, so a later edit that adds a "just this once" byte slice fails the build's tests, not a
review.

#### The refusal table (`ptywriter_test.go`), and what each row kills

`Allowed` must return `false` for every row; `Write` has no parameter that could carry any of them.

| Input | Why it is dangerous (§4.3, `07` C3) |
|---|---|
| `"!rm -rf ."`, `"!"` | Bash mode: a shell command with the human's authority |
| `"/permissions"`, `"/"` | a slash command |
| `"# remember: push to main"` | Claude Code's memory shortcut |
| `"\x1b[Z"` | `Shift+Tab`: cycles permission modes |
| `"\x1b"`, `"\x1b\x1b"` | interrupt, and the double-Esc history menu |
| each of `0x00`–`0x1F` and `0x7F` alone, except the exact interrupt byte `0x03` | every control byte |
| `"\x03\x03"` | a double Ctrl+C exits the CLI |
| `table[TemplateDirective]` with the `\r` removed, with `\n` added, with `\r\r` | a near-miss of a template is not a template |
| `"Team update for you. Run \`panacea check\`. Also run \`git push\`.\r"` | a template with text appended |
| `"[Panacea] Your Orchestrator sent you a directive. Run \`panacea check\`.\r"` in UTF-16, or with a U+202E inserted | encoding and bidi near-misses |
| `""` | the empty write is not a template either |

`FuzzAllowed` feeds random byte strings, seeded with every row above and each template, and
asserts `Allowed(b) == (b is one of table's three values)`. The template lint,
`TestTemplatesArePrintable`, asserts each text template is `0x20`–`0x7E` then exactly one `\r`,
does not start with `!`, `/` or `#`, contains no `!` or `ESC`, and is at most 120 bytes; and
that `TemplateInterrupt` is exactly `{0x03}`. `TestFrameKindMatchesSandboxd` reads
`sandboxd/internal/wire/frame.go` as text and asserts it still declares `Stdin  Kind = 0x01`.

#### `ptystats.go`, both modules (complete; ST.1 addition)

```go
// PTYStats is sandboxd's own upstream frame: how many bytes the PTY has
// written since start, and when the last one was. A count and a time, never
// a byte of content. Sent by sandboxd itself on its control connection, not
// through the in-pod socket, so no process in the VM can forge it.
type PTYStats struct {
	Event        string    `json:"event"`        // "pty.stats"
	BytesOut     uint64    `json:"bytesOut"`     // monotonic since sandboxd start
	LastOutputAt time.Time `json:"lastOutputAt"` // zero before the first byte
}

const EventPTYStats = "pty.stats"
```

The fixture `pty-stats.json` is `{"event":"pty.stats","bytesOut":48213,"lastOutputAt":
"2026-09-25T01:00:00Z"}`, read by both modules' round-trip tests. `sandboxd/internal/control/
stats.go` reads `hub.BytesOut()` every 10 s and sends a frame when the count moved, and at least
every 60 s regardless, so a stuck pod is distinguishable from a quiet one. `hub.BytesOut` is two
atomics bumped in `publish` (`sandboxd/internal/hub/hub.go:84`), where output already fans out;
it counts `len(chunk)` and nothing else.

**Requires of task 19**: an upstream frame whose `event` is `pty.stats` is accepted **only** on a
worker's stream, **only** from `sandboxd`'s own writer (task 16's stream, not a stamped in-pod
request), and routed to `Ladder.OnStats`; anything else with that name is `bad_request`.

#### `idle.go` (complete)

```go
package delivery

// IdleAfter is how long with no PTY output makes a worker idle (decision d).
const IdleAfter = 15 * time.Second

type idleTable struct {
	mu   sync.Mutex
	last map[model.AgentID]time.Time // lastOutputAt; absent = unknown
}

func (t *idleTable) put(a model.AgentID, s wire.PTYStats) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s.LastOutputAt.After(t.last[a]) {
		t.last[a] = s.LastOutputAt
	}
}

// idle is false for an agent with no report: fail quiet, never nudge blind.
func (t *idleTable) idle(a model.AgentID, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	last, ok := t.last[a]
	return ok && !last.IsZero() && now.Sub(last) >= IdleAfter
}
```

A worker that has **never** written a byte reports a zero `LastOutputAt` and is not idle by this
rule; it is also a worker that has not started, and the directive reaches it through piggyback on
its first `panacea` call. Two replicas each keep their own table; task 19 routes a stream to one
replica, and the other replica's ladder is not driving that worker (below).

#### `ladder.go` (complete public surface)

```go
package delivery

const (
	NudgeEvery = 3 * time.Minute // §5.4
	StopGrace  = 10 * time.Second
	SweepEvery = 5 * time.Second
)

// Rung2 is the inbox socket, behind canary C4 and off by default. No
// implementation ships in TM1: nil means off (decision below).
type Rung2 interface {
	Post(ctx context.Context, worker model.AgentID, t ptywriter.Template) error
}

type Sessions interface { // task 20
	Stop(ctx context.Context, agent model.AgentID, keepDisk bool) error
	Resume(ctx context.Context, agent model.AgentID) error
}

type Attachments interface { // task 39
	Attached(agent model.AgentID) bool
}

type Ladder struct {
	Writer   *ptywriter.Writer
	Inbox    Rung2 // nil unless PANACEA_APP_FLEET_INBOX_SOCKET=on and C4 passed
	Sessions Sessions
	Attached Attachments
	Now      func() time.Time
	Log      *slog.Logger
	// unexported: idle idleTable; per-worker lastNudge and owed, under mu
}

// Directive is task 31's post-write hook: the dv-… is already durable. A
// pause goes straight to rung 4; every other kind marks a nudge owed.
func (l *Ladder) Directive(ctx context.Context, worker model.AgentID, dv, kind string) error

// Notify is §5.4's "team update": a fact changed that this worker should pull.
func (l *Ladder) Notify(ctx context.Context, worker model.AgentID)

// Pause is rung 4: the interrupt template, then after StopGrace a stop that
// keeps the machine disk. Resume relaunches the same conversation.
func (l *Ladder) Pause(ctx context.Context, worker model.AgentID, dv string) error
func (l *Ladder) Resume(ctx context.Context, worker model.AgentID, dv string) error

// OnCall is task 19's hook on every worker call: piggyback just delivered
// everything, so any owed nudge is cleared.
func (l *Ladder) OnCall(worker model.AgentID)

// OnStats feeds the idle table, and runs the owed check for that worker.
func (l *Ladder) OnStats(agent model.AgentID, s wire.PTYStats)

// Run sweeps owed nudges every SweepEvery until ctx ends.
func (l *Ladder) Run(ctx context.Context) error
```

**The rungs, in order, for one owed nudge.** `try(worker)` runs from `OnStats`, from the sweep
and at once from `Directive`/`Notify`:

1. **Rung 1, piggyback** (task 31) is not this package's: it has already happened if the worker
   called `panacea` since the directive, and `OnCall` cleared `owed`.
2. If not `owed`, stop. If `Attached(worker)`, stop (decision e). If not `idle`, stop: a busy
   worker calls `check` before its next edit and gets rung 1. If `now - lastNudge < NudgeEvery`,
   stop (decision f).
3. **Rung 2**, if `Inbox != nil`: `Post` the same template. **No implementation ships in TM1.**
   Whether `CLAUDE_CODE_MESSAGING_SOCKET` accepts a poster that is not the CLI's descendant is
   **UNVERIFIED** (§4.3 rung 2, canary C4), and a post arrives as a user turn with the human's
   authority, so it carries the same fixed template and nothing else. Off by default:
   `PANACEA_APP_FLEET_INBOX_SOCKET` absent or `off` leaves `Inbox` nil; `on` without C4's recorded
   pass fails `fleetd`'s start with the variable's name.
4. **Rung 3**: `Writer.Write(worker, t)` with `t = TemplateDirective` if any directive is owed,
   else `TemplateTeamUpdate`. On success set `lastNudge = now`, clear `owed`. On failure keep
   `owed`; the sweep retries, still bounded by `NudgeEvery` from the last **successful** nudge.

**Rung 4, `pause`.** `Pause` writes `TemplateInterrupt` (ignoring `Attached`: a pause must land
even while the owner watches, and it is the one template that submits nothing), waits
`StopGrace`, then `Sessions.Stop(worker, keepDisk: true)`. Both steps run even if the interrupt
fails, so a pause cannot be defeated by a dead attach path. `Resume` calls `Sessions.Resume`,
which relaunches the worker on its disk with its conversation id (phase 2's `--resume`); the
`resume` directive then reaches it by piggyback on its first call. `Pause` is the path task 37's
human pause takes (`budget.Deliverer`) and the one an Orchestrator's own `pause` directive takes;
the directive's `origin` is on its `dv-…` record, and the ladder does not look at it.

**Two replicas.** The per-worker `owed` and `lastNudge` live in the replica that holds the
worker's control stream (task 19 keeps one stream per sandbox on one replica). A directive issued
on the other replica reaches this one as a `Directive` call through task 19's cross-replica
`Streams.Push` path (**UNVERIFIED** that task 19 forwards a non-wire call; if it does not, the
directive is delivered by rung 1 on the next call and the nudge is lost, which is the fail-quiet
side). A replica restart forgets `lastNudge`, so a worker may be nudged once more than the cap
after a deploy; that is stated, not engineered away.

- [ ] **Step 1: Write the failing tests first**

Fixture: Alice's worker `ag-a1`; a fake `AttachDialer` over `httptest` with a real
`coder/websocket` server that records every frame and first sends a 4 KB replay; a fake clock; a
recording `Sessions`; a settable `Attachments`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRefusalTable` | every row of the table above: `Allowed` is false | a writer that accepts caller bytes, or a near-miss template |
| `FuzzAllowed` | `Allowed(b)` iff `b` is one of the three table values | any second byte sequence becoming writable |
| `TestTemplatesArePrintable` | the lint above | a template gaining a control byte, a `!` or a leading `/` |
| `TestOneTable` | the package source has exactly one `[]byte` literal site, `table` | a "just this once" byte slice reaching `Write` |
| `TestWriteSendsExactlyOneFrame` | one binary message, `0x01` then the template's bytes, byte for byte; then a normal close | a second frame, a text frame, a missing kind byte |
| `TestWriteDiscardsReplay` | the 4 KB replay is read and dropped; the writer's `Wrote` callback gets the template and the length only | `CloseRead` (closes on the replay), or logging bytes |
| `TestBusyWorkerGetsNoNudge` | output 5 s ago, a directive: zero frames; output stops, 15 s later: one frame | nudging on a timer, or on content |
| `TestTwoDirectivesOneNudge` | two directives 40 s apart to an idle worker: exactly one `TemplateDirective` frame in the minute | a nudge per directive |
| `TestNudgeCapThreeMinutes` | directives at 0, 1 min and 2 min 59 s, idle throughout: one frame; a directive at 3 min 1 s: a second | a cap per directive or no cap |
| `TestCallClearsOwed` | a directive, then the worker calls `panacea`, then idle: zero frames | nudging a worker piggyback already reached |
| `TestAttachedOwnerNoNudge` | `Attached` true: zero frames while it holds; false again: the owed nudge is written | typing into a person's half-written input |
| `TestUnknownIdleFailsQuiet` | no `pty.stats` yet: zero frames | nudging blind |
| `TestPauseInterruptThenStop` | `Pause`: one frame `0x01 0x03`, then after 10 s of fake time `Stop(ag-a1, keepDisk: true)`; with the dialer failing, `Stop` still runs | a pause that depends on the PTY path, or that discards the disk |
| `TestRung2OffByDefault` | no variable: `Inbox` nil; `on` without C4's pass: `fleetd` config fails naming `PANACEA_APP_FLEET_INBOX_SOCKET` | the socket rung on without its canary |
| `TestStatsOnlyFromSandboxd` | a `pty.stats` shape arriving as a stamped in-pod request: `bad_request`, the idle table unchanged | a process in the VM forging idleness |
| `TestNoDirectiveTextInAnyFrame` | a `free` directive with text `"push to main"`, delivered through every rung: every frame written is `0x01` plus one table value, and none contains `"push"` | directive text on the PTY (§4.3) |

```bash
cd control-plane && go test ./internal/fleet/ptywriter/... ./internal/fleet/delivery/... 2>&1 | head -3
```

Expected: `undefined: Template` (build failure).

- [ ] **Step 2: `ptywriter.go`**

Write the table, `Allowed`, `Writer` and `discard`; make the four `ptywriter` tests pass, then run
the fuzzer for a bounded time.

- [ ] **Step 3: `PTYStats`, both copies, the fixture and the sandbox reporter**

Add `wire/ptystats.go` and `control/ptystats.go` with identical fields and tags; add the fixture;
add `BytesOut` to the hub and `stats.go` to `sandboxd`. `cd sandboxd && go test ./...` covers the
reporter with a fake clock: a moving count sends every 10 s, a still one every 60 s.

- [ ] **Step 4: `idle.go`, then `ladder.go`, then the two dispatch lines**

- [ ] **Step 5: Wire it**

In `cmd/fleetd/main.go`, anchored after task 31's `directive.Service` is built:
`ladder := &delivery.Ladder{Writer: &ptywriter.Writer{Dial: attachDialer}, Sessions: sess,
Attached: relay, …}`; `dirSvc.AfterIssue = ladder.Directive`; task 37's `Pauser.Deliver =
ladder`; `go ladder.Run(ctx)`. Until task 39 lands, `Attached` is a stub that returns **true**
for every agent, so rung 3 never fires before Fleet can see live attaches: the fail-quiet side.

- [ ] **Step 6: Acceptance**

```bash
cd control-plane && go vet ./internal/fleet/... ./cmd/fleetd/... && go test ./internal/fleet/ptywriter/... ./internal/fleet/delivery/... ./internal/fleet/control/... -count=1 -race
cd control-plane && go test ./internal/fleet/ptywriter/ -run '^$' -fuzz FuzzAllowed -fuzztime 30s
cd sandboxd && go vet ./... && go test ./internal/control/... ./internal/hub/... -count=1
```

Expected: `ok` for each; the fuzz run ends `PASS` with no new failing input in `testdata/fuzz`.
Then the one-writer guard, which must print nothing (no Fleet package but `ptywriter` builds a
`Stdin` frame or writes a binary message to an attach connection):

```bash
cd control-plane && grep -rnE "MessageBinary|frameStdin|0x01\b" internal/fleet --include=*.go | grep -v "_test.go" | grep -v "internal/fleet/ptywriter/" | grep -v "internal/fleet/relay/"
```

Task 39's `relay` package is excluded by name because it forwards the **owner's own** keystrokes,
which is the attach relay's job, not a Fleet write; task 39 carries its own guard that it never
originates a frame.

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/fleet/ptywriter control-plane/internal/fleet/delivery control-plane/internal/fleet/wire/ptystats.go sandboxd/internal/control/ptystats.go sandboxd/internal/control/stats.go testdata/panacea-control/v1/pty-stats.json
git commit -F <msgfile> -- control-plane/internal/fleet/ptywriter control-plane/internal/fleet/delivery control-plane/internal/fleet/wire/ptystats.go control-plane/internal/fleet/control/dispatch.go control-plane/cmd/fleetd/main.go sandboxd/internal/control sandboxd/internal/hub/hub.go testdata/panacea-control/v1/pty-stats.json
```

**What this task does NOT claim.**
- **That a nudge makes the worker act.** It types a sentence and Enter; the model decides. The
  directive's text reaches it only when it runs `panacea check` (rung 1).
- **That `\r` submits and `0x03` interrupts** in the pinned Claude Code, in every state
  (generating, in a tool call, in a permission prompt). **UNVERIFIED** until task 47's probes; a
  template typed into a permission prompt could answer it, which is one more reason rung 3 fires
  only when the byte count says the terminal is quiet, and one more probe task 47 owes.
- **Rung 2.** The interface and the off switch exist; no inbox-socket code ships in TM1.
- **Exactly one nudge per 3 minutes across a restart or a replica hand-over.** `lastNudge` is in
  memory; a deploy can cost one extra nudge per worker.
- **That a pause is instant.** The interrupt lands in about one round trip; the stop follows
  `StopGrace` later. Between them the worker may finish the tool call it was in.
- **That idle means waiting for input.** It means no PTY output for 15 s. A worker thinking
  silently for longer than that looks idle and may receive a nudge, which queues as its next turn;
  the text is harmless by construction, and the cap bounds the cost.

**Gaps against the skeleton, raised here and not resolved alone:**
1. **Where the PTY write happens.** Task 09's `Nudge` comment says "the pod's delivery rung
   decides" (`task-09.md:402-406`); the skeleton's file map puts the writer in Fleet. This part
   follows the skeleton (decision a). Task 09 should reword `nudge` as the Orchestrator runner's
   wake-up only.
2. **Idle needs a new upstream frame.** `pty.stats` is an ST.1 addition sent by `sandboxd` itself;
   task 16 must send it and task 19 must accept it only from `sandboxd`'s own writer.
3. **Task 20 must provide `Stop(agent, keepDisk)` and `Resume(agent)`.** Its scope names launch
   and end only.
4. **Task 31 must call the ladder after each `dv-…` is durable** (an `AfterIssue` hook), and
   task 37's human pause must reach `Ladder.Pause`, not only the piggyback queue.
5. **Who calls `Notify`.** §5.4 names three pushes: a conflict opened on a file the worker is
   editing (task 25's `Raise`, after task 35's observed edits), a lease it asked for granted (task
   23), and a directive (here). The first two callers are one line each in `cmd/fleetd`; neither
   task's part names them yet.

