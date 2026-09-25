### Task 180: Guided review: a large change grouped by the criterion each change serves, core first, each hunk with its span (L-172)

Stream **WL**, desktop with two small phase 3 deltas, about 3 hours (an estimate, not a
measurement). Depends on task 179 (categories, the token view) and on phase 3 task **P3-19** (the
Merge lens, part 1) sync-merged, with P3-07, P3-16 and P3-18 under it. If their files are not on
the sync base, this task **waits**; it never writes a second copy of a phase 3 file (rule H2).

05 row L-172 (`05-design.md:334`, verified): *"Review grouped by **the criterion each change
serves**, core first; each hunk has its span"*. A reviewer facing a 900-line lane today reads it
path by path, in alphabetical order, in P3-21's Files lens. After this task the lane's review in
the Merge lens can read it **by what each change is for**: the brief's criteria, the lane's own
issue's criteria first, and the changes that serve nothing named last, where they are easy to
question.

**The grouping is deterministic and local.** No gateway call, no model, no similarity: a hunk's
group is read from facts git and the Hub already hold. Each committed hunk has one tier-1 commit
(P3-16: the commit blamed for most of its `+` lines, `task-16.md:164-168` at `94f954d`). That
commit's `Panacea-Issue` trailers (SW.9, task 21) name its issues; each issue's head revision's
`serves` (task 02, `hubapi.Serve{BriefKey, CriterionID}`, `task-02.md:231-235`, verified) names
its criteria. That is the whole chain, and every link of it can be shown on the hunk.

**Two phase 3 deltas the scope did not name, and why they are needed.** The scope listed only
desktop files, but the desktop has neither input today:

1. **A hunk does not carry its commit's issues.** P3-16's tier-1 step already runs
   `log -1 --format=%cI%x00<TrailerFormat(Session)> <commit>` per hunk (`task-16.md:167`). This
   task appends `%x00<TrailerFormat(TrailerIssue)>` to that one format and adds
   `BackLink.Issues []string` (valid values only, through task 21's `gitx.ValidIssueTrailer`,
   sorted, deduplicated, `[]` never `null`). One extra field in a call that already runs; no new
   git call. The alternative, P3-07's lane commit list (`LaneCommit`, `task-18.md:37-41`), is
   capped at `MaxCommits = 50` (P3 `task-07.md:249`), so a large lane, the one this task is for,
   would lose issues past the fiftieth commit.
2. **The desktop does not know the lane's issue.** Task 21 stores `Lane.IssueID` in the lane
   index (`task-21.md:183`), but P3-07's `LaneObservation` (`task-18.md:42-47`) does not carry it.
   This task adds `LaneObservation.IssueID string` (`json:"issueId"`, `""` for a lane not started
   from an issue), copied from the `lanes.Lane` that `Observe` is already given
   (P3 `task-07.md:267`, `lane lanes.Lane`).

Both are additive wire fields (SW.1 permits an added field); both are named in the return as
skeleton gaps.

**One correction to the scope: "not read" is not "serves no criterion".** A hunk whose issue the
replica does not hold (erased, another project, not yet synced) has an issue; we just cannot see
what it serves. Putting it under *Serves no criterion* would state a fact we do not have. So there
is a fourth kind of group, *Issue not read*, before the last. The same honesty rule the Files
lens uses for spans ("never borrows a neighbour's") applies to issues: a trailer-less hunk is
never given the lane's issue, even though land will add that issue to the merge commit (task 21).

**Files** (every phase 3 path is **UNVERIFIED until merge**; the phase 3 part that plans it is
cited):
- `control-plane/internal/changes/diff.go` ⚑ (P3 `task-16.md:19`, `:88-120`) — `BackLink.Issues`,
  the tier-1 format's extra field (modified)
- `control-plane/internal/changes/diff_test.go` ⚑ (P3-16) — two rows (modified)
- `control-plane/internal/converge/observe.go` ⚑ (P3 `task-07.md:21`) — `LaneObservation.IssueID`
  (modified)
- `control-plane/internal/converge/observe_test.go` ⚑ (P3-07) — one row (modified)
- `desktop/src/api/convergence.ts` ⚑ (P3 `task-18.md:15`) — `BackLink.issues`,
  `LaneObservation.issueId` (modified)
- `desktop/src/diff/guidedReview.ts` (+ `guidedReview.test.ts`) — `guide`, the pure grouping (new)
- `desktop/src/diff/laneHunks.ts` (+ `laneHunks.test.ts`) — `loadLaneHunks`, one `getPathDiff`
  per changed path, at most four in flight (new)
- `desktop/src/components/merge/GuidedReview.tsx` (+ `GuidedReview.test.tsx`) — the panel and its
  mount (new; the scope's path, kept)
- `desktop/src/components/lens/merge/QueueList.tsx` ⚑ (P3 `task-19.md:19`, `:140-141`) —
  `LaneCard` gains a *Review* action that opens the panel under the card (modified)
- `desktop/src/components/lens/merge/merge.css` ⚑ (P3 `task-19.md:22`) — `.gr-*` rules (modified)
- `desktop/src/components/work/PlanSurface.tsx` ⚑ (task 45) — the criteria-building half of
  `PlanMount` moves, unchanged in behaviour, into exported `buildPlanCriteria` so both mounts
  build criteria one way (modified)

#### Interfaces — complete

```go
// changes/diff.go (P3-16) — one field added to BackLink, after Sessions. Nothing else changes.
	// Issues are the Panacea-Issue values (SW.9) on Commit, read in the same `log -1` as
	// Sessions: valid ids only (gitx.ValidIssueTrailer), sorted, deduplicated. [] is "this
	// commit names no issue"; always [] at tier 0. Never the lane's own issue as a fallback.
	Issues []string `json:"issues"`

// The tier-1 format, before and after (the only call that changes):
//   log -1 --format=%cI%x00<TrailerFormat(Session)> <commit>
//   log -1 --format=%cI%x00<TrailerFormat(Session)>%x00<TrailerFormat(TrailerIssue)> <commit>
// The third NUL-separated field is split on gitx.ValueSep and filtered by ValidIssueTrailer.

// converge/observe.go (P3-07) — one field added to LaneObservation, after Name.
	// IssueID is lanes.Lane.IssueID (task 21): the issue a Start made this lane for, or "".
	IssueID string `json:"issueId"`
```

```ts
// desktop/src/api/convergence.ts (P3-18) — the two mirrors, one field each.
//   BackLink gains        issues: string[];   // SW.9 ids on back.commit; [] at tier 0
//   LaneObservation gains issueId: string;    // "" when the lane was not started from an issue
```

```ts
// desktop/src/diff/guidedReview.ts — task 180. Pure: no fetch, no store, no clock.
import type { Hunk } from "../api/convergence";
import type { Convergence } from "../components/work/PlanSurface";   // task 45's five words
import type { PlanCriterion } from "../components/work/PlanSurface";
import { CATEGORY_DISPLAY_ORDER, categorize, type Category } from "./categories";

/** The Guided switch defaults on above either bound. Named, and quoted in the switch's tooltip,
 *  so the rule the reader sees is the rule the code runs. Not a speed or quality claim. */
export const GUIDED_MIN_LINES = 400; // "+" and "-" lines across the lane's committed hunks
export const GUIDED_MIN_FILES = 12;  // LaneObservation.changed.length

export function guidedByDefault(changedLines: number, changedFiles: number): boolean;
// → changedLines > GUIDED_MIN_LINES || changedFiles > GUIDED_MIN_FILES

/** One committed hunk of the lane, with the path it belongs to. */
export interface LaneHunk { path: string; hunk: Hunk }

/** What guide() needs about an issue: its alias for the words, and its head revision's serves.
 *  An issue id absent from the map is "not read" (erased, another project, not synced). */
export interface IssueServes { alias: string; serves: { briefKey: string; criterionId: string }[] }

export type GroupKind = "criterion" | "unread" | "none";

export interface GuidedHunk {
  path: string;
  category: Category;
  hunk: Hunk;
  /** The issues the hunk's commit names (hunk.back.issues), as aliases where read. */
  issues: { id: string; alias: string | null }[];
  /** Every other criterion this hunk's issues serve, in the group order; shown as "also serves". */
  alsoServes: PlanCriterion[];
  /** Why the hunk sits in an "unread" or "none" group, in words; "" in a criterion group:
   *  "no Panacea-Issue trailer on commit a41c9e2", "EP-142 serves no criterion",
   *  "issue is-… could not be read". */
  why: string;
}

export interface GuideGroup {
  kind: GroupKind;
  criterion: PlanCriterion | null;   // null for "unread" and "none"
  word: Convergence | null;          // the criterion's convergence word; null for the other kinds
  core: boolean;                     // a criterion the lane's own issue serves
  hunks: GuidedHunk[];               // never empty: a group exists only for the hunks under it
}

export interface GuideResult {
  /** The ordering rule, in words, shown at the top of the panel. */
  rule: string;
  groups: GuideGroup[];
}

/** Brief identity without its revision: "brief:bf-…@5" → "brief:bf-…". A serve pinned to an
 *  older brief revision still groups under the criterion Plan lists, matched by id. */
export function briefIdOf(briefKey: string): string;

export function guide(input: {
  laneIssueId: string;                         // LaneObservation.issueId; "" = no core
  hunks: readonly LaneHunk[];                  // committed only; uncommitted work does not land
  issues: ReadonlyMap<string, IssueServes>;    // from the replica, by issue id
  criteria: readonly PlanCriterion[];          // buildPlanCriteria's list: brief order, drifting appended
  words: ReadonlyMap<string, Convergence>;     // by criterionKey, from task 45's convergenceWord
}): GuideResult;

/** "<brief id> <criterion id>", the key `words` and the grouping use. */
export function criterionKey(c: { briefKey: string; criterionId: string }): string;
```

```ts
// desktop/src/diff/laneHunks.ts — task 180. The only file here that fetches.
import type { Convergence as ConvSnapshot, LaneObservation } from "../api/convergence";
import type { LaneHunk } from "./guidedReview";

export const LANE_HUNK_CONCURRENCY = 4;

export interface LaneHunks {
  hunks: LaneHunk[];                             // the lane's committed hunks, path order as read
  changedLines: number;                          // "+" and "-" lines across `hunks`, measured
  unreadPaths: { path: string; why: string }[];  // a getPathDiff that failed, with the client's sentence
  truncatedPaths: string[];                      // LaneDiff.truncated: P3-16's MaxDiffLines cut this path
}

/** One getPathDiff(projectId, branchId, path) per path in lane.changed, LANE_HUNK_CONCURRENCY at
 *  a time; from each PathDiff it keeps the LaneDiff whose worktreeId is the lane's, and from that
 *  its `committed` hunks only. A binary path contributes no hunk and no unread entry. */
export function loadLaneHunks(
  api: { getPathDiff(projectId: string, branchId: string, path: string): Promise<import("../api/convergence").PathDiff> },
  conv: ConvSnapshot,
  lane: LaneObservation,
): Promise<LaneHunks>;
```

```tsx
// desktop/src/components/merge/GuidedReview.tsx — task 180.
import type { JSX } from "react";
import type { Convergence as ConvSnapshot, LaneObservation } from "../../api/convergence";
import type { GuideResult } from "../../diff/guidedReview";
import type { LaneHunks } from "../../diff/laneHunks";

/** The panel under a LaneCard. `guided` false shows the same hunks flat, in path order, each
 *  with its category chip; true shows `result`'s groups. It never shows a reviewed count. */
export function GuidedReview(props: {
  lane: LaneObservation;
  loaded: LaneHunks;
  result: GuideResult;
  guided: boolean;
  onGuided(on: boolean): void;
  onOpenSession(id: string): void;
  onOpenSpan(sessionId: string, toolUseId: string): void;
  onClose(): void;
}): JSX.Element;

/** The mount: loads the hunks, reads IssueServes for each back.issues id from the Work replica
 *  (SW.16, task 45's `useWork`), builds criteria with buildPlanCriteria and words with task 45's
 *  convergenceWord and servedByCount, then calls guide(). The switch's initial value is
 *  guidedByDefault(loaded.changedLines, lane.changed.length); a flip is kept for this lane in
 *  this session only. */
export function GuidedReviewMount(props: { conv: ConvSnapshot; lane: LaneObservation; onClose(): void }): JSX.Element;

/** The tooltip text, built from the constants: "On by default above 400 changed lines or 12
 *  changed files." */
export function guidedTooltip(): string;
```

```tsx
// desktop/src/components/work/PlanSurface.tsx (task 45) — the extracted half of PlanMount.
// Behaviour is PlanMount's own; the parameters are PlanMount's inputs, **UNVERIFIED** until task
// 45 is built (its names win, and this signature follows them).
export function buildPlanCriteria(
  brief: readonly BriefCriterion[],                               // task 42, brief order
  readings: ReadonlyMap<string, CriterionReading>,                // P4 task 16's criterionStatesByKey
  servedInReplica: readonly { briefKey: string; criterionId: string; text: string }[], // for drifting
): PlanCriterion[];
```

#### The grouping, step by step (`guide`)

1. **Resolve each hunk.** `ids = hunk.back.issues`. No ids → group `none`, why *no
   Panacea-Issue trailer on commit <short sha>*. Any id absent from `issues` → group `unread`, why
   *issue <id> could not be read* (one unread id is enough: we cannot say what the hunk serves).
   Otherwise take the union of every id's `serves`; empty → group `none`, why *<alias> serves no
   criterion*.
2. **Rank the criteria.** `core` = the criterion keys the lane's own issue serves (from
   `issues.get(laneIssueId)`; none when `laneIssueId` is `""` or unread). Rank = `[core ? 0 : 1,
   index in criteria]`. A serve whose key is not in `criteria` gets index `criteria.length` plus
   its first-seen order and renders as *criterion <id>, no longer in the brief* with the word
   `drifting` (task 45's rule for `inBrief: false`).
3. **Place each hunk once.** A resolved hunk goes under its best-ranked criterion; the others
   become `alsoServes`, in rank order. So the multiset of hunks out equals the multiset in.
4. **Order the groups:** criterion groups by rank, then `unread`, then `none`, last.
5. **Order inside a group:** `CATEGORY_DISPLAY_ORDER` index of `categorize(path)` (task 179),
   then path by code point, then `hunk.newStart`.
6. **The rule line.** With a lane issue: *Core first: the criteria <alias> serves, in brief order.
   Then the other criteria this lane's commits serve. Then changes whose issue could not be read,
   then changes that serve no criterion.* Without: *This lane was not started from an issue, so no
   criterion is core.* followed by the same ordering sentences.

#### The panel

- **Where.** `LaneCard` (P3-19) gains one action, *Review*, beside its land controls. It opens
  `GuidedReviewMount` under the card, inline, not a modal: the queue stays visible. Only one lane's
  panel is open at a time; opening another closes the first. Esc closes and returns focus to
  *Review*.
- **The switch.** *Guided* is a toggle in the panel's header with `guidedTooltip()` as its
  `title` and `aria-describedby`. The rule line (step 6) sits under it in sans at `--ink-2`.
- **A group header** is the criterion's text in the reader's words, then a middle dot and its
  convergence word (*settled*, *open*, *contested*, *drifting*, *uncovered*), then the word
  *core* when `core`. `unread` reads *Issue not read*; `none` reads *Serves no criterion*. No
  header carries a hunk count, a line count, a share or a bar. Provenance is shown the way the
  thread is shown elsewhere: the criterion text is a `ThreadNode` link (task 45's
  `PlanCriterionRow` pattern) that opens the criterion in Define.
- **A hunk row** is task 179's `HunkView` (so the token view and the *plain diff* label come
  with it), preceded by the path, its category chip and, when present, *also serves* and the
  other criteria's words; and followed by one provenance line: P3-21's `BackLinks` (commit,
  session, *shell*), then either *transcript span · <toolName> · <HH:MM>*, a button that calls
  `onOpenSpan` when `back.tier === 2`, or the words *no span recorded* when it is not. A hunk
  never shows another hunk's span.
- **Generated paths** collapse inside their group exactly as task 179's `PathDiff` does
  (`collapsedByDefault`); migrations never collapse.
- **Paths that could not be read** are listed under *Issue not read* as one row each, *could not
  be read: <why>*, with no hunks, because we do not know what they contain. A truncated path adds
  *this file's diff was cut at P3's line cap; the rest is not shown* to its first hunk row.
- **Guided off** lists the same `hunks` flat, path by code point then `newStart`, each with its
  category chip and provenance line. The flip changes order, never content.
- **Load.** While `loadLaneHunks` runs, the panel shows the rule line and *Reading the lane's
  changes* driven by the one global heartbeat (DESIGN-SYSTEM: no spinner per row). It is text,
  never a percentage or an ETA.
- **CSS** (`merge.css`, appended): `.gr-panel` uses the card elevation (`--e-card`,
  `--r-card`), `.gr-group-h` is sans 600 at `--ink`, `.gr-core` the word *core* at `--accent-ink`,
  `.gr-why` at `--ink-3`. No new token. No motion except the panel's open, `opacity` and
  `transform` only, at `--dur-fast`.

#### Tests first — each with the mutant it kills

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `TestTierOneReadsIssueTrailers` (`changes/diff_test.go`) | a temp repo; a lane commit trailered `Panacea-Issue: is-…01` and `Panacea-Issue: bogus`; the hunk's `Back.Issues` is `["is-…01"]` | an unfiltered trailer, or the field never read |
| `TestTierZeroHasNoIssues` (`changes/diff_test.go`) | an uncommitted hunk's `Back.Issues` is `[]` and marshals as `[]` | `null` on the wire, or the lane's issue borrowed |
| `TestObserveCarriesLaneIssue` (`converge/observe_test.go`) | a `lanes.Lane{IssueID: "is-…02"}` observed → `LaneObservation.IssueID` is it; a lane without one → `""` | the field dropped between index and wire |
| `groups by the commit's issue` (`guidedReview.test.ts`) | three hunks in two files, all on commits trailered `is-A`, whose serves is criterion C2 → one group, C2, holding all three | grouped by file |
| `the lane's criteria come first` | lane issue serves C3; other hunks serve C1 (brief index 0); groups are `[C3 (core), C1]` | alphabetical, or brief order ignoring core |
| `no trailer is last, and says why` | a hunk with `back.issues: []` is in the last group, `none`, why *no Panacea-Issue trailer on commit a41c9e2* | the hunk dropped, or placed under the lane's issue |
| `unread is not none` | a hunk naming an id absent from `issues` is in `unread`, before `none`; its why names the id | a missing issue reported as serving no criterion |
| `every hunk exactly once` | over a 40-hunk fixture with multi-criterion issues, the multiset of `(path, newStart)` out equals in; a hunk serving C1 and C2 is under C1 with `alsoServes: [C2]` | a hunk shown twice, or lost |
| `an old brief revision still groups` | a serve with `brief:bf-…@3` groups under Plan's `brief:bf-…@5` criterion of the same id | a serve pinned to an older revision orphaned as drifting |
| `a vanished criterion is drifting` | a serve whose id is not in `criteria` makes a group with word `drifting`, after every listed criterion | a guessed text, or the hunk dropped |
| `inside a group, source first` | a group holding a `go.sum`, a `.md`, a `_test.go` and a `.go` hunk orders them `.go`, `_test.go`, `.md`, `go.sum` | an order that drifts from task 179's list |
| `the threshold is the constant` | `guidedByDefault(GUIDED_MIN_LINES, 0)` is false, `(GUIDED_MIN_LINES + 1, 0)` true, `(0, GUIDED_MIN_FILES + 1)` true; `guidedTooltip()` contains both numbers from the constants | a magic number, or a tooltip that drifts from the code |
| `no span is said, never borrowed` (`GuidedReview.test.tsx`) | two adjacent hunks, the first tier 2, the second tier 1: the second's row reads *no span recorded* and has no span button | the nearest span guessed |
| `no review progress` | the rendered panel contains no text matching `/\d+\s*(of|\/)\s*\d+/`, no `progressbar` role, and `expectNoPrediction(container)` passes | a "3 of 7 reviewed" count or a bar |
| `off changes order, not content` | the flat and guided renders contain the same set of hunk rows | a toggle that hides hunks |
| `loadLaneHunks keeps committed only` (`laneHunks.test.ts`) | a fake `getPathDiff` returning committed and uncommitted hunks for two lanes → only this lane's committed ones; a rejected path lands in `unreadPaths` with its sentence; at no point are more than four calls pending | uncommitted work shown as landing; another lane's hunks; an unbounded fan-out |
| `PlanMount unchanged` (`PlanSurface.test.tsx`, task 45's, re-run) | every existing row passes after the extraction | a refactor that changes Plan |

#### Steps

- [ ] **Step 1: Preflight.** Rule H2 for P3-07's `observe.go`, P3-16's `diff.go`, P3-18's
  `convergence.ts` and P3-19's `QueueList.tsx`; task 21's `gitx.TrailerIssue` and
  `ValidIssueTrailer`; task 45's `PlanSurface.tsx`; task 179's `categories.ts`. Any absent → wait.
  Re-read `BackLink`, `LaneObservation` and `LaneCard`'s props; the phase 3 names win.
- [ ] **Step 2: The Go deltas, tests first.** Write the three Go rows. Run
  `go test ./control-plane/internal/changes/... ./control-plane/internal/converge/... -run 'TierOneReadsIssue|TierZeroHasNoIssues|ObserveCarriesLaneIssue' -count=1`.
  Expected: FAIL, `Back.Issues undefined`. Add the field, the format's third field and
  `IssueID`; re-run. Expected: `ok` per package. Then P3's own suites unchanged:
  `go test ./control-plane/internal/changes/... ./control-plane/internal/converge/... -race -count=1`.
  Expected: `ok`.
- [ ] **Step 3: The mirrors.** Add `issues` and `issueId` to `convergence.ts`; `npm run
  typecheck` in `desktop/`. Expected: exit 0, or errors only in fixtures that build a
  `BackLink` or `LaneObservation` literal, each fixed by adding the field.
- [ ] **Step 4: The grouping, tests first.** Write `guidedReview.test.ts`; `npx vitest run
  src/diff/guidedReview.test.ts`. Expected: FAIL, the module is missing. Implement steps 1-6.
  Re-run. Expected: `ok`.
- [ ] **Step 5: The loader, tests first.** Same pattern for `laneHunks.ts`.
- [ ] **Step 6: Extract `buildPlanCriteria`.** Move the code; run task 45's
  `PlanSurface.test.tsx` unchanged. Expected: `ok`, same row count as before.
- [ ] **Step 7: The panel, tests first.** Write `GuidedReview.test.tsx`; expected FAIL. Build the
  panel and the mount; add *Review* to `LaneCard`. Re-run it and P3-19's own tests:
  `npx vitest run src/components/merge src/components/lens/merge`. Expected: `ok`.
- [ ] **Step 8: Acceptance, then commit** (`git add` the new files, then
  `git commit -F <msgfile> -- <paths>`).

#### Acceptance

```bash
go test ./control-plane/internal/changes/... ./control-plane/internal/converge/... -race -count=1
# expected: "ok" per package, no FAIL
cd desktop && npx vitest run src/diff src/components/merge src/components/lens/merge src/components/work/PlanSurface.test.tsx
# expected: the summary reads "Test Files  N passed (N)", no "failed"
npm run typecheck
# expected: exit 0
grep -rn "400\|12" src/components/merge/GuidedReview.tsx | grep -v GUIDED_MIN
# expected: no output (the thresholds appear only as the named constants)
```

#### What this task does not claim

- **Not that a reviewer is faster or better with it.** Nothing is measured about review time
  or defects found; the grouping is an order, not an assessment.
- **Not that a lane's trailers name every issue it served.** P3-05's hook is advisory (task 21's
  own caveat): an agent can skip it, and those hunks sit under *Serves no criterion*, truthfully.
- **Not that the blamed commit is the hunk's only author.** P3-16 takes the commit blamed for
  most `+` lines (ties: newest); a hunk two commits wrote shows one commit's issues.
- **Not that the grouping is complete for a truncated path.** P3-16's line cap cuts the diff;
  the panel says so on that path and shows what it has.
- **Not a review record.** Nothing is written: no *reviewed* mark, no approval, no Hub call.
- **Not that phase 3's files look as cited.** Every phase 3 path and line here is from the plan
  at `94f954d`, **UNVERIFIED until merge**; task 45's names are **UNVERIFIED** until it is built.
- Design: 05 §1 L (rows L-170, L-172; `05-design.md:334`); §2.3 (the land on the thread); §3.3
  (the five convergence words, `05-design.md:762`); AGENTIC-UX §8.5 per the scope
  (**UNVERIFIED** here: the section number was not re-read). Parity: closes **L-172**.

**Interfaces:**
- Produces:
  - Go: `changes.BackLink.Issues []string`; `converge.LaneObservation.IssueID string`
  - TS: `BackLink.issues`, `LaneObservation.issueId` in `api/convergence.ts`
  - `diff/guidedReview.ts` — `guide`, `GuideResult`, `GuideGroup`, `GuidedHunk`, `GroupKind`,
    `LaneHunk`, `IssueServes`, `criterionKey`, `briefIdOf`, `GUIDED_MIN_LINES`,
    `GUIDED_MIN_FILES`, `guidedByDefault`
  - `diff/laneHunks.ts` — `loadLaneHunks`, `LaneHunks`, `LANE_HUNK_CONCURRENCY`
  - `components/merge/GuidedReview.tsx` — `GuidedReview`, `GuidedReviewMount`, `guidedTooltip`
  - `components/work/PlanSurface.tsx` — `buildPlanCriteria` (extracted)
- Consumes:
  - task 179 — `categorize`, `CATEGORY_DISPLAY_ORDER`, `collapsedByDefault`, `HunkView`'s token view
  - task 45 — `Convergence`, `PlanCriterion`, `convergenceWord`, `servedByCount`, `useWork`
  - task 21 — `gitx.TrailerIssue`, `gitx.ValidIssueTrailer`, `lanes.Lane.IssueID`
  - task 20 — `expectNoPrediction`
  - P3-07, P3-16, P3-18, P3-19, P3-21 — `Observe`, the tier-1 read, the wire types, `LaneCard`,
    `BackLinks`, `SpanDrawer`
- Amends: nothing in the Hub. Two additive phase 3 wire fields (SW.1's added-field rule).
