### Task 31: The first-apply runbook and the smoke scripts — written now, run when the environment exists

**Owner-gated.** This task **writes** a runbook and scripts and lints them. It runs nothing against
Azure. Executing the runbook is the first time anything in this phase touches a subscription, and
it happens only after the owner hands one over (owner decisions `:52-56`) — at which point the
runbook is followed by a person, step by step, with its results recorded.

Why a runbook and not only a pipeline: the first apply has chicken-and-egg steps no pipeline can
do for itself — the state account must exist before any backend can be configured; the pool the
pipelines run on is created by the stack they plan; the service connection, environments, branch
policies and pipeline registrations are ADO settings; the DNS delegation is a record in someone
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
| 2 | **Bootstrap** — `stacks/bootstrap` with **local state**: `init -backend=false` is not enough here, so `terraform init` with no backend config, `plan -var-file=dev.tfvars`, review, `apply`; then configure the partial backend and `terraform init -migrate-state` into the container it just made; delete the local state file | operator | the deploy identity's object id |
| 3 | **ADO** — create the OIDC service connection per environment; the ADO Environments `panacea-app-dev`/`-prod` with the owner as approver; the variable group (names in `pipelines/`); register the seven PR pipelines and `infra-plan`/`deploy`; branch policies: every PR pipeline as build validation on `main` and `dev` | owner / ADO admin | ADO project rights |
| 4 | **Foundation, first pass on a hosted agent** — the Managed DevOps Pool does not exist yet, so the first foundation plan runs on a hosted agent (dev only: the API server is public with an allowlist that must include the hosted run's egress, or the operator's workstation runs `tf-stack.sh` directly); review; apply. From then on `PANACEA_APP_CI_POOL` points at the pool | operator | step 3 |
| 5 | **DNS delegation** — hand `dns_zone_name_servers` to the parent zone's owner; wait for `dig NS <zone>` to answer with them | owner | a parent zone |
| 6 | **`data`**, **`identity`** (the pipeline, or the owner under their own credentials, or the ids entered by hand — task 09's three paths), **`workloads`** — each through `infra-plan` → review → `deploy` | operator, owner for identity | step 5 for cert-manager's zone grant |
| 7 | **Charts and database** — `deploy` with `deployCharts: true`; its database stage runs `roles.sql` and `hub-principals.sql` before the Hub's first install | operator | image digests from a green `images` run |
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
