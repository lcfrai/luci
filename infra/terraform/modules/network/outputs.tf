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
