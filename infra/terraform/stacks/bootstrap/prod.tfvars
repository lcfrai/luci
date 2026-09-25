# stacks/bootstrap, prod. Holds no id: the state writers and readers arrive as
# TF_VAR_state_writer_object_ids and TF_VAR_state_reader_object_ids (guard G2).
environment   = "prod"
location      = "australiaeast"
unique_suffix = "r4nv" # prod's suffix, chosen here in task 02; every prod stack uses it

# true until task 31's prod section (carried from task 02 review I4): once foundation has made
# the VNet and the in-VNet pool, a bootstrap pass with TF_VAR_state_private_endpoint adds the
# account's private endpoint; once the pool resolves the account to that endpoint, this flips
# to false. Endpoint first, flip second: false without the endpoint is refused at plan.
state_public_network_access = true
