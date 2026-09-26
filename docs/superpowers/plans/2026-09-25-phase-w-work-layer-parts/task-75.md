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

**(f) The hold answers two ways, and neither writes half a move.** The Hub's write shape rolls a
refused step back and appends nothing (`task-11.md:314-315`), so a `409` cannot also keep the
evidence. Therefore: an **evidence** move on a held issue is **accepted and recorded, not
applied**: 200, the citations stored, the proof word decided and stored, the state left `landed`
(the head reads `state: landed`, `proof: proven`, `subState: in_qa`, which task 84 words as *held
by In QA*). A **hand** move to `done` on a held issue is **refused**: 409 `held_by_substate`,
nothing written, because a person closing past a person's hold should clear it first (one click,
and the clear itself closes the issue when the evidence holds). This corrects the scope, which
put the `409` on the evidence move; see *Corrections to the scope*.

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
| a **hand** move to `done` while an `after_landed` sub-state is set | 409 `held_by_substate`, `details.subState` |
| an **evidence** move while one is set | 200, `state: landed`, citations and proof word stored (decision (f)) |

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

#### The routes (`server/work_custom_states.go`), in order

```go
func (s *Server) registerWorkCustomStates(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/teams/{teamId}/states", s.getCustomStates)
	mux.HandleFunc("PUT /v1/teams/{teamId}/states", s.putCustomStates)
	mux.HandleFunc("PUT /v1/issues/{issueId}/sub-state", s.putSubState)
}
```

`PUT` is never stored for replay (Hub task 14 claims keys for `POST` only, `task-11.md:338-339`),
so all three run under `server.Write` or the read path. The order is the rule; each refusal rolls
the transaction back and appends nothing.

**`getCustomStates`** (viewer of the team; outside the caller's teams → 404 `team_not_found`, the
Hub's code): `CustomStates`, answered as `CustomStateList`.

**`putCustomStates`** (admin of the team):

1. `server.Decode[hubapi.PutCustomStatesRequest]` (strict: an unknown key, `teamId` included, is
   400, task 72 decision (h)); the team outside the caller's teams → 404; role below admin →
   `server.Forbidden("admin")`.
2. Per entry, the table's field checks, in index order, the first failure answered.
3. `CustomStates` (the stored list), then `DiffCustomStates`.
4. `HolderCounts` over `Removed ∪ Rebanded`; the first key (sorted) with a count → 409
   `state_in_use`, `details.key`, `details.count`, message *"N issues are <name>. Move them first."*
5. Nothing added, removed or edited → 200 with the list and **no** event (a repeated `PUT` is
   silent, so a settings screen that saves on blur writes nothing twice).
6. `PutCustomStates`; `ErrStateHeld` → the same 409 with the count re-read (a race the audit lock
   makes unreachable within one org; answered, not assumed).
7. One `team.states_changed` event, `TeamID` set, `ProjectID` empty, `Subject` the team id. 200
   with the new list.

**`putSubState`** (member of the issue's team):

1. `Decode[hubapi.SetSubStateRequest]`; `readableIssue(…, hubapi.RoleMember, lock=true)` (task
   08's lookup, which takes the head's row lock, so a concurrent evidence move waits on it).
2. The key: `nil` or `""` means clear. Else `CustomState(team, key)`; not found → 422
   `substate_out_of_band`, `details.key`, `details.band: ""`, `details.state` the head's state.
   Found but `!hubapi.BandHolds(band, head.State)` → the same code with the state's band.
3. The key equals the head's sub-state → 200 with the head, no write, no event.
4. `SetSubState`; `ErrSubStateBand` → the step 2 answer (never expected with step 2 right).
5. **The release** (decision (e)). When the old sub-state's band is `after_landed`, the new one is
   none, and `head.State == landed`: read task 10's `LiveCitations(issue, landCommit)` and the
   issue's served criteria, and run task 10's `hubapi.DecideProof`. On `proven` or
   `proven_self_attested`: `SetWorkCause(tx, cause)` with `evidence_recorded` if any live citation
   was recorded by hand, else `evidence_observed`; `MoveIssueState(…, done, "", now)`;
   `PutProof` with the word and `decided_at` now. The actor of both writes is the caller.
6. Events, in this order: `issue.sub_state_changed {from, to, released}`; when step 5 moved it,
   task 10's `issue.state {from: landed, to: done, cause, proof, landCommit}`. 200 with the head.

An `after_landed` sub-state set on a `landed` issue with `proof: waiting` releases nothing when it
is cleared (step 5 reads `waiting` and stops): the next evidence move closes it, as task 10 does.

#### The two insertions in task 10's route (`server/work_states.go` ⚑)

Task 10's order (`task-10.md:407-435`) is kept; these three lines slot in:

- **In task 10's step 4, `hand` into `done`:** before the reason check, when `head.SubState` is
  set and that key's band is `after_landed` → 409 `held_by_substate`, `details.subState` the key,
  message *"In QA holds this issue. Clear it first."* Nothing is written (decision (f)).
- **In task 10's step 4, the evidence causes, after `DecideProof`:** when the word is `proven` or
  `proven_self_attested` and `head.SubState` is an `after_landed` key, the target stays `landed`
  instead of becoming `done`. The citations and the proof word are stored as task 10 writes them,
  the answer is 200 with the head, and task 10's one `issue.state` event is written with `from` and
  `to` both `landed` (its step 6 rule for an evidence move that stays `landed`). The clear in
  `putSubState` step 5 then finds the citations and closes the issue.
- **In task 10's step 5, beside `MoveIssueState`:** when the move's target is outside the band of
  `head.SubState` (`!BandHolds(band, to)`), `SetSubState(tx, issue, "", now)` runs in the same
  transaction, and a second event `issue.sub_state_changed {from: key, to: "", released: false}`
  follows the `issue.state` event. A move that stays in the band (`backlog` → `todo`) keeps it. A
  reopen from `landed` to `todo` clears an `after_landed` key this way, and task 10's reopen
  clears the citations, so a re-landing starts with no hold and no proof.

Task 23 reads an evidence move's answer by the head's `state`, not by the status, so a 200 that
stays `landed` is not mistaken for a close (task 10 decision 5 already answers 200 for `waiting`).
