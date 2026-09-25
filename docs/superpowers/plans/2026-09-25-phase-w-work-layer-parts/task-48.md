### Task 48: the issue view, part 3: relations, the live agent line, *done, not proven* and its reason

Stream **WD3** (the issue view), about 2.5 hours (an estimate, not a measurement). Depends on task
46 (the view, the pickers, `AuthorClass`, `clock`). Uses SW.1's relations (task 11's five words),
SW.4's `StateMove` and `IssueHead.proof`, SW.12's `StateGlyph notProven`, SW.16's `mutate`, task
40's `AgentLine`, the one heartbeat (`useHeartbeat`, `desktop/src/hooks/heartbeat.ts:64`) and
`formatWaiting` (`desktop/src/lib/time.ts:49`). **Runs after tasks 40 and 43** as well: it edits
their `IssueRow.tsx` and `BoardView.tsx` by one call each (a gap in the skeleton's dependsOn,
named below).

This part closes the three places where an issue view is most tempted to guess:

1. **A relation is one record seen from both ends.** EP-142 *blocked by* EP-150 is EP-150
   *blocking* EP-142, because the Hub stores one row and each end reads its own word (task 11,
   `task-11.md:102-112`). The view never keeps a relation of its own.
2. **The agent line is built from facts, never from the terminal.** One line per the reader's
   **own** agent on the issue, from the control plane's sessions with an `issueRef` (task 19) and
   the session status; never PTY bytes (`OD-8`). A teammate's work is the person, with no dot and
   no count (05 §4.4 rule 3).
3. **Done by hand is not done by evidence.** On an issue that serves criteria, choosing `done`
   asks for a one-line reason and sends `StateMove{to: "done", cause: "hand", reason}`; the Hub
   records `proof: not_proven` (task 10, `task-10.md:47-55`), and the issue then carries
   `StateGlyph notProven` everywhere it appears. The **word** is the Hub's; this part maps it to a
   sentence and never decides it.

**Decisions made here, so nobody re-derives them:**

1. **Five words.** `blocks` reads *blocking*, `blocked_by` *blocked by*, `duplicates` *duplicate
   of*, `duplicated_by` *duplicated by*, `relates` *related*. A person adds only the three writable
   kinds plus *blocked by*, which is sent as the other issue's `blocks` from this end: task 11's
   `NormaliseRelation` accepts either word from either end (`task-11.md:263-264`) for delete, but a
   `RelationRequest` naming `blocked_by` is 400 (`:24`). So *blocked by EP-150* is sent as
   `{kind: "blocks"}` on EP-150's id, subject EP-150.
2. **The other end refreshes through the feed.** The Hub answers with the subject's head only and
   emits `issue.related` carrying `otherIssueId` (`task-13.md:166-170`). The replica must re-fetch
   **both** the subject and `otherIssueId` on that event; task 55's scope re-fetches the subject
   only. A gap for task 55, named below.
3. **A cross-owner relation is one op, and nothing else.** The Hub's event carries
   `otherAssigneeId` when the other issue's assignee is someone else (`task-11.md:329-332`), and
   task 65 turns it into that owner's Desk *inform*. This view sends the relation and no act at
   any session, its own or anyone's (05 §3.7 rule 3).
4. **"Blocking dylan · agent 1 · waiting 42m" is shown only from the reader's own facts.** On a
   blocker that is not `landed` or `done`, for each issue it `blocks`, the reader's own sessions on
   that issue whose status is `idle` are listed with `formatWaiting(lastOutputAt, beat)`. It says
   the two facts sit together, not that one caused the other; the Desk row with the cost of the
   wait is task 63. Another person's stalled agent cannot be seen in local mode (`OD-20`).
5. **The hand-done reason step lives in the state picker**, where task 46 left `done` shown and
   unsendable. The reason is trimmed and must be non-empty; there is no length rule beyond the
   Hub's (task 10's 400 names `reason`).
6. **Who set it and when.** *"the head shows the reason, its author and time"* needs the author
   and time of the state move, which `IssueHead` does not carry (`task-03.md:282-303`: only
   `stateReason` and `updatedAt`). This part reads `stateMovedBy` and `stateMovedAt` as nullable
   head fields, a gap for tasks 10 and 37; until they exist it shows the reason and *set by hand*.
7. **The evidence detail is a cited record's own words.** *own evidence at 7c1f0a2 (test run, exit
   0)* needs the cited record's `because` line (task 23 writes it: `task-23.md:553`), which no W1
   read returns: the citation lives in task 10's `issue_proof_citations`. This part reads
   `IssueHead.proofCitations` (a gap, named below) and renders each `because` **verbatim**; with
   none, the line is the word and the land commit alone.
8. **`glyphPropsOf(head)` is the one rule for a glyph.** Every place that draws an issue's glyph
   (the view, the row, the board card) calls it, so *done, not proven* cannot be drawn in one place
   and missed in another.

**Files:**
- `desktop/src/components/issue/Relations.tsx` — the relation list and its add and remove (new)
- `desktop/src/components/issue/IssueAgentLine.tsx` — own agents' lines, the teammate line (new)
- `desktop/src/components/issue/ProofLine.tsx` — the result words and the reason (new; not yet
  in the File map)
- `desktop/src/components/issue/glyphOf.ts` — `glyphPropsOf(head)` (new; not yet in the File map)
- `desktop/src/components/issue/pickers/StatePicker.tsx` ⚑ (task 46's) — the reason step (modified)
- `desktop/src/components/issue/IssueHead.tsx` ⚑, `IssueView.tsx` ⚑ (tasks 46-47's) — the glyph
  through `glyphPropsOf`; `ProofLine`, `Relations` and `IssueAgentLine` mounted (modified)
- `desktop/src/components/work/IssueRow.tsx` ⚑ (task 40's), `BoardView.tsx` ⚑ (task 43's) — their
  `StateGlyph` props through `glyphPropsOf` (modified; one call each)
- `desktop/src/components/issue/issue.css` ⚑ — the `.iv-rel*`, `.iv-agents*`, `.iv-proof*` and
  `.iv-reason*` rules, appended (modified)
- `desktop/src/components/issue/{Relations,IssueAgentLine,ProofLine}.test.tsx`,
  `pickers/reason.test.tsx`, `glyphOf.test.ts` — the tests (new)

