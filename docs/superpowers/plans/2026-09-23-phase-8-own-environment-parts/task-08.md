### Task 08: `modules/postgres` and `stacks/data` — the Hub's database, Entra-only

The Hub holds the provenance records OD-3 made shared (ADR-H1 §1.2). Panacea's written-but-off
Flexible Server has the right shape — Postgres 16, private endpoint only, `require_secure_transport`,
point-in-time restore (survey §1.9, `:192-198`) — but **authenticates with an admin login and
password** checked for length by a `check` (`:196-197`), and its Keycloak passwords sat in state in
plaintext (`:175-177`). Here there is no password at all: `password_auth_enabled = false`, the
administrator is an **Entra group** (owner input), and the Hub connects as its own managed identity
(task 10) with a token (P8-D10). Guard G8 (task 01) fails the build if a password attribute ever
appears.

A separate stack from `foundation` (P8-D1) because a database is the one thing whose accidental
replacement loses data: nothing in `foundation` can plan it away.

**Depends on ADR-H1 for** the version, extensions and size (HA-5). Step 1 re-reads the Hub
contract; if HA-5 is answered, its values replace the defaults below in `dev.tfvars` and
`prod.tfvars` and nothing else changes.

**Files:**
- `infra/terraform/modules/postgres/{versions,main,variables,outputs}.tf`, `tests/postgres.tftest.hcl` (new)
- `infra/terraform/stacks/data/{versions,backend,providers,main,variables,outputs}.tf` (new)
- `infra/terraform/stacks/data/{dev,prod}.tfvars`, `.terraform.lock.hcl`, `tests/data.tftest.hcl` (new)

#### Contract — `modules/postgres`

```hcl
variable "environment" {
  type        = string
  description = "dev or prod."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "location" {
  type        = string
  description = "Azure region."
}
variable "resource_group_name" {
  type        = string
  description = "Resource group for the server (the foundation group)."
}
variable "unique_suffix" {
  type        = string
  description = "Task 01 convention."
  validation {
    condition     = can(regex("^[a-z0-9]{4}$", var.unique_suffix))
    error_message = "unique_suffix must match ^[a-z0-9]{4}$."
  }
}
variable "tenant_id" {
  type        = string
  description = "Entra tenant for authentication."
}
variable "server" {
  type = object({
    version               = string
    sku_name              = string
    storage_mb            = number
    zone                  = string
    backup_retention_days = number
    geo_redundant_backup  = bool
    zone_redundant_ha     = bool
    standby_zone          = string
  })
  description = "Server shape. geo_redundant_backup is create-time immutable; set it right for prod at create."
  validation {
    condition     = contains(["16", "17"], var.server.version)
    error_message = "server.version must be 16 or 17."
  }
  validation {
    condition     = can(regex("^(B|GP|MO)_Standard_", var.server.sku_name))
    error_message = "server.sku_name must be a Flexible Server SKU, e.g. B_Standard_B2s or GP_Standard_D2ds_v5."
  }
  validation {
    condition     = var.server.backup_retention_days >= 7 && var.server.backup_retention_days <= 35
    error_message = "server.backup_retention_days must be 7-35."
  }
  validation {
    condition     = !var.server.zone_redundant_ha || !startswith(var.server.sku_name, "B_")
    error_message = "Burstable SKUs cannot run zone-redundant HA."
  }
}
variable "database_names" {
  type        = list(string)
  description = "Databases to create. The Hub's is \"hub\"; our Keycloak's is \"keycloak\" (OD-30, ruling §9). HA-5 may add more."
  default     = ["hub", "keycloak"]
}
variable "extensions" {
  type        = list(string)
  description = "Values for the azure.extensions allowlist (HA-5). Empty sets nothing."
  default     = []
}
variable "admin_group" {
  type = object({
    object_id    = string
    display_name = string
  })
  description = "The Entra group that administers the server (owner input, TF_VAR_*). The deploy pipeline acts as a member of it to create the Hub's roles."
}
variable "private_endpoint_subnet_id" {
  type        = string
  description = "subnet_ids.private_endpoints."
}
variable "private_dns_zone_id" {
  type        = string
  description = "private_dns_zone_ids.postgres."
}
variable "log_analytics_workspace_id" {
  type        = string
  description = "Diagnostics destination."
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

output "server_id" { value = azurerm_postgresql_flexible_server.this.id }
output "server_name" { value = azurerm_postgresql_flexible_server.this.name }
output "fqdn" {
  description = "Resolves to the private endpoint inside the VNet through the privatelink zone."
  value       = azurerm_postgresql_flexible_server.this.fqdn
}
output "database_names" { value = var.database_names }
output "admin_group_display_name" { value = var.admin_group.display_name }
```

Resources (described; names from azurerm 4.81.0):
- `azurerm_postgresql_flexible_server.this` — `psql-panacea-app-${env}-${suffix}`; `version`,
  `sku_name`, `storage_mb`, `auto_grow_enabled = true`, `zone`, `backup_retention_days`,
  `geo_redundant_backup_enabled`, **`public_network_access_enabled = false`**,
  **`authentication { active_directory_auth_enabled = true, password_auth_enabled = false,
  tenant_id }`**, no `administrator_login`, no password attribute of any kind;
  `dynamic "high_availability"` present only when `zone_redundant_ha` (`mode = "ZoneRedundant"`,
  `standby_availability_zone`); `maintenance_window { day_of_week = 0, start_hour = 3 }`.
- `azurerm_postgresql_flexible_server_active_directory_administrator.this` — `object_id`,
  `principal_name = var.admin_group.display_name`, **`principal_type = "Group"`**, `tenant_id`.
- `azurerm_postgresql_flexible_server_database` per name (`charset = "UTF8"`,
  `collation = "en_US.utf8"`).
- `azurerm_postgresql_flexible_server_configuration`: `require_secure_transport = "ON"`,
  `ssl_min_protocol_version = "TLSv1.2"`, `log_connections = "ON"`, and `azure.extensions =
  join(",", var.extensions)` only when the list is non-empty.
- `azurerm_private_endpoint.postgres` — **`subresource_names = ["postgresqlServer"]`**, the
  zone group.
- `azurerm_monitor_diagnostic_setting.postgres` — `enabled_log { category_group = "allLogs" }`.
- **No `azurerm_management_lock`.** Prod's `CanNotDelete` lock on the server is the owner's, made
  by hand in task 31 §6a after the first prod apply (plan re-review I-4). The infra identity that
  applies this stack holds no `Microsoft.Authorization/locks/write`: Contributor excludes
  `Microsoft.Authorization/*/Write`, and Role Based Access Control Administrator grants role
  assignments only (**training knowledge, UNVERIFIED**). A lock in OpenTofu would fail the first
  prod apply. Granting the infra identity lock rights would let it lift the lock as well, so the
  lock would guard only against accidents, never against the pipeline. An owner-held lock guards
  against both.

**No database role is created here.** A Postgres role for an Entra principal is SQL
(`pgaadauth_create_principal_with_oid`, by object id, run as a member of the admin group, review
I9) and a grant is SQL; OpenTofu's
Postgres provider would need a network path and a token to the private server from the plan agent.
Task 29 writes the SQL. A **human** member of the admin group runs it once per environment, as
task 31 §7. No pipeline identity is in the admin group (the plan's "Deploy identities" table,
review I8).

**Keycloak's database (OD-30, ruling §9, §12 U-9).** Our Keycloak keeps its realm and its persistent
user sessions (the offline tokens that make launches silent) in the database `keycloak` on this
server, beside `hub`. Its role is SQL like the Hub's (task 29, run by a human, task 31 §7), for
Keycloak's own user-assigned identity. **The auth method is unsettled (U-9):** this server is
Entra-only, and Keycloak has no native Entra auth to Postgres; the path is Azure's
`azure-identity-extensions` JDBC plugin as a provider jar with workload identity, **UNVERIFIED**. If
step 1 cannot confirm it, the implementer **stops and reports**: no password role is invented and
`password_auth_enabled` stays `false` without the owner's decision.

#### Contract — `stacks/data`

```hcl
variable "environment" {
  type        = string
  description = "dev or prod."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "unique_suffix" {
  type        = string
  description = "Task 01 convention; the same value as foundation's."
}
variable "state_storage_account_name" {
  type        = string
  description = "bootstrap's output; TF_VAR_* from the pipeline."
}
variable "state_resource_group_name" {
  type        = string
  description = "bootstrap's output; TF_VAR_* from the pipeline."
}
variable "server" {
  type = object({
    version               = string
    sku_name              = string
    storage_mb            = number
    zone                  = string
    backup_retention_days = number
    geo_redundant_backup  = bool
    zone_redundant_ha     = bool
    standby_zone          = string
  })
  description = "modules/postgres server."
}
variable "extensions" {
  type        = list(string)
  description = "modules/postgres extensions (HA-5)."
  default     = []
}
variable "postgres_admin_group" {
  type = object({
    object_id    = string
    display_name = string
  })
  description = "Owner input, TF_VAR_postgres_admin_group."
}

data "terraform_remote_state" "foundation" {
  backend = "azurerm"
  config = {
    resource_group_name  = var.state_resource_group_name
    storage_account_name = var.state_storage_account_name
    container_name       = "tfstate"
    key                  = "foundation.${var.environment}.tfstate"
    use_azuread_auth     = true
  }
}

output "postgres_server_id" { value = module.postgres.server_id }
output "postgres_server_name" { value = module.postgres.server_name }
output "postgres_fqdn" { value = module.postgres.fqdn }
output "postgres_database_names" { value = module.postgres.database_names }
```

`dev.tfvars`: `server = { version = "16", sku_name = "B_Standard_B2s", storage_mb = 32768,
zone = "1", backup_retention_days = 7, geo_redundant_backup = false, zone_redundant_ha = false,
standby_zone = "2" }`. `prod.tfvars`: `GP_Standard_D2ds_v5`, retention 35,
`geo_redundant_backup = true`, `zone_redundant_ha = true`. Both files carry `unique_suffix` and
`environment` only besides `server`; the admin group arrives as `TF_VAR_postgres_admin_group`.

#### Tests to write first

`modules/postgres/tests/postgres.tftest.hcl`:

| Run | Asserts | Mutant it kills |
|---|---|---|
| `password_auth_is_off_and_entra_on` | `authentication[0].password_auth_enabled == false`, `active_directory_auth_enabled == true` | password auth left on (the provider's default) |
| `server_is_not_public` | `public_network_access_enabled == false` | a public server |
| `administrator_is_a_group` | the AD administrator's `principal_type == "Group"` and `object_id == var.admin_group.object_id` | a single user as the only admin, lost when they leave |
| `tls_is_required` | a configuration `require_secure_transport` with value `ON` | TLS optional |
| `endpoint_targets_postgresqlServer_in_the_privatelink_zone` | `subresource_names == ["postgresqlServer"]`; zone group holds the zone id | a PE with no DNS |
| `no_extensions_sets_no_allowlist` | `extensions = []` → no `azure.extensions` configuration instance | an empty allowlist value Azure rejects |
| `extensions_are_joined` | `["pgcrypto", "uuid-ossp"]` → value `pgcrypto,uuid-ossp` | one extension dropped |
| `ha_block_only_when_asked` | `zone_redundant_ha = false` → zero `high_availability` blocks; `true` with a GP SKU → one, `ZoneRedundant` | HA billed in dev |
| `burstable_ha_is_rejected` | `B_Standard_B2s` with `zone_redundant_ha = true` → `expect_failures` | a plan Azure rejects at apply |
| `hub_database_exists_by_default` | two databases, `hub` and `keycloak` (OD-30) | the Hub or our Keycloak starting against no database |

`stacks/data/tests/data.tftest.hcl` (with `override_data` giving `data.terraform_remote_state.foundation`
the outputs task 07 declares): `committed_tfvars_plan` for each tfvars file (no file-level or
run-level key that a tfvars sets, guard G10, review I1); and
`reads_the_foundation_it_is_given`: the module's `private_endpoint_subnet_id` equals the mocked
`subnet_ids.private_endpoints` (kills a hard-coded subnet name).

#### Steps

- [ ] **Step 1:** Re-read the Hub contract (`.claude/worktrees/hub/docs/superpowers/contracts/`).
  If HA-5 is answered, use its version, extensions and size; if it names a second database or a
  schema split, add it to `database_names`. Record what you used in the commit message. Then
  settle U-9 (Keycloak's Entra auth to this server, above) from Microsoft's and Keycloak's current
  documentation; if it does not hold, stop and report before step 2.
- [ ] **Step 2:** The module: variables, tests, resources, outputs.
- [ ] **Step 3:** The stack: variables, the remote-state data source, the module call, tfvars,
  lock file, tests.
- [ ] **Step 4:** guards, validate, commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/postgres infra/terraform/stacks/data
# validate: OK   infra/terraform/modules/postgres
# validate: OK   infra/terraform/stacks/data
# validate: 2 ok, 0 failed
bash infra/scripts/guards.sh      # guards: 10 of 10 passed  (G8: no password attribute)
grep -rn "azurerm_management_lock" infra/terraform/modules/postgres infra/terraform/stacks/data > lock.out; echo "exit=$?"
# exit=1 and lock.out empty: prod's lock is the owner's (task 31 §6a, plan re-review I-4)
```

#### What this task does not claim

- That Entra-only Flexible Server with a private endpoint (rather than VNet injection) accepts
  every setting above together — Azure validates the combination only at apply (task 31).
- That `allLogs` is the right diagnostic category group for Flexible Server — **UNVERIFIED**.
- That `B_Standard_B2s` is enough for the Hub; HA-5 answers that.
