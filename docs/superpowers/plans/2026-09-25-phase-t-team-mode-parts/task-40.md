### Task 40: Runner rotation and DIVE

**Slice:** TM1. **Focused time:** 2.5 h. **dependsOn:** 27 (the runner, `Argv`, `ChildEnv`,
`Parked`), 39 (Fleet's attach handler, `RouteAgent`); 26 (ST.5, `SessionFor`, the claim) and 31
(`CurrentTurn` names DIVE's origin) through them. **Seam:** ST.5, extended.

Two things keep an Orchestrator's context and cost bounded, and one lets its owner step inside it.

1. **Rotation.** An Orchestrator is one Claude conversation continued turn after turn
   (`task-27.md:13-19`). Left alone it grows for ever: every turn re-reads the whole transcript,
   and the transcript holds everything the Orchestrator ever wrote about peer text. §4.2 and `06`
   I9 require a **new** session each day and at each task boundary, seeded **only** from Fleet's
   typed state, **never** from the old transcript, so no agent-written summary of a teammate's
   words is carried forward. Taint is not per session (task 31 taints every team Orchestrator from
   its first turn), so rotating resets nothing unsafe (§7.2).
2. **DIVE.** While the runner is parked between turns, the owner may attach an **interactive**
   `claude --resume <same id>` through the runner, see the conversation, and talk to it directly.
   Never a fork: the owner is inside the real session, not a copy (§4.2; AGENTIC-UX's `Enter` to
   dive, `docs/AGENTIC-UX.md:147`). Team events queue meanwhile and show as waiting. DIVE's start,
   its end and every directive it issues are `ot-…` records with origin `human`.

Six decisions the scope left open, made here so nobody re-derives them.

**(a) Fleet decides rotation; the runner only obeys `mode`.** Task 26 already made the session id
Fleet state and left `rotateDue` for this task (`task-26.md:22-29`, `:558`). `turns.RotateDue`
is one pure function: due when the session's first turn was on an earlier **Australia/Sydney**
calendar date than now (the product's one region; UTC would rotate at 10:00 local, mid-morning),
or when a task boundary is pending. The runner sees only `mode: "new"` with a fresh uuid, which it
already handles (`task-27.md:149-157`). Fleet embeds `time/tzdata`, so the zone does not depend on
the image's `/usr/share/zoneinfo`.

**(b) A task boundary is a task of this Orchestrator's reaching `done` or `abandoned`.** Task 24's
task verbs make those transitions (`task-24.md:140-141`); on each, after the Hub write is in the
outbox, they call `turns.MarkBoundary(orch)`, which sets `rotate_pending`; the next Fleet turn is
`new`. A boundary during a DIVE waits for the DIVE to end,
and a DIVE never rotates (decision e).

**(c) Each session gets its own `HOME`, and rotation deletes the old one whole.** The runner cannot
tell Claude Code "forget", and must not reach into its transcript files (OD-8: a transcript may be
**read** through a version-pinned adapter, and nothing may require it; nothing here reads or
names a file inside it). So `rotate.go` gives each session id its own directory,
`/run/panacea/sessions/<uuid>/home`, passes it as the child's `HOME`, and on the first `new`
turn after a rotation removes **every other** session directory with one `os.RemoveAll` per
directory, after the new child has started. The old transcript is then physically gone from the
pod: no `Read` tool call, no `--resume` and no seed can carry a byte of it. The runner never lists,
opens or parses anything under a session's `HOME`; it creates the directory and deletes it.

**(d) The seed is a typed verb, `panacea seed`, not text in the prompt.** Task 27 accepts only
task 26's `Input(n)` as the turn's input (`task-27.md:20-23`), and that stays. A new session
learns its world by calling `panacea seed`, an Orchestrator-only read verb (task 41's instructions
tell it to call `seed` first in any session with no history; the managed `claudeMd` loads in every
session). Its result is Fleet's typed state only: the `panacea team` view (task 22), open threads
by id, kind, state and counterpart (task 28, no message bodies), its human's escalation policy rows
(task 30), open escalations by id, and its **own human's** last five messages (`hm-…`, `text·H`,
from the Hub). No `ms-…` body, no turn reply, nothing the old session wrote. Peer words appear only
where the `team` view already quotes them (task 22's fence); thread bodies still come only through
`panacea inbox`, quoted (§7.1).

**(e) DIVE is a claimed turn with origin `human`, so nothing else can run.** `POST
/v1/agents/{ag}/dive` claims a turn through task 26's `ClaimTurn` (the two-replica guard) with
origin `human`, `Mode: resume` on the current uuid (or `new` if there has never been one), and
`Until = now + DiveMax` (2 h). The claim is what makes Fleet's scheduler start nothing while the
owner is inside (`task-26.md:565-566`), so team events queue exactly as they do behind any turn,
and `CurrentTurn` returns the DIVE's `TurnRef` with origin `human`, which is the hook task 31
left for this (`task-31.md:42-45`). Rotation is never due inside a DIVE: the owner dives into the
conversation they can see, not a fresh one.

**(f) The runner hands its own PTY to the interactive child, and reads nothing.** The runner is
the orchestrator pod's PTY command (`task-27.md:3-9`), so its stdin, stdout and stderr **are** the
PTY the owner attaches to through task 39. On `dive.begin` it starts `claude --resume <uuid>`
(interactive: no `-p`, no `--output-format`, never `--fork-session`) with those three descriptors,
the same environment allowlist plus the PTY's `TERM`, the session's `HOME`, and its own process
group; then it waits for the child to exit, and sends `dive.end`. The runner never reads the PTY
while the child owns it: the bytes go from the child to `sandboxd`'s hub to the owner, as in
phase 2.

**Files:**
- `sandboxd/internal/runner/rotate.go` — `SessionHome`, `PrepareHome`, `PruneHomes` (new)
- `sandboxd/internal/runner/dive.go` — `DiveArgv`, `(*Runner).dive` (new)
- `sandboxd/internal/runner/{rotate,dive}_test.go` (new)
- `sandboxd/internal/runner/runner.go` — `Run` dispatches `dive.begin` while parked; each turn
  runs with its session's `HOME`; `PruneHomes` after a `new` child starts (modify, task 27's file)
- `sandboxd/internal/runner/argv.go` — `ChildEnv(parent, home string)` sets `HOME` to the
  session's (modify, task 27's file, one parameter)
- `sandboxd/internal/control/dive.go`, `control-plane/internal/fleet/wire/dive.go` — `DiveBegin`,
  `DiveEnd`, `DiveStop`, one copy per module (new, beside task 26's `turn.go`)
- `testdata/panacea-control/v1/dive-begin.json`, `dive-end.json`, `dive-stop.json`,
  `seed-result.json` — join task 09's fixture set (new)
- `control-plane/internal/fleet/turns/rotate.go` — `RotateDue`, `MarkBoundary` (new)
- `control-plane/internal/fleet/turns/dive.go` — `(*Scheduler).BeginDive`, `EndDive`, `StopDive`
  (new)
- `control-plane/internal/fleet/turns/{rotate,dive}_test.go` (new)
- `control-plane/internal/fleet/turns/scheduler.go` — `TryBegin` passes `RotateDue(…)` to
  `SessionFor` in place of `false` (`task-26.md:558`); `End` clears `rotate_pending` and stamps
  `started_at` on a `new` session (modify, task 26's file, two anchored hunks)
- `control-plane/internal/fleet/verbs/seed.go` — the `seed` verb, Orchestrator class only (new)
- `control-plane/internal/fleet/server/dive.go` — the three DIVE routes (new)
- `control-plane/internal/fleet/store/migrations/00NN_rotation.sql` — three columns on task 26's
  two tables (new, numbered after the last Fleet migration at merge)
- `control-plane/internal/fleet/store/{memstore,pgstore}.go`, `.../storetest/suite.go` (modify)
- `control-plane/cmd/fleetd/main.go` — registers `seed`, the DIVE routes, `MarkBoundary` in task
  24's verbs (modify, one hunk)

The file map lists only `rotate.go` and `dive.go` under `sbx/internal/runner`. The Fleet halves
are added because rotation is Fleet's decision (decision a) and a DIVE is a Fleet claim (decision
e); neither can live in the pod.

**Interfaces:**
- Produces (ST.5 extension, consumed by task 55's panel and task 21's `fleetbe`):
  - `wire.DiveBegin` / `control.DiveBegin`, pushed as `{"event":"dive.begin", …}`;
    `wire.DiveStop`, pushed as `{"event":"dive.stop", …}`; `wire.DiveEnd`, the runner's
    request verb `dive.end`;
  - `turns.RotateDue(s SessionState, pending bool, now time.Time) bool`,
    `(*Scheduler).MarkBoundary(ctx, orch) error`;
  - `(*Scheduler).BeginDive`, `EndDive`, `StopDive`; routes `POST`/`GET`/`DELETE
    /v1/agents/{ag}/dive`;
  - the `seed` verb and `verbs.SeedResult`.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 26: `SessionState`, `SessionFor`, `ClaimRow`, `Store.ClaimTurn`, `Store.EndTurn`,
    `Store.Session`, `CurrentTurn`, `TurnRecord`, the nonce rule (`task-26.md:41-49`, `:201-250`,
    `:313-318`, `:535-599`);
  - task 27: `Runner`, `Parked`, `Client.Wait`/`End`, `ChildEnv`, `Preflight`, the process-group
    child and its kill (`task-27.md:87-109`, `:310-339`, `:497-540`);
  - task 16: `events.wait`, and the same-pid rule for a runner's upstream verb, which `dive.end`
    joins (`task-16.md:137-143`, `task-27.md:45-51`);
  - task 39: `Server.RouteAgent`, the 404 fold, the owner check (`task-39.md`);
  - task 22: the team view (`TeamResult`); task 28: `Store.OpenThreads` (`task-28.md:397`);
    task 30: the owner's policy rows; task 07: the human messages read (`hm-…`);
  - task 31: `CurrentTurn`'s origin drives the directive's `origin` (`task-31.md:133-134`).

#### `control-plane/internal/fleet/turns/rotate.go` (complete)

```go
package turns

import (
	"context"
	"time"
	_ "time/tzdata" // decision (a): the zone must not depend on the image

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// rotationZone is the product's one region (global constraint: Australia East).
var rotationZone = mustZone("Australia/Sydney")

// RotateDue is ST.5's rotation rule. SessionState gains StartedAt (the first
// turn of this uuid); pending is fleet.orch_sessions.rotate_pending.
func RotateDue(s SessionState, pending bool, now time.Time) bool {
	if s.UUID == "" {
		return false // SessionFor already starts a new one
	}
	if pending {
		return true
	}
	y1, m1, d1 := s.StartedAt.In(rotationZone).Date()
	y2, m2, d2 := now.In(rotationZone).Date()
	return y1 != y2 || m1 != m2 || d1 != d2
}

// MarkBoundary is task 24's call when one of orch's tasks reaches done or
// abandoned. Idempotent. It never ends a turn in flight or a DIVE: the flag
// is read at the next claim.
func (s *Scheduler) MarkBoundary(ctx context.Context, orch model.AgentID) error {
	return s.Store.SetRotatePending(ctx, orch, s.Now())
}

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("turns: " + err.Error()) // unreachable with time/tzdata embedded
	}
	return loc
}
```

`SessionState` gains `StartedAt time.Time` and `Store` gains `SetRotatePending(ctx, orch, at
time.Time)` (stamps `rotate_pending_at` if it is null, so the earliest request is kept) and
`RotatePending(ctx, orch) (bool, error)`; `End` on a `new` turn writes `StartedAt = Began` and
clears the flag **only if it was set at or before `Began`**, in the same transaction as the session
row (task 26's `EndTurn`). A boundary marked during the new session's first turn therefore
survives it, and a lost `new` turn keeps the flag, so the next turn is `new` again: the safe side.
Both times come from `Scheduler.Now`, never the database's `now()`, so the fake clock governs.

#### `sandboxd/internal/runner/rotate.go` (complete)

```go
package runner

import (
	"errors"
	"os"
	"path/filepath"
)

// SessionsDir holds one directory per harness session id. Its parent is the
// pod's /run/panacea emptyDir (task 15); nothing else is written under it.
const SessionsDir = "/run/panacea/sessions"

var ErrBadHome = errors.New("runner: session home is not a directory this runner created")

// SessionHome is the child's HOME for one session id (already validated as
// a v4 uuid by Argv, so it is a safe single path element).
func SessionHome(root, uuid string) string { return filepath.Join(root, uuid, "home") }

// PrepareHome creates the session's HOME, mode 0700, if it does not exist.
// For mode "new" it must NOT exist yet: a uuid is never reused, so an
// existing directory means something is wrong, and the turn is refused.
func PrepareHome(root, uuid, mode string) (string, error)

// PruneHomes removes every session directory under root except keep, each
// with one os.RemoveAll. Its own code reads directory NAMES under root only
// (one os.ReadDir(root), then Lstat per entry: a uuid-named real directory,
// never a symlink). It never opens or reads a file inside a session's HOME;
// os.RemoveAll lists names inside only to unlink them (OD-8: nothing is read).
func PruneHomes(root, keep string) error
```

`Run` calls `PrepareHome` before `Preflight` (so task 27's empty-folder and no-`CLAUDE.md` checks
see the session's own `HOME`, `task-27.md:35-41`), and `PruneHomes` only **after** the `new` child
has started, so a failed start leaves the previous session resumable. The child's working
directory stays task 27's `/run/panacea/orch`, the same for every session, so `--resume` finds its
transcript under the same project key.

#### The DIVE wire, ST.5 extension (complete; the sandbox copy is the same in `package control`)

```go
package wire

import "time"

const (
	EventDiveBegin = "dive.begin" // pushed to an orchestrator pod
	EventDiveStop  = "dive.stop"  // pushed: the owner ended it, or DiveMax passed
	VerbDiveEnd    = "dive.end"   // the runner's request up; orchestrator class only
)

// DiveMax bounds one DIVE. The claim's Until is Began + DiveMax.
const DiveMax = 2 * time.Hour

type DiveBegin struct {
	Event    string      `json:"event"`    // "dive.begin"
	Turn     string      `json:"turn"`     // ot-…, the DIVE's own turn id
	Nonce    string      `json:"nonce"`    // as TurnBegin's: runner memory only
	Session  TurnSession `json:"session"`  // resume the current uuid, or new if none
	Deadline time.Time   `json:"deadline"` // Began + DiveMax, RFC 3339 UTC
}

type DiveStop struct {
	Event  string `json:"event"`  // "dive.stop"
	Turn   string `json:"turn"`
	Reason string `json:"reason"` // "owner" | "deadline" | "access_removed"
}

// DiveEnd carries no reply: the owner saw the conversation themselves, and
// the runner reads nothing from the PTY (decision f).
type DiveEnd struct {
	Turn       string `json:"turn"`
	Nonce      string `json:"nonce"`
	Exit       string `json:"exit"` // ok | failed | timeout
	Code       int    `json:"code"`
	DurationMs int64  `json:"durationMs"`
}
```

The fixtures, byte for byte (task 09's decoders refuse unknown fields):

```json
{"event":"dive.begin","turn":"ot-9c0d1e2f","nonce":"ZGl2ZWRpdmVkaXZlZGl2ZWRpdmVkaXZlZGl2ZWRpdmVk","session":{"mode":"resume","id":"5f0c7a52-3b1e-4c2a-9d7e-0a1b2c3d4e5f"},"deadline":"2026-09-25T12:00:00Z"}
```

```json
{"v":1,"id":"rq-00000009","verb":"dive.end","args":{"turn":"ot-9c0d1e2f","nonce":"ZGl2ZWRpdmVkaXZlZGl2ZWRpdmVkaXZlZGl2ZWRpdmVk","exit":"ok","code":0,"durationMs":742000}}
```

```json
{"event":"dive.stop","turn":"ot-9c0d1e2f","reason":"owner"}
```

The three blocks are `dive-begin.json`, `dive-end.json` and `dive-stop.json`. `dive.end` is
accepted by task 16's socket only from the runner's pid, like `turn.end` (`task-27.md:45-51`);
the interactive child, whose pid differs, cannot end its own DIVE, and never sees the nonce.

#### `sandboxd/internal/runner/dive.go` (complete public surface)

```go
package runner

// DiveArgv is the interactive command line: no -p, no --output-format,
// never --fork-session. It is NOT Argv (task 27's always adds -p).
func DiveArgv(claude string, b control.DiveBegin) ([]string, error) {
	if !uuidRE.MatchString(b.Session.ID) {
		return nil, ErrBadSession
	}
	switch b.Session.Mode {
	case "resume":
		return []string{claude, "--resume", b.Session.ID}, nil
	case "new":
		return []string{claude, "--session-id", b.Session.ID}, nil
	}
	return nil, ErrBadSession
}

// dive runs one DIVE while parked: PrepareHome, Preflight, then the child
// on the runner's own stdin/stdout/stderr (the PTY), in its own process
// group, with ChildEnv(os.Environ(), home) plus the PTY's TERM. It returns
// when the child exits, the deadline passes (SIGTERM, SIGKILL 10 s later,
// exit "timeout"), or a dive.stop for this turn arrives (the same kill,
// exit "ok"). Then it sends dive.end with the nonce, retried as turn.end is.
func (r *Runner) dive(ctx context.Context, b control.DiveBegin) control.DiveEnd
```

`Run` accepts `dive.begin` **only while parked**; one arriving mid-turn (impossible if Fleet's
claim holds, so a sign of a bug) is answered with a `dive.end` of `exit: failed`, `code: -3`, and
the turn is untouched. While the DIVE child runs, `Run` keeps calling `events.wait` on its own
goroutine for `dive.stop` only; any `turn.begin` arriving then is refused the same way, `code: -3`.

#### `control-plane/internal/fleet/turns/dive.go` (complete public surface)

```go
package turns

var (
	ErrBusy       = errors.New("turns: a turn or a DIVE is already in flight")
	ErrNotRunning = errors.New("turns: the orchestrator has no live sandbox")
)

// DiveState is what GET …/dive and task 55's panel show. Waiting is a count
// of queued events, a fact, not a fraction (OD-27).
type DiveState struct {
	Active  bool      `json:"active"`
	Turn    string    `json:"turn,omitempty"`  // ot-…
	Since   time.Time `json:"since,omitempty"`
	Waiting int       `json:"waiting"`         // team events queued behind it
}

// BeginDive claims a turn with Origin human, EventIDs empty, Mode resume on
// the current uuid (new if none), Until now+DiveMax, and pushes dive.begin.
// ErrBusy if ClaimTurn loses (a turn in flight, or another DIVE); the owner
// retries when the turn ends. The claim is written through the outbox as
// the DIVE's start: an ot-… record, origin human, exit "dive-started".
func (s *Scheduler) BeginDive(ctx context.Context, orch model.AgentID) (DiveState, error)

// EndDive accepts the runner's dive.end: the claimed turn and its nonce,
// once, constant time (as End). It writes the ot-… record's end (origin
// human, Issued = every id RecordIssued saw in the DIVE, exit), deletes the
// claim, and wakes the scheduler so the queued events run at once.
func (s *Scheduler) EndDive(ctx context.Context, e wire.DiveEnd) error

// StopDive pushes dive.stop for the owner's DELETE, for the claim's Until,
// and for task 11's Revoker (reason access_removed). The runner ends the
// child; EndDive then closes the claim. A lapsed claim with no dive.end is
// swept like a lost turn (task 26 (d)), with no events to re-queue.
func (s *Scheduler) StopDive(ctx context.Context, orch model.AgentID, reason string) error
```

The DIVE's `ot-…` record is one record with two writes, start and end, through task 07's route 69
(`task-26.md:580-594`). `exit` gains two Fleet-only values, `dive-started` and `dive`, and the
record's `consumed` is empty (a DIVE consumes no event; the queued ones run after it). **Requires
of task 07**: route 69 accepts a second write to the same `ot-…` id when the first carried
`dive-started` (a revision, not a new record), or the start is its own `ot-…` and the end another;
this part prefers the first and lists it. Every directive issued inside a DIVE is `origin: human`,
because `CurrentTurn` returns the DIVE's claim (`task-31.md:42-45`) and task 31's gate lets a
human-origin `free` pass (`task-31.md:473`).

#### The routes, `server/dive.go`

All three through task 39's `RouteAgent` with `VerbSessionsOwn`, so a non-owner, a viewer, an
auditor and a missing agent all get the same `404 agent_not_found`; the agent must be of class
`orchestrator`, else the same 404.

```jsonc
// POST /v1/agents/{ag}/dive          → 202 DiveState {"active":true,"turn":"ot-…","since":…,"waiting":3}
//   409 orchestrator_busy (a turn in flight; details.retry "after_turn"), 409 agent_not_running
// GET  /v1/agents/{ag}/dive          → 200 DiveState
// DELETE /v1/agents/{ag}/dive        → 202 DiveState {"active":true,…} (dive.stop sent; the
//   runner's dive.end closes it); 404 when no DIVE is active
```

The owner then attaches to the **Orchestrator's** terminal through task 39's `GET
/v1/agents/{ag}/attach`, exactly as to a worker's. Starting a DIVE does not attach and attaching
does not start a DIVE: attached without a DIVE, the owner sees the parked runner's one fixed line,
*"Orchestrator parked. Start a DIVE from the panel to talk to it here."*; the runner writes it
once at start and after each turn and DIVE, beside task 27's status lines (its `slog` handler on
the PTY, `task-27.md:318`, never child output).

#### The `seed` verb (`verbs/seed.go`, complete result shape)

```go
package verbs

// SeedResult is a new session's typed world (decision d). Nothing in it was
// written by the old session: no turn reply, no message this Orchestrator
// sent, no transcript byte. Peer words appear only where `panacea team`
// already quotes them, in task 22's fence, attributed.
type SeedResult struct {
	Team        TeamOrchestrator `json:"team"`      // task 22 (task-22.md:218-224), as `panacea team`
	Threads     []SeedThread   `json:"threads"`     // open, by id; bodies via `panacea inbox`
	Escalations []SeedEsc      `json:"escalations"` // open es-… of this Orchestrator
	Policy      SeedPolicy     `json:"policy"`      // the owner's escalation rows (task 30)
	Human       []teamtext.Text `json:"human"`      // own human's last ≤ 5 hm-…, text·H only
	Rotated     string         `json:"rotated"`     // "" | "day" | "task-boundary" | "pod"
}

type SeedThread struct {
	ID          string `json:"id"`          // th-…
	Conflict    string `json:"conflict"`    // cf-… or ""
	State       string `json:"state"`       // open | escalated
	Counterpart string `json:"counterpart"` // the other Orchestrator's owner display name
	Round       int    `json:"round"`
}

type SeedEsc struct {
	ID    string `json:"id"`    // es-…
	Kind  string `json:"kind"`  // inform | decide
	State string `json:"state"` // open
}

type SeedPolicy struct {
	Floors []string `json:"floors"` // task 30's escalation floors in force
}
```

`Human` is filtered twice: author class `text·H` **and** author the Orchestrator's owner (task
02's `Text.Author`, `Text.Owner`); anything else is dropped, so a teammate's human text can never
reach another Orchestrator's seed. `Rotated` tells the new session why it is new, as a fixed enum.
`seed-result.json` is the fixture, with one of each.

`Rotated` is read from the **current claim**: `seed` is only ever called inside a turn (the new
session calls it first), so `Store.ClaimedTurn(orch)` holds the row, and `ClaimRow` gains `Rotated
string`, written at claim time by the hunk below. A `seed` with no live claim is `409
no_turn_in_flight` (an Orchestrator only runs inside a turn or a DIVE). The verb is registered for
the `orchestrator` class only in task 19's dispatch table, so a worker's call is task 19's
`forbidden` before this code runs.

#### Task 26's `scheduler.go` (modify: two anchored hunks)

Hunk 1 replaces the line `mode, uuid := SessionFor(prev, sandboxID, false) // task 40 supplies
rotateDue` (`task-26.md:558`) with:

```go
	pending, err := s.Store.RotatePending(ctx, orch)
	if err != nil {
		return false, err
	}
	why := RotateWhy(prev, sandboxID, pending, now)
	mode, uuid := SessionFor(prev, sandboxID, why == "day" || why == "task-boundary")
```

and the `ClaimRow` literal below it gains `Rotated: why,`. `RotateWhy` joins `rotate.go`:

```go
// RotateWhy names why this turn's session is new, or "" when it is not new
// or is the first session ever. "pod" is SessionFor's own sandbox rule
// (task 26: a changed sandbox never resumes); it is named, not decided, here.
func RotateWhy(prev SessionState, sandboxID string, pending bool, now time.Time) string {
	switch {
	case prev.UUID == "":
		return ""
	case prev.SandboxID != sandboxID:
		return "pod"
	case pending:
		return "task-boundary"
	case RotateDue(prev, false, now):
		return "day"
	}
	return ""
}
```

Hunk 2 is in `End`, where the `SessionState` passed to `EndTurn` is built: on `row.Mode ==
ModeNew` it sets `StartedAt: row.Began`; on resume it carries `prev.StartedAt` unchanged. `EndTurn`
gains one parameter, `clearPendingBefore time.Time` (zero on resume: nothing cleared), and runs
`UPDATE fleet.orch_sessions SET rotate_pending_at = NULL WHERE orchestrator = $1 AND
rotate_pending_at <= $2` inside its existing transaction. `LoseTurn` is unchanged: it clears the
session, so the next turn is `new` with `Rotated: ""`, which `seed` reports as a first session.
That undercounts one reason and is the safe side.

#### `00NN_rotation.sql` (complete; `NN` is the next Fleet migration number at merge)

```sql
-- ST.5 rotation (task 40). Three columns on task 26's two tables; no text
-- column that could hold a reply, a summary or a transcript byte.
ALTER TABLE fleet.orch_sessions
  ADD COLUMN IF NOT EXISTS started_at        timestamptz,
  ADD COLUMN IF NOT EXISTS rotate_pending_at timestamptz;

ALTER TABLE fleet.turn_claims
  ADD COLUMN IF NOT EXISTS rotated text NOT NULL DEFAULT ''
    CHECK (rotated IN ('', 'day', 'task-boundary', 'pod'));
```

`started_at` is null for a session that predates this migration; `RotateDue` treats a zero
`StartedAt` as an earlier date, so each such Orchestrator rotates once on its next turn, the safe
side. Task 26's `storetest.Suite` row that pins `turn_events` to ten columns is untouched (these are
other tables); the suite gains one row pinning `turn_claims.rotated`'s `CHECK` by inserting `'x'`
and expecting a violation.

**DIVE and revocation.** `turns` gains a fourth implementer of task 11's `Revoker` (`task-11.md:582-583`,
beside tasks 20, 23 and 39): `(*Scheduler).Revoke(ctx, d, why)` calls `StopDive(orch,
"access_removed")` when `d` is an Orchestrator's delegation and a DIVE is active, and does nothing
otherwise. Task 39's relay closes the owner's socket on the same revocation; the two are
independent, so the child is stopped even if the socket had already gone.

- [ ] **Step 1: Write the failing tests first**

Sandbox side, `package runner`: task 27's fakes (`Client`, `Child`, clock, `t.TempDir()` as
`root`), plus a fake `Child` that records the three descriptors it was started with.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestDiveArgvIsExact` | resume: `[claude --resume <uuid>]`; new: `[claude --session-id <uuid>]`, element by element | reuse of task 27's `Argv` (adds `-p`, `--output-format`); a fork builder |
| `TestDiveArgvNeverForks` | both modes: no element is `--fork-session`, `--continue`, `-c`, `-p`, or starts with `--dangerously` | a "safe default" fork; a headless DIVE |
| `TestDiveArgvRefusesBadSession` | uuid `../x`, uppercase, mode `fork`, mode `""`: each `ErrBadSession`, no child | the uuid check skipped for DIVE |
| `TestDiveChildOwnsThePTY` | the child's stdin, stdout and stderr are the runner's own three (`os.Stdin`…), not pipes | the runner reads the conversation through a pipe (OD-8) |
| `TestDiveEnvIsTheAllowlistPlusTerm` | parent env with both tokens, `ANTHROPIC_API_KEY`, `TERM=xterm-256color`: child env is exactly `HOME=<session home>`, `PATH`, `TERM=xterm-256color` | the parent env passed through; `TERM=dumb` kept (an unusable interactive UI) |
| `TestDiveNonceNeverReachesTheChild` | argv and env of the child hold no substring of `Nonce` or `Turn` | the nonce passed for correlation |
| `TestDiveOnlyWhileParked` | `dive.begin` while a turn's child runs: `dive.end` `failed`, `code -3`, the turn's child unsignalled and its `turn.end` sent as normal | a DIVE started over a running turn |
| `TestTurnBeginDuringDiveRefused` | a `turn.begin` while the DIVE child runs: refused `code -3`, no second child | two children on one PTY |
| `TestDiveStopKillsTheGroup` | `dive.stop` for this turn: `SIGTERM`, `SIGKILL` 10 s later, `dive.end` `ok`; a `dive.stop` naming another turn: ignored | stop ignored; stop honoured for any turn id |
| `TestDiveDeadlineIsTimeout` | the child never exits: killed at `Deadline`, `dive.end` `timeout`, `Code -1` | no deadline in the runner |
| `TestDiveEndRetried` | `End` answers `unavailable` twice: three calls, then parked again | a lost `dive.end` leaving Fleet's claim until the sweep |
| `TestEachSessionHasItsOwnHome` | two turns on uuid A then one on B: `HOME` is `SessionHome(root, A)` twice, then `…/B` | one shared `HOME` (the old transcript survives) |
| `TestPrepareHomeNewMustNotExist` | mode `new` with `…/<uuid>/home` present: turn refused `code -2`, no child | a reused uuid silently resuming old state |
| `TestPruneAfterStartOnly` | `new` on B with A's home present: A gone after B's child started; B's start fails: A still present | pruning first, losing the resumable session on a failed start |
| `TestPruneLeavesWhatItDidNotMake` | under `root`: a non-uuid directory, a uuid-named **symlink** to a temp dir with a file in it, a uuid-named file: all three left, the symlink target's file intact | `RemoveAll` of every entry; following a symlink out of `root` |
| `TestPruneNeverReadsInsideHome` (Linux) | A's home holds a FIFO and a mode-`0000` file: `PruneHomes` returns within 1 s and A is gone | any "summarise before delete" that opens a file (it blocks on the FIFO) |

Fleet side, `package turns`, `verbs` and `server`: task 26's fakes (memstore, `Streams` recording
every push, fake clock, fake Hub route 69), task 12's fake Entra and `Membership`. Alice (`u-a`)
owns Orchestrator `ag-oa` on `pj-1`; Bob (`u-b`) is a member of `pj-1`; Adam (`u-ad`) its admin;
Alice's worker is `ag-a1`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRotateDueOnTheSydneyDate` | `StartedAt` 2026-09-25T13:59Z, now 14:01Z: true (23:59 → 00:01 local); `StartedAt` 2026-09-24T14:30Z, now 2026-09-25T13:00Z: false (both the 25th local) | UTC dates, which answer the opposite in both cases |
| `TestRotateDueAcrossDST` | `StartedAt` 2026-10-03T14:30Z; now 2026-10-04T12:30Z: false; now 13:30Z: true (AEDT, +11, from 4 October) | a fixed +10 offset in place of the zone |
| `TestRotateDueRules` | empty uuid: false; pending: true; zero `StartedAt`: true | pending ignored; a pre-migration session never rotating |
| `TestRotateWhyNamesEachCause` | changed sandbox: `pod`; pending: `task-boundary`; new day: `day`; same day, same pod: `""`; first ever: `""` | a reason that disagrees with `SessionFor`'s mode |
| `TestTryBeginRotatesDaily` | session A started yesterday (local): the pushed `turn.begin` has `mode new` and a uuid ≠ A; the claim's `rotated` is `day` | `rotateDue` still hard-wired `false` |
| `TestBoundaryMakesNextTurnNew` | `MarkBoundary(ag-oa)`, then an event: `mode new`, `rotated task-boundary`; the turn after resumes | the flag never set, or never cleared |
| `TestBoundaryDuringTheNewTurnSurvives` | `MarkBoundary` at `Began + 1 s` of the `new` turn, then `End`: the next turn is `new` again | clearing the flag unconditionally at `End` |
| `TestLostNewTurnKeepsTheFlag` | a `new` turn swept `lost`: the flag is still set, and the next turn is `new` | clearing the flag at claim time |
| `TestSeedHoldsNoOldSessionByte` | the canary `C4N4RY-old` placed in the last human-origin turn's `Reply`, in a `ms-…` body `ag-oa` sent, in a `ms-…` body it received and in an escalation's text it wrote: `json.Marshal(SeedResult)` has no occurrence | any field fed from a reply, a sent message or an agent-written text |
| `TestSeedFieldsAreTheList` | reflection over `SeedResult`, `SeedThread`, `SeedEsc`, `SeedPolicy`: the JSON names are exactly those fenced above | a later `summary` or `lastReply` field |
| `TestSeedHumanIsOwnHOnly` | Hub fake holds Alice's `text·H` ×7, Bob's `text·H`, an `text·A` authored as Alice: `Human` is Alice's last five `text·H`, newest last | Bob's words in Alice's seed; agent text laundered as human |
| `TestSeedNeedsATurn` | `seed` with no live claim: `409 no_turn_in_flight`; from worker `ag-a1`: task 19's `forbidden` | a seed callable outside a turn, or by a worker |
| `TestDiveClaimsAsHuman` | `BeginDive(ag-oa)`: one claim, origin `human`, no event ids, mode `resume` on the current uuid, `Until` = now + 2 h; `CurrentTurn` returns it with origin `human` | a DIVE outside the claim (the scheduler would start a turn beside it) |
| `TestTurnEventDuringDiveWaits` | during a DIVE, three team events and one `human.message` are appended: `Tick` pushes no `turn.begin`, `DiveState.Waiting` is 4; after `EndDive`, the next `Tick` pushes one `turn.begin` with all four | events run beside a DIVE, or are dropped behind it |
| `TestDiveNeverRotates` | session started yesterday, pending set: `BeginDive` pushes `mode resume` on the same uuid | a DIVE into a fresh, empty conversation |
| `TestDiveBusy` | a turn in flight: `409 orchestrator_busy`; a second `BeginDive` during a DIVE: `409 orchestrator_busy`; no live pod: `409 agent_not_running` | two claims, or a DIVE into nothing |
| `TestDiveRecordsStartAndEnd` | route 69's fake sees two writes to one `ot-…`: first `origin human`, `consumed []`, `exit dive-started`; then `exit dive`, `issued` = the `dv-…` recorded during it | a DIVE with no audit record, or only an end |
| `TestDiveDirectiveIsHumanOrigin` | a directive issued through task 31's gate during the DIVE carries `origin: human` and is listed in the end record's `issued` | a DIVE's directive recorded as agent-origin (it would be gated as peer-driven) |
| `TestEndDiveNonceOnce` | a wrong nonce: rejected, claim kept; the right one: claim gone; the same `dive.end` replayed: `turn_mismatch`, no second record | a DIVE ended by anyone who knows the `ot-…` id; a double end record |
| `TestStopDiveReasons` | `DELETE …/dive`: `dive.stop` reason `owner`; the claim's `Until` passing: `deadline`; task 11's `Revoke` on Alice's Orchestrator delegation: `access_removed`; `Revoke` of an unrelated delegation: no push | a DIVE that outlives its owner's access |
| `TestLapsedDiveSwept` | no `dive.end` and the clock past `Until` + 5 s: `SweepLapsed` removes the claim, writes the end `exit lost`, and the queued events run at the next `Tick` | a runner crash leaving the Orchestrator claimed for ever |
| `TestBobCannotDive` | Bob's valid token on `POST /v1/agents/ag-oa/dive`: `404 agent_not_found`, no claim, no push | an ownerless DIVE |
| `TestAdminCannotDive` | Adam, admin: the same 404 (OD-15: nobody else's agent, not even an admin) | "admin sees everything" |
| `TestDiveNotYoursEqualsNotThere` | Bob on `ag-oa`, Alice on `ag-zz`, Alice on her **worker** `ag-a1`: identical status, `Content-Type` and body bytes | an id-existence or class oracle |
| `TestDiveFixturesRoundTrip` (both modules) | each of `dive-begin.json`, `dive-end.json`, `dive-stop.json`, `seed-result.json` decodes and re-encodes byte for byte; the same with one extra field is refused | the two copies drifting; a lax decoder |

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/sandboxd && GOPROXY=off GOFLAGS=-mod=vendor go test ./internal/runner/... ./internal/control/... 2>&1 | head -3
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/turns/... ./internal/fleet/verbs/... ./internal/fleet/server/... 2>&1 | head -3
```

Expected: build failures, `undefined: DiveArgv` and `undefined: RotateDue` (or the first missing
name in each module).

- [ ] **Step 2: The wire and the fixtures, both modules**

`wire/dive.go` and `control/dive.go` as fenced; the four fixtures under
`testdata/panacea-control/v1/`; `TestDiveFixturesRoundTrip` passes in both modules.

- [ ] **Step 3: The runner: `rotate.go`, `dive.go`, then task 27's two files**

`ChildEnv(parent, home)` in `argv.go` (every task 27 caller passes `SessionHome(root, uuid)`; task
27's `TestChildEnvIsTheAllowlist` now expects that `HOME`); then `Run`'s `dive.begin` branch and the
`PrepareHome`/`PruneHomes` calls. Every sandbox test above passes; task 27's still pass.

- [ ] **Step 4: Fleet: `rotate.go`, the two scheduler hunks, the migration and the store methods**

`memstore` and `pgstore` gain `SetRotatePending`, `RotatePending`, `StartedAt`, the conditional
clear in `EndTurn`, and `ClaimRow.Rotated`; `storetest.Suite` gains the flag-survives row and the
`CHECK` row, so both stores pass one table. Then task 24's `done`/`abandoned` transitions call
`MarkBoundary` after their outbox write.

- [ ] **Step 5: `dive.go`, the routes, the `seed` verb, the `Revoker`, and wiring**

In `cmd/fleetd/main.go`: register `dive.end` and `seed` in task 19's dispatch table for the
`orchestrator` class only; `srv.RouteAgent("POST /v1/agents/{ag}/dive", server.VerbSessionsOwn,
…)` and its `GET` and `DELETE`; add the `Scheduler` to task 11's `Revoker` fan-out; `StopDive` on
`Until` runs on the deadline-sweeper singleton beside `SweepLapsed`.

- [ ] **Step 6: Acceptance**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/sandboxd && GOPROXY=off GOFLAGS=-mod=vendor go test ./... -count=1 && GOPROXY=off GOFLAGS=-mod=vendor go vet ./... && GOOS=linux CGO_ENABLED=0 GOFLAGS=-mod=vendor go build -o /dev/null ./cmd/panacea-orchestrator
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/... ./cmd/fleetd/... -count=1 -race && GOPROXY=off go vet ./internal/fleet/... ./cmd/fleetd/... && gofmt -l internal/fleet cmd/fleetd
```

Expected: `ok` for every package, `-race` clean, `go vet` silent, `gofmt -l` prints nothing, the
Linux build succeeds. On Windows `TestPruneNeverReadsInsideHome` is skipped by build tag and runs in
task 32's Linux CI job. The Postgres half of `storetest.Suite` runs where
`PANACEA_APP_FLEET_TEST_PG_DSN` is set. Then three guards. The DIVE command line carries no
headless or fork flag (prints nothing):

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && grep -nE '"(--fork-session|-p|--print|--output-format|--continue|-c)"' sandboxd/internal/runner/dive.go
```

The runner's two new files read one directory listing and nothing else:

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && grep -cE "os\.(Open|OpenFile|ReadFile|ReadDir)|filepath\.(Walk|WalkDir|Glob)|io\.ReadAll" sandboxd/internal/runner/rotate.go sandboxd/internal/runner/dive.go
```

Expected: `sandboxd/internal/runner/rotate.go:1` (the `os.ReadDir(root)`) and
`sandboxd/internal/runner/dive.go:0`. And the seed verb reads no agent-written text (prints
nothing):

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && grep -nE "\.Reply\b|\.Body\b|[Tt]ranscript|\.Summary\b" control-plane/internal/fleet/verbs/seed.go
```

The end-to-end case, a real Orchestrator rotating overnight and a real owner diving into it from
the desktop, is task 44's; this task stops at the fakes and the Linux child test.

- [ ] **Step 7: Commit**

```bash
git add sandboxd/internal/runner/rotate.go sandboxd/internal/runner/dive.go sandboxd/internal/runner/rotate_test.go sandboxd/internal/runner/dive_test.go sandboxd/internal/control/dive.go control-plane/internal/fleet/wire/dive.go control-plane/internal/fleet/turns/rotate.go control-plane/internal/fleet/turns/dive.go control-plane/internal/fleet/turns/rotate_test.go control-plane/internal/fleet/turns/dive_test.go control-plane/internal/fleet/verbs/seed.go control-plane/internal/fleet/verbs/seed_test.go control-plane/internal/fleet/server/dive.go control-plane/internal/fleet/server/dive_test.go control-plane/internal/fleet/store/migrations testdata/panacea-control/v1
git commit -F <msgfile> -- sandboxd/internal/runner sandboxd/internal/control/dive.go control-plane/internal/fleet/wire/dive.go control-plane/internal/fleet/turns control-plane/internal/fleet/verbs control-plane/internal/fleet/server/dive.go control-plane/internal/fleet/server/dive_test.go control-plane/internal/fleet/store control-plane/cmd/fleetd/main.go testdata/panacea-control/v1
```

`server/` is committed file by file, not as a folder, because tasks 12 and 39 own the rest of it.

**What this task does NOT claim.**
- **That a rotated session knows nothing the old one learnt.** It knows what Fleet's typed state
  holds and nothing else. A fact the old session held only in its transcript is gone, which is the
  point; the cost is that a new session re-reads the world through `seed`, `team` and `inbox`.
- **That rotation bounds cost by itself.** It bounds the transcript that every turn re-reads. The
  turn budget is task 37's; nothing here meters or caps a DIVE's tokens, which are the owner's own
  conversation and are attributed to the Orchestrator's gateway key like any turn (task 20).
- **That `--resume` finds the transcript under the per-session `HOME`.** Claude Code keeps session
  files under the user's home today; that it does so on the pinned image, and that nothing else it
  needs lives there, is **UNVERIFIED**. The managed policy is not in `HOME`: it is
  `/etc/claude-code/managed-settings.json` (`task-13.md:21`), so a fresh `HOME` loses no policy.
- **That the runner understands the DIVE.** It starts a child and waits. It does not read, parse
  or summarise what the owner and the Orchestrator say; the conversation's audit is the `ot-…`
  record's start and end and the ids issued between them, not its words.
- **A deadline without a race.** The runner kills the child at `Deadline`, which equals the
  claim's `Until`; if the 5-second sweep runs first, the end record says `lost`, not `timeout`,
  and the runner's late `dive.end` gets `turn_mismatch`, which it treats as done. Both outcomes
  free the Orchestrator; only the label differs.
- **Anything on the desktop.** Task 55's panel shows `DiveState` and the `Waiting` count; task 21's
  `fleetbe` calls the three routes. Neither is built here.

**Gaps against the skeleton, raised here and not resolved alone:**
1. **The file map is sandbox-only.** It lists `sbx/internal/runner/{rotate.go,dive.go}`, but the
   rotation decision and the DIVE claim are Fleet's (decisions a and e): `turns/rotate.go`,
   `turns/dive.go`, `verbs/seed.go` and `server/dive.go` should join the skeleton's map.
2. **Task 07's route 69 must accept two writes to one `ot-…`** (start `dive-started`, then end
   `dive`), and those two `exit` values. If it will not, the start becomes its own `ot-…` and the
   end another, and `TestDiveRecordsStartAndEnd` changes to two ids.
3. **Task 26's interfaces grow**: `SessionState.StartedAt`, `ClaimRow.Rotated`, `EndTurn`'s
   `clearPendingBefore`, and `Store.SetRotatePending`/`RotatePending`. Task 26's part should carry
   them or name this task as their owner.
4. **Task 24 must call `MarkBoundary`** on `done` and `abandoned`, after its outbox write; its scope
   does not say so.
5. **First run in a fresh `HOME`.** An interactive `claude` in a new `HOME` may show first-run or
   folder-trust screens that the headless `-p` turns never see. Whether the pinned image's managed
   settings suppress them, or the owner simply answers them, needs the same probe task 38 asks for
   (task 33's TM1-α run is the natural place). Nothing here writes Claude Code's own config files.
6. **Task 21 and task 55 consume the DIVE routes**; their scopes should name `POST`/`GET`/`DELETE
   /v1/agents/{ag}/dive` and `DiveState` so the three stay one contract.

