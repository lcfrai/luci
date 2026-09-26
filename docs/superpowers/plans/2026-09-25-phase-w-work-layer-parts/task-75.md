### Task 75: custom states inside the human bands

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (the `custom_states` table, `issue_heads.sub_state` and its band trigger), after a
sync-merge of `feat/hub` (rule H1). **Seams:** consumes **SW.18** (task 72's `CustomState`,
`CustomStateSpec`, `StateBand`, `BandHolds`, `ValidCustomStateKey`, `CodeHeldBySubState`,
`CodeStateInUse`, `CodeSubStateOutOfBand`) and **SW.4** (task 10's state move and done rule);
adds two event kinds to **SW.5** (task 13's registry; additive, so SW.5's freeze allows it).

A team wants *Ready for design* before work starts and *In QA* after it lands. Linear lets a team
rename and reorder every state. We cannot: `started`, `in_review`, `landed` and `done` are facts
(skeleton rule 2), and a renamed fact would let a person type a fact into existence. So a team adds
**sub-states** that sit inside the two human bands only, and the one that matters most, an
`after_landed` sub-state, **holds `done`** until a person clears it (05 §3.3, `05-design.md:732-734`).
This task is the Hub half: the team's list, the per-issue sub-state, the band check, the hold on
task 10's done rule, and the release of that hold when a person clears it. Task 84 is the desktop.

**Anchors, re-read on 2026-09-26.** In this branch's parts (not yet code, so **UNVERIFIED until
merge**): task 72's `StateBand`, `BandHolds`, `ValidCustomStateKey` (`task-72.md:99-108`,
`:268-283`), `CustomState` and `CustomStateSpec` (`:488-503`), the three codes (`:358-361`);
task 10's route and rule (`task-10.md:402-435`) and store contract (`:353-400`); task 13's
registry (`task-13.md:250-270`) and `EventData` (`:297`); task 07's matrix helpers `wread`,
`wwrite`, `wadmin` and `workMatrixLive` (`task-07.md:362-391`). In the Hub worktree at `6463e568`
(read, not merged here): contract §5.2 roles (`2026-09-23-hub-contract.md:324-333`), §8's rule that
event `data` is metadata (`:1131` onward). **Task 74's part is not written at this writing**, so
its column names (`custom_states.key`, `.name`, `.band`, `.ord`; `issue_heads.sub_state`) are the
scope's (`tasks-61-80.md:474-483`), **UNVERIFIED**; a different spelling in task 74 is a
find-and-replace in this task's store file only.

#### Decisions the scope left open, made here

**(a) The list is replaced whole, and matched by key.** `PUT /v1/teams/{teamId}/states` sends every
custom state the team keeps. The server diffs by `key`: a key in both is an edit (name, band,
order); a key only in the stored list is a removal; a key only in the body is an addition. A key
never changes: renaming is a `name` change, so issues holding the key keep it.

**(b) Changing a held state's band is a removal plus an addition.** An issue in `todo` holding
`ready_for_design` would sit out of band if the state moved to `after_landed`. So a band change on
a key any head holds is `409 state_in_use`, the same answer as removing it. The admin clears the
holders first or makes a new key.

**(c) Order is re-derived, not trusted.** The body's `order` per band must be `0..n-1` without gaps
or repeats (400 naming `states[i].order`); the server stores it as sent. Gapless is checkable, so
a client bug cannot store two states at order 1.

**(d) `SetSubStateRequest.Key` is a `*string`; absent and `null` both clear.** The route has one
meaning, "the sub-state is now this", so an absent key clearing is not a surprise, and task 72's
`Change[T]` is not needed here. A key the team does not hold is `422 substate_out_of_band` with
`details.band: ""` (there is no band for an unknown key), not a 404: the issue exists.

**(e) Clearing an `after_landed` sub-state re-runs task 10's evidence decision, not the move.**
The clear calls task 10's `DecideProof` over the issue's live citations at its land commit (task
10 decision 5). `proven` or `proven_self_attested` moves the issue to `done` with cause
`evidence_recorded` if any cited record is recorded, else `evidence_observed`, written as the
**clearer** (skeleton rule 2: a fact is written as the person whose act caused it). Two events
follow, in one transaction: `issue.sub_state_changed`, then task 10's `issue.state`. The clearer
is a member; a viewer cannot release a hold.

**(f) A hand-set `done` is not held.** Task 10's `hand` cause into `done` needs a reason and reads
`not_proven`; an admin who hand-closes an issue sitting *In QA* has decided in words, so the hold
applies to the **evidence** causes only, and the hand move clears the sub-state (it leaves the
band). This keeps one rule: a sub-state never blocks a person, only the automatic close.

**(g) Removal counts are plain numbers.** `409 state_in_use` carries `details.key` and
`details.count` (task 72's code comment), and the message reads *"3 issues are In QA. Move them
first."* The reply does not list the issues: the count is enough to act on, and the desktop's
filter `subState is in_qa` (task 72's `FilterFields`) finds them in one step.

**Files:**

| Path | Change |
|---|---|
| `control-plane/internal/hub/store/work_custom_states.go` | new: the team's list, the diff, the holder counts, the sub-state write |
| `control-plane/internal/hub/server/work_custom_states.go` | new: the three routes and the clear-then-decide path |
| `control-plane/internal/hub/server/work_custom_states_integration_test.go` | new: the tests below |
| `control-plane/internal/hub/server/work_states.go` ⚑ (task 10's) | two insertions: the hold before an evidence `done`, and the band-exit clear |
| `control-plane/internal/hub/hubapi/work_events.go` ⚑ (task 13's) | two kinds and their data structs, appended to `WorkEventKinds` |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ (task 07's) | `workMatrixRows["75"]`; `"75"` in `workMatrixLive` |
| `control-plane/internal/hub/server/server.go` ⚑ | one line: `s.registerWorkCustomStates(mux)` |

The scope named `server/matrix_integration_test.go`; W1's Work rows live in task 07's
`work_matrix_rows_integration_test.go` (`task-07.md:64`), and task 100 made the same correction
(`task-100.md:102`). H16b's fixture file is untouched: this task's rows need only `TeamID`,
`ProjectID` and `IssueID`, which W1 already added. The file name `hubapi/work_events.go` is task
13's; if task 13 put the registry in `work.go`, the two kinds go there (a path, not a shape).

#### The wire (routes and bodies; the types are task 72's)

```text
GET  /v1/teams/{teamId}/states           viewer   200 {"states": [CustomState]}  ordered by band (before_started first), then order
PUT  /v1/teams/{teamId}/states           admin    200 {"states": [CustomState]}  body PutCustomStatesRequest
PUT  /v1/issues/{issueId}/sub-state      member   200 IssueHead                  body SetSubStateRequest
```

```go
// Task 72's block 6 declares these two; they are shown so this part stands alone. If task 72
// spells a field differently, task 72 wins and this task follows it.
type PutCustomStatesRequest struct {
	States []CustomStateSpec `json:"states"` // 0-24 entries; [] removes every custom state
}

type SetSubStateRequest struct {
	Key *string `json:"key"` // a key of the issue's team; null or absent clears (decision (d))
}

// The list reply. A struct, not a bare array, so a later field is additive (types.go's idiom).
type CustomStateList struct {
	States []CustomState `json:"states"`
}
```

A worked example, *In QA* added to a team that has *Ready for design*:

```json
{"states": [
  {"key": "ready_for_design", "name": "Ready for design", "band": "before_started", "order": 0},
  {"key": "in_qa",            "name": "In QA",            "band": "after_landed",   "order": 0}
]}
```

Refusals, each with task 72's code and the field it names:

| Body | Answer |
|---|---|
| a key `landed` (or any `IssueStates` name) | 422 `bad_request`, `field: "states[i].key"` (`ValidCustomStateKey` is false) |
| a band `started` | 422 `bad_request`, `field: "states[i].band"` (`ValidStateBand` is false) |
| two entries with one key | 400 `bad_request`, `field: "states[j].key"` (the second) |
| orders `0, 2` in one band | 400 `bad_request`, `field: "states[1].order"` (decision (c)) |
| a name empty, over 40 characters, or with a control character | 400 `bad_request`, `field: "states[i].name"` |
| 25 entries | 400 `bad_request`, `field: "states"` |
| removing, or re-banding, a key that heads hold | 409 `state_in_use`, `details.key`, `details.count` |
| a sub-state key on an issue whose state its band does not hold | 422 `substate_out_of_band`, `details.key`, `.band`, `.state` |
| an evidence move to `done` while an `after_landed` sub-state is set | 409 `held_by_substate`, `details.subState` |

The cap of 24 is a decision: two bands of twelve is more than any team this design names, and a
cap keeps the list one screen in task 84's settings (a number to revisit, not a measurement).

#### The two event kinds (complete, appended to task 13's registry)

`data` carries keys and counts only, every field a string, `int64` or `bool`, so task 13's
`TestEveryWorkDataFieldIsAScalar` covers them with no change. A custom state's `name` is team
metadata like a label's (task 11's precedent, `task-11.md:347-350`), but the key is enough for a
reader to look the name up, so the name stays out.

```go
const (
	EventTeamStatesChanged    = "team.states_changed"
	EventIssueSubStateChanged = "issue.sub_state_changed"
)

// TeamStatesChangedData lists the keys by what happened to them, comma-joined
// (hub-contract §8: data carries no arrays).
type TeamStatesChangedData struct {
	Added   string `json:"added"`
	Removed string `json:"removed"`
	Edited  string `json:"edited"` // name, band or order changed
}

// IssueSubStateChangedData is one issue's sub-state change; "" is none.
// Released is true when the clear let task 10's done rule close the issue.
type IssueSubStateChangedData struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Released bool   `json:"released"`
}
```

```go
// appended to WorkEventKinds, after task 72's W2 kinds if it declared any
{EventTeamStatesChanged, TeamStatesChangedData{}, ScopeTeam, VisibleToReaders},
{EventIssueSubStateChanged, IssueSubStateChangedData{}, ScopeProject, VisibleToReaders},
```

#### `store/work_custom_states.go` (signatures complete)

```go
package store

// Every function runs inside the caller's transaction, after set_config('hub.org_id') and,
// for writes, after the org's audit head lock (task 11's preamble, hub-contract §10.6).

// CustomStates reads a team's list, ordered by band (before_started first), then ord.
// An empty team answers [] (never nil).
func CustomStates(ctx context.Context, q Querier, teamID string) ([]hubapi.CustomState, error)

// CustomState reads one key of a team; ok is false when the team has no such key.
func CustomState(ctx context.Context, q Querier, teamID, key string) (s hubapi.CustomState, ok bool, err error)

// StateDiff is PutCustomStates' plan: what the body adds, removes and edits, by key.
type StateDiff struct {
	Added, Removed, Edited []string // keys, each sorted
	Rebanded               []string // Edited keys whose band changed
}

// DiffCustomStates compares the stored list with the body. Pure; no query.
func DiffCustomStates(stored []hubapi.CustomState, body []hubapi.CustomStateSpec) StateDiff

// HolderCounts answers, for each key, how many live issue heads of the team hold it as
// sub_state (erased issues included: an erased head keeps its sub_state column, 04 §5.5).
// Keys with no holder are absent from the map.
func HolderCounts(ctx context.Context, q Querier, teamID string, keys []string) (map[string]int, error)

// PutCustomStates writes the diff in one statement per kind: DELETE the removed keys,
// INSERT the added, UPDATE the edited. The caller has refused every held removal first;
// task 74's foreign key from issue_heads(team_id, sub_state) is the second guard, and a
// 23503 here is ErrStateHeld.
func PutCustomStates(ctx context.Context, tx pgx.Tx, org, teamID string, d StateDiff, body []hubapi.CustomStateSpec, by string, at time.Time) error

// SetSubState: UPDATE issue_heads SET sub_state = $2, updated_at = $3 WHERE issue_id = $1.
// key "" writes NULL. Task 74's trigger refuses a key outside the head's band with a
// 23514 whose message starts "hub: work: sub_state"; that is ErrSubStateBand.
func SetSubState(ctx context.Context, tx pgx.Tx, issueID, key string, at time.Time) error

var (
	ErrStateHeld    = errors.New("store: a custom state is held by an issue")
	ErrSubStateBand = errors.New("store: the sub-state trigger refused the band")
)
```

The foreign key in `PutCustomStates`' comment is **this task's ask of task 74**: a composite key
`issue_heads(team_id, sub_state) REFERENCES custom_states(team_id, key)`, so a removal that the Go
check missed fails in the database. If task 74 has no `issue_heads.team_id` (the head may reach
its team through `project_id`), the second guard is instead a `BEFORE DELETE` trigger on
`custom_states` that counts holders. Either is a guard; which one is task 74's call, and this
gap is reported to the orchestrator.
