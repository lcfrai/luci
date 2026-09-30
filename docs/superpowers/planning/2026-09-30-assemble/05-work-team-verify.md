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

*(to be filled)*

## 4. The recommendation, part 3: Panacea's roster, snippets and preview comments

*(to be filled)*

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
