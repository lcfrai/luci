# Phase W parity coverage map: the 257 rows

The checklist `OD-26` asks for. Every MATCH, EXCEED and PARTIAL row of the parity matrix names
the Phase W tasks that close it, and every SKIP gives its reason. The *Gaps* list at the end
names every non-SKIP row that no Phase W task closes.

**Sources.** *Item* is the numbered master list, §21 of
`docs/superpowers/planning/2026-09-25-linear-parity/01-linear-feature-inventory.md` (items 1-257).
*Verdict*, *05 stop* and the SKIP reason come from the §1 table of `05-design.md` (revision 2).
Its recount is 139 MATCH, 67 EXCEED, 24 PARTIAL and 27 SKIP, and this map counts the same. Task
ids come from the ten scope files beside this one, read on `feat/work-layer` at `c5841bf`.

**Method.** The map is extracted from the scope paragraphs mechanically. It records only what
each paragraph says, with no judgement added.

- A task **closes** a row when its paragraph cites the row after *Closes* or *closes*.
- A task **serves** a row when it cites the row after *Serves*, *Carries* or *Parity rows
  served*. Tasks 01-20 use *served* for a joint close and name the co-closers, who appear in the
  first list.
- These citations are not counted: rows after *none closed* (a precondition), rows that appear
  only in a *does not claim* clause, and rows named only in passing.
- The demo-and-results tasks (70, 97, 113, 142, 174, 187) appear as **proof** and are never
  counted as closers, so a row that is proved but never built still shows as a gap.
- A SKIP row has `—` in its task column. Tasks 49, 55, 146, 165 and the paragraph citing L-86
  mention SKIP rows only to say they stay SKIP (L-128, L-230, L-135, L-232, L-86).

**05 stop legend.** `W1`-`W5` means built in Phase W. `exists` means already built before Phase
W. `TM`, `phase 3`, `phase 5`, `phase 6` and `phase 8` mean the row is owned by that plan.
`later` means 05's deferred list, pulled into Phase W where a task closes the row.

**Counts.** 203 of the 230 non-SKIP rows have at least one closing Phase W task. 6 are served
but never closed, and 21 have no Phase W task at all. All 27 of those gaps have a 05 stop
outside Phase W (`exists`, `TM`, `phase 3/5/6/8`, or `exists / later`), and no row whose stop
is `W1`-`W5` lacks a closer.

**What this map does not claim.** It does not claim that any closing task's acceptance passes,
because no task is built yet. It does not claim that a row cited in prose without one of the
verbs above was caught: such a citation is missed, which errs toward reporting a gap. It also
does not claim that 05's `exists` rows are built, because this map did not re-check them
against the desktop tree (UNVERIFIED).

| L | Item | Verdict | 05 stop | Tasks that close it | SKIP reason |
|---:|---|---|---|---|---|
| 1 | Workspaces; one person in several workspaces | MATCH | exists | **none** |  |
| 2 | Teams as the primary unit, each with its own workflow, cycles and labels | MATCH | W1 | 75 · serves 08, 11 · proof 70 |  |
| 3 | Team identifier in human-readable issue IDs, for example `ENG-123` | MATCH | W1 | 40, 46 · serves 02, 04, 08, 17 · proof 70 |  |
| 4 | Sub-teams with enforced and optional inheritance, up to 5 levels | SKIP | — | — | Hub v1 has no per-project grants (hub-contract:333); nesting needs a grant model first. Revisit with org-level initiatives |
| 5 | Private teams, invisible to non-members | MATCH | exists | **none** |  |
| 6 | Private issue sharing with a non-member | SKIP | — | — | Same reason as L-4: no per-object grants in v1. A guest role is the later answer |
| 7 | Copy team automations and templates to another team | MATCH | later | 175 |  |
| 8 | Six fixed status categories with custom named statuses per team | EXCEED | W1 / W2 | 22, 23, 28, 46, 75, 84 · serves 03, 04, 10, 17 · proof 70 |  |
| 9 | Default status for new issues | MATCH | W1 | 25, 68 · serves 03-05, 11, 17, 42 · proof 70 |  |
| 10 | Reserved Duplicate status | MATCH | W1 | 42, 48 · serves 03, 05, 10, 11 · proof 70 |  |
| 11 | Auto-close stale issues | EXCEED | W2 | 73, 77, 94 |  |
| 12 | Auto-archive closed issues, projects and cycles | MATCH | W2 | 77, 91 |  |
| 13 | Five-level priority, deliberately coarse | MATCH | W1 | 28, 46 · serves 03, 04, 08, 17 · proof 70 |  |
| 14 | Manual drag order shared by the whole workspace | MATCH | W2 | 79, 89 · serves 73 |  |
| 15 | Urgent priority notifies the assignee | EXCEED | W1 | 63 · proof 70 |  |
| 16 | Estimates: four scales, extended scale, explicit zero | MATCH | W2 | 79, 89 · serves 73 |  |
| 17 | Workspace and team labels | MATCH | W1 | 46, 78, 90 · serves 03, 05, 11, 17, 72 · proof 70 |  |
| 18 | Mutually exclusive label groups | MATCH | W2 | 78, 90 · serves 72, 73 |  |
| 19 | `Group/Label` shorthand creates both | MATCH | W2 | 78, 90 · serves 72 |  |
| 20 | Parent and sub-issues with property inheritance | EXCEED | W2 | 78, 88 · serves 73 |  |
| 21 | Parent and sub-issue auto-close automations | EXCEED | W2 | 78, 88 · serves 73 |  |
| 22 | Turn a comment or a checklist into sub-issues | MATCH | W2 | 78, 88 |  |
| 23 | Convert a large issue into a project | EXCEED | W2 | 76, 86 · serves 73 |  |
| 24 | Relations: blocked by, blocking, related, duplicate | EXCEED | W1 | 48, 63 · serves 03, 05, 11, 17 · proof 70 |  |
| 25 | Resolved blocker downgrades to Related automatically | MATCH | W2 | 48, 79, 89 · serves 73 |  |
| 26 | Issue templates, workspace and team scope | MATCH | W2 | 78, 89 |  |
| 27 | Form templates with typed, required fields | MATCH | W5 | 148, 170 · proof 174 |  |
| 28 | Default templates, different for members and non-members | MATCH | W5 | 147 · serves 148 · proof 174 |  |
| 29 | Create from anywhere: `C`, `Alt+C`, create-from-selection | EXCEED | W1 | 59, 68 · proof 70 |  |
| 30 | `linear.new` URL scheme with prefill parameters | MATCH | W2 | 93 |  |
| 31 | Email intake, per team and per template | MATCH | W5 | 149 · proof 174 |  |
| 32 | Recurring issues | MATCH | W4 | 114, 121, 134 · proof 142 |  |
| 33 | Drafts, local and persistent, kept six months | MATCH | W2 | 77, 80, 92 · serves 72, 73 |  |
| 34 | Description auto-save and description history with restore | EXCEED | W1 | 47, 49 · serves 02, 04, 09, 17 · proof 70 |  |
| 35 | Move issue between teams; old ID and URL redirect | MATCH | W2 | 79, 89 · serves 73 |  |
| 36 | Undo | MATCH | W1 | 56, 66 · serves 61 · proof 70 |  |
| 37 | Due dates with colour-coded urgency and due-soon alerts | MATCH | W2 | 79, 89, 94 · serves 81 |  |
| 38 | Exactly one human assignee; `I` assigns to me | MATCH | W1 | 46, 53 · serves 03, 04, 08, 17 · proof 70 |  |
| 39 | Agent delegate alongside the human assignee | EXCEED | W1 / W3 | 24, 48, 61, 100, 102 · serves 98, 99 · proof 70, 113 |  |
| 40 | Personal auto-assign automations | MATCH | later | 176 |  |
| 41 | Comment threads, resolve with the answering reply, reactions, custom emoji | PARTIAL | W1 | 51 · serves 03, 05, 12, 17 · proof 70 |  |
| 42 | Inline comments on highlighted text | EXCEED | W1 | 51 · serves 03, 05, 12, 17 · proof 70 |  |
| 43 | AI summaries of resolved threads and long discussions | EXCEED | W4 | 139 · proof 142 |  |
| 44 | Comment drafts persist | MATCH | W2 | 80, 92 · serves 72 |  |
| 45 | Attachments by drag-drop, paperclip or shortcut | MATCH | W2 | 52 · serves 05, 09 · proof 70 |  |
| 46 | Per-team Triage inbox, hidden from other views | MATCH | W1 | 11, 42, 45 · serves 03, 05, 17, 63, 69 · proof 70 |  |
| 47 | Number-key triage actions: accept, duplicate, decline, snooze | EXCEED | W1 | 11, 24, 42, 53, 61 · proof 70 |  |
| 48 | Require priority before an issue leaves Triage | MATCH | W1 | 11, 42 · serves 03, 05, 17 · proof 70 |  |
| 49 | Ordered triage rules | MATCH | W5 | 147, 170 · proof 174 |  |
| 50 | Triage responsibility with rotation from PagerDuty, Opsgenie, Rootly, incident.io | PARTIAL | W4 | 114, 121, 134, 184 · proof 142 |  |
| 51 | Triage Intelligence suggestions for team, project, assignee, labels | EXCEED | W4 | 136 · proof 142 |  |
| 52 | "Why" reasoning on every suggestion | EXCEED | W4 | 136 · proof 142 |  |
| 53 | Per-property Show, Hide or Auto-apply dial | EXCEED | W4 | 114, 136 · proof 142 |  |
| 54 | Duplicate and related-issue detection | MATCH | W4 | 137 · proof 142 |  |
| 55 | Layered written AI guidance, most local wins | MATCH | W4 | 137 · proof 142 |  |
| 56 | Lightweight suggestions in the composer on all plans | MATCH | W4 | 137 · proof 142 |  |
| 57 | Automatic repeating cycles, 1 to 8 weeks | MATCH | W2 | 77, 87 |  |
| 58 | Cooldown periods | MATCH | W2 | 77, 87 |  |
| 59 | Up to 15 upcoming cycles | MATCH | W2 | 77, 87 |  |
| 60 | Automatic rollover of unfinished work | MATCH | W2 | 77, 87 · serves 73 |  |
| 61 | Auto-add started issues to the current cycle | MATCH | W2 | 77, 87 · serves 73 |  |
| 62 | Capacity dial from recent velocity | SKIP | — | — | A capacity number is a prediction (AX anti-pattern 2). We show the last three cycles' closed counts as plain facts instead (Q1) |
| 63 | Cycle graph: scope, started, completed, target | EXCEED | W2 | 77, 87 · serves 73 |  |
| 64 | Cycle calendar feed | MATCH | W2 | 77, 87 |  |
| 65 | Edit cycle names and dates; "cycles paused" gaps | MATCH | W2 | 77, 87 |  |
| 66 | AI "plan the next cycle" from backlog themes | EXCEED | W4 | 140 · proof 142 |  |
| 67 | Multi-team projects with one lead, members, fuzzy or exact dates | MATCH | W2 / W4 | 76, 85, 114, 117, 126 · proof 142 |  |
| 68 | Project tabs, including saved views as extra tabs | MATCH | W2 | 85 |  |
| 69 | Project overview: document-style description, resources, milestones | EXCEED | W2 | 76, 85 |  |
| 70 | Project statuses: five categories, custom names, set manually | EXCEED | W2 | 76, 85 |  |
| 71 | Project priority and project labels | MATCH | W2 | 76, 85 |  |
| 72 | Milestones with progress, suggested milestone on new issues | PARTIAL | W4 | 114, 120, 130 · proof 142 |  |
| 73 | Project graph with predicted completion and optimistic and pessimistic bounds | SKIP | — | — | Prediction is banned (AX anti-pattern 2; AX:237). Convergence per criterion replaces it (Q1) |
| 74 | Project dependencies, violation colouring, drag-to-bump chains | PARTIAL | W4 | 114, 120, 129 · proof 142 |  |
| 75 | Project templates with milestones and issues | MATCH | W4 | 114, 120 · proof 142 |  |
| 76 | Project updates with On track, At risk, Off track health | EXCEED | W4 | 114, 119, 127 · proof 142 |  |
| 77 | Update reminders, follow-up nudges, "update missing" and grey stale health | MATCH | W4 | 114, 119, 121, 127 · proof 142 |  |
| 78 | Agent-drafted updates from changes and Slack | EXCEED | W4 | 119, 139 · serves 141 · proof 142 |  |
| 79 | Automatic property-change digest inside updates | MATCH | W4 | 139 · proof 142 |  |
| 80 | Updates posted to Slack or Teams with synced comments | MATCH | W5 | 157, 182 · proof 174 |  |
| 81 | Project notifications by bell, and per-project Slack channels | MATCH | W5 | 157 · proof 174 |  |
| 82 | Agent drafts a whole project from workspace context | EXCEED | exists / W4 | 140 · proof 142 |  |
| 83 | Initiatives: status, priority, owner or lead team, target, labels, resources | MATCH | W4 | 114, 117, 118, 127 · proof 142 |  |
| 84 | Health roll-up of an initiative's projects | MATCH | W4 | 114, 118, 127 · proof 142 |  |
| 85 | Initiative graph | EXCEED | W4 | 114, 118, 127 · proof 142 |  |
| 86 | Sub-initiatives up to 5 levels with roll-up | SKIP | — | — | Needs cross-team grants (L-4's reason). Later, with org-level initiatives |
| 87 | Initiative updates | MATCH | W4 | 114, 118, 119, 127 · proof 142 |  |
| 88 | Team-led and private initiatives | MATCH | W4 | 114, 118, 127 · proof 142 |  |
| 89 | Timeline layout with day, week, month, quarter zoom | MATCH | W4 | 114, 128 · proof 142 |  |
| 90 | Drag to reschedule projects and milestones on the timeline | MATCH | W4 | 114, 129 · proof 142 |  |
| 91 | Team cycles overlaid on the timeline | MATCH | W4 | 114, 128 · proof 142 |  |
| 92 | Group the timeline by initiative or other attributes | MATCH | W4 | 114, 128 · proof 142 |  |
| 93 | Saved custom views of issues, projects or initiatives | MATCH | W2 | 80, 91 |  |
| 94 | View scopes workspace, team, personal; view owner | PARTIAL | W2 | 80, 91 |  |
| 95 | Save any filtered list as a view, `Alt+V` | MATCH | W2 | 80, 91 |  |
| 96 | Subscribe to a view for added, completed, canceled events | MATCH | W2 | 91, 94 · serves 81 |  |
| 97 | Post a view's changes to a Slack channel | MATCH | W5 | 157, 182 · proof 174 |  |
| 98 | Filters encoded in the URL, so every screen is a link | EXCEED | W1 / W2 | 47, 67, 80, 93 · proof 70 |  |
| 99 | Advanced AND/OR nested filter groups | MATCH | W2 | 80, 91 |  |
| 100 | AI natural-language filters | MATCH | W4 | 138 · proof 142 |  |
| 101 | List, board and timeline layouts | MATCH | W1 / W4 | 40, 41, 43, 45, 66, 128 · proof 70, 142 |  |
| 102 | Grouping and sub-grouping swim lanes; drag sets the property | EXCEED | W1 / W2 | 40, 43, 91 · proof 70 |  |
| 103 | Ordering options including shared manual order | MATCH | W2 | 79, 89 |  |
| 104 | Display-property toggles per row | MATCH | W1 | 44 · proof 70 |  |
| 105 | "Set as default" pushes display options to everyone | MATCH | W2 | 80, 91 · serves 73 |  |
| 106 | Workspace search `/`, view search, jump to issue `O I` | MATCH | W1 | 58 · serves 05, 14, 17 · proof 70 |  |
| 107 | `@`-mentions in search become filters | MATCH | W2 | 91 |  |
| 108 | Peek preview on `Space` | EXCEED | W1 | 60 · proof 70 |  |
| 109 | Favourites with folders | MATCH | W2 | 91 |  |
| 110 | Label views and user views | EXCEED | W2 / TM | 91 |  |
| 111 | Ad hoc issue lists by URL | MATCH | W2 | 91, 93 |  |
| 112 | My Issues with a focus order (urgent, SLA, blockers, cycle) | EXCEED | W1 | 45, 65 · serves 62, 63 · proof 70 |  |
| 113 | Keyboard-driven Inbox | EXCEED | W1 | 65, 66 · serves 62, 64 · proof 70 |  |
| 114 | Priority inbox | EXCEED | W1 | 62, 64 · serves 63, 65 · proof 70 |  |
| 115 | Snooze and reminders with natural-language times | PARTIAL | W2 | 94 |  |
| 116 | Notification channels: desktop, mobile, Slack, email digest | PARTIAL | W2 / W5 | 81, 94 |  |
| 117 | Automatic and manual subscriptions | MATCH | W2 | 52 · serves 03, 05, 13, 17, 81 · proof 70 |  |
| 118 | Pulse feed: For me, Popular, Recent, custom feeds | PARTIAL | W4 | 114, 123, 133 · proof 142 |  |
| 119 | Pulse daily or weekly digest, with audio playback | PARTIAL | W4 | 114, 123, 133 · proof 142 |  |
| 120 | Documents attached to projects, initiatives, teams, issues, cycles | PARTIAL | exists / later | **none** |  |
| 121 | Real-time multiplayer editing with cursors | MATCH | TM | **none** |  |
| 122 | Document templates | MATCH | W4 | 114, 120 · proof 142 |  |
| 123 | Version history with revert | EXCEED | exists | **none** |  |
| 124 | Per-passage authorship: human or agent loop | EXCEED | W1 | 49 · serves 02, 03, 09, 12, 68, 141 · proof 70 |  |
| 125 | Links to a specific heading | EXCEED | exists | **none** |  |
| 126 | Markdown shortcuts and slash menu | MATCH | W1 | 49, 50 · proof 70 |  |
| 127 | Mermaid diagrams, tables, collapsible sections, callouts | PARTIAL | W1 / later | 50, 92 · proof 70 |  |
| 128 | Auto-embeds: YouTube, Loom, Descript, Figma | SKIP | — | — | Third-party embeds pull outside content into a regulated tool and defeat egress control. Links unfurl as text. **Our own canvas frames embed natively** (L-208) |
| 129 | Copy as Markdown "for AI tools" | MATCH | W2 | 92 |  |
| 130 | Agent writes and edits with changes highlighted, `Cmd/Ctrl+J` | EXCEED | W4 | 141 · proof 142 |  |
| 131 | Customer object with revenue, size, tier, status, owner | SKIP | — | — | CRM data. "Who is it for" is already the brief's `audience` answer, and revenue data does not belong in a delivery tool's classification |
| 132 | Customer requests with source quote, linked to issues or projects | EXCEED | W5 | 146, 170 · proof 174 |  |
| 133 | Mark a request important | MATCH | W5 | 146, 170 · proof 174 |  |
| 134 | Customer pages | SKIP | — | — | Goes with L-131 |
| 135 | Prioritise by request count, tier and revenue | SKIP | — | — | Revenue and tier go with L-131. An issue lists its requesters by name instead |
| 136 | Customer attribute sync with field mapping; merge; domain exclusions | SKIP | — | — | Goes with L-131 |
| 137 | Asks from Slack with synced thread | MATCH | W5 | 146, 157, 182 · proof 174 |  |
| 138 | Asks by email, custom domain | PARTIAL | W5 | 146, 149 · proof 174 |  |
| 139 | Asks web forms behind IdP sign-in | MATCH | W5 | 148, 170 · proof 174 |  |
| 140 | Requester kept updated where they asked | EXCEED | W5 | 146, 149 · serves 71 · proof 174 |  |
| 141 | SLA rules with business days and custom work week | SKIP | — | — | A support-desk feature. Triage age is shown everywhere as "waiting 2d 4h" (AX §6.7), which covers an internal team. Revisit if a support team adopts us |
| 142 | SLA risk states, breach warnings, SLA analytics | SKIP | — | — | Goes with L-141 |
| 143 | Release pipelines, continuous and scheduled | EXCEED | exists | **none** |  |
| 144 | Release stages that can be frozen | MATCH | W4 | 131 · proof 142 |  |
| 145 | Monorepo path filters by glob | EXCEED | W4 | 131 · proof 142 |  |
| 146 | Issues attach to releases automatically from CI commit scans | EXCEED | W1 | 21, 22, 71 · serves 10 · proof 70 |  |
| 147 | Release notes, agent-drafted, and a per-pipeline changelog | EXCEED | W4 | 131 · proof 142 |  |
| 148 | Release events drive issue status and downstream tools | EXCEED | W4 | 131, 156 · serves 71 · proof 142 |  |
| 149 | `linear-release` CLI and GitHub Action | MATCH | W5 | 164 · proof 174 |  |
| 150 | Insights panel on any view: count, effort, cycle, lead, triage time, age | MATCH | W4 | 114, 122, 132 · proof 142 |  |
| 151 | Scatter with percentile markers; every point opens its issue | MATCH | W4 | 114, 122, 132 · proof 142 |  |
| 152 | Dashboards with charts, metric blocks, tables, layered filters | MATCH | later | 122, 177 |  |
| 153 | CSV export of issues, views, projects, requests | MATCH | W2 | 80, 94 |  |
| 154 | Warehouse and spreadsheet sync (Airbyte, Google Sheets) | MATCH | later | 156, 178 |  |
| 155 | Linear Agent chat with full workspace context, `Cmd/Ctrl+J` | EXCEED | TM | **none** |  |
| 156 | `@Linear` in comments, documents, Slack and Teams | PARTIAL | W3 | 100, 103, 112 · serves 99 · proof 113 |  |
| 157 | Agent acts only within the invoking user's permissions | MATCH | exists | 141 |  |
| 158 | Agent guidance, workspace and personal; admin kill switch | MATCH | W3 | 100, 109 · serves 99 · proof 113 |  |
| 159 | Saved agent Skills, personal or team | MATCH | W5 | 166 · proof 174 |  |
| 160 | Loops: event- or schedule-triggered agent automations | EXCEED | W5 | 162, 163, 172 · proof 174 |  |
| 161 | Loop permission list, run history, versions with restore, run now | MATCH | W5 | 162, 163, 172 · proof 174 |  |
| 162 | Code Intelligence: grounded codebase Q&A with links to evidence | MATCH | W3 | 110 · proof 113 |  |
| 163 | Coding sessions in a managed cloud sandbox, producing draft PRs | EXCEED | exists / TM | **none** |  |
| 164 | Per-repo environments: runtimes, env vars, separated secrets, prep scripts | PARTIAL | phase 8 | **none** |  |
| 165 | Per-repo agent and model choice, including Claude models | MATCH | phase 6 | **none** |  |
| 166 | Verification artifacts (screenshots, recordings) and browser testing | PARTIAL | W3 / later | 100, 101, 104, 112 · serves 99 · proof 113 |  |
| 167 | Steer or follow up a running session; notify when input is needed | EXCEED | exists | **none** |  |
| 168 | Automatic first-pass bug fixing from Triage | MATCH | W5 | 163 · proof 174 |  |
| 169 | Shared AI credit pool with top-ups, expiry, auto-replenish | EXCEED | phase 6 | serves 135 |  |
| 170 | Diffs: code review inside Linear, synced with GitHub, merge from Linear | EXCEED | phase 3 | serves 180 |  |
| 171 | Structural syntax-aware diff and `.gitattributes` file categories | MATCH | later | 179 |  |
| 172 | Guided Reviews: AI sections a large PR, core first | EXCEED | later | 180 |  |
| 173 | Agent-attributed review comments with name, avatar and model | MATCH | W3 | 100, 103 · serves 99 · proof 113 |  |
| 174 | PR risk scoring from connected tools | EXCEED | W3 | 111 · proof 113 |  |
| 175 | Open an issue in Claude Code, Cursor or a custom script | EXCEED | W1 | 24 · serves 19 |  |
| 176 | Enforce signed commits for agent sessions | MATCH | later | 181 |  |
| 177 | `TEAM-NEW` in a PR body creates an AI-written issue | MATCH | W5 | 155 · proof 174 |  |
| 178 | Agent files issues from whole support conversations | MATCH | W5 | 149, 157 · proof 174 |  |
| 179 | Agents as app users, not billable seats | MATCH | exists | **none** |  |
| 180 | Admin installs an agent with per-team access | MATCH | W5 | 166, 173 · proof 174 |  |
| 181 | Delegate versus assignee: the human keeps accountability | MATCH | W1 | 24, 46, 48, 61 · proof 70 |  |
| 182 | Agent profile pages; filter and chart by Delegate | EXCEED | W3 | 107 · serves 98 · proof 113 |  |
| 183 | Agent guidance with team override and history | MATCH | W3 | 100, 109 · serves 99 · proof 113 |  |
| 184 | AgentSession with states derived from activity | EXCEED | exists | 29 · serves 64 |  |
| 185 | Activity grammar: thought, action, elicitation, response, error, prompt | EXCEED | W3 | 108 · proof 113 |  |
| 186 | Ephemeral activities | MATCH | W3 | 40, 108 · proof 113 |  |
| 187 | Agent plans (checklist) | EXCEED | TM / W3 | 101 · serves 98 · proof 113 |  |
| 188 | External URLs from a session (agent UI, PR) | MATCH | W3 | 108 · proof 113 |  |
| 189 | Signals: stop, auth, select | MATCH | exists | serves 64 |  |
| 190 | Timing contract: 5 s webhook, 10 s first activity, 30 min idle | MATCH | W3 | 108 · serves 61 · proof 113 |  |
| 191 | Ready-made `promptContext` in the webhook | EXCEED | W1 | 24, 47, 67 · serves 09, 18, 19 · proof 70 |  |
| 192 | Repository suggestions with confidence | EXCEED | W3 | 111 · proof 113 |  |
| 193 | Agent-initiated sessions on an issue or comment | MATCH | W3 | 100, 111 · serves 99 · proof 113 |  |
| 194 | Published Agent Interaction Guidelines | MATCH | W3 | 109 · proof 113 |  |
| 195 | Partner agent directory: Cursor, Codex, Devin, Factory and more | SKIP | — | — | Third-party SaaS agents bring their own inference and break the ZDR promise (OD-19). We run harnesses as unmodified CLIs in our sandboxes; more harness adapters later |
| 196 | Hosted MCP server, read-write and read-only, OAuth 2.1, Okta | MATCH | W5 | 161 · proof 174 |  |
| 197 | MCP connectors that extend the Linear Agent and Loops | MATCH | W5 | 161, 166, 173 · proof 174 |  |
| 198 | GitHub: branch name copy with auto-assign and auto-start | EXCEED | W1 / W5 | 21, 24, 61, 155 · proof 70, 174 |  |
| 199 | GitHub: closing, non-closing and relation magic words | MATCH | W5 | 154, 155 · proof 174 |  |
| 200 | GitHub: commit linking by webhook | EXCEED | W1 / W5 | 21, 22, 154, 155 · proof 70, 174 |  |
| 201 | PR-stage automations (draft, open, review requested, ready, merged) with branch rules | EXCEED | W1 / W5 | 22, 154, 155 · serves 10, 67 · proof 70, 174 |  |
| 202 | GitHub Issues one-way or two-way sync | MATCH | W5 | 145, 150-153, 171 · proof 174 |  |
| 203 | PR linkback comments and autolinks | MATCH | W5 | 155 · proof 174 |  |
| 204 | GitHub Enterprise Cloud and Server | PARTIAL | phase 3 | **none** |  |
| 205 | GitLab MR linking and automations, self-managed | SKIP | — | — | OD-4 names Azure DevOps and GitHub. GitLab fits the same host interface later |
| 206 | Slack: create from message, unfurls with actions, synced threads, channel feeds | MATCH | later | 182 |  |
| 207 | Microsoft Teams: agent, batch filing, project update feeds | EXCEED | W5 | 157, 165 · proof 174 |  |
| 208 | Figma: frame previews and an in-Figma plugin | PARTIAL | phase 5 | **none** |  |
| 209 | Sentry: create or link, resolve on complete, assignee sync | MATCH | later | 183 |  |
| 210 | Zendesk: sidebar app, reopen ticket on completion | SKIP | — | — | Support desk (group J's reason) |
| 211 | Intercom: sidebar, internal note on completion, requests | SKIP | — | — | Support desk |
| 212 | Front: support intake and requests | SKIP | — | — | Support desk |
| 213 | Salesforce: cases to issues, real-time status sync, roles | SKIP | — | — | CRM (L-131's reason) |
| 214 | Gong: issues auto-filed from customer calls | SKIP | — | — | Sales calls; also puts client voice data in scope |
| 215 | Jira coexistence sync | MATCH | W5 | 145, 150, 153, 171 · proof 174 |  |
| 216 | Notion live embeds | SKIP | — | — | L-128's embed reason; notion.com is also blocked on the corporate network (03) |
| 217 | Zapier, Discord, Airbyte, Google Sheets, GUS | SKIP | — | — | The API and webhooks cover them; a Power Automate connector is the Microsoft-native answer, later |
| 218 | Incident tools for triage rotation | MATCH | later | 121, 184 |  |
| 219 | Directory of 250+ integrations; third parties can submit | SKIP | — | — | A short curated set (Azure DevOps, GitHub, Teams, Entra, Jira) serves regulated teams better than breadth; the API and MCP let others build |
| 220 | Importers: Jira, GitHub, Asana, Shortcut, Linear-to-Linear, CLI | PARTIAL | W1 / W5 | 25, 69, 150, 153, 171 · serves 03, 05, 15, 17 · proof 70, 174 |  |
| 221 | Contextual command menu `Cmd/Ctrl+K` that opens next to its trigger | MATCH | exists / W1 | 59 · proof 70 |  |
| 222 | Command-menu prefixes to scope results | MATCH | W1 | 59 · proof 70 |  |
| 223 | Key grammar: property letters, `G` go, `O` open, `M` relate | EXCEED | W1 | 53, 54, 102 · proof 70 |  |
| 224 | `?` shortcut sheet | MATCH | W1 | 54 · proof 70 |  |
| 225 | Multi-select and bottom bulk-action bar | EXCEED | W1 | 29, 44, 66 · proof 70 |  |
| 226 | Menus display the shortcut for every action | MATCH | W1 | 54, 59 · proof 70 |  |
| 227 | Safe-triangle submenus | MATCH | W1 | 54 · proof 70 |  |
| 228 | Local-first client database: no round trip per action | MATCH | W1 | 37, 55, 56 · proof 70 |  |
| 229 | Sync-action log with checkpoint delta sync | MATCH | exists / W1 | 55, 57 · serves 13, 17 · proof 70 |  |
| 230 | Offline change queue that survives restart | SKIP | — | — | Surviving a restart means writing Hub data to disk, which ADR-H1 forbids, and the product is cloud-first (OD-20). The in-memory queue survives a network drop, not a restart |
| 231 | Desktop apps for macOS and Windows, with tabs and per-tab history | PARTIAL | exists / later | **none** |  |
| 232 | Mobile apps: inbox, create, search, review, document editing | SKIP | — | — | A separate product. The phone's job, the 10-second check (AX §5), is served first by the Desk digest and answer-in-Teams (L-207). Revisit (Q6) |
| 233 | Web app and PWA | SKIP | — | — | The desktop is the portal (OD-20). The Hub serves only intake forms and read-only issue pages for people without the app |
| 234 | Light, dark, system and custom themes; community theme gallery | EXCEED | W1 / W2 | 26, 27, 95, 96 · proof 70 |  |
| 235 | Personal preferences: home view, font size, send key, auto-assign | PARTIAL | W2 | 96 |  |
| 236 | Roles: owner, admin, team owner, member, guest | PARTIAL | exists | **none** |  |
| 237 | SAML SSO with domain enforcement and JIT | PARTIAL | exists | **none** |  |
| 238 | SCIM with role groups and IdP-group-to-team mapping | MATCH | later | 185 |  |
| 239 | Audit log, 90 days, API queries, SIEM streaming | PARTIAL | exists / W5 | 159, 167 · serves 08, 14 · proof 174 |  |
| 240 | Data regions US or EU, full-stack replica per region | PARTIAL | phase 8 | **none** |  |
| 241 | SOC 2 Type II, GDPR, HIPAA with BAA | SKIP | — | — | An organisational programme, not a build item; KPMG's technology risk assessment is our gate |
| 242 | IP restrictions | MATCH | phase 8 | **none** |  |
| 243 | Domain claiming; block rogue workspaces on the domain | MATCH | exists | **none** |  |
| 244 | Third-party app approvals | MATCH | W5 | 145, 159, 161, 173 · proof 174 |  |
| 245 | Admin control of personal API keys; team-scoped keys | MATCH | W5 | 145, 160, 173 · proof 174 |  |
| 246 | AI feature toggles and Linear Agent kill switch | MATCH | W3 | 100, 109 · serves 99 · proof 113 |  |
| 247 | Uptime SLA, priority support, migration help | SKIP | — | — | Commercial terms, not product |
| 248 | Public GraphQL API, the same one Linear's apps use | MATCH | exists | serves 16 |  |
| 249 | TypeScript SDK | MATCH | W5 | 158 · proof 174 |  |
| 250 | OAuth 2.0, actor-as-app authorisation, app manifests | MATCH | W5 | 145, 159 · proof 174 |  |
| 251 | Webhooks with actor, previous values and HMAC signing | EXCEED | W5 | 156, 173 · serves 13 · proof 174 |  |
| 252 | Published rate and complexity limits with headers | MATCH | W5 | 160, 173 · proof 174 |  |
| 253 | A published product method that justifies every default | EXCEED | exists / W4 | 134 · proof 142 |  |
| 254 | A weekly public changelog; "build in public" | MATCH | later | 186 |  |
| 255 | AI spend limits at workspace, per-user and per-loop level, with usage by feature and user | EXCEED | phase 6 | serves 135 · proof 142 |  |
| 256 | Admin allow-list for agent MCP connectors; each user connects their own accounts | MATCH | W5 | 161, 166, 173 · proof 174 |  |
| 257 | Transparent AI pricing: provider token rates with no markup plus sandbox minutes | MATCH | phase 6 | serves 135 |  |
