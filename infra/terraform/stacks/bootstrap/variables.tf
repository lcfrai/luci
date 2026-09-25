variable "environment" {
  type        = string
  description = "Environment name. Selects nothing by itself; every name and tag carries it."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}

variable "location" {
  type        = string
  description = "Azure region, e.g. australiaeast. Every stack's tfvars must set the same value; nothing checks it across stacks here (task 07 is to check the product group's location, carried from task 02 review m3)."
}

variable "unique_suffix" {
  type        = string
  description = "Four lowercase alphanumerics appended to globally unique names. Chosen once per environment."
  validation {
    condition     = can(regex("^[a-z0-9]{4}$", var.unique_suffix))
    error_message = "unique_suffix must match ^[a-z0-9]{4}$."
  }
}

variable "state_writer_object_ids" {
  type        = list(string)
  description = "Entra object ids granted Storage Blob Data Contributor on the state container: the infra identity and the owner's break-glass group (the plan's Deploy identities table). Never the plan identity. Supplied as TF_VAR_state_writer_object_ids, never in tfvars."
  validation {
    condition     = length(var.state_writer_object_ids) > 0
    error_message = "At least one principal must be able to write state, or the next stack cannot init."
  }
}

variable "state_reader_object_ids" {
  type        = list(string)
  description = "Entra object ids granted Storage Blob Data Reader on the state container: the plan identity, which PR builds use (review C3). Supplied as TF_VAR_state_reader_object_ids, never in tfvars."
  validation {
    # Compared without case: an object id is a GUID, and one typed in two cases is one principal
    # (task 02 fix round 2).
    condition     = length(setintersection([for id in var.state_reader_object_ids : lower(id)], [for id in var.state_writer_object_ids : lower(id)])) == 0
    error_message = "A state reader must not also be a writer (object ids compared without case): the plan identity is read-only by construction."
  }
}

variable "state_public_network_access" {
  type        = bool
  description = "Whether the state account accepts traffic from outside its private endpoint. true in dev (hosted agents have no fixed IP), and in prod until task 31's prod section: a bootstrap pass that adds state_private_endpoint first, then this set to false (carried from task 02 review I4)."
  validation {
    condition     = var.state_public_network_access || var.state_private_endpoint != null
    error_message = "state_public_network_access = false needs state_private_endpoint: with no endpoint every caller, the in-VNet pool included, is locked out of state."
  }
}

variable "state_private_endpoint" {
  type = object({
    subnet_id           = string
    private_dns_zone_id = string
  })
  description = "The state account's private endpoint, on foundation's subnet_ids.private_endpoints and private_dns_zone_ids.blob: the one input bootstrap takes from a later stack, declared in infra/terraform/README.md (Stack order). null (the default) makes none: dev's posture, and prod's until task 31's prod section. Supplied as TF_VAR_state_private_endpoint by the operator, never in tfvars (the ids carry the subscription id, guard G2) and never through terraform_remote_state."
  default     = null
}

variable "state_ip_rules" {
  type        = list(string)
  description = "CIDRs allowed through the account firewall when public access is on. Empty means Entra RBAC is the only gate."
  default     = []
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged into local.tags."
  default     = {}
}
