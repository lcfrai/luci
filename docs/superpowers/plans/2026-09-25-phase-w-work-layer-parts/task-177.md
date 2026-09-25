### Task 177: dashboards: a secondary surface under Work, never the landing page; measured counts only (L-152)

Stream **WL**, mostly desktop, about 3 hours (an estimate, not a measurement). Depends on task
**132** (the insights panel and scatter) and task **174**; through them on task 122 (the insights
query, `task-122.md`), task 114 (its wire, `task-114.md:292-330`), task 80 (views `vw-` and the one
Go filter evaluator), tasks 83 and 91 (the desktop's `View`, `ViewFilter`, favourites) and task 176
(which opens `NNNN_work_later.sql`). 05 L-152 (`05-design.md:309`, verified 2026-09-26):
*"Dashboards · MATCH · A secondary surface under Work, never the landing page; the Desk stays
primary (AX §5) (Q1) · later"*. AX §5 (`docs/AGENTIC-UX.md:87`): the Desk *"is the default landing
surface, always."* Parts 80, 83, 91 and 132 are **not written** on 2026-09-26; names from them come
from their scopes and are **UNVERIFIED** until they land.

A dashboard is a page of task 122's measured numbers, kept, named and shared like a view. It adds
**no new measure** and **no new chart**: every block is one of task 122's queries drawn by one of
task 132's components.

**Decisions made here, so nobody re-derives them:**

1. **A dashboard is a view.** A shared dashboard is a Hub view `vw-` (task 80) with two added
   fields, `kind: "dashboard"` and `blocks`, so it inherits a view's team scope, favourites,
   archive, https link and events, and needs no new prefix or aggregate. An ordinary view is
   `kind: "list"`. A **personal** dashboard is a desktop preference, as a personal view is (tasks
   80 and 91: *"a personal view is never sent to the Hub"*); it holds only block definitions, never a
   number or a Hub record. The added fields are an added-field change to SW.18, not a reopening
   (task 05's rule).
2. **Four block types, each a task 122 query drawn by task 132.** `count` (one measured count,
   `measure: "count"`, no group), `table` (counts per group, `measure: "count"` with a `groupBy`,
   task 132's plain table, no bars), `duration` (one duration measure's p50, p75 and p90 as task
   132 prints them) and `scatter` (task 132's `InsightsScatter`). The picker offers exactly task
   114's four measures (`MeasureCount`, `MeasureCycleTime`, `MeasureLeadTime`,
   `MeasureTriageTime`) and exactly `GroupBy`'s keys. There is no free-form metric, no percentage,
   burn-up, forecast, completion rate, target or "N of M", and no block type that could hold one.
3. **A window is relative, and the Hub still bounds it.** A block stores `windowDays` (1 to 366,
   the bound task 122 enforces, decision 6 there); the desktop computes `from = now − windowDays`
   and `to = now` at render and sends an ordinary `InsightsQuery`. Nothing is cached: a dashboard
   opened tomorrow asks again.
4. **Each block says its rule in words, generated, not typed.** `ruleWords(block)` writes, from
   task 122's own definitions, *"lead time: created → done, issues done in the last 30 days"*,
   *"cycle time: first started → done"*, *"triage time: created → first move out of triage,
   issues created in triage"*, *"count: issues done"*. The person's `title` is optional and shown
   above; the rule line is always shown and cannot be edited, so a number is never shown without
   what it measured.
5. **Visibility is the Hub's, per block.** A block queries one team; task 122 answers 404
   `team_not_found` to anyone not a member (its decision 5). A block the viewer cannot read renders
   *"Not visible to you"* with **no number**, never a zero and never a total. A **team** dashboard's
   blocks are all over its own team (the Hub refuses another `teamId`), so every member sees the
   same numbers; only a personal dashboard can span teams, and then only teams the person reads.
6. **Filters narrow on the Hub, through task 80's one evaluator.** A block's `filter` (SW.18's
   `ViewFilter`) is sent as an added `filter` field on `InsightsQuery`, and task 122's route narrows
   the window population with task 80's Go evaluator (`store/work_filter.go`) before
   `ComputeInsights`. The client never filters points and never recomputes a percentile (task 132's
   rule).
7. **Place: under Work, never home.** Dashboards open from the Work lens's `ViewHeader` actions
   (SW.13) and from Plan's secondary entries, beside *Insights* (task 132). There is **no
   *Dashboards* node on the rail**, no Desk section, and **no *set as home***: the start route is the
   Desk whatever is favourited. A favourited dashboard appears in task 91's Favourites group as a
   **name only**, like any favourited view (task 91: *"names only, no counts"*), and opens under
   Work; the rail never shows a number from it.

**Files:**
- `control-plane/internal/hub/hubapi/work_plan.go` ⚑ (task 72's SW.18 file; its real name is task
  72's) — `View.Kind`, `View.Blocks`, `DashboardBlock`, `ValidateBlocks` (additive)
- `control-plane/internal/hub/hubapi/work_roadmap.go` ⚑ (task 114, `task-114.md:289`) —
  `InsightsQuery.Filter` (additive to SW.20)
- `control-plane/internal/hub/hubapi/work_dashboard_test.go` — the block rules, pure (new)
- `control-plane/internal/hub/migrate/sql/NNNN_work_later.sql` ⚑ — task 177's section, after 176's
- `control-plane/internal/hub/server/work_views.go` ⚑ (task 80) — `ValidateBlocks` on create and
  patch; `server/work_insights.go` ⚑ (task 122) — the filter narrowing
- `control-plane/internal/hub/server/work_dashboard_integration_test.go` (new, `-tags integration`)
- `desktop/src/api/work.ts` ⚑ — the block type and the query's `filter` (task 125's W4 types)
- `desktop/src/components/roadmap/{Dashboards.tsx, DashboardView.tsx, DashboardBlock.tsx}` (+
  tests) and `ruleWords.ts` (+ test), beside task 132's insights files (new)
- ⚑ task 31's `ViewHeader` actions in the Work lens and task 45's `PlanSurface.tsx` — one
  *Dashboards* entry each

#### `hubapi`, the additions (complete)

```go
// ViewKind: an ordinary view lists issues; a dashboard shows measured blocks.
type ViewKind string

const (
	ViewList      ViewKind = "list"
	ViewDashboard ViewKind = "dashboard"
)

// BlockType is one of four ways to draw a task 122 query. There is no type for a
// percentage, burn-up, forecast, target, completion rate or "N of M" (OD-27).
type BlockType string

const (
	BlockCount    BlockType = "count"    // measure count, no group: one number
	BlockTable    BlockType = "table"    // measure count with a groupBy: group names and counts
	BlockDuration BlockType = "duration" // a duration measure: p50, p75, p90 as the Hub gives them
	BlockScatter  BlockType = "scatter"  // a duration measure: task 132's scatter
)

// DashboardBlock is stored on a dashboard view. Query holds no from or to: the
// window is relative, and the desktop computes it at render (decision 3).
type DashboardBlock struct {
	Type       BlockType    `json:"type"`
	TeamID     string       `json:"teamId"`
	ProjectID  *string      `json:"projectId"`
	Measure    string       `json:"measure"` // MeasureCount | MeasureCycleTime | MeasureLeadTime | MeasureTriageTime
	GroupBy    *string      `json:"groupBy"` // table only; one of InsightsQuery.GroupBy's keys
	WindowDays int          `json:"windowDays"` // 1..366
	Filter     *FilterGroup `json:"filter"` // SW.18's filter; nil for none
	Title      string       `json:"title"` // optional, 0..80 runes; the rule line is generated
}

// View gains two fields (the rest is task 72's):
//	Kind   ViewKind         `json:"kind"`   // "list" when absent on the wire
//	Blocks []DashboardBlock `json:"blocks"` // [] for a list view; 1..12 for a dashboard

// InsightsQuery gains one field (the rest is task 114's):
//	Filter *FilterGroup `json:"filter"` // narrows the window population, task 80's evaluator

const MaxDashboardBlocks = 12

// ValidateBlocks returns the first bad field path, or "". viewTeamID is the
// dashboard's own team: a team dashboard's blocks query only that team.
func ValidateBlocks(kind ViewKind, blocks []DashboardBlock, viewTeamID string) string {
	if kind == ViewList {
		if len(blocks) != 0 {
			return "blocks"
		}
		return ""
	}
	if kind != ViewDashboard {
		return "kind"
	}
	if len(blocks) == 0 || len(blocks) > MaxDashboardBlocks {
		return "blocks"
	}
	for i, b := range blocks {
		p := func(f string) string { return fmt.Sprintf("blocks[%d].%s", i, f) }
		duration := b.Measure == MeasureCycleTime || b.Measure == MeasureLeadTime || b.Measure == MeasureTriageTime
		switch b.Type {
		case BlockCount, BlockTable:
			if b.Measure != MeasureCount {
				return p("measure")
			}
		case BlockDuration, BlockScatter:
			if !duration {
				return p("measure")
			}
		default:
			return p("type")
		}
		if (b.Type == BlockTable) != (b.GroupBy != nil) || (b.GroupBy != nil && !ValidGroupBy(*b.GroupBy)) {
			return p("groupBy")
		}
		if b.WindowDays < 1 || b.WindowDays > 366 {
			return p("windowDays")
		}
		if b.TeamID != viewTeamID {
			return p("teamId")
		}
		if utf8.RuneCountInString(b.Title) > 80 {
			return p("title")
		}
	}
	return ""
}
```

`ValidGroupBy` is the set of `InsightsQuery.GroupBy`'s keys as task 114 declares them (five at
`task-114.md:307`, seven once task 122's gap G1 lands); if 114 has no such function, this task adds
it beside the constants, reading the same list. `FilterGroup` is SW.18's name for the filter
(`task-80` scope: *"the one Go evaluator of `FilterGroup`"*); a different name is a find-and-replace.

#### The migration section (appended to `NNNN_work_later.sql` after task 176's, complete)

```sql
-- ── task 177: dashboards are views (05 L-152). Two columns on task 80's views table. ──
ALTER TABLE views ADD COLUMN kind text NOT NULL DEFAULT 'list';
ALTER TABLE views ADD CONSTRAINT views_kind_known CHECK (kind IN ('list', 'dashboard'));
ALTER TABLE views ADD COLUMN blocks jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE views ADD CONSTRAINT views_blocks_array CHECK (jsonb_typeof(blocks) = 'array');
ALTER TABLE views ADD CONSTRAINT views_blocks_only_on_dashboards
  CHECK (kind = 'dashboard' OR blocks = '[]'::jsonb);
GRANT UPDATE (blocks) ON views TO hub_app;
```

`kind` is set at create and never changes (no `UPDATE` grant): a list view does not become a
dashboard, which keeps task 91's list surfaces from meeting blocks. The table name `views` is task
80's scope's; the real name is task 80's part's. The existing rows default to `list`, so the
migration changes no view anyone has. Task 06's grants test gains `views.blocks` (the table is task
80's, so the list entry goes in whichever test task 80's part extended).

#### The route deltas

- **Views (task 80's `server/work_views.go`).** `POST /v1/teams/{teamId}/views` and `PATCH` run
  `ValidateBlocks(kind, blocks, teamId)` after decode; a path → 422 `bad_request` with that `field`.
  `view.created` and `view.updated` gain `data.kind` and, on a patch, `fields: "blocks"`; `data`
  never holds a block's title or filter (metadata only, `hub-contract:1132-1133`).
- **Insights (task 122's `server/work_insights.go`).** After `WindowIssues` and before
  `IssueFactsOf`, a non-nil `filter` keeps only the ids task 80's evaluator matches over the same
  readable rows; `groups` and `points` are then computed over what is left. A filter naming a label
  or project outside the team's reach matches nothing (it never widens the population).
- **The matrix rows** for both routes do not change: who may call them is unchanged.

#### Interfaces — complete: `desktop/src/components/roadmap/ruleWords.ts`

```ts
import type { DashboardBlock } from "../../api/work";

const MEASURE: Record<DashboardBlock["measure"], string> = {
  count: "count: issues done",
  cycle_time: "cycle time: first started → done",
  lead_time: "lead time: created → done",
  triage_time: "triage time: created → first move out of triage, issues created in triage",
};

/** The rule a block measures, in words, from task 122's definitions. Never typed by a person. */
export function ruleWords(b: Pick<DashboardBlock, "measure" | "groupBy" | "windowDays">): string {
  const days = b.windowDays === 1 ? "the last day" : `the last ${b.windowDays} days`;
  const where = b.measure === "triage_time" ? `left triage in ${days}` : `done in ${days}`;
  const by = b.groupBy ? `, by ${b.groupBy}` : "";
  return `${MEASURE[b.measure]}; issues ${where}${by}`;
}
```

For `triage_time` the population is issues whose first move out of triage falls in the window
(task 122, decision 1), so the words say *left triage*, not *done*.

#### Interfaces — complete: the block and the surfaces

```tsx
// desktop/src/components/roadmap/DashboardBlock.tsx
import type { DashboardBlock as Block, InsightsResult } from "../../api/work";

export type BlockState =
  | { state: "loading" }
  | { state: "ready"; result: InsightsResult }
  | { state: "not_visible" } // the Hub's 404: no number is drawn, never a zero
  | { state: "refused"; field: string }; // a 422 from the Hub, its field named

export type DashboardBlockProps = {
  block: Block;
  data: BlockState;
  /** Opens the issue list behind a number: the ids of that group, or all the points. */
  openIssues: (ids: string[], title: string) => void;
  openIssue: (id: string) => void;
};
export function DashboardBlock(props: DashboardBlockProps): JSX.Element;

// desktop/src/components/roadmap/DashboardView.tsx
export type DashboardViewProps = {
  view: { id: string | null; name: string; blocks: Block[] }; // id null: a personal dashboard
  query: (b: Block, now: Date) => Promise<BlockState>;
  now: () => Date;
  openIssues: (ids: string[], title: string) => void;
  openIssue: (id: string) => void;
};
export function DashboardView(props: DashboardViewProps): JSX.Element;

// desktop/src/components/roadmap/Dashboards.tsx: the list, the editor, the block picker
export const BLOCK_TYPES = ["count", "table", "duration", "scatter"] as const;
export type DashboardsProps = {
  teamId: string;
  shared: { id: string; name: string; blocks: Block[] }[];
  personal: { name: string; blocks: Block[] }[];
  save: (d: { id: string | null; name: string; blocks: Block[]; personal: boolean }) => Promise<void>;
  open: (d: { id: string | null; name: string; blocks: Block[] }) => void;
};
export function Dashboards(props: DashboardsProps): JSX.Element;
```

`DashboardBlock` draws `count` as one plain number with its rule line, `table` as task 132's group
table, `duration` as three labelled values (*"p50 5h 10m · p75 8h · p90 9h"*, the Hub's, in
`tabular-nums`) and `scatter` by rendering task 132's `InsightsScatter` with the block's result.
Every number is a button: a count opens the issues it counted, a group row that group's issues, a
duration its points' issues, a scatter mark its issue (AX Principle 7). Task 114's result carries
**no ids for a count** (`Points` is empty for `count`, `task-114.md:326`), so a `count` or `table`
number opens the **Work lens filtered the same way**: the block's team, project, filter and window
(and, for a table row, its group key) as an ad hoc list (task 91's `AdHocList`), which lists the
population the Hub counted, read through the same row-level security. Where the two reads could
differ (an issue closed between them), the list's header says *"as of 14:02"* with the query's time,
and the list is the truth.

#### Tests to write first, part 1 — `hubapi/work_dashboard_test.go` (pure)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAListViewHoldsNoBlocks` | `kind: list` with one block → `blocks` | a list view that quietly becomes a dashboard |
| `TestEveryBlockTypeMatchesItsMeasure` | `count` with `lead_time` → `blocks[0].measure`; `duration` with `count` → the same | a block that draws a count as a duration |
| `TestOnlyTheFourTypesExist` | `type: "burnup"`, `"percent"`, `"forecast"` → `blocks[0].type` each | a burn-up, percentage or forecast block |
| `TestATableNeedsAGroupAndOthersHaveNone` | `table` with no `groupBy` → `groupBy`; `count` with one → `groupBy` | a table of one row; a count split silently |
| `TestTheWindowIsBoundedAsTheHubBoundsIt` | `windowDays` 0 and 367 → `windowDays`; 366 → `""` | a window the insights route will refuse at render |
| `TestATeamDashboardQueriesOnlyItsTeam` | a block with another `teamId` → `blocks[0].teamId` | a team dashboard that shows another team's numbers |

#### Tests to write first, part 2 — `server/work_dashboard_integration_test.go` (`-tags integration`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestADashboardIsAViewWithBlocks` | `POST …/views {kind: dashboard, blocks: [count]}` → 201; `GET` returns both fields; one `view.created` with `data.kind`; no title in `data` | a new aggregate; block text on the feed |
| `TestAViewsKindNeverChanges` | `PATCH {kind: list}` on a dashboard → 400 unknown or immutable field | a list view meeting blocks |
| `TestAFilterNarrowsTheCount` | ten issues done, four labelled `Bug`; `filter: label = Bug` → `groups` total 4; without → 10 | a filter ignored by the Hub, then applied by the client |
| `TestANonMembersBlockGetsNoNumber` | Linus (not in Portal) queries a Portal block → 404 `team_not_found`, no body number | a server total that leaks a team's count |

#### Tests to write first, part 3 — desktop (`roadmap/Dashboard*.test.tsx`, `ruleWords.test.ts`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `no set as home, and the Desk stays the start route` | favourite a dashboard; no control named *home* or *default* exists; the app's start route is still the Desk (task 62's frame) after a restart | a dashboard as the landing page |
| `the rail has no dashboards node` | `Rail.tsx` and `Desk.tsx` import nothing from `roadmap/Dashboard*`; a favourited dashboard shows in Favourites as its name only, no digit | a rail entry or a number on the rail (Q1) |
| `a block the viewer cannot read shows no number` | `data: {state: "not_visible"}` → *"Not visible to you"*; no digit in the block | a zero or a stale total standing in |
| `each block names its rule` | every rendered block holds `ruleWords(block)`; the title input cannot edit it | a number without its source |
| `a click opens the issues behind the number` | a count → `openIssues` with the block's ad hoc list; a scatter mark → `openIssue(id)` | a decorative number |
| `every block type passes expectNoPrediction` | each of the four with a filled result | a burn-up or "N of M" sneaking into a block |
| `the picker lists exactly the four types and four measures` | the picker's options equal `BLOCK_TYPES` and task 114's measures | a free-form metric that invents a measure |
| `ruleWords says what was measured` | `lead_time, 30` → *"lead time: created → done; issues done in the last 30 days"*; `triage_time` says *left triage* | words that drift from task 122's definitions |

`expectNoPrediction` is task 20's helper (`task-20.md:33`). Numbers use `tabular-nums`; a block
that is loading shows its rule line and no number, under the one global heartbeat (no spinner per
block, `DESIGN-SYSTEM.md`'s rule).

#### Steps

- [ ] **Step 1: confirm the sync base.** Tasks 72, 80, 83, 91, 114, 122, 125, 132 and 176
  committed; read their parts; replace this part's scope-taken names (`FilterGroup`, the views
  table, `ValidGroupBy`, `AdHocList`, `InsightsScatter`'s props) with theirs.
- [ ] **Step 2: the pure tests**, then the `hubapi` additions; six pass.
- [ ] **Step 3: the migration section** after task 176's, and the grants test entry.
- [ ] **Step 4: the integration tests**, then the two route deltas; rerun tasks 80 and 122's
  integration tests, which must stay green (a query with no `filter` is unchanged).
- [ ] **Step 5: the desktop tests**, then `ruleWords.ts`, `DashboardBlock.tsx`, `DashboardView.tsx`,
  `Dashboards.tsx`, and the two entries (Work lens `ViewHeader` actions, Plan).
- [ ] **Step 6: run the acceptance commands** and commit by path, each message ending with the
  `Co-Authored-By` line.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -v \
  -run '^Test(AListViewHoldsNoBlocks|EveryBlockTypeMatches|OnlyTheFourTypesExist|ATableNeedsAGroup|TheWindowIsBounded|ATeamDashboardQueries)' 2>&1 | grep -c "^--- PASS"
# expect: 6
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(ADashboardIsAView|AViewsKindNeverChanges|AFilterNarrowsTheCount|ANonMembersBlockGetsNoNumber)' 2>&1 | grep -c "^--- PASS"
# expect: 4
npm --prefix desktop test -- components/roadmap/Dashboard components/roadmap/ruleWords
# expect: 8 tests pass
grep -rlE "roadmap/Dashboard" desktop/src/components/rail desktop/src/components/desk; echo "exit $?"
# expect: no file listed, then "exit 1"
npm --prefix desktop run typecheck
# expect: exit 0
```

#### What this task does NOT claim

- A dashboard shared outside the org, embedded, or sent by mail or Teams.
- Any chart beyond task 132's table and scatter, or a time-axis scatter before task 114 adds each
  point's `at` (task 122's gap G2).
- *Effort* (`estimate_sum`), which task 114 does not yet carry (G2).
- That numbers stay fresh while a dashboard is open: each block reads once on open and on the
  person's refresh; nothing polls.

**Parity.** Closes **L-152** (`05-design.md:309`, MATCH, slice `later`); task 187 records it.

#### Gaps found while writing this part

1. **Personal dashboards cannot be Hub views**: tasks 80 and 91 keep personal views on the device,
   so the scope's *"inherits views' owner, team or personal scope"* holds for shared dashboards
   only; personal ones are desktop preferences (decision 1). Task 175 records the same.
2. **`InsightsQuery` has no filter** (`task-114.md:301-308`); this task adds one (decision 6), an
   added field to SW.20 that task 114's part should list.
3. **A count carries no ids** (`task-114.md:326`), so the issues behind a count are re-read as an
   ad hoc list and can differ by what changed between the reads; the list says *as of*.
4. **The scope's rule text** (*"created → landed"*) does not match task 122's lead time
   (creation → the standing done); this part follows task 122.
5. **The file map has no WL rows** (`roadmap/Dashboard*`, `ruleWords.ts`).



