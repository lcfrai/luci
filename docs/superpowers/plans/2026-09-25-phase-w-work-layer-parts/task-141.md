### Task 141: an agent's edit to an issue or brief is a proposal, shown as a diff

W4, desktop (plus one Go test), about 2.5 hours (an estimate, not a measurement). Depends on task
47 (`lineDiff` and `DiffLine` in `desktop/src/components/issue/revisionWords.ts`,
`task-47.md:151-172`) and task 136 (the assists that file proposals; **scope only, no part on this
tree**, so its names are **UNVERIFIED**). Reads task 46's `IssueView` and its `slots`
(`task-46.md:520-527`), task 49's `authorClass.ts` (`task-49.md:101-129`, decision 1: a field is
`A` while any agent span remains), task 51's `Comments` (`task-51.md:466-526`), task 56's `revise`
and `resolve` ops (`task-56.md:157-168`), task 37's `IssueRevisionContent` and `IssueComment`
(`task-37.md:111-159`), task 19's SW.7 golden (`task-19.md:55-56`), and phase 4's `4-D2` and
`Answer.Author` (`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md:143`,
`…-parts/task-03.md:83-91`) with P4 task 18's `BriefPanel.tsx` (plan `:333`, **UNVERIFIED until
merge**, rule H2).

Closes **L-130** (05 §1 group I: *"An agent's edit to a brief or issue is a **proposal**
(`text·A`), shown as a diff, that a person accepts"*). Serves L-124 (closed in W1) and L-78 (task
139's drafted updates stay `text·A` until posted).

**The rule it rests on.** 05 §3.6: *"an agent writes only comments on its own owner's behalf"*; and
`4-D2`: *"A model-written answer is not inherited until a person accepts or changes it."* So an
agent never writes an issue revision. Its edit is an `ic-` comment of kind **`edit_proposal`**,
`text·A`, whose body is an `EditProposal`. That kind is in neither SW.19 (task 99: `note`,
`evidence_proposal`, `negotiation`) nor SW.20 (task 114, checked by grep on this tree: no
`edit_proposal`), so it is **a gap to close in task 114** (the kind constant, the body's
`Validate`, the fake's rule and the migration's kind `CHECK`), written below as complete source
for that owner and **not** applied by this task.

**What the reader sees** (05 §3.4, §3.6, §4.4; 04 §4.4 `:338-351`; AX Principle 7):

- **A strip above the issue's head fields** when an open proposal exists: *"Proposed edit · dylan
  · agent 1 · 11:02 · on @3"* with *Review*. The head fields still show @3: nothing is applied on
  arrival.
- **Review opens `ProposalDiff` in place**: title and description as a word diff, removed words
  struck through **and** announced as *removed*, added words underlined **and** announced as
  *added*, never colour alone. Serves changes read as words: *"adds C4 · removes C2"*. The head
  line reads *Proposed by dylan · agent 1 (claude-sonnet, session s-7f2c) at 11:02, on @3*, with
  ids and times in Commit Mono (`OD-28`), and each `why` key as a link.
- **Three acts, in place** (`confirm.inplace`): **Accept** writes one new revision as the person,
  the accepted texts still marked `text·A`; **Edit** opens the one editor on the proposed text;
  **Decline** writes nothing to the issue and resolves the proposal comment.
- **Stale**: when the head has moved past the proposal's base, the strip reads *"Stale: EP-142
  moved to @5 (Alice, 11:04) since this was proposed on @3"*, the diff is shown against @5, and
  Accept stays disabled until the person presses *I have read the changes against @5*.
- **In the thread** an `edit_proposal` shows as one line, *"Proposed an edit · Review"*, a link to
  the strip. Its body is never rendered as Markdown.
- **In Discover**, a `model_drafted` answer that replaces an existing answer shows as the same
  diff; Accept sends `model_accepted`, Edit sends `person`.

**Decisions made here, so nobody re-derives them:**

1. **`lineDiff` is not lifted.** It already lives in `revisionWords.ts`, outside `Revisions.tsx`
   (`task-47.md:56`, `:154`), so `wordDiff.ts` imports it unchanged and adds a word level inside
   each changed pair of lines. The word level uses the same longest-common-subsequence method as
   `lineDiff` (not Myers): one method, one output shape, no dependency.
2. **The proposer is carried in the body.** The comment's `createdBy` is the owner (the agent
   writes on its owner's behalf); `EditProposal.proposer {agentId, sessionId, model}` names the
   agent. The strip's *agent 1* is task 47's `agentsWhoRead` numbering over the reader's own
   sessions when the reader is the owner, and `agent · s-7f2c` otherwise (nobody else's agent is
   numbered as if it were yours).
3. **Author per text, never per revision** (L-124). Accept writes each proposed text with `author:
   "A"`. After Edit, each field's class is task 49's `fieldAuthor` of what the person committed: a
   title the person rewrote in full is `H`; a title with any drafted words left is still `A`; a
   field the person did not open keeps the proposal's `A`. The person is the revision's writer in
   every case.
4. **Accept always writes against the head it shows.** The `revise` op's `baseRev` is the head
   revision the diff was computed against, so a head that moves again during review is task 56's
   ordinary `409` and a fresh stale line, never an overwrite.
5. **Decline resolves; nothing is deleted.** Comments are append-only (task 12). Decline is task
   56's `resolve` op on the proposal comment. A resolved proposal leaves the strip.
6. **One strip, newest open proposal first.** With several open proposals the strip names the
   newest and *"2 more proposals"* as a link to the thread; each is reviewed one at a time.
7. **Proposals never reach an agent.** The Materialiser renders the issue revision, never
   comments (05 §3.4; skeleton rule 6). A Go test pins that an open `edit_proposal` leaves SW.7's
   managed block byte-identical.
