### Task 113: W3's proof and results

Stream **W3** (agents on the work), about 2.5 hours of work beside the run itself (an estimate,
not a measurement). Depends on tasks **98-112**. Writes the W3 section of
`docs/verification/2026-09-25-phase-w-results.md` ⚑, owned jointly with tasks 70, 97, 142, 174
and 187 and appended in task order; task 70 creates the file on the model of
`docs/verification/2026-09-17-phase-1-results.md` (built: a lead line, *What was tested*, *The
checks* as a table, *What is still unproven*, *Reproducing this*).

`OD-27` is the rule: **proof per criterion, no prediction.** Every row this section marks closed
carries the command that was run, an excerpt of what it printed, and the commit it ran at. A row
with no output beside it is not closed, whatever a part or a person says.

**Decisions made here, so nobody re-derives them:**

1. **The environment is named, first line.** The run uses TM1's cluster if Fleet is deployed
   there on the day, else Fleet's local stack (Phase T's local `fleetd` against the local Hub
   through `devissuer`). The section's lead line says which, with the Fleet commit and the Hub
   commit. A run on the local stack proves nothing about the cluster, and the section says so.
2. **The rows are re-derived, not copied.** `check-w3.sh` reads 05 §1's matrix and selects every
   row whose slice column names W3. On 2026-09-25 that gives eighteen: L-39, L-156, L-158, L-162,
   L-166, L-173, L-174, L-182, L-183, L-185, L-186, L-187, L-188, L-190, L-192, L-193, L-194,
   L-246 (checked by running the same selection over `05-design.md` while writing this part). If
   05 has moved a row in or out, the check fails and the section is corrected before it is filed.
3. **Each row names its closing task and test.** The table below is the plan; the section records
   the test's real name and its `PASS` line from the run. Four rows are **halves** (L-39, L-166,
   L-187 with W1 or Phase T; L-156 is PARTIAL by design) and say which half.
4. **One scripted run, eight frames**, each a step with its command, the observation to record and
   its time (`docs/verification/phase-w/w3-run.md`). The frames follow the scope: delegate, the
   Orchestrator reads, workers carry `IssueRef`, one note in one voice, evidence proposed and
   recorded, a cross-owner `blocks` seeds a `sequence`, both issues show the negotiation row, the
   switch stops the next turn.
5. **Observed, never inferred.** A frame's observation is what a person saw or a command printed,
   quoted. *"The Orchestrator decided to …"* is not an observation; *"`panacea issue show EP-142`
   printed `issue:is-…@3`"* is. Nothing is read from a harness's terminal (`OD-8`); what an agent
   did is read from Fleet's records and the Hub.
6. **The owner's look is its own row.** The W3 surfaces (delegate chip, agent byline, proposed
   evidence, negotiation row, task card, step fold, settings, ask, risk facts) are shown to the
   owner and the verdict is recorded in the owner's words. `feat/work-layer` is not merged onward
   before it (`OD-1`; `CLAUDE.md`: *"Visual acceptance is the owner's eyes"*).
7. **What stays unverified is listed by name** (the scope's five plus those parts 109-112 found):
   task 112's runner, the injection corpus for hostile issue text (05 §3.4, Q3), Orchestrator
   convergence (TM-05 §14), the transcript adapter's reach (task 108), the owner's visual
   acceptance, where guidance enters an agent's context (task 109), *pause all* in local mode
   (task 109), the local delegation for solo writes (task 112), and the `ask` class's policy on the
   pinned CLI (task 110).

**Files:**
- `docs/verification/2026-09-25-phase-w-results.md` ⚑ — the W3 section (modified; task 70's file)
- `docs/verification/phase-w/w3-run.md` — the eight frames a person runs (new)
- `docs/verification/phase-w/check-w3.sh` — the coverage check (new)
- `docs/verification/phase-w/testdata/{w3-complete,w3-missing-output}.md` — the check's fixtures (new)
- `desktop/src/test/w3Sweep.test.tsx` — `expectNoPrediction` over every W3 surface in one file (new)

#### The rows W3 closes, and what closes each

Test names for tasks 109-112 are those parts' own. For the others the row names the behaviour the
scope's test holds (`tasks-101-120.md`, `tasks-81-100.md`); step 1 replaces each with the test's
real name from that part's mutant table, and the run pastes its `PASS` line.

| Row | 05 §1 (`05-design.md`) | Closed by | The test that must pass |
|---|---|---|---|
| L-39, W3 half | `:148` EXCEED | 102 (W1 half: 61) | `Shift+O` on a teammate's issue is refused, *"only Dylan can delegate EP-142"*; task 98's non-assignee bind refused |
| L-156 (PARTIAL) | `:318` | 103; solo: 112 | Alice's `@me:orchestrator` on Dylan's issue enqueues Alice's Orchestrator, never Dylan's |
| L-158 | `:320` | 109 | `TestSwitchOffEnqueuesNoTurnForAnyOrigin`; `KillSwitch` *no control for a member*; Desk *pause all sends project ids only* |
| L-162 | `:324` | 110 | `TestCitationChecks`; `AskAnswer` *links only a checked citation* |
| L-166, W3 half | `:328` PARTIAL | 101 and 104 | an `evidence_proposal` leaves the done evaluator unchanged; *Record* is sent with the person's token |
| L-173 | `:335` | 103 | an agent comment renders owner, agent, model and session |
| L-174 | `:336` EXCEED | 111 | `TestAMigrationFileRaisesRunsMigrationWithItsPath`; `RiskFacts` *no number, level or colour* |
| L-182 | `:349` EXCEED | 107 | *Alice's agents* card offers no steering action; `whoLabel` goldens |
| L-183 | `:350` | 109 | `AgentGuidance` *person, team, org order* and *a new version, the old kept* |
| L-185 | `:352` EXCEED | 108 | the fold prints command, duration, exit and lines, never raw output; unreadable adapter degrades |
| L-186 | `:353` | 108 | liveness text changes only on the heartbeat tick; no spinner or `progressbar` |
| L-187, W3 half | `:354` EXCEED | 98 and 101 | a task's `issueId` for another project is refused; `panacea team` shows the issue title, `titleClass: "H"` |
| L-188 | `:355` | 108 | a link renders only where its fact exists |
| L-190 | `:357` | 108 | with no span after 10 s, *no output yet · 10s* under fake timers |
| L-192 | `:359` EXCEED | 111 | `TestAOneRepoProjectGetsNoProposal`; `TestAnUncalibratedConfidenceIsDropped` |
| L-193 | `:360` | 111 | conformance row *stored in triage whatever the body says*; `TestIssueFileRefusesAStateArgument` |
| L-194 | `:361` | 109 | `agentRules.test.ts` *every rule cites a heading that exists* |
| L-246 | `:428` | 109 | as L-158 |

Rows W3 **carries but does not close**: L-223's `Shift+O` chord becomes live (task 102; the row is
W1's, closed by task 53). Rows **closed only on team projects**, which the section says in the
row: L-158's *pause all* (task 109, gap 4), L-162 (task 110, decision 1), L-192 (task 111).

#### Interfaces — complete: the W3 section's shape (appended to the results file)

```markdown
## W3: agents on the work — <date>

**Environment:** <TM1 cluster `aks-…` | Fleet's local stack>, Fleet at `<sha>`, Hub at `<sha>`,
desktop and control plane at `<sha>`. <One sentence: what this environment does not prove.>

### The run (docs/verification/phase-w/w3-run.md)

| Frame | Command or act | Observed (quoted) | Time | Commit |
|---|---|---|---|---|
| 1 delegate | … | … | … | … |

### The rows

| Row | Task | Test (real name) | Command | Observed (excerpt) | Commit |
|---|---|---|---|---|---|
| L-39 (W3 half) | 102 | … | … | … | … |

### Task 112 spike

<task 112 step 2's paragraph: each command, its output, the commit, and yes or no>

### The suites

| Suite | Command | Result line (quoted) |
|---|---|---|
| Go | `go test ./control-plane/... -race -count=1` | … |
| Hub integration | `go test -tags integration ./control-plane/internal/hub/...` | … |
| Desktop | `npm --prefix desktop test` | … |
| Types | `npm --prefix desktop run typecheck` | … |

### The owner's look

<the owner's words, verbatim, with the date; or "not yet shown">

### What is still unproven

- <each item of task 113's decision 7, one line each, with the test that would settle it>
```

Every `…` is filled from the run. A cell left `…` fails the coverage check below.

#### Interfaces — complete: `docs/verification/phase-w/check-w3.sh`

```bash
#!/usr/bin/env bash
# check-w3.sh: every W3 parity row in 05 §1 has, in the W3 results section, a task, a test, a
# command, an observed excerpt and a commit (OD-27). Exit 0 only when all do.
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
MATRIX="${W3_MATRIX:-$ROOT/docs/superpowers/planning/2026-09-25-linear-parity/05-design.md}"
RESULTS="${W3_RESULTS:-$ROOT/docs/verification/2026-09-25-phase-w-results.md}"

rows=$(awk -F'|' '/^\| [0-9]+ \|/ && $(NF-1) ~ /W3/ { gsub(/ /, "", $2); print "L-" $2 }' "$MATRIX")
[ -n "$rows" ] || { echo "no W3 rows in 05 §1: the matrix moved"; exit 2; }

section=$(awk '/^## W3: /{on=1; print; next} on && /^## /{exit} on{print}' "$RESULTS")
[ -n "$section" ] || { echo "no W3 section in $RESULTS"; exit 2; }
table=$(printf '%s\n' "$section" | awk '/^### The rows/{on=1; next} on && /^### /{exit} on')

fail=0
for r in $rows; do
  line=$(printf '%s\n' "$table" | grep -E "^\| $r( |\))" | head -1 || true)
  if [ -z "$line" ]; then echo "$r: no row in the W3 section"; fail=1; continue; fi
  empty=$(printf '%s\n' "$line" | awk -F'|' '{c=0; for (i=2; i<NF; i++) { v=$i; gsub(/^ +| +$/, "", v); if (v=="" || v=="…") c++ }; print c}')
  commit=$(printf '%s\n' "$line" | awk -F'|' '{v=$(NF-1); gsub(/^ +| +$/, "", v); gsub(/`/, "", v); print v}')
  [ "$empty" = 0 ] || { echo "$r: $empty empty cell(s)"; fail=1; }
  printf '%s' "$commit" | grep -Eq '^[0-9a-f]{7,40}$' || { echo "$r: commit cell is not a sha"; fail=1; }
done
for e in $(printf '%s\n' "$table" | grep -oE '^\| L-[0-9]+' | tr -d '| ' | sort -u); do
  printf '%s\n' "$rows" | grep -qx "$e" || { echo "$e: in the section but not a W3 row of 05 §1"; fail=1; }
done
if printf '%s\n' "$section" | grep -q '…'; then echo "an unfilled … is left in the W3 section"; fail=1; fi
[ "$fail" = 0 ] && echo "W3 coverage: $(printf '%s\n' "$rows" | wc -l | tr -d ' ') rows, each with a command, an observation and a commit"
exit "$fail"
```

`W3_RESULTS` points the check at a fixture for its own test: two files under
`docs/verification/phase-w/testdata/`, `w3-complete.md` (all eighteen rows filled with a sample
sha) and `w3-missing-output.md` (the same with L-162's *Observed* cell empty). The first must print
the coverage line and exit 0; the second must print `L-162: 1 empty cell(s)` and exit 1 (mutant: a
row marked closed from a self-report).

#### Interfaces — complete: `docs/verification/phase-w/w3-run.md`

```markdown
# W3 run: agents on the work

Two people signed in: Dylan (owner of EP-142) and Alice (owner of EP-150), one team project.
Record every observation as quoted text or a pasted output line, with the time (Commit Mono).

1. **Delegate.** Dylan focuses EP-142 in the Work lens and presses `Shift+O`.
   Record: the row's chip text and the `issue.delegated` event id from the Hub's change feed.
2. **The Orchestrator reads.** From Fleet's dispatch audit for Dylan's Orchestrator.
   Record: the `issue.show` request's `issue` arg and the result's `issue:is-…@r` key.
3. **Workers carry the issue.** `panacea team` from one of Dylan's workers.
   Record: each row's `issue.key` and `titleClass`; the session's `IssueRef` from `GET /v1/sessions`.
4. **One note, one voice.** After the workers report, open EP-142's comments.
   Record: the number of agent comments and each byline (`dylan · orchestrator (agent)`), model, session.
5. **Evidence proposed, then recorded.** In the Merge lens, under the criterion: the proposal row.
   Dylan presses *Record* with a reason. Record: the `ev-…` id, its `recordedBy`, and the row's link.
6. **Cross-owner `blocks`.** Alice adds *EP-150 blocks EP-142*.
   Record: the turn origin `relation` on **Alice's** Orchestrator, the `th-…` thread and its `sequence` proposal.
7. **Both issues show the row.** Open EP-142 and EP-150.
   Record: the negotiation row's text on each, verbatim; then the folded glyph after it is seen.
8. **The switch.** An org admin turns Orchestrators off with a reason; Dylan presses `Shift+O` on EP-143.
   Record: the palette's sentence, and Fleet's refused-turn row with code `agents_disabled`.

No step reads a harness's terminal output (OD-8). If a step cannot be run in the environment, write
**not run** and why; never a predicted result.
```

#### Tests to write first — `desktop/src/test/w3Sweep.test.tsx`

The sweep does two things. It checks that **every** W3 surface's own test file calls
`expectNoPrediction` (a surface that skipped the helper is caught even when its props are not
known here), and it renders the four surfaces whose props this plan fixes (tasks 109-111) with
their most number-prone data, through the helper, in one place.

```tsx
import { readFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { expectNoPrediction } from "./expectNoPrediction";
import { AskAnswer } from "../components/work/AskAnswer";
import { RiskFacts } from "../components/work/RiskFacts";
import { RepoProposal } from "../components/work/RepoProposal";
import { agentsOffLine } from "../components/admin/agentsOff";

const SRC = resolve(__dirname, "..");
/** Every W3 surface and its owning task. A new W3 surface is added here by its task. */
const W3_SURFACES: [string, number][] = [
  ["components/work/DelegateChip.test.tsx", 102],
  ["components/issue/AgentByline.test.tsx", 103],
  ["components/issue/ProposedEvidence.test.tsx", 104],
  ["components/issue/NegotiationRow.test.tsx", 106],
  ["components/work/AgentTaskCard.test.tsx", 107],
  ["components/issue/StepFold.test.tsx", 108],
  ["components/issue/SessionLinks.test.tsx", 108],
  ["components/admin/KillSwitch.test.tsx", 109],
  ["components/admin/AgentGuidance.test.tsx", 109],
  ["components/work/AskAnswer.test.tsx", 110],
  ["components/work/RiskFacts.test.tsx", 111],
];

describe("W3 sweep: no prediction on any W3 surface (OD-27)", () => {
  it.each(W3_SURFACES)("%s (task %i) exists and calls expectNoPrediction", (file) => {
    const path = resolve(SRC, file);
    expect(existsSync(path)).toBe(true);
    expect(readFileSync(path, "utf8")).toMatch(/expectNoPrediction\(/);
  });

  it("renders the number-prone W3 surfaces with no percentage, bar, ETA or N of M", () => {
    const SHA = "7c1f0a2d9e0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d";
    const many = Array.from({ length: 7 }, (_, i) => `m/00${i}.sql`);
    const { container } = render(
      <div>
        <AskAnswer openAt={vi.fn()} result={{ state: "answered", question: "q", questionClass: "H",
          answer: "a", answerClass: "A", commit: SHA,
          citations: [{ path: "a.go", lineStart: 3, lineEnd: 7, check: "ok" }, { path: "b.go", lineStart: 1, lineEnd: 2, check: "missing_lines" }] }} />
        <RiskFacts reply={{ lane: "ln-1", globsSet: true, scopeChecked: true, teamLane: true,
          facts: [{ name: "runs-migration", paths: many, because: "adds or changes a file matching `m/*.sql` …" }] }} />
        <RepoProposal choice={{ remote: "rm-2", because: "b", confidence: 0.7 }}
          remotes={[{ id: "rm-1", label: "one" }, { id: "rm-2", label: "two" }]} />
        <p>{agentsOffLine({ orchestrators: false, by: null, at: null, reason: "" })}</p>
      </div>,
    );
    expectNoPrediction(container);
  });
});
```

`+6 paths` (seven paths in one collapsed fact) is the most number-shaped string W3 prints; the
sweep holds that task 20's guard does not read it as progress. If it does, the words change in
`RiskFacts.tsx`, never the guard.

| Test | Mutant it kills |
|---|---|
| each W3 surface's test calls the helper | a surface that skipped `expectNoPrediction` |
| the number-prone surfaces render clean | *"3 of 7"*, a `%` or a bar sneaking into an answer, a fact or a proposal |
| `check-w3.sh` on `w3-missing-output.md` exits 1 | a row marked closed from a self-report |
| `check-w3.sh` on `w3-complete.md` exits 0 | a check that fails everything, so nobody runs it |
| `check-w3.sh` rows from 05 §1 | a row that moved in 05 and was not re-derived |

#### Steps

- [ ] **Step 1: read on the sync base** the parts and code of tasks 98-112 and fill the closing
  table's *test* column with each real test name; read task 70's W1 section for its headings and
  keep the W3 section in the same shape. If task 70 already ships a coverage check, `check-w3.sh`
  calls it with the W3 row list instead of repeating it.
- [ ] **Step 2: the tests first**: `w3Sweep.test.tsx`, then `check-w3.sh` with its two fixtures.
  `npm --prefix desktop test -- test/w3Sweep` fails until every W3 surface exists; the check on
  the missing-output fixture exits 1.
- [ ] **Step 3: name the environment** (decision 1), bring it up, and record its commits.
- [ ] **Step 4: run the eight frames** of `w3-run.md` and fill the run table, quoting each
  observation. An excerpt is shortened with `[cut]`, never with `…`, so the check can tell an
  unfilled cell from a shortened one.
- [ ] **Step 5: run the suites** and each of tasks 98-112's acceptance commands; paste each result
  line into the rows table beside its row.
- [ ] **Step 6: paste task 112's spike paragraph**, list what is still unproven (decision 7), and
  show the W3 surfaces to the owner; record the owner's words, or *not yet shown*.
- [ ] **Step 7: run the acceptance commands.** Commit the results file, the run script, the check
  and its fixtures, and the sweep by path, each message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
bash docs/verification/phase-w/check-w3.sh
# expect: "W3 coverage: 18 rows, each with a command, an observation and a commit", exit 0
W3_RESULTS=docs/verification/phase-w/testdata/w3-missing-output.md bash docs/verification/phase-w/check-w3.sh; echo "exit $?"
# expect: "L-162: 1 empty cell(s)" then "exit 1"
npm --prefix desktop test -- test/w3Sweep
# expect: 12 tests pass (11 surfaces and the render)
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line
npm --prefix desktop test && npm --prefix desktop run typecheck
# expect: every suite passes; exit 0
grep -cE "^## W3: " docs/verification/2026-09-25-phase-w-results.md
# expect: 1
```

The row count is 18 on 2026-09-25; if 05 §1 moves a row, the expected line moves with it and the
section says why.

#### What this task does NOT claim

- That W3 is adopted or wanted (05 §2.2: *"designed, not built or proven wanted"*).
- Anything about the environment it did not run in: a local-stack run proves nothing about TM1's
  cluster, and neither proves Kata sandboxes (phase 8).
- That two Orchestrators converge on a resolution beyond the one scripted `sequence` (TM-05 §14).
- That hostile issue text is fenced: the injection corpus (05 §3.4, Q3) is not part of this run.
- That the transcript adapter reads any harness version beyond the one its canary pinned.
- The owner's acceptance until it is recorded in the owner's words.

**Parity.** This task closes no row itself; it records the eighteen rows of decision 2 as closed
**only** where the rows table carries a command, an observation and a commit.

#### Gaps found while writing this part

1. **The W3 surface list lives in the sweep**, by hand; the skeleton's file map (`:306-315`) should
   list the W3 desktop files (DelegateChip, AgentByline, ProposedEvidence, NegotiationRow,
   AgentTaskCard, StepFold, SessionLinks, the admin pair, AskAnswer, RiskFacts, RepoProposal) so
   the sweep can be checked against it.
2. **Task 70 defines no coverage check yet** (its part is not written on 2026-09-25). If it adds
   one, `check-w3.sh` should become a W3 row list passed to it, so the six results sections share
   one check.
3. **`docs/verification/phase-w/`** (run scripts, the check, fixtures) is not in the file map's
   *Proof and documents* table (`:316-324`).
4. **Four rows close only in part or only on team projects** (L-158's *pause all*, L-162, L-192;
   L-156 by design). The skeleton's W3 totals do not say so.
