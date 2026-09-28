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
    # As built: cidrnetmask, not cidrhost, which takes IPv6 too ("fd00::/16" passed); the
    # cidrsubnet(…, 0, 0) round trip refuses host bits ("10.40.1.0/16"), which Azure would
    # refuse only at apply.
    condition     = can(cidrnetmask(var.vnet_cidr)) && endswith(var.vnet_cidr, "/16") && try(cidrsubnet(var.vnet_cidr, 0, 0) == var.vnet_cidr, false)
    error_message = "vnet_cidr must be a valid IPv4 /16 written as its network address, e.g. 10.40.0.0/16."
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
  # As built: each entry becomes one rule's source, so a service tag or "*" would open the
  # edge to everyone; and a rule's name is built from its CIDR, so a repeat is refused too.
  validation {
    condition     = alltrue([for c in var.allowed_ingress_cidrs : can(cidrnetmask(c))])
    error_message = "Every allowed_ingress_cidrs entry must be an IPv4 CIDR such as 203.0.113.0/24; not a service tag, not *."
  }
  validation {
    condition     = length(distinct(var.allowed_ingress_cidrs)) == length(var.allowed_ingress_cidrs)
    error_message = "allowed_ingress_cidrs lists a CIDR twice; list each once."
  }
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
  # Review m8: AKS with outbound_type = userAssignedNATGateway needs each node subnet's NAT
  # association (and the gateway's public IP) at create time. The ids above come from
  # azurerm_subnet, so without this a consumer could create the cluster in parallel with the
  # associations. Empty for_each/count in firewall mode, where task 07's depends_on on the
  # firewall module does the same job for the route tables. depends_on names whole resources
  # (a local is not a valid depends_on reference), so the list is written out in both outputs.
  depends_on = [
    azurerm_subnet_nat_gateway_association.this,
    azurerm_nat_gateway_public_ip_association.this,
    azurerm_subnet_network_security_group_association.this,
  ]
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
  depends_on = [ # review m8, as subnet_ids
    azurerm_subnet_nat_gateway_association.this,
    azurerm_nat_gateway_public_ip_association.this,
    azurerm_subnet_network_security_group_association.this,
  ]
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

- `azurerm_virtual_network.this` — `vnet-panacea-app-${env}`, `address_space = [var.vnet_cidr]`,
  and (as built) `lifecycle { prevent_destroy = true }` (below, "The state path").
- Six `azurerm_subnet`s from `local.cidr`, each with **`default_outbound_access_enabled = false`**
  (egress is explicit: NAT or firewall). `private_endpoints` sets
  `private_endpoint_network_policies = "Enabled"` so its NSG applies. `agents` carries
  `delegation { name = "devops-pools", service_delegation { name = "Microsoft.DevOpsInfrastructure/pools" } }`
  (the delegation name Managed DevOps Pools require — **from training knowledge, UNVERIFIED**;
  task 30 is where it is used). `firewall` is `count = var.egress_mode == "firewall" ? 1 : 0` and
  is named exactly `AzureFirewallSubnet`. `private_endpoints` also carries
  `lifecycle { prevent_destroy = true }` (below, "The state path").
- Four `azurerm_network_security_group`s — `nodes-system`, `nodes-apps`, `nodes-sandbox`,
  `private-endpoints` — each associated with its subnet by one
  `azurerm_subnet_network_security_group_association.this` (`for_each`, keyed by subnet). Rules
  added on top of Azure's defaults:
  - `nodes-sandbox`: **deny inbound from `Internet`**, priority 100. Nothing finer: under Overlay a
    pod's packets carry its overlay address, not its node's subnet address, so a subnet-based rule
    ("only the apps subnet may reach 8080") would match node traffic and miss pod traffic. Pod-level
    isolation is task 11's NetworkPolicy (P8-D12), which sees pod identities.
  - `nodes-apps`: allow TCP 443 from each of `var.allowed_ingress_cidrs`, priority 200 onward,
    `for_each` so one rule per CIDR; no rule when the list is empty.
  - `private-endpoints`: allow TCP 443 and 5432 from the VNet, deny everything else inbound, 4000.

  As built: the NSGs are `azurerm_network_security_group.this` (`for_each`, keys `system`,
  `apps`, `sandbox`, `private_endpoints`), named `nsg-panacea-app-${env}-<role>`, with the rules
  **inline** (`dynamic "security_rule"`), so the module owns each NSG's whole rule set and a
  rule added by hand is removed on the next apply. Every rule is inbound with source port `*`.
  The sandbox deny is protocol, destination and port `*`. An edge rule is
  `allow-edge-443-<cidr with . and / as ->`, TCP, destination `*`, port 443, priority `200 + i`.
  Its destination is `*`, not the apps subnet, because AKS load-balancer rules use floating IP,
  so an edge packet still carries the frontend's public address when it reaches the node
  (training knowledge, **UNVERIFIED**; task 32 checks the edge from outside). The endpoint allow
  is source `var.vnet_cidr`, destination the endpoint subnet, ports 443 and 5432, priority 100.
- `azurerm_public_ip.nat` (Standard, static, `count` on nat mode), `azurerm_nat_gateway.this`
  (`sku_name = "Standard"`, `idle_timeout_in_minutes`), its IP association
  (`azurerm_nat_gateway_public_ip_association.this`, same `count`), and one
  `azurerm_subnet_nat_gateway_association.this` per entry of `local.egress_subnets` (four: the
  three node subnets and `agents`; `for_each = var.egress_mode == "nat" ? local.egress_subnets : {}`,
  so each is addressed by its key and firewall mode has none). That a NAT gateway can
  be associated with a subnet delegated to `Microsoft.DevOpsInfrastructure/pools` is **from
  training knowledge, UNVERIFIED**; the first foundation apply (task 31 step 4) proves it.
- `azurerm_private_dns_zone.this` for each of `local.private_dns_zones`, and one
  `azurerm_private_dns_zone_virtual_network_link` per zone with `registration_enabled = false`.
  As built: the gateway is `ng-panacea-app-${env}`, its IP `pip-natgw-panacea-app-${env}` (no
  zones, so the egress is not zone-redundant), and each link `link-vnet-panacea-app-${env}`.
  The zones carry `lifecycle { prevent_destroy = true }` (below); the links do not.

**The state path (carried from task 02 re-review I1).** `stacks/bootstrap` makes the state
account's private endpoint on this module's `subnet_ids.private_endpoints` and
`private_dns_zone_ids.blob`, passed by hand (`infra/terraform/README.md`, Stack order: the one
declared exception). That endpoint is not in foundation's state, so a foundation plan does not
see it, and once prod's public access is off it is the only path to every stack's state.
Replacing the subnet, the VNet or a zone would cut that path; a replaced zone drops every
endpoint record in it. So `azurerm_subnet.private_endpoints`, `azurerm_private_dns_zone.this`
(all four zones: each holds its own endpoints' records) and, as built, `azurerm_virtual_network.this`
carry `lifecycle { prevent_destroy = true }`. The VNet needs its own: a VNet replace plans a
subnet replace only when it changes the subnet's own arguments (a new VNet name or group), not
on a location change (As built, departure 7). **The cost:** `prevent_destroy` is a literal, so it
holds in dev too, and a deliberate teardown must first edit it out: the same shape as
bootstrap's `CanNotDelete` lock. The links are not protected: they are this module's own, and a
destroy removes a link before its zone. Whether Azure itself refuses to delete a subnet that
still holds another stack's endpoint (`InUseSubnetCannotBeDeleted`, training knowledge) is
**UNVERIFIED**; even if it does, that does not cover the zones. A plan cannot show a
`lifecycle` argument, so the test pins it the way bootstrap pins its provider lines: it reads
`main.tf` with `file()`, strips comments, and finds `prevent_destroy = true` inside the
`lifecycle` block of each of the three resource blocks. As built (round 3), it also refuses any
override file in the module: Terraform merges an override file's `lifecycle` into the block it
names, so one could set all three false without touching `main.tf`. **This is not a duplicate of
guard G1** (round-3 review M3): G1 fails an override file **tracked by git**, and deliberately
allows an untracked one (the runbook §2 `backend_override.tf`, which lives in `stacks/bootstrap`,
not here). The assert's new ground is an **untracked** override file inside `modules/network` —
which G1 lets through by design, and which is exactly what mutant OV1 is.

**Why the NSG rules are this small.** AKS manages its own rules for LoadBalancer services, and an
over-tight subnet NSG is the classic way to break node bootstrap. The rules above only add denies
that no AKS function needs and one allow the edge needs. **How AKS's managed rules and these
interact under Overlay is UNVERIFIED**; task 32 checks reachability from outside and between pools.

#### Tests to write first (`tests/network.tftest.hcl`)

As built, every run is `command = plan`, and the mocks and overrides apply at plan
(`override_during = plan`). azurerm checks the form of a Resource Manager id it is handed
(`subnet_id`, `nat_gateway_id`, `virtual_network_id`, …) even against a mock, so a random mock id
fails the plan (task 02's lesson). `mock_resource` defaults give the VNet, NAT gateway, public IP
(`ip_address = "192.0.2.10"`) and zones well-formed ids, and file-level `override_resource`
blocks give each of the six subnets and each of the four NSGs (an instance target,
`azurerm_network_security_group.this["sandbox"]`) an id of its own, so a run can tell which
subnet an association joins, not only that one exists.

```hcl
mock_provider "azurerm" {
  override_during = plan
  # mock_resource defaults: azurerm_virtual_network, azurerm_nat_gateway, azurerm_public_ip,
  # azurerm_private_dns_zone; then one override_resource per subnet and per NSG instance
}
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

As built: 27 runs. A list attribute compares with `tolist([...])` and tags with `tomap({...})`:
a tuple or object literal never equals a list or map. Each id in the last column is defined by that
column itself, here in this file — one clause per mutant, with the two accepted survivors in
"As built" section 9 — so a reader needs nothing outside the repo to resolve one. The harnesses
that produced them are gone: they lived in session temp directories (`t03r2/`, `t03r3/`,
`t03r3k/mutants.py`), there is no `scratchpad/` in this repo, and one of the three went with its
killed session (round-3 review M1). The per-round pass/fail tables are in
`.superpowers/sdd/env/task-03-report.md`, which is a **local stream artefact, not in the repo**
(`.superpowers/sdd/.gitignore` is `*`), so a fresh clone has the definitions but not those
tables. Round 4 corrected M1 by pointing the ids at that report; it is the committed column that
has to carry them, and it does.

| Run | Asserts | Mutant it kills |
|---|---|---|
| `subnets_follow_the_address_plan` | the VNet's `address_space` is `[vnet_cidr]`; `system`, `apps`, `sandbox`, `private_endpoints`, `agents` are `10.40.0.0/24`, `10.40.1.0/24`, `10.40.4.0/22`, `10.40.8.0/26`, `10.40.8.64/26`; `output.subnet_cidrs` is the whole plan, `firewall` included; the subnet names; each subnet in this VNet and group | a changed `newbits`/`netnum` that silently overlaps two subnets (A1-A4) |
| `no_subnet_has_default_outbound_access` | every subnet's `default_outbound_access_enabled == false` (the firewall subnet's in the firewall run) | one subnet left on the implicit Azure egress (D1, D2, D4; D3 in the firewall run) |
| `vnet_must_be_a_slash_16` | `vnet_cidr = "10.40.0.0/20"` → `expect_failures = [var.vnet_cidr]` | the validation dropped, letting `cidrsubnet` fail obscurely (V1) |
| `vnet_must_be_ipv4` | `"fd00::/16"` → `expect_failures` | a `cidrhost`-based check, which takes IPv6 (V2) |
| `vnet_must_be_written_as_its_network_address` | `"10.40.1.0/16"` → `expect_failures` | host bits let through to an apply-time refusal (V3) |
| `nat_mode_puts_nat_on_every_egress_subnet` | `keys(azurerm_subnet_nat_gateway_association.this) == ["agents", "apps", "sandbox", "system"]`, each joining its own subnet's id and the one gateway; `output.egress_subnet_ids` is exactly those four ids; one Standard gateway, idle 10; a Standard, Static IP joined to it; `output.nat_public_ip` is its address | NAT on the PE subnet (N1), on only one pool (N2), **missing on `agents`, so the CI pool never registers an agent (C2)** (N3, N3b), every link on one subnet (N4), the endpoint subnet listed as egress (O1), a dynamic IP (P1), no address output (F5) |
| `nat_idle_timeout_reaches_the_gateway` / `…_of_4_is_accepted` | 120, then 4, each reaches the gateway | the timeout hard-coded (T1); a bound moved inward (TB) |
| `nat_idle_timeout_below_4_is_rejected` / `…_above_120_…` | 3, then 121 → `expect_failures` | a bound dropped (T2, T3) |
| `firewall_mode_has_no_nat_and_has_the_firewall_subnet` | `egress_mode = "firewall"` → no gateway, IP or association of any kind; `azurerm_subnet.firewall[0]` is `AzureFirewallSubnet` at `10.40.8.128/26`, no default outbound; `output.subnet_ids` adds it (six keys); `output.egress_subnet_ids` still the four; `nat_public_ip` null | NAT and firewall both present, asymmetric routing (F1); misnamed (F3); no id output (F4) |
| `nat_mode_has_no_firewall_subnet` | no firewall subnet; `output.subnet_ids` is exactly the five, each key its own subnet | the firewall subnet made in nat mode (F2) |
| `sandbox_nsg_denies_internet_inbound` | the `nodes-sandbox` NSG holds exactly one rule: inbound Deny from `Internet`, source port, protocol, destination and port `*`, priority 100; the `nodes-system` NSG holds none | the deny dropped (S1) or given priority 4096, below Azure's allow (S2); from `*` (S3); an Allow (S4); every rule outbound (E7); source port 443 (PS1) |
| `edge_allow_is_one_rule_per_cidr` | the `nodes-apps` NSG holds exactly two rules, both Allow, sources equal to the two CIDRs; each inbound TCP from source port `*` to 443, destination `*`; priorities 200 and 201, names distinct | a single rule with only the first CIDR (E1); any protocol (E3) or port (E4); priorities colliding (E5), from 100 (E6) or from 300 (E8); the destination narrowed to the apps subnet (PS2) |
| `no_ingress_cidrs_means_no_allow_rule` | `allowed_ingress_cidrs = []` → `nodes-apps` holds no rule | an allow-from-`*` fallback (E2) |
| `ingress_cidrs_must_be_ipv4_cidrs` | `["203.0.113.0/24", "Internet"]` → `expect_failures` | the CIDR check dropped, so a service tag opens the edge (C1) |
| `ingress_cidrs_are_listed_once` | a repeated CIDR → `expect_failures` | the repeat check dropped: two rules of one name, refused at apply (C2) |
| `private_endpoint_nsg_admits_only_the_vnet_on_443_and_5432` | exactly two rules: Allow TCP from `var.vnet_cidr`, source port `*`, to the endpoint subnet on 443 and 5432, below 4000; Deny all inbound at 4000; the subnet's `private_endpoint_network_policies == "Enabled"` | any source (PE1); port 22 too (PE2); the deny dropped (PE3) or at 4096 (PE4); endpoint policies off, so the NSG does not apply (PE5); allow to anywhere (PE6) |
| `each_nsg_is_on_its_own_subnet` | `keys(azurerm_subnet_network_security_group_association.this) == ["apps", "private_endpoints", "sandbox", "system"]`, each joining its own NSG and subnet; the NSG names | the sandbox and apps NSGs swapped (AS1); the endpoint NSG left off (AS2) |
| `four_privatelink_zones_all_linked_without_registration` | the zone map is exactly `acr`, `vault`, `postgres`, `blob` with their `privatelink.` names; four links, each `registration_enabled == false`, on this VNet and its own zone; `output.private_dns_zone_ids` has the four keys, **and each value is that zone's `id`** (round 4) | a missing Postgres zone, so the Hub's database would not resolve (Z1); VM registration (Z2); every link on one zone (Z3); a misnamed zone (Z4); a zone id missing from the output (O2); **the output giving each zone's `name` where its `id` belongs (ZID1, round-3 review I1)** |
| `agents_subnet_is_delegated_to_devops_pools` | `azurerm_subnet.agents.delegation[0]` is `devops-pools` → `Microsoft.DevOpsInfrastructure/pools`; no other subnet is delegated | the delegation removed (G1) or pointed elsewhere (G2); its name changed (DN) |
| `the_state_path_refuses_destroy` | reads `main.tf` with `file()`, strips `/* */`, `#` and `//` comments, and finds `prevent_destroy = true` inside the `lifecycle` block of `azurerm_subnet.private_endpoints`, of `azurerm_private_dns_zone.this` (carried from task 02 re-review I1) and of `azurerm_virtual_network.this` (round 2); and (round 3) that the module holds no `*override.tf` or `*override.tf.json` file, since Terraform merges an override file into its block argument by argument, `lifecycle` included, so one could undo all three unseen | either set false (PD1, PD2); the line commented out (PD3) or in a block comment (PD4); the protection moved from the zone to its link (PD5); the VNet's set false (PD6), removed (PD7), in a block comment (PD8) or commented out with `//` (PD9); a `main_override.tf` (OV1) or an `override.tf.json` (OV2) setting the VNet's `prevent_destroy = false` |
| `every_join_by_name_is_a_reference` | (round 3) reads every `.tf` file, comments stripped: six `azurerm_subnet` blocks, each `virtual_network_name = azurerm_virtual_network.this.name`; the one `private_dns_zone_name = azurerm_private_dns_zone.this[each.key].name`; `output "vnet_name"` is `azurerm_virtual_network.this.name`; no `.tf.json` file. A literal of equal value plans the same, so only the text shows the lost graph edge | a subnet naming its VNet by a literal: the endpoint subnet (ORD1), system (ORD1s), the firewall's (ORD1f), a seventh in another `.tf` file (MV1) or in a `.tf.json` (KS1); the link naming its zone by `each.value` (ORD2); `vnet_name` output as a literal (ORD3) |
| `names_region_group_and_tags_come_from_the_inputs` | `environment = "prod"`, `location = "australiasoutheast"`, another group and two tags: every name carries `prod`; every located resource takes the region; every resource the group; every taggable one the tags; the VNet outputs | the region (L1), group (L3) or env (L4; L5 in an NSG's name) hard-coded; a resource left untagged (L2, L6); `vnet_id` giving the name (O5); the link name (LN) |
| `firewall_subnet_takes_the_group_and_vnet_from_the_inputs` | `egress_mode = "firewall"`, `environment = "prod"`, another group: `azurerm_subnet.firewall[0]` is in that group and in `vnet-panacea-app-prod` (the run above is nat mode, where the firewall subnet does not exist) | the firewall subnet's group (FG1) or VNet (FV1) hard-coded to dev's (round 2's predicted survivors) |
| `environment_must_be_dev_or_prod` / `egress_mode_must_be_nat_or_firewall` | `"test"`, `"none"` → `expect_failures` | a validation dropped (EV1, EG1) |

**The outputs' `depends_on` (review m8) is not a `terraform test` assertion.** Under mocks a plan
has no ordering to observe, so no run above can see it. Task 07 proves it on the foundation stack
with `terraform graph`, where `module.aks` is a real consumer; a dropped `depends_on` shows there
as a missing path. Output-level `depends_on` rather than `depends_on = [module.network]` on the
AKS call: a module-level `depends_on` makes every data source inside `module.aks` wait for the
whole network module, private DNS zones included, and defers those reads to apply (from training
knowledge, UNVERIFIED). This way only the subnet ids carry the wait.

#### Steps

- [x] **Step 1:** Write `versions.tf`, `variables.tf` and the test file. `validate.sh
  infra/terraform/modules/network` fails at `test` (as built: not at `validate`, which passes a
  test file that names undeclared resources; task 02 found the same).
- [x] **Step 2:** Write `main.tf` resource by resource, running the tests after each, until all
  pass (as built: 27 runs, not ten).
- [x] **Step 3:** Write `outputs.tf`; `terraform fmt`; guards; commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/network
# validate: OK   infra/terraform/modules/network
# validate: 1 ok, 0 failed                            (exit 0)
bash infra/scripts/guards.sh        # guards: 10 of 10 passed
```

`validate.sh` sends each step's own output to its log and prints it only when the step fails, so
`terraform test`'s `Success! 27 passed, 0 failed.` is **not** on stdout on a green run (task 04
review 1, M3). Run `terraform test` in the module directory to see the count.

#### What this task does not claim

- That `10.40.0.0/16` is free on KPMG's network — it is a placeholder; `vnet_cidr` is an owner
  input.
- That the NSG rules are sufficient or harmless in a running cluster (see above).
- That the `Microsoft.DevOpsInfrastructure/pools` delegation name is right.
- That `prevent_destroy` blocks every path to losing the state endpoint: it refuses a planned
  destroy or replace of the three protected resources, and nothing else (a portal delete, or
  a destroy of the resource group, is outside Terraform's plan).

#### As built (2026-09-26; rounds 3 and 4, 2026-09-28)

**Commits**, the durable record — `git log` holds them, the report does not have to:
`7667862b` red; `9386dbc0` green; `804fbb88` round-1 killers; `cce77676` round-2 killer (FG1,
FV1); `824eb14f` the VNet `prevent_destroy` red and `19553d81` its fix; `ab984d7b` the endpoint
subnet's comment; `312be719` this part as built; `099459f4` round 3's two killers and
`63dc64e8` their text; `8bdac2cb` round 4's zone-output assert (review I1) and `cd897329` its
text (I1, M1, M2, M3).

**Departures from the part as first written. Each fixes a root cause:**
1. Step 1's red is at `test`, not at `validate`. `terraform validate` passes a test file that
   names undeclared resources (task 02 found the same).
2. `vnet_cidr`'s check is `cidrnetmask` plus a network-address round trip. The part's
   `cidrhost` check took `fd00::/16` and `10.40.1.0/16`.
3. `allowed_ingress_cidrs` gained two validations: each entry must be an IPv4 CIDR, and each is
   listed once. Without the first, a service tag or `*` opens the edge to everyone. Without the
   second, two rules share a name and Azure refuses them at apply.
4. The NSGs are one `for_each` resource with inline rules. The part said four NSGs but not
   how.
5. 27 runs, not ten.
6. Carried I1: `prevent_destroy` on the endpoint subnet and on all four zones, pinned by a
   `file()` run.
7. **`prevent_destroy` on the VNet too (found in round 2; a ruling, declared).** The carried
   remedy said the subnet "also stops a VNet replace". That holds only when the replace changes
   the subnet's own arguments: a new VNet name or group. Terraform replaces a dependent only
   when its own planned arguments force it (the reason `replace_triggered_by` exists; training
   knowledge). So a location change replaces the VNet, plans no subnet replace, and the
   subnet's `prevent_destroy` never fires. Whether Azure would then refuse to delete a VNet
   whose subnet holds another stack's endpoint is **UNVERIFIED**. The cost is the same as the
   subnet's: a literal, holding in dev too.
   - A mock plan cannot decide this either way: the mock provider models no forced replace. A
     probe (round 2, in a session temp directory that no longer exists; recorded in
     `.superpowers/sdd/env/task-03-report.md`) applied, then planned a group change. That change
     forces both protected resources, yet the plan came out clean.
   - The probe also showed that `terraform test`'s teardown destroys mock state past
     `prevent_destroy`, so the protection does not break the plan-only runs.
8. **Two text pins more (round 3; each kills a predicted survivor).** Both are the class a plan
   cannot see, so each reads the module's text, as the `prevent_destroy` run does.
   - An override file (OV1). `the_state_path_refuses_destroy` read only `main.tf`, and Terraform
     merges `main_override.tf` or `override.tf.json` into the block it names, `lifecycle`
     included (training knowledge; OV1 and OV2 confirm the effect: each left every run green
     until the assert). The run now refuses any such file.
   - A join by name written as a literal of equal value (ORD1, and ORD2, ORD3 of the same class).
     Every planned value is unchanged, so every run passed, but the graph edge was gone: a
     subnet could be created before its VNet, a zone destroyed before its link, and task 07's
     consumer of `vnet_name` could race the VNet. New run `every_join_by_name_is_a_reference`.
   - Neither pin proves an order, and the join pin is narrower than "has an edge": it pins the
     **direct** reference. Routing the same reference through a local keeps the edge and still
     fails (verified by the round-3 reviewer, `out/S2_LOCALREF.log`), as does a legitimate
     seventh subnet against the `== 6` count. Both are deliberate edits to the run, and its
     messages now say so rather than diagnosing a lost edge. Only
     `terraform graph` shows the order, and task 07 runs it on the foundation stack for the NAT
     associations' paths (`parts/task-07.md:640-647`), not for these edges.
9. **`private_dns_zone_ids`'s values are pinned, not only its keys (round 4; round-3 review
   I1).** The run asserted `keys(output.private_dns_zone_ids)` and nothing read the values, so
   `k => z.name` in place of `k => z.id` survived all 27 runs (reviewer's `out/S1_ZONEIDS.log`,
   re-run here as the red proof: `Failure! 26 passed, 1 failed`, rc 1, failing at that run
   alone). A zone *name* where an *id* is required breaks task 07's
   `private_dns_zone_group.private_dns_zone_ids` and the `.blob` that README's stack order
   passes to bootstrap by hand — at apply against Azure, which this phase never runs, so no
   other check in the phase would catch it. The fix is one assert, in the style of
   `subnet_ids`': `alltrue([for k, v in output.private_dns_zone_ids : v ==
   azurerm_private_dns_zone.this[k].id])`. Still 27 runs. The accepted survivors declared in
   `.superpowers/sdd/env/task-03-report.md` are unchanged and remain those two only: O4 (the
   outputs' review-m8 `depends_on`, which task 07 proves with `terraform graph`) and P2 (the NAT
   gateway given zones, a choice rather than a defect).
