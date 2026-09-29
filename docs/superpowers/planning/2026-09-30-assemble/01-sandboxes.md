# 01 — Cloud agent machines: buy, adopt or integrate instead of build

**Status:** IN PROGRESS (skeleton). Sections fill in one at a time.
**Date:** 2026-09-30. **Capability:** isolated per-session sandboxes that run unmodified
coding-agent CLIs (Claude Code, Codex, Gemini CLI) with a real PTY, warm pools, microVM-class
isolation, on Azure or bought with an AU region, driven by API as a `Backend` of our Go control plane.
**Owner direction (2026-09-30):** third-party first, the tool ASAP, days not weeks.

## 1. Answer first

_TBD — the one recommendation, its integration days, and what it replaces._

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

_TBD — one row per candidate: licence, deploy, AU, isolation, PTY, warm start, verdict._

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
  deletion, pause and resume), `SandboxTemplate`, `SandboxWarmPool` (pre-created sandboxes; has a
  scale subresource, so a plain HPA drives it) and `SandboxClaim` (adopt one from the pool). It
  ships a **Go SDK** (`sigs.k8s.io/agent-sandbox/clients/go/sandbox`) and an optional
  `sandbox-router` reverse proxy.
- **Licence.** Apache-2.0. AKS is an Azure service.
- **Maturity.** 4,086 stars; **v1.0.4 released 2026-09-24**; pushed 2026-09-29. Upstream ships
  two AKS-specific examples, `kata-aks-sandbox` and a per-owner `kata-aks` warm pool, last edited
  2026-08-14. Recent releases add warm-pool refill shaping (`replenish-delay`, `max-refill-rate`).
- **Deploy.** Our AKS cluster in **Australia East**; later the same manifests into a customer's AKS.
  Kata pool rules: Azure Linux, a Gen2 VM size with nested virtualisation (Dsv5 class), Kubernetes
  ≥ 1.27. Install is two `kubectl apply` manifests or the Helm chart.
- **Enterprise fit.** Entra through AKS Entra RBAC; audit through AKS diagnostic logs to Log
  Analytics; residency is wherever the cluster is. Egress default-deny with an allowlist is
  ordinary Kubernetes: Cilium network policy plus the Azure Firewall already in the phase 8 plan.
- **Against the bar.** M1 yes (any image; our `sandboxd` owns the PTY). M2 yes (Kata microVM). M3
  yes: the control plane in the cluster dials the pod IP directly, or goes through `sandbox-router`
  (`kubectl port-forward` cannot enter a Kata guest). M4 yes: *(upstream)* claims from a
  `kata-aks-warmpool` become Ready "in a couple of seconds". M5 yes. M6 yes. M7 yes. M8 yes
  (a PVC per sandbox or per user; pause and resume). S1 yes (Go SDK). S5 no memory snapshot.
- **Gaps.** (1) A warm pod already runs when it is claimed, so per-session env (the attach token,
  the per-session gateway token) cannot be set at pod creation: `sandboxd` needs a **late-bind**
  call. This is true of every warm-pool option in this study. (2) Claim latency on our image is
  unmeasured. (3) Defender for Containers cannot assess Kata pods (already a recorded compliance
  gap). (4) AKS node auto-provisioning (Karpenter) does not support Kata, so the Kata pool scales
  with the cluster autoscaler. (5) The upstream router example runs with authentication off.
- **Integration effort: 6 days** (range 5 to 8), itemised in section 5.
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
- **Integration effort:** 3 to 5 days if WebSocket works through the pool endpoint, plus 2 for a
  reverse-dial fallback, plus whatever the egress gap costs to close.
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
- **Integration effort: 5 days** — a new `acibe` over ARM (2), late-bind via config map (0.5),
  the explicit-proxy egress (1), clone-in-sandbox (1, shared with every option), smoke (0.5).
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
  Firecracker infrastructure onto nested-virt VMs: weeks, not days. No Go SDK.
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

_TBD — solution, days, what it replaces (phases and task counts), glue we still build, risks._

## 6. Calls only the owner can make

_TBD_

## 7. What this study could not verify

_TBD_

## 8. Sources

_TBD_
