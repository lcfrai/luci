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
