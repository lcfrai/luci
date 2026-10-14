output "controller_instance_id" { value = lower(azurerm_linux_virtual_machine.controller.id) }
output "controller_management_private_ip" { value = var.controller_private_ip }
output "management_source_cidr" { value = var.management_source_cidr }
output "controller_recovery_instance_id" { value = lower(azurerm_linux_virtual_machine.controller_recovery.id) }
output "controller_disk_id" { value = lower(azurerm_managed_disk.controller.id) }
output "controller_recovery_identity_client_id" { value = azurerm_user_assigned_identity.recovery.client_id }
output "controller_identity_resource_id" { value = lower(azurerm_user_assigned_identity.controller.id) }
output "controller_recovery_identity_resource_id" { value = lower(azurerm_user_assigned_identity.recovery.id) }
output "controller_fence_blob_url" { value = "https://${azurerm_storage_account.fence.name}.blob.core.windows.net/${azurerm_storage_container.fence.name}/controller-fence.json" }
output "runtime_scale_set_id" { value = lower(azurerm_linux_virtual_machine_scale_set.runtime.id) }
output "runtime_identity_client_id" { value = azurerm_user_assigned_identity.runtime.client_id }
output "controller_identity_client_id" { value = azurerm_user_assigned_identity.controller.client_id }
output "private_gateway_host" { value = azurerm_private_dns_zone.gateway.name }
output "private_gateway_frontend_ip" { value = var.gateway_private_ip }
output "runtime_package_sha256" { value = var.runtime_package_sha256 }
output "runtime_host_config_sha256" { value = sha256(jsonencode(local.host_boot)) }
output "runtime_host_config_json" {
  value       = jsonencode(local.host_boot)
  sensitive   = true
  description = "Nonsecret exact host-generation selection for an authenticated protected rotation channel; names and versions are operational metadata."
}
output "runtime_image_version_id" { value = var.runtime_image_version_id }
