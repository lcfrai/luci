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
- **Maturity:** Microsoft-run. The Hub itself: tasks 44a, 00 and 01-07b done, about 24.5 of its 133.5 plan-hours.
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

#### 3.6 Realtime: keep the 5-second feed; NOTIFY, Electric and NATS later — **REFERENCE**

- **What is planned already meets N3:** the Hub's change feed (task 26) and the control plane's poller (task 36), 4 plan-hours, deliver a teammate's approval within 5 seconds.
- **The cheap upgrade:** Postgres `LISTEN`/`NOTIFY` pushed out as server-sent events from the Hub; `pgx` supports it; no new service. About half a day, when a surface needs sub-second.
- **Electric** (Apache-2.0, 10,380 stars, released 2026-09-09): syncs "shapes" of Postgres tables to clients over HTTP, authorised by a proxy or a gatekeeper token ([auth guide](https://electric-sql.com/docs/guides/auth)); needs logical replication. It serves Phase W's client replica (the earlier study's TanStack DB pick), not the Hub now.
- **NATS JetStream** (Apache-2.0, 20,810 stars): Phase T's bus, integrated when TM1-α is built (05-work-team).
- **Supabase Realtime:** only if the Hub moved to Supabase, which 3.2 rejects.

#### 3.7 Temporal — **REJECT (not needed)**

MIT, 23,372 stars. A durable workflow engine. The Hub's only multi-step process is a release's reserve, then confirm or abandon, with a janitor (tasks 23-24, 6 plan-hours): a state machine in one table. Temporal would add a cluster of services and its own database to run and secure. Revisit only if team mode's managed sessions need durable orchestration.

#### 3.8 A witness for the ledger: Azure confidential ledger — **REFERENCE (later)**; immudb — **REJECT**

- **Why a witness:** the hash chain lets anyone detect an edited row, but an administrator who rewrites the whole chain is caught only by a copy of the head kept somewhere the Hub cannot write.
- **Azure confidential ledger:** Microsoft-run, append-only, in hardware trusted execution environments; data replicates to the paired region, which for Australia East is Australia Southeast ([data residency](https://learn.microsoft.com/en-us/azure/confidential-ledger/data-residency)). Priced per ledger instance-hour; the pricing page publishes no figure (read 2026-09-30). About 1 day: a timer that appends the audit head, and a `hub verify` step that checks it.
- **Cheaper equivalent:** the same head written to an Azure Blob container under a time-based immutability (WORM) policy. About half a day.
- **immudb** relicensed to BUSL-1.1 (`2026-09-28-oss-adoption/05-hub-identity.md` §5): restricted.
- **Not now:** 06-deploy's signed in-toto statement (G2, Key Vault key) already gives each release an external proof.

#### 3.9 The git host as the Hub — **REJECT**

Briefs as files, approvals as pull-request reviews, releases as tags. Two hosts to implement twice (`OD-4`), no record before a repository exists (Define comes first), and it reverses ADR-H1 after four sibling phases coded to `hubclient`.

### 3B. Identity (Entra through Keycloak, decided)

#### 3.10 Panacea's Keycloak realm — **ADOPT now**

- **Does:** exactly `OD-30`: Entra brokered through Keycloak, zero clicks. **Licence:** Apache-2.0 (`keycloak/keycloak`, 37,059 stars, 26.7.4 of 2026-09-16). **Deploy:** Panacea's, `https://auth.<domain>/realms/panacea`.
- **Enterprise fit:** Panacea's existing Entra registration, tenant-pinned, auto-link, group mappers; no new Entra registration and no IT ticket (ruling §1).
- **Gaps:** the realm change (the client, K1-K4, four audience scopes, the `panacea-app-fleet` service client) is an ask of the Panacea stream, about half a day of theirs; Keycloak 25, one replica, sessions in memory, so a restart sends every user back through the browser; realm changes ride Panacea's deploys.
- **Effort on our side:** none beyond the planned client code (Hub tasks 30-32, 10.5 plan-hours).
- **Trade-off in one line:** it exists today; our sign-in shares Panacea's availability until 3.11.

#### 3.11 Our own Keycloak, by the Keycloak Operator — **LATER (reference now)**

- The operator's realm import applies the realm file the environment stream already wrote (`deploy/keycloak/realm/panacea-app.json`, task 09); its database on our Flexible Server; two replicas with Keycloak 26's persistent sessions.
- **Catch:** a Keycloak on our own hostname needs its broker endpoint added as a redirect URI on the Entra registration, which is the IT ticket `OD-30` avoided (inferred from the ruling's §1; not asked of IT).
- **When:** the customer-hosted tier (the customer's Entra, its realm), or sooner if the Panacea ask stalls. About 2 engineer-days; it replaces the Keycloak half of phase 8 task 29.

#### 3.12 A real Keycloak in the local stack instead of `devissuer` — **ADOPT**

- Hub tasks 11-12 (6 plan-hours) build `devissuer`, a local issuer that imitates Keycloak's layout. Keycloak runs in Docker in dev mode with `--import-realm`, from a dev copy of the same realm file; tests mint tokens through a test-only client in that dev realm.
- **Gain:** every test runs against the real issuer (discovery, key rotation, refresh rotation, audience mappers) instead of an imitation of it, and a security-sensitive piece of test code is never written. **Cost:** about 1 hour of compose and a test helper; a start of seconds. Net: about **5 plan-hours saved**.
- **Gap:** `tid` and `oid` for local users come from hard-coded attribute mappers in the dev realm, exactly as `devissuer` would fake them.

#### 3.13 Other identity servers — **REJECT (`OD-30` decided)**

Zitadel is AGPL-3.0 (Glasses, 2026-09-30): restricted. Authentik is MIT with an `authentik/enterprise/` carve-out (earlier study). Ory Hydra and Kratos and Dex are Apache-2.0 and clean, but replacing Keycloak reverses `OD-30` and the ruling's token profile. Entra directly stays the customer-hosted option (Panacea's BYO-Entra blueprint, `apothecary/docs/02-tenancy-and-identity.md`).

### 3C. The AI gateway (LiteLLM, decided)

#### 3.14 LiteLLM, ours, rebuilt from the MIT tree, started from Panacea's key code — **ADOPT (as planned)**

- **Does:** the endpoint every Claude Code session talks to: virtual keys with a lifetime, user and team ids and budgets; spend logs; the Global and AU model groups; the Anthropic `/v1/messages` format out to the Anthropic API or to Bedrock.
- **Licence:** MIT at the root; `enterprise/` under the BerriAI Enterprise License, so GitHub reports `NOASSERTION` (59,884 stars; v1.103.1 of 2026-09-30, while the plan pins v1.102.1; it releases about weekly, so pin by digest, `OD-33`).
- **Deploy:** our AKS in Australia East, the plan's `panacea-gateway` namespace, its database on our Flexible Server; `gatekeeper` and `keysmith` in front and beside it (phase 6 plan, P6-D1).
- **Enterprise fit:** meets `OD-19` with the plan's two services. The features the plan builds itself are the ones LiteLLM keeps for its paid tier: "IP address-based access control lists", "JWT-based Authentication", "Audit Logs", SSO beyond 5 users, tag-based budgets, team-managed models, key rotation, secret managers ([enterprise docs](https://docs.litellm.ai/docs/enterprise), read 2026-09-30).
- **Gaps:** the AU leak through `count_tokens` (task 26) and the `enterprise/` directory in the stock image (task 27), both `OD-33`.
- **What Panacea adds (snippets, attribution in the header):** `generateLiteLLMKey` and `EnsureProjectLLMKey` (`handlers/llm_vkeys.go:209`, `:458`), the model allow-list (`:79`), rate caps (`:263-340`) and `team_id` = project (`handlers/llm_env_keys.go:100-110`) seed keysmith's tasks 04 and 07 (5.5 plan-hours) from working code; the plan's pitfall table (phase 6 plan §"What Panacea's LiteLLM taught") stays the checklist.
- **Effort:** phase 6's **8.85 plan-days**, less about 0.3 for the copied code, less 0.4 if task 22's panel becomes an Orca panel (02's call): **about 8.2 engineer-days**.

#### 3.15 Panacea's LiteLLM as the agents' gateway — **REJECT (take its code)**

In-cluster only, spend by project alone, no Bedrock and so no AU route, retention not asserted in config, and a long-lived project token inside an untrusted sandbox (§2). It stays the gateway for apps Panacea hosts.

#### 3.16 A LiteLLM Enterprise licence — **BUY LATER (an option)**

- **Adds:** admin-UI SSO beyond 5 users, audit logs, key rotation, Azure Key Vault as a secret manager, tag budgets, team-managed models, global IP lists.
- **Does not add:** a key bound to one pod, the `count_tokens` fix, or the person-project-team budget as specified; so `gatekeeper` and `keysmith` stay either way.
- **Price basis:** "Pricing depends on your deployment size" (LiteLLM docs); third-party guides report Basic from about US$250 a month and Premium about US$30,000 a year ([TrueFoundry](https://www.truefoundry.com/blog/litellm-pricing-guide); not first-party).
- **When:** more than 5 operators need LiteLLM's own admin UI, or an assessor wants gateway-side audit logs beyond ours. Taking it puts `enterprise/` code under BerriAI's terms in a customer's tenancy; the per-customer terms are unverified.

#### 3.17 Other gateways — **REFERENCE (`OD-21` locked, no spike)**

| Gateway | Licence and deploy | Note |
|---|---|---|
| Azure API Management AI gateway | Azure service, Australia East | v2 tiers pass the Anthropic Messages API through and meter it with `llm-token-limit` ([policy reference](https://learn.microsoft.com/en-us/azure/api-management/llm-token-limit-policy)); per-session keys, spend by person, team and project, and Bedrock's SigV4 would all be ours (the last not verified) |
| Bifrost (`maximhq/bifrost`) | Apache-2.0, 8,451 stars (Glasses) | the vendor claims virtual keys, four-level budgets and Bedrock routing for Claude Code in the open tier, with clustering, guardrails, RBAC, vault and audit logs paid (vendor pages, not verified). The closest alternative on paper |
| Portkey gateway | MIT, 13,112 stars | routing is open; budgets and virtual keys live in Portkey's commercial control plane |
| Envoy AI Gateway | Apache-2.0 | Gateway API native, token rate limits, Bedrock with SigV4; no spend store |
| Claude in Microsoft Foundry | Azure-billed, Anthropic-run | not an AU route today: Australia East is not a supported region, per third-party reports of July 2026 (not verified first-hand); `OD-19`'s AU stays Bedrock Sydney |

### 3D. The environment (infrastructure as code on Azure)

#### 3.18 Panacea's production Terraform, copied into our OpenTofu modules — **INTEGRATE**

- **Does:** the KPMG-hardened estate code of §2: Postgres Flexible Server (private endpoint, zone-redundant HA, geo-redundant backup, customer-managed key), Blob with workload identity, Azure Firewall, private endpoints and DNS, CMK, AKS diagnostics, Defender and Sentinel.
- **Licence:** KPMG's own code; copying is allowed now with attribution in each file's header (`OD-13` relaxed); the `azurerm` provider stays MPL-2.0 and unmodified (`OD-32`).
- **Deploy:** under OpenTofu 1.12.6, the environment stream's pin (`148a21d1` on the env branch); `.tf` syntax carries over (`OD-32`).
- **Enterprise fit:** it has already been hardened against the ISM (the `.tf` files cite controls such as ISM-0459 37 times, ISM-1416 and ISM-1277 12 times each; grep, 2026-09-30); Australian regions; no BUSL.
- **Gaps:** no Kata pool (01-sandboxes adds it); Panacea's variable and resource-group model differs from ours; drop what we do not run (the ADO self-hosted agent, the elastic and build pools, the WAF application gateway). Panacea's dev and prod estates once collided on a shared AKS name, so every copied name is re-derived from our own estate variables.
- **Effort:** the environment stream has built the network and observability modules (tasks 03, 04). What remains in group A is tasks 05-08 (registry and Key Vault, AKS, the foundation stack with firewall and DNS, Postgres: 12 plan-hours); starting from Panacea's files halves them (our estimate): **about 6 plan-hours saved**.

#### 3.19 Azure Verified Modules — **REFERENCE**

- MIT, Microsoft-maintained modules for AKS, Postgres, Key Vault and networking (for example `Azure/terraform-azurerm-avm-res-dbforpostgresql-flexibleserver`, 8 stars, pushed 2026-09-21).
- **OpenTofu:** AVM "does not have the bandwidth to prioritize OpenTofu support" and relies on OpenTofu's compatibility promise ([AVM discussion #1512](https://github.com/Azure/Azure-Verified-Modules/discussions/1512)); the AKS module declares Terraform `>= 1.11` (registry page). Each module also brings the `azapi` and a telemetry provider.
- **Why not:** Panacea's code is already written against the plain `azurerm` provider and already hardened for KPMG; AVM would add a wrapper layer on an engine its maintainers do not test.

#### 3.20 Coolify, with Supabase, as our environment — **REJECT**

- Apache-2.0, 62,409 stars, v4.3.23 of 2026-09-18. Docker on our servers over SSH; no Kubernetes target, so no Kata (01-sandboxes 4.16); Azure DevOps is not a listed git source (06-deploy 3.1).
- It could stand the Hub, Supabase, LiteLLM and a Keycloak up on one Azure VM in an afternoon. But the sandboxes need AKS regardless, so it is a second platform to run, and the system of record would sit on one VM with backups we script ourselves.
- **Trade-off in one line:** the fastest demo host, and the wrong home for a system of record beside an AKS we must run anyway.

#### 3.21 Azure's managed services for the rest of the substrate — **ADOPT**

- **Key Vault:** secrets, the LiteLLM master key as a mounted file (phase 6 P6-D11 via the CSI driver) and the release-signing key (`azurekms://`, 06-deploy G2). OpenBao replaces it only at the Panacea lift (`OD-32`).
- **Blob:** the Hub's `blobstore` keeps blobs in Postgres first (task 21) behind an interface built for Azure Blob (ADR-H1 §5.6); the Blob implementation, with immutability for evidence and design snapshots, is about half a day when sizes demand it.
- **Azure Monitor:** Log Analytics with a daily cap and Container Insights (task 04, built), managed Prometheus; no AGPL Grafana stack of our own.
- **Registry:** ACR Premium (task 05; 01-sandboxes' images).

## 4. Side by side

Days are engineer-days still to do (section 3's rule). "Entra" means sign-in through our Keycloak broker (`OD-30`).

| # | Candidate | For | Licence | Deploy, residency | Entra | Maturity | Days | Verdict |
|---|---|---|---|---|---|---|---|---|
| 3.1 | Our Go Hub on Azure PG Flexible Server | Hub | ours; Azure service | our AKS + managed PG, Australia East, HA and geo backup inside Australia | validates realm tokens (`oidcauth`) | Azure GA; Hub 24.5 of 133.5 h done | **~10** | **adopt** |
| 3.2 | Supabase, self-hosted | Hub store | Apache-2.0 | Docker official, Kubernetes community (chart 0.8.0, 818 stars) | only by pasting the realm's keys into a static JWKS | 110,907 stars; no PITR, no branching, one project, community support self-hosted | ~13 | reference |
| 3.3 | Supabase Cloud | Hub store | commercial | AWS Sydney | third-party auth | GA | ~13 | reject |
| 3.4 | Hub hosted as a Panacea project | Hub host | KPMG | Panacea's AKS (Australia Central; Australia East in cutover) | Panacea's realm | Panacea API v1.163.0 | ~1.5 (+10) | reference (fallback) |
| 3.5 | PostgREST over the schema | Hub API | MIT | our AKS | JWT, static key or JWKS | 27,691 stars, v16.4 | ~11-12 | reference |
| 3.6 | NOTIFY, Electric, NATS | realtime | ours; Apache-2.0; Apache-2.0 | our AKS | through the Hub or a proxy | Electric 10,380 stars; NATS 20,810 | 0.5-3 | reference (later) |
| 3.7 | Temporal | workflows | MIT | our AKS | n/a | 23,372 stars | n/a | reject |
| 3.8 | Azure confidential ledger | ledger witness | Azure service | Australia East, pair Australia Southeast | Entra | GA | ~1 | reference (later) |
| 3.10 | Panacea's Keycloak realm | sign-in | Apache-2.0 | Panacea's | brokers Entra | Keycloak 25.0.0 there; 26.7.4 upstream | **0 ours** (+0.5 Panacea's) | **adopt** |
| 3.11 | Our own Keycloak (Operator) | sign-in, later | Apache-2.0 | our AKS | brokers Entra; needs a redirect URI (IT) | 37,059 stars | ~2 | later |
| 3.12 | Keycloak dev container | test issuer | Apache-2.0 | laptop Docker | n/a | as above | saves ~0.6 | **adopt** |
| 3.14 | LiteLLM, ours, MIT tree, Panacea's key code | gateway | MIT (`enterprise/` stripped) | our AKS, Australia East; Global = Anthropic API, AU = Bedrock Sydney | callers present realm tokens (`keysmith`) | 59,884 stars, v1.103.1 | **~8.2** | **adopt** |
| 3.15 | Panacea's LiteLLM | gateway | MIT | Panacea in-cluster | none for agents | v1.63.11 or v1.40.0 | n/a | reject (take its code) |
| 3.16 | LiteLLM Enterprise | gateway add-on | commercial | ours | admin-UI SSO | contact sales | ~0.5 | buy later |
| 3.17 | APIM AI gateway, Bifrost, Portkey, Envoy AI Gateway | gateway | Azure; Apache-2.0; MIT; Apache-2.0 | various | various | see 3.17 | n/a | reference |
| 3.18 | Panacea's Terraform under OpenTofu | environment | KPMG, copied with attribution | our subscription, Australia East | Entra RBAC | live estate code | saves ~0.75 | **integrate** |
| 3.19 | Azure Verified Modules | environment | MIT | n/a | n/a | OpenTofu not supported by AVM | n/a | reference |
| 3.20 | Coolify (+ Supabase) | environment | Apache-2.0 | an Azure VM, Docker over SSH | not verified | 62,409 stars, v4.3.23 | ~1 (a demo) | reject |
| 3.21 | Key Vault, Blob, Monitor, ACR | substrate | Azure services | Australia East | Entra | GA | inside phase 8 | **adopt** |

**What the table says.** Every capability here already rests on a product the plans chose (Keycloak, LiteLLM, managed Postgres, OpenTofu, Azure's services). No whole product removes the Hub's domain rules or the gateway's two small services; the second direction's gains are Panacea's realm today, Panacea's code in two places, and a real Keycloak in tests.

## 5. Recommendation

**Sign in through Panacea's Keycloak realm now. Run our own LiteLLM gateway, rebuilt from the MIT tree and started from Panacea's key-minting code. Finish the Hub as the thin Go service it already is, on Azure Database for PostgreSQL Flexible Server. Put all of it in our own AKS in Australia East, built from Panacea's production Terraform under OpenTofu. Do not use Supabase or Coolify here. About 19 engineer-days to first real use: about 1 is new glue, and the rest is planned work that no product removes.**

Why this answer:

1. **It is the most complete answer for every need.** Keycloak and LiteLLM are already decided, and Panacea already runs Keycloak. Azure's managed Postgres gives HA, point-in-time restore, geo-backup inside Australia and Entra auth. The Hub's domain rules exist in no product: digest-checked revisions, the release pin and the hash chain (the earlier study: nobody has open-sourced our product).
2. **The allowed alternative loses on merit.** Self-hosted Supabase lacks backups, PITR and multiple projects. It runs on Kubernetes only through a community chart, and it trusts Keycloak only through a static key list. The Hub's rules would move into SQL or Deno: about 13 days against about 10, and a dozen services to patch. Coolify cannot host the sandboxes, so it would be a second platform to run.
3. **Panacea is used wherever it is the better complete answer.** We take its realm now (no new Entra work), and copy its hardened Terraform and its LiteLLM key code with attribution. Its gateway and its tenant database are not used, for the reasons in §2.
4. **It keeps the customer-cloud path.** Every part goes into a customer's Azure subscription unchanged. The one runtime dependency on Panacea, the realm, is replaced by our own Keycloak (3.11) at the customer-hosted tier.
5. **It keeps the provenance thread in one ledger we control**, the one 06-deploy's signed release statement (G2) writes to.

### The 19 days, to first real use

| Days | Work | Whose | Plan tasks |
|---|---|---|---|
| 10 | The Hub to its minimum (S7), with `devissuer` replaced by a Keycloak dev container (3.12) | Hub stream | 08-10, 13-24, 28-35, 37-39: about 80 plan-hours |
| 8.2 | The gateway | phase 6 | all 29; task 22 becomes an Orca panel if 02 rules so |
| 1 | Environment glue for first use: one Flexible Server (databases `hub` and `litellm`, private endpoint), Key Vault, DNS and ingress for the Hub and the gateway, by a scripted runbook using Panacea's `.tf` values; codified afterwards in phase 8 tasks 05-08, 10 and 29 | environment stream | new runbook; then 05-08 at half their hours |
| 0 | The realm change: about half a day of the Panacea stream's time, not ours | Panacea stream | an ask |
| **19.2** | | | |

The total above is **about 19 engineer-days**. The elapsed time is set by lanes, not the total. The Hub's two lanes are 41 and about 38 plan-hours. With the gateway on two engineers and the environment on one, five engineers reach first use in about 6 working days. That is our estimate, not a scheduled plan.

### What it replaces in our plan

- **Hub** (50 tasks, 133.5 plan-hours): removes tasks 11-12 (`devissuer`, 6 hours) and adds about 1, so **net 5 hours**. Under Orca, tasks 32, 37-38 and 40-42 (about 22 plan-hours) may drop or move into Orca panels. That is 02's ruling, so it is not counted here.
- **Phase 6** (29 tasks, 8.85 plan-days): removes no task. Tasks 04 and 07 start from Panacea's code (about 0.3 days less), and task 22 moves to Orca if 02 so rules (0.4 days).
- **Phase 8** (37 part files, 98.5 plan-hours, about 53 of them this capability's): tasks 05-08 start from Panacea's files (**about 6 hours less**). The Keycloak half of task 29 waits for our own Keycloak (about 1.5 hours less now). Task 09's realm file also seeds the dev container.
- **Honest total:** about 2.3 engineer-days out of about 27 still to build across the three plans. **It does not replace most of the Hub, phase 6 or phase 8.** Those plans already stand on the best third-party parts. What remains is our domain logic and the glue between products, and 3.2, 3.4 and 3.20 show that swapping the substrate does not shrink it.

### Risks

1. **Panacea's realm:** Keycloak 25, one replica, sessions in memory. The ask lands in another session's repo during a cutover. Mitigation: our own Keycloak (3.11) is about 2 days whenever it is needed, and the realm file already exists.
2. **LiteLLM moves weekly** and has two verified defects (`OD-33`). Pin by digest, and run the canary first.
3. **The Hub's 80 plan-hours are estimates.** Review has grown them before (review M4 added 6.5 hours).
4. **Copied Terraform is a fork.** Panacea's later fixes do not reach us. Record the source commit in each header, and re-diff at the Panacea lift.
5. **A customer may forbid PaaS databases.** Then the Hub's Postgres moves to CloudNativePG on AKS (Panacea's tenant pattern, Apache-2.0) behind the same connection string, about 1 day.

## 6. Glue we still build

New work that no plan holds today. Planned tasks kept as they are (the Hub's minimum, phase 6) are in §5's table, not here.

| # | Glue | Where | Days | Needs first |
|---|---|---|---|---|
| P1 | **The Keycloak dev container**: a compose service in the Hub's local stack running Keycloak in dev mode with `--import-realm`; a dev copy of the realm file with a test-only client and `tid`/`oid` attribute mappers; a token helper for tests. Replaces Hub tasks 11-12 | `control-plane/cmd/hub` local stack, Hub test harness | 0.15 | environment task 09's realm file (built) |
| P2 | **The first-use environment runbook**: one Flexible Server (Burstable in dev; databases `hub` and `litellm`; private endpoint; Entra admin), Key Vault, DNS records and ingress for the Hub and the gateway, with values taken from Panacea's `postgres-flexible.tf` and `object-storage-blob.tf`; phase 8 tasks 05-08 codify it afterwards | `infra/scripts/`, then `infra/terraform/modules/` | 1 | the subscription; 01-sandboxes' AKS half-day |
| P3 | **Attribution for every Panacea snippet**: a header naming the source path and Panacea commit in each copied `.tf` file and in keysmith's key code, and one `THIRD-PARTY.md` line each | wherever a snippet lands | inside the tasks | `OD-13` relaxed (done) |
| P4 | **The realm ask, handed over as a block**: the ruling's K1-K4, the `panacea-app-desktop` client, the four audience scopes and the `panacea-app-fleet` client, for the Panacea stream to apply; accepted by the Hub's task 43 real-tenant checklist | Panacea stream; Hub task 43 | 0 of ours (about 0.5 of theirs) | the Panacea stream's capacity during its cutover |
| P5 | **Later, each on its trigger**: our own Keycloak by the Operator (2 days; a customer-hosted tier or a stalled P4), the ledger witness (0.5-1; an assessor asks), `NOTIFY` pushed as server-sent events (0.5; a surface needs sub-second), the Azure Blob `blobstore` with immutability (0.5; blob sizes) | our AKS | 3.5-4, not in the 19 | the trigger named |

**New glue for first use: about 1.15 days** (P1 and P2). Asks of the Panacea stream: the realm change (P4); recommended, not required, Keycloak 26 with two replicas, so that sessions survive a restart. 06-deploy's P-3 (Panacea trusts our realm token) is that study's ask, not ours.

## 7. Owner calls

1. **The subscription and its platform line items (money).** This adds to 01-sandboxes' call 1. A dev Flexible Server (Burstable B2ms) is US$0.208 an hour, about US$152 a month, plus US$0.138 per GB-month of storage (Azure Retail Prices API, Australia East, list, 2026-09-30). Key Vault and Blob are small, and Log Analytics is capped by task 04. Production adds zone-redundant HA (a second instance) and geo-backup; its General Purpose price was not retrieved. **If the subscription is late**, the fallback is hosting the Hub as a Panacea project (3.4). That sets `OD-5` aside for the Hub while it runs.
2. **Which accounts back inference on day one (money, contract, customer-visible).**
   - **Global:** `OD-19` names the direct Anthropic API with zero retention. That needs a KPMG Anthropic API organisation and a zero-retention agreement; neither is in the repo. KPMG's Kelios endpoint exists today as Panacea's Claude upstream, but its retention terms are not in the repo either.
   - **AU:** an AWS account for Bedrock Sydney (phase 6 task 20).
   - **The call:** which accounts pay, and which contract carries the zero-retention promise customers will be told.
3. **The budget defaults (money).** `OD-24` turns budgets on by default. The per-person, per-project and per-team amounts are the owner's numbers (the fallbacks in phase 6's P6-D8).
4. **Sign-in rests on Panacea until our own Keycloak (customer-visible).** A restart of Panacea's single Keycloak pod signs every user out, and a realm outage stops sign-in. Three ways to handle it:
   - accept it for the pilot;
   - wait for the Panacea stream to move to Keycloak 26 with two replicas;
   - stand up our own Keycloak now (3.11), about 2 days plus an IT ticket for the Entra redirect URI.
5. **What customers are told about AU (customer-visible).** No screen may say "AU only" until the `count_tokens` canary (phase 6 task 26) passes. Global calls leave Australia by definition, and the isolation and data wording should say so.

Not a call today: a LiteLLM Enterprise licence (3.16) waits until more than 5 operators need LiteLLM's own admin UI.

## 8. Revised for the owner's second direction

The first run (stopped at 09:47) left only this file's skeleton, so there was no earlier answer or verdict to revise in place; every section was written under the second direction. What that direction changed, against the plans and the 2026-09-28 study:

| # | Topic | Before | Now | Why |
|---|---|---|---|---|
| 1 | Panacea's code | read, never copied (`OD-13`; phase 6 plan: "Panacea's LiteLLM code is read for its lessons ... and nothing is copied") | **copied with attribution**: its Terraform into phase 8 tasks 05-08, its key code into keysmith's tasks 04 and 07 | the owner relaxed `OD-13` today |
| 2 | Supabase | not considered for the Hub | considered; **reference, not the Hub** (3.2); Supabase Cloud rejected (3.3) | allowed by the owner; loses on backups, Kubernetes support, Keycloak trust and the Hub's rules |
| 3 | Coolify | not considered | **rejected** for the environment (3.20) | cannot host the sandboxes; a single VM for the system of record |
| 4 | Panacea hosting the Hub | ruled out by `OD-5` | **the fallback host** (3.4) | "Panacea is fine to use majority of"; it saves the subscription wait, not work |
| 5 | Identity | Panacea's realm until phase 8's own Keycloak (`OD-30`) | unchanged; our own Keycloak moves to the Operator and waits for the customer-hosted tier; `devissuer` gives way to a real Keycloak (3.12) | fewer custom parts, and the real issuer in tests |
| 6 | Gateway | LiteLLM with keysmith and gatekeeper (`OD-21`, `OD-33`) | unchanged; alternatives listed as reference only | `OD-21` is locked, and LiteLLM's paid tier is exactly what the two services build |
| 7 | The cockpit | Hub tasks 41-42 are desktop surfaces | may become Orca panels (02's ruling) | Orca is the owner's pick |

**For the sibling studies.** 03-define, 04-design, 05-work-team and 06-deploy each left "does the Hub move to Supabase?" to this study. The answer is **no**, so Supabase Realtime (05) and Onlook's Supabase dependency (04) stay reference, and 06-deploy's signed release statement lands in the Hub as planned.

## 9. What was not verified

Nothing was installed, deployed or run. Product claims come from documentation, vendor pages, the GitHub API and the Azure Retail Prices API, read on 2026-09-30.

1. **Supabase with Keycloak:** the static-JWKS trust path and the `role` claim mapper are inferred from the self-hosted docs, not tried; 3.2's 13 days are our estimate.
2. **The Hub as a Panacea project (3.4):** whether Panacea builds from a monorepo subdirectory (`control-plane/cmd/hub`), and whether the `hub` schema's grants survive Panacea's migrate and runtime roles, were not checked.
3. **Flexible Server:** that `roles.sql` and `0001_init.sql` apply as the Entra administrator (two `NOLOGIN` roles, `SECURITY DEFINER` functions, row-level security) is supported by Azure's documentation but not run; the General Purpose price was not retrieved.
4. **Estimates, not measurements:** all plan-hours are the plans' own; the savings in §5 (5, 6 and about 2.5 hours) and the 6 elapsed days are ours; the realm ask's half day is our guess at another team's work.
5. **Third-party figures:** LiteLLM Enterprise prices (guides, not LiteLLM), Bifrost's open-tier features (vendor pages), Claude in Foundry's regions (press reports), Azure confidential ledger's Australia East listing (a third-party guide; Microsoft's residency page confirms paired-region replication only) and its price (not published).
6. **Not checked at all:** whether Keycloak events are enabled in Panacea's realm; AVM under OpenTofu; Kelios's API shape, retention terms, and support for `count_tokens` and streaming.
7. **Source limits:** the GitHub API's unauthenticated quota ran out once mid-run (shared with sibling agents), so Bifrost's, Portkey's and Zitadel's figures are from the Glasses catalogue (observed 2026-09-29 to 30). Glasses is untrusted discovery data; no licence in it was taken as approval.

## Sources

**Repository (read-only or ours):** `00-panacea.md` §3-10; `01-sandboxes.md` §4.15-4.16, §5, §6; `02-fleet-cockpit.md` §0; `03-define.md` 4.12; `05-work-team.md` §0.2; `06-deploy.md` §0, 3.1, 3.10, §6; `2026-09-23-owner-decisions.md` (`OD-3`, `OD-5`, `OD-13`, `OD-18`-`OD-24`, `OD-30`, `OD-32`-`OD-35`); `2026-09-25-identity-keycloak-broker.md` §1-6; `2026-09-28-oss-adoption/00-summary.md`, `05-hub-identity.md`, `licence-platform.md`; the plans `.claude/worktrees/hub/.../2026-09-23-phase-h-hub.md`, `.claude/worktrees/phase-6/.../2026-09-25-phase-6-gateway.md`, `.claude/worktrees/env/.../2026-09-23-phase-8-own-environment.md`; `control-plane/internal/hub/migrate/roles.sql` and `sql/0001_init.sql` on `feat/hub`; in `Projects/Panacea`: `deploy/postgres-flexible.tf`, `deploy/object-storage-blob.tf`, `deploy/*.tf` (ISM citations), `services/panacea-api/handlers/llm_vkeys.go`, `handlers/llm_env_keys.go`, `clients/db_roles_ensure.go`, `clients/db_roles_ops.go`.

**Web (read 2026-09-30):**
[Supabase self-hosting](https://supabase.com/docs/guides/self-hosting) ·
[Supabase self-hosted auth keys](https://supabase.com/docs/guides/self-hosting/self-hosted-auth-keys) ·
[supabase-kubernetes](https://github.com/supabase-community/supabase-kubernetes) ·
[LiteLLM enterprise](https://docs.litellm.ai/docs/enterprise) ·
[TrueFoundry on LiteLLM pricing](https://www.truefoundry.com/blog/litellm-pricing-guide) ·
[Keycloak 26.7.3](https://www.keycloak.org/2026/08/keycloak-2673-released) ·
[Azure PostgreSQL access control](https://learn.microsoft.com/en-us/azure/postgresql/security/security-access-control) ·
[Azure confidential ledger data residency](https://learn.microsoft.com/en-us/azure/confidential-ledger/data-residency) ·
[its pricing page](https://azure.microsoft.com/en-au/pricing/details/confidential-ledger/) ·
[Electric auth guide](https://electric-sql.com/docs/guides/auth) ·
[AVM on OpenTofu, discussion #1512](https://github.com/Azure/Azure-Verified-Modules/discussions/1512) ·
[APIM `llm-token-limit`](https://learn.microsoft.com/en-us/azure/api-management/llm-token-limit-policy) ·
[Bifrost for Claude Code (vendor)](https://www.getmaxim.ai/articles/best-claude-code-gateway-for-enterprise-governance/) ·
[Claude on Foundry and regions (InfoQ)](https://www.infoq.com/news/2026/07/claude-foundry-ga-europe/) ·
[Azure Retail Prices API](https://prices.azure.com/api/retail/prices).

**GitHub API (2026-09-30):** `supabase/supabase`, `supabase-community/supabase-kubernetes`, `PostgREST/postgrest`, `electric-sql/electric`, `nats-io/nats-server`, `temporalio/temporal`, `BerriAI/litellm`, `keycloak/keycloak`, `coollabsio/coolify`, `Azure/terraform-azurerm-avm-res-dbforpostgresql-flexibleserver` (repository and latest release). **Glasses:** offline `gsearch.py` and the MCP `glasses_search` for "llm gateway", "keycloak", "identity", "postgres", "realtime", "sync", "terraform", "supabase".
