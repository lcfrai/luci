### Task 16: invites and their binding — routes 10–12; and the authorisation matrix harness

**Lane A. Depends on: 15. About 3 hours. Contract task H2.**

Teammates join by an **email invite that binds at their first sign-in** (ADR-H1 Q4): no Microsoft
Graph call, so no Graph consent (contract §7.3). This task builds the three invite routes and the
binding inside `/v1/me`, and then the one test that holds every later route task to contract
§5.3: **the authorisation matrix**, every route × every kind of caller, with the `X` column run a
second time with the Go checks switched off to prove row-level security alone holds (§5.3,
§13).

**Files:**
- `control-plane/internal/hub/server/invites.go` + `invites_integration_test.go` (new)
- `control-plane/internal/hub/store/invites.go` (new)
- `control-plane/internal/hub/server/me.go` — binding (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` — the harness and the rows for routes 3–12 (new)
- `control-plane/internal/hub/server/export_test.go` — the `disableGoAuthz` switch (modify)

#### Contract — the routes, complete

| # | Route | Role | Success | Refusals | Events |
|---|---|---|---|---|---|
| 10 | `GET /v1/teams/{teamId}/invites?after=&limit=` | V | `200 Page[Invite]`, newest first | 404 `team_not_found` | — |
| 11 | `POST /v1/teams/{teamId}/invites` `CreateInviteRequest` | A | `201 Invite`, `state: pending` | 404; 403 `forbidden`; 400 `field: email` (lowercased, 3–254 characters, exactly one `@`) or `field: role`; 409 `conflict` (a pending invite for that email in this team); 409 `conflict` `field: email` (that email already belongs to a member of this team) | `team.invite_created {"email","role"}` |
| 12 | `DELETE /v1/teams/{teamId}/invites/{inviteId}` | A | `204`; the row stays, `state: revoked` | 404 `team_not_found`; 403; 404 `invite_not_found`; 409 `conflict` (already `bound`) | `team.invite_revoked {}` |
| 3 | `GET /v1/me` — **binding** | any | `invitesBound`: the number bound by this call | — | per bound invite: `team.invite_bound {"userId","role"}`; then, if membership changed, `team.member_set {"role"}` |

**Binding** (contract §7.2, as correction A15 amends it): every `pending` invite **in the caller's
org** whose `email` equals the caller's **`Claims.BindEmail`** (task 13) becomes `bound`, with
`bound_user_id` and `bound_at`. `BindEmail` is the `email` claim alone, lowercased, and only when
this token shows a member account (`acct == 0`) or a verified email domain owner
(`xms_edov == true`); otherwise it is `""` and **nothing binds** — `invitesBound: 0`, the invites
stay `pending`, and no event is written. It is never `preferred_username` (mutable, and not
verified by the tenant for a B2B guest; Microsoft says not to authorise by it or by `email`), and
never the `users` row's stored `email`, which stays display text (review I12). A member whose token
has no `email` claim binds nothing; a team admin adds them by `PUT …/members/{userId}` after
their first sign-in (task 15), as for any user; the caller becomes a member with the invite's role, **or
keeps a higher role already held**. `/v1/me` runs under `Write` (task 14) so binding takes the org's
audit lock; it sends no `Idempotency-Key` because it is a `GET`, and binding twice is impossible
because a bound invite is no longer pending.

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

`invites_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestInviteCreateAndList` | Grace invites `Vera@Example.test` → stored and answered as `vera@example.test`; two invites list newest first | a case-sensitive email that never binds |
| `TestInviteEmailRules` | `nobody`, `a@b@c`, `x@`, 255 characters → 400 `field: email`; role `owner` → 400 `field: role` | — |
| `TestASecondPendingInviteIs409` | the same email twice → 409 `conflict` | two pending rows racing to bind |
| `TestInvitingAMemberIs409` | inviting Linus's email while he is a member → 409 `conflict` `field: email` | — |
| `TestRevokeKeepsTheRow` | revoke → 204; the list shows `state: revoked`; revoking a bound invite → 409 `conflict`; an unknown id → 404 `invite_not_found` | history deleted with the invite |
| `TestBindingOnFirstSignIn` | Grace invites Vera as `viewer`; Vera's `GET /v1/me` → `invitesBound: 1` and T with role `viewer`; a second call → 0; events `team.invite_bound` then `team.member_set` | an invite that never binds, or binds twice |
| `TestBindingKeepsAHigherRole` | Linus is `member`; an invite for him as `viewer` binds and leaves him `member` | a demotion by invite |
| `TestBindingStaysInTheOrg` | an invite in org 1 for `xavier@other.test` does not bind when Xavier (org 2) signs in | a cross-tenant join by email (T3) |
| `TestAGuestCannotBindAMembersInvite` | the test's own issuer, from the committed config with one user flipped (task 11: `Options.Config` is a value). (1) Out Sider as `guest: true`, `emailVerified: false`; Grace invites `out@example.test` as `member`; Out Sider's `GET /v1/me` → `invitesBound: 0`, the invite still `pending`, Out Sider not in T, no `team.invite_bound` event; (2) the same world with `emailVerified: true` → `invitesBound: 1`; (3) Vera a member with `email` set to `""` (her `preferred_username` still `vera@example.test`) and an invite for `vera@example.test` → `invitesBound: 0` | a guest's unverified `email` granting a member's invite (review I12); a `preferred_username` fallback for binding; binding that reads the stored `users.email` instead of this token |

`matrix_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAuthorisationMatrix` | every row × every column → the expected status (and code where named), each cell on a fresh fixture | any missing check in any handler (T4 — the one single-layer control, §10.7) |
| `TestTheXColumnHoldsWithoutGoChecks` | with `disableGoAuthz` on, every row's `X` cell still answers 404 (or an empty list) | a tenancy line held by Go alone (G4, T3) |
| `TestEveryRouteHasAMatrixRow` | the meta-rule above | a later route added with no authorisation test |

#### Steps

- [ ] **Step 1:** Re-read contract §5.3, §7.2, §7.3, §8, §13 (the matrix paragraph).
- [ ] **Step 2:** Tests; they fail. Then `store/invites.go`, `invites.go`, the binding in `me.go`,
  the `disableGoAuthz` switch (an unexported package variable that `authz.go`'s `Require` and
  `TeamRole` consult; set only from `export_test.go`).
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestInvite|TestASecondPending|TestInvitingAMember|TestRevokeKeeps|TestBinding|TestAGuestCannotBind|TestAuthorisationMatrix|TestTheXColumn|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                     # 12
grep -rn "disableGoAuthz" control-plane/internal/hub/server/ --include=*.go | grep -v _test.go    # only authz.go's declaration and its two uses
```

#### What this task does not claim

- A people picker: v1 invites by email (ADR-H1 §5.9 item 7).
- Invitation email delivery: none is sent in v1 (contract §7.3).
- Rows for routes 13–45: each route task adds its own.
