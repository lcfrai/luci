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

#### `server/work_initiatives.go`, complete: the read rule and the view

```go
func (s *Server) registerWorkInitiatives() {
	s.route("POST /v1/teams/{teamId}/initiatives", s.createInitiative)
	s.route("GET /v1/teams/{teamId}/initiatives", s.listInitiatives)
	s.route("GET /v1/initiatives/{initiativeId}", s.getInitiative)
	s.route("PATCH /v1/initiatives/{initiativeId}", s.patchInitiative)
	s.route("PUT /v1/features/{featureId}/initiative", s.setFeatureInitiative)
}

func notFoundInitiative(id string) error {
	return workProblem(hubapi.CodeInitiativeNotFound, "There is no such initiative that you can see.",
		"initiativeId", id)
}

// readableInitiative is decision 4: the owning team's role, or "granted" for a
// member of a spanned team (who reads and never writes); else 404.
func readableInitiative(ctx context.Context, tx *Tx, id string, want hubapi.Role) (store.InitiativeRow, []string, error) {
	in, found, err := store.InitiativeByID(ctx, tx, id)
	if err != nil || !found {
		return store.InitiativeRow{}, nil, orErr(err, notFoundInitiative(id))
	}
	span, err := store.SpanOf(ctx, tx, id)
	if err != nil {
		return store.InitiativeRow{}, nil, err
	}
	role, ok, err := TeamRole(ctx, tx, in.TeamID) // H's helper (UNVERIFIED name): the caller's role in a team
	if err != nil {
		return store.InitiativeRow{}, nil, err
	}
	if ok && role != "" {
		return in, span, Require(role, true, want, notFoundInitiative(id))
	}
	for _, t := range span[1:] {
		if r, ok, err := TeamRole(ctx, tx, t); err != nil {
			return store.InitiativeRow{}, nil, err
		} else if ok && r != "" {
			if want != hubapi.RoleViewer {
				return in, span, Require("", true, want, notFoundInitiative(id)) // visible: 403
			}
			return in, span, nil
		}
	}
	return store.InitiativeRow{}, nil, notFoundInitiative(id)
}

// initiativeView: each readable feature's own latest health word and its date,
// and its criteria with task 120's words. No aggregate, no count (decision 1).
func (s *Server) initiativeView(ctx context.Context, tx *Tx, in store.InitiativeRow, span []string) (hubapi.InitiativeView, error) {
	v := hubapi.InitiativeView{Initiative: s.initiativeWire(ctx, tx, in), Teams: span,
		Features: []hubapi.InitiativeFeature{}}
	rows, err := store.FeatureRowsOf(ctx, tx, in.ID)
	if err != nil {
		return v, err
	}
	for _, f := range rows {
		a, err := workAccess(ctx, tx, f.ProjectID) // task 117: team or grant
		if err != nil {
			return v, err
		}
		if a.Read(errSkip) != nil {
			continue // not readable: neither its id nor its name leaves this function
		}
		row := hubapi.InitiativeFeature{FeatureID: f.ID, Name: f.Name, Lead: s.userWire(ctx, tx, f.LeadID),
			Criteria: []hubapi.CriterionWord{}}
		if u, ok, err := store.LatestUpdate(ctx, tx, hubapi.UpdateSubjectFeature, f.ID); err != nil { // task 119
			return v, err
		} else if ok {
			row.LatestUpdate = &hubapi.UpdateStamp{Health: hubapi.Health(u.Health), At: hubapi.FormatTime(u.At)}
		}
		words, err := store.CriterionWords(ctx, tx, f.BriefID) // task 120: settled·contested·open·uncovered
		if err != nil {
			return v, err
		}
		row.Criteria = append(row.Criteria, words...)
		v.Features = append(v.Features, row)
	}
	return v, nil
}
```

`errSkip` is a package-level sentinel (`errors.New("skip")`), never returned to a client: task
117's `Read` returns the error it is given when the caller may not read, so a `nil` there would
read every feature as readable. `store.LatestUpdate`
(task 119, **UNVERIFIED** name: its part names `UpdateMetaRow`, `task-119.md:212`) and
`store.CriterionWords` (task 120, `task-120.md:97`) are read-only; before those tasks land the
handler compiles against two stubs in this file that return `(_, false, nil)` and `nil`, deleted
by 119 and 120 respectively.

```go
func (s *Server) createInitiative(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamId")
	s.WriteReplayable(w, r, httpx.MaxBodyBytes, nil, func(tx *Tx, body []byte) (WriteResult, error) {
		ctx := r.Context()
		var req hubapi.CreateInitiativeRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		role, ok, err := TeamRole(ctx, tx, teamID)
		if err != nil {
			return WriteResult{}, err
		}
		if err := Require(role, ok && role != "", hubapi.RoleMember, notFoundTeam(teamID)); err != nil {
			return WriteResult{}, err
		}
		if err := req.Validate(); err != nil {
			return WriteResult{}, fieldError(err) // H's FieldError → 400 naming the field
		}
		if r, ok, err := TeamRoleOf(ctx, tx, teamID, req.OwnerID); err != nil || !ok || r == "" {
			return WriteResult{}, orErr(err, badField("ownerId", "The owner must be a member of the team."))
		}
		state := hubapi.InitiativePlanned
		if req.State != nil {
			state = *req.State
		}
		row := store.InitiativeRow{ID: ids.Wide(hubapi.PrefixInitiative), OrgID: tx.P.OrgID, TeamID: teamID,
			Name: strings.TrimSpace(req.Name), OwnerID: req.OwnerID, TargetDate: req.TargetDate,
			State: string(state), CreatedBy: tx.P.UserID, CreatedAt: tx.Now, UpdatedAt: tx.Now}
		if err := store.InsertInitiative(ctx, tx, row); err != nil {
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: hubapi.EventWorkInitiativeCreated, TeamID: teamID, Subject: row.ID,
			Data: map[string]any{"initiativeId": row.ID, "state": string(state)}})
		return WriteResult{Status: http.StatusCreated, Body: s.initiativeWire(ctx, tx, row)}, nil
	})
}
```

`listInitiatives` is `s.Read` with `TeamRole` and `RoleViewer` (the owning team only; a spanned
team lists its own). `getInitiative` is `s.Read`, `readableInitiative(…, RoleViewer)` and
`initiativeView`. `patchInitiative` is `s.Write`, `readableInitiative(…, RoleMember)`, the
`ownerId` membership check again when present, `store.PatchInitiative` and one
`work.initiative_updated` naming `changed` (no event when nothing changed). `setFeatureInitiative`
is `s.Write`: task 76's feature read with `workAccess(...).Write(RoleMember)` on the feature's
project; for a non-null id, `readableInitiative(…, RoleViewer)` and the span check of decision 5;
then `store.SetFeatureInitiative` and `work.feature_initiative_set`. `TeamRole`, `TeamRoleOf`,
`notFoundTeam`, `fieldError` and `userWire` are H's or task 08's helpers (**UNVERIFIED** names).

#### Tests first: `server/work_initiatives_integration_test.go` (`//go:build integration`)

Seeded with task 116's fixture: team T (Ada admin, Grace member, Vic viewer), feature F in T's
project P; team U (Uma admin, Gus member) with feature H in project Q.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheViewHasNoAggregate` | `GET /v1/initiatives/{id}` decoded into `map[string]any` has exactly the keys `initiative`, `teams`, `features`; each feature row exactly `featureId`, `name`, `lead`, `latestUpdate`, `criteria` | computed health; a progress or count field |
| `TestNoUpdateReadsNull` | F attached, no update posted: `latestUpdate` is JSON `null`; after Grace posts `at_risk` (task 119), it is `{"health":"at_risk","at":<that post's at>}` | defaults to `on_track` |
| `TestAnotherTeamsInitiativeIs404` | Gus's `GET` of T's initiative is 404 `initiative_not_found`, byte-equal to an unknown id's reply | cross-team read without a grant |
| `TestAWiderSpanReadsButDoesNotWrite` | after task 117's span `[T, U]`, Gus reads it (200) and his `PATCH` is 403 `forbidden` | a spanned team writes |
| `TestTheOwnerIsAMember` | create with `ownerId` Gus (not in T) is 400 `ownerId`; a `PATCH` to Gus likewise; Vic may not create (403) | owner outside the team; viewer writes |
| `TestAttachNeedsTheSpan` | Gus attaching H to T's initiative before the span is 422 `initiativeId`; after it, 200; detaching with `null` clears `features.initiative_id` | a feature joins any initiative |
| `TestEachWriteHasOneEvent` | create, patch (a real change), patch (no change), attach: events `1, 1, 0, 1` of their kinds, each in the write's transaction (a forced rollback leaves neither) | write without event; an event per no-op |
| `TestCriteriaWordsAreRecorded` | F's brief has criteria c1 (served by a live issue, no verdict) and c2 (served by none): the row reads `open` and `uncovered`; after a `settled` verdict on c1, `settled` | optimistic word |
| `TestAnUnreadableFeatureNeverAppears` | H attached to T's initiative through the span, then Uma revokes nothing but H's project has no grant to T: Grace's view has no row for H and the reply contains neither H's id nor its name | an unreadable feature's name leaks |

**Matrix rows, `"118"`:** `wwrite` for create and patch and the attach; `wread` for the list and
the view; each with the `X`, `O`, `N` columns 404 `initiative_not_found` or `team_not_found`, and
`"118"` appended to `workMatrixLive` in the commit that registers the routes.

#### Steps

- [ ] **Step 1: Sync and bind (15 min).** Sync-merge `feat/hub`; bind `TeamRole`, `TeamRoleOf`,
  task 74's feature columns, task 117's `workAccess`; note whether 119 and 120 have landed.
- [ ] **Step 2: Tests first (35 min).** All nine and the matrix rows; they fail (no routes).
- [ ] **Step 3: Store and routes (70 min).** Green, except `TestNoUpdateReadsNull`'s second half
  and `TestCriteriaWordsAreRecorded`, which need 119 and 120 and are skipped by name until then.
- [ ] **Step 4: After 119 and 120 (15 min).** Delete the two stubs, un-skip, run green.
- [ ] **Step 5: Commit by pathspec**, `feat(hub-w4): initiatives and the roll-up without a score`,
  ending with the attribution line.

#### Acceptance

```bash
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(TheViewHasNoAggregate|NoUpdateReadsNull|AnotherTeamsInitiativeIs404|AWiderSpanReadsButDoesNotWrite|TheOwnerIsAMember|AttachNeedsTheSpan|EachWriteHasOneEvent|CriteriaWordsAreRecorded|AnUnreadableFeatureNeverAppears)$' \
  2>&1 | grep -c '^--- PASS'
# 9   (7 plus 2 SKIP before tasks 119 and 120 land)
go test ./control-plane/internal/hub/... -tags integration -race -count=1
# ok for every package; the matrix includes the "118" rows
```

#### What this task does not claim

- **Not that a roll-up is useful to leads** (05 §2.6): the view shows words and dates; whether a
  lead can read convergence from them is the owner's judgement at task 127's review.
- **Not the graph Linear draws** (L-85): replaced by convergence across criteria and task 77's
  scope log (Q1), which task 127 draws.
- **Not initiatives across orgs, sub-initiatives or nested teams** (SKIP, Q12).
- **No measured cost.** The view reads one latest update and one criterion list per feature.

**Parity rows:** closes **L-83**, **L-84**, **L-85** and **L-88** on the Hub (task 127 the
desktop); **L-87** with task 119.

#### Gaps found

1. **The criterion words** are task 120's four; the scope's test says `open` for "no evidence",
   which is right only for a served criterion (decision 3). Task 125's gap 3 is answered here.
2. **`TeamRole` and `TeamRoleOf`** are assumed H helpers (a caller's, and a named user's, role in
   a team); if H exposes only `ProjectRole`, this task adds them beside it in a Work file.
3. **Task 120's decision 4 keeps dependencies inside one team**, while task 115's fake lets a
   granted feature be a dependency; one of the two must change (the owner of 115 and 120).
4. **Event kind names disagree:** task 114's table spells `work.milestone_created` and
   `work.dependency_added`; task 120's part spells `milestone.created` and
   `feature.dependency_added` (`task-120.md:99-100`). This task's kinds are 114's
   (`work.initiative_*`); the two parts must converge before task 124's story table.
