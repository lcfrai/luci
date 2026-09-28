# infra/terraform — PanaceaApp's own environment

This tree is written and validated, **never planned or applied, until the owner provides the
environment** (`OD-5`). Nothing here targets Panacea: phase 1's root, which attached a node
pool to Panacea's dev cluster, was retired in task 01; the pool is rewritten in `modules/aks`
(task 06).

## The engine is OpenTofu, not Terraform (`OD-32`)

**Run `tofu`. Never run the `terraform` binary.** HashiCorp Terraform has been **BUSL-1.1 since August
2023**, and this product is shipped to customers who run it on their own cloud, so BUSL is on
the forbidden list exactly as AGPL is. **OpenTofu (MPL-2.0) is the fork we use.** The same
ruling replaces HashiCorp Vault (BUSL-1.1 from 1.15) with **OpenBao** (MPL-2.0) — see
"Secrets" below for why that clause changes nothing here.

This section exists so that nobody, finding a directory named `terraform/` and a `terraform {}`
block, helpfully switches back. The directory name is a path, not a licence.

**What does NOT change, verified by reading this tree on 2026-09-28:**

- **The providers.** `hashicorp/terraform-provider-azurerm` is **MPL-2.0**, with no Change Date
  and no BUSL heading. Only the engine moved. Every `required_providers` entry here is the
  unqualified `source = "hashicorp/azurerm"`, so it resolves through whichever registry the
  engine defaults to and needs no edit.
- **The language.** `.tf` syntax is compatible. The `terraform { … }` block keeps its name;
  OpenTofu accepts it. So do the `terraform_remote_state` data source, the auto-loaded
  `terraform.tfvars` / `*.auto.tfvars` names that G1 refuses, and `.terraform/` and
  `.terraform.lock.hcl` (training knowledge, **UNVERIFIED here**: no `tofu` binary exists on
  the build host, so nothing below was run).

**What does change:** the CLI invocation (`terraform …` → `tofu …`) in every script, pipeline
and runbook; the tool pinned and installed in CI; and the registry host recorded inside a lock
file, which `tofu` writes as `registry.opentofu.org/…` where the old engine wrote
`registry.terraform.io/…`.

**The CI pin is CLOSED, 2026-09-28.** `pipelines/scripts/install-tool.sh` pins OpenTofu 1.12.6
for `linux-amd64` and `windows-amd64`, and `pipelines/infra.yml` asks for `tools: [tofu]`. Both
SHA-256 values are the ones published in `tofu_1.12.6_SHA256SUMS` on the OpenTofu release, and
both archives were downloaded and re-hashed here: the sums matched, the member is `tofu` /
`tofu.exe` at the archive root, and the archive's own `LICENSE` is Mozilla Public License 2.0.
`pipelines/scripts/lint.sh` passes 8 of 8, which is what checks that a requested tool has a pin
(R2) and that every pin carries 64 hex digits (R6).

**One item is NOT closed, because closing it needs a tool this host must not install.**
It is recorded rather than guessed, and may not be closed by inventing a value:

1. **The tracked lock file.** `stacks/bootstrap/.terraform.lock.hcl` still records
   `registry.terraform.io/hashicorp/azurerm`. Regenerate it, do not hand-edit it — the hashes
   are per-registry and per-archive, so a hand-written entry is a fabricated integrity check:
   `tofu providers lock -platform=windows_amd64 -platform=linux_amd64`. Until then guard G9
   strips both registry hosts; once the file is regenerated, narrow G9 to the OpenTofu host so
   a Terraform-written lock file fails the guard by itself. **This is not cosmetic:** until the file is regenerated, the first `tofu init -lockfile=readonly` on this stack (`validate.sh`'s stack path, `infra/scripts/validate.sh:10,117`, and task 30's `tf-stack.sh`) is expected to fail, because the lock holds no `registry.opentofu.org/hashicorp/azurerm` entry (training knowledge of how OpenTofu keys a lock file, **UNVERIFIED**: no `tofu` binary has run here).

## Secrets: Azure Key Vault, not HashiCorp Vault, so OpenBao is deferred

Every `vault` identifier in this tree's `.tf` files is **Azure Key Vault** — `azurerm_key_vault`, `modules/keyvault`, the
`privatelink.vaultcore.azure.net` zone, and the `vault` private-endpoint subresource. Key Vault
is an Azure service, not a HashiCorp dependency, and has no licence problem. **Phase 8 plans no
HashiCorp Vault anywhere**, so `OD-32`'s Vault→OpenBao clause deletes nothing here; it binds the
future, and the OSS research rates OpenBao "conditional — defer: only if Key Vault is
insufficient". **Do not rename anything matching `key_vault` or `keyvault`** — those are Azure
provider identifiers and renaming them breaks the plan.

## Layout

Every OpenTofu task in phase 8 writes into exactly this:

```
infra/terraform/
  README.md
  backends/<env>.hcl                 task 02
  modules/<name>/                    versions.tf main.tf variables.tf outputs.tf tests/*.tftest.hcl
                                     no provider blocks, no backend, no lock file committed
  stacks/<name>/                     versions.tf backend.tf providers.tf main.tf variables.tf
                                     outputs.tf <env>.tfvars .terraform.lock.hcl tests/*.tftest.hcl
infra/scripts/validate.sh  guards.sh  guards_test.sh
```

## Stack order

```
bootstrap → foundation → data → identity → workloads
```

That is the only direction data flows. A later stack reads an earlier one's outputs with
`terraform_remote_state`; an earlier stack never reads a later one.

One exception, declared (task 02 fix round 2). Bootstrap's optional `state_private_endpoint`
takes two of foundation's ids, `subnet_ids.private_endpoints` and `private_dns_zone_ids.blob`.
The operator copies them into `TF_VAR_state_private_endpoint` on a bootstrap pass after
foundation exists (task 31's prod section). Bootstrap never reads them through
`terraform_remote_state`, and its first apply never needs them. The price is a cycle: once
public access is off, every stack's state, foundation's included, is reached through an
endpoint on foundation's subnet, resolved by a record in foundation's `blob` zone. So:
- while that endpoint exists, foundation never replaces or destroys that subnet or that zone
  (`prevent_destroy`, tasks 03 and 07);
- tearing an environment down starts with a bootstrap pass that turns public access back on
  and drops the endpoint.

## Commands

The only OpenTofu commands this phase runs are the ones `validate.sh` runs (the one exception
is task 07's by-hand `tofu graph` of the foundation stack: config only).

```bash
bash infra/scripts/validate.sh                         # every dir under modules/ and stacks/
bash infra/scripts/validate.sh infra/terraform/modules/aks   # one directory
bash infra/scripts/guards.sh                           # the ten rules below
bash infra/scripts/guards_test.sh                      # one fixture per rule
```

`validate.sh` runs, per directory: `fmt -check -recursive -diff`, `init -backend=false`
(stacks add `-lockfile=readonly`), `validate`, and `tofu test` when the directory has a
`*.tftest.hcl` (in `tests/` or at its root). A stack's tests run once per `<env>.tfvars`, with
`-var-file`, so every committed tfvars file is proven to plan against mocks. It never runs plan,
apply, import, state or console, never passes `-backend-config`, scrubs `ARM_*`, the Azure SDK
credential variables and `TF_CLI_ARGS*` from its own process tree, and points
`AZURE_CONFIG_DIR` at a fresh empty directory, so no `az login` context is reachable: a test
that forgot a mock fails instead of reading the operator's subscription.

The provider start-up dominates: about 100 seconds for three test runs on the build laptop.
Run `validate.sh` one directory at a time, in the background when a directory has many runs.

## The ten rules (`guards.sh`)

**These guards are hygiene, not a security boundary.** They run from the branch being built, so
a pull request can edit or delete them. What stops a hostile pull request is outside the
repository: the read-only plan identity and the branch control on the write connections
(task 30). The rules read HCL text and are fail-closed: a comment that spells a forbidden
attribute fails too, and is reworded. The rules share one `find | sort` inventory and one `awk`
scan; if either fails, `guards.sh` prints `guards: inventory failed (…)` or
`guards: scanner failed (awk exit <n>)` and exits 2 before any rule reports.

| Id | Rule |
|---|---|
| G1 | No `terraform.tfvars` or `*.auto.tfvars` (or their `.json` forms) anywhere; no `*.tf.json` or `*.tfvars.json` configuration at all (the other rules read HCL only; the two JSON override names are left to the next clause); and no override file (`override.tf`, `override.tf.json`, `*_override.tf`, `*_override.tf.json`) tracked by git. Runbook §2's local-backend `backend_override.tf` stays on disk, untracked. Outside a git work tree G1 fails. JSON that is not OpenTofu configuration, such as `infra/rbac/*.json`, passes. |
| G2 | No `*.tfvars` contains a GUID: ids arrive as `TF_VAR_*` / `ARM_*`. |
| G3 | Every `*.tftest.hcl` (in `tests/` or at the module or stack root) has a `mock_provider` for every provider its module or stack requires. |
| G4 | No `provider "kubernetes"` or `provider "helm"`, and no `hashicorp/kubernetes` or `hashicorp/helm` source (P8-D3: charts deploy the cluster). |
| G5 | Every `stacks/<x>/backend.tf` holds exactly `terraform { backend "azurerm" {} }`, and a stack with no `backend.tf` fails: `init -backend-config=<file>` without a backend block exits 0 and uses local state. |
| G6 | No `provisioner "local-exec"` or `"remote-exec"`. |
| G7 | Every stack has a `.terraform.lock.hcl` tracked by git. |
| G8 | No `administrator_password`, `administrator_password_wo`, `client_secret` or `admin_password` attribute in any `.tf` (P8-D10, P8-D11). |
| G9 | Every provider source, in a `required_providers` entry (an entry with no source counts as `hashicorp/<name>`) or in a lock file, is `hashicorp/azurerm`, `hashicorp/azuread` or `azure/azapi`. |
| G10 | In every stack `*.tftest.hcl` (in `tests/` or at the stack root), the file-level `variables` block holds no key any `stacks/<x>/*.tfvars` sets, and is never written on one line. |

Why G10: a file-level `variables` block in a test file **overrides** `-var-file` (Terraform
1.15.8), so a file-level `environment = "dev"` would quietly turn the `prod.tfvars` pass into a
second dev pass. `tofu fmt` also leaves a one-argument block on one line, and a line-based
check cannot see a key on the opener line, so one-line file-level blocks are refused outright.

## Conventions (every later task assumes these)

- **Variables.** Every variable has a `type` and a `description`; closed sets get a
  `validation`. Nothing environment-specific (location, CIDRs, names) has a `default`:
  `<env>.tfvars` sets it. `environment` is validated against `["dev", "prod"]`.
- **Tags.** Every stack has
  `locals { tags = merge(var.tags, { product = "panacea-app", environment = var.environment, stack = "<stack>", managed-by = "opentofu" }) }`.
  Modules take `tags` as an input.
- **Names.** `<kind>-panacea-app-<env>[-<qualifier>]`, e.g. `rg-panacea-app-dev`,
  `aks-panacea-app-dev`. Globally unique names (storage, ACR, Key Vault) drop the hyphens and
  append `var.unique_suffix`: four lowercase alphanumerics, validated by `^[a-z0-9]{4}$` and set
  in `<env>.tfvars` (e.g. `stpanaceaappdevk7q2`). The suffix is chosen once per environment and
  never derived from an id.
- **Lock files.** A stack's `.terraform.lock.hcl` is created with
  `tofu providers lock -platform=windows_amd64 -platform=linux_amd64`, so the Linux CI
  agent and the Windows laptop verify against the same committed file. Modules commit no lock
  file (git-ignored).
- **Test variables.** A stack test's file-level `variables` block holds only what no
  `<env>.tfvars` sets (ids, as `TF_VAR_*` would supply). Every run that depends on an
  environment-specific value, `environment` included, sets it in its own run-level
  `variables` block. Each stack's test file has a `committed_tfvars_plan` run
  (`command = plan`) that sets none of them, so each `-var-file` pass plans its own tfvars.
- **Providers.** Every stack's `provider "azurerm"` block sets `storage_use_azuread = true` and
  `resource_provider_registrations = "none"`: the read-only plan identity cannot register
  resource providers, so the owner registers them by hand (task 31 §1). The attribute name and
  the value `"none"` were verified in task 02: `tofu validate` accepts both against
  azurerm 4.81.0, with no warning. Modules hold no provider blocks.
- **Backends.** A stack's `backend.tf` is exactly `terraform { backend "azurerm" {} }`: partial
  configuration, completed by `backends/<env>.hcl` (task 02) at init time in the pipeline.
- **Line endings.** `.gitattributes` pins `eol=lf` for OpenTofu files (`*.tf`, `*.tfvars`, `*.hcl`, `*.tftest.hcl`) and scripts.
- **Ids and secrets.** Subscription, tenant and object ids arrive as `TF_VAR_*` / `ARM_*`.
  No secret is ever set in OpenTofu configuration: identities are Entra-only.
