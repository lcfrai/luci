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

**Files:**
- `infra/terraform/modules/ci-pool/{versions,main,variables,outputs}.tf`, `tests/ci-pool.tftest.hcl` (new)
- `infra/terraform/stacks/foundation/{main,variables,outputs}.tf` and both tfvars — the `ci_pool` module call (modify, after task 07)
- `pipelines/infra-plan.yml`, `pipelines/deploy.yml` (new)
- `pipelines/scripts/estate-guard.sh`, `estate-guard_test.sh` (new)
- `pipelines/scripts/tf-stack.sh` — init/plan/apply one stack with the partial backend (new)
- `pipelines/scripts/helm-values.sh`, `helm-values_test.sh` — Terraform outputs → `--set` flags (new)
- `pipelines/templates/tools.yml` — `kubelogin`, `psql` (modify)

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

#### Contract — `estate-guard.sh` (complete behaviour)

```bash
#!/usr/bin/env bash
# pipelines/scripts/estate-guard.sh <environment>
# Refuses (exit 1, one sentence) unless ALL hold:
#   - <environment> is dev or prod
#   - BUILD_SOURCEBRANCH is allowed for it:   dev  ← refs/heads/dev | refs/heads/main
#                                             prod ← refs/heads/main only
#   - ARM_SUBSCRIPTION_ID equals the pipeline variable PANACEA_APP_<ENV>_SUBSCRIPTION_ID
#     (the service connection cannot point at another estate's subscription)
#   - infra/terraform/stacks/*/<environment>.tfvars exists for every stack being run
#   - no infra/terraform/**/terraform.tfvars exists (guard G1, again, at the last moment)
# Prints "estate-guard: <environment> on <branch> in <subscription> — ok" and exits 0.
```

#### Contract — the two pipelines

`pipelines/infra-plan.yml` — `trigger: none`; runnable by hand and as PR validation **once
registered**; `pool: name: $(PANACEA_APP_CI_POOL)` (the Managed DevOps Pool); parameters
`environment` (`dev`|`prod`) and `stacks` (default all five, in order). Steps: tools → the estate
guard → per stack, `tf-stack.sh plan <stack> <env>`, which runs `terraform init
-backend-config=infra/terraform/backends/<env>.hcl -backend-config=storage_account_name=$(…)
-backend-config=resource_group_name=$(…) -backend-config=key=<stack>.<env>.tfstate`, then `terraform
plan -var-file=<env>.tfvars -out=<stack>.tfplan -input=false`, then `terraform show -no-color` →
artefacts **`plan-<stack>`** (binary plan and text). Authentication: the `AzureCLI@2` task on
`$(PANACEA_APP_<ENV>_SERVICE_CONNECTION)` with `addSpnToEnvironment: true`, exporting
`ARM_USE_OIDC=true`, `ARM_CLIENT_ID`, `ARM_TENANT_ID`, `ARM_SUBSCRIPTION_ID` and
`ARM_OIDC_TOKEN` from the task's `idToken` — **the variable names the task exposes are from
training knowledge, UNVERIFIED**. No secret is read anywhere.

`pipelines/deploy.yml` — `trigger: none`, manual only; consumes an `infra-plan` run as a pipeline
resource; parameters `environment`, `applyStacks` (bool), `deployCharts` (bool). Stages:
1. **guard** — the estate guard.
2. **approve** — an ADO **Environment** `panacea-app-<env>` with an approval check (the owner).
3. **apply** — per stack, `tf-stack.sh apply <stack> <env>` applies **the saved plan artefact**
   from the consumed run and nothing newer; a plan older than 24 h is refused.
4. **cluster** — `az aks get-credentials` (**without** `--admin`: local accounts are off, P8-D2),
   `kubelogin convert-kubeconfig -l azurecli`; then `helm upgrade --install --atomic --wait` for
   `platform` → `edge` → `broker` → `hub`, each with `-f deploy/values/<env>/<chart>.yaml` and the
   `--set` flags `helm-values.sh` produces from `terraform output -json` of the stacks and from the
   `image-digests` artefact (task 27).
5. **database** — once per environment and idempotent: `psql` as the deploy identity (a member of
   the Postgres admin group) with an Entra token, running the Hub's `roles.sql` and the rendered
   `hub-principals.sql` (task 29) **before** the `hub` chart's first install.
6. **smoke** — `infra/smoke/run.sh <env>` (task 31).

#### Tests to write first

`estate-guard_test.sh`:

| Case | Expect | Mutant it kills |
|---|---|---|
| dev on `refs/heads/dev`, matching subscription | ok | a guard that refuses everything |
| prod on `refs/heads/dev` | refused, naming the branch | a feature branch deploying prod |
| prod on `refs/heads/main`, subscription of dev | refused, naming both ids' last 4 characters only | the dev service connection pointed at prod (Panacea's same-AKS-name collision class) |
| a stack missing `<env>.tfvars` | refused | a plan that silently falls back to defaults |
| a `terraform.tfvars` anywhere | refused | the auto-loaded-tfvars trap |

`helm-values_test.sh`: a fixture `terraform output -json` for each stack and a digests file →
exactly the expected `--set` list (every id the charts need, nothing else; kills a chart deployed
with an empty client id).

`modules/ci-pool/tests/ci-pool.tftest.hcl`: `agents_are_stateless` (`stateless_agent` present,
`stateful_agent` absent — kills credentials left on an agent between runs);
`pool_lives_in_the_agents_subnet`; `bad_org_url_is_rejected`.

And task 28's `lint.sh` now finds both files and passes its "starts with the estate guard" rule.

#### Steps

- [ ] **Step 1:** `ci-pool` module and tests; the foundation call; validate both.
- [ ] **Step 2:** `estate-guard.sh`, `tf-stack.sh`, `helm-values.sh` and their tests.
- [ ] **Step 3:** The two pipeline files; `lint.sh`; commit. **Do not register either pipeline.**

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/ci-pool infra/terraform/stacks/foundation   # 2 ok
bash pipelines/scripts/estate-guard_test.sh        # estate-guard_test: 5 of 5 passed
bash pipelines/scripts/helm-values_test.sh         # helm-values_test: ok
bash pipelines/scripts/lint.sh                     # lint: 6 of 6 rules pass over 9 pipeline files
```

#### What this task does not claim

- **That either pipeline works.** Neither has run; neither can until the subscription, the service
  connection, the pool and the environment approvals exist. Task 31 is the first run, by hand.
- That the deploy identity can join the Postgres admin group, or grant `AcrPush` and AKS roles —
  owner inputs (survey §7.1 item 3).
