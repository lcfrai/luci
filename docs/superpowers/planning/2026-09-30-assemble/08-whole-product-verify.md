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

**Verdict: "reference" holds.** The licence line is incomplete, the AI gate has moved at least
once this year, and there is a concurrent-agent cap the report did not see. Coder could pass B6
with a template of ours, but that only duplicates what 01 already adopted.

**Licence at source** (GitHub API and raw files, read 2026-09-30):

- `coder/coder` `LICENSE`: `spdx_id: AGPL-3.0`. Holds.
- **Missed:** a second file at the repo root, `LICENSE.enterprise`, governs everything under
  `enterprise/`: *"You may not move, change, disable, or circumvent the license key
  functionality"* and *"This Enterprise Software may only be used in production, if you ... have
  agreed to ... Coder's Terms of Service ... or other agreement"*. So the paid features ship in
  the same tree under a source-available, licence-key licence. It is neither AGPL nor permissive.
- **What each forces.** AGPL-3.0: a modified copy that users reach over a network must offer them
  its source. The enterprise licence: an agreement with Coder before any production use. For a
  customer tenancy that is Coder's commercial terms either way (as 02 §3b said).

| # | Report's claim | At source | Result |
|---|---|---|---|
| 1 | AGPL-3.0 core plus commercial Premium | as above | **Holds**, with the `LICENSE.enterprise` detail |
| 2 | AI Gateway and Agent Firewall are in the Premium "AI Governance add-on" | Docs today: *"AI Gateway is part of AI Governance, which is included with a Premium license."* The v2.35.3 docs (search summary) called it *"a separate, per-user license for Premium customers that is not included with a Premium subscription"*. The pricing page today ticks "AI Gateway" and "Agent Firewall" under **AI Premium** ("Custom") | **Paywalled either way. Which tier is unclear**, and the gate moved at least once in 2026 |
| 3 | Helm on AKS; OIDC with Entra or Keycloak | Pricing page: *"SSO (OpenID Connect)"* on all three plans; *"OIDC group & role sync"* on Premium and AI Premium; *"SCIM is a Premium feature"*. The docs name Entra ID; Keycloak is plain OIDC and not named on the page read. v2.36.6 ships `ai-gateway_helm_2.36.6.tgz` | **Holds.** Sign-in through our realm works on the free tier; group and role sync is paid |
| 4 | v2.36.6 on 2026-09-18, 16,761 stars | API: `v2.36.6` published 2026-09-18T16:10Z; 16,761 stars; pushed 2026-09-30; not archived | **Holds.** Active, one company behind it |
| 5 | Tasks goes to extended support and leaves new releases at v2.37 | `/docs/ai-coder/tasks` now serves the Coder Agents page; no extended-support sentence found | **Unverified**, but consistent with Tasks being retired |
| 6 | *(not checked)* Agent cap | Pricing: *"Up to 5"* active Coder Agents on Community and Premium, *"Unlimited"* only on AI Premium; its FAQ says AI Premium *"removes the concurrent-agent cap"* | **New gate.** It binds Coder's own agent. Claude Code inside a workspace is not named by it on the page read. A fleet of Coder Agents needs AI Premium, custom price |
| 7 | *(not checked)* Isolation | Coder provisions whatever the Terraform template declares, so a Kubernetes template can set `runtimeClassName: kata-vm-isolation` | **Could pass B6 with our template.** This is our inference, not a Coder doc |

**Price basis.** Premium is *"Annually per user"*; AI Premium is *"Custom"*. No dollar figure is
published. A quote is needed before either can be priced.

**Days, for our architecture.** The report's 8-12 (02) is for Coder as a `Backend` beside
`dockerbe` and `katabe`: create a workspace from a Kata template through Coder's REST API, run
`orca serve` in it, and expose the `wss://` pairing as a workspace app. That is the same work
01 already costs for AKS Pod Sandboxing with `agent-sandbox` (about 10, 01-verify §0), plus a
product to run, patch and license. Nothing is saved.

**For the one-app ask:** Coder is only acceptable **headless**, behind our `Backend`. Its own
dashboard in front of users would be a second cockpit (B9) and exactly the "separate app" the
user rejects.

**Trade-off in one line:** the best-governed workspace product on the list, and free for SSO, but
its AI features sit in a tier whose price and name are both moving, and it saves no days over 01.

## 5. The "buy" candidates the verdict leans on, re-checked

Three candidates carry the verdict: the two whose change would reopen "buy" (owner call 5 and
risk 4) and the one self-hosted whole-SDLC product. All three were read at the vendor's own docs.

| Candidate | Report's claim | At source (2026-09-30) | Result |
|---|---|---|---|
| **Claude Code self-hosted environments** | ZDR unavailable; no Bedrock, Foundry or gateway; GitHub only; control plane and transcript Anthropic-hosted; beta, Team and Enterprise, off by default | `code.claude.com/docs/en/self-hosted-environments`: every quote matches verbatim. Also: *"A runner serves one owner at a time"*, so *"The minimum fleet size is therefore the number of owners you expect to be active at once"*; sessions *"consume your organization's Claude Code usage"*; and *"the claude.ai interface continue[s] to run on Anthropic's infrastructure"* | **Holds.** Reject stands. The reopen trigger needs two more conditions than the report names: Azure DevOps repositories, and a way to start sessions from our shell rather than claude.ai's |
| **GitHub Copilot cloud agent** | In-region inference US and EU only; Australia on the roadmap | GitHub docs, *"Copilot with data residency is currently available in the following regions: United States, European Union"*. No Australia. Enforcement adds *"a 10% increase on AI credits consumption"* | **Holds.** Reject stands; the quarterly re-check is right |
| **GitLab Duo Agent Platform, external agents** | GA on Self-Managed; runs the unmodified Claude Code; "Yes, wholly in the customer's Azure, with self-hosted models" (report §5) | `docs.gitlab.com/user/duo_agent_platform/agents/external/`: external agents are *"Not available on GitLab Duo with self-hosted models"*. On Self-Managed, adding the Claude and Codex agents *"remains experimental"*. Creating a custom external agent there is an experiment behind `ai_catalog_create_third_party_flows`, *"disabled by default"*. Availability *"is controlled by a feature flag and enabled for verified customers"*. Tier: *"Premium, Ultimate"* | **Refuted in part.** Claude Code on GitLab cannot run wholly in a customer's Azure today. With self-hosted models, the external agent is not offered. With GitLab's gateway, inference leaves our route (B4). A custom agent pointed at our LiteLLM is an experiment. Reference stands, for a weaker reason |

**Not re-fetched, and why that is safe.** Devin, Cursor, Factory, Replit, Lovable, v0, Bolt,
Kiro, Jules, Ona and Codex each run **the vendor's own agent**. That fails B5 by design, and no
fact-check of regions or gateways can reverse it. Their region and gateway details are the
report's, unverified here.

**The one-app ask does not revive a buy.** The only candidates that are one app by construction
across several phases are the prompt-to-app builders and GitLab. The builders fail B1, B2, B4
and B5. GitLab fails B7 (a third git host) and, per the row above, B4 for Claude Code. So the
ask changes **how** we assemble (section 7). It does not change **whether** we assemble.

## 6. The days and the task counts, for our architecture

*(to be filled)*

## 7. Corrected recommendation

*(to be filled)*

## 8. What could not be verified

*(to be filled)*
