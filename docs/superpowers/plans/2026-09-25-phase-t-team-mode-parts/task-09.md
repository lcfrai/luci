### Task 09: Fleet model and the `panacea` control wire v1

**Slice TM1-α. Depends on: 02. About 2.5 hours. Seam: ST.1 (defines it).**

Two small packages and one fixture directory, with no transport and no behaviour beyond decoding.
`cp/internal/fleet/model` is Fleet's vocabulary: agent classes, ids, owners, agents, leases,
queued events, turns, and **the per-class verb table of §5.2 as data**. The wire is the `panacea`
control protocol every sandbox speaks to Fleet: one request envelope, one response envelope with
one error shape, and one server-pushed event envelope, written **twice**, once in each module,
because `control-plane` and `sandboxd` do not import each other (checked: no `panacea-app/sandboxd`
import under `control-plane/`; the skeleton's file map says the same). Both copies are pinned by
one fixture set in `testdata/panacea-control/v1/`, which both modules' tests read.

**Eight parts were written before this one** and each cited the names it needs from ST.1, saying
"task 09 wins". This part adopts those names wherever they agree, and settles the three places they
do not (see "Reconciled with the parts already written"). A renaming this part forces on a written
part is listed in "Gaps", so its writer or implementer changes one line.

**The one thing this task must not get wrong is that the two copies cannot drift.** A field added
to one module's `Response` and not the other's is a sandbox that silently drops Fleet's error
detail, or a Fleet that refuses every sandbox frame. So: both decoders refuse unknown fields (G2);
both round-trip **every** fixture byte for byte, found by glob, so a fixture added by a later task
(22, 23, 24, 26, 28 already add some) is tested by both modules without either test changing; and
one test in each module fails if a fixture exists that its module's types cannot decode.

**Files:**
- `control-plane/internal/fleet/model/model.go`: ids, owners, classes, agents, statuses (new)
- `control-plane/internal/fleet/model/lease.go`: modes, states, `Lease` (new)
- `control-plane/internal/fleet/model/event.go`: `EventKind`, `Event`, `Turn`, `TurnOrigin` (new)
- `control-plane/internal/fleet/model/verbs.go`: the verb table and `ClassMay` (new)
- `control-plane/internal/fleet/model/model_test.go` (new)
- `control-plane/internal/fleet/wire/control.go`: the Fleet copy of the wire (new)
- `control-plane/internal/fleet/wire/control_test.go`: round trip of every fixture (new)
- `sandboxd/internal/control/wire.go`: the sandbox copy (new)
- `sandboxd/internal/control/wire_test.go`: the same round trip (new)
- `testdata/panacea-control/v1/*.json`: the envelope fixtures and one request per verb (new; the
  directory is this task's, and later tasks add files to it)

**Interfaces:**
- Consumes: task 02's `teamtext` (only its `MaxGlobs` constant, for the verb table's comment; no
  code dependency, so `model` imports the standard library only); phase 2's `ids` is **not**
  imported: the wire's request id is opaque.
- Produces, in `control-plane/internal/fleet/model`: `AgentID`, `Owner`, `Class` and its three
  values, `Agent`, `Status`, `WaitingOn`, `ExitClass`, `LeaseMode`, `LeaseState`, `Lease`,
  `EventKind` and its nine values, `Event`, `TurnOrigin`, `Turn`, `Verb`, `Verbs`, `ClassMay`,
  `LocalVerb`.
- Produces, in `control-plane/internal/fleet/wire` and `sandboxd/internal/control`: `Version`,
  `Request`, `Response`, `Error`, `Event`, the event names, the codes, `Refuse`, `Refusal`,
  `DecodeRequest`, `DecodeResponse`, `DecodeEvent`, `OK`, `Fail`, `MaxRequestID`.
- Required by: 16 (the sandbox copy; `Stamped` and the snapshot frame are task 16's additions),
  18 (the CLI builds `Request` and prints `Result`), 19 (dispatch: `ClassMay`, `Refuse`, the
  codes), 22 to 31 (handlers use `model` and `wire.Refuse`), 26 and 27 (the turn frames are
  task 26's additions on this envelope), 38 (the `nudge` event).

#### `model` — complete source

```go
// control-plane/internal/fleet/model/model.go

// Package model is Fleet's vocabulary (05-design.md §3.3, §4, §5.2). It holds
// types and tables only: no I/O, no clock, no imports beyond the standard library.
// A Hub record is never a model type: Fleet keeps Hub records in memory as
// hubapi values (ADR-H1:467-474); model is what Fleet itself owns.
package model

import "time"

// AgentID is an ag-… id. Fleet mints it (task 20, ids.Wide) and the Hub checks it
// belongs to the delegation's owner (task-01.md:130-135). Never a session id.
type AgentID string

// Owner is the Hub user id of the human an agent works for (u-…, as every Hub team
// record names it, task-01.md:295). Fleet derives it from tid+oid once, at the
// desktop's request (task 12), and stores only this.
type Owner string

// Class is a sandbox's class (§2.1; phase 8 task 14b). The mirror is Fleet-internal
// (task 58): it holds no delegation and may call no panacea verb.
type Class string

const (
	ClassOrchestrator Class = "orchestrator"
	ClassWorker       Class = "worker"
	ClassMirror       Class = "mirror"
)

func (c Class) Valid() bool {
	return c == ClassOrchestrator || c == ClassWorker || c == ClassMirror
}

// Status extends phase 2's four session statuses (registry/session.go:14-31) with
// the two a team agent adds (§3.3 row "agent").
type Status string

const (
	StatusStarting Status = "starting"
	StatusWorking  Status = "working"
	StatusWaiting  Status = "waiting"
	StatusIdle     Status = "idle"
	StatusEnded    Status = "ended"
)

// WaitingOn is why a waiting agent waits. Lease and peer are Fleet's own facts;
// human only from `panacea report blocked --on human` or the cloud hook's
// Notification event, never guessed (§3.3; phase 2 refuses to guess, session.go:10-13).
type WaitingOn string

const (
	WaitingNone  WaitingOn = ""
	WaitingLease WaitingOn = "lease"
	WaitingPeer  WaitingOn = "peer"
	WaitingHuman WaitingOn = "human"
)

// ExitClass is how an agent ended; the values are task 01's §7.15 (task-01.md:336-337).
type ExitClass string

const (
	ExitDone          ExitClass = "done"
	ExitStopped       ExitClass = "stopped"
	ExitFailed        ExitClass = "failed"
	ExitAccessRemoved ExitClass = "access_removed"
	ExitLapsed        ExitClass = "lapsed"
)

// Agent is Fleet's row for one agent. Org and Project scope every query; Parent is
// "" for an Orchestrator; Delegation is the dl-… it was started under (task 20),
// which task 11's rebuild reads back (task-11.md:85).
type Agent struct {
	ID         AgentID
	Org        string
	Project    string
	Owner      Owner
	Class      Class
	Parent     AgentID
	Task       string // tk-… or ""
	Delegation string // dl-…
	Status     Status
	WaitingOn  WaitingOn
	StartedAt  time.Time
	EndedAt    time.Time // zero while live
	Exit       ExitClass // "" while live
}
```

```go
// control-plane/internal/fleet/model/lease.go
package model

import "time"

// LeaseMode is §6.3's three modes. Intent is a plan-time claim with no expiry;
// hold is an edit claim that lapses 30 minutes after the last edit to that path;
// bulk is observed, never requested (task 35).
type LeaseMode string

const (
	LeaseIntent LeaseMode = "intent"
	LeaseHold   LeaseMode = "hold"
	LeaseBulk   LeaseMode = "bulk"
)

// LeaseState is §3.3's seven. The last four are terminal.
type LeaseState string

const (
	LeaseRequested     LeaseState = "requested"
	LeaseGranted       LeaseState = "granted"
	LeaseContested     LeaseState = "contested"
	LeaseYielded       LeaseState = "yielded"
	LeaseReleased      LeaseState = "released"
	LeaseExpired       LeaseState = "expired"
	LeaseAdminReleased LeaseState = "admin-released"
)

func (s LeaseState) Terminal() bool {
	return s == LeaseYielded || s == LeaseReleased || s == LeaseExpired || s == LeaseAdminReleased
}

// Lease is Fleet's authoritative row (§3.3: Fleet's Postgres is authoritative, the
// Hub records each change). Seq is the last seq written to the Hub (task 23, task
// 06's gapless rule); the store increments it in the state change's transaction.
type Lease struct {
	ID           string // ls-…
	Org          string
	Project      string
	Holder       AgentID
	Owner        Owner
	Task         string // tk-… for an intent, else ""
	Globs        []string
	Mode         LeaseMode
	State        LeaseState
	Seq          int
	TrackedFiles int       // the count the breadth check used; 0 when unknown (TM1-α)
	ExpiresAt    time.Time // zero for an intent
	UpdatedAt    time.Time
}
```

```go
// control-plane/internal/fleet/model/event.go
package model

import "time"

// EventKind is one thing an Orchestrator's queue can hold (§4.2). Two are
// immediate (task 26's Immediate): peer.message and conflict.opened.
type EventKind string

const (
	EventHumanMessage       EventKind = "human.message"
	EventPeerMessage        EventKind = "peer.message"
	EventConflictOpened     EventKind = "conflict.opened"
	EventWorkerDone         EventKind = "worker.done"
	EventWorkerBlocked      EventKind = "worker.blocked"
	EventWorkerAssumed      EventKind = "worker.assumed"
	EventLeaseContested     EventKind = "lease.contested"
	EventLeaseExpiring      EventKind = "lease.expiring"
	EventEscalationAnswered EventKind = "escalation.answered"
)

// EventKinds is every kind, for validation and for the store's CHECK (task 10).
var EventKinds = []EventKind{EventHumanMessage, EventPeerMessage, EventConflictOpened,
	EventWorkerDone, EventWorkerBlocked, EventWorkerAssumed, EventLeaseContested,
	EventLeaseExpiring, EventEscalationAnswered}

func (k EventKind) Valid() bool {
	for _, v := range EventKinds {
		if k == v {
			return true
		}
	}
	return false
}

// Event is one queued item. It carries no text: Ref names the Hub record or Fleet
// row (hm-…, ms-…, cf-…, ag-…, ls-…, es-…) the Orchestrator reads through
// `panacea inbox` (§4.2: the turn input never carries peer text). Author is the
// human behind a human.message, "" for every other kind; task 26 grants human
// origin only when Author is the Orchestrator's own owner.
type Event struct {
	ID           string // ev-… (Fleet-local; never reaches the Hub)
	Orchestrator AgentID
	Kind         EventKind
	Ref          string
	Author       Owner
	At           time.Time
}

// TurnOrigin is §7.2's: human when the batch holds a message from the
// Orchestrator's own human, else fleet. Task 26's turns.Origin is an alias of it.
type TurnOrigin string

const (
	OriginHuman TurnOrigin = "human"
	OriginFleet TurnOrigin = "fleet"
)

// Turn is one batch delivered to one Orchestrator (§4.2), the Fleet side of the
// Hub's ot-… audit event (task 07's route 69).
type Turn struct {
	ID           string // ot-…
	Orchestrator AgentID
	Origin       TurnOrigin
	Events       []string // Event.ID, in queue order
	StartedAt    time.Time
	EndedAt      time.Time // zero while running
}
```
