### Task 04: `modules/observability` — the workspace, the collection rule, the alerts, the budget

Panacea learned three things here (survey §1.11, `:213-223`, and G-11): a workspace needs a
**daily ingest cap**; Container Insights on managed identity collects **nothing** until a Data
Collection Rule exists; and kube-audit "can blow the dev 1 GB/day workspace cap". So the DCR ships
in the first apply, the cap is sized for audit, and audit uses the **admin** category (writes, not
every read). One more rule is ours: **a sandbox's stdout is not ours to collect.** The agent's
process tree writes there; this module excludes `panacea-sandboxes` from container-log collection
and keeps only its inventory and events.

**Files:**
- `infra/terraform/modules/observability/{versions,main,variables,outputs}.tf` (new)
- `infra/terraform/modules/observability/tests/observability.tftest.hcl` (new)

#### Contract

```hcl
# versions.tf — as task 03 (azurerm 4.81.0 only)

# variables.tf
variable "environment" {
  type        = string
  description = "dev or prod."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "location" {
  type        = string
  description = "Azure region."
}
variable "resource_group_name" {
  type        = string
  description = "The foundation resource group."
}
variable "retention_days" {
  type        = number
  description = "Workspace retention. The ISM-required period is an owner input (survey §7.3); 30 in dev."
  validation {
    condition     = var.retention_days >= 30 && var.retention_days <= 730
    error_message = "retention_days must be between 30 and 730."
  }
}
variable "daily_quota_gb" {
  type        = number
  description = "Daily ingest cap in GB. Sized for kube-audit-admin plus Container Insights; ingestion stops for the day when reached."
  validation {
    condition     = var.daily_quota_gb >= 1
    error_message = "daily_quota_gb must be at least 1; -1 (uncapped) is not allowed here."
  }
}
variable "excluded_log_namespaces" {
  type        = list(string)
  description = "Namespaces whose container stdout/stderr is never collected. Must include panacea-sandboxes."
  default     = ["kube-system", "panacea-sandboxes"]
  validation {
    condition     = contains(var.excluded_log_namespaces, "panacea-sandboxes")
    error_message = "panacea-sandboxes must stay excluded: a sandbox's stdout belongs to the agent's process tree."
  }
}
variable "alert_emails" {
  type        = list(string)
  description = "Who receives budget and platform alerts (owner input)."
  validation {
    condition     = length(var.alert_emails) > 0
    error_message = "At least one alert recipient is required, or the budget alerts nobody."
  }
}
variable "monthly_budget_amount" {
  type        = number
  description = "Subscription budget per month, in the billing account's currency (AUD expected). Owner input."
  validation {
    condition     = var.monthly_budget_amount > 0
    error_message = "monthly_budget_amount must be positive."
  }
}
variable "budget_start_date" {
  type        = string
  description = "RFC 3339 first day of a month, not in the past at first apply, e.g. 2026-11-01T00:00:00Z."
  validation {
    condition     = can(regex("^\\d{4}-\\d{2}-01T00:00:00Z$", var.budget_start_date))
    error_message = "budget_start_date must be the first of a month at 00:00:00Z."
  }
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

# outputs.tf
output "workspace_id" { value = azurerm_log_analytics_workspace.this.id }
output "workspace_name" { value = azurerm_log_analytics_workspace.this.name }
output "container_insights_dcr_id" {
  description = "Associated with the cluster by modules/aks (the association needs the cluster id)."
  value       = azurerm_monitor_data_collection_rule.container_insights.id
}
output "action_group_id" { value = azurerm_monitor_action_group.platform.id }
```

#### Resources (described)

- `azurerm_log_analytics_workspace.this` — `log-panacea-app-${env}`, `sku = "PerGB2018"`,
  `retention_in_days`, `daily_quota_gb`, **`local_authentication_enabled = false`** (4.81.0's
  name; `local_authentication_disabled` is deprecated in this version).
- `azurerm_monitor_data_collection_rule.container_insights` — `dcr-ci-panacea-app-${env}`,
  `kind = "Linux"` (fix round, review 1 I3: Microsoft's own Container Insights DCR carries it and
  the nodes are AzureLinux; **UNVERIFIED** like the extension JSON below, true at apply only); one
  `data_sources.extension` named `ContainerInsightsExtension`, `extension_name = "ContainerInsights"`,
  streams `Microsoft-ContainerLogV2`, `Microsoft-KubeEvents`, `Microsoft-KubePodInventory`,
  `Microsoft-KubeNodeInventory`, `Microsoft-InsightsMetrics`, and
  `extension_json = jsonencode({ dataCollectionSettings = { interval = "1m",
  namespaceFilteringMode = "Exclude", namespaces = var.excluded_log_namespaces,
  enableContainerLogV2 = true } })`; one `log_analytics` destination (this workspace); one
  `data_flow` from those streams to it. **The extension's JSON keys are from training knowledge
  of the Container Insights DCR schema, UNVERIFIED** — the Azure side accepts or rejects them only
  at apply (task 31); `terraform validate` cannot see inside a JSON string.
- `azurerm_monitor_action_group.platform` — `short_name = "panacea"`, one `email_receiver` per
  address.
- `data "azurerm_subscription" "current" {}` and
  `azurerm_consumption_budget_subscription.monthly` — `time_grain = "Monthly"`,
  `time_period { start_date = var.budget_start_date }`, and four notifications:
  actual ≥ 50 %, actual ≥ 80 %, actual ≥ 100 %, **forecast** ≥ 100 %, each to `alert_emails` and
  the action group. Subscription scope, not resource group, because the Kata VMs are billed in
  AKS's node resource group, which a resource-group budget on the foundation RG would miss. The
  **infra** identity needs Cost Management Contributor on the subscription for this. That is a
  role the identity **holds**, granted by the owner, and it is listed in the plan's "Deploy
  identities" table. It is not a role any stack **assigns**, so it is not in task 10's allowlist,
  which lists only assigned roles (review I8 corrected an earlier line that said the allowlist
  held it).

AKS's own diagnostic setting (`kube-audit-admin`, `kube-apiserver`, `cluster-autoscaler`, `guard`)
is in `modules/aks` (task 06), because it needs the cluster id.

#### Tests to write first (`tests/observability.tftest.hcl`, mocks only)

| Run | Asserts | Mutant it kills |
|---|---|---|
| `workspace_has_no_shared_key_auth` | `local_authentication_enabled == false`, the name and SKU, and (as built) `daily_quota_gb == var.daily_quota_gb` and `retention_in_days == var.retention_days` | shared-key ingestion left on (the default); a cap or a retention that never reaches the workspace |
| `workspace_has_a_daily_cap` | the variable rejects `-1`: `daily_quota_gb = -1` → `expect_failures = [var.daily_quota_gb]` | an uncapped workspace |
| `retention_below_30_is_rejected` | `retention_days = 7` → `expect_failures = [var.retention_days]` | the validation dropped |
| `sandbox_stdout_is_never_collected` | `jsondecode(…extension[0].extension_json).dataCollectionSettings.namespaces` contains `panacea-sandboxes`, `tolist(…namespaces) == var.excluded_log_namespaces`, `namespaceFilteringMode == "Exclude"` and `enableContainerLogV2 == true` | the filter mode flipped to Include, or the namespace list emptied |
| `removing_the_sandbox_exclusion_is_rejected` | `excluded_log_namespaces = ["kube-system"]` → `expect_failures` | the validation dropped |
| `dcr_sends_to_this_workspace` | the single `log_analytics` destination's `workspace_resource_id` equals `azurerm_log_analytics_workspace.this.id` (mock id) | a DCR pointed at nothing, which collects nothing (Panacea's gap) |
| `budget_warns_at_four_points` | four notifications: thresholds 50/80/100 `Actual` and 100 `Forecasted` | the forecast alert dropped (it is the one that fires before the money is spent) |
| `budget_is_subscription_scoped` | the budget is `azurerm_consumption_budget_subscription` and its `subscription_id` is the data source's `id` | a resource-group budget that misses the node resource group |
| `no_recipients_is_rejected` | `alert_emails = []` → `expect_failures` | an alert that alerts nobody |
| `the_excluded_namespaces_come_from_the_input` (fix round) | a three-namespace input, and the decoded list equals that literal | the namespace list hard-coded in `main.tf`, so the variable reaches nothing |
| `every_alert_address_reaches_the_action_group` (fix round) | two addresses; each receiver read by `name=email_address`, `use_common_alert_schema`, and the budget's `contact_emails` | a constant receiver address, a mis-paired receiver name, a budget that mails only the first address |
| `names_region_group_and_tags_come_from_the_inputs` (fix round) | prod environment, `australiasoutheast`, a prod group and a real tag map: the four names, the two locations, the three groups, the three tag maps | three names frozen to `-dev`; the DCR or action group shipping untagged, so tag-based cost attribution misses them |
| `a_budget_of_zero_is_rejected` (fix round) | `monthly_budget_amount = 0` → `expect_failures` | the validation dropped |
| `a_mid_month_budget_start_is_rejected` (fix round) | `budget_start_date = "2026-11-15T00:00:00Z"` → `expect_failures` | the validation dropped |

Fourteen runs after the fix round (review 1: C1, I1, I2, I3, M1, M4).

Four notes from building it:

- **A mock default is a value the assert may be reading instead of the configuration.** An unset
  optional attribute is filled with its type's zero value, so
  `local_authentication_enabled == false` passed with the line deleted from `main.tf` (review 1,
  C1). The mock now sets the attribute to Azure's own default, `true`; configuration wins over a
  mock default, so the assert reads what the module sets and a module that stops setting it
  fails. Any `== false` assert on a security attribute needs the same treatment.

- **`expect_failures` and `assert` cannot share a run.** A run that expects a variable validation
  to fail never reaches its asserts, so the cap's *wiring* assert lives on the happy-path run
  (`workspace_has_no_shared_key_auth`) and `workspace_has_a_daily_cap` is only the `-1` rejection.
  The count stays at nine runs.
- **`jsondecode` returns a tuple**, not a `list(string)`, so comparing the decoded `namespaces`
  to `var.excluded_log_namespaces` needs `tolist(…)`; without it the assert fails with
  "LHS and RHS values are of different types".
- The provider's block collections are read through `one(…)` (`destinations[0].log_analytics`,
  `data_flow`, `time_period`) so the assert does not depend on whether the schema is a list or
  a set.

#### Steps

- [x] **Step 1:** Variables and tests; validate fails.
- [x] **Step 2:** Workspace, DCR, action group, budget, running tests after each.
- [x] **Step 3:** Outputs, fmt, guards, commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/observability
# validate: OK   infra/terraform/modules/observability
# validate: 1 ok, 0 failed                            (exit 0)
```

`validate.sh` prints a step's own output only when that step fails, so `Success! 14 passed,
0 failed.` is not on stdout on a green run; the runs are enumerated in a red run's output
instead.

#### What this task does not claim

- That the DCR's extension JSON, or `kind = "Linux"`, is what Azure accepts (above). `kind`
  validates in 4.81.0 and nothing offline can say more; task 31 is the first place it is true or
  false. If Container Insights collects nothing there, `kind` is the first thing to check.
- That the alert rules an operator would want exist. The action group is built and wired to the
  budget, but **no alert rule uses it**, so nothing announces that the daily cap has tripped and
  stopped ingestion for the day — `kube-audit-admin` included. Review 1 M5: a plan-level gap,
  nearest owner task 32 (measurement), not a deviation from this part.
- That the daily cap is large enough: the dev value (`5`, set in task 07) is a guess until task 32
  measures a day of kube-audit-admin.
- That Log Analytics needs no private link: dev uses public ingestion with Entra-authenticated
  agents; a private-link scope is phase 9's (S8-P9.1).
