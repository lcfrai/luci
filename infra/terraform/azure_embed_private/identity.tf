resource "azurerm_user_assigned_identity" "runtime" {
  name                = "${var.name}-runtime"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_user_assigned_identity" "controller" {
  name                = "${var.name}-controller"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

locals {
  key_vault_secrets_user_role = "4633458b-17de-408a-b874-0445c86b69e6"
}

# Each identity can read only the secret resources explicitly named for its
# role. No secret value, access token, certificate PEM or SAS enters Terraform.
resource "azurerm_role_assignment" "runtime_secret" {
  for_each                         = var.host_secret_ids
  scope                            = each.value
  role_definition_id               = "/subscriptions/${data.azurerm_client_config.current.subscription_id}/providers/Microsoft.Authorization/roleDefinitions/${local.key_vault_secrets_user_role}"
  principal_id                     = azurerm_user_assigned_identity.runtime.principal_id
  principal_type                   = "ServicePrincipal"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "controller_secret" {
  for_each                         = var.controller_secret_ids
  scope                            = each.value
  role_definition_id               = "/subscriptions/${data.azurerm_client_config.current.subscription_id}/providers/Microsoft.Authorization/roleDefinitions/${local.key_vault_secrets_user_role}"
  principal_id                     = azurerm_user_assigned_identity.controller.principal_id
  principal_type                   = "ServicePrincipal"
  skip_service_principal_aad_check = true
}

data "azurerm_client_config" "current" {}

# Only the separately hosted controller identity can read one runtime VMSS
# instance and invoke its protected guest-agent RunCommand. The role grants no
# VM create/delete, public SSH, disk mutation or broader resource-group rights.
resource "azurerm_role_definition" "controller_host_rotation" {
  name              = "${var.name}-host-rotation"
  scope             = azurerm_linux_virtual_machine_scale_set.runtime.id
  assignable_scopes = [azurerm_linux_virtual_machine_scale_set.runtime.id]
  permissions {
    actions = [
      "Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read",
      "Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommand/action",
    ]
    not_actions = []
  }
}

resource "azurerm_role_assignment" "controller_host_rotation" {
  scope                            = azurerm_linux_virtual_machine_scale_set.runtime.id
  role_definition_id               = azurerm_role_definition.controller_host_rotation.role_definition_resource_id
  principal_id                     = azurerm_user_assigned_identity.controller.principal_id
  principal_type                   = "ServicePrincipal"
  skip_service_principal_aad_check = true
}
