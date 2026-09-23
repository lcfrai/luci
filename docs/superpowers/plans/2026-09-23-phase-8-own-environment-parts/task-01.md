### Task 01: The `infra/terraform` layout, its conventions, and the two scripts that enforce them

`infra/terraform/` today is phase 1's root: one node pool attached to **Panacea's** dev cluster
(`infra/terraform/kata.tf:4-6`, `variables.tf:1-4`, and `dev.tfvars:4`, whose comment supplies the
id from `az aks show -g rg-aks-panacea-dev -n aks-panacea-dev`). OD-5 makes that target obsolete:
the pool belongs in our own cluster (survey §3.1, `:342-344`). This task retires the root and puts
in its place the skeleton every later Terraform task writes into, plus the two scripts that make
the rules mechanical rather than remembered.

**Verified before this plan was written (2026-09-23, scratch directory, not this repo):**
`terraform init -backend=false` downloads `hashicorp/azurerm` 4.81.0, `hashicorp/azuread` 3.9.0
and `azure/azapi` 2.12.0 through the corporate network; and `terraform test` with
`mock_provider "azurerm" {}` / `mock_provider "azuread" {}` passes a `command = plan` run, an
`expect_failures = [var.name]` run and a `command = apply` run **with no Azure credential** — the
basis for every Terraform test in this plan. The three runs took about **100 seconds** on the build
laptop (provider start-up dominates), so `validate.sh` takes one directory at a time and a caller
runs it per directory, in the background when a directory has many runs.

**Files:**
- `infra/terraform/kata.tf`, `providers.tf`, `variables.tf`, `dev.tfvars`, `.terraform.lock.hcl` — deleted with `git rm` (the pool is re-written in `modules/aks`, task 06)
- `infra/terraform/README.md` — the stack order, the rules, the commands (new)
- `infra/scripts/validate.sh` — fmt, init, validate, test for one directory or all (new)
- `infra/scripts/guards.sh` — the eight repository rules as named checks (new)
- `infra/scripts/guards_test.sh` — one fixture tree per rule, each a mutant guards.sh must kill (new)
- `.gitattributes` — `eol=lf` for `*.tf`, `*.tfvars`, `*.hcl`, `*.tftest.hcl`, `*.sh`, `*.yaml`, `*.yml`, `*.tpl` (new)
- `.gitignore` — adds `*.tfplan`, `crash*.log`, `infra/terraform/modules/**/.terraform.lock.hcl` (modify)

#### The layout (every later task writes into exactly this)

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

Stack order, and the only direction data flows: `bootstrap → foundation → data → identity →
workloads`. A later stack reads an earlier one's outputs with `terraform_remote_state`; an
earlier stack never reads a later one.

#### `validate.sh` — the contract

```bash
#!/usr/bin/env bash
# infra/scripts/validate.sh — the ONLY Terraform commands this phase may run.
#
#   infra/scripts/validate.sh                 every dir under modules/ and stacks/
#   infra/scripts/validate.sh <dir> [<dir>…]  just these, relative to the repo root
#
# For each directory, in order, stopping that directory at the first failure:
#   1. terraform fmt -check -recursive -diff
#   2. stacks:  terraform init -backend=false -input=false -lockfile=readonly
#      modules: terraform init -backend=false -input=false
#   3. terraform validate -no-color
#   4. if tests/ has a *.tftest.hcl:
#        modules: terraform test -no-color
#        stacks:  terraform test -no-color -var-file=<env>.tfvars, once per <env>.tfvars in the
#                 stack, so every committed tfvars file is proven to plan against mocks. The
#                 test files supply only what tfvars may not hold (ids, as TF_VAR_* would).
#
# Output, one line per directory:   validate: OK   <dir>
#                                   validate: FAIL <dir> (<step>)
# Last line:                        validate: <n> ok, <m> failed
#                                   validate: nothing to validate yet     (no dirs exist)
# Exit status: 0 iff m == 0.
#
# It never runs plan, apply, import, state or console, never passes -backend-config, and
# unsets ARM_SUBSCRIPTION_ID, ARM_CLIENT_ID, ARM_CLIENT_SECRET, ARM_USE_OIDC and
# ARM_OIDC_TOKEN for its own process tree, so a developer's shell credentials cannot reach a
# provider even if a test forgot a mock (guards.sh G3 is the first line; this is the second).
set -euo pipefail
```

#### `guards.sh` — the eight rules

Each check prints `guard G<n>: ok` or `guard G<n>: FAIL <path>: <why>`; the last line is
`guards: <k> of 8 passed`; exit status 0 iff k = 8. Paths are searched under `infra/` only.

| Id | Rule | Why |
|---|---|---|
| G1 | No `terraform.tfvars` and no `*.auto.tfvars` anywhere | auto-loaded tfvars is how a bare `apply` hit Panacea's production (survey §1.1, `:46-51`) |
| G2 | No `*.tfvars` contains a GUID (`[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}`) | subscription, tenant and object ids arrive as `TF_VAR_*` / `ARM_*`, never in source |
| G3 | Every `*.tftest.hcl` declares a `mock_provider` for **every** provider named in its directory's `versions.tf` `required_providers` | a test without a mock would call Azure |
| G4 | No `provider "kubernetes"`, `provider "helm"`, and no `hashicorp/kubernetes` or `hashicorp/helm` source anywhere | P8-D3 |
| G5 | Every `stacks/*/backend.tf` holds exactly `backend "azurerm" {}` with an empty body | partial config only; no storage account name in source |
| G6 | No `provisioner "local-exec"` or `"remote-exec"` | a provisioner is a script Terraform cannot plan |
| G7 | Every `stacks/*/` has a `.terraform.lock.hcl` **tracked by git** (`git ls-files --error-unmatch`) | reproducible providers |
| G8 | No attribute named `administrator_password`, `administrator_password_wo`, `client_secret` or `admin_password` in any `.tf` | P8-D10, P8-D11: there is no password to set |

#### Conventions the README records (and later parts assume)

- Every variable: `type`, `description`; closed sets get a `validation`. No `default` for anything
  environment-specific (location, CIDRs, names) — `<env>.tfvars` sets them.
- `locals { tags = merge(var.tags, { product = "panacea-app", environment = var.environment,
  stack = "<stack>", managed-by = "terraform" }) }` in every stack; modules take `tags` as input.
- Names: `<kind>-panacea-app-<env>[-<qualifier>]`, e.g. `rg-panacea-app-dev`, `aks-panacea-app-dev`;
  globally unique names (storage, ACR, Key Vault) drop hyphens and append `var.unique_suffix`,
  four lowercase alphanumerics validated by `^[a-z0-9]{4}$` and set in `<env>.tfvars` (e.g.
  `stpanaceaappdevk7q2`). The suffix is chosen once per environment and never derived from an id.
- Lock files for stacks are created with
  `terraform providers lock -platform=windows_amd64 -platform=linux_amd64`, so the Linux CI agent
  and the Windows laptop both verify against the committed file.
- `environment` is validated against `["dev", "prod"]`.

#### Tests to write first

`guards_test.sh` builds a throwaway tree under `$(mktemp -d)` per case, copies `guards.sh` in,
runs it with `INFRA_ROOT` pointed at the fixture, and asserts on exit status and the named line.

| Test (case name) | Asserts | Mutant it kills |
|---|---|---|
| `clean_tree_passes` | a tree with one conforming stack and one module → exit 0, `guards: 8 of 8 passed` | a guard that fails on everything |
| `g1_terraform_tfvars_fails` | `stacks/x/terraform.tfvars` present → exit ≠ 0, `guard G1: FAIL` naming the path | G1 deleted, or matching only `*.auto.tfvars` |
| `g2_guid_in_tfvars_fails` | `dev.tfvars` with `tenant_id = "00000000-0000-0000-0000-000000000000"` → `G2: FAIL` | G2 matching only lowercase hex |
| `g3_test_without_azuread_mock_fails` | `versions.tf` requires `azurerm` and `azuread`; the test mocks only `azurerm` → `G3: FAIL` | G3 that checks for "any" mock rather than "every" |
| `g4_helm_provider_fails` | a `provider "helm" {}` block → `G4: FAIL` | G4 that only greps `required_providers` |
| `g5_backend_with_body_fails` | `backend "azurerm" { storage_account_name = "x" }` → `G5: FAIL` | G5 that only checks the block exists |
| `g6_local_exec_fails` | a `provisioner "local-exec"` → `G6: FAIL` | G6 deleted |
| `g7_untracked_lock_fails` | a stack whose lock file exists but is not `git add`ed in the fixture repo → `G7: FAIL` | G7 that tests file existence instead of tracking |
| `g8_client_secret_fails` | `client_secret = var.x` in a `.tf` → `G8: FAIL` | G8 that matches only `administrator_password` |

`validate.sh` is proved by its own acceptance run below (it has nothing to validate yet) and by
every later task, whose acceptance command is `infra/scripts/validate.sh <its dir>`.

#### Steps

- [ ] **Step 1: Confirm the starting state.** `git ls-files infra` lists exactly the five phase 1
  files; `ls infra/terraform/.terraform` fails (no init state, no local tfstate). If a
  `*.tfstate` exists anywhere under `infra/`, **stop**: someone applied the phase 1 root, and
  deleting it would orphan real resources.
- [ ] **Step 2: Write `guards_test.sh` with the nine cases.** Run it: every case fails, because
  `guards.sh` does not exist.
- [ ] **Step 3: Write `guards.sh`.** Run `guards_test.sh` until `guards_test: 9 cases, 9 passed`.
- [ ] **Step 4: Write `validate.sh`** to the contract above.
- [ ] **Step 5: `git rm` the five phase 1 files**, add `.gitattributes`, extend `.gitignore`,
  write `README.md` with the layout, the stack order, the conventions and the command list.
- [ ] **Step 6: Run the acceptance commands** and commit by pathspec.

#### Acceptance

```bash
bash infra/scripts/guards_test.sh
# guards_test: 9 cases, 9 passed                                     exit 0
bash infra/scripts/guards.sh
# guard G1: ok … guard G8: ok
# guards: 8 of 8 passed                                              exit 0
bash infra/scripts/validate.sh
# validate: nothing to validate yet                                  exit 0
git ls-files infra
# infra/scripts/guards.sh
# infra/scripts/guards_test.sh
# infra/scripts/validate.sh
# infra/terraform/README.md
git check-attr eol -- infra/scripts/guards.sh
# infra/scripts/guards.sh: eol: lf
```

#### What this task does not claim

- That any Terraform is valid: there is none yet.
- That the phase 1 pool definition was ever applied (step 1 checks only for local state; a pool
  created by hand in Panacea's cluster would be invisible here, and is the owner's to remove).
- That `shellcheck` passes: it is not installed on the build laptop; task 28's `infra` pipeline runs
  it on every script under `infra/`.
