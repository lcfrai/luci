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
