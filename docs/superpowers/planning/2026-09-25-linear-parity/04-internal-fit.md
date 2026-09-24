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
line or section. Anything not read is marked **UNVERIFIED**. One public Linear page was read, to
check one claim; §8 lists it with its date. Other Linear facts are cited from the sibling inventory
(`01-linear-feature-inventory.md`), which records its own sources.

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

Fact-driven moves are Linear's pull-request automation done on our own ledger. Linear's GitHub
page says: *"we will move linked issues to 'In Progress' when PRs are open and 'Done' when PRs
merge"* (https://linear.app/docs/github, read 2026-09-25). Ours differ in
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

## 3. Is an issue a brief, a criterion, or a new object?

**Recommendation: a new object that points at the brief and at the criteria it serves.** Each of
the other two readings breaks something already built or contracted.

### 3.1 If an issue were a brief

| What breaks | Where it is fixed today |
|---|---|
| One live branch brief per `(project, branchRef)`. A 200-item backlog would reserve 200 branches | `hub-contract.md:760-761` |
| Every text change is an immutable, digested revision, and a project edit fans a new revision out to every inheriting branch brief. Reassigning or reprioritising would either churn revisions, and mark every running agent's context `superseded`, or need fields outside the digest that the brief does not have | `4-D3`, phase-4 plan `:144`; `P4-B`, `:172` |
| A brief is `complete` only when all five questions are settled, and only a complete revision is inherited, pinned or approved. A two-line bug report is never complete, so an agent could never inherit it | `content.go:40-41`; `C9`, phase-4 plan `:32` |
| Five questions for every bug report is *"a wall of configuration"* | `CLAUDE.md`, "Why this exists" |
| Inheritance is two levels, project → branch. Sub-issues would need a third | phase-4 plan `:310` (`Resolve(project, branch)`) |

### 3.2 If an issue were a criterion

| What breaks | Where it is fixed today |
|---|---|
| A criterion's identity is its text: its id lives only while *"a line with exactly this Text survives"*, and evidence matches by the text's digest. Retitling an issue would orphan its evidence | `content.go:160-162`; phase-3 plan `:208-210` |
| A criterion is a truth condition (*"a 40 MB PDF uploads"*), not work. Several issues serve one criterion; a refactor serves none | — |
| Criteria live inside an immutable revision. Assignee and state cannot | `hub-contract.md:646-650` |
| Adding an issue would mean writing a brief revision, with its fan-out | `4-D3` |

### 3.3 The new object, and what it gains

The issue carries **`serves: [{briefKey, criterionId}]`** in its immutable revision (§2.4). Both
identifiers are already immutable, so the thread is kept by reference and no brief changes. It
buys three things the other readings cannot:

1. **Two rates of change stay apart.** The spec changes rarely and is pinned; tracking changes all
   day and is not. An agent's context is invalidated only when either *text* changes.
2. **Coverage.** The branch's definition of done lists, per criterion, the issues serving it and
   the evidence settling it, as the convergence states `AGENTIC-UX.md:77` asks for: *settled*
   (evidence at the head), *open* (served, not settled), *contested*, **uncovered** (no issue
   serves it), and **drifting** (an issue serves a criterion whose text has changed since the issue
   was written; detected by comparing the criterion's digest at the brief head with the one the
   issue's revision resolved). An issue serving nothing is shown as *not tied to the spec*, which is
   allowed (a refactor) but visible.
3. **Both directions of planning.** *Plan from the brief:* the Orchestrator, or the model in solo
   mode, proposes one issue per uncovered criterion, `source: orchestrator`, into `triage`, as
   `text·A` that a person accepts (the `4-D2` rule, phase-4 plan `:143`, applied to issues).
   *Promote to feature:* a backlog issue becomes a feature, and its text drafts the new branch
   brief's answers as `model_drafted`, held until a person accepts them. The issue stays as the
   feature's first issue, so nothing written is lost.

Whether Linear itself has an acceptance-criteria or evidence object is **UNVERIFIED**. The sibling
inventory lists none: a search of `01-linear-feature-inventory.md` for *criteri*, *acceptance* and
*definition of done* found no hit on 2026-09-25. If that holds, coverage is the clearest place
where this product does more than Linear, not just the same.

## 4. How an agent session takes work from an issue, and writes back

### 4.1 The rule: issues go to people, and people's agents take them

An issue's `assignee` is always a **person**. *"Give it to the agents"* is the assignee's own act:
**Start** (solo) or **Delegate to my Orchestrator** (team). Nobody can hand an issue to another
person's agent, because *"no user steers another user's agent"* (OD-16, `owner-decisions.md:127`).
Assigning EP-142 to Alice puts it on Alice's Desk; Alice's Orchestrator acts on it only if Alice
delegates it or her standing policy says so (05 §6.5's legislated escalation defaults). This is the
one place the product deliberately differs from Linear's *delegate an issue to an agent* (01 §12.5):
there, the agent is the workspace's; here, every agent has an owner who answers for it.

### 4.2 Solo: Start

1. A person presses **Start** on EP-142 (issue view, palette, or dragging it to *started*).
2. The control plane picks the branch: the feature's branch if the issue has a feature, else a new
   `refs/heads/ep-142-<slug>` with a branch brief that inherits the project brief. An empty branch
   brief is fine: a session inherits *"the branch brief's latest complete revision, else the
   project brief's"* (`C9`, phase-4 plan `:32`).
3. It creates a worktree (phase 3's clone, OD-2) and a session whose record gains
   **`IssueRef = issue:is-…@r`**, the issue's head revision at that moment, beside phase 4's
   `ContextRef` (`brief:bf-…@n`). Both are minted by the session handlers only (phase-4 plan `:95`).
4. The Materialiser renders the managed block from **both keys**. Phase 4's rule *"the resolved
   text an agent read is a pure function of one key"* (`:97-99`) becomes *"of its keys"*: still
   immutable, still never read back from a file.
5. The sandbox gets `PANACEA_ISSUE=EP-142` beside the session id, so phase 3's commit hook adds a
   `Panacea-Issue:` trailer (§5.2).
6. The issue moves to `started`, written as that person, with the lane as the cause.

### 4.3 Team: Delegate, then the Orchestrator plans

1. Dylan delegates EP-142. The Hub writes `issue.delegated`; Fleet adds it to Dylan's
   Orchestrator's event queue, a new kind beside 05 §4.2's list.
2. The Orchestrator runs `panacea issue show EP-142` (new verb). The text arrives as a tool result,
   which the model reads as data (05 §5.2 point 5).
3. It plans with `panacea task create --issue EP-142 …`. **The task gains `issueId`**; its
   `briefRef` stays, so its reason reads task → issue → criterion → brief.
4. `panacea worker spawn --task tk-…` starts a worker whose session carries the task's issue as its
   `IssueRef`, rendered as in §4.2 step 4.
5. The team strip's line for Dylan (05 §9.1) becomes **the issue's title**, which is usually
   `text·H`, instead of the Orchestrator's task title, which is `text·A`. Calmer, and one fewer
   agent-written string on a teammate's screen.

### 4.4 Is issue text allowed in the managed block?

05 and 04 rule that *"nothing from a teammate is ever rendered into"* the managed block
(`05-design.md:875-876`; 04's F4). A brief, which teammates also write, is rendered, so F4's line
is about *coordination* text, not the spec. An issue sits between. Recommendation, for review:

- An issue **revision** is spec-class. It is rendered only after **its owner adopted it** (Start or
  Delegate is that act), passes the Hub's F2/F3 schema and character hygiene
  (`04-enterprise-constraints.md:340-341`), and is rendered with attribution: *"EP-142, written by
  Alice (person) 10:02; started by you."* (F6.)
- A `text·A` revision (an Orchestrator's proposal) is **never** rendered until a person accepts or
  rewrites it, the `4-D2` rule.
- **Comments are coordination-class and never rendered.** An agent reads them only through
  `panacea issue show`, as data.
- This needs F8's injection corpus to include hostile issue text before it ships
  (`04-enterprise-constraints.md:346`).

### 4.5 Progress, written back in three classes that never mix

| Class | What | Who writes it | How | Stored |
|---|---|---|---|---|
| **Observed** | lanes bound to the issue; files read and edited; commits carrying its trailer; queue position; lands | `sandboxd`, phase 3's lane syncer, Fleet's activity table | derived; no agent call | Fleet state, live (05 §3.3 `activity`, 45 s TTL). A land also writes the `landed` state move to the Hub, as the lander |
| **Agent text** | *"blocked on the upload size limit"*; an assumption; a hand-off note | the **owner's** agents on that issue, and in team mode **only the Orchestrator** | `panacea issue note <text>` (new verb) → an issue comment, `text·A`, ≤ 600, attributed, rate-limited per issue | Hub, append-only |
| **Human judgement** | state to `done`; relations; cycle; evidence; accepting a proposal | a person | the cockpit | Hub |

Three consequences:

- **Workers do not post to issues.** They `panacea report done|blocked|assumed` to their own
  Orchestrator (05 §5.2), which decides what reaches the issue. The issue hears one voice per owner,
  not one per worker, which is what keeps it calm (OD-15).
- **An agent cannot claim progress.** *Started*, *in review* and *landed* come from facts; agent
  text is labelled as such wherever it is shown (05 P5, `05-design.md:84`).
- **Solo mode has no `panacea` channel yet.** It is designed with Fleet (05 §5.2, §11.4). Until the
  control plane serves the same verbs through phase 3's `sandboxd` runner, solo issues get observed
  facts and human text only. **UNVERIFIED** that the local runner can host the verb set unchanged.

### 4.6 Evidence: agents propose, people record

Phase 3's `3-D5` fixes that *"evidence is recorded by a human, at a commit, with a required
because"* (phase-3 plan `:167`), and the Hub's route records the caller as `recordedBy`
(`hub-contract.md:1074`). That stays. What is added is a proposal step:

1. An agent runs `panacea evidence propose --criterion c-… --commit <sha> --because "<text>"`.
2. It becomes an issue comment of kind `evidence_proposal`, with typed fields (criterion id,
   commit, `because` as `text·A`). It **does not count** towards *settled*.
3. The Merge lens's definition of done shows it under its criterion, as *"proposed by Dylan's
   agent"*, through phase 4's planned `onRecordEvidence?(criterionId)` hook (phase-4 plan `:483`;
   phase-3 plan `:414`).
4. A person presses **Record**, edits or keeps the `because`, and the control plane calls
   `POST /v1/evidence` **as that person**. The comment is linked to the record it became.

Linear's coding sessions attach *verification artifacts (screenshots, recordings)* (01 §12.5).
The Hub already stores content-addressed blobs per project (`hub-contract.md:590-591`), so an
evidence record could name blob digests (a test log, a screenshot). That is a later, additive field.

## 5. The provenance thread, extended to issues

Today's thread (phase-4 plan `:54-58`; phase-3 plan `:26-35`; `hub-contract.md:957-986`):
`brief:bf-…@n` → session `ContextRef` and its chip → definition of done above the merge queue →
land commit trailer → evidence per criterion at a commit → release pin. With issues:

```
 Define            Design                 Develop                                   Deploy
 brief:bf-…@n ◄─serves── issue:is-…@r ──► session IssueRef + ContextRef
   criteria c-…         │  implements        │  commit trailers: Panacea-Session, Panacea-Issue
                        └─► design:cv-…@k    ▼
                                           land commit: brief trailer + Panacea-Issue per issue landed
                                             │  evidence(briefKey, criterionId, commit)   (unchanged)
                                             ▼
                                           release: pin = brief (unchanged); manifest lists issues
```

### 5.1 The key

`issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})`, added to the key grammar the Hub mints and accepts
(`hub-contract.md:114-128`), which phase 4's `provenance` package owns (P5). `/v1/refs` resolves it
in the same batch as brief and design keys, so the cockpit's one loader (phase-4 plan `:330`)
handles it with one more kind, and `provenance.Ref` gains `kind: "issue"`. An issue chip opens the
issue **at that revision**, read-only, as a brief chip opens Discover at its revision (`:57-58`).

**EP-142 is a display alias, never a key.** It depends on `project.key`, which an admin can change.
Every stored or git-written reference uses the `is-` id.

### 5.2 Commits, lands and the queue

- The commit hook phase 3 ships adds `Panacea-Issue: issue:is-…@r` from the session's `IssueRef`,
  beside `Panacea-Session` (phase-3 plan `:26-27`). A commit therefore says exactly which issue
  text the session that wrote it was told. Linear links work by the issue id in a branch name or
  a *magic word* in a PR or commit (https://linear.app/docs/github, read 2026-09-25); here the hook
  writes the link, so neither a person nor an agent has to remember a word.
- A land commit adds one `Panacea-Issue:` per distinct issue in the landed range, beside the brief
  trailer the human saw (`:32-35`). The land writes each issue's `landed` move, as the lander.
- **Trailers carry ids, never titles.** A title in git history is text an erase cannot reach
  (`hub-contract.md:1115-1127`). The same applies to a branch name: *Start* offers
  `ep-142-<slug>`, and the person may clear the slug. **Owner question:** should an `AU data only`
  project (OD-19's classification) default to no slug?

### 5.3 Evidence and coverage

Evidence is unchanged: `(briefKey, criterionId, criterionDigest, commit, verdict, because)`
(`hub-contract.md:1062-1083`). The issue link is **derived**: an issue serves a criterion (its
revision says so), and evidence settles it at a commit that contains the issue's landed work
(phase 3's evaluator over ancestry, phase-3 plan `:135`). Nothing is added to the H6 route.

### 5.4 Releases

The pin stays the brief (`hub-contract.md:983-986`). Issues are not pinned, because they do not
need to be: the shipped range is immutable git, and phase 7 already lists **sessions** from the
`Panacea-Session` trailers in that range (`D3`, phase-7 plan `:150`). It lists **issues** from the
`Panacea-Issue` trailers by the same parse. *"Shipped to `production`"* on an issue is then derived
from releases and ancestry, the evaluator phase 3 leaves for phase 7 (`S3-P7.5`, phase-3 plan
`:428`). Release notes can be assembled from the shipped issues' titles, all `text·H` or labelled.

### 5.5 Design, and erasure

- An issue revision may name **`implements: [design:cv-…@k]`**. A session started from it
  inherits that design ref through phase 5's session link (phase-4 plan `:500`, `S4-P5.2`).
- Issue text is customer content, so **an issue needs the brief's erase**: org admin only; text
  removed; key, digest and revision numbers kept, so every trailer and chip still resolves, as
  *erased* (`hub-contract.md:1115-1127`). Comments are erased with their issue.

## 6. What must be added

Sizes are focused days for one implementer who knows the code, in the style of 05 §11. They are
estimates from the plans' own task sizes, **not measured**.

### 6.1 The build list

| # | Addition | Where | Size |
|---|---|---|---|
| 1 | issue head and revision, `issue-revision` canonical kind, the `issue:` key, `/v1/refs` resolution, per-project numbers, `project.key` | Hub | ~3.5 d |
| 2 | comments (`text·H`/`text·A`), `evidence_proposal` comments, relations, labels, parent | Hub | ~2 d |
| 3 | feature `ft-`, cycle `cy-` with janitor rollover, initiative `in-`, `note.kind = update` | Hub | ~3.5 d |
| 4 | events for every write, authorisation matrix rows, issue erase, title search | Hub | ~2.5 d |
| 5 | `IssueRef` on the session, the Materialiser's second key, `provenance.Ref` kind `issue` | phase 4 | ~2 d |
| 6 | `Panacea-Issue` trailer in the hook, land trailers and the `landed` move, evidence proposals in the DoD, coverage data | phase 3 | ~2.5 d |
| 7 | issues in the release manifest, *shipped* derived | phase 7 | ~1 d |
| 8 | `panacea issue show\|note`, `panacea evidence propose`, the `issue.delegated` event, `task.issueId` | Fleet (team mode) | ~2 d |
| 9 | Work surface (project), Work lens (branch), issue view, Desk rows, palette commands | desktop | ~9 d |
| | **Total** | | **~28 d** |

The desktop row is the largest and the least certain: it is the part the owner judges by eye
(`CLAUDE.md`, working agreements), and `02-linear-design-language.md` sets its visual bar.

### 6.2 Where this does more than Linear, from the internal fit alone

1. **Done means proven.** An issue reaches `done` by itself only when every criterion it serves is
   settled by recorded evidence at the landed commit (§2.4, §4.6). Linear's default moves a linked
   issue to Done when its PR merges (https://linear.app/docs/github, read 2026-09-25).
2. **Coverage and drift.** Every criterion shows the issues serving it and the evidence settling it;
   an uncovered criterion and a drifted issue are visible before anyone ships (§3.3).
3. **An issue can prove it shipped.** The release manifest lists issues from immutable git
   trailers under a brief pin the Hub took before the deploy existed (§5.4).
4. **Every automatic move is a person's act.** Fact-driven states are written as the lander or the
   starter, with the fact attached (§2.4). The audit chain names a person, never *"the system"*.
5. **Agents across owners coordinate through the same issues people read.** `panacea team`
   (05 §5.2) gains each agent's issue and the criterion it serves, which is the owner's *"the task
   they have and the reason"* (`owner-decisions.md:116-121`) in human-typed words. A `blocks`
   relation across owners becomes an Orchestrator event when the blocker lands. A conflict thread
   (05 §6.4) cites both issues, so each Orchestrator can explain to its human *why* the other side
   is there. No cross-owner steering is added: each Orchestrator still acts only for its own human.

**Cost per issue** comes nearly free: the gateway attributes usage per session token (OD-19,
05 §3.3 `usage`). Minting the token with the session's issue id gives *"what EP-142 cost"*. That
is a small phase 6 addition, once the gateway is chosen (05 §13 Q1, **UNVERIFIED**).

### 6.3 The one conflict: progress displays

Linear shows cycle and project progress as percentages, graphs and completion predictions
(01 §4, §5.5). `AGENTIC-UX.md` bans *"percent-complete, progress bar, ETA, or 'N of M met'
anywhere in the product"* (`:237`) and *"any badge or count outside the queue"* (`:245`), because a
bar claims *"a denominator that does not exist"* (`:14`).

| Option | What the Work surface shows | Trade-off |
|---|---|---|
| **A (recommended)** | convergence per criterion (§3.3); issue lists grouped by state, with **plain group sizes** (*"Todo 4"*); no %, bar, burn-up, ETA or prediction | a cycle's issue set is a human commitment, so its size is a denominator the system *can* defend; everything that predicts stays banned. Needs a one-line amendment to `AGENTIC-UX.md:245` |
| B | convergence only; no counts anywhere | purest; a cycle review becomes harder, because *"how much did we commit to"* has no number |
| C | Linear's full set | breaks a binding rule for parity's sake; the first prototype's failure mode (`AGENTIC-UX.md:217-218`) |

_(Sections 7–8 follow, written in parts.)_
