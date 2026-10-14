### Task 77: cycles `cy-`: cadence, the janitor's creation and rollover, scope log, iCal; closure proposals and archive

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement; the largest of the W2
route tasks, and the first to split if it runs over: the iCal feed can go to its own task).
**Stream:** WH. **dependsOn:** 74, after a sync-merge of `feat/hub` (rule H1). **Seams:** consumes
**SW.18** (task 72's `Cycle`, `Cadence`, `ScopeEntry`, `ClosureProposal`, `CalendarToken`,
`TeamPlanPolicy`, `CodeCycleNotFound`, `CodeCycleOverlap`, `CodeProposalNotFound`) and **SW.4**
(task 10's state move); adds one cause to SW.4's `MoveAllowed`, one field to SW.18's `IssuePlan`,
and eight event kinds to SW.5, all additive.

A cycle is a team's time box: two weeks, numbered, with a cooldown between. Linear makes them
automatically, rolls unfinished work into the next, and draws a burn-up graph. We make and roll
them the same way, with the **janitor** as the only thing that acts on a clock, every act an event
with `actor: null` that names the policy's author (05 §3.6, `05-design.md:811-812`). We do not draw
a burn-up: a prediction is banned (AX anti-pattern 2), so the cycle shows a **scope log** of plain
facts (*3 added on day 4, by Linus*) and the desktop shows convergence per criterion (task 87).
The same janitor pass **proposes** closing stale issues as one batch on each assignee's Desk rather
than closing them silently (L-11, AX §7), archives closed work after a while (L-12), and expires
drafts (L-33).

**Anchors, re-read on 2026-09-26.** In the Hub worktree at `6463e568` (read, not merged here): the
janitor's contract (`hub-contract.md:2229` onward, §12.5) and Hub task 24's `RunJanitor`, `Sweep`,
its per-org transaction and its numbered passes 1-4 (`task-24.md:70-111`, Hub plan parts); the Hub
plan's file row (`2026-09-23-phase-h-hub.md:306`); *no browsers* and bearer-only `/v1`
(`hub-contract.md:36-39`); roles (`:324-333`). In this branch's parts (**UNVERIFIED until
merge**): task 72's records (`task-72.md:526-579`), `TeamPlanPolicy` (`:412-418`), `StaleMode`
(`:168-175`), `ValidDate` (`:285-289`), codes (`:353`, `:357`, `:366`); task 10's route
(`task-10.md:402-435`); task 13's registry (`task-13.md:250-270`). **Task 74's part is not written
at this writing**: its `cycles`, `cadences`, `cycle_scope` and `closure_proposals` columns are the
scope's (`tasks-61-80.md:474-480`), **UNVERIFIED**.

#### Decisions the scope left open, made here

**(a) Dates are UTC calendar days, both ends inclusive**, and "today" is the janitor's
`cfg.Now()` in UTC. A cycle of `lengthDays: 14` starting Monday 2026-10-05 ends Sunday 2026-10-18;
with `cooldownDays: 0` the next starts 2026-10-19. A team in Sydney sees its cycle turn at 11:00 or
10:00 local; per-team time zones are a later refinement, stated in *does not claim*.

**(b) One function makes cycles, called by two callers.** `store.EnsureCyclesAhead(team, today)`
tops the team up to `ahead` cycles that are `upcoming` or `current`, each numbered `max(number)+1`
under the org's audit head lock (so numbers are gapless per team, and task 74's unique
`(team_id, number)` is the second guard). `PUT /cadence` calls it in its own transaction, so the
admin sees the cycles at once, as the person; the janitor calls it every sweep, as `null`. The
first cycle made is the first whose `endsOn ≥ today` in the cadence's phase from `firstStartsOn`,
so a past `firstStartsOn` aligns the phase and makes no cycles in the past.

**(c) A cadence change keeps every cycle an issue references.** `PUT /cadence` deletes, highest
number first, the `upcoming` cycles that no issue head references and that have not started, and
stops at the first one that is referenced (a suffix only, so numbers stay gapless); then
`EnsureCyclesAhead` re-makes them from the new cadence. Referenced cycles keep their dates; an admin
moves them by `PATCH`. `lengthDays: 0` turns cycles off: no new ones are made, existing ones run
out.

**(d) The janitor's state walk.** Each sweep, per team, in order: `upcoming` → `current` when
`startsOn ≤ today`; `current` → `closed` when `endsOn < today`, then the rollover; then
`EnsureCyclesAhead`. During a cooldown no cycle is `current`. A cycle is `current` for exactly its
own dates; there is at most one, because a `PATCH` may not overlap (`422 cycle_overlap`).

**(e) Rollover** (L-60). At closing, every issue head in the cycle whose state is not `done`,
`cancelled` or `duplicate` moves to the next cycle by number (the lowest `upcoming`), or to none
when there is none (cycles turned off). One `issue.cycle_set {from, to, rollover: true}` event
**per issue**, `actor: null`, and one `ScopeEntry` `removed` on the closing cycle with `by: null`
(task 72's `ScopeEntry.By` comment). No entry is written on the next cycle, which has not started.

**(f) Stale issues** (L-11). An issue is **stale** when its state is `triage`, `backlog` or `todo`
(never a fact state: live work is not stale), the team's `staleAfterDays > 0`, and the later of
its `updated_at` and the `answered_at` of the last proposal naming it is older than that many
days. So a person who answers *keep* is not asked again for another full period, with no extra
column. `staleMode: propose`: at most **one open proposal per (team, assignee)**; an assignee who
has one open gets nothing new until they answer it. Unassigned stale issues go to the policy's
author. `staleMode: auto`: the janitor moves each to `cancelled` with cause `stale_policy` (the new
cause), reason *"untouched for 90 days; policy set by Grace"*, `actor: null`.

**(g) Answering closes the named ids only, as the assignee.** `AnswerClosureRequest{close}` must
be a subset of the proposal's `issueIds` (else 422 `bad_request`, `field: "close[n]"`). Each named
issue still stale-eligible (open, not moved since) is moved to `cancelled` by task 10's route
logic with cause `hand` and reason *"closed from a stale proposal"*, **written as the assignee**;
an issue that moved in the meantime is skipped and absent from `closed`. `close: []` is a valid
*keep all*. A proposal is answered once; a second answer is 409 `state_conflict` (task 10's code).

**(h) Archive** (L-12). The janitor sets `archived_at` on an issue head that has been `done`,
`cancelled` or `duplicate` for `archiveAfterDays` (0: never). Archived issues leave the default
lists (task 08's list gains `includeArchived`, default false) and **stay searchable** (the search
route, `GET /v1/issues/search`, `task-07.md:351`, reads no `archived_at` and is not changed). A reopen clears it. This needs `issue_heads.archived_at` and
`IssuePlan.ArchivedAt`, neither in task 72 or task 74's scope: both are added here, additively.

**(i) The iCal feed is outside `/v1`, and UNVERIFIED pending the Hub owner's review.** Every
`/v1` route needs `Authorization: Bearer` (`hub-contract.md:36-37`), which calendar clients cannot
send. So the feed is `GET /cal/{feedToken}.ics`, where the token is minted by `POST
/v1/me/calendar-token`, shown once, stored as its SHA-256 only, and revoked by `DELETE
/v1/me/calendar-token` (a new mint also revokes the old one: one live token per person). The token
reads `<orgId>.<secret>`, 32 bytes of `crypto/rand` in base32 without padding, so the route can
set `hub.org_id` from the token's first half **before** it looks the hash up, and row-level
security applies (no new owner-run function beside the three of §10.4). The feed carries **only**
cycle numbers, names and dates of the person's teams: no issue, no title, no person. It still
refuses a request carrying `Origin` (the *no browsers* rule, `:38-39`). The fallback, if the Hub
owner refuses a route outside `/v1`, is `GET /v1/teams/{teamId}/cycles.ics`, bearer-only, the same
body, which the desktop downloads and the person imports by hand; both are in this task so the
owner's answer removes one, not adds one.

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/migrate/sql/NNNN_work_plan.sql` ⚑ (task 74's) | appended: `calendar_tokens`, `issue_heads.archived_at`, the janitor's widened guard |
| `control-plane/internal/hub/store/work_cycles.go` | new: cadence, cycles, scope log, the ensure-ahead and roll functions |
| `control-plane/internal/hub/store/work_closure.go` | new: stale reads, proposals, archive, token hashes |
| `control-plane/internal/hub/server/work_cycles.go` | new: the ten routes |
| `control-plane/internal/hub/server/janitor.go` ⚑ (Hub task 24's) | passes 5-9 and five `SweepReport` fields |
| `control-plane/internal/hub/server/work_states.go` ⚑ (task 10's) | the `started` move joins the current cycle; `stale_policy` accepted from the janitor only |
| `control-plane/internal/hub/server/work_issues.go` ⚑ (task 08's) | `includeArchived` on the list; a reopen clears `archived_at` |
| `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's, task 10's additions) | `CauseStalePolicy` and its `MoveAllowed` rows |
| `control-plane/internal/hub/hubapi/work_plan.go` ⚑ (task 72's) | `IssuePlan.ArchivedAt`, one field |
| `control-plane/internal/hub/hubapi/work_events.go` ⚑ (task 13's) | eight kinds |
| `control-plane/internal/hub/server/work_cycles_integration_test.go`, `janitor_work_integration_test.go` | new: the tests below |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ | `workMatrixRows["77"]`; `"77"` in `workMatrixLive` |
| `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ (H16b's) | the fixture gains `CycleID` and `ProposalID` |
| `control-plane/internal/hub/server/server.go` ⚑ | `s.registerWorkCycles(mux)`, and the `/cal/` pattern beside `/healthz` |

#### The migration, appended to task 74's file (complete)

```sql
-- Task 77: calendar tokens (decision (i)), issue archive (decision (h)), the janitor's guard.
CREATE TABLE hub.calendar_tokens (
  org_id      text        NOT NULL REFERENCES hub.orgs(id),
  user_id     text        NOT NULL,
  token_hash  bytea       NOT NULL,              -- sha256 of the whole token; the token is never stored
  created_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  PRIMARY KEY (org_id, token_hash)
);
CREATE UNIQUE INDEX calendar_tokens_one_live ON hub.calendar_tokens (org_id, user_id) WHERE revoked_at IS NULL;
ALTER TABLE hub.calendar_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON hub.calendar_tokens TO hub_app
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT, UPDATE (revoked_at) ON hub.calendar_tokens TO hub_app;

ALTER TABLE hub.issue_heads ADD COLUMN archived_at timestamptz;
CREATE INDEX issue_heads_unarchived ON hub.issue_heads (project_id, state) WHERE archived_at IS NULL;
GRANT UPDATE (archived_at) ON hub.issue_heads TO hub_app;
```

The policy's shape is the Hub's own (`hub-contract.md:1906-1921`); task 06's grant-list test gains
the table (`TestWorkGrantsAreExactlyTheList`, `task-10.md:78`).

The **janitor's widened guard**. Task 74's scope lets the janitor (`hub.work_actor_kind =
'janitor'`, actor null) write **only** `cycle_id` (`tasks-61-80.md:484-485`). This task's passes
also write `archived_at`, and, under `staleMode: auto`, `state` to `cancelled` from `triage`,
`backlog` or `todo` with cause `stale_policy`. The clause, added to task 74's head guard function
(its name is task 74's, **UNVERIFIED**):

```sql
  -- inside task 74's issue_heads guard, the janitor branch, replacing "only cycle_id":
  IF current_setting('hub.work_actor_kind', true) = 'janitor' THEN
    IF (NEW.state IS DISTINCT FROM OLD.state) AND NOT (
         NEW.state = 'cancelled' AND OLD.state IN ('triage', 'backlog', 'todo')
         AND current_setting('hub.work_cause', true) = 'stale_policy') THEN
      RAISE EXCEPTION 'hub: work: the janitor may only cancel a stale open issue' USING ERRCODE = '42501';
    END IF;
    IF (NEW.estimate, NEW.due_on, NEW.rank, NEW.parent_id, NEW.feature_id, NEW.sub_state)
       IS DISTINCT FROM (OLD.estimate, OLD.due_on, OLD.rank, OLD.parent_id, OLD.feature_id, OLD.sub_state) THEN
      RAISE EXCEPTION 'hub: work: the janitor writes cycle_id, archived_at and a stale close only' USING ERRCODE = '42501';
    END IF;
  END IF;
```

`stale_policy` sets no fact state and no `done`, so task 07's W1 rule (SW.3) is unchanged.

#### The wire

```text
PUT    /v1/teams/{teamId}/cadence          admin      200 Cadence              PutCadenceRequest
GET    /v1/teams/{teamId}/cadence          viewer     200 Cadence              (lengthDays 0 when never set)
GET    /v1/teams/{teamId}/cycles           viewer     200 Page[Cycle]          ?state=upcoming|current|closed, by number
PATCH  /v1/cycles/{cycleId}                admin      200 Cycle                PatchCycleRequest
PUT    /v1/issues/{issueId}/cycle          member     200 IssueHead            SetCycleRequest
GET    /v1/cycles/{cycleId}/scope-log      viewer     200 Page[ScopeEntry]     oldest first
GET    /v1/closure-proposals?mine=true     signed in  200 Page[ClosureProposal] open first; mine=true is required in W2
POST   /v1/closure-proposals/{id}/answer   assignee   200 ClosureProposal      AnswerClosureRequest
POST   /v1/me/calendar-token               signed in  201 CalendarToken        {}; revokes the previous token
DELETE /v1/me/calendar-token               signed in  204
GET    /cal/{feedToken}                    the token  200 text/calendar        outside /v1; UNVERIFIED (decision (i))
GET    /v1/teams/{teamId}/cycles.ics       viewer     200 text/calendar        the fallback
```

`GET /v1/teams/{teamId}/cadence` is not in the scope's list; a settings screen (task 87) cannot
show a cadence it cannot read, so it is added. The request bodies are task 72's block 6; shown so
the part stands alone, task 72 winning on any spelling:

```go
type PutCadenceRequest struct {
	LengthDays    int    `json:"lengthDays"`    // 0 (off) or 7-56
	CooldownDays  int    `json:"cooldownDays"`  // 0-14
	Ahead         int    `json:"ahead"`         // 1-15
	FirstStartsOn string `json:"firstStartsOn"` // ValidDate
}

// PatchCycleRequest: tri-state (task 72 decision (d)); name may clear, dates may not.
type PatchCycleRequest struct {
	Name     json.RawMessage `json:"name,omitempty"`
	StartsOn json.RawMessage `json:"startsOn,omitempty"`
	EndsOn   json.RawMessage `json:"endsOn,omitempty"`
}

type SetCycleRequest struct {
	CycleID *string `json:"cycleId"` // a cycle of the issue's team, not closed; null clears
}

type AnswerClosureRequest struct {
	Close []string `json:"close"` // a subset of the proposal's issueIds; [] keeps them all
}
```

#### The `hubapi` additions (complete)

```go
// hubapi/work.go ⚑, beside task 10's six causes (SW.4, additive).
// CauseStalePolicy is the janitor's close of a stale open issue under staleMode auto. No
// person sends it: the route refuses it from a caller (400 naming cause).
const CauseStalePolicy StateCause = "stale_policy"
```

`MoveAllowed` gains exactly three rows, `(triage|backlog|todo, cancelled, stale_policy)`, and
task 10's `TestMoveAllowedIsTheTable` literal gains them (a test edit in task 10's file, stated).

```go
// hubapi/work_plan.go ⚑, IssuePlan's last field (SW.18, additive after its freeze).
	ArchivedAt *string `json:"archivedAt"` // set by the janitor (L-12); a reopen clears it
```

Task 72's `workExamples` `IssueHead` row gains `,"archivedAt":null` at its end, so
`TestWorkFieldNamesMatchTheExamples` still names every key.

#### The eight event kinds, and one field on task 10's (complete)

Every janitor event has `actor: null` and names the policy's author by id in `data` (05 §3.6).

```go
const (
	EventTeamCadenceSet   = "team.cadence_set"
	EventCycleCreated     = "cycle.created"
	EventCycleUpdated     = "cycle.updated"
	EventCycleClosed      = "cycle.closed"
	EventIssueCycleSet    = "issue.cycle_set"
	EventClosureProposed  = "closure.proposed"
	EventClosureAnswered  = "closure.answered"
	EventIssueArchived    = "issue.archived"
)

type TeamCadenceSetData struct {
	LengthDays   int64 `json:"lengthDays"`
	CooldownDays int64 `json:"cooldownDays"`
	Ahead        int64 `json:"ahead"`
}
type CycleCreatedData struct {
	CycleID        string `json:"cycleId"`
	Number         int64  `json:"number"`
	StartsOn       string `json:"startsOn"`
	EndsOn         string `json:"endsOn"`
	PolicyAuthorID string `json:"policyAuthorId"` // "" when a person made it
}
type CycleUpdatedData struct {
	CycleID string `json:"cycleId"`
	Fields  string `json:"fields"` // comma-joined: name,startsOn,endsOn
}
type CycleClosedData struct {
	CycleID        string `json:"cycleId"`
	Number         int64  `json:"number"`
	RolledOver     int64  `json:"rolledOver"`
	PolicyAuthorID string `json:"policyAuthorId"`
}
type IssueCycleSetData struct {
	From           string `json:"from"` // cy-… or ""
	To             string `json:"to"`
	Rollover       bool   `json:"rollover"`
	PolicyAuthorID string `json:"policyAuthorId"` // "" when a person moved it
}
type ClosureProposedData struct {
	ProposalID     string `json:"proposalId"`
	Count          int64  `json:"count"`
	PolicyAuthorID string `json:"policyAuthorId"`
}
type ClosureAnsweredData struct {
	ProposalID string `json:"proposalId"`
	Closed     int64  `json:"closed"`
	Kept       int64  `json:"kept"`
}
type IssueArchivedData struct {
	PolicyAuthorID string `json:"policyAuthorId"`
}
```

Task 10's `IssueStateData` gains `PolicyAuthorID string \`json:"policyAuthorId"\``, `""` for every
cause but `stale_policy`. Registry rows: the three `cycle.*` and `team.cadence_set` are
`ScopeTeam, VisibleToReaders`; `issue.cycle_set` and `issue.archived` are `ScopeProject,
VisibleToReaders`; `closure.proposed` and `closure.answered` are `ScopeTeam, SubjectOnly` with
`Subject` the assignee, because a proposal is one person's Desk item.

#### `store/work_cycles.go` and `store/work_closure.go` (signatures complete)

```go
package store

// Inside the caller's transaction, after hub.org_id and the org's audit head lock (task 11).
// "day" arguments are UTC calendar days as time.Time at 00:00 UTC.

func CadenceOf(ctx context.Context, q Querier, teamID string) (hubapi.Cadence, error) // LengthDays 0 when none
func PutCadence(ctx context.Context, tx pgx.Tx, org string, c hubapi.Cadence, by string, at time.Time) (changed bool, err error)

// DropUnreferencedSuffix deletes, highest number first, upcoming cycles of the team that have
// not started and that no issue head references, stopping at the first referenced one
// (decision (c)). It answers the numbers dropped.
func DropUnreferencedSuffix(ctx context.Context, tx pgx.Tx, teamID string, today time.Time) ([]int, error)

// EnsureCyclesAhead tops the team up to Ahead cycles that are upcoming or current (decision
// (b)); numbers are max(number)+1. It answers the cycles made, in number order. A cadence with
// LengthDays 0 makes none.
func EnsureCyclesAhead(ctx context.Context, tx pgx.Tx, org, teamID string, today time.Time) ([]hubapi.Cycle, error)

// NextCycleDates is the pure phase arithmetic EnsureCyclesAhead uses: the first cycle of the
// cadence whose EndsOn >= after, starting no earlier than notBefore. Exported for the unit test.
func NextCycleDates(c hubapi.Cadence, after, notBefore time.Time) (startsOn, endsOn time.Time)

func CycleByID(ctx context.Context, q Querier, id string) (c hubapi.Cycle, ok bool, err error)
func ListCycles(ctx context.Context, q Querier, teamID string, state *hubapi.CycleState, pr hubapi.PageRequest) (hubapi.Page[hubapi.Cycle], error)
func CurrentCycle(ctx context.Context, q Querier, teamID string) (c hubapi.Cycle, ok bool, err error)

// Neighbours are the cycles numbered n-1 and n+1, for the overlap check.
func Neighbours(ctx context.Context, q Querier, teamID string, number int) (prev, next *hubapi.Cycle, err error)
func PatchCycle(ctx context.Context, tx pgx.Tx, id string, name *string, startsOn, endsOn time.Time, at time.Time) (changed []string, err error)

// StartDueCycles and CloseDueCycles are the janitor's walk (decision (d)).
func StartDueCycles(ctx context.Context, tx pgx.Tx, teamID string, today time.Time) ([]hubapi.Cycle, error)
func CloseDueCycles(ctx context.Context, tx pgx.Tx, teamID string, today time.Time) ([]hubapi.Cycle, error)

// OpenIssuesIn answers the heads in the cycle whose state is not done, cancelled or duplicate.
func OpenIssuesIn(ctx context.Context, q Querier, cycleID string) ([]IssueRef, error)
type IssueRef struct{ ID, ProjectID string }

// SetIssueCycle: UPDATE issue_heads SET cycle_id = $2, updated_at = $3 WHERE issue_id = $1.
// The janitor calls it with bumpUpdated false, so a rollover does not make an issue look touched
// (decision (f) reads updated_at).
func SetIssueCycle(ctx context.Context, tx pgx.Tx, issueID, cycleID string, at time.Time, bumpUpdated bool) error

// AppendScope writes one ScopeEntry; by "" is the janitor (null on the wire).
func AppendScope(ctx context.Context, tx pgx.Tx, org, cycleID, issueID string, ch hubapi.ScopeChange, by string, at time.Time) error
func ScopeLog(ctx context.Context, q Querier, cycleID string, pr hubapi.PageRequest) (hubapi.Page[hubapi.ScopeEntry], error)
```

```go
// work_closure.go

// StaleIssues answers the team's stale heads by assignee ("" for unassigned), each list by id,
// under decision (f)'s rule; assignees with an open proposal are left out.
func StaleIssues(ctx context.Context, q Querier, teamID string, p hubapi.TeamPlanPolicy, today time.Time) (map[string][]string, error)

func InsertProposal(ctx context.Context, tx pgx.Tx, org, teamID string, p hubapi.ClosureProposal) error
func ProposalByID(ctx context.Context, q Querier, id string) (p hubapi.ClosureProposal, teamID string, ok bool, err error)
func MyProposals(ctx context.Context, q Querier, userID string, pr hubapi.PageRequest) (hubapi.Page[hubapi.ClosureProposal], error)

// AnswerProposal sets answered_at and closed once; answered is false when it was already set.
func AnswerProposal(ctx context.Context, tx pgx.Tx, id string, closed []string, at time.Time) (answered bool, err error)

// ArchiveDue sets archived_at on closed heads older than the team's ArchiveAfterDays; it
// answers the ids archived.
func ArchiveDue(ctx context.Context, tx pgx.Tx, teamID string, days int, today time.Time) ([]IssueRef, error)

// ExpireDrafts deletes drafts whose expires_at < now (task 80's table); it answers the count.
func ExpireDrafts(ctx context.Context, tx pgx.Tx, now time.Time) (int, error)

// The calendar token: InsertToken revokes the live one in the same statement pair.
func InsertToken(ctx context.Context, tx pgx.Tx, org, userID string, hash []byte, at time.Time) error
func RevokeToken(ctx context.Context, tx pgx.Tx, userID string, at time.Time) (revoked bool, err error)
func TokenOwner(ctx context.Context, q Querier, hash []byte) (userID string, ok bool, err error) // live only
```

`ProposalByID` answers the team so the route can check the caller's membership; `MyProposals`
reads by `assignee_id`, so a proposal is never listed to anyone else. The draft table is task 80's
(`tasks-61-80.md:478`, task 74's migration); if task 80 names its expiry column differently, the
query follows it.

#### The routes (`server/work_cycles.go`), in order

```go
func (s *Server) registerWorkCycles(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/teams/{teamId}/cadence", s.putCadence)
	mux.HandleFunc("GET /v1/teams/{teamId}/cadence", s.getCadence)
	mux.HandleFunc("GET /v1/teams/{teamId}/cycles", s.listCycles)
	mux.HandleFunc("GET /v1/teams/{teamId}/cycles.ics", s.teamCyclesICS)
	mux.HandleFunc("PATCH /v1/cycles/{cycleId}", s.patchCycle)
	mux.HandleFunc("PUT /v1/issues/{issueId}/cycle", s.putIssueCycle)
	mux.HandleFunc("GET /v1/cycles/{cycleId}/scope-log", s.scopeLog)
	mux.HandleFunc("GET /v1/closure-proposals", s.myProposals)
	mux.HandleFunc("POST /v1/closure-proposals/{proposalId}/answer", s.answerProposal)
	mux.HandleFunc("POST /v1/me/calendar-token", s.mintCalendarToken)
	mux.HandleFunc("DELETE /v1/me/calendar-token", s.revokeCalendarToken)
}

// Registered beside /healthz, outside the /v1 authentication chain (decision (i)).
func (s *Server) registerCalendarFeed(mux *http.ServeMux) {
	mux.HandleFunc("GET /cal/{feedToken}", s.calendarFeed)
}
```

Each refusal rolls the transaction back and appends nothing. `answerProposal` and
`mintCalendarToken` are `POST`s whose replies hold no issue text, so `server.Write`; the token is
**not** replayable (a stored reply would keep the secret), so `mintCalendarToken` refuses an
`Idempotency-Key` header with 400 `field: "Idempotency-Key"`.

**`putCadence`** (admin): decode; `lengthDays` 0 or 7-56, `cooldownDays` 0-14, `ahead` 1-15,
`firstStartsOn` `ValidDate`, each else 400 naming it. `PutCadence`; unchanged → 200, no event.
Else `DropUnreferencedSuffix`, then `EnsureCyclesAhead` with today; events `team.cadence_set`, then
one `cycle.created` per cycle made (`policyAuthorId: ""`, the admin is the actor). 200 `Cadence`.

**`listCycles`**, **`getCadence`**, **`scopeLog`** (viewer of the team; a cycle outside the
caller's teams → 404 `cycle_not_found`).

**`patchCycle`** (admin of the cycle's team): tri-state decode; a `closed` cycle → 422
`bad_request`, `field: "cycleId"`; dates `ValidDate`, `startsOn ≤ endsOn`; `Neighbours` →
`startsOn ≤ prev.endsOn` or `endsOn ≥ next.startsOn` → 422 `cycle_overlap`, `details.cycleId` the
neighbour; a `current` cycle may not move its `startsOn` past today (it would stop being current
by edit). A pause is a wider gap, not a state (L-65). `cycle.updated {cycleId, fields}`.

**`putIssueCycle`** (member): `readableIssue(…, RoleMember, lock=true)`; the cycle of the issue's
team, not `closed` → else 404 `cycle_not_found` / 422 `bad_request` `field: "cycleId"`; equal → 200,
no event. `SetIssueCycle(…, bumpUpdated: true)`; for each of the old and the new cycle whose
`startsOn ≤ today`, `AppendScope` (`removed` / `added`) **by the caller** (L-63). Event
`issue.cycle_set {from, to, rollover: false, policyAuthorId: ""}`. 200 `IssueHead`.

**`myProposals`**: `mine` absent or not `true` → 400 `field: "mine"`; `MyProposals(caller)`.

**`answerProposal`**: `ProposalByID`; not the caller's (any other person, including admins) → 404
`proposal_not_found` (no existence leak); answered → 409 `state_conflict`; `close` ⊄ `issueIds` →
422 naming `close[n]`. For each id in `close`, in order: `readableIssue(…, lock=true)` as the
caller; still `triage`, `backlog` or `todo` → task 10's move to `cancelled`, cause `hand`, reason
*"closed from a stale proposal"*, written as the caller, with task 10's `issue.state` event; else
skipped. `AnswerProposal` with the ids closed; `closure.answered {proposalId, closed, kept}`.

**`mintCalendarToken`**: `<orgId>.` + 32 random bytes in base32; `InsertToken(sha256(token))`.
201 `CalendarToken{token, feedPath: "/cal/<token>.ics", createdAt}`. No event: a token is a
credential, not a record anyone reads (Hub task 24's rule for idempotency keys, `task-24.md:97-98`).
**`revokeCalendarToken`**: `RevokeToken`; 204 whether or not one was live.

**`calendarFeed`**: an `Origin` header → 403 `forbidden_origin` (`hub-contract.md:38`); the path
must end `.ics` → else 404; split the token at its first `.`; the org part not `ValidID("org", …)`
→ 404. In one read transaction: `set_config('hub.org_id', org, true)`; `TokenOwner(sha256(token))`
→ none → **404** (a revoked or unknown token is indistinguishable from none); the owner's teams;
their cycles not older than 90 days past `endsOn`. The body is RFC 5545 `VCALENDAR` with one
all-day `VEVENT` per cycle: `UID:<cycleId>@panacea-app`, `DTSTART;VALUE=DATE`, `DTEND;VALUE=DATE`
(the day after `endsOn`, RFC 5545's exclusive end), `SUMMARY:<team name> cycle <number>` plus
`: <name>` when named. No issue, no person, no description. `Cache-Control: private, max-age=900`.
**`teamCyclesICS`** is the same body for one team, bearer-authenticated, viewer.

#### The janitor's W2 passes (`server/janitor.go` ⚑, after Hub task 24's four)

Inside each org's transaction, with `hub.work_actor_kind = 'janitor'` set after `hub.org_id` (task
07's setting), for each team of the org, in this order:

```go
//   5. cycles: StartDueCycles; CloseDueCycles, and for each closed cycle the rollover of decision
//      (e): per open issue, SetIssueCycle(next or "", bumpUpdated false), AppendScope(closing,
//      removed, by ""), Emit issue.cycle_set {rollover: true, policyAuthorId}; then Emit
//      cycle.closed {rolledOver}; then EnsureCyclesAhead, Emit cycle.created per cycle made.
//      policyAuthorId is the cadence's updatedBy.
//   6. stale: StaleIssues. staleMode propose → per assignee one InsertProposal and one Emit
//      closure.proposed {count, policyAuthorId}, Subject the assignee. staleMode auto → per issue
//      SetWorkCause(stale_policy), MoveIssueState(cancelled, reason), Emit issue.state with
//      policyAuthorId. policyAuthorId is the team policy's updatedBy.
//   7. archive: ArchiveDue; Emit issue.archived {policyAuthorId} per issue.
//   8. drafts (once per org, not per team): ExpireDrafts. No event (a draft is private; task 80).
//   9. calendar tokens: nothing to sweep (a revoked token is kept, for the audit trail).
// SweepReport gains: CyclesMade, CyclesClosed, RolledOver, Proposed, AutoClosed, Archived,
// DraftsExpired.
```

Every janitor `Emit` has `Actor: nil`. A team whose `staleAfterDays` or `archiveAfterDays` is 0
skips that pass. A pass that fails rolls back **that org's** transaction and is logged; the next
org still runs (Hub task 24's per-org transaction).

#### The two insertions in task 10's route (`server/work_states.go` ⚑)

- **In task 10's step 2:** a caller's `cause: stale_policy` → 400 `bad_request`, `field: "cause"`
  (only the janitor sends it, and it does not use the route).
- **In task 10's step 5, a `lane_bound` move into `started`:** when `head.CycleID` is null and
  `CurrentCycle(team)` exists, `SetIssueCycle(…, bumpUpdated: true)`, `AppendScope(added, by the
  caller)`, and `issue.cycle_set {from: "", to, rollover: false}` after the `issue.state` event,
  **written as the person** whose lane started it (L-61, skeleton rule 2). An issue already in a
  cycle keeps it.

#### The matrix rows (`server/work_matrix_rows_integration_test.go` ⚑), complete

H16b's fixture gains `CycleID` (Grace sets a cadence 14/0/1 from today, so one `current` cycle
exists) and `ProposalID` (a proposal to Linus, inserted by the store as the janitor would, since
no route makes one). The answer row's cells differ from every helper: only the assignee may
answer, so its `want` is written out.

```go
	"77": {
		wadmin("PUT /v1/teams/{teamId}/cadence", "PUT", "/v1/teams/{TeamID}/cadence", 200, "team_not_found",
			func(*fixture) any {
				return map[string]any{"lengthDays": 14, "cooldownDays": 0, "ahead": 2, "firstStartsOn": "2026-10-05"}
			}),
		wread("GET /v1/teams/{teamId}/cadence", "/v1/teams/{TeamID}/cadence", "team_not_found"),
		wread("GET /v1/teams/{teamId}/cycles", "/v1/teams/{TeamID}/cycles", "team_not_found"),
		wread("GET /v1/teams/{teamId}/cycles.ics", "/v1/teams/{TeamID}/cycles.ics", "team_not_found"),
		wadmin("PATCH /v1/cycles/{cycleId}", "PATCH", "/v1/cycles/{CycleID}", 200, "cycle_not_found",
			func(*fixture) any { return map[string]any{"name": "Uploads"} }),
		wwrite("PUT /v1/issues/{issueId}/cycle", "PUT", "/v1/issues/{IssueID}/cycle", 200, "issue_not_found",
			func(f *fixture) any { return map[string]any{"cycleId": f.CycleID} }),
		wread("GET /v1/cycles/{cycleId}/scope-log", "/v1/cycles/{CycleID}/scope-log", "cycle_not_found"),
		{"POST /v1/closure-proposals/{proposalId}/answer", row{name: "answer", method: "POST",
			path: "/v1/closure-proposals/{ProposalID}/answer", body: func(*fixture) any { return map[string]any{"close": []any{}} },
			want:     [6]int{404, 200, 404, 404, 404, 404}, // Linus is the assignee; nobody else learns it exists
			wantCode: map[column]string{colV: "proposal_not_found", colA: "proposal_not_found", colO: "proposal_not_found",
				colN: "proposal_not_found", colX: "proposal_not_found"}}},
	},
```

`GET /v1/closure-proposals`, `POST` and `DELETE /v1/me/calendar-token` are per-person routes with
no team to test against; they answer 200/201/204 in every signed-in column and are covered by the
integration tests, as H16b treats `/v1/me` (**UNVERIFIED**: H16b's handling of `/v1/me` rows is
not re-read here). `/cal/{feedToken}` is outside `/v1` and outside the matrix; its test is
`TestTheFeedHoldsNoIssueAndRevokes`. **The sync principal's** column is task 150's; its database
half is `TestTheSyncActorSetsNoCycle`.

#### Tests to write first, each with the mutant it kills

`store/work_cycles_test.go` (unit, no database) and `server/work_cycles_integration_test.go` plus
`server/janitor_work_integration_test.go` (`//go:build integration`). The janitor tests drive
`Sweep` with `cfg.Now` fixed, never a sleep. Each is seen failing before the code it covers.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestNextCycleDatesKeepThePhase` (unit) | 14/0 from Mon 2026-10-05, after 2026-10-21 → 2026-10-19..2026-11-01; 14/7 → 2026-10-26..2026-11-08 | an off-by-one inclusive end; cooldown ignored |
| `TestACadenceMakesGaplessCycles` | `PUT` 14/0/3 from Monday → cycles 1, 2, 3, contiguous dates, `cycle.created` ×3 as Grace; a second team's cadence → its own 1, 2, 3 | numbers from a global sequence |
| `TestASecondSweepIsANoOp` | two `Sweep`s at one `Now` → the same cycles, events and report counts zero on the second; two concurrent `Sweep`s → one reports `Locked: false` | duplicate cycles (no lock, or no idempotence) |
| `TestRolloverSkipsClosedIssues` | cycle 1 holds a `todo`, a `started`, a `done` and a `cancelled` issue; `Sweep` the day after `endsOn` → the two open move to cycle 2, one `issue.cycle_set` each with `actor: null`, `rollover: true` and `policyAuthorId` Grace; the closed two stay; `cycle.closed.rolledOver: 2` | closed issues rolled; one bulk event; an actor set |
| `TestTheScopeLogIsWhoAndWhen` | on day 4 of cycle 1, Linus adds EP-150 → a scope entry `added` by Linus at day 4; removing it → `removed` by Linus; an issue added to an upcoming cycle writes none | scope log derived from current membership |
| `TestStartedJoinsTheCurrentCycle` | `lane_bound` on EP-142 with no cycle, during cycle 1 → `cycleId` cycle 1, the event's actor Linus; during a cooldown → no cycle | a started issue left out; the move written as `null` |
| `TestAnOverlapIsRefused` | cycle 2's `startsOn` set to cycle 1's `endsOn` → 422 `cycle_overlap` naming cycle 1; a later `startsOn` leaving a gap → 200 | overlaps accepted |
| `TestACadenceChangeKeepsReferencedCycles` | cycles 1-3, cycle 3 unreferenced and cycle 2 referenced; `PUT` 7/0/3 → cycle 3 dropped and re-made with 7-day dates, cycle 2 unchanged, numbers 1, 2, 3 | referenced cycles re-dated; a gap left by dropping the middle |
| `TestStaleIssuesAreOneProposal` | six `backlog` issues of Linus untouched for 31 days, `staleAfterDays: 30` → one proposal of six, one `closure.proposed` visible to Linus only; a second sweep → no new proposal | six proposals; a proposal on the shared feed |
| `TestAnAnswerClosesOnlyTheNamed` | Linus answers with three ids → exactly those three `cancelled`, each `issue.state` with actor Linus; the other three kept and not re-proposed on the next sweep; Grace answering → 404 | closes all six; `actor: null`; an admin closing on another's behalf |
| `TestAutoCloseNamesThePolicyAuthor` | `staleMode: auto` → the six `cancelled`, cause `stale_policy`, `actor: null`, `policyAuthorId` Grace; a `started` stale issue untouched | a silent close; live work closed as stale |
| `TestArchiveKeepsItSearchable` | a `done` issue past `archiveAfterDays: 90` → `archivedAt` set, absent from the default list, present with `includeArchived=true` and in search; a reopen clears it | archive as delete; search filtered |
| `TestTheFeedHoldsNoIssueAndRevokes` | the `.ics` body has one `VEVENT` per cycle and none of EP-142's title, no user name; `Origin` set → 403; after `DELETE` the token → 404; a new mint revokes the old | titles in the feed; a token not revocable; a browser allowed |
| `TestTheJanitorWritesOnlyItsColumns` | as `hub_app` with `hub.work_actor_kind = 'janitor'`: `UPDATE issue_heads SET estimate` → `42501`; `state` to `done` → `42501`; `cycle_id`, `archived_at` → ok | the janitor writing a person's field or a fact |
| `TestTheSyncActorSetsNoCycle` | `hub.work_actor_kind = 'integration'`, `UPDATE issue_heads SET cycle_id` → `42501` | the database half of the sync rule |

#### Steps

- [ ] **Step 1: Check the base.** Task 74's tables and head guard as merged; Hub task 24's
  `Sweep` and its per-org transaction; task 80's draft table (if task 80 has not merged, pass 8 is
  written but its test waits). Ask the Hub owner about `/cal/` outside `/v1` (decision (i)); build
  both paths either way.
- [ ] **Step 2: The tests.** The unit test, the fifteen integration tests and the `"77"` rows with
  the fixture's `CycleID` and `ProposalID`. Run them: they fail.
- [ ] **Step 3: The migration append**, then `hubapi` (the cause, its three `MoveAllowed` rows,
  `IssuePlan.ArchivedAt`, the eight kinds, `IssueStateData.PolicyAuthorID`), then the store, then
  the routes, then the janitor passes, then task 10's two insertions and task 08's list flag.
- [ ] **Step 4: Green.** This task's suites, task 10's (`TestMoveAllowedIsTheTable` with its three
  new rows), Hub task 24's janitor tests (passes 1-4 unchanged), task 13's registry tests and the
  matrix.
- [ ] **Step 5: Commit by pathspec**, the fourteen files of *Files*, naming task 10's, task 08's
  and Hub task 24's files as touched.

#### Acceptance

```bash
go test ./control-plane/internal/hub/store/ -race -count=1 -run '^TestNextCycleDatesKeepThePhase$' -v 2>&1 | grep -c '^--- PASS'   # 1
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(ACadenceMakesGaplessCycles|ASecondSweepIsANoOp|RolloverSkipsClosedIssues|TheScopeLogIsWhoAndWhen|StartedJoinsTheCurrentCycle|AnOverlapIsRefused|ACadenceChangeKeepsReferencedCycles|StaleIssuesAreOneProposal|AnAnswerClosesOnlyTheNamed|AutoCloseNamesThePolicyAuthor|ArchiveKeepsItSearchable|TheFeedHoldsNoIssueAndRevokes|TheJanitorWritesOnlyItsColumns|TheSyncActorSetsNoCycle)$' \
  2>&1 | grep -c '^--- PASS'                                                    # 14
go test ./control-plane/internal/hub/... -tags integration -race -count=1       # every package ok
grep -rn "time.Sleep" control-plane/internal/hub/server/janitor_work_integration_test.go | wc -l   # 0
```

Expected: `1`, `14`, every package `ok`, `0` (the janitor tests fix the clock and never sleep).
None has been run; the code does not exist yet (OD-27).

#### What this task does not claim

- **Not convergence per criterion** on a cycle: the desktop derives it from phase 4's words (task
  87). The Hub stores only the scope log.
- **Not *propose the next cycle*** (W4, L-66), and **not a capacity figure** (L-62 is a SKIP: a
  capacity number is a prediction).
- **Not per-team time zones.** Cycles turn at 00:00 UTC (decision (a)).
- **Not that calendar clients accept the feed.** RFC 5545 is followed as written; Outlook's and
  Google's import has not been tried (the corporate network may also block the fetch).
- **Not that the Hub owner accepts `/cal/`.** It is **UNVERIFIED pending review**; the bearer
  download is the fallback.
- **No measured cost.** The stale read is one query per team per sweep; nobody has timed it at
  9,000 issues.

#### Corrections to the scope

1. **The janitor writes more than `cycle_id`.** Task 74's scope limits it to `cycle_id`
   (`tasks-61-80.md:484-485`), but this task's own scope has it archive (L-12) and, under
   `staleMode: auto`, close (L-11). The guard is widened here, in task 74's file, to exactly
   `cycle_id`, `archived_at` and a `stale_policy` cancel of an open issue.
2. **Archive needs a column and a wire field** neither task 72 nor task 74 lists:
   `issue_heads.archived_at` and `IssuePlan.ArchivedAt`, both added here, additively.
3. **The calendar token needs a table** (`calendar_tokens`) task 74 does not list; appended here.
4. **`GET /v1/teams/{teamId}/cadence`** is added (a settings screen must read what it writes).
5. **Answering a proposal is 404 to everyone but the assignee** (not 403), the no-leak rule the
   scope applies to drafts (L-33), applied to a private Desk item.

**Parity rows served:** the Hub halves of **L-11** (stale closure proposed as one batch, or auto
with the author named), **L-12** (archive, still searchable), **L-57** to **L-61** (automatic
cycles, cooldown, up to 15 ahead, rollover with one event per issue, started issues join the
current cycle), **L-63**'s scope log, **L-64** (the calendar feed) and **L-65** (edit names and
dates; a pause is a gap). Task 87 closes them with the desktop.
