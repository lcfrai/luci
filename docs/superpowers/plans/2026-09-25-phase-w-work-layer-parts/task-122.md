### Task 122: insights queries: measured counts and durations, percentiles for the scatter

W4, stream WH; no seam. Depends on task 116 (the W4 migration; this task adds no table) and reads
what W1 already records: task 13's `issue.created` and `issue.state` events (`task-13.md:122-165`)
in the Hub's `audit_events` table, whose `at` is the Hub's clock (`hub-contract:1099-1100`: the
cursor is the org's audit sequence, in commit order). It also reads task 117's grants. The wire is
task 114's part (`task-114.md`, in progress, cited `T114` by type name). Every `server/` and
`store/` path is **UNVERIFIED until merge** (rule H1).

**Insights** answer *how long does our work take, and how much of it is there*, for one team over a
window: a count, or a duration per issue with three percentiles, grouped by a field. The desktop's
panel and scatter (task 132) draw it; every point opens its issue. Nothing here forecasts: no
trend line, no expected date, no *remaining* (Q1, `OD-27`). A number is either read from a
recorded state move or it is absent.

Six decisions the scope left open, made here.

1. **The population is fixed by one recorded moment per measure.** An issue belongs to a query when
   its **window event** falls in `[from, to)`: for `count`, `cycle_time` and `lead_time` that is
   the **standing done**, the last `issue.state` event whose `to` is `done`, on an issue whose head
   state is now `done` (or `shipped`, which task 71 appends after `done`, `task-03.md:922`); for
   `triage_time` it is the first `issue.state` event whose `from` is `triage`. An issue reopened and
   not now done has no standing done, so it is in no population but `triage_time`'s.
2. **The durations.** `cycle_time` is the first move into `started` to the standing done; an issue
   never started (closed by hand from `todo`) is **counted** under `count` and is **no point**
   under `cycle_time`, never a zero-length one. `lead_time` is `issue.created.at` to the standing
   done. `triage_time` is creation to the first move out of `triage`, for an issue created in
   `triage` (`IssueCreatedData.State`, `task-13.md:146`). Milliseconds, `int64`.
3. **Groups count the points, or the population for `count`.** `groups [{key, count}]` counts, per
   key, the issues that are points (or, for `count`, the population). An issue with two labels, or
   serving two criteria, is counted **once in each** group and is still **one** point: the groups
   say how many issues have a key, never a share of a total. Keys: `state` the head state; `assignee`
   a `u-` or `none`; `label` an `lb-` or `none`; `feature` an `ft-` or `none`; `cycle` a `cy-` or
   `none`; `priority` the priority word; `criterion` `<bf-…>/<criterionId>` or `none`. T114's
   `GroupBy` lists the first five; **gap G1**, fixed in 114, adds `priority` and `criterion`, which
   the scope names.
4. **Percentiles are nearest rank, and absent when nothing was measured.** Over the points' `ms`
   sorted ascending, `pN` is the value at rank `ceil(N/100 · len)`; with no points all three are
   `null` (T114's pointer fields). One to ten hours give 5, 8 and 9 hours. For `count` there are no
   points and the three are `null`.
5. **Visibility is the team's reach.** The population is issues in the team's own projects plus
   projects granted to the team through task 117 (`workAccess` names both); `projectId` narrows to
   one of those, else 404 `project_not_found` (`hub-contract:242`). The caller must be a member of
   `{teamId}` (any role); anyone else gets 404 `team_not_found`, so a team's existence is not
   confirmed. Erased issues **count**: their ids and moves are kept (05 §3.6), only their text is
   gone, and the answer carries ids, not text.
6. **Bounded work.** `from < to`, both RFC 3339, the span at most 366 days, else 422 `field:
   "from"` or `"to"`; a population above 5,000 issues is 422 `field: "to"`, `reason:
   "too_many_issues"`, so a query is always one bounded read. The limit is a guard, not a
   measurement: how long 5,000 takes is **unmeasured**.

**Two gaps in SW.20, carried from the scope and fixed in 114, not here (G2).** L-150's *effort*
needs `measure: "estimate_sum"` over recorded estimates (task 79), which T114's measures lack.
L-151's scatter needs each point's `at` (the moment its duration ended, the window event), which
T114's `InsightPoint {issue, ms}` lacks. Until 114 adds both, this task serves the rest, and the
acceptance says so: L-150 is closed less *effort*, and L-151 only when 114 adds `at` and task 132
draws it.

**The one thing this task must not get wrong** is a number nobody measured: a zero-length point
for an issue that was never started, a percentile of an empty set, a mean passed off as a median,
or a key in the answer that reads as a forecast. Each is a row below.
