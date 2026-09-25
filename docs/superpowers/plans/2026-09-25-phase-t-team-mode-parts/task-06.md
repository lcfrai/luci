### Task 06: Hub routes, part 1: delegations, tasks, agents, lease events, conflicts

**Slice TM1-α. Depends on: 05, H14, H17. About 3 hours. Seams: ST.2 (defines the routes), ST.3
(creates the delegation record task 05 authenticates).**

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
delegation's owner and set `tx.P.Via = "fleet"` and `tx.P.DelegationID`. So every handler compares
record owners with `tx.P.UserID` and never reads a user id out of JSON. A request type that grew an
`ownerId` or `actor` field would make task 01's acceptance grep fail, and `Decode` refuses unknown
fields, so a body that sends one gets 400 `bad_request`.

**Two callers, two kinds of route.**

| Route group | Who may call | Why |
|---|---|---|
| `POST /v1/delegations`, `…/renew`, `DELETE …` | the user's **desktop** token only; a Fleet token is 403 `forbidden`, `required: "desktop"` | the user's own call creates the delegation (§2.4 item 2); Fleet cannot extend its own authority |
| tasks, agents, lease events, conflicts | **Fleet under a delegation** only; a desktop token is 403 `forbidden`, `required: "via:fleet"` | these records are written by an Orchestrator through `panacea` or by Fleet's forecaster (§3.3 "Written by") |

**Files:**
- `control-plane/internal/hub/server/delegations.go` + `delegations_integration_test.go` (new)
- `control-plane/internal/hub/server/teamtasks.go` + `teamtasks_integration_test.go` (new)
- `control-plane/internal/hub/server/agents.go` + `agents_integration_test.go` (new)
- `control-plane/internal/hub/server/leaseevents.go` + `leaseevents_integration_test.go` (new)
- `control-plane/internal/hub/server/conflicts.go` + `conflicts_integration_test.go` (new)
- `control-plane/internal/hub/server/teamguard.go` + `teamguard_test.go`: the shared guards (new)
- `control-plane/internal/hub/store/{delegations,teamtasks,agents,leaseevents,conflicts}.go` (new)
- `control-plane/internal/hub/server/server.go`: `s.registerTeamPart1()` (modify: one line)
- `control-plane/internal/hub/hubapi/codes.go`: five not-found codes and `team_state` (modify,
  additive; see "Gaps" at the end)

**Interfaces:**
- Consumes:
  - from task 03 (`hubapi/team.go`): `Delegation`, `CreateDelegationRequest`, `Task`,
    `CreateTaskRequest`, `PatchTaskRequest`, `AgentStart`, `AgentStartRequest`, `AgentEndRequest`,
    `LeaseEvent`, `LeaseEventRequest`, `Conflict`, `CreateConflictRequest`, `PatchConflictRequest`,
    the enums with `Valid()`, and the codes `delegation_required`, `delegation_expired`,
    `agent_not_owner`, `text_refused`, `glob_refused`, `breadth_exceeded`. **Where task 03's part
    names a type differently, task 03 wins and this part is renamed mechanically.** The JSON
    below is the wire this task tests, and task 03's golden files must agree with it.
  - from task 04: the tables `delegations`, `team_tasks`, `agents`, `lease_events`, `conflicts`
    under the Hub's RLS pattern.
  - from task 05: `Principal.Via` (`""` or `"fleet"`), `Principal.DelegationID`,
    `Principal.Delegation` (the loaded row: project, class, routes, expiry), and
    `store.AgentOwnedBy(ctx, tx, agentID, userID) (bool, error)`.
  - from task 02: `teamtext.Bound`, `teamtext.Clean`, `teamtext.ValidGlob`.
- Produces: the ten write and three read handlers below; `server.RequireFleet`, `server.RequireDesktop`,
  `server.RequireRoute`; the event kinds in the table after the routes.
- Required by: task 07 (reuses the three guards), task 08 (feed kinds; `hubclient.Team` methods,
  one per route here), task 11 (Fleet's delivery calls), task 14 (the agent start carries the
  policy version and digest), task 21 (the desktop creates the delegation at launch), task 43 (Z5
  rows), task 44 (revocation end to end).

#### The routes — complete wire

Conventions are the Hub contract's §1 (timestamps RFC 3339 UTC to the millisecond, ids by
`hubapi.ValidID`, unknown fields refused, problem bodies per §4). `{p}` is a `pj-…` id.

**`POST /v1/delegations`**: desktop only; project role `member` or `admin` (`viewer` is 403,
§3.4 row 2).

```jsonc
// request
{ "projectId": "pj-…", "class": "orchestrator" | "worker",
  "routes": [ "tasks.write", "agents.write", "leases.write", "conflicts.write",
              "threads.write", "messages.write", "escalations.write", "directives.write",
              "turns.write", "humanmsgs.read" ] }          // 1–10 distinct, from this closed set
// 201
{ "id": "dl-…", "projectId": "pj-…", "ownerId": "us-…", "class": "orchestrator",
  "routes": [ … ], "createdAt": "…", "expiresAt": "…",   // createdAt + 24 h exactly
  "revokedAt": null, "revision": 1 }
```

`routes` never contains a delegation route, so no delegation can create, renew or revoke another.
`ownerId` appears in the **response** only; it is `tx.P.UserID`.

**`POST /v1/delegations/{dl}/renew`**: desktop only, the delegation's owner only (another user is
404 `delegation_not_found`, so a delegation's existence does not leak). Body `{}`. A revoked
delegation is 409 `team_state`, `details.state = "revoked"`. A lapsed one **is** renewable: the
desktop signing back in is exactly the case §2.4 item 4 describes. → `200` the `Delegation`, with
`expiresAt = tx.Now + 24 h` and `revision + 1`.

**`DELETE /v1/delegations/{dl}`**: desktop only, owner only. Sets `revokedAt = tx.Now`. A second
delete is `204` and emits nothing. → `204`.

**`POST /v1/projects/{p}/tasks`**: Fleet, route `tasks.write`, delegation class `orchestrator`.

```jsonc
// request
{ "orchestratorAgentId": "ag-…",                          // must be the owner's, live, role orchestrator
  "title": { "body": "Split the invoice export by region", "author": "human" },   // ≤ 140
  "reason": { "briefRef": "bf-…#s-3", "why": { "body": "…", "author": "agent" } }, // why ≤ 200, agent
  "scope": [ "services/export/**", "docs/export.md" ],    // 0–20 globs, teamtext.ValidGlob each
  "state": "planned" | "active" }
// 201
{ "id": "tk-…", "projectId": "pj-…", "ownerId": "us-…", "orchestratorAgentId": "ag-…",
  "title": {…}, "reason": {…}, "scope": [ … ], "state": "planned",
  "createdAt": "…", "updatedAt": "…", "revision": 1 }
```

**`PATCH /v1/projects/{p}/tasks/{tk}`**: Fleet, `tasks.write`, the task's owner only (another
owner's delegation is 403 `forbidden`, `required: "owner"`; rule 5).

```jsonc
{ "ifRevision": 3,                                        // required; stale → 409 revision_conflict
  "state": "active" | "blocked" | "done" | "abandoned",   // optional
  "scope": [ … ],                                         // optional, replaces
  "reason": { "briefRef": "…", "why": {…} } }             // optional, replaces
```

Transitions: `planned → active | abandoned`; `active → blocked | done | abandoned`;
`blocked → active | abandoned`. `done` and `abandoned` are terminal. Any other is 409 `team_state`,
`details = {"from","to"}`. An empty patch (no field but `ifRevision`) is 400 `bad_request`.

**`POST /v1/projects/{p}/agents`**: Fleet, `agents.write`. Fleet mints the `ag-…` id (task 20);
the Hub validates it and refuses a reuse with 409 `team_state`, `details.state = "exists"`.

```jsonc
{ "id": "ag-…", "role": "orchestrator" | "worker",       // must equal the delegation's class
  "parentAgentId": "ag-…" | null,                         // required for a worker: the owner's live orchestrator
  "harness": { "id": "claude-code", "version": "2.1.14" },// id [a-z0-9-]{1,32}; version ≤ 32, semver-ish
  "taskId": "tk-…" | null,                                // the owner's task, not terminal
  "laneRef": "refs/panacea/lanes/…" | null,               // hubapi.ValidBranchRef and that prefix
  "policy": { "version": 7, "digest": "sha256:…" } }      // task 14: the version the pod mounted
// 201
{ "id": "ag-…", "projectId": "pj-…", "ownerId": "us-…", "role": "worker", … ,
  "startedAt": "…", "endedAt": null, "exitClass": null }
```

**`POST /v1/projects/{p}/agents/{ag}/end`**: Fleet, `agents.write`, the agent's owner only.
`{ "exitClass": "completed" | "stopped" | "crashed" | "access_removed" | "lapsed" }`. A second end
is 409 `team_state`, `details.state = "ended"`. → `200` the agent.

**`POST /v1/projects/{p}/leases/{ls}/events`**: Fleet, `leases.write`. **Fleet is authoritative
for leases** (§3.3 row "lease"); the Hub records each state change and validates shape and
ownership only. It never refuses a transition Fleet made, because a Hub that second-guessed Fleet's
lease engine would make two authorities. The first event for an `ls-…` id creates the lease's
Hub row; every later one appends.

```jsonc
{ "seq": 4,                                               // Fleet's per-lease counter, 1, 2, 3…; a gap or a repeat → 409 team_state
  "holderAgentId": "ag-…",                                // the delegation owner's, unless state is admin-released
  "globs": [ "services/export/**" ],                      // 1–20, teamtext.ValidGlob each
  "mode": "intent" | "hold" | "bulk",
  "state": "requested" | "granted" | "contested" | "yielded" | "released" | "expired" | "admin-released",
  "expiresAt": "…",
  "cause": { "kind": "verb" | "observed" | "resolution" | "admin", "ref": "th-…" | "es-…" | null } }
// 201
{ "leaseId": "ls-…", "seq": 4, "state": "granted", "at": "…", "revision": 4 }
```

The Hub checks `ValidGlob` and the count of 20, not breadth: it does not hold the tracked-file list,
so the 10% check is Fleet's (task 23). `admin-released` is the one state another person may write
(§3.4 last row): it requires the delegation owner to be a project `admin`, it records `releasedBy`,
and it emits its own event kind so an audit query finds every one.

**`POST /v1/projects/{p}/conflicts`**: Fleet, `conflicts.write`. A conflict has **two owners**, so
it is written under the delegation of **either party**: the Hub requires the delegation's owner to
own `agents[0]` or `agents[1]`, and both agents to be in `{p}`. The audit line reads `actor = that
owner, via = fleet` and `data.raisedBy = "forecaster"`, so nobody reads it as the person's own act.

```jsonc
{ "agents": [ "ag-…", "ag-…" ],                           // exactly 2, distinct, different owners
  "tier": "P" | "H" | "M" | "S",
  "paths": [ { "path": "services/export/region.go",       // ValidGlob with no metacharacter, ≤ 200
               "ranges": [ [ 40, 88 ] ] } ],               // 1–50 paths; 0–20 ranges each, 1 ≤ start ≤ end
  "basis": { "a": "<40 hex>", "b": "<40 hex>" } }         // tier P may send nulls: intent has no commit yet
// 201
{ "id": "cf-…", "projectId": "pj-…", "agents": [ … ], "tier": "P", "paths": [ … ],
  "basis": { … }, "state": "raised", "resolution": null, "createdAt": "…", "revision": 1 }
```

**`PATCH /v1/projects/{p}/conflicts/{cf}`**: Fleet, `conflicts.write`, either party's delegation.
`{ "ifRevision": 2, "state": "negotiating" | "accepted" | "resolved" | "escalated" | "cleared",
"resolution": "yield" | "sequence" | "proceed-both" | "counter" | "split" | "build-on" | null,
"threadId": "th-…" | null }`. `resolution` is required with `resolved` and refused otherwise.
`resolved` and `cleared` are terminal (409 `team_state`).

**The session-id rule.** Solo projects keep the Hub's `session_link` route (contract §7.8)
unchanged. No request type above has a `sessionId` field, so `Decode` answers a body carrying one
with 400 `bad_request`, `details.field = "sessionId"`. Beyond that, every string field that is an
id is checked with `hubapi.ValidID` for its own prefix, so a phase 2 session id pasted into
`laneRef`, `taskId` or `parentAgentId` is 400 too; one test sends one to each.

#### Events

One metadata-only event per changed record, in the write's transaction (contract §8). `data` holds
**no** text: no title, no why-line, no glob list; only ids, enums and counts.

| Kind | `projectId` | `subject` | `data` | In the feed |
|---|---|---|---|---|
| `delegation.created` | ✓ | dl id | `{"class","routes"}` (routes joined by `,`) | yes |
| `delegation.renewed` | ✓ | dl id | `{"expiresAt"}` | yes |
| `delegation.revoked` | ✓ | dl id | `{}` | yes |
| `task.created` | ✓ | tk id | `{"state","orchestratorAgentId","scopeCount"}` | yes |
| `task.updated` | ✓ | tk id | `{"state","previousState","scopeCount"}` | yes |
| `agent.started` | ✓ | ag id | `{"role","parentAgentId","taskId","harness","harnessVersion","policyVersion","policyDigest"}` | yes |
| `agent.ended` | ✓ | ag id | `{"exitClass"}` | yes |
| `lease.changed` | ✓ | ls id | `{"seq","holderAgentId","mode","state","globCount"}` | yes |
| `lease.admin_released` | ✓ | ls id | `{"seq","holderAgentId","releasedBy"}` | yes |
| `conflict.raised` | ✓ | cf id | `{"tier","agentA","agentB","pathCount","raisedBy"}` | yes |
| `conflict.updated` | ✓ | cf id | `{"state","previousState","resolution","threadId"}` | yes |

Every event from a Fleet call also carries `via = "fleet"` and `delegationId` in the audit row's
actor columns (task 05), not in `data`. Task 08 registers these kinds on the feed; until it merges
they are written but appear in no per-org subscription. **If task 01's contract §8 rows name a kind
differently, task 01 wins.**

#### The guards — complete source

```go
// control-plane/internal/hub/server/teamguard.go
package server

import (
	"slices"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// RequireDesktop refuses a Fleet call. Delegation routes are the user's own act (§2.4 item 2).
func RequireDesktop(p Principal) error {
	if p.Via != "" {
		return Forbidden("desktop")
	}
	return nil
}

// RequireFleet refuses a desktop call to a route only Fleet writes, and a Fleet call
// whose delegation does not list route, is for another project, or has the wrong class.
// class "" means either class may call.
func RequireFleet(p Principal, projectID, route string, class hubapi.AgentRole) error {
	if p.Via != "fleet" {
		return Forbidden("via:fleet")
	}
	d := p.Delegation
	if d.ProjectID != projectID {
		// A delegation for another project is not a 404 of that project: the caller is
		// Fleet, and telling it which delegation it used is safe.
		return Forbidden("delegation.project")
	}
	if !slices.Contains(d.Routes, route) {
		return Forbidden("delegation.route:" + route)
	}
	if class != "" && d.Class != class {
		return Forbidden("delegation.class:" + string(class))
	}
	return nil
}

// RequireOwner is rule 5: nobody writes another person's record, whatever their role.
func RequireOwner(p Principal, ownerID string) error {
	if p.UserID != ownerID {
		return Forbidden("owner")
	}
	return nil
}

// RequireParty is RequireOwner for a record with two owners (a conflict, a thread).
func RequireParty(p Principal, ownerA, ownerB string) error {
	if p.UserID != ownerA && p.UserID != ownerB {
		return Forbidden("party")
	}
	return nil
}
```

`Forbidden` is Hub task 14's; it also records `auth.forbidden` in its own short transaction, so a
refused Fleet write leaves an audit trace even though its transaction rolls back. `RequireFleet`
does **not** re-check expiry or revocation: task 05's auth pipeline did that before the handler
ran, and doing it twice would give two answers for one question the day they drift.

`p.Delegation.Class` is typed `hubapi.AgentRole` because the two sets are the same (`orchestrator`,
`worker`); the `mirror` class is Fleet-internal (task 58) and never holds a delegation.

#### The handler shape — one route in full

Every other handler has the same five lines before its store call; only the checks after them
differ, and those are the rules in the route text above.

```go
// control-plane/internal/hub/server/teamtasks.go (excerpt: the create handler)
func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("p")
	s.Write(w, r, func(tx *Tx, body []byte) (WriteResult, error) {
		if err := RequireFleet(tx.P, projectID, "tasks.write", hubapi.AgentRoleOrchestrator); err != nil {
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
		owned, err := store.AgentOwnedBy(r.Context(), tx, req.OrchestratorAgentID, tx.P.UserID)
		if err != nil {
			return WriteResult{}, err
		}
		if !owned {
			return WriteResult{}, Refuse(403, hubapi.CodeAgentNotOwner, "That agent is not yours.",
				map[string]string{"agentId": req.OrchestratorAgentID})
		}
		t, err := store.CreateTask(r.Context(), tx, projectID, tx.P.UserID, req)
		if err != nil {
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: "task.created", ProjectID: projectID, Subject: t.ID,
			Data: map[string]any{"state": string(t.State),
				"orchestratorAgentId": t.OrchestratorAgentID, "scopeCount": int64(len(t.Scope))}})
		return WriteResult{Status: 201, Body: t}, nil
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

func fleetP(user, project string, class hubapi.AgentRole, routes ...string) Principal {
	return Principal{UserID: user, Via: "fleet", DelegationID: "dl-TEST",
		Delegation: DelegationView{ProjectID: project, Class: class, Routes: routes}}
}

func required(t *testing.T, err error) string {
	t.Helper()
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 403 {
		t.Fatalf("want a 403 APIError, got %v", err)
	}
	return ae.Details["required"]
}

// Mutant killed: RequireFleet checks the route but not Via, so a desktop token whose
// Delegation field is zero but whose request names a listed route slips through.
func TestRequireFleetRefusesADesktopCaller(t *testing.T) {
	if got := required(t, RequireFleet(Principal{UserID: "us-L"}, "pj-P", "tasks.write", "")); got != "via:fleet" {
		t.Fatalf("required = %q", got)
	}
}

// Mutant killed: the project comparison is dropped, so Linus's delegation for P writes into Q.
func TestRequireFleetRefusesAnotherProjectsDelegation(t *testing.T) {
	p := fleetP("us-L", "pj-P", hubapi.AgentRoleOrchestrator, "tasks.write")
	if got := required(t, RequireFleet(p, "pj-Q", "tasks.write", "")); got != "delegation.project" {
		t.Fatalf("required = %q", got)
	}
}

// Mutant killed: slices.Contains replaced by a prefix match, so "tasks" covers "tasks.write".
func TestRequireFleetRefusesAnUnlistedRoute(t *testing.T) {
	p := fleetP("us-L", "pj-P", hubapi.AgentRoleWorker, "agents.write", "tasks")
	if got := required(t, RequireFleet(p, "pj-P", "tasks.write", "")); got != "delegation.route:tasks.write" {
		t.Fatalf("required = %q", got)
	}
}

// Mutant killed: the class check is skipped, so a worker's delegation creates tasks,
// which §3.3 gives to the Orchestrator alone.
func TestRequireFleetRefusesTheWrongClass(t *testing.T) {
	p := fleetP("us-L", "pj-P", hubapi.AgentRoleWorker, "tasks.write")
	err := RequireFleet(p, "pj-P", "tasks.write", hubapi.AgentRoleOrchestrator)
	if got := required(t, err); got != "delegation.class:orchestrator" {
		t.Fatalf("required = %q", got)
	}
}

// Mutant killed: RequireDesktop tests Via == "fleet" rather than Via != "", so a future
// second delegated caller would be treated as a desktop.
func TestRequireDesktopRefusesAnyDelegatedCaller(t *testing.T) {
	if got := required(t, RequireDesktop(Principal{UserID: "us-L", Via: "other"})); got != "desktop" {
		t.Fatalf("required = %q", got)
	}
	if err := RequireDesktop(Principal{UserID: "us-L"}); err != nil {
		t.Fatalf("a desktop caller was refused: %v", err)
	}
}

// Mutant killed: RequireParty accepts when either comparison is true of the *other* field
// (ownerA twice), so a conflict between Linus and Grace accepts Vera's delegation.
func TestRequirePartyNeedsOneOfTheTwo(t *testing.T) {
	if err := RequireParty(Principal{UserID: "us-G"}, "us-L", "us-G"); err != nil {
		t.Fatalf("party B refused: %v", err)
	}
	if got := required(t, RequireParty(Principal{UserID: "us-V"}, "us-L", "us-G")); got != "party" {
		t.Fatalf("required = %q", got)
	}
}
```

`DelegationView` is task 05's name for the loaded row on the principal (**UNVERIFIED**: task 05's
part is written in parallel; if it calls it something else, rename here). `Forbidden` records an
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
// member and class, and one orchestrator agent each. Built fresh per test.
type teamWorld struct {
	h                    *hubHarness // Hub task 06's harness: server, pool, clock
	ProjectID            string
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
| `TestDelegationCreateIsTheUsersOwnCall` | Linus's desktop → 201, `ownerId` = Linus, `expiresAt − createdAt` = 24 h exactly | expiry from config instead of the constant; owner read from a body field |
| `TestDelegationCreateRefusesAViewer` | Vera → 403 `forbidden`, `required: "member"` | the role floor is `viewer` |
| `TestDelegationCreateRefusesFleet` | `fleet(DLLinusOrch)` → 403 `required: "desktop"` | `RequireDesktop` omitted, so Fleet mints its own authority |
| `TestDelegationRoutesAreAClosedSet` | `routes: ["delegations.write"]` → 400, `field: "routes"`; a duplicate → 400 | the set check is a prefix or a regexp |
| `TestDelegationRenewIsOwnerOnlyAndHidesExistence` | Grace renews Linus's → 404 `delegation_not_found`; Linus → 200, `expiresAt` moved, `revision` 2 | the owner check answers 403, leaking the id |
| `TestDelegationRenewRevivesALapsedOneButNotARevokedOne` | clock +25 h, renew → 200; `DELETE` then renew → 409 `team_state` | the lapsed check shares the revoked branch |
| `TestDelegationDeleteIsIdempotentAndEmitsOnce` | two `DELETE`s → 204, 204; one `delegation.revoked` in the audit | the second delete emits again |
| `TestTaskCreateNeedsFleetUnderAnOrchestratorDelegation` | Linus desktop → 403 `via:fleet`; `fleet("")` → 401 `delegation_required` (task 05); `fleet(DLLinusWorker)` → 403 `delegation.class:orchestrator`; `fleet(DLLinusOrch)` → 201 | any one of the three guards removed |
| `TestTaskCreateRefusesAnotherOwnersAgent` | `fleet(DLLinusOrch)` with `AGGraceOrch` → 403 `agent_not_owner` | the agent check compares the agent's project, not its owner |
| `TestTaskPatchIsOwnerOnly` | `fleet(DLGraceOrch)` patching Linus's task → 403 `required: "owner"`, although Grace is team admin | admin exempted from rule 5 |
| `TestTaskTransitions` | every pair of the 5×5 state table: allowed → 200, others → 409 `team_state` with `from`/`to` | one terminal state left open (e.g. `done → active`) |
| `TestTaskPatchNeedsTheCurrentRevision` | `ifRevision` 1 after a patch → 409 `revision_conflict` | the `WHERE revision = $n` guard dropped |
| `TestTaskTextIsBoundedAndCleaned` | title of 141 runes → 400 `text_refused`, `details.field = "title"`; title with `U+202E` → 400 `text_refused`; why-line of 201 → 400 | the bound counts bytes, not runes; `Clean` skipped on the why-line |
| `TestTaskScopeGlobs` | 21 globs → 400 `glob_refused`; `../x`, `/abs`, `a\b` → 400 `glob_refused` | the count cap applied after dedupe only |
| `TestAgentStartBindsRoleClassAndParent` | worker with no parent → 400; worker whose parent is Grace's → 403 `agent_not_owner`; role `worker` under an orchestrator delegation → 403 `delegation.class:worker`; a reused `ag-…` → 409 | the parent's owner not checked |
| `TestAgentStartRecordsThePolicy` | the `agent.started` event's `data` has `policyVersion` and `policyDigest` exactly as sent | the digest is dropped from the event (task 14 then has no audit fact) |
| `TestAgentEndTwiceIs409` | end, end → 200, 409 `team_state` `ended` | a second end overwrites `exitClass` |
| `TestLeaseEventsAreAppendOnlyAndGapless` | seq 1, 2 → 201; seq 2 again → 409; seq 4 → 409 | the gap check is `>` instead of `== last + 1` |
| `TestLeaseEventHolderMustBeTheOwners` | holder `AGGraceOrch` under `DLLinusOrch` → 403 `agent_not_owner` | the holder check skipped when `cause.kind = observed` |
| `TestAdminReleaseNeedsAnAdminAndIsItsOwnKind` | `admin-released` under Linus's delegation (member) of Grace's lease → 403 `required: "admin"`; under Grace's delegation of Linus's lease → 201 and one `lease.admin_released` | admin release allowed to members; it reuses `lease.changed` |
| `TestConflictIsWrittenByEitherPartyOnly` | create with Linus's and Grace's agents under `DLLinusOrch` → 201; under a third member's delegation → 403 `required: "party"`; two agents of one owner → 400 | the party check is replaced by rule 5's owner check |
| `TestConflictResolutionOnlyWithResolved` | `resolved` with no resolution → 400; `negotiating` with one → 400; `cleared` then anything → 409 | resolution accepted on any state |
| `TestNoTeamBodyCarriesASessionID` | each of the ten write bodies plus `"sessionId": "se-…"` → 400 `bad_request`, `field: "sessionId"`; a phase 2 session id in `laneRef`, `taskId`, `parentAgentId` → 400 | `Decode` with unknown fields allowed; `ValidID` skipped on optional ids |
| `TestEveryTeamWriteIsOneEventWithNoText` | after one of each write, the audit has exactly one event per write, and no event's canonical `data` contains the title or the why-line | an event emitted per field changed; the title copied into `data` |
| `TestFleetEventsCarryViaAndDelegation` | every event from a Fleet call has `actor = owner`, `via = fleet`, `delegation = dl-…` | the actor recorded as Fleet's service principal |

Two of them in full; the rest follow the same shape and live beside their route's file.

```go
// control-plane/internal/hub/server/teamtasks_integration_test.go (excerpt)
//go:build integration

package server_test

// Mutant killed: the admin exemption. Grace is T's admin and Fleet holds a live
// delegation of hers; she still may not change Linus's task (§3.3 rule 5).
func TestTaskPatchIsOwnerOnly(t *testing.T) {
	w := newTeamWorld(t)
	tk := w.fleet(w.DLLinusOrch).mustPost(t, "/v1/projects/"+w.ProjectID+"/tasks", map[string]any{
		"orchestratorAgentId": w.AGLinusOrch,
		"title":  map[string]any{"body": "Split the export", "author": "human"},
		"reason": map[string]any{"briefRef": "bf-AAAAAAAAAAAAAAAAAAAAAAAAAA#s-1",
			"why": map[string]any{"body": "Brief s-1 asks for it", "author": "agent"}},
		"scope": []string{"services/export/**"}, "state": "planned",
	}, 201)
	res := w.fleet(w.DLGraceOrch).patch(t, "/v1/projects/"+w.ProjectID+"/tasks/"+tk.ID(),
		map[string]any{"ifRevision": 1, "state": "abandoned"})
	res.wantProblem(t, 403, "forbidden", map[string]string{"required": "owner"})
	if got := w.h.auditKinds(t); !slices.Contains(got, "auth.forbidden") {
		t.Fatalf("no auth.forbidden in %v", got)
	}
}

// Mutant killed: the session-id rule enforced only on the routes that carry an id field.
func TestNoTeamBodyCarriesASessionID(t *testing.T) {
	w := newTeamWorld(t)
	for _, c := range w.everyTeamWrite() { // ten (caller, method, path, valid body) cases
		body := maps.Clone(c.body)
		body["sessionId"] = "se-01J9ZK3Y6N4Q0V9T8W7X6Y5Z4A"
		res := c.caller.do(t, c.method, c.path, body)
		res.wantProblem(t, 400, "bad_request", map[string]string{"field": "sessionId"})
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
cd control-plane && go test -tags integration ./internal/hub/server/ -run 'TestDelegation|TestTask|TestAgent|TestLease|TestAdmin|TestConflict|TestNoTeam|TestEveryTeam|TestFleetEvents' -count=1
```

Expected: `FAIL` with every route answering 404 `route_not_found`, because nothing is registered.

- [ ] **Step 4: Write the guards, the store functions and the five handler files**

In this order, running the unit tests after the first: `teamguard.go` (above, complete); the store
functions, one file per record, each a single parameterised statement under `tx` with
`RETURNING` the full row; then the handlers. The store functions set nothing the handler did not
pass, and every `UPDATE` carries `WHERE id = $1 AND revision = $2`, so an `ifRevision` race is a
zero-row update the handler turns into 409 `revision_conflict`.

The route registration, in full:

```go
// control-plane/internal/hub/server/delegations.go (excerpt)
func (s *Server) registerTeamPart1() {
	s.mux.HandleFunc("POST /v1/delegations", s.createDelegation)
	s.mux.HandleFunc("POST /v1/delegations/{dl}/renew", s.renewDelegation)
	s.mux.HandleFunc("DELETE /v1/delegations/{dl}", s.revokeDelegation)
	s.mux.HandleFunc("POST /v1/projects/{p}/tasks", s.createTask)
	s.mux.HandleFunc("PATCH /v1/projects/{p}/tasks/{tk}", s.patchTask)
	s.mux.HandleFunc("POST /v1/projects/{p}/agents", s.startAgent)
	s.mux.HandleFunc("POST /v1/projects/{p}/agents/{ag}/end", s.endAgent)
	s.mux.HandleFunc("POST /v1/projects/{p}/leases/{ls}/events", s.leaseEvent)
	s.mux.HandleFunc("POST /v1/projects/{p}/conflicts", s.createConflict)
	s.mux.HandleFunc("PATCH /v1/projects/{p}/conflicts/{cf}", s.patchConflict)
	s.mux.HandleFunc("GET /v1/projects/{p}/tasks", s.listTasks)
	s.mux.HandleFunc("GET /v1/projects/{p}/agents", s.listAgents)
	s.mux.HandleFunc("GET /v1/projects/{p}/conflicts", s.listConflicts)
}
```

Ten writes and three reads. `s.mux` is the name Hub task 13's server uses (**UNVERIFIED until
merge**; if the field differs, rename).

**The three reads** exist because task 11's `rebuild.go` reloads Hub-owned state after a Fleet
restart, and the scope paragraph names no route for it (see "Gaps"). They are the Hub's usual
page shape (`hubapi.Page[T]`, `limit` 1–200, `cursor`), run under `s.Read`, and are open to every
project role including `viewer` (§3.4 row 1: the team strip). Filters: tasks `?state=` (repeatable),
agents `?live=true` (no `endedAt`), conflicts `?open=true` (not `resolved` or `cleared`). **A task's
`title` and `reason.why` are answered only to its owner and to Fleet under an orchestrator-class
delegation; everyone else gets `null`** (§3.3 row "task": "served to orchestrators only", §5.1).
`TestTaskTitleIsServedToOrchestratorsAndItsOwnerOnly` pins it: Vera and Grace's desktop see
`null`, Linus's desktop and `fleet(DLGraceOrch)` see the text. Mutant killed: the redaction applied
to the list but not to the single-row body of a `PATCH` answer, which is why the `PATCH` answer
goes through the same `renderTask(p, t)` function.

The new codes, appended to `hubapi/codes.go` beside task 03's:

```go
	CodeDelegationNotFound = "delegation_not_found" // 404
	CodeTaskNotFound       = "task_not_found"       // 404
	CodeAgentNotFound      = "agent_not_found"      // 404
	CodeLeaseNotFound      = "lease_not_found"      // 404 (task 08's reads)
	CodeConflictNotFound   = "conflict_not_found"   // 404
	CodeTeamState          = "team_state"           // 409, details.state or details.from/to
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane && go test ./internal/hub/server/ -run 'TestRequire' -count=1 -v
cd control-plane && go test -tags integration ./internal/hub/server/ -count=1
```

Expected: the six guard tests `--- PASS`, and the integration run ends
`ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/server	<n>s` with no `FAIL`
line. Three failures are worth predicting. `TestTaskTransitions` failing on exactly one pair means
the table was written as a switch with a fall-through. `TestDelegationCreateIsTheUsersOwnCall`
failing by one millisecond means `expiresAt` was computed from `time.Now()` rather than `tx.Now`,
which Hub task 14 truncates. `TestEveryTeamWriteIsOneEventWithNoText` counting two events for a
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
git commit -F .superpowers/sdd/team/msg/t06.txt -- control-plane/internal/hub
```

The message says what changed and why (the owner is never a body field; Fleet writes only under a
delegation; the Hub records lease changes but does not second-guess them) and ends with
`Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

#### Acceptance

1. Both `go test` commands of Step 5 pass, and every test in the table of Step 2 exists and names
   its mutant.
2. `grep -n '"ownerId"\|"actor"' control-plane/internal/hub/hubapi/team.go` shows `ownerId` only
   on response types (`Delegation`, `Task`, `AgentStart`), and no request type; the task 01 grep.
3. `grep -c 'HandleFunc' control-plane/internal/hub/server/delegations.go` prints `13`.
4. `go test -tags integration ./internal/hub/server/ -run TestEveryTeamWriteIsOneEventWithNoText -v`
   passes, and a manual `SELECT data FROM hub.audit_events WHERE kind LIKE 'task.%'` on the test
   database shows ids, enums and counts only.

#### What this task does not claim

- **Not the Z5 matrix rows.** The per-principal rows for these thirteen routes, including *auditor*
  and *compliance*, are task 43's; this task's tests cover owner, other member, viewer and Fleet
  with and without a delegation only.
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

1. **No read routes for rebuild.** Task 11's `rebuild.go` "reloads Hub-owned state", but no scope
   paragraph adds a team `GET`. This part adds three list reads (tasks, agents, conflicts); threads
   and escalations need the same in task 07.
2. **Not-found and state codes.** Task 03's code list has no `*_not_found` for the new records and
   no state-transition code; this part adds six to `codes.go`. Task 03's part should list them.
3. **"Fleet for itself".** Task 07's scope has Fleet writing escalations "for itself". With task
   05's rule (a Fleet token with no delegation is 401), every Fleet write names *some* delegation;
   this part resolves conflicts, which have two owners, by accepting either party's delegation.
4. **Team versus solo project.** The scopes name no Hub field that marks a project as a team
   project (§2.3, Share). Task 08's "team projects only" feed filter needs one; this part does not
   invent it.
