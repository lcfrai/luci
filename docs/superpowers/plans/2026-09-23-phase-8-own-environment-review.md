# Review: Phase 8 — Our own environment

**Plan under review:** `docs/superpowers/plans/2026-09-23-phase-8-own-environment.md` and its 33 parts
in `2026-09-23-phase-8-own-environment-parts/`, on `feat/env` at `b17f43a`.
**Reviewer:** adversarial plan reviewer, 2026-09-23. Read-only except this file.

**Verdict in one line:** a careful, well-cited plan with three defects that would each stop the
first real deploy (the edge namespace is sealed by its own default-deny, the CI agents' subnet has
no way out, and the plan-on-PR pipeline carries the apply identity), plus ten Important issues.
**Plan quality: Needs fixes.**

Part paths below are relative to `docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/`
unless they start with a repo directory. "Plan" means the top-level file.

## How this review checked things

- **Every Go, TypeScript and Terraform citation the plan makes into this repo was opened** for
  tasks 01, 06, 14, 15, 17, 18, 22, 23, 24 and 26: `manager.go:20`, `:139-210`, `:240-247`;
  `kata.go:1-4`, `:84-89`, `:229-231`; `kata_test.go:207`, `:228`, `:355`; `manager_test.go:34-36`;
  `manager_growth_test.go:61-70`; `cmd/api/main.go:231`, `:241-271`; `config.go:65-81`;
  `backend.go:29-37`, `:124-130`; `sessions.go:33-51`, `:94-101`, `:398`, `:422`, `:569-610`, `:616`,
  `:945`, `:1010`, `:1080`; `attach.go:148`, `:182`, `:199`, `:216`, `:330-343`; `api.go:43-45`,
  `:72-74`; `registry/session.go:45`; `catalog.go:29`, `:174`, `:203`; `MachineChip.tsx:45-47`;
  `StatusDot.tsx:16-27`; `Rail.tsx:16`; `selectors.ts:32-33`; `store.ts:17`; `heartbeat.ts:23`,
  `:64`; `useKeyboardMap.ts` (⌘⇧M exists); `infra/terraform/kata.tf`, `variables.tf:17-21`,
  `dev.tfvars:4`, `.terraform.lock.hcl:4-6`; `images/sandbox/Dockerfile:1-34`. **All were right.**
  The writer's in-repo citation discipline is the best this program has produced.
- **Sibling citations opened:** Hub contract §6.3 (`:417-494`, the interface task 16 adopts —
  matches verbatim), §6.5 (`:528-551`), §9.1 (`:1165`), §10.1 (`:1414-1450`), §12.2 (`:2140`),
  §12.3 (`:2193`), §14 (`:2257`); phase 3 `task-02.md:144-160` (`gitx.Machine`, four methods);
  phase 2 `task-26.md:936`, `:984-994` (S8.3); survey `:46-51`, `:419-427`, `:561`, `:582`. All
  right. **ADR-H1 line citations have drifted** (Minor m1).
- **Two facts were settled by running Terraform 1.15.8 in the scratchpad (provider-free probes, no
  Azure):**
  1. A `.tftest.hcl` file-level `variables` block **overrides** `terraform test -var-file=…`
     (probe: file sets `environment = "dev"`, `-var-file` sets `"prod"`; the run saw `"dev"`,
     `Failure! 0 passed, 1 failed`). Issue I1.
  2. With `backend "azurerm" {}`, `terraform plan` after `init -backend=false` refuses ("run
     terraform init with either the -reconfigure or -migrate-state flags"), and `terraform init`
     with no backend config errors (`One of access_key, sas_token, use_azuread_auth and
     resource_group_name must be specified`). But `validate` and `terraform test` **do** work after
     `init -backend=false` (`Success! 2 passed`), so the plan's validation strategy holds. A
     `backend_override.tf` holding `backend "local" {}` makes a local plan work. Issue I2.
- **Not re-verified by this review:** the writer's claim that every azurerm 4.81.0 / azuread 3.9.0
  argument name was checked against `terraform providers schema -json`. The provider download was
  not repeated. Facts from training knowledge are marked as such below.

---

## Coverage

**Owner decisions.** Every decision that binds phase 8 is carried, with a named task:

| Decision | Where the plan meets it | Verdict |
|---|---|---|
| OD-5 own environment, from scratch, in this repo | five stacks, four charts, pipelines; the phase 1 root that targets **Panacea's** cluster is deleted first (task 01) | met |
| OD-13 no Panacea source | `P:` citations are patterns only; nothing transcribed | met |
| OD-3 Hub hosted server-side | task 08 (Postgres), 10 (identities), 29 (chart) | met |
| OD-6 user's own token | broker validates the user's Entra token (16, 17); remotebe asks the Hub's TokenSource (22) | met |
| OD-2 local clones | one machine disk holds `projects/` and `lanes/`, so hard links work (P8-D6) | met, pending P03 |
| OD-4 ADO and GitHub | egress allowlist, `machined` clone path (21) | met |
| OD-8 transcript adapter pin | images pipeline runs the canary on the image it pushes (27) | met |
| OD-10 Forget deletes | `machined` deletes by minted id, never opens the file (20) | met |
| OD-11 rule 3 licence gate | task 28, repo-wide | met for Go/npm/Cargo; the plan's own claim that providers and Helm deps are gated is not (m6) |
| OD-12 owner-observed items unverified | tasks 26, 31, 32 say so | met |
| OD-1 UI branch unmerged | task 26 step 5 | met |
| "written and **planned**, never applied" (owner decisions `:56`) | `terraform test` plans every stack against mocks | met in spirit; a real `plan` needs a subscription |

**Phase brief (`PROGRAM.md:119-133`).** Kata backend, Terraform, warm pools, the Hub's hosting,
identity, networking: all present. The cross-cutting Kata constraints at `PROGRAM.md:133`:

- **Explicit CPU/memory requests**: sessions (`manager.go:188-193`), balloons (task 12), machine
  pods (task 19) — yes. **The pre-pull DaemonSet's init containers have none**, and the namespace
  LimitRange turns that into 8 CPU / 32 GiB each (I4).
- **No host networking**: kept (`manager.go:161-162`); nothing adds it.
- **Defender gap**: recorded as seam S8-P9.2 and a named `PROGRAM.md` gap with compensating
  controls (task 33). Honest.

**Functional spec "Machines" (`FUNCTIONAL-SPEC.md:42-44`).** One cloud machine at first sign-in
with no form (task 19 ensure-default, task 26 once-per-run call); the six states (five emitted,
`stopped` reserved and tested as never emitted); implicit wake with "waking… Ns" (tasks 24, 26);
the local machine badged `unsandboxed` (unchanged). "Claimed from a warm pool" is honestly
narrowed to the pod, not the disk (task 19, S8-D.1).

**Prototypes.** None of the four prototypes (`phase-3/4/5/7`) draws phase 8 UI; task 26 builds on
phase 2's rail and the functional spec. Nothing to compare.

**Seams.** S8.1–S8.7 and SX.10 are each consumed by a named task; S8.2's kubeconfig refusal is
kept exactly; S8.3 is closed the right way round (the refusal and its test stay).

## Strengths

1. **The Panacea blast radius is taken seriously from task 01.** The existing root
   (`infra/terraform/kata.tf:4-6`, `variables.tf:1-4`, `dev.tfvars:4`, whose comment says
   `az aks show -g rg-aks-panacea-dev -n aks-panacea-dev`) points at Panacea's cluster. Task 01
   deletes it before anything else, and step 1 stops if a `*.tfstate` exists. Guards G1 (no
   `terraform.tfvars`), G2 (no GUID in tfvars), G5 (empty backend) and the `-lockfile=readonly`
   init make the Panacea traps mechanical. Names (`aks-panacea-app-dev`, `rg-panacea-app-dev`) do
   not collide with Panacea's (`aks-panacea-dev`, `rg-aks-panacea-dev`).
2. **Secure at create time.** Overlay + Cilium, local accounts off, Entra RBAC, workload identity,
   host encryption, run-command off, Entra-only Postgres and state storage, ACR admin off, Key
   Vault RBAC-only with purge protection. Each is a test with a named mutant.
3. **The broker is least-privilege where it counts.** A namespaced `Role` only, no `pods/exec`,
   `attach` or `portforward`, secrets by name only (tested as absent); no Azure identity; sandboxes
   get no service-account token and `restricted` Pod Security; IMDS and wire-server blocked.
4. **Token handling is right.** Task 16 adopts the Hub contract's interface verbatim (checked),
   refuses `none`/`HS*` before claims are read, keys identity on `tid/oid` never `sub`; the
   middleware strips `Authorization` so no handler can forward it; the broker refuses any request
   with an `Origin`; the attach token moves out of the pod spec into a Secret owned by the pod.
5. **Test discipline.** Every task writes tests first, every test names the mutant it kills, and
   negative cases (`expect_failures`, 404-for-another-owner, escaping paths) are present
   throughout. Existing assertions that must change are named with their lines.
6. **Honesty about the unknown.** Every Azure fact from training knowledge is marked, and task 32
   turns each into a check with "what changes if false". The RuntimeClass name becomes
   configuration instead of a guess.

---

## Issues

### Critical

**C1. The edge namespace is sealed by its own default-deny, so the environment has no front door.**
- **Where:** `task-11.md:103-104` puts `NetworkPolicy/default-deny` (Ingress **and** Egress) in all
  four namespaces, `panacea-edge` included. `task-13.md:14-18` lists the edge chart's templates
  (`gateway`, `certificate`, `clusterissuer`, `pdb`) — no NetworkPolicy. A grep of every part for
  `NetworkPolicy|netpol|toEntities` finds policies only in `task-29.md:72-77` (broker) and
  `:133-135` (Hub). The broker policy admits "ingress from the Traefik pods in `panacea-edge`",
  which assumes Traefik can send.
- **What goes wrong:** with `platform` installed first (task 30 stage 4), Traefik receives nothing
  from the load balancer, cannot watch Gateway/HTTPRoute objects on the API server, and cannot
  reach the broker or Hub. cert-manager cannot reach the API server, Let's Encrypt, Azure DNS or
  Entra, and the API server cannot reach cert-manager's webhook. The cert-manager chart's
  post-install API check then fails, and `helm upgrade --install --atomic --wait` rolls the edge
  back. **A second cause stacks on top** (`task-13.md:29-36`, `:90-93`, `:121-127`): cert-manager is
  a subchart with `crds.enabled: true`, and the `ClusterIssuer` and `Certificate` are templates of
  the same release. On a first install Helm cannot map `cert-manager.io/v1` kinds that do not
  exist yet, and even with the CRDs present the `ClusterIssuer` create hits a webhook that is not
  ready (from training knowledge of Helm and cert-manager; this is a well-known first-install
  failure).
- **Why it matters:** nothing reaches the broker or the Hub, and no test catches it:
  helm-unittest renders one chart at a time and cannot see a policy that another chart lays down.
- **Fix:** (a) add `templates/networkpolicy.yaml` to the edge chart: Traefik ingress on 8443 from
  `allowedSourceRanges` (client IPs survive, `externalTrafficPolicy: Local`), Traefik egress to
  `panacea-broker`/`panacea-hub` on 8080 and to the API server, cert-manager egress 443 to the
  world minus RFC 1918 and to the API server, webhook ingress from the API server (the same
  `toEntities: [kube-apiserver]` / ipBlock fallback task 29 uses; for the webhook, Cilium's
  `fromEntities` for the control-plane tunnel is UNVERIFIED and belongs in task 32). (b) Split
  cert-manager into its own release (`deploy/charts/cert-manager`, or the upstream chart with
  pinned values) installed and waited on before `edge`; task 30's stage 4 order becomes
  `platform → cert-manager → edge → broker → hub`. (c) Add a test that renders `platform` and
  `edge` together and asserts every namespace that `platform` default-denies has at least one
  allow policy for each workload in it.

**C2. The CI agents' subnet has no egress, so the Managed DevOps Pool can never pick up a job.**
- **Where:** `task-03.md:150-151` sets `default_outbound_access_enabled = false` on **every**
  subnet; `:143` defines `local.node_subnets` as system/apps/sandbox; `:166-168` associates the NAT
  gateway with those three only; the run `nat_mode_puts_nat_on_all_three_node_subnets`
  (`:197`) pins exactly three. In firewall mode `task-07.md:43-46` routes only the node subnets.
  `task-30.md:42-45`, `:77-87` puts the pool in `snet-agents`.
- **What goes wrong:** agents in `snet-agents` have no route to `dev.azure.com`, the Terraform
  registry, ACR's public endpoint in dev, or anything else. The pool never registers an agent.
  `task-07.md:153-157` also claims "the NAT gateway's own /32 is added automatically, so the
  in-VNet agent pool reaches the API": that is true only if the agents egress through the NAT,
  which they do not.
- **Why it matters:** prod is a private cluster (I5, OQ-5), so the in-VNet pool is the **only**
  path for `deploy` in prod, and dev uses it from runbook step 4 onwards. The plan's goal line
  ("one reviewed pipeline run stands it up") fails at the first pipeline run, and the test in
  task 03 would kill the fix.
- **Fix:** associate the NAT gateway with `snet-agents` in nat mode, and add it to
  `routed_subnet_ids` in firewall mode with an ADO/agent FQDN rule (`dev.azure.com`,
  `*.visualstudio.com`, `*.vsassets.io`, `vstsagentpackage.azureedge.net`, the Terraform registry
  and the Helm repos the pipelines use — a starting list from training knowledge, UNVERIFIED).
  Change the task 03 run to "four associations: the three node subnets and agents; none on
  private endpoints".

**C3. `infra-plan` is designed to run as PR validation with the apply identity.**
- **Where:** `task-30.md:107-118`: `infra-plan.yml` is "runnable by hand and as PR validation
  once registered", and authenticates with `$(PANACEA_APP_<ENV>_SERVICE_CONNECTION)` (`:115`) —
  the same connection `deploy.yml` uses. `task-31.md:27` registers it and makes every PR pipeline
  build validation. That identity holds role-assignment rights (owner inputs, plan `:304`),
  AcrPush and **AKS RBAC Cluster Admin** (`task-07.md:354-356`), and is a member of the Postgres
  admin group (`task-30.md:131-133`).
- **What goes wrong:** a PR build runs the PR branch's own YAML and scripts, so the estate guard
  (`task-30.md:89-103`) and `guards.sh` are code the PR author can edit — they are not a security
  boundary. Anyone who can open a PR can run arbitrary steps with an identity that can assign
  roles in the subscription, read every Secret in the cluster (every user's attach token and
  machine token, so every user's agent session), and read or change the Hub's data.
- **Why it matters:** this is a designed-in path from "can open a PR" to "owns the environment
  and every user's sessions".
- **Fix:** two service connections per environment. `plan`: Reader on the subscription plus
  Storage Blob Data Reader on the state container, used by PR builds with `-lock=false`, so a
  malicious PR can read, not write. `deploy`: the write identity, protected by ADO-side checks the
  PR cannot edit — branch control (`refs/heads/main`, `refs/heads/dev`) on the connection and the
  Environment approval, with pipeline permissions granted to `deploy.yml` only. The same branch
  control goes on the ACR push connection that `images.yml` uses (`task-27.md:55-57`). Add a guard
  that `required_providers` sources are only `hashicorp/azurerm`, `hashicorp/azuread` and
  `azure/azapi` (a `data "external"` runs a program at plan time), and state plainly in task 30
  that guards are hygiene, not a boundary.

### Important

**I1. "Every committed tfvars is proven to plan" is false: a test file's variables override `-var-file` (verified).**
- **Where:** `task-01.md:59-63` runs each stack's tests once per `<env>.tfvars` and says the test
  files "supply only what tfvars may not hold". `task-02.md:126-135` then sets `environment`,
  `location`, `unique_suffix` and `state_public_network_access` at file level — every key the
  tfvars hold. `task-07.md:427` (`nat_mode_allowlists_its_own_address_on_the_api`) is dev-shaped.
- **What goes wrong:** the Terraform 1.15.8 probe above shows file-level `variables` win over
  `-var-file`. So bootstrap's "prod" run is a second dev run and proves nothing about
  `prod.tfvars`. In foundation the opposite happens: under `prod.tfvars` (`api_private = true`) the
  cluster has no authorised ranges, so the NAT-/32 run fails, and setting `api_private = false` in
  the run trips invariant I5 (`environment = "prod"`). The implementer meets a red test the part
  says is green, or silently tests one environment twice.
- **Fix:** in task 01, add guard G9: a stack's `.tftest.hcl` file-level `variables` block may hold
  only keys that no `<env>.tfvars` in that stack sets. Every run that depends on an
  environment-specific value sets it in its own `variables` block, including `environment`. Fix
  task 02's test file to the contract. Add a `guards_test.sh` case for G9.

**I2. The runbook's first command cannot run: bootstrap's partial azurerm backend blocks a local-state apply (verified).**
- **Where:** `task-31.md:26` (§2): "`init -backend=false` is not enough here, so `terraform init`
  with no backend config, `plan`, `apply`". `task-02.md:29-32` commits `backend "azurerm" {}`, and
  guard G5 requires it.
- **What goes wrong:** the probe above: `terraform init -input=false` on that block errors with
  `One of access_key, sas_token, use_azuread_auth and resource_group_name must be specified`, and
  `plan` after `init -backend=false` refuses. The first step of the first apply stops.
- **Fix:** runbook §2 writes an uncommitted `stacks/bootstrap/backend_override.tf` holding
  `terraform { backend "local" {} }` (the probe confirmed an override file replaces the backend),
  runs init/plan/apply with local state, then deletes the override and runs
  `terraform init -migrate-state` with the real `-backend-config` flags. `.gitignore` gains
  `*_override.tf`, and guard G1 gains "no `*_override.tf` is tracked".

**I3. The workstation path skips the estate guard, and nothing positively identifies our subscription as not Panacea's.**
- **Where:** `task-31.md:28` lets "the operator's workstation run `tf-stack.sh` directly";
  `task-30.md:19` gives `tf-stack.sh` no guard call; the guard (`:89-103`) lives only as a pipeline
  step. Its subscription check compares `ARM_SUBSCRIPTION_ID` with a pipeline variable from the
  same ADO project, which catches dev/prod swaps (`:144`) but not a connection pointed at a
  Panacea subscription by mistake. Runbook §2 (bootstrap) runs from a workstation before any guard
  exists.
- **Why it matters:** the operator runs Panacea day to day, so a shell whose `ARM_SUBSCRIPTION_ID`
  or `az` context is Panacea's is the likely accident. The names do not collide, so nothing is
  destroyed, but bootstrap, a subscription-scoped budget (task 04) and role assignments would land
  in Panacea's subscription. This is the "applied against Panacea by mistake" case the review was
  asked to look for.
- **Fix:** `tf-stack.sh` calls `estate-guard.sh` itself, on every path. Add a **positive
  fingerprint**: at handover the owner tags the subscription `panacea-app-estate=<env>` (runbook
  §1). The guard reads the tag (`az tag list --resource-id /subscriptions/$ARM_SUBSCRIPTION_ID`)
  and refuses unless it equals `<env>`. A Panacea subscription never carries the tag, and no id is
  committed (G2 holds). Add the case "subscription without the estate tag → refused" to
  `estate-guard_test.sh`. Hardening in the same spirit: `validate.sh` (`task-01.md:71-74`) also sets
  `AZURE_CONFIG_DIR` to an empty temp directory, so no `az login` context is reachable from a test.

**I4. The pre-pull DaemonSet asks for a whole node and can exhaust the namespace quota.**
- **Where:** `task-11.md:60-62`: the `LimitRange` in `panacea-sandboxes` sets only `max`
  (`8` CPU, `32Gi`). `task-11.md:100-101`: a `ResourceQuota` on requests. `task-12.md:62-67`: the
  pre-pull pod's init containers `pull-sandbox` and `pull-machine` have **no resources** (only
  `hold` does).
- **What goes wrong** (from training knowledge of Kubernetes, UNVERIFIED here): a Container
  LimitRange with `max` and no `default` gets `default = max` and `defaultRequest = default` at
  admission, and LimitRanger defaults init containers too. Each pre-pull pod then requests 8 CPU /
  32 GiB, more than a `Standard_D8s_v5` can allocate. It stays Pending on every sandbox node, and
  Pending pods count against the quota. At the dev maximum of four nodes, four such pods hold
  128 GiB, the namespace's whole `requestsMemory` (`task-11.md:56`), so every session create is
  refused by quota.
- **Fix:** explicit `requests == limits` (`10m`/`16Mi`) on both init containers, and a LimitRange
  `default`/`defaultRequest` at the session size, so an unsized container can never take a node.
  Add a helm-unittest case: every container **and init container** in every pod template in
  `platform` has `requests == limits` (a Kata rule anyway, `PROGRAM.md:133`).

**I5. A cloud machine is capped at one node's capacity, and the plan neither states the cap nor handles hitting it.**
- **Where:** `task-15.md:97-99` (required pod affinity on `kubernetes.io/hostname` to the machine
  pod), `task-19.md:26` (machine pod `500m`/`1Gi`), `task-17.md:40-44` (sessions 2 CPU / 4 GiB,
  `MAX_SANDBOXES_PER_USER = 8`), `task-07.md:380` (`Standard_D8s_v5`), `task-12.md:10-13`
  (balloons "so the next session also finds room").
- **What goes wrong:** every session of a machine must fit on the machine pod's node. On a
  D8s_v5 (8 vCPU, allocatable under 8), a machine holds about three 2-CPU sessions once the
  machine pod and per-pod Kata overhead are counted (arithmetic; the overhead is task 32's P07,
  UNVERIFIED). Session four is Pending forever: the autoscaler cannot help, because a new node
  does not hold the machine pod. A balloon helps only if it sits on that node, and after one
  preemption it moves elsewhere. The broker's create waits `READY_TIMEOUT` (120 s) and answers
  503 `not_ready`, and katabe **leaves the Pending pod** (`TestCreateTimesOutWithErrNotRunningAndLeavesThePod`,
  `kata_test.go:379`), so it still counts against the user's cap.
- **Why it matters:** parallel agents per machine is the product (`CLAUDE.md`: "run coding agents
  in parallel, one git worktree per agent"). A user who opens a fourth session sees a two-minute
  hang and a generic error.
- **Fix:** state the ceiling in P8-D6 and the cost model. Add a broker check before create:
  sum the requests of the machine's pods plus the Kata overhead read from the RuntimeClass,
  compare with the node's allocatable, and answer `409 machine_full` with
  `details.sessionsPerMachine`. Map it in remotebe and give the cockpit a sentence for it. On a
  create timeout the broker deletes the Pending pod. Make P07 (`task-32.md:28`) decide the SKU
  (D16s/D32s_v5) or a smaller default session. Record that the balloons in `task-12.md` warm new
  **machines**, not a busy one.

**I6. The laptop's reaper will destroy another laptop's cloud sessions.**
- **Where:** `task-23.md:84-95` extends the reaper to sweep each cloud machine in `Known()`
  against that machine's backend. The reaper destroys any listed sandbox that no registry record
  names (rule 4, `destroyOrphan`, `control-plane/internal/reaper/reaper.go:456-462`). The broker
  scopes by **owner** (`tid/oid`), not by laptop, and `task-25.md:79` (P3A-2) already concedes "a
  second laptop signed in as the same user".
- **What goes wrong:** the user signs in on a second PC, or reinstalls and gets a fresh
  registry. That control plane lists the machine's sandboxes, finds sessions it has no record of,
  and destroys them within one reap interval. Running agents are killed without warning.
- **Fix:** stamp each cloud sandbox with the creating control plane's install id (a random id
  under `statepath`, sent as a label in `SandboxSpec`, and filtered on by the broker on request).
  The reaper's rule 4 acts only on sandboxes carrying its own install id. Add the test
  `TestTheReaperNeverDestroysAnotherControlPlanesCloudSandbox`. State the single-laptop limitation
  for sessions as well as lanes.

**I7. `machined` closes two of git's many ways to run agent-written code, and then hands git the user's host tokens.**
- **Where:** `task-21.md:66-71` adds `-c core.hooksPath=/dev/null -c core.fsmonitor=false`
  because "a lane clone's `.git/hooks` and `core.fsmonitor` are written by an agent".
  `:28-30`, `:87`: `Env` accepts any `^GIT_[A-Z0-9_]+$` key. `:76-78`: the user's host token
  arrives in `Env` for clones (and for phase 3's push, `task-25.md:114-115`).
- **What goes wrong:** the agent owns the lane clone's `.git/config` and its `.gitattributes`.
  Other config keys also run commands: `filter.<x>.clean`/`smudge`/`process` (run by `status`,
  `diff` and `checkout` when the tree names that filter), `diff.<x>.textconv`, `diff.external`,
  `core.sshCommand`, `credential.helper`, `gpg.program`, merge drivers, `include.path`. Any of them
  runs agent-chosen code **in the machine pod**. That pod holds the user's whole disk (every
  project, lane and conversation, where a session sees only its own sub-paths) and receives the
  user's ADO/GitHub tokens. The `GIT_*` allowlist also admits `GIT_DIR`, `GIT_WORK_TREE`,
  `GIT_EXEC_PATH`, `GIT_SSH_COMMAND` and `GIT_EXTERNAL_DIFF`, which undo the `Dir` confinement
  that `TestADirOutsideTheRootIsRefused` claims to pin.
- **Why it matters:** the escalation is from "one session sees one lane" to "the agent sees the
  user's whole machine and git credentials". It stays inside one user, but it is exactly the
  boundary P8-D6's sub-path mounts exist to draw.
- **Fix:** (a) an explicit `Env` allowlist: `GIT_CONFIG_COUNT`, `GIT_CONFIG_KEY_<n>`,
  `GIT_CONFIG_VALUE_<n>` with the key restricted to `http.extraHeader` or
  `http.<url>.extraHeader`, `GIT_TERMINAL_PROMPT`, `GIT_OPTIONAL_LOCKS`. (b) Before any call whose
  `Dir` is under `lanes/`, read `git config --local --no-includes --list` and refuse unless every
  key is in an allowlist (`core.*` non-exec keys, `remote.*`, `branch.*`, `user.*`, `extensions.*`),
  and refuse credential `Env` for `lanes/` directories. Pushes run from `projects/`, whose config
  no session can write. (c) Start git with a scrubbed environment: no `PANACEA_APP_MACHINED_TOKEN`.
  (d) Tests: a lane with `filter.x.clean` and `* filter=x` → `status` refused and no marker file;
  `GIT_DIR` in `Env` → 400.

**I8. The deploy identity is one principal holding everything, and the plan never lists what it holds.**
- **Where:** owner inputs (plan `:304`): role-assignment rights limited by ABAC to the allowlist.
  `task-07.md:354-356`: AcrPush, AKS RBAC Cluster Admin, Cluster User. `task-30.md:131-133`: a
  member of the Postgres admin group. `task-09.md:5-9`: the identity stack runs under it "if the
  owner grants the deploy identity the rights". `task-04.md:123-125`: "Cost Management
  Contributor … (task 10's allowlist says so)" — but `task-10.md:149-159` does not list it (it is a
  role the identity **holds**, not one a stack **assigns**, and no table of held roles exists).
- **Why it matters:** the same principal can reshape Azure RBAC, read every Secret in the cluster
  (every user's session), read all Hub data as a database administrator, and — if it runs the
  identity stack — hold tenant-level Graph permissions to create applications and grant consent,
  which in Entra is close to tenant administration (from training knowledge, UNVERIFIED).
- **Fix:** add a "deploy identities" table to the plan: for each principal, the roles it holds and
  at what scope. Split them: an **infra** identity (Contributor on the product resource groups, the
  RBAC Administrator role with the ABAC condition, Cost Management Contributor, state Blob Data
  Contributor); a **cluster** identity (AKS RBAC Admin scoped to the four product namespaces plus
  the cluster-scoped kinds the charts need — CRDs, PriorityClasses — rather than Cluster Admin);
  the plan identity of C3. The database bootstrap (roles and principals, once per environment) is
  a runbook step run by a **human** member of the Postgres admin group. No pipeline identity is in
  that group. The identity stack is **owner-run only**. Correct `task-04.md:123-125`.

**I9. The first deploy installs the Hub before the database roles exist.**
- **Where:** `task-30.md:126-130`: stage 4 installs `platform → edge → broker → hub`; stage 5
  (`:131-133`) creates the roles and principals "**before** the `hub` chart's first install".
  `task-29.md:127-131`: the Hub chart's `pre-install` hook runs `hub migrate` as `hub-migrate`,
  which needs `hub_owner` (Hub contract §10.1, `:1414-1430`).
- **What goes wrong:** on the first deploy the migrate Job fails (no role, no principal),
  `backoffLimit: 0` and `--atomic` roll the Hub back, and the stage fails. Also, stage 5 is
  described as idempotent, but the Hub's `roles.sql` begins `CREATE ROLE hub_owner NOLOGIN;`
  (contract `:1421`), which errors on a second run.
- **Fix:** order the stages `guard → approve → apply → database → cluster → smoke`, or split the
  Hub out of stage 4. Wrap the role creation in `DO $$ … IF NOT EXISTS … $$` (an ask of the Hub
  contract, recorded in task 33). Prefer `pgaadauth_create_principal_with_oid` with the identity's
  object id from the workloads outputs over name lookup: user-assigned identity names are not
  unique in a tenant (from training knowledge, UNVERIFIED; task 29 already marks the signature
  unverified).

**I10. The firewall allowlist is described as an egress control it cannot be.**
- **Where:** `task-07.md:94-98`: rule 2 allows `*.azurecr.io` and the AKS FQDN tag; rule 3 allows
  the agent list from `var.source_cidrs`, which is the whole VNet (`:47-50`: nodes SNAT pod traffic
  under Overlay). `:412`'s test forbids `*.blob.core.windows.net` "that lets a sandbox exfiltrate to
  any storage account".
- **What goes wrong:** `*.azurecr.io` also matches any other tenant's registry and its data
  endpoint (`<name>.<region>.data.azurecr.io`), so the test guards a door the rule leaves open.
  Because sandbox and node traffic leave with the same node address, sandboxes get every rule
  meant for nodes (`management.azure.com`, `login.microsoftonline.com`, the AKS FQDN tag), and
  `github.com` plus `*.githubusercontent.com` accept arbitrary uploads anyway.
- **Why it matters:** Q1 asks the owner to choose Firewall (about A$1.5–2k a month) as the
  PROTECTED-grade answer. The choice should be made knowing that it limits *where* agents go, and
  does not stop a determined agent from sending data out.
- **Fix:** use `module.registry.login_server` exactly, not `*.azurecr.io`, and add a test that no
  FQDN in the ACR rule starts with `*`. Say in Q1 and task 07 that sources cannot be told apart
  under Overlay SNAT, and that real per-sandbox FQDN control needs Cilium FQDN policy on pod
  identity (the Advanced Container Networking option the plan already names as UNVERIFIED).

### Minor

Ranked; each is one edit.

**Citations and internal consistency**
- **m1. ADR-H1 line citations have drifted.** The plan read ADR-H1 at `67609ab` (653+ lines); it
  is now 776 lines. Checked: `:543-546` (plan `:255-257`, "none is ever forwarded") now holds the
  deploy summary, and the sentence is at `:552`. `:439-449` (C3's key, plan `:258-259`,
  `task-26.md:139-141`) now opens "Who calls the Hub"; the key is at `:449`. `:286-288` (plan
  `:250`) is a section heading. The same applies to `task-16.md:5-6` and `task-18.md:157`. **Fix:**
  cite `67609ab:<path>:<line>` or § numbers.
- **m2. The wave table breaks its own dependencies** (plan `:484-491`). Wave 1 holds 01 and 28, but
  28 depends on 01, and its `infra.yml` also calls `role-allowlist.sh`, which task 10 (wave 6)
  writes. Wave 8 holds 31, 32 (which depends on 31) and 33 (which depends on all). **Fix:** move 28
  to wave 2, then 32 and 33 into waves 9 and 10.
- **m3. The shared-file table misses one edit.** `task-18.md:167` (step 3) edits
  `backend.go:29-37`; the table (plan `:389`) lists `backend.go` for 14 → 15 only, and
  `task-22.md:24` says `backend.go` "is not touched again". **Fix:** add 18 to the row.
- **m4. P8-D6 (plan `:132`) still says `managed-csi-premium` (LRS).** Task 11 creates
  `panacea-machine-disk` on `Premium_ZRS` (`task-11.md:65-67`), and task 17 makes that class the
  default. ZRS is the right call, because sandbox pools span three zones. **Fix:** update P8-D6.
- **m5. Q3 (plan `:294`) says one variable feeds two things.** It says `allowed_ingress_cidrs`
  feeds both `loadBalancerSourceRanges` and the dev API server. Task 07 (`:153-157`, `:346-347`)
  uses a separate `api_authorized_ip_ranges` for the API server, and the writer's return repeats
  Q3's version. **Fix:** pick one; keeping the two separate is better, since admin workstations
  are not edge users.
- **m6. OD-11's row (plan `:104`) overclaims the licence gate.** It says the gate covers
  Terraform providers and Helm chart dependencies. Task 28 gates Go, npm and Cargo only, and task
  13 records chart licences in comments. **Fix:** add a `Chart.lock` licence check to
  `charts.yml`, or narrow the claim.
- **m7. P8-D11 (plan `:137`) says "no long-lived secret anywhere".** State still holds the Log
  Analytics shared keys and the state account's access keys, which exist but are disabled for use
  (from training knowledge of azurerm's exported attributes). **Fix:** say "no usable long-lived
  secret", and treat state as sensitive.

**Feasibility**
- **m8. The AKS create can race the NAT association.** `userAssignedNATGateway` needs the
  subnet's NAT association at create time (from training knowledge). `module.network.subnet_ids`
  (`task-03.md:93-105`) come from `azurerm_subnet`, not from the associations. **Fix:** output the
  ids through the association resources, or add `depends_on = [module.network]` on the AKS call
  (`task-07.md:343-349`).
- **m9. Smoke check S50.2 tests nothing of P8-D12.** It runs in `panacea-smoke`
  (`task-31.md:40`, `:52`), which has no default-deny and none of the sandbox policies, so IMDS
  answers and S50.2 fails for the wrong reason. **Fix:** probe from `panacea-sandboxes` with the
  sandbox labels, or defer to P08.
- **m10. The balloon priority sits exactly on the cutoff.** `-10` (`task-11.md:98`) equals the
  cluster autoscaler's default expendable cutoff; anything lower stops triggering scale-up (from
  training knowledge). **Fix:** add a test that pins `value >= -10`, with the reason.
- **m11. Broker concurrency has check-then-create races.** Two replicas (`task-29.md:19`) each run
  the sleeper (`task-19.md:88-95`). Ensure-default (`:71`) is check-then-create with a minted id
  (`:47`), and the per-user caps are check-then-create too. **Fix:** a default-machine id derived
  from the owner (`m-` + 8 hex of `sha256(owner + "/default")`), so a lost race is
  `AlreadyExists` → 200; a `coordination.k8s.io` Lease for the sleeper (the Role gains `leases`);
  `TestConcurrentEnsureDefaultMakesOne`.
- **m12. Machine objects carry no owner segment in their name.** They are `machine-<id>`
  (`task-19.md:24-27`), unlike task 14's owner-prefixed sandbox names, so a conflict on create can
  reveal another owner's id. The impact is low because the broker mints the ids. **Fix:** retry on
  conflict, or prefix the name as task 14 does.
- **m13. Machine disks have no data protection.** The claim is `reclaimPolicy: Delete`
  (`task-11.md:126`); the snapshot controller is on (`task-06.md:226-228`) but there is no
  `VolumeSnapshotClass` and no schedule. Unpushed lane work and conversations live only on that
  disk. **Fix:** add a daily snapshot, or record it as a seam.
- **m14. One code comment goes stale.** `backend.go:76` comments `KubeLabelComponent` as
  `// always "sandbox"`; tasks 12 and 19 add `balloon` and `machine`. **Fix:** update the comment.
- **m15. The sandbox base image is past end of life.** It is `node:20-bookworm-slim`
  (`images/sandbox/Dockerfile:10`), and Node 20 reached end of life on 2026-04-30 (from training
  knowledge). Task 27 publishes it to a regulated environment for the first time. **Fix:** record a
  base upgrade as its own task, since it moves the OD-8 pin.
- **m16. The `.invalid` placeholder may not fail where the plan says.** It is said to "fail at
  apply" (`task-07.md:401-402`), but Azure DNS may accept the zone (UNVERIFIED either way), and
  then cert-manager fails later and less clearly. **Fix:** make the estate guard refuse any
  `.invalid` value on `apply`.

**Sizing**
- **m17. Tasks 24 and 07 are underestimated.** Task 24 changes four catalog routes, adds
  `machines.go`, `cloudstore`, wake-before-create and the OD-10 fallback, with 14 tests. Task 07 is
  a module plus a nine-file stack whose test runs take minutes each, twice over. Each is nearer
  5 h than 3 h. **Fix:** split task 24 into "list, ensure-default, wake" and "catalog, create,
  cloudstore".
- **m18. Reduced motion is not tested for the waking ring.** Task 26's design-system table omits
  `prefers-reduced-motion` (`DESIGN-SYSTEM.md:90`). **Fix:** add one test that the waking ring and
  counter respect it.

---

## Assessment

**Test strategy.** Strong where it can be. Red-before-green is a step in every task, every test
names its mutant, and negative cases are everywhere: `expect_failures`, 404 for another owner,
escaping paths, tokens absent from logs. The gaps are the ones unit tests cannot see: cross-chart
behaviour (C1), `terraform test`'s variable precedence (I1), concurrency in the broker (m11), and
multi-laptop behaviour (I6). A render-both-charts policy test (C1 fix c) and G9 (I1) close the
first two cheaply.

**Security.** The in-cluster design is sound: owner scoping, a namespaced Role, no exec, no
tokens forwarded, `restricted` Pod Security, IMDS blocked. The weaknesses are all at the edges of
trust: the CI identity model (C3, I8), the machine pod as a place where agent-written git config
runs (I7), and the firewall's real strength (I10).

**Kata constraints.** Explicit resources everywhere except the pre-pull init containers (I4); no
host networking; the Defender gap is recorded. Warm-pool sizing is honestly left to P06/P07, but
the one-node machine ceiling (I5) is a design limit, not a sizing number, and belongs in P8-D6
now.

**Design system (task 26).** Compliant. The one heartbeat, no spinner, tabular numerals, accent
only on the current row, `Popover`'s existing elevation, and owner's-eyes acceptance with the
branch left unmerged. Its `DESIGN-SYSTEM.md` line citations (`:27-29`, `:38`, `:61-64`, `:73`,
`:90`) were opened and are right. Only m18 is open.

**What is missing, in one place:** an edge NetworkPolicy and a separate cert-manager release (C1);
egress for the agents subnet (C2); a plan/apply identity split with ADO-side checks (C3); a table
of what each deploy identity holds (I8); a positive subscription fingerprint (I3); a per-machine
session ceiling (I5); machine-disk snapshots (m13).

### The two defects a first implementer is most likely to hit

1. **The edge will not come up (C1).** Whoever runs task 30's `deploy` for the first time installs
   `platform`, and its default-deny silences `panacea-edge`. In the same `edge` release,
   cert-manager's CRDs and webhook race its own `ClusterIssuer`. **The plan does not prevent
   this.** No part writes an edge policy, and helm-unittest renders each chart alone, so every
   suite is green.
2. **`terraform test` behaves differently from what the parts say (I1, verified).** The task 02
   implementer copies the test file as written, and `validate.sh` reports "OK" for `prod.tfvars`
   while testing dev twice. The task 07 implementer then hits a red NAT-/32 run under
   `prod.tfvars` that the part promises is green, and cannot fix it without tripping invariant I5.
   **The plan does not prevent this.** Task 01 states the right rule in a comment
   (`task-01.md:62-63`), but nothing enforces it, and task 02 breaks it.

A close third, also verified: runbook §2's bootstrap `terraform init` fails on the committed
partial backend (I2).

**Plan quality: Needs fixes**
