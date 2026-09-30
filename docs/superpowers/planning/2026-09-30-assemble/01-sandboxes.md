# 01 — Cloud agent machines: buy, adopt or integrate instead of build

**Status:** COMPLETE, revised for the owner's second direction (relaunch, 2026-09-30). The first
run (stopped 09:47) stands except where section 1a says otherwise; revised passages are marked
*(revision)*. Desk research only: nothing installed or run (section 7).
**Date:** 2026-09-30. **Capability:** isolated per-session sandboxes that run unmodified
coding-agent CLIs (Claude Code, Codex, Gemini CLI) with a real PTY, warm pools, microVM-class
isolation, on Azure or bought with an AU region, driven by API as a `Backend` of our Go control plane.
**Owner direction (2026-09-30):** third-party first, the tool ASAP, days not weeks.

## 1. Answer first (revised for the owner's second direction: section 1a says what changed)

- **Adopt, do not buy:** AKS Pod Sandboxing (Kata microVMs, run by Microsoft) plus
  `kubernetes-sigs/agent-sandbox` v1.0.4 (Apache-2.0: warm pool, claim, pause, resume,
  persistent storage), driven by our existing `katabe` through agent-sandbox's Go SDK, in **our
  own AKS in Australia East** (`OD-5`). The cluster is stood up from Panacea's Terraform and the
  sandbox hardened with Panacea's Forge pod spec, deny-egress policy and package mirrors, copied
  with attribution. Fallback: ACI + standby pools if Kata nodes or claim latency fail on day 1.
- **Orca reaches it through one recipe.** An `orca.yaml` environment recipe's `create` script asks
  our control plane for a sandbox; at bind the sandbox starts `orca serve` and returns Orca's
  pairing code. Orca then owns the terminal stream; nothing reads PTY bytes for meaning (`OD-8`).
- **8 days** to the first real Claude Code session from Orca in a Kata microVM (range 6 to 10).
  4.5 are this option; 3.5 are glue any option needs (clone onto the sandbox's disk, the control
  plane in the cluster behind Keycloak, the `wss://` route, the recipe).
- **It replaces** the warm-capacity and machine-lifecycle work in phase 8 (2 tasks removed, 11
  reduced: about 36 plan-hours become about 14), the Kata runtime work in phase 9, and the
  capacity risk in `PROGRAM.md` (warm-pool sizing becomes configuration plus an HPA). If the
  cockpit study (02) replaces our desktop with Orca, phase 8 tasks 22 (`remotebe`) and 26 (the
  cockpit's cloud-machine view), 6 plan-hours, become the recipe too; the rest of group F
  (23 to 25b) follows 02's lane ruling.
- **Everything else, judged on merit.** Panacea is used, not hosted on: it has no agent-sandbox
  product (Forge's preview pods are shared-kernel, preview-shaped, with no pool or disk).
  Coolify and Supabase do not fit this capability. No bought product fits: each fails on Azure,
  our tenancy, a customer's cloud, licence or isolation; the SaaS sandboxes Orca already supports
  (Vercel Sandbox in Sydney, Modal `au`, Fly) are a same-day demo at most.

**Owner calls (section 6):** the subscription and a dev baseline of about US$2,000 a month at list
price; the warm-pool size; the idle policy; the isolation statement customers see.

## 1a. Revised for the owner's second direction

Relaunch, 2026-09-30, after the first run stopped at 09:47. The core answer stands. What changed:

| # | What | First run | Now | Why |
|---|---|---|---|---|
| 1 | How the cockpit reaches a cloud session | A thin `remotebe` in our desktop, over an in-cluster control plane | **Orca's per-workspace environment recipe** (`orca.yaml` → `environmentRecipes`). Its `create` script asks our control plane for a sandbox; at bind the sandbox starts Orca's headless runtime (`orca serve`) and hands back the pairing code; Orca streams terminals over its own end-to-end-encrypted WebSocket | The owner picked Orca. The recipe is Orca's documented seam for "a cloud sandbox, VM, or local Docker container" per worktree |
| 2 | Days to the first cloud session | 7 (5 to 9) | **8 (6 to 10)** | +0.5 to package Orca's runtime into the image; +0.5 for the Orca recipe. The `remotebe` share of the old glue becomes an authenticated `wss://` route to each sandbox |
| 3 | Panacea | The model only (`OD-13` then forbade copying) | **Used where it is the better answer** (4.15): its hardened pod spec, deny-egress policy, package mirrors, tenant network baseline and Terraform are copied with attribution; its Keycloak realm, LiteLLM gateway and Trivy scans are called, not rebuilt; `panacea-mcp` ships in the image so an unmodified agent can deploy (00-panacea §4). **It is not the sandbox host** | Owner: "Panacea is fine to use majority of"; snippets now allowed |
| 4 | Coolify, Supabase | Not considered | **Not used for this capability** (4.16): neither runs untrusted code in a microVM, keeps a warm pool, or hands out a PTY | Allowed by the owner; on merit they belong to deploy (06) and the Hub, not here |
| 5 | SaaS sandboxes Orca already supports | Modal: "reference, a demo fallback" | Vercel Sandbox (now in Sydney), Modal (`au`) and Fly: **reference, the fastest Orca demo, not the product** (4.17) | Orca wires them today; none is Azure or our environment (`OD-18`, `OD-5`) |
| 6 | A unit error | Phase 8 savings in "plan-days" | **Plan-hours.** The phase 8 task table's column is `Hours`; its 98.5 total is hours | The saving is 36 plan-hours to about 14, not 36 days |

**Unchanged:** the adopt (AKS Pod Sandboxing + `agent-sandbox` 1.0 in Australia East), the ACI
fallback and its switch rule, every reject (Daytona, Coder, E2B, Codespaces, Ona, DevPod), and
owner calls 1 to 5 (call 4 gains two disclosure lines; calls 6 and 7 are new options, both not
recommended).

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

**Under Orca (section 1a) the bar rises in two places.** The image must also run Orca's headless
runtime (an Electron AppImage under `xvfb`, glibc 2.31 or later), which rules out
code-interpreter APIs more firmly still (M1). And the stream must reach the user's desktop, not
only our control plane, through an authenticated `wss://` route to each sandbox (M3).

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

"Days" means days to the first real cloud session from the cockpit, including the glue every
option needs (section 5). "+ proc." means a new-vendor procurement comes first. **Revision:**
under Orca the shared glue is 3.5 days, not 3, and every image carries Orca's runtime (+0.5), so
add 1 day to each first-run figure; the two rows we would act on (AKS, ACI) show the new figure.

| Candidate | Licence | Deploys as | AU | Isolation | Stream into it | Warm start | Days | Verdict |
|---|---|---|---|---|---|---|---|---|
| **AKS Pod Sandboxing + `agent-sandbox` 1.0** | Apache-2.0 + Azure service | Our AKS; a customer's AKS later | Australia East | Kata microVM on Hyper-V | Our `sandboxd` on the pod IP; `orca serve` through a `wss://` route | `SandboxWarmPool`; "a couple of seconds" *(upstream)* | **8** | **adopt** |
| ACI + standby pools | Azure service | ARM in any subscription | Yes | Hypervisor per container group | Our `sandboxd` on a private IP; `orca serve` the same way | Standby pool; latency unpublished | 8 | integrate (fallback only) |
| ACA dynamic sessions (custom container) | Azure service | ARM in any subscription | East and Southeast | Hyper-V per session | HTTP through the pool endpoint; WebSocket undocumented | Ready sessions, "milliseconds" *(upstream)* | 9 | reference |
| Northflank | Commercial | SaaS, or BYOC data plane in our Azure | BYOC in AU East | Kata / Firecracker / gVisor; microVM on Azure undocumented | REST API | "under a second" *(upstream)* | 8 + proc. | reference |
| Runloop | Commercial | SaaS, or VPC deploy (Azure listed) | Not documented | microVM | Python / TS SDKs | "a few seconds" *(upstream)* | 8 + proc. | reference |
| Modal | Commercial SaaS; open SDKs | SaaS only | `au` pin at 1.75x | gVisor (not hardware) | Go SDK: PTY, port tunnels; an Orca recipe provider today | Fast *(upstream)* | 4 + proc. | reference |
| Vercel Sandbox *(added in revision)* | Commercial SaaS | Vercel's cloud only | `syd1` (Sydney) | Firecracker microVM | Orca recipe provider today; TS / Python SDKs | Snapshots, persistent sandboxes | 1 + proc. (demo only) | reference |
| Panacea: Forge preview pods on its AKS *(added)* | KPMG's own code | Panacea's estate (Australia Central; an Australia East estate in cutover) | Yes | Shared-kernel `runc`, well hardened; no Kata pool | Forge's own proxy and pod exec | None: a pod per session, capped, reaped on a TTL | — | integrate as snippets and services; reject as host |
| Coolify (+ Supabase) *(added)* | Apache-2.0 | Self-hosted PaaS: Docker on our servers over SSH | Ours | Docker (shared kernel) | None built in | None | — | reject for this capability |
| VMSS Flex + standby pools *(added)* | Azure service | ARM in any subscription | Yes | A full VM per session | SSH (Orca's SSH mode) or our `sandboxd` | Standby pool, running or deallocated | 8 | reference |
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
  (6) *(revision)* Falco, Panacea's runtime detector, watches the node's kernel, so it cannot see
  inside a Kata guest either (inferred from Kata's separate guest kernel; not verified).
- **Under Orca and Panacea (revision).** Orca's pairing needs a route from the desktop, so the
  router question in gap (5) becomes a requirement: one authenticated `wss://` path per sandbox
  (the router with its scoped tokens, or our ingress), TLS at the edge, and the pairing code kept
  out of every access log (Orca: "do not put it in proxy access logs"). The `SandboxTemplate`
  takes Panacea's Forge pod security context, and the egress step takes its deny-egress policy
  and package mirrors (4.15).
- **Integration effort: 8 days** (range 6 to 10) to the first real cloud session from Orca,
  itemised in section 5. 4.5 of them are this option's; 3.5 are glue every option needs.
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
- **Integration effort: 8 days** — a new `acibe` over ARM (2), late-bind via config map (0.5),
  Orca's runtime in the image (0.5), the explicit-proxy egress (1), smoke (0.5), plus the 3.5
  days of shared glue (section 5). *(Revision: was 7 before Orca.)*
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
  **Verdict: reference** — a demo fallback, not the product's home. *(Revision: Orca already
  lists Modal as a recipe provider, so an Orca-on-Modal demo is shorter still; it stays outside
  Azure and our tenancy. See 4.17.)*

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
| AWS Bedrock AgentCore Runtime | microVM-per-session service | Wrong cloud for `OD-18`; not researched further (Fly.io Machines moved to 4.17: Orca supports it) |
| `coder/agentapi` | Chat API synthesised from a terminal screen | Archived; and reading the rendered screen is what `OD-8` rules out |

### 4.15 Panacea, the owner's platform — **INTEGRATE its code and services; REJECT as the host** *(added in revision)*

- **What it has here, read-only in `Projects/Panacea` today.** No product for running agent CLIs.
  - Forge's preview pods (`services/forge-orchestrator/preview/manager.go:81-236`): one pod per
    Forge session for a Next or Vite dev server; UID 1001, read-only root, all capabilities
    dropped, no privilege escalation, `RuntimeDefault` seccomp, `emptyDir` volumes, a pod cap and
    a TTL reaper (`:581`). No `runtimeClassName` anywhere in the service: shared-kernel `runc`.
  - The `preview-deny-egress` NetworkPolicy and an in-cluster npm and PyPI mirror (Verdaccio on
    4873, proxpi on 5000) (`deploy/forge/deploy-forge-orchestrator.yaml:61-101`), so untrusted
    AI-written code installs packages without the internet (ISM-1234).
  - The tenant NetworkPolicy baseline (`services/panacea-api/clients/tenant_ensure.go:20-190`).
  - The estate's Terraform (`deploy/*.tf`): Azure Firewall, private endpoints, CMK, diagnostics,
    Defender and Sentinel. Its node pools are system, workload, build, CI and elastic; **none is
    Kata** (no `workload_runtime` in `deploy/*.tf`). The live estate is in Australia Central
    (`deploy/apply.sh:38`); a hardened Australia East estate is being cut over
    (`deploy/environments/panacea-aue.env:3-22`).
  - panacea-api's "TTL sandboxes" (`services/panacea-api/models/instance.go:70-87`) are expiring
    copies of a *release*, a deploy concept (06), not a place to run an agent.
- **As a sandbox host:** M2 no (no Kata pool), M4 no (no pool), M8 no (`emptyDir` only), S1 no
  (neither panacea-api nor panacea-mcp exposes a sandbox to another product).
- **Why not add a Kata pool to Panacea's cluster anyway:** `OD-5` (our environment first);
  untrusted agents beside live tenant workloads during a cutover whose bar is "no lost features,
  no data loss"; our uptime tied to Panacea's; and it is an edit in another session's repo.
- **What we take**, attributed in each file's header (`OD-13` relaxed): the pod security context
  into our `SandboxTemplate`; the deny-egress policy and mirror pattern into the egress step; the
  tenant baseline into each user's namespace; the Firewall, private-endpoint, CMK and diagnostics
  Terraform into phase 8's modules (tasks 03 to 07). **What we call:** the Keycloak realm brokering
  Entra (`OD-30`), the LiteLLM gateway's per-session token (`OD-19`, `OD-21`), Trivy on the image.
- **Effort:** inside the 8 days (section 5); it shortens phase 8 group A rather than adding days.
  **Verdict: integrate** the code and services; **reject** as the host.

### 4.16 Coolify and Supabase — **REJECT for this capability** *(added in revision)*

- **Coolify** (Apache-2.0, 62.4k stars): "an open-source, self-hostable PaaS alternative to
  Vercel, Heroku & Netlify" that runs Docker containers on our servers over SSH. No microVM (M2),
  no warm pool (M4), no per-session sandbox API (S1). It is a deploy tool (06).
- **Supabase** (Apache-2.0): Postgres, auth, storage, realtime and Deno edge functions. Nowhere to
  run a CLI on a PTY. A candidate for the Hub's store, not for sandboxes.
- **Trade-off in one line:** both host apps well; neither isolates an untrusted agent, so using
  them here means building the sandbox layer ourselves on top of them.

### 4.17 The SaaS sandboxes Orca already supports — **REFERENCE (demo only)** *(added in revision)*

- Orca's recipe guide: providers "people wire today include Vercel Sandbox, Fly, Modal, plain SSH
  hosts, and local Docker".
- **Vercel Sandbox:** a Firecracker microVM per sandbox; 19 regions including **`syd1` (Sydney)**
  (Vercel docs, updated 2026-09-22); snapshots, forks, persistent sandboxes, drives in beta;
  commercial, usage-priced. **Modal:** 4.7. **Fly Machines:** Firecracker microVMs with a Sydney
  region *(upstream, not re-verified)*; commercial.
- **Why not:** none is Azure or our environment (`OD-18`, `OD-5`) or can go into a customer's
  tenancy, and each is a new vendor holding client code (procurement, a TSRA, retention terms).
- **Use, at most:** an internal demo on non-client code while Kata capacity is confirmed, and
  only if the owner accepts it (section 6, call 6). About 1 day plus procurement.

### 4.18 VMSS Flex + standby pools — **REFERENCE** *(added in revision)*

- A full Azure VM per session from a scale set whose standby pool keeps VMs running or
  deallocated (GA 2024-11-19). M2 yes (a whole VM), M8 yes (a managed disk each), and Orca's SSH
  mode connects without exposing `orca serve`.
- **Why not first:** an image bump is a VM image build, not a container tag (S4); one session per
  VM; a deallocated standby VM still has to boot (latency not measured); claim and release are
  ours to write, as with ACI. About 8 days. The natural home if a customer demands a VM per session.

## 5. Recommendation

**Adopt AKS Pod Sandboxing (Microsoft-managed Kata) plus `kubernetes-sigs/agent-sandbox` v1.0.4
(Apache-2.0) as the `katabe` backend, in our own AKS in Australia East, hardened with Panacea's
code and reached from Orca through one environment recipe. 8 days to the first real Claude Code
session from Orca in a Kata microVM (range 6 to 10).** *(Revised: was 7 days, reached from our
own desktop through `remotebe`.)*

Why this and not a bought service, and not Panacea's cluster:

1. It is the only candidate that meets all eight musts today (section 2), with a permissive licence.
2. Microsoft runs the hard part (the Kata runtime, the guest kernel, the hypervisor) and
   Kubernetes SIG Apps maintains the lifecycle we were about to write (pool, claim, pause,
   resume, persistent storage, scheduled deletion). We write glue only.
3. It reuses what exists: `katabe` already targets Kubernetes behind the unchanged seven-method
   `Backend`; phase 8 is already written for AKS; `OD-18` chose Azure for this reason.
4. It goes into a customer's own AKS unchanged, which a SaaS (Modal, Runloop, Northflank's
   control plane, Vercel Sandbox) cannot, and it needs no new-vendor procurement, which would
   cost weeks.
5. *(Revision)* Panacea has no sandbox to adopt (4.15), so the best complete answer is this one
   built with Panacea's parts: its hardening, egress pattern, mirrors and Terraform are copied,
   and its Keycloak realm, LiteLLM gateway and Trivy are called. Hosting on Panacea's cluster
   would save about a day and US$1,250 a month, against `OD-5` and its cutover risk.

### The 8 days *(revised: was 7)*

| Day | Step | Whose |
|---|---|---|
| 0.5 | AKS in Australia East (Overlay + Cilium, Entra RBAC), an Azure Linux Kata pool on a nested-virt Dsv5 size with the cluster autoscaler, ACR. A scripted `az` runbook now, taking the Firewall and private-endpoint settings from Panacea's `deploy/firewall.tf` and `deploy/private-endpoints.tf`; phase 8 task 06's OpenTofu module codifies it after | this option |
| 0.5 | Install agent-sandbox v1.0.4; one `SandboxTemplate` (our image, `kata-vm-isolation`, explicit requests, no service-account token, Panacea's Forge security context: UID 1001, read-only root, drop ALL, seccomp; a PVC); one `SandboxWarmPool`; time 10 parallel claims. The spike the 2026-09-28 study asked for, shrunk because upstream ships the AKS manifests | this option |
| 1 | `katabe`: `Create` = a `SandboxClaim` through the Go SDK; `Get`/`List` from `Sandbox` status; `Destroy` deletes the claim; `Attach` dials the pod IP with the token | this option |
| 1 | Image and late-bind: add Orca's Linux AppImage and its `xvfb`/GTK libraries to the sandbox image (never run it at build time), and Panacea's single-file `panacea-mcp` bundle (00-panacea §4). A warm pod starts unbound; the first `POST /bind`, reachable only from the control plane by NetworkPolicy, sets the per-session gateway token (`OD-19`) and the session's project-scoped, expiring `pnc_` token (minted as 00-panacea §5 says), then starts `orca serve --json` with this sandbox's pairing address (or, off Orca, `sandboxd`'s PTY) and returns the ready block | this option |
| 1 | Egress default-deny; allow DNS, the LiteLLM gateway, the git hosts (`OD-4`), Panacea's API for `panacea-mcp` (Panacea's edge admits only KPMG AU corporate egress, `deploy/main.tf:580-602`, ISM-1416, so our cluster's egress IP must join `edge_allowed_ip_ranges`: a request to the Panacea side), and an in-cluster npm/PyPI mirror on Panacea's Verdaccio + proxpi pattern; Cilium FQDN policy, or phase 8 task 07's Azure Firewall; ingress only from the control plane and the pairing route | this option |
| 1.5 | Worktree into the sandbox (seam S8.3): the control plane's git runner clones onto the sandbox's PVC before bind with `OD-22`'s read-only credential, so none enters the sandbox; work comes back under `refs/panacea/lanes/…` (`OD-2`, phase 3); pushes use the signed-in person's credential, outside the sandbox | shared glue |
| 1.5 | The control plane in the cluster behind the Keycloak broker (`OD-30`, Panacea's realm), plus one authenticated `wss://` route per sandbox for Orca's pairing: TLS at the edge, the pairing code never logged (the first slice of phase 8 tasks 17 and 18) | shared glue |
| 0.5 | The Orca recipe: an `environmentRecipes` entry and `create`/`suspend`/`resume`/`destroy` scripts that call the control plane with the user's token and print Orca's JSON. It replaces the thin `remotebe`; counted here, so 02 should not count it again | shared glue |
| 0.5 | Smoke: 10 parallel Claude Code sessions from Orca; pair, type, suspend, resume, destroy; record claim p50/p95 and pool refill under a burst | this option |

### What it replaces in our plan

- **Phase 8** (38 task files, about 98.5 plan-hours by the plan's own estimates, whose column is
  `Hours`; the first run said "plan-days" in error; the stream reports 5 of 35 done).
  **Removed:** task 12 (warm capacity: balloons and pre-pull, 2 hours) and task 19 (broker
  machines: wake, sleep, idle sleeper, caps, 4 hours), which become `SandboxWarmPool` and the
  `Sandbox` lifecycle. **Materially reduced:** 14 (pod hardening, 2.5), 15 (machine volumes, 3),
  23 (per-machine backends and reaper, 3.5) and 32 (Kata proving checklist, 2). **Reduced
  further if** the plan's machine pod with a shared disk becomes one `Sandbox` per session with
  its own retained PVC, which `OD-2`'s one clone per worktree already implies: 20, 21, 21b, 24a,
  24b, 25 and 25b (19 plan-hours) shrink by about half. In all, 13 tasks and about 36 plan-hours
  become about 14. That last step is a plan edit that needs its own review; the word "machine"
  and what the user sees do not change.
- *(Revision)* **Under Orca**, if 02 replaces our desktop with it: task 22 (`remotebe`, 3) and
  task 26 (the cockpit shows cloud machines, 3) become the recipe, and group F's other tasks
  follow 02's lane ruling. **Under Panacea's Terraform**, group A (tasks 03 to 07, 14.5 hours)
  starts from Panacea's `deploy/*.tf` instead of a blank file; it shrinks, by an amount not
  estimated here.
- **The critical path.** The first cloud session needs only the 8-day slice above. The rest of
  phase 8 (OpenTofu stacks 01–10, charts 11 and 13, identity and broker 16–18, 22, UI 26,
  pipelines and runbooks 27–31, close-out 33) stays and follows, off the critical path.
- **Phase 9** has no plan yet. The Kata runtime, guest kernel and hypervisor patching become
  Microsoft's; what stays ours is admission policy (Kyverno, per the 2026-09-28 study), network
  policy and the ISM evidence.
- **Capacity.** `PROGRAM.md` open risk 1 (warm-pool sizing unproven) becomes a pool size, an HPA
  on the pool's scale subresource and the cluster autoscaler on the Kata pool, measured by the
  day-8 smoke instead of a hand-built pool controller.

### What we still build (the glue) *(revised)*

1. The `katabe` claim client and the late-bind that starts `orca serve` (or `sandboxd`) with a
   fresh identity and the per-session gateway token.
2. The control plane's clone onto the sandbox's disk and the lane fetch-back; no git credential
   inside the sandbox.
3. The in-cluster control plane behind Keycloak, the per-sandbox `wss://` pairing route, and the
   Orca recipe's four scripts (replacing `remotebe`).
4. The egress allowlist with the npm/PyPI mirror, the ingress policy and the template
   hardening, each starting from Panacea's code with attribution.
5. The canary-gated image bump that tracks the latest Claude Code CLI (`OD-20`) **and pins
   Orca's runtime to the desktop's version** (Orca releases almost daily, per 02), rolling the template and
   refilling the pool; and our own burst load driver (upstream has none).

### Risks, and the rule that switches to the fallback

- `katabe` has never run against Azure; Kata on AKS is new to us.
- Nested-virt Dsv5 capacity and quota in Australia East are unconfirmed for our subscription.
- The agent-sandbox CRDs are `v1beta1` and a month past 1.0; pin the version.
- Defender for Containers cannot assess Kata pods (a recorded compliance gap); AKS
  node auto-provisioning does not support Kata.
- *(Revision, Orca)* Orca's recipe guide documents bash scripts on macOS and Linux; how they run
  on a Windows desktop is not documented. If they do not, the control plane opens Orca's
  `orca://pair?code=…` link instead (Orca's remote-server mode), which costs the per-worktree
  lifecycle buttons, not the session.
- *(Revision, Orca)* Orca's runtime is Electron: its Chromium sandbox needs a non-root user and
  may need user namespaces inside the Kata guest; `--no-sandbox` would be a downgrade inside the
  microVM, not a breach of it. Its pairing has no documented revocation, so destroying the
  sandbox is the revocation. The upstream CLI for recipe environments (PR 22574) was closed
  unmerged on 2026-09-24; the desktop's recipe service is what we rely on.
- *(Revision, Panacea)* Reaching Panacea from the sandbox (`panacea-mcp`, and Panacea's LiteLLM
  if the gateway study picks it) needs our egress IP on Panacea's edge allowlist
  (`deploy/main.tf:580-602`). Until then, day 8's smoke runs without `panacea-mcp`.
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
   D8s_v5), so cost does not separate the adopt option from the fallback. *(Revision: Orca's
   runtime, Electron under `xvfb`, adds memory to every sandbox; not measured, checked on day 8.)*
2. **Warm-pool size (money against what users feel).** Each warm sandbox is a running microVM
   waiting for a user. The recommendation starts with 3 per image during business hours and
   scales down at night; a bigger pool is a faster first keystroke at a higher idle bill.
3. **Idle policy (customer-visible).** How long an idle session keeps running before it is paused.
   Pausing keeps the disk; resuming is a Kata cold boot of seconds rather than an instant attach.
4. **What customers are told (customer-visible).** The isolation badge (seam S8.5) would say, in
   substance, "each session runs in its own Hyper-V-isolated microVM in Australia East". The owner
   should also approve how the known gap is disclosed: Microsoft Defender for Containers cannot
   assess Kata pods, and *(revision)* Falco cannot see inside them either (inferred). Under Orca,
   customers would also be told that the terminal travels from the sandbox to the person's
   desktop over Orca's end-to-end-encrypted connection, through our TLS edge in Australia East.

**Options offered, not recommended (the owner may still choose one):**

5. **Buy instead of run (money).** Paying a vendor to operate the sandbox
   cluster (Northflank BYOC or Runloop VPC deploy) instead of running AKS ourselves. Against it
   today: new-vendor procurement takes weeks, microVM isolation on Azure is undocumented for
   both, and agent-sandbox costs nothing. Worth reopening only if running the cluster becomes the
   bottleneck.
6. *(Revision)* **A same-day demo on a SaaS sandbox (money; customer-visible if client code is
   used).** Orca already drives Vercel Sandbox (Sydney), Modal (`au`) and Fly, so an internal
   demo on non-client code could run the day a vendor account exists, while Kata capacity is
   confirmed. Against it: it sets aside `OD-18` and `OD-5` for as long as it runs, it is a new
   vendor (procurement, a TSRA), and none of it carries into the product.
7. *(Revision)* **Co-host on Panacea's estate (money).** A Kata pool and agent-sandbox in
   Panacea's cluster instead of a new subscription saves about US$1,250 a month at list (system
   nodes and firewall) and the wait for a subscription. Against it: it reverses `OD-5`, puts
   untrusted agents beside live tenant workloads during Panacea's cutover, and is an edit in
   another session's repo. Panacea's live estate is in Australia Central, whose nested-virt
   capacity was not checked.

## 7. What this study could not verify

Nothing was installed, deployed or run. Every capability claim is from documentation, a
project README, a vendor page or the GitHub API, read on 2026-09-30.

**Would change the recommendation if wrong (checked by the day-1 and day-8 steps):**

1. Claim latency on AKS with our image. "A couple of seconds" is upstream's claim on its own image.
2. Nested-virt Dsv5 capacity and quota in Australia East for our subscription.
3. That a pooled `Sandbox` can carry a PVC template and that pause and resume work on Kata. The
   README claims persistent storage and pause/resume in general, not on Kata specifically.
4. That the agent-sandbox Go SDK's pod-IP connectivity and the router's scoped tokens behave as
   the v1.0.2 release notes describe.
5. *(Revision)* That `orca serve` runs inside a Kata pod: the AppImage under `xvfb`, non-root,
   with its Chromium sandbox. Its docs cover an Ubuntu host, not a container or a microVM.
6. *(Revision)* That Orca's recipe scripts run on our users' Windows desktops (documented for
   macOS and Linux), and that its pairing works through a per-sandbox reverse-proxy path (its
   docs accept "a complete reverse proxy URL such as https://orca.example.com/runtime").

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
- *(Revision)* During the relaunch the GitHub API refused every call (HTTP 403, rate limit on
  the shared egress address), so Orca's facts come from its docs and README pages, and its star
  count and release cadence are the cockpit study's (02). agent-sandbox was not re-checked for a
  release after v1.0.4.
- *(Revision)* Panacea: nested-virt SKU capacity in Australia Central; whether Falco can observe
  a Kata guest (inferred no); the licences of the Verdaccio and proxpi images its mirror uses
  (not checked; both are third-party). `00-panacea.md` was still a skeleton when this revision
  was written, so the Panacea facts here are this study's own greps (4.15).
- *(Revision)* Vercel Sandbox's retention terms and SSO; Fly's Sydney region (not re-verified);
  VMSS standby-pool claim latency and Australia East support.

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

**Added in the revision (2026-09-30).**
Orca: <https://www.onorca.dev/docs/ways-to-run> · <https://github.com/stablyai/orca> (README) ·
<https://github.com/stablyai/orca/blob/main/skill-guides/orca-per-workspace-env.md> ·
<https://github.com/stablyai/orca/blob/main/docs/reference/headless-linux-server.md> ·
<https://github.com/stablyai/orca/pull/22574>.
Others: <https://vercel.com/docs/sandbox/concepts/regions> ·
<https://github.com/coollabsio/coolify> ·
<https://techcommunity.microsoft.com/blog/azurecompute/general-availability-standby-pools-for-virtual-machine-scale-sets-with-flexible-/4287343>.
Panacea, read-only, `Projects/Panacea` working tree today:
`services/forge-orchestrator/preview/manager.go` (lines 1–250, 581) ·
`deploy/forge/deploy-forge-orchestrator.yaml:61-101` ·
`services/panacea-api/clients/tenant_ensure.go:20-190` ·
`services/panacea-api/models/instance.go:70-87` · `services/panacea-api/routes/routes.go:83` ·
`deploy/*.tf` (node pools, `firewall.tf`, `private-endpoints.tf`) · `deploy/main.tf:580-602`
(the edge allowlist) · `deploy/apply.sh:38` · `deploy/environments/panacea-aue.env:3-22`.
`00-panacea.md` §4 (`panacea-mcp` in every sandbox, a scoped `pnc_` token).
This repo: the phase 8 task table's `Hours` column
(`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:558-633`);
`OD-22` in the owner decisions (line 150); `02-fleet-cockpit.md` (Orca's figures, C6).
