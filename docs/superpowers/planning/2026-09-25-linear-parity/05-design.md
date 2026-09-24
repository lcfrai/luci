# 05 — Design: the parity-plus work layer

**Date:** 2026-09-25. **Status:** design for the owner's review. Not a plan, and nothing in it is
built. **Branch:** `feat/phase-2-cockpit`.

**The owner's ask, verbatim:** *"I have just seen a product called linear.app. I need you to make
sure that we are doing everything they are doing and more. Make it prettier, better, everything.
Mixed with the other benefits we have found, and the ability for agents across owners to work
together, and I think we have something magic."*

**Inputs.** This file synthesises the four research files in this folder:
`01-linear-feature-inventory.md` (257 numbered Linear features, `L-1`…`L-257`),
`02-linear-design-language.md` (design craft; tokens `C1`…`C13`), `03-agent-work-landscape.md`
(ten agent-work products) and `04-internal-fit.md` (where a work layer sits in our plans). It is
bound by OD-1 to OD-20 (`../2026-09-23-owner-decisions.md`), most of all OD-3 (shared state in the
Hub), OD-14 to OD-17 (team mode), OD-19 (org-level ZDR inference) and OD-20 (cloud-first), and by
`DESIGN-SYSTEM.md`'s non-negotiables and `AGENTIC-UX.md`'s principles. Team mode's design is
`../2026-09-24-team-collaboration/05-design.md` (cited here as **TM-05**), with its critiques
`06-critique-feasibility.md` and `07-critique-security.md`.

**Clean room.** Linear is proprietary. This file names its features by their generic industry
words (issue, cycle, triage, view) and cites them by `L-n`. It copies no Linear asset, brand, copy
or code, and describes no Linear visual that we would reproduce. Plane's source was not opened.
No new web page was read for this file: every Linear fact is cited from 01 or 02, which record
their own URLs and read dates (all 2026-09-25). Anything those files mark **UNVERIFIED** stays
UNVERIFIED here.

---

## 0. The answer in ten lines

1. **We build the work layer, not an integration.** An issue becomes a Hub object that points at
   the brief and the criteria it serves (04 §3). That pointer is what Linear cannot copy quickly:
   the work item is *on the provenance thread*, not beside it.
2. **Parity, counted.** Of Linear's 257 features: **MATCH 128, EXCEED 70, SKIP 59** (§1). Every
   SKIP has a reason. Most skips are support-desk and sales connectors (Zendesk, Gong, Salesforce),
   predictive graphs that `AGENTIC-UX.md` bans, and mobile.
3. **The magic is four things Linear does not have** (§2): agents that are real teammates on the
   issue (they take it through their owner, report in one live line, and *propose* evidence);
   Orchestrators that negotiate across owners and escalate to inform or to ask; **one thread** from
   a brief through a design frame, issue, session, commit and land to the release; and managed
   zero-retention inference with an AU switch.
4. **Done means proven.** An issue closes when every criterion it serves is settled by recorded
   evidence at the landed commit, not when a PR merges. Linear moves an issue to Done on merge.
5. **Object model** (§3): `issue is-` (mutable head, immutable digested revision), `feature ft-`
   (Linear's "project"; our `project` is the product), `cycle cy-`, `initiative in-`, `view vw-`,
   `comment ic-`, `request rq-`, plus team mode's `task tk-`. All live in the Hub. Live activity
   stays Fleet state and is derived onto the issue, never stored on it.
6. **One app, not two** (§4). Work is a seventh lens, not a separate product. The Desk absorbs
   Linear's Inbox, My Issues and triage-for-me. The palette gains the key grammar. The thread is
   the visual signature that says "this is not a Linear clone".
7. **Fast is architecture.** A Hub replica in memory with checkpoint delta sync and optimistic
   writes for reversible acts (02 C8). Irreversible acts (push, deploy) are never optimistic.
8. **Minimum lovable slice W1** (§5): issues, triage, one board, the key grammar, Start-into-a-lane,
   the trailer, and "done means proven". About **14 focused days**, not measured. The whole layer,
   W1 to W5, is about **47 focused days**.
9. **Two cheap changes now** (04 §7.1): add `issue:` to the phase 4 key parser and the Hub key
   grammar; fix the stale `criterionId: "done:2"` example in the Hub contract.
10. **Six owner questions** (§6). The biggest: progress displays (Linear shows percentages and
    graphs; `AGENTIC-UX.md` bans them). Recommendation: convergence per criterion, plain counts,
    no predictions.

**How to read this file.** §1 is the full parity matrix and is long; read its tally first. §2 is
the "more". §3 to §5 are the design and the delivery. §6 is what the owner decides.

---

## 1. The parity matrix: all 257 items

**Verdicts.** **MATCH**: we do what the item does, to the same bar. **EXCEED**: we do it and the
row says what more. **SKIP**: we do not build it, and the row says why. A SKIP is a decision the
owner can reverse (§6), not an oversight.

**Slices** (defined in §5): **W0** now, inside the current wave; **W1** the minimum lovable slice;
**W2** planning; **W3** agents on the work; **W4** roadmap, release and reporting; **W5** intake
and the ecosystem; **later** means designed for and not scheduled; **exists** means an in-flight
phase already delivers it. `TM` names team mode's own delivery (TM-05 §12).

**Sources for the "how" column.** Our facts come from 04 (cited `04 §n`), TM-05, `AGENTIC-UX.md`
(cited `AX`) and 02's named tokens (`C1`…`C13`). Linear's facts come from 01 by item number.

### A. Structure (L-1 to L-7)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 1 | Workspaces; one person in several | MATCH | Hub `org`, one per Entra tenant; a person in two tenants signs in to each | exists |
| 2 | Teams own workflow, cycles, labels | MATCH | Hub `team` exists; it gains state names, labels and a cycle cadence | W1 |
| 3 | Team key in human ids (`ENG-123`) | MATCH | The key is on the **project** (`EP-142`), because an issue belongs to one project (04 §2.4) | W1 |
| 4 | Sub-teams, 5 levels, inheritance | SKIP | Hub v1 has no per-project grants (hub-contract:333); nesting needs a grant model first. Revisit with org-level initiatives | — |
| 5 | Private teams | MATCH | Hub access is already exactly team membership (hub-contract:333) | exists |
| 6 | Share one private issue with a non-member | SKIP | Same reason as L-4: no per-object grants in v1. A guest role is the later answer | — |
| 7 | Copy automations and templates between teams | MATCH | "Copy from team…" on templates, views and policies | later |

### B. Issues (L-8 to L-45)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 8 | Fixed status categories, custom names | EXCEED | Fixed categories, renameable (04 §2.4). `started`, `in_review` and `landed` are set by **facts** (a lane bound, a lane queued, a trailer landed), each written as the person whose act caused it | W1 |
| 9 | Default status for new issues | MATCH | Per-team default; intake always lands in `triage` | W1 |
| 10 | Reserved Duplicate status | MATCH | `duplicate` state plus a `duplicates` relation | W1 |
| 11 | Auto-close stale issues | EXCEED | The janitor **proposes** closure as one batch on the assignee's Desk ("close 6 issues untouched for 90 days?"); AX §7 structural batching. A team may opt into auto-close; its event names the policy's author | W2 |
| 12 | Auto-archive closed items | MATCH | Janitor archive after a team-set age; archived stays searchable | W2 |
| 13 | Five-level coarse priority | MATCH | `none`·`urgent`·`high`·`medium`·`low` (04 §2.4) | W1 |
| 14 | Shared manual drag order | MATCH | One fractional `rank` per issue, shared by the team | W2 |
| 15 | Urgent notifies the assignee | EXCEED | An urgent issue enters the assignee's Desk under NEEDS YOU, ranked and aging ("waiting 2h 10m"); never a toast (AX anti-pattern 8) | W1 |
| 16 | Estimates, four scales, zero | MATCH | Team picks a scale; integer points underneath (04 §2.4) | W2 |
| 17 | Workspace and team labels | MATCH | Team labels in W1; org labels in W2 | W1 |
| 18 | Mutually exclusive label groups | MATCH | A label group with `exclusive: true` | W2 |
| 19 | `Group/Label` shorthand | MATCH | Same typed shorthand in the label picker | W2 |
| 20 | Parent and sub-issues, inheritance | EXCEED | Human sub-issues **and**, below them, team mode's agent `task tk-` (TM-05 §3.3). People's plan and agents' plan never share a list | W2 |
| 21 | Parent/child auto-close | EXCEED | A parent is `done` only when its children are closed **and** every criterion it serves is settled (§2.3) | W2 |
| 22 | Comment or checklist to sub-issues | MATCH | "Split into issues" on a selection | W2 |
| 23 | Convert a big issue into a project | EXCEED | "Promote to feature" creates `ft-` plus a branch brief, seeded from the issue, and keeps its `serves` links; the issue becomes the feature's first child | W2 |
| 24 | Blocks, related, duplicate relations | EXCEED | Same kinds (04 §2.4). A blocker that stalls **someone's agent** reaches its assignee's Desk with the cost of waiting (AX §7.3); `blocks` also feeds phase 3's land-order hint | W1 |
| 25 | Resolved blocker becomes Related | MATCH | Same rule, one event, written as the person who closed the blocker | W2 |
| 26 | Issue templates, org and team | MATCH | Templates are Hub records; a template may pre-fill `serves` | W2 |
| 27 | Form templates with typed fields | MATCH | Intake forms (§3.5) with typed, required fields | W5 |
| 28 | Different defaults for members and non-members | MATCH | Intake form default versus team default | W5 |
| 29 | Create from anywhere (`C`, selection) | EXCEED | `C` in any list, never in a terminal (02 keyboard rule); palette everywhere. **Create from a transcript span or a diff hunk**: the issue carries the span link (AX §8.2) | W1 |
| 30 | `linear.new` prefill URL | MATCH | `panacea://issue/new?title=…&project=EP` deep link, plus a web form on the Hub | W2 |
| 31 | Email intake per team or template | MATCH | An intake request `rq-` by email address per team (§3.5) | W5 |
| 32 | Recurring issues | MATCH | A schedule on a template; created by the janitor with `actor: null` and the schedule's author on the event | W4 |
| 33 | Drafts kept six months | MATCH | Drafts held **in the Hub** as private records, because no client persists Hub text to disk (ADR-H1:467-474) | W2 |
| 34 | Description autosave, history, restore | EXCEED | Every text change is an **immutable, digested revision** `issue:is-…@r`; restore writes a new revision. An agent's context names the exact revision it read (04 §2.4) | W1 |
| 35 | Move between teams, old id redirects | MATCH | Move between projects; `EP-142` redirects to its new number | W2 |
| 36 | Undo | MATCH | `optimistic.undo` with `--undo-window 6000ms` (02 C8). Irreversible acts are never optimistic | W1 |
| 37 | Due dates, urgency colour, alerts | MATCH | Due text in `tabular-nums`; colour is signal only; due-soon reaches the Desk digest, not a toast | W2 |
| 38 | One human assignee; `I` = me | MATCH | The assignee is always a person (OD-16, 04 §4.1) | W1 |
| 39 | Agent delegate beside the assignee | EXCEED | The delegate is always **the assignee's own** agent: Start in solo mode, Delegate to the owner's Orchestrator in team mode (§2.1). Nobody can delegate to another person's agent | W1 / W3 |
| 40 | Personal auto-assign automations | MATCH | A personal rule in settings | later |
| 41 | Threads, resolve, reactions, emoji | MATCH | `ic-` comments, append-only, author class `text·H`/`text·A`; custom emoji later | W2 |
| 42 | Inline comments on highlighted text | EXCEED | A comment may anchor to issue text **or** to a brief section, a criterion, a transcript span or a diff hunk | W2 |
| 43 | AI summaries of long threads | EXCEED | The summary is an **index of links** to the comments it condenses (AX §6.4), made through the ZDR gateway (OD-19) | W4 |
| 44 | Comment drafts persist | MATCH | Hub-held, as L-33 | W2 |
| 45 | Attachments | MATCH | The Hub already stores content-addressed blobs per project (hub-contract:590-591, cited by 04 §4.6); an attachment names a blob digest | W2 |

### C. Triage (L-46 to L-56)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 46 | Per-team Triage inbox | MATCH | `triage` state and a Triage view on the Work surface. Items that name a person also join that person's Desk (§4.2) | W1 |
| 47 | Number-key triage actions | EXCEED | `1` accept, `2` duplicate, `3` decline, `4` snooze, list focus only; plus `Shift+S` **accept and Start**, which binds a lane at once | W1 |
| 48 | Priority required to leave Triage | MATCH | Team policy, checked by the Hub on the state change | W1 |
| 49 | Ordered triage rules | MATCH | Intake rules, first match wins, each with its author on the event | W5 |
| 50 | Triage duty with rotation from incident tools | MATCH | A Hub-held rotation; syncing it from PagerDuty and the like is L-218 | W4 |
| 51 | AI suggests team, project, assignee, labels | EXCEED | Suggestions also name **which brief criterion the issue serves** (`serves`), so triage attaches work to the thread. Made through the ZDR gateway (OD-19) | W4 |
| 52 | "Why" on every suggestion | EXCEED | Each "why" links to the issues, spans or criteria it used (AX Principle 7: no element without a nameable span) | W4 |
| 53 | Per-property Show, Hide, Auto-apply dial | EXCEED | The dial uses AX §7's named levels and shows its **calibration**: "you accepted 23 of 25 label suggestions unchanged" (AX anti-pattern 12) | W4 |
| 54 | Duplicate and related detection | MATCH | Similarity search over issue revisions through the gateway | W4 |
| 55 | Layered AI guidance, most local wins | MATCH | org → team → project guidance; at feature scope the brief itself is the guidance | W4 |
| 56 | Light composer suggestions | MATCH | Label and duplicate hints while typing | W4 |

### D. Cycles (L-57 to L-66)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 57 | Automatic repeating cycles | MATCH | `cy-` per team; the janitor creates them ahead (04 §2.5) | W2 |
| 58 | Cooldown periods | MATCH | A gap between cycles in the cadence | W2 |
| 59 | Up to 15 upcoming cycles | MATCH | The team sets how many ahead | W2 |
| 60 | Automatic rollover | MATCH | Janitor, one event per issue, `actor: null` (04 §2.5) | W2 |
| 61 | Started issues join the current cycle | MATCH | Part of the `started` fact move, written as the person who started it | W2 |
| 62 | Capacity dial from velocity | SKIP | A capacity number is a prediction (AX anti-pattern 2). We show the last three cycles' closed counts as plain facts instead (Q1) | — |
| 63 | Cycle graph: scope, started, done, target | EXCEED | Replaced by **convergence per criterion** (settled, open, contested) and a **scope log** ("3 added on day 4, by whom"). No target line (Q1) | W2 |
| 64 | Cycle calendar feed | MATCH | An iCal feed per team | W2 |
| 65 | Edit cycle names and dates; paused gaps | MATCH | Team admin edits; a pause is a cadence gap | W2 |
| 66 | AI plans the next cycle | EXCEED | The user's Orchestrator **proposes** a cycle as one Desk item, grouped by the criteria it would settle; a person accepts it | W4 |

### E. Projects, which we call features (L-67 to L-82)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 67 | Multi-team project, lead, members, dates | MATCH | `ft-` has a lead and a target date (04 §2.5). **Multi-team** needs a cross-team grant, so it comes with org-level initiatives | W2 / later |
| 68 | Tabs, saved views as tabs | MATCH | Feature tabs: Brief, Issues, Criteria, Lanes, Release; any saved view pins as a tab | W2 |
| 69 | Document-style overview | EXCEED | The overview **is the branch brief**: the same artefact Define wrote, Design cites and Deploy pins (04 §2.5) | W2 |
| 70 | Project status set by hand | EXCEED | A person sets `planned`·`active`·`paused`·`done`·`cancelled`; the facts sit beside it (lanes live, landed, released where) so a stale status is visible | W2 |
| 71 | Project priority and labels | MATCH | Same fields on `ft-` | W2 |
| 72 | Milestones with progress | EXCEED | A milestone is a **named subset of the brief's criteria** with a date, reached when those criteria are settled at the branch head (04 §2.5). Convergence, not a bar | W4 |
| 73 | Predicted completion with bounds | SKIP | Prediction is banned (AX anti-pattern 2; AX:237). Convergence per criterion replaces it (Q1) | — |
| 74 | Project dependencies on the timeline | MATCH | `blocks` between features, drawn on the timeline; a violated order is signal-coloured | W4 |
| 75 | Project templates | MATCH | A brief template plus a set of issue templates | W4 |
| 76 | Updates with On track, At risk, Off track | EXCEED | `nt-` with `kind: update` (04 §2.5); each update **pins the brief revision** it reports against and links the facts it cites | W4 |
| 77 | Update reminders, stale health | MATCH | "Update missing, 9 days" arrives in the lead's Desk digest | W4 |
| 78 | Agent-drafted updates | EXCEED | The draft is the product-language handover (AX §6.3): every claim links to its span, and it stays `text·A` until a person edits and posts it | W4 |
| 79 | Property-change digest in updates | MATCH | Built from the Hub change feed | W4 |
| 80 | Updates to Slack or Teams | MATCH | Teams first (KPMG runs on Microsoft), Slack next | W5 |
| 81 | Project notifications, per-project channel | MATCH | Desk subscriptions; a Teams channel per feature | W5 |
| 82 | Agent drafts a whole project | EXCEED | This is **Define**: phase 4's Discover writes the brief, its five questions and its criteria; W4 adds "propose issues, one per open criterion" | exists / W4 |

### F. Initiatives and roadmap (L-83 to L-92)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 83 | Initiatives with status, owner, target | MATCH | `in-` per team in v1 (04 §2.5) | W4 |
| 84 | Health roll-up of its projects | MATCH | Lists each feature's latest health word and its date; no computed score | W4 |
| 85 | Initiative graph | EXCEED | Replaced by convergence across its features' criteria plus the scope log, as L-63 (Q1) | W4 |
| 86 | Sub-initiatives, 5 levels | SKIP | Needs cross-team grants (L-4's reason). Later, with org-level initiatives | — |
| 87 | Initiative updates | MATCH | `nt-` update, as L-76 | W4 |
| 88 | Team-led and private initiatives | MATCH | Per team by default, so team-private by construction | W4 |
| 89 | Timeline with day to quarter zoom | MATCH | Timeline layout of features and milestones | W4 |
| 90 | Drag to reschedule | MATCH | Drag writes the target date as the person dragging, with undo | W4 |
| 91 | Cycles overlaid on the timeline | MATCH | Team cycles drawn as a band | W4 |
| 92 | Group the timeline | MATCH | By initiative, lead or label | W4 |

### G. Views, filters, display, search (L-93 to L-111)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 93 | Saved views of issues, features, initiatives | MATCH | `vw-`: a filter plus a layout (04 §2.2) | W2 |
| 94 | View scopes and owner | MATCH | Shared views in the Hub; personal views are preferences on the desktop | W2 |
| 95 | Save any filtered list | MATCH | "Save as view" in the palette and the list header | W2 |
| 96 | Subscribe to a view | MATCH | Changes arrive in the Desk digest, never a stream (AX §6.8) | W2 |
| 97 | View changes to a chat channel | MATCH | Teams first, Slack next | W5 |
| 98 | Filters in the URL; every screen a link | EXCEED | Every screen is a `panacea://` link, and a link to an issue can name **its exact revision** (`issue:is-…@r`), so "what did they read?" has an answer | W1 |
| 99 | Nested AND/OR filters | MATCH | Filter groups in the view editor | W2 |
| 100 | Natural-language filters | MATCH | Parsed through the gateway and shown as editable filter chips before they apply | W4 |
| 101 | List, board, timeline | MATCH | List and board in W1; timeline in W4 | W1 / W4 |
| 102 | Group and sub-group; drag sets the property | EXCEED | As Linear, except that dragging into a **fact state** (`started`, `landed`) offers the fact ("Start a lane?") instead of faking it; a manual move shows both | W1 / W2 |
| 103 | Ordering options, shared manual order | MATCH | Uses L-14's rank | W2 |
| 104 | Per-row display toggles | MATCH | Display menu on every list | W1 |
| 105 | "Set as default" for everyone | MATCH | A team admin action on a shared view | W2 |
| 106 | Search `/`, view search, jump to issue | MATCH | `/` searches the in-memory Hub replica; `O` `I` then a number opens an issue (§4.3) | W1 |
| 107 | `@`-mentions become filters | MATCH | `@person`, `#label`, `EP-` prefixes in search | W2 |
| 108 | Peek on `Space` | EXCEED | 02's `peek.span`, `peek.hunk`, `peek.person`: peek shows the **evidence** behind a row, not only the record. `peek.person` never shows a teammate's transcript (OD-15) | W1 |
| 109 | Favourites with folders | MATCH | Favourites in the rail, names only, no counts (AX:144) | W2 |
| 110 | Label views and user views | EXCEED | A person's page shows their issues **and** a one-line summary of their agents' work (TM-05 §9.2), never a transcript | W2 / TM |
| 111 | Ad hoc lists by URL | MATCH | `panacea://issues?ids=EP-1,EP-4` | W2 |

### H. Personal attention (L-112 to L-119)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 112 | My Issues with a focus order | EXCEED | *My issues* is a view. The ranking that matters is **the Desk**, which puts your issues, your agents' asks and your Orchestrator's escalations in one aging queue (AX §5) | W1 |
| 113 | Keyboard-driven Inbox | EXCEED | The Desk: answering never navigates; `j`/`k`, `a`, `d`, `Enter`; a reachable zero state (AX §5, anti-pattern 16) | W1 |
| 114 | Priority inbox | EXCEED | Fixed sections (NEEDS YOU → DECIDED WITHOUT YOU → FAILED → LANDED → RUNNING), each row with its rank reason and the consequence of doing nothing (AX §5) | W1 |
| 115 | Snooze and reminders | MATCH | Defer with a reason is a first-class verb, and deferred items age visibly (AX §7.4) | W2 |
| 116 | Desktop, mobile, chat, email digest | MATCH | Desktop and email digest, then Teams; at most one digest per 15 minutes (AX anti-pattern 8). Mobile is L-232 | W2 / W5 |
| 117 | Automatic and manual subscriptions | MATCH | Subscribed on create, assign, mention or comment; unsubscribe per issue | W2 |
| 118 | Pulse feed | EXCEED | **"Since you last looked"**: a read cursor per person per scope, four delta classes, a product-language handover whose every claim links to its span (AX §6) | W4 |
| 119 | Pulse digest, audio playback | MATCH | The same handover as a daily or weekly digest; audio later | W4 |

### I. Documents and editor (L-120 to L-130)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 120 | Documents on projects, teams, issues | EXCEED | For a product or a feature the document **is the brief**, on the thread. Free documents `dc-` for teams and initiatives come later | exists / later |
| 121 | Real-time multiplayer editing | MATCH | Team mode's TM2, "one document" (TM-05 §12.2); the phase 5 canvas is multiplayer (OD-11) | TM |
| 122 | Document templates | MATCH | Brief templates per team | W4 |
| 123 | Version history with revert | EXCEED | Brief revisions are already immutable, digested and keyed (`brief:bf-…@n`); an agent's context names the revision it was given | exists |
| 124 | Per-passage human or agent authorship | EXCEED | Every text field carries its author class, `text·H` or `text·A` (TM-05 §3.3), and agent text is fenced from other users' workers (TM-05 §7) | W1 |
| 125 | Link to a heading | EXCEED | A brief section is a **provenance key** that design frames and agent tasks cite (the thread, §2.3) | exists |
| 126 | Markdown shortcuts, slash menu | MATCH | One editor for issues, comments and briefs | W2 |
| 127 | Mermaid, tables, collapsibles, callouts | MATCH | Tables and callouts in W2; Mermaid later | W2 / later |
| 128 | Auto-embeds (video, Figma) | SKIP | Third-party embeds pull outside content into a regulated tool and defeat egress control. Links unfurl as text. **Our own canvas frames embed natively** (L-208) | — |
| 129 | Copy as Markdown for AI tools | MATCH | Copy carries the issue's Hub key, so pasted text keeps its provenance | W2 |
| 130 | Agent edits with changes highlighted | EXCEED | An agent's edit to a brief or issue is a **proposal** (`text·A`), shown as a diff, that a person accepts | W4 |

### J. Customers, Asks, SLAs (L-131 to L-142)

Our users are teams building an application for a business owner or a client, not support desks.
We keep **intake** (requests from people outside the team) and skip the CRM.

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 131 | Customer object with revenue and tier | SKIP | CRM data. "Who is it for" is already the brief's `audience` answer, and revenue data does not belong in a delivery tool's classification | — |
| 132 | Requests with the source quote, linked | EXCEED | An intake request `rq-` keeps the requester and the verbatim quote, linked to issues **and to the brief criterion** the ask became (§3.5) | W5 |
| 133 | Mark a request important | MATCH | A flag on the intake record | W5 |
| 134 | Customer pages | SKIP | Goes with L-131 | — |
| 135 | Prioritise by count, tier, revenue | SKIP | Revenue and tier go with L-131. An issue lists its requesters by name instead | — |
| 136 | Customer attribute sync | SKIP | Goes with L-131 | — |
| 137 | Asks from chat with a synced thread | MATCH | Teams first, Slack next; the thread syncs to the intake record | W5 |
| 138 | Asks by email | MATCH | As L-31 | W5 |
| 139 | Web forms behind IdP sign-in | MATCH | A Hub-hosted form behind Entra (OD-6) | W5 |
| 140 | Requester kept updated where they asked | EXCEED | The requester hears when it **shipped**, with the release that proves it (manifest, §2.3), not only that a ticket closed | W5 |
| 141 | SLA rules, business days | SKIP | A support-desk feature. Triage age is shown everywhere as "waiting 2d 4h" (AX §6.7), which covers an internal team. Revisit if a support team adopts us | — |
| 142 | SLA risk states and analytics | SKIP | Goes with L-141 | — |

### K. Releases and analytics (L-143 to L-154)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 143 | Release pipelines, continuous or scheduled | EXCEED | Phase 7 **deploys**, not only records: reserve, confirm, and the Hub pins the brief revision (04 §1). Linear records a release that CI made | exists |
| 144 | Freezable release stages | MATCH | A freeze flag per environment on the Deploy lens | W4 |
| 145 | Monorepo path filters | EXCEED | Path globs **and** several repositories per project (a Hub project binds several remotes) | W4 |
| 146 | Issues attach from CI commit scans | EXCEED | From the `Panacea-Issue` trailers **the product writes** at land (04 §5.2), not from words a person typed in a commit message | W1 |
| 147 | Agent-drafted release notes, changelog | EXCEED | Notes grouped by the **brief criteria this release settles**, each line linked to its evidence; drafted through the ZDR gateway | W4 |
| 148 | Release events drive status and tools | EXCEED | "Shipped" is a fact the manifest proves, shown on the issue: "in release to production, 14:02, pinned `brief:bf-…@6`" (04 §2.4); webhooks carry it (L-251) | W4 |
| 149 | Release CLI and CI action | MATCH | `panacea release` plus an Azure Pipelines task and a GitHub Action, for CI outside us | W5 |
| 150 | Insights: count, effort, cycle, lead, triage time | MATCH | Measured durations and counts only, on a secondary surface; never on the rail or the Desk (Q1) | W4 |
| 151 | Scatter with percentiles; points open issues | MATCH | Same, and every point opens its issue (AX Principle 7) (Q1) | W4 |
| 152 | Dashboards | MATCH | A secondary surface under Work, never the landing page; the Desk stays primary (AX §5) (Q1) | later |
| 153 | CSV export | MATCH | Issues, views, features and intake | W2 |
| 154 | Warehouse and spreadsheet sync | MATCH | Through the API and webhooks; named connectors later | later |

### L. Linear's own AI (L-155 to L-178)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 155 | Assistant chat with workspace context | EXCEED | Your **Orchestrator** is the assistant, and it can act: plan, spawn and direct your workers (TM-05 §4). Its panel is not chat-bubble chrome (AX anti-pattern 14) | TM |
| 156 | `@assistant` in comments and chat | MATCH | `@me:orchestrator` in a comment routes to **your own** Orchestrator only (OD-16) | W3 |
| 157 | Acts only within the invoker's permissions | MATCH | Every agent runs as its user, on a gateway token attributed to that user (OD-19; TM-05 §2.4-2.5) | exists |
| 158 | Guidance at two levels; admin kill switch | MATCH | Org, team and personal guidance; an org switch disables Orchestrators; *pause all* on the horizon bar (AX §5) | W3 |
| 159 | Saved Skills, personal or team | MATCH | Claude Code's own skills, shipped in the sandbox image or a team library; the harness is unmodified | W5 |
| 160 | Loops: event or schedule triggers | EXCEED | Scheduled and event turns of **your** Orchestrator (TM-05 §4.2 is already event-driven), bounded by your autonomy grid (AX §7) and billed to you | W5 |
| 161 | Loop permissions, history, versions, run now | MATCH | Each loop is a Hub record with versions and a run log | W5 |
| 162 | Grounded codebase Q&A | MATCH | An **ask** worker: read-only session on the repo; answers link files and lines | W3 |
| 163 | Coding sessions in a managed sandbox, draft PRs | EXCEED | Long-lived, steerable, **many in parallel** per person, one clone each (OD-2), Kata microVMs, Azure DevOps **and** GitHub (OD-4), landed through a reviewed queue (phase 3) | exists / TM |
| 164 | Per-repo environment, secrets, prep scripts | MATCH | Phase 8's sandbox image plus a per-project environment spec. **UNVERIFIED** that phase 8's plan has prep scripts | phase 8 |
| 165 | Per-repo agent and model choice | MATCH | Harness per session; model per project through the gateway (OD-19) | phase 6 |
| 166 | Verification artefacts, browser testing | EXCEED | Artefacts become **proposed evidence on a criterion**, recorded by a person (04 §4.6). Browser testing in the sandbox is later and **UNVERIFIED** on Kata | W3 / later |
| 167 | Steer a running session; notify for input | EXCEED | Take the keyboard in the real PTY at the point of difficulty (AX §8.7); the Desk shows the ask with 12 lines of transcript (AX anti-pattern 6) | exists |
| 168 | First-pass bug fixing from Triage | MATCH | A triage loop: "on accept, Start" for a label, within the owner's grid; evidence is still a person's act | W5 |
| 169 | Shared AI credit pool | EXCEED | Org gateway with attribution per user, team, project **and issue** (04 §7.7), quotas, no consumer subscription (OD-19) | phase 6 |
| 170 | Code review inside, merge from inside | EXCEED | The Merge lens reviews lanes before land; each hunk links to the span that wrote it and why (AX §8.5); both git hosts (OD-4) | phase 3 |
| 171 | Syntax-aware diff, file categories | MATCH | Later, through a permissively licensed differ (licence gate, OD-11 rule 3) | later |
| 172 | Guided review of a large change | EXCEED | Review grouped by **the criterion each change serves**, core first; each hunk has its span | later |
| 173 | Agent review comments with name and model | MATCH | Every agent comment names owner, agent, model and session; team mode always shows the owner ("Alice's agent") | W3 |
| 174 | PR risk score | EXCEED | **Named risk facts, not a score** (AX anti-pattern 12): touches prod config, runs a migration, edits outside declared scope (AX §7.6) | W3 |
| 175 | Open an issue in Claude Code or Cursor | EXCEED | **Start** opens it in a managed cloud session with the issue in context (04 §4.2); local mode stays (OD-20) | W1 |
| 176 | Signed commits for agent sessions | MATCH | Signed land commits; **UNVERIFIED** whether phase 3's plan signs today | later |
| 177 | Magic word in a PR body files an issue | MATCH | For PRs made outside the product, on both hosts | W5 |
| 178 | Agent files issues from a support thread | MATCH | Intake from a Teams or email thread keeps the quote (L-132) | W5 |

### M. Agent platform (L-179 to L-197)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 179 | Agents are app users, not seats | MATCH | An agent is a session **owned by a person**; its cost is gateway usage (OD-19), never a seat | exists |
| 180 | Admin installs an agent per team | MATCH | A per-team harness allow-list (Claude Code first, others as adapters) | W5 |
| 181 | Delegate is not assignee | MATCH | The same principle, and OD-16 makes it structural | W1 |
| 182 | Agent pages; filter by delegate | EXCEED | Filter "my agents" or "Alice's agents"; an agent's page is its **task card**: task, reason (a brief link), status, files observed, leases (TM-05 §0) | W3 |
| 183 | Agent guidance with team override | MATCH | Guidance records with history, org → team → person | W3 |
| 184 | Session states derived from activity | EXCEED | Ours derive from **facts observed outside the agent** (waiting on input, permission prompt, exit code; AX §8.1), not from what the agent reports; one closed set of 7 (AX anti-pattern 4) | exists |
| 185 | Activity grammar (thought, action, …) | EXCEED | Tool-call steps folded from the transcript adapter (OD-8): `pytest -q · 43s · exit 1 · 812 lines` (AX §8.3). Degrades to commit-level facts when the adapter cannot read | W3 |
| 186 | Ephemeral activities | MATCH | One live line per agent row (02 `row.three`), on the one heartbeat clock | W3 |
| 187 | Agent plan checklist | EXCEED | The plan is the Orchestrator's tasks `tk-`, each with its reason and scope globs, which take leases (TM-05 §6.3). No "3 of 7" count (02, DON'T COPY) | TM / W3 |
| 188 | External URLs from a session | MATCH | Session, lane and preview links on the issue | W3 |
| 189 | Signals: stop, auth, select | MATCH | Stop is pause or abandon; select is a Desk item with up to 4 options (TM-05 §6.5); auth is not needed because the gateway token replaces `/login` (OD-19) | exists |
| 190 | 5 s webhook, 10 s first activity, idle | MATCH | `budget.ack ≤10s` (02 C8); "last output 3s ago" as the honest idle fact (AX §8.8) | W3 |
| 191 | Ready-made prompt context | EXCEED | The managed block names `issue:is-…@r` beside the brief's key (04 §4.2), so **what the agent was told** is recoverable byte for byte | W1 |
| 192 | Repository suggestions with confidence | EXCEED | A project already knows its repos; with several, the Orchestrator proposes one, with no confidence number unless calibrated (AX anti-pattern 12) | W3 |
| 193 | Agent-initiated sessions | MATCH | An Orchestrator may file an issue (`source: orchestrator`, 04 §2.4); it lands in `triage` for a person | W3 |
| 194 | Published agent interaction rules | MATCH | We publish ours, drawn from `AGENTIC-UX.md` and TM-05 §7 (owner, evidence, fences) | W3 |
| 195 | Partner agent directory | SKIP | Third-party SaaS agents bring their own inference and break the ZDR promise (OD-19). We run harnesses as unmodified CLIs in our sandboxes; more harness adapters later | — |
| 196 | Hosted MCP server, read-only mode | MATCH | A Work MCP server on the Hub behind Entra OAuth, with read-only mode. Agents **inside** our sandboxes use the `panacea` verb instead (TM-05 §0) | W5 |
| 197 | MCP connectors for the assistant | MATCH | Behind the admin allow-list (L-256) | W5 |

### N. Integrations (L-198 to L-220)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 198 | Copy branch name; auto-assign, auto-start | EXCEED | **Start** makes the lane and branch itself. For work outside the product, copy `ep-142-slug`; the first push is the `started` fact | W1 / W5 |
| 199 | Closing and non-closing magic words | MATCH | For PRs made outside the product, on both hosts | W5 |
| 200 | Commit linking by webhook | EXCEED | In-product commits carry trailers the product writes (L-146); host webhooks cover outside commits | W1 / W5 |
| 201 | PR-stage automations with branch rules | EXCEED | Our states follow lane facts (bound, queued, landed); host PR stages apply to outside PRs | W1 / W5 |
| 202 | GitHub Issues sync | MATCH | GitHub Issues **and Azure Boards**, one-way or two-way, so teams can keep their tracker (03's open tension) | W5 |
| 203 | PR linkback comments, autolinks | MATCH | Both hosts | W5 |
| 204 | GitHub Enterprise Cloud and Server | MATCH | Phase 3's host interface (OD-4); **UNVERIFIED** that Server is in its plan | phase 3 |
| 205 | GitLab | SKIP | OD-4 names Azure DevOps and GitHub. GitLab fits the same host interface later | — |
| 206 | Slack | MATCH | After Teams; KPMG runs on Microsoft | later |
| 207 | Microsoft Teams | EXCEED | First-class: intake, feature feeds, the Desk digest, and asking **your own** Orchestrator from Teams (OD-16) | W5 |
| 208 | Figma frame previews and plugin | EXCEED | Our own canvas (OD-11): frames are native, cite brief sections, and agents edit them over MCP. Figma link previews later | phase 5 |
| 209 | Sentry | MATCH | Alert intake: Azure Monitor first (OD-18), Sentry next | later |
| 210 | Zendesk | SKIP | Support desk (group J's reason) | — |
| 211 | Intercom | SKIP | Support desk | — |
| 212 | Front | SKIP | Support desk | — |
| 213 | Salesforce | SKIP | CRM (L-131's reason) | — |
| 214 | Gong | SKIP | Sales calls; also puts client voice data in scope | — |
| 215 | Jira coexistence sync | MATCH | Two-way with Jira, as L-202, so adoption needs no big bang | W5 |
| 216 | Notion embeds | SKIP | L-128's embed reason; notion.com is also blocked on the corporate network (03) | — |
| 217 | Zapier, Discord, Airbyte, Sheets, GUS | SKIP | The API and webhooks cover them; a Power Automate connector is the Microsoft-native answer, later | — |
| 218 | Incident tools for triage rotation | MATCH | Rotation import from PagerDuty or Opsgenie | later |
| 219 | Directory of 250+ integrations | SKIP | A short curated set (Azure DevOps, GitHub, Teams, Entra, Jira) serves regulated teams better than breadth; the API and MCP let others build | — |
| 220 | Importers | MATCH | Jira, Azure Boards, GitHub Issues, Linear (through its public API) and CSV; others later | W5 |

### O. Keyboard, speed, apps (L-221 to L-235)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 221 | Contextual command menu `Ctrl+K` | MATCH | The palette is built (phase 2); W1 makes it open beside a row for that row's actions. Cockpit chords win over a focused terminal, with OD-9's toggle | exists / W1 |
| 222 | Command-menu prefixes | MATCH | Prefixes scope to issues, people, sessions or commands (§4.3) | W1 |
| 223 | Key grammar: property letters, `G`, `O`, `M` | EXCEED | The same grammar, live **only when focus is in a list**, never in a terminal (02; extends `isCockpitChord` in `useKeyboardMap.ts`); it also reaches agents: `Shift+S` Start, `Shift+D` Delegate (§4.3) | W1 |
| 224 | `?` shortcut sheet | MATCH | Generated from the same key map, so it cannot drift | W1 |
| 225 | Multi-select and bulk bar | EXCEED | Bulk acts include *Start 4 lanes* and *Delegate 4 to my Orchestrator*; irreversible bulk acts confirm in place (02 `confirm.inplace`) | W1 |
| 226 | Menus show every shortcut | MATCH | Every menu item renders its chord from the key map | W1 |
| 227 | Safe-triangle submenus | MATCH | 02 S7's pattern | W1 |
| 228 | Local-first client database | MATCH | An in-memory **Hub replica** with optimistic reversible writes (02 C8). In memory only, because no client persists Hub data (ADR-H1:467-474) | W1 |
| 229 | Sync log with checkpoint delta sync | MATCH | The Hub's `/v1/changes` is already a gapless per-org feed (hub-contract:1089-1107): reconnect with the last sequence seen, receive only later changes | exists / W1 |
| 230 | Offline queue that survives restart | SKIP | Surviving a restart means writing Hub data to disk, which ADR-H1 forbids, and the product is cloud-first (OD-20). The in-memory queue survives a network drop, not a restart | — |
| 231 | Desktop apps with tabs | EXCEED | Tauri desktop on Windows now; the same code builds for macOS **and Linux**, which Linear lacks (01 §23.6). **UNVERIFIED**: neither build has been made. Tabs later | exists / later |
| 232 | Mobile apps | SKIP | A separate product. The phone's job, the 10-second check (AX §5), is served first by the Desk digest and answer-in-Teams (L-207). Revisit (Q6) | — |
| 233 | Web app and PWA | SKIP | The desktop is the portal (OD-20). The Hub serves only intake forms and read-only issue pages for people without the app | — |
| 234 | Light, dark, custom themes, gallery | EXCEED | 02 C1: themes **generated in LCH from three seeds**, four shipped (graphite, paper, and a high-contrast pair). No gallery | W1 / W2 |
| 235 | Personal preferences | MATCH | Font size, send key, auto-assign. The home view is always the Desk (AX §5, "the default landing surface, always") | W2 |

### P. Enterprise and admin (L-236 to L-247)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 236 | Roles, including guest | MATCH | Org admin; team `admin`·`member`·`viewer` (hub-contract:324-333); guest later | exists |
| 237 | SSO with domain enforcement and JIT | MATCH | Entra, auth code with PKCE (OD-6); the org **is** the Entra tenant, so JIT is inherent. SAML for other IdPs later | exists |
| 238 | SCIM, IdP group to team | MATCH | Entra group → team mapping. **UNVERIFIED** whether the Hub plan has it | later |
| 239 | Audit log, API, SIEM streaming | EXCEED | Every Hub write appends its event in the same transaction (hub-contract:1133-1134); each event says **person or agent, and whose agent** (03's `actor_is_agent` idea); fact moves name the person whose act caused them. Sentinel streaming later | exists / W5 |
| 240 | Data regions US or EU | EXCEED | We place our own environment (OD-5), and inference has an **AU path** (Bedrock Sydney, OD-19). **UNVERIFIED**: phase 8 has not fixed the hosting region | phase 8 |
| 241 | SOC 2, GDPR, HIPAA | SKIP | An organisational programme, not a build item; KPMG's technology risk assessment is our gate | — |
| 242 | IP restrictions | MATCH | Edge allowlist in phase 8's environment, as Panacea does | phase 8 |
| 243 | Domain claiming | MATCH | Nothing to claim: an org is an Entra tenant, so a rogue workspace cannot exist | exists |
| 244 | Third-party app approvals | MATCH | Admin approval of OAuth apps on the Hub API and MCP | W5 |
| 245 | Admin control of API keys | MATCH | Personal and team-scoped keys, admin-revocable | W5 |
| 246 | AI toggles, assistant kill switch | MATCH | As L-158 | W3 |
| 247 | Uptime SLA, support, migration help | SKIP | Commercial terms, not product | — |

### Q. API and platform (L-248 to L-252)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 248 | The public API is the apps' own API | MATCH | The desktop calls the Hub's REST API as the user (ADR-H1); the same API is the public one | exists |
| 249 | TypeScript SDK | MATCH | Generated from the Hub contract, with a Go client | W5 |
| 250 | OAuth, actor-as-app, manifests | MATCH | Entra OAuth; an app acts as a person, never as a free agent | W5 |
| 251 | Webhooks with actor, previous values, HMAC | EXCEED | Built on the change feed, which is **metadata only** (hub-contract:1089-1107): payloads carry ids and changed fields, never customer text; the receiver fetches text by key with its own rights | W5 |
| 252 | Published rate limits with headers | MATCH | Per token, with headers | W5 |

### R, S. Method, cadence and AI spend (L-253 to L-257)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 253 | A published product method | EXCEED | `AGENTIC-UX.md` and `DESIGN-SYSTEM.md` are our method, with **numeric** anti-patterns a reviewer can check; a user-facing version ships in W4 | exists / W4 |
| 254 | Weekly public changelog | MATCH | An in-app changelog; making it public is KPMG's call | later |
| 255 | AI spend limits, usage by feature and user | EXCEED | Quotas per user, team and project (OD-19), plus spend **per issue and per loop** (04 §7.7) | phase 6 |
| 256 | Admin allow-list for MCP connectors | MATCH | Each user connects their own accounts; the admin allows the connector | W5 |
| 257 | Transparent AI pricing | MATCH | The gateway shows cost at the provider's rate per session and issue, and Fleet shows sandbox minutes. Pricing itself is the owner's commercial call | phase 6 |

### The tally

Counted from the rows above (257 rows, numbering checked gapless on 2026-09-25):

| Verdict | Items | Share |
|---|---|---|
| **MATCH** | 155 | 60% |
| **EXCEED** | 75 | 29% |
| **SKIP** | 27 | 11% |

**When each lands** (first slice named in the row): W1 42, W2 52, W3 16, W4 37, W5 36; **22 exist**
in plans already in flight; 13 arrive through other phases (team mode 3, phases 3, 5, 6 and 8: 10);
12 are *later*; 27 are skipped.

**The 27 skips, by reason.** Each is reversible by the owner (Q6).

| Reason | Items | Count |
|---|---|---|
| A support desk, CRM or sales feature; our users build apps, they do not run help desks | L-131, 134-136, 141, 142, 210-214 | 11 |
| Third-party embeds and long-tail connectors defeat egress control; the API covers them | L-128, 216, 217, 219 | 4 |
| Needs per-object or cross-team grants, which Hub v1 does not have | L-4, 6, 86 | 3 |
| A platform choice already made: no Hub data on disk (ADR-H1), desktop as portal (OD-20) | L-230, 232, 233 | 3 |
| A prediction, which `AGENTIC-UX.md` bans (AX:237) | L-62, 73 | 2 |
| Not a build item (certification, commercial terms) | L-241, 247 | 2 |
| Third-party SaaS agents bring their own inference and break ZDR (OD-19) | L-195 | 1 |
| Outside OD-4's two git hosts | L-205 | 1 |

**Conditional rows.** Seven rows carry **(Q1)**: L-62, 63, 73, 85, 150, 151 and 152. They follow the
recommended answer to Q1 (progress displays). If the owner chooses Linear-style graphs instead,
L-62 and L-73 become MATCH, L-63 and L-85 become MATCH, and `AGENTIC-UX.md` anti-pattern 2 is
amended.

---

## 2. The "more": what makes it magic

Linear's own trajectory (01 §13.7, 03 §1) shows it can copy features in weeks. So "more" here means
four things that need **the platform underneath**, not a screen: managed sessions the platform
observes, a server-side ledger that git trailers point into, and an inference path we own. Each
subsection ends with why Linear cannot follow quickly, and what is still unproven.

### 2.1 Agents that are real teammates on the issue

An issue goes to a **person**. Giving it to agents is that person's own act (04 §4.1):

- **Solo: Start.** One chord (`Shift+S`) on EP-142 picks the feature's branch or cuts `ep-142-<slug>`,
  creates a lane (a local clone, OD-2), and starts a managed cloud session whose record carries
  `IssueRef = issue:is-…@r` beside the brief's `ContextRef` (04 §4.2). The issue moves to
  `started`, written as that person, with the lane as the cause.
- **Team: Delegate.** `Shift+D` sends EP-142 to **your own** Orchestrator. It reads the issue with
  `panacea issue show`, plans tasks with `--issue`, and spawns workers whose sessions carry the
  issue (04 §4.3). Nobody can delegate to someone else's agents (OD-16).

What the teammate then does, visibly, on the issue:

```
EP-142  Uploads survive a dropped connection            Dylan · started 09:12
  serves  brief:bf-3a…@6 › done › c-91e2… "a 40 MB upload resumes after a drop"
  ● 2 agents · Dylan's      writing resume handler · last output 3s ago
    files  services/api/upload.go  store/chunks.go  +2
  ▸ proposed evidence  c-91e2… at 7c1f0a2 · "resume test passes on a cut at 60%"  [Record]
  ◦ note (Dylan's agent)  assumed chunk size 8 MB; the brief does not say
```

1. **It acknowledges fast.** First visible activity within 10 seconds (`budget.ack`, 02 C8; the
   rule Linear publishes, 01 L-190).
2. **It reports in one live line**, on the one heartbeat clock (DESIGN-SYSTEM; 02 `row.three`).
   The line is assembled from observed facts, never a model's summary of a transcript (TM-05 §9.1).
3. **It cannot claim progress.** `started`, `in_review` and `landed` come from facts (a lane bound,
   a lane queued, a trailer landed). Agent text is labelled `text·A` wherever it appears
   (04 §4.5).
4. **It proposes evidence; a person records it.** `panacea evidence propose` puts a typed proposal
   under its criterion; **Record** writes it as the person (04 §4.6). The issue is `done` only when
   every criterion it serves is settled at the landed commit.
5. **One voice per owner.** Workers report to their Orchestrator; only the Orchestrator writes
   notes on the issue (04 §4.5). Five workers do not produce five comment streams.

**Why Linear cannot follow quickly.** Linear's delegate model matches ours in spirit (01 L-181),
but its sessions end in a draft PR and "done on merge" (04 §2.4, linear.app/docs/github). To
say "done means proven" it would need acceptance criteria as first-class objects and evidence
recorded against them; 01 lists no acceptance-criteria object (**UNVERIFIED** that none exists).
**Unproven:** that people will press Record rather than rubber-stamp it (AX §7's calibration
record is the check).

### 2.2 Orchestrators that negotiate across owners (OD-14 to OD-17)

The work layer gives team mode a **shared language**: Orchestrators coordinate through issues
people wrote, not through each other's free text.

1. **Collisions are caught at plan time.** When Alice's Orchestrator plans EP-150 and Dylan's
   has EP-142 in flight, each task takes an `intent` lease on its scope globs. Fleet checks the
   overlap before any code is written (TM-05 §6.3) and opens a **conflict thread** between the
   two Orchestrators, with both issues, both reasons and both criteria attached (TM-05 §6.4).
2. **They negotiate with typed resolutions only**: `split`, `sequence`, `yield`, `build-on`,
   `proceed-both` (TM-05 §6.2). Each then directs **its own** workers to match.
3. **They escalate by a policy each human set in advance**: *inform* (a line under "since you
   last looked") or *decide* (a Desk item with up to four options and a deadline, whose safe
   default pauses and deletes nothing) (TM-05 §6.5).
4. **Issue relations feed the protocol.** `EP-150 blocks EP-142` across two owners becomes a
   `sequence` proposal automatically; a `duplicates` link between two people's issues opens a
   thread asking whose lane survives, which is always a *decide*.
5. **Both issues show the outcome**, as one negotiation row (02 C10 `negotiation.row`):
   *"Sequenced after EP-150 (Alice's). Agreed by both Orchestrators 10:41; Dylan informed."*
   The row links to the thread's typed state, never to the other side's transcript (OD-15).

**Why Linear cannot follow quickly.** Every Linear session is one human, one agent, one issue
(01 §23.1), and none of the ten products surveyed coordinates agents across owners (03 §12.3).
Copying it needs
live file observation of every agent, a per-user Orchestrator protocol and an injection fence
between users (TM-05 §7). 03 puts GitHub as the best placed, at perhaps 6 to 12 months. **It is a
lead, not a moat, and it is designed, not built or proven wanted.**

### 2.3 One thread, from the brief to the release, and "done means proven"

The work item sits **on** the provenance thread, not beside it (04 §5):

```
 Define                Design                Develop                                  Deploy
 brief:bf-…@6 ◄─serves── issue:is-…@3 ──► session: IssueRef + ContextRef
  └ criterion c-91e2…      │ implements      │ commits: Panacea-Session, Panacea-Issue
                           └► design:cv-…@4  ▼
                                           land: brief trailer + one Panacea-Issue per issue
                                             │ evidence (brief, criterion, commit), by a person
                                             ▼
                                           release: pins the brief; manifest lists the issues
```

What a person can do with it that no tracker offers:

1. **Ask "why does this code exist?"** From any hunk: its session, the issue revision that session
   was told, the criterion that issue serves, and the brief section behind it. One click each way.
2. **Ask "did it actually ship?"** An issue shows *"in release to production, 14:02, pinned
   `brief:bf-…@6`"*, derived from trailers in the shipped range (04 §5.4), not from someone
   moving a card.
3. **Ask "is the brief covered?"** The Work lens lists each criterion with the issues that serve
   it and its convergence: settled, open, contested. A criterion no issue serves is a visible gap.
4. **Erase customer text without rewriting git.** Trailers carry ids, never titles (04 §5.2), and
   an issue has the brief's erase (04 §5.5).

**Why Linear cannot follow quickly.** Linear Releases links issues to a commit SHA by scanning
for issue keys (01 L-146; 03 §12.2). Nobody has the front half: a brief section cited by a design
frame, carried as each agent task's reason, and attached back to the release. 03 estimates Linear
could assemble the back half in 2 to 3 quarters; the agent-edited design canvas (OD-11) is the
part it cannot copy quickly. **Unproven:** the thread only helps if people write briefs; the
low-ceremony entry (AX §9.7) must let a one-line fix skip Define without breaking the thread (an
issue with no `serves` is allowed, and shown as such).

### 2.4 Managed, zero-retention inference with an AU switch (OD-19)

- **Every agent reaches the model through the org gateway**, on a short-lived token scoped to one
  session and attributed to its user (OD-19; TM-05 §2.5). No consumer subscription, no `/login`.
- **Two zero-retention paths**: Global (direct Anthropic API, the default) and AU (Bedrock in
  Sydney). A project classified "AU data only" forces AU; otherwise a person can switch.
- **The issue carries the bill.** With the issue id on the gateway token (04 §7.7), cost is known
  per issue, per feature and per loop (L-255), at the provider's rate.
- **Every AI feature in the matrix uses the same gateway**: triage suggestions, duplicate search,
  drafted updates, release notes. No work-layer text goes to a third-party AI service.

**Why this is not enough alone.** 03 §12.4 is blunt: zero retention is **hygiene**. Atlassian
already states its providers retain nothing. What is ours is the combination: an AU path per
project, attribution per user and per issue, and Linear's data regions being US or EU only with
some metadata always in the US (01 §16.6; **UNVERIFIED** whether Linear offers a ZDR option on
request). Both parts could be copied in weeks.

### 2.5 The other benefits it mixes with

The owner asked for these mixed in. Each already exists in design, and the work layer feeds it:

| Benefit (source) | What the work layer adds to it |
|---|---|
| **The Desk**, a queue you empty (AX §5) | Issues that need you join its existing sections; it replaces Linear's Inbox, My Issues and triage-for-me in one place |
| **The horizon bar**, the next 15 minutes (AX §5) | Collisions forecast between issues, not only between files: "EP-142 and EP-150 will meet in `upload.go` ~14:20" |
| **Since you last looked** (AX §6) | The handover names issues: "EP-142 landed; EP-150 waits on your decision since 23:40" |
| **A fleet on one heartbeat** (02 C10 `fleet.beat`) | Thirty agents across twelve issues on one clock; Linear shows one text timeline per issue (01 §23.7) |
| **Azure DevOps and GitHub** (OD-4) | Issue links, trailers and sync on both; Linear's coding sessions need GitHub (01 §23.4) |
| **Long-lived, steerable cloud sessions** (OD-20; 03 §12.1) | Take the keyboard at the exact point of difficulty; the issue records that a person stepped in |

### 2.6 The honest scorecard

| The "more" | Status | Lead over Linear (03's estimate) |
|---|---|---|
| Agents as teammates with evidence | designed; builds in W1 and W3 | months: needs criteria and evidence objects |
| Cross-owner Orchestrators | designed (TM-05); TM1 not started | 6-12 months for GitHub; longer for Linear |
| One thread, brief to release | back half planned (phases 3, 4, 7); front half needs phase 5 | 2-3 quarters for the back half |
| ZDR with an AU switch | designed (phase 6 re-scope) | weeks |

No rival has more than two of the five parts of the whole line: brief, design, per-person agents,
cross-person negotiation, and release on one thread (03 §12.5). **The combination is the product.**

---

## 3. The object model, and where each object lives

This section adopts 04's recommendation unchanged (04 §2, §3) and adds the records the parity
matrix needs beyond it: request, template, label, milestone, loop and draft.

### 3.1 The placement rule

TM-05's rule decides every row: *if losing it would lose a decision or an accountability fact, it
is a Hub record; if it is a fact about now, it is Fleet state* (TM-05 §3.2). A third home exists
for preferences only: the desktop, which may keep **no Hub data on disk** (ADR-H1:467-474).

```
org ─ team ─┬─ project pj- (the product and its repos) ─┬─ project brief ─ criteria c-
            │                                           ├─ feature ft- ─ branch brief bf- ─ criteria c-
            │                                           │    ├─ milestone ml- (a named subset of criteria)
            │                                           │    └─ issue is- ─ issue (child) ─ task tk- (agent)
            │                                           ├─ issue is- with no feature (backlog, triage, fixes)
            │                                           └─ request rq- ─► becomes or joins an issue
            ├─ cycle cy-       ◄── an issue points at one cycle, across the team's projects
            ├─ initiative in-  ◄── a feature points at one initiative
            └─ labels lb-, templates tp-, views vw-, loops lp-
```

### 3.2 The records

| Record | Id | Home | Shape | New or from |
|---|---|---|---|---|
| issue head | `is-` | Hub | mutable: state, assignee (a person), priority, estimate, labels, parent, feature, cycle, due date, relations, rank, `source` | 04 §2.4 |
| issue revision | `issue:is-…@r` | Hub | **immutable, digested**: title, description, `serves [{briefKey, criterionId}]`, `implements [design:…]`, each text with its author class | 04 §2.4, §5.5 |
| comment | `ic-` | Hub | append-only; `text·H` or `text·A`; kinds `note`, `evidence_proposal`, `negotiation`; an optional anchor (text range, criterion, span, hunk) | 04 §2.2, extended |
| feature | `ft-` | Hub | lead, target date, state, initiative, `branchRef`, `briefId`; the spec **is** the branch brief | 04 §2.5 |
| milestone | `ml-` | Hub | name, date, a list of criterion ids on the feature's brief | 04 §2.5 (named "later") |
| cycle | `cy-` | Hub | per team: number, start, end, state; the janitor rolls it over | 04 §2.5 |
| initiative | `in-` | Hub | per team in v1: owner, target, state, updates | 04 §2.5 |
| status update | `nt-`, `kind: update` | Hub | immutable; health word; `text·H`; **pins the brief revision** it reports on | TM-05's `note`, reused |
| request | `rq-` | Hub | requester (a person or an email address), verbatim quote, channel, linked issue | **new** (§3.5) |
| label | `lb-` | Hub | team-owned; optional group with `exclusive` | **new** |
| template | `tp-` | Hub | issue or brief template; may pre-fill `serves`; optional schedule (recurring issues) | **new** |
| view | `vw-` | Hub if shared; desktop if personal | a filter and a layout | 04 §2.2 |
| loop | `lp-` | Hub | trigger (event or schedule), the owner, its autonomy bound, versions, run log | **new** (L-160) |
| draft | `df-` | Hub, private to its author | unsent issue or comment text; expires after 6 months | **new** (L-33) |
| agent task | `tk-` | Hub (Fleet writes it) | team mode's task, which **gains `issueId`** | TM-05 §3.3 |
| who is on an issue now: lanes, files, leases, the live line | — | **Fleet state**, derived | never stored on the issue | 04 §2.2 |

### 3.3 The issue's states, and "done means proven"

| State | Set by | The fact behind it |
|---|---|---|
| `triage` | intake, an Orchestrator's proposal, a person outside the team | nobody has accepted it |
| `backlog`, `todo` | a person | accepted, not started |
| `started` | **a fact**, written as the person who caused it | a lane or task was bound to it |
| `in_review` | **a fact** | a bound lane was marked ready or entered the merge queue |
| `landed` | **a fact**, written as the lander | a land commit carries its `Panacea-Issue` trailer |
| `done` | a person, **or** automatically at landing | every criterion in `serves` is `settled` at that commit |
| `cancelled`, `duplicate` | a person | closed without work |

**The rule.** An issue with a non-empty `serves` moves to `done` by itself only when phase 3's
evaluator finds every served criterion settled by recorded evidence at a commit containing the
issue's landed work (04 §5.3). An issue with an empty `serves` (a small fix) stops at `landed` and a
person closes it. *Shipped* is never a state; it is derived from the release manifest (04 §5.4).

These nine are **issue** states. `AGENTIC-UX.md` anti-pattern 4's closed set of seven is the
**agent** status set; the two never share a column, an icon family or a colour (§4.4).

### 3.4 How the issue joins team mode

| Team-mode object (TM-05) | What the work layer adds |
|---|---|
| task `tk-` | `issueId`. Its reason now reads task → issue → criterion → brief (04 §7.5) |
| Orchestrator events (TM-05 §4.2) | two kinds: *an issue was delegated to you*, and *a blocker you wait on landed* |
| verbs (TM-05 §5.2) | `panacea issue show`, `panacea issue note`, `panacea evidence propose`; `panacea team` shows each agent's issue and criterion |
| leases and conflicts (TM-05 §6.3-6.4) | a conflict thread cites both issues; `blocks` across owners seeds a `sequence` proposal (§2.2) |
| the team strip (TM-05 §9.1) | the line is the **issue title** (`text·H`) where one exists, instead of the task title (`text·A`) |
| the Desk (TM-05 §9.3) | *issues that need you* join its existing sections; no new section |

**What an agent may read.** An issue revision is spec-class: it enters the agent's managed block
only after its owner adopted it (Start or Delegate), with attribution, after the Hub's schema and
character checks. A `text·A` revision is never rendered until a person accepts it. Comments are
coordination-class and are never rendered; an agent reads them only as data, through
`panacea issue show` (04 §4.4). This waits on the owner's ruling (Q3) and on TM-05's injection
corpus covering hostile issue text.

### 3.5 Requests: work from outside the team

A **request** `rq-` is how a business owner, a client or another team asks for something without
being a member (L-27, L-31, L-137 to L-140):

1. It arrives by an Entra-protected web form, an email address per team, or a Teams message.
2. It keeps the requester and the **verbatim quote**, as `text·H` of that requester.
3. Intake rules (L-49) route it to a team's `triage`, where a person accepts it into a new issue or
   links it to an existing one. Triage suggestions may propose **which criterion** it serves.
4. The requester is told, where they asked, when the issue **shipped**, with the release name,
   because shipped is a fact the manifest proves (§2.3).

A request is customer content: it has the issue's erase, and its text never enters an agent's
managed block. An agent reads it only as data.

### 3.6 Keys, events, authorisation, erase

- **One new key**, `issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})`, in the grammar the Hub mints and
  `/v1/refs` resolves; `provenance.Ref` gains `kind: "issue"` (04 §5.1). **`EP-142` is a display
  alias, never a key**, because an admin can change the project's key.
- **Every write appends one metadata-only event in the same transaction** (hub-contract:1133-1134),
  so the desktop replica and every Orchestrator learn of changes through `/v1/changes`. Webhooks
  (L-251) are the same events, delivered out.
- **Authorisation rows** (04 §7.1): a team member writes issues; only a person moves an issue to
  `done` by hand; an agent writes only comments on its own owner's behalf, and in team mode only
  its Orchestrator does; `landed` and `started` moves are written as the person whose act caused
  them. Nothing writes as "the system" except janitor acts, whose events carry `actor: null` and
  name the policy's author.
- **Erase.** Issues, comments and requests get the brief's erase: org admin only; text removed;
  keys, digests and revision numbers kept, so every trailer and chip still resolves, as *erased*
  (04 §5.5). Trailers and branch names carry ids, never titles (04 §5.2).
- **Id prefixes, checked 2026-09-25.** The Hub's own (`org-`, `u-`, `tm-`, `pj-`, `bf-`, `ap-`,
  `ln-`, `iv-`, `rl-`, `ev-` and the single-letter ones, hub-contract:105-112 in the `feat/hub`
  worktree) and TM-05's (`ag-`, `tk-`, `ls-`, `cf-`, `th-`, `ms-`, `es-`, `nt-`, `dl-`, per 04
  §7.1): the new `is-`, `ic-`, `ft-`, `ml-`, `cy-`, `in-`, `rq-`, `lb-`, `tp-`, `vw-`, `lp-`,
  `df-` collide with none. Milestones take `ml-` because `ms-` is TM-05's.

---

## 4. The experience: one app, not a tracker bolted on

### 4.1 Where the work layer appears

The rail stays navigation only: project → branch → worktree → session, names and no counts
(AX:144). The work layer adds **one lens and one project surface**, and chips everywhere else.

| Surface | Scope | What it holds | New? |
|---|---|---|---|
| **The Desk** | the account, above projects | issues only when they need you, in the Desk's existing sections (§4.2) | Desk designed, **not built** (§5.4) |
| **Work** (project surface) | one project | Triage, Backlog, the current cycle, the features board, initiatives, saved views, *My issues* | new |
| **Work lens** | one feature (its branch) | its issues, **the brief's criteria with the issues that serve each**, who is on each issue now | new; `Lens` gains `"work"`, the seventh lens |
| **Issue view** | one issue | head, revision history, `serves`, relations, comments, the live agent line, proposed evidence, *shipped to* | new |
| **Discover, Design, Develop, Merge, Deploy** | a branch | an issue chip beside the brief chip on every artefact that carries one (§4.5) | chips new |
| **Palette** `Ctrl+K` | everywhere | every work verb, with its chord shown (L-226) | built; gains commands |

The Work surface, drawn to the design system's rules (plain group sizes follow Q1's option A):

```
┌ EP · Evidence Portal ─ Work                        Global ▾   view: Current cycle ▾   ⌘K ┐
│ CYCLE 14 · 22 Sep – 3 Oct                        scope log: +3 on day 4 (Priya, Dylan)   │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│ IN REVIEW 2                                                                              │
│ ◇ EP-142  Uploads survive a dropped connection      Dylan   ● 2 agents · last 3s  ┊c-91e2 │
│ ◇ EP-139  Reviewer sees evidence age                Priya   queued 2nd            ┊c-40aa │
│ STARTED 3                                                                                │
│ ▷ EP-150  Chunked upload API                        Alice   ● 1 agent · Alice's   ┊c-91e2 │
│   ⇄ sequenced after EP-142 · agreed 10:41 · you were informed                           │
│ TODO 4                                                            2 new · N to show     │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

- `┊c-91e2` is a `thread.node` (02 C2): the criterion the issue serves, in mono. Hovering it lights
  every issue, frame and lane on the same criterion (`thread.lit`).
- The agent line is Fleet state, on the one heartbeat (02 `fleet.beat`); a teammate's agents are
  labelled as theirs (02 `who.label`) and are never steerable from here (OD-16).
- The `⇄` row is 02's `negotiation.row`: the outcome, with a link to the typed thread.
- "N to show" is 02's `arrivals.hold`: new rows wait behind one line and never push the list. 02
  binds it to `R`, which 02 C9 also gives to *rewind*; this design moves arrivals to `N`.
- The state glyphs (`◇` in review, `▷` started) are placeholders for our own drawn set (§4.4). Each
  is a distinct shape per state; none fills as work progresses (02 C13).

### 4.2 The Desk absorbs Inbox, My Issues and triage-for-me

No new Desk section (TM-05 §9.3). Issue rows join the fixed ones (AX §5):

| Desk section | Issue rows that join it |
|---|---|
| NEEDS YOU | assigned to you in `triage`; an urgent issue assigned to you; `in_review` where you are the reviewer; a comment naming you with a question; **your issue is blocking someone's agent** (with the cost of the wait); a `decide` escalation about an issue |
| DECIDED WITHOUT YOU | an Orchestrator agreement within your policy that changed your issue (sequenced, split), with Undo |
| FAILED | a lane bound to your issue failed; an intake rule could not route a request |
| LANDED | your issue landed; a criterion it serves became settled; your issue **shipped** |
| RUNNING | your agents on your issues, one line each |

Every row answers in place and never navigates (AX §5): accept, snooze with a reason, record
proposed evidence, answer a question in one line. The zero state stays reachable: *"Nothing needs
you. 7 agents on 4 issues. Next decision expected ~14:22."*

### 4.3 The keyboard and command model

We adopt Linear's **idea** of a grammar (letters open properties, `G` goes, `O` opens, menus teach
the keys; 01 §15.1) and write our own letters. Linear's exact bindings are **UNVERIFIED** (02 D4:
its shortcut page returned 404) and are not copied. Two rules come first:

1. **Single keys and sequences fire only when focus is in a list** (02 C9 `keys.list`), never in a
   terminal. This extends `isCockpitChord` in `desktop/src/hooks/useKeyboardMap.ts`. `Ctrl+K`
   works everywhere, as built, with OD-9's toggle.
2. **Lower case edits the record; `Shift` hands work to your agents.** A reader can tell from the
   key alone whether an agent will run.

| Key (list focus) | Meaning | Where |
|---|---|---|
| `J` `K`, `↑` `↓` · `Enter` · `Esc` | move · open or dive · back | every list |
| `Space` tap / hold | peek: `peek.span`, `peek.hunk`, `peek.person` (02 C9) | every list |
| `X`, `Shift+X` | select, select a range; the bulk bar appears | every list |
| `/` · `?` · `U` · `N` | search · shortcut sheet · undo within `--undo-window` · show held arrivals | every list |
| `A` · `D` · `R` | approve · defer with a reason · rewind an assumed decision | Desk items (AX §5; 02 C9) |
| `M` | ask **my** Orchestrator about this row (TM-05 §9.2's one action) | every list |
| `C` | new issue; on a Desk row or a span, the new issue links to it | every list |
| `I` | assign to me | issues |
| `S` `P` `L` `E` `Y` `F` `B` | pick state · priority · label · estimate · cycle · feature · relation | issues |
| `1` `2` `3` `4` | accept · duplicate · decline · snooze | Triage view only |
| `Shift+S` | **Start**: a lane and a managed session on this issue | issues |
| `Shift+D` | **Delegate** to my Orchestrator (team projects) | issues |
| `G` then `D` `W` `I` `T` `C` `P` `M` | go to Desk · Work · my issues · Triage · current cycle · projects · my Orchestrator | outside a terminal |
| `O` then `I` `F` `P` | open an issue by number · a feature · a project | outside a terminal |

**The palette** gains every verb above, each row showing its chord (DESIGN-SYSTEM: hotkey hints
inline). Prefixes scope it: `#` issues, `@` people, `>` commands, `~` sessions. It opens beside
the row it acts on when invoked from a row (L-221).

### 4.4 The design-language uplift, applied to work

02's Part C is adopted as written, with three work-specific rules and one correction:

| From 02 | Applied to the work layer |
|---|---|
| C1 `theme.generate` | Four generated themes; the Work surface is tested in all four (C12 `stress.sets`) |
| C2 `thread` | **The signature.** `thread.node` on every issue that serves a criterion; `thread.trace` grows from criterion to issue to lane on drill-in |
| C3 `voice.split` | Issue titles and descriptions are sans; ids, keys, paths, costs and durations are mono |
| C4 density | Issue rows at `--row-h` 32px; `compact` 28px; group headers carry plain sizes only (Q1) |
| C5 `chrome.dim`, one `ViewHeader` | Work, the Work lens and the issue view use the one header, with the route badge `Global`/`AU` |
| C6 icons | **Issue states get our own drawn glyphs**, one distinct shape per state, in ink; none fills with progress |
| C7 motion | A state change flashes over `--dur-flash`; `arrivals.hold` for new issues; FLIP only on explicit re-sort |
| C10 agent activity | The agent line on an issue is `row.three` compressed to one line; `who.label` on every agent string |
| C11 first run | `first.landing`'s rehearsal task **is a real issue**, EP-1, started by the user's first `Shift+S` |
| C13 not-a-clone test | Applies to every work screen; a filling status icon or an indigo accent fails the review |

**Work-specific rules.**
1. **Issue states and agent states never share colour.** Issue glyphs are neutral ink. Colour
   appears on an issue only when it carries a status: `--need` when it needs you, `--fail` when a
   bound lane failed (DESIGN-SYSTEM: colour is signal, and status carries a shape as well).
2. **Priority is a shape, not a colour,** except urgent, which is `--need` only on the assignee's
   own rows.
3. **No count outside a group header or the Desk** (AX:245, as amended by Q1).

**One correction to 02.** 02 C8 says *"Tauri stores [the replica] on disk"*. ADR-H1:467-474 says no
client persists Hub data to disk, and 04 §2.2 holds to it. This design follows ADR-H1: the replica
is **in memory**. Consequence: after the app restarts (not after sleep), the first paint needs a
full load, so 02's `budget.return ≤ 2s` is **UNVERIFIED** for large projects. Q5 asks the owner.

### 4.5 How it fits Define, Design, Develop and Deploy

The four phases are states of one project (`CLAUDE.md`), and `AGENTIC-UX.md` §9 item 12 forbids
showing them as a pipeline. So the work layer is **not** a fifth phase and has no screen of its own
that the others hand off to. An issue is one object that every lens shows at its own point:

| Lens | What the issue looks like there | New behaviour |
|---|---|---|
| **Discover** (Define) | each criterion lists the issues that serve it; an unserved criterion is marked | *Propose issues*: one draft issue per unserved criterion, `text·A` until a person accepts (W4) |
| **Design** | a frame shows the issues whose revision `implements` it | a design approval can close a design issue (04 §7.7) |
| **Develop** | each session pane's header shows its issue chip beside the brief chip | `Shift+S` from an issue lands here, the session already told |
| **Merge** | each lane row names its issue; the definition of done shows proposed evidence under each criterion | *Record* writes the evidence as the person (04 §4.6) |
| **Deploy** | the release manifest lists the issues shipped, from trailers | each issue gains *shipped to production, 14:02* |
| **Work** (new) | the feature's issues against the brief's criteria: coverage, not a burn-down | the planning surface; everything else links here |

**Four rules that keep it one app.**
1. **One issue view.** Every chip in every lens opens the same view, at the revision the chip names.
2. **No second tree.** The rail does not gain issues, cycles or initiatives. They live on the Work
   surface and in the palette.
3. **Entry accepts any depth** (AX §9 item 7). *"Fix this"* is an issue with no brief work; *"an
   idea"* starts in Discover; *"a ticket from Jira"* arrives as a request. None is forced through
   another.
4. **The thread is visible where work is.** Wherever an issue appears, its `thread.node` is there
   and lights its criterion, frame, lane and release.

### 4.6 Speed

| Act | Class (02 C8) | Budget (proposed, unmeasured) |
|---|---|---|
| change state, priority, label, cycle, assignee; defer; mark seen | `optimistic.undo` | `budget.paint` ≤ 16 ms to the changed row |
| open the palette; search | replica read | `budget.palette` ≤ 50 ms |
| peek an issue, span or hunk | replica read, spans stream after | `budget.peek` ≤ 100 ms |
| Start or Delegate | reversible until the session's first write | row resolves at once; `budget.ack` ≤ 10 s to the first span, else "no output yet · 10s" |
| land, push, deploy, erase | `confirm.inplace`, never optimistic | the consequence line and a two-step confirm inside the row |

Each budget gets a Playwright timing test against the local app (the repo's recipe), recorded
rather than asserted until the owner accepts the numbers (02 C8).

---

## 5. Phased delivery: the minimum lovable slice first

Proposed as **Phase W, the work layer**, lettered like Phase H so no phase number moves (04 §7.6).
Sizes are **focused days for one implementer who knows the code**, estimated from the existing
plans' task sizes (04 §6.1) and 02 D2. **None is measured.** A prototype comes first and the owner
reviews it before any plan (`CLAUDE.md`); OD-1's waiver covered phases 3, 4, 5 and 7 only.

### 5.1 The slices at a glance

| Slice | The promise, in one line | Size | Depends on |
|---|---|---|---|
| **W0** now | Two one-line changes that are cheap today and rework later | < 0.5 d | nothing |
| **W1** minimum lovable | *"I file it, my agent takes it, and it is done only when it is proven"* | ~27.5 d | Hub through task 43; phase 4 tasks 06, 09, 16-17; phase 3 tasks 05, 09; phase 7 task 19 |
| **W2** planning | Cycles, features, sub-issues, comments, views: a team can plan in it | ~24.5 d | W1 |
| **W3** agents on the work | Delegate, Orchestrator notes, proposed evidence, negotiation on issues | ~11.5 d | W1 and team mode's TM1 |
| **W4** roadmap and release | Initiatives, timeline, updates, release notes by criterion, insights, AI assists | ~22.5 d | W2; phase 6's gateway for the AI rows; phase 7 |
| **W5** intake and ecosystem | Requests, tracker sync and import, Teams, public API, MCP, Loops | ~30.5 d | W2 |
| | **Total** | **~117 d** | |

About **23 weeks for one implementer**; on three streams (Hub, pipeline phases, desktop) W1 is
about **3 weeks elapsed**, with the desktop the long pole. The whole layer is the size of a large
phase. That is the honest cost of the owner's "everything they are doing"; §6 Q2 offers smaller
stopping points.

### 5.2 W0: now, inside the current wave (04 §7.1)

1. Add `issue:` to phase 4 task 06's key parser (`provenance/key.go`, not yet built) and to the
   Hub's key grammar (hub-contract:114-128). One more case now; a change to a built parser later.
2. Fix the stale example `criterionId: "done:2"` at hub-contract:1065 and :1079 to a `c-` id.
3. Optional, unblocked: 02 D2 step 1, the missing tokens (`--dur-flash`, `--dur-peek`,
   `--undo-window`, `--s-*`, `--row-h*`, `--t-*`, `--icon*`) in `tokens.css`, about 2 hours.

### 5.3 W1: the minimum lovable slice, solo mode (~27.5 d)

**The demo that makes it lovable.** Press `C`, type a title, pick the criterion it serves. Press
`Shift+S`. Within 10 seconds a cloud agent is working on it and its line is live on the issue. It
lands; the issue shows `landed`. You record one piece of evidence; the issue turns `done` by itself,
because its criterion is settled. Deploy, and the issue says *shipped to production, 14:02*.

| # | Work | Where | Size |
|---|---|---|---|
| 1 | Issue head and revision, `issue-revision` canonical kind, the `issue:` key in `/v1/refs`, `project.key` and numbers | Hub | 3.5 d |
| 2 | Relations (`blocks`, `duplicates`, `relates`), team labels, `triage` and team policies | Hub | 1.5 d |
| 3 | Events for every write, authorisation rows, issue erase, title search | Hub | 2.5 d |
| 4 | `Session.IssueRef`, the Materialiser's second key, `provenance.Ref` kind `issue` | phase 4 | 2 d |
| 5 | `Panacea-Issue` trailer, land trailers and the `landed` move, coverage data, the `done` rule | phase 3 | 2 d |
| 6 | Issues in the release manifest; *shipped to* derived | phase 7 | 1 d |
| 7 | Prototype of Work, the issue view and the Desk rows, for owner review | desktop | 1.5 d |
| 8 | Work surface: Triage, list and board, grouping, display options, bulk bar | desktop | 3 d |
| 9 | Issue view: head, revisions, `serves` with `thread.node`, relations, the live agent line | desktop | 2 d |
| 10 | `keys.list` grammar, number-key triage, `peek.span`, the `?` sheet, palette commands | desktop | 2.5 d |
| 11 | In-memory Hub replica for issues on `/v1/changes`, `optimistic.undo`, `confirm.inplace` | desktop | 2.5 d |
| 12 | Issue chips in Develop and Merge; Start from an issue | desktop | 0.5 d |
| 13 | **A minimal Desk** with NEEDS YOU and LANDED, issue rows, answer in place, zero state (§5.6) | desktop | 3 d |
| | Hub 7.5 · phases 3, 4, 7: 5 · desktop 15 | | **~27.5 d** |

**Out of W1, on purpose:** comments, cycles, features, views (W2); anything that needs team mode
or the `panacea` verb channel (W3); anything that needs the gateway (W4).

### 5.4 W2 to W5

**W2, planning (~24.5 d).** A team can run its fortnight in it.

| Work | Size |
|---|---|
| Comments `ic-`: mentions, reactions, anchors (text, criterion, span, hunk), resolve; subscriptions (Hub) | 2 d |
| Features `ft-`: branch reservation, *promote to feature*, the features board and tabs (Hub 1.5, desktop 2) | 3.5 d |
| Cycles `cy-`: janitor rollover, cooldowns, iCal feed, the scope log, convergence per criterion (Hub 1.5, desktop 1.5) | 3 d |
| Sub-issues, templates `tp-`, label groups, estimates, due dates, rank, move, drafts `df-` (Hub 2, desktop 1.5) | 3.5 d |
| Views `vw-`: nested filters, `panacea://` links, ad hoc lists, favourites, set as default (Hub 1, desktop 2.5) | 3.5 d |
| The editor (Markdown, slash menu, tables), comment UI, attachments | 2.5 d |
| The digest (desktop and email); defer with a reason; CSV export | 1.5 d |
| 02 D2 steps 2, 3 and 5: `chrome.dim` and one `ViewHeader`, `voice.split`, generated themes | 5 d |

**W3, agents on the work (~11.5 d; ships with TM1).**

| Work | Size |
|---|---|
| Fleet: `panacea issue show`/`note`, `panacea evidence propose`, the `issue.delegated` event, `task.issueId` | 2 d |
| `Shift+D` Delegate; Orchestrator notes on the issue, one voice per owner | 1.5 d |
| Proposed evidence under each criterion in the Merge lens; *Record* as the person (phase 3) | 1 d |
| `negotiation.row` on both issues; `blocks` across owners seeds a `sequence` proposal (Fleet 1, desktop 1) | 2 d |
| Agent task-card pages, "whose agents" filters, `who.label`, `step.fold` on the issue | 1.5 d |
| Guidance records, the org kill switch, our published agent rules | 1 d |
| The read-only *ask* worker; named risk facts on lanes | 1 d |
| The same verbs for solo mode through phase 3's runner (**UNVERIFIED** that it can host them, 04 §4.5) | 1.5 d |

**W4, roadmap, release and reporting (~22.5 d).**

| Work | Size |
|---|---|
| Initiatives `in-`, updates `nt-` with health and a pinned brief, reminders (Hub 1.5, desktop 2) | 3.5 d |
| Timeline: zoom, drag to reschedule, the cycle band, grouping, feature dependencies | 3 d |
| Milestones `ml-` as named criterion subsets (Hub 0.5, desktop 1) | 1.5 d |
| Release notes grouped by criterion; freeze per environment; *shipped* for requesters (phase 7) | 1.5 d |
| Insights panel and scatter: measured counts and durations only (Q1) (Hub 1, desktop 2) | 3 d |
| Gateway-backed assists: triage suggestions with `serves` and a "why", the calibrated dial, duplicates, natural-language filters, composer hints | 4 d |
| Drafted updates and thread summaries as span-linked indexes; *propose issues per criterion*; *propose the next cycle* | 3 d |
| "Since you last looked" names issues; recurring issues; triage rotation; the user-facing method page | 3 d |

**W5, intake and the ecosystem (~30.5 d).**

| Work | Size |
|---|---|
| Requests `rq-`: Entra web form, email per team, intake rules, updates to the requester | 5 d |
| Tracker sync and import: Azure Boards, GitHub Issues and Jira one way into `triage` (4 d), then two way (3 d); Linear and CSV import (1 d) | 8 d |
| Outside PRs on both hosts: webhooks, magic words, linkbacks, branch-name start | 2.5 d |
| A Teams app: intake, feature feeds, the Desk digest, ask **your own** Orchestrator | 3 d |
| Public API: docs, generated TypeScript and Go clients, HMAC webhooks, app approvals, keys, rate limits | 3 d |
| The Work MCP server with a read-only mode; the connector allow-list | 2 d |
| Loops `lp-`: triggers, versions, run log, run now; the first-pass triage loop | 3.5 d |
| Release CLI, an Azure Pipelines task, a GitHub Action; harness allow-list and team skills; audit to Sentinel | 3.5 d |

