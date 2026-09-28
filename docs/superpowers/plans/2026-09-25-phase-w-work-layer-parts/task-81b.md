### Task 81b: `implements` — the write and read paths, and the design approval that closes a design issue

W2, stream WH. Seams: **SW.1** (an added *use*, not an added field), **SW.2** (**one added
interface**, `WorkDesign`, which is how task 04 says that seam grows — `task-04.md:162-164` — and
never a method inside `WorkIssues`) and **SW.4** (an added
`StateCause`, not an added done rule — decision 2). Depends on task 04 (the `Work` interface and
the fake, whose freeze SW.2 states), task 10 (the state move and the done
rule), task 23 (the evaluator), task 74 (the W2 migration), **Hub task 22** — which owns
`control-plane/internal/hub/server/approvals.go` and `store/approvals.go`, the write this task hooks
(hub plan `:304`, `:425`; hub contract §7.7, routes 30-32 at
`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:592-594`) — and, for
the phase-5 half of the approval record, **P5-21**. Write it `dependsOn: 04, 10, 23, 74; H22;
P5-21`. **Not** task 13 and **not** task 72: decision 2b adds no event kind, so the registry those
two own is untouched.

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
`:1009`, the two cause switches), task 03's `MoveCauses` list (`task-03.md:220`, appended in
`init()` — decision 6) and the **desktop's cause vocabulary**, which 96b widens: `task-37.md:99-100`'s
closed `MoveCause` union and `task-41.md:260-267`'s `PHRASE` map, exhaustive over that union and
read at `:274` as `PHRASE[move.cause](move)`, so the file does not compile until `design_approved`
has a phrase. **Not task 28's glyph set** — `GlyphKey = IssueState | "done_not_proven"`
(`task-28.md:104`) is keyed on state and priority, never on cause, so round 2's instruction to give
it a word could not be carried out. The golden vectors' bytes do not move: no example carries
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

**And the Goal sentence now says it too.** The skeleton's thread read *brief → criterion → design
frame → issue → …* (`work-layer.md:7`), which named a hop this plan has ruled does not exist: there
is no frame type, no frame key and no frame-level list anywhere in what 81b and 96b build. It reads
*design canvas* from this batch on, the word the link actually joins — issue revision → canvas
revision. The owner fork's undo instruction (`…-scopes/tasks-81-100.md:554-559`) names the new word.
The thread is unchanged; only the sentence describing it stopped over-promising.

**Six decisions, made here.**

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
   trailer does — and only as far as a land trailer's own `no_criteria` shortcut already reaches:**
   it moves the issue with the new cause `design_approved`, task 10's existing rule 6 decides the
   word with no change at all, and it moves **only an issue whose current revision serves nothing**.
   - **`serves: []`, a `notTiedReason` design chore.** Rule 6's `no_criteria` branch closes it **at
     `done` in the same write**, unless the team's `stopAtLandedWithoutServes` holds it at `landed`
     (`task-10.md:51-52`, `TestNoCriteriaLandsAtDone` at `:462`). *This is the case 04 §7.7 means by
     "a design approval can close a design issue", and it is the only case that closes.*
   - **`serves` non-empty: the approval moves no state at all.** Round 2 of this part had such an
     issue sit at `landed` with `proof: waiting`; that state is unreachable, and the reason is
     pinned here so nobody re-derives it. (a) Task 10 rule 7 makes the real merge's later
     `land_trailer` **at a different commit** a 409 `state_conflict` on an already-`landed` issue
     (`task-10.md:56-60`, enforced at `:413-416`) — a design landing would lock the Merge lens out
     until a human reopened the issue. (b) `issue_proof_citations.land_commit` is `NOT NULL` 40-hex
     (`task-10.md:137`) and rule 3 requires the evidence's commit to equal the land commit
     (`task-10.md:29-35`), so a `waiting` with no land commit is terminal, not waiting. So **this
     path writes nothing at all against such an issue** — no state, no fact, no event, no row — and
     **the Merge lens still owns the close, unchanged**. The approval got the design in — it did not
     prove the brief, and it does not pretend to.

   2a. **What records the approval against a serving issue, since this path writes nothing** — the
   question round 3 left open, ruled here (rule 8). The record already exists and is **derived, not
   written twice**: Hub task 22's approval row in `store/approvals.go` is the durable fact that this
   design key was approved, by whom and when, and the issue's `issue_implements` row (written by its
   revision, decision 4) is the durable fact that it implements that key. The join of the two **is**
   the trace; nothing else is needed and a second table would be a second source of truth, which
   decision 4 forbids for exactly this data. `DesignLanding{DesignKey, ApprovedBy, ApprovedAt}` is
   therefore a **read shape over that approval row, never a stored record** — it is what the read
   route returns beside the issues, and 96b renders it as the *Issues* group's approved-at line. It
   has no table, no migration and no writer, and the Files section says so. The alternative — a
   `design_landings` table in task 74's section — was rejected because it would duplicate Hub task
   22's row, would need its own RLS, grants and backfill, and would go stale the moment phase 5's
   approval path writes without it.

   2b. **No new event kind, and none is needed.** `emitWork` refuses a kind the registry does not
   pair with its `data` type (`task-13.md:73`), and W2's kinds are appended by task 72
   (`task-13.md:250`) — so an invented kind here would be a dependency on 72 and a registry edit
   this task does not want. It needs neither. The **chore** path moves state, so it emits task 10's
   existing `EventIssueState` through the unmodified `server/work_states.go` write, with `who` = the
   approver and `cause` = `design_approved`; that is the only event this task produces and it
   registers nothing. The **serving** path emits no work event; the approval's own event is Hub task
   22's, on the approval, where it belongs. **This task therefore adds no event kind, edits no
   registry, and does not depend on task 13 or task 72.**

   **This is a ruling, declared not inherited (rule 8).** Round 1 ruled *an approval is a landing*;
   that is kept for the design chore and withdrawn for the mixed issue, because the two consequences
   above are task 10 rules, not preferences, and the alternatives — a tenth `design_landed` state,
   or a carve-out in task 10 rules 3 and 7 — both reopen SW.4 for a case that no acceptance
   criterion asks for. The narrow form reopens nothing.

   Consequences, all of them wanted: no seventh `ProofWord` (the word is the existing
   `no_criteria`); no synthesised `Cited` row; no "design criterion" class to invent; no second
   landing to conflict with; and a mixed issue cannot close early because nothing about it moved.
3. **`landCommit` stays `""`, and that empty string is the signal.** A design landing has no commit.
   `IssueHead.landCommit` is `""` and `state` is `landed` or `done` — **the pair
   (`landed`-or-beyond, `landCommit == ""`, a non-empty `implements`) is exactly a design-approved
   issue**, which is what 96b renders and what the `landCommit` field's *"`""` before landed"*
   comment must be widened to say. No new field carries this. **And no `issue_proofs` landing row is
   written by this path at all**: `issue_proofs_landed_together` requires `land_commit`, `landed_at`
   and `landed_by` to be all-NULL or all-set (`task-10.md:123-124`), and a design close has no
   commit, so writing the approver there is a `23514` the store does not map. The approver is
   attributed on the **move event's `who`** — which is what the fact line renders — and nowhere
   else; test 3 asserts exactly that and not an `issue_proofs` row. The move is written **as the
   approver**: only a human approves, and only through the chrome (`P5:115-117`, ADR-H1 R5), so
   rule 2 holds unbroken — no sync or integration principal reaches this path, and no reason string
   is required because the cause is not `hand`.
4. **The index is derived, never authoritative.** `issue_implements` is a reverse index over the
   revisions; it is rebuildable from them, and the rebuild is a test, not an operator procedure.
5. **The database guard is replaced too, and it is what pins decision 2's narrow form.** The plan
   keeps two legality tables on purpose (SW.3): the Go `MoveAllowed` and the SQL
   `guard_issue_state()`. Widening one is a 422 on the other. Task 74's W2 section therefore carries
   task 10's function body line for line (`task-10.md:185-199`) with two arms widened, exactly as
   task 145 does for task 144's causes (`task-145.md:441-449`):
   `WHEN 'landed' THEN v_cause = 'land_trailer' OR (v_cause = 'design_approved' AND NOT v_serves)`
   and, on the `done` arm, a third `OR (v_cause = 'design_approved' AND NOT v_serves)` beside the
   existing `land_trailer` shortcut. **Both are gated on `NOT v_serves`**, so an issue that serves
   a criterion cannot be moved by this cause even by a caller that ignores decision 2 — the database
   is the pin, not the prose. Two consequences this task also owns: `task-07.md:70-71`'s published
   set `hub.work_cause ∈ {six causes}` becomes seven; and `TestEachFactHasItsOneCause`
   (`task-07.md:512`) says *"for each of `started`, `in_review`, `landed`: its cause → ok; each of
   the other five causes → `42501`"*, which now asserts the opposite of acceptance (a) — its
   `landed` row gains *"…except `design_approved` on a head that serves nothing"*, its "other five"
   becomes "other six", and a new row `TestDesignApprovedOnlyWithoutServes` (cause `design_approved`
   into `landed` and into `done`: head serves nothing → ok; head serves one criterion → `42501`)
   sits beside `TestLandTrailerLandsAtDoneOnlyWithoutServes` (`task-07.md:513`), which it mirrors.
   Task 07 forbids a later task *loosening* a rule its tests pin (`task-07.md:93-95`); this widens a
   published set and rewrites the row that states it, in the open, which is the same thing task 145
   was allowed to do.
6. **The cause is appended in `init()`, not in the declaration.** `task-144.md:185` is the plan's
   precedent and its reason: task 03's `MoveCauses` line stays untouched, so no W2 commit rewrites a
   W1 declaration. The one thing that pattern does **not** carry is the count — `task-03.md:807`
   pins `want["MoveCauses"]: 6` in `TestWorkValueListsAreSets`, and an unnamed append turns task 03's
   suite red on a W2 commit. This task names it: 6 → 7.

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's file, modified additively):
  `ValidateImplements([]string) error`, which parses each entry through `hubapi/keys.go` and
  refuses anything that is not `design:cv-…@<rev>`; `DesignLanding{DesignKey, ApprovedBy,
  ApprovedAt}` — **a read shape only** (decision 2a): it is projected from Hub task 22's approval
  row on the way out of the read route, it has no table, no migration, no RLS and no writer, and a
  `store/` function that persists one is the mutant test 8 kills;
  `ImplementsRow{IssueID, Revision, DesignKey}`; `DesignIssues{Design *DesignLanding, Issues
  []IssueHead}`, the read route's envelope, whose `Design` is `nil` when the key names a canvas
  revision the caller can read but nobody has approved; the two codes this task needs
  (`implements_key_form`, `design_not_approved`) if task 03 has not declared them
- `control-plane/internal/hub/hubapi/work_implements_test.go` (new): the pure rules, no database
- `control-plane/internal/hub/store/work_implements.go`, `server/work_implements.go` (new): the
  reverse index, written in the **same transaction** as the revision it indexes, and
  `GET /v1/designs/{designKey}/issues`
- `control-plane/internal/hub/store/work_revisions.go` ⚑ (task 09's file): the revision write fills
  the index; restore (task 09's path) fills it too, which is decision 4's second test
- `control-plane/internal/hub/hubapi/work_states.go` ⚑ (task 10's file, where `MoveAllowed` and
  `DecideProof` live): **one** new case arm, and nothing else, mirroring `CauseLandTrailer` at
  `task-10.md:277-279` —
  `case CauseDesignApproved: ok = to == StateLanded && in(from, StateTriage, StateBacklog,
  StateTodo, StateStarted, StateInReview)`. It ends at `landed`, never at `done`, so no
  existing arm is loosened and `DecideProof` is not edited at all. `landed` and the terminal states
  (`done`, `cancelled`, `duplicate`) are absent from `from` on purpose, and unlike the land-trailer
  arm there is no same-commit repeat to admit: a landed or closed issue is not re-landed by an
  approval, and a second approval of the same key is a 200 no-op above this table.
- `control-plane/internal/hub/hubapi/work.go` ⚑ again: `CauseDesignApproved StateCause =
  "design_approved"`, appended in **`func init() { MoveCauses = append(MoveCauses,
  CauseDesignApproved) }`** — task 144's pattern at `task-144.md:185`, which keeps task 03's
  declaration at `task-03.md:220` untouched. `task-03.md:807`'s `want["MoveCauses"]` goes **6 → 7**
  in `TestWorkValueListsAreSets`; this task edits that number, and task 144's three outside causes
  count up from 7.
- `control-plane/internal/hub/server/work_states.go` ⚑ (task 10's server half): the `landed` write
  accepts the new cause and leaves `land_commit` NULL; the `no_criteria` branch and the
  `stopAtLandedWithoutServes` switch run exactly as they do for a land trailer, uncopied
- `control-plane/internal/hub/hubfake/work.go` ⚑ (task 04's fake, `task-04.md:876`, `:1009`): the
  same arm in both cause switches, so WS0 writers see the behaviour before the Hub exists
- `control-plane/internal/hub/server/approvals.go` ⚑ **(Hub task 22's file — this is the call
  site)**: after the approval, the design link and the snapshot are written, and **inside the same
  transaction**, one call to the new `closeIssuesForDesign(ctx, tx, designKey, approver)`. Without
  this row the close has nowhere to be called from, which is how round 1 failed.
- `control-plane/internal/hub/server/work_design_close.go` (new): `closeIssuesForDesign` — it reads
  every open issue whose **current** revision implements that exact key, and **moves only those
  whose head serves nothing** (`serves: []`) `→ landed` with cause `design_approved`, attributed to
  the approver, then handed to the unchanged done rule. **An issue whose head serves one or more
  criteria is skipped, silently and by design** (decision 2): no move, no fact, no event, nothing
  written against it by this path — its `issue_implements` row was written by the revision
  (decision 4) and the approval it now joins is the Hub's own approval row (decision 2a). Skipping
  is not optional bookkeeping: decision 5 gates both SQL guard arms on `NOT v_serves`, so attempting
  the move raises `42501` → `ErrStateGuard` **inside the approval's transaction**, which would roll
  the whole approval back. A canvas implemented by one chore and one brief-serving issue must
  approve cleanly and close exactly the chore
- `control-plane/internal/hub/migrate/sql/NNNN_work_w2.sql` ⚑ (task 74's file, a section appended
  before it is applied anywhere shared; the next free number instead if 74 has merged, rule H1):
  `issue_implements(issue_id, revision, design_key, canvas_id, canvas_rev)`, its RLS, its grants,
  the named constraint `issue_implements_key_form`, **and a `CREATE OR REPLACE FUNCTION
  guard_issue_state()` block** — decision 5, task 145's precedent at `task-145.md:441-449`. Without
  it the Go table admits the move and the database refuses it: the guard live after task 10
  (`task-10.md:185-199`) reads `WHEN 'landed' THEN v_cause = 'land_trailer'`, so `design_approved`
  raises `42501` → `ErrStateGuard` → 422 `state_not_allowed` on the only hop this task exists for.
- `control-plane/internal/hub/migrate/work_guards_integration_test.go` ⚑ (task 07's file,
  `task-07.md:61`, table at `:502-520`): the two test rows decision 5 names
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑: one authorisation-matrix row
  per new route, including the row that proves the sync principal is refused
- `control-plane/internal/hub/hubclient/work.go` ⚑ (task 04's file): **`WorkDesign`, a new
  standalone interface with exactly one method** —
  `ListIssuesForDesign(ctx context.Context, designKey string) (hubapi.DesignIssues, error)` — and
  one line appending it to `Work`. This is task 72's `WorkPlan` pattern
  (`task-72.md:876-886`, `:1315`) at one method instead of forty-six, and it is how SW.2 is allowed
  to grow: task 04 freezes the interface to *"an added interface"* per slice
  (`task-04.md:162-164`), **not** a method appearing inside `WorkIssues`. Declaring it and embedding
  it in the **same commit** as the fake and the client stub below is task 73's rule, so
  `var _ Hub = (*Fake)(nil)` never breaks between commits
- `control-plane/internal/hub/hubclient/fake_work_design.go` (new) and
  `hubclient/methods_work_design.go` (new): the fake's in-memory answer and the HTTP method, the two
  halves every other `Work` method has. **This task owns both**, rather than handing the method to
  task 82: 81b is not in 82's `dependsOn` and 82's own text names no design route, so a hand-off
  would leave the method unbuilt in whichever order the two run
- `control-plane/internal/hub/hubclient/conformance/work_design.go` (new): one conformance row, run
  against the fake and the real Hub, on task 05's pattern
- `control-plane/internal/api/work.go` ⚑ (task 16's relay): one `WorkRoutes` row for the read route,
  a thin call to `Deps.Work` (`task-16.md:21`) — which resolves to the `WorkDesign` method above —
  passed through untouched, problem and all (`task-16.md:25-32`)

**Routes.** Two, both added to task 07's list and its matrix in this task:

```
// GET /v1/designs/design:cv-7c01…@11/issues     member of the project's team; 200
// → the issues whose CURRENT revision implements exactly that key, newest first, and the
//   approval projected from Hub task 22's row (decision 2a; "design": null when unapproved)
{ "design": { "designKey": "design:cv-7c01…@11", "approvedBy": {…}, "approvedAt": "…" },
  "issues": [ { "id": "is-0a1b…", "alias": "EP-142", "revision": 3, … IssueHead … } ] }
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
2. An issue serving **one criterion with no evidence** is **untouched** by the approval: same
   state, `proof: ""`, `landCommit: ""`, **its event count and its fact count unchanged**, and the
   `issue_implements` row it had from its revision still there and unrewritten. A forced move with
   cause `design_approved` on that head is `42501` at the guard →
   422 `state_not_allowed` (decision 5). A second test: the same issue with
   `serves: []` and `notTiedReason: "chore"` is `done`, `no_criteria`, in the same write; and with
   the team's `stopAtLandedWithoutServes` on it stops at `landed`. *Mutants:* the approval closes
   the issue whatever it serves — i.e. fabricated proof; and the approval lands it at `landed` with
   `waiting`, which locks the Merge lens out of its own close (decision 2).
2b. `DecideProof` is byte-identical to task 10's. *Mutant:* a `design_approval` branch inside it.
3. The chore close is attributed to the approver on the **event's `who`**, appends exactly one
   event — task 10's existing `EventIssueState`, no new kind and no registry edit (decision 2b) —
   and one state fact, and writes **no `issue_proofs` landing row** (decision 3: the row's
   `landed_together` constraint cannot hold without a commit). *Mutant:* attributed to the Hub or to
   the sync principal; or an `issue_proofs` row with a NULL `land_commit`, which is a `23514`.
4. The sync principal calling the approval path is refused by the authorisation row, not by a
   handler check. *Mutant:* the guard lives in one handler and the next route forgets it.
5. A revision whose `implements` names a `brief:` key, a bare `cv-…`, or a canvas with no revision
   is 400 `implements_key_form`. *Mutant:* any key form accepted.
6. The index rebuilt from `issue_revisions` equals the stored index, after a create, a restore and
   an erase. *Mutant:* the index is written on create only.
7. `GET …/issues` for a canvas of another team is 404, never 403 (the reason task 11 records at
   `task-11.md:31-35`). *Mutant:* 403, which leaks that the canvas exists.
8. **The mixed canvas approves.** One canvas revision implemented by a `serves: []` chore *and* a
   brief-serving issue: the approval returns 200, the approval row, the design link and the snapshot
   are all committed, the chore is `done`/`no_criteria` and the serving issue is untouched.
   *Mutant:* `closeIssuesForDesign` without the `serves` filter — the guard's `NOT v_serves` raises
   `42501` inside the approval's own transaction, the whole approval rolls back and the route answers
   422 `state_not_allowed` on a request that asked to move no issue. This is the test that pins
   decision 2's filter to the file rather than to the prose.
9. `grep` finds no `DesignLanding` write in `store/`, and the read route returns `"design": null`
   for a readable but unapproved canvas revision. *Mutant:* a `design_landings` table, a second
   source of truth beside Hub task 22's approval row (decision 2a).

**Acceptance, per criterion (`OD-27`, no predictions).** (a) An issue whose revision implements an
approved canvas revision and serves **no** criterion reads `done`, `no_criteria`, `landCommit: ""`
with no click; the same issue under `stopAtLandedWithoutServes` reads `landed`. (a2) An issue that
serves a criterion is **byte-identical** after the approval — same state, same event count, same
fact count — and the approval against it is read, not written: the read route answers with that
issue under a non-null `design`, so the Design lens shows *approved, and these issues implement it*
while the issue itself waits for the Merge lens to close it with no reopen in the way. (a3) The
canvas that carries both kinds approves in one 200 and nothing rolls back. (b) No proof word outside
task 10's six is written, `DecideProof`'s source is unchanged, and **no event kind is added**.
(c) The reverse read returns the issues of a canvas and nothing else. (d) No percentage, bar, ETA or
"N of M" figure is produced by any of it.

**Does not claim.** A per-frame key (P5-H). A revocation path — the Hub has none (`P5:195`), so an
approval withdrawn is a phase 5 question, not this task's. A cross-project implement (the read is
inside the caller's readable set, as task 11's decision 3 has it). That phase 5's approval route
exists: until phase 5 is sync-merged this task codes against the Hub's own approval write and its
fake, and the part says so rather than asserting a path it has not seen.
