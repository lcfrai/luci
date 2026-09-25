### Task 142: W4's proof and results

Stream **W4** (roadmap, release and reporting), about 2.5 hours of work beside the run itself (an
estimate, not a measurement). Depends on tasks **114-141**. Writes the W4 section of
`docs/verification/2026-09-25-phase-w-results.md` ⚑, owned jointly with tasks 70, 97, 113, 174
and 187 and appended in task order, after W3's (task 113). The section keeps task 113's shape
(`task-113.md:86-128`) so the six sections read alike; task 70 creates the file on the model of
`docs/verification/2026-09-17-phase-1-results.md` (built).

`OD-27` is the rule: **proof per criterion, no prediction.** A row this section marks closed
carries the command that was run, an excerpt of what it printed, and the commit it ran at. A row
with no output beside it is not closed, whatever a part or a person says.

**Decisions made here, so nobody re-derives them:**

1. **The environment is named, first line.** The run uses the local Hub stack
   (`docker compose -f control-plane/cmd/hub/compose.yaml up -d`, task 124's step 5; `compose.yaml`
   and `devissuer` are Phase H's, **planned**) with sign-in through `devissuer`, and the control
   plane in **local mode** (`OD-20`). Where a row needs the gateway, it uses phase 6's gateway
   **only if** phase 6 is on the sync base; the lead line says whether it was. A local run proves
   nothing about a cluster, and the section says so.
2. **The rows are re-derived, not copied.** `check-w4.sh` selects every row of 05 §1 whose slice
   column names W4. On 2026-09-26 that is **forty-one** rows (checked while writing this part by
   running the selection over `05-design.md`; the list matches the scope's, `tasks-141-160.md:106-109`).
   Four of them are **halves** whose other half is an earlier slice or an existing phase (L-67 with
   W2, L-101 with W1, L-82 and L-253 *exists*); the row says which half.
3. **One row that is not W4's is carried**: L-255's per-issue and per-loop half (task 135). 05 §1
   places L-255 in phase 6 (`05-design.md:447`), so the selection does not find it; the check reads
   it from an explicit carried-rows list, and the row says *W4 half; phase 6 closes*.
4. **PARTIAL stays PARTIAL.** L-50, L-72, L-74, L-118 and L-119 are recorded with the word
   *PARTIAL* in their row and the open part named (the table below). A PARTIAL row with no open
   part named fails the check.
5. **Conditional rows are recorded as found.** L-53 closes only once the calibration record's home
   exists (task 114's noted gap); L-144 only once P7 task 19's freeze record has merged; L-147 only
   in the half task 131 builds (notes by criterion) unless the drafted prose has a task (decision 9).
   On the day, each is *closed*, *PARTIAL (the open part)* or *not run (why)*, never assumed.
6. **One scripted run, fifteen frames** (`docs/verification/phase-w/w4-run.md`), following the
   scope (`tasks-141-160.md:90-103`). Frames 10-15 need phase 6 (the spend rollup or the gateway);
   without it they are **not run**, and their rows are *not run*, never *passed*.
7. **Observed, never inferred.** A frame's observation is what a person saw or a command printed,
   quoted. *"The draft looked right"* is not an observation; *"the update's body read `text·A` in
   `GET /v1/features/ft-…/updates` until Post"* is. Nothing is read from a harness's terminal (`OD-8`).
8. **The owner's look is its own row.** The W4 surfaces (sharing, initiatives, the composer, the
   timeline, milestones, release notes and freeze, insights, *since you last looked*, recurring
   issues, spend, the assists, drafts, proposals and the diff) are shown to the owner and the
   verdict is recorded in the owner's words (`OD-1`; `CLAUDE.md`: *"Visual acceptance is the
   owner's eyes"*).
9. **Gaps are carried, not fixed here.** Task 139 found that L-147's drafted release-note prose has
   no task (`task-139.md`, gap 1); task 134's three surfaces do not call `expectNoPrediction`
   (grep: zero matches in `task-134.md`). The section lists both under *What is still unproven*,
   and the sweep below fails until 134's tests call the helper.
10. **What stays unverified is listed by name**: every assist row if phase 6 is not on the sync
    base (136-140); phase 7's manifest and freeze if P7 task 19 has not merged (131); the
    calibration dial's accept counts' home (task 114's gap); the Hub's `edit_proposal` kind (task
    141's gap 1); the owner's visual acceptance of the timeline, insights and diff surfaces.

**Files:**
- `docs/verification/2026-09-25-phase-w-results.md` ⚑ — the W4 section (modified; task 70's file)
- `docs/verification/phase-w/w4-run.md` — the fifteen frames a person runs (new)
- `docs/verification/phase-w/check-w4.sh` — the coverage check (new)
- `docs/verification/phase-w/testdata/{w4-complete,w4-missing-output,w4-partial-unnamed}.md` —
  the check's fixtures (new)
- `desktop/src/test/w4Sweep.test.ts` — every W4 surface's test calls `expectNoPrediction` (new)
