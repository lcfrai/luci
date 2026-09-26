### Task 187: WL's proof and results

Stream **WL** (the *later* rows), about 2.5 hours of work beside the run itself (an estimate, not
a measurement). Depends on tasks **175-186**. Writes the WL section of
`docs/verification/2026-09-25-phase-w-results.md` ⚑, the **last** section, owned jointly with
tasks 70, 97, 113, 142 and 174 and appended in task order. It keeps task 174's shape (a lead line
naming the environment, the rows, the suites, the owner's look, what is still unproven) and adds
one thing no earlier section has: **the phase ledger**, all 257 rows of 05 §1.

`OD-27` is the rule: **proof per criterion, no prediction.** Every row this section marks closed
carries the command that was run, an excerpt of what it printed, and the commit it ran at. A
criterion that was not run says *not run* and why, never *expected to pass*.

**Decisions made here, so nobody re-derives them:**

1. **The environment is named, first line.** The local Hub stack through `devissuer`, with the
   fakes and golden payloads WL's parts define: Graph (task 185), Slack (182), Azure Monitor and
   Sentry (183), PagerDuty and Opsgenie (184), and the warehouse sink (178). The lead line names
   each fake, the Hub commit and the control plane and desktop commit, and says, row by row, that
   no live vendor was called; if one was, the row names its tenant.
2. **The WL rows are re-derived, not copied.** `check-wl.sh` selects every row of 05 §1 whose
   slice cell is exactly `later`. On 2026-09-26 that is **twelve**
   (`grep -c "| later |" …/05-design.md` prints `12`, run while writing this part), the same
   twelve the scope lists. A different count or row means a row moved; the section says which
   before closing anything, and the expected line in *Acceptance* moves with it.
3. **Four clause rows are carried, not closed.** WL lifts a clause from rows other slices close:
   the *later* part of **L-50** (184, with 121 and 134; L-50 is PARTIAL, `05-design.md:164`) and
   the *Slack next* clause of **L-80**, **L-97** and **L-137** (182; W5 closes the rows with Teams,
   `:204`, `:231`, `:289`). Each gets a row in *The clauses*, with its own test, and the check
   requires them by a fixed list, since no slice cell selects them.
4. **L-152 records the Q1 answer.** L-152 is one of the seven **(Q1)** rows (`05-design.md:499`):
   it follows the recommended answer (measured counts, no forecast). Its row quotes which answer
   the owner gave, with the date, or *Q1 not answered; the recommended answer was built*.
5. **The ledger is the phase's last word.** Every one of the 257 rows gets one status, from a
   closed set: `closed` (a task, a test, a command, an excerpt, a commit), `skip` (one of the 27,
   with 05's reason), `other` (owned by team mode or phases 3, 5, 6 or 8; the owner named), `exists`
   (built before Phase W; *not re-checked* unless it was), `partial` (the closed part proved, the
   deferred part named), or `gap`. **A row with no closing task and no reason is `gap`, listed, never
   hidden.** The ledger's own count of complete rows is the number filed, not 05's forecast of 188
   (`05-design.md:480`).
6. **The ledger's source is the coverage map**, `…-scopes/coverage.md`, whose table names each
   row's closing tasks from the scope paragraphs mechanically (`coverage.md:13-24`). The ledger
   adds what the map cannot: the test that proved it, the command and the commit. The map's 27
   gaps (21 with no task, 6 served and never closed, `coverage.md:311-349`) start as `other` or
   `exists` rows by their 05 stop, and become `gap` if the stop is not proved either.
7. **The owner's look is its own row**: dashboards (177), guided review (180), *What's new* (186),
   and the WL settings surfaces (175, 176, 184, 185). `CLAUDE.md`: *"Visual acceptance is the
   owner's eyes"*; the row holds the owner's words or *not yet shown*.
8. **What stays unverified is listed by name** (the scope's list): every vendor payload shape,
   Slack and alert sources reaching the Hub through phase 8's edge allowlist, the Graph permission
   for 185, host display of 181's signatures and a host rule that wants every commit signed,
   signing in the cloud, the differ's licence and its behaviour on real repositories (179), and
   the owner's look (decision 7). Step 6 adds whatever parts 175-186 name.

**Files:**
- `docs/verification/2026-09-25-phase-w-results.md` ⚑ — the WL section and the ledger (task 70's
  file)
- `docs/verification/phase-w/check-wl.sh` — the WL coverage check (new)
- `docs/verification/phase-w/check-ledger.sh` — the 257-row ledger check (new)
- `docs/verification/phase-w/testdata/{wl-complete,wl-missing-output,ledger-complete,ledger-hidden-row,ledger-wrong-verdict}.md`
  — the checks' fixtures (new)
- `desktop/src/test/wlSweep.test.tsx` — every WL surface's test calls `expectNoPrediction` (new)

#### The rows WL closes, and what closes each

Line numbers are `05-design.md`'s on 2026-09-26 (each re-read while writing this part). The test
is a real name from the closing part's mutant table (parts 175-186 are all written on
2026-09-26); step 1 re-reads each on the sync base in case a part moved it.

| Row | 05 §1 | Closed by | The test that must pass |
|---|---|---|---|
| L-7 | `:111` | 175 | `TestACopiedTemplateDropsWhatTheTargetLacksAndCreatesNothing` (`task-175.md:498`) |
| L-40 | `:149` | 176 | `TestTheRuleAssignsOnTheOwnersOwnCreate` (`task-176.md:292`) |
| L-152 (Q1) | `:309` | 177 (with 122) | `TestOnlyTheFourTypesExist` (`task-177.md:288`): no burn-up, percentage or forecast block |
| L-154 | `:311` | 178 (with 156's webhooks) | `TestFollowResumesWithNoDuplicateAndNoGap` (`task-178.md:315`) |
| L-171 | `:333` | 179 | `renamed identifier marks one token` (`task-179.md:303`) |
| L-172 | `:334` | 180 | `every hunk exactly once` (`task-180.md`, the `guidedReview.test.ts` rows) |
| L-176 | `:338` | 181 | `TestSignedLandVerifies` (`task-181.md:204`) |
| L-206 | `:378` | 182 | `TestShortcutMakesOneRequest` (`task-182.md`, after `:291`) |
| L-209 | `:381` | 183 | `TestFiredMakesOneTriageRequest` (`task-183.md:253`) |
| L-218 | `:390` | 184 | `TestAMatchedOnCallIsOnDuty` (`task-184.md:371`) |
| L-238 | `:420` | 185 | `TestLeavingRemovesOnlyGroupSourced` (`task-185.md:347`) |
| L-254 | `:446` | 186 | `refuses an entry dated after the build` (`task-186.md`, part 1) |

#### The clauses WL carries from rows other slices close

| Row | 05 §1 | Its closer | WL's clause, and its test |
|---|---|---|---|
| L-50 (PARTIAL) | `:164` | 114, 121, 134 (W4) | the *later* part, a rotation synced from PagerDuty or Opsgenie: 184's `TestAnOutageKeepsDuty` and `TestANonMemberAddsNobody` |
| L-80 | `:204` | 157, 165 (W5, Teams) | *Slack next*: 182's `TestNoCustomerTextLeaves` |
| L-97 | `:231` | 157 (W5, Teams) | *Slack next*: 182's `TestUnlinkedSubscribeRefused` |
| L-137 | `:289` | 157 (W5, Teams) | *Slack next*: 182's `TestThreadReplyAttributed` |

A clause row is not a close: the row's verdict stays with its own slice's section (task 142 for
L-50, task 174 for the other three), and this table records only that the clause was run. L-50
keeps 05's verdict, **PARTIAL** (`05-design.md:164`, `:464`: *"syncing it from PagerDuty and the
like is L-218"*); with 184's rows passing, its ledger status is `closed`, both parts proved, and
whether 05 should then read MATCH is left to the owner (gap 3). The inventory's item also names
Rootly and incident.io (`coverage.md:93`), which 184 does not read, and the ledger says so.

#### Interfaces — complete: the WL section's shape (appended to the results file)

```markdown
## WL: the later rows — <date>

**Environment:** the local Hub stack through `devissuer`; fakes and golden payloads standing in:
<Graph, Slack, Azure Monitor, Sentry, PagerDuty, Opsgenie, the warehouse sink: each named, with the
one it stands for>. Hub at `<sha>`, control plane and desktop at `<sha>`. No live vendor was called
<or: row L-nnn called <vendor> in tenant <name>>.

### The rows

| Row | Task | Test (real name) | Command | Observed (excerpt) | Commit |
|---|---|---|---|---|---|
| L-7 | 175 | … | … | … | … |

### The clauses

| Row | Task | Clause | Test | Observed (excerpt) | Commit |
|---|---|---|---|---|---|
| L-50 | 184 | later: rotation sync | … | … | … |

### Q1

<the owner's answer to Q1, verbatim, with the date; or "Q1 not answered; the recommended answer
was built">

### The suites

| Suite | Command | Result line (quoted) |
|---|---|---|
| Go | `go test ./control-plane/... -race -count=1` | … |
| Go, integration | `go test ./control-plane/... -race -count=1 -tags integration` | … |
| Desktop | `npm --prefix desktop test` | … |
| Types | `npm --prefix desktop run typecheck` | … |
| WL acceptance | each of tasks 175-186's *Acceptance* block, one row per task | … |

### The owner's look

<the owner's words, verbatim, with the date, per surface of decision 7; or "not yet shown">

### What is still unproven

- <each item of task 187's decision 8, one line each, with the test that would settle it>

## The phase ledger — <date>

<the ledger, shaped as below>
```

Every `…` is filled from the run. A cell left `…` fails `check-wl.sh`.

#### Interfaces — complete: the phase ledger's shape

```markdown
## The phase ledger — <date>

**Sources:** 05 §1 (revision 2) at `<sha>`; the coverage map at `<sha>`.
**Counts, as filed:** closed <n> · partial <n> · exists <n> · other <n> · skip <n> · gap <n> · total 257.

| L | Verdict | 05 stop | Status | Closed by | Test | Command | Observed (excerpt) | Commit | Note |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | MATCH | exists | exists | — | — | — | — | — | not re-checked |
| 4 | SKIP | — | skip | — | — | — | — | — | Hub v1 has no per-project grants [cut] |
| 7 | MATCH | later | closed | 175 | `TestACopiedTemplate…` | `go test ./control-plane/hub/…` | `ok … 0.41s` | `3f9c2a1` | |
| 50 | PARTIAL | W4 | closed | 121, 134, 184 | … | … | … | … | later part by 184; Rootly, incident.io not read |

### Gaps

- L-<n>: <05 stop>; <why no closing task, and which stop was not proved>
```

The example rows show the shape only; the filed ledger holds the run's values. The column rules,
which `check-ledger.sh` enforces:

| Status | Needs | Never |
|---|---|---|
| `closed` | *Closed by* names a task 01-186 (never 70, 97, 113, 142, 174 or 187); *Test*, *Command*, *Observed* filled; *Commit* a sha | a `—` in any of those five cells |
| `partial` | as `closed`, and *Note* names the deferred part | an empty *Note* |
| `skip` | 05's verdict is SKIP; *Note* holds 05's reason, shortened with `[cut]` | on a row 05 does not mark SKIP |
| `other` | *Note* names the owner: `TM`, `phase 3`, `phase 5`, `phase 6` or `phase 8` | on a row whose 05 stop is `W1`-`W5` or `later` |
| `exists` | *Note* reads `not re-checked`, or *Commit* is the sha it was re-checked at | on a row whose 05 stop is not `exists` (or `exists / later`) |
| `gap` | the row's id also appears in *Gaps* with a reason | a gap missing from *Gaps* |

The *Verdict* and *05 stop* cells are copied from 05 §1 by the check's own read, so a ledger row
whose verdict disagrees with 05 fails (mutant: a SKIP quietly re-filed as MATCH). The *Counts*
line is recomputed by the check from the status column; a line that disagrees fails (mutant: a
count typed from 05's forecast of 188 instead of counted).

#### Interfaces — complete: `docs/verification/phase-w/check-wl.sh`

```bash
#!/usr/bin/env bash
# check-wl.sh: every row of 05 §1 whose stop is exactly "later", and the four clause rows WL
# carries, has in the WL section a task, a test, a command, an excerpt and a commit (OD-27).
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
MATRIX="${W_MATRIX:-$ROOT/docs/superpowers/planning/2026-09-25-linear-parity/05-design.md}"
RESULTS="${WL_RESULTS:-$ROOT/docs/verification/2026-09-25-phase-w-results.md}"
CLAUSES="L-50 L-80 L-97 L-137"

# §1 only: from "## 1." to the next "## ", so §5's delta tables ("| 1 | …") are never read.
rows=$(awk -F'|' '/^## 1\. /{on=1; next} on && /^## /{exit}
  on && /^\| [0-9]+ \|/ { s=$(NF-1); gsub(/^ +| +$/, "", s); if (s=="later") { gsub(/ /, "", $2); print "L-" $2 } }' "$MATRIX")
[ -n "$rows" ] || { echo "no later rows in 05 §1: the matrix moved"; exit 2; }

section=$(awk '/^## WL: /{on=1; print; next} on && /^## /{exit} on{print}' "$RESULTS")
[ -n "$section" ] || { echo "no WL section in $RESULTS"; exit 2; }
sub() { printf '%s\n' "$section" | awk -v h="### $1" '$0==h{on=1; next} on && /^### /{exit} on'; }

check_line() { # $1 id, $2 table name, $3 table text
  local line empty commit
  line=$(printf '%s\n' "$3" | grep -E "^\| $1( |,|\))" | head -1 || true)
  if [ -z "$line" ]; then echo "$1: no row in $2"; fail=1; return; fi
  empty=$(printf '%s\n' "$line" | awk -F'|' '{c=0; for (i=2; i<NF; i++) { v=$i; gsub(/^ +| +$/, "", v); if (v=="" || v=="…") c++ }; print c}')
  commit=$(printf '%s\n' "$line" | awk -F'|' '{v=$(NF-1); gsub(/^ +| +$/, "", v); gsub(/`/, "", v); print v}')
  [ "$empty" = 0 ] || { echo "$1: $empty empty cell(s)"; fail=1; }
  printf '%s' "$commit" | grep -Eq '^[0-9a-f]{7,40}$' || { echo "$1: commit cell is not a sha"; fail=1; }
}

fail=0
table=$(sub "The rows"); clauses=$(sub "The clauses")
for r in $rows; do check_line "$r" "The rows" "$table"; done
for c in $CLAUSES; do check_line "$c" "The clauses" "$clauses"; done
for e in $(printf '%s\n' "$table" | grep -oE '^\| L-[0-9]+' | tr -d '| ' | sort -u); do
  printf '%s\n' "$rows" | grep -qx "$e" || { echo "$e: in The rows but its 05 stop is not later"; fail=1; }
done
q1=$(sub "Q1" | grep -v '^ *$' || true)
printf '%s\n' "$q1" | grep -Eq '^<|^$' && { echo "Q1: the owner's answer or the not-answered line is missing"; fail=1; }
if printf '%s\n' "$section" | grep -q '…'; then echo "an unfilled … is left in the WL section"; fail=1; fi
[ "$fail" = 0 ] && echo "WL coverage: $(printf '%s\n' "$rows" | wc -l | tr -d ' ') rows and 4 clauses, each with a command, an observation and a commit"
exit "$fail"
```

It is task 174's `check-w5.sh` with three changes: the stop cell is matched **exactly**
(`later`; never the four mixed stops, `exists / later` on L-120 and L-231, `W1 / later` on L-127
and `W3 / later` on L-166, `05-design.md:264`, `:408`, `:271`, `:328`, whose later halves no WL
task builds: gap 1), the read
is fenced to §1, and the clause rows are required by the fixed list of decision 3. If task 70 has
by then grown the shared `check-slice.sh` (task 174's gap 3), this file becomes a call to it plus
the clause loop. `WL_RESULTS` points it at `testdata/wl-complete.md` (twelve rows and four
clauses, each with a sample sha, and a Q1 line) and `testdata/wl-missing-output.md` (the same with
L-218's *Observed* cell empty).
