### Task 118: initiatives `in-`: owner, target, status, the health roll-up

**Slice:** W4. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 116 (`work_initiatives`, `work_initiative_teams`, `features.initiative_id`, the
guards). It reads task 119's latest update per feature and task 120's criterion words when they
land; until then those two fields read `null` and `[]` and the tests that need them wait (Step 4).
**Seams:** consumes SW.20 (task 114's `Initiative`, `InitiativeView`, `InitiativeFeature`,
`UpdateStamp`, `CriterionWord`, the requests and `work.initiative_*` kinds); no new seam.

An initiative groups features toward one outcome, per team (05 §3.2, 04 §2.5). Linear's version
rolls the features up into a health and a progress graph; we do neither, because both would be a
number nobody measured (Q1, OD-27). The roll-up here is **each feature's own latest health word
and its date**, set by a person in a status update, and **each feature's criteria with their
words**, recorded facts. A lead reads convergence across features; the Hub computes no score.

**Anchors, re-read on 2026-09-25 at `feat/hub` `1271ddb`:** `hub-contract.md:341-359` (§5.3's
rows: a team's reads are `V M A` ✓ and `O N X` 404), `:333` (a project's access is its team's).
**Read in this branch's parts, UNVERIFIED until merge:** task 08's server and store idiom
(`task-08.md:253-400`); task 114's types (`task-114.md`) and `WorkRoadmapCodeStatus`; task 116's
tables (`task-116.md`, the tables block); task 117's `workAccess` (`task-117.md`, `work_access.go`)
and its span route, which calls this task's `store.InitiativeByID`; task 119's store read of the
latest update per subject (`task-119.md:212-226`, `UpdateMetaRow`); task 120's
`store.CriterionWords` and its four words `settled`, `contested`, `open`, `uncovered`
(`task-120.md:31-39`, `:97-98`, `:185`).

#### Decisions the scope left open, made here

1. **No aggregate field, by type.** `InitiativeView` (task 114) has `Initiative`, `Teams` and
   `Features`; nothing else. A test decodes the reply into `map[string]any` and fails on any key
   outside those three, so a later "helpful" `health` or `progress` breaks the build's tests, not
   just a review.
2. **A feature with no update reads `latestUpdate: null`.** Never a default word, never the
   initiative's own state.
3. **The criterion words are task 120's.** `settled`, `contested`, `open` (a live issue serves it,
   no verdict yet) and `uncovered` (nothing serves it). The scope's test *"a criterion with no
   evidence reads `open`"* holds for a served criterion; an unserved one reads `uncovered`, the
   stricter word. This part says which vocabulary the Hub sends (task 125's gap 3): these four,
   never `drifting`, which only the desktop can compute.
4. **Who reads.** A member (viewer or better) of the owning team, or of a team in the initiative's
   span (task 117); anyone else is 404 `initiative_not_found`, as if it did not exist. Each
   feature row is included only if `workAccess` on the feature's project passes for the caller, so
   a spanned reader sees the features their own team or a grant lets them read, and no other name.
5. **Who writes.** Create and patch: a member of the owning team (`RoleMember`); the owner must be
   a member of the team, checked at create and at every change of owner. Attaching a feature: a
   member of the **feature's** team, and the feature's team must be in the initiative's span, else
   422 `bad_request`, `field: "initiativeId"`.
6. **State words are the feature's five** (task 114, decision 1). A person sets them; nothing moves
   an initiative's state by itself, and `done` needs no condition (a person's word, like a
   feature's).
7. **List order** is newest first by `created_at`, through H's `Page` (`types.go:196-205`).

#### Files

| Path (under `control-plane/internal/hub/`) | Change |
|---|---|
| `store/work_initiatives.go` (+ `_integration_test.go`) | new: the rows, the span read, the feature link |
| `server/work_initiatives.go` | new: five routes and the view |
| `server/work_initiatives_integration_test.go` | new: the tests below |
| `server/work_matrix_rows_integration_test.go` ⚑ | the `"118"` rows; `"118"` added to `workMatrixLive` |

#### The routes (SW.20; paths are task 124's rows, `task-124.md:103-107`)

| Route | Body → reply | Who | Refusals |
|---|---|---|---|
| `POST /v1/teams/{teamId}/initiatives` | `CreateInitiativeRequest` → 201 `Initiative` | member of the team | 400 by `Validate`; 400 `ownerId` (not a member); 404 `team_not_found` |
| `GET /v1/teams/{teamId}/initiatives` | → 200 `Page[Initiative]`, newest first | viewer of the team | 404 `team_not_found` |
| `GET /v1/initiatives/{initiativeId}` | → 200 `InitiativeView` | viewer of the owning team or a spanned team | 404 `initiative_not_found` |
| `PATCH /v1/initiatives/{initiativeId}` | `PatchInitiativeRequest` → 200 `Initiative` | member of the owning team | 400; 403 (a spanned team's member); 404 |
| `PUT /v1/features/{featureId}/initiative` | `SetFeatureInitiativeRequest{initiativeId \| null}` → 200 `Feature` | member of the feature's team | 404 `feature_not_found`, `initiative_not_found`; 422 `bad_request` `initiativeId` (the feature's team is not in the span) |

Events, each in the write's transaction: `work.initiative_created` (`{initiativeId, state}`),
`work.initiative_updated` (`{initiativeId, fields}`, the changed field names, never their values),
`work.feature_initiative_set` (`{featureId, initiativeId}`, `""` when detached). Scope: the owning
team (`TeamID` on the event), so the feed's team rule shows them to its members.

#### `store/work_initiatives.go`, complete (signatures, and the view's one query)

```go
package store

type InitiativeRow struct {
	ID, OrgID, TeamID, Name, OwnerID, State, CreatedBy string
	TargetDate                                          *string // YYYY-MM-DD
	CreatedAt, UpdatedAt                                time.Time
}

// InsertInitiative writes the row and its span's first entry (the owning team, ord 0).
func InsertInitiative(ctx context.Context, tx pgx.Tx, r InitiativeRow) error
// InitiativeByID is the row under RLS; found is false for absent or another org's.
// Task 117's span route and task 119's initiative updates call it.
func InitiativeByID(ctx context.Context, q Querier, id string) (r InitiativeRow, found bool, err error)
// InitiativesOf pages a team's initiatives, newest first.
func InitiativesOf(ctx context.Context, q Querier, teamID string, page hubapi.PageRequest) ([]InitiativeRow, *string, error)
// PatchInitiative applies the non-nil fields; changed names the columns written.
func PatchInitiative(ctx context.Context, tx pgx.Tx, id string, p InitiativePatch, at time.Time) (changed []string, err error)
// SpanOf is work_initiative_teams in ord order: the owning team first.
func SpanOf(ctx context.Context, q Querier, initiativeID string) ([]string, error)
// SetFeatureInitiative writes features.initiative_id (nil clears).
func SetFeatureInitiative(ctx context.Context, tx pgx.Tx, featureID string, initiativeID *string) error

type InitiativePatch struct {
	Name, OwnerID, State *string
	TargetDate           **string // nil: unchanged; &nil: cleared
}

// FeatureRowsOf lists the features pointing at the initiative, with each
// feature's project and lead, oldest first. The handler drops the rows the
// caller may not read (decision 4) before any other field is looked at.
func FeatureRowsOf(ctx context.Context, q Querier, initiativeID string) ([]InitiativeFeatureRow, error) {
	rows, err := q.Query(ctx, `
		SELECT f.id, f.project_id, f.name, f.lead_id, f.brief_id
		  FROM hub.features f
		 WHERE f.initiative_id = $1
		 ORDER BY f.created_at, f.id`, initiativeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[InitiativeFeatureRow])
}

type InitiativeFeatureRow struct {
	ID, ProjectID, Name, LeadID, BriefID string
}
```

`features.lead_id`, `brief_id`, `created_at` are task 74's column names (**UNVERIFIED** until its
part); `pgx.CollectRows` and `RowToStructByPos` are `pgx/v5`'s, as task 08's store uses pgx.
