resource "azurerm_virtual_network" "private" {
  name                = "${var.name}-private"
  location            = var.location
  resource_group_name = var.resource_group_name
  address_space       = [var.runtime_subnet_cidr, var.controller_subnet_cidr]
  tags                = var.tags
}

resource "azurerm_subnet" "runtime" {
  name                 = "${var.name}-runtime"
  resource_group_name  = var.resource_group_name
  virtual_network_name = azurerm_virtual_network.private.name
  address_prefixes     = [var.runtime_subnet_cidr]
}

resource "azurerm_subnet" "controller" {
  name                              = "${var.name}-controller"
  resource_group_name               = var.resource_group_name
  virtual_network_name              = azurerm_virtual_network.private.name
  address_prefixes                  = [var.controller_subnet_cidr]
  private_endpoint_network_policies = "Disabled"
}

resource "azurerm_network_security_group" "runtime" {
  name                = "${var.name}-runtime"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags

  security_rule {
    name                       = "ControllerToPrivateGateway"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "${var.controller_private_ip}/32"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "AzureLoadBalancerTcpProbe"
    priority                   = 150
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "AzureLoadBalancer"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "DenyOtherInbound"
    priority                   = 200
    direction                  = "Inbound"
    access                     = "Deny"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

resource "azurerm_subnet_network_security_group_association" "runtime" {
  subnet_id                 = azurerm_subnet.runtime.id
  network_security_group_id = azurerm_network_security_group.runtime.id
}

resource "azurerm_network_security_group" "controller" {
  name                = "${var.name}-controller"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags

  dynamic "security_rule" {
    for_each = var.management_source_cidr == "" ? [] : [var.management_source_cidr]
    content {
      name                       = "PrivateDesktopToControllerManagement"
      priority                   = 100
      direction                  = "Inbound"
      access                     = "Allow"
      protocol                   = "Tcp"
      source_port_range          = "*"
      destination_port_range     = "22"
      source_address_prefix      = security_rule.value
      destination_address_prefix = "${var.controller_private_ip}/32"
    }
  }

  security_rule {
    name                       = "DenyUnconfiguredInbound"
    priority                   = 200
    direction                  = "Inbound"
    access                     = "Deny"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

resource "azurerm_subnet_network_security_group_association" "controller" {
  subnet_id                 = azurerm_subnet.controller.id
  network_security_group_id = azurerm_network_security_group.controller.id
}

resource "azurerm_lb" "gateway" {
  name                = "${var.name}-gateway"
  location            = var.location
  resource_group_name = var.resource_group_name
  sku                 = "Standard"
  tags                = var.tags

  frontend_ip_configuration {
    name                          = "private"
    subnet_id                     = azurerm_subnet.runtime.id
    private_ip_address_allocation = "Static"
    private_ip_address            = var.gateway_private_ip
  }
}

resource "azurerm_lb_backend_address_pool" "gateway" {
  name            = "owned-embed-host"
  loadbalancer_id = azurerm_lb.gateway.id
}

resource "azurerm_lb_probe" "gateway" {
  name            = "private-tls"
  loadbalancer_id = azurerm_lb.gateway.id
  protocol        = "Tcp"
  port            = 443
}

resource "azurerm_lb_rule" "gateway" {
  name                           = "controller-mtls-only"
  loadbalancer_id                = azurerm_lb.gateway.id
  protocol                       = "Tcp"
  frontend_port                  = 443
  backend_port                   = 443
  frontend_ip_configuration_name = "private"
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.gateway.id]
  probe_id                       = azurerm_lb_probe.gateway.id
}

resource "azurerm_private_dns_zone" "gateway" {
  name                = var.gateway_zone_name
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "gateway" {
  name                  = "${var.name}-gateway"
  resource_group_name   = var.resource_group_name
  private_dns_zone_name = azurerm_private_dns_zone.gateway.name
  virtual_network_id    = azurerm_virtual_network.private.id
  registration_enabled  = false
  tags                  = var.tags
}

resource "azurerm_private_dns_a_record" "gateway" {
  name                = "@"
  zone_name           = azurerm_private_dns_zone.gateway.name
  resource_group_name = var.resource_group_name
  ttl                 = 30
  records             = [var.gateway_private_ip]
  tags                = var.tags
}

resource "azurerm_private_dns_a_record" "guest" {
  name                = "*"
  zone_name           = azurerm_private_dns_zone.gateway.name
  resource_group_name = var.resource_group_name
  ttl                 = 30
  records             = [var.gateway_private_ip]
  tags                = var.tags
}

resource "azurerm_private_dns_zone" "vault" {
  name                = "privatelink.vaultcore.azure.net"
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "vault" {
  name                  = "${var.name}-vault"
  resource_group_name   = var.resource_group_name
  private_dns_zone_name = azurerm_private_dns_zone.vault.name
  virtual_network_id    = azurerm_virtual_network.private.id
  registration_enabled  = false
  tags                  = var.tags
}

resource "azurerm_private_endpoint" "vault" {
  name                = "${var.name}-vault"
  location            = var.location
  resource_group_name = var.resource_group_name
  subnet_id           = azurerm_subnet.controller.id
  tags                = var.tags

  private_service_connection {
    name                           = "${var.name}-vault"
    private_connection_resource_id = var.key_vault_id
    subresource_names              = ["vault"]
    is_manual_connection           = false
  }
  private_dns_zone_group {
    name                 = "vault"
    private_dns_zone_ids = [azurerm_private_dns_zone.vault.id]
  }
}
