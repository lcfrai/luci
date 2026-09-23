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
                                                   canvas/mcp.Server (JSON-RPC, 35 tools) ── Caller{session} ─┐
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
| **OD-1** | Build now on the recommendations on file; this plan is adversarially reviewed before code; **every UI branch stays unmerged until the owner has looked at it** | throughout; tasks 24–26, 28, 29 |
| **OD-3**, **OD-5**, **OD-6** | Approvals, design links, snapshots and session links are shared, so they live in the Hub; the control plane calls it as the signed-in user; phase 5 runs no sign-in of its own | tasks 21, 22 |
| **OD-8** | Phase 5 reads **no** transcript and derives nothing from PTY bytes. Everything an agent does to the canvas arrives as a typed MCP call | global constraint G5 |
| **OD-9** | The cockpit's chords win by default; the canvas's single-letter tools act only while focus is inside the viewport | task 25 |
| **OD-11** | Clean-room (rule 1): nothing from `~/Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas. Public behaviour only (rule 2): the spec. Licences (rule 3): the gate of task 04. Own package (rule 4): `canvas/` and `canvas/editor` | tasks 02–04; global constraint G6 |
| **OD-12** | Every owner-observed step — a real agent designing in a sandbox, the visual pass — is batched to the end and stays **unverified** until then | task 30; every UI task's acceptance |
| **OD-13** | No Panacea source and no Panacea call in phase 5 | — |

### The phase 5 brief (`scope-and-risk-phase-5-design.md`), accepted under OD-1

| Id | Decision | How this plan builds it |
|---|---|---|
| **D1**, **D2** | Moot under OD-11 (owner decisions `:31-32`) | — |
| **D3** option A (`:194-206`) | **Pin and mark**: a brief edit never voids an approval; it shows as `superseded`, and a human re-reviews | there is no mirror to rewrite: *Re-review* appends a `review_mark` op (a new design revision whose digest covers the new brief key) and approves that revision (tasks 06, 21, 28) |
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
| **S4-P5.1** | phase 4 plan `:410` | one key-source selector for the canvas's approved brief key (task 27) |
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
| **P5-L** | **One canvas per branch**, created by the lens's *Create canvas* or by the agent's first `create_artboard`; a read on a branch with no canvas answers `not-found` with the sentence that says how to create one — except `get_canvas_info`, which answers `exists: false` so an agent can orient | spec §7.3; the prototype's state 1 | per-project canvases later are a binding change |
| **P5-M** | The canvas has **no bound-brief field**. Its brief is its branch's, resolved at call time; an approval records the key it was reviewed against | a stored binding is a second copy of phase 4's inheritance rule | — |
| **P5-N** | *Start a Develop session from this design* is **three desktop calls**: materialise the bundle, create the session through the existing route, then link it | the control plane never calls its own session handler (no import cycle, no second create path); phase 4's P4-A ignores a caller's `contextRef` anyway | a crash between the second and third call leaves an unlinked session, which the lens shows with a *Link* action |
| **P5-O** | The canvas Go module is a **separate module in `go.work`**, required by the control plane through a `replace` | `OD-11` rule 4: ship and test alone. ADR-H1 §4.1 kept the Hub inside the control-plane module because it needs `httpx` and `ids`; the canvas needs neither | moving it into the control-plane module is a mechanical import rewrite |
| **P5-P** | The **licence gate** allows `MIT`, `Apache-2.0`, `BSD-2-Clause`, `BSD-3-Clause`, `0BSD`, `ISC`; evaluates SPDX `AND`/`OR`; denies `tldraw` and `@tldraw/*` by name whatever they declare; **enforces** every Go module linked into a binary and every npm production dependency; **reports** npm dev dependencies. It is **our own stdlib-only program** (`tools/licencegate`) over `go list -deps -json` and `npm ls --json`, not `go-licenses` and `license-checker` (spec L22, L23), because both must be downloaded and the corporate proxy breaks downloads. **Open question OQ-P5-2** | `OD-11` rule 3 names four families; build tools never ship, and several ship data under CC licences (unverified which); a gate developers cannot run locally is met only after a push | enforcing dev dependencies is one flag in the gate |
| **P5-Q** | The **hidden renderer** (spec OQ-7) is an off-screen renderer the desktop mounts for every canvas whose branch has a running Claude Code session, plus the one open in the lens | layout tools work while the lens is closed; `renderer-unavailable` means the desktop is not running | the registration rule is one selector |
| **P5-R** | **No shared claim vocabulary with phase 3** (spec OQ-6) | phase 3's claims are *observed* from git (phase 3 task 07), not leases; there is no second lease to share a shape with | revisit when phase 3 adds a lease |
| **P5-S** | `request_review` creates a **review request on the canvas**, shown in the Design lens and as a toast | the cockpit's ranked queue (`AGENTIC-UX.md` §4 rule 4) does not exist yet; seam S5-Q.1 | — |
| **P5-T** | `get_font_info` is answered by the renderer from `document.fonts` and a manifest of the app's bundled families; no `fontkit` | availability is a renderer fact; axis introspection is v2 | — |
| **P5-U** | The review digest uses **the canvas module's own canonical JSON** (struct fields in lexicographic tag order, `SetEscapeHTML(false)`), pinned by golden vectors | the canvas module cannot import `hubapi.Canonical` (control-plane `internal`), and the Hub never recomputes `reviewDigest` (contract §7.7, "What the Hub does not check") | — |
| **P5-V** | Events carry **resulting node state** (upserts and deletions), never ops, to the desktop | the desktop then needs no TypeScript copy of op semantics, which would drift from Go (ADR-H1 §7.3's reason, applied to ops) | a larger event per batch; bounded by the 64 KiB write cap |

---

## Global constraints

**Commands.**

- Go, from the repository root: `go test ./canvas/... ./control-plane/... ./sandboxd/... ./tools/... -race -count=1`,
  or one package path (`./tools/...` exists from task 04). `go test ./...` at the root is invalid (`go.work` lists modules, and the root
  is not one). `go vet` on the same three patterns must stay clean. Task 02 adds `./canvas` to
  `go.work`; before task 02 lands the `./canvas/...` pattern does not exist.
- `sandboxd` is **vendored** (`sandboxd/vendor/`). Phase 5 adds no `sandboxd` requirement: its relay
  uses `github.com/coder/websocket`, already required and vendored (`sandboxd/go.mod`).
- The editor package: `cd canvas/editor && npx vitest run` and `npx tsc --noEmit`.
- The desktop: `cd desktop && npx vitest run <path>` and `npx tsc --noEmit`.
- **Never `rm -rf desktop/node_modules`** after task 03. `npm install` makes
  `desktop/node_modules/@panacea-app/canvas-editor` a **junction** to `canvas/editor`; a recursive
  delete that follows it deletes the editor's source. Remove the junction first
  (`cmd //c rmdir desktop\\node_modules\\@panacea-app\\canvas-editor`), then the tree.
- Every shell command in an agent session goes through `~/.claude/tools/claudezsh/claudezsh`.
  Commit by pathspec: `git commit -F <message file> -- <paths>`, after `git add <new paths>`. Never
  `git add -A`/`-u`/`.`, stash, reset, checkout, switch, rebase, merge, amend or push. Name the
  message file **uniquely per commit** (`p5-<task>-<n>.txt`): this plan's own commit `7e3a367`
  carries a stale message because an earlier run's file had the same name.
- Every commit message ends with exactly:
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`

**What never changes.**

- **G1.** `go` stays `1.23` in every `go.mod` and in `go.work`. The canvas module's one requirement
  is `golang.org/x/net v0.26.0` (BSD-3-Clause, spec L1) — **that exact version**, because it is
  already in the control plane's module graph (`control-plane/go.mod:52`) and so in the local module
  cache: the corporate proxy breaks TLS to `proxy.golang.org` (`images/sandbox/Dockerfile:3-6`), so a
  new download may fail. For the same reason the inline-CSS tokenizer is written in task 07 rather
  than taken from `tdewolff/parse` (spec L3). The control plane gains one requirement, the canvas
  module itself, through a `replace` (task 19, its first importer).
- **G2.** `desktop/src/store/types.ts` does not change, byte for byte (phase 2 task 15's gate).
  Design state lives in its own slice with its own types file.
- **G3.** `PERSIST_VERSION` stays `1`; `partialize` does not change. Phase 5 claims **no** persist
  version (ruling R3): the design slice is not persisted.
- **G4.** The control plane listens on loopback only; `sandboxd`'s MCP listener binds
  `127.0.0.1:8731` inside the sandbox and nothing else. Configuration comes from `PANACEA_APP_*`
  variables (task 20 lists every new one). No secret is in source. No subscription is pooled: the
  canvas never calls a model, and its agents are the user's own sessions (`CLAUDE.md`).
- **G5.** Harnesses run **unmodified and unwrapped** (`OD-8`). Phase 5 adds two documented Claude
  Code flags to the argv and nothing else. It reads no PTY byte and no transcript. `sandboxd`'s
  relay forwards bytes it never parses.
- **G6.** **Clean room** (`OD-11` rule 1). No implementer opens, reads, searches or copies
  `~/Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas. No search in this phase is
  rooted above the repository. Tool names follow Paper's public verbs (spec OQ-5); no Paper name,
  mark or claim of compatibility appears in the UI, in a guide, or in `serverInfo`.
- **G7.** **No script runs in the canvas.** No code path turns agent- or paste-supplied text into
  `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `srcdoc` markup, `document.write` or `eval`. The
  renderer's only HTML string is the fixed artboard shell of task 24. Its one other string-to-DOM
  path is `DOMParser` over an SVG node's server-sanitised canonical markup, followed by an
  allowlist walk that removes anything outside task 07's SVG lists. A test in task 24 greps the
  editor package for those identifiers and allows exactly the shell.
- **G8.** Only the authority mutates a document. The desktop replaces nodes from events (P5-V); it
  never applies an op of its own, optimistic or otherwise. A human's drag is previewed as a
  transform on the overlay and committed as one batch on pointer-up.
- **G9.** Only the `provenance` package (phase 4) parses a brief key; only `canvas/doc` mints or
  parses a canvas id and a design key. The desktop compares keys for equality and never splits one.
  **No TypeScript digest exists.**
- **G10.** One `setInterval` for data (the poller) and one clock for display (`hooks/heartbeat.ts`).
  Phase 5 adds **no** interval: the event stream is a `fetch`, the render queue is driven by
  `requestAnimationFrame` only while a paint is pending, and presence pulses ride the heartbeat.
- **G11.** One window `keydown` listener (`useKeyboardMap`). The canvas's single-letter tools are an
  element-scoped handler on the viewport, active only while focus is inside it (`OD-9`).
- **G12.** Every UI task reads `docs/DESIGN-SYSTEM.md` first and holds its non-negotiables: no
  indigo or violet, **no gradient in our chrome** (a gradient inside an artboard is design content,
  not chrome), elevation is a 1px ring plus a layered shadow (`--e-hair` … `--e-overlay`), radius
  by element size (`--r-chip` 6, `--r-ctl` 8, `--r-card` 10, `--r-window` 14), `transform`/
  `opacity` only and never `transition: all`, one heartbeat and no spinner per row, `tabular-nums`
  on every live number, colour plus shape for every state, the accent only for interactive and
  current state, `prefers-reduced-motion` on everything. The flat `--canvas` surface has **no dot
  grid** (brief R13). **The owner's eyes are the only visual acceptance** (`CLAUDE.md`); a
  screenshot or a passing test is not.
- **G13.** The editor package imports nothing from `desktop/`; it styles itself only with the CSS
  custom properties the host defines (`--page` … `--field`, `--ink` … `--ink-4`, `--line-*`,
  `--e-*`, `--r-*`, `--accent*`, `--run`/`--need`/`--fail`/`--drift`/`--idle`, `--ease`,
  `--dur*`, `--sans`, `--mono`, `--numeric` — the set in `desktop/src/styles/tokens.css`). A test
  in task 23 fails on any `import` that leaves the package.

---

## File map

**New Go module `canvas/`** (`github.com/kpmg-au-emu/panacea-app/canvas`):

| Path | Holds | Task |
|---|---|---|
| `canvas/go.mod` | module, `go 1.23`; its one requirement (G1) is added by task 07, its first importer | 02, 07 |
| `canvas/boundary_test.go`, `canvas/doc/` | the module boundary; `model.go` (Page, Node, Token, Asset, Thread, ReviewRequest, Author, Document, Doc), `ids.go`, `key.go` (design key), `order.go` (fractional index), `validate.go` | 05 |
| `canvas/doc/` | `op.go`, `apply.go` (Apply, Prepare, Commit), `invert.go` | 06 |
| `canvas/dialect/` | `parse.go`, `css.go` (our tokenizer), `allowlist.go`, `svg.go`, `report.go`, `checker.go`, `gen.go`, `allowlist.json` (generated) | 07 |
| `canvas/serialize/` | `html.go` (canonical dialect), `export.go` (standalone HTML), `canonjson.go`, `digest.go` | 08 |
| `canvas/serialize/`, `canvas/tokens/` | `jsx.go`, `tailwind.go`, `tree.go`; `css.go`, `dtcg.go` | 09 |
| `canvas/store/` | `store.go` (interface), `filestore.go`, `memstore.go`, `replay.go`, `storetest/suite.go` | 10 |
| `canvas/authority/` | `service.go`, `binding.go`, `apply.go`, `subscribe.go`, `activity.go`, `errors.go` | 11 |
| `canvas/authority/` | `claims.go`, `precondition.go`, `notices.go`, `presence.go`, `undo.go`, `revert.go` | 12 |
| `canvas/authority/` | `render.go` (renderer registry), `selection.go` | 13 |
| `canvas/mcp/` | `jsonrpc.go`, `transport.go`, `server.go`, `registry.go`, `schema.go`, `errors.go`, `identity.go` | 14 |
| `canvas/mcp/` | `tools.go` (the builder and the error table), `ports.go` (Briefs, Approvals), `views.go`, `tools_read.go` | 15 |
| `canvas/mcp/` | `tools_write.go`, `place.go` | 16 |
| `canvas/mcp/` | `tools_tokens.go`, `tools_comments.go`, `tools_export.go`, `guides.go`, `instructions.go`, `guides/*.md` | 17 |
| `canvas/handoff/` | `snapshot.go`, `manifest.go`, `slug.go` (21); `bundle.go` (22) | 21, 22 |
| `canvas/canvastest/` | `fixtures.go` (11), `renderer.go` (13), `ports.go` (15) | 11, 13, 15 |

**New npm package `canvas/editor/`** (`@panacea-app/canvas-editor`, private):

| Path | Holds | Task |
|---|---|---|
| `package.json`, `package-lock.json`, `tsconfig.json`, `vitest.config.ts`, `src/index.ts`, `src/test/setup.ts`, `src/probe.tsx` (deleted by 23) | scaffold; `react`/`react-dom` as peers | 03 |
| `spike/` (throwaway, outside the build) | the iframe spike page and its runner | 01 |
| `src/wire.ts`, `ops.ts`, `docstore.ts`, `transport.ts`, `memoryTransport.ts`, `allowlist.ts`, `generated/allowlist.json`, `scripts/sync-allowlist.mjs`, `boundary.test.ts` | wire types, op builders, the document store, the transport port, the allowlist copy | 23 |
| `src/renderer/` | `shell.ts`, `buildDom.ts`, `patch.ts`, `ArtboardFrame.tsx`, `assets.ts`, `measure.ts`, `capture.ts`, `fonts.ts`, `RenderResponder.ts`, `HiddenRenderer.tsx`; `src/g7.test.ts` | 24 |
| `src/editor/` | `CanvasEditor.tsx`, `viewport.ts`, `hitTest.ts`, `selection.ts`, `handles.tsx`, `drag.ts`, `snap.ts`, `keys.ts`, `presence.tsx`, `ids.ts`, `editor.css`; `dev/index.html` (the hand-driven harness) | 25 |
| `src/panels/` | `LeftPanel.tsx`, `Layers.tsx`, `ThemeTab.tsx`, `RightPanel.tsx`, `LayoutPanel.tsx`, `FillPanel.tsx`, `BorderPanel.tsx`, `EffectsPanel.tsx`, `TypePanel.tsx`, `TokenPicker.tsx`, `ColorField.tsx`, `LengthField.tsx`, `CodePanel.tsx`, `TextEdit.tsx`, `panels.css`, `virtualList.ts` | 26 |
| `src/editor/paste.ts`, `copy.ts` | the paste event's routing; copy and cut | 29 |

**Control plane** (`control-plane/`):

| Path | Change | Task |
|---|---|---|
| `go.mod` | `require github.com/kpmg-au-emu/panacea-app/canvas` + `replace … => ../canvas`, added by the first importer | 19 |
| `internal/mcprelay/` (new) | `frame.go` (the tunnel framing), `tunnel.go` (a `net.Listener` over one channel), `supervisor.go` | 18, 19 |
| `internal/harness/argv.go`, `harness.go` | `Request.CanvasMCP`, `Descriptor.SupportsMCPConfig`, the two flags | 19 |
| `internal/api/sessions.go` | set `CanvasMCP` in create and restart | 19 |
| `internal/api/api.go` | `Deps.CanvasMCP bool` (19); `Deps.Design RouteMounter` (phase 4's interface, task 10 of its plan) (20) | 19, 20 |
| `internal/events/` (new) | `broker.go`, `sse.go` — the window stream | 20 |
| `internal/designapi/` (new) | `routes.go`, `mount.go`, `canvas.go`, `ops.go`, `assets.go`, `comments.go`, `render.go`, `export.go`, `briefs.go` (the neutral port; `briefs_phase4.go` adapts `discoverhub` once it exists), `problems.go` (20); `approve.go`, `approvals.go`, `hub.go` (21); `handoff.go`, `links.go`, `gitexclude.go` (22); `paste.go` (29); `e2e_test.go` (30) | 20–22, 29, 30 |
| `internal/httpx/problem.go` | the design codes (task 20 lists them) | 20 |
| `cmd/api/config.go`, `main.go` | `PANACEA_APP_CANVAS_*` variables; wiring | 19, 20 |
| `internal/api/routes_audit_test.go` | the new GET routes in `catalogReadOnlyRoutes` | 20 |

**`sandboxd/`**: `internal/mcprelay/` (new: `frame.go`, `listener.go`, `channel.go`), `internal/server/server.go`
(`/mcp-channel`), `cmd/sandboxd/main.go` (`-mcp-listen`) — task 18.

**Desktop** (`desktop/`):

| Path | Change | Task |
|---|---|---|
| `package.json`, `vite.config.ts`, `vitest.config.ts`, `tsconfig.json` | the `file:` dependency; `resolve.dedupe` for React; `server.fs.allow` for `../canvas/editor`; `paths` pinning one `@types/react` — **declared amendments** | 03 |
| `src/api/design.ts`, `src/api/events.ts` (new) | the design client; the SSE reader | 27 |
| `src/design/` (new) | `designTypes.ts`, `designSlice.ts`, `transportAdapter.ts`, `DesignLens.tsx`, `ApprovalBar.tsx`, `DesignSide.tsx`, `Presence.tsx`, `Comments.tsx`, `design.css` | 27, 28 |
| `src/store/store.ts`, `src/store/selectors.ts` | mount the slice (not persisted); one key-source selector (S4-P5.1) | 27 |
| `src/components/AppShell.tsx` | the `design` arm renders `<DesignLens/>` in place of `LensPlaceholder` (`:36-43`); one `<DesignRenderers/>` beside `<Toasts/>` for the hidden renderers (P5-Q) | 28 |
| `src/api/client.ts` | the `ProblemCode` union gains the design codes — **declared amendment** of UI contract §4.2 | 27 |

**Repository**: `go.work` (task 02); `tools/licencegate/` (new module: `main.go`, `spdx.go`,
`exceptions.json`, `testdata/`) and `azure-pipelines.yml` (tasks 02–04);
`docs/superpowers/contracts/2026-09-23-phase-5-design-contract.md` (new, task 30: the wire contract
gathered from the parts), and amendments to the phase 2 UI contract §10.5 and `PROGRAM.md` phase 5
(task 30).

---

## Tasks — dependencies and order

Each part is `2026-09-23-phase-5-design-canvas-parts/task-NN.md`. Hours assume an implementer who
has read this file, the spec §7–§8 and the part. **External** dependencies are on sibling plans'
tasks; each part says what it codes against until that task lands.

| # | Task | Depends on | External | Hours |
|---|---|---|---|---|
| 01 | **Spike**: the script-less artboard iframe in WebView2 — measure, hit-test, text overlay, keys, PNG, CSP | — | — | 2 |
| 02 | The `canvas/` Go module, `go.work`, the control plane's `replace`, CI build/test/vet | — | — | 1 |
| 03 | The `canvas/editor` npm package, the desktop's `file:` dependency and its three declared config amendments | — | — | 1.5 |
| 04 | **The licence gate**: `tools/licencegate`, SPDX evaluation, deny list, exceptions, CI step | 02, 03 | — | 2.5 |
| 05 | The document model, ids, the design key, fractional order | 02 | — | 2 |
| 06 | Ops: kinds, validation, apply, inverse, `review_mark` | 05 | — | 3 |
| 07 | The dialect: HTML and inline CSS into nodes, the allowlist, the dropped report, `data:` into assets | 05 | — | 3 |
| 08 | Canonical HTML, export HTML, canonical JSON, the review digest | 05, 07 | — | 2.5 |
| 09 | JSX (inline and Tailwind), `tree.json`, `tokens.css`, DTCG `tokens.json` | 08 | — | 3 |
| 10 | The store: op log, snapshots, reconstruction at a revision, crash safety | 06 | — | 2.5 |
| 11 | Authority I: canvases by branch, `Apply` with an author, subscriptions, the activity feed | 06, 10 | — | 3 |
| 12 | Authority II: claims, `ifRevision`, notices, per-author undo and redo, revert by session | 11 | — | 3 |
| 13 | The render bridge and the human's selection | 11 | — | 2 |
| 14 | The MCP protocol layer | 05 | — | 2.5 |
| 15 | MCP read tools, `get_brief` and its port | 08, 09, 12, 13, 14 | phase 4 task 07 (coded against a port and a fake) | 3 |
| 16 | MCP write tools | 07, 12, 14 | — | 3 |
| 17 | MCP tokens, comments, export, guides, server instructions | 09, 15, 16 | — | 2.5 |
| 18 | The relay framing in both modules; `sandboxd`'s listener and `/mcp-channel` | — | — | 3 |
| 19 | The relay supervisor; `--mcp-config` and `--allowedTools`; session wiring | 14, 18 | — | 2.5 |
| 20 | `designapi` routes, the SSE window stream, configuration, wiring | 11, 12, 13 | phase 4 task 10 (`RouteMounter`; declared here if absent) | 3 |
| 21 | Approval: snapshot, blobs, the Hub approval, *changed after approval*, re-review | 08, 09, 13, 20 | Hub H0/C2 (`hubclient.Designs` + fake); phase 4 task 07 (`discoverhub`) | 3 |
| 22 | Handoff: the bundle into the worktree; the session link | 21 | Hub H4 fake; phase 3 3-D6 | 2 |
| 23 | Editor data layer: wire types, the document store, the transport port, the allowlist check | 03, 07, 11 | — | 2.5 |
| 24 | The renderer: artboard iframes, assets, measurement, capture, the render responder, the hidden renderer | 01, 23 | — | 3 |
| 25 | The editor: viewport, selection, marquee, move and resize, snapping, canvas keys | 24 | — | 3 |
| 26 | The editor: layers, layout, fill, border, effects, type, token picker, code panel, text in place | 25 | — | 3 |
| 27 | Desktop data: `design.ts`, `events.ts`, the design slice, the transport adapter, the key-source selector | 20, 23 | phase 4 task 16 (the loader) | 2.5 |
| 28 | The Design lens chrome: seven states, approval bar, side panel, presence, comments, review requests | 21, 22, 26, 27 | phase 4 task 17 (`ContextChip`) | 3 |
| 29 | Paste: HTML, SVG and images | 20, 26 | — | 1.5 |
| 30 | End to end, the phase 5 contract document, and the amendments | all | OD-12's hands-on session | 2 |

**Total ≈ 77 hours.** Critical path: 02 → 05 → 06 → 10 → 11 → 12 → 15 → 17, then 20 → 21 → 22 → 28 → 30.

**Order, in waves** (a wave's tasks are independent of each other):

1. **01, 02, 03, 18** — the spike, both scaffolds, the relay framing. 01 gates 24–26; if it fails
   an item, its report names the fallback each later part already carries.
2. **04, 05** — the gate before any dependency is added; the model.
3. **06, 07, 14** → **08, 10** → **09, 11**.
4. **12, 13, 19** → **15, 16, 20, 23**.
5. **17, 21, 24** → **22, 25** → **26, 27** → **28, 29** → **30**.

**Parallel lanes for worktree agents.** Lane A (Go core): 05–13. Lane B (MCP): 14–17, after 12 and
13. Lane C (relay): 18–19. Lane D (editor): 23–26, after 01 and 03. Lane E (control-plane HTTP and
provenance): 20–22. The desktop lane (27–29) joins D and E. **Two lanes never edit one file**: the
only shared files are `control-plane/go.mod` (19 only), `internal/api/api.go` (19, then 20),
`internal/api/sessions.go` (19 only), `internal/httpx/problem.go` (20 only) and
`azure-pipelines.yml` (02, then 03, then 04, in that order).

---

## Seams to later phases

| Id | Seam | The later phase | Must not |
|---|---|---|---|
| **S5-H.1** | `authority.Service` over the `store.Store` interface; authors carry a `UserID` that is `"local"` in v1 | v2: the Hub hosts this package, with a Postgres `Store` and the Hub principal as `UserID`, behind `/v1/canvas/…` (ADR-H1 §5.5) | fork the op protocol for the Hub |
| **S5-H.2** | The Hub's boundary test (`go list -deps ./cmd/hub`, contract §2) allows no canvas import in v1 | v2 adds `github.com/kpmg-au-emu/panacea-app/canvas/...` to its allowlist | host the canvas without amending the test |
| **S5-P3.1** | The handoff writes into a worktree the caller names (S3-P5.1) | phase 3's *create a worktree from this design* calls task 22's route after its own create | commit `.panacea/` |
| **S5-P6.1** | Every agent-caused op is attributed to a session | phase 6's activity and cost views read `list_activity` / the activity route | derive activity from PTY bytes |
| **S5-P7.1** | `design:` keys resolve through the Hub's `/v1/refs` (contract §7.5) | phase 7's release manifest renders the approved design key | read the canvas for a release |
| **S5-P8.1** | The relay dials `<endpoint>/mcp-channel`, and `Endpoint` is whatever the backend reports (`backend.Sandbox.Endpoint`) | phase 8's `katabe` pods serve the same path on `:8080` | open a port on the node for MCP |
| **S5-Q.1** | `request_review` makes a canvas review request | a later cockpit queue (`AGENTIC-UX.md` §4 rule 4) ranks it | count it outside the queue |
| **S5-Hx.1** | `harness.Descriptor.SupportsMCPConfig` | v2: Codex, OpenCode and Gemini CLI, each through its own documented MCP configuration | wrap a CLI to inject MCP |
| **S5-UI.1** | `GET /v1/events` topics | a later phase moves sessions onto the stream (UI contract §10.5) and retires the poller's session read | open a second stream per window |

---

## Hub contract asks

Each is a shape phase 5 needs that the Hub contract (read at `.claude/worktrees/hub`, commit
`f684067`) does not state, or states two ways. **No Hub route is invented here.** Each says what
phase 5 does until it is answered.

| Id | Ask | Exact shape | Until answered |
|---|---|---|---|
| **HA5-1** | `PutBlob`'s retry (contract §9.2: *"retries a transport failure or a 502/503/504 twice"*) cannot replay an `io.Reader` it has consumed | change the parameter to `body io.ReadSeeker` and seek to 0 before each attempt; or state that `PutBlob` never retries | phase 5 always passes a `*bytes.Reader` (every snapshot file is in memory, ≤ 16 MiB) and retries a failed upload itself, once, with a fresh reader (task 21) |
| **HA5-2** | Contract §9.2's `Designs` has five methods; the Hub plan's `task-04.md:91-100` has seven (`Approval`, `SessionLinks` added) | amend §9.2 to the plan's seven | phase 5 uses only the contract's five plus `Approval` (task 22 reads one approval by id) and codes a local interface of exactly those six |
| **HA5-3** | The fake must model every check of §7.7 (1–6) and §7.8, and §7.6's `digest_mismatch` and `quota_exceeded`, because phase 5's tests run only against it | confirmation that `NewFake()` implements them, with the same error codes and `details` | task 21's tests assert those codes against the fake; a gap is a failing test that names this ask |
| **HA5-4** | ADR-H1 R5 says *"every approval is audited with its session context"*; `ApprovalRequest` (contract §7.7; `task-03.md:353-360`) has no field that could carry one, and §5.3 forbids actor fields | delete the phrase from R5 (recommended — the Hub records the principal and the time, which is all it can know), **or** add an optional `"surface": "design-lens"` to `ApprovalRequest` | phase 5 sends the contract's request exactly; no extra field |
| **HA5-5** | v2 only, recorded now so it is not a surprise: canvases move into the Hub | a `canvases` table keyed by `(project, branchRef)`, a `/v1/canvas/{canvasId}/stream` WebSocket, and `canvas/...` in the §2 boundary allowlist (S5-H.2) | nothing in v1 |

---

## Open questions for the owner — each with the recommendation this plan builds on

| # | Question | Recommendation (built) | Changes if overruled |
|---|---|---|---|
| **OQ-P5-1** | May an agent use the canvas tools **without a permission prompt per call**? | **Yes** (P5-D): `--allowedTools mcp__panacea-canvas`. Every canvas write is attributed to the session, claimable, and revertible by session; none touches a file | task 19 drops one flag; agents prompt on every canvas call |
| **OQ-P5-2** | Should the licence gate **fail** on a dev-only npm dependency (a build tool that never ships)? And should it scan **Cargo** (the Tauri shell's crate tree, which predates `OD-11`)? | **No — report dev dependencies; do not scan Cargo in phase 5** (P5-P). The gate fails on everything the canvas could add that ships; the canvas adds no crate | task 04 flips one flag, or adds a `cargo metadata` source; either may surface existing dependencies that need review |
| **OQ-P5-3** | Tool names that follow Paper's public verbs (`write_html`, `update_styles` …) — spec OQ-5 | **Yes for the verbs, no for the brand** (G6). A short legal confirmation is cheap; this is not legal advice | task 14's registry renames |
| **OQ-P5-4** | Is v1 one human with many agents, and cross-person co-editing v2? — spec OQ-1 | **Yes.** The shared part of the thread (approvals, links, snapshots) is shared from v1 (`OD-3`) | v2 moves ahead of phases 6–8 |
| **OQ-P5-5** | The canvas's bundled fonts are the design system's Geist Sans and Commit Mono, both **OFL-1.1** — a font licence outside `OD-11` rule 3's four | **Allowed as assets**: the rule governs code dependencies, and `DESIGN-SYSTEM.md` already chose these faces. The gate does not scan fonts | fonts become system-only on the canvas |

Answered elsewhere, recorded so nobody asks again: spec OQ-2 (documents in the control plane in
v1 — ADR-H1 Q7 agrees); OQ-3 (P5-B); OQ-4 (the relay, tasks 18–19); OQ-6 (P5-R); OQ-7 (P5-Q);
OQ-8 (no CC0 — fractional indexing is written in task 05); OQ-9 (spike task 01); OQ-10 (the bundle
is not committed — phase 3's 3-D6); OQ-11 (Figma via its MCP is v2, behind owner and security).

---

## What this plan did not verify

- **Nothing was compiled, run or rendered.** Every claim about code is a reading of the tree at
  `3303fd6` plus this branch's docs commits. Phase 2 is still committing on `feat/phase-2-cockpit`;
  line numbers in `sessions.go`, `api.go`, `argv.go` and `desktop/src` will drift, so every part
  names the symbol as well as the line.
- **WebView2 behaviour**: a `srcdoc` iframe with `sandbox="allow-same-origin"` and no
  `allow-scripts` — that the parent can read its layout, that nothing inside runs, that a `<meta>`
  CSP applies, and how a PNG is best captured. Task 01 is the spike; every later UI part names its
  fallback.
- **Claude Code's flags**: that `--mcp-config` accepts an inline JSON string with `"type":"http"`,
  whether it and `--allowedTools` are variadic (they are emitted first for that reason, task 19),
  and that `mcp__panacea-canvas` allows every tool of the server. From knowledge of the CLI, not
  from `claude --help` 2.1.274 in `panacea-app/sandbox:dev`; task 19's first step checks all three.
- **Claude Code's MCP client**: the protocol revision it asks for, whether it ever batches, how
  long it waits at start for a server that is not yet reachable (the listener holds connections
  until the channel opens, task 18), and how it recovers when a connection drops.
- **The MCP specification details** used in task 14 (Streamable HTTP; `application/json` single
  responses; `GET` answered 405; no session id) are from knowledge of revisions 2025-03-26,
  2025-06-18 and 2025-11-25, not re-read tonight.
- **The licence tools**: `google/go-licenses`' handling of a compound `LICENSE` file and its own Go
  version requirement; `license-checker-rseidelsohn`'s JSON output. Task 04 pins both and tests the
  gate's evaluator on fixtures, not on the tools' behaviour.
- **npm and Vite on Windows**: that `file:../canvas/editor` produces a junction, that
  `resolve.dedupe` yields one React in both the build and vitest, and that `server.fs.allow` lets
  `tauri dev` serve the package. Task 03's acceptance measures each.
- **Sibling plans are plan text.** Phase 4's `discoverhub`, `provenance`, `ContextChip` and loader;
  phase 3's 3-D6 exclude rule; the Hub's `hubclient`, `hubapi` and fake. Each part codes against a
  port it declares and a fake, and names the sibling task that replaces the fake.
- **The prototype** `prototype/phase-5-design.html` (577 lines) was read for its seven states and
  their notes, not opened in a browser. Its states were drawn around an embedded Northstar viewport;
  task 28 maps each onto our canvas and says which changed.
- **The owner's eyes remain the visual acceptance** (`CLAUDE.md`), batched to the end (`OD-12`).
