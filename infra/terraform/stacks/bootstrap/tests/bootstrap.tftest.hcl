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
# The GUIDs are placeholders that name nobody (all ones, all twos, all threes). This is a
# .tftest.hcl, not a .tfvars, so guard G2 does not apply.

mock_provider "azurerm" {}

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
}

run "the_account_is_locked" {
  command = plan
  assert {
    condition     = azurerm_management_lock.state.lock_level == "CanNotDelete"
    error_message = "The state account has no CanNotDelete lock."
  }
}
