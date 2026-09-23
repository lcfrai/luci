# Phase 5 — clean-room product specification: a paper.design-class canvas with a first-class MCP

**Status: DRAFT, planning input. Not an implementation plan.** Written 2026-09-23 on
`feat/phase-5-canvas`, based on `3303fd6`.

**Binding rule (`OD-11`, `docs/superpowers/planning/2026-09-23-owner-decisions.md:22`, `:84-100`):**
this document was written from **public product behaviour only** — paper.design's public site, docs,
build log (changelog), blog, compare pages and published MCP tool surface, plus third-party public
reviews. The author **did not open, read, search or copy** `~/Projects/Paperdesign`,
`kgoedecke/doop`, or any AGPL/GPL canvas source. The only in-repo inputs about a canvas were the
two documents `OD-11` rule 1 allows for product ideas (`survey-phase-5-design.md`,
`scope-and-risk-phase-5-design.md`); no code from them is reused, and nothing below is derived from
them unless it says so.

The owner's words (`OD-11`): *"I want it pretty much exactly like paper.design with the MCP, it is
super powerful."*

---

## 0. Read this first

1. **What we are cloning is the behaviour, not the code.** Paper is a closed-source commercial
   product. Every fact below carries a source id (`[S1]`…) from §11. A fact with no id is **our
   design**, not Paper's.
2. **Paper's power is one idea:** the canvas *is* HTML/CSS, so an agent can write it with the
   language it already knows, read it back as JSX, and a human can hand-edit the same nodes. The MCP
   is small (21 tools on the page we read) because the data model needs no translation.
3. **Our v1 is §9.1.** It is deliberately a strict subset: a local MCP server on loopback, an
   HTML/CSS node tree, artboards, the read/write tool surface, screenshots via the cockpit's own
   webview, a working indicator, and a handoff bundle into the worktree. Multiplayer and Figma
   import are v2.
4. **Open questions are §10.** None blocks v1; each has a recommendation we plan around.

## 1. What paper.design is

| Fact | Source |
|---|---|
| Paper describes itself as *"the connected canvas for teams shipping with agents"* | [S1] |
| It is a design tool whose canvas is built on HTML/CSS: *"Paper's HTML/CSS canvas means your design exports as code. nothing gets lost in translation."* | [S1] |
| It markets a *"continuous loop, from Paper to code and back"*, with agents that *"sync design tokens, styles, and components between your codebase and your canvas"* — *"one source of truth, always current"* | [S1] |
| It ships as **Paper Web** (`app.paper.design`) and **Paper Desktop** (native). The MCP server runs only in Paper Desktop: *"opening a file in the app will automatically start the MCP server in the background"* | [S1], [S2] |
| Sibling products on the same site: **Paper Shaders** (`shaders.paper.design`) and **Paper Snapshot** | [S1] |
| Agents it names as MCP clients: Claude Code, Claude Desktop, Cursor, Codex, VS Code Copilot, Antigravity, OpenCode, Zed | [S1], [S2] |
| Its pitch against slop: agents handle *"responsive layouts, style variations, and other repetitive tasks"* while humans make creative decisions | [S1] |
| The site references a **$34M Series A** | [S1] (not independently verified) |

**In one sentence, for our purposes:** Paper is a Figma-class visual editor whose document model is a
tree of real DOM nodes with real CSS, exposed to local coding agents over an MCP server on
loopback, so that design and code are the same material.

### 1.1 Timeline, from the public build log

| When | What shipped | Source |
|---|---|---|
| 2025-02-25 | $4.2M seed round | [S14] |
| 2025-09-09 | **Paper Alpha**, open sign-ups: image generation, shaders, *"real flex layout"*, **Copy as React**, OKLCH colour picker, vectorise | [S5] |
| 2025-09 → 2025-12 | Radius panel, text gradients, video export (MP4, then WebP/AVIF stills), image fills, variable fonts, **absolute position inside flex**, **Tailwind export** from the Copy as React menu (`⌥R` React CSS, `⌥T` Tailwind) | [S5] |
| 2026-01-19 | Filters panel (blur, saturation, …), gap/padding handles on canvas, SVG colour editing | [S5] |
| 2026-03 | **Paper Desktop with the MCP server**; constraints panel; backdrop filters; OpenType features; *"Wrap in flex"*; a `position: relative` code-export fix | [S5] |
| 2026-04 | **Paper Snapshot** (web page → editable layers); teams with Editor/Admin roles; view-only sharing; **MCP layer ordering/reparenting tools**; *"improved agent CSS layout support"* | [S5] |
| 2026-05 | **Figma copy-and-paste** (with SVGs and images); PDF export; agents create/open files and switch pages; **paste HTML with inline styles**; follow a teammate's cursor; cursor chat | [S5] |
| 2026-06 | **Tokens**, added *"via MCP, CSS variables, or canvas designs"*; pen tool and vector editing; folders; **teammate/agent presence**; *"reduce agent animations"* option | [S5] |
| 2026-07-22 | $34M Series A | [S13] |
| 2026-08 | **Comments**; free editors; **desktop tabs — "agents work across tabs"**; tokens created in-app; flex wrapping; Figma slots on paste | [S5] |

The homepage labels comments / free editors / desktop tabs / tokens-in-app a *"July 2026 update"*;
the build log files them under August 2026 [S1] vs [S5]. The dates above follow the build log.

## 2. The canvas model

### 2.1 The document is a tree of HTML nodes with real CSS

| Property | What Paper does | Source |
|---|---|---|
| **Rendering** | *"Real HTML/CSS — web-native, no translation step"*, against Figma's *"proprietary model (WebGL-based canvas)"*. Series A post: *"Paper bet on HTML and CSS as our rendering engine because they form a natural language for agents."* | [S6], [S13] |
| **Hierarchy** | File → **pages** → **artboards** (top-level frames with dimensions) → nodes. Nodes have ID, name (a layer name), type, size, visibility, lock, parent, children, text content | [S2] (`get_basic_info`, `get_node_info`) |
| **Node types seen publicly** | Frame (container), Text, image (as an *image fill* on a node), SVG/vector, shader, video. The MCP distinguishes "Text nodes" (`set_text_content`) and "a node that has an image fill" (`get_fill_image`) | [S2], [S5] |
| **Styles** | Every node carries CSS. Agents read *"computed CSS styles"* and write *"CSS styles"* (`update_styles`). Panels map onto CSS: fills (colours, gradients, images, composable), borders, outlines, shadows, filters, backdrop filters, radius per corner, opacity, blend | [S2], [S5], [S6] |
| **Colour** | *"sRGB and Display P3 simultaneously — mix `oklch()`, `display-p3`, and hex per element"*; OKLCH picker; LCH/LAB paste | [S6], [S5] |
| **Typography** | Local and Google fonts, variable axes, optical size, OpenType features, auto line height, casing/wrapping/truncation. Agents must check a family first (`get_font_family_info`) | [S5], [S2], [S10] |

### 2.2 Layout: flex plus absolute, and nothing else yet

- **Flex is the layout system** (*"real flex layout"* at launch; flex wrapping 2026-08; *"Wrap in
  flex"* for multiple selections; gap and padding handles on the canvas) [S5].
- **Absolute position** is an explicit per-child escape (*"'Ignore flex layout' renamed to 'Absolute
  position'"*), and a **constraints panel** governs children when a parent resizes [S5].
- **CSS Grid is on the roadmap as "Planned"**, *"rendered with real CSS"* [S7].
- The agent rules a third-party MIT bridge publishes for Paper's `write_html` say: *"flex + padding
  + gap (no margin, grid, or tables)"* [S11]. This is **third-party, unverified against Paper
  itself**, but it matches the build log and the roadmap.
- Paper's own advice for design-to-code: *"you'll get the best results if the structure of your
  design is already optimized for code. In Paper, this means using flex layouts and containers"* [S2].

**Why it matters for us:** a restricted, code-shaped CSS subset is what makes the canvas editable by
both a human (panels, handles) and an agent (HTML), and what makes the JSX export clean. It is a
feature, not a limitation. Our v1 adopts the same subset (§9.1).

### 2.3 Components: not shipped

| Item | Status on Paper's roadmap | Source |
|---|---|---|
| *"Use your code components"* — *"No need to maintain two design systems"* | In progress | [S7] |
| *"Components with slots"* — *"a component system reimagined for today's needs, aligned with code concepts"* | Coming soon | [S7] |
| Icon packs, shadcn, component kits, with Base UI | Coming soon | [S7] |
| Figma slots survive a paste | Shipped 2026-08 | [S5] |

**Paper has no component/instance system today.** Reuse is by duplication
(`duplicate_nodes`, `⌥`-drag) and, for agents, a clone element in `write_html` HTML — the bridge's
rules describe `<x-paper-clone node-id="…" style="…" />` [S11], unverified against Paper. The
homepage's *"sync … components between your codebase and your canvas"* [S1] is ahead of the build log.

### 2.4 Tokens

| Fact | Source |
|---|---|
| Tokens *"are CSS variables"* — *"values that make up a design system that you can apply across your Paper canvas and codebase"*; aligned with Tailwind | [S3] |
| Types: colour, radius, spacing, container, breakpoint, font family, font weight, font size, line height, letter spacing | [S3] |
| Created in the Theme tab, **or by an agent** from a frame (*"point the agent at a frame and request token generation"*) or from code (*"create tokens in Paper using your CSS variables in code"*) | [S3] |
| Applied from a token dropdown in each property panel; an edit *"reflects everywhere that token is used"* | [S3] |
| Copy/paste between files and into CSS files; copies do **not** stay linked | [S3] |
| Roadmap: reusable theme classes and *"multiple theme modes, such a 'dark mode' or 'compact'"*; *"based on real CSS, use calc, color-mix, blend modes"* | [S3], [S7] |
| In agent HTML, tokens are referenced as `var(--color-primary)` | [S11], unverified |

### 2.5 Code export

- **Copy as React** since the alpha; **Tailwind** export from the same menu; hotkeys `⌥R` / `⌥T` [S5].
- **`get_jsx`**: *"JSX for a node and its descendants (Tailwind or inline-styles format)"* [S2].
- A third-party comparison: the JSX *"maps naturally to React and Tailwind"* and is *"generally better
  than Figma MCP because there's no proprietary format to translate"* [S12] (opinion, not measured).
- Roadmap: *"Native Tailwind CSS integration — real-time rendering and import/export of idiomatic
  Tailwind code"*, in progress [S7].
- Image/asset export: PNG, JPG, SVG, WebP, AVIF, PDF (single and combined), MP4 video; size and
  scale presets; default @2x [S2], [S5].

## 3. The MCP server

### 3.1 Shape of the server

| Fact | Source |
|---|---|
| Transport **Streamable HTTP** at `http://127.0.0.1:29979/mcp` — loopback, fixed port. Codex's setup names *"Streamable HTTP"* explicitly | [S2] |
| It **runs inside Paper Desktop**, started when a file is opened; Paper Web has no MCP | [S2] |
| **No authentication is documented.** A third-party bridge implements generic MCP OAuth discovery (RFC 8414 / RFC 7591, PKCE) as *"best-effort"*, and filters out a tool named `mcp_auth`; whether Paper's server challenges at all is **unverified** | [S2], [S22], [S24] |
| Calls are **metered per plan**: 100 tool calls/week on Free, 1M/week on Pro. Metering implies the desktop app knows the signed-in account; *how* it attributes calls is not published | [S8] |
| Installed as a one-line `claude mcp add paper --transport http … --scope user`, or as a plugin (`paper-design/agent-plugins`, which ships per-client MCP config for Claude, Codex and Cursor). That repo has **no licence** (GitHub reports `license: null`), so it is read for behaviour only | [S2], [S18] |
| The server carries its own **agent instructions**: a `get_guide` tool with topics `paper-mcp-instructions` (the full rules), `mobile-status-bar` (paste-ready markup) and `figma-import` | [S2], [S10] |
| A **working indicator** marks artboards an agent is editing; `finish_working_on_nodes` clears it, and the published rules say an agent *"MUST"* call it when done. There is a user setting to *"reduce agent animations"*, and *"teammate/agent presence"* shipped 2026-06 | [S2], [S10], [S5] |
| With desktop tabs (2026-08) *"agents work across tabs"*; every node tool takes an explicit **`fileId`**, and **`pageId`** where relevant | [S5], [S20] |
| Paper's own caveats: restart the agent session to pick up config; *"LLMs can hallucinate available tools"*; WSL needs mirrored networking; *"Large, deeply nested designs may cause errors"* | [S2] |

### 3.2 The tool surface

**Two public lists disagree, and the difference is itself a finding.** Paper's docs page lists **21
tools** [S2]. A third-party MIT bridge that discovers tools **at runtime** from the live server
(`client.listTools()`, no hard-coded names) lists **34**, and its live test asserts *more than 33*
[S19], [S24], [S25]. The docs lag the product; the 34 is the better picture of today's surface. A
comparison site counts *"24 tools (11 read, 8 write, 5 utility)"* [S12], a snapshot from between the
two. **Exact input schemas are not published anywhere we could reach**; they are visible only in a
live `tools/list`, and we did not run Paper Desktop (§11.3). Argument names below are the ones the
public sources state; everything else is a description.

| # | Tool | Group | What it does (public wording) | Arguments stated publicly | Source |
|---|---|---|---|---|---|
| 1 | `list_files` | Files | List the user's files | — | [S19] |
| 2 | `open_file` | Files | Open a file (a desktop tab) | `fileId` | [S19], [S11] |
| 3 | `create_file` | Files | Create a file | — | [S19] |
| 4 | `create_page` | Files | Create a page | — | [S19] |
| 5 | `get_basic_info` | Read | *"File name, page name, node count, and list of artboards with dimensions"* | `fileId` | [S2] |
| 6 | `get_selection` | Read | *"Details about the currently selected nodes (IDs, names, types, size, artboard)"* — the human's selection is agent context | `fileId` | [S2] |
| 7 | `get_node_info` | Read | *"Details for a node by ID (size, visibility, lock, parent, children, text content)"* | `fileId`, node id | [S2] |
| 8 | `get_children` | Read | *"Direct children of a node (IDs, names, types, child counts)"* | node id | [S2] |
| 9 | `get_tree_summary` | Read | *"Compact text summary of a node's subtree hierarchy (optional depth limit)"* | node id, optional depth | [S2] |
| 10 | `get_screenshot` | Read | *"Screenshot of a node by ID (base64 image; optional scale 1x or 2x)"* | node id, `scale` ∈ {1, 2} | [S2] |
| 11 | `get_jsx` | Read | *"JSX for a node and its descendants (Tailwind or inline-styles format)"* | node id, format ∈ {tailwind, inline-styles} | [S2] |
| 12 | `get_computed_styles` | Read | *"Computed CSS styles for one or more nodes (batch)"* | node ids (batch) | [S2] |
| 13 | `get_fill_image` | Read | *"Image data from a node that has an image fill (base64 JPEG)"* | node id | [S2] |
| 14 | `find_nodes` | Read | Search for nodes | — | [S19] |
| 15 | `get_font_family_info` | Read | *"Look up whether a font family is available; inspect weights and styles"* | family | [S2] |
| 16 | `get_guide` | Read | *"Retrieve guided workflows for topics (e.g. figma-import)"* | `topic` | [S2], [S10] |
| 17 | `list_comment_threads` | Comments | List comment threads | `fileId` | [S19], [S20] |
| 18 | `get_comment_thread` | Comments | Read one thread | — | [S19] |
| 19 | `list_comment_thread_authors` | Comments | Who commented | — | [S19] |
| 20 | `set_comment_thread_status` | Comments | Resolve / reopen a thread | — | [S19] |
| 21 | `write_html` | Write | *"Parse HTML and add or replace nodes (insert-children or replace mode)"* | `html`, mode ∈ {insert-children, replace}, target node | [S2] |
| 22 | `create_artboard` | Write | *"Create a new artboard; optional name and styles (e.g. width, height)"* | optional `name`, `styles` | [S2] |
| 23 | `set_text_content` | Write | *"Set text content of one or more Text nodes (batch)"* | batch of (node id, text) | [S2] |
| 24 | `rename_nodes` | Write | *"Rename one or more layers (batch)"* | batch | [S2] |
| 25 | `duplicate_nodes` | Write | *"Deep-clone nodes; returns new IDs and a descendant ID map"* | node ids | [S2] |
| 26 | `move_nodes` | Write | *"Reposition or reparent existing nodes while preserving their IDs"* | node ids, target parent | [S2], [S11] |
| 27 | `update_styles` | Write | *"Update CSS styles on one or more nodes"* | node ids, `styles` | [S2] |
| 28 | `delete_nodes` | Write | *"Delete one or more nodes and all their descendants"* | node ids | [S2] |
| 29 | `finish_working_on_nodes` | Write | *"Clear the working indicator from artboards"* | optional node ids | [S2] |
| 30 | `get_tokens` | Tokens | Read the file's tokens | — | [S19] |
| 31 | `create_tokens` | Tokens | Create tokens (from a frame or from code CSS variables, per the tokens doc) | — | [S19], [S3] |
| 32 | `set_tokens` | Tokens | Change token values | — | [S19] |
| 33 | `export` | Export | *"Export nodes as image or video files (PNG, JPG, SVG, MP4, etc.)"* | node ids, format | [S2] |
| 34 | `export_combined_pdf` | Export | Several frames as one PDF | — | [S19], [S5] |

The bridge prefixes every name with `paper_` for its own harness [S19]; the names above strip that.

### 3.3 The HTML dialect agents write

The published agent rules (third-party copy, [S11], unverified against Paper) describe a
**constrained HTML dialect**, not arbitrary web pages:

- **Inline styles only**, with tokens as `var(--…)`.
- **Layout is flex + padding + gap.** No margin, no grid, no tables.
- A `layer-name` attribute names the layer.
- Images are referenced as `paper-asset:///absolute/path` — a local-file scheme the app resolves.
- Icons are SVG or images, never emoji.
- Reuse is a clone element, `<x-paper-clone node-id="…" style="…" />`, which instantiates an
  existing node with overrides.

### 3.4 How agents are told to work

From Paper's docs [S2] and the rules its server serves through `get_guide` (as cached by the MIT
bridge, [S10], [S11], [S23]):

1. **Orient:** `get_basic_info` first, then `get_selection` to see what the human is looking at.
2. **Fonts before type:** *"you MUST call paper_get_font_family_info before your first typographic
   styling"*.
3. **Plan before drawing:** unless the user gives a design system, write a short brief — palette,
   type scale, spacing, direction.
4. **Write in small pieces:** *"each paper_write_html call should add roughly one visual group"*.
   Prefer `duplicate_nodes` + `update_styles` + `set_text_content` over rewriting HTML.
5. **Verify visually:** `get_screenshot` *"after meaningful changes"*; check spacing, typography,
   contrast, alignment, artboard fit, repetition. Let content set height (`height: 'fit-content'`)
   rather than guessing.
6. **Read exact values from data, not pixels:** `get_jsx`, `get_computed_styles`, `get_fill_image`.
7. **Finish:** call `finish_working_on_nodes`. Never show raw node IDs to the user.

The documented agent workflows [S2], [S18]:
- **Code → design:** *"Use the CSS styles from my repo and design a settings page in Paper"* — the
  agent reads the repo's stylesheets and tokens and draws in the codebase's own visual language.
- **Design → code:** *"Implement my design from Paper in my codebase, using my code conventions"* —
  `get_jsx` / `get_computed_styles` / `get_fill_image`, then code in the project's own framework.
- **Tokens:** from Figma (via Figma's MCP) or from code CSS variables into Paper; sticker sheets of
  colours, type and spacing.
- **Real content:** pull copy or data from Notion or an API into frames (*"Fetch live data — Done"* [S7]).
- **Multi-MCP:** Paper with Figma or Notion in one agent session — *"ensure correct files open in both apps"*.

## 4. Multiplayer

What is public is behaviour, not protocol. Paper publishes nothing about its sync engine.

| Behaviour | Source |
|---|---|
| Live multiplayer cursors; *"multiplayer cursor logic"* refined 2026-03 and 2026-06 | [S5] |
| **Follow a teammate's cursor**; **cursor chat** from the right-click menu (2026-05) | [S5] |
| **Teammate and agent presence** — agents appear as present, like people (2026-06) | [S5] |
| Multiplayer selection highlights (a fix for deleting while highlighted, 2025-12) | [S5] |
| **Comments** (2026-08): leave, resolve, search, filter and sort in the right panel. Agents read and resolve them over MCP (`list_comment_threads`, `set_comment_thread_status`) | [S5], [S19], [S20] |
| Roles: Editor and Admin in teams; free viewer roles; view-only sharing outside the team; anonymous visitors; *"Full sharing settings"* in progress | [S5], [S7] |
| *"Improved disconnection handling"*; an *"internet loss warning"* | [S5] |
| Organisations plan: SAML/SSO and admin controls | [S8] |

**For the design of ours** the closest public, detailed reference is Figma's own write-up [S26]
(2019): **server-authoritative, not a true CRDT** — *"Figma isn't using true CRDTs though"* —
**per-property last-writer-wins** as ordered by the server, object ids that embed the client id,
parent links stored on the child with **cycles rejected by the server**, **fractional indexing**
for child order, **multiplayer-aware undo**, and on reconnect a fresh download plus replay of
offline edits. That model fits a Go authority far better than a CRDT library, and needs no
third-party code (§10).

## 5. Import, export, and the handoff to code

### 5.1 Into Paper

| Path | Behaviour | Source |
|---|---|---|
| **Paste HTML** | *"as editable layers"*; HTML with inline styles (2026-05) | [S4], [S5] |
| **Paste from Figma** | objects with SVGs and images (2026-05); Figma slots survive (2026-08). Mechanism unpublished | [S4], [S5] |
| **Figma via Figma's MCP** | the `figma-import` guide; an agent reads Figma's MCP and writes Paper. Paper's listed Figma-MCP quirks: *"SVG fills returned as images, spacer elements ignored, inset borders not converted"* | [S2], [S10] |
| **Paper Snapshot** | *"Convert webpages to editable layers"*; runs from `app.paper.design`, so a local dev server must allow CORS from that origin for images; Vite 6+ blocks `/@fs/` cross-origin regardless | [S5], [S16] |
| **Images, SVG, text** | *"all common content types"*; paste on top `⌘V`, paste to replace `⌘R`; SVG becomes editable layers | [S4], [S9] |
| **From code** | an agent reads the repo's CSS and tokens and draws with them | [S2], [S18] |
| **Live data** | agents pull from APIs, Google Sheets, Notion into frames | [S2], [S7] |

### 5.2 Out of Paper

| Path | Behaviour | Source |
|---|---|---|
| **Copy as React** / **Tailwind** | menu and `⌥R` / `⌥T` | [S5] |
| **`get_jsx`** | Tailwind or inline styles, per node subtree | [S2] |
| **`get_computed_styles`**, **`get_fill_image`** | exact values and image bytes for an agent writing code | [S2] |
| **`export`** | PNG, JPG, SVG, MP4 *"etc."*; WebP, AVIF, PDF per the build log; combined PDF | [S2], [S5] |
| **Tokens** | copy/paste into a CSS file | [S3] |
| **Host assets from Paper** | direct CDN links — in progress | [S7] |

### 5.3 The handoff, as Paper frames it

There is **no handoff artefact**. Paper's claim is that the design *is* the code's material:
*"Not a picture of the change, but the change itself"* [S15]. The handoff is an agent, in the user's
repo, reading the canvas over MCP and writing code in the project's conventions (§3.4). Paper's
forward bet is round-tripping — *"pull a section of your live app onto a canvas, iterate on it
spatially … and push it back as code"* [S15] — plus *"Use your code components"* (in progress) and
CI that *"will create canvases with visual diffs"* [S13].

**For us this is the one place we deliberately go further than Paper:** our product spine needs a
**recorded** handoff — an approved, digested snapshot bound to a brief revision, carried into
Develop and attached to the release (`CLAUDE.md`, "Product spine";
`provenance-contract-draft.md` §3.2). Paper's live-read handoff is still offered; the recorded one
is added (§7.6).

## 6. What makes it powerful, compared with Figma-style tools

1. **No translation layer.** The document is HTML/CSS [S6]. An agent writes it in the language it
   is best at, and reads it back as JSX [S2]. Figma's model is proprietary and its MCP must
   translate; Paper's comparison page says so directly [S6].
2. **Read *and* write, as peers.** Agents create artboards, write HTML, restyle, duplicate, move,
   delete and export [S2]. The human's selection is agent input (`get_selection`), and the agent's
   work is visible to the human as it happens (the working indicator, agent presence) [S2], [S5].
   Figma's MCP now also writes to canvas on its remote server [S27], so the gap here is narrowing.
   The data-model gap is not.
3. **A small, sharp tool surface.** About 34 tools cover orient → read → write → verify → export
   [S19]. Batch variants everywhere. `duplicate_nodes` returns a descendant id map so an agent can
   keep editing the copies without a re-read [S2].
4. **The server teaches the agent.** `get_guide` serves the design-quality rules and the workflows
   (fonts first, one visual group per write, screenshot to verify, finish-working) [S10]. The
   quality floor is in the server, not in every user's prompt.
5. **Code-shaped constraints.** Flex + padding + gap, tokens as CSS variables, absolute as an
   explicit escape [S5], [S11]. What the human draws is already structured like the code will be,
   which is why the JSX comes out clean [S2].
6. **Local and composable.** Loopback MCP inside the desktop app [S2]: the agent has the repo, the
   design and other MCP servers (Figma, Notion) in one context [S15]. No upload of the codebase to
   the design tool.
7. **Modern colour and type** — OKLCH/P3 per element, variable fonts, OpenType features [S5], [S6].
   Our design system is OKLCH already (`docs/DESIGN-SYSTEM.md:33`).
8. **Cheap to iterate.** Duplicate-and-vary is the core loop; *"Right click → Remix"* is planned
   [S7]. Spatial side-by-side comparison is the thing chat cannot do [S15].

**Where Paper is weaker, and where we can win** (our analysis, not Paper's claims):

| Gap in Paper (public evidence) | Why it matters for our users | Our answer |
|---|---|---|
| No component/instance system yet [S7] | Design systems need instances | v2 components with slots (§9.2) |
| No CSS Grid yet [S7] | Dashboards and tables are grid-shaped | v2 grid (§9.2) |
| **Parallel agents are not addressed.** The working indicator is a visual cue; nothing public says two agents are stopped from overwriting each other | Parallel agents are our product | Claims with leases and revision preconditions on every write (§7.4) |
| The MCP needs the desktop app open with the file open [S2] | Our agents run in sandboxes while the user is away (`docs/AGENTIC-UX.md` §4 rule 1) | The document authority lives in the control plane, not the window (§7.2) |
| No recorded handoff, no approval, no provenance | Regulated enterprise needs the thread | Approval with a digest, a design link, a bundle in the worktree (§7.6) |
| Egress by design: image generation vendors, CDN asset hosting, Snapshot from a public origin [S5], [S7], [S16] | Regulated tenants ask about every egress | No server-side egress in v1; assets stay local (§7.5) |

---

## 7. Our canvas — the product specification

Everything in this section is **our design**, informed by §1–§6. Where it cites Paper it says
which behaviour it matches. Where it cites this repo, the file and line were opened for this
document.

### 7.1 Principles

1. **Match Paper's model**: HTML/CSS nodes, flex-first layout, tokens as CSS variables, a small
   read/write MCP surface that teaches the agent, JSX out. This is what the owner asked for.
2. **Beat it on parallel agents**: many sessions on one canvas is the default case for us, so every
   write is attributed, claimable and revision-checked (§7.4).
3. **Keep the thread**: a canvas is bound to a branch's brief, approval is a human act over a
   digest, and the approved design travels into Develop and Deploy (§7.6).
4. **No script ever runs in the canvas.** The dialect has no `<script>`, no event handlers and no
   remote URLs (§7.3). This deletes, by construction, the whole class of risks the earlier survey
   found in a script-executing canvas (`scope-and-risk-phase-5-design.md` R4, R6).
5. **Own package, clean seam** (`OD-11` rule 4): the canvas is `canvas/` in the monorepo — a Go
   module for the document authority and MCP server, and a TypeScript package for the editor —
   mounted by the cockpit's `design` lens arm (`desktop/src/components/AppShell.tsx:36-38`, today a
   `LensPlaceholder`).

### 7.2 Where each part runs

| Part | Runs in | Why |
|---|---|---|
| **Document authority** — the node tree, ops, revisions, claims, tokens, comments, sanitiser, JSX/HTML serialisers, the MCP server | **The control plane** (Go), a new `canvas` package, persisting under the control plane's state directory (`PANACEA_APP_STATE_DIR`, `control-plane/internal/statepath/statepath.go:13`) | Agents run in sandboxes while the user is away. Paper's MCP dies when its window closes [S2]; ours must not. Loopback only, as today (`config.go:54`, `127.0.0.1:8730`) |
| **Renderer** — real DOM for layout, computed styles and screenshots | **The desktop webview**: the Design lens when open, otherwise a hidden renderer the desktop keeps per open project | Layout needs a browser engine; Go has none. Tools that need layout say so with a typed error when no renderer is attached (§8.4) |
| **Editor UI** — selection, handles, panels, layers, tokens, comments | **The desktop**, `canvas/editor` mounted in the Design lens | The cockpit is the one window (`CLAUDE.md`) |
| **Approvals, design links, approved snapshots** | **The Hub** (`OD-3`, `OD-5`) | Shared from day one: a teammate sees the same approval and the same thread |
| **Live shared documents across people** | The Hub, from v2 | The same Go package, hosted server-side; v1 is single-person (§9) |

**Transport inside the window.** The desktop polls today, *"There is no event stream in the Go
contract … so Phase 2 polls"* (`desktop/src/api/poller.ts:1-3`). A canvas that shows agents'
edits live is push-shaped. **Recommendation (OQ-3):** the canvas's op channel is the first topic
on the event stream the poller comment asks for — one WebSocket per window, multiplexed — not a
second, private transport.

### 7.3 The document model and the HTML dialect

**Entities** (field names are the plan's to finalise):

- **Canvas** — one per branch, `(projectId, branchId) → canvasId`, created on first open. Carries
  `revision` (monotonic, bumped by every op), the bound brief key, and pages.
- **Page** — ordered list of artboards.
- **Node** — `id`, `kind` ∈ {`artboard`, `frame`, `text`, `image`, `svg`}, `name` (layer name),
  `parentId`, `order` (a fractional index, [S26]), `styles` (CSS property → value, from the
  allowlist), `text` for text nodes, `asset` for images, `locked`, `hidden`, `revision`,
  `lastEditedBy` (user + session handle).
- **Token** — `name`, `type` (Paper's ten types, [S3]), `value`; serialised as CSS custom
  properties and as a W3C Design Tokens file [S28].
- **Asset** — content-addressed bytes (`sha256`), referenced from nodes as `asset:<sha256>`.
- **Comment thread** — anchored to a node, with messages, author (a human, or a session), status.
- **Claim** — `(sessionId, artboardId, expiresAt)`, see §7.4.
- **Op** — every mutation, with author, time and the revision it produced. The log is the undo
  stack, the activity feed and the audit trail.

**The v1 dialect** — what `write_html`, paste and the editor accept (Paper's rules [S11] adopted,
then tightened for security):

| Allowed | Rejected, with a report naming what was dropped |
|---|---|
| `div`, `span`, `p`, `h1`–`h6`, `img`, `svg` (sanitised subset), `button`/`input`/`a` as **inert** visual elements, `x-clone` for reuse | `script`, `style` blocks, `iframe`, `object`, `embed`, `form` actions, `link`, `meta`, `base`, any `on*` attribute, `javascript:` anything |
| **Inline styles only.** Layout by `display: flex` (direction, wrap, gap, align, justify), padding, `position: absolute` with insets, width/height incl. `fit-content`, min/max | `margin` (v1, as Paper [S11]); `display: grid` (v1; v2 adds it); `position: fixed`/`sticky`; `@import`; `url()` to anything but `asset:` and small `data:` images |
| Colour in hex, `rgb()`, `oklch()`, `display-p3` [S6]; `var(--token)` | `expression()`, `behavior`, `-moz-binding` and every legacy script vector |
| Fills, gradients, borders, outlines, radius per corner, shadows, opacity, filters, backdrop filters, blend modes, typography incl. variable axes and OpenType features [S5] | Remote fonts; fonts are local or bundled |
| A `name` attribute for the layer name (Paper's `layer-name` [S11]) | — |

A write never fails silently: the response lists every dropped element, attribute and property,
so the agent can correct itself. Sanitising runs **in Go, in the authority**, before the op is
accepted, and again in the renderer as defence in depth (§10).

### 7.4 Many agents on one canvas

Paper shows a working indicator [S2]; nothing public says two agents are kept apart. We need more:

1. **Attribution is free and unforgeable.** Every MCP call arrives over a sandbox's own relay
   (§7.7), so the authority knows the session without trusting anything the agent says. Every op
   records user + session handle, and the cockpit row and the canvas presence show the same name.
2. **Claims.** The first write into an artboard claims it for that session (Paper's working
   indicator, promoted to a lease). `finish_working` releases it; it also lapses after an idle
   period. Another agent's write into a claimed artboard fails with `claimed` naming the holder,
   unless it asks for a shared claim. **Humans are never blocked** — a human edit in a claimed
   artboard succeeds and is shown to the claiming agent on its next read. This is the canvas twin
   of phase 3's file claims (`PROGRAM.md` phase 3, "file-claim coordination"); one vocabulary for
   both is an open question (OQ-6).
3. **Revision preconditions.** Every write accepts `ifRevision` for the nodes it touches. A stale
   write fails with `conflict` and the current revision, instead of silently overwriting. Without
   it, per-property last-writer-wins [S26] applies — that is the right default for humans dragging
   things, not for agents rewriting a subtree.
4. **Per-author undo.** A human's undo undoes their own ops, never an agent's; an agent's work is
   reverted by name from the activity feed (*"revert everything session `hero-v2` did"*). This is
   `docs/AGENTIC-UX.md` §4 rule 6 — reversible with a rewind, not a manual unpick.
5. **Agents signal, humans decide.** An agent can `request_review` (§8.2); it cannot approve.

### 7.5 Security and egress

- **No script execution** anywhere in the canvas (§7.3).
- **Each artboard renders in its own sandboxed iframe with `allow-same-origin` and without
  `allow-scripts`.** The editor, in the parent, can read layout and handle input inside it; nothing
  inside can run. A per-artboard viewport also gives real media-query behaviour at the artboard's
  width. **Unverified in WebView2** — the first spike (§9.1, item 1).
- **A strict CSP inside each artboard document**: no network at all; images and fonts only from
  the asset scheme and `data:`.
- **The canvas origin gets no Tauri IPC.** Today the app's CSP is `null`
  (`desktop/src-tauri/tauri.conf.json:21`); the canvas must not widen what that exposes.
- **No server-side egress in v1**: no image generation, no stock search, no web snapshot, no CDN.
  Content from outside arrives through the user's own agents and MCP servers, under their own
  identity — the capacity model `CLAUDE.md` locks.
- **Scope**: a session sees only its own branch's canvas. There is no `list_files` across projects.

### 7.6 Provenance: binding, brief, approval, handoff

The provenance contract draft was written against the withdrawn Northstar canvas. Its **shapes**
stand; its **mechanisms** get simpler because the canvas is now ours.

| Concern | Contract draft said | Clean-room canvas does |
|---|---|---|
| Binding | `(projectId, branchId) → canvasId`, new state in our store | Same. It is a field of the Canvas entity, not a side table |
| Brief visible to agents | A **brief mirror** written into Northstar guidelines, with a marker and a drift check (§3.3) | **No mirror.** An MCP tool, `get_brief`, resolves the bound branch's brief through the phase 4 resolver at call time and returns its key and revision. Nothing is copied, so there is nothing to drift. Contract §3.2's *"never holds a brief copy in the canvas database"* holds by construction |
| Who approves | A human, at Northstar's `VISION` gate | A human, from our chrome. **No MCP tool approves** |
| Digest | Northstar's SHA-256 over frames, guidelines, pinned references | Ours: SHA-256 over the canonical serialisation of the approved artboards, their assets' hashes, the tokens they use, and the brief key reviewed against |
| Design link | `design:<canvasId>@<designRevision>` + `reviewDigest` → brief key (`provenance-contract-draft.md:265`) | **Same shape**, written to the Hub ledger in the same operation as the approval |
| Changed after approval | Needed Northstar's activity feed to name the editor (unverified) | Recomputed digest ≠ approved digest; the op log names every op since, with its author |
| Deleted canvas orphans links (brief R8 / P-g) | Unowned | **The approved snapshot is captured into the Hub at approval.** A handoff works while the canvas is unreachable, and a release's design key always resolves |
| Brief edited after approval | Open question 3; recommended *pin and mark* | Unchanged: the approval stands as `superseded`; one explicit re-bind. With no mirror, a brief edit cannot void an approval by accident |

**The handoff bundle**, materialised into the worktree on request, with a marker carrying the design
key and the brief key:

- `design/<artboard>.html` — the canonical dialect, the exact approved revision;
- `design/<artboard>.png` — rendered at 2× (Paper's default [S5]);
- `design/<artboard>.jsx` — React with Tailwind or inline styles (Paper's `get_jsx` formats [S2]);
- `design/tokens.css` (custom properties) and `design/tokens.json` (W3C DTCG [S28]);
- `design/manifest.json` — design key, brief key, digest, artboard list, approver, time.

And Paper's live path, kept: a Develop session's agent can call the read tools against the
**approved revision** (`revision` argument, §8.1) to pull exact values while it writes code.
*"Start a Develop session from this design"* passes the brief key as the session's `contextRef` and
writes a `session → design key` link (contract §3.2). Whether the bundle is **committed** follows
phase 3's answer for `CLAUDE.md` (contract open question 4).

### 7.7 How an agent in a sandbox reaches the MCP server

Paper's agents run on the same machine as the app and dial `127.0.0.1:29979` [S2]. Ours run inside
a sandbox (Docker now, Kata later), and the control plane listens on loopback only. What exists
today: the control plane **dials into** `sandboxd` in the sandbox, which serves `/attach` and
`/healthz` (`sandboxd/internal/server/server.go:85-86`), and relays payload-blind
(`control-plane/internal/proxy/proxy.go:1-2`; its hooks see sizes only, `:12-15`).

**Recommended design (OQ-4):**

1. `sandboxd` gains a **loopback MCP listener inside the sandbox**, Streamable HTTP, like Paper's.
2. It forwards each request over a channel **the control plane opens** into `sandboxd` — the same
   direction as `/attach`. Nothing in the sandbox can reach the host; the host reaches in.
3. The control plane stamps the session id on each forwarded call. The agent needs **no token**:
   the channel is the credential. This removes, rather than solves, the *"headless agents cannot
   complete MCP OAuth"* risk the earlier brief carried (R7).
4. The agent is pointed at the listener through the harness's **documented** MCP configuration.
   For Claude Code that is a launch flag, next to the flags the control plane already passes
   (`--model`, `--effort`, `--permission-mode`, `--resume`, `--session-id`:
   `control-plane/internal/harness/argv.go:83-118`). The CLI is not modified, wrapped or parsed
   (rule 6). There is no MCP wiring in the harness package today (checked).

This is `sandboxd` work, and `sandboxd` MCP forwarding is a relay of opaque bytes. The relay's
**hooks see sizes, not payloads**, as the existing proxy's do.

### 7.8 The Design lens

The cockpit chrome around the canvas follows `docs/DESIGN-SYSTEM.md` and the six-state brief chip
(`provenance-contract-draft.md` §4.1). The prototype `prototype/phase-5-design.html` drew that
chrome around an embedded Northstar viewport; under `OD-11` the viewport is ours, so its seven
states stand and the viewport gains our editor. What the editor carries, modelled on Paper [S5]:

- **Canvas**: infinite pan/zoom surface of artboards; flat `--canvas` surface, no dot grid (the
  phase 2 audit failed that construct — brief R13).
- **Left**: pages; layer tree (drag to reorder/reparent, lock, hide, rename); Theme tab for tokens.
- **Right**: layout (flex controls, absolute toggle, constraints, size incl. fit-content), fill,
  border, radius, shadow, filters, typography; a token picker in every field [S3].
- **On canvas**: select, marquee, move, resize, rotate, gap/padding handles [S5], snapping and
  smart guides, text editing in place, `⌥`-drag duplicate.
- **Agents**: presence per session (name, working artboards, claim state) on **the one heartbeat**
  (`desktop/src/hooks/heartbeat.ts`); a *"reduce agent animations"* setting, as Paper has [S5].
- **Comments**: pinned to nodes; a human writes, an agent reads and resolves over MCP.
- **Code panel**: selected node as HTML, JSX-inline or JSX-Tailwind (Paper's `⌥R`/`⌥T` [S5]).
- **Keyboard**: the canvas's own tools inside the viewport, the cockpit's chords outside. The
  iframes are same-origin, so the cockpit can listen in them — the dead-keyboard seam the earlier
  brief found (S2) does not arise. `OD-9` says the cockpit wins by default; the plan settles which
  canvas keys (`V`, `F`, `T`, `H`, `Z`, …) sit under that rule.

