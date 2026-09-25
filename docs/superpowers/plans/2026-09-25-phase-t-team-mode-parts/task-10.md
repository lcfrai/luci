### Task 10: Fleet's Postgres schema and `Store`

**Slice TM1-α. Depends on: 09. About 3 hours. Seams: none defined; every Fleet task reads it.**

Fleet's facts about now, kept where two replicas can share them. `cp/internal/fleet/store` holds
the `Store` interface, an in-memory `memstore`, a Postgres `pgstore` over `pgx/v5`, one
behaviour suite both must pass (`storetest.Run`, phase 2's `registrytest` shape,
`control-plane/internal/registry/registrytest/registrytest.go:1-18`), and the first migration,
`0001_fleet.sql`, in its own schema `fleet`.

**What lives here, and what never does** (`05-design.md` §3.2, Global constraints 1-2):

| Here (Fleet state) | Never here |
|---|---|
| agents and their status; leases and each lease path's last-edit time; the turn queue and its claim; the Hub outbox; policy versions; per-counterpart counters | any Hub record (task, conflict, thread, message, escalation, directive, human message, note) **except as an outbox payload** (`ADR-H1:467-474`) |

Open-thread working state is Fleet state too, but task 28 has already written its tables
(`task-28.md:640-664`, `fleet.threads`, `fleet.messages`, `fleet.message_reads`) in its own
migration. This task does not create them; see "Reconciled with the parts already written".

**The one thing this task must not get wrong is the two-replica property.** Fleet runs two
replicas (task 32). Every state change that two replicas could race on is decided **in the
database**, never by reading and then writing from Go: `ClaimTurn` is one `INSERT … ON CONFLICT
DO NOTHING`; a lease's `seq` is incremented by the `UPDATE` that changes its state; an outbox
row's order is a sequence. `memstore` models the same with one mutex, and the suite runs the
race against both. A property that holds in `memstore` and not `pgstore` is exactly the bug this
suite exists to catch.

**Files:**
- `control-plane/internal/fleet/store/store.go`: `Store`, `Tx`, the row types (new)
- `control-plane/internal/fleet/store/memstore/memstore.go` (new)
- `control-plane/internal/fleet/store/pgstore/pgstore.go`: `Open`, `Pool`, `Migrate`, the methods (new)
- `control-plane/internal/fleet/store/pgstore/pgstore_test.go`: runs the suite on Postgres (new)
- `control-plane/internal/fleet/store/memstore/memstore_test.go`: runs the suite in memory (new)
- `control-plane/internal/fleet/store/storetest/suite.go`: `Run` and its rows (new)
- `control-plane/internal/fleet/store/migrations/0001_fleet.sql`, `migrations/embed.go` (new)
- `control-plane/go.mod`, `go.sum`: `github.com/jackc/pgx/v5 v5.7.5` (modify; see below)

The skeleton's file map names `store/{store.go,memstore.go,pgstore.go}`; this part puts the two
implementations in sub-packages, because task 12 imports `pgstore.Open` and `pgstore.Migrate` by
that package name (`task-12.md:76`), and a `memstore` sub-package keeps `pgx` out of every test
binary that only needs memory.

**The `pgx` pin.** The Hub pins `github.com/jackc/pgx/v5 v5.7.5`
(`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-06.md:20`, Hub
task 06, **planned**; `feat/hub`'s `control-plane/go.mod` has no `pgx` line yet, read 2026-09-25).
Both land in the one `control-plane` module, so they must be one version: this task pins
`v5.7.5`. If the Hub's merge arrives with another, the later merge takes the Hub's and this
task's tests rerun. `go 1.23` does not move.

**Interfaces:**
- Consumes: task 09's `model` (`AgentID`, `Owner`, `Class`, `Agent`, `Status`, `WaitingOn`,
  `ExitClass`, `Lease`, `LeaseMode`, `LeaseState`, `Event`, `EventKind`, `EventKinds`,
  `TurnOrigin`).
- Produces: `store.Store`, `store.Tx`, `store.OutboxRow`, `store.QueuedEvent`,
  `store.TurnClaim`, `store.DelegationUse`, `store.PolicyVersion`, `store.ErrNotFound`,
  `store.ErrConflict`; `memstore.New`; `pgstore.Open`, `(*pgstore.Store).Pool`,
  `pgstore.Migrate`; `storetest.Run`.
- Required by: 11 (outbox, delegations in use), 12 (`Open`, `Migrate`, `Pool`), 14 (policy
  versions), 19 (agent lookup), 20 (agent rows), 22 (agents, leases), 23 (lease writes), 24, 25,
  26 (the queue and the claim), 35 (lease paths), 37 (counterpart counters); and tasks 19, 26, 28,
  29, which add their own tables in later migrations.

#### `store.go` — complete

```go
// Package store is Fleet's own state (05-design.md §3.2): facts about now, shared
// by two replicas. It holds no Hub record except as an outbox payload
// (ADR-H1:467-474). Every method scopes by org; none reads an org from context.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

var (
	ErrNotFound = errors.New("store: not found")
	// ErrConflict is a write the database refused on a uniqueness or state rule:
	// a second live claim, a reused id, a lease seq that is not last+1.
	ErrConflict = errors.New("store: conflict")
)

// Store is the whole of Fleet's schema through one interface. Reads outside Tx see
// committed state; every write goes through Tx.
type Store interface {
	Tx(ctx context.Context, fn func(Tx) error) error

	Agent(ctx context.Context, org string, id model.AgentID) (model.Agent, bool, error)
	Agents(ctx context.Context, org, project string) ([]model.Agent, error)
	Leases(ctx context.Context, org, project string) ([]model.Lease, error)
	// DelegationsInUse is every dl-… a live agent was started under, with its key
	// fields, for task 11's rebuild (task-11.md:761-769).
	DelegationsInUse(ctx context.Context) ([]DelegationUse, error)

	// The outbox queue (task 11's outbox.Queue, task-11.md:184-195).
	OutboxHeads(ctx context.Context, now time.Time, limit int) ([]OutboxRow, error)
	OutboxMarkSent(ctx context.Context, id string, at time.Time) error
	OutboxMarkRetry(ctx context.Context, id string, attempts int, next, failingSince time.Time) error
	OutboxMarkDead(ctx context.Context, id string, problem json.RawMessage) error
	OutboxFailingSince(ctx context.Context) (map[string]time.Time, error)
	OutboxDeleteSentBefore(ctx context.Context, t time.Time) (int, error)

	// The turn queue (task 26's turns.Store, task-26.md:208-235, first half).
	PendingEvents(ctx context.Context, orch model.AgentID) ([]QueuedEvent, error)
	ClaimedTurn(ctx context.Context, orch model.AgentID) (TurnClaim, bool, error)

	// Policy versions (task 14 writes; task 20 reads the current one at start).
	CurrentPolicy(ctx context.Context, org, project string, class model.Class) (PolicyVersion, bool, error)

	// Per-counterpart counters (task 37): turns one owner's Orchestrator spent on
	// another owner's messages in the window starting at since.
	CounterpartTurns(ctx context.Context, org string, owner, counterpart model.Owner, since time.Time) (int, error)
}

// Tx is one transaction. Every Fleet state change and the outbox row that
// reports it to the Hub are written in one Tx (task 11's rule, task-11.md:10).
type Tx interface {
	PutAgent(ctx context.Context, a model.Agent) error
	SetStatus(ctx context.Context, org string, id model.AgentID, s model.Status, w model.WaitingOn) error
	EndAgent(ctx context.Context, org string, id model.AgentID, exit model.ExitClass, at time.Time) error

	// PutLease inserts or updates; on update the store sets Seq = previous+1 and
	// returns the stored lease, so the caller's outbox row carries the true seq.
	PutLease(ctx context.Context, l model.Lease) (model.Lease, error)
	Lease(ctx context.Context, org, id string) (model.Lease, error)
	LiveLeases(ctx context.Context, org, project string) ([]model.Lease, error)
	// TouchLeasePath records an observed edit to one path under a hold (task 35).
	TouchLeasePath(ctx context.Context, org, leaseID, path string, at time.Time) error
	LeasePathEdits(ctx context.Context, org, leaseID string) (map[string]time.Time, error)

	AppendOutbox(ctx context.Context, r OutboxRow) error

	AppendEvent(ctx context.Context, e QueuedEvent) error
	// ClaimTurn is the two-replica guard: iff no claim row exists for c.Orch, insert
	// c and mark c.EventIDs batched under c.TurnID; ok false means another won.
	ClaimTurn(ctx context.Context, c TurnClaim) (ok bool, err error)
	// DropClaim deletes the claim and, when done, its batched events; when not
	// done, un-batches them (task 26's EndTurn and LoseTurn compose on it).
	DropClaim(ctx context.Context, orch model.AgentID, turnID string, done bool) error

	PutPolicy(ctx context.Context, p PolicyVersion) error
	AddCounterpartTurn(ctx context.Context, org string, owner, counterpart model.Owner, at time.Time) error
}

// OutboxRow is task 11's outbox.Item field for field (task-11.md:163-182), with Op
// and State as strings, because this package is built before task 11's and must
// not import it. Task 11 converts at its boundary (see "Handoffs").
type OutboxRow struct {
	Seq          int64 // assigned by the store: the delivery order within an org
	ID           string
	OrgID        string
	Owner        model.Owner
	ProjectID    string
	Class        model.Class
	Op           string
	Subject      string
	Payload      json.RawMessage // stored and returned byte for byte, never re-marshalled
	CreatedAt    time.Time
	State        string // pending | sent | dead
	Attempts     int
	NextAt       time.Time
	FailingSince time.Time
	Problem      json.RawMessage
}

// QueuedEvent is a model.Event in the queue, with its batch mark.
type QueuedEvent struct {
	model.Event
	Org     string
	TurnID  string // "" until batched
	Retried bool   // un-batched once by a lost turn; the next loss drops it (task 26)
}

// TurnClaim is task 26's ClaimRow (task-26.md:238-251), field for field.
type TurnClaim struct {
	Org       string
	Orch      model.AgentID
	TurnID    string
	Nonce     string
	Origin    model.TurnOrigin
	EventIDs  []string
	Issued    []string
	Mode      string // new | resume
	SessionID string // a canonical v4 uuid
	SandboxID string
	Began     time.Time
	Until     time.Time
}

// DelegationUse is task 11's delegation.Use flattened (task-11.md:375-380, :761-764).
type DelegationUse struct {
	ID        string // dl-…
	OrgID     string
	Owner     model.Owner
	ProjectID string
	Class     model.Class
}

// PolicyVersion is one append-only version of OD-23's managed settings for one
// scope (task 13's layers, task 14's store). Project "" is the org layer.
type PolicyVersion struct {
	Org       string
	Project   string
	Class     model.Class
	Version   int    // 1, 2, 3 … per (org, project, class)
	Digest    string // sha256:<64 hex> of the rendered file (task 13's digest)
	Body      json.RawMessage
	CreatedBy model.Owner
	CreatedAt time.Time
}
```

`TurnClaim` carries `Org` where task 26's `ClaimRow` does not: every row in this schema is keyed
by org, and task 26's three-line adapter (`task-26.md:107-109`) fills it from the agent.
