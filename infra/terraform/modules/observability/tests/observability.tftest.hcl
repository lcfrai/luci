# modules/observability's tests (phase 8 task 04). Mocks only: nothing here reaches Azure.
#
# Every run is `command = plan`, with the mocks applied at plan (override_during = plan).
# azurerm checks the form of a Resource Manager id it is given — the DCR's
# workspace_resource_id, the budget's subscription_id, the notification's contact_groups — even
# against a mock, so every mocked id is well formed and lives in the all-zeros subscription,
# naming nothing real (task 02's lesson, carried by task 03).
mock_provider "azurerm" {
  override_during = plan
  mock_resource "azurerm_log_analytics_workspace" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.OperationalInsights/workspaces/log-panacea-app-dev"
    }
  }
  mock_resource "azurerm_monitor_data_collection_rule" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Insights/dataCollectionRules/dcr-ci-panacea-app-dev"
    }
  }
  mock_resource "azurerm_monitor_action_group" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.Insights/actionGroups/ag-panacea-app-dev"
    }
  }
  mock_data "azurerm_subscription" {
    defaults = {
      id              = "/subscriptions/00000000-0000-0000-0000-000000000000"
      subscription_id = "00000000-0000-0000-0000-000000000000"
      display_name    = "mock"
    }
  }
}

variables {
  environment           = "dev"
  location              = "australiaeast"
  resource_group_name   = "rg-panacea-app-dev"
  retention_days        = 30
  daily_quota_gb        = 5
  alert_emails          = ["owner@example.invalid"]
  monthly_budget_amount = 3000
  budget_start_date     = "2026-11-01T00:00:00Z"
  tags                  = {}
}

# The cap's wiring is asserted here, on the happy path, because the run that proves -1 is refused
# is a run Terraform expects to fail at variable validation: expect_failures and assert cannot
# both be reached in one run (task 04 as built).
run "workspace_has_no_shared_key_auth" {
  command = plan
  assert {
    condition     = azurerm_log_analytics_workspace.this.local_authentication_enabled == false
    error_message = "The workspace still accepts shared-key ingestion; only Entra-authenticated agents may write to it."
  }
  assert {
    condition     = azurerm_log_analytics_workspace.this.name == "log-panacea-app-dev" && azurerm_log_analytics_workspace.this.sku == "PerGB2018"
    error_message = "The workspace is not log-panacea-app-<env> on PerGB2018."
  }
  assert {
    condition     = azurerm_log_analytics_workspace.this.daily_quota_gb == var.daily_quota_gb && azurerm_log_analytics_workspace.this.retention_in_days == var.retention_days
    error_message = "The daily cap or the retention does not reach the workspace."
  }
  assert {
    condition     = azurerm_log_analytics_workspace.this.resource_group_name == var.resource_group_name && azurerm_log_analytics_workspace.this.location == var.location
    error_message = "The workspace is not in the foundation resource group and region."
  }
}

run "workspace_has_a_daily_cap" {
  command = plan
  variables {
    daily_quota_gb = -1
  }
  expect_failures = [var.daily_quota_gb]
}

run "retention_below_30_is_rejected" {
  command = plan
  variables {
    retention_days = 7
  }
  expect_failures = [var.retention_days]
}

# A test file has no locals, so the extension's JSON is decoded in each condition it is read in.
run "sandbox_stdout_is_never_collected" {
  command = plan
  assert {
    condition     = jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.namespaceFilteringMode == "Exclude"
    error_message = "The DCR's namespaceFilteringMode is not Exclude; Include would make the list an allowlist and collect a sandbox's stdout."
  }
  assert {
    condition     = contains(jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.namespaces, "panacea-sandboxes")
    error_message = "panacea-sandboxes is not among the DCR's excluded namespaces: a sandbox's stdout belongs to the agent's process tree, not to us."
  }
  assert {
    condition     = jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.namespaces == var.excluded_log_namespaces
    error_message = "The DCR's excluded namespaces are not exactly var.excluded_log_namespaces."
  }
  assert {
    condition     = jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.enableContainerLogV2 == true
    error_message = "The DCR does not enable ContainerLogV2, whose schema carries the namespace the filter is applied to."
  }
}

run "removing_the_sandbox_exclusion_is_rejected" {
  command = plan
  variables {
    excluded_log_namespaces = ["kube-system"]
  }
  expect_failures = [var.excluded_log_namespaces]
}

run "dcr_sends_to_this_workspace" {
  command = plan
  assert {
    condition     = length(azurerm_monitor_data_collection_rule.container_insights.destinations[0].log_analytics) == 1
    error_message = "The DCR does not have exactly one Log Analytics destination."
  }
  assert {
    condition     = one(azurerm_monitor_data_collection_rule.container_insights.destinations[0].log_analytics).workspace_resource_id == azurerm_log_analytics_workspace.this.id
    error_message = "The DCR's destination is not this module's workspace: a DCR pointed at nothing collects nothing (Panacea's gap)."
  }
  assert {
    condition     = one(azurerm_monitor_data_collection_rule.container_insights.data_flow).destinations == tolist([one(azurerm_monitor_data_collection_rule.container_insights.destinations[0].log_analytics).name])
    error_message = "The DCR's data flow does not name its one destination, so the streams reach nowhere."
  }
  assert {
    condition     = toset(one(azurerm_monitor_data_collection_rule.container_insights.data_flow).streams) == toset(["Microsoft-ContainerLogV2", "Microsoft-KubeEvents", "Microsoft-KubePodInventory", "Microsoft-KubeNodeInventory", "Microsoft-InsightsMetrics"])
    error_message = "The data flow does not carry the five Container Insights streams."
  }
  assert {
    condition     = toset(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].streams) == toset(one(azurerm_monitor_data_collection_rule.container_insights.data_flow).streams) && azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_name == "ContainerInsights"
    error_message = "The extension collects a different set of streams from the one the flow carries, or is not the ContainerInsights extension."
  }
}

run "budget_warns_at_four_points" {
  command = plan
  assert {
    condition     = length(azurerm_consumption_budget_subscription.monthly.notification) == 4
    error_message = "The budget does not have exactly four notifications."
  }
  assert {
    condition     = toset([for n in azurerm_consumption_budget_subscription.monthly.notification : "${n.threshold_type}:${n.threshold}:${n.operator}:${n.enabled}"]) == toset(["Actual:50:GreaterThanOrEqualTo:true", "Actual:80:GreaterThanOrEqualTo:true", "Actual:100:GreaterThanOrEqualTo:true", "Forecasted:100:GreaterThanOrEqualTo:true"])
    error_message = "The budget does not warn at actual 50, 80 and 100 and forecast 100; the forecast alert is the one that fires before the money is spent."
  }
  assert {
    condition     = alltrue([for n in azurerm_consumption_budget_subscription.monthly.notification : toset(n.contact_emails) == toset(var.alert_emails) && toset(n.contact_groups) == toset([azurerm_monitor_action_group.platform.id])])
    error_message = "A budget notification does not reach both the alert emails and the platform action group."
  }
  assert {
    condition     = azurerm_monitor_action_group.platform.short_name == "panacea" && length(azurerm_monitor_action_group.platform.email_receiver) == length(var.alert_emails)
    error_message = "The action group is not short-named panacea with one email receiver per address."
  }
}

run "budget_is_subscription_scoped" {
  command = plan
  assert {
    condition     = azurerm_consumption_budget_subscription.monthly.subscription_id == data.azurerm_subscription.current.id
    error_message = "The budget is not scoped to this subscription: a resource-group budget misses AKS's node resource group, where the Kata VMs are billed."
  }
  assert {
    condition     = azurerm_consumption_budget_subscription.monthly.time_grain == "Monthly" && one(azurerm_consumption_budget_subscription.monthly.time_period).start_date == var.budget_start_date && azurerm_consumption_budget_subscription.monthly.amount == var.monthly_budget_amount
    error_message = "The budget is not a monthly budget of var.monthly_budget_amount starting at var.budget_start_date."
  }
}

run "no_recipients_is_rejected" {
  command = plan
  variables {
    alert_emails = []
  }
  expect_failures = [var.alert_emails]
}
