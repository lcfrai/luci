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

