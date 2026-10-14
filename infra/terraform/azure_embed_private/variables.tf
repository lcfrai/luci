variable "location" { type = string }
variable "resource_group_name" { type = string }
variable "name" { type = string }
variable "zone" { type = string }
variable "runtime_vm_size" { type = string }
variable "controller_vm_size" { type = string }
variable "runtime_image_version_id" {
  type = string
  validation {
    condition     = can(regex("^/subscriptions/[^/]+/resourcegroups/[^/]+/providers/microsoft.compute/galleries/[^/]+/images/[^/]+/versions/[^/]+$", lower(var.runtime_image_version_id)))
    error_message = "Runtime image must be an exact Azure Compute Gallery image version ARM ID."
  }
}
variable "controller_image_version_id" {
  type = string
  validation {
    condition     = can(regex("^/subscriptions/[^/]+/resourcegroups/[^/]+/providers/microsoft.compute/galleries/[^/]+/images/[^/]+/versions/[^/]+$", lower(var.controller_image_version_id)))
    error_message = "Controller image must be an exact Azure Compute Gallery image version ARM ID."
  }
}
variable "runtime_package_sha256" {
  type = string
  validation {
    condition     = can(regex("^[0-9a-f]{64}$", var.runtime_package_sha256))
    error_message = "The runtime host package must be pinned by SHA-256."
  }
}
variable "scope" {
  type = string
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{1,127}$", var.scope))
    error_message = "Scope must match the selected controller deployment scope."
  }
}
variable "generation" {
  type = string
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{1,127}$", var.generation))
    error_message = "Gateway generation must match the selected controller generation."
  }
}
variable "previous_generation" {
  type    = string
  default = null
}
variable "runtime_subnet_cidr" {
  type = string
  validation {
    condition     = can(regex("^(10\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.|192\\.168\\.)", var.runtime_subnet_cidr)) && can(cidrnetmask(var.runtime_subnet_cidr))
    error_message = "Runtime subnet must be an IPv4 RFC1918 CIDR."
  }
}
variable "controller_subnet_cidr" {
  type = string
  validation {
    condition     = can(regex("^(10\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.|192\\.168\\.)", var.controller_subnet_cidr)) && can(cidrnetmask(var.controller_subnet_cidr))
    error_message = "Controller subnet must be an IPv4 RFC1918 CIDR."
  }
}
variable "controller_private_ip" {
  type = string
  validation {
    condition     = can(regex("^(10\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.|192\\.168\\.)", var.controller_private_ip))
    error_message = "Controller address must be private IPv4."
  }
}
variable "gateway_private_ip" {
  type = string
  validation {
    condition     = can(regex("^(10\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.|192\\.168\\.)", var.gateway_private_ip))
    error_message = "Gateway address must be private IPv4."
  }
}
variable "gateway_zone_name" { type = string }
variable "controller_admin_ssh_public_key" { type = string }
variable "runtime_admin_ssh_public_key" { type = string }
variable "controller_disk_size_gb" {
  type = number
  validation {
    condition     = var.controller_disk_size_gb >= 64 && floor(var.controller_disk_size_gb) == var.controller_disk_size_gb
    error_message = "Authoritative controller disk size must be an integer of at least 64 GiB."
  }
}
variable "key_vault_id" { type = string }
variable "key_vault_name" { type = string }
variable "host_secret_ids" {
  description = "Exact Key Vault secret ARM IDs for the host identity; no values."
  type        = set(string)
}
variable "host_secret_refs" {
  description = "Nonsecret Key Vault names and exact versions for teamApiKey, adminToken, sandboxTokenSeed, gatewayServerCert, gatewayServerKey, gatewayClientCa."
  type = map(object({
    name    = string
    version = string
  }))
  validation {
    condition = length(var.host_secret_refs) == 6 && alltrue([
      for key in ["teamApiKey", "adminToken", "sandboxTokenSeed", "gatewayServerCert", "gatewayServerKey", "gatewayClientCa"] :
      contains(keys(var.host_secret_refs), key)
    ]) && alltrue([for ref in values(var.host_secret_refs) : can(regex("^[0-9a-f]{32}$", ref.version))])
    error_message = "Host secrets require six named exact-version Key Vault references."
  }
}
variable "controller_secret_ids" {
  description = "Exact Key Vault secret ARM IDs for the controller identity; no values."
  type        = set(string)
}
variable "tags" {
  type    = map(string)
  default = {}
}
