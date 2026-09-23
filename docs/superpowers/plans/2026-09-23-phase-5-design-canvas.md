# Phase 5 — the Design canvas: implementation plan

**Status: DRAFT, written 2026-09-23 on `feat/phase-5-canvas` (based on `3303fd6`).** Built under
`OD-1` (the owner waived the prototype-before-plan gate for this phase); still adversarially
reviewed before any code, and every UI branch stays unmerged until the owner has looked at it.
**Nothing in this plan was compiled or run.**

**Primary input:** `docs/superpowers/planning/phase-5/paper-design-cleanroom-spec.md` ("the spec"
below; 861 lines, read in full). **Binding:** `OD-11`'s clean-room rules
(`docs/superpowers/planning/2026-09-23-owner-decisions.md:84-100`). The author of this plan did not
open, read, search or copy `~/Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas
source, and no search in this plan's making touched `~/Projects/Paperdesign`.

**Builds against the Hub**, designed tonight in a sibling worktree and read there by absolute path,
never written to:
- `.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md` ("ADR-H1"), 776 lines,
  read in full at commit `4eefb51`;
- `.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md` ("the Hub
  contract"), 2289 lines; the sections this plan builds on are cited by section and line.
  **Re-read at hub commit `ec9d05b`** (the Hub branch moved on while this plan was written; §1–§4,
  §7.0, §7.5–§7.8, §7.11, §8, §9.2–§9.4, §13–§15 were re-opened at that commit).

---

## Goal

Give the Design phase a canvas the owner can call *"pretty much exactly like paper.design with the
MCP"* (`OD-11`), built clean-room on permissive parts, that does four things end to end:

1. **Agents design on it.** A Claude Code session running in a sandbox connects to an MCP server
   named `panacea-canvas`, orients itself, reads the branch's brief, writes real HTML/CSS into
   artboards, verifies its work with screenshots, and releases its claim — while the Design lens is
   open, or while it is closed and the desktop is running.
2. **A human finishes what agents started.** The Design lens shows the same artboards as real DOM
   in script-less, sandboxed iframes, live as agents write, and lets the human select, move,
   resize, restyle, retype, reorder and undo their own work without touching HTML.
3. **Many writers never silently collide.** Every write is attributed to a session the server
   knows (never one the agent names), claims an artboard, and may carry an `ifRevision`
   precondition. The op protocol is server-authoritative, per-property last-writer-wins, and is
   written so the **same Go package** hosts cross-person multiplayer in the Hub in v2.
4. **The thread is kept.** A canvas is bound to its branch's brief; only a human approves, from our
   chrome, over a SHA-256 digest; the approval writes `design:<canvasId>@<rev>` → `brief:<id>@<rev>`
   to the Hub with the snapshot captured there; and a handoff bundle (HTML, PNG, JSX, `tokens.css`,
   DTCG `tokens.json`, `manifest.json`) lands in the worktree for a Develop session that is started
   *from* the design and linked to it.

**Out of this plan** (named seams in §"Seams"): cross-person live co-editing (v2, Hub-hosted),
components with slots, CSS Grid and `margin`, theme modes, MCP wiring for harnesses other than
Claude Code, Figma via Figma's MCP, import from a running preview, SVG/PDF export, visual diff,
pen tool, sketch boards.

_Sections follow as they are written._
