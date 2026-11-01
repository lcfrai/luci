# Optional host capacity is a separately reviewed resource scope. The ordinary
# controller/runtime VMSS configuration is unchanged when this name is empty.
resource "azurerm_resource_group" "elastic_hosts" {
  count    = var.elastic_host_resource_group_name == "" ? 0 : 1
  name     = var.elastic_host_resource_group_name
  location = var.location
  tags     = var.tags
  lifecycle { prevent_destroy = true }
}
resource "azurerm_role_definition" "elastic_hosts" {
  count             = var.elastic_host_resource_group_name == "" ? 0 : 1
  name              = "${var.name}-elastic-hosts"
  scope             = azurerm_resource_group.elastic_hosts[0].id
  assignable_scopes = [azurerm_resource_group.elastic_hosts[0].id]
  permissions {
    actions = [
      "Microsoft.Compute/virtualMachines/read",
      "Microsoft.Compute/virtualMachines/write",
      "Microsoft.Compute/virtualMachines/delete",
      "Microsoft.Compute/disks/read",
      "Microsoft.Network/networkInterfaces/read",
      "Microsoft.Network/networkInterfaces/write",
      "Microsoft.Network/networkInterfaces/delete",
    ]
    not_actions = []
  }
}
resource "azurerm_role_assignment" "elastic_hosts" {
  count              = var.elastic_host_resource_group_name == "" ? 0 : 1
  scope              = azurerm_resource_group.elastic_hosts[0].id
  role_definition_id = azurerm_role_definition.elastic_hosts[0].role_definition_resource_id
  principal_id       = azurerm_user_assigned_identity.controller.principal_id
  principal_type     = "ServicePrincipal"
}
resource "azurerm_role_definition" "elastic_subnet" {
  count             = var.elastic_host_resource_group_name == "" ? 0 : 1
  name              = "${var.name}-elastic-subnet"
  scope             = "/subscriptions/${data.azurerm_client_config.current.subscription_id}"
  assignable_scopes = ["/subscriptions/${data.azurerm_client_config.current.subscription_id}"]
  permissions {
    actions = ["Microsoft.Network/virtualNetworks/subnets/read", "Microsoft.Network/virtualNetworks/subnets/join/action"]
    not_actions = []
  }
}
resource "azurerm_role_assignment" "elastic_subnet" {
  count              = var.elastic_host_resource_group_name == "" ? 0 : 1
  scope              = azurerm_subnet.runtime.id
  role_definition_id = azurerm_role_definition.elastic_subnet[0].role_definition_resource_id
  principal_id       = azurerm_user_assigned_identity.controller.principal_id
  principal_type     = "ServicePrincipal"
}
resource "azurerm_role_assignment" "elastic_identity" {
  count                = var.elastic_host_resource_group_name == "" ? 0 : 1
  scope                = azurerm_user_assigned_identity.runtime.id
  role_definition_name = "Managed Identity Operator"
  principal_id         = azurerm_user_assigned_identity.controller.principal_id
  principal_type       = "ServicePrincipal"
}
resource "azurerm_role_assignment" "elastic_image" {
  count                = var.elastic_host_resource_group_name == "" ? 0 : 1
  scope                = var.runtime_image_version_id
  role_definition_name = "Reader"
  principal_id         = azurerm_user_assigned_identity.controller.principal_id
  principal_type       = "ServicePrincipal"
}
