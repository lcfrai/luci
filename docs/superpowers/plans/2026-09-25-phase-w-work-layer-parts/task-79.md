### Task 79: estimates, due dates, rank, move with redirect, the resolved-blocker rule

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (the `issue_heads` plan columns, `issue_aliases`, the W2 guards), after a
sync-merge of `feat/hub` (rule H1). **Seams:** consumes **SW.18** (task 72's `IssuePlan`,
`IssueAlias`, `TeamPlanPolicy`, `TeamPlanPolicyPatch`, `EstimateScale`, `EstimatePoints`,
`ValidEstimate`, `ValidDate`, `ValidRank`, `RankMaxLen`, `Change[T]`, `PatchIssuePlanRequest`,
`RankRequest`, `MoveIssueRequest`, and the codes `estimate_off_scale`, `date_invalid`,
`rank_neighbours`, `children_open`), **SW.4** (task 10's state move) and task 11's relations;
adds four event kinds to **SW.5** (task 13's registry; additive, so SW.5's freeze allows it).

Five small planning facts, each a person's act: how big an issue is, when it is due, where it sits
in the team's shared order, which project it lives in, and what happens to the issues it blocked
once it closes. None of them is in the issue-revision canonical form (task 72 block 4), so none
re-hashes a revision, and each is one `PATCH`- or `POST`-shaped write with one event. This task is
the Hub half; task 89 is the desktop.

**Anchors, re-read on 2026-09-26.** In this branch's parts (not yet code, so **UNVERIFIED until
merge**): task 72's scales and points (`task-72.md:143-164`), `RankMaxLen` and `rankRE`
(`:249`, `:255`), `ValidDate` and `ValidEstimate` (`:285-301`), the codes (`:352-370`),
`IssuePlan`, `IssueAlias`, `TeamPlanPolicy`, `TeamPlanPolicyPatch` (`:390-430`), the bodies and
`Change[T]` (`:667-736`), the `hubclient` methods (`:935-940`); task 03's relation kinds
(`task-03.md:147-152`); task 06's `issue_relations` table (`task-06.md:259-269`); task 08's
by-number route (`task-08.md:149`, `:270`, `:320`) and `issue_not_found` (`:171`); task 10's
codes (`task-10.md:242-244`) and its `hub.work_actor_kind` setting (`:359`); task 11's relation
delete and `issue.unrelated` (`task-11.md:343`); task 13's `IssueRelationData`
(`task-13.md:166-170`). In the parity docs: 05 §1 rows 14, 16, 25, 35, 37 and 103
(`05-design.md:123-237`) and §3.6's one-event rule (`:798-812`). **Task 74's part is not written
at this writing**, so its column names (`issue_heads.estimate`, `.due_on`, `.rank`;
`issue_aliases (project_id, number)`) are the scope's (`tasks-61-80.md:479-483`),
**UNVERIFIED**; a different spelling in task 74 is a find-and-replace in this task's store files.

#### Decisions the scope left open, made here

**(a) A scale change rewrites no estimate.** Changing a team's `estimateScale` through task 11's
`work-policy` route leaves every stored estimate as it was; the head reads it, and task 89 marks
one that is off the current scale. Only a write is checked: `ValidEstimate` against the team's
policy **at write time**. Rewriting on a scale change would be a mass edit no person made issue
by issue, with one event for many rows. Clearing (`"estimate": null`) is always allowed, on scale
`none` too.

**(b) A plan patch that changes nothing writes nothing.** `PATCH /plan` with both fields absent,
or with values equal to the stored ones, answers 200 with the head and appends no event (task 75's
"the same list twice" precedent). A patch that changes one field or both writes one
`issue.planned` event.

**(c) The rank key is the server's, in three shapes.** `after` is the issue above, `before` the
one below (task 72's comment). Between two neighbours the key is the digit-wise midpoint
(`rankBetween`); at the bottom of the order (`before: null`) it is the next step in a four-digit
grid (`rankAfter`); at the top (`after: null`) the previous step (`rankBefore`). Stepping, not
bisecting, at the two ends matters because every new issue joins at the bottom: bisection there
adds one character per five creates and would force a rebalance every 140 or so issues, where the
grid gives about 839,000 bottom slots from the first key (the arithmetic is under *Rank*).

**(d) Every head has a rank.** A new issue joins its team's order at the bottom, in the same
transaction as task 08's create (one call, `store.BottomRank`). A move to another team's project
takes the target team's bottom; a move inside one team keeps its key (the team's order spans all
its projects). Task 74 backfills W1 heads in `(created_at, id)` order with `rankSpread`. Both are
asked of those tasks here and reported as gaps.

**(e) A move is refused while work is bound.** From `started` or `in_review` a lane and a branch
in the source project's repository are live, so a move is 422 `state_not_allowed` with
`details.cause: "move"` (task 10's code; no new code). Every other state moves, `landed` and the
closed states included: proof and trailers cite ids, not numbers (05 §3.6).

**(f) What a move clears.** Kept: the id, revisions, comments, `serves` (criterion ids),
relations, priority, estimate, due date, the assignee if a member of the target team (else
cleared). Cleared when the teams differ: team labels (org labels stay), `cycleId`, `subState`.
Always cleared: `featureId` (features are per project). A parent is cleared; an issue that has
children is refused 409 `children_open` with `details.open` counting every child, any state,
because task 78's parent rule keeps a child in its parent's project. The event names what was
cleared.

**(g) A move rewrites no history, so two W1 keys must loosen.** Task 06 pins an issue's project
into `issue_revisions` and `issue_attachments` with `FOREIGN KEY (org_id, issue_id, project_id)
REFERENCES issues (org_id, id, project_id)` (`task-06.md:214`, `:362`, its decision 3). Changing
`issues.project_id` would be `23503`, and cascading would rewrite revisions whose canonical form
covers `ProjectID` (`task-08.md:356-357`). So task 74 replaces both keys with `(org_id, issue_id)
REFERENCES issues (org_id, id)` and a `BEFORE INSERT` trigger refusing a row whose `project_id` is
not the issue's current one: new rows keep decision 3's guarantee, old rows keep the project they
were written in. Task 74 also grants `UPDATE (project_id, team_id, number)` on `issues`, guarded
by a trigger that admits the change only under `hub.work_cause = 'move'`. Asked of task 74 here,
reported as a gap.

**(h) The resolved-blocker rule reads one direction.** Only rows where the closing issue is the
blocker (`issue_id` = it, `kind = 'blocks'`) turn into `relates`. A blocked issue closing leaves
its blocker's row alone: that blocker still blocks nothing open, and task 11's reads already say
so. Where a `relates` row between the two already exists (either direction, task 11 checks both,
`task-05.md:469`), the `blocks` row is deleted instead of updated, so the primary key never
collides. One event, `issue.blocks_resolved`, only when at least one row changed; its actor is
the `issue.state` event's actor in the same transaction (task 10's rule, task 75 decision (e)'s
clearer), which is the person who closed the blocker (05 row 25).

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/store/work_plan_fields.go` | new: the plan write, the rank keys (`rankBetween`, `rankAfter`, `rankBefore`, `rankSpread`), neighbours, rebalance |
| `control-plane/internal/hub/store/work_plan_fields_test.go` | new: the rank keys as pure functions (no build tag) |
| `control-plane/internal/hub/server/work_plan_fields.go` | new: `PATCH /plan`, `POST /rank` |
| `control-plane/internal/hub/store/work_move.go` | new: the move, the alias write, the alias read |
| `control-plane/internal/hub/server/work_move.go` | new: `POST /move` |
| `control-plane/internal/hub/server/work_plan_fields_integration_test.go`, `work_move_integration_test.go` | new: the tests below |
| `control-plane/internal/hub/server/work_policies.go` ⚑ (task 11's) | the W2 half of `PUT /work-policy` |
| `control-plane/internal/hub/server/work_states.go` ⚑ (task 10's) | the resolved-blocker clause |
| `control-plane/internal/hub/server/work_issues.go` ⚑ (task 08's) | `by-number` reads `issue_aliases`; create takes `store.BottomRank` |
| `control-plane/internal/hub/hubapi/work_events.go` ⚑ (task 13's) | four kinds and their data structs |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ (task 07's) | `workMatrixRows["79"]`; `"79"` in `workMatrixLive` |
| `control-plane/internal/hub/server/server.go` ⚑ | one line: `s.registerWorkPlanFields()` |

The scope named `server/matrix_integration_test.go`; W1's Work rows live in task 07's
`work_matrix_rows_integration_test.go` (task 75's correction, `task-75.md:86-88`). The file name
`hubapi/work_events.go` is task 13's as task 75 reads it (`task-75.md:89-90`); if the registry sits
in `work.go`, the kinds go there. Registration uses task 08's recorder, `s.route` (`task-08.md:312-323`),
so `(*Server).Patterns()` lists the routes and task 07's meta-tests see them.

#### The wire (the bodies are task 72's)

```text
PATCH /v1/issues/{issueId}/plan   member  200 IssueHead   body PatchIssuePlanRequest{estimate, dueOn}
POST  /v1/issues/{issueId}/rank   member  200 IssueHead   body RankRequest{after, before}
POST  /v1/issues/{issueId}/move   member of both teams  200 IssueHead  body MoveIssueRequest{toProjectId}
GET   /v1/projects/{projectId}/issues/by-number/{n}  viewer  200 IssueWithRevision (task 08's), head.movedFrom set
PUT   /v1/teams/{teamId}/work-policy  admin  200 TeamWorkPolicy; W2 fields absent keep (task 72's patch)
```

Worked examples, on a Fibonacci team, EP-142 in `todo`:

```json
PATCH /v1/issues/is-3f2a91c07b1e4d2a/plan   {"estimate": 5, "dueOn": "2026-10-02"}
→ 200 {… "estimate": 5, "dueOn": "2026-10-02", "rank": "i001", …}
PATCH /v1/issues/is-3f2a91c07b1e4d2a/plan   {"estimate": null}
→ 200 {… "estimate": null, "dueOn": "2026-10-02", …}
POST  /v1/issues/is-3f2a91c07b1e4d2a/rank   {"after": null, "before": "is-00000000000000a1"}
→ 200 {… "rank": "hzzz", …}     (a1 was the team's first issue, key "i"; EP-142 now sits above it)
POST  /v1/issues/is-3f2a91c07b1e4d2a/move   {"toProjectId": "pj-00000000000000b2"}
→ 200 {… "number": 8, "alias": "PAY-8", "movedFrom": {"projectId": "pj-00000000000000b1", "projectKey": "EP", "number": 142}, …}
GET   /v1/projects/pj-00000000000000b1/issues/by-number/142
→ 200 {"head": {… "id": "is-3f2a91c07b1e4d2a", "alias": "PAY-8", "movedFrom": {… "number": 142}}, "revision": {…}}
```

Refusals, each with its code and the field it names:

| Body or state | Answer |
|---|---|
| `estimate: 4` on Fibonacci; `0` without `estimateZero`; any value on scale `none` | 422 `estimate_off_scale`, `details.scale`, `details.value` |
| `estimate: 2.5`, `"3"`, or over 2^31 | 400 `bad_request`, `field: "estimate"` (strict decode) |
| `dueOn: "2026-10-02T09:00:00Z"`, `"2026-02-30"`, `"02/10/2026"` | 422 `date_invalid`, `details.field: "dueOn"` |
| a `rank` key in a rank body | ignored: task 72's strict decode refuses unknown keys, so 400 `bad_request`, `field: "rank"` |
| `after` or `before` in another team, erased, the issue itself, or `after` not above `before` | 422 `rank_neighbours`, `details.field` the one at fault |
| a move from `started` or `in_review` | 422 `state_not_allowed`, `details.from` the state, `details.cause: "move"` |
| a move of an issue with children | 409 `children_open`, `details.open` every child |
| a move to its own project | 400 `bad_request`, `field: "toProjectId"` |
| a target the caller cannot read | 404 `project_not_found` (no leak); readable but not a member: 403 |

A client that sends `rank` is refused, not ignored silently: task 72's `TestWorkPlanRequestsRejectUnknownFields`
row `"RankRequest": {"rank":"i"}` (`task-72.md:1129-1130`) already fixes this, so the scope's "ignored"
reads as "never used"; see *Corrections to the scope*.

#### Rank (`store/work_plan_fields.go`, complete)

The alphabet is in byte order, so Postgres with `COLLATE "C"` on `issues.rank` (asked of task 74)
sorts exactly as Go's `<` does. No key ends in `'0'`, so a gap exists below every key.

```go
const rankDigits = "0123456789abcdefghijklmnopqrstuvwxyz"

const (
	rankBase     = int64(len(rankDigits)) // 36
	rankGrid     = 4                      // the two ends step on a four-digit grid
	rankGridSize = rankBase * rankBase * rankBase * rankBase // 1,679,616 slots
	rankMinGap   = 64                     // rankSpread's narrowest gap, in steps
)

func rankDigit(c byte) int64 { return int64(strings.IndexByte(rankDigits, c)) }

// rankBetween returns k with lo < k < hi; lo "" is the top end, hi "" the bottom.
// ok is false past hubapi.RankMaxLen (the caller rebalances) or when lo >= hi.
func rankBetween(lo, hi string) (string, bool) {
	out := make([]byte, 0, hubapi.RankMaxLen)
	bound := hi != "" // out is still a prefix of hi
	for i := 0; len(out) < hubapi.RankMaxLen; i++ {
		l, h := int64(0), rankBase
		if i < len(lo) {
			l = rankDigit(lo[i])
		}
		if bound {
			if i >= len(hi) {
				return "", false // lo >= hi
			}
			h = rankDigit(hi[i])
		}
		switch {
		case h-l > 1:
			return string(append(out, rankDigits[(l+h)/2])), true
		case h-l == 1:
			bound = false // out < hi from here on
		case h < l:
			return "", false
		}
		out = append(out, rankDigits[l])
	}
	return "", false
}

// rankGridValue reads s's first rankGrid digits as a number, '0'-padded.
func rankGridValue(s string) int64 {
	var v int64
	for i := 0; i < rankGrid; i++ {
		d := int64(0)
		if i < len(s) {
			d = rankDigit(s[i])
		}
		v = v*rankBase + d
	}
	return v
}

// rankKey writes x as width digits and strips trailing '0's (x >= 1, so never "").
// Stripping keeps order among keys of one width: a stripped key is a prefix of
// its original and still sorts before every key that shared the prefix.
func rankKey(x int64, width int) string {
	b := make([]byte, width)
	for i := width - 1; i >= 0; i-- {
		b[i] = rankDigits[x%rankBase]
		x /= rankBase
	}
	return strings.TrimRight(string(b), "0")
}

// rankAfter is the next grid step below lo, for a join at the bottom of the order.
// lo < rankKey(g+1) because lo's value is below g+1 whatever its later digits.
func rankAfter(lo string) (string, bool) {
	if lo == "" {
		return rankBetween("", "")
	}
	if g := rankGridValue(lo) + 1; g < rankGridSize {
		return rankKey(g, rankGrid), true
	}
	return rankBetween(lo, "")
}

// rankBefore is the previous grid step above hi, for a join at the top.
func rankBefore(hi string) (string, bool) {
	if hi == "" {
		return rankBetween("", "")
	}
	if g := rankGridValue(hi) - 1; g >= 1 {
		return rankKey(g, rankGrid), true
	}
	return rankBetween("", hi)
}

// rankSpread returns n increasing keys over the key space's middle half, a quarter
// left free at each end for joins. Width grows past the grid only while a gap
// would be under rankMinGap steps: four digits hold 13,121 issues, five 472,391,
// six 17,006,111 (36^w / 2 / 64 − 1).
func rankSpread(n int) []string {
	width, size := rankGrid, rankGridSize
	for size/2/int64(n+1) < rankMinGap && width < 12 { // 36^12 < 2^63
		width, size = width+1, size*rankBase
	}
	keys := make([]string, n)
	for i := range keys {
		keys[i] = rankKey(size/4+int64(i+1)*(size/2)/int64(n+1), width)
	}
	return keys
}
```

**The arithmetic behind decision (c).** `rankBetween` spends about one digit per five inserts at
one spot: below a key the new digit runs `i`, `9`, `4`, `2`, `1` (the midpoints of 36, 18, 9, 4,
2), then a new position opens. At the bottom, the first key `i` is grid value 839,808, leaving
1,679,616 − 839,808 − 1 = 839,807 steps before `rankAfter` falls back to bisection. After a
rebalance the free quarter at each end is 419,904 steps. **What was run:** a Python model of the
four functions above (a transcription, not the Go; `scratchpad/rank.py` in the planning session,
not committed) printed, on 2026-09-26: the first two keys `i` and `i001`; `rankBefore("i")` is
`hzzz`; the first insert between them `i000i`; 200 inserts just below `i` rebalance **once, at
insert 141**, with 142 keys, and end with the longest key at 14 characters and the order intact;
`rankSpread(5)` is `c f i l o`; `rankSpread(13121)` is four digits wide and `rankSpread(13122)`
five; and 839,807 bottom steps before the fallback. The Go test below re-proves the rebalance
count and the order; the model is not evidence for the Go.

**(i) Neighbours need order, not adjacency.** A board or a filtered view shows a subset of the
team's order, so the two issues a person drops between are often not adjacent in the whole order.
Refusing them would refuse most drags. So: with `after` named, the key goes between `after` and
its true successor in the team (the moving issue excluded); with only `before`, between `before`'s
true predecessor and `before`; with both, `after` must sort above `before` (else 422
`rank_neighbours`, `details.field: "before"`), and the key goes below `after`. Either way it sorts
between the two on a fresh read, which is all L-14 promises. This loosens task 72's comment
("non-adjacent … are 422", `task-72.md:724-727`); see *Corrections to the scope*.

#### The four event kinds (complete, appended to task 13's registry)

Every field is a string, `int64` or `bool`, so task 13's scalar test covers them unchanged. An
estimate, a date and a rank key are planning metadata, not content (hub-contract §8).

```go
const (
	EventIssuePlanned        = "issue.planned"
	EventIssueRanked         = "issue.ranked"
	EventIssueMoved          = "issue.moved"
	EventIssueBlocksResolved = "issue.blocks_resolved"
)

// IssuePlannedData names the fields the patch changed ("estimate", "dueOn",
// comma-joined) and their values after it; HasEstimate false is none.
type IssuePlannedData struct {
	Fields      string `json:"fields"`
	Estimate    int64  `json:"estimate"`
	HasEstimate bool   `json:"hasEstimate"`
	DueOn       string `json:"dueOn"` // "" is none
}

// IssueRankedData is one key write. Rebalance marks a key moved by a rebalance,
// not by a person's drag; RebalanceOf is the dragged issue that caused it.
type IssueRankedData struct {
	Rank        string `json:"rank"`
	Rebalance   bool   `json:"rebalance"`
	RebalanceOf string `json:"rebalanceOf"` // "" unless Rebalance
}

// IssueMovedData is the move; Cleared names the fields the move cleared
// (decision (f)), comma-joined, in IssuePlan's JSON names plus "labels" and "assigneeId".
type IssueMovedData struct {
	FromProjectID string `json:"fromProjectId"`
	FromNumber    int64  `json:"fromNumber"`
	ToProjectID   string `json:"toProjectId"`
	ToNumber      int64  `json:"toNumber"`
	Cleared       string `json:"cleared"`
}

// IssueBlocksResolvedData is the resolved-blocker rule's one event (decision (h)).
type IssueBlocksResolvedData struct {
	OtherIssueIDs string `json:"otherIssueIds"` // the issues it had blocked, comma-joined
	Converted     int64  `json:"converted"`     // rows now relates
	Merged        int64  `json:"merged"`        // rows deleted: a relates row already existed
}
```

```go
// appended to WorkEventKinds
{EventIssuePlanned, IssuePlannedData{}, ScopeProject, VisibleToReaders},
{EventIssueRanked, IssueRankedData{}, ScopeProject, VisibleToReaders},
{EventIssueMoved, IssueMovedData{}, ScopeProject, VisibleToReadersAndSource},
{EventIssueBlocksResolved, IssueBlocksResolvedData{}, ScopeProject, VisibleToReaders},
```

**A move is seen from both sides.** `issue.moved` carries the target's `teamId` and `projectId`,
so under task 13's `VisibleToReaders` a reader of the source project in another team would never
learn the issue left, and that reader's replica would keep it. One more `Visibility`, appended
after `SubjectOnly` (additive; existing values keep their numbers):

```go
// VisibleToReadersAndSource adds readers of the project named in data.fromProjectId.
// Only issue.moved uses it.
VisibleToReadersAndSource
```

Task 13's `/v1/changes` filter gains the matching clause, `OR (e.kind = 'issue.moved' AND
e.data->>'fromProjectId' = ANY($readable))`; task 13 owns the query, so this is asked of it and
reported as a gap. The source reader sees the event and a head it may no longer read: the desktop
drops the row and shows *"EP-142 moved to PAY-8"* only when the target is readable (task 89).
The `e.data->>` form assumes task 13 stores `data` as `jsonb`, **UNVERIFIED**.

#### The store (signatures complete)

`store/work_plan_fields.go`, beside the rank functions above:

```go
// PlanRow is the issues row's W2 plan columns (task 74), read with the head.
type PlanRow struct {
	Estimate *int
	DueOn    *string // "2006-01-02"; the column is a Postgres date
	Rank     string
}

// SetIssuePlan: UPDATE issues SET estimate = $2, due_on = $3, updated_at = $4
// WHERE id = $1. The caller has decided both values (decision (b)).
func SetIssuePlan(ctx context.Context, tx pgx.Tx, issueID string, estimate *int, dueOn *string, at time.Time) error

// TeamPlanPolicyOf reads task 11's team_work_policies row's W2 columns; the
// defaults (scale none, zero off, stale 0/propose, archive 0) when no row exists.
func TeamPlanPolicyOf(ctx context.Context, q Querier, teamID string) (hubapi.TeamPlanPolicy, error)

// PatchTeamPlanPolicy applies the present fields of p to the row, inserting it
// with the W1 defaults if absent. changed lists the JSON names that differ.
func PatchTeamPlanPolicy(ctx context.Context, tx pgx.Tx, org, teamID string, p hubapi.TeamPlanPolicyPatch, by string, at time.Time) (changed []string, err error)

// LockTeamRank serialises every key write in one team for the transaction:
//   SELECT pg_advisory_xact_lock(hashtextextended('hub.rank:' || $1, 0))
// An advisory lock needs no grant on teams; hub_app holds none for FOR UPDATE there.
func LockTeamRank(ctx context.Context, tx pgx.Tx, teamID string) error

// RankOf reads one issue's key and team under row-level security; found=false
// for an issue in another org or erased.
func RankOf(ctx context.Context, q Querier, issueID string) (rank, teamID string, found bool, err error)

// NextRankBelow / NextRankAbove are the true neighbours (decision (i)), the moving
// issue excluded; "" at that end of the order:
//   SELECT rank FROM issues WHERE team_id = $1 AND id <> $2 AND erased_at IS NULL
//     AND rank > $3 ORDER BY rank LIMIT 1        -- Above: rank < $3 ORDER BY rank DESC
func NextRankBelow(ctx context.Context, q Querier, teamID, exclude, rank string) (string, error)
func NextRankAbove(ctx context.Context, q Querier, teamID, exclude, rank string) (string, error)

// BottomRank is rankAfter(the team's last key), for a create (decision (d)) or a
// move into the team. The caller holds LockTeamRank.
func BottomRank(ctx context.Context, tx pgx.Tx, teamID string) (string, error)

// RankMove is one key a write changed.
type RankMove struct {
	IssueID, ProjectID, From, To string
}

// SetRank: UPDATE issues SET rank = $2, updated_at = $3 WHERE id = $1.
func SetRank(ctx context.Context, tx pgx.Tx, issueID, rank string, at time.Time) error

// RebalanceTeam re-keys the team's whole order with rankSpread, placing issueID
// immediately below afterID ("" is the top), and returns every row whose key
// changed, issueID first. The UNIQUE (team_id, rank) constraint task 74 adds is
// DEFERRABLE INITIALLY DEFERRED, so the mass update cannot collide mid-statement.
func RebalanceTeam(ctx context.Context, tx pgx.Tx, teamID, issueID, afterID string, at time.Time) ([]RankMove, error)
```

`store/work_move.go`:

```go
// MoveIssueRow sets the head's project, team and number under hub.work_cause =
// 'move' (task 74's guard admits the three columns only then), and clears the
// fields decision (f) names. cleared lists what it cleared, in IssueMovedData's words.
//   UPDATE issues SET project_id = $2, team_id = $3, number = $4, rank = $5,
//     feature_id = NULL, parent_id = NULL, cycle_id = CASE WHEN $6 THEN NULL ELSE cycle_id END,
//     sub_state = CASE WHEN $6 THEN NULL ELSE sub_state END,
//     assignee_user_id = CASE WHEN $7 THEN NULL ELSE assignee_user_id END, updated_at = $8
//   WHERE id = $1
// plus, when the teams differ, DELETE FROM issue_labels of labels whose team_id is the source's.
type MoveSpec struct {
	IssueID, ToProjectID, ToTeamID string
	ToNumber                       int
	Rank                           string
	CrossTeam, ClearAssignee       bool
}

func MoveIssueRow(ctx context.Context, tx pgx.Tx, m MoveSpec, at time.Time) (cleared []string, err error)

// InsertIssueAlias records the number the issue is leaving. The primary key
// (project_id, number) and task 74's immutability trigger make it permanent.
//   INSERT INTO issue_aliases (org_id, project_id, number, issue_id, moved_at, moved_by)
func InsertIssueAlias(ctx context.Context, tx pgx.Tx, org, projectID string, number int, issueID, by string, at time.Time) error

// IssueByAlias resolves an old number: found=false when no alias has it.
func IssueByAlias(ctx context.Context, q Querier, projectID string, n int) (issueID string, found bool, err error)

// LastAliasOf is the newest alias of an issue, for IssuePlan.MovedFrom; nil when
// it never moved.
func LastAliasOf(ctx context.Context, q Querier, issueID string) (*hubapi.IssueAlias, error)

// ChildCount counts every issue whose parent_id is issueID, any state (task 78's column).
func ChildCount(ctx context.Context, q Querier, issueID string) (int, error)
```

`issue_aliases`' `moved_at` and `moved_by` are this task's columns; task 74's scope names only
the primary key (`tasks-61-80.md:479`), so they are asked of task 74 (a gap).

#### The routes (`server/work_plan_fields.go`, `server/work_move.go`), in order

```go
func (s *Server) registerWorkPlanFields() {
	s.route("PATCH /v1/issues/{issueId}/plan", s.patchIssuePlan)
	s.route("POST /v1/issues/{issueId}/rank", s.rankIssue)
	s.route("POST /v1/issues/{issueId}/move", s.moveIssue)
}
```

Each runs under task 08's `s.Write` with `readableIssue(…, hubapi.RoleMember, lock=true)` first
(404 `issue_not_found` for unreadable, absent or another org's; 403 for a viewer), and each event
goes through task 13's `emitWork(tx, kind, teamID, projectID, subject, data)`
(`task-13.md:330`), subject the issue id.

**`patchIssuePlan`:**

1. `Decode` the body; `hubapi.ChangeOf[int](req.Estimate)` and `ChangeOf[string](req.DueOn)`; a
   decode error is 400 naming the field.
2. Estimate present and not null: `TeamPlanPolicyOf(head.TeamID)`; `!ValidEstimate(p, v)` → 422
   `estimate_off_scale`, `details.scale` the team's scale, `details.value` v. DueOn present and
   not null: `!ValidDate(v)` → 422 `date_invalid`, `details.field: "dueOn"`.
3. The result per field: absent keeps the stored value, null clears, a value sets. Both results
   equal to the stored ones → 200 with the head, no write, no event (decision (b)).
4. `SetIssuePlan`; one `issue.planned` with `Fields` the changed names in the order `estimate`,
   `dueOn`; 200 with `issueHead`.

**`rankIssue`:**

1. `Decode` (unknown keys, `rank` among them, are 400, task 72). `LockTeamRank(head.TeamID)`.
2. Each named neighbour through `RankOf`: absent, erased, in another team, or the issue itself →
   422 `rank_neighbours`, `details.field` `"after"` or `"before"`. Both named and `after`'s key not
   below `before`'s → the same, field `"before"`.
3. The key (decision (i)): with `after`, `lo = rank(after)`, `hi = NextRankBelow(lo)`; with only
   `before`, `hi = rank(before)`, `lo = NextRankAbove(hi)`; with neither, `lo = ""` and `hi` the
   team's first key (a drop at the top). Then `rankBefore(hi)` when `lo == ""`, `rankAfter(lo)`
   when `hi == ""`, else `rankBetween(lo, hi)`.
4. `ok` true and the key equal to the stored one → 200, no event. `ok` true → `SetRank`, one
   `issue.ranked {rank, rebalance: false}`. `ok` false → `RebalanceTeam(…, afterID)` with
   `afterID` the issue whose key is `lo` ("" for the top); the dragged issue's move gets
   `issue.ranked {rebalance: false}` and each other moved row `issue.ranked {rebalance: true,
   rebalanceOf: <dragged id>}`, each under its own project.
5. 200 with `issueHead`.

**`moveIssue`:**

1. `Decode`; `ToProjectID` equal to the head's project → 400 `bad_request`, `field:
   "toProjectId"`. `ProjectRole(ToProjectID)` (`task-08.md:333`): not found → 404
   `project_not_found`; below member → 403.
2. `head.State` `started` or `in_review` → 422 `state_not_allowed`, `details.from` the state,
   `details.to` the same, `details.cause: "move"` (decision (e)). `ChildCount > 0` → 409
   `children_open`, `details.open` the count (decision (f)).
3. `n = NextIssueNumber(ToProjectID)` (`task-08.md:253`, the gapless counter, so the source's
   number is never re-issued: its counter never goes back). Same team → keep the key; else
   `LockTeamRank(toTeam)` and `BottomRank(toTeam)`. `ClearAssignee` when the assignee is not a
   member of the target team.
4. `SetWorkCause(tx, "move")` (task 74 adds `move` to the causes its guard reads);
   `InsertIssueAlias(sourceProject, head.Number)`; `MoveIssueRow`.
5. One `issue.moved {fromProjectId, fromNumber, toProjectId, toNumber, cleared}` under the target's
   team and project; 200 with `issueHead`, whose `movedFrom` is `LastAliasOf`.

#### The matrix rows (`server/work_matrix_rows_integration_test.go` ⚑), complete

Task 07's fixture gains `OtherProjectID` (project Q in the same org, Linus a member of both teams)
and `RankPeerID` (a second issue in P, so `after` names a real neighbour). `wwrite` is task 07's
member-writes shape; the move needs one shape of its own, because the target project's membership
is the check and the six columns differ:

```go
// wmove: a member of BOTH teams writes; a member of only the source team is 403
// (decision: ProjectRole on the target, task-08.md:333). Columns, in task 07's
// order: viewer, member(source only), team admin, org admin, other team, sync.
func wmove(p, path string, body func(*fixture) any) workRow {
	return workRow{p, row{name: p, method: "POST", path: path, body: body,
		want: [6]int{403, 403, 200, 403, 403, 403}, missing: "issue_not_found"}}
}

	"79": {
		wwrite("PATCH /v1/issues/{issueId}/plan", "PATCH", "/v1/issues/{IssueID}/plan", 200, "issue_not_found",
			func(*fixture) any { return map[string]any{"estimate": 5} }),
		wwrite("POST /v1/issues/{issueId}/rank", "POST", "/v1/issues/{IssueID}/rank", 200, "issue_not_found",
			func(f *fixture) any { return map[string]any{"after": f.RankPeerID} }),
		wmove("POST /v1/issues/{issueId}/move", "/v1/issues/{IssueID}/move",
			func(f *fixture) any { return map[string]any{"toProjectId": f.OtherProjectID} }),
	},
```

`"79"` joins `workMatrixLive`. The org admin is 403 on all three: contract §5.2 keeps an org admin
out of project content. The sync principal is 403 at the route and refused again in the database by
task 74's guard, whose half is `TestTheSyncActorSetsNoPlanField`.

#### Tests to write first, each with the mutant it kills

`store/work_plan_fields_test.go` is a unit file with no build tag (the key functions are pure);
the two integration files carry `//go:build integration`. P and Q are projects of teams TP and TQ;
Linus is a member of both, Grace of TP only. Each test is seen failing before its code.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRankKeysSortAsStrings` (unit) | 10,000 random `rankBetween(lo, hi)` calls → `lo < key < hi` by byte compare, and no key ends in the lowest digit | a key that sorts wrong, or one that cannot be bisected again |
| `TestBottomRankSteps` (unit) | 100,000 `rankAfter` steps from the first key → every key ≤ 8 bytes and strictly increasing | bisection at the bottom, which grows a byte per five creates |
| `TestRebalanceKeepsOrder` (unit) | a spread of 5,000 keys → `rankSpread` returns them in the same order, evenly, each ≤ 8 bytes | a rebalance that reorders the team |
| `TestAnEstimateOffTheScaleIsRefused` | Fibonacci TP: `4` → 422 `estimate_off_scale` with `details.scale: "fibonacci"` and `details.value: 4`; `5` → 200; `0` → 422 until `estimateZero`, then 200; `null` → 200 on scale `none` | any integer accepted; zero always accepted |
| `TestAScaleChangeRewritesNothing` | estimate `5` stored, `PUT /work-policy` to `linear` → the head still reads `5`, no event | a mass edit no person made (decision (a)) |
| `TestADueDateCarriesNoTime` | `"2026-10-01T09:00:00Z"` → 422 `date_invalid` with `details.field: "dueOn"`; `"2026-10-01"` → 200; the response carries no colour field | timestamps accepted; colour on the wire (L-37) |
| `TestAnUnchangedPatchWritesNothing` | `PATCH` with the stored values, then again with both fields absent → 200 twice, `/v1/changes` unchanged | an event per no-op (decision (b)) |
| `TestRankSortsBetweenTheNeighbours` | `rank(C, after: A, before: B)` → a fresh `GET /issues?sort=rank` reads A, C, B; a `"rank"` key in the body → 400 `bad_request` | client-trusted rank; rank not persisted |
| `TestRankTakesOrderNotAdjacency` | A, B, C, D; `rank(D, after: A, before: C)` → D between A and C, B undisturbed | adjacency demanded, which no drag can guarantee |
| `TestFortyInsertsRebalanceOnce` | 40 inserts between one pair → the order holds, every key ≤ 32 bytes, exactly one row carries `issue.ranked{rebalance: true}` per moved issue and the dragged one `false` | a key past 32 characters; a rebalance with no events |
| `TestAMoveRenumbersAndRedirects` | `move(EP-7 in P, Q)` → the same `is-` id, `number` the next in Q; `GET /projects/P/issues/by-number/7` → the moved issue with `movedFrom: {projectId: P, number: 7}`; a new issue in P gets 8, never 7 | 404 after a move; the number reused |
| `TestAMoveClearsOnlyWhatItMust` | revisions, comments, `serves`, relations, priority, estimate and due date byte-equal after the move; team labels, `cycleId`, `subState` and `featureId` cleared; the assignee kept when a member of TQ and cleared when not; `issue.moved.cleared` names each | a move that drops `serves`, or keeps a cycle from another team |
| `TestAMoveIsRefusedWhileBound` | `started` → 422 `state_not_allowed` with `details.cause: "move"`; `in_review` → the same; `landed` and `done` → 200 | a move out from under a live lane (decision (e)) |
| `TestAMoveWithChildrenIsRefused` | one child, any state → 409 `children_open`, `details.open: 1`, nothing written | a parent split across projects (decision (f)) |
| `TestAMoveNeedsBothTeams` | Grace (TP only) → 403; Linus → 200; an unknown target → 404 `project_not_found` | source membership alone |
| `TestClosingABlockerRelaxesIt` | B blocks A; B → `done` → the row reads `relates`, one `issue.blocks_resolved`, actor Linus; a `todo` B keeps `blocks`; A closing leaves B's row alone | the relation deleted; `actor: null`; both directions rewritten (decision (h)) |
| `TestABlockerWithARelatesRowIsDeleted` | a `relates` row already between the two → the `blocks` row is deleted, one event, no primary-key error | a collision on the key |
| `TestEachWriteHasItsEvents` | each accepted write above writes exactly its listed kinds and no other, each under its own team and project | a silent write; a rebalance event under the wrong project |
| `TestTheSyncActorSetsNoPlanField` | `hub.work_actor_kind = 'integration'`, `UPDATE issue_heads SET estimate` → `42501` | the database half of the sync rule |

#### Steps

- [ ] **Step 1: Check the base.** Task 74 merged with the SW.18 head columns, `issue_aliases`,
  the loosened revision and attachment keys (decision (g)), the `UPDATE` grant on `issues` and its
  `hub.work_cause` trigger, and the W1 rank backfill (decision (d)). If any is missing, it is
  added to task 74 in that task's commit, not here; the two gaps are already reported below.
- [ ] **Step 2: The unit tests and the keys.** `store/work_plan_fields_test.go`'s three key tests,
  then `rankBetween`, `rankAfter`, `rankBefore` and `rankSpread`. They run in milliseconds and fix
  the only arithmetic in the task.
- [ ] **Step 3: The integration tests.** The sixteen above and the `"79"` matrix rows with the
  fixture's two new ids. Run them: they fail, each naming its rule.
- [ ] **Step 4: The code, in this order.** `hubapi`'s four kinds; the store's plan write, neighbour
  read and rebalance; `PATCH /plan` and `POST /rank`; task 11's policy half; the move store and
  route with the alias write; task 08's `by-number` alias read and `BottomRank` on create; task
  10's resolved-blocker clause; `server.go`'s one registration line.
- [ ] **Step 5: Green.** This task's suites, then task 08's, 10's, 11's and 13's unchanged (an
  issue with no estimate, no due date, no alias and no `blocks` row takes none of the new paths),
  then the matrix.
- [ ] **Step 6: Commit by pathspec**, every file of *Files*, naming tasks 08, 10, 11 and 13 as
  touched and listing the four kinds in the message.

#### Acceptance

```bash
cd control-plane
go test ./internal/hub/store/ -race -count=1 -run '^Test(RankKeysSortAsStrings|BottomRankSteps|RebalanceKeepsOrder)$' -v 2>&1 | grep -c '^--- PASS'   # 3
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(AnEstimateOffTheScaleIsRefused|AScaleChangeRewritesNothing|ADueDateCarriesNoTime|AnUnchangedPatchWritesNothing|RankSortsBetweenTheNeighbours|RankTakesOrderNotAdjacency|FortyInsertsRebalanceOnce|AMoveRenumbersAndRedirects|AMoveClearsOnlyWhatItMust|AMoveIsRefusedWhileBound|AMoveWithChildrenIsRefused|AMoveNeedsBothTeams|ClosingABlockerRelaxesIt|ABlockerWithARelatesRowIsDeleted|EachWriteHasItsEvents|TheSyncActorSetsNoPlanField)$' \
  2>&1 | grep -c '^--- PASS'                                                     # 16
go test ./internal/hub/... -tags integration -race -count=1                      # every package ok
go test ./internal/hub/server/ -tags integration -race -count=1 -run '^TestWorkMatrix$/79' -v 2>&1 | grep -c '^        --- PASS'   # 18 (3 routes x 6 principals)
grep -rn '"rank"' internal/hub/server/*.go | grep -v _test.go | grep -c 'Decode\|json:'   # 0 (no route reads a rank from a body)
grep -rn "issue_aliases" internal/hub --include=*.go | grep -v _test.go | grep -vc "store/work_move.go\|server/work_issues.go"   # 0
```

Expected: `3`, `16`, every package `ok`, `18`, `0`, `0`. None has been run; no file exists yet
(OD-27), and the two `0`s are the point: the rank is never read from a client and the alias table
is touched in exactly two files.

#### What this task does not claim

- **Not due-soon anywhere a person sees it.** `dueOn` leaves the Hub as a date. The colour, the
  *due soon* band and the Desk row are tasks 89 and 94; the digest is task 81.
- **Not a move between orgs or between teams outside one org.** `toProjectId` must name a project
  of the caller's org; a cross-org move has no owner ruling and no route.
- **Not rank across teams.** The order is one per team, spanning that team's projects. A view that
  mixes two teams sorts by rank within each and by the view's secondary key between them (task 91).
- **Not a measured rebalance.** The 8-byte and 32-byte bounds are arithmetic, and
  `TestFortyInsertsRebalanceOnce` is a correctness test. Nobody has timed a rebalance of a team
  with 10,000 issues, and no budget is claimed for it.
- **Not the desktop.** Drag-to-rank, the estimate control, the due-date picker and the move dialog
  are task 89. A green suite here is evidence about the Hub alone (OD-27).
- **Not task 78's parent rule.** The move refuses a parent outright (decision (f)); moving a whole
  tree is unbuilt and unasked.

#### Corrections to the scope

1. **Task 74 must loosen two W1 foreign keys** (decision (g)) and grant a guarded `UPDATE
   (project_id, team_id, number)` on `issues`. Without it a move is `23503` and no Go code can fix
   it. **Reported as a gap in task 74.**
2. **Task 74 must backfill a rank for every W1 head** (decision (d)) and task 08's create must take
   `store.BottomRank`, or a W1 issue sorts before or after everything at random. **Reported as a
   gap in tasks 74 and 08.**
3. **Rank neighbours are read as order, not adjacency** (decision (i)): the server checks
   `rank(after) < rank(before)`, not that nothing sits between them. A drag in a filtered view can
   never guarantee adjacency.
4. **The matrix file is task 07's** `work_matrix_rows_integration_test.go`, not the scope's
   `server/matrix_integration_test.go` (task 75's correction).
5. **A fourth event kind.** The scope named the rank, plan and move events; the resolved-blocker
   rule needs `issue.blocks_resolved` of its own, so the reader of `/v1/changes` can tell a
   relation a person changed from one a close relaxed.
6. **One extra refusal, `children_open` on a move** (decision (f)), which the scope did not
   anticipate and task 78's parent rule requires.

**Parity rows served:** the Hub halves of **L-14** (rank), **L-16** (estimates), **L-25** (the
resolved blocker), **L-35** (move with redirect), **L-37** (due dates) and **L-103** (the
rebalance). Task 89 closes them with the desktop; task 97 records them.
