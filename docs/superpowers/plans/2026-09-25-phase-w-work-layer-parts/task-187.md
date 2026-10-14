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
| L-80 | `:204` | 157, 165 (W5, Teams) | *Slack next*: 182's `TestNoCustomerTextLeaves`, which must insert its Slack binding itself: no route writes one yet (182's *Feed bindings*, a gap returned), so the clause is run and proven safe, and no user can reach it |
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

#### Interfaces — complete: `docs/verification/phase-w/check-ledger.sh`

```bash
#!/usr/bin/env bash
# check-ledger.sh: the phase ledger holds each of 05 §1's 257 rows exactly once, each with one
# status from the closed set and the evidence that status needs (OD-27; task 187 decision 5).
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
MATRIX="${W_MATRIX:-$ROOT/docs/superpowers/planning/2026-09-25-linear-parity/05-design.md}"
RESULTS="${W_RESULTS:-$ROOT/docs/verification/2026-09-25-phase-w-results.md}"
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
US=$'\037'  # a non-whitespace separator, so an empty cell never collapses into its neighbour

cells() { awk -F'|' -v OFS="$US" '/^\| [0-9]+ \|/ { o=""; for (i=2; i<NF; i++) { v=$i; gsub(/^ +| +$/, "", v); o = o (i>2 ? OFS : "") v }; print o }'; }
awk '/^## 1\. /{on=1; next} on && /^## /{exit} on' "$MATRIX" | cells | awk -F"$US" -v OFS="$US" '{print $1, $3, $NF}' > "$TMP/m"
[ "$(wc -l < "$TMP/m" | tr -d ' ')" = 257 ] || { echo "05 §1 holds $(wc -l < "$TMP/m" | tr -d ' ') rows, not 257: the matrix moved"; exit 2; }

ledger=$(awk '/^## The phase ledger/{on=1; print; next} on && /^## /{exit} on{print}' "$RESULTS")
[ -n "$ledger" ] || { echo "no phase ledger in $RESULTS"; exit 2; }
printf '%s\n' "$ledger" | cells > "$TMP/l"
gaps=$(printf '%s\n' "$ledger" | awk '/^### Gaps/{on=1; next} on && /^#/{exit} on' | grep -oE '^- L-[0-9]+' | grep -oE '[0-9]+' || true)

fail=0
while IFS="$US" read -r n verdict stop; do
  hits=$(awk -F"$US" -v n="$n" '$1==n' "$TMP/l" | wc -l | tr -d ' ')
  if [ "$hits" = 0 ]; then echo "L-$n: not in the ledger"; fail=1; continue; fi
  [ "$hits" = 1 ] || { echo "L-$n: $hits rows in the ledger"; fail=1; }
  IFS="$US" read -r _ lv ls st by test cmd obs commit note < <(awk -F"$US" -v n="$n" '$1==n' "$TMP/l" | head -1)
  [ "$lv" = "$verdict" ] || { echo "L-$n: verdict $lv, 05 says $verdict"; fail=1; }
  [ "$ls" = "$stop" ] || { echo "L-$n: stop $ls, 05 says $stop"; fail=1; }
  [ "$verdict" = SKIP ] && [ "$st" != skip ] && { echo "L-$n: a SKIP row filed as $st"; fail=1; }
  case "$st" in
    closed|partial)
      for c in "$by" "$test" "$cmd" "$obs"; do case "$c" in ""|"—") echo "L-$n: $st with an empty evidence cell"; fail=1; break;; esac; done
      printf '%s' "$commit" | tr -d '`' | grep -Eq '^[0-9a-f]{7,40}$' || { echo "L-$n: commit cell is not a sha"; fail=1; }
      printf '%s' "$by" | grep -Eqw '70|97|113|142|174|187' && { echo "L-$n: a proof task is named as its closer"; fail=1; }
      if [ "$st" = partial ]; then case "$note" in ""|"—") echo "L-$n: partial names no deferred part"; fail=1;; esac; fi ;;
    skip) [ "$verdict" = SKIP ] || { echo "L-$n: skip on a $verdict row"; fail=1; } ;;
    other)
      case "$stop" in W[1-5]*|later) echo "L-$n: other on a Phase W row ($stop)"; fail=1;; esac
      printf '%s' "$note" | grep -Eq 'TM|phase [3568]' || { echo "L-$n: other names no owner"; fail=1; } ;;
    exists)
      case "$stop" in exists*) ;; *) echo "L-$n: exists on a $stop row"; fail=1;; esac
      [ "$note" = "not re-checked" ] || printf '%s' "$commit" | tr -d '`' | grep -Eq '^[0-9a-f]{7,40}$' \
        || { echo "L-$n: exists with neither 'not re-checked' nor a commit"; fail=1; } ;;
    gap) printf '%s\n' "$gaps" | grep -qx "$n" || { echo "L-$n: a gap missing from Gaps"; fail=1; } ;;
    *) echo "L-$n: status '$st' is not in the closed set"; fail=1 ;;
  esac
done < "$TMP/m"

[ "$(wc -l < "$TMP/l" | tr -d ' ')" = 257 ] || { echo "the ledger holds $(wc -l < "$TMP/l" | tr -d ' ') rows, not 257"; fail=1; }
line=$(printf '%s\n' "$ledger" | grep -m1 '^\*\*Counts, as filed:\*\*' || true)
summary=""
for s in closed partial exists other skip gap; do
  want=$(awk -F"$US" -v s="$s" '$4==s' "$TMP/l" | wc -l | tr -d ' ')
  got=$(printf '%s' "$line" | grep -oE "$s [0-9]+" | grep -oE '[0-9]+' || true)
  [ "$got" = "$want" ] || { echo "Counts: $s reads ${got:-nothing}, the rows hold $want"; fail=1; }
  summary="$summary $s $want ·"
done
if printf '%s\n' "$ledger" | grep -q '…'; then echo "an unfilled … is left in the ledger"; fail=1; fi
[ "$fail" = 0 ] && echo "Ledger: 257 rows,${summary% ·}"
exit "$fail"
```

The status column is the ledger's fourth cell (`$4` after the leading `|` is dropped), matching the
shape above. Its three fixtures: `ledger-complete.md` (257 rows, every status used at least once,
the counts line right), `ledger-hidden-row.md` (the same with L-209's row deleted) and
`ledger-wrong-verdict.md` (the same with L-62, a SKIP, filed `closed` under verdict MATCH).

#### Tests to write first — `desktop/src/test/wlSweep.test.tsx`

Task 174's sweep, for WL's surfaces. Each path below was read from the owning part's *Files* or
test heading on 2026-09-26; the three `Dashboard*` names follow task 177's *"(+ tests)"*
(`task-177.md:71-72`, `:302`) and are **UNVERIFIED** as file names until 177 is built. Tasks
178, 182 and 183 add no desktop surface; 179's `syntax.test.tsx` is the Files lens surface.

```tsx
import { readFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(__dirname, "..");
/** Every WL surface and its owning task. ⚑ marks a file an earlier task owns and WL extends. */
const WL_SURFACES: [string, number][] = [
  ["components/plan/CopyFromTeam.test.tsx", 175],
  ["components/settings/AssignMe.test.tsx", 176],
  ["components/roadmap/Dashboards.test.tsx", 177],
  ["components/roadmap/DashboardView.test.tsx", 177],
  ["components/roadmap/DashboardBlock.test.tsx", 177],
  ["components/lens/files/syntax.test.tsx", 179],
  ["components/merge/GuidedReview.test.tsx", 180],
  ["components/work/PlanSurface.test.tsx", 180], // ⚑ task 45's
  ["components/admin/SignedLand.test.tsx", 181],
  ["components/plan/TriageRotationSettings.test.tsx", 184], // ⚑ task 134's
  ["components/admin/GroupLinks.test.tsx", 185],
  ["components/changelog/WhatsNew.test.tsx", 186],
];

describe("WL sweep: no prediction on any WL surface (OD-27)", () => {
  it.each(WL_SURFACES)("%s (task %i) exists and calls expectNoPrediction", (file) => {
    const path = resolve(SRC, file);
    expect(existsSync(path)).toBe(true);
    expect(readFileSync(path, "utf8")).toMatch(/expectNoPrediction\(/);
  });

  it("names every WL surface once", () => {
    const files = WL_SURFACES.map(([f]) => f);
    expect(new Set(files).size).toBe(files.length);
  });

  it("dashboards say counts, never a forecast (L-152, Q1)", () => {
    const src = ["Dashboards", "DashboardView", "DashboardBlock"]
      .map((n) => readFileSync(resolve(SRC, `components/roadmap/${n}.tsx`), "utf8"))
      .join("\n");
    // Word-bounded, so "details" or "metadata" never trips the "eta" guard.
    expect(src).not.toMatch(/\b(forecast|projected|burn-?up|on track|eta)\b|<meter/i);
  });
});
```

The dashboard test is the one surface-specific row: L-152 is the only WL row with a **(Q1)** mark,
and the recommended answer is *measured counts only* (decision 4), so a forecast word in any of
177's three sources fails the sweep even if 177's own test was loosened.

| Test | Mutant it kills |
|---|---|
| each WL surface's test calls the helper | a WL surface that skipped `expectNoPrediction` |
| every surface named once | a pasted duplicate hiding a missing surface |
| dashboards say counts | a burn-up or *on track* line added to a dashboard block (Q1 broken) |
| `check-wl.sh` on `wl-complete.md` exits 0 | a check that fails everything, so nobody runs it |
| `check-wl.sh` on `wl-missing-output.md` exits 1 | a later row marked closed from a self-report |
| `check-wl.sh` with a clause row deleted | a clause (L-50, L-80, L-97, L-137) dropped because no slice cell selects it |
| `check-ledger.sh` on `ledger-complete.md` exits 0 | a ledger check nobody can pass |
| `check-ledger.sh` on `ledger-hidden-row.md` exits 1 | a row with no closer quietly left out (decision 5) |
| `check-ledger.sh` on `ledger-wrong-verdict.md` exits 1 | a SKIP re-filed as a close |
| the ledger's counts line recomputed | 05's forecast of 188 typed in instead of counted |

#### Steps

- [ ] **Step 1: read on the sync base** parts 175-186 and their code. Re-derive the twelve rows
  (decision 2) and the four clauses; fill each *test* cell above with the name the built test
  carries, and correct any line number that moved. Read tasks 70's and 174's sections and keep
  WL's in the same shape.
- [ ] **Step 2: the tests first.** `wlSweep.test.tsx`, then `check-wl.sh` and `check-ledger.sh`
  with their five fixtures. `npm --prefix desktop test -- test/wlSweep` fails until every WL
  surface exists; each check exits 1 on its failing fixture.
- [ ] **Step 3: name the environment** (decision 1): bring up the local Hub stack through
  `devissuer` and the fakes 178 and 182-185 define, and record the Hub, control plane and desktop
  commits in the lead line.
- [ ] **Step 4: run each row's and each clause's test** with the command its part gives, and fill
  *The rows* and *The clauses*, quoting the result line. Shorten with `[cut]`, never with `…`.
- [ ] **Step 5: record Q1** (decision 4) from `docs/superpowers/planning/2026-09-23-owner-decisions.md`
  or the owner's words; if Q1 is unanswered, write the not-answered line.
- [ ] **Step 6: run the suites**, with and without `-tags integration`, and each of tasks
  175-186's *Acceptance* blocks; paste each result line beside its row.
- [ ] **Step 7: file the ledger.** Start from `coverage.md`'s table (decision 6): one ledger row
  per map row, *Verdict* and *05 stop* copied from 05 §1. For each row pick the status by the
  column rules, pulling the test, command, excerpt and commit from sections 70, 97, 113, 142, 174
  and this one. A row whose closing section is missing its evidence is `gap`, never `closed`.
  Write *Gaps*, then the counts line, then run `check-ledger.sh` until it exits 0.
- [ ] **Step 8: list what is still unproven** (decision 8, plus any a part 175-186 names), and
  show decision 7's surfaces to the owner; record the owner's words, or *not yet shown*.
- [ ] **Step 9: run the acceptance commands.** Commit the results file, both checks and their
  fixtures, and the sweep by path, each message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
bash docs/verification/phase-w/check-wl.sh
# expect: "WL coverage: 12 rows and 4 clauses, each with a command, an observation and a commit", exit 0
WL_RESULTS=docs/verification/phase-w/testdata/wl-missing-output.md bash docs/verification/phase-w/check-wl.sh; echo "exit $?"
# expect: "L-218: 1 empty cell(s)" then "exit 1"
WL_RESULTS=<(grep -v '^| L-97 |' docs/verification/phase-w/testdata/wl-complete.md) bash docs/verification/phase-w/check-wl.sh; echo "exit $?"
# expect: "L-97: no row in The clauses" then "exit 1"
bash docs/verification/phase-w/check-ledger.sh
# expect: "Ledger: 257 rows, closed <n> · partial <n> · exists <n> · other <n> · skip 27 · gap <n>", exit 0
W_RESULTS=docs/verification/phase-w/testdata/ledger-hidden-row.md bash docs/verification/phase-w/check-ledger.sh; echo "exit $?"
# expect: a line "L-209: not in the ledger", then "exit 1"
W_RESULTS=docs/verification/phase-w/testdata/ledger-wrong-verdict.md bash docs/verification/phase-w/check-ledger.sh; echo "exit $?"
# expect: "L-62: verdict MATCH, 05 says SKIP" and "L-62: a SKIP row filed as closed", then "exit 1"
npm --prefix desktop test -- test/wlSweep
# expect: 14 tests pass (12 surfaces, the once-each check, the dashboard words)
go test ./control-plane/... -race -count=1 && go test ./control-plane/... -race -count=1 -tags integration
# expect: ok for every package; no FAIL line
npm --prefix desktop test && npm --prefix desktop run typecheck
# expect: every suite passes; exit 0
grep -cE "^## (WL: |The phase ledger)" docs/verification/2026-09-25-phase-w-results.md
# expect: 2
```

The ledger's `<n>` counts are the run's, never typed in advance (decision 5); only `skip 27` and
the total are known now (`coverage.md:10`). If 05 §1 moves a row, the expected lines move with it
and the section says why.

#### What this task does NOT claim

- That any vendor accepts what WL sends: Slack, Azure Monitor, Sentry, PagerDuty, Opsgenie,
  Microsoft Graph and the warehouse sink are fakes with golden payloads; each proves our side of
  its wire only (decision 1).
- That a signed land commit shows as verified on Azure DevOps or GitHub, or that signing works in
  the cloud sandbox (181); or that the differ behaves on real repositories (179).
- That the phase is complete. The ledger records what was proved; its `gap` and `partial` rows
  are open work, and its count is the number filed, not 05's forecast.
- That an `exists` row is built, unless its row carries the commit it was re-checked at.
- That L-50 should now read MATCH in 05: that is the owner's call (the clause section).
- The owner's acceptance of any WL surface until the owner's words are recorded (decision 7).

**Parity.** This task closes no row itself (`coverage.md:22-23`). It records the twelve *later*
rows and four clauses as closed **only** where the tables carry a command, an observation and a
commit, and files every one of the 257 rows in the ledger.

#### Gaps found while writing this part

1. **Four mixed-stop rows have a *later* half no task builds.** L-120 (free `dc-` documents,
   `exists / later`, no closer at all, `coverage.md:163`), L-127 (Mermaid, `W1 / later`, `:170`),
   L-166 (browser testing in the sandbox, `W3 / later`, `:209`) and L-231 (tabs, `exists /
   later`, `:274`). `check-wl.sh` does not select them, by design; the ledger files each as
   `partial` with the later half named in *Note*. The skeleton has no WL row for any of the four.
2. **The coverage map is pinned to `c5841bf`** (`coverage.md:11`), before parts 143-186 were
   written. Step 7 builds the ledger from it, so a part that closes a row its scope did not cite
   (L-116's W5 half is the known case, task 174's gap 1) shows as a gap until the map is re-run.
   No task owns re-running it.
3. **Seven results sections, seven copies of one check.** `check-wl.sh` is `check-w5.sh` with a
   fence, an exact match and a clause loop; task 174's gap 3 asks task 70 for one
   `check-slice.sh <slice>`. `check-ledger.sh` could then share its `cells` reader.
4. **The three dashboard test file names are inferred** from task 177's *"(+ tests)"*; the sweep
   is corrected in step 1 if 177 names them otherwise.
5. **05 §1's cell parser assumes no `|` inside a cell.** Both checks split on `|`; an escaped
   `\|` in a *how* cell would shift the stop cell. None was found on 2026-09-26 (the twelve-row
   count matched the scope), but nothing enforces it.
