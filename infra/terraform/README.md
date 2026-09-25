# infra/terraform — PanaceaApp's own environment

This tree is written and validated, **never planned or applied, until the owner provides the
environment** (`OD-5`). Nothing here targets Panacea: phase 1's root, which attached a node
pool to Panacea's dev cluster, was retired in task 01; the pool is rewritten in `modules/aks`
(task 06).

## Layout

Every Terraform task in phase 8 writes into exactly this:

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

## Commands

The only Terraform commands this phase runs are the ones `validate.sh` runs (the one exception
is task 07's by-hand `terraform graph` of the foundation stack: config only).

```bash
bash infra/scripts/validate.sh                         # every dir under modules/ and stacks/
bash infra/scripts/validate.sh infra/terraform/modules/aks   # one directory
bash infra/scripts/guards.sh                           # the ten rules below
bash infra/scripts/guards_test.sh                      # one fixture per rule
```

`validate.sh` runs, per directory: `fmt -check -recursive -diff`, `init -backend=false`
(stacks add `-lockfile=readonly`), `validate`, and `terraform test` when the directory has a
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
| G1 | No `terraform.tfvars` or `*.auto.tfvars` (or their `.json` forms) anywhere; no `*.tf.json` or `*.tfvars.json` configuration at all (the other rules read HCL only; the two JSON override names are left to the next clause); and no override file (`override.tf`, `override.tf.json`, `*_override.tf`, `*_override.tf.json`) tracked by git. Runbook §2's local-backend `backend_override.tf` stays on disk, untracked. Outside a git work tree G1 fails. Non-Terraform JSON, such as `infra/rbac/*.json`, passes. |
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
second dev pass. `terraform fmt` also leaves a one-argument block on one line, and a line-based
check cannot see a key on the opener line, so one-line file-level blocks are refused outright.

## Conventions (every later task assumes these)

- **Variables.** Every variable has a `type` and a `description`; closed sets get a
  `validation`. Nothing environment-specific (location, CIDRs, names) has a `default`:
  `<env>.tfvars` sets it. `environment` is validated against `["dev", "prod"]`.
- **Tags.** Every stack has
  `locals { tags = merge(var.tags, { product = "panacea-app", environment = var.environment, stack = "<stack>", managed-by = "terraform" }) }`.
  Modules take `tags` as an input.
- **Names.** `<kind>-panacea-app-<env>[-<qualifier>]`, e.g. `rg-panacea-app-dev`,
  `aks-panacea-app-dev`. Globally unique names (storage, ACR, Key Vault) drop the hyphens and
  append `var.unique_suffix`: four lowercase alphanumerics, validated by `^[a-z0-9]{4}$` and set
  in `<env>.tfvars` (e.g. `stpanaceaappdevk7q2`). The suffix is chosen once per environment and
  never derived from an id.
- **Lock files.** A stack's `.terraform.lock.hcl` is created with
  `terraform providers lock -platform=windows_amd64 -platform=linux_amd64`, so the Linux CI
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
  the value `"none"` were verified in task 02: `terraform validate` accepts both against
  azurerm 4.81.0, with no warning. Modules hold no provider blocks.
- **Backends.** A stack's `backend.tf` is exactly `terraform { backend "azurerm" {} }`: partial
  configuration, completed by `backends/<env>.hcl` (task 02) at init time in the pipeline.
- **Line endings.** `.gitattributes` pins `eol=lf` for Terraform files and scripts.
- **Ids and secrets.** Subscription, tenant and object ids arrive as `TF_VAR_*` / `ARM_*`.
  No secret is ever set in Terraform: identities are Entra-only.
