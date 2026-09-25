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
	if role == "" && !p.Compliance { // not on the team, and not compliance
		return Refuse(404, hubapi.CodeProjectNotFound, "No such project.", nil)
	}
	actor := dataclass.Actor{Member: Can(p, role, CapRaiseRoute), Compliance: p.Compliance}
	if !actor.Member && !actor.Compliance { // a viewer
		return Forbidden("member")
	}
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

#### The Z5 matrix for team routes — the harness, complete

Hub task 16b's `row` has six columns (`want [6]int`, `hub …/task-16b.md:50-58`). Team rows need
eleven, and three kinds of 403 and an "empty list" that a bare status cannot say, so they get their
own row type beside it, in the same file, run by the same fresh-fixture loop.

```go
// control-plane/internal/hub/server/matrix_integration_test.go (added)

// teamColumn is task-01.md:554-560's legend. "Own" and "another's" are about
// the record the row's path names: Linus's record unless the row says so.
type teamColumn int

const (
	tcV     teamColumn = iota // Vera, viewer of T
	tcM                       // Linus, on his own record
	tcMp                      // Linus, on Grace's record (M′)
	tcA                       // Grace, admin of T, on Linus's record
	tcAu                      // Audrey, org auditor, in no team
	tcN                       // Out Sider, same org, in no team
	tcX                       // Xavier, another enrolled org
	tcF                       // Fleet under Linus's live delegation listing the route
	tcFnone                   // Fleet naming no delegation
	tcFx                      // Fleet under Grace's delegation, on Linus's record
	tcD                       // Linus's desktop token sending X-Panacea-Delegation
	teamColumns
)

// teamCell is one expected answer.
type teamCell struct {
	status   int
	code     string // the problem code, "" when the status alone is the answer
	required string // details.required on a 403 forbidden
	empty    bool   // 200 whose list holds none of the other person's rows (∅)
}

var (
	ok      = teamCell{status: 200}
	created = teamCell{status: 201}
	gone    = teamCell{status: 404}
	d401    = teamCell{status: 401, code: "delegation_required"}
	a403    = teamCell{status: 403, code: "agent_not_owner"}
	f403    = teamCell{status: 403, code: "forbidden", required: "fleet"}
	k403    = teamCell{status: 403, code: "forbidden", required: "desktop"}
	r403    = func(req string) teamCell { return teamCell{status: 403, code: "forbidden", required: req} }
	empty   = teamCell{status: 200, empty: true}
)

// teamRow is one team route (or one case of it). path uses teamFixture's
// fields; mine selects Linus's record, theirs Grace's (for the M′ column).
type teamRow struct {
	name   string
	method string
	path   func(f *teamFixture, record string) string
	body   func(f *teamFixture, record string) any
	want   [teamColumns]teamCell
	ownA   bool // task 01's "✓ own" in A: the tcA cell runs Grace on her own record
}

// teamFixture is task 07's newTeam2World plus Priya (compliance) and one
// record of each kind per owner, built fresh per cell.
type teamFixture struct {
	w                        *team2World
	LinusTask, GraceTask     string // tk-…
	LinusLease, GraceLease   string // ls-…
	LinusEsc, GraceEsc       string // es-…
	Thread                   string // th-…, Linus's and Grace's Orchestrators
}

// allTeamRows is appended to by each team task's var block: rowsT06, rowsT07,
// rowsT43, rowsT44, rowsT46.
var allTeamRows = [][]teamRow{rowsT06, rowsT07, rowsT43}

// orgRows are the cells no team column holds: Ada (org admin) and Priya
// (compliance), one row per route that treats them specially.
var orgRows = []orgRow{ /* see the table below */ }

type orgRow struct {
	name   string
	who    string // "ada" | "priya"
	method string
	path   func(f *teamFixture) string
	body   func(f *teamFixture) any
	want   teamCell
}
```

`TestTeamAuthorisationMatrix` runs every `teamRow` × every column and every `orgRow` on a fresh
`teamFixture`; for `empty` it also asserts the list holds none of the other owner's ids. Hub task
16b's `TestEveryRouteHasAMatrixRow` is extended: a `/v1` pattern is covered by a row in `allRows`
**or** in `allTeamRows`. `TestTheXColumnHoldsWithoutGoChecks` gains the `tcX` cell of every team
row, with `disableGoAuthz` on.

#### The rows

Task 01's team table (`task-01.md:562-579`) is the source for routes 46–73, cell for cell; this
part only translates its marks: `✓` is `ok`/`created`, `401d` is `d401`, `403a` is `a403`, a `—`
in a desk column is `f403`, a `—` in a Fleet column is `k403`, `∅` is `empty`, and a bare `403`
is `r403(<the route's required role>)`. `rowsT06` and `rowsT07` are generated from task 06's
`everyTeamWrite` and task 07's route list, one row per (route, case), so the two cannot drift:
`TestEveryTeamWriteHasAZ5Row` fails if `everyTeamWrite` names a (method, path) with no row.
`rowsT43` is new:

| Operation | V | M | M′ | A | Au | N | X | F | F∅ | Fx | D |
|---|---|---|---|---|---|---|---|---|---|---|---|
| read team policy (74) | ✓ | ✓ | ✓ | ✓ | 404 | 404 | 404 | ✓ | 401d | ✓ | 403 |
| set team policy (75) | 403 admin | 403 admin | 403 admin | ✓ | 404 | 404 | 404 | k403 | 401d | k403 | 403 |
| raise the route to `au` (79) | 403 member | ✓ | ✓ | ✓ | 404 | 404 | 404 | k403 | 401d | k403 | 403 |
| lower the route to `global` (79) | 403 member | 403 needs_compliance | 403 needs_compliance | 403 needs_compliance | 404 | 404 | 404 | k403 | 401d | k403 | 403 |

Route 74 to 79 are project-level, so `M` and `M′` are the same person on the same record and hold
the same cell; the column stays so the table has one shape. `D` (a desktop token that sends a
delegation header) is 403 on every team route: a desktop acts as its user, never as a delegate
(`task-01.md:560`).

`orgRows`, complete:

| Who | Operation | Want |
|---|---|---|
| Priya (compliance, no team) | lower the route of an `internal` · `any` project (79) | `ok`; event `by: "compliance"` |
| Priya | lower the route of an `au-only` project (79) | 422 `below_floor` |
| Priya | read team policy (74), set team policy (75) | `gone`, `gone` |
| Priya | read escalations, directives, human messages, thread messages (66, 68, 71, 63) | `gone` |
| Ada (org admin, no team) | set team policy (75), lower the route (79) | `gone`, `gone` |
| Ada | read escalations (66) | `gone`: the org admin is not an auditor |
| Audrey (auditor) | raise the route (79), set team policy (75) | `gone`, `gone` |

**The rule-5 check is structural, not per row.** `TestNoWriteRowLetsAnyoneWriteAnothersRecord`
walks `allTeamRows`: for every row whose method is not `GET`, the cells `tcMp`, `tcA`, `tcAu` and
`tcFx` must each be a status ≥ 400, with three listed exceptions, each by route number: the
project-level rows 74–79; route 72, because a note is its **author's** record whatever it is about
(`task-01.md:577`); and the `✓ own` cells of routes 46 and 70 (`:564`, `:576`), which a row marks
`ownA: true` so its `tcA` cell runs Grace on **her own** record. A later task adding a write row
with a `✓` in one of those cells fails the build before any handler runs.

**`TestCapTableAgreesWithTheMatrix`** maps each `TeamCap` to the rows of its routes
(`CapReadAcross` → 63, 66, 68, 71's `tcAu` cells; `CapSetPolicy` → 75; `CapLowerRoute` → the
Priya row; …) and asserts `Can`'s answer for each column's principal equals "the cell is ≤ 299".
`capTable` and the matrix can then only change together.

#### Tests to write first

Every Hub test is `//go:build integration` on Hub task 06's harness and task 07's
`newTeam2World`, plus Ada (org admin), Audrey (auditor, task 07), Priya (compliance, added here
to `dev-tenant.json`) and Xavier (org 2).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTeamAuthorisationMatrix` | every `teamRow` × every column, and every `orgRow`, on a fresh fixture: status, code, `details.required`, and for `empty` no other owner's id in the list | any missing or loosened check in any team handler |
| `TestAMemberLoweringTheRouteIs403` | a project at `au`; Linus `POST`s route 79 `{"route":"global"}` → 403 `needs_compliance`; the project's route is still `au`; no `team_policy.route_changed` event | a lowering allowed to any member, or refused after the write |
| `TestAFloorThatRemovesAskingIsRefused` | Grace `PUT`s `escalationFloors: ["free-directive-fleet-turn"]` (a row whose default is already *ask*) → 422 `details.field = "escalationFloors"`; a body key `escalationDefaults` → 400 `details.field`; `["yield-unedited","yield-unedited"]` → 422 | a floor list validated by type, not by the four values; a way to say "ask less" |
| `TestComplianceLowersAndItIsAudited` | Priya lowers an `internal`·`any` project → 200; the audit (route 44) holds one `team_policy.route_changed` `{"route":"global","by":"compliance"}`; an `au-only` project → 422 `below_floor` and no event | a missing `by`; the floor check skipped for compliance |
| `TestTheDefaultsAreOnBeforeAnyPut` | route 74 on a fresh team project → `revision: 0`, every quota > 0, turns 30, budgets `{20, 3, 30}`, floors `[]`; `dataClass` and `route` equal the project record's | a zero quota default (OD-24); a policy row as the class's source |
| `TestPolicyBoundsTightenOnly` | each §7.24 bound at its edge passes and one past it fails with its own `details.field`: turns 0 and 31, pair 21, rounds 4, deadline 29 and 1441, a quota 0 | a bound off by one, or reported on the wrong field |
| `TestAStalePutConflicts` | two `PUT`s naming `revision: 0` → the second 409 `revision_conflict`; the row's revision is 1 | a lost update between two admins |
| `TestDataClassIsNotWritableHere` | a `PUT` carrying `dataClass`, `route`, `projectId` or `updatedAt` → 400 naming the field; the project record is unchanged | the policy route as a back door onto the class |
| `TestTheAuditorReadsAcrossAndWritesNothing` | Audrey → 200 with both owners' rows on 63, 66, 68, 71; every write row's `tcAu` cell ≥ 400 | an auditor granted a write, or filtered to an owner |
| `TestComplianceIsNotMembership` | Priya → 404 on every team route but 79 | the role treated as a team seat |
| `TestNoWriteRowLetsAnyoneWriteAnothersRecord` | the structural rule-5 walk above | a `✓` added in an `M′`, `A`, `Au` or `Fx` write cell |
| `TestCapTableAgreesWithTheMatrix` | `Can` equals each mapped cell's success | the table and the handlers drifting apart |
| `TestEveryTeamWriteHasAZ5Row` | every `everyTeamWrite` case has a `rowsT06` row | a route added in task 06's list and not here |
| `TestTheXColumnHoldsWithoutGoChecks` (Hub 16b's, extended) | every team row's `tcX` cell is 404 or `empty` with `disableGoAuthz` on; no 500 | a tenancy line held by Go alone on a team route |

`TestAMemberLoweringTheRouteIs403` and `TestAFloorThatRemovesAskingIsRefused` are the scope's
(`2026-09-25-phase-t-team-mode.md:895`) and are written first, with the matrix.

#### Fleet reads the policy

`hubclient` (task 08) gains three methods and their fake, complete:

```go
// TeamPolicy is route 74 under the caller's delegation (Fleet) or token (desk).
func (c *Client) TeamPolicy(ctx context.Context, projectID string) (hubapi.TeamPolicy, error)
// PutTeamPolicy is route 75; the desktop's (task 67) only.
func (c *Client) PutTeamPolicy(ctx context.Context, projectID string, r hubapi.PutTeamPolicyRequest) (hubapi.TeamPolicy, error)
// ChangeRoute is route 79; the desktop's (task 56) only.
func (c *Client) ChangeRoute(ctx context.Context, projectID string, r hubapi.RouteChangeRequest) (hubapi.TeamPolicy, error)
```

Fleet reads route 74 at rebuild (task 11) and again on each `team_policy.updated` or
`team_policy.route_changed` in its per-org feed (task 08's kinds), and holds the result in memory
only: quotas and turns for task 37, budgets for task 28, floors for task 30. A Fleet that cannot
read the policy uses `DefaultTeamPolicy`'s numbers **and** the AU route whenever the project's
route is unknown: fail closed toward AU, never toward Global (phase 6's P6-D5,
`phase-6-gateway.md:69`).
