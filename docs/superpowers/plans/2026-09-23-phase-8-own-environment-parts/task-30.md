### Task 30: `infra-plan` and `deploy` — written, guarded, and not registered

These two pipelines are the only things in this phase that would touch Azure, and **neither runs
until the owner provides the environment** (owner decisions `:52-56`; the plan's header). They are
written now so the first apply is a reviewed pipeline run, not a script someone types (Panacea's
engine is a 6,406-line script with Terraform as one step inside it, survey §1.12 `:235-238`).

Three Panacea lessons shape them: a pipeline **refuses** a wrong branch/environment pairing before
it does anything (`P:azure-pipelines-dev.yml:78-86`, survey `:229-232`); a Microsoft-hosted agent
"CANNOT reach a private API server or a private-endpoint-only registry" (`P:deploy/ado-selfhosted-agent.tf:5-9`),
so plan and deploy run **in the VNet**; and the service connection is **OIDC**, never a secret
(survey §2.2 `:315`).

A fourth rule comes from review C3 and I8: **a pull request can never use a write identity.** A PR
build runs the PR branch's own YAML and scripts, so everything in this repository, the estate
guard and `guards.sh` included, can be edited by whoever opens the PR. **The guards are hygiene,
not a security boundary.** The boundary is held in Azure and ADO, where a PR cannot reach:
- PR builds run on the read-only **plan** connection.
- The write connections carry ADO branch control and pipeline permissions. They are **infra**
  (OpenTofu apply), **cluster** (Helm) and **image push** (task 27's `images.yml`).

The plan's "Deploy identities" table lists every principal, what it holds and at what scope.

**Files:**
- `infra/terraform/modules/ci-pool/{versions,main,variables,outputs}.tf`, `tests/ci-pool.tftest.hcl` (new)
- `infra/terraform/stacks/foundation/{main,variables,outputs}.tf` and both tfvars — the `ci_pool` module call (modify, after task 07)
- `pipelines/infra-plan.yml`, `pipelines/deploy.yml` (new)
- `pipelines/scripts/estate-guard.sh`, `estate-guard_test.sh` (new)
- `pipelines/scripts/lib/where.sh` — `is_pipeline`, the one rule for "pipeline or workstation", sourced by `estate-guard.sh` and `tf-stack.sh` (plan re-review b03 I-2; new)
- `pipelines/scripts/tf-stack.sh`, `tf-stack_test.sh` — init/plan/apply one stack with the partial backend; plan with `-lock=false`, init with `-lockfile=readonly`; **runs `estate-guard.sh` first on every subcommand and every path**, pipeline or workstation (review I3; new)
- `infra/rbac/cluster-deployer.json`, `infra/rbac/sandboxes-deployer.json` — the cluster identity's two custom role definitions, which the owner creates (review I8; new)
- `infra/rbac/plan-reader.json` — the plan identity's custom role, `panacea-app-plan-reader`, which the owner creates (plan re-review M-2; new)
- `infra/rbac/grant-pipeline-roles.sh`, `grant-pipeline-roles_test.sh` — the owner's script that grants the cluster and push identities their roles once `foundation` has made the cluster and the registry. It is the only place those grants are made (plan re-review I-2, I-3; new)
- `pipelines/scripts/cluster-kinds_test.sh` — the two roles checked against what the charts render (new)
- `pipelines/scripts/helm-values.sh`, `helm-values_test.sh` — OpenTofu outputs → `--set` flags (new)
- `pipelines/scripts/api-server-cidrs.sh`, `api-server-cidrs_test.sh` — the API server's addresses → `--set apiServer.cidrs=…` for a chart in `cidr` mode (review I3; new)
- `pipelines/templates/tools.yml` — `kubelogin` (modify). No `psql`: no pipeline identity is a
  database administrator. The database bootstrap is a human step (task 31 §7, review I8)

#### Contract — `modules/ci-pool` (Managed DevOps Pool in the `agents` subnet)

```hcl
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
  description = "The foundation resource group."
}
variable "agents_subnet_id" {
  type        = string
  description = "modules/network subnet_ids.agents (delegated to Microsoft.DevOpsInfrastructure/pools)."
}
variable "ado_organization_url" {
  type        = string
  description = "https://dev.azure.com/<org> (owner input)."
  validation {
    condition     = startswith(var.ado_organization_url, "https://dev.azure.com/")
    error_message = "ado_organization_url must be an https://dev.azure.com/<org> URL."
  }
}
variable "ado_projects" {
  type        = list(string)
  description = "ADO projects allowed to use the pool (owner input)."
}
variable "max_concurrency" {
  type        = number
  description = "Agents running at once."
  default     = 2
}
variable "vm_size" {
  type        = string
  description = "Agent VM size."
  default     = "Standard_D4s_v5"
}
variable "tags" {
  type        = map(string)
  description = "Tags."
}

output "pool_name" { value = azurerm_managed_devops_pool.this.name }
output "pool_id" { value = azurerm_managed_devops_pool.this.id }
```

Resources (azurerm 4.81.0 has all three): `azurerm_dev_center.this`,
`azurerm_dev_center_project.this` (the pool's required `dev_center_project_id`), and
`azurerm_managed_devops_pool.this` with `maximum_concurrency`, `azure_devops_organization {
organization { url, projects, parallelism = var.max_concurrency } }`, `stateless_agent {}` (a fresh
VM per job: no state leaks between runs), and `virtual_machine_scale_set_fabric { sku_name =
var.vm_size, subnet_id = var.agents_subnet_id, image { well_known_image_name =
"ubuntu-24.04/latest" } }` — **the well-known image name is from training knowledge, UNVERIFIED**.
Foundation composes it as `module "ci_pool"` with `count = var.ci_pool.enabled ? 1 : 0` and a
`ci_pool = { enabled, ado_organization_url, ado_projects, max_concurrency }` variable. **Whether
Managed DevOps Pools are enabled in KPMG's ADO organisation is UNVERIFIED** (survey §5, `:465-468`);
the fallback is a VMSS agent in the same subnet, Panacea's pattern (`P:deploy/ado-selfhosted-agent.tf`).

**The agents' egress is not this module's** (review C2). Every subnet has default outbound
access off (task 03), so the pool reaches `dev.azure.com`, the OpenTofu registry, the Helm repos
and ARM only because `snet-agents` is one of `modules/network`'s four `egress_subnets`: NAT-associated
in nat mode (task 03), routed through the firewall with `ci_agent_allowed_fqdns` from the agents
subnet only in firewall mode (task 07). The API server's authorised ranges hold the environment's
egress /32 in either mode (task 07), which is the address the agents present. This module adds
nothing to that path and must not add a public IP of its own.

#### Contract — `estate-guard.sh` (complete behaviour)

```bash
#!/usr/bin/env bash
# pipelines/scripts/estate-guard.sh <environment> <plan|deploy> [<stack>…]
# Called by tf-stack.sh before every subcommand, by grant-pipeline-roles.sh, by deploy's
# guard stage, and first in every workstation section of the task 31 runbook (review I3).
# WHICH PATH (plan re-review b03 I-2: an error must land on the STRICTER path). The job is a
# pipeline if TF_BUILD=True OR any of BUILD_SOURCEBRANCH, SYSTEM_JOBID, AGENT_ID is set and
# non-empty (ADO sets all four on every job, training knowledge, UNVERIFIED). It is a
# workstation only when ALL FOUR are unset or empty. On the pipeline path an empty
# BUILD_SOURCEBRANCH is a refusal, never "no branch". The one function that decides this,
# `is_pipeline`, lives in pipelines/scripts/lib/where.sh, which tf-stack.sh sources too, so the
# two scripts cannot disagree about which path a job is on.
# Refuses (exit 1, one sentence) unless ALL hold:
#   - <environment> is dev or prod
#   - ARM_SUBSCRIPTION_ID is set and non-empty (azurerm 4.x targets it; nothing falls back
#     to whatever `az` has selected)
#   - THE POSITIVE FINGERPRINT, on both of the guard's paths: the subscription ARM_SUBSCRIPTION_ID names
#     carries the tag panacea-app-estate=<environment>, which the owner sets at handover
#     (task 31 §1). Read with
#       az rest --method get --url "https://management.azure.com/subscriptions/$ARM_SUBSCRIPTION_ID?api-version=2022-12-01" --query 'tags."panacea-app-estate"' -o tsv
#     A missing tag, a tag naming the other environment, or a failed read is a refusal,
#     never a skip. A Panacea subscription never carries the tag, and no id is committed
#     (guard G2 holds)
#   - pipeline: ARM_SUBSCRIPTION_ID equals the pipeline variable PANACEA_APP_<ENV>_SUBSCRIPTION_ID,
#     which must be set (catches the dev connection pointed at prod)
#   - workstation: ARM_SUBSCRIPTION_ID equals `az account show --query id -o tsv`, so the
#     subscription OpenTofu targets and the one the runbook's next `az` command hits agree
#   - pipeline, deploy only: BUILD_SOURCEBRANCH is allowed for it:   dev  ← refs/heads/dev | refs/heads/main
#                                                          prod ← refs/heads/main only
#     (plan mode accepts any branch, refs/pull/* included: that is what PR validation is,
#      and the plan connection can only read — review C3; a workstation has no branch, and
#      its bound is the operator's own rights plus the fingerprint)
#   - infra/terraform/stacks/<stack>/<environment>.tfvars exists for every stack named
#     (default in a pipeline: the pipeline's `stacks` parameter; on a workstation with no
#     stack named — the runbook's section-start call, grant-pipeline-roles.sh — the rule checks
#     nothing, and an unset list is not an error under `set -u`; tf-stack.sh always names its
#     stack, so the stack-bearing call is checked — plan re-review b03 M-1)
#   - no infra/terraform/**/terraform.tfvars exists (guard G1, again, at the last moment)
#   - deploy only, on both paths: no unedited placeholder. Neither a named stack's
#     <environment>.tfvars nor any deploy/values/<environment>/*.yaml holds `.invalid` as a
#     TLD (the regex \.invalid\b, matched after stripping each line's `#` comment). A match is
#     a refusal naming file:line and the value (review m16). Task 07's placeholders
#     (dns_zone_name, alert_emails) use the RFC 2606 TLD so they cannot reach someone else's
#     domain. Whether Azure DNS would refuse such a zone at apply is UNVERIFIED either way,
#     and if it accepted one, cert-manager would fail later and less clearly. So the refusal
#     is the guard's, before any apply. Plan mode does not check: a PR plans the committed
#     placeholders, and a plan creates nothing
#   - every provider source is allowlisted (guard G9, again, at the last moment)
#   - pipeline, deploy only: every consumed pipeline resource (the infra-plan run, the images run)
#     has a source branch allowed for <environment> by the rule above. A run from
#     refs/pull/* or a feature branch is refused, naming the resource and its branch.
#     Read from RESOURCES_PIPELINE_<ALIAS>_SOURCEBRANCH (variable name from training
#     knowledge, UNVERIFIED)
# Prints "estate-guard: <environment> on <branch|workstation> in …<last 4 of the subscription>,
# tagged panacea-app-estate=<environment> — ok" and exits 0.
```

**Why a tag, and why this read** (review I3). The subscription-variable check compares two
values from the same ADO project, so it catches a dev/prod swap but not a connection, or an
operator's shell, pointed at a Panacea subscription. The operator runs Panacea day to day, so a
shell whose `ARM_SUBSCRIPTION_ID` or `az` context is Panacea's is the likely accident, and
bootstrap, the subscription-scoped budget (task 04) and role assignments would land there. The
tag is a positive claim only the owner makes, on our subscription only. The read is the
subscription's own `GET`, not `az tag list` (which the review suggested): `az tag list
--resource-id /subscriptions/…` needs `Microsoft.Resources/tags/read` **at subscription scope**,
which the infra identity does not hold (its Contributor is on `rg-panacea-app-<env>`), while the
subscription `GET` needs only `Microsoft.Resources/subscriptions/read`: the plan identity's
Reader includes it, and so does the infra identity's Cost Management Contributor on the
subscription (task 04). **That both roles include that action, and that the `GET` response
carries `tags`, are training knowledge, UNVERIFIED**; task 31 §4's first plan and first apply
run the guard on the plan and infra connections and record what each read. If a connection cannot read it, the guard refuses on that
connection, and the fix is a one-action custom role, never a skipped check. Removing or moving
the tag needs `Microsoft.Resources/tags/write` at subscription scope, which no pipeline identity
holds.

The consumed-resource rule closes a gap that the plan connection alone leaves open (review C3).
A PR build can publish an artefact named `plan-<stack>` or `image-digests`, and if an approver
picked that run, `deploy` would apply the PR author's plan or run the PR author's image. The
same rule sits in `helm-values.sh`: it refuses a digest whose registry is not the environment's
ACR, the `foundation` stack's `acr_login_server` output (task 07).

#### Contract — the two pipelines

`pipelines/infra-plan.yml` — `trigger: none`; runnable by hand and as PR validation **once
registered**; `pool: name: $(PANACEA_APP_CI_POOL)` (the Managed DevOps Pool); parameters
`environment` (`dev`|`prod`) and `stacks`. The default is `foundation`, `data` and `workloads`, in
order. `bootstrap` is operator-run (task 31 §2). `identity` is **owner-run only** (task 09): the
plan identity holds no Microsoft Graph permission, so it cannot refresh that stack's Entra
objects. In a pipeline (`is_pipeline`, the estate guard's rule: `TF_BUILD=True` or any ADO agent
variable set) `tf-stack.sh` refuses either name. On a workstation it
refuses `bootstrap` (local state, task 31 §2, which runs the guard itself) and runs `identity` for
the owner, after the guard like every other path, so the owner's own identity run is fingerprinted
too (review I3). Steps: tools → the estate guard → per stack,
`tf-stack.sh plan <stack> <env>`. That runs `tofu init -lockfile=readonly
-backend-config=infra/terraform/backends/<env>.hcl -backend-config=storage_account_name=$(…)
-backend-config=resource_group_name=$(…) -backend-config=key=<stack>.<env>.tfstate`, then
`tofu plan -lock=false -var-file=<env>.tfvars -out=<stack>.tfplan -input=false`, then
`tofu show -no-color` → artefacts **`plan-<stack>`** (binary plan and text).
- `-lockfile=readonly` fails `init` when a provider is not already in the committed lock file.
  This is the second half of guard G9: an implicit `hashicorp/external` cannot be fetched on the
  way in.
- `-lock=false` is required because the plan identity can read the state blob but cannot take a
  lease on it.
- A plan made without the lock is still safe to apply. `tofu apply <planfile>` refuses a plan
  whose recorded state lineage or serial no longer matches (**from training knowledge,
  UNVERIFIED**). Task 31 §6 checks that refusal on purpose once.
- **A workstation `apply <stack> <env>`** (the owner's `identity`, §4's workstation foundation
  pass) applies the `<stack>.tfplan` that the same workstation's `tf-stack.sh plan <stack> <env>`
  just wrote, and nothing else: it never re-plans, and never reads a pipeline artefact. No
  `<stack>.tfplan` → exit 1 naming it, before `tofu apply` runs (plan re-review b03 M-2). The
  24 h age rule of stage 3 is the pipeline's; on a workstation, the stale-plan refusal above is the
  bound.

**Authentication: the plan connection.** The `AzureCLI@2` task runs on
`$(PANACEA_APP_<ENV>_PLAN_SERVICE_CONNECTION)` with `addSpnToEnvironment: true`. It exports
`ARM_USE_OIDC=true`, `ARM_CLIENT_ID`, `ARM_TENANT_ID`, `ARM_SUBSCRIPTION_ID`, and `ARM_OIDC_TOKEN`
from the task's `idToken`. **The variable names the task exposes are from training knowledge,
UNVERIFIED.** No secret is read anywhere.

The plan identity holds **Reader** on the subscription, **Storage Blob Data Reader** on the
state container and the custom role **`panacea-app-plan-reader`** on `rg-panacea-app-<env>`, and
nothing else. It is in no Entra group and holds no Graph, Key Vault data-plane or AKS data-plane
role. This is the only connection a PR build can reach, so a hostile PR can read the estate's
configuration and its state, never change either (review C3). State holds no secret by
construction (P8-D10, P8-D11, guard G8). A PR build still runs on the pool inside the VNet; the
plan's "Deploy identities" residual 5 declares that network position.

Reader is not enough for every refresh. Some azurerm reads are POST actions that Reader lacks:
AKS `listClusterUserCredential`, a storage account's `listKeys`, a registry's `listCredentials`.
The AKS one is needed from the first `foundation` PR plan: azurerm's cluster read calls it on
every refresh (**training knowledge, UNVERIFIED**). So `infra/rbac/plan-reader.json` exists from
day one (plan re-review M-2), with exactly one action,
`Microsoft.ContainerService/managedClusters/listClusterUserCredential/action`, and
`AssignableScopes` naming the `rg-panacea-app-<env>` groups only, never a subscription. With local accounts off (P8-D2) the user
credential is a kubeconfig that names `kubelogin` and holds no token, key or certificate secret.
**That it holds none is UNVERIFIED:** task 31 §6 decodes the credential the plan identity
receives and records that it carries no `client-key-data` and no `token`. If it carries either,
the action comes out of the role. Holding it gives no data-plane access: the kubeconfig is useless
without an AKS data-plane role, and the plan identity holds none.

Which other actions the provider calls at plan time is **UNVERIFIED**. The first dev plan in task
31 §6 records every 403 the plan identity meets. An action that returns no secret may be added to
`plan-reader.json`, and the plan's "Deploy identities" plan row changes in the same commit. An
action that returns a secret, such as `listKeys` or `listCredentials`, is **never** added. Instead
the resource is changed so the provider has no reason to call it: shared keys off (task 02
already does this for state), ACR admin user off.

The azurerm 4.x provider registers resource providers when it starts, and that needs
`*/register/action`, which Reader lacks. So every stack's `provider "azurerm"` block sets
`resource_provider_registrations = "none"`, and task 31 §1 registers the providers by hand. **The
attribute name is from training knowledge of azurerm 4.x, UNVERIFIED.** `tofu validate`
first sees it in task 02's `bootstrap`, which rejects an unknown attribute.

`pipelines/deploy.yml` — `trigger: none`, manual only; consumes an `infra-plan` run as a pipeline
resource; parameters `environment`, `applyStacks` (bool, default `false`), `deployCharts` (bool,
default `false`), so a run does only what its operator ticked; the first `data`/`workloads` run of
an environment ticks `applyStacks` alone, and charts wait for the database bootstrap (task 31 §6,
§7, §7a; review I9). Stages:
1. **guard** — the estate guard on the plan connection, including the consumed-resource rule:
   the `infra-plan` run and the `images` run must both come from an allowed branch. It runs again
   inside every `tf-stack.sh` call of stage 3, on the infra connection, so the fingerprint is
   read by the identity that writes **OpenTofu** (review I3). **Residual, stated** (plan
   re-review b03 M-3): the writers that are not OpenTofu never run the guard — stage 4 (Helm on
   the cluster connection), stage 5, and `images.yml`'s `push` job (task 27). Their identities
   hold nothing at subscription scope (the plan's "Deploy identities"), so they could not read the
   tag. They rely on stage 1's read over the plan connection in the same run (the push job on
   none), on branch control of their connections, and on their scopes: each holds roles only on
   our cluster and our registry, both named from `stack-outputs` of our fingerprinted `foundation`
   run, and an ACR login server is globally unique. "Every path" in this plan means every
   OpenTofu path and every owner or operator workstation section, not these three.
2. **approve** — an ADO **Environment** `panacea-app-<env>` with an approval check (the owner).
3. **apply** — on `$(PANACEA_APP_<ENV>_INFRA_SERVICE_CONNECTION)`. Per stack,
   `tf-stack.sh apply <stack> <env>` applies **the saved plan artefact** from the consumed run and
   nothing newer; a plan older than 24 h is refused. `apply` takes the state lock, which the infra
   identity can do. Then **outputs**: `tofu output -json` per stack goes to the artefact
   `stack-outputs`, one file `<stack>.json` per stack (`helm-values.sh` here, and `render-sql.sh`
   in task 31 §7, read them by output name). So the cluster identity never needs to read state.
4. **cluster** — on `$(PANACEA_APP_<ENV>_CLUSTER_SERVICE_CONNECTION)`: `az aks get-credentials`
   (**without** `--admin`: local accounts are off, P8-D2), then `kubelogin convert-kubeconfig -l
   azurecli`. If `applyStacks` is false, `stack-outputs` comes from an outputs-only job that runs on
   the plan connection. Then `helm upgrade --install --atomic --wait` runs for each release in the
   order below, one at a time. Each call gets:
   - `-f deploy/values/<env>/<chart>.yaml`;
   - the `--set` flags that `helm-values.sh` produces from `stack-outputs` and from the
     `image-digests` artefact (task 27);
   - for a chart whose values set `apiServer.mode: cidr`, the flag that
     `api-server-cidrs.sh <chart> <env>` prints.

   The order:

   | # | Release | `--namespace` | Why this namespace |
   |---|---|---|---|
   | 1 | `platform` | `default` | it creates the four namespaces itself and names each object's namespace explicitly (task 11), so `--namespace` only holds the release record. Never `--create-namespace` into one of the four: Helm would then refuse to create the `Namespace` document it does not own |
   | 2 | `cert-manager` | `panacea-edge` | the upstream chart renders into `.Release.Namespace` |
   | 3 | `edge` | `panacea-edge` | the Traefik chart renders into `.Release.Namespace` (review M2) |
   | 4 | `broker` | `panacea-broker` | its own |
   | 5 | `hub` | `panacea-hub` | its own |

   `netpolcover` (task 13) renders with this exact table; a change to one is made to both in one
   commit. `cert-manager` is its own release and must return before `edge` starts: `edge`'s
   `ClusterIssuer` and `Certificate` need its CRDs mapped and its webhook serving (review C1,
   task 13). A failed release stops the stage; nothing after it is attempted. `hub` is last on
   purpose: it is the only release that needs the database bootstrap, so a skipped task 31 §7
   leaves the other four installed and fails on the Hub alone. When the `hub` release fails, the
   step prints `kubectl logs job/hub-migrate -n panacea-hub --tail=50` before it exits (the Job
   survives the rollback, since its delete policy is `before-hook-creation`, task 29; that the
   cluster identity's namespace role may read pod logs is training knowledge, UNVERIFIED).
5. **smoke** — `infra/smoke/run.sh <env>` (task 31), on the cluster connection, **without**
   `--require-kata`. The cluster identity holds no `pods` in `panacea-sandboxes` (the
   `cluster-kinds` rule below), so S50.1 and S50.2 print `NOT RUN` here and do not fail the stage;
   they run on task 31 §8's human path (plan re-review b08-1 issue 1).

**No database stage** (review I8). The database bootstrap is `roles.sql` and the rendered
`hub-principals.sql` (task 29), run once per environment. It is task 31 §7: a **human** member of
the Postgres admin group runs it with their own Entra token, before the first `deploy` with
`deployCharts: true`. No pipeline identity is in that group, so no pipeline can read or change the
Hub's data as a database administrator. Review I9 proposed a `database` stage between `apply` and
`cluster`; it is not taken, because the stage would need a pipeline identity in the admin group,
which review I8 removed. I9's ordering is held three ways instead: `deployCharts` defaults to
`false` and the runbook's first `data`/`workloads` run leaves it off (§6); the bootstrap comes
before the first chart run (§7, then §7a); and `hub` is the last release, so a skipped bootstrap
fails the Hub alone, fail-closed, with the migrate Job's log printed. That failure **is** the
check, and the only exact one: it is the Hub's own DDL identity trying the real server. No
pipeline identity can see Postgres roles, and an ADO variable saying "bootstrapped" would be a
claim, not an observation. I9's other two asks are met in task 29 and task 33: principals bound
by object id (`pgaadauth_create_principal_with_oid`), and an idempotent `roles.sql` (ask HC-1),
with `hub migrate` naming a missing bootstrap (ask HC-2).

**The four service connections** (the plan's "Deploy identities" table has their roles). Every
check below is an ADO setting outside the repository, which task 31 §3 makes:

| Connection | Used by | ADO checks (outside the repo, so no PR can edit them) |
|---|---|---|
| `PANACEA_APP_<ENV>_PLAN_SERVICE_CONNECTION` | `infra-plan` (PR validation and manual); `deploy`'s outputs-only job | pipeline permission: `infra-plan` and `deploy` only. No branch control: it is the one connection PR builds may use, so it holds only read |
| `PANACEA_APP_<ENV>_INFRA_SERVICE_CONNECTION` | `deploy` stage 3 | pipeline permission: `deploy` only. **Branch control**: `refs/heads/main` (prod), `refs/heads/main` and `refs/heads/dev` (dev), with "verify branch protection" on and "allow unknown branches" off. The Environment approval |
| `PANACEA_APP_<ENV>_CLUSTER_SERVICE_CONNECTION` | `deploy` stages 4–5 | the same pipeline permission and branch control |
| `PANACEA_APP_<ENV>_PUSH_SERVICE_CONNECTION` | `images.yml`'s `push` job (task 27) | pipeline permission: `images` only; the same branch control |

The branch-control settings are named from training knowledge of ADO's "Branch control" check,
**UNVERIFIED**. Task 31 §3 records the real names. The estate guard's branch rule still runs, but
as hygiene. The branch control is the boundary, because a PR build can delete the guard and cannot
change the check.

#### Tests to write first

`estate-guard_test.sh`. A stub `az` first on `PATH` answers the tag read and `az account show` from per-case fixtures;
every case below not about the tag has the subscription tagged with its own environment; every
case not about the workstation or the discriminator sets `TF_BUILD=True` and `BUILD_SOURCEBRANCH`;
and every "workstation" case unsets all four of `TF_BUILD`, `BUILD_SOURCEBRANCH`, `SYSTEM_JOBID`
and `AGENT_ID`, so the runner's own environment cannot pick the path.

| Case | Expect | Mutant it kills |
|---|---|---|
| dev on `refs/heads/dev`, matching subscription | ok | a guard that refuses everything |
| **dev on `refs/heads/dev`, matching subscription, no `panacea-app-estate` tag** | refused, naming the missing tag and the subscription's last 4 characters | a guard with no positive fingerprint, so a Panacea subscription whose id was copied into the variable passes (review I3) |
| dev, subscription tagged `panacea-app-estate=prod` | refused, naming both values | a guard that checks only that the tag exists |
| dev, the tag read fails (stub `az` exits 1, as on a 403) | refused, naming the failed read | a guard that treats an unreadable tag as absent and carries on, or skips the check when `az` fails |
| workstation (all four ADO variables unset), `az account show` returns a different id from `ARM_SUBSCRIPTION_ID` | refused, naming both ids' last 4 characters | OpenTofu and the runbook's next `az` command aimed at different subscriptions |
| workstation, `ARM_SUBSCRIPTION_ID` unset | refused | azurerm left to pick up whatever subscription the operator's `az` has selected |
| workstation, `az account show` equals `ARM_SUBSCRIPTION_ID`, tagged `dev`, `dev deploy` with **no stack** | ok, and the line names `workstation` | a guard that refuses every workstation run (every other workstation case is a refusal), or a tfvars rule that exits 1 under `set -u` when no stack is named, which stops every runbook section (plan re-review b03 M-1) |
| **`TF_BUILD` unset, `BUILD_SOURCEBRANCH=refs/heads/dev`**, deploy consuming an `infra-plan` run whose source branch is `refs/pull/12/merge` | refused, naming the resource and its branch | a discriminator on `TF_BUILD` alone, which sends a deploy job lacking it down the workstation path and skips the consumed-resource rule (plan re-review b03 I-2) |
| `TF_BUILD` and `BUILD_SOURCEBRANCH` unset, `SYSTEM_JOBID` set, `dev deploy foundation` | refused, naming the empty source branch | a pipeline job with no branch treated as "no branch to check", or as a workstation |
| prod on `refs/heads/dev` | refused, naming the branch | a feature branch deploying prod |
| prod on `refs/heads/main`, subscription of dev | refused, naming both ids' last 4 characters only | the dev service connection pointed at prod (Panacea's same-AKS-name collision class) |
| a stack missing `<env>.tfvars` | refused | a plan that silently falls back to defaults |
| a `terraform.tfvars` anywhere | refused | the auto-loaded-tfvars trap |
| plan mode, dev, on `refs/pull/12/merge` | ok | PR validation of `infra-plan` that can never pass |
| deploy on `refs/heads/dev` consuming an `infra-plan` run whose source branch is `refs/pull/12/merge` | refused, naming the resource and its branch | a PR author's plan artefact applied by the infra identity (review C3) |
| a module requiring `hashicorp/external` | refused, naming guard G9 | a provider that runs a program at plan time |
| `dev deploy foundation`, whose fixture `dev.tfvars` holds task 07's `dns_zone_name = "dev.panacea-app.invalid"` | refused, naming `dev.tfvars:<line>` and the value | an unedited placeholder reaching apply, where Azure DNS may accept the zone and cert-manager fails later (review m16) |
| workstation, the same tfvars, `dev deploy foundation` (as `tf-stack.sh apply` calls it) | refused, the same sentence | a placeholder check on the pipeline path only, while a workstation `apply` goes through |
| `dev plan foundation` on `refs/pull/12/merge`, the same tfvars | ok | a placeholder check in plan mode, which fails every PR on the committed placeholders |
| `dev deploy` whose fixture `deploy/values/dev/edge.yaml` holds a host ending `.invalid` | refused, naming the file and line | a check that reads the tfvars and forgets the chart values |
| `dev deploy foundation`, where `.invalid` appears only after a `#` in `dev.tfvars` | ok | a guard that refuses a commented example |
| deploy on `refs/heads/dev` consuming an `images` run whose source branch is `refs/pull/12/merge` (the `infra-plan` run from `refs/heads/dev`) | refused, naming `images` and its branch | a guard that checks only the `infra-plan` alias, so a PR-built `image-digests` naming an older digest in the environment's own registry passes both the guard and `helm-values.sh`'s registry check (plan re-review M-1) |

`tf-stack_test.sh` puts first on `PATH`, with `TOFU_BIN` unset, a stub named for the binary `tf-stack.sh` calls, `tofu` (OD-32), that records its arguments, and stops before the first case unless `command -v tofu` resolves to that stub, so a real engine on the host is never reached; beside it a stub `terraform` that records too and must have recorded **nothing** after every case; and a stub
`estate-guard.sh` (found through `PANACEA_APP_ESTATE_GUARD`, default the real script beside it)
that records its arguments and exits with a per-case status. Cases run with `TF_BUILD=True` and
`BUILD_SOURCEBRANCH` set unless they say "workstation" (all four ADO variables unset). The first case is the control for the rest: a `tofu` stub that recorded nothing there fails the test, because a stub the script never calls lets every "recorded **nothing**" below pass either way:

| Case | Expect | Mutant it kills |
|---|---|---|
| `plan foundation dev` | the `tofu` stub recorded `init` then `plan`, and an empty record fails the test; `init` received `-lockfile=readonly`, `plan` received `-lock=false`; the `terraform` stub recorded nothing | a plan that tries to take a lease it cannot hold, or an `init` that fetches an unlisted provider; a `tf-stack.sh` that calls `terraform` (OD-32); a stub named for a binary the script never calls, under which every "recorded **nothing**" row below passes either way |
| `apply foundation dev` | the `tofu` stub recorded an `apply`, and it received no `-lock=false` | an apply that races another apply, or one that never reached `tofu` and so received nothing at all |
| `plan identity dev`, `plan bootstrap dev` | exit 1, naming the stack as owner-run or operator-run | the identity stack reached from a pipeline (review I8) |
| `plan foundation dev`, `apply foundation dev`, and workstation `plan foundation dev`, each with the guard stub exiting 1 | exit 1, and the `tofu` stub recorded **nothing** | the workstation path, or any subcommand, reaching `tofu` without the guard (review I3) |
| workstation `apply foundation dev`, the guard stub exiting 1 | exit 1, and the `tofu` stub recorded **nothing** | a `tf-stack.sh` that guards workstation `plan` but not workstation `apply` — the owner's `apply identity` and §4's workstation foundation apply (plan re-review b03 M-2) |
| workstation `apply foundation dev`, the guard stub exiting 0, `foundation.tfplan` present from the `plan` just run | `apply` received `foundation.tfplan` and no other plan source; with no `foundation.tfplan` → exit 1 naming it, `tofu` recorded no `apply` | a workstation apply that re-plans and applies unreviewed changes, or reaches for a pipeline artefact |
| `TF_BUILD` unset, `AGENT_ID` set, `plan identity dev` | exit 1, naming the stack as owner-run | a `tf-stack.sh` with its own `TF_BUILD`-only test, which lets an agent job run the owner-only stack while the guard (via `where.sh`) calls it a pipeline (plan re-review b03 I-2) |
| `apply foundation dev` with the guard stub exiting 0 | the guard received `dev deploy foundation`, and only after it did the `tofu` stub record its first call | the guard called in `plan` mode on the write path, which skips the branch and consumed-resource rules |
| workstation `plan identity dev` | the guard received `dev plan identity`, then `init` and `plan` ran; workstation `plan bootstrap dev` still exits 1 | the owner's identity run left outside the guard, or bootstrap's local-state run forced through the partial backend |

`cluster-kinds_test.sh` checks the cluster identity's two custom roles, `infra/rbac/cluster-deployer.json`
and `infra/rbac/sandboxes-deployer.json`, against what the charts actually render. It runs
`helm template` over the five releases of the table above with the dev values and asserts:
- every cluster-scoped kind rendered has a matching data action in `cluster-deployer.json`, and
  that file names no namespaced kind and no `*`;
- every object rendered into `panacea-sandboxes` is a kind `sandboxes-deployer.json` names;
- neither file ever names `secrets`, `pods`, `pods/exec` or `serviceaccounts/token` as a
  **resource**;
- every release's namespace is one of the four RBAC Admin namespaces that
  `grant-pipeline-roles.sh` declares (it reads the script's `RBAC_ADMIN_NAMESPACES` array, so the
  two cannot drift).

This kills a chart change that needs a grant nobody gave, and a role file that **names** a token
resource. It does **not** prove the cluster identity cannot read users' tokens (plan re-review
I-1). The `broker` release renders a `Role` and `RoleBinding` into `panacea-sandboxes` whose
rules name `pods` and `secrets` (task 29). Kubernetes lets an identity create such a Role only if
it already holds those rights or holds `escalate` on `roles`, and bind it only with `bind` on
`roles` (**training knowledge of Kubernetes RBAC with the AKS Azure RBAC webhook, UNVERIFIED**).
`sandboxes-deployer.json` therefore carries the `escalate` and `bind` data actions on
`rbac.authorization.k8s.io/roles` in that namespace, and with them the identity can write itself
a Role that reads every Secret there. So the test prints a second line,
`cluster-kinds: sandboxes RBAC verbs: <verbs>`, naming every RBAC verb in `sandboxes-deployer.json`,
and task 31 §7a records whether the `broker` release installs with them and fails without them.

The first printed line, `cluster-kinds: the cluster identity can bind: <kinds>`, lists the
ClusterRole and ClusterRoleBinding objects the upstream cert-manager and Traefik charts bring. This
is what it means:
- To create those objects, Kubernetes' RBAC escalation check requires `bind` (or `escalate`) on
  the identity. That lets the identity bind `cluster-admin` to itself (**training knowledge of
  Kubernetes RBAC with the AKS Azure RBAC webhook, UNVERIFIED**).
- So the split removes Azure RBAC, state and database-administrator rights from the cluster
  identity. It does **not** prove that identity cannot reach cluster-admin, and it does not stop
  it reaching users' tokens: it deploys the broker, so it can act as the broker (above, and task
  07).
- The boundary around both is the ADO checks above. The stricter option, cluster-scoped RBAC
  installed by an owner-run release, is owner question Q4. Q4 would close the cluster-admin route
  only; the token route through the broker stays for as long as the pipeline deploys the broker.

`grant-pipeline-roles.sh <env> [--dry-run]` is run by the **owner** under their own credentials,
never by a pipeline (task 31 §4a). Before anything else it runs `estate-guard.sh <env> deploy`
(the workstation path: `ARM_SUBSCRIPTION_ID` set, equal to the `az` context, and tagged
`panacea-app-estate=<env>`), so no assignment can land in a Panacea subscription the owner's
shell happens to have selected (review I3). It reads the two object ids from
`PANACEA_APP_<ENV>_CLUSTER_PRINCIPAL_OBJECT_ID` and `PANACEA_APP_<ENV>_PUSH_PRINCIPAL_OBJECT_ID`,
looks up the cluster and registry ids with `az aks show` and `az acr show` in
`rg-panacea-app-<env>`, and makes exactly the eight assignments of task 07's list: AcrPush on the
registry, Cluster User Role on the cluster, RBAC Admin on each of the four namespaces in
`RBAC_ADMIN_NAMESPACES` (`default`, `panacea-edge`, `panacea-broker`, `panacea-hub`),
`panacea-app-cluster-deployer` on the cluster, `panacea-app-sandboxes-deployer` on
`<cluster id>/namespaces/panacea-sandboxes`. For each it runs `az role assignment list` first
and creates only what is missing (`az role assignment create --assignee-object-id …
--assignee-principal-type ServicePrincipal`), printing `grant: <role> @ <scope> — created|present`.
`--dry-run` prints the eight lines and creates nothing. The namespaced assignments are made before
`platform` creates the namespaces. That Azure accepts a namespace scope that does not exist yet is
**training knowledge, UNVERIFIED**; task 31 §4a records it. A re-created cluster has a new id, so
the owner runs the script again (task 31 §4a).

`grant-pipeline-roles_test.sh` puts a stub `az` on `PATH` that records its arguments:

| Case | Expect | Mutant it kills |
|---|---|---|
| `--dry-run dev` | exactly the eight lines above; none names `Azure Kubernetes Service RBAC Cluster Admin`; no RBAC Admin scope ends in `/namespaces/panacea-sandboxes` | the old single-principal grant, which reads every Secret in the cluster (review I8); RBAC Admin widened to the namespace that holds users' tokens (moved from task 07's `cluster_identity_is_not_cluster_admin`) |
| `--dry-run dev` | AcrPush's only principal is the push id, and the push id has no other line | the push identity given more than the registry (moved from task 07) |
| `dev`, the stub reporting AcrPush already present | seven `create` calls, and one `present` line | a script that fails with a conflict, or duplicates a grant, on its second run |
| `dev`, the cluster and push ids equal | exit 1 before any `az` call, naming both variables | one principal holding both rows (review I8) |
| `dev`, an object id unset | exit 1 before any `az` call, naming the variable | a grant to an empty principal |
| `dev`, the guard stub (`PANACEA_APP_ESTATE_GUARD`) exiting 1 | exit 1, and the `az` stub recorded no `role assignment` call | role assignments made in whatever subscription the owner's shell had selected, Panacea's included (review I3) |

`helm-values_test.sh`: a fixture `tofu output -json` for each stack and a digests file →
exactly the expected `--set` list per release (every id the charts need, nothing else; kills a
chart deployed with an empty client id). `cert_manager_client_id` appears twice, as
`cert-manager`'s `identity.clientId` and `edge`'s `certManagerIdentity.clientId`: kills an issuer
and a service account that name different identities. A digests file naming an image outside
the fixture's `acr_login_server` exits 1, naming the image. This kills a PR-published
`image-digests` artefact that points the cluster at a registry the environment does not own
(review C3).

`api-server-cidrs.sh <chart> <env>` (review I3): reads `apiServer.mode` from
`deploy/values/<env>/<chart>.yaml`; for `entity` prints nothing; for `cidr` runs
`kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes -o jsonpath=…`
(no `jq`), and prints one `--set apiServer.cidrs={<a>/32,<b>/32}` flag (IPv6 as `/128`). **No
address, or a `kubectl` failure, exits non-zero naming the chart** — it never prints an empty list,
and the chart schemas refuse one anyway. `api-server-cidrs_test.sh` stubs `kubectl` on `PATH`:

| Case | Expect | Mutant it kills |
|---|---|---|
| `entity` mode | no output, exit 0 | a `cidr` flag passed to a chart in `entity` mode |
| `cidr` mode, two addresses | `--set apiServer.cidrs={10.1.2.3/32,10.1.2.4/32}` | a single address when the API server has two |
| `cidr` mode, no address | exit 1, message names the chart | an empty list reaching `helm` (a peerless rule if a schema is ever loosened) |
| `cidr` mode, `kubectl` fails | exit 1 | a deploy that continues with no API server allow |

`modules/ci-pool/tests/ci-pool.tftest.hcl`: `agents_are_stateless` (`stateless_agent` present,
`stateful_agent` absent — kills credentials left on an agent between runs);
`pool_lives_in_the_agents_subnet`; `bad_org_url_is_rejected`.

And task 28's `lint.sh` now finds both files and passes its "starts with the estate guard" rule.

#### Steps

- [ ] **Step 1:** `ci-pool` module and tests; the foundation call; validate both.
- [ ] **Step 2:** `estate-guard.sh`, `tf-stack.sh`, `helm-values.sh`, `api-server-cidrs.sh` and their tests.
  Mutation checks for review I3, each verified applied and red: delete the tag comparison →
  the no-tag case; compare the tag's presence but not its value → the `prod`-tagged case; make a
  failed tag read fall through → the failed-read case; drop the guard call from `tf-stack.sh`'s
  workstation branch only → the guard-first case's workstation row. For plan re-review b03:
  guard workstation `plan` but not workstation `apply` → the workstation-apply guard row (M-2);
  make `is_pipeline` test `TF_BUILD` alone → the `TF_BUILD`-unset consumed-resource case and
  tf-stack's `AGENT_ID` case (I-2); treat an empty branch on the pipeline path as a pass → the
  `SYSTEM_JOBID` case; make the tfvars rule `exit 1` on an empty stack list → the no-stack
  workstation `ok` case (M-1). For review m16: delete the placeholder rule → the `.invalid`
  deploy case; apply it on the pipeline path only → its workstation row; apply it in plan mode
  too → the `refs/pull` plan row; scan tfvars but not `deploy/values/` → the values row; match
  before stripping comments → the comment-only row. For OD-32: make `tf-stack.sh` call `terraform` for any one subcommand → the `terraform` stub's after-every-case check; name the test's own stub `terraform` again → the `command -v tofu` stop and the first case's control, the two that go red where every "recorded **nothing**" row would have stayed green.
- [ ] **Step 3:** `infra/rbac/cluster-deployer.json`, `infra/rbac/sandboxes-deployer.json`,
  `infra/rbac/plan-reader.json`, `grant-pipeline-roles.sh` and its test, and
  `cluster-kinds_test.sh` (needs the charts of tasks 11–13, 26 and 29 to render).
- [ ] **Step 4:** The two pipeline files; `lint.sh`; commit. **Do not register either pipeline.**

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/ci-pool infra/terraform/stacks/foundation   # 2 ok
bash pipelines/scripts/estate-guard_test.sh        # estate-guard_test: 22 of 22 passed
bash pipelines/scripts/tf-stack_test.sh            # tf-stack_test: 9 of 9 passed
bash infra/rbac/grant-pipeline-roles_test.sh       # grant-pipeline-roles_test: 6 of 6 passed
bash pipelines/scripts/cluster-kinds_test.sh       # cluster-kinds: ok, the "can bind" line and the "sandboxes RBAC verbs" line
bash pipelines/scripts/helm-values_test.sh         # helm-values_test: ok
bash pipelines/scripts/api-server-cidrs_test.sh    # api-server-cidrs_test: 4 of 4 passed
bash pipelines/scripts/lint.sh                     # lint: 8 of 8 rules pass over 9 pipeline files
```

#### What this task does not claim

- **That either pipeline works.** Neither has run; neither can until the subscription, the service
  connection, the pool and the environment approvals exist. Task 31 is the first run, by hand.
- That the four identities get the roles in the plan's "Deploy identities" table. Those are owner
  inputs (survey §7.1 item 3), made in task 31 §2a and §4a. No pipeline identity joins the Postgres
  admin group (review I8).
- That the plan identity's Reader and `panacea-app-plan-reader` are enough for every refresh
  (task 31 §6 records the 403s), or that the AKS user credential holds no secret (task 31 §6).
- That the cluster identity cannot reach cluster-admin through Kubernetes RBAC `bind`; see
  `cluster-kinds_test.sh` above.
- That the cluster identity cannot read users' tokens. It deploys the broker, so it can act as
  the broker (plan re-review I-1). The ADO checks are the bound.
- That ADO's branch control check behaves as described. Its settings are named from training
  knowledge, UNVERIFIED.
