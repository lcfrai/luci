# Phase 6 gateway plan: adversarial review

Reviewer: plan-review agent, 2026-09-25. Read-only on the plan; this file is the only write.
Scope: `2026-09-25-phase-6-gateway.md` (the skeleton, 331 lines) and `2026-09-25-phase-6-gateway-parts/task-01.md` … `task-25.md` (20,481 lines), read one part at a time in line ranges.

Severity: **Critical** = an implementer following the text builds something insecure or that cannot work; **Important** = a real defect or contradiction that costs rework; **Minor** = wording, citation or sizing.

## 1. Citations checked against the real tree

| Citation | Result |
|---|---|
| `go.work:3-6` (`./control-plane`, `./sandboxd`), `go 1.23` | verified |
| `control-plane/go.mod:1` module path; `k8s.io/client-go v0.31.1` already required | verified (`go.mod:10`) |
| `control-plane/internal/httpx/problem.go:93` `WriteProblem`, `:125` `DecodeJSON` | verified |
| `control-plane/internal/backend/backend.go:168-172` Kata returns `ErrUnsupported` for `Mounts` | verified (comment at `:168-172`) |
| `control-plane/internal/api/api.go:218-264` the route table | verified (table comment `:218`, `HandleFunc` `:264`) |
| `docs/superpowers/planning/2026-09-23-owner-decisions.md:140-152` OD-19 … OD-24 | verified |
| `docs/PROGRAM.md:113-114` phase 6, `:132` never-pool, `:138` open risk 2 | verified |
| `05-design.md:193-257` §2.5 | verified; the plan follows it except where §3 says |
| Hub contract `:576-578` routes 14, 15, 16; `:614-615` null rule; `:618-619` `User` | verified, **but route 16 (`PATCH /v1/projects/{projectId}`) is role `A`, not `M`**: see I-3 |
| phase 8 plan `:141` P8-D12, `:330` precondition, `:452` Entra-only Postgres, `:464` proving dir (task 32), `:469` netpolcover | verified in `.claude/worktrees/env` at `9ef70c3` |
| phase 8 task 32 "the Kata proving checklist" (`:621`) | exists; the Secret-volume and source-address probes are **asks** into it, not yet in its text |

## 2. Skeleton findings

**I-1 (Important) Wave 6 runs task 19 beside its own dependency.** The task table gives 19 `Depends on 02, 08, 12`; the wave list puts 12 and 19 both in wave 6 (skeleton `:195`). Rule stated at `:188`: a task starts when everything it depends on is merged. Fix: move 19 to wave 7 (it then still precedes 24 in wave 8).

**I-2 (Important) The ratchet is universal; the source design keeps a solo project's choice.** `05-design.md` §2.5: *"A solo project keeps the user's choice. On a team project the route is project-wide … only an org-level compliance role may lower it."* Task 01's `CanChange` refuses every member lowering, solo or team, and task 21 passes the local user as a member, so a solo developer who pressed *Keep inference in Australia* can never go back without a compliance role that local mode does not have (ask H-4). OD-19 says only that the user can trigger AU. Either take the design's rule (`Actor` gains `Solo bool`; a solo project's owner may lower to the floor) or put the universal ratchet to the owner as a question. Today it is a silent change to the design.

**I-3 (Important) Ask H-1 asks a member to raise through an admin route.** H-1 says route 16 accepts `inferenceRoute` and enforces *"raise by any member"*; route 16's role is `A` (`hub-contract.md:578`). As written the Hub refuses a member's raise before task 01's rule runs. H-1 must ask for either a member-role inference route (`PATCH /v1/projects/{projectId}/inference`, the same shape task 21 serves locally) or a field-level role exception on route 16, and say which.

**M-1 (Minor) The skeleton contradicts task 07 on idempotency.** Pitfall row 3 (`:102`) says *"A mint is idempotent on `key_alias`, so a retry after a timeout finds the first key"*. Task 07 decision (a) does the opposite, deliberately: a retry revokes the first key and mints again, and task 07 `:1113` says nothing relies on alias uniqueness. Rewrite `:102` to task 07's rule. Likewise task 05's scope still says plain `unique (session_id, generation)`; the part correctly uses the partial index `where revoked_at is null`.

**M-2 (Minor) Attribution ids do not match the Hub's.** keysmith keys a person as `<tid>:<oid>` (task 07); the Hub's principal is `User.id` = `u-…` (`hub-contract.md:618`). The cockpit's per-person usage (task 22) and H-2's per-person budget need one mapping, owned by someone. Name it in H-2.


