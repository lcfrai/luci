### Task 02: `stacks/bootstrap` — where every other stack keeps its state

Panacea keeps state in an `azurerm` backend configured at `init` time
(`P:deploy/versions.tf:8-15`, survey §1.12 `:239-241`); its estate file names the storage account,
container and key. We do the same, with three differences: the account is **ours, in Terraform**;
it accepts **Entra authentication only** (no shared key, so no key to leak); and it keeps
**versions and soft-deleted blobs for 30 days**, so a bad `apply` that corrupts state is a restore,
not a rebuild. This stack is the only one whose own state starts local (the container it creates
does not exist yet); task 31's runbook migrates it into that container on first apply.

**Files:**
- `infra/terraform/stacks/bootstrap/{versions,backend,providers,main,variables,outputs}.tf` (new)
- `infra/terraform/stacks/bootstrap/dev.tfvars`, `prod.tfvars` (new)
- `infra/terraform/stacks/bootstrap/tests/bootstrap.tftest.hcl` (new)
- `infra/terraform/stacks/bootstrap/.terraform.lock.hcl` (new, both platforms)
- `infra/terraform/backends/dev.hcl`, `backends/prod.hcl` (new)

#### Contract

```hcl
# versions.tf
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = "4.81.0" }
  }
}

# backend.tf — partial; guard G5 keeps it empty. The first apply cannot init this block without
# the account it creates, so task 31 §2 writes an uncommitted backend_override.tf
# (`terraform { backend "local" {} }`, git-ignored, guard G1 refuses it tracked), applies with
# local state, deletes the override and runs `init -migrate-state -force-copy -input=false` into
# this backend (review I2; -input=false alone refuses the copy, plan re-review b04 round 1)
terraform {
  backend "azurerm" {}
}

# providers.tf
provider "azurerm" {
  features {}
  storage_use_azuread            = true   # the account has no shared key; data-plane calls use Entra
  resource_provider_registrations = "none" # the read-only plan identity cannot register providers; task 31 §1 does (review C3; name and value verified in task 02, As built)
}

# variables.tf
variable "environment" {
  type        = string
  description = "Environment name. Selects nothing by itself; every name and tag carries it."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}

variable "location" {
  type        = string
  description = "Azure region, e.g. australiaeast. Every stack must use the same value (foundation's check enforces it)."
}

variable "unique_suffix" {
  type        = string
  description = "Four lowercase alphanumerics appended to globally unique names. Chosen once per environment."
  validation {
    condition     = can(regex("^[a-z0-9]{4}$", var.unique_suffix))
    error_message = "unique_suffix must match ^[a-z0-9]{4}$."
  }
}

variable "state_writer_object_ids" {
  type        = list(string)
  description = "Entra object ids granted Storage Blob Data Contributor on the state container: the infra identity and the owner's break-glass group (the plan's Deploy identities table). Never the plan identity. Supplied as TF_VAR_state_writer_object_ids, never in tfvars."
  validation {
    condition     = length(var.state_writer_object_ids) > 0
    error_message = "At least one principal must be able to write state, or the next stack cannot init."
  }
}

variable "state_reader_object_ids" {
  type        = list(string)
  description = "Entra object ids granted Storage Blob Data Reader on the state container: the plan identity, which PR builds use (review C3). Supplied as TF_VAR_state_reader_object_ids, never in tfvars."
  validation {
    condition     = length(setintersection(var.state_reader_object_ids, var.state_writer_object_ids)) == 0
    error_message = "A state reader must not also be a writer: the plan identity is read-only by construction."
  }
}

variable "state_public_network_access" {
  type        = bool
  description = "Whether the state account accepts traffic from outside a private endpoint. true in dev (hosted agents have no fixed IP); prod sets false once the in-VNet pool exists (task 30)."
}

variable "state_ip_rules" {
  type        = list(string)
  description = "CIDRs allowed through the account firewall when public access is on. Empty means Entra RBAC is the only gate."
  default     = []
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged into local.tags."
  default     = {}
}

# outputs.tf — consumed by every later stack's -backend-config and by task 31's runbook
output "state_resource_group_name" { value = azurerm_resource_group.state.name }
output "state_storage_account_name" { value = azurerm_storage_account.state.name }
output "state_container_name" { value = azurerm_storage_container.tfstate.name }
output "product_resource_group_id" { value = azurerm_resource_group.product.id }
```

```hcl
# infra/terraform/backends/dev.hcl — what every stack passes as -backend-config=…/dev.hcl.
# The account name and resource group are NOT here: the pipeline passes them from bootstrap's
# outputs as further -backend-config key=value flags, so this file holds no environment identity
# beyond the container and the auth mode. key is passed per stack: <stack>.<env>.tfstate.
container_name   = "tfstate"
use_azuread_auth = true
```

#### Resources (bodies described; argument names verified against azurerm 4.81.0)

- `azurerm_resource_group.state` — `rg-panacea-app-${env}-tfstate`, `var.location`.
- `azurerm_resource_group.product` — `rg-panacea-app-${env}`, `var.location`. Bootstrap creates it,
  not `foundation`, so that it exists **before** the infra identity's first apply. The owner can
  then scope that identity's Contributor to this group instead of the subscription (the plan's
  "Deploy identities" table, review I8). `foundation` reads it with `data
  "azurerm_resource_group"` (task 07). It has no management lock: a lock on a resource group is
  inherited by everything in it, and would block every replace `foundation` plans.
- `azurerm_storage_account.state` — `sttfpanaceaapp${env}${suffix}`; `account_tier = "Standard"`,
  `account_replication_type = "ZRS"`, `min_tls_version = "TLS1_2"`,
  `https_traffic_only_enabled = true`, **`shared_access_key_enabled = false`**,
  `default_to_oauth_authentication = true`, `allow_nested_items_to_be_public = false`,
  `infrastructure_encryption_enabled = true`, `cross_tenant_replication_enabled = false`,
  `public_network_access_enabled = var.state_public_network_access`; `network_rules` with
  `default_action = length(var.state_ip_rules) > 0 ? "Deny" : "Allow"` and `ip_rules`;
  `blob_properties { versioning_enabled = true, delete_retention_policy { days = 30 },
  container_delete_retention_policy { days = 30 } }`.
- `azurerm_storage_container.tfstate` — `name = "tfstate"`, `storage_account_id` (not the
  deprecated `storage_account_name`), `container_access_type = "private"`.
- `azurerm_role_assignment.state_writer` — `for_each = toset(var.state_writer_object_ids)`,
  `Storage Blob Data Contributor`, scope the container's `resource_manager_id`-equivalent id
  (the storage account id + `/blobServices/default/containers/tfstate`).
- `azurerm_role_assignment.state_reader` — `for_each = toset(var.state_reader_object_ids)`,
  `Storage Blob Data Reader`, the same scope. With `-lock=false` (task 30) the plan identity needs
  no blob lease, so read is enough.
- `azurerm_management_lock.state` — `CanNotDelete` on the storage account, `notes` saying why.

#### Tests to write first (`tests/bootstrap.tftest.hcl`, mocks only)

```hcl
mock_provider "azurerm" {}
# File level holds ONLY what no <env>.tfvars sets (guard G10, review I1). A file-level
# variables block overrides -var-file, so an `environment = "dev"` here would make
# validate.sh's prod.tfvars pass a second dev pass. environment, location, unique_suffix and
# state_public_network_access come from the -var-file validate.sh passes.
variables {
  state_writer_object_ids = ["11111111-1111-1111-1111-111111111111"]
  state_reader_object_ids = ["22222222-2222-2222-2222-222222222222"]
}

# Plans the committed tfvars untouched: no run-level key a tfvars sets.
run "committed_tfvars_plan" {
  command = plan
  assert {
    condition     = azurerm_storage_account.state.name == "sttfpanaceaapp${var.environment}${var.unique_suffix}"
    error_message = "The state account name is not built from this tfvars' environment and suffix."
  }
  assert {
    condition     = azurerm_storage_account.state.public_network_access_enabled == var.state_public_network_access
    error_message = "state_public_network_access from the tfvars did not reach the account."
  }
}
```

Every other run reads `var.environment` in its assertions instead of a literal `"dev"`, so it
holds under both passes. A bare `terraform test` in the stack, without `-var-file`, stops at
"No value for required variable": that is intended, and `validate.sh` is the only caller.

| Run | Asserts | Mutant it kills |
|---|---|---|
| `committed_tfvars_plan` | the block above; runs once under `dev.tfvars` and once under `prod.tfvars` | a committed tfvars that does not plan. It **cannot** kill a file-level `environment` that turns the prod pass into a dev pass: its assertions read `var.environment`, so they agree with whatever value won (b04 probe). Guard G10 kills that mutant, statically |
| `state_account_has_no_shared_key` | `azurerm_storage_account.state.shared_access_key_enabled == false` | the key left on (the default) |
| `state_keeps_versions_and_deletes_for_30_days` | `versioning_enabled == true` and both retention `days == 30` | versioning or soft delete dropped |
| `state_name_fits_storage_rules` | the name matches `^[a-z0-9]{3,24}$` and ends with `var.unique_suffix` | a hyphen in the name, or a suffix left out |
| `every_writer_gets_blob_contributor` | two ids in → two `azurerm_role_assignment.state_writer` instances, each `Storage Blob Data Contributor` | `for_each` over the first id only; the wrong role |
| `the_plan_identity_only_reads_state` | one `azurerm_role_assignment.state_reader` instance, `Storage Blob Data Reader`, on the container scope | a reader given Contributor, which lets a PR build rewrite state (review C3) |
| `a_reader_that_is_also_a_writer_is_rejected` | the same id in both lists → `expect_failures = [var.state_reader_object_ids]` | the plan identity quietly made a writer |
| `the_product_group_exists_before_foundation` | `azurerm_resource_group.product.name == "rg-panacea-app-${var.environment}"` | the group left to `foundation`, so the infra identity's Contributor would need the subscription (review I8) |
| `no_writers_is_rejected` | `state_writer_object_ids = []` → `expect_failures = [var.state_writer_object_ids]` | the validation removed |
| `bad_suffix_is_rejected` | `unique_suffix = "K7Q2"` → `expect_failures = [var.unique_suffix]` | a case-insensitive regex |
| `a_five_character_suffix_is_rejected` *(added as built)* | `unique_suffix = "k7q2x"` → `expect_failures = [var.unique_suffix]` | the length relaxed to `{4,6}` (round 2 predicted survivor X1) |
| `a_three_character_suffix_is_rejected` *(added as built)* | `unique_suffix = "k7q"` → `expect_failures = [var.unique_suffix]` | the length relaxed to `{3,4}` (X2) |
| `ip_rules_turn_default_deny_on` | `state_ip_rules = ["203.0.113.0/24"]` → `network_rules[0].default_action == "Deny"`; *as built, also* `ip_rules == toset(["203.0.113.0/24"])` | Deny never set; *as built:* `ip_rules = []`, Deny with no rules (E1) |
| `the_account_is_locked` | `azurerm_management_lock.state.lock_level == "CanNotDelete"`; *as built, also* `scope == azurerm_storage_account.state.id`, so the run applies | the lock removed; *as built:* the lock moved to the product group (predicted survivor S2) or to the container (N2) |
| `the_state_account_is_hardened` *(added as built)* | Standard ZRS; `TLS1_2` and HTTPS only; OAuth by default; no public nested items and a `private` container; infrastructure encryption; no cross-tenant replication | any of those flags flipped (E2–E9) |
| `public_access_off_reaches_the_account` *(added as built)* | `state_public_network_access = false` → `public_network_access_enabled == false` | `public_network_access_enabled = true` hard-coded: both committed tfvars say `true`, so `committed_tfvars_plan` cannot tell (predicted survivor S1) |
| `no_ip_rules_leave_entra_as_the_only_gate` *(added as built)* | `state_ip_rules = []` → `default_action == "Allow"` | `"Deny"` always, which with no rules locks every caller out (E18) |
| `a_bad_environment_is_rejected` *(added as built)* | `environment = "test"` → `expect_failures = [var.environment]` | the environment validation removed (E17v2) |
| `state_lives_in_its_own_tagged_group` *(added as built)* | `tags = { cost-centre = "test", stack = "overridden" }` and `location = "australiasoutheast"` (a region no tfvars uses) → the state group is `rg-panacea-app-${var.environment}-tfstate` and holds the account; all three located in `var.location`; each tag map is `cost-centre` plus the stack's own four, `stack = "bootstrap"` winning | the state group sharing the product group's name (E14); the account in the product group (E15); a wrong or overridable tag (E12, N1); a location hard-coded to `australiaeast`, which both tfvars would agree with (round 2 predicted survivor L1; L2, L3) |
| `the_outputs_name_what_was_built` *(added as built)* | each output equals the resource it names; `state_container_name == "tfstate"` | an output naming the wrong thing (E13, N3); the container renamed (E16) |
| `the_backend_file_names_this_container` *(added as built)* | `backends/${var.environment}.hcl`, read with `file()`: its `container_name` is the container this stack creates, it sets `use_azuread_auth = true`, and it holds no account, group, key, credential or tenant line. Each tfvars pass reads its own environment's file | `use_azuread_auth = false` (B1; B1p in prod.hcl, the prod pass only); prod's container misnamed (B2, the prod pass only); an account name in the file (B3) |

The GUID in the test's `variables` block is in a `.tftest.hcl`, not a `.tfvars`, so guard G2 does
not apply; it is the all-ones placeholder and names nobody.

#### Steps

- [x] **Step 1:** Write `versions.tf` and the test file; `validate.sh infra/terraform/stacks/bootstrap`
  fails at `test` (resources missing). *As built: two facts differ from the step as first
  written. Without a lock file `init -lockfile=readonly` fails first ("Provider dependency
  changes detected"), so step 3 runs before this step. And `terraform validate` accepts a test
  file whose assertions name undeclared resources, so the red is at `test`, not `validate`:
  "Reference to undeclared resource", 0 passed, 1 failed, 11 skipped.*
- [x] **Step 2:** Write the variables, resources and outputs until every run passes.
- [x] **Step 3:** `terraform providers lock -platform=windows_amd64 -platform=linux_amd64` in the
  stack; `git add` the lock file. *(Run before step 1's red; see step 1.)*
- [x] **Step 4:** Write `dev.tfvars` (`environment = "dev"`, `location = "australiaeast"`,
  `unique_suffix`, `state_public_network_access = true`) and `prod.tfvars` (same, `prod`,
  `state_public_network_access = true` with a comment that task 30's runbook flips it to `false`
  after the in-VNet pool exists — the flip is the one planned change to this stack).
  *As built: dev's suffix is `k7q2`, task 07's; prod's is `r4nv`, chosen here, and every later
  prod tfvars uses it.*
- [x] **Step 5:** Write `backends/dev.hcl` and `backends/prod.hcl`; run guards and validate; commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/stacks/bootstrap
# validate: OK   infra/terraform/stacks/bootstrap
# validate: 1 ok, 0 failed
bash infra/scripts/guards.sh
# guards: 10 of 10 passed
```

`terraform test` prints `Success! 21 passed, 0 failed.` twice inside the validate step, once per
tfvars file (`dev.tfvars`, `prod.tfvars`). *As built: 21, not 12: the nine runs marked "added
as built" kill the mutants the first twelve let live (As built, below).*

#### What this task does not claim

- That the account can be created in the owner's subscription (policy may require private
  endpoints or CMK on storage; task 31 finds out).
- That hosted agents can reach it: with public access on and no IP rules, Entra RBAC is the only
  gate, which is a deliberate dev choice and is **not** the prod posture.
- *As built:* that `sttfpanaceaappdevk7q2` and `sttfpanaceaappprodr4nv` are free in Azure's
  global namespace (no Azure call is allowed here); that the CanNotDelete lock lets a writer's
  role assignment be removed later; that the operator's `init -migrate-state` (task 31 §2d)
  works without the operator being, or being in, a writer principal; and that azurerm keeps no
  account key in bootstrap's state with shared keys off ("Open concerns", below).

#### As built (task 02, 2026-09-25)

- **Verified against azurerm 4.81.0** (`terraform validate`, no warning): every argument in
  "Resources", including `https_traffic_only_enabled`, `storage_account_id` on the container, and
  the provider's `resource_provider_registrations` (a bogus value is refused at validate, which
  lists the legal ones; probe V1).
- **Step order.** The lock file comes first, and the red is at `test` (step 1).
- **The mock id.** azurerm validates the container's `storage_account_id` as a Resource Manager id
  even under `mock_provider` (plan-time validation passes through to the real provider), so a
  random mock id failed every apply run. The test pins
  `mock_resource "azurerm_storage_account" { defaults = { id = "/subscriptions/0000…/…/storageAccounts/stmock" } }`.
  The same pass-through means a hyphen in the account name is refused by azurerm's own name
  check in the first run, before `state_name_fits_storage_rules` reads it.
- **Nine runs and two assertions added** (the table's "added as built" rows), in two rounds of
  the mutation check. Round 1 (the first twelve runs): the two predicted survivors (S1 public
  access hard-coded, S2 the lock on the product group) and the other untested attributes all
  lived. Seven runs and two assertions killed them in round 2, which left alive only its own
  two predicted survivors: L1 (a location hard-coded to the one both tfvars use) and X1 (the
  suffix length relaxed). Round 3 killed those with a run-level `location` no tfvars uses and
  the five- and three-character suffix runs.
- **Mutants the file cannot kill:** `storage_use_azuread = false` (probe V2). Provider
  configuration is not visible to `terraform test`. It should fail loudly at apply rather than
  weaken anything, because with shared keys off azurerm has no key to fall back on
  (**UNVERIFIED**: reasoning, not a run).
- **Malformed first forms.** `condition = true` in a validation is refused at `init` ("Invalid
  validation expression": a condition must name its variable), so the "validation removed"
  mutants delete the block instead (T7v2, T9v2, E17v2).

#### Open concerns (task 02, for task 31)

- **The lock and the role assignments.** Both state roles are scoped beneath the locked account
  (the container). If a `CanNotDelete` lock also blocks deleting a role assignment at a child
  scope, then removing a writer or reader (rotating the infra identity, say) needs the lock
  lifted by hand first. **UNVERIFIED**: no Azure call is allowed here; task 31 finds out the
  first time a principal changes.
- **Account keys in state.** `azurerm_storage_account` exports `primary_access_key` and the
  connection strings as sensitive attributes. Whether azurerm 4.81.0 still reads the keys
  (`listKeys`, a control-plane call) with `shared_access_key_enabled = false` is **UNVERIFIED**.
  If it does, the keys sit in `bootstrap.<env>.tfstate`, which the plan identity can read. They
  authorise nothing while shared keys stay off, and turning them back on is a control-plane
  write the plan identity cannot make, but the keys would then work at once.
- **Who migrates the state.** Already covered by task 31 §2 step (d): the operator must hold
  Storage Blob Data Contributor on the container, so is in the break-glass group, and a 403
  right after the first apply is role propagation (retry). Nothing more for this stack.
