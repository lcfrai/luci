### Task 117: cross-team planning: a feature shared with a second team; initiatives across teams

**Slice:** W4. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 116 (`work_project_grants`, `work_initiative_teams`, the `G` principal) and, for
the initiative half only, **118** (`work_initiatives` and `store.InitiativeByID`); the skeleton's
row names 116 alone, a gap the scope records. Order: the grant half lands first; the span route
lands in a second commit after 118. **Seams:** consumes SW.20 (task 114); no new seam.

The Hub today says *"A project's access is exactly its team's membership. There are no
per-project grants in v1"* (`hub-contract.md:333`). Q12 chose one widening: a
**project-scoped read-and-comment grant**, so a second team can follow a shared feature and talk
on it without being able to change it. Nested sub-teams, per-object sharing and sub-initiatives
stay SKIP (05 §5.4 W4). The rule must hold on every Work route, so it is one helper,
`workAccess`, that **only Work routes** call; H's `ProjectRole` and every brief, ref and design
route are untouched.

**Anchors, re-read on 2026-09-25 at `feat/hub` `1271ddb`:** `hub-contract.md:333` (no per-project
grants), `:341-359` (§5.3, the matrix), `:356` (`GET /v1/refs`: *"resolves keys in readable
projects; every other key is `missing`"*), `:357` (`GET /v1/changes`: *"events of readable projects
and own teams only"*), `:309` (`hub.org_id`).

**Read in this branch's parts, UNVERIFIED until merge:** task 08's server idiom
(`s.route`, `s.WriteReplayable`, `ProjectRole(ctx, tx, projectID)`, `Require`, `tx.Emit`, `tx.P`,
`tx.Now`, `task-08.md:316-381`) and `readableIssue` (`:397-400`); task 13's `server/changes.go`
per-caller filter (scope `tasks-01-20.md:599`; its part, when written); task 01's proposal form
(`task-01.md:53-92`); task 114's `ProjectGrant`, `GrantProjectRequest`, `InitiativeTeams`,
`SetInitiativeTeamsRequest`, `CodeGrantExists`, `CodeGrantNotFound`, and the kinds
`work.grant_added`, `work.grant_revoked`, `work.initiative_teams_set`; task 115's rows W4-06 to
W4-10 (the same rules on the fake); task 116's tables and `grantedExpect`
(`task-116.md`, *The matrix*); task 124's route rows (`task-124.md:97-101`).

#### Decisions the scope left open, made here

1. **`workAccess` returns a value, not a role string.** `WorkAccess{TeamID, Role, Granted, OK}`
   with three methods: `Read()` passes a member of the owning team or a granted member; `Comment()`
   the same; `Write(want)` is H's `Require` on `Role` and refuses a granted-only caller with 403
   `forbidden`, `required: "member"`. So no Work handler compares role strings, and a handler that
   calls `Write` where it meant `Read` fails its matrix row, not silently.
2. **Which calls switch.** Task 08's `readableIssue` (a Work-only helper) calls `workAccess`
   instead of `ProjectRole`; every Work handler that took `ProjectRole` for a Work read (issue
   lists, features, milestones, dependencies, updates) takes `workAccess`. Writes keep their `want`
   through `WorkAccess.Write`. H's `ProjectRole` has no new caller and no change.
3. **The feed.** Task 13's filter gains one `OR`: an event of a project granted to one of the
   caller's teams, **and** of a kind in `grantedFeedKinds` (issue, comment, feature, milestone,
   dependency and feature-update kinds). A brief, design, release or team event of that project is
   never sent to the granted team. The clause reads `work_project_grants` in the same query, so a
   revocation stops the feed at the next page.
4. **Who grants.** An admin of the project's owning team; the granted team must be another team of
   the same org; a granted member cannot grant onward (they hold no role in the owning team).
5. **"Teams its owner's team may name"** is task 115's decision 5: the owning team first, then
   teams linked to it by a live grant in either direction. The Hub and the fake agree by row W4-09.
6. **Revoking a grant does not rewrite history.** Comments the granted team wrote stay, attributed
   to their authors; they become unreadable to that team at once, like the rest of the project.
7. **The brief across teams is a proposal, not a rule** (the scope's gap): the feature's spec is its
   branch brief, whose routes and `/v1/refs` stay team-only (`hub-contract.md:356`). This task
   writes `w4-hub-contract-proposal.md` in task 01's form and applies nothing on `feat/hub`.

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/server/work_access.go` (+ `_test.go`) | new: `WorkAccess`, `workAccess`, `grantedFeedKinds` |
| `control-plane/internal/hub/store/work_grants.go` (+ `_integration_test.go`) | new: insert, delete, list, `GrantedTo`, `TeamsLinked` |
| `control-plane/internal/hub/server/work_grants.go` | new: the three grant routes |
| `control-plane/internal/hub/server/work_initiative_teams.go` | new, **after task 118**: `PUT /v1/initiatives/{initiativeId}/teams` |
| `control-plane/internal/hub/server/work_issues.go` ⚑ (task 08's) | `readableIssue` and the Work read handlers call `workAccess` (decision 2) |
| `control-plane/internal/hub/server/changes.go` ⚑ (task 13's) | the granted clause (decision 3) |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ | the `"117"` rows; `"117"` added to `workMatrixLive` |
| `control-plane/internal/hub/server/work_grants_integration_test.go` | new: the tests below |
| `docs/superpowers/planning/2026-09-25-linear-parity/w4-hub-contract-proposal.md` | new: the proposal (decision 7) |

#### The routes (SW.20; the paths are task 124's rows, `task-124.md:97-101`)

| Route | Body → reply | Who | Refusals |
|---|---|---|---|
| `POST /v1/projects/{projectId}/grants` | `GrantProjectRequest{teamId}` → 201 `ProjectGrant` | admin of the owning team | 400 `teamId` (own team, other org, unknown); 409 `grant_exists` |
| `GET /v1/projects/{projectId}/grants` | → 200 `[]ProjectGrant`, by `teamId` | viewer of the owning team | 404 `project_not_found` |
| `DELETE /v1/projects/{projectId}/grants/{teamId}` | → 204 | admin of the owning team | 404 `grant_not_found` |
| `PUT /v1/initiatives/{initiativeId}/teams` | `SetInitiativeTeamsRequest{teamIds}` → 200 `InitiativeTeams` | admin of the owning team | 400 `teamIds` (owner not first, over ten); 422 `bad_request` `teamIds[i]` (not linked); 404 `initiative_not_found` |

Every write appends one event in its transaction: `work.grant_added` and `work.grant_revoked`
(`{projectId, teamId}`, scope the owning team), `work.initiative_teams_set` (`{initiativeId,
teams}`, a count).

#### `server/work_access.go`, complete

```go
package server

import (
	"context"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

// WorkAccess is the caller's reach into one project for Work routes only.
// Role is the caller's role in the owning team ("" if none); Granted is true
// when a team the caller belongs to holds a read_comment grant (Q12).
type WorkAccess struct {
	ProjectID, TeamID string
	Role              hubapi.Role
	Granted           bool
	OK                bool // the project exists in the org and is reachable at all
}

// workAccess is ProjectRole widened by a grant. Only Work routes call it;
// H's brief, ref, design and release routes keep ProjectRole (decision 7).
func workAccess(ctx context.Context, tx *Tx, projectID string) (WorkAccess, error) {
	role, teamID, ok, err := ProjectRole(ctx, tx, projectID) // task 08's anchor; H's helper
	if err != nil {
		return WorkAccess{}, err
	}
	a := WorkAccess{ProjectID: projectID, TeamID: teamID, Role: role, OK: ok && role != ""}
	if a.OK {
		return a, nil
	}
	granted, owner, err := store.GrantedTo(ctx, tx, projectID, tx.P.UserID)
	if err != nil {
		return WorkAccess{}, err
	}
	if granted {
		a.TeamID, a.Granted, a.OK = owner, true, true
	}
	return a, nil
}

// Read passes a member of the owning team (viewer or better) or a granted member.
func (a WorkAccess) Read(notFound error) error {
	if !a.OK {
		return notFound // 404 before 403: an outsider learns nothing
	}
	return nil
}

// Comment is Read: a grant is read and comment, and nothing else.
func (a WorkAccess) Comment(notFound error) error { return a.Read(notFound) }

// Write is H's Require for the owning team's role; a granted-only caller is
// 403 forbidden, required "member" or higher, because the project is visible.
func (a WorkAccess) Write(want hubapi.Role, notFound error) error {
	if !a.OK {
		return notFound
	}
	// ok is true: the project is visible, so a granted-only caller (Role "")
	// gets Require's 403 forbidden, required want, and never its 404.
	return Require(a.Role, true, want, notFound)
}

// grantedFeedKinds: an event of a granted project reaches the granted team
// only if its kind is one of these prefixes (decision 3).
var grantedFeedKinds = []string{"issue.", "comment.", "feature.", "work.milestone_",
	"work.dependency_", "work.update_posted"}

func grantedKind(kind string) bool {
	for _, p := range grantedFeedKinds {
		if strings.HasPrefix(kind, p) {
			return true
		}
	}
	return false
}
```

`Require(role, ok, want, notFound)` (task 08's use, `task-08.md:337`) answers `notFound` when
`ok` is false and 403 `forbidden` when the role ranks below `want`; the empty role ranks below
every role. **UNVERIFIED** until H's `Require` merges; `TestAGrantedWriteIs403Not404` pins the
answer, so a `Require` that ranks `""` otherwise fails it at once.
