locals {
  tags = merge(var.tags, {
    product     = "panacea-app"
    environment = var.environment
    stack       = "bootstrap"
    managed-by  = "terraform"
  })

  # The tfstate container's Resource Manager id: the account id + the container's path. Both
  # state roles are scoped here, to the one container, never to the account.
  state_container_scope = "${azurerm_storage_account.state.id}/blobServices/default/containers/${azurerm_storage_container.tfstate.name}"
}

resource "azurerm_resource_group" "state" {
  name     = "rg-panacea-app-${var.environment}-tfstate"
  location = var.location
  tags     = local.tags
}

# The product's own group. Bootstrap creates it, not foundation, so it exists before the infra
# identity's first apply and the owner can scope that identity's Contributor to this group
# instead of the subscription (the plan's Deploy identities table, review I8). foundation reads
# it with a data source (task 07). No management lock: a lock on a resource group is inherited
# by everything in it, and would block every replace foundation plans.
resource "azurerm_resource_group" "product" {
  name     = "rg-panacea-app-${var.environment}"
  location = var.location
  tags     = local.tags
}

# Entra authentication only: no shared key, so there is no account key to leak. Versions and
# soft-deleted blobs and containers are kept for 30 days, so a bad apply that corrupts state is
# a restore, not a rebuild.
resource "azurerm_storage_account" "state" {
  name                              = "sttfpanaceaapp${var.environment}${var.unique_suffix}"
  resource_group_name               = azurerm_resource_group.state.name
  location                          = azurerm_resource_group.state.location
  account_tier                      = "Standard"
  account_replication_type          = "ZRS"
  min_tls_version                   = "TLS1_2"
  https_traffic_only_enabled        = true
  shared_access_key_enabled         = false
  default_to_oauth_authentication   = true
  allow_nested_items_to_be_public   = false
  infrastructure_encryption_enabled = true
  cross_tenant_replication_enabled  = false
  public_network_access_enabled     = var.state_public_network_access
  tags                              = local.tags

  # No IP rules: allow, and Entra RBAC is the only gate (the dev posture, not prod's: prod goes
  # private through azurerm_private_endpoint.state, below).
  network_rules {
    default_action = length(var.state_ip_rules) > 0 ? "Deny" : "Allow"
    ip_rules       = var.state_ip_rules
  }

  blob_properties {
    versioning_enabled = true
    delete_retention_policy {
      days = 30
    }
    container_delete_retention_policy {
      days = 30
    }
  }
}

# storage_account_id, not the deprecated storage_account_name: a Resource Manager call, which
# needs no data-plane access to an account without shared keys.
resource "azurerm_storage_container" "tfstate" {
  name                  = "tfstate"
  storage_account_id    = azurerm_storage_account.state.id
  container_access_type = "private"
}

# The infra identity and the owner's break-glass group: they apply, so they take the state
# lease and write the blob. Never the plan identity (the reader validation refuses one).
resource "azurerm_role_assignment" "state_writer" {
  for_each             = toset(var.state_writer_object_ids)
  scope                = local.state_container_scope
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = each.value
}

# The plan identity, which PR builds use: read only. With -lock=false (task 30) a plan takes
# no blob lease, so read is enough, and a pull request cannot rewrite state (review C3).
resource "azurerm_role_assignment" "state_reader" {
  for_each             = toset(var.state_reader_object_ids)
  scope                = local.state_container_scope
  role_definition_name = "Storage Blob Data Reader"
  principal_id         = each.value
}

# The account's private endpoint, made only when state_private_endpoint is given: never in dev
# today, and in prod by the bootstrap pass task 31's prod section runs after foundation, before
# public access goes off (carried from task 02 review I4). It lives here, not in foundation:
# the operator who applies bootstrap owns the account, so the connection is auto-approved and
# no pipeline identity gains a right on the state group; and the endpoint is not in
# foundation's state, so no foundation plan can destroy the one path to the state it writes.
resource "azurerm_private_endpoint" "state" {
  count               = var.state_private_endpoint == null ? 0 : 1
  name                = "pe-panacea-app-${var.environment}-tfstate"
  resource_group_name = azurerm_resource_group.state.name
  location            = azurerm_resource_group.state.location
  subnet_id           = var.state_private_endpoint.subnet_id
  tags                = local.tags

  private_service_connection {
    name                           = "psc-panacea-app-${var.environment}-tfstate"
    private_connection_resource_id = azurerm_storage_account.state.id
    subresource_names              = ["blob"]
    is_manual_connection           = false
  }

  private_dns_zone_group {
    name                 = "blob"
    private_dns_zone_ids = [var.state_private_endpoint.private_dns_zone_id]
  }
}

resource "azurerm_management_lock" "state" {
  name       = "lock-panacea-app-${var.environment}-tfstate"
  scope      = azurerm_storage_account.state.id
  lock_level = "CanNotDelete"
  notes      = "Holds the Terraform state of every stack in this environment. Versions and soft delete do not survive deleting the account itself, so it cannot be deleted while this lock stands. Remove the lock by hand, deliberately."
}
