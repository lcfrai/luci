# 06 — Deploy and release provenance: the sceptic's verification

**Status:** IN PROGRESS (skeleton). Sections are filled one at a time and committed after each.
**Date:** 2026-09-30. **Verifies:** `06-deploy.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation and its two best alternatives at source:
licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run. `Projects/Panacea` was read with grep and cat only.

## 0. Verdict first

*(to be filled)*

## 1. What was checked, and how

*(to be filled)*

## 2. The recommendation: Panacea as the deploy step

### 2.1 Ownership and licence at source

- **Panacea itself:** KPMG's own code, no `LICENSE` file at the repo root. It holds for v1:
  nothing of Panacea is shipped while it stays a separate system (`OD-5`, `OD-13`).
- **The licence line is incomplete.** The researcher names only Vault. Panacea's own manifests
  (`git grep` on HEAD, `kubernetes/`, `deploy/`, `pipelines/`) deploy **five restricted
  components**, not one:

| Component, as Panacea deploys it | Licence at source | What it forces | Permissive route |
|---|---|---|---|
| `hashicorp/vault:1.17.2` (holds the Tekton Chains signing key) | BUSL-1.1 | no hosted "competitive offering"; internal use is fine | OpenBao, MPL-2.0 (`OD-32`) |
| `minio/minio` (19 references; Argo chart `minio`). The cutover dossier says it holds all files and all DB backups, preview-branch backups included | **AGPL-3.0, and upstream is archived**: GitHub API `archived: true`, last push 2026-04-24 | a modified copy offered over a network must offer its source; no upstream security fixes | the cutover moves to `pgsty/silo`, a MinIO fork: **still AGPL-3.0** (API: 3.6k stars, pushed 2026-09-29). Azure Blob, already the cutover's backup target, is the permissive route |
| Grafana, in `kube-prometheus-stack` (`grafana.enabled: true` in `prometheus-values.yaml:186-187`, off in the dev values) | AGPL-3.0 | as above | Azure Managed Grafana, or Perses (Apache-2.0) |
| ClamAV (`clamav/clamav`, a scan in the catalogue) | GPL-2.0 | a copy handed to a customer comes with its source; running it is free | acceptable as a separate program |
| `redis` (mostly `redis:7.2.4`, one `redis:7.4` reference, a Bitnami chart) | 7.2.4 is BSD-3; 7.4 and later are RSALv2/SSPLv1 | SSPL would require releasing the source of the whole service stack | pin 7.2.x or move to Valkey (BSD-3) |

- **None of it blocks v1.** In our own environment each runs unmodified as an internal service,
  and running is not distributing. **It does block the customer-hosted tier.** That tier is the
  lifted Panacea (`CLAUDE.md`), and there every row above is a restricted component deployed into
  a customer's tenancy. Swapping them is **Panacea-lift work, not phase 7 work**. It still has to
  be priced before anyone promises a customer-hosted date.
- **The scanners' own terms were not read**, apart from the engines: Semgrep's registry rules,
  Nuclei templates and ZAP add-ons have their own terms (§8).

### 2.2 The claimed capabilities, checked against Panacea's source

Read in `Projects/Panacea` on 2026-09-30 with `git show`, `git grep`, `grep` and `sed -n`. Nothing
was run. The working tree is on branch `finish-dev-outstanding` at `eee94cd94`, one docs commit
past the researcher's `efc637879`. **The researcher read a working tree, so the first question
is which estate runs that code.**

**Finding 1: the features exist on the dev line only.** The table below comes from
`git cat-file -e <ref>:<path>` and a route grep per ref.

| Ref | What it is (Panacea's notes, 2026-09-21 and 09-28) | Routes | Evidence export | D10 redaction (`branch_data_ops.go`) | `preview_data.go` | `panacea-mcp` |
|---|---|---|---|---|---|---|
| `ado-gov/dev` `ba17a82d3` (09-28) | the line deployed to `dev.panacea.kpmg` | 586 | yes | yes | yes | yes, 31 tools |
| `ado-gov/main` `c9cf66e09` (09-21) | promoted, but **"Production is NOT deployed"**: `azure-pipelines-main.yml` is `trigger: none` | — | yes | yes | yes | yes |
| `kpmg/main` `bf958d0b2` (08-29) | what production runs: **api v1.108.16** (Panacea's 09-28 incident note) | 482 | **no** | **no** | **no** | **no** (no `services/panacea-mcp`) |

Production, `panacea.kpmg`, has previews, share links, env promote, rollback and the stage
routes. It has **no evidence export, no sanitised preview data and no MCP server**. Those reach
production with the Central→East cutover, which is being built now (§2.4). The dev line cannot
be pushed to production's GitHub remote: nine Argo CD apps self-heal from `kpmg/main` there
(the owner's 09-21 note). So v1 targets **the dev estate**,
or it waits for the cutover. That is an owner call (§7, call 1).

**Finding 2: each claim, at source.**

| Claim (06-deploy) | At source | Verdict |
|---|---|---|
| Route lines `:1024-1028`, `:1162-1164`, `:1286`, `:1291`, `:1308-1309`, `:1424-1442`, `:2720`, `:2814-2862` | every line matches, verb and path, on HEAD | **holds** |
| 31 `panacea-mcp` tools | 31 `name: "panacea_…"` on HEAD and `ado-gov/dev`. Added `65bfc12c3`, 2026-09-08 | **holds, dev line only** |
| Stage ladder; production unreachable until DLP; stage floor a no-op | `deployment_stages.go:40-60` (rank constants, the `dlp-label` block gate) and `:1600-1610` (the 200 no-op) | **holds** |
| DB branch per preview, with honest degrade | `db_branch_ops.go:36-45`: opt-in label; degrades to the shared DB on the shared tier, with no backup store, or **when the object store is unreachable**. The object store is MinIO (§2.1) | **holds**, but only on the dedicated tier, and it depends on MinIO |
| 18-check scan catalogue and posture gate | real in code. But `STATUS.md:131-140`: *"LIVE (opt-in, defaults off/warn)"*. Falco and trivy-operator are *"pending ACR image/chart mirroring"*. VVAH was retired on 2026-09-23 | **overstated**: the gate exists, and its defaults do not block |
| Kyverno verifies signatures | in **Audit** (`STATUS.md:136-138`). Gatekeeper's constraints are *"in DRYRUN on the dev cluster"* (`clients/pod_security.go` header) | **holds as audit only** |
| Chains signs with `hashivault://panacea-release`, transparency off | `kubernetes/tekton-chains/chains-config.yaml:35-80`: `slsa/v2alpha4`, OCI storage, a KMS signer | **config holds**. The key lives in Vault, and `VAULT_AUTO_UNSEAL="false"` on both live estates (`panacea-dev.env:136`, `panacea-sandbox.env:78`; `"true"` only on East, `panacea-aue.env:139`). So Vault comes back sealed after a restart, and Chains cannot sign until someone unseals it by hand (§2.4) |
| "Tenant namespace isolation" is better than Coolify's | `tenant_ensure.go` gives a default-deny NetworkPolicy baseline. `pod_security.go` enforces PSA **baseline**. It was added after live pentest finding #3: a tenant `hostNetwork` pod could reach the node's metadata endpoint and lift its identity token. **No `runtimeClassName` anywhere** in Panacea's tracked Go, YAML or Terraform | **partly holds**: shipped apps get hardened containers on a shared kernel (runc), not microVMs |
| P-3: our realm token is refused | `middleware/auth.go:147-200`: `pnc_` keys, a Backstage identity JWT behind `BACKEND_SECRET`, or asserted headers. No foreign bearer is accepted | **holds** |
| "Until P-3, a user pastes an unscoped `pnc_`" | **scoped keys exist today**: `scopeProject` plus `scopeRole` = `read` or `deploy`; a 90-day default and a 365-day cap (`handlers/api_keys.go:34-39`, `:68-100`, `:147-159`). Minting one needs an SSO browser session (`:112-116`) | **corrected**: one project-scoped key per project is possible now. Project **creation** still needs an unscoped key |
| Single Keycloak replica | `kubernetes/argocd-apps/platform/keycloak.yaml`: `replicas: 1` | **holds** |
| GitHub and Azure DevOps sources | `clients/pipeline.go:733` (`github` or `azuredevops`), `handlers/webhook_ado.go`, `clients/gitpush/azuredevops.go` | **holds**. Two limits: a repo host allowlist, and a **shared platform PAT** that only reaches KPMG's own orgs. A customer-org repo needs a per-project token (`clients/repo_allowlist.go:11-43`) |

**Finding 3: something the glue never does.** Phase 7 **binds to an existing** Panacea project by
`repoUrl` (plan lines 139 and 290). It never creates one. `POST /api/v1/projects` (`routes.go:1017`,
`authedGrant`) takes `labels`, `team`, `jobCode`, `resourceTier` and `gitToken`
(`models/models.go:639-663`). So today each PanaceaApp project needs a hand-made Panacea project
in the Portal first. Creating it from our side is new glue (§2.5). It also sets the data class at
creation, as a label (`handlers/projects.go:360-374` passes labels through). That shrinks P-2 to
"re-classify later", because `OD-24` sets the class at creation.

### 2.3 Azure, Entra through Keycloak, AU residency

- **Azure: holds.** Both estates run on AKS.
- **Region: corrected.** The researcher says Australia East. Both live estates are in
  **Australia Central**: `panacea-dev.env` sets `LOCATION="australiacentral"`, commented "ISM
  sovereignty", and so does `panacea-sandbox.env`, the current production. Only the prepared
  cutover target, `panacea-aue.env`, is `australiaeast`, commented "3 AZs (vs Central's none)".
  **AU residency holds either way.** High availability does not: Central has no availability
  zones.
- **Entra through Keycloak: holds, with one limit.** Our desktop signs in against Panacea's dev
  realm, `https://auth.dev.panacea.kpmg/realms/panacea` (identity ruling `:94`, `:104`). The same
  realm's tokens are **not** accepted by Panacea's REST API (§2.2, P-3). Our control plane
  therefore holds a `pnc_` key per user.
- **Reachability: missed, and customer-visible.** Panacea's public edge is **one** Traefik load
  balancer. Its source-IP allowlist holds **14 KPMG AU corporate egress /32s**, enforced in the
  node NSG below HTTP, so it cannot tell one hostname from another (`deploy/main.tf:578-604`;
  Panacea's 2026-08-10 and 08-11 notes). Everything behind it is covered: the Portal, **every
  tenant app**, Pages and webhooks. The East estate keeps it: "D2 East clusters MATCH DEV
  EXACTLY (public API + 14-IP allowlist…)".
  1. **A shipped app opens only from KPMG's network.** That is fine for KPMG-staff pilots. For a
     customer's own users it means a second public entrypoint, which is a Panacea edge change and
     a sovereignty call (Front Door is global).
  2. **Adding our egress IP (the researcher's risk 5) opens the whole edge to it,** tenant apps
     included, not only the API.

### 2.4 Maintenance and live state

- **Activity: very high, one person.** HEAD has 1,255 commits since 2026-08-31, and every one is
  under the owner's name (`git shortlog`). The work is fast and agent-driven, and the bus factor
  is one human. There is no product company behind it, which cuts both ways: we can change it,
  and we must staff it.
- **Live state.** Not observed by this study. The sources are Panacea's `docs/cutover/RESUME.md`
  (2026-09-30 morning) and the owner's session notes (dated). They say more than `STATUS.md`:
  1. **Production is mid-incident and frozen.** On 2026-09-28 every cert-manager renewal had been
     failing since mid-August, from a CoreDNS SERVFAIL. A break-glass patch renewed all 18 certs.
     The fix is **not merged to `kpmg/main`**, so Argo CD reverts it when it reconnects, and Argo CD
     "can't sync anything on prod". RESUME: "prod stays code/doc only (off limits)".
  2. **Functions are frozen** at tag `panacea-v2-freeze-2026-09-24` for the Central→East cutover. RESUME's table
     runs from SL-0 to SL-12, all being built in parallel now. The owner's 09-24 note sizes the
     plan at about 72.5 engineer-days. **P-1, P-2 and P-3 are new
     features, so they queue behind a freeze.** The 06 report treats them as "0.5-2 days in
     Panacea" with no date.
  3. **Tenant-isolation holes are still being found on dev.** This week's rounds found MinIO
     CVE-2026-41145 and -40344, which allow cross-tenant writes because Panacea's keys are
     sha256 of the project name. They also found a "cross-tenant bucket takeover" in a fix, a
     "cross-project token write via the Env Vars editor", and a host-ambiguity "lockout" class.
     All were found, fixed or in round 3 by 06:35 today. That is diligent review. It is also not
     yet "the regulated half, done".
  4. **Vault seals on restart** on both live estates (§2.2). RESUME's infra stream exists to "make
     sure Vault is consistently readable". Chains' signing key sits there, and so do tenant env
     vars.
  5. **Resilience gaps**, from the 2026-09-24 cutover dossier: "Keycloak/Vault/Flagsmith have NO
     backup and MinIO holds all files + all DB backups on one disk." With a single Keycloak
     replica, that is the sign-in for our desktop too, until phase 8.
- **What survives:** none of this is a reason to pick another product. Coolify would start at
  zero on every one of these axes (§4). It is a reason to **date** the Panacea-side asks against
  the cutover, and to ship v1 as pilots on the dev estate.

### 2.5 Are 6 days realistic for our architecture?

**No. About 8.5 (7.5 to 9.5), or about 5 with two people.** Each row was checked against our
code (`feat/hub`, the phase 7 plan's task table at lines 636-655) and Panacea's source.

| Glue | 06 says | At source | Corrected |
|---|---|---|---|
| G1 Orca Deploy panel | 1.5 | **The plugin route holds.** Panels are sandboxed frames, and the worker is unsandboxed Node that can reach our backend today (02-verify §3.3), so no fork is needed. **But the panel sends users into Panacea's Portal** for logs, scans, governance and approval. That fails the owner's ask that the product feel like one app (§6). Bringing those screens in-app is phase 7 tasks 11, 12, 17 and 18: **10 plan-hours**, about 1.4 days, not the "2-3 more days" in 06 §7. Add 0.25 for an in-app approve act: `POST …/stage/approve` is owner-gated (`routes.go:2860`), so an owner can approve from our panel with their own key | **about 2.75** |
| G2 signed provenance | 1 | The Hub is Go and has no signing code (`git grep` on `feat/hub`: no sigstore, cosign, Key Vault or DSSE). **The evidence document is not stable.** Every export stamps `exportedAt` and `actor` (`evidence_export.go:135-137`, `:243`). Its audit-chain block carries `checked` and `lastId`, which grow with every audit row (`audit_integrity.go:40-52`). So the "sha256 of Panacea's evidence document" matches one fetch only. The Hub must **store the exact bytes it hashed**, which contradicts 06's "bound in without being copied", or no assessor can re-derive the digest | **about 1.25** |
| G3 promotion by class | 0.75 | holds: `POST …/stage/promote` is developer-gated (`routes.go:2818`), so a user's key can request a rung. `GET …/stage` (`:2814`) feeds the checklist | 0.75 |
| G4 agent allowlist | 1 | **Feasible as a drop-in.** `panacea-mcp` takes `PANACEA_API_URL` and `PANACEA_TOKEN` from env (`index.ts:32-35`), so the sandbox gets our proxy's address and a session token, never a `pnc_`. **But the rules are not path-only.** "Non-production branch" means reading the deploy body's branch and looking up the project's production branch. "Rollback on non-production" means resolving the deployment id to its environment. `panacea_api_request` must hit the same filter. SSE has to pass through unbuffered. **One route leaves our AI boundary.** `panacea_diagnose` is safe: it is deterministic, and its LLM enricher is nil in production (`diagnose.go:23-24`). But `POST …/pipelines/{run}/analyse` (`routes.go:2291`) sends build logs to **Panacea's own LiteLLM** (`deploy_analysis.go:499`), not our gateway. Panacea's `skill.md:152` tells agents to call it, and `panacea_api_request` reaches it. Its retention and AU path under `OD-19`/`OD-21` are unverified, so the proxy refuses it for `synthetic` and `protected` projects until that is checked | **about 1.5** |
| G5 project creation *(missing from 06)* | 0 | Phase 7 binds an existing Panacea project by `repoUrl` and never creates one (§2.2, finding 3). "Nothing to a shipped application" needs our control plane to call `POST /api/v1/projects` with the labels (data class, `panacea.io/branch-database`), team and job code. Without it, every project starts with a hand-made step in the Portal | **about 0.75** |
| Kept phase 7 tasks (07 rest, 08, 09, 10, 19, 20) | 1.7 | holds at the plan's own rates. They are control-plane tasks, not the Tauri lens. Tasks 09 and 19 run against the Hub fake until H5 | 1.7 |
| **Total** | **about 6** | | **about 8.5** |

**Day 1 is not "day 1" for a user.** Before an agent's first preview deploy, four things have to be
true. Three sit on the Panacea side, and one is a Portal visit per user:
1. Our proxy's egress IP is on Panacea's edge allowlist, which opens the whole edge to it (§2.3).
2. The user has minted a `pnc_` key. Minting needs an SSO browser session in Panacea's UI
   (`api_keys.go:112-116`).
3. A Panacea project exists: either G5, or a hand-made step.
4. The lane is on GitHub. An ADO push goes to Forge's agent-deploy pipeline, not to a preview
   (02-verify §4.1). So the "Azure DevOps and GitHub" row in §2.2 holds for API-started deploys,
   and **fails for webhook previews**.

**Fallback cost.** If IT refuses Orca (02-verify §5.3), the fallback cockpit is phase 2's frozen
Tauri desktop, which has no Deploy lens. Tasks 13-18 come back: 17 plan-hours, about 2.3 days.

## 3. The signing glue: cosign + in-toto Statement v1 + Azure Key Vault

**The choice holds; three things about how G2 is built do not.** Sources: the GitHub API, raw source
files, cosign's v3 flag reference, sigstore's KMS page and the Azure Retail Prices API, all read on
2026-09-30. Panacea's pins were read with grep.

| Claim (06 §3.7, G2) | At source | Verdict |
|---|---|---|
| cosign is Apache-2.0 | API `/license`: `spdx_id: Apache-2.0`, file `LICENSE` | **holds** |
| v3.1.3 of 2026-08-06 fixes GHSA-fx35-mq7g-6g98, so pin at or above it | the release list's latest is v3.1.3 (2026-08-06T01:09Z). Its notes: "Verification bypass via public key in legacy bundle (GHSA-fx35-mq7g-6g98)", backported the same day as v2.6.5. It is a repository advisory: the global advisory API returns 404 | **holds** |
| KMS keys `azurekms://`, `hashivault://`, `openbao://` | sigstore's KMS page lists `awskms`, `gcpkms`, `azurekms`, `hashivault`, `openbao`, `ovhcloud` and `k8s`: "Hashicorp Vault and OpenBao keys can be used" | **holds**, so `OD-32`'s OpenBao swap keeps the signer |
| (auth, not in 06) | the page says `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET` "must be set". The code always builds `azidentity.NewDefaultAzureCredential` (`sigstore/pkg/signature/kms/azure/client.go:120`) | **better than documented**: AKS workload identity works, so the Hub holds no client secret |
| An assessor checks it offline with `cosign verify-blob-attestation` | the v3 reference has `--digest` and `--digestAlg` "instead of providing a blob", `--key` taking a KMS URI or a public key, and `--type <URI>` | **holds**, with two conditions. The assessor needs the bundle file, and has to pass `--insecure-ignore-tlog`, because there is no log entry |
| "No public Sigstore" | true of Chains' config (§2.2). For **our** signer, cosign v3 defaults `--use-signing-config` to true: "a TUF-provided signing config for the service URLs", which means the public-good instance unless we supply our own | **corrected.** G2 either signs in-process with the Go libraries (DSSE plus `sigstore/pkg/signature/kms/azure`), or passes a private signing config with no transparency-log service. Otherwise each release's entry is sent to a public log. Not tested |
| The Statement binds "the sha256 of Panacea's evidence document" | the document is re-stamped at every export (§2.5, G2) | **corrected**: store the hashed bytes beside the Statement |
| Kyverno v1.19.1 of 2026-09-10; `ImageValidatingPolicy`; the #17363 risk | upstream holds: v1.19.1 is the latest release. #17363 was **closed on 2026-09-29**, after that release. **Panacea runs Kyverno v1.12.5** (`kubernetes/argocd-apps/platform/kyverno.yaml:172`, chart 3.2.6), seven minors behind. Its check is a classic `ClusterPolicy` with `verifyImages` in Audit (`base/security-policies/verify-solution-image-signatures.yaml:37`, `:58`, `:102`) | **the version is wrong for our estate.** #17363 does not touch Panacea today. The "Enforce later" flip comes with an upgrade, and that is Panacea's work (P-4) |
| Tekton Chains v0.29.6 of 2026-09-24, Apache-2.0 | v0.29.6 exists (2026-09-24T10:24Z; the API's "latest" marker still points at v0.29.0). `LICENSE` is Apache-2.0. **Panacea runs v0.23.0** (`tekton-chains.yaml:112`) | **holds upstream; six minors behind in Panacea** |
| (price, not in 06) | Key Vault, Australia East **and** Australia Central, USD retail. An HSM-protected advanced (EC) key is **US$5 per key-month**, and advanced-key operations are **US$0.15 per 10,000**. A software key on Standard costs operations only. Managed HSM Standard B1 is **US$3.20 an hour** | **AU residency holds.** One Premium P-256 key costs about **US$5 a month**, well below the owner's money threshold |

**What survives:** the format (in-toto Statement v1 in a DSSE envelope), the signer (a Key Vault key
now, an OpenBao Transit key at the lift) and the offline check are all real and permissive. **G2
costs 1.25 days, not 1**: the in-process signer, the private signing config, and the stored
evidence bytes are each small, and none was in the estimate.

**Formats a customer's assessor meets.** Two different signers sign two different things:
- **Panacea's Chains** signs build provenance with Vault Transit.
- **Our Hub** signs the brief-to-release Statement with Key Vault.

So an assessor needs both public keys. That is fine if the release card says which key signed what.
It is confusing if it does not. The card and the export must carry both key ids (§6).

## 4. Alternative 1: Coolify + Supabase + Panacea snippets

**The rejection holds on merit, but two of its reasons are overstated and one is missing.** Sources:
the GitHub API and the raw `LICENSE` files, Coolify's v4.4-rc.1 notes, and coolify.io's pricing and
preview pages. For Supabase, its branching feature page and its self-hosting page. All were read on
2026-09-30.

| Claim (06 §3.1, 3.9, 3.10) | At source | Verdict |
|---|---|---|
| Coolify is Apache-2.0 | API `spdx_id: Apache-2.0`; `LICENSE` on `v4.x` is the Apache License 2.0 | **holds** |
| Latest stable v4.3.23 (2026-09-18); OIDC only in `v4.4-rc.1` (2026-08-19) | release list: v4.3.23 on 2026-09-18T13:38Z, ten 4.3.x patches in September; `v4.4-rc.1` is `prerelease: true`. Its notes: "first-class OpenID Connect (OIDC) authentication, including discovery, JWKS … PKCE support, and identity linking", plus "automatically join OIDC users to the root team" | **holds.** Keycloak would work directly once 4.4 is stable. The role mapping is coarse (one team), which matters for a regulated tenancy |
| 62,408 stars; one company | 62,409, pushed 2026-09-29, not archived | **holds.** A patch every few days is active, and a lot of churn to pin against |
| Paid tiers | pricing page: self-hosted is "Free Forever … Full access to all features". Cloud is **US$5 a month for 2 servers, plus US$3 per extra server** | **holds: no gate.** Self-hosting gets only "Community support", which is a real gap for a KPMG service |
| "Preview deployments per pull request need the GitHub App" | the preview page: "Manual repository webhooks can trigger previews, but automated pull-request comments require a GitHub App". Only GitHub's setup is documented | **overstated.** A webhook is enough on GitHub. **ADO still has no preview path**, so `OD-4` fails for previews, as 06 says |
| Supabase is Apache-2.0; branching is hosted-only | API `spdx_id: Apache-2.0`, 110,908 stars, pushed 2026-09-30; `LICENSE` is Apache 2.0. supabase.com/features/branching: **"Available on self-hosted: N/A"** | **holds** |
| (missed) Supabase's self-hosted console | self-hosting docs: "Access to Studio (Dashboard) is protected with **HTTP basic authentication**", one `DASHBOARD_PASSWORD` in `.env`. No SSO or SAML is documented for it | **a new fail for a regulated tenancy**: one shared password and no per-user audit. It needs oauth2-proxy in front, which is one more piece of glue |
| "Would still isolate agent-written apps less well" | Coolify: each destination is one Docker network, and databases join it (Coolify's networking docs and discussions, via search). Panacea: namespaces, a default-deny NetworkPolicy, PSA baseline and quotas, on **runc** (§2.2) | **true in degree, not in kind.** Both run shipped apps on a shared kernel. Neither gives them a microVM. Running Kata under Coolify's Docker is **untested**: Coolify documents no per-app runtime setting, and a host-wide default would also wrap its own Traefik, which mounts the Docker socket |
| 16-24 days of ported snippets | the rows follow the size of the Panacea code each would port (06 §9 says so). Four costs are left out: an oauth2-proxy for Studio, HA for a single Coolify instance, backups, and a support contract | **plausible as a floor; call it 18-26** |

**What the stack would do to the "one app" ask.** Coolify's Laravel dashboard and Supabase Studio are
two more consoles, each with its own look, next to Orca. Running both headless through their APIs
is possible, but it is more of our glue, not less. Panacea's Portal is a single console, and §6 keeps
users out of it.

**Trade-off in one line:** permissive and quick to click through, but 18-26 days to reach what
Panacea already runs, with two more consoles, one shared Studio password and still a shared kernel.
**Verdict: reject as the deploy step, upheld.** It stays the reference for how short the Deploy
panel should feel.

## 5. Alternative 2: Azure Container Apps behind a second `deploy.Deployer`

Picked as the second-best because it is the only other candidate that is Azure-native, has an
AU-only option and supports Entra without extra glue (06 §3.3, §4). Argo CD with Kargo needs 10+ days
and reverses Panacea's ADR-021, so it ranks below. Sources: Microsoft Learn (revisions, updated
2026-02-26; authentication, updated 2026-07-08; environments, updated 2026-02-27; billing, updated
2026-03-25) and the Azure Retail Prices API, all read on 2026-09-30.

| Claim (06 §3.3) | At source | Verdict |
|---|---|---|
| Revisions are immutable, a label gives a revision its own URL, 100 inactive revisions are kept, "deployment labels" is preview | verbatim: "A label provides a unique URL that you can use to route traffic to the revision". "By default, you have access to 100 inactive revisions". The mode table: "Deployment labels (preview)". Labels also work in multiple-revision mode, which is not preview | **holds.** A preview per branch works without the preview mode |
| Traffic splitting is promotion; re-activating is rollback | "You can activate a revision at any time". Blue-green and percentage splits are documented | **holds** |
| Built-in Entra authentication | Easy Auth lists Microsoft Entra ID **and "Any OpenID Connect provider"** | **holds, and better**: Keycloak can sit in front directly. The Entra path needs one Entra app registration per app, which is an IT ticket each time. The OIDC path through our realm needs none |
| Billing, and a free grant of 180,000 vCPU-s, 360,000 GiB-s and 2M requests | verbatim on the billing page. Retail prices in Australia East: US$0.000034 per active vCPU-second, US$0.000004 per GiB-second, US$0.40 per million requests. **Dedicated plan management is US$0.10 an hour per environment** (about US$73 a month) | **holds.** Price basis: per second of allocation, plus requests |
| Australia East and Southeast | meters exist in **Australia East, Southeast and Central** | **holds**, and wider than 06 says |
| (missed) Isolation of agent-written apps | "A Container Apps environment is a secure boundary". Apps in one environment "share the same virtual network". Separate environments are needed to "never share the same compute resources". No VM-level isolation is documented for ordinary apps. Hyper-V is documented only for dynamic sessions | **one environment per project** is the boundary, at about US$73 a month each on the Dedicated plan. It is no stronger in kind than Panacea's |
| 3-5 days for a second `Deployer` | the seam and its conformance suite exist on `feat/phase-7-deploy` (tasks 01-06). Revisions, labels and traffic are ARM resource properties (`properties.template`, `properties.configuration`), so any ARM client can drive them. The Go SDK itself was not read. **But 3-5 days buys N1 and N3 only.** N2's scan gate, the data-class ladder and N4's evidence are ours to build, at roughly the sizes 06 §3.9 prices for Coolify (scans 3-5, gate 4-6, evidence 2-3) | **about 12-19 days as a whole deploy step** |

**Cohesion, the one thing it does best.** ACA has no user-facing console of its own, so every screen
would be ours, which is exactly what the owner asked for. It loses anyway, because the regulated
half (scans by data class, the ladder, evidence) would all be new code.

**Trade-off in one line:** the cleanest Azure runtime, with Entra or Keycloak built in and AU-only,
but it leaves the regulated half to us, at about 12-19 days.
**Verdict: reference, upheld.** Revisit only if a customer refuses a cluster and accepts that we own
the gate and the evidence.

## 6. Orca as the surface deploy shows up in

*(to be filled)*

## 7. The corrected recommendation

*(to be filled)*

## 8. What was not verified

*(to be filled)*

## 9. Sources

*(to be filled)*
