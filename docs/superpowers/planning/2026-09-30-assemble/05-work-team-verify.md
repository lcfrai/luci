# 05 — Work layer and team collaboration: the sceptic's verification

**Status:** IN PROGRESS (skeleton). Sections are filled one at a time and committed after each.
**Date:** 2026-09-30. **Verifies:** `05-work-team.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation (no tracker of our own: Azure DevOps Boards
first and GitHub Issues/Projects second behind one `tracker` interface, Teams through the
first-party apps plus one Microsoft 365 Agents SDK bot, Panacea's team roster and connector
snippets, Orca's task sources) and its two best alternatives at source: licence, the vendor's
own docs, paid-tier gates, maintenance, and whether the days fit OUR architecture (Go control
plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run.

## 0. Verdict first

*(to be filled)*

## 1. What was checked, and how

*(to be filled)*

## 2. The recommendation, part 1: Azure DevOps Boards as the first connector

**Holds, with two corrections.** Licence, price, AU geography and the MCP facts check out at
source. The Done rule does not work as written, and the identity it needs does not exist yet.

| # | Claim | At source (read 2026-09-30) | Holds? |
|---|---|---|---|
| 1 | Basic $6/user/month after 5 free; Stakeholder free | Pricing page: *"$6 per user per month"*, *"First 5 users free"*; Stakeholder can *"Create and edit work items"* and *"View backlog and boards"*. Also: *"GitHub Enterprise Cloud customers who manage users with Entra ID can access Azure DevOps without additional cost"* | **Yes.** A service identity can be a free Stakeholder |
| 2 | `azure-devops-mcp` and `azure-devops-go-api` are MIT | Both `LICENSE` files: MIT, *"Copyright (c) Microsoft Corporation"* | **Yes.** But the Go client's newest release is `azuredevops/v7.1.0` (17 April; the releases page shows no year). It targets API 7.1, still the current GA version. Low activity: Panacea's snippets call REST with plain `net/http`, the safer base |
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

*(to be filled)*

## 6. The Teams surface: first-party apps plus a Microsoft 365 Agents SDK bot

*(to be filled)*

## 7. The integration days, against our architecture

*(to be filled)*

## 8. Corrected recommendation

*(to be filled)*

## 9. What this verification could not verify

*(to be filled)*

## 10. Sources

*(to be filled)*
