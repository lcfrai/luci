# Licence audit — Phase 6 / Phase 8 candidates (sandbox isolation, warm pools, inference gateway, environment)

**Audited 2026-09-28** by an adversarial licence auditor against
`docs/superpowers/planning/2026-09-28-oss-adoption/06-platform.md`.

**Method.** Every licence below was fetched as a raw blob from the actual repository over HTTPS
(`curl -sL --ssl-no-revoke`) on **2026-09-28**, and the identifying line read first-hand. Nothing
here is taken from a GitHub badge, a README summary, a blog post or memory. Where a claim about a
relicence, an enterprise carve-out or a sub-package licence mattered, a second artefact was fetched
to prove it. Where a source could not be reached, the row says **UNVERIFIED** and says why.

**Clean-room discipline (`OD-11`).** No AGPL or GPL *source* was opened. Two candidates ship a
GPL-2.0 licence *text* inside an eBPF subtree (Cilium); that file is licence text, not program
source, and reading it is what an audit is for.

**Rate-limit note.** The GitHub REST API refused every call from this machine's egress IP
(`API rate limit exceeded for 24.239.128.117`), so star counts and release dates below come from the
rendered repository page via `WebFetch`, and are marked as such. Raw blob fetches
(`raw.githubusercontent.com`) were unaffected and are the basis of every licence verdict.

---

## 1. Verdict table

`yes` = permissive, ship it · `conditional` = usable, but a named obligation or boundary attaches ·
`no` = do not take it into the product.

| # | Candidate | Researcher claimed | **Verified (SPDX)** | Blob read 2026-09-28 | Verdict | Obligation / condition |
|---|---|---|---|---|---|---|
| 1 | **LiteLLM** `BerriAI/litellm` | MIT core; `enterprise/` proprietary (BerriAI Enterprise License) | **MIT (root) + BerriAI Enterprise License (`enterprise/`)** — confirmed verbatim | `BerriAI/litellm@main/LICENSE` (1,419 B); `…/enterprise/LICENSE.md` (2,223 B) | **conditional** | Root file reads: *"All content that resides under the `enterprise/` directory of this repository, if that directory exists, is licensed under the license defined in `enterprise/LICENSE`. Content outside of the above mentioned directories … is available under the MIT license"*, © 2023 Berri AI. The carve-out file is *"The BerriAI Enterprise license (the 'Enterprise License') … Copyright (c) 2024 - present Berrie AI Inc."* **See §2.1 — the split is not confined to a directory you can delete.** |
| 2 | **kubernetes-sigs/agent-sandbox** | Apache-2.0 | **Apache-2.0** — stock 11,357-byte Apache text, no appendix, no carve-out | `kubernetes-sigs/agent-sandbox@main/LICENSE` | **yes** | Apache §4(a)–(c): ship the licence, mark modified files, keep copyright headers. §4(d) does not bite — no `NOTICE` at root. Kubernetes-SIG governance (CNCF), DCO not CLA, so no single owner can relicense. |
| 3 | **anthropic-experimental/sandbox-runtime** | Apache-2.0, © 2025 Anthropic | **Apache-2.0** — appendix filled: *"Copyright 2025 Anthropic"* | `anthropic-experimental/sandbox-runtime@main/LICENSE` (11,339 B) | **conditional** — see §2.3 | Apache §4. **The licence is clean; the runtime dependency is not.** On Linux this wraps `bubblewrap` (LGPL-2.0-or-later) and on macOS `sandbox-exec`. We would invoke bubblewrap as a **separate process**, never link it — that is outside LGPL §6 entirely — but the sandbox *image* would ship an LGPL binary, so the image needs an LGPL notice and a written offer of source. |
| 4 | **Kyverno** `kyverno/kyverno` | Apache-2.0 | **Apache-2.0** — stock text, no carve-out | `kyverno/kyverno@main/LICENSE`; `…/NOTICE` present (HTTP 200) | **conditional** (notice only) | Apache §4(d) **does** bite here: a root `NOTICE` exists, so if we redistribute Kyverno (even as a chart in our AKS baseline) we must reproduce its NOTICE text. Nirmata sells a commercial "Enterprise for Kyverno" — that is a **separate product**, not a carve-out inside this repo. CNCF-governed. |
| 5 | **Cilium** `cilium/cilium` (as *Azure CNI powered by Cilium*) | Apache-2.0 | **Apache-2.0 at root — but the `bpf/` datapath is DUAL `GPL-2.0` / `BSD-2-Clause`**, which the researcher did not surface | `cilium/cilium@main/LICENSE`; `…/bpf/LICENSE.GPL-2.0` (HTTP 200, *"GNU GENERAL PUBLIC LICENSE Version 2, June 1991"*); `…/bpf/LICENSE.BSD-2-Clause` (HTTP 200, *"Copyright Authors of Cilium"*) | **yes — but only on the consumption model we actually use** | Two facts make this safe and both must be written into the plan. (a) The dual grant means **the BSD-2-Clause option is always available** — GPL never attaches unless we elect it. (b) We consume Cilium as a **Microsoft-managed AKS add-on**; we neither compile, link nor redistribute it. **Rule: never vendor a file out of `cilium/bpf/` into our tree.** See §2.4. |
| 6 | **kube-fledged** `senthilrch/kube-fledged` | Apache-2.0 at `main/LICENSE` | **Apache-2.0 — but the researcher's URL is wrong.** `main/LICENSE` returns **HTTP 404**; the default branch is `master`, and `master/LICENSE` returns 200 with stock Apache text | `senthilrch/kube-fledged@master/LICENSE` (200) — `@main/LICENSE` (404) | **yes** on licence — **flag on maintenance** | Apache §4. The wrong-branch 404 is not a licence defect, but it is the same *shape* of evidence the researcher treated as "no grant" for Daytona (row 23) and treated as "verified" here. Both readings were unchecked. Correct the citation before this goes to the owner. |
| 7 | **KEDA** `kedacore/keda` | Apache-2.0 | **Apache-2.0** — appendix filled: *"Copyright 2020 The KEDA Authors."* | `kedacore/keda@main/LICENSE` (11,412 B) | **yes** | Apache §4. CNCF graduated project, DCO-based, no CLA assigning copyright to a vendor — the relicence risk that hit HashiCorp and Redis is structurally absent. |
| 8 | **Azure Workload Identity** `Azure/azure-workload-identity` | MIT | **MIT** — *"MIT License … Copyright (c) Microsoft Corporation."* | `Azure/azure-workload-identity@main/LICENSE` (1,141 B) | **yes** | MIT notice. Microsoft-owned, so no third-party CLA risk; Microsoft could relicense future versions but cannot revoke the grant on versions already published. |
| 9 | **External Secrets Operator** `external-secrets/external-secrets` | Apache-2.0 | **Apache-2.0** — stock text | `external-secrets/external-secrets@main/LICENSE` (11,357 B); `…/go.mod` scanned for copyleft modules — none found | **yes** | Apache §4. CNCF incubating. There is a commercial **"ESO Enterprise"** from the maintainers' company — again a separate product, not a directory carve-out in this repo. |
| 10 | **OpenTofu** `opentofu/opentofu` | MPL-2.0 (file-level copyleft) | **MPL-2.0** — confirmed, and note the **dual copyright line**: *"Copyright (c) The OpenTofu Authors"* / *"Copyright (c) 2014 HashiCorp, Inc."* (16,048 B = MPL-2.0 text) | `opentofu/opentofu@main/LICENSE` | **conditional** | MPL §3.2: **any OpenTofu *file* we modify must be published under MPL-2.0.** Our own `.tf` configuration and our Go control plane are a "Larger Work" (§3.3) and stay proprietary. **Rule: use OpenTofu as an unmodified binary; never fork it.** The HashiCorp copyright line is the fork provenance — OpenTofu is the pre-BUSL Terraform lineage, which is exactly *why* it is usable and Terraform ≥1.6 is not. |
| 11 | **terraform-provider-azurerm** `hashicorp/terraform-provider-azurerm` | MPL-2.0 | **MPL-2.0** — *"Mozilla Public License Version 2.0 … Copyright (c) 2017 HashiCorp, Inc."* (16,761 B) | `hashicorp/terraform-provider-azurerm@main/LICENSE` | **conditional** — and the boundary must be stated, see §2.5 | MPL §3.2 as above. **The material point the researcher skipped: HashiCorp's August 2023 BUSL-1.1 relicence covered Terraform *core*, not the providers — this provider's `main` LICENSE is still MPL-2.0 today.** That split is a *current fact, not a guarantee*. HashiCorp holds a CLA and is now IBM-owned, so it *can* flip this repo at any future version. **Pin the provider version, record the SPDX at pin time, and re-verify on every bump.** |
| 12 | **Cluster overprovisioner** (pause-pod pattern) `kubernetes/autoscaler` | Apache-2.0 | **Apache-2.0** — stock text | `kubernetes/autoscaler@master/LICENSE` (11,358 B) | **yes** | Apache §4. In practice we take a *pattern* (a low-priority pause Deployment), which is an idea and carries no copyright at all; the Apache grant covers us if we also copy any manifest. |
| 13 | **Claude apps gateway** (`claude gateway`) | Anthropic-proprietary, not open source; entitlement terms unverified | **Not open source — confirmed by absence.** There is no source repository, therefore no `LICENSE` blob to read. Use is governed by Anthropic's Commercial Terms of Service, the same instrument that governs `anthropics/claude-agent-sdk-typescript` (*"© Anthropic PBC. All rights reserved."*) | No repository exists; docs page `code.claude.com/docs/en/claude-apps-gateway` | **no** (as an OSS adoption) | This is **not an OSS-licence question at all** — it is a procurement/contract question. Nothing in this audit clears it. If the platform team wants it, route it to KPMG legal as a commercial-terms review, not through the OSS rule. Reselling gateway access to regulated customers on their own cloud is exactly the use most vendor terms restrict. |
| 14 | **sysbox** `nestybox/sysbox` | Apache-2.0, © 2020 Nestybox, Inc. | **Apache-2.0** — appendix filled: *"Copyright 2020 Nestybox, Inc."* (10,760 B). Sub-repo `nestybox/sysbox-runc` also **Apache-2.0** | `nestybox/sysbox@master/LICENSE`; `nestybox/sysbox-runc@master/LICENSE` | **conditional** — the licence is fine, the *governance* is not | Apache §4. **Three facts the researcher did not surface.** (a) The repo is a **multi-repo umbrella** — the shipped runtime is `sysbox-runc` (an Apache-2.0 fork of `opencontainers/runc`), `sysbox-fs` and `sysbox-mgr`; verifying only the umbrella `LICENSE` verifies none of the binaries. `sysbox-runc` verified separately here. (b) **Sysbox Enterprise Edition existed and was proprietary**; the repo now states it *"is no longer offered as a standalone product but has instead been incorporated into Docker Desktop"* after Docker acquired Nestybox in May 2022 — so the CE/EE split is historical, not live, but **Docker owns the copyright and can relicense future versions**. (c) The repo states Sysbox is *"not officially supported by Docker"* and support is *"best effort"*. Evidence: repository page, 3.9k stars, read 2026-09-28. |
| 15 | **Agones** `googleforgames/agones` | Apache-2.0 | **Apache-2.0** — stock text (11,359 B) | `googleforgames/agones@main/LICENSE` | **yes** | Apache §4. CNCF sandbox, Google-originated but DCO/CNCF-governed. We would take the **fleet/warm-pool pattern**, not the code — patterns are uncopyrightable, so even the Apache obligation is only engaged if we copy a CRD or controller file. |
| 16 | **Portkey AI Gateway** `Portkey-AI/gateway` | MIT, © 2024 Portkey, Inc | **MIT** — *"MIT License … Copyright (c) 2024 Portkey, Inc"* (1,069 B, plain MIT, **no carve-out clause**) | `Portkey-AI/gateway@main/LICENSE`; `…/package.json` (`@portkey-ai/gateway` v1.15.2) | **yes** | MIT notice. **I checked specifically for the LiteLLM-shaped trap and it is absent**: the LICENSE is unmodified MIT with no "portions of this software" preamble and no `enterprise/` reference. Portkey does sell an "enterprise version … for org management, governance, security" — that is a **separate closed product**, not a directory inside this repo. Evidence: repository page, 13.1k stars, 1.3k forks, read 2026-09-28. |
| 17 | **Envoy AI Gateway** `envoyproxy/ai-gateway` | Apache-2.0 | **Apache-2.0** — stock text (11,357 B). **No root `NOTICE`** (HTTP 404), so Apache §4(d) does not engage | `envoyproxy/ai-gateway@main/LICENSE`; `…/NOTICE` → 404 | **yes** | Apache §4(a)–(c) only. CNCF/Envoy governance under the Linux Foundation, DCO not CLA — the cleanest governance of the three gateway candidates. |
| 18 | **Bifrost** `maximhq/bifrost` | Apache-2.0, © 2025 H3 Labs Inc. | **Apache-2.0** — appendix filled: *"Copyright 2025 H3 Labs Inc."* (11,308 B). `core/go.mod` = `github.com/maximhq/bifrost/core`, Go 1.27 | `maximhq/bifrost@main/LICENSE`; `…/core/go.mod` | **conditional** — adopt only with a watch, see §2.6 | Apache §4. Licence verified clean *today*. **But the open-core shape is the risk**: the README markets *"Enterprise Deployments … enterprise-grade, private deployments"* with clustering, guardrails and an MCP gateway behind a "Book a Demo" wall, and **H3 Labs Inc. is a single company that owns the whole copyright**. That is precisely the configuration that produced BUSL relicences at HashiCorp, Elastic and Redis. **Pin the version, record the SPDX at pin time, and treat an upstream relicence as a planned-for event, not a surprise.** 8.4k stars, read 2026-09-28. |
| 19 | **gVisor** `google/gvisor` | Apache-2.0 | **Apache-2.0 + per-file MIT + per-file BSD-3-Clause** — the LICENSE is 14,110 B, *not* the stock 11,357, because it appends two further grants: *"Some files carry the following license"* (MIT) and *"Some files carry the 'BSD' license"* (BSD-3-Clause, Google Inc. no-endorsement clause). All three are on the allowed list | `google/gvisor@master/LICENSE` (full text read, tail included) | **yes** | Apache §4 **plus** the MIT and BSD-3 notices if we ship any file carrying those headers. The researcher reported "Apache-2.0" flat and missed the tri-licence structure — harmless here because all three are permissive, but it is the same reading error that would have been fatal on a GPL appendix. **If we build a gVisor-based image, run a per-file SPDX scan, not a root-LICENSE scan.** |
| 20 | **Firecracker** `firecracker-microvm/firecracker` | Apache-2.0 | **Apache-2.0** — stock text (11,358 B) | `firecracker-microvm/firecracker@main/LICENSE` | **yes** | Apache §4. AWS-owned copyright. Note for the plan, not for the licence: Firecracker drives **KVM** through `ioctl` on `/dev/kvm`; the kernel is GPL-2.0 but the syscall boundary carries the Linux kernel's explicit user-space exception, so no GPL obligation reaches us. This is the same reasoning that clears Kata. |
| 21 | **SPIFFE / SPIRE** `spiffe/spire` | Apache-2.0 | **Apache-2.0** — stock text (11,358 B) | `spiffe/spire@main/LICENSE` | **yes** | Apache §4. CNCF graduated, DCO-based. |
| 22 | **Knative Serving** `knative/serving` | Apache-2.0 | **Apache-2.0** — stock text (11,358 B). **No root `NOTICE`** (HTTP 404) | `knative/serving@main/LICENSE`; `…/NOTICE` → 404 | **yes** | Apache §4(a)–(c). CNCF incubating. |
| 23 | **Daytona** `daytonaio/daytona` | *"NO LICENCE FILE PUBLISHED — raw LICENSE returns 404; no grant"* | **WRONG, AND DANGEROUSLY SO. Daytona is `AGPL-3.0`.** The 404 is real but means nothing: the repo is **archived**, and `LICENSE` at the release tags returns HTTP 200 with *"GNU AFFERO GENERAL PUBLIC LICENSE, Version 3, 19 November 2007"* — at **both** `v0.190.0` and `v0.100.0`, so there is no permissive earlier era to fall back to | `daytonaio/daytona@v0.190.0/LICENSE` (200, AGPL-3.0); `@v0.100.0/LICENSE` (200, AGPL-3.0); `@main/LICENSE` and `@master/LICENSE` both 404; repository page read 2026-09-28 | **no — hard reject, and a READ BAN** | **This is the Paperdesign failure repeating.** "No licence" invites someone to conclude "grey area, maybe ask legal". **AGPL-3.0 is on the forbidden list with no exceptions**, and because the product is a network-served sandbox orchestrator, §13 would compel us to offer our whole Corresponding Source to every user. **Nobody on this program may open, read, clone or copy `daytonaio/daytona`.** Add it to the same clean-room ban list as `Projects/Paperdesign` and `kgoedecke/doop`. Separately: archived, with the notice that as of June 2026 core development moved to a private codebase and the repository will receive no further updates, fixes or releases. 71.7k stars is irrelevant. |
| 24 | **Langfuse** `langfuse/langfuse` | MIT with proprietary `ee/` directories (`web/src/ee`, `worker/src/ee`) | **MIT-Expat core + Langfuse Enterprise License in `ee/`, `web/src/ee/`, `worker/src/ee/` — and the copyright holder is no longer Langfuse.** The root LICENSE reads *"Copyright (c) 2023-2026 ClickHouse, Inc."*; `ee/LICENSE` reads *"Langfuse Enterprise license … Copyright (c) 2023-2026 ClickHouse, Inc"* | `langfuse/langfuse@main/LICENSE` (1,612 B, full text read); `…/ee/LICENSE` (200, full preamble read) | **conditional** | The split is real and the researcher described it correctly, but **missed the acquisition**: ClickHouse, Inc. now holds the copyright on both halves, which is a live relicence lever held by a single owner. The EE file's own preamble is helpfully explicit — it states the Enterprise License does not apply to the core as defined in `/LICENSE`, which can be used and run without infringing it. **Obligation if adopted: MIT notice; delete or never build `ee/`, `web/src/ee/`, `worker/src/ee/`; never set an EE licence key; pin the version and re-verify on bump.** Self-hosting the OSS core is permitted; SSO enforcement, RBAC and data retention sit in `ee/`. |

---

## 2. Where the auditor disagrees with the researcher

Four disagreements. One of them is a hard stop.

### 2.1 Daytona is AGPL-3.0, not "no licence" — and this is the Paperdesign mistake, repeated

The researcher recorded: *"NO LICENCE FILE PUBLISHED — raw LICENSE returns 404; no grant."* The 404 is genuine, but the inference is wrong, and the error mode is the one that has already cost this program a phase.

What actually happened: `daytonaio/daytona` was **archived** in June 2026 and its default branch no longer carries a `LICENSE`. The licence has not disappeared — it is at every release tag:

```
raw.githubusercontent.com/daytonaio/daytona/v0.190.0/LICENSE  -> 200
raw.githubusercontent.com/daytonaio/daytona/v0.100.0/LICENSE  -> 200
   "GNU AFFERO GENERAL PUBLIC LICENSE, Version 3, 19 November 2007"
```

Both tags, ninety releases apart, are AGPL-3.0. There is no permissive early version to fall back to.

Why the difference matters practically:

- **"No grant" sounds like a paperwork problem.** It invites a follow-up — email the maintainers, ask legal, maybe get a written grant. Someone reads the source in the meantime to see whether it is even worth chasing.
- **AGPL-3.0 is a *read* ban, not a paperwork problem.** Once an engineer has read AGPL source, a clean-room defence for the equivalent component is gone. And §13 — the network clause — is aimed squarely at what PanaceaApp is: a service users interact with over a network. Shipping AGPL code inside the sandbox orchestrator would oblige us to offer the Corresponding Source of the whole modified work to every user of the deployed service, at the customer sites where KPMG ships it.

**Action: add `daytonaio/daytona` to the clean-room ban list in `CLAUDE.md` alongside `Projects/Paperdesign` and `kgoedecke/doop`.** Verdict `no`, on licence, not merely on abandonment.

### 2.2 A `main`-branch 404 is not evidence of anything — it fooled the researcher twice, in opposite directions

Two candidates in this set 404'd at `main/LICENSE`. The researcher drew opposite and equally unfounded conclusions:

| Repo | Researcher | Truth |
|---|---|---|
| `senthilrch/kube-fledged` | cited `main/LICENSE` as the verified Apache-2.0 source | `main/LICENSE` is **404**; default branch is `master`; `master/LICENSE` is Apache-2.0 |
| `daytonaio/daytona` | read the 404 as "no grant exists" | licence exists at every tag and is **AGPL-3.0** |

Neither URL was actually fetched, or the first would have failed loudly and the second would have prompted a tag probe. **Standing rule for this program: a `LICENSE` 404 means "wrong ref", never "no licence". Probe `master`, then the latest tag, then the repository page, before writing a verdict.**

### 2.3 LiteLLM — the MIT/Enterprise split is not a directory you can simply avoid

The researcher's quotation of the carve-out is accurate. The gap is what it means operationally, and the Phase 2 auditor already documented it in `licence-agent-fleet.md` §2.1: `pip install 'litellm[proxy]'` pins `litellm-enterprise`, and the official Dockerfile copies `/app/enterprise` into the final image. **If the platform plan deploys the stock LiteLLM proxy image as "the LiteLLM-style gateway" of `OD-19`, it takes Enterprise-licensed code into a KPMG-shipped artifact.** The MIT Python SDK (`pip install litellm`, no extras) is clean. Those are two different products and the plan must name which one it means.

This is also why rows 16–18 matter: **Portkey (MIT, no carve-out), Envoy AI Gateway (Apache-2.0, CNCF governance) and Bifrost (Apache-2.0)** are all cleaner at the licence layer than the LiteLLM proxy. `OD-19` says "LiteLLM-*style*", not "LiteLLM" — the decision is still open.

### 2.4 Cilium's `bpf/` tree is dual GPL-2.0 / BSD-2-Clause, which the researcher did not surface

`cilium/cilium@main/LICENSE` is Apache-2.0. But the eBPF datapath carries its own pair of licence files, both of which return 200:

```
bpf/LICENSE.GPL-2.0        "GNU GENERAL PUBLIC LICENSE Version 2, June 1991"
bpf/LICENSE.BSD-2-Clause   "Copyright Authors of Cilium. All rights reserved."
```

This is the same structure the Phase 2 auditor found in Tetragon, and it is benign **for the reason it is benign there**: a dual grant lets the taker elect either arm, so **we elect BSD-2-Clause and GPL never attaches.** On top of that, we consume Cilium through **Azure CNI powered by Cilium**, a Microsoft-managed AKS add-on — we do not compile it, link it, or redistribute it, so no outbound obligation arises at all.

Two rules belong in the plan, written down rather than assumed:

1. **Elect BSD-2-Clause in writing** for anything taken from `cilium/bpf/`.
2. **Never vendor a file out of `cilium/bpf/`** into our tree. Ideas and interfaces are free; files carry SPDX headers.

### 2.5 The HashiCorp MPL boundary is a fact about today, not a guarantee

`terraform-provider-azurerm@main/LICENSE` is MPL-2.0 (*"Copyright (c) 2017 HashiCorp, Inc."*), verified first-hand. HashiCorp's August 2023 BUSL-1.1 relicence covered **Terraform core and the other HashiCorp products, not the providers.** That is the only reason OpenTofu (MPL-2.0, forked from pre-BUSL Terraform) plus the azurerm provider is a legal stack at all.

But: HashiCorp holds a contributor CLA, HashiCorp is now owned by IBM, and the provider is a single-vendor repository. **Every one of those is a lever to relicense a future version.** The obligation the plan must take on is operational, not legal: **pin the provider version, record its SPDX id and commit at pin time, and re-verify the LICENSE on every version bump.** An automated check in CI is cheap; discovering a BUSL provider during a customer deployment is not.

### 2.6 Bifrost's licence is clean and its *governance* is the risk — say both

Apache-2.0, `Copyright 2025 H3 Labs Inc.`, verified first-hand. No carve-out, no Commons Clause, no BUSL. The researcher's claim holds exactly.

The finding the researcher omitted is the shape of the project: **one company owns the entire copyright, and its README sells an "Enterprise Deployments" tier behind a demo booking.** Apache-2.0 + single-owner + open-core is the precise configuration that produced the Elastic, Redis, HashiCorp, Sentry and MongoDB relicences. Nothing about that makes today's Apache grant less valid — a published Apache-2.0 version stays Apache-2.0 forever. It makes the **upgrade path** the risk. Adopt with a pinned version and a named owner for re-verification, or prefer Envoy AI Gateway, whose Linux Foundation governance removes the lever entirely.

---

## 3. Dependency findings the researcher did not check

Step 4 of the brief — "check its own dependencies for a copyleft dependency that would infect us" — was not evident in the researcher's notes. These are the direct-dependency results for the candidates that would ship inside an artifact we hand to a customer.

### 3.1 `anthropic-experimental/sandbox-runtime` — one dual-licensed dependency, plus an LGPL binary in the image

The package itself is clean. Verified from **npm package metadata at `latest`**, not from the repo badge:

| Package | npm `latest` | `license` field |
|---|---|---|
| `@anthropic-ai/sandbox-runtime` | 0.0.77 | **`Apache-2.0`** |
| `@pondwader/socks5-server` | 1.0.10 | `MIT` |
| `commander` | 15.0.0 | `MIT` |
| `zod` | 4.6.5 | `MIT` |
| **`node-forge`** | **1.4.0** | **`(BSD-3-Clause OR GPL-2.0)`** |

**`node-forge` is dual-licensed.** That is not a problem — we elect **BSD-3-Clause** and GPL-2.0 never attaches — but the election must be recorded, and the BSD-3 notice (including the no-endorsement clause) must appear in our third-party notices. A naive SBOM scanner will flag `GPL-2.0` here and someone will panic; pre-empt it in the plan.

Two further notes:
- The package publishes **vendored subtrees** — `vendor/seccomp`, `vendor/srt-win`, `vendor/java-proxy-agent` are in the published `files` list. Vendored code is exactly where a third-party licence hides. **Before adoption, run a per-file SPDX scan across `vendor/`, not just a root-LICENSE read.** Not done here; see §5.
- `package.json` declares `"repository": "git+https://github.com/anthropics/sandbox-runtime.git"` — i.e. the **`anthropics/`** org, while the researcher cited **`anthropic-experimental/`**. One redirects to the other; cite the canonical one in the plan.
- On Linux this wraps **`bubblewrap`**, which is **LGPL-2.0-or-later** (keywords in `package.json`: `sandbox`, `seatbelt`, `sandbox-exec`, `bubblewrap`). We would invoke it as a **separate process**, never link it, so LGPL §6's relinking obligation does not engage. But the **sandbox image would ship an LGPL binary**, which carries a notice obligation and a written offer of source for that binary. That is routine — every Linux base image already carries several — but it must be in the notices file, not discovered at a customer security review.

### 3.2 `kubernetes-sigs/agent-sandbox` — direct Go dependency tree is clean

`go.mod` read first-hand (`sigs.k8s.io/agent-sandbox`, Go 1.26). Direct requires are `k8s.io/*`, `sigs.k8s.io/controller-runtime`, `go.opentelemetry.io/otel/*`, `google.golang.org/grpc`, `google.golang.org/protobuf`, `prometheus/client_golang`, `gorilla/websocket`, `fsnotify`, `creack/pty`, `go-logr/logr`, `stretchr/testify`, `golang.org/x/{net,sync}`. **Every one is Apache-2.0, BSD-3-Clause or MIT. No copyleft, no MPL, in the direct tree.**

### 3.3 `external-secrets/external-secrets` — direct Go dependency tree is clean

`go.mod` read first-hand and scanned for copyleft-shaped modules. No GPL, LGPL, AGPL or MPL module in the direct tree. The `tidwall/*` family (`gjson`, `sjson`, `match`, `pretty`) that the scan surfaced is **MIT** throughout.

### 3.4 `kyverno/kyverno` — a real `NOTICE` exists and it is not decorative

Unlike Envoy AI Gateway and Knative Serving (both 404 on `NOTICE`), Kyverno ships one, and it is substantive: *"Kyverno / Copyright The Kyverno Authors … This product bundles third-party software. The full list of dependencies is declared in go.mod and pinned by content hash in go.sum. License compliance is audited on every push to main via FOSSA … The dependency summary below was produced from the kyverno module dependency graph (`go-licenses report ./cmd/kyverno/...`)"*.

**Apache-2.0 §4(d) therefore binds us**: if we redistribute Kyverno — and shipping it in the AKS baseline chart to a customer's own cloud *is* redistribution — we must reproduce that NOTICE in our distribution. Kyverno having FOSSA-audited its own tree is a genuine asset; it means the transitive audit is already done and published.

---

## 4. The obligations register — what adoption actually costs us

Everything marked `yes` or `conditional` above lands in one of four buckets. This is the list a KPMG legal reviewer will ask for.

**Bucket A — notice only (cheap, do it once, automate it).**
`agent-sandbox`, `sandbox-runtime`, `KEDA`, `Azure Workload Identity`, `External Secrets Operator`, `kubernetes/autoscaler`, `Agones`, `Portkey`, `Envoy AI Gateway`, `Bifrost`, `gVisor`, `Firecracker`, `SPIRE`, `Knative Serving`, `sysbox`/`sysbox-runc`, `kube-fledged`.
Obligation: ship the licence text, keep copyright headers on any file we copy, and state modifications on any Apache-2.0 file we change. **Generate `THIRD-PARTY-NOTICES.md` in CI from `go-licenses` / `license-checker`; do not hand-maintain it.**

**Bucket B — notice plus a reproduced upstream NOTICE.**
`Kyverno` (root `NOTICE` present). Any other candidate that grows a `NOTICE` on upgrade. **Add a CI check that fails when a pinned dependency gains a `NOTICE` file we are not reproducing.**

**Bucket C — file-level copyleft, no-fork rule.**
`OpenTofu` (MPL-2.0), `terraform-provider-azurerm` (MPL-2.0).
Obligation: **any OpenTofu or provider *file* we modify must be published under MPL-2.0.** Our `.tf` configuration and our Go control plane are a "Larger Work" under MPL §3.3 and stay proprietary. **Rule: consume both as unmodified released binaries. If we ever need a patch, upstream it rather than carry it.**

**Bucket D — dual licence, election must be recorded in writing.**
`cilium/bpf/*` (GPL-2.0 **OR** BSD-2-Clause → elect **BSD-2-Clause**), `node-forge` (BSD-3-Clause **OR** GPL-2.0 → elect **BSD-3-Clause**).
Obligation: a one-line written election per component in the plan, and the permissive licence's notice in `THIRD-PARTY-NOTICES.md`. Without the written election an auditor sees "GPL" in the SBOM and the conversation starts from the wrong place.

**Bucket E — carve-out avoidance, enforced by build, not by intention.**
`LiteLLM` (`enterprise/`), `Langfuse` (`ee/`, `web/src/ee/`, `worker/src/ee/`).
Obligation: **do not rely on a developer remembering.** If either is adopted, the build must fail when the proprietary path is present in the produced image, and the plan must name which artifact we take (LiteLLM MIT SDK, *not* the `[proxy]` extra or the stock proxy image).

---

## 5. Not verified — stated plainly rather than inferred

| Item | Why it is unverified | What would close it |
|---|---|---|
| **Star counts and release dates** for most rows | The GitHub REST API refused every call from this machine: `API rate limit exceeded for 24.239.128.117`. Figures quoted above (agent-sandbox 4.0k / v1.0.2; kube-fledged 1.4k, default branch `master`; sysbox 3.9k; Portkey 13.1k; Bifrost 8.4k; Daytona 71.7k) come from the **rendered repository page** on 2026-09-28, which does not show commit dates in the extracted text | An authenticated `gh api` call, or a `git ls-remote --tags` per repo |
| **Last release date** for Kyverno, Cilium, KEDA, ESO, autoscaler, Agones, Envoy AI Gateway, gVisor, Firecracker, SPIRE, Knative, OpenTofu, azurerm | Same rate limit. **These are not asserted.** The brief asks to prefer a release in the last 12 months; I can confirm all of these are actively released projects only from general knowledge, which this audit does not accept as evidence | `git ls-remote --tags` per repo, or an authenticated API call |
| **`sandbox-runtime` `vendor/` subtree licences** | Not scanned. `vendor/seccomp`, `vendor/srt-win`, `vendor/java-proxy-agent` ship in the published npm package and were not opened | Per-file SPDX scan of the published tarball before adoption |
| **Transitive (not direct) dependency licences** for every Go candidate | Only direct `require` blocks were read. A transitive copyleft module is possible and is the classic late surprise | `go-licenses report ./...` against a real build, once a candidate is actually vendored |
| **Bifrost sub-module licences** beyond `core/` | Only `core/go.mod` and the root `LICENSE` were read. A monorepo can licence `transports/` or `plugins/` differently | Fetch `LICENSE` under each module directory before adoption |
| **Kyverno / Cilium / KEDA `NOTICE` completeness** | Kyverno's NOTICE was read only to 600 bytes; the dependency summary beyond that was not enumerated | Read the full NOTICE and diff it against our generated notices file |
| **Claude apps gateway entitlement terms** | No source repository exists, and the commercial terms were not read | A KPMG procurement/legal review of Anthropic's Commercial Terms of Service — **not an OSS-adoption decision** |

---

## 6. Bottom line

- **1 hard reject on licence, with a read ban: `daytonaio/daytona` is AGPL-3.0**, not unlicensed. The researcher's "no grant" reading is the second-most-dangerous error available here; the most dangerous would have been reading its source first.
- **1 out of scope: the Claude apps gateway** is a contract question, not an OSS one. It cannot be cleared by this audit at all.
- **2 file-level copyleft, usable with a no-fork rule:** OpenTofu and terraform-provider-azurerm, both MPL-2.0, both pinned and re-verified on every bump.
- **2 carve-out traps needing a build-enforced boundary:** LiteLLM `enterprise/` and Langfuse `ee/` — and Langfuse's copyright now sits with **ClickHouse, Inc.**, which the researcher did not catch.
- **2 dual-licence elections to write down:** Cilium's `bpf/` tree (elect BSD-2-Clause) and `node-forge` (elect BSD-3-Clause).
- **1 citation to correct before this reaches the owner:** `kube-fledged`'s licence URL is on `master`, not `main`; the researcher's cited URL returns 404.
- **Everything else in this set is genuinely permissive and genuinely usable.** The largest single governance asset in the list is that most of it — Kyverno, Cilium, KEDA, ESO, autoscaler, SPIRE, Knative, agent-sandbox, Envoy AI Gateway — is CNCF- or Linux-Foundation-governed on DCO rather than a vendor CLA, which structurally removes the relicence lever that burned HashiCorp, Elastic and Redis. The three single-vendor projects in the list — **Bifrost (H3 Labs), sysbox (Docker), azurerm (HashiCorp/IBM)** — are the three to pin and watch.
