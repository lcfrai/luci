resource "azurerm_network_interface" "controller" {
  name                = "${var.name}-controller"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags

  ip_configuration {
    name                          = "private"
    subnet_id                     = azurerm_subnet.controller.id
    private_ip_address_allocation = "Static"
    private_ip_address            = var.controller_private_ip
  }
}

resource "azurerm_linux_virtual_machine" "controller" {
  name                            = "${var.name}-controller"
  location                        = var.location
  resource_group_name             = var.resource_group_name
  size                            = var.controller_vm_size
  network_interface_ids           = [azurerm_network_interface.controller.id]
  admin_username                  = "luci"
  disable_password_authentication = true
  zone                            = var.zone
  tags                            = var.tags

  admin_ssh_key {
    username   = "luci"
    public_key = var.controller_admin_ssh_public_key
  }
  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.controller.id]
  }
  source_image_id = var.controller_image_version_id
  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }
}

resource "azurerm_managed_disk" "controller" {
  name                          = "${var.name}-authoritative"
  location                      = var.location
  resource_group_name           = var.resource_group_name
  storage_account_type          = "Premium_LRS"
  create_option                 = "Empty"
  disk_size_gb                  = var.controller_disk_size_gb
  public_network_access_enabled = false
  network_access_policy         = "DenyAll"
  zone                          = var.zone
  tags                          = var.tags

  lifecycle {
    prevent_destroy = true
  }
}

resource "azurerm_virtual_machine_data_disk_attachment" "controller" {
  managed_disk_id    = azurerm_managed_disk.controller.id
  virtual_machine_id = azurerm_linux_virtual_machine.controller.id
  lun                = 0
  caching            = "None"
}

locals {
  host_boot = merge({
    version              = 1
    scope                = var.scope
    generation           = var.generation
    runtimePackageSHA256 = var.runtime_package_sha256
    vaultURL             = "https://${var.key_vault_name}.vault.azure.net"
    gatewayHost          = var.gateway_zone_name
    gatewayPrivateIP     = var.gateway_private_ip
    controllerPrivateIP  = var.controller_private_ip
    runtimeSubnetCIDR    = var.runtime_subnet_cidr
    hostClientID         = azurerm_user_assigned_identity.runtime.client_id
    controllerClientID   = azurerm_user_assigned_identity.controller.client_id
    secretRefs           = var.host_secret_refs
  }, var.previous_generation == null ? {} : { previousGeneration = var.previous_generation })
}

resource "azurerm_linux_virtual_machine_scale_set" "runtime" {
  name                            = "${var.name}-runtime"
  location                        = var.location
  resource_group_name             = var.resource_group_name
  sku                             = var.runtime_vm_size
  instances                       = 1
  upgrade_mode                    = "Manual"
  zones                           = [var.zone]
  admin_username                  = "luci"
  disable_password_authentication = true
  source_image_id                 = var.runtime_image_version_id
  custom_data                     = base64encode("#cloud-config\nwrite_files:\n  - path: /etc/luci/azure-embed-deployment.json\n    permissions: '0600'\n    owner: root:root\n    content: '${base64encode(jsonencode(local.host_boot))}'\n    encoding: b64\nruncmd:\n  - [systemctl, enable, --now, run-luci_embed.mount]\n  - [systemctl, enable, --now, luci-azure-embed-host.service]\n")
  tags                            = var.tags

  admin_ssh_key {
    username   = "luci"
    public_key = var.runtime_admin_ssh_public_key
  }
  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.runtime.id]
  }
  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }
  network_interface {
    name    = "private"
    primary = true
    ip_configuration {
      name                                   = "private"
      primary                                = true
      subnet_id                              = azurerm_subnet.runtime.id
      load_balancer_backend_address_pool_ids = [azurerm_lb_backend_address_pool.gateway.id]
    }
  }

  lifecycle {
    precondition {
      condition     = length(base64encode("#cloud-config\nwrite_files:\n  - path: /etc/luci/azure-embed-deployment.json\n    permissions: '0600'\n    owner: root:root\n    content: '${base64encode(jsonencode(local.host_boot))}'\n    encoding: b64\nruncmd:\n  - [systemctl, enable, --now, run-luci_embed.mount]\n  - [systemctl, enable, --now, luci-azure-embed-host.service]\n")) < 65536
      error_message = "Nonsecret Azure host custom data exceeds the provider limit."
    }
  }
}
