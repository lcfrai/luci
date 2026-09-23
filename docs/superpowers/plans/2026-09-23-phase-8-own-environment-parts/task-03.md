### Task 03: `modules/network` — the VNet, its subnets, their NSGs, the NAT gateway, the privatelink zones

Panacea's VNet is a `/16` with two `/23` subnets on classic Azure CNI, where pod IPs come out of
the node subnets and cap pods per node (survey §1.2, `:66-79`); its own comment names that ceiling
as the reason for Overlay. We start on **Azure CNI Overlay** (P8-D2, task 06), so pods take
addresses from a private overlay range and **subnets are sized for nodes only**. One subnet per
node pool keeps each blast radius in its own NSG (survey §2.1, `:292`).

**Files:**
- `infra/terraform/modules/network/{versions,main,variables,outputs}.tf` (new)
- `infra/terraform/modules/network/tests/network.tftest.hcl` (new)

#### The address plan (fixed inside the module; the caller supplies only a `/16`)

| Subnet | `cidrsubnet(vnet, newbits, netnum)` | For `10.40.0.0/16` | Holds |
|---|---|---|---|
| `snet-nodes-system` | `(8, 0)` | `10.40.0.0/24` | system pool nodes |
| `snet-nodes-apps` | `(8, 1)` | `10.40.1.0/24` | apps pool nodes (Hub, broker, Traefik) |
| `snet-nodes-sandbox` | `(6, 1)` | `10.40.4.0/22` | Kata nodes — up to ~1,000 |
| `snet-private-endpoints` | `(10, 32)` | `10.40.8.0/26` | ACR, Key Vault, Postgres, state (prod) endpoints |
| `snet-agents` | `(10, 33)` | `10.40.8.64/26` | Managed DevOps Pool agents (task 30), delegated |
| `AzureFirewallSubnet` | `(10, 34)` | `10.40.8.128/26` | only when `egress_mode = "firewall"` |

Pod and service ranges are **not** VNet ranges under Overlay; they are task 06's variables, and
task 07's check proves none of them overlaps `vnet_cidr`.

#### Contract

```hcl
# versions.tf
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = "4.81.0" }
  }
}

# variables.tf
variable "environment" {
  type        = string
  description = "dev or prod; used in names."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "location" {
  type        = string
  description = "Azure region for every resource in this module."
}
variable "resource_group_name" {
  type        = string
  description = "The foundation resource group."
}
variable "vnet_cidr" {
  type        = string
  description = "The VNet range. Must be a /16 that does not overlap the corporate network (an owner input)."
  validation {
    condition     = can(cidrhost(var.vnet_cidr, 0)) && endswith(var.vnet_cidr, "/16")
    error_message = "vnet_cidr must be a valid IPv4 /16."
  }
}
variable "egress_mode" {
  type        = string
  description = "nat: a NAT gateway on every egress subnet — the three node subnets and agents (dev). firewall: no NAT here; the AzureFirewallSubnet is created and modules/egress-firewall routes the same four subnets through it (task 07)."
  validation {
    condition     = contains(["nat", "firewall"], var.egress_mode)
    error_message = "egress_mode must be nat or firewall."
  }
}
variable "nat_idle_timeout_minutes" {
  type        = number
  description = "NAT gateway TCP idle timeout. Long-lived agent connections (model streaming) need more than the 4-minute default."
  default     = 10
  validation {
    condition     = var.nat_idle_timeout_minutes >= 4 && var.nat_idle_timeout_minutes <= 120
    error_message = "nat_idle_timeout_minutes must be between 4 and 120."
  }
}
variable "allowed_ingress_cidrs" {
  type        = list(string)
  description = "Source CIDRs allowed to reach the edge on 443 (owner question Q3). Empty means no inbound rule is added, so nothing outside the VNet reaches the apps pool."
}
variable "tags" {
  type        = map(string)
  description = "Tags applied to every resource."
}

# outputs.tf
output "vnet_id" { value = azurerm_virtual_network.this.id }
output "vnet_name" { value = azurerm_virtual_network.this.name }
output "vnet_cidr" { value = var.vnet_cidr }
output "subnet_ids" {
  description = "Keys: system, apps, sandbox, private_endpoints, agents, and firewall when egress_mode = firewall."
  value = merge(
    {
      system            = azurerm_subnet.system.id
      apps              = azurerm_subnet.apps.id
      sandbox           = azurerm_subnet.sandbox.id
      private_endpoints = azurerm_subnet.private_endpoints.id
      agents            = azurerm_subnet.agents.id
    },
    var.egress_mode == "firewall" ? { firewall = azurerm_subnet.firewall[0].id } : {}
  )
}
output "subnet_cidrs" {
  value = {
    system            = local.cidr.system
    apps              = local.cidr.apps
    sandbox           = local.cidr.sandbox
    private_endpoints = local.cidr.private_endpoints
    agents            = local.cidr.agents
    firewall          = local.cidr.firewall
  }
}
output "egress_subnet_ids" {
  description = "Every subnet that must reach the internet, and only those: system, apps, sandbox, agents. NAT-associated in nat mode; task 07 routes exactly these through the firewall in firewall mode. Never private_endpoints or firewall."
  value       = local.egress_subnets
}
output "nat_public_ip" {
  description = "The sandboxes', nodes' and CI agents' egress address in nat mode (null in firewall mode). Owner input: providers may need to allowlist it."
  value       = var.egress_mode == "nat" ? azurerm_public_ip.nat[0].ip_address : null
}
output "private_dns_zone_ids" {
  description = "Keys: acr, vault, postgres, blob."
  value       = { for k, z in azurerm_private_dns_zone.this : k => z.id }
}
```

```hcl
# main.tf — the locals every resource derives from
locals {
  cidr = {
    system            = cidrsubnet(var.vnet_cidr, 8, 0)
    apps              = cidrsubnet(var.vnet_cidr, 8, 1)
    sandbox           = cidrsubnet(var.vnet_cidr, 6, 1)
    private_endpoints = cidrsubnet(var.vnet_cidr, 10, 32)
    agents            = cidrsubnet(var.vnet_cidr, 10, 33)
    firewall          = cidrsubnet(var.vnet_cidr, 10, 34)
  }
  private_dns_zones = {
    acr      = "privatelink.azurecr.io"
    vault    = "privatelink.vaultcore.azure.net"
    postgres = "privatelink.postgres.database.azure.com"
    blob     = "privatelink.blob.core.windows.net"
  }
  node_subnets = { system = azurerm_subnet.system.id, apps = azurerm_subnet.apps.id, sandbox = azurerm_subnet.sandbox.id }
  # Review C2: every subnet has default_outbound_access_enabled = false, so a subnet missing here
  # has no internet at all. The Managed DevOps Pool's agents (task 30) must reach dev.azure.com,
  # the Terraform registry and the Helm repos, and in prod they are the only path to the private
  # API server, so agents is an egress subnet like the three node subnets.
  egress_subnets = merge(local.node_subnets, { agents = azurerm_subnet.agents.id })
}
```

#### Resources (described; names verified against azurerm 4.81.0)

- `azurerm_virtual_network.this` — `vnet-panacea-app-${env}`, `address_space = [var.vnet_cidr]`.
- Six `azurerm_subnet`s from `local.cidr`, each with **`default_outbound_access_enabled = false`**
  (egress is explicit: NAT or firewall). `private_endpoints` sets
  `private_endpoint_network_policies = "Enabled"` so its NSG applies. `agents` carries
  `delegation { name = "devops-pools", service_delegation { name = "Microsoft.DevOpsInfrastructure/pools" } }`
  (the delegation name Managed DevOps Pools require — **from training knowledge, UNVERIFIED**;
  task 30 is where it is used). `firewall` is `count = var.egress_mode == "firewall" ? 1 : 0` and
  is named exactly `AzureFirewallSubnet`.
- Four `azurerm_network_security_group`s — `nodes-system`, `nodes-apps`, `nodes-sandbox`,
  `private-endpoints` — each associated with its subnet. Rules added on top of Azure's defaults:
  - `nodes-sandbox`: **deny inbound from `Internet`**, priority 100. Nothing finer: under Overlay a
    pod's packets carry its overlay address, not its node's subnet address, so a subnet-based rule
    ("only the apps subnet may reach 8080") would match node traffic and miss pod traffic. Pod-level
    isolation is task 11's NetworkPolicy (P8-D12), which sees pod identities.
  - `nodes-apps`: allow TCP 443 from each of `var.allowed_ingress_cidrs`, priority 200 onward,
    `for_each` so one rule per CIDR; no rule when the list is empty.
  - `private-endpoints`: allow TCP 443 and 5432 from the VNet, deny everything else inbound, 4000.
- `azurerm_public_ip.nat` (Standard, static, `count` on nat mode), `azurerm_nat_gateway.this`
  (`sku_name = "Standard"`, `idle_timeout_in_minutes`), its IP association, and one
  `azurerm_subnet_nat_gateway_association.this` per entry of `local.egress_subnets` (four: the
  three node subnets and `agents`; `for_each = var.egress_mode == "nat" ? local.egress_subnets : {}`,
  so each is addressed by its key and firewall mode has none). That a NAT gateway can
  be associated with a subnet delegated to `Microsoft.DevOpsInfrastructure/pools` is **from
  training knowledge, UNVERIFIED**; the first foundation apply (task 31 step 4) proves it.
- `azurerm_private_dns_zone.this` for each of `local.private_dns_zones`, and one
  `azurerm_private_dns_zone_virtual_network_link` per zone with `registration_enabled = false`.

**Why the NSG rules are this small.** AKS manages its own rules for LoadBalancer services, and an
over-tight subnet NSG is the classic way to break node bootstrap. The rules above only add denies
that no AKS function needs and one allow the edge needs. **How AKS's managed rules and these
interact under Overlay is UNVERIFIED**; task 32 checks reachability from outside and between pools.

#### Tests to write first (`tests/network.tftest.hcl`)

```hcl
mock_provider "azurerm" {}
variables {
  environment           = "dev"
  location              = "australiaeast"
  resource_group_name   = "rg-panacea-app-dev"
  vnet_cidr             = "10.40.0.0/16"
  egress_mode           = "nat"
  allowed_ingress_cidrs = ["203.0.113.0/24", "198.51.100.0/24"]
  tags                  = {}
}
```

| Run | Asserts | Mutant it kills |
|---|---|---|
| `subnets_follow_the_address_plan` | `azurerm_subnet.system.address_prefixes == ["10.40.0.0/24"]`, `sandbox == ["10.40.4.0/22"]`, `private_endpoints == ["10.40.8.0/26"]`, `agents == ["10.40.8.64/26"]` | a changed `newbits`/`netnum` that silently overlaps two subnets |
| `no_subnet_has_default_outbound_access` | every subnet's `default_outbound_access_enabled == false` | one subnet left on the implicit Azure egress |
| `vnet_must_be_a_slash_16` | `vnet_cidr = "10.40.0.0/20"` → `expect_failures = [var.vnet_cidr]` | the validation dropped, letting `cidrsubnet` fail obscurely |
| `nat_mode_puts_nat_on_every_egress_subnet` | four associations: `keys(azurerm_subnet_nat_gateway_association.this) == ["agents", "apps", "sandbox", "system"]`, and `output.egress_subnet_ids` has the same four keys; none on private endpoints | NAT on the PE subnet, on only one pool, or **missing on `agents`, so the CI pool never registers an agent (C2)** |
| `firewall_mode_has_no_nat_and_has_the_firewall_subnet` | `egress_mode = "firewall"` → `length(azurerm_nat_gateway.this) == 0`, `azurerm_subnet.firewall[0].name == "AzureFirewallSubnet"` | NAT and firewall both present (asymmetric routing) |
| `sandbox_nsg_denies_internet_inbound` | the `nodes-sandbox` NSG has a rule with `access == "Deny"`, `direction == "Inbound"`, `source_address_prefix == "Internet"` and priority 100 | the deny rule dropped or given priority 4096, below Azure's allow |
| `edge_allow_is_one_rule_per_cidr` | the `nodes-apps` NSG has exactly two 443 allow rules, sources equal to the two CIDRs | a single rule with only the first CIDR |
| `no_ingress_cidrs_means_no_allow_rule` | `allowed_ingress_cidrs = []` → no 443 allow rule on `nodes-apps` | an allow-from-`*` fallback |
| `four_privatelink_zones_all_linked_without_registration` | four zones with the names in `local.private_dns_zones`; four links, each `registration_enabled == false` | a missing Postgres zone (the Hub's database would not resolve) |
| `agents_subnet_is_delegated_to_devops_pools` | `azurerm_subnet.agents.delegation[0].service_delegation[0].name == "Microsoft.DevOpsInfrastructure/pools"` | the delegation removed |

#### Steps

- [ ] **Step 1:** Write `versions.tf`, `variables.tf` and the test file. `validate.sh
  infra/terraform/modules/network` fails at `validate`.
- [ ] **Step 2:** Write `main.tf` resource by resource, running the tests after each, until all
  ten pass.
- [ ] **Step 3:** Write `outputs.tf`; `terraform fmt`; guards; commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/network
# validate: OK   infra/terraform/modules/network
# validate: 1 ok, 0 failed          (terraform test: Success! 10 passed, 0 failed.)
bash infra/scripts/guards.sh        # guards: 9 of 9 passed
```

#### What this task does not claim

- That `10.40.0.0/16` is free on KPMG's network — it is a placeholder; `vnet_cidr` is an owner
  input.
- That the NSG rules are sufficient or harmless in a running cluster (see above).
- That the `Microsoft.DevOpsInfrastructure/pools` delegation name is right.
