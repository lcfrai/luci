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
from the Hub). No `ms-…` body, no turn reply, nothing an agent wrote. Peer text still reaches it
only through `panacea inbox`, quoted (§7.1).

**(e) DIVE is a claimed turn with origin `human`, so nothing else can run.** `POST
/v1/agents/{ag}/dive` claims a turn through task 26's `ClaimTurn` (the two-replica guard) with
origin `human`, `Mode: resume` on the current uuid (or `new` if there has never been one), and
`Until = now + DiveMax` (2 h). The claim is what makes Fleet's scheduler start nothing while the
owner is inside (`task-26.md:565-566`), so team events queue exactly as they do behind any turn,
and `CurrentTurn` returns the DIVE's `TurnRef` with origin `human`, which is the hook task 31
left for this (`task-31.md:41-44`). Rotation is never due inside a DIVE: the owner dives into the
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
- `control-plane/internal/fleet/store/migrations/00NN_rotation.sql` — two columns on task 26's
  session state (new, numbered after the last Fleet migration at merge)
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
  - task 22: the team view (`TeamResult`); task 28: `Store.OpenThreads` (`task-28.md:366`);
    task 30: the owner's policy rows; task 07: the human messages read (`hm-…`);
  - task 31: `CurrentTurn`'s origin drives the directive's `origin` (`task-31.md:132-133`).

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
	return s.Store.SetRotatePending(ctx, orch, true)
}

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("turns: " + err.Error()) // unreachable with time/tzdata embedded
	}
	return loc
}
```

`SessionState` gains `StartedAt time.Time` and `Store` gains `SetRotatePending(ctx, orch, bool)`
and `RotatePending(ctx, orch) (bool, error)`; `End` on a `new` turn writes `StartedAt = Began` and
clears the flag in the same transaction as the session row (task 26's `EndTurn`). A lost `new`
turn keeps the flag set, so the next turn is `new` again, which is the safe side.

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
// with one os.RemoveAll. It reads directory NAMES under root only: it never
// opens, lists or stats anything inside a session's HOME (OD-8).
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

