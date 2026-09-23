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

