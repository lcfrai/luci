# 05 — Work layer and team collaboration: what to adopt instead of build

**Date:** 2026-09-30. **Status:** first run complete (stopped 09:47); revised on relaunch for the
owner's second direction (§0, which wins where it differs). Research only; nothing adopted, built
or run.
**Capability:** Phase W (work layer, 190 tasks, plan only) and much of Phase T (team mode, 69
tasks, plan only): issues, projects, cycles, boards, Linear-class speed, presence, notifications,
seeing each member work, agents taking issues.
**Owner direction (2026-09-30):** third-party first, the tool ASAP, days not weeks.
**Second direction (2026-09-30, later):** Panacea is the strong default, the best fully-fledged
solution wins, Coolify and Supabase allowed, Panacea snippets allowed with attribution (`OD-13`
relaxed), Orca (`stablyai/orca`) is the cockpit direction. See §0.

## 0. Revised for the owner's second direction

**The answer stands; where the glue comes from changes.** Integrate Azure DevOps Boards and GitHub
Issues/Projects, carry *inform* and *decide* in Teams, and build only the provenance glue. None of
the options the second direction opens is a tracker: Panacea has no work items (§0.1), Coolify is a
deploy PaaS, Supabase is a backend (§0.2). So the fully-fledged tracker is still the one KPMG
already runs. What moves: the glue borrows Panacea's code and team model, Start and the Work lens
land in Orca if Orca becomes the desktop (§0.3), and one collaboration surface comes whole from
Panacea (preview comments).

| # | What changed | First run | Now | Why |
|---|---|---|---|---|
| 1 | Team roster, members, invites, join requests, team budgets | Implied Hub and Phase T work (43, 44, 66, 67) | **Panacea's team API**: teams are Keycloak groups in the realm `OD-30` already signs us in through | Built and running; `routes.go:1897-2020` (§0.1) |
| 2 | Connector code | Written fresh on `azure-devops-go-api` and `go-github` | **Panacea snippets**: the sticky comment upsert, the ADO Service Hooks and GitHub hook lifecycle, the ADO hook check | Production code, same APIs; `OD-13` relaxed for snippets |
| 3 | Audit chain head (Phase T 45) | Sentinel connectors only | Plus **Panacea's signed chain head** (`audit_signed_head.go`) | The same property task 45 asks for |
| 4 | Collaboration on the running app | Not covered | **Adopt Panacea's preview comments** plus *file as issue* (1 day) | A stakeholder's comment on a preview becomes a tracker item on the thread |
| 5 | Start from an issue; the Work lens | Our Tauri desktop, `Shift+R` | **Orca's "open a worktree from any task"** for GitHub and Jira; we add an **ADO Boards source to Orca** (2 days) | Orca is the cockpit direction and has no ADO (§0.3) |
| 6 | Presence and the team feed | The Hub feed; NATS later | Unchanged. **Supabase Realtime** only if the Hub itself moves to Supabase | Otherwise a third real-time protocol (§0.2) |
| 7 | Integration days | 16 | **17**, or 15 if Orca does not replace the desktop | Snippets −1.5, Spec Kit −0.5, preview comments +1, Orca ADO source +2 (§7) |
| 8 | Owner calls | 4 | **5**: call 1 now names `OD-35`; new call 5, *PanaceaApp teams are Panacea teams* | §8 |

**Candidates added by this revision** (verdicts in §0.1-§0.3 and the table under *The answer*):
Panacea's team API, snippets and preview comments (**integrate**); Orca's task sources (**adopt
with the cockpit, extend with ADO**); Supabase Realtime (**reference**); Coolify (**reject** for
this capability); Panacea's Backstage (**reject** for the work layer); Claude Code agent teams
(**reference**). **No first-run verdict is reversed**; §2-§5 stand as written.

### 0.1 Panacea for this capability, on merit

Read-only, `git grep` over Panacea's tracked files on 2026-09-30 (another session is working there,
so paths are as of today's tree).

| Panacea asset | What it does | Use for us | Verdict |
|---|---|---|---|
| Work items, boards, sprints | **None.** `git grep -i "workitem\|_apis/wit\|wiql"` over `services/` finds nothing | — | Panacea is not a tracker; ADO and GitHub stay |
| **Team API** (`services/panacea-api/routes/routes.go:1897-2020`) | Teams are **Keycloak groups** (`handlers/team_onboarding.go:16-26`). Members list, add and remove; invite; join requests a team admin approves or denies; self-create gated by the cost-centre owner and a SuperAdmin (`handlers/team_create_approval.go`); per-team billing, cost, budget, git PATs, security and risk policy; project-to-owning-team RBAC (`routes.go:134-140`) | **The roster for Phase T and the team view**: `GET /api/v1/teams/{name}/members`, or a groups claim if the realm maps one for our client (not verified). The realm is Panacea's until phase 8 runs its own (`OD-30`) | **integrate (API), 0.5 day** |
| **Sticky comment upsert** (`services/panacea-build/pkg/prcomment/client.go:54`) | Creates the bot's PR comment or **updates it in place**, for GitHub, Gitea and Azure DevOps | **The one live status comment per session** (§7 piece 1). GitHub issue and PR comments share one endpoint (`/repos/{r}/issues/{n}/comments`, `client.go:78-85`), so that half ports as is, with its hard-coded `api.github.com` made a base URL for GHE.com; ADO work-item comments are another endpoint, so that half keeps the pattern and auth | **snippet** |
| **Hook lifecycle** (`handlers/webhook.go`, `handlers/webhook_ado.go:17-42`) | Creates, lists and deletes GitHub hooks and **ADO Service Hooks subscriptions** (publisher `tfs`, consumer `webHooks`), matched on the delivery URL | The tracker's subscriptions: the same API with work-item events (`workitem.created`, `workitem.updated`, `workitem.commented`) | **snippet** |
| **ADO hook check** (`services/webhook-receiver/providers/azuredevops.go:31`) | HMAC header or Basic secret, constant-time, rejects an empty secret | The service hook receiver | **snippet** |
| **Notifications** (`handlers/notifications.go:35-43, 247`) | Per-project channels (Slack, email, webhook); `NotifyOnce` dedupes across restarts. **No Teams channel**, and SMTP *"is not configured anywhere on this platform"* (`team_create_approval.go:17-18`) | The dedupe for the Teams digest; not the delivery path | **snippet**; our Teams bot stays |
| **Preview comments** (`handlers/preview_comments.go:19-32`, `routes.go:542-546`) | Threaded comments **pinned to elements on preview and staging pages** by an embedded widget; reply, resolve; a `preview_comment` event on each | **Review on the running app**, already there because phase 7 deploys previews through Panacea. Glue: route the event through a webhook channel and *file it as an issue* under the lane's work item, keeping its `serves` edge | **adopt as-is + 1 day** |
| **Audit chain** (`clients/audit_pg.go`, `clients/audit_signed_head.go:10-18`, `routes.go:454-473`) | Hash-chained `audit_logs` plus **signed chain-head anchors** through Vault transit, which catch a truncated tail | Phase T 45's *anchored chain head*. `OD-32` swaps Vault for OpenBao, whose transit API should match (not verified) | **snippet** |
| **Backstage** (`backstage/packages/*/package.json`) | Catalog, scaffolder, TechDocs, API docs, OIDC sign-in. **No** notifications, signals, Azure DevOps, GitHub Issues or Jira plugins installed | A developer portal, not where a team works | **reject** for the work layer |
| **panacea-mcp** (`services/panacea-mcp/src/index.ts`) | Deploy, rollback, logs, pods, env, projects, audit, health, plus a generic `panacea_api_request` | Agents read **deploy state** (phase 7's evidence). No team, issue or notification tools | reference |

**Trade-off, in one line:** Panacea's team model costs nothing and is the one KPMG users already
belong to, but it ties PanaceaApp's teams to Panacea's API until the lift-in (`OD-13`); sign-in is
already tied the same way (`OD-30`), so the dependency is not new.

**Snippet rule:** a file that takes Panacea code opens with its source, e.g.
`// Adapted from Panacea services/panacea-build/pkg/prcomment/client.go @ <commit>`.

### 0.2 Supabase and Coolify, on merit

Licences read from each repository's `LICENSE` on raw.githubusercontent.com, 2026-09-30.

| Candidate | Does, for us | Licence | Deploy and residency | Gaps | Days | Verdict |
|---|---|---|---|---|---|---|
| **Supabase Realtime** | Presence per client with sync, join and leave events; Broadcast for fast messages (the docs warn that calling `track()` rapidly "will flood the channel"); Postgres Changes as a feed; private channels authorised by RLS | `supabase/realtime` **Apache-2.0**; `supabase/supabase` **Apache-2.0**; `supabase/auth` **MIT** | Self-host on AKS in Australia East (Compose is official, Kubernetes is a community chart), or Supabase Cloud in **Sydney, `ap-southeast-2`, on AWS**: a new processor, not Azure. Third-party auth takes asymmetric JWTs from an OIDC issuer with a `kid`, so Keycloak should fit (**not verified**) | A **third real-time protocol** beside the PTY relay and the Hub feed (`08-sceptic.md` §2.4) unless the Hub itself runs on Supabase; the facts it would carry (sessions, files, leases) still come from our control plane | 2-3 standalone; about 1 if the Hub is on Supabase | **reference** |
| **Supabase as a tracker backend** | Tables, RLS, PostgREST and Realtime make a custom tracker quicker to write | as above | as above | Still Phase W's 190 tasks on a different backend; the answer is not to build a tracker | — | **reject** |
| **Coolify** | A self-hosted PaaS whose one-click services could run an open tracker | **Apache-2.0** | Docker on a VM; no Kata, no AKS (`06-deploy.md` §3.1) | Hosting a tracker does not change its licence (Plane AGPL-3.0, Huly EPL-2.0) or make it less of a third tracker | — | **reject** for this capability |

**Where Supabase would earn its place:** if the Hub itself moves to Supabase Postgres, Phase T's
feed and presence come from Realtime for about a day, and Phase T's Hub route tasks (04, 06-08)
shrink toward RLS tables and PostgREST. That is a Hub decision, not this capability's, and none
of it is counted in the days here.

### 0.3 Orca and the work layer

Read 2026-09-30: Orca's `README.md` and `LICENSE` (**MIT**, "Copyright (c) 2026 Lovecast Inc.")
on raw.githubusercontent.com; the docs pages `onorca.dev/docs/review/linear` and
`/docs/notifications` through WebFetch. The GitHub API was rate-limited, so Orca's source tree was
not read. Not re-argued: Orca is the owner's cockpit direction.

- **Task sources:** *"Browse PRs, issues, and project boards in-app — open a worktree from any
  task"* (README). The docs name **GitHub Issues, Linear and Jira**; **no Azure DevOps, no
  GitLab**, and **no documented plugin point** for another source.
- **Linear:** a personal API token pasted into settings; moving the issue to *In Progress* on
  worktree creation is *"opt-in per team"*; the branch uses Linear's suggested name.
- **Notifications:** fired when *"an agent transitions from working to idle"*: system alert,
  sound, a chip on the worktree, a header bell with an unread count, and a phone app. **Single
  user: no team view, no Slack, Teams or webhook.**
- **Review:** comments on any diff line go back to the agent.

| Our glue (§7) | With Orca as the desktop |
|---|---|
| 3. Start from an issue | GitHub and Jira: Orca's *open a worktree from any task* is the gesture. Our control-plane route still puts the session in our sandbox, posts the live status and tags the gateway token (how an Orca worktree reaches our sandbox is `02-fleet-cockpit.md` §R3). ADO: **we add an ADO Boards task source to Orca**, a WIQL list, the item view and *open a worktree*, as a fork patch offered upstream: **2 days** |
| 4. Work lens v0 and team view | Orca panels instead of Tauri views, the same 2.5 days; the roster from Panacea's team API |
| 5. Teams bot | Unchanged. Orca notifies *me* about *my* agents, on the desktop and the phone; the bot carries what crosses people (*inform*, *decide*, file overlap) |
| Linear (§3.1) | Orca already speaks Linear, so a customer whose data class allows US or EU hosting gets it with no connector UI to build. **Off for AU-only projects**: Linear has no AU region |

### 0.4 Phase T, revisited

The first run found about 5 of Phase T's 69 tasks taken by products. Panacea's team model and code
raise that to **about 8, 2 of them mostly**:

| Phase T task | What takes it | How much |
|---|---|---|
| 45 Audit to Sentinel; the anchored chain head | Sentinel's ADO and GitHub audit connectors (§7) plus Panacea's signed chain head (`audit_signed_head.go`) | mostly |
| 53 The team strip and click-in | A view over tracker assignee, git host state and central sessions; the roster from Panacea's team API | mostly (the UI stays ours) |
| 44 Leavers, member removal, delegation revocation | Panacea's `DELETE /api/v1/teams/{name}/members/{userId}` and Keycloak; revoking delegations stays ours | part |
| 43 Role-to-verb, *auditor* and *compliance*, team policy | Panacea's team-admin role and per-team security policy; the verbs stay ours | part |
| 66, 67 Team and project quotas; the quotas view | Panacea's per-team budget and cost routes (`routes.go:1946-1961`), and LiteLLM's team budgets (the gateway's cluster) | part |
| 30, 54 Escalation delivery; Desk rows | Teams cards (§5) | part |

The core still has no product anywhere: Fleet, the Orchestrator runner, leases, negotiation, the
`OD-23` policy mechanism and the lane mirror, **about 55 tasks**.

**Claude Code agent teams: reference.** It is built into the CLI we run unmodified, and is
*"experimental and disabled by default"* (`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`): a lead and its
teammates share a task list (pending, in progress, completed, with dependencies and file-locked
claims) and per-agent JSON mailboxes under `~/.claude/teams/`
([docs](https://code.claude.com/docs/en/agent-teams), read 2026-09-30). It cannot be Phase T's
channel: *"one team per session"*, and teammates share one machine's files, so it spans neither
Kata sandboxes nor people, and `OD-8` lets nothing require a harness's own files. Worth taking: its
task states, and its `TaskCompleted` hook, which blocks completion on exit code 2, the same shape
as done-means-proven.

## The answer (revised for the second direction, §0)

**Do not build a tracker. Integrate the two KPMG already runs — Azure DevOps Boards first, GitHub
Issues/Projects beside it — behind one `tracker` interface, and make PanaceaApp the agent the
work is handed to and the proof that closes it. Teams carries *inform* and *decide*. Panacea
supplies the team roster and much of the connector code; Orca, as the desktop, supplies Start for
GitHub and Jira, and we give it an ADO Boards source.**

- **Integration: ~17 engineer-days (15 if Orca does not replace the desktop); first end-to-end
  demo on ADO in ~5 working days** with two builders in parallel (§7).
- **Replaces 127 of Phase W's 190 tasks outright**; 38 survive as the glue; 19 are not tracker
  work and move to their owning phase; 6 wait for Phase T (§6).
- **Phase T: about 8 of 69 tasks taken by products and Panacea, 2 of them mostly** (§0.4). The
  Orchestrator core has no product anywhere. The tracker buys a fast team v1 instead, if the
  owner accepts the order change (§8, call 4).
- **No new spend.** No AGPL, no new SaaS processor, AU residency on both trackers (with one
  Microsoft profile-data exception, §8).
- **Five owner calls**, all customer-visible (§8).

| Candidate | Verdict | One line |
|---|---|---|
| Panacea: team API, snippets, preview comments | **integrate** (added 2026-09-30, §0.1) | Teams are Keycloak groups already; sticky comment upsert, ADO Service Hooks lifecycle, signed audit head; element-pinned preview comments. No tracker |
| Orca's task sources (GitHub, Linear, Jira) | **adopt with the cockpit; extend with ADO** (added, §0.3) | *Open a worktree from any task*; no ADO and no plugin point, so a 2-day ADO Boards source |
| Supabase Realtime | reference (added, §0.2) | Apache-2.0 presence and feed; a third real-time protocol unless the Hub moves to Supabase |
| Coolify; Panacea's Backstage | reject for this capability (added, §0.1-§0.2) | A deploy PaaS; a developer portal with no work plugins |
| Claude Code agent teams | reference (added, §0.4) | Experimental; one team per session on one machine |
| Azure DevOps Boards | **integrate (first)** | Entra-native, AU geography, `Done` enforceable by process rule, MIT MCP server |
| GitHub Issues/Projects | **integrate (second)** | Issue fields GA, AU GHE.com, MIT MCP server; cannot restrict closing |
| Jira Cloud | integrate (third, when a customer brings it) | AU residency, assignable remote agents (Preview); Data Center end of life 2029 |
| Linear | reference; optional connector | The best agent-session model; **no AU region** |
| Teams (first-party apps + our bot) | adopt + integrate | Where KPMG people already are; Agents SDK MIT |
| Plane | reject | AGPL-3.0 community, closed-source commercial; a third tracker |
| Huly, OpenProject, Taiga, GitLab CE, Vibe Kanban | reject | EPL / GPL / dormant / a third forge / sunsetting |
| beads | reference | Agent-side claims and ready-work, worth reading for Phase T leases |
| MCP Agent Mail | reject | MIT with an anti-Anthropic rider |
| Liveblocks | reject | AGPL server, no AU residency |
| Azure Web PubSub, NATS, Yjs | reference / later | Transport only; not needed for the first cut |

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
GitHub Teams apps as-is, plus one small PanaceaApp bot for *inform* and *decide*. **Panacea**
supplies the team roster (its team API over Keycloak groups), four pieces of connector and audit
code, and preview comments on the running app (§0.1). **Orca**, as the desktop, supplies Start
from GitHub and Jira issues and hosts the Work lens; we add its ADO Boards source (§0.3).

**Integration: about 17 engineer-days (15 if Orca does not replace the desktop); first end-to-end
demo on ADO in about 5; all of it in about 7 working days on three parallel lanes** (connectors;
provenance, Start and preview comments; lens, Orca's ADO source and Teams). Itemised in §7. These
are estimates; nothing here has been built or run.

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
6. **It is still the best fully-fledged answer under the second direction.** Panacea has no
   tracker; Coolify would only host an AGPL or EPL one; Supabase would back one we write. ADO
   Boards and GitHub are complete trackers (boards, sprints, roadmaps, dashboards, mobile and
   Teams apps), already paid for. Panacea shrinks the glue instead of replacing the tracker (§0).

**What it replaces** (counted from the Phase W task tables; the day figures are the plan's):

| Phase W, 190 tasks | Tasks | What happens |
|---|---|---|
| Replaced outright by the tracker | **127** | W1 02-08, 11-17, 25, 37-39, 41-60, 66-69; W2 all but 95, 96, 96b; W3 99, 100, 107, 113; W4 all but 131, 135; W5 143-149, 156, 158-163, 168-170, 172-174; WL 175-178, 182-184, 186-187 |
| Survive, reshaped into the ~17 days of glue | **38** | W0 01; W1 09, 10, 18-24 with 23b, 36, 40, 61-65, 70; W1b 71; W2 96b; W3 98, 101, 103, 104, 108, 109; W4 131, 135; W5 150-155, 157, 165, 171 |
| Not tracker work: move to their owning phase unchanged | **19** | W1 26-35 and W2 95-96 (design-language uplift, `OD-28`); W5 164, 166, 167; WL 179-181, 185 |
| Wait for Phase T's negotiation | **6** | W3 102, 105, 106, 110, 111, 112 |

**Phase T, 69 tasks: honestly, products replace little of it directly.** The tracker plus Teams
take the human-delivery surfaces — the team strip becomes a view over tracker assignment and
session state (53), Desk rows and escalation delivery move to Teams cards (parts of 30, 54), and
tracker audit reaches Sentinel through Microsoft's own ADO and GitHub connectors (part of 45):
**about 5 tasks**, and **about 8 once Panacea's team API, budgets and signed audit head are
counted (§0.4)**. The core — Fleet, the Orchestrator runner, leases, negotiation, the `OD-23`
policy file, the lane mirror (**about 55 tasks**) — has no product anywhere. What the tracker
*does* buy is a **fast team v1**: agents read teammates' in-progress issues and live status
comments through the tracker's MCP server, the platform flags file overlap between sessions as an
*inform* to both people, and the Orchestrator negotiation is built after. That is a scope change
to `OD-16`, so it is an owner call (§8).

## 7. What we still build (glue)

Seven pieces, three lanes (revised for the second direction: pieces 1, 2 and 4 changed, 6 and 7
are new). Days are one engineer's focused days, estimated, not measured.

| # | Piece | Lane | Days | Built on | Surviving Phase W tasks it absorbs |
|---|---|---|---|---|---|
| 1 | **`tracker` interface + two connectors.** ADO: `microsoft/azure-devops-go-api` (**MIT**, read 2026-09-30), WIQL reads, field and state writes, one **live status comment edited in place** per session, a hyperlink back to the session, service hooks in the cloud and **polling in local mode** (`OD-20`: a laptop cannot take a webhook). GitHub: a GitHub App, `go-github`, webhooks, GraphQL for issue fields and Projects. State mapping by **ADO state categories** (Proposed, In Progress, Resolved, Completed, Removed) and GitHub open/closed, so no per-customer mapping screen | A | **5.5** (ADO 3, GitHub 2.5; was 7) | Phase 3's host-interface pattern (`OD-4`); **Panacea snippets** (§0.1): `prcomment` sticky upsert (GitHub half as is, ADO half as pattern), the hook lifecycle in `webhook.go` and `webhook_ado.go` with work-item event types, the receiver check in `webhook-receiver/providers/azuredevops.go` | 150-155, 171 |
| 2 | **The provenance edge and the proof.** A Hub record per linked issue: `{source, externalId, url, serves[{briefKey, criterionId}], implements[design:…]}`; **"create issues from the brief's criteria"** so the thread starts in Define; the done evaluator posts the observed evidence and sets ADO `Done` (through the process rule) or GitHub's `Proof` field. Stopgap: the control plane holds the edge until the Hub's routes land | B | **2** (was 2.5) | Phase H (6/50 built), phase 3's land and evidence, phase 4's brief keys; on GitHub, **Spec Kit's `taskstoissues`** (MIT, adopted in `03-define.md`) creates the issues and we attach the edge; ADO creation stays ours | 01, 09, 10, 18, 19, 21-23b, 71, 96b, 104, 131 |
| 3 | **Start from an issue.** A control-plane route that opens a managed session on its own worktree with the issue text as spec-class text (`OD-8`: never parsed back), posts the first live status within 10 s, tags the gateway token with the issue id; `Shift+R` in our cockpit, or Orca's *open a worktree from any task* for GitHub and Jira (§0.3). Sandboxed agents get the **ADO local MCP server** (MIT) or **GitHub MCP server** (MIT) with a **read-only**, per-session token: the platform writes facts, agents do not | B | **2** | Phase 2's control plane (built), phase 6's per-session token | 24, 61, 98, 101, 103, 108, 109, 135 |
| 4 | **The Work lens v0 and the team view.** Issues grouped by criterion (the plan's rule), one agent line, Start, and a deep link to the tracker for everything else; a team view that joins tracker assignee, branch and PR state, and live sessions, with files touched from each worktree's `git status` (mechanical, `OD-8`-safe); **the roster from Panacea's team API** | C | **2.5** | Phase 2's desktop, or Orca panels if Orca is the desktop (§0.3); Panacea's `GET /api/v1/teams/{name}/members` | 20, 36, 40, 62-65, 70; Phase T 53 (view) |
| 5 | **The Teams bot.** Microsoft 365 Agents SDK (**MIT**): *inform* cards, *decide* cards with `Action.Execute`, overlap warnings to both people, a digest at most every 15 minutes; plus configuring the first-party ADO and GitHub Teams apps for channel notifications | C | **2** | Azure Bot Service in our subscription; the dedupe from Panacea's `NotifyOnce` | 157, 165; Phase T parts of 30, 54 |
| 6 | **Preview comment to issue (new).** Panacea's preview comments, adopted as is, emit `preview_comment` through a webhook channel; our receiver files it under the lane's work item or issue, with a link back to the pinned element and the thread's `serves` edge | B | **1** | Panacea `handlers/preview_comments.go` and `handlers/notifications.go`; piece 1's connectors | W5 intake, reshaped |
| 7 | **ADO Boards source in Orca (new, only if Orca is the desktop).** A WIQL list, the work item view and *open a worktree* in Orca's task drawer, as a fork patch offered upstream | C | **2** | Orca (**MIT**); piece 1's ADO client | — |

**Total: 17 days** (15 without piece 7). Lane A (connectors) 5.5, lane B (proof, Start, preview
comments) 5, lane C (lens, team view, Teams, Orca's ADO source) 6.5. **Critical path: lane A's ADO
half (3 days) beside lane B's pieces 2 and 3 (4 days), so about 5 working days with two builders
in parallel, is the first end-to-end demo:** a brief criterion becomes an ADO work item; `Shift+R` starts a sandboxed
agent; a PR lands; the watched test passes at the landed commit; the work item moves to `Done`
by rule, with the evidence linked; an *inform* arrives in Teams.

**Adopted as-is, zero build:** the ADO and GitHub Teams apps (notifications), the Microsoft
Sentinel **Azure DevOps Audit Logs** and **GitHub Enterprise Audit Log** connectors
([MS docs](https://learn.microsoft.com/en-us/azure/sentinel/data-connectors/github-enterprise-audit-log))
for tracker audit, and the trackers' own boards, sprints, roadmaps, dashboards and search.

**Also adopted as-is from Panacea (§0.1):** preview comments on every preview and staging page,
and the team model (members, invites, join requests, team budgets) behind its API.

**Next, not in the 17 days:** a Jira connector (3-4 days, §3.2) when a customer brings Jira, whose
UI side Orca already has; a Linear connector (2-3 days, §3.1) for a customer whose data class
allows US or EU hosting, again with Orca's UI already there; NATS for Phase T's Orchestrator bus
when TM1-α is built; Supabase Realtime for the team feed only if the Hub moves to Supabase
(§0.2).

## 8. Owner calls

Only these need the owner; everything else in this document is a technical call already made.

1. **Retire `OD-26`'s own tracker (customer-visible).** PanaceaApp stops building Linear
   parity-plus. Issues live in the customer's ADO or GitHub (Jira next); PanaceaApp shows a thin
   Work lens grouped by criterion and deep-links into the tracker for editing, planning and
   reporting. Backlog grooming happens at ADO/GitHub speed, not Linear speed. **This reverses
   `OD-26` and, for Phase W, `OD-35`** (2026-09-28: *"build the full plan; no scope cut"*).
   Today's "third party … ASAP" direction points this way but names neither, so it needs an
   explicit yes.
2. **How "done means proven" (`OD-27`) reads in someone else's tracker (customer-visible).** On
   ADO, a process rule lets only PanaceaApp's proof identity move a work item to `Done`; that
   adds one rule and two fields to the customer's process and needs a process administrator. On
   GitHub, people can still close an issue; PanaceaApp shows it as *done, not proven* and our
   `Proof` field stays empty.
3. **A PanaceaApp Teams app in KPMG's tenant, then each customer's (customer-visible, IT
   consent).** Custom Teams apps need tenant-admin approval. Without it, escalations fall back to
   the cockpit's Desk and the first-party ADO/GitHub Teams apps.
4. **Team mode v1 on the tracker, Orchestrator negotiation after (customer-visible, changes
   `OD-16`'s order, not its end state).** v1: agents read teammates' in-progress issues and live
   status through MCP; the platform detects file overlap and sends an *inform* to both people.
   The Orchestrator-to-Orchestrator settlement follows as Phase T proper.
5. **PanaceaApp teams are Panacea teams (customer-visible; added by the second direction).** One
   team list across both products: joining, inviting, removing and creating a team go through
   Panacea's existing flow, including its cost-centre-owner and SuperAdmin approval for a new
   team (`team_create_approval.go`). The alternative is a second team model in the Hub, which the
   lift-in (`OD-13`) would later have to merge.

**Money: no new spend is needed.** ADO and GitHub are already licensed (an ADO service
principal may take one Basic seat, $6/month). Panacea's parts, Orca (MIT) and the Teams SDK (MIT)
cost nothing new. A Jira customer pays for its own Atlassian Guard; a Linear customer for its own
Linear. Buying Linear for KPMG is **not recommended** (no AU region, a second tracker), and
Supabase Cloud is not needed (a new processor, on AWS).

### Risks we accept with this recommendation

| Risk | Why it is acceptable, or what contains it |
|---|---|
| The "Linear-class speed" wedge leaves the tracker | Our speed claim narrows to the agent loop: Start, a first sign in 10 s, a PR, proof. That is where cube.computer's lesson points anyway |
| Microsoft and GitHub ship the same trigger (Agent HQ, Boards → Copilot) | The trigger was never the product. Ours runs in the customer's Kata sandbox, through the AU-capable gateway, and closes on observed evidence; theirs cannot say that. Watch it quarterly |
| Residency is not total | ADO keeps profile data for users outside US/EU/UK in the US; GitHub AU residency needs a GHE.com enterprise (KPMG's is unverified). Work item and issue content stays in AU |
| Customer processes vary | State categories (ADO) and open/closed (GitHub) avoid a mapping screen; a customer with exotic states gets a one-line override, not a wizard |
| The Hub is 6/50 built | The edge is small; the control plane holds it until the Hub's route lands, then one migration |
| Remote ADO MCP does not accept Claude Code yet | Ship the local ADO MCP server (MIT) in the sandbox image; agents are read-only on the tracker anyway |
| PanaceaApp's teams depend on Panacea's API before the lift-in | Behind one small `roster` interface; sign-in already shares Panacea's realm (`OD-30`), so the dependency is not new |
| Orca ships daily and our ADO source is a fork patch | Offer it upstream; keep it in one module. If upstream declines, the rebase cost per release is unknown |
| Preview comments ride Panacea's shared parent-domain SSO cookie (`preview_comments.go:21-25`) | Works while previews are on Panacea's domain (phase 7); a customer-hosted estate needs the same domain arrangement |

## 9. What was not verified

Nothing in this document has been installed, built or run. Integration days are estimates.

**Read and confirmed on 2026-09-30:** licence files for `microsoft/azure-devops-mcp`,
`github/github-mcp-server`, `microsoft/azure-devops-go-api`, `microsoft/Agents-for-js` (all
MIT), `BloopAI/vibe-kanban` (Apache-2.0), `kaleidos-ventures/taiga-back` (MPL-2.0) and
`Dicklesworthstone/mcp_agent_mail` (MIT plus a rider), via `raw.githubusercontent.com`; vendor
pages for Linear's regions, Plane's editions and prices, the ADO remote MCP GA post, Agent HQ,
Jira agents, Atlassian Data Center end of life, Teams connector retirement.

**Not verified:**

1. **The GitHub API was rate-limited** for this pass (the shared corporate egress IP had spent
   the unauthenticated quota), so stars and last-push dates come from Glasses (observed
   2026-09-29: Plane ~60k, beads ~27.5k) or the 2026-09-28 study (Huly, OpenProject, Taiga), not
   re-read.
2. **KPMG's own tenancy facts:** which geography KPMG's ADO organisation is in; whether KPMG's
   GitHub EMU enterprise is on github.com or an AU GHE.com subdomain; whether KPMG runs Jira at
   all; whether an ADO process administrator will add the `Done` rule and fields.
3. **Whether our own platform can be an issue assignee on GitHub.** Agent HQ documents only
   Copilot, Claude and Codex; a custom agent as assignee is not documented, so the GitHub trigger
   is Start from our cockpit, a label, or a Projects field.
4. **Jira's `rovo:agentConnector` is Preview** and was not tried.
5. **Prices** for Linear Enterprise, Jira and Atlassian Guard come from vendor list pages or
   third-party roundups, not quotes; KPMG's GitHub contract price is unknown.
6. **Whether the ADO remote MCP server has gained Entra dynamic client registration since its
   GA post**; the plan assumes the local server.
7. **Whether ADO comment edit-in-place and GitHub comment edits stay inside each API's rate
   limits at one update per session per state change** — expected yes, not measured.
8. **Plane's agent features and its Helm chart** were not read; neither changes the verdict.

**Added on the relaunch (second direction), read and confirmed 2026-09-30:** Panacea's tracked
files through `git grep`, read-only (every path cited in §0.1); `LICENSE` files for
`supabase/realtime`, `supabase/supabase` (Apache-2.0), `supabase/auth` (MIT),
`coollabsio/coolify` (Apache-2.0) and `stablyai/orca` (MIT) via raw.githubusercontent.com; Orca's
README and its `docs/review/linear` and `docs/notifications` pages; Supabase's Presence and
third-party-auth pages; Claude Code's agent-teams page.

**Not verified on the relaunch:**

9. **Orca's source was not read** (the GitHub API was rate-limited again): whether its task
   drawer has an internal provider seam that makes the ADO source 2 days, and how it stores the
   Linear token.
10. **Panacea's team API as a PanaceaApp caller**: the team routes run `selfService`, which is
    Panacea's `pnc_` API-key check (narrowed to the owner's live Keycloak groups), its SSO auth
    and its audit (`routes.go:3162-3182, 3349-3355`), so a user's `pnc_` token works. Whether our
    own Keycloak access token is accepted as a bearer, and whether the realm can put group
    membership in a claim for our client, were not read.
11. **The snippets were read, not run**: the sticky-comment, hook-lifecycle and hook-check code
    is quoted by path; its behaviour against ADO work-item comments, and OpenBao's transit API
    for the signed head, are expected to match and were not tried.
12. **Supabase with Keycloak tokens**: the docs ask for asymmetric JWTs with a `kid` from an OIDC
    issuer; Keycloak's default signing should fit, not tried. The Sydney region comes from a
    search summary of Supabase's regions page, not the page itself.
13. **`00-panacea.md` was a skeleton** while this revision was written; §0.1 rests on this
    agent's own read of Panacea and should be cross-checked against that inventory when it lands.
