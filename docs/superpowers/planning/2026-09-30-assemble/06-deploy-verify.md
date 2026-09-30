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

*(to be filled)*

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
