### Task 97: W2's proof and results

**Slice:** W2. **Size:** 2.5 focused hours beside the fortnight itself (an estimate, not a
measurement). **Stream:** WD2 with WH (the last W2 task; it runs on the merged W2 tree).
**Depends on** tasks 72-96. **Seams:** none. Appends the **W2 section** to
`docs/verification/2026-09-25-phase-w-results.md`, which task 70 created and tasks 113, 142, 174
and 187 append to after this one, in task order (the skeleton's *Proof and documents* table,
`2026-09-25-phase-w-work-layer.md:322`).

`OD-27` is the rule, and it is the whole task: **proof per criterion, no predictions.** A W2
parity row this section marks closed carries the command that was run, an excerpt of what it
printed or of what a person saw, and the commit it ran at. A row with no output beside it is not
closed, whatever a part, a test name or a person says. Nothing here is a measurement beyond the
run it was taken in, and the section says which runs those were.

**Anchors, checked on 2026-09-26.** Task 70's part is the model and the machinery this task
reuses, not re-derives: its results-file shape (`task-70.md:139-212`), its
`docs/verification/phase-w/check-slice.sh` (`:262-323`), its `moved-rows.txt` and
`prefs-allow.txt` (`:324-339`), its `hub-facts.sql` (`:340-382`) and its `w1Sweep.test.tsx`
(`:463-537`). Task 113's W3 section shape (`task-113.md:86-129`) is the heading set every later
section matches, this one included. The scope's W2 row list is
`2026-09-25-phase-w-work-layer-scopes/tasks-81-100.md:385-408`; the coverage map's *proof 97*
column is `2026-09-25-phase-w-work-layer-scopes/coverage.md:45-277`. 05's W2 surfaces:
cycles and convergence (`05-design.md` §2.3), triage and the digest (§2.4), views and favourites
(§2.6), themes (§4.4 with `02-design-language.md` C1 and D2). Task 82's end-to-end file and its
`-tags integration` build tag are `task-82.md` (Step 1 re-reads its final name).
**UNVERIFIED until merge:** `control-plane/cmd/hub/compose.yaml` (Hub task 09,
`phase-h-hub.md:288`), `desktop/e2e/` (phase 2 task 25), and every W2 file this section cites by
line, because W2 is not built when this part is written. Step 1 re-reads each and corrects the
section's citations before a single row is filled in.

#### Decisions made here, so nobody re-derives them

1. **The environment is named on the section's first line, and it is local mode.** Local Hub
   stack (`control-plane/cmd/hub/devstack.sh up`), the control plane on loopback, the desktop from
   `npm --prefix desktop run tauri dev`, sandboxes **local** (`OD-20`), sign-in through
   `devissuer` with task 17's four fictional people. The line gives the Hub, control-plane and
   desktop commits. **A local run proves nothing about the cloud path**, and the section says so
   in its own words, not by implication.
2. **`check-slice.sh W2` is the gate, unchanged.** Task 70 already takes the slice as an argument
   and already reads `moved-rows.txt`; this task adds **rows**, never code. If W2 needs a moved
   row (05 §5.3 prices a row in W2 that the matrix cell leaves in another slice), it is a line in
   `moved-rows.txt` with a reason, and the part records the addition. If `check-slice.sh` needs a
   change to pass, that is a defect in the section, not in the script.
3. **Fifty-two W2 rows, and the list is the scope's, re-derived.** The scope names L-8 (W2 part),
   L-11, L-12, L-14, L-16, L-17 (org), L-18 to L-23, L-25, L-26, L-30, L-33, L-35, L-37, L-44,
   L-57 to L-61, L-63 to L-65, L-67 (W2 part), L-68 to L-71, L-93 to L-96, L-98 (W2 part), L-99,
   L-102 (W2 part), L-103, L-105, L-107, L-109 to L-111, L-115, L-116, L-127, L-129, L-153, L-234
   and L-235. Step 1 runs `check-slice.sh W2` against an **empty** section first: its "missing"
   list is the authority, and any row the scope names that the matrix does not is recorded as a
   correction, not quietly dropped.
4. **A row whose scope note says *"W2 part"* records only that half, and says which half.**
   L-8, L-67, L-98 and L-102 are split with W1 or W3; their cells read *"W2 half: …"*, and the
   other half points at task 70's or task 113's row by its section heading.
5. **L-62 stays a SKIP and is still written down.** Its row carries the plain-facts substitute
   from 05 §1 and the sentence *"not built; this is what we do instead"*. A SKIP with no
   substitute recorded is a hole, so the row is present and marked `SKIP` in the status column.
6. **L-45 and L-117 are not this section's.** 05 prices them W2; the skeleton builds them in W1
   (tasks 13 and 52). Their cells read *"closed in W1 — see the W1 section, task 70"* and carry
   no new command. `moved-rows.txt` already holds both (`task-70.md:324-339`), so
   `check-slice.sh W2` does not ask for them.
7. **The fortnight is a script, not a story.** `docs/verification/phase-w/w2-fortnight.md` lists
   **ten frames** in order; each frame names the person, the click or command, and the observation
   as a quote or a printed line. A frame's observation is what a person saw. *"Convergence
   updated"* is not an observation; *"`Converged 7/12` in the cycle header, from `6/12`"* is.
8. **Both new themes are shown, not asserted.** Tasks 95-96 generate four themes; the owner sees
   `graphite`, `paper` and the high-contrast pair on one screen, and the verdict is recorded in
   the owner's words (`CLAUDE.md`: *"Visual acceptance is the owner's eyes"*). A contrast number
   from a test is proof of the number, not of the look; the section keeps them apart.

#### The rows W2 closes, and what closes each

The grouping the section's rows table is ordered by. The *closed by* column is the coverage map's
(`coverage.md:45-277`); Step 1 re-derives it and records any disagreement rather than trusting
this list.

| Group | Rows | Closed by (tasks) | The proof shape |
|---|---|---|---|
| Cycles and cadence | L-57 to L-61, L-63 to L-65 | 72-78 | a frame plus a Go or desktop test |
| Convergence per criterion | L-67 (W2 part), L-68 to L-71 | 79-83 | frame 5 plus task 82's `-tags integration` run |
| Triage and the inbox | L-11, L-12, L-14, L-16, L-17 (org) | 84-87 | a frame plus a desktop test |
| Move, split, duplicate | L-18 to L-23, L-25, L-26 | 88-90 | a frame plus a Hub row from `hub-facts.sql` |
| Views, saving, favourites | L-30, L-33, L-35, L-37, L-44 | 91-93 | a frame plus a desktop test |
| Defer with a reason | L-93 to L-96 | 89, 94 | frame 8, which shows the reason stored |
| The digest | L-98 (W2 part), L-99, L-102 (W2 part), L-103, L-105, L-107 | 94 | frame 9 plus the email and desktop renders |
| Notifications and callouts | L-109 to L-111, L-115, L-116, L-127, L-129 | 94-96 | a desktop test plus one frame each |
| Themes and preferences | L-153, L-234, L-235 | 95, 96 | a contrast test plus the owner's look |
| Split halves | L-8 (W2 part) | 72, 84 | the W2 half only, named in the cell |
| SKIP | L-62 | — | the plain-facts substitute, written out |

#### Interfaces — complete: the W2 section, appended to the results file

Appended below task 70's W1 section, headings identical to it and to task 113's, so
`check-slice.sh` finds `### The rows` the same way in every section.

```markdown
## W2: the team's week — <date>

**Environment:** local Hub stack (`devstack.sh up`) at `<sha>`, control plane on `127.0.0.1` at
`<sha>`, desktop (`tauri dev`) at `<sha>`, local-mode sandboxes (`OD-20`) on <machine>. Four
people through `devissuer` (task 17). This run proves nothing about the cloud path, about Graph
mail in a real tenant, or about the iCal feed in Outlook.

### The run (docs/verification/phase-w/w2-fortnight.md)

| Frame | Command or act | Observed (quoted) | Time | Commit |
|---|---|---|---|---|
| 1 cadence | Dylan sets a 2-week cadence, Mon 09:00 | … | … | … |
| 2 triage | Alice triages 9 items from the inbox | … | … | … |
| 3 promote | a request becomes a feature, provenance kept | … | … | … |
| 4 cycle | the cycle starts; 12 issues, 12 criteria | … | … | … |
| 5 converge | the cycle view after two lands | `Converged 7/12` (was `6/12`) | … | … |
| 6 move | 3 issues move project; ids follow | … | … | … |
| 7 split | one issue splits into two, parent kept | … | … | … |
| 8 defer | one issue deferred, reason recorded | … | … | … |
| 9 digest | the Monday digest, desktop and email | … | … | … |
| 10 view | a view saved, favourited, reopened | … | … | … |

### The rows

| Row | Task | Test (real name) or frame | Command | Observed (excerpt) | Commit | Status |
|---|---|---|---|---|---|---|
| L-8 | 72 | frame 1 | … | … | … | closed (W2 half: cadence) |
| L-62 | — | — | — | plain-facts substitute: <text from 05 §1> | — | SKIP |
```

The same section continues in the same file:

```markdown
### The checks

| Check | Command | Result line (quoted) | The defect it would catch |
|---|---|---|---|
| Go | `go test ./control-plane/... -race -count=1` | … | a cycle, triage or digest regression |
| Hub integration | `go test -tags integration ./control-plane/internal/hub/... -count=1` | … | a store, RLS or matrix regression |
| End to end | task 82's run with `-tags integration` against `control-plane/cmd/hub/compose.yaml` | … | convergence computed from stale criteria |
| Desktop | `npm --prefix desktop test` | … | a W2 surface regression |
| Types | `npm --prefix desktop run typecheck` | … | a wire type drift |
| No predictions | `npm --prefix desktop test -- w2Sweep` | … | a W2 surface that predicts (`OD-27`) |
| Rows | `bash docs/verification/phase-w/check-slice.sh W2` | … | a row closed from a self-report |
| Local writes | `bash docs/verification/phase-w/check-slice.sh --local-writes` | … | a Hub record kept on disk (task 55's rule) |
| Hub facts | `psql … -f docs/verification/phase-w/hub-facts.sql` | … | a cycle closed by a sync principal |

### The owner's look

| Surface | Shown at | The owner's words (quoted) | Verdict |
|---|---|---|---|
| cycle view and convergence | frame 5 | … | … |
| triage inbox and digest | frames 2, 9 | … | … |
| `graphite`, `paper`, both high-contrast | after frame 10 | … | … |

### What is still unproven after W2

- the cloud path: every surface here ran in local mode (`OD-20`)
- Graph mail delivery in a real tenant; the digest email was rendered and read locally
- the iCal feed opened in Outlook; only the feed's bytes were checked
- anything W3 adds: no agent writes a comment in this run
```

`feat/work-layer` is not merged onward before the owner's-look table is filled in (`OD-1`;
`CLAUDE.md`: *"Visual acceptance is the owner's eyes, not a self-report"*).

#### Interfaces — complete: `docs/verification/phase-w/w2-fortnight.md`

Same shape as task 70's `w1-demo.md` (`task-70.md:213-261`): a lead line naming the stack and the
people, then one block per frame. A frame that was not observed is written **not run**, never
left blank and never filled in from a part's expectation.

```markdown
# W2: a fortnight in local mode

Stack: `devstack.sh up` (Hub `<sha>`), control plane `<sha>`, desktop `<sha>`. People: Dylan,
Alice, Sam, Priya (`dev-tenant.json`, task 17), all fictional. Mode: local (`OD-20`).

## Frame 1 — the cadence

**Act:** Dylan opens *Team · Cadence*, picks 2 weeks, Monday 09:00, and saves.
**Observed:** "<quoted from the screen, or the line a command printed>"
**Hub:** `select kind, subject from hub.audit_events order by at desc limit 1;` → `<row>`
**Rows:** L-8 (W2 half), L-57. **Commit:** `<sha>`. **Time:** `<hh:mm>`.
```

#### Tests to write first — `desktop/src/test/w2Sweep.test.tsx`

Task 70's `w1Sweep.test.tsx` (`task-70.md:463-537`) is the model: one file, one sweep, every W2
surface mounted with a fixture and handed to task 20's `expectNoPrediction`. Written before the
section is filled in, so a surface that skipped the helper fails while there is still time to fix
it rather than at the owner's review.

```tsx
import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { expectNoPrediction } from "./expectNoPrediction";
import { w2Fixtures } from "./fixtures/w2";

// Every W2 surface, in one sweep. A new W2 surface adds a row here or this test is a lie.
const surfaces = [
  ["cycle view", w2Fixtures.cycleView],
  ["convergence panel", w2Fixtures.convergence],
  ["triage inbox", w2Fixtures.triage],
  ["move and split dialogs", w2Fixtures.moveSplit],
  ["saved views and favourites", w2Fixtures.views],
  ["defer dialog", w2Fixtures.defer],
  ["digest, desktop render", w2Fixtures.digest],
  ["notification list and callouts", w2Fixtures.notifications],
  ["theme picker, four themes", w2Fixtures.themes],
] as const;

describe("W2 surfaces state facts, never predictions (OD-27)", () => {
  it.each(surfaces)("%s", (_name, mount) => {
    const { container } = render(mount());
    expectNoPrediction(container);          // mutant: a surface that skipped the helper
  });

  it("covers every W2 surface the router registers", async () => {
    const { w2RouteIds } = await import("../work/routes");
    // mutant: a surface added to the router and left out of this sweep
    expect(new Set(surfaces.map(([n]) => n)).size).toBe(w2RouteIds.length);
  });
});
```

The mutants this file kills, named: a W2 surface that renders *"should converge by Friday"* or
*"likely to slip"* (the helper's word list, task 20); a surface added to the router after this
part and never swept; a fixture with no data, which would pass the helper by rendering nothing
(`w2Fixtures` are task 82's end-to-end fixtures, not empty objects).

The **coverage mutant** is `check-slice.sh`'s, not a new test: a row in the section's table with
a task and a test name but an empty *Observed* or *Commit* cell exits non-zero (task 70's script,
the `awk` over `### The rows`). That is the mutant the scope names — *a row marked closed from a
self-report* — and it is already killed by task 70's script, so this task adds rows and a
regression guard, not a second checker.

#### Steps

1. **Re-read, then correct.** Task 70's shipped `check-slice.sh`, `moved-rows.txt`,
   `w1-demo.md` and results head; task 82's real end-to-end test name and build tag; task 20's
   `expectNoPrediction` import path; the coverage map's *proof 97* column. Every citation in this
   part that moved is corrected in the section, and the move is listed under *Gaps* below.
2. **Derive the row list before writing a word of prose.** `check-slice.sh W2` against the
   empty section prints the missing rows: that list, not the scope's paragraph, is the table's
   spine. Record any row the scope named that the matrix does not, and the reverse.
3. **Write `w2Sweep.test.tsx` and watch it fail** on at least one surface (if every surface
   already passes, the test proves the helper is wired, not that it works: flip one fixture to a
   predicting string, see it fail, flip it back).
4. **Run the five commands**, in this order, and paste an excerpt of each into *The checks*:
   the Go suite with `-race`, the Hub integration suite, task 82's end-to-end run, the desktop
   suite and typecheck. A failure is fixed or recorded as an open row; it is never omitted.
5. **Run the fortnight**, ten frames, two people at the keyboard, writing each frame's
   observation into `w2-fortnight.md` as it happens. Nothing is written from memory at the end.
6. **Fill the rows table** from the frames and the test output, one row at a time, with the
   commit each ran at. L-62's SKIP substitute is written out in full.
7. **Run `check-slice.sh W2` until it exits 0**, then `--local-writes`, then `hub-facts.sql`.
8. **Show the owner** the W2 surfaces and all four themes, and write the verdict in the owner's
   words. Commit the section, `w2-fortnight.md` and any `moved-rows.txt` line together.

#### Acceptance

```bash
# 1. The rows check passes for W2, and says how many rows it checked.
bash docs/verification/phase-w/check-slice.sh W2
# expect: exit 0, last line "W2: 52 rows, each with a command, an excerpt and a commit"

# 2. No W2 surface predicts.
npm --prefix desktop test -- w2Sweep
# expect: "Test Files  1 passed", and the sweep's row count equals w2RouteIds.length

# 3. Every check row quotes a real result line, not a placeholder.
grep -n '…' docs/verification/2026-09-25-phase-w-results.md | awk -F: '$1 > <W2 head line>'
# expect: no output (every ellipsis in the shipped section replaced by observed text)

# 4. The owner's look is recorded in words, not a tick.
awk '/^### The owner.s look/{on=1} on && /^### /{c++} on && c>1{exit} on' \
  docs/verification/2026-09-25-phase-w-results.md
# expect: three rows, each with a quoted sentence; no row reading "ok" or "approved"

# 5. The section claims no cloud result.
grep -c 'local mode' docs/verification/2026-09-25-phase-w-results.md
# expect: >= 2 (the W2 environment line and the unproven list)
```

#### What this task does NOT claim

- **W2 in the cloud.** Every frame ran in local mode (`OD-20`). The cloud path is phase 6
  stream A and phase 8 stream C; the section lists it **UNVERIFIED**.
- **Graph mail delivery in a real tenant.** The digest was rendered and read locally (task 94's
  fake transport). A real send is UNVERIFIED.
- **The iCal feed in Outlook.** The feed's bytes were checked; no calendar client subscribed.
- **URL-scheme registration on a managed device.** Deep links were exercised on an unmanaged
  developer machine; KPMG MDM behaviour is UNVERIFIED.
- **That a passing row is a good feature.** A row proves the criterion was observed once, in one
  run, by one pair of people. The owner's look is the judgement; the table is only the evidence.
- **Any performance figure.** No budget in this section is a measurement beyond its run;
  05 §4.6's budgets are task 70's W1 numbers and are not re-stated here as facts.
- **That W2's tests cover W3.** No agent writes a comment in this run; the kill switch, guidance
  and agent attribution are tasks 98-113's, recorded in the W3 section.

#### Gaps found while writing this part (for the skeleton's owner and the named tasks)

1. **The scope's row list and the matrix may disagree.** The scope names ~52 rows in prose; the
   matrix's slice column is the machine-readable source and `check-slice.sh W2` is the arbiter.
   If they differ, the matrix wins and the difference is recorded in the section — the skeleton's
   row for task 97 should say so, as task 70's does for W1.
2. **L-45 and L-117 are priced W2 in 05 but built in W1** (tasks 13, 52). Task 70 already put
   both in `moved-rows.txt`; this part depends on that file shipping with task 70. If task 70's
   part changes, this section's row count changes with it.
3. **L-17's *org* half has no named owner in the coverage map** beyond "triage". Task 87 is
   assumed here; the skeleton should confirm, or the row moves to W3 with the org switch (99).
4. **One file, six writers** (70 creates; 97, 113, 142, 174 and 187 append). The order is task
   order and nothing enforces it. Two of them running in the same week will collide on
   `2026-09-25-phase-w-results.md`; the skeleton should name a merge rule, as it does for the Hub
   deltas. This part appends only the W2 section and never edits another slice's.
5. **`w2Fixtures` has no owning task.** Task 82's end-to-end fixtures are the nearest; if task 82
   ships them under a different name or only in Go, task 97 builds the desktop fixtures itself and
   the estimate above is short by about half an hour.
6. **No task owns the results file's *What was tested* row for W2.** Task 70 created the table
   with the W1 row only; this task appends the W2 row, and each later section does the same. The
   skeleton should state the rule once rather than leaving it to each part.
