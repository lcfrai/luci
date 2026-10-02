# Pod Sandboxing (Kata Containers) requires Azure Linux, a Gen2
# nested-virtualisation-capable SKU, and workload_runtime KataVmIsolation
# (see the NOTE below on this field).
resource "azurerm_kubernetes_cluster_node_pool" "kata" {
  name                  = "katapool"
  kubernetes_cluster_id = var.aks_cluster_id
  vm_size               = var.vm_size
  os_sku                = "AzureLinux"
  os_type               = "Linux"
  # NOTE: brief said "KataMshvVmIsolation" but the installed azurerm provider
  # (v4.81.0) only accepts KataVmIsolation | OCIContainer | WasmWasi for this
  # field (confirmed via `terraform validate` against the real provider
  # schema). "kata-mshv-vm-isolation" is the Kubernetes RuntimeClass name
  # (see Interfaces section of the task brief), a distinct string from this
  # provider argument's enum. Corrected to the value the provider accepts.
  workload_runtime      = "KataVmIsolation"

  auto_scaling_enabled = true
  node_count           = var.node_count
  min_count            = var.node_count
  max_count            = var.max_count

  node_labels = {
    "luci/role" = "sandbox"
  }

  node_taints = [
    "luci/sandbox=true:NoSchedule",
  ]

  tags = {
    product = "luci"
    phase   = "1"
  }

  lifecycle {
    # auto_scaling_enabled lets the cluster autoscaler change node_count
    # outside Terraform; without ignore_changes, Terraform would fight the
    # autoscaler on every plan.
    ignore_changes = [node_count]
  }
}

output "kata_node_pool_name" {
  value = azurerm_kubernetes_cluster_node_pool.kata.name
}
