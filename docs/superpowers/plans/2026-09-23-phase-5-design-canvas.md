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
  **Re-read at hub commit `f684067`** (the Hub branch moved on while this plan was written; ADR
  §0–§5.9 and §6.5–§11, contract §1–§4.3, §7.0–§7.1, §7.5–§7.8, §7.11–§8, §9.2–§9.4, §13–§15 were
  re-opened at that commit). The Hub's own plan parts `2026-09-23-phase-h-hub-parts/task-03.md`
  (`hubapi` wire types, `:326-410`) and `task-04.md` (`hubclient.Designs`, `:91-100`) were read
  there too: `task-04` adds `Approval` and `SessionLinks` to the contract's `Designs` interface.

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

---

## Architecture

```
 sandbox (Docker now, Kata in phase 8)             control plane (Go, 127.0.0.1:8730)                        Hub (Phase H)
 ────────────────────────────────────              ──────────────────────────────────────────                 ─────────────
 claude … --mcp-config {panacea-canvas:            mcprelay.Supervisor ── dials ws://<endpoint>/mcp-channel
   http://127.0.0.1:8731/mcp}                        (bearer = the session's attach token; one channel
      │ Streamable HTTP (POST /mcp)                   per running session; stamps the session id)
      ▼                                                  │  tunnelled TCP streams, payload-blind framing
 sandboxd  127.0.0.1:8731 ──► mcprelay (mux) ═══════════╡
           :8080 /attach /healthz /mcp-channel           ▼
                                                   canvas/mcp.Server (JSON-RPC, 33 tools) ── Caller{session} ─┐
                                                                                                              │
 desktop (Tauri webview)                           canvas/authority.Service  ◄────────────────────────────────┤
 ───────────────────────                             one per process; canvases by (projectId, branchId)       │
 DesignLens (chrome) ─ designSlice                   ops · revisions · claims · undo · activity · subscribe   │
   │  <CanvasEditor/>  <CanvasRenderer/>             dialect (sanitiser) · serialize (HTML/JSX) · tokens      │
   │  from canvas/editor (own npm package)           store: statepath.Dir("canvas")/<cv-…>/                   │
   │                                                    │                                                     │
   ├─ REST /v1/design/… ──────────────────────────► designapi (routes through api's RouteMounter) ────────────┘
   └─ SSE  /v1/events?topic=canvas:cv-…@rev ◄──── events.Broker (one stream per window, topics)
          topic=renderer:cv-…  ◄─ render requests      │  approval: snapshot → PutBlob × n → Approve ──────────► /v1/projects/{id}/blobs/{sha}
          POST /v1/design/render-results/{id} ─►       │  handoff: GetBlob × n → <worktree>/.panacea/design/     /v1/projects/{id}/design-approvals
                                                       │  develop link: LinkSession ──────────────────────────► /v1/projects/{id}/session-links
                                                       └─ brief: discoverhub.Service (phase 4) ─ HeadRef/Binding ► /v1/refs, …/briefs/for-branch
```

**Five rules hold the shape.**

1. **The document authority is Go, in the control plane, and it is the only writer.** Every
   mutation — an agent's MCP call, a human's drag, a paste, an undo — becomes an op batch that
   `authority.Service.Apply` accepts or refuses, and each accepted batch is one new canvas
   revision. The desktop never mutates its copy except by applying batches the authority returned.
   This is the spec's §7.2 split, and the reason agents keep designing while the Design lens is
   closed: the authority lives as long as the control plane does, not as long as a window.
2. **No script ever runs in the canvas** (spec §7.1 rule 4). The dialect has no `<script>`, no `on*`
   attribute, no remote URL (§7.3). It is enforced three times: the Go sanitiser before an op is
   accepted; the renderer, which builds artboard DOM with `createElement` from allowlisted kinds and
   re-checks every style property against the same allowlist (generated from Go, never hand-copied);
   and the artboard document's own CSP inside an iframe whose `sandbox` has no `allow-scripts`.
3. **The canvas is its own package** (`OD-11` rule 4): a Go module `canvas/` (model, dialect,
   serialisers, authority, MCP server, handoff) and an npm package `canvas/editor` (renderer and
   editor). Neither imports the cockpit. The control plane adapts the Go module to its HTTP surface
   in `internal/designapi`; the desktop adapts the npm package to its API client in
   `desktop/src/design`. Both halves are testable alone: the Go module against a fake renderer and
   an in-memory store, the npm package against an in-memory transport.
4. **Identity comes from the channel, never from the payload.** An MCP call reaches the authority
   through the relay channel of exactly one sandbox, so the control plane stamps the session id on
   it. No tool takes an author, session or approver argument. A human's op arrives on the
   local-key-guarded REST surface and is authored `human`. Only a human approves, and only through
   `POST /v1/design/canvases/{id}/approvals`, which no MCP tool reaches (ADR-H1 R5's mitigation).
5. **The thread lives in the Hub; the canvas holds no copy of it.** The canvas stores no brief text
   and no approval: `get_brief` resolves the branch's brief through phase 4's
   `discoverhub.Service` at call time, and approvals are read from the Hub and held in memory only
   (ADR-H1 §5.4, A5 widened). An approval writes `design:<canvasId>@<rev>` + `reviewDigest` →
   `brief:<id>@<rev>` together with the snapshot's blobs, in one Hub route (Hub contract §7.7).

**Transport inside the window** (spec OQ-3, UI contract §10.5). Phase 5 adds the event stream the
phase 2 contracts asked for: `GET /v1/events`, **Server-Sent Events**, one stream per window,
multiplexed by `topic`. §10.5 asked for SSE by name; SSE is a read-only `GET`, so it sits in
`catalogReadOnlyRoutes` without a §10a.1 exception, and the desktop reads it with `fetch` and a
`ReadableStream`, so it carries the local caller key header (Hub contract §9.4) that `EventSource`
could not. Writes stay `POST`. The canvas is the stream's first two topics (`canvas:` and
`renderer:`); the poller is untouched, and moving sessions onto the stream is a later phase's seam.

**How an agent reaches MCP** (spec §7.7, OQ-4). `sandboxd` listens on `127.0.0.1:8731` inside the
sandbox and forwards each accepted TCP connection, as an opaque byte stream, over one WebSocket
the **control plane dials into** `sandboxd` (`/mcp-channel`, same bearer as `/attach`). The control
plane serves `canvas/mcp` on the far end of the tunnel with the session id in the request context.
Nothing in the sandbox reaches the host, no token is in the sandbox beyond the attach token it
already holds, and the sandbox side never parses a byte. Claude Code is pointed at the listener by
its documented `--mcp-config` flag, emitted by `harness.claudeArgv` beside the flags it already
passes (`control-plane/internal/harness/argv.go:80-125`).

**Where each part runs, v1 and v2.**

| Part | v1 (this plan) | v2 (named seam, not built) |
|---|---|---|
| Canvas documents, ops, claims, MCP | `canvas/authority` inside the control plane; files under `statepath.Dir("canvas")` | the same package hosted by the Hub over its own `/v1/canvas/…` WebSocket (ADR-H1 §5.5), with a Postgres `Store` |
| Renderer (layout, computed styles, PNG) | the desktop webview: the Design lens, or a hidden renderer while the desktop runs | unchanged |
| Approvals, design links, snapshots, session links | the Hub (contract §7.6–§7.8) | unchanged |
| Cross-person co-editing | **not built**: one human, many agents (spec OQ-1) | Hub-hosted, server-authoritative per-property last-writer-wins (spec §4, [S26]) |
