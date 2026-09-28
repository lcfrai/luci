# Carried plan issues — task 16 (Fake Anthropic upstream)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## B04R2-1 (Minor), your third of it — one stale "task 23 step 3" citation

Source: `.superpowers/sdd/p6/plan-rereview-b04-2.md:39-44`, finding `B04R2-1`.
Tasks 21 and 22 carry the other two.

> **B04R2-1 (Minor): three stale "task 23 step 3" citations of the raise remain, one of them on task 21's own interface line.**
> - ...
> - `task-16.md:248`, a test comment: "task 23's step 3 could not see the route change". It should say step 6.
> - `review.md:276` says the restatement grep found only `task-21.md:1016-1017`. The grep matched "task 23 step 3" but not "step 3 raises" or "task 23's step 3".
> - All three are pre-existing text, but they sit in the exact restatement set that B04-1's fix says it swept. Fix, about 3 minutes: change the three to step 6.

Why step 6: b04 moved task 21's by-hand acceptance into task 23, where step 3 starts the
control plane and step 6 does the raise through the cockpit chip
(`.superpowers/sdd/p6/plan-rereview-b04-2.md:9-16`).
