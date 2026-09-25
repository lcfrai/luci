### Task 45: Plan (`G P`): Triage, Backlog, criteria with their issues, *My issues*

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 42 (`TriageView`,
embedded as Plan's first section) and, through it, tasks 40 and 41 (the row parts, the hold model).
Uses SW.12, SW.13 (`ViewHeader`), SW.14 (`ThreadLine`, `ThreadNode`), SW.15 and SW.16 (`useWork`).
Touches `desktop/src/components/AppShell.tsx` ⚑ after task 40 and before task 62 (the file map's
order). No Hub route is added.

Parity: closes **L-112**'s view (*My issues*; the ranking of what needs the reader is the Desk,
tasks 62-65, 05 §4.2) and **L-46**'s second home (Triage inside Plan); serves **L-101**. Design:
05 §4.1's surface table (`05-design.md:873`, the Plan row), §4.5 (Discover marks an unserved
criterion), §3.3 (the five convergence words, `05-design.md:760-763`), §4.2.

**What the reader sees.** One scrolling surface headed *Plan* (the one `ViewHeader`, with the
project's route badge and a *Work* action back), with **exactly four sections, in this order**:

| Section | What it lists | Grouped by |
|---|---|---|
| *Triage* | task 42's `TriageView`, embedded, its keys intact | — (Triage's own order) |
| *Backlog* | issues in `backlog` | criterion, brief order, *Serves no criterion* last |
| *Criteria* | every criterion of the project's briefs, each with its convergence word, a plain count and the issues that serve it, under its `ThreadLine` | the criterion itself |
| *My issues* | issues whose **assignee** is the reader | criterion |

The features board, cycles, initiatives and saved views are **not drawn at all**, not even as an
empty heading: 05 §4.1 shows a noun only once a team has one, and W2 and W4 bring them.

**Decisions made here, so nobody re-derives them:**

1. **Plan is a Work surface, not a seventh lens tab.** 05 says `Lens` gains `"work"` in code but
   Work is not a tab (`05-design.md:874`). Plan rides the same lens: task 37's `workSlice` gains
   `surface: "work" | "plan"` (task 62 adds `"desk"`), and `AppShell`'s `LensRouter` arm for
   `"work"` renders `surface === "plan" ? <PlanMount/> : <WorkLens/>`. **UNVERIFIED** until task
   40's part fixes how it mounts Work; if task 40 chose another mechanism, Plan follows it and this
   decision's only claim is *not a tab*. In W1, Plan is reached from a *Plan* action in the Work
   lens's `ViewHeader` (SW.13); `G P` is task 54's and the palette verb task 59's.
2. **The convergence word is read, never derived from issue states** (05 §3.3: *done* and
   *settled* are two facts). `settled`, `open` and `contested` come from phase 4's criterion reading
   (`CriterionReading {id, state, because}`, P4 task 15, `phase-4-discover-parts/task-15.md:157`,
   loaded by the planned `desktop/src/provenance/loader.ts`, P4 task 16; **UNVERIFIED until
   merge**). The two words phase 4 does not produce are facts the work layer holds: `uncovered`
   is *no issue serves it* (a `serves` fact, not a state), and `drifting` is *an issue serves a
   criterion id the brief no longer has* (05:754-756: an edited criterion gets a new id). Order:
   `drifting` for an id not in the brief; else the reading's `settled` or `contested`; else
   `uncovered` when nothing serves it; else `open`. No issue's state enters the function.
3. **A drifting criterion is still listed**, after the brief's criteria, with its old text from
   the issue revision that serves it and the word `drifting`. Its one-key *re-point* is the Desk's
   (05:755; task 63), not Plan's.
4. **The reader is `me: User`, from `GET /v1/me`** (the Hub's route, relayed; `hub-contract.md:653`
   in the Hub worktree), the same as task 46's decision 4. The scope names `/v1/auth/status`, but
   its `account` is `{tenantId, name, username}` (`hub-contract.md:1370`), with no `u-` id to
   compare with `assignee.id`. A gap, named in the return.
5. ***My issues* is the assignee filter and nothing else.** Not issues whose running agent the
   reader owns, not issues the reader created, not subscriptions. Those are the Desk's and the
   inbox's (05 §4.2); a wider filter would make *My issues* a second, worse Desk.
6. **Counts are plain and every section holds.** A group header reads `3 issues`; a criterion reads
   `open · 3 issues` (05's own form, `05-design.md:885`, without the `· 1 moving` until one is).
   Each section owns one task 41 `HoldState`, keyed by section, so a Triage accept does not move a
   row in *Backlog* under the reader until attention leaves it.
7. **Plan draws the criteria it is given.** `PlanSurface` takes `criteria` as a prop: the brief's
   criteria in brief order, each with its reading. A thin `PlanMount` builds that prop from the
   phase 4 loader and passes `me`; tests render `PlanSurface` directly, so they need no phase 4.

**Files** (the skeleton's file map lists `PlanSurface.tsx` and `AppShell.tsx` ⚑ for this task):
- `desktop/src/components/work/PlanSurface.tsx` (+ `PlanSurface.test.tsx`) — the surface, `PlanMount`, `convergenceWord` (new)
- `desktop/src/components/AppShell.tsx` ⚑ — the `"work"` arm renders Plan when `surface === "plan"` (modified)
- `desktop/src/store/slices/workSlice.ts` ⚑ (task 37) — `surface` and `setSurface` (modified; not in the file map for 45, a gap)
- `desktop/src/components/work/WorkLens.tsx` — the *Plan* action in its `ViewHeader` (modified, task 40's)
- `desktop/src/components/work/work.css` — `.wk-plan`, `.wk-section`, `.wk-conv` (modified)

