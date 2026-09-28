# Licence audit — desktop, terminal and UI primitives (`07-ui-primitives.md`)

**Adversarial verification, audit date 2026-09-28.** Every SPDX id below was re-fetched on this
machine from the project's own `LICENSE`/`COPYING` file, its `Cargo.toml`/`package.json` `license`
field as published, or the GitHub licence API — read as raw bytes, never from a badge, a sidebar, a
blog or memory. Where the researcher and I disagree, the disagreement is stated plainly with the
bytes that settle it.

Tooling used: `curl --ssl-no-revoke` against `registry.npmjs.org`, `crates.io`,
`raw.githubusercontent.com`, and `gh api repos/<r>`. No copyleft project's *source* was opened —
for the one MPL-2.0 candidate and the copyleft-adjacent ones, only licence files and package
metadata were fetched.

---

## 1. Verdict in one paragraph

**No forbidden licence (AGPL/GPL/LGPL/SSPL/BUSL/Elastic/Commons Clause/source-available) governs
any project the researcher marked adopt or vendor.** Every adopt-list item verifies as MIT, or as
MIT-or-Apache-2.0 dual, at source. Two of the researcher's three sharpest calls hold exactly as
written -- `dockview` is a permissive core with a proprietary `dockview-enterprise` add-on, and
`@melloware/react-logviewer` is MPL-2.0 -- and the dockview one is **worse** than stated, because
`dockview-core`'s own npm metadata reads plain MIT while the repo classifies as NOASSERTION. The
third call, "`react-virtuoso` has no licence file", is **wrong about the artifact we would
actually ship** (see D-2).

There are **five material corrections and two minor ones**. Four change what an SBOM gate or a
lockfile must pin; one is a stale citation for a repository that has *moved owners*; and one --
`shiki` -- is the finding that matters most, because adopting it as the researcher proposed would
have put **GPL-3.0 data files into a product KPMG ships to regulated customers**, under a package
whose own npm metadata truthfully reads `"license":"MIT"`.

---

## 2. The disagreements — read these before the table

### D-1 (material, and the biggest finding in this audit). `shiki` is MIT. The grammars it ships are not.

The researcher recorded `shiki` as **MIT** and flagged, correctly and to their credit, that "the
bundled grammars are TextMate grammars and themes, and those carry their own licences… the root
`LICENSE` I read does not enumerate them." That flag is right and it is worse than a flag: **I
enumerated them, and the default bundle contains GPL-3.0.**

- `shiki@4.4.3` (published 2026-08-10) depends on `@shikijs/langs@4.4.3`, whose own npm metadata
  reads `"license":"MIT"` (https://registry.npmjs.org/@shikijs/langs/latest, read 2026-09-28).
- The grammars in that package are generated from `shikijs/textmate-grammars-themes`
  (repo licence MIT, 282★, pushed 2026-09-22). That repo ships a **693,215-byte `NOTICE`** at
  https://raw.githubusercontent.com/shikijs/textmate-grammars-themes/main/packages/tm-grammars/NOTICE
  (read 2026-09-28) which opens: *"This project incorporates material from the project(s) listed
  below… This Third Party Code is licensed to you under their original license terms."*
- I parsed all **155** `Files:/License:/SPDX:` blocks in it. The tally:

  | SPDX in the NOTICE | count |
  | --- | --- |
  | MIT | 111 |
  | Apache-2.0 | 22 |
  | **NOASSERTION** | **12** |
  | **GPL-3.0** | **5** |
  | **MPL-2.0** | **2** |
  | **"GNU"** (unresolved) | **1** |
  | ISC | 1 |
  | BSD-3-Clause | 1 |

- The named offenders, exactly: **GPL-3.0** — `ada.json`, `gnuplot.json`, `nginx.json`,
  `org.json`, `racket.json`. **"GNU"** — `ahk2.json`. **MPL-2.0** — `bird2.json`, and
  **`hcl.json` + `terraform.json`** (HashiCorp `syntax`). **NOASSERTION** — `apache.json`,
  `apex.json`, `asciidoc.json`, `cue.json`, `elixir.json`, `llvm.json`, `luau.json`, `nim.json`,
  `nsis.json`, `purescript.json`, `sass.json`, `wolfram.json`.

**Why this is not theoretical for us.** `hcl.json` and `terraform.json` are the two grammars a
Deploy phase most wants, and they are MPL-2.0. `sass.json` and `elixir.json` are NOASSERTION —
i.e. *no licence grant found at all*, which for redistribution purposes is worse than copyleft.
And `nginx.json` is GPL-3.0.

**Verdict: `shiki` is CONDITIONAL, not a plain adopt.** The obligation and the mitigation are
precise and cheap:
1. Never import the `shiki` full bundle. Import `shiki/core` + `createHighlighterCore` and pass an
   explicit list of grammars from `@shikijs/langs/<lang>`.
2. Put that list in a checked-in allowlist and gate it in CI against the `NOTICE` above, so a new
   language cannot be added by a one-line import without a licence look.
3. Record the per-grammar MIT/Apache-2.0 notices we do ship in `NOTICES`. Apache-2.0 grammars
   additionally require the `NOTICE` text be carried.

The researcher's recommendation ("adopt — phase 3, not now") survives with that condition attached.
Adopting the default bundle as-is would put GPL-3.0 data files into a product KPMG ships.

### D-2 (material). `react-virtuoso` DOES ship a licence — in the artifact we would actually consume

The researcher rejected it partly on provenance: *"`api.github.com/repos/petyosi/react-virtuoso/license`
returns 404, and the repository root has no LICENSE, LICENCE or COPYING file."* Both halves of that
are true — I re-confirmed the 404 on 2026-09-28. **But the researcher checked the repository and not
the package.** The published tarball contains a full MIT text:

> `https://unpkg.com/react-virtuoso@4.18.15/LICENSE` → `MIT License` / `Copyright (c) 2020 Petyo Ivanov`
> (read 2026-09-28)

The thing we would redistribute carries its own MIT grant. So the "no licence file anywhere"
framing is wrong and must not be repeated in a plan or a legal memo — it is the kind of overstated
claim that gets a real finding discounted later.

**I still agree with the verdict: reject.** But on the two reasons that survive: (a) the sibling
`VirtuosoMessageList` is a commercial product with its own EULA, which is the permissive-core /
commercial-add-on shape we were told to catch, and (b) `virtua` and `@tanstack/react-virtual` are
equally capable, both MIT at repo *and* package level, and both released within the last three
weeks. **Reject on redundancy plus adjacency risk, not on "it has no licence".**

### D-3 (material). `keyring-rs` is no longer at `hwchen/keyring-rs` — the project changed owner

The researcher cites `https://github.com/hwchen/keyring-rs`. The crate's own manifest disagrees:

> `https://raw.githubusercontent.com/hwchen/keyring-rs/master/Cargo.toml` (read 2026-09-28) →
> `repository = "https://github.com/open-source-cooperative/keyring-rs"`

and crates.io agrees: `https://crates.io/api/v1/crates/keyring` (read 2026-09-28, UA set — the API
returns HTTP 403 without one) reports `repository: https://github.com/open-source-cooperative/keyring-rs`,
newest `4.2.0` published 2026-08-29, `"license": "MIT OR Apache-2.0"` on 4.2.0, 4.1.6 and 4.1.5.

The licence is unchanged and the dual grant is real — I read both files at the new home:
`LICENSE-MIT` (*"Copyright (c) 2016 keyring Developers"*) and `LICENSE-APACHE` (Apache License
Version 2.0), at `raw.githubusercontent.com/open-source-cooperative/keyring-rs/master/`, 2026-09-28.
Note the filenames are `LICENSE-MIT`/`LICENSE-APACHE`; `LICENSE.MIT` is a 404.

**Consequence: an ownership transfer happened, and our citation points at the old owner.** A
transfer is exactly the event that precedes a relicence. The SBOM entry and any allowlist must name
`open-source-cooperative/keyring-rs`, and the crate should be version-pinned rather than
caret-ranged so a future owner's licence change cannot arrive in a `cargo update`.
**Verdict unchanged: yes, adopt, take the MIT arm.**

### D-4 (material). CodeMirror has left GitHub. Every `codemirror/*` repo is archived.

The researcher cites `https://github.com/codemirror` for `@codemirror/merge` and `@codemirror/view`.
Those repositories are **archived**:

> `gh api repos/codemirror/view` → `MIT | 178★ | pushed 2026-04-15 | archived: true`
> `gh api repos/codemirror/merge` → `MIT | 105★ | pushed 2026-04-15 | archived: true`
> `gh api repos/codemirror/dev` → `NOASSERTION | 7817★ | pushed 2026-04-15 | archived: true`

Meanwhile the packages are still being released — `@codemirror/view@6.43.13` was published
**2026-09-22**, three months after the GitHub mirror froze. The `repository` field explains it:

> `"repository": "git+https://code.haverbeke.berlin/codemirror/view.git"`
> (https://registry.npmjs.org/@codemirror/view, read 2026-09-28)

**I could not reach `code.haverbeke.berlin` from this machine** — `curl` fails with
`schannel: server closed abruptly (missing close_notify)`, which is the Netskope TLS interception
signature. So the canonical repository is **UNREACHABLE from the corporate network** and the
GitHub copy is a frozen mirror.

I therefore verified the licence from the strongest available source, the published artifact itself:

> `https://unpkg.com/@codemirror/view@6.43.13/LICENSE` → `MIT License` /
> `Copyright (C) 2018-2021 by Marijn Haverbeke <marijn@haverbeke.berlin> and others`
> `https://unpkg.com/@codemirror/merge@6.12.2/LICENSE` → `MIT License` / `Copyright (C) 2018-2022 …`
> (both read 2026-09-28)

**Verdict: yes, MIT, confirmed at the artifact.** Two consequences worth recording: any licence gate
that resolves SPDX by walking to the GitHub repo will read an archived mirror (and `codemirror/dev`
reads `NOASSERTION`), and we cannot audit future CodeMirror commits from a KPMG machine without a
proxy exception. Audit the **tarball**, not the repo, for this dependency.

### D-5 (material). Three "active" projects have not shipped a release in 18 months. `pushed_at` is not a release.

The researcher used GitHub `pushed_at` as the maturity evidence. For three candidates that is
misleading, because commits are landing on `main` while npm sits still:

| Package | Researcher's evidence | npm `latest` publish date (read 2026-09-28) | Gap |
| --- | --- | --- | --- |
| `uplot` 1.6.32 | "10,518★, pushed 2026-09-27" — marked **adopt** | **2025-03-14** | **18 months, no release** |
| `cmdk` 1.1.1 | MIT, 12,991★ | **2025-03-14** | **18 months, no release**; repo last pushed 2025-10-29 |
| `@base-ui-components/react` | "MIT, v1.0.0-rc.0" | **2025-12-04** | **10 months on the same release candidate** |

None of this changes a licence verdict. It changes two adoption verdicts against the task's own
"prefer projects with a release in the last 12 months" bar:
- **uPlot**: still adopt — it is a small, finished, single-file charting library and a frozen release
  is a reasonable state for one — but the plan must say *"last release 2025-03-14"* rather than
  *"pushed 2026-09-27"*, or the next reviewer will find the discrepancy and distrust the rest.
- **Base UI**: do not pin a `1.0.0-rc.0` that has stood still for ten months into a KPMG product.
  Wait for 1.0.0 or treat it as study-only. (Licence is clean: `gh api repos/mui/base-ui/license` →
  `LICENSE | MIT`, read 2026-09-28. It is a separate repo from MUI X, which is the commercial one.)
- **cmdk**: the researcher already rejected it on behaviour. The 18-month gap is a second reason.

### D-6 (minor, but it changes what a CI gate must pin). `dockview` — confirmed, and the trap is sharper than reported

The researcher's finding is correct and I reproduced it at source:

> `https://raw.githubusercontent.com/dockview/dockview/master/LICENCE.md` (read 2026-09-28):
> *"This repository is a monorepo containing several packages, and **not all of them share the same
> licence.**"* … `dockview-enterprise` → *"**Proprietary — commercial licence**"* … *"`dockview-enterprise`
> is **not** covered by the MIT Licence; it is proprietary software governed solely by its own
> commercial licence agreement."*

Two additions. First, the canonical repo is **`dockview/dockview`** (an org), and `mathuo/dockview`
redirects to it — cite the org. Second, and this is the sharp bit: **`dockview-core@8.3.1` declares
a bare `"license":"MIT"` on npm** and so does `dockview@8.3.1`, while `gh api repos/mathuo/dockview`
reports `NOASSERTION`. A gate that reads the **package** sees clean MIT and lets the whole family
through, including a future `dockview-enterprise` pulled in as a peer. **If dockview is ever
reconsidered, the allowlist must name packages, not the repo, and must explicitly deny-list
`dockview-enterprise`.** Verdict unchanged: **reject**.

### D-7 (minor). Two of the four "adjacent cockpits" are unusable, and for sharper reasons than recorded

- `kochul2000/laymux` — the researcher recorded *"NONE DECLARED / license: null"*. Confirmed:
  `gh api repos/kochul2000/laymux/license` → **HTTP 404**, no licence file of any kind
  (read 2026-09-28). The correct statement is stronger than "none declared": **no licence means no
  grant — all rights reserved by default.** Copying one line of it is infringement. **No.**
- `awaysu/AwayTerminal2` — recorded as `NOASSERTION`. I read the file. It is **plain MIT text**
  (`Copyright (c) 2026 Chih-Wei Su (Awaysu)`) with a trailing paragraph appended —
  *"This product bundles third-party components with their own licenses. See THIRD-PARTY-NOTICES.md"*
  — which is why GitHub's classifier declines to assert. The project's **own** code is MIT
  (https://raw.githubusercontent.com/awaysu/AwayTerminal2/main/LICENSE, 1,219 bytes, read 2026-09-28).
  It is still a **no**, on maturity not licence: 0 stars, first commits this month.
- `Surething-io/cockpit` (OpenCockpit) — MIT confirmed at
  `raw.githubusercontent.com/Surething-io/cockpit/main/LICENSE` (*"Copyright (c) 2025 Robert"*),
  37★, pushed 2026-09-23. Licence-clean; **study-only** on maturity, and note it is built on the
  Claude Agent SDK, which is a different harness assumption from ours.

---

## 3. The table — claimed, verified, source, verdict, obligation

`Verified` = what I read at source on **2026-09-28**. `Agrees` = whether the researcher's claim
survives. `Verdict` = yes / conditional / no, on **licence and licence-adjacent risk only**; where I
also disagree on adoption for a non-licence reason, it is said in the obligation column.

| # | Candidate | Claimed | Verified 2026-09-28 | Source read | Agrees | Verdict | Obligation we take on |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `@xterm/addon-webgl` 0.19.0 | MIT | **MIT** | `registry.npmjs.org/@xterm/addon-webgl/latest` → `"license":"MIT"`; repo `LICENSE` (xtermjs/xterm.js master) is MIT text with 3 copyright lines (xterm.js authors / SourceLair / Christopher Jeffrey) | yes | **yes** | MIT notice; carry **all three** copyright lines, not just "xterm.js authors". No runtime deps. |
| 2 | `@xterm/addon-serialize` 0.14.0 | MIT | **MIT** | same registry + repo `LICENSE` | yes | **yes** | as #1. No deps. |
| 3 | `@xterm/addon-search` 0.16.0 | MIT | **MIT** | `registry.npmjs.org/@xterm/addon-search/latest` | yes | **yes** | as #1. No deps. |
| 4 | `@xterm/addon-unicode11` 0.9.0 | MIT | **MIT** | registry | yes | **yes** | as #1. No deps. |
| 5 | `@xterm/addon-web-links` 0.12.0 | MIT | **MIT** | registry | yes | **yes** | as #1. No deps. |
| 6 | `@xterm/addon-clipboard` 0.2.0 | MIT | **MIT** | registry | **partly** | **yes** | **Researcher missed its one dependency.** It ships `js-base64`, which is **BSD-3-Clause**, not MIT (`registry.npmjs.org/js-base64/latest` → 3.9.4, `"license":"BSD-3-Clause"`). Permissive and allowed, but BSD-3 adds a **no-endorsement clause** and a second notice to carry. Every other xterm addon has zero deps; this one does not. |
| 7 | `@xterm/addon-attach` 0.12.0 | MIT | **MIT** | `registry.npmjs.org/@xterm/addon-attach/latest` | yes | **yes** | as #1. No deps. |
| 8 | `@xterm/headless` 6.0.0 | MIT | **MIT** | `registry.npmjs.org/@xterm/headless/latest` | yes | **yes** | as #1. No deps. Licence is not the constraint here; the Node-in-a-Go-control-plane question in `07-ui-primitives.md` §2 is, and that stands. |
| 9 | `tauri-apps/plugins-workspace` — deep-link, single-instance, updater, window-state, store, log, global-shortcut, os, process, notification, dialog | `MIT OR Apache-2.0`; GitHub classifier says Apache-2.0 | **`Apache-2.0 OR MIT`, dual, confirmed at the Rust source of truth** | `raw.githubusercontent.com/tauri-apps/plugins-workspace/v2/Cargo.toml` → `[workspace.package] license = "Apache-2.0 OR MIT"`; `LICENSE_MIT` present ("Copyright (c) 2017 - Present Tauri Apps Contributors"); per-plugin `Cargo.toml` inherits `license = { workspace = true }`; crates.io `tauri-plugin-deep-link` 2.5.0 and `tauri-plugin-updater` 2.13.0 both `"Apache-2.0 OR MIT"` | yes | **yes** | Elect **MIT** in writing, once, in `NOTICES` — a dual grant is only unambiguous if we record which arm we took. The npm shim declares `"MIT OR Apache-2.0"` and Cargo declares `"Apache-2.0 OR MIT"`; same set, and the **Cargo** one governs the code that ships in the binary. |
| 10 | `keyring` (keyring-rs) 4.2.0 | `MIT OR Apache-2.0`, repo `hwchen/keyring-rs` | **`MIT OR Apache-2.0`** — but **the repo moved** | crates.io `/api/v1/crates/keyring` → `"license":"MIT OR Apache-2.0"` on 4.2.0 (2026-08-29), 4.1.6, 4.1.5; `repository: open-source-cooperative/keyring-rs`; `LICENSE-MIT` + `LICENSE-APACHE` read at that org | **licence yes, citation no** — see **D-3** | **yes** | Elect MIT. **Re-point the citation to `open-source-cooperative/keyring-rs`** and version-pin, because an ownership transfer is the event that precedes a relicence. crates.io returns **HTTP 403 without a User-Agent** — set one in any CI licence check. |
| 11 | `tauri-plugin-oauth` (FabianLars) | `MIT OR Apache-2.0` dual; GitHub says Apache-2.0 | **`MIT OR Apache-2.0`, dual** | repo `LICENSE_MIT` ("Copyright (c) 2022 - Present FabianLars"); crates.io newest **2.1.0** (2026-07-07) `"MIT OR Apache-2.0"` | yes | **yes (vendor)** | Two traps. (a) `main`'s `Cargo.toml` reads `version = "1.0.0"` — **`main` is the Tauri-v1 line**; the v2 crate is 2.1.0 on crates.io. Vendor from the **published 2.1.0 crate**, not from `main`. (b) Vendoring MIT is fine but the notice travels with the code: put the FabianLars copyright line **in the vendored file header** and in `NOTICES`, and record the source version in a comment so the next reader can diff it. |
| 12 | `@tanstack/react-virtual` 3.14.13 | MIT | **MIT** | `registry.npmjs.org/@tanstack/react-virtual/latest`; repo `LICENSE` = MIT ("Copyright (c) 2021-present Tanner Linsley"); `gh api repos/TanStack/virtual` → MIT, 7,123 stars, pushed 2026-09-21; published 2026-09-14 | yes | **yes** | MIT notice x2 — it ships `@tanstack/virtual-core` 3.17.11, also MIT (verified). |
| 13 | `virtua` 0.52.7 | MIT | **MIT** | `registry.npmjs.org/virtua/latest`; repo `LICENSE` = MIT ("Copyright (c) 2022 inokawa"); `gh api repos/inokawa/virtua` → MIT, 3,746 stars, pushed 2026-09-28; published 2026-09-22 | yes | **yes** | MIT notice. Zero runtime deps (framework peers only) — the cleanest of the two virtualisers, and the reason to prefer it over #12 if only one is taken. |
| 14 | `uPlot` 1.6.32 | MIT | **MIT** | `registry.npmjs.org/uplot/latest`; repo `LICENSE` = "The MIT License (MIT) — Copyright (c) 2022 Leon Sorokin"; `gh api repos/leeoniya/uPlot` → MIT, 10,518 stars | yes | **yes** | MIT notice. **Correct the maturity line:** last npm release **2025-03-14, 18 months ago** — see **D-5**. No deps. |
| 15 | `@floating-ui/react` 0.27.20 | MIT | **MIT** | `registry.npmjs.org/@floating-ui/react/latest`; repo `LICENSE` = MIT ("Copyright (c) 2021 Floating UI contributors"); `gh api repos/floating-ui/floating-ui` → MIT, 32,753 stars | yes | **yes** | MIT notice x4 — the tree it pulls is `@floating-ui/react-dom` 2.1.9, `@floating-ui/utils` 0.2.12, `@floating-ui/core` 1.8.0, `@floating-ui/dom` 1.8.0 and `tabbable` 6.5.0, **all verified MIT** on npm. Published 2026-07-11. |
| 16 | `react-resizable-panels` 4.14.1 | MIT | **MIT** | `registry.npmjs.org/react-resizable-panels/latest`; `gh api repos/bvaughn/react-resizable-panels/license` → **path `LICENSE.md`**, MIT (a bare `LICENSE` is a 404 — the researcher's URL shape would have missed it); 5,377 stars, published 2026-09-27 | yes | **yes** | MIT notice. No runtime deps. |
| 17 | `shiki` 4.4.3 | MIT (root LICENSE: Pine Wu 2021, Anthony Fu 2023) | **shiki's own code MIT — the bundled grammars are GPL-3.0 / MPL-2.0 / NOASSERTION** | repo `LICENSE` (MIT, both copyright lines as claimed); `@shikijs/langs`, `@shikijs/themes`, `@shikijs/core`, `@shikijs/engine-oniguruma`, `@shikijs/vscode-textmate` 10.0.2 all `"license":"MIT"` on npm; **`textmate-grammars-themes` `NOTICE`, 693,215 bytes, 155 blocks parsed** | **no — this is the material one** | **CONDITIONAL** | See **D-1**. Import `shiki/core` + explicit `@shikijs/langs/<lang>` only; checked-in grammar allowlist gated in CI; deny `ada ahk2 apache apex asciidoc bird2 cue elixir gnuplot hcl terraform llvm luau nginx nim nsis org purescript racket sass wolfram`; carry per-grammar MIT/Apache-2.0 notices (Apache-2.0 grammars also require the NOTICE text). |
| 18 | `cmdk` 1.1.1 | MIT | **MIT** | `registry.npmjs.org/cmdk/latest`; repo `LICENSE.md` = MIT ("Copyright (c) 2022 Paco Coursey"); `gh api repos/pacocoursey/cmdk` → MIT, 12,991 stars | yes | **yes (licence) / no (adopt)** | Licence clean, and its four `@radix-ui/*` deps are MIT (spot-checked `@radix-ui/react-dialog` 1.1.23). Rejected on two other grounds: **last release 2025-03-14, 18 months**, and it drags a Radix dialog into a shell that has its own overlay model. |
| 19 | `tinykeys` 4.0.1 | MIT | **MIT** | `registry.npmjs.org/tinykeys/latest`; `gh api repos/jamiebuilds/tinykeys/license` → **path `LICENSE`** (not `LICENSE.md` — that URL 404s), MIT; 4,099 stars, pushed 2026-09-25 | yes | **yes (licence) / no (adopt)** | No obligation beyond the MIT notice if taken. The researcher's rejection is on behaviour (adopting it would delete our own keydown fix) and is unrelated to licence. |
| 20 | `dockview` 8.3.1 | Mixed monorepo; `dockview-enterprise` proprietary; repo NOASSERTION | **Confirmed, verbatim** | `raw.githubusercontent.com/dockview/dockview/master/LICENCE.md`; `dockview` and `dockview-core` both `"license":"MIT"` at 8.3.1 on npm; `gh api repos/mathuo/dockview` → NOASSERTION, 3,446 stars | yes | **no** | If ever reconsidered: gate on **package** names and explicitly deny `dockview-enterprise`. See **D-6**. |
| 21 | `react-virtuoso` 4.18.15 | MIT per package.json **only**; no licence file in the repo; sibling commercial | **MIT in the repo: absent. MIT in the published tarball: present.** | `gh api repos/petyosi/react-virtuoso/license` → **HTTP 404** (confirmed); `unpkg.com/react-virtuoso@4.18.15/LICENSE` → **full MIT text, "Copyright (c) 2020 Petyo Ivanov"** | **no — see D-2** | **no** | Reject on **redundancy + commercial-sibling adjacency**, not on absence of a licence. Do not repeat "no licence file anywhere" in a memo; it is disprovable in one fetch. |
| 22 | `@melloware/react-logviewer` 6.6.0 | MPL-2.0 (conditional); upstream `react-lazylog` 4.5.3 also MPL-2.0 | **MPL-2.0, both** | `registry.npmjs.org/@melloware/react-logviewer/latest` → `"MPL-2.0"`; `registry.npmjs.org/react-lazylog/latest` → 4.5.3, `"MPL-2.0"`; `gh api repos/melloware/react-logviewer` → MPL-2.0, 169 stars, pushed 2026-09-27 | yes | **CONDITIONAL (and I recommend no)** | The exact MPL-2.0 obligation, so nobody has to guess: **(a)** any MPL-2.0 *file* we modify must be made available in source form under MPL-2.0, including for a customer-hosted deployment, because shipping the desktop binary to a customer is distribution; **(b)** the notice header stays in every such file; **(c)** it does **not** infect our own files — that is the point of file-level copyleft. The objection is ours specifically: the log pane is where the provenance thread is rendered, so it is exactly the file we *will* modify, so we would take obligation (a) with near certainty. Its dep tree is clean (`virtua`, `hotkeys-js`, `immutable`, `mitt`, `react-string-replace` — all MIT, verified), so a from-scratch pane over `virtua` gets the same engine with none of the obligation. |
| 23 | `libghostty` / `libghostty-vt` (Ghostty) | MIT | **MIT** | `raw.githubusercontent.com/ghostty-org/ghostty/main/LICENSE` → `MIT License` / "Copyright (c) 2024 Mitchell Hashimoto, Ghostty contributors"; `gh api repos/ghostty-org/ghostty` → MIT, 61,617 stars, pushed 2026-09-27; repo root holds a single `LICENSE`, no separate NOTICE or THIRD-PARTY file | yes | **yes (licence) / study-only (adopt)** | MIT notice if ever taken. The researcher's "reject on architecture, not licence" holds — we render in a WebView. Re-open when `libghostty-vt` tags 1.0. |
| 24 | Base UI `@base-ui-components/react` 1.0.0-rc.0 | MIT | **MIT** | `registry.npmjs.org/@base-ui-components/react/latest`; `gh api repos/mui/base-ui/license` → path `LICENSE`, MIT; 11,007 stars, pushed 2026-09-28 | yes | **yes (licence) / not yet (adopt)** | MIT notice; `@base-ui-components/utils` 0.2.2 also MIT (verified). **Distinct from MUI X, which is the commercial one — different repo, and I checked the right one.** Adoption blocker is **D-5**: `1.0.0-rc.0` published 2025-12-04 and unchanged for ten months. Do not pin an RC into a KPMG product. |
| 25 | `react-arborist` 3.16.0 | MIT | **MIT** | `registry.npmjs.org/react-arborist/latest`; `gh api repos/brimdata/react-arborist/license` → path `LICENSE`, MIT; 3,708 stars, published 2026-07-25 | yes | **yes (licence) / study-only (adopt)** | MIT notice x4 — it ships `react-dnd` 16.0.1, `react-window` 2.3.3, `redux` 5.0.1 and `use-sync-external-store`, all MIT (verified). A four-package tail for a tree we have not designed yet, which strengthens the researcher's "study-only". |
| 26 | `visx` (`@visx/visx` 4.0.0) | MIT | **MIT** | `registry.npmjs.org/@visx/visx/latest`; `gh api repos/airbnb/visx/license` → path `LICENSE`, MIT; 21,064 stars, published 2026-06-11 | yes | **yes (licence) / no (adopt)** | **Do not depend on the `@visx/visx` umbrella**: it declares **33** `@visx/*` dependencies and pulls the whole library for one chart. If visx is ever used, depend on the two or three leaf packages and verify those individually. uPlot (#14) is the better answer for live meters. |
| 27 | `@codemirror/merge` 6.12.2 + `@codemirror/view` 6.43.13 | MIT | **MIT — verified at the tarball, because the repo is archived** | `unpkg.com/@codemirror/merge@6.12.2/LICENSE` and `unpkg.com/@codemirror/view@6.43.13/LICENSE`, both MIT (Marijn Haverbeke); GitHub mirrors archived 2026-04-15; canonical `code.haverbeke.berlin` **unreachable through the KPMG proxy** | **licence yes, provenance no** — see **D-4** | **yes** | MIT notice. Audit the **tarball**, not the GitHub repo, for this family; its deps `@codemirror/state`, `@codemirror/language`, `@lezer/highlight`, `style-mod`, `crelt`, `w3c-keyname` need the same treatment. Record that upstream review needs a proxy exception. |
| 28 | `Surething-io/cockpit` (OpenCockpit) | MIT, 37 stars | **MIT** | `raw.githubusercontent.com/Surething-io/cockpit/main/LICENSE` → MIT, "Copyright (c) 2025 Robert"; pushed 2026-09-23 | yes | **yes (licence) / study-only** | Clean, but built on the Claude Agent SDK — a different harness assumption from ours (`OD-8`). Read for ideas; do not depend. |
| 29 | `timwukp/agent-terminal` | MIT, 0 stars | **MIT** (GitHub licence API; 0 stars, pushed 2026-09-24) | `gh api repos/timwukp/agent-terminal` | yes | **yes (licence) / no** | Zero stars, days old. No. |
| 30 | `kochul2000/laymux` | "NONE DECLARED / license: null" | **No licence file at all — `gh api repos/kochul2000/laymux/license` → HTTP 404** | read 2026-09-28 | **yes, but understated** | **NO — hard** | **No licence means no grant, i.e. all rights reserved.** Not "unlicensed, proceed with care" — copying any of it infringes, and there is no permission to create a derivative, so it should not be mined for code at all. |
| 31 | `awaysu/AwayTerminal2` | NOASSERTION | **MIT plus an appended third-party-bundle notice** (which is *why* the classifier says NOASSERTION) | `raw.githubusercontent.com/awaysu/AwayTerminal2/main/LICENSE`, 1,219 bytes, read 2026-09-28 | **no — see D-7** | **no (maturity)** | Own code MIT; bundles unaudited third-party components via `THIRD-PARTY-NOTICES.md`. 0 stars, days old. No. |

---

## 4. Late addition to D-7 — `awaysu/AwayTerminal2` is worse than "MIT plus a notice"

Having found the pointer, I read the file it points at. `THIRD-PARTY-NOTICES.md`
(https://raw.githubusercontent.com/awaysu/AwayTerminal2/main/THIRD-PARTY-NOTICES.md, read
2026-09-28) names, by licence-string frequency: MIT 50, Apache-2.0 21, ISC 13, BSD-3-Clause 5,
BSD-2-Clause 3, **MPL-2.0 4, LGPL 2, GPL-3.0 1, AGPL 1.**

I did **not** open any of the copyleft-covered source, and there is no reason to: a 0-star,
days-old project that self-declares an AGPL entry in its own bundle is a hard **no**, and it should
not be read for ideas either, because "how they did it" is exactly the exposure the Paperdesign
withdrawal was about. Recording it here so nobody re-discovers it as a fresh idea in three weeks.

---

## 5. The obligations register — what adopting the "yes" list actually commits us to

Nothing on the yes list requires source disclosure, a copyleft grant, a field-of-use limit, or a
licence key. The whole obligation is **notice and attribution**, plus two elections. Concretely, a
`NOTICES` file shipped with the desktop binary and the Hub image must carry:

1. **MIT notices** for: xterm.js (all three copyright lines), `@tanstack/react-virtual` +
   `@tanstack/virtual-core`, `virtua`, `uPlot`, `@floating-ui/{react,react-dom,utils,core,dom}` +
   `tabbable`, `react-resizable-panels`, `shiki` + each shipped `@shikijs/*`,
   `@codemirror/{merge,view,state,language}` + `@lezer/highlight` + `style-mod` + `crelt` +
   `w3c-keyname`, and every grammar we actually bundle.
2. **BSD-3-Clause notice** for `js-base64`, pulled in by `@xterm/addon-clipboard` — including its
   **no-endorsement** clause. This is the only non-MIT licence on the whole yes list, and the
   researcher did not have it.
3. **A written election of the MIT arm** for the two dual-licensed Rust dependencies — the Tauri
   plugins (`Apache-2.0 OR MIT`) and `keyring` (`MIT OR Apache-2.0`) — recorded once, in
   `NOTICES`, rather than left implicit. If instead we ever elect the Apache-2.0 arm, that brings
   the `NOTICE`-propagation and patent-termination terms with it; electing MIT avoids both.
4. **Apache-2.0 NOTICE propagation** for any Apache-2.0 TextMate grammar we bundle (22 of the 155
   are Apache-2.0). Apache-2.0 §4(d) requires carrying the upstream `NOTICE` text, which MIT does
   not — so the grammar allowlist has to record *which* licence each entry carries, not just that
   it passed.
5. **An in-file attribution header** in the vendored `tauri-plugin-oauth` code, naming FabianLars,
   the crate version vendored (2.1.0), and the date.

## 6. What a CI licence gate must do differently because of this audit

Five rules, each of which exists because something in this audit would have slipped past a naive
gate:

1. **Gate on packages, not repositories.** `dockview-core` reads MIT while its repo reads
   NOASSERTION (D-6); `react-virtuoso` reads MIT in the tarball while its repo has no licence file
   at all (D-2). Repository-level SPDX is the wrong resolution unit in both directions.
2. **Resolve the licence from the published artifact.** For CodeMirror the GitHub repo is an
   archived mirror and the canonical host is unreachable from the corporate network (D-4). The
   tarball's own `LICENSE` is the only source we can read *and* the only one that describes what
   we ship.
3. **Walk into data, not just code.** `shiki` declares MIT and redistributes GPL-3.0 (D-1). Any
   package that bundles grammars, themes, icon sets, fonts or dictionaries needs its own NOTICE
   parsed, because the npm `license` field describes the wrapper.
4. **Alert on repository transfer, not only on licence change.** `keyring-rs` moved owner between
   the researcher's citation and this audit with no licence change (D-3). A transfer is the
   precursor event; catching it a release later is catching it too late.
5. **Set a User-Agent, and treat a fetch failure as a failure.** `crates.io` returns HTTP 403 to a
   bare `curl`; `code.haverbeke.berlin` fails TLS through Netskope. A gate that silently treats an
   unreadable licence as a pass is worse than no gate.

## 7. What I could not verify — stated as UNVERIFIED, not inferred

- **`code.haverbeke.berlin`** (canonical CodeMirror host) — unreachable from this machine,
  `schannel: server closed abruptly (missing close_notify)`. CodeMirror's licence is nonetheless
  **verified** from the published tarballs; what is unverified is the *upstream repository's*
  current licence file and any commit-level relicence signal. Needs a proxy exception to audit.
- **`crates.io` version history** — reachable only with a User-Agent set. I read the current
  licence on the three newest `keyring` versions (4.2.0, 4.1.6, 4.1.5) and all three agree; I did
  **not** walk the full version history for an older, differently licensed line.
- **Per-grammar licence *text*** for shiki — I parsed 155 `Files:/License:/SPDX:` blocks and read
  the SPDX ids and the upstream licence URLs. I did **not** fetch all 155 upstream licence files to
  confirm each SPDX id is itself correct. The 12 `NOASSERTION` entries in particular are
  *unresolved by upstream*, not *verified as permissive* — they must be treated as denied until
  someone reads them one by one.
- **`dockview-enterprise`'s own `packages/dockview-enterprise/LICENCE.md`** — not fetched. The root
  `LICENCE.md` is unambiguous that it is proprietary and that is sufficient to reject; reading the
  commercial terms of a package we are not taking adds nothing.
- **Tauri plugin crates individually** — I verified the workspace `license` field (which every
  plugin inherits via `license = { workspace = true }`, confirmed in `deep-link` and `updater`) and
  spot-checked two on crates.io. I did **not** open all eleven `Cargo.toml` files. The inheritance
  mechanism makes a per-plugin divergence structurally impossible without editing the manifest, so
  this is low risk, but it is a spot check and not a census.

---

## 8. Bottom line

**Adopt as the researcher proposed, with four changes:**

1. `shiki` moves from **adopt** to **conditional**, with the explicit-grammar-import rule and the
   21-entry deny-list in D-1. This is the one finding that would have put GPL-3.0 into a KPMG
   product.
2. `keyring` keeps its **adopt**, with the citation re-pointed to `open-source-cooperative/keyring-rs`.
3. `@xterm/addon-clipboard` keeps its **adopt**, with `js-base64` (BSD-3-Clause) added to `NOTICES`.
4. `react-virtuoso`'s rejection keeps its verdict but loses its stated reason; `laymux`'s rejection
   gets harder (no grant at all); `Base UI` and `uPlot` get accurate maturity lines.

**Nothing on the adopt or vendor list is AGPL, GPL, LGPL, SSPL, BUSL, Elastic, Commons Clause or
source-available.** The one MPL-2.0 candidate was already flagged conditional by the researcher and
I recommend declining it for a product reason rather than a legal one.
