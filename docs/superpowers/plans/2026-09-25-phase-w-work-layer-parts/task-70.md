### Task 70: W1's local-mode demo, end to end, and its results

**Slice:** W1. **Size:** 3 focused hours beside the run itself (an estimate, not a measurement).
**Stream:** WD2 with WH (the last W1 task; it runs on the merged W1 tree). **Depends on** tasks
01-69 and 23b. **Seams:** none. Creates `docs/verification/2026-09-25-phase-w-results.md`, which tasks 97,
113, 142, 174 and 187 append to in task order (the skeleton's *Proof and documents* table, `:323`).

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
   and each theme W1 builds, go to the owner; the verdict is recorded in the owner's words, or
   *not yet shown* (`CLAUDE.md`: a screenshot is not evidence). W1 builds `graphite` and the
   provisional `paper` (task 26) and honours Windows `forced-colors` (task 34); 02 C1's other two,
   `graphite-hc` and `paper-hc`, are W2's task 96, so the scope's "four themes" is three screens
   here (gap 2).

**Files:**

| Path | Change |
|---|---|
| `docs/verification/2026-09-25-phase-w-results.md` | new: the lead, *What was tested*, and the W1 section |
| `docs/verification/phase-w/w1-demo.md` | new: the twelve frames a person runs |
| `docs/verification/phase-w/check-slice.sh` | new: the coverage check every slice's section runs |
| `docs/verification/phase-w/moved-rows.txt` | new: rows 05 moved between slices by text, one per line |
| `docs/verification/phase-w/hub-facts.sql` | new: the post-run query of frame 10 |
| `docs/verification/phase-w/prefs-allow.txt` | new: the files allowed a local write, each naming the preference |
| `docs/verification/phase-w/w1-backlog.csv` | new: frame 2's twelve fictional rows |
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
| L-108 | W1 | 60 | frame 7: `Space` peeks the landed row's span |
| L-112 | W1 | 45, 65 | task 45's My Issues focus-order test |
| L-113 | W1 | 65, 66 | frame 9: the Desk driven by keys only |
| L-114 | W1 | 62, 64 | frame 9: LANDED under NEEDS YOU, never live while open |
| L-117 (moved in) | W2 → W1 | 52 | task 52's subscribe-on-comment test; frame 6 |
| L-124 | W1 | 49 | frame 6: Alice's comment and the agent's line carry their author class |
| L-126 | W1 | 49, 50 | task 50's slash menu test |
| L-127, W1 half | W1 / later | 50 | task 50's table and callout test (Mermaid stays later) |
| L-146, W1 half | W1 | 21, 22 | frame 7: the land commit carries `Panacea-Issue: is-…` |
| L-175, local half | W1 | 24 | frame 5: Start opens a local-mode session with the issue in context |
| L-181 | W1 | 24, 46, 48, 61 | frame 5: the assignee stays Dylan while the agent runs |
| L-191 | W1 | 24, 47, 67 | task 24's issue-in-context test; frame 5 |
| L-198, W1 half | W1 / W5 | 21, 24, 61 | frame 5: the lane's branch name from the issue |
| L-200, in-product half | W1 / W5 | 21, 22 | frame 7: the commit linked from the trailer, not a webhook |
| L-201, in-product half | W1 / W5 | 22 | frame 7: the `landed` move from phase 3's land |
| L-220, W1 half | W1 / W5 | 25, 69 | frame 2: the CSV import dialog and its result |
| L-221 | exists / W1 | 59 | task 59's palette-opens-beside-its-trigger test |
| L-222 | W1 | 59 | task 59's prefix test |
| L-223 | W1 | 53, 54 | task 53's grammar test (`Shift+O` stays W3's) |
| L-224 | W1 | 54 | task 54's `?` sheet test |
| L-225 | W1 | 29, 44, 66 | task 44's bulk bar test |
| L-226 | W1 | 54, 59 | task 54's *every menu item shows its key* test |
| L-227 | W1 | 54 | task 54's safe-triangle test |
| L-228 | W1 | 37, 55, 56 | frame 11: `budget.paint`, recorded; task 56's same-frame test |
| L-229 | exists / W1 | 55, 57 | task 57's checkpoint test |
| L-234, W1 half | W1 / W2 | 26, 27 | frame 12: the four themes, to the owner's eyes |

Fifty-one rows. None is closed by this table: each is closed when the section's row carries its
command, its observed excerpt and its commit, and `check-slice.sh W1` passes.

#### Interfaces — complete: the results file as this task creates it

The file's head is written once, here; each later slice appends its own `## Wn:` section and one
line under *What was tested*. The W1 section's headings are the ones task 113 copies.

```markdown
# Phase W verification results

Proof per criterion, no prediction (`OD-27`). A row is closed only where it carries the command
that was run, what it printed or what a person saw, and the commit it ran at. Every figure here
was taken in the run it names and is not a measurement beyond it.

## What was tested

| Slice | Environment | Commits (Hub · control plane · desktop) | Rows | Check |
|---|---|---|---|---|
| W1 | local Hub stack, local mode (`OD-20`) | `<sha>` · `<sha>` · `<sha>` | 51 | `check-slice.sh W1`: <its last line> |

## W1: the first adoptable slice, solo mode — <date>

**Environment:** local Hub stack (`devstack.sh up`) at `<sha>`, control plane on `127.0.0.1` at
`<sha>`, desktop (`tauri dev`) at `<sha>`, local-mode sandboxes on <machine>. This run proves
nothing about the cloud path (phase 6 stream A, phase 8 stream C).

### The run (docs/verification/phase-w/w1-demo.md)

| Frame | Command or act | Observed (quoted) | Time | Commit |
|---|---|---|---|---|
| 1 stack | … | … | … | … |

### The rows

| Row | Task | Test (real name) or frame | Command | Observed (excerpt) | Commit |
|---|---|---|---|---|---|
| L-2 | 75 | … | … | … | … |

### The checks

| Check | Command | Result line (quoted) | The defect it would catch |
|---|---|---|---|
| Go | `go test ./control-plane/... -race -count=1` | … | a Work route regression |
| Hub integration | `go test -tags integration ./control-plane/internal/hub/...` | … | a store or RLS regression |
| Desktop | `npm --prefix desktop test` | … | a surface regression |
| Types | `npm --prefix desktop run typecheck` | … | a wire type drift |
| Local writes | `bash docs/verification/phase-w/check-slice.sh --local-writes` | … | a Hub record kept on disk (task 55's rule) |
| Hub facts | `hub-facts.sql` (frame 10) | … | a `done` by a sync principal, or with no observed cause |

### The budgets (recorded, not asserted)

| Budget | Design value (05 §4.6) | Median of 20 | Worst of 20 | Machine, browser |
|---|---|---|---|---|
| `budget.paint` | ≤ 16 ms | … | … | … |
| `budget.palette` | ≤ 50 ms | … | … | … |
| `budget.peek` | ≤ 100 ms | … | … | … |
| `budget.ack` | ≤ 10 s to the first span | … | … | … |

### The owner's look

<the owner's words, verbatim, with the date, for: the Desk at 125 % and at 150 % with 30 fixture
agents; `graphite`; the provisional `paper`; Windows `forced-colors`; or "not yet shown". The
`-hc` pair is W2's (task 96) and is not shown in W1 (gap 2)>

### What is still unproven

- <one line each, with the test that would settle it>

## Reproducing this

<the commands of w1-demo.md's frame 1, and `check-slice.sh W1`>
```

Every `…` and `<…>` is filled from the run. A cell left `…` fails `check-slice.sh`. An excerpt is
shortened with `[cut]`, never `…`, so the check can tell an unfilled cell from a shortened one.

#### Interfaces — complete: `docs/verification/phase-w/w1-demo.md`

```markdown
# W1 run: the first adoptable slice, local mode

Two people signed in through `devissuer`: Dylan (team T's admin, EP-142's assignee) and Alice (a
member). Record each observation as quoted text or a pasted output line, with the local time from
the desktop's clock (Commit Mono). If a frame cannot be run, write **not run** and why; never a
predicted result. Nothing is read from an agent's terminal (OD-8).

1. **Stack.** `bash control-plane/cmd/hub/devstack.sh up`; start the control plane on loopback;
   `npm --prefix desktop run tauri dev`. Sign in as Dylan.
   Record: `git rev-parse --short HEAD`, the Hub's `/healthz` line, the desktop's signed-in name.
2. **Import.** Work lens → *Import* → a CSV of twelve rows (`docs/verification/phase-w/w1-backlog.csv`
   from task 69's fixture) into project P.
   Record: the dialog's result line (created, existing, unmatched labels), verbatim.
3. **Triage.** Open team T's Triage.
   Record: EP-142's row text, which must read *not tied · imported*.
4. **Tie it; set its check.** Focus EP-142; press `T`; take the text-match suggestion for the resume
   criterion; set priority; leave Triage. Open EP-142; on that pair *Set check*, type `node --test test/resume.test.mjs`, `Enter`.
   Record: the suggestion's words, the *Undo* chip's text, EP-142's new `serves` line and its check line.
5. **Start.** `Shift+R` on EP-142. Start a stopwatch at the key press.
   Record: seconds to the first sign on the row (**measured**, this run only), the chip text
   (*Undo Start*), the lane's branch name, and that the assignee still reads Dylan.
6. **Alice comments.** As Alice, on EP-142: a comment mentioning Dylan, a reaction.
   Record: the comment as Dylan sees it, its author class, and Dylan's subscription cause.
7. **Land.** Land the lane through phase 3's Merge lens (`confirm.inplace`).
   Record: the land commit's `Panacea-Issue:` trailer (`git log -1 --format=%B <sha>`); that the
   row flashed `landed` in place and kept its position in list and board; `Space` peek's first line.
8. **Proven.** Wait for frame 4's check, run by the platform at the landed commit.
   Record: the evidence id, and that EP-142 read `done` with no click, and at what time.
9. **The Desk.** Open the Desk.
   Record: its whole text, which must end *"Nothing needs you. EP-142 landed and proven at 14:02."*
   (the time is the run's own). Then leave and re-enter; record that nothing moved while it was open.
10. **Hub facts.** `docker compose -f control-plane/cmd/hub/compose.yaml exec -T db psql -U postgres
    -d hub -At -f - < docs/verification/phase-w/hub-facts.sql`.
    Record: every line it prints.
11. **Budgets.** `node desktop/e2e/w1-budgets.e2e.mjs` against the dev server.
    Record: its four result lines, with the machine name it prints.
12. **The owner's look.** Show the owner the Desk at 125 % and 150 % with 30 fixture agents
    (`VITE_PANACEA_FIXTURE=desk-30`), and the Work lens, the issue view and the Desk in `graphite`,
    `paper` and Windows `forced-colors`.
    Record: the owner's words, verbatim, with the date; or *not yet shown*.
```

Frame 12's fixture switch is task 62's (`deskFixtures.ts`; **UNVERIFIED** name until merge). Frame
2's CSV is committed with this task, twelve fictional rows, one of them *"Resume reads from the
last checkpoint"* so that task 42's text match has a real candidate.

#### Interfaces — complete: `docs/verification/phase-w/check-slice.sh`

```bash
#!/usr/bin/env bash
# check-slice.sh: every parity row of one Phase W slice has, in that slice's results section, a
# task, a test or frame, a command, an observed excerpt and a commit (OD-27). Exit 0 only when
# all do.   Usage: check-slice.sh W1|W2|W3|W4|W5     check-slice.sh --local-writes
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
HERE="$ROOT/docs/verification/phase-w"
MATRIX="${W_MATRIX:-$ROOT/docs/superpowers/planning/2026-09-25-linear-parity/05-design.md}"
RESULTS="${W_RESULTS:-$ROOT/docs/verification/2026-09-25-phase-w-results.md}"
slice="${1:?usage: check-slice.sh W1..W5 | --local-writes}"

if [ "$slice" = "--local-writes" ]; then   # task 55's rule: no Hub record is kept on disk
  hits=$(cd "$ROOT" && git grep -nE "localStorage|indexedDB|writeTextFile" -- desktop/src/work \
    desktop/src/components/work desktop/src/components/issue desktop/src/components/desk \
    ':!*.test.ts' ':!*.test.tsx' | grep -vE '^[^:]+:[0-9]+:[[:space:]]*(//|/?\*)' || true)
  bad=$(printf '%s\n' "$hits" | awk -F: 'NR==FNR { if ($0 !~ /^#/ && NF) ok[$1]=1; next }
    NF && !($1 in ok)' FS=' ' "$HERE/prefs-allow.txt" FS=: - )
  [ -z "$bad" ] || { printf 'a local write outside prefs-allow.txt:\n%s\n' "$bad"; exit 1; }
  echo "local writes: $(printf '%s\n' "$hits" | grep -c . || true) hit(s), each a named preference"
  exit 0
fi

# The slice's rows: its 05 §1 slice cell names it, minus rows moved out, plus rows moved in.
from_matrix=$(awk -F'|' -v s="$slice" '/^\| [0-9]+ \|/ { n = split($(NF-1), p, "/")
  for (i = 1; i <= n; i++) { v = p[i]; gsub(/^ +| +$/, "", v); if (v == s) { gsub(/ /, "", $2); print "L-" $2 } } }' "$MATRIX")
[ -n "$from_matrix" ] || { echo "no $slice rows in 05 §1: the matrix moved"; exit 2; }
moved=$(grep -v '^#' "$HERE/moved-rows.txt" | grep . || true)
moved_out=$(printf '%s\n' "$moved" | awk -v s="$slice" '$2 == s { print $1 }')
moved_in=$(printf '%s\n' "$moved" | awk -v s="$slice" '$3 == s { print $1 }')
rows=$( { printf '%s\n' "$from_matrix" | grep -vxF -f <(printf '%s\n' "$moved_out") || true
          printf '%s\n' "$moved_in"; } | grep . | sort -u -t- -k2,2n)

section=$(awk -v h="## $slice: " 'index($0, h) == 1 { on = 1; print; next } on && /^## /{ exit } on' "$RESULTS")
[ -n "$section" ] || { echo "no $slice section in $RESULTS"; exit 2; }
table=$(printf '%s\n' "$section" | awk '/^### The rows/{ on = 1; next } on && /^### /{ exit } on')

fail=0
for r in $rows; do
  line=$(printf '%s\n' "$table" | grep -E "^\| $r( |\))" | head -1 || true)
  if [ -z "$line" ]; then echo "$r: no row in the $slice section"; fail=1; continue; fi
  empty=$(printf '%s\n' "$line" | awk -F'|' '{ c = 0; for (i = 2; i < NF; i++) { v = $i; gsub(/^ +| +$/, "", v); if (v == "" || v == "…") c++ }; print c }')
  commit=$(printf '%s\n' "$line" | awk -F'|' '{ v = $(NF-1); gsub(/^ +| +$/, "", v); gsub(/`/, "", v); print v }')
  [ "$empty" = 0 ] || { echo "$r: $empty empty cell(s)"; fail=1; }
  printf '%s' "$commit" | grep -Eq '^[0-9a-f]{7,40}$' || { echo "$r: commit cell is not a sha"; fail=1; }
done
for e in $(printf '%s\n' "$table" | grep -oE '^\| L-[0-9]+' | tr -d '| ' | sort -u); do
  printf '%s\n' "$rows" | grep -qx "$e" || { echo "$e: in the section but not a $slice row"; fail=1; }
done
if printf '%s\n' "$section" | grep -q '…'; then echo "an unfilled … is left in the $slice section"; fail=1; fi
[ "$fail" = 0 ] && echo "$slice coverage: $(printf '%s\n' "$rows" | wc -l | tr -d ' ') rows, each with a command, an observation and a commit"
exit "$fail"
```

The row logic is task 113's `check-w3.sh` (`task-113.md:132-163`) with the slice as an argument
and `moved-rows.txt` added, so the six sections share one check.

A comment line that names `localStorage` (task 32's `flags.ts` header is one, though outside these
directories) is not a write, so the grep drops lines whose code starts `//` or `*`.

#### Interfaces — complete: `moved-rows.txt` and `prefs-allow.txt`

```text
# moved-rows.txt: rows 05 moved between slices in its text, not in its §1 slice column.
# <row> <from> <to> <where 05 says so>
L-117 W2 W1 05 §5.3 row 4: subscriptions, from W2
L-45 W2 W1 05 §5.3 row 12: attachments, from W2
```

```text
# prefs-allow.txt: files under desktop/src/{work,components/work,components/issue,components/desk}
# allowed to write to local storage, each with the desktop preference it keeps. A Hub record is
# never one (task 55; ADR-H1: no Hub data on disk). W1 adds none; W4's task 136 adds its dial.
# <path> <preference>
```

#### Interfaces — complete: `docs/verification/phase-w/hub-facts.sql`

```sql
-- hub-facts.sql: frame 10. Every issue.state event of the run with its actor and cause, then two
-- counts that must be 0. Run as the database owner, which RLS does not filter; it only reads.
-- Names are the Hub contract's (hub-contract.md:1762-1776; task 13's IssueStateData): the
-- migrated table and the canonical bytes' JSON path are UNVERIFIED until step 1.
\set ON_ERROR_STOP on
\pset footer off

SELECT e.seq, to_char(e.at, 'HH24:MI:SS') AS at, e.subject,
       d.j #>> '{data,from}' AS from_state, d.j #>> '{data,to}' AS to_state,
       d.j #>> '{data,cause}' AS cause, coalesce(u.email, '(no actor)') AS actor
  FROM hub.audit_events e
  CROSS JOIN LATERAL (SELECT convert_from(e.canonical, 'UTF8')::jsonb AS j) d
  LEFT JOIN hub.users u ON u.id = e.actor_user_id
 WHERE e.kind = 'issue.state'
 ORDER BY e.org_id, e.seq;

-- Rule 2 and SW.3: a fact state is entered by a person, with its fact cause (task 07 decision 2).
SELECT 'fact moves without a person: ' || count(*)
  FROM hub.audit_events e
  CROSS JOIN LATERAL (SELECT convert_from(e.canonical, 'UTF8')::jsonb AS j) d
 WHERE e.kind = 'issue.state'
   AND d.j #>> '{data,to}' IN ('started', 'in_review', 'landed', 'done')
   AND e.actor_user_id IS NULL;

SELECT 'fact moves with a wrong cause: ' || count(*)
  FROM hub.audit_events e
  CROSS JOIN LATERAL (SELECT convert_from(e.canonical, 'UTF8')::jsonb AS j) d
 WHERE e.kind = 'issue.state'
   AND ((d.j #>> '{data,to}' = 'started'   AND d.j #>> '{data,cause}' <> 'lane_bound')
     OR (d.j #>> '{data,to}' = 'in_review' AND d.j #>> '{data,cause}' <> 'lane_queued')
     OR (d.j #>> '{data,to}' = 'landed'    AND d.j #>> '{data,cause}' <> 'land_trailer')
     OR (d.j #>> '{data,to}' = 'done'      AND d.j #>> '{data,cause}' NOT IN
           ('evidence_observed', 'evidence_recorded', 'land_trailer', 'hand')));
```

The expected run: EP-142's `done` row names Dylan and `evidence_observed`, both counts print `0`,
and no row names an actor other than Dylan or Alice. W1 has no sync principal (it arrives in W5,
tasks 145 and 150), so "no sync principal wrote a fact state" is shown here by every fact move
having a person as its actor; task 07's `work_actor_kind` guard is proved by its own test.

#### Interfaces — complete: `desktop/e2e/w1-budgets.e2e.mjs`

Phase 2 task 25's harness idiom: the `playwright` library, headed Chromium, `PANACEA_E2E_APP`,
`.mjs` outside `src/` so vitest and `tsc` never collect it. The clock starts at the keydown's own
`timeStamp` and stops at the first animation frame after the change is in the DOM, so a figure is
input to the frame that shows it, measured inside the page.

```js
// desktop/e2e/w1-budgets.e2e.mjs: 05 §4.6's four budgets, RECORDED and never asserted (02 C8).
// Prints one line per budget: median and worst of n, the machine and the browser. Exit 0 always
// once every act ran; a timeout is printed as "not reached" and is a finding, not a pass or fail.
import { hostname } from "node:os";
import { chromium } from "playwright";

const APP = process.env.PANACEA_E2E_APP ?? "http://localhost:1420";
const N = Number(process.env.PANACEA_E2E_RUNS ?? 20);
// Selectors and keys are the merged tree's (tasks 40, 53, 56, 59, 60, 61, 66): UNVERIFIED until step 1.
const SEL = { row: '[role="row"][data-issue]', palette: '[role="dialog"][data-palette]',
  peek: "[data-peek]", agentLine: "[data-agent-line]" };

async function measure(page, key, readyBody, arg, timeout) {
  await page.evaluate(({ readyBody, arg }) => {
    const ready = new Function("arg", readyBody);
    const b = (window.__budget = { t0: 0, t1: 0 });
    addEventListener("keydown", (e) => { b.t0 = e.timeStamp; }, { capture: true, once: true });
    const tick = () => (b.t0 && ready(arg)
      ? requestAnimationFrame(() => { b.t1 = performance.now(); })
      : requestAnimationFrame(tick));
    requestAnimationFrame(tick);
  }, { readyBody, arg });
  await page.keyboard.press(key);
  try {
    await page.waitForFunction(() => window.__budget.t1 > 0, null, { timeout });
  } catch { return null; }
  return page.evaluate(() => window.__budget.t1 - window.__budget.t0);
}

function line(name, xs, unit, browser) {
  const got = xs.filter((x) => x !== null).sort((a, b) => a - b);
  const scale = unit === "s" ? 1000 : 1;
  const f = (x) => (x / scale).toFixed(unit === "s" ? 2 : 1) + " " + unit;
  const tail = `(n=${xs.length}, not reached ${xs.length - got.length}) on ${hostname()}, Chromium ${browser}`;
  return got.length === 0 ? `${name} not reached ${tail}`
    : `${name} median ${f(got[Math.floor(got.length / 2)])} worst ${f(got[got.length - 1])} ${tail}`;
}

const browser = await chromium.launch({ headless: false });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
await page.goto(`${APP}/#/work`);
await page.waitForSelector(SEL.row);
const rows = await page.$$eval(SEL.row, (els) => els.map((e) => e.getAttribute("data-issue")));
const out = { paint: [], palette: [], peek: [], ack: [] };

for (let i = 0; i < N; i++) {
  const id = rows[i % rows.length];
  await page.focus(`${SEL.row}[data-issue="${id}"]`);
  const before = await page.textContent(`${SEL.row}[data-issue="${id}"]`);
  out.paint.push(await measure(page, "i", `return document.querySelector('${SEL.row}[data-issue="' + arg.id + '"]')?.textContent !== arg.before`, { id, before }, 2000));
  await page.keyboard.press("Control+z");
  out.palette.push(await measure(page, "Control+k", `return !!document.querySelector('${SEL.palette}')`, null, 2000));
  await page.keyboard.press("Escape");
  out.peek.push(await measure(page, " ", `return !!document.querySelector('${SEL.peek}')`, null, 2000));
  await page.keyboard.press("Escape");
}
for (const id of rows.slice(0, 3)) { // three Starts, not twenty: each one boots a local sandbox
  await page.focus(`${SEL.row}[data-issue="${id}"]`);
  out.ack.push(await measure(page, "Shift+R", `return !!document.querySelector('${SEL.row}[data-issue="' + arg + '"] ${SEL.agentLine}')`, id, 60_000));
  await page.getByRole("button", { name: /Undo Start|Stop and discard/ }).first().click();
}
const v = browser.version();
console.log(line("budget.paint", out.paint, "ms", v));
console.log(line("budget.palette", out.palette, "ms", v));
console.log(line("budget.peek", out.peek, "ms", v));
console.log(line("budget.ack", out.ack, "s", v));
await browser.close();
```

`budget.ack` runs three times, and each Start is undone at once; a `Stop and discard` after a file
write is taken with its confirm (task 61), which the run records in frame 11's observation.

#### Tests to write first — `desktop/src/test/w1Sweep.test.tsx`

Two jobs, as task 113's W3 sweep: every W1 surface's own test file calls `expectNoPrediction`
(task 20, SW.8), and the demo's last frame is the sentence task 65 builds, word for word, with
the 30-agent Desk's count sentence still a plain count.

```tsx
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { expectNoPrediction } from "./expectNoPrediction";
import { zeroSentence } from "../work/desk/informs";

const SRC = resolve(__dirname, "..");
/** Every W1 surface the demo shows, and its owning task (the skeleton's file map, :305-314). */
const W1_SURFACES: [string, number][] = [
  ["components/work/WorkLens.test.tsx", 40],
  ["components/work/IssueRow.test.tsx", 41],
  ["components/work/TriageView.test.tsx", 42],
  ["components/work/BoardView.test.tsx", 43],
  ["components/work/BulkBar.test.tsx", 44],
  ["components/work/PlanSurface.test.tsx", 45],
  ["components/issue/IssueView.test.tsx", 46],
  ["components/issue/Serves.test.tsx", 48],
  ["components/issue/Comments.test.tsx", 51],
  ["components/work/Peek.test.tsx", 60],
  ["components/work/StartChip.test.tsx", 61],
  ["components/desk/Desk.test.tsx", 62],
  ["components/desk/DeskZero.test.tsx", 65],
  ["components/work/ImportDialog.test.tsx", 69],
];
const at = () => "14:02";

describe("W1 sweep: no prediction on any W1 surface (OD-27)", () => {
  it.each(W1_SURFACES)("%s (task %i) exists and calls expectNoPrediction", (file) => {
    const path = resolve(SRC, file);
    expect(existsSync(path)).toBe(true);
    expect(readFileSync(path, "utf8")).toMatch(/expectNoPrediction\(/);
  });

  it("the demo's last frame is 05 §5.3's sentence", () => {
    const text = zeroSentence({ landed: { alias: "EP-142", word: "landed and proven", at: "2026-09-26T04:02:00Z" },
      live: { agents: 0, issues: 0 }, time: at });
    expect(text).toBe("Nothing needs you. EP-142 landed and proven at 14:02.");
  });

  it("the 30-agent Desk's line is a plain count", () => {
    const text = zeroSentence({ landed: null, live: { agents: 30, issues: 12 }, time: at });
    const { container } = render(<p>{text}</p>);
    expect(text).toBe("Nothing needs you. 30 agents live on 12 issues.");
    expectNoPrediction(container);
  });
});
```

The test file names are the skeleton's file map with `.test.tsx` beside each; step 1 corrects any
that the merged parts named differently. `LANDED`'s `word` (*"landed and proven"*) is task 64's
payload (**UNVERIFIED**: task 64 has no part on the tree at this writing, `task-65.md:7`).

| Test | Mutant it kills |
|---|---|
| each W1 surface's test calls the helper | a surface that skipped `expectNoPrediction` |
| the last frame's sentence | the demo's frame 9 drifting from 05 §5.3 (a reworded or reordered sentence) |
| the 30-agent line | *"30 of 42"* or a percentage replacing the count |
| `check-slice.sh W1` on `w1-missing-output.md` exits 1 | a row marked closed from a self-report |
| `check-slice.sh W1` on `w1-complete.md` exits 0 | a check that fails everything, so nobody runs it |
| `check-slice.sh W2` counts L-45 and L-117 out | a moved row counted in both slices |
| `check-slice.sh --local-writes` with a planted `localStorage.setItem` in `src/work/` | task 55's rule unchecked |
| `hub-facts.sql` prints `fact moves without a person: 0` | a `done` written with `actor: null` |

The fixtures: `testdata/w1-complete.md` is a `## W1: ` section whose rows table fills all fifty-one
rows with the sample sha `0123abc`; `testdata/w1-missing-output.md` is the same with L-41's
*Observed* cell empty.

#### Steps

- [ ] **Step 1: read on the merged W1 tree.** Take the real names this part marks UNVERIFIED:
  `devstack.sh` and `compose.yaml` (Hub task 09), `hub.audit_events` and `hub.users`, the
  selectors and keys of `w1-budgets.e2e.mjs`, the fixture switch of frame 12, and each closing
  test's real name from its part's mutant table into the rows table. Re-run the matrix selection;
  if 05 §1 has moved a row, correct the count here and say why.
- [ ] **Step 2: the tests first.** Write `w1Sweep.test.tsx`, `check-slice.sh` and its fixtures,
  `moved-rows.txt` and `prefs-allow.txt`. The sweep fails until every W1 surface exists; the check
  on `w1-missing-output.md` exits 1.
- [ ] **Step 3: create the results file** with its head and an empty W1 section in the shape above.
- [ ] **Step 4: bring the stack up** (frame 1) and record the three commits in the lead line.
- [ ] **Step 5: run frames 2-10** of `w1-demo.md`, filling the run table with quoted
  observations. Shorten with `[cut]`, never `…`.
- [ ] **Step 6: run the budgets** (frame 11) and paste the four lines into the budgets table.
- [ ] **Step 7: run the suites** and each of tasks 01-69's and 23b's acceptance commands that the rows table
  cites; paste each result line beside its row.
- [ ] **Step 8: show the owner** (frame 12) and record the words, or *not yet shown*. List what
  is still unproven.
- [ ] **Step 9: run the acceptance commands.** Commit the results file, `docs/verification/phase-w/`
  and the two desktop files by path, the message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
bash docs/verification/phase-w/check-slice.sh W1
# expect: "W1 coverage: 51 rows, each with a command, an observation and a commit", exit 0
W_RESULTS=docs/verification/phase-w/testdata/w1-missing-output.md bash docs/verification/phase-w/check-slice.sh W1; echo "exit $?"
# expect: "L-41: 1 empty cell(s)" then "exit 1"
W_RESULTS=docs/verification/phase-w/testdata/w1-complete.md bash docs/verification/phase-w/check-slice.sh W1
# expect: "W1 coverage: 51 rows, …", exit 0
bash docs/verification/phase-w/check-slice.sh --local-writes
# expect: "local writes: 0 hit(s), each a named preference", exit 0
npm --prefix desktop test -- test/w1Sweep
# expect: 16 tests pass (14 surfaces, the sentence, the count)
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line
npm --prefix desktop test && npm --prefix desktop run typecheck
# expect: every suite passes; exit 0
grep -cE "^## W1: " docs/verification/2026-09-25-phase-w-results.md
# expect: 1
```

The `-tags integration` rows run against the local stack: `go test -tags integration
./control-plane/internal/hub/... -count=1` prints `ok` for each package, and the line goes in the
checks table. A timing line reading `not reached` is recorded as it is; it fails nothing here.

#### What this task does NOT claim

- **The cloud demo.** It is the owner's, after phase 6 stream A and phase 8 stream C (the
  skeleton's *Start lines*); a local run proves nothing about managed sessions or Kata sandboxes.
- **Team mode**, Orchestrators or any W3 verb.
- **Any number as a measurement beyond the run** it was taken in: the budgets are one machine's,
  in Chromium against the dev server, not WebView2 in the packaged app; the first-sign seconds
  of frame 5 are one Start.
- **Screen-reader use.** Task 66's roles and live region are tested in jsdom; nobody has used W1
  with Narrator or NVDA.
- **`first.landing`'s starter repo** (task 69's rehearsal), unless frame 2 is run from it.
- **L-175's cloud half** (*"Start opens it in a managed cloud session"*) and the `-hc` themes.
- The owner's acceptance, until it is recorded in the owner's words.

**Parity.** This task closes no row itself; it records the fifty-one rows of decision 3 as closed
**only** where the rows table carries a command, an observation and a commit.

#### Gaps found while writing this part

1. **L-175 is in 05 §1 as W1** but not in the scope's row list, and the coverage map gives it no
   *proof* task. This part adds it as its local-mode half.
2. **"Each of the four themes"** cannot be shown in W1: `graphite-hc` and `paper-hc` are task 96
   (W2). Frame 12 shows `graphite`, the provisional `paper` and `forced-colors`.
3. **L-146 is claimed twice**: 05 §1 and the scope put it in W1 (tasks 21-22's trailers at land);
   the skeleton's task 71 says it *closes* L-146. This part records the W1 half; task 71's part
   should say it closes the release half.
4. **`docs/verification/phase-w/`** (the check, fixtures, SQL, CSV) and `desktop/e2e/` are not in
   the skeleton's *Proof and documents* table (`:317-325`).
5. **Task 113's `check-w3.sh`** duplicates `check-slice.sh`; the skeleton should name one check.
   Phase 2 task 25's `desktop/e2e/` harness is not built on this branch, so the budgets script's
   dependency on `playwright` lands with it or with this task, whichever merges first.

#### Addendum (fix batch b02, round 2): the check frame 4 sets, and what it needs

Task 23b's check line is W1's only writer of `Serve.Check` (`task-23b.md`, decision 5), and task
23's Closer runs a check only for a pair whose `Serve.Check` is set (`task-23.md:454`). Before this
round no frame set one, so EP-142 stayed `landed` and frames 8 and 9 could not happen. Frame 4 now
sets it on the head revision, after `T` ties the pair, and records the check line's text.

- **The check is `node --test test/resume.test.mjs`.** Node 20's built-in test runner is in the
  sandbox image (`images/sandbox/Dockerfile:10`, `node:20-bookworm-slim`), so the check installs
  nothing and needs no network, no credential and no CA bundle: it passes or fails on the landed
  commit alone. Project P's repository must hold `test/resume.test.mjs`, failing before EP-142's
  lane and passing after it. That repository is the person's: no task builds a demo repository
  (task 69's gap 2 is the same absence for `first.landing`). **UNVERIFIED** until the run.
- **If the lane does not make it pass**, the check exits non-zero, nothing is recorded, and frame
  8 records *not reached* and that EP-142 still reads `landed`: an honest result, never a predicted one.
- **Set before Start** (frame 5), so the agent reads the revision that carries the check and no
  revision is written under a running lane.
- **Typed, not pasted.** It is the first typed check the plan runs, so it also proves that task
  47's list `T` leaves a key typed in the field to the field (`task-47.md:414`).
- **`dependsOn` gains 23b** (the line and the runner): `:4-5` here, the skeleton's row 70, and the
  scope's block (`tasks-61-80.md:354`). Size unchanged: one act inside an existing frame.
