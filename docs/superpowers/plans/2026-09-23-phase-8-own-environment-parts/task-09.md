### Task 09: `stacks/identity` — the Entra registrations, as configuration

OD-6 has the desktop sign in with Entra auth-code + PKCE and the control plane call servers **as
the user**. That needs registrations in Entra: a public client for the desktop, and one resource
API per server that validates the user's token. Survey G-8 (`:429`) found none in code. This stack
writes them on the `azuread` provider, **as its own stack** so it can run under a different identity
from the others: if the owner grants the deploy identity Application Administrator, the pipeline
runs it; if not, the owner runs it once with their own credentials, or creates the registrations by
hand and hands over the ids (survey §7.2 item 1, `:539-541`) — `stacks/workloads` and the charts
read only this stack's **outputs**, so either path works.

**This task is the Hub's ask HA-3.** ADR-H1 (read at 195 lines, §0–§3) scores design A highest: the
control plane runs PKCE against Entra directly, the Hub validates Entra access tokens itself
against JWKS, no broker (ADR-H1 `:136-141`). That fits the three registrations below. Its final
list of registrations was not yet written; step 1 re-reads the Hub contract and reconciles.

**Files:**
- `infra/terraform/stacks/identity/{versions,backend,providers,main,variables,outputs}.tf` (new)
- `infra/terraform/stacks/identity/{dev,prod}.tfvars`, `.terraform.lock.hcl`, `tests/identity.tftest.hcl` (new)

#### What is registered

| Registration | Kind | Exposes / requests | Why |
|---|---|---|---|
| `Panacea App Desktop (<env>)` | public client, single tenant | redirect `http://localhost` (loopback, system browser — RFC 8252); requests `Hub.Access` and `Sandboxes.Access`; pre-authorised on both APIs so the user sees no consent prompt | OD-6 |
| `Panacea App Hub API (<env>)` | resource API, single tenant, v2 tokens | delegated scope **`Hub.Access`**; identifier `api://<client id>` | ADR-H1 design A; provisional until HA-3 |
| `Panacea App Broker API (<env>)` | resource API, single tenant, v2 tokens | delegated scope **`Sandboxes.Access`**; identifier `api://<client id>` | P8-D4 — a separate audience, so a Hub token cannot drive sandboxes and a broker token cannot read briefs |

Each has a service principal with **`app_role_assignment_required = true`**, and the product-users
group (owner input) is assigned to all three, so only that group can obtain a token at all. **Which
service principal Entra enforces assignment on for a delegated token — the client's, the resource's
or both — is from training knowledge and UNVERIFIED**; assigning on all three is correct under
either reading. No client secret and no certificate exists on any registration: the desktop is a
public client and the two APIs only validate.

#### Contract

```hcl
# versions.tf
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    azuread = { source = "hashicorp/azuread", version = "3.9.0" }
  }
}
# backend.tf — backend "azurerm" {}; providers.tf — provider "azuread" {} (tenant from ARM_TENANT_ID)
data "azuread_client_config" "current" {}

# variables.tf
variable "environment" {
  type        = string
  description = "dev or prod; in every display name."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "user_group_object_id" {
  type        = string
  description = "The Entra group whose members may use the product (owner input, TF_VAR_*)."
}
variable "owner_object_ids" {
  type        = list(string)
  description = "Entra principals set as owners of every registration, so it is never orphaned (owner input, TF_VAR_*)."
  validation {
    condition     = length(var.owner_object_ids) > 0
    error_message = "An application with no owner can be changed only by a tenant administrator."
  }
}

# main.tf — scope ids are fixed, generated once for this repository; a scope id is not a secret
locals {
  scope_ids = {
    hub    = "3f7c2a10-5b8e-4d61-9a0c-7e21b4d9c6a1"
    broker = "b8e41d27-0c9a-4f3e-8d52-61a7c3e9f0b4"
  }
  default_access_role = "00000000-0000-0000-0000-000000000000"
}

# outputs.tf — every consumer reads these, never a display name
output "tenant_id" { value = data.azuread_client_config.current.tenant_id }
output "desktop_client_id" { value = azuread_application.desktop.client_id }
output "hub_api_client_id" { value = azuread_application.hub.client_id }
output "hub_api_scope" {
  description = "The scope string the control plane requests for the Hub."
  value       = "api://${azuread_application.hub.client_id}/Hub.Access"
}
output "broker_api_client_id" {
  description = "The broker's expected audience (v2 tokens carry the client id as aud)."
  value       = azuread_application.broker.client_id
}
output "broker_api_scope" {
  description = "The scope string remotebe requests (task 22)."
  value       = "api://${azuread_application.broker.client_id}/Sandboxes.Access"
}
```

The two fixed scope ids above are written into this plan so the part is complete; the implementer
**replaces both with freshly generated UUIDs** in step 2 (`[guid]::NewGuid()` or `uuidgen`) and
records that in the commit, because a scope id copied from a document is a scope id someone else
may also have copied.

#### Resources (described; names from azuread 3.9.0)

- `azuread_application.hub` — `display_name = "Panacea App Hub API (${env})"`,
  `sign_in_audience = "AzureADMyOrg"`, `owners`, `prevent_duplicate_names = true`,
  `api { requested_access_token_version = 2, oauth2_permission_scope { id = local.scope_ids.hub,
  value = "Hub.Access", type = "User", enabled = true, admin_consent_display_name,
  admin_consent_description, user_consent_display_name, user_consent_description } }`,
  `lifecycle { ignore_changes = [identifier_uris] }`; `azuread_application_identifier_uri.hub` —
  `identifier_uri = "api://${client_id}"` (a separate resource because the URI embeds the client
  id, which exists only after create).
- `azuread_application.broker` and its identifier URI — the same shape with `Sandboxes.Access`.
- `azuread_application.desktop` — `display_name = "Panacea App Desktop (${env})"`,
  `sign_in_audience = "AzureADMyOrg"`, `owners`, `public_client { redirect_uris = ["http://localhost"] }`,
  `fallback_public_client_enabled = false`, two `required_resource_access` blocks (Hub scope,
  broker scope), `api { requested_access_token_version = 2 }`.
- `azuread_application_pre_authorized` × 2 — `application_id` = each API's `id`,
  `authorized_client_id = azuread_application.desktop.client_id`, `permission_ids` = its scope id.
- `azuread_service_principal` × 3 — `client_id`, `app_role_assignment_required = true`, `owners`.
- `azuread_app_role_assignment` × 3 — `app_role_id = local.default_access_role`,
  `principal_object_id = var.user_group_object_id`, `resource_object_id` = each service principal.

#### Tests to write first (`tests/identity.tftest.hcl`, `mock_provider "azuread"`; validate.sh runs it per tfvars)

| Run | Asserts | Mutant it kills |
|---|---|---|
| `desktop_is_a_public_client_on_loopback` | `public_client[0].redirect_uris == ["http://localhost"]`, no `web` redirect, no `password` block | a confidential client that needs a secret on every laptop |
| `apis_issue_v2_tokens` | both APIs `requested_access_token_version == 2` | v1 tokens, whose `aud` and `iss` differ from what task 16 validates |
| `broker_scope_is_sandboxes_access` | the broker's one scope has `value == "Sandboxes.Access"`, `type == "User"` | a renamed scope the broker would reject |
| `hub_and_broker_are_separate_audiences` | `azuread_application.hub` and `.broker` are distinct resources with distinct scope ids | one API for both (a Hub token could drive sandboxes) |
| `desktop_is_pre_authorised_on_both` | two `azuread_application_pre_authorized`, each naming the desktop client and the right scope id | a consent prompt at first sign-in, or pre-authorisation of the wrong scope |
| `assignment_is_required_everywhere` | all three service principals `app_role_assignment_required == true`; three role assignments to `var.user_group_object_id` | anyone in the tenant can get a token |
| `every_registration_has_owners` | `owners == var.owner_object_ids` on all three; `owner_object_ids = []` → `expect_failures` | an orphaned registration |
| `no_secret_anywhere` | no `azuread_application_password` or `azuread_application_certificate` resource in the plan (asserted by the plan's resource addresses) | a secret added "for testing" |
| `scope_output_is_the_string_remotebe_requests` | `command = apply` → `output.broker_api_scope == "api://${client_id}/Sandboxes.Access"` | a scope output missing the `api://` prefix |

#### Steps

- [ ] **Step 1: Reconcile with the Hub.** Read `.claude/worktrees/hub/docs/superpowers/contracts/`
  and the final ADR-H1. If the Hub contract names its registrations, scopes or app roles, write
  those instead of the Hub row above and note the difference in the commit message. **If the Hub
  wants one API registration for both servers, do not merge the broker into it** — record the
  disagreement as a question to the orchestrator (P8-D4's separate audience is a security
  property, not a preference) and keep the broker's own registration.
- [ ] **Step 2:** Variables, locals (fresh scope UUIDs), tests; validate fails.
- [ ] **Step 3:** Resources, outputs, until the nine runs pass for both tfvars.
- [ ] **Step 4:** `dev.tfvars` / `prod.tfvars` hold only `environment`; lock file; guards; commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/stacks/identity
# validate: OK   infra/terraform/stacks/identity         (9 runs × 2 tfvars pass)
bash infra/scripts/guards.sh     # guards: 8 of 8 passed
```

#### What this task does not claim

- That the deploy identity may create applications — an owner input.
- That `http://localhost` without a port is accepted as a loopback redirect for any port by the
  owner's tenant — Entra's documented behaviour for public clients, **from training knowledge,
  UNVERIFIED**; the Hub's desktop sign-in task proves it on a laptop first.
- The Azure DevOps delegated permission OD-4/OD-6 may want the desktop to hold (the Azure DevOps
  resource's app id and scope id were not verified); phase 3's credential store (its H-1) and the
  Hub contract decide whether the desktop registration requests it.
