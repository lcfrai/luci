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

---

## Fix batch b01 (2026-09-24) — C1, C2

Commits on `feat/env`: `436e004`, `99b948d` (C1), `4c67af0` (C2), and the commit that adds this
section. Paths below are relative to `docs/superpowers/plans/`; `parts/` is
`2026-09-23-phase-8-own-environment-parts/`, `plan` is `2026-09-23-phase-8-own-environment.md`.
Line numbers are as of that last commit.

### C1 — the edge namespace sealed; cert-manager racing its own issuer

The review's fix (a), (b) and (c) were taken as written, with three additions found while
applying them: cert-manager's policies live in its own chart, because its `startupapicheck` hook
runs inside the cert-manager release, before `edge` exists; the controller also needs UDP/TCP 53
to the world for the DNS-01 self-check (training knowledge, UNVERIFIED); and the Hub's migrate
Job needs an allow too, which the new test would otherwise fail on.

| Fix | Where |
|---|---|
| (b) cert-manager is its own release, before `edge`; why, in the task's words | `parts/task-13.md:1`, `:10-17` |
| (b) the `cert-manager` wrapper chart: `Chart.yaml`, `values.yaml` (`identity.clientId`, the `apiServer` seam, `webhook.securePort`, `crds.keep`, `startupapicheck`) | `parts/task-13.md:42-100` |
| (a) cert-manager's `serviceaccount.yaml` and `networkpolicy.yaml` (DNS, API server, controller egress 443 + 53 minus IMDS/wire server/RFC 1918, webhook ingress from the API server) | `parts/task-13.md:101-124` |
| (b) `edge` loses the cert-manager dependency; values gain `apiServer` and `backends`; Traefik's update checks off | `parts/task-13.md:126-219` |
| (a) the edge's `networkpolicy.yaml`: `traefik-ingress` (8443 from `allowedSourceRanges` only), `traefik-backends` (broker/Hub on 8080), `traefik-api-server`, no internet egress | `parts/task-13.md:236-253` |
| tests: 3 new edge runs, 8 cert-manager runs | `parts/task-13.md:255-283` |
| (c) `deploy/tests/netpolcover`, the cross-chart render test (4 cases), its `go.work` entry and its CI job | `parts/task-13.md:19-25`, `:38-40`, `:285-300` |
| steps: the red run that reproduces C1 before any policy exists; named mutants | `parts/task-13.md:304-320` |
| stage 4 is `platform → cert-manager → edge → broker → hub` | `parts/task-30.md:135-141` |
| one Terraform output feeds both releases; the helm-values test asserts it | `parts/task-10.md:137-138`, `parts/task-30.md:159-163` |
| default-deny names which chart brings each namespace's allows | `parts/task-11.md:103-109` |
| broker/Hub ingress peers use Traefik's pod labels; the Hub's egress also selects the migrate Job; task 29 adds `broker`/`hub` to `netpolcover` | `parts/task-29.md:13-14`, `:73-76`, `:136-142`, `:201-204`, `:212` |
| `charts.yml` gains the `netpolcover` job (added by task 13) | `parts/task-28.md:58` |
| proving rows for the two UNVERIFIED Cilium facts: P20 (`kube-apiserver` entity), P21 (the webhook's source through the AKS tunnel) | `parts/task-32.md:16`, `:41-42`, `:64` |
| restatements: the chart list, the file map, task 13's row (depends on 11, 28; 3.5 h), `go.work` and the new module in "Shared files" | `plan:71-74`, `:189-191`, `:349-351`, `:394-397`, `:437` |

### C2 — `snet-agents` has no egress

The review's fix was taken as written. One addition: in firewall mode the API server's
authorised ranges did not include the firewall's egress IP, so a dev cluster in firewall mode would
have locked out the pool even with a route. Both modes now add the egress /32.

| Fix | Where |
|---|---|
| `local.egress_subnets` = the three node subnets + `agents`; output `egress_subnet_ids` | `parts/task-03.md:116-119`, `:148-153` |
| one NAT association per egress subnet (`for_each`, nat mode only) | `parts/task-03.md:176-181` |
| the run becomes `nat_mode_puts_nat_on_every_egress_subnet`: four associations, none on private endpoints | `parts/task-03.md:210` |
| firewall mode routes the same four (`routed_subnet_ids`, validated to hold `agents`) | `parts/task-07.md:43-50`, `:145-149`, `:392-393` |
| a CI FQDN rule from the agents subnet only (`ci_agent_source_cidrs`, `ci_agent_allowed_fqdns`, UNVERIFIED starting list) | `parts/task-07.md:51-89`, `:142-143` |
| the API server allowlists the egress /32 in either mode; the claim at old `:153-157` corrected | `parts/task-07.md:202`, `:396-398` |
| one run renamed and widened (`every_egress_subnet_is_routed`) and three new: `ci_agents_get_their_allowlist_from_their_subnet_only`, `firewall_mode_allowlists_its_own_address_on_the_api` (pins dev + public, so it means the same under either tfvars file), and `agents_egress_in_both_modes` (read through the module's new `routed_subnet_keys` output) | `parts/task-07.md:130`, `:463-464`, `:481-482` |
| what the list does not claim; the dev state account's blob host | `parts/task-07.md:512-515` |
| the ci-pool contract says where its egress comes from | `parts/task-30.md:89-95` |
| proving row P22: a real pipeline through the real egress | `parts/task-32.md:43`, `:64` |
| restatement: the architecture diagram's egress line | `plan:61` |

### Still UNVERIFIED after this batch

- AKS-managed Cilium's `kube-apiserver` entity, and which source the webhook's calls arrive from
  (P20, P21). The `apiServer.mode: cidr` seam is the fallback, and a wrong guess fails closed at
  `--atomic`.
- The cert-manager and Traefik chart key names and pod labels; `netpolcover` checks the rendered
  pods, not the keys.
- The CI FQDN list, and whether a delegated `Microsoft.DevOpsInfrastructure/pools` subnet takes a
  NAT gateway or a route table (P22).

## Fix batch b01, round 2 (2026-09-24) — the re-review's I1-I4, M1-M5

Against `.superpowers/sdd/env/plan-rereview-b01-1.md` (local). Commits `77edf29` (I1-I4, M1-M3)
and `2941429` (M4, M5). Paths are relative to `docs/superpowers/plans/`; `parts/` is
`2026-09-23-phase-8-own-environment-parts/`. Every fix took the re-review's "how to fix", except
where noted.

| Finding | Change | Where |
|---|---|---|
| **I1** migrate Job sealed on first install | the Job's `ServiceAccount` and egress policy are `pre-install,pre-upgrade` hooks at weights `-10`, `-5`, Job `0`, `before-hook-creation`; the regular policy selects serving pods only | `parts/task-29.md:137-162` |
| I1 | new per-chart test `hub / migrate hooks exist before the Job` | `parts/task-29.md:209` |
| I1 | new cross-chart case `hook_pods_are_allowed_before_they_run` (phase-aware: a `post-install` hook may use regular templates, which is why cert-manager's `startupapicheck` still passes) | `parts/task-13.md:348` |
| I1 | named mutants: the policy as a regular template; the SA without hook annotations | `parts/task-29.md:227-230` |
| **I2** direction-blind coverage | `every_denied_workload_has_an_allow` counts Egress for every pod and Ingress for Service- or webhook-backed pods | `parts/task-13.md:346` |
| I2 | new cert-manager run `every_component_reaches_dns_and_the_api_server` | `parts/task-13.md:326` |
| I2 | both deletions named as mutants (record that `netpolcover` stays green), plus a direction mutant on Traefik | `parts/task-13.md:380-385` |
| I2 | a consequence of counting ingress: cert-manager's metrics `Service` would need an ingress allow nobody uses, so `prometheus.enabled: false` (nothing scrapes in this environment) | `parts/task-13.md:89-90` |
| I2 | the same gap on the broker: `broker-api-server` deleted → `broker / the API server is reachable in both modes` | `parts/task-29.md:231-233` |
| **I3a** broker cannot switch | the broker gets the same `apiServer` block and names its policy `broker-api-server`; switching is one value per chart | `parts/task-29.md:47-49`, `:79-87` |
| I3a | broker test `broker / the API server is reachable in both modes` | `parts/task-29.md:205` |
| **I3b** nothing supplies the addresses | `apiServer.cidrs`: new `pipelines/scripts/api-server-cidrs.sh` reads the `default/kubernetes` EndpointSlice after `get-credentials` and never prints an empty list, with a 4-case test. `webhookSourceCidrs`: committed per environment from P21 | `parts/task-13.md:75-78`, `:134-142`; `parts/task-30.md:21`, `:140`, `:177-190`, `:200`, `:209` |
| **I3c** an empty list fails open | three guards: schema `if`/`then` `minItems: 1` (cert-manager, edge, broker, and `backends`); a template `fail`; cross-chart `no_allow_rule_is_peerless`. Runs `cidr_mode_with_an_empty_list_is_refused` in both wrapper charts | `parts/task-13.md:118-133`, `:208`, `:314`, `:327`, `:349`, `:386-388` |
| I3 restatements | P20 and P21 remedies name the values files and the script | `parts/task-32.md:41-42` |
| **I4** `alias: certManager` names | `certManager.fullnameOverride` and `nameOverride: cert-manager`, with the reason beside the alias | `parts/task-13.md:55-68`, `:81-82`, `:111-113` |
| I4 | `rendered_names_are_dns_1123` in **`netpolcover`**, not helm-unittest. Deviation, and why: helm-unittest asserts per named template file and cannot sweep a subchart's documents; `netpolcover` already renders both wrapper charts with their real subcharts | `parts/task-13.md:353`, mutant `:389` |
| **M1** no `helm dependency build` in the job | `netpolcover`'s `TestMain` builds dependencies itself | `parts/task-13.md:40`, `:332-334`; `parts/task-28.md:58` |
| **M2** no release namespace | stage 4 is a table of release → `--namespace` (`platform` in `default`, holding only its release record, never `--create-namespace` into one of its own four); `netpolcover` renders with the same table; new case `every_workload_is_in_a_denied_namespace` | `parts/task-30.md:135-153`; `parts/task-13.md:332-342`, `:347`, mutant `:390` |
| **M3** named webhook `targetPort` | the row resolves a named port through the pod's container ports; an unresolved name fails | `parts/task-13.md:351` |
| **M4** apt on 443; GitHub assets host | `azure.archive.ubuntu.com` moves to a new `ci_agent_allowed_http_fqdns` (`Http:80`, agents subnet only, exact hosts, validated); `release-assets.githubusercontent.com` added, UNVERIFIED; new run `ci_http_takes_exact_hosts_only`; the module now has seven runs | `parts/task-07.md:83-97`, `:151-154`, `:476`, `:502`; P22 `parts/task-32.md:43` |
| **M5** row rationale; one row = two runs | the ARM rationale is corrected: rule 2's AKS tag already admits ARM from the nodes, and a sandbox's guard is having no credential. `agents_egress_in_both_modes` is split into `agents_egress_in_nat_mode` and `agents_egress_in_firewall_mode` | `parts/task-07.md:475`, `:494-495` |
| restatement: platform's netpolcover sentence | now names both directions and the hook rule | `parts/task-11.md:107-111` |

**Owner question carried forward** (not decided here): allow `*.queue.core.windows.net` from the
agents subnet only (`parts/task-07.md:67-72`). The fixer and the re-reviewer both recommend
accepting it, and the plan builds that option. The wildcard is confined to `ci_agent_source_cidrs`,
and P22's deny log is the fallback if the owner refuses it.

**Still UNVERIFIED after round 2** (training knowledge, nothing rendered or run; no chart exists
in the tree yet):
- Helm's hook order and weights (I1).
- The cert-manager `fullname` helper's use of the alias, and both override keys (I4).
- Helm 3's support for schema `if`/`then`.
- That the `kubernetes` EndpointSlice holds the address Cilium sees (P20 checks it).
- The upstream webhook's named port (M3).
- Ubuntu's archive on port 80 and GitHub's asset host (M4).
- The contents of the AKS FQDN tag (M5).

## Fix batch b02 (2026-09-24) — C3, I8

Commits on `feat/env`: `502aec6`, `4b59507`, and the commit that adds this section. Paths are
relative to `docs/superpowers/plans/`, with `parts/` and `plan` as in b01. Line numbers are as of
`4b59507`. C3 and I8 are one fix, because I8's table is built on C3's plan identity. The result
is four pipeline identities per environment (plan, infra, cluster, push), each holding one row of
a single table.

### C3 — a PR build could use the apply identity

The review's fix was taken, with three additions found while applying it:
- **The estate guard refused every PR branch.** `infra-plan`'s PR validation could therefore never
  pass. The guard now has `plan` and `deploy` modes.
- **A PR build can publish an artefact named `plan-<stack>` or `image-digests`.** `deploy` would
  then apply or run the PR author's work if an approver picked that run. `deploy` now refuses a
  consumed run from a disallowed branch, and `helm-values.sh` refuses a digest from another
  registry.
- **The provider-source guard alone misses implicit providers.** A `data "external"` with no
  `required_providers` entry makes Terraform infer `hashicorp/external`. So G9 also reads the
  lock files, and `init` runs with `-lockfile=readonly`.

| Fix | Where |
|---|---|
| guards are hygiene, not a boundary, stated in task 01 and task 30 | `parts/task-01.md:83-86`; `parts/task-30.md:14-22` |
| G9, the provider-source allowlist (`hashicorp/azurerm`, `hashicorp/azuread`, `azure/azapi`), over `required_providers` **and** lock files; two cases; counts 9 of 9 and 11 cases | `parts/task-01.md:98`, `:135-136`, `:143-158`; `parts/task-02.md:195`, `parts/task-03.md:232`, `parts/task-07.md:545`, `parts/task-08.md:270`, `parts/task-09.md:182` |
| the plan connection: Reader on the subscription + Storage Blob Data Reader on state; `-lock=false`; `init -lockfile=readonly`; identity and bootstrap not in its stack list; the Reader-too-little rule (no secret-returning action is ever added); `resource_provider_registrations = "none"` | `parts/task-30.md:145-189`; `parts/task-02.md:38`, `:74-81`, `:139-141`, `:164-165`; `parts/task-07.md:170-171`; `parts/task-01.md:113-116` |
| the estate guard: `plan` accepts any branch; `deploy` enforces the branch rule, re-runs G9, and refuses a consumed `infra-plan` or `images` run from a disallowed branch; 8 cases | `parts/task-30.md:115-137`, `:250-262`, `:334` |
| `deploy`'s stages on the infra and cluster connections; outputs published so the cluster identity never reads state | `parts/task-30.md:191-232` |
| the four service connections, with branch control and per-pipeline permission (ADO-side, so no PR can edit them) | `parts/task-30.md:234-247` |
| `images.yml`'s push connection: branch control, `images` only, AcrPush only; a PR's digests never deploy | `parts/task-27.md:55-68` |
| `tf-stack_test.sh` (3 cases); `helm-values` refuses a foreign registry | `parts/task-30.md:264-270`, `:297-300` |
| lint rule 7: each pipeline names only its own connections (7 of 7) | `parts/task-28.md:99`, `:113`; `parts/task-30.md:339` |
| runbook §3: four connections, branch control, the negative check (a feature-branch `deploy` must fail at the check) | `parts/task-31.md:30` |

### I8 — one principal held everything

The review's fix was taken, except for one scope. **RBAC Admin is not granted on
`panacea-sandboxes`.** The review said "the four product namespaces", and those four include
`panacea-sandboxes`. That namespace holds every user's pod and token Secret (`parts/task-17.md:29`,
`PANACEA_APP_BROKER_NAMESPACE`, "where every pod, Secret and claim lives"). An admin grant there
would leave C3's session-read path open. So the cluster identity gets RBAC Admin on the four
**release** namespaces (`default`, `panacea-edge`, `panacea-broker`, `panacea-hub`). In
`panacea-sandboxes` it gets a custom role that never names `secrets` or `pods`.

A further change: `bootstrap` now creates `rg-panacea-app-<env>`, so that the infra identity's
Contributor can be scoped to that group rather than the subscription.

| Fix | Where |
|---|---|
| the "Deploy identities" table (plan, infra, cluster, push, and the three human roles), with the declared residuals | `plan:322-360` |
| P8-D16; P8-D10 corrected (a human creates the database roles) | `plan:145`, `plan:139` |
| owner inputs rewritten: four identities, the ABAC self-exclusion, two custom roles, four connections, identity stack owner-run, humans-only admin groups, a human Postgres admin | `plan:311-316` |
| infra: Contributor on `rg-panacea-app-<env>`, which bootstrap now creates, and foundation reads it | `parts/task-02.md:104`, `:119-124`, `:166`; `parts/task-07.md:319` (output), `:405-408` (data source) |
| cluster: Cluster User, RBAC Admin on the four release namespaces, two custom roles, **no** Cluster Admin, **no** RBAC Admin on `panacea-sandboxes`; test | `parts/task-07.md:271-278`, `:425-447`, `:524-527` |
| the custom roles checked against the rendered charts (`cluster-kinds_test.sh`); the `bind` residual printed | `parts/task-30.md:30-31`, `:272-294`; `plan:429` |
| allowlist: Cluster Admin removed; Storage Blob Data Reader, RBAC Admin and the two custom roles added (12 roles); the ABAC condition excludes the infra identity itself; "assigns" versus "holds" | `parts/task-10.md:11-20`, `:157-177`, `:209`, `:218`, `:227` |
| database bootstrap by a human Postgres admin; `deploy` has no database stage; no `psql` in tools | `parts/task-30.md:34-35`, `:226-232`; `parts/task-31.md:34-35`; `parts/task-08.md:155-157`; `parts/task-29.md:4-5`, `:165-172`; `parts/task-10.md:133` |
| identity stack owner-run only | `parts/task-09.md:5-13`; `parts/task-31.md:33`; `parts/task-30.md:145-148` |
| runbook §2a: the owner creates the identities and custom roles, and checks that no pipeline identity is in an admin group | `parts/task-31.md:28-29` |
| `task-04.md:123-125` corrected: Cost Management Contributor is a **held** role (in the table), not an allowlisted one | `parts/task-04.md:124-128` |

**I9 interplay.** Taking the database out of `deploy` resolves I9's ordering, because runbook §7
now comes before §7a, the charts. I9's other two asks are left to I9's own batch: an idempotent
`roles.sql`, and principals by object id.

**Declared residuals** (`plan:340-360`):
1. The infra identity can reach any allowlisted role indirectly, through a user-assigned identity
   it creates.
2. The cluster identity may reach cluster-admin through Kubernetes RBAC `bind`, which the upstream
   ClusterRoleBindings need. This is owner question **Q4** (`plan:301`). The recommended option is
   built: the pipeline installs those objects in dev. The seam is `infra/rbac/cluster-deployer.json`.
3. Reader may be too little for some refreshes.
4. Group-scoped Contributor may be too little for AKS's node resource group.

**Still UNVERIFIED after b02** (training knowledge; nothing was run against Azure or ADO):
- ADO branch control's setting names.
- `RESOURCES_PIPELINE_<ALIAS>_SOURCEBRANCH`.
- `terraform apply` refusing a stale plan made with `-lock=false`.
- `resource_provider_registrations = "none"`.
- Namespace-scope AKS role assignments made before the namespace exists.
- The Kubernetes RBAC `bind` requirement under the Azure RBAC webhook.
- Which POST actions azurerm calls at plan time.
- `az account get-access-token --resource-type oss-rdbms`.

**Mutation checks:** none run. This batch changes plan documents only, with no code or test to
mutate. The named mutants live in the new test rows, for the implementer.

### Fix batch b02, round 2 (2026-09-24) — the re-review's I-1 to I-4, M-1 to M-5

Commits on `feat/env`: `54eb1fa`, `610dab9`, and the commit that adds this section. The
re-review is `.superpowers/sdd/env/plan-rereview-b02-1.md` (git-ignored). Line numbers are as of
`610dab9`. **This subsection supersedes the round-1 "Declared residuals" list above**: the plan
now declares six, numbered as below.

**I-2 took the re-review's option 1, the root-cause option.** No stack grants a pipeline identity
anything. The cluster and push roles moved out of `foundation`'s `grants.tf` into an owner-run
script, `infra/rbac/grant-pipeline-roles.sh`, and the allowlist went from 12 roles to 7. Option 2
(keep the grants and declare that the infra identity can grant cluster-wide RBAC Admin) would
have left a route to every session token open with only a note on it. A script, not a new
owner-run stack, because the grants need the cluster's id, which exists only after §4, and a
stub-`az` test gives the moved assertions a home.

Two routes the re-review did not name were found while making the table true, and are declared
with a bound, not hidden:
- **The cluster resource** (residual 1). Contributor on the cluster can turn local accounts back
  on and list the admin credential. Bounded by an owner-assigned Deny policy on AKS local
  accounts (runbook §2a) and probed in §4a, with AKS run command, whose rights are UNVERIFIED.
- **Pipeline identities in the infra identity's own group.** If they were user-assigned
  identities in `rg-panacea-app-<env>`, Contributor could add a federated credential to the
  cluster identity and become it. §1a puts them outside that group.

| Finding | Fix | Where |
|---|---|---|
| I-1 | the cluster row's "Cannot" no longer claims the tokens; "It can read every user's token Secret" in the row, and residual 3 | `plan:338`, `plan:375-382` |
| I-1 | Q4 says what it does not decide (the token route through the broker) | `plan:301` |
| I-1 | P8-D16: the split removes the **administrator** routes only; an identity that deploys a workload can act as it | `plan:145` |
| I-1 | task 07: the narrowing's real value (a direct grant removed), and the broker route stated | `parts/task-07.md:453-459` |
| I-1 | `sandboxes-deployer.json` carries `escalate`/`bind` on `roles` for the broker's Role; `cluster-kinds_test.sh` prints `sandboxes RBAC verbs`, and its claim is scoped to **names**; "does not claim" bullet | `parts/task-30.md:294-312`, `:320-325`, `:398`, `:415-416` |
| I-1 | runbook §7a records whether the broker's Role installs with those verbs | `parts/task-31.md:38` |
| I-2 | `grants.tf` keeps only the kubelet's AcrPull and the control plane's Network Contributor; the cluster and push grants listed as the owner script's | `parts/task-07.md:417-462` |
| I-2 | foundation takes no pipeline object id: `push_principal_object_id`, `cluster_principal_object_id` removed; the test file's inputs; `kubelet_pulls_from_the_registry`; the moved runs named | `parts/task-07.md:267-271`, `:522-524`, `:539-545` |
| I-2 | allowlist 12 → 7; the ABAC condition permits the five the infra identity's stacks assign (not bootstrap's two Storage Blob roles); "no route through a role assignment", with the other routes pointed to residual 1; counts; a mutant for Step 4 | `parts/task-10.md:11-33`, `:169-194`, `:226`, `:235-237`, `:246` |
| I-2 | the federated-credential route named, in task 10's "does not claim" and in residual 1 | `parts/task-10.md:255-259`; `plan:345-368` |
| I-2 | `grant-pipeline-roles.sh` (owner-run, idempotent, `--dry-run`, eight assignments, `RBAC_ADMIN_NAMESPACES` as the single source `cluster-kinds_test.sh` reads) and its 5-case test, with task 07's two runs moved in | `parts/task-30.md:32`, `:327-351`, `:387-388`, `:397` |
| I-2 | the table's intro says who grants what; the infra row's "Cannot"/"It can"; the owner row; owner inputs | `plan:325-329`, `plan:337`, `plan:340`, `plan:311-313` |
| I-2 | the push identity's AcrPush is the owner script's | `parts/task-27.md:63-66` |
| I-3 | runbook §1a (identities, no role, outside `rg-panacea-app-<env>`); §2 needs §1a; §2a grants only what exists after §2 (custom roles, plan and infra roles, the Deny policy, the checks); §4a runs the owner script after `foundation`, records the namespace-scope question, and probes residual 1 | `parts/task-31.md:28-30`, `:33` |
| I-4 | the lock leaves Terraform (`delete_lock` and `azurerm_management_lock.server` removed, with the reason); an acceptance grep keeps it out; runbook §6a has the owner make it, with a check; the infra row says it cannot create or lift a lock | `parts/task-08.md:146-153`, `:274-275`; `parts/task-31.md:12-13`, `:36`; `plan:337` |
| M-1 | estate-guard's ninth case (a PR-built `images` run); 9 of 9 | `parts/task-30.md:279`, `:395` |
| M-2 | `infra/rbac/plan-reader.json` with `listClusterUserCredential` only, from day one; §6 decodes the credential; the plan row; owner inputs (three custom roles); file map | `parts/task-30.md:31`, `:171-196`; `parts/task-31.md:35`; `plan:336`, `plan:312`, `plan:469` |
| M-3 | a PR build's VNet position declared as residual 5. **Deferred**: the `private-endpoints` NSG and a separate PR pool are network changes to tasks 03 and 30's pool, outside this identity batch | `plan:386-393`; `parts/task-30.md:176-177` |
| M-4 | **G9 is taken by provider sources. I1's file-level test-variable guard becomes G10**, and I1's batch moves the counts to `guards: 10 of 10` and `guards_test: 12 cases` in tasks 01, 02, 03, 07, 08 and 09 | this line |
| M-5 | the missing backtick; `task-07.md:314` → `:319` (output) and `:405-408` (data source), checked against `4b59507` | review `:658`, `:686` |

**Restatements checked** (grep over `plan` and `parts/`): `12 roles`, `twelve`, `push_principal`,
`cluster_principal`, `delete_lock`, `two custom role`, `grants.tf`, `residual N`, `8 of 8`,
`Reader; Storage`. None is left stale. The round-1 map above keeps its `4b59507` line numbers.

**Still UNVERIFIED after round 2** (training knowledge; nothing was run against Azure or ADO), in
addition to round 1's list:
- That an ABAC condition on `roleAssignments/write` cannot constrain the assignment's scope.
- That Contributor covers `federatedIdentityCredentials/write`.
- That Contributor and RBAC Administrator both lack `Microsoft.Authorization/locks/write`.
- That azurerm's AKS read calls `listClusterUserCredential`, and that the credential holds no secret with local accounts off.
- That a built-in Azure Policy denies AKS clusters with local accounts enabled, with a Deny effect.
- Whether AKS run command, with local accounts off, runs with the caller's own rights.
- That the broker's Role needs `escalate`/`bind` under the AKS Azure RBAC webhook.

**Mutation checks:** none run, again. Plan documents only. The new mutants are in
`grant-pipeline-roles_test.sh`'s table, estate-guard's ninth case and task 10's Step 4.

## Fix batch b03 (2026-09-24) — I9, I3

Commits `50839bf` (I9) and `fb10d54` (I3). Line numbers are at `fb10d54`.

**I9. The first deploy installs the Hub before the database roles exist.**

| Ask | Change | Where |
|---|---|---|
| order the stages `guard → approve → apply → database → cluster → smoke`, or split the Hub out of stage 4 | **The `database` stage is not taken**: it needs a pipeline identity in the Postgres admin group, which the I8 fix (b02) removed. The order is held three ways: `applyStacks` and `deployCharts` default to `false`, and the runbook's first `data`/`workloads` run leaves charts off; the human bootstrap (§7) comes before the first chart run (§7a); and `hub` is the last release, so a skipped bootstrap fails the Hub alone, fail-closed, and the stage prints the `hub-migrate` Job's log. The migrate hook is the only exact check (the Hub's own DDL identity against the real server). An ADO "bootstrapped" variable was rejected: it would be a claim, not an observation | `parts/task-30.md:245-248`, `:283-288`, `:291-305`; `parts/task-31.md:35`, `:38` |
| idempotent role creation, as a Hub-contract ask in task 33 | ask **HC-1**: wrap `roles.sql`'s `CREATE ROLE`s in `DO … IF NOT EXISTS` (contract `:1421-1430`, checked in the sibling `hub` worktree). Until the Hub answers, runbook §7 counts `hub_owner`/`hub_app` first: 0 → run `roles.sql`, 2 → skip it, 1 → stop. That is correct for both the current file and the amended one. Ask **HC-2**: `hub migrate` names a missing bootstrap | `parts/task-33.md:57-58`; `parts/task-31.md:37`; `parts/task-29.md:171-177` |
| `pgaadauth_create_principal_with_oid` by object id | the template creates both principals with `_with_oid(name, oid, 'service', false, false)`. On a re-run, a role that already exists must be bound to the expected oid (`pgaadauth_list_principals`), or the block `RAISE`s. `workloads` gains `hub_principal_id` and `hub_migrate_principal_id`. `render-sql.sh` refuses an unfilled placeholder. There are two new render tests, and the Hub is told the grants are rendered, not literal | `parts/task-29.md:14`, `:171-206`, `:212-219`, `:238-240`, `:272-273`; `parts/task-10.md:146-155`; `parts/task-08.md:156`; `parts/task-33.md:59`; plan `:439-440` |

**I3. The workstation path skips the estate guard, and nothing positively identifies our subscription as not Panacea's.**

| Ask | Change | Where |
|---|---|---|
| `tf-stack.sh` calls `estate-guard.sh` itself, on every path | `tf-stack.sh` runs the guard before every subcommand, pipeline or workstation. So does `grant-pipeline-roles.sh`, the owner script that makes role assignments. The runbook runs the guard first in every workstation section, and §2 (bootstrap, which `tf-stack.sh` refuses) calls it directly. In a pipeline `tf-stack.sh` still refuses `identity`; on the owner's workstation it runs it after the guard, so the owner's identity run is guarded too. On a workstation the guard also requires `ARM_SUBSCRIPTION_ID` to be set and to equal `az account show` | `parts/task-30.md:29`, `:117-152`, `:186-189`, `:249-252`, `:397-401`; `parts/task-31.md:27`, `:29`, `:32`, `:35`; `parts/task-09.md:11-14` |
| a positive tag `panacea-app-estate=<env>`, checked by the guard, with a refusal test | The owner sets the tag in §1, and the plan's owner-inputs table lists it. The guard refuses a missing tag, the other environment's tag, and a failed read. **Deviation:** the read is the subscription's own `GET` (`az rest`), not `az tag list`. `az tag list` on a subscription needs `tags/read` at subscription scope, which the infra identity lacks; the `GET` needs only `subscriptions/read`, which Reader and Cost Management Contributor carry (UNVERIFIED, recorded per connection in §4). No grant was added. Five new guard cases: no tag (**the refusal test**), wrong tag, failed read, workstation context mismatch, `ARM_SUBSCRIPTION_ID` unset. Three new `tf-stack` cases (the guard runs first, `terraform` never reached), one new `grant-pipeline-roles` case. Counts: `14 of 14`, `6 of 6`, `6 of 6`. Four mutants are named | `parts/task-30.md:155-172`, `:322-343`, `:345-358`, `:425`, `:459-463`, `:473-475`; `parts/task-31.md:27`, `:32`; plan `:310` |
| `validate.sh` sets an empty `AZURE_CONFIG_DIR` | exported as a fresh `mktemp -d`, removed by an `EXIT` trap; Step 4 proves it red-first with a stub `terraform` that logs the directory and `ARM_SUBSCRIPTION_ID` | `parts/task-01.md:75-78`, `:154-160` |

**Restatements checked** (grep over the plan and `parts/`): `pgaadauth_create_principal`,
`hub_identity_name`, `render-sql`, `four statements`, `deployCharts`, `No database stage`,
`stage 4`/`stages 4–5`, `tf-stack.sh refuses`, `estate-guard_test: 9`, `3 of 3`, `5 of 5`,
`task 31 §3` (the per-connection tag read moved to §4). None is left stale. The release table and
its stage numbers are unchanged, so task 13's copy of it stands. Earlier batches' maps keep
their own line numbers.

**Deferred, with reason:** a Terraform-level fingerprint (a `data "azurerm_subscription"`
postcondition on the tag in every stack) would also catch raw `terraform` typed by hand. It is not
in I3's fix, and it touches every stack's part and its mocks. The guard covers every scripted path
and every runbook section. Recorded for the owner as an optional hardening, not built.

**Still UNVERIFIED** (training knowledge; nothing was run against Azure, ADO or Postgres):
- `pgaadauth_create_principal_with_oid`'s signature and `'service'` object type; `pgaadauth_list_principals`' `rolname`/`objectid` columns.
- That a subscription `GET` returns `tags`, and that Reader and Cost Management Contributor include `Microsoft.Resources/subscriptions/read`.
- That ADO sets `TF_BUILD=True` on every job.
- That `az tag update --operation Merge` is the right command for the owner's tag.
- That the cluster identity's namespace role may read the `hub-migrate` Job's logs.

**Mutation checks:** none run. These are plan documents only. The new mutants are in task 30's
Step 2 (four, for I3), task 29's two new `sql /` rows, and task 01's Step 4.

## Fix batch b03, round 2 (2026-09-24) — re-review `plan-rereview-b03-1.md`

Commits `694b3a4` (I-1, M-4, M-5) and `a92b81b` (I-2, M-1, M-2, M-3, M-6, M-7). Line numbers are
at `a92b81b`. The I9 and I3 rows above stand; these close the re-review's open issues in them.

| Re-review | Change | Where |
|---|---|---|
| **I-1** psql's default runs the GRANTs after the oid check fails | Both grants move **inside** the DO block, after both checks, as `EXECUTE format('GRANT … TO %I', …)`; a `RAISE` aborts the block, so no grant runs whatever psql's settings. Nothing but a comment follows `$$;`. §7 runs every file as `psql -v ON_ERROR_STOP=1 --single-transaction -f …`, with the exit code written to a file and read. New row `sql / no grant runs outside the checked block`. HC-1 asks the Hub to state the same flags in `roles.sql`'s header. The "or the §7 command lacks `ON_ERROR_STOP`" test is not built: it would test runbook prose; the grant-inside-the-block structure is the control, and it holds without the flag | `parts/task-29.md:205-215`, `:262`; `parts/task-31.md:37`; `parts/task-33.md:57` |
| **I-2** the pipeline test rests on `TF_BUILD` alone and fails open | Root-cause option taken: a pipeline is `TF_BUILD=True` **or** any of `BUILD_SOURCEBRANCH`, `SYSTEM_JOBID`, `AGENT_ID`; a workstation only when all four are unset; an empty branch on the pipeline path is refused. One `is_pipeline` in `pipelines/scripts/lib/where.sh`, sourced by both `estate-guard.sh` and `tf-stack.sh`, so they cannot disagree. New guard cases: `TF_BUILD` unset with a branch, consuming a `refs/pull/*` run → refused; only `SYSTEM_JOBID` set → refused naming the empty branch. New `tf-stack` case: `AGENT_ID` alone, `plan identity` → refused. Workstation cases unset all four. §4 records that every pipeline `ok` line names a branch, and stops on `workstation` | `parts/task-30.md:29`, `:121-127`, `:195-196`, `:350-353`, `:364-365`, `:377-378`, `:388`, `:496-501`; `parts/task-31.md:32`; plan `:441` |
| **M-1** no test proves a correct workstation passes; no stack on a workstation | No stack named on a workstation → the tfvars rule checks nothing, and an unset list is not an error under `set -u`. New case: workstation, ids agree, tagged `dev`, no stack → `ok` naming `workstation` | `parts/task-30.md:148-152`, `:363` |
| **M-2** workstation `apply` untested and its plan source unstated | A workstation `apply` applies the `<stack>.tfplan` its own `plan` just wrote, never re-plans, never reads a pipeline artefact; no file → exit 1. Two new `tf-stack` cases: the guard stub exiting 1 on workstation `apply` → `terraform` recorded nothing; the plan-file source. Mutant named | `parts/task-30.md:213-218`, `:386-387`, `:496-501` |
| **M-3** "every path" overclaims | Stated as a residual: stages 4–5 and `images.yml`'s push job never run the guard; their identities hold nothing at subscription scope; they rely on stage 1's read, branch control and resource-scoped roles on our fingerprinted cluster and registry. "Every path" now reads "every Terraform path and every workstation section". The commit title `fb10d54` cannot be changed (no amend); this row corrects it | `parts/task-30.md:132`, `:267-276`; `parts/task-31.md:27`; plan `:310` |
| **M-4** at a count of 2 the §7 gate skips `GRANT CREATE ON DATABASE hub` | At 2, §7 re-runs that `GRANT` alone (safe to repeat), then checks `has_database_privilege('hub_owner','hub','CREATE')`. At 0, `roles.sql` runs in one transaction, so a failed `GRANT` leaves 0 roles, never 2 without it. HC-1's "correct for both files" is now true | `parts/task-31.md:37`; `parts/task-33.md:57`; `parts/task-29.md:176-178` |
| **M-5** no mapping from render-sql's keys to Terraform outputs; fixture path unclear | `render-sql.sh <template> <workloads.json>` reads the `stack-outputs` artefact's `workloads.json` by output name through one written table (`HUB_APP_IDENTITY`←`hub_identity_name`, `HUB_APP_OBJECT_ID`←`hub_principal_id`, `HUB_MIGRATE_IDENTITY`←`hub_migrate_identity_name`, `HUB_MIGRATE_OBJECT_ID`←`hub_migrate_principal_id`, checked against task 10 `:145-155`), with `python3`'s `json` module (no `jq`). Each value must match a name pattern or a GUID, so no quote reaches the SQL. `stack-outputs` holds one `<stack>.json` per stack. The fixture is `pipelines/scripts/tests/fixtures/workloads-outputs.json` in both places. Two new rows; `render-sql_test: 6 of 6` | `parts/task-29.md:14`, `:227-241`, `:263-265`, `:297-298`; `parts/task-30.md:282-283`; `parts/task-31.md:37` |
| **M-6** dev and prod must be separate subscriptions | The owner-inputs row now asks for one subscription per environment, and says why; §1 repeats it | plan `:309`; `parts/task-31.md:27` |
| **M-7** `pg_has_role` raises on a missing role | HC-2 now runs two queries in order: the role's existence, then `pg_has_role`; not one `AND`, whose evaluation order Postgres does not promise. A missing role gives the one-line bootstrap message | `parts/task-33.md:58` |

**Restatements checked** (grep over the plan and `parts/`): `ids.env`, `KEY=value`,
`only when neither`, `stops on a role`, `two grants`, `14 of 14`, `tf-stack_test: 6`, `3 of 3`,
`on every path`, `every path to Azure`, `TF_BUILD`, `render-sql`, `pg_has_role`. The stale ones
are fixed; `task-29.md:259`'s "the two grants" is still true. `grant-pipeline-roles_test: 6 of 6`
is a different script and is unchanged.

**Still UNVERIFIED** (training knowledge; nothing was run):
- psql's exit code under `ON_ERROR_STOP` (3 for a script error), and that it exits 0 without it; §7 records the real code.
- That ADO sets `TF_BUILD`, `BUILD_SOURCEBRANCH`, `SYSTEM_JOBID` and `AGENT_ID` on every job, deployment jobs included. The guard no longer fails open if one is missing; §4 records which were set.
- That `pg_has_role` raises on a missing role (the reason HC-2 splits its query), and that `GRANT` of an already-held role is a notice, not an error.
- That `python3` is on the operator's workstation; §7 lists it as a prerequisite.

**Mutation checks:** none run. Plan documents only. The new mutants are the five in task 30's
Step 2 (`:496-501`) and the three new `sql /` rows of task 29.

## Fix batch b04 (2026-09-24) — I1, I2, I10

Commit `c71ccbb`. Line numbers are at `c71ccbb`.

**I1. "Every committed tfvars is proven to plan" is false: a test file's variables override `-var-file`.**

| Ask | Change | Where |
|---|---|---|
| guard G9: a stack's `.tftest.hcl` file-level `variables` may hold no key any `<env>.tfvars` sets | **Renumbered G10.** G9 was already taken by the provider-source allowlist (b02, C3). G10 reads the column-0 `variables { … }` block (fmt puts a run's block at column 2) against every tfvars' top-level keys. Guards count 9 → 10 | `parts/task-01.md:23`, `:93-96`, `:114`; counts restated in `parts/task-02.md:216`, `parts/task-03.md:232`, `parts/task-07.md:593`, `parts/task-08.md:275`, `parts/task-09.md:188` |
| every run that depends on an environment-specific value sets it at run level, `environment` included | Written into the `validate.sh` contract and the README conventions. **Addition:** each stack test has a `committed_tfvars_plan` run with no run-level tfvars key. That run is what actually plans each tfvars untouched, because run-level values override `-var-file` too | `parts/task-01.md:64-74`, `:130-133` |
| fix task 02's test file | the file-level block holds only `state_writer_object_ids` and `state_reader_object_ids`. There is a new `committed_tfvars_plan` run. `the_product_group_exists_before_foundation` asserts `rg-panacea-app-${var.environment}` rather than `"dev"`. The count is `12 passed`, printed twice (once per tfvars) | `parts/task-02.md:147-176`, `:180`, `:187`, `:219-220` |
| task 07's dev-shaped runs | foundation's file-level block holds only `cluster_admin_group_object_ids`. `nat_mode_allowlists_its_own_address_on_the_api` pins `environment`, `api_private` and `egress_mode`. **Found beyond the review:** `overlapping_pod_range_fails_the_plan` depended on dev's `vnet_cidr`, because prod's differs and would not overlap, so it now pins `vnet_cidr` and the whole `aks_network`. `dns_ip_outside_service_range_fails` pins the whole `aks_network` too. `tfvars_plan_against_mocks` is renamed `committed_tfvars_plan` | `parts/task-07.md:550-563`; `parts/task-08.md:253-254` |
| a `guards_test.sh` case for G9 | `g10_tftest_file_level_tfvars_key_fails`: the key is set only in `prod.tfvars`, which kills a G10 that reads only `dev.tfvars`. The same key at run level passes. Cases 11 → 13, counting I2's case | `parts/task-01.md:158`, `:169-171`, `:187-190` |
| (consistency) | `stacks/identity` and `stacks/workloads` gain the same `committed_tfvars_plan` run and the file-level rule. The identity run count goes 12 → 13 | `parts/task-09.md:156-161`, `:181`; `parts/task-10.md:220-226` |

**Verified this batch.** The probe was provider-free Terraform 1.15.8 in the scratchpad `env/b04-probe`, with no provider and no Azure, run as `terraform test -var-file=prod.tfvars`:
- A run with no pin and no file-level key saw `"prod"`.
- A run-level `environment = "dev"` saw `"dev"`.
- With a file-level `environment = "dev"` added, the unpinned run saw `"dev"` and failed (`Failure! 1 passed, 1 failed`).

**Correction recorded:** a `committed_tfvars_plan` whose assertions read `var.environment` agrees with whichever value won, so it **cannot** kill the file-level mutant. G10 is the only kill, and the rows say so (`parts/task-02.md:180`, `parts/task-07.md:558`).

**I2. The runbook's first command cannot run.**

| Ask | Change | Where |
|---|---|---|
| §2 uses an uncommitted `backend_override.tf` (local), then `init -migrate-state` with the real `-backend-config` flags | Steps (a)–(f): (a) write the override and check it with `git check-ignore -v`; (b) init/plan/apply to a plan file; (c) delete the override; (d) `init -migrate-state` with `backends/<env>.hcl` plus the resource group, account and `bootstrap.<env>.tfstate` key; (e) a no-change `plan` against the migrated state, then delete the local state; (f) guards. **Addition:** step (d) needs the operator in the break-glass writer group, and a 403 straight after apply is role-assignment propagation (UNVERIFIED) | `parts/task-31.md:29`; `parts/task-02.md:29-35` (backend.tf comment) |
| `.gitignore` gains `*_override.tf` | plus `*_override.tf.json` | `parts/task-01.md:26` |
| G1 refuses a tracked override | G1 checks `git ls-files` for `*_override.tf` and `*_override.tf.json`. New case `g1_tracked_override_fails`: tracked → FAIL, and untracked → ok, because §2 needs the override on disk | `parts/task-01.md:105`, `:148` |

**I10. The firewall allowlist is described as an egress control it cannot be.**

| Ask | Change | Where |
|---|---|---|
| `module.registry.login_server` exactly, not `*.azurecr.io` | the module takes `acr_login_server`, validated to have no leading `*` and to end in `.azurecr.io`. `acr_data_endpoints` is validated to have no leading `*`. Rule 2 lists exactly those names. Foundation passes `module.registry.login_server` and `data_endpoint_host_names` (task 05 `:72-75`) | `parts/task-07.md:102-117`, `:160-162`, `:429-430` |
| a test that no FQDN in the ACR rule starts with `*` | `acr_rule_names_our_registry_exactly` (the exact name is present, and `alltrue(!startswith(f, "*"))`). A second, separate run, `wildcard_acr_login_server_is_rejected`, is added. The module's runs go 7 → 9 | `parts/task-07.md:544-545`, `:579` |
| Q1 and task 07 state that Overlay SNAT hides sources and that per-sandbox FQDN control needs Cilium FQDN policy (UNVERIFIED) | Q1: the firewall limits where agents go and does not stop exfiltration. Node rules are open to sandboxes, and `github.com` / `*.githubusercontent.com` accept uploads. Task 07 gets a paragraph after the rule list, a `source_cidrs` description, and a "does not claim" bullet | plan `:298`; `parts/task-07.md:100`, `:170-178`, `:609-612` |

**Still UNVERIFIED after this batch:**
- `init -migrate-state` from `local` to `azurerm` with `use_azuread_auth` (not probed, because it needs Azure).
- The RBAC propagation delay.
- That Advanced Container Networking's FQDN policy enforces per pod on the Kata pool.

**Mutation checks:** none run beyond the precedence probe. These are plan documents only. The new mutants are those in the three new guard/test rows (`parts/task-01.md:148`, `:158`; `parts/task-07.md:544-545`).

## Fix batch b04, round 2 (2026-09-24) — re-review b04 round 1, findings 1–7

Commits `0cc9e8b` (findings 1, 2, 6) and `debac7d` (findings 3, 4, 5, 7). Line numbers are at `debac7d`.
The re-review is `.superpowers/sdd/env/plan-rereview-b04-1.md` (local, git-ignored).

| # | Finding | Change | Where |
|---|---|---|---|
| 1 | (I2) step (d)'s `-input=false` refuses the migration, exit 1 | **`-force-copy` chosen**, not a hand-answered prompt, so the step runs the same on a workstation or a pipeline agent. Step (d) now reads `init -migrate-state -force-copy -input=false …` and quotes the refusal it prevents. The local→azurerm case stays UNVERIFIED, with an instruction to record its output. `backend.tf`'s comment says the same | `parts/task-31.md:29` (d); `parts/task-02.md:29-33` |
| 2 | (I1) G10's `^  <key> =` misses the `=` alignment `fmt` writes | G10 matches `^  <key> *=` and says why the `*` matters. The `g10_…` fixture is a two-key block of different lengths (`state_writer_object_ids`, then `api_private` with 13 spaces before `=`), committed as `fmt` leaves it. It names "a single-space key pattern" as a mutant it kills, and asserts that only `api_private` is named | `parts/task-01.md:114`, `:158` |
| 3 | (I1) three firewall-mode runs fail I4 | **I4 is retired**, and its id is not reused, so I5 keeps its name everywhere. Since C2, `authorized_ip_ranges` adds the egress /32 in both modes (`parts/task-07.md:433-434`), so a public cluster's list is never empty and I4 was dead weight that refused valid plans. A comment in `terraform_data.invariants` records this. `firewall_mode_uses_udr_and_has_no_nat` pins `environment` and `api_private`. `firewall_mode_allowlists_its_own_address_on_the_api` now names "I4 restored" as a mutant it kills. Neither tfvars sets `api_authorized_ip_ranges` (`:493-529`), so no run needs that pin. Making admin ranges mandatory would be a new invariant and is not in scope | `parts/task-07.md:402-406`, `:593`, `:596` |
| 4 | (I10) the `acr_login_server` validation fails the firewall-mode apply runs under a bare mock | foundation's `mock_provider "azurerm"` gains `mock_resource "azurerm_container_registry"` with `login_server` and `data_endpoint_host_names` defaults (task 05 `:79`'s dev name). A paragraph says why: the validation is deferred to apply for a computed value, and it names the three runs. The fallback is an `override_resource` on `module.registry.azurerm_container_registry.this` (task 05 `:72`, `:79`) | `parts/task-07.md:557-583` |
| 5 | (Minor) stale `12 runs × 2 tfvars` | now `14 runs × 2 tfvars`, because of finding 7 | `parts/task-09.md:188` |
| 6 | (Minor) G1 and `.gitignore` miss the bare `override.tf` / `override.tf.json` | both lists name all four override forms. `g1_tracked_override_fails` adds a tracked bare `stacks/y/override.tf` and names "G1 matching only `*_override.tf`" as a mutant | `parts/task-01.md:26`, `:105`, `:148` |
| 7 | (Minor) one row, two environments | **two pinned runs** chosen, not a `var.environment` assertion, so each tfvars pass checks both names: `names_carry_the_environment_outside_prod` (pins dev) and the new `prod_names_carry_no_suffix` (pins prod). Identity goes 13 → 14 runs. This supersedes round 1's "12 → 13" (`:872` above) | `parts/task-09.md:171-172`, `:182`, `:188` |

**Verified this round.** Offline probes on Terraform 1.15.8 in scratchpad `env/b04r2-probe` (`out.txt`), with no provider and no Azure:
- `fmt` rewrote `api_private = false` under `state_writer_object_ids = ["x"]` to `  api_private             = false`. Against the block, `^  api_private *=` matched (exit 0) and `^  api_private =` did not (exit 1).
- With `backend.tf` holding `backend "azurerm" {}`, a bare `override.tf` holding `backend "local" {}` let `terraform init -input=false` succeed (exit 0). Without the file, init failed with `One of access_key, sas_token, use_azuread_auth and resource_group_name must be specified` (exit 1).
- I did not re-run the `-force-copy` probe; the re-reviewer's (`out.*.txt` in `env/b04rr-probe`) stands.

**Still UNVERIFIED:**
- That `mock_resource` defaults reach `module.registry`'s registry, and what a bare azurerm mock generates for `login_server`. Both need the azurerm provider, which was not downloaded. The fallback is written into `parts/task-07.md:581-583`.
- `init -migrate-state -force-copy` from local to azurerm with `use_azuread_auth`.
- The round-1 items stand: the RBAC propagation delay, and ACNS per-pod FQDN enforcement on Kata.

**Mutation checks:** both probes above are the red/green halves of two named mutants: the single-space G10 pattern, and override matching that sees only `*_override.tf`. Before running them I named those two as the likeliest survivors. Both would have survived round 1's fixtures, and both are now killed by the case rows at `parts/task-01.md:148` and `:158`. No `guards.sh` exists yet to mutate; these are plan documents.

## Fix batch b05 (2026-09-24) — I4, I5

Commits `08cd790` (I4), `adf4169` (I5, broker side), `2383ef9` (I5, laptop side and the
restatements), `6c8aab7` (the two likeliest survivors). Line numbers are at `6c8aab7`.

**I4. The pre-pull DaemonSet asks for a whole node and can exhaust the namespace quota.**

| Ask | Change | Where |
|---|---|---|
| `requests == limits` (`10m`/`16Mi`) on both init containers | done, with a paragraph on why (the scheduler sizes a pod by its largest init container, and a `max`-only LimitRange defaults an unsized one to 8 CPU / 32 GiB). New helm-unittest case `prepull_init_containers_are_tiny`. Counts: 7 suites and 21 tests | `parts/task-12.md:73-90`, `:109`, `:122-123` |
| a LimitRange `default`/`defaultRequest` at session size | `sandboxes.limitRange.defaultCpu: 2000m`, `defaultMemory: 4096Mi`. `quota.yaml` renders `max`, `default` and `defaultRequest` in one `Container` item. The default is a backstop, not a sizing policy. New case `quota_test / limitrange_defaults_to_one_session`. Counts: 6 suites and 12 tests | `parts/task-11.md:60-67`, `:105-115`, `:168`, `:188-189` |
| a helm-unittest case: every container and init container in `platform` has `requests == limits` | **Moved into `netpolcover`, as `every_sandbox_container_is_sized`.** This is a deviation, for two reasons. First, helm-unittest has no assertion that compares two paths, so it cannot state "requests equal limits" for every container; it can only pin values, and task 12's rows already do that. Second, `netpolcover` already renders `platform` with real values, as the b01 round-2 precedent did for `rendered_names_are_dns_1123`. Scope: every pod template rendered into `panacea-sandboxes` by any release, including CronJob `jobTemplate`s and bare Pods. The comparison is by string, so the module keeps its one dependency. Two mutants | `parts/task-13.md:354`, `:393-396` |
| (restatement) "one value" feeds the broker, the balloons and now the LimitRange | **Found beyond the review:** the "task 29 passes both from one value" claim had nothing enforcing it, because each chart reads its own `deploy/values/<env>/<chart>.yaml` (task 30). Task 29 therefore adds a `netpolcover` case, `session_size_is_one_value`, that compares the two files for each environment, with a mutant. Task 12's comment named the wrong variable (`PANACEA_APP_SESSION_CPU_MILLIS`, not `…_BROKER_SESSION_…`) and is corrected | `parts/task-29.md:37`, `:282-290`; `parts/task-12.md:41-43`; `parts/task-11.md:63-65`; plan `:485` |

**I5. A cloud machine is capped at one node's capacity.**

| Ask | Change | Where |
|---|---|---|
| state the ceiling in P8-D6 | P8-D6 now says that every session of a machine shares its machine pod's node with whatever else runs there, and that neither a balloon nor the autoscaler adds room. On `Standard_D8s_v5` that is **about three default sessions per machine** (arithmetic, UNVERIFIED until P07). It also says the per-user cap of 8 over 2 machines cannot be reached on one machine, and how the limit is handled | plan `:135` |
| state the ceiling in the cost model | the cost is counted per node that can hold a machine's sessions, not per session. There is one row per candidate SKU, plus one row for a smaller session, and the warm floor is costed at the SKU that P07 selects | `parts/task-33.md:48-56` |
| a broker check **before** create: sum the machine's pods plus the RuntimeClass overhead, against the node's allocatable | **Replaced by the scheduler's verdict**, because the ask is wrong on two facts. (1) The node is shared: other users' machines and sessions, and the DaemonSets, run beside the machine pod, so summing the machine's own pods under-counts. (2) Reading a node's allocatable needs a permission on `nodes`, which task 29 grants to no one and tests as absent (least privilege). The DaemonSets live in namespaces the broker cannot list either. The scheduler already counts all of this, Kata `pod.spec.overhead` included. So `watchUnschedulable` cancels the create when `PodScheduled=False/Unschedulable` holds, with **no** `nominatedNodeName`, continuously for `UNSCHEDULABLE_GRACE` (5 s). A nominated node means a balloon is being preempted, which is the warm path, so the create keeps waiting | `parts/task-18.md:89-124`, `:161-176` |
| `409 machine_full` with `details.sessionsPerMachine` | `brokerapi.CodeMachineFull`. The count is the machine's other non-terminating session pods at the moment of refusal. The route table lists it | `parts/task-17.md:113`; `parts/task-18.md:78`, `:114-115` |
| (config) | `PANACEA_APP_BROKER_UNSCHEDULABLE_GRACE`, default `5s`, must be positive and below `READY_TIMEOUT`. Test `TestUnschedulableGraceMustBeBelowTheReadyTimeout`. The broker chart gains a `timeouts` block. **Found beyond the review:** `READY_TIMEOUT` and `SHUTDOWN_GRACE` were not in the chart's values, although the chart claims to set "every" variable; the block holds all three | `parts/task-17.md:51`, `:57-59`, `:86`, `:163-165`; `parts/task-29.md:49-52` |
| on a create timeout the broker deletes the Pending pod | the broker deletes the pod after `ErrNotRunning`, after the unschedulable cancel, and after the caller leaves. It never deletes after `ErrAlreadyExists`, whose pod is the owner's running session. katabe's keep-the-pod rule is unchanged, because it is right for the laptop | `parts/task-18.md:117-121` |
| broker tests | `TestAFullMachineIs409MachineFullInSeconds`, `TestABriefUnschedulableIsNotFull`, `TestPreemptionUnderWayIsNotFull`, `TestAReadyTimeoutDeletesThePod`, `TestAConflictNeverDeletesTheExistingPod` and `TestTheCallerLeavingDeletesThePod`, plus a mutation step with four mutants | `parts/task-18.md:193-198`, `:224-229` |
| map it in remotebe | `backend.ErrMachineFull` and `*MachineFullError{SessionsPerMachine}` go in `ready.go`. There is a mapping-table row and `TestMachineFullCarriesTheCount`, which asserts the error is **not** `ErrNotRunning` or `ErrAlreadyExists` | `parts/task-22.md:24`, `:44-56`, `:153`, `:169`; plan `:446` |
| (control plane) | `httpx.CodeMachineFull`. `ErrMachineFull` becomes `409 machine_full` with the count and one server-written sentence. Test `TestAFullMachineIs409MachineFullWithASentence` | `parts/task-24.md:24`, `:119-126`, `:150`; plan `:459` |
| a cockpit sentence | the sentence is rendered verbatim through `useCreateSession`'s existing `server-said` arm (`useCreateSession.ts:37-46`, verified). It needs no new `ProblemKind`, and no `types.ts` or UI-contract change, because `toProblem` passes unknown codes through (`client.ts:199-207`, verified). New test `useCreateSession / a full machine shows the server's sentence verbatim`, in a new `useCreateSession.test.ts` | `parts/task-26.md:24-25`, `:103-110`, `:137` |
| P07 (task 32) decides the SKU (D16s/D32s_v5) or a smaller default session | P07 measures sessions per machine **through the broker** (the count, the time to refusal, and that no pod is left), and the balloon-on-the-machine's-node path. The target is `MAX_SANDBOXES_PER_USER / MAX_MACHINES_PER_USER` = 4. P07 picks the smallest option that meets it, and the owner chooses on cost (see owner question) | `parts/task-32.md:28` |
| record that balloons warm new machines, not a busy one | done, in task 12's framing | `parts/task-12.md:14-21` |
| (restatements) | task 15's affinity bullet states the consequence. task 07's dev tfvars marks `Standard_D8s_v5` as provisional. The plan's vCPU-quota prerequisite no longer assumes 8 vCPUs per node, and its "does not claim" list names sessions per machine | `parts/task-15.md:100-103`; `parts/task-07.md:511-513`; plan `:311`, `:635-636` |

**Owner question (recorded, recommended option built behind a seam).** If P07 confirms about three sessions per machine on `Standard_D8s_v5`, which should it be: move the sandbox pool to `Standard_D16s_v5`, which holds about seven, at roughly twice the per-node price and a doubled warm floor; or shrink the default session below 2 CPU / 4 GiB? **Recommended: `Standard_D16s_v5`.** The product is parallel agents per machine, and a smaller session slows every agent. The seam is `sandbox_pool.vm_size` (task 07). *Corrected in round 2 (re-review 4):* the other branch's setting is the broker's `sandbox.sessionCpuMillis` / `sessionMemoryMib`, which now decides every cloud session's size because remotebe sends none (`parts/task-22.md:136`); the balloons and the LimitRange default follow it, and `session_size_is_one_value` compares the rendered objects. Before round 2 the laptop's `sessionResources()` (`sessions.go:332-337`) set the size, so this "one setting" changed no real session.

**Verified this batch** (against the tree at `6c8aab7`):
- `kata_test.go:379`: `TestCreateTimesOutWithErrNotRunningAndLeavesThePod`. katabe keeps the pod on purpose (`kata.go:108-110`).
- `PROGRAM.md:133` is the Kata-constraints line.
- `useCreateSession.ts:37-46` is the `sentenceFor` switch.
- `client.ts:199-207` passes an unknown code through intact.
- task 29 `:64-75`: the broker's `Role` is namespaced, and `nodes` is tested as absent.

**Still UNVERIFIED:**
- LimitRanger's defaulting of init containers under a `max`-only LimitRange. This is training knowledge, but explicit sizes make it moot.
- That kube-scheduler writes `nominatedNodeName` no later than it reports `Unschedulable` for a preempting pod. The 5 s grace absorbs a lag. P06 (d) and P07 exercise it.
- The per-pod Kata overhead and D8s_v5 allocatable, and so the number three. This is P07's job.
- That the fake clientset's watch delivers status updates as the new broker tests assume.

**Mutation checks:** no code exists yet, so none were run. Before writing the tests, I named the two mutants most likely to survive:
- (a) a watcher that fires on the first `Unschedulable` event and ignores the grace;
- (b) a workload kind that `netpolcover`'s pod-template walk does not visit, such as a CronJob.

Both would have survived the first draft of this batch. (a) is now killed by `TestABriefUnschedulableIsNotFull` (`parts/task-18.md:194`, mutant `:226-227`). (b) is now covered by the walk's scope (`parts/task-13.md:354`). The other named mutants are listed at `parts/task-13.md:393-396`, `parts/task-18.md:224-229` and `parts/task-29.md:289-290`.

## Fix batch b05, round 2 (re-review `plan-rereview-b05-1.md`)

Tree after `7ddf82e` plus this section's commit. Every finding is fixed; none is deferred.

| Re-review finding | Change | Where |
|---|---|---|
| 1 (Important): cleanup keyed on the error class misses a cancel katabe hides | The pod is found **by identity, not by the error's class**. After every `Create` error except `ErrAlreadyExists`, `deleteIfOurs` gets `sandbox-<id>` and deletes it only if its `backend.EnvToken` env equals this request's attach token (constant-time), with a UID precondition. This goes further than the re-review's "every error except `ErrAlreadyExists` and `ErrUnsupported`" rule, which is wrong on one fact: a create of an id the owner already runs, refused at admission (quota full, LimitRange), is a 403 that katabe also wraps as `ErrUnavailable`. That rule would destroy the owner's running session. The answer comes from `fired`, then the request context, then `errors.Is`. New tests: `TestAFullMachineIs409EvenWhenKatabeHidesTheCancel` (a reactor holds katabe's first poll and returns an unwrapped cancel error), `TestAnAdmissionRefusalNeverDeletesTheRunningSession`; `TestAConflictNeverDeletesTheExistingPod` gains a same-token case. Mutants: the old rule, and a delete by name without the token | `parts/task-18.md:129-158`, `:210-218`, `:237`, `:241-243`, `:276-279` |
| 2 (Important): `Unschedulable` read as full for a session that fits on no node | The broker's `MAX_CPU_MILLIS` / `MAX_MEMORY_MIB` and the LimitRange `max` are **derived from the sandbox SKU**: `6000m` / `20480Mi` on `Standard_D8s_v5` (arithmetic, UNVERIFIED), the largest session that fits beside a machine pod on an empty node. The config check refuses a `SESSION_*` above its `MAX_*`. `session_size_is_one_value` holds the broker's max equal to the LimitRange `max`. P07 measures the largest session and rewrites all three places. `TestCreateRefusesOversizedResources` tests the boundary (6001 / 6000, 20481 / 20480). Cordon case: the sentence has a no-count form that suggests nothing, and P12's "if true" column records it | `parts/task-17.md:42-43`, `:57-58`; `parts/task-11.md:61-67`, `:116-119`, `:175`; `parts/task-29.md:40-41`, `:292-294`, `:302`; `parts/task-18.md:86-88`, `:232`; `parts/task-32.md:28`, `:33`; `parts/task-12.md:89-90`; plan `:135` |
| 3 (Important): the 5 s refusal fires with the affinity off | `watchUnschedulable` is armed only when `Cfg.MachineAffinity` is true. Without it, the create keeps the `READY_TIMEOUT` path, which deletes its pod. New test `TestWithoutAffinityUnschedulableWaitsForTheTimeout`, plus the mutant "armed whatever the affinity". The condition is stated in P8-D6, in task 17's grace row and in P03's fallback row. P03's row also says to measure a scale-up against `READY_TIMEOUT` | `parts/task-18.md:92`, `:117-123`, `:244`, `:280-281`; `parts/task-17.md:51`; `parts/task-32.md:24`; plan `:135` |
| 4 (Important): the laptop always sends a size, so the broker's default is never used | Root cause: **the cloud size is server policy**. remotebe never sends a size (the re-review's "better still" option), so every cloud session takes the broker's `SESSION_*`. New tests: `TestCreateLeavesTheSizeToTheBroker` (task 22) and `TestZeroResourcesTakeTheBrokersDefault` (task 18; a test config of `1500`/`3072` shows the size comes from `Cfg`, not `backend.DefaultResources`). P07's "smaller default session" branch names the broker's `sandbox.sessionCpuMillis`, and so does task 07's tfvars comment. The b05 owner question's seam sentence is corrected in place | `parts/task-22.md:136`, `:165`; `parts/task-18.md:84-88`, `:233`, `:285`; `parts/task-17.md:40`; `parts/task-32.md:28`; `parts/task-07.md:511-514`; this file `:963` |
| 5 (Minor): task 12 still says "so the next session also finds room" | Now "so the next machine's first session also finds room" | `parts/task-12.md:12-13` |
| 6 (Minor): the sentence is false on a shared node and misprints 1 | Three forms, chosen by the count: 2+, 1 (singular: "this machine's one session", "Stop it"), and 0 or absent (says nothing of stopping a session). None claims this machine's sessions fill the node. All three are pinned in `TestAFullMachineIs409MachineFullWithASentence` and in task 26's verbatim test | `parts/task-24.md:122-131`, `:158`; `parts/task-26.md:103-108`, `:138` |
| 7 (Minor): the `sessionsPerMachine` fixture cannot tell per-machine from per-owner | The fixture adds this owner's pod on `m-2`, another owner's pod on `m-3`, the `m-1` machine pod and a terminating pod on `m-1`; the count must stay `"3"`. Machine ids are namespace-unique (task 19), so "another owner's pod on `m-1`" cannot exist. Three mutants: no `machine-id` selector, no `component` selector, no terminating filter | `parts/task-18.md:237`, `:281-283` |
| 8 (Minor): the walk's CronJob / bare-Pod reach is claimed, never tested | New negative fixture `testdata/unsized-kinds.yaml`: an unsized CronJob, Job and bare Pod in `panacea-sandboxes`, each of which must be reported by kind and name. Mutants: the walk skips `CronJob`, then the walk skips bare `Pod`s | `parts/task-13.md:38`, `:354`, `:397-399` |
| 9 (Minor): `session_size_is_one_value` with an absent key | The case compares **rendered objects**: each chart rendered with its env file, as task 30 deploys it, so a key left to the chart default is compared at its effective value. A missing object or field fails the case. Quantities are compared parsed. New mutant: delete `warm.balloons.cpu` from the env file and change the chart default | `parts/task-29.md:282-304` |
| Outside the count: a preempted balloon holds its slot 30 s | Fixed, since the warm path this batch waits on depends on it: `terminationGracePeriodSeconds: 0` on the balloon, asserted in `balloon_is_lowest_priority_and_evictable`. The suite and test counts are unchanged | `parts/task-12.md:63-67`, `:108` |
| Outside the count: a cloud `backend_unavailable` reads "Docker is not running" | **Deferred.** The reason: it is the cockpit's wording for a different code (`useCreateSession.ts:41-42`), not I4 or I5, and it belongs with task 26's cloud sentences in a cockpit batch. It is recorded here and in the ledger | — |

**Named survivors this round, and how each is closed** (no code exists, so no mutant was run):
- (a) `deleteIfOurs` without its UID precondition. The fake clientset may ignore preconditions, so the only thing that catches this is the full-machine test's `delete pods` reactor, which records `DeleteOptions.Preconditions.UID`. See `parts/task-18.md:237` and `:286-288`.
- (b) `machineSessions` ignoring `except`. A fake delete removes the pod at once, so this mutant is caught only because the count now runs **before** the delete; the count then becomes `"4"`. See `parts/task-18.md:124-127` and `:288-290`.

**Verified this round** (tree at `7ddf82e`):
- `kata.go:105` and `:160` wrap the client error with `%v` and only `ErrUnavailable` with `%w`. `:112` calls `waitScheduled` at once after the create, and its first `Get` is `:130`. `:143-146` is the only `ctx.Err()`.
- `ValidateSpec` wraps `ErrUnsupported` throughout, and refuses an empty token (`backend.go:222-254`).
- `manager.go:110` puts the attach token in `backend.EnvToken`.
- task 29's `Role` grants `pods delete`.
- `sessions.go:332-337` and `:506`.

**Still UNVERIFIED:**
- `Standard_D8s_v5`'s allocatable and the Kata overhead, and so `6000m` / `20480Mi`. P07 measures them.
- That the scheduler filters a cordoned node for an affinity-bound session.
- Whether the fake clientset honours delete preconditions or passes contexts to reactors. No test depends on either.
- Whether PID 1 ignores SIGTERM inside a Kata guest the same way.

**Owner question:** none new. The b05 question (a larger SKU or a smaller session) stands, with its seam corrected.
