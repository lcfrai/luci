# 04 — Design: an agent-editable canvas. Adopt, integrate or buy instead of phase 5

Status: IN PROGRESS (skeleton written 2026-09-30; sections fill in as research lands).

Replaces, if adopted: phase 5, the canvas (31 tasks, 5 built).

## 1. The answer in one screen

_Pending._

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
| **Deploys** | Self-host: official Helm chart (`helm repo add penpot https://helm.penpot.app/`), Docker Compose, OpenShift. Parts: frontend (nginx), backend (JVM), exporter (headless Chromium), `penpot-mcp` (its own container since 2.17, proxied at `<public-uri>/mcp/stream`), PostgreSQL, Valkey/Redis, object storage (filesystem or S3). Here that means AKS in Australia East with Azure Database for PostgreSQL and a PVC for objects |
| **Enterprise fit** | **SSO:** generic OIDC (`enable-login-with-oidc`, `PENPOT_OIDC_BASE_URI=https://<keycloak>/realms/<realm>/`), so it sits behind our Keycloak broker to Entra (`OD-30`). Password login and self-registration can be switched off, and roles map from `PENPOT_OIDC_ROLES`. **Integration surface:** RPC API with access tokens, outbound webhooks (file created/updated, comment added). **Telemetry:** opt-in, off by default. **Residency:** wherever we run it, so Australia East now and the customer's own cloud later |
| **Maturity** | ~60.5k stars (GitHub page, read 2026-09-30). Backed by a company (Kaleidos). 2.18.0 is the current release, with 2.19.0 on `develop`. The MCP work is being hardened fast: the 2.16–2.18 changelog adds multi-instance scaling over Redis, a concurrency limiter, tenant-prefixed channels and a security fix (*"MCP tokens being usable as API access tokens"*). Paid support is on offer: Private Server $50,000/yr; cloud Enterprise $25/member/month (penpot.app/pricing) |
| **Gaps against us** | 1. **The MCP needs an open tab.** *"The MCP plugin runs inside your Penpot browser tab"*, and when the tab freezes, *"MCP fails fast"*. So an agent needs a live tab: either the human's Design lens or a headless runner we host. 2. The repo's multi-user doc still says *"under development … for testing purposes only"*, even though the self-host compose ships it. 3. Frames are **vector shapes with a CSS layout model, not live HTML**. HTML/CSS comes out through `generateMarkup`/`generateStyle`. HTML goes in only through a community plugin (`HTML TO DESIGN`). 4. Token export is **in Tokens Studio format, not DTCG 2025.10**; staff acknowledged this on 2026-05-05. A converter is needed. 5. The UI is Penpot's own, not our design system, which customers will see. 6. Penpot has no idea of a brief, so the provenance wiring is ours |
| **Integration** | **~8 engineer-days in total. It is usable after about 4:** deploy, SSO, embed in the lens, and agents working through the human's open tab. Breakdown in §6 |
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

## 4. Comparison table

_Pending._

## 5. Recommendation

_Pending._

## 6. What we still build (glue)

_Pending._

## 7. Owner calls (money or customer-visible)

_Pending._

## 8. What this study could not verify

_Pending._

## 9. Sources

_Pending._
