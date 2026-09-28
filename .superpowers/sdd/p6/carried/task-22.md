# Carried plan issues — task 22 (Cockpit inference panel)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

OD-12 batches the look of the panel to the owner at the end of the programme. Nothing here
is a licence to accept the UI yourself.

---

## B04R2-1 (Minor), your third of it — one stale "task 23 step 3" citation

Source: `.superpowers/sdd/p6/plan-rereview-b04-2.md:39-44`, finding `B04R2-1`.
Tasks 16 and 21 carry the other two.

> **B04R2-1 (Minor): three stale "task 23 step 3" citations of the raise remain ...**
> - `task-22.md:22`: "Required by: task 23 (step 3 raises the project through this panel)". It should say step 6.
> - Fix, about 3 minutes: change the three to step 6.

---

## Context: your gap 5 formula is right, the skeleton's gloss of it was not

`canLowerRoute` at `task-22.md:718` was re-derived by hand against `task-01.md:591-618` and
`Floor`'s table and is correct
(`.superpowers/sdd/p6/plan-rereview-b05-2.md:14`). The wrong statement was the skeleton's
H-5 gloss ("always `false` here"), which is fixed plan-wide in this run — see
`.superpowers/sdd/p6/carried/plan-wide.md`. Do not change the field to match the old gloss:
for a caller holding H-4's compliance role on a project whose route is above its floor, the
formula returns `true`, and that caller is the only person OD-24's ratchet lets lower a team
project.
