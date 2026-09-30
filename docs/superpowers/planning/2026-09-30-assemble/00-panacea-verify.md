# 00 - What Panacea already provides: the sceptic's verification

**Status:** IN PROGRESS (2026-09-30). Skeleton; sections fill in one at a time.
**Date:** 2026-09-30. **Verifies:** `00-panacea.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation (C1, Panacea via REST + `panacea-mcp` + the
Keycloak realm + copied snippets) and its two best alternatives (C3 Azure-native managed services
for our own state; Coolify plus Supabase with Panacea snippets, the owner's named option) at
source: licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run; `Projects/Panacea` read only.

## 0. Verdict first

_Pending._

## 1. What was checked, and how

_Pending._

## 2. C1 Panacea: claims against source

Every line below was read at Panacea commit `efc637879` with `git show` (read-only; Panacea's
HEAD is `eee94cd94`, one docs commit later, on `finish-dev-outstanding`). "Holds" means the
source says what `00-panacea.md` says; "understated" means the source is worse for us.

| # | Claim in `00-panacea.md` | At source | Verdict |
|---|---|---|---|
| 1 | Deploy, versions, rollback, promote, previews, branches, evidence, project create, MCP download and token mint are routes at the cited lines | 14 lines spot-checked in `routes/routes.go`: `:1017`, `:1024-1028`, `:1033`, `:1162-1164`, `:1170`, `:1286`, `:1290-1291`, `:2720`, `:3010`, `:3050`; wrappers `:3226-3286` run `apiKeyMW`, `authMW`, the RBAC gate, then `middleware.Audit` | **holds** |
| 2 | `panacea-mcp` has 31 tools: 17 read, 4 action, 3 admin, 6 kubectl, 1 escape | `services/panacea-mcp/src/index.ts:249-579`, counted by `group:` field: 17, 4, 3, 6, 1 | **holds** |
| 3 | A realm access token is refused by the REST API | Any bearer that is not `BACKEND_SECRET`, `pnc_` or `panacea_` falls to 401 (`middleware/auth.go:156-160`, `:232`, `:245-251`). The one JWKS the API trusts is **Backstage's own issuer**, `https://${PANACEA_DOMAIN}/api/auth`, audience `backstage` (`deploy/portal/deploy-portal.yaml:605-621`), cached per one env key (`middleware/jwks.go:145-160`) | **holds, and the fix is code**: "trust the realm" means a second verifier in a single-issuer design, not a config line |
| 4 | A `pnc_` cannot mint a `pnc_`; minting needs a real grant | `handlers/api_keys.go:112-116`; `RequirePortalGrant` refuses "a person still waiting for team approval" (`middleware/rbac.go:357-373`) | **holds, with a missed step**: every PanaceaApp user must first be approved into a Panacea team before they can mint the token P-glue relies on |
| 5 | Every token expires, 90 days by default, 365 at most | `api_keys.go:71-100` | **holds**; the paste recurs quarterly by default, against `OD-30`'s zero clicks |
| 6 | The control plane holds "the user's `pnc_` token" | Account routes such as the git-PAT registration P3 needs refuse scoped tokens (`authedUnscoped`, `routes.go:3234-3243`); an unscoped token is "a full-access (owner-privilege) token" (`api_keys.go:35-37`) | **understated**: the stored token is owner-level on every Panacea project the user has, for up to a year. G4's proxy guards a high-value credential, and the report does not say so |
| 7 | Keycloak is one replica with in-memory sessions | `deploy/main.tf:928` (`replicas: 1`), `:954-957` (`--cache=local`), `:941-945` (every pod move logs everyone out), `:997-1005` (non-optimised start re-runs a ~5-minute Quarkus build on every fresh pod; dev SSO outage 2026-09-07); SSO outage 2026-09-21 (`:933`). Chart `keycloakx` 2.4.4 with no tag override (`:920-921`) is **Keycloak 25.0.0** (the chart's `appVersion`, fetched from codecentric's repo) | **understated**: 25.0.0 is two years old and out of community support; 25.0.6 (2024-09-19) fixed CVE-2024-8883 (open redirect) and CVE-2024-8698 (SAML signature bypass); Tenable flags 25.0.x below 26.0.6 as vulnerable |
| 8 | The edge "may be" locked to 14 KPMG `/32`s | The Terraform **default** is those 14 addresses; "`[]` opens the edge to the internet" (`deploy/variables.tf:278-297`), applied by every `terraform apply` (`main.tf:580-606`). The cited `deploy/restrict-edge-allowlist.sh` is **not in commit `efc637879`** (on disk, untracked); its header says the edge is "no longer reachable from Cloud Shell (an Azure address, deliberately not on the allowlist)" | **understated**: locked is the declared state. Our AKS in Azure cannot reach the API or `panacea-mcp`'s target until Panacea adds our egress. This machine egresses as `24.239.148.12` (seen in a GitHub rate-limit reply), which is on the list, so local mode works from a KPMG network |
| 9 | "A copy-on-write database per preview" | A three-mode policy, `seed`, `sanitised-clone`, `shared` (`clients/branch_data_policy.go:14-19`); the clone is a CloudNativePG point-in-time recovery from production's backup (`db_branch_ops.go:17-22`), refused at PROTECTED. **With nothing requested and no `panacea/seed.sql` in the repo, the preview shares the project's own database** (`branch_data_policy.go:47`, `:195-200`) | **understated**: an agent's preview writes to the app's production database by default. New glue (section 7) |
| 10 | Scans default to warn | `build-deploy.yaml:413` describes the **secret scan only**. The pipeline defaults (`:137-249`) are `warn` for `checks-mode` and `secret-scan-mode`, `true` for `enable-trivy-scan` (`:87-99`), and **`off` for SAST, DAST, IaC, SCA, licence, model, DB posture, Nuclei and Schemathesis**. On an unconfigured project panacea-api appends no mode at all (`clients/pipeline.go:472-473`); only a team policy, a capability or a posture declaration turns a scan on (`:474-491`). The platform baseline is SuperAdmin runtime data (`handlers/security_policy.go:969`), so what dev enforces cannot be read from source | **wrong**: a new project runs 3 checks at warn, not 18. P3 must declare the posture when it creates the project (section 7) |
| 11 | "18 scan checks" | `STATUS.md:125`. The SAST check cannot inspect anything as written: `semgrep scan --config auto --metrics=off` (`pipelines/tasks/sast-scan.yaml:122-124`, image `semgrep:1.86.0` at `:79`) is refused by Semgrep 1.86.0 before any output ("Cannot create auto config when metrics are off", `cli/src/semgrep/commands/scan.py:637-640`; `abort` is `sys.exit(2)`, `util.py:65-67`, fetched at tag `v1.86.0`). Step 1 then writes `{"results":[]}` and `SCANNED` (`:129-130`), so the ERROR branch (`:195-202`) is unreachable and the gate records **PASS** (`:216`) | **wrong in part**: switched on, SAST records a clean pass having scanned nothing. Read from code, not run. A Panacea-stream fix; until then the release evidence carries a false SAST pass |
| 12 | Hash-chained audit on every route | `clients/audit_pg.go:42` (`row_hash = sha256(prev_hash ‖ canonical(row))`); every wrapper ends in `middleware.Audit` | **holds** |
| 13 | Runs in `australiacentral` | `deploy/terraform.tfvars:24-28`: `rg-panacea-platform-dev`, `australiacentral`, `environment = "dev"` | **holds for dev**; AU residency yes |
| 14 | Forge is platform-admin only, not callable with an API key | `routes.go:3261-3273` (`authedAgent`) | **holds** |
| 15 | Promote with a stage ladder and owner approval | Routes exist (`:2814-2860`); 06-deploy §0 item 5 finds the regulated rungs wait on Panacea (secure-test needs Chains signing live, production needs DLP enforcing) | **holds for the innovation rung only** |

**Net for C1:** the routes, tools and token rules exist as described; nothing was invented.
Two claims are wrong: scans do not default to warn (3 checks run on a new project), and the
SAST check passes without scanning when switched on. What the report understates is the price
of reaching the rest from our architecture: a locked edge by default, a full-power year-long
token per user, a team-approval step before that token exists, a preview that shares
production data by default, and a sign-in on an unpatched single-replica Keycloak.

## 3. C1 Panacea: licences in the stack

C1 ships none of Panacea into a customer tenancy: Panacea runs on KPMG's estate and we call it
(`OD-5`, we-host). So each licence is tested three ways: what our we-host use triggers now, what
the snippets we copy carry, and what a customer-hosted tier would force. Pins read at
`efc637879`; licences read at source on 2026-09-30.

| Component | Pin at source | Licence at source | Forces now (we-host) | Forces at a customer-hosted tier |
|---|---|---|---|---|
| Panacea's own code (every snippet P4 copies) | no `LICENSE` at the repo root (`git ls-tree efc637879`) | none: KPMG proprietary | nothing: KPMG copies KPMG code. P4's ledger must still record any third-party origin inside a copied file (not checked file by file) | nothing |
| Vault | chart 0.28.1 (`argocd-apps/platform/vault.yaml:21-22`, `deploy/main.tf:835-837`), no image override, so Vault **1.17.2** (chart `appVersion`, `hashicorp/vault-helm` tag `v0.28.1`) | **BUSL-1.1**, Licensed Work "Vault Version 1.15.0 or later"; production use allowed unless "offering the Licensed Work to third parties on a hosted or embedded basis in order to compete with HashiCorp's paid version(s)" (`hashicorp/vault` `LICENSE` at `v1.17.2`) | nothing while Panacea sells deploys, not secrets management | **restricted** by our policy; the grant turns on "compete", a legal call avoided by `OD-32`'s OpenBao (MPL-2.0) swap |
| MinIO | chart 5.2.0 (`argocd-apps/data/minio.yaml:21-23`), no image override | **AGPL-3.0**; repository **archived**, last push 2026-04-24 (GitHub API) | nothing for an unmodified server behind `s3proxy`; the cost is no upstream fixes. **Missed by the report:** Panacea already has the swap, Azure Blob (ADLS Gen2) behind the same S3Proxy (`deploy/object-storage-blob.tf:1-22`, ADR-029), off on dev (`managed_object_storage_enabled` defaults false), on for the Australia East rebuild | **restricted**: AGPL §13 would force a source offer for any modified copy users reach over a network; moot once Blob replaces it |
| Grafana | kube-prometheus-stack 61.3.0 (`argocd-apps/platform/observability.yaml:20-22`), `grafana.enabled: true` (`helm-values/prometheus-values.yaml:186-187`) | **AGPL-3.0** | nothing, unmodified | **restricted**, as MinIO; Grafana Enterprise is by quote |
| Loki, Tempo | values files only (`helm-values/loki-values.yaml`, `tempo-values.yaml`); **no Argo CD app or Terraform release installs them** at this commit, and the cutover plan deletes their AUE overlays | AGPL-3.0 (upstream) | nothing: the report's "in the estate" is not shown by source | only if installed |
| Redis | Bitnami chart 19.5.0 (`argocd-apps/data/redis.yaml:21-23`), no image override, so Redis **7.2.5** | BSD-3-Clause (Redis's 2024 RSALv2/SSPL change starts at 7.4) | licence fine. **Maintenance risk missed:** the chart pulls `docker.io/bitnami/redis:7.2.5-debian-12-r0`, and Bitnami moved versioned tags to the unmaintained `docker.io/bitnamilegacy` from 2025-08-28 (paid replacement: Bitnami Secure Images; bitnami/charts#35164). A fresh node pulling that tag fails unless it is mirrored; `redis-values.yaml` sets no mirror. What depends on this Redis was not traced (oauth2-proxy uses cookie sessions, `oauth2-proxy-entra-values.yaml:109-112`) | as now; a maintained image source first |
| Semgrep | image `panacea/semgrep:1.86.0` (`pipelines/tasks/sast-scan.yaml:79`), `--config auto` (`:123`) | engine LGPL-2.1; registry rules under the **Semgrep Rules License v1.0**: use "only for your own internal business purposes", not distributed or made "available to others as a service" (Semgrep's licensing pages via search; `semgrep.dev` itself is blocked on this network) | **restricted from the first external customer**, not only at a customer-hosted tier: scanning a customer's app in our we-host SaaS with registry rules is offering them as a service. The report's "fine inside KPMG" holds only while every user is KPMG. Moot today, because the check aborts before it scans (section 2, row 11) | **restricted**; Opengrep (LGPL-2.1, active, GitHub API 2026-09-30) with rules baked into the image |
| ClamAV | `argocd-apps/app-services/clamav.yaml` | GPL-2.0 (GitHub API) | nothing: GPL has no network clause | **restricted**: shipping its binary is distribution, with GPL's source duty |
| Keycloak, Backstage, CloudNativePG | as the report's sections 5, 7 | Apache-2.0 (`LICENSE` files at source) | nothing | nothing |

**Net for licences.** For we-host C1 the stack forces nothing we cannot live with, with one
correction in timing: Semgrep's registry rules leave "internal business purposes" the day a
non-KPMG customer's app is scanned, which is `OD-5`'s own model, so owner call 4 is early for
everything except Semgrep. Two maintenance risks the report did not name sit on Panacea's side
and ride into our shipped apps' uptime: the Bitnami Redis tag, and MinIO on dev (Blob replaces it
at the rebuild, which the report also missed).

## 4. C1 Panacea: the auth path from our control plane and sandboxes

Three callers reach Panacea in C1. Each was traced at source.

| Caller | Presents | Accepted today? | Reach | Gap |
|---|---|---|---|---|
| Desktop or Orca, sign-in | realm auth-code + PKCE on a new public client `panacea-app-desktop` (P5a) | the client does not exist: `backstage` and `panacea-oauth2-proxy` are both confidential (`setup-keycloak.sh:139-142`, `:320-323`) | signs the user in to us | Keycloak 25.0.0, one replica (section 2, row 7) |
| Our control plane to panacea-api | the user's pasted `pnc_` | yes (`middleware/api_key.go:189`, `AuthMethod: "api-key"`) | unscoped: everything the user can do on every project, up to 365 days. P3 needs unscoped (git PATs are `authedUnscoped`; project create is `authedGrant`) | a realm token is refused (`auth.go:245-251`) |
| Sandbox `panacea-mcp` to our G4 proxy to panacea-api | nothing of Panacea's; the proxy adds the token | by design | only G4's allowlist | 4.2 |

**4.1 P-3, "trust our realm token", is code, and 1-2 days is the low end.** `Auth()` trusts a
JWT only behind `BACKEND_SECRET` (`auth.go:156-160`). The one verifier (`jwks.go:147-153`) is built
for that transport: when the JWKS is unset or unreachable inside a grace window it falls back to an
**unverified decode** (`jwks.go:225-259`), safe only because the shared secret already
authenticated the caller. A directly presented realm token needs a new branch in `Auth()`, a strict
verifier with no unverified fallback, Keycloak claim mapping (groups carry teams) and an audience
mapper on our client. Minting then works, because the guard refuses only `AuthMethod == "api-key"`
(`api_keys.go:112-116`) and `Auth()` stamps `sso` on every other identity (`auth.go:254-255`).
Judged 2-3 days in Panacea with its security review. Not our days, but on the path to removing
the paste (section 8).

**4.2 An 8-hour scoped token is not an 8-hour bound (new).**
- `scopeRole=deploy` is developer (`models/models.go:550-553`).
- Developer can mint a legacy project key, `POST P/api-keys` (`routes.go:2942`). The handler has
  no token-bootstrap guard (`handlers/apikeys.go:32-90`); `expiresIn` is days and `0` = never
  (`:39`); expiry is checked only when set (`:178-179`).
- A legacy key authenticates as a full project member with its scopes discarded (`auth.go:232-240`,
  `project, _, valid`), bound to no person (`Username: "apikey:" + project`), so the owner
  re-check that kills a `pnc_` when its owner leaves does not apply.

So any holder of a developer-role token on a project, an agent included, can turn it into a
credential that outlives the session, the user's membership and the token's scopes. Three
consequences: the per-session token must never enter the sandbox, even after P-3; G4 (06-deploy
§6, which already refuses "token minting" and stays after P-3) must match method and route, never
a prefix on `P/`; and one more Panacea ask, small: guard legacy key minting as `api_keys.go:112-116`
does, or retire the route. For the Panacea stream only: the `pnc_` mint guard keys on `AuthMethod`
alone, which a legacy-key identity never carries, while `handlers/stepup.go:272` counts both as
tokens; whether a legacy key can mint a `pnc_` was not traced to the end (it does pass
`RequirePortalGrant`, `rbac.go:374-390`, holding `project-<name>`).

**4.3 The other auth claims.**
- Remote MCP: holds as reported, including the host mismatch: `https://sso.<domain>/realms/panacea`
  (`mcp_wiring.go:41`) against `KC_HOSTNAME` `https://auth.<domain>` (`deploy/main.tf:963-964`).
- `panacea-cli`: holds. Client `panacea-cli`, device flow (`internal/auth/auth.go:50`, `:67`); 0
  matches in `setup-keycloak.sh`.
- Preview sign-in (new): oauth2-proxy sends no `kc_idp_hint` (no match in `kubernetes/helm-values`,
  `deploy/portal`, `deploy/*.tf`) and the realm script sets no redirector, so a browser without a
  Keycloak session shows Keycloak's own login page before Entra. An embedded browser such as Orca's
  normally keeps its own cookie store (not checked in Orca's source): the first preview opened
  there shows that page and a Microsoft sign-in. Section 8 counts it as a seam.
- Egress: only our control plane crosses Panacea's edge (sandboxes reach our proxy), so P5b is one
  address, our AKS egress IP. Holds.

**Net for auth.** Day 1 works with the pasted token and G4, as the report says. Zero clicks
(`OD-30`) and a scoped token per session need **two** Panacea changes, not one: realm trust (2-3
days, not 1-2) and a guard on legacy key minting.

## 5. Alternative: Coolify plus Supabase (the owner's named option)

06-deploy-verify §4 already tested this pair as a deploy step and upheld the reject at 18-26
days. This section agrees on the shared facts and adds what this cluster owns: sign-in, the
shipped app's backend, and the cockpit's ability to create one. Read 2026-09-30 from Coolify's
tag `v4.3.23` and branch `v4.x` (raw files), its release feed and docs index (`coolify.io/docs/llms.txt`),
Supabase's self-hosting, regions and pricing pages, and `supabase/supabase` `docker/` on `master`.

| Claim in `00-panacea.md` | At source | Verdict |
|---|---|---|
| Both Apache-2.0; Supabase Auth MIT | `LICENSE` files: Coolify (`v4.x`) and `supabase/supabase` Apache 2.0; `supabase/auth` MIT | **holds**; nothing restricted, portable to a customer's cloud |
| Coolify stable is v4.3.23 (2026-09-18) | release feed: v4.3.23 at 2026-09-18, six 4.3.x releases in September; `config/constants.php:5` | **holds**; active, one company, community support only when self-hosted |
| Coolify OIDC only in a 4.4 RC | at `v4.3.23` the OAuth seeder lists `azure`, `authentik`, `zitadel` and others but **no `oidc`** (`database/seeders/OauthSettingSeeder.php:19-28`); `oidc` exists on `v4.x` only (`:26`) | **holds for Keycloak.** Understated for Entra: direct Entra sign-in is stable, but it needs "an app registration, tenant ID, and client secret" (docs, Azure OAuth page), which is an IT ticket and a bypass of `OD-30`'s broker |
| No Azure DevOps (fails `OD-4`) | any git host deploys by deploy key plus an authenticated deploy webhook (`/applications/sources/deploy-keys`, `/core/automation/deploy-webhooks`); previews come from GitHub and GitLab events only (preview page) | **overstated**: ADO deploys work from an ADO pipeline; ADO previews do not. Agrees with 06-deploy-verify |
| Docker hosts rather than AKS | 0 mentions of Kubernetes in the docs index; "Docker Swarm (Deprecated)" (`llms.txt:81`) | **holds, and narrower**: single Docker hosts, with Swarm on its way out |
| Self-hosted Supabase is one project per stack, with no branching, PITR or management API | self-hosting page: "runs as a single project"; unavailable: "branching, ..., managed backups and PITR, ..., and the platform management API". One stack is 11 services (`docker/docker-compose.yml`: studio, api-gw, auth, rest, realtime, storage, imgproxy, meta, functions, db, supavisor) | **holds** |
| "No API to create one from the cockpit" (section 10) | Coolify's API has `create-service` (docs index `:938`), its CLI has `service create` for one-click templates (`:500`), and scheduled Postgres dumps to S3 exist (`/databases/backups`) | **wrong**: the cockpit can create a Supabase stack per app through Coolify's API. What it cannot get is a database branched per preview or a point-in-time restore |
| (missed) Studio sign-in | `docker/.env.example:58-59`: one `DASHBOARD_USERNAME` and `DASHBOARD_PASSWORD` | a shared password, no SSO, no per-user audit (06-deploy-verify found the same) |
| Supabase Cloud in Sydney fails `OD-18` | regions page: AWS only; Sydney is `ap-southeast-2` on AWS | **holds** |
| Paid gates | Coolify self-hosted: none (06-deploy-verify, pricing page). Supabase: branching US$0.01344 per branch-hour on Pro (from US$25 a month), PITR US$100 a month per 7 days of retention, dashboard SSO on Team (from US$599 a month) | **holds**: every feature C1 already runs is, on Supabase, a Cloud feature, paid, and on AWS |
| 16-24 days | 06-deploy-verify: a floor; 18-26 | **agree**, plus this cluster's share: each shipped app becomes its own 11-service stack on a Docker host, with backups, upgrades and key rotation ours. Judged 3-5 days more (an estimate, not measured) |

**Cohesion, for the user's ask.** Headless, the pair could sit fully behind our own screens: both
have APIs, and Coolify even ships an MCP (`/mcp/capabilities/*`). But run as products they add two
consoles with two sign-ins (Coolify's own users or a separate Entra registration; Studio's shared
password) and a second identity system for shipped apps (Supabase Auth beside Keycloak). Panacea
has the same problem once (its Portal); this pair has it twice.

**Verdict: reject upheld** for the platform layer. Its real strengths, permissive licences and
portability to any VM, matter at the customer-hosted tier, not for a we-host tool needed now.

## 6. Alternative: C3 Azure-native managed services for our own state

C3 is not a rival to C1 for shipped apps; the report uses it for our own backend (the Hub), next
to C1. Tested on Microsoft Learn (Flexible Server overview, updated 2026-09-05; Entra
authentication concepts, updated 2026-09-06), the Azure Retail Prices API, and the phase 8 plan on
the `env` worktree (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/`).

| Claim | At source | Verdict |
|---|---|---|
| Entra-native | three modes, one of them "Microsoft Entra authentication only"; the administrator "can be a Microsoft Entra user, Microsoft Entra group, service principal, or managed identity"; managed identities connect | **holds**. A managed identity needs no app registration, so no IT ticket; the Entra admin group is an owner input (task 08) |
| PITR | "restore your server to any point within the backup retention period", 7 days by default, up to 35, on zone-redundant storage | **holds** |
| Zone-redundant HA, AU | region table: **Australia East** has zone-redundant HA, same-zone HA and geo-redundant backup, with no deployment block; Australia Central (where Panacea's dev runs) has **no** zone-redundant HA | **holds for Australia East only**; our environment must stay there |
| Private endpoints | "With virtual network integration, public access is denied"; task 08 plans a private endpoint | **holds**. The V6 SKU family (preview) does not support VNet integration, so pin v5 |
| Already planned (tasks 05, 08) | task 05 "`modules/registry` and `modules/keyvault` ... both private"; task 08 "the Hub's database, Entra-only", Postgres 16, private endpoint, an Entra group as administrator, the Hub on its own managed identity | **holds**: no new days |
| Price basis (report: "Azure consumption") | General Purpose Ddsv5 in `australiaeast`: **US$0.122 per vCore-hour**, so 2 vCores is US$0.244 an hour, about US$178 a month (retail API, `priceType` Consumption). Zone-redundant HA adds a standby of the same size. Storage, backup beyond the free allowance, Key Vault (per operation) and Blob (per GB and operation) were not priced | **holds**; about US$360 a month for a 2-vCore Hub with HA, before storage |
| Defender | not checked | **unverified** |

**Maintenance and backing.** GA, Microsoft-run, monthly patching in a chosen window (overview
page). No licence question: the service is bought; the engine is community PostgreSQL.

**Cohesion.** The best of the three: no console a user ever sees, so nothing in it can make the
product feel like another app.

**What could still refute it.** Only the subscription: C3 needs our own Azure subscription in
Australia East, which is the report's owner call 3 (the C6 stopgap if it is late). Its status was
not checked here.

**Verdict: buy, upheld**, for our own state. It is a complement to C1, not an alternative to it.

## 7. Days against our architecture

The report's 2 days (section 12 there) sit on top of 06-deploy's 6. Each item was set against the
Go control plane's `Backend` seam, Orca or the Tauri desktop, Keycloak, LiteLLM and Azure.

| # | Report | What our architecture adds, at source | Days |
|---|---|---|---|
| P1 | `panacea-mcp` bundle into the image, 0.25 | the download needs an authenticated caller (`routes.go:3010`, `authed`), so fetching it at image build puts a Panacea token in our pipeline. Copy `services/panacea-mcp` (KPMG code, P4's ledger) and build it instead | 0.25, holds |
| P2 | bootstrap skill into the worktree, 0.25 | holds: Panacea's own design commits it in each repo (`handlers/agent_context.go:23`); `agent-context` is on G4's read list | 0.25, holds |
| P3 | create the project from the cockpit, 0.5 | also: declare the scan posture from the data class, because a new project runs 3 checks (section 2, row 10); the call needs the user's unscoped token (section 4); a user with no Panacea team cannot create one (`authedGrant`; section 2, row 4), so the cockpit must say so; the job code is owner call 2 | **0.75-1** |
| P4 | snippet ledger, 0.25 | holds; P1 adds one entry | 0.25 |
| P5 | three asks, 0.25 | six now: PKCE client, egress IP, realm trust (2-3 days of theirs, section 4.1), a guard on legacy key minting (4.2), the SAST fix (section 2, row 11), `kc_idp_hint=entra` on oauth2-proxy's login URL (4.3) | **0.5** |
| P6 | live proof on dev, 0.5 | runs from a KPMG network only until P5b lands: this machine's egress is on the edge list, our AKS's is not (section 2, row 8) | 0.5, holds for local mode |
| G4 | 06-deploy's proxy, 1 | must match method and route and never hold a token the sandbox can read (4.2). A Go `httputil.ReverseProxy` with SSE flushing fits the control plane | 1, holds (06 counts it) |
| — | cohesion glue (section 8) | Panacea's steps surfaced inside our cockpit: team-request state, the token connect with a deep link that returns, previews opened where the Keycloak session lives | **0.5-1**, new |
| | **Total here** | | **about 3-3.75**, not 2 |

**The shortened tasks.**
- Phase 6 tasks 04, 06, 09 exist as named (LiteLLM admin client, budgets, usage rollup). The code
  to copy targets whichever LiteLLM Panacea runs, `v1.63.11` or a `main-v1.40.0` build, still
  unresolved (report section 6); ours will be current. The spend query it relies on,
  `/spend/logs/v2`, still exists on LiteLLM `main` with no premium gate
  (`spend_management_endpoints.py:2454-2464`). Shortening is plausible; per-call parameters were
  not diffed.
- Phase 8 task 09 (our own realm) deferred: holds, and it is a risk, not a saving: our sign-in's
  uptime becomes Panacea's single-replica Keycloak 25.0.0, which had SSO outages on 2026-09-07 and
  2026-09-21 (section 2, row 7).
- Hub task 14 exists ("the one transaction shape — principal, org, audit lock, idempotency,
  events"); copying `audit_pg.go`'s chain into it holds.

**"Usable on day 1" holds for local mode on a KPMG network**: a sandbox with `panacea-mcp` behind
G4 deploys a branch preview, reads logs and rolls back, once a project and a pasted token exist.
Cloud mode waits on Panacea applying our egress address (P5b), which is another team's Terraform
apply. Orca or Tauri makes no difference to these days: the PKCE client's redirect URIs must cover
whichever shell signs in.

## 8. Corrected recommendation

_Pending._

## 9. Not verified

_Pending._

## 10. Sources

_Pending._
