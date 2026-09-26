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
