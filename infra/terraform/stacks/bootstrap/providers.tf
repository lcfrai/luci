provider "azurerm" {
  features {}
  # The state account has no shared key, so every data-plane call authenticates with Entra.
  storage_use_azuread = true
  # The read-only plan identity cannot register resource providers; task 31 §1 registers them
  # by hand (review C3). The attribute name was verified by `terraform validate` against
  # azurerm 4.81.0 (task 02) — that probe predates OD-32 and was run on Terraform 1.15.8; the
  # attribute belongs to the provider, not the engine, so it is expected to hold under
  # `tofu validate`, which has not been run (no tofu binary on the build host).
  resource_provider_registrations = "none"
}
