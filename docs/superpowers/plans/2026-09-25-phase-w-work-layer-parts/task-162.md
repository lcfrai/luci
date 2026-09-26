### Task 162: loops `lp-`, part 1: the record and its versions; event and schedule triggers in the janitor, as turns of the owner's Orchestrator within their grid, billed to them

W5, stream WH plus one Phase T delta, about 3 hours (an estimate, not a measurement). Depends on
task 145 (the W5 tables `work_loops`, `work_loop_versions`, `work_loop_runs` and the guards,
`…-scopes/tasks-141-160.md:249`), Phase T's TM-26 (the turn queue, origin and the fixed template,
`phase-t-team-mode-parts/task-26.md` on `feat/team-mode` at `43b6801c`, cited `T26:N`) and TM-37
(budgets, `task-37.md`, cited `T37:N`). It extends task 121's janitor passes
(`task-121.md:192-247`). **No part exists for tasks 143 or 145** (checked 2026-09-26), so the table
shapes below are this task's requirement of task 145 and the wire is this task's proposal to
task 143, which wins on names when it lands (SW.21). Task 169 already mirrors this wire on the
desktop from this task's scope (`task-169.md:195-245`), and task 172 draws it
(`task-172.md:81-160`); this part keeps their field names. Every `server/`, `store/` and Fleet path
is **UNVERIFIED until merge** (rules H1, H2).

Parity: closes, jointly with tasks 163 and 172, **L-160** (loops: event or schedule triggers,
`05-design.md:322`) and **L-161** (versions and the run log). Design: 05 §2.2, §3.2 (`:715`:
*"trigger (event or schedule), the owner, its autonomy bound, versions, run log"*), §3.6; AX §7
(`docs/AGENTIC-UX.md:175-185`); OD-16 (a person's agents are theirs), OD-19 (billing), OD-20 (no
Orchestrator in local mode).

**What the reader does.** Dylan saves *Morning triage* (task 172): every weekday at 08:30,
Australia/Sydney, action *run a turn*, commit at ASK FIRST. The Hub stores loop `lp-…` at
version 1. At 08:30 the janitor writes one `work.loop_fired`; Fleet, subscribed to the org's feed,
puts one `loop.fired` event on Dylan's Orchestrator queue; the next turn reads *1 team event. Run
`panacea inbox` to read it.* (`T26:265-275`), counts against Dylan's turn budget, and bills his
gateway token. The Hub was down over the weekend: Monday fires once, not three times.

**Decisions made here, so nobody re-derives them:**

1. **The Hub records and fires; it never runs anything.** A loop is a Hub record and a feed
   event. The turn is Fleet's (TM-26), on the owner's Orchestrator, with the owner's gateway
   token (OD-19). No Hub process holds an agent credential, and no loop is a free agent.
2. **Versions are append-only, twice over.** `work_loop_versions` has no `UPDATE` or `DELETE`
   grant, and a trigger `hub.forbid_change()` refuses both even as the owner role (the Hub's
   other logs do the same, `hub-contract.md` §10.7). A save is `POST …/versions`; the head's
   `current_version` moves by one in the same transaction.
3. **Only the owner writes a version; a team admin may pause.** OD-16. `PATCH {state:"paused"}`
   is the owner's or a team admin's; `PATCH {state:"on"}` is the owner's only, so an admin can
   stop a teammate's loop but never restart what its owner may have stopped for a reason.
4. **The owner's grid is not stored anywhere yet, so the bound fails closed.** Phase T's plan and
   parts name no per-person autonomy grid (searched `feat/team-mode` at `43b6801c` for *autonomy
   grid*, *ask_first*, *ASK FIRST*: no hit). Until one exists, the Hub's `OwnerGrid` answers
   `full` for `read` and `ask_first` for every other class, and a version with any level above it
   is 422 `loop_bound_exceeds_owner` naming the class. This matches task 172's decision 10
   (`task-172.md:152-155`): a narrower cap than the real one is safe, a wider one is not. When
   Phase T stores the grid, `OwnerGrid` reads it and nothing else changes (gap 2).
5. **A trigger names feed kinds, never loop kinds.** `eventKinds` must each be in
   `LoopEventKinds` (task 13's W1 kinds and task 169's `W5_FEED_KINDS`, copied by name), and
   `work.loop_fired`, `work.loop_*` and `team.*` are refused, so one loop cannot fire another
   (task 172's decision 9). 422 `loop_trigger_invalid`, `details.kind`.
6. **`action` is `turn` or `start`, nothing else.** `move`, `comment` or any other word is 422
   `loop_action_invalid`: a loop never writes state (SW.3). `start` requires `labelId` and a
   trigger of the shape task 163 names; its behaviour is task 163's.
7. **The janitor fires at most once per `(loop, version, cause)`.** The unique key is
   `(loop_id, version, event_seq)` for an event and `(loop_id, version, scheduled_for)` for a
   schedule, so a restart or a second janitor replica never fires twice; the insert is `ON
   CONFLICT DO NOTHING` and the event is written only when a row was inserted.
8. **A missed schedule fires once.** The janitor computes the **latest** due occurrence not
   after `now`; if it is later than the loop's `last_scheduled_for`, it fires that one only. Three
   missed weekdays fire Monday's 08:30 once (task 121's no-burst rule, `task-121.md:64-68`).
9. **A janitor act names its author.** `work.loop_fired` has `actor: null` and `data.author` =
   the owner's user id (05 §3.6, as task 121's decision 5 applies it).
10. **A loop does not outlive its owner's access.** Before firing, the step checks the owner is
    still a member of the loop's team; if not, it pauses the loop with `pausedCause:
    "owner_left"` and writes `work.loop_paused {loopId, cause}` instead, `actor: null`. A
    `team.member_removed` event (`hub-contract.md:1141`) is therefore acted on within one sweep.
11. **No loop text enters a turn.** The event's `data` holds ids and numbers only. The turn input
    stays `turns.Input(n)` (`T26:265-275`); the Orchestrator reads the instruction through task
    163's `panacea loop show`, as data.

**Files:**
- `control-plane/internal/hub/hubapi/work_loops.go` (+ `work_loops_test.go`) — the wire below, proposed to task 143 (new, or merged into 143's `work_intake.go` if it has landed)
- `control-plane/internal/hub/store/work_loops.go` (+ `work_loops_integration_test.go`) — heads, versions, firings (new)
- `control-plane/internal/hub/server/work_loops.go` — five routes, the bound check (new)
- `control-plane/internal/hub/server/janitor_loops.go` — `passLoops`, called after task 121's `sweepWorkW4` (new)
- `control-plane/internal/hub/server/janitor.go` ⚑ (Hub task 24's; one call and two `SweepReport` counters)
- `control-plane/internal/hub/server/loopsched.go` (+ `loopsched_test.go`) — the pure occurrence rule (new)
- `control-plane/internal/hub/server/work_loops_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — `workMatrixRows["162"]`
- `control-plane/internal/fleet/model/events.go` ⚑ (TM-09) — one kind, `EventLoopFired` (Phase T delta, rule H2)
- `control-plane/internal/fleet/turns/deliver.go` ⚑ (TM-26's deliverable-event table; its file name is TM-26's) — one row

No migration: task 145 creates the three loop tables (`tasks-141-160.md:249`). The columns this
task needs are under *The tables*, a requirement of task 145, plus one table 145's list does not
name, `work_loop_cursors` (gap 1).

**What is on the tree today (verified 2026-09-26).** On `feat/work-layer` no `control-plane/internal/hub`
or `internal/fleet` directory exists; parts exist for tasks 13, 121, 169 and 172. On `feat/team-mode`
(`43b6801c`) TM-09's `model.EventKind` constants are at `task-09.md:213-221` and TM-26's `turns`
package at `task-26.md:146-275`; TM-26's deliverable kinds table is `T26:117-126`, and its
files list names `queue.go`, `origin.go`, `template.go`, `session.go`, `scheduler.go`
(`T26:63-70`), no separate delivery file, so the row lands where TM-26 keeps its kind table
(gap 3). On `feat/hub` (`3df9fab8`) `team.member_removed` is `hub-contract.md:1141`; Hub task
24's janitor is **UNVERIFIED until merge**.

#### The wire — `hubapi/work_loops.go` (complete; task 143 wins on names)

```go
package hubapi

// ActionClass and AutonomyLevel are AX §7's rows and columns (docs/AGENTIC-UX.md:179),
// as task 169 mirrors them (task-169.md:196-200). Order matters: it is the rank.
var ActionClasses = []string{"read", "edit_own_worktree", "edit_outside_worktree", "run_tests",
	"install_dependency", "run_migration", "touch_infra", "commit", "push", "deploy"}
var AutonomyLevels = []string{"ask_first", "propose_and_wait", "act_and_tell", "full"}

type LoopBound struct {
	TurnsPerDay int               `json:"turnsPerDay"` // 1-24
	Levels      map[string]string `json:"levels"`      // class → level; absent = the owner's own
}

type LoopFilter struct {
	LabelID   *string `json:"labelId"`   // lb-…
	FromState *string `json:"fromState"` // an IssueState (task 03)
	ToState   *string `json:"toState"`
}

type LoopSchedule struct {
	Every string `json:"every"` // "day" | "weekday" | "week"
	At    string `json:"at"`    // "HH:MM", 24 h, local to TZ
	TZ    string `json:"tz"`    // IANA, loaded with time.LoadLocation (time/tzdata embedded)
	// "week" fires on the local weekday of the version's savedAt (decision 12).
}

type LoopTrigger struct {
	Kind       string        `json:"kind"`       // "event" | "schedule"
	EventKinds []string      `json:"eventKinds"` // event: 1-10 of LoopEventKinds; schedule: []
	Filter     LoopFilter    `json:"filter"`
	Schedule   *LoopSchedule `json:"schedule"`   // schedule only
}

type Loop struct {
	ID             string  `json:"id"` // lp-…
	TeamID         string  `json:"teamId"`
	ProjectID      string  `json:"projectId"`
	Owner          User    `json:"owner"`
	Name           string  `json:"name"`  // 1-80
	State          string  `json:"state"` // "on" | "paused"
	PausedCause    *string `json:"pausedCause"` // "owner_left" | "owner" | "admin"
	CurrentVersion int     `json:"currentVersion"`
}

type LoopVersion struct {
	LoopID      string       `json:"loopId"`
	Version     int          `json:"version"`
	Trigger     LoopTrigger  `json:"trigger"`
	Instruction AuthoredText `json:"instruction"` // task 03's; author "H" (the owner), ≤ 2000 runes
	Action      string       `json:"action"`      // "turn" | "start"
	LabelID     *string      `json:"labelId"`     // required for "start" (task 163)
	Bound       LoopBound    `json:"bound"`
	SavedAt     time.Time    `json:"savedAt"`
	SavedBy     User         `json:"savedBy"`
}
```

```go
// Request bodies. The Hub stamps loopId, version, savedAt and savedBy.
type LoopVersionBody struct {
	Trigger     LoopTrigger  `json:"trigger"`
	Instruction AuthoredText `json:"instruction"`
	Action      string       `json:"action"`
	LabelID     *string      `json:"labelId"`
	Bound       LoopBound    `json:"bound"`
}
type CreateLoopRequest struct {
	Name  string          `json:"name"`
	First LoopVersionBody `json:"first"` // version 1; the caller becomes owner
}
type SetLoopStateRequest struct {
	State string `json:"state"` // "on" | "paused"
}
type LoopWithVersion struct {
	Loop    Loop        `json:"loop"`
	Current LoopVersion `json:"current"`
}

// LoopEventKinds is the closed list a trigger may name (decision 5): task
// 13's W1 kinds and task 169's W5 feed kinds, by name, never work.loop_*.
// A test reads task 13's workevents.go and fails on a kind listed here that
// it does not define.
var LoopEventKinds []string

// Codes appended to task 143's list.
const (
	CodeLoopNotFound          = "loop_not_found"           // 404
	CodeLoopBoundExceedsOwner = "loop_bound_exceeds_owner" // 422, details.class, details.max
	CodeLoopTriggerInvalid    = "loop_trigger_invalid"     // 422, details.field
	CodeLoopActionInvalid     = "loop_action_invalid"      // 422, details.action
	CodeLoopPaused            = "loop_paused"              // 409 (task 163's run now)
)

// Event kinds, in the feed (W5_FEED_KINDS, task 169).
const (
	EventLoopCreated      = "work.loop_created"       // subject lp-; data {version:1}
	EventLoopVersionSaved = "work.loop_version_saved" // data {version}
	EventLoopStateSet     = "work.loop_state_set"     // data {state, cause}
	EventLoopFired        = "work.loop_fired"         // actor null; data below
	EventLoopPaused       = "work.loop_paused"        // actor null; data {cause:"owner_left"}
)
```

`work.loop_fired`'s `data` is `{loopId, version, trigger: "event"|"schedule", eventSeq?: int,
scheduledFor?: "RFC 3339 UTC", author: "u-…"}`: ids, integers and one time string, as §8 allows
(`hub-contract.md:1166`). Task 143's scope names the grid on the head as one level word and
`LoopRun` with an `lr-` prefix (`tasks-141-160.md:168-171`); this part follows task 162's own
scope and task 169, as task 169's G1 recorded (`task-169.md:247-253`); task 143 decides once.
`work.loop_paused` is a separate kind from `work.loop_state_set` because its actor is null and
its cause is the janitor's, not a person's.

#### The tables — required of task 145 (complete)

```sql
-- In task 145's NNNN_work_intake.sql; the four tables get task 116's RLS form.
CREATE TABLE work_loops (
  id text PRIMARY KEY CHECK (id LIKE 'lp-%'), org_id text NOT NULL, team_id text NOT NULL,
  project_id text NOT NULL, owner_id text NOT NULL, name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
  state text NOT NULL CHECK (state IN ('on','paused')), paused_cause text
    CHECK (paused_cause IN ('owner','admin','owner_left')),
  current_version int NOT NULL CHECK (current_version >= 1),
  last_scheduled_for timestamptz,          -- decision 8's high-water mark
  CHECK ((state = 'paused') = (paused_cause IS NOT NULL))
);
CREATE TABLE work_loop_versions (
  org_id text NOT NULL, loop_id text NOT NULL REFERENCES work_loops(id), version int NOT NULL,
  body jsonb NOT NULL,                     -- LoopVersionBody, canonical (hub-contract §3.6)
  saved_at timestamptz NOT NULL DEFAULT now(), saved_by text NOT NULL,
  PRIMARY KEY (loop_id, version)
);
CREATE TRIGGER work_loop_versions_append_only BEFORE UPDATE OR DELETE ON work_loop_versions
  FOR EACH ROW EXECUTE FUNCTION hub.forbid_change();
CREATE TABLE work_loop_runs (             -- a firing is a run row; task 163 adds its outcome columns
  org_id text NOT NULL, loop_id text NOT NULL REFERENCES work_loops(id),
  run int NOT NULL, version int NOT NULL,
  trigger_kind text NOT NULL CHECK (trigger_kind IN ('event','schedule','manual')),
  event_seq bigint, scheduled_for timestamptz, request_id text,   -- request_id: task 163's run now
  fired_at timestamptz NOT NULL DEFAULT now(),
  outcome text NOT NULL DEFAULT 'queued',  -- task 163's six words; 162 writes only 'queued'
  PRIMARY KEY (loop_id, run),
  CHECK ((trigger_kind = 'event') = (event_seq IS NOT NULL)),
  CHECK ((trigger_kind = 'schedule') = (scheduled_for IS NOT NULL))
);
CREATE UNIQUE INDEX work_loop_runs_event ON work_loop_runs (loop_id, version, event_seq)
  WHERE event_seq IS NOT NULL;
CREATE UNIQUE INDEX work_loop_runs_sched ON work_loop_runs (loop_id, version, scheduled_for)
  WHERE scheduled_for IS NOT NULL;
CREATE TABLE work_loop_cursors (org_id text PRIMARY KEY, after_seq bigint NOT NULL DEFAULT 0);
GRANT SELECT, INSERT ON work_loops, work_loop_versions, work_loop_runs, work_loop_cursors TO hub_app;
GRANT UPDATE (state, paused_cause, current_version, last_scheduled_for) ON work_loops TO hub_app;
GRANT UPDATE (after_seq) ON work_loop_cursors TO hub_app;
```

A firing **is** a run row, `outcome 'queued'`, so the two unique indexes are the no-double-fire
guarantee (decision 7) and task 163 adds the rest of the run log to the same table, never a
second one. `run` is `coalesce(max(run), 0) + 1` under the loop head's row lock (`SELECT … FOR
UPDATE` on `work_loops`), so runs number 1, 2, … per loop with no gap. `work_loop_cursors` is not
in task 145's list (gap 1). `hub.forbid_change()` is 0001's (`hub-contract.md:1799-1812`).

12. **A weekly loop keeps the weekday it was saved on.** `every: "week"` fires on the local
    weekday of the version's `savedAt` in `tz`; a new version saved on another weekday moves it.
    There is no weekday picker in task 172, so none is invented here.

#### The routes — `server/work_loops.go` (contract complete)

```go
// POST  /v1/projects/{projectId}/work/loops   {name, first}  member or admin of the project's team
//        → 201 LoopWithVersion; the caller is owner; state "on"; events loop_created (+ nothing else)
// GET   /v1/projects/{projectId}/work/loops                   any team member → 200 []Loop
// GET   /v1/work/loops/{loopId}                               any team member → 200 LoopWithVersion
// GET   /v1/work/loops/{loopId}/versions                      any team member → 200 []LoopVersion, oldest first
// POST  /v1/work/loops/{loopId}/versions  LoopVersionBody     OWNER only → 201 LoopVersion (n+1)
// PATCH /v1/work/loops/{loopId}  {state}                      owner: on|paused; team admin: paused only
//        → 200 Loop; a no-op (same state) writes nothing
// A viewer (V) reads, never writes. A team admin's POST …/versions is 403 even on their own
// team's loop (OD-16): the loop is its owner's agent turn, not the team's.
func (s *Server) routesWorkLoops(mux *http.ServeMux)

// validateVersion runs before any write, in this order, each a 422:
//  1. action ∉ {"turn","start"}              → loop_action_invalid
//  2. action "start" with labelId null        → loop_trigger_invalid, field "labelId"
//  3. trigger.kind "event": 1-10 eventKinds, each in LoopEventKinds; schedule nil
//     trigger.kind "schedule": eventKinds empty; every/at/tz valid; LoadLocation(tz) ok
//                                             → loop_trigger_invalid, field named
//  4. instruction.author != "H", or > 2000 runes, or empty
//                                             → loop_trigger_invalid, field "instruction"
//  5. bound.turnsPerDay ∉ 1..24               → loop_bound_exceeds_owner, class "turnsPerDay", max 24
//  6. any bound.levels[c] ranked above OwnerGrid(owner)[c], or c not an ActionClass, or a
//     level not an AutonomyLevel             → loop_bound_exceeds_owner, class c, max the owner's
func validateVersion(b hubapi.LoopVersionBody, owner hubapi.User, grid OwnerGrid) error

// OwnerGrid is the owner's own AX §7 grid. Decision 4: until Phase T stores one,
// FailClosedGrid is the only implementation (read = full, every other = ask_first).
type OwnerGrid func(ctx context.Context, userID string) (map[string]string, error)

func FailClosedGrid(context.Context, string) (map[string]string, error)
```

A person's own message to their Orchestrator is the only thing TM-37 lets past the turn budget
(`T37:13-14`); a loop's turn is `fleet` origin and counted like any other (decision 13, below),
so `turnsPerDay` is a second, per-loop cap under the owner's hourly one, never a way round it.

#### The occurrence rule — `server/loopsched.go` (complete)

```go
package server

// LatestDue is the latest occurrence of s at or before now, in s.TZ, or
// ok=false if none has come yet since from. "day": every local day at At;
// "weekday": Monday-Friday at At; "week": the weekday of anchor (the version's
// savedAt, in TZ) at At. A local time in a DST gap runs at the first real
// minute after it; one repeated by a DST fold runs once, at its first instant
// (task 121's rule, `task-121.md:121-162`). from is the version's savedAt, so a
// new version never fires for a window before it existed.
func LatestDue(s hubapi.LoopSchedule, anchor, from, now time.Time) (at time.Time, ok bool)
```

The janitor fires `at` only when `at` is after `work_loops.last_scheduled_for` (decision 8), then
sets `last_scheduled_for = at` in the same transaction. Three missed weekday windows give one
`LatestDue` answer, so one firing.

#### The janitor pass — `server/janitor_loops.go` (contract complete)

```go
package server

// passLoops runs in the org's janitor transaction after task 121's sweepWorkW4
// (task-121.md:197-207), under the same advisory lock and hub.org_id setting.
//
//  1. Owners who left: for each "on" loop whose owner is no longer a member of
//     its team, UPDATE state='paused', paused_cause='owner_left'; emit
//     work.loop_paused {loopId, cause:"owner_left"}, actor nil (decision 10).
//  2. Events: read the org's feed rows with seq > work_loop_cursors.after_seq,
//     at most 1000, in seq order, through the Hub's own feed read (§7.11). For
//     each row and each "on" loop in the row's project whose current version
//     is an event trigger naming the row's kind and whose filter matches
//     (below): fire(loop, version, eventSeq=row.seq). Then set after_seq to the
//     last row read, in the same transaction.
//  3. Schedules: for each "on" loop whose current version is a schedule,
//     at, ok := LatestDue(...); if ok && at > last_scheduled_for:
//     fire(loop, version, scheduledFor=at); UPDATE last_scheduled_for = at.
//
// fire: lock the head (SELECT … FOR UPDATE); n := runs in the last 24 h for the
// loop; INSERT a run row (next run number) ON CONFLICT DO NOTHING on the two
// unique keys; if no row was inserted, stop (decision 7). If n >= turnsPerDay,
// set its outcome 'refused_bound' and emit work.loop_capped; else emit
// work.loop_fired. Both: actor nil, data.author = owner (decision 9).
func passLoops(ctx context.Context, tx *Tx, org string, now time.Time, r *SweepReport) error

// SweepReport (Hub task 24's struct, already widened by task 121) gains:
//	LoopsFired, LoopsCapped, LoopsPausedOwnerLeft int
```

**The filter.** `fromState`/`toState` compare to the SW.4 state-move event's `data` (task 10's
`issue.state`, `{from, to, cause, proof, landCommit}`, `task-10.md:247`);
`labelId` is checked against the issue head's labels **at match time**, read in the same
transaction, because no event's `data` carries labels. A null filter field matches anything. A
loop sees only rows of its own project (`row.projectId == loop.projectId`), so a loop cannot
learn of another project's events.

`work.loop_fired`'s `data` also carries `run` (the run row's number), so Fleet's ref is
`lp-…#<run>` and task 163's outcome names the same row. `work.loop_capped {loopId, version, run,
author}` is not delivered to any Orchestrator; the owner sees it in task 172's run log as
*refused: bound*.

#### The Phase T delta (rule H2; the owner phase's files win)

```go
// control-plane/internal/fleet/model/events.go (TM-09, task-09.md:213-221), one constant appended:
EventLoopFired EventKind = "loop.fired"
```

TM-26's deliverable-kinds table (`T26:117-126`) gains one row, and its code the same mapping:

| Kind | Raised by | Immediate? | Origin it can produce |
|---|---|---|---|
| `loop.fired` | task 162's janitor, reached through Fleet's per-org feed subscription (TM-01 (d)): a Hub `work.loop_fired` becomes one event on the Orchestrator of `data.author` in `projectId`, `Ref = "lp-…#<run>"` | no, coalesces | `fleet` |

13. **Billed to the owner, by construction.** The event lands on the owner's own Orchestrator
    queue, so the turn runs with that Orchestrator's per-turn gateway key (TM-20, `T37:3-4`) and
    is admitted by TM-37's gate like any fleet event: counted in the owner's sliding hour and
    refused when it is spent, queued and the owner told (`T37:13-20`). The origin is `fleet`,
    never `human`: `OriginOf` makes `human` only from the owner's own `human.message`
    (`T26:12-13`), and a loop the owner wrote yesterday is not the owner speaking now. That the
    usage row carries the loop id through task 135's tag is **UNVERIFIED** (phase 6).

If the owner has no Orchestrator in that project, whether Fleet holds the event until one starts
is TM-26's to say and is **UNVERIFIED** here (gap 3). Either way nothing is lost silently: the
run row stays `queued`, and task 163's janitor rule closes it `failed` after 24 h, which the
owner sees in the run log.

#### Tests to write first, part 1 — pure (`server/loopsched_test.go`, `hubapi/work_loops_test.go`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWeekdayScheduleSkipsTheWeekend` | Friday 09:00 → Friday 08:30; Sunday → Friday 08:30; Monday 08:31 → Monday 08:30 | `weekday` read as `day` |
| `TestThreeMissedWindowsAreOneAnswer` | from Friday 08:30 to Wednesday 09:00, `LatestDue` is Wednesday 08:30, one value | a list of every missed window |
| `TestAWeeklyLoopKeepsItsSavedWeekday` | saved on a Tuesday in `Australia/Sydney`: due on Tuesdays only | a fixed Monday |
| `TestTheDSTGapRunsAtTheFirstRealMinute` | `at: "02:30"` on Sydney's spring-forward day → 03:00 local | a skipped day at the gap |
| `TestNoWindowBeforeTheVersionExisted` | from = savedAt 09:00, now 09:10, `at: "08:30"` → `ok == false` | a new version firing for yesterday |
| `TestLoopEventKindsHasNoLoopKind` | no entry starts `work.loop` or `team.`; every entry is defined in task 13's `workevents.go` or task 169's W5 list | a loop chain; a kind retyped wrongly |
| `TestFailClosedGrid` | `read` → `full`; every other class → `ask_first` | a guessed wider cap |

#### Tests to write first, part 2 — `server/work_loops_integration_test.go` (`//go:build integration`)

Each seeds a team with Ada (admin), Dylan (member, owner) and Val (viewer), one project, and
drives the janitor with task 121's `RunJanitorOnce` at a fixed `now`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestOneFiringPerMatchingEvent` | two `on` event loops on `issue.state`, one matching filter; three matching moves and one sweep → three `work.loop_fired` for the matching loop, none for the other | fires per loop per tick; filter ignored |
| `TestARestartReplaysNoFiring` | sweep, reset `after_seq` to 0 by hand, sweep again → no new run row, no new event | no unique key |
| `TestMissedDailyWindowsFireOnce` | a daily loop, the janitor idle for three days, one sweep → one run, `scheduledFor` today's 08:30 | a catch-up burst |
| `TestAPausedLoopNeverFires` | paused by Dylan; a matching event and a due window → nothing | pause ignored |
| `TestOnlyTheOwnerWritesAVersion` | Ada's `POST …/versions` → 403; Val's → 403; Dylan's → 201, version 2, `currentVersion` 2 | an admin edits another's loop (OD-16) |
| `TestAnAdminPausesButNeverResumes` | Ada `PATCH paused` → 200, `pausedCause "admin"`; Ada `PATCH on` → 403; Dylan `PATCH on` → 200 | an admin restarts a loop its owner stopped |
| `TestVersionsCannotBeUpdated` | as `hub_app`, `UPDATE work_loop_versions SET body = …` → error from `forbid_change` | the guard only in Go |
| `TestAMoveActionIs422` | `action: "move"` → 422 `loop_action_invalid`; no row, no event | a loop writes state (SW.3) |
| `TestABoundAboveTheGridIs422` | `levels.commit: "act_and_tell"` → 422 `loop_bound_exceeds_owner`, `class "commit"`, `max "ask_first"`; `turnsPerDay: 25` → 422 | the owner's grid not checked |
| `TestAJanitorFiringNamesItsAuthor` | the event's `actor` is null and `data.author` is Dylan's id | an unattributed "system" act |
| `TestAnOwnerWhoLeftPausesTheLoop` | Dylan removed from the team; one sweep → `paused`, `pausedCause "owner_left"`, one `work.loop_paused`, and a matching event in the same sweep fires nothing | a loop outlives its owner's access |
| `TestTheCapRefusesAndSaysSo` | `turnsPerDay: 1`, two matching events → one `work.loop_fired`, one `work.loop_capped`, run 2 `refused_bound` | a cap that drops silently, or none |
| `TestEachWriteAppendsOneEvent` | create, version, pause, resume: one event each; a no-op `PATCH` none | an event missing, or two |
| `TestNoLoopTextInTheFeed` | every loop event's `data` over the run above has no string equal to or containing the instruction | loop text in the feed, so in a turn |

#### Tests to write first, part 3 — the Phase T delta (`fleet/turns`, TM-26's test file)

`TestALoopFiredEventReachesTheOwnersOrchestrator`: a `work.loop_fired` feed row for Dylan in
project P enqueues one `loop.fired` on Dylan's Orchestrator in P, none on Ada's (kills a
broadcast); `TestALoopTurnIsFleetOrigin`: a batch of one `loop.fired` has `OriginOf == fleet` and
`Input(1)` as its only input (kills a loop turn with human authority; loop text in the input).
