### Task 10: `modules/workload-identity` and `stacks/workloads` — the pods' identities, and nothing more

Every pod that touches Azure gets a user-assigned identity federated to its Kubernetes service
account through the cluster's OIDC issuer (survey §2.1, `:296`: "no client secrets"). Only three
pods touch Azure in this phase: the **Hub** (Postgres, and Key Vault if HA-4 says so), the Hub's
**migration Job** (Postgres DDL, a separate identity per HA-5's default), and **cert-manager**
(DNS-01 TXT records in our zone). The broker, the machine agent and every sandbox **have no Azure
identity at all**: the broker speaks only to the Kubernetes API with its service account, and
sandboxes run with no service-account token (task 14).

This stack also holds the **role allowlist**: the exact set of role names any Terraform stack in
this repo **assigns**. Five of its seven lines are what the owner's ABAC condition on the **infra**
identity should permit (owner input, survey §7.1 item 3): the roles the stacks that identity
applies (`foundation`, `data`, `workloads`) assign to the pods' and the cluster's own identities.
The other two, the Storage Blob Data roles, are `bootstrap`'s, which the operator applies (task 31
§2), so the condition leaves them out. The condition also excludes the infra identity's own
object id as a principal.

The allowlist is **not** the list of roles any identity **holds**. The pipeline identities' roles
are the owner's grants, not a stack's, and they are in the plan's "Deploy identities" table
(review I8). **No stack grants a pipeline identity anything** (plan re-review I-2 of fix batch
b02). The cluster and push identities' roles are granted by the owner's
`infra/rbac/grant-pipeline-roles.sh` (task 30), which is a script the owner runs, not a stack, so
they are not on this list. This matters because an ABAC condition limits the role and the
principal, not the scope of the assignment (**training knowledge, UNVERIFIED**). With
`Azure Kubernetes Service RBAC Admin` on the list, the infra identity could assign it at cluster
scope, or on `panacea-sandboxes`, to an identity it created, and read every user's token.

So the allowlist holds **no AKS data-plane role and no AcrPush**: not RBAC Cluster Admin (review
I8), not RBAC Admin, not Cluster User Role, not the two `panacea-app-*-deployer` custom roles. The
infra identity has no route to an AKS data-plane role **through a role assignment**. It keeps
other routes, because it manages the resources those roles protect. The plan's "Deploy
identities" residual 1 lists them.

**Files:**
- `infra/terraform/modules/workload-identity/{versions,main,variables,outputs}.tf`, `tests/workload-identity.tftest.hcl` (new)
- `infra/terraform/stacks/workloads/{versions,backend,providers,main,variables,outputs}.tf` (new)
- `infra/terraform/stacks/workloads/{dev,prod}.tfvars`, `.terraform.lock.hcl`, `tests/workloads.tftest.hcl` (new)
- `infra/terraform/role-allowlist.txt` — one role name per line (new)
- `infra/scripts/role-allowlist.sh` — fails if any `role_definition_name` under `infra/terraform` is not in the file (new)

#### Contract — `modules/workload-identity`

```hcl
variable "name" {
  type        = string
  description = "Identity name, e.g. id-hub-panacea-app-dev."
}
variable "location" {
  type        = string
  description = "Azure region."
}
variable "resource_group_name" {
  type        = string
  description = "Where the identity lives (the foundation group)."
}
variable "oidc_issuer_url" {
  type        = string
  description = "foundation's aks_oidc_issuer_url."
}
variable "namespace" {
  type        = string
  description = "Kubernetes namespace of the service account."
  validation {
    condition     = can(regex("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", var.namespace))
    error_message = "namespace must be a DNS-1123 label."
  }
}
variable "service_account" {
  type        = string
  description = "Kubernetes service account name."
  validation {
    condition     = can(regex("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", var.service_account))
    error_message = "service_account must be a DNS-1123 label."
  }
}
variable "role_assignments" {
  type = list(object({
    scope                = string
    role_definition_name = string
  }))
  description = "Grants for this identity. Every role name must be in infra/terraform/role-allowlist.txt."
  default     = []
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

output "id" { value = azurerm_user_assigned_identity.this.id }
output "name" { value = azurerm_user_assigned_identity.this.name }
output "client_id" {
  description = "Goes on the service account as azure.workload.identity/client-id (the charts)."
  value       = azurerm_user_assigned_identity.this.client_id
}
output "principal_id" { value = azurerm_user_assigned_identity.this.principal_id }
output "subject" { value = "system:serviceaccount:${var.namespace}:${var.service_account}" }
```

Resources: `azurerm_user_assigned_identity.this`; `azurerm_federated_identity_credential.this`
with **`user_assigned_identity_id`** (4.81.0 deprecates `parent_id` and `resource_group_name` on
this resource), `issuer = var.oidc_issuer_url`, `audience = ["api://AzureADTokenExchange"]`,
`subject = "system:serviceaccount:${ns}:${sa}"`; one `azurerm_role_assignment` per entry of
`role_assignments` (keyed by `"${role}|${scope}"` so reordering the list changes nothing).

#### Contract — `stacks/workloads`

```hcl
variable "environment" {
  type        = string
  description = "dev or prod."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "state_storage_account_name" {
  type        = string
  description = "bootstrap output, TF_VAR_*."
}
variable "state_resource_group_name" {
  type        = string
  description = "bootstrap output, TF_VAR_*."
}
variable "hub_key_vault_access" {
  type        = bool
  description = "Grant the Hub Key Vault Secrets User on the vault (HA-4). false until the Hub contract says it reads a secret."
  default     = false
}
variable "namespaces" {
  type = object({
    hub          = string
    cert_manager = string
  })
  description = "Where the charts put these service accounts (task 13's cert-manager release, task 29's hub)."
  default = {
    hub          = "panacea-hub"
    cert_manager = "panacea-edge"
  }
}

# data "terraform_remote_state" "foundation" and "data" — the same config shape as task 08's

output "hub_client_id" { value = module.hub.client_id }
output "hub_identity_name" {
  description = "The Postgres role name the database bootstrap creates with pgaadauth_create_principal (task 29's SQL), run by a human Postgres admin, never a pipeline (task 31 §7, review I8)."
  value       = module.hub.name
}
output "hub_migrate_client_id" { value = module.hub_migrate.client_id }
output "hub_migrate_identity_name" { value = module.hub_migrate.name }
output "cert_manager_client_id" { value = module.cert_manager.client_id }
output "helm_values" {
  description = "Exactly what the deploy pipeline passes to the charts with --set, so no chart hard-codes an id."
  value = {
    "hub.serviceAccount.clientId"         = module.hub.client_id
    "hub.migrate.serviceAccount.clientId" = module.hub_migrate.client_id
    "cert-manager.identity.clientId"      = module.cert_manager.client_id   # the service account (task 13)
    "edge.certManagerIdentity.clientId"   = module.cert_manager.client_id   # the ClusterIssuer (task 13)
  }
}
```

Module calls: `hub` (`panacea-hub` / `hub`, Key Vault Secrets User on the vault only when
`hub_key_vault_access`); `hub_migrate` (`panacea-hub` / `hub-migrate`, no Azure role — its
Postgres rights are SQL); `cert_manager` (`panacea-edge` / `cert-manager`, **DNS Zone Contributor
scoped to the zone**, not the resource group).

#### `role-allowlist.txt` (complete)

```
Storage Blob Data Contributor
Storage Blob Data Reader
Network Contributor
Managed Identity Operator
AcrPull
DNS Zone Contributor
Key Vault Secrets User
```

Who assigns each: the two Storage Blob Data roles are `bootstrap`'s state grants, to the infra
identity (write) and the plan identity (read) (task 02). The operator makes them under their own
credentials (task 31 §2), not the infra identity, so the ABAC condition leaves them out. They are
listed because `role-allowlist.sh` checks every literal under `infra/terraform`.
`Network Contributor` is the AKS control plane's, on the VNet (task 06) and on the edge IP
(`foundation`'s `grants.tf`, task 07); `AcrPull` is the kubelet's (`grants.tf`);
`Managed Identity Operator` is task 06's control-plane grant on the kubelet identity;
`DNS Zone Contributor` and `Key Vault Secrets User` are this stack's. Those five are the ABAC
condition's list.

Not on the list, and never assigned by a stack (plan re-review I-2): `AcrPush`,
`Azure Kubernetes Service RBAC Admin`, `Azure Kubernetes Service Cluster User Role`,
`panacea-app-cluster-deployer`, `panacea-app-sandboxes-deployer`, and
`Azure Kubernetes Service RBAC Cluster Admin`. The first five are the owner's
`grant-pipeline-roles.sh` (task 30). The last is granted by nobody.

`role-allowlist.sh` greps every `role_definition_name = "…"` literal under `infra/terraform` (and
every `role_definition_name` value in a `role_assignments` list literal) and prints
`role-allowlist: <n> roles used, all allowed` or names the first unlisted one and exits 1. A role
assigned through a variable is resolved by the stack tests below, which assert on planned role
names, not by the script.

#### Tests to write first

`modules/workload-identity/tests/workload-identity.tftest.hcl`:

| Run | Asserts | Mutant it kills |
|---|---|---|
| `subject_is_the_service_account` | `subject == "system:serviceaccount:panacea-hub:hub"` | a subject typo; the token exchange fails with an unhelpful 401 at runtime |
| `audience_is_the_token_exchange` | `audience == ["api://AzureADTokenExchange"]` | an empty or wrong audience |
| `credential_uses_the_non_deprecated_parent` | `user_assigned_identity_id` set; `parent_id` null | the deprecated argument, which the next provider major removes |
| `issuer_is_passed_through` | `issuer == var.oidc_issuer_url` | a hard-coded issuer |
| `reordering_grants_changes_nothing` | two runs with the same two grants in opposite order plan the same instance keys | `count` keyed by index |
| `bad_namespace_is_rejected` | `namespace = "Panacea_Hub"` → `expect_failures` | an unvalidated name |

`stacks/workloads/tests/workloads.tftest.hcl` (`override_data` for both remote states):

| Run | Asserts | Mutant it kills |
|---|---|---|
| `cert_manager_can_write_only_its_zone` | cert-manager's one grant is `DNS Zone Contributor` with `scope == dns_zone_id` | Contributor on the resource group |
| `hub_has_no_azure_role_by_default` | `hub_key_vault_access = false` → the hub module has zero role assignments | a speculative Key Vault grant |
| `hub_vault_grant_is_secrets_user_on_the_vault` | `true` → exactly `Key Vault Secrets User` on `key_vault_id` | Key Vault Administrator |
| `migrate_is_a_separate_identity` | `module.hub_migrate` exists with subject `system:serviceaccount:panacea-hub:hub-migrate` | DDL rights on the serving identity |
| `no_identity_for_the_broker_or_sandboxes` | no module or identity whose subject names `panacea-broker` or `panacea-sandboxes` | a broker identity added "in case" |
| `helm_values_carry_every_client_id` | `command = apply` → `output.helm_values` has the three keys, each non-empty | a chart left with an empty client id |

And `bash infra/scripts/role-allowlist.sh` → `role-allowlist: 7 roles used, all allowed`.

#### Steps

- [ ] **Step 1:** Re-read HA-4/HA-5 in the Hub contract: whether the Hub reads Key Vault (sets
  `hub_key_vault_access` in tfvars), and whether migrations need their own identity.
- [ ] **Step 2:** Module: variables, tests, resources, outputs.
- [ ] **Step 3:** Stack: remote states, calls, outputs, tfvars, lock, tests.
- [ ] **Step 4:** `role-allowlist.txt` and its script; run it over the whole of `infra/terraform`
  (tasks 02–10) — it must list exactly the seven roles, or a task introduced a role nobody listed.
  A mutant that puts `AcrPush` or `Azure Kubernetes Service RBAC Admin` back into `grants.tf` must
  turn it red (plan re-review I-2).
- [ ] **Step 5:** guards, validate, commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/workload-identity infra/terraform/stacks/workloads
# validate: 2 ok, 0 failed
bash infra/scripts/role-allowlist.sh
# role-allowlist: 7 roles used, all allowed
```

#### What this task does not claim

- That the owner's ABAC condition can be written to exactly this list — a condition on
  `Microsoft.Authorization/roleAssignments/write` constrained by role definition id is the
  documented pattern **from training knowledge, UNVERIFIED**; the owner's identity team writes it.
- That cert-manager's namespace is `panacea-edge`: task 13 fixes it, and this default must match.
- That the infra identity cannot act as the Hub or as cert-manager. It applies this stack, so it
  writes these identities' federated credentials, and could make one trust an issuer it controls
  (Contributor covering `federatedIdentityCredentials/write` is **training knowledge, UNVERIFIED**).
  No allowlist bounds that. Whoever deploys an identity can act as it. The bound is the infra
  connection's ADO checks (the plan's "Deploy identities" residual 1, plan re-review I-2).
