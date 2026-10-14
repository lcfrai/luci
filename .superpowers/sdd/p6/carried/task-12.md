# Carried plan issues — task 12 (`cmd/gatekeeper`: wiring, `/v1/models`, `HEAD /api/hello`, body-free logs)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## M-2 (Minor) — task 12's config fixture names a container path the stack never mounts

Source: `.superpowers/sdd/p6/plan-rereview-b08-1.md:65-69`, finding `M-2`.

> **M-2. Task 12's config fixture still names a container path the stack never mounts.**
> `task-12.md:304` and `:313` use `file:/etc/panacea/projects.json`, while every real gatekeeper reads
> `file:/etc/panacea/state/projects.json` (`task-17.md:51`, `:360`). Harmless to the parse test,
> misleading to an implementer who copies it — the same one-character drift b08 just closed. Task
> 08's fixture (`task-08.md:599`) is a `t.TempDir()` join and needs nothing.

Context: b08 settled the whole phase on `state/projects.json`. Do not move the path; move the
fixture onto it.
