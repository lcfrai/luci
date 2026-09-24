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
    "objects.githubusercontent.com",          # release downloads, historically
    "release-assets.githubusercontent.com",   # where GitHub release downloads now redirect (training knowledge, UNVERIFIED; P22 sees the deny log either way)
    # The agent image's OS packages over HTTPS. Ubuntu's archive is plain HTTP: see ci_agent_allowed_http_fqdns.
    "packages.microsoft.com",
  ]
}
variable "ci_agent_allowed_http_fqdns" {
  type        = list(string)
  description = "Hosts the CI agents may reach on HTTP 80 only, from ci_agent_source_cidrs only. apt fetches Ubuntu's archive over http by default and verifies each package by its signed Release file, so the transport is not the integrity control. Nothing else goes here; a 443 host never moves to this list."
  default     = ["azure.archive.ubuntu.com"]   # review M4: on the 443-only rule, apt fails (Panacea's build egress hit exactly this)
  validation {
    condition     = alltrue([for f in var.ci_agent_allowed_http_fqdns : !strcontains(f, "*")])
    error_message = "ci_agent_allowed_http_fqdns takes exact hosts only: no wildcard is allowed out in the clear."
  }
}
variable "source_cidrs" {
  type        = list(string)
  description = "Addresses allowed out: the VNet range (nodes SNAT pod traffic to their own address under Overlay). Because of that SNAT the firewall cannot tell a sandbox from its node: every rule here is open to both (review I10, Q1)."
}
variable "acr_login_server" {
  type        = string
  description = "modules/registry login_server (<name>.azurecr.io), allowlisted exactly. Never *.azurecr.io: that matches every other tenant's registry (review I10)."
  validation {
    condition     = !startswith(var.acr_login_server, "*") && endswith(var.acr_login_server, ".azurecr.io")
    error_message = "acr_login_server must be our registry's exact login server, not a wildcard."
  }
}
variable "acr_data_endpoints" {
  type        = list(string)
  description = "modules/registry data_endpoint_host_names (<name>.<region>.data.azurecr.io), allowlisted exactly instead of *.blob.core.windows.net or *.data.azurecr.io."
  validation {
    condition     = alltrue([for h in var.acr_data_endpoints : !startswith(h, "*")])
    error_message = "acr_data_endpoints must be exact host names: *.data.azurecr.io admits any tenant's registry data endpoint (review I10)."
  }
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
   **exactly** `var.acr_login_server` + `var.acr_data_endpoints` on 443 (the ACR rule: our
   registry's login server and data endpoints, never `*.azurecr.io`, which admits every other
   tenant's registry, review I10), Entra (`login.microsoftonline.com`, `graph.microsoft.com`) on 443;
3. application rules — **`var.agent_allowed_fqdns` on 443, from `var.source_cidrs`**;
4. application rules — **`var.ci_agent_allowed_fqdns` on 443, from `var.ci_agent_source_cidrs`
   only** (review C2: the CI pool's agents in `snet-agents`); and, in the same collection,
   **`var.ci_agent_allowed_http_fqdns` on `Http:80`, from `var.ci_agent_source_cidrs` only**
   (re-review M4: Ubuntu's archive for apt);
5. nothing else: the policy's default is deny.

**What this allowlist is, and is not (review I10, Q1).** Under CNI Overlay a sandbox pod's traffic
leaves as its node's address, so `source_cidrs` (the VNet) cannot tell a sandbox from a node. A
sandbox therefore gets every rule meant for nodes: rule 2's FQDN tag (which admits
`management.azure.com`, training knowledge of AKS's required egress, UNVERIFIED), Entra and the
registry. And rule 3's `github.com` and `*.githubusercontent.com` accept arbitrary uploads. The
firewall limits **where** agents go; it does not stop a determined agent sending data out. Real
per-sandbox FQDN control needs Cilium FQDN policy keyed on pod identity: AKS Advanced Container
Networking's `security_enabled` (Q1's third option). That it enforces per-pod FQDN rules on the Kata
pool, and that it meets the classification's egress control, is **UNVERIFIED** and not built here.

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
  storage_use_azuread            = true
  resource_provider_registrations = "none" # as task 02: the plan identity cannot register providers
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
  description = "Extra CIDRs allowed to the public API server (the owner's admin workstations; Q3's admin half, kept apart from allowed_ingress_cidrs, which is the edge's). The environment's own egress /32 (the NAT gateway's in nat mode, the firewall's in firewall mode) is added automatically: the in-VNet agent pool egresses through it (task 03's egress_subnets include agents), so that is the address the API server sees."
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
output "resource_group_name" { value = data.azurerm_resource_group.this.name }
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
    # I4 is retired, and its id is not reused. It required a public cluster in firewall mode to
    # name its own authorised ranges, because "there is no NAT address to add". Since review C2 the
    # AKS module's list is concat(var.api_authorized_ip_ranges, [<nat or firewall egress IP>/32])
    # in BOTH modes (below), so a public cluster's list is never empty and I4 only refused valid
    # firewall-mode plans (plan re-review b04 round 1, finding 3).
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

- `data "azurerm_resource_group" "this"` — `rg-panacea-app-${env}`, which `bootstrap` creates
  (task 02). It is not a resource here so that the infra identity's Contributor can be scoped to
  the group instead of the subscription (the plan's "Deploy identities" table, review I8). Every
  `azurerm_resource_group.this` reference in this stack reads `data.azurerm_resource_group.this`.
- `module "network"`, `"observability"`, `"registry"`, `"keyvault"`, `"aks"` with the variables
  above; `module "egress_firewall"` with `count = var.egress_mode == "firewall" ? 1 : 0`,
  `routed_subnet_ids = module.network.egress_subnet_ids` (one source for both modes: the four
  subnets nat mode NATs are the four firewall mode routes),
  `ci_agent_source_cidrs = [module.network.subnet_cidrs.agents]`,
  `acr_login_server = module.registry.login_server` (exactly; review I10) and
  `acr_data_endpoints = module.registry.data_endpoint_host_names` (task 05 `:72-75`).
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
- `grants.tf`: kubelet → **AcrPull** on the registry, and the AKS control-plane identity →
  **Network Contributor** on `azurerm_public_ip.edge` (so the Traefik Service can bind it by name —
  whether the cloud provider also needs read on the resource group is **UNVERIFIED**, task 31
  checks). **Nothing else.** This stack makes **no** grant to a pipeline identity (plan re-review
  I-2 of fix batch b02).

  The cluster and push identities' roles are **not** here. The infra identity applies this stack,
  so every role this stack assigns must be on the ABAC allowlist of its role-assignment right
  (task 10). An ABAC condition limits the role and the principal, not the scope of the assignment
  (**training knowledge, UNVERIFIED**). So if `Azure Kubernetes Service RBAC Admin` were on the
  allowlist, the infra identity could assign it at cluster scope, or on
  `<cluster id>/namespaces/panacea-sandboxes`, to an identity it controls, and read every user's
  token. The same holds for AcrPush and the two custom roles. The **owner** grants those roles
  instead, with `infra/rbac/grant-pipeline-roles.sh` (task 30), once `foundation` has created the
  cluster and the registry (task 31 §4a). The script makes exactly these grants, and is the only
  place that does:
  - the push identity → **AcrPush** on the registry;
  - the cluster identity → **Azure Kubernetes Service Cluster User Role** on the cluster. With
    local accounts off (P8-D2), that role yields a kubeconfig that holds no secret;
  - the cluster identity → **Azure Kubernetes Service RBAC Admin** at namespace scope
    (`<cluster id>/namespaces/<ns>`), one assignment each for `default`, `panacea-edge`,
    `panacea-broker` and `panacea-hub`. These are the release namespaces of task 30's table,
    where Helm keeps its release Secrets;
  - the cluster identity → the custom role **`panacea-app-cluster-deployer`** at cluster scope,
    which covers the cluster-scoped kinds only;
  - the cluster identity → the custom role **`panacea-app-sandboxes-deployer`** at
    `<cluster id>/namespaces/panacea-sandboxes`, which covers the kinds the `platform` chart
    renders there, and the `escalate` and `bind` verbs the `broker` release needs there (task 30).

  The grant is **not RBAC Cluster Admin**, and it is **not RBAC Admin on `panacea-sandboxes`**.
  Every user's machine and sandbox tokens are Secrets in that namespace: task 17's
  `PANACEA_APP_BROKER_NAMESPACE` is "where every pod, Secret and claim lives" (P8-D5). Either grant
  would hand the deploy pipeline every user's session **directly** (review I8). The review's fix
  named "the four product namespaces". That list includes `panacea-sandboxes`, so the fix is
  narrowed here, on the facts.

  The narrowing removes a direct grant. It does **not** stop the cluster identity reaching the
  tokens, and the plan does not claim it does (plan re-review I-1). The cluster identity deploys
  the broker, and whoever deploys the broker can act as the broker. RBAC Admin on
  `panacea-broker` lets it run a pod as the broker's service account, which can `get` every token
  Secret in `panacea-sandboxes` (task 29). The `escalate` and `bind` verbs let it write itself a
  Role there. The bound is the ADO checks on the cluster connection (the plan's "Deploy identities"
  table, residual 3).

  Task 30 writes the two custom roles and checks them against the rendered charts. The owner
  creates them, because no pipeline identity can write role definitions.

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
# ^ provisional: a machine holds about three default sessions on D8s_v5 (P8-D6's ceiling, review
#   I5). Task 32's P07 decides the SKU (D16s_v5 / D32s_v5) or a smaller default session (set in
#   the broker chart's sandbox.sessionCpuMillis, task 29); this line, the vCPU-quota prerequisite
#   and the SKU-derived session max (tasks 11, 17, 29) change with it.
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
| `ci_agents_get_their_allowlist_from_their_subnet_only` | the CI rule's `destination_fqdns == var.ci_agent_allowed_fqdns`, HTTPS 443 only, `source_addresses == var.ci_agent_source_cidrs`; the CI HTTP rule's `destination_fqdns == var.ci_agent_allowed_http_fqdns`, `Http:80` only, the same source; the coding-agent rule does not contain `management.azure.com` | the CI list granted to the whole VNet, so a sandbox can reach Azure DevOps, the `*.queue.core.windows.net` wildcard, GitHub and the Terraform and Helm registries. (Re-review M5: it does **not** by itself keep Azure Resource Manager from a sandbox. Rule 2's `AzureKubernetesService` FQDN tag already admits `management.azure.com` from the nodes, and a sandbox pod leaves as its node's address — training knowledge of AKS's required egress, UNVERIFIED. What keeps ARM from a sandbox is that it holds no Azure credential: task 14's pod mounts no token, and IMDS is denied by task 11's `egress-internet`.) |
| `ci_http_takes_exact_hosts_only` | `ci_agent_allowed_http_fqdns = ["*.ubuntu.com"]` → `expect_failures = [var.ci_agent_allowed_http_fqdns]` | a wildcard allowed out in the clear (re-review M4) |
| `acr_is_allowed_by_data_endpoint_not_blob_wildcard` | the ACR rule's FQDNs contain the data endpoints and no `*.blob.core.windows.net` | the wildcard that lets a sandbox exfiltrate to any storage account |
| `acr_rule_names_our_registry_exactly` | with `acr_login_server = "acrpanaceaappdevk7q2.azurecr.io"` (task 05 `:79`'s name): the ACR rule's `destination_fqdns` contains that name, and `alltrue([for f in <the ACR rule's destination_fqdns> : !startswith(f, "*")])` | `*.azurecr.io` (every tenant's registry, and with it `<name>.<region>.data.azurecr.io`) put back in the rule beside the exact name (review I10) |
| `wildcard_acr_login_server_is_rejected` | its own `run`: `acr_login_server = "*.azurecr.io"` → `expect_failures = [var.acr_login_server]` | the validation removed, so a caller can pass the wildcard in (review I10) |
| `agents_get_exactly_the_allowlist` | the agent rule's `destination_fqdns == var.agent_allowed_fqdns` and its protocol is HTTPS 443 only | an allow-all application rule |
| `dns_goes_only_to_azure_dns` | the DNS network rule's destination is `["168.63.129.16"]` | DNS to `*`, a classic tunnel |

`stacks/foundation/tests/foundation.tftest.hcl` (run by `validate.sh` once with `dev.tfvars` and once
with `prod.tfvars`; its file-level `variables` block holds **only** `cluster_admin_group_object_ids`,
a placeholder GUID no tfvars sets (guard G10, review I1: a file-level key overrides `-var-file`).
The stack takes no pipeline identity's object id, because it grants none, plan re-review I-2).
Every run below that depends on an environment-specific value pins it in its own `variables`
block, so it means the same under either pass.

Its `mock_provider "azurerm"` is not empty: it holds

```hcl
mock_provider "azurerm" {
  mock_resource "azurerm_container_registry" {
    defaults = {
      login_server             = "acrpanaceaappdevk7q2.azurecr.io"
      data_endpoint_host_names = ["acrpanaceaappdevk7q2.australiaeast.data.azurecr.io"]
    }
  }
}
```

The reason is the module's `acr_login_server` validation (`endswith(…, ".azurecr.io")`, above).
Foundation feeds it `module.registry.login_server`, a computed attribute. At plan the value is
unknown, so the validation waits; at apply Terraform enforces it against whatever value the
provider produced (Terraform 1.15.8 probe, plan re-review b04 round 1, finding 4). A bare mock
gives a computed string a random 8-character value (Terraform's documented mock behaviour;
training knowledge, not probed with azurerm, UNVERIFIED), which fails `endswith`. Without the
default, every firewall-mode run that applies would stop at the validation:
`firewall_mode_allowlists_its_own_address_on_the_api` (`command = apply`),
`agents_egress_in_firewall_mode` and `firewall_mode_uses_udr_and_has_no_nat` (no `command`, so
apply). The name is task 05 `:79`'s `acrpanaceaapp${env}${suffix}` for dev; under the prod pass
it is still the dev name, which is fine, because no run asserts the registry's environment. If the
validation still fires under the default, the mock is not reaching the module's registry: record
it, and add an `override_resource { target = module.registry.azurerm_container_registry.this }`
with the same values to each firewall-mode run instead.

| Run | Asserts | Mutant it kills |
|---|---|---|
| `committed_tfvars_plan` | `command = plan` succeeds with the `-var-file`'s values; the run sets no key a tfvars sets, so the prod pass plans `prod.tfvars` (`api_private = true`) untouched | a committed tfvars that does not plan. A file-level key that makes the prod pass a dev pass is guard G10's to kill; this run cannot see which pass it is in |
| `overlapping_pod_range_fails_the_plan` | pins `vnet_cidr = "10.40.0.0/16"` and the whole `aks_network` object with `pod_cidr = "10.40.128.0/17"` (prod's `vnet_cidr` differs, so without the pin the prod pass would not overlap) → `expect_failures = [terraform_data.invariants]` | I2 written as a `check` (warning only) |
| `dns_ip_outside_service_range_fails` | pins the whole `aks_network` object with `service_cidr = "172.16.0.0/16"`, `dns_service_ip = "172.17.0.10"` → `expect_failures` | I3 dropped |
| `non_australian_region_fails` | `location = "eastus"` → `expect_failures` | I1 dropped |
| `prod_must_be_private` | `environment = "prod"`, `api_private = false` → `expect_failures` | I5 dropped |
| `nat_mode_allowlists_its_own_address_on_the_api` | the run pins `environment = "dev"`, `api_private = false`, `egress_mode = "nat"` (under `prod.tfvars`, `api_private = true` leaves no authorised ranges, and `api_private = false` alone trips I5; review I1), `command = apply` with `override_resource` giving the NAT IP `20.0.0.1` → the AKS module's authorised ranges include `20.0.0.1/32` | the in-VNet pool locked out of its own cluster |
| `firewall_mode_allowlists_its_own_address_on_the_api` | the run pins `environment = "dev"`, `api_private = false`, `egress_mode = "firewall"` (so it means the same under either tfvars file: a private cluster has no authorised ranges, review I1/I5), `command = apply` with `override_resource` giving the firewall's public IP `20.0.0.2` → the authorised ranges include `20.0.0.2/32`. It sets no `api_authorized_ip_ranges`, and neither tfvars does, so the list holds only the firewall's /32 | the same lock-out in firewall mode, where the agents egress through the firewall, not a NAT; and the retired I4 put back, which refuses exactly this plan (a public firewall-mode cluster with no extra ranges; plan re-review b04 round 1, finding 3) |
| `agents_egress_in_nat_mode` | a `run` block pinning `egress_mode = "nat"`: `module.network.egress_subnet_ids` has key `agents` (task 03's `nat_mode_puts_nat_on_every_egress_subnet` pins one NAT association per key) | nat mode stranding the CI pool (C2) |
| `agents_egress_in_firewall_mode` | a second `run` block pinning `egress_mode = "firewall"` (and `environment = "dev"`, `api_private = false`, as the authorised-ranges run): `module.egress_firewall[0].routed_subnet_keys == ["agents", "apps", "sandbox", "system"]` | the two modes drifting apart, so firewall mode strands the CI pool (C2). Re-review M5: one row was two `run` blocks in one; each `egress_mode` needs its own |
| `firewall_mode_uses_udr_and_has_no_nat` | pins `egress_mode = "firewall"`, `environment = "dev"`, `api_private = false`, as its two firewall-mode siblings do (plan re-review b04 round 1, finding 3: unpinned, the row meant a public cluster under `dev.tfvars` and a private one under `prod.tfvars`) → `module.egress_firewall` has one instance and the cluster's `outbound_type == "userDefinedRouting"` | a UDR cluster without the firewall, which boots with no egress |
| `edge_records_alias_the_edge_ip` | one A record per hostname, each `target_resource_id == azurerm_public_ip.edge.id`, `ttl == 300` | a literal IP record that goes stale on the first re-create |
| `kubelet_pulls_from_the_registry` | an `AcrPull` assignment to the kubelet identity, scoped to the registry, and `grants.tf`'s only other assignment is `Network Contributor` on `azurerm_public_ip.edge` | the shared admin credential's replacement missing; a second grant slipped into `grants.tf` |

A pipeline identity's grant creeping back into this stack is killed by task 10's
`role-allowlist.sh`, not by this file: `AcrPush`, both AKS roles and both custom roles are off the
allowlist, so their literal anywhere under `infra/terraform` fails it. The two runs that checked the
cluster and push grants here (`cluster_identity_is_not_cluster_admin`, the push half of the AcrPull
run) move with the grants to task 30's `grant-pipeline-roles_test.sh`.

#### Steps

- [ ] **Step 1:** `modules/egress-firewall`: variables, tests, resources, outputs — its nine runs pass.
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
bash infra/scripts/guards.sh       # guards: 10 of 10 passed
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
- That firewall mode is an exfiltration control. It limits where agents go; under Overlay SNAT it
  cannot tell a sandbox from a node, and allowlisted hosts accept uploads (review I10, Q1).
  Per-sandbox FQDN control needs Cilium FQDN policy on pod identity (Advanced Container
  Networking), **UNVERIFIED** and not built here.
