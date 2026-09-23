# Phase H (Hub) plan — adversarial review

**Reviewed:** `docs/superpowers/plans/2026-09-23-phase-h-hub.md` (484 lines) and its 44 parts in
`2026-09-23-phase-h-hub-parts/`, on `feat/hub` at `2f84662`, 2026-09-24.
**Reviewer's method:** every file, line, type and route this review cites was opened in this
worktree (or, where stated, in the main tree / a sibling worktree, read-only). Where a claim is
not checked, it says so. Nothing was compiled or run.

_Review in progress — sections below are appended as they are written._

## Working notes (folded into Issues below)

- Rotation write is not atomic: `PutChunked` overwrites chunks in place (task-28:100-104), so a
  failure mid-rotation mixes old and new chunks (task-31:39-42 claims the old token survives).
- Attach stays exempt from the local key (task-37:53-58) — the one route that types into a PTY.
- devissuer's page CSP `form-action 'self'` (task-12:49-52) vs a 302 to another loopback port.
- Sibling asks now exist (HA4-1..9, HA5-1..5, HA7-1..5) and several are unmet by task 04/07.
- Task 13's test needs devissuer (task 11) but 13 depends on 07, 08, 10 only.
- Cross-org FKs: single-column FKs let an org-A row reference an org-B parent (task-07).
- Invite binding keys on the `email` claim, else `preferred_username` (task-13:35).
