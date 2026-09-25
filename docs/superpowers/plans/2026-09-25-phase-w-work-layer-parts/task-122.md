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

**Files:**

- `control-plane/internal/hub/server/insights.go`, `insights_test.go` (new): the pure computation,
  moves in, `InsightsResult` out
- `control-plane/internal/hub/store/work_insights.go` (new): the two bounded reads
- `control-plane/internal/hub/server/work_insights.go` (new): the route
- `control-plane/internal/hub/server/work_insights_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑: the `"122"` rows

**Interfaces:**

- Produces: `POST /v1/teams/{teamId}/insights/query` (T114's route table; a read that takes a body,
  so `POST`, and it appends no event); `server.ComputeInsights`
- Consumes: T114's `InsightsQuery`, `InsightsResult`, `InsightGroup`, `InsightPoint` and the
  `Measure*` constants; task 13's `issue.created` and `issue.state` kinds and their data; task
  117's `workAccess` and its granted-project list; task 03's issue fields (assignee, labels,
  feature, cycle, priority) and task 08's `issue_serves`
- Consumed by: task 124 (`QueryInsights` over HTTP and the relay), task 132 (the panel and scatter)

#### The computation (`server/insights.go`, complete)

```go
package server

import (
	"sort"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Move is one recorded event of an issue's life, in audit order.
type Move struct {
	Kind     string    // "issue.created" | "issue.state"
	At       time.Time // the Hub's clock
	From, To string    // issue.state only; for issue.created, To is the initial state
}

// IssueFacts is one issue of the reach, its moves, and its keys per group.
type IssueFacts struct {
	ID    string
	Head  hubapi.IssueState // the head state now
	Moves []Move
	Keys  map[string][]string // groupBy → its keys; a field with no value is ["none"]
}

// windowAndDuration is decisions 1 and 2. ok is false when the issue is not in
// the measure's population; hasMs is false when it is but has no duration.
func windowAndDuration(f IssueFacts, measure string) (window time.Time, ms int64, ok, hasMs bool) {
	var created, firstStarted, standingDone, firstOutOfTriage time.Time
	var createdInTriage bool
	for _, m := range f.Moves {
		if m.Kind == "issue.created" {
			created, createdInTriage = m.At, m.To == string(hubapi.StateTriage)
			continue
		}
		// Three independent rules: one move (triage → started) can feed two.
		if m.To == string(hubapi.StateStarted) && firstStarted.IsZero() {
			firstStarted = m.At
		}
		if m.From == string(hubapi.StateTriage) && firstOutOfTriage.IsZero() {
			firstOutOfTriage = m.At
		}
		if m.To == string(hubapi.StateDone) {
			standingDone = m.At // the last one wins
		}
	}
	isDone := f.Head == hubapi.StateDone || f.Head == hubapi.StateShipped
	switch measure {
	case hubapi.MeasureTriageTime:
		if !createdInTriage || firstOutOfTriage.IsZero() {
			return time.Time{}, 0, false, false
		}
		return firstOutOfTriage, firstOutOfTriage.Sub(created).Milliseconds(), true, true
	case hubapi.MeasureCount, hubapi.MeasureCycleTime, hubapi.MeasureLeadTime:
		if !isDone || standingDone.IsZero() {
			return time.Time{}, 0, false, false
		}
		switch measure {
		case hubapi.MeasureCycleTime:
			if firstStarted.IsZero() || firstStarted.After(standingDone) {
				return standingDone, 0, true, false
			}
			return standingDone, standingDone.Sub(firstStarted).Milliseconds(), true, true
		case hubapi.MeasureLeadTime:
			return standingDone, standingDone.Sub(created).Milliseconds(), true, true
		}
		return standingDone, 0, true, false // count: in the population, no duration
	}
	return time.Time{}, 0, false, false
}

// ComputeInsights is decisions 1-4 over the facts the store read. groupBy ""
// means no grouping: one group, key "all".
func ComputeInsights(facts []IssueFacts, measure, groupBy string, from, to time.Time) hubapi.InsightsResult {
	res := hubapi.InsightsResult{Groups: []hubapi.InsightGroup{}, Points: []hubapi.InsightPoint{}}
	counts := map[string]int{}
	var ms []int64
	for _, f := range facts {
		w, d, ok, hasMs := windowAndDuration(f, measure)
		if !ok || w.Before(from) || !w.Before(to) {
			continue
		}
		if measure != hubapi.MeasureCount && !hasMs {
			continue // in the population, not a point: never a zero-length one
		}
		if hasMs {
			res.Points = append(res.Points, hubapi.InsightPoint{Issue: f.ID, Ms: d})
			ms = append(ms, d)
		}
		keys := []string{"all"}
		if groupBy != "" {
			keys = f.Keys[groupBy]
		}
		for _, k := range keys {
			counts[k]++ // once per key; the point above is once per issue
		}
	}
	for k, n := range counts {
		res.Groups = append(res.Groups, hubapi.InsightGroup{Key: k, Count: n})
	}
	sort.Slice(res.Groups, func(i, j int) bool { return res.Groups[i].Key < res.Groups[j].Key })
	sort.Slice(res.Points, func(i, j int) bool { return res.Points[i].Issue < res.Points[j].Issue })
	res.P50Ms, res.P75Ms, res.P90Ms = nearestRank(ms, 50), nearestRank(ms, 75), nearestRank(ms, 90)
	return res
}

// nearestRank is decision 4: the value at rank ceil(p/100 · n) of the sorted
// values, or nil when there are none. Never interpolated, never a mean.
func nearestRank(values []int64, p int) *int64 {
	if len(values) == 0 {
		return nil
	}
	s := append([]int64(nil), values...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	// ceil(p·n/100) in integers. In float64, 0.07*100 is 7.000000000000001,
	// whose ceiling is 8: a float rank is off by one on some (p, n) pairs.
	rank := (p*len(s) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	v := s[rank-1]
	return &v
}
```

Groups are sorted by key and points by issue id, so one query answers the same bytes twice and the
golden test is stable. The `count` measure keeps its points empty (T114: *"empty for `count`"*),
and its percentiles `nil`, because `hasMs` is false for every issue it counts.

#### The store (`store/work_insights.go`, signatures complete)

```go
package store

// ReachProjects is decision 5: the team's own projects and the projects
// granted to it (task 117's work_project_grants), optionally narrowed to one.
func ReachProjects(ctx context.Context, q Querier, teamID, projectID string) ([]string, error)

// WindowIssues is the first bounded read: the ids of issues in those projects
// with a window-event candidate in [from, to), capped at limit+1 so the
// handler can refuse a population above limit (decision 6):
//   SELECT DISTINCT e.subject FROM audit_events e JOIN issues i ON i.id = e.subject
//    WHERE e.kind = 'issue.state' AND i.project_id = ANY($1)
//      AND e.at >= $2 AND e.at < $3
//      AND (e.data->>'to' = 'done' OR e.data->>'from' = 'triage')
//    LIMIT $4
func WindowIssues(ctx context.Context, q Querier, projects []string, from, to time.Time, limit int) ([]string, error)

// IssueFactsOf is the second: every issue.created and issue.state event of
// those issues in seq order, the head state, and the keys for groupBy (one
// join per field; labels, issue_serves and the like give several rows).
func IssueFactsOf(ctx context.Context, q Querier, issueIDs []string, groupBy string) ([]server.IssueFacts, error)
```

`IssueFactsOf` returning `server.IssueFacts` would make `store` import `server`; the build puts
`IssueFacts` and `Move` in a small shared package or in `store` itself and `server` aliases them.
Which of the two is a build-time choice, named so the reader does not read the sketch as an import
cycle. Row-level security binds both reads to `hub.org_id`; the project list binds them to the
team's reach. The candidate filter is wider than the window rule (an issue done in the window and
reopened later is read and then dropped by `windowAndDuration`), which is what makes it safe to
compute the rule once, in Go.

#### The route (`server/work_insights.go`)

```go
mux.HandleFunc("POST /v1/teams/{teamId}/insights/query", s.queryInsights)
```

**`queryInsights`**, inside `s.Read` (no event: a query changes no record): the caller a member of
`{teamId}`, any role, else 404 `team_not_found`; `Decode[hubapi.InsightsQuery]`; its `teamId`
absent or equal to the path (else 422 `field: "teamId"`); `measure` one of T114's four (422
`field: "measure"`); `groupBy` `null` or one of the seven keys of decision 3 (422 `field:
"groupBy"`); `from` and `to` RFC 3339 with `from < to` and at most 366 days apart (422); then
`ReachProjects` (a `projectId` outside it → 404 `project_not_found`), `WindowIssues` with
`limit 5000` (5,001 ids → 422 `field: "to"`, `reason: "too_many_issues"`), `IssueFactsOf`,
`ComputeInsights`, 200.

#### Tests to write first, part 1 — `server/insights_test.go` (pure)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCycleTimeRunsFromStartedToDone` | moves `triage → todo → started → in_review → landed → done` one hour apart → `cycle_time` is 3 h; `lead_time` 5 h; `triage_time` 1 h | cycle time from creation; lead time from started |
| `TestAHandClosedIssueIsCountedAndNoPoint` | `todo → done` by hand → in `count`'s group, no `cycle_time` point, `points: []`, percentiles `null` | a zero-length point |
| `TestNearestRankNotInterpolated` | ten points of 1 to 10 hours → `p50` 5 h, `p75` 8 h, `p90` 9 h | "interpolated" (5.5, 7.75, 9.1 h) and "mean" (5.5 h) |
| `TestIntegerRankNeverOffByOne` | 100 points of 1 to 100 ms, `nearestRank(…, 7)` → 7 ms | a float rank (8 ms) |
| `TestNoPointsMeansNoPercentiles` | no closed issue → `groups: []`, `points: []`, three `null` | "p50 of 0" |
| `TestAReopenedIssueHasNoDuration` | `started → done → todo` → no point for `cycle_time` or `lead_time`, and not in `count` | a stale done counted |
| `TestTheLastDoneStands` | `started → done → started → done` → cycle time from the **first** started to the **second** done | the first done taken |
| `TestTwoCriteriaAreOnePoint` | an issue serving two criteria, `groupBy: criterion` → two groups of count 1, one point | double-counted durations |
| `TestTheWindowIsHalfOpen` | a done exactly at `to` is out; one exactly at `from` is in | an inclusive or shifted window |

#### Tests to write first, part 2 — `server/work_insights_integration_test.go` (`//go:build integration`)

The cast is task 11's (Grace, a Portal member; Linus, Billing only; Xavier, another org) and task
117's grant of Billing's project to Portal's team. Moves are made through the real routes, with the
Hub's clock moved by the test harness (task 11's clock hook; its name is **UNVERIFIED** here, and
Hub task 24's `JanitorConfig.Now` shows the Hub already takes an injected clock).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestInsightsReadTheHubsClock` | an issue moved with a client sending a `Date` header a day off → durations from the events' `at`, not the header | the client's clock |
| `TestVisibilityIsTheTeamsReach` | Portal's query counts Portal's issues and the granted Billing project's, never another Billing project's; with `projectId` of an ungranted project → 404 | visibility not applied |
| `TestOnlyMembersQuery` | Linus on Portal's team → 404 `team_not_found`; Xavier → 404; a Portal viewer → 200 | a query readable outside the team |
| `TestErasedIssuesStillCount` | an issue erased by an org admin (task 14) stays one point, its id kept, no title anywhere in the answer | erase dropping history, or leaking text |
| `TestTheAnswerForecastsNothing` | every key of the JSON answer, at any depth, is checked against `forecast`, `eta`, `trend`, `expected`, `remaining`, `projected`, `percent` and `progress` → none | a prediction in the handler (beside T114's type test) |
| `TestARangeIsBounded` | `from ≥ to` → 422 `field: from`; a 400-day range → 422; 5,001 issues in range → 422 `reason: too_many_issues` | an unbounded scan |
| `TestAQueryAppendsNoEvent` | the org's feed `next` is the same before and after ten queries | a read that writes |

#### The matrix rows (`workMatrixRows["122"]`, appended)

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `POST /v1/teams/{teamId}/insights/query` | 200 | 200 | 200 | 404 | 404 | 404 |

`O` is 404 as for every Work read (`task-07.md`'s `wread`). A granted team's member queries their
**own** team's reach, which includes the grant; they get no row on the granting team's route.

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1): tasks 13, 114, 116 and 117 committed; re-read
  task 13's `IssueStateData` and `IssueCreatedData` at the base.
- [ ] **Step 2:** Check T114 for G1 (`priority`, `criterion` in `GroupBy`) and G2 (`estimate_sum`,
  `InsightPoint.at`). G1 missing: stop and raise it. G2 missing: go on, and the parity line below
  stays as written.
- [ ] **Step 3:** Write `server/insights_test.go`; it fails to compile; write `insights.go`; green.
- [ ] **Step 4:** Write the integration test and the matrix row; every row fails.
- [ ] **Step 5:** `store/work_insights.go` and the route; run until green.
- [ ] **Step 6:** `git diff -- go.work control-plane/go.mod` shows nothing; commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(CycleTimeRunsFromStartedToDone|AHandClosedIssueIsCountedAndNoPoint|NearestRankNotInterpolated|IntegerRankNeverOffByOne|NoPointsMeansNoPercentiles|AReopenedIssueHasNoDuration|TheLastDoneStands|TwoCriteriaAreOnePoint|TheWindowIsHalfOpen)$' \
  2>&1 | grep -c "^--- PASS"                                                                    # 9
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(InsightsReadTheHubsClock|VisibilityIsTheTeamsReach|OnlyMembersQuery|ErasedIssuesStillCount|TheAnswerForecastsNothing|ARangeIsBounded|AQueryAppendsNoEvent)$' \
  2>&1 | grep -c "^--- PASS"                                                                    # 7
go test ./control-plane/... -race -count=1                                                      # ok
```

#### What this task does not claim

- Dashboards (L-152, task 177), or any surface on the rail or the Desk (never, Q1); the panel and
  scatter are task 132's.
- Query speed at any size: the 5,000-issue guard is a bound, not a measurement.
- *Effort* (L-150's `estimate_sum`) and the scatter's time axis (L-151's `at`) until task 114 adds
  them (G2).
- That a short cycle time is good work: the numbers are what was recorded, and the reading is the
  team's.

**Parity rows served:** **L-150** (insights: counts and durations, less *effort*) and **L-151**
(the scatter, with task 132, once G2's `at` lands).

**Gaps for the skeleton and siblings:** G1 (`priority` and `criterion` in `GroupBy`) and G2
(`estimate_sum`; `InsightPoint.at`) belong to task 114; `IssueFacts` needs a home both `store` and
`server` can import (a build-time choice).
