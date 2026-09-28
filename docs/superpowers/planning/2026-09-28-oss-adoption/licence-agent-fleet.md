# Licence audit — Phase 2 / Team-mode candidates (cockpit, agent fleet, terminals, sessions)

**Audited 2026-09-28** by an adversarial licence auditor against
`docs/superpowers/planning/2026-09-28-oss-adoption/03-agent-fleet.md`.

**Method.** Every licence below was fetched as a raw blob from the actual repository over HTTPS
(`curl --ssl-no-revoke -L`), or from `registry.npmjs.org` package metadata, on **2026-09-28**, and
the identifying line read. Nothing is taken from a GitHub badge, a README, a blog or memory.
Relicensing history was checked by search plus first-hand fetch of the artefact that proves it.
Where a source could not be reached, the row says **UNVERIFIED** and says why.

**Clean-room discipline.** No AGPL or GPL source was opened. Copyleft projects named here were
touched only at their `LICENSE`/`COPYING` blob — which is licence text, not program source.

---

## Verdict table

`yes` = permissive, ship it · `conditional` = usable, but a named obligation attaches ·
`no` = do not take it into the product.

| # | Candidate | Researcher claimed | **Verified (SPDX)** | Blob read 2026-09-28 | Verdict | Obligation / condition |
|---|---|---|---|---|---|---|
| 1 | **LiteLLM** `BerriAI/litellm` | MIT root; `enterprise/` proprietary | **MIT (root) + BerriAI Enterprise License (`enterprise/`)** — *and the split is worse than stated, see §2.1* | `BerriAI/litellm@main/LICENSE`; `…/enterprise/LICENSE.md`; `…/pyproject.toml`; `…/Dockerfile` | **conditional — auditor materially expands the finding** | The MIT SDK (`pip install litellm`) is clean. **`pip install 'litellm[proxy]'` hard-pins `litellm-enterprise==0.1.71`**, and the **official Dockerfile copies `/app/enterprise` into the final image**. If we deploy the stock proxy image we take Enterprise-licensed code, which "may only be used in production if you … have agreed to … the BerriAI Subscription Terms of Service". |
| 2 | **NATS / JetStream** `nats-io/nats-server` | Apache-2.0 | **Apache-2.0** — clients `nats.go` **Apache-2.0**, `nkeys` **Apache-2.0** | `nats-io/nats-server@main/LICENSE`; `nats-io/nats.go@main/LICENSE`; `nats-io/nkeys@main/LICENSE` | **yes** — with a governance note the researcher omitted (§2.2) | Apache §4: retain `LICENSE`, carry `NOTICE`, state modifications. **Pin the Apache-2.0 version and watch the licence on upgrade**: Synadia attempted a BUSL relicence of `nats-server` in 2025 and reverted only after a CNCF trademark dispute. |
| 3 | **centrifuge** (Go library) `centrifugal/centrifuge` | MIT | **MIT**, © 2023 Centrifugal Labs LTD | `centrifugal/centrifuge@master/LICENSE` | **yes** | MIT notice. Direct deps verified clean: `centrifugal/protocol` MIT, `centrifugal/fdelta` **BSD-2-Clause** (carries the Fossil/D. R. Hipp attribution chain — keep all five copyright lines), `FZambia/eagle` MIT, `segmentio/encoding` MIT, `maypok86/otter` Apache-2.0, `redis/rueidis` Apache-2.0, `quagmt/udecimal` BSD-3-Clause. No copyleft in the direct tree. |
| 4 | **River** `riverqueue/river` | MPL-2.0 (conditional) | **MPL-2.0** — and **`riverqueue/riverui` is also MPL-2.0** | `riverqueue/river@master/LICENSE`; `riverqueue/riverui@master/LICENSE` | **conditional** — confirmed | MPL §3.2: any River **file** we modify must be published under MPL-2.0. Our own files that merely call River stay proprietary (§1.10 "Larger Work"). Hold the no-fork rule the researcher proposes. Note a separate commercial **River Pro** exists; do not pull it in. |
| 5 | **`@xterm/addon-{webgl,serialize,search,unicode11,clipboard,web-links}`** | MIT each | **MIT each**, confirmed per package at npm `latest`: webgl 0.19.0 · serialize 0.14.0 · search 0.16.0 · unicode11 0.9.0 · clipboard 0.2.0 · web-links 0.12.0 | `registry.npmjs.org/@xterm/addon-*/latest` (six calls) | **yes** | MIT notice ×6. **One dependency the researcher did not name: `@xterm/addon-clipboard` pulls `js-base64` (`BSD-3-Clause`)** — the only addon with a runtime dep; the other five have none. Add its BSD-3 notice, including the no-endorsement clause. |
| 6 | **asciinema-player** (player only) | Apache-2.0, "relicensed" | **Apache-2.0 — and there is NO relicensing boundary**: every published npm version from the first (2.6.1) to `latest` (3.17.0) declares `Apache-2.0` | `asciinema/asciinema-player@main/LICENSE`; `registry.npmjs.org/asciinema-player` full document | **yes** | Apache §4. Runtime deps verified: `solid-js` MIT, `@babel/runtime` MIT, `solid-transition-group` MIT (which pulls `@solid-primitives/*`, MIT). The Rust VT engine `asciinema/avt` is Apache-2.0. **The recorder CLI `asciinema/asciinema` is GPL-3.0 — never link, never read.** |
| 7 | **CloudEvents** `cloudevents/spec` + `sdk-go` | Apache-2.0 both | **Apache-2.0 both** | `cloudevents/spec@main/LICENSE`; `cloudevents/sdk-go@main/LICENSE` | **yes** | Apache §4. The spec is a document — copying the envelope shape carries no code obligation beyond attribution. |
| 8 | **Cedar** `cedar-policy/cedar-go` (core `cedar-policy/cedar`) | Apache-2.0 both | **Apache-2.0 both** | `cedar-policy/cedar-go@main/LICENSE`; `cedar-policy/cedar@main/LICENSE` | **yes** | Apache §4. Amazon-governed; no CLA-backed relicence found. |
| 9 | **A2A** `a2aproject/A2A` + `a2a-go` (vendor the schema) | Apache-2.0 both | **Apache-2.0 both** | `a2aproject/A2A@main/LICENSE`; `a2aproject/a2a-go@main/LICENSE` | **yes** | Apache §4(a)(b)(c): ship the licence, mark our modified files as changed, keep the copyright header on any vendored type file. **§4(d) does not bite: `A2A@main/NOTICE` returns 404**, so there is no upstream NOTICE text to propagate. Verified, not assumed. |
| 10 | **gotty** `sorenisanerd/gotty` (ring-buffer idea) | MIT | **MIT**, © 2015-2017 Iwasaki Yudai, © 2021-2022 Søren L. Hansen | `sorenisanerd/gotty@master/LICENSE` | **yes** for a small attributed copy | MIT notice **naming both copyright holders**. If we copy the ring buffer, copy only that file's logic and keep the header — do not take gotty's `go.mod` tree with it (see §2.5). |

### Study-only and rejected candidates — licences still verified first-hand

| # | Candidate | Researcher claimed | **Verified (SPDX)** | Blob read 2026-09-28 | Verdict | Obligation / note |
|---|---|---|---|---|---|---|
| 11 | **OpenHands** `OpenHands/OpenHands` (+ `All-Hands-AI/openhands-aci`) | MIT both | **MIT both** — "The MIT License (MIT), © 2025 OpenHands contributors" | `OpenHands/OpenHands@main/LICENSE`; `All-Hands-AI/openhands-aci@main/LICENSE` | **yes** (reading is free; study-only by design) | MIT notice if any code is copied. The researcher's `enterprise/README.md` 404 probe is one path, not proof; re-probe `enterprise/LICENSE` before copying anything. |
| 12 | **Cline** `cline/cline` | Apache-2.0 | **Apache-2.0** | `cline/cline@main/LICENSE` | **yes** (study-only) | Apache §4 if copied. |
| 13 | **Goose** `block/goose` | Apache-2.0 | **Apache-2.0** | `block/goose@main/LICENSE` | **yes** (study-only) | Apache §4 if copied. |
| 14 | **OpenCode** `sst/opencode` | MIT | **MIT**, "© 2025 opencode" | `sst/opencode@dev/LICENSE` (default branch is `dev`, not `main`) | **yes** (study-only) | MIT notice. Do not confuse with `opencode-ai/opencode`, whose lineage leads to `charmbracelet/crush` (row 22). |
| 15 | **SWE-ReX** `SWE-agent/SWE-ReX` (+ SWE-agent) | MIT both | **MIT both** — SWE-ReX © 2024 Kilian Lieret, Carlos E. Jimenez; SWE-agent © 2024 John Yang et al. | `SWE-agent/SWE-ReX@main/LICENSE.txt`; `SWE-agent/SWE-agent@main/LICENSE` | **yes** on licence — rejected on `OD-8`, not on law | MIT notice if copied. |
| 16 | **humanlayer** `humanlayer/humanlayer` | "Apache-2.0 per LICENSE, but GitHub says NOASSERTION" | **Apache-2.0 by reference only.** The `LICENSE` file is a **header stub** — "Apache Software License 2.0 … Licensed under the Apache License, Version 2.0 … You may obtain a copy of the License at" — it does **not** contain the licence text | `humanlayer/humanlayer@main/LICENSE` | **conditional** — reading is safe, copying is not | GitHub's `NOASSERTION` is correct, and the cause is now established: the file incorporates Apache-2.0 **by reference**. That is legally workable but sloppy. **Do not copy code from humanlayer** without written confirmation from the maintainers, or at minimum a per-file `SPDX-License-Identifier` header check. Study-only makes this moot; keep it that way. |
| 17 | **vibe-kanban** `BloopAI/vibe-kanban` | Apache-2.0 | **Apache-2.0** | `BloopAI/vibe-kanban@main/LICENSE` | **yes** (study-only) | Apache §4 if copied. Company is sunsetting — an Apache-2.0 grant survives the company, but nobody will fix a defect. |
| 18 | **Crystal** `stravu/crystal` | MIT | **MIT**, © 2024 Stravu | `stravu/crystal@main/LICENSE` | **yes** (study-only) | MIT notice. |
| 19 | **e2b infra** `e2b-dev/infra` | Apache-2.0 | **Apache-2.0** | `e2b-dev/infra@main/LICENSE` | **yes** (study-only) | Apache §4 if copied. |
| 20 | **microsandbox** | Apache-2.0 | **Apache-2.0** | `microsandbox/microsandbox@main/LICENSE` | **yes** (study-only) | Apache §4 if copied. |
| 21 | **Falco** / **Tetragon** | Falco Apache-2.0; Tetragon Apache-2.0 with `bpf/` dual GPL-2.0 / BSD-2-Clause | **Falco Apache-2.0** and `falcosecurity/libs` Apache-2.0 at root — **but the `driver/` tree is `SPDX-License-Identifier: GPL-2.0-only OR MIT`**, which the researcher did not surface. **Tetragon Apache-2.0** confirmed, `bpf/LICENSE.BSD-2-Clause` and `bpf/LICENSE.GPL-2.0` both present | `falcosecurity/falco@master/LICENSE`; `falcosecurity/libs@master/LICENSE`; `…/libs/master/driver/ppm_events_public.h`; `cilium/tetragon@main/LICENSE` + `bpf/LICENSE.*` | **conditional — auditor adds a finding** | Both are **dual**-licensed at the kernel layer, so the **MIT** (Falco) / **BSD-2-Clause** (Tetragon) option is available and GPL never attaches to us. **Elect the permissive option in writing** in the plan, and never copy a `driver/` or `bpf/` header into our tree without carrying its SPDX line. Neither runs in-process with our Go control plane in any case. |
| 22 | **coder/agentapi** | MIT | **MIT** (grant text; "Copyright (c) 2025 Coder Technologies, Inc.", no "MIT License" title line) | `coder/agentapi@main/LICENSE` | **yes** on licence — rejected on `OD-8` | MIT notice if copied. Licence is not the reason to reject it; `OD-8` is. |
| 23 | **tmux** | ISC | **ISC** — "Permission to use, copy, modify, and distribute this software for any purpose with or without fee is hereby granted…", "Copyright (c) Various Authors" | `tmux/tmux@master/COPYING` | **yes** on licence — rejected on fidelity | ISC notice if shipped in the sandbox image. Rule L allows it; §6.2's fidelity argument is the reason it is out. |
| 24 | **charmbracelet/crush** | FSL-1.1-MIT, competitor restriction | **FSL-1.1-MIT** — "Functional Source License, Version 1.1, MIT Future License", "Copyright 2025-2026 Charmbracelet, Inc." | `charmbracelet/crush@main/LICENSE.md` | **no** | Source-available with a competing-use restriction; converts to MIT two years after each release. **Forbidden by rule L today.** A given release becomes MIT on its own two-year anniversary — that is a future option, not a present one, and adopting on that basis pins us to two-year-old code. |
| 25 | **Anthropic Agent SDK** `claude-agent-sdk-typescript` | Not open source | **Not open source** — `LICENSE.md` in full: "© Anthropic PBC. All rights reserved. Use is subject to Anthropic's Commercial Terms of Service" | `anthropics/claude-agent-sdk-typescript@main/LICENSE.md` | **no** (as an OSS dependency) | Governed by a commercial contract, not a licence grant. Any use is a procurement question, not an OSS-adoption question. |
| 26 | **Daytona** `daytonaio/daytona` | Not resolvable; unmaintained | **UNRESOLVED — confirmed.** `main/LICENSE` returns **HTTP 404** on an independent fetch | `raw.githubusercontent.com/daytonaio/daytona/main/LICENSE` → 404 | **no** | No licence at the default branch means **no grant we can rely on**. Reject on licence as well as on abandonment. |

---

## 2. Where the auditor disagrees with the researcher

### 2.1 LiteLLM — the MIT/Enterprise split is not confined to a directory you can ignore

The researcher's statement ("MIT at root; `enterprise/` under the proprietary BerriAI Enterprise
License") is **true and incomplete**, and the incompleteness is the dangerous part. Three facts read
first-hand from `BerriAI/litellm@main` on 2026-09-28:

1. **`pyproject.toml`, `[project.optional-dependencies] proxy`, line 79:**
   `"litellm-enterprise==0.1.71"` — a **hard pin, not an extra-of-an-extra**. Anyone who runs
   `pip install 'litellm[proxy]'` installs the Enterprise package. That PyPI package declares
   **no licence at all**: `https://pypi.org/pypi/litellm-enterprise/json` returns
   `info.license = null` with **zero** `License ::` classifiers (read 2026-09-28).
2. **`Dockerfile` line 150 and `docker/Dockerfile.non_root` line 162:**
   `COPY --from=builder /app/enterprise /app/enterprise`, with the adjacent comment
   *"enterprise/ is imported by source path at runtime (proxy_cli puts the … enterprise.enterprise_hooks from it)"*.
   **The official images ship Enterprise-licensed source and import it at runtime.**
3. **`enterprise/LICENSE.md`** closes with: *"Subject to the foregoing, it is forbidden to copy,
   merge, publish, distribute, sublicense, and/or sell the Software."* — and permits production use
   only with a BerriAI Subscription.

**Why this matters more here than for a normal adopter.** `OD-5` has KPMG **shipping** PanaceaApp to
customer-hosted Azure. Shipping the stock LiteLLM proxy image is *distribution* of the Enterprise
Software, which that licence forbids outright.

What is genuinely clean: `pyproject.toml` declares `license = "MIT"`, `license-files = ["LICENSE"]`,
and `[tool.maturin] exclude` lists `litellm/proxy/enterprise` and `litellm/proxy/enterprise/**`, so
the **litellm wheel itself carries no Enterprise source**. The SDK is MIT. The *server* is not
cleanly MIT as distributed.

**Auditor's verdict: conditional, with a build rule.** If the Hub's gateway is LiteLLM, we build our
own image from the MIT tree with `enterprise/` absent and the `proxy` extra's
`litellm-enterprise` pin removed, and we add the CI gate the researcher already proposed (fail if
`enterprise/` or `litellm_enterprise` appears in any layer or lockfile) — **and that gate must also
fail on `litellm-enterprise` as a resolved Python dependency**, not only on a directory name.
If that build turns out to be non-trivial, the honest answer is to write the gateway ourselves or
choose a single-licence alternative.

### 2.2 NATS — Apache-2.0 today, but it survived a BUSL relicence attempt, and the plan must say so

`nats-io/nats-server@main/LICENSE` is Apache-2.0, verified. The researcher stopped there. In 2025
**Synadia announced it would move `nats-server` to the Business Source License**; the CNCF filed a
USPTO petition over the NATS trademark; the dispute was settled on 2025-05-01 with Synadia agreeing
**not** to fork under BUSL, to keep `nats-server` Apache-2.0, and to transfer the trademarks to the
CNCF. Sources read 2026-09-28:
[CNCF](https://www.cncf.io/blog/2025/05/01/protecting-nats-and-the-integrity-of-open-source-cncfs-commitment-to-the-community/) ·
[Synadia](https://www.synadia.com/blog/nats-server-next-steps) ·
[The Register](https://www.theregister.com/2025/05/02/cncf_synadia_nats/) ·
[nats-server issue 6832](https://github.com/nats-io/nats-server/issues/6832)

This does **not** change the verdict — Apache-2.0 is irrevocable for released versions and CNCF
stewardship makes a repeat unlikely. It changes the **operating rule**: pin the version, record the
SPDX id at the pinned commit, and re-check the LICENSE blob on every major upgrade. A candidate
with a live relicensing attempt in its history is exactly the profile that burned this program with
Paperdesign, and it belongs in the record.

### 2.3 asciinema-player — "relicensed" is wrong in the summary table

The researcher's table cell reads "**Apache-2.0** (relicensed; see 3.5)", while their own 3.5 body
correctly says *"there is no relicensing boundary on the player"*. I checked the **full npm
document**, not five spot versions: **every published version from the earliest (2.6.1) to
latest (3.17.0) declares `"license": "Apache-2.0"`.** There is no boundary and no version below
which the licence differs.

Fix the table cell. Leaving "relicensed" in a summary that a lawyer may read alone invents a risk
that does not exist, and a false alarm costs credibility the same way a missed AGPL does.

The real split — which the researcher got right and which I confirm — is **between repositories**:
`asciinema-player` Apache-2.0 (adopt) · `asciinema/avt` Apache-2.0 (the Rust VT engine, adopt with
it) · **`asciinema/asciinema`, the recorder CLI, GPL-3.0** (`asciinema/asciinema@develop/LICENSE`
read 2026-09-28 — "GNU GENERAL PUBLIC LICENSE Version 3"): **never link it, never read it.**

### 2.4 The clipboard addon has a dependency the researcher did not name

Five of the six addons have an empty `dependencies` object. The sixth does not:
`@xterm/addon-clipboard@0.2.0` depends on **js-base64 ^3.7.5**, which is **BSD-3-Clause**
(`registry.npmjs.org/js-base64/latest`, read 2026-09-28). BSD-3 adds the no-endorsement clause to
the notice set. Not a blocker — a NOTICE-file line the plan must not forget.

Also: `xtermjs/xterm.js@master/LICENSE` is MIT with **three** copyright holders (the xterm.js
authors 2017-2019, SourceLair 2014-2016, Christopher Jeffrey 2012-2013). Attribution must carry all
three, not just "xterm.js".

### 2.5 gotty — MIT to copy from, **MPL-2.0 to depend on**

`sorenisanerd/gotty@master/LICENSE` is MIT, confirmed. But `go.mod` (read 2026-09-28) pulls
**github.com/yudai/hcl** (direct) and **github.com/hashicorp/go-multierror** + **hashicorp/errwrap**
(indirect) — **all three Mozilla Public License 2.0**, verified at their own LICENSE blobs.

So the researcher's recommendation (vendor the ring buffer, ~200 lines, with attribution) is the
**only** safe shape. A `go get github.com/sorenisanerd/gotty` would add three MPL-2.0 modules to the
Hub's dependency set for a 200-line ring buffer. MPL-2.0 is survivable (row 4), but taking it by
accident for this is not a trade anyone would make on purpose. Write "vendor, never import" into the
plan.

### 2.6 Falco's driver is dual GPL-2.0/MIT — the researcher caught Tetragon and missed Falco

The researcher flagged Tetragon's `bpf/` dual licence (correct; I re-verified both
`bpf/LICENSE.BSD-2-Clause` and `bpf/LICENSE.GPL-2.0` return 200) but reported Falco as flatly
Apache-2.0. `falcosecurity/falco@master/LICENSE` and `falcosecurity/libs@master/LICENSE` are indeed
Apache-2.0 — **but the kernel driver is not**. From
`falcosecurity/libs@master/driver/ppm_events_public.h`, read 2026-09-28:

    // SPDX-License-Identifier: GPL-2.0-only OR MIT
    Copyright (C) 2023 The Falco Authors.
    This file is dual licensed under either the MIT or GPL 2.

Same shape as Tetragon, same answer: **dual**, so the MIT arm is available and no GPL obligation
reaches us. Record the election explicitly. Both projects run as separate processes or DaemonSets,
so nothing links into our Go binaries either way — but "Apache-2.0" as a bare statement about Falco
is wrong and would fail an audit.

### 2.7 humanlayer — the NOASSERTION has a cause, and it is not a false positive

The researcher noted GitHub's NOASSERTION and left it ambiguous. The cause is now established by
reading the file: `humanlayer/humanlayer@main/LICENSE` is a **boilerplate header**, not the licence.
Its entire opening is "Apache Software License 2.0 / Copyright (c) 2024, humanlayer Authors /
Licensed under the Apache License, Version 2.0 (the License); you may not use this file except in
compliance with the License. You may obtain a copy of the License at ..." — i.e. Apache-2.0
**incorporated by reference**, with no grant text in the repository.

Defensible, but it is why GitHub's detector abstains, and it is not the unambiguous grant our bar
wants. Study-only is the right verdict and it should stay study-only; **do not copy humanlayer code**
without a per-file SPDX check or written confirmation from the maintainers.

### 2.8 Daytona — reject on **licence**, not only on abandonment

The researcher rejected Daytona for abandonment and treated the missing licence as a footnote. I
re-fetched `raw.githubusercontent.com/daytonaio/daytona/main/LICENSE` independently: **HTTP 404**.
A repository with no licence at its default branch grants **no rights at all** by default. That is a
harder reject than "unmaintained", and it should be written as the primary reason so nobody revives
it on the argument that a fork could be maintained.

---

## 3. Where the researcher was right, and I could not break it

These survived an attempt to disprove them. Recorded so nobody re-litigates them.

- **centrifuge is genuinely clean to the second level.** I read its `go.mod` and then every direct
  dependency's own LICENSE: `centrifugal/protocol` MIT · `centrifugal/fdelta` BSD-2-Clause (it
  carries a five-deep attribution chain back to D. Richard Hipp's 2007 Fossil delta — all five
  copyright lines must be reproduced) · `FZambia/eagle` MIT · `segmentio/encoding` MIT ·
  `maypok86/otter` Apache-2.0 · `redis/rueidis` Apache-2.0 · `quagmt/udecimal` BSD-3-Clause ·
  `cel-go`, `prometheus/client_golang`, `google.golang.org/protobuf` all Apache-2.0. **No copyleft
  anywhere in the direct tree.** Single-vendor governance (Centrifugal Labs LTD) is the residual
  risk, not the licence.
- **Cedar and A2A are clean to the second level too.** `cedar-go@main/go.mod` has exactly one direct
  dependency, `golang.org/x/exp` (BSD-3-Clause), plus `google/go-cmp` (BSD-3-Clause) as indirect.
  `a2a-go@main/go.mod` is a `/v2` module depending on `a2a-go v0.3.15` and `go-cmp`. Nothing to
  infect us.
- **River's MPL-2.0 conclusion holds, and extends.** `riverqueue/riverui` is **also MPL-2.0**
  (verified), so the optional UI carries the same file-level obligation, not a worse one.
- **The six xterm addons are MIT individually**, not merely "MIT because the monorepo is MIT". I
  verified the `license` field on each package's own npm `latest` document, which is the level that
  matters when only one package is installed.
- **The GPL-3.0 asciinema recorder was correctly quarantined.** Verified GPL-3.0 first-hand at the
  LICENSE blob; the plan's "write our own recorder" is the right call and costs about a day.
- **`OD-8` rejections are correct and are not licence rejections.** `coder/agentapi` (MIT) and
  `SWE-ReX` (MIT) are both permissively licensed and both rightly rejected on architecture. Keeping
  the two reasons separate is what stops a future reader "fixing" the licence objection and
  re-adopting them.

## 4. Obligations we would actually take on

If every `yes`/`conditional` row is adopted, this is the complete list of what PanaceaApp owes.
Nothing here touches our own source licence.

1. **A `THIRD-PARTY-NOTICES` file shipped with the desktop app and with every Hub image**, carrying:
   MIT text + copyright lines for centrifuge, `centrifugal/protocol`, `FZambia/eagle`,
   `segmentio/encoding`, the six `@xterm/*` addons, **xterm.js's three copyright holders**, gotty's
   **two** copyright holders, `solid-js`, `@babel/runtime`, `solid-transition-group`; BSD-2-Clause
   text + the **five-line Fossil-delta attribution chain** for `centrifugal/fdelta`; BSD-3-Clause
   text for `js-base64`, `quagmt/udecimal`, `golang.org/x/exp`.
2. **Apache-2.0 §4 compliance** for NATS (`nats-server`, `nats.go`, `nkeys`), CloudEvents
   (`spec`, `sdk-go`), Cedar (`cedar-go`, `cedar`), A2A (`A2A`, `a2a-go`), asciinema-player and
   `avt`, `maypok86/otter`, `redis/rueidis`: ship the licence, propagate any upstream `NOTICE`,
   and mark files we modify as changed. **A2A has no root NOTICE** (404 verified), so nothing to
   propagate there.
3. **MPL-2.0 §3.2 for River (and riverui if used)**: if we modify a River **file**, that file must
   be published under MPL-2.0. Our own files that merely import River stay proprietary. The
   mitigation is the no-fork rule — upstream changes or wrap them.
4. **An explicit written election of the permissive arm** for Falco's `driver/` (MIT) and Tetragon's
   `bpf/` (BSD-2-Clause), if either is ever deployed. Dual licences do not elect themselves.
5. **A CI licence gate** that fails the build on: any AGPL/GPL/LGPL/SSPL/BUSL/Elastic/Commons-Clause
   SPDX id in the Go, npm, Rust or Python lock files; the string `litellm-enterprise` or a path
   `enterprise/` in any image layer; and any dependency whose SPDX id is `NOASSERTION`. Run it on
   every dependency bump, not only at adoption.

## 5. Sources I could not reach, and what is therefore unverified

| Attempted | Result | Consequence |
|---|---|---|
| `raw.githubusercontent.com/daytonaio/daytona/main/LICENSE` | **HTTP 404** (independently re-confirmed) | Daytona's current licence is **UNVERIFIED and unresolvable**. Rejected. |
| `raw.githubusercontent.com/a2aproject/A2A/main/NOTICE` | **HTTP 404** | Confirms there is **no** upstream NOTICE to propagate. A negative result, used as such. |
| `raw.githubusercontent.com/falcosecurity/libs/master/driver/COPYING` | **HTTP 404** | The driver's dual licence was established from the SPDX header in `driver/ppm_events_public.h` instead, which is authoritative. |
| `riverqueue/rivepro`, `riverqueue/river-pro` public LICENSE | not resolvable | **A paid "River Pro" product exists** (`riverqueue.com/pro`, read 2026-09-28) but I could read no licence for it. **UNVERIFIED — treat as proprietary and never depend on it.** The open `riverqueue/river` and `riverqueue/riverui` are both MPL-2.0 and are what the plan adopts. |
| PyPI `litellm-enterprise` licence metadata | reachable, but `info.license = null`, **zero** licence classifiers | The Enterprise package ships to PyPI **with no declared licence**. Its terms come only from `enterprise/LICENSE.md` in the GitHub repo. |
| `OpenHands/OpenHands/main/enterprise/**` beyond one path | not exhaustively probed | Still **not proof of absence** of a paid directory. Immaterial while OpenHands is study-only; re-probe before copying a line. |

A TLS-intercepting corporate proxy (Netskope) is in the path; every fetch above used
`curl --ssl-no-revoke -L`. No fetch was silently substituted by a cached or guessed page: each row's
HTTP status was captured alongside the body.

## 6. Bottom line

**Nothing the researcher put in the adopt column is forbidden by rule L.** The nine adopt/vendor
candidates — NATS, centrifuge, River, the six xterm addons, asciinema-player, CloudEvents, Cedar,
A2A's schema and gotty's ring buffer — are all permissive or MPL-2.0, all verified at the blob, all
clean to the direct-dependency level. No AGPL, GPL, SSPL, BUSL or Elastic licence is hiding in any
of them.

**Four things must change before this plan is executed:**

1. **LiteLLM: do not deploy the stock proxy image or `pip install 'litellm[proxy]'`.** Both carry
   BerriAI Enterprise code that is forbidden to distribute, and KPMG distributes. Build from the MIT
   tree, or choose a single-licence gateway. (§2.1)
2. **Falco: record it as Apache-2.0 *with a dual GPL-2.0/MIT driver*, and elect MIT in writing.**
   The current text says Apache-2.0 flat, which is wrong. (§2.6)
3. **gotty: vendor the file, never import the module** — three MPL-2.0 modules ride along with a
   `go get`. (§2.5)
4. **Correct the asciinema-player table cell.** There was no relicence; the word invents a risk.
   (§2.3)

**Two operating rules to carry forward:** pin NATS and re-read its `LICENSE` on every major upgrade
(a BUSL relicence was attempted in 2025 and reversed only by CNCF trademark pressure, §2.2); and
keep humanlayer and anything else with a `NOASSERTION` grant at study-only until the grant is
unambiguous (§2.7).

**The residual legal exposure of this phase's OSS plan, if the four corrections are made, is one
MPL-2.0 file-level obligation (River) and an attribution file.** That is a normal, defensible
position for proprietary software shipped to regulated customers.
