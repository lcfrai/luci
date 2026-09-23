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
# local state, deletes the override and runs `init -migrate-state` into this backend (review I2)
terraform {
  backend "azurerm" {}
}

# providers.tf
provider "azurerm" {
  features {}
  storage_use_azuread            = true   # the account has no shared key; data-plane calls use Entra
  resource_provider_registrations = "none" # the read-only plan identity cannot register providers; task 31 §1 does (review C3; attribute name UNVERIFIED)
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
| `ip_rules_turn_default_deny_on` | `state_ip_rules = ["203.0.113.0/24"]` → `network_rules[0].default_action == "Deny"` | Deny never set |
| `the_account_is_locked` | `azurerm_management_lock.state.lock_level == "CanNotDelete"` | the lock removed |

The GUID in the test's `variables` block is in a `.tftest.hcl`, not a `.tfvars`, so guard G2 does
not apply; it is the all-ones placeholder and names nobody.

#### Steps

- [ ] **Step 1:** Write `versions.tf` and the test file; `validate.sh infra/terraform/stacks/bootstrap`
  fails at `validate` (resources missing).
- [ ] **Step 2:** Write the variables, resources and outputs until every run passes.
- [ ] **Step 3:** `terraform providers lock -platform=windows_amd64 -platform=linux_amd64` in the
  stack; `git add` the lock file.
- [ ] **Step 4:** Write `dev.tfvars` (`environment = "dev"`, `location = "australiaeast"`,
  `unique_suffix`, `state_public_network_access = true`) and `prod.tfvars` (same, `prod`,
  `state_public_network_access = true` with a comment that task 30's runbook flips it to `false`
  after the in-VNet pool exists — the flip is the one planned change to this stack).
- [ ] **Step 5:** Write `backends/dev.hcl` and `backends/prod.hcl`; run guards and validate; commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/stacks/bootstrap
# validate: OK   infra/terraform/stacks/bootstrap
# validate: 1 ok, 0 failed
bash infra/scripts/guards.sh
# guards: 10 of 10 passed
```

`terraform test` prints `Success! 12 passed, 0 failed.` twice inside the validate step, once per
tfvars file (`dev.tfvars`, `prod.tfvars`).

#### What this task does not claim

- That the account can be created in the owner's subscription (policy may require private
  endpoints or CMK on storage; task 31 finds out).
- That hosted agents can reach it: with public access on and no IP rules, Entra RBAC is the only
  gate, which is a deliberate dev choice and is **not** the prod posture.
