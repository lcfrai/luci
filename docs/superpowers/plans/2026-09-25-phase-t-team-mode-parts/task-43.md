### Task 43: Hub delta: role-to-verb, *auditor* and *compliance*, team policy, Z5 rows

**Slice TM1. Depends on: 07, H16b (and reads 01, 03, 06, 46; phase 6 task 01). About 3 hours.
Seams: ST.2 (consumes).**

This task gives the Hub its team authorisation as **one table and one test**. `05-design.md` §3.4
(`:345-358`) says who may do what across five principals: `viewer`, `member`, `admin`, and two
**org-level** capabilities that belong to no team, *auditor* and *compliance*. Task 01 routed the
team records and wrote their rows of the contract's §5.3 (`task-01.md:545-597`); tasks 06 and 07
built the routes with per-rule tests; task 46 added notes and erasure. None of them built the Z5
matrix, which is the one test that checks **every route × every principal on a fresh fixture**
(`04` Z5; Hub task 16b's harness, `hub …/task-16b.md:18-64`). This task builds it for every team
route, and adds the three routes that are its own: 74, 75 and 79, the project's **team policy**
(task 01 §7.24, `task-01.md:466-488`).

It is a delta on the Hub branch, built after the sync-merge of `feat/hub`. The files it extends are
Hub task 16b's `server/matrix_integration_test.go` and Hub task 13's `server/auth.go`, both
**planned** on 2026-09-25 (only `hubapi` and `hubclient` exist on `.claude/worktrees/hub` at
`9f2fa31`, checked), so every path below is **UNVERIFIED until merge**.

**The one thing this task must not get wrong is rule 5** (Global constraint 3): nobody writes
another person's task, lease, message, directive or escalation, whatever their role. An admin
administers the *team*; the org *auditor* reads across owners; *compliance* moves a route. **None
of the three is a write on another person's record**, and the matrix makes that a failing test,
not a convention: in every write row, the `M′`, `A`, `Au` and `Fx` cells are refusals.

**The second thing is that the route only ever tightens without compliance.** Any `member` may
raise a project to AU; only a token carrying `Hub.Compliance` may lower it to Global, and never
below the data class's floor (an `au-only` class stays AU whoever asks). The rule is phase 6's
`dataclass.CanChange` (`phase-6 …/2026-09-25-phase-6-gateway.md:216`, planned), reused, not
rewritten: one ratchet for solo and team projects.

**Decisions this part makes.**

- **(a) The data class and the route live on the project record, not in the policy row.** Phase 6's
  ask H-1 puts `dataClass` and `inferenceRoute` on the project (`phase-6-gateway.md:56`, `:216`),
  set at creation (OD-24). Team policy's `dataClass` and `route` are **read through** from there,
  so a solo project that becomes a team project keeps its class, and there is one source for the
  gateway. Route 79 writes the project's route field. **UNVERIFIED until merge** (the Hub's project
  record does not carry them on 2026-09-25: `hubapi/types.go` has no `dataClass`, checked).
- **(b) A project with no policy row has the defaults.** Route 74 answers the product defaults
  with `revision: 0` until the first `PUT`; the first `PUT` names `revision: 0` and creates the row.
  So Share (task 21) creates nothing, and a policy never has a zero quota (OD-24: budgets on).
- **(c) A floor is a row name, not a setting.** `escalationFloors` lists which of the four §6.5
  rows whose product default is *act* now ask. A floor cannot name a row whose default is already
  *ask*, so no `PUT` can express "ask less". An admin may remove a floor they added: that returns
  the row to the product default, never below it.

**Files:**
- `control-plane/internal/hub/server/teamcaps.go` — the §3.4 table as data; `Can` (new)
- `control-plane/internal/hub/server/teampolicy.go` — routes 74, 75, 79 (new)
- `control-plane/internal/hub/store/teampolicy.go` — the policy row and the route write (new)
- `control-plane/internal/hub/migrate/sql/00NN_team_policy.sql` — `team_policies`, numbered after
  task 04's `00NN_team.sql` (new; see Gap 1)
- `control-plane/internal/hub/server/auth.go` (Hub task 13, task 07) — `Claims.Compliance`,
  `Principal.Compliance`, `AuthConfig.ComplianceRole` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerTeamPolicy()` (modify: one line)
- `control-plane/internal/hub/hubapi/team.go` (task 03) — `DefaultTeamPolicy()`,
  `CodeNeedsCompliance`, `CodeBelowFloor` (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` (Hub task 16b) — the team
  columns, `teamRow`, `allTeamRows`, and the meta-rule extended (modify)
- `control-plane/internal/hub/server/teammatrix_integration_test.go` — every team row (new)
- `control-plane/internal/hub/server/teampolicy_integration_test.go` (new)
- `control-plane/internal/hub/hubclient/{hubclient.go,fake.go}` (task 08) — `TeamPolicy`,
  `PutTeamPolicy`, `ChangeRoute` (modify)
- `control-plane/cmd/devissuer/dev-tenant.json` (Hub task 11) — **Priya**, `Hub.Compliance`, no
  team (modify)
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` — §5.3 team rows for 74, 75, 79; §7.24's
  two codes (modify)

**Interfaces:**
- Consumes:
  - task 01: routes 74, 75, 79 and §7.24 (`task-01.md:279-284`, `:466-488`); the §8 rows
    `team_policy.updated` and `team_policy.route_changed` (`:531-532`); the team §5.3 table and its
    column legend (`:547-597`).
  - task 03: `TeamPolicy`, `PutTeamPolicyRequest`, `RouteChangeRequest`, `Quotas`,
    `MessageBudgets`, `EscalationFloor` and its four values, `InferenceRoute`, `DataClass`,
    `RoutePolicyRead`, `AppRoleAuditor`, `AppRoleCompliance` (`task-03.md:272-278`, `:358-359`,
    `:388`, `:890-924`).
  - task 06: `RequireDesktop`, `RequireFleet` (`task-06.md:267-288`), `teamWorld`, `everyTeamWrite`
    (`:461-473`, `:555-556`); task 07: `newTeam2World`, Audrey (`task-07.md:311-316`).
  - Hub task 16b: `row`, `column`, `fixture`, `allRows`, `TestEveryRouteHasAMatrixRow`,
    `disableGoAuthz`, `server.Patterns()`; Hub task 14: `Principal`, `Tx`, `Refuse`, `Forbidden`,
    `Emit`.
  - phase 6 task 01 (planned): `dataclass.Class`, `dataclass.Route`, `dataclass.Floor`,
    `dataclass.CanChange`, `dataclass.Actor`, `ErrNeedsCompliance`, `ErrBelowFloor`.
- Produces: routes 74, 75, 79; `server.Can`, `server.TeamCap`; `Principal.Compliance`;
  `hubapi.DefaultTeamPolicy`; codes `needs_compliance` (403) and `below_floor` (422);
  `teamRow`, `teamCell`, `allTeamRows`; `hubclient.TeamPolicy`, `PutTeamPolicy`, `ChangeRoute`.
- Required by: task 37 (quotas, turn budget), task 28 (message budgets), task 30 (floors), task 44
  and 46 (their Z5 rows join `allTeamRows`), task 56 (the route badge), task 67 (quotas admin),
  task 57 (TM1 proof).

#### `teamcaps.go` — the §3.4 table, complete source

```go
// control-plane/internal/hub/server/teamcaps.go
package server

// TeamCap is one row of 05-design.md §3.4. Handlers of routes 74, 75 and 79
// call Can; tasks 06, 07 and 46's handlers keep their own tested checks, and
// TestCapTableAgreesWithTheMatrix holds all of them to this one table.
type TeamCap string

const (
	CapSeeTeam       TeamCap = "see-team"        // strip, cards, observed files (52, 55, 59, 73, 74)
	CapLaunch        TeamCap = "launch"          // an Orchestrator or a worker; hold leases (own only)
	CapReadThreads   TeamCap = "read-threads"    // 63: parties only, through their Orchestrator
	CapReadOwnDesk   TeamCap = "read-own-desk"   // 66, 68, 71: own only
	CapWriteNote     TeamCap = "write-note"      // 72
	CapSetPolicy     TeamCap = "set-policy"      // 75: within floors
	CapRaiseRoute    TeamCap = "raise-route"     // 79 to au
	CapLowerRoute    TeamCap = "lower-route"     // 79 to global
	CapReleaseStale  TeamCap = "release-stale"   // task 06's admin release, audited
	CapReadAcross    TeamCap = "read-across"     // 63, 66, 68, 71 across owners
)

// who is one §3.4 row: which team roles, and which org capabilities, hold it.
type who struct {
	viewer, member, admin bool
	auditor, compliance   bool
}

var capTable = map[TeamCap]who{
	CapSeeTeam:      {viewer: true, member: true, admin: true, auditor: false},
	CapLaunch:       {member: true, admin: true},
	CapReadThreads:  {member: true, admin: true},
	CapReadOwnDesk:  {member: true, admin: true},
	CapWriteNote:    {member: true, admin: true},
	CapSetPolicy:    {admin: true},
	CapRaiseRoute:   {member: true, admin: true},
	CapLowerRoute:   {compliance: true},
	CapReleaseStale: {admin: true},
	CapReadAcross:   {auditor: true},
}

// Can reports whether p, holding teamRole on the project's team ("" when
// none), has cap. Owner-bound rules ("own only", "parties only") are the
// handler's, after Can: Can answers "may this kind of person ever".
func Can(p Principal, teamRole hubapi.Role, cap TeamCap) bool {
	w, ok := capTable[cap]
	if !ok {
		return false // an unknown capability is nobody's
	}
	switch {
	case w.auditor && p.Auditor, w.compliance && p.Compliance:
		return true
	}
	switch teamRole {
	case hubapi.RoleViewer:
		return w.viewer
	case hubapi.RoleMember:
		return w.member
	case hubapi.RoleAdmin:
		return w.admin
	}
	return false
}
```

**§3.4 says the auditor sees the team strip** ("yes", `05-design.md:349`); task 01's matrix says
`Au` is 404 on route 74's read row (`task-01.md:579`). The later text wins (task 01 was written
against §3.4 and chose "reads escalations, directives, human messages and threads … and nothing
else of project content", `:547-548`), so `CapSeeTeam` holds `auditor: false`, and Gap 3 names
the disagreement for the owner. `hubapi.Role` and its three values are built
(`hub …/control-plane/internal/hub/hubapi/types.go:23-25`, checked at `9f2fa31`).

#### `auth.go` (Hub task 13, task 07) — the compliance capability

Beside task 07's `Auditor`, complete:

```go
// in AuthConfig
	ComplianceRole string // "Hub.Compliance" (hubapi.AppRoleCompliance)

// in Claims, beside OrgAdmin and Auditor, read from THIS token's roles only
	Compliance bool

// in Principal (tx.go)
	Compliance bool // org compliance: may lower a team project's route (79), nothing else
```

`Compliance`, like `OrgAdmin`, is read from each token's `roles` and never cached across tokens
(`hub …/task-13.md:36`, `:172`). It grants **no** team membership: Priya, with the role and no
team, is `N` on every other route (404), which the matrix pins.

#### Team policy — defaults, codes and the migration

`hubapi/team.go` (task 03), added, complete:

```go
// DefaultTeamPolicy is §7.24's product default: every quota on (OD-24), no
// extra asking, the tightest message budgets §6.6 allows. Revision 0 means
// "no row yet"; the first PUT names revision 0.
func DefaultTeamPolicy(projectID string, class DataClass, route InferenceRoute) TeamPolicy {
	return TeamPolicy{
		ProjectID: projectID, DataClass: class, Route: route,
		Quotas:                   Quotas{MemberDailyCents: 2500, ProjectDailyCents: 20000},
		OrchestratorTurnsPerHour: 30,
		EscalationFloors:         []EscalationFloor{},
		MessageBudgets:           MessageBudgets{PerPairPerHour: 20, RoundsPerSide: 3, ThreadDeadlineMinutes: 30},
		Revision:                 0,
	}
}

const (
	CodeNeedsCompliance = "needs_compliance" // 403: lowering a route needs Hub.Compliance
	CodeBelowFloor      = "below_floor"      // 422: the data class does not permit that route
)
```

The two codes are appended in contract §4.2's order, so `keys_test.go`'s own-code count rises by
exactly two from its value at merge (task 46 raises it by three; the order of merge decides the
final number, never the delta).

`migrate/sql/00NN_team_policy.sql`, complete. The data class and the route are **not** columns
here (decision a).

```sql
-- One row per team project, created by the first PUT (route 75).
CREATE TABLE team_policies (
  org_id                      text        NOT NULL,
  project_id                  text        NOT NULL,
  member_daily_cents          bigint      NOT NULL CHECK (member_daily_cents > 0),
  project_daily_cents         bigint      NOT NULL CHECK (project_daily_cents > 0),
  orchestrator_turns_per_hour int         NOT NULL CHECK (orchestrator_turns_per_hour BETWEEN 1 AND 30),
  escalation_floors           text[]      NOT NULL DEFAULT '{}' CHECK (escalation_floors <@
                                ARRAY['yield-unedited','accept-split','proceed-both-ps','sequence-short']),
  per_pair_per_hour           int         NOT NULL CHECK (per_pair_per_hour BETWEEN 1 AND 20),
  rounds_per_side             int         NOT NULL CHECK (rounds_per_side BETWEEN 1 AND 3),
  thread_deadline_minutes     int         NOT NULL CHECK (thread_deadline_minutes BETWEEN 30 AND 1440),
  revision                    int         NOT NULL CHECK (revision >= 1),
  updated_at                  timestamptz NOT NULL,
  PRIMARY KEY (org_id, project_id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id)
);
ALTER TABLE team_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_policies FORCE ROW LEVEL SECURITY;
CREATE POLICY team_policies_org ON team_policies
  USING (org_id = current_setting('hub.org_id'))
  WITH CHECK (org_id = current_setting('hub.org_id'));
GRANT SELECT, INSERT, UPDATE ON team_policies TO hub_app;
```

The composite foreign key follows Hub correction A16 (`hub …/phase-h-hub.md:195`): a cross-org
link is refused by the database with every Go check off. The `CHECK`s repeat the handler's bounds
so a handler bug cannot store a zero quota or a floor that is not one of the four. The RLS policy
name and the `hub.org_id` setting follow Hub task 07a's pattern (**UNVERIFIED until merge**; Step
1 takes the real names from the merged migration).

#### The routes — complete wire

**Route 74, `GET /v1/projects/{p}/team-policy`.** Desk (`V`, `M`, `A`), or Fleet under a
delegation listing `policy.read`. 200 `TeamPolicy`: the row, or `DefaultTeamPolicy` with
`revision: 0`; `dataClass` and `route` always from the project record. 404 `project_not_found` for
a non-member. A solo project (mode not `team`) → 409 `team_state` `{"from":"solo"}`.

**Route 75, `PUT /v1/projects/{p}/team-policy`.** `RequireDesktop`, then `Can(p, role,
CapSetPolicy)` or 403 `forbidden` `required: "admin"`. Body `PutTeamPolicyRequest`, strict: a
`dataClass`, `route`, `projectId` or `updatedAt` key is 400 `bad_request` `details.field`. Each
bound of §7.24 is 422 `bad_request` `details.field` (`quotas.memberDailyCents`, …). A floor
outside the four, or repeated, is 422 `details.field = "escalationFloors"`. `revision` must equal
the stored one (0 when none) or 409 `revision_conflict`. 200 `TeamPolicy`, revision + 1; event
`team_policy.updated` `{"revision"}`.

**Route 79, `POST /v1/projects/{p}/team-policy/route`.** `RequireDesktop`. Body
`RouteChangeRequest`. The handler reads the project's class and current route, then:

```go
	actor := dataclass.Actor{Member: Can(p, role, CapRaiseRoute), Compliance: p.Compliance}
	switch err := dataclass.CanChange(class, from, to, actor); {
	case errors.Is(err, dataclass.ErrNeedsCompliance):
		return Refuse(403, hubapi.CodeNeedsCompliance, "Only compliance may lower this route.", nil)
	case errors.Is(err, dataclass.ErrBelowFloor):
		return Refuse(422, hubapi.CodeBelowFloor, "This project's data class requires AU.", nil)
	case err != nil:
		return Refuse(422, hubapi.CodeBadRequest, err.Error(), map[string]string{"field": "route"})
	}
```

A non-member with `Compliance` passes this (Priya is no member), so route 79 is the one team route
where `N` with the role is not 404. A same-route request is 200 with no event. Otherwise 200
`TeamPolicy`, and one event `team_policy.route_changed` `{"route", "by"}`, `by` = `compliance`
when the change lowered the route, else `member`.
