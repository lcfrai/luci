# Carried plan issues — task 09 (Usage rollup and the team-budget reader)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## b06 Important 1 — one of the three mutant rows added to close round 1 cannot fail

Source: `.superpowers/sdd/p6/plan-rereview-b06-2.md:59-74`, "Important 1".
This is the row that guards the exact defect it was added for, so treat it first.

> **1. `task-09.md:1025` — one of the three mutant rows added to close round 1's Important 2 cannot
> fail, and it is the row that guards the exact defect Important 2 named.**
> The row is `RecordKey`: `CreatedAt: b.CreatedAt` -> `CreatedAt: s.clock()`, killed by
> `TestRecordKeyFilesTheSpendASweepWouldMiss` "(filed under the call's month, not the key's, so the
> month boundary double-counts)". In that test `now` is `2026-09-25 09:00 UTC` (`:405`) and the key is
> `ref("k1", now.Add(-30*time.Minute))` (`:409`) — `2026-09-25 08:30 UTC`. Both are in September, so
> `budget.PeriodStart(k.CreatedAt)` and `budget.PeriodStart(s.clock())` are the same `sep`
> (`:88`), the single assertion `sum(t, store, wire.ScopeTeam, "team-1", sep) != 7` (`:415`) still
> holds, and the mutant survives.
> Failure scenario: the implementer reaches step 7, applies the edit, the named test passes, and the
> gate has no verdict for it — and the defect round 1 described (a `RecordKey` that files under
> `PeriodStart(now)` instead of the key's month, double-counting the team dimension across a month
> boundary) passes the whole suite exactly as it did before the fix. Fix: move the key's `CreatedAt`
> into the previous month — `now := oct.Add(9 * time.Hour)` with `k := ref("k1", sep.Add(36*time.Hour))`
> — and assert both `sum(..., sep) == 7` and `sum(..., oct) == 0`, so the two months are distinguishable.
> `oct` already exists at `task-09.md:89`.

---

## b06 Minor 5 — the skeleton's task 09 summary does not carry `RecordTimeout`

Source: `.superpowers/sdd/p6/plan-rereview-b06-2.md:143-148`, "Minor 5".
The edit is in the skeleton, not in `task-09.md`; commit it by pathspec alongside your part.

> **5. `2026-09-25-phase-6-gateway.md:238` — the skeleton's task 09 summary does not carry
> `RecordTimeout`.** It states `RecordKey` is "the same read and upsert for one key on demand ...
> called immediately before every `/key/delete`", and task 07's summary at `:234` states the rule and
> the report-and-proceed behaviour, but neither says the call bounds itself at 5 s — the whole of
> round 1's Important 3. Every other cross-task invariant of this pair is in the skeleton, and M-1 was
> raised because the skeleton and a part disagreed. One clause on `:238`.

---

## R-5 (Minor, gap, b02) — one mutant in `checkPrices` survives

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:36-39`, finding `R-5`.

> **R-5 (Minor, gap, task 09). One mutant in `checkPrices` survives.**
>   - In `TestSweepReportsAnUnpricedGroup` (`task-09.md:432-453`), no group has a positive output price and a zero input price. So deleting `!(p.Input > 0) ||` at `:838` leaves an output-only check that still reports exactly `au/m` and `au/unlisted`, and the test passes.
>   - The table (`:937`) kills only the symmetric deletion.
>   - Fix: add a fixture group such as `"au/n": {Output: 5e-6}` (now three reports) and a mutant row. Step 7 becomes 11 edits.

---

## R-6 (Minor, gap, b02) — shared with tasks 04 and 24

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:40-44`, finding `R-6`. Full text in
`.superpowers/sdd/p6/carried/task-04.md`. Your half: `GroupPrice` carries only `Input` and
`Output`, so the cache-read and cache-write figures are gated at build time only, and the
sweep's run-time check sees two of four figures. Coordinate with tasks 04 and 24 before you
change the shared `GroupPrice`.
