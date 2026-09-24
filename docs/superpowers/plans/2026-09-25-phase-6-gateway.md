# Phase 6: The Inference Gateway (LiteLLM)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task's full text is in `2026-09-25-phase-6-gateway-parts/task-NN.md`; this file is the skeleton those parts are written from.

**Goal:** Every agent, cloud or local, gets its inference from **one org-level gateway** and from nothing else. The gateway holds the org's Anthropic and Bedrock credentials; a sandbox holds only `ANTHROPIC_BASE_URL` and a **one-hour, pod-bound, per-session token**. Each project runs on one of two routes, both zero data retention: **Global** (the direct Anthropic API, the default) or **AU** (Amazon Bedrock, `ap-southeast-2`). The route follows the project's **data class**, set at project creation and updatable (OD-24), and a user may raise it to AU. Every call is attributed to a user, a team and a project, and each of the three has a budget that is **on by default**. The whole stack runs on Docker Desktop with zero cloud spend (local mode, OD-20), and the same charts run on AKS beside phase 8's workloads.

**Architecture:** Three processes of ours, one of LiteLLM's, one new seam in the sandbox.

```
sandbox pod (Kata)                        panacea-gateway namespace (no public ingress)
 ┌───────────────────────────┐           ┌───────────────────────────────────────────────┐
 │ claude (unmodified)       │  POST     │ gatekeeper (Go, ours)          :4000          │
 │  ANTHROPIC_BASE_URL ──────┼──────────►│  1 token → binding (hash, pod IP, session)    │
 │  apiKeyHelper =           │ /v1/      │  2 source IP == bound pod IP, or 403          │
 │   "panacea token"         │ messages  │  3 live project route (Hub, cached ≤30 s)     │
 │        │                  │           │  4 model → "global/<m>" or "au/<m>"           │
 │ sandboxd /v1/token ◄──────┼─ Secret ─┐│  5 path allowlist; strip x-litellm-* headers  │
 └───────────────────────────┘          ││         │ forward, key unchanged             │
                                        ││         ▼                                    │
 broker (phase 8) ── mint(session) ─────┼┼► keysmith (Go, ours)  ──► LiteLLM proxy      │
   workload identity, pod IP known      ││   master key holder       (pinned digest)    │
                                        │└── one-hour virtual key    budgets, spend logs │
                                        │                             │        │         │
 Hub (phase H) ◄── project dataClass, ──┘    usage rollup ◄───────────┘        │         │
   route, budgets (read, workload id)                          Postgres (gateway db)    │
                                                               └───────────────────────┘
                                       egress only to: api.anthropic.com (Global)
                                                       bedrock-runtime.ap-southeast-2 (AU)
```

- **LiteLLM** does what it is good at: upstream calls, the two model groups, virtual keys, native user and team budgets, spend logs. It is a pinned image digest with our `config.yaml`, never a fork, and **nothing outside the namespace can reach it**.
- **gatekeeper** is the only ingress. It exists because LiteLLM has no per-key source binding and no notion of a project route that tightens mid-key. It rewrites exactly one JSON field (`model`) and passes every other byte of the request through.
- **keysmith** is the only holder of the LiteLLM master key besides LiteLLM itself. It mints, refreshes and revokes session keys, records each key's **binding**, enforces the budget dimension LiteLLM lacks, and serves the usage rollup the cockpit reads.
- **sandboxd** gains one loopback route, `/v1/token`, and the sandbox image gains `panacea token`, which the product's managed settings name as `apiKeyHelper`. The token is never an environment variable.

The governing constraint is unchanged: **agent CLIs are never parsed, wrapped, reimplemented or interpreted** (`docs/PROGRAM.md` cross-cutting constraints, amended by OD-8). The gateway reads HTTP requests the CLI sends to an API, which is what any API endpoint does; it reads no PTY byte and no transcript.

**Tech Stack:** Go 1.23 (the `control-plane` module's pinned directive; `github.com/jackc/pgx/v5` for the binding store, version pinned in task 03), LiteLLM proxy (image pinned by digest in task 01, version **UNVERIFIED** until task 01 pulls it), Postgres 16 (compose locally; phase 8's Flexible Server in the cloud), Helm 3, Docker Compose v2.

**Spec:** OD-19, OD-20, OD-21, OD-23, OD-24 (`docs/superpowers/planning/2026-09-23-owner-decisions.md:140-152`); team-mode design §2.1 and §2.5 (`docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:127`, `:193-257`); phase 8 plan (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md`, P8-D4, P8-D11, P8-D12).

**Re-scope.** `docs/PROGRAM.md:113-114` still describes phase 6 as *"Capacity and credential rotation … per-user subscription registration … Never pooled across users"*. OD-19 supersedes that text; task 20 rewrites it. No consumer subscription is used anywhere in this plan.

---

## The decisions this plan rests on

### Owner decisions (they win over everything older)

| Id | What it fixes here | Where it lands |
|---|---|---|
| **OD-19** | Org-level inference through a gateway; no personal subscription; Global (direct Anthropic API, default) and AU (Bedrock `ap-southeast-2`), both ZDR; a scoped per-session token, never an org key; attribution and quotas per user, team, project; route per project, user-triggered | the whole plan |
| **OD-20** | Local mode stays supported; the cloud must track the latest Claude Code CLI | tasks 15–18, 23: the full stack on Docker Desktop. The gateway pins nothing about the CLI version: it forwards `anthropic-beta` verbatim (task 11) |
| **OD-21** | LiteLLM, reusing Panacea's patterns; no spike of the alternative | tasks 02, 04, 17, 18 |
| **OD-23** | The control plane supplies policy (managed settings) into every sandbox; mechanism, not policy | task 14 ships a settings **fragment**; OD-23's mechanism merges it (seam S6.6) |
| **OD-24** | Budgets on by default; tighten, never loosen; a data class set at project **creation**, updatable, driving AU vs Global and the deploy-side scans | tasks 01, 06, 13, 21, 22; asks H-1, H-2, P7-1 |
| **OD-5** | Our own environment, all in this monorepo | the gateway is ours, not Panacea's LiteLLM |
| **OD-8** | No meaning from PTY bytes; transcripts read-only through a pinned adapter | the gateway reads neither; it sees only the HTTP API calls the CLI makes |
| **OD-13** | No Panacea source copied yet | Panacea's LiteLLM code is read for its lessons (below) and nothing is copied |

### Plan-level decisions

| Id | Decision | Why |
|---|---|---|
| **P6-D1** | **LiteLLM behind our own front, never exposed.** `gatekeeper` is the only listener a sandbox can reach; LiteLLM listens only to `gatekeeper` and `keysmith` (NetworkPolicy, task 19; compose network, task 17) | LiteLLM has no per-key source binding and no project route that can tighten mid-key. Adding both as LiteLLM plugins would be Python inside a pinned third-party image; a 300-line Go front is testable in this repo's toolchain |
| **P6-D2** | **One Anthropic-format endpoint for both routes.** A sandbox always sets `ANTHROPIC_BASE_URL` to the gatekeeper and never sets `CLAUDE_CODE_USE_BEDROCK`. The gatekeeper chooses the upstream model group per request from the **live** project route | The docs: provider switching *"without touching developer machines … depends on the gateway exposing a single Anthropic-format endpoint regardless of upstream"* (https://code.claude.com/docs/en/llm-gateway, read 2026-09-25). A route raised to AU mid-session therefore takes effect on the next request |
| **P6-D3** | **AU sessions launch with `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1`**, and LiteLLM's AU group runs with `drop_params: true`. The Bedrock-format connection is a seam (S6.4), not built | The docs warn that in Anthropic format Claude Code sends *"Everything current Claude models accept on the Claude API, including adaptive reasoning, effort, and context management, which an Amazon Bedrock … upstream can reject"*, and that the variable *"stops Claude Code from sending pre-release capabilities"* but *"doesn't affect adaptive reasoning"* (https://code.claude.com/docs/en/llm-gateway-protocol, read 2026-09-25). Whether LiteLLM's Anthropic→Bedrock translation carries the rest is **UNVERIFIED** until task 24's live probe |
| **P6-D4** | **Tighten immediately, loosen at the next mint.** A project raised to AU is served AU from the next request, whatever route the key was minted on. A project lowered to Global keeps AU until each key is re-minted | OD-24's "tighten, never loosen" applied to time: no request of an AU project ever leaves on Global, including the ones in flight on an older key |
| **P6-D5** | **Fail closed toward refusal, not toward Global.** An unknown data class, an unreadable project route, or a binding miss is a `403`/`503` with a problem code. Global is the default only for a project whose class is known and permits it | "Default Global" must never mean "Global when we could not tell" |
| **P6-D6** | **The token is a LiteLLM virtual key**, minted by `keysmith` with `duration: "1h"`, `key_alias: "sess/<sessionId>/<generation>"`, `user_id` = the person, `team_id` = the **project**, `metadata` = `{tid, oid, team, project, agent, session, route}`, and a `max_budget` slice (P6-D8). The gatekeeper keeps the key unchanged and forwards it | Panacea already keys `team_id` to the project so spend totals by project (`Projects/Panacea/services/panacea-api/handlers/llm_env_keys.go:100-110`). One credential end to end; LiteLLM's own budgets bite on it |
| **P6-D7** | **The binding** (key hash → session, pod source address, project, route at mint, expiry, revoked) lives in keysmith's store, in the gateway database's own `keysmith` schema. The gatekeeper reads it, cached for at most 10 s, and a revocation is visible within that | The gatekeeper needs pod binding and revocation without calling LiteLLM on every request |
| **P6-D8** | **Budgets: three dimensions, on by default.** Person → LiteLLM user budget. Project → LiteLLM team budget. Team → enforced by keysmith at mint from the usage rollup. Each key's `max_budget` = `min(remaining person, remaining project, remaining team, per-session cap)`. Defaults come from the Hub's team policy (ask H-2), with non-zero fallbacks in config | 05-design §2.5: *"an unset quota is the org pool by another name"* (`05-design.md:232-236`). LiteLLM has no third tier we can rely on without its enterprise features (**UNVERIFIED** per image, task 18 probes) |
| **P6-D9** | **ZDR on our side too.** No LiteLLM logging callback is configured; prompt and response storage in spend logs is off; the gatekeeper and keysmith never log a request or response body; the path allowlist admits only `/v1/messages` and `/v1/messages/count_tokens` upstream, so no Files or Batches API call can store data | The upstreams' ZDR terms are the org's contracts, **UNVERIFIED** here (`05-design.md:204`); our own stores must not undo them |
| **P6-D10** | **Everything in Australia East**: LiteLLM, its Postgres, keysmith, gatekeeper | `05-design.md:228-231`. Global calls leave Australia by definition, which is what Global means |

### What Claude Code needs from a gateway, from its docs

Read 2026-09-25 at https://code.claude.com/docs/en/llm-gateway-protocol unless marked. Task 11 turns each line into a test.

| Fact from the docs | What this plan does |
|---|---|
| Anthropic format: `ANTHROPIC_BASE_URL`; endpoints `/v1/messages`, `/v1/messages/count_tokens` (optional); *"Inference requests post to `/v1/messages?beta=true`"*; *"Match on the path, not the full URL"* | The gatekeeper routes on `URL.Path` and passes the query through. `ANTHROPIC_BASE_URL` carries **no path** |
| *"Forward `anthropic-version` and `anthropic-beta` unchanged … don't allowlist individual values, because the set changes with Claude Code releases"* | Forwarded verbatim. This is also what keeps the cloud current with the CLI (OD-20) |
| The helper value is sent as `Authorization: Bearer …`, and *"When a helper value is the only credential, this header carries it too"* in `x-api-key` | The gatekeeper accepts either header; if both are present they must be equal, or `401` |
| Streams must not be buffered; Claude Code *"aborts a stream that goes silent for 300 seconds by default"*, and SSE `ping`s are *"the only traffic during long thinking pauses"* | Flush every write; no response deadline; idle-read timeout above 300 s |
| `GET /v1/models?limit=1000`, *"a timeout of 3 seconds by default, and any redirect is treated as failure"* | Served by the gatekeeper from the catalogue, locally, with no upstream call and no redirect |
| A `HEAD /api/hello` connection-warming probe, which the gateway *"can reject without breaking anything"* | Answered `204` locally |
| The fast-mode check and the WebFetch domain safety check call `api.anthropic.com` directly, not the base URL | Recorded for the AU egress allowlist (ask P8-3): they fail there, and that is accepted |
| `apiKeyHelper`, `CLAUDE_CODE_API_KEY_HELPER_TTL_MS`, `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`; a non-default base URL bypasses server-managed settings | Cited by `05-design.md:205-215`, `:237-250` (settings-reference and server-managed-settings, read 2026-09-25 there, **not re-read here**). Task 14 ships the fragment |
| Bedrock format: `ANTHROPIC_BEDROCK_BASE_URL` with `CLAUDE_CODE_USE_BEDROCK=1`, endpoints `/model/{model}/invoke…`, with provider model ids | Not used (P6-D2). Seam S6.4 names it, with `CLAUDE_CODE_SKIP_BEDROCK_AUTH=1` (that variable's name is **training knowledge, UNVERIFIED**) |

**Never set in a sandbox:** `ANTHROPIC_API_KEY` and `ANTHROPIC_AUTH_TOKEN` (an environment variable is inherited by every test suite and `postinstall`, `05-design.md:207-208`), `CLAUDE_CODE_USE_BEDROCK`, any `AWS_*` credential, any LiteLLM key other than the session's own.

### What Panacea's LiteLLM taught, and the rule each becomes

Read, not copied (OD-13). Line references verified 2026-09-25 in `~/Projects/Panacea`.

| Pitfall | Evidence | Rule here |
|---|---|---|
| **Base URL without the full path.** LiteLLM's Anthropic provider, in Panacea's build, posted to `api_base` verbatim, so a bare-host `api_base` answered `405` and every completion failed | Panacea's incident record of 2026-08-31 (owner memory, not in the repo tree, **UNVERIFIED** against source); Panacea pins LiteLLM `v1.63.11` (`kubernetes/helm-values/litellm-values.yaml:29-31`) | The generated config (task 02) **omits `api_base` on the Global route**. An override is accepted only if it ends in `/v1/messages`, and a test fails otherwise. Our own clients build URLs through one helper, `wire.MessagesURL` (task 03), never by concatenation |
| **The master key silently absent.** `LITELLM_MASTER_KEY` comes from a `secretKeyRef` with `optional: true`, so a missing Secret yields an empty key and a pod that still reports ready | `deploy/portal/deploy-portal.yaml:779-784` | The master key is a **file** mounted from Key Vault (CSI) or the local `.env`. keysmith refuses to start without it (task 08). A chart test fails on any `optional: true` secret reference (task 19) |
| **A slow mint drains someone else's deadline.** `/key/generate` ran inside a request's shared 15-second context, and the PipelineRun create after it failed with `context deadline exceeded` | `/key/generate` at `services/panacea-api/handlers/llm_env_keys.go:120`; the shared 15-second contexts at `handlers/projects.go:1891`, `:2622`; the chain between them from Panacea's incident record | Every LiteLLM admin call runs on **its own** deadline (task 04). Session start never waits on a mint: the mint runs beside sandbox start, and only `panacea token` waits for it (tasks 14, 15). A mint is idempotent on `key_alias`, so a retry after a timeout finds the first key instead of minting a second |

## Global constraints

1. **Modules and toolchain.** New Go code goes in the existing modules (`go.work:3-6`: `./control-plane`, `./sandboxd`). The `go 1.23` directive never moves. `sandboxd` vendors its dependencies (`sandboxd/vendor/`), so a new `sandboxd` import is vendored in the same commit. New `control-plane` dependencies are pinned in `require` lines; `pgx/v5` is the only one planned.
2. **No secret in the repository, no usable long-lived secret in the cloud.** Keys arrive as files (Key Vault CSI, P8-D11) or a gitignored local `.env`. The one exception is phase 8's to settle: LiteLLM's Postgres login and the Bedrock credential (asks P8-4, P8-5).
3. **Bodies are never logged.** No request or response body, and no header value except `anthropic-version`, reaches a log line, a metric label, or a problem body, in gatekeeper, keysmith or the generated LiteLLM config. A canary test (tasks 12, 18) proves it.
4. **Every error body is phase 2's problem shape** (`control-plane/internal/httpx/problem.go:93`, `WriteProblem`) with the codes task 03 fixes. No handler writes its own error JSON.
5. **Configuration** is `PANACEA_APP_GATEWAY_*`, parsed once per binary, with defaults, and a bad value fails start-up with the variable's name.
6. **Fail closed** (P6-D5). No code path turns "unknown" into Global.
7. **The CLI is untouched.** Claude Code runs unmodified; it is configured only through documented settings and environment variables (task 14). No wrapper, no patched binary, no PTY read.
8. **UI** follows `docs/DESIGN-SYSTEM.md`: budgets are words and `tabular-nums` figures, never a progress bar; one accent, for interactive state only.
9. **Clean room** (OD-11) does not bite here: nothing in this phase touches canvas or Linear material.

## File map

`cp/` is `control-plane/` (module `github.com/kpmg-au-emu/panacea-app/control-plane`, `control-plane/go.mod:1`).

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `cp/internal/dataclass/dataclass.go` | `Level`, `Residency`, `Class`, `Route`; `Decide`; `CanChange` (the ratchet); the Hub wire shape | new | 01 |
| `cp/internal/inference/catalogue/{catalogue.go,models.yaml}` | The model catalogue, embedded: public id → Global upstream id, AU Bedrock profile id; `ForRoute`; `ModelsResponse` for `/v1/models` | new | 02 |
| `cp/cmd/gatewaygen/main.go` | Renders LiteLLM `config.yaml` from the catalogue for `local` and `cloud`; never hand-edited | new | 02 |
| `cp/internal/inference/wire/wire.go` | Every wire shape between the pieces: mint, refresh, revoke, binding, usage, problem codes; `HashToken`; `MessagesURL`; the `BindingReader` and `RouteSource` interfaces | new | 03 |
| `cp/internal/litellmadmin/client.go` | LiteLLM's admin API, typed: keys, users, teams, spend. Own deadlines; master key from a file | new | 04 |
| `cp/internal/binding/{binding.go,memstore.go,pgstore.go,migrations/0001_keysmith.sql}` | The binding `Store`, in memory and in Postgres (`keysmith` schema) | new | 05 |
| `cp/internal/binding/bindingtest/suite.go` | The behaviour suite both stores pass (phase 2's `registrytest` shape) | new | 05 |
| `cp/internal/budget/budget.go` | Three dimensions, defaults on, `Slice` (the per-key cap), `Policy` source interface, static fallback | new | 06 |
| `cp/internal/keysmith/keysmith.go` | `Mint`, `Refresh`, `Revoke`, `RevokeSession`; idempotent on alias; budget pre-check | new | 07 |
| `cp/cmd/keysmith/{main.go,config.go,api.go,callerauth.go}` | keysmith's HTTP API, caller authentication, config, fail-closed start | new | 08 |
| `cp/internal/usage/usage.go` | Spend rolled up per person, project, team; the team-budget reader | new | 09 |
| `cp/internal/gatekeeper/authz/authz.go` | One request → allow, deny, route: credential, binding, source address, expiry, live route | new | 10 |
| `cp/internal/gatekeeper/proxy/{proxy.go,rewrite.go}` | Path allowlist, `model` rewrite, header hygiene, unbuffered streaming | new | 11 |
| `cp/cmd/gatekeeper/{main.go,config.go}` | Wiring, `/v1/models`, `HEAD /api/hello`, health, body-free logging | new | 12 |
| `cp/internal/routesource/{hub.go,file.go,cache.go}` | The live project route: from the Hub, from a local file, cached, failing closed | new | 13 |
| `sandboxd/cmd/panacea/main.go` | `panacea token`: prints the session token from the mounted token directory, waits for a slow mint | new | 14 |
| `images/sandbox/{Dockerfile,managed-settings.gateway.json}` | Installs `panacea`; the gateway settings fragment for OD-23's mechanism | modified, new | 14 |
| `cp/internal/gatewayclient/client.go` | The keysmith client every launcher uses: mint beside start, atomic token-file write, refresh, revoke | new | 15 |
| `cp/internal/backend/backend.go`, `cp/internal/backend/dockerbe/docker.go` | `Spec.Network` (docker only); join the gateway network | modified | 15 |
| `cp/internal/api/sessions.go`, `cp/cmd/api/config.go` | Local control plane: mint on create, revoke on delete, the gateway variables | modified | 15 |
| `cp/cmd/fakeanthropic/main.go` | A zero-spend Anthropic-format upstream for tests and local mode: SSE with pings, request capture, a route label | new | 16 |
| `deploy/local/gateway/{compose.yaml,init.sh,litellm.config.yaml,projects.json}` | Local mode: LiteLLM, Postgres, keysmith, gatekeeper, two fakes, one network; `.env` gitignored | new | 17 |
| `deploy/local/gateway/probe.sh`, `docs/verification/2026-09-25-phase-6-litellm-probe.md` | What the pinned LiteLLM actually does, measured | new | 18 |
| `deploy/charts/gateway/**` | Helm chart: three Deployments, Services, PDBs, NetworkPolicies, SecretProviderClass; no route | new | 19 |
| `infra/terraform/modules/bedrock-federation/**` | AWS side of the AU route: OIDC trust, one role, `au.` profiles in-region only. Written, never applied | new | 20 |
| `cp/internal/api/inference.go` | `GET`/`PATCH /v1/projects/{id}/inference` on the local control plane | new | 21 |
| `desktop/src/components/inference/*`, `desktop/src/api/client.ts`, `desktop/src/store/types.ts` | The cockpit's inference panel: class, route, raise to AU, spend against budget | new, modified | 22 |
| `docs/verification/2026-09-25-phase-6-local-e2e.md` | The local end-to-end proof with unmodified Claude Code, by hand | new | 23 |
| `infra/smoke/gateway/*.sh` | Cloud proving scripts, tested on canned output, **not run** | new | 24 |
| `docs/PROGRAM.md` | Phase 6 rewritten; open risk 2 closed; the never-pool line amended | modified | 25 |
