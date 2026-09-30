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

## 3. Alternative 1: OpenPencil

*(to be filled)*

## 4. Alternative 2: Figma Enterprise + Figma MCP

*(to be filled)*

## 5. The corrected recommendation

*(to be filled)*

## 6. What was not verified

*(to be filled)*
