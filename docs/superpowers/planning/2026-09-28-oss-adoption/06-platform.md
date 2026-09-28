# Open-source adoption survey — Phase 6 (inference gateway) and Phase 8 (our own environment)

**Researched 2026-09-28, in two passes** — a survey pass, then a verification pass that went
back and re-fetched every claim the survey had marked unverified. Read-only. No code
changed, no test run, no cluster touched.
Scope given: sandbox isolation, warm pools, the inference gateway, the environment.
Phase 8: 35 planned tasks, 5 approved. Phase 6: 27 planned tasks, 0 built.

Every licence below was read from the repository's own licence file or package metadata on
the date shown — not from a badge, a blog, a GitHub API field or memory. Where I could not
read it, the row says so.

**Third pass, 2026-09-28 (independent spot-check of the two decisive claims).** Both hold.
(a) `raw.githubusercontent.com/BerriAI/litellm/main/LICENSE` is MIT, copyright Berri AI 2023,
and carries the carve-out verbatim: *"All content that resides under the 'enterprise/'
directory of this repository, if that directory exists, is licensed under the license defined
in 'enterprise/LICENSE'."* (b) `raw.githubusercontent.com/kubernetes-sigs/agent-sandbox/main/LICENSE`
is the Apache License 2.0 text, and the README confirms the scope quoted below — *"Agent
Sandbox is a sandbox orchestrator. It delegates low-level container isolation to secure
'Sandbox Runtimes' (like gVisor or Kata Containers) by managing Pods configured to use these
runtimes (via `RuntimeClass`)"* — with `examples/kata-aks/` present in the tree.
**Not re-confirmable on this pass:** `api.github.com` returned
`403 API rate limit exceeded` for unauthenticated requests, so the star counts, issue states
and push dates quoted below stand on the earlier passes' reads and were not re-fetched.

---

## The headline, before the detail

1. **Phase 6 is mostly already written by someone else.** LiteLLM's MIT core does virtual
   keys, per-key/user/team budgets, TPM/RPM limits, key expiry, spend tracking and an
   Anthropic-spec `/v1/messages` endpoint that routes to both the Anthropic API **and**
   Bedrock. That is `OD-19` almost verbatim. Of 27 planned tasks, the honest estimate is
   that **about 18 become configuration and a thin Go broker**, and about 9 remain ours.
2. **Phase 8's warm pool has a real precedent and it is not a Kubernetes one.**
   Kata's own VM cache is QEMU-only and needs node-level config we do not own on AKS, so
   it is unavailable to us. The pattern that fits is **Agones** (Apache-2.0): `Fleet` +
   buffer autoscaler + an atomic `Allocation` API — a warm pool of pre-created pods handed
   out in one call. Adopting it whole is heavy; the allocation semantics are worth copying
   exactly.
3. **Two things we were about to depend on are not open source any more.** HashiCorp
   **Terraform** and **Vault** are BUSL-1.1, not MPL, since August 2023. Phase 8 should
   plan on **OpenTofu** (MPL-2.0) and **OpenBao** (MPL-2.0) unless KPMG already has paid
   HashiCorp entitlements. This is a licence finding, not a functional one.
4. **The largest risk found is not a licence — it is a live upstream bug that breaks AU data
   residency.** LiteLLM's `/v1/messages/count_tokens` ignores a custom `api_base` and goes to
   `api.anthropic.com` with the deployment's own key. **Verified OPEN on 2026-09-28**
   (issues #31149, open since 2026-06-24, and #42073). Claude Code calls that endpoint on the
   hot path, so an "AU data only" project (`OD-19`, `OD-24`) leaks silently to the Global
   endpoint. **Write the conformance test before writing any phase 6 code.** Detail in its
   own section below.

---

## Phase 6 — the inference gateway

### The decisive fact

`OD-21` already chose LiteLLM. The question this research had to answer is whether it can be
**deployed as-is**. It can, and the reason is narrower than "LiteLLM is a gateway":

> LiteLLM's proxy exposes an **Anthropic-specification `/v1/messages`** endpoint, and that
> endpoint routes to "all LiteLLM supported providers: `openai`, `anthropic`, `bedrock`,
> `vertex_ai`, `gemini`, `azure`…", with cost tracking, streaming, tool use, extended
> thinking, fallbacks and load balancing.
> — https://docs.litellm.ai/docs/anthropic_unified (read 2026-09-28)

That is the whole OD-19 shape in one endpoint. A Claude Code process in a Kata sandbox is
pointed at the gateway with `ANTHROPIC_BASE_URL` plus a per-session virtual key; the gateway
decides Global (Anthropic direct) or AU (Bedrock `ap-southeast-2`) by model alias; the
sandbox never sees an org key. No fork of LiteLLM is needed to get there.

### What is MIT and what is not

LiteLLM's root `LICENSE` is **MIT**, with one carve-out quoted exactly:

> "All content that resides under the 'enterprise/' directory of this repository, if that
> directory exists, is licensed under the license defined in 'enterprise/LICENSE'."
> — https://raw.githubusercontent.com/BerriAI/litellm/main/LICENSE (read 2026-09-28)

`enterprise/LICENSE.md` is the **BerriAI Enterprise License** — proprietary, production use
requires a paid subscription agreement and seat licences
(https://raw.githubusercontent.com/BerriAI/litellm/main/enterprise/LICENSE.md, read
2026-09-28). This is the classic permissive-core / commercial-add-on trap the brief warned
about, and **it is present here**. The directory exists and contains
`enterprise_hooks/`, `enterprise_ui/`, `litellm_enterprise/` and its own `pyproject.toml`.

**Therefore: deploy the MIT proxy, never enable an enterprise feature, and prove it in CI.**
The proxy image ships the enterprise code; what gates it is a licence key. Our build must
assert no licence key is configured and that no `enterprise_hooks` callback is in the
config.

| LiteLLM capability | Licence | Phase 6 relevance |
|---|---|---|
| Virtual keys (`/key/generate`), aliases, `duration`/`expires` | MIT | **Per-session scoped token (OD-19) — this is the feature** |
| `max_budget`, `budget_duration`, `tpm_limit`, `rpm_limit` | MIT | Quotas per user/team/project |
| `user_id` / `team_id` on a key; `/key/info`, `/user/info`, `/team/info` | MIT | Attribution per user, team, project |
| `/v1/messages` Anthropic-spec routing to Anthropic + Bedrock | MIT | Global vs AU switching |
| Router fallbacks, retries, load balancing | MIT | Provider failover |
| Prometheus `litellm_spend_metric`, `litellm_*_tokens_metric` labelled by key/team/user/model | MIT (see caveat) | Usage attribution surface |
| Admin-UI SSO beyond 5 users, SAML, SCIM | **Proprietary** | Not needed — humans never log into LiteLLM; the Hub is the UI |
| OIDC/JWT request auth | **Proprietary** | Not needed — the control plane mints virtual keys with the master key |
| Audit logs | **Proprietary** | Not needed — the Hub's provenance ledger is the audit record |
| Guardrails suite, key rotation, RBAC for orgs | **Proprietary** | Rotation is ours anyway: a session ends, its key dies |

**Caveat on Prometheus, stated honestly.** A third-party page
(truefoundry.com/blog/litellm-enterprise) lists Prometheus metrics as enterprise-gated. The
primary sources disagree: `docs.litellm.ai/docs/proxy/prometheus` states no licence
requirement, and `litellm/integrations/prometheus.py` on `main` contains no
`premium_user` / `_license_check` gate that I could find. I am reporting the primary source
and flagging that this has moved between versions before. **Verify against the exact image
tag we pin, in CI, before relying on it.**

### What is genuinely left for us to build (the ~9 tasks)

None of these exist in LiteLLM and none should:

1. **The broker in the Go control plane.** Session start calls `/key/generate` with
   `user_id`, `team_id`, project metadata, `duration` matched to the session's life, a
   budget, and a model allow-list; session end calls `/key/delete`. LiteLLM has no notion of
   "a sandbox session", so the lifecycle binding is ours.
2. **The data-classification routing rule (`OD-19`, `OD-24`).** "AU data only" must *force*
   AU. LiteLLM routes by model name and by key-level `models`; the policy that decides which
   alias a project may receive is a Hub decision, enforced by which aliases the minted key
   permits. LiteLLM will happily route anywhere if the key allows it — the constraint has to
   be expressed at mint time, and tested.
3. **Zero-data-retention proof.** LiteLLM logs request/response bodies to its DB and to
   callbacks by default. ZDR means `turn_off_message_logging` plus a test that asserts no
   prompt text reaches Postgres or any callback. This is a small config with a large
   compliance consequence and **must be pinned by a test, not by a comment.**
4. **Provenance.** A LiteLLM spend row knows key, user, team and model. It does not know
   *which brief, which lane, which task* spent the money. Carrying the provenance id through
   as key metadata and joining it in the Hub is ours, and it is the spine rule
   (`CLAUDE.md`: a component that cannot carry a provenance reference is a poor fit) — here
   the carrier is the key's `metadata`, so the thread survives.
5. **Attribution export into the Hub.** Scrape Prometheus or read the spend table; the
   per-user/team/project rollup that the cockpit shows is ours.
6. **Bedrock credentials and cross-cloud egress from Azure.** LiteLLM takes AWS creds; who
   holds them, how they rotate, and how an Azure-hosted pod reaches `ap-southeast-2` is
   phase 8 work that LiteLLM does not touch.
7. **Failure behaviour in the cockpit.** Budget exhausted, provider down, key expired
   mid-session — LiteLLM returns an error code; what the user sees is `AGENTIC-UX.md`'s
   problem.
8. **The policy-injection seam (`OD-23`).** Managed settings into the sandbox at session
   start is control-plane work.
9. **Pinning and upgrade.** A version-pinned image, a canary test on `/v1/messages` shape,
   and an upgrade pipeline — the same discipline `OD-20` demands for the CLI.

### Maturity evidence

**Verified from the GitHub API on 2026-09-28** (`api.github.com/repos/BerriAI/litellm`):

| Measure | Value |
|---|---|
| Stars | **59,734** |
| Open issues | **5,349** |
| Last push | **2026-09-28T00:25Z** — the same day |
| Created | 2023-07-27 |
| API `license.spdx_id` | **`NOASSERTION`** — GitHub cannot classify it, *because* of the `enterprise/` carve-out. The field's failure is itself evidence the carve-out is real. |

5,349 open issues against 59,734 stars is what a project with that install base looks like;
the risk it carries is **churn**, not abandonment. Pin the tag.

**The proxy is being rewritten in Rust, and this changes what we pin.** The repository
description now reads *"The fastest, litest AI Gateway. Rust core with Python SDK"* and the
docs carry **`[Beta] Rust AI Gateway`** (`docs.litellm.ai/docs/proxy/rust_gateway`, found
2026-09-28). Their stated migration rule is that Rust takes only request/response
translation, route by route, after each passes a parity suite, while **Python keeps auth,
configuration, routing, logging, callbacks and spend tracking** until parity lands
(`docs.litellm.ai/blog/litellm-rust-launch`).

Every phase 6 feature we depend on — virtual keys, budgets, spend, attribution — is in the
part Python still owns. **So: plan on the Python proxy, pin a Python-proxy tag, and treat
the Rust gateway as a later performance upgrade to re-qualify, not as the thing we build
against.** Throughput is not our constraint.

### The alternatives, and why they do not displace LiteLLM

All three are permissively licensed and real. They lose on one specific axis: the
Anthropic-native `/v1/messages` surface that Claude Code needs.

| Project | Repo | Licence (verified) | Verdict |
|---|---|---|---|
| Portkey AI Gateway | github.com/Portkey-AI/gateway | **MIT** — `Copyright (c) 2024 Portkey, Inc` | study-only |
| Envoy AI Gateway | github.com/envoyproxy/ai-gateway | **Apache-2.0** | study-only |
| Bifrost | github.com/maximhq/bifrost | **Apache-2.0** — `Copyright 2025 H3 Labs Inc.` | study-only |

- **Portkey (MIT)** is a fast TypeScript/Hono gateway with good provider coverage and a
  clean config model. It is a legitimate fallback if LiteLLM's enterprise creep ever reaches
  a feature we need. It does not give us LiteLLM's virtual-key/budget/spend database out of
  the box, which is the part of phase 6 we would otherwise write.
- **Envoy AI Gateway (Apache-2.0)** is the architecturally strongest option and the wrong one
  *now*: it is an Envoy Gateway extension, so adopting it means adopting Envoy Gateway,
  Gateway API CRDs and a control-plane-per-cluster. That is a wall of configuration, which
  `CLAUDE.md` names as a failure of our bar. Revisit it if we ever need per-request policy at
  mesh level.
- **Bifrost (Apache-2.0)** is a Go gateway — attractive because our control plane is Go and it
  would share a language and a build. It is young. Its claim is throughput, which is not our
  constraint: our constraint is attribution and governance.

**Recommendation: adopt LiteLLM (MIT core only), study Portkey as the escape hatch.** Do not
fork LiteLLM. If a needed feature is behind the enterprise wall, write it in our Go broker
rather than patching their tree, because a patch makes every upgrade a merge.

---

## Phase 8 — the environment, the sandbox and the warm pool

### The single most useful finding in this survey

**`kubernetes-sigs/agent-sandbox`** — Apache-2.0, a Kubernetes SIG project — is a controller
for exactly our workload, and it already ships a **Kata-on-AKS example**.

| Evidence | Value | Read |
|---|---|---|
| Repository | https://github.com/kubernetes-sigs/agent-sandbox | 2026-09-28 |
| Licence | **Apache-2.0** (`LICENSE`, verified at the raw file; GitHub metadata `spdx_id: Apache-2.0`) | 2026-09-28 |
| Stars | 4,044 | 2026-09-28 |
| Created / last push | 2025-08-12 / 2026-09-25 | 2026-09-28 |
| Releases | `v1.0.0` 2026-08-28, then `v1.0.1`…`v1.0.4` weekly to 2026-09-24 | 2026-09-28 |
| Open issues | 205 (active, not stale) | 2026-09-28 |
| API groups | `agents.x-k8s.io/v1beta1`, `extensions.agents.x-k8s.io/v1beta1` | 2026-09-28 |

**What it defines**

- **`Sandbox`** — one isolated, stateful, *singleton* pod with a stable hostname and
  optional persistent storage. Singleton with stable identity is precisely our
  one-agent-one-worktree model; a Deployment or a Job is the wrong shape, and we would
  otherwise have written this ourselves.
- **`SandboxTemplate`** — the reusable pod/PVC shape.
- **`SandboxClaim`** — the user-facing request, which gets *adopted* by a warm pod.
- **`SandboxWarmPool`** — pre-warmed pods so a claim "can be assigned in milliseconds".
- **`Lifecycle.shutdownTime` / `shutdownPolicy`** (`Retain` | `Delete`) — an absolute expiry
  after which the pod and service are torn down. That is our session TTL, already modelled.
- **Suspend / resume** — KEPs 119 and 694 (`sandbox-suspended-state`,
  `suspend-and-resume-for-beta`), plus pod-snapshot examples: hibernate an idle sandbox,
  resume on activity.

**The Kata-on-AKS example is in the tree**, at `examples/kata-aks/`
(`README.md`, `sandboxtemplate.yaml`, `sandboxclaim.yaml`, `sandboxwarmpool.yaml`,
`router.yaml`, plus `agent/` and `client/` — directory listing re-read 2026-09-28), and its
warm-pool
manifest reads:

```yaml
apiVersion: extensions.agents.x-k8s.io/v1beta1
kind: SandboxWarmPool
spec:
  replicas: 3
  sandboxTemplateRef: { name: kata-aks-template }
  updateStrategy:
    type: OnReplenish      # replace stale pods only on adoption — do not kill healthy Kata VMs
```

with the comment: *"Pre-warms a pool of Kata-isolated … sandboxes so that SandboxClaims get
adopted (and become Ready) without paying the cold-start cost of scheduling a new Kata VM,
pulling the image, and binding a PVC."* The pod template sets
`runtimeClassName: kata-vm-isolation` — the AKS Pod Sandboxing RuntimeClass we already
chose. `examples/hpa-swp-scaling/` drives `replicas` from a claim-creation-rate custom
metric; `examples/keda-scale-to-zero/` and `examples/podsnapshot-golden-warmpool/` cover the
two harder cases.

**What it would replace in phase 8.** The warm-pool half of the phase, and more of the pod
manager than expected: pool sizing and replenishment, the atomic claim→pod handoff, the
stale-template update strategy, per-sandbox PVC binding, stable identity, TTL expiry and
teardown, and suspend/resume. On a 35-task phase with 5 approved, this plausibly retires
**8 to 12 tasks** and de-risks several more. It does **not** replace our Terraform, our Hub
hosting, our identity configuration, our networking or our image pipeline.

**Verdict: adopt, with a deliberate seam.** Three conditions:

1. **Keep `control-plane/internal/backend/katabe` as the interface.** The Go control plane
   talks to `Sandbox`/`SandboxClaim` objects *through* our existing backend interface, so the
   day the project stalls we swap an implementation, not the product. A generated Go client
   ships in-tree (`clients/k8s/extensions/clientset/…`), which matters because our control
   plane is Go.
2. **Pin `v1.0.x` and own the upgrade.** `v1.0.0` is five weeks old and the API moved
   `v1alpha1 → v1beta1` recently enough that a migration guide ships with it. Four releases
   in a month is healthy velocity and also churn.
3. **Provenance check.** A `Sandbox` carries `PodMetadata.labels` and `annotations`, so the
   brief id, lane id and task id ride on the object and come back on every event. The
   provenance thread survives adoption. That was the test the brief set, and it passes.

**The one thing to verify before committing** — not verifiable from documentation alone:
whether `SandboxWarmPool` replenishment behaves acceptably when the Kata node pool has to
scale out a nested-virt node (minutes, not seconds). A warm pool that empties faster than
nodes arrive is a cold start with extra steps. Load-test it with
`dev/load-test/test-recipes/warmpool-burst-test.yaml`, which the repo ships for this.

### Why Kata's own warm-pool features do not help us

Kata has two built-in accelerators and **both are unavailable on AKS Pod Sandboxing**:

- **VM cache** (`vm_cache_number`, `vm_cache_endpoint`, `kata-runtime factory init`) keeps a
  pool of ready VMs. **QEMU only.**
- **VM templating** clones from a template with shared read-only kernel/initramfs/agent
  memory. Mutually exclusive with VM cache, and Kata's own documentation flags the shared
  memory as a security consideration.
  — https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/what-is-vm-cache-and-how-do-I-use-it.md (read 2026-09-28)

AKS Pod Sandboxing runs Kata on **Cloud Hypervisor** under Microsoft Hyper-V, not QEMU, and
the node's `configuration.toml` is managed by AKS — we do not own it. Both features also need
a `kata-runtime` invocation on the node, which a managed node pool does not give us.

**Consequence, and it is a design consequence rather than a footnote:** on AKS the warm pool
must be a **pod-level** pool of already-started Kata pods. That is what `SandboxWarmPool`
does, and it is why "warm pools are mandatory" has to be solved above the runtime rather than
inside it. Anyone planning phase 8 on the assumption that `vm_cache_number` will do the job
is planning the wrong thing.

### The pattern to copy if we ever outgrow agent-sandbox

**Agones** (https://github.com/googleforgames/agones, **Apache-2.0**, verified at `LICENSE`
2026-09-28) has run this exact pattern in production for years: a `Fleet` of warm
`GameServer` pods, a `FleetAutoscaler` with a **buffer** policy, and a `GameServerAllocation`
that atomically moves one pod `Ready → Allocated` and returns its address in a single call.
Its `Packed` vs `Distributed` scheduling strategies are the right vocabulary for a
nested-virt node pool where bin-packing saves real money.

**Verdict: study-only.** Adopting Agones means adopting a game-server product — an SDK the
workload must call, health semantics we do not want, and a vocabulary nobody on a KPMG
platform team will recognise. Read its allocation design, copy the buffer and allocation
semantics, do not take the dependency.

### Isolation: keep Kata, and know what the alternatives buy

All four are Apache-2.0 and all four are alive. The question is not "which is best" but "does
anything beat a managed Kata on AKS for us", and the answer is no — because *managed* is the
feature.

| Runtime | Repo | Licence (verified 2026-09-28) | Stars | Last push | What it buys | Verdict |
|---|---|---|---|---|---|---|
| **Kata Containers** | kata-containers/kata-containers | Apache-2.0 | 8,807 | 2026-09-25 | Per-pod microVM, own kernel, a Kubernetes RuntimeClass, **managed by AKS** | **Keep (already locked)** |
| Firecracker | firecracker-microvm/firecracker | Apache-2.0 | 36,979 | 2026-09-25 | ~125 ms boot, tiny footprint, snapshot/restore | study-only |
| gVisor | google/gvisor | Apache-2.0 | 19,433 | 2026-09-28 | Userspace kernel: much faster start than a VM, weaker boundary, syscall gaps | study-only |
| Cloud Hypervisor | cloud-hypervisor/cloud-hypervisor | **Dual Apache-2.0 / BSD-3-Clause — now verified**, see below | 6,263 | 2026-09-26 | The VMM AKS already runs under Kata | n/a — we get it via AKS |

- **Firecracker** is the strongest technical story for warm starts: snapshot a booted VM and
  restore it in tens of milliseconds, which is how E2B and others get "instant" sandboxes.
  It is also the reason to *not* switch: Firecracker on Azure means building the node
  substrate ourselves, and `OD-18` chose Azure precisely because AKS manages Kata. Adopting
  Firecracker trades a phase-8 warm-pool problem for a whole-platform problem.
- **gVisor** is worth keeping as a **second, cheaper tier**. Agent Sandbox ships a gVisor
  example (`examples/openclaw-gvisor-sandbox/`), so a lower-assurance workload — say a
  preview render, or local-mode parity in cloud — could use `runtimeClassName: gvisor` on the
  same controller with the same manifests. Note it is a weaker boundary than a microVM, so
  it is a tier, never a substitute, and a regulated-data project must never land on it.
- **Cloud Hypervisor — the earlier caveat is now resolved.** GitHub's classifier returns no
  SPDX id and `api.github.com/repos/.../license` answers 404, which is why the first pass
  could not confirm it. The reason is that the project follows the **REUSE** convention:
  there is no root `LICENSE` file, and the licences live in a `LICENSES/` **directory**
  containing exactly `Apache-2.0.txt`, `BSD-3-Clause.txt` and `CC-BY-4.0.txt`
  (`api.github.com/repos/cloud-hypervisor/cloud-hypervisor/contents/LICENSES`, read
  2026-09-28). **Dual Apache-2.0 / BSD-3-Clause for code, CC-BY-4.0 for documentation — all
  permissive, all allowed.** No decision depended on it, but the record is now closed rather
  than open.

  *Generalise this:* a 404 on the licence API means "GitHub could not classify it", never
  "there is no licence". Check for `LICENSES/`, `LICENSE.md`, `COPYING` and a `REUSE.toml`
  before concluding anything — the opposite mistake (Daytona, below) is the one that costs.

Two adjacent projects that answer "could someone else's whole sandbox product replace ours":

- **E2B infra** (github.com/e2b-dev/infra, **Apache-2.0**, `Copyright 2023 FoundryLabs, Inc.`,
  verified 2026-09-28) — Firecracker-based sandbox orchestration with snapshotting, on Nomad,
  not Kubernetes. **Study-only**: read their snapshot/resume design, which is the part we
  cannot get from Kata on AKS. Adopting it means adopting Nomad and abandoning `OD-18`.
- **microsandbox** (github.com/microsandbox/microsandbox, **Apache-2.0**, 8,450 stars, pushed
  2026-09-27) — "fast branchable microVM runtime". The *branchable* microVM idea maps
  suspiciously well to our one-worktree-per-agent model. **Study-only** for the same reason:
  it is not a Kubernetes runtime, so it does not fit the locked architecture.
- **Daytona** (github.com/daytonaio/daytona) — **reject, and this is a warning about how fast
  this category moves.** 71,700 stars, and the repository's own README now opens: *"This
  repository is no longer maintained. As of June 2026, Daytona's core development has moved
  to a private codebase."* There is **no LICENSE file at the repository root** and GitHub
  reports no SPDX id. A popular project with no licence is not a permissive project; it is an
  unlicensed one, which means no grant at all. Do not use it.

### Per-session credentials: the broker is ours, the primitives are not

Nothing off the shelf is "a per-session ephemeral credential broker for coding agents". What
exists is the plumbing, and the plumbing is good.

| Project | Licence (verified 2026-09-28) | Stars | Last push | Use | Verdict |
|---|---|---|---|---|---|
| **LiteLLM virtual keys** | MIT | — | — | The per-session inference token itself (`duration`, budget, `models` allow-list) | **adopt** |
| **SPIFFE / SPIRE** | Apache-2.0 | 2,559 | 2026-09-26 | Workload identity: a per-pod SVID, short-TTL, auto-rotated, no secret to leak | **study-only now, adopt later** |
| **Azure Workload Identity** | MIT | 340 | 2026-09-23 | Federated pod → Entra token on AKS with no stored secret | **adopt** (AKS-native path) |
| **External Secrets Operator** | Apache-2.0 | 6,885 | 2026-09-25 | Key Vault → Kubernetes secret sync for the *platform's* secrets | adopt |
| **OpenBao** | MPL-2.0 | 8,135 | 2026-09-24 | Dynamic short-lived secrets, if we need more than Key Vault | **conditional** (MPL — see below) |
| **Keycloak** | Apache-2.0 | — | — | Already locked by `OD-30` | adopt (locked) |
| **ory/hydra** | Apache-2.0 | 17,573 | 2026-07-29 | A pure OAuth2/OIDC token service, if Keycloak is ever the wrong shape | reject (Keycloak is chosen; two IdPs is worse than one) |

**HashiCorp Vault is not an option, and this needs saying plainly.** Its `LICENSE` on `main`
is **BUSL-1.1**, Licensor "International Business Machines Corporation (IBM)", Change License
MPL-2.0 after four years, with an Additional Use Grant that permits production use only
where you do not offer the work "to third parties on a hosted or embedded basis in order to
compete with IBM Corp's paid version(s)". BUSL is on the brief's forbidden list. Use **Azure
Key Vault** (a service, not a dependency) for platform secrets, and **OpenBao** (MPL-2.0, the
Linux Foundation fork) only if we need dynamic secrets. Verified at
https://raw.githubusercontent.com/hashicorp/vault/main/LICENSE, 2026-09-28.

**The three credential flows phase 6/8 must build, none of which are off the shelf:**

1. **Inference token** — mint a LiteLLM virtual key at session start, scoped to the project's
   model aliases, budgeted, expiring with the session. Adopt LiteLLM; write the broker.
2. **Git credential** — `OD-22`'s read-only per-repository fetch credential, plus the
   signed-in person's own token for every push (`OD-4`). This is Azure DevOps / GitHub
   app-installation token work. Nothing generic replaces it.
3. **Sandbox → platform identity** — how a Kata pod proves which session it is when it calls
   the control plane. **Azure Workload Identity (MIT)** is the AKS-native answer and should be
   the default; **SPIRE** is the portable answer and becomes interesting when `CLAUDE.md`'s
   "customer-hosted Azure Kubernetes App tier later" arrives. Do not take SPIRE now — it is a
   second identity system to run, and the wedge is usability.

### Admission policy and egress control for the sandbox

Phase 8 has to *enforce* the Kata constraints `PROGRAM.md` lists — mandatory explicit
CPU/memory requests, no host networking, the right RuntimeClass — or a mis-specced pod
silently loses its microVM. That is an admission-policy job, and writing one by hand is a
mistake.

| Project | Licence (verified 2026-09-28) | Stars | Verdict |
|---|---|---|---|
| **Kyverno** | Apache-2.0 | 8,190 | **adopt** — YAML policies, no new language, mutate *and* validate |
| OPA / Gatekeeper | Apache-2.0 | 4,283 | study-only — Rego is a language to learn; more power than we need |
| Kubewarden | Apache-2.0 | 238 | reject — smallest community of the three |
| **Cilium** | Apache-2.0 | 25,564 | **adopt** — AKS already offers "Azure CNI powered by Cilium"; egress policy per sandbox namespace is how the gateway becomes the *only* route to an LLM |
| Falco | Apache-2.0 | 9,420 | adopt later — Panacea already runs it; note `PROGRAM.md`'s existing compliance gap that Defender cannot assess Kata pods, which makes a runtime sensor more valuable here, not less |

**Kyverno is the recommendation and it is a cheap one**: a handful of `validate` policies
("a sandbox pod MUST set `runtimeClassName`, MUST set requests and limits, MUST NOT set
`hostNetwork`") turn a class of phase-8 bug into a rejected admission. Mutate policies can
inject the gateway's `ANTHROPIC_BASE_URL` so no image has to hard-code it.

**Cilium matters more than it looks.** `OD-19` says a sandbox never holds an org key. The
complement is that a sandbox must not be able to reach `api.anthropic.com` directly at all.
A default-deny egress policy with the gateway as the only permitted destination turns a
policy statement into a network fact, and that is the difference between a control and a
claim in a regulated review.

### Node capacity, image pull, and the rest of the warm-pool problem

A pod-level warm pool only helps if a node and an image are already there.

| Project | Licence (verified 2026-09-28) | Stars | What it solves | Verdict |
|---|---|---|---|---|
| **KEDA** | Apache-2.0 | 10,548 | Event-driven scaling of the warm pool; Agent Sandbox ships a `keda-scale-to-zero` example | **adopt** |
| **kube-fledged** | Apache-2.0 | 1,374 | Pre-pulls and pins images on nodes via an `ImageCache` CRD — kills the image-pull half of Kata cold start | **adopt** (small, single-purpose, exactly our problem) |
| Karpenter | Apache-2.0 | 2,207 | Node-level provisioning; AKS Node Auto-Provisioning is Karpenter | study-only — use the AKS-managed form, not the upstream |
| Cluster overprovisioner (pause pods) | Apache-2.0 (Helm chart pattern) | — | Low-priority placeholder pods reserve node headroom so a real pod schedules instantly | **vendor** — it is ~30 lines of YAML, not a dependency |
| Knative Serving | Apache-2.0 | 6,099 | Scale-to-zero + activator queueing | reject — request-scoped; our sessions are long-lived and stateful |
| OpenKruise | Apache-2.0 (verified at `master/LICENSE.md`; GitHub's API reports `NOASSERTION` because the file is `LICENSE.md`) | 5,348 | `ImagePullJob`, advanced workloads | study-only — overlaps kube-fledged and brings far more surface |

**The layered answer, in order of what to build first:** pause-pod overprovisioning for node
headroom (cheapest, one manifest) → `kube-fledged` `ImageCache` for the image → Agent
Sandbox `SandboxWarmPool` for the started Kata pod → KEDA or an HPA on claim rate to size the
pool. Each layer removes one term from the cold-start sum, and each is separately testable.

**A note on GitHub's licence field, because it misled this research twice.** GitHub reported
`NOASSERTION` for OpenKruise and for Langfuse. OpenKruise turned out to be plain Apache-2.0
in a file named `LICENSE.md` that the classifier does not read; Langfuse turned out to be
MIT with a commercial carve-out that the classifier cannot express. **The API's licence field
is a hint, never the answer.** Every licence in this document was read from the actual file.

### Usage attribution and telemetry

| Project | Licence (verified 2026-09-28) | Stars | Verdict |
|---|---|---|---|
| **LiteLLM Prometheus metrics** | MIT (see the phase 6 caveat) | — | **adopt** — `litellm_spend_metric`, `litellm_input/output/total_tokens_metric`, labelled by `hashed_api_key`, `api_key_alias`, `team`, `team_alias`, `user`, `user_email`, `model` |
| **OpenTelemetry GenAI semantic conventions** | Apache-2.0 | — | **adopt the vocabulary** — name our spans and attributes theirs, so we are never the only system with our own words for "input tokens" |
| OpenLLMetry (traceloop) | Apache-2.0 | 7,454 | study-only — an SDK that instruments *callers*; our caller is Claude Code, which we do not modify (`PROGRAM.md` cross-cutting constraint) |
| OpenLIT | Apache-2.0 | 2,801 | study-only — same reason |
| **envoyproxy/ratelimit** | Apache-2.0 | 2,699 | study-only — a proven global rate-limit service, but LiteLLM's per-key TPM/RPM already covers phase 6; revisit only if we need limits LiteLLM cannot express |

**The point about OTel is the important one.** LiteLLM's Prometheus labels already carry
key, user, team and model. What they do not carry is *project*, *brief*, *lane* and *task* —
the provenance thread. Put those on the virtual key's `metadata` at mint time, emit them as
OTel GenAI attributes from our broker, and the cockpit can answer "what did this brief cost"
rather than only "what did this key cost". That is the difference between telemetry and
provenance, and the spine rule makes it non-optional.

**The OTel names, recovered after the proxy block.** `opentelemetry.io` is blocked by
Netskope (see the blocked list at the end), so the first pass could not quote the
convention. The spec is readable from GitHub raw instead, and it has **moved repositories**:
`open-telemetry/semantic-conventions` now says *"GenAI semantic conventions have moved to the
OpenTelemetry GenAI semantic conventions repository"*. The live source is
`open-telemetry/semantic-conventions-genai`, `docs/gen-ai/gen-ai-metrics.md`, read
2026-09-28:

| Metrics | Attributes |
|---|---|
| `gen_ai.client.operation.duration` · `gen_ai.client.operation.time_to_first_chunk` · `gen_ai.client.operation.time_per_output_chunk` · `gen_ai.server.request.duration` · `gen_ai.server.time_to_first_token` · `gen_ai.server.time_per_output_token` · `gen_ai.invoke_workflow.duration` · `gen_ai.invoke_agent.duration` · `gen_ai.invoke_agent.inference_calls` · `gen_ai.invoke_agent.tool_calls` · `gen_ai.execute_tool.duration` | `gen_ai.operation.name` · `gen_ai.provider.name` · `gen_ai.request.model` · `gen_ai.response.model` · `gen_ai.workflow.name` · `gen_ai.agent.name` · `gen_ai.tool.name` · `gen_ai.tool.type` (plus the stable cross-spec `error.type`, `server.address`) |

**Two warnings that change how we should use this.**

1. **Every one of those names is at `Development` stability** — OTel's weakest tier, which
   may be renamed or removed without a major version. Adopt the vocabulary in our *emitter*,
   behind our own constant names, so a rename is one file and not a schema migration in the
   Hub.
2. **The convention has no attribute for the thing we most need.** There is no
   `gen_ai.*` name for project, brief, lane or task, and the agent attributes it does define
   (`gen_ai.agent.name`) do not carry an id. So the provenance thread cannot be expressed in
   OTel's vocabulary alone — it must ride as our own namespaced attributes
   (e.g. `panacea.brief.id`, `panacea.lane.id`, `panacea.task.id`) alongside theirs. Use
   their words for the LLM facts, our words for the provenance facts, and never invent a
   second word for "input tokens".

---

## The one integration risk that could break AU data residency

This is the most consequential single finding after the two adoption calls, and it is a
known open defect, not a hypothetical.

Claude Code does not only call `/v1/messages`. It also calls **`/v1/messages/count_tokens`**,
and LiteLLM has repeated production reports that this endpoint **ignores a deployment's
custom `api_base` and falls back to `api.anthropic.com`**, sending the deployment's own key
there:

**Both issues were confirmed OPEN from the GitHub API on 2026-09-28**, not inferred from
search metadata:

| Issue | Title (verbatim) | State | Opened | Last updated |
|---|---|---|---|---|
| **BerriAI/litellm #31149** | *"[Bug]: Anthropic `/v1/messages/count_tokens` ignores custom `api_base` despite repeated production reports"* | **OPEN** | 2026-06-24 | 2026-09-20 |
| **BerriAI/litellm #42073** | *"[Bug]: Proxy `/v1/messages/count_tokens` sends third-party `api_key` to `api.anthropic.com` when deployment uses custom `api_base` (`anthropic/*` model)"* | **OPEN** | 2026-09-20 | 2026-09-20 |

Read at `api.github.com/repos/BerriAI/litellm/issues/31149` and `/42073`, 2026-09-28.

**#31149 has been open for three months and its own title says "despite repeated production
reports".** #42073 names the mechanism precisely: `AnthropicTokenCounter.count_tokens()`
does not pass the custom `api_base` through to the count-tokens handler, so it defaults to
`https://api.anthropic.com/v1/messages/count_tokens` — **and carries the deployment's key
with it**. This is not a rumour and it is not stale. It is live, reproduced, and upstream
has not fixed it.

A third report, anthropics/claude-code-action **#1089** (403 when using `ANTHROPIC_BASE_URL`
with a LiteLLM proxy), is recorded from search metadata only — **that one I did not open**,
so treat it as unverified.

**Why this matters more to us than to most users.** `OD-19` and `OD-24` say a project
classified "AU data only" is *forced* to Bedrock Sydney. If `count_tokens` silently leaves
for the Global endpoint, message content has crossed a boundary that a compliance review was
told it would not cross. The failure is silent and it is on the hot path of every Claude Code
turn.

**What phase 6 must therefore do, whatever else it does:**

1. A **conformance test** that points a real Claude Code at the gateway with an AU-classified
   key and asserts, from egress logs or a Cilium policy denial, that **no request reached
   `api.anthropic.com`** — including `count_tokens`.
2. **Cilium default-deny egress** from the gateway's namespace to anything but the two
   intended provider endpoints, so a regression is a blocked connection rather than a leak.
3. Pin the LiteLLM version, and re-run the conformance test on every bump. This is the same
   canary discipline `OD-8` and `OD-20` already demand elsewhere.

That test is worth writing before any other phase 6 task.

---

## Candidate table

Licences were read at the repository's own licence file on **2026-09-28** unless the row says
otherwise. Stars and last-push dates are from the GitHub API the same day.

| # | Candidate | Licence | Replaces | Cost | Verdict |
|---|---|---|---|---|---|
| 1 | **LiteLLM** (core only) | MIT + proprietary `enterprise/` | Phase 6: virtual keys, budgets, TPM/RPM, spend, Anthropic↔Bedrock routing — ~18 of 27 tasks | Deploy + config + a Go broker; a CI assertion that no enterprise feature is on | **adopt** |
| 2 | **kubernetes-sigs/agent-sandbox** | Apache-2.0 | Phase 8: warm pool, claim→pod adoption, per-sandbox PVC, TTL, suspend/resume — ~8–12 of 35 tasks | CRDs + controller + Go client behind `katabe`; pin `v1.0.x` | **adopt** |
| 3 | **Kyverno** | Apache-2.0 | Phase 8: enforcing the Kata constraints at admission | A handful of YAML policies | **adopt** |
| 4 | **Cilium** (Azure CNI powered by Cilium) | Apache-2.0 | Phase 8: sandbox egress default-deny; makes "never an org key" a network fact | An AKS CNI choice at cluster creation | **adopt** |
| 5 | **kube-fledged** | Apache-2.0 | Phase 8: image pre-pull, one term of the cold-start sum | One CRD, one `ImageCache` object | **adopt** |
| 6 | **KEDA** | Apache-2.0 | Phase 8: sizing the warm pool from claim rate | Standard AKS add-on | **adopt** |
| 7 | **Azure Workload Identity** | MIT | Phase 8: sandbox→platform identity with no stored secret | AKS-native, small | **adopt** |
| 8 | **External Secrets Operator** | Apache-2.0 | Phase 8: Key Vault → cluster secrets | Small | **adopt** |
| 9 | **OpenTofu** | MPL-2.0 | Phase 8: the IaC engine, replacing BUSL Terraform | Drop-in; providers are unchanged | **adopt (conditional — MPL)** |
| 10 | **terraform-provider-azurerm** | MPL-2.0 | Phase 8: all Azure resources | Already the plan | **adopt (conditional — MPL)** |
| 11 | Cluster overprovisioner (pause pods) | Apache-2.0 pattern | Phase 8: node headroom | ~30 lines of YAML | **vendor** |
| 12 | **OpenBao** | MPL-2.0 | Phase 8: dynamic secrets, only if Key Vault is insufficient | A service to run | **conditional — defer** |
| 13 | Portkey AI Gateway | MIT | Phase 6 fallback if LiteLLM's enterprise wall moves | — | study-only |
| 14 | Envoy AI Gateway | Apache-2.0 | Phase 6 at mesh scale | High: Envoy Gateway + Gateway API | study-only |
| 15 | Bifrost | Apache-2.0 | Phase 6, in Go | Young; solves throughput, not governance | study-only |
| 16 | Agones | Apache-2.0 | Phase 8 warm-pool *semantics* | High if adopted whole | study-only |
| 17 | Firecracker | Apache-2.0 | Phase 8 isolation + snapshot warm start | Whole-platform change; off AKS | study-only |
| 18 | gVisor | Apache-2.0 | Phase 8: a cheaper second isolation tier | Low — Agent Sandbox already has an example | study-only |
| 19 | E2B infra | Apache-2.0 | Phase 8 snapshot/resume design | Nomad, not Kubernetes | study-only |
| 20 | microsandbox | Apache-2.0 | Phase 8 branchable-microVM idea | Not a Kubernetes runtime | study-only |
| 21 | SPIFFE / SPIRE | Apache-2.0 | Phase 8 portable workload identity | A second identity system to run | study-only (revisit for customer-hosted) |
| 22 | OpenKruise | Apache-2.0 (`LICENSE.md`) | Overlaps kube-fledged | Large surface | study-only |
| 23 | Karpenter | Apache-2.0 | Node provisioning — use the AKS-managed form | — | study-only |
| 24 | envoyproxy/ratelimit | Apache-2.0 | Phase 6 limits LiteLLM cannot express | A service to run | study-only |
| 25 | OpenLLMetry / OpenLIT | Apache-2.0 | Caller-side LLM instrumentation | Instruments callers; we do not modify Claude Code | study-only |
| 26 | OPA / Gatekeeper | Apache-2.0 | Alternative to Kyverno | Rego | study-only |
| 27 | Knative Serving | Apache-2.0 | Scale-to-zero | Request-scoped; our sessions are long-lived | reject |
| 28 | Kubewarden | Apache-2.0 | Alternative to Kyverno | Smallest community | reject |
| 29 | ory/hydra | Apache-2.0 | Alternative to Keycloak | `OD-30` already chose Keycloak | reject |
| 30 | **Langfuse** | MIT + proprietary `ee/` | LLM observability | Same carve-out trap as LiteLLM; we already get attribution from LiteLLM | reject (for now) |
| 31 | **Daytona** | **No LICENSE file; repo unmaintained since June 2026, core moved private** | — | — | **reject** |
| 32 | **HashiCorp Terraform** | **BUSL-1.1 (IBM)** | — | — | **reject — use OpenTofu** |
| 33 | **HashiCorp Vault** | **BUSL-1.1 (IBM)** | — | — | **reject — use Key Vault / OpenBao** |
| 34 | **Arize Phoenix** | **Elastic License 2.0** | LLM tracing | — | **reject — ELv2 is on the forbidden list** |
| 35 | **anthropic-experimental/sandbox-runtime** | Apache-2.0 | Phase 8: the second ring inside the microVM (filesystem + network limits on the agent process); the only real boundary in local Docker mode | Wrap the launch command in the sandbox image; pin a commit, not a branch | **adopt** (added fifth pass) |
| 36 | **nestybox/sysbox** | Apache-2.0 | Phase 2/8 local mode: docker-in-docker without --privileged | A host runtime; redundant on Kata | study-only (added fifth pass) |
| 37 | Claude apps gateway (`claude gateway`) | **Anthropic-proprietary, ships in the `claude` binary** | Phase 6 for Claude Code only: OIDC sign-in, per-group model access, managed settings, spend limits, OTLP | Not OSS; serves only Anthropic clients while we run four harnesses | study-only (added fifth pass) |

### On the MPL-2.0 rows, since the brief asked for the obligation to be stated

MPL-2.0 is **file-level** copyleft. If we modify an MPL-licensed *file* and distribute the
result, that file's source must be made available under MPL — it does **not** reach our own
files, and it does **not** make PanaceaApp open source. For OpenTofu and the Azure provider
the practical position is simpler still: we **run** them in a pipeline, we do not link them
into the shipped product and we do not modify their source. The obligation is effectively
dormant. **The rule to write into the licence gate: never patch an MPL file in-tree.** If a
provider bug needs a workaround, work around it in our configuration, not in their code.

---

## Copyleft and restricted projects found, named, and not read

Recorded so the file shows we looked. **No source from any of these was opened.**

| Project | Licence | What it is | Why excluded |
|---|---|---|---|
| Coder (`coder/coder`) | **AGPL-3.0** | Self-hosted cloud development environments on Kubernetes — the closest whole-product analogue to phases 3/8 | AGPL §13 fires on network interaction with a hosted modified copy. The exact trap `OD-11` already cost this programme once. |
| Gitpod (`gitpod-io/gitpod`) | **AGPL-3.0** | Ephemeral dev environments, workspace lifecycle | AGPL |
| Teleport (`gravitational/teleport`) | **AGPL-3.0** | Short-lived certificate access broker — genuinely relevant to per-session credentials | AGPL |
| HashiCorp Terraform (≥1.6) | **BUSL-1.1**, Licensor IBM, Change License MPL-2.0 after 4 years | IaC engine | BUSL. Additional Use Grant forbids offering it "on a hosted or embedded basis in order to compete". Not worth the legal reading when OpenTofu exists. |
| HashiCorp Vault | **BUSL-1.1**, Licensor IBM | Secrets engine | BUSL |
| Arize Phoenix | **Elastic License 2.0** | LLM tracing and evaluation | ELv2 forbids providing the software to third parties as a hosted or managed service, and forbids circumventing the licence key — we ship to customers who run it on their own cloud |
| LiteLLM `enterprise/` | **BerriAI Enterprise License** (proprietary) | SSO/SAML/SCIM, JWT auth, audit logs, guardrails, key rotation | Production use requires a paid subscription. The MIT core is used; this directory is not. |
| Langfuse `ee/`, `web/src/ee/`, `worker/src/ee/` | proprietary (`ee/LICENSE`) | Enterprise observability features | Same shape |
| Daytona | **none published** | AI-code sandbox infrastructure | No licence file at all = no grant. Also unmaintained. |
| tldraw | commercial licence (already excluded by `OD-11`) | Canvas | Out of scope here; listed for completeness of the exclusion record |

---

## What I could not verify

Stated plainly, because a survey that hides its gaps is worse than a shorter one.

This section was written in two passes. A second pass on 2026-09-28 went back and closed
four of the six gaps; what is listed as closed was actually re-fetched, not reasoned away.

**Closed on the second pass:**

1. ~~Cloud Hypervisor's licence text.~~ **Closed.** REUSE convention: `LICENSES/` holds
   `Apache-2.0.txt`, `BSD-3-Clause.txt`, `CC-BY-4.0.txt`. All permissive.
2. ~~The status of LiteLLM issues #31149 and #42073.~~ **Closed, and it is the bad answer.**
   Both **OPEN** on 2026-09-28 via the GitHub API; #31149 open since 2026-06-24. The AU
   data-residency risk is live, not historical.
3. ~~LiteLLM star count and release cadence as numbers.~~ **Closed.** 59,734 stars, 5,349
   open issues, last push 2026-09-28T00:25Z.
4. ~~The OTel GenAI attribute names (blocked by the proxy).~~ **Closed by another route** —
   the spec moved to `open-telemetry/semantic-conventions-genai` and is readable on GitHub
   raw. Names quoted above; all at `Development` stability.

**Still open, and these are the ones that matter now:**

5. **`SandboxWarmPool` behaviour under a node-scale-out burst on nested-virt SKUs.** Nobody's
   documentation answers this. **This is the single unknown that decides whether 8–12 phase 8
   tasks are deleted or kept.** **CORRECTED on the fourth pass: there is no
   `warmpool-burst-test.yaml` in the repository** — the earlier passes asserted one exists and
   that was wrong. The burst harness has to be written by us. See the fourth-pass section for
   what the repo actually ships and what the nearest load driver is.
6. ~~**Whether LiteLLM's Prometheus exporter is licence-gated in the version we would pin.**~~
   **Closed on the fourth pass, and the answer is that it is not gated.** Detail in the
   fourth-pass section. The advice to re-assert it against the pinned tag in CI still stands,
   because this has moved between versions before.
7. **Agent Sandbox's production users.** 4,044 stars and weekly `v1.0.x` releases is evidence
   of activity, not of anyone running it at our scale in a regulated environment. No
   reference customer was found.
8. **claude-code-action issue #1089.** Title from search metadata only; thread not opened.
9. **The Rust gateway's parity boundary at any specific tag.** The migration is route-by-route
   and the docs describe the rule, not a per-release matrix. Read the release notes of the
   tag we pin; do not assume Python still owns a path because it did last month.
10. **Nothing here was run.** No cluster, no proxy, no test. Every claim is documentation,
    licence text and API metadata. The three spikes in "Recommended order of work" are where
    this turns into evidence.

## Fetches the corporate proxy blocked

| URL | Result | Worked around? |
|---|---|---|
| `https://opentelemetry.io/docs/specs/semconv/gen-ai/` | **Blocked by Netskope** — "Unable to verify if domain opentelemetry.io is safe to fetch… enterprise security policies" | **Yes.** `raw.githubusercontent.com/open-telemetry/semantic-conventions-genai/...` served the same spec. The names are quoted above. |

**One domain blocked, in the whole survey.** `raw.githubusercontent.com`, `api.github.com`,
`github.com` and `docs.litellm.ai` were all reachable, which is what made licence
verification from source possible. **The workaround generalises: when a spec site is blocked,
its GitHub source repository usually is not.**

Fetches that failed for ordinary reasons rather than the proxy, listed so nobody re-runs
them: `raw.githubusercontent.com/BerriAI/litellm/main/enterprise/LICENSE` (404 — the file is
`LICENSE.md`), `docs.litellm.ai/docs/proxy/enterprise` (404 — moved to `/docs/enterprise`),
`agent-sandbox.sigs.k8s.io/docs/concepts/warm-pools/` and `/docs/concepts/` (404 — the site's
navigation differs from the docs tree), `raw.githubusercontent.com/daytonaio/daytona/main/LICENSE`
(404 — **no licence file exists**), `api.github.com/search/code` (401 — unauthenticated code
search is not permitted), `examples/kata-aks/warmpool.yaml` (404 — the file is
`sandboxwarmpool.yaml`), and on the third pass **all of `api.github.com`** (403, *"API rate
limit exceeded"* — the earlier passes spent the unauthenticated hourly budget; use
`gh api` with the repo's PAT if these need re-checking).

---

## Recommended order of work

1. **Write the phase 6 AU-residency conformance test first** (the `count_tokens` risk),
   before any gateway code. It is the cheapest thing here and the only one whose absence is a
   compliance finding.
2. **Stand up LiteLLM from the MIT core**, with the Go broker minting per-session virtual
   keys carrying provenance ids in `metadata`. That is phase 6's spine.
3. **Spike Agent Sandbox's `examples/kata-aks/` against a scratch AKS cluster**, behind the
   existing `katabe` interface. That example ships `sandboxtemplate.yaml`,
   `sandboxwarmpool.yaml`, `sandboxclaim.yaml` and `router.yaml` — **but no burst harness, so
   write the load driver yourself** (the nearest thing upstream is
   `examples/hpa-swp-scaling/create-claim.py`). One spike decides whether 8–12 phase 8 tasks
   get deleted or kept.
4. **Add the Kyverno policies and the Cilium egress default-deny** at the same time as the
   first Kata node pool, not after — retrofitting a default-deny to a running estate is the
   expensive version.
5. **Switch the IaC engine to OpenTofu** before phase 8's Terraform is written, because
   changing it later is a rewrite of every pipeline that calls it.

## The honest bottom line

Open source takes a real bite out of these two phases, and the bite is uneven. **Phase 6 is
largely a configuration and integration job now** — LiteLLM's MIT core does the hard part and
`OD-21` already picked it, so the remaining ~9 tasks are the ones only we can do: lifecycle,
classification, ZDR proof, provenance and the cockpit's failure behaviour. **Phase 8 gets a
serious but narrower gift**: the warm pool and the sandbox lifecycle from Agent Sandbox, plus
a set of standard platform components, against a phase that is mostly Terraform, networking,
identity and hosting that nobody has written for us.

Neither adoption changes the locked architecture. Both fit behind interfaces that already
exist in the tree. And the biggest risk found in this survey is not a licence — it is a token
counter that goes to the wrong continent.

---


# Fourth pass — 2026-09-28, independent re-verification and new findings

This pass re-read the decisive licences from source without trusting the passes above, closed
two of the open gaps, **corrected one wrong claim**, and found material in
`kubernetes-sigs/agent-sandbox` that the earlier passes did not reach. Read-only throughout.

## Corrections to what is written above

1. **`warmpool-burst-test.yaml` does not exist.** Passes 1–3 asserted the repository ships a
   burst-test manifest and built recommendation 3 and open-gap 5 on it. It is not there.
   `examples/kata-aks/` contains exactly `README.md`, `router.yaml`, `sandboxclaim.yaml`,
   `sandboxtemplate.yaml`, `sandboxwarmpool.yaml`, plus `agent/` and `client/` directories
   (listed at `github.com/kubernetes-sigs/agent-sandbox/tree/main/examples/kata-aks`, read
   2026-09-28). No file anywhere under `examples/` has `burst`, `load` or `test` in its name.
   **Consequence: the burst harness is ours to write.** The nearest upstream starting point is
   `examples/hpa-swp-scaling/create-claim.py` plus its `test_create_claim.py`, which drive
   claim creation at a measured rate.
2. Everything else the earlier passes marked verified and that this pass re-checked **held**.

## Licences re-read from source on this pass

| Project | File read | Result |
|---|---|---|
| **LiteLLM** | `raw.githubusercontent.com/BerriAI/litellm/main/LICENSE` | **MIT**, © 2023 Berri AI, with the carve-out verbatim: *"All content that resides under the `enterprise/` directory of this repository, if that directory exists, is licensed under the license defined in `enterprise/LICENSE`."* Confirms row 1. |
| **Portkey AI Gateway** | `raw.githubusercontent.com/Portkey-AI/gateway/main/LICENSE` | **MIT**, © 2024 Portkey, Inc. **No enterprise or plugin carve-out in the licence file** — cleaner than LiteLLM's on that one axis. Confirms row 13. |
| **Bifrost** | `raw.githubusercontent.com/maximhq/bifrost/main/LICENSE` | **Apache-2.0**, © 2025 **H3 Labs Inc.** (not "Maxim"), standard text, no additional use grant, no competitor restriction, no carve-out. Confirms row 15. Note the copyright holder differs from the GitHub org — worth one line in a licence register. |
| **Agent Sandbox** | `raw.githubusercontent.com/kubernetes-sigs/agent-sandbox/main/LICENSE` (pass 3) | **Apache-2.0**. Confirms row 2. |

**The licence conclusion for phase 6 is unchanged and now triple-sourced: LiteLLM's MIT core is
usable, its `enterprise/` directory is not.** Portkey being carve-out-free is the one fact that
would matter if LiteLLM ever moves a feature we depend on behind the wall — it is a real
fallback, not a theoretical one.

## Gap 6 closed — the Prometheus exporter is NOT licence-gated

Two independent reads on 2026-09-28:

- `docs.litellm.ai/docs/proxy/prometheus` describes `/metrics` as an ordinary proxy feature and
  **contains no sentence about enterprise, premium or licence**.
- `raw.githubusercontent.com/BerriAI/litellm/main/litellm/integrations/prometheus.py` contains
  **no `premium_user` check, no `_license_check`, and raises no error for non-enterprise users**.

The metrics that carry our attribution are `litellm_spend_metric`,
`litellm_total_tokens_metric`, `litellm_input_tokens_metric` and `litellm_output_tokens_metric`,
each labelled `hashed_api_key`, `api_key_alias`, `team`, `team_alias`, `user`, `end_user` and
`model`. **`team` plus `end_user` plus `api_key_alias` is enough to carry per-user, per-team and
per-project attribution (`OD-19`) without writing our own metering.** Keep the CI assertion
anyway: assert against the pinned tag, not `main`.

## Gap 2 re-confirmed independently — the AU residency bug is live

`github.com/BerriAI/litellm/issues/31149`, read directly on 2026-09-28:
**state OPEN, opened 24 June 2026**, titled *"[Bug]: Anthropic `/v1/messages/count_tokens`
ignores custom `api_base` despite repeated production reports"*. The body reports the endpoint
going to `api.anthropic.com` regardless of the configured base, breaking self-hosted and
gateway deployments, reported repeatedly without resolution.

**Nothing has changed. The conformance test stays recommendation 1.** The finding is not that
LiteLLM is unusable; it is that one endpoint on Claude Code's hot path bypasses our routing, and
an "AU data only" project (`OD-24`) would leak to the Global endpoint silently.

## New finding 1 — Agent Sandbox ships a per-session credential broker, and it is our exact OD-19 shape

`examples/containarium-execution-scoped-token/` (Apache-2.0, same repository, read 2026-09-28)
is a working reference implementation of the thing phase 6 was going to invent. Its README
states the pattern in three lines that could have been lifted from `OD-19`:

> *"The real key is never in the box"* — it stays in a credential proxy (the gateway).
> The token reaches *"one process, not the pod"*, so a later process does not inherit it.
> *"The token is revoked at `ExitEvent`, not left to expire."*

Files: `gateway.yaml`, `sandbox.yaml`, `networkpolicy.yaml`, `runner/`, `Dockerfile`,
`kind-config.yaml`, `run-test-kind.sh`. The token is injected through the sandbox runtime's
`ProcessConfig.env_vars` at process start; the example uses a deliberately long 30-minute TTL
precisely to prove that *revocation on exit*, not expiry, is what bounds the credential.

**What it replaces.** Phase 6's per-session token broker design — the "scoped per-session
token, never an org key" half of `OD-19` — and the NetworkPolicy that makes it a network fact
rather than a convention. That is a design and a test harness, not a dependency: it is a `kind`
example, so **vendor the pattern and the NetworkPolicy, write the broker in Go ourselves**,
against LiteLLM's virtual-key API instead of their toy gateway.

**Why it matters more than it looks.** It answers the one question our design had left open:
*what ends a session credential's life.* Expiry alone leaves a live org-scoped path open for the
remainder of the TTL after an agent dies; revocation at process exit closes it. LiteLLM's
`/key/delete` is the equivalent call on our side. **Add "revoke on session end, not expiry only"
to phase 6's acceptance criteria**, and make the provenance ledger record the revocation.

**Verdict: vendor** (pattern + NetworkPolicy, with attribution), **for the ~2 phase 6 tasks that
define session-credential lifecycle.**

## New finding 2 — `SandboxWarmPool` has a scale subresource, so the pool sizes itself

`examples/hpa-swp-scaling/` (read 2026-09-28) drives a `SandboxWarmPool` **directly from an
HPA** — `REFERENCE: SandboxWarmPool/python-sdk-warmpool` — against the controller's own
Prometheus counter `agent_sandbox_claim_creation_total`, holding a target claim rate (the
example uses 0.5 claims/second). Files: `hpa.yaml`, `sandboxwarmpool.yaml`,
`python-sandbox-template.yaml`, `pod-monitoring.yaml`, `create-claim.py`,
`test_create_claim.py`.

**This changes the KEDA row (row 6) from "adopt" to "probably unnecessary".** The warm pool
exposing `scale` means a plain HPA on a claim-rate metric is enough; KEDA earns its place only
if we need scale-to-zero out of hours or a scaler HPA cannot express. The example is written for
GKE's Managed Prometheus and its custom-metrics adapter, so **on AKS the substitution is Azure
Monitor managed Prometheus plus `prometheus-adapter` or KEDA's Prometheus scaler** — which is
the one honest reason to keep KEDA on the list.

**Revised verdict for row 6 (KEDA): adopt only as the metrics bridge, not as the pool autoscaler.**
Phase 8 tasks affected: warm-pool sizing. **Saving is small but the design question is answered:
size the pool from claim arrival rate, not from a fixed replica count or a guess.**

## New finding 3 — the snapshot warm-start trick exists upstream and we CANNOT use it

`examples/podsnapshot-golden-warmpool/` snapshots one "primer" sandbox — *"memory and
filesystem"* — so every subsequently pre-warmed sandbox boots **already restored from that
snapshot**, skipping the expensive initialisation once per pool instead of once per sandbox.
Files `00-storageconfig.yaml` through `50-refresh-claim.yaml` plus `run-test-gke.sh`.

**Its prerequisites rule it out for us**: a **GKE** cluster (Autopilot or Standard ≥ 1.35.3)
with Pod Snapshots enabled, and a **gVisor** node pool. Pod Snapshots is a GKE feature and the
example's runtime is gVisor, not Kata. There is no AKS equivalent.

**This is a negative finding worth recording, because it is the fastest known answer to open
risk 1 ("Kata cold start is seconds") and it is closed to us by `OD-18`'s choice of Azure.**
The honest position: on AKS our only cold-start levers are (a) pre-created pods in a warm pool,
(b) image pre-pull (`kube-fledged`, row 5), and (c) node headroom (row 11). We pay the microVM
boot once per pooled sandbox and hide it behind the pool; we cannot amortise the *in-guest*
initialisation the way the snapshot pattern does. **If in-guest setup — CLI install, npm cache,
repo clone warmth — turns out to dominate, bake it into the sandbox image rather than hoping for
a snapshot feature.** **Verdict: study-only, with the reason it is excluded recorded.**

## New finding 4 — the kata-aks example confirms our locked constraints, and adds two

Read verbatim from `examples/kata-aks/README.md` on 2026-09-28:

- **`runtimeClassName: kata-vm-isolation`**, with the note that *"newer Microsoft documentation
  sometimes calls it `kata-mshv-vm-isolation` — same feature, different label."* **Pin the name
  in one constant and assert it at admission**; a silent rename is a whole-fleet outage.
- ***"Azure Linux only. The `kata-vm-isolation` RuntimeClass is only registered on Azure Linux
  node pools. Ubuntu pools cannot host Kata sandboxes on AKS today."*** Confirms `CLAUDE.md`.
- **Gen2 nested-virt SKU required; *"B-series burstable SKUs and most ARM SKUs do not"*** support
  it. Confirms `CLAUDE.md`, and rules out the cheap SKUs somebody will eventually propose.
- ***"Resource requests are not optional"*** — confirms the `PROGRAM.md` Kata constraint that
  the guest VM is sized from the requests.
- **NEW, and a real constraint: *"the agent listener lives inside the Kata guest VM, not on the
  host"*, so `kubectl port-forward` does not reach it.** The example solves it with a
  header-aware proxy (`router.yaml`) in front of a single public IP, routing a request tagged
  for a person to that person's pod.
- **NEW: *"History stored in-pod memory means restarting the pod (or the warm pool replacing it)
  wipes the history."*** A warm pool **replaces** pods. Anything the cockpit treats as durable
  must not live in the sandbox's memory.

**Why the port-forward caveat matters to us specifically.** Our `katabe.Attach` dials
`backend.Dial(ctx, sb.Endpoint, token)` — the pod's own endpoint — so it does not depend on
`kubectl port-forward` and is unaffected *in-cluster*. But the desktop cockpit is **outside** the
cluster. **Phase 8 needs the `router.yaml` shape — an authenticated, session-addressed ingress in
front of sandboxd — and no task on the current list obviously owns it.** That is a gap the survey
should hand to the owner, not a saving.

## New finding 5 — Agent Sandbox fits our existing backend interface with no architecture change

Checked against the tree on 2026-09-28 (read-only). `control-plane/internal/backend/backend.go:289`
defines the whole seam:

```go
type Backend interface {
	Kind() Kind
	Create(ctx context.Context, spec Spec) (Sandbox, error)
	Get(ctx context.Context, id string) (Sandbox, error)
	List(ctx context.Context) ([]Sandbox, error)
	Attach(ctx context.Context, id, token string) (*websocket.Conn, error)
	Destroy(ctx context.Context, id string) error
	Close() error
}
```

`katabe` implements it today by creating a **Pod** directly (`kata.go:94`) and waiting for it to
schedule (`waitScheduled`, `kata.go:142`). The Agent Sandbox model substitutes cleanly:

| Our method | Today (pod-direct) | With Agent Sandbox |
|---|---|---|
| `Create` | create Pod, `waitScheduled` | create `SandboxClaim`, wait for it to **bind an already-warm Sandbox** |
| `Get` / `List` | read Pods | read `Sandbox` / `SandboxClaim` objects |
| `Attach` | `backend.Dial(sb.Endpoint, token)` | **unchanged — stays ours** |
| `Destroy` | delete Pod + token Secret | delete the claim; the controller reclaims |

**`Create` is where the whole saving lands**, because `waitScheduled` is exactly the wait a warm
pool removes. **`Attach` and our `sandboxd` stay ours** — Agent Sandbox has no PTY-streaming
daemon that replaces `sandboxd/internal/ptysession` + `ring` + `wire`; its runtime API is a
process/exec surface, not a terminal multiplexer with a scrollback ring.

**So the adoption is surgical: one new backend implementation beside `katabe`, roughly the size
of `katabe` itself, and nothing above the interface moves.** That is the strongest argument in
this document for row 2, and it is now checked against our code rather than asserted.

**One caution.** Agent Sandbox's API is at **`v1beta1`, with a published `v1alpha1 → v1beta1`
migration guide** — it has already broken its API once. Pin the CRD version and the controller
tag together, and treat a version bump as a task, not a dependabot merge.

## New finding 6 — the examples directory is itself the maturity evidence the survey lacked

Open gap 7 above says no reference customer was found. That is still true, but
`examples/` holds **62 subdirectories** (listed 2026-09-28), and their names are a better
signal than a star count because each is somebody's integration:
`kata-aks`, `kata-aks-sandbox`, `openclaw-kata-aks-sandbox`, `kata-gke-sandbox`,
`firecracker-sandbox`, `openclaw-gvisor-sandbox`, `demo-cilium-egress`,
`composing-sandbox-nw-policies`, `network-policy-api-sandbox`, `policy`, `sandbox-ksa`,
`irsa-simulation-localstack`, `keda-scale-to-zero`, `hpa-swp-scaling`, `kueue-agent-sandbox`,
`latebind-storage-gke-sandbox`, `manual-pdb`, `webhook-inject-timestamp`, `sandboxd-sandbox`,
`vscode-sandbox`, `playwright-sandbox`, `chrome-sandbox`, `mcp-server-sandbox`,
`agentclientprotocol`, `windows-sandbox`, and more.

Three of those corroborate rows already in the table and are worth reading before writing the
equivalent ourselves: **`demo-cilium-egress` and `composing-sandbox-nw-policies`** (row 4,
Cilium egress default-deny), **`policy`** (row 3, admission policy), and **`sandbox-ksa` +
`irsa-simulation-localstack`** (row 7, workload identity into a sandbox).

**Four Kata-specific examples and a dedicated AKS one is the evidence that this project takes our
exact deployment seriously.** It does not make it battle-tested at our scale — gap 7 stays open —
but it moves row 2 from "promising" to "the intended use case".

## Fourth-pass fetch log

**Blocked by the corporate proxy on this pass: none.** `raw.githubusercontent.com`,
`github.com`, `docs.litellm.ai` and `agent-sandbox.sigs.k8s.io` all resolved and returned
content. `api.github.com` was not used on this pass (pass 3 exhausted the unauthenticated
budget), so star counts and last-push dates are still the earlier passes' numbers.

Non-proxy failures, recorded so nobody repeats them: none on this pass. One page
(`agent-sandbox.sigs.k8s.io/docs/`) returned an index that does not itself document `sandboxd`;
the `sandboxd` detail quoted above comes from the `containarium-execution-scoped-token` README,
not from the docs site.

## What this pass changes about the recommendation

Nothing in the verdicts is reversed. Three refinements:

1. **Row 6 (KEDA) is downgraded** from "sizes the warm pool" to "the AKS metrics bridge", because
   `SandboxWarmPool` has a scale subresource and a plain HPA drives it.
2. **A new vendor candidate is added**: the execution-scoped-token pattern, for phase 6's session
   credential lifecycle — and it adds an acceptance criterion we did not have
   (**revoke on session end, not expiry alone**).
3. **A new phase 8 gap is surfaced, not filled**: an authenticated, session-addressed ingress
   between the desktop cockpit and `sandboxd`, because the Kata guest is not reachable by
   `kubectl port-forward` and the desktop is outside the cluster. Upstream's answer is
   `router.yaml`; ours does not exist yet.

**The bottom line above still holds.** Phase 6 is a configuration-and-integration job on an
MIT core whose licence this pass read for a third time; phase 8 gets a real but narrower gift,
and the one thing that would have solved Kata cold start outright — pod snapshots — is a GKE
feature we cannot have.

---

# Fifth pass — 2026-09-28: the first-party gateway, the `count_tokens` escape hatch, two new sandbox candidates

This pass went looking for what the first four missed and found one thing that changes how
phase 6 should be argued, one first-party sentence that de-fangs the survey's biggest risk,
and two permissively-licensed sandbox components nobody had listed. Nothing below contradicts
the adopt/reject verdicts above; two of them get sharper.

## New finding 1 — Anthropic now ships its own self-hosted gateway, and it is inside the `claude` binary

**This is not an open-source candidate. It is the reason phase 6's argument needs one more
paragraph.**

> "Claude apps gateway is a self-hosted service that sits between your developers' Claude Code
> clients and your model provider. Developers sign in with your corporate identity provider
> (IdP) instead of holding API keys or cloud credentials. The gateway holds the upstream
> credential, enforces model access and managed settings by IdP group, and relays usage
> telemetry to your own observability stack."
> **"It is included in the `claude` binary, so the same executable that runs Claude Code on a
> laptop runs the gateway server with `claude gateway --config gateway.yaml`."**
> — https://code.claude.com/docs/en/claude-apps-gateway (read 2026-09-28)

Read the feature list against `OD-19`, `OD-23` and `OD-30` and it is uncomfortable reading,
because it is most of them: OIDC sign-in against any compliant IdP (our Keycloak realm is
one), **per-user and per-group spend limits**, server-side model access enforcement,
**managed settings delivered per IdP group** — which is `OD-23`'s policy-injection seam as a
product feature — Postgres for auth state, OTLP telemetry fan-out stamped with user identity,
and upstreams for Amazon Bedrock, Claude Platform on AWS, Google Cloud, Microsoft Foundry and
the Anthropic API. Its stated purpose is data residency, which is `OD-19`'s AU path.

**It still does not displace LiteLLM for us, and the reasons are structural, not preferential:**

1. **It serves Claude Code and Claude Desktop. We run four harnesses.** `CLAUDE.md` locks
   "unmodified Claude Code / Codex / OpenCode / Gemini CLI, plus plain shells". A gateway that
   only speaks to Anthropic's own clients cannot be the single inference plane, and two
   inference planes is worse than one imperfect one.
2. **It is proprietary.** It ships in Anthropic's binary under Anthropic's terms. This survey's
   licence rule is about what we may adopt into a product KPMG ships to customers who run it on
   their own cloud; a first-party binary is a vendor dependency decision, not an OSS adoption,
   and I could not verify what entitlement it requires (see gaps below).
3. **Its clients must reach it at a private address.** "Claude Code only connects to a gateway
   whose address is private… RFC 1918, link-local, CGNAT `100.64.0.0/10`, IPv6 ULA `fc00::/7`,
   or loopback", *"because a trusted gateway can push settings that run commands on developer
   machines"*. In-cluster sandboxes satisfy that trivially; laptop local mode (`OD-20`) needs
   the VPN up. Worth knowing before it is a support ticket.
4. **Named losses on gateway sessions**: server-side **web search is disabled** ("the CLI can't
   see which upstream provider the gateway routes to"), 1-hour cache TTL is off (5-minute only),
   first-party optimisations such as global cache scope and token-efficient tools are off,
   Remote Control is unavailable, and feature-flag-gated commands are skipped. **Several of
   these apply to any `ANTHROPIC_BASE_URL` gateway, LiteLLM included** — they are properties of
   not being `api.anthropic.com`. Price that into phase 6's expectations now rather than
   discovering it in a demo.
5. **Operationally it is thin where we are thick**: no admin UI ("configuration is the YAML
   file; redeploy to change it"), no Helm chart, Linux only, **one OIDC issuer per gateway**
   (multi-tenant means multiple instances — relevant to the customer-hosted tier).

**Verdict: study-only, and revisit at the customer-hosted tier (phase 9).** Where it earns its
place today is as *the reference specification for what a Claude-compatible gateway must do* —
Anthropic wrote our phase 6 acceptance criteria for us, and the next finding is the proof.

## New finding 2 — the AU data-residency risk has a first-party escape hatch, and it is one line of config

The survey's headline risk (section "The one integration risk that could break AU data
residency") is LiteLLM's `/v1/messages/count_tokens` ignoring a custom `api_base` and calling
`api.anthropic.com` directly — a silent Global leak on an "AU data only" project. Issues
#31149 and #42073 are still open. **The mitigation does not depend on that bug being fixed:**

> "Token-counting endpoints are the only optional ones: when they're absent, Claude Code falls
> back to a character-based estimate of context usage."
> — https://code.claude.com/docs/en/llm-gateway-protocol (read 2026-09-28)

and, in the same table:

> Token counting — "No error: Claude Code falls back to a character-based estimate, so
> `/context` shows approximate counts."

**So the AU control is: do not serve `/v1/messages/count_tokens` on the AU route.** Return
404 at the ingress in front of LiteLLM for any session whose project carries the AU data
classification, and the harness degrades to an approximate context meter. The cost is a
cosmetic inaccuracy in `/context`; the benefit is that the leak becomes structurally
impossible rather than dependent on an upstream fix.

**What to write into phase 6, concretely:**

1. An ingress rule (or LiteLLM route exclusion) that 404s `POST /v1/messages/count_tokens` on
   AU-classified sessions.
2. A conformance test that asserts the 404, and a second that asserts no request originating
   from an AU session reaches `api.anthropic.com` — belt and braces, because the test is the
   compliance evidence, not the config.
3. On the Global route, leave `count_tokens` served; exact counts are worth having and there
   is no residency question.

This also **re-prices the risk in the headline**: it is no longer "a bug that can break AU
residency", it is "a bug we route around in one rule". Keep the test; drop the alarm.

## New finding 3 — the gateway contract Claude Code actually requires, which becomes phase 6 acceptance criteria

All from https://code.claude.com/docs/en/llm-gateway-protocol (read 2026-09-28). Each line is
a test we should write rather than a fact to remember:

| Contract | Consequence for us |
|---|---|
| Endpoints for the Anthropic Messages format are `/v1/messages` and `/v1/messages/count_tokens` (optional) | The whole surface is two paths. LiteLLM serves them. |
| Forward `anthropic-beta` **verbatim**, never allowlist values — "the set changes with Claude Code releases" | A gateway that filters beta headers breaks features on CLI upgrade. Canary test, pinned to the sandbox image's CLI version, exactly as `OD-8` demands for transcript adapters. |
| Return `retry-after` as **integer seconds**, not an HTTP date; a value above 60 stops retries and surfaces the error | Budget-exhausted and throttle behaviour in the cockpit (`AGENTIC-UX.md`) is driven by this header. |
| Forward `anthropic-ratelimit-unified-*` unchanged | This is the same header family `PROGRAM.md`'s old phase 6 wanted to parse for quota. It survives `OD-19` — pass it through and the cockpit can still show headroom. |
| Model discovery is `GET /v1/models?limit=1000`, **3-second timeout, any redirect is treated as failure** | If our ingress redirects (even http→https) the `/model` picker silently empties. A one-line test prevents a confusing bug. |
| Claude Code prepends a system-prompt attribution block; `api.anthropic.com` strips it, other upstreams see it. Keep it as its own array entry, or set `CLAUDE_CODE_ATTRIBUTION_HEADER=0` | If LiteLLM ever reshapes system content (guardrail hooks do), set the variable at the client rather than rewriting server-side. |
| `CLAUDE_CODE_USE_BEDROCK` / `_USE_VERTEX` / `_USE_FOUNDRY` "have no effect behind an `ANTHROPIC_BASE_URL` gateway" | **Confirms `OD-19`'s shape from the client side**: Global vs AU is a server-side routing decision on the model alias, and a sandbox cannot override it with an environment variable. That is the property the data classification needs. |
| `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` stops pre-release capabilities and their body fields | The compatibility lever when LiteLLM lags a Claude Code release. Cheaper than pinning the CLI back. |

**Add one task to phase 6 that was not on the list: a gateway-conformance canary.** It runs the
seven rows above against the pinned LiteLLM tag and the pinned CLI version, and fails the build
when either moves. Everything else in phase 6 assumes this contract holds.

## New finding 4 — two permissively-licensed sandbox components the first four passes missed

Both verified from the repository's own licence file on 2026-09-28, and both are small enough
to adopt without adopting an architecture.

### `anthropic-experimental/sandbox-runtime` — Apache-2.0

- **Licence**: Apache License 2.0, *"Copyright 2025 Anthropic"* —
  https://raw.githubusercontent.com/anthropic-experimental/sandbox-runtime/main/LICENSE
  (read 2026-09-28). GitHub API `license.spdx_id` agrees: `Apache-2.0`.
- **Maturity**: 5,365 stars, last push **2026-09-27** — yesterday.
- **What it does**, in its own description: *"A lightweight sandboxing tool for enforcing
  filesystem and network restrictions on arbitrary processes at the OS level, without requiring
  a container."*
- **What phase 8 work it replaces**: the **second ring inside the microVM**. Kata gives us a
  kernel boundary between sessions; it gives us nothing between the agent process and the rest
  of *its own* VM — the session token, the mounted worktree of another lane, the metadata
  endpoint. This is the piece that makes "never an org key in a sandbox" (`OD-19`) a property
  of the process, not only of the pod. It is also the cheapest way to get a real boundary in
  **local Docker mode**, where there is no microVM at all.
- **Integration cost**: it wraps a process; nothing in our locked architecture moves. It goes
  into the sandbox image and the session's launch command. Days, not weeks.
- **Risks, stated honestly**: the `anthropic-experimental` namespace carries no stability
  promise, and a repository that is pushed to daily is a repository that moves. Pin a commit,
  not a branch. Linux and macOS only — a Windows laptop runs the local backend under WSL2
  anyway, but that dependency becomes load-bearing.
- **Verdict: adopt**, as a sandbox-image component, pinned, behind a feature flag.

### `nestybox/sysbox` — Apache-2.0

- **Licence**: Apache License 2.0, *"Copyright 2020 Nestybox, Inc."* —
  https://raw.githubusercontent.com/nestybox/sysbox/master/LICENSE (read 2026-09-28).
- **Maturity**: 3,884 stars, last push **2026-09-15**. Nestybox was acquired by Docker; the
  project is Apache-2.0 and still moving, but the commercial edition split is the shape this
  survey has flagged twice already — **check what is in the CE tree before depending on a
  feature**, the same discipline as LiteLLM's `enterprise/`.
- **What it does**: a container runtime that runs **unprivileged** containers able to host
  systemd, Docker and Kubernetes inside them, without `--privileged`.
- **What phase 8 / phase 2 work it replaces**: the honest answer for **local Docker mode**.
  Agents build and run containers; today that means either `--privileged` (a hole straight to
  the laptop) or a docker socket mount (the same hole with extra steps). Sysbox is the
  permissively-licensed way to give a local sandbox docker-in-docker without either.
- **Integration cost**: a runtime installed on the host and a `runtimeClassName`/`--runtime`
  choice. On **AKS with Kata it is redundant** — the microVM already provides the boundary,
  and two runtimes is a complication we do not need.
- **Risks**: Linux hosts only (on Windows, inside WSL2); vendor-owned; the CE/commercial split
  needs reading before adoption.
- **Verdict: study-only now, adopt if and when local mode needs docker-in-docker.** Do not put
  it on the cloud path.

## Pin targets, since "pin the tag" was advice without a number

| Component | Pin | Published | Evidence |
|---|---|---|---|
| LiteLLM | **`v1.102.1`** | 2026-09-23 | `api.github.com/repos/BerriAI/litellm/releases/latest`, read 2026-09-28. Release notes lead with cosign image signing and a recommendation to pin the **commit hash** rather than the tag — do that; it is a supply-chain win we get for free. |
| Agent Sandbox | **`v1.0.4`** | 2026-09-24 | `gh api repos/kubernetes-sigs/agent-sandbox/releases`, read 2026-09-28. `v1.0.2` 2026-09-11, `v1.0.3` 2026-09-17, `v1.0.4` 2026-09-24 — a weekly cadence, which is activity and also churn. Pin, and re-qualify deliberately. |
| Agent Sandbox repo state | 4,044 stars, 205 open issues, last push 2026-09-25, `license.spdx_id: Apache-2.0` | — | `gh api repos/kubernetes-sigs/agent-sandbox`, read 2026-09-28 |

## Fifth-pass fetch log, and what it could not answer

**Fetched and read on 2026-09-28**: `code.claude.com/docs/en/llm-gateway`,
`/llm-gateway-protocol`, `/claude-apps-gateway`;
`raw.githubusercontent.com/anthropic-experimental/sandbox-runtime/main/LICENSE`;
`raw.githubusercontent.com/nestybox/sysbox/master/LICENSE`;
`api.github.com/repos/BerriAI/litellm/releases/latest`; and via `gh api` (authenticated, which
is how the third pass's rate-limit wall was avoided) `repos/kubernetes-sigs/agent-sandbox`,
`repos/kubernetes-sigs/agent-sandbox/releases`, `repos/nestybox/sysbox`,
`repos/anthropic-experimental/sandbox-runtime`.

**Blocked or failed on this pass:**

| URL | Result | Worked around? |
|---|---|---|
| `docs.claude.com/en/docs/claude-code/llm-gateway` | 301 to `code.claude.com/docs/en/llm-gateway` — a redirect, **not** a proxy block | Yes — fetched the redirect target. |
| `api.github.com/repos/kubernetes-sigs/agent-sandbox/releases/latest` (first attempt) | `net/http: TLS handshake timeout` — transient, through the corporate proxy | Yes — the same call succeeded on retry via the releases list. |

**Nothing was blocked by Netskope on this pass.** The only proxy block in the whole survey
remains `opentelemetry.io`, recorded above.

**Still not verified after five passes:**

1. **What entitlement the Claude apps gateway requires.** The docs describe deployment, not
   commercial terms. It is Anthropic-proprietary software in any case, so it is a vendor
   question for the owner, not an OSS-adoption question — but if phase 9's customer-hosted tier
   is ever argued on it, the terms have to be read first.
2. **Whether LiteLLM's `/v1/models` satisfies the 3-second, no-redirect discovery rule behind
   our ingress.** LiteLLM serves the route; the timing and the redirect behaviour are a property
   of *our* ingress, and only a spike answers it.
3. **The warm-pool burst behaviour on nested-virt SKUs** — unchanged from the fourth pass, and
   still the one unknown that decides 8–12 phase 8 tasks.
4. **Nothing here was run.** Five passes of documentation, licence text and API metadata. No
   cluster, no proxy, no test.

## What the fifth pass changes about the recommendation

- **Phase 6: unchanged in direction, sharper in content.** LiteLLM (MIT core, pinned at
  `v1.102.1` by commit hash) is still the answer, and the reason is now sharper than "OD-21 said
  so": Anthropic's own gateway proves the contract, and cannot take the job because it serves
  only Anthropic's clients while we run four harnesses. Two tasks join the ~9 that remain ours:
  the **gateway-conformance canary** and the **AU `count_tokens` 404 rule**.
- **The headline risk is downgraded.** The `count_tokens` residency leak is a routing rule plus
  a test, not a dependency on an upstream fix. It stays first in the order of work because it is
  cheap and it is compliance evidence — but it is no longer a reason to hesitate over LiteLLM.
- **Phase 8 gains one adopt and one deferred candidate**: `sandbox-runtime` (Apache-2.0) as the
  in-VM second ring and the local-mode boundary, `sysbox` (Apache-2.0) held for the day local
  mode needs docker-in-docker.
- **The arithmetic does not move.** Roughly 18 of phase 6's 27 tasks and 8–12 of phase 8's 35
  are still the realistic open-source bite. What moved is confidence: every decisive licence has
  now been read from source twice by different passes, and the two adoption decisions survive.
