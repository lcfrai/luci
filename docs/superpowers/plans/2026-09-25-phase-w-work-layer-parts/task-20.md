### Task 20: desktop test kit — `expectNoPrediction` and the Work fixtures

W1, stream WD1, seam **SW.8** (called by every desktop surface task). No dependency: it is pure
test code over the DOM and plain data, so it can start on day one.

`OD-27` is the owner's ruling on progress: show **which** criteria are proven and plain counts per
group; still banned are percentages, progress bars, ETAs and "N of M" completion figures. It
restates `AGENTIC-UX.md`'s anti-pattern 2 (*"No percent-complete, progress bar, ETA, or "N of M
met" anywhere in the product"*, `docs/AGENTIC-UX.md:237`). A rule that every surface must keep is
only kept if every surface test checks it, the same way, with the same exemptions. This task
writes that check once, `expectNoPrediction(container)`, and the fixtures every Work surface
test renders, so a surface task's test is two lines: render the fixture, call the check.

**The one thing this task must not get wrong is the exemptions.** A check that bans `50%` in a
quoted diff, or bans *"waiting 2h 10m"*, will be switched off by the first surface that trips it,
and then it guards nothing. So the check exempts by **explicit attribute only**, never by a
heuristic: `data-verbatim` skips an element whole (transcript, file or diff text, which may
legitimately say `50%`), and `data-projection-rule` allows a time projection that names its rule
(05's answer to Q1, `05-design.md:1358`, *"A projection that names its rule and source span
(next decision ~14:22 · rule: migration gate) is allowed"*, one of Q13's amendments, which scope
20 numbers as the fourth — **UNVERIFIED** which number `AGENTIC-UX.md` will give it) while still checking that
element for percentages, bars and "N of M". Plain counts (*"4 issues"*), clock times
(*"14:02"*), aging (*"waiting 2h 10m"*) and ids (*"EP-142"*) always pass.

**What is on the tree today (verified 2026-09-25).** `desktop/src/test/` holds `fakes.ts`,
`fixtures.ts` and `setup.ts`. `fixtures.ts:1-9` sets the kit's rule: builders are declared
**once**, a variant is an override object, never a near-duplicate or a local builder. `FIXED_NOW`
(`1789722000000`, `2026-09-18T09:00:00.000Z`) and `FIXED_NOW_ISO` are at `fixtures.ts:25-26`.
`desktop/vitest.config.ts:11` runs `./src/test/setup.ts` for every test, in `jsdom`, with
`globals: false`, so every test imports `describe`/`it`/`expect` from `vitest`.

**Files:**
- `desktop/src/test/expectNoPrediction.ts` — the check (new)
- `desktop/src/test/expectNoPrediction.test.ts` — the rows below (new)
- `desktop/src/test/workFixtures.ts` — `WORK_IDS`, the local mirror types, eight builders and
  `workScenario()` (new)
- `desktop/src/test/workFixtures.test.ts` — the grammar, coverage and self-check rows (new)
- nothing else

**Interfaces:**
- Produces (**SW.8**): `expectNoPrediction(container: HTMLElement): void`;
  `findPredictions(container: HTMLElement): Prediction[]` (the same scan, returning instead of
  throwing, for tests that assert a specific finding); `WORK_IDS`; `makeIssueHead`,
  `makeIssueRevision`, `makeServe`, `makeCriterion`, `makeComment`, `makeLabel`,
  `makeTeamPolicy`, `makeWorkEvent`; `workScenario()`; `WORK_STATES`, `PROOF_WORDS`.
- Consumes: `FIXED_NOW`, `FIXED_NOW_ISO` from `fixtures.ts`; task 03's JSON field names
  (`task-03.md:279-398`) and task 02's (`task-02.md:226-244`), mirrored by hand here.
- Required by: every desktop surface task of W1-W5 (tasks 26 onward), task 37 (which replaces the
  local mirror types with imports from `src/api/work.ts` and whose typecheck proves they agree).

