# 04 — Internal fit: a Linear-class work layer on PanaceaApp

**Date:** 2026-09-25. **Status:** research and design, for the owner's review. Not a plan.
**Question:** where does a Linear-class work layer (issues, projects, cycles, initiatives)
sit in PanaceaApp as it is planned and built, and what must change so that it does?

**Scope of this file.** The *internal* fit only. What Linear does, and what it does well, is
the other files in this folder. This file names Linear's concepts only by their public, generic
names (issue, project, cycle, initiative, triage), which are ordinary industry terms. It copies
no Linear asset, copy, brand or code. Plane's source was not opened.

**Binding inputs.** OD-1 to OD-20 (`2026-09-23-owner-decisions.md`), most of all OD-3 (shared
state in the Hub), OD-14 to OD-17 (team mode: Orchestrators are the only cross-user channel,
agent awareness primary, human view a calm summary), OD-19 (org-level ZDR inference) and OD-20
(cloud-first). `CLAUDE.md`'s product spine: **one provenance thread, and any feature that breaks
it is wrong regardless of how good it is in isolation.** `DESIGN-SYSTEM.md`'s non-negotiables.

**Evidence rule.** Every claim about PanaceaApp below cites a file and, where it matters, a
line or section. Anything not read is marked **UNVERIFIED**. No external web page was needed
for this file; the sources list at the end says so.

## 0. The answers in eight lines

1. **An issue is a new object, not a brief and not a criterion.** A brief says *why and what done
   means* and is immutable per revision; an issue says *who, when, and how far* and changes all day.
   Folding one into the other breaks either the digest (§3.2) or the tracker. The issue **points at**
   the brief and the criteria it serves, and that pointer is how the thread reaches it (§3).
2. **All four Linear-class records live in the Hub** (`OD-3`, 05-design §3.2's placement rule:
   assignment, priority and scope are decisions). Live progress stays Fleet state and is *derived*
   onto the issue, never stored on it.
3. **Names are the first trap.** The Hub already has a `project` (`pj-`): a team's product, bound to
   repo remotes. Linear's "project" is a deliverable. Here the deliverable already exists: **a branch
   with its branch brief**. Call it a **feature** in the product; keep `project` for the product.
4. **Mapping:** Linear team → Hub `team` (exists). Linear project → **feature** = branch + branch brief
   (exists) plus a thin planning record (new). Linear issue → **issue** (new). Linear sub-issue →
   issue with a parent (new) and, below it, team mode's agent **task** `tk-` (designed, 05 §3.3).
   Cycle → **cycle** (new, per team). Initiative → **initiative** (new, per org or team).
5. **An agent takes work through its owner, never directly.** An issue is assigned to a *person*;
   that person's Orchestrator plans tasks for it (OD-16). No one can hand an issue to someone
   else's agent. Solo mode skips the Orchestrator: *Start* opens a lane with the issue bound.
6. **Progress written back is three kinds, kept apart:** observed facts (lanes, commits, lands, files;
   from git and `sandboxd`), bounded agent text (`text·A`, attributed), and human judgements
   (state to Done, evidence). Agents may **propose** evidence; a human records it (phase 3's 3-D5).
7. **The thread gains one hop:** `brief:bf-…@n` → **`issue:is-…@r`** → session → commit trailer
   `Panacea-Issue` → land → evidence per criterion → release manifest. The issue is the only new
   key, and it names immutable text the same way a brief key does.
8. **Plans that must change:** the Hub contract (most), phase 4 (session gets an issue ref, coverage
   view), phase 3 (trailer, lane ↔ issue, evidence proposals), phase 7 (issues in the manifest),
   team mode 05 (task gains `issueId`; the Desk absorbs Linear's inbox), phase 2 (a `work` lens).
   Section 7 lists each change with the file and line.

**One conflict needs the owner.** Linear's cycle and project progress are percentages and graphs.
`AGENTIC-UX.md` bans percent-complete, bars and "N of M" anywhere (`AGENTIC-UX.md:77`, `:237`).
Section 6.3 recommends convergence instead, and asks one question.

## 1. What exists already

Build status is as of 2026-09-25, read from each worktree's `git log` and plan. "Built" means code on
a branch; "planned" means a reviewed plan; "designed" means a design doc with no plan.

| Linear-class need | What PanaceaApp has | Status | Where |
|---|---|---|---|
| Workspace / org | `org` (`org-`), one per Entra tenant | Hub, planned; task 02 of 43 in build | `ADR-H1-hub-architecture.md:402` |
| Team, members, roles | `team` (`tm-`), `admin`·`member`·`viewer`, invites | planned | `hub-contract.md:566-574`, `:324-333` |
| A product with repos | Hub `project` (`pj-`), owned by one team, bound to remotes; repo resolves to one project | planned | `ADR-H1:407-408`; `hub-contract.md:638` |
| **Spec of a deliverable** | **brief**: one project brief per project, one branch brief per `(project, branchRef)`; five questions `purpose`, `audience`, `constraints`, `done`, `out_of_scope`; a branch brief inherits the project brief | phase 4 in build (task 03 done) | `hub-contract.md:755-761`; phase-4 `discover/content.go:33-37` |
| **Acceptance criteria** | `Criterion{ID "c-"+8hex, Text}` on the `done` answer; id kept while the text survives | built (phase 4 task 03) | phase-4 `discover/content.go:160-174` |
| Immutable versions, keys | `brief:bf-…@n`, canonical bytes, SHA-256 digest, `/v1/refs` resolver | planned | `hub-contract.md:114-128`, `:139-176` |
| Work units, one agent each | **lanes**: one local clone per worktree, fetched to `refs/panacea/lanes/<wt>` | phase 3 planned; no lanes code yet | phase-3 plan `:22-31`; 05-design `:841` |
| Ordering of work into a branch | **merge queue** per branch, order in the lane index, chained forecast | planned | phase-3 plan `:180-184`, `:364` |
| Commit ↔ who made it | `Panacea-Session` trailer; land commit carries the brief key trailer | planned | phase-3 plan `:26-27`, `:32-35` |
| Done-ness | **evidence** per criterion at a commit, `settled`·`contested`, a human's `because`, matched by criterion-text digest | planned; Hub route H6 | phase-3 plan `:167`, `:208-210`; `hub-contract.md:1062-1087` |
| Release with scope | **release** reserve → confirm; the Hub **pins** the branch's latest complete brief revision | planned (b11 review) | `hub-contract.md:957-986`; phase-7 plan `:108`, `:155` |
| Change notifications | `/v1/changes`, gapless per-org feed, metadata only | planned | `hub-contract.md:1089-1107` |
| Erasure of customer text | brief erase keeps keys, removes text | planned | `hub-contract.md:1115-1127` |
| Agent-level plan item | team mode **task** `tk-`: title, reason = `briefRef` + why-line, scope globs, state | designed | 05-design `:218` |
| Who is doing what | team strip, one line per person, files observed | designed | 05-design `:650-673` |
| Inbox | **the Desk**: one ranked queue, account-level, above projects | designed; not built (no `Desk` in `desktop/src`) | `AGENTIC-UX.md:85-151`; 05-design `:691-704` |
| Keyboard-first command surface | command palette | built (phase 2) | `desktop/src/components/palette/CommandPalette.tsx`, `commands.ts` |
| Navigation | rail: project → branch → worktree → session; six lenses | built; only `develop` has a surface | `desktop/src/store/types.ts:171-173`; `components/rail/` |

**What does not exist anywhere, planned or designed:** a mutable, assignable, prioritised work item;
a backlog or triage state for work not yet started; a time box; a grouping above one branch; a
human-readable short id (`EP-142`); relations such as *blocks*; comments on a unit of work; saved
views. None of the prototypes has an issue list either: the only matches for issue-tracker words are
v2's and v3's "Inbox — needs you" rail button (`prototype/v2.html:394`, `v3.html:342`), which
`AGENTIC-UX.md` §5 already turned into the Desk.

**One stale example to correct.** `hub-contract.md:1065` shows `criterionId: "done:2"`. Phase 4 as
built mints `c-` + 8 hex (`content.go:164`). The contract's example should follow the code.

## 2. Where issues, projects, cycles and initiatives live

### 2.1 The names

The product cannot have two things called *project*. The Hub's `project` is already contracted,
routed and in the cockpit's rail (`hub-contract.md:638`; `components/rail/ProjectRow.tsx`). It
means *a team's product and its repositories*. So:

| Linear's word (01 §1) | PanaceaApp's word | Why this word |
|---|---|---|
| workspace | **org** | exists |
| team | **team** | exists; same meaning (owns members; here also owns cycles) |
| — | **project** | exists; the product. Linear has no exact match: nearest is a team's product area |
| project | **feature** | the plain word for one branch's worth of work: it has a spec, a done line, lanes and a release. A proposal; the owner may rename it |
| issue | **issue** | generic industry word; no collision (checked: no `issue` entity in `hub-contract.md`) |
| sub-issue | **issue** with a parent | one object, not two |
| (none) | **task** `tk-` | team mode's agent plan step (05 §3.3). Below an issue; written by an Orchestrator |
| cycle | **cycle** | generic word; no collision |
| initiative | **initiative** | generic word; no collision |
| project milestone | **milestone** on a feature | later; see §2.4 |

### 2.2 The placement rule applied

05-design's rule (`05-design.md:205-208`): *if losing it would lose a decision or an
accountability fact, it is a Hub record; if it is a fact about now, it is Fleet state.*

| Record | Id | Lives in | Why there |
|---|---|---|---|
| issue head (state, assignee, priority, cycle, feature, parent, labels, estimate) | `is-` | **Hub** | every field is a decision someone made and may be asked about |
| issue revision (title, description, the criteria it serves) | `issue:is-…@r` | **Hub**, immutable | an agent is told this text; §3 needs to name exactly what it read |
| issue relation (blocks, duplicates, relates) | on the issue | **Hub** | a decision |
| issue comment | `ic-` | **Hub**, append-only | an accountability fact; author class `text·H` or `text·A` (05 §3.3) |
| feature (planning fields of a branch brief: lead, target date, state, initiative) | `ft-` | **Hub** | decisions; the spec itself stays the brief |
| cycle | `cy-` | **Hub**, per team | a commitment with dates |
| initiative | `in-` | **Hub**, per team or org | a commitment |
| status update on a feature or initiative | `nt-` with `kind: update` | **Hub**, immutable | reuse 05's `note` rather than add a record |
| saved view (a filter and a layout) | `vw-` | **Hub** if shared; renderer memory if private | a shared view is team knowledge; a private one is a preference |
| *who is on this issue right now, which files, which lanes* | — | **Fleet state**, derived | a fact about now (05 §3.3 `activity`); never stored on the issue |

Two rules come with it, both already the Hub's: **every write appends one metadata-only event in
the same transaction** (`hub-contract.md:1133-1134`), so the cockpit and every Orchestrator learn
of issue changes through the existing `/v1/changes` feed; and **no client persists Hub data to
disk** (`ADR-H1:467-474`), so the issue list is held in memory, like briefs. That second rule costs
offline issue editing. It is the same trade `ADR-H1` already made for Discover.

### 2.3 The containment tree

```
org ─ team ─┬─ project (product, repos) ─┬─ project brief ─ criteria
            │                            ├─ feature ft- ── branch brief bf- ─ criteria
            │                            │     └─ issue is- ─ issue (child) ─ task tk- (agent)
            │                            └─ issue is- with no feature  (backlog, triage, small fixes)
            ├─ cycle cy-  ◄── issues point at one cycle (across the team's projects)
            └─ initiative in- ◄── features point at one initiative (across projects)
```

An issue belongs to **exactly one project** (as a Linear issue belongs to one team, 01 §1),
because its access must be exactly its project's team membership (`hub-contract.md:333`). A
cycle is a **team's**, because the people are the team's, and it may hold issues from several of
the team's projects. An initiative groups features, possibly across the team's projects; an
org-level initiative (across teams) is later: v1's only org-level role is the org admin, for audit
and erasure (`hub-contract.md:359`, `:606-607`), and access to an initiative spanning teams would need
a grant v1 does not have (*"There are no per-project grants in v1"*, `hub-contract.md:333`).

### 2.4 The issue, field by field

**Head** (mutable, outside any digest; like `BriefHead`, `hub-contract.md:641-644`):
`id is-…`; `projectId`; `number` (gapless per project, so a person can say **EP-142**; the prefix
is a new `project.key`, 2–5 capitals, set by a team admin); `state`; `assignee` (a **person**,
never an agent, §4.1); `priority` `none`·`urgent`·`high`·`medium`·`low`; `estimate` (integer
points, optional); `labels` (team-owned); `parentId`; `featureId` or null; `cycleId` or null;
`dueDate`; `relations` `[{kind: blocks·duplicates·relates, issueId}]`; `headRevision`; `source`
`person`·`orchestrator`·`intake`; `createdBy`.

**Revision** (immutable, digested with the Hub's canonical encoding, `hub-contract.md:139-176`,
new `kind: "issue-revision"`): `title` ≤ 200 and `description` ≤ 16 KiB, each with its author class
(`text·H`·`text·A`, 05 §3.3); and **`serves`**: a list of `{briefKey, criterionId}` pairs, the
criteria this issue exists to satisfy (§3.3). A revision is written only when the text or `serves`
changes, so reprioritising all afternoon writes no revision and invalidates no agent's context.

**States**, in categories the product fixes (a team may rename them later, not add categories):

| State | Set by | Meaning |
|---|---|---|
| `triage` | intake, an Orchestrator's proposal, a person outside the team's normal flow | not yet accepted by anyone |
| `backlog`, `todo` | a person | accepted; not started |
| `started` | **the fact**: a lane or task was bound to it (§4), written as the person who started it | work exists |
| `in_review` | **the fact**: a bound lane was marked ready or entered the merge queue | a human must look |
| `landed` | **the fact**: a land commit carries its `Panacea-Issue` trailer (§5.2) | on the feature branch |
| `done` | a person; **or** automatically at landing when every criterion in `serves` is `settled` at that commit | finished by the product's own standard |
| `cancelled`, `duplicate` | a person | closed without work |

Fact-driven moves are Linear's Git automation (01 §2.1) done on our own ledger, and they differ in
one way that matters: **each is written as the human whose act caused it** (the lander, the person
who started the lane), with the fact that caused it on the event. Nothing writes as *"the system"*
except cycle rollover (§2.5), whose event has `actor: null`, which the feed already allows
(`hub-contract.md:1096-1097`). A person can always move an issue by hand; the row then shows both.

*Shipped* is **not a state**. It is a fact the release manifest proves (§5.4), shown on the issue:
*"In release to `production`, 14:02, pinned `brief:bf-…@6`."*

### 2.5 Feature, cycle, initiative

**Feature `ft-`.** A thin planning record beside a branch brief: `projectId`; `name`; `branchRef`;
`briefId`; `lead` (a person); `targetDate`; `state` `planned`·`active`·`paused`·`done`·`cancelled`
(a person sets it; the facts beside it are derived: lanes live, landed, released where);
`initiativeId`. The spec is **not** on the feature. It is the branch brief, unchanged.

*A feature before its branch exists.* A roadmap holds features nobody has started, and Define comes
first, so the brief must be writable before the branch is cut. The Hub already allows this: it
validates a `branchRef`'s **format** (`hub-contract.md:131-137`) and never reads git (*"the Hub
stores; the control plane decides"*, phase-4 plan `:90`). So creating a feature reserves
`refs/heads/<slug>` and creates its branch brief at once; the git branch is created on the first
Start (§4). The cockpit must then refuse to cut a branch of that name for anything else. The Hub's
`brief_exists` (409, `hub-contract.md:253`) already stops a second brief on the ref.

*Milestones* (Linear's checkpoints inside a project, 01 §5.4) map onto **a subset of the branch
brief's criteria**, named, with a date. A milestone is *reached* when its criteria are settled at
the branch head. That is later, and it is cheap because it is only a label on criteria.

**Cycle `cy-`.** Per team: `number` (gapless per team), `startsOn`, `endsOn`, `state`
`upcoming`·`current`·`closed`. Issues point at one cycle. At `endsOn`, the Hub's janitor (it already
exists under a Postgres advisory lock, `hub-contract.md:2209`) moves every issue not `done`,
`cancelled` or `duplicate` to the next cycle, one event per issue, `actor: null`. The team sets the
cadence once; the janitor creates the next cycle ahead, so nobody sets one up by hand.

**Initiative `in-`.** Per team in v1: `name`, `owner` (a person), `targetDate`, `state`,
and status updates (`nt-` with `kind: update`, a health word `on_track`·`at_risk`·`off_track` and
`text·H`). Features point at it. Nothing else.

### 2.6 Where they appear in the cockpit

| Surface | Scope | What it holds | Today |
|---|---|---|---|
| **The Desk** | account, above projects (`AGENTIC-UX.md:87`) | issues **only when they need you**: assigned to you in `triage`; `in_review` where you are asked; a comment naming you; your issue blocking someone's agent. Rows join the Desk's existing sections, no new section, as 05 §9.3 does for escalations | designed, not built |
| **Work** (new, project level) | selected project | triage, backlog, the features board, the current cycle, initiatives, saved views, *My issues* as a view | nothing |
| **Work** lens (new, in a branch) | one feature | its issues, the brief's criteria with the issues that serve each (coverage, §3.3), who is on each issue now | nothing |
| **Develop** | a branch | each session pane's header shows its issue beside the brief chip | brief chip planned (phase-4 plan `:54-57`) |
| **Merge** | a branch | each lane row names its issue; the DoD above the queue shows agent-proposed evidence (§4.4) | planned (phase-3 plan `:515-516`) |
| **Deploy** | a branch | the release manifest lists the issues shipped | planned (phase-7 plan `:606`) |
| **Palette** `⌘K` | everywhere | *new issue*, *go to EP-142*, *assign to me*, *start EP-142 in a new lane*, *move to cycle* | palette built |

`Lens` gains one member, `"work"` (`desktop/src/store/types.ts:173`). The rail stays navigation
only, with no counts (`AGENTIC-UX.md:144`, `:245`). The union is persisted (`store.ts:26`, `:46`),
and an added member is backward compatible, so `PERSIST_VERSION` can stay `1`. **UNVERIFIED:**
whether rehydration validates an unknown lens value, which matters only for a downgrade.

_(Sections 3–8 follow, written in parts.)_
