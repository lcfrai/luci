# Carried plan issues — task 14 (`panacea token` and the managed-settings fragment)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## b07 Important 1 — the sentence that replaced the dropped guard claims an enforcement that does not exist

Source: `.superpowers/sdd/p6/plan-rereview-b07-2.md:64-80`, "Important 1".

> **1. The sentence that replaced the dropped guard claims an enforcement that does not exist.**
> `task-14.md:199`. It reads: *"It does not check `REFRESH_AT > Grace`. ... The relation is enforced
> where both values live: task 07 refuses a keysmith whose `TTL <= Grace` at construction
> (`task-07.md:915-916`), and task 15 bounds `PANACEA_APP_GATEWAY_REFRESH_AT` to 1 m-55 m against the
> token's one-hour life (`task-15.md:1639`, `:1643`)."* Both citations are real and neither enforces
> that relation. `task-07.md:915-916` is `if d.Grace < 0 || d.TTL <= d.Grace` — keysmith's **token
> TTL** against `Grace`, a different pair. `task-15.md:1639`/`:1643` bounds `REFRESH_AT` to 1 m-55 m
> against the **token's one-hour life**, and says so in its own error text; `Grace` is not a term in
> it. I grepped both parts: `Grace` appears in `task-15.md` once, in a comment at `:1243`, and
> `REFRESH_AT` appears in `task-07.md` not at all. The two values do not "live" in one place — `Grace`
> is keysmith's config and `REFRESH_AT` is the control plane's — so nothing in the phase compares
> them, and `REFRESH_AT=1m` with `Grace=5m` is accepted by every check the plan has, leaving up to
> five live generations per session. Dropping the guard was the right call (re-review b07-1, Minor 6:
> it compared two literals in its own file); the sentence that took its place should say the relation
> is **unenforced and why that is tolerable**, or task 15's parse should gain the bound, which is a
> seam because the other term is another process's config. One sentence either way, but as written a
> later reader is told a check exists that does not.

Ruling for the build, declared under rule 7 (root-cause option): say the relation is
**unenforced**, name the consequence (up to five live generations per session), and open the
bound as a seam rather than silently claiming a check. Adding the bound to task 15's parse
makes one process validate another process's config, which is the seam — do not do it
without recording it.

---

## m-1 (Minor, b03) — the named residual credits the network for a bound the local network does not give, and one command-running settings key is unnamed

Source: `.superpowers/sdd/p6/plan-rereview-b03-2.md:12-21`, finding `m-1`.
Your half is item 1 of the fix; item 2 is optional and belongs to task 23.

> **m-1. The named residual says the network bounds it, but locally the sandbox network has a route out. One command-running settings key is not named.**
> - Where: `task-14.md:537` says the agent's Bash tool "can `curl` anything the sandbox's network allows, which is the network's job (task 17's wall, P6-D1)".
> - Task 17's sandbox network `panacea-app-gateway` is a plain bridge (`task-17.md:257-258`, no `internal`). The wall only separates the backend. Locally, a sandbox reaches any internet host.
> - The same bound covers any repository-chosen command that the CLI runs by itself, with no model call. Example: a project `statusLine` command gets the session JSON, including `transcript_path`, on every refresh, and could post the transcript anywhere.
>   - Training knowledge, **UNVERIFIED**: `statusLine` exists in project settings, and it receives `transcript_path`.
>   - Whether `allowManagedHooksOnly` covers `statusLine` is also **UNVERIFIED**. The docs I remember tie the status line to `disableAllHooks`, not to `allowManagedHooksOnly`.
>   - This is the same class as R-1's hook route. It is not named in `task-14.md:537`, `task-25.md:84`, skeleton `:248` or S6.6 `:291`, which name only env keys, MCP and plugins.
> - Fix, about 15 minutes:
>   1. In `:537`, name the class: "any repository-chosen command, from the Bash tool or from a settings key such as `statusLine`". Say that its bound is sandbox egress, which is open locally and belongs to phase 8 / P8-3 in the cloud (`task-19.md:895`). Do not credit it to task 17's wall.
>   2. Optional: add `"statusLine": {...}` to item 10's hostile file, and add the door shape `POST /status-S4`. The result is then recorded, not assumed.

Item 1 also edits `task-25.md:84`, skeleton `:248` and S6.6 at skeleton `:291`. Commit those
by pathspec with your part.
