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
# The GUIDs are placeholders that name nobody (all zeros, all ones through all sixes, and one
# mixed-case abcdef00-... id for the case run). This is a .tftest.hcl, not a .tfvars, so guard
# G2 does not apply. The private endpoint's subnet
# and zone ids below are well-formed placeholders in the all-zeros subscription.
#
# No file-level state_private_endpoint: its default, null, is dev's posture. If prod.tfvars
# ever sets state_public_network_access = false, every prod-pass run without an endpoint meets
# that variable's validation, so that change adds a placeholder endpoint here as well (no
# tfvars sets the key, so guard G10 allows it; task 02 fix round 1, carried to task 31). The
# one run that needs no endpoint, public_access_off_needs_a_private_endpoint, sets
# state_private_endpoint = null itself, so that placeholder cannot reach it (task 02 fix round
# 2, re-review m1: without the null, the flip commit fails validate.sh in both passes).

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

# Two readers, and the principals read back (task 02 review m1): one reader could not tell a
# for_each over the first id only, nor the Reader role granted to a writer.
run "the_plan_identity_only_reads_state" {
  variables {
    state_reader_object_ids = ["22222222-2222-2222-2222-222222222222", "44444444-4444-4444-4444-444444444444"]
  }
  assert {
    condition     = length(azurerm_role_assignment.state_reader) == 2
    error_message = "Two readers in did not give two state_reader role assignments."
  }
  assert {
    condition     = toset([for a in azurerm_role_assignment.state_reader : a.principal_id]) == toset(var.state_reader_object_ids)
    error_message = "The state_reader assignments are not one per reader id."
  }
  assert {
    condition     = alltrue([for a in azurerm_role_assignment.state_reader : a.role_definition_name == "Storage Blob Data Reader"])
    error_message = "The plan identity holds more than Storage Blob Data Reader, so a PR build could rewrite state (review C3)."
  }
  assert {
    condition     = alltrue([for a in azurerm_role_assignment.state_reader : a.scope == "${azurerm_storage_account.state.id}/blobServices/default/containers/tfstate"])
    error_message = "The plan identity's role is not scoped to the tfstate container."
  }
}

# The stack's one security invariant (review C3): the plan identity is never a writer. The
# shared id sits in the middle of both lists and at a different index in each (task 02 review
# I1), so a check narrowed to the first or last writer, the first or last reader, or pairs at
# the same index finds no overlap. The Deploy identities table gives two writers, so the plan
# identity listed after the first is the realistic mistake.
run "a_reader_that_is_also_a_writer_is_rejected" {
  command = plan
  variables {
    state_writer_object_ids = ["11111111-1111-1111-1111-111111111111", "33333333-3333-3333-3333-333333333333", "44444444-4444-4444-4444-444444444444"]
    state_reader_object_ids = ["22222222-2222-2222-2222-222222222222", "55555555-5555-5555-5555-555555555555", "33333333-3333-3333-3333-333333333333", "66666666-6666-6666-6666-666666666666"]
  }
  expect_failures = [var.state_reader_object_ids]
}

# The ends of both lists (task 02 re-review m2). The mid-list run above cannot tell a check that
# drops the first writer or the last reader (RS1, RS1b), nor one that drops the last writer or
# the first reader (RS1c, RS1d), so the shared id sits at each pair of ends in turn.
run "a_reader_that_is_the_first_writer_is_rejected" {
  command = plan
  variables {
    state_writer_object_ids = ["33333333-3333-3333-3333-333333333333", "11111111-1111-1111-1111-111111111111"]
    state_reader_object_ids = ["22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"]
  }
  expect_failures = [var.state_reader_object_ids]
}

run "a_reader_that_is_the_last_writer_is_rejected" {
  command = plan
  variables {
    state_writer_object_ids = ["11111111-1111-1111-1111-111111111111", "33333333-3333-3333-3333-333333333333"]
    state_reader_object_ids = ["33333333-3333-3333-3333-333333333333", "22222222-2222-2222-2222-222222222222"]
  }
  expect_failures = [var.state_reader_object_ids]
}

# An Entra object id is a GUID, which compares without case; setintersection compares with it.
# The same id typed in two cases is one principal, given both roles (task 02 fix round 2). The
# case differs within each list too, so folding only one list still finds no overlap.
run "a_reader_that_is_a_writer_in_another_case_is_rejected" {
  command = plan
  variables {
    state_writer_object_ids = ["11111111-1111-1111-1111-111111111111", "aBcDeF00-0000-0000-0000-00000000000a"]
    state_reader_object_ids = ["AbCdEf00-0000-0000-0000-00000000000A"]
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

# Exactly four (README, Conventions: `^[a-z0-9]{4}$`, one suffix per environment for every
# stack's globally unique names). Round 2 survivor X1, `{4,6}`, lived through the upper-case
# run above, which has the right length.
run "a_five_character_suffix_is_rejected" {
  command = plan
  variables {
    unique_suffix = "k7q2x"
  }
  expect_failures = [var.unique_suffix]
}

run "a_three_character_suffix_is_rejected" {
  command = plan
  variables {
    unique_suffix = "k7q"
  }
  expect_failures = [var.unique_suffix]
}

# Two ranges (task 02 review m2): one could not tell every rule from the first rule only.
run "ip_rules_turn_default_deny_on" {
  command = plan
  variables {
    state_ip_rules = ["203.0.113.0/24", "198.51.100.0/24"]
  }
  assert {
    condition     = azurerm_storage_account.state.network_rules[0].default_action == "Deny"
    error_message = "IP rules were given but the account firewall does not deny by default."
  }
  assert {
    condition     = azurerm_storage_account.state.network_rules[0].ip_rules == toset(["203.0.113.0/24", "198.51.100.0/24"])
    error_message = "The IP rules did not all reach the firewall: Deny with a rule missing locks that caller out."
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
  # The part asks for notes saying why (task 02 review m2). can() is false for null notes too.
  assert {
    condition     = can(regex("(?i)terraform state", azurerm_management_lock.state.notes))
    error_message = "The lock's notes do not say it guards the Terraform state; whoever meets the lock is told nothing."
  }
  # Fix round 2's predicted survivor LN1: a name hard-coded to one environment.
  assert {
    condition     = azurerm_management_lock.state.name == "lock-panacea-app-${var.environment}-tfstate"
    error_message = "The state account's lock is not named for this environment."
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
# from the variable (predicted survivor S1). Public access off needs the private endpoint
# (task 02 review I4), so this run gives one.
run "public_access_off_reaches_the_account" {
  command = plan
  variables {
    state_public_network_access = false
    state_private_endpoint = {
      subnet_id           = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-mock/providers/Microsoft.Network/virtualNetworks/vnet-mock/subnets/private_endpoints"
      private_dns_zone_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-mock/providers/Microsoft.Network/privateDnsZones/privatelink.blob.core.windows.net"
    }
  }
  assert {
    condition     = azurerm_storage_account.state.public_network_access_enabled == false
    error_message = "state_public_network_access = false did not turn the account's public access off."
  }
}

# Task 02 review I4: public access off with no private endpoint locks every caller out, the
# in-VNet pool included, and state is then reachable by nobody. The validation refuses it.
run "public_access_off_needs_a_private_endpoint" {
  command = plan
  variables {
    state_public_network_access = false
    state_private_endpoint      = null
  }
  expect_failures = [var.state_public_network_access]
}

# null is the default: dev's posture, and prod's until task 31's prod section. Both keys set here
# so the run means the same if prod.tfvars later sets public access off.
run "no_private_endpoint_unless_one_is_given" {
  command = plan
  variables {
    state_public_network_access = true
    state_private_endpoint      = null
  }
  assert {
    condition     = length(azurerm_private_endpoint.state) == 0
    error_message = "bootstrap made a private endpoint nobody asked for."
  }
}

# apply: the connection's target is the account's computed id. The endpoint sits in the state
# group, on the subnet and blob zone given (foundation's subnet_ids.private_endpoints and
# private_dns_zone_ids.blob), auto-approved: the operator who applies bootstrap owns the account.
# Public access stays ON here: endpoint first, flip second, so the endpoint must be made while
# public access is still on (fix round 1's predicted survivor PC1 made it only once off). The
# location is one no tfvars uses, or a hard-coded australiaeast agrees with both (PL1).
run "a_private_endpoint_puts_state_on_the_vnet" {
  variables {
    state_public_network_access = true
    location                    = "australiasoutheast"
    state_private_endpoint = {
      subnet_id           = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-mock/providers/Microsoft.Network/virtualNetworks/vnet-mock/subnets/private_endpoints"
      private_dns_zone_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-mock/providers/Microsoft.Network/privateDnsZones/privatelink.blob.core.windows.net"
    }
  }
  assert {
    condition     = length(azurerm_private_endpoint.state) == 1
    error_message = "state_private_endpoint was given but bootstrap made no private endpoint."
  }
  assert {
    condition     = azurerm_private_endpoint.state[0].subnet_id == var.state_private_endpoint.subnet_id && azurerm_private_endpoint.state[0].resource_group_name == azurerm_resource_group.state.name && azurerm_private_endpoint.state[0].location == var.location
    error_message = "The endpoint is not on the given subnet, in the state group, in var.location."
  }
  assert {
    condition     = azurerm_private_endpoint.state[0].private_service_connection[0].private_connection_resource_id == azurerm_storage_account.state.id && toset(azurerm_private_endpoint.state[0].private_service_connection[0].subresource_names) == toset(["blob"])
    error_message = "The endpoint does not connect to the state account's blob service."
  }
  assert {
    condition     = azurerm_private_endpoint.state[0].private_service_connection[0].is_manual_connection == false
    error_message = "The endpoint waits for a manual approval, so state stays unreachable after the flip."
  }
  assert {
    condition     = toset(azurerm_private_endpoint.state[0].private_dns_zone_group[0].private_dns_zone_ids) == toset([var.state_private_endpoint.private_dns_zone_id])
    error_message = "The endpoint writes no record in the blob zone, so the pool resolves the account to its public address."
  }
  assert {
    condition     = azurerm_private_endpoint.state[0].tags == azurerm_storage_account.state.tags
    error_message = "The endpoint does not carry the stack's tags."
  }
  # Task 02 re-review m5 (PNM): a name hard-coded to one environment dies in the other's pass.
  assert {
    condition     = azurerm_private_endpoint.state[0].name == "pe-panacea-app-${var.environment}-tfstate" && azurerm_private_endpoint.state[0].private_service_connection[0].name == "psc-panacea-app-${var.environment}-tfstate"
    error_message = "The endpoint or its connection is not named for this environment."
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
# The location is one no tfvars uses: both say australiaeast, so under them a location
# hard-coded to it passes (round 2 survivor L1).
run "state_lives_in_its_own_tagged_group" {
  command = plan
  variables {
    tags     = { cost-centre = "test", stack = "overridden" }
    location = "australiasoutheast"
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
  # An allow-list, not a deny-list (task 02 review I2): a list of refused keys let
  # client_secret, oidc_token and every other credential it did not name through. Every line is
  # blank, a # comment, or one of the two settings, whole. Anything else fails, a // or /* */
  # comment included: this file is config, and free text is the secret scan's to read (task
  # 28's gitleaks over every PR's commits).
  assert {
    condition = alltrue([
      for l in split("\n", replace(file("${path.module}/../../backends/${var.environment}.hcl"), "\r", "")) :
      can(regex("^(\\s*(#.*)?|container_name\\s*=\\s*\"[a-z0-9-]*\"\\s*|use_azuread_auth\\s*=\\s*true\\s*)$", l))
    ])
    error_message = "backends/<env>.hcl holds a line other than a # comment, container_name or use_azuread_auth = true. An identity arrives as a -backend-config flag and a credential never: this file is committed."
  }
}

# ---- Added in fix round 1 (task 02 review I3): three choices the runs above let live. -------

# providers.tf, read as text. Provider configuration is not an object a run can assert on, but
# its file is (the technique the backend run uses). Each line exactly once, whole, never
# commented out. This pins the source, not what azurerm does with it. Block comments are
# stripped before counting: a line regex reads a line inside /* */ as live (task 02 re-review
# m3, RS2). Terraform's regex form takes the last / as its delimiter, hence the escaped \\/. One
# azurerm block only: with a second, aliased one, the lines could sit there while the default
# provider, the one every resource uses, has neither (task 02 fix round 2, PA1).
run "the_provider_is_entra_only_and_registers_nothing" {
  command = plan
  assert {
    condition     = length(regexall("(?m)^\\s*storage_use_azuread\\s*=\\s*true\\s*$", replace(file("${path.module}/providers.tf"), "/(?s)/\\*.*?\\*\\//", ""))) == 1
    error_message = "providers.tf does not set storage_use_azuread = true once, outside a comment: the account has no shared key, so data-plane calls must use Entra."
  }
  assert {
    condition     = length(regexall("(?m)^\\s*resource_provider_registrations\\s*=\\s*\"none\"\\s*$", replace(file("${path.module}/providers.tf"), "/(?s)/\\*.*?\\*\\//", ""))) == 1
    error_message = "providers.tf does not set resource_provider_registrations = \"none\" once, outside a comment: the read-only plan identity cannot register providers (review C3)."
  }
  assert {
    condition     = length(regexall("(?m)^\\s*provider\\s+\"azurerm\"", replace(file("${path.module}/providers.tf"), "/(?s)/\\*.*?\\*\\//", ""))) == 1
    error_message = "providers.tf does not hold exactly one azurerm provider block, so the two lines above may not be the default provider's."
  }
}

# apply: the account id is computed. storage_account_id keeps the container on Resource
# Manager; the deprecated storage_account_name goes through the data plane.
run "the_container_is_made_through_resource_manager" {
  assert {
    condition     = azurerm_storage_container.tfstate.storage_account_id == azurerm_storage_account.state.id
    error_message = "The tfstate container is not made by storage_account_id, so managing it needs data-plane reach to the account."
  }
}

# ---- Added in fix round 2 (task 02 re-review I1). -----------------------------------------------

# README's stack order says an earlier stack never reads a later one, yet state_private_endpoint
# takes two of foundation's ids. So the exception is declared beside the rule, in that section
# (HTML comments stripped: a reader never sees them), and it stays what it declares: the ids
# arrive by hand, and no bootstrap file reads terraform_remote_state.
run "the_one_input_from_a_later_stack_is_declared" {
  command = plan
  assert {
    condition     = strcontains(try(regex("(?s)\n## Stack order\n(.*?)\n## ", replace(replace(file("${path.module}/../../README.md"), "\r", ""), "/(?s)<!--.*?-->/", ""))[0], ""), "`state_private_endpoint`")
    error_message = "infra/terraform/README.md's Stack order section does not declare state_private_endpoint, the one input bootstrap takes from a later stack, so the rule reads as unbroken."
  }
  # A data block, not the word: variables.tf's description says "never through
  # terraform_remote_state", and a bare-word check fails on that sentence.
  assert {
    condition     = alltrue([for f in fileset(path.module, "*.tf") : length(regexall("(?m)^\\s*data\\s+\"terraform_remote_state\"", file("${path.module}/${f}"))) == 0])
    error_message = "A bootstrap file declares a terraform_remote_state data source: the declared exception is ids copied by hand on an operator pass, never a read of a later stack's state."
  }
}
