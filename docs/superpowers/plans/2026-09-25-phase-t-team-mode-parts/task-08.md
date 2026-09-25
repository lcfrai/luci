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

#### The fake's team half — its surface, complete

```go
// control-plane/internal/hub/hubclient/fake_team.go (the exported surface)

// FakeFleet is Fleet's workload identity. It is never a user and is never
// provisioned (task-01.md:122-124); a view built by AsFleet acts as the owner of the
// delegation its context names, or, with none, may call OrgChanges only.
var FakeFleet = FakeUser{ID: "fleet", DisplayName: "Fleet"}

// FakeVera is a viewer of the fake's team, and FakeAudrey an org auditor in no team,
// so that conformance rows can run task 01's V and Au columns against the fake.
var (
	FakeVera   = FakeUser{ID: "u-00000000000000a4", DisplayName: "Vera Example", Email: "vera@example.test"}
	FakeAudrey = FakeUser{ID: "u-00000000000000a5", DisplayName: "Audrey Example", Email: "audrey@example.test", Auditor: true}
)

// AsFleet returns a view acting as Fleet.
func (f *Fake) AsFleet() *Fake { return &Fake{w: f.w, as: FakeFleet} }

// SetProjectMode sets a fake project's mode ("solo" or "team"). The real Hub's
// Share route is not in Phase T's task list yet (task-06.md, "Gaps" 3); tests set
// the mode directly.
func (f *Fake) SetProjectMode(projectID, mode string) error

// TeamEvents is every team event the fake has recorded, in order, with its two feed
// flags, for tests that assert what reached which feed.
func (f *Fake) TeamEvents() []FakeEvent

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
| Fleet names a delegation | a Fleet view with no `WithDelegation` gets 401 `delegation_required` on every method but `OrgChanges`; with one on `OrgChanges`, 400 | task 01 §5.10 rule 2 |
| a live delegation | revoked → 401 `delegation_revoked`; past `expiresAt` → 401 `delegation_expired`; unknown → 401 `delegation_revoked`, `reason: unknown` | `task-01.md:113-119`, `:202-205` |
| the route is listed | a method whose route name the delegation lacks → 403 `forbidden` | rule 2 |
| a desktop never names one | a desktop view with `WithDelegation` → 403 `required: "fleet"` | rule 2 |
| the owner is the delegation's | every record's `ownerUserId` is the delegation owner; any body field naming one does not exist in the Go types | rule 3 |
| `ag-…` owner check | as task 06's `OwnedAgent` / `EitherAgent`, per route | rule 5 |
| class route sets | `CreateDelegation` refuses a route outside the class set, 422 | `task-01.md:268-277` |
| state machines | task, agent end, thread, conflict, lease `seq` and terminal states: the same 409s | tasks 06, 07 |
| the taint rule | `AppendDirective` applies `TaintAllows`' table, over the fake's escalations | task 07 |
| private reads | escalations, directives, human messages filtered to the caller's own; everyone's for `FakeAudrey`; thread messages to a party's owner through Fleet·O and to the auditor | task 07 |
| title redaction | `Tasks` returns `title` and `reason.why` only to a Fleet·O view | task 06 |
| bounds and hygiene | `teamtext.Bound` and `Clean` on every text field, `ValidGlob` on every glob, the same 422 codes | task 02 |
| idempotency | a repeated id with an equal body replays; with a different body, 409 `conflict` | `task-01.md:180-182` |
| the org feed | every team write appends an event with task 01's flags; `OrgChanges` serves `fleet_feed` events of `team` projects and `team.member_removed`; the cursor and `latest` rules as route 78 | route 78 |

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
//	ProjectID       string            // a project of T whose mode is team
//	SoloProjectID   string            // a project of T whose mode is solo
//	OrgID           string
//	Fleet           hubclient.Hub     // Fleet's workload identity; its delegation comes from ctx
//	Auditor         hubclient.Hub     // an org auditor in no team
//	RemoveMember    func(userID string) // Admin removing a member of T, as Teams.RemoveMember
```

`AllGroups` appends `GroupTeam, GroupTeamFeed`, last, because both need a project. The real-Hub
`newEnv` (Hub task 34's integration harness) builds Fleet's view with task 05's client-credentials
token from `devissuer`; the fake's is `NewFake().AsFleet()`.

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
| `team-feed/fleet-only-no-delegation` | `Fleet.OrgChanges` → 200; under `WithDelegation` → 400; `Member.OrgChanges` → 403 `fleet` | the delegation exemption applied to every route |
| `team-feed/other-org-is-404` | `Fleet.OrgChanges` on a made-up `org-…` → 404 `route_not_found` | org taken from the path |
| `team-feed/team-projects-only` | a task in `ProjectID` appears; the same writes in `SoloProjectID` do not | the mode filter missing |
| `team-feed/fleet-kinds-not-in-member-feed` | after an escalation, `Fleet.OrgChanges` has `escalation.created`; `Member.Changes` does not; `task.created` is in both | the flags swapped |
| `team-feed/revocation-within-one-poll` | poll to `next`; `RevokeDelegation`; the next `OrgChanges(after=next)` holds `delegation.revoked` with `subject` = the id | the revocation written to the member feed only (the scope's test) |
| `team-feed/member-removed-revokes` | `RemoveMember(Member)`; the next poll holds `team.member_removed` and `delegation.revoked`, `data.reason = "member_removed"`; Fleet's next write under that delegation → `delegation_revoked` (fake only until task 44) | removal leaves the delegation live |
| `team-feed/latest-and-cursor` | `after=latest` → no events, `next` = head; a poll with nothing visible still advances `next` | Hub task 26's cursor rule not carried to route 78 |

One row in full, the one the scope paragraph names:

```go
// control-plane/internal/hub/hubclient/conformance/team.go (excerpt)

// team-feed/revocation-within-one-poll. Fake mutant killed: RevokeDelegation records its
// event with FleetFeed false, so Fleet (task 11) never learns and keeps the sessions alive.
func rowRevocationWithinOnePoll(t *testing.T, e Env) {
	ctx := context.Background()
	dl, err := e.Member.CreateDelegation(ctx, hubapi.CreateDelegationRequest{
		ProjectID: e.ProjectID, Class: hubapi.AgentRoleOrchestrator, Routes: []string{"tasks.create"}})
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
`server/orgchanges.go`. The store query is Hub task 26's with three changes: `fleet_feed = true`
in place of `feed = true`; the visibility clause replaced by `(project mode = 'team' OR kind =
'team.member_removed')`; and `kind = ANY($kinds)` when `kinds` is set.

- [ ] **Step 5: Run everything to verify it passes**

```bash
cd control-plane && go test ./internal/hub/hubclient/... -count=1 -run 'TestFakeConforms/team' -v
cd control-plane && go test ./... -count=1
cd control-plane && go test -tags integration ./internal/hub/... -count=1
```

Expected: every `team/` and `team-feed/` sub-test `--- PASS` against the fake, then against the
real Hub under Hub task 34's integration conformance test, with exactly one `--- SKIP`
(`team-feed/member-removed-revokes`: `task 44`) and, on the real Hub only,
`team/revoked-and-expired` skipped for its expiry half (no clock). No line says `FAIL`.

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

1. Step 5's three commands pass as described; the conformance run lists 29 team rows.
2. `grep -c '^	[A-Z][A-Za-z]*(ctx' control-plane/internal/hub/hubclient/hubclient.go` rises by exactly
   27 (26 `Team` methods and `OrgChanges`) over the merged Hub's count.
3. `var _ hubclient.Hub = (*hubclient.Fake)(nil)` and `var _ hubclient.Hub = (*hubclient.Client)(nil)`
   compile. `Client` is the built HTTP type (`client.go:17`, a stub until Hub task 33); neither
   assertion exists on `feat/hub` as read on 2026-09-25, so this task adds both to
   `hubclient_test.go`.
4. `team-feed/revocation-within-one-poll` passes on both implementations: the scope's "a revoked
   delegation appears on the feed within one poll".

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

1. **Who sets a project's `mode`.** Route 78 filters on it; no scope paragraph writes it (as
   task 06's gap 3). The fake has `SetProjectMode` so tests can proceed.
2. **Thread listing.** As task 07's gap 2: `Team` has `Messages(threadID)` but no `Threads`, so
   task 11's rebuild cannot list open threads from the Hub.
3. **`Tasks` takes a query, not a page.** Task 11's part proposes `Tasks(ctx, projectID,
   hubapi.PageRequest)`; this part uses `hubapi.TaskQuery`, which embeds it. Task 11's call site
   changes by one composite literal.
