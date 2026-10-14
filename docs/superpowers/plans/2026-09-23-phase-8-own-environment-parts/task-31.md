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

**The engine is OpenTofu (`tofu`), never `terraform` (OD-32).** Every command below that drives
the engine says `tofu`. Three kinds of "terraform" in this table are deliberately left alone:
the directory `infra/terraform/…`, which is a path and not a licence; the `terraform { … }`
block and the `terraform.tfstate` / `terraform.tfstate.backup` file names, which OpenTofu keeps
unchanged; and **the two probe records**, which say "a Terraform 1.15.8 probe" because that is
the binary those readings were actually taken with, before OD-32. Do not rewrite a probe record
to say `tofu` — that would manufacture evidence. **UNVERIFIED and to be re-confirmed on the
first run:** that OpenTofu 1.12.6 reproduces both probes, including the exact error text
`Can't ask approval for state migration when interactive input is disabled`. Record what it
actually prints.

#### The runbook's sections (each step: the command, the expected output, what to do if it differs)

| § | Step | Who | Needs |
|---|---|---|---|
| 0 | **Preconditions** — the owner-inputs table of the plan (`2026-09-23-phase-8-own-environment.md`, "Owner inputs"), each ticked with where the value was put | owner | all of them |
| 1 | **Subscription preparation** — first, the **owner** tags the handed-over subscription `panacea-app-estate=<env>` (`az tag update --resource-id /subscriptions/<id> --operation Merge --tags panacea-app-estate=<env>`; the flags are training knowledge, UNVERIFIED, so record the command that worked), one tag per subscription and one subscription per environment (a subscription shared by dev and prod can never pass the prod guard, plan re-review b03 M-6), never on a Panacea subscription: it is the positive fingerprint the estate guard requires on every OpenTofu path and every workstation section (task 30, review I3; stages 4–5 and the image push rely on stage 1's read, task 30's stated residual). Then, and at the start of **every** section below run from a workstation (§1, §2, §2a, §4a, §6's `identity`, §6a), `export ARM_SUBSCRIPTION_ID=<id>`, `az account set --subscription <id>`, and `bash pipelines/scripts/estate-guard.sh <env> deploy`; a refusal stops the section (`tf-stack.sh` and `grant-pipeline-roles.sh` also run it themselves). Record the guard's `ok` line. Then register the resource providers the stacks use (`Microsoft.ContainerService`, `.Compute`, `.Network`, `.ContainerRegistry`, `.KeyVault`, `.DBforPostgreSQL`, `.OperationalInsights`, `.Insights`, `.DevCenter`, `.DevOpsInfrastructure`, `.Consumption`) and the `EncryptionAtHost` feature; check vCPU quota for the Kata VM family; `az aks get-versions -l <region>` and set `kubernetes_version` to a supported minor; list Azure Policy assignments that could deny Kata SKUs, public IPs or CNI Overlay | owner (the tag), then operator | subscription owner for the tag (`Microsoft.Resources/tags/write` on the subscription), then owner or contributor |
| 1a | **Identities, created** — the owner creates the four pipeline identities per environment (plan, infra, cluster, push) and records their object ids. **No role yet**: the scopes do not exist (plan re-review I-3). Each identity lives **outside** `rg-panacea-app-<env>` (an app registration, or a user-assigned identity in an owner-held group), so the infra identity's Contributor cannot write a federated credential on another pipeline identity and become it | owner | Entra, or an owner-held resource group |
| 2 | **Bootstrap** — first `bash pipelines/scripts/estate-guard.sh <env> deploy bootstrap` (the workstation path of §1; `tf-stack.sh` refuses `bootstrap`, so this call is the guard here, review I3). Then `stacks/bootstrap` with **local state** (review I2). The committed `backend "azurerm" {}` cannot init without the account it creates (a Terraform 1.15.8 probe: `terraform init -input=false` on it errors `One of access_key, sas_token, use_azuread_auth and resource_group_name must be specified`, and `plan` after `init -backend=false` refuses), so: (a) write an **uncommitted** `infra/terraform/stacks/bootstrap/backend_override.tf` holding exactly `terraform { backend "local" {} }` (an override file replaces the backend; the probe confirmed it), and confirm `git check-ignore -v` names the `.gitignore` rule `*_override.tf`; (b) `tofu init -input=false`, `plan -var-file=<env>.tfvars -out=bootstrap.tfplan`, review, `apply bootstrap.tfplan`; (c) delete `backend_override.tf`; (d) `tofu init -migrate-state -force-copy -input=false -backend-config=../../backends/<env>.hcl -backend-config=resource_group_name=<output state_resource_group_name> -backend-config=storage_account_name=<output state_storage_account_name> -backend-config=key=bootstrap.<env>.tfstate`. `-force-copy` is the answer to the copy prompt: without it, `-input=false` leaves the engine nothing to ask and it refuses with `Can't ask approval for state migration when interactive input is disabled` (exit 1). A Terraform 1.15.8 probe, local→local, showed `-force-copy` copies the state, leaves the local `terraform.tfstate` behind for (e) to delete, and gives a no-change plan (plan re-review b04 round 1, finding 1); local→azurerm with `use_azuread_auth` is **UNVERIFIED**, so record the output; (e) `plan -var-file=<env>.tfvars` against the migrated state shows **no changes**, then delete the local `terraform.tfstate`, `terraform.tfstate.backup` and `bootstrap.tfplan`; (f) `bash infra/scripts/guards.sh` passes (G1 refuses a tracked override). Step (d) needs the operator to hold Storage Blob Data Contributor on the container, so the operator is in the break-glass group that `state_writer_object_ids` names; a 403 straight after (b) is role-assignment propagation, so wait and retry (training knowledge, UNVERIFIED). The operator's own credentials make bootstrap's grants, which are the infra identity's state write and the plan identity's state read | operator | §1a's object ids |
| 2a | **Grants on what now exists** — the owner creates the three custom role definitions from `infra/rbac/*.json` (task 30: `panacea-app-cluster-deployer`, `panacea-app-sandboxes-deployer`, `panacea-app-plan-reader`). Then the grants whose scopes exist after §2, each exactly as the plan's "Deploy identities" table says: the plan identity's Reader on the subscription and `panacea-app-plan-reader` on `rg-panacea-app-<env>`; the infra identity's Contributor and Role Based Access Control Administrator on `rg-panacea-app-<env>`, with the ABAC condition that permits the **five** roles task 10 names for it and excludes the infra identity itself as a principal; its Cost Management Contributor on the subscription. The owner assigns the built-in Azure Policy that denies AKS clusters with local accounts enabled, effect **Deny**, on `rg-panacea-app-<env>` (the plan's residual 1; that the built-in policy offers Deny is **training knowledge, UNVERIFIED** — record its real name and effect). Then the checks: **no** pipeline identity is in the Postgres admin group or the cluster admin groups (`az ad group member check` per group per identity, every answer `false`), and `az role assignment list --assignee <id> --all` per identity matches its table row and nothing more | owner | subscription owner / User Access Administrator; the Entra groups |
| 3 | **ADO** — create the **four** OIDC service connections per environment: plan, infra, cluster, push (task 30's service-connection table). On infra, cluster and push, set **branch control** (dev: `refs/heads/main`, `refs/heads/dev`; prod: `refs/heads/main`) with "verify branch protection" on and "allow unknown" off. Grant pipeline permission on each connection to the named pipeline(s) only; never "grant access to all pipelines". Create the ADO Environments `panacea-app-dev`/`-prod` with the owner as approver, and the variable group (names in `pipelines/`). Register the seven PR pipelines and `infra-plan`/`deploy`. Branch policies: every PR pipeline, `infra-plan` among them, as build validation on `main` and `dev`. **Check**: queue `deploy` from a feature branch, and it must fail at the connection's check before any step runs. Record the check's real setting names (task 30 names them from training knowledge). **Check** (task 28 fix round 1, review I3): the first `secrets` PR run's scan step prints `secrets-pr: scanning <sha>..HEAD`, which shows the full-depth checkout fetched `refs/remotes/origin/<target>` with the job's token (training knowledge, UNVERIFIED). If it prints `… is not in the checkout`, record the checkout's fetch line from the log; the fallback is `persistCredentials: true` on that one checkout and a fetch of the target in `secrets-pr.sh`, as a reviewed change | owner / ADO admin | ADO project rights |
| 4 | **Foundation, first pass on a hosted agent** — the Managed DevOps Pool does not exist yet, so the first foundation plan runs on a hosted agent (dev only: the API server is public with an allowlist that must include the hosted run's egress, or the operator's workstation runs `tf-stack.sh` directly, which runs the estate guard itself, review I3); review; apply. Record the guard's `ok` line from the plan run (plan connection) and from the apply (infra connection): each shows its identity could read the `panacea-app-estate` tag (task 30: that Reader and Cost Management Contributor include `Microsoft.Resources/subscriptions/read` is UNVERIFIED). **Each pipeline `ok` line must name a branch (`refs/heads/…`), never `workstation`**; a pipeline line naming `workstation` means the guard's pipeline test is wrong, so stop and record which of `TF_BUILD`, `BUILD_SOURCEBRANCH`, `SYSTEM_JOBID` and `AGENT_ID` the job had (task 30's `is_pipeline`, plan re-review b03 I-2). From then on `PANACEA_APP_CI_POOL` points at the pool | operator | step 3 |
| 4a | **Cluster and push grants** — now that `foundation` has made the cluster and the registry, the owner runs `infra/rbac/grant-pipeline-roles.sh <env> --dry-run`, reviews the eight lines, then runs it without `--dry-run` under their own credentials (task 30). It is the only place these grants are made; no stack makes them (plan re-review I-2). Record whether Azure accepted the four namespace scopes before `platform` created the namespaces. Check: `az role assignment list --assignee <cluster id> --all` shows the seven cluster rows and nothing more, and the push identity shows AcrPush on the registry only. **If the cluster is ever re-created, run the script again**: the old assignments point at the old cluster id. Then record two probes of the infra identity's residual 1 (the plan's "Deploy identities"): (a) the owner, under their own credentials, runs `az aks update --enable-local-accounts` on the dev cluster, and it must be refused by the §2a policy (a Deny policy binds every caller, the owner included); (b) a throwaway identity holding only Contributor on `rg-panacea-app-dev` runs `az aks command invoke -c "kubectl get secrets -n panacea-sandboxes"`, and it must fail; the owner deletes that identity afterwards. If either succeeds, stop and record it: that route is open, and closing it (for example a custom infra role without `runCommand/action`) is an owner decision | owner | step 4 |
| 5 | **DNS delegation** — hand `dns_zone_name_servers` to the parent zone's owner; wait for `dig NS <zone>` to answer with them | owner | a parent zone |
| 6 | **`data`** and **`workloads`** go through `infra-plan` → review → `deploy` with `applyStacks: true` and **`deployCharts: false`** (both default to `false`, task 30), so no chart is installed before §7 (review I9). **`identity`** is owner-run only: the owner runs `tf-stack.sh plan identity <env>`, reviews, then `tf-stack.sh apply identity <env>` (which applies the plan file just written) under their own credentials from a workstation, so the estate guard runs first (task 30, review I3); or enters the ids by hand (task 09's two paths). No pipeline identity holds Graph application rights. On the first dev plan, record every 403 the plan identity meets (task 30: a no-secret action may join `panacea-app-plan-reader`, a secret-returning one never does). Decode the AKS user credential the plan identity receives (`az aks get-credentials --file -` under the plan connection) and record that it holds no `client-key-data` and no `token`; if it holds either, the action leaves `plan-reader.json` (task 30, plan re-review M-2). Once, on purpose, apply a plan after the state has moved on, and record that OpenTofu refuses it as stale | operator; owner for identity | step 5 for cert-manager's zone grant |
| 6a | **Prod database lock** (prod only) — after the first prod `data` apply, the **owner** creates a `CanNotDelete` lock on the Postgres server under their own credentials (`az lock create --lock-type CanNotDelete --resource <server id>`). It is not in OpenTofu: no pipeline identity holds `Microsoft.Authorization/locks/write` (Contributor excludes it, training knowledge, UNVERIFIED), and one that did could also delete the lock (plan re-review I-4). Check: `az lock list --resource <server id>` shows it. From then on a `data` plan that would replace or destroy the prod server fails at apply until the owner lifts the lock, which is the point | owner | step 6 (`data` applied) |
| 7 | **Database bootstrap** — once per environment, **by a human** member of the Postgres admin group, with their own Entra token (`az account get-access-token --resource-type oss-rdbms`, from training knowledge, UNVERIFIED), from the pool's subnet or an allowlisted workstation, with `psql` and `python3` on its `PATH`. **Every file in this step runs as `psql -v ON_ERROR_STOP=1 --single-transaction -d <db> -f <file> > out.txt 2>&1; echo $? > rc.txt`, and the step reads `rc.txt`: anything but `0` stops the section** (plan re-review b03 I-1: without `ON_ERROR_STOP`, psql reports an error, runs the next statement and still exits 0; with it and `--single-transaction`, a failure rolls the whole file back. Exit code 3 for a script error is training knowledge, UNVERIFIED — record the real code from the first refusal). (a) In database `hub`, `SELECT count(*) FROM pg_roles WHERE rolname IN ('hub_owner','hub_app')`: **0** → run the Hub's `roles.sql` as above (one transaction, so a failed `GRANT CREATE` leaves 0 roles, never 2 without the grant); **2** → skip the `CREATE ROLE`s, but run `GRANT CREATE ON DATABASE hub TO hub_owner;` anyway, which is safe to repeat (plan re-review b03 M-4); **1** → stop and record it, a half-made bootstrap is repaired by hand, never by a re-run (review I9: the contract's `roles.sql` is a bare `CREATE ROLE` until ask HC-1 in task 33 is answered; once it is idempotent, this count becomes a check only). Then check `SELECT has_database_privilege('hub_owner','hub','CREATE')` returns `t`; `f` stops the section. (b) `SELECT * FROM pgaadauth_list_principals(false)` and record its real columns (task 29 names `rolname` and `objectid` from training knowledge). (c) Download the `stack-outputs` artefact of the §6 `deploy` run; `bash pipelines/scripts/render-sql.sh deploy/sql/hub-principals.sql.tpl <artefact>/workloads.json > hub-principals.sql` (task 29's mapping table reads the names **and** `hub_principal_id`/`hub_migrate_principal_id` by output name; a non-zero exit stops here), then run it as above. A re-run is safe; a role bound to another object id raises, and because both grants sit inside the checked block and the file is one transaction, nothing is granted and `rc.txt` is non-zero. Record `out.txt`. No pipeline identity is in that group (review I8), and `deploy` has no database stage | a human Postgres admin | step 6 (`workloads` outputs, the `stack-outputs` artefact) |
| 7a | **Charts** — `deploy` with `deployCharts: true`, after step 7, so the Hub's first install finds its roles (review I9). If §7 was skipped, the `hub` release is the one that fails, last in stage 4, after the other four have installed: its `hub-migrate` hook cannot log in or cannot `SET ROLE hub_owner`, `--atomic` removes the release, and the stage prints the Job's log (task 30). Record that log's first line; with ask HC-2 (task 33) answered it names the missing bootstrap. Record whether the `broker` release's `Role` in `panacea-sandboxes` installs with `sandboxes-deployer.json`'s `escalate`/`bind` verbs, and the exact error if it fails (task 30, plan re-review I-1). Record the same for its `Role/broker-lease` in `panacea-broker`, and that exactly one broker replica's log says it holds the `broker-sleeper` Lease with no `forbidden` on `leases` (task 29, review m11) | operator | image digests from a green `images` run on `main`/`dev` |
| 8 | **Smoke** — `infra/smoke/run.sh dev --require-kata` from the pool or an allowlisted workstation, run by a member of the cluster admin group signed in through `kubelogin` with their own Entra account (never `--admin`): S50 needs `pods` create, get and delete and `pods/log` get in `panacea-sandboxes`, which no pipeline identity holds (the smoke contract below, plan re-review b08-1 issue 1). Record `kubectl auth can-i create pods -n panacea-sandboxes`'s answer. S50.1 and S50.2 must be `PASS` here; the pipeline's own smoke stage prints them `NOT RUN` | operator in the cluster admin group | all above |
| 9 | **Record** — fill `docs/verification/phase-8-first-apply-results.md`; every row is `PASS`, `FAIL` with the output, or `NOT RUN` with a reason | operator | — |

#### The smoke scripts (contract)

`run.sh <env>` runs every numbered script in order, prints one line per check —
`smoke <id>: PASS|FAIL <detail>` — and exits non-zero if any failed. They use `kubectl` through
`kubelogin` (never an admin credential), `curl`, and `openssl`; they create nothing except
`50-kata.sh`'s one pod, `smoke-kata-<random>` in **`panacea-sandboxes`**, which an `EXIT` trap
deletes. It runs there, not in a namespace of its own, so that task 11's P8-D12 policies
(`default-deny`, `allow-dns`, `egress-internet`) select it exactly as they select a session
(review m9: a `panacea-smoke` namespace has no default-deny, so IMDS would answer and S50.2 would
fail for the wrong reason, or pass for one). The pod carries `panacea-app/component: sandbox` and
`panacea-app/smoke: "true"`, and **no** `panacea-app/owner`, so no owner view lists it (task 14).
It uses the sandbox image by the digest of the `images` run that §7a deployed, whose `curl` does
the probes, with its entrypoint replaced by the probe command. It is restricted-compliant: UID
1001, `runAsNonRoot`, no privilege escalation, `drop: [ALL]`, `seccompProfile: RuntimeDefault`.
It is sized (explicit CPU and memory requests and limits, which Kata needs to size the microVM and
which task 13's `every_sandbox_container_is_sized` requires in this namespace). The script reads
the results with `kubectl logs`, so it needs `pods` create, get and delete and `pods/log` get in
`panacea-sandboxes`, and no `exec`.

**Who can run S50 (plan re-review b08-1 issue 1).** The pipeline's cluster identity cannot, and
must not: in `panacea-sandboxes` it holds only `panacea-app-sandboxes-deployer`, which
`cluster-kinds_test.sh` forbids ever to name `pods` (task 30), because `pods` create in the
namespace that holds users' tokens can mount any of those Secrets. So S50 runs on the **human
path** only: task 31 §8 (whose S50.1 result task 32's P02 reads), by a member of the cluster admin group
(`var.cluster_admin_group_object_ids`) signed in through `kubelogin` with their own Entra account,
never `--admin`. Before creating anything, `50-kata.sh` runs
`kubectl auth can-i create pods -n panacea-sandboxes` (that `can-i` answers through the AKS Azure
RBAC webhook is **training knowledge, UNVERIFIED**; §8 records its answer). On `no`:
- by default (the pipeline's `deploy` stage 5, task 30), it prints
  `smoke S50.1: NOT RUN (the cluster identity holds no pods in panacea-sandboxes, task 30)` and
  the same line for S50.2, and those rows do not fail `run.sh`;
- with `run.sh <env> --require-kata` (§8 passes it), the same answer prints
  `smoke S50.1: FAIL (no pods create in panacea-sandboxes; run as a cluster admin group member)`
  and the same for S50.2, and `run.sh` exits non-zero. So the one run that must prove P8-D12
  cannot skip it quietly.

No grant is added for the pipeline. Granting the cluster identity `pods` there would break task
30's `cluster-kinds` rule and test, the `grant-pipeline-roles` test, and the reasoning behind
them, and would be an owner question like Q4; this plan does not take it.

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
| S50.2 | `50-kata.sh` | from that pod in `panacea-sandboxes`, each probe `curl --max-time 3`, **without** `-f`. "Answers" means any HTTP response, whatever its status: curl exits 0. "Does not answer" means curl exits non-zero (a timeout or a refused connection). IMDS answers **400** to a request without the `Metadata: true` header (training knowledge, UNVERIFIED), so a probe that used `-f` or tested for 200 would read a reachable IMDS as blocked. `http://169.254.169.254/metadata/instance` does not answer (`egress-internet`'s `except`); `https://kubernetes.default.svc` does not answer (no allow to the API server); `https://registry.npmjs.org` does (`allow-dns` plus `egress-internet`). This is P8-D12's smoke only. Task 32's P08 is the full probe set (another session's 8080, a machine's 8081, the VNet, the Azure wire server) |

**Authenticated end-to-end** (a real user's token through the broker, a session in a Kata pod) is
**not** in the scripts: it needs a signed-in user, and it is the owner's batched hands-on item
(OD-12, task 32).

#### Tests to write first — `smoke_test.sh`

Each script reads its commands through `lib.sh`'s `run_cmd`, which `smoke_test.sh` replaces with
canned outputs: `S00.2 passes when the configured class is listed and prints every class`,
`S00.2 fails naming the classes it saw when the configured one is absent` (kills a smoke that
passes on `kata-mshv-vm-isolation` when `kata-vm-isolation` was configured, or vice versa),
`S50.1 fails when uname matches the node` (kills a runc pod counted as Kata), `S20.2 fails if port
80 answers`, `S50 creates its pod in panacea-sandboxes with the sandbox labels and no owner label`
(reads the manifest `run_cmd` was handed; kills a probe moved back to a namespace with no
default-deny, review m9), `S50.2 fails naming IMDS when IMDS answers 400` (the canned curl exits 0
with status 400; kills a probe using `curl -f` or testing for 200, which reads a reachable IMDS as
blocked, plan re-review b08-1 issue 1), `S50 prints NOT RUN for S50.1 and S50.2 and run.sh exits 0
when can-i answers no` (kills a pipeline smoke that 403s at pod create and fails every `deploy`),
`S50 fails both rows and run.sh exits non-zero when can-i answers no under --require-kata` (kills a
human-path run that skips P8-D12 quietly), `S50 creates no pod when can-i answers no` (reads the
calls `run_cmd` recorded), and one case per remaining check. `shellcheck` over `infra/smoke/*.sh` runs in task
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
