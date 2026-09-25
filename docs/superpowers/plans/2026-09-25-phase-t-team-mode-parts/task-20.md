### Task 20: Fleet sessions over the broker; `ag-…` ids; gateway tokens with team tags

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 11 (outbox, delegation cache, `Revoker`), 14
(`policy.Store.Effective`), 15 (`Spec.Class`, the policy mount), 19 (`control.Streams`), P6-15
(`gatewayclient`), P8-22 (`remotebe.Client`). **Seams:** S6.3 (the mint API), S8.1 (unchanged; this is
Fleet's side, not a `backend.Backend`).

A team agent is born here. `Launch` takes a request that has already passed task 12's role check,
mints the agent's `ag-…` id, writes the agent row and its Hub `agents.start` record in one
transaction, and returns — then, in the background, asks the broker for a Kata sandbox under the
owner's delegation, reads the pod's source address, mints a gateway key bound to that address and
tagged for the team, and hands the sandbox to task 19's streams. Everything the pod needs to prove who
it is (its agent id, its class, its control and attach tokens, its policy digest) is rendered into
the spec here and nowhere else.

Three rules from the design shape it. **One Orchestrator per person per project**, created on that
person's first team launch and never for a `viewer` (OD-16, `05-design.md` §4.1). **The owner is the
delegation's**, never Fleet's and never a body's (§3.3 rule 1; `task-01.md:122-126`). **No org key
and no Global fallback**: a sandbox holds a one-hour key bound to its pod, and an AU project whose AU
route is down gets no key at all (P6-D5, `.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway.md:69`).

**Built after the sync-merges of `feat/env` and `feat/phase-6-gateway`.** `remotebe` is phase 8 task
22's plan (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-22.md:60-116`),
`gatewayclient` is phase 6 task 15's plan (`2026-09-25-phase-6-gateway.md:250`) over task 03's wire
(`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-03.md:26-40`).
Neither exists on any branch on 2026-09-25; both are **UNVERIFIED until merge**. `ids.Wide` is built on
`feat/hub` (`control-plane/internal/ids/ids.go:61`) and not yet on `feat/team-mode`.

**Decisions made here.**

1. **`Launch` answers before the pod is ready.** The broker's create waits for `/healthz` for up to
   its `READY_TIMEOUT`, 120 s (phase 8 `task-22.md:91-92`). Task 21's `fleetbe` already expects Fleet to
   answer first and polls for readiness (`task-21.md:231-233`). So `Launch` commits the row with status
   `starting` and returns `201`; a goroutine does the broker create, the address, the mint and the
   stream. A failure there ends the agent with `exitClass: failed` through the same outbox path, so
   the Hub's record is never left `starting` forever.
2. **The id is `ids.Wide("ag")`** (task 09, `task-09.md:70`), and it is also the broker sandbox id,
   the keysmith `SessionID` and the pod's `PANACEA_APP_AGENT_ID`. One id end to end means the audit
   line, the binding and the Hub record can be joined without a mapping table. It matches
   `backend.Spec.ID`'s pattern (lower-case, `-`, ≤ 40), which the broker validates.
3. **One Orchestrator, enforced by the database.** Migration `0003_one_orchestrator.sql` adds a
   partial unique index on live Orchestrators per `(org, project, owner)`. `EnsureOrchestrator` inserts;
   on `store.ErrConflict` it reads the winner and returns it. Two replicas launching at once therefore
   yield one Orchestrator and two identical answers, with no lock.
4. **The Orchestrator needs its own delegation.** A worker delegation covers worker routes only
   (`task-01.md:292-297`), and the broker lets only an `orchestrator` delegation create an
   Orchestrator (task 17 decision 5). So a member's first team launch creates the Orchestrator **only
   if** a live `orchestrator`-class delegation for that owner and project is in task 11's cache;
   otherwise the worker launches alone, which task 01 allows ("a worker the member launched with no
   Orchestrator", `:294`), and the answer says `orchestrator: "awaiting-delegation"`. Who creates that
   delegation is *Gap S20-1*.
5. **The policy is read, never composed, here.** `policy.Store.Effective(ctx, org, project, class)`
   (task 14, `task-14.md:360`) returns the rendered file, version and digest; the file goes into the
   spec's policy field (task 15), the digest into `PANACEA_APP_POLICY_SHA256` (task 13's `EnvDigest`,
   `task-13.md:133`), and both into the `agents.start` body's `policy` (`task-01.md:350-360`). A render
   error fails the launch with `409 policy_invalid` before any sandbox exists.
6. **Tokens and control secrets are derived or minted, never stored.** The attach and control tokens
   are task 19's HMAC derivation (decision 6 there) with labels `panacea-attach/v1/` and
   `panacea-control/v1/`, so no replica stores them and task 39 can recompute the attach token. The
   gateway key is minted per session (worker) or per turn (Orchestrator) and sent down the stream with
   `Streams.Send` (task 16 step 6b), never `Push`, so it never touches Postgres.
7. **The tags are the mint request's fields.** `Owner{TID, OID}` is the delegation owner's (Gap S17-1:
   the Hub's delegation answer must carry them), `TeamID` and `ProjectID` from the delegation's project,
   `AgentID` the `ag-…`, `SessionClass` the class, `PodAddr` the broker's recorded source (task 17
   decision 9), and for a peer-triggered Orchestrator turn `Counterpart`, the other owner, which phase
   6's `MintRequest` does not have yet (*Gap S20-2*). Route is not sent: keysmith decides it from the
   project's route source and returns it (`task-03.md:31`).
8. **AU fails closed, twice.** Keysmith refuses to mint when the AU route is unknown (P6-D5). Fleet
   also reads the project's required route through phase 6's `wire.RouteSource` before minting and
   checks `MintResponse.Route` against it after; a mismatch revokes the key at once and fails the
   session with `gateway_route_mismatch`. There is no retry with different parameters, no Global
   default, and no path in this file that constructs a Global route.
9. **Revocation can always clean up.** Task 11's feed calls `Revoker.Revoke` on a revoked delegation
   (`task-11.md:569-582`). By then the broker refuses that delegation, so Fleet could not destroy the
   pod it made. Task 17 therefore lets `DELETE /v1/sandboxes/{id}` through under a revoked or lapsed
   delegation **for a sandbox labelled with that same `dl-…`** (task 17 decision 6's label) and for
   nothing else: a dead delegation may only undo its own work. `Revoke(…, Revoked)` destroys each of
   the delegation's sandboxes, revokes each keysmith session, and ends each agent with
   `access_removed`; `Revoke(…, Lapsed)` mints no further keys and stops redialling (task 19 decision
   5), and `Resume` restarts both. A lapsed agent's current key runs out within its hour.

**Files:**
- `control-plane/internal/fleet/sessions/sessions.go` — `Sessions`, `Config`, `LaunchRequest`,
  `Launch`, `EnsureOrchestrator`, `Stop`, `Live`, `Current`, `Revoke`, `Resume`, the spec render (new)
- `control-plane/internal/fleet/sessions/tokens.go` — `Tokens`: mint at open, refresh, mint per turn,
  the route check, revoke on end (new)
- `control-plane/internal/fleet/sessions/http.go` — task 21's three session routes on task 12's server (new)
- `control-plane/internal/fleet/sessions/{sessions,tokens,http}_test.go` and `fakebroker_test.go`,
  `fakekeysmith_test.go` (new)
- `control-plane/internal/fleet/store/migrations/0003_one_orchestrator.sql` (new)
- `control-plane/internal/backend/remotebe/client.go` — `WithDelegation(ctx, dl)`: `Do` and `Stream`
  add `X-Panacea-Delegation` when the context carries one (modified; phase 8 task 22's file,
  **UNVERIFIED until merge**)
- `control-plane/internal/gateway/wire/wire.go` — `MintRequest.Counterpart` (modified; phase 6 task
  03's file, **UNVERIFIED until merge**; path as phase 6's plan names it, read at merge)
- `control-plane/cmd/fleetd/main.go`, `config.go` — build `Sessions` and `Tokens`; five variables below
  (modified; task 12's files)
- Nothing in `cmd/broker`: decision 9's `DELETE` rule is task 17's code (its decision 11)

**Interfaces (complete):**

```go
// control-plane/internal/fleet/sessions/sessions.go
package sessions

// Broker is what Sessions needs from the broker, over remotebe.Client with
// the delegation on the context. The fake in fakebroker_test.go implements it.
type Broker interface {
	CreateSandbox(ctx context.Context, spec brokerapi.SandboxSpec) (brokerapi.Sandbox, error)
	DestroySandbox(ctx context.Context, id string) error
	Source(ctx context.Context, id string) (brokerapi.SourceAddr, error) // task 17
	DefaultMachine(ctx context.Context) (string, error)                 // POST /v1/machines/default
}

// Store is the slice of task 10's store.Store this package uses.
type Store interface {
	Tx(ctx context.Context, fn func(store.Tx) error) error
	Agent(ctx context.Context, org string, id model.AgentID) (model.Agent, bool, error)
	Agents(ctx context.Context, org, project string) ([]model.Agent, error)
	LiveAgents(ctx context.Context) ([]model.Agent, error) // added here: every non-ended agent, all orgs
}

// Policies is task 14's Effective.
type Policies interface {
	Effective(ctx context.Context, org, project string, class model.Class) (store.PolicyVersion, error)
}

type Config struct {
	Broker      Broker
	Store       Store
	Policies    Policies
	Delegations *delegation.Cache // task 11
	Hub         DelegationFetcher // a cache miss reads GET /v1/delegations/{id} as Fleet under it
	Tokens      *Tokens
	Streams     *control.Streams // task 19
	ControlKey  []byte           // task 19 decision 6's key
	InstallID   string           // PANACEA_APP_FLEET_INSTALL_ID, 32 lower-case hex (brokerapi.SandboxSpec.ControlPlane)
	GatewayURL  string           // PANACEA_APP_FLEET_GATEWAY_BASE_URL, the gatekeeper's in-cluster URL
	Now         func() time.Time
	Log         *slog.Logger
}

type DelegationFetcher interface {
	Delegation(ctx context.Context, owner model.Owner, id string) (delegation.Delegation, error)
}

// LaunchRequest is one agent to start. Principal is task 12's, already
// authorised for server.VerbLaunch; nothing here names an owner directly.
type LaunchRequest struct {
	Principal  server.Principal // TID, OID, Member{UserID, OrgID, Role}
	Project    string           // pj-…
	Class      model.Class      // orchestrator | worker
	Harness    string           // catalogue id, e.g. "claude-code"
	Version    string           // the harness's pinned version, for agents.start
	Command    string
	Args       []string // may hold the private opening prompt (OD-24): into the spec, nowhere else
	Delegation string   // dl-…
	Task       string   // tk-… or ""
	Parent     model.AgentID
	Ref        string // task 21's laptop ref; "" when Fleet-spawned
	Instance   string // task 21's instance id; "" when Fleet-spawned
}

// Launched is Launch's answer. Orchestrator is set on a worker launch: the
// owner's Orchestrator id, or "" with OrchestratorState "awaiting-delegation".
type Launched struct {
	Agent             model.Agent
	Orchestrator      model.AgentID
	OrchestratorState string // "live" | "created" | "awaiting-delegation" | "" for an orchestrator launch
}

var (
	ErrViewer          = errors.New("sessions: a viewer launches nothing")  // 403 role_forbids
	ErrDelegation      = errors.New("sessions: no live delegation")        // 401 delegation_expired
	ErrPolicyInvalid   = errors.New("sessions: the policy does not render") // 409 policy_invalid
	ErrAlreadyExists   = errors.New("sessions: ref already launched")      // 409 already_exists
)

func New(cfg Config) (*Sessions, error)

func (s *Sessions) Launch(ctx context.Context, r LaunchRequest) (Launched, error)

// EnsureOrchestrator returns the owner's live Orchestrator for the project,
// creating it under dl (an orchestrator-class delegation) if there is none.
func (s *Sessions) EnsureOrchestrator(ctx context.Context, p server.Principal, project, dl string) (model.Agent, bool, error)

// Stop ends one agent: destroy, revoke its keys, end it with exit, outbox agents.end.
func (s *Sessions) Stop(ctx context.Context, org string, id model.AgentID, exit model.ExitClass) error

// Live is task 19's control.Targets.
func (s *Sessions) Live(ctx context.Context) ([]control.Target, error)

// Current is task 26's turns.Sandboxes (task-26.md:504-506).
func (s *Sessions) Current(ctx context.Context, orch model.AgentID) (sandboxID string, ok bool)

// Revoke and Resume are the sessions half of task 11's Revoker (task-11.md:578-582).
func (s *Sessions) Revoke(ctx context.Context, d delegation.Delegation, why delegation.Reason) error
func (s *Sessions) Resume(ctx context.Context, d delegation.Delegation) error
```

```go
// control-plane/internal/fleet/sessions/tokens.go

// Minter is phase 6's gatewayclient (P6-15) as Fleet uses it.
type Minter interface {
	Mint(ctx context.Context, r gwire.MintRequest) (gwire.MintResponse, error)
	RevokeSession(ctx context.Context, sessionID string) error
}

// Counterparts tells which owner a turn's events came from, for the
// counterpart tag. Task 26's Scheduler or task 37 satisfies it (Gap S20-2).
type Counterparts interface {
	Counterpart(ctx context.Context, orch model.AgentID, turnID string) (model.Owner, bool, error)
}

type Tokens struct{ /* minter, routes, counterparts, streams, sources, generations */ }

func NewTokens(m Minter, routes gwire.RouteSource, c Counterparts, st *control.Streams, now func() time.Time) *Tokens

// Open mints generation 1 (or the next) for t and Sends it. Registered as
// Streams.OnOpen. A worker's key is refreshed at 45 minutes (P6's REFRESH_AT).
func (k *Tokens) Open(ctx context.Context, t control.Target) error

// BeforeTurn mints a fresh key for an Orchestrator's turn and Sends it before
// the turn.begin is written. Registered as Streams.Before("turn.begin", …).
func (k *Tokens) BeforeTurn(ctx context.Context, t control.Target, event json.RawMessage) error

// End revokes every key of the session. Called by Stop and Revoke.
func (k *Tokens) End(ctx context.Context, agent model.AgentID) error

var ErrRouteMismatch = errors.New("sessions: keysmith returned a route the project may not use")
```

`gwire` is phase 6's `wire` package (`task-03.md:26-40`); `gwire.RouteSource` is its project route
source (task 13 there, "Hub, file, cache, fail closed"). The phase 6 delta this task adds, one field:

```go
// In gwire.MintRequest, after AgentID. Empty for a worker and for a
// human-origin turn. Validated with the other ids: 1–128 of [A-Za-z0-9._:-].
Counterpart string `json:"counterpart,omitempty"`
```

```sql
-- control-plane/internal/fleet/store/migrations/0003_one_orchestrator.sql
-- One live Orchestrator per person per project (OD-16). Two replicas launching
-- at once: the second insert fails and EnsureOrchestrator reads the first.
CREATE UNIQUE INDEX one_live_orchestrator
    ON fleet.agents (org_id, project_id, owner)
    WHERE class = 'orchestrator' AND status <> 'ended';
```

**The spec, as rendered.** Every value the pod receives, and where each is consumed:

```go
// spec builds the broker's create body. It never reads a value from the
// request body except Command, Args and Harness; everything that proves
// identity is derived here.
func (s *Sessions) spec(a model.Agent, r LaunchRequest, pv store.PolicyVersion, route string, machine string) brokerapi.SandboxSpec {
	env := map[string]string{
		"PANACEA_APP_AGENT_ID":         string(a.ID),         // task 16: the stamp
		"PANACEA_APP_CLASS":            string(a.Class),      // task 16: the stamp
		"PANACEA_APP_CONTROL_TOKEN":    control.ControlToken(s.cfg.ControlKey, a.ID), // task 16; via the pod Secret, below
		policy.EnvDigest:               pv.Digest,            // task 15's policycheck
		"ANTHROPIC_BASE_URL":           s.cfg.GatewayURL,     // phase 6: the gatekeeper, never a provider
		"PANACEA_APP_GATEWAY_TOKEN_DIR": "/run/panacea/gateway", // task 16 step 6b writes here
	}
	if route == "au" {
		env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] = "1" // phase 6 task 14's AU rule
	}
	return brokerapi.SandboxSpec{
		ID: string(a.ID), SessionID: string(a.ID), Command: r.Command, Args: r.Args, Env: env,
		AttachToken:  attachToken(s.cfg.ControlKey, a.ID),
		Machine:      machine, // "" for an Orchestrator (task 15: no machine disk)
		ControlPlane: s.cfg.InstallID,
		// Task 15 adds Class and PolicyFile to SandboxSpec; the broker writes
		// PolicyFile into the per-pod Secret "<pod>-policy" and moves
		// PANACEA_APP_CONTROL_TOKEN from Env into that pod's attach Secret, as
		// it does AttachToken (phase 8 task-18.md:53), so neither is in the pod spec.
		Class: string(a.Class), PolicyFile: pv.Body,
	}
}
```

`route` is `"au"` or `"global"`, from `gwire.RouteSource` before the create; if the source errors, no
create happens (decision 8). A worker's `MachineMounts` (its worktree on the owner's machine) is phase 8
task 24b's machine agent's to create and is **not** rendered in TM1-α (*Gap S20-3*); task 33's script
prepares the clone.

**Routes (task 21's wire, `task-21.md:80-87`), on task 12's server:**

| Route | Verb (task 12) | Handler |
|---|---|---|
| `POST /v1/projects/{p}/agents` | `server.VerbLaunch` | decode task 21's `CreateAgentRequest` strictly; `Launch`; `201 AgentJSON`. `ErrViewer` → `403 role_forbids`; `ErrDelegation` → `401 delegation_expired`; `ErrAlreadyExists` → `409 already_exists`; `ErrPolicyInvalid` → `409 policy_invalid` |
| `GET /v1/agents?instance=<id>[&ref=<ref>]` | `server.VerbSessionsOwn` | the caller's own agents with that instance only (task 21 decision 2: a Fleet-spawned agent has no instance and never appears) |
| `DELETE /v1/agents/{ag}` | `server.VerbSessionsOwn` | the caller's own agent only, else `404 agent_not_found`; `Stop(…, stopped)`; `204` |

`AgentJSON` is built from `model.Agent` and holds no args, token, pod address or node (task 21's
`TestAgentJSONHasNoArgs`, `task-21.md:132-135`, `:390`). `GET /v1/agents/{ag}/attach` stays task 39's.

**Configuration** (task 12's `config.go`, a bad value fails start naming it):
`PANACEA_APP_FLEET_INSTALL_ID` (32 lower-case hex), `PANACEA_APP_FLEET_GATEWAY_BASE_URL` (http(s), the
gatekeeper's Service URL), `PANACEA_APP_FLEET_KEYSMITH_URL`, `PANACEA_APP_FLEET_KEYSMITH_TOKEN_FILE`
(the projected service-account token keysmith's `kube` caller auth reviews, phase 6 task 08), and
`PANACEA_APP_FLEET_BROKER_SCOPE` (`api://<broker>/.default`).

Consumes: task 09 `model.*`; task 10 `store.Tx.PutAgent`, `EndAgent`, `AppendOutbox`,
`store.PolicyVersion`, `store.ErrConflict`; task 11 `outbox.Item`, `outbox.OpAgentStart`,
`OpAgentEnd`, `delegation.Cache`, `delegation.Key`, `delegation.Reason`; task 12 `server.Principal`,
`Server.Route`, `VerbLaunch`, `VerbSessionsOwn`; task 13 `policy.EnvDigest`; task 14 `Effective`; task
17 `brokerapi.SourceAddr` and the Fleet routes; task 19 `control.Target`, `Streams.Send`, `OnOpen`,
`Before`, `ControlToken`; phase 6 `gatewayclient`, `gwire.MintRequest`, `MintResponse`,
`RouteSource`; phase 8 `remotebe.Client`, `brokerapi.SandboxSpec`. Consumed by: task 19 (`Live`), task
24 (`worker spawn|stop` call `Launch` and `Stop`), task 26 (`Current`), task 11 (`Revoke`, `Resume`),
task 21 (the routes), task 37 (the counterpart tag), task 39 (`attachToken`).

- [ ] **Step 1: Write the failing tests**

The harness: task 10's `memstore`, task 11's `delegation.Cache` holding `dl-a-w` (Alice, `pj-shop`,
`worker`) and `dl-a-o` (Alice, `pj-shop`, `orchestrator`), a fake broker that records every create
and destroy and answers `source` with `10.244.3.17`, a fake keysmith (`Minter`) that records every
`MintRequest` and answers with a route it is told to, a fake `RouteSource`, a fake `Streams` sink that
records `Send`, and a settable clock. Principals: Alice as `member`, Vic as `viewer`.

| Test | What it checks | Mutant it kills |
|---|---|---|
| `TestSecondOrchestratorLaunchReturnsTheFirst` | two `EnsureOrchestrator` calls for Alice, `pj-shop`: one agent row, one broker create, both return the same `ag-…`; the second's `created` is false | an Orchestrator per launch |
| `TestConcurrentOrchestratorLaunchesMakeOne` | 8 goroutines call `EnsureOrchestrator` at once on `memstore` (which enforces the same partial unique rule as `0003`): one row, one create | a check-then-insert race |
| `TestViewersLaunchIs403` | Vic's `Launch` → `ErrViewer`; no row, no create, no Orchestrator; over HTTP `403 role_forbids` | relying on task 12's route check alone, or creating a viewer's Orchestrator |
| `TestFirstWorkerLaunchCreatesTheOrchestrator` | with `dl-a-o` live, Alice's first worker launch creates her Orchestrator; `OrchestratorState == "created"`; the second worker launch says `"live"` | an Orchestrator only on explicit request |
| `TestNoOrchestratorDelegationWorkerRunsAlone` | with `dl-a-o` absent: the worker launches, no Orchestrator create, `"awaiting-delegation"` | creating an Orchestrator under a worker delegation |
| `TestAUProjectNeverMintsOnGlobal` | route source says `au`; fake keysmith answers `Route: global` → the key is revoked (`RevokeSession` recorded), nothing is `Send`-ed, the agent ends `failed` with `gateway_route_mismatch` | trusting keysmith's answer |
| `TestAURouteDownFailsClosed` | route source errors → no broker create, no mint, launch fails; keysmith `503 route_unknown` after create → no token, no second mint with other parameters | a fallback of any kind |
| `TestMintCarriesTheTeamTags` | a worker's first `MintRequest` has `Owner{tid,oid}` of the delegation owner, `TeamID`, `ProjectID`, `AgentID` = the `ag-…`, `SessionClass: worker`, `PodAddr: 10.244.3.17`, `Counterpart: ""` | Fleet's own identity in the tags, or a missing field |
| `TestEachOrchestratorTurnMintsAFreshKey` | two `turn.begin` pushes through `BeforeTurn`: two mints, generations 1 and 2, each `Send` precedes its turn; a peer-triggered turn carries `Counterpart` = Bob | one key per session for an Orchestrator |
| `TestLaunchReturnsBeforeTheSandboxIsReady` | the fake broker's create blocks 2 s; `Launch` returns in < 200 ms with status `starting` | the shared-deadline chain |
| `TestAgentStartIsInTheSameTx` | after `Launch`, the store holds the agent row **and** an outbox row `agents.start` whose payload matches `task-01.md:352-356`'s shape with `policy.version` and `digest` from `Effective`; a `Tx` that fails leaves neither | two writes that can diverge |
| `TestOpeningPromptStaysInTheSpec` | args `["-p","PROMPT-MARKER-7"]`: the marker is in the broker's recorded spec and absent from the agent row, the outbox payload, every log line and `AgentJSON` | persisting the private prompt (OD-24) |
| `TestSpecCarriesNoOrgKey` | the recorded spec's env has no `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` or `CLAUDE_CODE_USE_BEDROCK`, and `ANTHROPIC_BASE_URL` is the gatekeeper | a key in the environment (§2.5 constraint 9) |
| `TestRevokeDestroysAndEnds` | `Revoke(dl-a-w, Revoked)`: every sandbox of that delegation destroyed, `RevokeSession` for each, each agent ended `access_removed` with an outbox `agents.end` | stranded pods after a revocation |
| `TestLapseMintsNothingMore` | `Revoke(dl-a-w, Lapsed)` then a refresh tick: no mint; `Resume` → the next tick mints | minting for a lapsed delegation |

The three the scope names, in full:

```go
func TestSecondOrchestratorLaunchReturnsTheFirst(t *testing.T) {
	h := newHarness(t)
	a1, created1, err1 := h.s.EnsureOrchestrator(ctx, h.alice, "pj-shop", "dl-a-o")
	a2, created2, err2 := h.s.EnsureOrchestrator(ctx, h.alice, "pj-shop", "dl-a-o")
	if err1 != nil || err2 != nil || !created1 || created2 || a1.ID != a2.ID || h.broker.creates() != 1 {
		t.Fatalf("got %s/%v %s/%v, creates %d; want one Orchestrator", a1.ID, created1, a2.ID, created2, h.broker.creates())
	}
}

func TestViewersLaunchIs403(t *testing.T) {
	h := newHarness(t)
	_, err := h.s.Launch(ctx, h.req(h.vic, model.ClassWorker, "dl-v-w"))
	if !errors.Is(err, ErrViewer) || h.broker.creates() != 0 || h.store.count(t) != 0 {
		t.Fatalf("err %v, creates %d; want ErrViewer and nothing made", err, h.broker.creates())
	}
}

func TestAUProjectNeverMintsOnGlobal(t *testing.T) {
	h := newHarness(t)
	h.routes.set("pj-shop", "au")
	h.keysmith.answerRoute("global") // a keysmith bug, or a lowered class mid-flight
	l, _ := h.s.Launch(ctx, h.req(h.alice, model.ClassWorker, "dl-a-w"))
	h.waitEnded(t, l.Agent.ID)
	if h.streams.sent(l.Agent.ID, "token") != 0 || !h.keysmith.revoked(string(l.Agent.ID)) ||
		h.store.exit(t, l.Agent.ID) != model.ExitFailed {
		t.Fatal("an AU project's session received or kept a Global key")
	}
}
```
