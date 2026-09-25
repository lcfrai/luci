# stacks/bootstrap's tests (phase 8 task 02). Mocks only: nothing here reaches Azure.
#
# validate.sh runs this file once per <env>.tfvars, with -var-file. A file-level variables
# block OVERRIDES -var-file, so it holds ONLY what no <env>.tfvars sets (guard G10, review I1):
# an `environment = "dev"` here would make the prod.tfvars pass a second dev pass.
# environment, location, unique_suffix and state_public_network_access come from the
# -var-file. Every run reads var.environment, never a literal "dev", so it holds under both
# passes. A bare `terraform test` without -var-file stops at "No value for required
# variable": intended, validate.sh is the only caller.
#
# The GUIDs are placeholders that name nobody (all zeros, ones, twos, threes). This is a
# .tftest.hcl, not a .tfvars, so guard G2 does not apply.

# The account's id is pinned to a well-formed Resource Manager id: azurerm validates the
# container's storage_account_id as one even against a mock, and a random mock string fails
# the apply runs ("parsing the StorageAccount ID").
mock_provider "azurerm" {
  mock_resource "azurerm_storage_account" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-mock/providers/Microsoft.Storage/storageAccounts/stmock"
    }
  }
}

variables {
  state_writer_object_ids = ["11111111-1111-1111-1111-111111111111"]
  state_reader_object_ids = ["22222222-2222-2222-2222-222222222222"]
}

# Plans the committed tfvars untouched: no run-level key a tfvars sets. It cannot tell which
# pass it is in (its assertions read var.environment), so guard G10, not this run, kills a
# file-level environment that turns the prod pass into a dev pass (b04 probe).
run "committed_tfvars_plan" {
  command = plan
  assert {
    condition     = azurerm_storage_account.state.name == "sttfpanaceaapp${var.environment}${var.unique_suffix}"
    error_message = "The state account name is not built from this tfvars' environment and suffix."
  }
  assert {
    condition     = azurerm_storage_account.state.public_network_access_enabled == var.state_public_network_access
    error_message = "state_public_network_access from the tfvars did not reach the account."
  }
}

run "state_account_has_no_shared_key" {
  command = plan
  assert {
    condition     = azurerm_storage_account.state.shared_access_key_enabled == false
    error_message = "The state account accepts shared keys; it must be Entra-only, with no key to leak."
  }
}

run "state_keeps_versions_and_deletes_for_30_days" {
  command = plan
  assert {
    condition     = azurerm_storage_account.state.blob_properties[0].versioning_enabled == true
    error_message = "Blob versioning is off: a bad apply that corrupts state would be a rebuild, not a restore."
  }
  assert {
    condition     = azurerm_storage_account.state.blob_properties[0].delete_retention_policy[0].days == 30
    error_message = "Deleted blobs are not kept for 30 days."
  }
  assert {
    condition     = azurerm_storage_account.state.blob_properties[0].container_delete_retention_policy[0].days == 30
    error_message = "Deleted containers are not kept for 30 days."
  }
}

run "state_name_fits_storage_rules" {
  command = plan
  assert {
    condition     = can(regex("^[a-z0-9]{3,24}$", azurerm_storage_account.state.name))
    error_message = "The state account name breaks Azure's storage rules: 3 to 24 lowercase letters and digits."
  }
  assert {
    condition     = endswith(azurerm_storage_account.state.name, var.unique_suffix)
    error_message = "The state account name does not end with unique_suffix."
  }
}

# apply, against the mocks: the container scope is built from the account's computed id.
run "every_writer_gets_blob_contributor" {
  variables {
    state_writer_object_ids = ["11111111-1111-1111-1111-111111111111", "33333333-3333-3333-3333-333333333333"]
  }
  assert {
    condition     = length(azurerm_role_assignment.state_writer) == 2
    error_message = "Two writers in did not give two state_writer role assignments."
  }
  assert {
    condition     = toset([for a in azurerm_role_assignment.state_writer : a.principal_id]) == toset(var.state_writer_object_ids)
    error_message = "The state_writer assignments are not one per writer id."
  }
  assert {
    condition     = alltrue([for a in azurerm_role_assignment.state_writer : a.role_definition_name == "Storage Blob Data Contributor"])
    error_message = "A state writer does not hold Storage Blob Data Contributor."
  }
  assert {
    condition     = alltrue([for a in azurerm_role_assignment.state_writer : a.scope == "${azurerm_storage_account.state.id}/blobServices/default/containers/tfstate"])
    error_message = "A state writer's role is not scoped to the tfstate container."
  }
}

run "the_plan_identity_only_reads_state" {
  assert {
    condition     = length(azurerm_role_assignment.state_reader) == 1
    error_message = "One reader in did not give one state_reader role assignment."
  }
  assert {
    condition     = azurerm_role_assignment.state_reader["22222222-2222-2222-2222-222222222222"].role_definition_name == "Storage Blob Data Reader"
    error_message = "The plan identity holds more than Storage Blob Data Reader, so a PR build could rewrite state (review C3)."
  }
  assert {
    condition     = azurerm_role_assignment.state_reader["22222222-2222-2222-2222-222222222222"].scope == "${azurerm_storage_account.state.id}/blobServices/default/containers/tfstate"
    error_message = "The plan identity's role is not scoped to the tfstate container."
  }
}

run "a_reader_that_is_also_a_writer_is_rejected" {
  command = plan
  variables {
    state_reader_object_ids = ["11111111-1111-1111-1111-111111111111"]
  }
  expect_failures = [var.state_reader_object_ids]
}

run "the_product_group_exists_before_foundation" {
  command = plan
  assert {
    condition     = azurerm_resource_group.product.name == "rg-panacea-app-${var.environment}"
    error_message = "bootstrap does not create rg-panacea-app-<env>, so the infra identity's Contributor would need the subscription (review I8)."
  }
}

run "no_writers_is_rejected" {
  command = plan
  variables {
    state_writer_object_ids = []
  }
  expect_failures = [var.state_writer_object_ids]
}

run "bad_suffix_is_rejected" {
  command = plan
  variables {
    unique_suffix = "K7Q2"
  }
  expect_failures = [var.unique_suffix]
}

run "ip_rules_turn_default_deny_on" {
  command = plan
  variables {
    state_ip_rules = ["203.0.113.0/24"]
  }
  assert {
    condition     = azurerm_storage_account.state.network_rules[0].default_action == "Deny"
    error_message = "IP rules were given but the account firewall does not deny by default."
  }
  assert {
    condition     = azurerm_storage_account.state.network_rules[0].ip_rules == toset(["203.0.113.0/24"])
    error_message = "The IP rules did not reach the firewall: Deny with no rules locks every caller out."
  }
}

# apply: the lock's scope is the account's computed id (as built: plan read the level only).
run "the_account_is_locked" {
  assert {
    condition     = azurerm_management_lock.state.lock_level == "CanNotDelete"
    error_message = "The state account has no CanNotDelete lock."
  }
  assert {
    condition     = azurerm_management_lock.state.scope == azurerm_storage_account.state.id
    error_message = "The CanNotDelete lock is not on the state account. On a resource group it would block every replace foundation plans."
  }
}

# ---- Added as built (task 02): the mutants the twelve runs above let live. ----------------

run "the_state_account_is_hardened" {
  command = plan
  assert {
    condition     = azurerm_storage_account.state.account_tier == "Standard" && azurerm_storage_account.state.account_replication_type == "ZRS"
    error_message = "The state account is not Standard ZRS."
  }
  assert {
    condition     = azurerm_storage_account.state.min_tls_version == "TLS1_2" && azurerm_storage_account.state.https_traffic_only_enabled == true
    error_message = "The state account accepts plain HTTP or TLS older than 1.2."
  }
  assert {
    condition     = azurerm_storage_account.state.default_to_oauth_authentication == true
    error_message = "The portal does not default to Entra for the state account."
  }
  assert {
    condition     = azurerm_storage_account.state.allow_nested_items_to_be_public == false && azurerm_storage_container.tfstate.container_access_type == "private"
    error_message = "State can be made public: the account allows public containers, or tfstate is not private."
  }
  assert {
    condition     = azurerm_storage_account.state.infrastructure_encryption_enabled == true
    error_message = "The state account has no infrastructure (double) encryption."
  }
  assert {
    condition     = azurerm_storage_account.state.cross_tenant_replication_enabled == false
    error_message = "The state account can be replicated to another tenant."
  }
}

# Both committed tfvars set true today, so committed_tfvars_plan cannot tell a hard-coded true
# from the variable (predicted survivor S1). This is prod's one planned change (task 30).
run "public_access_off_reaches_the_account" {
  command = plan
  variables {
    state_public_network_access = false
  }
  assert {
    condition     = azurerm_storage_account.state.public_network_access_enabled == false
    error_message = "state_public_network_access = false did not turn the account's public access off."
  }
}

run "no_ip_rules_leave_entra_as_the_only_gate" {
  command = plan
  variables {
    state_ip_rules = []
  }
  assert {
    condition     = azurerm_storage_account.state.network_rules[0].default_action == "Allow"
    error_message = "With no IP rules the firewall denies by default, which locks every caller out."
  }
}

run "a_bad_environment_is_rejected" {
  command = plan
  variables {
    environment = "test"
  }
  expect_failures = [var.environment]
}

# An extra tag passes through; a clashing one loses to the stack's own (README, Conventions).
run "state_lives_in_its_own_tagged_group" {
  command = plan
  variables {
    tags = { cost-centre = "test", stack = "overridden" }
  }
  assert {
    condition     = azurerm_resource_group.state.name == "rg-panacea-app-${var.environment}-tfstate" && azurerm_storage_account.state.resource_group_name == azurerm_resource_group.state.name
    error_message = "The state account is not in its own rg-panacea-app-<env>-tfstate group."
  }
  assert {
    condition     = alltrue([for l in [azurerm_resource_group.state.location, azurerm_resource_group.product.location, azurerm_storage_account.state.location] : l == var.location])
    error_message = "A bootstrap resource is not in var.location."
  }
  assert {
    condition = alltrue([for t in [azurerm_resource_group.state.tags, azurerm_resource_group.product.tags, azurerm_storage_account.state.tags] : t == tomap({
      cost-centre = "test", product = "panacea-app", environment = var.environment, stack = "bootstrap", managed-by = "terraform"
    })])
    error_message = "A bootstrap resource's tags are not var.tags under the stack's own four."
  }
}

# apply: product_resource_group_id is computed.
run "the_outputs_name_what_was_built" {
  assert {
    condition     = output.state_resource_group_name == azurerm_resource_group.state.name && output.state_storage_account_name == azurerm_storage_account.state.name
    error_message = "The state group or account output does not name what bootstrap built."
  }
  assert {
    condition     = output.state_container_name == "tfstate" && output.state_container_name == azurerm_storage_container.tfstate.name
    error_message = "state_container_name is not the tfstate container."
  }
  assert {
    condition     = output.product_resource_group_id == azurerm_resource_group.product.id
    error_message = "product_resource_group_id is not the product group's id."
  }
}

# backends/<env>.hcl, read as text: each tfvars pass checks its own environment's file.
run "the_backend_file_names_this_container" {
  command = plan
  assert {
    condition     = try(regex("(?m)^container_name\\s*=\\s*\"([^\"]*)\"\\s*$", file("${path.module}/../../backends/${var.environment}.hcl"))[0], "") == azurerm_storage_container.tfstate.name
    error_message = "backends/<env>.hcl does not name the container bootstrap creates."
  }
  assert {
    condition     = can(regex("(?m)^use_azuread_auth\\s*=\\s*true\\s*$", file("${path.module}/../../backends/${var.environment}.hcl")))
    error_message = "backends/<env>.hcl does not set use_azuread_auth = true; the account has no shared key."
  }
  assert {
    condition     = !can(regex("(?m)^\\s*(storage_account_name|resource_group_name|key|access_key|sas_token|subscription_id|tenant_id|client_id)\\s*=", file("${path.module}/../../backends/${var.environment}.hcl")))
    error_message = "backends/<env>.hcl holds an identity or a credential; those arrive as -backend-config flags."
  }
}
