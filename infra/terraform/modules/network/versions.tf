# modules/network — the VNet, its subnets and NSGs, the NAT gateway, the privatelink zones
# (phase 8 task 03). Called once, by stacks/foundation (task 07).
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = "4.81.0" }
  }
}
