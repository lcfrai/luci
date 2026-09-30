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
routes. It has **no evidence export, no sanitised preview data and no MCP server**. Those arrive
with the Central→East cutover, which is being built now (§2.4). So v1 targets **the dev estate**,
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
| Chains signs with `hashivault://panacea-release`, transparency off | `kubernetes/tekton-chains/chains-config.yaml:35-80`: `slsa/v2alpha4`, OCI storage, a KMS signer | **config holds**. The key lives in Vault, and the dev Vault is Shamir-sealed after every restart (§2.4) |
| "Tenant namespace isolation" is better than Coolify's | `tenant_ensure.go` gives a default-deny NetworkPolicy baseline. `pod_security.go` enforces PSA **baseline** after a live pentest finding: a `hostNetwork` pod lifted the node's identity token. **No `runtimeClassName` anywhere** in Panacea's tracked Go, YAML or Terraform | **partly holds**: shipped apps get hardened containers on a shared kernel (runc), not microVMs |
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

*(to be filled)*

### 2.4 Maintenance and live state

*(to be filled)*

### 2.5 Are 6 days realistic for our architecture?

*(to be filled)*

## 3. The signing glue: cosign + in-toto Statement v1 + Azure Key Vault

*(to be filled)*

## 4. Alternative 1: Coolify + Supabase + Panacea snippets

*(to be filled)*

## 5. Alternative 2

*(to be filled)*

## 6. Orca as the surface deploy shows up in

*(to be filled)*

## 7. The corrected recommendation

*(to be filled)*

## 8. What was not verified

*(to be filled)*

## 9. Sources

*(to be filled)*
