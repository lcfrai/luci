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

<!-- APPEND -->
