### Task 44b: the contract made true to what was **built**; the seams handed over

**`feat/hub`, last. Depends on: 43. About 1 hour. Documents only.**

Task 44a folded every correction, addition and sibling answer this **plan** knew into the contract
before task 01 (it was task 44's first half, split so that the siblings read the corrected text
first — review finding I11). This task folds in only what the **build** changed afterwards, so the
contract that phases 3, 4, 5 and 7 hold at the end describes the Hub that exists.

**Files:**
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` (modify)
- `docs/PROGRAM.md` (modify ⚑ — only if a seam's shape changed; additions only)
- `docs/superpowers/plans/2026-09-23-phase-h-hub.md` — the seams table, only if a shape changed (modify)

#### What changes, exactly

1. **Every declared departure.** Read each task report's *departures* and *what this task does
   not claim* sections (`.superpowers/sdd/hub/`) and task 43's results record. Each departure that
   changes a shape, a code, a status or a limit the contract states is made **in place** and
   marked `(Phase H build, task NN)`, and gets a row in §16's table beside 44a's. A departure that
   changes an S0 interface (task 00's rule: anything but an added method, field or route) is
   **not** folded silently: it is a reopened ask, named in the report to the orchestrator, which
   tells the sibling phase that codes to it, with the sha and the `git show <sha>:<contract path>`
   read path, as task 44a's step 3 does.
2. **§15, what the build verified.** Remove the pgx pin and the `go 1.23.0` directive once task 06
   built them; add anything task 43 found unverified (the real-tenant checklist's open rows stay
   until the owner's batched session, `OD-12`).
3. **The seams handed over.** For each seam of the plan's table (S-CRED, S-TOKEN, S-HUB, S-LINK,
   S-FEED, S-ENTRA, S-CHART), one line in §16 naming the commit on `feat/hub` where it landed; a
   reserved seam (S-CI, S-CANVAS, S-SSE, S-BYO, S-CSP, S-DEPLOY-TARGET) stays reserved.

If the build changed nothing the contract states, step 1 records *"no departures"* in §16 and the
task is steps 2 and 3 only.

#### Tests to write first

Documents have no unit tests. Task 01's `TestTheContractsRepoKeyRegexIsTheCodes` runs again here:
a build departure in the repo-key rule that was not folded in turns it red.

#### Steps

- [ ] **Step 1:** Read the task reports and task 43's record; list the departures.
- [ ] **Step 2:** Make each change above.
- [ ] **Step 3:** Commit by pathspec — the documents changed.

#### Acceptance

```bash
C=docs/superpowers/contracts/2026-09-23-hub-contract.md
grep -c "(Phase H build, task" $C                                   # = the departures listed in step 1 (0 is valid, with "no departures")
grep -n "S-CRED\|S-HUB\|S-FEED" $C                                   # each seam's landing line
go test ./control-plane/internal/hub/hubapi/ -run TestTheContractsRepoKeyRegexIsTheCodes -count=1   # ok
```

#### What this task does not claim

- That sibling phases have re-read the amended contract: the orchestrator tells them.
- Any change to the ADR's decision: ADR-H1 stands; only the contract's text moves.
