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
| `deploy/local/gateway/{compose.yaml,init.sh,compose_test.sh,litellm.config.yaml,projects.json}`, `.gitignore` | Local mode: LiteLLM, Postgres, keysmith, gatekeeper, two fakes, one network; `secrets/` gitignored | new, modified | 17 |
| `images/gateway/Dockerfile` | One distroless image carrying `keysmith`, `gatekeeper`, `fakeanthropic` | new | 17 |
| `deploy/local/gateway/probe.sh`, `docs/verification/2026-09-25-phase-6-litellm-probe.md` | What the pinned LiteLLM actually does, measured | new | 18 |
| `deploy/charts/gateway/**` | Helm chart: three Deployments, Services, PDBs, NetworkPolicies, SecretProviderClass; no route | new | 19 |
| `infra/terraform/modules/bedrock-federation/**` | AWS side of the AU route: OIDC trust, one role, `au.` profiles in-region only. Written, never applied | new | 20 |
| `cp/internal/api/inference.go` | `GET`/`PATCH /v1/projects/{id}/inference` on the local control plane | new | 21 |
| `desktop/src/components/inference/*`, `desktop/src/api/client.ts`, `desktop/src/store/types.ts` | The cockpit's inference panel: class, route, raise to AU, spend against budget | new, modified | 22 |
| `docs/verification/2026-09-25-phase-6-local-e2e.md` | The local end-to-end proof with unmodified Claude Code, by hand | new | 23 |
| `infra/smoke/gateway/*.sh` | Cloud proving scripts, tested on canned output, **not run** | new | 24 |
| `docs/PROGRAM.md` | Phase 6 rewritten; open risk 2 closed; the never-pool line amended | modified | 25 |

## Task list

Slices: **A** the contract (pure Go, no I/O) · **B** keysmith · **C** gatekeeper · **D** the sandbox side and launch · **E** local mode · **F** cloud hosting · **G** product surfaces · **H** proof and close. A focused day is 8 hours; every task is 1–3 hours. Seam and ask ids are defined in "Seams and asks" below.

| Id | Title | Slice | Focused days | Depends on | Seams |
|---|---|---|---|---|---|
| 01 | Data class, route and the ratchet | A | 0.25 (2 h) | — | S6.1, S6.7, S6.8 |
| 02 | Model catalogue and the generated LiteLLM config | A | 0.3 (2.5 h) | — | — |
| 03 | Wire contracts, problem codes, `MessagesURL`, `HashToken` | A | 0.25 (2 h) | 01 | S6.3, S6.5 |
| 04 | LiteLLM admin client on its own deadlines | B | 0.3 (2.5 h) | 03 | — |
| 05 | Binding store: memory, Postgres, behaviour suite | B | 0.4 (3 h) | 03 | P8-4 |
| 06 | Budgets: three dimensions, defaults on, the per-key slice | B | 0.25 (2 h) | 01, 03 | S6.2 |
| 07 | keysmith core: mint, refresh, revoke | B | 0.4 (3 h) | 04, 05, 06 | — |
| 08 | `cmd/keysmith`: API, caller authentication, config, fail-closed start | B | 0.4 (3 h) | 07 | S6.3, S6.5 |
| 09 | Usage rollup and the team-budget reader | B | 0.25 (2 h) | 04, 06 | S6.2 |
| 10 | gatekeeper authorisation | C | 0.3 (2.5 h) | 03, 05 | — |
| 11 | gatekeeper proxy: allowlist, `model` rewrite, headers, streaming | C | 0.4 (3 h) | 02, 10 | S6.4 |
| 12 | `cmd/gatekeeper`: wiring, `/v1/models`, `HEAD /api/hello`, body-free logs | C | 0.25 (2 h) | 11, 13 | — |
| 13 | Project route source: Hub, file, cache, fail closed | C | 0.3 (2.5 h) | 01, 03 | S6.1 |
| 14 | `panacea token` and the managed-settings fragment | D | 0.25 (2 h) | 03 | S6.6 |
| 15 | `gatewayclient` and the local control plane's launch | D | 0.4 (3 h) | 08, 14 | S6.3 |
| 16 | Fake Anthropic upstream | E | 0.25 (2 h) | — | — |
| 17 | Local stack on Docker Desktop | E | 0.3 (2.5 h) | 02, 08, 12, 16 | — |
| 18 | LiteLLM capability, ZDR and pitfall probe | E | 0.3 (2.5 h) | 17 | — |
| 19 | Helm chart, NetworkPolicies, Key Vault CSI | F | 0.4 (3 h) | 02, 08, 12 | P8-1 to P8-5 |
| 20 | Bedrock federation module (AWS), never applied | F | 0.3 (2.5 h) | 02 | P8-5 |
| 21 | Control-plane inference API | G | 0.3 (2.5 h) | 01, 09, 13, 15 | S6.1, S6.2 |
| 22 | Cockpit inference panel | G | 0.4 (3 h) | 21 | S6.8 |
| 23 | Local end-to-end proof with unmodified Claude Code | H | 0.25 (2 h) | 15, 18, 22 | — |
| 24 | Cloud proving scripts, written and not run | H | 0.3 (2.5 h) | 19, 20 | S6.4, P8-2 |
| 25 | Close the phase: `PROGRAM.md`, seams, asks, what stays unverified | H | 0.2 (1.5 h) | all | all |

**Total: 61 focused hours, about 7.6 focused days.** The critical path is 01 → 03 → 05 → 07 → 08 → 15 → 21 → 22 → 23 → 25 (25 h).

**Waves for parallel execution** (a task starts when everything it depends on is merged):

1. 01, 02, 16
2. 03, 20
3. 04, 05, 06, 13, 14
4. 07, 09, 10
5. 08, 11
6. 12, 15, 19
7. 17, 21
8. 18, 22, 24
9. 23
10. 25

**Shared names every part uses**, so parallel writers agree without reading each other:

- Binaries: `keysmith` (listens `:4100`), `gatekeeper` (listens `:4000`), `fakeanthropic` (listens `:4900`, a second instance on `:4901` labelled `au` in local mode), `gatewaygen`, `panacea`.
- LiteLLM listens on `:4001`, reached only as `http://litellm:4001` (compose) or `http://litellm.panacea-gateway.svc.cluster.local:4001` (cloud).
- Model groups in LiteLLM are named `global/<public id>` and `au/<public id>`. A sandbox only ever names the public id.
- The token directory in a sandbox is `/run/panacea/gateway/`, holding one file, `token`. The directory is mounted, never the file, so an atomic rename on the host is seen inside (a bind-mounted file keeps its old inode).
- Postgres: database `gateway`; LiteLLM owns schema `public` of it (its own migrations), keysmith owns schema `keysmith`.
- Environment variables: `PANACEA_APP_GATEWAY_` + `KEYSMITH_URL`, `BASE_URL`, `LITELLM_URL`, `MASTER_KEY_FILE`, `DATABASE_URL_FILE`, `ROUTE_SOURCE` (`hub` or `file:<path>`), `HUB_URL`, `CALLER_AUTH` (`kube` or `bearer-file:<path>`), `TOKEN_TTL` (default `1h`), `REFRESH_AT` (default `45m`), `MINT_TIMEOUT` (default `20s`), `BINDING_CACHE_TTL` (default `10s`), `ROUTE_CACHE_TTL` (default `30s`), `ROUTE_STALE_MAX` (default `10m`), `SESSION_CAP_USD` (default `25`), `DEFAULT_{USER,TEAM,PROJECT}_BUDGET_USD` (defaults `200`, `2000`, `1000` a month, **proposed**, owner question Q2).

## Task scopes

Each paragraph is what the part file `2026-09-25-phase-6-gateway-parts/task-NN.md` must make complete: the files, every interface in full, the tests first with the mutant each kills, the steps, the acceptance commands with expected output, and what the task does not claim.

### Slice A — the contract

**01 — Data class, route and the ratchet.** Files: `control-plane/internal/dataclass/{dataclass.go,dataclass_test.go}`; no I/O, no dependency outside the standard library. Types: `Level` (`public`, `internal`, `confidential`, `restricted` — a **proposed** list until the owner names KPMG's scheme, Q1, so the list is one table the rest of the code never switches on), `Residency` (`any`, `au-only`), `Class{Level, Residency}` with JSON keys `level` and `residency`, `Route` (`global`, `au`, ordered so `au` is the tighter). Functions: `(Class) Validate() error` (the zero value is invalid, never "public"); `Floor(Class) Route` (`au-only` → `au`, else `global`); `Decide(c Class, chosen Route) (Route, error)` = the tighter of `Floor(c)` and `chosen`, with `ErrUnknownClass` and `ErrUnknownRoute` for anything unparsed; `Tighter(a, b Route) Route`; `Actor{Member, Compliance bool}`; `CanChange(c Class, from, to Route, a Actor) error` returning `ErrNeedsCompliance` for any lowering by a non-compliance actor and `ErrBelowFloor` for a lowering under `Floor(c)` by anyone. It also fixes the **Hub wire shape** of ask H-1, `ProjectInferenceFields{DataClass Class; InferenceRoute Route; RouteChangedBy, RouteChangedAt}`, with `DisallowUnknownFields` decoding. Tests, each with its mutant: `au-only` plus chosen `global` gives `au` (kills `Decide` returning `chosen`); an unknown level errors (kills a `default: return RouteGlobal`); a zero `Class` fails `Validate` (kills treating empty as public); a member lowering is refused (kills a dropped role check); a compliance actor lowering an `au-only` project is refused (kills a missing floor check); raising needs only `Member`. Does not claim: that the level list is KPMG's; how the Hub authorises a class change (ask H-1); which scans a class requires (phase 7, ask P7-1).

**02 — Model catalogue and the generated LiteLLM config.** Files: `control-plane/internal/inference/catalogue/{catalogue.go,models.yaml,catalogue_test.go}` (the YAML embedded with `go:embed`), `control-plane/cmd/gatewaygen/{main.go,main_test.go}`, and the two generated outputs `deploy/local/gateway/litellm.config.yaml` and `deploy/charts/gateway/files/litellm.config.yaml`. Each `models.yaml` entry: public id (the Anthropic id Claude Code sends, e.g. `claude-opus-4-8`, the docs' own example), display name, Global upstream (`anthropic/<id>`, no `api_base` in cloud), AU upstream (`bedrock/au.anthropic.<id>…`, region `ap-southeast-2`, `drop_params: true`). Every concrete Bedrock profile id is **UNVERIFIED** and marked so in the YAML until task 24 lists the account's profiles. API: `Load() (*Catalogue, error)`, `(*Catalogue) Resolve(publicID string, r dataclass.Route) (group string, ok bool)` returning `global/<id>` or `au/<id>`, `Public() []Model`, `ModelsResponse() []byte` in Anthropic's `GET /v1/models` list shape (`data`, `has_more`, `first_id`, `last_id`; the exact field set is from training knowledge, **UNVERIFIED**, and task 23 watches the CLI's picker). `gatewaygen -target local|cloud -out <file>` renders `model_list` (both groups per model), `general_settings` (`master_key: os.environ/LITELLM_MASTER_KEY`, `database_url: os.environ/DATABASE_URL`), and the ZDR settings (`store_prompts_in_spend_logs: false`, `turn_off_message_logging: true`, no `callbacks`; key names **UNVERIFIED** until task 18). In `local` the groups point at `fakeanthropic-global:4900` and `fakeanthropic-au:4901` with an `api_base` whose path form is one generator flag that task 18 sets. Tests: every AU entry is `bedrock/au.` in `ap-southeast-2` (kills accepting `apac.` or `global.` profiles, 05-design's CI rule); a Global `api_base`, if present, ends in `/v1/messages` (kills the Panacea bare-host 405); public ids unique; each checked-in config equals the generator's output byte for byte (kills a hand edit); the rendered YAML contains no `callbacks` key and no literal secret. Does not claim: that the model ids are current, or that LiteLLM honours every rendered key.

**03 — Wire contracts, problem codes, `MessagesURL`, `HashToken`.** Files: `control-plane/internal/inference/wire/{wire.go,wire_test.go}`; depends on 01 only. Complete Go source for: `Owner{TID, OID}`; `MintRequest{SessionID, Generation (≥1), Owner, TeamID, ProjectID, AgentID, SessionClass ("solo"|"worker"|"orchestrator"), PodAddr (netip.Addr as a string)}` with `Validate()`; `MintResponse{Token, KeyID, ExpiresAt, Route, BaseURL, MaxBudgetUSD}`; `RefreshRequest{SessionID, Generation}`; `RevokeRequest{SessionID}`; `Binding{KeyHash, KeyID, SessionID, Generation, Owner, TeamID, ProjectID, AgentID, PodAddr netip.Addr, RouteAtMint, CreatedAt, ExpiresAt, RevokedAt *time.Time}`; `UsageRow{Scope ("person"|"project"|"team"), ID, PeriodStart, SpendUSD, BudgetUSD, Requests}`; `ProjectInference{ProjectID, DataClass, Route, Effective, Version}`; the interfaces `BindingReader{ByKeyHash(ctx, hash) (Binding, error)}` and `RouteSource{Project(ctx, id) (ProjectInference, error)}`; sentinels `ErrNotFound`, `ErrRouteUnknown`; the problem codes as constants (`unauthenticated`, `credential_conflict`, `token_expired`, `token_revoked`, `binding_mismatch`, `route_unknown`, `budget_exhausted`, `model_not_offered`, `path_not_allowed`, `mint_timeout`, `caller_forbidden`, `needs_compliance`) with the HTTP status each maps to; `HashToken(token) string` (hex SHA-256); `MessagesURL(base) (string, error)`, which accepts a base with no path or `/` and **refuses** one that already ends in `/v1/messages` or carries a query. Tests: JSON keys golden for every type (kills a renamed field breaking a parallel part); `MessagesURL` table (kills double-pathing and the bare-host case); `HashToken` output never contains the input (kills an identity hash); `Validate` rejects generation 0, an empty project, an unparsable `PodAddr`. Does not claim: transport or authentication, which are tasks 08 and 12.

### Slice B — keysmith

**04 — LiteLLM admin client on its own deadlines.** Files: `control-plane/internal/litellmadmin/{client.go,client_test.go}`; depends on 03. `New(baseURL, masterKeyFile string, o Options{Timeout (default 20 s), HTTP *http.Client}) (*Client, error)` reads the key file once, trims it, and returns `ErrNoMasterKey` when it is empty or lacks LiteLLM's `sk-` prefix. Methods, request and response structs complete: `GenerateKey(ctx, GenerateKeyRequest{Key (keysmith-chosen value), KeyAlias, Duration, UserID, TeamID, Models, MaxBudget, Metadata})`, `DeleteKeys(ctx, keys)`, `KeyInfo(ctx, key) (KeyInfo{Spend, Expires, …})`, `EnsureUser(ctx, id, maxBudget, budgetDuration)`, `EnsureTeam(ctx, id, maxBudget, budgetDuration)`, `Health(ctx)`. Every call runs on `context.WithTimeout(context.WithoutCancel(ctx), o.Timeout)`: a caller that gives up does not abort a mint half-way, and no caller's deadline is ever shared with it (the Panacea lesson). Errors carry the status and LiteLLM's error type, **never the response body** (it can echo a key). Endpoint paths and field names follow LiteLLM's documented admin API, **UNVERIFIED** against the pinned image until task 18. Tests against an `httptest` fake: `Authorization: Bearer <master>` on every call; a caller context of 1 ms still lets a 50 ms reply complete (kills using the caller's context); a hung server fails after `Timeout` (kills no deadline); an empty key file fails `New` (kills `optional: true` behaviour); an error string contains neither the master key nor the body (kills body echo); request JSON golden per method. Does not claim: idempotency, which is task 07's.

**05 — Binding store: memory, Postgres, behaviour suite.** Files: `control-plane/internal/binding/{binding.go,memstore.go,pgstore.go,migrate.go,migrations/0001_keysmith.sql}`, `control-plane/internal/binding/bindingtest/suite.go`, and `memstore_test.go`, `pgstore_test.go`; depends on 03; adds `github.com/jackc/pgx/v5` (pinned) to `control-plane/go.mod`. `Store` = `wire.BindingReader` plus `Put(ctx, Binding) error` (`ErrExists` on a duplicate key hash **or** a duplicate `(session, generation)`), `BySession(ctx, id) ([]Binding, error)`, `Revoke(ctx, keyHash, at)`, `RevokeSession(ctx, id, at) (int, error)`, `DeleteExpired(ctx, before) (int, error)`. The migration creates schema `keysmith`, table `bindings` (`key_hash text primary key`, `unique (session_id, generation)`, `pod_addr inet not null`, `route_at_mint text check (route_at_mint in ('global','au'))`, timestamps), and `keysmith.schema_version`; `Migrate(ctx, pool)` is idempotent. `bindingtest.Run(t, func(t) binding.Store)` is the suite both stores pass, in the shape of phase 2's `registrytest` (`docs/superpowers/plans/2026-09-18-phase-2-backend-and-cockpit.md:159`). The Postgres run needs `PANACEA_APP_GATEWAY_TEST_DATABASE_URL` (task 17's compose Postgres) and skips with that variable's name otherwise. Tests: duplicate `(session, generation)` refused (kills losing the idempotency backbone); `RevokeSession` revokes every generation (kills revoking only the newest); a revocation survives a new store over the same database (kills an in-memory-only revoke); IPv6 `PodAddr` round-trips; a missing hash is `wire.ErrNotFound`. Does not claim: the cloud database's login, which is ask P8-4.

**06 — Budgets: three dimensions, defaults on, the per-key slice.** Files: `control-plane/internal/budget/{budget.go,budget_test.go}`; depends on 01, 03. Types: `Dimension` (`person`, `project`, `team`), `Limits{PersonUSD, ProjectUSD, TeamUSD, SessionCapUSD, Period ("monthly", UTC calendar month)}`, `Spent{PersonUSD, ProjectUSD, TeamUSD}`, `Policy{Limits(ctx, teamID, projectID) (Limits, error)}`, `Static{Default Limits}` whose constructor refuses any zero or negative field, `Layered{Primary Policy; Fallback Static}` where a zero from the primary falls back to the default and **never** means unlimited, and `Slice(l, s) (maxUSD float64, exhausted Dimension, ok bool)` = the least of each dimension's remainder and the session cap, `ok=false` naming the first dimension at or below zero. The Hub-backed `Policy` is ask H-2 (seam S6.2) and not built here. Tests: each dimension, exhausted alone, is the one named (kills a fixed dimension); the minimum is chosen (kills `max`); a zero from the primary yields the non-zero default (kills "an unset quota is the org pool by another name", `05-design.md:233-234`); `Static` refuses zero (kills budgets off by default). Does not claim: a hard ceiling. N concurrent sessions can each spend a full slice, so the overrun bound is N × the slice, and the part states it.

**07 — keysmith core: mint, refresh, revoke.** Files: `control-plane/internal/keysmith/{keysmith.go,keysmith_test.go}`; depends on 04, 05, 06. `Deps{Admin AdminAPI (the subset of task 04's methods, as an interface); Store binding.Store; Budgets budget.Policy; Spent SpentReader{Spent(ctx, owner, teamID, projectID) (budget.Spent, error)}; Routes wire.RouteSource; Clock func() time.Time; Rand io.Reader; TTL, Grace time.Duration (1 h, 5 min); BaseURL string}`, `New(Deps) (*Service, error)`, `Mint`, `Refresh`, `RevokeSession`. Mint, in this order: validate; live route through `Routes` and `dataclass.Decide` (`route_unknown` → no further call); limits and spend → `budget.Slice` (`budget_exhausted` naming the dimension → no key); `EnsureUser` (`<tid>:<oid>`) and `EnsureTeam` (the project) with their budgets, remembered for 10 minutes; generate the token itself (`sk-pa-` + 32 bytes of `Rand`, base64url); `Store.Put` the binding; `GenerateKey` with `Key` = that token, `KeyAlias` = `sess/<session>/<generation>`, `Duration` = TTL, `Models` = every `global/*` and `au/*` group, `MaxBudget` = the slice, `Metadata` = tid, oid, team, project, agent, session, route. **At most one live key per session generation**: a retried mint of a generation that already has a live binding revokes that binding and deletes its LiteLLM key before minting again, since the caller never received the first token. If `GenerateKey` fails after `Put`, the binding is revoked before returning. `Refresh` mints generation + 1 and revokes the previous one after `Grace`, so a stream in flight is not cut. Whether LiteLLM accepts a caller-chosen `key` is **UNVERIFIED** until task 18; the fallback, reading the generated key from the reply and binding after the call, is written into the part. Tests with fakes: unknown route makes no LiteLLM call (kills check-after-call); exhausted budget mints nothing; a retried generation leaves exactly one live binding and one LiteLLM delete (kills orphaned budgeted keys); a failed `GenerateKey` leaves no live binding; the old generation survives until `Grace` and not after (kills immediate revoke and never-revoke); no error string contains the token. Does not claim: transport or caller identity (task 08).

**08 — `cmd/keysmith`: API, caller authentication, config, fail-closed start.** Files: `control-plane/cmd/keysmith/{main.go,config.go,api.go,callerauth.go}` and a test per file; depends on 07. Routes, all through `httpx.WriteProblem` and `httpx.DecodeJSON` (`control-plane/internal/httpx/problem.go:93`, `:125`): `POST /v1/mint` (201 `MintResponse`), `POST /v1/refresh` (201), `POST /v1/revoke` (204), `GET /v1/bindings/{keyHash}` (200 `Binding`, for the gatekeeper, so the gatekeeper holds no database login), `GET /v1/usage?scope=&id=` (task 09), `GET /healthz`, `GET /readyz` (database ping and LiteLLM health). A mint that runs past `MINT_TIMEOUT` answers `504 mint_timeout` while the mint itself completes (task 04's detached context), and the caller's retry of the same generation is safe (task 07). Caller authentication, `PANACEA_APP_GATEWAY_CALLER_AUTH`: `kube` validates a projected service-account token with audience `keysmith` through a `TokenReview` (`k8s.io/client-go`, already a phase 2 dependency) and maps the service account to its routes: broker → mint, refresh, revoke; gatekeeper → bindings; Fleet → mint, refresh, revoke, **off** until seam S6.5; `bearer-file:<path>` holds `name token routes` lines for local mode. Config parses every variable in "Shared names" with defaults; a missing or empty master-key file, or an unreadable database URL file, exits non-zero naming the variable. Tests: every route refuses a caller not listed for it with `403 caller_forbidden` (kills checking identity but not the route); start-up without a master key exits 1 with `PANACEA_APP_GATEWAY_MASTER_KEY_FILE` in stderr (kills the Panacea silent-empty key); the 504 path leaves the mint to finish; the config table. Does not claim: the broker's side of the call (ask P8-1).

**09 — Usage rollup and the team-budget reader.** Files: `control-plane/internal/usage/{usage.go,sweep.go,usage_test.go}` and migration `control-plane/internal/binding/migrations/0002_key_spend.sql` (task 05's `Migrate` runs the ordered list); depends on 04, 06. LiteLLM's native budgets cover the person and the project; the team is ours. A sweep every 60 s reads each live or recently expired key's spend through `KeyInfo` and upserts `keysmith.key_spend(key_hash, owner, team_id, project_id, period_start, spend_usd, requests)`, so a key LiteLLM has deleted keeps its last recorded spend. `Reader.Spent(ctx, owner, teamID, projectID) (budget.Spent, error)` sums the current UTC month per dimension (cached 60 s) and implements task 07's `SpentReader`; `Reader.Rows(ctx, scope, id) ([]wire.UsageRow, error)` serves `GET /v1/usage`. Relying on LiteLLM's tag reports instead is rejected because those features may be enterprise-only in the pinned image (**UNVERIFIED**, task 18). Tests: spend sums across a session's generations; a key gone from LiteLLM keeps its spend (kills zeroing on delete); the month boundary in UTC; the team total spans projects (kills a team read that is really a project read). Does not claim: accuracy finer than 60 s, which also bounds how far a team can overshoot between sweeps; the part states the bound.

### Slice C — gatekeeper

**10 — gatekeeper authorisation.** Files: `control-plane/internal/gatekeeper/authz/{authz.go,authz_test.go}`; depends on 03, 05 (the suite's memory store as the fake). `Request{Authorization, XAPIKey string; Source netip.Addr; Now time.Time}`, `Decision{Allow bool; Status int; Code string; Binding wire.Binding; Route dataclass.Route; Token string}`, `Authorizer{Bindings wire.BindingReader; Routes wire.RouteSource}` with `Decide(ctx, Request) Decision`. Order: the credential from `Authorization: Bearer` or `x-api-key`, both present and different → `401 credential_conflict`, neither → `401 unauthenticated`; hash, lookup, `401` on a miss; `401 token_revoked`, `401 token_expired`; `Source` not equal to `Binding.PodAddr` → `403 binding_mismatch`; the live route, failure → `503 route_unknown`; **`Route = dataclass.Tighter(live effective route, Binding.RouteAtMint)`** (P6-D4). `Source` comes from `r.RemoteAddr` only. Tests, one per branch, each with its mutant, plus: a project raised to AU after mint is served AU (kills using the route at mint); a project lowered after an AU mint stays AU (kills using the live route alone); `X-Forwarded-For` is ignored (kills trusting a header a sandbox can set). Does not claim: that AKS preserves the pod's source address to a service, which is `05-design.md:212-214`'s open item and task 24's probe P03; the mTLS fallback is seam S6.10.

**11 — gatekeeper proxy: allowlist, `model` rewrite, headers, streaming.** Files: `control-plane/internal/gatekeeper/proxy/{proxy.go,rewrite.go,proxy_test.go}`; depends on 02, 10. `New(Config{Upstream *url.URL; Catalogue *catalogue.Catalogue; Authz Decider; MaxBody int64 (64 MiB)}) http.Handler`. Only `POST /v1/messages` and `POST /v1/messages/count_tokens` are forwarded, matched on `URL.Path` with the query kept (`?beta=true`); anything else is `404 path_not_allowed` (task 12 adds `/v1/models` and `/api/hello` in front). The rewrite decodes the body as `map[string]json.RawMessage`, resolves `model` through `Catalogue.Resolve(model, decision.Route)` (`400 model_not_offered` if absent), replaces that one value, and re-encodes; every other value's bytes are unchanged. Headers: `anthropic-version` and `anthropic-beta` verbatim; every `x-litellm-*` and `litellm-*` header dropped; the credential forwarded as `Authorization: Bearer` alone. Transport: `httputil.ReverseProxy` with `FlushInterval: -1`, no response deadline. Tests: an SSE upstream that pauses between events delivers the first event before the second is sent (kills buffering); `anthropic-beta: some-future-beta-2027` arrives upstream unchanged (kills an allowlist, which the docs forbid); `/v1/files` and `/v1/messages/batches` are 404 (kills a prefix match that would reach storing APIs, P6-D9); `?beta=true` survives; an AU decision sends `au/<id>` (kills ignoring the route); a body with an unknown top-level field keeps it byte for byte (kills struct re-encoding); `x-litellm-tags` never reaches upstream (kills spoofed attribution). Does not claim: whether LiteLLM also reads body-level tags; task 18's P08 decides, and if it does, this rewrite drops `metadata.tags` as its one other edit.

**12 — `cmd/gatekeeper`: wiring, `/v1/models`, `HEAD /api/hello`, body-free logs.** Files: `control-plane/cmd/gatekeeper/{main.go,config.go,bindings.go,main_test.go}`; depends on 11, 13. The binding reader is an HTTP client to keysmith's `GET /v1/bindings/{hash}` with a positive cache of `BINDING_CACHE_TTL` and a negative cache of 2 s, authenticated as the gatekeeper (projected token or local bearer). `GET /v1/models` is answered from `Catalogue.ModelsResponse()` for a request that passes authorisation, with no upstream call and no redirect; `HEAD /api/hello` is `204` with no credential needed; `/healthz`, `/readyz`. The server sets `ReadHeaderTimeout` 10 s, `IdleTimeout` 120 s and **no `WriteTimeout`**. Logs are `slog` JSON with `session`, `project`, `route`, `path`, `status`, `ms` and nothing else. Tests: a request whose prompt holds a canary string, run through the full handler against a fake LiteLLM that fails with a 500 echoing the body, leaves the canary in no log line and no problem body (kills logging bodies on error); `/v1/models` needs a valid binding and never redirects; the server's `WriteTimeout` is zero (kills cutting a five-minute stream); config table. Does not claim: TLS on the private path; phase 8's network is the boundary, and mTLS is seam S6.10.
