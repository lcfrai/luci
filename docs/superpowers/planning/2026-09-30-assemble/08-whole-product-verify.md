# 08 - Whole products that already do most of it. The sceptic's verification

**Status:** COMPLETE (2026-09-30). Relaunched after the 11:59 stop; section 3 is the first run's,
kept as written.
**Date:** 2026-09-30. **Verifies:** `08-whole-product.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation ("buy no whole product, assemble the per-phase
picks with Orca on top and Panacea underneath") and its two best alternatives at source: licence,
the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR architecture (Go
control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure). Desk research
only: nothing installed or run.

## 0. Verdict first

**UPHELD at the core, CORRECTED in four places.** "Assemble, and buy no whole product" survives at
source. Four things do not: v1 as "several faces", the Hub as "built", Panacea as "the platform",
and 59 days.

- **Why the core survives:** no candidate passes. Anthropic's self-hosted environments: no ZDR,
  no gateway, GitHub only, and the control plane stays Anthropic's (verbatim at source). Copilot
  residency: the US and the EU only. GitLab's external agents: not offered with self-hosted
  models, and experimental on Self-Managed. OpenHands Enterprise: PolyForm Free Trial, Sysbox
  isolation, Claude Code behind ACP. Coder: AGPL-3.0 plus a source-available `enterprise/`, with
  its AI tier moving. The rest run their own agent (B5).
- **What does not survive:**
  1. **"Customers see several faces in v1" (owner call 2).** The user asked for the opposite. The
     premise that one app takes weeks is wrong too. Six rules make it one app: one window and
     name, one sign-in, one look, one spine, one vocabulary, all proven by the journey test. The
     whole bar costs about 9 engineer-days across three clusters. Most of it is 02-verify's thin
     build of Orca under our name, which is cheap because Orca runs on the same Tailwind and
     shadcn stack as our design system. This cluster adds Penpot in our frame and one page
     template: about 2, plus 0.5 in the journey test (7.1-7.2).
  2. **The Hub is a schema, not a service.** Its routes and client are about 10 engineer-days
     (07-platform). The gateway, about 8.4 days (07-verify §5), was costed by no cluster the
     report summed.
  3. **Panacea is the deploy step and the sign-in, not "the platform".** v1 means the dev estate,
     KPMG-network pilots and GitHub previews only. The Hub, the gateway and the environment are
     ours (06-verify, 00-verify, 07-platform).
  4. **59 engineer-days becomes about 103 (95-114).** The first app takes about 10-12 working days
     with five builders, or about a month with two. Day 0 can decide the date (section 6).
- **Alternatives:** OpenHands Enterprise stays reference, and it is a weaker fallback than the
  report says. Its "no Azure DevOps" is wrong (ADO is supported). "Takes our LiteLLM as is" is
  overstated. It isolates with Sysbox (section 3). Coder stays reference, and only headless
  behind our `Backend` (section 4).
- **Owner calls:** "several faces" is withdrawn. What replaces it is the desktop's name and icon,
  plus code signing for our build (02-verify calls 5 and 6). New: staffing, about five builders
  if "ASAP" is the bar (money).

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
  while this was written, so their figures are as read at the time). 02-verify's relaunched §7-§8
  (the one-app check on Orca, read at source) and 06-verify's §6 (Deploy under the one-app bar)
  landed during this pass. Both were read, and sections 2, 6 and 7 follow them. Also `07-platform.md` §0-§3.2
  and its §5 day table, a cluster the report does not cite.
- **Vendor docs and the GitHub API** (sections 4-5): Coder's licence files, releases, docs and
  pricing page; Anthropic's self-hosted environments page; GitHub's Copilot data-residency page;
  GitLab's external-agents page. URLs are in section 9.
- **Orca at source:** the blobless clone at `59b746f` (2026-09-29) in this session's scratchpad,
  read with `git ls-tree` and `git show`: `src/shared/global-settings-types.ts`,
  `src/renderer/src/lib/document-theme.ts`, `src/shared/plugins/plugin-panel-shell.ts`,
  `src/renderer/src/components/right-sidebar/plugin-panel-design-token-css.ts`,
  `use-plugin-panel-theme-revision.ts`, `src/renderer/src/assets/main.css`, `package.json` and
  `components.json`; and the file list of the browser pane (`src/renderer/src/components/browser-pane/`,
  `src/main/browser/`). The question was how far Orca lets our surfaces look like one app.
- **Our side, read-only:** `docs/DESIGN-SYSTEM.md` (the token layer and stack).
- **OpenHands at source** (section 3): the raw `LICENSE` files of `OpenHands/OpenHands`,
  `OpenHands/enterprise` and `OpenHands/OpenHands-Cloud`; the chart README and `values.yaml`, and
  `charts/infra/values.yaml`, at `main`.

## 2. The recommendation: assemble, buy no whole product

**Verdict: the core holds; four load-bearing parts do not.** "Assemble, buy none" survives every
check. What fails is how the assembly is presented to users, what "the Hub" is today, how much
of the platform Panacea covers, and the days.

| # | Report's claim | Test | Result |
|---|---|---|---|
| 1 | No whole product passes B1-B10 | Sections 3-5 at source; the rest fail B5 by design | **Holds** |
| 2 | Owner call 2 and risk 2: *"Customers see several products' faces ... accept for v1"*, because *"One UI of our own over all of them is weeks of build"* | The user's own words (section 1) reject it. The premise is also wrong. The one-app bar does not need one UI over every product. It needs six rules applied to surfaces the clusters already build (section 7). At Orca's source, plugin panels are handed Orca's live design tokens (`PANEL_DESIGN_TOKEN_ALLOWLIST`: `--background`, `--card`, `--primary`, `--accent`, `--border`, `--ring`, `--radius` and others) *"so panel documents can match the app"*. Both Orca and our design system are shadcn/ui on Tailwind with Geist | **Refuted.** It is a v1 requirement, about 9 engineer-days across 02, 06 and this cluster, 2.5 of them here (7.2). Not weeks |
| 3 | The provenance thread lives in *"the Hub (built; 03)"* (report 8.3) | 07-platform §1 and 3.1: Hub tasks 01-07b are merged (schema, row-level security, the hash chain). The server routes (08, 13-27) and the client (28-42) are unwritten: about 81 plan-hours, about 10 engineer-days | **Refuted.** The thread's store is a schema, not yet a service. Journey steps 2, 5 and 7 all wait on it |
| 4 | The gateway is *"a dependency, not a given"*, costed by no cluster | 07-platform §0: our LiteLLM plus `keysmith` and `gatekeeper`, **8.2 engineer-days** (8.4 in 07-verify §5). Panacea's LiteLLM cannot serve agents: no AU route, spend per project only, in-cluster | **Holds as a warning, now costed.** The total leaves it out |
| 5 | Panacea "underneath" as the platform: deploy, the app's backend, sign-in, teams | 06-verify §2.2-2.4: evidence export, sanitised previews and `panacea-mcp` exist on the dev line only. Production is frozen mid-incident, and P-1 to P-3 queue behind the cutover freeze. The edge admits 14 KPMG `/32`s, so a shipped app opens only from KPMG's network. ADO pushes go to Forge, not to previews. 00-verify §2 adds: Keycloak 25.0.0 on one replica, a year-long owner-level `pnc_`, and a preview that shares the production database by default. 07-platform: the Hub, the gateway and the environment are ours, not Panacea's | **Holds for deploy and sign-in**, on the dev estate, for KPMG-network pilots. "The platform" overstates it |
| 6 | Orca on top | 02-verify §0: upheld, with landing moved to the laptop CLI, an `OD-8` ruling or our OSC patch, and about 15.5 days, 5 of them to ship it as our app | **Holds, as our thin build**, not under Orca's name |
| 7 | 59 engineer-days; first app in about 9 working days with two builders; all glue in 12-13 | Section 6 | **Refuted.** About 103 engineer-days. The first app takes about 10-12 working days with five builders, about a month with two |

**What Orca's source says about one look.** Orca's defaults are shadcn's neutral greys
(`main.css`: `--background: #fff`, `--primary: #171717`; dark `--background: #0a0a0a`), Geist,
and `--radius: 0.625rem`. Its only app-theme setting is `theme: 'system' | 'dark' | 'light'`, plus
`appFontFamily` and custom terminal themes (`global-settings-types.ts:80-146`). There is no
custom-CSS or accent setting. So **upstream Orca cannot wear our locked palette** (warm graphite,
teal `#2fb3bd`). Its browser pane has its own address bar
(`browser-pane/assemble-chrome/BrowserAddressBar.tsx`), so a Hub page in an Orca tab looks like a
website inside a code tool. 02-verify §7 read further and found three more blockers: the name is
in the build, a plugin cannot add a top-level view, and a lane's tab routes through the sandbox.
**Its fix holds, and this study adopts it:** a thin build of the pinned Orca under our name, with
one CSS file mapping our tokens onto Orca's shadcn variables, and a journey bar hosting Hub pages
in one app-level view. The shared stack is what keeps that build small.

**What the ask leaves for the owner.** Owner call 2 is answered by the user: not accepted. What
remains is the desktop's name and icon, and code signing (7.4, call 2).

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

**59 engineer-days becomes about 103 (95 to 114).** The report sums the clusters' own figures.
Each sibling sceptic has since corrected its cluster, and two whole layers were never in the sum.
Estimates composed from estimates; nothing measured.

| Layer | Report | Corrected | Source of the correction |
|---|---|---|---|
| Develop cockpit (02) | 7.5 | **15.5** (14-18): 10.5 of corrections plus 5 to ship Orca as our app | 02-verify §0 and §8, complete |
| Sandboxes (01) | 8 | **10** (8-13), counted from day 0 | 01-verify §0, complete |
| Define (03) | 8 | **10** stream-days | 03-verify, complete |
| Design (04) | 9 | **13** | 04-verify, complete |
| Work and team (05) | 17 | **20** (17-23) | 05-verify, complete |
| Deploy (06) | 6 | **8.5** (7.5-9.5), including 1.4 to bring Portal screens in-app for the one-app ask | 06-verify §2.5, being finished |
| Platform glue (00) | 2 | **2**, a floor | 00-verify's days are still pending; its §2 adds unsized glue (the preview shares the production database by default) |
| Hub, gateway, environment (07) | 0 | **20.5** (Hub 10, signed-in Hub pages 1, gateway 8.4, environment 1) | 07-verify §5, which corrects 07-platform's 19.2. Its +1 for signed-in Hub pages overlaps 02-verify's 0.5 in the Hub |
| Journey test (this cluster) | 1.5 | **2**, with the one-app checks | section 7 |
| One-app work, this cluster's share *(new)* | 0 | **2** (1.5-2.5): Penpot in our frame, the shared page template | 7.2; the rest of the one-app bar is inside 02's and 06's rows |
| **Total** | **59** | **about 103** (95-114) | Rows may overlap by up to 2.5: 00's 2 with 07's gateway, and 02's session hand-off with 07's Hub pages |

**The calendar does not survive either.**

- **First app end to end** (journey steps 1-7) needs these lanes at once: the sandbox (01, about
  10 from day 0), Orca's first ship (02: about 6.5 calendar days, *only if* our build runs as its
  own stream from day 1; about 11.5 in series), the minimum Hub (07: about 5 working days on two
  engineers), a minimal gateway (07-verify: 8.4), signed-in Hub pages (1), Deploy's first-ship
  glue (about 5 of 06's 8.5) and the shared page template (1). **With five builders that is about
  10-12 working days. With the report's two, about a month** (about 47 engineer-days of
  first-ship lanes).
- **All of it:** about 20-21 working days with five builders; about 10 weeks with two.
- **Day 0 can decide the date more than the build.** None of it is engineering: an Azure
  subscription, Kata quota and feature registration (01); a Defender suppression rule, an ASR
  allowlist and a code-signing route for our build (02); on Panacea's side, our egress on the edge allowlist, the realm
  client, and P-1 to P-3 queued behind the cutover freeze (06-verify §2.4, 07); for our own Hub
  and gateway, a DNS name, an edge allowlist and Keycloak 26 on two replicas (07-verify §5); a
  Teams admin (05).

**Task counts.** "About 180 planned tasks replaced" is the clusters' own sum. It is not re-derived
here. Three notes: 02-verify freezes phase 2's Tauri desktop as the fallback rather than retiring
it; 07-platform finds the Hub, phase 6 and phase 8 plans lose only about 2.1 of about 27 days,
because they already stand on third-party parts; and the one-app ask adds work but removes none.

**Staffing is the lever, and it is money.** "Days, not weeks" holds only with about five builders
and a clear day 0. With two it is about a month to the first app. That is an owner call (section 7).

## 7. Corrected recommendation

**Assemble, and buy no whole product: upheld. But v1 ships as one app, not "several faces", and
it costs about 103 engineer-days, not 59.** The one app is a desktop under our name: 02-verify's
thin build of the pinned Orca. Every phase opens inside it, in our look, behind one sign-in, with
the same project and brief in view. The third-party products are engines behind our surfaces where
they can be (Spec Kit, the trackers, Panacea, LiteLLM). Where they cannot (Penpot's editor), our
frame surrounds them.

### 7.1 The one-app rules (every cluster's glue must meet them)

These restate 02-verify §7's five checks as rules for every cluster, and add the proof.

| # | Rule | How, with the products already picked | Built by | Left over in v1 |
|---|---|---|---|---|
| R1 | **One window, one name.** The user starts one app. Nothing in the main path opens another app or an outside browser | Our build of pinned Orca: name, icon, KPMG signature and feed, renamed at build time. Define, Design, Work and Deploy are Hub pages in the journey bar's one app-level view on the laptop's network, not in worktree tabs. Previews open there too (06-verify §6). Landing runs from the lane strip through the laptop CLI | 02; 06 and 03/05 for their pages | Reviewers read comments on the host's PR page; Teams notifications link back in |
| R2 | **One sign-in.** Entra once, zero clicks, no second prompt | The desktop hands the Hub a one-time session from `panacea login`, so Electron never meets Entra (02-verify §7). Penpot trusts the same realm (`PENPOT_OIDC_*`, 04-verify 2.3 row 9). Panacea's previews use the realm's SSO | 02; 08 for Penpot | Whether Penpot's OIDC completes silently inside the app-level view is unverified |
| R3 | **One look.** Our tokens on every surface the user works in | One CSS file in our build maps our tokens onto Orca's shadcn variables. Both stacks are Tailwind plus shadcn, and Orca hands its panels those tokens *"so panel documents can match the app"*. So Orca's chrome, its panels and our Hub pages all wear our palette. Geist on both | 02; every page owner | Penpot's own editor chrome, inside our frame |
| R4 | **One spine.** Define, Design, Develop, Deploy and the brief chip are reachable from every screen | 02's journey bar. A `lane` deep link on the registered scheme. Every Hub page carries the project and `brief:bf-…@n` | 02; 08 for the page template | None |
| R5 | **One vocabulary.** `CONTEXT.md` nouns on every surface | The build-time rename removes "Orca". A copy review of our pages. Panacea's rung names are mapped, or added to `CONTEXT.md` (06-verify §6) | 02, 06, 08 | Orca's other labels (its tree says "workspace") unless the rename list maps them |
| R6 | **Proven, not claimed.** | The journey test fails if a step leaves the app, asks for a second sign-in, or drops the brief key. 02's five checks run on every pin move. The look itself is the owner's eyes: headless screenshots are blocked on the corporate network | 08, 02 | None |

### 7.2 The one-app work, costed once

| Piece | Days | Where it is counted |
|---|---|---|
| Our build of pinned Orca: name, build-time rename, token CSS, the journey bar with its session hand-off, OSC status off, Orca's cloud features off | about 5 | 02-verify §7, inside its 15.5 |
| Our Deploy page in place of Panacea's Portal, with an in-app approve | 1.65 | 06-verify §6, inside its 8.5 (G1) |
| **Penpot in our frame.** Serve Penpot same-site under our domain. Our ingress swaps its `X-Frame-Options SAMEORIGIN` for `frame-ancestors` (04-verify 2.5 point 5). Sign-in is silent OIDC, and the journey bar and brief chip stay around it | 1 | Here. No other cluster owns Penpot's face (02-verify §7, "Still open") |
| **One shared page template** for every Hub page: layout, our tokens, the empty, error and unauthorised states, and the brief card that the bar's chip also uses | 1 | Here |
| R6's assertions | 0.5 | Here, inside the journey test's 2 |
| **This cluster's share** | **about 2, plus 0.5** | Section 6 |

The whole one-app bar is therefore about 9 engineer-days across three clusters, not the weeks the
report feared, and each piece is counted once.

### 7.3 The layers, corrected

| Layer | Pick | Change from the report |
|---|---|---|
| Develop cockpit, and the one window | Our thin build of the pinned Orca, with one Panacea plugin and one recipe | 02-verify: shipped as our app, not under Orca's name (§7-8); land through the laptop CLI; our build's OSC patch; phase 2's desktop frozen as the fallback, not retired; 15.5 days |
| Sandboxes | AKS Pod Sandboxing plus `agent-sandbox`, Australia East | 01-verify: day 0 first; the `kata-vm-isolation` name fix; default-deny egress added |
| Define | Spec Kit in an Orca terminal; the brief page in our shell | The pick is unchanged |
| Design | Penpot 2.18 and its MCP server, inside our Design page | 04-verify: 13 days and one tab per user; wrapped per R1-R4 |
| Work and team | ADO Boards and GitHub Issues behind our Work page; Teams for notifications that link back in | 05-verify: about 20 days |
| Deploy | Panacea's API and `panacea-mcp` on the **dev estate**, for KPMG-network pilots; Portal screens brought in-app | 06-verify: GitHub previews only in v1; the edge allowlist |
| Sign-in | Panacea's Keycloak realm, now | 00-verify: 25.0.0 on one replica. 07-verify §5 makes Keycloak 26 on two replicas required before anyone outside the build team relies on it; otherwise our own Keycloak, about 2 days |
| Hub, gateway, environment | **Ours** (07-platform): the Go Hub on Flexible Server; our LiteLLM with `keysmith` and `gatekeeper`; our AKS in Australia East | New to the whole: about 20.5 days (07-verify §5) |
| One-app work and journey test | Ours | About 4 days here (2, plus the journey test's 2); the rest of the one-app bar sits in 02 and 06 (7.2) |

### 7.4 Owner calls, revised (money or customer-visible only)

1. **No whole-product seats (money).** *Recommend: confirm.* Unchanged.
2. **"Several faces in v1" is withdrawn.** The user has asked for one app, and 02-verify §8 call 5
   rules the same way: Orca ships as our app from a thin build of the pinned upstream. Upstream
   Orca cannot pass. Its name is in the build, a plugin cannot add a top-level view, and its
   browser tabs are per-worktree and route through the sandbox (02-verify §7). What is left for
   the owner is small:
   - **the desktop's name and icon** (customer-visible);
   - **code signing** (money): Azure Artifact Signing at US$9.99 a month on Basic, if an Australian
     entity qualifies, or else an OV certificate in Key Vault (02-verify call 6).

   *Recommend: the thin build, our name, our tokens.* Our locked teal and warm graphite then reach
   Orca's chrome through its own shadcn variables.
3. **Staffing (money, new).** *Recommend five builders if "ASAP" is the bar.* That means the first
   app in about 10-12 working days and all of it in about 20-21. Two builders means about a
   month to the first app.
4. **Fallback if Orca's seams break (money, only if it fires).** *Recommend: note it.* Fork Orca
   first (02). OpenHands Enterprise is a weaker fallback than the report says. It runs Claude
   Code behind ACP, isolates with Sysbox, needs two Keycloaks and is PolyForm until bought: weeks,
   once procurement counts (section 3).
5. **GitLab only on customer demand (customer-visible).** *Recommend: unchanged.* Claude Code on
   GitLab Self-Managed is experimental today (section 5).
6. **Re-check "buy" quarterly (money, later).** *Recommend: unchanged, with more triggers.* For
   Anthropic's self-hosted environments: ZDR, a gateway, **Azure DevOps repositories**, and
   **sessions started from our shell**. For Copilot: Australia.

## 8. What could not be verified

Nothing was installed, signed up for or run. Every day figure is an estimate composed from other
estimates, and the calendar assumes the lanes run in parallel as the clusters describe.

| Area | Not verified | How to close it |
|---|---|---|
| Sibling sceptics | 00-, 06- and 07-verify were unfinished when read. 00's days, 06's §3 onward and all of 07-verify's verdict may move the 103 | Re-sum section 6 when all three are complete |
| Orca, one window | The journey bar and its app-level view are 02-verify's design, not sized against Orca's layout code (its softest number). Whether the `panacea login` session hand-off keeps every Hub page and Penpot free of a second prompt | 02-verify's canary on the first pin, with R6's assertions |
| Penpot in our frame | Whether an admin can set one default UI theme; whether its OIDC login stays silent inside a same-site frame; the header swap is 04-verify's reading of config, not a test | Part of the 1-day Penpot wrap |
| The one-app work | The 2 days here and 02-verify's 5 are estimates. Whether it *feels* like one app is the owner's eyes: headless screenshots are blocked on the corporate network | The owner reviews a clickable prototype of the shared shell before build, as the working agreements require |
| Coder | Which tier holds AI Gateway and Agent Firewall (the docs and the pricing page disagree); any price; whether the 5-agent cap reaches Claude Code inside a workspace; Tasks leaving at v2.37; an AKS-specific install page | A Coder quote and a release-notes check when v2.37 ships |
| OpenHands | The GitHub API was rate-limited, so stars and release dates are the report's. `openhands.dev` and `docs.openhands.dev` are blocked (HTTP 403). Azure DevOps support comes from `values.yaml` and a search summary of a blog, not docs. Running on `kata-vm-isolation` instead of Sysbox is untested. Price is by quote | A vendor call and a trial on AKS |
| GitLab | Whether a custom external agent pointed at our LiteLLM works on Self-Managed (an experiment, off by default); Kata's `runtimeClassName` on the Kubernetes executor | A trial on a self-managed instance, only if a customer brings GitLab |
| Anthropic, GitHub | Read at the vendor's docs today; both change monthly | The quarterly re-check (7.4 call 6) |
| The other rejects | Devin, Cursor, Factory, Replit, Lovable, v0, Bolt, Kiro, Jules, Ona and Codex were not re-fetched. Their region and gateway facts are the report's | Not needed: each fails B5 by design |
| Task counts | "About 180 tasks replaced" was not re-derived | Each cluster's own task table |
| Day 0 | No date exists for the subscription, Kata quota, the Defender rule, the Panacea-side asks or the Teams admin | Owner and IT; these can decide the date more than the build |

## 9. Sources

Read 2026-09-30. Vendor pages are primary unless marked "search".

- **Coder:** GitHub API `repos/coder/coder`, `/license` and `/releases/latest`; raw
  `LICENSE.enterprise` at `main`; <https://coder.com/docs/ai-coder/ai-gateway>,
  <https://coder.com/docs/ai-coder/ai-governance>, <https://coder.com/pricing>,
  <https://coder.com/docs/admin/users/oidc-auth>, <https://coder.com/docs/ai-coder/tasks>; the
  v2.35.3 add-on wording (search).
- **Anthropic:** <https://code.claude.com/docs/en/self-hosted-environments>.
- **GitHub:** <https://docs.github.com/en/enterprise-cloud@latest/admin/data-residency/github-copilot-with-data-residency>.
- **GitLab:** <https://docs.gitlab.com/user/duo_agent_platform/agents/external/>.
- **OpenHands:** as listed in section 3.
- **Orca:** the blobless clone at `59b746f`, files as listed in section 1, plus
  `src/renderer/src/assets/main.css`, `package.json` and `components.json`.
- **This programme:** `08-whole-product.md`; `01-` to `07-*-verify.md` and `07-platform.md` in
  this folder; `docs/DESIGN-SYSTEM.md`.
