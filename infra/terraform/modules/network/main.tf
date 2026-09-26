# The address plan is fixed here; the caller supplies only a /16 (part 03, "The address plan").
# Under Azure CNI Overlay (P8-D2, task 06) pods take overlay addresses, so these subnets are
# sized for nodes only, one per pool, each with its own NSG.
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

# prevent_destroy (task 03 round 2, extending task 02 re-review I1): the endpoint subnet's own
# prevent_destroy fires only when a plan replaces the subnet. Terraform replaces a dependent
# only when its own arguments force it, so a VNet replace that leaves them alone (a location
# change: a subnet takes no location) would plan no subnet replace and still take the subnet,
# and the state account's endpoint in it, with the VNet. The same cost as the subnet's.
resource "azurerm_virtual_network" "this" {
  name                = "vnet-panacea-app-${var.environment}"
  location            = var.location
  resource_group_name = var.resource_group_name
  address_space       = [var.vnet_cidr]
  tags                = var.tags

  lifecycle {
    prevent_destroy = true
  }
}

# Every subnet: default_outbound_access_enabled = false. Egress is explicit, the NAT gateway
# (nat mode) or the firewall's routes (firewall mode, task 07), never Azure's implicit address.
resource "azurerm_subnet" "system" {
  name                            = "snet-nodes-system"
  resource_group_name             = var.resource_group_name
  virtual_network_name            = azurerm_virtual_network.this.name
  address_prefixes                = [local.cidr.system]
  default_outbound_access_enabled = false
}

resource "azurerm_subnet" "apps" {
  name                            = "snet-nodes-apps"
  resource_group_name             = var.resource_group_name
  virtual_network_name            = azurerm_virtual_network.this.name
  address_prefixes                = [local.cidr.apps]
  default_outbound_access_enabled = false
}

resource "azurerm_subnet" "sandbox" {
  name                            = "snet-nodes-sandbox"
  resource_group_name             = var.resource_group_name
  virtual_network_name            = azurerm_virtual_network.this.name
  address_prefixes                = [local.cidr.sandbox]
  default_outbound_access_enabled = false
}

# prevent_destroy (task 02 re-review I1, carried): the state account's private endpoint, made by
# stacks/bootstrap and so absent from this module's state, sits on this subnet. Once prod's
# public access is off it is the only path to every stack's state, and a replace of this subnet
# would cut that path. A VNet replace plans one only when it changes this subnet's own arguments
# (a new VNet name or group), hence the VNet's own prevent_destroy above. The cost: it is a
# literal, so it holds in dev too, and a deliberate teardown first edits it out, the same shape
# as bootstrap's CanNotDelete lock.
resource "azurerm_subnet" "private_endpoints" {
  name                              = "snet-private-endpoints"
  resource_group_name               = var.resource_group_name
  virtual_network_name              = azurerm_virtual_network.this.name
  address_prefixes                  = [local.cidr.private_endpoints]
  default_outbound_access_enabled   = false
  private_endpoint_network_policies = "Enabled" # so the private-endpoints NSG applies to endpoints

  lifecycle {
    prevent_destroy = true
  }
}

# The Managed DevOps Pool's agents (task 30). The delegation's service name is from training
# knowledge, UNVERIFIED (task 30 is where it is used). That a NAT gateway can be associated with
# a subnet delegated to it is UNVERIFIED as well; task 31 step 4's first apply proves both.
resource "azurerm_subnet" "agents" {
  name                            = "snet-agents"
  resource_group_name             = var.resource_group_name
  virtual_network_name            = azurerm_virtual_network.this.name
  address_prefixes                = [local.cidr.agents]
  default_outbound_access_enabled = false

  delegation {
    name = "devops-pools"
    service_delegation {
      name = "Microsoft.DevOpsInfrastructure/pools"
    }
  }
}

# Firewall mode only (task 07's modules/egress-firewall). Azure requires exactly this name and at
# least a /26. It carries no NSG: Azure does not allow one on AzureFirewallSubnet.
resource "azurerm_subnet" "firewall" {
  count                           = var.egress_mode == "firewall" ? 1 : 0
  name                            = "AzureFirewallSubnet"
  resource_group_name             = var.resource_group_name
  virtual_network_name            = azurerm_virtual_network.this.name
  address_prefixes                = [local.cidr.firewall]
  default_outbound_access_enabled = false
}

# --- NSGs: one per node subnet and one for the private endpoints ------------------------------
# The rules are small on purpose. An over-tight subnet NSG is the classic way to break AKS node
# bootstrap, so these only add denies no AKS function needs and the one allow the edge needs, on
# top of Azure's default rules. How AKS's own rules and these interact under Overlay is
# UNVERIFIED; task 32 checks reachability from outside and between pools. The rules are inline,
# so this module owns each NSG's whole rule set and a rule added by hand is removed next apply.
locals {
  nsg_names = {
    system            = "nodes-system"
    apps              = "nodes-apps"
    sandbox           = "nodes-sandbox"
    private_endpoints = "private-endpoints"
  }
  nsg_subnet_ids = merge(local.node_subnets, { private_endpoints = azurerm_subnet.private_endpoints.id })
  # Every rule has the same attributes; null means "not set" (port_range and port_ranges conflict).
  nsg_rules = {
    system = []
    # One allow per edge CIDR, priority 200 onward (owner question Q3). Destination "*", not the
    # apps subnet: AKS load-balancer rules use floating IP, so an edge packet still carries the
    # frontend's public address when it reaches the node (training knowledge, UNVERIFIED; task
    # 32 checks the edge from outside). No CIDR, no rule: Azure's DenyAllInBound then holds.
    apps = [for i, c in var.allowed_ingress_cidrs : {
      name                       = "allow-edge-443-${replace(c, "/[./]/", "-")}"
      priority                   = 200 + i
      access                     = "Allow"
      protocol                   = "Tcp"
      source_address_prefix      = c
      destination_address_prefix = "*"
      destination_port_range     = "443"
      destination_port_ranges    = null
    }]
    # Deny all Internet inbound, ahead of anything. Nothing finer: under Overlay a pod's packets
    # carry its overlay address, not its node's, so a subnet-based rule would match node traffic
    # and miss pod traffic. Pod-level isolation is task 11's NetworkPolicy (P8-D12).
    sandbox = [{
      name                       = "deny-internet-inbound"
      priority                   = 100
      access                     = "Deny"
      protocol                   = "*"
      source_address_prefix      = "Internet"
      destination_address_prefix = "*"
      destination_port_range     = "*"
      destination_port_ranges    = null
    }]
    # HTTPS (ACR, Key Vault, blob) and Postgres from the VNet; nothing else reaches an endpoint.
    private_endpoints = [
      {
        name                       = "allow-vnet-inbound-443-5432"
        priority                   = 100
        access                     = "Allow"
        protocol                   = "Tcp"
        source_address_prefix      = var.vnet_cidr
        destination_address_prefix = local.cidr.private_endpoints
        destination_port_range     = null
        destination_port_ranges    = ["443", "5432"]
      },
      {
        name                       = "deny-all-inbound"
        priority                   = 4000
        access                     = "Deny"
        protocol                   = "*"
        source_address_prefix      = "*"
        destination_address_prefix = "*"
        destination_port_range     = "*"
        destination_port_ranges    = null
      },
    ]
  }
}

resource "azurerm_network_security_group" "this" {
  for_each            = local.nsg_names
  name                = "nsg-panacea-app-${var.environment}-${each.value}"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags

  dynamic "security_rule" {
    for_each = local.nsg_rules[each.key]
    content {
      name                       = security_rule.value.name
      priority                   = security_rule.value.priority
      direction                  = "Inbound"
      access                     = security_rule.value.access
      protocol                   = security_rule.value.protocol
      source_port_range          = "*"
      source_address_prefix      = security_rule.value.source_address_prefix
      destination_address_prefix = security_rule.value.destination_address_prefix
      destination_port_range     = security_rule.value.destination_port_range
      destination_port_ranges    = security_rule.value.destination_port_ranges
    }
  }
}

resource "azurerm_subnet_network_security_group_association" "this" {
  for_each                  = local.nsg_names
  subnet_id                 = local.nsg_subnet_ids[each.key]
  network_security_group_id = azurerm_network_security_group.this[each.key].id
}

# --- NAT gateway: nat mode only -----------------------------------------------------------------
# One gateway, one static IP, on the four egress subnets. Firewall mode makes none of it: NAT on
# a subnet the firewall also routes would send replies out a different way (asymmetric routing).
# No zones: a Standard NAT gateway lives in one zone or none (training knowledge, UNVERIFIED), so
# this is not zone-redundant egress.
resource "azurerm_public_ip" "nat" {
  count               = var.egress_mode == "nat" ? 1 : 0
  name                = "pip-natgw-panacea-app-${var.environment}"
  location            = var.location
  resource_group_name = var.resource_group_name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = var.tags
}

resource "azurerm_nat_gateway" "this" {
  count                   = var.egress_mode == "nat" ? 1 : 0
  name                    = "ng-panacea-app-${var.environment}"
  location                = var.location
  resource_group_name     = var.resource_group_name
  sku_name                = "Standard"
  idle_timeout_in_minutes = var.nat_idle_timeout_minutes
  tags                    = var.tags
}

resource "azurerm_nat_gateway_public_ip_association" "this" {
  count                = var.egress_mode == "nat" ? 1 : 0
  nat_gateway_id       = azurerm_nat_gateway.this[0].id
  public_ip_address_id = azurerm_public_ip.nat[0].id
}

resource "azurerm_subnet_nat_gateway_association" "this" {
  for_each       = var.egress_mode == "nat" ? local.egress_subnets : {}
  subnet_id      = each.value
  nat_gateway_id = azurerm_nat_gateway.this[0].id
}

# --- privatelink DNS zones ----------------------------------------------------------------------
# One zone per private endpoint kind, each linked to this VNet with no auto-registration (VM
# records do not belong in a privatelink zone).
# prevent_destroy (task 02 re-review I1, carried): each zone holds the A records of every
# private endpoint in it, and blob holds the state account's, made by stacks/bootstrap outside
# this module's state. A replaced zone drops them all. The same cost as the endpoint subnet's:
# it holds in dev too, and a deliberate teardown first edits it out. The links are not
# protected: they are this module's own, and a destroy removes a link before its zone.
resource "azurerm_private_dns_zone" "this" {
  for_each            = local.private_dns_zones
  name                = each.value
  resource_group_name = var.resource_group_name
  tags                = var.tags

  lifecycle {
    prevent_destroy = true
  }
}

resource "azurerm_private_dns_zone_virtual_network_link" "this" {
  for_each              = local.private_dns_zones
  name                  = "link-vnet-panacea-app-${var.environment}"
  resource_group_name   = var.resource_group_name
  private_dns_zone_name = azurerm_private_dns_zone.this[each.key].name
  virtual_network_id    = azurerm_virtual_network.this.id
  registration_enabled  = false
  tags                  = var.tags
}
