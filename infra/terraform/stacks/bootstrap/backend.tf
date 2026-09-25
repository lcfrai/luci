# Partial configuration; guard G5 keeps this block empty. The account, resource group and key
# arrive at init time: -backend-config=infra/terraform/backends/<env>.hcl plus key=value flags
# from this stack's outputs.
#
# The first apply cannot init this block, because the account it names is what this stack
# creates. Task 31 §2 writes an uncommitted backend_override.tf holding a local backend
# (git-ignored; guard G1 refuses one tracked), applies with local state, deletes the override
# and runs `init -migrate-state -force-copy -input=false` into this backend (review I2;
# -input=false alone refuses the copy, plan re-review b04 round 1).
terraform {
  backend "azurerm" {}
}
