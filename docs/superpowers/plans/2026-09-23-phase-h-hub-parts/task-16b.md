### Task 16b: the authorisation matrix harness, and the rows for routes 3–12

**Lane A. Depends on: 16a. About 2.5 hours. Contract task H2.**

Review M4 split task 16 in two. Task 16a built the invite routes and the binding. This task builds
the one test that holds every later route task to contract §5.3: **the authorisation matrix**,
every route × every kind of caller, each cell on a fresh fixture. The `X` column runs a second
time with the Go checks switched off, to prove that row-level security alone holds (§5.3, §13).
It starts with the rows for routes 3–12: task 15's `/v1/me` and teams, and task 16a's invites.

**Files:**
- `control-plane/internal/hub/server/matrix_integration_test.go`: the harness and the rows for
  routes 3–12 (new)
- `control-plane/internal/hub/server/authz.go`: the `disableGoAuthz` variable, and its use in
  `Require` and `TeamRole` (modify; the unsplit task used it but did not list the file)
- `control-plane/internal/hub/server/export_test.go`: the `disableGoAuthz` switch (modify)

#### Contract — the matrix harness, complete

```go
// control-plane/internal/hub/server/matrix_integration_test.go
//go:build integration

package server_test

// The six callers of contract §5.3, each a different person of dev-tenant.json (task 11).
type column int

const (
	colV column = iota // Vera, viewer of T
	colM               // Linus, member of T
	colA               // Grace, admin of T (she created it)
	colO               // Ada, org admin, in no team
	colN               // Out Sider, same org, in no team
	colX               // Xavier, another enrolled org
)

// fixture is one freshly built world per cell: team T with Grace, Linus and Vera,
// and — as later tasks add them — a project P with a remote, a brief B with a
// complete revision, an uploaded blob, an approval, a reserved and a confirmed
// release, an evidence record. Each task that adds a route adds its piece here.
type fixture struct {
	TeamID, ProjectID, BriefID, BriefKey, ApprovalID, DesignKey string
	ReservedReleaseID, ConfirmedReleaseID, InviteID, RepoKey    string
	// …added by later tasks, never renamed
}

// row is one route. path uses the fixture's fields as {TeamID}, {ProjectID}, ….
// want is the expected status per column; body builds a fresh request per cell.
type row struct {
	name   string
	method string
	path   string
	body   func(f *fixture) any
	want   [6]int
	// wantCode, when set for a column, is the problem code that status must carry.
	wantCode map[column]string
}

// rows is appended to by each route task (17–27) in its own var block below,
// named rowsTask17, rowsTask18, …, and listed in allRows.
var rowsTask15and16 = []row{ /* routes 3–12 */ }
var allRows = [][]row{rowsTask15and16}
```

The rows for routes 3–12, from §5.3 (`404` means the route's own `*_not_found`):

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/me` | 200 | 200 | 200 | 200 | 200 | 200 |
| `GET /v1/teams` | 200 | 200 | 200 | 200 | 200 | 200 |
| `POST /v1/teams` | 201 | 201 | 201 | 201 | 201 | 201 |
| `GET /v1/teams/{TeamID}` | 200 | 200 | 200 | 404 | 404 | 404 |
| `PATCH /v1/teams/{TeamID}` | 403 | 403 | 200 | 404 | 404 | 404 |
| `PUT /v1/teams/{TeamID}/members/<Out Sider>` | 403 | 403 | 201 | 404 | 404 | 404 |
| `DELETE /v1/teams/{TeamID}/members/<Linus>` | 403 | 204 | 204 | 404 | 404 | 404 |
| `DELETE /v1/teams/{TeamID}/members/<Vera>` | 204 | 403 | 204 | 404 | 404 | 404 |
| `GET /v1/teams/{TeamID}/invites` | 200 | 200 | 200 | 404 | 404 | 404 |
| `POST /v1/teams/{TeamID}/invites` | 403 | 403 | 201 | 404 | 404 | 404 |
| `DELETE /v1/teams/{TeamID}/invites/{InviteID}` | 403 | 403 | 204 | 404 | 404 | 404 |

The two `DELETE …/members` rows are the "leave the team" rule of §5.3: removing **Linus** is
allowed for column `M` because the caller *is* Linus, and removing **Vera** is allowed for column
`V` because the caller *is* Vera; each is 403 for the other non-admin.

**The meta-rule.** `TestEveryRouteHasAMatrixRow` walks the server's registered patterns (the
chassis records them, task 08) and fails for any `/v1` pattern with no row in `allRows`. So a
later route task cannot add a route without adding its authorisation rows — the step that was
most likely to be skipped.

#### Tests to write first

`matrix_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAuthorisationMatrix` | every row × every column → the expected status (and code where named), each cell on a fresh fixture | any missing check in any handler (T4 — the one single-layer control, §10.7) |
| `TestTheXColumnHoldsWithoutGoChecks` | with `disableGoAuthz` on (defined in step 2: every authorisation predicate answers "allowed", and every lookup still runs under row-level security), every row's `X` cell still answers 404 (or an empty list). A 500 in an `X` cell is a failure too: it means a handler wrote a link it never looked up, and task 07a's composite key refused it with `23503`. Afterwards, as the administrator, no org-2 row holds an org-1 id | a tenancy line held by Go alone (G4, T3); a switch defined so it hides the hole (review I5: a `TeamRole` that answers "allowed" **without** reading the team, so the insert, not the lookup, meets org 1's id) |
| `TestEveryRouteHasAMatrixRow` | the meta-rule above | a later route added with no authorisation test |

#### Steps

- [ ] **Step 1:** Re-read contract §5.3 and §13 (the matrix paragraph), and task 16a's routes.
- [ ] **Step 2:** The tests first. Run them and see them fail: no harness, and no switch. Then the
  `disableGoAuthz` switch, an unexported package variable that `authz.go`'s `Require` and
  `TeamRole` consult, set only from `export_test.go`. Its meaning (review I5): **authorisation
  predicates answer "allowed"; lookups still run under row-level security.** With it on, `Require`
  returns `nil` whatever the role. `TeamRole` skips the membership query but still reads the team
  row itself: a team the org cannot see answers `ok = false` (the route's 404), and a visible one
  answers `admin, ok = true`. Every other query runs as written. Task 17's `ProjectRole` honours it
  the same way, reading the project row. So the switch removes the Go layer and only that layer:
  the `X` column then tests row-level security, and task 07a's `TestACrossOrgReferenceIsRefused`
  tests the composite keys with no Go at all.
- [ ] **Step 3:** Commit by pathspec, the three files.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestAuthorisationMatrix|TestTheXColumn|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                     # 3
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1                     # ok: 16a's nine still pass
grep -rn "disableGoAuthz" control-plane/internal/hub/server/ --include=*.go | grep -v _test.go    # only authz.go's declaration and its two uses (task 17 adds a third, in ProjectRole)
```

#### What this task does not claim

- Rows for routes 13–45: each route task adds its own.
- That a passing matrix proves a handler's business rules. It proves who may call each route, not
  what the route then does.
