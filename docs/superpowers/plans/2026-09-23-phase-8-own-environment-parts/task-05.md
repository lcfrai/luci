### Task 05: `modules/registry` and `modules/keyvault` — both in OpenTofu, both private

Panacea creates ACR and Key Vault with `az … create` in its deploy script and references them as
`data` sources (survey §1.8, `:178-180`), pulls with the shared ACR admin credential because its
deploy identity cannot assign `AcrPull` (`:207-209`), and so has a registry outside state and a
secret in every node. Here both are OpenTofu resources, the admin user is off, and pulls are the
kubelet identity's `AcrPull` (granted in task 07, where the kubelet identity is known).

**Files:**
- `infra/terraform/modules/registry/{versions,main,variables,outputs}.tf`, `tests/registry.tftest.hcl` (new)
- `infra/terraform/modules/keyvault/{versions,main,variables,outputs}.tf`, `tests/keyvault.tftest.hcl` (new)

#### Contract — `modules/registry`

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
  description = "The foundation resource group."
}
variable "unique_suffix" {
  type        = string
  description = "Four lowercase alphanumerics (task 01 convention)."
  validation {
    condition     = can(regex("^[a-z0-9]{4}$", var.unique_suffix))
    error_message = "unique_suffix must match ^[a-z0-9]{4}$."
  }
}
variable "public_network_access" {
  type        = bool
  description = "true in dev: hosted CI agents push from no fixed IP. false in prod, where the in-VNet pool pushes through the private endpoint."
}
variable "zone_redundant" {
  type        = bool
  description = "Zone-redundant registry (prod)."
}
variable "untagged_retention_days" {
  type        = number
  description = "Days before untagged manifests are purged. Images are deployed by digest, so a tag is only a convenience."
  default     = 7
  validation {
    condition     = var.untagged_retention_days >= 1 && var.untagged_retention_days <= 365
    error_message = "untagged_retention_days must be 1-365."
  }
}
variable "private_endpoint_subnet_id" {
  type        = string
  description = "modules/network subnet_ids.private_endpoints."
}
variable "private_dns_zone_id" {
  type        = string
  description = "modules/network private_dns_zone_ids.acr."
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

output "id" { value = azurerm_container_registry.this.id }
output "name" { value = azurerm_container_registry.this.name }
output "login_server" { value = azurerm_container_registry.this.login_server }
output "data_endpoint_host_names" {
  description = "The dedicated data endpoints, which a firewall allowlists instead of *.blob.core.windows.net (survey §1.6, :166-179)."
  value       = azurerm_container_registry.this.data_endpoint_host_names
}
```

Resources: `azurerm_container_registry.this` — `acrpanaceaapp${env}${suffix}`,
**`sku = "Premium"`** (private endpoints require it, survey §6.2 `:513`), **`admin_enabled = false`**,
**`anonymous_pull_enabled = false`**, `data_endpoint_enabled = true`,
`public_network_access_enabled = var.public_network_access`,
`network_rule_bypass_option = "AzureServices"`, `zone_redundancy_enabled = var.zone_redundant`,
`retention_policy_in_days = var.untagged_retention_days`, and
`export_policy_enabled = var.public_network_access` (Azure accepts `false` only with public
access off). `azurerm_private_endpoint.acr` — `subresource_names = ["registry"]`, a
`private_dns_zone_group` with `var.private_dns_zone_id`.

#### Contract — `modules/keyvault`

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
  description = "The foundation resource group."
}
variable "unique_suffix" {
  type        = string
  description = "Four lowercase alphanumerics (task 01 convention)."
  validation {
    condition     = can(regex("^[a-z0-9]{4}$", var.unique_suffix))
    error_message = "unique_suffix must match ^[a-z0-9]{4}$."
  }
}
variable "tenant_id" {
  type        = string
  description = "Entra tenant. Supplied from data.azurerm_client_config in the stack, never typed."
}
variable "public_network_access" {
  type        = bool
  description = "false in both environments unless a deploy step genuinely needs the data plane from outside the VNet."
}
variable "allowed_ip_ranges" {
  type        = list(string)
  description = "Firewall allowlist used only when public_network_access is true."
  default     = []
}
variable "private_endpoint_subnet_id" {
  type        = string
  description = "modules/network subnet_ids.private_endpoints."
}
variable "private_dns_zone_id" {
  type        = string
  description = "modules/network private_dns_zone_ids.vault."
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

output "id" { value = azurerm_key_vault.this.id }
output "name" { value = azurerm_key_vault.this.name }
output "vault_uri" { value = azurerm_key_vault.this.vault_uri }
```

Resources: `azurerm_key_vault.this` — `kv-panacea-app-${env}-${suffix}` (≤ 24 characters),
`sku_name = "standard"`, **`rbac_authorization_enabled = true`** (4.81.0's name;
`enable_rbac_authorization` is deprecated in this version), **`purge_protection_enabled = true`**,
`soft_delete_retention_days = 90`, `public_network_access_enabled`,
`network_acls { default_action = "Deny", bypass = "AzureServices", ip_rules = var.allowed_ip_ranges }`.
`azurerm_private_endpoint.vault` — `subresource_names = ["vault"]`, DNS zone group.

**What the vault holds in this phase:** nothing OpenTofu writes. It exists for what ADR-H1's Hub
needs (HA-4) and for a KPMG-CA certificate if owner question Q2 is answered that way. Role
assignments on it are task 10's.

#### Tests to write first

`registry.tftest.hcl` (mocks only):

| Run | Asserts | Mutant it kills |
|---|---|---|
| `registry_is_premium_with_no_admin_user` | `sku == "Premium"`, `admin_enabled == false`, `anonymous_pull_enabled == false` | Standard SKU (no private endpoint), or the admin credential Panacea runs on |
| `registry_uses_dedicated_data_endpoints` | `data_endpoint_enabled == true` | data served from shared `*.blob` hosts a firewall cannot scope |
| `registry_name_is_alphanumeric_and_suffixed` | name matches `^[a-z0-9]{5,50}$` and ends with the suffix | a hyphen (ACR rejects it only at apply) |
| `registry_private_endpoint_targets_registry_and_the_acr_zone` | `subresource_names == ["registry"]`; the zone group holds `var.private_dns_zone_id` | an endpoint with no DNS, which resolves to the public address |
| `export_policy_follows_public_access` | `public_network_access = false` → `export_policy_enabled == false`; `true` → `true` | a plan Azure rejects at apply |

`keyvault.tftest.hcl` (mocks only):

| Run | Asserts | Mutant it kills |
|---|---|---|
| `vault_is_rbac_only` | `rbac_authorization_enabled == true`, no `access_policy` | access policies, which bypass RBAC review |
| `vault_is_purge_protected_for_90_days` | `purge_protection_enabled == true`, `soft_delete_retention_days == 90` | a vault that can be purged the day it is deleted |
| `vault_network_defaults_to_deny` | `network_acls[0].default_action == "Deny"` | Allow |
| `vault_name_fits_24` | `length(name) <= 24` for `environment = "prod"` | a name Azure rejects at apply |
| `vault_private_endpoint_targets_vault_and_its_zone` | `subresource_names == ["vault"]`; the zone id | as for the registry |

#### Steps

- [ ] **Step 1:** Both modules' variables and tests; validate fails for both.
- [ ] **Step 2:** Registry resources until its five runs pass; then the vault's until its five pass.
- [ ] **Step 3:** Outputs, fmt, guards, commit both modules together (one commit, two pathspecs).

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/registry infra/terraform/modules/keyvault
# validate: OK   infra/terraform/modules/registry
# validate: OK   infra/terraform/modules/keyvault
# validate: 2 ok, 0 failed
```

#### What this task does not claim

- That `export_policy_enabled` must follow public access exactly as stated — Azure's pairing rule
  is **from training knowledge, UNVERIFIED**; the test pins the rule this module applies, and task
  31's first apply is where Azure agrees or not.
- That the registry is reachable from hosted agents in dev with no IP rules: it is, by design, with
  Entra auth as the only gate; that is the dev trade-off, not the prod one.
