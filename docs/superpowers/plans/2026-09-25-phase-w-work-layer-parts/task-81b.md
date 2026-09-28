### Task 81b: `implements` — the write and read paths, and the design approval that closes a design issue

W2, stream WH. Seams: **SW.1** (an added *use*, not an added field) and **SW.4** (an added
done-rule input). Depends on task 10 (the state move and the done rule), task 23 (the evaluator),
task 74 (the W2 migration) and, for the approval record, **P5-21**.

Anchors, all read on 2026-09-28 unless marked. Phase W's own parts:
`2026-09-25-phase-w-work-layer-parts/task-02.md` and `task-03.md` (cited `task-NN.md:N`). The
design spec `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (cited `05:N`) and
`04-internal-fit.md` §7.7 at `:587-590`. The phase 5 plan
`.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md` (cited
`P5:N`), which is **another worktree's branch**: every path it names is **UNVERIFIED until phase 5
is sync-merged here**, and rule H2 gives that phase's file the win on any conflict.

**Read this before anything else: the wire is already done.** `implements` is not missing.
`IssueRevisionContent.Implements []string` — a list of `design:…` keys — is declared at
`task-02.md:243`, is part of the canonical bytes every revision digest covers, appears in all three
golden vectors (`task-02.md:430`, `:479`, `:520`, the middle one carrying
`design:cv-7c01d2e3f4a5b6c7@11`), is on the API type at `task-03.md:424`, and has its field-name row
at `task-03.md:738`. **So this task adds no wire field, reopens no golden vector, and does not
reopen task 05 under the WS0 rule.** It adds a store, two routes and one done-rule input. If you
find yourself editing `workcanonical.go`, stop: you have misread the task.

What was missing, and is what you build: nothing **wrote** the field from a surface, nothing
**read** it back the other way (*which issues implement this canvas?*), and nothing acted on
04 §7.7's second half, *a design approval can close a design issue*.

**Vocabulary, settled — do not reopen it.** 05 says *frame*; phase 5's document model says
**artboard** (`P5:359`), and its design key `design:<canvasId>@<rev>` names a **canvas revision,
not a subset** (`P5:232`, decision P5-H: *"subsets need a second key form"*). So `implements` names
a canvas revision, exactly as `task-02.md:243` already has it, and there is no per-frame key to
point at. A frame is a locator inside the revision. **Do not invent a second key form.**

**Four decisions, made here.**

1. **The match is the exact key.** An approval of `design:cv-…@12` does not close an issue whose
   revision implements `@11`. The approval's digest covers the revision that was reviewed, and a
   brief edit shows as `superseded` rather than voiding it (`P5:183`, D3 option A *Pin and mark*).
   Widening this to "any revision at or after" is a *later* row, not a W2 liberty.
2. **`design_approval` is evidence, never a hand move.** It satisfies a criterion the issue serves
   when the issue's **current** revision implements the approved key. An issue that also serves a
   criterion of the code class still waits for that criterion's own evidence, so a mixed issue
   never closes early. This is rule 5's *proof per criterion* applied to a second evidence class,
   not a new way to skip proof.
3. **The move is written as the approver.** Only a human approves, and only through the chrome
   (`P5:115-117`, ADR-H1 R5). So rule 2 holds unbroken: no sync or integration principal can reach
   this path. The move's cause is `observed`; the issue reads *done, proven by approval
   `design:cv-…@rev`*, never *done, not proven*, and no reason string is required.
4. **The index is derived, never authoritative.** `issue_implements` is a reverse index over the
   revisions; it is rebuildable from them, and the rebuild is a test, not an operator procedure.

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's file, modified additively):
  `ValidateImplements([]string) error`, which parses each entry through `hubapi/keys.go` and
  refuses anything that is not `design:cv-…@<rev>`; `DesignApprovalEvidence{DesignKey, ApprovedBy,
  ApprovedAt}`; `ImplementsRow{IssueID, Revision, DesignKey}`; the two codes this task needs
  (`implements_key_form`, `design_not_approved`) if task 03 has not declared them
- `control-plane/internal/hub/hubapi/work_implements_test.go` (new): the pure rules, no database
- `control-plane/internal/hub/store/work_implements.go`, `server/work_implements.go` (new): the
  reverse index, written in the **same transaction** as the revision it indexes, and
  `GET /v1/designs/{designKey}/issues`
- `control-plane/internal/hub/store/work_revisions.go` ⚑ (task 09's file): the revision write fills
  the index; restore (task 09's path) fills it too, which is decision 4's second test
- `control-plane/internal/hub/server/work_states.go` ⚑ (task 10's file): the done rule gains the
  `design_approval` evidence kind, as one more input to SW.4, not a branch around it
- `control-plane/internal/hub/server/work_design_close.go` (new): the Hub's own approval write
  (contract §7.7, `P5:195`) closes every open issue whose current revision implements that exact
  key and whose remaining criteria are satisfied
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

The close path has **no route of its own**. It runs inside the Hub's design-approval write
(contract §7.7, one call that writes approval, design link and snapshot, `P5:195`), in the same
transaction, so an approval that is rolled back closes nothing.

**Tests first.** Each names the mutant it kills.

1. An approval at `@12` leaves an issue implementing `@11` open. *Mutant:* matching on `canvas_id`
   closes it.
2. An issue serving one design criterion and one code criterion stays `started` after the approval.
   *Mutant:* any approval closes the whole issue.
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
approved canvas revision, and which serves only that criterion, turns `done` with no click, and the
proof is the approval key. (b) A mixed issue does not. (c) The reverse read returns the issues of a
canvas and nothing else. (d) No percentage, bar, ETA or "N of M" figure is produced by any of it.

**Does not claim.** A per-frame key (P5-H). A revocation path — the Hub has none (`P5:195`), so an
approval withdrawn is a phase 5 question, not this task's. A cross-project implement (the read is
inside the caller's readable set, as task 11's decision 3 has it). That phase 5's approval route
exists: until phase 5 is sync-merged this task codes against the Hub's own approval write and its
fake, and the part says so rather than asserting a path it has not seen.
