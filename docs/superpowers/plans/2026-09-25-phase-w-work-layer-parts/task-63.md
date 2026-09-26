### Task 63: the Desk: issue rows in NEEDS YOU, aging, *blocking someone's agent*

Stream **WD2**, about 3 hours (an estimate, not a measurement). Depends on task 62 (SW.17:
`registerDeskRow`, `registerDeskSource`, `DeskItem`, `DeskSnapshot`, `takeDeskSnapshot`,
`task-62.md`) and task 55 (the replica behind task 40's `WorkReader`; SW.16). Uses task 56's
`mutate` and `WorkOp` (`task-56.md:156-169`), task 42's `acceptTarget`, `nextMorning` and its
`snooze` op (`task-42.md:174-222`), task 46's `handMove` (`task-46.md:248`), task 51's
`comments(issueId)` read (`task-51.md:601-611`, `task-56.md:184`), task 29's `MarkGlyph`
(`task-29.md:145-158`), task 53's `useVerb` and the `triage`, `issueRow` scopes
(`task-53.md:106`, `:183-193`, `:310-316`), SW.5's feed kinds (`task-13.md:122-138`) and the built
`formatWaiting` (`desktop/src/lib/time.ts:49-52`) and heartbeat (`hooks/heartbeat.ts:64`). Task 55
has no part on the tree at this writing: its feed read is **UNVERIFIED** (gap 2).

After this task the Desk's NEEDS YOU section holds the person's own issue work: a triage issue
assigned to them, an urgent issue of theirs, a comment that names them, and an issue of theirs
that blocks a started issue; each says why it ranks where it does, what happens if they do
nothing, and how long it has waited.

**What the reader sees** (05 §4.2's NEEDS YOU row, `05-design.md:932-937`; §4.4 rule 2; AX §5;
`prototype/work-layer.html:764-766`):

- **A blocking row first**: *"EP-150 blocks EP-142 (Alice) · started 1h 12m ago"*, `waiting 1h
  12m` in mono, and under it *"ranked first: blocks EP-142, which is started"* and *"if you do
  nothing: EP-142 stays blocked by EP-150"*.
- **A triage row**: *EP-160*, its title, *"in triage, assigned to you"*, `waiting 2h 10m`, and two
  buttons, *Accept `1`* and *Snooze `4`*, that answer in place.
- **An urgent row**: the `needs_you` mark in `--need` and the words *"urgent, assigned to you"*.
  Urgency is colour **here only**; elsewhere priority is a notch in ink (05 §4.4 rule 2).
- **A mention row**: *"Alice named you on EP-163"* and the comment's first line, `text·H`.
- **No reviewer row**: `IssueHead` has no reviewer field, so *"in review where you are the
  reviewer"* is not built (gap 1).
- The age ticks on the one heartbeat, and a row that stops being true stays while the Desk is
  open; it is gone on the next visit (task 62's decision 3).

**Decisions made here, so nobody re-derives them:**

1. **Four kinds, one pure file.** `needs.ts` holds four selectors over a `DeskSnapshot` and returns
   `DeskItem<NeedPayload>[]`; it has no React, no store and no clock (`snap.takenAt` is the now).
   `deskRows.tsx` registers each kind's row and source (task 62's decision 2).
2. **Mine means assigned to me.** `issue.triage_mine` is `state === "triage"` and `assignee.id ===
   snap.me`; `issue.urgent_mine` is `priority === "urgent"`, assignee me, and a state not in
   `CLOSED` (`landed`, `done`, `cancelled`, `duplicate`). An unassigned triage issue is the Triage
   view's (task 42), not a Desk row: nobody is named, so nobody is needed.
3. **A mention is an unresolved comment whose `mentions` holds my id**, on any readable issue, not
   written by me. W1 does not infer *"a question"* from text (scope), so any unresolved mention
   qualifies; `resolvedAt !== null` makes no row.
4. **Blocking reads the stored relation.** X blocks Y when X is mine, X's `relations` holds
   `{kind: "blocks", issueId: Y}` (`task-37.md:118`), X is not `landed` or closed, and Y is `started`
   or `in_review`. Y's state is the Hub's convergence word that a lane is bound to it (task 22); the
   Desk never reads Alice's sessions (OD-15). Tier 1; the rest are tier 2 (AX §5, task 62's
   `DeskRank`).
5. **Age comes from the feed, then the record.** `since` is the newest SW.5 event on the subject
   that made the row true (triage: `issue.created` or `issue.state` with `to: "triage"`, or
   `issue.updated` naming `assignee`; urgent: `issue.updated` naming `priority` or `assignee`;
   mention: `comment.mentioned` with `data.commentId`; blocking: Y's `issue.state` with `to` in
   `started`/`in_review`), else the head's `updatedAt`, else the comment's `createdAt`. It is fixed
   in the snapshot, so a re-render never restarts it; `formatWaiting(since, useHeartbeat())` prints
   it. `issue.updated`'s `fields` are wire names (`task-13.md:150`), matched as `assignee` and
   `priority` (**UNVERIFIED** against task 11's field list, gap 3).
6. **Words carry the rank and the consequence, as rules.** `rankReason` and `ifNothing` are
   sentences built from the snapshot's facts (L-114), never a time: *"if you do nothing: EP-160
   stays in triage and nothing starts on it"*. No `~`, no *"will"* with a time, no cost of waiting
   in money (no gateway in W1; the wait is shown as time, `OD-27`).
7. **Answer in place, through the verbs that exist.** A triage row's *Accept* sends task 42's
   `{kind: "setState", move: handMove(acceptTarget(policy))}` and *Snooze* task 42's `{kind:
   "snooze", until: nextMorning(now, zone)}`, both through `mutate`; a refusal comes back through
   `settled` and the row says the Hub's words in place (task 56). `I` assigns to me (task 53's
   `issue.assignMe`) and `Enter` opens the issue view beside the Desk (`open.issue`). The Desk does
   not navigate away (AX §5). The team policy is read as task 42's mount reads it (**UNVERIFIED**,
   gap 4); with none, `acceptTarget(null)` is `backlog` (`task-42.md:179-182`).
8. **An answered row stays, answered.** After *Accept* the row reads *"Accepted · backlog"* and
   keeps its place; the next visit drops it (task 62's decision 3). Nothing slides out.
9. **Two additive deltas on task 62's files, no rename.** `DeskItem` gains `keyScopes?: readonly
   KeyScope[]`, which `DeskSection` joins after `issueRow` on the wrapper, so `1` and `4` reach a
   triage row through task 53's `triage` scope; `DeskSnapshot` gains `comments(issueId)`, read once
   per issue at entry from the reader's `comments` (task 51's sixth read). `DeskInputs` gains
   nothing: `events` is task 62's optional input, which `liveDeskInputs` now fills from task 55's
   feed.
10. **The snapshot moves onto the replica by installing it, not by a new read.** Task 62's snapshot
    already reads `workSource()` (task 40's seam). Task 55 installs its Replica there
    (`task-40.md:162`), so once it boots, the Desk reads the replica with no change here. This task
    adds only the feed: `liveDeskInputs.events = () => replicaEvents()` (gap 2).

**Files** (the skeleton's file map covers `desk/*` for tasks 62-65, `:311`; the scope names
`rows/IssueNeedRow.tsx` and `work/desk/needs.ts`, `tasks-61-80.md:103-105`):

- `desktop/src/work/desk/needs.ts` — the four selectors, `since`, the words (new)
- `desktop/src/work/desk/needs.test.ts` — the pure tests (new)
- `desktop/src/components/desk/rows/IssueNeedRow.tsx` — the row and its answers (new)
- `desktop/src/components/desk/rows/IssueNeedRow.test.tsx` — the surface tests (new)
- `desktop/src/components/desk/deskRows.tsx` ⚑ — task 63's block, first after the head (modified)
- `desktop/src/components/desk/deskModel.ts` ⚑ — `keyScopes?` and `comments` (modified, additive)
- `desktop/src/components/desk/DeskSection.tsx` ⚑ — the wrapper joins `keyScopes` (modified)
- `desktop/src/components/desk/Desk.tsx` ⚑ — `events` in `liveDeskInputs`; `useDeskIssueVerbs()` (modified)
- `desktop/src/components/desk/desk.css` ⚑ — `.dn-*`, appended (modified)
