# 07 Platform: shared state, identity, AI gateway, environment

**Status: IN PROGRESS (relaunch; sections fill in one at a time).** Research of 2026-09-30, under the owner's two directions of that day ("third-party first, ASAP" and "Panacea the strong default, the best fully-fledged solution wins; Coolify plus Supabase allowed"). The first run (stopped 09:47) left only this skeleton, so every section below is written under the second direction.

**Inputs reused, not repeated:** `00-panacea.md` (the Panacea inventory, sections 5-7: Keycloak, LiteLLM, Backstage, Vault, Postgres, storage), `2026-09-28-oss-adoption/00-summary.md` and its `05-hub-identity.md` / `06-platform.md` (the library-level picks: go-oidc, goose, merkle, in-toto, OpenTofu, External Secrets, the LiteLLM `enterprise/` strip and the AU `count_tokens` canary).

Scope: the Hub (shared briefs, approvals, provenance ledger, releases, realtime), identity (Entra via Keycloak, decided `OD-30`), the AI gateway (LiteLLM, decided `OD-19`/`OD-21`) and the environment (infrastructure as code on Azure).

## 0. The answer in one screen

_To be written._

## 1. What we need, and what the plans budget for it

| # | Need | Source |
|---|---|---|
| N1 | **Shared records, server-side**: orgs, users, teams, projects and their git remotes, brief revisions (immutable, digest-checked), design approvals and snapshots, session links, releases with their pins, evidence | `OD-3`; Hub contract §7 |
| N2 | **A tamper-evident ledger**: append-only, hash-chained, and doubling as the change feed; no request body names an actor | `OD-3`; ADR-H1 grafts G3, G5; Hub plan rule 2 |
| N3 | **Near-realtime**: a teammate's approval shows within seconds | Hub tasks 26 and 36 (a 5-second poll) |
| N4 | **Sign-in**: Entra through a Keycloak broker, auth-code + PKCE, zero clicks, refresh token in Windows Credential Manager, every call made as the user | `OD-6`, `OD-30` |
| N5 | **Org isolation and audit**: row-level security per org, an audit row per write | ADR-H1 §5.3; contract §10 |
| N6 | **AI gateway**: org keys held centrally; a scoped per-session token; Global (Anthropic API) or AU (Bedrock Sydney) by data class; zero retention; spend per user, team and project; budgets on by default | `OD-19`, `OD-21`, `OD-24`, `OD-33` |
| N7 | **Environment**: Azure, Australia East, our subscription first and a customer's cloud later; AKS (the Kata pool is 01-sandboxes'), Postgres, secrets, registry, egress control, observability; infrastructure as code on OpenTofu | `OD-5`, `OD-18`, `OD-32` |
| N8 | **Licences**: permissive only in anything we ship; no BUSL (so OpenTofu, not Terraform; OpenBao, not Vault) | `OD-11`, `OD-32` |
| N9 | **Local-first**: the whole stack runs on a laptop in Docker | Hub plan goal; `OD-20` |
| N10 | **Days, not weeks** | owner, 2026-09-30 |

**What the plans budget.** Hours are each plan's own implementation estimate, summed by script from its task table on 2026-09-30; none is a measurement.

| Plan | Tasks | Built today | Estimate | What it builds |
|---|---|---|---|---|
| Phase H, the Hub (`.claude/worktrees/hub/.../2026-09-23-phase-h-hub.md`) | 50 part files | stream reports 6 of 50; `feat/hub`'s log shows tasks 01 to 07b merged: `hubapi`, `hubclient` with its fake and conformance suite, the migrate runner, `0001_init.sql` (693 lines: tables, row-level security, column grants, guards, a `SECURITY DEFINER` hash-chained `audit_append`), about 22.5 hours | **133.5 engineer-hours** (ADR-H1: 16 engineer-days) | a Go server (`cmd/hub`) on Postgres 16; `devissuer`, a local OIDC issuer; `signin` (PKCE against Keycloak); `credstore` (Credential Manager); `hubclient` over HTTP; two desktop surfaces |
| Phase 6, the gateway (`.../2026-09-25-phase-6-gateway.md`) | 29 | 0 | **8.85 plan-days** (about 71 hours) | LiteLLM rebuilt from the MIT tree; `keysmith` (mints one-hour per-session keys, budgets); `gatekeeper` (the only ingress: pod binding, live route, `model` rewrite, the `count_tokens` refusal); `panacea token`; chart; Bedrock federation; cockpit panel |
| Phase 8, the environment (`.../2026-09-23-phase-8-own-environment.md`) | 37 part files (the stream counts 35) | stream reports 5; the tree holds tasks 01 to 04 (layout, state bootstrap, network, observability), 09 (the realm file), 11 (the platform chart) and 16 (`internal/entra`), some in part | **98.5 plan-hours**: about **53** are this capability's (01-11, 13, 16, 17, 27-31, 33) and about 45.5 the sandbox cluster's (12, 14, 15, 18-26, 32; 01-sandboxes) | OpenTofu modules and stacks, AKS with a Kata pool, Flexible Server for the Hub, the Keycloak realm as code, charts, pipelines, runbooks |

**Two corrections to the brief's framing.** Phase 8's column is hours, so its 35 tasks are about 12 working days, not 35 (01-sandboxes found the same). And the earlier study already rejected `go-oidc` and `goose` for the Hub (`OD-35`), so nothing here proposes them again.

## 2. What Panacea already meets, with entry points

_To be written._

## 3. Candidates

_To be written._

## 4. Side by side

_To be written._

## 5. Recommendation

_To be written._

## 6. Glue we still build

_To be written._

## 7. Owner calls

_To be written._

## 8. Revised for the owner's second direction

_To be written._

## 9. What was not verified

_To be written._

## Sources

_To be written._
