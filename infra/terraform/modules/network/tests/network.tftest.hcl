# modules/network's tests (phase 8 task 03). Mocks only: nothing here reaches Azure.
#
# Every run is `command = plan`, with the mocks and overrides applied at plan
# (override_during = plan). azurerm checks the form of a Resource Manager id it is given
# (subnet_id, nat_gateway_id, virtual_network_id, …) even against a mock, so a random mock id
# would fail the plan (task 02's lesson). Each subnet and NSG gets its own well-formed id,
# so a run can tell which subnet an association or a NAT link points at, not only that one
# exists. The ids live in the all-zeros subscription and name nothing real.
mock_provider "azurerm" {
  override_during = plan
  mock_resource "azurerm_virtual_network" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/virtualNetworks/vnet-panacea-app-dev"
    }
  }
  mock_resource "azurerm_nat_gateway" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/natGateways/ng-panacea-app-dev"
    }
  }
  mock_resource "azurerm_public_ip" {
    defaults = {
      id         = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/publicIPAddresses/pip-natgw-panacea-app-dev"
      ip_address = "192.0.2.10"
    }
  }
  mock_resource "azurerm_private_dns_zone" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/privateDnsZones/mock.example"
    }
  }
}

override_resource {
  target          = azurerm_subnet.system
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/virtualNetworks/vnet-panacea-app-dev/subnets/snet-nodes-system" }
}
override_resource {
  target          = azurerm_subnet.apps
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/virtualNetworks/vnet-panacea-app-dev/subnets/snet-nodes-apps" }
}
override_resource {
  target          = azurerm_subnet.sandbox
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/virtualNetworks/vnet-panacea-app-dev/subnets/snet-nodes-sandbox" }
}
override_resource {
  target          = azurerm_subnet.private_endpoints
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/virtualNetworks/vnet-panacea-app-dev/subnets/snet-private-endpoints" }
}
override_resource {
  target          = azurerm_subnet.agents
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/virtualNetworks/vnet-panacea-app-dev/subnets/snet-agents" }
}
override_resource {
  target          = azurerm_subnet.firewall
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/virtualNetworks/vnet-panacea-app-dev/subnets/AzureFirewallSubnet" }
}
override_resource {
  target          = azurerm_network_security_group.this["system"]
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/networkSecurityGroups/nsg-panacea-app-dev-nodes-system" }
}
override_resource {
  target          = azurerm_network_security_group.this["apps"]
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/networkSecurityGroups/nsg-panacea-app-dev-nodes-apps" }
}
override_resource {
  target          = azurerm_network_security_group.this["sandbox"]
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/networkSecurityGroups/nsg-panacea-app-dev-nodes-sandbox" }
}
override_resource {
  target          = azurerm_network_security_group.this["private_endpoints"]
  override_during = plan
  values          = { id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Network/networkSecurityGroups/nsg-panacea-app-dev-private-endpoints" }
}

# A module has no <env>.tfvars, so guard G10 does not apply: the file-level block is the
# defaults every run starts from, and a run that proves a variable reaches a resource sets a
# value that differs from these (task 02's lesson: a value both sides agree on proves nothing).
variables {
  environment           = "dev"
  location              = "australiaeast"
  resource_group_name   = "rg-panacea-app-dev"
  vnet_cidr             = "10.40.0.0/16"
  egress_mode           = "nat"
  allowed_ingress_cidrs = ["203.0.113.0/24", "198.51.100.0/24"]
  tags                  = {}
}

run "subnets_follow_the_address_plan" {
  command = plan
  assert {
    condition     = azurerm_virtual_network.this.address_space == toset(["10.40.0.0/16"])
    error_message = "The VNet's address space is not exactly var.vnet_cidr."
  }
  assert {
    condition     = azurerm_subnet.system.address_prefixes == tolist(["10.40.0.0/24"]) && azurerm_subnet.apps.address_prefixes == tolist(["10.40.1.0/24"])
    error_message = "The node subnets for the system and apps pools are not 10.40.0.0/24 and 10.40.1.0/24."
  }
  assert {
    condition     = azurerm_subnet.sandbox.address_prefixes == tolist(["10.40.4.0/22"])
    error_message = "The sandbox node subnet is not 10.40.4.0/22."
  }
  assert {
    condition     = azurerm_subnet.private_endpoints.address_prefixes == tolist(["10.40.8.0/26"]) && azurerm_subnet.agents.address_prefixes == tolist(["10.40.8.64/26"])
    error_message = "The private endpoint and agents subnets are not 10.40.8.0/26 and 10.40.8.64/26."
  }
  assert {
    condition     = output.subnet_cidrs == { system = "10.40.0.0/24", apps = "10.40.1.0/24", sandbox = "10.40.4.0/22", private_endpoints = "10.40.8.0/26", agents = "10.40.8.64/26", firewall = "10.40.8.128/26" }
    error_message = "output.subnet_cidrs does not match the address plan."
  }
  assert {
    condition     = [azurerm_subnet.system.name, azurerm_subnet.apps.name, azurerm_subnet.sandbox.name, azurerm_subnet.private_endpoints.name, azurerm_subnet.agents.name] == ["snet-nodes-system", "snet-nodes-apps", "snet-nodes-sandbox", "snet-private-endpoints", "snet-agents"]
    error_message = "The subnets are not named as the address plan names them."
  }
  assert {
    condition     = alltrue([for s in [azurerm_subnet.system, azurerm_subnet.apps, azurerm_subnet.sandbox, azurerm_subnet.private_endpoints, azurerm_subnet.agents] : s.virtual_network_name == azurerm_virtual_network.this.name && s.resource_group_name == "rg-panacea-app-dev"])
    error_message = "A subnet is not in this module's VNet and resource group."
  }
}

run "no_subnet_has_default_outbound_access" {
  command = plan
  assert {
    condition     = alltrue([for s in [azurerm_subnet.system, azurerm_subnet.apps, azurerm_subnet.sandbox, azurerm_subnet.private_endpoints, azurerm_subnet.agents] : s.default_outbound_access_enabled == false])
    error_message = "A subnet is left on Azure's implicit default outbound access; egress must be the NAT gateway or the firewall."
  }
}

run "vnet_must_be_a_slash_16" {
  command = plan
  variables {
    vnet_cidr = "10.40.0.0/20"
  }
  expect_failures = [var.vnet_cidr]
}

# cidrhost accepts IPv6, so a condition built on it alone lets an IPv6 /16 through.
run "vnet_must_be_ipv4" {
  command = plan
  variables {
    vnet_cidr = "fd00::/16"
  }
  expect_failures = [var.vnet_cidr]
}

run "vnet_must_be_written_as_its_network_address" {
  command = plan
  variables {
    vnet_cidr = "10.40.1.0/16"
  }
  expect_failures = [var.vnet_cidr]
}

run "nat_mode_puts_nat_on_every_egress_subnet" {
  command = plan
  assert {
    condition     = keys(azurerm_subnet_nat_gateway_association.this) == ["agents", "apps", "sandbox", "system"]
    error_message = "NAT is not on exactly the four egress subnets (system, apps, sandbox, agents); agents without it cannot register a CI agent (C2)."
  }
  assert {
    condition     = alltrue([for k, a in azurerm_subnet_nat_gateway_association.this : a.subnet_id == { agents = azurerm_subnet.agents.id, apps = azurerm_subnet.apps.id, sandbox = azurerm_subnet.sandbox.id, system = azurerm_subnet.system.id }[k] && a.nat_gateway_id == azurerm_nat_gateway.this[0].id])
    error_message = "A NAT association joins the wrong subnet, or another gateway."
  }
  assert {
    condition     = output.egress_subnet_ids == { agents = azurerm_subnet.agents.id, apps = azurerm_subnet.apps.id, sandbox = azurerm_subnet.sandbox.id, system = azurerm_subnet.system.id }
    error_message = "output.egress_subnet_ids is not exactly the four egress subnets' ids."
  }
  assert {
    condition     = length(azurerm_nat_gateway.this) == 1 && azurerm_nat_gateway.this[0].sku_name == "Standard" && azurerm_nat_gateway.this[0].idle_timeout_in_minutes == 10
    error_message = "nat mode does not make one Standard NAT gateway with the 10-minute default idle timeout."
  }
  assert {
    condition     = azurerm_public_ip.nat[0].sku == "Standard" && azurerm_public_ip.nat[0].allocation_method == "Static"
    error_message = "The NAT gateway's public IP is not Standard and Static; a dynamic address would move the egress providers allowlist."
  }
  assert {
    condition     = azurerm_nat_gateway_public_ip_association.this[0].nat_gateway_id == azurerm_nat_gateway.this[0].id && azurerm_nat_gateway_public_ip_association.this[0].public_ip_address_id == azurerm_public_ip.nat[0].id
    error_message = "The NAT gateway is not joined to its public IP."
  }
  assert {
    condition     = output.nat_public_ip == "192.0.2.10"
    error_message = "output.nat_public_ip is not the NAT public IP's address in nat mode."
  }
}

run "nat_idle_timeout_reaches_the_gateway" {
  command = plan
  variables {
    nat_idle_timeout_minutes = 30
  }
  assert {
    condition     = azurerm_nat_gateway.this[0].idle_timeout_in_minutes == 30
    error_message = "nat_idle_timeout_minutes does not reach the NAT gateway."
  }
}

run "nat_idle_timeout_below_4_is_rejected" {
  command = plan
  variables {
    nat_idle_timeout_minutes = 3
  }
  expect_failures = [var.nat_idle_timeout_minutes]
}

run "nat_idle_timeout_above_120_is_rejected" {
  command = plan
  variables {
    nat_idle_timeout_minutes = 121
  }
  expect_failures = [var.nat_idle_timeout_minutes]
}

run "firewall_mode_has_no_nat_and_has_the_firewall_subnet" {
  command = plan
  variables {
    egress_mode = "firewall"
  }
  assert {
    condition     = length(azurerm_nat_gateway.this) == 0 && length(azurerm_public_ip.nat) == 0 && length(azurerm_nat_gateway_public_ip_association.this) == 0 && length(azurerm_subnet_nat_gateway_association.this) == 0
    error_message = "firewall mode still makes NAT: NAT and the firewall's routes together route asymmetrically."
  }
  assert {
    condition     = azurerm_subnet.firewall[0].name == "AzureFirewallSubnet" && azurerm_subnet.firewall[0].address_prefixes == tolist(["10.40.8.128/26"]) && azurerm_subnet.firewall[0].default_outbound_access_enabled == false
    error_message = "The firewall subnet is not AzureFirewallSubnet at 10.40.8.128/26 with no default outbound access."
  }
  assert {
    condition     = output.subnet_ids.firewall == azurerm_subnet.firewall[0].id && length(output.subnet_ids) == 6
    error_message = "output.subnet_ids does not add the firewall subnet in firewall mode."
  }
  assert {
    condition     = keys(output.egress_subnet_ids) == ["agents", "apps", "sandbox", "system"] && output.nat_public_ip == null
    error_message = "firewall mode changes the egress subnets task 07 routes, or reports a NAT address."
  }
}

run "nat_mode_has_no_firewall_subnet" {
  command = plan
  assert {
    condition     = length(azurerm_subnet.firewall) == 0 && keys(output.subnet_ids) == ["agents", "apps", "private_endpoints", "sandbox", "system"]
    error_message = "nat mode makes the AzureFirewallSubnet or lists a firewall subnet id."
  }
  assert {
    condition     = output.subnet_ids == { agents = azurerm_subnet.agents.id, apps = azurerm_subnet.apps.id, private_endpoints = azurerm_subnet.private_endpoints.id, sandbox = azurerm_subnet.sandbox.id, system = azurerm_subnet.system.id }
    error_message = "output.subnet_ids maps a key to the wrong subnet."
  }
}

run "sandbox_nsg_denies_internet_inbound" {
  command = plan
  assert {
    condition     = length([for r in azurerm_network_security_group.this["sandbox"].security_rule : r if r.access == "Deny" && r.direction == "Inbound" && r.source_address_prefix == "Internet" && r.priority == 100 && r.protocol == "*" && r.destination_port_range == "*" && r.destination_address_prefix == "*"]) == 1
    error_message = "The sandbox NSG has no deny of all inbound Internet traffic at priority 100, ahead of every allow."
  }
  assert {
    condition     = length(azurerm_network_security_group.this["sandbox"].security_rule) == 1 && length(azurerm_network_security_group.this["system"].security_rule) == 0
    error_message = "The sandbox NSG holds more than its one deny, or the system NSG holds a rule of its own."
  }
}

run "edge_allow_is_one_rule_per_cidr" {
  command = plan
  assert {
    condition     = length(azurerm_network_security_group.this["apps"].security_rule) == 2 && sort([for r in azurerm_network_security_group.this["apps"].security_rule : r.source_address_prefix if r.access == "Allow"]) == sort(var.allowed_ingress_cidrs)
    error_message = "The apps NSG does not hold exactly one allow per allowed_ingress_cidrs entry, and nothing else."
  }
  assert {
    condition     = alltrue([for r in azurerm_network_security_group.this["apps"].security_rule : r.direction == "Inbound" && r.protocol == "Tcp" && r.destination_port_range == "443" && r.priority >= 200 && r.priority < 4000])
    error_message = "An edge rule is not inbound TCP 443 at a priority from 200 up."
  }
  assert {
    condition     = length(distinct([for r in azurerm_network_security_group.this["apps"].security_rule : r.priority])) == 2 && length(distinct([for r in azurerm_network_security_group.this["apps"].security_rule : r.name])) == 2
    error_message = "Two edge rules share a priority or a name, which Azure refuses."
  }
}

run "no_ingress_cidrs_means_no_allow_rule" {
  command = plan
  variables {
    allowed_ingress_cidrs = []
  }
  assert {
    condition     = length(azurerm_network_security_group.this["apps"].security_rule) == 0
    error_message = "With no allowed_ingress_cidrs the apps NSG still holds a rule (an allow-from-anywhere fallback?)."
  }
}

run "ingress_cidrs_must_be_ipv4_cidrs" {
  command = plan
  variables {
    allowed_ingress_cidrs = ["203.0.113.0/24", "Internet"]
  }
  expect_failures = [var.allowed_ingress_cidrs]
}

run "ingress_cidrs_are_listed_once" {
  command = plan
  variables {
    allowed_ingress_cidrs = ["203.0.113.0/24", "198.51.100.0/24", "203.0.113.0/24"]
  }
  expect_failures = [var.allowed_ingress_cidrs]
}

run "private_endpoint_nsg_admits_only_the_vnet_on_443_and_5432" {
  command = plan
  assert {
    condition     = length([for r in azurerm_network_security_group.this["private_endpoints"].security_rule : r if r.access == "Allow" && r.direction == "Inbound" && r.protocol == "Tcp" && r.source_address_prefix == "10.40.0.0/16" && r.destination_address_prefix == "10.40.8.0/26" && toset(r.destination_port_ranges) == toset(["443", "5432"]) && r.priority < 4000]) == 1
    error_message = "The private endpoint NSG does not allow exactly TCP 443 and 5432 from the VNet to the endpoint subnet, ahead of its deny."
  }
  assert {
    condition     = length([for r in azurerm_network_security_group.this["private_endpoints"].security_rule : r if r.access == "Deny" && r.direction == "Inbound" && r.priority == 4000 && r.protocol == "*" && r.source_address_prefix == "*" && r.destination_address_prefix == "*" && r.destination_port_range == "*"]) == 1
    error_message = "The private endpoint NSG does not deny all other inbound traffic at priority 4000."
  }
  assert {
    condition     = length(azurerm_network_security_group.this["private_endpoints"].security_rule) == 2 && azurerm_subnet.private_endpoints.private_endpoint_network_policies == "Enabled"
    error_message = "The private endpoint NSG holds another rule, or the subnet's endpoint network policies are off, so the NSG would not apply to endpoints."
  }
}

run "each_nsg_is_on_its_own_subnet" {
  command = plan
  assert {
    condition     = keys(azurerm_subnet_network_security_group_association.this) == ["apps", "private_endpoints", "sandbox", "system"]
    error_message = "The NSG associations are not exactly system, apps, sandbox and private_endpoints."
  }
  assert {
    condition     = alltrue([for k, a in azurerm_subnet_network_security_group_association.this : a.network_security_group_id == azurerm_network_security_group.this[k].id && a.subnet_id == { apps = azurerm_subnet.apps.id, private_endpoints = azurerm_subnet.private_endpoints.id, sandbox = azurerm_subnet.sandbox.id, system = azurerm_subnet.system.id }[k]])
    error_message = "An NSG is associated with another pool's subnet: one blast radius per NSG is the point."
  }
  assert {
    condition     = [for k in ["system", "apps", "sandbox", "private_endpoints"] : azurerm_network_security_group.this[k].name] == ["nsg-panacea-app-dev-nodes-system", "nsg-panacea-app-dev-nodes-apps", "nsg-panacea-app-dev-nodes-sandbox", "nsg-panacea-app-dev-private-endpoints"]
    error_message = "The NSGs are not named nsg-panacea-app-<env>-<subnet role>."
  }
}

run "four_privatelink_zones_all_linked_without_registration" {
  command = plan
  assert {
    condition     = { for k, z in azurerm_private_dns_zone.this : k => z.name } == { acr = "privatelink.azurecr.io", vault = "privatelink.vaultcore.azure.net", postgres = "privatelink.postgres.database.azure.com", blob = "privatelink.blob.core.windows.net" }
    error_message = "The four privatelink zones (acr, vault, postgres, blob) are not exactly these names; a missing Postgres zone leaves the Hub's database unresolvable."
  }
  assert {
    condition     = keys(azurerm_private_dns_zone_virtual_network_link.this) == ["acr", "blob", "postgres", "vault"] && alltrue([for k, l in azurerm_private_dns_zone_virtual_network_link.this : l.registration_enabled == false && l.virtual_network_id == azurerm_virtual_network.this.id && l.private_dns_zone_name == azurerm_private_dns_zone.this[k].name])
    error_message = "A zone is not linked to this VNet, is linked to another zone's name, or registers VM records."
  }
  assert {
    condition     = keys(output.private_dns_zone_ids) == ["acr", "blob", "postgres", "vault"]
    error_message = "output.private_dns_zone_ids does not carry the four zone keys."
  }
}

run "agents_subnet_is_delegated_to_devops_pools" {
  command = plan
  assert {
    condition     = length(azurerm_subnet.agents.delegation) == 1 && azurerm_subnet.agents.delegation[0].service_delegation[0].name == "Microsoft.DevOpsInfrastructure/pools"
    error_message = "The agents subnet is not delegated to Microsoft.DevOpsInfrastructure/pools, so task 30's pool cannot use it."
  }
  assert {
    condition     = alltrue([for s in [azurerm_subnet.system, azurerm_subnet.apps, azurerm_subnet.sandbox, azurerm_subnet.private_endpoints] : length(s.delegation) == 0])
    error_message = "A node or endpoint subnet is delegated; only agents is."
  }
}

# The state account's private endpoint (bootstrap, task 02) sits on this module's
# private_endpoints subnet and blob zone, but is not in this module's state, so a foundation
# plan cannot see it. Replacing the subnet or a zone would cut every stack off from its state
# (carried from task 02 re-review I1). A plan cannot show a lifecycle argument, so this run
# reads main.tf, with comments stripped, and finds prevent_destroy = true inside the lifecycle
# block of each of the two resource blocks. fmt closes a top-level block with a "}" at column 0.
run "the_state_path_refuses_destroy" {
  command = plan
  assert {
    condition     = can(regex("lifecycle\\s*\\{[^}]*prevent_destroy\\s*=\\s*true\\b", try(regex("(?s)(?:^|\\n)resource\\s+\"azurerm_subnet\"\\s+\"private_endpoints\"\\s*\\{(.*?)\\n\\}", replace(replace(replace(file("${path.module}/main.tf"), "\r", ""), "/(?s)/\\*.*?\\*\\//", ""), "/(?m)(#|\\/\\/).*$/", ""))[0], "")))
    error_message = "azurerm_subnet.private_endpoints has no lifecycle { prevent_destroy = true }: a replace would cut the state account's private endpoint off."
  }
  assert {
    condition     = can(regex("lifecycle\\s*\\{[^}]*prevent_destroy\\s*=\\s*true\\b", try(regex("(?s)(?:^|\\n)resource\\s+\"azurerm_private_dns_zone\"\\s+\"this\"\\s*\\{(.*?)\\n\\}", replace(replace(replace(file("${path.module}/main.tf"), "\r", ""), "/(?s)/\\*.*?\\*\\//", ""), "/(?m)(#|\\/\\/).*$/", ""))[0], "")))
    error_message = "azurerm_private_dns_zone.this has no lifecycle { prevent_destroy = true }: a replaced zone drops every endpoint record in it, the state account's included."
  }
}

# Every value here differs from the file-level defaults, so a name, region, group or tag set
# hard-coded to the defaults fails.
run "names_region_group_and_tags_come_from_the_inputs" {
  command = plan
  variables {
    environment         = "prod"
    location            = "australiasoutheast"
    resource_group_name = "rg-panacea-app-prod"
    tags                = { owner = "platform", stack = "foundation" }
  }
  assert {
    condition     = azurerm_virtual_network.this.name == "vnet-panacea-app-prod" && output.vnet_name == "vnet-panacea-app-prod" && output.vnet_id == azurerm_virtual_network.this.id && output.vnet_cidr == "10.40.0.0/16"
    error_message = "The VNet is not vnet-panacea-app-<env>, or its outputs do not report it."
  }
  assert {
    condition     = azurerm_nat_gateway.this[0].name == "ng-panacea-app-prod" && azurerm_public_ip.nat[0].name == "pip-natgw-panacea-app-prod" && alltrue([for k, n in azurerm_network_security_group.this : startswith(n.name, "nsg-panacea-app-prod-")])
    error_message = "The NAT gateway, its IP or an NSG does not carry the environment in its name."
  }
  assert {
    condition     = alltrue([for r in concat([azurerm_virtual_network.this, azurerm_nat_gateway.this[0], azurerm_public_ip.nat[0]], values(azurerm_network_security_group.this)) : r.location == "australiasoutheast" && r.resource_group_name == "rg-panacea-app-prod" && r.tags == tomap({ owner = "platform", stack = "foundation" })])
    error_message = "The VNet, NAT gateway, its IP or an NSG does not take var.location, var.resource_group_name and var.tags."
  }
  assert {
    condition     = alltrue([for r in concat(values(azurerm_private_dns_zone.this), values(azurerm_private_dns_zone_virtual_network_link.this)) : r.resource_group_name == "rg-panacea-app-prod" && r.tags == tomap({ owner = "platform", stack = "foundation" })])
    error_message = "A privatelink zone or link does not take var.resource_group_name and var.tags."
  }
  assert {
    condition     = alltrue([for s in [azurerm_subnet.system, azurerm_subnet.apps, azurerm_subnet.sandbox, azurerm_subnet.private_endpoints, azurerm_subnet.agents] : s.resource_group_name == "rg-panacea-app-prod" && s.virtual_network_name == "vnet-panacea-app-prod"])
    error_message = "A subnet is not in var.resource_group_name's VNet."
  }
}

run "environment_must_be_dev_or_prod" {
  command = plan
  variables {
    environment = "test"
  }
  expect_failures = [var.environment]
}

run "egress_mode_must_be_nat_or_firewall" {
  command = plan
  variables {
    egress_mode = "none"
  }
  expect_failures = [var.egress_mode]
}
