# 04 — Design: an agent-editable canvas. Adopt, integrate or buy instead of phase 5

Status: REVISING for the owner's second direction (relaunch, 2026-09-30). The first run wrote
§2–§5 and was stopped at 09:47; §1a records what the second direction changed.

Replaces, if adopted: phase 5, the canvas (31 tasks, 5 built).

## 1. The answer in one screen

_Pending (written last, after §1a and §6)._

## 1a. Revised for the owner's second direction

The second direction (Panacea is the strong default, the best fully-fledged solution wins, Coolify
and Supabase allowed, Panacea snippets allowed with attribution, Orca is the cockpit) **leaves the
canvas answer in place and changes where it runs, what it borrows, and how the lens looks.**

**1. The canvas is still Penpot.** Panacea has no design canvas. Its only mention is a
requirement line, *"G-05 | Visual design editor (drag-and-drop) | **P2** | Phase 2 — GrapesJS or
custom"* (`Panacea/docs/architecture/AGENT-BUILDER-SPEC.md:113`), and it was never built. Coolify
and Supabase have no canvas either. "Fully fledged" makes the case for Penpot stronger than
finishing the phase 5 clean-room build: Penpot already ships the multiplayer, components, grid,
prototyping, comments and version history that the plan left out (§4).

**2. Where Penpot runs, judged on merit.** Three hosts were compared. All three are permissive.

| Host | For | Against | Days to a running Penpot | Verdict |
|---|---|---|---|---|
| **Panacea project** (`docker-compose.yml` or a `components:` block, `launchdoc/LAUNCH.md:70`, `:1244`) | Gives us Trivy scans, signed images, backups, rollback, the Keycloak SSO gate, managed Postgres and cost attribution. The canvas would dogfood our Deploy step | OPA Gatekeeper *"writable root filesystems are rejected"* (`LAUNCH.md:173`), and all four Penpot containers ship `readOnlyRootFilesystem: false` (Helm chart 1.10.0 `values.yaml`). That means four wrapper images to redo on every Penpot upgrade. The redis add-on is `redis:7.4-alpine` (`clients/tenant_redis.go:70`), which is RSALv2/SSPL and restricted, so Valkey would be a hand-added component. The storage add-on is in-cluster MinIO (AGPL-3.0) everywhere except the dormant Aus East seam (`clients/managed_object_storage.go`, header). The outer SSO gate would also sit between our MCP relay and Penpot | ~2.5, plus upgrade friction | Named alternative |
| **Coolify one-click Penpot** (`coollabsio/coolify` `templates/compose/penpot.yaml`) | The template exists and uses Valkey 8.1 (BSD-3) | It is pinned to **2.11.1** (*"Released on 13 Nov 2025"*), seven minor versions behind, and has **no MCP container** (MCP arrived in 2.17). Password login is on by default. It runs on a single Docker VM, and Coolify's own OIDC login is only in a release candidate (06-deploy §3.1). It would be a second platform with no net saving | ~1, after rewriting the compose | Reject |
| **Upstream Helm chart** `penpot/penpot-helm` 1.10.0 (appVersion 2.18.0), on the AKS our control plane and sandboxes already need (01-sandboxes §1) | This is the shape its makers test. **The chart now includes the MCP server** (`mcp:` component, `penpotapp/mcp:2.18.0`, HTTP 4401 for AI clients, WebSocket 4402 for the plugin), and it installs the same way into a customer's AKS | We run Postgres, storage and backups ourselves, and that is where Panacea's snippets come in (point 3) | **~1** | **Chosen** |

**The trade-off in one line:** Panacea would give Penpot governance evidence for free, but it forces
four wrapper images per release and puts an SSO gate in the MCP path. The upstream chart runs
Penpot the way it is tested, and Panacea stays our Deploy step for the customer's app (06-deploy).

**3. The Panacea snippets we take** (`OD-13` relaxed: copy with attribution in the file header).
All paths are in `Projects/Panacea`, read, not run:

| Snippet | Source | Used for |
|---|---|---|
| Default-deny ingress, plus allow-same-namespace NetworkPolicies | `services/panacea-api/clients/tenant_ensure.go:96-126` | The Penpot namespace and the runner pods (G1, G5) |
| Pod hardening: `RunAsNonRoot`, seccomp `RuntimeDefault`, `ReadOnlyRootFilesystem`, drop `ALL` | `services/forge-orchestrator/preview/manager.go:113-206` | The headless MCP runner pod, which executes agent-written JS (G5) |
| S3Proxy on Azure Blob, with per-tenant HMAC-derived keys and the storage-account key kept server-side | `services/panacea-api/clients/managed_object_storage.go` (header) | Penpot's `s3` object backend on Blob, not a PVC or MinIO. S3Proxy is Apache-2.0, v4.1.1, 2026-09-10 (G1) |
| A per-tenant database on Azure Database for PostgreSQL Flexible Server | `services/panacea-api/clients/managed_postgres.go` (header) | Penpot's database (G1) |
| Project access groups in Keycloak (`findOrCreateGroup`, `AddProjectAccess`, `deriveRole`) | `services/panacea-api/clients/keycloak.go:174`, `:338`, `:1080` | Limiting Penpot sign-in to project groups through `PENPOT_OIDC_ROLES`, and the team roster (G2) |
| The thin-client MCP stance: *"holds **no permission logic of its own**"*; every call forwards the caller's token | `services/panacea-mcp/README.md` | The relay forwards a per-session token and lets the server enforce it (G4) |

**4. Orca as the cockpit changes the lens, not the canvas.** Penpot is a web app, so any cockpit
that can show a browser pane can host it. Orca's README describes *"a real Chromium window"* for its
Design Mode, and its docs name a *"Per-worktree browser"* (onorca.dev/docs/browser/design-mode). The
lane's Penpot file opens there. While it is open, that tab is also the live tab the Penpot MCP
needs (§3.1, gap 1).
- **Task 27** (desktop data for the Tauri lens) falls away if Orca ships.
- **Task 28** (lens chrome) becomes a small **Penpot plugin panel of our own**: the bound brief,
  *Approve this version*, and *changed after approval*. It then works the same in Orca, in the
  Tauri shell or in a plain browser tab, whichever cockpit the owner ships.
- **Orca's Design Mode** hands an agent a clicked element's HTML, computed CSS, a cropped
  screenshot and, where a source map exists, the source file and line. That covers the "point at
  the running app" idea in Develop, so React Grab (§3.14) and Onlook's editing of the running app
  (§3.5) are not needed.

**5. Supabase adds nothing to the canvas.** Penpot needs plain PostgreSQL and Valkey. The one
candidate Supabase would unblock is Onlook, whose self-host needs Supabase. Onlook still fails
without it: dev containers run on CodeSandbox, it is locked to Next.js and Tailwind, and it has no
MCP. So it stays Reference. If the Hub moves to Supabase (the Define and Hub clusters decide that),
the glue writes its bindings there instead, and the days do not change.

**6. Corrections found in this pass.** The Helm chart's defaults are unsafe for a regulated
tenancy. It ships `telemetryEnabled: true`, a sample `apiSecretKey` in plain text, and the flags
`enable-registration enable-login-with-password disable-email-verification`.
- §3.1 said telemetry was *"opt-in, off by default"*. That is true of Penpot's own config, but not
  of the chart. G1 overrides all three settings.
- The MCP server is now part of the chart, so it needs no separate install.
- The repo's MCP multi-user page still says the pairing token *"is hard-coded in the plugin's
  source code for testing purposes"* (`penpot/penpot` `mcp/docs/multi-user-mode.md`). The
  2.16–2.18 changelog, however, fixes *"MCP tokens being usable as API access tokens"*, which
  implies that Penpot now issues these tokens itself. The day-1 spike settles which is true (§8).

**7. The days do not move: ~9 in total, usable after ~3.5 (§6).** The snippets save perhaps half
a day across G1 and G5. The plugin panel costs about what the lens chrome did. Both figures are
unmeasured. Sections §3.1, §3.5, §3.14, §4 and §5 below are revised in place to match; anything
not mentioned here stands as the first run wrote it.

## 2. What the capability has to do

The bar comes from the phase 5 plan's own goal
(`.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md:28-62`)
and the owner's words in `OD-11`: *"pretty much exactly like paper.design with the MCP"*. Eight
tests, used for every candidate below:

| # | Test | Why it matters here |
|---|---|---|
| T1 | **Agents write through MCP, headless and in parallel** — an unmodified Claude Code session in a sandbox can create and change frames with no human tab open | Plan goal 1: agents design "while the Design lens is open, or while it is closed". A fleet is parallel by definition |
| T2 | **Humans finish the work by direct manipulation, multiplayer** — select, move, restyle, undo, with several people and agents in one file | Plan goals 2 and 3 |
| T3 | **Fidelity to code** — frames are real HTML/CSS or real components, not a drawing that someone re-types | `OD-11`: "real-HTML frames". It is why paper.design felt powerful |
| T4 | **Export into the codebase** — HTML, JSX, tokens (DTCG), PNG per screen into the worktree | Plan goal 4: the handoff bundle a Develop session starts from |
| T5 | **Runs on Azure in our environment, later in a customer's cloud, with an AU option** | `OD-5`, `OD-18`, `OD-19`, `OD-24` |
| T6 | **Signs in through Keycloak → Entra, with an audit trail** | `OD-30` |
| T7 | **Licence we can ship into a customer tenancy** | MIT/Apache/BSD/ISC/MPL-2.0 fine; AGPL/GPL/SSPL/BUSL/fair-code/Elastic restricted; commercial fine if KPMG can buy it |
| T8 | **Provenance hooks** — an API to bind a file to a brief, snapshot it at approval, and pull the approved revision | The product spine: brief → design → develop → release. Nothing off the shelf does this; the question is only how cheap the glue is |

**The untrusted-agent rule applies here too.** A design MCP that needs a *user's* credential inside
the sandbox (a Figma or Penpot token) hands an untrusted agent everything that user can see. The
answer in every "adopt" or "buy" case is the same: the credential stays in the control plane and
the sandbox reaches the design tool only through our MCP relay (phase 5 tasks 18–19, already
planned), which pins the call to the project's own file.

## 3. Candidates

Glasses (local catalogue and the lcfr.ai MCP, both snapshot `2026-09-29T04:25Z`) holds exactly one
whole product for this capability: **Onlook**. Searches for penpot, excalidraw, tldraw, grapesjs,
plasmic, webstudio, whiteboard, "design canvas mcp" and "figma alternative open source" returned
nothing. Every other candidate below came from the web. Licences are from the project's own
`LICENSE`, README or npm `license` field, read 2026-09-30, unless marked.

### 3.1 Penpot + its official MCP server — ADOPT (the recommendation)

| | |
|---|---|
| **What it does** | Open-source design and prototyping tool: boards, components and variants, CSS flex and grid layout, prototyping, comments, version history, native design tokens, plugins, real-time multiplayer. The **official MCP server** (in `penpot/penpot` under `mcp/` since the separate repo was archived in Feb 2026) lets an agent run Plugin-API code in an open file. Tools: `execute_code`, `high_level_overview`, `penpot_api_info`, `export_shape`, `import_image`. The Plugin API's `generateMarkup({type:'html'})` and `generateStyle({type:'css'})` turn shapes into HTML and CSS |
| **Licence** | **MPL-2.0** (repo page; the Helm chart repo is MPL-2.0 too). Allowed. File-level copyleft only: running it unmodified carries no source duty, and hosting it as SaaS is not distribution at all. If we patch a Penpot file and ship it into a customer tenancy, that one file's source goes with it. Our glue is separate code and stays ours |
| **Deploys** | Self-host: official Helm chart (`helm repo add penpot https://helm.penpot.app/`), Docker Compose, OpenShift. Parts: frontend (nginx), backend (JVM), exporter (headless Chromium), `penpot-mcp` (its own container since 2.17, proxied at `<public-uri>/mcp/stream`), PostgreSQL, Valkey/Redis, object storage (filesystem or S3). *Revised:* here that means the upstream Helm chart 1.10.0 (appVersion 2.18.0, with the `mcp` component built in) on our AKS in Australia East, backed by Azure Database for PostgreSQL Flexible Server, with objects on Azure Blob through S3Proxy (Panacea's pattern). A PVC is the fallback. §1a weighs Panacea and Coolify as hosts |
| **Enterprise fit** | **SSO:** generic OIDC (`enable-login-with-oidc`, `PENPOT_OIDC_BASE_URI=https://<keycloak>/realms/<realm>/`), so it sits behind our Keycloak broker to Entra (`OD-30`). Password login and self-registration can be switched off, and roles map from `PENPOT_OIDC_ROLES`. **Integration surface:** RPC API with access tokens, outbound webhooks (file created/updated, comment added). **Telemetry:** off by default in Penpot's own config, but **on in the Helm chart** (`telemetryEnabled: true`, corrected in §1a), and G1 turns it off. **Residency:** wherever we run it, so Australia East now and the customer's own cloud later |
| **Maturity** | ~60.5k stars (GitHub page, read 2026-09-30). Backed by a company (Kaleidos). 2.18.0 is the current release, with 2.19.0 on `develop`. The MCP work is being hardened fast: the 2.16–2.18 changelog adds multi-instance scaling over Redis, a concurrency limiter, tenant-prefixed channels and a security fix (*"MCP tokens being usable as API access tokens"*). Paid support is on offer: Private Server $50,000/yr; cloud Enterprise $25/member/month (penpot.app/pricing) |
| **Gaps against us** | 1. **The MCP needs an open tab.** *"The MCP plugin runs inside your Penpot browser tab"*, and when the tab freezes, *"MCP fails fast"*. So an agent needs a live tab: either the human's Design lens or a headless runner we host. 2. The repo's multi-user doc still says *"under development … for testing purposes only"*, even though the self-host compose ships it. 3. Frames are **vector shapes with a CSS layout model, not live HTML**. HTML/CSS comes out through `generateMarkup`/`generateStyle`. HTML goes in only through a community plugin (`HTML TO DESIGN`). 4. Token export is **in Tokens Studio format, not DTCG 2025.10**; staff acknowledged this on 2026-05-05. A converter is needed. 5. The UI is Penpot's own, not our design system, which customers will see. 6. Penpot has no idea of a brief, so the provenance wiring is ours |
| **Integration** | **~9 engineer-days in total. It is usable after about 3.5:** deploy, SSO, embed in the lens, and agents working through the human's open tab. Breakdown in §6 |
| **Verdict** | **Adopt** |

### 3.2 OpenPencil — REFERENCE, the named fallback

| | |
|---|---|
| **What it does** | MIT design editor that opens `.fig` and `.pen` files. Built on Skia CanvasKit, Yoga (flex plus grid), Vue 3 and **Tauri v2**, the same desktop shell as ours. It has **100+ MCP tools over stdio and HTTP that work headlessly on files**. It imports HTML, CSS and Tailwind (`openpencil import page.html --css …`) and exports JSX/Tailwind, HTML and Storybook stories. It runs Figma Plugin API scripts via `eval`, and supports components, variants, token extraction and lint. Multiplayer is peer-to-peer over WebRTC plus Yjs |
| **Licence** | MIT (README; npm `@open-pencil/cli`, `@open-pencil/core`, `@open-pencil/mcp`, all 0.15.1) |
| **Deploys** | There is no server: a desktop app, a PWA and a CLI/MCP that runs in any container. An agent in our sandbox edits a design file inside its own worktree, and git carries it |
| **Enterprise fit** | No accounts, no SSO, no audit and no server-side authority. Multiplayer relies on public WebRTC signalling (Trystero), with no TURN setup for a corporate network. The built-in AI is bring-your-own-key to public providers, so it would have to be pointed at LiteLLM or switched off |
| **Maturity** | ~8.7k stars (GitHub page). First npm publish was 2026-03-01; 0.15.1 shipped 2026-09-18. Pre-1.0: *"Usable today, with some rough edges"*. The copyright holder is one named person plus contributors, and no company backing was found. It reads and writes Figma's proprietary `.fig` by reverse engineering, so any Figma release could break that |
| **Gaps** | T2 (no enterprise-grade multiplayer), T6 (no identity or audit), and maturity |
| **Integration** | ~6 days to embed its Vue SDK in the lens and run its MCP headless in sandboxes. Multiplayer and authority would still be ours to build |
| **Verdict** | **Reference.** It has the best agent ergonomics found (headless, HTML in and out, files in git) and the weakest enterprise shell. Revisit at 1.0, or sooner if Penpot fails the day-1 runner spike (§6) |

### 3.3 Figma Enterprise + the Figma MCP server — BUY, as an optional adapter only

| | |
|---|---|
| **What it does** | The industry-standard design tool, multiplayer included. Its remote MCP server has 18 read tools and 11 write tools. The write tools include `use_figma` (*"General-purpose create, edit, and inspect"*, running Plugin API JavaScript) and `generate_figma_design`, which captures a running UI as editable layers (the 2026-02-17 "Claude Code to Figma" launch). Code Connect maps frames to production components, and `get_design_context` returns code |
| **Licence / price basis** | Commercial SaaS, priced per seat per month and billed annually. Enterprise: Full $90 (includes 4,250 AI credits), Dev $35, Collab $5 (figma.com/pricing, read 2026-09-30). **Agents can only write through a Full seat**; a Dev seat is read-only. MCP limits per Enterprise seat: 600 calls a day and 20 a minute, with write tools exempt. Secondary sources say `use_figma` is *free in beta, planned to become usage-billed* |
| **Deploys** | SaaS only. Australian data hosting exists on Enterprise **but with holes**: comments, plugins, file names, fonts, thumbnails, Dev Mode/Code Connect code and AI chats stay outside the region. Edits may sit in the US for up to 30 days, and beta features and Weave tools are out of scope (help.figma.com, "Enable localized file hosting") |
| **Enterprise fit** | SAML SSO direct to Entra (not through our broker), SCIM, and Governance+ with IP allowlist and network restrictions. But it cannot run in a customer's cloud, and each customer must buy seats. **Only clients in Figma's MCP catalogue may connect.** Our relay is not in that catalogue, so keeping the credential in the control plane needs Figma's agreement. The alternative puts a user's account-wide Figma token inside an untrusted sandbox |
| **Maturity** | The most mature candidate |
| **Gaps** | T5 (SaaS, with gaps in residency), T6 (bypasses our broker; catalogue rule), T7 (per-seat cost, with usage billing coming), T3 (vector, not HTML) |
| **Integration** | ~3 days as a second adapter behind the same design seam (link to a brief, snapshot via the REST images endpoint, handoff), **once the catalogue question has an answer** |
| **Verdict** | **Buy, but only as an optional adapter** for customers who already own Figma. Not the default |

### 3.4 paper.design — REFERENCE (the owner's benchmark)

- **Does:** a canvas that is real HTML/CSS (*"Paper is built on HTML and CSS, which LLMs already
  understand well"*). MCP can read and write, and frames export to React/Tailwind. A secondary review
  counts 24 MCP tools.
- **Licence / price:** commercial SaaS. Free (100 MCP calls/week), Pro $20/editor/month ($16 annual,
  1M MCP calls/week), and Organizations at custom pricing with *"SAML / SSO"* and admin controls
  (paper.design/pricing, read 2026-09-30).
- **Deploys:** SaaS only. **The MCP server runs inside Paper Desktop on the user's machine**
  (*"install and open Paper Desktop. That starts the MCP server"*, stdio via `paper mcp`), so it
  cannot serve an agent in our cloud sandbox.
- **Enterprise fit:** the pricing page says nothing about residency, self-hosting, SCIM or audit.
- **Verdict: Reference.** Keep it as the bar for the Design lens. Note that its MCP also needs a
  running app on the user's side, which is the same shape as Penpot's tab-bound MCP. The owner's
  reference product lives with the constraint we are accepting.

### 3.5 Onlook — REFERENCE (a Develop-phase idea, not a Design canvas)

- **Does:** Apache-2.0 "Cursor for designers". It edits a running **Next.js + Tailwind** app on an
  infinite canvas and writes changes straight back to code. It has real-time collaborative editing,
  design branching and its own AI chat.
- **Licence:** Apache-2.0 (Glasses record; the 2026-09-28 study read `LICENSE.md`).
- **Deploys:** self-hosts via Docker Compose, but needs Supabase, an OpenRouter key and a
  **CodeSandbox token for its dev containers** (docs.onlook.com/self-hosting/docker-compose). In
  other words, customer code runs on CodeSandbox's cloud unless we write a provider (its
  `code-provider` package ships `codesandbox` and `nodefs`).
- **Maturity:** 26.8k stars (Glasses, 2026-09-29), YC W25. Its own checklist shows
  `[ ] MCP (Model Context Protocol) setup and usage` still open, and no support for projects that
  are not Next.js or not Tailwind.
- **Gaps:** it is its own agent rather than a surface our fleet drives (no MCP), it locks in one
  framework, and it documents no SSO.
- **Verdict: Reference.** It is the right idea for *editing the running app* in Develop. Adopting
  it would take ~10+ days (a sandbox provider on our control plane, Supabase, SSO, the gateway).
- *Revised (§1a):* Supabase is now allowed, so that one objection falls away. The others remain:
  CodeSandbox dev containers, Next.js and Tailwind only, and no MCP. Orca's Design Mode already
  covers clicking an element of the running app and handing it to an agent.

### 3.6 pen.dev (formerly Pencil) — REJECT

- **Does:** agentic infinite canvas. `.pen` is an open JSON schema kept in git. Offers MCP for
  Claude Code, a headless CLI, and HTML/CSS/Tailwind export.
- **Licence:** proprietary. npm `@pen.dev/cli` 0.3.9 is `UNLICENSED`, under an EULA.
- **Deploys:** desktop app, IDE extension or CLI. **The CLI needs `pen login` and a connection to
  `api.pen.dev`** (docs.pencil.dev/for-developers/pen-cli).
- **Enterprise fit:** free today, *"may introduce paid features or plans later"*. No SSO,
  residency or enterprise terms found.
- **Verdict: Reject.** It phones home and has no enterprise terms. OpenPencil (MIT) can open
  `.pen` files, which keeps the idea within reach.

### 3.7 Builder.io Fusion — REJECT

- **Does:** a SaaS visual editor plus agent that works on your real repo (GitHub, GitLab, Bitbucket,
  Azure DevOps) and opens PRs. Builder Code MCP lives at `mcp.builder.io`.
- **Licence / price:** commercial. Pro $24/user/month, Team $40, Enterprise custom (Azure DevOps
  Enterprise, SSO, "Privacy mode"), per builder.io/pricing. SOC 2 Type II.
- **Deploys:** SaaS. It runs your code in its own containers; private link reaches firewalled Git.
  No residency region is stated.
- **Verdict: Reject.** It is a competing Develop phase, not a canvas, and customer code leaves
  the tenancy.

### 3.8 Plasmic — REJECT (AGPL studio)

- **Does:** a visual builder whose canvas ("arenas") renders **real React code components**, with
  multiplayer, branching and codegen into your repo. On paper, a strong fit for "production
  components as frames".
- **Licence:** split. README: *"All content outside of `platform/` is licensed under the MIT
  license"* and *"`platform/` is licensed under the AGPL"*. The Studio, the only part that is the
  product, is AGPL. SaaS is commercial with SAML SSO.
- **What AGPL would force:** self-hosting the Studio means any modified version must offer its
  complete source to every user who reaches it over the network (§13). Shipping it into a customer
  tenancy conveys AGPL code. Only the MIT SDKs are usable.
- **Verdict: Reject.** The SaaS path puts customer designs outside the tenancy (region not
  verified).

### 3.9 tldraw SDK — REJECT (buyable, but it does not remove the work)

- **Does:** an infinite-canvas SDK for React (npm `tldraw` 5.4.2, 2026-09-10). It has an "agent
  starter kit" in which a model creates, updates and deletes shapes from inside the browser using
  direct provider keys. tldraw also mentions an MCP App that embeds a canvas in a chat. No MCP server
  for outside agents is documented. HTML exists only as custom shapes we would write.
- **Licence / price:** commercial (`SEE LICENSE IN LICENSE.md`). Production needs a licence key, on
  *"value-based pricing"* through sales, with a 100-day trial (tldraw.dev/pricing). The licence lets
  the software send *"usage data to tldraw for license compliance purposes"* and detect deployment
  environments. You may not *"disable, change, or interfere with"* key enforcement.
- **What buying changes:** `OD-11` rejected tldraw because of its commercial licence. Under
  today's direction that alone would no longer rule it out. It stays rejected because it would
  replace only the viewport (phase 5 task 25), which MIT Moveable/React Flow already cover. It would
  also put phone-home key enforcement inside a regulated customer's tenancy.

### 3.10 Excalidraw (+ MCP) — ADOPT for sketch mode only, optional

- MIT; `@excalidraw/excalidraw` 0.18.1 (2026-04-20). MCP options: Excalidraw's own "Excalidraw MCP
  App" (a chat widget) and the community `mcp-excalidraw-server` 2.0.0 (MIT, 2026-08-19, 26 tools,
  live canvas sync).
- Scenes are shapes, not HTML. Good for Define-phase whiteboarding and `OD-11`'s sketch mode, about
  1 day. **It is not the canvas.**

### 3.11 GrapesJS (+ Studio SDK) — REFERENCE

- The core is BSD-3-Clause (npm `grapesjs` 0.23.6, 2026-08-25): an HTML-backed component tree
  rendered in an iframe. It is a **page builder, not a multi-artboard infinite canvas**. Core has no
  MCP and no multiplayer.
- The Studio SDK is commercial (`@grapesjs/studio-sdk` 1.2.1, `SEE LICENSE IN LICENSE.md`).
  Secondary sources give session-based pricing: free up to 1,000 sessions a month, Startup
  $200/month, Business $2,000/month.
- Canvas, MCP and multiplayer would still all be ours to build, so this is a build, not an
  adoption. The 2026-09-28 study already took its component-API shape as a reference.

### 3.12 Webstudio — REJECT (AGPL; not opened)

- `LICENSE` is the GNU AGPL v3, and `@webstudio-is/sdk` 0.303.0 (2026-09-29) is
  `AGPL-3.0-or-later`. It is a visual builder that works on real HTML/CSS, the closest functional
  match to "real-HTML frames".
- **What AGPL would force:** any modified version we host must offer its complete source to every
  user who uses it over the network. Shipping it into a customer tenancy conveys it under AGPL. Glue
  linked into it risks becoming part of the covered work. Its source was not opened, under the
  clean-room rule.

### 3.13 Prompt-to-app generators: v0, Lovable, Figma Make, Google Stitch, bolt.diy — REJECT as the canvas

- These are SaaS generators that compete with our **Develop** phase rather than giving us a Design
  canvas. Enterprise tiers exist: search summaries credit v0 with SAML SSO, audit logs and
  "data residency controls", and Lovable with SAML/OIDC including Azure AD, SCIM and EU/US/APAC
  regions. None of these was verified at a vendor page in this run.
- None is a multi-artboard canvas that our fleet drives over MCP, and none runs in a customer's
  tenancy. (bolt.diy is MIT, but it runs on StackBlitz WebContainers, whose commercial terms were
  not checked here.)

### 3.14 Element pickers for the running app: stagewise, React Grab

- **stagewise** is `AGPL-3.0-only` (npm `stagewise` 0.12.0 and `@stagewise/toolbar` 0.6.3), so it is
  restricted.
- **React Grab** is MIT (0.2.0, 2026-08-16). You pick an element in a running React app and it
  hands the element's source location to an agent. That is a one-day Develop-phase addition, not a
  canvas.
- *Revised (§1a):* if Orca ships as the cockpit, its built-in Design Mode does this (HTML,
  computed CSS, a cropped screenshot, and the source file and line where there is a source map),
  so React Grab is not needed.

### 3.15 Panacea, Coolify and Supabase as canvas candidates (added for the second direction)

- **Panacea:** no canvas. The only mention is an unbuilt P2 requirement, *"Visual design editor
  (drag-and-drop) … GrapesJS or custom"* (`AGENT-BUILDER-SPEC.md:113`). What it contributes here
  is snippets, and it stays the Deploy step (§1a points 2 and 3).
- **Coolify:** no canvas. It has a one-click Penpot template, but that is stale at 2.11.1 and
  lacks MCP. Rejected as the host (§1a point 2).
- **Supabase:** no canvas. Postgres, auth and realtime, none of which Penpot needs (§1a point 5).

## 4. Comparison table

Scored against §2's tests. "Part" means it works with a stated limitation. Days are integration
estimates for one engineer who knows this repo. Every figure here is unmeasured.

| Candidate | T1 agents headless, parallel | T2 human multiplayer | T3 fidelity to code | T4 export to repo | T5 Azure / AU / customer cloud | T6 Keycloak→Entra, audit | T7 licence | T8 provenance hooks | Days | Verdict |
|---|---|---|---|---|---|---|---|---|---|---|
| **Penpot + MCP** | Part: needs a live tab (lens or our runner) | **Yes** | Part: CSS flex/grid model, HTML/CSS via `generateMarkup`/`generateStyle` | Yes: PNG/SVG exporter, HTML/CSS, tokens (Tokens Studio format) | **Yes**: self-host on AKS | **Yes**: OIDC; webhooks | MPL-2.0 (allowed) | Yes: RPC API, webhooks, access tokens | **~9** (usable ~3.5) | **Adopt** |
| OpenPencil | **Yes**: headless MCP on files | Part: P2P WebRTC, no server | Yes: HTML/Tailwind in and out | Yes: JSX, HTML, Storybook | Yes: no server needed | **No**: no accounts or audit | MIT | Part: files in git only | ~6 + multiplayer | Reference / fallback |
| Figma + MCP | Yes (execution model not verified) | **Yes** | Part: vector; Code Connect | Yes: `get_design_context` | Part: SaaS, AU hosting with gaps; no customer cloud | Part: SAML direct, not our broker; MCP catalogue rule | Commercial, per seat | Yes: REST, webhooks | ~3 after catalogue answer | Buy, optional adapter |
| paper.design | No: MCP in the user's desktop app | Yes | **Yes**: real HTML/CSS | Yes: React/Tailwind | No: SaaS, no residency stated | Part: SAML on the Org plan | Commercial | No API found | — | Reference |
| Onlook | No: its own agent; MCP not built | Yes | **Yes**: the running app itself | Yes: writes code | Part: self-host, but CodeSandbox for dev containers | No SSO documented | Apache-2.0 | Part | ~10+ | Reference (Develop idea) |
| pen.dev | Yes: headless CLI | No real-time found | Yes | Yes | No: login to `api.pen.dev` | No | Proprietary | Part: files in git | — | Reject |
| Builder.io Fusion | Its own agent | Yes | Yes: real repo | Yes: PRs | No: SaaS, code runs in its containers | Part: SSO | Commercial | Part | — | Reject |
| Plasmic | Not documented | Yes | Yes: real React components | Yes: codegen | Studio self-host is AGPL | Part: SAML (SaaS) | **AGPL** studio | Part | — | Reject |
| tldraw SDK | Part: in-browser agent kit | Yes (tldraw sync) | No: shapes | No | Part: self-host with licence key | n/a | Commercial + telemetry | n/a | ~10+ (a build) | Reject |
| Excalidraw | Yes: community MCP | Yes | No: shapes | No | Yes | n/a | MIT | n/a | ~1 | Sketch mode only |
| GrapesJS | No MCP | No (core) | Yes: HTML | Yes | Yes | n/a | BSD-3 / commercial SDK | n/a | a build | Reference |
| Webstudio | — | — | Yes | Yes | — | — | **AGPL** | — | — | Reject |
| Panacea / Coolify / Supabase *(added, §3.15)* | None has a canvas | — | — | — | Hosts only: Panacea (governance for free, wrapper images), Coolify (stale Penpot 2.11.1 template, no MCP) | — | Ours / Apache-2.0 / Apache-2.0 | — | — | Not canvases: Panacea is a snippet source and the Deploy step |
| *Baseline: finish phase 5 as planned* | Yes: Go authority, 37 MCP tools | **Part: cross-person co-editing is v2**, outside the plan | **Yes**: real HTML | Yes | Yes | Yes | Ours | **Yes**, designed in | ~71 planned h left (~9 days); 35.5 h critical path | — |

**The honest comparison with the baseline.** On the plan's own figures, finishing phase 5 costs
about as many engineer-days as adopting Penpot: 85 h in total, about 14 h built (tasks 01–06),
about 71 h left. The difference is **what arrives and how much risk is left**. The plan's
"Out of this plan" list (`…-design-canvas.md:59-62`) leaves out cross-person live co-editing,
components with slots, CSS Grid, `margin`, theme modes, the pen tool, SVG/PDF export and custom
fonts. Penpot ships all of those today. The plan's riskiest remaining work is the editor (tasks
23–26 and 29), which the 2026-09-28 study calls *"the classic two weeks that became six"*, and
that is exactly the part Penpot replaces. The plan also has a record of fix rounds: task 04, estimated at 4 h, went
through two numbered fix rounds after review (`feat/phase-5-canvas` log).

## 5. Recommendation

**Adopt Penpot 2.18.x (MPL-2.0) as the Design phase's canvas.** *Revised for the second
direction (§1a):*
- Install it with the upstream Helm chart 1.10.0, which has its MCP server built in, on the AKS in
  Australia East that our control plane and sandboxes run on.
- Open it in the cockpit's browser pane: Orca's per-worktree Chromium, or the Tauri WebView if
  Orca does not ship.
- Send every agent's MCP traffic through our relay.
- Keep the brief binding, approval and handoff as our own code. Borrow Panacea's snippets for
  network policy, pod hardening, Blob storage and Postgres.
- Panacea stays the Deploy step for the app that the design turns into.

**Integration: ~9 engineer-days in total. It is usable after ~3.5:** Penpot deployed, SSO through
Keycloak, open in the cockpit, and agents designing through the human's open tab. With two
streams in parallel, that is about 5 calendar days. (§3.1 and §4 said ~8; §6's line-by-line
breakdown gives 9, and 9 is the figure to use.)

**Why this one**

1. **It is the only candidate that passes T2, T5, T6, T7 and T8 together.** It self-hosts in
   Australia or in a customer's own cloud, signs in through our Keycloak broker to Entra, has a
   licence we can deploy, has a real API and webhooks, and has mature multiplayer between people.
2. **Agents already design in it over MCP by writing Plugin-API code.** That is the same model as
   Figma's `use_figma`. Paper, the owner's reference, also needs a running app on the user's side
   before its MCP answers, so the Penpot tab constraint is the benchmark's constraint too.
3. **It removes the riskiest part of phase 5**, the document engine and the editor. It also ships
   what phase 5 had left out: co-editing between people, components and variants, grid,
   prototyping, comments, version history and native tokens.
4. **No per-seat licence.** Paid support is optional: Private Server at $50,000/yr list.

**What it replaces in our plan.** Phase 5 has 31 tasks and 85 planned hours
(`.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md:440-474`).

| Fate | Tasks | Planned hours |
|---|---|---|
| **Replaced by Penpot** | 05, 06, 07, 08, 09, 10, 11, 12, 13, 14, 15 (except `get_brief`), 16, 17 (except the server instructions), 23, 24, 25, 26, 29 — **18 tasks**. 05 and 06 are already built and become sunk cost | **~50.5 h**, of which ~45.5 h is unbuilt |
| **Kept, re-pointed at Penpot** (they become the glue) | 18, 19 (relay), 20a, 20b (routes and wiring, smaller), 21 (approval), 22 (handoff), 27 (desktop data: dropped if Orca ships, small if the cockpit is Tauri), 28 (lens chrome, which becomes our Penpot plugin panel, §1a point 4), 30 (end to end) — **9 tasks** | ~25.5 h at plan rates; §6 costs the glue at 9 days, because it also carries the deploy, SSO and runner work that the plan never had |
| **Built, kept** | 01 (the WebView2 spike, kept as knowledge), 02, 03 (thin homes for the glue), 04 (the licence gate, needed unchanged) — **4 tasks** | ~9 h, already spent |

It also **supersedes `OD-34`**, the canvas's own op-log and authority, because Penpot's backend
becomes the authority. And it turns `OD-11`'s clean-room build into an adoption (see §7).

**Risks and their answers**

| Risk | Answer |
|---|---|
| The MCP only works while a Penpot tab is open and connected | Day 1 spike of the headless runner (§6 G5). If it fails, run agents only while the lens is open, as Paper does, and in the fallback order try either the community RPC-based MCP (`zcube/penpot-mcp-server`, MIT, 19 stars, needs no tab) or OpenPencil (§3.2) |
| Upstream still labels multi-user MCP *"under development"* | Pin 2.18.x by image digest. Add a canary test that creates a board over MCP on every upgrade |
| Frames are not live HTML | Fidelity to code comes from `generateMarkup`/`generateStyle` plus the Develop agent. Anyone expecting paper.design's live HTML will notice the difference (§7, owner call 3) |
| An untrusted agent's `execute_code` runs arbitrary JS | It runs only in the runner's Chromium, as a bot user limited to one project, in a pod whose only egress is Penpot. It never runs in the user's own tab when the user is away, and the sandbox never holds a key |
| Tokens export in Tokens Studio format | A small converter to DTCG 2025.10 plus `tokens.css` in G6 |
| A Clojure/JVM codebase we cannot easily patch | Treat it as a black box and take only upstream fixes. Any local patch to an MPL file ships with its source |
| Relicensing upstream | MPL-2.0 today, and a relicence is not retroactive. Pin the version and let the licence gate record it |
| Licence traps in its dependencies | Use **Valkey (BSD-3)**, not Redis ≥ 7.4 (RSALv2/SSPL, restricted). That rules out Panacea's `redis:7.4-alpine` add-on too. Store objects on Azure Blob through **S3Proxy (Apache-2.0)**, or on a PVC, and **not in MinIO (AGPL-3.0)**, which is Panacea's in-cluster default |
| The chart's defaults are unsafe (§1a point 6): telemetry on, a sample `apiSecretKey`, open registration and password login | G1 overrides all of them, and the key comes from the vault. The upgrade canary asserts the flags every time |
| Orca's embedded browser may throttle or freeze a background Penpot tab (not verified), which Penpot warns makes MCP *"reject tasks"* | The lens tab serves only the human who is looking at it. Agents that work unattended go through the runner (G5) |

## 6. What we still build (glue)

Nine pieces, **9 engineer-days**. G1–G4 make the canvas usable, with agents designing through a
human's open tab, after **3.5 days**. Days are for one engineer who knows this repo, and all of
them are unmeasured.

| # | What we build | Phase 5 task it re-points | Days | Panacea snippet (§1a pt 3) |
|---|---|---|---|---|
| **G1** | **Deploy.** Upstream chart 1.10.0 and images pinned by digest. Azure PostgreSQL Flexible Server, Valkey 8 (BSD-3), Blob through S3Proxy. Flags set for OIDC sign-in only, telemetry off, `apiSecretKey` from the vault. MCP runs with `PENPOT_MCP_REMOTE_MODE=true` (*"disables file system access"*) and multi-instance over Valkey (`PENPOT_MCP_REDIS_URI`). Default-deny NetworkPolicy | new (the plan's canvas ran in-process) | 1 | `tenant_ensure.go`, `managed_postgres.go`, `managed_object_storage.go` |
| **G2** | **Identity and roster.** A `penpot` OIDC client in the Keycloak realm that brokers Entra (`OD-30`). `PENPOT_OIDC_ROLES` limits sign-in to project groups. One Penpot team per PanaceaApp project, with its roster synced from the Hub, and one bot user per project for the runner | new | 0.5 | `keycloak.go` groups |
| **G3** | **Cockpit pane.** A deep link to the lane's file in Orca's per-worktree browser, or in the Tauri WebView. Silent SSO. The MCP plugin is connected when the file opens | 27 (dropped or smaller) | 0.5 | — |
| **G4** | **MCP relay.** The sandbox's `--mcp-config` names only the relay. The relay holds the Penpot user token (the human's, or the runner bot's) and adds it to `/mcp?userToken=…`, so the sandbox never sees it. It pins every call to the project's file, adds our `get_brief` tool, and writes each call to the audit trail. `--allowedTools` limits the agent to what the relay exposes | 18, 19; 15's `get_brief`; 17's server instructions | 1.5 | `panacea-mcp` thin-client stance |
| **G5** | **Headless runner.** One Chromium pod (Playwright) per active file, signed in as the project's bot user, with the file open, the MCP plugin connected and the tab kept active. Its only egress is Penpot, and it idles out. **Day 1 is a spike**: if it fails, agents work only while a human tab is open (Paper's model), and §5's fallbacks apply | new | 1.5 | `forge-orchestrator/preview/manager.go` hardening, NetworkPolicy |
| **G6** | **Tokens.** Tokens Studio JSON to DTCG 2025.10 `tokens.json` and `tokens.css`, with a golden-file test | the token half of 09 | 0.5 | — |
| **G7** | **Brief binding and approval.** A Hub record binds a Penpot file and a named version to a brief revision. The *file updated* webhook raises *changed after approval*. *Approve* saves a named version, exports a PNG per board and runs `generateMarkup`/`generateStyle` per board, then stores the bundle in the Hub under the brief revision. The approval panel is our own Penpot plugin; a Hub page beside the canvas is the fallback | 20a, 20b, 21, 28 | 2 | — |
| **G8** | **Handoff.** The approved bundle goes into `design/` in the lane's worktree: manifest, HTML/CSS, PNG and tokens, plus the session link. Manifest paths are validated and no link is followed (the plan's review rules C2 and I1 carry over) | 22 | 1 | — |
| **G9** | **End to end and canary.** On every upgrade, create a board over MCP and read it back, and assert the flags. The contract document. The licence gate (task 04, unchanged) records Penpot (MPL-2.0), Valkey (BSD-3) and S3Proxy (Apache-2.0) | 30 | 0.5 | — |
| | **Total** | | **9** | |

**Order and calendar.** Two streams:
- **A:** G1, G2, G3, G4, then G9 (4 days).
- **B:** starts on day 1 against A's Penpot: G5's spike first, then G6, G7 and G8 (5 days).

That is **about 5–6 calendar days with two engineers**, with the usable milestone at the end of
A's day 3.5.

**What we no longer build**, because Penpot has it:
- the document model, op log and authority (tasks 05–12, and `OD-34`)
- the HTML dialect and canonical forms (07, 08)
- the render bridge, renderer and editor (13, 23–26)
- paste (29) and most of the MCP tool layer (14–17)
- cross-person multiplayer, comments, version history, components with variants, CSS grid, the
  pen tool, SVG/PDF export and custom fonts

The plan left the last group out of v1 altogether (`…-design-canvas.md:59-62`).

## 7. Owner calls (money or customer-visible)

The technical choices are ours, and none of them is listed here. That includes the host (§1a), the
relay and the runner.

| # | Call | Why it is the owner's | Recommendation |
|---|---|---|---|
| 1 | **Withdraw `OD-11`'s clean-room build and `OD-34`'s op-log in favour of adopting Penpot (MPL-2.0).** `OD-11` rule 3 reads *"Dependencies are MIT, Apache-2.0, BSD or ISC only"*, and MPL-2.0 is not on that list. Today's constraints do list MPL-2.0 as allowed. Rule 1, never opening AGPL or GPL canvas source, is unaffected | It reverses a recorded owner decision, and it decides what customers get as their canvas | **Yes.** Penpot runs unmodified as its own service, and none of our code links it. MPL's file-level copyleft would bite only if we patched a Penpot file and shipped it into a customer tenancy (§3.1) |
| 2 | **Customers see Penpot's own UI and name in the Design phase**, not our design system. Rebranding means patching MPL files, and MPL-2.0 *"does not grant any rights in the trademarks, service marks, or logos"* (§2.3) | Customer-visible | **Accept for v1**, shown as "Design, powered by Penpot". Our own surface is the plugin panel (G7) and the cockpit around it |
| 3 | **Frames are vectors with a CSS flex/grid layout, not paper.design's live HTML** (T3) | Customer-visible, and it is the owner's own benchmark | **Accept.** HTML/CSS arrives at handoff through `generateMarkup`/`generateStyle`, and the Develop agent turns that into components. Revisit OpenPencil (§3.2) at its 1.0 |
| 4 | **Paid support: Penpot Private Server at US$50,000 a year list** (penpot.app/pricing). The Azure running cost of Penpot and its runner is **not priced** here; it rides on 01-sandboxes' environment budget | Money | **Run unsupported in our own environment now.** Decide before the first install in a customer's tenancy, where a regulated customer will ask who supports the canvas |
| 5 | **A Figma adapter for customers who already own Figma.** It needs Full seats for agents to write (Enterprise, US$90 a seat a month), and Figma's agreement to admit our relay to its MCP catalogue (§3.3) | Money, the customer's or ours, and customer-visible | **Not in v1.** Build it (~3 days) on the first customer who asks |

## 8. What this study could not verify

Nothing was installed, built or run, here or in `Projects/Panacea`. Every figure for days is an
estimate. Prices are list prices read on 2026-09-30.

**The day-1 spike settles these, in this order:**
1. Whether a headless Chromium tab keeps the MCP plugin connected for hours (G5). Upstream warns
   that when a tab is frozen or suspended, MCP *"will reject tasks"*.
2. How 2.18 issues the MCP `userToken`. The repo's multi-user page says it is *"hard-coded in the
   plugin's source code for testing purposes"*, while the changelog implies Penpot issues the
   token. Also whether the plugin connects without a pasted token.
3. Which flags give OIDC-only sign-in, with password login and open registration off, while still
   creating an account on a user's first SSO login.
4. Whether the RPC API can add a team member without an e-mailed invitation (G2's roster sync).
5. Whether Penpot's `s3` backend works against S3Proxy on Blob. If not, fall back to a PVC.

**Later, before first ship:**
1. How faithful `generateMarkup`/`generateStyle` output is on real screens. Also the webhook
   payloads, their delivery guarantees, and the API for named versions (G7).
2. Orca's embedded browser: OIDC sign-in, whether cookies persist, and background throttling. Its
   docs are silent on all three. Also whether a Penpot plugin iframe can hold a Keycloak session
   under third-party-cookie rules (G7's fallback exists for exactly this).
3. `@penpot/plugin-types` 1.4.2, which our plugin would compile against, **declares no licence on
   npm**. The licence gate needs a recorded exception for this compile-time-only package, or we
   write the few types we need ourselves.
4. The Panacea-project host was judged from Panacea's docs and source (`LAUNCH.md:173`, the chart
   values), not from a deploy. Figma's MCP catalogue rule and its planned usage billing come from
   secondary sources (§3.3).
5. Whether running agent-written JS in the runner's Chromium sandbox is enough isolation for a
   regulated customer, or whether the runner should sit in the Kata pool beside the agent
   sandboxes (01-sandboxes).

## 9. Sources

All were read on 2026-09-30 unless marked otherwise. Glasses snapshot: `2026-09-29T04:25Z`.

**Read in the relaunch (second direction)**
- Penpot release: `api.github.com/repos/penpot/penpot/releases/latest`, which gives 2.18.0,
  published 2026-09-23.
- Penpot Helm chart: `raw.githubusercontent.com/penpot/penpot-helm/main/charts/penpot/values.yaml`
  and `Chart.yaml` (chart 1.10.0, appVersion 2.18.0; `mcp:` component; `telemetryEnabled: true`;
  `readOnlyRootFilesystem: false`).
- Penpot MCP: `raw.githubusercontent.com/penpot/penpot/develop/mcp/README.md` and
  `mcp/docs/multi-user-mode.md`.
- Coolify's Penpot template: `raw.githubusercontent.com/coollabsio/coolify/v4.x/templates/compose/penpot.yaml`.
- S3Proxy: `api.github.com/repos/gaul/s3proxy` and its latest release (Apache-2.0, 4.1.1,
  2026-09-10).
- Orca: `raw.githubusercontent.com/stablyai/orca/main/README.md` and
  `onorca.dev/docs/browser/design-mode`.
- `registry.npmjs.org/@penpot/plugin-types/latest` (1.4.2, no `license` field).

**Panacea, read-only** (`Projects/Panacea` working tree, 2026-09-30):
- `docs/architecture/AGENT-BUILDER-SPEC.md:113`
- `services/panacea-api/launchdoc/LAUNCH.md:70`, `:173`, `:1244`
- `services/panacea-api/routes/routes.go:1286`, `:1319-1320` (deploy, database and storage routes)
- `services/panacea-api/clients/tenant_redis.go:70`, `managed_object_storage.go`,
  `managed_postgres.go`, `tenant_ensure.go:96-126` and `keycloak.go:174`, `:338`, `:1080`
- `services/forge-orchestrator/preview/manager.go:113-206`
- `services/panacea-mcp/README.md`

**This repo**
- `.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md:28-62`,
  `:440-474`
- `docs/superpowers/planning/2026-09-23-owner-decisions.md`: `OD-11` (lines 22 and 84-99),
  `OD-13`, `OD-30` and `OD-34` (line 223)
- The sibling studies `01-sandboxes.md` §1, `02-fleet-cockpit.md` and `06-deploy.md` §3.1
- `2026-09-28-oss-adoption/00-summary.md`

**Read in the first run** (§3; the dates and versions are given inline there)
- Penpot:
  - `github.com/penpot/penpot`: the README, `mcp/` and the 2.16–2.18 changelog
  - `helm.penpot.app`
  - help.penpot.app, OIDC configuration
  - `penpot.app/pricing`
  - `zcube/penpot-mcp-server`
- Figma: `figma.com/pricing` and help.figma.com, *"Enable localized file hosting"*.
- The other candidates' own pages:
  - `paper.design/pricing`
  - `docs.onlook.com/self-hosting/docker-compose`
  - `docs.pencil.dev/for-developers/pen-cli`
  - `builder.io/pricing`
  - `tldraw.dev/pricing`
- Licence files: the Plasmic README, Webstudio's `LICENSE`, and the OpenPencil README.
- npm registry entries:
  - `tldraw` 5.4.2
  - `@excalidraw/excalidraw` 0.18.1 and `mcp-excalidraw-server` 2.0.0
  - `grapesjs` 0.23.6 and `@grapesjs/studio-sdk` 1.2.1
  - `@webstudio-is/sdk` 0.303.0
  - `stagewise` 0.12.0 and `@stagewise/toolbar` 0.6.3
  - `react-grab` 0.2.0
  - `@open-pencil/*` 0.15.1
  - `@pen.dev/cli` 0.3.9
- Secondary search summaries: v0, Lovable and the GrapesJS Studio pricing. These are marked as
  unverified in §3.
