# stacks/bootstrap, dev. Holds no id: the state writers and readers arrive as
# TF_VAR_state_writer_object_ids and TF_VAR_state_reader_object_ids (guard G2).
environment   = "dev"
location      = "australiaeast"
unique_suffix = "k7q2" # dev's suffix, the one every dev stack uses (task 07's dev.tfvars)

# Hosted agents have no fixed IP, so the account takes public traffic and, with no IP rules,
# Entra RBAC is the only gate. A deliberate dev choice; not the prod posture.
state_public_network_access = true
