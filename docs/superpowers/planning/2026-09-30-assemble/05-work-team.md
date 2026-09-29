# 05 — Work layer and team collaboration: what to adopt instead of build

**Date:** 2026-09-30. **Status:** research in progress, nothing adopted.
**Capability:** Phase W (work layer, 190 tasks, plan only) and much of Phase T (team mode, 69
tasks, plan only): issues, projects, cycles, boards, Linear-class speed, presence, notifications,
seeing each member work, agents taking issues.
**Owner direction (2026-09-30):** third-party first, the tool ASAP, days not weeks.

## The answer

_Pending — filled in last._

## 1. What Phase W and Phase T actually ask for

Read from the plans (grep only) on 2026-09-30:
`.claude/worktrees/work/docs/superpowers/plans/2026-09-25-phase-w-work-layer.md` and
`.claude/worktrees/team/docs/superpowers/plans/2026-09-25-phase-t-team-mode.md`.

**Phase W — 190 tasks, 528.5 planned hours, ~66 focused days** (the plan's own table, lines
339-351; the design's figure is ~138 days for one implementer, `OD-26`). A PanaceaApp-native,
Linear-parity-plus tracker: W1 (70 tasks) issues on the thread, Work lens, issue view, keys,
replica, Desk, Linear/CSV import, `Shift+R` Start; W2 (28) cycles, views, rank, filters; W3 (16)
agents on the work; W4 (29) roadmap, release, reporting; W5 (32) intake and ecosystem; WL (13)
later rows. Its goal line: *"A team moves its backlog into PanaceaApp and never goes back."*

**Phase T — 69 tasks** in three slices: TM1-α (tasks 01-33) the Orchestrator runner, the `OD-23`
policy mechanism, Fleet as a delegated Hub caller; TM1 (34-57) activity and presence, leases,
budgets, the team strip, Desk rows, the Orchestrator panel; TM2 (58-69) the lane mirror, team
land queue, quotas.

Split by who could supply it:

| Need | Phase / tasks | Commodity (a product can own it) | Ours (no product has the objects) |
|---|---|---|---|
| Issues, sub-issues, labels, states, comments, mentions, search | W1 02-17, 37-50 | **Yes** — every tracker | — |
| Cycles/sprints, saved views, filters, rank, board, timeline, roadmap | W2, W4 | **Yes** | — |
| Import from Linear/CSV | W1 15, 25, 69; W5 150-153 | **Yes** — or unnecessary if the customer's tracker *is* the store | — |
| Intake, magic words, linkbacks, webhooks, Slack/Teams | W5 | **Yes** | — |
| Notifications, mentions, "since you last looked" | W1 62-65, TM 54 | **Mostly** — Teams + tracker notifications | Desk wording rules (`OD-27`) |
| Presence: who is on the repo, what files | TM 34-35, 52-53 | **Transport yes** (pub/sub) | The facts (observed files, sessions) come from our control plane |
| An agent takes an issue | W1 24, W3 98-113 | **The trigger yes** (assign/delegate/@mention to an agent identity) | The sandboxed managed session, the worktree, the gateway token |
| Issue `serves` a brief criterion; done means proven (`OD-27`) | W1 02-10, 21-23b | No tracker models a criterion | **The provenance edge and the evaluator** |
| Orchestrators negotiating across owners (`OD-14`-`OD-17`) | TM1-α 23-31, TM1 36-42 | Nothing, open or closed | **All of it** |
| Leases on files, a turn queue, the policy file in every sandbox | TM 13-15, 23-26 | Parts (a KV with TTL) | The rules |

The line that decides the recommendation: **about three quarters of Phase W is a tracker, and
KPMG already pays for two.** The provenance edge, the done evaluator and the Orchestrators are
the product; the tracker is not.

## 2. Candidates — trackers KPMG already runs

Both are already licensed, already Entra-backed and already the git hosts of `OD-4`. Neither
needs procurement, a new DPA or a new login for a KPMG user.

### 2.1 Azure DevOps Boards — **integrate (first connector)**

| | |
|---|---|
| **Does** | Work items (epic/feature/story/task/bug), boards, backlogs, sprints, queries, dashboards, Delivery Plans, comments, @mentions, links to commits/PRs/builds. REST API, a Go client (`microsoft/azure-devops-go-api`), service hooks (webhooks) for created/updated/commented. |
| **Agents** | **Azure Boards → GitHub Copilot is GA**, including custom agents, with Copilot activity shown on the board card ([MS blog](https://devblogs.microsoft.com/devops/github-copilot-for-azure-boards/), [sprint 269 notes](https://learn.microsoft.com/en-us/azure/devops/release-notes/2026/sprint-269-update)). It sends work to *GitHub's* agent, not to our sandboxes; our own trigger is a tag or field plus a service hook. |
| **MCP** | `microsoft/azure-devops-mcp`, **MIT** (LICENSE.md read 2026-09-30). The **remote** server at `mcp.dev.azure.com` went **GA 2026-08-05**, Entra-authenticated, but Claude Code cannot connect yet: it needs Entra dynamic client registration ([GA post](https://devblogs.microsoft.com/devops/azure-devops-remote-mcp-server-ga/)). The local server works today. |
| **Licence / price** | Commercial SaaS KPMG already pays for. Basic **$6/user/month after 5 free**; Stakeholder access free. |
| **Deploys** | SaaS. **Australia is a selectable geography**; data is geo-replicated inside it ([data location](https://learn.microsoft.com/en-us/azure/devops/organizations/security/data-location?view=azure-devops)). Exception: **profile data of users outside US/EU/UK is stored in the US**. Azure DevOps Server exists for a customer who must host it. |
| **Enterprise fit** | Entra-native, conditional access, audit streaming, service principals and managed identities as callers. **Done-means-proven is enforceable here**: an inherited-process rule can restrict the transition to `Done` to members of one group ([rule docs](https://learn.microsoft.com/en-us/azure/devops/organizations/settings/work/apply-rules-to-workflow-states?view=azure-devops)), so only PanaceaApp's proof identity can set it. |
| **Maturity** | Microsoft first-party, the incumbent at KPMG. |
| **Gaps** | Not Linear-class speed. No notion of a brief criterion (a custom field or link holds it). |
| **Integration** | **3-4 days**: connector (read, write, comment, link), service hook receiver, Start-from-work-item, the Done rule and field. |

### 2.2 GitHub Issues and Projects — **integrate (second connector)**

| | |
|---|---|
| **Does** | Issues, **sub-issues, issue types, dependencies**, "relates to" (preview), **issue fields GA 2026-07-02** (typed org-level metadata) ([changelog](https://github.blog/changelog/2026-07-02-issue-fields-are-now-generally-available/)), Projects tables/boards/roadmaps, iterations, saved views. GraphQL + REST, webhooks, `go-github`. |
| **Agents** | **Agent HQ**: assign an issue to Copilot, **Claude** or Codex from the Assignees menu; each opens a draft PR ([changelog 2026-02-04](https://github.blog/changelog/2026-02-04-claude-and-codex-are-now-available-in-public-preview-on-github/)). The documented third-party list is **only Claude and Codex**; registering *our own* agent as an assignee is not documented. Those agents run on GitHub's compute, outside our Kata sandbox and LiteLLM gateway. |
| **MCP** | `github/github-mcp-server`, **MIT** (LICENSE read 2026-09-30), local or remote. |
| **Licence / price** | Commercial SaaS KPMG already pays for (EMU per memory; Enterprise ~$21/user/month list, unverified for KPMG's contract). |
| **Deploys** | SaaS. **GHE.com data residency in Australia is GA since 2025-02-04** ([changelog](https://github.blog/changelog/2025-02-04-github-enterprise-cloud-data-residency-in-australia-is-generally-available/)). Whether KPMG's EMU tenant is on github.com or an AU GHE.com subdomain is **unverified**. |
| **Enterprise fit** | EMU with Entra OIDC, SCIM, audit-log streaming to Azure Event Hubs/Blob. **Cannot restrict who closes an issue**, so "proven" becomes an issue field only our app writes, and a close without proof reads *done, not proven* in our cockpit. |
| **Gaps** | Projects are fast enough, not Linear-fast. No criterion concept (issue field or sub-issue link holds it). |
| **Integration** | **3-4 days** (GitHub App, webhooks, GraphQL for Projects/fields, Start-from-issue), sharing the tracker interface with 2.1. |

**Why these two first.** Every KPMG user already has them, both have an AU hosting answer, both
speak Entra, both ship an MIT MCP server our sandboxed agents can use under a scoped token, and
`OD-4` already puts both behind one host interface in phase 3. The tracker connector is the same
shape one layer up.

## 3. Candidates — SaaS trackers to buy

### 3.1 Linear — **reference now; optional connector later. Not the default.**

| | |
|---|---|
| **Does** | The speed benchmark Phase W was written against: issues, cycles, projects, initiatives, triage, views, keyboard-first, real-time sync. GraphQL API, webhooks. |
| **Agents** | The best agent model on the market. **Linear for Agents**: an agent is an app user you can @mention or **delegate** an issue to; Linear opens an `AgentSession` (states `pending`, `active`, `error`, `awaitingInput`, `complete`, `stale`) and the agent reports back as typed activities (thought, action, elicitation, response, error). It must answer within **10 seconds** or be marked unresponsive ([agent interaction docs](https://linear.app/developers/agent-interaction)) — the same bar as Phase W's "first sign of work within 10 seconds". |
| **MCP** | Official remote server `https://mcp.linear.app/mcp`, OAuth 2.1 with dynamic client registration ([docs](https://linear.app/docs/mcp)). Works from Claude Code today. |
| **Licence / price** | Commercial SaaS. Basic $10, Business **$16/user/month**; Enterprise (SAML, SCIM, audit log) custom, third-party estimates $20-30 (unverified). |
| **Deploys** | SaaS only. **Regions: United States or European Union, chosen at workspace creation. No Australia** ([security page](https://linear.app/docs/security), read 2026-09-30). |
| **Enterprise fit** | SOC 2 Type II, GDPR, HIPAA BAA. SAML/SCIM on Enterprise only. **Fails `OD-19`/`OD-24` for any project classified *AU data only*.** A second tracker for KPMG staff who already live in ADO and GitHub. |
| **Maturity** | Venture-backed, large and fast-moving; no self-host, no customer-cloud option. |
| **Gaps** | AU residency; runs outside the customer's cloud; KPMG does not own it. |
| **Integration** | 2-3 days as a third connector (OAuth app + AgentSession webhook + activities). |

**What to take from it now, free:** its agent-session contract. Our ADO and GitHub connectors
should report an agent's work in the same five activity kinds on the work item, so the three
trackers read alike and a Linear connector is a thin add later.

### 3.2 Jira Cloud (Atlassian) — **integrate as the third connector, when a customer brings it**

| | |
|---|---|
| **Does** | The enterprise incumbent: issues, sprints, boards, roadmaps (Premium), JQL, automation. REST, webhooks, Forge apps. |
| **Agents** | You can **assign a work item to Claude, Cursor, Codex, GitHub Copilot or Jira's own coding agent** from the assignee menu, or @mention one ([support](https://support.atlassian.com/jira-software-cloud/docs/collaborate-on-work-items-with-ai-agents/)). **Our own platform can register as an assignable remote agent** through the Forge `rovo:agentConnector` module — **Preview** ([dev docs](https://developer.atlassian.com/platform/forge/remote-agents-in-jira/)). The Marketplace "Claude Agent for Jira" takes a direct Anthropic API key and a GitHub service account: that bypasses our gateway and sandbox, so it is **not** our path (`OD-19`). |
| **Licence / price** | Commercial SaaS. Standard ~$7.53, Premium ~$13.53/user/month (Jan 2025 list); **SAML SSO with Entra needs Atlassian Guard**, ~$4/user/month extra across the whole Atlassian org (third-party figures, unverified). |
| **Deploys** | SaaS. **AU data residency (AWS Sydney) on Standard, Premium and Enterprise.** **Data Center is closed to new customers since 2026-03-30 and reaches end of life 2029-03-28** ([Atlassian](https://www.atlassian.com/licensing/data-center-end-of-life)), so there is no customer-hosted Jira to plan for. |
| **Enterprise fit** | Strong: Guard, audit log, residency. |
| **Maturity** | Atlassian, the market default in AU regulated enterprise. KPMG's own use of Jira: **unverified**. |
| **Gaps** | Speed; Guard cost; agent connector in Preview. |
| **Integration** | 3-4 days behind the same tracker interface. |

**Not assessed in depth, on purpose:** Shortcut, ClickUp, Monday dev, Asana. None is a KPMG
system, and a customer who runs one can be served by the same connector shape later.

## 4. Candidates — self-hosted trackers

The earlier library study (`2026-09-28-oss-adoption/01-work-layer.md` §3.1) already found that
**no permissively licensed Linear-class tracker exists**. This pass asks the product question
instead: would running one in our environment beat integrating the customer's own? It would not,
for one reason that is independent of licence: **it gives a KPMG team a third tracker** beside the
ADO and GitHub it already uses, and a customer a tracker nobody asked for.

### 4.1 Plane — **reject as the default** (licence and a third tracker)

| | |
|---|---|
| **Does** | The strongest open Linear clone: work items, cycles, modules, views, pages, intake, an MCP server. ~60k stars (Glasses, observed 2026-09-29). |
| **Licence** | Two editions, and neither is permissive. **Community Edition: AGPL-3.0.** **Commercial and Airgapped Editions: closed source** ([editions](https://developers.plane.so/self-hosting/editions-and-versions)). |
| **What AGPL-3.0 forces** | (1) If we **modify** Plane and people use it over a network, we must offer **those users the complete source of our modified version** under AGPL (§13). (2) Installing it **into a customer's tenancy is distribution**, with the same source obligation. (3) Our own services talking to an **unmodified** Plane over its API are not a derivative work and are not forced to AGPL. (4) Many enterprise legal policies ban AGPL outright; the licence rule here marks it RESTRICTED. The permissive-alternative answer is not another tracker: it is integrating the customer's. |
| **Price** | Commercial: Pro **$6/seat/month** annual, Enterprise Grid by quote, same price cloud or self-hosted, **12 seats free per workspace** ([pricing](https://plane.so/pricing)). |
| **Deploys** | Self-hosted Docker or Kubernetes, or Plane Cloud (no AU region found). |
| **Enterprise fit** | OIDC and SAML SSO documented for self-host (so Keycloak/Entra works); audit and governance in Commercial. |
| **Gaps** | A third tracker; no criterion concept; its agent story is its own, not our sandbox. |
| **Integration** | 5-8 days (AKS deploy, Keycloak OIDC, backups, connector) plus running it forever. |

### 4.2 The rest, one line each

| Candidate | Licence (read) | Verdict | Why |
|---|---|---|---|
| **Huly** (`hcengineering/platform`) | **EPL-2.0** (read 2026-09-28): weak, file-level copyleft; modified EPL files we distribute must be published under EPL | reject | A heavy all-in-one (tracker, chat, docs, HR) that overlaps Teams; a third tracker; licence not on the allowed list |
| **OpenProject** | **GPL-3.0** core plus a commercial Enterprise token | reject | Distributing a modified copy forces GPL source; mature on-prem, but a slow Rails UI and no agent model |
| **Taiga** | **MPL-2.0** (read 2026-09-30) | reject | Allowed licence, but dormant: last push 2023-12-13 (earlier study) |
| **GitLab CE** | MIT (CE only; epics and iterations are paid tiers) | reject | A third forge beside ADO and GitHub |
| **Vibe Kanban** (`BloopAI/vibe-kanban`) | **Apache-2.0** (read 2026-09-30), 28.2k stars | reject | Kanban where each card runs an agent in its own worktree: our cockpit's shape. **The repository says it is sunsetting** |
| **beads** (`gastownhall/beads`) | **MIT**, 27.5k stars | reference | A Dolt-backed graph issue tracker *for agents*: atomic `--claim`, ready-work detection, dependencies, MCP server, messaging. A per-repo agent scratchpad, not a team tracker for people; its claim model is worth reading for Phase T leases |
| **MCP Agent Mail** (`Dicklesworthstone/mcp_agent_mail`) | "MIT **with OpenAI/Anthropic Rider**" (read 2026-09-30): prohibits use by Anthropic, OpenAI and anyone acting on their behalf | **reject (licence)** | Agent inboxes and file reservations are exactly Phase T's shape, but a field-of-use rider is not an open licence, and our agents are Claude Code |

Also rejected on licence in the earlier study and not re-opened: Focalboard (AGPL source),
Planka (fair-code since v2), Vikunja and Leantime (AGPL), Redmine (GPL-2.0).

## 5. Candidates — presence, notifications, team surfaces

KPMG's chat is Microsoft Teams; Slack is not assumed. Phase T's awareness facts (which session,
which files, which lease) come from our control plane and Fleet, so a product can carry them but
cannot produce them.

| Candidate | What it does for us | Licence / price | Deploy / residency | Verdict | Days |
|---|---|---|---|---|---|
| **Azure Boards app for Teams** and **GitHub app for Teams** | Channel subscriptions on work items, issues, PRs, checks; threaded cards; create items from chat ([ADO](https://learn.microsoft.com/en-us/azure/devops/boards/integrations/boards-teams?view=azure-devops), [GitHub](https://docs.github.com/en/integrations/how-tos/teams/use-github-in-teams)) | First-party, included | Microsoft 365 tenant | **adopt as-is** — every tracker notification W1 62-65 would have built | 0 (config) |
| **Our own Teams bot** on the **Microsoft 365 Agents SDK** | The `OD-16` escalations: *inform* as a card, *decide* as an Adaptive Card with `Action.Execute` buttons answered in Teams, proactive DMs | SDK **MIT** (`microsoft/Agents-for-js`, read 2026-09-30); Azure Bot Service, Teams channel | Bot registration in our Azure subscription; messages live in the user's M365 tenant | **integrate** — the Desk's NEEDS YOU row reaches people where they already are | 2-3 |
| **Teams Workflows webhook** | One-way posts to a channel. Office 365 connector webhooks were **retired in May 2026** ([MS](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/)); Workflows cards post as the Flow bot | Included | M365 | reference — too thin for *decide*, fine for a digest | 0.5 |
| **Microsoft Graph presence** | Teams availability (Available/Busy/Away) beside each person in the team strip | Included (`Presence.Read.All`, admin consent) | M365 | reference — nice, not needed for the first cut | 1 |
| **Azure Web PubSub** | Managed WebSocket fan-out, groups, Entra auth; **Australia East available**; Standard ~$49/unit/month per 1,000 connections ([pricing](https://azure.microsoft.com/en-us/pricing/details/web-pubsub/)) | Commercial Azure service | Azure, AU East | reference — only if the Hub feed outgrows itself | 1-2 |
| **NATS JetStream** | Subject-permissioned bus, durable inboxes, KV with TTL for leases, presence (earlier study #7) | Apache-2.0 | Self-host on AKS | **integrate when TM1-α is built**, as the earlier study said — not part of the fast cut | 2-4 |
| **centrifuge / Centrifugo** | Presence and pub/sub | MIT (lib) | Self-host | reject for now: the sceptic's point stands, it is a **third real-time protocol** beside the PTY relay and the Hub feed (`08-sceptic.md` §2.4) | — |
| **Liveblocks** | Presence, comments, notifications, CRDT rooms | Clients Apache-2.0; **server AGPL-3.0**; SaaS, self-host only as an Enterprise add-on ([data storage](https://liveblocks.io/docs/platform/data-storage)) | Their cloud; no AU residency found | reject — presence is not worth a US SaaS or an AGPL server | — |
| **Yjs awareness** | Cursor and presence protocol for a shared document | MIT | Anywhere | reference — belongs to the design canvas (phase 5), not the work layer | — |

**Seeing each member work, without building a surface first.** Three facts already exist outside
Phase T once the connectors land: who is assigned to which issue (the tracker), which branch and
PR it produced and whether checks pass (the git host), and what their sessions are doing now (our
control plane, already built in phase 2). A team strip that joins those three is a view, not a
subsystem.

## 6. Recommendation

**Bring your own tracker.** Azure DevOps Boards and GitHub Issues/Projects become the work
layer's system of record, behind one `tracker` interface (the same shape as `OD-4`'s host
interface in phase 3). PanaceaApp stops building a tracker and keeps only what no tracker has:
the edge from an issue to the brief criterion it serves, the agent session that works it, and
the proof that closes it. Teams carries notifications and escalations: the first-party ADO and
GitHub Teams apps as-is, plus one small PanaceaApp bot for *inform* and *decide*.

**Integration: about 16 engineer-days; first end-to-end demo on ADO in about 5; all of it in
about 6-7 working days on three parallel lanes** (connectors; provenance and Start; lens and
Teams). Itemised in §7. These are estimates; nothing here has been built or run.

**Why this, over the alternatives:**

1. **Zero adoption cost for the customer.** A KPMG team already has ADO and GitHub, already signs
   in with Entra, already has the Teams apps. Phase W's own goal was *"a team moves its backlog
   into PanaceaApp and never goes back"*; the fastest way to meet it is to not ask them to move.
2. **It passes the hard constraints today.** AU residency: ADO's Australia geography, GitHub's
   AU GHE.com. Customer's own cloud later: Azure DevOps Server and GitHub Enterprise Server take
   the same connectors with a different base URL. No AGPL, no new SaaS processor, no new DPA.
3. **Done means proven survives, and on ADO it is enforced by the tracker itself**: a process
   rule restricts `Done` to PanaceaApp's proof identity (§2.1). On GitHub it degrades to a
   `Proof` field only our app writes, and *done, not proven* in our cockpit.
4. **"An agent takes an issue" is now a tracker-native gesture** (GitHub Agent HQ, Azure Boards →
   Copilot, Jira agents, Linear delegation). We match the gesture — Start from the issue or from
   our cockpit, a first sign of work within 10 seconds as one live status comment edited in
   place, Linear's five activity kinds (§3.1) — and differ where they cannot: the agent runs in
   **our Kata sandbox, through our LiteLLM gateway (Global or AU), on the customer's cloud**, and
   the issue closes on **observed** evidence.
5. **The plan already half-knew.** Phase W tasks 150-155 and 171 were tracker sync *into* our own
   tracker. Inverting that — the tracker is the store, we are the agent and the proof — keeps those
   tasks' work and deletes the tracker behind them.

**What it replaces** (counted from the Phase W task tables; the day figures are the plan's):

| Phase W, 190 tasks | Tasks | What happens |
|---|---|---|
| Replaced outright by the tracker | **127** | W1 02-08, 11-17, 25, 37-39, 41-60, 66-69; W2 all but 95, 96, 96b; W3 99, 100, 107, 113; W4 all but 131, 135; W5 143-149, 156, 158-163, 168-170, 172-174; WL 175-178, 182-184, 186-187 |
| Survive, reshaped into the ~16 days of glue | **38** | W0 01; W1 09, 10, 18-24 with 23b, 36, 40, 61-65, 70; W1b 71; W2 96b; W3 98, 101, 103, 104, 108, 109; W4 131, 135; W5 150-155, 157, 165, 171 |
| Not tracker work: move to their owning phase unchanged | **19** | W1 26-35 and W2 95-96 (design-language uplift, `OD-28`); W5 164, 166, 167; WL 179-181, 185 |
| Wait for Phase T's negotiation | **6** | W3 102, 105, 106, 110, 111, 112 |

**Phase T, 69 tasks: honestly, products replace little of it directly.** The tracker plus Teams
take the human-delivery surfaces — the team strip becomes a view over tracker assignment and
session state (53), Desk rows and escalation delivery move to Teams cards (parts of 30, 54), and
tracker audit reaches Sentinel through Microsoft's own ADO and GitHub connectors (part of 45):
**about 5 tasks**. The core — Fleet, the Orchestrator runner, leases, negotiation, the `OD-23`
policy file, the lane mirror (**about 55 tasks**) — has no product anywhere. What the tracker
*does* buy is a **fast team v1**: agents read teammates' in-progress issues and live status
comments through the tracker's MCP server, the platform flags file overlap between sessions as an
*inform* to both people, and the Orchestrator negotiation is built after. That is a scope change
to `OD-16`, so it is an owner call (§8).

## 7. What we still build (glue)

_Pending._

## 8. Owner calls

_Pending._

## 9. What was not verified

_Pending._
