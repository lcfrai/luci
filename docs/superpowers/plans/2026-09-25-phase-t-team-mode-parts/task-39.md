### Task 39: Fleet's own attach handler and socket revocation

**Slice:** TM1. **Focused time:** 3 h. **dependsOn:** 11 (delegations, the `Revoker`, the
per-replica cache), 12 (Entra per request, `Server.Route`, the §3.4 table), 19 (one control stream
per sandbox, the broker dial); 17 (the broker's attach relay under a delegation) and 21 (`fleetbe`
dials `GET /v1/agents/{ag}/attach`) through them.

Phase 2's attach handler was written for a loopback control plane that one person runs on their
own laptop. It admits a caller by an origin check and a DNS-rebinding host check
(`control-plane/internal/api/attach.go:61-85`), with no per-person identity at all, then upgrades
and dials the sandbox with the session's stored attach secret (`:127-140`, `:194`). Fleet is a
**network service** that
holds every member's delegations, so its trust model is new, not ported (§7.4, global constraint
11). This task writes Fleet's own handler, in a new package, and pins four properties with tests:

1. **Every upgrade is authorised per request, per owner.** The Entra token (task 12's middleware)
   names a person; the agent's row names its owner; they must be the same person, whatever that
   person's role. An `admin` does not see a teammate's terminal (OD-15: no teammate terminal by
   default; the watch view is TM3 and opt-in).
2. **Knowing an `ag-…` id grants nothing.** Another owner's agent and a non-existent agent get the
   same answer, byte for byte, so the route is not an oracle for which ids exist.
3. **The relay carries a terminal only to its owner, and forwards only the owner's keystrokes.**
   It never originates a frame (task 38's `ptywriter` is the only Fleet code that does).
4. **Every open socket closes within 60 seconds of a revocation, on every replica.** Task 11's
   `Revoker` is called only on the replica holding the `fleet.feed` lock (`task-11.md:591-593`), but
   a socket lives on whichever replica the load balancer picked; so each replica closes its own.

And, per global constraints, the route joins the Z5 role matrix (§3.4, §7.4 item 5).

Five decisions the scope left open, made here so nobody re-derives them.

**(a) The route has no `{p}`, so the handler resolves the agent before it authorises.** Task 21
fixed the path as `GET /v1/agents/{ag}/attach` (`task-21.md:98`). Task 12's `authorize` looks up
membership by `r.PathValue("p")` (`task-12.md:466-467`), which is empty here. This task adds
`Server.RouteAgent`: it reads `{ag}` from task 10's agent rows, and runs task 12's `authorize` with
that agent's project in place of `{p}`, then requires `agent.Owner == principal.UserID`. The verb
is `VerbSessionsOwn` (`task-12.md:1154`: viewer no, member own, admin own, auditor no), so the
route sits in the matrix like every other.

**(b) Not yours and not there are one answer: `404 agent_not_found`.** The owner check runs
before any other refusal that could differ by agent (running, class, project), and a missing row
and another owner's row both produce `httpx` problem `agent_not_found` with the same title and no
`details`. `TestNotYoursEqualsNotThere` compares the two responses' bytes. Task 21's table lists
`404` and `409 agent_not_running`; the `409` is reached only by the owner.

**(c) Revocation is a per-replica sweep plus the leader's fast path.** Every replica runs task
11's `Feed`, so every replica's `delegation.Cache` drops a revoked delegation within one poll (10
s, `task-11.md:634-637`). That holds only because each replica's `Feed.Refresh` reads task 10's
`DelegationsInUse` every tick: a cache filled by `Adopt` alone would neither hold the delegations
of launches this replica did not serve — so this sweep would close their sockets at once as if
revoked — nor poll their orgs at all (review S02-C1, fix batch b03). Refresh's `pruneOrgs` does
not reopen that hole: it drops an org only when the in-use read no longer lists it **and** the
cache holds no entry for it, so no entry this sweep reads is ever removed by the prune (b03
round 2, M2). Each replica's relay keeps a registry of its own open sockets and, every
`SweepEvery` (10 s), closes each socket whose delegation is gone from its cache or past expiry, or
whose owner's membership no longer grants `VerbSessionsOwn` (read **uncached**, one Hub call per
distinct owner and project per sweep, so the membership cache's 60 s TTL does not stack on top).
The leader replica's `Revoke` also closes its own matching sockets at once. Worst case: poll 10 s
+ sweep 10 s + one Hub round trip, inside 60 s with room.

**(d) A socket also closes when the token that opened it expires.** An upgrade is authorised
once, by one token; a socket that outlived it would be authorised by nothing. At the token's
`exp` the relay closes with `4001` *"sign in again"*; `fleetbe` re-dials with a fresh token and
phase 2's replay restores the screen. Revocation closes with `4003` *"access removed"* (task 17
uses `4003` for the same condition on the broker's side), and `fleetbe` maps `4003` to
`ErrNotRunning`, so the cockpit says the session is no longer available and does not retry.

**(e) Attached is cluster-wide, and fails towards "attached".** Task 38 skips a nudge while the
owner is attached (`task-38.md` decision e), but its ladder runs on the replica holding the
worker's control stream, not the one holding the owner's socket. Each open socket is a row in
`fleet.attachments`, heartbeated every 15 s; `Attached(agent)` is true when a row for that agent
has a heartbeat inside 30 s. A store error answers **true**: the cost is a missed nudge, never a
template typed under someone's hands.

**Files:**
- `control-plane/internal/fleet/relay/attach.go` — `Relay`, `Handler`: resolve, owner check,
  upgrade, dial through the broker, pump both ways (new)
- `control-plane/internal/fleet/relay/revoke.go` — the socket registry, `Revoke`/`Resume`
  (`delegation.Revoker`), the sweep, the token-expiry timer, `Attached` (new)
- `control-plane/internal/fleet/relay/{attach,revoke}_test.go` (new)
- `control-plane/internal/fleet/server/server.go` — `RouteAgent` (modify, task 12's file, one
  method beside `Route`)
- `control-plane/internal/fleet/server/server_test.go` — the attach row in task 12's Z5 table test
  (modify, one row)
- `control-plane/internal/fleet/store/migrations/00NN_attachments.sql` — numbered after the last
  Fleet migration at merge (new)
- `control-plane/internal/fleet/store/{memstore,pgstore}.go`, `.../storetest/suite.go` — the
  `relay.Store` methods (modify, task 10's files)
- `control-plane/internal/backend/fleetbe/attach.go` — close code `4001` re-dials once with a fresh
  token, `4003` maps to `backend.ErrNotRunning` (modify, task 21's file)
- `control-plane/cmd/fleetd/main.go` — replaces task 21's `501` stub with the relay; adds the relay
  to task 11's `Revoker` fan-out; hands `relay.Attached` to task 38's ladder (modify, one hunk)

**Interfaces:**
- Produces:
  - `relay.New(Deps) *Relay`; `(*Relay).Handler() server.Handler`; `(*Relay).Run(ctx) error`
    (the sweep and heartbeat);
  - `(*Relay).Revoke` / `Resume`, satisfying `delegation.Revoker` (`task-11.md:582-585`);
  - `(*Relay).Attached(agent) bool`, task 38's `delivery.Attachments`;
  - `(*Relay).DialAttach(ctx, agent)`, task 38's `ptywriter.AttachDialer`, so both paths reach
    the sandbox through one dialer under one delegation rule;
  - `(*server.Server).RouteAgent(pattern, v, resolve, h)`;
  - close codes `4001` (sign in again) and `4003` (access removed), consumed by task 21's `fleetbe`.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 12: `entra.ClaimsFrom` (with the token's `exp`), `Server.authorize`, `Principal`,
    `Allowed`, `VerbSessionsOwn`, `Membership.Lookup` (`task-12.md:368-489`);
  - task 11: `delegation.Cache.Get(Key, now)`, `Delegation`, `Key`, `Reason`, `Revoker`
    (`task-11.md:383-447`, `:572-585`);
  - task 10: `store.Store`'s agent read (`Agent(ctx, id) (model.Agent, error)`, with `Owner`,
    `Project`, `Class`, `Status`, `SandboxID`);
  - task 17: the broker's attach relay for a principal under a delegation (`task-17.md:336-351`),
    reached through a one-method `BrokerAttach`;
  - phase 2: `backend.MaxMessageBytes` (4 MiB, the three-sites rule,
    `control-plane/internal/api/attach.go:127-140`, `sandboxd/internal/server/server.go:19-25`),
    `httpx.WriteProblem`, the attach frame kinds (`sandboxd/internal/wire/frame.go:14-19`).

#### `attach.go` (complete public surface and the path that carries the properties)

```go
// Package relay is Fleet's attach handler. It is not phase 2's api/attach.go
// and shares no code with it (05-design.md §7.4). It carries an agent's
// terminal to that agent's owner and nobody else, and it never originates a
// frame: every byte it sends up is a byte the owner's client sent.
package relay

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/delegation"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/server"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

const (
	frameStdin  = 0x01 // sandboxd/internal/wire/frame.go:16
	frameResize = 0x02 // :17

	CloseSignIn        websocket.StatusCode = 4001 // the opening token expired
	CloseAccessRemoved websocket.StatusCode = 4003 // revoked, lapsed, or no longer a member

	SweepEvery     = 10 * time.Second
	HeartbeatEvery = 15 * time.Second
	AttachedWithin = 30 * time.Second
)

// CodeAgentNotFound is task 21's 404 code (task-21.md:97-98), one answer for
// "not yours" and "not there" (decision b).
const CodeAgentNotFound = "agent_not_found"

var ErrNotRunning = errors.New("relay: agent not running")

// BrokerAttach dials the sandbox's /attach through the broker, as Fleet
// under the owner's delegation for the agent's class (task 17).
type BrokerAttach interface {
	Dial(ctx context.Context, d delegation.Delegation, sandboxID string) (*websocket.Conn, error)
}

type Agents interface {
	Agent(ctx context.Context, id model.AgentID) (model.Agent, error)
}

// Sandboxes is task 20's sessions.Current: the live sandbox of an agent.
type Sandboxes interface {
	Current(ctx context.Context, agent model.AgentID) (sandboxID string, ok bool)
}

type Deps struct {
	Agents      Agents
	Sandboxes   Sandboxes
	Delegations *delegation.Cache
	Members     server.Membership // UNCACHED: the sweep's re-check (decision c)
	Broker      BrokerAttach
	Store       Store // revoke.go: fleet.attachments
	Replica     string
	Now         func() time.Time
	Log         *slog.Logger
}

type Relay struct {
	d   Deps
	reg registry // revoke.go
}

func New(d Deps) *Relay

// Resolve is RouteAgent's resolver: the agent's project, for authorize.
// A missing agent resolves to "" and the route answers 404 (decision b).
func (r *Relay) Resolve(ctx context.Context, ag string) (project string) {
	a, err := r.d.Agents.Agent(ctx, model.AgentID(ag))
	if err != nil {
		return ""
	}
	return a.Project
}

// Handler runs after RouteAgent's authorize, so p is a member or admin of
// the agent's project with VerbSessionsOwn. Order: owner, delegation,
// sandbox, upgrade, dial, pump. Every refusal before the upgrade is a
// problem body; after it, a close code.
func (r *Relay) Handler() server.Handler {
	return func(w http.ResponseWriter, req *http.Request, p server.Principal) {
		ctx := req.Context()
		a, err := r.d.Agents.Agent(ctx, model.AgentID(req.PathValue("ag")))
		if err != nil || string(a.Owner) != p.UserID { // not there == not yours
			httpx.WriteProblem(w, http.StatusNotFound, CodeAgentNotFound, "No such agent.", nil)
			return
		}
		key := delegation.Key{OrgID: a.Org, Owner: a.Owner, ProjectID: a.Project, Class: a.Class}
		d, ok := r.d.Delegations.Get(key, r.d.Now())
		if !ok {
			httpx.WriteProblem(w, http.StatusConflict, "agent_not_running", "This agent is not running.", nil)
			return
		}
		sbx, ok := r.d.Sandboxes.Current(ctx, a.ID)
		if !ok {
			httpx.WriteProblem(w, http.StatusConflict, "agent_not_running", "This agent is not running.", nil)
			return
		}
		c, _ := entra.ClaimsFrom(ctx) // present: authorize refused otherwise
		client, err := websocket.Accept(w, req, nil) // default origin rule; fleetbe sends none
		if err != nil {
			return
		}
		client.SetReadLimit(backend.MaxMessageBytes)
		up, err := r.d.Broker.Dial(ctx, d, sbx)
		if err != nil {
			client.Close(websocket.StatusTryAgainLater, "sandbox unreachable")
			return
		}
		up.SetReadLimit(backend.MaxMessageBytes)
		s := r.reg.open(a, d.ID, p, c.ExpiresAt, client, up) // revoke.go
		defer r.reg.close(s, websocket.StatusNormalClosure, "")
		r.pump(ctx, s)
	}
}
```

`entra` is feat/env's `control-plane/internal/entra` (`Claims.ExpiresAt`, `entra.go:23-62`
there), which task 12 already imports; it joins the import list. The delegation checked is the
**agent's owner's**, which is the caller's by the line above, so a caller can never ride a
teammate's delegation. The upgrade happens **before** the broker dial, the order phase 2 and task
17 both use (`task-17.md:336`), so a dead sandbox is a close code the cockpit renders, not a
failed handshake.

**`pump`, the two directions.**
- **Down** (sandbox to owner): every message is copied as it arrives, unread beyond its length.
- **Up** (owner to sandbox): each message's first byte is checked, and only `frameStdin` and
  `frameResize` are forwarded; anything else (an `Stdout` or `Exit` kind the client has no
  business sending, an empty frame) is dropped and counted. A frame is forwarded **only while
  `s.live` is true**; revocation sets it false **before** it starts closing, so a keystroke racing
  a revocation is dropped, never delivered. The relay creates no frame: every up-message is a
  client message, forwarded as the same bytes.

#### `RouteAgent` (modify task 12's `server.go`: one method, complete)

```go
// RouteAgent registers a route keyed by {ag}, not {p}. It resolves the
// agent's project, authorises against it exactly as Route does, and folds
// every refusal that could depend on the agent (no such agent, not a member
// of its project, a role the verb refuses) into ONE 404, so an ag-… id is
// not an oracle (task 39 decision b). A Hub outage stays a 503: it says
// nothing about the agent.
func (s *Server) RouteAgent(pattern string, v Verb,
	resolve func(ctx context.Context, ag string) (project string), h Handler) {
	if _, ok := table[v]; !ok {
		panic("server: route " + pattern + " names verb " + string(v) + ", which has no §3.4 row")
	}
	inner := s.authorize(v, h)
	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := resolve(r.Context(), r.PathValue("ag"))
		if project == "" {
			notFound(w)
			return
		}
		r.SetPathValue("p", project) // authorize reads {p}
		rec := &refusalFolder{ResponseWriter: w}
		inner.ServeHTTP(rec, r)
	}))
}
```

`refusalFolder` is a `ResponseWriter` wrapper whose `WriteHeader(403)` writes `notFound`'s bytes
instead, and passes every other status through; it implements `http.Hijacker` by delegation, so
the WebSocket upgrade inside the handler still works (`coder/websocket`'s `Accept` needs it).
`notFound(w)` is `httpx.WriteProblem(w, 404, "agent_not_found", "No such agent.", nil)`, the same
call the handler makes, so all three refusals are byte-identical. `http.Request.SetPathValue` is
Go 1.22+, inside the 1.23 floor.

#### `revoke.go` (complete public surface)

```go
package relay

// socket is one open attach on this replica.
type socket struct {
	id        string // at-…, ids.Wide("at"), never leaves Fleet
	agent     model.Agent
	dl        string // the delegation it rides, dl-…
	key       delegation.Key
	tid, oid  string
	tokenExp  time.Time
	live      atomic.Bool
	client    *websocket.Conn
	up        *websocket.Conn
	timer     *time.Timer // fires at tokenExp: close CloseSignIn
}

type registry struct {
	mu   sync.Mutex
	open map[string]*socket // by id
}

// Store is the cross-replica attach table (decision e).
type Store interface {
	PutAttachment(ctx context.Context, id string, agent model.AgentID, replica string, at time.Time) error
	HeartbeatAttachments(ctx context.Context, replica string, ids []string, at time.Time) error
	DeleteAttachment(ctx context.Context, id string) error
	AttachedSince(ctx context.Context, agent model.AgentID, since time.Time) (bool, error)
	// DeleteReplicaAttachments clears a replica's rows at its start: a crash
	// must not leave an agent "attached" for ever.
	DeleteReplicaAttachments(ctx context.Context, replica string) error
}

// Revoke is delegation.Revoker on the leader: close this replica's sockets
// riding d, at once. Other replicas close theirs in their own sweep.
func (r *Relay) Revoke(ctx context.Context, d delegation.Delegation, why delegation.Reason) error

// Resume does nothing for sockets: a renewed delegation re-admits NEW
// upgrades; a closed socket is not reopened by Fleet (fleetbe re-dials).
func (r *Relay) Resume(ctx context.Context, d delegation.Delegation) error

// Run is the per-replica loop: every SweepEvery, close each socket whose
// delegation is gone or expired in THIS replica's cache, or whose owner no
// longer holds VerbSessionsOwn on the project (uncached Members); every
// HeartbeatEvery, heartbeat this replica's rows.
func (r *Relay) Run(ctx context.Context) error

// Attached is task 38's check: any heartbeat inside AttachedWithin, on any
// replica. A store error answers true (decision e).
func (r *Relay) Attached(agent model.AgentID) bool

// DialAttach is task 38's ptywriter.AttachDialer, under the agent owner's
// delegation, through the same BrokerAttach.
func (r *Relay) DialAttach(ctx context.Context, agent model.AgentID) (*websocket.Conn, error)
```

**Closing, in order**, for any cause: `live.Store(false)`; `client.Close(code, reason)` with
`CloseAccessRemoved` / `"access removed"` for a revocation, a lapse or a lost membership, and
`CloseSignIn` / `"sign in again"` at `tokenExp`; `up.CloseNow()`; `DeleteAttachment`; drop from
`open`. Closing `up` detaches only: `sandboxd` keeps the process running when a client goes
(`sandboxd/internal/server/server.go:200-203`), so a revocation ends the **view**, and task 20's
own `Revoker` half stops the **session** (`task-11.md:576`). Nothing here destroys an agent.

#### `00NN_attachments.sql` (complete; `NN` is the next Fleet migration number at merge)

```sql
CREATE TABLE fleet.attachments (
  attach_id     text        PRIMARY KEY CHECK (attach_id ~ '^at-'),
  agent_id      text        NOT NULL CHECK (agent_id ~ '^ag-'),
  replica       text        NOT NULL,
  heartbeat_at  timestamptz NOT NULL
);
CREATE INDEX attachments_by_agent ON fleet.attachments (agent_id, heartbeat_at);
```

The table holds which agent has a live viewer, nothing about who (the owner is the agent's by
construction) and nothing the socket carried. `at-` is a Fleet-local prefix, like task 09's `ev-`,
and never reaches the Hub.

- [ ] **Step 1: Write the failing tests first**

Fixture: Alice (`u-a`, member) with worker `ag-a1` on project `pj-1`; Bob (`u-b`, member of
`pj-1`); Vera (`u-v`, viewer of `pj-1`); Adam (`u-ad`, admin of `pj-1`); Olga (`u-o`, member of
another project only). Task 12's fake Entra validator and fake `Membership`; task 11's `Cache`
with Alice's worker delegation `dl-a`; a fake `BrokerAttach` that returns one end of an
in-process `coder/websocket` pair whose other end records every frame; a fake clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestBobCannotAttachToAlicesWorker` | Bob's valid token on `/v1/agents/ag-a1/attach`: 404 `agent_not_found`, **no** broker dial | an ownerless attach; a role check without the owner check |
| `TestAdminCannotAttachToAlicesWorker` | Adam, admin: the same 404 | "admin sees everything" (OD-15) |
| `TestNotYoursEqualsNotThere` | Bob on `ag-a1`, Olga on `ag-a1`, Vera on `ag-a1` and Alice on `ag-zz` (no such agent): four responses with identical status, headers `Content-Type` and body bytes | an id-existence oracle (403 vs 404, or a different title) |
| `TestAliceAttaches` | Alice: `101`, the broker dialled with `dl-a` and the worker's sandbox, a 4 KB replay reaches her client unchanged | a handler that never relays, or re-encodes |
| `TestOnlyStdinAndResizeGoUp` | Alice's client sends `0x01 "ls"`, `0x02 …`, `0x00 "x"`, `0x03 "y"` and an empty frame: the sandbox side receives exactly the first two, byte for byte | forwarding every kind, or rewriting bytes |
| `TestNonOwnerInputNeverForwarded` | Bob's refused upgrade leaves zero frames at the sandbox; a frame Alice sends after `live` is set false is dropped | a keystroke racing a revocation |
| `TestRevocationClosesInside60s` | `dl-a` revoked in the Hub fake: on a **non-leader** replica (no `Revoke` call at all), the socket closes with `4003` within 20 s of fake time (poll 10 s + sweep 10 s), and in every case before 60 s | relying on the leader's `Revoke` alone, which never reaches another replica's sockets |
| `TestLeaderRevokeClosesAtOnce` | `Revoke(dl-a, Revoked)` on the socket's replica: closed `4003` before `Revoke` returns | a fast path that waits for the sweep |
| `TestLostMembershipCloses` | Alice removed from `pj-1` in the fake `Membership`, delegation still cached: closed `4003` at the next sweep | re-checking only the delegation, or reading the 60 s cache |
| `TestTokenExpiryCloses4001` | a token with `exp` at t+5 min: open at t, closed `4001` at t+5 min exactly | a socket outliving the token that opened it |
| `TestCloseDetachesOnly` | after any close, the sandbox side saw a normal client close and **no** destroy call on the fake broker | a revocation that kills the agent from the relay |
| `TestAttachedAcrossReplicas` | Alice's socket on replica A; `Attached(ag-a1)` on replica B (same store) is true; 31 s after A's last heartbeat, false; a store error: true | per-replica memory, or failing towards "not attached" |
| `TestReplicaStartClearsItsRows` | rows left by a crashed replica `r1` are deleted when `r1` starts | an agent "attached" for ever after a crash |
| `TestRelayOriginatesNoFrame` | over a session with no client input, the sandbox side receives zero frames | a keepalive or a hello written as Stdin |
| `TestAttachInZ5` | task 12's matrix test gains the attach route: viewer, auditor, compliance, org admin, nobody → the 404; member own → 101; member not own → the 404 | a route outside §3.4 |

```bash
cd control-plane && go test ./internal/fleet/relay/... ./internal/fleet/server/... 2>&1 | head -3
```

Expected: `undefined: New` (build failure).

- [ ] **Step 2: `RouteAgent` and `refusalFolder` in task 12's `server.go`**

Add the attach row to `TestTheTableIsSection34`'s route half; run task 12's tests: every existing
row still passes.

- [ ] **Step 3: `attach.go`, then `revoke.go`, the migration and the store methods**

`memstore` and `pgstore` gain `relay.Store`; `storetest.Suite` gains the heartbeat window and the
replica-clear case, so both stores pass the same table.

- [ ] **Step 4: `fleetbe`'s close codes (task 21's file)**

`4001`: re-dial once with a fresh token from the desktop's token source; a second `4001` inside
one minute is returned as `backend.ErrNotRunning` (no loop). `4003`: `backend.ErrNotRunning` at
once. One test each in `fleetbe`'s `attach_test.go`.

- [ ] **Step 5: Wire it**

In `cmd/fleetd/main.go`: `rel := relay.New(relay.Deps{…, Replica: cfg.ReplicaID})`;
`DeleteReplicaAttachments(cfg.ReplicaID)` before serving; `srv.RouteAgent("GET
/v1/agents/{ag}/attach", server.VerbSessionsOwn, rel.Resolve, rel.Handler())` in place of task
21's `501` stub; add `rel` to task 11's `Revoker` fan-out beside tasks 20 and 23; `go rel.Run(ctx)`;
hand `rel` to task 38 as `Attached` and as the writer's `AttachDialer`, replacing task 38's
always-attached stub. `cfg.ReplicaID` is the pod name (`POD_NAME` from the downward API, task
32's chart); **required of task 12** if its config has no replica id.

- [ ] **Step 6: Acceptance**

```bash
cd control-plane && go vet ./internal/fleet/... ./internal/backend/fleetbe/... ./cmd/fleetd/... && go test ./internal/fleet/relay/... ./internal/fleet/server/... ./internal/fleet/store/... ./internal/backend/fleetbe/... -count=1 -race
```

Expected: `ok` for each, `-race` clean; the Postgres half of `storetest.Suite` runs where
`PANACEA_APP_FLEET_TEST_PG_DSN` is set. Then two guards, each of which must print nothing. No
Fleet package reuses phase 2's handler:

```bash
cd control-plane && grep -rn "internal/api\"" internal/fleet cmd/fleetd --include=*.go
```

and the relay never builds a frame (it forwards client messages only; task 38's guard excludes
this package by name on that promise):

```bash
cd control-plane && grep -nE "frameStdin\)|append\(\[\]byte\{|\[\]byte\{0x0" internal/fleet/relay/*.go | grep -v _test.go
```

The second guard matches a frame being **built**; the kind constants are compared, never written.
The 60-second bound on the real cluster is task 44's end-to-end revocation test (skeleton task
list), deferred there because it needs two replicas behind the real load balancer.

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/fleet/relay control-plane/internal/fleet/store
git commit -F <msgfile> -- control-plane/internal/fleet/relay control-plane/internal/fleet/server control-plane/internal/fleet/store control-plane/internal/backend/fleetbe/attach.go control-plane/internal/backend/fleetbe/attach_test.go control-plane/cmd/fleetd/main.go
```

**What this task does NOT claim.**
- **That a revocation stops the agent.** It closes the view. Task 20's `Revoker` half stops or
  pauses the session; this task only proves the socket closes.
- **60 s on the real cluster.** Proven here in fake time, per replica; the two-replica,
  real-load-balancer run is task 44's.
- **A teammate view.** No principal but the owner attaches. The opt-in watch view is TM3 (§9.6).
- **That the relay's up-direction check reads no meaning.** It reads one byte, the frame kind, of
  a protocol Fleet and `sandboxd` define; never the payload. That is framing, not PTY content, and
  it is the only byte inspected.
- **Protection from a compromised Fleet.** A Fleet replica holds every member's delegations and
  can dial any sandbox through the broker; the broker's own delegation check (task 17) is the
  second line, and the Hub's audit of every delegation is the third. This handler closes the
  **front** door only.
- **That `websocket.Accept`'s default origin rule is right for a browser.** Fleet's only client is
  `fleetbe` (Go, no `Origin`). A browser client would need an explicit origin list; none exists.

**Gaps against the skeleton, raised here and not resolved alone:**
1. **Task 21's route has no `{p}`** (`task-21.md:98`) while task 12's `authorize` needs one
   (`task-12.md:466-467`). Resolved here by `RouteAgent`, a new method on task 12's server; task
   12's part should list it.
2. **Task 11's `Revoker` runs on the leader only** (`task-11.md:591-593`); sockets are
   per-replica. Resolved here by a per-replica sweep over each replica's own `Cache`; tasks 20 and
   23 should check whether their halves have the same shape of problem (a session or lease held
   in one replica's memory).
3. **Task 12 needs a replica id** in its config for `fleet.attachments` (the pod name), and task
   32's chart must pass `POD_NAME`.
4. **Task 20 must expose `Current(ctx, agent)` for workers too**, not only Orchestrators (task
   26 names it for `orch`, `task-26.md:512-514`).
5. **Close codes.** `4001` and `4003` are defined here and in task 17 (`4003 delegation ended`);
   one list should live in task 09's wire file so `fleetbe`, the broker and Fleet share it.

