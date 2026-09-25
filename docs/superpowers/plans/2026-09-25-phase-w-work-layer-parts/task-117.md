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

// roleGranted is what readableIssue returns for a granted-only caller, the
// same word as task 115's fake; it ranks below every role, so a Require on it
// is 403. It never reaches the wire.
const roleGranted hubapi.Role = "granted"

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

#### `store/work_grants.go`, complete (signatures and the two queries that carry rules)

```go
package store

// GrantRow is one work_project_grants row.
type GrantRow struct {
	OrgID, ProjectID, TeamID, GrantedBy string
	GrantedAt                           time.Time
}

// InsertGrant inserts; exists is true (and nothing written) when the pair is live.
func InsertGrant(ctx context.Context, tx pgx.Tx, g GrantRow) (exists bool, err error)
// DeleteGrant deletes; found is false when there was no such grant.
func DeleteGrant(ctx context.Context, tx pgx.Tx, projectID, teamID string) (found bool, err error)
// GrantsOf lists a project's grants by team id.
func GrantsOf(ctx context.Context, q Querier, projectID string) ([]GrantRow, error)

// GrantedTo reports whether userID belongs to a team holding a grant on
// projectID, and returns the project's owning team. One query, under RLS.
func GrantedTo(ctx context.Context, q Querier, projectID, userID string) (granted bool, ownerTeam string, err error) {
	err = q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM hub.work_project_grants g
		                 JOIN hub.team_members m ON m.team_id = g.team_id
		                WHERE g.project_id = $1 AND m.user_id = $2),
		       coalesce((SELECT team_id FROM hub.projects WHERE id = $1), '')`,
		projectID, userID).Scan(&granted, &ownerTeam)
	return granted, ownerTeam, err
}

// TeamsLinked: a live grant in either direction between two teams (decision 5).
func TeamsLinked(ctx context.Context, q Querier, a, b string) (bool, error) {
	var linked bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM hub.work_project_grants g JOIN hub.projects p ON p.id = g.project_id
		                WHERE (p.team_id = $1 AND g.team_id = $2) OR (p.team_id = $2 AND g.team_id = $1))`,
		a, b).Scan(&linked)
	return linked, err
}

// SetInitiativeTeams replaces the span: DELETE then INSERT in ord order.
func SetInitiativeTeams(ctx context.Context, tx pgx.Tx, org, initiativeID string, teamIDs []string) error
```

`hub.team_members` and `hub.projects` are H's table names (**UNVERIFIED** until H's `0001` merges);
the imports are `context`, `time` and `pgx/v5`, as task 08's store.

#### `server/work_grants.go`, complete: the create; the other two are its shape

```go
func (s *Server) registerWorkGrants() {
	s.route("POST /v1/projects/{projectId}/grants", s.grantProject)
	s.route("GET /v1/projects/{projectId}/grants", s.listGrants)
	s.route("DELETE /v1/projects/{projectId}/grants/{teamId}", s.revokeGrant)
}

func (s *Server) grantProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	s.WriteReplayable(w, r, httpx.MaxBodyBytes, nil, func(tx *Tx, body []byte) (WriteResult, error) {
		ctx := r.Context()
		var req hubapi.GrantProjectRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		role, owner, ok, err := ProjectRole(ctx, tx, projectID) // the owning team only: no grant onward
		if err != nil {
			return WriteResult{}, err
		}
		if err := Require(role, ok, hubapi.RoleAdmin, notFoundProject(projectID)); err != nil {
			return WriteResult{}, err
		}
		if !hubapi.ValidID(hubapi.PrefixTeam, req.TeamID) || req.TeamID == owner {
			return WriteResult{}, badField("teamId", "Grant a different team of this org.")
		}
		if found, err := store.TeamExists(ctx, tx, req.TeamID); err != nil || !found { // RLS: this org only
			return WriteResult{}, orErr(err, badField("teamId", "Grant a different team of this org."))
		}
		g := store.GrantRow{OrgID: tx.P.OrgID, ProjectID: projectID, TeamID: req.TeamID,
			GrantedBy: tx.P.UserID, GrantedAt: tx.Now}
		exists, err := store.InsertGrant(ctx, tx, g)
		if err != nil {
			return WriteResult{}, err
		}
		if exists {
			return WriteResult{}, workProblem(hubapi.CodeGrantExists, "That team already has this project.",
				"projectId", projectID, "teamId", req.TeamID)
		}
		tx.Emit(Event{Kind: hubapi.EventWorkGrantAdded, TeamID: owner, ProjectID: projectID, Subject: projectID,
			Data: map[string]any{"projectId": projectID, "teamId": req.TeamID}})
		return WriteResult{Status: http.StatusCreated, Body: s.grantWire(ctx, tx, g)}, nil
	})
}
```

`listGrants` is `s.Read` with `ProjectRole` and `RoleViewer` (the owning team only: a granted team
does not list who else is granted). `revokeGrant` is `s.Write` with `RoleAdmin`, `DeleteGrant`,
404 `grant_not_found` when absent, `work.grant_revoked`, 204. `badField`, `orErr`, `workProblem`
(status from `hubapi.WorkRoadmapCodeStatus`) and `store.TeamExists` are small helpers in this file
or task 08's (**UNVERIFIED** names; Step 1 takes the merged ones). A replay (`rebuild` `nil`)
answers the stored reply, as H's idempotency does for writes with no content to re-read.

#### The two ⚑ edits, complete

Task 08's `readableIssue` (`task-08.md:397-400`), the one line that changes:

```go
	// was: role, _, ok, err := ProjectRole(ctx, tx, row.ProjectID); Require(role, ok, want, nf)
	a, err := workAccess(ctx, tx, row.ProjectID)
	if err != nil {
		return store.IssueRow{}, "", err
	}
	if want == hubapi.RoleViewer {
		err = a.Read(notFoundIssue(id))
	} else {
		err = a.Write(want, notFoundIssue(id))
	}
	role := a.Role
	if a.Granted && role == "" {
		role = roleGranted // "granted"; task 12's comment create accepts it (below)
	}
```

Task 12's comment create calls `readableIssue(ctx, tx, id, hubapi.RoleMember, true)`; it changes to
`hubapi.RoleViewer` and then refuses a **granted** caller any comment kind but `note` (task 99's
kinds: an evidence proposal or a negotiation is a member's act), 403 `forbidden`,
`required: "member"`. Every other Work write keeps its `want`, so it is refused as above.

Task 13's feed query in `server/changes.go` gains one disjunct in its per-caller `WHERE`
(**UNVERIFIED** shape until task 13's part; the clause is the rule):

```sql
   OR (e.project_id IN (SELECT g.project_id FROM hub.work_project_grants g
                          JOIN hub.team_members m ON m.team_id = g.team_id
                         WHERE m.user_id = $caller)
       AND e.kind LIKE ANY ($granted_kinds))   -- grantedFeedKinds, each with '%' appended
```

#### `server/work_initiative_teams.go`, after task 118

`PUT /v1/initiatives/{initiativeId}/teams`: `s.Write`; `store.InitiativeByID` (task 118) or 404
`initiative_not_found` (a spanned team's member reads the initiative, task 118's rule, but only an
admin of the owning team writes this); `Require(role, ok, RoleAdmin)` on the **owning** team;
`teamIds[0]` must be the owning team, at most ten, none twice (400 `teamIds`); each other id must
be `store.TeamsLinked` to the owner, else 422 `bad_request`, `field: "teamIds[i]"`; then
`store.SetInitiativeTeams` and `work.initiative_teams_set`. Features of a spanned team appear in the
initiative's view only for readers who may read them (task 118's view calls `workAccess` per
feature), so a span never shows a name its reader could not open.

#### `w4-hub-contract-proposal.md`, complete

`````markdown
# W4: one proposed edit to the Hub contract, from Phase W

**To:** the owner of `feat/hub` (`docs/superpowers/contracts/2026-09-23-hub-contract.md`).
**From:** Phase W, task 117 (cross-team planning, Q12).
**Status:** proposed. Nothing on `feat/hub` has been changed. Read at `feat/hub` `1271ddb`.

## Item 1: a granted team may read the branch brief of a shared feature

**Where:** §5.2 (`hub-contract:333`, *"There are no per-project grants in v1"*) and §5.3's
`GET /v1/refs` row (`:356`).

**Today** Phase W adds a project-scoped read-and-comment grant for **Work objects only** (issues,
features, milestones, updates, comments). A granted team sees a feature but not its spec, because
the spec is the branch brief, and briefs and `/v1/refs` stay team-only.

**Propose** one sentence under §5.2: *"A team holding a Phase W `read_comment` grant on a project
may read that project's branch briefs of features it can see, and `/v1/refs` resolves their keys
for it; it writes none of them."* And the `/v1/refs` row gains *"and briefs readable through a
Phase W grant"*.

**If declined**, nothing changes: the feature header (task 126) shows the brief's key and says it
is readable by the owning team only. Nothing in Phase W depends on the answer.
`````

#### Tests first: `server/work_grants_integration_test.go` (`//go:build integration`)

Seeded with task 116's fixture: team T (Ada admin, Grace member) owns project P with feature F and
issue EP-1; team U (Uma admin, Gus member) owns project Q with feature H.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheGrantedTeamReadsAndComments` | Ada grants P to U; Gus reads EP-1, F, F's milestones and updates (200) and posts a `note` comment (201) | grant not consulted |
| `TestAGrantedWriteIs403Not404` | Gus's issue create, `PATCH`, state move, relation, label, sub-state and an `evidence_proposal` comment are each 403 `forbidden`, `required: member`; none is 404 | grant is write; a leak by 404/403 difference inverted |
| `TestRevocationIsImmediate` | Ada revokes; Gus's next `GET /v1/issues/EP-1` is 404 and his next `/v1/changes` page carries no P event, with no cache flush between | revocation ignored |
| `TestTheFeedNeverOverShares` | after the grant, a brief revision on P and a label change on T reach Gus's feed **zero** times; an issue event on P reaches it once; nothing of Q's sibling project R reaches Grace | feed over-shares |
| `TestTheBriefStaysTeamOnly` | Gus's `GET /v1/briefs/{F's brief}` is 404 and `/v1/refs` of its key is `missing` | a grant widening H's reads (decision 7) |
| `TestOnlyAnAdminGrants` | Grace's grant is 403; Gus cannot grant P onward (404, he has no role in T); a grant to T itself is 400; a second grant is 409 `grant_exists` | member grants; grant onward |
| `TestASpanShowsOnlyWhatEachTeamReads` | (after 118) T's initiative spans U; F and H are in it; before Uma grants Q to T, Grace's view lists F only, and no byte of the response holds H's id or name | an unreadable feature's name leaks |
| `TestASpanNeedsALink` | `PUT …/teams [T, U]` with no grant either way is 422 `teamIds[1]`; `[U, T]` is 400 | spans over any team |
| `TestEachWriteHasItsEvent` | grant, revoke and span each append exactly one event of its kind, in the same transaction (a forced rollback leaves neither row nor event) | write without event |

`server/work_access_test.go` (unit), four tests: `TestWorkAccessOutsiderIs404`
(`WorkAccess{OK: false}.Read` and `.Write` are the 404 passed in); `TestWorkAccessGrantedWriteIs403`
(`{OK: true, Granted: true}.Write(member)` is 403); `TestWorkAccessRoleBelowWant`
(`{OK: true, Role: member}.Write(admin)` is 403, `.Write(member)` nil);
`TestWorkAccessGrantedKinds` (`grantedKind("brief.revised")` false, `grantedKind("issue.state")`
true; kills a prefix list that lets briefs through).

**Matrix rows, `"117"`:** `wadmin` for the grant create and delete and the span `PUT`; `wread` for
the list, with the owning team's columns (a granted `G` is 404 on the list, by `grantedRowFor`'s
override); and `"117"` appended to `workMatrixLive`, which switches on task 116's `G` rows.

#### Steps

- [ ] **Step 1: Sync and bind (15 min).** Sync-merge `feat/hub`; bind `Require`, `ProjectRole`,
  `team_members`, task 13's filter, task 12's comment path. Confirm 116 is in.
- [ ] **Step 2: Tests first (40 min).** The unit test, the integration tests but the span pair,
  and the matrix rows; all fail.
- [ ] **Step 3: The grant half (60 min).** `work_access.go`, the store, the three routes, the
  two ⚑ edits and task 12's comment rule. Green; then H16b's whole matrix and every W1-W3 row.
- [ ] **Step 4: Commit the grant half**, then, **after task 118 lands**, the span route and its two
  tests (30 min), and a second commit.
- [ ] **Step 5: The proposal file (10 min)**, handed to the orchestrator for the `feat/hub` owner.

#### Acceptance

```bash
go test ./control-plane/internal/hub/server/ -race -count=1 -run '^TestWorkAccess' -v 2>&1 | grep -c '^--- PASS'
# 4
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(TheGrantedTeamReadsAndComments|AGrantedWriteIs403Not404|RevocationIsImmediate|TheFeedNeverOverShares|TheBriefStaysTeamOnly|OnlyAnAdminGrants|ASpanShowsOnlyWhatEachTeamReads|ASpanNeedsALink|EachWriteHasItsEvent|WorkGrantedPrincipal)$' \
  2>&1 | grep -c '^--- PASS'
# 10   (8 before task 118: the two span tests are added with the second commit)
grep -rn "workAccess(" control-plane/internal/hub/server --include=*.go | grep -v _test.go | grep -v -E "work_[a-z_]+\.go" | wc -l
# 0   (no H route calls it)
```

#### What this task does not claim

- **Not that a granted team reads the brief.** The proposal decides; until then the feature header
  names the brief and says who may read it (task 126).
- **Not per-object sharing, nested teams or sub-initiatives** (SKIP, Q12).
- **Not that the feed filter is fast.** One sub-select per page; not measured.

**Parity rows:** closes **L-67**'s W4 half (with task 76's feature and task 126's header) and the
span in **L-83** (with task 118's initiatives and task 127's surface).

#### Gaps found

1. **The skeleton's row lacks 118** in `dependsOn`; the span half lands after 118 (Step 4).
2. **No caller-scoped list of projects granted to my teams** (task 124's gap 7): a granted member
   has no route to discover what was shared with them except the feed. Add `GET
   /v1/me/granted-projects` here or in task 126, with a `WorkRoadmap` method (task 114's gap 6).
3. **Task 12's comment kinds** are narrowed for a granted caller (`note` only); the scope says
   "write comments" without kinds.
