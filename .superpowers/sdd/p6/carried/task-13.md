# Carried plan issues — task 13 (Project route source: Hub, file, cache, fail closed)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## R-8 (Minor, gap, b02) — `Version` no longer covers every field

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:46`, finding `R-8`.
Shared with task 03, which is merged before you and settles the wording.

> **R-8 (Minor, gap, tasks 03 and 13). `Version` no longer covers every field.** The `Version` comment says it "changes when any field above changes" (`task-03.md:554`). `TeamID` now sits above it, but `fromFields` hashes only the H-1 fields (`task-13.md:695-701`), and the team is set afterwards (`:893`). The behaviour is unaffected: a team cannot change in v1 (`hub-contract.md:729`), and nothing reads `.Version` outside task 13's tests. Fix: reword the comment, or hash the team too.

Root-cause reading, declared under rule 7: hashing `TeamID` too makes the comment true
without a new claim, and R-8's own reason for calling it harmless (a team cannot change in
v1) is a v1-only argument. Prefer hashing the team; if task 03 has already reworded the
comment instead, follow task 03.
