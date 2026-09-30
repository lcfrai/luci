# 01 — Cloud agent machines: buy, adopt or integrate instead of build

**Status:** REVISING for the owner's second direction (relaunch, 2026-09-30); the first run
(stopped 09:47) stands except where section 1a says otherwise. Desk research only: nothing
installed or run (section 7).
**Date:** 2026-09-30. **Capability:** isolated per-session sandboxes that run unmodified
coding-agent CLIs (Claude Code, Codex, Gemini CLI) with a real PTY, warm pools, microVM-class
isolation, on Azure or bought with an AU region, driven by API as a `Backend` of our Go control plane.
**Owner direction (2026-09-30):** third-party first, the tool ASAP, days not weeks.

## 1. Answer first

- **Adopt, do not buy:** AKS Pod Sandboxing (Kata microVMs, run by Microsoft) plus
  `kubernetes-sigs/agent-sandbox` v1.0.4 (Apache-2.0: warm pool, claim, pause, resume,
  persistent storage), driven by our existing `katabe` through agent-sandbox's Go SDK, in
  Australia East.
- **7 days** to the first real cloud session from the cockpit (range 5 to 9). 4 of them are this
  option; 3 are glue any option needs (clone into the sandbox; control plane in the cloud behind
  Keycloak).
- **It replaces** the warm-capacity and machine-lifecycle work in phase 8 (2 tasks removed, 11
  reduced: about 36 plan-days become about 14), most of the Kata hardening in phase 9, and the
  capacity risk in `PROGRAM.md` (warm-pool sizing becomes configuration plus an HPA).
- **No bought product fits.** Every SaaS fails on Azure, AU residency, customer-cloud deployment,
  licence or isolation; the two credible "buy" options (Northflank BYOC, Runloop VPC deploy)
  document microVMs on AWS, not Azure, and need weeks of procurement. Daytona is AGPL and frozen;
  Coder is AGPL at its core; E2B has no Azure.
- **Fallback:** ACI + standby pools (same Hyper-V isolation class, no cluster) if Kata nodes or
  claim latency fail on day 1.
- **Owner calls:** the subscription and a dev baseline of about US$2,000 a month at list price;
  the warm-pool size; the idle policy; the isolation statement customers see.

## 1a. Revised for the owner's second direction

Relaunch, 2026-09-30, after the first run stopped at 09:47. The core answer stands. What changed:

| # | What | First run | Now | Why |
|---|---|---|---|---|
| 1 | How the cockpit reaches a cloud session | A thin `remotebe` in our desktop, over an in-cluster control plane | **Orca's per-workspace environment recipe** (`orca.yaml` → `environmentRecipes`). Its `create` script asks our control plane for a sandbox; at bind the sandbox starts Orca's headless runtime (`orca serve`) and hands back the pairing code; Orca streams terminals over its own end-to-end-encrypted WebSocket | The owner picked Orca. The recipe is Orca's documented seam for "a cloud sandbox, VM, or local Docker container" per worktree |
| 2 | Days to the first cloud session | 7 (5 to 9) | **8 (6 to 10)** | +0.5 to package Orca's runtime into the image; +0.5 for an authenticated `wss://` route to each sandbox. The `remotebe` share of the old glue becomes a 0.5-day recipe |
| 3 | Panacea | The model only (`OD-13` then forbade copying) | **Used where it is the better answer** (4.15): its hardened pod spec, deny-egress policy, package mirrors, tenant network baseline and Terraform are copied with attribution; its Keycloak realm, LiteLLM gateway and Trivy scans are called, not rebuilt. **It is not the sandbox host** | Owner: "Panacea is fine to use majority of"; snippets now allowed |
| 4 | Coolify, Supabase | Not considered | **Not used for this capability** (4.16): neither runs untrusted code in a microVM, keeps a warm pool, or hands out a PTY | Allowed by the owner; on merit they belong to deploy (06) and the Hub, not here |
| 5 | SaaS sandboxes Orca already supports | Modal: "reference, a demo fallback" | Vercel Sandbox (now in Sydney), Modal (`au`) and Fly: **reference, the fastest Orca demo, not the product** (4.17) | Orca wires them today; none is Azure or our environment (`OD-18`, `OD-5`) |
| 6 | A unit error | Phase 8 savings in "plan-days" | **Plan-hours.** The phase 8 task table's column is `Hours`; its 98.5 total is hours | The saving is 36 plan-hours to about 14, not 36 days |

**Unchanged:** the adopt (AKS Pod Sandboxing + `agent-sandbox` 1.0 in Australia East), the ACI
fallback and its switch rule, every reject (Daytona, Coder, E2B, Codespaces, Ona, DevPod), and
owner calls 1 to 5.

**What Orca changes inside the sandbox:**

- The image carries Orca's Linux AppImage plus its `xvfb` and GTK libraries beside the unmodified
  CLIs, and `orca serve` owns the terminals of an Orca session. `sandboxd` stays in the image for
  the paths that are not Orca (team mode's managed sessions); whether it also keeps the PTY under
  Orca is the cockpit study's ruling (02, §R4), not this one.
- A warm pod must not start `orca serve` before it is claimed. Orca's own guide: "Never snapshot
  a machine on which the Orca runtime has already run" (the runtime's identity and pairing keys
  are written on first start). The first run's late-bind is exactly where it starts.
- No git credential enters the sandbox. Orca's recipe guide passes a `GH_TOKEN` into the
  environment; we do not. The control plane clones onto the sandbox's disk before bind, and work
  comes back under `refs/panacea/lanes/…` (`OD-2`).

**The Panacea trade-off, in one line:** co-hosting on Panacea's estate would save the new
subscription and about US$1,250 a month of baseline (system nodes and firewall), but puts untrusted
agents beside live tenant workloads during its cutover, on a cluster with no Kata pool; our own
AKS keeps `OD-5`, copies Panacea's Terraform, and costs about one more day.

## 2. What the capability must do (the bar every candidate is scored against)

What already exists, so the bar is concrete: the phase 2 control plane drives every sandbox through
one Go interface, `backend.Backend` (`control-plane/internal/backend/backend.go:337`): `Kind`,
`Create(Spec)`, `Get`, `List`, `Attach(id, token) -> *websocket.Conn`, `Destroy`, `Close`. `Spec`
carries an image, a command plus args, env, a per-sandbox attach token, a workdir and resources.
Inside the image, our own `sandboxd` owns the PTY and serves `/healthz` and a WebSocket `/attach`.
`dockerbe` (local) is built; `katabe` (Kubernetes pods with a Kata `RuntimeClass`) exists in code
but has never run against Azure. So a candidate does **not** have to supply a terminal UI, an agent
loop or a cockpit. It has to supply **a place to run our image, fast, isolated, in AU, by API**.

| # | Must | Why it is hard-required |
|---|---|---|
| M1 | Runs an **arbitrary OCI image** (ours, with `sandboxd` and the unmodified CLIs) and a long-lived process with a **real PTY** | `OD-8`: nothing reads PTY bytes for meaning; the harness stays unmodified. A code-interpreter API is not enough |
| M2 | **Hardware-virtualised isolation** per session (microVM, Hyper-V or equivalent), not shared-kernel containers alone | Agents are untrusted. The locked plan is Kata on AKS for exactly this |
| M3 | A **bidirectional stream** into the sandbox (WebSocket or TCP port) reachable from our control plane, not only request/response exec | `Backend.Attach` returns a WebSocket; live terminals, attach, fork and resume depend on it |
| M4 | **Warm start**: a claim served from a pre-created pool in about a second, not a cold VM boot | Kata cold start is seconds to tens of seconds; the cockpit spawns sessions in parallel |
| M5 | **Azure, AU region**, in our subscription first (`OD-5`, `OD-18`), then a customer's own tenancy; or a bought service with an **AU region** and zero data retention | `OD-19`, `OD-24`: AU-only projects must stay in AU |
| M6 | **Licence we can ship into a customer tenancy**: permissive, or commercial that KPMG can buy | AGPL, GPL, SSPL, BUSL, fair-code and Elastic are restricted |
| M7 | **Egress control**: default-deny with an allowlist (the LiteLLM gateway, the git hosts, package mirrors) | Agents must reach only the gateway (`OD-19`, `OD-21`) and the repo hosts (`OD-4`) |
| M8 | **Session lifetime of hours to days**, and a way to keep a user's clones and `~/.claude` transcripts between sessions | Phase 8's "machine": a disk that keeps clones and conversations (`OD-2`, `OD-8`) |

| # | Should | Why |
|---|---|---|
| S1 | An API a Go program can call (REST or a Go SDK) | It becomes a third `Backend` beside `dockerbe` and `katabe` |
| S2 | Per-session env and secrets (the attach token, the per-session gateway token) | `OD-19`: each sandbox gets a scoped token, never an org key |
| S3 | Entra-based admin and audit (Azure RBAC, Activity Log, Log Analytics) | Regulated-enterprise evidence (phase 9) |
| S4 | Fast image bumps, so the cloud tracks the latest Claude Code CLI (`OD-20`) | The canary-gated image pipeline is the whole cloud-first argument |
| S5 | Snapshot or fork of a running sandbox | The cockpit already forks sessions; a memory snapshot makes it instant |

## 3. Candidates at a glance

"Days" means days to the first real cloud session from the cockpit, including the 3 days of glue
every option needs (section 5). "+ proc." means a new-vendor procurement comes first.

| Candidate | Licence | Deploys as | AU | Isolation | Stream into it | Warm start | Days | Verdict |
|---|---|---|---|---|---|---|---|---|
| **AKS Pod Sandboxing + `agent-sandbox` 1.0** | Apache-2.0 + Azure service | Our AKS; a customer's AKS later | Australia East | Kata microVM on Hyper-V | Our `sandboxd` on the pod IP | `SandboxWarmPool`; "a couple of seconds" *(upstream)* | **7** | **adopt** |
| ACI + standby pools | Azure service | ARM in any subscription | Yes | Hypervisor per container group | Our `sandboxd` on a private IP | Standby pool; latency unpublished | 7 | integrate (fallback only) |
| ACA dynamic sessions (custom container) | Azure service | ARM in any subscription | East and Southeast | Hyper-V per session | HTTP through the pool endpoint; WebSocket undocumented | Ready sessions, "milliseconds" *(upstream)* | 9 | reference |
| Northflank | Commercial | SaaS, or BYOC data plane in our Azure | BYOC in AU East | Kata / Firecracker / gVisor; microVM on Azure undocumented | REST API | "under a second" *(upstream)* | 8 + proc. | reference |
| Runloop | Commercial | SaaS, or VPC deploy (Azure listed) | Not documented | microVM | Python / TS SDKs | "a few seconds" *(upstream)* | 8 + proc. | reference |
| Modal | Commercial SaaS; open SDKs | SaaS only | `au` pin at 1.75x | gVisor (not hardware) | Go SDK: PTY, port tunnels | Fast *(upstream)* | 4 + proc. | reference |
| Coder | **AGPL-3.0** core + commercial Premium | Helm on our AKS | Ours | Whatever the template runs | Reconnecting PTY over WebSocket | Prebuilds (Premium only) | 9 + proc. | reject |
| E2B | Apache-2.0 SDK and runtime; SaaS | SaaS; BYOC AWS/GCP only | No Azure | Firecracker microVM | PTY API | Snapshot resume | 20+ | reject |
| Daytona | **AGPL-3.0**, frozen June 2026; core now private | SaaS US / EU / India | No | Not assessed | PTY API | "under 90ms" *(upstream)* | — | reject |
| GitHub Codespaces | Commercial | GitHub-hosted | Via GHE.com residency | VM per codespace | Through GitHub's tunnel | Prebuilt images only | — | reject |
| Ona (ex-Gitpod) | Commercial; OpenAI-owned | SaaS + AWS/GCP runners | No Azure | VM | — | — | — | reject |
| OpenHands runtime | MIT | Self-hosted Docker / remote | Ours | Container | Its own action server | — | — | reference |
| DevPod | MPL-2.0, stalled since 2025 | Client-side launcher | — | Provider's | SSH | None | — | reject |

**GLASSES coverage gap.** The catalogue (offline and its MCP server, both generated 2026-09-29)
returns one entry for this capability, `daytonaio/daytona`, with licence "unknown" and no note
that it is frozen. None of the adopt or fallback candidates is catalogued. Worth adding:
`kubernetes-sigs/agent-sandbox`, `e2b-dev/runtime`, `coder/coder`, and the two Azure services.

## 4. Candidates in detail

Facts marked *(upstream)* are the vendor's or project's own claim, read on 2026-09-30 and not
measured by us. GitHub numbers are from the GitHub API on 2026-09-30.

### 4.1 AKS Pod Sandboxing (Kata) + `kubernetes-sigs/agent-sandbox` — **ADOPT (the recommendation)**

- **What it is.** Two third-party pieces that together are the whole sandbox layer. **AKS Pod
  Sandboxing** is Microsoft's managed Kata Containers: a node pool created with
  `--workload-runtime KataVmIsolation` on Azure Linux ships Kata pre-installed and registers the
  `kata-vm-isolation` RuntimeClass, so every pod gets its own Hyper-V-backed microVM and kernel.
  **agent-sandbox** (Kubernetes SIG Apps) adds the lifecycle we were going to write:
  `Sandbox` (a stateful singleton pod with stable identity, persistent storage, scheduled
  deletion, pause and resume), `SandboxTemplate`, `SandboxWarmPool` (pre-created sandboxes; per
  the 2026-09-28 study it has a scale subresource, so a plain HPA drives it) and `SandboxClaim`
  (adopt one from the pool). It
  ships a **Go SDK** (`sigs.k8s.io/agent-sandbox/clients/go/sandbox`) and an optional
  `sandbox-router` reverse proxy.
- **Licence.** Apache-2.0. AKS is an Azure service.
- **Maturity.** 4,086 stars; v1.0.0 on 2026-08-28, then weekly patches to **v1.0.4
  (2026-09-24)**; pushed 2026-09-29. The CRDs are **`v1beta1`** (1.0 removed `v1alpha1`), so the
  API is stable-ish, not GA. Upstream ships three AKS examples, including a per-owner `kata-aks`
  warm pool, last edited 2026-08-14. Recent releases add warm-pool refill shaping
  (`replenish-delay`, `max-refill-rate`), Ed25519 scoped tokens on the router (v1.0.2), direct
  in-cluster pod-IP connectivity in the Go SDK (v1.0.2) and an egress-control blueprint (v1.0.3).
  Naming trap: upstream also ships a daemon called `sandboxd`; ours is a different program.
- **Deploy.** Our AKS cluster in **Australia East**; later the same manifests into a customer's AKS.
  Kata pool rules: Azure Linux, a Gen2 VM size with nested virtualisation (Dsv5 class), Kubernetes
  ≥ 1.27. Install is two `kubectl apply` manifests or the Helm chart.
- **Enterprise fit.** Entra through AKS Entra RBAC; audit through AKS diagnostic logs to Log
  Analytics; residency is wherever the cluster is. Egress default-deny with an allowlist is
  ordinary Kubernetes: Cilium network policy plus the Azure Firewall already in the phase 8 plan.
- **Against the bar.** M1 yes (any image; our `sandboxd` owns the PTY). M2 yes (Kata microVM). M3
  yes: the control plane in the cluster dials the pod IP directly, or goes through `sandbox-router`
  (`kubectl port-forward` cannot enter a Kata guest). M4 yes: *(upstream, the v1.0.4 example
  README)* the warm pool "adopts a pre-warmed Kata pod ... so the claim becomes Ready in a couple
  of seconds". M5 yes. M6 yes. M7 yes. M8 yes
  (a PVC per sandbox or per user; pause and resume). S1 yes (Go SDK). S5 no memory snapshot.
- **Gaps.** (1) A warm pod already runs when it is claimed, so per-session env (the attach token,
  the per-session gateway token) cannot be set at pod creation: `sandboxd` needs a **late-bind**
  call. This is true of every warm-pool option in this study. (2) Claim latency on our image is
  unmeasured. (3) Defender for Containers cannot assess Kata pods (already a recorded compliance
  gap). (4) AKS node auto-provisioning (Karpenter) does not support Kata, so the Kata pool scales
  with the cluster autoscaler. (5) The upstream router example sets
  `ALLOW_UNAUTHENTICATED_ROUTER=true` ("an open proxy into your sandbox pods"); we either turn
  its scoped tokens on or skip the router and dial pod IPs from an in-cluster control plane.
- **Integration effort: 7 days** (range 5 to 9) to the first real cloud session from the
  cockpit, itemised in section 5. Four of them are this option's; three are glue every option
  needs.
- **Verdict: adopt.** It is the only candidate that meets all eight musts today with a permissive
  licence, and it reuses what is built: `katabe` already speaks Kubernetes and has compiled
  against the `Backend` seam since phase 2.

### 4.2 Azure Container Apps dynamic sessions (custom container pools) — **REFERENCE**

- **What it is.** Microsoft's managed sandbox pool. Each session is a container in its own
  **Hyper-V sandbox**, allocated from a pool of ready sessions in *(upstream)* milliseconds.
  Microsoft lists "AI agents, and development environments" and "a cloud-based shell" as uses.
- **Licence / deploy.** An Azure service (a `Microsoft.App/sessionPools` resource in a
  workload-profiles environment). Regions include **Australia East and Australia Southeast**.
- **API.** ARM for the pool. Data plane: every request carries an Entra token (audience
  `https://dynamicsessions.io`, role *Azure ContainerApps Session Executor*) and an `identifier`;
  the path is forwarded to the container's target port. `getSession`, `listSessions`,
  `stopSession`. Lifecycle `Timed` (idle cooldown 300 to 3,600 s) or `OnContainerExit`
  (`maxAlivePeriodInSeconds`; Microsoft's own REST example uses 86,400, i.e. 24 hours).
- **Billing.** Custom container pools run on dedicated **E16** instances (Dedicated plan, plus the
  plan management fee); the node count follows active plus ready sessions.
- **Why not.** M7 fails: egress is a pool-wide on/off toggle, and a Microsoft Q&A answer
  (2025-01-17) confirms no CIDR or FQDN allowlist for session pools on their own. M8 fails: the
  2026-01-01 API has no volume or disk, so every session starts empty. M3 unverified: the docs
  describe HTTP requests only and say nothing about WebSocket. S2 weak: env and secrets are
  pool-wide, so per-session tokens need the same late-bind.
- **Integration effort:** about 9 days: 4 for the pool, the `Backend`, late-bind and transcript
  export (there is no disk), 2 for a reverse-dial fallback if WebSocket does not pass the pool
  endpoint, 3 of shared glue; and the egress gap stays open.
- **Verdict: reference.** The best whole product on paper and the lowest operations burden;
  re-test it when Microsoft ships per-pool egress rules and volumes.

### 4.3 Azure Container Instances + standby pools — **INTEGRATE only as the fallback**

- **What it is.** Serverless container groups with no cluster. Microsoft: ACI "guarantees your
  application is as isolated in a container as it would be in a VM" (hypervisor isolation per
  container group). **Standby pools** (GA June 2025) keep `maxReadyCapacity` groups fully
  provisioned and running from a *container group profile*, refill `Always`, zone-aware, and can
  be placed on our subnets. **Config maps** can change a running group's configuration without a
  restart, which is the late-bind channel for per-session tokens.
- **Licence / deploy.** An Azure service in Australia East, in our subscription or a customer's.
  Go access through the MIT-licensed Azure SDK for Go.
- **Against the bar.** M1 yes (x64 images up to 15 GB; no privileged operations). M2 yes. M3 yes:
  a control plane inside the VNet dials `sandboxd` on the group's private IP. M4 partly: claims
  come from the pool, but Microsoft publishes no claim latency, and an empty pool falls back to a
  cold create. M5 yes. M6 yes. M7 partly: inside a VNet, Microsoft says a NAT gateway is "the only
  supported configuration for outbound connectivity", so an FQDN allowlist needs an NSG that only
  permits an explicit proxy (Azure Firewall explicit-proxy mode or Envoy). M8 partly: Azure Files
  (SMB) only, which is slow for git working trees; clone to local disk and keep only
  `~/.claude` on the share. Groups can restart during platform maintenance and change IP.
- **Gaps.** One image per profile (an image bump is a profile revision plus a pool update); no
  snapshot or fork; per-region ACI core quotas need raising; standby groups are running groups,
  so they are billed while they wait (a reasonable reading of "provisioned and running"; the
  exact meter was not found).
- **Integration effort: 7 days** — a new `acibe` over ARM (2), late-bind via config map (0.5),
  the explicit-proxy egress (1), smoke (0.5), plus the 3 days of shared glue (section 5).
- **Verdict: integrate only if the Kata spike in section 5 fails** (for example no nested-virt
  Dsv5 capacity in Australia East). Same Hyper-V class of isolation, no cluster to run, but a
  weaker egress story and no fast disk.

### 4.4 Coder (Community / Premium, self-hosted) — **REJECT** (reference for prebuild claims)

- **What it is.** The best-known self-hosted cloud development environment. Workspaces are
  Terraform templates (Kubernetes pods, VMs, containers); an in-workspace agent serves a
  reconnecting PTY over WebSocket, SSH and port forwarding. **Prebuilt workspaces** keep
  `instances` of a preset warm and hand one over on create ("ownership transfers from the
  `prebuilds` user"), with cron-scheduled pool sizes. OIDC sign-in (Entra works), audit log, RBAC.
- **Licence.** Core **AGPL-3.0**; the `enterprise/` directory is under `LICENSE.enterprise`
  (commercial, licence-key enforced). **Prebuilt workspaces need a Premium licence** (Coder docs).
  Premium is priced **annually per user**; an "AI Premium" tier is custom-priced.
- **Maturity.** 16,759 stars; v2.36.6 on 2026-09-18; VC-backed vendor. `coder/agentapi` (MIT),
  which drives Claude Code by reading the rendered terminal, was **archived** in September 2026;
  Coder's v2.35 docs say Tasks leave new releases from v2.37 *(upstream, not re-verified)*.
- **Deploy / fit.** Helm into our AKS, with Kata through the template's `runtime_class_name`, so
  it does not remove the cluster, the Kata pool or the egress work. Residency is ours.
- **Why not.** (1) AGPL core: restricted for anything we deploy into a customer tenancy, and
  importing `codersdk` (Go, AGPL) into our control plane would pull it under AGPL, so we would
  speak REST only. (2) A second control plane with its own users, templates and workspace model
  sitting under ours; every session becomes a Terraform apply. (3) Its provisioner is Terraform,
  BUSL-1.1 since 2023; OpenTofu substitution was not verified. (4) Procurement for Premium.
- **Integration effort:** about 9 days plus procurement. **Verdict: reject**; borrow the
  prebuild-claim and cron-sized-pool ideas.

### 4.5 Northflank (managed, or BYOC into our Azure) — **REFERENCE** (the strongest "buy")

- **What it is.** A PaaS with first-class sandboxes: Kata with Cloud Hypervisor, Firecracker or
  gVisor; *(upstream)* microVM boot "in under a second"; volumes 4 GB to 64 TB; API, CLI and
  GitOps. **BYOC** installs its data plane into our own AWS, GCP or **Azure** account: "your data
  and compute stay inside your infrastructure, and you pay your cloud provider directly".
- **Licence / price.** Commercial SaaS. Managed sandboxes $0.01667 per vCPU-hour and $0.00833 per
  GB-hour; the BYOC platform fee is not published. SOC 2 Type 2 and HIPAA claimed.
- **Gaps.** Its BYOC docs describe microVM node pools only for AWS ("bare-metal instances or
  instance types that support nested virtualisation"); **microVM isolation on Azure BYOC is not
  documented**, and where nested virtualisation is missing it falls back to gVisor. Its control
  plane is Northflank's SaaS, not in AU. Entra SSO and audit export not verified. New vendor,
  so KPMG procurement and a TSRA before first use.
- **Integration effort:** about 8 days after procurement (a REST `Backend`, late-bind, clone,
  control-plane placement). **Verdict: reference.** Revisit only if the owner prefers paying a
  vendor to run the sandbox cluster over running AKS ourselves (section 6).

### 4.6 Runloop (managed devboxes, or VPC deploy) — **REFERENCE**

- **What it is.** MicroVM "devboxes" for coding agents: blueprints from Dockerfiles, *(upstream)*
  "a few seconds" to first command, snapshot, suspend and resume, 10,000+ parallel sandboxes.
- **Licence / price.** Commercial. $0.108 per CPU-hour and $0.0252 per GB-hour on the standard
  tier; VPC deploy is Enterprise, custom-priced. SOC 2 Type II, HIPAA; SSO and audit logs listed.
- **Gaps.** "VPC Deploy support for AWS, GCP and Azure" is on its site, but what runs in our
  account, Azure's maturity and AU region support are not documented. SDKs are Python and
  TypeScript (no Go). Procurement as for Northflank.
- **Integration effort:** about 8 days after procurement. **Verdict: reference.**

### 4.7 Modal (managed Sandboxes) — **REFERENCE** (fastest integration, wrong shape)

- **What it is.** Serverless containers; Sandboxes on **gVisor**. The official **Go SDK**
  (`libmodal/modal-go`) covers PTY, exec, encrypted port tunnels, volumes, filesystem snapshots,
  OIDC identity tokens and region placement. Region pinning includes **`au`** and
  `ap-melbourne`, at a 1.75x price multiplier for narrow regions. Enterprise: SSO, HIPAA BAA.
- **Gaps.** gVisor is a user-space kernel, not hardware virtualisation (M2 partial). SaaS only:
  it cannot be deployed into our or a customer's Azure tenancy (`OD-5`). Residency of images,
  logs and control-plane metadata is not documented beyond compute pinning. Procurement.
- **Integration effort:** about 4 days after procurement, the shortest of any candidate.
  **Verdict: reference** — a demo fallback, not the product's home.

### 4.8 E2B (managed, BYOC or self-host) — **REJECT for Azure**

- **What it is.** Firecracker microVM sandboxes with PTY, Dockerfile templates, and pause and
  resume that keeps memory. SDK Apache-2.0 (14,031 stars, `e2b@2.51.0` on 2026-09-18);
  infrastructure open source as `e2b-dev/runtime` (formerly `e2b-dev/infra`, Apache-2.0, 1,642
  stars, release `2026.30` on 2026-09-10).
- **Why not.** BYOC is AWS and Google Cloud only ("Azure is not yet supported"). Self-hosting
  marks GCP supported, AWS beta, Azure an unchecked roadmap item. Running it on Azure is a port of
  Firecracker infrastructure onto nested-virt VMs: weeks, not days. No official Go SDK was found.
- **Integration effort:** 20+ days on Azure. **Verdict: reject**; reference its PTY and
  pause/resume API shape.

### 4.9 Daytona — **REJECT**

- The repository's own README: "As of June 2026, Daytona's core development has moved to a
  private codebase"; the public repo (71,690 stars) is frozen at **v0.190.0 (2026-06-23)**,
  AGPL-3.0, with no further fixes or security patches. Self-hosting therefore means running a
  frozen AGPL codebase. The SaaS regions are US East, US West, EU Central, EU West and India;
  **no AU region**. "Custom regions" (our own runners on Daytona's control plane) are
  invite-only and experimental. **Verdict: reject** on licence, maintenance and residency. It is
  the only entry the GLASSES catalogue returns for this capability, and that entry is stale.

### 4.10 GitHub Codespaces — **REJECT**

- Per-codespace VMs, now GA on GitHub Enterprise Cloud **with data residency in Australia**
  (2026-04-01), organisation-owned codespaces only. But: GitHub repositories only, while `OD-4`
  needs Azure DevOps too; KPMG's GitHub is an EMU on github.com, not a GHE.com data-residency
  tenant; prebuilds speed the image, but there is no pool of running machines to claim; the
  terminal arrives through GitHub's tunnel, not a socket we own; it cannot run in a customer's
  cloud. **Verdict: reject.**

### 4.11 Ona (formerly Gitpod) — **REJECT**

- OpenAI announced its acquisition of Ona on 2026-06-11 to run Codex agents (several press
  reports; no primary source read). Runners are AWS and GCP; no Azure runner. Gitpod Classic
  (`gitpod-io/gitpod`, AGPL-3.0) last released 2022.11.3. **Verdict: reject**: no Azure, and
  its roadmap now belongs to one model vendor.

### 4.12 OpenHands runtime — **REFERENCE**

- MIT core, 89,540 stars, v1.24.0 on 2026-09-25 (`OpenHands/OpenHands`). Its runtime is built
  around OpenHands' own action-execution server for its own agent, with Docker and remote
  runtimes; it is not a general place to run unmodified CLIs on a PTY. **Verdict: reference.**

### 4.13 DevPod — **REJECT**

- MPL-2.0, 15,248 stars, but the last release is v0.6.15 (2025-03-10) and the last push
  2025-11-14. It is a client-side launcher for dev containers with no multi-tenant control plane
  and no warm pool. **Verdict: reject.**

### 4.14 Also looked at, briefly

| Candidate | What it is | Why not |
|---|---|---|
| Morph Cloud | SaaS microVMs with snapshot, branch and restore *(upstream)* under 250 ms | SaaS; regions and residency not documented; not deployable in Azure |
| `superradcompany/microsandbox` | Apache-2.0 libkrun microVMs on one KVM host; 8,478 stars; v0.7.4 on 2026-09-29 | A single-host runtime, not a pool or a service; a candidate for **local mode** later, not cloud |
| Microsoft Dev Box | Managed Windows dev VMs | Retiring 2026–2028 (already ruled out in `CLAUDE.md`) |
| AWS Bedrock AgentCore Runtime, Fly.io Machines | microVM-per-session services | Wrong cloud for `OD-18`, or no Azure; not researched further |
| `coder/agentapi` | Chat API synthesised from a terminal screen | Archived; and reading the rendered screen is what `OD-8` rules out |

## 5. Recommendation

**Adopt AKS Pod Sandboxing (Microsoft-managed Kata) plus `kubernetes-sigs/agent-sandbox` v1.0.4
(Apache-2.0) as the `katabe` backend, in Australia East. 7 days to the first real cloud session
from the cockpit.**

Why this and not a bought service:

1. It is the only candidate that meets all eight musts today (section 2), with a permissive licence.
2. Microsoft runs the hard part (the Kata runtime, the guest kernel, the hypervisor) and
   Kubernetes SIG Apps maintains the lifecycle we were about to write (pool, claim, pause,
   resume, persistent storage, scheduled deletion). We write glue only.
3. It reuses what exists: `katabe` already targets Kubernetes behind the unchanged seven-method
   `Backend`; phase 8 is already written for AKS; `OD-18` chose Azure for this reason.
4. It goes into a customer's own AKS unchanged, which a SaaS (Modal, Runloop, Northflank's
   control plane) cannot, and it needs no new-vendor procurement, which would cost weeks.

### The 7 days

| Day | Step | Whose |
|---|---|---|
| 0.5 | AKS in Australia East (Overlay + Cilium, Entra RBAC), an Azure Linux Kata pool on a nested-virt Dsv5 size with the cluster autoscaler, ACR. A scripted `az` runbook now; phase 8 task 06's OpenTofu module codifies it after | this option |
| 0.5 | Install agent-sandbox v1.0.4; one `SandboxTemplate` (our image, `kata-vm-isolation`, explicit requests, no service-account token, seccomp, a PVC); one `SandboxWarmPool`; time 10 parallel claims. This is the spike the 2026-09-28 study asked for, shrunk because upstream ships the AKS manifests | this option |
| 1 | `katabe`: `Create` = a `SandboxClaim` through the Go SDK; `Get`/`List` from `Sandbox` status; `Destroy` deletes the claim; `Attach` dials the pod IP with the token | this option |
| 0.5 | `sandboxd` late-bind: a warm pod starts unbound; the first `POST /bind`, reachable only from the control plane by NetworkPolicy, sets the attach token, the per-session gateway token (`OD-19`) and the harness command, then starts the PTY | this option |
| 1 | Egress default-deny; allow DNS, the LiteLLM gateway, the git hosts (`OD-4`) and package mirrors by FQDN (Cilium FQDN policy, or phase 8 task 07's Azure Firewall); ingress only from the control plane | this option |
| 1.5 | Worktree into the sandbox (seam S8.3): clone inside it with a scoped credential (`OD-22` for fetch, the signed-in person's for push), fetch back under `refs/panacea/lanes/…` (`OD-2`, phase 3) | shared glue |
| 1.5 | The control plane in the cluster behind the Keycloak broker (`OD-30`), and a thin `remotebe` so the desktop lists, creates and attaches cloud sessions (the first slice of phase 8 tasks 17, 18, 22) | shared glue |
| 0.5 | Smoke: 10 parallel Claude Code sessions; attach, fork, resume, destroy; record claim p50/p95 and pool refill under a burst | this option |

### What it replaces in our plan

- **Phase 8** (38 task files, about 98.5 plan-days by the plan's own estimates; the stream
  reports 5 of 35 done). **Removed:** task 12 (warm capacity:
  balloons and pre-pull, 2 days) and task 19 (broker machines: wake, sleep, idle sleeper, caps,
  4 days), which become `SandboxWarmPool` and the `Sandbox` lifecycle. **Materially reduced:**
  14 (pod hardening, 2.5), 15 (machine volumes, 3), 23 (per-machine backends and reaper, 3.5)
  and 32 (Kata proving checklist, 2). **Reduced further if** the plan's machine pod with a shared
  disk becomes one `Sandbox` per session with its own retained PVC, which `OD-2`'s one clone per
  worktree already implies: 20, 21, 21b, 24a, 24b, 25 and 25b (19 plan-days) shrink by about
  half. In all, 13 tasks and about 36 plan-days become about 14. That last step is a plan edit
  that needs its own review; the word "machine" and what the user sees do not change.
- **The critical path.** The first cloud session needs only the 7-day slice above. The rest of
  phase 8 (OpenTofu stacks 01–10, charts 11 and 13, identity and broker 16–18, 22, UI 26,
  pipelines and runbooks 27–31, close-out 33) stays and follows, off the critical path.
- **Phase 9** has no plan yet. The Kata runtime, guest kernel and hypervisor patching become
  Microsoft's; what stays ours is admission policy (Kyverno, per the 2026-09-28 study), network
  policy and the ISM evidence.
- **Capacity.** `PROGRAM.md` open risk 1 (warm-pool sizing unproven) becomes a pool size, an HPA
  on the pool's scale subresource and the cluster autoscaler on the Kata pool, measured by the
  day-7 smoke instead of a hand-built pool controller.

### What we still build (the glue)

1. The `katabe` claim client and the `sandboxd` late-bind.
2. Clone-in-sandbox and lane fetch-back with scoped git credentials.
3. The in-cluster control plane behind Keycloak, and the thin `remotebe`.
4. The egress allowlist, the ingress policy and the template hardening.
5. The canary-gated image bump that tracks the latest Claude Code CLI (`OD-20`), rolling the
   template and refilling the pool; and our own burst load driver (upstream has none).

### Risks, and the rule that switches to the fallback

- `katabe` has never run against Azure; Kata on AKS is new to us.
- Nested-virt Dsv5 capacity and quota in Australia East are unconfirmed for our subscription.
- The agent-sandbox CRDs are `v1beta1` and a month past 1.0; pin the version.
- Defender for Containers cannot assess Kata pods (a recorded compliance gap); AKS
  node auto-provisioning does not support Kata.
- **Switch rule:** if day 1 cannot get Kata nodes in Australia East, or claim p95 stays above
  about 5 seconds after tuning, move to **ACI + standby pools (4.3)**: same Hyper-V isolation
  class, no cluster, about 2 extra days for a new `acibe`.

## 6. Calls only the owner can make

List prices below are from the Azure Retail Prices API on 2026-09-30, USD, Australia East,
pay-as-you-go, Linux. No discount, reservation or KPMG agreement is applied.

1. **The subscription and its spend (money).** Day 1 needs `OD-5`'s environment. A dev baseline
   is about **US$2,000 a month**: 2 × D4s_v5 system nodes ($0.24/h each, about $350), 2 × D8s_v5
   Kata nodes kept for the warm pool ($0.48/h each, about $700), Azure Firewall Standard
   ($1.25/h, about $910, plus $0.016/GB). Not included: Log Analytics, ACR, disks, data transfer,
   the AKS Standard tier. Azure Firewall Basic ($0.395/h) would cut about $620 of that for dev.
   Each extra concurrent 2 vCPU / 4 GB session costs roughly **US$0.12 to 0.16 an hour**
   (ACI list: $0.0486 per vCPU-hour, $0.00532 per GB-hour; AKS: about three such sessions on a
   D8s_v5), so cost does not separate the adopt option from the fallback.
2. **Warm-pool size (money against what users feel).** Each warm sandbox is a running microVM
   waiting for a user. The recommendation starts with 3 per image during business hours and
   scales down at night; a bigger pool is a faster first keystroke at a higher idle bill.
3. **Idle policy (customer-visible).** How long an idle session keeps running before it is paused.
   Pausing keeps the disk; resuming is a Kata cold boot of seconds rather than an instant attach.
4. **What customers are told (customer-visible).** The isolation badge (seam S8.5) would say, in
   substance, "each session runs in its own Hyper-V-isolated microVM in Australia East". The owner
   should also approve how the known gap is disclosed: Microsoft Defender for Containers cannot
   assess Kata pods.
5. **Buy instead of run (money; not recommended now).** Paying a vendor to operate the sandbox
   cluster (Northflank BYOC or Runloop VPC deploy) instead of running AKS ourselves. Against it
   today: new-vendor procurement takes weeks, microVM isolation on Azure is undocumented for
   both, and agent-sandbox costs nothing. Worth reopening only if running the cluster becomes the
   bottleneck.

## 7. What this study could not verify

Nothing was installed, deployed or run. Every capability claim is from documentation, a
project README, a vendor page or the GitHub API, read on 2026-09-30.

**Would change the recommendation if wrong (checked by the day-1 and day-7 steps):**

1. Claim latency on AKS with our image. "A couple of seconds" is upstream's claim on its own image.
2. Nested-virt Dsv5 capacity and quota in Australia East for our subscription.
3. That a pooled `Sandbox` can carry a PVC template and that pause and resume work on Kata. The
   README claims persistent storage and pause/resume in general, not on Kata specifically.
4. That the agent-sandbox Go SDK's pod-IP connectivity and the router's scoped tokens behave as
   the v1.0.2 release notes describe.

**Recorded, not load-bearing:**

- ACA dynamic sessions: WebSocket through the pool endpoint; whether a VNet environment's UDR
  and firewall apply to session egress; the ceiling on `maxAlivePeriodInSeconds`; custom-container
  CPU and memory limits.
- ACI: standby-pool claim latency; the billing meter for waiting standby groups; per-group
  CPU and memory limits in Australia East; the explicit-proxy egress design.
- Northflank: microVM isolation on Azure BYOC; the BYOC fee; Entra SSO and audit export.
- Runloop: what runs in our account on an Azure VPC deploy; AU regions.
- Modal: residency of control-plane metadata, images and logs; Entra SSO.
- Coder: the Tasks deprecation line in its v2.35 docs; whether OpenTofu can replace Terraform in
  its provisioner; whether a Premium licence changes the AGPL position of the core (a legal
  question, not answered here).
- Ona's acquisition by OpenAI: press reports only, no primary source read.
- Prices: vendor list prices from their own pages or third-party summaries; Azure prices from the
  Retail Prices API; no quote obtained.
- The phase 8 machine-pod simplification in section 5 is a proposal; it needs a plan edit and
  the usual adversarial review before anyone builds it.
- The corporate proxy blocked nothing in this study; the GitHub API rate limit cut off one
  release-notes call, read instead from the GitHub releases page.

## 8. Sources

**In this repo (read-only):** `docs/superpowers/planning/2026-09-23-owner-decisions.md`
(`OD-2`, `OD-4`, `OD-5`, `OD-8`, `OD-14`, `OD-18`–`OD-24`, `OD-30`); `docs/PROGRAM.md` (phases
8 and 9, open risk 1); `control-plane/internal/backend/backend.go:337`;
`control-plane/internal/backend/katabe/kata.go` (the dormant note); the phase 8 plan at
`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md` (task table,
lines 558–633); `docs/superpowers/planning/2026-09-28-oss-adoption/00-summary.md` (row 8 and the
Daytona reject). GLASSES: `gsearch.py` offline and `https://lcfr.ai/api/mcp` `glasses_search`.

**Kubernetes / AKS:** <https://github.com/kubernetes-sigs/agent-sandbox> (README, releases) ·
<https://agent-sandbox.sigs.k8s.io/docs/use-cases/examples/kata-aks/> ·
<https://pkg.go.dev/sigs.k8s.io/agent-sandbox/examples/kata-aks> ·
<https://learn.microsoft.com/en-us/azure/aks/use-pod-sandboxing>

**Azure Container Apps dynamic sessions:**
<https://learn.microsoft.com/en-us/azure/container-apps/sessions> ·
<https://learn.microsoft.com/en-us/azure/container-apps/sessions-custom-container> ·
<https://learn.microsoft.com/en-us/azure/container-apps/session-pool> ·
<https://learn.microsoft.com/en-us/azure/container-apps/sessions-usage> ·
<https://learn.microsoft.com/en-us/rest/api/resource-manager/containerapps/container-apps-session-pools/create-or-update?view=rest-resource-manager-containerapps-2026-01-01> ·
<https://learn.microsoft.com/en-us/answers/questions/2148110/with-session-pools-can-you-specify-a-custom-cidr-r> ·
<https://learn.microsoft.com/en-us/azure/container-apps/billing>

**Azure Container Instances:**
<https://learn.microsoft.com/en-us/azure/container-instances/container-instances-overview> ·
<https://learn.microsoft.com/en-us/azure/container-instances/container-instances-standby-pool-overview> ·
<https://learn.microsoft.com/en-us/azure/container-instances/container-instances-standby-pool-create> ·
<https://techcommunity.microsoft.com/blog/azurecompute/general-availability-standby-pools-for-azure-container-instances/4415378>

**Prices:** <https://prices.azure.com/api/retail/prices> (Australia East, 2026-09-30).

**Vendors:** <https://coder.com/pricing> ·
<https://coder.com/docs/admin/templates/extending-templates/prebuilt-workspaces> ·
<https://coder.com/docs/ai-coder/tasks> · <https://northflank.com/product/sandboxes> ·
<https://northflank.com/docs/v1/application/sandboxes/deploy-sandboxes-in-your-cloud> ·
<https://runloop.ai/deploy-to-vpc> · <https://docs.runloop.ai/docs/devboxes/overview> ·
<https://modal.com/docs/guide/region-selection> ·
<https://pkg.go.dev/github.com/modal-labs/libmodal/modal-go> · <https://docs.e2b.dev/byoc> ·
<https://github.com/daytonaio/daytona> · <https://www.daytona.io/docs/en/regions/> ·
<https://www.daytona.io/docs/en/runners/> ·
<https://github.blog/changelog/2026-04-01-codespaces-is-now-generally-available-for-github-enterprise-with-data-residency/> ·
<https://thenextweb.com/news/openai-acquires-ona-codex> (press) ·
<https://cloud.morph.so/docs/developers>

**GitHub API** (`api.github.com/repos/…`, 2026-09-30): `kubernetes-sigs/agent-sandbox`,
`daytonaio/daytona`, `e2b-dev/runtime`, `e2b-dev/E2B`, `coder/coder`, `coder/agentapi`,
`OpenHands/OpenHands`, `loft-sh/devpod`, `superradcompany/microsandbox`, `gitpod-io/gitpod`.
