### Task 163: loops, part 2: the run log and *run now*; the first-pass triage loop, "on accept, Start" for a label

W5, stream WH with one Phase T delta, about 3 hours (an estimate, not a measurement). Depends on
task 24 (SW.10: Start's result `{issueKey, worktreeId, sessionId, branch, route, routeForced,
undo}`, `task-24.md:96-109`), task 162 (the loop record, `work_loop_runs` as the firing row,
`work.loop_fired`, `task-162.md`) and Phase T's TM-24 (`task create`, `worker spawn`,
`phase-t-team-mode-parts/task-24.md:67-106` on `feat/team-mode` at `43b6801c`, cited `T24:N`),
TM-05 (the delegated caller, cited `T05:N`), TM-19 (dispatch), TM-26 (`T26:N`) and TM-37
(`T37:N`). Task 101's verb pattern is followed (`task-101.md:81-95`). Task 98's part does not
exist, so `task.issueId` is its scope's and **UNVERIFIED**; every Fleet, `sandboxd`, `server/` and
`store/` path is **UNVERIFIED until merge** (rules H1, H2).

Parity: closes **L-168** (the first-pass triage loop, `05-design.md:330`); **L-160** and
**L-161** jointly with tasks 162 and 172. Design: 05 §2.2, §3.3, §3.5; L-161, L-168 (`:323`,
`:330`).

**What the reader does.** Dylan has a loop on the *Checkout* project: *when a person accepts an
issue labelled `quick-fix`, Start it*. Ada accepts EP-150 from Triage. The janitor fires run 7;
Dylan's Orchestrator runs `panacea loop show`, sees `action: start` on `issue:is-…@2`, creates a
task for it and spawns a worker. Task 172's run log reads *run 7 · event · ran · ot-5c…*, with
`s-…` and branch `ep-150-…` under it. EP-150 goes `started`, then `landed` when the lane lands.
It stays at `landed`: Ada clears it to `done`.

**Decisions made here, so nobody re-derives them:**

1. **A run has one final outcome, written once.** `queued` is the only non-final word. Fleet
   writes `ran`, `skipped_budget`, `skipped_paused` and `failed` through the outcome route as a
   **delegated caller** (TM-05); the Hub's janitor writes two with `actor: null`: `refused_bound`
   at firing (task 162's cap) and `failed` for a run still `queued` 24 h after `firedAt`. A second
   outcome on a closed run is 409 `loop_run_closed`. Every outcome write emits one
   `work.loop_run_closed {loopId, run, outcome}` in the same transaction.
2. **Fleet, not the model, fills `started`.** At the end of the turn that consumed a
   `loop.fired` event, Fleet lists the workers that turn spawned whose task carries the run's
   `issueId` (task 98), and posts them. A worker spawned for another issue is not listed; a
   sentence in the turn's reply is never read (OD-8).
3. **`started` in the cloud.** SW.10's shape is kept (`StartedLane {issueKey, sessionId, branch,
   route}`, `task-169.md:235`): `sessionId` holds the worker's `ag-…` id, the unit TM-24's
   `WorkerResult.Agent` names (`T24:103-106`); `branch` is the lane branch Fleet created;
   `route` the project's. `issueKey` is the revision the run pinned at firing, `@r`.
4. **A spent budget is recorded, not deferred, for a loop.** TM-37 queues a non-human event
   when the budget is spent and drops nothing (`T37:17-18`). A loop's turn is different: a stale
   *08:30 triage* run at 14:00 is a turn the owner did not ask for. So when TM-37's gate would
   hold a `loop.fired` event for budget, Fleet removes that one event, posts `skipped_budget`,
   and the owner is told as TM-37 tells them. This is a **delta to TM-37's gate** (gap 1):
   *nothing is dropped* becomes *nothing is dropped unrecorded*, for this kind only.
5. ***Run now* is a person's act and still pays the budget.** `POST …/runs {requestId}` by the
   owner writes a run with trigger `manual` and `work.loop_fired` with `actor` = the owner. Its
   Fleet event is still `loop.fired`, origin `fleet`: only the owner's own `human.message` makes
   a `human` turn (`T26:12-13`), so *run now* never skips TM-37 (the scope's rule).
6. **The Orchestrator learns what to do from a verb, as data.** The turn input stays
   `turns.Input(n)` (`T26:265-275`). `panacea loop show` returns the queued runs of this turn's
   `loop.fired` events, each instruction as TM-22's `Quoted` with class `text·H` and the owner as
   `by`, the same envelope task 101 uses for issue text (`task-101.md:105-116`).
7. **Start is only for the one triage shape.** A version with `action: "start"` must be exactly
   `trigger {kind:"event", eventKinds:["issue.state"], filter {fromState:"triage",
   toState:"todo", labelId: L}}` with `labelId == L`; anything else is 422 `loop_trigger_invalid`.
   The janitor fires it only for an `issue.state` event whose actor is a person acting directly
   (a non-null `actorUserId` and no `via`, TM-05's v2 event, `T05:736`), so an agent's delegated
   move never starts a lane.
8. **One live lane per issue.** `loop show` marks a start run `startable: false` with the live
   `ag-…` when the owner already has a live worker on that issue, and Fleet refuses a `worker
   spawn` for a task whose issue has one (`conflict`, a TM-24 delta, gap 2). The run closes `ran`
   with `started: []`.
9. **The loop never proves or closes.** `started` and `landed` are task 22's lane facts, written
   as the owner whose loop caused them; `done` stays a person's act (§3.3). No loop verb touches
   state or evidence.

**Files:**
- `control-plane/internal/hub/hubapi/work_loops.go` ⚑ (task 162) — `LoopRun`, `StartedLane`, the two bodies, codes, one kind
- `control-plane/internal/hub/store/work_loops.go` ⚑ (task 162) — runs: list, run now, close
- `control-plane/internal/hub/server/work_loops.go` ⚑ (task 162) — three routes; `validateVersion` gains decision 7
- `control-plane/internal/hub/server/janitor_loops.go` ⚑ (task 162) — the 24 h close; the person-only match for `start`
- `control-plane/internal/hub/server/work_loops_integration_test.go` ⚑ (task 162) — this task's rows, appended
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — `workMatrixRows["163"]`
- `control-plane/internal/fleet/verbs/loop.go` (+ `loop_test.go`) — `loop.show`, `CloseLoopRuns` (new)
- `control-plane/internal/fleet/model/verbs.go` ⚑ (TM-09) — one row, `loop.show`, Orchestrators only
- `control-plane/internal/fleet/control/dispatch.go` ⚑ (TM-19) — none if registration is `Handle` in `fleetd` (as task 101 found, `task-101.md:92-95`); else one call
- `control-plane/internal/fleet/turns/scheduler.go` ⚑ (TM-26) — one call to `CloseLoopRuns` at `turn.end`
- `control-plane/internal/fleet/budget/gate.go` ⚑ (TM-37's gate; its file name is TM-37's) — decision 4's one branch
- `sandboxd/internal/panaceacli/loop.go` (new), `verbs.go` ⚑ (TM-18) — `panacea loop show`, one table row
- `testdata/panacea-control/v1/result-loop-show.json` (new) — the fixture both modules read

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 24, 101, 162, 169 and
172; there is none for 22, 98 or 143. On `feat/team-mode` (`43b6801c`): `WorkerArgs` and
`WorkerResult` at `T24:94-106`; `delegation_required` at `T05:35` and `:253`, the v2 event's
`via: fleet` at `T05:736`; TM-37's rule 3 at `T37:17`. None of it is merged anywhere.

#### The wire — appended to task 162's `hubapi/work_loops.go` (complete; task 143 wins on names)

```go
// Outcomes. "queued" is the only non-final word (decision 1).
var LoopOutcomes = []string{"queued", "ran", "skipped_budget", "skipped_paused", "refused_bound", "failed"}

// StartedLane is SW.10's Start result as a run carries it (task-24.md:103-105;
// task-169.md:235). In the cloud SessionID holds the worker's ag-… (decision 3).
type StartedLane struct {
	IssueKey  string `json:"issueKey"` // issue:is-…@r
	SessionID string `json:"sessionId"`
	Branch    string `json:"branch"`
	Route     string `json:"route"` // "global" | "au"
}

type LoopRunTrigger struct {
	Kind         string     `json:"kind"`         // "event" | "schedule" | "manual"
	EventSeq     *int64     `json:"eventSeq"`     // event only
	ScheduledFor *time.Time `json:"scheduledFor"` // schedule only
}

// LoopRun is keyed by (loopId, run); no id prefix (task 163's scope; task
// 143's scope proposed lr-, task-169.md:247-253 records the conflict as G1).
type LoopRun struct {
	LoopID   string         `json:"loopId"`
	Run      int            `json:"run"` // 1, 2, … per loop, gapless (task 162)
	Version  int            `json:"version"`
	FiredAt  time.Time      `json:"firedAt"`
	Trigger  LoopRunTrigger `json:"trigger"`
	Outcome  string         `json:"outcome"`
	IssueKey *string        `json:"issueKey"` // start runs: the accepted issue at firing, @r
	TurnID   *string        `json:"turnId"`   // ot-…, Fleet's
	Started  []StartedLane  `json:"started"`  // [] unless a Start happened
	EndedAt  *time.Time     `json:"endedAt"`
}

type RunLoopNowRequest struct {
	RequestID string `json:"requestId"` // 8-64 of [A-Za-z0-9._-], as SW.10's (task-24.md:100)
}
type LoopRunOutcomeRequest struct {
	Outcome string        `json:"outcome"` // ran | skipped_budget | skipped_paused | failed
	TurnID  *string       `json:"turnId"`  // required for ran
	Started []StartedLane `json:"started"` // ran only; ≤ 10
}

const (
	CodeLoopRunClosed   = "loop_run_closed"    // 409: the run already has a final outcome
	CodeLoopRunNotFound = "loop_run_not_found" // 404
	EventLoopRunClosed  = "work.loop_run_closed" // data {loopId, run, outcome}; in the feed
)
```

**Columns required of task 145** on `work_loop_runs`, beyond task 162's: `issue_key text`,
`turn_id text CHECK (turn_id LIKE 'ot-%')`, `started jsonb NOT NULL DEFAULT '[]'`, `ended_at
timestamptz`, `CHECK (outcome IN (…the six…))`, `CHECK ((outcome = 'queued') = (ended_at IS
NULL))`, a unique index on `(loop_id, request_id) WHERE request_id IS NOT NULL`, and `GRANT UPDATE
(outcome, turn_id, started, ended_at)`. A trigger refuses any `UPDATE` whose `OLD.outcome <>
'queued'`, so a closed run cannot be rewritten even as `hub_app` (decision 1, two layers).

#### The routes — appended to `server/work_loops.go` (contract complete)

```go
// GET  /v1/work/loops/{loopId}/runs?after=&limit=      any team member → 200 []LoopRun, newest first;
//      after is a run number (exclusive, older than it), limit 1-100, default 50
// POST /v1/work/loops/{loopId}/runs  {requestId}       OWNER only → 201 LoopRun (queued, trigger manual);
//      a repeat requestId → 200 with the first run, no new event; a paused loop → 409 loop_paused;
//      emits work.loop_fired with actor = the owner (decision 5)
// POST /v1/work/loops/{loopId}/runs/{run}/outcome  LoopRunOutcomeRequest
//      a DELEGATED caller only (TM-05: Fleet's service token + X-Panacea-Delegation naming a
//      delegation whose owner is the loop's owner) → 200 LoopRun; no header → 401
//      delegation_required (T05:35); another owner's delegation → 403 forbidden; a closed run →
//      409 loop_run_closed; "ran" without turnId → 400 bad_request, field "turnId"; "queued" or
//      "refused_bound" → 400 (those are the Hub's words, decision 1)
//      emits work.loop_run_closed, actor = the owner, via = fleet (T05:736)
func (s *Server) routesWorkLoopRuns(mux *http.ServeMux)
```

`validateVersion` (task 162) gains decision 7's rule as its step 2: `action: "start"` must carry
exactly the triage shape, else 422 `loop_trigger_invalid` with `field` the first that differs.
`passLoops` (task 162) gains two things: step 2's match for a `start` version also requires the
row's actor be a person acting directly (decision 7), and the fired run's `issue_key` is set to
`issue:<subject>@<head revision>` read in the same transaction; and a new step 4, *close stale
runs*: every run `queued` with `fired_at < now − 24h` becomes `failed`, `ended_at = now`, one
`work.loop_run_closed`, `actor: null`, `data.author` the owner.

#### The verb — ST.1 request and result (complete; `testdata/panacea-control/v1/result-loop-show.json`)

```jsonc
// panacea loop show  →  {"verb":"loop.show","args":{}}          Orchestrators only (TM-09 row)
// result: the runs named by the loop.fired events of THIS turn's batch, oldest first
{ "runs": [
    { "loop": "lp-…", "version": 3, "run": 7, "action": "start",
      "issue": "issue:is-3f2a91c07b1e4d2a@2", "startable": true, "liveAgent": null,
      "instruction": { "text": "Start small fixes the team accepted; keep the lane to one file.",
                       "class": "text·H", "by": "Dylan Example", "owner": "Dylan Example" } },
    { "loop": "lp-…", "version": 1, "run": 12, "action": "turn", "issue": null,
      "startable": false, "liveAgent": null,
      "instruction": { "text": "Summarise what arrived in Triage overnight.",
                       "class": "text·H", "by": "Dylan Example", "owner": "Dylan Example" } } ] }
// a run whose loop is paused by the time of the turn is omitted, and closed skipped_paused;
// refusals: forbidden_class (a worker calls it) · unavailable {since} (CLI exit 3)
```

`instruction` is TM-22's `Quoted` (as task 101's `title`/`description`, `task-101.md:105-116`), so
TM-18's envelope quotes it like any person's words; `class` is always `text·H` because only the
owner writes a version (task 162). The scope's `{body, author:"human", owner}` maps to `text`,
`class: "text·H"`, `owner`: one envelope for all free text, not a second shape (gap 5).

```go
// control-plane/internal/fleet/verbs/loop.go (package verbs; interfaces complete)
const VerbLoopShow = "loop.show"

// LoopShow serves loop.show. It reads the caller's current TurnRef (TM-26's
// Scheduler.CurrentTurn, T26:92), the batch's loop.fired refs ("lp-…#<run>"),
// and for each: the run and its version from the Hub as the owner's delegation
// (hubclient.WithDelegation, task-101.md:23-24); startable is false when the
// owner has a live worker whose task.issueId is the run's issue (decision 8).
func LoopShow(deps LoopDeps) control.Handler

// CloseLoopRuns is called once at turn.end (TM-26's scheduler) for a turn whose
// batch held loop.fired events. For each run: "ran" with turnId and started =
// the workers this turn spawned whose task.issueId is the run's issue
// (decision 2), or "skipped_paused" if LoopShow omitted it. A lost turn that
// TM-26 re-queues is not closed; one lost twice is closed "failed".
func CloseLoopRuns(ctx context.Context, deps LoopDeps, turn turns.TurnRef, batch []turns.Queued, ok bool) error

type LoopDeps struct {
	Hub     hubclient.WorkIntake // ListLoopRuns, loop version read, PostLoopRunOutcome (task 143)
	Workers WorkerIndex          // task 98 / TM-24: live workers by owner and issueId
	Now     func() time.Time
}
```

#### The Phase T deltas (rule H2; the owner phase's files win)

**TM-37's gate.** TM-37 names its gate file `control-plane/internal/fleet/budget/budget.go`
(`Gate`, `Admit`, `T37:72`, `:212-254`, read on the team worktree at `997e5869`), not `gate.go` as
the Files list above guesses; the delta lands in `Admit`'s step 3 (`T37:244-248`), and the Files
row reads `budget/budget.go` ⚑ (gap 3). The one branch, in step 3's *spent* arm, before the
notice:

```go
// budget.go ⚑ (TM-37), Admit step 3, spent arm. Decision 4: a loop turn is recorded, not held.
if spent {
	var keep []turns.Queued
	for _, q := range pending {
		if q.Event.Kind == model.EventLoopFired { // task 162's constant
			g.LoopSkips.Skip(ctx, q.Event.Ref, "skipped_budget") // "lp-…#<run>"; one POST each
			continue
		}
		keep = append(keep, q)
	}
	// keep stays queued exactly as today; the notice below is unchanged (T37:246-247)
}
```

```go
// budget.go ⚑ (TM-37), one field on Gate and one interface; fleetd wires it to task 163's
// verbs.CloseLoopRuns path (the same hubclient, the owner's delegation).
type LoopSkipper interface {
	// Skip posts {outcome} for the run named by ref and removes the event from the
	// Orchestrator's pending queue (task 26's store), in that order. A Hub 409
	// loop_run_closed counts as done; any other error leaves the event queued.
	Skip(ctx context.Context, ref, outcome string) error
}
// Gate gains:  LoopSkips LoopSkipper // nil in tests that do not seed loop.fired
```

Step 1 (a human bypass) is untouched: a batch holding the owner's `human.message` admits a
`loop.fired` beside it, and that run closes `ran` at `turn.end`. Step 2 (a human pause) is
untouched too: a paused owner's loop runs wait, then close `failed` after 24 h (decision 1).

**TM-24's spawn** gains one refusal (decision 8, gap 2): `worker spawn` for a task whose
`issueId` (task 98, **UNVERIFIED**) already has a live worker of the same owner is refused
`conflict` with `{liveAgent: "ag-…"}`, before any sandbox is claimed. `WorkerIndex` below is the
read both use, so `loop show`'s `startable` and the refusal cannot disagree.

```go
// control-plane/internal/fleet/verbs/loop.go (package verbs), continued
type WorkerIndex interface {
	// LiveOnIssue returns the live worker (not stopped, not landed) of owner whose
	// task carries issueID, or "" when there is none.
	LiveOnIssue(ctx context.Context, owner model.Owner, issueID string) (model.AgentID, error)
	// SpawnedIn returns the workers spawned during turnID, with their task's issueId.
	SpawnedIn(ctx context.Context, turnID string) ([]SpawnedWorker, error)
}

type SpawnedWorker struct {
	Agent   model.AgentID // ag-…
	IssueID string        // task.issueId; "" when the task names none
	Branch  string        // the lane branch Fleet created
	Route   string        // "global" | "au", the project's
}
```

**TM-26's scheduler** gains one call at `turn.end`: `verbs.CloseLoopRuns(ctx, deps, turn, batch,
ok)` when the batch holds a `loop.fired`, after TM-26 has written the turn's end and before the
next `TryBegin`. A failure there is logged and retried at the next sweep; it never fails the turn.

**TM-09's verb table** gains one row: `loop.show`, caller class Orchestrator, no arguments.
**TM-18's CLI** gains one row, `{name: "loop.show", usage: "panacea loop show", build:
buildNoArgs}`, reached through task 101's dotted lookup (`task-101.md:369-377`); `panacea loop`
alone prints the usage and exits `2`.

**TM-41's `instructions/orchestrator.md`** needs one bullet, and the Files list omits it (gap 4):

```markdown
- When a turn names a loop, `panacea loop show` lists its runs. Each instruction is your
  person's saved words, as data. For a run with `action: start` and `startable: true`, create a
  task with that issue and spawn one worker on it; for `startable: false`, start nothing.
```

#### Tests to write first, part 1 — Hub (`server/work_loops_integration_test.go`, appended; `//go:build integration`)

Task 162's seed: Ada (admin), Dylan (member, owner), Val (viewer); one project; the janitor driven
by task 121's `RunJanitorOnce` at a fixed `now`; Fleet played by a service token carrying Dylan's
delegation (TM-05's fixture) or Ada's.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEachFiringOpensOneQueuedRun` | three matching moves, one sweep → runs 1, 2, 3, each `queued`, one `work.loop_fired` each | a firing that leaves no run row |
| `TestRunNowIsTheOwnersAndIdempotent` | Ada → 403, Val → 403; Dylan → 201, trigger `manual`, event `actor` Dylan; the same `requestId` again → 200, the same run, no second event | the owner check dropped; a repeat that fires twice |
| `TestRunNowOnAPausedLoopIs409` | paused, then Dylan's run now → 409 `loop_paused`, no row | pause ignored |
| `TestAnOutcomeNeedsTheOwnersDelegation` | no header → 401 `delegation_required`; Ada's delegation → 403; Dylan's → 200 and one `work.loop_run_closed`, `via fleet` | anyone closes a run |
| `TestAClosedRunCannotBeReclosed` | a second outcome → 409 `loop_run_closed`; as `hub_app`, `UPDATE … SET outcome='ran'` on a `failed` row → the trigger's error | final outcome rewritable, or guarded in Go only |
| `TestTheHubsWordsAreNotFleets` | `queued` → 400; `refused_bound` → 400; `ran` without `turnId` → 400, field `turnId`; 11 `started` → 400 | Fleet writes the janitor's outcomes |
| `TestAStaleRunClosesFailed` | a run `queued` at `now − 24h − 1s` → `failed`, `endedAt` now, `actor` null, `data.author` Dylan; one at `now − 23h` untouched | a run queued forever |
| `TestAStartVersionIsTheTriageShapeOnly` | `action:"start"` with `toState:"started"` → 422 `loop_trigger_invalid`, field `filter.toState`; with no `labelId` → 422, field `filter.labelId` | Start fired by any trigger |
| `TestAnAcceptWithoutTheLabelStartsNothing` | EP-151 without `quick-fix` accepted by Ada → no run | the filter ignored |
| `TestAnAgentsMoveFiresNoStart` | the same move as Fleet with Dylan's delegation (`via fleet`) → no run | an agent's move starts a lane |
| `TestAStartRunPinsTheRevision` | EP-150 at revision 2, accepted → run's `issueKey` = `issue:is-…@2` | an unpinned or head-moving key |
| `TestTheRunLogPagesNewestFirst` | 60 runs: default page 50, run 60 first; `after=11&limit=10` → runs 10…1 | oldest first; `limit` ignored |
| `TestALoopNeverClosesWork` | run 7 closed `ran` with EP-150's lane; task 22's lane fact *landed* → EP-150 `landed`, never `done`; no loop route writes state or evidence | a loop closes work |

#### Tests to write first, part 2 — Fleet (`fleet/verbs/loop_test.go`, `fleet/budget/budget_test.go` ⚑)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLoopShowReturnsTheInstructionAsData` | `instruction.class` `text·H`, `by` the owner; `turns.Input(n)` for the turn holds none of the instruction's bytes | loop text in the turn input |
| `TestLoopShowIsForOrchestratorsOnly` | a worker's call → `forbidden_class` | a worker reads its owner's loops |
| `TestStartableIsFalseWhileALaneLives` | a live `ag-…` on EP-150 → `startable:false`, `liveAgent` it | a double Start |
| `TestCloseListsOnlyTheRunsIssue` | the turn spawned two workers, EP-150 and EP-151; run 7 (EP-150) closes `ran`, `started` one lane, `sessionId` the `ag-…` | every spawned worker listed |
| `TestAPausedRunClosesSkippedPaused` | paused between firing and turn → omitted from `loop show`, closed `skipped_paused` | a paused loop's run executed |
| `TestALostTurnClosesFailedTheSecondTime` | lost once → still `queued`; lost again → `failed` | a lost turn closes `ran` |
| `TestASpentBudgetRecordsSkippedBudget` | spent budget; pending one `loop.fired` and one `peer.message` → `Skip(ref, "skipped_budget")` once, the peer event still queued, one notice | a loop run held silently |
| `TestRunNowStillPaysTheBudget` | a `manual` run's `loop.fired` under a spent budget → `skipped_budget`; `HumanBypass` false | run now bypasses the budget |
| `TestASecondSpawnOnALiveIssueIsConflict` | `worker spawn` on a task for EP-150 while `ag-…` lives → `conflict`, `liveAgent`; no sandbox claimed | two lanes for one issue |

#### Tests to write first, part 3 — CLI and fixture (`sandboxd/internal/panaceacli/loop_test.go`)

`TestLoopShowSendsTheDottedVerb` (request `{"verb":"loop.show","args":{}}`; kills a flat `loop`
verb), `TestLoopAloneIsUsage` (exit `2`; kills a default subcommand), `TestLoopFleetDownIsExitThree`
(canned `unavailable` → exit `3`; kills exit `1` on an outage), and
`TestLoopShowFixtureRoundTrips`, in both modules, decoding `result-loop-show.json` into each side's
type and re-encoding it byte-equal (kills a field renamed on one side).

#### The matrix rows (`workMatrixRows["163"]`, appended)

Columns as `hub-contract.md:337-339`. The loop's team is the row's team; `M` split by ownership.

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/work/loops/{loopId}/runs` | 200 | 200 | 200 | 404 | 404 | 404 |
| `POST /v1/work/loops/{loopId}/runs` (M owns it) | 403 | 201 | 403 | 404 | 404 | 404 |
| `POST /v1/work/loops/{loopId}/runs` (M does not) | 403 | 403 | 403 | 404 | 404 | 404 |
| `POST …/runs/{run}/outcome`, a person's own token | 401 | 401 | 401 | 404 | 404 | 404 |

The outcome route's delegated caller is not a column: its three rows are
`TestAnOutcomeNeedsTheOwnersDelegation` above. An app or key principal gets 403 on all three
writes (task 162's gap 4).
