# Survey — Phase 5, the Design canvas (product journey: **Design**)

Written 2026-09-23. Read-only pass. No code, no plan, no git write, no network probe, no
`bun install`, no test run. Scope: what `~/Projects/Paperdesign` actually is, what an agent can
do to a frame through its MCP server, how frames are sandboxed, whether multiplayer is
load-bearing, the licence position stated precisely, and what phase 5 needs from phase 2's
substrate and from the provenance thread.

**Sibling repo present and readable:** `~/Projects/Paperdesign`. Read from `src/`, `server/`,
`desktop/`, `shared/`, `tests/` and the top-level manifests only. `node_modules/` and `data/`
were not searched. Everything below is read from source unless explicitly marked unverified.

---

## Verdict, first

**Paperdesign is a working, well-built canvas with a real MCP surface — and it is not the
component the programme thinks it is. It is a stateful server application, not a Tauri app we
can merge; it has grown a second product spine; it lives uncommitted on one laptop; and its
licence question is unanswered and already live.**

| | |
|---|---|
| **Better than expected** | A 32-tool MCP server over stateless Streamable HTTP with OAuth and per-user canvas gating (§2). Frames sandboxed `allow-scripts`-only with an opaque origin (§3.1). A **digest-bound human approval** over the exact design — the best provenance primitive anywhere in the programme so far (§7.1). PGlite-or-Postgres with no code change (§4.3). |
| **Wrong — blocker** | **The licence.** `AGPL-3.0-only`; §13 fires under our we-host model; the copy on disk is already modified (50 files); the running app offers no Corresponding Source. **Decider: legal** (§5). |
| **Wrong — high** | **"Already Tauri-shelled" (`PROGRAM.md:92`) is true and irrelevant.** The shell is 159 lines of window chrome over a remote URL. The product is a 14,409-line Node server plus a React 18 client. Phase 5 needs a **server to host** (§1.2, §6.2). |
| **Wrong — high** | **The fork has its own Define→Deploy workflow** (`DISCOVERING` … `DEPLOYED`), duplicating phases 4 and 7. Two spines drift (§7.1). **Decider: product owner.** |
| **Wrong — high** | **Nothing is committed.** Shallow clone of upstream; 75 local changes; no remote of ours (§0.2). **Decider: product owner** — where to push. |
| **Wrong — medium** | **The handoff bundle does not exist.** Screens yes (frames, plus approved snapshots). Tokens only as markdown. **No component tree** (§2.3). |
| **Multiplayer** | Server-authoritative broadcast, **no CRDT**, last-writer-wins per whole frame. **Not optional** — the canvas loads over the WebSocket even for one user. Single process; scales up, not out (§4). |
| **Two defects found** | `$`-pattern corruption in `edit_frame_html` (upstream's too) (§2.4); a frame's script can persist edits to itself as the viewing human (§3.1). Both read from source, not reproduced. |

---

## 0. The repo is not what the brief says it is

Three facts about `~/Projects/Paperdesign` that the brief, `CLAUDE.md` and `PROGRAM.md` do not
carry, each of which changes how phase 5 is scoped.

**0.1 It has already been rebranded and re-purposed — as "Northstar Canvas", with its own
Define→Deploy workflow.** `README.md:1-3`: *"Northstar Canvas is a self-hostable, multiplayer
HTML design workspace … The Northstar workflow adds explicit Define-stage gates so evidence,
responsive design, review, and approval stay visible."* `package.json:2` is
`"name": "northstar-canvas"`. The fork carries a ten-state per-canvas state machine —
`DISCOVERING → DEFINING → VISION_REVIEW → VISION_APPROVED → BUILDING → VERIFYING → VERIFIED →
DEPLOY_PENDING → DEPLOYED`, plus `REJECTED` — in `shared/northstarWorkflow.ts:1-11` and
`:207-216`, with human-only approval gates `VISION`, `VERIFICATION`, `DEPLOYMENT` (`:23`,
`:245-255`). That is **a second product spine**, owned by the canvas, overlapping PanaceaApp's
own Discover/Design/Develop/Deploy lenses. See §7 — it is the single largest scoping question
in this phase.

**0.2 None of that re-purposing is committed.** The clone is shallow (`.git/shallow` exists;
`git rev-list --count HEAD` = 1). Its one commit is upstream's `bbb174e` *"feat(mcp): add
read-only access to canvas comments (#117)"*, authored 2026-09-07 by an upstream contributor. The
only remote is `upstream → github.com/kgoedecke/doop.git`. `git status --short` reports **75
entries: 50 modified tracked files (+1007 / −787 lines) and 25 untracked**, including every
Northstar file — `server/northstarWorkflow.ts`, `shared/northstarWorkflow.ts`,
`server/agentScope.ts`, `src/components/WorkflowGateDrawer.tsx`, migrations
`0010_northstar_workflows.sql` … `0012_task_targets.sql`, and ten untracked test files
(`tests/northstarWorkflow*` ×4, `mcp-workflow`, `agentScope`, `workflowGateDrawer`, three more).
**The whole fork exists only in one working tree on one laptop, with no remote to push to.**
That is a data-loss risk before it is a design question.

**0.3 `IDEA.md` is a placeholder.** It is one line: *"Replace Paper.Design."* `CLAUDE.md:1-3`
points at `@.context/INDEX.md` and says *"This is a placeholder. Will need to be updated."*
The in-repo intent documents are `PROPOSAL.md` (245 lines) and `RESEARCH.md` (166 lines), both
untracked. I did not read them in full; this survey is from source.

---

## 1. Stack and build

**One line:** a TypeScript monolith — React 18 + Vite 5 + Tailwind 4 + shadcn/Radix client,
Express 4 + `ws` + better-auth + MCP SDK server, Drizzle ORM over Postgres (embedded PGlite by
default), run with `tsx` under Bun, wrapped by a Tauri 2 shell that bundles none of it.

### 1.1 What runs

| Piece | Evidence | Notes |
|---|---|---|
| Package manager | `package.json:7` `"packageManager": "bun@1.3.10"` | `README.md:17`: Bun **and** Node. Scripts use `tsx`, `vite`, `vitest` — Node tools. Bun is the installer, not the runtime. |
| Dev | `package.json:9` — `concurrently … "tsx watch --env-file-if-exists=.env server/index.ts" "vite"` | Two processes. Web on **:4300**, API/WS/MCP on **:4400** (`vite.config.ts:10-11`, `server/index.ts:49`). |
| Build | `package.json:10` `"build": "vite build"` | **Client only.** The server is never compiled — `package.json:11` `"start": "NODE_ENV=production tsx server/index.ts"` runs TypeScript directly in production. |
| Prod serving | `server/index.ts:1591-1592` | Express serves `dist/` and falls back to `index.html`. One origin in production. |
| Vite proxy | `vite.config.ts:27-43` | In dev, `/api`, `/mcp`, `/i`, `/a/`, `/u/`, `/ingest`, `/relay`, `/.well-known` and `/ws` proxy to :4400. `changeOrigin` is deliberately off so better-auth mints OAuth URLs on the web origin (`:28-30`). |
| Server entry | `server/index.ts` (1719 lines) | Routes at `:231-1592`, `WebSocketServer({ server, path: '/ws' })` at `:1598`, `listen` at `:1706`. |
| Database | `server/db/index.ts:7-16`, `:26-44` | `DATABASE_URL` set → node-postgres; unset → **PGlite** persisted to `./data/pg`. Migrations applied **at boot** by `drizzle-orm/*/migrator` (`:32`, `:41`); `drizzle.config.ts:3-5` says drizzle-kit only *generates*. |
| Self-host | `docker-compose.yml:4-40` | App + `postgres:16-alpine`. Default secret `change-me-please-not-for-production` (`:11`). |
| Server size | `wc -l server/*.ts` = **14,409 lines** across 40 files | Largest: `index.ts` 1719, `actions.ts` 1517, `mcp.ts` 1395, `resident.ts` 1365. |

### 1.2 Its Tauri shell and ours are different kinds of thing

| | Paperdesign `desktop/` | PanaceaApp `desktop/src-tauri` |
|---|---|---|
| What the webview loads | **A remote URL.** `desktop/src-tauri/src/main.rs:22-25` `APP_URL` = `option_env!("DOOP_APP_URL")` or `http://localhost:4300`; window built at `:130-131` with `WebviewUrl::External`. `tauri.conf.json:7-8` sets both `devUrl` and `frontendDist` to `http://localhost:4300`. | **A bundled frontend.** `tauri.conf.json:10` `"frontendDist": "../dist"`, `beforeBuildCommand: "npm run build"` (`:9`). |
| Description | `Cargo.toml:4` *"a thin webview over the hosted app"*; `desktop/README.md:3` *"It does not bundle the web application."* | Our own React app. |
| Rust code | `main.rs`, 159 lines — navigation allow-list (`:113-145`), a `window.__DOOP_DESKTOP__` marker (`:126-129`), macOS traffic-light placement (`:44-86`). No commands, no IPC handlers. | `lib.rs` 14 + `main.rs` 6 lines. |
| Remote IPC | `capabilities/default.json:6-8` grants IPC to `http://localhost:4300` and `https://canvas.example.com` | `opener:default` only. |
| CSP | `tauri.conf.json:14` `"csp": null` | `tauri.conf.json:21` `"csp": null` |
| Bundle id | `design.doop.desktop` (`tauri.conf.json:5`) — kept for compatibility (`desktop/README.md:17`) | `com.au-dev-dmackie1.desktop` |

**Consequence.** There is nothing in Paperdesign's shell to merge into ours. The shell is 159
lines of window chrome; the product is the web app plus its server. Phase 5 is therefore a
choice between (a) **embedding the Northstar web app** as a webview or iframe inside our cockpit,
pointed at a running Northstar server, or (b) **porting its client components** into our
`desktop/src`. (b) is not a copy-paste: our app is React **19** and zustand **5**
(`PanaceaApp/desktop/package.json:20,22`), Vite **8** (`:34`); Paperdesign is React **18**,
zustand **4**, Vite **5** (`Paperdesign/package.json:55,62,89`), and styles through Tailwind 4 +
shadcn, which our `docs/DESIGN-SYSTEM.md` token layer does not use. Either way, **a Northstar
server process must run somewhere** — the canvas state, the MCP endpoint and the WebSocket room
all live in it (§2, §4). Where it runs — the user's microVM, the control plane, or a shared
Panacea service — is a phase-5 decision that phase 2's substrate does not make for us (§6).

## 2. The MCP surface — what an agent can actually do to a frame

### 2.1 How an agent connects

| Aspect | Evidence | What it means for us |
|---|---|---|
| Endpoint | `server/index.ts:1523` `app.all('/mcp', handleMcpRequest)` | One HTTP endpoint on the Northstar server. In dev, reached via the web port (`README.md:53` uses `http://localhost:4300/mcp`). |
| Transport | `server/mcp.ts:1381` `new StreamableHTTPServerTransport({ sessionIdGenerator: undefined })`; `:1326-1332` rejects non-POST with *"This MCP endpoint is stateless; use POST."* | **Stateless Streamable HTTP.** A fresh `McpServer` is built per request (`:1380`). No MCP session, so nothing can be pushed to the agent between its calls — human feedback is instead piggybacked onto tool results (`withFeedback`, defined at `:109`). |
| Auth | `server/mcp.ts:1336` `auth.api.getMcpSession(...)`; `:1337-1350` 401 + `WWW-Authenticate: Bearer … resource_metadata=…`; `server/auth.ts:294` `mcp({ loginPage: '/' })`; RFC 9728 metadata at `server/index.ts:1536-1554` | **MCP OAuth, issued by better-auth inside Northstar.** A human approves in a browser; the agent gets a bearer token for that human's account. Our in-microVM Claude Code agent would need that approval once per Northstar instance — an interactive browser step inside a headless sandbox. **This is a friction point for phase 5 and is not solved by the fork.** |
| Ban check | `server/mcp.ts:1352-1361` | A banned user's live MCP token is refused. |
| Identity | `server/mcp.ts:1378-1380` → `buildMcpServer(owner, session.userId)` | Every tool runs **as the approving human**. |
| Canvas gate | `server/mcp.ts:185-196` — `canvasFor` / `frameFor` run `canAccessCanvas(ownerId, c)` | Same gate as the browser. An agent sees exactly the canvases its human can. |
| Workflow gate | `server/mcp.ts:197-203` — `hasDurableCanvasAccess` | Workflow tools are owner-and-invited-members only; share-link visitors excluded. |
| Attribution | `server/mcp.ts:144-148` — `agent_name` is a free-text string the agent supplies on every call | **Display name is caller-chosen.** Two agents on one account can present as each other; neither can present as a different account. For our fleet (N agents per user) that is the only thing distinguishing them on the canvas — phase 5 must pass our session handle as `agent_name` if it wants attribution to line up with the cockpit. |

### 2.2 The 32 tools, grouped

Tool names are the literal strings at the `server.registerTool(` sites in `server/mcp.ts`.

| Group | Tools (`mcp.ts` line) | What they let an agent do |
|---|---|---|
| **Frame write** | `create_frame` :730 · `set_frame_html` :786 · `append_frame_html` :1214 · `edit_frame_html` :1252 · `update_frame` :1284 · `delete_frame` :1308 | Create a frame with a full HTML document (inline `<style>`/`<script>` allowed, `:733`); replace it; **stream it in section by section** (`:1216-1218`); exact find-and-replace; rename/move/resize; delete. |
| **Frame read / see** | `get_frame` :768 · `get_canvas` :417 · `get_frame_screenshot` :1175 · `export_frame` :813 | Read full HTML; read layout; get a **server-rendered PNG** so the agent can see its work (`:1177-1178`); get public PNG/JPG URLs that re-render on change (`:815-817`). |
| **Canvas** | `list_canvases` :293 · `create_canvas` :311 | Scoped to the human's own and invited canvases. |
| **Design memory** | `list_guidelines` :480 · `get_guidelines` :513 · `set_guidelines` :540 · `get_reference` :581 · `save_decision` :618 | Named **markdown** style guides, max 24,000 chars (`:542-543`); pinned exemplar frames; recorded human decisions. |
| **Collaboration** | `set_status` :653 · `get_comments` :674 · `get_feedback` :703 | Broadcast a status line; read element-pinned comments; claim open human feedback. |
| **Workflow (Northstar)** | `get_workflow` :325 · `request_workflow_transition` :338 · `attach_workflow_evidence` :374 | Advance the per-canvas state machine (never through a human gate, `:341`); attach `VERIFICATION` or `DEPLOYMENT` evidence (`:376-378`). |
| **Assets and outside world** | `upload_asset` :840 · `search_images` :913 · `search_icons` :973 · `search_logos` :1002 · `view_website` :1058 · `import_webpage` :1103 | Server fetches a URL or accepts an upload and returns a **public** asset URL; searches stock/icon/logo providers; renders and imports third-party pages. **Every one of these is server-side egress** (§3.4). |
| **Guide** | `get_guide` :214 · `get_style_recipe` :225 · `search_inspiration` :247 | Protocol doctrine; the agent is told to call `get_guide` once first (`:216`). |

### 2.3 What an agent cannot do

- **There is no structured design data to act on.** A frame is `{ id, canvasId, name, x, y,
  width, height, html, … }` (`shared/types.ts:1-16`) — an opaque HTML string. There is no
  component model, no token object, no layer tree. **"Tokens" exist only as the markdown body of
  a `GuidelineDoc`** (`shared/types.ts:119-131`). Searches of `server/`, `shared/` and `src/lib/`
  for a handoff/bundle/token/component-tree export found none. **The phase-5 handoff bundle
  ("tokens, component tree, screens") does not exist in the fork. Screens exist (frames);
  tokens and component tree would be new work** — either derived from HTML, or authored by the
  agent into a new structured artifact.
- **No per-task frame scope for external agents.** `server/agentScope.ts:5-15` defines
  `canMutateFrame` / `canCreateFrame`, but they are enforced only in the *resident* agent
  (`server/resident.ts:967`, `:1036`). `server/mcp.ts` does not import `agentScope` (the grep of
  `server/*.ts` for `agentScope|canMutateFrame|targetFrameIds` hits `actions.ts`, `index.ts`,
  `resident.ts` only). **An MCP-connected agent can edit or delete any frame on any canvas its
  human can reach.** With N parallel agents per user, one misdirected agent can overwrite
  another's frame. Phase 5 needs a scope the fork does not give MCP clients.
- **No optimistic concurrency on frames.** `actions.updateFrame` (`server/actions.ts:1147-1157`)
  takes a patch and writes it — no expected revision. Two agents replacing one frame's HTML is
  last-writer-wins, silently. (The workflow *does* have version checks —
  `shared/northstarWorkflow.ts:228` — frames do not.)

### 2.4 A defect found in `edit_frame_html`

`server/mcp.ts:1270`:

```ts
const frame = actions.updateFrame(frame_id, { html: f.html.replace(old_str, new_str) }, actorFrom(agent_name))!
```

`String.prototype.replace` with a **string** pattern still interprets `$`-patterns in the
replacement: `$&` inserts the match, `` $` `` and `$'` insert the text before/after it, `$$`
collapses to `$`. Frame HTML routinely contains `$` in prices and inline scripts —
`el.textContent = '$' + price` contains `$'`. An agent editing that line gets the rest of the
document spliced in. Uniqueness is checked with `split` (`:1265`), which is literal, so the check
passes and the corruption is silent. **Fix is one token (`() => new_str`).** The defect is
upstream's: the same line is at `git show HEAD:server/mcp.ts` line 1169, outside every hunk of the
local diff. **The resident agent has it too** — `server/resident.ts:1044`
`f.html.replace(input.find, input.replace)`. **Unverified by execution** — this is standard
ECMAScript semantics read from source, not a reproduced run.

## 3. Frame sandboxing — and running a user's generated code

Frame HTML runs in **two** places, not one: in every viewer's browser, and on the Northstar
server in headless Chrome. They are isolated differently and must be assessed separately.

### 3.1 In the viewer's browser

| Control | Evidence | Effect |
|---|---|---|
| Sandbox | `src/components/FrameView.tsx:603` `sandbox="allow-scripts"` | Scripts run; **no `allow-same-origin`**, so the frame gets an opaque origin — no parent DOM, no cookies, no `localStorage`, no session. Also no forms, popups, top-navigation. This is the right baseline. |
| Content delivery | `FrameView.tsx:604` `srcDoc={FRAME_BOOTSTRAP}`; `:279` posts `{ type: 'doop:html', html }` | A fixed bootstrap loads once; frame HTML arrives by `postMessage` and is **morphed into the live DOM** (`src/lib/frameRuntime.ts:1-6`, `:36-60`). Changed `<script>` elements are re-executed (`frameRuntime.ts:52-56`). |
| Parent-side message check | `FrameView.tsx:270`, `:370`, `:451` — `if (ev.source !== iframeRef.current?.contentWindow) return` | One frame cannot drive another frame's `FrameView`. Good. |
| Frame-side message check | `frameRuntime.ts:332-360` — the listener checks `ev.data.type` only | No `ev.source === parent` test. Low consequence: the only other writer into that window is the frame's own content. |
| Network from frame | No CSP in `FRAME_BOOTSTRAP` (`frameRuntime.ts:7-8` — bare `<head></head>`); Tauri `"csp": null` (`desktop/src-tauri/tauri.conf.json:14`) | **A frame's script can `fetch` any CORS-permissive endpoint, load any image/font, and beacon out.** The opaque origin stops it reading our cookies, not phoning home. |

**A defect found: a frame's own script can persist edits to itself as the viewing human.**
The `doop:edited` listener (`FrameView.tsx:448-457`) accepts any message whose `source` is the
frame's window and saves it through the human path — `patchFrameLocal` then
`api.updateFrame(frame.id, { html: ev.data.html })` (`:453-454`). It is registered for the
frame's lifetime (effect deps `[frame.id]`, `:466`), **not only while the human is in edit
mode** (`enterEdit`/`exitEdit` at `:430-445` flip a separate flag). The frame's own content runs
in that same window, so generated HTML containing
`parent.postMessage({type:'doop:edited',html:'…'},'*')` rewrites its frame, persistently, under
**whichever human is viewing** — the owner's name in the activity feed, a new design revision,
and a tripped `Approved input has changed` guard (`shared/northstarWorkflow.ts:260-262`) if a
gate had been approved. Scope is limited to that one frame. **Unverified by execution** — read
from source; no browser was run. Fix: drop `doop:edited` unless `editing` is true, and have the
bootstrap sign its messages with a per-load nonce the content cannot read.

### 3.2 On the server, in headless Chrome

| Control | Evidence | Effect |
|---|---|---|
| Who renders | `server/screenshot.ts:1-9`, `:42-55` — `puppeteer-core` launching a **system** Chrome found via `CHROME_PATH` or a fixed list (`:11-20`) | The Northstar image **must ship a Chromium**. `get_frame_screenshot`, `export_frame` and the `/i/:id.:ext` preview route (`server/index.ts:301`) depend on it. |
| JavaScript | `screenshot.ts:102` `page.setContent(frame.html, …)` with JS left **enabled** | Agent- or user-authored scripts **execute on our server**. (The *importer* disables JS for third-party pages — `server/importer.ts:516`, `:663` — frames are not given the same treatment.) |
| Isolation | `screenshot.ts:65-83` — a fresh browser context per render, torn down on close | No cookie/cache bleed between renders. |
| Chrome's own sandbox | `screenshot.ts:53` — `--no-sandbox` when `CHROME_NO_SANDBOX` is set, which the comment says containers need | In a container, the renderer runs **without Chrome's sandbox**. In a Kata microVM (phase 8) the VM is the boundary; before phase 8 it is not. |
| Network | `screenshot.ts:89-92` → `guardPublicPageRequests` (`server/publicUrl.ts:233-263`) | Blocks `ws://`/`wss://` (`:245`) and non-http(s) schemes except `about:`/`blob:`/`data:` (`:250-253`); every other request must parse as a **public** URL (`:261`) and is fetched through a pinned resolver. **It still permits `POST`/`PUT`/`DELETE` to any public host** (`:263`). So a frame script can make our server egress to the internet. Private ranges **are** blocked: `server/publicUrl.ts:17-28` puts `10/8`, `169.254/16` (which covers Azure IMDS), `172.16/12`, `192.168/16` in a `net.BlockList`, `:37-48` does the same for IPv6 including `fc00::/7`, and resolved addresses are re-checked after DNS (`:110`, `:117`). I did not read the full list or test DNS-rebinding behaviour. |

### 3.3 What this means for running a user's generated code

Phase 5's premise — *"real-HTML frames an agent can edit directly"* — is **code execution by
design**, twice. The browser half is well contained by `allow-scripts`-only, apart from §3.1's
defect and open egress. The server half is the harder one: it is untrusted JavaScript in an
unsandboxed Chrome, with public-internet egress, on whatever host runs Northstar. That host
choice (§6) is therefore a security decision, not just a deployment one. **If Northstar runs
inside the user's own microVM, the blast radius is that user**; if it runs as a shared service,
every user's frames execute in one process next to every other user's canvases, all of which sit
in one in-memory `Map` (§4.1).

### 3.4 Egress the regulated customer will ask about

Read from the manifest and tool descriptions, not traced call by call:

- `posthog-js` (`package.json:53`) plus server-side `capture(...)` on every MCP `initialize` and
  tool call (`server/mcp.ts:1368-1376`) — **product analytics to PostHog**.
- `@anthropic-ai/sdk` (`:31`) — the resident agent's server tier on one server-wide
  `ANTHROPIC_API_KEY` (`server/agentModel.ts:15-17`, `:61-67`), plus an Azure OpenAI option.
- A **ChatGPT-subscription connector** that calls `https://chatgpt.com/backend-api/codex/responses`
  presenting `originator: codex_cli_rs` (`server/openaiAgent.ts:27`, `:61`) via the Codex CLI
  device flow (`server/modelAccounts.ts:407-418`). That is a user's consumer subscription driven
  from a server by a client impersonating the Codex CLI. It is outside our locked AI-capacity
  model (per-user rotation across the user's **own Claude** subscriptions) and carries its own
  terms-of-service question. **Recommend it is disabled in any Panacea build; decider: product
  owner, with legal on the terms question.**
- Stock-image, icon, logo, inspiration and webpage fetches (§2.2), `@aws-sdk/client-s3` asset
  storage (`:32`), `nodemailer` SMTP (`:51`), GitHub App import (`server/githubApp.ts`).

## 4. Multiplayer — server, sync model, database

**Short answer: there is a server and a database, both load-bearing; there is no CRDT.
"Multiplayer" is a server-authoritative broadcast room, and the canvas cannot be opened without
it — even by one person.**

### 4.1 The sync model

| Question | Answer | Evidence |
|---|---|---|
| CRDT / OT? | **None.** No `yjs`, `automerge`, `loro` or similar in dependencies. | `package.json:30-63` (manifest read in full). |
| Source of truth | **An in-memory `Map` in one Node process.** | `server/store.ts:5-13` — *"In-memory canvas/frame state — the hot path … Every committed mutation is mirrored to the database via the write-through helpers."* Boot hydrates from the DB (`server/index.ts:66-70`, `persist.hydrate` at `server/db/persist.ts:667`). |
| Writes | REST (`PATCH /api/frames/:id`, `server/index.ts:1231`) and MCP both call `actions.*`, which mutate the store and `broadcast`. | `server/actions.ts:1126-1175`; `broadcast` at `server/index.ts:147-152`. |
| What the client sends over WS | Only ephemeral state: `join`, `cursor`, `editing`, `frame:drag`. | `shared/types.ts:287-291`; handler `server/index.ts:1670-1693`. |
| What the server sends | The whole canvas on `init`, then `frame:created`/`frame:updated` carrying **whole `Frame` objects**, plus presence, comments, tasks, workflow-adjacent memory events. | `shared/types.ts:293-334`. |
| Conflict policy | **Last writer wins, per whole frame**, silently. No revision on frame writes. | `server/actions.ts:1147-1157` (§2.3). Streaming agent writes get a "reveal" playback that rewinds on divergence (`:1165-1170`) — cosmetic, not a merge. |
| Persistence timing | Frame saves are **debounced 400 ms per frame** (`FRAME_DEBOUNCE_MS`) during streaming; flushed on shutdown. | `server/db/persist.ts:298-313`, `:638-639`. A crash mid-stream loses the tail. |
| Auth on the socket | Session cookie on the upgrade; `canAccessCanvas` on `join`; presence name taken from the session, *"not whatever the client claims"*. | `server/index.ts:1600-1628`. |

### 4.2 Is it optional?

**No, in either sense that matters.**

1. **The WebSocket is how the canvas loads.** `src/pages/CanvasPage.tsx:116` calls
   `connect(canvasId)`; the canvas arrives only in the `init` message
   (`src/lib/ws.ts:78-88` → `s.setCanvas(msg.canvas)`). No REST call in `CanvasPage` fetches it
   (the only `api.getCanvas` caller is the Home share dialog, `src/pages/Home.tsx:123`). And an
   agent's MCP edits reach the human's screen **only** through the room broadcast. For our
   product, where the whole point is watching an agent design, the room is the feature.
2. **The server is a single stateful process.** State in a process-local `Map` and rooms in a
   process-local `conns` map (`server/index.ts:137`) mean **two replicas would split a canvas
   into two diverging copies**. There is no Redis, no pub/sub, no sticky-routing hint — the grep
   of `store.ts` and `index.ts` for `redis|cluster|replicas` finds only `designLocks`, which is
   itself an in-process `Set` (`server/store.ts:13`, `:89-99`). It scales **up, not out**.

Multi-*human* collaboration is optional in the product sense — nothing forces a second person
onto a canvas. But the machinery that serves one human is the machinery that serves ten.

### 4.3 The database

| | Evidence |
|---|---|
| Engine | Postgres dialect via Drizzle; **PGlite embedded** when `DATABASE_URL` is unset, persisted to `./data/pg` (`server/db/index.ts:7-10`, `:33-43`). Real Postgres otherwise (`:28-32`). |
| Load-bearing? | **Yes.** It holds better-auth users, sessions, OAuth clients and MCP access tokens (`server/db/auth-schema.ts:6-98`, seven tables), and 22 product tables in `server/db/schema.ts:11-398` — canvases, frames, members, workflow + events, tasks, comments, guidelines + versions, memory, assets, sync keys, GitHub connections, model accounts. |
| Migrations | 13 files, `0000_init.sql` … `0012_task_targets.sql`, applied at boot. **0010–0012 are the uncommitted Northstar ones** (§0.2); upstream stops at 0009. If upstream ships its own `0010`, the journal (`server/db/migrations/meta/_journal.json`, modified locally) conflicts on rebase. |
| Our fit | PGlite-per-instance suits "one Northstar per user microVM". Real Postgres suits a shared service. The code supports both **without change** — the one genuinely pleasant finding in this section. |

### 4.4 What this means against phase 2's heartbeat rule

`CLAUDE.md`: *"Many concurrent live processes share one global heartbeat clock."* Northstar's
room pushes cursor and frame events as they happen, uncoalesced. Inside the canvas that is
correct — cursors are the product. It becomes a problem only if the cockpit tries to show
**several** canvases live at once (one per agent row). Phase 5 should show one live canvas and
summarise the rest from the phase-2 poller, never open N rooms.

## 5. The licence, precisely

**This section reports facts and names the question. It does not answer it. The decider is
legal.**

### 5.1 The project licence — verified

- `package.json:5` — `"license": "AGPL-3.0-only"`. `README.md:11` agrees: *"GNU Affero General
  Public License v3.0 only"*.
- `LICENSE:540-548` — **§13, Remote Network Interaction**: *"if you modify the Program, your
  modified version must prominently offer all users interacting with it remotely through a
  computer network … an opportunity to receive the Corresponding Source of your version by
  providing access to the Corresponding Source from a network server at no charge."*
- **The copy on disk is a modified version.** §0.2: 50 tracked files modified, 25 added. The §13
  condition *"if you modify the Program"* is met by what already exists, before phase 5 writes a
  line.
- **Both triggers are in scope**, as corrected in `3c54287`: §13 fires under our locked
  we-host model the moment a user's browser talks to our modified server; the Tauri shell is
  classic distribution (conveying object code).

### 5.2 The running app has no §13 source offer today — a fact, not a legal view

A search of `src/`, `server/`, `shared/`, `desktop/src-tauri/src/`, `index.html` and
`public/help.html` for `agpl|corresponding source|source code|github.com/kgoedecke` finds only a
prose mention in `public/help.html:48` (*"setup, upstream attribution, and AGPL obligations"*).
There is **no route, footer, About panel or MCP `get_guide` topic that offers users the
Corresponding Source**. Whether an offer is needed, where it must appear and what it must
contain are legal's questions; what is certain is that nothing in the code provides one now.

### 5.3 The CLA — what it grants, and to whom

`CLA.md:1-4`: *"Doop Individual Contributor License Agreement … maintained by Kevin Goedecke
("the Maintainer")."* It is a **contributor-to-maintainer** instrument, signed by people sending
pull requests upstream.

- `CLA.md:22-27` — contributors grant *"the Maintainer and recipients of software distributed by
  the Project"* a licence to *"sublicense and distribute your Contributions and derivative works
  under any license terms — including … commercial/proprietary licenses."*
- `CLA.md:63-66` — its stated purpose: *"lets the Maintainer offer the software under other terms
  (for example a commercial license)."*

What that means for a downstream fork, stated as facts for legal to weigh:

1. **The CLA covers *Contributions* only** — defined at `CLA.md:13-15` as work *submitted to the
   Project*. It says nothing about code the Maintainer wrote himself, which reaches us under
   AGPL-3.0-only and nothing else.
2. **The phrase "and recipients of software distributed by the Project" (`:22`) is broader than a
   maintainer-only grant.** Read literally it could reach downstream recipients for contributed
   code. I am not qualified to say whether it does, and it cannot reach point 1's code either way.
   **Flag for legal; do not rely on it.**
3. **Our own local changes are not covered at all** — we have signed nothing and contributed
   nothing (`git remote -v` shows only `upstream`).

`docs/PROGRAM.md:98` summarises the CLA as granting *"the upstream maintainer sublicensing rights
over contributions … and does nothing for us as a downstream fork."* Point 2 shows the grant text
is wider than "the maintainer" alone. The conclusion may still be right; the summary should not be
what legal reads — `CLA.md:20-27` should be.
4. **The CLA's own footnote names the route that would change the answer:** a commercial licence
   from the Maintainer, who holds CLA rights over contributions and copyright in his own code.
   Whether he holds enough to grant one for the whole codebase depends on whether every
   contributor signed — not determinable from this repo.

### 5.4 Direct dependency licences

Read from each **direct** dependency's own `package.json` `license` field
(`node_modules/<name>/package.json`, one file per name — no recursive search). Transitive
dependencies were not examined.

| Licence | Direct runtime dependencies (`package.json:31-62`) |
|---|---|
| **MIT** | `@anthropic-ai/sdk`, `@modelcontextprotocol/sdk`, all ten `@radix-ui/*`, `better-auth`, `clsx`, `express`, `nanoid`, `pg`, `react`, `react-dom`, `tailwind-merge`, `tsx`, `undici`, `ws`, `zod`, `zustand` |
| **MIT-0** | `nodemailer` |
| **Apache-2.0** | `@aws-sdk/client-s3`, `@electric-sql/pglite`, `class-variance-authority`, `drizzle-orm`, `puppeteer-core` |
| **Apache-2.0 AND MIT** | `posthog-js` |

**No direct dependency is copyleft.** The AGPL obligation comes from the project itself, not its
dependencies. Apache-2.0's NOTICE-file duty applies to redistribution of those packages. The
Tauri shell's `Cargo.toml` declares no `license` field; its crates (`tauri`,
`tauri-plugin-opener`, `objc2-app-kit`) were not checked.

### 5.5 The blocker, stated for legal

> **Question for legal.** PanaceaApp proposes to run a *modified* copy of an
> `AGPL-3.0-only` web application (Doop, by Kevin Goedecke and contributors) as a hosted service
> on Panacea's AKS, reached by KPMG staff and later by external clients, and to ship a Tauri
> desktop shell that connects to it. (1) What does §13 require of us, to whom, and does it reach
> PanaceaApp code that talks to the canvas over HTTP/MCP/WebSocket, or only the canvas server?
> (2) Does the Tauri shell change the answer? (3) Does `CLA.md:22`'s "recipients of software
> distributed by the Project" give us any additional rights? (4) Is a commercial licence from
> the Maintainer the route to proprietary use, and can he grant it for the whole codebase?

**Decider: legal.** Not engineering, not the product owner. Until it is answered, phase 5 should
be designed so the canvas server is a **separable process behind a network boundary** — the
architecture that keeps every answer open — and nothing from Paperdesign should be copied into
`desktop/src`.

**One residual of the old framing in live docs:** `docs/PROGRAM.md:127` still reads *"Paperdesign
licence — AGPL-3.0 is fine internally, blocking for sale."* "Internally" and "for sale" are the
distribution framing `3c54287` corrected at `:94-96` of the same file. It sits 33 lines below
the correction and contradicts it. **The same stale framing survives in this directory:**
`dependency-graph-and-order.md:210` — *"AGPL-3.0 clearance for Paperdesign — a legal gate on
*distribution*, not on internal build."* `3c54287`'s message names that document as repeating the
error, but the commit touched only `docs/PROGRAM.md` (`git show --stat 3c54287`: 1 file). Two
one-line fixes, owned by whoever owns those documents; this survey edits neither.

## 6. What phase 5 needs from phase 2's substrate

Taken from `survey-phase-2-substrate.md`; phase 2 was not re-surveyed. That survey pre-dates
`1db2062` (*"command palette and keyboard map — salvaged, incomplete"*), so its task-23 notes may
be stale.

### 6.1 What phase 2 already gives phase 5

| Need | Phase 2 provides | Source |
|---|---|---|
| A place to mount | The `design` arm of `LensRouter`, today a `LensPlaceholder` with `phase={5}`, which is the right number | substrate §4.1, `AppShell.tsx:34-41` |
| The recipe for filling it | Import, swap placeholder for `<DesignLens />`, anchored guard, update `AppShell.test.tsx` | substrate §4.3 |
| A fixed box to render into | `.shell-lens` is `min-block-size: 0; overflow: hidden`; the lens owns its own scroll | substrate §4.3, `shell.css:29-32` |
| Tokens | `styles/tokens.css` — seven surfaces, full token layer | substrate §7, §9 item 5 |
| One clock | `hooks/heartbeat.ts` — *"do not build a third mechanism"* | substrate §9 item 6 |
| A provenance carrier on every session | `Session.contextRef`, persisted, carried through resume and fork | substrate §6.1 |
| A typed client and problem classifier | `api/client.ts`, `api/problems.ts` — eighteen codes, three kinds | substrate §5, §9 item 4 |

### 6.2 What phase 2 does not give, and phase 5 must bring or get added

1. **A place for the Northstar server to run.** Phase 2's control plane launches agent sessions;
   nothing in the substrate survey describes a long-lived per-user or per-project *service*
   process (inferred from that survey — `control-plane/` was not read here). Phase 5 must pick
   one of three homes, each with a different owner:

   | Home | Consequence | Who owns the decision |
   |---|---|---|
   | **Inside each user's microVM**, beside the agents | Blast radius of §3's server-side JS is one user. PGlite fits. Agents reach MCP on localhost. Multi-*human* collaboration across users needs a route into the VM. Warm-pool image grows by a Chromium. | Product owner + phase 8 (Kata) |
   | **One per project, in the tenant namespace** | Real multiplayer across a team. Needs real Postgres. Needs its own ingress and SSO. | Product owner + Panacea platform |
   | **One shared service** | Cheapest to run. Every tenant's frames execute in one Chrome, every canvas in one in-memory `Map`. **Hard to defend for regulated tenants.** | Not recommended; security would decide |

2. **A push transport.** Substrate §8 item 5: *"There is no event stream … a design canvas with
   multiplayer … cannot get it from this substrate … Phase 5's Paperdesign already brings
   multiplayer, which means a second transport in the same window."* Confirmed from the other
   side: Northstar's room is load-bearing (§4.2). **Phase 5 will run a second transport.** The
   rule to hold is §4.4's: one live canvas, not N.
3. **Identity bridging.** Northstar authenticates with its own better-auth users, email/password or
   generic OIDC (`server/auth.ts:227`, `:296-305`), and mints its own MCP OAuth tokens (§2.1).
   The control-plane HTTP surface binds `127.0.0.1` with no auth until phase 9 (`docs/superpowers/plans/2026-09-18-phase-4-discover.md:6512`).
   Three identity systems meet at the canvas: the user's Panacea SSO, Northstar's own accounts, and
   the in-VM agent's MCP token. Northstar's generic OIDC could point at Keycloak realm `panacea`
   (**unverified** — not configured or tested), which would collapse the first two. The third
   — an interactive browser approval for an agent running headless in a VM — has no answer in
   the fork.
4. **An agent scope.** Phase 2 knows which session is which; Northstar sees one human with N
   agents sharing one token and a free-text `agent_name` (§2.1, §2.3). Mapping a phase-2 session to
   a set of frames it may touch needs `agentScope` extended from the resident agent to MCP clients.
   **That is a change to Northstar's server**, which the licence question (§5) governs.
5. **The lens-seam task.** `dependency-graph-and-order.md:147` recommends one 30-minute task that
   replaces all placeholder arms with stub lens files, closing `AppShell.tsx` for the programme.
   Phase 5 benefits directly: `DesignLens.tsx` becomes a file only phase 5 opens.

### 6.3 Embed or port — the decision that sets phase 5's size

| | **Embed** the Northstar client (iframe/webview inside `DesignLens`) | **Port** its components into `desktop/src` |
|---|---|---|
| Size | Small: a lens that frames a URL, plus the server-hosting work of §6.2 | Large: React 18→19, zustand 4→5, Tailwind/shadcn → our token layer, 88 client files |
| Design-system fit | **Fails `docs/DESIGN-SYSTEM.md` on sight** — a second visual language inside the cockpit — until reskinned inside the fork | Can be made to fit |
| Licence exposure | Keeps the AGPL code in its own process behind a network boundary | **Copies AGPL code into our app.** Blocked until legal answers §5.5 |
| Provenance | Must be pushed across the frame boundary (`postMessage` or URL) | Native |

**Recommendation, for the product owner:** embed first, port nothing, until legal has answered.
That is also the only choice that does not make §5.5's answer harder.

## 7. The provenance thread — what Design reads and writes

`CLAUDE.md`: *"the spec written in Define is visibly referenced in Design, carried into Develop,
and attached to the release in Deploy."* Design is the **second** rung. It must read the brief and
must write something Develop and Deploy can point at.

### 7.1 The fork already has a provenance mechanism — and it is a rival, not a helper

Northstar's workflow is a serious piece of work and it is **per canvas**:

- **A digest-bound human approval.** `canvasReviewInput` (`server/northstarWorkflow.ts:46-75`)
  canonicalises every frame (id, geometry, **full HTML**, `updatedAt`), every guideline and every
  pinned reference; `digestInput` SHA-256s it (`shared/northstarWorkflow.ts:190-192`). A human
  approving the `VISION` gate approves **that digest** at **that** `designRevision`
  (`shared/northstarWorkflow.ts:75-81`, `server/northstarWorkflow.ts:256-274`). Any later edit
  bumps `canvases.design_revision` (`server/db/schema.ts:17-20`) and invalidates the approval
  (`shared/northstarWorkflow.ts:256-262`). `reviewArtifacts` keeps the exact frame documents that
  were approved (`:79-81`).
- **An audit trail.** `canvas_workflow_events` (`server/db/migrations/0011_northstar_evidence.sql`)
  records from/to phase, actor id, actor kind, actor source and input digest for every transition.
- **Its own Define and its own Deploy.** Phases `DISCOVERING` and `DEFINING` precede the design
  review; `BUILDING`, `VERIFYING`, `DEPLOY_PENDING`, `DEPLOYED` follow it
  (`shared/northstarWorkflow.ts:1-11`). `attach_workflow_evidence` takes `buildCommit`,
  `buildImage`, `releaseTarget`, `artifact` and `rollback` (`server/mcp.ts:374-414`).

The first two are exactly what the Design rung needs. The third duplicates Discover (phase 4),
Develop/Merge (phases 2–3) and Deploy's `ShippedManifest` (phase 7). **Two spines, one in our Go
control plane and one in the canvas's Postgres, will drift**, and `CLAUDE.md` rules on that
directly: *"Any feature that breaks that thread is wrong regardless of how good it is in
isolation."*

**Recommendation, for the product owner:** in a Panacea build, keep the `VISION` gate and its
digest; **hide or disable every other Northstar phase**, so Discover owns Define and Deploy owns
release. The alternative — mapping Northstar's ten states onto our lenses — keeps two sources of
truth for the same fact.

### 7.2 What Design must read

| Reads | From | How it would reach the canvas |
|---|---|---|
| `provenance.Ref` for the branch | `GET /api/v1/discover/branches/{branchID}/provenance`, never 404, `Present:false` when absent (`survey-phase-4-discover.md` §5) | **Rendered by our chrome, outside the iframe** — the same `ContextChip` Develop uses. Phase 4's contract: a surface takes a `ProvenanceRef` *"and nothing else derived from a brief."* |
| The brief's text, for agents designing against it | Same endpoint, or the worktree's managed `CLAUDE.md` block | **Proposal, not verified:** mirror it into the canvas as a read-only `GuidelineDoc` (e.g. `name: "brief"`), overwritten on each brief revision. Guidelines are already inside the review digest (`server/northstarWorkflow.ts:61-63`), so **the `VISION` approval would bind the brief revision the design was reviewed against** — free provenance. The cost: guidelines are editable by anyone on the canvas (`set_guidelines`, `server/mcp.ts:540`), so the mirror needs a marker and a drift check, the same discipline as phase 4's `<!-- panacea:discover:begin brief=… rev=… -->` block. |

There is **no field on a Northstar canvas for an external reference** today — `canvases` is `id,
name, owner_id, link_access, design_revision, created_at, updated_at`
(`server/db/schema.ts:11-23`). Binding a canvas to a branch and a brief is new state, in one of our
stores or in theirs.

### 7.3 What Design must write

| Writes | Shape (proposed, not built) | Consumer |
|---|---|---|
| **The approved-design reference** | `{ canvasId, designRevision, reviewDigest, approvedBy, approvedAtVersion }`, taken from Northstar's `Workflow` (`shared/northstarWorkflow.ts:66-85`) | Develop — a session started "from the design" carries it, as `contextRef` carries the brief. Deploy — the manifest can say *"this release implements design rev N, digest D, approved by X."* |
| **The handoff bundle** | Screens: `reviewArtifacts` — the exact approved frame HTML (`:79-81`) plus `export_frame` PNGs. Tokens: guideline markdown (§2.3). **Component tree: does not exist**; would be derived from frame HTML or authored by an agent as a new artifact. | Develop agents, materialised into the worktree the way phase 4 materialises `CLAUDE.md`. |

**The contract question this raises, for the phase 4 owner.** Phase 4's rule is that *"if a
mounting surface needs a field, it goes into `provenance.Ref` in Task 8 — never computed
locally."* A design reference is exactly such a field. Either `provenance.Ref` grows an optional
`design` member (a Go change in phase 4's package, owned by phase 4), or phase 5 mints a sibling
type and the three read surfaces render two. **The first keeps one thread.** It should be raised
before phase 4's Task 8 merges, since `dependency-graph-and-order.md:63` already requires that
package to merge first.

### 7.4 One constraint on the carrier

`Session.contextRef` is **opaque by design** (substrate §6.2; `session.go:72-74`, *"never
interprets it"*) and holds **one** ref. A session started from an approved design needs both the
brief and the design. Two options, both the phase 4 owner's call: encode both in the one opaque
string and let Discover's resolver expand it, or add a second carrier field (a Go contract change).

## 8. Risks, ranked, with the decider for each

| # | Risk | Severity | Decider | Evidence |
|---|---|---|---|---|
| 1 | **The AGPL §13 question is unanswered, and the copy on disk is already a modified version.** Porting any Paperdesign code into `desktop/src`, or hosting the canvas for users, lands inside it. | **Blocker** for hosting and porting; not for surveying or prototyping | **Legal** | §5.1, §5.5; `package.json:5`, `LICENSE:540-548` |
| 2 | **The fork exists in one uncommitted working tree with no remote.** 75 changed entries, including all Northstar work, on one laptop. A disk failure or a stray `git checkout .` deletes it. | **High**, and cheap to fix | **Product owner** — where it is pushed. KPMG EMU blocks personal repos; an org repo under `kpmg-au-emu` is the known route (auto-memory). Pushing an AGPL fork to an *internal* repo is not a §13 event, but confirm with legal before any *public* push. | §0.2 |
| 3 | **Two product spines.** Northstar's per-canvas `DISCOVERING…DEPLOYED` workflow duplicates Discover, Develop/Merge and Deploy's manifest. Left on, the canvas and the cockpit give different answers to "where is this project?" | **High** — `CLAUDE.md` calls this class of defect wrong by definition | **Product owner** | §7.1; `shared/northstarWorkflow.ts:1-11`, `:207-216` |
| 4 | **Agent- and user-authored JavaScript runs on the canvas server** in Chrome without its sandbox in containers, with public-internet egress for any HTTP method. | **High** if the server is shared across users; **medium** if it runs per user in a microVM | **Security**, via the hosting choice in §6.2 (product owner + phase 8) | §3.2; `server/screenshot.ts:53`, `:102`; `server/publicUrl.ts:263` |
| 5 | **The canvas server scales up, not out.** In-memory state and in-process rooms mean one replica per canvas population. | **Medium** — decides the hosting shape more than it blocks | **Product owner + Panacea platform** | §4.2; `server/store.ts:5-13`, `server/index.ts:137` |

**Later, not now:**

- **No per-agent frame scope for MCP clients** (§2.3). With N agents per user, one agent can
  overwrite another's frame. Needs a Northstar server change, so it waits on risk 1. Decider:
  phase 5 implementer, once legal allows server changes.
- **Two defects, both unverified by execution, neither covered by a test** — `tests/` has no MCP
  `edit_frame_html` test and nothing references `doop:edited`:
  (a) `$`-pattern corruption in `edit_frame_html` and the resident's edit path
  (`server/mcp.ts:1270`, `server/resident.ts:1044`) — upstream's defect too;
  (b) a frame's own script can persist edits to itself as the viewing human
  (`src/components/FrameView.tsx:448-466`). Decider: whoever owns the fork; (a) is worth reporting
  upstream once legal is comfortable with us contributing.
- **The ChatGPT-subscription connector** impersonates the Codex CLI from a server
  (`server/openaiAgent.ts:27`, `:61`) and sits outside the locked AI-capacity model. Decider:
  product owner to disable; legal on the terms question.
- **PostHog analytics** on every MCP connect and tool call (`server/mcp.ts:1368-1376`). Decider:
  product owner; almost certainly off for regulated tenants.
- **MCP OAuth needs an interactive browser approval** for an agent that runs headless in a VM
  (§2.1, §6.2 item 3). Decider: phase 5 implementer, with the phase 9 auth design.
- **React 18 / zustand 4 / Vite 5 vs our React 19 / zustand 5 / Vite 8** (§1.2). Matters only if
  the product owner chooses port over embed.
- **Migration numbering collision** with upstream's future `0010` (§4.3). Matters only if the fork
  ever rebases.

---

## 9. What I did not verify

- **I ran nothing.** No `bun install`, no `bun run dev`, no `vitest`, no `tsc`, no browser. The two
  defects in §2.4 and §3.1 are read from source and standard ECMAScript/DOM semantics, not
  reproduced. Every statement about behaviour is a statement about code.
- **I did not read `PROPOSAL.md` or `RESEARCH.md`** (§0.3). They are the fork author's statement of
  intent and may explain the Northstar workflow's purpose. Reading them before the product owner
  decides risk 3 is a 15-minute task.
- **I did not read `server/resident.ts`** beyond the agentScope and edit-path lines, nor
  `server/actions.ts` beyond frame create/update, nor `src/components/` beyond `FrameView.tsx`.
  The client's 88 files were counted, not read.
- **I did not trace every egress call.** §3.4 is from the manifest and tool descriptions.
- **I did not check transitive dependency licences**, nor the Tauri crates' licences (§5.4).
- **I did not test Northstar's generic OIDC against Keycloak realm `panacea`** (§6.2 item 3). That
  it *could* work is inferred from `server/auth.ts:296-305` accepting any discovery URL.
- **I did not read `WorkflowGateDrawer.tsx`**, so I cannot say how the Northstar workflow looks to a
  user, only how it behaves in code.
- **Whether the fork's 75 local changes pass the fork's own tests is unknown.** Ten of the 42 test
  files (883 lines together, including `agentScope.test.ts`) are untracked Northstar tests.
- **`data/` was not opened.** It holds 1,114 files per the task brief — most likely the PGlite store
  at `data/pg` (`server/db/index.ts:36`). If it contains real canvases, it is also unbacked-up data
  under risk 2.
- **The phase-2 facts in §6 are the substrate survey's**, which pre-dates `1db2062`.
