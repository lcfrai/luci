### Task 26: The turn queue, turn origin and the fixed template

This task owns seam **ST.5**: how an Orchestrator's turn starts, what its input is, how it ends, and
which session id it runs under. An Orchestrator is not a long-lived chat. It is a queue of typed
events in Fleet's Postgres, drained one batch at a time; each batch is one turn of a headless child
started by the runner (task 27). Three properties carry the design and each is pinned by a test,
not by review.

1. **Thread events are never paced by the batcher** (`05-design.md` §4.2, `06` I8). A peer message
   and a conflict opened are deliverable at once; every other kind waits up to 60 seconds for
   company, so a burst of worker reports costs one turn, not five.
2. **Origin is computed, never claimed** (§7.2). A batch's origin is `human` only if it holds a
   message from the Orchestrator's **own** human; otherwise `fleet`. Task 31's taint rule reads
   this value in Fleet's verb handler; nothing in the prompt carries it.
3. **The turn input carries no event text** (§4.2, §7.1). It is a product sentence rendered from
   one integer. Peer text reaches the model only as the output of `panacea inbox` (task 28), quoted
   in the fixed envelope (G4). The function that renders the input takes an `int` and nothing else,
   so the property is structural, and the test proves the structure.

Four decisions the scope left open, made here so nobody re-derives them.

**(a) The session id is Fleet state, not runner state.** Fleet mints the uuid (`ids.UUID()`, phase
2's `control-plane/internal/ids/ids.go:46`) and sends it in `turn.begin` with a `mode` of `new` or
`resume`. The runner never chooses. The rule is one pure function, `SessionFor`: `resume` iff
Fleet holds a uuid for this Orchestrator **and** the sandbox that ran the last turn is the one that
will run this one **and** no rotation is due (task 40 sets that flag; here it is always false).
Anything else is `new` with a fresh uuid. That is what makes pod loss harmless (§4.2 "Pod loss"):
a relaunched pod has a new sandbox id, so its first turn is a new session seeded from typed state,
never an attempt to `--resume` a transcript that died with the old `emptyDir`.

**(b) Token use comes from the gateway, not from the runner.** The scope says `turn.end` "comes
back with the turn's token use". The runner cannot know it without reading fields out of the CLI's
output, which OD-8 forbids (`PROGRAM.md` cross-cutting constraints), and task 27 picks
`--output-format text` for exactly that reason. Task 20 already mints a **fresh gateway key per
Orchestrator turn** (`tokens.go`), so the turn's tokens are that key's metered usage. This task
reads it through a one-method `TokenMeter` interface when it writes the turn record; `turn.end`
itself carries exit, duration and (for a human-origin turn) the bounded reply. If the meter is
unavailable, the record is written with `tokens` absent, never zero. This is a gap in the skeleton's
wording, listed at the end.

**(c) `turn.end` carries a nonce the child never sees.** If the Orchestrator's own Claude session
could send `turn.end`, it could end its turn early, dropping the turn key and leaving work
unrecorded. The pod closes the channel: task 16's decision 10 answers `events.wait` (which
carries `turn.begin`) only to the PTY's own pid, checked with `SO_PEERCRED`, and task 27 requires
the same check on `turn.end`, so the harness (the runner's child) can neither read a nonce nor
send an end. The nonce is the second lock, in Fleet, where it does not depend on the pod. `turn.begin` carries a 43-character `nonce`
(`ids.Token()`, `ids.go:30`); the runner keeps it in memory, never in the child's argv or
environment (task 27 tests that), and Fleet accepts a `turn.end` only for the claimed turn **with**
its nonce, once, compared in constant time. A wrong nonce is `turn_mismatch` and changes nothing.

**(d) A turn has a deadline, and a lost turn re-queues its events once.** A turn that never ends
(pod killed mid-turn, stream gone) would otherwise hold `ClaimTurn` forever and silence the
Orchestrator. The claim carries `until = now + TurnTimeout` (default 10 minutes,
`PANACEA_APP_FLEET_TURN_TIMEOUT`, parsed by task 12's `config.go`; **this task adds the variable**).
When the claim lapses, or task 19 reports the stream closed for good, the turn ends with
`exit: "lost"`, and its events go back on the queue **once**, marked `retried`. A second loss of
the same events drops them from the queue and writes an `ot-…` record naming them, so no event is
silently lost and none loops. Dropping a queue entry drops only the **nudge**: the fact behind it
is a Hub record (a message, a conflict) or Fleet state (a lease), and the next turn's `panacea
inbox` or `panacea team` still shows it.

**Files:**
- `control-plane/internal/fleet/turns/queue.go` — `Queue`: enqueue with the own-human check, the
  readiness rule, batch selection, lost-turn re-queue (new)
- `control-plane/internal/fleet/turns/origin.go` — `Origin`, `OriginOf(batch, owner)` (new)
- `control-plane/internal/fleet/turns/template.go` — `Input(n int) string`, the only turn input (new)
- `control-plane/internal/fleet/turns/session.go` — `SessionFor`, the ST.5 session id rule (new)
- `control-plane/internal/fleet/turns/scheduler.go` — `Scheduler`: claim, send `turn.begin`, accept
  `turn.end`, write the `ot-…` record through the outbox, sweep lapsed claims (new)
- `control-plane/internal/fleet/turns/{queue,origin,template,session,scheduler}_test.go` (new)
- `control-plane/internal/fleet/wire/turn.go` — `TurnBegin`, `TurnEnd`, the Fleet copy of ST.5 (new;
  beside task 09's `control.go`, not an edit of it)
- `sandboxd/internal/control/turn.go` — the same two shapes, the sandbox copy (new; beside task
  09's `wire.go`)
- `testdata/panacea-control/v1/turn-begin-new.json`, `turn-begin-resume.json`, `turn-end.json`,
  `turn-end-reply.json`, `turn-end-mismatch.json` — joins task 09's fixture set; both modules'
  round-trip tests (task 09's) pick them up by glob (new)
- `control-plane/cmd/fleetd/config.go` — `PANACEA_APP_FLEET_TURN_TIMEOUT` (modified; task 12's file)

The file map row for task 26 lists `queue.go`, `origin.go` and `template.go`. `session.go` and
`scheduler.go` are added because ST.5's "session id rule" and the `turn.begin`/`turn.end` exchange
need a home, and neither belongs in the queue. The two `turn.go` wire files are new files so that
task 09's `control.go` and `wire.go` stay that task's alone.

**Interfaces:**
- Produces (ST.5, consumed by tasks 27 and 40):
  - `turns.Input(n int) string` and the exact sentences it returns
  - `wire.TurnBegin` / `control.TurnBegin`, pushed as `{"event":"turn.begin", …}`
  - `wire.TurnEnd` / `control.TurnEnd`, sent up as the request verb `turn.end`
  - `turns.SessionFor(prev SessionState, sandboxID string, rotateDue bool) (mode, uuid string)`
- Produces (consumed by tasks 28, 30, 31, 37, 38):
  - `turns.Origin`, `OriginHuman`, `OriginFleet`; `turns.Scheduler.CurrentTurn(orch) (TurnRef, bool)`,
    which a verb handler calls to learn the origin of the turn a call belongs to (task 31's taint
    rule) and to record the ids the turn issued
  - `turns.Queue.Enqueue(ctx, model.Event) error`, the single entry point every producer uses:
    task 25 (conflict opened), 28 (peer message), 24 (worker report), 23 (lease contested or
    expiring), 30 (escalation answered), 52 (human message)
- Consumes:
  - task 09: `model.AgentID`, `model.Event` and its kinds (table below), `model.Class`
  - task 10: the store methods in `turns.Store` below, including `ClaimTurn`
  - task 11: `outbox.Writer` for the `ot-…` record; task 07's Hub `turns.go` route is its target
  - task 19: `control.Streams.Push(ctx, agent model.AgentID, event any) error`
  - task 20: `sessions.Current(ctx, orch) (sandboxID string, ok bool)`
  - phase 2: `ids.Token`, `ids.UUID` (`control-plane/internal/ids/ids.go:30,46`); `ids.Wide` for
    `ot-…` (`internal/ids/ids.go:61` on `feat/hub`, built; arrives with the Hub sync-merge)
  - task 02: `teamtext.Clean` (G3), passed in as the `Cleaner`
- Requires of **task 10** (cross-task, not satisfiable here): the `turns.Store` methods, in both
  `memstore` and `pgstore`, and in `storetest.Suite`. If task 10's names differ, a three-line
  adapter in `scheduler.go` maps them; the semantics below are the contract.
- Requires of **task 12**: `PANACEA_APP_FLEET_TURN_TIMEOUT`, Go duration, default `10m`, minimum
  `1m`, a bad value fails start with its name.

**The event kinds** (§4.2). The names are task 09's `model.EventKind` constants; this table is what
this task requires of them. If task 09's part spells one differently, task 09 wins and this table
is renamed, not the other way round.

| Kind | Raised by | Immediate? | Origin it can produce |
|---|---|---|---|
| `human.message` | task 52, the owner's `hm-…` from the panel | no, coalesces | `human` (only if its author is the Orchestrator's owner) |
| `peer.message` | task 28, a new `ms-…` in one of its threads | **yes** | `fleet` |
| `conflict.opened` | task 25, a new `cf-…` naming one of its workers | **yes** | `fleet` |
| `worker.done` · `worker.blocked` · `worker.assumed` | task 24, `panacea report` | no | `fleet` |
| `lease.contested` · `lease.expiring` | task 23 (and task 35's expiry) | no | `fleet` |
| `escalation.answered` | task 30, its human answers an `es-…` | no | `fleet` |

`escalation.answered` is `fleet`, not `human`, on purpose: the answer is a typed choice among the
options Fleet wrote, applied by Fleet (task 30), and §7.2 grants a `human`-origin turn its wider
authority only for something the human **said to their Orchestrator**. `human.message` is not
immediate; the scope and §4.2 list only the two thread kinds. A panel message can therefore wait
up to 60 seconds for its turn. That latency is recorded as a gap at the end, not changed here.

**The `turns` package, complete:**

```go
// Package turns is Fleet's per-Orchestrator event queue and the ST.5 turn
// exchange. It never reads event text: an Event is an id, a kind, a ref
// to a Hub record or Fleet row, and an author. The turn input is rendered
// from a count alone.
package turns

import (
	"context"
	"errors"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/outbox"
)

// Origin is who started a turn (05-design.md §7.2).
type Origin string

const (
	OriginHuman Origin = "human"
	OriginFleet Origin = "fleet"
)

// CoalesceWindow is the longest a non-immediate event waits for company.
const CoalesceWindow = 60 * time.Second

// MaxBatch caps one turn's events; the rest wait for the next turn.
const MaxBatch = 50

// MaxReplyBytes bounds a human-origin turn's kept reply (task 27 cuts it).
const MaxReplyBytes = 8000

var (
	ErrNotOwnHuman   = errors.New("turns: human.message author is not the orchestrator's owner")
	ErrTurnMismatch  = errors.New("turns: turn.end does not match the claimed turn")
	ErrNotOrchestrator = errors.New("turns: queue belongs to orchestrators only")
)

// Immediate reports whether a kind skips coalescing (§4.2, 06 I8).
func Immediate(k model.EventKind) bool {
	return k == model.EventPeerMessage || k == model.EventConflictOpened
}

// Queued is an event as the queue holds it.
type Queued struct {
	Event   model.Event // ID, Orchestrator, Kind, Ref, Author, At
	TurnID  string      // "" until batched
	Retried bool        // set when a lost turn re-queued it once
}

// TurnRef is what a verb handler learns about the turn a call belongs to.
type TurnRef struct {
	ID     string // ot-…
	Origin Origin
	Orch   model.AgentID
	Began  time.Time
}

// SessionState is Fleet's memory of an Orchestrator's harness session.
type SessionState struct {
	UUID      string // "" before the first turn
	SandboxID string // the sandbox that ran the last turn
	Turns     int    // turns run on this UUID
}

// Store is what this package needs of task 10's fleet store. Every method
// is one transaction. ClaimTurn is the two-replica guard: of two concurrent
// callers for one orchestrator, exactly one gets ok == true.
type Store interface {
	AppendEvent(ctx context.Context, q Queued) error
	PendingEvents(ctx context.Context, orch model.AgentID) ([]Queued, error)
	// OrchestratorsDue prefilters: an unbatched immediate event, or an
	// unbatched event older than now-CoalesceWindow, and no live claim.
	OrchestratorsDue(ctx context.Context, now time.Time) ([]model.AgentID, error)
	// ClaimTurn inserts row as the orchestrator's one live claim and marks
	// row.EventIDs batched, iff no claim row exists (a lapsed one included:
	// only LoseTurn removes it, because only LoseTurn un-batches its events).
	ClaimTurn(ctx context.Context, row ClaimRow) (ok bool, err error)
	ClaimedTurn(ctx context.Context, orch model.AgentID) (ClaimRow, bool, error)
	// EndTurn deletes the claim and its batched events, writes session and
	// the Hub-bound item in the same transaction (task 11's outbox rule).
	// It fails with ErrTurnMismatch unless turnID is the live claim.
	EndTurn(ctx context.Context, orch model.AgentID, turnID string,
		session SessionState, hub outbox.Item) error
	LapsedClaims(ctx context.Context, now time.Time) ([]ClaimRow, error)
	// LoseTurn deletes the claim, un-batches its events that were not yet
	// Retried (marking them Retried), deletes the rest and returns their
	// ids, clears the session, and writes hub, in one transaction.
	LoseTurn(ctx context.Context, orch model.AgentID, turnID string,
		hub outbox.Item) (dropped []string, err error)
	Session(ctx context.Context, orch model.AgentID) (SessionState, error)
	RecordIssued(ctx context.Context, orch model.AgentID, turnID, issuedID string) error
}

// ClaimRow is an in-flight turn.
type ClaimRow struct {
	Orch      model.AgentID
	TurnID    string // ot-…
	Nonce     string
	Origin    Origin
	EventIDs  []string
	Issued    []string // ms-/dv-/es-/tk- ids issued during the turn
	Mode      string   // ModeNew | ModeResume
	SessionID string   // the harness uuid this turn runs under
	SandboxID string   // the sandbox it was pushed to
	Began     time.Time
	Until     time.Time
}

// TokenMeter reads the gateway's metered usage for one turn's key (task 20
// mints a fresh key per turn). ok == false means "not known", never zero.
type TokenMeter interface {
	TurnTokens(ctx context.Context, turnID string) (input, output int64, ok bool, err error)
}
```

**`template.go`, complete.** The signature is the property: an `int` in, one of two sentences out.

```go
package turns

import "strconv"

// Input is the only text a turn's child receives (05-design.md §4.2). It
// is rendered from the batch size and nothing else, so no byte of any
// event, message, path or name can reach the model through it.
func Input(n int) string {
	if n <= 1 {
		return "1 team event. Run `panacea inbox` to read it."
	}
	return strconv.Itoa(n) + " team events. Run `panacea inbox` to read them."
}
```

**`origin.go`, complete.**

```go
package turns

import "github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"

// OriginOf is human iff the batch holds a human.message authored by the
// orchestrator's own owner (§7.2). Enqueue already refuses any other
// author; the second check here means a store row written around Enqueue
// still cannot raise a turn's authority.
func OriginOf(batch []Queued, owner model.Owner) Origin { // model.Owner as task 22 cites it
	for _, q := range batch {
		if q.Event.Kind == model.EventHumanMessage && q.Event.Author == owner {
			return OriginHuman
		}
	}
	return OriginFleet
}
```

**`session.go`, complete: the ST.5 session id rule.**

```go
package turns

import "github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"

const (
	ModeNew    = "new"    // runner passes --session-id <uuid>
	ModeResume = "resume" // runner passes --resume <uuid>
)

// SessionFor decides the next turn's harness session. Resume only when the
// transcript can still exist: same uuid, same sandbox (the transcript is in
// that pod's emptyDir, §4.2), and no rotation due (task 40). Never a fork.
func SessionFor(prev SessionState, sandboxID string, rotateDue bool) (mode, uuid string) {
	if prev.UUID != "" && prev.SandboxID == sandboxID && sandboxID != "" && !rotateDue {
		return ModeResume, prev.UUID
	}
	return ModeNew, ids.UUID()
}
```

**The wire, ST.5.** Task 09 owns the envelope (`{"v":1,"id","verb","args"}`, the response, and
server-pushed `{"event": …}`); these are the two payloads. Both copies are written out in full;
the Fleet copy is shown, and the sandbox copy in `sandboxd/internal/control/turn.go` is the same
declarations in `package control` with the same JSON tags.

```go
package wire

import "time"

// EventTurnBegin is pushed down the control stream to an orchestrator pod.
const EventTurnBegin = "turn.begin"

// VerbTurnEnd is the runner's request up the stream. It is in the
// orchestrator class's verb table (task 09) and in no CLI (task 18).
const VerbTurnEnd = "turn.end"

type TurnSession struct {
	Mode string `json:"mode"` // "new" | "resume"
	ID   string `json:"id"`   // canonical v4 uuid
}

type TurnBegin struct {
	Event     string      `json:"event"`     // "turn.begin"
	Turn      string      `json:"turn"`      // ot-…
	Nonce     string      `json:"nonce"`     // 43 url-safe chars; never shown to the child
	Session   TurnSession `json:"session"`
	Input     string      `json:"input"`     // turns.Input(n), nothing else
	KeepReply bool        `json:"keepReply"` // true iff origin is human
	Deadline  time.Time   `json:"deadline"`  // the claim's until, RFC 3339 UTC
}

// Exit values. An int exit code is carried separately.
const (
	ExitOK      = "ok"      // child exited 0
	ExitFailed  = "failed"  // child exited non-zero
	ExitTimeout = "timeout" // runner killed the child at the deadline
	ExitLost    = "lost"    // written by Fleet only, never sent by a runner
)

type TurnEnd struct {
	Turn           string `json:"turn"`
	Nonce          string `json:"nonce"`
	Exit           string `json:"exit"`  // ok | failed | timeout
	Code           int    `json:"code"`  // the child's exit status; -1 if killed
	DurationMs     int64  `json:"durationMs"`
	Reply          string `json:"reply,omitempty"` // only when keepReply; ≤ MaxReplyBytes
	ReplyTruncated bool   `json:"replyTruncated,omitempty"`
}
```

The fixtures, byte for byte (task 09's decoders refuse unknown fields, G2):

```json
// testdata/panacea-control/v1/turn-begin-new.json
{"event":"turn.begin","turn":"ot-1a2b3c4d","nonce":"Zm9vYmFyYmF6cXV4cXV1eGNvcmdlZ3JhdWx0Z2FycGx5","session":{"mode":"new","id":"5f0c7a52-3b1e-4c2a-9d7e-0a1b2c3d4e5f"},"input":"3 team events. Run `panacea inbox` to read them.","keepReply":false,"deadline":"2026-09-25T10:10:00Z"}
```

```json
// testdata/panacea-control/v1/turn-begin-resume.json
{"event":"turn.begin","turn":"ot-5e6f7a8b","nonce":"cXV1eGNvcmdlZ3JhdWx0Z2FycGx5Wm05dlltRnlZbUY2","session":{"mode":"resume","id":"5f0c7a52-3b1e-4c2a-9d7e-0a1b2c3d4e5f"},"input":"1 team event. Run `panacea inbox` to read it.","keepReply":true,"deadline":"2026-09-25T10:25:00Z"}
```

```json
// testdata/panacea-control/v1/turn-end.json
{"v":1,"id":"rq-00000001","verb":"turn.end","args":{"turn":"ot-1a2b3c4d","nonce":"Zm9vYmFyYmF6cXV4cXV1eGNvcmdlZ3JhdWx0Z2FycGx5","exit":"ok","code":0,"durationMs":41250}}
```

```json
// testdata/panacea-control/v1/turn-end-reply.json
{"v":1,"id":"rq-00000002","verb":"turn.end","args":{"turn":"ot-5e6f7a8b","nonce":"cXV1eGNvcmdlZ3JhdWx0Z2FycGx5Wm05dlltRnlZbUY2","exit":"ok","code":0,"durationMs":18800,"reply":"I asked Bob's orchestrator to sequence auth/session.go after his lane lands.","replyTruncated":false}}
```

```json
// testdata/panacea-control/v1/turn-end-mismatch.json  (the response Fleet sends)
{"id":"rq-00000003","ok":false,"error":{"code":"turn_mismatch","message":"no claimed turn matches this turn and nonce"}}
```

The fixtures carry no `//` line in the files themselves; the comment above each block names the
file. `rq-` is task 09's request id prefix (**UNVERIFIED** until task 09's part is read; if it uses
another, the fixtures follow it). `turn_mismatch` joins task 09's ST.1 error codes.

**`queue.go`, complete.**

```go
package turns

import (
	"context"
	"sort"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// Agents is the one lookup the queue needs (task 10's store has it).
type Agents interface {
	Agent(ctx context.Context, id model.AgentID) (model.Agent, error)
}

// Queue is the single entry point for every event producer.
type Queue struct {
	Store  Store
	Agents Agents
	Wake   func(model.AgentID) // nudges the scheduler; may be nil
}

// Enqueue refuses an event for a non-orchestrator, and a human.message whose
// author is not that orchestrator's owner (§3.3 rule 5). The event carries
// ids and a kind only; Ref points at the Hub record or Fleet row.
func (q *Queue) Enqueue(ctx context.Context, e model.Event) error {
	a, err := q.Agents.Agent(ctx, e.Orchestrator)
	if err != nil {
		return err
	}
	if a.Class != model.ClassOrchestrator {
		return ErrNotOrchestrator
	}
	if e.Kind == model.EventHumanMessage && e.Author != a.Owner {
		return ErrNotOwnHuman
	}
	if err := q.Store.AppendEvent(ctx, Queued{Event: e}); err != nil {
		return err
	}
	if q.Wake != nil && Immediate(e.Kind) {
		q.Wake(e.Orchestrator)
	}
	return nil
}

// Ready picks the batch to run now from an orchestrator's unbatched events,
// or returns nil. Pure: the scheduler and the tests call the same function.
// A batch is due when any event is immediate, or when the oldest has waited
// CoalesceWindow. Immediate events go first; the rest oldest first; at most
// MaxBatch, the remainder waiting for the next turn.
func Ready(pending []Queued, now time.Time) []Queued {
	var open []Queued
	for _, q := range pending {
		if q.TurnID == "" {
			open = append(open, q)
		}
	}
	if len(open) == 0 {
		return nil
	}
	sort.SliceStable(open, func(i, j int) bool {
		ii, ij := Immediate(open[i].Event.Kind), Immediate(open[j].Event.Kind)
		if ii != ij {
			return ii
		}
		return open[i].Event.At.Before(open[j].Event.At)
	})
	due := Immediate(open[0].Event.Kind)
	for _, q := range open {
		if !now.Before(q.Event.At.Add(CoalesceWindow)) {
			due = true
		}
	}
	if !due {
		return nil
	}
	if len(open) > MaxBatch {
		open = open[:MaxBatch]
	}
	return open
}
```

**`scheduler.go`, the exported surface and the two paths that matter.** Bodies not shown are
mechanical (a store read, a loop); the begin and end paths are written out because they carry the
properties.

```go
package turns

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/outbox"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

type Pusher interface { // task 19's control.Streams
	Push(ctx context.Context, agent model.AgentID, event any) error
}
type Sandboxes interface { // task 20's sessions
	Current(ctx context.Context, orch model.AgentID) (sandboxID string, ok bool)
}
// Cleaner is task 02's G3 hygiene, teamtext.Clean: it refuses, it never
// strips, and its error names the rule. Bounding is End's (MaxReplyBytes).
type Cleaner func(s string) error

// ReplyWithheld replaces a reply that G3 refuses. The turn record keeps the
// fact and the rule, never the refused bytes.
const ReplyWithheld = "Reply withheld: it contained characters the product does not store."

type Scheduler struct {
	Store       Store
	Agents      Agents
	Streams     Pusher
	Sandboxes   Sandboxes
	Meter       TokenMeter
	Clean       Cleaner
	TurnTimeout time.Duration // PANACEA_APP_FLEET_TURN_TIMEOUT
	Now         func() time.Time
	Log         *slog.Logger
}

// TryBegin starts a turn for orch if one is due and none is in flight.
func (s *Scheduler) TryBegin(ctx context.Context, orch model.AgentID) (bool, error) {
	pending, err := s.Store.PendingEvents(ctx, orch)
	if err != nil {
		return false, err
	}
	now := s.Now()
	batch := Ready(pending, now)
	if batch == nil {
		return false, nil
	}
	sandboxID, ok := s.Sandboxes.Current(ctx, orch)
	if !ok {
		return false, nil // no live pod; task 20 relaunches, events wait
	}
	a, err := s.Agents.Agent(ctx, orch)
	if err != nil {
		return false, err
	}
	prev, err := s.Store.Session(ctx, orch)
	if err != nil {
		return false, err
	}
	mode, uuid := SessionFor(prev, sandboxID, false) // task 40 supplies rotateDue
	row := ClaimRow{
		Orch: orch, TurnID: ids.Wide("ot"), Nonce: ids.Token(),
		Origin: OriginOf(batch, a.Owner), EventIDs: eventIDs(batch),
		Mode: mode, SessionID: uuid, SandboxID: sandboxID,
		Began: now, Until: now.Add(s.TurnTimeout),
	}
	if ok, err := s.Store.ClaimTurn(ctx, row); err != nil || !ok {
		return false, err // another replica, or a turn in flight
	}
	begin := wire.TurnBegin{
		Event: wire.EventTurnBegin, Turn: row.TurnID, Nonce: row.Nonce,
		Session: wire.TurnSession{Mode: mode, ID: uuid},
		Input:   Input(len(batch)), KeepReply: row.Origin == OriginHuman,
		Deadline: row.Until.UTC(),
	}
	if err := s.Streams.Push(ctx, orch, begin); err != nil {
		return true, s.lose(ctx, a, row, "push: "+err.Error())
	}
	return true, nil
}

// TurnRecord is the ot-… body of task 07's route 69, POST …/turns
// (task-07.md:175-180): id, orchestratorAgentId, origin, consumed ≤ 200,
// issued ≤ 100, tokens. Exit, Reply and ReplyTruncated are this task's
// additions to that body, and Tokens optional; see "Requires of task 07".
type TurnRecord struct {
	ID             string      `json:"id"`
	Orchestrator   string      `json:"orchestratorAgentId"`
	Origin         Origin      `json:"origin"`
	Consumed       []string    `json:"consumed"`
	Issued         []string    `json:"issued"`
	Tokens         *TurnTokens `json:"tokens,omitempty"` // absent = not known, never zero
	Exit           string      `json:"exit"`             // ok | failed | timeout | lost
	Reply          string      `json:"reply,omitempty"`  // human-origin only, text·A, ≤ MaxReplyBytes
	ReplyTruncated bool        `json:"replyTruncated,omitempty"`
}

type TurnTokens struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
}

// End accepts a runner's turn.end (task 19 dispatches the verb here, after
// its class check: only an orchestrator-class stamp reaches it).
func (s *Scheduler) End(ctx context.Context, orch model.AgentID, e wire.TurnEnd) error {
	row, ok, err := s.Store.ClaimedTurn(ctx, orch)
	if err != nil {
		return err
	}
	if !ok || row.TurnID != e.Turn ||
		subtle.ConstantTimeCompare([]byte(row.Nonce), []byte(e.Nonce)) != 1 {
		return ErrTurnMismatch
	}
	if e.Exit != wire.ExitOK && e.Exit != wire.ExitFailed && e.Exit != wire.ExitTimeout {
		return ErrTurnMismatch // "lost" is Fleet's word, never a runner's
	}
	a, err := s.Agents.Agent(ctx, orch)
	if err != nil {
		return err
	}
	rec := s.record(ctx, row, e.Exit)
	if row.Origin == OriginHuman && e.Reply != "" {
		if len(e.Reply) > MaxReplyBytes {
			return ErrTurnMismatch // the runner cuts at MaxReplyBytes; more is not a runner
		}
		rec.Reply, rec.ReplyTruncated = e.Reply, e.ReplyTruncated
		if err := s.Clean(e.Reply); err != nil {
			rec.Reply, rec.ReplyTruncated = ReplyWithheld, false
			s.Log.Warn("turn reply withheld", "turn", row.TurnID, "rule", err.Error())
		}
	} // a reply on a fleet-origin turn is dropped: nobody asked for it
	next := SessionState{} // any unclean end: next turn is a new session
	if e.Exit == wire.ExitOK {
		prev, _ := s.Store.Session(ctx, orch)
		next = SessionState{UUID: row.SessionID, SandboxID: row.SandboxID, Turns: prev.Turns + 1}
		if row.Mode == ModeNew {
			next.Turns = 1
		}
	}
	if err := s.Store.EndTurn(ctx, orch, row.TurnID, next, s.item(a, rec)); err != nil {
		return err
	}
	_, err = s.TryBegin(ctx, orch) // the next batch may already be due
	return err
}

// lose ends a turn that no runner will end: push failed, the claim lapsed,
// or task 19 reports the stream gone for good. Events re-queue once.
func (s *Scheduler) lose(ctx context.Context, a model.Agent, row ClaimRow, why string) error {
	rec := s.record(ctx, row, wire.ExitLost) // consumed = the batch; a second loss names the same ids
	dropped, err := s.Store.LoseTurn(ctx, row.Orch, row.TurnID, s.item(a, rec))
	if err == nil && len(dropped) > 0 {
		s.Log.Warn("turn lost twice; nudges dropped", "turn", row.TurnID, "events", dropped, "why", why)
	}
	return err
}

// SweepLapsed runs on task 12's deadline-sweeper singleton, every 5 s.
func (s *Scheduler) SweepLapsed(ctx context.Context) error
// StreamLost is task 19's callback when an orchestrator's stream closes
// and the broker reports its pod gone.
func (s *Scheduler) StreamLost(ctx context.Context, orch model.AgentID) error
// Tick runs every second on every replica; ClaimTurn keeps it single.
func (s *Scheduler) Tick(ctx context.Context) error
// CurrentTurn and RecordIssued are called by verb handlers (tasks 28–31).
func (s *Scheduler) CurrentTurn(ctx context.Context, orch model.AgentID) (TurnRef, bool, error)
func (s *Scheduler) RecordIssued(ctx context.Context, orch model.AgentID, issuedID string) error
```

`record` builds a `TurnRecord` from the row (`Consumed` = `row.EventIDs`, capped at 200 as route
69 requires; `MaxBatch` is 50, so the cap never cuts), reading `Meter.TurnTokens(row.TurnID)` and
leaving `Tokens` nil when `ok` is false. `item` builds the `outbox.Item` with `ID: ids.Wide("ob")`, the
agent's org, owner, project and delegation, `Op: outbox.OpTurn`, `Subject: row.TurnID` and the
marshalled record (task 11's part, `outbox.Item` and `OpTurn`). A lost turn's record says
`exit: "lost"` and lists its batch in `consumed`; when the same events are lost a second time,
the second `lost` record names the same ids, and that pair is the audit trail of a dropped nudge.
The store never touches a payload (task 11: a payload is never re-marshalled). `LoseTurn` is the
only writer of `Retried`.

**Requires of task 07** (a Hub route this task writes to; raised, not changed here): route 69's
body gains `exit` (`ok`·`failed`·`timeout`·`lost`), an optional `reply` (`text·A`, ≤ 8,000
bytes, stored with the audit event and readable by the owner and an org auditor, §3.3 and §4.2),
`replyTruncated`, and `tokens` becomes optional. Until it does, strict decoding refuses the body.
If task 07 prefers the reply as its own record rather than audit-event metadata, `TurnRecord`
splits in two and this part's `item` writes two outbox rows in the same transaction.

- [ ] **Step 1: Write the tests, each with the mutant it kills**

All tests run on task 10's `memstore`, a fake `Pusher` that records frames, a fake `Sandboxes`, a
fake `TokenMeter` and a fixed clock. `package turns`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestPeerEventIsNotDelayedByCoalescing` | one `peer.message` at t0; `Ready` at t0 returns it; `TryBegin` pushes `turn.begin` at t0 | `Immediate` returns false for `peer.message`; the 60 s rule applied to every kind |
| `TestConflictOpenedIsImmediate` | same for `conflict.opened` | `Immediate` lists only `peer.message` |
| `TestWorkerReportsCoalesceForSixtySeconds` | three `worker.done` at t0, t0+10s, t0+20s: `Ready` is nil at t0+59s, returns all three at t0+60s; one push, `Input(3)` | `Ready` fires per event; the window measured from the newest event |
| `TestImmediateEventCarriesTheWaitingOnes` | a `worker.done` at t0 and a `peer.message` at t0+5s: one batch of two at t0+5s, peer first | the batch holds immediate kinds only, leaving the report for a second turn |
| `TestHumanMessageMakesOriginHuman` | a batch of `worker.done` + `human.message` authored by the owner: `row.Origin == human`, `KeepReply == true` | `OriginOf` checks the first event only; origin fixed to `fleet` |
| `TestForeignHumanMessageIsRefused` | `Enqueue` of a `human.message` whose `Author` is another principal returns `ErrNotOwnHuman`; nothing appended | the author check removed from `Enqueue` |
| `TestOriginOfIgnoresAForeignRowWrittenAroundEnqueue` | a store row with a foreign author passed straight to `OriginOf` yields `fleet` | `OriginOf` trusts `Kind` alone |
| `TestEscalationAnsweredIsFleetOrigin` | a batch of one `escalation.answered`: origin `fleet` | `escalation.answered` treated as human |
| `TestTemplateCarriesNoEventByte` | for every event in `internal/fleet/turns/testdata/events.json` (12 events, one per kind plus three whose refs and authors hold distinctive strings such as `IGNORE-PREVIOUS` and a bidi override; task 41's `testdata/team-events/` is a different, later corpus), the pushed `Input` is exactly `Input(len(batch))`, and contains none of the fixture's `id`, `ref`, `kind` or `author` strings | a template that names the first event's kind, or its count by kind |
| `TestInputExactSentences` | `Input(1)`, `Input(3)`, `Input(50)` equal the three literal strings | wording drift; singular/plural swapped |
| `TestOneTurnAtATime` | two due batches for one orchestrator: second `TryBegin` returns false while the first is claimed | `ClaimTurn` result ignored |
| `TestTwoReplicasOneTurn` | two `Scheduler`s on one `memstore`, 50 concurrent `TryBegin`: exactly one push | the claim taken after the push |
| `TestTurnEndNeedsTheNonce` | `End` with the right turn and a wrong nonce: `ErrTurnMismatch`, claim unchanged | nonce not compared; compared with `==` on the turn id only |
| `TestTurnEndRefusesLost` | `End` with `exit: "lost"`: `ErrTurnMismatch` | the exit whitelist removed |
| `TestReplyKeptOnlyForHumanOrigin` | a fleet-origin `End` with a reply: the outbox item's record has no `reply`; a human-origin one has it, cleaned | the reply stored whatever the origin |
| `TestReplyRefusedByG3IsWithheld` | a reply with an ANSI escape: the stored reply is exactly `ReplyWithheld`, the turn still ends, and the log line names the rule, not the bytes | `Clean` bypassed; the whole `End` failed on a refused reply, losing the turn record |
| `TestOversizeReplyRefused` | a reply of `MaxReplyBytes+1` bytes: `ErrTurnMismatch`, claim unchanged | the bound trusted to the runner |
| `TestSessionNewThenResume` | first turn `mode: new` with a uuid; after `End(ok)`, the next is `resume` with the same uuid | the uuid re-minted per turn; resume on the first turn |
| `TestSessionNewAfterPodReplaced` | after `End(ok)` on sandbox `sb-1`, `Current` returns `sb-2`: next is `new`, fresh uuid | `SessionFor` ignores the sandbox id |
| `TestSessionNewAfterUncleanEnd` | `End(failed)` then next turn: `new` | the session kept on failure |
| `TestLostTurnRequeuesOnce` | push fails: events un-batched and `Retried`; the next `TryBegin` runs them; a second loss deletes them, and two `ot-` records with `exit: "lost"` list the same ids in `consumed` | re-queue forever; drop on first loss |
| `TestLapsedClaimIsLost` | clock past `Until`: `SweepLapsed` ends the turn `lost`, writes one `ot-` item, and a new turn can begin | the sweep only logs |
| `TestTurnRecordWrittenInTheEndTransaction` | `EndTurn` on `memstore` with an injected outbox failure leaves the claim in place and no event deleted | the item appended after commit |
| `TestTokensAbsentWhenMeterUnknown` | meter `ok=false`: the record's JSON has no `tokens` key; meter 1200/34: `"tokens":{"input":1200,"output":34}` | `Tokens` written as zeros |
| `TestTurnRecordMatchesRoute69` | the marshalled record's keys are exactly route 69's six plus `exit`, `reply`, `replyTruncated` | a renamed key (`eventIds` for `consumed`) the Hub's strict decode refuses |
| `TestIssuedIdsReachTheRecord` | two `RecordIssued` calls during a turn: the record's `issued` holds both | `Issued` not carried from the claim |

Wire tests, in both modules, extend task 09's round-trip suite by glob: the five new fixtures
decode, re-encode byte-identically, and an extra field (`"origin":"human"` inside `turn.begin`) is
refused. The mutant is a `TurnBegin` that carries origin to the pod: the pod has no use for it,
and a field that reaches the pod reaches, in the worst case, the child.

- [ ] **Step 2: Run the tests and watch them fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/turns/... ./internal/fleet/wire/...
```

Expected: compile failure, `undefined: Ready`, `undefined: Input`, `undefined: SessionFor`,
`undefined: Scheduler`, `undefined: wire.TurnBegin`. A test that passes here tests nothing.

- [ ] **Step 3: Write `template.go`, `origin.go`, `session.go` and the two `turn.go` wire files**

As fenced above. Run the template, origin and session tests; they pass. The sandbox copy's test
runs in its own module:

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/sandboxd && GOPROXY=off GOFLAGS=-mod=vendor go test ./internal/control/...
```

- [ ] **Step 4: Write `queue.go`, then `scheduler.go`**

`Tick` lists `OrchestratorsDue(now)` and calls `TryBegin` for each; a `TryBegin` error is logged
with the orchestrator id and never stops the loop. `SweepLapsed` lists `LapsedClaims(now)` and
calls `lose` for each; a claim that another replica already ended returns `ErrTurnMismatch` from
`LoseTurn`, which is skipped silently (two sweepers are one too many, not an error, though task
12's advisory lock means there is one). `StreamLost` calls `lose` for the live claim, if any, and
does nothing else: the relaunch is task 20's.

- [ ] **Step 5: Add `turns.Store` to `memstore`, `pgstore` and `storetest.Suite`**

This is task 10's code; the methods are added there, in task 10's files, as that task's
extension. The schema adds to `0001_fleet.sql` only if task 10 has not merged; otherwise a new
`0002_turns.sql`:

```sql
-- fleet.turn_events: the per-orchestrator queue. No text column exists.
CREATE TABLE IF NOT EXISTS fleet.turn_events (
  org_id        text        NOT NULL,
  event_id      text        NOT NULL,
  orchestrator  text        NOT NULL,              -- ag-…
  kind          text        NOT NULL,
  ref           text        NOT NULL,              -- a Hub id or Fleet row id
  author_tid    text        NOT NULL DEFAULT '',
  author_oid    text        NOT NULL DEFAULT '',
  at            timestamptz NOT NULL,
  turn_id       text,                              -- NULL until batched
  retried       boolean     NOT NULL DEFAULT false,
  PRIMARY KEY (org_id, event_id)
);
CREATE INDEX IF NOT EXISTS turn_events_open
  ON fleet.turn_events (orchestrator, at) WHERE turn_id IS NULL;

-- fleet.turn_claims: at most one live claim per orchestrator.
CREATE TABLE IF NOT EXISTS fleet.turn_claims (
  orchestrator  text        PRIMARY KEY,
  org_id        text        NOT NULL,
  turn_id       text        NOT NULL UNIQUE,       -- ot-…
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

-- fleet.orch_sessions: the ST.5 session state, a fact about now.
CREATE TABLE IF NOT EXISTS fleet.orch_sessions (
  orchestrator  text        PRIMARY KEY,
  org_id        text        NOT NULL,
  session_id    uuid,
  sandbox_id    text        NOT NULL DEFAULT '',
  turns         integer     NOT NULL DEFAULT 0
);
```

`ClaimTurn` in `pgstore` is `INSERT … ON CONFLICT (orchestrator) DO NOTHING` plus the `UPDATE
turn_events SET turn_id` in the same transaction; zero rows inserted is `ok == false`. A lapsed
claim is **not** overwritten here, even though that would save up to one sweep interval: its
events are batched under the old turn id, and only `LoseTurn` un-batches them. So the interface
comment's "iff no unexpired claim exists" is implemented as "iff no claim row exists", and the
sweep (every 5 s) is what turns a lapsed claim into no claim. Row-level security, if task 10
applies it to `fleet.*`, covers these tables by the same `org_id` policy; **UNVERIFIED** until
task 10's part is read.

The absence of a text column in `turn_events` is itself a property: a `storetest.Suite` row
asserts the table's columns, from `information_schema.columns`, are exactly the ten above. A
later edit that adds `body text` fails the suite.

- [ ] **Step 6: Wire into `fleetd`**

In task 12's `main.go`: build one `turns.Queue` and one `turns.Scheduler`; hand the `Queue` to
every producer (tasks 23–25, 28, 30, 52 take it as a constructor argument, never a global);
register `turn.end` in task 19's dispatch table for the `orchestrator` class only, calling
`Scheduler.End`; run `Tick` every second on every replica and `SweepLapsed` on the deadline-sweeper
singleton; register `Scheduler.StreamLost` with task 19's `Streams`. Parse
`PANACEA_APP_FLEET_TURN_TIMEOUT` in `config.go`.

- [ ] **Step 7: Run everything**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/... ./cmd/fleetd/... && GOPROXY=off go vet ./internal/fleet/... ./cmd/fleetd/... && gofmt -l internal/fleet cmd/fleetd
```

Expected: `ok` for `internal/fleet/turns`, `internal/fleet/wire`, `internal/fleet/store/...` and
`cmd/fleetd`; `go vet` silent; `gofmt -l` prints nothing. With `PANACEA_APP_FLEET_TEST_PG` unset
the `pgstore` half of the suite reports `SKIP` (task 10's convention, **UNVERIFIED** until its
part is read), and that skip is recorded, not read as a pass.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && grep -rn "Event.Ref\|Event.Author\|\.Kind" control-plane/internal/fleet/turns/template.go
```

Expected: no output. `template.go` reads no event field.

- [ ] **Step 8: Commit**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && git add control-plane/internal/fleet/turns control-plane/internal/fleet/wire/turn.go sandboxd/internal/control/turn.go testdata/panacea-control/v1/turn-*.json && git commit -F .superpowers/sdd/team/msg/task-26.txt -- control-plane/internal/fleet/turns control-plane/internal/fleet/wire/turn.go sandboxd/internal/control/turn.go testdata/panacea-control/v1 control-plane/internal/fleet/store control-plane/cmd/fleetd
```

**What this task does not claim:**

- **That a turn runs.** Nothing here starts a harness. The runner is task 27; a real turn on a
  real pod is task 33's (TM1-α on the cluster).
- **That tokens are known at `turn.end`.** They come from phase 6's usage store through
  `TokenMeter`, whose implementation is task 20's `tokens.go` over P6 task 15's
  `gatewayclient` (**UNVERIFIED until the phase 6 merge**). The meter's lag is unmeasured; a
  record written before the gateway has the numbers says `tokens` absent, and task 37 reconciles.
- **That 60 seconds is the right window**, or 10 minutes the right turn timeout. Both are
  proposals from §4.2 and this part; task 33 measures turn length on the pinned CLI.
- **That the queue survives a Fleet restart with no duplicate turn.** It survives (Postgres), and
  a claim survives with it, so a restarted replica cannot double-start a turn. A runner whose
  `turn.end` arrives while both replicas are down retries it (task 27); if the claim lapsed in
  the meantime, the turn is recorded `lost` and its events run once more. That duplicate is
  accepted: every Orchestrator verb is idempotent over typed state or refused by the limits
  (tasks 28, 30), and the `ot-` records show both.
- **Anything about rotation or DIVE.** `SessionFor`'s `rotateDue` is always false here; task 40
  sets it and parks the runner for DIVE.

**Gaps against the skeleton, raised here and not resolved alone:**

1. The scope says `turn.end` "comes back with the turn's token use"; OD-8 makes that impossible
   for the runner, so tokens come from the gateway (decision b). The skeleton's sentence should
   read "the turn's token use is read from the gateway for the turn's key".
2. `human.message` coalesces like any non-thread event, so a message typed in the panel may wait
   up to 60 s for its turn. §4.2 and the scope say so; the product may want it immediate. That is
   an owner call, not a planner's.
3. Task 09's scope lists `turn.end` among the **server-pushed** events. In ST.5 it is a request
   the runner sends **up** (verb `turn.end`), and Fleet's answer is the ordinary ST.1 response.
   Fleet never pushes a `turn.end`; a turn Fleet ends itself is recorded `lost`, and the runner
   learns of it only through `stop` (task 27). Task 09's event list should drop `turn.end`, and
   task 16's `events.wait` queue then never sees one.
4. **The human-origin reply has no Hub home.** §4.2 and §3.3 keep it "with the turn's `ot-…`
   record as `text·A`"; task 07's route 69 is metadata-only with a strict body. See "Requires of
   task 07" above.

