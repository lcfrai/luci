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

Paths are in `Projects/Panacea` (read-only; grep only), as of 2026-09-30. `00-panacea.md` §5-7 holds the full inventory; this table judges each piece against N1-N10.

| Need | Panacea piece and entry point | Meets? | Gap for us | Verdict |
|---|---|---|---|---|
| N4 sign-in | Keycloak realm `panacea`, `https://auth.<domain>/realms/panacea`, Keycloak 25.0.0 (chart `codecentric/keycloakx` 2.4.4); Entra brokered as IdP `entra` (`deploy/portal/setup-keycloak.sh:395`), tenant-pinned (`:509`) | **Yes, with one realm change**: the public PKCE client `panacea-app-desktop`, its browser-flow override (K1), the four audience scopes `panacea-app-hub/-broker/-gateway/-fleet` and the service client, all already specified (`2026-09-25-identity-keycloak-broker.md` §3-6). No Entra change, no IT ticket | one replica with in-memory sessions (`deploy/main.tf:928`, `:954-957`), so a pod move signs everyone out; Keycloak 26 persists sessions by default ([26.7.3 notes](https://www.keycloak.org/2026/08/keycloak-2673-released)); every realm change is a Panacea deploy | **adopt now** |
| N6 gateway | LiteLLM `panacea-litellm/litellm-proxy:4000`; a virtual key per project (`services/panacea-api/handlers/llm_vkeys.go:209`, `:458`), model allow-list (`:79`), rate caps (`:263-340`); metered proxy `/ai/proxy/{project}/{mode}/v1/...` (`routes/routes.go:1763-1764`) | **No, for agents** | in-cluster only; spend attributed to the project alone (`handlers/deploy_analysis.go:30-31`), not the user or session; Bedrock removed (`handlers/ai_settings.go:218`), so no AU route; zero retention not asserted in config (no `turn_off_message_logging` under `deploy/` or `kubernetes/`); a sandbox would hold a long-lived project-wide token; the pinned version is ambiguous (v1.63.11 in Helm values, v1.40.0 in the custom image) | **integrate the code, not the service**: copy the key-minting, allow-list and rate-cap code into `keysmith` with attribution; Panacea's gateway stays the one for apps Panacea hosts |
| N1, N2, N5 the Hub's store | per-project Postgres (CloudNativePG 0.21.2) from `POST /api/v1/databases` (`routes.go:1325`); Data API with row-level security `P/rest/{table}` (`:2915-2918`); realtime `P/realtime` (`:2911-2912`); an audit row per call (`middleware.Audit`) | **Partly**: it is a Supabase-class backend for the apps Panacea hosts, and the Hub could be one of those apps | tenant roles are created `NOCREATEROLE` (`clients/db_roles_ensure.go:97-109`), so the Hub's `roles.sql` (`hub_owner`, `hub_app`) and its column grants need reworking; the Hub validates its own bearer tokens, which Panacea's posture engine sees as serve-without-SSO; client briefs would sit behind Panacea's production DLP gate (06-deploy call 1); our system of record would share Panacea's cutover risk | **reference**: the fallback host only if our subscription is late (§7) |
| N1 blobs | object storage: MinIO chart 5.2.0 behind `s3proxy` (`kubernetes/argocd-apps/data/minio.yaml:22-23`); Azure Blob in `deploy/object-storage-blob.tf` (315 lines: private endpoint, workload identity through a federated credential, `:248-310`) | **No** as a service (MinIO is AGPL-3.0 and its upstream is archived) | the Blob Terraform is the useful part | **copy the Blob snippet** |
| secrets | Vault, KV v2 at `secret/data/tenants/<project>/<env>` (`clients/vault.go:308-345`) | **No** | BUSL-1.1 (`OD-32`); Shamir-unsealed; reliability is on Panacea's own owner list | **reject**; Azure Key Vault now, OpenBao at the lift |
| portal | Backstage (Apache-2.0), OIDC to the realm (`backstage/app-config.production.yaml:36-47`), 8 scaffolder templates (`:103-139`) | not a store | a portal, not a transactional ledger | **reference**: deep links and templates (03-define, 06-deploy) |
| N7 environment | the live estate's Terraform, `deploy/*.tf`, 22 files: `postgres-flexible.tf` (475 lines: private endpoint only `:263`, zone-redundant HA `:268-270`, geo-redundant backup `:255-256`, customer-managed key `:285`), `firewall.tf` (272), `private-endpoints.tf` (96), `private-dns.tf` (80), `cmk.tf` (219), `aks-diagnostics.tf` (75), `defender-sentinel.tf` (113), `main.tf` (1,077) | **Yes, as code** | no Kata pool (01-sandboxes 4.15); the live estate is Australia Central, the hardened Australia East estate is mid-cutover; written for Terraform, but `.tf` runs on OpenTofu with the same MPL-2.0 azurerm provider (`OD-32`) | **copy with attribution** into phase 8's modules |
| observability | Prometheus, Grafana, Loki, Tempo (Argo CD apps under `kubernetes/argocd-apps/`) | not needed | Grafana, Loki and Tempo are AGPL-3.0 | **reject** for our estate; Azure Monitor (managed Prometheus, Container Insights) instead |

**Trade-off in one line:** Panacea meets sign-in today and hands us its best production Terraform and key-minting code; it cannot be the agents' gateway at all, and it can host the Hub only with a role rework, a posture exception and a shared failure domain.

## 3. Candidates

**How to read the effort figures.** Engineer-days of work still to do for this capability if the candidate is chosen (plan hours divided by 8, or our estimate where no plan exists), not elapsed time. Stars and releases are from the GitHub API on 2026-09-30 unless marked.

### 3A. The Hub's store, realtime and ledger

#### 3.1 Our Go Hub, finished, on Azure Database for PostgreSQL Flexible Server — **ADOPT (the substrate), keep the Hub**

- **Does:** the Hub as planned (ADR-H1): one Go binary (`cmd/hub`) over the schema already merged (`0001_init.sql`: row-level security per org through `current_setting('hub.org_id')`, column-level grants, no `UPDATE` or `DELETE` on the ledger tables, a `SECURITY DEFINER` hash-chained `audit_append`), on a managed Postgres.
- **Licence:** ours; `pgx` MIT; the database is an Azure service.
- **Deploy:** Flexible Server in Australia East, private endpoint only, zone-redundant HA and geo-redundant backup to the paired region, as Panacea's `postgres-flexible.tf:255-285` already does; the Hub as one Deployment on our AKS beside the control plane.
- **Enterprise fit:** Entra authentication, custom roles and row-level security are supported ([Azure docs](https://learn.microsoft.com/en-us/azure/postgresql/security/security-access-control)); the schema needs no superuser and no extension (`roles.sql` creates two `NOLOGIN` roles); an AU-only estate by construction; goes into a customer's subscription unchanged. Burstable B2ms is US$0.208 an hour (about US$152 a month) at list (Azure Retail Prices API, 2026-09-30).
- **Maturity:** Microsoft-run. The Hub itself: tasks 01-07b merged, 24.5 of 133.5 plan-hours.
- **Gaps:** the server routes (tasks 08, 13-27) and the client (28-42) are still to write: about **80 plan-hours** to the minimum Hub (S7), after 3.12 removes 5.
- **Effort:** **about 10 engineer-days**; the plan's critical path with two engineers is about 5 working days (lane A, tasks 08 and 13-24, 41 hours).
- **Why it wins:** the Hub's value is its domain rules (digest-checked revisions, the pin, the chain), and those are ours under every option; the store underneath is the part a product can take, and Azure's managed Postgres takes it with HA, point-in-time restore and Entra built in.

#### 3.2 Supabase, self-hosted on our AKS — **REFERENCE (not the Hub)**

- **Does:** Postgres plus Auth (GoTrue), PostgREST, Realtime, Storage, Edge Functions and Studio, behind Kong.
- **Licence:** Apache-2.0 (`supabase/supabase`, 110,907 stars, pushed 2026-09-30); the community Kubernetes chart is Apache-2.0 (`supabase-community/supabase-kubernetes`, 818 stars, `supabase-0.8.0` of 2026-09-05).
- **Deploy:** "The fastest and recommended way to self-host Supabase is to use Docker"; Kubernetes is community-maintained, not supported ([self-hosting docs](https://supabase.com/docs/guides/self-hosting)).
- **Enterprise fit:** weak where we need it. Self-hosted, "branching, advanced metrics beyond logs, managed backups and PITR ... and the platform management API are unavailable"; it is "a single project", and support is community-only (same page). Its services read one **static** `JWT_JWKS` from the environment and the self-hosted docs describe no external issuer ([self-hosted auth keys](https://supabase.com/docs/guides/self-hosting/self-hosted-auth-keys)), so trusting Keycloak means pasting the realm's public keys in and re-pasting them at every key rotation, plus a `role` claim mapper. Storage speaks S3 or a file system; Azure Blob needs a gateway in between.
- **Gaps against the Hub:** our row-level security keys on a setting the server sets, not on JWT claims; the digest re-check and canonical JSON (`hubapi.Canonical`, golden vectors) would move into PL/pgSQL or Deno; `hubclient` would be rewritten against PostgREST's error shape; about twelve more containers to run and patch.
- **Effort:** about **13 engineer-days** to reach the same minimum (our estimate: roughly 21 plan-hours removed, 46 added for hardening, Keycloak trust, backups, the storage gateway and the RLS port), against 10 for 3.1.
- **Trade-off in one line:** Supabase gives CRUD, realtime and storage for free, but the Hub is not CRUD; we would trade 21 hours of Go for 46 hours of glue and a dozen services.

#### 3.3 Supabase Cloud (Sydney) — **REJECT**

- Managed Supabase in `ap-southeast-2` on AWS (03-define 4.12). Not Azure (`OD-18`), a new processor holding client briefs, and it cannot go into a customer's own cloud (`OD-5`). Same Hub-shape gaps as 3.2.

#### 3.4 The Hub hosted as a Panacea project — **REFERENCE (the fallback host)**

- **Does:** `POST /api/v1/projects` with `enableDatabase: true` gives the Hub a namespace, TLS, DNS, a CloudNativePG database with backups and point-in-time revert, scans, rollback and logs (`00-panacea.md` §3, §7).
- **Gaps:** the tenant role is `NOCREATEROLE` (`clients/db_roles_ensure.go:97-109`), so `hub_owner` and `hub_app` map onto Panacea's migrate and runtime roles with reworked grants; Panacea's runtime-role grants cover `public` only (`clients/db_roles_ops.go:113-124`), so the `hub` schema keeps its own, which needs checking; a serve-without-SSO posture exception; our control plane's egress IP on Panacea's edge allow-list; client briefs behind the production DLP gate.
- **Effort:** about 1.5 engineer-days of hosting glue, against about 1.25 for 3.1 on an AKS we stand up anyway (01-sandboxes). The Hub's own 10 days are the same under both.
- **Trade-off in one line:** it saves the wait for a subscription, not work; it costs a shared failure domain for our system of record during Panacea's cutover.

#### 3.5 PostgREST over the built schema — **REFERENCE**

- MIT, 27,691 stars, v16.4 of 2026-09-24. A REST API from a Postgres schema, JWT-authenticated. The thin sibling of 3.2: no Realtime, Storage or Studio to run. The same gaps as 3.2 for the Hub's rules (they become SQL functions), plus no rate limiting or Origin checks of its own. About 11-12 engineer-days. Useful later for read-only reporting over the ledger.

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
