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

## 3. Alternative 1: OpenPencil

*(to be filled)*

## 4. Alternative 2: Figma Enterprise + Figma MCP

*(to be filled)*

## 5. The corrected recommendation

*(to be filled)*

## 6. What was not verified

*(to be filled)*
