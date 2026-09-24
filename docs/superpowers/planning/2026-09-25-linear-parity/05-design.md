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
2. **Parity, counted.** Of Linear's 257 features: **MATCH 155, EXCEED 75, SKIP 27** (§1). Every
   SKIP has a reason. Eleven are support-desk, CRM and sales features; the rest are embeds and
   long-tail connectors, grants Hub v1 lacks, predictions `AGENTIC-UX.md` bans, mobile and web
   apps, and non-build items.
3. **The magic is four things Linear does not have** (§2): agents that are real teammates on the
   issue (they take it through their owner, report in one live line, and *propose* evidence);
   Orchestrators that negotiate across owners and escalate to inform or to ask; **one thread** from
   a brief through a design frame, issue, session, commit and land to the release; and managed
   zero-retention inference with an AU switch.
4. **Done means proven.** An issue closes when every criterion it serves is settled by recorded
   evidence at the landed commit, not when a PR merges. Linear moves an issue to Done on merge.
5. **Object model** (§3): `issue is-` (mutable head, immutable digested revision), `feature ft-`
   (Linear's "project"; our `project` is the product), `cycle cy-`, `initiative in-`, `milestone
   ml-`, `view vw-`, `comment ic-`, `request rq-`, `template tp-`, `label lb-`, `loop lp-`, plus
   team mode's `task tk-`. All live in the Hub. Live activity stays Fleet state, derived onto the
   issue, never stored on it.
6. **One app, not two** (§4). Work is a seventh lens plus a project surface, not a separate
   product; every other lens shows an issue chip. The Desk absorbs Linear's Inbox, My Issues and
   triage-for-me. Lower-case keys edit the record; `Shift` hands work to your agents. The thread is
   the visual signature that says "this is not a Linear clone".
7. **Fast is architecture.** An in-memory Hub replica on the Hub's existing change feed, optimistic
   writes with undo for reversible acts (02 C8), in-row confirmation for irreversible ones. This
   corrects 02 C8, which put the replica on disk against ADR-H1 (§4.4, Q5).
8. **Minimum lovable slice W1** (§5.3): file an issue, `Shift+R` starts a cloud agent on it, it
   lands, a person records evidence, it turns `done` by itself, and the release shows it shipped.
   About **28 focused days** (3 weeks elapsed on three streams). **W1-W3 is about 64 days; all of
   W1-W5 about 117.** None is measured.
9. **Changes now** (§5.2): add `issue:` to the phase 4 key parser and the Hub key grammar; fix the
   stale `criterionId: "done:2"` example. **One gap found:** no plan builds the Desk (§5.6).
10. **Ten owner questions** (§6), five before W1. The biggest: progress displays (Q1, recommend
    convergence plus plain group sizes, nothing that predicts) and how far to build (Q2,
    recommend W1-W3 first).

**How to read this file.** §1 is the full parity matrix and is long; read its tally first. §2 is
the "more". §3 to §5 are the design and the delivery. §6 is what the owner decides; §7 is what is
unverified.

---

## 1. The parity matrix: all 257 items

**Verdicts.** **MATCH**: we do what the item does, to the same bar. **EXCEED**: we do it and the
row says what more. **PARTIAL**: a named part is left *later* or rests on an UNVERIFIED plan (the
row says which). **SKIP**: we do not build it, and the row says why. A SKIP is a decision the
owner can reverse (§6), not an oversight.

**Slices** (defined in §5): **W0** now, inside the current wave; **W1** the first slice a team can
adopt (revision 2: it now carries comments, the editor and Linear import);
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
| 8 | Fixed status categories, custom names | EXCEED | Fixed **fact** states; teams add their own states inside the human bands, before `started` and after `landed` (*In QA*), in W2 (§3.3). `started`, `in_review` and `landed` are set by **facts** (a lane bound, a lane queued, a trailer landed), each written as the person whose act caused it | W1 / W2 |
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
| 41 | Threads, resolve, reactions, emoji | PARTIAL | `ic-` comments, append-only, author class `text·H`/`text·A`; custom emoji later | W1 |
| 42 | Inline comments on highlighted text | EXCEED | A comment may anchor to issue text **or** to a brief section, a criterion, a transcript span or a diff hunk | W1 |
| 43 | AI summaries of long threads | EXCEED | The summary is an **index of links** to the comments it condenses (AX §6.4), made through the ZDR gateway (OD-19) | W4 |
| 44 | Comment drafts persist | MATCH | Hub-held, as L-33 | W2 |
| 45 | Attachments | MATCH | The Hub already stores content-addressed blobs per project (hub-contract:590-591, cited by 04 §4.6); an attachment names a blob digest | W2 |

### C. Triage (L-46 to L-56)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 46 | Per-team Triage inbox | MATCH | `triage` state and a Triage view on the Work surface. Items that name a person also join that person's Desk (§4.2) | W1 |
| 47 | Number-key triage actions | EXCEED | `1` accept, `2` duplicate, `3` decline, `4` snooze, list focus only; plus `Shift+R` **accept and Start**, which binds a lane at once | W1 |
| 48 | Priority required to leave Triage | MATCH | Team policy, checked by the Hub on the state change | W1 |
| 49 | Ordered triage rules | MATCH | Intake rules, first match wins, each with its author on the event | W5 |
| 50 | Triage duty with rotation from incident tools | PARTIAL | A Hub-held rotation; syncing it from PagerDuty and the like is L-218 | W4 |
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
| 67 | Multi-team project, lead, members, dates | MATCH | `ft-` has a lead and a target date (04 §2.5). **Multi-team** needs a cross-team grant, which W4 builds: a feature shared with a second team (§5.4 W4, Q12) | W2 / W4 |
| 68 | Tabs, saved views as tabs | MATCH | Feature tabs: Brief, Issues, Criteria, Lanes, Release; any saved view pins as a tab | W2 |
| 69 | Document-style overview | EXCEED | The overview **is the branch brief**: the same artefact Define wrote, Design cites and Deploy pins (04 §2.5) | W2 |
| 70 | Project status set by hand | EXCEED | A person sets `planned`·`active`·`paused`·`done`·`cancelled`; the facts sit beside it (lanes live, landed, released where) so a stale status is visible | W2 |
| 71 | Project priority and labels | MATCH | Same fields on `ft-` | W2 |
| 72 | Milestones with progress | PARTIAL | A milestone is a **named subset of the brief's criteria** with a date, reached when those criteria are settled at the branch head (04 §2.5). Convergence, not a bar | W4 |
| 73 | Predicted completion with bounds | SKIP | Prediction is banned (AX anti-pattern 2; AX:237). Convergence per criterion replaces it (Q1) | — |
| 74 | Project dependencies on the timeline | PARTIAL | `blocks` between features, drawn on the timeline; a violated order is signal-coloured | W4 |
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
| 83 | Initiatives with status, owner, target | MATCH | `in-` may span teams through W4's cross-team grant (§5.4 W4, Q12) | W4 |
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
| 94 | View scopes and owner | PARTIAL | Shared views in the Hub; personal views are preferences on the desktop | W2 |
| 95 | Save any filtered list | MATCH | "Save as view" in the palette and the list header | W2 |
| 96 | Subscribe to a view | MATCH | Changes arrive in the Desk digest, never a stream (AX §6.8) | W2 |
| 97 | View changes to a chat channel | MATCH | Teams first, Slack next | W5 |
| 98 | Filters in the URL; every screen a link | EXCEED | Every shared link is an **https Hub URL** that opens the app when installed and a read-only page when not (W2); `panacea://` is the internal form, and a link to an issue can name **its exact revision** (`issue:is-…@r`), so "what did they read?" has an answer | W1 / W2 |
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
| 115 | Snooze and reminders | PARTIAL | Defer with a reason is a first-class verb, and deferred items age visibly (AX §7.4) | W2 |
| 116 | Desktop, mobile, chat, email digest | PARTIAL | Desktop and email digest, then Teams; at most one digest per 15 minutes (AX anti-pattern 8). Mobile is L-232 | W2 / W5 |
| 117 | Automatic and manual subscriptions | MATCH | Subscribed on create, assign, mention or comment; unsubscribe per issue | W2 |
| 118 | Pulse feed | PARTIAL | **"Since you last looked"**: a read cursor per person per scope, four delta classes, a product-language handover whose every claim links to its span (AX §6) | W4 |
| 119 | Pulse digest, audio playback | PARTIAL | The same handover as a daily or weekly digest; audio later | W4 |

### I. Documents and editor (L-120 to L-130)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 120 | Documents on projects, teams, issues | PARTIAL | For a product or a feature the document **is the brief**, on the thread. Free documents `dc-` for teams and initiatives come later | exists / later |
| 121 | Real-time multiplayer editing | MATCH | Team mode's TM2, "one document" (TM-05 §12.2); the phase 5 canvas is multiplayer (OD-11) | TM |
| 122 | Document templates | MATCH | Brief templates per team | W4 |
| 123 | Version history with revert | EXCEED | Brief revisions are already immutable, digested and keyed (`brief:bf-…@n`); an agent's context names the revision it was given | exists |
| 124 | Per-passage human or agent authorship | EXCEED | Every text field carries its author class, `text·H` or `text·A` (TM-05 §3.3), and agent text is fenced from other users' workers (TM-05 §7) | W1 |
| 125 | Link to a heading | EXCEED | A brief section is a **provenance key** that design frames and agent tasks cite (the thread, §2.3) | exists |
| 126 | Markdown shortcuts, slash menu | MATCH | One editor for issues, comments and briefs | W1 |
| 127 | Mermaid, tables, collapsibles, callouts | PARTIAL | Tables and callouts in W2; Mermaid later | W1 / later |
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
| 138 | Asks by email | PARTIAL | As L-31 | W5 |
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
| 156 | `@assistant` in comments and chat | PARTIAL | `@me:orchestrator` in a comment routes to **your own** Orchestrator only (OD-16) | W3 |
| 157 | Acts only within the invoker's permissions | MATCH | Every agent runs as its user, on a gateway token attributed to that user (OD-19; TM-05 §2.4-2.5) | exists |
| 158 | Guidance at two levels; admin kill switch | MATCH | Org, team and personal guidance; an org switch disables Orchestrators; *pause all* on the horizon bar (AX §5) | W3 |
| 159 | Saved Skills, personal or team | MATCH | Claude Code's own skills, shipped in the sandbox image or a team library; the harness is unmodified | W5 |
| 160 | Loops: event or schedule triggers | EXCEED | Scheduled and event turns of **your** Orchestrator (TM-05 §4.2 is already event-driven), bounded by your autonomy grid (AX §7) and billed to you | W5 |
| 161 | Loop permissions, history, versions, run now | MATCH | Each loop is a Hub record with versions and a run log | W5 |
| 162 | Grounded codebase Q&A | MATCH | An **ask** worker: read-only session on the repo; answers link files and lines | W3 |
| 163 | Coding sessions in a managed sandbox, draft PRs | EXCEED | Long-lived, steerable, **many in parallel** per person, one clone each (OD-2), Kata microVMs, Azure DevOps **and** GitHub (OD-4), landed through a reviewed queue (phase 3) | exists / TM |
| 164 | Per-repo environment, secrets, prep scripts | PARTIAL | Phase 8's sandbox image plus a per-project environment spec. **UNVERIFIED** that phase 8's plan has prep scripts | phase 8 |
| 165 | Per-repo agent and model choice | MATCH | Harness per session; model per project through the gateway (OD-19) | phase 6 |
| 166 | Verification artefacts, browser testing | PARTIAL | Artefacts become **proposed evidence on a criterion**, recorded by a person (04 §4.6). Browser testing in the sandbox is later and **UNVERIFIED** on Kata | W3 / later |
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
| 204 | GitHub Enterprise Cloud and Server | PARTIAL | Phase 3's host interface (OD-4); **UNVERIFIED** that Server is in its plan | phase 3 |
| 205 | GitLab | SKIP | OD-4 names Azure DevOps and GitHub. GitLab fits the same host interface later | — |
| 206 | Slack | MATCH | After Teams; KPMG runs on Microsoft | later |
| 207 | Microsoft Teams | EXCEED | First-class: intake, feature feeds, the Desk digest, and asking **your own** Orchestrator from Teams (OD-16) | W5 |
| 208 | Figma frame previews and plugin | PARTIAL | Our own canvas (OD-11): frames are native, cite brief sections, and agents edit them over MCP. Figma link previews later | phase 5 |
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
| 220 | Importers | PARTIAL | Linear (through its public API) and CSV into `triage` in W1; Jira, Azure Boards and GitHub Issues in W5; others later | W1 / W5 |

### O. Keyboard, speed, apps (L-221 to L-235)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 221 | Contextual command menu `Ctrl+K` | MATCH | The palette is built (phase 2); W1 makes it open beside a row for that row's actions. Cockpit chords win over a focused terminal, with OD-9's toggle | exists / W1 |
| 222 | Command-menu prefixes | MATCH | Prefixes scope to issues, people, sessions or commands (§4.3) | W1 |
| 223 | Key grammar: property letters, `G`, `O`, `M` | EXCEED | The same grammar, live **only when focus is in a list**, never in a terminal (02; extends `isCockpitChord` in `useKeyboardMap.ts`); it also reaches agents: `Shift+R` Start, `Shift+O` Delegate (§4.3) | W1 |
| 224 | `?` shortcut sheet | MATCH | Generated from the same key map, so it cannot drift | W1 |
| 225 | Multi-select and bulk bar | EXCEED | Bulk acts include *Start 4 lanes* and *Delegate 4 to my Orchestrator*; irreversible bulk acts confirm in place (02 `confirm.inplace`) | W1 |
| 226 | Menus show every shortcut | MATCH | Every menu item renders its chord from the key map | W1 |
| 227 | Safe-triangle submenus | MATCH | 02 S7's pattern | W1 |
| 228 | Local-first client database | MATCH | An in-memory **Hub replica** with optimistic reversible writes (02 C8). In memory only, because no client persists Hub data (ADR-H1:467-474) | W1 |
| 229 | Sync log with checkpoint delta sync | MATCH | The Hub's `/v1/changes` is already a gapless per-org feed (hub-contract:1089-1107): reconnect with the last sequence seen, receive only later changes | exists / W1 |
| 230 | Offline queue that survives restart | SKIP | Surviving a restart means writing Hub data to disk, which ADR-H1 forbids, and the product is cloud-first (OD-20). The in-memory queue survives a network drop, not a restart | — |
| 231 | Desktop apps with tabs | PARTIAL | Tauri desktop on Windows now; the same code builds for macOS **and Linux**, which Linear lacks (01 §23.6). **UNVERIFIED**: neither build has been made. Tabs later | exists / later |
| 232 | Mobile apps | SKIP | A separate product. The phone's job, the 10-second check (AX §5), is served first by the Desk digest and answer-in-Teams (L-207). Revisit (Q6) | — |
| 233 | Web app and PWA | SKIP | The desktop is the portal (OD-20). The Hub serves only intake forms and read-only issue pages for people without the app | — |
| 234 | Light, dark, custom themes, gallery | EXCEED | 02 C1: themes **generated in LCH from three seeds**, four shipped (graphite, paper, and a high-contrast pair). No gallery | W1 / W2 |
| 235 | Personal preferences | PARTIAL | Font size, send key, auto-assign. **A deliberate difference:** the home view is always the Desk (AX §5, "the default landing surface, always") | W2 |

### P. Enterprise and admin (L-236 to L-247)

| L | Linear item (short) | Verdict | How, or why not | Slice |
|---|---|---|---|---|
| 236 | Roles, including guest | PARTIAL | Org admin; team `admin`·`member`·`viewer` (hub-contract:324-333); guest later | exists |
| 237 | SSO with domain enforcement and JIT | PARTIAL | Entra, auth code with PKCE (OD-6); the org **is** the Entra tenant, so JIT is inherent. SAML for other IdPs later | exists |
| 238 | SCIM, IdP group to team | MATCH | Entra group → team mapping. **UNVERIFIED** whether the Hub plan has it | later |
| 239 | Audit log, API, SIEM streaming | PARTIAL | Every Hub write appends its event in the same transaction (hub-contract:1133-1134); each event says **person or agent, and whose agent** (03's `actor_is_agent` idea); fact moves name the person whose act caused them. Sentinel streaming later | exists / W5 |
| 240 | Data regions US or EU | PARTIAL | We place our own environment (OD-5), and inference has an **AU path** (Bedrock Sydney, OD-19). **UNVERIFIED**: phase 8 has not fixed the hosting region | phase 8 |
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

Counted by script from the rows above (257 rows, numbering checked gapless), revision 2,
2026-09-25. **PARTIAL** (new in revision 2, P C3) means a named part of the Linear item is left
*later* (unscheduled) or rests on an UNVERIFIED plan, so even W5 does not fully deliver it:

| Verdict | Items | Share |
|---|---|---|
| **MATCH** | 139 | 54% |
| **EXCEED** | 67 | 26% |
| **PARTIAL** | 24 | 9% |
| **SKIP** | 27 | 11% |

The 24 PARTIAL rows: L-41, 50, 72, 74, 94, 115, 116, 118, 119, 120, 127, 138, 156, 166, 208, 220,
231, 235, 236, 237 and 239 have a deferred part; L-164, 204 and 240 rest on an UNVERIFIED plan.
The critique named 28; four are no longer partial because revision 2 schedules the missing part:
custom states (L-8, W2), https links (L-98, W2), and cross-team features and initiatives (L-67,
L-83, W4).

**Parity at each stopping point** (P C3). *Reached* means the row's first slice is built;
*complete* means every slice it names is built and it is not PARTIAL. The 22 *exists* rows count
from W1, although their phases are not finished; the 3 team-mode rows count from W3, with TM1.

| Stop after | Reached | Complete | Of 257 |
|---|---|---|---|
| W1 | 69 | 49 | 19% complete |
| W2 | 117 | 95 | 37% |
| W3 (recommended first target) | 136 | 115 | 45% |
| W4 | 173 | 151 | 59% |
| W5, **"everything they are doing"** | 208 | 188 | 73%; plus 12 *later*, 10 through phases 3, 5, 6 and 8, and the 27 skips |

**When each lands** (first slice named in the row): W1 47, W2 48, W3 16, W4 37, W5 35; **22 exist**
in plans already in flight; 13 arrive through other phases (team mode 3, phases 3, 5, 6 and 8: 10);
12 are *later*; 27 are skipped.

**The 27 skips, by reason.** Each is reversible by the owner (Q6).

| Reason | Items | Count |
|---|---|---|
| A support desk, CRM or sales feature; our users build apps, they do not run help desks | L-131, 134-136, 141, 142, 210-214 | 11 |
| Third-party embeds and long-tail connectors defeat egress control; the API covers them | L-128, 216, 217, 219 | 4 |
| Needs per-object grants or nested teams, which Hub v1 does not have. W4's cross-team grant (Q12) covers the planning need; these three stay out | L-4, 6, 86 | 3 |
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

- **Solo: Start.** One chord (`Shift+R`) on EP-142 picks the feature's branch or cuts `ep-142-<slug>`,
  creates a lane (a local clone, OD-2), and starts a managed cloud session whose record carries
  `IssueRef = issue:is-…@r` beside the brief's `ContextRef` (04 §4.2). The issue moves to
  `started`, written as that person, with the lane as the cause.
- **Team: Delegate.** `Shift+O` sends EP-142 to **your own** Orchestrator. It reads the issue with
  `panacea issue show`, plans tasks with `--issue`, and spawns workers whose sessions carry the
  issue (04 §4.3). Nobody can delegate to someone else's agents (OD-16).

What the teammate then does, visibly, on the issue:

```
EP-142  Uploads survive a dropped connection            Dylan · started 09:12
  serves  brief:bf-3a…@6 › done › c-91e2… "a 40 MB upload resumes after a drop"
  ● dylan · agent 1   writing the resume handler in upload.go                      live
    files  services/api/upload.go  store/chunks.go  store/chunks_test.go  api/routes.go
  ▸ observed evidence  c-91e2… at 7c1f0a2 · resume test, exit 0, run by the platform 14:01
  ▸ proposed evidence  c-40aa… at 7c1f0a2 · "reviewer sees the age column"        [Record]
  ◦ note (Dylan's agent)  assumed chunk size 8 MB; the brief does not say
```

1. **It acknowledges fast.** First visible activity within 10 seconds (`budget.ack`, 02 C8; the
   rule Linear publishes, 01 L-190).
2. **It reports in one live line**, on the one heartbeat clock (DESIGN-SYSTEM; 02 `row.three`).
   The line is assembled from observed facts, never a model's summary of a transcript (TM-05 §9.1).
3. **It cannot claim progress.** `started`, `in_review` and `landed` come from facts (a lane bound,
   a lane queued, a trailer landed). Agent text is labelled `text·A` wherever it appears
   (04 §4.5).
4. **It proposes evidence; a fact or a person records it.** Where the platform watched the proof
   (a test run and its exit code at the landed commit, AX §8.1), that observation **is** the
   evidence and needs no click. Otherwise `panacea evidence propose` puts a typed proposal under its
   criterion and **Record** writes it as the person (04 §4.6). The issue is `done` when its **own**
   work landed and its **own** evidence holds; the criterion settles separately (§3.3).
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
4. **Issue relations feed the protocol, through the editor's own Orchestrator** (§3.7 rule 3).
   When Alice adds `EP-150 blocks EP-142` across two owners, **her** Orchestrator opens a `sequence`
   proposal with Dylan's; the edit itself pauses nothing. A `duplicates` link between two people's
   issues opens a thread asking whose lane survives, which is always a *decide*.
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
   it and its convergence: `settled`, `open`, `contested`, `drifting` or `uncovered` (§3.3). A
   criterion no issue serves is a visible gap, named *uncovered*.
4. **See where every issue stands on the thread** (P I3). Every issue carries one **thread state**,
   shown on its row and in peek: *on the thread* (it serves a criterion), *project-level* (it serves
   the project brief's standing criteria, such as "no regression" or "no new critical finding"), or
   *not tied*, with a reason picked at triage (`chore`, `spike`, `imported`, `support`). Attaching a
   criterion is one key in triage (`T`), with a suggestion from text match in W1 and from the
   gateway in W4. An open end is allowed; a silent one is not.
5. **Erase customer text without rewriting git.** Trailers carry ids, never titles (04 §5.2), and
   an issue has the brief's erase (04 §5.5).

**Why Linear cannot follow quickly.** Linear Releases links issues to a commit SHA by scanning
for issue keys (01 L-146; 03 §12.2). Nobody has the front half: a brief section cited by a design
frame, carried as each agent task's reason, and attached back to the release. 03 estimates Linear
could assemble the back half in 2 to 3 quarters; the agent-edited design canvas (OD-11) is the
part it cannot copy quickly. **Unproven:** the thread only helps if people write briefs; the
low-ceremony entry (AX §9.7) must let a one-line fix skip Define without breaking the thread (an
issue with no `serves` is allowed, and carries the thread state *project-level* or *not tied*, point
4). **Sync does not cut the thread:** imports and mirrors arrive in `triage` as *not tied ·
imported*, and the other tracker can never set state or `done` (§5.4 W5, Q9).

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
| `done` | **a fact** at landing, or a person with a reason | the issue's **own** evidence holds (below) |
| `cancelled`, `duplicate` | a person | closed without work |

Teams may add **custom states inside the human bands** only: before `started` (for example *Ready
for design*) and after `landed` (for example *In QA*, which holds `done` until a person clears it).
The five fact states are fixed and cannot be renamed, reordered or removed, because facts set them.

**The rule, revision 2 (P C4).** Two facts, kept apart:

- **An issue is `done`** when its own work has landed **and** its own evidence is recorded: for
  each criterion it serves, an evidence record that **cites this issue** and is at or after the
  issue's land commit. Evidence recorded before the issue landed never closes it, and a sibling's
  evidence never closes it. **Evidence is an observed fact where one exists** (AX §8.1): a test run
  the platform watched, its exit code at the landed commit, closes the issue with no click. A person
  presses **Record** only where a person must attest.
- **A criterion is `settled`** when all of its evidence holds at the head (04 §5.3). That is the
  criterion's state, not the issue's. The issue shows both: *"done · c-91e2 still open (EP-150 not
  landed)"*.
- **An issue with an empty `serves`** turns `done` at `landed`, as Linear closes on merge. A team
  switch makes it stop at `landed` instead.
- **A hand-set `done` on an issue that serves criteria** needs a one-line reason and shows as
  **done, not proven** wherever the issue appears, in its own glyph (§4.4).
- **A self-attested Record** (the same person filed the issue, started the agent and pressed Record,
  which is every solo-mode Record) shows as *proven, self-attested*. Observed evidence does not.
- **Sync never writes a fact state or `done`** (§5.4 W5). Only phase 3's evaluator and a person do.
- **A criterion whose text was edited** gets a new id (04:263), so issues serving the old id show
  **drifting** and a one-key *re-point* on the Desk (an *inform* to the issue's assignee). They close
  on evidence against the new id once re-pointed; they never wait silently.

*Shipped* is never a state; it is derived from the release manifest (04 §5.4).

These nine are **issue** states. `AGENTIC-UX.md` anti-pattern 4's closed set of seven is the
**agent** status set; the two never share a column, an icon family or a colour (§4.4). A
**criterion** has five convergence words, kept from 04 §3.3: `settled`, `open`, `contested`,
`drifting` (its text changed under its issues) and `uncovered` (no issue serves it).

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
  `done` by hand, and on an issue that serves criteria that move needs a reason and is shown as
  *done, not proven* (§3.3); no sync or integration token may write a fact state or `done`; an
  agent writes only comments on its own owner's behalf, and in team mode only
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
  `df-` and `dc-` (free documents, L-120, *later*) collide with none. Milestones take `ml-`
  because `ms-` is TM-05's.

### 3.7 When a person edits someone else's in-flight issue (P I8, OD-16)

Any team member may write any issue (§3.6). When the issue has **another owner's agents on it**,
three rules keep OD-16's "nobody steers another user's agent" true:

1. **An edit reaches the owner, never the agents.** Alice edits the description or `serves` of
   Dylan's EP-142. Dylan's agents keep the revision they were given (`@3`, §3.4). Dylan gets a Desk
   *inform*: *"EP-142 changed since your agent read @3 (Alice, 11:02): serves c-40aa added."* with
   one verb, *re-brief my agents at @4*. Nothing reaches the agents until Dylan presses it.
2. **Reassigning an issue with agents on it asks the current owner first.** Alice's reassign
   becomes a Desk *decide* for Dylan: *hand over the worktree to Alice* (it and its branch move to
   her; his agents stop at their next checkpoint) or *keep working, decline*. The safe default
   pauses nothing and moves nothing. Until he answers, EP-142 stays Dylan's and shows *reassign
   requested by Alice*. In team mode his Orchestrator winds the agents down; in solo mode his
   session gets a checkpoint and stops.
3. **A cross-owner relation is a proposal from the editor's own Orchestrator.** Alice adding
   *EP-150 blocks EP-142* does not pause Dylan's agents. It asks **Alice's** Orchestrator to open a
   `sequence` proposal with Dylan's, which then negotiates under OD-16 (§2.2 point 4, now reworded).
   In solo mode, with no Orchestrators, the relation is recorded and Dylan gets an *inform*.

Rules 1 and 2 need only the Desk and land in W1; rule 3 needs Orchestrators and lands in W3.

---

## 4. The experience: one app, not a tracker bolted on

### 4.1 Where the work layer appears

**The words a person sees (P I5).** Revision 1 put 18 nouns in front of a user. The screen now
uses **nine**: *project, feature, brief, criterion, issue, worktree, agent, Orchestrator,
evidence*, plus *cycle* and *initiative* once W2 and W4 bring them. The rest are internal:

| Internal word (this file, the Hub, Fleet) | What the screen says |
|---|---|
| lane (phase 3's record; `refs/panacea/lanes`) | **worktree**, the word the built rail already uses |
| task `tk-`, worker | nothing: *your agents*, each named by `who.label` (`dylan · agent 2`) |
| session | nothing on the Work surfaces; *open session* is a link |
| branch | never alone: **a feature and its branch are one row**, the branch in mono under the name |
| sub-issue, milestone, request | shown only once a team creates one |

C13's not-a-clone review gains one check: *a new user can say what each visible noun means after
five minutes*.

The rail stays navigation only: project → feature → worktree → session, names and no counts
(AX:144). The work layer adds **one project surface, Plan, and one lens, Work**, and chips
everywhere else. Two surfaces named *Work* in revision 1 are now two names.

| Surface | Scope | What it holds | New? |
|---|---|---|---|
| **The Desk** | the account, above projects | issues only when they need you, in the Desk's existing sections (§4.2) | Desk designed, **not built** (§5.6) |
| **Plan** (project surface, `G P`) | one project | Triage, Backlog, all criteria with their issues, the features board, cycles, initiatives, saved views, *My issues* | new |
| **Work lens** | one feature (its branch) | its issues, **the brief's criteria with the issues that serve each**, who is on each issue now | new; `Lens` gains `"work"`, the seventh lens |
| **Issue view** | one issue | head, revision history, `serves`, relations, comments, the live agent line, proposed evidence, *shipped to* | new |
| **Discover, Design, Develop, Merge, Deploy** | a branch | an issue chip beside the brief chip on every artefact that carries one (§4.5) | chips new |
| **Palette** `Ctrl+K` | everywhere | every work verb, with its chord shown (L-226) | built; gains commands |

**The Work lens, revision 2: grouped by criterion (D C1).** Revision 1 drew Linear's default list
(grouped by workflow state) in teal. The skeleton is now the thread, which Linear has no object
to build. Drawn as W1 ships it: no cycles (W2), no negotiation (W3), plain group sizes (Q1 A).

```
┌ EP · Resumable uploads  feat/resumable-uploads ─ Work           Global ▾  by criterion ▾ ⌘K ┐
│ Uploads survive a dropped connection                            open · 3 issues · 1 moving │
│ ┃ ◇ EP-142  Resume handler for cut uploads            Dylan                              │
│ ┃   ● dylan · agent 1   writing the resume handler in upload.go                    live │
│ ┃ ▷ EP-150  Chunked upload API                        Alice · touching upload/*.go      │
│ ┃ ✓ EP-139  Retry budget per chunk                    Priya                        held │
│ ┃   → done · own evidence at 7c1f0a2 (test run, exit 0) · 14:02                         │
│ A reviewer sees how old each piece of evidence is               settled · 1 issue        │
│ ┃ ✓ EP-131  Evidence age column                       Priya                              │
│ Serves no criterion                                                         2 issues     │
│   ○ EP-155  Bump the chunk-store client               Dylan   not tied · chore           │
│   EP-128 left this view · cancelled                                        N applies     │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

- **The group header is the criterion, in words, in sans**, with its convergence word (§3.3). Its
  issues hang from `thread.line` (┃) in the leading gutter. *Serves no criterion* is always the last
  group, which is itself a fact worth seeing. The machine id `c-91e2` lives in peek, in mono.
- **The workflow state is a glyph column inside the row**, not the grouping. *Current cycle, by
  state* survives as a saved view (W2) for people who want Linear's shape; it is not the default,
  and the prototype does not show it first. C13 gains a structural check (§4.4).
- **One agent line per row, yours only (D C3, D I2).** Your issue carries one line: the lead
  agent's MEANING in sans, prefixed by `who.label` in mono (`dylan · agent 1`), and one dot for that
  agent alone, so the dot has one span. Other agents on the issue are one `Space` away, never
  counted. **A teammate's issue carries no dot and no count**: the person, and one sans line of
  what they touch (`Alice · touching upload/*.go`). Their fleet is in `peek.person`. The heartbeat
  carries only what the reader can direct.
- **Liveness text coarsens (D I10).** `live` under 10 s; then `quiet 2m`, `quiet 14m`, changing
  only on the beat. The quiet rows are the news, so only they change.
- **Rows never move under the reader (D C2).** Beside `arrivals.hold` (new rows wait behind one
  line) sits **`moves.hold`**: a row whose group or state changes stays in place, takes its new
  glyph, flashes over `--dur-flash` and shows one mono cause line (EP-139 above). It relocates only
  on `N` (apply held changes), on leaving the view, or when focus is outside its group. A
  **departure** leaves a one-line ghost until `N`. The group header shows what is held
  (`· 1 moving`). More than three changes in one group at once flash the header once instead of
  each row (D M9).
- **Lit is not selected (D I1).** Hovering **or focusing** a criterion (`thread.lit`) brightens its
  `thread.node` and `thread.line` in the gutter, across Work, Design and Develop; it never rings a
  row. Selection keeps the ring and gains a leading check glyph, so it is a shape too. The prototype
  shows three rows selected and a fourth lit, together.
- **Negotiation (W3)** shows as a `⇄` line under the issue only until you have seen it (the read
  cursor), then folds to a `⇄` glyph on the row (D M3). A queue position names its cause in mono
  (`review · behind EP-142`), never a bare ordinal (D M2).
- The glyphs above are placeholders until W1 row 9's drawn set (§4.4, §5.3). Each state is a
  distinct shape; none fills as work progresses (02 C13).

### 4.2 The Desk absorbs Inbox, My Issues and triage-for-me

No new Desk section (TM-05 §9.3). Issue rows join the fixed ones (AX §5):

| Desk section | Issue rows that join it |
|---|---|
| NEEDS YOU | assigned to you in `triage`; an urgent issue assigned to you; `in_review` where you are the reviewer; a comment naming you with a question; **your issue is blocking someone's agent** (with the cost of the wait); a `decide` escalation about an issue |
| DECIDED WITHOUT YOU | an Orchestrator agreement within your policy that changed your issue (sequenced, split), with Undo |
| FAILED | a lane bound to your issue failed; an intake rule could not route a request |
| LANDED | your issue landed; it turned `done`; a criterion it serves became settled; your issue **shipped** |
| RUNNING | your agents on your issues: see the shape below |
| (informs) | someone edited or asked to reassign your in-flight issue (§3.7); a criterion your issue serves drifted (§3.3) |

**RUNNING's shape, revision 2 (D C3.3, D I10.2).** AX §5 says a card is *"three lines, always
three"*; revision 1 said one line. Now: an agent that **changed since you last looked** (new
state, a question, quiet over 10 minutes) gets AX's three-line card; the steady ones fold into
one line that expands in place: *"23 steady, all live"*. The folded line's number is a group size
under Q1 A. The Desk is never live while you are on it (AX §5's fixed order); it re-ranks on
arrival. This changes a binding rule, so it is one of Q13's amendments.

**Budgets are CSS px at 125% scaling** (D I10.2): a 1080p KPMG laptop at 125% is 864 px tall, at
150% 720. Revision 1's "30 rows at 960px fits one screen" was wrong; the fold is what makes 30
agents fit.

Every row answers in place and never navigates (AX §5): accept, defer with a reason, record
proposed evidence, answer a question in one line. The zero state stays reachable and says only
what W1 can back: *"Nothing needs you. EP-142 landed and proven at 14:02. 3 agents live on 2
issues."* The horizon line (*"Next decision expected ~14:22 · rule: migration gate"*) joins it
when the horizon bar exists, and every projection names its rule and source span (D M8).

### 4.3 The keyboard and command model

**The policy, revision 2 (P I2, D I3).** Key letters are function, not expression, and muscle
memory is what a migrating team brings. So **we match Linear's unshifted letters and its `G`, `O`
and `M` sequences on purpose**, wherever 01 §15.1 records them, and **depart only where an agent or
a cost is involved**. Revision 1 said Linear's letters were "not copied"; that was wrong in fact,
and the table below now says which keys match. 01's list is partly **UNVERIFIED** (from a search
summary; 02 D4 got a 404 on the shortcut page), so the table is checked against Linear's public
shortcut page once it is reachable, before W1's plan.

Four rules come first:

1. **Single keys and sequences fire only when focus is in a list** (02 C9 `keys.list`), never in a
   terminal and never in a text field. This extends `isCockpitChord` in
   `desktop/src/hooks/useKeyboardMap.ts`. `G`, `O` and `M` sequences obey the same rule (revision
   1 let them fire "outside a terminal", which reached text fields). `Ctrl+K` works everywhere, as
   built, with OD-9's toggle.
2. **Lower case edits the record; `Shift` plus a letter hands work to your agents.** A reader can
   tell from the key alone whether an agent will run. Range select moves to `Shift+↓`/`Shift+↑`
   (the platform convention, and Linear's, 01 §15.1), so no `Shift`+letter is a record act.
3. **No agent key sits on a chord Linear uses.** Linear binds `Shift+D` (due date), `Shift+E`,
   `Shift+P`, `Shift+M` and `Shift+S` (subscribe, 01:381, :394). Here **those five chords do
   nothing**, so a Linear user's muscle memory never starts an agent or a bill; the `?` sheet lists
   our equivalent for each. Agent keys are `Shift+R` **Run** (was `Shift+S`), `Shift+O` delegate to
   my **Orchestrator** (was `Shift+D`) and `Shift+A` **ask** my Orchestrator (was `M`), chords 01
   records no Linear binding for (**UNVERIFIED** as a complete list).
4. **Agent keys match the physical key, not the character.** The handler tests
   `event.code === "KeyR" && event.shiftKey`, never `event.key === "R"`, so Caps Lock cannot turn a
   record act into a run. **Settings → Keyboard → Single-key shortcuts** turns every single-key and
   sequence binding off (WCAG 2.1.4, for speech-input users); the palette and chords remain.

Key letters are sliced: `Shift+O` and `Shift+A` do nothing until W3 (no Orchestrator before it),
and the property keys for W2's records (`Y` cycle, `B` feature, `D` due date) arrive with them.

| Key (list focus) | Meaning | Where |
|---|---|---|
| `J` `K`, `↑` `↓` · `Enter` · `Esc` | move · open or dive · back | every list |
| `Space` tap / hold | peek: `peek.span`, `peek.hunk`, `peek.person` (02 C9) | every list |
| `X`; `Shift+↓` `Shift+↑` | select; extend the selection. The bulk bar appears. **Matches** | every list |
| `/` · `?` · `U` · `N` | search · shortcut sheet · undo · **apply held changes** (arrivals, moves and departures, §4.1) | every list |
| `H` | defer with a reason (Linear's snooze key, **matches**) | every row |
| `A` · `D` · `R` | approve · defer · rewind an assumed decision (AX §5, unchanged) | **decision** rows on the Desk only |
| `C` | new issue; on a Desk row or a span, the new issue links to it. **Matches** | every list |
| `A` · `I` | assignee picker · assign to me. **Match**; answers D I3.4 | issue rows |
| `S` `P` `L` `E` · `F` | state · priority · label · estimate · filter. **Match** (Linear's `Shift+E` is unshifted here, rule 3) | issue rows |
| `Y` `B` `D` | cycle · feature (a feature **is** a branch) · due date | issue rows, from W2 |
| `T` | attach a criterion (thread state, §2.3 point 4) | issue rows; Triage |
| `M` then `B` `X` `R` `D` | relation: blocked by · blocking · related · duplicate of. **Matches** | issue rows |
| `1` `2` `3` `4` | accept · duplicate · decline · snooze. **Matches** | Triage view only |
| `Shift+R` | **Start** (`R` for run): a lane and a managed session on this issue | issue rows |
| `Shift+O` | delegate to my **Orchestrator** (team projects) | issue rows, from W3 |
| `Shift+A` | **ask** my Orchestrator about this row (TM-05 §9.2's one action) | every row, from W3 |
| `G` then `I` `M` `T` `V` `B` `W` `P` `O` | go to Desk (Linear's Inbox) · my issues · Triage · current cycle · backlog · Work · Plan · my Orchestrator. **`I` `M` `T` `V` `B` match** | list focus |
| `O` then `I` `P` `C` `F` | open an issue by number · a project · a cycle · a favourite. **Match** | list focus |

A decision row and an issue row differ in shape (the decision row carries its options line), so
`A` and `D` mean one thing per row type: on a decision, AX §5's approve and defer; on an issue,
Linear's assignee and due date. `H` defers on both.

**The palette** gains every verb above, each row showing its chord (DESIGN-SYSTEM: hotkey hints
inline). Prefixes scope it: `#` issues, `@` people, `>` commands, `~` sessions. It opens beside
the row it acts on when invoked from a row (L-221).

### 4.4 The design-language uplift, applied to work

02's Part C is adopted, with the revision 2 changes the two critiques required, six work-specific
rules and one correction. **Revision 2 moves the visible uplift into W1** (P I6): the owner judges
the new look at the prototype review, not the old one.

| From 02 | Applied to the work layer |
|---|---|
| C1 `theme.generate` | Four generated themes; the Work surface is tested in all four (C12 `stress.sets`) |
| C2 `thread` | **The signature.** `thread.node` on every issue that serves a criterion; `thread.trace` grows from criterion to issue to lane on drill-in |
| C3 `voice.split` | Issue titles and descriptions are sans; ids, keys, paths, costs and durations are mono. **Facts at 12px** (a new `--t-fact`, the `--t-meta` size), never 11px: 11px mono is for `kbd` hints only. Any fact is `--ink-2` or stronger (7.63:1); `--ink-3` (3.43:1) never carries one (D I7) |
| C3 faces | **Revision 2 (D I6):** Geist is now a generator default, so it is no longer the safe pick. `flag.compare` puts **Segoe UI Variable** (recommended: native on Windows, optically sized, nobody's default, already the built fallback) beside an OFL face with its own character (Atkinson Hyperlegible Next, **UNVERIFIED** licence and weights) and Geist; the owner's eyes choose (Q14). The mono is Q10 |
| C4 density | Issue rows at `--row-h` 32px; `compact` 28px; group headers carry plain sizes only (Q1) |
| C5 `chrome.dim`, one `ViewHeader` | Work, the Work lens and the issue view use the one header, with the route badge `Global`/`AU` |
| C6 icons | **Issue states get our own drawn glyphs**, one distinct shape per state, in ink; none fills with progress. Drawn **in W1, before the prototype** (§5.3 row 9): about 12, including *done, not proven*, the priority notch, the selection check and the needs-you mark |
| C7 motion | A state change flashes over `--dur-flash`; `arrivals.hold` for new issues, **`moves.hold`** for changed ones (§4.1); FLIP only on explicit re-sort |
| C7 reduced motion | **Revision 2 (D C4):** with the breath stopped, `--run` against `--idle` is about 2.3:1 (computed from the hex values, **UNVERIFIED** on a display), so running and idle differed by colour alone. **Running gets its own shape**, a disc inside a 1px open ring, so shape alone separates running, idle, exited and failed, as DESIGN-SYSTEM requires. The flash is replaced by a **static 2px leading tick** in `--ink` that stays until the row is seen (the read cursor, AX §6.1) |
| C10 agent activity | One agent line per row: `who.label` in mono, then MEANING in sans (AX Principle 3), one dot for that one agent (§4.1). The Desk uses three-line cards for changed agents (§4.2) |
| C11 first run | `first.landing`'s rehearsal task **is a real issue**, EP-1, started by the user's first `Shift+R` |
| C12 `stress.sets` | Gains three screens (D C4, D I8): reduced motion × deuteranopia simulation; a **Windows contrast theme** (`forced-colors: active`); 125% and 150% scaling. Built in W1 (§5.3 row 8), with `flag.compare` |
| C13 not-a-clone test | Applies to **every work screen, in every theme** (D M5: grain is dark-only, so `paper` carries its own fifth mark, the thread gutter at full strength). A filling status icon or an indigo accent fails the review. **Structural checks, new (D C1):** *a work screen whose default grouping is workflow state* fails; *a new user cannot name each visible noun after five minutes* fails (P I5) |

**Work-specific rules.**
1. **Issue states and agent states never share colour.** Issue glyphs are neutral ink. Colour
   appears on an issue only when it carries a status: `--need` when it needs you, `--fail` when a
   bound lane failed (DESIGN-SYSTEM: colour is signal, and status carries a shape as well).
2. **Priority is a shape in ink, never a colour, urgent included** (D I4). `--need` keeps one
   meaning, *an agent or a person is waiting on you*. Priority is drawn as a notch depth on the
   state glyph's leading edge, not as signal bars, so a row carries one glyph, not two. An urgent
   issue that really needs its assignee enters NEEDS YOU, and earns amber there.
3. **No count outside a group header or the Desk** (AX:245, as amended by Q1). Rows name owners
   and list files; they never print `2 agents` or `+2` (P I7, D C3).
4. **Windows contrast themes (D I8).** A `@media (forced-colors: active)` layer turns every ring
   (`--e-*` elevation, `thread.node`, `lease.ring`, the selected row) into an `outline` or `border`
   in system colours (`CanvasText`, `Highlight`); glyphs are SVG in `currentColor`; grain is off. An
   escalated lease gets the needs-you **shape** on its chip, not only amber. The built focus ring is
   already an `outline` (tokens.css:187-191) and stays.
5. **Assistive technology on a live fleet (D I9).** The heartbeat, dots and flashes are
   `aria-hidden`. **One** polite live region per window carries a digest at AX §6.8's rate (at most
   one per 15 minutes) plus direct answers to the user's own acts (*"EP-142 deferred. Undo, U"*).
   Lists are ARIA grids or listboxes with `aria-rowcount`, since they virtualise over 50 rows; the
   shortcut sheet says `Space` means peek.
6. **Grain is measured, not assumed (D M6).** Scroll frame time with and without the fixed grain
   overlay at 150% in WebView2, before W1's prototype review; grain is off in `-hc` themes.

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
| **Develop** | each session pane's header shows its issue chip beside the brief chip | `Shift+R` from an issue lands here, the session already told |
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
| Start or Delegate | reversible until the session's first **file** write | row resolves at once with an **Undo chip**; `budget.ack` ≤ 10 s to the first span, else "no output yet · 10s" |
| land, push, deploy, erase | `confirm.inplace`, never optimistic | the consequence line and a two-step confirm inside the row |

**Undo, revision 2 (P Minor 5, D I3.2, D I9.3).** The `--undo-window` (6 s) **pauses while the row
has focus or hover**. After it lapses, a reversible act stays reversible from the issue's revision
history, so six seconds is the fast path, not the only one. For Start, the chip reads *Undo Start*
until the session's first file write, even past six seconds; pressing it stops the session,
removes the worktree and shows what it cost (the context read so far, at the provider's rate).
After the first file write the chip becomes *Stop and discard*, a `confirm.inplace` act, because
work would be lost.

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
| **W1** first adoptable slice | *"I bring my backlog, my agent takes an issue, and it is done only when it is proven"*, in the new look | ~48.5 d | **Ready when** (P C1): Hub through task 43; phase 4 tasks 06, 09-10, 16-17; phase 3 tasks 05, 07-09; **local mode** (OD-20) for the demo. **In the cloud** it also needs phase 6 stream A and phase 8 stream C (TM1's streams A and C, 8.5 + 14.5 d) and the owner's environment. The user's own prerequisite: a brief with one complete revision (04:255); `first.landing` seeds one |
| **W1b** shipped | *"shipped to production, 14:02"* on the issue | ~1 d | phase 7 task 19 |
| **W2** planning | Cycles, features, sub-issues, views, custom states, https links, generated themes | ~20.5 d | W1 |
| **W3** agents on the work | Delegate, Orchestrator notes, proposed evidence, negotiation on issues, §3.7 rule 3 | ~12.5 d | W1 and team mode's **TM1 (≈95.5 d, TM-05 §12.1)** |
| **W4** roadmap and release | Initiatives, timeline, updates, notes by criterion, insights, AI assists, **cross-team** | ~26.5 d | W2; phase 6's gateway for the AI rows; phase 7 |
| **W5** intake and ecosystem | Requests, tracker sync, Teams, public API, MCP, Loops | ~28.5 d | W2 |
| | **Total** | **~138 d** | plus TM1, phase 6 and phase 8, which are not Phase W's |

**Why W1 grew from 27.5 to 48.5 days.** Revision 1's W1 was a demo (its own Q2 said so), and its
sizes leaned low. Revision 2 moves in what a Linear team needs on day one and what the owner's
"prettier" needs before the prototype review: comments and the editor (+4.5 d, from W2), Linear and
CSV import (+1 d, from W5), the visible uplift and drawn glyphs (+6 d), a real prototype (+1.5 d),
FAILED and RUNNING on the Desk and §3.7's informs (+1.5 d), the accessibility floor (+1.5 d),
thread state, observed evidence and the key fixes (+1.5 d), and honest sizes for the Work surface
(+2 d) and the replica (+2.5 d, §5.3). *Shipped* moves to W1b (−1 d) so W1 never waits on phase 7.
That is +21 d. The other slices net to zero (W1b +1, W2 −4, W3 +1, W4 +4, W5 −2), so the total
moves from ~117 to ~138. **None of it is measured.**

The whole layer is the size of a large phase, and **TM1 is larger than all of W1 to W3**. §5.7
gives the elapsed time to each stopping point, with TM1, phase 6 and phase 8 in it.

### 5.2 W0: now, inside the current wave (04 §7.1)

1. Add `issue:` to phase 4 task 06's key parser (`provenance/key.go`, not yet built) and to the
   Hub's key grammar (hub-contract:114-128). One more case now; a change to a built parser later.
2. Fix the stale example `criterionId: "done:2"` at hub-contract:1065 and :1079 to a `c-` id.

These two are **the one exception** to §5.5's "new Hub tasks after task 43, not edits to tasks in
flight" (P Minor 4). Both are harmless whatever the owner decides about Phase W: item 1 adds a
grammar case nobody mints until W1, and item 2 fixes an example that is wrong today. They go to the
`feat/hub` worktree's owner as a proposed edit, not a direct change, because that contract is
under live edit.
3. Optional, unblocked: 02 D2 step 1, the missing tokens (`--dur-flash`, `--dur-peek`,
   `--undo-window`, `--s-*`, `--row-h*`, `--t-*`, `--icon*`) in `tokens.css`, about 2 hours.

### 5.3 W1: the first slice a team can adopt, solo mode (~48.5 d)

**Revision 1 called W1 "minimum lovable" and, in Q2, "a demo". Both could not be true (P I1).**
Revision 2 reshapes it into a slice a Linear team could move into: their own backlog, comments,
Markdown and the new look, with Run on any row.

**The demo, revision 2.** Import your Linear backlog (or a CSV); it lands in Triage as *not tied ·
imported*. Press `T` on EP-142 and attach the criterion it serves, from a text-match suggestion.
Press `Shift+R`. Within 10 seconds an agent is working on it (in **local mode**, OD-20, until TM1's
streams A and C give W1 its cloud path) and its line is live on the row. Teammates comment. It
lands; the row flashes `landed` in place and does not move. The platform watched the resume test
pass at the landed commit, so EP-142 turns `done` by itself, with no click. **The last frame is
the Desk** (D C5): *"Nothing needs you. EP-142 landed and proven at 14:02."* W1b adds the frame
Linear cannot show: *"EP-142 shipped to production at 14:02."*

| # | Work | Where | Size |
|---|---|---|---|
| 1 | Issue head and revision, `issue-revision` canonical kind, the `issue:` key in `/v1/refs`, `project.key` and numbers | Hub | 3.5 d |
| 2 | Relations, team labels, `triage` and team policies; **thread state** and its reason (§2.3 point 4) | Hub | 2 d |
| 3 | Events for every write; authorisation rows, including *no sync writes `done`* and the reason on a hand-set `done`; issue erase; title search | Hub | 2.5 d |
| 4 | Comments `ic-`: mentions, reactions, anchors, resolve; subscriptions (**from W2**) | Hub | 2 d |
| 5 | Linear (public API) and CSV import into `triage` (**from W5**) | Hub | 1 d |
| 6 | `Session.IssueRef`, the Materialiser's second key, `provenance.Ref` kind `issue` | phase 4 | 2 d |
| 7 | `Panacea-Issue` trailer, land trailers and the `landed` move, coverage data; §3.3's rule: own evidence, **observed evidence from a watched test run**, *self-attested* marking | phase 3 | 2.5 d |
| 8 | **The look first** (P I6): 02 D2 steps 2, 3 and 7, `chrome.dim` and one `ViewHeader`, `voice.split`, `stress.sets` with its new screens and `flag.compare` | desktop | 4 d |
| 9 | **Drawn glyphs**, about 12, at 14 and 16px on both grounds; the icon family's licence check; fonts bundled as files once Q10 and Q14 are answered; grain measured | desktop | 2 d |
| 10 | Prototype of the Work lens (by criterion, `moves.hold`, lit beside selected), Plan, the issue view and the Desk, for owner review at 125% in `graphite` and `paper`, behind `flag.compare` | desktop | 3 d |
| 11 | Work lens and Plan: Triage, list by criterion, board, display options, bulk bar, `moves.hold`, departures. **Resized after row 10's review** | desktop | 5 d |
| 12 | Issue view: head, revisions, `serves` with `thread.node`, relations, the live agent line; **the editor** (Markdown, slash menu, tables), comment UI, attachments (**from W2**) | desktop | 4.5 d |
| 13 | `keys.list` grammar as revised (§4.3), number-key triage, `peek.span`, the `?` sheet, palette commands, the single-key switch | desktop | 3 d |
| 14 | In-memory Hub replica on `/v1/changes` (below), `optimistic.undo`, `confirm.inplace` | desktop | 5 d |
| 15 | Issue chips in Develop and Merge; Start from an issue, with the Undo chip | desktop | 0.5 d |
| 16 | **The Desk frame**: NEEDS YOU, **FAILED** (*Restart from checkpoint*, *Abandon*), LANDED, **RUNNING** (§4.2's fold), §3.7's informs, answer in place, zero state | desktop | 4.5 d |
| 17 | The accessibility floor: the `forced-colors` layer, one live region, grid roles, the Undo pause (§4.4 rules 4 and 5) | desktop | 1.5 d |
| | Hub 11 · phases 3 and 4: 4.5 · desktop 33 | | **~48.5 d** |

**The replica, reconciled with 02 (P I9).** 02 D2 step 6 sizes `hub.replica` plus
`sync.checkpoint` at about two weeks. Holding it in memory (ADR-H1) removes the encrypted disk
store and the erase propagation to disk, which is most of that. It keeps the cursor on
`/v1/changes`, ordering by the feed's sequence, rolling back an optimistic write the Hub refuses,
and loading only what the caller may read. Revision 1's 2.5 d did not cover the last two; 5 d does,
if `/v1/changes` already filters per caller (**UNVERIFIED** in hub-contract).

**Elapsed.** Desktop is the long pole at 33 d, so it runs on **two** desktop streams (look,
glyphs, prototype, Work lens; issue view, keys, replica, Desk, accessibility) beside the Hub and
the phases: **about 4 to 5 weeks after Hub task 43**, to a local-mode demo (§5.7).

**W1b, shipped (~1 d, phase 7 task 19):** issues in the release manifest from trailers; *shipped
to production, 14:02* derived on the issue and on the Desk's LANDED row.

**Out of W1, on purpose:** cycles, features, views, custom states and https links (W2); anything
that needs team mode or the `panacea` verb channel (W3); anything that needs the gateway (W4, so
W1's criterion suggestion is text match); tracker sync (W5). **Still missing for a Linear power
user after W1**, and said so: cycles, sub-issues, estimates and saved views until W2; outside-PR
linking until W5; an assistant in solo mode (Linear's `Ctrl+J`, L-155) only with team mode.

### 5.4 W2 to W5

**W2, planning (~20.5 d).** A team can run its fortnight in it. (Comments and the editor moved to
W1 in revision 2.)

| Work | Size |
|---|---|
| **Custom states** inside the human bands, before `started` and after `landed` (§3.3; P I4) (Hub 0.5, desktop 0.5) | 1 d |
| **https Hub links** for every shared link: open the app when installed, a read-only Hub page when not; `panacea://` stays internal (P I10) (Hub 1, desktop 0.5) | 1.5 d |
| Features `ft-`: branch reservation, *promote to feature*, the features board and tabs (Hub 1.5, desktop 2) | 3.5 d |
| Cycles `cy-`: janitor rollover, cooldowns, iCal feed, the scope log, convergence per criterion (Hub 1.5, desktop 1.5) | 3 d |
| Sub-issues, templates `tp-`, label groups, estimates, due dates, rank, move, drafts `df-` (Hub 2, desktop 1.5) | 3.5 d |
| Views `vw-`: nested filters, links, ad hoc lists, favourites, set as default; *Current cycle, by state* as a saved view (Hub 1, desktop 2.5) | 3.5 d |
| The digest (desktop and email); defer with a reason; CSV export | 1.5 d |
| 02 D2 step 5: `tokens.gen.ts`, `paper` and the high-contrast themes (steps 2 and 3 moved to W1) | 3 d |

**W3, agents on the work (~12.5 d; ships with TM1).**

| Work | Size |
|---|---|
| Fleet: `panacea issue show`/`note`, `panacea evidence propose`, the `issue.delegated` event, `task.issueId` | 2 d |
| `Shift+O` Delegate; Orchestrator notes on the issue, one voice per owner | 1.5 d |
| Proposed evidence under each criterion in the Merge lens; *Record* as the person (phase 3) | 1 d |
| `negotiation.row` on both issues; `blocks` across owners seeds a `sequence` proposal (Fleet 1, desktop 1) | 2 d |
| Agent task-card pages, "whose agents" filters, `who.label`, `step.fold` on the issue | 1.5 d |
| Guidance records, the org kill switch, our published agent rules | 1 d |
| The read-only *ask* worker; named risk facts on lanes | 1 d |
| The same verbs for solo mode through phase 3's runner (**UNVERIFIED** that it can host them, 04 §4.5) | 1.5 d |
| §3.7 rule 3 (a cross-owner relation becomes a proposal from the editor's own Orchestrator) and rule 2's wind-down through the owner's Orchestrator | 1 d |

**W4, roadmap, release and reporting (~26.5 d).**

| Work | Size |
|---|---|
| **Cross-team planning** (P I4): a feature shared with a second team through a project-scoped read-and-comment grant; initiatives that span teams (L-67, L-83). Nested sub-teams, per-object sharing and sub-initiatives stay skipped (Q12) (Hub 2.5, desktop 1.5) | 4 d |
| Initiatives `in-`, updates `nt-` with health and a pinned brief, reminders (Hub 1.5, desktop 2) | 3.5 d |
| Timeline: zoom, drag to reschedule, the cycle band, grouping, feature dependencies | 3 d |
| Milestones `ml-` as named criterion subsets (Hub 0.5, desktop 1) | 1.5 d |
| Release notes grouped by criterion; freeze per environment; *shipped* for requesters (phase 7) | 1.5 d |
| Insights panel and scatter: measured counts and durations only (Q1) (Hub 1, desktop 2) | 3 d |
| Gateway-backed assists: triage suggestions with `serves` and a "why", the calibrated dial, duplicates, natural-language filters, composer hints | 4 d |
| Drafted updates and thread summaries as span-linked indexes; *propose issues per criterion*; *propose the next cycle* | 3 d |
| "Since you last looked" names issues; recurring issues; triage rotation; the user-facing method page | 3 d |

**W5, intake and the ecosystem (~28.5 d).**

| Work | Size |
|---|---|
| Requests `rq-`: Entra web form, email per team, intake rules, updates to the requester | 5 d |
| Tracker sync and import: Azure Boards, GitHub Issues and Jira **one way in**, into `triage` as *not tied · imported* (4 d); then **two way for comments only** (2 d). Sync never writes a state, a fact or `done` (§3.3; P I3). Linear and CSV import moved to W1 | 6 d |
| Outside PRs on both hosts: webhooks, magic words, linkbacks, branch-name start | 2.5 d |
| A Teams app: intake, feature feeds, the Desk digest, ask **your own** Orchestrator | 3 d |
| Public API: docs, generated TypeScript and Go clients, HMAC webhooks, app approvals, keys, rate limits | 3 d |
| The Work MCP server with a read-only mode; the connector allow-list | 2 d |
| Loops `lp-`: triggers, versions, run log, run now; the first-pass triage loop | 3.5 d |
| Release CLI, an Azure Pipelines task, a GitHub Action; harness allow-list and team skills; audit to Sentinel | 3.5 d |

### 5.5 Exactly what changes in the in-flight plans

All changes are **additive**; none throws away built work or interrupts the current wave. The Hub's
route table says *"Nothing else is routed"* (hub-contract:609), so the work layer arrives as **new
Hub tasks after task 43**, not edits to tasks in flight (04 §7).

| Plan | Change | Anchor | Slice |
|---|---|---|---|
| **Hub (Phase H)** | `issue:` in the key grammar the Hub mints and accepts | hub-contract:114-128 | **W0** |
| | Example `criterionId: "done:2"` → a `c-` id | :1065, :1079 | **W0** |
| | New tasks: id prefixes (§3.6); `issue-revision` canonical kind with golden vectors; `project.key` and numbers; routes for issues, comments, features, cycles, initiatives, views, requests, templates, labels, loops, drafts; the `Hub` interface gains `Work` | :103-112, :199-205, :561-607, :638 | W1-W5 |
| | Authorisation rows and events per write; erase for issues, comments and requests | :335-359, :1115-1160 | W1 |
| | The janitor gains cycle rollover, closure proposals, recurring issues | :2209 | W2, W4 |
| **Phase 4** | Task 06: the parser gains `issue:`; `provenance.Ref` gains `kind: "issue"` | `provenance/key.go` (not yet built) | **W0** |
| | Tasks 09-10: `Session.IssueRef`; the Materialiser renders from its keys | plan rules 2-3 | W1 |
| | Tasks 16-17: the loader's second key source; the issue chip; coverage in the definition of done | plan :330, :55, :483 | W1 |
| | After task 18: *promote to feature* drafts a branch brief (`model_drafted`); later *propose issues per criterion* | `4-D2` | W2, W4 |
| **Phase 3** | Task 05: the hook stamps `Panacea-Issue` | plan :26-27, :138 | W1 |
| | Tasks 07-08: a worktree made from an issue records it in the lane index | :116 | W1 |
| | Task 09: land trailers per issue, the `landed` move, the `done` rule's evaluator | :32-35, :108, :135 | W1 |
| | Task 19: lane rows name their issue. Tasks 15, 20: proposed evidence, *Record* as the person | :515-516; 3-D5, :414 | W1; W3 |
| **Phase 7** | Task 19: the manifest lists issues from trailers. Task 17: the drawer shows them | plan :150, :606 | W1b |
| | Release notes by criterion; freeze per environment | — | W4 |
| **Team mode (TM-05)** | `task.issueId`; two Orchestrator event kinds; three verbs; the strip line is the issue title; issue rows on the Desk; plan the Hub's new records together with W1's. **Stream F builds its Desk rows into W1 row 16's Desk frame**, not a second Desk (§5.6). If Q11 is B, W1's Hub rows review with stream B and its desktop rows run beside stream F | TM-05 §3.3, §4.2, §5.2, §9.1, §9.3, §11.1, §12.1 | W1, W3 |
| **Phase 5** | An issue revision's `implements: [design:…]`; a design approval can close a design issue | 04 §7.7 | later (0.5 d) |
| **Phase 6** | The per-session token carries the issue id: spend per issue and per loop | 04 §7.7 | W4 (0.5 d) |
| **Phase 8** | A per-project environment spec with prep scripts (**UNVERIFIED** it is planned); the Hub's web forms behind the edge allowlist | — | W5 |
| **Cockpit** | `Lens` gains `"work"` (`desktop/src/store/types.ts:173`); `isCockpitChord` extended for list keys; palette commands | `useKeyboardMap.ts`, `commands.ts` | W1 |
| **Docs** | `FUNCTIONAL-SPEC.md` object model gains Feature and Issue; `PROGRAM.md` gains Phase W; `AGENTIC-UX.md:245` one-line amendment (if Q1 is A) | FUNCTIONAL-SPEC:5-16; PROGRAM:54-65 | W1 |
| **02 (this folder)** | C8's replica "on disk" → in memory (§4.4), unless Q5 amends ADR-H1; `arrivals.hold` moves from `R` to `N`, and `N` applies every held change (D M10); `moves.hold` added beside it; the faces question reopened (Q14) | 02 C3, C7, C8 | W1 |
| **`AGENTIC-UX.md`** | Q13's four one-line amendments, if the owner accepts them | AX:245, §10 item 1, §5 | W1 |

### 5.6 A gap this design found: the Desk has no plan

`AGENTIC-UX.md` §5 makes the Desk the default landing surface, and TM-05's cockpit stream adds
rows to it (TM-05 §9.3, §12). **No plan builds it**: a search of `docs/superpowers/plans` for it
found nothing, and 04 found no `Desk` in `desktop/src`. W1 row 13 builds the minimum the work
layer needs. Revision 2 makes that minimum the **Desk frame** (W1 row 16: NEEDS YOU, FAILED,
LANDED, RUNNING, answer in place, the zero state), so a failed worktree is never hidden behind a
calm empty state (D C5). **There is one Desk build, not two** (P C2): TM1's stream F adds its rows
into this frame. The horizon bar and quiet mode still need a slice; Q4 names the owner.

### 5.7 Order against OD-17, and the elapsed path to each stop (P C2)

OD-17 says team mode is built *"straight after the current wave, as the product's next phase"*.
TM1 is ≈95.5 focused days on five streams, about 7 to 8 working weeks, behind a two-week TM1-α
spike that is the first slice of its streams B, C and D1 (TM-05 §12.0-§12.1). W1 needs the same
Hub and desktop people. Only the owner can reorder OD-17, so the order is **Q11**:

| Option | What happens | Cost |
|---|---|---|
| **A. TM1 first, then Phase W** | OD-17 as written | the Linear-shaped work starts about 8 weeks later |
| **B. W1 beside TM1** (recommended) | W1 runs as **two added streams** from TM1's start: its Hub rows review with stream B, its desktop rows sit beside stream F and share its Desk frame. Its demo runs in local mode; TM1's streams A and C then give it the cloud path. W2 overlaps TM1's tail; W3 follows TM1 | seven streams, not five. TM1's critical path (D1 → D2 → G, about 30 days) is untouched, but B and F gain a reviewer's load |
| **C. W1 before TM1** | W1 first, TM1 after | delays team mode by about 5 weeks, against OD-17 |

**Elapsed path to each stop, under option B** (focused days, then elapsed; none measured):

| Stop after | Phase W days | Also needs, not Phase W | Elapsed, from TM1's start | What the owner has |
|---|---|---|---|---|
| **W1, local mode** | ~48.5 | Hub task 43; phases 3 and 4 at the named tasks; a finished brief | **~4-5 weeks** | a team's backlog, comments and the new look; Run on any row; *landed and proven* on the Desk |
| **W1 in the cloud** | 0 more | phase 6 stream A (8.5 d), then phase 8 stream C (14.5 d), and **the owner's environment** (PROGRAM.md:64) | **~5-6 weeks** if the environment exists | the same, on managed ZDR sessions |
| **W1b** | ~1 | phase 7 task 19 | when phase 7 lands | *shipped to production, 14:02* |
| **W1-W3**, recommended first target | ~82.5 | **TM1 (≈95.5 d)** | **~10-12 weeks** | agents across owners negotiating on issues: the owner's "magic" |
| **W1-W5**, "everything they are doing" | ~137.5 | TM1; phase 6 for W4's assists | **~15-18 weeks** | 188 of 257 complete (§1 tally), 22 more reached |
| **W1 + W5's sync only** | ~55 | as W1 | ~5-6 weeks | the other tracker stays the plan; imports arrive *not tied*. Cheaper, but the thread stops at the mirror |

Under option A, add about 8 weeks to every row. Under option C, W1 is ~4-5 weeks after Hub task
43, and TM1 and the rows after it move out by the same.

---

## 6. Owner questions, each with a recommendation

### 6.1 Decide before W1 starts

| # | Question | Options | Recommendation |
|---|---|---|---|
| **Q1** | Progress displays. Linear shows percentages, burn-ups and predicted dates; `AGENTIC-UX.md` bans them (AX:237, :245) | **A** convergence per criterion plus plain group sizes for sets people committed to; **B** convergence only, no counts; **C** Linear's full set | **A.** A cycle is a human commitment, so its size is a number we can defend; anything that predicts stays banned. Needs one line added to AX:245 (04 §6.3) |
| **Q2** | How far to build | stop after W1 (~28 d); W1-W3 (~64 d); everything (~117 d); W1 plus sync only (~36 d) (§5.7) | **W1-W3 as the first target**, then decide W4 and W5 on what the first teams use. W1 alone is a demo; W3 is where the cross-owner magic shows |
| **Q3** | May an issue's text enter the agent's managed block once its owner started or delegated it? | yes with guards; no, agents read issues only as data | **Yes, with 04 §4.4's four guards**, and only after TM-05's injection corpus covers hostile issue text |
| **Q4** | Who builds the Desk, which no plan owns (§5.6)? | Phase W builds a minimum in W1; a separate Desk slice first; team mode's cockpit stream | **Phase W's minimum in W1**, then a full Desk slice before TM1's cockpit stream needs it |
| **Q5** | Keep the Hub replica in memory (ADR-H1) or allow an encrypted disk cache? | in memory; DPAPI-encrypted cache with erase propagation | **In memory for W1**, and measure `budget.return`. Amend ADR-H1 only if a real project misses 2 s |

### 6.2 Decide later

| # | Question | Recommendation |
|---|---|---|
| **Q6** | The 27 skips (§1 tally): mobile (L-232), SLAs (L-141, 142), the CRM (L-131, 134-136), support connectors | **Keep them skipped.** Revisit mobile after W3, starting with answering Desk items from Teams |
| **Q7** | Call Linear's *project* a **feature**, keeping *project* for the product (04 W1)? | **Yes.** Two meanings of one word in one rail confuses a first-time user permanently |
| **Q8** | Should an "AU data only" project's branch names default to no title slug (04 W4)? | **Yes.** An erase cannot reach a branch name on a customer's remote |
| **Q9** | Build the tracker, or sync with Azure Boards, GitHub Issues and Jira (03's open tension; 04 W5)? | **Both, in order:** build the first-class work layer, and sync out and import one way into `triage` in W5. Which trackers KPMG teams use is **UNVERIFIED** |
| **Q10** | Accept 02 D1's four design recommendations: Berkeley Mono, a light theme now, a 7-day UI-defect clock, the thread as the signature? | **Yes to all four**; the light theme lands in W2 through the generator |

---

## 7. What is unverified, and sources

### 7.1 Unverified

1. **Every size in §5.** Estimated from the plans' task sizes and 02 D2; none is measured.
2. **Linear facts** carry 01's and 02's caveats: pages were read through a summarising fetch tool;
   plan tiers conflict; the shortcut page returned 404; whether Linear has an acceptance-criteria
   object, or a ZDR option on request, is unknown.
3. **Our own plans:** that the Desk has no plan (searched `docs/superpowers/plans` on this branch
   only); that phase 3's runner can host the `panacea` verbs in solo mode; that phase 8 plans
   per-project prep scripts and fixes an AU hosting region; that phase 3 signs land commits and
   supports GitHub Enterprise Server; that the Tauri app builds for macOS and Linux.
4. **Performance:** 02's budgets with an in-memory replica, especially `budget.return` after a restart.
5. **Browser testing** inside a Kata sandbox (L-166).
6. **TM-05 was being revised while this was written** (its revision 3 began at commit `f032119`).
   Its section numbers are cited as read on 2026-09-25 and may move.
7. **Visual acceptance is the owner's eyes** (`CLAUDE.md`). The ASCII in §2 and §4 is a layout
   sketch, not a design; nothing here has been drawn or run.

### 7.2 Sources

**No web page was read for this file.** Every Linear fact is cited from `01`-`04` in this folder,
which list their URLs and read dates (all 2026-09-25). Plane's source was not opened.

**Read in this repository, 2026-09-25:** `docs/superpowers/planning/2026-09-23-owner-decisions.md`
(OD-1 to OD-20); `01-linear-feature-inventory.md` §21-§23; `02-linear-design-language.md` Part C
and D; `03-agent-work-landscape.md` headings and §12; `04-internal-fit.md` §1-§8;
`../2026-09-24-team-collaboration/05-design.md` §0, §6.2-§6.5, §9.1-§9.3; `docs/AGENTIC-UX.md`
§5-§10; `docs/DESIGN-SYSTEM.md` (rules only); `desktop/src/store/types.ts:173`; the Hub contract's
id-prefix lines 105-112 in the `feat/hub` worktree.

---

## 8. Response to critique (revision 2, 2026-09-25)

Two adversarial reviews read revision 1: `06-critique-product.md` (**P**, 4 Critical, 10
Important, 8 Minor) and `07-critique-design.md` (**D**, 5 Critical, 10 Important, 10 Minor).
**Every Critical and Important finding is fixed in the body of this file**, at the section named
below. None is rebutted outright; three are accepted with a narrower fix than the critic proposed,
and the reason is given in the row. Minor findings are fixed where the fix is one line (§8.3).

The two reviews agree on the direction (Hub issues on the thread; fact-driven states; the Desk
absorbing Inbox) and both list what must survive (P "What holds up", D "What is genuinely
better"). Nothing on either list was changed except where a Critical finding required it.

