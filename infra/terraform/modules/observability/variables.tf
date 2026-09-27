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

# -1 is the provider's "uncapped" value, and Panacea's lesson (survey §1.11) is that an uncapped
# workspace is how a runaway kube-audit stream becomes a bill. The floor is 1, so -1 is refused
# by the same validation that refuses 0.
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
