terraform {
  required_version = ">= 1.9.0"
  # BYO selection supplies an existing durable Blob backend with Azure AD auth.
  # Terraform state must survive loss of the operator's setup machine.
  backend "azurerm" {}
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.81"
    }
  }
}

provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}
