# 05 — Work layer and team collaboration: the sceptic's verification

**Status:** COMPLETE (2026-09-30).
**Date:** 2026-09-30. **Verifies:** `05-work-team.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation (no tracker of our own: Azure DevOps Boards
first and GitHub Issues/Projects second behind one `tracker` interface, Teams through the
first-party apps plus one Microsoft 365 Agents SDK bot, Panacea's team roster and connector
snippets, Orca's task sources) and its two best alternatives at source: licence, the vendor's
own docs, paid-tier gates, maintenance, and whether the days fit OUR architecture (Go control
plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run.

## 0. Verdict first

**UPHELD, with corrections.** Do not build a tracker: Azure DevOps Boards first, GitHub
Issues/Projects behind the same `tracker` interface, Teams for *inform* and *decide*, Panacea's
code and preview comments, Orca as the desktop.

- **Why it survives:** every licence is what the researcher said, at source (MIT for
  `azure-devops-mcp`, `azure-devops-go-api`, `github-mcp-server`, Orca and `Agents-for-js`).
  Australia is a real ADO geography. Panacea really has no work items. And both alternatives fail
  on merit: Linear has no AU region; Plane is a third tracker we would run forever.
- **What does not survive:**
  1. **The Done rule as written would block everyone.** ADO's group rule takes one condition and
     lands on every project using the process. The fix is two fields and three standard rules,
     which also need a PanaceaApp service identity that does not exist yet (section 2).
  2. **GitHub is not AU-resident for KPMG.** Its EMU is on github.com, not GHE.com, and any
     triager can set a Proof field by hand (section 3).
  3. **Panacea's roster is admin-only.** `GET /teams/{name}/members` checks team-admin first, so a
     member's `pnc_` token gets no roster (section 4).
  4. **No Orca fork.** `02-fleet-cockpit.md` rules Orca unforked, and a plugin cannot call ADO (no
     `net:fetch`). ADO Start goes through the recipe's pending-start path instead (section 5).
  5. **17 days becomes about 20 (17 to 23),** plus day-0 admin asks on four fronts that nobody
     counted (section 8).
- **In our favour:** a managed identity can be a free ADO Stakeholder. Orca now also reads GitLab
  issues and Jira Server/DC. Jira's remote agents page carries no Preview label.
  `azure-devops-mcp` shipped `v2.10.0` on 2026-09-09.
- **Alternatives:** Linear stays rejected as the default (no AU region). Plane Commercial stays
  rejected as the default, but not for its licence: the commercial edition can be bought, has
  OIDC on Pro+ self-hosted, and would be AU-resident on our AKS. It is the fallback if KPMG's ADO
  organisation turns out not to be in Australia (section 7).
- **One addition:** a PanaceaApp-administered ADO organisation in Australia, for v1 and for
  anyone with no tracker (section 9).

## 1. What was checked, and how

All reads on 2026-09-30. Nothing installed, built or run; no tenant, organisation or repository
touched.

- **The report under test:** `05-work-team.md`, all 549 lines.
- **At source, GitHub:** `LICENSE` files on raw.githubusercontent.com for
  `microsoft/azure-devops-mcp`, `microsoft/azure-devops-go-api`, `github/github-mcp-server`,
  `stablyai/orca` and `microsoft/Agents-for-js`. Through the GitHub API (rate-limited by the shared
  corporate IP for the first hour, then answered): repository metadata and the latest releases of
  those five, and the licences of `makeplane/plane`, `supabase/supabase`, `supabase/realtime` and
  `coollabsio/coolify`. Orca's README and the imports of three recipe source files. Spec Kit's
  `templates/commands/taskstoissues.md`.
- **Vendor docs:** Microsoft Learn on ADO workflow rules, data location, service hooks and Web
  Hooks, the Work Items Update and Comments Update REST pages, the permissions reference, PATs and
  their lifecycle API, service principals and managed identities, and the Boards app for Teams.
  Also the ADO pricing page, the remote MCP GA post, and the Office 365 connector retirement post.
  GitHub: the issue-fields, Agent HQ and AU-residency changelogs, the GHE.com docs, agent apps,
  Teams integration and pricing. Orca's docs (`review/linear`, `review/github`, `review/jira`).
  Linear's security, pricing and agent-interaction pages. Plane's pricing and editions pages.
  Atlassian's remote-agents page.
- **Panacea, read-only** (`git grep` and file reads at `eee94cd94`; nothing written, built or
  run): `routes/routes.go`, `handlers/billing_team.go`, `me.go`, `team_onboarding.go`,
  `team_create_approval.go`, `preview_comments.go`, `notifications.go`, `webhook_ado.go`,
  `panacea-build/pkg/prcomment/client.go`, `webhook-receiver/providers/azuredevops.go` and
  `clients/audit_signed_head.go`.
- **Our own docs:** the owner decisions (`OD-4`, `-16`, `-20`, `-24`, `-26`, `-27`, `-30`, `-35`),
  the identity ruling §7, `00-panacea.md` §5, `02-fleet-cockpit.md` §R2-§R3, `03-define.md`'s
  Define flow, `08-whole-product.md`'s journey shell, the phase 3 plans (grep only), and this
  device's memory note on KPMG's GitHub EMU (2026-07-28).
- **Web searches** (summaries, marked where used): issue-field permissions, Boards to Copilot,
  custom agents as GitHub assignees, and Azure Bot identity types.

## 2. The recommendation, part 1: Azure DevOps Boards as the first connector

**Holds, with two corrections.** Licence, price, AU geography and the MCP facts check out at
source. The Done rule does not work as written, and the identity it needs does not exist yet.

| # | Claim | At source (read 2026-09-30) | Holds? |
|---|---|---|---|
| 1 | Basic $6/user/month after 5 free; Stakeholder free | Pricing page: *"$6 per user per month"*, *"First 5 users free"*; Stakeholder can *"Create and edit work items"* and *"View backlog and boards"*. Also: *"GitHub Enterprise Cloud customers who manage users with Entra ID can access Azure DevOps without additional cost"* | **Yes.** A service identity can be a free Stakeholder |
| 2 | `azure-devops-mcp` and `azure-devops-go-api` are MIT | Both `LICENSE` files: MIT, *"Copyright (c) Microsoft Corporation"* | **Yes.** But the Go client's newest release is `azuredevops/v7.1.0` of **2023-04-17** (GitHub API). It targets API 7.1, still the current GA version. Low activity: Panacea's snippets call REST with plain `net/http`, the safer base |
| 3 | Australia is a selectable geography | Data-location page (updated 2026-03-17) lists Australia; customer data *"includes source code, work items"* | **Yes** |
| 4 | Profile data of users outside US/EU/UK is stored in the US | Same page: *"All other countries and regions"* → US | **Yes, and two more US items were missed.** *"Token data, such as personal access tokens and SSH keys, in a US datacenter"*: our only ADO path is a per-user PAT (identity ruling §7). And the remote MCP server *"runs instances in both Europe and the United States"* (pass-through, no storage) |
| 5 | Remote MCP GA 2026-08-05; Claude Code cannot connect yet | GA post: Claude Code needs *"dynamic OAuth client registration or client ID Metadata documents in Microsoft Entra"*; until then *"continue using the local Azure DevOps MCP Server"* | **Yes.** For an AU-only project the local server stays the answer even after that lands (row 4) |
| 6 | A process rule restricts Done to our proof identity | Rules page (updated 2026-07-23): the group rule takes *"one condition based on user or group membership"* and *"one action to restrict a transition"*; *"any projects that use the process automatically reflect the customizations"* | **Not as written.** See below |
| 7 | So the tracker enforces done-means-proven | Rules apply to *"REST API: Add or modify work items"*. But `bypassRules=true` means *"Do not enforce the work item type rules"*, for holders of the project-level *Bypass rules on work item updates* permission | **Partly.** Enforced against members, not against whoever holds bypass |
| 8 | Service hooks for created, updated, commented | `vso.work_write` covers *"notifications about work item events via service hooks"*. But *"By default, only project administrators"* can edit subscriptions, and *"you must allow specific IP address ranges for inbound connections"* | **Yes,** with a project administrator per project and an edge rule |

**Row 6: why it fails, and the fix.** A group rule has exactly one condition, so it cannot say
"only items PanaceaApp works". On an inherited process it lands on every work item of that type in
every project that uses the process. No person could move *any* User Story to Done, including work
PanaceaApp never touched. The fix is the docs' own *Approved By* example (standard rules take
*"one or two conditions"* and up to 10 actions), with two custom fields per work item type:

1. *When the state changes to Done* and *`PanaceaApp.Linked` = True* → *make `PanaceaApp.Proof` required*.
2. *Current user is not a member of group PanaceaApp Proof* → *make `PanaceaApp.Proof` read-only*,
   and the same for `PanaceaApp.Linked`, so nobody unlinks an item to escape rule 1.

A linked item then reaches Done only once the proof identity has written Proof; unlinked work is
untouched. A dedicated inherited process for PanaceaApp projects also works. Either way the
cockpit still checks Proof against our own evidence record, because of row 7. The two-condition
combination was read from the docs, not tried.

**The proof identity does not exist yet.** `OD-30`'s ruling makes ADO a per-user PAT with
*Code (Read & write)*. Two consequences:

- The PAT must add **Work Items (Read & write)**. Creating hook subscriptions from it works only
  for a project administrator.
- A non-person identity is an Entra service principal or managed identity added to the ADO
  organisation. A new app registration is the IT ticket `OD-30` exists to avoid. A user-assigned
  managed identity in our own subscription needs no IT, but needs an ADO organisation
  administrator to add it, and our subscription must sit in the tenant behind KPMG's ADO
  organisation (not verified). Writing Proof with the user's own PAT is not enforcement: that user
  could write Proof by hand.

**Days: 3 is fair for the code. The calendar is gated by four asks outside the team:** a process
administrator (fields and rules, per process and work item type), an organisation administrator
(the proof identity), a project administrator per project (hooks), and an edge rule for ADO's
service-hook ranges. Polling in the cloud as well as in local mode removes the last two for the
first demo.

**Not re-read at source:** *"Boards to GitHub Copilot is GA"*. A search result says preview with GA
planned, and custom agents arrived in sprint 269. It is a competitor fact and changes nothing here.

## 3. The recommendation, part 2: GitHub Issues and Projects as the second connector

**Holds as the second connector. Its AU claim does not hold for KPMG today, and its Proof field
is not protected.**

| # | Claim | At source (read 2026-09-30) | Holds? |
|---|---|---|---|
| 1 | `github/github-mcp-server` is MIT | `LICENSE`: MIT, *"Copyright (c) 2025 GitHub"* | **Yes** |
| 2 | Enterprise about $21/user/month list | Pricing page: *"$21 USD per user/month for the first 12 months"*; EMU and data residency are Enterprise-only | **Yes** (list price; KPMG's contract unknown) |
| 3 | Issue fields GA 2026-07-02 | Changelog: GA *"July 2, 2026"* on *"Free, Team, Enterprise, and GitHub Enterprise Cloud with data residency"*, plus GHES 3.23. Types: single-select, text, number, date. Changes show in the timeline | **Yes** |
| 4 | "Proven" becomes an issue field only our app writes | Org admins define fields. A search summary of GitHub's docs says people with triage access or greater set values; the docs page names no way to limit edits to a user, team or app | **No.** Any triager can type a Proof value. The cockpit must count Proof only when it matches our own evidence record, and show a hand-set value as *claimed, not proven* |
| 5 | GHE.com data residency in Australia GA since 2025-02-04 | Changelog of that date. Docs: *"hosted on a dedicated subdomain of GHE.com"*, EMU required, regions *"EU ..., Australia, US, Japan"*; *"By default, GitHub stores data for GitHub.com in the USA"* | **Yes,** as a product fact |
| 6 | Whether KPMG's EMU is on an AU GHE.com subdomain is unverified | This device's `gh` is signed in to **github.com** as an EMU account of the org `kpmg-au-emu` (memory note of 2026-07-28, not re-run today) | **Resolved against the claim.** KPMG AU's enterprise is on github.com, so its issues are US-hosted today. AU needs a new GHE.com enterprise and a migration: a KPMG IT programme, not ours |
| 7 | Agent HQ assigns issues to Copilot, Claude or Codex; our own agent as an assignee is undocumented | Changelog 2026-02-04: *"for Copilot Pro+ and Copilot Enterprise customers"*. GitHub's *agent apps* doc (public preview): *"a GitHub App that a partner has configured to act as an agent"*, *"powered by your Copilot subscription"* | **Yes, and sharper.** A partner can now make an assignable agent, but it runs as Copilot, outside our sandbox and our gateway. Our trigger stays Start, a label or a Projects field |
| 8 | Cannot restrict who closes an issue | GitHub's permission model: triage and above close, authors close their own. Not re-read | **Yes** |

**What changes.**

- *"AU residency on both trackers"* becomes: **AU residency on ADO; GitHub serves projects classed
  Global until KPMG moves to GHE.com.** ADO-first is then a residency requirement for `OD-24`
  AU-only projects, not a preference.
- **Days: 2.5 is fair for the code,** because Panacea's comment upsert and hook lifecycle exist
  (section 4). Day 0 adds one ask: an owner of `kpmg-au-emu` creates and installs the GitHub App
  and the org-level Proof field (the memory note records this account as a member, not an owner).
  Webhooks also need an edge rule for GitHub's hook ranges, or polling.

## 4. The recommendation, part 3: Panacea's roster, snippets and preview comments

**Every path the researcher cites exists** (read-only `git grep` and file reads at Panacea
`eee94cd94`, 2026-09-30 11:03 +1000). **Two of the uses do not work as described:** the roster,
and preview comment to issue.

| # | Claim | At source | Holds? |
|---|---|---|---|
| 1 | Panacea has no work items | `git grep -i "workitem\|_apis/wit\|wiql"` over `services/`: no hits | **Yes** |
| 2 | The roster: `GET /api/v1/teams/{name}/members` with the user's `pnc_` token | The route exists (`routes.go:1962`, `selfService`). Its handler opens with `h.authorizeTeamAdmin(w, r)` (`handlers/billing_team.go:530-533`): platform admins administer every team, *"everyone else administers exactly the teams whose configured TeamAdmins list matches their identity"* (`:94-104`). `GET /api/v1/me` returns only the caller's own groups (`me.go:44-51`) | **No, for most users.** A member who is not the team's admin gets no roster. The team view needs one Panacea-side change: a member-readable roster route, or a Keycloak role for our realm client that can read group members (a realm change, so a Panacea deploy, `00-panacea.md` §5.1) |
| 3 | The sticky comment upsert ports, GitHub half as is | `prcomment/client.go:54` switches github, gitea and azuredevops. The GitHub list is `https://api.github.com/repos/%s/issues/%d/comments` (`:78-80`), with no `per_page` and no `Link` handling | **Ports with one fix.** GitHub returns 30 comments a page by default. On an issue that already has 30 or more comments the sticky is never found, and a new one is posted at every update. Keep the comment id in the edge record, or paginate. The ADO half is PR threads (`api-version=7.0`, `:186-209`), so work-item comments are new code, as the researcher said |
| 4 | The hook lifecycle ports with work-item events | `webhook_ado.go:17-42`: Service Hooks REST, `publisherId=tfs`, `consumerId=webHooks`, create/list/delete matched on the delivery URL; the base URL can be overridden (Azure DevOps Server) | **Yes, as a pattern.** Its create call resolves repository GUIDs for `git.push` and `git.pullrequest.created`; work-item events take other publisher inputs |
| 5 | The ADO hook check | `webhook-receiver/providers/azuredevops.go:31-75`: an empty secret is rejected; the Basic password is compared in constant time | **Yes.** ADO's Web Hooks page describes Basic authentication over HTTPS and no payload signature, so the `X-Azure-Signature` branch never runs for ADO; Basic is the check |
| 6 | The signed audit chain head | `clients/audit_signed_head.go:10-18`: Vault-transit-signed head anchors, append-only | **Yes** (OpenBao's transit API match stays unverified) |
| 7 | Preview comments as is, plus 1 day to file one as an issue | Element-pinned threads, a widget on the tenant subdomain, the shared parent-domain SSO cookie (`preview_comments.go:19-32`). The event is `Notify(project, "preview_comment", "<author> commented on <host> (PR #n)")` (`:481-495`). The webhook channel posts `{event, message, project}` with a plain `http.Post` and no signature (`notifications.go:294-305`) | **Partly.** The event carries no comment id, body, element or thread, and it is unsigned. The receiver must treat it as a hint, then read `GET /api/v1/projects/{name}/comments` (`routes.go:2932`, `authedProject`) with a member's `pnc_` and find the new comment. **About 1.5 days, not 1** |
| 8 | SMTP is not configured; no Teams channel | `team_create_approval.go:17-18` says so; channel types are slack, email and webhook (`notifications.go:39, 218-223`) | **Yes** |

**Trade-off, restated in one line:** Panacea's team model is the right one to reuse, because KPMG
users already belong to it, but today it is an admin surface, not a directory; opening it to every
member costs one Panacea change, the same kind as the realm-token trust `00-panacea.md` §5.3
already ranks first.

## 5. The recommendation, part 4: Orca task sources and the ADO fork patch

**Orca holds. "Start for GitHub and Jira for free" is unverified, and "a 2-day fork patch" does not
survive `02-fleet-cockpit.md`'s own ruling.** Not re-argued: Orca is the owner's pick.

| # | Claim | At source (read 2026-09-30) | Holds? |
|---|---|---|---|
| 1 | MIT, "Copyright 2026 Lovecast Inc." | `LICENSE` read. GitHub API: MIT, 81,654 stars, TypeScript, pushed 2026-09-30; releases `v1.4.216` (09-28) and `v1.4.217` (09-29) | **Yes.** It ships daily |
| 2 | Browse PRs, issues and boards; open a worktree from any task | README: *"Browse PRs, issues, and project boards in-app — open a worktree from any task"*. GitHub docs page: a GitHub Projects view, *"create a worktree from any card"*; from an issue it *"opens the interactive workspace composer"* and *"links the issue"* | **Yes** |
| 3 | Sources are GitHub, Linear and Jira; no ADO, no GitLab | GitHub docs page: *"browse, filter, and edit GitHub and GitLab issues"*. Jira page: *"Jira Cloud and self-hosted Server/Data Center"*. ADO: *"Azure DevOps and Gitea pull requests appear in the worktree sidebar"*; no ADO work items | **Mostly.** GitLab issues exist, and Jira Server/DC works; ADO work items do not |
| 4 | No documented plugin point for another source | Plugin API v1 (02 §R2, read from source): panels, commands, events, `vmRecipes`, but a closed capability set, and *"scoped `net:fetch` and `process:exec` 'arrive in later phases'"*; hosted-review providers *"are built in, not pluggable"* | **Yes.** It is also why an ADO source cannot be a plugin today: a panel cannot call ADO |
| 5 | Start for GitHub and Jira comes free | A task-linked worktree opens the composer. 02 §R3 makes a sandbox lane only through the *Run on: Panacea sandbox* recipe; the recipe inputs 02 read are the repo URL, ref and branch; and *"Orca's CLI cannot create a recipe workspace"* | **Unverified; probably not free.** Not checked: whether the composer offers our recipe for a task-linked workspace, and whether the issue link reaches the recipe's `create` script. Our live status comment, token tag and provenance edge all need that link. Budget 0.5 to 1 day after a day-1 spike |
| 6 | An ADO Boards source as a fork patch, 2 days | 02 §R3 rules Orca *"unforked"*, because a fork means *"our own Windows code signing ..., a fresh Defender prevalence reset on every build we ship, and a daily rebase"*; it names this patch as the first fork candidate, *"offered upstream as a pull request first"* | **No.** The 2 days build the patch. Carrying it means shipping our own Orca build, which 02 rejects. Offer it upstream (the same 2 days, off the critical path) and ship nothing forked |

**ADO Start without a fork: 1 day.** 02 §R3 already has the pattern, for fork: *"mark the fork ...,
then create a workspace with 'Run on: Panacea sandbox'; the recipe's `create` picks up the pending
fork"*. Start from a work item does the same:

1. A Start button on the Hub's Work page, or a link on the work item, marks a pending start.
2. The user creates a workspace with *Run on: Panacea sandbox*.
3. The recipe's `create` claims the pending start, with its work item id.

The same path covers GitHub and Jira if the spike in row 5 finds that the composer drops the link.

**The Work lens moves too.** Researcher piece 4 puts it in *"Orca panels instead of Tauri views, the
same 2.5 days"*. With no `net:fetch`, a panel cannot read the tracker or the Hub. 02 §R3's fallback
applies: a Hub page in Orca's browser tab, which is also `08-whole-product.md`'s journey shell. The
2.5 days stand, but they build a Hub web page, and the Hub is 6/50 built.

**Maintenance of the other parts, from the GitHub API:** `azure-devops-mcp` `v2.10.0` (2026-09-09),
`github-mcp-server` `v1.12.2` (2026-09-16) and `Agents-for-js` `v1.9` (2026-09-29) are active.
`azure-devops-go-api`'s newest release is **`azuredevops/v7.1.0` of 2023-04-17**, last push
2026-06-10: build on REST, as Panacea does (section 2 row 2).

## 6. The Teams surface: first-party apps plus a Microsoft 365 Agents SDK bot

**Holds. The bot brings a runtime and an identity that the 2 days did not count.**

| # | Claim | At source (read 2026-09-30) | Holds? |
|---|---|---|---|
| 1 | The Agents SDK is MIT | `microsoft/Agents-for-js` `LICENSE`: MIT. GitHub API: `v1.9` on 2026-09-29, TypeScript | **Yes,** and active |
| 2 | The ADO and GitHub Teams apps carry tracker notifications as is, 0 days | Boards app page (updated 2026-05-08): channel subscriptions, create a work item from a message, link previews. But *"Azure Boards notifications aren't supported inside Teams chat or direct messages"*, subscriptions need a Project or Team Administrator, and it is *"only available on Azure DevOps Services"*. GitHub's page: *"an admin for your Teams workspace must have installed the GitHub app"* | **Yes, for channels.** Configuration only, but by an admin per channel, and never as a DM |
| 3 | Office 365 connector webhooks retired in May 2026 | Microsoft's post (updated 2026-04-14): rollout *"completes: May 22, 2026"*; the replacement is Workflows webhooks | **Yes** |
| 4 | Our bot on the Agents SDK and Azure Bot Service, 2 days | Multi-tenant bot creation ended after 2025-07-31; new bots are single-tenant or a user-assigned managed identity, and those types are supported in the C#, JavaScript and Python SDKs only (search summary of Microsoft's pages, not the pages themselves) | **Partly.** See below |
| 5 | A custom Teams app needs tenant-admin approval (owner call 3) | Standard Teams behaviour, not re-read: an admin allows the custom app, and a proactive DM needs the app installed for that user (an admin setup policy can pre-install it) | **Yes,** as the researcher said |

**Row 4, what the 2 days miss.**

- **A second runtime.** There is no Go SDK, so the bot is a Node service beside our Go control
  plane, to build, sign, deploy and patch.
- **An identity.** A single-tenant bot needs an app registration: the IT ticket `OD-30` avoided.
  A user-assigned managed identity in our subscription avoids IT, if the subscription sits in
  KPMG's tenant (not verified; the same question as the ADO proof identity, section 2).
- **Customers.** A single-tenant bot reaches another tenant's users only through the Teams store
  or guest accounts. In the customer-hosted tier, each customer tenant runs its own bot instance.

**Days: about 3, not 2.** The calendar waits on the Teams admin (owner call 3). Until then the Desk
in the cockpit and the first-party channel apps carry *inform*; *decide* waits for the bot.

## 7. The two best alternatives, tested: Linear, and Plane on our own AKS

Both would give PanaceaApp a tracker of its own, which is what `OD-26` asked for. **Neither
survives as the default,** and one of the researcher's reasons for rejecting Plane is wrong.

**Linear, bought as SaaS: rejected as the default, for residency. The researcher's verdict holds.**

| Check | At source (read 2026-09-30) | Result |
|---|---|---|
| Licence and price | Commercial SaaS. Pricing page: Basic *"$10 per user/month"*, Business *"$16 per user/month"*, Enterprise custom; SCIM and the audit log are Enterprise-only | As claimed |
| AU region | Security page: *"United States"* and *"European Union"*, chosen *"When creating a new workspace"*; the choice *"isn't self-serve to change later"* | **No AU region.** Fails `OD-24` for AU-only projects |
| Agent model | Agent-interaction docs: six session states, five activity kinds; an agent must respond *"within 10 seconds to avoid the session being marked as unresponsive"*; triggered by @mention or delegation | As claimed. The best agent contract of the four trackers |
| Fit | A third tracker for KPMG staff; a new US or EU processor; no customer-cloud option | **Reject as default.** Keep "adopt its session contract" |

**Plane Commercial, self-hosted in Australia East: rejected as the default, but not for its licence.**

| Check | At source (read 2026-09-30) | Result |
|---|---|---|
| Licence | GitHub API: `makeplane/plane` is **AGPL-3.0** (the Community edition). Editions page: Commercial and Airgapped are *"Closed-source"* | Community is RESTRICTED: serving a modified copy to users over a network forces us to offer them our source, and installing it in a customer tenancy is distribution. **Commercial is a product KPMG can buy, which the licence rule allows** |
| Price | Pricing page: Pro *"$6/year"* per seat per month billed annually ($8 monthly), Business $13 annually; *"Pricing identical"* for cloud and self-hosted; 12 users free | About $6-13 per seat per month |
| SSO | OIDC and SAML on *"Pro+ (self-hosted)"* | Keycloak brokering Entra fits |
| Residency | Self-hosted on our AKS in Australia East: total. Plane Cloud names no AU region | **Better than GitHub is for KPMG today** (section 3 row 6) |
| Maintenance | GitHub API: 60,125 stars, pushed 2026-09-29 | Active |
| Fit | A third tracker; we run it (backups, upgrades, on-call); no criterion concept; how its AI features reach a model was not checked | **Reject as the default,** on product and operations grounds |

Plane stays the fallback for one case: **an AU-only project where no AU-resident tracker exists**.
That case turns on a fact nobody has checked: the geography of KPMG's ADO organisation. If it is
not Australia, the AU answer is a new ADO organisation in Australia, or Plane Commercial in
Australia East.

**Also re-checked, one line each:**

- **Jira Cloud (third connector).** The remote-agents page (`rovo:agentConnector`) says agents
  *"can be assigned work items, @mentioned in comments"*, and carries no Preview label today (the
  researcher read Preview). If it is GA, Jira is the one tracker where PanaceaApp can be a native
  assignee. KPMG's own Jira use is still unverified. It stays "when a customer brings it".
- **Coolify plus Supabase.** Apache-2.0 at source (`supabase/supabase`, `supabase/realtime`,
  `coollabsio/coolify`, GitHub API). Neither is a tracker. Supabase would back a tracker we write,
  which is Phase W again; Coolify runs Docker hosts, not Kata. The researcher's verdicts stand.

## 8. The integration days, against our architecture

**17 becomes about 20 engineer-days (17 to 23); 18 without Orca as the desktop.** Plus 2 optional
days for the upstream ADO task-source pull request, off the critical path. Estimates, nothing run.

| # | Piece (researcher §7) | Researcher | Verified | What changed |
|---|---|---|---|---|
| 1 | `tracker` interface + ADO and GitHub connectors | 5.5 | **6** | ADO +0.5: the two fields and three rules per work item type, scripted through the process REST API, and the proof identity's token (section 2). GitHub: the sticky-comment id is stored, not re-found (section 4 row 3) |
| 2 | Provenance edge and proof | 2 | **2.5** | +0.5: the cockpit counts Proof only when it matches our evidence record, on both trackers (sections 2 and 3) |
| 3 | Start from an issue; tracker MCP in the sandbox | 2 | **2.5** | +0.5 to 1: no per-session ADO token can be minted. Microsoft: the PAT API needs *"a Microsoft Entra access token from a user-delegated flow"*, and *"Service principals and managed identities can't create or manage PATs"*; under `OD-30` we hold no such token. So sandbox reads go through a control-plane proxy that holds the user's PAT. GitHub's read-only installation token works as planned |
| 4 | Work lens v0 and team view | 2.5 | **2.5** | Built as a Hub page in Orca's browser tab, not an Orca panel (no `net:fetch`, section 5). The roster needs a Panacea change (section 4 row 2), done by the Panacea stream |
| 5 | Teams bot | 2 | **3** | A Node service beside the Go control plane, its identity, the Teams manifest (section 6) |
| 6 | Preview comment to issue | 1 | **1.5** | The event is an unsigned hint with no comment id; read the comment back from Panacea (section 4 row 7) |
| 7 | ADO Boards source in Orca, as a fork patch | 2 | **1.5** | Replaced: the pending-start path (1) and the day-1 spike on task links (0.5), both without a fork. The patch itself goes upstream as a PR (2, optional) |
| | **Total** | **17** | **19.5** | 18 without piece 7 |

**The first end-to-end demo on ADO: about 5.5 working days with two builders,** lane A's ADO half
(3.5) beside lane B's pieces 2 and 3 (5). That holds **only in an ADO organisation we administer**,
with the Teams step carried by the first-party Boards app in a channel, not by our bot. Pointed at
a KPMG team's existing organisation, it waits on the day-0 asks below. None of them is in anyone's
day count.

| Who | Ask | Avoidable for v1? |
|---|---|---|
| ADO administrators | Process: two fields and three rules per work item type. Organisation: add the proof managed identity as a Stakeholder. Project: hook subscriptions | Hooks, yes (poll). Fields and rules, only by running in our own organisation |
| KPMG ADO policy owner | Confirm PAT creation is not restricted and users make organisation-scoped PATs: *"All existing global PATs stop working on December 1, 2026"* | No. The whole ADO path is PATs (`OD-30`) |
| `kpmg-au-emu` owner | Create and install the GitHub App; create the org-level Proof field | No, for GitHub |
| Teams administrator | Allow the custom app; the bot's identity | Yes: channel apps and the Desk carry *inform* until then |
| Panacea stream | A member-readable roster route, beside the realm-token trust it already ranks first | Yes: team admins only, until then |

**Fit with our architecture, the rest:** the connectors live in the control plane, not behind the
sandbox `Backend` interface; local mode polls with the PAT from Credential Manager (`OD-6`); the
gateway token's issue tag depends on LiteLLM key metadata (the gateway cluster, not checked here).

## 9. Corrected recommendation

**Do not build a tracker. Azure DevOps Boards is the system of record, and the only one for AU-only
projects; GitHub Issues/Projects sits behind the same `tracker` interface for projects classed
Global. PanaceaApp is the agent the work is handed to and the proof that closes it.** Teams
carries *inform* through the first-party channel apps now and *decide* through our bot once the
Teams admin allows it. Panacea supplies the code snippets, the signed audit head and preview
comments as they are, and the team model once it opens a member-readable roster. Orca stays
unforked.

What changes from the researcher's version:

1. **Done-means-proven on ADO is two fields and three standard rules** (Proof required on Done for
   linked items; Proof and Linked read-only to all but a PanaceaApp Proof group), not one
   group-transition rule, which would block every person in every project on the process. Or a
   dedicated inherited process. The cockpit still checks Proof against our evidence record.
2. **A service identity in ADO:** a user-assigned managed identity in our subscription, added to
   the organisation as a free Stakeholder. It writes Proof and posts the live status comment, so
   the comment reads as PanaceaApp's and not the user's. Users keep their PATs (`OD-30`), now with
   *Work Items (Read & write)* and organisation-scoped.
3. **GitHub is not AU-resident for KPMG today** (its EMU is on github.com). GitHub's Proof field
   is writable by any triager, so a hand-set value shows *claimed, not proven*.
4. **Orca: no fork.** Start for ADO is the pending-start path through the *Run on: Panacea
   sandbox* recipe (1 day); the ADO task source goes upstream as a PR. A day-1 spike checks whether
   task-linked worktrees carry the issue link into the recipe. The Work lens is a Hub page in
   Orca's browser tab.
5. **Sandbox reads of ADO go through a control-plane proxy.** Nothing can mint a per-session ADO
   token. The platform, not the agent, creates issues from the brief on both trackers: Spec Kit's
   `taskstoissues` calls the GitHub MCP server's `issue_write`, which needs a write token in the
   sandbox.
6. **Days: about 20 (17 to 23), 18 without Orca.** The first ADO demo is about 5.5 working days
   **in an ADO organisation we administer**. Otherwise it waits on the day-0 asks (section 8).

**One addition, for the owner's call 1:** give PanaceaApp its own ADO organisation in the
Australia geography, for v1 and for anyone who arrives with no tracker. We would hold every admin
role the Done rule, the proof identity and the hooks need. Residency would be known, not assumed.
And a person from nothing gets a repo and a tracker in one place. Whether KPMG's tenant allows a
new organisation was not checked: Azure DevOps has a tenant policy that can restrict organisation
creation. Nothing in 03, 06 or 08 provisions a repo for a person from nothing today, so that gap
is the whole product's, not this cluster's.

**Trade-off, in one line:** we trade the Linear-class tracker `OD-26` wanted for trackers KPMG
already runs, and pay for it with admin asks instead of engineering days.

## 10. What this verification could not verify

**KPMG's tenancy facts, the hinges:**

1. **The geography of KPMG's ADO organisation.** If it is not Australia, no KPMG tracker is
   AU-resident today, and the AU answer becomes a new ADO organisation in Australia or Plane
   Commercial in Australia East (section 7).
2. **KPMG's ADO policies:** whether PAT creation is restricted, whether users already make
   organisation-scoped PATs (global PATs stop working on 2026-12-01), and whether the tenant
   allows a new ADO organisation.
3. **Whether our Azure subscription sits in the Entra tenant behind KPMG's ADO organisation and
   Teams.** The managed-identity paths in sections 2 and 6 depend on it: *"You can't add a managed
   identity directly from another tenant."*
4. **KPMG's GitHub host:** taken from a memory note of 2026-07-28 (`gh` on github.com, org
   `kpmg-au-emu`). Not re-run today.

**Product behaviour, read in docs and not tried:**

5. ADO's two-condition standard rule (*state changes to Done* and *Linked = True*); which work
   item types and processes KPMG's projects use; who holds *Bypass rules on work item updates*.
6. GitHub issue-field write permission: "triage or greater" comes from a search summary. The docs
   page names no restriction mechanism, and none was found.
7. Orca: whether the workspace composer offers our recipe for a task-linked worktree, and whether
   the task link reaches the recipe's `create` script or a plugin event. Only three source files'
   imports were read; `02-fleet-cockpit.md` read the tree at `59b746ff`.
8. Whether the local `azure-devops-mcp` server can point at a control-plane proxy (a base URL and
   auth override). If it cannot, sandbox agents read the tracker through a small CLI instead.
9. Jira's remote agents: the page carries no Preview label; GA was not confirmed in a changelog.
10. Azure Bot identity types and SDK language support come from a search summary; Boards to
    Copilot's GA status likewise.

**Carried over from the researcher, still open:** Panacea's snippets were read, not run; OpenBao's
transit API match for the signed head; LiteLLM key metadata for the issue tag (the gateway
cluster); prices are list prices, not quotes.

## 11. Sources

All read 2026-09-30.

- **Microsoft Learn (Azure DevOps):**
  [workflow rules](https://learn.microsoft.com/en-us/azure/devops/organizations/settings/work/apply-rules-to-workflow-states?view=azure-devops) ·
  [data location](https://learn.microsoft.com/en-us/azure/devops/organizations/security/data-location?view=azure-devops) ·
  [service hooks](https://learn.microsoft.com/en-us/azure/devops/service-hooks/overview?view=azure-devops) ·
  [Web Hooks](https://learn.microsoft.com/en-us/azure/devops/service-hooks/services/webhooks?view=azure-devops) ·
  [Work Items Update](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/update?view=azure-devops-rest-7.1) ·
  [Comments Update](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/update-work-item-comment?view=azure-devops-rest-7.1) ·
  [permissions](https://learn.microsoft.com/en-us/azure/devops/organizations/security/permissions?view=azure-devops) ·
  [PATs](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops) ·
  [service principals and managed identities](https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/service-principal-managed-identity?view=azure-devops) ·
  [Boards in Teams](https://learn.microsoft.com/en-us/azure/devops/boards/integrations/boards-teams?view=azure-devops)
- **Microsoft, other:**
  [ADO pricing](https://azure.microsoft.com/en-us/pricing/details/devops/azure-devops-services/) ·
  [remote MCP GA](https://devblogs.microsoft.com/devops/azure-devops-remote-mcp-server-ga/) ·
  [connector retirement](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/) ·
  [sprint 269 notes](https://learn.microsoft.com/en-us/azure/devops/release-notes/2026/sprint-269-update) (search result only) ·
  [Azure Bot quickstart](https://learn.microsoft.com/en-us/azure/bot-service/abs-quickstart?view=azure-bot-service-4.0) (search result only)
- **GitHub:**
  [issue fields GA](https://github.blog/changelog/2026-07-02-issue-fields-are-now-generally-available/) ·
  [issue fields docs](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/adding-and-managing-issue-fields) ·
  [Claude and Codex](https://github.blog/changelog/2026-02-04-claude-and-codex-are-now-available-in-public-preview-on-github/) ·
  [agent apps](https://docs.github.com/en/copilot/concepts/agents/agent-apps) ·
  [AU residency GA](https://github.blog/changelog/2025-02-04-github-enterprise-cloud-data-residency-in-australia-is-generally-available/) ·
  [about GHE.com](https://docs.github.com/en/enterprise-cloud@latest/admin/data-residency/about-github-enterprise-cloud-with-data-residency) ·
  [GHE.com features](https://docs.github.com/en/enterprise-cloud@latest/admin/data-residency/feature-overview-for-github-enterprise-cloud-with-data-residency) ·
  [GitHub in Teams](https://docs.github.com/en/integrations/how-tos/teams/use-github-in-teams) ·
  [pricing](https://github.com/pricing)
- **Orca:** [repository](https://github.com/stablyai/orca) (`LICENSE`, README, API) ·
  [Linear](https://www.onorca.dev/docs/review/linear) · [GitHub](https://www.onorca.dev/docs/review/github) ·
  [Jira](https://www.onorca.dev/docs/review/jira)
- **Alternatives:** Linear [security](https://linear.app/docs/security), [pricing](https://linear.app/pricing),
  [agent interaction](https://linear.app/developers/agent-interaction) · Plane
  [pricing](https://plane.so/pricing), [editions](https://developers.plane.so/self-hosting/editions-and-versions) ·
  Atlassian [remote agents in Jira](https://developer.atlassian.com/platform/forge/remote-agents-in-jira/) ·
  Spec Kit [`taskstoissues.md`](https://github.com/github/spec-kit/blob/main/templates/commands/taskstoissues.md)
- **GitHub API** (`/repos/<r>`, `/repos/<r>/releases`): `stablyai/orca`, `microsoft/azure-devops-go-api`,
  `microsoft/azure-devops-mcp`, `github/github-mcp-server`, `microsoft/Agents-for-js`,
  `makeplane/plane`, `supabase/supabase`, `supabase/realtime`, `coollabsio/coolify`.
- **Panacea, read-only, at `eee94cd94`:** the files listed in section 1.
