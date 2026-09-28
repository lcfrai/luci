# Carried plan issues — task 07 (keysmith core: mint, refresh, revoke)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## R-1 (Minor, new in b02) — one mutant row names a call that `mint` no longer makes

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:29`, finding `R-1`.

> **R-1 (Minor, new, task 07). One mutant row names a call that `mint` no longer makes.** `task-07.md:1267` reads "in `mint`, move the two `s.ensure` calls above `s.route`". b02 replaced that call; the diff of `8c96bc9` shows `-route, err := s.route(ctx, sub.project)` / `+route, team, err := s.project(ctx, sub.project)`. Only `ByKeyHash` still calls `s.route` (`:961`), so the row cannot be applied as written. Fix: "above `s.project`".
