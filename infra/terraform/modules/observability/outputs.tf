output "workspace_id" {
  description = "The Log Analytics workspace's resource id, for AKS's own diagnostic setting (task 06)."
  value       = azurerm_log_analytics_workspace.this.id
}

output "workspace_name" {
  description = "The workspace's name."
  value       = azurerm_log_analytics_workspace.this.name
}

output "container_insights_dcr_id" {
  description = "Associated with the cluster by modules/aks (the association needs the cluster id)."
  value       = azurerm_monitor_data_collection_rule.container_insights.id
}

output "action_group_id" {
  description = "The platform action group, for any alert rule another module adds."
  value       = azurerm_monitor_action_group.platform.id
}
