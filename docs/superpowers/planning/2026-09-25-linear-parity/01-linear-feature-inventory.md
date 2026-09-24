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

## 12. Linear's own AI: Linear Agent, Skills, Loops, Code Intelligence, coding sessions, Diffs

**Headline for the synthesizer.** As of 2026-09-25 Linear is no longer only a tracker with AI
sprinkled on. Its own docs describe a first-party agent that runs **coding sessions in a
managed sandbox**, opens draft PRs, and hosts **code review inside Linear**. That overlaps our
Develop phase directly. Read §12.4 to §12.6 as competitive, not as nice-to-haves.

### 12.1 Linear Agent (first-party assistant)
- **Does:** a chat assistant with workspace context (teams, initiatives, projects, milestones,
  cycles, issues and relations, comments, activity history, documents). It creates and updates
  issues, projects, milestones and initiatives; summarises threads and customer feedback;
  answers questions about the workspace; posts and edits comments; drafts docs, status updates
  and stakeholder messages; recommends ("what should I prioritise this week?"). Keeps **chat
  history**. Acts **within the invoking user's permissions only**.
- **Reach:** `Cmd/Ctrl+J` chat (can be the **default home view**); `@Linear` in any comment,
  description or document; `@Linear` in Slack (and Microsoft Teams, per the coding-sessions
  page).
- **Control:** workspace and personal **guidance** (instructions); admin kill switch at
  Settings → AI → Linear Agent. Admins connect **MCP servers** to extend it
  (`/docs/connect-mcp-servers`, not read separately).
- **Src:** https://linear.app/docs/linear-agent

### 12.2 Skills
- **Does:** save a conversation that worked as a reusable, named shortcut; personal skills or
  team-shared skills for repeatable processes.
- **Src:** https://linear.app/docs/linear-agent (no separate page found; details UNVERIFIED)

### 12.3 Loops (paid plans)
- **Does:** standing agent automations. **Triggers:** events on issues (status, priority,
  assignee, cycle, project, labels), projects, initiatives, cycles and **releases**; schedules
  (hourly to yearly, timezone-aware); triage loops for issues entering Triage that match
  conditions. **Can:** follow written instructions, read and post in Slack, use Code
  Intelligence, **start coding sessions that open draft PRs**, call MCP connectors (GitHub,
  Notion, Sentry named), create, update and comment on issues, write summaries into documents.
- **Permissions per loop:** which teams it reads and writes; web access; code intelligence;
  coding sessions; run on externally created issues or not; write beyond the triggering object;
  write to synced threads.
- **Operate:** run history for audit, version history with restore, enable, disable, run now.
  Loops that only set fields cost no AI credits; loops that use the agent draw on credits.
- **Examples given:** bug triage with root-cause investigation; post-incident RCA with
  follow-up issues; customer message after close; weekly project review to Slack.
- **Good because:** "the automation" and "the agent" are one object with an explicit,
  reviewable permission list and an audit trail per run.
- **Src:** https://linear.app/docs/loops

### 12.4 Code Intelligence (Business, Enterprise)
- **Does:** grounded questions over connected GitHub repos ("how does auth work", "who wrote
  payments", "when was search added") with links to the files, commits and PRs behind the
  answer. Permission-aware (you see only repos you can reach in GitHub); admins pick which repos;
  admins can extend access to non-GitHub members (support, sales, product). Not for guests.
- **Src:** https://linear.app/docs/code-intelligence

### 12.5 Coding sessions (Basic, Business, Enterprise; AI credits)
- **Does:** delegate an issue and Linear runs an implementation session in **a managed
  development sandbox** (Python, Ruby, Go, Rust, Java, Node; dependencies from lockfiles), which
  drafts a **pull request** with diffs and **verification artifacts (screenshots, recordings)**.
  Only explicit implementation asks start coding; "investigate" or "plan" asks do not.
- **Start from:** issue delegation; `@linear` in Slack or Teams; triage automations and loops.
- **Models (as listed on the page, read via summary, UNVERIFIED exact list):** a GPT default,
  with Claude (Opus 5.5, Fable 5, Opus 5, Opus 4.8, Sonnet 5), other GPT variants and GLM
  selectable by the workspace owner.
- **Environments:** one environment per repository: runtimes and versions, env vars, **secrets
  held apart** from visible vars, preparation scripts, guidance files, agent and model per repo.
  Requires the user's GitHub account linked.
- **Steer:** open the session to steer or follow up, or let it run; "you'll be notified when your
  input is needed". Review in the Reviews tab; approve, or **delegate review fixes back** to the
  agent. No explicit stop control documented (UNVERIFIED).
- **Credits:** a shared workspace pool, launched at $20 per active user, top-ups by admins,
  credits expire after 12 months, optional auto-replenish (Settings → AI & Agents).
- **Src:** https://linear.app/docs/coding-sessions, https://linear.app/docs/loops,
  https://linear.app/docs/ai-credits (listed; not read separately)

### 12.6 Diffs and Reviews (code review inside Linear)
- **Does:** a Reviews area (`G` then `R`; `O` then `R`) with "For me" and "Created" tabs, group by
  status, author or repo. Unified or split diff (`Cmd/Ctrl+B`), **syntax-aware structural
  highlighting**, files categorised by `.gitattributes` (implementation, tests, docs, generated,
  localisation, assets). Inline threads and reactions; approve, request changes or comment,
  **synced both ways with GitHub**; merge from Linear with a chosen merge strategy. Replace
  `github.com` with `linear.review` in a PR URL to open it in Linear.
- **Guided Reviews (Business, Enterprise):** AI splits a large PR into sections, core
  implementation first, supporting and low-signal changes grouped after, each with an
  explanation.
- **Agent attribution:** coding agents (Claude Code, Codex, Linear, Pi, OpenCode named) can post
  review comments with **agent name, avatar and model** shown. **Risk scoring:** connected tools
  can attach a low-to-very-high risk rating with reasons to a PR.
- **Settings:** notifications for comments, reviews, team review requests (teams of 10 or
  fewer), failed checks, merge-queue removals; bot filtering; colour-blind diff theme;
  auto-ready draft PRs when review is requested; enforce **signed commits** for session commits;
  "open issue in" Claude Code, Cursor or a custom script.
- **Good because:** the issue, the agent's PR and the review are one thread in one tool.
- **Src:** https://linear.app/docs/diffs, https://linear.app/docs/code-and-reviews,
  https://linear.app/docs/open-issues-with-custom-scripts (listed; not read)

## 13. Linear for Agents: the third-party agent platform

### 13.1 Agents as workspace members (user view)
- **Does:** admins install third-party agents from the integrations directory and choose which
  teams each may access. Agents are **not billable seats**, cannot sign in, reach admin, or
  manage users. Anyone on those teams can `@mention` an agent in a comment or description, or
  **delegate** an issue to it.
- **Delegate versus assignee:** delegating does not transfer ownership. "The human assignee
  remains responsible for the issue, even after delegation." Delegated issues stay in the
  human's My Issues; views, filters and Insights can slice by **Delegate**; each agent has a
  profile page of its contributions.
- **Guidance:** Settings → Agents → Additional guidance, Markdown with history; team guidance
  overrides workspace guidance (for example: which repo, commit conventions, review rules).
- **Good because:** a clean accountability rule (a human always owns it) that makes letting an
  agent loose socially safe.
- **Src:** https://linear.app/docs/agents-in-linear, https://linear.app/docs/assigning-issues
  (listed; not read)

### 13.2 Agent developer model (API)
- **App actor:** an OAuth app installed with `actor=app` gets its own identity per workspace;
  scopes `app:assignable` (can be delegated to) and `app:mentionable` (can be mentioned); cannot
  hold admin scope. `PermissionChange` webhook on access changes.
- **AgentSession:** created automatically on mention or delegation (or proactively by the agent
  on an issue or comment). States, **derived by Linear from the latest activity**, never set by
  the agent: `pending`, `active`, `awaitingInput`, `error`, `complete`, `stale`.
- **Webhooks:** `created` carries a ready-made `promptContext` (issue, comments, guidance) plus
  structured fields; `prompted` carries a new human message. Receiver must answer within
  **5 seconds**; the agent must emit a first activity within **10 seconds**, then may keep
  emitting for up to 30 minutes between activities (timeout recoverable).
- **AgentActivity types:** `thought`, `action` (with "Searching" → "Searched" progression and a
  result), `elicitation` (moves the session to awaitingInput), `response` (done), `error` (with
  remediation), `prompt` (human only). `thought` and `action` can be **ephemeral** (replaced by
  the next activity). Activities are immutable records, unlike comments.
- **Plans (preview):** a session checklist of steps with `pending`, `inProgress`, `completed`,
  `canceled`; the agent replaces the whole plan on each update.
- **externalUrls:** labelled links out (for example to the agent's own UI or the PR); adding a PR
  URL "unlocks PR-specific features".
- **Signals:** human-to-agent `stop` (halt now, no more code changes or API calls, then emit a
  final response or error). Agent-to-human `auth` (render an account-link button, optionally only
  for one user) and `select` (render option buttons; free-text answers still allowed).
- **Repository suggestions:** `issueRepositorySuggestions` ranks the repos an agent can reach
  against the issue, guidance and signals, with confidence scores.
- **Best practice:** acknowledge instantly with a thought; move the issue to the first Started
  status; set yourself as delegate if none; leave assignment to humans; read history from
  activities, not comments.
- **Good because:** a small, strict protocol lets any vendor's agent look native: one timeline
  grammar, one state machine, one stop button.
- **Src:** https://linear.app/developers/agents, https://linear.app/developers/agent-interaction,
  https://linear.app/developers/agent-signals, https://linear.app/developers/agent-best-practices

### 13.3 Agent partners named on the public agents page
- Linear Agent (first party), Cursor, OpenAI Codex, Devin, Sentry (Seer root-cause and fix),
  ChatPRD (requirements), Oz by Warp, Factory, Charlie (TypeScript PRs), Ranger (bug formatting,
  test plans), Tembo (routes work to any coding agent). Claude Code appears as a supported
  MCP client and a review-comment author (§12.6), not in this partner list (read via summary;
  UNVERIFIED completeness).
- **Positioning lines (paraphrased):** agents are full workspace members; delegate the issue,
  not the accountability; humans stay in the loop with visibility into agent actions and
  reasoning.
- **Src:** https://linear.app/agents

### 13.4 Linear MCP server
- **Does:** a hosted remote MCP server at `mcp.linear.app/mcp` (Streamable HTTP; deprecated SSE
  fallback). Auth: OAuth 2.1 with dynamic client registration, or a bearer token or API key;
  Okta-managed auth for enterprise clients. A **read-only** endpoint (`/mcp/readonly`) or `read`
  scope. Tools find, create and update issues, projects, comments and more (tool list not
  enumerated on the page). Setup guides for Claude, Claude Code, Codex, Cursor, Jules, VS Code,
  v0, Windsurf, Zed.
- **Good because:** any agent, including an unmodified Claude Code, can read and write the
  tracker with the user's own permissions and no custom integration.
- **Src:** https://linear.app/docs/mcp

### 13.5 AI features as marketed (product page)
- Linear Agent (context across projects, issues, customer requests, docs and code); create
  issues from a Slack or Teams conversation; draft project updates; **plan the next cycle**
  (reads the backlog, finds themes, suggests items); check project status (at-risk work);
  reusable **skills** (chat or slash command); triage automation; Triage Intelligence;
  automated rules; coding sessions ("streams reviewable diffs"); Guided Reviews; codebase Q&A;
  MCP server.
- **Src:** https://linear.app/ai

### 13.6 Product Intelligence (name history)
- "Product Intelligence" shipped as a technology preview on 2025-08-14 and is now called
  **Triage Intelligence** (§3.4): routing suggestions, likely duplicates, related issues, hover
  for reasoning, guidance at workspace, team and sub-team level. Auto-apply followed on
  2025-09-19. "Similar issues" in the owner's list maps to this duplicate and related-issue
  detection; no separate "similar issues" feature was found.
- **Src:** https://linear.app/changelog/2025-08-14-product-intelligence-technology-preview,
  https://linear.app/changelog/2025-09-19-auto-apply-triage-suggestions (title from search
  result only; page not opened)

### 13.7 Recent agent trajectory (changelog, newest first)
- 2026-09-14: loops react to workspace changes, edit documents, post to Slack; callout blocks.
- 2026-09-03: **priority inbox**; agent drafts whole projects; document editing on mobile.
- 2026-08-20: coding sessions get configurable environments, **browser testing**, and
  published pricing (token charges plus sandbox runtime fees; amounts not captured, UNVERIFIED).
- 2026-08-13: teams can lead initiatives; private teams; MCP with Okta-managed identity.
- 2026-07-30: code review on mobile; Guided Reviews for larger PRs; GitHub team review requests.
- 2026-07-23: document **author attribution**; agent edits with change highlighting; issue
  sidebar pins diffs at top.
- 2026-07-20: **Loops** launched.
- 2026-07-02: initiative statuses, priority, label groups; MCP via Okta; shareable filtered views.
- 2026-06-18: agent-drafted project updates; **desktop tabs** with per-tab history; private
  sub-teams.
- 2026-06-11: **coding sessions** in the cloud, using **Claude Code and Codex** under the hood;
  review diffs, iterate in-thread with preview links. Linear says it resolves roughly 30% of
  its own incoming bug reports this way, mostly first pass. No multi-agent coordination
  described.
- 2026-05-14: Code Intelligence. 2026-04-23: Linear Agent MCP support. 2026-03-24: Linear Agent
  introduced.
- **Src:** https://linear.app/changelog, https://linear.app/changelog/2026-06-11-coding-sessions;
  other entry URLs from a web search result list (titles only)

**Gap worth naming for the synthesizer (observation, not a Linear claim):** nothing read
describes agents from **different owners coordinating with each other**. Each Linear agent
session is one human, one agent, one issue. Cross-owner agent negotiation (OD-16) has no
counterpart in Linear's public material as read today.

## 14. Integrations

### 14.1 GitHub
- **Linking:** copy a branch name containing the issue ID (`Cmd/Ctrl+Shift+.`); optional
  **auto-assign and move to Started when you copy the branch**. Magic words in PR title or body:
  closing words (close, fix, resolve and variants) advance on merge; non-closing words (ref,
  part of, contributes to) link only; "relates to" associates. `TEAM-NEW` in a PR body creates
  and links a new issue (AI writes title and description on paid plans). **Commit linking** by
  webhook: push moves to In Progress, reaching the default branch moves to Done.
- **Workflow automations** (Settings → Team → Workflows & automations): per PR stage Drafted,
  Open, Review requested, **Ready for merge** (checks pass), Merged. **Branch-specific rules** by
  regex (merge to `staging` → In QA; `main` → Deployed).
- **GitHub Issues sync:** one-way (many repos into one team) or two-way (one repo per team);
  syncs title, description, status, assignees, labels, sub-issues, comments. Personal account
  linking for correct attribution. **Linkback** comment on the PR with the issue title and
  description (can be hidden for private teams). GitHub autolinks for issue IDs.
- **Enterprise:** GitHub Enterprise Cloud (`*.ghe.com`) integration; GitHub Enterprise Server
  with reduced features (PR linking and description magic words only). One Linear workspace per
  GitHub org.
- **Good because:** the tracker updates itself from the developer's normal git actions; nobody
  drags cards.
- **Src:** https://linear.app/docs/github (the `/docs/github-integration` URL returned only a
  heading)

### 14.2 GitLab
- MR linking by branch name, MR title or magic words (several issues per MR). Status automations
  on Draft, Open, Review requested, Ready for merge, Merged, with branch-specific rules.
  GitLab.com and self-managed (must be publicly reachable; 15.6 or later). No GitLab issue import.
- **Src:** https://linear.app/docs/gitlab

### 14.3 Slack
- Create an issue from any message (template aware); `@Linear` natural-language filing ("file a
  bug, assign me"); `/linear` quick create. **Rich unfurls** with inline actions (assign,
  comment, subscribe); bare issue IDs get a threaded reply. **Synced threads** both ways, with
  close, cancel and duplicate mirrored. Channel notifications per team, project, initiative or
  view; personal DMs; auto-created project channels. Linear Agent in channels and group DMs with
  admin guidance. Loops can read and post. Slackbot MCP connection. Asks for non-users (§10.2).
  Private team content never unfurls.
- **Src:** https://linear.app/docs/slack

### 14.4 Figma
- Paste a frame link to get a preview snapshot (static; refresh on demand). A **Linear plugin
  inside Figma**: select elements to create linked issues, link existing issues, edit team,
  status, assignee and project from Figma, list all issues linked to the file and jump between
  frame and issue.
- **Src:** https://linear.app/docs/figma

### 14.5 Sentry
- Create or link Linear issues from Sentry (prefilled). Completing the Linear issue **resolves
  the Sentry issue**; assignee changes sync when emails match. Sentry icon on list and board rows.
  Public teams only; one Sentry org; cloud only. Sentry's Seer agent is a Linear agent partner.
- **Src:** https://linear.app/docs/sentry

### 14.6 Zendesk (Business, Enterprise)
- Ticket sidebar app to create or link issues; when the issue completes or cancels, the ticket
  **reopens with an internal note** so the support agent can tell the customer. Linear Agent can
  read a whole support conversation and file the right issues. Up to 10 templates; customer
  requests created; duplicates consolidate onto the canonical issue.
- **Src:** https://linear.app/docs/zendesk

### 14.7 Intercom (Business, Enterprise)
- Create and link issues from conversations; linked issues shown in Intercom's sidebar (ID,
  status, assignee). Linear Agent reads the whole conversation (messages, replies, attachments)
  to draft the issue. Customer requests created from contact and company data. **Internal note
  posted back** when the issue completes or cancels. Filter views to Intercom-linked issues.
- **Src:** https://linear.app/docs/intercom

### 14.8 Microsoft Teams
- `@Linear` in Teams: file and assign bugs, ask project status, batch ("an issue per feature
  request in this thread"). Project updates posted to a Teams channel. Members only (guests in
  shared channels ignored); shared channels unsupported.
- **Src:** https://linear.app/docs/microsoft-teams

### 14.9 Jira (coexistence sync)
- One-way (Jira-created issues mirror into Linear, updates both ways) or two-way. Syncs title,
  description, assignee, creator, priority, status, labels, due date; Jira epics become Linear
  projects. Framed for pilots and gradual migration.
- **Src:** https://linear.app/docs/jira

### 14.10 Gong (Enterprise)
- Automatically files issues from **customer call recordings** (customer-facing calls over 10
  minutes), with speaker-attributed transcript excerpts and a link to the exact moment; lands in
  Triage; non-actionable items filtered out.
- **Src:** https://linear.app/docs/gong

### 14.11 Salesforce (Enterprise add-on)
- Create issues from cases (with templates) or link cases to issues and projects; status and
  priority sync in real time; roles Admin, Create issues, Link only; account attributes mapped to
  Linear customers; triage rules; optional case reopen on resolution.
- **Src:** https://linear.app/docs/salesforce

### 14.12 Notion
- Paste Linear links into Notion for live previews (detailed, mention or URL) of issues,
  projects, initiatives and views; auto-refresh; per-member setup.
- **Src:** https://linear.app/docs/notion

### 14.13 Others in the doc tree (pages listed, not read: UNVERIFIED details)
- Airbyte (warehouse sync), Discord, Front (support; customer requests), Google Sheets
  (reporting sync), Zapier, GUS, GitHub Enterprise Cloud (beta), incident tools for triage
  rotation (PagerDuty, Opsgenie, Rootly, incident.io, from §3.3). The integration directory
  claims over 250 integrations and lets third parties build and submit their own.
- **Src:** sitemap; https://linear.app/docs/integration-directory

### 14.14 Importers
- Jira, GitHub Issues, Asana, Shortcut, Linear-to-Linear (workspace merge), and a CLI importer.
  Imports issues (filter active, stale, completed or all), projects, labels, users (mapped),
  custom views, initiatives and statuses, varying by source. A Jira terminology guide exists.
- **Src:** https://linear.app/docs/import-issues

## 15. Keyboard model, command menu, speed, sync engine, apps

### 15.1 Command menu and keyboard model
- **Command menu** `Cmd/Ctrl+K`: every action, searchable. It is **contextual**: it acts on the
  current view or selection, and when opened from a UI element it appears next to that element
  like a dropdown while staying searchable and keyboard-driven. Prefix letters in the menu scope
  results (`i` issues, `p` projects, `u` users, `t` teams, `l` labels, `f` favourites, `d`
  documents).
- **Grammar:** single letters set properties (`P` priority, `L` labels, `Shift+E` estimate,
  `Shift+D` due date, `Shift+P` project, `Shift+M` milestone, `H` snooze or remind); **`G` then
  a key goes somewhere** (`G I` inbox, `G M` my issues, `G T` triage, `G V` current cycle, `G B`
  backlog, `G R` reviews); **`O` then a key opens a picker** (`O F` favourites, `O P` projects,
  `O C` cycles, `O I` issue, `O D` docs, `O R` review); **`M` then a key makes a relation**
  (`M B`, `M X`, `M R`, `M M`). `?` shows every shortcut.
- **Selection:** `J`/`K` or arrows to move, `X` to select, `Shift`+arrows or `Shift`+click to
  extend, `Cmd/Ctrl+A` all; bulk actions via `Cmd/Ctrl+K`, right-click, or a toolbar at the
  bottom; `Esc` clears. `Space` peeks (§7.5).
- **Contextual menus** show the keyboard shortcut next to every mouse action, so the mouse
  teaches the keyboard. Submenus use a **"safe triangle"** so a diagonal mouse path to a submenu
  does not close it.
- **Good because:** a small, learnable grammar (verb letters, G for go, O for open, M for
  relate) covers the whole product, and every mouse path advertises its shortcut.
- **Src:** https://linear.app/docs/select-issues, https://linear.app/docs/search,
  https://linear.app/now/invisible-details,
  https://linear.app/changelog/2019-10-07-contextual-command-menu; `G`/`O` examples from a
  web-search summary of Linear docs (UNVERIFIED as a complete list)

### 15.2 Speed and the sync engine (local-first)
- **Does:** every client keeps a **local database**, so creating an issue, changing a status or
  navigating needs no network round trip; the UI feels immediate. Each change is a **sync
  action** (ordered ID, model, change type, routing metadata) in an application-level log that
  clients replay into their local store. On reconnect a client sends a **checkpoint** (last
  applied ID) and gets a **delta**, not the whole workspace. Large workspaces generate close to a
  million sync actions a day; Linear rebuilt the delta read path so latency stays flat as
  workspaces grow, filtering by permissions and subscriptions.
- **Offline:** changes queue locally and retry on reconnect, even after restart. Linear itself
  calls offline "a failsafe", not a full feature: bulk offline edits can overwrite teammates'
  concurrent changes (no timestamp checking).
- **Good because:** speed is architectural, not a coat of paint; every interaction is local.
- **Src:** https://linear.app/now/rebuilding-delta-sync-read-path,
  https://linear.app/docs/get-the-app, https://linear.app/now/scaling-the-linear-sync-engine
  (talk page; content not in text form)

### 15.3 Desktop, web and mobile
- **Desktop:** macOS (Intel and Apple Silicon) and Windows; no Linux (browser instead). OS
  notifications, dock or taskbar badge, fewer shortcut clashes than a browser, **tabs** (with
  per-tab history since 2026-06-18), near-full offline. A preference opens Linear links in the
  desktop app.
- **Web:** Chrome, Firefox, Safari; installable PWA.
- **Mobile (iOS, Android):** Home (assigned, created, favourites, team queues), Inbox (synced
  read state), create issues with camera-roll media, search, settings. Since mid-2026: code
  review iteration on mobile (2026-07-30) and document editing (2026-09-03).
- **Src:** https://linear.app/docs/get-the-app, https://linear.app/changelog,
  https://linear.app/mobile (listed; not read)

## 16. Enterprise, security and administration

### 16.1 Roles
- **Workspace owner** (Enterprise): billing, security, audit log, exports; decides which roles
  may do workspace-level actions. **Admin**: routine workspace management (everyone is admin on
  Free). **Team owner** (Business, Enterprise): runs one team, can restrict who manages its
  labels, templates, settings and members; alone can delete it, change privacy or reparent it.
  **Member**: no workspace admin pages. **Guest** (Business, Enterprise; billed as a member):
  only the teams they are added to; no Code Intelligence. Suspended users drop off billing but
  stay visible for history. Agents are app users, not seats (§13.1).
- **Src:** https://linear.app/docs/members-roles

### 16.2 SAML SSO (Enterprise per the SAML doc)
- Any mainstream IdP (Okta, Entra, OneLogin and others). SAML is enforced for approved domains
  (verified by DNS TXT), with non-SAML login allowed for other domains (contractors). JIT
  provisioning (profile set only at first login). Stop non-admins creating other workspaces on
  the claimed domain. Existing sessions are not killed on enablement.
- **Src:** https://linear.app/docs/saml-and-access-control

### 16.3 SCIM (Enterprise)
- Create, update and suspend users from the IdP (tested with Okta and OneLogin; Entra not named,
  UNVERIFIED). Role push groups `linear-owners`, `linear-admins`, `linear-guests`. **IdP groups
  map 1:1 to Linear teams**; a linked team's membership is then managed only in the IdP.
- **Src:** https://linear.app/docs/scim

### 16.4 Audit log (Enterprise per the audit-log doc)
- Account access, subscription and settings changes, with actor IP and country. 90-day
  retention in UI and API. Owners only. Filter by event type; hide session events. GraphQL
  queries by actor, email, IP, date, type. **Stream to a webhook** for SIEM, with a signing
  secret.
- **Src:** https://linear.app/docs/audit-log

### 16.5 Private teams and private issue sharing
- Private teams (Business, Enterprise): invisible to non-members; admins can see them in
  settings with a warning before joining; no mentions of non-members inside. Private or
  restricted sub-teams. **Private issue sharing** (Enterprise): share one issue with a
  non-member who may comment and edit but not re-share. Several integrations (Sheets, Intercom,
  Sentry, Zendesk) cannot see private-team data.
- **Src:** https://linear.app/docs/private-teams

### 16.6 Security posture and data regions
- SOC 2 Type II, GDPR, HIPAA (BAA on Enterprise). Encrypted in transit and at rest. **Data
  region US or EU**, chosen at workspace creation (default from timezone). The whole production
  stack is replicated per region, not sharded. Some metadata (workspace info, credentials, API
  keys, notification emails, analytics, crash logs) stays in the US regardless. No region move
  for existing workspaces as of the blog post (date of post not captured; UNVERIFIED whether
  that changed). Trust Center; published egress IPs.
- **Src:** https://linear.app/docs/security,
  https://linear.app/now/how-we-built-multi-region-support-for-linear

### 16.7 Other admin controls (pages listed, partly read)
- Login methods (Google SSO on all plans per pricing), invite controls, domain claiming, IP
  restrictions, **third-party application approvals** (admins approve OAuth apps), workspace
  settings for AI (per-feature toggles, Linear Agent kill switch), team issue limit, AI credits.
- **Src:** https://linear.app/pricing, sitemap entries `/docs/login-methods`,
  `/docs/third-party-application-approvals`, `/docs/team-issue-limit`, `/docs/ai-credits`
  (not read separately: UNVERIFIED details)

## 17. Pricing tiers (read 2026-09-25, via summary)

| Plan | Price | Limits |
|---|---|---|
| Free | $0 | unlimited members, 2 teams, 250 issues, 10 MB uploads |
| Basic | $10 per user per month, billed yearly | 5 teams, unlimited issues and uploads |
| Business | $16 per user per month, billed yearly | unlimited teams |
| Enterprise | custom, annual only | adds SCIM, IP restrictions, HIPAA, uptime SLA, invoice billing, migration help, priority support |

- **Free already includes** (per the pricing summary): the agent platform, MCP access, Linear
  Agent, Diffs and Guided Reviews, API and webhooks, Pulse, import and export, Microsoft Teams,
  Google SSO. That is aggressive: agent integration is a free-tier feature.
- **Business adds:** private teams, guests, Triage Intelligence, Loops, Code Intelligence,
  Insights, Asks, Zendesk and Intercom, unlimited release pipelines, coding sessions.
- **Coding sessions and Loops also draw AI credits** on top of the seat price.
- **Conflicts to resolve before relying on tiers (UNVERIFIED):** two reads of the pricing page
  disagreed with each other and with feature docs on audit log (doc: Enterprise), dashboards
  (doc: Enterprise), sub-initiatives (doc: Enterprise), web forms (doc: Enterprise), SAML (doc:
  Enterprise; pricing: Business "Google + SAML"), Diffs and Guided Reviews (doc: Guided Reviews
  Business+). Feature docs are more specific and should win.
- **Src:** https://linear.app/pricing, https://linear.app/docs/billing-and-plans

## 18. API, SDK and webhooks

### 18.1 GraphQL API
- **Does:** the public API is "the same API" Linear's own apps use; full query and mutation,
  cursor pagination, filtering (`/developers/pagination`, `/developers/filtering`), attachments,
  file upload, customer management, deprecation policy. Personal API keys (Account → Security &
  Access) with permissions Read, Write, Admin, Create issues, Create comments, optionally scoped
  to teams; admins decide whether members may create keys. OAuth 2.0 apps, **OAuth actor
  authorization** (act as the app, §13.2), **OAuth app manifests**. A TypeScript **SDK** (with
  webhook helpers; v1 to v2 migration guide).
- **Rate limits per hour:** API key 2,500 requests and 3,000,000 complexity points per user;
  OAuth app 5,000 requests and 2,000,000 points per user or app user; unauthenticated 600
  requests and 100,000 points per IP; 10,000 points per query. Rate-limit and complexity
  headers on every response.
- **Src:** https://linear.app/docs/api-and-webhooks, https://linear.app/developers/rate-limiting,
  developer page list from the sitemap (`/developers/graphql`, `/developers/sdk`,
  `/developers/oauth-2-0-authentication`, `/developers/oauth-actor-authorization`,
  `/developers/oauth-app-manifests`; not read separately)

### 18.2 Webhooks
- **Models:** issues, attachments, comments, labels, reactions, projects, project updates,
  documents, initiatives, initiative updates, cycles, customers, customer requests, users; plus
  issue SLA events, OAuth app revoked, and the agent events of §13.2 (`AgentSessionEvent`,
  `PermissionChange`).
- **Payload:** action (create, update, remove), type, **actor** (user, OAuth client or
  integration), createdAt, data, url, **`updatedFrom` (previous values)**, webhookTimestamp,
  organisation and webhook IDs.
- **Delivery:** `Linear-Signature` header, HMAC-SHA256 of the raw body; 5-second timeout; up to
  3 retries at 1 minute, 1 hour and 6 hours.
- **Good because:** every event carries who did it and what it was before, so consumers can
  build audit and diff views without extra calls.
- **Src:** https://linear.app/developers/webhooks

### 18.3 Other developer surfaces
- `linear.new` URL scheme (§2.8), "create issues using linear.new" guide; `linear-release` CLI
  and GitHub Action (§11.1); agent APIs (§13.2); AI gateway page `/developers/aig` (listed; not
  read, UNVERIFIED what it is); integration directory submission.
- **Src:** sitemap; https://linear.app/developers/create-issues-using-linear-new (not read)

### 18.4 Agent Interaction Guidelines (AIG)
- A public, six-point design code for agents in any product: an agent **discloses it is an
  agent**; **inhabits the platform natively** (the same actions and UI patterns humans use);
  gives **instant feedback** on invocation; is **transparent about its state** (thinking,
  waiting, executing, done) with inspectable reasoning; **respects requests to disengage** until
  clearly re-engaged; and **cannot be held accountable**, so a human always is.
- **Good because:** it is short enough to be a checklist, and it doubles as a positioning claim
  ("we set the rules for agents in the workplace").
- **Src:** https://linear.app/developers/aig

## 19. The Linear Method (published philosophy)

- **Principles (paraphrased):** build for the people doing the work, not for reports;
  purpose-built over infinitely configurable; create momentum, do not sprint; meaningful
  direction (daily work tied to goals); aim for clarity (standard words); say no to busy work;
  simple first, then powerful; decide and move on.
- **Practices (paraphrased):** set strategic initiatives while reserving capacity for the
  unplanned; connect daily work to goals through projects; work in n-week cycles (typically
  two); keep the backlog manageable; mix feature and quality work every cycle; name one owner
  per project and issue; write short project specs (why, what, how); understand users
  systematically; scope issues as small as possible; measure progress by real output such as
  diffs; run cross-functional teams; write a changelog.
- **Direction and building chapters:** set product direction, set useful goals, prioritise
  enablers and blockers, scope projects down, generate momentum, **write issues not user
  stories**, manage design projects, build with users, launch and keep launching, build in
  public.
- **Why it matters for parity:** the Method is the reason Linear's defaults are opinionated
  (five priorities, fixed status categories, automatic cycles). The product encodes a
  philosophy; that is much of why it "feels" better than configurable tools. The synthesizer
  should decide what PanaceaApp's equivalent published method is (Define → Deploy with one
  provenance thread is a natural candidate).
- **Src:** https://linear.app/method, https://linear.app/method/introduction

## 20. Addenda: teams, assignment, favourites, preferences

### 20.1 Teams and sub-teams
- **Team settings:** General (name, identifier used in issue IDs, timezone, estimates, email
  intake, issue history), Workflows and automations, Cycles, Triage (with rotation), Templates
  (issue, project, document), Labels, Recurring issues, Members, Slack notifications. PR
  automations and templates can be copied between teams.
- **Sub-teams** (Business; deeper nesting up to 5 levels on Enterprise): sub-team members must
  be parent members; cycle schedule is enforced from the parent; statuses and estimates
  optionally adopted; labels and templates usable from any level; parent views roll up
  sub-team issues. Independent: timezone, recurring issues, git automations, Slack.
- **Src:** https://linear.app/docs/teams, https://linear.app/docs/sub-teams

### 20.2 Assignment and delegation
- **One human assignee per issue**, always. `A` opens the assignee menu, **`I` assigns to me**,
  bulk reassign with `X` and right-click, "Assign to…" in the command menu. **Delegate** to an
  agent alongside the human (the agent "contributes on their behalf"); "No agent" removes it.
  Personal automations: auto-assign issues I create; auto-assign when moved to Started; git
  branch copy auto-assigns. Triage rules can assign and delegate.
- **Src:** https://linear.app/docs/assigning-issues

### 20.3 Cycle edits
- Rename and describe a cycle; renumbering shifts later cycles. Upcoming cycles: start and end
  editable; current: end only; past: frozen. Shortening creates a "Cycles paused" gap; cooldown
  labels update to the real gap.
- **Src:** https://linear.app/docs/update-cycles

### 20.4 Favourites
- Star issues, projects, views, documents, initiatives, cycles, labels, teams, customers,
  dashboards, pull requests and releases into a personal sidebar section, with **folders** and
  drag ordering. `O` then `F` opens; `Alt+F` toggles.
- **Src:** https://linear.app/docs/favorites

### 20.5 Account preferences and themes
- Default home view (can be the Linear Agent chat); full names or usernames; first day of week;
  emoticon conversion; Enter or `Cmd+Enter` to send; font size; pointer cursor; link underlines;
  **light, dark, system, or custom themes** (a custom theme builder and a community gallery of
  70+ themes at an external site); desktop: open links in app, badges, spell check.
- **Good because:** personalisation is cheap and fun, and it is part of why people love the tool.
- **Src:** https://linear.app/docs/account-preferences

### 20.6 Project notifications
- Bell icon on a project subscribes you to new issues in it, description comments and changes,
  completions and cancellations, and new updates. Slack channel notifications per project let
  channel members (including non-Linear users) see and act on activity.
- **Src:** https://linear.app/docs/project-notifications

---

## 21. Master parity checklist (numbered)

One line per feature, with the section that describes it. Numbering is stable: the synthesizer
should cite these numbers (for example "L-162") when mapping PanaceaApp parity and "more".

### A. Structure (§1, §16.5, §20.1)
1. Workspaces; one person in several workspaces (§1)
2. Teams as the primary unit, each with its own workflow, cycles and labels (§1)
3. Team identifier in human-readable issue IDs, for example `ENG-123` (§20.1)
4. Sub-teams with enforced and optional inheritance, up to 5 levels (§20.1)
5. Private teams, invisible to non-members (§16.5)
6. Private issue sharing with a non-member (§16.5)
7. Copy team automations and templates to another team (§20.1)

### B. Issues (§2, §20.2)
8. Six fixed status categories with custom named statuses per team (§2.1)
9. Default status for new issues (§2.1)
10. Reserved Duplicate status (§2.1)
11. Auto-close stale issues (§2.1)
12. Auto-archive closed issues, projects and cycles (§2.1)
13. Five-level priority, deliberately coarse (§2.2)
14. Manual drag order shared by the whole workspace (§2.2)
15. Urgent priority notifies the assignee (§2.2)
16. Estimates: four scales, extended scale, explicit zero (§2.3)
17. Workspace and team labels (§2.4)
18. Mutually exclusive label groups (§2.4)
19. `Group/Label` shorthand creates both (§2.4)
20. Parent and sub-issues with property inheritance (§2.5)
21. Parent and sub-issue auto-close automations (§2.5)
22. Turn a comment or a checklist into sub-issues (§2.5)
23. Convert a large issue into a project (§2.5)
24. Relations: blocked by, blocking, related, duplicate (§2.6)
25. Resolved blocker downgrades to Related automatically (§2.6)
26. Issue templates, workspace and team scope (§2.7)
27. Form templates with typed, required fields (§2.7)
28. Default templates, different for members and non-members (§2.7)
29. Create from anywhere: `C`, `Alt+C`, create-from-selection (§2.8)
30. `linear.new` URL scheme with prefill parameters (§2.8)
31. Email intake, per team and per template (§2.8)
32. Recurring issues (§2.8)
33. Drafts, local and persistent, kept six months (§2.8)
34. Description auto-save and description history with restore (§2.9)
35. Move issue between teams; old ID and URL redirect (§2.9)
36. Undo (§2.9)
37. Due dates with colour-coded urgency and due-soon alerts (§2.10)
38. Exactly one human assignee; `I` assigns to me (§20.2)
39. Agent delegate alongside the human assignee (§20.2, §13.1)
40. Personal auto-assign automations (§20.2)
41. Comment threads, resolve with the answering reply, reactions, custom emoji (§2.11)
42. Inline comments on highlighted text (§2.11)
43. AI summaries of resolved threads and long discussions (§2.11)
44. Comment drafts persist (§2.11)
45. Attachments by drag-drop, paperclip or shortcut (§2.11)

### C. Triage (§3)
46. Per-team Triage inbox, hidden from other views (§3.1)
47. Number-key triage actions: accept, duplicate, decline, snooze (§3.1)
48. Require priority before an issue leaves Triage (§3.1)
49. Ordered triage rules (§3.2)
50. Triage responsibility with rotation from PagerDuty, Opsgenie, Rootly, incident.io (§3.3)
51. Triage Intelligence suggestions for team, project, assignee, labels (§3.4)
52. "Why" reasoning on every suggestion (§3.4)
53. Per-property Show, Hide or Auto-apply dial (§3.4)
54. Duplicate and related-issue detection (§3.4, §13.6)
55. Layered written AI guidance, most local wins (§3.4)
56. Lightweight suggestions in the composer on all plans (§3.4)

### D. Cycles (§4, §20.3)
57. Automatic repeating cycles, 1 to 8 weeks (§4)
58. Cooldown periods (§4)
59. Up to 15 upcoming cycles (§4)
60. Automatic rollover of unfinished work (§4)
61. Auto-add started issues to the current cycle (§4)
62. Capacity dial from recent velocity (§4)
63. Cycle graph: scope, started, completed, target (§4)
64. Cycle calendar feed (§4)
65. Edit cycle names and dates; "cycles paused" gaps (§20.3)
66. AI "plan the next cycle" from backlog themes (§13.5)

### E. Projects (§5)
67. Multi-team projects with one lead, members, fuzzy or exact dates (§5.1)
68. Project tabs, including saved views as extra tabs (§5.1)
69. Project overview: document-style description, resources, milestones (§5.2)
70. Project statuses: five categories, custom names, set manually (§5.3)
71. Project priority and project labels (§5.3)
72. Milestones with progress, suggested milestone on new issues (§5.4)
73. Project graph with predicted completion and optimistic and pessimistic bounds (§5.5)
74. Project dependencies, violation colouring, drag-to-bump chains (§5.6)
75. Project templates with milestones and issues (§5.7)
76. Project updates with On track, At risk, Off track health (§5.8)
77. Update reminders, follow-up nudges, "update missing" and grey stale health (§5.8)
78. Agent-drafted updates from changes and Slack (§5.8)
79. Automatic property-change digest inside updates (§5.8)
80. Updates posted to Slack or Teams with synced comments (§5.8, §14.8)
81. Project notifications by bell, and per-project Slack channels (§20.6)
82. Agent drafts a whole project from workspace context (§13.7)

### F. Initiatives and roadmap (§6)
83. Initiatives: status, priority, owner or lead team, target, labels, resources (§6.1)
84. Health roll-up of an initiative's projects (§6.1)
85. Initiative graph (§6.1)
86. Sub-initiatives up to 5 levels with roll-up (§6.2)
87. Initiative updates (§6.1, §5.8)
88. Team-led and private initiatives (§6.1)
89. Timeline layout with day, week, month, quarter zoom (§6.3)
90. Drag to reschedule projects and milestones on the timeline (§6.3)
91. Team cycles overlaid on the timeline (§6.3)
92. Group the timeline by initiative or other attributes (§6.3)

### G. Views, filters, display, search (§7)
93. Saved custom views of issues, projects or initiatives (§7.1)
94. View scopes workspace, team, personal; view owner (§7.1)
95. Save any filtered list as a view, `Alt+V` (§7.1)
96. Subscribe to a view for added, completed, canceled events (§7.1)
97. Post a view's changes to a Slack channel (§7.1)
98. Filters encoded in the URL, so every screen is a link (§7.2)
99. Advanced AND/OR nested filter groups (§7.2)
100. AI natural-language filters (§7.2)
101. List, board and timeline layouts (§7.3)
102. Grouping and sub-grouping swim lanes; drag sets the property (§7.3)
103. Ordering options including shared manual order (§7.3)
104. Display-property toggles per row (§7.3)
105. "Set as default" pushes display options to everyone (§7.3)
106. Workspace search `/`, view search, jump to issue `O I` (§7.4)
107. `@`-mentions in search become filters (§7.4)
108. Peek preview on `Space` (§7.5)
109. Favourites with folders (§20.4)
110. Label views and user views (§7.6)
111. Ad hoc issue lists by URL (§7.1)

### H. Personal attention (§8)
112. My Issues with a focus order (urgent, SLA, blockers, cycle) (§8.1)
113. Keyboard-driven Inbox (§8.2)
114. Priority inbox (§8.2, §13.7)
115. Snooze and reminders with natural-language times (§8.2)
116. Notification channels: desktop, mobile, Slack, email digest (§8.3)
117. Automatic and manual subscriptions (§8.3)
118. Pulse feed: For me, Popular, Recent, custom feeds (§8.4)
119. Pulse daily or weekly digest, with audio playback (§8.4)

### I. Documents and editor (§9)
120. Documents attached to projects, initiatives, teams, issues, cycles (§9.1)
121. Real-time multiplayer editing with cursors (§9.1)
122. Document templates (§9.1)
123. Version history with revert (§9.1)
124. Per-passage authorship: human or agent loop (§9.1)
125. Links to a specific heading (§9.1)
126. Markdown shortcuts and slash menu (§9.2)
127. Mermaid diagrams, tables, collapsible sections, callouts (§9.2, §13.7)
128. Auto-embeds: YouTube, Loom, Descript, Figma (§9.2)
129. Copy as Markdown "for AI tools" (§11.4)
130. Agent writes and edits with changes highlighted, `Cmd/Ctrl+J` (§9.1)

### J. Customers, Asks, SLAs (§10)
131. Customer object with revenue, size, tier, status, owner (§10.1)
132. Customer requests with source quote, linked to issues or projects (§10.1)
133. Mark a request important (§10.1)
134. Customer pages (§10.1)
135. Prioritise by request count, tier and revenue (§10.1)
136. Customer attribute sync with field mapping; merge; domain exclusions (§10.1)
137. Asks from Slack with synced thread (§10.2)
138. Asks by email, custom domain (§10.2)
139. Asks web forms behind IdP sign-in (§10.2)
140. Requester kept updated where they asked (§10.2)
141. SLA rules with business days and custom work week (§10.3)
142. SLA risk states, breach warnings, SLA analytics (§10.3)

### K. Releases and analytics (§11)
143. Release pipelines, continuous and scheduled (§11.1)
144. Release stages that can be frozen (§11.1)
145. Monorepo path filters by glob (§11.1)
146. Issues attach to releases automatically from CI commit scans (§11.1)
147. Release notes, agent-drafted, and a per-pipeline changelog (§11.1)
148. Release events drive issue status and downstream tools (§11.1)
149. `linear-release` CLI and GitHub Action (§11.1)
150. Insights panel on any view: count, effort, cycle, lead, triage time, age (§11.2)
151. Scatter with percentile markers; every point opens its issue (§11.2)
152. Dashboards with charts, metric blocks, tables, layered filters (§11.3)
153. CSV export of issues, views, projects, requests (§11.4)
154. Warehouse and spreadsheet sync (Airbyte, Google Sheets) (§11.4)

### L. Linear's own AI (§12)
155. Linear Agent chat with full workspace context, `Cmd/Ctrl+J` (§12.1)
156. `@Linear` in comments, documents, Slack and Teams (§12.1)
157. Agent acts only within the invoking user's permissions (§12.1)
158. Agent guidance, workspace and personal; admin kill switch (§12.1)
159. Saved agent Skills, personal or team (§12.2)
160. Loops: event- or schedule-triggered agent automations (§12.3)
161. Loop permission list, run history, versions with restore, run now (§12.3)
162. Code Intelligence: grounded codebase Q&A with links to evidence (§12.4)
163. Coding sessions in a managed cloud sandbox, producing draft PRs (§12.5)
164. Per-repo environments: runtimes, env vars, separated secrets, prep scripts (§12.5)
165. Per-repo agent and model choice, including Claude models (§12.5)
166. Verification artifacts (screenshots, recordings) and browser testing (§12.5, §13.7)
167. Steer or follow up a running session; notify when input is needed (§12.5)
168. Automatic first-pass bug fixing from Triage (§13.7)
169. Shared AI credit pool with top-ups, expiry, auto-replenish (§12.5)
170. Diffs: code review inside Linear, synced with GitHub, merge from Linear (§12.6)
171. Structural syntax-aware diff and `.gitattributes` file categories (§12.6)
172. Guided Reviews: AI sections a large PR, core first (§12.6)
173. Agent-attributed review comments with name, avatar and model (§12.6)
174. PR risk scoring from connected tools (§12.6)
175. Open an issue in Claude Code, Cursor or a custom script (§12.6)
176. Enforce signed commits for agent sessions (§12.6)
177. `TEAM-NEW` in a PR body creates an AI-written issue (§14.1)
178. Agent files issues from whole support conversations (§14.6, §14.7)

### M. Agent platform (§13, §18.4)
179. Agents as app users, not billable seats (§13.1)
180. Admin installs an agent with per-team access (§13.1)
181. Delegate versus assignee: the human keeps accountability (§13.1)
182. Agent profile pages; filter and chart by Delegate (§13.1)
183. Agent guidance with team override and history (§13.1)
184. AgentSession with states derived from activity (§13.2)
185. Activity grammar: thought, action, elicitation, response, error, prompt (§13.2)
186. Ephemeral activities (§13.2)
187. Agent plans (checklist) (§13.2)
188. External URLs from a session (agent UI, PR) (§13.2)
189. Signals: stop, auth, select (§13.2)
190. Timing contract: 5 s webhook, 10 s first activity, 30 min idle (§13.2)
191. Ready-made `promptContext` in the webhook (§13.2)
192. Repository suggestions with confidence (§13.2)
193. Agent-initiated sessions on an issue or comment (§13.2)
194. Published Agent Interaction Guidelines (§18.4)
195. Partner agent directory: Cursor, Codex, Devin, Factory and more (§13.3)
196. Hosted MCP server, read-write and read-only, OAuth 2.1, Okta (§13.4)
197. MCP connectors that extend the Linear Agent and Loops (§12.1, §12.3)

### N. Integrations (§14)
198. GitHub: branch name copy with auto-assign and auto-start (§14.1)
199. GitHub: closing, non-closing and relation magic words (§14.1)
200. GitHub: commit linking by webhook (§14.1)
201. PR-stage automations (draft, open, review requested, ready, merged) with branch rules (§14.1)
202. GitHub Issues one-way or two-way sync (§14.1)
203. PR linkback comments and autolinks (§14.1)
204. GitHub Enterprise Cloud and Server (§14.1)
205. GitLab MR linking and automations, self-managed (§14.2)
206. Slack: create from message, unfurls with actions, synced threads, channel feeds (§14.3)
207. Microsoft Teams: agent, batch filing, project update feeds (§14.8)
208. Figma: frame previews and an in-Figma plugin (§14.4)
209. Sentry: create or link, resolve on complete, assignee sync (§14.5)
210. Zendesk: sidebar app, reopen ticket on completion (§14.6)
211. Intercom: sidebar, internal note on completion, requests (§14.7)
212. Front: support intake and requests (§14.13, UNVERIFIED detail)
213. Salesforce: cases to issues, real-time status sync, roles (§14.11)
214. Gong: issues auto-filed from customer calls (§14.10)
215. Jira coexistence sync (§14.9)
216. Notion live embeds (§14.12)
217. Zapier, Discord, Airbyte, Google Sheets, GUS (§14.13, UNVERIFIED detail)
218. Incident tools for triage rotation (§3.3)
219. Directory of 250+ integrations; third parties can submit (§14.13)
220. Importers: Jira, GitHub, Asana, Shortcut, Linear-to-Linear, CLI (§14.14)

### O. Keyboard, speed, apps (§15, §20.5)
221. Contextual command menu `Cmd/Ctrl+K` that opens next to its trigger (§15.1)
222. Command-menu prefixes to scope results (§15.1)
223. Key grammar: property letters, `G` go, `O` open, `M` relate (§15.1)
224. `?` shortcut sheet (§15.1)
225. Multi-select and bottom bulk-action bar (§15.1)
226. Menus display the shortcut for every action (§15.1)
227. Safe-triangle submenus (§15.1)
228. Local-first client database: no round trip per action (§15.2)
229. Sync-action log with checkpoint delta sync (§15.2)
230. Offline change queue that survives restart (§15.2)
231. Desktop apps for macOS and Windows, with tabs and per-tab history (§15.3)
232. Mobile apps: inbox, create, search, review, document editing (§15.3)
233. Web app and PWA (§15.3)
234. Light, dark, system and custom themes; community theme gallery (§20.5)
235. Personal preferences: home view, font size, send key, auto-assign (§20.5)

### P. Enterprise and admin (§16)
236. Roles: owner, admin, team owner, member, guest (§16.1)
237. SAML SSO with domain enforcement and JIT (§16.2)
238. SCIM with role groups and IdP-group-to-team mapping (§16.3)
239. Audit log, 90 days, API queries, SIEM streaming (§16.4)
240. Data regions US or EU, full-stack replica per region (§16.6)
241. SOC 2 Type II, GDPR, HIPAA with BAA (§16.6)
242. IP restrictions (§16.7, §17)
243. Domain claiming; block rogue workspaces on the domain (§16.2)
244. Third-party app approvals (§16.7)
245. Admin control of personal API keys; team-scoped keys (§18.1)
246. AI feature toggles and Linear Agent kill switch (§12.1, §16.7)
247. Uptime SLA, priority support, migration help (§17)

### Q. API and platform (§18)
248. Public GraphQL API, the same one Linear's apps use (§18.1)
249. TypeScript SDK (§18.1)
250. OAuth 2.0, actor-as-app authorisation, app manifests (§18.1)
251. Webhooks with actor, previous values and HMAC signing (§18.2)
252. Published rate and complexity limits with headers (§18.1)

### R. Philosophy and cadence (§19)
253. A published product method that justifies every default (§19)
254. A weekly public changelog; "build in public" (§13.7, §19)

**Count:** 254 features. Items with "UNVERIFIED detail" exist per Linear's doc tree but were
not read in full.

<!-- APPEND -->
