# Three of Panacea's lessons are built in here (survey §1.11 and G-11):
#   1. A workspace with no daily cap is how a runaway stream becomes a bill, so daily_quota_gb
#      is required (the variable refuses the provider's -1).
#   2. Container Insights on a managed identity collects NOTHING until a Data Collection Rule
#      exists, so the DCR ships in the first apply; modules/aks (task 06) associates it, because
#      the association needs the cluster id.
#   3. kube-audit can blow a 1 GB/day cap, so AKS's own diagnostic setting (task 06) takes
#      kube-audit-admin — writes, not every read.
# One rule is ours: a sandbox's stdout is not ours to collect. The agent's process tree writes
# there, so panacea-sandboxes is excluded from container-log collection and only its inventory
# and events are kept.
locals {
  container_insights_streams = [
    "Microsoft-ContainerLogV2",
    "Microsoft-KubeEvents",
    "Microsoft-KubePodInventory",
    "Microsoft-KubeNodeInventory",
    "Microsoft-InsightsMetrics",
  ]
  log_analytics_destination_name = "ciworkspace"
}

# local_authentication_enabled = false: shared-key ingestion is on by default, and a shared key
# is a password that any pod that reads it can use. The cluster's agents write with their own
# Entra identity instead (4.81.0's name for this; local_authentication_disabled is deprecated).
resource "azurerm_log_analytics_workspace" "this" {
  name                         = "log-panacea-app-${var.environment}"
  location                     = var.location
  resource_group_name          = var.resource_group_name
  sku                          = "PerGB2018"
  retention_in_days            = var.retention_days
  daily_quota_gb               = var.daily_quota_gb
  local_authentication_enabled = false
  tags                         = var.tags
}

# UNVERIFIED: the extension's JSON keys are training knowledge of the Container Insights DCR
# schema. Azure accepts or rejects them only at apply (task 31); terraform validate cannot see
# inside a JSON string, and no test here can either.
resource "azurerm_monitor_data_collection_rule" "container_insights" {
  name                = "dcr-ci-panacea-app-${var.environment}"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags

  destinations {
    log_analytics {
      name                  = local.log_analytics_destination_name
      workspace_resource_id = azurerm_log_analytics_workspace.this.id
    }
  }

  data_sources {
    extension {
      name           = "ContainerInsightsExtension"
      extension_name = "ContainerInsights"
      streams        = local.container_insights_streams
      extension_json = jsonencode({
        dataCollectionSettings = {
          interval               = "1m"
          namespaceFilteringMode = "Exclude"
          namespaces             = var.excluded_log_namespaces
          enableContainerLogV2   = true
        }
      })
    }
  }

  data_flow {
    streams      = local.container_insights_streams
    destinations = [local.log_analytics_destination_name]
  }
}

# short_name is what a paging channel shows; Azure caps it at 12 characters.
resource "azurerm_monitor_action_group" "platform" {
  name                = "ag-panacea-app-${var.environment}"
  resource_group_name = var.resource_group_name
  short_name          = "panacea"
  tags                = var.tags

  dynamic "email_receiver" {
    for_each = var.alert_emails
    content {
      name                    = "email-${email_receiver.key}"
      email_address           = email_receiver.value
      use_common_alert_schema = true
    }
  }
}

data "azurerm_subscription" "current" {}

# Subscription scope, not resource group: the Kata VMs are billed in AKS's node resource group,
# which a budget on the foundation resource group would miss entirely. The infra identity holds
# Cost Management Contributor on the subscription for this — a role the owner grants it, not one
# any stack assigns, so it is not in task 10's allowlist of assigned roles.
#
# Four notifications, because three of them only report the spend after it happened. The
# forecast one is the only alert that arrives while the month can still be changed.
resource "azurerm_consumption_budget_subscription" "monthly" {
  name            = "budget-panacea-app-${var.environment}"
  subscription_id = data.azurerm_subscription.current.id
  amount          = var.monthly_budget_amount
  time_grain      = "Monthly"

  time_period {
    start_date = var.budget_start_date
  }

  dynamic "notification" {
    for_each = [
      { threshold = 50, threshold_type = "Actual" },
      { threshold = 80, threshold_type = "Actual" },
      { threshold = 100, threshold_type = "Actual" },
      { threshold = 100, threshold_type = "Forecasted" },
    ]
    content {
      enabled        = true
      threshold      = notification.value.threshold
      threshold_type = notification.value.threshold_type
      operator       = "GreaterThanOrEqualTo"
      contact_emails = var.alert_emails
      contact_groups = [azurerm_monitor_action_group.platform.id]
    }
  }
}
