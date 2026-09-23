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
