# Adversarial review — Phase 4 Discover plan (2026-09-23)

**Reviewed:** `docs/superpowers/plans/2026-09-23-phase-4-discover.md` and its 23 parts, branch
`feat/phase-4-discover`, head `0d82e7a`. The reviewer is read-only except for this file.

**How it was checked.** Every phase 2 citation below was opened in this worktree (base `3303fd6`).
The Hub contract was read at `.claude/worktrees/hub` HEAD `6a6bc31`;
`git log a6ccd77..HEAD` shows no change to the contract or ADR-H1 since the commit the plan read,
so the plan's Hub citations are current. Nothing was compiled or run by the reviewer either.

*Working notes, filled in as the review proceeds. The final sections follow below.*

## Draft findings (to be ordered)

- restart: Materialise position vs `Backend.Destroy` (`sessions.go:944-951`) unspecified.
- Materialiser required in task 10, but `cmd/api/main.go:122`, `attach_test.go:303`,
  `sessions_test.go:575` construct `Deps` directly and are not in task 10's file list.
- desktop `DEFAULT_TIMEOUT_MS = 10_000` (`client.ts:33`) vs hubclient 10 s x 3 attempts, and
  modelclient 120 s: the 409 and the model routes can arrive as `timeout`.
- fan-out skips branch briefs whose head is open: their latest complete revision keeps pinning
  the old project revision (4-D3 gap).
