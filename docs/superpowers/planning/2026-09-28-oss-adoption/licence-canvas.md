# Licence audit — Phase 5 (design canvas) candidates

**Adversarial verification of `02-canvas.md`. Audit date 2026-09-28.** Every licence below was
re-fetched from the project's own `LICENSE`/`COPYING` file or its published package metadata, by
`curl` on this machine, and read as raw bytes — not from a badge, a GitHub sidebar, a blog or
memory. Where the researcher and I disagree, the disagreement is stated plainly with the bytes that
settle it.

**Clean-room discipline held.** No source was opened for any copyleft project. For Webstudio,
Penpot, `@y/hub` and Liveblocks I fetched the **licence file and package metadata only** — which is
the record that we looked, and is not source.

---

## 1. Verdict in one paragraph

**No forbidden licence found in anything the researcher marked adopt or vendor.** Every adopt-list
item verifies as MIT, Apache-2.0, BSD-3-Clause or ISC at source. The researcher's four headline
findings — Excalidraw is genuinely MIT, tldraw is source-available with licence-key enforcement,
`y-redis` is AGPL-or-proprietary, and every open-source Figma/Webflow is copyleft — **all hold, and
two of them are stronger than stated.** But there are **eight material corrections**, three of
which change what gets pinned in a lockfile and one of which changes what ships in the Kata image.

---

## 2. The disagreements — read these before the table

### D-1 (material). "Yjs v14.0.0-rc.26" is not the package you would install

The researcher verified `https://raw.githubusercontent.com/yjs/yjs/main/package.json` and reported
**"MIT, v14.0.0-rc.26"**. That file is real and it is MIT — but its `"name"` field reads
**`"@y/y"`**, not `"yjs"`. The Yjs project has moved `main` onto a **v14 release-candidate line
published under a brand-new npm scope**. The package an implementer actually installs is:

- `yjs` — **latest `13.6.33`**, `"license": "MIT"`, one runtime dep `lib0 ^0.2.99`
  (https://registry.npmjs.org/yjs/latest, read 2026-09-28)
- versus `@y/y` — `14.0.0-rc.26`, `"license": "MIT"`, dep `lib0 ^1.0.0-rc.29`
  (https://raw.githubusercontent.com/yjs/yjs/main/package.json, read 2026-09-28)

Both are MIT, so **the licence verdict is unchanged — but the version boundary in the report is
wrong**, and it matters twice over. First, a plan that cites "Yjs 14.0.0-rc.26" is citing a
release candidate for a KPMG product. Second, see D-3: the AGPL scale-out backend is pinned to
exactly that RC line, so the two are entangled.

Same defect on **y-websocket**: repo `master` is **`@y/websocket` 4.0.0-rc.4** (MIT); the shipping
package is **`y-websocket` 3.1.0**, `"license": "MIT"`, deps `lib0`, `y-protocols`, peer `yjs ^13.5.6`
(https://registry.npmjs.org/y-websocket/latest, read 2026-09-28). And on **y-protocols**: npm
`y-protocols@1.0.7` is MIT as reported, but the v14-line twin is the differently-named
**`@y/protocols`**. Three packages, two npm namespaces, and the report mixes them.

**Action: the SBOM allowlist must name `yjs`, `y-protocols`, `y-websocket` AND `@y/y`,
`@y/protocols`, `@y/websocket` separately.** A gate written from the report's version numbers would
be a gate written for the RC line.

### D-2 (material). The MCP Go SDK is not "Apache-2.0". It is Apache-2.0 **AND** MIT, in one tree

The researcher's row says "Apache-2.0 — LICENSE says code and specifications Apache-2.0, docs
CC-BY-4.0, pre-transition contributions MIT". The parenthetical is right; the headline is not. The
first four lines of the file
(https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/LICENSE, read 2026-09-28) say:

> "The MCP project is undergoing a licensing transition from the MIT License to the Apache License,
> Version 2.0 ("Apache-2.0"). … Contributions made by authors who originally licensed their work
> under the MIT License and who have not yet granted explicit permission to relicense **remain
> licensed under the MIT License**. No rights beyond those granted by the applicable original
> license are conveyed for such contributions."

That is a **live, incomplete relicence**. The correct SPDX expression for the artifact today is
**`Apache-2.0 AND MIT`**, not `Apache-2.0`, and the obligation set is the union of both: Apache-2.0
§4 (retain NOTICE, state changes, patent grant/termination) **plus** MIT attribution. Any generated
attribution page that prints one licence for this module is wrong. `go.mod` deps are clean
(`golang-jwt/jwt/v5` MIT, `google/go-cmp` BSD-3, `google/jsonschema-go` Apache-2.0).

### D-3 (agree, and stronger). `y-redis` is worse than the README quote — the package metadata says it

The researcher quoted the README. There is harder evidence. The repository's own `package.json`
(https://raw.githubusercontent.com/yjs/y-redis/master/package.json, read 2026-09-28) reads:

```
"name": "@y/hub",
"version": "0.9.0",
"license": "AGPL-3.0 OR PROPRIETARY",
"repository": { "url": "git+https://github.com/yjs/yhub.git" }
```

A machine-readable `AGPL-3.0 OR PROPRIETARY` in the licence field means **any SBOM scanner will
catch it — if the gate names the right string.** The report's deny list says `y-redis` and `y/hub`.
Neither is the npm package name. **The deny list must read `@y/hub`, `y-redis` (the old name),
`yjs/y-redis` and `yjs/yhub` (the old and new repo paths).** Note also its dependency on
`@y/y ^14.0.0-rc.24` — the AGPL backend is built on exactly the RC line D-1 says the report
mistook for shipping Yjs.

### D-4 (agree, and sharper). The AGPL Liveblocks CLI is published under the bare name `liveblocks`

`LICENSE` (https://raw.githubusercontent.com/liveblocks/liveblocks/main/LICENSE, read 2026-09-28)
in full:

> "2. The following components are licensed under the GNU Affero General Public License v3.0 or
> later (AGPL-3.0-or-later) … - `packages/liveblocks-server` (published as `@liveblocks/server`)
> - `tools/liveblocks-cli` (published as `liveblocks`)"

The researcher had this right. What the report does not say is that **the AGPL half is published as
the unscoped, most-obvious-to-type npm package `liveblocks`.** A developer running
`npm i liveblocks` to "try the collaboration thing" installs AGPL-3.0-or-later code into the
lockfile with no scope prefix to warn them. Deny `liveblocks` and `@liveblocks/server` by name; the
remaining `@liveblocks/*` client packages are Apache-2.0 and would only be useful against their
hosted service anyway.

### D-5 (material, and missed entirely). Playwright is Apache-2.0. **The browsers it installs are not.**

`LICENSE` at `microsoft/playwright` is the Apache-2.0 text and `playwright@1.63.0` publishes
`"license": "Apache-2.0"` — both verified 2026-09-28. That is where the report stops, and the
report proposes Playwright as a **server-side component inside the Kata image** for the handoff
bundle's PNG per screen. That is redistribution, and Playwright's npm package is not what gets
redistributed — the browser binaries it downloads are.

`packages/playwright-core/browsers.json`
(https://raw.githubusercontent.com/microsoft/playwright/main/packages/playwright-core/browsers.json,
read 2026-09-28) lists five artifacts with **`"installByDefault": true`**: `chromium`,
`chromium-headless-shell`, **`firefox`**, **`webkit`**, and **`ffmpeg`**. Playwright's own
`ThirdPartyNotices.txt` covers only the npm code it inlines — *"Each bundled output has a sidecar
`<bundle>.js.LICENSE` file"* — and says nothing about the browsers, because the browsers are not
in the npm package.

Firefox is MPL-2.0, WebKit is LGPL-2.1-or-later plus BSD, and an FFmpeg build is LGPL-2.1+ at
best. **A default `npx playwright install` in a Dockerfile puts weak-copyleft and LGPL binaries
into an image KPMG ships to a customer's cloud.**

**Obligation, and it is cheap to discharge:** pin `npx playwright install chromium
--with-deps` (or `chromium-headless-shell`), never bare `playwright install`; add a build assertion
that `firefox`, `webkit` and `ffmpeg` are absent from the image; and carry Chromium's own
third-party notices in the attribution bundle. Verdict stays **yes**, conditional on that pin.
This is the one row in the report where "Apache-2.0" as a whole answer would have shipped a defect.

### D-6 (material). Excalidraw is MIT. Its dependency tree contains a GPL-dual package one minor bump away

The report says Excalidraw is MIT and it is right — `LICENSE` is the MIT text, `Copyright (c) 2020
Excalidraw`, and `@excalidraw/excalidraw` publishes `"license": "MIT"`
(npm latest is **0.18.1**, not the 0.18.0 the report quotes from the repo file; both MIT). The
report then says, honestly, that **no dependency tree was audited transitively**. I audited the one
that matters, because "embed Excalidraw whole" means shipping its whole tree. Three results:

1. **A dated time bomb.** `@excalidraw/excalidraw` → `@excalidraw/mermaid-to-excalidraw@2.2.2` →
   `mermaid@^11.12.1`. **`mermaid@11.12.1` is MIT and has no `elkjs`** (npm, read 2026-09-28). But
   **`mermaid@12.0.0` adds `"elkjs": "^0.9.3"`**, and `elkjs@0.12.0` publishes
   **`"license": "EPL-2.0 OR GPL-3.0-or-later"`** (https://registry.npmjs.org/elkjs/latest, read
   2026-09-28). EPL-2.0 is not on our allow list and GPL-3.0-or-later is forbidden outright. The
   caret range `^11.12.1` will not float to 12 — **so we are clean today and become dirty the day
   `mermaid-to-excalidraw` bumps its major.** This is precisely the "a relicence arrives in a
   version bump" case, with a name and a version number attached. Gate `elkjs` by name now.
2. **A dual we must elect.** `mermaid@11.12.1` → `dompurify@^3.2.5`; `dompurify@3.4.16` publishes
   **`"license": "(MPL-2.0 OR Apache-2.0)"`**. Allowed — we elect **Apache-2.0** and the MPL
   obligation never arises — but the election must be recorded in the SBOM, not left as "MPL?".
3. **The fonts are an open question, and they are binaries we would redistribute.**
   `packages/excalidraw/fonts/` contains directories named **Assistant, Cascadia, ComicShanns,
   Emoji, Excalifont, Helvetica, Liberation, Lilita, Nunito, Virgil, Xiaolai**
   (github.com/excalidraw/excalidraw tree listing, read 2026-09-28). I probed nine plausible
   per-font licence paths (`LICENSE`, `LICENSE.txt`, `OFL.txt` under Excalifont, Nunito,
   Liberation, Xiaolai, ComicShanns, Assistant) and **every one returned HTTP 404**. So those
   third-party typefaces appear to sit under the repository's blanket MIT `LICENSE`, which is
   very unlikely to be their actual upstream licence — Nunito, Assistant, Lilita One and Cascadia
   are SIL OFL-1.1 families upstream, and one directory is named **Helvetica**, a registered
   trademark. **UNVERIFIED and it must not stay that way**: before any embedded Excalidraw ships,
   enumerate the font binaries actually in the built bundle and attach their upstream licences.
   OFL-1.1 is acceptable but is **not on the task's allow list**, so it needs a recorded exception.

None of this changes the verdict — **Excalidraw is yes** — but "MIT, verified" is not the whole
answer for a component we would embed whole.

### D-7 (disagreement). "React Flow Pro does not gate the library" is half true, and the missing half is a decision

The report states: *"'React Flow Pro' is a paid subscription for examples and support — it does not
gate the library"*, and lists the exact terms as unverified item #7. I read the vendor's own docs
page (https://reactflow.dev/learn/troubleshooting/remove-attribution, read 2026-09-28). It says:

> "Subscribing to React Flow Pro permits you to remove the attribution from your flows" … "Only
> remove this attribution, if you are subscribed to React Flow Pro"

The library really is MIT — root `LICENSE`, `Copyright (c) 2019-2025 webkid GmbH`, plain MIT, no
extra clauses; `@xyflow/react@12.12.0` publishes `"license": "MIT"` — so **legally we may set
`proOptions={{hideAttribution: true}}` without paying.** But React Flow renders a visible "React
Flow" badge on the canvas by default, and the vendor's documented position is that removing it is
a Pro benefit. For a KPMG product a regulated customer runs on their own cloud that is three bad
options, not zero: **ship a third party's badge inside the Design canvas** (which also collides
with `docs/DESIGN-SYSTEM.md`), **remove it against the vendor's stated request**, or **buy Pro**.
The report's phrase "it does not gate the library" is true and hides that choice. **Verdict: yes on
licence, with an owner decision attached** — and it feeds D-OSS-3, because "our own world layer on
`react-zoom-pan-pinch` (MIT, no badge) plus Moveable" has no badge question at all.

### D-8 (smaller, but pin them)

- **perfect-freehand is MIT — and its author is tldraw's author.** `LICENSE` is MIT,
  `Copyright (c) 2021 **Stephen Ruiz Ltd**`; npm `perfect-freehand@1.2.3` is MIT (both read
  2026-09-28). The same person and company relicensed **tldraw** to a source-available licence with
  key enforcement. Nothing is wrong with the package today; **the precedent is why the lockfile pin
  and the re-scan-on-bump rule exist.** The report treats "stable, not actively developed" as the
  only risk; the relicence risk is the more interesting one, and both point to the same mitigation.
- **`liveblocks` (unscoped) is the AGPL CLI** — see D-4. Deny the bare name.
- **`modern-screenshot` resolves fine**: `4.7.0`, `"license": "MIT"` (npm, read 2026-09-28). The
  report's "version field unparseable" is closed.
- **`@automerge/automerge-repo`'s published latest is `2.6.0-alpha.3`** (MIT). The report cites the
  repo `LICENSE` without noting the shipping artifact is an alpha.
- **Incantly `canvas`'s `LICENSE` reads `Copyright (c) 2026 Quickdraw contributors`** — a holder
  name that matches neither the org nor the repo. Reject stands; provenance is unclear on top of
  zero adoption.
- **`@use-gesture/react`'s `LICENSE` has no title line** — it opens `Copyright (c) 2018-present
  Paul Henschel` and then the MIT body. `package.json` says MIT. A scanner keyed on the first line
  will report `NOASSERTION`; add it to the same documented-exception list as Konva.
- **Konva's `NOASSERTION` explanation is correct** (dual `KineticJS`/`Konva` copyright header over
  plain MIT terms, verified). Note additionally that `konva@10.7.0` declares peer deps on `canvas`
  and `skia-canvas`, and `linkedom@0.18.13` (ISC) declares a peer dep on `canvas >= 2` —
  **node-canvas links Cairo, which is LGPL.** Both peers are optional and we should never install
  them; say so in the plan rather than discovering it in a server build.
- **`@hocuspocus/transformer` pulls the whole Tiptap editor** (`@tiptap/starter-kit@3.31.3`, MIT).
  Adopt `@hocuspocus/server` / `@hocuspocus/provider` only. Tiptap's paid Pro extensions live in a
  private registry under a commercial licence and must never enter the lockfile.
- **ygo's MIT holds and its Go dependency tree is clean** — `LICENSE` MIT, `Copyright (c) 2026
  Re:Earth contributors`; `go.mod` is `gorilla/websocket`, `redis/go-redis/v9`, `miniredis`,
  `testify`, `golang.org/x/*`, `modernc.org/sqlite` — permissive throughout, **no cgo, no copyleft**.
  Its README describes a **port of Yjs (MIT)** and positions its cluster relay as covering "the same
  ground" as y-redis rather than deriving from it, citing y-redis only as the *pattern* for
  per-item attribution. **A README cannot prove non-derivation from an AGPL project**: if ygo ever
  goes on the critical path, the pre-adoption check is a scan of its history and file headers for
  copied `y-redis`/`y/hub` code, not another read of the README. Maturity confirmed as the report
  states: **55 stars, 21 open issues, MIT in the sidebar** (repo page, read 2026-09-28).
- **y-sweet's `LICENSE` is MIT plus two verbatim third-party MIT notices** (y-crdt, and Yjs). Crate
  `y-sweet@0.9.1` `license = "MIT"`; `@y-sweet/sdk@0.9.1` MIT. Obligation: reproduce **all three**
  notices, not one.
- **GrapesJS is BSD-3-Clause, confirmed including the no-endorsement clause** (*"used to endorse or
  promote products derived from this software without…"*, line 13 of `packages/core/LICENSE`).
  Study-only, so the obligation is moot — but if anyone vendors a file, BSD-3 requires the notice
  **and** forbids using "GrapesJS" or the author's name to promote PanaceaApp.

---

## 3. The table — every candidate, claimed vs verified

All fetched 2026-09-28 from the URL given. "Agrees" = my reading matches `02-canvas.md`.

| # | Candidate | Claimed | Verified at source | Agrees | Usable | Obligation we take on |
|---|---|---|---|---|---|---|
| 1 | **Yjs** | MIT | **MIT**. `yjs@13.6.33` `"license":"MIT"`, dep `lib0 ^0.2.99` ([npm](https://registry.npmjs.org/yjs/latest)); repo `main` is **`@y/y@14.0.0-rc.26`**, MIT ([pkg](https://raw.githubusercontent.com/yjs/yjs/main/package.json)); `LICENSE` = MIT, *"Copyright (c) 2023 - Kevin Jahns … RWTH Aachen"* | **Partly** — version/package name wrong (D-1) | **yes** | MIT notice for Yjs + `lib0`. Pin `yjs@13.x`, not the `@y/*` RC line |
| 2 | **y-protocols** | MIT | **MIT**. `y-protocols@1.0.7` ([npm](https://registry.npmjs.org/y-protocols/latest)); `LICENSE` MIT, *"Copyright (c) 2019-2026 Kevin Jahns"* | Yes | **yes** | MIT notice. v14 twin is the separate package `@y/protocols` |
| 3 | **y-websocket** | MIT | **MIT**. shipping `y-websocket@3.1.0`, peer `yjs ^13.5.6` ([npm](https://registry.npmjs.org/y-websocket/latest)); repo `master` is `@y/websocket@4.0.0-rc.4` | **Partly** (D-1) | **yes** | MIT notice. Client only; the relay is ours |
| 4 | **React Flow / xyflow** | MIT | **MIT**. root `LICENSE` plain MIT *"Copyright (c) 2019-2025 webkid GmbH"*; `@xyflow/react@12.12.0` `"license":"MIT"`; `@xyflow/system@0.0.83` MIT | **Partly** — Pro/attribution understated (D-7) | **conditional** | MIT notice. **Owner call on the on-canvas "React Flow" badge**: keep it, buy Pro, or remove it against the vendor's request |
| 5 | **Moveable** | MIT | **MIT**. `moveable@0.53.0` ([npm](https://registry.npmjs.org/moveable/latest)); `LICENSE` MIT *"Copyright (c) 2019 Daybrush"*. Tree checked: `react-moveable@0.56.0`, `gesto@1.19.4`, `@daybrush/utils@1.13.0`, `@scena/event-emitter@1.0.5`, `@egjs/children-differ@1.0.1` — **all MIT** | Yes (and its tree verifies) | **yes** | MIT notices for Moveable + 5 transitive packages |
| 6 | **Selecto** | MIT | **MIT**. `selecto@1.26.3` ([npm](https://registry.npmjs.org/selecto/latest)); `LICENSE` MIT *"Copyright (c) 2020 Daybrush"*. Shares Moveable's tree | Yes | **yes** | MIT notices as above |
| 7 | **rbush** | MIT | **MIT**. `rbush@4.0.1` ([npm](https://registry.npmjs.org/rbush/latest)); `LICENSE` MIT *"Copyright (c) 2024 Volodymyr Agafonkin"*; dep `quickselect@3.0.0` **ISC** | Yes | **yes** | MIT + ISC notices |
| 8 | **perfect-freehand** | MIT | **MIT**. `LICENSE` MIT *"Copyright (c) 2021 Stephen Ruiz Ltd"*; `perfect-freehand@1.2.3` MIT | Yes | **yes** | MIT notice. Pin + re-scan on bump (D-8: same author relicensed tldraw) |
| 9 | **Rough.js** | MIT | **MIT**. `roughjs@4.6.6` ([npm](https://registry.npmjs.org/roughjs/latest)); `LICENSE` MIT *"Copyright (c) 2019 Preet Shihn"*; deps `hachure-fill`, `points-on-path`, `points-on-curve`, `path-data-parser` (not individually verified) | Yes | **yes** | MIT notice |
| 10 | **@use-gesture/react** | MIT | **MIT**. `@use-gesture/react@10.3.1` ([npm](https://registry.npmjs.org/@use-gesture/react/latest)); `LICENSE` is the MIT body with **no title line**, *"Copyright (c) 2018-present Paul Henschel"* | Yes | **yes** | MIT notice; scanner exception for the untitled file |
| 11 | **MCP Go SDK** | Apache-2.0 (+CC-BY docs, pre-transition MIT) | **`Apache-2.0 AND MIT`** — live relicence in progress, quoted in D-2 ([LICENSE](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/LICENSE)); `go.mod` deps MIT/BSD-3/Apache-2.0 | **No — headline wrong** (D-2) | **yes** | Apache-2.0 §4 NOTICE + state changes + patent terms, **and** MIT attribution. SBOM must carry both |
| 12 | **Excalidraw** | MIT | **MIT**. `LICENSE` MIT *"Copyright (c) 2020 Excalidraw"*; `@excalidraw/excalidraw@0.18.1` MIT; `@excalidraw/common`, `@excalidraw/element`, `@excalidraw/laser-pointer@1.3.2`, `@excalidraw/mermaid-to-excalidraw@2.2.2` all MIT | **Partly** — tree + fonts unexamined (D-6) | **conditional** | MIT notices; **gate `elkjs`**; elect Apache-2.0 for `dompurify`; resolve the bundled-font licences before shipping |
| 13 | **Playwright** | Apache-2.0 | **Apache-2.0** for the package (`LICENSE`; `playwright@1.63.0`). **The default-installed browsers are separately licensed** (D-5) | **No — incomplete** (D-5) | **conditional** | Apache-2.0 NOTICE; **install `chromium` only**; assert `firefox`/`webkit`/`ffmpeg` absent from the image; carry Chromium's notices |
| 14 | **Hocuspocus** | MIT | **MIT**. `LICENSE.md` MIT *"Copyright (c) 2023, Tiptap GmbH"*; `@hocuspocus/server@4.7.0`, `@hocuspocus/provider@4.7.0`, `@hocuspocus/extension-database@4.7.0` all MIT; dep `crossws@0.4.12` MIT | Yes | **yes** | MIT notices. Avoid `@hocuspocus/transformer` (drags Tiptap); never add Tiptap Pro |
| 15 | **GrapesJS** | BSD-3-Clause | **BSD-3-Clause**, endorsement clause present; `grapesjs@0.23.6` `"license":"BSD-3-Clause"`, deps `backbone`, `codemirror@5.63.0`, `underscore` (MIT family, not individually verified) | Yes | **yes** (study-only) | If ever vendored: BSD-3 notice + no-endorsement |
| 16 | **Craft.js** | MIT | **MIT**. `LICENSE` *"Copyright (c) 2020 Previnash Wong Sze Chuan"*; `@craftjs/core@0.2.12` MIT | Yes | **yes** (study-only) | MIT notice if used |
| 17 | **Puck** | MIT | **MIT**. `LICENSE` *"Copyright (c) The Puck Contributors."*; `@measured/puck@0.20.2` MIT | Yes | **yes** (study-only) | MIT notice if used |
| 18 | **Onlook** | Apache-2.0 | **Apache-2.0**, full text at `LICENSE.md` | Yes | **yes** (study-only) | Apache NOTICE if any code were reused; **deps not audited** |
| 19 | **Loro** | MIT | **MIT**. `LICENSE` *"Copyright (c) 2023 Loro"*; `loro-crdt@1.16.3` MIT | Yes | **yes** | MIT notice |
| 20 | **Automerge / automerge-repo** | MIT | **MIT**. `rust/automerge/Cargo.toml` `license = "MIT"` v0.12.0; `@automerge/automerge@3.5.0` MIT; **`@automerge/automerge-repo@2.6.0-alpha.3`** MIT (alpha) | Yes, with the alpha unstated | **yes** | MIT notice |
| 21 | **ygo (pure-Go Yjs)** | MIT (repo page) | **MIT at file.** `LICENSE` MIT *"Copyright (c) 2026 Re:Earth contributors"*; `go.mod` all permissive | Yes, strengthened | **yes** (spike only) | MIT notice + Yjs attribution; **pre-adoption: prove non-derivation from AGPL `y/hub`** |
| 22 | **y-sweet** | MIT | **MIT ×3.** `LICENSE` = MIT + verbatim y-crdt MIT + Yjs MIT; crate `y-sweet@0.9.1` MIT; `@y-sweet/sdk@0.9.1` MIT | Yes | **yes** | Reproduce all three MIT notices |

| # | Candidate | Claimed | Verified at source | Agrees | Usable | Obligation / reason |
|---|---|---|---|---|---|---|
| 23 | **tldraw** | Proprietary / source-available | **Confirmed.** `LICENSE.md`: *"This License from tldraw, Inc. … governs your use"*; *"Not to use the Software in Production Environments"*; *"Alternative licenses are available … for commercial and non-commercial use"* ([LICENSE.md](https://raw.githubusercontent.com/tldraw/tldraw/main/LICENSE.md)) | Yes | **no** | Not open source. Production use forbidden without a paid key; the report's added point about **licence-key enforcement and deployment detection** is the stronger objection for a customer-hosted product |
| 24 | **Konva** | MIT (GitHub says NOASSERTION) | **MIT**. `LICENSE` = MIT terms under a dual header *"Original work Copyright (C) 2011-2013 Eric Rowell (KineticJS) / Modified work … Anton Lavrenov"*; `konva@10.7.0` `"license":"MIT"` | Yes | **yes** on licence, **no** on architecture | MIT notice if ever used. Peer deps `canvas`/`skia-canvas` would pull Cairo (LGPL) — do not install |
| 25 | **Fabric.js** | MIT | **MIT**. `fabric@7.4.0` `"license":"MIT"` ([npm](https://registry.npmjs.org/fabric/latest)) | Yes | **yes** on licence, **no** on architecture | Canvas renderer; incompatible with real-HTML frames |
| 26 | **PixiJS** | MIT | **MIT**. `pixi.js@8.21.0` `"license":"MIT"` ([npm](https://registry.npmjs.org/pixi.js/latest)) | Yes | **yes** on licence, **no** on architecture | As above |
| 27 | **Liveblocks** | Split Apache-2.0 / AGPL-3.0-or-later | **Confirmed verbatim** (D-4): AGPL covers `packages/liveblocks-server` (**published as `@liveblocks/server`**) and `tools/liveblocks-cli` (**published as `liveblocks`**) | Yes, sharpened | **no** | Deny `liveblocks` **and** `@liveblocks/server` by name. The Apache client is only useful against their SaaS or their AGPL server |
| 28 | **y-redis / "y/hub"** | AGPL-3.0 or proprietary (dual) | **Confirmed, and machine-readable**: `package.json` `"name":"@y/hub"`, `"version":"0.9.0"`, **`"license":"AGPL-3.0 OR PROPRIETARY"`**, repo now `yjs/yhub` | Yes, strengthened (D-3) | **no** | Deny `@y/hub`, `y-redis`, `yjs/y-redis`, `yjs/yhub`. Scale-out path is y-sweet or our own Go persistence |
| 29 | **Incantly `canvas`** | MIT | **MIT text**, but *"Copyright (c) 2026 **Quickdraw contributors**"* — holder does not match repo or org | Yes, with a provenance doubt added | **no** | Zero adoption + unclear provenance |
| 30 | **ElectricSQL** | Apache-2.0 | **Apache-2.0**, full text at `LICENSE` | Yes | **yes** on licence | Wrong tool for phase 5; hand to Phase H |
| 31 | **Penpot** | MPL-2.0 | **MPL-2.0**, full Mozilla text at `LICENSE` | Yes | **conditional** | File-level copyleft: unmodified use is fine with notice + source pointer; **any modified MPL file must be published**. Not recommended for other reasons |
| 32 | **satori** | MPL-2.0 | **MPL-2.0** at `LICENSE` and `satori@0.33.5` `"license":"MPL-2.0"` | Yes | **conditional** | Same MPL obligation. Usable **unmodified** for HTML→SVG thumbnails; a fork of any satori file becomes publishable source |
| 33 | **Webstudio** | AGPL-3.0 | **AGPL-3.0** — `LICENSE` is the *"GNU AFFERO GENERAL PUBLIC LICENSE Version 3, 19 November 2007"* text (licence file only; **no source read**) | Yes | **no** | §13 fires under we-host. Never open the source |

### Supporting packages the report lists (all verified 2026-09-28)

| Package | Claimed | Verified | Usable | Note |
|---|---|---|---|---|
| `@modelcontextprotocol/sdk` (TS) | MIT | **MIT**, `1.30.1` | **yes** | Asymmetry with the Go SDK is real: TS is plain MIT, Go is `Apache-2.0 AND MIT` |
| `flatbush` | ISC | **ISC**, `4.6.2`, dep `flatqueue` | **yes** | ISC notice |
| `react-zoom-pan-pinch` | MIT | **MIT**, `4.2.0`, no runtime deps | **yes** | The no-badge alternative to React Flow (D-7) |
| `linkedom` | ISC | **ISC**, `0.18.13` | **conditional** | Peer dep `canvas >= 2` → Cairo/LGPL. Never install the peer |
| `happy-dom` | MIT | **MIT**, `20.14.5` | **yes** | MIT notice |
| `html2canvas` | MIT | **MIT**, `1.4.1` | **yes** | Unmaintained since 1.4.1 |
| `modern-screenshot` | MIT (version unreadable) | **MIT**, **`4.7.0`** | **yes** | Report's open item closed |
| `zustand` / `classcat` / `d3-zoom` / `d3-drag` (via React Flow) | — | **MIT**, **MIT**, **ISC**, **ISC** | **yes** | React Flow's whole runtime tree is permissive |
| `yrs` (y-crdt, the cgo fallback) | MIT | **MIT**, `yrs@0.28.0` `license = "MIT"` | **yes** | MIT notice |
| `elkjs` (arrives via mermaid 12) | not assessed | **`EPL-2.0 OR GPL-3.0-or-later`**, `0.12.0` | **no** | **Gate by name.** See D-6 |
| `dompurify` (via mermaid) | not assessed | **`(MPL-2.0 OR Apache-2.0)`**, `3.4.16` | **conditional** | Elect Apache-2.0 and record the election |
| `mermaid` (via Excalidraw) | not assessed | **MIT** at `11.12.1` (no elkjs) and at `12.0.0` (**with** elkjs) | **conditional** | Pin below 12 until elkjs is resolved |
| `@tiptap/starter-kit` (via `@hocuspocus/transformer`) | not assessed | **MIT**, `3.31.3` | **yes** | Avoid the transformer package anyway |

---

## 4. UNVERIFIED — what I could not close, and what blocked me

Stated plainly, because an auditor who guesses is worse than no auditor.

1. **Excalidraw's bundled font licences.** The directories exist (Assistant, Cascadia, ComicShanns,
   Emoji, Excalifont, Helvetica, Liberation, Lilita, Nunito, Virgil, Xiaolai) but **nine probed
   licence paths all returned HTTP 404** and the GitHub contents API returned **HTTP 403**
   (unauthenticated rate limit), so I could not enumerate the files. **UNVERIFIED.** Blocking for
   "embed Excalidraw whole", not for "read its API".
2. **Firefox's licence was inferred from a pointer, not read inline.**
   `mozilla/gecko-dev/LICENSE` says only *"Please see the file toolkit/content/license.html for the
   copyright licensing conditions attached to this codebase"* — it does not name MPL-2.0 in the
   file I read. Chromium's `LICENSE` **is** the BSD-3 text (*"Copyright 2015 The Chromium
   Authors"*) and WebKit's `Source/JavaScriptCore/COPYING.LIB` **is** the LGPL text, both read
   2026-09-28. So D-5's conclusion stands on WebKit alone even if Firefox were mislabelled.
3. **No git-history walk anywhere.** The task asks for relicence boundaries; I found two by
   metadata (the MCP Go SDK's in-progress MIT→Apache-2.0 transition, and mermaid 11→12 introducing
   `elkjs`) but I did not diff any `LICENSE` file's history, because `api.github.com` was
   rate-limited (403) throughout. A project that relicensed *silently in the past* would not have
   been caught by this audit.
4. **Transitive trees walked selectively, not exhaustively.** Verified in full: Yjs/lib0,
   Moveable/Selecto (5 packages), rbush→quickselect, React Flow→system→d3/zustand/classcat,
   Hocuspocus (4 packages), Excalidraw→mermaid-to-excalidraw→mermaid→{dompurify, elkjs}, ygo's
   `go.mod`. **Not verified individually**: Rough.js's four helper packages, the CodeMirror 6 and
   `@lezer/*` packages inside Excalidraw, `@braintree/sanitize-url`, the remaining ~15 mermaid
   deps, GrapesJS's Backbone/CodeMirror-5 tree, Onlook's tree, `flatqueue`, `crossws`→`srvx`.
5. **`curl` to `reactflow.dev` failed with code `000`** through the corporate proxy; I read that
   page with the `WebFetch` tool instead, which succeeded. Everything else was fetched by `curl`
   directly. `raw.githubusercontent.com` and `registry.npmjs.org` were never blocked;
   `api.github.com` answered but rate-limited.
6. **Nothing was installed, built or scanned.** No SBOM was produced. Every claim here is
   metadata-and-licence-text; none is "we ran `syft` and it came back clean".

---

## 5. The gate, corrected

The report's gate is sound in shape. Five amendments, each earned by something above.

1. **Deny by name — the exact published names**: `@y/hub`, `y-redis`, `liveblocks` (unscoped CLI),
   `@liveblocks/server`, `tldraw`, `@tldraw/*`, **`elkjs`**. Repo paths too: `yjs/y-redis`,
   `yjs/yhub`.
2. **Add `EPL-1.0`/`EPL-2.0` to the list the gate must not silently allow**, and treat any
   `X OR GPL-*` dual as a hard fail even though the other half is permissive — because the report's
   allow list has no EPL row and `elkjs` would sail through a gate that only knows AGPL/GPL/LGPL.
3. **Dual licences must record an election, not just pass.** `(MPL-2.0 OR Apache-2.0)` for
   `dompurify` is fine *because we elect Apache-2.0*; the SBOM must say which half we took.
4. **Allow-list both Yjs namespaces separately** (`yjs`, `y-protocols`, `y-websocket` **and**
   `@y/y`, `@y/protocols`, `@y/websocket`) and pin the 13.x line — otherwise a gate written from
   this survey is a gate written for a release candidate.
5. **The gate must cover artifacts npm cannot see**: browser binaries fetched at image build
   (Playwright — assert `firefox`, `webkit`, `ffmpeg` absent), font binaries, Go modules and Rust
   crates. Three of the findings in this audit (D-5, D-6.3, and ygo's `go.mod`) are invisible to a
   lockfile-only scanner.

---

## 6. Bottom line

**Nothing on the adopt list is forbidden.** The report's licence reading is right in substance on
every candidate; where it is wrong it is wrong about *which package*, *which version*, or *what
else comes with it* — and those three are how an AGPL component got adopted here once before.

Three items change before a phase 5 plan can cite this: **Playwright installs three non-Apache
browsers by default (D-5)**, **Excalidraw's tree acquires a GPL-dual package the day mermaid goes
to 12 (D-6)**, and **the MCP Go SDK is `Apache-2.0 AND MIT`, not `Apache-2.0` (D-2)**. One needs an
owner, not a lawyer: **the React Flow attribution badge (D-7)**.






