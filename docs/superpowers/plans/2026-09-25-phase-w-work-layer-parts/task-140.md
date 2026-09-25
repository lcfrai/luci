### Task 140: drafts, part 2: *propose issues per criterion*; *propose the next cycle*

W4, control plane and desktop, about 3 hours (an estimate, not a measurement). Depends on task 87
(`CycleView`, `CycleConvergence`; **scope only, no part on this tree**, `…-scopes/tasks-81-100.md:165-184`)
and task 136 (the assist client; **scope only**, `tasks-121-140.md:473-513`). Every name taken from
either is **UNVERIFIED** and their parts win at build. Reads task 80's `df-` draft routes and task
72's `Draft{id, kind, issueId, body AuthoredText, updatedAt, expiresAt}` (scope,
`tasks-61-80.md:411-412`, `665-676`), task 77's `SetCycleRequest{cycleId *string}` and `PUT
/v1/issues/{issueId}/cycle` (scope, `tasks-61-80.md:417`, `568`), task 03's `CreateIssueRequest`
(`task-03.md:420-430`), task 56's `mutate` batch (`task-56.md:47-48`), task 62's Desk registration
(SW.17, scope `tasks-61-80.md:72-99`, as task 127 uses it, `task-127.md:397-436`), phase 4's
`Criterion{ID, Text}` (`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover-parts/task-03.md:114-117`)
and task 139's `Service` usage of 136's `Complete`.

Closes **L-82**'s W4 half (*"propose issues, one per open criterion"*; Discover's half exists) and
**L-66** (the proposed next cycle), with the solo-mode caveat below.

**What the reader sees** (05 §1 D L-66, §1 E L-82, §4.5 Discover row, AX §7.1):

- **A feature's criteria panel gains *Propose issues*.** It is enabled when at least one
  criterion is *uncovered* (no issue serves it, §3.3) and reads *"Propose issues for 3 uncovered
  criteria"*. Pressed, it shows one draft card per uncovered criterion: the criterion's text as the
  card's head, a drafted title and description marked *drafted* in the agent class, and *"Serves
  C3"* in mono. Each card has *Accept*, *Edit* and *Discard*; the list has *Accept all*.
- **Drafts survive a restart.** Each card is a `df-` draft in the Hub, private to the person, so
  closing the app and returning shows the same cards (the Hub keeps them; the desktop keeps
  nothing on disk, `ADR-H1:467-474`).
- **Accept** creates the issue in `triage` **as the person**, with `serves` set to that card's
  criterion, its title and description still `text·A` until someone edits them, and deletes the
  draft. **Accept all** is one batch with one Undo chip: *"3 issues created · Undo"*.
- **The cycle view gains *Propose the next cycle*.** The proposal arrives as **one** Desk item in
  NEEDS YOU on the next Desk entry: *"Proposed for Cycle 14: 7 issues across 3 criteria"*. Opened in
  place, it lists issues **grouped by the criteria they serve** (a group *"Serves no criterion"*
  last), each with a checkbox (checked by default), its priority word, its due date when set and
  its blocker when one exists, and each with a *why* line whose keys are links.
- **No forecast anywhere.** The proposal never says *would settle*, *fits*, *capacity*,
  *velocity* or a percentage; the group header is a fact, *"Serves C3 · 2 issues"*.
- **Accepting the cycle** sets the cycle of each checked issue as the person, one Undo for the set.

**Decisions made here, so nobody re-derives them:**

1. ***Uncovered* is computed mechanically, never by the model.** A criterion of the feature's
   branch brief head is uncovered when no issue of the feature's project that the caller can read,
   in any state but `cancelled`, has a `serves` entry naming it (§3.3: *"a criterion no issue serves
   is a visible gap, named uncovered"*). The model is asked only for words.
2. **`serves` is set by code, never taken from the model.** The prompt lists uncovered criteria as
   `k1`…`kN`; the reply is keyed the same. The draft's `serves` is the criterion the key names, so a
   crossed or invented criterion is impossible by construction. A key not in the list is dropped;
   a key answered twice keeps the first answer; a criterion the model skipped gets **no** card and
   the panel says *"No draft for C4. Write one yourself."*
3. **The draft's body carries the proposal.** Task 72's `Draft` has one `body` field and no title
   or `serves` (gap 1). Until it gains them, a proposal draft is `kind: "issue"` whose
   `body.text` is the canonical JSON of `ProposedIssueDraft` (below), `author: "A"`, and whose
   `issueId` is `null`. `drafts.ts` (task 92) decodes that shape by its `"v": "proposal-1"` marker
   and shows every other issue draft as before.
4. **Assist routes write nothing to the Hub.** The control plane returns proposals; the desktop
   writes the `df-` drafts, the issues and the cycle moves, **as the person**, through task 37's
   client and task 56's `mutate`. So every Hub write has the person as its actor and the assist
   service holds no Hub write path at all (the same rule as task 139).
5. **The backlog read for a cycle proposal** is the team's issues the caller can read in `triage`,
   `backlog` or `todo`, with no cycle or a past cycle, at most 200 by priority then oldest (task
   77's states; `started` and later are already under way, `done` and `cancelled` are closed).
   The prompt carries each issue's alias, title, priority, due date, blocker aliases and the
   criteria it serves. **No estimate, velocity, capacity or history** is sent or computed.
6. **Grouping is mechanical.** The model returns `{issues: [{key, why: [key]}]}`; the control
   plane keeps only candidate keys, then groups the kept issues by their own `serves` (an issue
   serving two criteria appears under each, marked *also serves C2*).
7. **One Desk item per proposal, held until the next entry.** The proposal is held in memory by a
   Desk source (task 127's `reminders.ts` pattern); task 62's frame takes its snapshot on entry, so
   an item made while the Desk is open appears on the next entry and never re-ranks the open Desk.
   A new proposal for the same cycle replaces the older one.
8. **Accept all creates, and Undo cancels.** Issue creation is not a replica op (task 56's
   `WorkOp` has no create kind, `task-56.md:157-168`). *Accept all* creates the issues one by one
   through the client; its Undo is one `mutate([...])` batch of `setState` to `cancelled` with
   cause `undo`, and the drafts are written back from memory. The aliases stay consumed (gap 4).
9. **Solo mode.** The assist client makes the cycle proposal. 05 §1's *"the user's Orchestrator
   proposes"* needs team mode, and this row has no TM dependency, so the Orchestrator path is
   **UNVERIFIED** here and not built.
