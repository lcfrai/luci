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

