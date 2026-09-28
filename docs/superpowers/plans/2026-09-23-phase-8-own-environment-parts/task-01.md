### Task 01: The `infra/terraform` layout, its conventions, and the two scripts that enforce them

`infra/terraform/` today is phase 1's root: one node pool attached to **Panacea's** dev cluster
(`infra/terraform/kata.tf:4-6`, `variables.tf:1-4`, and `dev.tfvars:4`, whose comment supplies the
id from `az aks show -g rg-aks-panacea-dev -n aks-panacea-dev`). OD-5 makes that target obsolete:
the pool belongs in our own cluster (survey §3.1, `:342-344`). This task retires the root and puts
in its place the skeleton every later OpenTofu task writes into, plus the two scripts that make
the rules mechanical rather than remembered.

**Verified before this plan was written (2026-09-23, scratch directory, not this repo), on Terraform 1.15.8 — the build laptop's engine that day (plan P8-D14, `:145`), before OD-32 — so this probe record keeps the binary it ran on:**
`terraform init -backend=false` downloads `hashicorp/azurerm` 4.81.0, `hashicorp/azuread` 3.9.0
and `azure/azapi` 2.12.0 from `registry.terraform.io` through the corporate network; and `terraform test` with
`mock_provider "azurerm" {}` / `mock_provider "azuread" {}` passes a `command = plan` run, an
`expect_failures = [var.name]` run and a `command = apply` run **with no Azure credential** — the
basis for every OpenTofu test in this plan. The three runs took about **100 seconds** on the build
laptop (provider start-up dominates), so `validate.sh` takes one directory at a time and a caller
runs it per directory, in the background when a directory has many runs. **UNVERIFIED under OpenTofu 1.12.6**, because no `tofu` binary has run on this host: that `tofu test` passes the same three runs in about the same time, and that the corporate proxy passes `registry.opentofu.org`, the host `tofu init` fetches from instead — an open risk (plan `:177-179`, `:737`). The first `validate.sh` run settles both: record what it prints, and how long it takes.

**Files:**
- `infra/terraform/kata.tf`, `providers.tf`, `variables.tf`, `dev.tfvars`, `.terraform.lock.hcl` — deleted with `git rm` (the pool is re-written in `modules/aks`, task 06)
- `infra/terraform/README.md` — the stack order, the rules, the commands (new)
- `infra/scripts/validate.sh` — fmt, init, validate, test for one directory or all (new)
- `infra/scripts/guards.sh` — the ten repository rules as named checks (new)
- `infra/scripts/guards_test.sh` — one fixture tree per rule, each a mutant guards.sh must kill (new)
- `.gitattributes` — `eol=lf` for `*.tf`, `*.tfvars`, `*.hcl`, `*.tftest.hcl`, `*.sh`, `*.yaml`, `*.yml`, `*.tpl` (new)
- `.gitignore` — adds `*.tfplan`, `crash*.log`, `infra/terraform/modules/**/.terraform.lock.hcl`, `override.tf`, `override.tf.json`, `*_override.tf`, `*_override.tf.json` (modify; the override lines keep runbook §2's local-backend file out of git, review I2; OpenTofu treats the bare names as override files too, plan re-review b04 round 1, finding 6)

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
# infra/scripts/validate.sh — the ONLY OpenTofu commands this phase may run (the one exception
# is task 07's by-hand `tofu graph` of the foundation stack: config only, review m8).
#
#   infra/scripts/validate.sh                 every dir under modules/ and stacks/
#   infra/scripts/validate.sh <dir> [<dir>…]  just these, relative to the repo root
#
# For each directory, in order, stopping that directory at the first failure:
#   1. tofu fmt -check -recursive -diff
#   2. stacks:  tofu init -backend=false -input=false -lockfile=readonly
#      modules: tofu init -backend=false -input=false
#   3. tofu validate -no-color
#   4. if the directory has a *.tftest.hcl, in tests/ or at its root (tofu test runs both,
#      plan re-review b04-2 M2):
#        modules: tofu test -no-color
#        stacks:  tofu test -no-color -var-file=<env>.tfvars, once per <env>.tfvars in the
#                 stack, so every committed tfvars file is proven to plan against mocks. The
#                 test files supply only what tfvars may not hold (ids, as TF_VAR_* would).
#                 A file-level `variables` block in a .tftest.hcl OVERRIDES -var-file
#                 (Terraform 1.15.8 probe, review I1), so guards.sh G10 forbids a file-level
#                 key that any <env>.tfvars in the stack sets, and a one-line file-level block
#                 outright (fmt leaves one on one line, b04-2 I1); a run that needs an
#                 environment-specific value sets it in that run's own `variables` block.
#                 Run-level values override -var-file too (b04 probe, Terraform 1.15.8: a
#                 run pinning "dev" saw "dev" under a prod -var-file; a run with no pin and
#                 no file-level key saw "prod"), so the proof rests on each stack's
#                 `committed_tfvars_plan` run: `command = plan`, and no run-level key that a
#                 tfvars sets, so the dev pass plans dev.tfvars and the prod pass plans
#                 prod.tfvars, each untouched. That run cannot tell which pass it is in, so
#                 G10, not the run, is what kills a file-level override.
#
# A directory is a stack if its path has a stacks/ component, and a module otherwise.
#
# Output, one line per directory:   validate: OK   <dir>
#                                   validate: FAIL <dir> (<step>)
# Last line:                        validate: <n> ok, <m> failed
#                                   validate: nothing to validate yet     (no dirs exist)
# Exit status: 0 iff m == 0. A failed step's own output goes to stderr, indented.
#
# It never runs plan, apply, import, state or console, never passes -backend-config, and
# unsets ARM_SUBSCRIPTION_ID, ARM_CLIENT_ID, ARM_CLIENT_SECRET, ARM_USE_OIDC and
# ARM_OIDC_TOKEN for its own process tree, so a developer's shell credentials cannot reach a
# provider even if a test forgot a mock (guards.sh G3 is the first line; this is the second).
# It unsets every other ARM_* variable too, the Azure SDK's AZURE_* credential variables, the
# pipeline token variables, and TF_CLI_ARGS / TF_CLI_ARGS_* (which could add -backend-config).
# It also exports AZURE_CONFIG_DIR as a fresh empty directory (mktemp -d, removed by an EXIT
# trap), so no `az login` context is reachable either: azurerm's Azure CLI authentication
# finds no account, and a missed mock fails instead of reading the operator's subscription —
# who runs Panacea day to day, so that context is likely Panacea's (review I3).
set -euo pipefail
```

#### `guards.sh` — the ten rules

Each check prints `guard G<n>: ok` or `guard G<n>: FAIL <path>: <why>`; the last line is
`guards: <k> of 10 passed`; exit status 0 iff k = 10. Paths are searched under `infra/` only.
The rules share one `find | sort` inventory and one `awk` scan. If either fails, guards.sh prints
`guards: inventory failed (find | sort exit <n>)` or `guards: scanner failed (awk exit <n>)` and
exits 2 before any rule reports, because a read that failed must never be taken for "nothing
found" (task 01 review I1: with a failing `awk`, seven rules printed `ok` having checked nothing).

**These guards are hygiene, not a security boundary** (review C3). They run from the branch being
built, so a pull request can edit or delete them. They catch mistakes in reviewed code. What stops
a hostile pull request is outside the repository: the read-only plan identity and the branch
control on the write connections (task 30, and the plan's "Deploy identities" table).

| Id | Rule | Why |
|---|---|---|
| G1 | No `terraform.tfvars` and no `*.auto.tfvars` anywhere (nor their `.json` forms, which OpenTofu auto-loads too); no `*.tf.json` or `*.tfvars.json` anywhere, the two JSON override names excepted; and no override file tracked by git (`git ls-files`): `override.tf`, `override.tf.json`, `*_override.tf` or `*_override.tf.json`, the four names OpenTofu loads as overrides. Outside a git work tree the tracked check cannot run, and G1 fails | auto-loaded tfvars is how a bare `apply` hit Panacea's production (survey §1.1, `:46-51`). An override file silently replaces the block it names, including `backend`: runbook §2 writes an uncommitted `backend_override.tf` for the one local-state bootstrap apply (review I2), and a tracked one would swap any stack's backend for local state. JSON configuration is refused because G3, G5, G9 and G10 read HCL only, so a `.tf.json` would slip past four rules; refusing it makes every rule cover every legal form (task 01 review M3). Other JSON under `infra/` is not OpenTofu (task 30's `infra/rbac/*.json` role definitions) and passes |
| G2 | No `*.tfvars` contains a GUID (`[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}`) | subscription, tenant and object ids arrive as `TF_VAR_*` / `ARM_*`, never in source |
| G3 | Every `*.tftest.hcl` declares a `mock_provider` for **every** provider named in a `required_providers` block of any `.tf` in its directory (`versions.tf` by convention). Its directory is the parent of `tests/` for a file in `tests/`, and its own directory for a root-level file, which `tofu test` runs too (plan re-review b04-2 minor 2) | a test without a mock would call Azure |
| G4 | No `provider "kubernetes"`, `provider "helm"`, and no `hashicorp/kubernetes` or `hashicorp/helm` source anywhere | P8-D3 |
| G5 | Every `stacks/*/backend.tf` holds exactly `backend "azurerm" {}` with an empty body, and a stack with no `backend.tf` fails too | partial config only; no storage account name in source. Missing counts as a failure because `tofu init -backend-config=<file>` on a configuration with no backend block exits 0, with only a "Missing backend configuration" warning, and uses local state (Terraform 1.15.8 probe, task 01 review I3): the pipeline's init would succeed and an apply would leave state on the agent's disk |
| G6 | No `provisioner "local-exec"` or `"remote-exec"` | a provisioner is a script OpenTofu cannot plan |
| G7 | Every `stacks/*/` has a `.terraform.lock.hcl` **tracked by git** (`git ls-files --error-unmatch`) | reproducible providers |
| G8 | No attribute named `administrator_password`, `administrator_password_wo`, `client_secret` or `admin_password` in any `.tf` | P8-D10, P8-D11: there is no password to set |
| G9 | Every `source =` inside a `required_providers` block, and every `provider "registry.terraform.io/<ns>/<name>"` in a `.terraform.lock.hcl`, is one of `hashicorp/azurerm`, `hashicorp/azuread`, `azure/azapi` | a provider is a program that runs at plan time with the pipeline's token: `data "external"` runs any command. A resource type used without a `required_providers` entry makes OpenTofu infer `hashicorp/<prefix>`, which only the lock file shows. The allowlist in G9 and `tofu init -lockfile=readonly` (task 30's `tf-stack.sh`) catch this together (review C3) |
| G10 | In every `*.tftest.hcl` under `stacks/<x>/`, in `tests/` or at the stack root alike (`tofu test` runs both; plan re-review b04-2 minor 2), the file-level `variables { … }` block (the one outside any `run`) holds no key that any `stacks/<x>/*.tfvars` sets, and is not written on one line | a file-level `variables` block overrides `-var-file` (Terraform 1.15.8 probe: file `environment = "dev"`, `-var-file` `"prod"`, the run saw `"dev"`; review I1), so a file-level `environment` turns validate.sh's `prod.tfvars` run into a second dev run. Environment-specific values a run depends on go in that run's own `variables` block. `tofu fmt` puts a file-level block at column 0 and a run's at column 2, so G10 reads the lines between `^variables {` and `^}` for `^  <key> *=` (column 2, then **any** run of spaces before `=`), against every tfvars' `^<key> *=`. The `*` is load-bearing: `fmt` lines up the `=` of consecutive attributes, so a short key under a long one is written `  environment             = "dev"` (Terraform 1.15.8 probe, plan re-review b04 round 1, finding 2), and a single-space pattern would miss the very layout `fmt` produces. That is not the only layout `fmt -check` accepts: `fmt` leaves a one-argument block on one line, so `variables { environment = "dev" }` passes `fmt -check` (exit 0) with the key on the opener line, which "the lines between" leaves out (plan re-review b04-2 important 1, probe p1; re-probed in task 01 with Terraform 1.15.8). So G10 fails any file-level block written on one line (`^variables *\{.*=`), naming the file, whatever its key; the multi-line layout it reads is then the only legal one |

#### Conventions the README records (and later parts assume)

- Every variable: `type`, `description`; closed sets get a `validation`. No `default` for anything
  environment-specific (location, CIDRs, names) — `<env>.tfvars` sets them.
- `locals { tags = merge(var.tags, { product = "panacea-app", environment = var.environment,
  stack = "<stack>", managed-by = "opentofu" }) }` in every stack; modules take `tags` as input.
- Names: `<kind>-panacea-app-<env>[-<qualifier>]`, e.g. `rg-panacea-app-dev`, `aks-panacea-app-dev`;
  globally unique names (storage, ACR, Key Vault) drop hyphens and append `var.unique_suffix`,
  four lowercase alphanumerics validated by `^[a-z0-9]{4}$` and set in `<env>.tfvars` (e.g.
  `stpanaceaappdevk7q2`). The suffix is chosen once per environment and never derived from an id.
- Lock files for stacks are created with
  `tofu providers lock -platform=windows_amd64 -platform=linux_amd64`, so the Linux CI agent
  and the Windows laptop both verify against the committed file.
- `environment` is validated against `["dev", "prod"]`.
- A stack's `.tftest.hcl` file-level `variables` block holds only what no `<env>.tfvars` sets (ids,
  as `TF_VAR_*` would supply). Every run that depends on an environment-specific value, including
  `environment` itself, sets it in its own run-level `variables` block, and each stack's test file
  has a `committed_tfvars_plan` run that sets none of them (G10, review I1).
- Every stack's `provider "azurerm"` block sets `storage_use_azuread = true` and
  `resource_provider_registrations = "none"`. The second is needed because the read-only plan
  identity cannot register resource providers, so task 31 §1 registers them by hand (task 30,
  review C3; the attribute name is **UNVERIFIED** until `validate` runs in task 02).

#### Tests to write first

`guards_test.sh` builds a throwaway tree under `$(mktemp -d)` per case, copies `guards.sh` in,
runs it with `INFRA_ROOT` pointed at the fixture, and asserts on exit status and the named line.
A case may put stub commands first on guards.sh's `PATH` (the scanner-failure case does).

| Test (case name) | Asserts | Mutant it kills |
|---|---|---|
| `clean_tree_passes` | a tree with one conforming stack, one module and a non-OpenTofu `infra/rbac/plan-reader.json` → exit 0, `guards: 10 of 10 passed` | a guard that fails on everything, or a G1 JSON rule that refuses every `*.json` (task 30 keeps role definitions there) |
| `g1_terraform_tfvars_fails` | `stacks/x/terraform.tfvars` present → exit ≠ 0, `guard G1: FAIL` naming the path; then `stacks/x/dev.auto.tfvars` → `G1: FAIL`; then `stacks/x/extra.tf.json` and `stacks/x/prod.tfvars.json` → `G1: FAIL … JSON configuration` naming each (task 01 review M3) | G1 deleted, matching only `*.auto.tfvars` or only `terraform.tfvars`, without the JSON rule, or refusing only `*.tf.json` |
| `g1_tracked_override_fails` | `stacks/x/backend_override.tf` holding `terraform { backend "local" {} }`, `git add`ed in the fixture repo → `G1: FAIL` naming the path; the same body as a tracked bare `stacks/y/override.tf` → `G1: FAIL` naming that path; the same files present but untracked → G1 ok; then the fixture's `.git` removed → `G1: FAIL infra: not inside a git work tree` and `G7: FAIL …/.terraform.lock.hcl: not inside a git work tree` (task 01 review M2) | G1 without the override check, G1 matching only `*_override.tf` (the bare `override.tf` is an override file too), one that fails on an untracked override (runbook §2 needs one on disk), or G1 or G7 passing when `git ls-files` fails |
| `g2_guid_in_tfvars_fails` | `dev.tfvars` with `tenant_id = "00000000-0000-0000-0000-000000000000"` → `G2: FAIL`; then the same key holding an uppercase id, `"7A3F9C2E-1B4D-4E8A-9C6B-2D5E8F1A3B7C"` → `G2: FAIL` (the all-zero id has no hex letter, so it alone cannot kill the mutant on the right; task 01 found this) | G2 matching only lowercase hex |
| `g3_test_without_azuread_mock_fails` | `versions.tf` requires `azurerm` and `azuread`; the test mocks only `azurerm` → `G3: FAIL`; then `tests/` conforming and a root-level `stacks/x/x.tftest.hcl` mocking only `azurerm` and `azapi` → `G3: FAIL` naming that path (task 01 review I2) | G3 that checks for "any" mock rather than "every", or G3 reading only `tests/` (a root-level file checked against `stacks/`, which requires nothing) |
| `g4_helm_provider_fails` | a `provider "helm" {}` block → `G4: FAIL` | G4 that only greps `required_providers` |
| `g5_backend_with_body_fails` | `backend "azurerm" { storage_account_name = "x" }` → `G5: FAIL`; then the stack's `backend.tf` removed → `G5: FAIL …/backend.tf: missing` (task 01 review I3) | G5 that only checks the block exists, or G5 that checks only the `backend.tf` files that exist |
| `g6_local_exec_fails` | a `provisioner "local-exec"` → `G6: FAIL` | G6 deleted |
| `g7_untracked_lock_fails` | a stack whose lock file exists but is not `git add`ed in the fixture repo → `G7: FAIL` | G7 that tests file existence instead of tracking |
| `g8_client_secret_fails` | `client_secret = var.x` in a `.tf` → `G8: FAIL` | G8 that matches only `administrator_password` |
| `g9_external_source_fails` | a module's `versions.tf` with `external = { source = "hashicorp/external" }` → `G9: FAIL` naming the path; then `external = { version = "~> 2.3" }` with no source (OpenTofu infers `hashicorp/external`) → `G9: FAIL`; then a one-line `required_providers { external = { source = "hashicorp/external" } }`, which `fmt -check` accepts → `G9: FAIL` (the last two added by task 01, whose mutation run found both survivors) | G9 deleted, checking stacks only (a module's providers flow into every stack that calls it), skipping an entry without a source, or reading only the spread-out layout |
| `g9_lockfile_provider_fails` | a clean `versions.tf`, and a stack's `.terraform.lock.hcl` holding `provider "registry.terraform.io/hashicorp/external"` → `G9: FAIL` | G9 that reads only `required_providers`, which an implicit `data "external"` never appears in |
| `g10_tftest_file_level_tfvars_key_fails` | `stacks/x/dev.tfvars` sets `environment`; `stacks/x/prod.tfvars` sets `environment` and `api_private`; `stacks/x/tests/x.tftest.hcl` has a file-level `variables` block of **two keys of different lengths, laid out by `tofu fmt`**: `state_writer_object_ids = ["x"]` (not in any tfvars) then `  api_private             = false` (13 spaces before `=`, the alignment `fmt` wrote in the b04 round-1 probe; the implementer runs `tofu fmt` on the fixture once and commits what it leaves) → `G10: FAIL` naming the file and the key `api_private`, and not `state_writer_object_ids`; the same key moved inside a `run "…" { variables { … } }` block → G10 ok; then the same two-key file-level block in a root-level `stacks/x/x.tftest.hcl` → `G10: FAIL` naming that path (plan re-review b04-2 minor 2) | G10 deleted, G10 that reads only `dev.tfvars` (not every `*.tfvars`), G10 with a single-space key pattern (`^  <key> =`, which misses the `fmt` alignment; plan re-review b04 round 1, finding 2), one that also flags run-level `variables` (which the fix needs), or one that reads only `tests/` |
| `g10_one_line_file_level_variables_fails` | `stacks/x/tests/x.tftest.hcl` whose file-level block is `variables { api_private = false }` on one line (what `fmt` leaves, and `fmt -check` exits 0), with `api_private` set in `prod.tfvars` → `G10: FAIL` naming the file; the same one-line layout holding a key no tfvars sets (`state_writer_object_ids`) → `G10: FAIL` too; a one-line `variables { environment = "dev" }` inside a `run` block → G10 ok (plan re-review b04-2 important 1) | G10 that reads only the lines after `variables {`, or one that also flags a run's one-line block |
| `scanner_failure_fails_closed` | a tree whose `dev.tfvars` holds a GUID (control: with the real `awk`, `expect_only 2`); then a stub `awk` first on guards.sh's `PATH` that exits 2 → exit 2, the line `guards: scanner failed (awk exit 2)`, and no `guard G` line and no `of 10 passed` line at all; then a stub `find` that exits 1 → exit 2 and `guards: inventory failed (find \| sort exit 1)`, again with no rule reported (task 01 review I1); then a stub `sort`, and then a stub `awk`, that each run the real command and then exit non-zero, so the output is complete and only the status says the read failed → the same two lines, no rule reported (fix round 1's two predicted survivors) | a scanner failure reported as ok (awk's exit status unread), the inventory's exit status unread, `sort`'s status ignored (only `find`'s read), or `awk`'s failure ignored whenever it printed something |

`validate.sh` is proved by its own acceptance run below (it has nothing to validate yet) and by
every later task, whose acceptance command is `infra/scripts/validate.sh <its dir>`.

#### Steps

- [ ] **Step 1: Confirm the starting state.** `git ls-files infra` lists exactly the five phase 1
  files; `ls infra/terraform/.terraform` fails (no init state, no local tfstate). If a
  `*.tfstate` exists anywhere under `infra/`, **stop**: someone applied the phase 1 root, and
  deleting it would orphan real resources.
- [ ] **Step 2: Write `guards_test.sh` with the fifteen cases** (the fifteenth,
  `scanner_failure_fails_closed`, was added by task 01's fix round 1, review I1). Run it: every
  case fails, because `guards.sh` does not exist.
- [ ] **Step 3: Write `guards.sh`.** Run `guards_test.sh` until `guards_test: 15 cases, 15 passed`.
- [ ] **Step 4: Write `validate.sh`** to the contract above. Prove the credential scrub once, red
  first (review I3): in a scratch directory holding one `main.tf`, with `ARM_SUBSCRIPTION_ID`
  exported and a stub `terraform` first on `PATH` that appends `AZURE_CONFIG_DIR=…`, the output
  of `ls -A "$AZURE_CONFIG_DIR"` and `ARM_SUBSCRIPTION_ID=…` to a log, run
  `validate.sh <scratch dir>`. The log must show an existing, empty directory that is not
  `~/.azure`, and an empty `ARM_SUBSCRIPTION_ID`; after the run the directory is gone. Run it
  first against a `validate.sh` without the `AZURE_CONFIG_DIR` export and record that it fails.
- [ ] **Step 5: `git rm` the five phase 1 files**, add `.gitattributes`, extend `.gitignore`,
  write `README.md` with the layout, the stack order, the conventions and the command list.
- [ ] **Step 6: Run the acceptance commands** and commit by pathspec.

#### Acceptance

```bash
bash infra/scripts/guards_test.sh
# guards_test: 15 cases, 15 passed                                   exit 0
bash infra/scripts/guards.sh
# guard G1: ok … guard G10: ok
# guards: 10 of 10 passed                                            exit 0
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

- That any OpenTofu is valid: there is none yet.
- That the phase 1 pool definition was ever applied (step 1 checks only for local state; a pool
  created by hand in Panacea's cluster would be invisible here, and is the owner's to remove).
- That `shellcheck` passes: it is not installed on the build laptop; task 28's `infra` pipeline runs
  it on every script under `infra/`.
