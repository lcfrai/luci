### Task 06: `modules/aks` — the cluster and its three pools, Kata among them

This is the module every other part of the phase lands on, so every create-time field is set
right once (P8-D2): **Azure CNI Overlay with Cilium** (Panacea's is classic CNI, its Overlay behind
a flag it cannot flip, survey §1.2 `:73`), **Entra RBAC with local accounts disabled** (Panacea's
behind `aad_rbac_enabled`, `:91-95`), **workload identity + OIDC issuer**, **host encryption**, and
**user-assigned control-plane and kubelet identities** so the grants the cluster needs exist
before the cluster does (Panacea's `NO_RBAC_MODE` is the cost of not doing this, `:183-186`).

Three pools, one per blast radius (survey §2.1, `:292`): `system` (critical add-ons only), `apps`
(Hub, broker, Traefik, cert-manager), `sandbox` (Kata, tainted). The sandbox pool's label and taint
are **exactly** the strings the pod manager already schedules on
(`control-plane/internal/sandbox/manager.go:164-170`: toleration `panacea-app/sandbox=true:NoSchedule`,
nodeSelector `panacea-app/role=sandbox`) and that phase 1's pool used (`infra/terraform/kata.tf:23-29`);
a Go test pins the two files together so neither can drift alone.

**Files:**
- `infra/terraform/modules/aks/{versions,main,pools,identity,diagnostics,variables,outputs}.tf` (new)
- `infra/terraform/modules/aks/tests/aks.tftest.hcl` (new)
- `control-plane/internal/sandbox/pool_contract_test.go` — reads `modules/aks/pools.tf` and `manager.go` (new)

#### Contract

```hcl
# variables.tf
variable "environment" {
  type        = string
  description = "dev or prod."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "location" {
  type        = string
  description = "Azure region."
}
variable "resource_group_name" {
  type        = string
  description = "The foundation resource group. The node resource group is derived: rg-panacea-app-<env>-nodes."
}
variable "kubernetes_version" {
  type        = string
  description = "Minor version, e.g. \"1.33\". Pinned, with automatic patch upgrades; bump deliberately before the minor leaves support (a pinned minor out of support blocks creates, survey §1.3 :85-87)."
  validation {
    condition     = can(regex("^1\\.[0-9]{2}$", var.kubernetes_version))
    error_message = "kubernetes_version must be a minor version like 1.33."
  }
}
variable "sku_tier" {
  type        = string
  description = "Free (dev) or Standard (prod, uptime SLA)."
  validation {
    condition     = contains(["Free", "Standard"], var.sku_tier)
    error_message = "sku_tier must be Free or Standard."
  }
}
variable "api_access" {
  type = object({
    private              = bool
    authorized_ip_ranges = list(string)
  })
  description = "private = true: private cluster (prod, OQ-5), reached from the in-VNet pool. private = false: public endpoint restricted to authorized_ip_ranges, which must not be empty."
  validation {
    condition     = var.api_access.private || length(var.api_access.authorized_ip_ranges) > 0
    error_message = "A public API server must have authorized_ip_ranges; an unrestricted public API is refused."
  }
}
variable "tenant_id" {
  type        = string
  description = "Entra tenant for AKS Entra integration; from data.azurerm_client_config."
}
variable "cluster_admin_group_object_ids" {
  type        = list(string)
  description = "Entra groups that are cluster admins (owner input). Supplied as TF_VAR_*."
  validation {
    condition     = length(var.cluster_admin_group_object_ids) > 0
    error_message = "With local accounts disabled, a cluster with no admin group can be administered by nobody."
  }
}
variable "subnet_ids" {
  type = object({
    system  = string
    apps    = string
    sandbox = string
  })
  description = "From modules/network."
}
variable "vnet_id" {
  type        = string
  description = "From modules/network; the control-plane identity gets Network Contributor on it."
}
variable "network" {
  type = object({
    pod_cidr       = string
    service_cidr   = string
    dns_service_ip = string
    outbound_type  = string
  })
  description = "Overlay pod range, service range, DNS IP (inside service_cidr), and userAssignedNATGateway | userDefinedRouting. Foundation's check proves no range overlaps the VNet."
  validation {
    condition     = contains(["userAssignedNATGateway", "userDefinedRouting"], var.network.outbound_type)
    error_message = "outbound_type must be userAssignedNATGateway (egress_mode nat) or userDefinedRouting (egress_mode firewall)."
  }
}
variable "system_pool" {
  type = object({
    vm_size   = string
    min_count = number
    max_count = number
    zones     = list(string)
  })
  description = "Critical add-ons only."
  validation {
    condition     = var.system_pool.min_count >= 1 && var.system_pool.max_count >= var.system_pool.min_count
    error_message = "system_pool needs min_count >= 1 and max_count >= min_count."
  }
}
variable "apps_pool" {
  type = object({
    vm_size   = string
    min_count = number
    max_count = number
    zones     = list(string)
  })
  description = "Hub, broker, edge."
  validation {
    condition     = var.apps_pool.min_count >= 1 && var.apps_pool.max_count >= var.apps_pool.min_count
    error_message = "apps_pool needs min_count >= 1 and max_count >= min_count."
  }
}
variable "sandbox_pool" {
  type = object({
    vm_size         = string
    min_count       = number
    max_count       = number
    zones           = list(string)
    os_disk_size_gb = number
  })
  description = "Kata nodes. vm_size must be a Gen2 SKU with nested virtualisation (Pod Sandboxing's requirement, infra/terraform/kata.tf:1-3). min_count is the warm FLOOR (G-6): a floor, not a timeout."
  validation {
    condition     = var.sandbox_pool.min_count >= 0 && var.sandbox_pool.max_count >= max(1, var.sandbox_pool.min_count)
    error_message = "sandbox_pool needs min_count >= 0 and max_count >= max(1, min_count)."
  }
}
variable "maintenance" {
  type = object({
    day_of_week    = string
    start_time     = string
    utc_offset     = string
    duration_hours = number
  })
  description = "Weekly window for patch and node-image upgrades, e.g. Sunday 02:00 +10:00 for 4 hours. Node-image upgrades drain Kata nodes; the window keeps them off working hours."
}
variable "log_analytics_workspace_id" {
  type        = string
  description = "modules/observability workspace_id."
}
variable "container_insights_dcr_id" {
  type        = string
  description = "modules/observability container_insights_dcr_id."
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

# outputs.tf
output "cluster_id" { value = azurerm_kubernetes_cluster.this.id }
output "cluster_name" { value = azurerm_kubernetes_cluster.this.name }
output "oidc_issuer_url" {
  description = "Issuer for every federated identity credential (task 10)."
  value       = azurerm_kubernetes_cluster.this.oidc_issuer_url
}
output "node_resource_group" { value = azurerm_kubernetes_cluster.this.node_resource_group }
output "kubelet_identity_object_id" {
  description = "Granted AcrPull on the registry in foundation (task 07)."
  value       = azurerm_user_assigned_identity.kubelet.principal_id
}
output "control_plane_identity_principal_id" { value = azurerm_user_assigned_identity.control_plane.principal_id }
output "secrets_provider_identity_object_id" {
  description = "The Secrets Store CSI add-on's identity (key_vault_secrets_provider), for task 10's Key Vault grant if the Hub needs one."
  value       = azurerm_kubernetes_cluster.this.key_vault_secrets_provider[0].secret_identity[0].object_id
}
output "sandbox_pool" {
  description = "The scheduling contract the pod manager depends on. Pinned by pool_contract_test.go."
  value = {
    name       = azurerm_kubernetes_cluster_node_pool.sandbox.name
    label_key  = "panacea-app/role"
    label      = "sandbox"
    taint      = "panacea-app/sandbox=true:NoSchedule"
    vm_size    = var.sandbox_pool.vm_size
  }
}
```

#### Resources (described; every argument name checked against azurerm 4.81.0)

`identity.tf`:
- `azurerm_user_assigned_identity.control_plane` (`id-aks-panacea-app-${env}`) and
  `azurerm_user_assigned_identity.kubelet` (`id-aks-kubelet-panacea-app-${env}`).
- `azurerm_role_assignment`: control plane → **Network Contributor** on `var.vnet_id`; control plane
  → **Managed Identity Operator** on the kubelet identity. The cluster `depends_on` both.

`main.tf` — `azurerm_kubernetes_cluster.this`:
- `name = "aks-panacea-app-${env}"`, `dns_prefix`, `node_resource_group = "rg-panacea-app-${env}-nodes"`,
  `kubernetes_version`, `sku_tier`, `cost_analysis_enabled = var.sku_tier == "Standard"`.
- `automatic_upgrade_channel = "patch"`, `node_os_upgrade_channel = "NodeImage"`,
  `maintenance_window_auto_upgrade` and `maintenance_window_node_os` (weekly, `var.maintenance`).
- **`local_account_disabled = true`**, `role_based_access_control_enabled = true`,
  `azure_active_directory_role_based_access_control { tenant_id, admin_group_object_ids, azure_rbac_enabled = true }`.
- **`oidc_issuer_enabled = true`, `workload_identity_enabled = true`**.
- `identity { type = "UserAssigned", identity_ids = [control_plane.id] }`,
  `kubelet_identity { client_id, object_id, user_assigned_identity_id }` from the kubelet identity.
- `private_cluster_enabled = var.api_access.private`; `api_server_access_profile {
  authorized_ip_ranges = var.api_access.private ? null : var.api_access.authorized_ip_ranges }`.
- `network_profile { network_plugin = "azure", network_plugin_mode = "overlay",
  network_data_plane = "cilium", network_policy = "cilium", pod_cidr, service_cidr,
  dns_service_ip, outbound_type, load_balancer_sku = "standard" }`.
- `default_node_pool` = **system**: `name = "system"`, `temporary_name_for_rotation = "systemtmp"`,
  `only_critical_addons_enabled = true`, `os_sku = "AzureLinux"`, `host_encryption_enabled = true`,
  `auto_scaling_enabled = true`, min/max, `zones`, `vnet_subnet_id = var.subnet_ids.system`,
  `upgrade_settings { max_surge = "33%" }`.
- `oms_agent { log_analytics_workspace_id, msi_auth_for_monitoring_enabled = true }`.
- `key_vault_secrets_provider { secret_rotation_enabled = true }` (P8-D11).
- `storage_profile { disk_driver_enabled = true, file_driver_enabled = true,
  blob_driver_enabled = false, snapshot_controller_enabled = true }` — Azure Disk for machines
  (P8-D6), Azure Files kept on for P8-D6's NFS fallback. The snapshot controller is on, but
  **no snapshot is taken in v1**: no `VolumeSnapshotClass` and no schedule exist (review m13).
  Machine-disk snapshots wait on an owner decision against OD-10 (seam S8-D6.1, task 11). The
  controller stays on so that the seam needs no cluster update.
- **`run_command_enabled = false`** (a way around Entra RBAC), `azure_policy_enabled = false`
  (phase 9, S8-P9.1), `image_cleaner_enabled = false`.
- `auto_scaler_profile { scale_down_unneeded = "10m", skip_nodes_with_local_storage = false }` —
  local storage is every sandbox's `emptyDir`; session pods are protected from consolidation by
  their `safe-to-evict: "false"` annotation instead (task 14).
- `lifecycle { ignore_changes = [default_node_pool[0].node_count] }` (the autoscaler owns it, as
  phase 1's pool already said, `kata.tf:36-41`).

`pools.tf`:
- `azurerm_kubernetes_cluster_node_pool.apps` — `name = "apps"`, `mode = "User"`, `os_sku =
  "AzureLinux"`, `host_encryption_enabled = true`, autoscaling, `node_labels =
  { "panacea-app/role" = "apps" }`, `vnet_subnet_id = var.subnet_ids.apps`, `zones`.
- `azurerm_kubernetes_cluster_node_pool.sandbox` — `name = "sandbox"`, `mode = "User"`,
  **`workload_runtime = "KataVmIsolation"`** (the value the provider accepts, `kata.tf:10-16`),
  **`os_sku = "AzureLinux"`**, `os_type = "Linux"`, `vm_size`, `os_disk_size_gb`,
  `host_encryption_enabled = true`, autoscaling with **`min_count` the warm floor**,
  `node_labels = { "panacea-app/role" = "sandbox" }`,
  `node_taints = ["panacea-app/sandbox=true:NoSchedule"]`, `vnet_subnet_id = var.subnet_ids.sandbox`,
  `zones`, `upgrade_settings { max_surge = "1", drain_timeout_in_minutes = 30,
  undrainable_node_behavior = "Cordon" }` — a node whose running sessions a PodDisruptionBudget
  protects (task 19) is cordoned and left, not force-drained mid-session.
  `lifecycle { ignore_changes = [node_count] }`.

`diagnostics.tf`:
- `azurerm_monitor_data_collection_rule_association.container_insights` — the DCR to the cluster.
- `azurerm_monitor_diagnostic_setting.aks` — `log_analytics_destination_type = "Dedicated"`,
  `enabled_log` for **`kube-audit-admin`** (not `kube-audit`: writes only, survey §1.11 `:219-220`),
  `kube-apiserver`, `kube-controller-manager`, `cluster-autoscaler`, `guard`. **The category names
  are from training knowledge, UNVERIFIED**; Azure rejects an unknown category at apply.

#### Tests to write first

`tests/aks.tftest.hcl` (mocks only; a `variables` block with a public dev API, one admin group,
mock subnet ids and `outbound_type = "userAssignedNATGateway"`):

| Run | Asserts | Mutant it kills |
|---|---|---|
| `local_accounts_off_and_entra_rbac_on` | `local_account_disabled == true`; the Entra block has `azure_rbac_enabled == true` and the admin group | local accounts on — the admin certificate Panacea's Helm provider depends on |
| `overlay_with_cilium` | `network_plugin == "azure"`, `network_plugin_mode == "overlay"`, `network_data_plane == "cilium"`, `network_policy == "cilium"` | classic CNI, or Cilium data plane without policy enforcement |
| `workload_identity_and_oidc_on` | both `true` | either off — task 10's federated credentials would have no issuer |
| `public_api_needs_authorized_ranges` | `api_access = { private = false, authorized_ip_ranges = [] }` → `expect_failures = [var.api_access]` | an open public API server |
| `private_api_is_a_private_cluster` | `private = true` → `private_cluster_enabled == true` and no authorised ranges | a private flag that sets nothing |
| `sandbox_pool_is_kata_on_azure_linux` | `workload_runtime == "KataVmIsolation"`, `os_sku == "AzureLinux"` | `OCIContainer` (a plain runc pool that looks right in a plan) |
| `sandbox_pool_label_and_taint_are_the_managers` | labels `{"panacea-app/role" = "sandbox"}`, taints `["panacea-app/sandbox=true:NoSchedule"]` | a renamed key; sessions would stay Pending forever |
| `every_pool_encrypts_at_host` | system, apps and sandbox all `host_encryption_enabled == true` | one pool missed |
| `system_pool_takes_critical_addons_only` | `only_critical_addons_enabled == true` | the Hub landing on the system pool |
| `kubelet_identity_is_user_assigned` | `kubelet_identity[0].user_assigned_identity_id == azurerm_user_assigned_identity.kubelet.id` | a system-assigned kubelet identity, which cannot hold AcrPull before the cluster exists |
| `audit_is_the_admin_category` | the diagnostic setting has `kube-audit-admin` and not `kube-audit` | the full audit stream that filled Panacea's dev cap |
| `run_command_is_off` | `run_command_enabled == false` | a path around Entra RBAC |
| `version_must_be_a_minor` | `kubernetes_version = "1.33.2"` → `expect_failures` | a pinned patch that blocks the patch channel |
| `sandbox_max_must_exceed_zero` | `sandbox_pool.max_count = 0` → `expect_failures` | a pool that can never run a session |

`control-plane/internal/sandbox/pool_contract_test.go` (Go, reads files, no OpenTofu):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSandboxPoolTaintMatchesTheTolerationInCreate` | the literal `panacea-app/sandbox=true:NoSchedule` appears in `infra/terraform/modules/aks/pools.tf`, and `Manager.Create`'s pod (built against a fake clientset) tolerates exactly key `panacea-app/sandbox`, value `true`, effect `NoSchedule` | the taint renamed in one file only |
| `TestSandboxPoolLabelMatchesTheNodeSelector` | `"panacea-app/role" = "sandbox"` appears in `pools.tf`, and the pod's `NodeSelector` is `{"panacea-app/role": "sandbox"}` | the label renamed in one file only |

The Go test finds the repo root by walking up from the test's working directory to the directory
holding `go.work`, so it runs the same from `go test ./control-plane/...` at the root.

#### Steps

- [ ] **Step 1:** Variables, outputs skeleton and the OpenTofu tests; validate fails.
- [ ] **Step 2:** `identity.tf`, then the cluster, then `pools.tf`, then `diagnostics.tf`, running
  the tests after each.
- [ ] **Step 3:** Write `pool_contract_test.go`; run
  `go test ./control-plane/internal/sandbox/ -run PoolContract -count=1` — both pass against the
  real `manager.go`. Then **mutate by hand once**: change the taint value in a scratch copy of
  `pools.tf`, rerun, see it fail, restore. (Record the mutation and the failure in the commit
  message; do not commit the mutation.)
- [ ] **Step 4:** fmt, guards, commit by pathspec (the module directory and the Go test).

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/aks
# validate: OK   infra/terraform/modules/aks
# validate: 1 ok, 0 failed                            (exit 0)
#   validate.sh prints a step's own output only when that step fails, so tofu test's
#   "Success! 14 passed, 0 failed." is NOT on stdout on a green run (task 04 review 1, M3);
#   run tofu test in the module directory to see the count.
go test ./control-plane/internal/sandbox/ -run 'TestSandboxPool' -race -count=1
# ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox
```

#### What this task does not claim

- **That Kata works on `vm_size`.** Nested-virtualisation support by SKU family is **UNVERIFIED**
  here; dev's `Standard_D8s_v5` is phase 1's choice (`infra/terraform/variables.tf:17-21`). Task 32
  proves it, and measures whether `Standard_D8ds_v5` with an ephemeral OS disk starts nodes faster.
- That host encryption is enabled on the subscription (it needs a feature registration —
  `Microsoft.Compute/EncryptionAtHost` — **from training knowledge, UNVERIFIED**; task 31's
  runbook checks it before the first apply).
- That `undrainable_node_behavior = "Cordon"` does what its name says with a PDB-protected Kata
  pod; task 32 lists it.
