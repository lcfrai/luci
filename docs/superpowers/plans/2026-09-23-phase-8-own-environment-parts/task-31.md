### Task 31: The first-apply runbook and the smoke scripts — written now, run when the environment exists

**Owner-gated.** This task **writes** a runbook and scripts and lints them. It runs nothing against
Azure. Executing the runbook is the first time anything in this phase touches a subscription, and
it happens only after the owner hands one over (owner decisions `:52-56`) — at which point the
runbook is followed by a person, step by step, with its results recorded.

Why a runbook and not only a pipeline: the first apply has chicken-and-egg steps no pipeline can
do for itself — the state account must exist before any backend can be configured; the pool the
pipelines run on is created by the stack they plan; the four service connections and their
branch control, environments, branch policies and pipeline registrations are ADO settings; the
pipeline identities' roles, the database bootstrap and prod's database lock are, by design,
things no pipeline may do for itself (review I8, plan re-review I-2, I-4); the DNS delegation is a record in someone
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
| 1 | **Subscription preparation** — first, the **owner** tags the handed-over subscription `panacea-app-estate=<env>` (`az tag update --resource-id /subscriptions/<id> --operation Merge --tags panacea-app-estate=<env>`; the flags are training knowledge, UNVERIFIED, so record the command that worked), one tag per subscription and one subscription per environment (a subscription shared by dev and prod can never pass the prod guard, plan re-review b03 M-6), never on a Panacea subscription: it is the positive fingerprint the estate guard requires on every Terraform path and every workstation section (task 30, review I3; stages 4–5 and the image push rely on stage 1's read, task 30's stated residual). Then, and at the start of **every** section below run from a workstation (§1, §2, §2a, §4a, §6's `identity`, §6a), `export ARM_SUBSCRIPTION_ID=<id>`, `az account set --subscription <id>`, and `bash pipelines/scripts/estate-guard.sh <env> deploy`; a refusal stops the section (`tf-stack.sh` and `grant-pipeline-roles.sh` also run it themselves). Record the guard's `ok` line. Then register the resource providers the stacks use (`Microsoft.ContainerService`, `.Compute`, `.Network`, `.ContainerRegistry`, `.KeyVault`, `.DBforPostgreSQL`, `.OperationalInsights`, `.Insights`, `.DevCenter`, `.DevOpsInfrastructure`, `.Consumption`) and the `EncryptionAtHost` feature; check vCPU quota for the Kata VM family; `az aks get-versions -l <region>` and set `kubernetes_version` to a supported minor; list Azure Policy assignments that could deny Kata SKUs, public IPs or CNI Overlay | owner (the tag), then operator | subscription owner for the tag (`Microsoft.Resources/tags/write` on the subscription), then owner or contributor |
| 1a | **Identities, created** — the owner creates the four pipeline identities per environment (plan, infra, cluster, push) and records their object ids. **No role yet**: the scopes do not exist (plan re-review I-3). Each identity lives **outside** `rg-panacea-app-<env>` (an app registration, or a user-assigned identity in an owner-held group), so the infra identity's Contributor cannot write a federated credential on another pipeline identity and become it | owner | Entra, or an owner-held resource group |
| 2 | **Bootstrap** — first `bash pipelines/scripts/estate-guard.sh <env> deploy bootstrap` (the workstation path of §1; `tf-stack.sh` refuses `bootstrap`, so this call is the guard here, review I3). Then `stacks/bootstrap` with **local state**: `init -backend=false` is not enough here, so `terraform init` with no backend config, `plan -var-file=dev.tfvars`, review, `apply`; then configure the partial backend and `terraform init -migrate-state` into the container it just made; delete the local state file. The operator's own credentials make bootstrap's grants, which are the infra identity's state write and the plan identity's state read | operator | §1a's object ids |
| 2a | **Grants on what now exists** — the owner creates the three custom role definitions from `infra/rbac/*.json` (task 30: `panacea-app-cluster-deployer`, `panacea-app-sandboxes-deployer`, `panacea-app-plan-reader`). Then the grants whose scopes exist after §2, each exactly as the plan's "Deploy identities" table says: the plan identity's Reader on the subscription and `panacea-app-plan-reader` on `rg-panacea-app-<env>`; the infra identity's Contributor and Role Based Access Control Administrator on `rg-panacea-app-<env>`, with the ABAC condition that permits the **five** roles task 10 names for it and excludes the infra identity itself as a principal; its Cost Management Contributor on the subscription. The owner assigns the built-in Azure Policy that denies AKS clusters with local accounts enabled, effect **Deny**, on `rg-panacea-app-<env>` (the plan's residual 1; that the built-in policy offers Deny is **training knowledge, UNVERIFIED** — record its real name and effect). Then the checks: **no** pipeline identity is in the Postgres admin group or the cluster admin groups (`az ad group member check` per group per identity, every answer `false`), and `az role assignment list --assignee <id> --all` per identity matches its table row and nothing more | owner | subscription owner / User Access Administrator; the Entra groups |
| 3 | **ADO** — create the **four** OIDC service connections per environment: plan, infra, cluster, push (task 30's service-connection table). On infra, cluster and push, set **branch control** (dev: `refs/heads/main`, `refs/heads/dev`; prod: `refs/heads/main`) with "verify branch protection" on and "allow unknown" off. Grant pipeline permission on each connection to the named pipeline(s) only; never "grant access to all pipelines". Create the ADO Environments `panacea-app-dev`/`-prod` with the owner as approver, and the variable group (names in `pipelines/`). Register the seven PR pipelines and `infra-plan`/`deploy`. Branch policies: every PR pipeline, `infra-plan` among them, as build validation on `main` and `dev`. **Check**: queue `deploy` from a feature branch, and it must fail at the connection's check before any step runs. Record the check's real setting names (task 30 names them from training knowledge) | owner / ADO admin | ADO project rights |
| 4 | **Foundation, first pass on a hosted agent** — the Managed DevOps Pool does not exist yet, so the first foundation plan runs on a hosted agent (dev only: the API server is public with an allowlist that must include the hosted run's egress, or the operator's workstation runs `tf-stack.sh` directly, which runs the estate guard itself, review I3); review; apply. Record the guard's `ok` line from the plan run (plan connection) and from the apply (infra connection): each shows its identity could read the `panacea-app-estate` tag (task 30: that Reader and Cost Management Contributor include `Microsoft.Resources/subscriptions/read` is UNVERIFIED). **Each pipeline `ok` line must name a branch (`refs/heads/…`), never `workstation`**; a pipeline line naming `workstation` means the guard's pipeline test is wrong, so stop and record which of `TF_BUILD`, `BUILD_SOURCEBRANCH`, `SYSTEM_JOBID` and `AGENT_ID` the job had (task 30's `is_pipeline`, plan re-review b03 I-2). From then on `PANACEA_APP_CI_POOL` points at the pool | operator | step 3 |
| 4a | **Cluster and push grants** — now that `foundation` has made the cluster and the registry, the owner runs `infra/rbac/grant-pipeline-roles.sh <env> --dry-run`, reviews the eight lines, then runs it without `--dry-run` under their own credentials (task 30). It is the only place these grants are made; no stack makes them (plan re-review I-2). Record whether Azure accepted the four namespace scopes before `platform` created the namespaces. Check: `az role assignment list --assignee <cluster id> --all` shows the seven cluster rows and nothing more, and the push identity shows AcrPush on the registry only. **If the cluster is ever re-created, run the script again**: the old assignments point at the old cluster id. Then record two probes of the infra identity's residual 1 (the plan's "Deploy identities"): (a) the owner, under their own credentials, runs `az aks update --enable-local-accounts` on the dev cluster, and it must be refused by the §2a policy (a Deny policy binds every caller, the owner included); (b) a throwaway identity holding only Contributor on `rg-panacea-app-dev` runs `az aks command invoke -c "kubectl get secrets -n panacea-sandboxes"`, and it must fail; the owner deletes that identity afterwards. If either succeeds, stop and record it: that route is open, and closing it (for example a custom infra role without `runCommand/action`) is an owner decision | owner | step 4 |
| 5 | **DNS delegation** — hand `dns_zone_name_servers` to the parent zone's owner; wait for `dig NS <zone>` to answer with them | owner | a parent zone |
| 6 | **`data`** and **`workloads`** go through `infra-plan` → review → `deploy` with `applyStacks: true` and **`deployCharts: false`** (both default to `false`, task 30), so no chart is installed before §7 (review I9). **`identity`** is owner-run only: the owner runs `tf-stack.sh plan identity <env>`, reviews, then `tf-stack.sh apply identity <env>` (which applies the plan file just written) under their own credentials from a workstation, so the estate guard runs first (task 30, review I3); or enters the ids by hand (task 09's two paths). No pipeline identity holds Graph application rights. On the first dev plan, record every 403 the plan identity meets (task 30: a no-secret action may join `panacea-app-plan-reader`, a secret-returning one never does). Decode the AKS user credential the plan identity receives (`az aks get-credentials --file -` under the plan connection) and record that it holds no `client-key-data` and no `token`; if it holds either, the action leaves `plan-reader.json` (task 30, plan re-review M-2). Once, on purpose, apply a plan after the state has moved on, and record that Terraform refuses it as stale | operator; owner for identity | step 5 for cert-manager's zone grant |
| 6a | **Prod database lock** (prod only) — after the first prod `data` apply, the **owner** creates a `CanNotDelete` lock on the Postgres server under their own credentials (`az lock create --lock-type CanNotDelete --resource <server id>`). It is not in Terraform: no pipeline identity holds `Microsoft.Authorization/locks/write` (Contributor excludes it, training knowledge, UNVERIFIED), and one that did could also delete the lock (plan re-review I-4). Check: `az lock list --resource <server id>` shows it. From then on a `data` plan that would replace or destroy the prod server fails at apply until the owner lifts the lock, which is the point | owner | step 6 (`data` applied) |
| 7 | **Database bootstrap** — once per environment, **by a human** member of the Postgres admin group, with their own Entra token (`az account get-access-token --resource-type oss-rdbms`, from training knowledge, UNVERIFIED), from the pool's subnet or an allowlisted workstation, with `psql` and `python3` on its `PATH`. **Every file in this step runs as `psql -v ON_ERROR_STOP=1 --single-transaction -d <db> -f <file> > out.txt 2>&1; echo $? > rc.txt`, and the step reads `rc.txt`: anything but `0` stops the section** (plan re-review b03 I-1: without `ON_ERROR_STOP`, psql reports an error, runs the next statement and still exits 0; with it and `--single-transaction`, a failure rolls the whole file back. Exit code 3 for a script error is training knowledge, UNVERIFIED — record the real code from the first refusal). (a) In database `hub`, `SELECT count(*) FROM pg_roles WHERE rolname IN ('hub_owner','hub_app')`: **0** → run the Hub's `roles.sql` as above (one transaction, so a failed `GRANT CREATE` leaves 0 roles, never 2 without the grant); **2** → skip the `CREATE ROLE`s, but run `GRANT CREATE ON DATABASE hub TO hub_owner;` anyway, which is safe to repeat (plan re-review b03 M-4); **1** → stop and record it, a half-made bootstrap is repaired by hand, never by a re-run (review I9: the contract's `roles.sql` is a bare `CREATE ROLE` until ask HC-1 in task 33 is answered; once it is idempotent, this count becomes a check only). Then check `SELECT has_database_privilege('hub_owner','hub','CREATE')` returns `t`; `f` stops the section. (b) `SELECT * FROM pgaadauth_list_principals(false)` and record its real columns (task 29 names `rolname` and `objectid` from training knowledge). (c) Download the `stack-outputs` artefact of the §6 `deploy` run; `bash pipelines/scripts/render-sql.sh deploy/sql/hub-principals.sql.tpl <artefact>/workloads.json > hub-principals.sql` (task 29's mapping table reads the names **and** `hub_principal_id`/`hub_migrate_principal_id` by output name; a non-zero exit stops here), then run it as above. A re-run is safe; a role bound to another object id raises, and because both grants sit inside the checked block and the file is one transaction, nothing is granted and `rc.txt` is non-zero. Record `out.txt`. No pipeline identity is in that group (review I8), and `deploy` has no database stage | a human Postgres admin | step 6 (`workloads` outputs, the `stack-outputs` artefact) |
| 7a | **Charts** — `deploy` with `deployCharts: true`, after step 7, so the Hub's first install finds its roles (review I9). If §7 was skipped, the `hub` release is the one that fails, last in stage 4, after the other four have installed: its `hub-migrate` hook cannot log in or cannot `SET ROLE hub_owner`, `--atomic` removes the release, and the stage prints the Job's log (task 30). Record that log's first line; with ask HC-2 (task 33) answered it names the missing bootstrap. Record whether the `broker` release's `Role` in `panacea-sandboxes` installs with `sandboxes-deployer.json`'s `escalate`/`bind` verbs, and the exact error if it fails (task 30, plan re-review I-1) | operator | image digests from a green `images` run on `main`/`dev` |
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
