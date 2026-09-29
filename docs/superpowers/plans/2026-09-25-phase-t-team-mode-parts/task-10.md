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
- `control-plane/internal/fleet/store/pgstore/pgstore.go`: `Open`, `WithEntraAuth`, `Pool`, `Migrate`, `RequireSchema` (b07), the methods (new)
- `control-plane/internal/fleet/store/pgstore/pgstore_test.go`: runs the suite on Postgres (new)
- `control-plane/internal/fleet/store/memstore/memstore_test.go`: runs the suite in memory (new)
- `control-plane/internal/fleet/store/storetest/suite.go`: `Run` and its rows (new)
- `control-plane/internal/fleet/store/migrations/0001_fleet.sql`, `migrations/embed.go` (new)
- `control-plane/go.mod`, `go.sum`: `github.com/jackc/pgx/v5 v5.7.5` (modify; see below)

The skeleton's file map names `store/{store.go,memstore.go,pgstore.go}`; this part puts the two
implementations in sub-packages, because task 12 imports `pgstore.Open` and `pgstore.Migrate` by
that package name (`task-12.md:108`), and a `memstore` sub-package keeps `pgx` out of every test
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
  `store.ErrConflict`; `memstore.New`; `pgstore.Open` with `pgstore.WithEntraAuth`, `(*pgstore.Store).Pool`,
  `pgstore.Migrate`, `pgstore.RequireSchema` (b07, G32-3/G32-4); `storetest.Run`.
- Required by: 11 (outbox, delegations in use), 12 (`Open`, `WithEntraAuth`, `Migrate`, `RequireSchema`, `Pool`), 14 (policy
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
	// fields, for task 11's rebuild and refresh (task-11.md:853-868). It is the
	// set the feed ACTS on, so it stays live-only: a delegation no live agent
	// runs under has nothing to stop and must never be resumed.
	DelegationsInUse(ctx context.Context) ([]DelegationUse, error)
	// DelegationForKey answers the outbox's question, which is not the feed's:
	// "which dl-… did the work queued for this (org, owner, project, class) go
	// out under?" It is the most recently started agent row with that key,
	// ENDED ROWS INCLUDED — `agents.end` is enqueued at the moment the agent
	// ends, and a restart delivers a backlog whose agents are all ended, so a
	// live-only read would dead-letter both (task-11.md:1096-1118). Not found
	// is (DelegationUse{}, false, nil), never an error.
	DelegationForKey(ctx context.Context, org, owner, project string, class model.Class) (DelegationUse, bool, error)

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

// DelegationUse is task 11's delegation.Use flattened (task-11.md:386-392, :847-851).
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
-- DelegationForKey: the latest agent row on a delegation key, ended ones included,
-- so no partial predicate here. ORDER BY started_at DESC LIMIT 1 rides this index.
CREATE INDEX agents_by_delegation_key
  ON fleet.agents (org_id, owner, project_id, class, started_at DESC);

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

#### `pgstore` — the three places the database decides

```go
// control-plane/internal/fleet/store/pgstore/pgstore.go (excerpts)
// Open connects and pings. Task 12 calls it once at start (task-12.md:108). Its one Option, WithEntraAuth (b07, G32-4, the Hub's pattern, hub task-08.md:142-172): on, pgx's BeforeConnect sets each NEW
// connection's password to ts.Token(ctx, DatabaseScope) and MaxConnLifetime is 45 min; off, Open is exactly as before. DatabaseScope = "https://ossrdbms-aad.database.windows.net/.default".
func Open(ctx context.Context, dsn string, opts ...Option) (*Store, error)
func WithEntraAuth(on bool, ts interface{ Token(context.Context, string) (string, error) }) Option // fleetd's assertion source is ts
func (s *Store) Pool() *pgxpool.Pool

// Migrate applies migrations/*.sql in name order, each in its own transaction,
// under pg_advisory_xact_lock(hashtext('fleet.migrate')) so two replicas starting
// together apply each file once. Applied names are kept in fleet.schema_migrations,
// which Migrate creates first. A file already applied is skipped by name; a file
// whose recorded sha256 differs from the embedded one fails start, naming it.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error
func RequireSchema(ctx context.Context, pool *pgxpool.Pool) error // b07, G32-3: Migrate's read-only half: nil only when every embedded file is in fleet.schema_migrations with its sha256, else an error naming the first missing or differing one; it writes nothing
const claimSQL = `
INSERT INTO fleet.turn_claims (orchestrator, org_id, turn_id, nonce, origin, event_ids,
  issued, mode, session_id, sandbox_id, began, until)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (orchestrator) DO NOTHING`

const batchSQL = `
UPDATE fleet.turn_events SET turn_id = $1
WHERE org_id = $2 AND orchestrator = $3 AND event_id = ANY($4) AND turn_id IS NULL`

// ClaimTurn: zero rows inserted is ok == false (another replica holds the claim).
// Then every named event must be batched by this statement; fewer rows means an
// event was already batched or never queued, and the whole Tx is ErrConflict.
func (t *tx) ClaimTurn(ctx context.Context, c store.TurnClaim) (bool, error)

const putLeaseSQL = `
INSERT INTO fleet.leases AS l (org_id, lease_id, project_id, holder, owner, task_id, globs,
  mode, state, seq, tracked_files, expires_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,1,$10,$11,$12)
ON CONFLICT (org_id, lease_id) DO UPDATE SET
  globs = EXCLUDED.globs, mode = EXCLUDED.mode, state = EXCLUDED.state,
  tracked_files = EXCLUDED.tracked_files, expires_at = EXCLUDED.expires_at,
  updated_at = EXCLUDED.updated_at, seq = l.seq + 1
WHERE l.state NOT IN ('yielded','released','expired','admin-released')
  AND l.holder = EXCLUDED.holder
RETURNING seq`

// PutLease: no row returned means the lease was terminal or held by another agent;
// both are ErrConflict. The returned seq is what the outbox row must carry
// (task 06's gapless rule at the Hub).
func (t *tx) PutLease(ctx context.Context, l model.Lease) (model.Lease, error)
```

The outbox's order is `nextval('fleet.outbox_seq')`, assigned by the database at insert, so two
replicas enqueueing for one org cannot give two rows one `seq`. `OutboxHeads` is `SELECT DISTINCT
ON (org_id) … WHERE state = 'pending' AND next_at <= $1 ORDER BY org_id, seq`: one head per org,
the lowest `seq`, which is task 11's in-order delivery (`task-11.md:187-188`).

**`memstore`** is one `sync.Mutex` around maps, and `Tx` copies the maps, runs `fn` on the copy,
and swaps them in only when `fn` returns nil, so a failed `Tx` leaves no partial write, exactly as
Postgres rolls back. It enforces every `CHECK` of the migration that a Go caller could violate
(class, status, mode, state, the `ag-` prefix, 1 to 20 globs, the digest grammar), so a test that
passes on memory fails the same way it would on Postgres.

- [ ] **Step 1: Write the failing suite**

`storetest.Run(t *testing.T, open func(t *testing.T) store.Store)` runs every row as
`t.Run("<row>")` on a fresh store. `memstore_test.go` calls it with `memstore.New`;
`pgstore_test.go` calls it with a fresh schema per row on the database named by
`PANACEA_APP_FLEET_TEST_PG_DSN` (the variable task 12 reuses, `task-12.md:1101`), and **skips the
whole run, saying so, when it is unset**. It also holds two pgstore-only tests (b07, G32-3/G32-4): `TestRequireSchemaNamesTheGap` (a fresh schema → an error naming `0001_fleet.sql`; after `Migrate` → nil; a tampered recorded sha256 → an error naming the file; mutant: a check of the row count alone) and `TestEntraAuthSetsEachPassword` (no database needed: over a fake token source, the pool config's `BeforeConnect` sets `Password` to a fresh `Token(ctx, DatabaseScope)` per connection and `MaxConnLifetime` is 45 min; off, `BeforeConnect` is nil; mutant: a token read once at `Open`, which expires under a long-lived pool).

| Row | Asserts | Mutant it kills |
|---|---|---|
| `ClaimTurnOneWinner` | 20 goroutines call `ClaimTurn` for one Orchestrator with distinct turn ids over the same events; exactly one `ok`, the rest `ok == false`, no error; the events carry the winner's turn id | a read-then-insert claim (the two-replica bug) |
| `ClaimTurnNeedsUnbatchedEvents` | claim over an event already batched → `ErrConflict`, and the claim row does not exist after | the batch `UPDATE`'s row count ignored |
| `DropClaimDoneDeletesNotDoneUnbatches` | after `DropClaim(done)`, `PendingEvents` is empty; after `DropClaim(!done)`, the events are pending again with `Retried` true | un-batching on a completed turn, so a turn replays |
| `LapsedClaimStillBlocks` | a claim with `Until` in the past still makes a second `ClaimTurn` `ok == false` | overwriting a lapsed claim, which orphans its batched events (`task-26.md:803-806`) |
| `LeaseSeqIncrements` | `PutLease` new → seq 1; two updates → 2, 3; the returned lease carries it | seq taken from the caller's struct |
| `LeaseTerminalIsFinal` | after `released`, `PutLease` → `ErrConflict`, stored state unchanged | the `WHERE` terminal guard dropped, so Fleet writes an event the Hub will refuse |
| `LeaseHolderCannotChange` | `PutLease` of an existing id with another holder → `ErrConflict` | a lease silently transferred |
| `LeasePathsCascade` | `TouchLeasePath` twice for one path keeps the later time; paths of a deleted lease vanish | a per-path clock that keeps the first edit |
| `OutboxOrderPerOrg` | 3 rows for org A, 2 for B, interleaved; `OutboxHeads` returns one per org, each the lowest `seq`; after `MarkSent` of A's head, A's next | a global head, which lets one org's failing row stall another's |
| `OutboxPayloadBytesExact` | a payload `{"b":1, "a":2}` with a space comes back byte for byte | `jsonb` storage |
| `OutboxOwnerRequired` | `AppendOutbox` with `Owner` "" → error | a Fleet-as-itself write reaching the Hub |
| `OutboxFailingSinceSurvivesReopen` | `MarkRetry` with `failingSince`; reopen (pg: a new pool; mem: skipped); `OutboxFailingSince` has it | the gate's clock held in memory only (task 11) |
| `TxRollsBackAll` | a `Tx` that puts an agent, appends an outbox row and then returns an error leaves neither | memstore mutating in place |
| `AgentStatusAndEnd` | `SetStatus(waiting, human)` then `EndAgent(lapsed)`; `Agents` omits ended ones; `Agent` still finds it with `Exit = lapsed` | ended agents listed as live |
| `DelegationsInUseLiveOnly` | two live agents under one `dl-…`, one ended under another; the result is the one live `dl-…` once | ended agents keep a delegation "in use", so rebuild renews a dead one |
| `DelegationForKeyIncludesEndedAgents` | the same fixture: `DelegationForKey` of the ended agent's key answers its `dl-…`, `true` — the read the outbox uses is **not** the read the feed acts on | one live-only read serving both, so an ended agent's `agents.end` dead-letters on the other replica (task-11.md:1096-1118) |
| `DelegationForKeyIsTheLatestAgent` | two agents on one key, started 10 min apart under `dl-1` then `dl-2`, the later one ended; the answer is `dl-2` | a first-row-wins read that sends a renewed key's backlog under the superseded `dl-…` |
| `DelegationForKeyMissIsNotAnError` | a key with no agent row → `(DelegationUse{}, false, nil)` | a `sql.ErrNoRows` escaping as an error, which task 11 would classify transient and retry forever |
| `PolicyVersionsAppendOnly` | `PutPolicy` v1, v2; `CurrentPolicy` is v2; `PutPolicy` v2 again → `ErrConflict`; project `""` and `pj-…` are separate scopes | versions overwritten |
| `CounterpartWindow` | 3 turns at t−70 min, t−30, t−10; `CounterpartTurns(since t−60 min)` = 2 | a count with no window |
| `EventKindChecked` | `AppendEvent` with kind `turn.begin` → error | a wire event queued as a model event |

The central row in full:

```go
// control-plane/internal/fleet/store/storetest/suite.go (excerpt)

// Mutant killed: ClaimTurn implemented as "SELECT claim; if none, INSERT", which two
// replicas pass through together. The primary key on orchestrator is the guard.
func claimTurnOneWinner(t *testing.T, s store.Store) {
	ctx := context.Background()
	orch := seedOrchestrator(t, s) // an agent row plus three queued events
	evs := pendingIDs(t, s, orch)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := s.Tx(ctx, func(tx store.Tx) error {
				ok, err := tx.ClaimTurn(ctx, claim(orch, fmt.Sprintf("ot-%016d", i), evs))
				if ok {
					wins.Add(1)
				}
				return err
			})
			if err != nil && !errors.Is(err, store.ErrConflict) {
				t.Errorf("claim %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if got := wins.Load(); got != 1 {
		t.Fatalf("%d winners, want exactly 1", got)
	}
}
```

A loser that reaches the batch `UPDATE` after the winner committed finds zero rows and returns
`ErrConflict`; one that loses at the `INSERT` returns `ok == false` and nil. Both are "not the
winner", which is why the row accepts either.

- [ ] **Step 2: Run the suite to verify it fails**

```bash
cd control-plane && go test ./internal/fleet/store/... -count=1
```

Expected: a build failure (`undefined: memstore.New`, `undefined: storetest.Run`).

- [ ] **Step 3: Write `store.go`, the migration, `memstore`, then `pgstore`**

Add the dependency first, from `control-plane/`: `go get github.com/jackc/pgx/v5@v5.7.5`, then
`go mod tidy`. Write `memstore` until every row passes in memory, then `pgstore` against a local
Postgres 16 (`docker run --rm -e POSTGRES_PASSWORD=x -p 5439:5432 postgres:16`, then
`PANACEA_APP_FLEET_TEST_PG_DSN=postgres://postgres:x@127.0.0.1:5439/postgres?sslmode=disable`).

- [ ] **Step 4: Run the suite to verify it passes, in memory and on Postgres**

```bash
cd control-plane && go test ./internal/fleet/store/... -count=1 -race -v
```

Expected: every row `--- PASS` under `TestMemstore`, and under `TestPgstore` when the DSN is set;
without it, one line `--- SKIP: TestPgstore (PANACEA_APP_FLEET_TEST_PG_DSN unset)`. `ClaimTurnOneWinner`
failing only on Postgres with 2 winners means the claim was split into a `SELECT` and an `INSERT`;
failing on memory with 0 winners means `Tx`'s swap drops a successful claim.

- [ ] **Step 5: Vet, format, the whole module**

```bash
gofmt -l control-plane/internal/fleet/store
cd control-plane && go vet ./... && go test ./... -count=1
```

- [ ] **Step 6: Commit**

`.superpowers/sdd/team/msg/t10.txt` (the database decides every race; no Hub record but the
outbox payload), ending `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`:

```bash
git add control-plane/internal/fleet/store control-plane/go.mod control-plane/go.sum
git commit -F .superpowers/sdd/team/msg/t10.txt -- control-plane/internal/fleet/store control-plane/go.mod control-plane/go.sum
```

#### Acceptance

1. Step 4 passes in memory; on a real Postgres it passes too, or the run records the skip and
   the Postgres half is deferred to task 33, where Fleet's database first exists.
2. `grep -n "hubapi" -r control-plane/internal/fleet/store` prints nothing: no Hub type is stored.
3. `grep -n "pgx/v5 " control-plane/go.mod` prints `github.com/jackc/pgx/v5 v5.7.5`.
4. `ClaimTurnOneWinner` passes 50 times running: `go test ./internal/fleet/store/... -run
   'TestMemstore/ClaimTurnOneWinner|TestPgstore/ClaimTurnOneWinner' -count=50`.

#### Handoffs to tasks built later

- **Task 11** converts at its boundary: `outbox.Item` ⇄ `store.OutboxRow` (identical fields, `Op`
  and `State` typed there), and `store.DelegationUse` → `delegation.Use{ID, Key{…}}`. Its part
  says the store imports `outbox` (`task-11.md:85`); built in this order it cannot, so task 11
  either adds the two converters or, once `outbox` exists, changes these signatures to its types
  in these files, keeping every suite row.
- **Task 26** composes `EndTurn` and `LoseTurn` from `DropClaim`, `AppendOutbox` and a session
  write it adds, and needs no migration: its three tables are in `0001` with the one column change
  above.
- **Tasks 19, 28, 29** add `0002` onward. Task 19 and task 26 both named `0002`
  (`task-19.md:69`, `task-26.md:754`); with the turn tables here, only task 19 needs it.

#### What this task does not claim

- **Not row-level security.** Fleet's schema has one tenant, Fleet; every method takes an org,
  and no request reaches SQL without one. Hub-style RLS is not added (**a decision**, reversible
  by one migration).
- **Not the runtime role or the Azure Postgres token flow** (task 32; `task-12.md:1294`).
- **Not the thread working state**, which task 28's migration owns.
- **Not a server.** Nothing here listens; task 12 opens it.

#### Gaps found in the skeleton and in written parts

1. **Open-thread working state** is in task 10's scope and in task 28's written DDL; this part
   leaves it to task 28.
2. **`author_tid`/`author_oid`** in task 26's DDL disagree with its own `model.Owner` author; this
   part stores one `author` (`u-…`).
3. **Migration `0002` is claimed twice** (tasks 19 and 26); resolved by absorbing task 26's tables.
4. **Task 11's import direction** (store imports outbox) is impossible in build order; see
   "Handoffs".
5. **Task 24's `reports` table** (`task-24.md:241`) is required of task 10 but not in its scope;
   not added here. `panacea report` writes an event (`worker.*`) and the agent's status, which
   this schema holds; if task 24 needs the report text kept, that is a Hub or task 24 decision.