# 08 - Whole products that already do most of it. The sceptic's verification

**Status:** IN PROGRESS (relaunch after the 11:59 stop). Sections 1 and 3 written; the rest are
filled one at a time and committed after each.
**Date:** 2026-09-30. **Verifies:** `08-whole-product.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation ("buy no whole product, assemble the per-phase
picks with Orca on top and Panacea underneath") and its two best alternatives at source: licence,
the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR architecture (Go
control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure). Desk research
only: nothing installed or run.

## 0. Verdict first

*(to be filled)*

## 1. What was checked, and how

All reads on 2026-09-30. Nothing installed, built, signed up for or run.

- **The report under test:** `08-whole-product.md`, all 641 lines, in four chunks.
- **The user's own words**, relayed by the workflow harness with this run, and they outrank the
  computed task: *"make sure that even though we are pulling from a bunch of different areas, it
  still all feels cohesive and a part of the same app. I hate when it feels like they are all
  seperate apps merged into one..."* The report's owner call 2 (*"Customers see several products'
  faces ... Recommend: accept for v1"*) is tested against that sentence in section 2.
- **The sibling sceptics**, for their corrected days and findings: `01-`, `02-`, `03-`, `04-` and
  `05-*-verify.md` (complete), and `00-`, `06-` and `07-*-verify.md` (being finished in parallel
  while this was written, so their figures are as read at the time). Also `07-platform.md` §0-§3.2,
  a cluster the report does not cite.
- **Orca at source:** the blobless clone at `59b746f` (2026-09-29) in this session's scratchpad,
  read with `git ls-tree` and `git show`: `src/shared/global-settings-types.ts`,
  `src/renderer/src/lib/document-theme.ts`, `src/shared/plugins/plugin-panel-shell.ts`,
  `src/renderer/src/components/right-sidebar/plugin-panel-design-token-css.ts` and
  `use-plugin-panel-theme-revision.ts`. The question was how far Orca lets our surfaces look like
  one app.
- **Our side, read-only:** `docs/DESIGN-SYSTEM.md` (the token layer and stack).
- **OpenHands at source** (section 3): the raw `LICENSE` files of `OpenHands/OpenHands`,
  `OpenHands/enterprise` and `OpenHands/OpenHands-Cloud`; the chart README and `values.yaml`, and
  `charts/infra/values.yaml`, at `main`.

## 2. The recommendation: assemble, buy no whole product

*(to be filled)*

## 3. Alternative 1: OpenHands Agent Canvas + OpenHands Enterprise

**Verdict: the "reference" verdict holds, but for different reasons than the report gives, and it
is a weaker fallback than owner call 3 implies.** One fail the report lists is wrong (Azure
DevOps), one pass is overstated (LiteLLM), and one fail it missed is the one that matters most for
untrusted agents (isolation).

**Licence at source** (raw `LICENSE` files, read 2026-09-30):

- `OpenHands/OpenHands` (Agent Canvas): *"The MIT License (MIT) ... Copyright © 2025 OpenHands
  contributors"*. Holds.
- `OpenHands/OpenHands-Cloud` (the Helm charts) and `OpenHands/enterprise` (the server): both
  *"PolyForm Free Trial License 1.0.0"*, the second *"Copyright (c) 2026 All Hands AI"*. The
  operative clause: *"Use of the software for more than 30 days per calendar year is not allowed
  without a commercial license."* The chart README repeats it: *"This is **NOT** an open source
  license."* Holds. **What it forces:** a commercial licence by quote before day 31 of any year,
  for us and for every customer tenancy. No public price.

**The claims, checked at source** (chart `charts/openhands/README.md` and `values.yaml` at `main`;
Agent Canvas `README.md`; `openhands.dev` and `docs.openhands.dev` return HTTP 403 through the
proxy, as the report found):

| # | Report's claim | At source | Result |
|---|---|---|---|
| 1 | Runs *"OpenHands, Claude Code, Codex, Gemini, or any ACP-compatible agent"* | README line 10, verbatim; line 46: *"any agent with Agent-Client Protocol (ACP)"* | Holds. Claude Code is reached through ACP, so the session is an ACP client's chat, not the CLI's own TUI in a PTY. B5 fail holds |
| 2 | Agent Canvas is beta | README badge *"status-beta"*; image `ghcr.io/openhands/agent-canvas:1.24.0` | Holds |
| 3 | **"No Azure DevOps"**, fails B7 | `values.yaml:262-267`: an `azureDevOps:` block with `tenantId`, `organization` and an `azure-devops-app` secret; `values.yaml:31` wires `SaaSAzureDevOpsService`. OpenHands' blog (2026-07-22, per search; the page is 403 here) announces Entra sign-in, Azure Repos branches and pull requests, and Boards work items and PR comments as triggers | **Refuted.** Azure DevOps and GitHub are both supported, so B7 passes. The README simply does not document it |
| 4 | **"Takes our LiteLLM as is"** | README: *"We recommend using the provided LiteLLM instance rather than bringing your own. The provided LiteLLM instance uses an admin key for automated user management ... Our automation relies on this admin key to create and delete users automatically."* It also creates a team `openhands` unless given one | **Overstated.** Our LiteLLM works only if OpenHands holds its admin key and provisions users in it; the tested path is a second, bundled LiteLLM. Under `OD-21` that is a trust decision, not a setting |
| 5 | Sign-in through the bundled Keycloak; SAML for enterprise SSO | README: *"You'll need to set up GitHub, GitLab, and/or BitBucket as an auth provider"*, plus an `enterpriseSSO` SAML provider in **its own** Keycloak realm `allhands` | Holds. Our realm can sit in front as the SAML IdP, so `OD-30` is reachable, but that is two Keycloaks in the sign-in chain |
| 6 | *(not checked by the report)* Sandbox isolation | `charts/infra/values.yaml:126-146`: *"a privileged DaemonSet installs the Sysbox container runtime on every node and registers the sysbox RuntimeClass, giving pods with runtimeClassName: sysbox-runc VM-like (user-namespace)"* isolation; the runtime-api's `RUNTIME_CLASS` is empty by default (`values.yaml:1073`); a device plugin exposes `/dev/kvm` and `/dev/fuse` to sandboxes | **New fail: B6.** The shipped isolation is Sysbox, a shared-kernel runtime, installed by a privileged node DaemonSet. Pointing `RUNTIME_CLASS` at AKS's `kata-vm-isolation` is untested and unsupported by any doc read |
| 7 | AKS not documented | No `azure`/`aks` in the chart README; the only cloud examples are EKS/S3 and a GCS `us-central1` line; *"Ingress controller (recommended: Traefik)"* | Holds |
| 8 | Maintenance | Chart app image `enterprise-server:1.67.0`; the README's install badges read *"Replicated Unstable"* and *"Replicated Beta"*, and Replicated is *"the preferred method"* | Active. The GitHub API was rate-limited this pass, so stars and release dates are the report's |

**Days, for our architecture.** The report's 5-8 days to stand the chart up does not include
running it on AKS with Kata instead of Sysbox (a spike, and possibly a support ticket), a second
LiteLLM or an admin key into ours, or two Keycloaks. The 10-15 days as a backend under our cockpit
(02) is unchanged. **As a fallback it replaces Orca and our sandbox API together**, so it is
weeks, not days, once procurement is counted.

**Trade-off in one line:** the most complete self-hostable Develop product, and it now reaches
Azure DevOps, but it wraps Claude Code in ACP, isolates with Sysbox, and is PolyForm until bought.

## 4. Alternative 2: Coder (workspaces, AI Gateway, Agent Firewall)

*(to be filled)*

## 5. The "buy" candidates the verdict leans on, re-checked

*(to be filled)*

## 6. The days and the task counts, for our architecture

*(to be filled)*

## 7. Corrected recommendation

*(to be filled)*

## 8. What could not be verified

*(to be filled)*
