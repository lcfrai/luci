### Task 09: `stacks/identity` — the Entra registrations, as configuration

OD-6 has the desktop sign in with Entra auth-code + PKCE and the control plane call servers **as
the user**. That needs registrations in Entra: a public client for the desktop and one resource API
per server. Survey G-8 (`:429`) found none in code. This stack writes them on the `azuread`
provider, **as its own stack** so it runs under a different identity from the others. **It is
owner-run only** (review I8). Creating applications and granting their consent takes tenant-level
Graph permissions, which in Entra come close to tenant administration (training knowledge,
UNVERIFIED). No pipeline identity is given them. So the owner either applies this stack once with
their own credentials from a workstation, or makes the registrations by hand and hands over the
ids (survey §7.2 item 1, `:539-541`). Task 30's `tf-stack.sh` refuses the stack name, and
`infra-plan`'s default stack list leaves it out. `stacks/workloads` and the charts read only this
stack's **outputs**, so either path works. The plan's "Deploy identities" table records the
decision.

**The values are the Hub contract's.** HA-3 asked for the exact list; the Hub contract answered it
(sibling `feat/hub`, `2026-09-23-hub-contract.md` §6.5, `:528-551`, "exact values for phase 8's
task 09"). The desktop and Hub API rows below are that table, transcribed; the broker API row is
this plan's own (P8-D4) and follows the Hub API's pattern. Two deliberate differences from the
table, both recorded for the Hub's reviewer: display names carry an environment suffix outside
prod (two environments in one tenant would otherwise collide under `prevent_duplicate_names`), and
the broker API is assignment-required like the Hub API.

**Files:**
- `infra/terraform/stacks/identity/{versions,backend,providers,data,main,variables,outputs}.tf` (new)
- `infra/terraform/stacks/identity/{dev,prod}.tfvars`, `.terraform.lock.hcl`, `tests/identity.tftest.hcl` (new)

#### What is registered (Hub contract §6.5, plus the broker)

| Setting | Desktop — *PanaceaApp Desktop* | Hub API — *PanaceaApp Hub API* | Broker API — *PanaceaApp Broker API* (phase 8) |
|---|---|---|---|
| Sign-in audience | `AzureADMyOrg` | `AzureADMyOrg` | `AzureADMyOrg` |
| Platform | public client | none | none |
| Redirect URIs | `http://127.0.0.1` and `http://localhost` | none | none |
| Credentials | **none** | **none** | **none** |
| `fallback_public_client_enabled` | `false` | `false` | `false` |
| Application ID URI | — | `api://<its own client id>` | `api://<its own client id>` |
| `requested_access_token_version` | — | `2` | `2` |
| Exposed scope | — | `Hub.Access`, *Admins and users* | `Sandboxes.Access`, *Admins and users* |
| Pre-authorised clients | — | the desktop, for `Hub.Access` | the desktop, for `Sandboxes.Access` |
| App roles | — | `Hub.OrgAdmin` (*Users/Groups*) | none |
| Optional claims | — | access token: `email` | none |
| `group_membership_claims` | `["None"]` | `["None"]` | `["None"]` |
| Required resource access | Graph delegated `openid`, `profile`, `offline_access`, `email`; `Hub.Access`; `Sandboxes.Access`; Azure DevOps `user_impersonation` only if `request_azure_devops` (ADR-H1 open question Q5) | none | none |
| Admin consent | **tenant-wide**, for every row above | — | — |
| Assignment required | no | **yes** — group *PanaceaApp Users* | **yes** — the same group |
| Role assignments | — | `Hub.OrgAdmin` → group *PanaceaApp Org Admins* | — |

**Graph ids are looked up, not typed.** `data "azuread_application_published_app_ids"` gives
Microsoft Graph's client id, and `data "azuread_service_principal" "graph"` its
`oauth2_permission_scope_ids` map, so no well-known GUID is transcribed from memory. The Azure
DevOps resource is looked up the same way when `request_azure_devops` is true.

#### Contract

```hcl
# versions.tf — azuread 3.9.0 only; backend.tf — backend "azurerm" {}
# providers.tf — provider "azuread" {}   (tenant from ARM_TENANT_ID)

# variables.tf
variable "environment" {
  type        = string
  description = "dev or prod; non-prod display names carry \" (<env>)\"."
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}
variable "user_group_object_id" {
  type        = string
  description = "Group PanaceaApp Users: who may obtain a token for the Hub and the broker (owner input, TF_VAR_*)."
}
variable "org_admin_group_object_id" {
  type        = string
  description = "Group PanaceaApp Org Admins: assigned Hub.OrgAdmin (owner input, TF_VAR_*)."
}
variable "owner_object_ids" {
  type        = list(string)
  description = "Owners of every registration, so none is orphaned (owner input, TF_VAR_*)."
  validation {
    condition     = length(var.owner_object_ids) > 0
    error_message = "An application with no owner can be changed only by a tenant administrator."
  }
}
variable "request_azure_devops" {
  type        = bool
  description = "Whether the desktop also requests Azure DevOps user_impersonation (ADR-H1 Q5). One sign-in then serves ADO too."
  default     = false
}

# main.tf — ids fixed once for this repository (not secrets); REPLACE with fresh UUIDs in step 2.
locals {
  suffix = var.environment == "prod" ? "" : " (${var.environment})"
  ids = {
    hub_scope       = "3f7c2a10-5b8e-4d61-9a0c-7e21b4d9c6a1"
    broker_scope    = "b8e41d27-0c9a-4f3e-8d52-61a7c3e9f0b4"
    hub_org_admin   = "d2a9e4c1-7f36-4b08-a5e2-0c9b1f7d3e68"
  }
  graph_scopes        = ["openid", "profile", "offline_access", "email"]
  default_access_role = "00000000-0000-0000-0000-000000000000"
}

# outputs.tf — every consumer reads these, never a display name
output "tenant_id" { value = data.azuread_client_config.current.tenant_id }
output "desktop_client_id" { value = azuread_application.desktop.client_id }
output "hub_api_client_id" { value = azuread_application.hub.client_id }
output "hub_api_scope" {
  description = "PANACEA_APP_HUB_SCOPE (Hub contract §9.1)."
  value       = "api://${azuread_application.hub.client_id}/Hub.Access"
}
output "broker_api_client_id" {
  description = "The broker's PANACEA_APP_BROKER_ENTRA_AUDIENCE (v2 tokens carry the client id as aud)."
  value       = azuread_application.broker.client_id
}
output "broker_api_scope" {
  description = "PANACEA_APP_BROKER_SCOPE on the control plane (task 23)."
  value       = "api://${azuread_application.broker.client_id}/Sandboxes.Access"
}
```

#### Resources (described; names from azuread 3.9.0)

- `data.azuread_client_config.current`, `data.azuread_application_published_app_ids.well_known`,
  `data.azuread_service_principal.graph` (by Graph's published client id); and, when
  `request_azure_devops`, the Azure DevOps service principal the same way.
- `azuread_application.hub` / `.broker`: display name + `local.suffix`, `sign_in_audience`,
  `owners`, `prevent_duplicate_names = true`, `group_membership_claims = ["None"]`,
  `api { requested_access_token_version = 2, oauth2_permission_scope { id, value, type = "User",
  enabled = true, admin_consent_display_name, admin_consent_description, user_consent_display_name,
  user_consent_description } }` (`type = "User"` is the portal's *Admins and users*; the Hub's
  admin display name is the contract's *"Use PanaceaApp's shared workspace as the signed-in
  user"*); the Hub's `app_role { id = local.ids.hub_org_admin,
  value = "Hub.OrgAdmin", allowed_member_types = ["User"], display_name = "PanaceaApp org
  administrator", description, enabled = true }`; the Hub's `optional_claims { access_token {
  name = "email" } }`; `lifecycle { ignore_changes = [identifier_uris] }`; and
  `azuread_application_identifier_uri` for `api://${client_id}` (a separate resource because the
  URI embeds the id).
- `azuread_application.desktop`: `public_client { redirect_uris = ["http://127.0.0.1/",
  "http://localhost/"] }` (the provider requires a trailing path; the Hub contract's rule is "the
  registered path", here `/`), `fallback_public_client_enabled = false`, `required_resource_access`
  for Graph (the four scope ids from the data source), the Hub, the broker, and ADO when asked.
- `azuread_application_pre_authorized` × 2 (Hub/`Hub.Access`, broker/`Sandboxes.Access`, each
  authorising the desktop's client id).
- `azuread_service_principal` × 3; `app_role_assignment_required` **false** on the desktop,
  **true** on both APIs.
- `azuread_service_principal_delegated_permission_grant` × 3 (tenant-wide admin consent: desktop →
  Graph with the four claim values; desktop → Hub with `Hub.Access`; desktop → broker with
  `Sandboxes.Access`); a fourth for ADO when asked.
- `azuread_app_role_assignment`: users group → Hub API (default access role), users group → broker
  API (default access role), org-admin group → Hub API (`Hub.OrgAdmin`).

#### Tests to write first (`tests/identity.tftest.hcl`, `mock_provider "azuread"`, `override_data` for the Graph lookups)

| Run | Asserts | Mutant it kills |
|---|---|---|
| `desktop_is_a_public_client_on_both_loopbacks` | redirect URIs `http://127.0.0.1/` and `http://localhost/`; no `web` block; no `password` | a confidential client, or a redirect the contract's `PANACEA_APP_ENTRA_REDIRECT_HOST` switch cannot use |
| `apis_issue_v2_tokens` | Hub and broker `requested_access_token_version == 2` | v1 tokens, whose `aud`/`iss` task 16 rejects |
| `scopes_are_the_contracts` | Hub scope `Hub.Access`, broker `Sandboxes.Access`, both `type == "User"` | a renamed scope the servers reject |
| `hub_and_broker_are_separate_audiences` | two applications, distinct scope ids | one API for both (P8-D4) |
| `desktop_is_pre_authorised_and_consented` | two pre-authorisations naming the desktop; three delegated grants, Graph's claim values exactly `openid profile offline_access email` | a consent prompt at first sign-in; a missing `offline_access` (no refresh token) |
| `graph_ids_come_from_the_directory` | the Graph `resource_app_id` equals the overridden data source's value | a transcribed GUID |
| `assignment_is_required_on_the_apis_only` | APIs true, desktop false; users group assigned to both APIs | anyone in the tenant using the APIs; or the desktop blocked before it can show "not assigned" |
| `org_admin_role_is_assigned` | `Hub.OrgAdmin` on the Hub API, assigned to `org_admin_group_object_id` | no org admin able to erase (ADR-H1 G6) |
| `email_is_an_optional_claim_on_the_hub` | the Hub's `optional_claims.access_token` has `email` | Hub invites keyed on a missing claim |
| `names_carry_the_environment_outside_prod` | dev → `PanaceaApp Desktop (dev)`; prod → `PanaceaApp Desktop` | dev and prod colliding in one tenant |
| `ado_only_when_asked` | `request_azure_devops = false` → no ADO access or grant | a consent the owner did not approve |
| `no_secret_anywhere` | no `azuread_application_password` or certificate resource | a secret added "for testing" |

#### Steps

- [ ] **Step 1:** Re-read Hub contract §6.5; if it changed, it wins for the desktop and Hub API
  rows.
- [ ] **Step 2:** Variables, locals (**fresh UUIDs** replace the three above — a scope id copied
  from a document is one someone else may also have copied), tests; validate fails.
- [ ] **Step 3:** Data sources, resources, outputs until the twelve runs pass for both tfvars.
- [ ] **Step 4:** tfvars hold only `environment`; lock file; guards; commit.

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/stacks/identity   # validate: 1 ok, 0 failed (12 runs × 2 tfvars)
bash infra/scripts/guards.sh                                      # guards: 9 of 9 passed
```

#### What this task does not claim

- That the running identity may grant tenant-wide consent to Microsoft Graph delegated scopes — in
  Entra that needs a privileged administrator role (**from training knowledge, UNVERIFIED** which
  one); the owner input "app-registration rights" must include it, or the owner consents by hand
  once.
- That Entra accepts `http://127.0.0.1/` as a registered loopback redirect and matches any port
  (the Hub contract's own unverified item, `:549-551`).
