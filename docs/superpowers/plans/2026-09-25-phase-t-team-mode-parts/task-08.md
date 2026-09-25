### Task 08: Hub feed kinds, Fleet's per-org subscription; `hubclient` team methods, fake, conformance

**Slice TM1-α. Depends on: 07, H26, H33, H34. About 3 hours. Seam: ST.2.**

Three pieces, one purpose: after this task, Fleet's tests (tasks 11, 22 to 31) run against an
in-memory Hub that keeps every rule of routes 46 to 71, and the same rows prove the real Hub keeps
them too.

1. **Route 78, `GET /v1/orgs/{orgId}/changes?after=&kinds=&limit=`**: Fleet's per-org feed (task
   01 §7.26, `task-01.md:464-468`), beside the member feed Hub task 26 builds.
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
- `control-plane/internal/hub/hubclient/hubclient.go`: `Team`, `OrgFeed`, `Hub` (modify)
- `control-plane/internal/hub/hubclient/context.go`: `WithDelegation`, `WithIdempotencyKey`,
  `DelegationFrom`, `IdempotencyKeyFrom` (new)
- `control-plane/internal/hub/hubclient/client_team.go`: the HTTP methods (new)
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
- Consumes: task 01's route 78 and §8 feed column (`task-01.md:464-504`); task 03's types (the
  same list tasks 06 and 07 consume, plus `TaskQuery`, `AgentQuery`, `ConflictQuery`); tasks 06 and
  07's handlers; task 04's `fleet_feed` flag and a project's `mode`; task 05's exempt route and
  Fleet principal; Hub task 26's `store.Changes`; Hub task 33's transport (`c.do`).
- Produces: the interfaces below, complete; `hubclient.NewFake().AsFleet()`; the conformance
  groups `GroupTeam` and `GroupTeamFeed`.
- Required by: task 11 (every call it makes, `task-11.md:86`), task 12 (`/readyz` calls
  `OrgChanges` with `after=latest`), tasks 20 to 31 (their tests use the fake), task 21 and
  task 52 (the desktop's delegation, escalation-answer and human-message calls).

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
	// Desktop: routes 46–49.
	CreateDelegation(ctx context.Context, req hubapi.CreateDelegationRequest) (hubapi.Delegation, error)
	Delegation(ctx context.Context, delegationID string) (hubapi.Delegation, error)
	RenewDelegation(ctx context.Context, delegationID string) (hubapi.Delegation, error)
	RevokeDelegation(ctx context.Context, delegationID string) error

	// Fleet writes: routes 50, 51, 53, 54, 56, 57, 58.
	CreateTask(ctx context.Context, projectID, _ string, req hubapi.CreateTaskRequest) (hubapi.Task, error)
	PatchTask(ctx context.Context, projectID, taskID string, req hubapi.PatchTaskRequest) (hubapi.Task, error)
	StartAgent(ctx context.Context, projectID, _ string, req hubapi.StartAgentRequest) (hubapi.Agent, error)
	EndAgent(ctx context.Context, projectID, agentID string, req hubapi.EndAgentRequest) (hubapi.Agent, error)
	AppendLeaseEvent(ctx context.Context, projectID, leaseID string, req hubapi.LeaseEventRequest) (hubapi.LeaseEventResult, error)
	CreateConflict(ctx context.Context, projectID, _ string, req hubapi.CreateConflictRequest) (hubapi.Conflict, error)
	PatchConflict(ctx context.Context, projectID, conflictID string, req hubapi.PatchConflictRequest) (hubapi.Conflict, error)

	// Reads: routes 52, 55, 59.
	Tasks(ctx context.Context, projectID string, q hubapi.TaskQuery) (hubapi.Page[hubapi.Task], error)
	Agents(ctx context.Context, projectID string, q hubapi.AgentQuery) (hubapi.Page[hubapi.Agent], error)
	Conflicts(ctx context.Context, projectID string, q hubapi.ConflictQuery) (hubapi.Page[hubapi.Conflict], error)

	// Fleet writes: routes 60–62, 64, 67, 69.
	CreateThread(ctx context.Context, projectID, _ string, req hubapi.OpenThreadRequest) (hubapi.Thread, error)
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
	WriteHumanMessage(ctx context.Context, projectID string, req hubapi.WriteHumanMessageRequest) (hubapi.HumanMessage, error)
}

// OrgFeed is route 78: Fleet's per-org twin of Feed.Changes, over team projects
// only. after is a decimal seq or "latest"; kinds nil means every kind.
type OrgFeed interface {
	OrgChanges(ctx context.Context, orgID, after string, kinds []string, limit int) (hubapi.ChangesPage, error)
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

`Tasks` takes `hubapi.TaskQuery{State []TaskState; hubapi.PageRequest}`, not the bare
`PageRequest` task 11's part proposes (`task-11.md:86`), because route 52 has a `state` filter;
a zero `TaskQuery` is the same call. **This is the one change task 11's map needs.** `Teams`
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
request: `if dl := DelegationFrom(ctx); dl != "" { req.Header.Set("X-Panacea-Delegation", dl) }`,
and, for a `POST`, `IdempotencyKeyFrom(ctx)` in place of `ids.UUID()` when it is non-empty. A key
that fails the Hub's grammar (`^[A-Za-z0-9_-]{1,64}$`, `task-14.md:103`) is refused **locally**
with a `bad_request` `*Error` before any network call, so a malformed outbox id never costs a
round trip. The team methods in `client_team.go` are each one line over Hub task 34's generic
helpers (`get[T]`, `post[Req, Res]`, `patch[Req, Res]`, `del`), with the path built by
`url.PathEscape` on every id.

#### Route 78 — the wire

**`GET /v1/orgs/{orgId}/changes?after=<seq | latest>&kinds=<comma list>&limit=<1–500, default 200>`**

- **Caller**: Fleet's workload identity only, **with no delegation** (task 05's one exempt route).
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
adds the kind table of `task-01.md:478-499` to `store.Append`, and `TestMemberFeedHasNoFleetKinds`
below is what proves it either way). A `fleet` kind in the member feed would show every member
who escalated what, which §3.4 forbids.
