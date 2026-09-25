# stacks/bootstrap, prod. Holds no id: the state writers and readers arrive as
# TF_VAR_state_writer_object_ids and TF_VAR_state_reader_object_ids (guard G2).
environment   = "prod"
location      = "australiaeast"
unique_suffix = "r4nv" # prod's suffix, chosen here in task 02; every prod stack uses it

# true until the in-VNet agent pool exists: then task 30's runbook flips this to false. That
# flip is the one planned change to this stack.
state_public_network_access = true
