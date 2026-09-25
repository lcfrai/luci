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
