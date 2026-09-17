variable "aks_cluster_id" {
  type        = string
  description = "Resource id of the existing dev AKS cluster"
}

variable "node_count" {
  type        = number
  default     = 1
  description = "Baseline node count; the autoscaler grows this on demand"
}

variable "max_count" {
  type        = number
  default     = 4
}

variable "vm_size" {
  type        = string
  default     = "Standard_D8s_v5"
  description = "Must be a Gen2 SKU that supports nested virtualisation"
}
