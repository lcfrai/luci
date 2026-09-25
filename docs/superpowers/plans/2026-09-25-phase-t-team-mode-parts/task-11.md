### Task 11: Fleet as a delegated Hub caller — outbox, delegation client, feed, rebuild

**Slice:** TM1-α. **Focused time:** 3 hours. **dependsOn:** 08 (the `hubclient` team methods, the
fake's team half, the per-org feed), 10 (Fleet's `Store`, its transaction and the outbox table).
**Seam:** ST.3 (delegation), consumed; nothing here defines a wire.

**Every Hub-bound write is a row before it is a request.** Fleet decides things — a lease moved, a
thread opened, an agent started — and the Hub must hear each one exactly once, in order, even if
Fleet is killed between deciding and telling. So Fleet never calls the Hub from a handler. It writes
the Fleet state change **and** an outbox row in one Postgres transaction (task 10's `Store.Tx`), and
a singleton delivery loop (task 12 takes its advisory lock) sends the row with an
`Idempotency-Key` derived from the row's id. The Hub stores the response under *(principal, key)*
for 24 hours in the same transaction as the write (Hub contract §1, `hub-contract.md:50-55`, and
its write shape §10.6, `:2029-2046`), so a retry after a lost response replays instead of
double-writing. The key is the row id and never a fresh UUID per attempt: a per-attempt key is how
an idempotent API double-writes anyway.

**Order is per org, and a transient failure blocks only its own org.** Hub writes depend on each
other: an `agents/{ag}/end` before its `agents` start is a 404, a message before its thread is a
404. Delivery is FIFO per `org_id` by the outbox's sequence, head-of-line: a transient failure
(network, 5xx, 429, 408) holds that org's queue and retries the same row with backoff (1 s doubling,
capped at 30 s); other orgs keep flowing. A **permanent** failure (any other 4xx, including 422
`idempotency_mismatch`, which is a Fleet bug) moves the row to `dead` with the problem body and the
queue continues; a dead row is an operator alert, not a retry.

**Ten minutes of Hub failure pauses negotiation, not work** (`05-design.md` §2.2 item 3). When an
org's oldest undelivered row has been failing transiently for 10 minutes, the outbox's `Gate`
reports that org paused. Task 28's protocol and task 29's resolutions ask the gate before they
advance a thread; they do not advance, and they say `unavailable` with `since` (ST.1). Workers are
not touched: sessions, leases already granted and activity keep running, because a Hub outage must
not stop anyone's agent. The first successful delivery after the outage clears the pause. The
pause is per org because the outage is observed per org's queue; a Hub that is down for everyone
pauses everyone, one org at a time, within one backoff cycle.

**Fleet's workload identity alone can do nothing in the Hub.** ADR-H1's amendment (task 01) admits
Fleet's `azp` only on a request that names a delegation in `X-Panacea-Delegation: dl-…`, and the
Hub makes the delegation's owner the actor (task 05). So `delegation.Client` holds a cache of live
delegations keyed by *(org, owner, project)*, and every call it makes goes out under one: the
context carries the `dl-…` (task 08's `hubclient.WithDelegation`), and the call fails locally with
`ErrNoDelegation` before any network if the cache has none, or `ErrRouteNotDelegated` if the
delegation does not list the route name (task 01's §7.0 team table, `task-01.md:226-273`). Exactly
one call names no delegation: the per-org feed read, exempt by name (`task-01.md:26-29`). Records
Fleet itself authors — a conflict, a thread, a Fleet escalation — still go out under the delegation
of the owner the record is **first about** (`task-01.md:31-36`), so the outbox has no
Fleet-in-its-own-name write at all.

**A revoked delegation stops what it authorised within 60 seconds; a lapsed one pauses it**
(`05-design.md` §2.4 item 4, lines 174-181). The two are different acts and the `Revoker` is told
which: `revoked` stops the owner's sessions in that project, closes their sockets, withdraws their
leases and marks their cards *ended: access removed*; `lapsed` pauses the sessions and withdraws
the leases, and a renewal resumes them. Two paths reach the `Revoker`, and both must exist. The feed
path: task 08's per-org subscription carries a `delegation.revoked` event, polled every 10 s. The
clock path: a delegation whose `expiresAt` passes without a `delegation.renewed` event is lapsed,
and the sweeper, also every 10 s, finds it; a feed that is down must not keep a lapsed delegation
alive. The `Revoker` is an interface here, implemented by tasks 20, 23 and 39. This task proves
only that the right call is made, once per delegation, inside 60 s of fake time.

**Fleet persists no Hub record** (`ADR-H1:467-474`, widened to Fleet by task 01's amendment). The
outbox payload is the only place Hub-shaped bytes touch Fleet's disk, and a sent row keeps its
payload only until the janitor (24 hours, matching the Hub's idempotency window) deletes it. After a
restart, `rebuild.go` reads the per-org feed head **first** (`after=latest`, `hub-contract.md:1100`),
then lists the live delegations, open tasks, open threads and open escalations of every org that
has a Fleet agent row, holds them in memory, and starts the feed consumer **from that head**. Head
first, lists second is what makes the rebuild gapless: a change committed during the lists is on
the feed after the head, and applying it twice is harmless because every apply is by id and
revision.

**Files:**
- `control-plane/internal/fleet/outbox/outbox.go` — `Item`, `Queue`, `Sender`, `Deliverer`, `Gate`, the backoff and classify rules (new)
- `control-plane/internal/fleet/outbox/outbox_test.go` — order, idempotency key, transient vs permanent, the 11-minute outage, the janitor (new)
- `control-plane/internal/fleet/delegation/client.go` — `Key`, `Delegation`, `Cache`, `Client`, `ErrNoDelegation` (new)
- `control-plane/internal/fleet/delegation/feed.go` — `Revoker`, `Feed`, the 10-second poll and the expiry sweep (new)
- `control-plane/internal/fleet/delegation/rebuild.go` — `HubState`, `Rebuild` (new)
- `control-plane/internal/fleet/delegation/delegation_test.go` — the cache, the header, revocation by feed and by clock, the gapless rebuild (new)

**Interfaces:**
- Produces:
  - `outbox.Item`, `outbox.Op`, `outbox.State`, `outbox.Queue`, `outbox.Sender`, `outbox.Deliverer` with `New`, `(*Deliverer).RunOnce`, `(*Deliverer).Paused`, `(*Deliverer).Janitor`, `outbox.Gate`, `outbox.Classify`, `outbox.Enqueue`, `outbox.AllOps`, `outbox.PauseAfter` (10 min), `outbox.Retention` (24 h) — consumed by 06–07's payload shapes, 14, 20, 23–31 (every Fleet write), 12 (runs the loop)
  - `delegation.Key`, `delegation.Delegation`, `delegation.Cache` with `Put`/`Get`/`Drop`/`Expired`, `delegation.Client` with `Ctx`, `delegation.ErrNoDelegation`
  - `delegation.Revoker`, `delegation.Reason` (`revoked`, `lapsed`), `delegation.Feed` with `Poll`, `delegation.PollEvery` (10 s)
  - `delegation.HubState`, `delegation.Rebuild`
- Consumes:
  - task 10 — `store.Store` and its `Tx(ctx, func(store.Tx) error) error`; the store implements `outbox.Queue` (the outbox **owns** `Item`; the store imports `outbox`, never the reverse), and `store.Tx` has `AppendOutbox(ctx, outbox.Item) error`; `store.Store.OrgsWithAgents(ctx) ([]string, error)` for the rebuild
  - task 08 — `hubclient.Team` (one method per task 06–07 route), `hubclient.OrgFeed` with `OrgChanges(ctx, org, after string, kinds []string) (hubapi.ChangePage, error)`, `hubclient.WithDelegation(ctx, dl string) context.Context`, `hubclient.WithIdempotencyKey(ctx, key string) context.Context`, and the fake's team half
  - task 03 — `hubapi.Delegation` (`ID`, `OrgID`, `Owner{TID,OID}`, `ProjectID`, `Class`, `Routes`, `ExpiresAt`, `RevokedAt`), the list types for tasks, threads and escalations, and the feed kinds `delegation.created`, `delegation.renewed`, `delegation.revoked`
  - task 01 — the delegable route names (`task-01.md:226-261`), the class route sets (`:267-273`), the one exempt route (`:26-29`), "first about" for Fleet-authored records (`:31-36`), and the codes `delegation_required`, `delegation_revoked`, `delegation_expired`
  - Hub (built) — `hubclient.Error{Status, Problem, RetryAfter}`, `hubclient.ErrUnavailable` and `hubclient.Code` (`hub/hubclient/hubclient.go:111-135` on `feat/hub`); `hubclient.TokenSource` (`:19-20`); `ids.Wide` (`internal/ids/ids.go:61`)
- Amends: **the hubclient's own fresh key per call** (`hub-contract.md:1345`: "a fresh `Idempotency-Key` (UUID, `ids.UUID()`) on every `POST`, reused across its own retries") is right for a desktop and wrong for an outbox, whose retries span process restarts. `WithIdempotencyKey` overrides it for one call. If task 08's part does not add it, this task adds it to `hubclient/client.go`, eight lines, and says so in its commit.

**The interface, complete.** `control-plane/internal/fleet/outbox/outbox.go`:

```go
// Package outbox is Fleet's durable queue of Hub-bound writes. A write is
// enqueued in the same transaction as the Fleet state change it reports, and
// a singleton loop delivers it with the row id as its Idempotency-Key, so a
// kill between deciding and telling loses nothing and a retry never
// double-writes (hub-contract.md:50-55).
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// Op is the Hub route an Item is delivered to, spelled as task 01's
// delegable route name (task-01.md:226-261), so the Sender's route check and
// the Hub's are the same string. Only write routes appear; reads never queue.
type Op string

const (
	OpTaskCreate     Op = "tasks.create"
	OpTaskUpdate     Op = "tasks.update"
	OpAgentStart     Op = "agents.start"
	OpAgentEnd       Op = "agents.end"
	OpLeaseEvent     Op = "leases.events"
	OpConflictCreate Op = "conflicts.create"
	OpConflictUpdate Op = "conflicts.update"
	OpThreadCreate   Op = "threads.create"
	OpThreadUpdate   Op = "threads.update"
	OpMessageAppend  Op = "messages.append"
	OpEscalation     Op = "escalations.create"
	OpDirective      Op = "directives.append"
	OpTurn           Op = "turns.record"
)

// AllOps is every Op; TestEveryOpHasADeliverer ranges over it.
var AllOps = []Op{OpTaskCreate, OpTaskUpdate, OpAgentStart, OpAgentEnd, OpLeaseEvent,
	OpConflictCreate, OpConflictUpdate, OpThreadCreate, OpThreadUpdate, OpMessageAppend,
	OpEscalation, OpDirective, OpTurn}

type State string

const (
	StatePending State = "pending"
	StateSent    State = "sent"
	StateDead    State = "dead" // a permanent failure; an operator alert, never retried
)

const (
	// PauseAfter is how long an org's head row may fail transiently before
	// negotiation in that org pauses (05-design.md §2.2 item 3, "proposed").
	PauseAfter = 10 * time.Minute
	// Retention matches the Hub's idempotency window: a sent row older than
	// this can no longer be replayed safely, so it is deleted, payload and all.
	Retention  = 24 * time.Hour
	backoffMin = 1 * time.Second
	backoffMax = 30 * time.Second
)

// Item is one Hub-bound write. Payload is the exact request body and is never
// re-marshalled on retry: the Hub compares bodies under one key (§1).
type Item struct {
	Seq          int64  // assigned by the store; the delivery order within an org
	ID           string // ids.Wide("ob"); also the Idempotency-Key (19 chars, [a-z0-9-])
	OrgID        string
	OwnerTID     string // with OwnerOID, the owner the record is first about
	OwnerOID     string //   (task-01.md:31-36); always set, never Fleet's own id
	ProjectID    string // pj-…
	Class        string // model.Class of the delegation: "orchestrator" | "worker"
	// No dl-… is stored: the Sender resolves the owner's live delegation at
	// delivery, so a renewal between enqueue and delivery is used, and a
	// revocation fails the write locally (permanent) instead of at the Hub.
	Op           Op
	Subject      string // the record id the write concerns (tk-…, ag-…, th-…), for logs
	Payload      json.RawMessage
	CreatedAt    time.Time
	State        State
	Attempts     int
	NextAt       time.Time
	FailingSince time.Time       // zero while healthy; set on the first transient failure
	Problem      json.RawMessage // the Hub's problem body, on StateDead only
}

// Queue is the outbox table. Task 10's Store implements it; Enqueue itself
// goes through store.Tx.AppendOutbox so it shares the state change's commit.
type Queue interface {
	// Heads returns, per org, the lowest-Seq pending row whose NextAt <= now.
	Heads(ctx context.Context, now time.Time, limit int) ([]Item, error)
	MarkSent(ctx context.Context, id string, at time.Time) error
	MarkRetry(ctx context.Context, id string, attempts int, next, failingSince time.Time) error
	MarkDead(ctx context.Context, id string, problem json.RawMessage) error
	// Oldest failing row per org; the Gate's only input, so it survives a restart.
	FailingSince(ctx context.Context) (map[string]time.Time, error)
	DeleteSentBefore(ctx context.Context, t time.Time) (int, error)
}

// Appender is the one method of task 10's store.Tx the outbox needs.
type Appender interface {
	AppendOutbox(ctx context.Context, it Item) error
}

// Enqueue validates it and appends it inside the caller's transaction.
func Enqueue(ctx context.Context, tx Appender, it Item) error {
	if it.ID == "" || it.OrgID == "" || it.Op == "" || len(it.Payload) == 0 {
		return errors.New("outbox: id, org, op and payload are required")
	}
	if it.OwnerTID == "" || it.OwnerOID == "" || it.ProjectID == "" || it.Class == "" {
		return errors.New("outbox: every write names its owner, project and class: Fleet never writes as itself")
	}
	if !slices.Contains(AllOps, it.Op) {
		return errors.New("outbox: unknown op " + string(it.Op))
	}
	it.State = StatePending
	return tx.AppendOutbox(ctx, it)
}

// Sender delivers one Item: it sets the delegation (or AsFleet) and the
// Idempotency-Key = it.ID on ctx, then calls the Op's hubclient method.
type Sender interface {
	Send(ctx context.Context, it Item) error
}

// Class is how a delivery failure is treated.
type Class int

const (
	Delivered Class = iota
	Transient       // hold the org's queue, back off, retry the same row
	Permanent       // dead-letter the row, move on
)

// Classify is the whole failure rule. ErrUnavailable is hubclient's transport,
// timeout and 5xx (hubclient.go:131-133); 408 and 429 are the Hub asking us to
// wait. Everything else the Hub answered is a request it will never accept,
// including 422 idempotency_mismatch, which means Fleet re-marshalled a body.
// An error that is not a Hub answer at all (ErrNoDelegation, a bad Op) is a
// local bug and permanent too: retrying it forever would pause the org.
func Classify(err error) Class {
	if err == nil {
		return Delivered
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, hubclient.ErrUnavailable) {
		return Transient
	}
	var he *hubclient.Error
	if errors.As(err, &he) &&
		(he.Status == http.StatusRequestTimeout || he.Status == http.StatusTooManyRequests) {
		return Transient
	}
	return Permanent
}

// problemOf keeps the Hub's problem body for the dead-letter row, or a
// Fleet-side one naming the local error.
func problemOf(err error) json.RawMessage {
	var he *hubclient.Error
	if errors.As(err, &he) {
		b, _ := json.Marshal(he.Problem)
		return b
	}
	b, _ := json.Marshal(map[string]string{"error": "fleet_local", "message": err.Error()})
	return b
}

// Gate answers "may negotiation advance in this org?" Tasks 28-30 call it.
type Gate interface {
	Paused(org string) (paused bool, since time.Time)
}

// Deliverer is the singleton delivery loop's body. Task 12 runs RunOnce under
// the "fleet.outbox" advisory lock every 500 ms, and the janitor hourly.
type Deliverer struct {
	q    Queue
	send Sender
	now  func() time.Time

	mu       sync.Mutex
	failing  map[string]time.Time // org -> FailingSince, read from the Queue
	loadedAt time.Time
}

// gateTTL bounds how stale Paused may be. Paused reads the Queue, not the
// loop's memory, because only one replica runs RunOnce and both must answer.
const gateTTL = 5 * time.Second

func New(q Queue, s Sender, now func() time.Time) *Deliverer {
	return &Deliverer{q: q, send: s, now: now}
}

// RunOnce delivers at most one row per org, so one org's backlog cannot
// starve another, and returns how many rows changed state.
func (d *Deliverer) RunOnce(ctx context.Context) (int, error) {
	now := d.now()
	heads, err := d.q.Heads(ctx, now, 256)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, it := range heads {
		sendErr := d.send.Send(ctx, it)
		switch Classify(sendErr) {
		case Delivered: // the org's next head has a zero FailingSince: the pause clears
			err = d.q.MarkSent(ctx, it.ID, now)
		case Transient:
			since := it.FailingSince
			if since.IsZero() {
				since = now
			}
			err = d.q.MarkRetry(ctx, it.ID, it.Attempts+1, now.Add(backoff(it.Attempts+1)), since)
		case Permanent:
			err = d.q.MarkDead(ctx, it.ID, problemOf(sendErr))
		}
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// Paused implements Gate on every replica. It reads Queue.FailingSince at most
// once per gateTTL; a read error keeps the last answer rather than unpausing,
// because "the Hub is unreachable" and "Postgres blinked" must not look alike.
func (d *Deliverer) Paused(org string) (bool, time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if now := d.now(); d.failing == nil || now.Sub(d.loadedAt) >= gateTTL {
		if f, err := d.q.FailingSince(context.Background()); err == nil {
			d.failing, d.loadedAt = f, now
		}
	}
	since, ok := d.failing[org]
	return ok && d.now().Sub(since) >= PauseAfter, since
}

// Janitor deletes sent rows older than Retention.
func (d *Deliverer) Janitor(ctx context.Context) (int, error) {
	return d.q.DeleteSentBefore(ctx, d.now().Add(-Retention))
}

func backoff(attempt int) time.Duration {
	b := backoffMin << (attempt - 1)
	if b <= 0 || b > backoffMax {
		return backoffMax
	}
	return b
}
```

`control-plane/internal/fleet/delegation/client.go`:

```go
// Package delegation makes Fleet a delegated Hub caller (ADR-H1's amendment,
// task 01): every call but the per-org feed names a live dl-… of the owner
// it acts for, and the Hub records that owner as the actor.
package delegation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/outbox"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// Key is what a delegation is found by. An owner holds at most one live
// delegation per project and class (05-design.md §2.4 item 2).
type Key struct {
	OrgID, OwnerTID, OwnerOID, ProjectID string
	Class                                model.Class
}

type Delegation struct {
	ID        string   // dl-…
	Key       Key
	Routes    []string // task 01's route names
	ExpiresAt time.Time
}

func FromHub(d hubapi.Delegation) Delegation {
	return Delegation{
		ID: d.ID, Routes: slices.Clone(d.Routes), ExpiresAt: d.ExpiresAt,
		Key: Key{OrgID: d.OrgID, OwnerTID: d.Owner.TID, OwnerOID: d.Owner.OID,
			ProjectID: d.ProjectID, Class: model.Class(d.Class)},
	}
}

var (
	ErrNoDelegation      = errors.New("delegation: no live delegation for this owner, project and class")
	ErrRouteNotDelegated = errors.New("delegation: the live delegation does not list this route")
)

// Cache holds live delegations in memory only (Fleet persists no Hub record).
type Cache struct {
	mu   sync.RWMutex
	byK  map[Key]Delegation
	byID map[string]Key
}

func NewCache() *Cache { return &Cache{byK: map[Key]Delegation{}, byID: map[string]Key{}} }

// Put replaces the entry for d.Key: a newer delegation for the same key wins.
func (c *Cache) Put(d Delegation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.byK[d.Key]; ok {
		delete(c.byID, old.ID)
	}
	c.byK[d.Key], c.byID[d.ID] = d, d.Key
}

// Get is false for an absent or an expired entry: an expired delegation is
// never used, even before the sweeper removes it.
func (c *Cache) Get(k Key, now time.Time) (Delegation, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.byK[k]
	return d, ok && now.Before(d.ExpiresAt)
}

func (c *Cache) Drop(id string) (Delegation, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k, ok := c.byID[id]
	if !ok {
		return Delegation{}, false
	}
	d := c.byK[k]
	delete(c.byID, id)
	delete(c.byK, k)
	return d, true
}

// Expired removes and returns every entry with ExpiresAt <= now: the lapsed.
func (c *Cache) Expired(now time.Time) []Delegation {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Delegation
	for k, d := range c.byK {
		if !now.Before(d.ExpiresAt) {
			out = append(out, d)
			delete(c.byK, k)
			delete(c.byID, d.ID)
		}
	}
	slices.SortFunc(out, func(a, b Delegation) int { return a.ExpiresAt.Compare(b.ExpiresAt) })
	return out
}

// Client attaches the owner's delegation to a call's context.
type Client struct {
	cache *Cache
	now   func() time.Time
}

func NewClient(c *Cache, now func() time.Time) *Client { return &Client{cache: c, now: now} }

// Ctx returns ctx carrying k's live dl-…, checked locally against route so a
// doomed call never leaves the process. The Hub checks both again (task 05).
func (c *Client) Ctx(ctx context.Context, k Key, route string) (context.Context, error) {
	d, ok := c.cache.Get(k, c.now())
	if !ok {
		return ctx, ErrNoDelegation
	}
	if !slices.Contains(d.Routes, route) {
		return ctx, fmt.Errorf("%w: %s", ErrRouteNotDelegated, route)
	}
	return hubclient.WithDelegation(ctx, d.ID), nil
}

// Sender is outbox.Sender over the Hub: the row id is the Idempotency-Key,
// and the delegation is resolved now, not at enqueue.
type Sender struct {
	Client *Client
	Hub    hubclient.Team
}

func (s Sender) Send(ctx context.Context, it outbox.Item) error {
	k := Key{OrgID: it.OrgID, OwnerTID: it.OwnerTID, OwnerOID: it.OwnerOID,
		ProjectID: it.ProjectID, Class: model.Class(it.Class)}
	ctx, err := s.Client.Ctx(ctx, k, string(it.Op))
	if err != nil {
		return err
	}
	deliver, ok := deliverers[it.Op]
	if !ok {
		return fmt.Errorf("delegation: no deliverer for op %q", it.Op)
	}
	return deliver(hubclient.WithIdempotencyKey(ctx, it.ID), s.Hub, it)
}

type deliverer func(ctx context.Context, h hubclient.Team, it outbox.Item) error

// call adapts one task 08 method, whose path ids are (projectID, subject) or
// (projectID) alone, to a deliverer that decodes Payload into its request.
func call[Req, Res any](m func(hubclient.Team, context.Context, string, string, Req) (Res, error)) deliverer {
	return func(ctx context.Context, h hubclient.Team, it outbox.Item) error {
		var req Req
		if err := json.Unmarshal(it.Payload, &req); err != nil {
			return fmt.Errorf("delegation: payload for %s: %w", it.Op, err)
		}
		_, err := m(h, ctx, it.ProjectID, it.Subject, req)
		return err
	}
}

// deliverers has one entry per outbox.Op; TestEveryOpHasADeliverer fails on
// a missing one. Subject is "" for a create and the record id otherwise.
var deliverers = map[outbox.Op]deliverer{
	outbox.OpTaskCreate:     call(hubclient.Team.CreateTask),
	outbox.OpTaskUpdate:     call(hubclient.Team.PatchTask),
	outbox.OpAgentStart:     call(hubclient.Team.StartAgent),
	outbox.OpAgentEnd:       call(hubclient.Team.EndAgent),
	outbox.OpLeaseEvent:     call(hubclient.Team.AppendLeaseEvent),
	outbox.OpConflictCreate: call(hubclient.Team.CreateConflict),
	outbox.OpConflictUpdate: call(hubclient.Team.PatchConflict),
	outbox.OpThreadCreate:   call(hubclient.Team.CreateThread),
	outbox.OpThreadUpdate:   call(hubclient.Team.PatchThread),
	outbox.OpMessageAppend:  call(hubclient.Team.AppendMessage),
	outbox.OpEscalation:     call(hubclient.Team.CreateEscalation),
	outbox.OpDirective:      call(hubclient.Team.AppendDirective),
	outbox.OpTurn:           call(hubclient.Team.RecordTurn),
}
```

**The shape this consumes from task 08, stated so task 08 can match it.** `call` takes a method
expression of the form `func(hubclient.Team, context.Context, projectID, subjectID string, req Req)
(Res, error)`. Every task 06–07 write route has a project in its path and at most one record id
after it (`task-01.md:233-252`: `/v1/projects/{p}/tasks/{tk}`, `…/agents/{ag}/end`,
`…/leases/{ls}/events`, …), so one uniform signature covers all thirteen. A create method ignores
`subjectID`, except `AppendLeaseEvent` and `AppendMessage`, whose subject is the lease and the
thread. The method names above are this part's proposal; **if task 08's part names them otherwise,
task 08 wins** and only this map changes. The request types are task 03's (`CreateTaskRequest`,
`PatchTaskRequest`, `AgentStartRequest`, `AgentEndRequest`, `LeaseEventRequest`,
`CreateConflictRequest`, `PatchConflictRequest`, as `task-06.md:49-51` lists them, and task 07's
thread, message, escalation, directive and turn requests).

**The route names are task 01's, not task 06's.** `task-01.md:226-261` names thirteen write routes,
one per verb (`tasks.create`, `tasks.update`, `agents.start`, …); `task-06.md:80-82` lists a coarser
set (`tasks.write`, `agents.write`, …). ST.3 is defined in task 01 (skeleton seam table), so this
part follows task 01, and the disagreement is reported as a gap. Whichever set wins, it lives in one
place here: the `Op` constants.
