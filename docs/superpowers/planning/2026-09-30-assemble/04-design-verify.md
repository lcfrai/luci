# 04 — Design: an agent-editable canvas. The sceptic's verification

**Status:** IN PROGRESS (skeleton). Sections are filled one at a time and committed after each.
**Date:** 2026-09-30. **Verifies:** `04-design.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation and its two best alternatives at source:
licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run.

## 0. Verdict first

*(to be filled)*

## 1. What was checked, and how

*(to be filled)*

## 2. The recommendation: Penpot 2.18 + its built-in MCP server

### 2.1 Licence at source

- **Penpot:** MPL-2.0. `raw.githubusercontent.com/penpot/penpot/2.18.0/LICENSE` (and `develop`)
  opens *"Mozilla Public License Version 2.0"*. Holds.
- **The MCP server:** no `mcp/LICENSE` of its own (404 at tag `2.18.0`); `mcp/package.json`
  declares `"name": "@penpot/mcp"`, `"license": "MPL-2.0"`. Holds. (Oddity: at tag `2.18.0` that
  file still says `"version": "2.17.0"`, and the README says the plugin warns on a version
  mismatch. Harmless if the image sets its own version; not checked.)
- **The Helm chart:** `penpot/penpot-helm` `LICENSE` is MPL-2.0. Maintainer in `Chart.yaml`:
  *"KALEIDOS INC Sucursal en España SL"*. Holds.
- **`@penpot/plugin-types` 1.4.2:** the npm `latest` document has no `license`, `repository` or
  `author` field. The researcher's flag holds.
- **What MPL-2.0 forces here** (unchanged from the researcher): nothing for running it
  unmodified; a patched Penpot file shipped into a customer tenancy ships with its source.
- **New, and it matters:** the licence is not the only gate. Penpot sells a paid **Enterprise**
  tier for self-hosted installs, and the governance features a regulated tenant needs sit there
  (§2.4). "No per-seat licence" (04-design §5 point 4) does not hold for our use.

### 2.2 Maintenance

- **Releases** (`github.com/penpot/penpot/releases.atom`): 2.17.2 on 2026-08-27, 2.18.0 on
  2026-09-23, 2.18.1-RC1 on 09-25, 2.19.0-RC1 on 09-28. Seven 2.18 release candidates in
  two weeks. Holds.
- **Commits:** ten on `develop` between 2026-09-28 15:00Z and 09-29 21:07Z, by six authors.
- **Chart:** `helm.penpot.app/index.yaml` lists 1.10.0 (appVersion 2.18.0), created
  2026-09-23T12:16Z, and eight chart releases since 1.3.0 on 2026-06-11. Holds.
- **Images:** Docker Hub `penpotapp/mcp:2.18.0` pushed 2026-09-23 (digest `sha256:9270da9f…`),
  `penpotapp/backend:2.18.0` the same day (`sha256:b89b3fae…`). Pinning by digest is possible.
- **Stability signal the report understates:** the MCP layer is weeks old and still taking
  security fixes. *"Add MCP server integration"* is a 2.15.0 changelog line (the report dates MCP
  to 2.17; Coolify's 2.11.1 predates both). The changelog since then fixes a
  *"ReplServer binding to all interfaces (0.0.0.0) … allowing unauthenticated RCE"* (2.15),
  *"MCP tokens being usable as API access tokens"*, *"webhook endpoints allowing unauthorized
  access via creator-id fallback"* and missing *"permission checks in WebSocket subscription
  handlers"* (all 2.18.0). That is a vendor fixing things fast, and also a component that had an
  RCE four minor versions ago. Pin, canary, and read every changelog on bump.
- **Stars and backing:** 60,509 stars, 4,157 forks, 766 open issues, owner an Organization,
  last push 2026-09-29 (GitHub API). Top contributors `niwinz` 9,778 commits, `Alotor` 2,669,
  `superalex` 2,630: a staffed team, not one person. Holds.

### 2.3 The claims, checked against the vendor's own source

| # | Claim in 04-design | At source | Result |
|---|---|---|---|
| 1 | Chart 1.10.0 = appVersion 2.18.0, with the `mcp` component (HTTP 4401, WS 4402) | `Chart.yaml`, `values.yaml` `mcp:` block: `penpotapp/mcp` tag `2.18.0`, `httpPort: 4401` *"for AI clients (e.g. Claude)"*, `wsPort: 4402` | Holds |
| 2 | Unsafe defaults: `telemetryEnabled: true`, sample `apiSecretKey`, open registration + password login | All three in `values.yaml` lines 41-57. Also unlisted: default flags add `enable-mcp enable-admin-console`; `postgresql.password: "penpot"`; OIDC `scopes: "scope1 scope2"` (a placeholder) and `roles: "designer developer"` with an empty `rolesAttribute`, so role gating is off until set. The official compose at tag 2.18.0 also sets `PENPOT_TELEMETRY_ENABLED: "true"` | Holds, and wider than stated |
| 3 | "All four Penpot containers ship `readOnlyRootFilesystem: false`" | **Five**: backend, frontend, exporter, mcp and the new `adminConsole` (lines 415, 558, 693, 831, 966) | Corrected: five, see 2.4 for the fifth |
| 4 | The MCP needs an open, active tab | README at 2.18.0: *"Do not close the plugin's UI … keep the Penpot tab active … the MCP server will reject tasks until the tab wakes up"*. 2.18.0 fixes *"MCP integration hanging when the Penpot tab is backgrounded"* | Holds |
| 5 | Agents run Plugin-API code (`execute_code`, `high_level_overview`, `penpot_api_info`, `export_shape`, `import_image`) | `mcp/packages/server/src/tools/` at 2.18.0 has those plus `ImportPenpotFileTool` and five dev tools (`CljsRepl`, `ReadTaigaIssue` …) that 2.18.0 disables in multi-user mode. The plugin runs the agent's code with `new Function(...)` and hands it `penpot`, `penpotUtils` and a persistent `storage` object | Holds; see 2.5 for what `new Function` in the tab means |
| 6 | Multi-user MCP is *"under development … for testing purposes only"*; the token is *"hard-coded in the plugin"* | Both sentences still in `mcp/docs/multi-user-mode.md` at 2.18.0. But the shipped image's `CMD` is `node index.js --multi-user` (OCI config of `penpotapp/mcp:2.18.0`), and 2.16.1 added *"Show MCP key on Integrations page"*. So production runs the mode the doc calls testing-only, and the key is a per-user key Penpot issues. The doc is stale on the token | Label holds; the token worry is stale |
| 7 | Remote mode and scale-out: `PENPOT_MCP_REMOTE_MODE`, `PENPOT_MCP_REDIS_URI` | README table: *"Enable remote mode (disables file system access)"*; Redis URI *"enabling multi-instance horizontal scaling … (multi-user mode only)"* | Holds |
| 8 | `generateMarkup`/`generateStyle` give HTML and CSS | `plugin-types/index.d.ts`: `generateMarkup(shapes, { type?: 'html' \| 'svg' })`, `generateStyle(…{ type?: 'css' })`, `generateFontFaces`. No HTML import in the Plugin API (only `createShapeFromSvg`) | Holds |
| 9 | OIDC behind Keycloak; password login and registration can be off; roles gate sign-in | Config guide: `PENPOT_OIDC_BASE_URI`, `PENPOT_OIDC_ROLES`, `PENPOT_OIDC_ROLES_ATTR`, `enable-oidc-registration` (*"register and login with oidc"*), `disable-login-with-password`, `disable-registration` | Holds. This answers the report's day-1 question 3 on paper |
| 10 | RPC API with access tokens, and webhooks | Integration guide: team-level webhooks, *"All backend RPC calls labelled as `WEBHOOK` trigger webhook calls"*; no delivery guarantee stated. Both need flags the chart does not set: `enable-access-tokens`, `enable-webhooks` | Holds; add the two flags to G1 |
| 11 | Named versions for approval (G7) | RPC `create-file-snapshot`, `lock-file-snapshot` (*"Only the version creator can lock it"*); Plugin API `saveVersion(label)`, `FileVersion.pin()` | Holds. The approver's own tab must save and pin, which fits a plugin panel |
| 12 | Roster sync from the Hub (G2) | `teams.clj` has `update-team-member-role` and `delete-team-member` but no add-member; joining is by invitation (`create-team-invitations`, `get-team-invitation-token`). The MPL management API has only `/authenticate`, `/get-customer`, `/update-customer` | **Refuted as costed.** Each user must accept an invitation once, or we buy the admin console (2.4). +0.5 day |
| 13 | Tokens export in Tokens Studio format, not DTCG | Help centre: tokens *"adhere to the Design Tokens Format Module … a draft by the W3C DTCG"*; 2.17.0 *"Use `$` as DTCG token/group discriminator"* | Overstated. It is the DTCG draft; a small draft-to-2025.10 converter (G6) may still be needed |
| 14 | `penpot/penpot-mcp` archived Feb 2026 | README: *"archived on 2026-02-03"*; GitHub's `archived` flag is still false | Holds in substance |
| 15 | Fallback `zcube/penpot-mcp-server` is MIT | `package.json` says MIT, but **the repo has no LICENSE file** (GitHub `/license`: Not Found). 19 stars, last push 2026-06-24, uses an account-wide access token | **Refuted as a clean fallback** |

### 2.4 Paid tiers, and a closed component the report did not see

**Two findings the report missed. Neither kills Penpot today; both change the owner calls.**

**1. Penpot sells an Enterprise tier for self-hosted installs, and the governance sits there.**
`penpot.app/pricing/self-host` (read 2026-09-30):
- **Professional, free:** *"Unlimited users, teams, files"*, *"all design and prototyping
  features"*, community support.
- **Enterprise: "$25/user/month", "Starting at $950 / month".** It adds a centralised admin
  console, multiple organisations, advanced permissions, *"Single Sign-On (SSO) & identity
  governance"*, **"Plugins Allowlist, including Agentic AI"**, **"Audit logs"**, 180-day version
  history and 180-day deleted-file recovery. Add-ons: SOC2/HIPAA support, air-gapped deployment.
- Penpot's own pages disagree on the basis: `penpot.app/penpot-enterprise` says *"Enterprises
  pay $950 per month, regardless of team size"*. Get a quote.
- **What stays free:** generic OIDC sign-in (the env-configured provider is MPL code, 2.3 row 9),
  MCP (*"AI Workflows with any Agent/LLM"* is on both plans), webhooks, access tokens,
  multiplayer and every design feature. Our Keycloak-to-Entra path needs no purchase.
- **What a regulated customer's security review will ask for, and only Enterprise has:**
  - the plugin allowlist. The config guide documents no flag to restrict plugins, so any user
    can load any plugin from any URL, and plugin code gets `fetch` to any URL (2.5).
  - audit logs. The config guide documents no audit flag.
- **The report's money call is priced against the wrong product.** "Private Server $50k year"
  is on the *cloud* pricing page, *"For organizations that need dedicated infrastructure"*. It is
  not support for our self-host. The self-host line item is Enterprise.
- So 04-design §5 point 4, *"No per-seat licence"*, holds for a demo, not for a regulated
  tenancy.

**2. The Admin Console the chart enables by default is closed source.**
- The OCI config of `penpotapp/admin-console:2.18.0` (registry metadata, nothing pulled) says:
  - `org.opencontainers.image.licenses: NOASSERTION`
  - `org.opencontainers.image.source: https://github.com/penpot/penpot-nitrate`
  - title `penpot-nitrate`, workdir `/nitrate`
- `github.com/penpot/penpot-nitrate` returns 404 to the public.
- `penpot/penpot` at tag 2.18.0 has no source or Dockerfile for it. `docker/images/` holds only
  backend, exporter, frontend, mcp, media-processor and storybook.
- The MPL backend's client for it, `backend/src/app/nitrate.clj`, calls `admin-console-uri` with
  a shared key. It models subscriptions of type `"unlimited" "professional" "enterprise"
  "nitrate"` and runs a connectivity check with a `:licenses` field. This is the licensing and
  organisations service.
- **Chart 1.10.0 turns it on by default.** The `enable-admin-console` flag is set, and the
  changelog says *"Admin Console component … enabled by default"*. The official compose at tag
  2.18.0 runs it too.
- **Penpot's Docker guide:** *"Enabling the Admin Console is not required in this version, but it
  will be in a future release."*

**What this forces:**
- **Now:** G1 must remove the flag and disable the component. Otherwise the default install puts
  a closed image with no licence grant into our tenancy, and the licence gate (task 04) cannot
  pass it.
- **Later:** once a release makes it mandatory, there are two ways on. KPMG accepts Kaleidos's
  terms for a closed component, which is allowed because KPMG can buy it, but it costs money and
  customers will see it. Or we stay on the last release that runs without it.
- **Why it matters:** Penpot is drifting to open-core. That is a real forward licence risk, and
  the licence line "MPL-2.0" no longer describes the whole deployment.

### 2.5 Parallel agents and untrusted code: where the report's design breaks

The report treats the tab as a liveness problem (keep it awake). At source it is also an
identity, a routing and a security problem.

**1. One connected tab per user. The key is per user, not per file.**
- Issue #9923 (the 2.16.1 *"single-tab connection control"*): *"only one tab owns the MCP
  session at any time"*. *"Connect here"* in another tab disconnects the rest. The MCP key is
  checked *"at the profile level"*.
- Issue #9493 (2.17.0 limiter): a *"per-connection (not global) queue"*, with a default of 5
  concurrent tasks (`PENPOT_MCP_MAX_CONCURRENT_TASKS=5`).
- **Consequence:** every agent holding a user's key writes into whichever single file that
  user's one tab has open. G2's *"one bot user per project"* therefore serves one file at a
  time, which contradicts G5's *"one Chromium pod per active file"*.
- **Fix:** one bot identity, one MCP key and one runner per concurrently active lane file. The
  bots are Keycloak-local users (not Entra accounts, so no IT ticket), and each joins the team
  through the invitation flow (2.3 row 12).

**2. Agents that share a bot share state.** The plugin keeps one `storage` object per tab
across calls (*"stored attributes can be referenced in future calls"*, `ExecuteCodeTool.ts`).
Two agents on one bot can read, and plant functions in, each other's `storage`. One bot per lane
fixes this too.

**3. "Pins every call to the project's file" (G4) cannot be done by routing alone.** The relay
picks a *user* (the token), not a file. It must also wrap every `execute_code` in a guard
(`penpot.currentFile.id === <lane file>`, else throw), and the runner must never navigate
away. This is feasible, but it is new glue.

**4. Agent code can send the design anywhere from the browser it runs in.**
- The plugin runtime gives plugin code a `fetch` to any URL. `create-sandbox.ts` at 2.18.0:
  `safeFetch` sets `credentials: 'omit'` and blanks `Authorization`, with no URL allowlist.
- The MCP plugin runs the agent's code with `new Function(...)` inside that sandbox.
- Penpot's nginx sends no Content-Security-Policy. `nginx-security-headers.conf` sets only
  `nosniff`, `Referrer-Policy`, `Permissions-Policy` and `X-Frame-Options SAMEORIGIN`.
- **So, in the report's "usable after 3.5 days" mode** (agents working through the human's open
  tab), untrusted agent JavaScript runs:
  - in the human's signed-in browser, as the human;
  - with a route to post the file's contents anywhere that browser can reach. This bypasses the
    sandbox's egress lock completely.
  - Its edits are also recorded as the human's, which breaks provenance (agent versus person).
- **Correction:** the headless runner is not the backup plan. **For any real data it is the
  only acceptable mode.** The human-tab mode is for demos on non-sensitive files.
- **Add G10, a compensating control we own:** a CSP at our ingress with `connect-src` limited to
  Penpot's origin, the Hub and our plugin origin, and `frame-src` limited to our plugin origin.
  That both blocks exfiltration by `fetch` and acts as the plugin allowlist that the free edition
  lacks (2.4). It must not touch `script-src`, because SES and `new Function` need eval. It is
  untested whether Penpot's own UI works under it (fonts, plugin manifests), so 1 day, with a
  spike.

**5. `X-Frame-Options SAMEORIGIN` limits G3.** A top-level window works: Orca's per-worktree
Chromium, or a separate Tauri WebView window. An `<iframe>` inside our React cockpit is blocked
unless our ingress swaps the header for `frame-ancestors`. This is small, but it has to be
chosen.

**What still holds:** the runner-in-Kata idea (hardened pod, egress to Penpot only) is the right
containment, and G7's approval via the approver's own tab is sound (2.3 row 11).

### 2.6 Fit with Azure, Entra, LiteLLM and our code

| Constraint | Verdict | Evidence |
|---|---|---|
| Azure, AU, customer cloud (`OD-5`, `OD-18`, `OD-24`) | **Holds** | Self-hosted chart on AKS in Australia East. The only outbound calls we found are telemetry and the admin console's licence check, and both get switched off. Other browser-side egress (fonts, for example) is not verified. G10's CSP will show it |
| Keycloak brokering Entra (`OD-30`) | **Holds** | Generic OIDC with a roles attribute (2.3 row 9). Bots are Keycloak-local users |
| LiteLLM, zero retention (`OD-19`) | **Holds, n/a** | Penpot's MCP server calls no model. The agents call LiteLLM as they already do |
| Go control plane `Backend` | **Fits, with new work** | A runner is `Backend.Create(Spec)` with a runner image. But `Spec` (`backend.go:181`) has no network-policy or MCP-config field, and `control-plane/` has no `--mcp-config` or `allowedTools` code today. Both are G4 and G5 work, not re-use |
| Tauri/React desktop, or Orca | **Fits, as a top-level window** | Penpot sends `X-Frame-Options SAMEORIGIN` (2.5 point 5). `desktop/` embeds no external web content yet |
| Untrusted agents, Kata | **Fits, if the runner is mandatory** | 2.5 point 4 |
| Panacea as the host (second direction) | **The report's rejection holds** | Re-read, read-only: `services/panacea-api/launchdoc/LAUNCH.md:173` *"OPA Gatekeeper — writable root filesystems are rejected"*; `clients/tenant_redis.go:70` `panacea/redis:7.4-alpine`; `docs/architecture/AGENT-BUILDER-SPEC.md:113` G-05 *"GrapesJS or custom"*, P2, unbuilt. One nuance: emptyDir mounts might avoid wrapper images, if Panacea's pod spec allows them (not checked) |
| Coolify as the host | **The report's rejection holds** | `coollabsio/coolify` `templates/compose/penpot.yaml`: `penpotapp/*:2.11.1 # Released on 13 Nov 2025`, no MCP service, `enable-login-with-password` by default |

### 2.7 Are 9 days realistic for our architecture?

**No. About 13 engineer-days.** The largest error is structural (one tab per user), not effort.
Every figure is unmeasured, as in the report.

| # | Glue | Report | Verified | Why it moves |
|---|---|---|---|---|
| G1 | Deploy | 1 | **1.5** | Five components, not four. Switch the admin console off. Add `enable-access-tokens`, `enable-webhooks`, `enable-login-with-oidc`, `enable-oidc-registration`, `disable-login-with-password`, `disable-registration`. Replace the OIDC `scopes`/`rolesAttribute` placeholders and the database password. S3Proxy on Blob is unverified |
| G2 | Identity and roster | 0.5 | **1** | There is no add-member RPC, so each human accepts an invitation once. Bot identities are per lane, each with an MCP key |
| G3 | Cockpit pane | 0.5 | 0.5 | Top-level window only, or swap the header at ingress |
| G4 | MCP relay | 1.5 | **2** | Per-lane token routing, a file-id guard on `execute_code` and `export_shape`, and MCP config injection, which the control plane does not do today |
| G5 | Headless runner | 1.5 | **2.5** | One per active lane. A new workload type, with a NetworkPolicy the `Spec` cannot express yet, *"Connect here"* automation, keep-alive and idle-out. It is now mandatory, so a failed spike blocks real data rather than just degrading |
| G6 | Tokens | 0.5 | 0.5 | Smaller gap than stated (DTCG draft to 2025.10) |
| G7 | Brief binding and approval | 2 | 2 | Holds: `saveVersion` plus `pin()` in the approver's tab, and team-level webhooks |
| G8 | Handoff | 1 | 1 | Holds |
| G9 | Canary | 0.5 | **0.75** | Also asserts that the admin console is off, the flag set, and that the CSP is present |
| G10 | CSP and plugin allowlist *(new)* | — | **1** | 2.5 point 4 |
| | **Total** | **9** | **~12.75** | |

**Milestones, corrected:**
- **Demo mode**, agents through a human's tab on non-sensitive files: after G1–G4, about 5 days
  for one engineer. The report said 3.5.
- **Real data:** after G5 and G10 as well. With two engineers (A: G1, G2, G3, G4, G10;
  B: G5, then G6, G7, G8, G9) that is about day 6, and everything is done in about 7 calendar
  days.
- Still days, not weeks. It is also no longer cheaper than the baseline's own plan figure of
  ~9 days to finish phase 5 (04-design §4). Penpot wins on what arrives, not on cost (§5).

### 2.8 Verdict on Penpot

**UPHELD as the canvas, with the corrections in 2.3-2.7.** Nothing found beats it on
multiplayer, self-hosting in AU, Keycloak-to-Entra sign-in, an MPL core and a real API all
together. But three things in the report do not survive:
- "No per-seat licence": a regulated tenancy wants the Enterprise tier (2.4).
- "Usable after 3.5 days with agents through the human's tab": that is unsafe for real data
  (2.5).
- "9 days": the figure is about 13 (2.7).

The closed admin console is a forward licence risk the report did not name (2.4).

## 3. Alternative 1: OpenPencil

**Verdict: REFERENCE, the named fallback: UPHELD. One headline claim is refuted: its MCP
server is not headless.**

**Licence at source: MIT. Holds.**
- `LICENSE`: *"MIT License / Copyright (c) 2026 Danila Poyarkov and OpenPencil
  contributors"*. The GitHub API detects SPDX `MIT`.
- npm `@open-pencil/cli`, `@open-pencil/core` and `@open-pencil/mcp` 0.15.1 are all `MIT`.
- The collaboration dependencies are `trystero ^0.22.0` and `yjs ^13.6.32` (root
  `package.json`).

**Maintenance:**
- 8,668 stars, 859 forks, 67 open issues. Created 2026-02-27, last push 2026-09-30.
- Releases: v0.13.0 on 05-29, v0.14.0 on 08-11, v0.15.0 on 09-16, v0.15.1 on 09-18.
- **Bus factor one.** `dannote` has 2,549 commits; the next contributor has 117. No company was
  found.
- README: *"Active development. Usable today, with some rough edges."*

**Claims against its own docs:**

| Claim in 04-design | At source | Result |
|---|---|---|
| *"100+ MCP tools over stdio and HTTP that work headlessly on files"* | `packages/docs/programmable/mcp-server.md`: the server lets tools *"read and modify designs **through the running app**"*, and *"Make sure the desktop app is open with a document loaded."* The README's *"headlessly"* line contradicts it; the detailed doc is explicit | **Refuted.** Headless-on-files is the **CLI** (`openpencil eval design.fig -c … -w` with the Figma Plugin API, plus `export`, `import`, `lint`), not the MCP. The MCP needs the running app, which is the same class of constraint as Penpot's tab |
| HTML/CSS/Tailwind in; JSX, HTML, Storybook out | README: `openpencil import page.html --css styles.css -o page.fig`; `export -f jsx --style tailwind`, `-f html`, `-f storybook` | Holds |
| P2P WebRTC multiplayer, no server | README: *"P2P via WebRTC, no server, no account"*; `trystero` in `package.json` | Holds. Public signalling is not acceptable in a regulated tenancy unless we replace the provider |
| MCP security | Token auth *"enabled by default"*; CORS limited to `tauri://localhost`; file operations limited to `OPENPENCIL_MCP_ROOT`, *"reject symlink escapes"* | Better than stated |
| Built-in AI is bring-your-own-key | README: OpenRouter, Anthropic, OpenAI … *"or compatible endpoints"* | Holds. It can point at LiteLLM |
| Format risk | Its native save format is `.fig` (`save_file` *"to a `.fig` file"*; `packages/kiwi`, `packages/fig`), read by reverse engineering | Holds as a risk |

**Paid tiers:** none. **Azure, AU, Entra:** there is no server and no identity. The CLI runs
inside our sandbox on files in the lane's worktree, so residency is wherever the sandbox is, and
no credential ever enters the sandbox. That is structurally the best fit for untrusted,
parallel agents of anything examined.

**Days for our architecture:** the report's ~6 cannot be spent as it describes, because the MCP
is not headless. Agents would use the CLI from their shell instead (about 0.5 day in the sandbox
image), and humans would edit through the desktop app or PWA as one writer at a time over git.
That is about 6 days for a single-writer tool. Reaching Penpot's bar also needs:
- a self-hosted Yjs provider in place of trystero, behind Keycloak
- persistence and server authority
- audit

That is **10+ days**, on a pre-1.0, one-maintainer base. **Keep it as the fallback.** If Penpot's
runner spike fails, the OpenPencil CLI is the headless agent path.

## 4. Alternative 2: Figma Enterprise + Figma MCP

**Verdict: BUY, as an optional adapter only: UPHELD. Two details are corrected, and one is
unverified.**

**Licence and price at source (figma.com/pricing, read 2026-09-30, billed annually). Holds.**
- Commercial SaaS; there is no source licence to read.
- Enterprise: Full *"$90/mo + 4,250 AI credits/mo"*, Dev *"$35/mo + 500"*, Collab *"$5/mo +
  500"*.
- Organization: Full $55, Dev $25, Collab $5. The pricing page states no usage billing.

**Claims against Figma's own docs:**

| Claim in 04-design | At source | Result |
|---|---|---|
| AU hosting exists on Enterprise, with holes | help.figma.com *"Enable localized file hosting"*: regions *"EU, Australia, India and Brazil"*, *"Available on the Enterprise plan"*. Not localised: *"Comments … Plugins … File names … Fonts … File preview thumbnails … Dev Mode / Code Connect code … AI chats"*. New edits may sit *"in the US for up to 30 days"*. Whether MCP traffic is in scope is not stated | Holds, and MCP scope is a new open question |
| Only catalogued MCP clients may connect | developers.figma.com MCP intro: *"Only clients listed in the Figma MCP Catalog can connect to the Figma MCP Server."* Claude Code is catalogued | Holds. An unmodified Claude Code can connect, but only by holding the user's Figma OAuth grant inside the untrusted sandbox. Our relay is not catalogued |
| `use_figma` runs Plugin-API JS, plus `generate_figma_design` | The tools page lists `use_figma` (*"general-purpose create, edit, and inspect"*, Plugin API JavaScript), `generate_figma_design`, `create_new_file`, `upload_assets`, `generate_diagram`, `add_code_connect_map`, and generative plugin and shader tools | Holds |
| Enterprise limits of 600 calls a day and 20 a minute, *"with write tools exempt"* | Rate-limits page: Dev/Full on Enterprise *"Up to 600/day 20/min"*; View/Collab *"Up to 6/month"*. *"Exempt tools include: add_code_connect_map create_new_file whoami"*. `use_figma` is not named | **Partly holds.** If `use_figma` counts, one seat caps a busy agent at 600 calls a day. That is a fleet-scale ceiling |
| *"Agents can write only through a Full seat"* | Not stated on the MCP pages read | **Unverified** |
| *"SAML to Entra directly, not through our broker"* | help.figma.com *"Set up a custom SAML configuration"* for any IdP (entity ID, SSO URL, X.509) | **Corrected.** Keycloak can be the SAML IdP in front of Entra, so `OD-30` can be met |
| No customer-cloud option | SaaS only | Holds |

**Paid gates:** all of it is paid. AU residency and SCIM are Enterprise-only. **Maintenance:**
the most mature candidate, vendor-run. **Days:** ~3 for a second adapter behind the design seam
is plausible. The blocker is not effort: it is Figma admitting our relay to the catalogue, or
accepting a user's Figma grant in the sandbox, and neither has a date. Keep it off v1, as the
report says.

## 5. The corrected recommendation

*(to be filled)*

## 6. What was not verified

*(to be filled)*
