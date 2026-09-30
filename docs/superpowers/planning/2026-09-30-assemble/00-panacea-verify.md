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
| 10 | Scans default to warn | `pipelines/pipelines/build-deploy.yaml:413` ("default warn = record but never block") | **holds** |
| 11 | "18 scan checks" | `STATUS.md:125` ("scan catalogue (18 checks"), a status file the report itself says lags the code | **holds as cited**; the count was not re-derived from code |
| 12 | Hash-chained audit on every route | `clients/audit_pg.go:42` (`row_hash = sha256(prev_hash ‖ canonical(row))`); every wrapper ends in `middleware.Audit` | **holds** |
| 13 | Runs in `australiacentral` | `deploy/terraform.tfvars:24-28`: `rg-panacea-platform-dev`, `australiacentral`, `environment = "dev"` | **holds for dev**; AU residency yes |
| 14 | Forge is platform-admin only, not callable with an API key | `routes.go:3261-3273` (`authedAgent`) | **holds** |
| 15 | Promote with a stage ladder and owner approval | Routes exist (`:2814-2860`); 06-deploy §0 item 5 finds the regulated rungs wait on Panacea (secure-test needs Chains signing live, production needs DLP enforcing) | **holds for the innovation rung only** |

**Net for C1:** the capability claims are accurate at source; nothing was invented. What the
report understates is the price of reaching them from our architecture: a locked edge by
default, a full-power year-long token per user, a team-approval step before that token exists,
a preview that shares production data by default, and a sign-in on an unpatched single-replica
Keycloak.

## 3. C1 Panacea: licences in the stack

_Pending._

## 4. C1 Panacea: the auth path from our control plane and sandboxes

_Pending._

## 5. Alternative: Coolify plus Supabase (the owner's named option)

_Pending._

## 6. Alternative: C3 Azure-native managed services for our own state

_Pending._

## 7. Days against our architecture

_Pending._

## 8. Corrected recommendation

_Pending._

## 9. Not verified

_Pending._

## 10. Sources

_Pending._
