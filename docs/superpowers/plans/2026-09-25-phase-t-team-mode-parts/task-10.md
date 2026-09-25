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

#### `migrations/0001_fleet.sql` — complete

```sql
-- Fleet's own state (05-design.md §3.2). No Hub record is stored here except
-- as an outbox payload (ADR-H1:467-474). No column holds free text but the
-- outbox payload, which is the exact Hub request body.
CREATE SCHEMA IF NOT EXISTS fleet;

CREATE TABLE fleet.agents (
  org_id      text        NOT NULL,
  agent_id    text        NOT NULL CHECK (agent_id LIKE 'ag-%'),  -- memstore checks the same prefix
  project_id  text        NOT NULL,
  owner       text        NOT NULL,                 -- u-…
  class       text        NOT NULL CHECK (class IN ('orchestrator','worker','mirror')),
  parent      text        NOT NULL DEFAULT '',      -- ag-… of the Orchestrator, '' for one
  task_id     text        NOT NULL DEFAULT '',
  delegation  text        NOT NULL DEFAULT '',      -- dl-… it was started under
  status      text        NOT NULL CHECK (status IN ('starting','working','waiting','idle','ended')),
  waiting_on  text        NOT NULL DEFAULT '' CHECK (waiting_on IN ('','lease','peer','human')),
  started_at  timestamptz NOT NULL,
  ended_at    timestamptz,
  exit_class  text        NOT NULL DEFAULT ''
              CHECK (exit_class IN ('','done','stopped','failed','access_removed','lapsed')),
  PRIMARY KEY (org_id, agent_id),
  CHECK ((status = 'ended') = (ended_at IS NOT NULL))
);
CREATE INDEX agents_by_project ON fleet.agents (org_id, project_id) WHERE status <> 'ended';

CREATE TABLE fleet.leases (
  org_id        text        NOT NULL,
  lease_id      text        NOT NULL,
  project_id    text        NOT NULL,
  holder        text        NOT NULL,
  owner         text        NOT NULL,
  task_id       text        NOT NULL DEFAULT '',
  globs         text[]      NOT NULL CHECK (cardinality(globs) BETWEEN 1 AND 20),
  mode          text        NOT NULL CHECK (mode IN ('intent','hold','bulk')),
  state         text        NOT NULL CHECK (state IN ('requested','granted','contested',
                  'yielded','released','expired','admin-released')),
  seq           integer     NOT NULL CHECK (seq >= 1),
  tracked_files integer     NOT NULL DEFAULT 0,
  expires_at    timestamptz,                         -- NULL for an intent
  updated_at    timestamptz NOT NULL,
  PRIMARY KEY (org_id, lease_id),
  FOREIGN KEY (org_id, holder) REFERENCES fleet.agents (org_id, agent_id)
);
CREATE INDEX leases_live ON fleet.leases (org_id, project_id)
  WHERE state IN ('requested','granted','contested');

-- The per-path clock a hold lapses on: 30 min after the last edit TO THAT PATH (§3.3).
CREATE TABLE fleet.lease_paths (
  org_id        text        NOT NULL,
  lease_id      text        NOT NULL,
  path          text        NOT NULL CHECK (char_length(path) <= 200),
  last_edit_at  timestamptz NOT NULL,
  PRIMARY KEY (org_id, lease_id, path),
  FOREIGN KEY (org_id, lease_id) REFERENCES fleet.leases (org_id, lease_id) ON DELETE CASCADE
);

CREATE SEQUENCE fleet.outbox_seq;
CREATE TABLE fleet.outbox (
  seq           bigint      NOT NULL DEFAULT nextval('fleet.outbox_seq'),
  id            text        PRIMARY KEY,             -- also the Idempotency-Key
  org_id        text        NOT NULL,
  owner         text        NOT NULL CHECK (owner <> ''),   -- Fleet never writes as itself
  project_id    text        NOT NULL,
  class         text        NOT NULL CHECK (class IN ('orchestrator','worker')),
  op            text        NOT NULL,
  subject       text        NOT NULL DEFAULT '',
  payload       bytea       NOT NULL,                -- the exact bytes; jsonb would reorder keys
  created_at    timestamptz NOT NULL,
  state         text        NOT NULL CHECK (state IN ('pending','sent','dead')),
  attempts      integer     NOT NULL DEFAULT 0,
  next_at       timestamptz NOT NULL,
  failing_since timestamptz,
  problem       jsonb,
  sent_at       timestamptz
);
CREATE UNIQUE INDEX outbox_order ON fleet.outbox (org_id, seq);
CREATE INDEX outbox_pending ON fleet.outbox (org_id, seq) WHERE state = 'pending';

-- The per-Orchestrator queue (task 26, task-26.md:758-799, adopted here so that
-- task 26 needs no migration). No text column exists.
CREATE TABLE fleet.turn_events (
  org_id        text        NOT NULL,
  event_id      text        NOT NULL,
  orchestrator  text        NOT NULL,
  kind          text        NOT NULL CHECK (kind IN ('human.message','peer.message',
                  'conflict.opened','worker.done','worker.blocked','worker.assumed',
                  'lease.contested','lease.expiring','escalation.answered')),
  ref           text        NOT NULL,
  author        text        NOT NULL DEFAULT '',     -- u-… of a human.message, else ''
  at            timestamptz NOT NULL,
  turn_id       text,                                -- NULL until batched
  retried       boolean     NOT NULL DEFAULT false,
  PRIMARY KEY (org_id, event_id)
);
CREATE INDEX turn_events_open ON fleet.turn_events (orchestrator, at) WHERE turn_id IS NULL;

-- At most one live claim per Orchestrator: the primary key IS the two-replica guard.
CREATE TABLE fleet.turn_claims (
  orchestrator  text        PRIMARY KEY,
  org_id        text        NOT NULL,
  turn_id       text        NOT NULL UNIQUE,
  nonce         text        NOT NULL,
  origin        text        NOT NULL CHECK (origin IN ('human','fleet')),
  event_ids     text[]      NOT NULL,
  issued        text[]      NOT NULL DEFAULT '{}',
  mode          text        NOT NULL CHECK (mode IN ('new','resume')),
  session_id    uuid        NOT NULL,
  sandbox_id    text        NOT NULL,
  began         timestamptz NOT NULL,
  until         timestamptz NOT NULL
);

CREATE TABLE fleet.orch_sessions (
  orchestrator  text        PRIMARY KEY,
  org_id        text        NOT NULL,
  session_id    uuid,
  sandbox_id    text        NOT NULL DEFAULT '',
  turns         integer     NOT NULL DEFAULT 0
);

-- OD-23: versioned, append-only managed settings per org (project '') and project.
CREATE TABLE fleet.policy_versions (
  org_id      text        NOT NULL,
  project_id  text        NOT NULL DEFAULT '',
  class       text        NOT NULL CHECK (class IN ('orchestrator','worker','mirror')),
  version     integer     NOT NULL CHECK (version >= 1),
  digest      text        NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
  body        bytea       NOT NULL,
  created_by  text        NOT NULL,
  created_at  timestamptz NOT NULL,
  PRIMARY KEY (org_id, project_id, class, version)
);

-- Task 37: one row per peer-triggered turn, counted over a window.
CREATE TABLE fleet.counterpart_turns (
  org_id       text        NOT NULL,
  owner        text        NOT NULL,
  counterpart  text        NOT NULL,
  at           timestamptz NOT NULL
);
CREATE INDEX counterpart_turns_window ON fleet.counterpart_turns (org_id, owner, counterpart, at);

-- Append-only, by grant: Fleet's runtime role may not rewrite a policy version.
REVOKE UPDATE, DELETE ON fleet.policy_versions FROM PUBLIC;
```

**Three choices worth naming.** `payload` is `bytea`, not `jsonb`: the Hub compares request
bodies byte for byte under one idempotency key (`task-11.md:161-162`), and `jsonb` normalises key
order and whitespace, so a retried row would be a different body. `turn_events.author` is one
`u-…` column, not task 26's `author_tid`/`author_oid` pair (`task-26.md:765-766`), because
`model.Event.Author` is a `model.Owner` and task 26's own queue compares it with `a.Owner`
(`task-26.md:431`); task 26's `pgstore` code follows this. And the `REVOKE` is the grant half of
append-only: the runtime role Fleet connects as (task 32) is granted `INSERT, SELECT` only on
`policy_versions`; **which role that is, and whether it is not the migration owner, is
UNVERIFIED** until task 32 writes the chart and the database identity.
