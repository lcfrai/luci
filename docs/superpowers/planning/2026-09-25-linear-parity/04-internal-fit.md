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

_(Sections 2–8 follow, written in parts.)_
