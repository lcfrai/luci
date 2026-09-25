# stacks/bootstrap — where every other stack keeps its state (phase 8 task 02).
# The first stack in the order bootstrap → foundation → data → identity → workloads.
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = "4.81.0" }
  }
}
