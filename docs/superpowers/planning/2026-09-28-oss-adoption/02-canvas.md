# Phase 5 — the design canvas: what we can take from open source

**Research date: 2026-09-28.** Read-only survey. No code was changed, no test run, nothing started.
Scope: phase 5 only — infinite canvas, real-HTML frames, MCP editing, multiplayer. 31 planned
tasks, 4 approved.

**Clean-room discipline held.** `~/Projects/Paperdesign`, `kgoedecke/doop` and every AGPL/GPL canvas
named in this document were **not opened, not cloned, not read**. Where a copyleft project appears
below it appears by name, licence and one line of description only, taken from its public README or
licence file — which is the record that we looked and why it is excluded.

---

## 1. The short answer

**Adopt six things, vendor two, study three, reject the rest.** Open source can carry roughly the
multiplayer layer, the gesture/transform layer, the spatial index, the sketch primitives and the
screenshot path of phase 5. It cannot carry the thing that makes phase 5 *phase 5*: the real-HTML
frame model, the MCP tool surface over it, and the provenance binding to a brief. Those are ours.

Honest framing of the saving: **of the 31 planned tasks, adoption plausibly removes or shrinks
about 8–11 of them** — the multiplayer/CRDT cluster, transform handles and snapping, hit-testing at
scale, freehand/sketch mode, and frame-to-PNG for the handoff bundle. It does **not** shrink the
canvas binding, the brief chip's six states, the brief mirror and drift check, the approval and
design link, the handoff bundle, or the failure states. Those are the majority of the brief
(`scope-and-risk-phase-5-design.md` §1.1, I1–I8) and they are all provenance work.

The one genuinely surprising find is **GrapesJS (BSD-3-Clause)**: a mature visual builder whose
document model *is* real HTML with a structured, scriptable component tree. It is the closest thing
in permissive open source to "real-HTML frames an agent can edit through an API". It is also a
2015-era Backbone-lineage codebase with its own UI, so the recommendation is **study-only, with a
narrow vendor option** — take the idea of the HTML-backed component tree and the shape of its
Component API, not the editor.

---

## 2. The licence findings that change decisions

Four results here are worth reading before the table, because each one either opens or closes a
path.

**1. Excalidraw really is MIT — the editor package and the repo.** Verified at source:
`excalidraw/excalidraw` root `LICENSE` is the MIT text, `Copyright (c) 2020 Excalidraw`
(https://raw.githubusercontent.com/excalidraw/excalidraw/master/LICENSE, read 2026-09-28), and
`packages/excalidraw/package.json` publishes `@excalidraw/excalidraw` **0.18.0** with
`"license": "MIT"`
(https://raw.githubusercontent.com/excalidraw/excalidraw/master/packages/excalidraw/package.json,
read 2026-09-28). The subtlety the task warned about is real but harmless: the hosted app folder
`excalidraw-app/package.json` is `"private": true` and carries **no `license` field at all**, so it
falls under the repo's MIT `LICENSE` by default — and it is the *app*, which we would not consume
anyway. **Excalidraw Plus is a separate closed commercial product; nothing in this repo is it.**
OD-11 §3's "Excalidraw (MIT) is allowed for a sketch mode" stands, verified.

**2. tldraw's exclusion is correctly stated, and stronger than "commercial licence".** Its
`LICENSE.md` obliges you *"Not to use the Software in Production Environments"* and *"Not to
disable, change, or interfere with the Software's License Key enforcement"*, and states the
software *"includes technical measures to verify License Key validity, detect deployment
environments, enforce usage restrictions … and ensure proper watermark display"*
(https://raw.githubusercontent.com/tldraw/tldraw/main/LICENSE.md, read 2026-09-28). That is
telemetry and an enforcement kill-switch inside a product we would ship to regulated customers on
their own cloud. **Reject, and not only on licence grounds.**

**3. The Yjs trap is real, and it is exactly where the task said to look.** Yjs itself is MIT
(`package.json` 14.0.0-rc.26, one runtime dependency, `lib0`, also MIT — both read 2026-09-28).
`y-websocket` is MIT. `y-protocols` is MIT. **But `yjs/y-redis` — the scale-out backend most
production Yjs deployments reach for — has been renamed "y/hub" and is dual-licensed AGPL or
proprietary.** Its README, read 2026-09-28: *"y/hub is dual-licensed (either AGPL or proprietary).
Please contact me to buy a license if you intend to use y/hub in your commercial product … Otherwise,
you may use this software under the terms of the AGPL"*. **Forbidden.** If we adopt Yjs, the
horizontal-scale story must be `y-sweet` (MIT) or our own Go persistence — not y-redis. This is a
CI licence-gate rule, not a note: the gate must fail on `y-redis`/`y/hub` by name.

**4. Every "open-source Figma/Webflow" is copyleft, without exception found.** Penpot is MPL-2.0
(conditional, see §6). Webstudio — the closest thing to "real HTML frames, visually edited" — is
**AGPL-3.0**, verified from its `LICENSE` file on 2026-09-28; source not read, per the clean-room
rule. Plus the two the owner already excluded. **There is no permissive Figma clone to adopt.** The
canvas chrome is ours to write; open source supplies the parts underneath it.

---

## 3. What open source can carry, and what it cannot

Phase 5 decomposes into six layers. Open source covers four of them well, one partially, and one
not at all.

| Layer | Can open source carry it? | Why |
|---|---|---|
| **Multiplayer: CRDT document, sync transport, presence/awareness** | **Yes, almost entirely** | Yjs + `y-protocols/awareness` + a Go or Node relay is a solved, battle-tested problem. This is the single largest genuine saving |
| **Direct manipulation: transform handles, snapping, alignment guides, group select, gestures** | **Yes** | Moveable + Selecto (MIT) do resize/rotate/warp/snap/guides/group-select out of the box. This is weeks of fiddly work |
| **Spatial index / hit-testing at scale** | **Yes, trivially** | `rbush` (MIT) is 250 lines of R-tree we should not write |
| **Sketch primitives (freehand, hand-drawn look), and a whole sketch mode** | **Yes** | perfect-freehand + Rough.js; and Excalidraw entire, embedded, for OD-11 §3's sketch mode |
| **Real-HTML frames an agent edits through a structured API** | **Partly — as an idea, not a dependency** | GrapesJS proves the shape (HTML-backed component tree + mutation API). Craft.js and Puck prove the JSON-tree shape. None of them is an infinite canvas, and each brings its own editor UI |
| **The provenance spine: canvas↔branch binding, brief chip's six states, brief mirror + drift check, approval writing the design link, handoff bundle, failure states** | **No. Nothing exists** | This is `scope-and-risk-phase-5-design.md` I1–I8 and it is the reason phase 5 exists. Every candidate below is neutral to it — which is the bar CLAUDE.md sets: a component that cannot carry a provenance reference is a poor fit, and these carry it because we wrap them, not because they know about it |

### The architectural fork adoption forces you to make

**Every mature "canvas engine" — Konva, Fabric, PixiJS — renders to `<canvas>`. Real HTML frames
render to the DOM. You cannot have both in the same node.** This is the one place where adopting a
library would force an architecture we have not locked, and it deserves a deliberate decision
rather than a default.

- **DOM-first** (what paper.design does, and what "real-HTML frames" means): frames are real DOM
  subtrees — or sandboxed iframes — inside a CSS-transformed world layer. Zoom and pan are one
  `transform: translate() scale()` on the world. **No canvas engine is used at all.** Konva, Fabric
  and PixiJS become irrelevant; Moveable, Selecto, use-gesture, rbush, Yjs all still apply, because
  they are DOM- or data-level, not renderer-level.
- **Canvas-first**: you get cheap 10,000-element scenes, and you lose real HTML. Frames become
  screenshots, or `foreignObject` hacks, and agent-authored HTML stops being live.

**Recommendation: DOM-first, and therefore do not adopt a canvas engine at all.** That is the only
choice consistent with "real-HTML frames" in OD-11 and with the MCP requirement, because an agent
editing a DOM-backed frame edits HTML it can also read back. It also means three of the most
famous names on the shortlist — Konva, Fabric, PixiJS — are **reject-for-this-purpose**, not
because they are bad but because they answer a different question. Adopting one of them would be
the classic mistake of picking the well-known library and then fighting it for a year.

There is one narrow canvas-first exception worth keeping: **an embedded Excalidraw for sketch
mode**, which is a *separate surface* from the frame canvas and does not need to hold HTML.

---

## 4. The candidates

Licences below were read from the project's own `LICENSE` file or package metadata on
**2026-09-28**, at the URL given. Where a licence is asserted from a repository page rather than
the licence text, it is marked *(page, not file)*.

### 4.1 Adopt

| Candidate | Licence (verified) | Maturity evidence | What it does | What it replaces |
|---|---|---|---|---|
| **Yjs** `yjs/yjs` | MIT — `package.json` `"license":"MIT"`, v14.0.0-rc.26, sole runtime dep `lib0` also MIT ([yjs package.json](https://raw.githubusercontent.com/yjs/yjs/main/package.json), [lib0 package.json](https://raw.githubusercontent.com/dmonad/lib0/main/package.json)) | The default CRDT of the industry; continuous releases; v14 line active 2026 | Conflict-free replicated document types (`Y.Map`, `Y.Array`, `Y.Text`, `Y.XmlFragment`) with binary updates | The whole "two agents edit the same frame at once" problem, and undo/redo across users (`Y.UndoManager`) |
| **y-protocols** (`awareness`) | MIT, v1.0.7 (npm metadata, 2026-09-28) | Ships with every Yjs deployment | Presence/awareness: cursors, selections, who-is-here, with automatic timeout | Presence, the *agent awareness* half OD-15 calls the primary surface |
| **y-websocket** | MIT, v4.0.0-rc.4 ([package.json](https://raw.githubusercontent.com/yjs/y-websocket/master/package.json)) | Reference provider, maintained | Websocket sync provider + a runnable relay server | The client transport. **Use the client; write the server in Go** (see risks) |
| **rbush** `mourner/rbush` | MIT, v4.0.1 (npm, 2026-09-28) | Mourner (Mapbox/Leaflet lineage); stable for a decade | R-tree spatial index, bulk-load + search | Hit-testing, marquee selection and viewport culling at frame counts where a linear scan stops being free |
| **Moveable** `daybrush/moveable` | MIT, v0.53.0 (npm, 2026-09-28) | Very active; framework bindings for React/Vue/Svelte/vanilla | Drag, resize, rotate, warp, scale, **snapping to elements and guidelines**, group transform | The transform-handle and snapping/alignment tasks outright. This is the single biggest "do not write this by hand" on the list after Yjs |
| **Selecto** `daybrush/selecto` | MIT, v1.26.3 (npm, 2026-09-28) | Same author, same release cadence, pairs with Moveable | Marquee/drag selection of DOM elements, including nested targets | Multi-select on a DOM-first canvas |
| **perfect-freehand** `steveruizok/perfect-freehand` | MIT — GitHub metadata + npm; 5,738 stars; last push 2026-04-13 | Small, finished, low churn — but **no push in ~5 months**, so treat as *stable, not actively developed* | Pointer input → pressure-sensitive stroke outline polygon | Freehand annotation on the canvas |
| **Rough.js** `rough-stuff/rough` | MIT, v4.6.6 (npm, 2026-09-28) | Long-lived, widely embedded (it is inside Excalidraw) | Hand-drawn/sketchy rendering of primitives | Low-fidelity/wireframe rendering mode, if we want one |
| **@use-gesture/react** `pmndrs/use-gesture` | MIT, v10.3.1 (npm, 2026-09-28) | pmndrs; broadly used | Normalised pointer/wheel/pinch gestures as React hooks | Pan/zoom/pinch input handling, including the trackpad-vs-wheel distinction that is always the bug |
| **MCP Go SDK** `modelcontextprotocol/go-sdk` | **Apache-2.0** — `LICENSE` says code and specifications Apache-2.0, docs CC-BY-4.0, pre-transition contributions MIT; `Copyright (c) 2024-2025 Model Context Protocol a Series of LF Projects, LLC` ([LICENSE](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/LICENSE)) | Official SDK, Linux Foundation governance | Server and client implementation of MCP in Go | The transport, framing and tool-registration half of phase 5's "first-class MCP server". **The tool surface is still ours to design** |

### 4.2 Adopt, but the decision is bigger than the library

| Candidate | Licence (verified) | Maturity | The call |
|---|---|---|---|
| **React Flow / xyflow** `xyflow/xyflow` | MIT — root `LICENSE`, `Copyright (c) 2019-2025 webkid GmbH`, no extra clauses ([LICENSE](https://raw.githubusercontent.com/xyflow/xyflow/main/LICENSE)); GitHub repo metadata `spdx_id: MIT` | 38,512 stars, last push 2026-09-24 (GitHub API, 2026-09-28) | **The most underrated candidate on this list.** React Flow is already a DOM-first infinite canvas: a CSS-transformed pane, pan/zoom/fit-view, box selection, multi-select, `NodeResizer`, snap-to-grid, and **nodes that are arbitrary React components — i.e. real HTML**. That is a large fraction of the canvas viewport, free, under MIT, with no visual language imposed beyond a minimal stylesheet you can override. Cost: its data model is a flat `nodes[]`/`edges[]` graph with `parentId` for nesting; we would use nodes as frames and ignore edges entirely, and we would be carrying graph machinery (handles, connection logic) we do not need. **"React Flow Pro" is a paid subscription for examples and support — it does not gate the library**, which is MIT in the repo we read |
| **Excalidraw** `excalidraw/excalidraw` (`@excalidraw/excalidraw` 0.18.0) | **MIT**, verified at repo `LICENSE` and at `packages/excalidraw/package.json` (both read 2026-09-28) | The best-known permissive canvas; the hosted app is private/unpublished but MIT-covered; Excalidraw Plus is a separate closed product | **Adopt as a whole embedded surface for OD-11 §3's sketch mode, not as the frame canvas.** Its documented programmatic API is real — `updateScene`, `getSceneElements`, `getSceneElementsIncludingDeleted`, `addFiles`, `scrollToContent`, `onChange`, plus an element-skeleton API for creating elements from code — which means **an agent can edit an Excalidraw scene over MCP today**. But the skeleton API's element types are rectangle, ellipse, diamond, text, arrow, line, sticky note and frame ([docs, read 2026-09-28](https://docs.excalidraw.com/docs/@excalidraw/excalidraw/api/excalidraw-element-skeleton)) — **no HTML**. So it gives us the MCP-editable half and not the real-HTML half |
| **Hocuspocus** `ueberdosis/hocuspocus` | MIT — `LICENSE.md`, `Copyright (c) 2023 Tiptap GmbH` ([LICENSE.md](https://raw.githubusercontent.com/ueberdosis/hocuspocus/main/LICENSE.md)) | Maintained by Tiptap; the de-facto production Yjs server | A batteries-included Yjs websocket server with an extension API for auth, persistence and `onChange` hooks — exactly where a server-side MCP handler applying agent edits would sit. **The catch is architectural: it is Node, and our control plane is Go.** That is a sidecar process in the Kata image and a second runtime to patch. Adopt only if §4.5's "who owns the document server" decision goes the Node way |

### 4.3 Study-only — read the approach, write our own

| Candidate | Licence (verified) | What to steal (the idea, not the code) |
|---|---|---|
| **GrapesJS** `GrapesJS/grapesjs` | **BSD-3-Clause** — `packages/core/LICENSE`, `Copyright (c) 2017-current, Artur Arseniev` ([LICENSE](https://raw.githubusercontent.com/GrapesJS/grapesjs/dev/packages/core/LICENSE)); npm `grapesjs@0.23.6` `"license":"BSD-3-Clause"`, published **2026-08-25** (npm registry, read 2026-09-28 — note: the GitHub releases page rendered a year that did not match; npm is the authority here) | **The single most relevant document model in permissive open source.** GrapesJS keeps a Component tree that is *"your Source of Truth"* and renders it into a real DOM inside an iframe, with `editor.getWrapper()`, `editor.addComponents(html)`, `component.toHTML()`, `component.toJSON()`, `component.components()`, `component.setAttributes()`, `component.append()`. **That method list is almost exactly the MCP tool surface phase 5 needs**: read the tree, read a subtree as HTML, mutate attributes, insert at a position. Take the shape. Do not take the editor — it is a 2015-lineage codebase with its own panels, its own Style Manager and its own visual language, and BSD-3's attribution clause plus a full-editor dependency is a lot of surface for a UI we would have to hide anyway |
| **Craft.js** `prevwong/craft.js` (`@craftjs/core` 0.2.12) | MIT — `LICENSE`, `Copyright (c) 2020 Previnash Wong Sze Chuan`; npm confirms MIT | 8.8k stars, **213 open issues**, no major release in a long while — this is a framework for building page editors, not an editor. Its lesson is the serialisable React node tree and the hook API for mutating it. Useful precedent for "the agent edits a tree, the human sees the render". Not a dependency: it would put React component identity into the document model, which is the opposite of real HTML |
| **Puck** `measuredco/puck` (`@measured/puck` 0.20.2) | MIT — `LICENSE`, "Copyright (c) The Puck Contributors."; npm confirms MIT | A cleaner, newer take on the same JSON-tree editor idea. Same reason it is not a dependency. Worth 30 minutes for the way it separates *config* (what components exist) from *data* (what is on the page) — that separation maps onto our "design tokens + frames" split |
| **Onlook** `onlook-dev/onlook` | **Apache-2.0** — `LICENSE.md`, `Copyright 2024 On Off, Inc.`, standard Apache patent-termination clause ([LICENSE.md](https://raw.githubusercontent.com/onlook-dev/onlook/main/LICENSE.md)) | ~26.8k stars. The closest public prior art to what phase 5 wants: a visual editor where an AI agent edits a real running React app, with a Figma-like canvas over the live DOM. **Read it for the approach to mapping DOM nodes back to source** — that is the hard problem in "the agent changed the design, show me where". Not adoptable: it is Next.js + Tailwind specific, it is a whole product, and it carries its own hosted-service assumptions |
| **Loro** `loro-dev/loro` | MIT — `LICENSE`, `Copyright (c) 2023 Loro` ([LICENSE](https://raw.githubusercontent.com/loro-dev/loro/main/LICENSE)) | Rust CRDT, 1.0 released, with a **Movable Tree** and Movable List CRDT. That matters: a design document is a *tree of frames and elements that get re-parented*, and tree-move is the case Yjs handles worst. If reordering and re-parenting under concurrent agent edits turns out to be the real problem, Loro is the better primitive — and its Rust core suits Tauri and a cgo-free Go story less well than Yjs's ecosystem suits everything else. **Keep as the named fallback, not the default** |
| **Automerge / automerge-repo** `automerge/automerge` | MIT — `rust/automerge/Cargo.toml` `license = "MIT"`, v0.12.0, `rust-version 1.90.0`; `automerge-repo` `LICENSE` MIT, `Copyright (c) 2019-2023, Ink & Switch LLC` | The better-documented CRDT with the better Rust story and a real sync/storage layer (`automerge-repo`). Loses to Yjs on one thing only: the size of the provider and editor ecosystem. Study it; choose Yjs unless the Rust side dominates |
| **ygo** `reearth/ygo` | MIT (repo page, read 2026-09-28) | A **pure-Go, no-cgo Yjs implementation** claiming byte-for-byte V1/V2 wire compatibility with `yjs@13.6.30`, with a conformance suite and a websocket server. If it holds up, it removes the Node sidecar entirely and lets the Go control plane *be* the document server — which is exactly where MCP edits want to live. **But: 55 stars.** A 55-star dependency cannot sit on the critical path of a KPMG product. Treat as a 1-day spike with a pinned commit; if it passes our own conformance test, reconsider. `y-crdt/y-crdt` (Rust, MIT, `Copyright (c) 2020 Bartosz Sypytkowski, Kevin Jahns`) via cgo is the safer version of the same idea |
| **y-sweet** `jamsocket/y-sweet` | MIT — `LICENSE`, `Copyright (c) 2023 Drifting in Space Corp.`, with attribution to y-crdt and Yjs ([LICENSE](https://raw.githubusercontent.com/jamsocket/y-sweet/main/LICENSE)) | A Rust Yjs server with S3-compatible persistence and per-document auth tokens. The **licence-clean replacement for `y-redis`** if we ever need horizontal scale. Study now, adopt if scale demands it |

### 4.4 Reject

| Candidate | Licence | Why rejected |
|---|---|---|
| **tldraw** | **Proprietary / source-available.** `LICENSE.md`: *"Not to use the Software in Production Environments"*, *"Not to disable, change, or interfere with the Software's License Key enforcement"*, and the software *"includes technical measures to verify License Key validity, detect deployment environments, enforce usage restrictions … and ensure proper watermark display"* | Already excluded by OD-11 §3. Worth recording that the exclusion is stronger than a fee: **licence-key enforcement and deployment detection is phone-home code inside a product a regulated customer runs on their own cloud.** Even a paid key would need a security conversation |
| **Konva** `konvajs/konva` | MIT — `LICENSE` carries a dual "Original work © 2011-2013 Eric Rowell (KineticJS) / Modified work © 2014-present Anton Lavrenov" header and standard MIT terms, no extra restrictions (GitHub metadata reads `NOASSERTION` **only because of that dual header** — the terms are plain MIT). 14,832 stars, last push 2026-09-23 | **Not on licence — on architecture.** Konva renders to `<canvas>`. Real-HTML frames cannot live in a canvas scene graph. Adopting it would force the canvas-first fork in §3 and lose the thing OD-11 asked for |
| **Fabric.js** `fabricjs/fabric.js` | MIT (GitHub metadata, 31,463 stars, last push 2026-09-27) | Same reason as Konva. Excellent object model, wrong medium |
| **PixiJS** `pixijs/pixijs` | MIT (GitHub metadata, 48,232 stars, last push 2026-09-25) | Same reason, plus WebGL. Right answer for 50,000 particles, wrong answer for 40 HTML frames |
| **Liveblocks** `liveblocks/liveblocks` | **Split licence.** `LICENSE`: *"Most of the code in this repository is licensed under the Apache License 2.0"*, but **`packages/liveblocks-server` and `tools/liveblocks-cli` are AGPL-3.0-or-later** | The textbook trap the licence rule warns about: permissive client, **copyleft server**. The Apache-2.0 client is only useful against their hosted service or their AGPL server. Both ends fail us — SaaS egress for a regulated customer, or AGPL |
| **`yjs/y-redis` (now "y/hub")** | **AGPL-3.0 or proprietary, dual.** README: *"y/hub is dual-licensed (either AGPL or proprietary) … Otherwise, you may use this software under the terms of the AGPL"* | **Add to the CI licence gate by name.** It is the component a team reaches for at the moment Yjs needs to scale, it sits under the `yjs/` org so it looks like part of an MIT project, and it is not |
| **Incantly `canvas`** ("MIT-licensed tldraw alternative") | MIT (repo page) | **0 stars** as read on 2026-09-28. Found by search, checked, discarded: a zero-adoption SDK is not a saving, it is an unpaid maintenance contract |
| **ElectricSQL** `electric-sql/electric` | Apache-2.0 — `LICENSE` ([LICENSE](https://raw.githubusercontent.com/electric-sql/electric/main/LICENSE)) | Licence is fine; it is simply the wrong tool. It syncs Postgres subsets to clients. It does not do concurrent text/tree editing. **Relevant to the Hub, not to phase 5** — pass it to whoever researches Phase H |

### 4.5 The supporting parts — small, boring, adopt them

These are not headline candidates; they are the pieces that quietly become two tasks each if
written by hand. All licences from the npm registry `/latest` document, read 2026-09-28.

| Package | Licence | Version | Use in phase 5 |
|---|---|---|---|
| `@modelcontextprotocol/sdk` (TypeScript) | **MIT** | 1.30.1 | If any part of the MCP surface lives in the canvas's own process rather than the Go control plane. **Note the asymmetry: the TypeScript SDK publishes MIT, the Go SDK's `LICENSE` is Apache-2.0** — both allowed, but the SBOM must say so correctly |
| `flatbush` | **ISC** | 4.6.2 | Static spatial index; faster build than `rbush` when the frame set is rebuilt wholesale rather than mutated. Pick one, not both |
| `react-zoom-pan-pinch` | MIT | 4.2.0 | A ready-made CSS-transform pan/zoom wrapper if we go DOM-first **without** React Flow. Cheap alternative to writing the world-transform layer |
| `linkedom` | **ISC** | 0.18.13 | Server-side DOM for parsing and mutating frame HTML when an MCP tool edits a frame with no browser in the loop. Far lighter than jsdom |
| `happy-dom` | MIT | 20.14.5 | The heavier, more accurate alternative to `linkedom` when CSS or layout matters |
| `html2canvas` | MIT | 1.4.1 | In-browser frame → PNG. Aging (1.4.1 is old) and imperfect on modern CSS |
| `modern-screenshot` | MIT | — (npm metadata's version field was unparseable in our read) | The actively maintained successor for DOM → image, if the PNG must be produced client-side |
| **Playwright** `microsoft/playwright` | **Apache-2.0** — `LICENSE`, *"Portions Copyright (c) Microsoft Corporation. Portions Copyright 2017 Google Inc."* | current | **The right answer for the handoff bundle's PNG per screen (`scope-and-risk-phase-5-design.md` I7).** Server-side, deterministic, already a Kata-compatible headless browser story, and it removes the in-browser screenshot libraries entirely |

---

## 5. The copyleft register — found, named, not read

Recorded so the program can show it looked. **No source from any project in this table was opened.**

| Project | Licence (verified at source unless noted) | What it is | Why excluded |
|---|---|---|---|
| **Webstudio** `webstudio-is/webstudio` | **AGPL-3.0** — `LICENSE` is the GNU Affero General Public License v3 text, read 2026-09-28 | Visual builder whose document *is* real HTML/CSS with a structured tree — functionally the closest public match to phase 5's frame model | AGPL. §13 fires under our we-host model, exactly as it did for Paperdesign (`PROGRAM.md` phase 5) |
| **Penpot** `penpot/penpot` | **MPL-2.0** — `LICENSE` is the Mozilla Public License 2.0 text, read 2026-09-28 | The mature open-source design tool: boards, components, prototyping, a plugin API | **CONDITIONAL, not forbidden** — see §6. Not recommended anyway: Clojure/ClojureScript, its own visual language, its own identity model, and SVG-shape semantics rather than HTML frames |
| **y-redis / "y/hub"** `yjs/y-redis` | **AGPL-3.0 or proprietary (dual)** — README licence section, read 2026-09-28 | Redis-backed scale-out for Yjs | AGPL, and it sits inside the `yjs/` org where it looks safe. Gate it by name |
| **Liveblocks server + CLI** | **AGPL-3.0-or-later** for `packages/liveblocks-server` and `tools/liveblocks-cli`; Apache-2.0 for the rest — `LICENSE`, read 2026-09-28 | Self-hostable realtime collaboration backend | AGPL on the only self-hostable half |
| **`~/Projects/Paperdesign`** | AGPL-3.0-only (already on file, `PROGRAM.md`) | The withdrawn canvas | OD-11. **Never opened in this research** |
| **`kgoedecke/doop`** | AGPL/GPL family (per OD-11) | Upstream of the above | OD-11. **Never opened, never fetched** |
| **tldraw** | Proprietary source-available (not copyleft, but equally excluded) | See §4.4 | OD-11 §3 |

**MPL-2.0 candidates found (CONDITIONAL — allowed with an obligation):** **Penpot** (whole product),
and **`vercel/satori`** — `LICENSE` is the Mozilla Public License 2.0, read 2026-09-28 — which turns
HTML+CSS into SVG and would otherwise be attractive for rendering frame thumbnails without a
browser. See §6 before using either.

---

## 6. MPL-2.0: what the obligation actually is

MPL-2.0 is **file-level** copyleft. If we use an MPL-2.0 library **unmodified, as a dependency**,
we must ship the licence text and say where the source is; our own proprietary files are untouched
and may stay closed. The obligation bites in exactly two places:

1. **If we modify an MPL file, that file's source must be made available** to recipients — including,
   under a distributed Tauri desktop, to the customer. Our patch to their file becomes public; our
   code that merely calls it does not.
2. **Static linking / bundling is fine**, which is the practical difference from LGPL — MPL-2.0 §3.3
   explicitly permits distributing the covered software as part of a Larger Work under our own terms.

**Practical rule for phase 5: MPL-2.0 is allowed as an unmodified dependency, and any fork of an
MPL file must be flagged in the plan and approved.** The CI licence gate should classify MPL-2.0 as
`warn`, not `fail`, with a named allowlist — otherwise it will block `satori` on a day someone needs
it and nobody will remember why.

**LGPL** (none found relevant here) stays **CONDITIONAL-RISK**: dynamic linking only, and a Tauri
desktop that bundles a `.dll` is close enough to the edge that it should not happen without a
written decision.

---

## 7. The three decisions this research forces

Adoption is not free of architecture. Each of these needs an owner ruling before a phase 5 plan can
be written, and each one is *created* by the candidate list, not resolved by it.

**D-OSS-1 — DOM-first or canvas-first?** §3. **Recommendation: DOM-first**, and therefore no canvas
engine. It is the only fork consistent with "real-HTML frames" and with an agent that can read back
what it wrote. Consequence: Konva, Fabric and PixiJS leave the list; React Flow, Moveable, Selecto,
use-gesture, rbush and Yjs stay.

**D-OSS-2 — where does the collaborative document live, and in what runtime?** The MCP server must
*apply* agent edits to the document, which means whatever process hosts MCP must be able to
interpret the CRDT. Three shapes, in order of recommendation:

1. **Go control plane owns the document, using a pinned Go Yjs (`reearth/ygo`, MIT) or `y-crdt` via
   cgo.** One runtime, one binary in the Kata image, MCP sits where the rest of our tools sit.
   **Risk: a 55-star dependency on the critical path.** Gate it behind our own byte-level
   conformance test before committing.
2. **A Node sidecar running Hocuspocus (MIT), with MCP tools proxied from Go.** Lowest technical
   risk, highest operational cost: a second runtime to patch, size and sandbox in every Kata image,
   and a second place for a CVE.
3. **The desktop owns the document; the server only relays opaque updates.** Cheapest, and it breaks
   the moment a *cloud* agent must edit a design while nobody is signed in — which OD-14's central
   team agents require. **Do not pick this one.**

**D-OSS-3 — React Flow as the viewport, or our own?** Adopting React Flow buys the transformed pane,
pan/zoom, box-select, resize handles and node nesting immediately, at the price of carrying a
graph library's data model. **Recommendation: a 2-day spike that renders three real-HTML frames as
React Flow nodes and tries one MCP-shaped mutation against them.** If the frame model survives
contact with `nodes[]`, adopt; if we find ourselves fighting `edges`, write our own world layer on
`react-zoom-pan-pinch` plus Moveable and keep the rest of the list.

---

## 8. What this actually saves, task by task

Mapped against `scope-and-risk-phase-5-design.md` §1.1. Estimates are for an implementer already
inside this codebase.

| Phase 5 work | Open-source coverage | Honest saving |
|---|---|---|
| Multiplayer document + conflict resolution + cross-user undo | **Yjs + y-protocols + a provider** | **High.** Weeks. Writing a CRDT by hand is not a task we should have on the board at all |
| Presence / awareness (cursors, who-is-editing) — OD-15's *"the agents know"* | **`y-protocols/awareness`** | **High.** Days, not weeks, and it is the primitive OD-15 needs |
| Viewport: pan, zoom, fit, box select, multi-select, nesting | **React Flow (MIT)** or `react-zoom-pan-pinch` + our own | **Medium-high**, subject to D-OSS-3 |
| Transform handles, resize, rotate, snapping, alignment guides | **Moveable + Selecto (MIT)** | **High.** This is the classic "two weeks that became six" task |
| Hit-testing, culling, marquee at scale | **rbush / flatbush** | **Medium.** Small but certain |
| Sketch mode (OD-11 §3) | **Embed `@excalidraw/excalidraw` (MIT) whole** | **High** — an entire optional surface for the price of a dependency, with a real programmatic API an agent can drive |
| Freehand / annotation strokes | **perfect-freehand + Rough.js** | **Medium** |
| Frame → PNG for the handoff bundle (I7) | **Playwright (Apache-2.0)** server-side | **Medium.** Also better than a browser-side screenshot, which we would not trust as evidence |
| MCP transport, framing, tool registration | **MCP Go SDK (Apache-2.0)** | **Medium.** The transport is free; **the tool surface is the product and is ours** |
| **Real-HTML frame model** (what a frame is, how it is stored, how it renders sandboxed) | **Nothing adoptable.** GrapesJS shows the shape; Webstudio does it and is AGPL | **Zero.** Write it |
| **The MCP tool surface over frames** (`list_frames`, `read_frame`, `patch_element`, …) | **Nothing.** GrapesJS's Component API is the best template to copy *in design* | **Zero.** Write it, and this is the differentiator |
| **Canvas↔branch binding (I2), brief chip's six states (I3), brief mirror + drift check (I4), design sessions (I5), approval + design link (I6), handoff bundle (I7), failure states (I8)** | **Nothing, by definition** | **Zero.** This is the provenance spine, it is most of the 31 tasks, and it is why the phase exists |

**Net:** open source removes or materially shrinks roughly **8 to 11 of the 31 tasks** and makes two
or three others lower-risk. It does not change the shape of the phase. Anyone who reports that "an
open-source canvas does phase 5" has not read the brief.

---

## 9. What I could not verify, and what the proxy blocked

**Blocked or failed fetches (all retried at least once from another source):**

- `https://raw.githubusercontent.com/onlook-dev/onlook/main/LICENSE` — **HTTP 404** (not a proxy
  block; the file is `LICENSE.md`). Resolved by fetching `LICENSE.md`, which verified Apache-2.0.
- `https://raw.githubusercontent.com/GrapesJS/grapesjs/dev/LICENSE` — returned a pointer to
  `packages/core/LICENSE` rather than licence text. Resolved by fetching that path (BSD-3-Clause).
- `https://raw.githubusercontent.com/xyflow/xyflow/main/packages/react/LICENSE` — **HTTP 404**.
  Resolved from the repository root `LICENSE` (MIT) plus GitHub repo metadata.
- **GitHub REST API rate limit exhausted mid-survey** (unauthenticated, 60/hour; `remaining: 0` at
  the time of the second batch). Star counts and `pushed_at` dates were therefore collected for
  Konva, Fabric, PixiJS, xyflow and perfect-freehand only; every other maturity claim comes from the
  npm registry or the repository page and is labelled as such.
- **No site was blocked by Netskope in this run.** `raw.githubusercontent.com`,
  `api.github.com`, `registry.npmjs.org`, `github.com`, `reactflow.dev`, `docs.excalidraw.com` and
  `grapesjs.com` all resolved. `curl` through the proxy is slow — 20 to 40 seconds per request —
  which is why batches were run in the background.

**Explicitly unverified, and someone must close these before a plan cites them:**

1. **`reearth/ygo`'s wire compatibility claim is the project's own.** 55 stars. Nothing here tested
   it. Do not put it on the critical path on the strength of a README.
2. **Onlook's licence was read at `LICENSE.md`; its dependency tree was not audited.** A 26.8k-star
   Next.js product will pull a wide tree. It is study-only, so this does not matter yet.
3. **GrapesJS's release recency is asserted from npm** (`0.23.6` published 2026-08-25). The GitHub
   releases page rendered a year that contradicted npm; npm's `time` field is the stronger evidence,
   but a human should glance at the repo before we depend on it.
4. **No dependency tree was audited transitively.** `lib0` (Yjs's only runtime dependency) was
   checked and is MIT. Nothing else was walked. **Before adoption, run a real SBOM
   (`cyclonedx`/`syft`) over the candidate set and fail on any non-allowlisted SPDX id.**
5. **`modern-screenshot`'s npm `version` field did not parse** in our read; its `license` field read
   MIT. Re-check before use.
6. **Nothing was installed, built, benchmarked or run.** Every performance and bundle-size claim in
   this document is absent on purpose, because none was measured.
7. **React Flow Pro's exact terms were not read at source** — the claim that it does not gate the
   MIT library rests on the repository `LICENSE` (plain MIT, no extra clauses) and the docs site
   footer. That is good evidence, but if React Flow becomes load-bearing, read xyflow's commercial
   page directly and record it.

---

## 10. Recommended CI licence gate (OD-11 §3 asks for one)

The gate OD-11 already requires should be written from this survey rather than from a generic list.

1. **Allow**: `MIT`, `Apache-2.0`, `BSD-2-Clause`, `BSD-3-Clause`, `ISC`, `0BSD`, `Unlicense`,
   `Zlib`.
2. **Warn, with a named allowlist and a recorded reason**: `MPL-2.0` (see §6), `CC-BY-4.0` (docs
   only — the MCP SDKs carry it).
3. **Fail, hard**: anything in the AGPL, GPL or LGPL families; `SSPL`; `BUSL-1.1`/`BSL`;
   `Elastic-2.0`; `Commons-Clause`; and **`UNKNOWN`/`NOASSERTION`** — with one documented exception
   process, because `konvajs/konva` reads `NOASSERTION` purely from a dual-copyright header on
   otherwise plain MIT text, and a gate that cannot express that will be switched off by the first
   person it blocks.
4. **Deny by name, regardless of what the scanner reports**: `y-redis`, `y/hub`,
   `@liveblocks/server`, `liveblocks` (CLI), `tldraw`, `@tldraw/*`.
5. **Re-run on every lockfile change, not on a schedule.** A relicence arrives in a version bump.

---

## 11. One-line verdicts

- **Adopt now:** Yjs, `y-protocols` (awareness), `y-websocket` client, rbush, Moveable, Selecto,
  perfect-freehand, Rough.js, `@use-gesture/react`, the MCP Go SDK, Playwright, `linkedom`.
- **Adopt after a short spike:** React Flow (D-OSS-3), `@excalidraw/excalidraw` for sketch mode.
- **Adopt only if D-OSS-2 goes the Node way:** Hocuspocus.
- **Study, then write our own:** GrapesJS's Component API shape, Craft.js, Puck, Onlook, Loro,
  Automerge, `reearth/ygo`, y-sweet.
- **Reject:** tldraw, Konva, Fabric, PixiJS, Liveblocks, y-redis/y-hub, Incantly `canvas`,
  ElectricSQL (wrong phase, not wrong licence).
- **Never open:** Paperdesign, doop, Webstudio, and any other AGPL/GPL canvas.




