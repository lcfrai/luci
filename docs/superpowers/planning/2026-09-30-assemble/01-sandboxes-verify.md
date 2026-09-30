# 01 — Cloud agent machines: the sceptic's verification

**Status:** IN PROGRESS (skeleton). Sections are filled one at a time and committed after each.
**Date:** 2026-09-30. **Verifies:** `01-sandboxes.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation and its two best alternatives at source:
licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run.

## 0. Verdict first

*(to be filled)*

## 1. What was checked, and how

All reads on 2026-09-30. Nothing installed, deployed or run; no Azure subscription touched.

- **The report under test:** `01-sandboxes.md`, all 710 lines.
- **At source, GitHub API** (it answered this time, no 403): repo metadata, `/license` and
  releases for `kubernetes-sigs/agent-sandbox`; licences for `kata-containers/kata-containers`,
  `stablyai/orca`, `verdaccio/verdaccio`, `EpicWink/proxpi`; agent-sandbox's `go.mod` and CRD
  types at tag `v1.0.4` (`api/v1beta1/sandbox_types.go`,
  `extensions/api/v1beta1/{sandboxclaim,sandboxtemplate,sandboxwarmpool}_types.go`), its Go
  SDK README and its AKS example page.
- **Vendor docs:** Microsoft Learn `use-pod-sandboxing` and `considerations-pod-sandboxing`
  (both updated 2026-09-18), the ACI standby-pool and ACA dynamic-sessions pages (sections 3
  and 4), the Azure Retail Prices API (Australia East, Linux, pay-as-you-go).
- **Our code:** `control-plane/internal/backend/katabe/kata.go`,
  `control-plane/internal/sandbox/manager.go`, `control-plane/go.mod`, `infra/terraform/kata.tf`;
  the phase 8 plan by grep.
- **Panacea, read-only greps:** `services/forge-orchestrator/preview/manager.go`,
  `deploy/forge/deploy-forge-orchestrator.yaml`, `deploy/*.tf`,
  `docs/superpowers/plans/2026-06-04-magic-roadmap.md`.

## 2. The recommendation: AKS Pod Sandboxing (Kata) + `kubernetes-sigs/agent-sandbox`

**Verdict: UPHELD, with six corrections:** 2.3 rows 1 (a bug in our own code), 5, 6, 8, 9
and 15, plus the unlisted limits and costs in 2.4 and 2.6. None of them changes the choice.

### 2.1 Licence at source

- **agent-sandbox:** Apache-2.0. `GET /repos/kubernetes-sigs/agent-sandbox/license` returns
  `LICENSE`, SPDX `Apache-2.0`, text "Apache License, Version 2.0, January 2004". Holds.
- **AKS Pod Sandboxing:** an Azure feature with no fee of its own; it bills as the node VMs.
  The Kata runtime inside it is `kata-containers/kata-containers`, Apache-2.0 (GitHub API).
  Microsoft ships it on the node; we redistribute nothing.

### 2.2 Maintenance

- 4,087 stars, 538 forks, 202 open issues; created 2025-08-12; last push 2026-09-29.
- Releases (GitHub API): v0.5.4 2026-07-30, v1.0.0 08-28, v1.0.1 09-03, v1.0.2 09-11, v1.0.3
  09-17, v1.0.4 09-24. Weekly, as claimed.
- **Stability signal the report understates:** v1.0.2, a patch release, carried two breaking
  changes (the router's `authz.Authorizer` signature, and the router's move to the
  `agent-sandbox-system` namespace). Pin the exact tag and read every release note on bump.
- **AKS is a second-class platform upstream.** The flagship blueprints are GKE
  (`openclaw-fleet-gke`, v1.0.3). The AKS example was last modified 2026-08-14 and already
  disagrees with Microsoft: it creates the pool with `--workload-runtime KataMshvVmIsolation`,
  while Microsoft's page (2026-09-18) says `KataVmIsolation`. Follow Microsoft.

### 2.3 Claims checked against the vendor's own docs and source

| # | Claim in `01-sandboxes.md` | Verdict | Evidence |
|---|---|---|---|
| 1 | A Hyper-V-backed Kata microVM per pod; `KataVmIsolation`; RuntimeClass `kata-vm-isolation` | **Holds** for Azure. **Our code is wrong:** `control-plane/internal/sandbox/manager.go:20` hardcodes `kata-mshv-vm-isolation` (test at `manager_test.go:33`; `infra/terraform/kata.tf:13` asserts the same). On a `KataVmIsolation` pool every `katabe` pod would fail to schedule | Microsoft: "`runtimeClassName: kata-vm-isolation`"; Hyper-V + Cloud Hypervisor + Azure Linux. Phase 8 decision P8-D15 already flags this as UNVERIFIED; it is now verified |
| 2 | Nested-virt Dsv5 nodes | **Holds** | Microsoft: "a generation 2 VM size that supports nested virtualization"; upstream: "D-series v3/v4/v5 work; B-series ... do not". D8s_v5 is US$0.48/h in Australia East (Retail Prices API) |
| 3 | Warm claim "Ready in a couple of seconds" | **Upstream claim only** | AKS example page. Not measured on our image |
| 4 | `SandboxWarmPool` has a scale subresource, so an HPA drives it | **Holds** | `sandboxwarmpool_types.go:119`, `+kubebuilder:subresource:scale` |
| 5 | "Pause and resume" | **Partly.** It is `operatingMode: Suspended`, which "terminates the backing Pod but retains the Sandbox object and its volumes". Resume is a new Kata boot plus a disk re-attach, not from the pool, no memory kept | `sandbox_types.go:297-300` |
| 6 | M8: "a PVC per sandbox or per user" | **Partly.** A claim that brings its own volumes "forces a cold start because warm pool pods will not have these volumes". A warm claim gets the pool pod's own fresh, empty PVC; a per-user disk means a cold start | `sandboxclaim_types.go:134-135` |
| 7 | Per-session env needs a late-bind | **Holds, and is stronger:** claim `env` "means the Sandbox will always be cold-started"; the template default is `envVarsInjectionPolicy: Disallowed` | `sandboxclaim_types.go:127-129`; `sandboxtemplate_types.go:130-132` |
| 8 | M7: "Egress default-deny with an allowlist is ordinary Kubernetes" | **Does not hold as written.** Upstream's managed default is "Egress: Allow Public Internet only. Blocks internal IPs (RFC1918), Metadata Server", i.e. the internet is open. Its policy is "a restricted subset of the standard Kubernetes NetworkPolicySpec": IPs and ports, no FQDNs. An FQDN allowlist needs Azure Firewall application rules (costed) or ACNS FQDN filtering (a paid add-on, 2.4). The default ingress admits "only ... the Sandbox Router", so a pod-IP dial from our control plane needs an explicit ingress rule | `sandboxtemplate_types.go:84-113` (default at 105-106) |
| 9 | "Go SDK" (S1) | **Holds, but it is the wrong tool.** The SDK lives in the root module: `go 1.26.0`, `k8s.io/* v0.37.0`, `controller-runtime v0.25.0`, OTel, gRPC. Ours is `go 1.23`, `k8s.io/* v0.31.1`. Its extras (`Run`, `Read`, `Write`) target upstream's own `sandboxd`, which we do not run | agent-sandbox `go.mod` at v1.0.4; `clients/go/README.md`; `control-plane/go.mod` |
| 10 | Router scoped tokens (Ed25519, v1.0.2); SDK pod-IP connectivity (v1.0.2) | **Holds** | v1.0.2 notes: "Scoped-Token v2 Target Binding", `ConnectivityInClusterPodIP`. `ALLOW_UNAUTHENTICATED_ROUTER` is "demo-only" (example page) |
| 11 | Defender for Containers cannot assess Kata pods | **Holds** | Microsoft limitations list |
| 12 | Falco cannot see inside a Kata guest | **Holds as inference.** Third-party write-ups agree the host sees only the VMM process; no Falco primary doc found | cloudsecburrito.com "Runtime Security in Kata" |
| 13 | Managed node auto-provisioning does not support Kata | **Holds** for managed NAP (`Azure/AKS#5843`, open since 2026-06-26). Self-hosted `karpenter-provider-azure` merged `workloadRuntime` on 2026-09-04 (PR 1721) | GitHub API |
| 14 | `kubectl port-forward` cannot enter a Kata guest | **Holds** | Example page: "kubectl port-forward will not work for Kata pods" |
| 15 | The template takes "Panacea's Forge security context: UID 1001, read-only root, drop ALL, seccomp" | **Mostly already done, one line is a no-op.** Our `manager.go:151-208` already sets UID 1001, non-root, read-only root, no escalation, drop ALL. Only `seccompProfile: RuntimeDefault` (Forge `manager.go:117-118`, `:205-206`) is missing, and inside Kata it likely does nothing: Kata's upstream default is `disable_guest_seccomp = true`, "seccomp is not applied within the guest" | kata-containers `src/runtime/Makefile:236`; `configuration-clh.toml.in:390-393`. AKS's own Kata build not checked |

### 2.4 Paid-tier gates

- **agent-sandbox:** none. Router, warm pool, SDKs and blueprints are all in the Apache-2.0 tree.
- **AKS Pod Sandboxing:** no fee; the Kata node VMs are the cost. Density is worse than `runc`:
  each pod VM's memory is fixed at its limit plus overhead (4Gi costs 374Mi more), and
  fractional CPU limits round up to a whole vCPU (Microsoft considerations page).
- **Paid items the report did not count:**
  - **ACNS**, needed only for per-pod FQDN egress rules on Azure CNI powered by Cilium
    (Kubernetes 1.29 or later). Priced "per node per hour"; Microsoft's page did not render the
    figure; third-party write-ups say about US$0.025 a node-hour. Azure Firewall, already costed,
    does FQDN rules for the whole cluster instead, so ACNS is optional.
  - **AKS Standard tier** (the uptime SLA): US$0.10 per cluster-hour, about US$73 a month
    (Retail Prices API, Australia East). The report says it is excluded; production needs it.
  - **Defender for Containers** is billed per vCore and cannot assess Kata pods, so on the Kata
    pool it buys node-level coverage only.

### 2.5 Azure, AU region, Entra

- **Azure, Australia East:** yes. Microsoft's page names no region restriction, and the D-series
  v5 sizes are priced in `australiaeast`. Our quota: unverified (no subscription yet, section 7).
- **Entra:** AKS's Entra integration covers operators (a standard AKS feature, not re-read here).
  End users never touch AKS; they sign in to our control plane through Keycloak (`OD-30`). That
  is glue, not something this product supplies.
- **A customer's cloud:** the same manifests fit any AKS whose owner permits an Azure Linux Kata
  pool and the two prerequisites in 2.6 (items 1 and 2).

### 2.6 Limits the report did not list (Microsoft's page, 2026-09-18)

1. **A subscription prerequisite:** `Microsoft.Network/AllowBringYourOwnPublicIpAddress` "must be
   registered in your subscription ... required for Pod sandboxing workloads that use the Kata
   runtime". In a KPMG-managed or customer subscription this is a platform-team request. It goes
   on the day-0 checklist.
2. **"You can't enable FIPS or Trusted Launch on a node pool that uses Pod sandboxing."** A
   customer baseline or Azure Policy that mandates either needs an exemption for the Kata pool.
   Panacea's `deploy/*.tf` sets neither (no `fips_enabled`, no `secure_boot`), so copying its
   Terraform does not collide.
3. No Arm64; Azure CLI 2.80.0 or later.
4. The pod VM's memory is fixed when it starts (512Mi if no limit is set), with no ballooning.
   Orca's Electron runtime (section 6) has to fit inside that fixed size.
5. "Kata containers might not reach the IOPS performance limits that traditional containers can
   reach on Azure Files and high-performance local SSD." Git-heavy agents write to the PVC, so
   the day-8 smoke must time a clone and a build.

## 3. Alternative 1: Azure Container Instances + standby pools

**Verdict: the FALLBACK ruling is upheld.** Two claims weaken: the late-bind and the claim call
both rest on a preview API version, and the "+2 days to switch" is nearer 2 to 3.

- **Licence:** an Azure service under Microsoft's product terms; nothing to ship. The Go access is
  the Azure SDK for Go (the report says MIT; not re-read at source in this pass).
- **Maintenance:** Standby pools went GA on 2025-06-03 (Microsoft Tech Community post, "General
  Availability: Standby Pools for Azure Container Instances"). The overview page was updated
  2026-01-13; its only listed limitation is "isn't yet available in the Azure portal".

| # | Claim | Verdict | Evidence (Microsoft Learn) |
|---|---|---|---|
| 1 | Hypervisor isolation per container group | **Holds** | Overview (2026-07-26): "guarantees your application is as isolated in a container as it would be in a VM" |
| 2 | Claim from a pool of running groups; empty pool falls back to a cold create | **Holds.** Add: a claim is an ARM `PUT containerGroups` carrying `standByPoolProfile`, and every documented example uses `api-version=2024-05-01-preview`. Each claim counts against the default create limits (100 per 5 minutes, 300 per hour; raisable) | Standby-pool request page (2025-11-17); quota page |
| 3 | Claim latency | **Unpublished**, as the report says ("immediately pull one") | Standby-pool overview |
| 4 | Config maps are the late-bind channel | **Holds, on a preview API.** Values land at `/mnt/configmap/<container>/<key>` with no restart, but every example is `2024-05-01-preview`. **Simpler:** reuse the same `POST /bind` to `sandboxd` on the group's private IP that the AKS design uses; no config map needed | Config-map page (2025-11-21) |
| 5 | M7: in a VNet, "a NAT gateway ... is the only supported configuration for outbound connectivity" | **Quote holds**, but the same page lists "Integrate with Azure Firewall to identify outbound traffic" as a scenario. Microsoft contradicts itself; the report's explicit-proxy design is the safe reading. Ask Microsoft before relying on a UDR to a firewall | VNet concepts page (2026-07-26) |
| 6 | M8: Azure Files only | **Holds** (plus `emptyDir`, secret and `gitRepo` volumes) | Overview |
| 7 | Groups restart for maintenance and may change IP | **Holds** | Overview, "Considerations" |
| 8 | x64 only, images up to 15 GB, no privileged operations | **Holds** | Overview |
| 9 | Quota | **New detail:** defaults are 100 Standard cores and 100 container groups per region per subscription, both raisable. Standby groups are real groups, so they count (inferred). The 10-session smoke fits; team scale needs a quota request | Quota page (2026-07-26) |
| 10 | Size | **Holds** for 2 vCPU / 4 GB. Maximum 31 vCPU / 240 GB, but "more than 4 vCPUs and 16 GB" can fail on regional capacity | Quota page |

- **Paid tiers:** none gated. Per-second vCPU and GB billing; waiting standby groups are running
  groups, so presumably billed (no standby meter published; the report says the same).
- **Azure, AU, Entra:** Australia East; ARM under Azure RBAC and Entra; managed identity pulls
  from ACR. Private-registry pulls other than ACR are unsupported in a VNet.
- **Days:** 8 claimed from scratch. A new `acibe` on a preview API version with profile
  revisions is 2 to 3 days, not 2; the explicit-proxy egress is 1 to 1.5; and the shared glue
  grows exactly as it does for AKS (section 7). **About 10 to 11 from scratch.** The switch rule
  holds in shape: after a failed Kata day 1, moving costs **2 to 3 days**, not 2.

## 4. Alternative 2: Azure Container Apps dynamic sessions (custom container)

**Verdict: REFERENCE is upheld, and under Orca it is a firmer no.** Every failing must in the
report is confirmed on Microsoft's current pages, and the Orca pairing path adds one more.

- **Licence:** an Azure service; nothing to ship.
- **Maintenance:** active. The custom-container page was updated 2026-05-10, and container
  probes are new (they need `2025-02-02-preview` or later). The data-plane calls it documents
  (`getSession`, `listSessions`, `stopSession`) are all on `api-version=2025-02-02-preview`.

| # | Claim | Verdict | Evidence (Microsoft Learn) |
|---|---|---|---|
| 1 | A Hyper-V sandbox per session, sold for agents and dev environments | **Holds** | Custom-container page: "secure, sandboxed spaces with Hyper-V"; uses include "a cloud-based shell, AI agents, and development environments" |
| 2 | Allocation from ready sessions is near-instant | **Holds (upstream)** | Session-pool page: "subsecond session allocation times" |
| 3 | M7 fails: egress is on or off for the whole pool | **Holds** | `--network-status`: "`EgressDisabled` (default) and `EgressEnabled`"; no allowlist anywhere on the page |
| 4 | M3 unverified: HTTP only, WebSocket undocumented | **Holds, and fails outright for Orca.** "You invoke or communicate with each session using HTTP requests"; the custom paths list `POST`, `GET`. Every data-plane call also carries an Entra bearer token. Orca's desktop dials the pairing address directly and sends no Entra token, so `orca serve` in a session is unreachable without a relay we would build | Session-pool page (2026-05-10); `sessions-usage` authentication (report) |
| 5 | M8 fails: no volumes | **Holds.** No volume or storage setting on either page | Both pages |
| 6 | S2 weak: env is pool-wide | **Holds, and it is a leak:** `--env-vars` is set at pool create, and Microsoft warns "Assume the code is malicious and has full access to the container, including its environment variables, secrets, and files". Per-session tokens cannot be pool secrets | Session-pool page |
| 7 | Lifecycle `Timed` 300 to 3,600 s, or `OnContainerExit` with `maxAlivePeriodInSeconds` | **Holds.** The ceiling on `maxAlivePeriodInSeconds` is still not stated | Session-pool page |

- **Paid tiers:** custom-container pools need a workload-profiles environment; the report's
  "dedicated E16 instances" was not re-read here.
- **Azure, AU, Entra:** Australia East per the report; the data plane is Entra-token based, which
  suits a server caller and blocks Orca's desktop (row 4).
- **Days:** the report's 9 excludes a relay for Orca's `wss://` pairing. With it, about 11, and
  the M7 and M8 gaps stay open. Re-test when Microsoft ships per-pool egress rules, volumes and a
  documented WebSocket path.

## 5. Panacea as parts (snippets and services), not host

**Verdict: UPHELD.** Panacea has no agent sandbox to adopt; every fact the report cites was found
where it says. One correction lowers the value of a snippet the report counts on.

| # | Claim | Verdict | Evidence (read-only greps of `Projects/Panacea`) |
|---|---|---|---|
| 1 | Forge preview pods are shared-kernel `runc`, no Kata anywhere | **Holds.** No `runtimeClassName`, `kata` or `gvisor` in any Panacea source or `.tf`. Its own roadmap lists "Forge sandbox isolation (gVisor/Kata pool for untrusted AI-generated code) — L" as future work | `docs/superpowers/plans/2026-06-04-magic-roadmap.md:76` |
| 2 | Forge hardening: UID 1001, read-only root, drop ALL, `RuntimeDefault` seccomp, `emptyDir`, TTL reaper | **Holds** | `preview/manager.go:114-118`, `:198-206`, `:217-219`, `:581-597` |
| 3 | "Copy Forge's security context into our `SandboxTemplate`" | **Mostly already done.** Our `sandbox/manager.go:151-208` already carries UID 1001, non-root, read-only root, no escalation and drop ALL. The one missing line, seccomp, is likely a no-op inside Kata (2.3 row 15). The snippet worth taking is the egress pattern, not the pod spec | our `control-plane/internal/sandbox/manager.go` |
| 4 | `preview-deny-egress` NetworkPolicy plus npm/PyPI mirrors (Verdaccio 4873, proxpi 5000) | **Holds** | `deploy/forge/deploy-forge-orchestrator.yaml:61-101` |
| 5 | Mirror licences (the report left them unchecked) | **Both MIT.** `verdaccio/verdaccio`: MIT, 17,900 stars, pushed 2026-09-29. `EpicWink/proxpi`: MIT, 204 stars, pushed 2026-09-28, one maintainer: a small project to pin and watch | GitHub API |
| 6 | Tenant NetworkPolicy baseline | **Holds** ("Tenant NetworkPolicy baseline (ISM-1234)", default deny plus same-namespace) | `clients/tenant_ensure.go:20`, `:83-126` |
| 7 | No node pool has a workload runtime; no FIPS or Trusted Launch | **Holds** (no `workload_runtime`, `fips_enabled` or `secure_boot` in `deploy/*.tf`) | grep |
| 8 | The edge admits only KPMG AU egress, so our egress IP must be added before `panacea-mcp` works | **Holds** | `deploy/main.tf:581`, `:602-605` (`loadBalancerSourceRanges = var.edge_allowed_ip_ranges`) |
| 9 | panacea-api "TTL sandboxes" are release copies, not agent machines | **Holds** ("a TTL-bound test/demo copy of any release") | `models/instance.go:70-72`, `:87` |

- **The trade-off line stands:** co-hosting on Panacea's estate saves the new subscription and
  about US$1,250 a month at list, against `OD-5`, a cutover whose bar is no lost features, and a
  cluster with no Kata pool. Our own AKS, built from Panacea's patterns, is the better whole answer.
- **Coolify and Supabase:** reject for this capability holds. Neither runs a microVM, keeps a warm
  pool or offers a sandbox API; both are Docker or Postgres platforms (not re-read at source).

## 6. Orca inside a sandbox

Orca is the owner's pick and is not re-argued here. What was checked is only what the sandbox
design assumes about it. Sources: Orca's `skill-guides/orca-per-workspace-env.md`, its
`references/windows-scripts.md` and `docs/reference/headless-linux-server.md` (raw GitHub, `main`,
2026-09-30); the GitHub API.

- **Licence and maintenance:** MIT (GitHub API), 81,627 stars, pushed 2026-09-30; releases
  v1.4.215 to v1.4.217 on 09-27, 09-28 and 09-29. Pin the runtime to the desktop's version, as
  the report says.

| # | Assumption in `01-sandboxes.md` | Verdict | Evidence |
|---|---|---|---|
| 1 | The recipe's `create` prints one JSON object with the pairing code | **Holds.** "`pairingCode` and `projectRoot` are required"; `resume` "must print fresh recipe JSON because the pairing may have changed" | env guide §7 |
| 2 | The sandbox starts "`orca serve --json`" | **Wrong flag.** The verified invocation is `orca serve --port … --project-root … --pairing-address "$EXTERNAL_WSS_URL" --recipe-json`, and "There is no `--host` flag". The Linux CLI is `orca-ide`; a packaged start writes a bare `orca` into `~/.local/bin` | env guide, "The `orca serve` invocation"; headless doc |
| 3 | Orca's runtime is Electron under Xvfb, glibc 2.31 or later | **Holds.** "Current Orca builds start Xvfb automatically ... but Xvfb must be installed first". In a container: extract the AppImage (no FUSE; "neither requires a privileged container") and run a real init as PID 1, which "reaps exited PTY" processes | headless doc |
| 4 | A warm pod must not have run `orca serve` before its claim | **Holds.** "Never snapshot a machine on which the Orca runtime has already run"; two VMs from one such snapshot "emitted identical `deviceToken` and `pairedDeviceId`" | env guide §3 |
| 5 | Chromium's sandbox needs a non-root user | **Holds.** As root it needs `--no-sandbox` ("This disables a security boundary"). Our pod runs as UID 1001 with `allowPrivilegeEscalation: false`, so the setuid helper cannot work; only the user-namespace sandbox can, if the Kata guest kernel allows it. Unproven | headless doc |
| 6 | The pairing code must stay out of access logs | **Holds.** "A pairing offer is a capability containing a device credential and E2EE material ... do not put it in proxy access logs" | headless doc |
| 7 | Pairing works through a per-sandbox reverse-proxy path | **Holds in principle.** "Reverse proxies must support WebSocket upgrade and route the advertised path" | headless doc |
| 8 | Recipe scripts are "documented for macOS and Linux only" | **Wrong, in our favour.** `references/windows-scripts.md`: "A bare `.sh` will not execute there. Either require WSL or Git Bash ... through a `.cmd` file, or scaffold PowerShell equivalents." The residual risk is KPMG's script policy (PowerShell execution policy, WDAC), not checked | Windows reference |
| 9 | "Orca documents no pairing revocation" | **Contradicted by 02.** The cockpit study read Orca's source: "each paired client gets a revocable token" (`02-fleet-cockpit.md:83`). Not re-read here (GitHub API rate limit) | 02 |
| 10 | The upstream recipe CLI (PR 22574) closed unmerged | **Holds** (closed 2026-09-24, not merged) | GitHub API |

**One design gap the report does not name: who authenticates the `wss://` route.** Orca's desktop
dials the pairing address and presents Orca's own device credential, not a Keycloak token. The
router's scoped tokens (a header) and its session cookies (for browsers) are not something
Orca's client sends (inferred from the docs; not tested). So "one authenticated `wss://` route per
sandbox" can only mean Orca's pairing credential plus what our edge adds: TLS, a source-IP
allowlist to KPMG egress (Panacea's `edge_allowed_ip_ranges` pattern), an unguessable
per-sandbox path, and no path or query logging. That is defensible, but it should be stated in
owner call 4 rather than implied to be SSO.

## 7. Are 8 days realistic for our architecture?

*(to be filled)*

## 8. Corrected recommendation

*(to be filled)*

## 9. What could not be verified

*(to be filled)*
