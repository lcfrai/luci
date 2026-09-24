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
