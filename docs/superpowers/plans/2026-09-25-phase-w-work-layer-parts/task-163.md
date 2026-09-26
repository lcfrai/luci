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
