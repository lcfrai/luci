### Task 70: W1's local-mode demo, end to end, and its results

**Slice:** W1. **Size:** 3 focused hours beside the run itself (an estimate, not a measurement).
**Stream:** WD2 with WH (the last W1 task; it runs on the merged W1 tree). **Depends on** tasks
01-69. **Seams:** none. Creates `docs/verification/2026-09-25-phase-w-results.md`, which tasks 97,
113, 142, 174 and 187 append to in task order (the skeleton's *Proof and documents* table, `:322`).

`OD-27` is the rule: **proof per criterion, no prediction.** A parity row this section marks closed
carries the command that was run, an excerpt of what it printed or what a person saw, and the
commit it ran at. A row with no output beside it is not closed, whatever a part or a person says.
Nothing here is a measurement beyond the run it was taken in.

**Anchors, checked on 2026-09-26:** the house model `docs/verification/2026-09-17-phase-1-results.md`
(read: a lead line, *What was tested*, *The checks* as a table, *What is still unproven*,
*Reproducing this*); 05 §5.3's demo (`05-design.md:1174-1181`), §4.6's budgets (`:1098-1117`);
05 §1's matrix, whose slice column names W1 in **49** rows (selected with the awk of `check-slice.sh`
below while writing this part); the coverage map's *proof 70* column
(`2026-09-25-phase-w-work-layer-scopes/coverage.md:45-277`); task 113's W3 section shape
(`task-113.md:86-126`), which this section's headings must match; the Hub's `audit_events`
columns (`hub-contract.md:1762-1776`: `kind`, `subject`, `actor_user_id`, `canonical`); task 13's
`IssueStateData{from, to, cause}` (`task-13.md:159-162`); task 17's four people and
`dev-tenant.json` (`task-17.md:46-60`); phase 2 task 25's `playwright` library harness under
`desktop/e2e/` (`phase-2-parts/task-25.md:42-49`, `:344-350`).
**UNVERIFIED until merge:** `control-plane/cmd/hub/{compose.yaml,devstack.sh}` (Hub task 09,
`phase-h-hub.md:288`; not on `feat/hub` at `3a14dd70`), `desktop/e2e/` (phase 2 task 25, not built
on this branch), and the `hub.audit_events` table name as migrated. Step 1 takes the merged names.

#### Decisions made here, so nobody re-derives them

1. **The environment is named, first line.** Local Hub stack (`devstack.sh up`: Postgres in
   Docker, `devissuer`, `migrate`, `hub` on the host), the control plane on loopback, the desktop
   from `npm --prefix desktop run tauri dev`, **local mode** sandboxes (`OD-20`). Two people
   signed in through `devissuer`, on two Windows accounts or two desktop profiles: Dylan (the
   owner of EP-142) and Alice (who comments), both fictional (`dev-tenant.json`). The lead line
   gives the Hub, control-plane and desktop commits. **A local run proves nothing about the cloud
   path**, and the section says so.
2. **One check for every slice.** `docs/verification/phase-w/check-slice.sh <W1|W2|…>` selects the
   slice's rows from 05 §1 by its slice column, adds the rows 05 moved in by text (a committed
   `moved-rows.txt`: `W1 L-45` and `W1 L-117`, from 05 §5.3 rows 4 and 12), and checks the
   section's rows table. Task 113's gap 2 asked for this; tasks 97, 142, 174 and 187 call it, and
   task 113's `check-w3.sh` may become `check-slice.sh W3`.
3. **Fifty-one W1 rows.** The 49 the matrix names plus L-45 and L-117. That is the scope's fifty
   plus **L-175** (*"Open an issue in Claude Code…"*, matrix `W1`, coverage map *closed by 24*),
   which the scope's list left out; its local-mode half is observed at frame 5 and its cloud
   session stays UNVERIFIED (gap 1). **L-146** is recorded as its W1 half (the `Panacea-Issue`
   trailer at land, tasks 21-22); the release half is task 71's.
4. **Halves say which half.** A row whose slice cell names two slices (`W1 / W2`, `W1 / W5`,
   `exists / W1`) is recorded as its W1 half, and the cell says so: *"W1 half: list and board"*.
5. **Twelve frames, observed, never inferred** (`docs/verification/phase-w/w1-demo.md`). A frame's
   observation is what a person saw, quoted, or a line a command printed. *"The agent decided…"*
   is not an observation. Nothing is read from a harness's terminal (`OD-8`).
6. **Budgets are recorded, not asserted** (05 §4.6, 02 C8). `desktop/e2e/w1-budgets.e2e.mjs`
   times each act twenty times against the dev server in Chromium and writes the median and the
   worst with the machine's name. They are Chromium numbers, not WebView2's, and one machine's.
7. **The owner's eyes are their own rows.** The Desk at 125 % and 150 % with 30 fixture agents,
   and each of the four themes, go to the owner; the verdict is recorded in the owner's words, or
   *not yet shown* (`CLAUDE.md`: a screenshot is not evidence).

**Files:**

| Path | Change |
|---|---|
| `docs/verification/2026-09-25-phase-w-results.md` | new: the lead, *What was tested*, and the W1 section |
| `docs/verification/phase-w/w1-demo.md` | new: the twelve frames a person runs |
| `docs/verification/phase-w/check-slice.sh` | new: the coverage check every slice's section runs |
| `docs/verification/phase-w/moved-rows.txt` | new: rows 05 moved between slices by text, one per line |
| `docs/verification/phase-w/hub-facts.sql` | new: the post-run query of frame 10 |
| `docs/verification/phase-w/prefs-allow.txt` | new: the files allowed a local write, each naming the preference |
| `docs/verification/phase-w/testdata/{w1-complete,w1-missing-output}.md` | new: the check's fixtures |
| `desktop/e2e/w1-budgets.e2e.mjs` | new: the four budget timings, recorded |
| `desktop/src/test/w1Sweep.test.tsx` | new: `expectNoPrediction` over the W1 surfaces the demo shows |

#### The rows W1 closes, and what closes each

*Closed by* is the coverage map's column; *observed at* is the frame or the test whose `PASS` line
the section pastes. Step 1 replaces each behaviour with its part's real test name.

| Row | 05 §1 slice | Closed by | Observed at |
|---|---|---|---|
| L-2 | W1 | 75 (team half here: 08, 11) | frame 2: the import lands in team T's Triage |
| L-3 | W1 | 40, 46 | frame 3: the row reads `EP-142` |
| L-8, W1 half | W1 / W2 | 22, 23, 28, 46 | frames 7-8: `landed`, then `done`, fixed states only |
| L-9 | W1 | 25, 68 | frame 2: imported issues sit in the team's default `triage` |
| L-10 | W1 | 42, 48 | task 42's number-key *duplicate* test |
| L-13 | W1 | 28, 46 | frame 4: priority set before it leaves Triage |
| L-15 | W1 | 63 | task 63's *urgent notifies the assignee* test |
| L-17, team half | W1 | 46 (org half: 78, 90) | task 46's label picker test |
| L-24 | W1 | 48, 63 | task 48's relations test; *blocking someone's agent* (63) |
| L-29 | W1 | 59, 68 | task 68's create-from-a-span test |
| L-34 | W1 | 47, 49 | task 49's history-and-restore test |
| L-36 | W1 | 56, 66 | frame 4: the *Undo* chip after `T` |
| L-38 | W1 | 46, 53 | task 53's `I` test |
| L-39, solo half | W1 / W3 | 24, 48, 61 | frame 5: `Shift+R` starts Dylan's own agent; Dylan stays assignee |
| L-41 | W1 | 51 | frame 6: Alice's comment, a reaction, *resolve* |
| L-42 | W1 | 51 | task 51's inline-anchor test |
| L-45 (moved in) | W2 → W1 | 52 | task 52's attach test |
| L-46 | W1 | 11, 42, 45 | frame 3: Triage shows *not tied · imported* |
| L-47 | W1 | 11, 24, 42, 53, 61 | task 42's number keys test |
| L-48 | W1 | 11, 42 | frame 4: leaving Triage without priority is refused |
| L-98, W1 half | W1 / W2 | 47, 67 | task 47's route-is-a-link test |
| L-101, list and board | W1 / W4 | 40, 41, 43, 45, 66 | frame 7: the row flashes and does not move, list and board |
| L-102, W1 half | W1 / W2 | 40, 43 | task 43's group-by test |
| L-104 | W1 | 44 | task 44's display toggles test |
| L-106 | W1 | 58 | task 58's jump-to-issue test |
