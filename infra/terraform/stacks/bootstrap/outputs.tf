# Consumed by every later stack's -backend-config and by task 31's runbook.
output "state_resource_group_name" {
  description = "The resource group holding the state account; passed as -backend-config=resource_group_name=…"
  value       = azurerm_resource_group.state.name
}

output "state_storage_account_name" {
  description = "The state account; passed as -backend-config=storage_account_name=…"
  value       = azurerm_storage_account.state.name
}

output "state_container_name" {
  description = "The state container; backends/<env>.hcl names it too."
  value       = azurerm_storage_container.tfstate.name
}

output "product_resource_group_id" {
  description = "rg-panacea-app-<env>, created here so the infra identity's Contributor can be scoped to it (review I8)."
  value       = azurerm_resource_group.product.id
}
