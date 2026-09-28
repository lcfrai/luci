### Task 08: Hub feed kinds, Fleet's per-org subscription; `hubclient` team methods, fake, conformance

**Slice TM1-α. Depends on: 07, H26, H33, H34. About 3 hours. Seam: ST.2.**

Three pieces, one purpose: after this task, Fleet's tests (tasks 11, 22 to 31) run against an
in-memory Hub that keeps every rule of routes 46 to 71, and the same rows prove the real Hub keeps
them too.

1. **Route 78, `GET /v1/orgs/{orgId}/changes?after=&kinds=&limit=`**: Fleet's per-org feed (task
   01 §7.26, `task-01.md:473-477`), beside the member feed Hub task 26 builds.
2. **`hubclient`**: a `Team` interface with one method per route of tasks 06 and 07, an `OrgFeed`
   interface, and two context helpers, `WithDelegation` and `WithIdempotencyKey`. The HTTP half is
   written on Hub tasks 33 and 34's transport.
3. **The fake's team half and the conformance rows**: `fake_team*.go`, and two new conformance
   groups that run against the fake in the default `go test` and against the real Hub under Hub
   task 34's integration tag.

It is a delta on the Hub branch, built after the sync-merge of `feat/hub`. It extends these
sibling files:

| Sibling file | State on 2026-09-25 | What this task adds |
|---|---|---|
| `control-plane/internal/hub/hubclient/hubclient.go` (`:96-109`, `Feed` and `Hub`) | **built** (Hub task 04, `142bc7b`) | `Team`, `OrgFeed`; `Hub` embeds `Team` |
| `control-plane/internal/hub/hubclient/fake.go` (`:31-91`, `Fake`, `FakeUser`, `fakeWorld`, `As`) | **built** (Hub task 04) | `FakeFleet`, `AsFleet`, the team maps on `fakeWorld` |
| `control-plane/internal/hub/hubclient/conformance/conformance.go` (`Env`, `Group`, `Run`) | planned (Hub task 05, `task-05.md:36-72`); **UNVERIFIED until merge** | `GroupTeam`, `GroupTeamFeed`; `Env` gains Fleet and a team project |
| `control-plane/internal/hub/hubclient/client.go` (the HTTP transport) | the file exists; its transport is Hub task 33, **UNVERIFIED until merge** | the header and idempotency-key overrides; the team methods |
| `control-plane/internal/hub/server/changes.go`, `store/changes.go` | planned (Hub task 26, `task-26.md:12-37`); **UNVERIFIED until merge** | route 78, and the member feed's exclusion of `fleet` kinds |

**The one thing this task must not get wrong is that the fake is a second implementation of the
Hub's rules, and Fleet's tests trust it.** A rule the fake forgets is a rule every Fleet test
silently skips. So each rule of tasks 06 and 07 that a caller can observe is a conformance row,
and each row runs against both. A rule the fake cannot keep (anything that needs Postgres: RLS,
the append-only triggers, a `CHECK` bound) is named in "What this task does not claim", not
approximated.

**Files:**
- `control-plane/internal/hub/hubapi/teamquery.go` + `teamquery_test.go`: `TaskQuery`,
  `AgentQuery`, `ConflictQuery` (new; task 03 does not define them)
- `control-plane/internal/hub/hubapi/principal.go`: `Principal{UserID, OrgID, ProjectID, Role,
  OrgAdmin, Auditor, Compliance}`, route 88's answer, contract §7.28, and `Delegation`'s two
  broker-only fields `OwnerTID`/`OwnerOID` (`json:",omitempty"`) — task 03 defines neither (new;
  fix batch b02). It holds no name, no mail address and no free text: §7.28 rule 5
- `control-plane/internal/hub/hubclient/hubclient.go`: `Team`, `OrgFeed`, `Hub` (modify)
- `control-plane/internal/hub/hubclient/context.go`: `WithDelegation`, `WithIdempotencyKey`,
  `DelegationFrom`, `IdempotencyKeyFrom` (new)
- `control-plane/internal/hub/hubclient/client_team.go`: the HTTP methods, route 87's
  `SetProjectMode` among them (new)
- `control-plane/internal/hub/hubclient/fake_team.go`: delegations, tasks, agents, leases,
  conflicts (new)
- `control-plane/internal/hub/hubclient/fake_team_protocol.go`: threads, messages, escalations,
  directives, turns, human messages, and the org feed (new)
- `control-plane/internal/hub/hubclient/conformance/team.go`: the rows of both groups (new)
- `control-plane/internal/hub/hubclient/conformance/conformance.go`: `Env`, the group list (modify)
- `control-plane/internal/hub/server/orgchanges.go` + `orgchanges_integration_test.go` (new)
- `control-plane/internal/hub/store/changes.go`: `OrgChanges`, and `fleet_feed` in the member
  query (modify)
- `control-plane/internal/hub/server/server.go`: `s.registerOrgChanges()` (modify: one line)

**Interfaces:**
- Consumes: task 01's route 78 and §8 feed column (`task-01.md:473-513`); task 03's types (the
  same list tasks 06 and 07 consume, plus `TaskQuery`, `AgentQuery`, `ConflictQuery`); tasks 06 and
  07's handlers; task 04's `fleet_feed` flag and a project's `mode`; task 05's exempt route and
  Fleet principal; Hub task 26's `store.Changes`; Hub task 33's transport (`c.do`).
- Produces: the interfaces below, complete; `hubclient.NewFake().AsFleet()` and
  `.AsBroker()` (b02); `hubapi.Principal` and `Delegation`'s two broker-only fields (b02); the
  conformance groups `GroupTeam` and `GroupTeamFeed`.
- Required by: task 11 (every call it makes, `task-11.md:86`), task 12 (`/readyz` calls
  `OrgChanges` with `after=latest`, **and every Fleet route's `authorize` calls `Principal`** —
  b02, S02-C2: without it task 12 does not build), tasks 20 to 31 (their tests use the fake),
  task 21 and task 52 (the desktop's delegation, escalation-answer and human-message calls), and
  phase 8's broker through `Delegation` with `OwnerTID`/`OwnerOID` (b02, S02-C5; `task-17.md:51`).

#### The interfaces — complete

```go
// control-plane/internal/hub/hubclient/hubclient.go (the additions)

// Team is every route of Phase T tasks 06 and 07. Fleet calls the writes and the
// Fleet·O reads under a delegation (WithDelegation); the desktop calls the
// delegation, answer and human-message routes as the signed-in user. The Hub, and
// the fake, refuse the wrong caller; the interface does not split them, because
// task 01's matrix, not the type system, is the authority on who may call what.
//
// Every Fleet write has the same shape, (ctx, projectID, subjectID, req), so that
// Fleet's outbox can hold one method expression per operation (task-11.md:534-540).
// subjectID is the record id in the path, or "" for a create whose path has none.
type Team interface {
	// Desktop: routes 46, 48, 49. Route 47 (Delegation) also Fleet, under the
	// delegation it reads (delegations.read, task-06.md's route 47), and also the
	// broker as itself (app-only, Hub.DelegationReader, ADR-H1 §5.10 rule 10; fix
	// batch b02, S02-C5). Only the broker's answer carries OwnerTID and OwnerOID;
	// both are omitempty, so a member's and Fleet's answers are unchanged.
	CreateDelegation(ctx context.Context, req hubapi.CreateDelegationRequest) (hubapi.Delegation, error)
	Delegation(ctx context.Context, delegationID string) (hubapi.Delegation, error)
	RenewDelegation(ctx context.Context, delegationID string) (hubapi.Delegation, error)
	RevokeDelegation(ctx context.Context, delegationID string) error

	// Desktop, a team admin: route 87 (fix batch b01, X-C1), the one writer of a
	// project's mode. One way; 503 unavailable, details.reason audit_not_streaming,
	// while the Hub's audit gate is shut. The real Hub's and the fake's only path to
	// a team project: no test sets the mode any other way.
	SetProjectMode(ctx context.Context, projectID string, mode hubapi.ProjectMode) (hubapi.Project, error)

	// Fleet writes: routes 50, 51, 53, 54, 56, 57, 58.
	CreateTask(ctx context.Context, projectID, _ string, req hubapi.CreateTaskRequest) (hubapi.Task, error)
	PatchTask(ctx context.Context, projectID, taskID string, req hubapi.PatchTaskRequest) (hubapi.Task, error)
	StartAgent(ctx context.Context, projectID, _ string, req hubapi.AgentStartRequest) (hubapi.AgentStart, error)
	EndAgent(ctx context.Context, projectID, agentID string, req hubapi.AgentEndRequest) (hubapi.AgentStart, error)
	AppendLeaseEvent(ctx context.Context, projectID, leaseID string, req hubapi.LeaseEventRequest) (hubapi.LeaseEvent, error)
	CreateConflict(ctx context.Context, projectID, _ string, req hubapi.CreateConflictRequest) (hubapi.Conflict, error)
	PatchConflict(ctx context.Context, projectID, conflictID string, req hubapi.PatchConflictRequest) (hubapi.Conflict, error)

	// Fleet, with NO delegation: route 88, contract §7.28 (fix batch b02, S02-C2).
	// The one Fleet read called before a delegation is in hand, and the only source
	// of a caller's standing: every Fleet route's authorize() is built on it
	// (task-12.md's Membership). orgID in the answer is the PROJECT's org, never
	// the caller's, and the three flags are that org's; a caller who compares them
	// with its own token has misread §7.28. Under WithDelegation it is 400.
	// An unprovisioned (tid, oid) is principal_not_found; an unknown, another
	// org's, or an unseeable project is route_not_found, the same answer.
	Principal(ctx context.Context, tid, oid, projectID string) (hubapi.Principal, error)

	// Reads: routes 52, 55, 59.
	Tasks(ctx context.Context, projectID string, q hubapi.TaskQuery) (hubapi.Page[hubapi.Task], error)
	Agents(ctx context.Context, projectID string, q hubapi.AgentQuery) (hubapi.Page[hubapi.AgentStart], error)
	Conflicts(ctx context.Context, projectID string, q hubapi.ConflictQuery) (hubapi.Page[hubapi.Conflict], error)

	// Fleet writes: routes 60–62, 64, 67, 69.
	CreateThread(ctx context.Context, projectID, _ string, req hubapi.CreateThreadRequest) (hubapi.Thread, error)
	PatchThread(ctx context.Context, projectID, threadID string, req hubapi.PatchThreadRequest) (hubapi.Thread, error)
	AppendMessage(ctx context.Context, projectID, threadID string, req hubapi.AppendMessageRequest) (hubapi.Message, error)
	CreateEscalation(ctx context.Context, projectID, _ string, req hubapi.CreateEscalationRequest) (hubapi.Escalation, error)
	AppendDirective(ctx context.Context, projectID, _ string, req hubapi.AppendDirectiveRequest) (hubapi.Directive, error)
	RecordTurn(ctx context.Context, projectID, _ string, req hubapi.RecordTurnRequest) (hubapi.TurnRecorded, error)

	// Private reads: routes 63, 66, 68, 71. The Hub filters to the caller's own,
	// or everyone's for an org auditor's desktop.
	Messages(ctx context.Context, projectID, threadID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Message], error)
	Escalations(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Escalation], error)
	Directives(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Directive], error)
	HumanMessages(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[hubapi.HumanMessage], error)

	// Desktop: routes 65, 70.
	AnswerEscalation(ctx context.Context, escalationID string, req hubapi.AnswerEscalationRequest) (hubapi.Escalation, error)
	WriteHumanMessage(ctx context.Context, projectID string, req hubapi.CreateHumanMessageRequest) (hubapi.HumanMessage, error)
}

// OrgFeed is route 78: Fleet's per-org twin of Feed.Changes, over team projects
// only. after is a decimal seq or "latest"; kinds nil means every kind.
type OrgFeed interface {
	OrgChanges(ctx context.Context, orgID, after string, kinds []string, limit int) (hubapi.ChangesPage, error)

	// OrgEvent is a BY-ID read of one Fleet-feed event: route 78 with
	// subject=, after empty, limit 1. It is not a poll and holds no cursor, so
	// it sees an event written long before Fleet ever polled this org — which
	// is why activation uses it and not the feed (task 14, fix batch b04,
	// S02-C4). ok false means the org's feed holds no such subject; that is
	// the caller's answer, not an error.
	OrgEvent(ctx context.Context, orgID, kind, subject string) (hubapi.Event, bool, error)
}

type Hub interface {
	Projects
	Teams
	Briefs
	Designs
	Releases
	Evidence
	Feed
	Team    // Phase T task 08
	OrgFeed // Phase T task 08
}
```

`Tasks` takes `hubapi.TaskQuery`, not the bare `PageRequest` task 11's part first proposed,
because route 52 has a `state` filter; a zero `TaskQuery` is the same call. Task 03 does not
define the three query types (`task-03.md` has none, checked 2026-09-25), so this task adds them,
in `hubapi`, beside task 03's `team.go`:

```go
// control-plane/internal/hub/hubapi/teamquery.go — complete (new)
package hubapi

import (
	"net/url"
	"strconv"
)

// TaskQuery is route 52's query: ?state= (repeatable, task-06.md) and the page.
type TaskQuery struct {
	State []TaskState
	PageRequest
}

// AgentQuery is route 55's: ?live=true omits ended agents.
type AgentQuery struct {
	Live bool
	PageRequest
}

// ConflictQuery is route 59's: ?open=true omits resolved and cleared ones.
type ConflictQuery struct {
	Open bool
	PageRequest
}

func (q TaskQuery) Values() url.Values {
	v := q.PageRequest.values()
	for _, s := range q.State {
		v.Add("state", string(s))
	}
	return v
}

func (q AgentQuery) Values() url.Values    { return flag(q.PageRequest.values(), "live", q.Live) }
func (q ConflictQuery) Values() url.Values { return flag(q.PageRequest.values(), "open", q.Open) }

func (p PageRequest) values() url.Values {
	v := url.Values{}
	if p.After != "" {
		v.Set("after", p.After)
	}
	if p.Limit != 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	return v
}

func flag(v url.Values, name string, on bool) url.Values {
	if on {
		v.Set(name, "true")
	}
	return v
}
```

`PageRequest{After, Limit}` is the built type (`hub` worktree `hubapi/types.go:201-205`). If
Hub task 33's transport already has a page-to-query helper, `values` is replaced by it. Its test,
in `hubapi/teamquery_test.go`: `TaskQuery{State: []TaskState{TaskActive, TaskBlocked}}.Values().Encode()`
is `state=active&state=blocked` (`task-03.md:134-140`), and `ConflictQuery{}.Values().Encode()` is `""` (mutants: `Set`
for `Add`, dropping the second state; `open=false` sent for a zero query, which the Hub's strict
parse would refuse). **This is the one change task 11's map needs**, plus
`ConflictQuery{State: "open"}` at `task-11.md:888` becoming `ConflictQuery{Open: true}`. `Teams`
(Hub task 04, plural, team membership) and `Team` (this task, team mode) differ by one letter; the
doc comment on each says which, and no file embeds one without the other's comment beside it.

```go
// control-plane/internal/hub/hubclient/context.go — complete
package hubclient

import "context"

type ctxKey int

const (
	keyDelegation ctxKey = iota
	keyIdempotency
)

// WithDelegation names the delegation a Fleet call runs under; the HTTP client sends
// it as X-Panacea-Delegation, and the fake reads it with DelegationFrom. A desktop
// client never sets it: the Hub refuses a desktop token carrying the header (task 05).
func WithDelegation(ctx context.Context, delegationID string) context.Context {
	return context.WithValue(ctx, keyDelegation, delegationID)
}

// WithIdempotencyKey replaces the client's fresh per-call key (hub-contract.md:1345)
// for one call. Fleet's outbox passes its item id, so a retry after a restart is a
// replay, not a second write (task-11.md:91).
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, keyIdempotency, key)
}

func DelegationFrom(ctx context.Context) string {
	s, _ := ctx.Value(keyDelegation).(string)
	return s
}

func IdempotencyKeyFrom(ctx context.Context) string {
	s, _ := ctx.Value(keyIdempotency).(string)
	return s
}
```

The HTTP client (Hub task 33's `do`, **UNVERIFIED until merge**) gains two lines where it builds a
request: `if dl := DelegationFrom(ctx); dl != "" { req.Header.Set(hubapi.HeaderDelegation, dl) }`
(`"X-Panacea-Delegation"`, `task-03.md:359-360`, the constant task 05's `Authenticate` reads),
and, for a `POST`, `IdempotencyKeyFrom(ctx)` in place of `ids.UUID()` when it is non-empty. A key
that fails the Hub's grammar (`^[A-Za-z0-9_-]{1,64}$`, `task-14.md:103`) is refused **locally**
with a `bad_request` `*Error` before any network call, so a malformed outbox id never costs a
round trip. The team methods in `client_team.go` are each one line over Hub task 34's generic
helpers (`get[T]`, `post[Req, Res]`, `patch[Req, Res]`, `del`), with the path built by
`url.PathEscape` on every id.

#### Route 78 — the wire

**`GET /v1/orgs/{orgId}/changes?after=<seq | latest>&kinds=<comma list>&limit=<1–500, default 200>`**

- **Caller**: Fleet's workload identity only, **with no delegation** — the first of §5.10 rule 2's
  **two** exempt routes since b02 (route 88 is the second; `task-05.md:37-42`). "Fleet's" means
  `c.AppRole == hubapi.AppRoleFleet`, **not merely `AppOnly`**: since b02 a second app-only
  identity exists, and task 06's `delegationHeader` arms refuse the broker's
  `Hub.DelegationReader` token 403 `forbidden`, `required: "fleet"`, before this route runs
  (b02 round 1, C2 — without that narrowing a pod-level component reads the whole org feed).
  A desktop token is 403 `forbidden`, `required: "fleet"`; Fleet naming a delegation is 400
  `bad_request`, `details.field = "X-Panacea-Delegation"`, because the feed is not an owner's act.
- **`orgId`** must be the org enrolled for Fleet's token `tid`; anything else is 404
  `route_not_found`, so Fleet cannot probe for other orgs.
- **`kinds`**: optional; each a kind of contract §8 (Hub or team), else 400 `field: kinds`.
- **Which events**: those with `fleet_feed = true` (task 04's flag; every `fleet` and `yes` team
  kind of task 01's §8) **in a project whose `mode` is `team`**, plus the org's
  `team.member_removed` events (Hub task 15) whatever the project, because a leaver is how a
  delegation dies (§2.4 item 4). Audit-only kinds never appear. **Solo projects never appear.**
- **Answer**: exactly contract §7.11's `{ "events", "next", "more" }`, the built
  `hubapi.ChangesPage` (`hubapi/types.go:547`), with the same `next = H` rule when nothing is
  visible (`task-26.md:32-34`), and the same gapless read of the head first.

**The member feed does not change.** Task 01's three values map onto two flags on the audit row:
`yes` is `feed = true, fleet_feed = true`; `fleet` is `feed = false, fleet_feed = true`; `no` is
both false. Hub task 26's member query (`feed = true`) therefore already excludes every `fleet`
kind, and route 78 reads `fleet_feed = true`. Task 04 sets both flags from the kind at append time
(**UNVERIFIED**: task 04's part is written in parallel; if it leaves the setting to Go, this task
adds the kind table of `task-01.md:487-508` to `store.Append`, and `TestMemberFeedHasNoFleetKinds`
below is what proves it either way). A `fleet` kind in the member feed would show every member
who escalated what, which §3.4 forbids.

#### The fake's team half — its surface, complete

```go
// control-plane/internal/hub/hubclient/fake_team.go (the exported surface)

// FakeFleet is Fleet's workload identity. It is never a user and is never
// provisioned (task-01.md:127-129); a view built by AsFleet acts as the owner of the
// delegation its context names, or, with none, may call OrgChanges and
// OrgEvent only — both are route 78, so both sit under the ONE exemption it
// already holds (§5.10 rule 7); b04 added no new one.
var FakeFleet = FakeUser{ID: "fleet", DisplayName: "Fleet"}

// FakeVera is a viewer of the fake's team, and FakeAudrey an org auditor in no team,
// so that conformance rows can run task 01's V and Au columns against the fake.
var (
	FakeVera   = FakeUser{ID: "u-00000000000000a4", DisplayName: "Vera Example", Email: "vera@example.test"}
	FakeAudrey = FakeUser{ID: "u-00000000000000a5", DisplayName: "Audrey Example", Email: "audrey@example.test", Auditor: true}
)

// FakeBroker is the sandbox broker's workload identity (fix batch b02, S02-C5).
// Like Fleet it is never a user and never provisioned, but it names no delegation
// and reaches route 47 alone: every other method answers 403 forbidden,
// required "fleet" (ADR-H1 §5.10 rule 10).
var FakeBroker = FakeUser{ID: "broker", DisplayName: "Broker"}

// AsFleet returns a view acting as Fleet.
func (f *Fake) AsFleet() *Fake { return &Fake{w: f.w, as: FakeFleet} }

// AsBroker returns a view acting as the broker (b02).
func (f *Fake) AsBroker() *Fake { return &Fake{w: f.w, as: FakeBroker} }

// SetProjectMode is Team's route 87 on the fake, with the real route's rules
// (fix batch b01, X-C1): Fleet's view → 403 forbidden, required "route"; not an
// admin → 403, required "admin"; a mode but "team" → 400, field "mode"; the gate
// shut → 503 audit_not_streaming, nothing written; already team → the project, no
// event; else one project.mode_set event with both feed flags set. It replaces the
// test-only setter the fake had, so no fake test reaches a team project by a path
// the real Hub does not have.
func (f *Fake) SetProjectMode(ctx context.Context, projectID string, mode hubapi.ProjectMode) (hubapi.Project, error)

// ShutTeamModeGate makes route 87 answer 503 on this fake, as a real Hub whose audit
// sink is not "logs" and whose dev flag is off (task 06, task 45). Open by default,
// as the dev Hub every other test runs against.
func (f *Fake) ShutTeamModeGate()

// TeamEvents is every team event the fake has recorded, in order, with its two feed
// flags, for tests that assert what reached which feed.
func (f *Fake) TeamEvents() []FakeEvent

// AppendEventForTest appends one event of kind on projectID with task 01's flags
// for that kind, through no route (b01 round 2, re-review I-5). Since route 46
// refuses a solo project, no route writes a fleet_feed event there; this is how
// team-feed/team-projects-only still reaches the mode filter. It never sets a mode.
func (f *Fake) AppendEventForTest(projectID, kind string) hubapi.Event

// FakeTeamProject builds a fake whose default view (FakeAda) creates a team, so
// is its admin (fake_projects.go:223-224), and a project on it, then turns the
// project team by SetProjectMode as Ada (route 87's real rule, not a setter;
// b01 round 2, re-review I-6). Any suite outside tasks 06 to 08 that creates a
// delegation on the fake starts here, or its CreateDelegation is 409 not_team_project.
func FakeTeamProject(t testing.TB) (f *Fake, projectID string)

type FakeEvent struct {
	hubapi.Event
	MemberFeed, FleetFeed bool
	Via, DelegationID     string // "" for a desktop write
}
```

`FakeUser` gains one field, `Auditor bool`, beside `OrgAdmin` (`fake.go:37-40`). `fakeWorld` gains
maps for each record, keyed by id, plus a delegation map and a per-org event log; every one is
guarded by the existing `mu`.

**What the fake keeps**, each a rule of task 06 or 07 that a caller can observe:

| Rule | Fake behaviour | Source |
|---|---|---|
| Fleet names a delegation | a Fleet view with no `WithDelegation` gets 401 `delegation_required` on every method but `OrgChanges`, `OrgEvent` and `Principal`; with one on any of those three, 400 | task 01 §5.10 rule 2, both exemptions (b02); `OrgEvent` is route 78's own, not a third (b04) |
| the principal read is the project's | `Principal` answers the **project's** `orgId`, its role for that user, and the three flags as held in that org: a user with `Auditor` in another org gets three false flags and `role` `""` | contract §7.28 rules 1 to 3 (b02) |
| the broker reads one route | a broker view reaches `Delegation` only, on any delegation of its org whatever its state, and gets `OwnerTID`/`OwnerOID`; every other method → 403 `required: "fleet"` | §5.10 rule 10 (b02) |
| a live delegation | revoked → 401 `delegation_revoked`; past `expiresAt` → 401 `delegation_expired`; unknown → 401 `delegation_revoked`, `reason: unknown` | `task-01.md:118-124`, `:207-210` |
| the route is listed | a method whose route name the delegation lacks → 403 `forbidden` | rule 2 |
| a desktop never names one | a desktop view with `WithDelegation` → 403 `required: "fleet"` | rule 2 |
| the owner is the delegation's | every record's `ownerUserId` is the delegation owner; any body field naming one does not exist in the Go types | rule 3 |
| `ag-…` owner check | as task 06's `OwnedAgent` / `EitherAgent`, per route | rule 5 |
| class route sets | `CreateDelegation` refuses a route outside the class set, 422 | `task-01.md:274-283` |
| state machines | task, agent end, thread, conflict, lease `seq` and terminal states: the same 409s | tasks 06, 07 |
| the taint rule | `AppendDirective` applies `TaintAllows`' table, over the fake's escalations | task 07 |
| private reads | escalations, directives, human messages filtered to the caller's own; everyone's for `FakeAudrey`; thread messages to a party's owner through Fleet·O and to the auditor | task 07 |
| title redaction | `Tasks` returns `title` and `reason.why` only to a Fleet·O view | task 06 |
| bounds and hygiene | `teamtext.Bound` and `Clean` on every text field, `ValidGlob` on every glob, the same 422 codes | task 02 |
| idempotency | a repeated id with an equal body replays; with a different body, 409 `conflict` | `task-01.md:185-187` |
| the org feed | every team write appends an event with task 01's flags; `OrgChanges` serves `fleet_feed` events of `team` projects and `team.member_removed`; the cursor and `latest` rules as route 78 | route 78 |
| the org feed, by subject (b04) | `OrgEvent(org, kind, subject)` is route 78 with `subject=`, `after` empty, `limit=1`: it answers that one event **whenever it was written**, including before Fleet first polled the org, and `ok=false` with 200 for an unknown subject; `subject=` with no `kinds`, or with `after=latest`, is 400 | route 78 |

The fake does **not** keep what needs Postgres: RLS across orgs (the fake has one org), the
append-only triggers (the fake has no update method to call), the `CHECK` bounds (Go's are the
same numbers, and task 04's database tests hold the other layer), or the hash chain.

**`RemoveMember` in the fake revokes that member's delegations** for the team's projects and
records `team.member_removed`, because that is what task 44 makes the real Hub do and what
task 11's revocation test needs. Until task 44 merges, the conformance row for it runs against the
fake only and the real-Hub run skips it with `t.Skip("task 44")`. This is the only row that does.

#### The conformance groups

```go
// control-plane/internal/hub/hubclient/conformance/conformance.go (the additions)

const (
	GroupTeam     Group = "team"      // Phase T tasks 06, 07
	GroupTeamFeed Group = "team-feed" // Phase T task 08
)

// Env gains, for the two team groups (zero for every other group):
//	ProjectID       string            // a project of T made team by Admin.SetProjectMode (route 87)
//	SoloProjectID   string            // a project of T whose mode is solo
//	SoloRepoKey     string            // SoloProjectID's one remote, for route 13's read-back (b01 r2)
//	OrgID           string
//	Fleet           hubclient.Hub     // Fleet's workload identity; its delegation comes from ctx
//	Auditor         hubclient.Hub     // an org auditor in no team
//	Broker          hubclient.Hub     // the sandbox broker (app-only, Hub.DelegationReader,
//	                                  // §5.10 rule 10): route 47 and nothing else (b02, S02-C5)
//	OtherOrgProjectID string          // a project of a SECOND org, and OtherOrgAuditor, a user
//	OtherOrgAuditor func() hubclient.Hub // with Hub.Auditor there and nothing here. The second
//	                                  // org is what witnesses the NEGATIVE half — a flag never
//	                                  // crossing an org (S02-I6) — and route 88's other-org 404.
//	                                  // The positive half is Env.Auditor, whose standing is in
//	                                  // ProjectID's own org, because Fleet cannot ask about a
//	                                  // project outside its own (b02, S02-C2; round 1, I2)
//	RemoveMember    func(userID string) // Admin removing a member of T, as Teams.RemoveMember
//	AppendEvent     func(projectID, kind string) // test-only: one audit event of kind on
//	                                  // projectID, flags from the kind, no route (b01 r2, I-5).
//	                                  // Real Hub: Hub task 14's store.Append(ctx, tx, head, row)
//	                                  // (hub task-14.md:174) in one transaction that locks the
//	                                  // org's audit head as Tx does, so the chain and task 04's
//	                                  // flag trigger hold; UNVERIFIED until merge. Fake:
//	                                  // Fake.AppendEventForTest. It never changes a project's mode.
```

`AllGroups` appends `GroupTeam, GroupTeamFeed`, last, because both need a project. The real-Hub
`newEnv` (Hub task 34's integration harness) builds Fleet's view with task 05's client-credentials
token from `devissuer`; the fake's is `NewFake().AsFleet()`. Both `newEnv`s make `ProjectID` a
team project by `Admin.SetProjectMode(ctx, id, hubapi.ProjectModeTeam)`, never by SQL or a fake
setter (fix batch b01, X-C1); the real one runs its Hub with
`PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true` **and** `PANACEA_APP_HUB_AUDIT_SINK=fake` in `local`
(task 06's `teamWorld` rule; task 45's gate opens on the flag only beside sink `fake`, b01
round 2, re-review I-4).

- [ ] **Step 1: Write the failing conformance rows**

Each row is `t.Run("team/<name>")` or `t.Run("team-feed/<name>")` with a fresh `Env`, and each
names the mutant of **the fake** it kills, because the fake is the implementation most likely to
drift silently (the real Hub has tasks 06 and 07's own tests).

| Row | Asserts | Fake mutant it kills |
|---|---|---|
| `team/fleet-needs-a-delegation` | `Fleet.CreateTask` with no `WithDelegation` → `Code(err) == "delegation_required"` | the fake treats Fleet as a user |
| `team/revoked-and-expired` | `RevokeDelegation`, then a Fleet write → `delegation_revoked`; a fresh one past `Advance(25h)` → `delegation_expired` (skipped on the real Hub: no clock) | one code for both |
| `team/unknown-delegation-is-revoked-not-404` | a made-up `dl-…` → `delegation_revoked`, `details.reason = "unknown"` | the fake answers `delegation_not_found` and lets Fleet probe |
| `team/desktop-cannot-name-a-delegation` | `Member.Tasks` under `WithDelegation` → 403, `required: "fleet"` | the header ignored on desktop views |
| `team/route-must-be-listed` | a delegation of `["agents.start"]`, then `CreateTask` → 403 `forbidden` | the fake checks class, not routes |
| `team/worker-class-cannot-carry-the-channel` | `CreateDelegation` class `worker` with `messages.append` → 422, `details.route` | the fake has one route set |
| `team/owner-is-the-delegations` | a task created under Member's delegation has `ownerUserId` = Member | the fake stamps the view's user (Fleet) |
| `team/agent-not-owner` | Fleet under Member's delegation, `orchestratorAgentId` of Admin's → `agent_not_owner` | the ownership check missing from the fake |
| `team/admin-cannot-patch-anothers-task` | under Admin's delegation, `PatchTask` of Member's → `agent_not_owner` | admin exemption in the fake |
| `team/task-transitions` | `done → active` → 409 `conflict`; `planned → active` → 200; stale `revision` → `revision_conflict` | transitions unchecked |
| `team/lease-seq-gapless` | seq 1, then 3 → 409 `conflict`; after `released`, any → 409 | `seq` stored, never compared |
| `team/conflict-agentA-is-owners` | `agentA` Admin's under Member's → `agent_not_owner`; `PatchConflict` under Admin's when `agentB` is Admin's → 200 | `EitherAgent` missing from the fake |
| `team/title-for-fleet-orchestrator-only` | `Member.Tasks` and Fleet under a `worker` delegation see no `title`; Fleet under an `orchestrator` one does | redaction missing, so every Fleet test reads titles a worker never would |
| `team/text-bounds` | 141-rune title → 422 `text_refused`; 21 globs → 422 `breadth_exceeded`; `../x` → 422 `glob_refused` | the fake skips `teamtext` |
| `team/idempotent-ids` | `CreateTask` twice with one id and one body under one `WithIdempotencyKey` → equal answers; a different body → 409 `conflict` | the fake mints its own ids |
| `team/participants-two-owners` | `CreateThread` with two Orchestrators of Member → 422 | participants unchecked |
| `team/messages-by-parties` | Admin's thread with Member's: Fleet under a third member's delegation `Messages` → 403 `party`; `Auditor.Messages` → 200 | the thread read open to any Fleet·O |
| `team/private-reads` | Member's escalation absent from `Admin.Escalations`, from Fleet under Admin's delegation, and present for `Auditor` | the filter missing from one of the four private reads |
| `team/answer-by-owner-desktop-only` | `Admin.AnswerEscalation` of Member's → 403 `owner`; Fleet → 403 `desktop`; Member → 200; again → 409 | Fleet may answer, so an Orchestrator answers its own question |
| `team/taint` | Fleet, `origin: fleet`, `kind: free`, no `approvedBy` → 403 `decide`; with Member's answered decide → 201; with Admin's → 403 | the fake skips `TaintAllows` |
| `team/human-message-to-own-orchestrator` | `Member.WriteHumanMessage` to Admin's Orchestrator → `agent_not_owner`; `Fleet.WriteHumanMessage` → 403 `desktop` | Fleet can write a human message |
| `team/turn-has-no-row` | `RecordTurn` twice with one id and key → equal; the event is in neither feed | turn events on the feed |
| `team/mode-is-the-admins-one-way` (b01) | on `SoloProjectID`: `Member.SetProjectMode` → 403 `required: "admin"`; `Fleet.SetProjectMode` → 403 `required: "route"`; `Admin`, `solo` → 400 `details.field = "mode"`; `Admin`, `team` → 200 `Mode == team`; again → 200, and one `project.mode_set` in `Admin.Changes`; then read back (b01 round 2, re-review I-1): `Member.Project(SoloProjectID).Mode == team` (route 15) and `Member.ProjectForRepo(SoloRepoKey)` → `ok`, `Mode == team` (route 13), and before the call both were `solo`, never `""` | the fake's route 87 skips the role or the one-way check; on the real Hub, the project-row reader of routes 13 and 15 not selecting `mode` (task 06) |
| `team/mode-waits-for-the-audit-gate` (b01) | `ShutTeamModeGate()`; `Admin.SetProjectMode(SoloProjectID, team)` → 503, `details.reason = "audit_not_streaming"`; `CreateDelegation` on it → 409 `not_team_project` (skipped on the real Hub: its gate is start-up configuration; task 06's `TestProjectModeWaitsForTheAuditGate` covers it) | the fake writes the mode with the gate shut |
| `team/delegation-needs-a-team-project` (b01) | `Member.CreateDelegation` on `SoloProjectID` → 409 `conflict`, `details.reason = "not_team_project"` | the fake delegates on any project, so no fake test needs route 87 |
| `team/principal-org-is-the-projects` (b02) | `Fleet.Principal(Member.TID, Member.OID, ProjectID)` with no `WithDelegation` → 200, `OrgID == ProjectOrgID`, `Role == "member"`; the same for `Admin` → `Role == "admin"`; a provisioned user of the org who is on no team of `ProjectID` → 200, `Role == ""`, three false flags | the fake answers the **caller's** org, which is review S02-I6's hole: an org-A auditor then passes on an org-B project |
| `team/principal-flags-are-the-projects-org` (b02; second clause rewritten, b02 round 1, I2) | both halves asked by `Fleet` about `ProjectID`, because another org's project is 404 `route_not_found` for Fleet (§7.28 rule 4) and no Fleet identity exists in the second org: `Fleet.Principal(OtherOrgAuditor().TID, OtherOrgAuditor().OID, ProjectID)` → 200, `Role == ""`, `Auditor == false` and all three flags false, though that user holds `Hub.Auditor` in their own org; `Fleet.Principal(Auditor.TID, Auditor.OID, ProjectID)` → 200, `Role == ""`, `Auditor == true`, the same standing held **in the project's own org** | the flag read off the user, not off the (user, org) pair — the negative half alone passes against a Hub that answers every flag false |
| `team/principal-needs-no-delegation` (b02) | `Fleet.Principal` under `WithDelegation` → 400; `Member.Principal` → 403 `required: "fleet"`; an unprovisioned `(tid, oid)` → 404 `principal_not_found`; a made-up `pj-…`, and a real one of another org → 404 `route_not_found` in both cases, the same code and the same details | the fake answers `principal_not_found` for an unknown project, which tells Fleet whether a project exists |
| `team/broker-reads-any-delegation` (b02) | `Broker.Delegation(dl)` for Member's, Admin's, a revoked one and an expired one → 200 each, `OwnerTID`/`OwnerOID` set and equal to that owner's; `Member.Delegation(own)` and `Fleet.Delegation(own)` → 200 with both fields empty | the broker refused a revoked delegation, so task 20's cleanup cannot find the pods to delete; or the two fields served to Fleet |
| `team/broker-reaches-one-route` (b02) | every other `Team` method on the broker view → 403 `forbidden`, `required: "fleet"`, `Broker.OrgChanges` included; `Broker.Delegation` of another **org's** delegation → 404 `route_not_found` | the broker admitted as a second Fleet, which would give a pod-level component the whole team API |
| `team-feed/fleet-only-no-delegation` | `Fleet.OrgChanges` → 200; under `WithDelegation` → 400; `Member.OrgChanges` → 403 `fleet` | the delegation exemption applied to every route |
| `team-feed/other-org-is-404` | `Fleet.OrgChanges` on a made-up `org-…` → 404 `route_not_found` | org taken from the path |
| `team-feed/by-subject-reads-the-past` (b04) | write an event, poll to `next` so the cursor is past it, then `Fleet.OrgEvent(org, kind, subject)` → that event, `ok` true; an unknown subject → `ok` false and no error; `Fleet.OrgEvent` on another org's subject → `ok` false | a by-subject read implemented on top of a cursor, which would answer nothing for exactly the events activation needs (S02-C4) |
| `team-feed/team-projects-only` (restated, b01 round 2, re-review I-5) | poll to `next`; a task in `ProjectID` through `Fleet.CreateTask`; then `AppendEvent(SoloProjectID, "task.created")`, because route 46 now refuses a delegation there so no route can write it; the next `Fleet.OrgChanges` holds the first `task.created` and not the second | the mode filter missing (the clause stays as defence in depth: a future `fleet_feed` kind on a solo project must not reach Fleet) |
| `team-feed/fleet-kinds-not-in-member-feed` | after an escalation, `Fleet.OrgChanges` has `escalation.created`; `Member.Changes` does not; `task.created` is in both | the flags swapped |
| `team-feed/revocation-within-one-poll` | poll to `next`; `RevokeDelegation`; the next `OrgChanges(after=next)` holds `delegation.revoked` with `subject` = the id | the revocation written to the member feed only (the scope's test) |
| `team-feed/member-removed-revokes` | `RemoveMember(Member)`; the next poll holds `team.member_removed` and `delegation.revoked`, `data.reason = "member_removed"`; Fleet's next write under that delegation → `delegation_revoked` (fake only until task 44) | removal leaves the delegation live |
| `team-feed/mode-set-tells-fleet` (b01) | poll to `next`; `Admin.SetProjectMode(SoloProjectID, team)`; the next `Fleet.OrgChanges` holds `project.mode_set` with `subject` = that id, `data = {"from":"solo","to":"team"}` | the event written with `FleetFeed` false, so Fleet never creates the team project (`05-design.md` §10.1 step 2) |
| `team-feed/latest-and-cursor` | `after=latest` → no events, `next` = head; a poll with nothing visible still advances `next` | Hub task 26's cursor rule not carried to route 78 |

One row in full, the one the scope paragraph names:

```go
// control-plane/internal/hub/hubclient/conformance/team.go (excerpt)

// team-feed/revocation-within-one-poll. Fake mutant killed: RevokeDelegation records its
// event with FleetFeed false, so Fleet (task 11) never learns and keeps the sessions alive.
func rowRevocationWithinOnePoll(t *testing.T, e Env) {
	ctx := context.Background()
	dl, err := e.Member.CreateDelegation(ctx, hubapi.CreateDelegationRequest{
		ProjectID: e.ProjectID, Class: hubapi.DelegationClassOrchestrator,
		Routes: []hubapi.RouteName{hubapi.RouteTasksCreate}}) // task-03.md:495-499
	if err != nil {
		t.Fatal(err)
	}
	head, err := e.Fleet.OrgChanges(ctx, e.OrgID, "latest", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Member.RevokeDelegation(ctx, dl.ID); err != nil {
		t.Fatal(err)
	}
	page, err := e.Fleet.OrgChanges(ctx, e.OrgID, head.Next, []string{"delegation.revoked"}, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range page.Events {
		if ev.Kind == "delegation.revoked" && ev.Subject == dl.ID {
			return
		}
	}
	t.Fatalf("delegation.revoked for %s not in the next poll: %+v", dl.ID, page.Events)
}
```

- [ ] **Step 2: Write the failing server tests for route 78**

`orgchanges_integration_test.go`, on Hub task 06's harness, beside Hub task 26's tests and in
their shape:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestOrgFeedIsFleetsAlone` | Fleet, no header → 200; Fleet with a header → 400; Linus's desktop → 403 `fleet` | task 05's exemption keyed on the path prefix, so `/v1/orgs/{o}/…` is exempt too |
| `TestOrgFeedOrgMustBeTokens` | Fleet on Xavier's org → 404 `route_not_found` | the org read from the path under RLS bypass |
| `TestOrgFeedTeamProjectsOnly` | writes in a team and a solo project; only the team's appear | the `mode` join dropped |
| `TestOrgFeedCarriesMemberRemoved` | Grace removes Linus from T; `team.member_removed` appears | the filter on `project_id` drops team-level events, which have none |
| `TestMemberFeedHasNoFleetKinds` | after one of each task 06–07 write, Linus's `GET /v1/changes` holds only `yes` kinds | a `fleet` kind written `feed = true` |
| `TestOrgFeedKindsFilter` | `kinds=delegation.revoked` → only those; `kinds=nope.nope` → 400 `field: kinds` | the filter ignored, or unknown kinds silently matched nothing |
| `TestOrgFeedGapless` | Hub task 26's 20-writer test, over route 78 | the head not read first |

- [ ] **Step 3: Run everything to verify it fails**

```bash
cd control-plane && go test ./internal/hub/hubclient/... -count=1
cd control-plane && go test -tags integration ./internal/hub/server/ -run 'TestOrgFeed|TestMemberFeed' -count=1
```

Expected: the first fails to build (`undefined: hubclient.Team`, `undefined: conformance.GroupTeam`);
the second answers 404 `route_not_found` for every route 78 call.

- [ ] **Step 4: Write the interfaces, the context helpers, the HTTP methods, the fake, route 78**

In that order, so each compiles before the next: `hubclient.go` and `context.go` (above,
complete); `client_team.go` (one line per method over Hub task 34's helpers); the fake, record by
record, running the `team/` rows after each record lands; then `store.OrgChanges` and
`server/orgchanges.go`. The store query is Hub task 26's with two changes: its `feed = true` and
visibility clause together replaced by, over `audit_events a`,

```sql
((a.fleet_feed = true AND EXISTS (SELECT 1 FROM projects p
                                  WHERE p.id = a.project_id AND p.org_id = a.org_id
                                    AND p.mode = 'team'))
 OR a.kind = 'team.member_removed')
```

— the mode lives on `projects` (task 04's column), never on `audit_events`, which carries only
`project_id` (`hub task-07a.md:412-428`), so the filter is a join and not a column reference
(b01 round 3, re-review I-B). Bracketed so, because task 04's trigger leaves `fleet_feed = false` on
`team.member_removed` (`task-04.md:259-260`: "by kind, not by this flag"), so ANDing the flag
with the kind clause would hide every removal from Fleet (b01 round 2, re-review's passing note);
and `kind = ANY($kinds)` when `kinds` is set.

- [ ] **Step 5: Run everything to verify it passes**

```bash
cd control-plane && go test ./internal/hub/hubclient/... -count=1 -run 'TestFakeConforms/team' -v
cd control-plane && go test ./... -count=1
cd control-plane && go test -tags integration ./internal/hub/... -count=1
```

Expected: every `team/` and `team-feed/` sub-test `--- PASS` against the fake, then against the
real Hub under Hub task 34's integration conformance test, with exactly two `--- SKIP` on the
real Hub (`team-feed/member-removed-revokes`: `task 44`; `team/mode-waits-for-the-audit-gate`:
`start-up gate`, b01) and one on the fake (the first), and, on the real Hub only,
`team/revoked-and-expired` skipped for its expiry half (no clock). No line says `FAIL`.
`client_team.go`'s `SetProjectMode` is `POST /v1/projects/{p}/mode` with
`hubapi.SetProjectModeRequest`, decoding `hubapi.Project` (b01).

- [ ] **Step 6: Vet and format**

```bash
gofmt -l control-plane/internal/hub
cd control-plane && go vet ./...
```

Both print nothing.

- [ ] **Step 7: Commit**

`.superpowers/sdd/team/msg/t08.txt` says what changed and why (the fake is Fleet's test double and
now keeps every observable team rule; the per-org feed is Fleet's alone and team projects only),
ending with `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`:

```bash
git add control-plane/internal/hub/hubclient/{context,client_team,fake_team,fake_team_protocol}.go \
        control-plane/internal/hub/hubclient/conformance/team.go \
        control-plane/internal/hub/server/orgchanges*.go
git commit -F .superpowers/sdd/team/msg/t08.txt -- control-plane/internal/hub
```

#### Acceptance

1. Step 5's three commands pass as described; the conformance run lists 39 team rows (29, plus fix
   batch b01's four, plus fix batch b02's five, plus fix batch b04's one —
   `team-feed/by-subject-reads-the-past`). That is this table's count at **this task's** step:
   later tasks add their own rows to the same suite (task 14 adds two, `task-14.md:970-971`), so
   a run after them lists more, and that is not drift.
2. `grep -c '^	[A-Z][A-Za-z]*(ctx' control-plane/internal/hub/hubclient/hubclient.go` rises by exactly
   29 (27 `Team` methods — `Principal` is b02's — plus `OrgChanges` and `OrgEvent`, b04's) over
   the merged Hub's count.
3. `var _ hubclient.Hub = (*hubclient.Fake)(nil)` and `var _ hubclient.Hub = (*hubclient.Client)(nil)`
   compile. `Client` is the built HTTP type (`client.go:17`, a stub until Hub task 33); neither
   assertion exists on `feat/hub` as read on 2026-09-25, so this task adds both to
   `hubclient_test.go`.
4. `team-feed/revocation-within-one-poll` passes on both implementations: the scope's "a revoked
   delegation appears on the feed within one poll".
5. (b02, S02-C2) `team/principal-org-is-the-projects` passes on both implementations, and
   `Env` carries a `Broker` view and a second org whose user and project the conformance rows name
   — without the second org, `team/principal-flags-are-the-projects-org` has no cross-org half and
   `team/principal-needs-no-delegation` no other-org project to be answered 404 about. `hubclient.Team.Principal` exists, so
   task 12's `hubMembership` compiles against the real interface rather than a proposal.

#### What this task does not claim

- **Not delivery.** Nothing here sends a Fleet write; task 11's outbox does.
- **Not the fake's Postgres half.** RLS across orgs, the append-only triggers, the `CHECK` bounds
  and the chain are the real Hub's alone (task 04's database tests).
- **Not member removal revoking delegations on the real Hub.** The fake does it; the real Hub does
  it after task 44, and the row skips there until then.
- **Not that Fleet polls within 60 seconds.** The feed answers within one poll; Fleet's 10-second
  poll and its 60-second revocation budget are task 11's.
- **Not that it runs as built.** Hub tasks 05, 26, 33 and 34 are planned; **UNVERIFIED until merge**.

#### Gaps found in the skeleton

1. **Who sets a project's `mode`. Resolved by fix batch b01 (review X-C1):** task 06's route 87
   is the one writer; `Team.SetProjectMode` calls it, and the fake's `SetProjectMode` keeps its
   rules instead of setting the mode directly, so the fake no longer lets every test skip the
   step the real Hub needs (`task-08.md` had a direct setter before b01).
2. **Thread listing.** As task 07's gap 2: `Team` has `Messages(threadID)` but no `Threads`, so
   task 11's rebuild cannot list open threads from the Hub.
3. **`Tasks` takes a query, not a page.** Task 11's part proposes `Tasks(ctx, projectID,
   hubapi.PageRequest)`; this part uses `hubapi.TaskQuery`, which embeds it. Task 11's call site
   changes by one composite literal.
4. **The three query types had no owner.** Neither the skeleton's scope for 03 nor task 03's part
   defines `TaskQuery`, `AgentQuery` or `ConflictQuery`, yet task 11 already calls them; this part
   adds `hubapi/teamquery.go`. Task 11's `ConflictQuery{State: "open"}` (`task-11.md:888`) must
   read `ConflictQuery{Open: true}`, matching route 59's `?open=true` (task 06).
