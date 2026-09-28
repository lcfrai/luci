# modules/observability's tests (phase 8 task 04). Mocks only: nothing here reaches Azure.
#
# Every run is `command = plan`, with the mocks applied at plan (override_during = plan).
# azurerm checks the form of a Resource Manager id it is given — the DCR's
# workspace_resource_id, the budget's subscription_id, the notification's contact_groups — even
# against a mock, so every mocked id is well formed and lives in the all-zeros subscription,
# naming nothing real (task 02's lesson, carried by task 03).
mock_provider "azurerm" {
  override_during = plan
  # local_authentication_enabled is mocked to Azure's OWN default, true, not to the value we want.
  # An unset optional attribute is otherwise filled with its type's zero value — false — which is
  # exactly what the security assert below looks for, so deleting main.tf's line left the suite
  # green (review 1, C1). Mocking the unsafe default makes the assert read the configuration:
  # config wins over a mock default, and a module that stops setting it fails.
  mock_resource "azurerm_log_analytics_workspace" {
    defaults = {
      id                           = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-panacea-app-dev/providers/Microsoft.OperationalInsights/workspaces/log-panacea-app-dev"
      local_authentication_enabled = true
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
    condition     = tolist(jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.namespaces) == var.excluded_log_namespaces
    error_message = "The DCR's excluded namespaces are not exactly var.excluded_log_namespaces."
  }
  assert {
    condition     = jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.enableContainerLogV2 == true
    error_message = "The DCR does not enable ContainerLogV2, whose schema carries the namespace the filter is applied to."
  }
  # The default is pinned to a literal, not to var.excluded_log_namespaces: no run set that
  # variable, so the old comparison was default == default and a default that quietly lost
  # kube-system shipped green (review 1, I1). kube-system is CoreDNS, kube-proxy, metrics-server
  # and the AMA agent's own stdout — enough to trip a 5 GB/day cap on its own.
  assert {
    condition     = tolist(jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.namespaces) == tolist(["kube-system", "panacea-sandboxes"])
    error_message = "The default excluded namespaces are no longer exactly kube-system and panacea-sandboxes."
  }
}

# I1's other half: a run whose input differs from the default, so a hard-coded namespace list in
# main.tf (which the default pin above cannot see) fails here.
run "the_excluded_namespaces_come_from_the_input" {
  command = plan
  variables {
    excluded_log_namespaces = ["kube-system", "panacea-sandboxes", "gatekeeper-system"]
  }
  assert {
    condition     = tolist(jsondecode(azurerm_monitor_data_collection_rule.container_insights.data_sources[0].extension[0].extension_json).dataCollectionSettings.namespaces) == tolist(["kube-system", "panacea-sandboxes", "gatekeeper-system"])
    error_message = "var.excluded_log_namespaces does not reach the DCR's namespace filter: the list is hard-coded somewhere in main.tf."
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
  # kind = "Linux" is what Microsoft's own Container Insights DCR carries. UNVERIFIED against a
  # real apply (task 31), like the extension JSON, but it validates in 4.81.0 and a DCR that
  # Azure reads as the wrong kind is the same failure this module exists to close: it exists, the
  # association succeeds, and nothing is collected (review 1, I3).
  assert {
    condition     = azurerm_monitor_data_collection_rule.container_insights.kind == "Linux"
    error_message = "The DCR is not kind Linux; Container Insights on AzureLinux nodes is collected by the Linux agent."
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

# Counting receivers proved only that there were as many as there were addresses, so a constant
# address in main.tf's dynamic block left nine runs green (review 1, I2). Two addresses, and each
# receiver is read by address and by the name it is paired with, so a constant, a dropped address
# and a mis-paired name all fail. The budget's own contact_emails are read here too: its earlier
# assert ran with a single address, where alert_emails[0] is indistinguishable from alert_emails.
run "every_alert_address_reaches_the_action_group" {
  command = plan
  variables {
    alert_emails = ["platform@example.invalid", "oncall@example.invalid"]
  }
  assert {
    condition     = toset([for r in azurerm_monitor_action_group.platform.email_receiver : "${r.name}=${r.email_address}"]) == toset(["email-0=platform@example.invalid", "email-1=oncall@example.invalid"])
    error_message = "The action group's receivers are not one per var.alert_emails address, named email-<index>: an alert that alerts nobody."
  }
  assert {
    condition     = alltrue([for r in azurerm_monitor_action_group.platform.email_receiver : r.use_common_alert_schema])
    error_message = "A receiver does not use the common alert schema, so its payload differs per alert type."
  }
  assert {
    condition     = alltrue([for n in azurerm_consumption_budget_subscription.monthly.notification : toset(n.contact_emails) == toset(var.alert_emails)])
    error_message = "A budget notification does not carry every alert address."
  }
}

# Every value here differs from the file-level variables, so a name, region, group or tag set
# frozen to dev's fails. Without it three names could be hard-coded to -dev and the DCR and action
# group could drop tags with the suite green (review 1, M1); a prod estate would then carry
# dcr-ci-panacea-app-dev and miss tag-based cost attribution.
run "names_region_group_and_tags_come_from_the_inputs" {
  command = plan
  variables {
    environment         = "prod"
    location            = "australiasoutheast"
    resource_group_name = "rg-panacea-app-prod"
    tags                = { owner = "platform", stack = "foundation" }
  }
  assert {
    condition     = azurerm_log_analytics_workspace.this.name == "log-panacea-app-prod" && azurerm_monitor_data_collection_rule.container_insights.name == "dcr-ci-panacea-app-prod" && azurerm_monitor_action_group.platform.name == "ag-panacea-app-prod" && azurerm_consumption_budget_subscription.monthly.name == "budget-panacea-app-prod"
    error_message = "The workspace, DCR, action group or budget does not carry var.environment in its name."
  }
  assert {
    condition     = alltrue([for r in [azurerm_log_analytics_workspace.this, azurerm_monitor_data_collection_rule.container_insights] : r.location == "australiasoutheast" && r.resource_group_name == "rg-panacea-app-prod"]) && azurerm_monitor_action_group.platform.resource_group_name == "rg-panacea-app-prod"
    error_message = "The workspace, DCR or action group does not take var.location and var.resource_group_name."
  }
  assert {
    condition     = alltrue([for r in [azurerm_log_analytics_workspace.this, azurerm_monitor_data_collection_rule.container_insights, azurerm_monitor_action_group.platform] : r.tags == tomap({ owner = "platform", stack = "foundation" })])
    error_message = "The workspace, DCR or action group does not take var.tags, so tag-based cost attribution misses it."
  }
}

run "a_budget_of_zero_is_rejected" {
  command = plan
  variables {
    monthly_budget_amount = 0
  }
  expect_failures = [var.monthly_budget_amount]
}

# Azure rejects a mid-month start at apply; this rejects it at plan, where the message names the
# variable (review 1, M4).
run "a_mid_month_budget_start_is_rejected" {
  command = plan
  variables {
    budget_start_date = "2026-11-15T00:00:00Z"
  }
  expect_failures = [var.budget_start_date]
}
