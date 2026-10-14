# Carried plan issues — task 21 (Control-plane inference API)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## B04R2-1 (Minor), your third of it — a stale "task 23 step 3" citation on task 21's own interface line

Source: `.superpowers/sdd/p6/plan-rereview-b04-2.md:39-44`, finding `B04R2-1`.
Tasks 16 and 22 carry the other two.

> **B04R2-1 (Minor): three stale "task 23 step 3" citations of the raise remain, one of them on task 21's own interface line.**
> - `task-21.md:30`: "Required by: ... task 23 (step 3 raises a project through this route)". Task 23's step 3 starts the control plane; the raise is step 6. This is also the line that should say task 23 now runs 21's by-hand acceptance (steps 4 and 6), and it does not.
> - ...
> - Fix, about 3 minutes: change the three to step 6. At `task-21.md:30`, add "and steps 4 and 6 run this task's by-hand acceptance (re-review B04-1)".

---

## Context you must not re-litigate: the route file is `state/projects.json`

b08 settled the route-file path for the whole phase
(`.superpowers/sdd/p6/plan-rereview-b08-1.md:9-31`). Your gap 3 was the origin of the drift:
task 21 is wave 6 and task 17 wave 7, so gap 3 recorded the path it assumed before any mount
existed, and task 23 copied that assumption. Gap 3 is now marked settled by task 17. Keep the
mount-the-directory requirement of decision (h); do not reintroduce a `projects/`
subdirectory.
