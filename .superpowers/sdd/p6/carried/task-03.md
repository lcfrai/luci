# Carried plan issues — task 03 (Wire contracts, problem codes, `MessagesURL`, `HashToken`)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## R-8 (Minor, gap, b02) — `Version` no longer covers every field

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:46`, finding `R-8`.
Shared with task 13. The comment is yours (`task-03.md:554`); the hash is task 13's
(`task-13.md:695-701`). Task 03 is wave 2 and task 13 wave 3, so settle the wording here and
let task 13 follow it.

> **R-8 (Minor, gap, tasks 03 and 13). `Version` no longer covers every field.** The `Version` comment says it "changes when any field above changes" (`task-03.md:554`). `TeamID` now sits above it, but `fromFields` hashes only the H-1 fields (`task-13.md:695-701`), and the team is set afterwards (`:893`). The behaviour is unaffected: a team cannot change in v1 (`hub-contract.md:729`), and nothing reads `.Version` outside task 13's tests. Fix: reword the comment, or hash the team too.
