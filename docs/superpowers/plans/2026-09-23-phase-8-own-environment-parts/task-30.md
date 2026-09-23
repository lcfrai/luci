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
  (Terraform apply), **cluster** (Helm) and **image push** (task 27's `images.yml`).

The plan's "Deploy identities" table lists every principal, what it holds and at what scope.

**Files:**
- `infra/terraform/modules/ci-pool/{versions,main,variables,outputs}.tf`, `tests/ci-pool.tftest.hcl` (new)
- `infra/terraform/stacks/foundation/{main,variables,outputs}.tf` and both tfvars — the `ci_pool` module call (modify, after task 07)
- `pipelines/infra-plan.yml`, `pipelines/deploy.yml` (new)
- `pipelines/scripts/estate-guard.sh`, `estate-guard_test.sh` (new)
- `pipelines/scripts/tf-stack.sh`, `tf-stack_test.sh` — init/plan/apply one stack with the partial backend; plan with `-lock=false`, init with `-lockfile=readonly` (new)
- `infra/rbac/cluster-deployer.json`, `infra/rbac/sandboxes-deployer.json` — the cluster identity's two custom role definitions, which the owner creates (review I8; new)
- `pipelines/scripts/cluster-kinds_test.sh` — the two roles checked against what the charts render (new)
- `pipelines/scripts/helm-values.sh`, `helm-values_test.sh` — Terraform outputs → `--set` flags (new)
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
access off (task 03), so the pool reaches `dev.azure.com`, the Terraform registry, the Helm repos
and ARM only because `snet-agents` is one of `modules/network`'s four `egress_subnets`: NAT-associated
in nat mode (task 03), routed through the firewall with `ci_agent_allowed_fqdns` from the agents
subnet only in firewall mode (task 07). The API server's authorised ranges hold the environment's
egress /32 in either mode (task 07), which is the address the agents present. This module adds
nothing to that path and must not add a public IP of its own.

#### Contract — `estate-guard.sh` (complete behaviour)

```bash
#!/usr/bin/env bash
# pipelines/scripts/estate-guard.sh <environment> <plan|deploy>
# Refuses (exit 1, one sentence) unless ALL hold:
#   - <environment> is dev or prod
#   - deploy only: BUILD_SOURCEBRANCH is allowed for it:   dev  ← refs/heads/dev | refs/heads/main
#                                                          prod ← refs/heads/main only
#     (plan mode accepts any branch, refs/pull/* included: that is what PR validation is,
#      and the plan connection can only read — review C3)
#   - ARM_SUBSCRIPTION_ID equals the pipeline variable PANACEA_APP_<ENV>_SUBSCRIPTION_ID
#     (the service connection cannot point at another estate's subscription)
#   - infra/terraform/stacks/*/<environment>.tfvars exists for every stack being run
#   - no infra/terraform/**/terraform.tfvars exists (guard G1, again, at the last moment)
#   - every provider source is allowlisted (guard G9, again, at the last moment)
#   - deploy only: every consumed pipeline resource (the infra-plan run, the images run)
#     has a source branch allowed for <environment> by the rule above. A run from
#     refs/pull/* or a feature branch is refused, naming the resource and its branch.
#     Read from RESOURCES_PIPELINE_<ALIAS>_SOURCEBRANCH (variable name from training
#     knowledge, UNVERIFIED)
# Prints "estate-guard: <environment> on <branch> in <subscription> — ok" and exits 0.
```

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
objects. `tf-stack.sh` refuses either name. Steps: tools → the estate guard → per stack,
`tf-stack.sh plan <stack> <env>`. That runs `terraform init -lockfile=readonly
-backend-config=infra/terraform/backends/<env>.hcl -backend-config=storage_account_name=$(…)
-backend-config=resource_group_name=$(…) -backend-config=key=<stack>.<env>.tfstate`, then
`terraform plan -lock=false -var-file=<env>.tfvars -out=<stack>.tfplan -input=false`, then
`terraform show -no-color` → artefacts **`plan-<stack>`** (binary plan and text).
- `-lockfile=readonly` fails `init` when a provider is not already in the committed lock file.
  This is the second half of guard G9: an implicit `hashicorp/external` cannot be fetched on the
  way in.
- `-lock=false` is required because the plan identity can read the state blob but cannot take a
  lease on it.
- A plan made without the lock is still safe to apply. `terraform apply <planfile>` refuses a plan
  whose recorded state lineage or serial no longer matches (**from training knowledge,
  UNVERIFIED**). Task 31 §6 checks that refusal on purpose once.

**Authentication: the plan connection.** The `AzureCLI@2` task runs on
`$(PANACEA_APP_<ENV>_PLAN_SERVICE_CONNECTION)` with `addSpnToEnvironment: true`. It exports
`ARM_USE_OIDC=true`, `ARM_CLIENT_ID`, `ARM_TENANT_ID`, `ARM_SUBSCRIPTION_ID`, and `ARM_OIDC_TOKEN`
from the task's `idToken`. **The variable names the task exposes are from training knowledge,
UNVERIFIED.** No secret is read anywhere.

The plan identity holds **Reader** on the subscription and **Storage Blob Data Reader** on the
state container, and nothing else. It is in no Entra group and holds no Graph, Key Vault
data-plane or AKS data-plane role. This is the only connection a PR build can reach, so a hostile
PR can read the estate's configuration and its state, never change either (review C3). State
holds no secret by construction (P8-D10, P8-D11, guard G8).

Reader may not be enough for every refresh. Some azurerm reads are POST actions that Reader lacks:
AKS `listClusterUserCredential`, a storage account's `listKeys`, a registry's `listCredentials`.
Which ones the provider calls at plan time is **UNVERIFIED**. The first dev plan in task 31 §6
records every 403 the plan identity meets. An action that returns no secret may be added to a
custom role `panacea-app-plan-reader`. An action that returns a secret, such as `listKeys` or
`listCredentials`, is **never** added. Instead the resource is changed so the provider has no
reason to call it: shared keys off (task 02 already does this for state), ACR admin user off.

The azurerm 4.x provider registers resource providers when it starts, and that needs
`*/register/action`, which Reader lacks. So every stack's `provider "azurerm"` block sets
`resource_provider_registrations = "none"`, and task 31 §1 registers the providers by hand. **The
attribute name is from training knowledge of azurerm 4.x, UNVERIFIED.** `terraform validate`
first sees it in task 02's `bootstrap`, which rejects an unknown attribute.

`pipelines/deploy.yml` — `trigger: none`, manual only; consumes an `infra-plan` run as a pipeline
resource; parameters `environment`, `applyStacks` (bool), `deployCharts` (bool). Stages:
1. **guard** — the estate guard, including the consumed-resource rule: the `infra-plan` run and
   the `images` run must both come from an allowed branch.
2. **approve** — an ADO **Environment** `panacea-app-<env>` with an approval check (the owner).
3. **apply** — on `$(PANACEA_APP_<ENV>_INFRA_SERVICE_CONNECTION)`. Per stack,
   `tf-stack.sh apply <stack> <env>` applies **the saved plan artefact** from the consumed run and
   nothing newer; a plan older than 24 h is refused. `apply` takes the state lock, which the infra
   identity can do. Then **outputs**: `terraform output -json` per stack goes to the artefact
   `stack-outputs`. So the cluster identity never needs to read state.
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
   task 13). A failed release stops the stage; nothing after it is attempted.
5. **smoke** — `infra/smoke/run.sh <env>` (task 31), on the cluster connection.

**No database stage** (review I8). The database bootstrap is `roles.sql` and the rendered
`hub-principals.sql` (task 29), run once per environment. It is task 31 §7: a **human** member of
the Postgres admin group runs it with their own Entra token, before the first `deploy` with
`deployCharts: true`. No pipeline identity is in that group, so no pipeline can read or change the
Hub's data as a database administrator. The ordering problem of review I9 (the Hub's first install
must come after the roles) is resolved by that runbook order, not by a stage. I9's other two asks
remain open for I9's own batch: an idempotent `roles.sql`, and a principal created by object id.

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

`estate-guard_test.sh`:

| Case | Expect | Mutant it kills |
|---|---|---|
| dev on `refs/heads/dev`, matching subscription | ok | a guard that refuses everything |
| prod on `refs/heads/dev` | refused, naming the branch | a feature branch deploying prod |
| prod on `refs/heads/main`, subscription of dev | refused, naming both ids' last 4 characters only | the dev service connection pointed at prod (Panacea's same-AKS-name collision class) |
| a stack missing `<env>.tfvars` | refused | a plan that silently falls back to defaults |
| a `terraform.tfvars` anywhere | refused | the auto-loaded-tfvars trap |
| plan mode, dev, on `refs/pull/12/merge` | ok | PR validation of `infra-plan` that can never pass |
| deploy on `refs/heads/dev` consuming an `infra-plan` run whose source branch is `refs/pull/12/merge` | refused, naming the resource and its branch | a PR author's plan artefact applied by the infra identity (review C3) |
| a module requiring `hashicorp/external` | refused, naming guard G9 | a provider that runs a program at plan time |

`tf-stack_test.sh` puts a stub `terraform` on `PATH` that records its arguments:

| Case | Expect | Mutant it kills |
|---|---|---|
| `plan foundation dev` | `init` received `-lockfile=readonly`, `plan` received `-lock=false` | a plan that tries to take a lease it cannot hold, or an `init` that fetches an unlisted provider |
| `apply foundation dev` | `apply` received no `-lock=false` | an apply that races another apply |
| `plan identity dev`, `plan bootstrap dev` | exit 1, naming the stack as owner-run or operator-run | the identity stack reached from a pipeline (review I8) |

`cluster-kinds_test.sh` checks the cluster identity's two custom roles, `infra/rbac/cluster-deployer.json`
and `infra/rbac/sandboxes-deployer.json`, against what the charts actually render. It runs
`helm template` over the five releases of the table above with the dev values and asserts:
- every cluster-scoped kind rendered has a matching data action in `cluster-deployer.json`, and
  that file names no namespaced kind and no `*`;
- every object rendered into `panacea-sandboxes` is a kind `sandboxes-deployer.json` names;
- neither file ever names `secrets`, `pods`, `pods/exec` or `serviceaccounts/token`;
- every release's namespace is one of the four RBAC Admin scopes of task 07.

This kills a chart change that needs a grant nobody gave. It also kills a role widened to read
users' tokens. The test prints one line, `cluster-kinds: the cluster identity can bind: <kinds>`,
listing the ClusterRole and ClusterRoleBinding objects the upstream cert-manager and Traefik charts
bring. This is what it means:
- To create those objects, Kubernetes' RBAC escalation check requires `bind` (or `escalate`) on
  the identity. That lets the identity bind `cluster-admin` to itself (**training knowledge of
  Kubernetes RBAC with the AKS Azure RBAC webhook, UNVERIFIED**).
- So the split removes Azure RBAC, state and database rights from the cluster identity. It does
  **not** prove that identity cannot reach cluster-admin.
- The boundary around it is the ADO checks above. The stricter option, cluster-scoped RBAC
  installed by an owner-run release, is an owner question (the plan's "Deploy identities" table).

`helm-values_test.sh`: a fixture `terraform output -json` for each stack and a digests file →
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
- [ ] **Step 3:** `infra/rbac/cluster-deployer.json`, `infra/rbac/sandboxes-deployer.json` and
  `cluster-kinds_test.sh` (needs the charts of tasks 11–13, 26 and 29 to render).
- [ ] **Step 4:** The two pipeline files; `lint.sh`; commit. **Do not register either pipeline.**

#### Acceptance

```bash
bash infra/scripts/validate.sh infra/terraform/modules/ci-pool infra/terraform/stacks/foundation   # 2 ok
bash pipelines/scripts/estate-guard_test.sh        # estate-guard_test: 8 of 8 passed
bash pipelines/scripts/tf-stack_test.sh            # tf-stack_test: 3 of 3 passed
bash pipelines/scripts/cluster-kinds_test.sh       # cluster-kinds: ok, and the "can bind" line
bash pipelines/scripts/helm-values_test.sh         # helm-values_test: ok
bash pipelines/scripts/api-server-cidrs_test.sh    # api-server-cidrs_test: 4 of 4 passed
bash pipelines/scripts/lint.sh                     # lint: 7 of 7 rules pass over 9 pipeline files
```

#### What this task does not claim

- **That either pipeline works.** Neither has run; neither can until the subscription, the service
  connection, the pool and the environment approvals exist. Task 31 is the first run, by hand.
- That the four identities get the roles in the plan's "Deploy identities" table. Those are owner
  inputs (survey §7.1 item 3). No pipeline identity joins the Postgres admin group (review I8).
- That the plan identity's Reader is enough for every refresh (task 31 §6 records the 403s).
- That the cluster identity cannot reach cluster-admin through Kubernetes RBAC `bind`; see
  `cluster-kinds_test.sh` above.
- That ADO's branch control check behaves as described. Its settings are named from training
  knowledge, UNVERIFIED.
