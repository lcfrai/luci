### Task 137: assists, part 2: duplicates, composer hints, guidance layers

W4, control plane and desktop; no seam. Depends on task 136 (`workassist.Service.Complete`,
`Why`, `Keys`, `keepLinked`, `registerWorkAssist`, `assistProblem`, the rig: `task-136.md`, its
table *"What this part fixes for tasks 137 to 140"*). Reads task 14's title search
(`task-14.md:284`, `hubclient.Work.SearchIssues`, `task-16.md:398`), task 37's relations
(`RelationKind`, `addRelation`, `task-37.md:95`, `:334`), task 100's `ResolveGuidance`
(`task-100.md:231-234`, `:310`), task 109's guidance types (`task-109.md:174-189`) and task 20's
`expectNoPrediction`. Estimate 0.375 (3 h).

Closes **L-54** (duplicate and related detection), **L-55** (layered guidance, most local wins)
and **L-56** (light composer suggestions), 05 §1 C.

**What the reader sees:**

- **In Triage and on a triage issue's page:** *"Possible duplicate: EP-128"*, EP-128 a link, and
  under it both titles as the Hub holds them: *"'Resume upload fails on Safari' · 'Upload resume
  broken in Safari'"*. Or *"Related: EP-131"*, the same way. No similarity score, no percentage,
  no *likely*. **Mark duplicate** and **Mark related** write one relation, as the person; neither
  closes, declines or moves either issue. Declining a duplicate stays task 42's `2` key, a
  separate act.
- **While composing a new issue:** after the title and description hold 12 characters and the
  typing has paused for 800 ms, chips appear under the editor: *"Label: bug · because EP-120"*
  and *"Possible duplicate: EP-128"*. A chip is a suggestion: clicking a label chip adds the
  label to the draft, clicking a duplicate chip opens EP-128 in a peek. Nothing is written until
  the person creates the issue.
- **Guidance is visible where it was used.** A why may read *"because team guidance v3"*, a link
  to that version in task 109's history. At feature scope the brief is the guidance: *"because
  Upload brief @4"*.
- **Failures are words:** *"Duplicate check unavailable: the gateway did not answer"*; the
  composer shows no chips and says nothing while it simply has no hint.

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 14, 16, 37, 42, 100,
109 and 136. There is **no part for task 68** (the composer, `NewIssue.tsx`,
`…-scopes/tasks-61-80.md:269-285`) or task 11's relation route beyond task 37's client
(`addRelation`, `task-37.md:428`); their names here are their scopes' and **UNVERIFIED**. Task
37's `RelationKind` is `"blocks" | "duplicates" | "relates"` (`task-37.md:95`): the scope's
*duplicate* and *related* are these two words.

**Decisions made here, so nobody re-derives them:**

1. **Retrieval is the Hub's title search, not embeddings.** Candidates are the result of
   `SearchIssues{ProjectID: the issue's project, Q: the title's terms, Limit: 20}` (task 14): the
   caller's readable issues only, in that project only, so never another team's. Nothing here
   embeds, indexes or stores text. A duplicate in another project of the same team is not found;
   that is stated, not hidden.
2. **The model judges; the Hub quotes.** The prompt lists the candidates as `k1…kN` with their
   titles. The model returns `[{key, relation: duplicates|relates}]`. The control plane maps each
   key back to the candidate and fills both titles **from the Hub's records**, never from the
   model's text, so the quote is always exactly what the issues say. A key not in the list, or a
   relation outside the two words, is dropped.
3. **An erased issue is never a candidate.** An erased head's title reads `""`
   (`task-03.md:306-307`); the candidate list drops any head whose title is empty, and the issue
   itself. An erased issue asked about is `404 issue_not_found`, as in task 136.
4. **Accepting writes a relation and nothing else.** `addRelation(issueId, {kind, issueId:
   other})` through the person's client (task 37). No state move, no decline, no close.
5. **The composer's timing is a small state machine, tested with fake timers.** A call is due when
   the draft (title plus description) has at least 12 characters and 800 ms have passed since the
   last keystroke. At most one call is in flight. A keystroke while one is in flight marks its
   answer stale; a stale answer is discarded when it lands, and the next call waits for the pause
   again. Typing 30 characters in a burst, pausing, typing more and pausing makes two calls.
6. **The composer's hint call carries the draft, which has no author yet.** It is the person's
   text, typed now, sent to their project's route under the kill switch like every assist. It is
   never stored: the control plane keeps nothing, and the desktop drops the hints when the
   composer closes.
7. **Guidance layers, most local last.** `guidanceFor(ctx, scope)` returns
   `[{level, key, text}]` ordered org, team, person (task 100's `ResolveGuidance`, which answers
   most local first, reversed), then, at feature scope, the feature's brief at its current
   revision. The last entry is marked `winning: true`. SW.19 has **no project layer**
   (`GuidanceScope = "org" | "team" | "person"`, `task-109.md:176`); the scope's `project` level is
   not invented here (gap 1).
8. **Guidance enters an assist's prompt, and only its prompt.** Each assist's prompt gains the
   layers with keys `g1…gN`, and `Keys` maps each to its link key (`gd-…@v<version>`, or the brief
   key), so a why may cite the guidance it used. Guidance is person-written, `text·H`
   (`task-109.md:16-17`). Whether guidance may enter a *session agent's* context is task 99's
   not-claim and stays so (gap 4).
9. **Numbers are counts of things, never of confidence.** No hint, chip or row shows a score, a
   percentage, a rank number or an ordinal (*"top match"*) (AX anti-pattern 12).
