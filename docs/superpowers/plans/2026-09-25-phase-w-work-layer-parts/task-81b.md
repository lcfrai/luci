### Task 81b: `implements` — the write and read paths, and the design approval that closes a design issue

W2, stream WH. Seams: **SW.1** (an added *use*, not an added field) and **SW.4** (an added
`StateCause`, not an added done rule — decision 2). Depends on task 10 (the state move and the done
rule), task 23 (the evaluator), task 74 (the W2 migration), **Hub task 22** — which owns
`control-plane/internal/hub/server/approvals.go` and `store/approvals.go`, the write this task hooks
(hub plan `:304`, `:425`; hub contract §7.7, routes 30-32 at
`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:592-594`) — and, for
the phase-5 half of the approval record, **P5-21**. Write it `dependsOn: 10, 23, 74; H22; P5-21`.

Anchors, all read on 2026-09-28 unless marked. Phase W's own parts:
`2026-09-25-phase-w-work-layer-parts/task-02.md` and `task-03.md` (cited `task-NN.md:N`). The
design spec `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (cited `05:N`) and
`04-internal-fit.md` §7.7 at `:587-590`. The phase 5 plan
`.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md` (cited
`P5:N`), which is **another worktree's branch**: every path it names is **UNVERIFIED until phase 5
is sync-merged here**, and rule H2 gives that phase's file the win on any conflict.

**Read this before anything else: the wire is already done.** `implements` is not missing.
`IssueRevisionContent.Implements []string` — a list of `design:…` keys — is declared at
`task-02.md:264`, is part of the canonical bytes every revision digest covers, appears in all three
golden vectors (`task-02.md:458`, `:507`, `:548`, the middle one carrying
`design:cv-7c01d2e3f4a5b6c7@11`), is on the API type at `task-03.md:424`, and has its field-name row
at `task-03.md:738`. **So this task adds no wire field and reopens no golden vector.** If you find
yourself editing `workcanonical.go`, stop: you have misread the task.

**What it does add, declared here rather than discovered by the implementer.** One `StateCause`
constant, `design_approved`, and one row in task 10's `MoveAllowed` table. That is an *added
constant*, which the WS0 rule (skeleton `:366`) permits without reopening task 05 — but it is
visible in three more places, and this task changes all three: task 04's fake (`task-04.md:876`,
`:1009`, the two cause switches), task 03's `MoveCauses` list (`task-03.md:220`) and task 28's glyph
set, which needs a word for the cause. The golden vectors' bytes do not move: no example carries
this cause, and `StateMove.cause` is a string either way. If a reviewer reads *"one more cause"* as
an SW.4 reopening, the answer is that SW.4 is the done **rule**, and the rule is untouched — see
decision 2.

What was missing, and is what you build: nothing **wrote** the field from a surface, nothing
**read** it back the other way (*which issues implement this canvas?*), and nothing acted on
04 §7.7's second half, *a design approval can close a design issue*.

**Vocabulary, settled — do not reopen it.** 05 says *frame*; phase 5's document model has no
*frame* at all — its types are `Page, Node, Token, Asset, Thread, ReviewRequest, Author, Document`
(`P5:359`), so 05's *frame* reads as phase 5's **`Page`** — and its design key
`design:<canvasId>@<rev>` names a **canvas revision, not a subset** (`P5:232`, decision P5-H:
*"subsets need a second key form"*). So `implements` names
a canvas revision, exactly as `task-02.md:264` already has it, and there is no per-frame key to
point at. A frame is a locator inside the revision. **Do not invent a second key form.**

**Four decisions, made here.**

1. **The match is the exact key.** An approval of `design:cv-…@12` does not close an issue whose
   revision implements `@11`. The approval's digest covers the revision that was reviewed, and a
   brief edit shows as `superseded` rather than voiding it (`P5:183`, D3 option A *Pin and mark*).
   Widening this to "any revision at or after" is a *later* row, not a W2 liberty.
2. **An approval is a LANDING, not a citation — the done rule is untouched.** This is the decision
   the round-1 part got wrong, and the reason is worth stating so nobody re-derives it. To close by
   *evidence*, the Hub needs a `Cited{BriefKey, CriterionID, Cause, RecordedBy}` row per served
   criterion (`task-10.md:289-303`). An approval names a **design key**, and **nothing anywhere
   maps a design key to a brief criterion**: phase 4's `Criterion` is `{ID, Text}` with no class,
   and the phase 5 plan contains the string *criteri* **zero** times — a canvas revision does not
   record what it serves. So the only way to close by evidence is to *invent* citations, which is
   fabricating proof, the exact thing `OD-27` forbids. **Instead the approval does what a land
   trailer does:** it moves the issue to `landed` with the new cause `design_approved`, and task
   10's existing rule 6 then decides the word with no change at all —
   - the issue serves no criterion (a `notTiedReason` design chore): task 10 rule 6's `no_criteria`
     branch lands it **at `done` in the same write**, unless the team's `stopAtLandedWithoutServes`
     holds it at `landed` (`task-10.md:52`, `TestNoCriteriaLandsAtDone` at `:462`). *This is the
     case 04 §7.7 means by "a design approval can close a design issue", and it closes.*
   - the issue serves criteria: it sits at `landed` with `proof: waiting` until each criterion gets
     its own evidence (task 10 rule 5). The approval got the design in; it did not prove the brief.

   Consequences, all of them wanted: no seventh `ProofWord` (the words are the existing
   `no_criteria` and `waiting`); no synthesised `Cited` row; no "design criterion" class to invent;
   and a mixed issue cannot close early because nothing about it changed.
3. **`landCommit` stays `""`, and that empty string is the signal.** A design landing has no commit.
   `IssueHead.landCommit` is `""` and `state` is `landed` or `done` — **the pair
   (`landed`-or-beyond, `landCommit == ""`, a non-empty `implements`) is exactly a design-approved
   issue**, which is what 96b renders and what the `landCommit` field's *"`""` before landed"*
   comment must be widened to say. No new field carries this. The move is written **as the
   approver**: only a human approves, and only through the chrome (`P5:115-117`, ADR-H1 R5), so
   rule 2 holds unbroken — no sync or integration principal reaches this path, and no reason string
   is required because the cause is not `hand`.
4. **The index is derived, never authoritative.** `issue_implements` is a reverse index over the
   revisions; it is rebuildable from them, and the rebuild is a test, not an operator procedure.

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's file, modified additively):
  `ValidateImplements([]string) error`, which parses each entry through `hubapi/keys.go` and
  refuses anything that is not `design:cv-…@<rev>`; `DesignLanding{DesignKey, ApprovedBy,
  ApprovedAt}` — a *landing* record, not an evidence record, because decision 2 makes it one;
  `ImplementsRow{IssueID, Revision, DesignKey}`; the two codes this task needs
  (`implements_key_form`, `design_not_approved`) if task 03 has not declared them
- `control-plane/internal/hub/hubapi/work_implements_test.go` (new): the pure rules, no database
- `control-plane/internal/hub/store/work_implements.go`, `server/work_implements.go` (new): the
  reverse index, written in the **same transaction** as the revision it indexes, and
  `GET /v1/designs/{designKey}/issues`
- `control-plane/internal/hub/store/work_revisions.go` ⚑ (task 09's file): the revision write fills
  the index; restore (task 09's path) fills it too, which is decision 4's second test
- `control-plane/internal/hub/hubapi/work_states.go` ⚑ (task 10's file, where `MoveAllowed` and
  `DecideProof` live): **one** new case arm, and nothing else —
  `case CauseDesignApproved: ok = to == StateLanded && in(from, StateTriage, StateBacklog,
  StateTodo, StateStarted, StateInReview, StateLanded)`. It ends at `landed`, never at `done`, so no
  existing arm is loosened and `DecideProof` is not edited at all. Terminal states
  (`done`, `cancelled`, `duplicate`) are absent on purpose: a closed issue is not re-landed.
- `control-plane/internal/hub/hubapi/work.go` ⚑ again: `CauseDesignApproved StateCause =
  "design_approved"`, appended to `MoveCauses` (`task-03.md:220`)
- `control-plane/internal/hub/server/work_states.go` ⚑ (task 10's server half): the `landed` write
  accepts the new cause and leaves `land_commit` NULL; the `no_criteria` branch and the
  `stopAtLandedWithoutServes` switch run exactly as they do for a land trailer, uncopied
- `control-plane/internal/hub/hubfake/work.go` ⚑ (task 04's fake, `task-04.md:876`, `:1009`): the
  same arm in both cause switches, so WS0 writers see the behaviour before the Hub exists
- `control-plane/internal/hub/server/approvals.go` ⚑ **(Hub task 22's file — this is the call
  site)**: after the approval, the design link and the snapshot are written, and **inside the same
  transaction**, one call to the new `closeIssuesForDesign(ctx, tx, designKey, approver)`. Without
  this row the close has nowhere to be called from, which is how round 1 failed.
- `control-plane/internal/hub/server/work_design_close.go` (new): `closeIssuesForDesign` — every
  open issue whose **current** revision implements that exact key is moved `→ landed` with cause
  `design_approved`, attributed to the approver, then handed to the unchanged done rule
- `control-plane/internal/hub/migrate/sql/NNNN_work_w2.sql` ⚑ (task 74's file, a section appended
  before it is applied anywhere shared; the next free number instead if 74 has merged, rule H1):
  `issue_implements(issue_id, revision, design_key, canvas_id, canvas_rev)`, its RLS, its grants,
  and the named constraint `issue_implements_key_form`
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑: one authorisation-matrix row
  per new route, including the row that proves the sync principal is refused
- `control-plane/internal/api/work.go` ⚑ (task 16's relay): the read route, passed through
  untouched, problem and all (`task-16.md:25-32`)

**Routes.** Two, both added to task 07's list and its matrix in this task:

```
// GET /v1/designs/design:cv-7c01…@11/issues     member of the project's team; 200
// → the issues whose CURRENT revision implements exactly that key, newest first
{ "issues": [ { "id": "is-0a1b…", "alias": "EP-142", "revision": 3, … IssueHead … } ] }
// 404 design_not_found when the key is well formed but names no canvas the caller can read;
// 400 implements_key_form when it is not design:cv-…@<rev>

// POST /v1/issues/{issueId}/revisions            (task 09's route, unchanged shape)
// content.implements: ["design:cv-7c01…@11"]     400 implements_key_form on any other key form
```

The close path has **no route of its own**. Its call site is named, not implied: the Hub's own
`POST /v1/projects/{projectId}/design-approvals` (hub contract §7.7, routes 30-32, one call that
writes approval, design link and snapshot), handled in **Hub task 22's**
`control-plane/internal/hub/server/approvals.go`, which this task extends with one call in the same
transaction — so an approval that is rolled back closes nothing. Phase 5's
`internal/designapi/approve.go` (P5-21) is that route's **caller**, not the writer; this task does
not touch it, and the skeleton's delta row says so.

**Tests first.** Each names the mutant it kills.

1. An approval at `@12` leaves an issue implementing `@11` open. *Mutant:* matching on `canvas_id`
   closes it.
2. An issue serving **one criterion with no evidence** is `landed`, `proof: waiting`,
   `landCommit: ""` after the approval — not `done`. A second test: the same issue with
   `serves: []` and `notTiedReason: "chore"` is `done`, `no_criteria`, in the same write; and with
   the team's `stopAtLandedWithoutServes` on it stops at `landed`. *Mutant:* the approval closes the
   issue whatever it serves — i.e. fabricated proof.
2b. `DecideProof` is byte-identical to task 10's. *Mutant:* a `design_approval` branch inside it.
3. The close is attributed to the approver, and appends exactly one event and one state fact.
   *Mutant:* attributed to the Hub, or to the sync principal.
4. The sync principal calling the approval path is refused by the authorisation row, not by a
   handler check. *Mutant:* the guard lives in one handler and the next route forgets it.
5. A revision whose `implements` names a `brief:` key, a bare `cv-…`, or a canvas with no revision
   is 400 `implements_key_form`. *Mutant:* any key form accepted.
6. The index rebuilt from `issue_revisions` equals the stored index, after a create, a restore and
   an erase. *Mutant:* the index is written on create only.
7. `GET …/issues` for a canvas of another team is 404, never 403 (the reason task 11 records at
   `task-11.md:31-35`). *Mutant:* 403, which leaks that the canvas exists.

**Acceptance, per criterion (`OD-27`, no predictions).** (a) An issue whose revision implements an
approved canvas revision and serves **no** criterion reads `done`, `no_criteria`, `landCommit: ""`
with no click; the same issue under `stopAtLandedWithoutServes` reads `landed`. (a2) An issue that
serves a criterion reads `landed`, `waiting` — the design is in, the brief is not proven, and the
Merge lens still owns the close. (b) No proof word outside task 10's six is ever written, and
`DecideProof`'s source is unchanged. (c) The reverse read returns the issues of a
canvas and nothing else. (d) No percentage, bar, ETA or "N of M" figure is produced by any of it.

**Does not claim.** A per-frame key (P5-H). A revocation path — the Hub has none (`P5:195`), so an
approval withdrawn is a phase 5 question, not this task's. A cross-project implement (the read is
inside the caller's readable set, as task 11's decision 3 has it). That phase 5's approval route
exists: until phase 5 is sync-merged this task codes against the Hub's own approval write and its
fake, and the part says so rather than asserting a path it has not seen.
