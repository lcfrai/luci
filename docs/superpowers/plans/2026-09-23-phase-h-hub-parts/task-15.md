### Task 15: `/v1/me`, teams and members — routes 3–9

**Lane A. Depends on: 03, 14. About 3 hours. Contract task H2.**

The first routes behind the auth pipeline, and the membership that decides every later
authorisation: **a project's access is exactly its team's membership** (contract §5.2). Invites
are task 16a; `/v1/me` answers `invitesBound: 0` until then.

**Files:**
- `control-plane/internal/hub/server/me.go`, `teams.go` + `teams_integration_test.go` (new)
- `control-plane/internal/hub/server/authz.go` — the role checks every route task reuses (new)
- `control-plane/internal/hub/store/teams.go`, `users.go` (new)
- `control-plane/internal/hub/server/server.go` — `s.registerTeams()` (modify: one line)

#### Contract — the routes, complete

Bodies are `hubapi` types (task 03). **No request body carries an actor**: `hubapi`'s request
types have no such field, and `DecodeJSON` refuses one.

| # | Route | Role (§5.3) | Success | Refusals | Events (§8) |
|---|---|---|---|---|---|
| 3 | `GET /v1/me` | any | `200 hubapi.Me`: the principal as `User`, its `Org`, `orgAdmin` **from this token's `roles`** (never stored), its teams by name, `invitesBound` | 403 `tenant_not_enrolled` (task 14) | `auth.user_provisioned` on first sight (task 14) |
| 4 | `GET /v1/teams` | any | `200 {"items":[Team],"next":null}` — the caller's teams, by `lower(name)` | — | — |
| 5 | `POST /v1/teams` `CreateTeamRequest` | any | `201 Team` with `role: "admin"` | 400 `bad_request` `field: name` (1–120 characters after trimming); 409 `conflict` `field: name` (unique per org, case-insensitive) | `team.created {"name"}`; `team.member_set {"role":"admin"}` subject = the caller |
| 6 | `GET /v1/teams/{teamId}` | V | `200 TeamDetail`, members by `lower(display_name)` | 404 `team_not_found` | — |
| 7 | `PATCH /v1/teams/{teamId}` `RenameTeamRequest` | A | `200 Team` | 404; 403 `forbidden` `required: admin`; 400/409 as route 5 | `team.updated {"name"}` |
| 8 | `PUT /v1/teams/{teamId}/members/{userId}` `SetMemberRequest` | A | `201 Member` added, `200 Member` changed (`addedBy` = the caller on add) | 404 `team_not_found`; 403 `forbidden`; 404 `member_not_found` (no such user **in the caller's org**); 400 `field: role`; 409 `last_admin` (demoting the last admin) | `team.member_set {"role"}` subject = `userId` |
| 9 | `DELETE /v1/teams/{teamId}/members/{userId}` | A, or self | `204` | 404 `team_not_found`; 403 `forbidden` (a non-admin removing someone else); 404 `member_not_found`; 409 `last_admin` | `team.member_removed {}` subject = `userId` |

`teamId` and `userId` path values that are not `tm-`/`u-` + 16 hex are 404 of their kind, not 400
— a malformed id and an unknown one must be indistinguishable (§4.2's reasoning).

```go
// control-plane/internal/hub/server/authz.go — the checks every route task calls.
package server

// TeamRole returns the caller's role in teamID, or ok=false when the caller is
// not a member — which every caller turns into the route's 404, never a 403.
func TeamRole(ctx context.Context, tx *Tx, teamID string) (role hubapi.Role, ok bool, err error)

// ProjectRole is TeamRole for the team that owns projectID (task 17 fills it).
func ProjectRole(ctx context.Context, tx *Tx, projectID string) (role hubapi.Role, teamID string, ok bool, err error)

// AtLeast reports whether have ≥ want in viewer < member < admin.
func AtLeast(have, want hubapi.Role) bool

// Require is the one-line check: not a member → notFound; below want → Forbidden(want).
func Require(role hubapi.Role, isMember bool, want hubapi.Role, notFound *APIError) error
```

**Last admin under concurrency.** Every write holds the org's audit lock (task 14), so counting a
team's admins inside the transaction and refusing when the change would leave none is race-free:
two admins demoting each other at once serialise, and the second gets 409 `last_admin`.

#### Tests to write first — `teams_integration_test.go` (`//go:build integration`)

Principals are minted with `devissuer.MintAccessToken` (task 11) against an in-process issuer and a
Hub `httptest.Server` over `hubtest.Migrated`; a helper `as(name)` returns an HTTP client carrying
that user's token.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMeProvisionsAndDescribesTheCaller` | Grace's first `GET /v1/me` → her `User` (a `u-` id, her name, her email), the org, `orgAdmin: false`, `teams: []`, `invitesBound: 0` | — |
| `TestOrgAdminIsReadFromEachToken` | Ada's token with `roles: ["Hub.OrgAdmin"]` → `orgAdmin: true`; a token for Ada without it → `false` | a role cached in the database, surviving its removal in Entra (§5.1) |
| `TestCreatingATeamMakesTheCallerAdmin` | `201`, `role: admin`; `GET /v1/teams/{id}` lists Ada as admin; two events, `team.created` then `team.member_set` | a team with no admin |
| `TestTeamNameRules` | `""`, `"   "`, 121 characters → 400 `field: name`; `"Portal"` then `"portal"` → 409 `conflict` `field: name` | two teams a person cannot tell apart |
| `TestTeamsListsOnlyTheCallersTeams` | Ada has two, Grace one; each sees exactly theirs, by name | a list query without the membership join |
| `TestANonMemberSees404Everywhere` | Linus on routes 6–9 of Ada's team → 404 `team_not_found`, and the same for a malformed id | a 403 that confirms the team exists (§4.2) |
| `TestAMemberCannotAdminister` | Grace (member) on routes 7, 8, and 9-for-someone-else → 403 `forbidden`, `required: admin`; each writes one `auth.forbidden` event (feed = false) | a refused attempt nobody can audit |
| `TestSetMemberAddsThenChanges` | PUT Grace `member` → 201; PUT `viewer` → 200; `addedBy` is Ada | an add reported as a change |
| `TestSetMemberNeedsAUserOfTheOrg` | an unknown `u-…` → 404 `member_not_found`; Xavier (another enrolled org) → 404 `member_not_found` | a cross-org member added by id (T3) |
| `TestLastAdminIsProtected` | Ada demoting or removing herself as sole admin → 409 `last_admin`; with Grace promoted to admin, both succeed | a team nobody can administer |
| `TestAnyoneMayLeave` | Grace deletes herself → 204; she then gets 404 on the team | a member trapped in a team |
| `TestNoBodyCarriesAnActor` | `PUT …/members/{grace}` with `{"role":"member","addedBy":"u-…"}` → 400 `bad_request` | a caller-asserted actor (contract §5.3) |

#### Steps

- [ ] **Step 1:** Re-read contract §5.2, §5.3 (the rows for teams), §7.2, §7.3, §8.
- [ ] **Step 2:** Tests; they fail. Then `store/users.go`, `store/teams.go`, `authz.go`, `me.go`,
  `teams.go`, and the registration line.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestMeProvisions|TestOrgAdminIsRead|TestCreatingATeam|TestTeamNameRules|TestTeamsListsOnly|TestANonMemberSees|TestAMemberCannot|TestSetMemberAdds|TestSetMemberNeeds|TestLastAdminIs|TestAnyoneMay|TestNoBodyCarries' \
  2>&1 | grep -c "^--- PASS"                                                                                   # 12
go test -tags integration ./control-plane/internal/hub/... -race -count=1                                     # ok
go test ./sandboxd/... ./control-plane/... -race -count=1                                                                                          # ok
```

#### What this task does not claim

- Invites and their binding: task 16a.
- The full authorisation matrix: task 16b builds its harness, with rows for routes 3–12, and every later task adds rows.
