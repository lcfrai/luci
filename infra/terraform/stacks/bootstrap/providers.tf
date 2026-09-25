provider "azurerm" {
  features {}
  # The state account has no shared key, so every data-plane call authenticates with Entra.
  storage_use_azuread = true
  # The read-only plan identity cannot register resource providers; task 31 §1 registers them
  # by hand (review C3). The attribute name is verified by `terraform validate` against
  # azurerm 4.81.0 (task 02).
  resource_provider_registrations = "none"
}
