### Task 07: `stacks/foundation` — the composition, the firewall option, DNS, the edge IP, the invariants

This stack is where the modules of tasks 03–06 become one environment. It adds only what needs two
modules at once: the grants between them, the edge's public IP and its DNS records, the Azure
Firewall alternative to NAT (`modules/egress-firewall`, OQ-3), and the **invariants that must fail
the plan** — not warn — when an input is wrong.

Panacea's lessons applied here: the wildcard A record there is a manual step after Traefik gets an
IP (survey §1.5 `:124-125`), so here **Terraform owns the IP and the records**; its firewall
allowlists ACR's dedicated data endpoint rather than `*.blob.core.windows.net` (`:147`), so ours
does too; and its region guard is a `check` that only warns (`P:deploy/firewall.tf:12-17`), so ours
are **preconditions** on a `terraform_data` resource, which stop the plan.

**Files:**
- `infra/terraform/modules/egress-firewall/{versions,main,variables,outputs}.tf`, `tests/egress-firewall.tftest.hcl` (new)
- `infra/terraform/stacks/foundation/{versions,backend,providers,main,edge,grants,invariants,variables,outputs}.tf` (new)
- `infra/terraform/stacks/foundation/{dev,prod}.tfvars`, `.terraform.lock.hcl` (new)
- `infra/terraform/stacks/foundation/tests/foundation.tftest.hcl` (new)

#### Contract — `modules/egress-firewall`

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
variable "firewall_subnet_id" {
  type        = string
  description = "modules/network subnet_ids.firewall (AzureFirewallSubnet)."
}
variable "routed_subnet_ids" {
  type        = map(string)
  description = "Every egress subnet, whose default route goes through the firewall: system, apps, sandbox, agents (modules/network egress_subnet_ids — the same four nat mode NATs)."
  validation {
    condition     = alltrue([for k in ["system", "apps", "sandbox", "agents"] : contains(keys(var.routed_subnet_ids), k)])
    error_message = "routed_subnet_ids must hold system, apps, sandbox and agents: a subnet left off has no egress at all (default outbound access is off, task 03)."
  }
}
variable "ci_agent_source_cidrs" {
  type        = list(string)
  description = "The agents subnet's range only (modules/network subnet_cidrs.agents). The CI allowlist below applies from here and nowhere else, so a sandbox never gains the CI agents' reach."
}
variable "ci_agent_allowed_fqdns" {
  type        = list(string)
  description = "What the Managed DevOps Pool's agents (task 30) may reach on 443, beyond rule 2's AKS, ACR and Entra entries. A starting list from training knowledge, UNVERIFIED (task 32 P22): a missing entry fails closed — the pool registers no agent, or a pipeline step fails — and the firewall's deny log names the host."
  default = [
    # Azure DevOps: the agent's service connection, logs, artefacts, agent updates
    "dev.azure.com",
    "*.dev.azure.com",
    "*.visualstudio.com",
    "*.vsassets.io",
    "*.vsblob.visualstudio.com",
    "vstsagentpackage.azureedge.net",
    "download.agent.dev.azure.com",
    # Managed DevOps Pools' own service endpoints. The queue entry is a storage wildcard like the one
    # the ACR rule refuses; it is tolerable here only because ci_agent_source_cidrs confines it to
    # the agents subnet.
    "*.prod.manageddevops.microsoft.com",
    "rmprodbuilds.azureedge.net",
    "*.queue.core.windows.net",
    "server.pipe.aria.microsoft.com",
    # Azure Resource Manager (terraform apply, az CLI)
    "management.azure.com",
    # Terraform and Helm, and the chart repositories the deploy pipeline builds from (task 13)
    "registry.terraform.io",
    "releases.hashicorp.com",
    "get.helm.sh",
    "charts.jetstack.io",
    "traefik.github.io",
    "github.com",
    "objects.githubusercontent.com",
    # The agent image's OS packages
    "azure.archive.ubuntu.com",
    "packages.microsoft.com",
  ]
}
variable "source_cidrs" {
  type        = list(string)
  description = "Addresses allowed out: the VNet range (nodes SNAT pod traffic to their own address under Overlay)."
}
variable "acr_data_endpoints" {
  type        = list(string)
  description = "modules/registry data_endpoint_host_names, allowlisted instead of *.blob.core.windows.net."
}
variable "agent_allowed_fqdns" {
  type        = list(string)
  description = "What a coding agent in a sandbox may reach on 443. The default is a starting list from training knowledge, UNVERIFIED; an allowlist that misses one registry breaks agents silently (Q1)."
  default = [
    "registry.npmjs.org",
    "pypi.org",
    "files.pythonhosted.org",
    "proxy.golang.org",
    "sum.golang.org",
    "github.com",
    "*.githubusercontent.com",
    "codeload.github.com",
    "api.github.com",
    "dev.azure.com",
    "*.visualstudio.com",
    "api.anthropic.com",
    "claude.ai",
    "console.anthropic.com",
    "statsig.anthropic.com",
  ]
}
variable "log_analytics_workspace_id" {
  type        = string
  description = "Firewall logs destination."
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

output "firewall_private_ip" { value = azurerm_firewall.this.ip_configuration[0].private_ip_address }
output "egress_public_ip" { value = azurerm_public_ip.firewall.ip_address }
output "route_table_id" { value = azurerm_route_table.egress.id }
output "routed_subnet_keys" { value = sort(keys(azurerm_subnet_route_table_association.this)) }   # the foundation test reads it
```

Resources: a Standard static `azurerm_public_ip.firewall`; `azurerm_firewall_policy.this`
(`sku = "Standard"`, `dns { proxy_enabled = true }`, `threat_intelligence_mode = "Deny"`);
`azurerm_firewall.this` (`sku_name = "AZFW_VNet"`, `sku_tier = "Standard"`, the policy, zones 1–3);
one `azurerm_firewall_policy_rule_collection_group` with, in priority order:
1. network rules — UDP 123 to `*` (NTP), UDP/TCP 53 to `168.63.129.16` only;
2. application rules — `AzureKubernetesService` FQDN tag on 443 (AKS's own required egress),
   `var.acr_data_endpoints` + `*.azurecr.io` login server on 443, Entra
   (`login.microsoftonline.com`, `graph.microsoft.com`) on 443;
3. application rules — **`var.agent_allowed_fqdns` on 443, from `var.source_cidrs`**;
4. application rules — **`var.ci_agent_allowed_fqdns` on 443, from `var.ci_agent_source_cidrs`
   only** (review C2: the CI pool's agents in `snet-agents`);
5. nothing else: the policy's default is deny.

And `azurerm_route_table.egress` (`bgp_route_propagation_enabled = false`, one route `0.0.0.0/0`
→ `VirtualAppliance` at the firewall's private IP) with one `azurerm_subnet_route_table_association.this`
per `routed_subnet_ids` entry (`for_each`; four, `agents` among them), plus a diagnostic setting to the
workspace. That a route table can be associated with a subnet delegated to
`Microsoft.DevOpsInfrastructure/pools` is **from training knowledge, UNVERIFIED**.

#### Contract — `stacks/foundation`

```hcl
# versions.tf — azurerm 4.81.0; backend.tf — backend "azurerm" {}
# providers.tf
provider "azurerm" {
  features {}
  storage_use_azuread = true
}
data "azurerm_client_config" "current" {}

# variables.tf
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
  description = "Azure region. Australian regions only (residency; ADR-H1 §1.4 force 4)."
}
variable "unique_suffix" {
  type        = string
  description = "Four lowercase alphanumerics (task 01)."
}
variable "vnet_cidr" {
  type        = string
  description = "A /16 (owner input)."
}
variable "aks_network" {
  type = object({
    pod_cidr       = string
    service_cidr   = string
    dns_service_ip = string
  })
  description = "Overlay pod range and service range; must not overlap vnet_cidr or each other (invariant I2)."
}
variable "egress_mode" {
  type        = string
  description = "nat (dev) or firewall (Q1)."
}
variable "allowed_ingress_cidrs" {
  type        = list(string)
  description = "Who reaches the edge on 443 (Q3)."
}
variable "api_authorized_ip_ranges" {
  type        = list(string)
  description = "Extra CIDRs allowed to the public API server (the owner's admin workstations). The environment's own egress /32 (the NAT gateway's in nat mode, the firewall's in firewall mode) is added automatically: the in-VNet agent pool egresses through it (task 03's egress_subnets include agents), so that is the address the API server sees."
  default     = []
}
variable "api_private" {
  type        = bool
  description = "true in prod (OQ-5): a private cluster, create-time immutable."
}
variable "kubernetes_version" {
  type        = string
  description = "Minor version, e.g. 1.33."
}
variable "aks_sku_tier" {
  type        = string
  description = "Free or Standard."
}
variable "system_pool" {
  type = object({
    vm_size   = string
    min_count = number
    max_count = number
  })
  description = "modules/aks system_pool without zones (zones come from var.zones)."
}
variable "apps_pool" {
  type = object({
    vm_size   = string
    min_count = number
    max_count = number
  })
  description = "modules/aks apps_pool without zones."
}
variable "sandbox_pool" {
  type = object({
    vm_size         = string
    min_count       = number
    max_count       = number
    os_disk_size_gb = number
  })
  description = "modules/aks sandbox_pool without zones. min_count is the warm floor."
}
variable "zones" {
  type        = list(string)
  description = "Availability zones for every pool, e.g. [\"1\", \"2\", \"3\"]."
}
variable "maintenance" {
  type = object({
    day_of_week    = string
    start_time     = string
    utc_offset     = string
    duration_hours = number
  })
  description = "modules/aks maintenance."
}
variable "cluster_admin_group_object_ids" {
  type        = list(string)
  description = "Owner input, TF_VAR_*."
}
variable "deploy_principal_object_id" {
  type        = string
  description = "The pipeline's identity (owner input, TF_VAR_*): AcrPush, and AKS RBAC Cluster Admin to install charts."
}
variable "dns_zone_name" {
  type        = string
  description = "The delegated public zone for the edge, e.g. panacea-app.example (owner input)."
}
variable "edge_hostnames" {
  type        = list(string)
  description = "Record names under dns_zone_name pointing at the edge IP."
  default     = ["hub", "broker"]
}
variable "log_retention_days" {
  type        = number
  description = "modules/observability retention_days."
}
variable "log_daily_quota_gb" {
  type        = number
  description = "modules/observability daily_quota_gb."
}
variable "alert_emails" {
  type        = list(string)
  description = "Owner input."
}
variable "monthly_budget_amount" {
  type        = number
  description = "Owner input."
}
variable "budget_start_date" {
  type        = string
  description = "First of a month, RFC 3339."
}
variable "registry_public_network_access" {
  type        = bool
  description = "modules/registry public_network_access."
}
variable "tags" {
  type        = map(string)
  description = "Extra tags."
  default     = {}
}

# outputs.tf — the next stacks and the deploy pipeline read these, never a guessed name
output "resource_group_name" { value = azurerm_resource_group.this.name }
output "location" { value = var.location }
output "vnet_id" { value = module.network.vnet_id }
output "subnet_ids" { value = module.network.subnet_ids }
output "private_dns_zone_ids" { value = module.network.private_dns_zone_ids }
output "log_analytics_workspace_id" { value = module.observability.workspace_id }
output "acr_id" { value = module.registry.id }
output "acr_login_server" { value = module.registry.login_server }
output "key_vault_id" { value = module.keyvault.id }
output "key_vault_uri" { value = module.keyvault.vault_uri }
output "aks_id" { value = module.aks.cluster_id }
output "aks_name" { value = module.aks.cluster_name }
output "aks_oidc_issuer_url" { value = module.aks.oidc_issuer_url }
output "aks_node_resource_group" { value = module.aks.node_resource_group }
output "aks_secrets_provider_identity_object_id" { value = module.aks.secrets_provider_identity_object_id }
output "sandbox_pool" { value = module.aks.sandbox_pool }
output "dns_zone_name" { value = azurerm_dns_zone.this.name }
output "dns_zone_id" { value = azurerm_dns_zone.this.id }
output "dns_zone_name_servers" {
  description = "Hand these to the parent zone's owner for delegation (owner input)."
  value       = azurerm_dns_zone.this.name_servers
}
output "edge_public_ip" { value = azurerm_public_ip.edge.ip_address }
output "edge_public_ip_name" { value = azurerm_public_ip.edge.name }
output "edge_fqdns" { value = { for h in var.edge_hostnames : h => "${h}.${var.dns_zone_name}" } }
output "egress_public_ip" {
  description = "Where sandboxes appear to come from. Providers or git hosts that allowlist need it."
  value       = var.egress_mode == "nat" ? module.network.nat_public_ip : module.egress_firewall[0].egress_public_ip
}
```

#### The invariants (`invariants.tf`) — preconditions, so they fail the plan

```hcl
locals {
  australian_regions = ["australiaeast", "australiasoutheast", "australiacentral", "australiacentral2"]

  # Two CIDRs overlap iff their networks agree when both are cut to the shorter prefix.
  cidrs = {
    vnet    = var.vnet_cidr
    pod     = var.aks_network.pod_cidr
    service = var.aks_network.service_cidr
  }
  cidr_pairs = [["vnet", "pod"], ["vnet", "service"], ["pod", "service"]]
  overlaps = [
    for p in local.cidr_pairs : p if(
      cidrsubnet("${cidrhost(local.cidrs[p[0]], 0)}/${min(tonumber(split("/", local.cidrs[p[0]])[1]), tonumber(split("/", local.cidrs[p[1]])[1]))}", 0, 0)
      ==
      cidrsubnet("${cidrhost(local.cidrs[p[1]], 0)}/${min(tonumber(split("/", local.cidrs[p[0]])[1]), tonumber(split("/", local.cidrs[p[1]])[1]))}", 0, 0)
    )
  ]
  service_prefix = tonumber(split("/", var.aks_network.service_cidr)[1])
}

resource "terraform_data" "invariants" {
  lifecycle {
    precondition {
      condition     = contains(local.australian_regions, var.location)
      error_message = "I1: location must be an Australian region (residency)."
    }
    precondition {
      condition     = length(local.overlaps) == 0
      error_message = "I2: these ranges overlap: ${jsonencode(local.overlaps)}."
    }
    precondition {
      condition     = cidrsubnet("${var.aks_network.dns_service_ip}/${local.service_prefix}", 0, 0) == cidrsubnet(var.aks_network.service_cidr, 0, 0)
      error_message = "I3: dns_service_ip must lie inside service_cidr."
    }
    precondition {
      condition     = var.api_private || length(var.api_authorized_ip_ranges) > 0 || var.egress_mode == "nat"
      error_message = "I4: a public API server needs at least one authorised range; in firewall mode there is no NAT address to add automatically."
    }
    precondition {
      condition     = var.environment != "prod" || var.api_private
      error_message = "I5: prod is a private cluster (OQ-5)."
    }
  }
}
```

Every module call `depends_on = [terraform_data.invariants]` would be redundant (a failed
precondition already stops the plan); instead, the module calls read nothing from it and the plan
simply never gets that far.

#### The rest (`main.tf`, `edge.tf`, `grants.tf`, described)

- `azurerm_resource_group.this` — `rg-panacea-app-${env}`.
- `module "network"`, `"observability"`, `"registry"`, `"keyvault"`, `"aks"` with the variables
  above; `module "egress_firewall"` with `count = var.egress_mode == "firewall" ? 1 : 0`,
  `routed_subnet_ids = module.network.egress_subnet_ids` (one source for both modes: the four
  subnets nat mode NATs are the four firewall mode routes) and
  `ci_agent_source_cidrs = [module.network.subnet_cidrs.agents]`.
  The AKS module gets `outbound_type = var.egress_mode == "nat" ? "userAssignedNATGateway" :
  "userDefinedRouting"` and `authorized_ip_ranges = concat(var.api_authorized_ip_ranges,
  ["${var.egress_mode == "nat" ? module.network.nat_public_ip : module.egress_firewall[0].egress_public_ip}/32"])`
  (review C2: in either mode the CI agents reach the API from the environment's egress address; a
  private cluster ignores the list, task 06), and `depends_on` the
  firewall module in firewall mode (a UDR cluster that boots before its allow rules comes up
  broken — Panacea's pentest #2, survey §1.3 `:99-100`).
- `edge.tf`: `azurerm_public_ip.edge` — `pip-edge-panacea-app-${env}`, Standard, Static, zones;
  `azurerm_dns_zone.this`; one `azurerm_dns_a_record` per `edge_hostnames` entry with
  **`target_resource_id = azurerm_public_ip.edge.id`** (an alias record: it follows the IP) and
  `ttl = 300`.
- `grants.tf`: kubelet → **AcrPull** on the registry; `deploy_principal_object_id` → **AcrPush** on
  the registry, **Azure Kubernetes Service RBAC Cluster Admin** and **Azure Kubernetes Service
  Cluster User Role** on the cluster; the AKS control-plane identity → **Network Contributor** on
  `azurerm_public_ip.edge` (so the Traefik Service can bind it by name — whether the cloud provider
  also needs read on the resource group is **UNVERIFIED**, task 31 checks).

#### `dev.tfvars` (complete — holds no id)

```hcl
environment    = "dev"
location       = "australiaeast"
unique_suffix  = "k7q2"
vnet_cidr      = "10.40.0.0/16"
aks_network = {
  pod_cidr       = "172.20.0.0/16"
  service_cidr   = "172.16.0.0/16"
  dns_service_ip = "172.16.0.10"
}
egress_mode           = "nat"
allowed_ingress_cidrs = []   # owner input (Q3); empty = nothing outside reaches the edge
api_private           = false
kubernetes_version    = "1.33" # UNVERIFIED as an AKS-supported minor on the day of first apply
aks_sku_tier          = "Free"
zones                 = ["1", "2", "3"]
system_pool  = { vm_size = "Standard_D2s_v5", min_count = 1, max_count = 2 }
apps_pool    = { vm_size = "Standard_D4s_v5", min_count = 1, max_count = 3 }
sandbox_pool = { vm_size = "Standard_D8s_v5", min_count = 1, max_count = 4, os_disk_size_gb = 128 }
maintenance = {
  day_of_week    = "Sunday"
  start_time     = "02:00"
  utc_offset     = "+10:00"
  duration_hours = 4
}
dns_zone_name                  = "dev.panacea-app.invalid" # placeholder until the owner delegates a name
log_retention_days             = 30
log_daily_quota_gb             = 5
alert_emails                   = ["owner@example.invalid"] # placeholder; owner input
monthly_budget_amount          = 3000
budget_start_date              = "2026-11-01T00:00:00Z"
registry_public_network_access = true
```

`prod.tfvars` differs in: `environment = "prod"`, its own `unique_suffix` and `vnet_cidr`,
`api_private = true`, `aks_sku_tier = "Standard"`, larger pools, `log_retention_days = 90`
(pending the ISM figure), `registry_public_network_access = false`, and a real budget. It sets
`egress_mode = "nat"` **with a comment that Q1 decides it**.

The `.invalid` TLD (RFC 2606) makes an unedited placeholder fail at apply rather than succeed
against someone else's domain.

#### Tests to write first

`modules/egress-firewall/tests/egress-firewall.tftest.hcl`:

| Run | Asserts | Mutant it kills |
|---|---|---|
| `default_route_goes_to_the_firewall` | one route `0.0.0.0/0`, `next_hop_type == "VirtualAppliance"`, next hop = the firewall's private IP (override_resource) | a route to `Internet` that bypasses the firewall |
| `every_egress_subnet_is_routed` | four associations for the four `routed_subnet_ids` (system, apps, sandbox, agents); a map missing `agents` → `expect_failures = [var.routed_subnet_ids]` | the sandbox subnet left on the default route, or **the CI agents left with no route at all (C2)** |
| `ci_agents_get_their_allowlist_from_their_subnet_only` | the CI rule's `destination_fqdns == var.ci_agent_allowed_fqdns`, HTTPS 443 only, `source_addresses == var.ci_agent_source_cidrs`; the coding-agent rule does not contain `management.azure.com` | the CI list granted to the whole VNet, so a sandbox can reach Azure Resource Manager |
| `acr_is_allowed_by_data_endpoint_not_blob_wildcard` | the ACR rule's FQDNs contain the data endpoints and no `*.blob.core.windows.net` | the wildcard that lets a sandbox exfiltrate to any storage account |
| `agents_get_exactly_the_allowlist` | the agent rule's `destination_fqdns == var.agent_allowed_fqdns` and its protocol is HTTPS 443 only | an allow-all application rule |
| `dns_goes_only_to_azure_dns` | the DNS network rule's destination is `["168.63.129.16"]` | DNS to `*`, a classic tunnel |

`stacks/foundation/tests/foundation.tftest.hcl` (run by `validate.sh` once with `dev.tfvars` and once
with `prod.tfvars`; supplies `cluster_admin_group_object_ids` and `deploy_principal_object_id`
with all-ones GUIDs):

| Run | Asserts | Mutant it kills |
|---|---|---|
| `tfvars_plan_against_mocks` | `command = plan` succeeds with the file's values | a committed tfvars that does not plan |
| `overlapping_pod_range_fails_the_plan` | `aks_network.pod_cidr = "10.40.128.0/17"` → `expect_failures = [terraform_data.invariants]` | I2 written as a `check` (warning only) |
| `dns_ip_outside_service_range_fails` | `dns_service_ip = "172.17.0.10"` → `expect_failures` | I3 dropped |
| `non_australian_region_fails` | `location = "eastus"` → `expect_failures` | I1 dropped |
| `prod_must_be_private` | `environment = "prod"`, `api_private = false` → `expect_failures` | I5 dropped |
| `nat_mode_allowlists_its_own_address_on_the_api` | `command = apply` with `override_resource` giving the NAT IP `20.0.0.1` → the AKS module's authorised ranges include `20.0.0.1/32` | the in-VNet pool locked out of its own cluster |
| `firewall_mode_allowlists_its_own_address_on_the_api` | the run pins `environment = "dev"`, `api_private = false`, `egress_mode = "firewall"` (so it means the same under either tfvars file: a private cluster has no authorised ranges, review I1/I5), `command = apply` with `override_resource` giving the firewall's public IP `20.0.0.2` → the authorised ranges include `20.0.0.2/32` | the same lock-out in firewall mode, where the agents egress through the firewall, not a NAT |
| `agents_egress_in_both_modes` | nat mode: `module.network.egress_subnet_ids` has key `agents`; firewall mode: `module.egress_firewall[0].routed_subnet_keys == ["agents", "apps", "sandbox", "system"]` | the two modes drifting apart, so one of them strands the CI pool (C2) |
| `firewall_mode_uses_udr_and_has_no_nat` | `egress_mode = "firewall"` → `module.egress_firewall` has one instance and the cluster's `outbound_type == "userDefinedRouting"` | a UDR cluster without the firewall, which boots with no egress |
| `edge_records_alias_the_edge_ip` | one A record per hostname, each `target_resource_id == azurerm_public_ip.edge.id`, `ttl == 300` | a literal IP record that goes stale on the first re-create |
| `kubelet_pulls_and_deploy_pushes` | an `AcrPull` assignment to the kubelet identity and an `AcrPush` to the deploy principal, both scoped to the registry | the shared admin credential's replacement missing |

#### Steps

- [ ] **Step 1:** `modules/egress-firewall`: variables, tests, resources, outputs — its six runs pass.
- [ ] **Step 2:** The stack's variables, the invariants and the test file; the invariant runs pass
  before any module is called (the preconditions need no module).
- [ ] **Step 3:** Module calls, `edge.tf`, `grants.tf`, outputs; the remaining runs pass.
- [ ] **Step 4:** `dev.tfvars`, `prod.tfvars`; the lock file for both platforms; guards (G2 proves
  the tfvars hold no GUID); commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/egress-firewall infra/terraform/stacks/foundation
# validate: OK   infra/terraform/modules/egress-firewall
# validate: OK   infra/terraform/stacks/foundation        (tests pass for dev.tfvars and prod.tfvars)
# validate: 2 ok, 0 failed
bash infra/scripts/guards.sh       # guards: 8 of 8 passed
```

This stack's tests plan every module; allow several minutes and run it in the background.

#### What this task does not claim

- That `agent_allowed_fqdns` is complete or right — it is a starting list from training knowledge
  (Q1). Task 32 runs a real agent through it.
- That `ci_agent_allowed_fqdns` is complete or right — the same kind of starting list, for the CI
  pool (review C2); task 32's P22 runs a real pipeline through it. It deliberately omits the dev
  state account's public blob host: in firewall mode with a public state account, add that one
  host (never `*.blob.core.windows.net`), or reach the state through its private endpoint.
- That `1.33` is a supported AKS minor on the day of first apply; the runbook (task 31) checks
  `az aks get-versions` first.
- That the DNS zone resolves: delegation from the parent is an owner input.
