variable "environment" {
  type        = string
  description = "dev or prod; used in names."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}

variable "location" {
  type        = string
  description = "Azure region for every resource in this module."
}

variable "resource_group_name" {
  type        = string
  description = "The foundation resource group."
}

# cidrnetmask, not cidrhost: cidrhost takes an IPv6 prefix too, and "fd00::/16" would pass a
# /16 test. The cidrsubnet(…, 0, 0) round trip refuses host bits ("10.40.1.0/16"), which
# Azure would refuse only at apply (task 03 as built).
variable "vnet_cidr" {
  type        = string
  description = "The VNet range. Must be a /16 that does not overlap the corporate network (an owner input)."
  validation {
    condition     = can(cidrnetmask(var.vnet_cidr)) && endswith(var.vnet_cidr, "/16") && try(cidrsubnet(var.vnet_cidr, 0, 0) == var.vnet_cidr, false)
    error_message = "vnet_cidr must be a valid IPv4 /16 written as its network address, e.g. 10.40.0.0/16."
  }
}

variable "egress_mode" {
  type        = string
  description = "nat: a NAT gateway on every egress subnet — the three node subnets and agents (dev). firewall: no NAT here; the AzureFirewallSubnet is created and modules/egress-firewall routes the same four subnets through it (task 07)."
  validation {
    condition     = contains(["nat", "firewall"], var.egress_mode)
    error_message = "egress_mode must be nat or firewall."
  }
}

variable "nat_idle_timeout_minutes" {
  type        = number
  description = "NAT gateway TCP idle timeout. Long-lived agent connections (model streaming) need more than the 4-minute default."
  default     = 10
  validation {
    condition     = var.nat_idle_timeout_minutes >= 4 && var.nat_idle_timeout_minutes <= 120
    error_message = "nat_idle_timeout_minutes must be between 4 and 120."
  }
}

# Each entry becomes one NSG rule's source, so a service tag ("Internet") or "*" here would open
# the edge to everyone: only an IPv4 CIDR is accepted (task 03 as built).
variable "allowed_ingress_cidrs" {
  type        = list(string)
  description = "Source CIDRs allowed to reach the edge on 443 (owner question Q3). Empty means no inbound rule is added, so nothing outside the VNet reaches the apps pool."
  validation {
    condition     = alltrue([for c in var.allowed_ingress_cidrs : can(cidrnetmask(c))])
    error_message = "Every allowed_ingress_cidrs entry must be an IPv4 CIDR such as 203.0.113.0/24; not a service tag, not *."
  }
  # A rule's name is built from its CIDR, so a repeated entry would make two rules of one name,
  # which Azure refuses only at apply.
  validation {
    condition     = length(distinct(var.allowed_ingress_cidrs)) == length(var.allowed_ingress_cidrs)
    error_message = "allowed_ingress_cidrs lists a CIDR twice; list each once."
  }
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to every resource."
}
