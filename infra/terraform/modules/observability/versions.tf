# modules/observability — the Log Analytics workspace and its daily cap, the Container Insights
# data collection rule, the platform action group and the subscription budget (phase 8 task 04).
# Called once, by stacks/foundation (task 07).
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = "4.81.0" }
  }
}
