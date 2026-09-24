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
   `comment ic-`, `intake it-`, plus team mode's `task tk-`. All live in the Hub. Live activity
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
| 31 | Email intake per team or template | MATCH | Intake `it-` by email address per team | W5 |
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
| 45 | Attachments | MATCH | Needs a Hub blob route; **UNVERIFIED** whether the Hub contract has one (not checked for this file) | W2 |

### C. Triage (L-46 to L-56)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 46 | Per-team Triage inbox | MATCH | `triage` state and a Triage view on the Work surface. Items that name a person also join that person's Desk (§4.2) | W1 |
| 47 | Number-key triage actions | EXCEED | `1` accept, `2` duplicate, `3` decline, `4` snooze, list focus only; plus `S` **accept and Start**, which binds a lane at once | W1 |
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
| 106 | Search `/`, view search, jump to issue | MATCH | `/` searches the in-memory Hub replica; `G I` then a number jumps (§4.3) | W1 |
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
| 132 | Requests with the source quote, linked | EXCEED | Intake `it-` keeps the requester and the verbatim quote, linked to issues **and to the brief criterion** the ask became (§3.5) | W5 |
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

