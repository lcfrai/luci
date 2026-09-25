### Task 132: the insights panel and scatter; every point opens its issue

W4, stream WD, the desktop half of task 122 (Q1: *measured counts and durations, a secondary
surface*). Depends on task 122 (the query) and task 125 (`InsightsQuery`, `InsightsResult`,
`WorkClient.queryInsights`). Estimate 0.375 (3 h). Closes **L-150** (less *effort*) and **L-151**
with task 122, the time axis waiting on task 114 (gap 1).

After this task a person opens *Insights* from Plan's header, picks a measure, a grouping and a
range, and reads plain counts per group and a strip of measured durations with the Hub's p50, p75
and p90 marked. Every mark is a button that opens its issue, and the arrow keys walk them. There
is no trend line, no forecast band and no target: nothing on the panel is a number nobody measured.

**Decisions made here, so nobody re-derives them:**

1. **The Hub's numbers, drawn, never recomputed.** Percentile lines sit at `p50Ms`, `p75Ms` and
   `p90Ms` from `InsightsResult` (`task-125.md:172-175`) and are labelled with those values
   (*"p75 8h 00m"*). The client computes no mean, median or rank; a test holds the label to the
   Hub's value when the points' mean differs. `null` percentiles draw no line.
2. **A strip until the Hub sends `at`.** A scatter needs each point's end time on the x axis;
   `InsightPoint` is `{issue, ms}` (`task-125.md:170`) and task 122 names the missing `at` as its
   gap G2 (`task-122.md:52-58`). So this task draws **one axis**: duration along x, marks stacked
   in lanes where they would overlap, with the same percentile lines, and a caption says *"By
   duration. The time axis needs each point's end time, which the Hub does not send yet."* When
   114 adds `at`, the strip becomes the two-axis scatter by adding a y scale, and nothing else.
3. **Counts are a table, not bars.** `groups [{key, count}]` renders as a two-column table:
   the group's name (resolved from the replica: a person's name for `u-`, a label's for `lb-`, a
   feature's for `ft-`, a cycle's for `cy-`, the criterion's text for `bf-…/done:n`; `none` reads
   *No assignee*, *No label*, and so on) and the plain number in `tabular-nums`. No bar, no share,
   no total: an issue in two groups is counted in each (`task-122.md:29-35`), so a total would lie.
4. **Every mark is a button, in one tab stop.** Marks are HTML `<button>`s placed with
   `transform`, a roving `tabIndex` (one mark is `0`, the rest `-1`), in **point order**: ascending
   `ms`, ties by issue id, which is also left to right. `ArrowRight`/`ArrowLeft` move one mark,
   `Home`/`End` to the ends; `Enter` or a click opens the issue through the store's
   `openIssue(id)` (task 37, `task-37.md:560`). Each mark's name is *"EP-142, cycle time 5h 10m"*
   (AX Principle 7: every visual claim is a link to its source).
5. **The query is bounded as the Hub bounds it.** `from < to` and at most 366 days
   (`task-122.md:48-51`); the bar refuses anything else inline and sends nothing. Days are the
   person's, converted to the instant their day starts in their time zone.
6. **A secondary surface.** The only entry is an *Insights* action in Plan's `<ViewHeader
   actions>` (SW.13). The rail and the Desk import nothing from it (Q1); a test reads their source.
   The result lives in the panel's state only: never in the replica, the store or storage (task
   125 decision 3).

**Files:**
- `desktop/src/components/roadmap/InsightsQueryBar.tsx` — measure, group, range; `startOfDayIso`
  (new)
- `desktop/src/components/roadmap/InsightsScatter.tsx` — the strip, the lines, the marks (new)
- `desktop/src/components/roadmap/InsightsPanel.tsx` — the query, the table, the strip (new)
- `desktop/src/components/roadmap/insights.css` — `.in-*` rules (new)
- `desktop/src/components/roadmap/Insights.test.tsx` — the tests below (new)
- `desktop/src/components/work/PlanSurface.tsx` ⚑ (task 45) — the *Insights* action (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 37, 45, 108, 122 and
125. `formatDuration` is task 108's (`task-108.md:139-145`, `"1h 02m"`), in
`desktop/src/components/issue/StepFold.tsx`; this task imports it from there rather than writing a
second one. `useCockpit` is the store hook on the tree today (`desktop/src/store/store.ts:91`);
`openIssue` is task 37's slice member. The rail is `desktop/src/components/rail/Rail.tsx` (on the
tree); the Desk is task 62's `desktop/src/components/desk/Desk.tsx` (**UNVERIFIED**, planned).
