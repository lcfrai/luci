# 01 — Linear feature inventory (parity baseline)

Date read: **2026-09-25**. Author: research agent, Linear-parity workflow.
Status: research input for the synthesizer. Not a design. Not a decision.

## Why this exists

The owner, verbatim: *"I have just seen a product called linear.app. I need you to make sure that
we are doing everything they are doing and more. Make it prettier, better, everything."*

This file is the **what Linear does** half of that. It lists every Linear feature found in
Linear's public material as of 2026-09-25, with how a user reaches it, what makes it good, and
the source URL. The numbered master list at the end is the parity checklist.

## Clean-room rules followed

- Sources: Linear's **public** site, docs (`linear.app/docs/*`), developer docs
  (`linear.app/developers/*`), changelog, method pages, and public product pages only.
- No Linear assets, brand, copy or code are reproduced. Descriptions are paraphrased.
  Short quoted fragments are marked as quotes and kept under a sentence.
- Plane (AGPL) source was not opened. No Plane material is used here at all.
- Every claim not read directly from a Linear page on 2026-09-25 is marked **UNVERIFIED**.
- WebFetch returns a model summary of each page, not the raw page. A summary can drop or
  distort detail. Where a detail matters for parity (limits, plan gating), treat it as
  "read via summary" and re-check before building against it.

## Access notes

- `linear.app/sitemap.xml` was readable and gave the full doc tree (about 150 doc pages,
  27 developer pages, 10 product pages). That tree drives the section order below.
- Blocked or unreadable pages are listed in the "Gaps" section at the end, per page.

## How to read each entry

Each feature is one row or one short block:

- **Does** — what it does, paraphrased.
- **Reach** — how a user gets to it (menu, shortcut, surface).
- **Good because** — the design quality worth matching or beating.
- **Src** — the URL it was read from (all read 2026-09-25 unless marked).

---

## 1. Conceptual model

- **Workspace** holds everything for one company; one person can belong to several workspaces.
- **Team** is the main unit: it owns its workflow statuses, cycles, labels and issues.
  Teams can nest (parent team, sub-teams that inherit settings).
- **Issue** is the unit of work and belongs to **exactly one team**.
- **Cycle** is a team's repeating planning period. **Project** groups issues around one
  outcome and holds **milestones**. **Initiative** sits above projects (and can nest).
- **View** is a saved way of looking at the same work; it never changes the data.
- **Good because:** few nouns, strict containment, each with one job. A new user can hold the
  whole model in their head.
- **Src:** https://linear.app/docs/conceptual-model

## 2. Issues

### 2.1 Workflow statuses
- **Does:** six fixed categories in fixed order: Triage (optional), Backlog, Unstarted, Started,
  Completed, Canceled. Each team adds its own named statuses inside a category, with colour and
  description, and picks the default for new issues. A reserved **Duplicate** status applies
  when an issue is marked duplicate.
- **Auto-close** closes issues untouched for a set period; **auto-archive** archives closed
  work (issues, and also projects and cycles) and tells the creator. Archiving is automatic only.
- **Reach:** Settings → Teams → Issue statuses.
- **Good because:** teams customise names freely, while analytics still work across teams
  because the categories are fixed.
- **Src:** https://linear.app/docs/configuring-workflows

### 2.2 Priority
- **Does:** five levels only: No priority, Low, Medium, High, Urgent. Deliberately coarse to
  avoid decision fatigue. Drag order inside a priority-sorted view is shared by everyone.
  Urgent notifies the assignee (and emails, if email notifications are on).
- **Reach:** select issue(s), press `P`.
- **Src:** https://linear.app/docs/priority

### 2.3 Estimates
- **Does:** per-team point scales: Exponential (1-16), Fibonacci (1-8), Linear (1-5), T-shirt
  (XS-XL, mapped to Fibonacci). Optional extended scale (two higher values) and explicit zero.
  Unestimated issues count as 1 point. Estimates feed cycle and project graphs.
- **Reach:** Team settings → General → Estimates; `Shift+E` on an issue.
- **Src:** https://linear.app/docs/estimates

### 2.4 Labels
- **Does:** workspace labels (all teams) and team labels. **Label groups** make one label per
  group mutually exclusive (up to 250 per group). `Group/Label` shorthand creates both at once.
  Same-named team labels merge in cross-team views. Archive (keep history) or delete.
  Reserved names (priority, status, estimate and so on) are blocked.
- **Reach:** `L`, or the sidebar property. Label views (see §6).
- **Src:** https://linear.app/docs/labels

### 2.5 Parent and sub-issues
- **Does:** any issue can be a parent. Sub-issues inherit team, priority and project (cycle if
  created active; assignee in some cases; never labels). Optional automations: close parent when
  all sub-issues are done; close remaining sub-issues when the parent closes. A large issue can
  be converted into a project.
- **Reach:** "+ Add sub-issues" on the parent; `Cmd/Ctrl+Shift+O`; convert a comment or a
  highlighted checklist into sub-issues; "Set parent" from the command menu.
- **Good because:** breakdown happens where the thinking happens (a comment or a checklist
  becomes real work in one keystroke).
- **Src:** https://linear.app/docs/parent-and-sub-issues

### 2.6 Relations
- **Does:** Blocked by (`M` then `B`, orange flag), Blocking (`M` then `X`, red flag), Related
  (`M` then `R`, or created by mentioning an issue), Duplicate (moves the viewed issue to the
  Duplicate status). When a blocker resolves, the relation drops to Related automatically.
- **Src:** https://linear.app/docs/issue-relations

### 2.7 Templates (issue and form)
- **Does:** workspace templates (all teams, no team-only properties) and team templates (full
  team properties; sub-teams inherit). Prefill team, status, priority, assignee, **delegated
  agent**, project, labels, estimate and sub-issues. **Form templates** add typed fields (text,
  dropdown, checkbox, date, plus property fields such as customer or label group) and can make
  any field required. **Default templates** apply automatically, with different defaults for
  team members and non-members. Templates are used by Slack, Asks, Intercom, Zendesk, email and
  Zapier, and "created from template" is filterable in Insights.
- **Reach:** `Alt/Option+C`, or "Template" in the create modal; Settings → Templates.
- **Good because:** the intake form and the issue are the same object, so requests from
  non-engineers arrive complete.
- **Src:** https://linear.app/docs/issue-templates

### 2.8 Creating issues
- **Does:** `C` opens the create modal; `Alt+C` from template; a "create" icon top-left;
  highlight text anywhere and "Create issue from selection"; `linear.new` in any browser,
  with query parameters that prefill title, description, status, team, priority, assignee,
  estimate, cycle, labels, project, milestone, links and template. Team intake email address,
  and a per-template email address. **Recurring issues** from the composer's menu or Team
  settings. **Drafts**: local auto-draft on navigate-away, persistent draft on `Esc`, kept six
  months.
- **Good because:** capture is one keystroke from anywhere, and the URL scheme lets any tool
  (or agent) deep-link a prefilled issue.
- **Src:** https://linear.app/docs/creating-issues

### 2.9 Editing and moving issues
- **Does:** inline edit of title and description (description auto-saves). **Description
  history** with restore. Move to another team with `Cmd/Ctrl+Shift+M` (new ID and URL; the old
  URL redirects and the old ID stays searchable; cycle, team labels and project may clear;
  status maps to the nearest equivalent). `Cmd/Ctrl+A` selects all; bulk move and bulk edit;
  `Cmd/Ctrl+Z` undo.
- **Src:** https://linear.app/docs/editing-issues, https://linear.app/docs/select-issues

### 2.10 Due dates
- **Does:** `Shift+D`. Icon is red when due today or overdue, orange within a week, grey
  otherwise; hover shows days left or elapsed. Optional "due soon" and "overdue" notifications.
- **Src:** https://linear.app/docs/due-dates

### 2.11 Comments and threads
- **Does:** comments post explicitly (`Cmd/Ctrl+Enter`); unsent comments persist as drafts.
  Attachments by paperclip, `Cmd/Ctrl+Shift+A` or drag-drop. Threaded replies. **Resolve a
  thread**, pinned to the reply that answered it; AI summary of resolved threads and long
  discussions (Business and Enterprise; team setting). Emoji reactions on issues, comments and
  updates; custom emoji upload. Turn a comment into a sub-issue. **Inline comments** on
  highlighted text in descriptions, documents and project overviews (`Cmd+Alt+M`).
  `@Linear` in a comment invokes the Linear Agent (see §12).
- **Good because:** decisions get closed out and summarised, not lost in a scroll.
- **Src:** https://linear.app/docs/comment-on-issues

## 3. Triage

### 3.1 Triage inbox
- **Does:** a per-team review queue for incoming work. Issues land here when they come from
  integrations (Slack, Sentry, Intercom, Front, Zendesk), from Asks, from members outside the
  team, or are created in the Triage view. Triage issues are hidden from all other views unless
  a filter asks for them. A team can require a priority before an issue leaves Triage.
- **Actions:** Accept `1` (optional comment, moves to default status); Mark duplicate `2` or
  `M M` (merges attachments and customer requests into the canonical issue, cancels the source);
  Decline `3` (cancel, optional reason); Snooze `H` (hide until a time or until new activity).
- **Reach:** enable in Team settings; `G` then `T`, or `O` then `T`.
- **Good because:** a single numbered keystroke per decision makes triage a fast queue, not a
  meeting.
- **Src:** https://linear.app/docs/triage

### 3.2 Triage rules (Business, Enterprise)
- **Does:** ordered if-this-then-that rules evaluated top-down on triage issues; can set team,
  status, assignee, label, project and priority. `Shift`+click builds "any of" conditions.
- **Src:** https://linear.app/docs/triage

### 3.3 Triage responsibility
- **Does:** names first responders for the team's Triage, either fixed people or a rotation
  synced from PagerDuty, Opsgenie, Rootly or incident.io (API for other providers).
- **Src:** https://linear.app/docs/triage

### 3.4 Triage Intelligence (Business, Enterprise)
- **Does:** AI suggestions on incoming issues: team, project, assignee and labels from history,
  plus likely **duplicates** and **related** issues by semantic similarity. Results arrive in
  about 1 to 4 minutes. Each suggestion can be accepted, declined, or asked **"why"** (reasoning
  shown). Per property type a team picks **Show / Hide / Auto-apply**, optionally only for
  specific values; sub-teams inherit and can override. Teams can add written **guidance**;
  more local guidance weighs more heavily. Lightweight suggestions in the composer and property
  menus exist on all plans (basic search, not the full model). Linear states it does not train
  its own models on customer data.
- **Good because:** the AI is a suggestion with a reason attached, with a per-property dial
  from "show me" to "just do it". Trust is earned property by property.
- **Src:** https://linear.app/docs/triage-intelligence

## 4. Cycles

- **Does:** automated, repeating time-boxes per team (1 to 8 weeks), explicitly not tied to
  releases. Start day and time (12:01 local, team timezone); optional **cooldown** after each
  cycle (no issues can be assigned to cooldown); up to 15 future cycles pre-created.
- **Automations:** incomplete issues **roll over** to the next cycle (except backlog, triage,
  canceled); optionally **auto-add** Started or Completed issues with no cycle to the current
  cycle.
- **Capacity dial** on an upcoming cycle predicts whether the load will finish, from the
  velocity of the last three completed cycles.
- **Cycle graph:** grey scope line, blue completed, yellow started stacked on top, dotted blue
  target line (even burn over remaining working days). Stats are frozen at cycle end.
  Linear's "cycle success" counts started issues as 25% complete (UNVERIFIED that this formula
  is still current; read via summary).
- **Calendar:** subscribe to cycles in Google Calendar, by feed URL, or `.ics`.
- **Reach:** Team settings → Cycles; cycle pages in the team sidebar.
- **Good because:** zero ceremony. The schedule runs itself, rollover is automatic, and the
  graph answers "are we on track" at a glance.
- **Src:** https://linear.app/docs/use-cycles, https://linear.app/docs/cycle-graph,
  https://linear.app/docs/update-cycles (not read separately; see Gaps)

## 5. Projects

### 5.1 Project object
- **Does:** groups issues around one outcome, can span several teams. Properties: single
  **lead**, members (opt in to notifications), teams, status, priority, labels, icon, start and
  target dates at the granularity the team knows (year, half, quarter, month or exact day).
  Project IDs are sequential. Deleted projects sit in the archive for 30 days.
- **Tabs:** Overview, Issues (per-team sub-tabs when multi-team), Updates, Docs, plus any number
  of saved **project views as extra tabs**.
- **Reach:** add issues with `Shift+P`; details pane `Cmd/Ctrl+I`.
- **Src:** https://linear.app/docs/projects

### 5.2 Project overview page
- **Does:** short summary line, a document-style description (mentions, links to issues,
  projects, docs), a property table, **Resources** (labelled links and in-Linear documents),
  and the milestone list.
- **Good because:** the "why" document and the "what" issues live on one page; the spec is not in
  another tool.
- **Src:** https://linear.app/docs/project-overview

### 5.3 Project status, priority, labels
- **Does:** five status categories (Backlog, Planned, In Progress, Completed, Canceled) with
  custom named statuses inside each (Settings → Projects → Statuses). Status is **manual**, not
  derived from issue completion. Project priority and project labels exist (pages
  `/docs/project-priority`, `/docs/project-labels` listed in the sitemap; not read in detail,
  UNVERIFIED specifics).
- **Src:** https://linear.app/docs/project-status

### 5.4 Milestones
- **Does:** ordered stages inside a project, optional target date, description. Progress counts
  from "started" and again at "completed". Diamond icon; the focused milestone is yellow.
  Double-click a milestone on a timeline to open the project filtered to it. New issues in a
  project get a suggested milestone.
- **Reach:** under the project description, the details pane `+`, command menu; `Shift+M` to
  add or move issues; drag issues onto milestones; right-click a date on the timeline.
- **Src:** https://linear.app/docs/project-milestones

### 5.5 Project graph and completion prediction
- **Does:** appears once a project is Started, refreshed hourly. Grey scope line (scope creep is
  visible), started and completed lines, blue completed bars, red target-date line, and a dotted
  **predicted completion date** from weekly completed points (recent weeks weighted more), with
  optimistic and pessimistic bounds. Breakdown by assignee and label.
- **Good because:** "when will it ship" is answered by data, with honest error bars.
- **Src:** https://linear.app/docs/project-graph

### 5.6 Project dependencies
- **Does:** end-to-start "blocked by / blocking" between projects. Set from the project menu,
  the command menu, or by dragging from a bar's end circle on the timeline. Lines are blue when
  fine, **red when violated**. Dragging a project bumps its dependants (`Cmd/Ctrl` to stop,
  `Shift` to move the whole chain). Filter "has violated dependencies".
- **Src:** https://linear.app/docs/project-dependencies

### 5.7 Project templates
- **Does:** prefill name, description, status, lead, members, initiatives, milestones, and
  issues with sub-issues. Workspace or team scope; a team default template.
- **Src:** https://linear.app/docs/project-templates

### 5.8 Project and initiative updates
- **Does:** a written update with health **On track / At risk / Off track**. Workspace-level
  reminder cadence (weekly, fortnightly, day and time) sent to project leads and initiative
  owners, with nudges 1 and 2 working days later; per-project override. "Update missing" flag
  after a missed cycle plus three days; health goes grey when stale. **Write with Agent** drafts
  the update from changes since the last one and from the linked Slack channel. Updates include
  automatic property-change digests (date slips, milestone progress, lead changes). Posts to a
  Slack channel with two-way comment sync. Comments and reactions on updates.
- **Good because:** status reporting is a one-minute ritual with a draft already written, and
  staleness is visible rather than hidden.
- **Src:** https://linear.app/docs/initiative-and-project-updates

## 6. Initiatives, roadmaps and timelines

### 6.1 Initiatives
- **Does:** group projects under an organisational objective. Properties: status (Proposed,
  Planned, Active, Completed, Canceled), priority, owner or lead team, target date, labels,
  description, resources. The initiative page lists its projects with a **health roll-up**
  (latest update health per project, colour-coded) and an **initiative graph** of completion.
  Initiative updates work as in §5.8.
- **Plan gating:** team initiatives (including private team-led ones) on Business and
  Enterprise; saved **initiative views** Enterprise only.
- **Src:** https://linear.app/docs/initiatives

### 6.2 Sub-initiatives (Enterprise)
- **Does:** initiatives nest up to five levels. A parent rolls up every project it owns directly
  plus every project of its sub-initiatives; filters on the parent span the whole tree.
- **Src:** https://linear.app/docs/sub-initiatives

### 6.3 Timeline (roadmap)
- **Does:** a layout for project and initiative views that draws projects as bars over time.
  Zoom days, weeks, months, quarters. Shows milestones, dependencies (§5.6), and optionally
  **team cycles as an overlay**. Group by initiative or other attributes. Shown properties
  include lead, members, priority, status and health. Drag to reschedule (bars and milestones).
  Issues are not drawn on the timeline (list and board only).
- **Reach:** Display options → Timeline in any project or initiative view.
- **Good because:** the roadmap is not a separate artefact; it is the live projects drawn on a
  time axis, so it cannot drift from reality.
- **Src:** https://linear.app/docs/timeline

## 7. Views, filters, display and search

### 7.1 Custom views
- **Does:** saved, shareable filtered lists of **issues, projects or initiatives**. Scope:
  workspace, team (or attached to a project or initiative as a tab), or personal. Each view has
  an owner. Save any filtered list with `Alt+V`. Favourite with a star (appears in the sidebar;
  can be set as the default home page). **Subscribe** to a view for notifications when issues
  are added, completed or canceled (own actions filtered out). Post a view's changes to a Slack
  channel. Edit or duplicate. Ad hoc lists by URL (`/issues/ENG-1,ENG-2`).
- **Src:** https://linear.app/docs/custom-views

### 7.2 Filters
- **Does:** `F` opens filters; every filter is written into the URL, so a filtered screen is a
  shareable link. Operators: is / is not, is either of, includes any / all / none, before /
  after, relative date ranges. **Advanced filters** with AND/OR and nested groups. **AI
  filters**: type plain language ("due next week") and the filter is built.
- **Src:** https://linear.app/docs/filters

### 7.3 Display options
- **Does:** `Shift+V`. Layout list or board (`Cmd/Ctrl+B`), timeline for projects and
  initiatives. Group by status, assignee, project, priority, cycle, label, parent, team,
  customer, release, SLA status; a smart **Focus** grouping in My Issues. **Sub-grouping** makes
  swim lanes; dragging between lanes sets the property. Ordering by status, manual, priority,
  created, updated, due, link count; manual order is shared workspace-wide. Toggle shown
  properties (ID, SLA, PRs, commits, Sentry, customers, revenue, time in status and more), sub-
  issues, empty groups, completed window, triage issues. Per-user by default; **"Set as
  default"** pushes it to everyone; reset.
- **Good because:** one control surface for every list in the product, with the same keys.
- **Src:** https://linear.app/docs/display-options, https://linear.app/docs/board-layout

### 7.4 Search
- **Does:** `/` searches the workspace (issues, projects, documents; IDs, titles,
  descriptions, comments); `Cmd/Ctrl+F` searches the current view; `O` then `I` jumps to an
  issue. `@`-mention a team, user or status in the search box to turn it into a filter. Quoted
  phrases; loose ID forms (`lin123`). Recent searches and recent issues shown on open. Results
  rank active work above backlog, then closed, then archived. Semantic search is not documented
  on this page (UNVERIFIED whether AI search exists elsewhere).
- **Src:** https://linear.app/docs/search

### 7.5 Peek
- **Does:** `Space` toggles a preview panel for the focused item (hold `Space` to preview
  briefly). Issues show description and key properties; projects show details and their graph.
  Arrow keys move through the list while the preview follows; `Esc` closes.
- **Good because:** scan a list at reading speed without losing your place (Quick Look model).
- **Src:** https://linear.app/docs/peek

### 7.6 Favourites, label views, user views
- Favourites (star any view, project, issue, document into the sidebar), label views (every
  issue with a label across teams) and user views (a person's assigned and created work) have
  their own pages. Not read in detail: **UNVERIFIED** beyond their existence in the sitemap.
- **Src:** sitemap entries `/docs/favorites`, `/docs/label-views`, `/docs/user-views`

## 8. Personal surfaces: My Issues, Inbox, notifications, Pulse, reminders

### 8.1 My Issues
- **Does:** tabs Assigned, Created, Subscribed, Activity. Assigned uses a **focus order**
  (urgent, SLA-bound, blockers, current-cycle work, then the rest) ranked by priority. Activity
  shows your recent changes including PR activity.
- **Reach:** `G` then `M`.
- **Good because:** "what should I do next" is answered by the product, not by the user.
- **Src:** https://linear.app/docs/my-issues

### 8.2 Inbox
- **Does:** one in-app notification list for subscribed work (auto-subscribed to what you
  create, are assigned or are mentioned in). A **Priority** tab for what needs attention now
  (definable). Keys: `J`/`K` move, `U` read/unread (`Alt+U` all), `Backspace` delete
  (`Shift+Backspace` all read), `H` snooze, `Shift+S` unsubscribe, `Cmd/Ctrl+F` search by title,
  ID, type, assignee, team, project, priority. Display toggles: show snoozed, unread first, badge
  count, grouping. Cap of 2,000 open notifications.
- **Reminders:** `H` on any issue, document, project or initiative; natural phrases ("Jan 3
  10am", "next quarter"). The reminder shows at the top of the item.
- **Reach:** `G` then `I`.
- **Good because:** the inbox is keyboard-driven triage of your own attention, same keys as
  Triage.
- **Src:** https://linear.app/docs/inbox

### 8.3 Notification channels
- **Does:** desktop, mobile push, Slack and email (digest by default, timed by issue status, or
  immediate). Settings → Account → Notifications, grouped by category with a channel dot per
  group. `Shift+S` subscribe, `Cmd/Ctrl+Shift+S` edit subscribers. No documented quiet hours
  (UNVERIFIED whether mobile has them).
- **Src:** https://linear.app/docs/notifications

### 8.4 Pulse
- **Does:** a feed of project and initiative updates with tabs **For me**, **Popular**
  (engagement), **Recent**, plus personal feeds from custom filters. A daily or weekly summary
  lands in Inbox about 06:00 local; admins set the default (weekly Monday, weekdays, never) and
  users override. **Audio**: press Play on the Inbox summary to hear it read aloud. All plans;
  not for guests.
- **Good because:** leadership reads one feed instead of chasing twenty project pages.
- **Src:** https://linear.app/docs/pulse

## 9. Documents and the editor

### 9.1 Documents
- **Does:** long-form docs attached to a project, initiative, team, issue or cycle (runbooks,
  specs, meeting notes). **Real-time multiplayer** with visible cursors; "last edited by".
  Document templates (workspace or team). Full **version history** with revert, and an
  author toggle that shows **who, or which agent loop, wrote each passage**. Link to a document
  or to a specific heading with `@`. Inline comments; resolved comments hidden by default.
  Subscribers are notified of comments, "material changes", deletion and mentions.
- **AI:** `Cmd/Ctrl+J` asks the Linear Agent to write or edit; agent-written text is highlighted
  for review.
- **Reach:** `+` in a project or team, command menu; `O` then `D` for a project's docs.
- **Good because:** specs live next to the work they describe, and authorship between human
  and agent text stays legible.
- **Src:** https://linear.app/docs/documents

### 9.2 Editor (shared by issues, comments, docs, updates)
- **Does:** Markdown shortcuts converted live to rich text (bold, italic, strike, underline,
  inline code, headings 1-4, lists, checklists `[]`, quotes, dividers). Slash menu `/`: code
  block, **diagram (Mermaid)**, collapsible section (`>>>`), table (`|--`), date (`@Oct 1`),
  file. `Cmd/Ctrl+K` links (issue IDs auto-link). `@` mentions for people, issues, projects,
  docs. Auto-embeds YouTube, Loom, Descript, Figma. `:emoji:`. "Copy as Markdown".
- **Src:** https://linear.app/docs/editor

## 10. Customers, requests, Asks and SLAs

### 10.1 Customers and customer requests
- **Does:** a **Customer** object (domain, name, logo, revenue, size, tier, status, owner;
  custom revenue units and tier names). A **customer request** links a customer's words (quote,
  person, timestamp, link to source) to an issue or project, optionally **marked important**.
  Customer pages list every linked issue and project. Views filter and sort by customer count,
  tier, revenue, size, request status and importance (for example "enterprise-requested issues
  with 20+ requests, most-requested first"). Manual add with `Ctrl+R`.
- **Sources:** manual and Slack (all plans); Asks, Intercom (real-time), Zendesk and Front
  (12-hourly) on Business; Salesforce Service Cloud on Enterprise. Attribute sync from data
  sources with field mapping; merge duplicates; exclude domains (free-mail excluded by
  default); default team for request-created issues; CSV export; full GraphQL API.
- **Good because:** "who is asking for this and what are they worth" is a sort order on the
  backlog, not a spreadsheet.
- **Src:** https://linear.app/docs/customer-requests

### 10.2 Linear Asks (Business; some parts Enterprise)
- **Does:** request intake for people who do not use Linear (bug reports, IT, HR, ops). Channels:
  **Slack** (from a message or `/ask`, with a two-way synced thread), **email** (Linear address
  or custom domain; thread synced), **web forms** (Enterprise; requester signs in through the
  identity provider, gets a confirmation email). Asks land in the target team's **Triage**,
  use form templates (§2.7), and keep the requester updated wherever they asked.
- **Good because:** requesters stay in their own tool; the team gets a clean, structured issue.
- **Src:** https://linear.app/docs/linear-asks, sub-pages `/docs/linear-asks-slack`,
  `/docs/linear-asks-email`, `/docs/linear-asks-web-forms` (not read separately)

### 10.3 SLAs (Business, Enterprise)
- **Does:** rules that attach a deadline to matching issues on create or update (first match
  wins). Defaults: Urgent 24h, High 1 week, others none. Presets from 12h to 4 weeks or custom
  hours, days, **business days** (work week configurable, for example Sunday-Thursday), weeks.
  Conditions on almost any field. Fire icon grey → yellow → orange → red; statuses Low, Medium,
  High risk, Breached, Achieved, Failed. Notifies subscribers 24h before and at breach; Slack a
  business day before. Filter, group and chart by SLA status. Manual "Set SLA". An SLA replaces
  a due date.
- **Src:** https://linear.app/docs/sla

## 11. Releases, insights, dashboards and export

### 11.1 Releases and release pipelines (Business: 15 pipelines; Enterprise: unlimited)
- **Does:** a **release** is a name, a commit SHA and a set of issues, inside a **pipeline**
  (one per product and environment). Pipeline types: **Continuous** (a completed release is
  created whenever code reaches the main branch) and **Scheduled** (dated releases, optional
  **stages** that can be **frozen** so no new issues join, and "move open issues to the next
  release"). Monorepo **path filters** by glob. Issues attach automatically when CI scans
  commits for issue IDs; manual attach too; filter any view by release, stage or pipeline.
  **Release notes** written by hand or drafted by the agent (auto on scheduled completion);
  a per-pipeline **Changelog** tab. Status automations (for example, move to Done on release)
  that ripple into Intercom and similar tools.
- **Reach:** Settings → Releases (pipelines, CI access keys); an open-source `linear-release`
  CLI and a GitHub Action.
- **Good because:** "did this reach customers, and when" is answered on the issue itself. This
  is the closest Linear comes to our Deploy phase.
- **Src:** https://linear.app/docs/releases

### 11.2 Insights (Business, Enterprise)
- **Does:** an analytics panel on any view (`Cmd/Ctrl+Shift+I`): team, project, cycle and
  custom views. Measures: issue count, effort (points), **cycle time**, **lead time**, **triage
  time**, issue age. Bar (with colour segments), scatter (with 25/50/75/95 percentile markers,
  click a point to open the issue) and burn-up (cumulative flow). Slice and segment by any shown
  property; filters by created or completed date and status type. Share link, CSV export.
- **Good because:** analytics are a lens on the view you are already in, not a separate BI tool;
  every dot is a clickable issue.
- **Src:** https://linear.app/docs/insights

### 11.3 Dashboards (Enterprise)
- **Does:** combine insight charts, **metric blocks** and tables on one page. Dashboard-level
  filters plus per-insight filters; "Add to dashboard" from any insight. Owner; team, workspace
  or personal scope; private-team data excluded from workspace dashboards by default.
- **Src:** https://linear.app/docs/dashboards

### 11.4 Export
- **Does:** CSV of workspace issues and members (admins, or owners on Enterprise), of any view
  (members up to 250 issues, admins 2,000), projects and initiatives, and customer requests.
  **Copy as Markdown** (`Cmd+Alt+C`) captures an issue or doc with comments and requests, framed
  "for use with AI tools". API and webhooks; Airbyte and Google Sheets syncs. Guests cannot
  export.
- **Src:** https://linear.app/docs/exporting-data

<!-- APPEND -->
