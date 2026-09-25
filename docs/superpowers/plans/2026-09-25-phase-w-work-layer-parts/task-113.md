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
