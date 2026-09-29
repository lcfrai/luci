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

## 5. Recommendation

_TBD — solution, days, what it replaces (phases and task counts), glue we still build, risks._

## 6. Calls only the owner can make

_TBD_

## 7. What this study could not verify

_TBD_

## 8. Sources

_TBD_
