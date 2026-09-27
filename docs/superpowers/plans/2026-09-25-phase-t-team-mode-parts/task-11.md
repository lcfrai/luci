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
delegation does not list the route name (task 01's §7.0 team table, `task-01.md:231-279`). Exactly
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
then reads, by id, every delegation Fleet's own store says is in use (task 01's table has no
delegation list route, only `GET /v1/delegations/{id}`, route 47), then lists the open tasks and
conflicts of each project that has a live delegation, holds them in memory, and starts the feed
consumer **from that head**. Head first, reads second is what makes the rebuild gapless: a change
committed during the reads is on the feed after the head, and applying it twice is harmless because
every apply is by id. Open threads are **not** rebuilt from the Hub: their working state is Fleet's
own (task 10), and task 01's table has no thread list route either.

**Files:**
- `control-plane/internal/fleet/outbox/outbox.go` — `Item`, `Queue`, `Sender`, `Deliverer`, `Gate`, the backoff and classify rules (new)
- `control-plane/internal/fleet/outbox/outbox_test.go` — order, idempotency key, transient vs permanent, the 11-minute outage, the janitor (new)
- `control-plane/internal/fleet/delegation/client.go` — `Key`, `Delegation`, `Cache`, `Source`, `Client`, `ErrNoDelegation` (new)
- `control-plane/internal/fleet/delegation/feed.go` — `Revoker`, `Feed`, the 10-second poll, the per-tick `Refresh` from shared state and the expiry sweep (new)
- `control-plane/internal/fleet/delegation/rebuild.go` — `HubState`, `Rebuild` (new)
- `control-plane/internal/fleet/delegation/delegation_test.go` — the cache, the header, revocation by feed and by clock, the gapless rebuild (new)
- `control-plane/internal/fleet/outbox/store.go`, `control-plane/internal/fleet/delegation/store.go` — the two adapters to task 10's row types (new; task 10's handoff, `task-10.md:562-567`)

**Interfaces:**
- Produces:
  - `outbox.Item`, `outbox.Op`, `outbox.State`, `outbox.Queue`, `outbox.Sender`, `outbox.Deliverer` with `New`, `(*Deliverer).RunOnce`, `(*Deliverer).Paused`, `(*Deliverer).Janitor`, `outbox.Gate`, `outbox.Classify`, `outbox.Enqueue`, `outbox.AllOps`, `outbox.PauseAfter` (10 min), `outbox.Retention` (24 h) — consumed by 06–07's payload shapes, 14, 20, 23–31 (every Fleet write), 12 (runs the loop)
  - `delegation.Key`, `delegation.Delegation`, `delegation.Cache` with `Put`/`Get`/`Drop`/`Known`/`Expired`, `delegation.Source`, `delegation.Client` with `Ctx` (`NewClient(cache, src, now)`), `delegation.ErrNoDelegation`
  - `delegation.Revoker`, `delegation.Reason` (`revoked`, `lapsed`), `delegation.Feed` with `Poll`, `Refresh`, `Resolve`, `Adopt`, `Orgs`, `Sweep`, `Reset`, `delegation.PollEvery` (10 s)
  - `delegation.HubState`, `delegation.Rebuild(ctx, *Feed, *HubState)`, `delegation.Use`, `delegation.InUse`
- Consumes:
  - task 10 — `store.Store` and its `Tx(ctx, func(store.Tx) error) error`; the outbox methods `OutboxHeads`, `OutboxMarkSent`, `OutboxMarkRetry`, `OutboxMarkDead`, `OutboxFailingSince`, `OutboxDeleteSentBefore`, `Tx.AppendOutbox(ctx, store.OutboxRow)`, `store.OutboxRow`, `Store.DelegationsInUse(ctx) ([]store.DelegationUse, error)` (`task-10.md:95-110,146-170,196-202`). Task 10 is built first and does not import `outbox` (`task-10.md:562-567`), so **this task's `store.go` files convert**: `outbox.StoreQueue`, `outbox.TxAppender` and `delegation.StoreInUse`, below
  - task 08 — `hubclient.Team` (one method per task 06–07 route), `hubclient.OrgFeed` with `OrgChanges(ctx, org, after string, kinds []string, limit int) (hubapi.ChangesPage, error)` (the per-org twin of the built `Feed.Changes`, `hubclient.go:96-99`; `hubapi.Event` and `hubapi.ChangesPage` are built, `hubapi/types.go:536-551`), `Team.Delegation(ctx, id) (hubapi.Delegation, error)` (route 47, `delegations.read`), `Team.Tasks(ctx, projectID, hubapi.TaskQuery) (hubapi.Page[hubapi.Task], error)` (route 52; `TaskQuery{State []TaskState; hubapi.PageRequest}`, `task-08.md:101,137-139`, where task 08 names this change), `Team.Conflicts(ctx, projectID, hubapi.ConflictQuery) (hubapi.Page[hubapi.Conflict], error)` (route 59; `ConflictQuery{State string; hubapi.PageRequest}` from task 03), `hubclient.WithDelegation(ctx, dl string) context.Context`, `hubclient.WithIdempotencyKey(ctx, key string) context.Context`, and the fake's team half
  - task 09 — `model.Owner`, `model.Class`. **Assumed:** `model.Owner` is a string type holding the Hub user id (`us-…`), because every Hub team record names its owner that way (`task-06.md:105,111`); if task 09 makes it `(tid, oid)`, `FromHub` and the feed's decode are the two places that change
  - task 03 — `hubapi.Delegation` (`ID`, `OrgID`, `OwnerID`, `ProjectID`, `Class`, `Routes`, `ExpiresAt`, `RevokedAt`, as `task-06.md:105-107` shows the wire), the list types for tasks, threads and escalations, and the feed kinds `delegation.created`, `delegation.renewed`, `delegation.revoked`
  - task 01 — the delegable route names (`task-01.md:231-267`), the class route sets (`:273-279`), the one exempt route (`:26-29`), "first about" for Fleet-authored records (`:31-36`), and the codes `delegation_required`, `delegation_revoked`, `delegation_expired`
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

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// Op is the Hub route an Item is delivered to, spelled as task 01's
// delegable route name (task-01.md:231-267), so the Sender's route check and
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
	Owner        model.Owner // the owner the record is first about (task-01.md:31-36); never Fleet
	ProjectID    string      // pj-…
	Class        model.Class // of the delegation it goes under: orchestrator | worker
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

// Queue is the outbox table. StoreQueue (store.go) adapts task 10's Store to
// it; Enqueue goes through TxAppender so it shares the state change's commit.
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
	if it.Owner == "" || it.ProjectID == "" || it.Class == "" {
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
//
// ErrTransientLocal is the one exception, and it is the rule's point: Fleet
// failing to look something up — a Postgres blip, a Hub read timing out —
// is not the Hub refusing the row. It says nothing about the row, so the row
// must survive it. It lives here, not in delegation, because delegation
// imports outbox (Sender takes an outbox.Item) and the reverse would cycle;
// delegation.ErrResolveUnavailable wraps it.
var ErrTransientLocal = errors.New("outbox: a local failure, which says nothing about the row")

func Classify(err error) Class {
	if err == nil {
		return Delivered
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, hubclient.ErrUnavailable) ||
		errors.Is(err, ErrTransientLocal) {
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
	OrgID     string
	Owner     model.Owner
	ProjectID string
	Class     model.Class
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
		Key: Key{OrgID: d.OrgID, Owner: model.Owner(d.OwnerID),
			ProjectID: d.ProjectID, Class: model.Class(d.Class)},
	}
}

var (
	ErrNoDelegation      = errors.New("delegation: no live delegation for this owner, project and class")
	ErrRouteNotDelegated = errors.New("delegation: the live delegation does not list this route")
	// ErrResolveUnavailable is every Source failure: the store read failed, or
	// the Hub read of the one id did. It wraps outbox.ErrTransientLocal, so
	// outbox.Classify calls it Transient and the row waits instead of dying.
	// "Not found" is NOT one of these — that is (false, nil), then
	// ErrNoDelegation, which is permanent on purpose.
	ErrResolveUnavailable = fmt.Errorf("%w: delegation source unavailable", outbox.ErrTransientLocal)
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

// Known is Refresh's: an entry Get refuses as expired is still known, so a
// delegation waiting for the next Sweep is read from the Hub once, not once
// per tick.
func (c *Cache) Known(id string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.byID[id]
	return ok
}

// Expired returns every entry with ExpiresAt <= now: the lapsed. It removes
// nothing — Get already refuses them — so a sweep whose Revoker call failed
// sees the same entry next time, and a renewal (Put) replaces it in place.
func (c *Cache) Expired(now time.Time) []Delegation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []Delegation
	for _, d := range c.byK {
		if !now.Before(d.ExpiresAt) {
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(a, b Delegation) int { return a.ExpiresAt.Compare(b.ExpiresAt) })
	return out
}

// Source resolves a key this replica has no entry for, from shared state. A
// launch adopted on the other replica is a row in Fleet's own store before
// its first Hub write is (both are in task 20's one transaction), so the
// outbox never has to wait a poll tick to learn it. *Feed implements it; a
// nil Source is memory only, which is what the cache-only tests construct.
//
// A Source is a READ. It never acts: no Revoker, no Resume, no cursor move.
// The delivery goroutine must not be able to destroy a sandbox, and the feed
// goroutine is the only writer of the poll cursor.
//
// Every failure it returns wraps ErrResolveUnavailable, because a Source
// failure is Fleet's, never the Hub's verdict on the row: see Classify.
type Source interface {
	Resolve(ctx context.Context, k Key) (Delegation, bool, error)
}

// Client attaches the owner's delegation to a call's context.
type Client struct {
	cache *Cache
	src   Source
	now   func() time.Time
}

func NewClient(c *Cache, src Source, now func() time.Time) *Client {
	return &Client{cache: c, src: src, now: now}
}

// Ctx returns ctx carrying k's live dl-…, checked locally against route so a
// doomed call never leaves the process. The Hub checks both again (task 05).
// A miss asks the Source once: ErrNoDelegation then means "shared state has
// none either", not "this replica has not heard yet".
func (c *Client) Ctx(ctx context.Context, k Key, route string) (context.Context, error) {
	d, ok := c.cache.Get(k, c.now())
	if !ok && c.src != nil {
		var err error
		// The error is already wrapped in ErrResolveUnavailable, which
		// Classify calls Transient: the row retries, it does not die.
		if d, ok, err = c.src.Resolve(ctx, k); err != nil {
			return ctx, err
		}
	}
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
	k := Key{OrgID: it.OrgID, Owner: it.Owner, ProjectID: it.ProjectID, Class: it.Class}
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
after it (`task-01.md:238-258`: `/v1/projects/{p}/tasks/{tk}`, `…/agents/{ag}/end`,
`…/leases/{ls}/events`, …), so one uniform signature covers all thirteen. A create method ignores
`subjectID`, except `AppendLeaseEvent` and `AppendMessage`, whose subject is the lease and the
thread. The method names above are this part's proposal; **if task 08's part names them otherwise,
task 08 wins** and only this map changes. The request types are task 03's (`CreateTaskRequest`,
`PatchTaskRequest`, `AgentStartRequest`, `AgentEndRequest`, `LeaseEventRequest`,
`CreateConflictRequest`, `PatchConflictRequest`, as `task-06.md:63-65` lists them, and task 07's
thread, message, escalation, directive and turn requests).

**The route names are task 01's, not task 06's.** `task-01.md:231-267` names thirteen write routes,
one per verb (`tasks.create`, `tasks.update`, `agents.start`, …); `task-06.md:101-103` lists a coarser
set (`tasks.write`, `agents.write`, …). ST.3 is defined in task 01 (skeleton seam table), so this
part follows task 01, and the disagreement is reported as a gap. Whichever set wins, it lives in one
place here: the `Op` constants.

`control-plane/internal/fleet/delegation/feed.go`:

```go
package delegation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// PollEvery and the sweep period together bound revocation at 10 s + one
// Revoker call, well inside §2.4 item 4's 60 s, with room for a slow Hub.
const PollEvery = 10 * time.Second

// Reason tells the Revoker which act §2.4 item 4 requires.
type Reason string

const (
	Revoked Reason = "revoked" // stop sessions, close sockets, withdraw leases, card "ended: access removed"
	Lapsed  Reason = "lapsed"  // pause sessions, withdraw leases; a renewal resumes
)

// Revoker is implemented by tasks 20 (sessions), 23 (leases) and 39 (sockets)
// behind one fan-out. It must be idempotent: a leadership change may repeat a call.
type Revoker interface {
	Revoke(ctx context.Context, d Delegation, why Reason) error
	Resume(ctx context.Context, d Delegation) error
}

// FeedKinds is what Fleet subscribes to here; tasks 22-31 add their own kinds
// to the same poll through Feed.Also.
var FeedKinds = []string{"delegation.created", "delegation.renewed", "delegation.revoked"}

// Feed runs on every replica, so each keeps its Cache current; only the
// replica that holds the "fleet.feed" advisory lock (task 12) calls the
// Revoker — leader() reports that.
type Feed struct {
	hub    hubclient.OrgFeed
	team   hubclient.Team
	in     InUse // shared state: the one thing both replicas agree on
	cache  *Cache
	rev    Revoker
	leader func() bool
	now    func() time.Time

	mu     sync.Mutex
	cursor map[string]string // org -> next; set by Rebuild or Refresh before the first Poll
	done   map[string]Reason // dl id -> the last act the Revoker completed for it
	also   []func(context.Context, hubapi.Event) error
}

func NewFeed(hub hubclient.OrgFeed, team hubclient.Team, in InUse, c *Cache, rev Revoker,
	leader func() bool, now func() time.Time) *Feed {
	return &Feed{hub: hub, team: team, in: in, cache: c, rev: rev, leader: leader, now: now,
		cursor: map[string]string{}, done: map[string]Reason{}}
}

// Also registers another consumer of the same poll (the kinds it needs must
// be in the kinds list task 12 passes to Poll).
func (f *Feed) Also(fn func(context.Context, hubapi.Event) error) { f.also = append(f.also, fn) }

// Poll reads one page of org's feed and applies it. The cursor moves only
// after the whole page applied, so a failure re-reads the page; every apply
// is by id, so reading twice is harmless. more = ask again at once (§7.11).
func (f *Feed) Poll(ctx context.Context, org string, kinds []string) (more bool, err error) {
	f.mu.Lock()
	after, ok := f.cursor[org]
	f.mu.Unlock()
	if !ok {
		return false, fmt.Errorf("delegation: org %s polled before Rebuild set its cursor", org)
	}
	page, err := f.hub.OrgChanges(ctx, org, after, kinds, 500)
	if err != nil {
		return false, err
	}
	for _, ev := range page.Events {
		if err := f.apply(ctx, org, ev); err != nil {
			return false, err
		}
		for _, fn := range f.also {
			if err := fn(ctx, ev); err != nil {
				return false, err
			}
		}
	}
	f.mu.Lock()
	f.cursor[org] = page.Next
	f.mu.Unlock()
	return page.More, nil
}

func (f *Feed) apply(ctx context.Context, org string, ev hubapi.Event) error {
	switch ev.Kind {
	case "delegation.created", "delegation.renewed":
		// The feed carries metadata only (task-06.md:290-291), so read the record
		// itself, under itself: route 47 is delegations.read.
		hd, err := f.team.Delegation(hubclient.WithDelegation(ctx, ev.Subject), ev.Subject)
		switch hubclient.Code(err) {
		case "":
		case "delegation_revoked", "delegation_expired":
			return nil // a later event, or the sweep, handles it
		default:
			return err
		}
		if err != nil {
			return err
		}
		d := FromHub(hd)
		f.cache.Put(d)
		f.mu.Lock()
		delete(f.done, d.ID) // a renewal after a lapse must be able to lapse again
		f.mu.Unlock()
		if f.leader() {
			return f.rev.Resume(ctx, d) // idempotent; resumes only what the lapse paused
		}
	case "delegation.revoked":
		d, ok := f.cache.Drop(ev.Subject)
		if !ok { // unknown here: the Revoker still finds work by d.ID
			d = Delegation{ID: ev.Subject, Key: Key{OrgID: org, ProjectID: deref(ev.ProjectID)}}
		}
		return f.act(ctx, d, Revoked)
	}
	return nil
}

// Sweep lapses every expired delegation. It runs every PollEvery on every
// replica, independent of Poll, so a feed that is down cannot keep one alive.
// Refresh has already put every in-use delegation in this replica's cache, so
// the sweep covers the whole fleet, not only the launches this replica served.
func (f *Feed) Sweep(ctx context.Context) error {
	for _, d := range f.cache.Expired(f.now()) {
		if err := f.act(ctx, d, Lapsed); err != nil {
			return err
		}
	}
	return nil
}

// act calls the Revoker at most once per (delegation, reason) that succeeded;
// a revocation is final and outranks a lapse. A failed call is retried by the
// next Poll (the page is re-read) or the next Sweep (the entry is still there).
func (f *Feed) act(ctx context.Context, d Delegation, why Reason) error {
	f.mu.Lock()
	last := f.done[d.ID]
	f.mu.Unlock()
	if last == Revoked || last == why {
		return nil
	}
	if f.leader() {
		if err := f.rev.Revoke(ctx, d, why); err != nil {
			return err
		}
	}
	f.mu.Lock()
	f.done[d.ID] = why
	f.mu.Unlock()
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

**A leader change must not lose a revocation, and events alone would.** A follower marks `done`
without calling, so that it does not repeat every act when it becomes leader. But if the leader
died between reading `delegation.revoked` and calling the Revoker, the follower has already moved
its cursor past the event and dropped the entry, and nothing would ever stop those sessions. So
revocation is also enforced **by state**: on acquiring the `fleet.feed` lock, task 12 calls
`Reset` and then `Rebuild`, which reads every delegation Fleet's own store says is in use and asks
the Hub for each one's state (below). A revoked one answers 401 `delegation_revoked` and is acted
on then. Events make revocation fast; the state read makes it certain.

```go
// Reset forgets completed acts; task 12 calls it, then Rebuild, on becoming leader.
func (f *Feed) Reset() {
	f.mu.Lock()
	f.done = map[string]Reason{}
	f.mu.Unlock()
}

// SetCursor is Rebuild's: the head read before the lists, never after.
func (f *Feed) SetCursor(org, next string) {
	f.mu.Lock()
	f.cursor[org] = next
	f.mu.Unlock()
}
```

`control-plane/internal/fleet/delegation/rebuild.go`:

```go
package delegation

import (
	"context"
	"fmt"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// Use is one delegation a live agent row in Fleet's own store runs under.
type Use struct {
	ID  string // dl-…
	Key Key
}

// InUse is the slice of task 10's Store that Rebuild and Refresh read. It is
// the only state both replicas share about which delegations are live, so it
// is read every tick, not only at start.
type InUse interface {
	DelegationsInUse(ctx context.Context) ([]Use, error)
}

// HubState is the Hub-owned state Fleet reads from: memory only, rebuilt at
// every start, kept current by the feed consumers tasks 22-25 register.
type HubState struct {
	mu        sync.RWMutex
	tasks     map[string]map[string]hubapi.Task     // project -> tk -> task
	conflicts map[string]map[string]hubapi.Conflict // project -> cf -> open conflict
}

func NewHubState() *HubState {
	return &HubState{tasks: map[string]map[string]hubapi.Task{},
		conflicts: map[string]map[string]hubapi.Conflict{}}
}

func (s *HubState) PutTask(t hubapi.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tasks[t.ProjectID] == nil {
		s.tasks[t.ProjectID] = map[string]hubapi.Task{}
	}
	s.tasks[t.ProjectID][t.ID] = t
}

func (s *HubState) PutConflict(c hubapi.Conflict) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conflicts[c.ProjectID] == nil {
		s.conflicts[c.ProjectID] = map[string]hubapi.Conflict{}
	}
	s.conflicts[c.ProjectID][c.ID] = c
}

func (s *HubState) Task(project, id string) (hubapi.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[project][id]
	return t, ok
}

// Rebuild is gapless: for each org it reads the feed head, then the state,
// then sets the cursor to the head it read first. It is safe to call again:
// task 12 calls it at start on every replica and on becoming leader. It
// takes the Feed and reads the Feed's own InUse and clients, so Rebuild and
// Refresh can never be pointed at two different stores.
func Rebuild(ctx context.Context, f *Feed, s *HubState) error {
	hub, team := f.hub, f.team
	uses, err := f.in.DelegationsInUse(ctx)
	if err != nil {
		return fmt.Errorf("rebuild: delegations in use: %w", err)
	}
	byOrg := map[string][]Use{}
	for _, u := range uses {
		byOrg[u.Key.OrgID] = append(byOrg[u.Key.OrgID], u)
	}
	for org, us := range byOrg {
		head, err := hub.OrgChanges(ctx, org, "latest", FeedKinds, 1)
		if err != nil {
			return fmt.Errorf("rebuild %s: feed head: %w", org, err)
		}
		live, err := rebuildDelegations(ctx, team, f, us)
		if err != nil {
			return fmt.Errorf("rebuild %s: %w", org, err)
		}
		for project, dl := range live {
			if err := rebuildProject(hubclient.WithDelegation(ctx, dl), team, s, project); err != nil {
				return fmt.Errorf("rebuild %s/%s: %w", org, project, err)
			}
		}
		f.SetCursor(org, head.Next)
	}
	return nil
}

// rebuildDelegations asks the Hub for each in-use delegation's state, by id,
// under itself: 200 is live, 401 delegation_revoked / delegation_expired say
// exactly which act is owed. It returns one live dl per project to read under.
func rebuildDelegations(ctx context.Context, team hubclient.Team, f *Feed, us []Use) (map[string]string, error) {
	live := map[string]string{}
	for _, u := range us {
		hd, err := team.Delegation(hubclient.WithDelegation(ctx, u.ID), u.ID)
		switch hubclient.Code(err) {
		case "delegation_revoked":
			err = f.act(ctx, Delegation{ID: u.ID, Key: u.Key}, Revoked)
		case "delegation_expired":
			err = f.act(ctx, Delegation{ID: u.ID, Key: u.Key}, Lapsed)
		case "":
			if err != nil {
				return nil, err // transport: the whole rebuild retries
			}
			d := FromHub(hd)
			f.cache.Put(d)
			live[d.Key.ProjectID] = d.ID
			if f.leader() { // a renewal while Fleet was down left no event after the head
				err = f.rev.Resume(ctx, d)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return live, nil
}

// rebuildProject replaces a project's tasks and open conflicts, page by page.
func rebuildProject(ctx context.Context, team hubclient.Team, s *HubState, project string) error {
	for after := ""; ; {
		p, err := team.Tasks(ctx, project, hubapi.TaskQuery{PageRequest: hubapi.PageRequest{After: after, Limit: 200}})
		if err != nil {
			return err
		}
		for _, t := range p.Items {
			s.PutTask(t)
		}
		if p.Next == nil {
			break
		}
		after = *p.Next
	}
	for after := ""; ; {
		p, err := team.Conflicts(ctx, project, hubapi.ConflictQuery{State: "open",
			PageRequest: hubapi.PageRequest{After: after, Limit: 200}})
		if err != nil {
			return err
		}
		for _, c := range p.Items {
			s.PutConflict(c)
		}
		if p.Next == nil {
			return nil
		}
		after = *p.Next
	}
}

// Adopt is task 20's entry at launch; org comes from the caller's
// authenticated membership (task 12), never from the request body. Head
// first, record second, as in Rebuild. A launch never waits for the feed.
// It is this replica's fast path only: the other replica learns the same
// launch from Refresh, which reads the store the launch wrote.
func (f *Feed) Adopt(ctx context.Context, org, id string) (Delegation, error) {
	f.mu.Lock()
	_, tracked := f.cursor[org]
	f.mu.Unlock()
	if !tracked {
		head, err := f.hub.OrgChanges(ctx, org, "latest", FeedKinds, 1)
		if err != nil {
			return Delegation{}, err
		}
		f.SetCursor(org, head.Next)
	}
	hd, err := f.team.Delegation(hubclient.WithDelegation(ctx, id), id)
	if err != nil {
		return Delegation{}, err
	}
	d := FromHub(hd)
	if d.Key.OrgID != org {
		return Delegation{}, fmt.Errorf("delegation: %s is not in org %s", id, org)
	}
	f.cache.Put(d)
	return d, nil
}

// Refresh makes the tracked orgs and the live delegations shared state
// instead of replica memory. Task 12 calls it at the top of every tick,
// before Orgs: it reads task 10's DelegationsInUse — Fleet's own Postgres,
// the one thing both replicas see — and learns every org and every dl-… a
// launch on the other replica adopted. Head first for a new org, as in
// Adopt. Only ids the cache does not know are read, so a steady fleet costs
// one store query per tick and no Hub call.
func (f *Feed) Refresh(ctx context.Context) error {
	uses, err := f.in.DelegationsInUse(ctx)
	if err != nil {
		return fmt.Errorf("refresh: delegations in use: %w", err)
	}
	var unknown []Use
	for _, u := range uses {
		f.mu.Lock()
		_, tracked := f.cursor[u.Key.OrgID]
		_, acted := f.done[u.ID]
		f.mu.Unlock()
		if !tracked {
			head, err := f.hub.OrgChanges(ctx, u.Key.OrgID, "latest", FeedKinds, 1)
			if err != nil {
				return fmt.Errorf("refresh %s: feed head: %w", u.Key.OrgID, err)
			}
			f.SetCursor(u.Key.OrgID, head.Next)
		}
		if !acted && !f.cache.Known(u.ID) {
			unknown = append(unknown, u)
		}
	}
	_, err = rebuildDelegations(ctx, f.team, f, unknown)
	return err
}

// Resolve is the Client's read-through (Source): the outbox on the replica
// that did not serve the launch asks here rather than dead-lettering a live
// delegation it has not polled yet.
func (f *Feed) Resolve(ctx context.Context, k Key) (Delegation, bool, error) {
	if err := f.Refresh(ctx); err != nil {
		return Delegation{}, false, err
	}
	d, ok := f.cache.Get(k, f.now())
	return d, ok, nil
}

// Orgs is what task 12's feed loop polls, every PollEvery, after Refresh.
func (f *Feed) Orgs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.cursor))
	for org := range f.cursor {
		out = append(out, org)
	}
	return out
}
```

**Head first everywhere, because the other order has a hole.** Had `Adopt` read the delegation and
then the head, a revocation committed between the two reads would sit before the cursor and never
be applied, and the delegation would stay cached as live until its expiry, up to 24 hours. Reading
the head first puts that revocation after the cursor, so the next `Poll` applies it.
`TestAdoptReadsTheHeadBeforeTheRecord` pins the order with a fake that revokes between the calls.

**Neither the tracked orgs nor the live delegations may live in one replica's memory.** There are
two `fleetd` replicas (task 32) and three roles, and `Adopt` runs on whichever one served the
launch — which is neither the feed leader nor the outbox holder more often than by chance. Were
`cursor` and the `Cache` filled by `Adopt` alone, the other replica would learn the org only from
`Rebuild`, at its start and when it takes `fleet-feed`. Until then: its outbox would answer
`ErrNoDelegation` for every write of that launch and `Classify` would call that Permanent, so the
launch's whole Hub trail would be dead-lettered; and if it held `fleet-feed`, nobody would poll
that org or hold a cache entry to sweep, so a revocation would wait for the next leader change
rather than the 60 s §2.4 item 4 promises. So the org set and the delegation set are read from
**Fleet's own store** — `DelegationsInUse`, which task 10 already shares between the replicas —
by `Refresh`, at the top of every tick, on both replicas, and by `Resolve` on a cache miss in the
outbox. `Adopt` stays as the serving replica's fast path, not as the only path. Tasks 19, 20 and
39 read this same `Cache` to decide whether a delegation is live (`task-19.md:225`,
`task-20.md:133`, `task-39.md:174`); task 39's socket sweep closes a socket whose delegation is
"gone from its cache" (`task-39.md:45-49`), which without `Refresh` would close every socket of
every launch this replica did not serve.
`TestASecondFeedSeesADelegationAdoptedOnTheFirst` is the two-`Feed` test that pins it.

**The two adapters to task 10's store, complete.** Task 10 is built first and cannot import
these packages, so it keeps its own row types (`task-10.md:146-170,196-202`) and these files
convert, field for field. They are the only files here that import `internal/fleet/store`.

`control-plane/internal/fleet/outbox/store.go`:

```go
package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/store"
)

// StoreQueue is Queue over task 10's Store.
type StoreQueue struct{ S store.Store }

func (q StoreQueue) Heads(ctx context.Context, now time.Time, limit int) ([]Item, error) {
	rows, err := q.S.OutboxHeads(ctx, now, limit)
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, err
}
func (q StoreQueue) MarkSent(ctx context.Context, id string, at time.Time) error {
	return q.S.OutboxMarkSent(ctx, id, at)
}
func (q StoreQueue) MarkRetry(ctx context.Context, id string, n int, next, since time.Time) error {
	return q.S.OutboxMarkRetry(ctx, id, n, next, since)
}
func (q StoreQueue) MarkDead(ctx context.Context, id string, p json.RawMessage) error {
	return q.S.OutboxMarkDead(ctx, id, p)
}
func (q StoreQueue) FailingSince(ctx context.Context) (map[string]time.Time, error) {
	return q.S.OutboxFailingSince(ctx)
}
func (q StoreQueue) DeleteSentBefore(ctx context.Context, t time.Time) (int, error) {
	return q.S.OutboxDeleteSentBefore(ctx, t)
}

// TxAppender is Appender over one task 10 transaction: Enqueue(ctx, TxAppender{tx}, it).
type TxAppender struct{ Tx store.Tx }

func (a TxAppender) AppendOutbox(ctx context.Context, it Item) error {
	return a.Tx.AppendOutbox(ctx, toRow(it))
}

func toRow(it Item) store.OutboxRow {
	return store.OutboxRow{Seq: it.Seq, ID: it.ID, OrgID: it.OrgID, Owner: it.Owner,
		ProjectID: it.ProjectID, Class: it.Class, Op: string(it.Op), Subject: it.Subject,
		Payload: it.Payload, CreatedAt: it.CreatedAt, State: string(it.State), Attempts: it.Attempts,
		NextAt: it.NextAt, FailingSince: it.FailingSince, Problem: it.Problem}
}

func fromRow(r store.OutboxRow) Item {
	return Item{Seq: r.Seq, ID: r.ID, OrgID: r.OrgID, Owner: r.Owner, ProjectID: r.ProjectID,
		Class: r.Class, Op: Op(r.Op), Subject: r.Subject, Payload: r.Payload, CreatedAt: r.CreatedAt,
		State: State(r.State), Attempts: r.Attempts, NextAt: r.NextAt, FailingSince: r.FailingSince,
		Problem: r.Problem}
}
```

`control-plane/internal/fleet/delegation/store.go`:

```go
package delegation

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/store"
)

// StoreInUse is InUse over task 10's Store.
type StoreInUse struct{ S store.Store }

func (s StoreInUse) DelegationsInUse(ctx context.Context) ([]Use, error) {
	rows, err := s.S.DelegationsInUse(ctx)
	out := make([]Use, 0, len(rows))
	for _, r := range rows {
		out = append(out, Use{ID: r.ID, Key: Key{OrgID: r.OrgID, Owner: r.Owner, ProjectID: r.ProjectID, Class: r.Class}})
	}
	return out, err
}
```

`TestRowsRoundTrip` (in `outbox`) fills every `Item` field with a distinct value and asserts
`fromRow(toRow(it))` deep-equals `it`; the mutant is a field dropped in one direction, which a
retry would then send without its `FailingSince` and un-pause the org. `OutboxRow`'s field set is
`task-10.md:146-170`; if a field is added there, the round-trip test fails until it is added here.

- [ ] **Step 1: Write the failing outbox tests**

`control-plane/internal/fleet/outbox/outbox_test.go`, package `outbox`. The fixture is a
`memQueue` (a slice guarded by a mutex, implementing `Queue` and `Appender`, 60 lines, in the test
file: task 10's `memstore` is not imported, so this package's tests do not wait on task 10's
schema), a `scriptSender` whose answer per call is a queue of errors, and a `fakeClock` with
`Advance(d)`. Every test drives `RunOnce` directly; no goroutine, no sleep.

| Test | What it proves | The mutant it kills |
|---|---|---|
| `TestEnqueueRefusesAWriteWithNoOwner` | an `Item` with `Owner == ""` is refused; so is an unknown `Op` | a Fleet-in-its-own-name write slipping in (task 01 (c) forbids one) |
| `TestRetriesResendTheSameRowUnchanged` | three transient failures then success: the sender saw the same `Item.ID` and byte-identical `Payload` four times | a retry that re-marshals the body or re-mints the id (either one makes the Hub's replay check fail, `hub-contract.md:52-53`) |
| `TestOneOrgsFailureDoesNotHoldAnother` | org A's head fails transiently; org B's two rows are both delivered in two `RunOnce`s | a global head-of-line (one `Heads` row, not one per org) |
| `TestOrderIsPerOrgFIFO` | A1 fails once, A2 is queued behind it: A2 is not sent until A1 is | `Heads` returning any due row instead of the lowest `Seq` |
| `TestA429IsTransientAndA400IsPermanent` | `&hubclient.Error{Status:429}` → retry; `{Status:400}` → `dead` with the problem body; `hubclient.ErrUnavailable` wrapped in `fmt.Errorf("%w")` → retry | `Classify` comparing with `==` instead of `errors.Is`/`errors.As` |
| `TestIdempotencyMismatchIsDeadNotRetried` | 422 `idempotency_mismatch` → `dead` after one attempt | treating every 4xx as transient, which retries a re-marshalled body forever |
| `TestBackoffDoublesToThirtySeconds` | attempts 1..7 → 1, 2, 4, 8, 16, 30, 30 s; attempt 64 → 30 s, not a negative shift | an unbounded shift overflowing to zero, which retries in a hot loop |
| `TestElevenMinuteOutagePausesAndLosesNothing` | below | a pause keyed on attempts instead of `FailingSince`; a pause that drops rows; a pause that never clears |
| `TestPauseSurvivesARestart` | fail for 11 min, build a **new** `Deliverer` on the same queue: `Paused` is true at once | the pause held only in the loop's memory |
| `TestPausedIsReadOnAReplicaThatDoesNotDeliver` | a second `Deliverer` that never calls `RunOnce` answers `Paused` true | only the lock holder knowing the Hub is down |
| `TestJanitorDeletesOnlySentRowsPastRetention` | a sent row 25 h old goes; a sent row 23 h old and a pending row 3 days old stay | deleting pending rows, which loses writes |
| `TestEveryOpHasADeliverer` | lives in package `delegation`, ranges `outbox.AllOps` over `deliverers` | a new `Op` with no route, which would dead-letter every row of it |

The outage test, complete, because it is the property the design names (§2.2 item 3):

```go
func TestElevenMinuteOutagePausesAndLosesNothing(t *testing.T) {
	ctx := context.Background()
	clk := newFakeClock(time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC))
	q := newMemQueue()
	for i := 0; i < 5; i++ {
		mustEnqueue(t, q, testItem("org-a", i))
	}
	down := true
	s := &scriptSender{fn: func(Item) error {
		if down {
			return fmt.Errorf("dial: %w", hubclient.ErrUnavailable)
		}
		return nil
	}}
	d := New(q, s, clk.Now)

	for clk.Since() < 11*time.Minute {
		if _, err := d.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		clk.Advance(5 * time.Second)
		if got, _ := d.Paused("org-a"); clk.Since() < PauseAfter && got {
			t.Fatalf("paused after %s; the pause is at %s", clk.Since(), PauseAfter)
		}
	}
	if got, since := d.Paused("org-a"); !got || !since.Equal(clk.start) {
		t.Fatalf("Paused = %v since %s after 11 min down; want true since %s", got, since, clk.start)
	}
	if n := q.count(StatePending); n != 5 {
		t.Fatalf("%d rows pending after the outage; want all 5, none dropped or dead", n)
	}

	down = false
	for i := 0; i < 5; i++ {
		clk.Advance(backoffMax) // every row is due again
		if _, err := d.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := q.count(StateSent); n != 5 {
		t.Fatalf("%d rows sent after recovery; want 5", n)
	}
	clk.Advance(gateTTL)
	if got, _ := d.Paused("org-a"); got {
		t.Fatal("still paused after the Hub came back")
	}
	if got := s.ids(); !slices.Equal(dedupe(got), q.idsInSeqOrder()) {
		t.Fatalf("delivery order %v; want %v", dedupe(got), q.idsInSeqOrder())
	}
}
```

Run it and see it fail for the right reason:

```
cd control-plane && go test ./internal/fleet/outbox/ -run . -count=1
```

Expected before step 2: `undefined: New` and the other names — a compile failure, which is the
failure a test-first file has before its package exists.

- [ ] **Step 2: Write `outbox.go`** exactly as the interface block above, then run step 1's command.
  Expected: `ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/outbox`.
  Commit: `feat(fleet): the Hub outbox, per-org FIFO, row-id keys, the 10-minute pause`.

- [ ] **Step 3: Write the failing delegation tests**

`control-plane/internal/fleet/delegation/delegation_test.go`, package `delegation`, on task 08's
fake with its team half, built by `hubclient.FakeTeamProject(t)`, never a bare `NewFake()`: since
fix batch b01 the fake refuses a delegation on a solo project (409 `not_team_project`), and the
helper turns the project team through `SetProjectMode` as its team's admin (b01 round 2, re-review
I-6). Every delegation the tests create (`dl-1`, `dl-2`, …) is on that project. The same
`fakeClock`. A `recRevoker` records
every `Revoke(d, why)` and `Resume(d)` with the fake time it was called at. `leader` is a
`*atomic.Bool` the test flips. A `memInUse` is the shared store both `Feed`s read: a slice of
`Use` behind a mutex, with `add(Use)` for what a launch would have written and a counter of its
reads. Tests that exercise the cache alone construct `NewClient(cache, nil, clk.Now)`; the two
that exercise the read-through pass the `Feed`.

| Test | What it proves | The mutant it kills |
|---|---|---|
| `TestCtxRefusesWithoutADelegation` | no cache entry and a nil `Source` → `ErrNoDelegation`, and the fake saw **no** request | calling the Hub without the header and letting it answer 401 |
| `TestCtxRefusesARouteTheDelegationDoesNotList` | a worker-class delegation and `messages.append` → `ErrRouteNotDelegated` | a route check that trusts the Hub alone, so a doomed call still leaves |
| `TestAnExpiredEntryIsNeverUsed` | `ExpiresAt` one ms in the past, before any sweep → `ErrNoDelegation` | `Get` ignoring expiry until the sweeper runs |
| `TestSenderSetsTheRowIdAsTheKeyAndTheDelegation` | the fake records `Idempotency-Key = it.ID` and `X-Panacea-Delegation = dl-…` for an `agents.start` | the hubclient's fresh key surviving; the header missing |
| `TestSenderResolvesTheDelegationAtDelivery` | enqueue under dl-1, renew to dl-2 (a new launch), deliver: the fake saw dl-2 | a dl id frozen into the row at enqueue |
| `TestRevocationReachesTheRevokerInsideSixtySeconds` | below | a feed consumer that only refreshes the cache; a poll period over 60 s |
| `TestALapseWithTheFeedDownStillReachesTheRevoker` | the fake's feed returns `ErrUnavailable` throughout; `ExpiresAt` passes; within one `Sweep` period `Revoke(d, Lapsed)` | expiry noticed only through feed events |
| `TestRenewalAfterALapseResumes` | lapse, then `delegation.renewed`: one `Resume`, and a second lapse later is acted on again | `done` never cleared on renewal, so the second lapse is ignored |
| `TestRevokedOutranksLapsed` | lapse then revoke → two calls, `Lapsed` then `Revoked`; revoke then a stale sweep → one call | a lapse repeated after the final revocation |
| `TestAFollowerNeverCallsTheRevoker` | `leader=false`: events applied, cache updated, zero Revoker calls | every replica stopping the same sessions |
| `TestLeaderChangeCannotLoseARevocation` | leader A reads `revoked` and dies before `Revoke`; follower B advanced its cursor; B takes the lock, `Reset` + `Rebuild` → one `Revoke(Revoked)` | revocation by event only |
| `TestRebuildIsGapless` | a fake hook commits `delegation.revoked` after `OrgChanges(latest)` returns and before `Delegation(id)` returns; the next `Poll` applies it | reading the head **after** the state |
| `TestAdoptReadsTheHeadBeforeTheRecord` | the same hook on `Adopt`; the next `Poll` revokes | `Adopt` in the other order (a 24-hour window) |
| `TestASecondFeedSeesADelegationAdoptedOnTheFirst` | below, and it is the S02-C1 test: two `Feed`s on one `memInUse` and one fake Hub, A follower, B leader; B never calls `Adopt`, yet B's `Client` resolves the key with no poll and B calls `Revoke(Revoked)` within 60 s of the Hub revoking it | `Adopt` as the only way an org or a delegation is learned: the replica that did not serve the launch dead-letters its every write and never sweeps its revocation |
| `TestRefreshReadsOnlyTheDelegationsItDoesNotKnow` | one `Refresh`, then a second on the same `memInUse`: the fake saw exactly one `Delegation(dl-1)`; after `Drop`, a third makes one more | a per-tick re-read of every live delegation (one Hub call per agent per 10 s) |
| `TestResolveNeverInventsALapsedDelegation` | `memInUse` still lists a `dl-…` the Hub answers 401 `delegation_expired` for → `Ctx` is `ErrNoDelegation` and the `Revoker` got `Lapsed` exactly once | a read-through that trusts Fleet's own store instead of the Hub's answer |
| `TestRebuildPersistsNothing` | after `Rebuild`, the `InUse` fake saw only its one read, and no method of task 10's store that writes was called | Hub records drifting into Fleet's schema (`ADR-H1:467-474`) |
| `TestEveryOpHasADeliverer` | `outbox.AllOps` ⊆ keys of `deliverers`, and every Op is in task 01's orchestrator route set | a new `Op` with no route |

The revocation test, complete, because its number is a design commitment (§2.4 item 4):

```go
func TestRevocationReachesTheRevokerInsideSixtySeconds(t *testing.T) {
	ctx := context.Background()
	clk := newFakeClock(time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC))
	hub := hubclient.NewFake(hubclient.WithClock(clk.Now))
	dl := hub.SeedDelegation(t, "org-a", "us-alice", "pj-one", "worker") // task 08's seed helper
	rev := &recRevoker{now: clk.Now}
	leader := &atomic.Bool{}
	leader.Store(true)
	f := NewFeed(hub, hub, &memInUse{}, NewCache(), rev, leader.Load, clk.Now)
	if _, err := f.Adopt(ctx, "org-a", dl); err != nil {
		t.Fatal(err)
	}

	hub.RevokeDelegation(t, dl) // the owner's DELETE, route 49
	revokedAt := clk.Now()
	for clk.Since() < 2*time.Minute && len(rev.calls) == 0 {
		clk.Advance(PollEvery)
		if _, err := f.Poll(ctx, "org-a", FeedKinds); err != nil {
			t.Fatal(err)
		}
	}
	if len(rev.calls) != 1 || rev.calls[0].why != Revoked || rev.calls[0].d.ID != dl {
		t.Fatalf("revoker calls %+v; want exactly one Revoked for %s", rev.calls, dl)
	}
	if lag := rev.calls[0].at.Sub(revokedAt); lag > 60*time.Second {
		t.Fatalf("revocation reached the Revoker after %s; §2.4 item 4 allows 60 s", lag)
	}
	if _, err := NewClient(f.cache, nil, clk.Now).Ctx(ctx, Key{OrgID: "org-a",
		Owner: "us-alice", ProjectID: "pj-one", Class: "worker"}, "agents.start"); !errors.Is(err, ErrNoDelegation) {
		t.Fatalf("Ctx after revocation = %v; want ErrNoDelegation", err)
	}
}
```

`hubclient.WithClock`, `SeedDelegation` and `RevokeDelegation` are fake-only helpers this test needs
from task 08; if task 08's fake names them otherwise, rename here.

The two-`Feed` test, complete, because one `Feed` cannot show the defect S02-C1 found:

```go
func TestASecondFeedSeesADelegationAdoptedOnTheFirst(t *testing.T) {
	ctx := context.Background()
	clk := newFakeClock(time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC))
	hub, project := hubclient.FakeTeamProject(t)
	dl := hub.SeedDelegation(t, "org-a", "us-alice", project, "worker")
	shared := &memInUse{} // one Fleet store; two replicas read it

	// A served the launch and is a follower. B is the feed leader and holds
	// the outbox. Only A calls Adopt. Nothing but the store passes between.
	revA, revB := &recRevoker{now: clk.Now}, &recRevoker{now: clk.Now}
	a := NewFeed(hub, hub, shared, NewCache(), revA, func() bool { return false }, clk.Now)
	b := NewFeed(hub, hub, shared, NewCache(), revB, func() bool { return true }, clk.Now)
	if _, err := a.Adopt(ctx, "org-a", dl); err != nil {
		t.Fatal(err)
	}
	k := Key{OrgID: "org-a", Owner: "us-alice", ProjectID: project, Class: "worker"}
	shared.add(Use{ID: dl, Key: k}) // task 20's agent row, its one transaction

	// B delivers that launch's first write without waiting for a poll tick.
	if _, err := NewClient(b.cache, b, clk.Now).Ctx(ctx, k, "agents.start"); err != nil {
		t.Fatalf("B cannot resolve a delegation A adopted: %v", err)
	}

	hub.RevokeDelegation(t, dl)
	revokedAt := clk.Now()
	for clk.Since() < 2*time.Minute && len(revB.calls) == 0 {
		clk.Advance(PollEvery)
		for _, f := range []*Feed{a, b} {
			if err := f.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			for _, org := range f.Orgs() {
				if _, err := f.Poll(ctx, org, FeedKinds); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if len(revA.calls) != 0 {
		t.Fatalf("the follower called the Revoker: %+v", revA.calls)
	}
	if len(revB.calls) != 1 || revB.calls[0].why != Revoked || revB.calls[0].d.ID != dl {
		t.Fatalf("leader calls %+v; want exactly one Revoked for %s", revB.calls, dl)
	}
	if lag := revB.calls[0].at.Sub(revokedAt); lag > 60*time.Second {
		t.Fatalf("the replica that did not serve the launch revoked after %s; §2.4 item 4 allows 60 s", lag)
	}
}
```

Run: `cd control-plane && go test ./internal/fleet/delegation/ -count=1`. Expected: a compile
failure naming `NewFeed`, `NewCache`, `Rebuild` and the rest.

- [ ] **Step 4: Write `client.go`, `feed.go` and `rebuild.go`** exactly as above. If task 08's
  merged `hubclient` lacks `WithIdempotencyKey`, add it now to `hubclient/client.go` (a context key,
  read where the client sets the header today, falling back to `ids.UUID()`), with one test in
  `hubclient_test.go`, `TestWithIdempotencyKeyOverridesTheFreshKey`, whose mutant is the override
  being ignored. Run step 3's command. Expected: `ok`.
  Commit: `feat(fleet): Fleet as a delegated Hub caller: cache, feed, sweep, gapless rebuild`.

- [ ] **Step 5: Acceptance**

```
cd control-plane
go vet ./internal/fleet/... ./internal/hub/hubclient/...
go test ./internal/fleet/outbox/ ./internal/fleet/delegation/ ./internal/hub/hubclient/... -count=1 -race
grep -rn "time.Sleep" internal/fleet/outbox internal/fleet/delegation
grep -rln "internal/fleet/store" internal/fleet/outbox internal/fleet/delegation
```

Expected: `go vet` prints nothing; three `ok` lines (the `-race` run matters: `Paused` is read from
request goroutines while `RunOnce` writes); both `grep`s print nothing — no test sleeps, and
the second lists exactly `internal/fleet/outbox/store.go` and `internal/fleet/delegation/store.go`:
the two adapters are the only files that know task 10's types.

**What this task does not claim.**
- Nothing here runs a loop. Task 12 owns the goroutines, the 500 ms delivery tick, the 10 s poll and
  sweep, and the advisory locks `fleet.outbox` and `fleet.feed`; this task proves the bodies.
- **`Refresh` tracks exactly what Fleet's own store calls in use, and nothing else.** A delegation
  no live agent row runs under is not tracked and needs no act: every effect the `Revoker` has —
  sessions, sockets, leases — hangs off an agent row. The window between `Adopt` and that row is
  covered by the adopting replica's own cache, and it cannot leak a write to the other replica,
  because task 20 writes the agent row and that launch's first outbox row in **one** transaction
  (`task-20.md:395-400`). If a later task ever enqueues a row before the agent row commits, this
  invariant breaks and the other replica is back to dead-lettering; pin it where that write is.
- The `Revoker`'s effects — sessions stopped or paused, sockets closed, leases withdrawn, cards
  marked — are tasks 20, 23 and 39, and closing a socket held by the **other** replica is task 39's
  fan-out. Task 44 proves member removal end to end against the real Hub.
- The 10-minute pause is the design's **proposed** figure (§2.2 item 3); it is a constant, not a
  measurement. Whether 10 minutes is right is for task 33's run to question.
- It does not check a leaver's Entra account state at renewal or daily (§2.4 item 4, **UNVERIFIED**
  which Graph read KPMG's tenant permits); a disabled account is caught here only by the 24-hour
  lapse.
- Its tests run on task 08's fake. That the real Hub answers route 47 with 401
  `delegation_revoked` / `delegation_expired` for a Fleet caller under a dead delegation is task
  05's rule (`task-01.md:118-122`) and is proven against the real Hub by task 08's conformance rows,
  not here.
- `Classify` treats every 4xx but 408 and 429 as permanent. A Hub that answers a transient
  condition with another 4xx would dead-letter a good write; the dead-letter row keeps the problem
  body so an operator can replay it by hand, and no replay tool is built here.


