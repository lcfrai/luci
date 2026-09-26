### Task 19: Fleet's control streams and verb dispatch

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 12 (the chassis and `server.Singleton`), 17 (the
broker's `control` relay). **Seams:** consumes ST.1 and ST.3; produces `control.Dispatcher`,
`control.Handler`, `control.Caller` and `control.Streams`, which tasks 22–31 and 38 are already written
against (`task-22.md:49-52`, `task-23.md:63`, `task-26.md:102`, `:501-503`, `:650-652`, `:810-812`).

This is the Fleet end of the channel task 16 opened in the pod. Fleet holds one WebSocket to every live
sandbox, through the broker's `control` relay (task 17), and everything a `panacea` verb does arrives on
one of those sockets as a stamped request. Dispatch is where §3.3 rule 1 — *"the owner comes from the
token or the delegation, and only the owner's own agents write the owner's rows"* — becomes code. It
runs before any handler and answers three questions in order: is this a live session Fleet knows, may
its class call this verb, and does every object the arguments name belong to its owner. A handler that
runs has a `Caller` it can trust and nothing else; no handler reads an owner, an agent or a class out of
its `args`.

The answers come from Fleet's own state, not from the frame. `sandboxd` stamps every request with the
pod's agent id and class (task 16 decision 4), but the stronger fact is which socket the frame arrived
on: Fleet opened that socket, to that sandbox, for that agent. So dispatch builds the `Caller` from the
stream's own target and uses the stamp only as a cross-check. A frame whose stamp disagrees with its
stream is refused and logged as a security event, because it can only mean a broken or subverted
`sandboxd`.

**Decisions made here.**

1. **One replica holds every stream.** Task 16's `sandboxd` keeps only the newest `/control`
   connection (its decision 1), so two replicas each holding a stream to one sandbox would supersede
   each other forever. The stream manager is a task 12 singleton, `server.NewSingleton(pool,
   "streams", log)` (`task-12.md:70`); failover is the singleton's (`retry` and `check` 5 s,
   `task-12.md:589`). Sharding by agent is a later change behind the same `Streams` type, not a TM1-α
   one: a team of two people is a handful of sockets.
2. **`Push` is durable and callable on any replica.** Task 26's scheduler claims a turn on whichever
   replica wins `ClaimTurn` and then pushes `turn.begin` (`task-26.md:509-511`). So `Push` writes a row
   to `fleet.pushes` and returns; the leader drains each agent's rows in id order every 250 ms and
   deletes a row once it has been written to the socket. A push whose `until` has passed is dropped
   unsent and counted. A leader that dies mid-drain leaves the rows for the next one; a row written
   twice is safe because every pushed event carries its own id (`turn`, a nonce) and its consumer
   ignores a repeat (task 26 (c), task 27).
3. **`Send` is leader-local and never persisted.** A gateway token (task 20) and a `team.json`
   snapshot (task 22's `WorkerSnapshot`) go through `Send`, which writes to the socket only if this
   replica holds it and returns `ErrNoStream` otherwise. A secret is never written to Postgres, and a
   snapshot is regenerated, not replayed.
4. **Hooks on the leader, not callers from everywhere.** `OnOpen` runs after every (re)connect — task 20
   uses it to send the current token and task 22's snapshot. `Before(kind, fn)` runs before a pushed
   event of that kind is written — task 20 uses `Before("turn.begin", …)` to mint the fresh
   per-turn key and `Send` it first (S6.5, one key per Orchestrator turn). `OnLost` runs when a stream
   is gone for good — task 26 registers `Scheduler.StreamLost`.
5. **"Gone for good" is the broker's word, not a timeout.** A dropped socket is redialled with backoff
   (1 s doubling to 30 s, ±20% jitter). Only a `404 sandbox_not_found` from the broker (the pod no
   longer exists for this owner) or the target leaving `Targets.Live` ends the stream and fires
   `OnLost`. A `401 delegation_*` stops redialling without `OnLost`: revocation is task 11's `Revoker`,
   which ends the session through task 20 (`task-11.md:580-585`).
6. **The control token is derived, not stored.** Fleet presents `Panacea-Control-Token` (task 17) and
   `sandboxd` checks it (task 16). It is `base64url(HMAC-SHA256(K, "panacea-control/v1/" + agentID))`
   with `K` from `PANACEA_APP_FLEET_CONTROL_KEY_FILE` (a Key Vault file, task 32). Any replica can
   recompute it after failover, and no token sits in Postgres. Rotating `K` invalidates every live
   sandbox's token, so it is done with a relaunch; that is stated, not solved, in TM1-α.
7. **Handlers get 10 s.** `sandboxd` answers `unavailable` after 15 s (task 16 decision 5), so a handler
   deadline under that means a slow handler yields Fleet's own error, not a misleading "the team view
   is unavailable". A deadline miss is `internal` and logged with the verb, never the args.

**Files:**
- `control-plane/internal/fleet/control/dispatch.go` — `Caller`, `Handler`, `Option`, `OwnArgs`,
  `Dispatcher`, `Decode`, the refusal codes (new)
- `control-plane/internal/fleet/control/streams.go` — `Target`, `Targets`, `Streams`: the dial, the
  reconnect loop, the push drain, `Send`, the hooks (new)
- `control-plane/internal/fleet/control/token.go` — `ControlToken(key []byte, agent model.AgentID) string` (new)
- `control-plane/internal/fleet/control/{dispatch,streams,token}_test.go` (new)
- `control-plane/internal/fleet/store/migrations/0002_pushes.sql` — `fleet.pushes` (new, numbered after
  task 10's `0001_fleet.sql`)
- `control-plane/internal/fleet/store/{store.go,memstore.go,pgstore.go,storetest/suite.go}` — three push
  methods and their suite rows (modified; task 10's files)
- `control-plane/cmd/fleetd/main.go`, `config.go` — build the `Dispatcher` and `Streams`, run `Streams`
  on the `streams` singleton, `PANACEA_APP_FLEET_CONTROL_KEY_FILE` and `PANACEA_APP_FLEET_BROKER_URL`
  (modified; task 12's files)
- `testdata/panacea-control/v1/stamped-check.json` — read, not written: task 16's fixture, so the Fleet
  decode and the sandbox encode are pinned by the same bytes

**Interfaces (complete):**

```go
// control-plane/internal/fleet/control/dispatch.go
package control

// Caller is who a request is from. Dispatch builds it from the stream's
// Target, never from the frame, and only after every check below passed.
type Caller struct {
	Org     string
	Project string
	Agent   model.AgentID
	Owner   model.Owner
	Class   model.Class
}

// Handler answers one verb. A returned error made with wire.Refuse becomes
// that code; any other error becomes "internal" and is logged by verb only.
type Handler func(ctx context.Context, c Caller, args json.RawMessage) (any, error)

// Option adjusts one registration.
type Option func(*route)

// OwnArgs names top-level args keys whose value (a string id, or an array of
// them) must belong to the caller's owner. A missing object is agent_not_owner too,
// so a probe cannot tell "not yours" from "does not exist".
func OwnArgs(keys ...string) Option

type route struct {
	h     Handler
	owned []string
}

// Sessions and Owners are the two lookups dispatch needs; task 10's store
// and task 11's rebuild satisfy them.
type Sessions interface {
	Agent(ctx context.Context, org string, id model.AgentID) (model.Agent, bool, error)
}
type Owners interface {
	OwnerOf(ctx context.Context, org, id string) (model.Owner, bool, error) // ag-, tk-, ls-, dv-, es-
}

type Dispatcher struct {
	mu       sync.RWMutex
	routes   map[string]route
	sessions Sessions
	owners   Owners
	log      *slog.Logger
	timeout  time.Duration // 10 s
}

func NewDispatcher(s Sessions, o Owners, log *slog.Logger) *Dispatcher

// Handle registers verb. It panics on a duplicate verb or on a verb that no
// class may call in task 09's table: both are programming errors at start.
func (d *Dispatcher) Handle(verb string, h Handler, opts ...Option)

// Dispatch answers one frame that arrived on bound's stream.
func (d *Dispatcher) Dispatch(ctx context.Context, bound Target, frame []byte) wire.Response

// Decode is the strict args decode every handler uses (G2): unknown fields,
// trailing data and a non-object are wire.Refuse(CodeBadRequest, …).
func Decode[T any](args json.RawMessage) (T, error)

// Size limits. A frame is task 16's 64 KiB socket cap plus the stamp.
const (
	MaxFrameBytes = 66 << 10
	MaxArgsBytes  = 32 << 10
)
```

The refusal codes dispatch itself emits. Task 09 owns ST.1's code list; these are the names this part
proposes, and task 09 wins where it differs (*Gap S19-1*):

| Code | When | Logged as |
|---|---|---|
| `bad_request` | the frame is not ST.1 v1 (strict decode), or `args` is not an object | warn |
| `too_large` | frame > `MaxFrameBytes` or `args` > `MaxArgsBytes` | warn |
| `stamp_mismatch` | the frame's `agent` or `class` differs from the stream's target | **security**, error |
| `unknown_session` | the target's agent is absent from the store or `ended` | warn |
| `unknown_verb` | no handler is registered | info |
| `forbidden_class` | task 09's table does not give this class this verb | info |
| `agent_not_owner` | an `OwnArgs` key names an object of another owner, or none | **security**, warn |
| `internal` | a handler's non-refusal error or its 10 s deadline | error |

```go
// control-plane/internal/fleet/control/streams.go

// Target is one live sandbox Fleet should hold a stream to. Task 20's
// sessions.Live produces them; Delegation is the key task 11's cache is read by.
type Target struct {
	Org        string
	Project    string
	Agent      model.AgentID
	Owner      model.Owner
	Class      model.Class
	Sandbox    string         // the broker's sandbox id
	Delegation delegation.Key // task 11 (`task-11.md:374-381`)
}

type Targets interface {
	Live(ctx context.Context) ([]Target, error)
}

// Dialer opens the broker's control relay (task 17): GET
// {broker}/v1/sandboxes/{id}/control with Fleet's workload token, the
// X-Panacea-Delegation header and Panacea-Control-Token. A *Gone error means
// the broker answered 404 sandbox_not_found; a *Revoked error means 401
// delegation_*; anything else is retryable.
type Dialer interface {
	DialControl(ctx context.Context, t Target, delegationID, controlToken string) (*websocket.Conn, error)
}

type (
	Gone    struct{ Agent model.AgentID }
	Revoked struct{ Agent model.AgentID }
)

func (g *Gone) Error() string    { return "control: sandbox gone: " + string(g.Agent) }
func (r *Revoked) Error() string { return "control: delegation ended: " + string(r.Agent) }

// PushStore is the durable half of Push (task 10's Store gains these).
type PushStore interface {
	EnqueuePush(ctx context.Context, p Pushed) error
	PendingPushes(ctx context.Context, agent model.AgentID, now time.Time, limit int) ([]Pushed, error)
	DeletePush(ctx context.Context, id int64) error
}

type Pushed struct {
	ID    int64           // assigned by the store
	Org   string
	Agent model.AgentID
	Kind  string          // the event's "event" value, e.g. "turn.begin"
	Body  json.RawMessage // the event exactly as it is written to the socket
	Until time.Time       // dropped unsent after this
}

var ErrNoStream = errors.New("control: this replica holds no stream to that agent")

type Streams struct{ /* unexported: dispatcher, targets, dialer, store, cache, key, hooks, conns */ }

type StreamsConfig struct {
	Dispatcher  *Dispatcher
	Targets     Targets
	Dialer      Dialer
	Pushes      PushStore
	Delegations *delegation.Cache // task 11
	ControlKey  []byte            // read once from PANACEA_APP_FLEET_CONTROL_KEY_FILE
	Now         func() time.Time
	Log         *slog.Logger
}

func NewStreams(cfg StreamsConfig) *Streams

// Push is task 26's Pusher (`task-26.md:509-511`): durable, any replica.
// event must marshal to an object with an "event" key; until defaults to
// now + 10 min when the event carries no deadline of its own.
func (s *Streams) Push(ctx context.Context, agent model.AgentID, event any) error

// Send writes event now on this replica's stream, or returns ErrNoStream.
func (s *Streams) Send(ctx context.Context, agent model.AgentID, event any) error

func (s *Streams) OnOpen(fn func(ctx context.Context, t Target) error)
// Before runs fn with the pushed event's own bytes, so task 20 can read the turn id
// from a turn.begin and mint a key tagged for that turn's counterpart.
func (s *Streams) Before(kind string, fn func(ctx context.Context, t Target, event json.RawMessage) error)
func (s *Streams) OnLost(fn func(ctx context.Context, agent model.AgentID) error)

// Run is the singleton's loop: every 2 s reconcile Live() against held
// streams, every 250 ms drain pushes. It returns when ctx ends.
func (s *Streams) Run(ctx context.Context) error
```

Task 26 registers `OnLost(func(ctx, a) error { return sched.StreamLost(ctx, a) })`; its `StreamLost`
signature (`task-26.md:658-660`) is exactly that shape.

```sql
-- control-plane/internal/fleet/store/migrations/0002_pushes.sql
-- Events Fleet has decided to deliver to a sandbox and not yet written to its
-- control stream. Never a secret: tokens go through Streams.Send, not here.
CREATE TABLE fleet.pushes (
    id         bigserial   PRIMARY KEY,
    org_id     text        NOT NULL,
    agent_id   text        NOT NULL CHECK (agent_id LIKE 'ag-%'),
    kind       text        NOT NULL CHECK (kind IN ('turn.begin', 'turn.end', 'nudge', 'stop')),
    body       jsonb       NOT NULL,
    until_at   timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pushes_agent_order ON fleet.pushes (agent_id, id);
```

The `kind` check is the storage half of decision 3: `token` and `snapshot` cannot be enqueued even by
mistake. `PendingPushes` deletes rows with `until_at <= now` for that agent in the same statement
(`DELETE … RETURNING` then `SELECT`, one transaction) and returns the rest oldest first.

Consumes: task 09's `model.AgentID`, `model.Owner`, `model.Class`, `model.Agent`, `wire.Response`,
`wire.Refuse`, and a class table query this part calls `model.ClassMay(c model.Class, verb string) bool`
(task 09 is unwritten on 2026-09-25; **its spelling wins**); task 11's `delegation.Key`,
`delegation.Cache.Get` (`task-11.md:422-429`); task 12's `server.Singleton`; `coder/websocket`
(already a `control-plane` dependency, `go.mod`).

- [ ] **Step 1: Write the failing tests for dispatch (`dispatch_test.go`)**

The fixtures: `alice` and `bob` (owners), `ag-a-orch` (Alice's Orchestrator), `ag-a-w1` (Alice's
worker), `ag-b-w1` (Bob's worker), one fake `Sessions` and `Owners` over maps, and a probe handler that
records its `Caller` and returns `{"ok":1}`. `frame(agent, class, verb, args)` builds a `Stamped` line
the way task 16 does.

| Test | What it checks | Mutant it kills |
|---|---|---|
| `TestWorkerCallingPeerIsRefused` | `peer` registered; frame from `ag-a-w1` (`worker`) → `forbidden_class`, probe not called | dispatch without the class table |
| `TestAliceCannotStopBobsWorker` | `worker` registered with `OwnArgs("agent")`; `ag-a-orch` sends `{"op":"stop","agent":"ag-b-w1"}` → `agent_not_owner`, probe not called | skipping `OwnArgs`, or checking the caller's class but not the object's owner |
| `TestAliceStopsHerOwnWorker` | the same with `ag-a-w1` → ok, the probe's `Caller.Owner` is `alice` | an `OwnArgs` that refuses everything |
| `TestNonexistentObjectIsNotOwner` | `{"agent":"ag-nope"}` → `agent_not_owner`, not a distinct code | disclosing existence |
| `TestArrayOfIDsIsCheckedWhole` | `OwnArgs("agents")` with `["ag-a-w1","ag-b-w1"]` → `agent_not_owner` | checking only the first element |
| `TestStampMismatchIsRefusedAndLogged` | the stream's target is `ag-a-w1`; the frame says `ag-a-orch` → `stamp_mismatch` and one `level=ERROR` log with `security=true` | trusting the stamp over the stream |
| `TestCallerComesFromTheStreamNotTheFrame` | a frame whose `args` carries `"owner":"bob"` for a verb that takes no owner: the probe's `Caller.Owner` is `alice` and the probe's own `Decode` refuses the unknown key | reading identity from the body |
| `TestUnknownVerbAndUnknownField` | verb `frobnicate` → `unknown_verb`; a frame with top-level `"x":1` → `bad_request` | a lenient frame decode |
| `TestOversizedArgs` | `args` of 32 KiB + 1 → `too_large`, probe not called | no args cap |
| `TestEndedSessionIsUnknown` | `ag-a-w1` status `ended` → `unknown_session` | dispatching for a stopped agent |
| `TestHandlerDeadline` | a handler that blocks → `internal` within 10 s of a fake-clocked context; the log has the verb and no args | an unbounded handler |
| `TestStampedFixtureDecodes` | `testdata/panacea-control/v1/stamped-check.json` decodes strictly into this package's frame type | the two modules' frame types drifting |

The two tests the scope names, in full:

```go
func TestWorkerCallingPeerIsRefused(t *testing.T) {
	d, probe := newDispatcher(t)
	d.Handle("peer", probe.handler)
	got := d.Dispatch(ctx, targetFor("ag-a-w1"), frame("ag-a-w1", "worker", "peer", `{"kind":"ask"}`))
	if got.OK || got.Error == nil || got.Error.Code != wire.CodeForbiddenClass || probe.calls != 0 {
		t.Fatalf("got %+v, calls %d; want forbidden_class and no call", got, probe.calls)
	}
}

func TestAliceCannotStopBobsWorker(t *testing.T) {
	d, probe := newDispatcher(t)
	d.Handle("worker", probe.handler, OwnArgs("agent"))
	got := d.Dispatch(ctx, targetFor("ag-a-orch"),
		frame("ag-a-orch", "orchestrator", "worker", `{"op":"stop","agent":"ag-b-w1"}`))
	if got.OK || got.Error == nil || got.Error.Code != wire.CodeAgentNotOwner || probe.calls != 0 {
		t.Fatalf("got %+v, calls %d; want agent_not_owner and no call", got, probe.calls)
	}
}
```

`peer` is an Orchestrator verb and `worker` is registered for Orchestrators only in task 09's table
(§5.2, `05-design.md:547-558`); the tests register them here with a probe so the class check is tested
without tasks 24 and 28. Every code above is task 09's, in `control-plane/internal/fleet/wire/codes.go`
(`task-09.md:422-465`): `wire.CodeBadRequest`, `CodeTooLarge`, `CodeStampMismatch`,
`CodeUnknownSession`, `CodeUnknownVerb`, `CodeForbiddenClass`, `CodeInternal` and
`CodeAgentNotOwner`. Dispatch declares none of its own.

- [ ] **Step 2: Write the failing tests for the streams (`streams_test.go`)**

A fake sandbox is an `httptest.Server` whose `/control` accepts one WebSocket, records every frame it
receives, and can be told to send a stamped request or to drop the connection. The fake `Dialer`
dials it directly (no broker in the test) and can be told to answer `*Gone` or `*Revoked`. The store is
task 10's `memstore`, with the push methods.

| Test | What it checks | Mutant it kills |
|---|---|---|
| `TestOpensOneStreamPerLiveTarget` | `Live` returns two targets → two connections; a third appears → three; one leaves → its socket closes | a manager that never reconciles |
| `TestDroppedStreamReconnects` | the fake drops the socket; within the backoff (fake clock 1 s) a new connection arrives; a stamped request sent on the new one is answered | no redial |
| `TestNoConnectionWhileDown` | between the drop and the redial the fake records zero live connections, so a real `sandboxd` answers `unavailable` (task 16's `TestNoStreamAnswersUnavailableSinceStart`) | holding a half-closed socket as "up" |
| `TestGoneFiresOnLostOnce` | the dialer answers `*Gone` → `OnLost` called once with the agent, no further dials | redialling a deleted pod forever |
| `TestRevokedStopsWithoutOnLost` | `*Revoked` → no further dials, `OnLost` not called | treating revocation as a lost pod (task 26 would re-queue a revoked turn) |
| `TestPushFromAnyReplicaIsDelivered` | a second `Streams` value (not running) `Push`es `turn.begin`; the running one writes it within one drain tick; the row is gone | leader-only `Push` |
| `TestPushOrderPerAgent` | three pushes to one agent arrive in id order | draining by kind or by time |
| `TestExpiredPushIsDropped` | a push with `until` in the past is never written | replaying a stale turn |
| `TestBeforeRunsFirst` | `Before("turn.begin", fn)` where `fn` calls `Send(token)`: the fake receives the token frame, then the `turn.begin` | minting after the turn starts |
| `TestOnOpenAfterReconnect` | `OnOpen` runs on the first connection and again after a drop | a token sent once per process |
| `TestSendIsNeverPersisted` | `Send(token)` with no stream → `ErrNoStream`, and `PendingPushes` is empty; `EnqueuePush` with kind `token` fails on `pgstore` (the `CHECK`) | a secret at rest |
| `TestControlTokenIsStable` | `ControlToken(k, "ag-1")` equals a golden base64url string, differs for `ag-2` and for another key | a random token no failover can recompute |

- [ ] **Step 3: Run and confirm they fail**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && go test ./internal/fleet/control/ 2>&1 | head -3'
```

Expected: `undefined: NewDispatcher` (build failure).

- [ ] **Step 4: Implement `Dispatch`**

```go
// frame is task 16's Stamped, decoded strictly.
type frame struct {
	V     int             `json:"v"`
	ID    string          `json:"id"`
	Verb  string          `json:"verb"`
	Args  json.RawMessage `json:"args"`
	Agent model.AgentID   `json:"agent"`
	Class model.Class     `json:"class"`
}

func (d *Dispatcher) Dispatch(ctx context.Context, bound Target, raw []byte) wire.Response {
	if len(raw) > MaxFrameBytes {
		return refuse("", wire.CodeTooLarge, "frame too large")
	}
	var f frame
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&f) != nil || f.V != 1 || f.ID == "" || f.Verb == "" {
		return refuse(f.ID, wire.CodeBadRequest, "not a panacea control v1 frame")
	}
	if f.Agent != bound.Agent || f.Class != bound.Class {
		d.log.Error("control: stamp does not match stream", "security", true,
			"stream_agent", bound.Agent, "stamp_agent", f.Agent, "verb", f.Verb)
		return refuse(f.ID, wire.CodeStampMismatch, "stamp does not match stream")
	}
	a, ok, err := d.sessions.Agent(ctx, bound.Org, bound.Agent)
	if err != nil {
		return d.internal(f, err)
	}
	if !ok || a.Status == model.StatusEnded {
		return refuse(f.ID, wire.CodeUnknownSession, "no live session")
	}
	d.mu.RLock()
	r, known := d.routes[f.Verb]
	d.mu.RUnlock()
	switch {
	case !known:
		return refuse(f.ID, wire.CodeUnknownVerb, "unknown verb")
	case !model.ClassMay(bound.Class, f.Verb):
		return refuse(f.ID, wire.CodeForbiddenClass, "this class may not call "+f.Verb)
	case len(f.Args) > MaxArgsBytes:
		return refuse(f.ID, wire.CodeTooLarge, "args too large")
	}
	args := f.Args
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	c := Caller{Org: bound.Org, Project: bound.Project, Agent: bound.Agent, Owner: bound.Owner, Class: bound.Class}
	if code := d.checkOwned(ctx, c, r.owned, args); code != "" {
		return refuse(f.ID, code, "an argument names an object that is not yours")
	}
	hctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	res, err := r.h(hctx, c, args)
	if err != nil {
		var rf *wire.Refusal
		if errors.As(err, &rf) {
			r := refuse(f.ID, rf.Code, rf.Message)
			r.Error.Details = rf.Details
			return r
		}
		return d.internal(f, err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		return d.internal(f, err)
	}
	return wire.Response{ID: f.ID, OK: true, Result: b}
}
```

`checkOwned` decodes `args` into `map[string]json.RawMessage` (an object is required, else
`bad_request`), and for each owned key reads a string or an array of strings; every id goes through
`owners.OwnerOf`, and any miss or mismatch returns `wire.CodeAgentNotOwner` and logs `security=true`
with the verb and the key, never the value. `*wire.Refusal` is what task 09's `wire.Refuse` and
`RefuseWith` return (`task-09.md:470-484`), so `errors.As` recovers it and its `Details`; any other
error is `internal` with a fixed message, never the Go error string, as task 09 requires.
`model.StatusEnded` is task 09's (`task-09.md:102`). The session check reads Fleet's store on every
request; at a handful of agents per project that is one indexed read, and it is what makes a stopped
agent's late request fail rather than act.

- [ ] **Step 5: Implement `Streams`**

`Run(ctx)` loops on two tickers. Every 2 s: `Live()`, then for each new target start a goroutine
`hold(t)`, and cancel the goroutine of each target no longer live. Every 250 ms: for each held agent,
`PendingPushes(agent, now, 32)`; for each row run the `Before` hook for its kind (a hook error skips the
row this tick and logs it; the row stays), write `Body` to the socket, then `DeletePush(id)`.

`hold(t)` is the reconnect loop:

```go
func (s *Streams) hold(ctx context.Context, t Target) {
	backoff := time.Second
	for ctx.Err() == nil {
		d, ok := s.cfg.Delegations.Get(t.Delegation, s.cfg.Now())
		if !ok {
			s.cfg.Log.Info("control: no live delegation; not dialling", "agent", t.Agent)
			return // task 11's Revoker ends the session
		}
		conn, err := s.cfg.Dialer.DialControl(ctx, t, d.ID, ControlToken(s.cfg.ControlKey, t.Agent))
		var gone *Gone
		var revoked *Revoked
		switch {
		case errors.As(err, &gone):
			s.lost(ctx, t.Agent)
			return
		case errors.As(err, &revoked):
			return
		case err != nil:
			sleepJitter(ctx, backoff)
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		s.attach(t, conn) // Send and the drain can now reach it
		s.runOnOpen(ctx, t)
		s.readLoop(ctx, t, conn) // each frame: go-dispatch, write the Response back
		s.detach(t, conn)
	}
}
```

`readLoop` reads with `SetReadLimit(MaxFrameBytes)` and dispatches each frame in its own goroutine, so a
slow `check` does not hold up a `lease`; writes to the socket go through one mutex per connection.
`sleepJitter` sleeps `backoff ± 20%` or until `ctx` ends. `ControlToken` is
`base64.RawURLEncoding.EncodeToString(hmac(sha256, key, "panacea-control/v1/"+agent))`.

The broker `Dialer` (`brokerDialer`, in `streams.go`) is a `websocket.Dial` to
`{PANACEA_APP_FLEET_BROKER_URL}/v1/sandboxes/{t.Sandbox}/control` with `Authorization: Bearer <Fleet's
workload token for the broker audience>`, `X-Panacea-Delegation` and `Panacea-Control-Token`. It maps
the broker's HTTP answer before the upgrade: `404 sandbox_not_found` → `*Gone`; `401` with a
`delegation_*` code → `*Revoked`; anything else → a plain error. The workload token source is task 11's
(`delegation`'s token source, audience changed); **UNVERIFIED** that task 11 exposes an audience
parameter, else this task adds one.

- [ ] **Step 6: Wire into `fleetd`**

In task 12's `main.go`: read `PANACEA_APP_FLEET_CONTROL_KEY_FILE` (required, at least 32 bytes, else
exit naming the variable) and `PANACEA_APP_FLEET_BROKER_URL` (required, `https` unless loopback) in
`config.go`; build one `Dispatcher` and hand it to every `verbs.Register` (tasks 22–31); build
`Streams` with task 20's `sessions` as `Targets`; run `Streams.Run` under
`server.NewSingleton(pool, "streams", log)`. Task 26's step 6 then registers `turn.end` on this
`Dispatcher` and `Scheduler.StreamLost` with `Streams.OnLost` (`task-26.md:818-820`).

- [ ] **Step 7: Run the tests and the acceptance commands**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && go test ./internal/fleet/control/ ./internal/fleet/store/... ./cmd/fleetd/ && go vet ./internal/fleet/control/ ./internal/fleet/store/...'
```

Expected: `ok` for each; `go vet` silent. The `pgstore` half of the suite (`TestSendIsNeverPersisted`'s
`CHECK` case and the push rows) runs only with task 10's `PANACEA_APP_FLEET_TEST_DATABASE_URL` set; when
it is unset those cases skip and say so, and the CI job that sets it is the gate.

```bash
~/.claude/tools/claudezsh/claudezsh 'cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && go test -race -run "Dropped|Reconnect|NoConnectionWhileDown|PushFromAnyReplica" -count=5 ./internal/fleet/control/'
```

Expected: `ok`. Five runs under the race detector, because the reconnect path is the one with goroutines
racing a socket close.

- [ ] **Step 8: Commit**, after `git add` of the new files: `git commit -F
  .superpowers/sdd/team/msg/t19.txt -- control-plane/internal/fleet/control
  control-plane/internal/fleet/store control-plane/cmd/fleetd`.

**What this task does not claim.**

- That the stream reaches a real `sandboxd` through a real broker. The test's `Dialer` dials a fake
  sandbox directly; task 33 runs the path on the cluster.
- That one stream-holding replica is enough at scale. It is enough for TM1-α's two people; sharding is
  noted in decision 1 and not built.
- That `OwnArgs` alone keeps owners apart. It is the generic check; each handler still refuses what its
  own verb forbids (task 23's own-lease rule, task 31's own-worker rule), and the Hub's `agent_not_owner`
  check (`task-01.md:142-147`) is the last layer.
- **S19-1 — resolved.** Dispatch's codes were a gap until task 09's part landed; they are now
  task 09's `wire` constants, `ClassMay` is task 09's (`task-09.md:305-307`), and local verbs
  (`token`, `events.wait`) have no registration, so they reach `unknown_verb` as task 09 asks
  (`:274-276`).
- **Gap S19-2 — `fleet.pushes` is a schema change outside task 10.** The skeleton's file map gives
  Fleet's schema to task 10 alone. Durable cross-replica push (decision 2) needs one table; it is added
  here as migration `0002` rather than reopening task 10's `0001`.
- **Gap S19-3 — the control token.** The skeleton says task 15 puts "the control token" in the pod spec
  but not who makes it. This part derives it (decision 6), so task 20 renders
  `PANACEA_APP_CONTROL_TOKEN = ControlToken(K, agent)` into the spec and task 15 only mounts it.
