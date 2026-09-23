### Task 31: The first-apply runbook and the smoke scripts — written now, run when the environment exists

**Owner-gated.** This task **writes** a runbook and scripts and lints them. It runs nothing against
Azure. Executing the runbook is the first time anything in this phase touches a subscription, and
it happens only after the owner hands one over (owner decisions `:52-56`) — at which point the
runbook is followed by a person, step by step, with its results recorded.

Why a runbook and not only a pipeline: the first apply has chicken-and-egg steps no pipeline can
do for itself — the state account must exist before any backend can be configured; the pool the
pipelines run on is created by the stack they plan; the four service connections and their
branch control, environments, branch policies and pipeline registrations are ADO settings; the
pipeline identities' roles and the database bootstrap are, by design, things no pipeline may do
for itself (review I8); the DNS delegation is a record in someone
else's zone.

**Files:**
- `docs/runbooks/phase-8-first-apply.md` (new)
- `docs/verification/phase-8-first-apply-results.md` — the results template, every row `NOT RUN` (new)
- `infra/smoke/{run.sh,lib.sh,00-cluster.sh,10-platform.sh,20-edge.sh,30-broker.sh,40-hub.sh,50-kata.sh}` (new)
- `infra/smoke/smoke_test.sh` — runs each script against canned command output (new)

#### The runbook's sections (each step: the command, the expected output, what to do if it differs)

| § | Step | Who | Needs |
|---|---|---|---|
| 0 | **Preconditions** — the owner-inputs table of the plan (`2026-09-23-phase-8-own-environment.md`, "Owner inputs"), each ticked with where the value was put | owner | all of them |
| 1 | **Subscription preparation** — register the resource providers the stacks use (`Microsoft.ContainerService`, `.Compute`, `.Network`, `.ContainerRegistry`, `.KeyVault`, `.DBforPostgreSQL`, `.OperationalInsights`, `.Insights`, `.DevCenter`, `.DevOpsInfrastructure`, `.Consumption`) and the `EncryptionAtHost` feature; check vCPU quota for the Kata VM family; `az aks get-versions -l <region>` and set `kubernetes_version` to a supported minor; list Azure Policy assignments that could deny Kata SKUs, public IPs or CNI Overlay | operator | subscription owner or contributor |
| 2 | **Bootstrap** — `stacks/bootstrap` with **local state**: `init -backend=false` is not enough here, so `terraform init` with no backend config, `plan -var-file=dev.tfvars`, review, `apply`; then configure the partial backend and `terraform init -migrate-state` into the container it just made; delete the local state file. The operator's own credentials make bootstrap's grants, which are the infra identity's state write and the plan identity's state read | operator | the four identities' object ids (the plan's "Deploy identities" table) |
| 2a | **Identities** — the owner creates the four pipeline identities per environment (plan, infra, cluster, push) and grants exactly the roles in the plan's "Deploy identities" table. That includes the infra identity's ABAC condition, which allows task 10's allowlist and excludes the infra identity itself as a principal. The owner also creates the two custom roles from `infra/rbac/*.json` (task 30) and checks that **no** pipeline identity is in the Postgres admin group or the cluster admin groups: `az ad group member check` per group per identity, and every answer must be `false` | owner | subscription owner / User Access Administrator; the Entra groups |
| 3 | **ADO** — create the **four** OIDC service connections per environment: plan, infra, cluster, push (task 30's service-connection table). On infra, cluster and push, set **branch control** (dev: `refs/heads/main`, `refs/heads/dev`; prod: `refs/heads/main`) with "verify branch protection" on and "allow unknown" off. Grant pipeline permission on each connection to the named pipeline(s) only; never "grant access to all pipelines". Create the ADO Environments `panacea-app-dev`/`-prod` with the owner as approver, and the variable group (names in `pipelines/`). Register the seven PR pipelines and `infra-plan`/`deploy`. Branch policies: every PR pipeline, `infra-plan` among them, as build validation on `main` and `dev`. **Check**: queue `deploy` from a feature branch, and it must fail at the connection's check before any step runs. Record the check's real setting names (task 30 names them from training knowledge) | owner / ADO admin | ADO project rights |
| 4 | **Foundation, first pass on a hosted agent** — the Managed DevOps Pool does not exist yet, so the first foundation plan runs on a hosted agent (dev only: the API server is public with an allowlist that must include the hosted run's egress, or the operator's workstation runs `tf-stack.sh` directly); review; apply. From then on `PANACEA_APP_CI_POOL` points at the pool | operator | step 3 |
| 5 | **DNS delegation** — hand `dns_zone_name_servers` to the parent zone's owner; wait for `dig NS <zone>` to answer with them | owner | a parent zone |
| 6 | **`data`** and **`workloads`** go through `infra-plan` → review → `deploy`. **`identity`** is owner-run only: the owner runs `tf-stack.sh`'s commands under their own credentials from a workstation, or enters the ids by hand (task 09's two paths). No pipeline identity holds Graph application rights. On the first dev plan, record every 403 the plan identity meets (task 30: a no-secret action may join `panacea-app-plan-reader`, a secret-returning one never does). Once, on purpose, apply a plan after the state has moved on, and record that Terraform refuses it as stale | operator; owner for identity | step 5 for cert-manager's zone grant |
| 7 | **Database bootstrap** — once per environment, **by a human** member of the Postgres admin group, with their own Entra token (`az account get-access-token --resource-type oss-rdbms`, from training knowledge, UNVERIFIED), from the pool's subnet or an allowlisted workstation: `psql` runs the Hub's `roles.sql`, then the `hub-principals.sql` rendered from the `workloads` outputs (task 29). Record the output. No pipeline identity is in that group (review I8), and `deploy` has no database stage | a human Postgres admin | step 6 (`workloads` outputs) |
| 7a | **Charts** — `deploy` with `deployCharts: true`, after step 7, so the Hub's first install finds its roles (review I9) | operator | image digests from a green `images` run on `main`/`dev` |
| 8 | **Smoke** — `infra/smoke/run.sh dev` from the pool or an allowlisted workstation | operator | all above |
| 9 | **Record** — fill `docs/verification/phase-8-first-apply-results.md`; every row is `PASS`, `FAIL` with the output, or `NOT RUN` with a reason | operator | — |

#### The smoke scripts (contract)

`run.sh <env>` runs every numbered script in order, prints one line per check —
`smoke <id>: PASS|FAIL <detail>` — and exits non-zero if any failed. They use `kubectl` through
`kubelogin` (never an admin credential), `curl`, and `openssl`; they create nothing except in
`50-kata.sh`, which uses a namespace `panacea-smoke` it creates and deletes.

| Id | Script | Check |
|---|---|---|
| S00.1 | `00-cluster.sh` | three node pools, each with its `panacea-app/role` label; the sandbox nodes carry the `panacea-app/sandbox=true:NoSchedule` taint |
| S00.2 | `00-cluster.sh` | `kubectl get runtimeclass` lists the configured `PANACEA_APP_RUNTIME_CLASS` — **P8-D15's first observation; the script prints every RuntimeClass name it saw** |
| S10.1 | `10-platform.sh` | the four namespaces enforce `restricted`; the four priority classes exist; every namespace has `default-deny` |
| S20.1 | `20-edge.sh` | `https://broker.<zone>/healthz` and `https://hub.<zone>/healthz` answer 200 with a certificate chaining to the configured issuer, from an allowlisted address |
| S20.2 | `20-edge.sh` | `http://…` (port 80) does not answer — there is no plaintext listener |
| S30.1 | `30-broker.sh` | `/readyz` 200; `/v1/whoami` with no token → 401 with `WWW-Authenticate: Bearer`; with `Origin:` and no token → 403 `forbidden_origin` |
| S40.1 | `40-hub.sh` | `/readyz` 200 (so Postgres, the schema version and the JWKS all answered, Hub contract §12.4) |
| S50.1 | `50-kata.sh` | a pod with the RuntimeClass, the sandbox toleration and node selector runs `uname -r`; the value **differs** from the node's kernel (a microVM, not a container) |
| S50.2 | `50-kata.sh` | from that pod: `169.254.169.254` does not answer; `kubernetes.default.svc` does not answer; `https://registry.npmjs.org` does |

**Authenticated end-to-end** (a real user's token through the broker, a session in a Kata pod) is
**not** in the scripts: it needs a signed-in user, and it is the owner's batched hands-on item
(OD-12, task 32).

#### Tests to write first — `smoke_test.sh`

Each script reads its commands through `lib.sh`'s `run_cmd`, which `smoke_test.sh` replaces with
canned outputs: `S00.2 passes when the configured class is listed and prints every class`,
`S00.2 fails naming the classes it saw when the configured one is absent` (kills a smoke that
passes on `kata-mshv-vm-isolation` when `kata-vm-isolation` was configured, or vice versa),
`S50.1 fails when uname matches the node` (kills a runc pod counted as Kata), `S20.2 fails if port
80 answers`, and one case per remaining check. `shellcheck` over `infra/smoke/*.sh` runs in task
28's `infra` pipeline.

#### Steps

- [ ] **Step 1:** `lib.sh`, `smoke_test.sh`, then the numbered scripts until the canned cases pass.
- [ ] **Step 2:** The runbook, section by section, citing the task that produced each artefact it
  uses; the results template with every row `NOT RUN`.
- [ ] **Step 3:** Commit.

#### Acceptance

```bash
bash infra/smoke/smoke_test.sh             # smoke_test: all cases passed
grep -c "NOT RUN" docs/verification/phase-8-first-apply-results.md   # one per check, none filled
```

#### What this task does not claim

- **That any step of the runbook works.** It has not been executed; every result row says so.
- That the first foundation apply can run from a hosted agent in every posture: a **private**
  (prod) cluster cannot be reached that way at all — prod's first apply runs after dev's pool
  exists, or from the owner's network. Recorded in §4.
