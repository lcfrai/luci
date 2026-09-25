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
