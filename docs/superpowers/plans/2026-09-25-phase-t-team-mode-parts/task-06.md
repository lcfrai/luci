### Task 06: Hub routes, part 1: delegations, tasks, agents, lease events, conflicts, the principal read

**Slice TM1-α. Depends on: 05, H14, H17. About 4 hours (route 87 added by fix batch b01; route 88
and route 47's broker caller by fix batch b02, reviews S02-C2 and S02-C5).
Seams: ST.2 (defines the routes), ST.3 (creates the delegation record task 05 authenticates).**

This task gives the Hub its first team writes. It is a **delta on the Hub branch**, built on
`feat/team-mode` only after the sync-merge of `feat/hub`, and it extends two sibling files that are
**planned, not built** on 2026-09-25:

- `control-plane/internal/hub/server/tx.go`: Hub task 14's `Write`, `Read`, `Tx`, `Principal`,
  `Emit`, `Refuse`, `Forbidden` and `Decode` (Hub plan
  `.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-14.md:19-139`,
  read 2026-09-25). **UNVERIFIED until merge.**
- `control-plane/internal/hub/server/projects.go` and `authz.go`: Hub task 17's `ProjectRole(ctx,
  tx, projectID) (role, teamID, ok, err)` (`task-17.md:48`). **UNVERIFIED until merge.**

Every handler below is one `s.Write(...)` call, so each write already gets the org's audit lock,
idempotency (contract §10.6), one audit event per changed record and commit-before-reply. This task
adds no transaction machinery of its own.

**The one thing this task must not get wrong is who the owner is.** A team write names no owner in
its body. The owner is `tx.P.UserID`, and for a Fleet call task 05 has already made that the
delegation's owner and set `tx.P.Via = hubapi.ViaFleet` (`"fleet"`, `task-03.md:361-362`) and
`tx.P.DelegationID` (`task-05.md:386-391`). So every handler compares
record owners with `tx.P.UserID` and never reads a user id out of JSON. A request type that grew an
`ownerUserId` or `actor` field would make task 01's acceptance grep fail, and `Decode` refuses unknown
fields, so a body that sends one gets 400 `bad_request`.

**Two callers, two kinds of route.**

| Route group | Who may call | Why |
|---|---|---|
| routes 46, 48, 49 (create, renew, revoke a delegation) | the user's **desktop** token only; Fleet cannot reach them, because they have no route name to delegate | the user's own call creates the delegation (§2.4 item 2); Fleet cannot extend its own authority |
| routes 50, 51, 53, 54, 56, 57, 58 (task, agent, lease event, conflict writes) | **Fleet under a delegation** only; a desktop token is 403 `forbidden`, `required: "fleet"` (`task-01.md:581-583`) | these records are written by an Orchestrator through `panacea` or by Fleet's forecaster (§3.3 "Written by") |
| routes 47, 52, 55, 59 (reads) | the desktop, or Fleet under a delegation listing the read | the team strip is visible to every project role (§3.4 row 1) |
| route 87 (a project's mode, fix batch b01) | the **desktop** of the project's team `admin` only; no route name | turning a project to team is a human act, one way, behind the audit gate |
| route 88 (the principal read, fix batch b02) | **Fleet, with no delegation** — §5.10 rule 2's second and last exemption (`task-01.md`, amendment (b)). A desktop token is 403 `forbidden`, `required: "fleet"`; a Fleet token carrying the header is 400 `bad_request` | it is what tells Fleet *who a caller is*, so it is called before a delegation is in hand. Without it no Fleet route authorises at all (task 12's `Membership`, review S02-C2) |
| route 47, third caller (fix batch b02) | also **the broker**, app-only, role `Hub.DelegationReader`, §5.10 rule 10: no delegation header, any delegation of its own org, revoked or expired included, and **no other route** | the broker must derive a pod's owner label itself rather than take Fleet's word for it (`task-17.md:47-54`, gap S17-1, review S02-C5) |

**Files:**
- `control-plane/internal/hub/server/delegations.go` + `delegations_integration_test.go` (new)
- `control-plane/internal/hub/server/teamtasks.go` + `teamtasks_integration_test.go` (new)
- `control-plane/internal/hub/server/agents.go` + `agents_integration_test.go` (new)
- `control-plane/internal/hub/server/leaseevents.go` + `leaseevents_integration_test.go` (new)
- `control-plane/internal/hub/server/conflicts.go` + `conflicts_integration_test.go` (new)
- `control-plane/internal/hub/server/teamguard.go` + `teamguard_test.go`: the shared guards (new)
- `control-plane/internal/hub/store/{delegations,teamtasks,agents,leaseevents,conflicts}.go` (new)
- `control-plane/internal/hub/server/server.go`: `s.registerTeamPart1()` (modify: one line)
- `control-plane/internal/hub/server/sessionlinks.go` (Hub task 20's route 33): the team-project
  refusal (modify: one check)
- `control-plane/internal/hub/server/projectmode.go` + `projectmode_integration_test.go`: route 87
  and `teamModeGate` (new; fix batch b01)
- `control-plane/internal/hub/store/projectmode.go`: `ProjectMode(ctx, tx, id)` (read under route
  46 and 87) and `SetProjectMode(ctx, tx, id)`, the only `SET mode` statement (new; fix batch b01)
- `control-plane/internal/hub/store/projects.go` (Hub task 17's project store, `task-17.md:14`):
  its one project-row reader, shared by routes 13 to 16 and by route 87's reply, selects and scans
  `mode` into `hubapi.Project.Mode`, and route 14's `INSERT ... RETURNING` returns it (modify: the
  column list and the scan; fix batch b01 round 2, re-review I-1). Without it every Hub read answers
  `"mode":""` and the control plane never sees a team project
- `control-plane/internal/hub/server/projects_integration_test.go` (Hub task 17's file):
  `TestProjectReadsCarryMode` (modify: one test; fix batch b01 round 2)
- `control-plane/cmd/hub/config.go` + `config_test.go`: `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED`,
  refused unless `PANACEA_APP_HUB_ENV` is `local` or `dev` (modify; fix batch b01); and
  `PANACEA_APP_HUB_BROKER_CLIENT_ID`, a start-up error when it equals
  `PANACEA_APP_HUB_FLEET_CLIENT_ID`, appended to `entra.Config.AppCallers` as
  `{ClientID: broker, Role: hubapi.AppRoleDelegationReader}` (task 03) beside Fleet's (modify; fix batch b02)
- `control-plane/internal/hub/server/principals.go` + `principals_integration_test.go`: route 88,
  contract §7.28 (new; fix batch b02, S02-C2)
- `control-plane/internal/hub/store/principals.go`: `PrincipalFor(ctx, tx, tid, oid, projectID)`,
  one statement joining the user, the project, its team and the org-role rows; it returns the
  **project's** `orgId`, never the caller's (new; fix batch b02)
- `control-plane/internal/hub/server/auth.go` (task 05's file): **four** deltas, not two — this
  task adds the second app-only identity, so it owns every check that until now read `AppOnly` and
  meant "Fleet" (modify; fix batch b02, round 1 re-review C1 and C2):
  1. `Claims.AppRole`, the `Role` of the `entra.AppCaller` that matched, so a handler can tell
     Fleet from the broker;
  2. `isExemptPath` = `isFeedPath(r) || isPrincipalPath(r)` — task 05's route-78 matcher is kept
     under its own name, because only route 78 sets `FleetFeed`, and route 88 gets the same
     exact-segment matcher of its own — `isPrincipalPath` is `GET` and segments exactly
     `v1`, `principals`, never a prefix, so `/v1/principals/anything` is not exempt;
  3. `delegationHeader` gains the `Hub.DelegationReader` arm of §5.10 rule 10 — no header
     required, a header 400, every path but `GET /v1/delegations/{dl}` 403 before the route runs —
     and its two exempt-path cases are **narrowed to `AppRoleFleet`**, so the broker can never
     become `FleetFeed` (the hole C2 names: route 78's "Fleet's workload identity only" had no
     mechanism but `AppOnly`);
  4. `resolveCaller` gains the broker branch: **no principal**, tenancy from the token's own `tid`.
  Task 05's `AppCallers` list already admits more than one identity (`task-05.md:149-155`), so no
  shape of it changes, and task 05's own `AppOnly ⇒ Fleet` reading is sound until this task runs:
  `task-05.md:622` configures exactly one caller
- `control-plane/internal/entra/entra.go`: nothing. `AppCallers` is already a list (**verified**
  in `task-05.md:145-156` on 2026-09-28); the broker is one more element

**Interfaces:**
- Consumes:
  - from task 01: the contract §7.13 to §7.17, rows 46 to 59, 87 and 88, the class route sets, the
    principal read §7.28 and the §8 rows (`task-01.md:392-471`; rows `:313-326` with `:347` and
    `:348`; `:363-372`; `:589-628`; the §8 table from `:663`) — one each, in that order. Every
    anchor re-derived by grep at `feat/team-mode` on 2026-09-28, **after** b02 round 1's edits:
    b02's first pass copied ranges from before its own, which is the drift round 1's I3 named, so
    grep the heading rather than trust a range that looks close.
    **Task 01 is authoritative for every field, bound, code and kind; this part implements it.**
  - from task 03 (`hubapi/team.go`, `codes.go`): the Go types of those records and requests
    (`Delegation`, `CreateDelegationRequest`, `Task`, `CreateTaskRequest`, `PatchTaskRequest`,
    `AgentStart`, `AgentStartRequest`, `AgentEndRequest`, `LeaseEventRequest`, `LeaseEvent`,
    `Conflict`, `CreateConflictRequest`, `PatchConflictRequest`), the enums with `Valid()`, the
    codes of `task-01.md:198-210` including `delegation_not_found`, and `CodeConflict` (409
    `conflict`). Task 03 keeps these names (`task-03.md:27-32`) and wins wherever the two differ.
    Also its `RouteName` constants (`task-03.md:369-395`), `ClassRoutes`, `DelegationClass`
    (`DelegationClassOrchestrator`, `DelegationClassWorker`, `task-03.md:107-108`) and `ViaFleet`.
    For route 87 (fix batch b01): `ProjectMode` (`ProjectModeSolo`, `ProjectModeTeam`),
    `hubapi.Project.Mode` and `SetProjectModeRequest` (task 03, Block 7); the kind `project.mode_set` is a string.
  - from task 04: the tables `delegations`, `team_tasks`, `agents`, `lease_events`, `conflicts`,
    `human_messages` under the Hub's RLS pattern, the audit row's `fleet_feed` flag, and a
    project's `mode` (`solo`·`team`) with its one-way trigger and `GRANT UPDATE (mode)`
    (`task-04.md:218-230`, `:800`), which route 87 writes and route 46 reads.
  - from task 05: `Principal.Via` (`""` or `"fleet"`), `Principal.DelegationID`,
    `Principal.Delegation` (a `DelegationView`: project, class, routes, expiry), the route-name
    check, the v2 audit write, and `store.AgentOwnedBy(ctx, tx, agentID, userID) (bool, error)`
    (`task-05.md:115-121`, `:303-311`, `:506-510`); and `server.Delegable(name, h)`, which every
    Fleet-reachable route here is registered through (task 05 decision (d), `task-05.md:49-53`,
    `:837-839`). A route registered bare has no name, so Fleet calling it is 403.
  - from task 02: `teamtext.Bound`, `teamtext.Clean`, `teamtext.ValidGlob`, `teamtext.Author`.
- Produces: the ten write and four read handlers below, plus route 87's write (`projects.mode`,
  the only writer) and `Server.teamModeGate` with `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED`, the
  TM1-α seam task 45 fills; `server.RequireDesktop`,
  `server.RequireFleet`, `server.OwnedAgent`, `server.EitherAgent`; `store.HumanMessageEquals`
  (reads task 04's `human_messages` table, for `titleSource`); `renderTask(p, t)`.
- Required by: task 07 (reuses the four guards), task 08 (feed kinds; `hubclient.Team` methods,
  one per route here), task 11 (Fleet's delivery calls), task 14 (the agent start carries the
  policy version and digest), task 21 (the desktop creates the delegation at launch), task 43 (Z5
  rows), task 44 (revocation end to end); for route 87: task 08 (`hubclient.SetProjectMode`),
  task 21 (`TeamProjects.MakeTeam`), task 33 (E5 runs it), task 45 (fills the gate), task 56 (the
  Share path calls it).

#### The routes — complete wire

Conventions are the Hub contract's §1 (timestamps RFC 3339 UTC to the millisecond, ids by
`hubapi.ValidID`, unknown fields refused, problem bodies per §4). `{p}` is a `pj-…` id.

**This section implements task 01's contract §7.13 to §7.17 and rows 46 to 59 of its §7.0 team
table (`task-01.md:329-400`, `:259-272`); where anything below disagrees with task 01, task 01
wins.** Request free text is a plain string: the Hub bounds and cleans it (task 02), stamps the
author class from the route (a Fleet route writes `agent`, a desk route `human`) and answers it as
a `Text` object (`task-01.md:287-290`). Fleet mints every id here but `dl-…` and sends it as the
body's `id` (`task-01.md:172-187`); a reuse with a different body is 409 `conflict`, a retry with
the same `Idempotency-Key` replays.

**Route 46, `POST /v1/delegations`**: desktop only; project role `member` or `admin` (`viewer`
403, §3.4 row 2). `routes` is 1 to 25 distinct names from the **class's set**
(`task-01.md:274-283`), else 422 `bad_request`, `details.route`.

```jsonc
// request
{ "projectId": "pj-…", "class": "orchestrator" | "worker",
  "routes": [ "tasks.create", "agents.start" ] }
// 201
{ "id": "dl-…", "projectId": "pj-…", "ownerUserId": "u-…", "class": "orchestrator",
  "routes": [ "tasks.create", "agents.start" ],
  "createdAt": "…", "expiresAt": "…",                     // createdAt + PANACEA_APP_HUB_DELEGATION_TTL (24h)
  "renewedAt": null, "revokedAt": null, "revokedReason": "" }
```

No delegation route (46 to 49) has a route name, so no delegation can create, renew or revoke
another. Fleet calling route 46 is refused by task 05's route check (403 `forbidden`) before this
handler runs; `RequireDesktop` below is the second layer. `ownerUserId` is `tx.P.UserID`, and it
appears in responses only.

**Route 46 on a solo project is 409 `conflict`, `details.reason = "not_team_project"`** (fix
batch b01, X-C1). The handler reads `projects.mode` in the write's transaction and refuses unless it
is `team`. Every Fleet call but route 78 (the feed, which serves team projects only) needs a
delegation (`task-01.md:125-129`), and route 46 is the only way to make one, so this one check puts
every team record behind route 87, and route 87 behind the audit gate. Without it, the gate before the first real teammate guards a column nothing reads.

**Route 47, `GET /v1/delegations/{dl}`**: its owner's desktop, or Fleet under that same
delegation (`delegations.read`). Anyone else is 404 `delegation_not_found`, so an id does not leak.

**Route 48, `POST /v1/delegations/{dl}/renew`**: its owner's desktop only (another user 404
`delegation_not_found`). Body `{}`. Sets `expiresAt = tx.Now + TTL`, `renewedAt = tx.Now`. A
lapsed but unrevoked delegation **is** renewable, and its event carries `"lapsed": true`; a revoked
one is 409 `conflict`. → `200` the delegation.

**Route 49, `DELETE /v1/delegations/{dl}`**: its owner, or an `admin` of its project, from the
desktop. Sets `revokedAt = tx.Now` and `revokedReason` = `owner` or `admin`. A second delete is
`204` and emits nothing. → `204`. (The `member_removed` and `account_disabled` reasons are
task 44's.)

**Route 50, `POST /v1/projects/{p}/tasks`**: Fleet·O (an `orchestrator`-class delegation),
`tasks.create`.

```jsonc
// request
{ "id": "tk-…", "orchestratorAgentId": "ag-…",           // the owner's Orchestrator, not ended
  "title": "Add the export route",                         // ≤ 140
  "titleSource": "hm-…",                                   // optional: the owner's hm-… whose trimmed body equals title
  "reason": { "briefRef": "brief:bf-…@3#export", "why": "…" },  // why ≤ 200
  "scope": [ "src/export/**" ], "trackedFiles": 812,       // ≤ 20 globs; count Fleet supplies
  "state": "planned" }                                     // optional, default planned
// 201
{ "id": "tk-…", "projectId": "pj-…", "ownerUserId": "u-…", "orchestratorAgentId": "ag-…",
  "title": Text, "reason": { "briefRef": "…", "why": Text }, "scope": [ … ], "state": "planned",
  "revision": 1, "createdAt": "…", "updatedAt": "…" }
```

`title` is `author: "human"` only when `titleSource` names the owner's own human message whose
trimmed body equals it; otherwise `agent`. `briefRef` is a brief key (contract §3.2) plus `#` and a
section id `^[a-z0-9][a-z0-9-]{0,63}$`. More than 20 globs is 422 `breadth_exceeded`; a bad glob
422 `glob_refused`; a bad text 422 `text_refused` (`task-01.md:198-205`).

**Route 51, `PATCH /v1/projects/{p}/tasks/{tk}`**: Fleet·O, `tasks.update`, the task's owner only.
Body `{ "revision": 3, "state"?, "scope"?, "trackedFiles"?, "why"? }`; a stale revision is 409
`revision_conflict`. Transitions: `planned → active | abandoned`; `active → blocked | done |
abandoned`; `blocked → active | abandoned`. `done` and `abandoned` are terminal; any other move is
409 `conflict`, `details = {"from","to"}`. A patch with nothing but `revision` is 400 `bad_request`.

**Route 52, `GET /v1/projects/{p}/tasks`**: any project role from the desktop, or Fleet under
either class (`tasks.read`). **`title` and `reason.why` are answered only to Fleet·O**; every
other reader, the owner's own desktop included, gets the record without them
(`task-01.md:330-331`, `05-design.md` §5.1).

**Route 53, `POST /v1/projects/{p}/agents`**: Fleet (either class), `agents.start`.

```jsonc
{ "id": "ag-…", "role": "orchestrator" | "worker",
  "parentAgentId": "ag-…",                                 // optional; an Orchestrator has none; a worker's is the same owner's Orchestrator
  "taskId": "tk-…",                                        // optional; the owner's task, not terminal
  "harness": { "id": "claude-code", "version": "2.1.4" },  // ^[a-z][a-z0-9-]{0,31}$, ^[0-9A-Za-z.+-]{1,32}$
  "laneRef": "refs/panacea/lanes/ag-…/main",               // OD-2 prefix plus ValidBranchRef's component rules
  "policy": { "version": 3, "digest": "sha256:…" } }       // OD-23: what the sandbox mounted (task 14)
// 201: the request plus "projectId", "ownerUserId", "startedAt", "endedAt": null, "exitClass": ""
```

**Route 54, `POST /v1/projects/{p}/agents/{ag}/end`**: Fleet, `agents.end`, the agent's owner
only. `{ "exitClass": "done" | "stopped" | "failed" | "access_removed" | "lapsed" }`. A second end
is 409 `conflict`. → `200` the agent. Status (`working`, `waiting`, …) is Fleet state and never
reaches the Hub.

**Route 55, `GET /v1/projects/{p}/agents`**: as route 52, `agents.read`; `?live=true` omits ended
agents.

**Route 33 on a team project.** Contract §7.8's `session_link` stays for solo projects. On a
project whose mode is `team` (`task-01.md:474-475`), route 33 answers 409 `conflict` with
`details.use = "agents"`, so a team session cannot be linked by a machine-local id.

**Route 87, `POST /v1/projects/{p}/mode`** (fix batch b01, X-C1): the only writer of
`projects.mode`. Desktop only, `admin` of the project's team (`A`); the Hub has no project owner
role, because a project's access is exactly its team's membership (Hub contract §5.2,
`2026-09-23-hub-contract.md:326-333`), so the team admin is the owner the review names. It has
**no route name**, so no delegation can list it and Fleet is 403 before the handler runs (task 05);
`RequireDesktop` is the second layer. Registered bare in `registerTeamPart1`. Its number is 87,
the first one no part claims: task 01's §7.0 ends at 79 and 80 to 86 are taken by tasks 14, 46,
49, 52, 61, 66 to 69 (`task-14.md:600` is route 80). Task 01 adds row 87 to §7.0.

```jsonc
// request: "team" is the only value; the move is one way
{ "mode": "team" }
// 200: the project record (Hub contract §7.4) with task 03's "mode" field
{ "id": "pj-…", "teamId": "tm-…", "mode": "team", … }
```

The handler, in this order, inside one `s.Write`:

1. `RequireDesktop`; `ProjectRole` (Hub task 17): not a member 404 `project_not_found`, `viewer`
   or `member` 403 `forbidden`, `required: "admin"`.
2. `mode` other than `"team"` (`"solo"` included) is 400 `bad_request`, `details.field = "mode"`.
   The one-way trigger of task 04 is the second layer; the handler never lets it be the first,
   because its `RAISE` would answer 500.
3. **The audit gate, before any write:** `s.teamModeGate()`. Under TM1-α it is nil only when
   `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true` (below); otherwise 503 `unavailable`,
   `details.reason = "audit_not_streaming"`, and nothing is written. Task 45 swaps its body for
   `TeamModeAllowed` (`task-45.md`, Gap 1). The gate runs even when the project is already `team`,
   so a replay does not tell a caller the gate's state is open when it is not.
4. Already `team`: 200 with the record, **no event** (as route 49's second `DELETE`).
5. `UPDATE projects SET mode = 'team' WHERE id = $1` (task 04's `GRANT UPDATE (mode)`), one
   event `project.mode_set`, `data {"from":"solo","to":"team"}`; 200.

**The TM1-α audit gate, a seam.** Task 45's sink does not exist before TM1, and TM1-α's first
real run (task 33) needs a team project. So this task adds one variable to `cmd/hub/config.go`:
`PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED` (`false` by default). `true` is a **start-up error unless
`PANACEA_APP_HUB_ENV` is `local` or `dev`** (Hub contract §12.2, `:2158`), so a production Hub
cannot turn a project to team with its audit inside the building. It is a dev-only bypass of the
gate, stated as one: under it the audit stays in the Hub's own table, which an owner-role
credential can rewrite (Hub contract T5, `:2073`). Task 45 keeps the exception only with its
`fake` sink and still refuses it in `prod`.

```go
// control-plane/internal/hub/server/projectmode.go
// teamModeGate answers nil when a project may become a team project.
// TM1-α: the dev-only flag, refused in prod by loadConfig. Task 45 replaces the
// body with `return s.TeamModeAllowed()`, which keeps the flag's exception.
func (s *Server) teamModeGate() error {
	if s.cfg.TeamModeUnstreamed {
		return nil
	}
	return Refuse(http.StatusServiceUnavailable, hubapi.CodeUnavailable,
		"Team projects need the audit to leave the Hub first; it is not configured.",
		map[string]string{"reason": "audit_not_streaming"})
}
```

**Route 56, `POST /v1/projects/{p}/leases/{ls}/events`**: Fleet, `leases.events`, the holder's
owner. **Fleet is authoritative for leases** (§3.3 row "lease"); the Hub checks shape, owner, that
`seq` is the lease's last `seq` + 1 (409 `conflict` otherwise) and that nothing follows a terminal
state (`yielded`, `released`, `expired`, `admin-released`), and nothing else. A Hub that judged
Fleet's transitions would make two authorities. The first event for an `ls-…` creates its row.

```jsonc
{ "seq": 4, "holderAgentId": "ag-…", "mode": "intent" | "hold" | "bulk",
  "globs": [ "src/export/**" ], "trackedFiles": 812,       // ≤ 20, ValidGlob each
  "state": "requested" | "granted" | "contested" | "yielded" | "released" | "expired" | "admin-released",
  "expiresAt": "…", "at": "…" }
// 201
{ "leaseId": "ls-…", "seq": 4, "state": "granted", "at": "…" }
```

**The one exception to "the holder's owner"**: an `admin-released` event is written under the
delegation of an `admin` of the project, and records that admin (`task-01.md:355-356`, §3.4 last
row). A member's delegation writing `admin-released` on another's lease is 403 `forbidden`,
`required: "admin"`.

**Route 57, `POST /v1/projects/{p}/conflicts`**: Fleet, `conflicts.create`. A conflict has two
owners; task 01's rule 5 (`task-01.md:135-140`) requires the **first-named** agent, `agentA`, to be
the delegation owner's. The forecaster therefore writes under `agentA`'s owner's delegation.

```jsonc
{ "id": "cf-…", "agentA": "ag-…", "agentB": "ag-…", "tier": "P" | "H" | "M" | "S",
  "paths": [ { "path": "src/export/route.ts", "lines": [ [ 10, 24 ] ] } ],   // ≤ 50 paths, ≤ 20 ranges, 1 ≤ start ≤ end
  "basis": { "a": "<40 or 64 hex>", "b": "<40 or 64 hex>" } }
// 201: the request plus "projectId", "state": "raised", "resolution": "", "threadId": "",
//      "revision": 1, "createdAt", "updatedAt"
```

The two agents differ and both are in `{p}`. **Route 58, `PATCH …/conflicts/{cf}`**:
`conflicts.update`, under the delegation of the owner of **either** agent; body `{ "revision",
"state", "resolution"?, "threadId"? }`, `state` one of `negotiating`·`accepted`·`resolved`·
`escalated`·`cleared`, `resolution` one of `split`·`sequence`·`yield`·`build-on`·`proceed-both`,
required with `resolved` and `accepted` and refused otherwise (400 `bad_request`). `resolved` and
`cleared` are terminal (409 `conflict`). **Route 59, `GET …/conflicts`**: as route 52,
`conflicts.read`; `?open=true` omits `resolved` and `cleared`.

**The session-id rule.** No request type above has a `sessionId`, `ownerUserId`, `actor`, `via` or
`delegationId` field, so strict decoding answers a body carrying one with 400 `bad_request`,
`details.field` naming it (`task-01.md:130-134`). Beyond that, every id field is checked with
`hubapi.ValidID` for its own prefix, so a phase 2 session id pasted into `laneRef`, `taskId` or
`parentAgentId` is 400 too.

#### Events

One metadata-only event per changed record, in the write's transaction (contract §8). `data` holds
**no** text: no title, no why-line, no glob list; only ids, enums and counts.

The kinds and `data` are task 01's contract §8 rows (`task-01.md:541-574`), copied, not chosen
here. **In the feed** is task 01's three-valued column: `yes` (the member feed and Fleet's),
`fleet` (Fleet's feed only, the audit row's `fleet_feed` flag of task 04), `no`.

| Kind | `projectId` | `subject` | `data` | In the feed |
|---|---|---|---|---|
| `delegation.created` | ✓ | delegation id | `{"class","expiresAt"}` | fleet |
| `delegation.renewed` | ✓ | delegation id | `{"expiresAt","lapsed"}` | fleet |
| `delegation.revoked` | ✓ | delegation id | `{"reason"}` | fleet |
| `task.created` | ✓ | task id | `{"state","orchestratorAgentId"}` | yes |
| `task.updated` | ✓ | task id | `{"state","revision"}` | yes |
| `agent.started` | ✓ | agent id | `{"role","parentAgentId","harnessId","harnessVersion","policyVersion"}` | yes |
| `agent.ended` | ✓ | agent id | `{"exitClass"}` | yes |
| `lease.state_changed` | ✓ | lease id | `{"seq","state","mode","holderAgentId"}` | yes |
| `conflict.raised` | ✓ | conflict id | `{"tier","agentA","agentB"}` | yes |
| `conflict.updated` | ✓ | conflict id | `{"state","resolution"}` | yes |
| `project.mode_set` | ✓ | project id | `{"from","to"}` | yes |

`project.mode_set` is a desktop write (`v: 1`, the admin as actor). It is the first event of a new
team project on Fleet's per-org feed (route 78 serves team projects only), which is how Fleet
learns the project exists (`05-design.md` §10.1 step 2).

Every event of a delegated write is the audit-event `v: 2` of task 01's §3.6 (`task-01.md:189-193`):
`actorUserId = owner`, `via = "fleet"`, `delegationId`, set by task 05's write path, not by these
handlers. An `admin-released` lease event is `lease.state_changed` like any other; its `actorUserId`
is the admin, which is how an audit query finds every one. Task 08 makes these kinds readable on
Fleet's per-org feed.

#### The guards — complete source

```go
// control-plane/internal/hub/server/teamguard.go
package server

import (
	"context"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

// RequireDesktop refuses a Fleet call. Delegation routes are the user's own act (§2.4 item 2).
func RequireDesktop(p Principal) error {
	if p.Via != "" {
		return Forbidden("desktop")
	}
	return nil
}

// RequireFleet refuses a desktop call to a Fleet-only route (task-01.md:581-583), and a
// Fleet call whose delegation is for another project than the path names.
func RequireFleet(p Principal, projectID string) error {
	if p.Via != hubapi.ViaFleet {
		return Forbidden("fleet")
	}
	if p.Delegation.ProjectID != projectID {
		// Not a 404 of the project: the caller is Fleet, and naming its own
		// delegation's mismatch tells it nothing it did not send.
		return Forbidden("delegation.project")
	}
	return nil
}

// OwnedAgent is task 01's rule 5 (task-01.md:135-140): the agent the record names
// first, and every agent it says acts, belongs to the principal. Whatever the
// principal's role: an admin is refused on another member's agent like anyone.
func OwnedAgent(ctx context.Context, tx *Tx, agentID string) error {
	ok, err := store.AgentOwnedBy(ctx, tx, agentID, tx.P.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return Refuse(403, hubapi.CodeAgentNotOwner, "That agent is not yours.",
			map[string]string{"agentId": agentID})
	}
	return nil
}

// EitherAgent is OwnedAgent for a record with two owners (a conflict update, a
// thread update): one of the two must be the principal's.
func EitherAgent(ctx context.Context, tx *Tx, a, b string) error {
	if err := OwnedAgent(ctx, tx, a); err == nil {
		return nil
	} else if !isAgentNotOwner(err) {
		return err
	}
	return OwnedAgent(ctx, tx, b)
}

func isAgentNotOwner(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Code == hubapi.CodeAgentNotOwner
}
```

The imports are `context`, `errors`, `hubapi` and `store`. **What the guards do not re-check, on
purpose:** a delegation's expiry, revocation and route list, and its class. Task 05's pipeline
checked the first three before the handler ran, and task 01's class route sets
(`task-01.md:274-283`) mean a `worker` delegation can never list `tasks.create` in the first place.
Checking any of them twice gives two answers to one question the day the two copies drift.
`Forbidden` is Hub task 14's; it records `auth.forbidden` in its own short transaction, so a refused
Fleet write leaves an audit trace although its transaction rolls back.

#### The handler shape — one route in full

Every other handler has the same five lines before its store call; only the checks after them
differ, and those are the rules in the route text above.

```go
// control-plane/internal/hub/server/teamtasks.go (excerpt: the create handler)
func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("p")
	s.Write(w, r, func(tx *Tx, body []byte) (WriteResult, error) {
		if err := RequireFleet(tx.P, projectID); err != nil {
			return WriteResult{}, err
		}
		if _, _, ok, err := ProjectRole(r.Context(), tx, projectID); err != nil {
			return WriteResult{}, err
		} else if !ok {
			return WriteResult{}, Refuse(404, hubapi.CodeProjectNotFound, "No such project.", nil)
		}
		var req hubapi.CreateTaskRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		if err := validateTask(req); err != nil { // teamtext.Bound, Clean, ValidGlob; ≤ 20 globs
			return WriteResult{}, err
		}
		if err := OwnedAgent(r.Context(), tx, req.OrchestratorAgentID); err != nil {
			return WriteResult{}, err
		}
		author := teamtext.Author("agent")
		if req.TitleSource != "" && store.HumanMessageEquals(r.Context(), tx, req.TitleSource, tx.P.UserID, req.Title) {
			author = "human"
		}
		t, err := store.CreateTask(r.Context(), tx, projectID, tx.P.UserID, author, req)
		if err != nil { // a reused id with another body: store returns 409 conflict
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: "task.created", ProjectID: projectID, Subject: t.ID,
			Data: map[string]any{"state": string(t.State), "orchestratorAgentId": t.OrchestratorAgentID}})
		return WriteResult{Status: 201, Body: renderTask(tx.P, t)}, nil
	})
}
```

The order is fixed and the tests pin it: **caller class first** (so a desktop token learns nothing
about the project), then the project, then the body, then ownership. A body that fails validation
from another owner's delegation therefore answers 400, not 403; that is deliberate, because shape is
not a secret and the 403 must mean exactly one thing.

- [ ] **Step 1: Write the failing tests: the guards, as unit tests**

`teamguard_test.go` is a plain `package server` unit test; it needs no database, so it runs in the
default `go test` and fails first, fast.

```go
// control-plane/internal/hub/server/teamguard_test.go
package server

import (
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

func fleetP(user, project string, class hubapi.DelegationClass, routes ...hubapi.RouteName) Principal {
	return Principal{UserID: user, Via: hubapi.ViaFleet, DelegationID: "dl-TEST",
		Delegation: DelegationView{ProjectID: project, Class: class, Routes: routes}} // u-… ids: task 01's §7.13 sample
}

func required(t *testing.T, err error) string {
	t.Helper()
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 403 {
		t.Fatalf("want a 403 APIError, got %v", err)
	}
	return ae.Details["required"]
}

// Mutant killed: RequireFleet checks the project but not Via, so a desktop token
// (whose Delegation is the zero value) slips through on a zero project comparison.
func TestRequireFleetRefusesADesktopCaller(t *testing.T) {
	if got := required(t, RequireFleet(Principal{UserID: "u-L"}, "")); got != "fleet" {
		t.Fatalf("required = %q", got)
	}
}

// Mutant killed: the project comparison is dropped, so Linus's delegation for P writes into Q.
func TestRequireFleetRefusesAnotherProjectsDelegation(t *testing.T) {
	p := fleetP("u-L", "pj-P", hubapi.DelegationClassOrchestrator, hubapi.RouteTasksCreate)
	if got := required(t, RequireFleet(p, "pj-Q")); got != "delegation.project" {
		t.Fatalf("required = %q", got)
	}
	if err := RequireFleet(p, "pj-P"); err != nil {
		t.Fatalf("its own project was refused: %v", err)
	}
}

// Mutant killed: RequireDesktop tests Via == "fleet" rather than Via != "", so a future
// second delegated caller would be treated as a desktop.
func TestRequireDesktopRefusesAnyDelegatedCaller(t *testing.T) {
	if got := required(t, RequireDesktop(Principal{UserID: "u-L", Via: "other"})); got != "desktop" {
		t.Fatalf("required = %q", got)
	}
	if err := RequireDesktop(Principal{UserID: "u-L"}); err != nil {
		t.Fatalf("a desktop caller was refused: %v", err)
	}
}
```

`OwnedAgent` and `EitherAgent` read the database, so their tests are integration tests (Step 2:
`TestTaskCreateRefusesAnotherOwnersAgent`, `TestConflictUpdateByEitherParty`).

`DelegationView` is task 05's name for the loaded row on the principal, with `Class
hubapi.DelegationClass` and `Routes []hubapi.RouteName` (`task-05.md:303-311`). `Forbidden` records an
`auth.forbidden` event through the server's store; in a unit test with no store it must no-op on a
nil store rather than panic. If Hub task 14's `Forbidden` does not already tolerate that, this test
file constructs the errors through `Refuse(403, hubapi.CodeForbidden, …)` in a local helper instead.

- [ ] **Step 2: Write the failing integration tests: one world, every route, five callers**

These run on Hub task 06's integration harness (`//go:build integration`, a real Postgres) and use
Hub task 11's `dev-tenant.json` people, so the columns match the matrix harness of Hub task 16b
(`task-16b.md:26-36`): **Linus** (member of T), **Grace** (admin of T), **Vera** (viewer of T),
plus **Fleet** through task 05's client-credentials grant, with and without a delegation header.

```go
// control-plane/internal/hub/server/team1_world_test.go
//go:build integration

package server_test

// teamWorld is task 06's piece of the world: project P on team T, one delegation per
// member and class, and one orchestrator agent each. Built fresh per test. The harness
// runs with PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true AND PANACEA_APP_HUB_AUDIT_SINK=fake
// (re-review I-4: task 45's TeamModeAllowed opens on the flag only beside sink "fake";
// before task 45 the sink variable is unread, which is harmless), and P becomes a team
// project by Grace calling route 87, never by SQL, so every test below runs the real writer.
// SoloID is a second project on T that stays solo (route 33's and route 46's refusals).
type teamWorld struct {
	h                    *hubHarness // Hub task 06's harness: server, pool, clock
	ProjectID, SoloID    string
	DLLinusOrch, DLLinusWorker, DLGraceOrch string
	AGLinusOrch, AGGraceOrch             string
}

// linus, grace, vera call as the desktop (Hub task 11's dev tokens).
// fleet(dl) calls as Fleet's workload identity naming dl; fleet("") names none.
func (w *teamWorld) linus() *caller
func (w *teamWorld) grace() *caller
func (w *teamWorld) vera() *caller
func (w *teamWorld) fleet(delegationID string) *caller
```

The tests, one per rule. Each comment names the mutant that makes it fail; a test with no mutant
is not written.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestDelegationCreateIsTheUsersOwnCall` | Linus's desktop → 201, `ownerUserId` = Linus, `expiresAt − createdAt` = the TTL (24 h default; 1 h when the harness sets `PANACEA_APP_HUB_DELEGATION_TTL=1h`) | the TTL variable ignored; owner read from a body field |
| `TestDelegationCreateRefusesAViewer` | Vera → 403 `forbidden`, `required: "member"` | the role floor is `viewer` |
| `TestDelegationCreateRefusesFleet` | `fleet(DLLinusOrch)` on route 46 → 403 `forbidden` (task 05: no route name) | a route name given to route 46, so Fleet mints its own authority |
| `TestDelegationRoutesAreTheClassSet` | a `worker` delegation listing `tasks.create` → 422 `bad_request`, `details.route = "tasks.create"`; an unknown name → 422; a duplicate → 422; 26 names → 422 | the class sets merged into one list, so a worker carries the cross-user channel |
| `TestDelegationRenewIsOwnerOnlyAndHidesExistence` | Grace renews Linus's → 404 `delegation_not_found`; Linus → 200, `expiresAt` moved, `renewedAt` set | the owner check answers 403, leaking the id |
| `TestDelegationRenewRevivesALapsedOneButNotARevokedOne` | clock +25 h, renew → 200 and `data.lapsed = true`; `DELETE` then renew → 409 `conflict` | the lapsed check shares the revoked branch |
| `TestDelegationDeleteByOwnerOrAdmin` | Vera → 403; Linus on Grace's → 403; Grace (admin) on Linus's → 204, `revokedReason = "admin"`; a second `DELETE` → 204, one `delegation.revoked` in the audit | admin revoke refused, or the second delete emits again |
| `TestDelegationGetIsOwnerOrFleetUnderIt` | Linus → 200; `fleet(DLLinusOrch)` on its own id → 200; `fleet(DLLinusOrch)` on `DLGraceOrch` → 404 | the Fleet read is keyed on the org, not the delegation |
| `TestTaskCreateNeedsFleet` | Linus desktop → 403 `required: "fleet"`; `fleet("")` → 401 `delegation_required` (task 05); `fleet(DLLinusOrch)` → 201 | `RequireFleet` removed |
| `TestTaskCreateRefusesAnotherOwnersAgent` | `fleet(DLLinusOrch)` with `AGGraceOrch` → 403 `agent_not_owner` | the agent check compares the agent's project, not its owner |
| `TestTaskPatchIsOwnerOnly` | `fleet(DLGraceOrch)` patching Linus's task → 403 `agent_not_owner`, although Grace is team admin (task 01's `Fx` column) | admin exempted from rule 5 |
| `TestTaskTransitions` | every pair of the 5×5 state table: allowed → 200, others → 409 `conflict` with `from`/`to` | one terminal state left open (e.g. `done → active`) |
| `TestTaskPatchNeedsTheCurrentRevision` | `revision` 1 after a patch → 409 `revision_conflict` | the `WHERE revision = $n` guard dropped |
| `TestTaskTitleAuthor` | no `titleSource` → `author: agent`; the owner's `hm-…` with an equal trimmed body → `human`; Grace's `hm-…` or an unequal body → `agent` | the source's owner not compared |
| `TestTaskTextIsBoundedAndCleaned` | title of 141 runes → 422 `text_refused`, `details.field = "title"`; title with `U+202E` → 422; why of 201 → 422 | the bound counts bytes, not runes; `Clean` skipped on the why |
| `TestTaskScopeGlobs` | 21 globs → 422 `breadth_exceeded`; `../x`, `/abs`, `a\b` → 422 `glob_refused`, `details.index` | the count cap applied after dedupe only |
| `TestAgentStartBindsParent` | a worker whose parent is Grace's Orchestrator → 403 `agent_not_owner`; an Orchestrator with a parent → 400; a reused `ag-…` with another body → 409 `conflict`; the same body and key → replay | the parent's owner not checked |
| `TestAgentStartRecordsThePolicyVersion` | the `agent.started` event's `data.policyVersion` is exactly as sent | the version dropped from the event (task 14 then has no audit fact) |
| `TestAgentEndTwiceIs409` | end, end → 200, 409 `conflict` | a second end overwrites `exitClass` |
| `TestSessionLinkRefusedOnATeamProject` | route 33 on the team project → 409 `conflict`, `details.use = "agents"`; on `SoloID` → 201 as before | route 33 left open to team projects |
| `TestDelegationCreateNeedsATeamProject` (b01) | Linus, route 46 on `SoloID` → 409 `conflict`, `details.reason = "not_team_project"`, no row, no event; on P → 201 | the mode read dropped from route 46, so Fleet writes team records on a project that never passed the audit gate |
| `TestProjectModeIsTheAdminsCall` (b01) | route 87 on `SoloID`: Vera → 403 `required: "admin"`; Linus → 403 `required: "admin"`; a user of another team → 404 `project_not_found`; `fleet(DLGraceOrch)` → 403 `required: "route"`; Grace → 200, `mode: "team"` | the role floor is `member`; the route given a route name |
| `TestProjectModeIsOneWay` (b01) | Grace, `{"mode":"solo"}` on P → 400 `bad_request`, `details.field = "mode"` (not 500 from the trigger), P still `team`; `{"mode":"team"}` on P again → 200, exactly one `project.mode_set` for P in the audit | the body's value written as sent; the idempotent replay emitting a second event |
| `TestProjectModeWaitsForTheAuditGate` (b01) | a second server on the same pool with the flag `false`: Grace, route 87 on `SoloID` → 503 `unavailable`, `details.reason = "audit_not_streaming"`; `SoloID` still `solo`; no `project.mode_set` | the gate called after the `UPDATE`, or not at all |
| `TestProjectModeSetIsOneEventOnTheFeed` (b01) | the Grace call's event is `project.mode_set`, `v: 1`, `actorUserId` Grace, `data` exactly `{"from":"solo","to":"team"}`, a feed row (route 78 itself is task 08's, which asserts Fleet reads it) | the event written with no feed row, so Fleet never learns the project |
| `TestProjectReadsCarryMode` (b01 round 2, in Hub task 17's `projects_integration_test.go`) | Grace, route 14 with remote `github.com/lcfrai/modeprobe` → 201, `mode: "solo"` (never `""`); route 87 on it → 200; then route 15 → `mode: "team"`, route 13 `?repo=github.com/lcfrai/modeprobe` → one item, `mode: "team"`, route 13 `?teamId=` → that item `mode: "team"`, route 16 rename → 200, `mode: "team"`; each asserted on the raw JSON, not a decoded struct | the project-row reader left without `mode`, so the real Hub answers `"mode":""` and `IsTeam` is never true (re-review I-1) |
| `TestLeaseEventsAreGaplessAndStopAtTerminal` | seq 1, 2 → 201; seq 2 again → 409; seq 4 → 409; after `released`, seq n+1 → 409 | the gap check is `>` instead of `== last + 1` |
| `TestLeaseEventHolderMustBeTheOwners` | holder `AGGraceOrch` under `DLLinusOrch` → 403 `agent_not_owner` | the holder check skipped for `expired` |
| `TestAdminReleaseNeedsAnAdmin` | `admin-released` of Grace's lease under Linus's delegation → 403 `required: "admin"`; of Linus's lease under Grace's → 201, `actorUserId` Grace | admin release allowed to members |
| `TestConflictCreateNeedsAgentAsOwner` | `agentA` Linus's under `DLLinusOrch` → 201; `agentA` Grace's under `DLLinusOrch` → 403 `agent_not_owner`; `agentA == agentB` → 400 | rule 5 applied to `agentB` instead |
| `TestConflictUpdateByEitherParty` | `fleet(DLGraceOrch)` patching a conflict whose `agentB` is Grace's → 200; a third member's delegation → 403 `agent_not_owner` | `EitherAgent` checks `a` twice |
| `TestConflictResolutionOnlyWithResolvedOrAccepted` | `resolved` with none → 400; `negotiating` with one → 400; `cleared` then anything → 409 | resolution accepted on any state |
| `TestNoTeamBodyCarriesAnActorOrASessionID` | each of the ten write bodies plus one of `sessionId`, `ownerUserId`, `actor`, `via`, `delegationId` → 400 `bad_request` naming it; a phase 2 session id in `laneRef`, `taskId`, `parentAgentId` → 400 | unknown fields allowed; `ValidID` skipped on optional ids |
| `TestTaskTitleIsServedToFleetOrchestratorOnly` | route 52: Linus's desktop, Vera and `fleet(DLLinusWorker)` get no `title` or `reason.why`; `fleet(DLGraceOrch)` gets both | the redaction keyed on the owner, not on Fleet·O |
| `TestEveryTeamWriteIsOneEventWithNoText` | one event per write; no event's canonical `data` contains the title or the why | an event per field changed; the title copied into `data` |
| `TestFleetEventsAreV2` | every event of a Fleet call is `v: 2` with `actorUserId` = owner, `via = fleet`, `delegationId` | the actor recorded as Fleet's service principal |
| `TestEveryFleetRouteIsNamed` (`delegations_integration_test.go`) | for each of the eleven named routes, Fleet under a delegation of the right class listing exactly that route → no 403 with `required: "route"`; for routes 46, 48 and 49, Fleet → 403 `required: "route"` | one route registered bare (Fleet shut out of a route it holds), or a desktop route given a name |

One unit test, in `cmd/hub/config_test.go` (b01): `TestTeamModeUnstreamedIsDevOnly`:
`PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true` with `PANACEA_APP_HUB_ENV=prod` → `loadConfig`
error naming both variables; with `local` and with `dev` → `cfg.TeamModeUnstreamed == true`; unset
→ `false`; `yes` → an error (a boolean only). Mutant: the env check dropped, so a production Hub
turns projects to team with its audit unstreamed.

Two of them in full; the rest follow the same shape and live beside their route's file.

```go
// control-plane/internal/hub/server/teamtasks_integration_test.go (excerpt)
//go:build integration

package server_test

// Mutant killed: the admin exemption. Grace is T's admin and Fleet holds a live
// delegation of hers; she still may not change Linus's task (§3.3 rule 5).
func TestTaskPatchIsOwnerOnly(t *testing.T) {
	w := newTeamWorld(t)
	tkID := ids.Wide("tk")
	w.fleet(w.DLLinusOrch).mustPost(t, "/v1/projects/"+w.ProjectID+"/tasks", map[string]any{
		"id": tkID, "orchestratorAgentId": w.AGLinusOrch,
		"title":  "Split the export",
		"reason": map[string]any{"briefRef": w.BriefKey + "#export", "why": "Brief section export asks for it"},
		"scope":  []string{"src/export/**"}, "trackedFiles": 812, "state": "planned",
	}, 201)
	res := w.fleet(w.DLGraceOrch).patch(t, "/v1/projects/"+w.ProjectID+"/tasks/"+tkID,
		map[string]any{"revision": 1, "state": "abandoned"})
	res.wantProblem(t, 403, "agent_not_owner", map[string]string{"agentId": w.AGLinusOrch})
	if got := w.h.taskState(t, tkID); got != "planned" {
		t.Fatalf("the refused patch changed the task: state %q", got)
	}
}

// Mutant killed: strict decoding applied only on the routes whose type has an id field.
func TestNoTeamBodyCarriesAnActorOrASessionID(t *testing.T) {
	w := newTeamWorld(t)
	extras := map[string]any{"sessionId": "se-01J9ZK3Y6N4Q0V9T8W7X6Y5Z4A", "ownerUserId": "u-X",
		"actor": "u-X", "via": "fleet", "delegationId": w.DLGraceOrch}
	for _, c := range w.everyTeamWrite() { // ten (caller, method, path, valid body) cases
		for field, v := range extras {
			body := maps.Clone(c.body)
			body[field] = v
			res := c.caller.do(t, c.method, c.path, body)
			res.wantProblem(t, 400, "bad_request", map[string]string{"field": field})
		}
	}
}
```

`everyTeamWrite` lives in `team1_world_test.go` and is the list task 43 turns into Z5 matrix rows,
so the two cannot drift: a route added here and not there fails task 43's count check.

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd control-plane && go test ./internal/hub/server/ -run 'TestRequire' -count=1
```

Expected: a build failure naming the guards, one line per use site, for example
`./teamguard_test.go:13:9: undefined: DelegationView` if task 05 has not merged, or
`undefined: RequireFleet` if it has. Then the integration half:

```bash
cd control-plane && go test -tags integration ./internal/hub/server/ -run 'TestDelegation|TestTask|TestAgent|TestLease|TestAdmin|TestConflict|TestNoTeam|TestEveryTeam|TestFleetEvents|TestProjectMode|TestProjectReadsCarryMode' -count=1
cd control-plane && go test ./cmd/hub/ -run 'TestTeamModeUnstreamedIsDevOnly' -count=1
```

Expected: `FAIL` with every route answering 404 `route_not_found`, because nothing is registered;
the config test fails to compile on `cfg.TeamModeUnstreamed`.

- [ ] **Step 4: Write the guards, the store functions and the five handler files**

In this order, running the unit tests after the first: `teamguard.go` (above, complete); the store
functions, one file per record, each a single parameterised statement under `tx` with
`RETURNING` the full row; then the handlers. The store functions set nothing the handler did not
pass, and every `UPDATE` carries `WHERE id = $1 AND revision = $2`, so an `ifRevision` race is a
zero-row update the handler turns into 409 `revision_conflict`.

Then Hub task 17's `store/projects.go` (b01 round 2, re-review I-1): add `mode` to the one
project-row column list and scan that routes 13 to 16 share, so a project read answers
`"mode":"solo"` or `"mode":"team"`, and point route 87's reply at the same reader. Route 14's
insert names no `mode` (the column's default, task 04, makes it `solo`) but its `RETURNING` does.
`TestProjectReadsCarryMode` fails before this edit and passes after it.

The route registration, in full:

```go
// control-plane/internal/hub/server/delegations.go (excerpt)
func (s *Server) registerTeamPart1() {
	// Desktop only (routes 46, 48, 49): no route name, so Fleet is 403 here by
	// construction (task-05.md:49-53), before RequireDesktop runs.
	s.mux.HandleFunc("POST /v1/delegations", s.createDelegation)
	s.mux.HandleFunc("POST /v1/delegations/{dl}/renew", s.renewDelegation)
	s.mux.HandleFunc("DELETE /v1/delegations/{dl}", s.revokeDelegation)
	// Route 87 (fix batch b01): desktop admin only, bare for the same reason.
	s.mux.HandleFunc("POST /v1/projects/{p}/mode", s.setProjectMode)
	// Route 88 (fix batch b02): Fleet only, and bare on purpose — it is the one
	// Fleet route with no route name, because it is called before a delegation is
	// known (§5.10 rule 2, the second and last exemption). RequireFleetNoDelegation
	// is the whole gate; a desktop token never reaches the handler.
	s.mux.HandleFunc("GET /v1/principals", s.getPrincipal)

	// Fleet-reachable (routes 47, 50-59): each under its task 03 name.
	d := func(pattern string, name hubapi.RouteName, h http.HandlerFunc) {
		s.mux.Handle(pattern, Delegable(name, h))
	}
	d("GET /v1/delegations/{dl}", hubapi.RouteDelegationsRead, s.getDelegation)
	d("POST /v1/projects/{p}/tasks", hubapi.RouteTasksCreate, s.createTask)
	d("PATCH /v1/projects/{p}/tasks/{tk}", hubapi.RouteTasksUpdate, s.patchTask)
	d("POST /v1/projects/{p}/agents", hubapi.RouteAgentsStart, s.startAgent)
	d("POST /v1/projects/{p}/agents/{ag}/end", hubapi.RouteAgentsEnd, s.endAgent)
	d("POST /v1/projects/{p}/leases/{ls}/events", hubapi.RouteLeasesEvents, s.leaseEvent)
	d("POST /v1/projects/{p}/conflicts", hubapi.RouteConflictsCreate, s.createConflict)
	d("PATCH /v1/projects/{p}/conflicts/{cf}", hubapi.RouteConflictsUpdate, s.patchConflict)
	d("GET /v1/projects/{p}/tasks", hubapi.RouteTasksRead, s.listTasks)
	d("GET /v1/projects/{p}/agents", hubapi.RouteAgentsRead, s.listAgents)
	d("GET /v1/projects/{p}/conflicts", hubapi.RouteConflictsRead, s.listConflicts)
}
```

Routes 46 to 59: ten writes and four reads; three bare, eleven named. Route 87 is one more bare
write, outside `everyTeamWrite` (it writes no team record; its own tests cover it). Route 88 is a
bare **read**, also outside `everyTeamWrite`.

**Route 88, the principal read (fix batch b02, S02-C2).** `getPrincipal` runs under `s.Read`. It
refuses anything but Fleet (`c.AppOnly && c.AppRole == hubapi.AppRoleFleet`) with 403 `forbidden`,
`details.required = "fleet"`, and a Fleet token that carried `X-Panacea-Delegation` with 400
`bad_request` — task 05's `delegationHeader` cannot do the second for it, because on an exempt
path that function returns nil, so the handler makes the check. It requires all three of `tid`,
`oid` and `projectId` (400 `bad_request`, `details.field`), runs `hubapi.ValidID` on `projectId`
before any query, and then makes **one** call, `store.PrincipalFor`. That statement resolves the
project first and answers **the project's** `orgId`, its team role for that user, and the three
org flags **as held in the project's org** — never the caller's, which is the whole of review
S02-I6's rule and is why it is one statement and not three. An unprovisioned `(tid, oid)` is 404
`principal_not_found`; an unknown project, another org's project, or one the user may not see is
404 `route_not_found`, the same answer, so the route cannot be used to probe. The read emits §8's
`principal.read`, audit only, subject the `u-…`, `data = {"projectId"}`. Mutants killed by Step 2:
`orgId` taken from the token's org rather than the project's (`TestPrincipalOrgIsTheProjects`);
the flags read in the caller's org (`TestAnAuditorOfAnotherOrgHasNoFlagsHere`); `principal_not_found`
answered for an unknown project, which tells Fleet a project exists.

**Route 47's third caller, the broker (fix batch b02, S02-C5).** `getDelegation` gains one branch
before its owner check: when `c.AppOnly && c.AppRole == hubapi.AppRoleDelegationReader`, it serves **any**
delegation of the org enrolled for the token's `tid`, `revokedAt` and `expiresAt` in any state,
and the answer carries `ownerTid` and `ownerOid` (§7.13). Every other path, and every other
route, answers that token 403 `forbidden`, `required: "fleet"` — **the gate below, in this task's
`auth.go` deltas, is what makes that true**; this handler is its only consumer and never its
enforcement. The two extra fields are **omitted** for every other caller (`omitempty` on both, set
only in the broker branch), so a member's or Fleet's answer is unchanged byte for byte. The read
emits `delegation.read_by_broker`, audit only, with `actorUserId = ""` (the broker is not a
principal). Mutants killed: the two fields returned to Fleet
(`TestFleetSeesNoOwnerTidOnRoute47`); the broker admitted on route 52
(`TestBrokerReachesOnlyRoute47`); a revoked delegation hidden from the broker
(`TestBrokerReadsARevokedDelegation`), which would leave task 20's cleanup unable to find the pods
it must delete. `s.mux` is the name Hub
task 13's server uses (**UNVERIFIED until merge**; if the field differs, rename). A desktop call
to a named route is unaffected: `Delegable` only puts the name in the context, and task 05 reads it
only for an app-only token. Mutant killed by `TestEveryFleetRouteIsNamed` (Step 2): one `d(…)`
line written as a bare `HandleFunc`, which answers Fleet 403 `required: "route"` on a route its
delegation lists.

**The gate that admits the broker, and admits it nowhere else (b02 round 1, C1 and C2).** Task 05
wrote `delegationHeader` when `AppOnly` had one meaning; this task gives it a second, so the two
named hunks below land in `auth.go` with it. Without hunk (3) the broker is refused **401
`delegation_required`** at step 3b and never reaches route 47 at all; without its `AppRoleFleet`
narrowing the broker reads route 78, the org-wide feed of every team project.

```go
// (3) delegationHeader: the two app-only identities, told apart by role.
//     These four cases REPLACE task 05's `case isFeedPath(r) …` pair, in its
//     place — straight after `case !c.AppOnly:` and straight BEFORE task 05's
//     `case len(vals) == 0:`. Every case from `len(vals) == 0` down is KEPT
//     unedited (`task-05.md:267-276`): it is Fleet's 401 `delegation_required`,
//     the 400 on two headers and the 401 `delegation_revoked` on a malformed id,
//     and the two role arms above are what make it Fleet's alone. Deleting it
//     instead would drop Fleet's 401 and fall a headerless request through to
//     `vals[0]`, which panics on an empty slice.
case c.AppRole == hubapi.AppRoleDelegationReader:
	// §5.10 rule 10. Checked here, before the mux, so no handler can forget it.
	if len(vals) > 0 {
		return Refuse(400, hubapi.CodeBadRequest, "The broker takes no delegation.",
			map[string]string{"field": hubapi.HeaderDelegation})
	}
	if !isBrokerPath(r) {
		return Refuse(403, hubapi.CodeForbidden, "Only Fleet may call this.",
			map[string]string{"required": "fleet"})
	}
	c.BrokerRead = true
	return nil
case c.AppRole != hubapi.AppRoleFleet:
	// A third app-only role, configured and ungated, is refused rather than
	// admitted as Fleet. Task 66's Hub.Gateway lands as its own case (rule 9).
	return Refuse(403, hubapi.CodeForbidden, "Only Fleet may call this.",
		map[string]string{"required": "fleet"})
case isExemptPath(r) && len(vals) > 0:
	// Task 05's 400 (`task-05.md:261-263`), widened from isFeedPath to
	// isExemptPath, its sentence generalised because route 88 now shares it,
	// and reached only by AppRoleFleet. The code and `details.field` are
	// task 05's, unchanged, and what task 08 asserts is the code and the field,
	// never the sentence (`task-08.md:298`, route 78's wire; `:500`, row
	// team/principal-needs-no-delegation, for route 88). Written out because an
	// empty case body is not an elision in Go: it would fall out of the switch
	// to `c.DelegationID = vals[0]` and serve the feed under a delegation.
	return Refuse(400, hubapi.CodeBadRequest, "This route takes no delegation.",
		map[string]string{"field": hubapi.HeaderDelegation})
case isExemptPath(r):
	c.FleetFeed = isFeedPath(r) // route 88 sets no feed principal; route 78 does
	return nil

// isBrokerPath is rule 10's one allowed pattern: GET, and the segments are
// exactly "v1", "delegations", one non-empty id. Never a prefix match, and
// never `/v1/delegations` itself (route 51's list is Fleet's).
func isBrokerPath(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	s := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	return len(s) == 3 && s[0] == "v1" && s[1] == "delegations" && s[2] != ""
}

// (4) resolveCaller: the broker is no principal. Its tenancy is its own tid's
//     org (rule 6's ordinary case), so a cross-org read is 404 route_not_found
//     on the same statement that serves a member's.
case c.BrokerRead:
	org, err := store.FleetOrg(ctx, tx, c.TenantID)
	if err != nil {
		return Principal{}, false, err // ErrNotEnrolled → 403
	}
	return Principal{OrgID: org, TenantID: c.TenantID}, false, nil
```

`Claims.BrokerRead` is the fifth `Claims` field (beside task 05's three and `AppRole`); it exists
so `resolveCaller`, which has no `*http.Request`, can tell the caller apart without re-parsing a
path. `userKey` gains `case c.BrokerRead: return "broker/" + c.TenantID`, so the broker's polling
cannot drain a member's bucket. Two more tests in Step 2: `TestBrokerWithAHeaderIs400` and
`TestAnUngatedAppRoleIsRefused` (a configured third role reaching route 52 → 403, not 200).

**The three reads** (routes 52, 55, 59) are what task 11's `rebuild.go` reloads after a Fleet
restart. They are the Hub's usual page shape (`hubapi.Page[T]`, `?limit=` 1–200 and `?after=`,
the built `hubapi.PageRequest`, `hub` worktree `hubapi/types.go:201-205`), run
under `s.Read`, and open to every project role including `viewer` (§3.4 row 1: the team strip);
an org *auditor* who is not a member gets 404 (`task-01.md:578`). Filters: tasks `?state=`
(repeatable), agents `?live=true`, conflicts `?open=true`. The title redaction of route 52 lives in
one function, `renderTask(p Principal, t hubapi.Task) hubapi.Task`, which every task answer passes
through, route 50's and 51's included. Mutant killed by `TestTaskTitleIsServedToFleetOrchestratorOnly`:
the redaction applied to the list but not to a create or patch answer, which Fleet·W never sees but
a future caller would.

**Codes.** This task adds none. It uses task 01's (`task-01.md:198-210`) through task 03, plus
404s for the new records. Task 03 must therefore carry `CodeConflict = "conflict"` (409), which
task 01 uses throughout and the Hub's `codes.go:20-52` as built does not have, and
`CodeTaskNotFound`, `CodeAgentNotFound`, `CodeConflictNotFound` (404), which task 01 does not name
(see "Gaps").

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane && go test ./internal/hub/server/ -run 'TestRequire' -count=1 -v
cd control-plane && go test -tags integration ./internal/hub/server/ -count=1
```

Expected: the three guard tests `--- PASS`, and the integration run ends
`ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/server	<n>s` with no `FAIL`
line. Three failures are worth predicting. `TestTaskTransitions` failing on exactly one pair means
the table was written as a switch with a fall-through. `TestDelegationCreateIsTheUsersOwnCall`
failing by under a millisecond means `expiresAt` was computed from `time.Now()` rather than
`tx.Now`, which Hub task 14 truncates to the millisecond. `TestEveryTeamWriteIsOneEventWithNoText` counting two events for a
task create means the handler emitted and the store also emitted; the store never emits.

- [ ] **Step 6: Vet, format, and the whole suite**

```bash
gofmt -l control-plane/internal/hub
cd control-plane && go vet ./... && go test ./... -count=1
cd control-plane && go test -tags integration ./internal/hub/... -count=1
```

`gofmt -l` prints nothing. Every package reports `ok` or `[no test files]`; no line says `FAIL`.
The integration run needs Hub task 06's Postgres; without it, it is deferred to task 33 and the
part's record says so.

- [ ] **Step 7: Commit**

Write the message to `.superpowers/sdd/team/msg/t06.txt`, then:

```bash
git add control-plane/internal/hub/server/{delegations,teamtasks,agents,leaseevents,conflicts,teamguard}*.go \
        control-plane/internal/hub/server/team1_world_test.go \
        control-plane/internal/hub/store/{delegations,teamtasks,agents,leaseevents,conflicts}.go
git commit -F .superpowers/sdd/team/msg/t06.txt -- control-plane/internal/hub/server control-plane/internal/hub/store
```

The message says what changed and why (the owner is never a body field; Fleet writes only under a
delegation; the Hub records lease changes but does not second-guess them) and ends with
`Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

#### Acceptance

1. Both `go test` commands of Step 5 pass, and every test in the table of Step 2 exists and names
   its mutant.
2. `grep -n 'json:"ownerUserId\|json:"actor\|json:"via\|json:"delegationId' control-plane/internal/hub/hubapi/team.go`
   shows `ownerUserId` only on record types (`Delegation`, `Task`, `Agent`) and none of the four on
   any `…Request` type; task 01's acceptance grep.
3. `grep -c 's.mux.HandleFunc(' control-plane/internal/hub/server/delegations.go` prints `5` (routes
   46, 48, 49, 87 and 88; b02 added 88) and `grep -c '^	d("' control-plane/internal/hub/server/delegations.go` prints `11`.
4. `go test -tags integration ./internal/hub/server/ -run TestEveryTeamWriteIsOneEventWithNoText -v`
   passes, and a manual `SELECT data FROM hub.audit_events WHERE kind LIKE 'task.%'` on the test
   database shows ids, enums and counts only.
5. (b01) `grep -rn "SET mode" control-plane/internal/hub/` prints exactly one line, in
   `store/projectmode.go`; in `server/projectmode.go`, `grep -n "teamModeGate()\|SetProjectMode("`
   shows the gate's line number smaller than the store call's: the gate runs first.
6. (b01 round 2, re-review I-1) `grep -n "mode" control-plane/internal/hub/store/projects.go`
   shows `mode` in the project-row reader's column list and in its scan, and
   `TestProjectReadsCarryMode` passes on the real Hub: routes 13, 15 and 16 answer `"mode":"team"`
   after route 87, and route 14 answers `"mode":"solo"`, never `""`.
7. (b02, S02-C2) `go run ./cmd/fleetd` is no longer blocked on an unowned Hub read: route 88
   exists, `TestPrincipalOrgIsTheProjects` and `TestAnAuditorOfAnotherOrgHasNoFlagsHere` pass, and
   `grep -c 'SELECT' control-plane/internal/hub/store/principals.go` prints `1` — the project's
   org, the role and the three flags come from one statement, so no code path can answer the
   caller's org by mistake.
8. (b02, S02-C5) `TestBrokerReachesOnlyRoute47`, `TestBrokerReadsARevokedDelegation`,
   `TestBrokerWithAHeaderIs400`, `TestAnUngatedAppRoleIsRefused` and
   `TestFleetSeesNoOwnerTidOnRoute47` pass — the first four are the gate, and
   `TestBrokerReachesOnlyRoute47` must include `GET /v1/orgs/{org}/changes` in the routes it
   proves 403, because route 78's exemption is where an ungated broker would land (b02 round 1,
   C2) — and
   `grep -n 'ownerTid' control-plane/internal/hub/server/delegations.go` shows it set inside the
   broker branch only. Gap S17-1 (`task-17.md:401-416`) is then closed, and task 17's tests can
   leave their `DelegationSource` fake for the real Hub.

#### What this task does not claim

- **Not the Z5 matrix rows.** The per-principal rows for these fourteen routes and route 87,
  including *auditor* and *compliance*, are task 43's; this task's tests cover owner, other member,
  viewer and Fleet with and without a delegation only. Route 87 is not in `everyTeamWrite`, so
  task 43's count check does not catch a missing row for it: task 43 adds it by hand.
- **Not a streamed audit.** Under TM1-α a project turns team only with the dev-only
  `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true`; the real gate is task 45's `TeamModeAllowed`.
- **Not revocation end to end.** A revoked delegation stops Hub writes at once (task 05's check),
  but Fleet noticing it and stopping sessions is task 11, and a member's removal revoking every
  delegation of theirs is task 44.
- **Not breadth.** The Hub caps glob count and shape; the 10% breadth check needs the tracked-file
  list, which only Fleet holds (task 23).
- **Not the feed.** The kinds are written to the audit chain; making them visible to Fleet's
  per-org subscription is task 08.
- **Not that it runs on the Hub branch as built.** Hub tasks 13, 14 and 17 are planned, not built,
  on 2026-09-25; every sibling file this part names is **UNVERIFIED until merge**.

#### Gaps found in the skeleton

1. **The scope paragraph names no reads and no route 47.** Task 01's table gives task 06 routes
   47, 52, 55 and 59; this part builds them. The skeleton's scope for 06 should list them.
1a. **Who owns the two Hub reads nobody owned. Resolved by fix batch b02** (reviews S02-C2 and
   S02-C5): this task owns route 88 (the principal read, contract §7.28) and route 47's broker
   caller. Task 12's `Membership` gap and task 17's gap S17-1 both close on it, so
   `go run ./cmd/fleetd` and the broker's owner derivation stop depending on an unwritten route.
   **Four** deltas land in **task 05's** file, named in Files above and owned here: `Claims.AppRole`
   and `Claims.BrokerRead`; `isExemptPath` over routes 78 and 88; `delegationHeader`'s role arms;
   and `resolveCaller`'s broker branch. The last two are the gate — a second app-only identity and
   the check that tells it from Fleet must land together, or the broker is 401 before route 47 and
   Fleet-shaped on route 78 (b02 round 1, C1 and C2). The skeleton's Hub table carries both rows.
2. **Codes task 03 must carry.** Task 01 uses 409 `conflict` throughout; the Hub's `codes.go` as
   built has no such code, and the skeleton's task 03 list omits it. Task 01 names no 404 for a
   task, agent or conflict.
3. **Who sets a project's `mode` to `team`. Resolved by fix batch b01 (review X-C1):** this task
   owns route 87, the only writer, and route 46 refuses a solo project so the write gates every
   team record. Task 21 wraps it as `TeamProjects.MakeTeam`; task 56's Share path is its product
   caller (`05-design.md` §2.3, §10.1: Share turns a solo project into a team one); task 33's E5
   runs it on the real cluster; task 45 fills the gate. The skeleton's scope for 06 lists it.
