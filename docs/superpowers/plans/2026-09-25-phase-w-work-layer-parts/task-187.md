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
