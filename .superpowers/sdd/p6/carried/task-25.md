# Carried plan issues — task 25 (Close the phase: `PROGRAM.md`, seams, asks, what stays unverified)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it. You also close every carried finding:
before you write the phase close, check
`.superpowers/sdd/p6/carried/INDEX.md` and confirm each task's report says what it did with
its file.

---

## m-4 (Minor, b03) — U21's last clause cannot be settled where the plan says it will be

Source: `.superpowers/sdd/p6/plan-rereview-b03-2.md:39-44`, finding `m-4`.
(The bookkeeping nit at the end of `m-4` is plan-wide and is fixed in this run — see
`.superpowers/sdd/p6/carried/plan-wide.md`.)

> **m-4. U21's last clause cannot be settled where the plan says it will be, plus one bookkeeping nit.**
> - `task-25.md:127` assigns "a drop-in winning a top-level key over the base file" to task 23's item 10.
> - Phase 6 installs no base `managed-settings.json`. `task-14.md:532` installs one only if the drop-in alone is ignored, and even then it is the fragment itself. So item 10 has nothing to observe.
> - Gap 6 (`task-14.md:548`) already makes the question moot by requiring the base file to set `true` itself.
> - Fix: move that clause's "settled by" to S6.6.

---

## Inbound edit from task 14's m-1

Task 14's carried `m-1` (`.superpowers/sdd/p6/plan-rereview-b03-2.md:12-21`) names
`task-25.md:84` as one of four places that must stop crediting task 17's wall for a bound
the local sandbox network does not give. If task 14's implementer did not make that edit,
make it here: the residual at `:84` must name "any repository-chosen command, from the Bash
tool or from a settings key such as `statusLine`", and say its bound is sandbox egress —
open locally, phase 8 / P8-3 in the cloud (`task-19.md:895`).
