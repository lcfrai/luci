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

**(c) `turn.end` carries a nonce the child never sees.** `turn.end` travels up the same in-pod
socket every `panacea` call uses (task 16), so the Orchestrator's own Claude session could, in
principle, send one and end its turn early, dropping the turn key and leaving work unrecorded.
`turn.begin` therefore carries a 43-character `nonce` (`ids.Token()`, `ids.go:30`); the runner keeps
it in memory, never in the child's argv or environment (task 27 tests that), and Fleet accepts a
`turn.end` only for the claimed turn **with** its nonce, once. A wrong nonce is `turn_mismatch`
and changes nothing.

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
  - phase 2: `ids.New`, `ids.Token`, `ids.UUID` (`control-plane/internal/ids/ids.go:20,30,46`)
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
	OrchestratorsWithPending(ctx context.Context) ([]model.AgentID, error)
	ClaimTurn(ctx context.Context, orch model.AgentID, turnID, nonce string,
		origin Origin, eventIDs []string, until time.Time) (ok bool, err error)
	ClaimedTurn(ctx context.Context, orch model.AgentID) (ClaimRow, bool, error)
	EndTurn(ctx context.Context, orch model.AgentID, turnID string) (ClaimRow, error)
	LapsedClaims(ctx context.Context, now time.Time) ([]ClaimRow, error)
	RequeueOnce(ctx context.Context, turnID string) (dropped []string, err error)
	Session(ctx context.Context, orch model.AgentID) (SessionState, error)
	PutSession(ctx context.Context, orch model.AgentID, s SessionState) error
	RecordIssued(ctx context.Context, turnID, issuedID string) error
}

// ClaimRow is an in-flight or just-ended turn.
type ClaimRow struct {
	Orch     model.AgentID
	TurnID   string
	Nonce    string
	Origin   Origin
	EventIDs []string
	Issued   []string // ms-/dv-/es-/tk- ids issued during the turn
	Began    time.Time
	Until    time.Time
}

// TokenMeter reads the gateway's metered usage for one turn's key (task 20
// mints a fresh key per turn). ok == false means "not known", never zero.
type TokenMeter interface {
	TurnTokens(ctx context.Context, turnID string) (tokens int64, ok bool, err error)
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
func OriginOf(batch []Queued, owner model.Principal) Origin {
	for _, q := range batch {
		if q.Event.Kind == model.EventHumanMessage && q.Event.Author == owner {
			return OriginHuman
		}
	}
	return OriginFleet
}
```
