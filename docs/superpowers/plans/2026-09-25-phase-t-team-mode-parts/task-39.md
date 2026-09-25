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
fixed the path as `GET /v1/agents/{ag}/attach` (`task-21.md:87`). Task 12's `authorize` looks up
membership by `r.PathValue("p")` (`task-12.md:419-420`), which is empty here. This task adds
`Server.RouteAgent`: it reads `{ag}` from task 10's agent rows, and runs task 12's `authorize` with
that agent's project in place of `{p}`, then requires `agent.Owner == principal.UserID`. The verb
is `VerbSessionsOwn` (`task-12.md:1076`: viewer no, member own, admin own, auditor no), so the
route sits in the matrix like every other.

**(b) Not yours and not there are one answer: `404 agent_not_found`.** The owner check runs
before any other refusal that could differ by agent (running, class, project), and a missing row
and another owner's row both produce `httpx` problem `agent_not_found` with the same title and no
`details`. `TestNotYoursEqualsNotThere` compares the two responses' bytes. Task 21's table lists
`404` and `409 agent_not_running`; the `409` is reached only by the owner.

**(c) Revocation is a per-replica sweep plus the leader's fast path.** Every replica runs task
11's `Feed`, so every replica's `delegation.Cache` drops a revoked delegation within one poll (10
s, `task-11.md:568-570`). Each replica's relay keeps a registry of its own open sockets and, every
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
    `Allowed`, `VerbSessionsOwn`, `Membership.Lookup` (`task-12.md:321-442`);
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

// CodeAgentNotFound is task 21's 404 code (task-21.md:86-87), one answer for
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

