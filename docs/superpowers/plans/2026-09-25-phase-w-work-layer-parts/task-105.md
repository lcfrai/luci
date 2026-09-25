### Task 105: Fleet: §3.7 rules 2 and 3 through Orchestrators; cross-owner `blocks` seeds a `sequence` proposal

Slice **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on task
98 (`task.issueId`, `issue.delegated`, `blocker.landed`), Phase T's TM-26 (the turn queue,
`turns.Queue.Enqueue`), TM-29 (`protocol.Action`, `Rules`) and TM-30 (`escalate.Table`, `KeyOf`).
Uses SW.5's `issue.related` and `issue.updated` (task 13, `task-13.md:120-170`), SW.19's org switch
(task 99) and task 65's reassign *decide*. Every Phase T path is `cp/`→`control-plane/` and
**UNVERIFIED until `feat/team-mode` is sync-merged here** (rule H2).

05 §3.7 says two things that only work if the right Orchestrator hears them:

1. **Rule 3: an edit across owners is a proposal, never an act.** When Alice adds *EP-150 blocks
   EP-142* and Dylan's agents are on EP-142, nothing pauses. Fleet tells **Alice's** Orchestrator,
   the editor's own, and her Orchestrator opens a `sequence` proposal on a Phase T thread with
   Dylan's (TM-05 §6.2, §6.4). Dylan's agents keep running until his side accepts.
2. **Rule 2: a hand-over winds down the former owner's agents, through the former owner's
   Orchestrator.** When Dylan answers task 65's *hand over the worktree to Alice*, Fleet tells
   **Dylan's** Orchestrator, which stops his workers at their next checkpoint (`panacea worker
   stop`, TM task 18's verb row `task-18.md:144`). Alice's Orchestrator is told nothing: nobody
   steers another person's agents (`OD-16`).

**Decisions made here, so nobody re-derives them:**

1. **"Turn origin" is Phase T's word for who started a turn, and has two values.** TM-26's
   `turns.Origin` is `human` or `fleet` (`task-26.md:158-163`). A relation or a hand-over is a
   Fleet-started turn, so its origin is `fleet`, and the scope's *origin `relation`* and *origin
   `handover`* are **event kinds**: `work.relation` and `work.handover`, appended to TM-09's
   `model.EventKinds` beside task 98's two. That is also what makes rule 7 of Phase T hold: in a
   Fleet-started turn an Orchestrator may only propose, inform and ask (Phase T plan `:148`).
2. **The editor's Orchestrator is seeded, and only when both sides have agents.** Seeded when
   the actor is the assignee of one side, the other side's assignee is someone else, and both
   assignees have a live task bound to their issue (task 98's `issueId`). Otherwise Fleet does
   nothing and the Hub's recorded relation reaches the other owner as task 65's Desk *inform*.
   An actor who is neither side's assignee seeds nothing: two other people's agents are theirs.
3. **`relates` seeds nothing; `blocks` asks for `sequence`; `duplicates` asks whose worktree
   survives.** The event says which; the Orchestrator's turn template (TM-26) does the asking.
   The resolution a `duplicates` thread reaches is always a *decide* for both humans: one row
   appended to TM-30's table, **locked** (`OD-24`'s always-decide), keyed on one added
   `protocol.Action.Relation` field that `resolve.go` fills from the thread's two issues.
4. **A hand-over is read from the assignee moving, not from a new Hub record.** Task 65's reassign
   record is itself a gap there (no Hub route yet), and a declined reassign never moves the
   assignee. So `issue.updated` with `assignee` in `fields` (`task-13.md:150-152`), on an issue
   that has live tasks bound to an owner who is no longer its assignee, is the hand-over; each such
   owner's Orchestrator gets one `work.handover`. The new assignee's Orchestrator gets nothing.
5. **The switch is read on every event, and fails closed.** SW.19's `AgentSwitch` off, or a failed
   read, enqueues nothing and returns the reason; the relation stays recorded on the Hub (it is a
   person's edit). Task 109 later puts its cached read behind the same interface.
6. **Events carry ids only.** `Ref` is `relation:<kind>:<from is->:<to is->` or the bare `is-…`;
   no title or text, per TM-09's rule that an event carries no text (`task-09.md:237-241`).
7. **The wake is task 98's.** After a `sequence` resolution, the waiting side is woken by task 98's
   `blocker.landed` when the first issue lands; this task adds no second path for it.

**Files:**
- `control-plane/internal/fleet/work/relations.go` (+ `relations_test.go`) — `OnRelated` (new;
  **path UNVERIFIED**: the skeleton's file map has no Fleet row for W3's deltas)
- `control-plane/internal/fleet/work/handover.go` (+ `handover_test.go`) — `OnAssigneeChanged` (new)
- `control-plane/internal/fleet/model/events.go` ⚑ (TM-09; task 98 also appends) — two kinds (modified)
- `control-plane/internal/fleet/protocol/{messages.go,resolve.go}` ⚑ (TM-28, TM-29) — `Action.Relation`,
  filled through one registered hook (modified)
- `control-plane/internal/fleet/escalate/rules.go` ⚑ (TM-30) — one locked row and one `KeyOf` case
  (modified; `rules_work_test.go` new)
- `control-plane/internal/fleet/delegation/feed.go` ⚑ (TM-11; task 98 maps its kinds here) — the two
  handlers called from the feed reader (modified)
