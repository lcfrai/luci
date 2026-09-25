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

#### Interfaces — complete: `desktop/src/components/work/PlanSurface.tsx`, the exported half

```tsx
// desktop/src/components/work/PlanSurface.tsx — task 45. The project surface (05 §4.1, Plan row).
import type { JSX } from "react";
import type { User } from "../../api/work";                              // SW.15
import type { CriterionReading } from "../../api/discover";             // P4 task 15; UNVERIFIED until merge
import type { BriefCriterion } from "../../work/suggestCriterion";      // task 42
import type { Route } from "../common/ViewHeader";                      // SW.13

/** 05 §3.3's five words for a criterion, kept from 04 §3.3. Never a count, never a share. */
export type Convergence = "settled" | "open" | "contested" | "drifting" | "uncovered";

/** A criterion as Plan lists it: the brief's own, in brief order, with phase 4's reading. */
export interface PlanCriterion extends BriefCriterion {
  reading: CriterionReading | null;   // null when phase 4 has not read it yet
  inBrief: boolean;                   // false for a served id the brief no longer has (drifting)
}

/** Decision 2. The inputs are the reading and two serves facts; no issue state is an input. */
export function convergenceWord(c: { reading: CriterionReading | null; inBrief: boolean }, servedBy: number): Convergence {
  if (!c.inBrief) return "drifting";
  const s = c.reading?.state;
  if (s === "settled" || s === "contested") return s;
  return servedBy === 0 ? "uncovered" : "open";
}

/** The four sections, in order (the test pins this list and its order). */
export const PLAN_SECTIONS = ["Triage", "Backlog", "Criteria", "My issues"] as const;

export function PlanSurface(props: {
  projectId: string;
  me: User;                               // GET /v1/me, decision 4
  route: Route | null;                    // the project's, for the ViewHeader badge
  criteria: readonly PlanCriterion[];     // brief order; drifting ids appended after
  timeZone: string;                       // passed to the embedded TriageView
  now(): number;
  onBack(): void;                         // the ViewHeader's Work action
}): JSX.Element;

/** The mount: builds `criteria` from the phase 4 loader's criterionStatesByKey (P4 task 16) and
 *  the served ids in the replica, and `me` from GET /v1/me. Renders PlanSurface. */
export function PlanMount(): JSX.Element;
```

`PlanMount` appends a `drifting` entry for every `serves` criterion id the replica's issue
revisions carry that is not among the brief's criteria (its `text` from the newest revision that
names it, `reading: null`, `inBrief: false`). It holds no state of its own and writes nothing.

#### How each section is built, inside `PlanSurface`

```tsx
const backlog = useWork((r) => groupByCriterion(issuesOf(r, projectId).filter((h) => h.state === "backlog"), criteria));
const mine    = useWork((r) => groupByCriterion(issuesOf(r, projectId).filter((h) => h.assignee?.id === me.id), criteria));
const serving = useWork((r) => servedByCount(r, projectId));       // Map<criterionKey, issue ids>, from serves only
// …
<section className="wk-section" aria-labelledby="plan-triage"><h2 id="plan-triage">Triage</h2>
  <TriageView projectId={projectId} timeZone={timeZone} now={now} /></section>
<section className="wk-section" aria-labelledby="plan-backlog"><h2 id="plan-backlog">Backlog</h2>
  <HeldGroups section="backlog" grouped={backlog} /></section>
<section className="wk-section" aria-labelledby="plan-criteria"><h2 id="plan-criteria">Criteria</h2>
  {criteria.map((c) => <PlanCriterionRow key={`${c.briefKey}#${c.criterionId}`} c={c}
     word={convergenceWord(c, serving.get(key(c))?.length ?? 0)} issueIds={serving.get(key(c)) ?? []} />)}</section>
<section className="wk-section" aria-labelledby="plan-mine"><h2 id="plan-mine">My issues</h2>
  <HeldGroups section="mine" grouped={mine} /></section>
```

`issuesOf`, `groupByCriterion` and `servedByCount` are selectors over SW.16's replica: the first
two are task 40's (the Work lens groups the same way; **UNVERIFIED** names, task 40 wins), and
`servedByCount` is this task's, reading only each head revision's `serves`. `HeldGroups` is the
Work lens's group list over one task 41 `HoldState` (task 40's `CriterionGroup` in a loop).
`PlanCriterionRow` renders `ThreadLine` > the criterion's `ThreadNode` and text in sans, then
`<span className="wk-conv" data-word={word}>{word}</span> · <span className="wk-count">{n} issue(s)</span>`,
then its issues as task 40's rows. An `uncovered` criterion keeps its row and reads
*No issue serves this criterion. C files one.* (task 43's `EMPTY_CRITERION`, one string).

#### `work.css`, appended

```css
/* task 45 — Plan. Section headings are the one type step above a row; no card grid. */
.wk-plan { display: flex; flex-direction: column; gap: var(--s-5); padding: var(--s-4) var(--s-5); }
.wk-section > h2 { font-size: var(--t-ui); font-weight: 600; color: var(--ink-2); margin: 0 0 var(--s-2); }
.wk-conv { font-size: var(--t-ui); color: var(--ink-2); }
.wk-conv[data-word="uncovered"], .wk-conv[data-word="drifting"], .wk-conv[data-word="contested"] { color: var(--ink); }
```

The word carries the meaning; the ink only lifts the three that ask for attention. No colour, bar
or fill encodes convergence (02 C13: nothing fills as work progresses).

#### Tests to write first — `desktop/src/components/work/PlanSurface.test.tsx`

`PlanSurface` renders on `workScenario()` through task 40's test seam (**UNVERIFIED** name), with
`me = DYLAN` and `criteria` built from the scenario's two criteria plus a third no issue serves.
`convergenceWord` is also tested bare.

| Test | What it asserts | Mutant it kills |
|---|---|---|
| `the four sections render in order, and nothing else` | the `h2`s read exactly `PLAN_SECTIONS`, in order; no heading or text matches `/feature|cycle|initiative|saved view|favourite/i` | empty W2 placeholders |
| `a criterion no issue serves reads uncovered and stays listed` | the third criterion's row is present, reads `uncovered · 0 issues` and the empty-criterion sentence | hiding an uncovered criterion |
| `done issues do not make a criterion settled` | three issues serving `c-91e2a4b0`, all `done` with `proof: "proven"`; its reading is `{state: "open", because: "evidence does not hold at the head"}`; the row reads `open`; and `convergenceWord({reading: open, inBrief: true}, 3)` is `open` | deriving `settled` from issue states |
| `settled and contested come from the reading` | a reading of `settled` with zero serving issues reads `settled`; `contested` reads `contested` | an `uncovered` that overrides the reading |
| `a served id the brief no longer has reads drifting` | EP-164 serves `c-deadbeef`, not among the brief's criteria; a `drifting` row is listed after the brief's own, with EP-164 under it | a drifted serve that silently vanishes |
| `My issues is the assignee filter only` | EP-163 is Alice's with Dylan's bound session in the fixture's agent lines, and Dylan subscribes to EP-150 (assignee Alice); *My issues* lists every issue with `assignee.id === DYLAN.id` and neither EP-163 nor EP-150 | a filter on agents owned or subscriptions |
| `every group size is a plain count` | every `.wk-count` matches `/^\d+ issues?( · \d+ moving)?$/`; `expectNoPrediction(container)` passes | a coverage percentage, or `2 of 3 criteria` |
| `Triage's keys still work inside Plan` | a `1` on the embedded Triage row sends one `setState` (task 42's op); a `1` on a *Backlog* row sends none | a Plan that swallows or widens the Triage keys |

#### Steps

- [ ] Write `PlanSurface.test.tsx`; it fails to import.
- [ ] Write the exported half (`Convergence`, `PlanCriterion`, `convergenceWord`, `PLAN_SECTIONS`),
      then `PlanSurface` (the four sections, one `HoldState` per held section), then `PlanMount`.
- [ ] Add `surface` and `setSurface` to `workSlice.ts` (task 37's file, not persisted); make
      `AppShell`'s `"work"` arm render `PlanMount` when `surface === "plan"`; add the *Plan*
      action to `WorkLens`'s `ViewHeader` and the *Work* action to Plan's.
- [ ] Mutate by hand and watch each named test go red, then revert: add a *Cycles* `h2` (1); skip
      criteria with no issues (2); return `settled` when every serving issue is `done` (3); test
      `servedBy === 0` before the reading (4); drop the appended drifting entries (5); widen *My
      issues* to `createdBy` (6); print `n / total` (7).
- [ ] Append the CSS. Run the suite and the typecheck; commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/components/work/PlanSurface.test.tsx'
# expected: 1 file, 8 tests, all passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, the AppShell and store suites included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -n "state === \"done\"\|\.state ===" desktop/src/components/work/PlanSurface.tsx'
# expected: exactly one line, the Backlog filter (h.state === "backlog"); none in convergenceWord
```

#### What this task does NOT claim

- **The features board** (L-67 onward), **cycles** (L-57 onward), **initiatives**, **saved views**
  and **favourites**: W2 and W4. Their absence is tested, not stubbed.
- **The ranking** of what needs the reader: the Desk, tasks 62-65. *My issues* is a filter.
- **`G P`** (task 54) and the palette's *Plan* verb (task 59).
- **The drifting criterion's re-point**: the Desk's one key, task 63.
- **That phase 4's readings are right or current**: Plan shows what the loader holds; the loader's
  freshness is phase 4's (tasks 16-17), **UNVERIFIED until merge**.

#### Gaps found while writing this part

1. The scope's `/v1/auth/status` account has no `u-` id (`hub-contract.md:1370`); *My issues* needs
   `GET /v1/me` (`hub-contract.md:653`), as task 46 decided. Where the mount gets `me` is task 40's.
2. `workSlice.surface` is this part's proposal; task 37 must carry it (task 62 adds `"desk"`).
3. Phase 4's `CriterionState` is `open | settled | contested` only (`phase-4 task-06.md:79-84`,
   `task-15.md:93`); `drifting` and `uncovered` are computed here from `serves`, which the scope's
   *read, never computed* sentence does not separate. Both use no issue state.
4. The skeleton's row gives 45 `dependsOn: 42`; it also uses task 43's `EMPTY_CRITERION` and task
   41's hold model directly.

