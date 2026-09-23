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

---

## The decisions this plan rests on

### Owner decisions (`planning/2026-09-23-owner-decisions.md`)

| Id | What it fixes here | Where |
|---|---|---|
| **OD-1** | Build now on the recommendations on file; this plan is adversarially reviewed before code; **every UI branch stays unmerged until the owner has looked at it** | throughout; tasks 23–28 |
| **OD-3**, **OD-5**, **OD-6** | Approvals, design links, snapshots and session links are shared, so they live in the Hub; the control plane calls it as the signed-in user; phase 5 runs no sign-in of its own | tasks 21, 22 |
| **OD-8** | Phase 5 reads **no** transcript and derives nothing from PTY bytes. Everything an agent does to the canvas arrives as a typed MCP call | global constraint G5 |
| **OD-9** | The cockpit's chords win by default; the canvas's single-letter tools act only while focus is inside the viewport | task 25 |
| **OD-11** | Clean-room (rule 1): nothing from `~/Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas. Public behaviour only (rule 2): the spec. Licences (rule 3): the gate of task 04. Own package (rule 4): `canvas/` and `canvas/editor` | tasks 02–04; global constraint G6 |
| **OD-12** | Every owner-observed step — a real agent designing in a sandbox, the visual pass — is batched to the end and stays **unverified** until then | task 29; every UI task's acceptance |
| **OD-13** | No Panacea source and no Panacea call in phase 5 | — |

### The phase 5 brief (`scope-and-risk-phase-5-design.md`), accepted under OD-1

| Id | Decision | How this plan builds it |
|---|---|---|
| **D1**, **D2** | Moot under OD-11 (owner decisions `:31-32`) | — |
| **D3** option A (`:194-206`) | **Pin and mark**: a brief edit never voids an approval; it shows as `superseded`, and a human re-reviews | there is no mirror to rewrite: *Re-review* appends a `review_mark` op (a new design revision whose digest covers the new brief key) and approves that revision (tasks 06, 21, 27) |
| R4, R6 (script on a server; a frame forging edits) | Deleted by construction (spec §11.2) | the dialect (task 07), the renderer (task 24) |
| R5 (parallel agents overwrite each other) | Designed out | claims and `ifRevision` (task 12) |
| R7 (headless agents cannot complete MCP OAuth) | Removed | the relay channel is the credential (tasks 18, 19) |
| R8 / P-g (a deleted canvas orphans design links) | Closed | the snapshot is captured into the Hub at approval (task 21) |
| R9 (a second transport) | Closed | one SSE stream per window, canvas as its first topics (task 20) |

### The Hub (ADR-H1 and the Hub contract), as built against

| Id | What phase 5 takes | Where |
|---|---|---|
| contract §7.6 | `PUT`/`GET …/blobs/{sha}` — raw bytes, SHA-256-addressed, the ten content types, 16 MiB a blob, 1 GiB a project | tasks 21, 22 |
| contract §7.7 | `POST …/design-approvals` writes approval, design link and snapshot in one call; the approver is the principal; immutable, no revocation; one approval per design key (`design_already_approved`) | task 21 |
| contract §7.8 | `POST …/session-links` for *Start a Develop session from this design* | task 22 |
| contract §3.2 | canvas ids minted `cv-` + 16 hex so they stay unique when v2 moves canvases into the Hub | task 05 |
| contract §4.3, §9.2 | `hubclient.Designs` and its fake; the relay rule (a Hub 4xx passes through; 5xx and transport are `503 upstream_unavailable`, `upstream: "hub"`) | tasks 21, 22 |
| ADR Q1 | an approval needs a **complete** brief revision (`brief_incomplete`) | task 21 |
| ADR Q3 | a designer may approve their own design in v1 | task 21 |
| ADR Q7, §5.5 | the Hub holds no canvas document in v1; v2 hosts this Go package | seam S5-H.1 |
| ADR §5.4 | nothing Hub-derived is persisted on the machine: approvals are held in memory by the control plane and never written to the canvas store | tasks 10, 21 |
| ADR R5 | no MCP tool approves; the approve route is reachable only from our chrome | tasks 16, 21 |

### The sibling plans, as built against

| Id | From | What phase 5 does with it |
|---|---|---|
| **P4-A**, **S4-P5.2** | phase 4 plan `:169`, `:411` | a session started from an approved design inherits the branch's latest complete brief like any other; phase 5 does **not** send `contextRef`, and records `session → design:` through the Hub's session links (task 22) |
| **S4-P5.1** | phase 4 plan `:410` | one key-source selector for the canvas's approved brief key (task 23) |
| **S4-P5.3** | phase 4 plan `:412` | `get_brief` resolves through `discoverhub.Service.HeadRef`; nothing is copied into the canvas (task 15) |
| **3-D6**, **S3-P5.1** | phase 3 plan `:134`, `:330` | the handoff bundle is never committed; it is excluded through the clone's `.git/info/exclude` (task 22) |

### Decisions this plan makes itself — recorded so a reviewer can overturn them

| Id | Decision | Reason | Cost if wrong |
|---|---|---|---|
| **P5-A** | **Our own MCP protocol layer**, no SDK: JSON-RPC 2.0 over Streamable HTTP `POST` with `application/json` responses; `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`; stateless (no `Mcp-Session-Id`) | the server needs five methods and no server-to-client request; the repository pins Go 1.23 (`go.work:1`, `azure-pipelines.yml` `GoTool@0 1.23`) and the SDK's minimum Go version is **unverified**; zero dependencies in the authority; the SDK's compound licence (spec L4) never reaches the gate | `modelcontextprotocol/go-sdk` drops in behind the same `http.Handler` and the same tool table (task 14 keeps them separate for this) |
| **P5-B** | The window stream is **SSE** over `fetch`, not a WebSocket | UI contract §10.5 asked for SSE; a `GET` needs no §10a.1 exception; `fetch` carries the local caller key, `EventSource` and a browser WebSocket cannot | a WebSocket later is one transport swap in `desktop/src/api/events.ts` and `internal/events` |
| **P5-C** | The relay is a **byte tunnel with our own stream framing**, not an HTTP proxy and not `hashicorp/yamux` | `yamux` is MPL-2.0, outside `OD-11` rule 3; a tunnel keeps `sandboxd` payload-blind (spec §7.7) and carries Streamable HTTP, keep-alive and any later SSE unchanged | the framing is ~300 lines in each module, pinned by shared golden vectors |
| **P5-D** | **Canvas tools are pre-allowed** for the agent: `--allowedTools mcp__panacea-canvas` beside `--mcp-config` | every canvas write is attributed, claimable and revertible by session, and touches no file; a permission prompt per `update_styles` makes the canvas unusable. **Open question OQ-P5-1** | one flag; a setting to turn it off |
| **P5-E** | Text nodes are **plain text** in v1 (newlines allowed). Mixed inline content (`<p>a <b>b</b></p>`) is flattened, with a `flattened-inline` entry in the dropped report | rich-text runs complicate every serialiser and the in-place editor; the guide teaches separate text nodes in a wrapping flex row | v2 adds `runs` to the text node; nothing else in the model moves |
| **P5-F** | An **SVG is one node** holding sanitised markup, not a subtree of layers | vector editing is v2 (spec §9.2); one node keeps the sanitiser, the digest and the editor simple | v2 decomposes it |
| **P5-G** | The handoff bundle lands in **`<worktree>/.panacea/design/<canvasId>@<rev>/`**, never `design/` | a repository may own a `design/` directory; a dot-directory excluded through `.git/info/exclude` (3-D6) collides with nothing | a path constant |
| **P5-H** | An approval covers **every artboard of the canvas at one revision** | the design key names a canvas revision, not a subset; subsets need a second key form | v2 adds artboard selection to the digest envelope |
| **P5-I** | *Approve*'s Undo is a **5-second delayed commit in the desktop**; nothing reaches the Hub until it lapses | the Hub has no revocation (contract §7.7), and `DESIGN-SYSTEM.md` asks for optimistic mutation with Undo, not a confirm dialog | closing the window inside 5 s drops the approval — stated in the toast |
| **P5-J** | **No handle, marquee, code-editor or sanitiser library** in the editor: own handles and marquee, a read-only code panel, and a renderer that builds DOM from the Go-generated allowlist | `moveable`/`selecto` were last pushed 2024-06 (spec L7, L8); a read-only panel needs no editor; DOMPurify purifies HTML strings, and the renderer never has one | the spec's recommended libraries remain allowed and gated |
| **P5-K** | **One revision per accepted batch**; a node's `revision` is the last revision that changed its own fields; `ifRevision` checks every node a batch modifies or deletes | a 30-node `write_html` is one step of undo and one entry in the feed; two agents on two artboards never conflict | — |
| **P5-L** | **One canvas per branch**, created by the lens's *Create canvas* or by the first MCP write; a read on a branch with no canvas answers `not-found` with the sentence that says how to create one | spec §7.3; the prototype's state 1 | per-project canvases later are a binding change |
| **P5-M** | The canvas has **no bound-brief field**. Its brief is its branch's, resolved at call time; an approval records the key it was reviewed against | a stored binding is a second copy of phase 4's inheritance rule | — |
| **P5-N** | *Start a Develop session from this design* is **three desktop calls**: materialise the bundle, create the session through the existing route, then link it | the control plane never calls its own session handler (no import cycle, no second create path); phase 4's P4-A ignores a caller's `contextRef` anyway | a crash between the second and third call leaves an unlinked session, which the lens shows with a *Link* action |
| **P5-O** | The canvas Go module is a **separate module in `go.work`**, required by the control plane through a `replace` | `OD-11` rule 4: ship and test alone. ADR-H1 §4.1 kept the Hub inside the control-plane module because it needs `httpx` and `ids`; the canvas needs neither | moving it into the control-plane module is a mechanical import rewrite |
| **P5-P** | The **licence gate** allows `MIT`, `Apache-2.0`, `BSD-2-Clause`, `BSD-3-Clause`, `0BSD`, `ISC`; evaluates SPDX `AND`/`OR`; denies `tldraw` and `@tldraw/*` by name whatever they declare; **enforces** every Go module linked into a binary and every npm production dependency; **reports** npm dev dependencies. **Open question OQ-P5-2** | `OD-11` rule 3 names four families; build tools never ship, and several ship data under CC licences (unverified which) | enforcing dev dependencies is one flag in the gate |
| **P5-Q** | The **hidden renderer** (spec OQ-7) is an off-screen renderer the desktop mounts for every canvas whose branch has a running Claude Code session, plus the one open in the lens | layout tools work while the lens is closed; `renderer-unavailable` means the desktop is not running | the registration rule is one selector |
| **P5-R** | **No shared claim vocabulary with phase 3** (spec OQ-6) | phase 3's claims are *observed* from git (phase 3 task 07), not leases; there is no second lease to share a shape with | revisit when phase 3 adds a lease |
| **P5-S** | `request_review` creates a **review request on the canvas**, shown in the Design lens and as a toast | the cockpit's ranked queue (`AGENTIC-UX.md` §4 rule 4) does not exist yet; seam S5-Q.1 | — |
| **P5-T** | `get_font_info` is answered by the renderer from `document.fonts` and a manifest of the app's bundled families; no `fontkit` | availability is a renderer fact; axis introspection is v2 | — |
| **P5-U** | The review digest uses **the canvas module's own canonical JSON** (struct fields in lexicographic tag order, `SetEscapeHTML(false)`), pinned by golden vectors | the canvas module cannot import `hubapi.Canonical` (control-plane `internal`), and the Hub never recomputes `reviewDigest` (contract §7.7, "What the Hub does not check") | — |
