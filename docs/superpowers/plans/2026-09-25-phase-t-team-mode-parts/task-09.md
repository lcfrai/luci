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
- `control-plane/internal/fleet/wire/{control.go,codes.go,decode.go}`: the Fleet copy of the wire
  (new; the skeleton's file map names `control.go`, and the other two are its split, so each file
  stays under 120 lines)
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

```go
// control-plane/internal/fleet/model/verbs.go
package model

// Verb is one row of §5.2's table. Local verbs are answered inside the pod and
// never reach Fleet: token by phase 6's apiKeyHelper (P6-14), events.wait by
// sandboxd (task 16, decision 10). Fleet's dispatcher (task 19) refuses a local
// verb as unknown_verb if one ever arrives.
type Verb struct {
	Name    string
	Classes []Class
	Local   bool
}

// Verbs is §5.2 as data. The mirror class has no row: it calls nothing.
var Verbs = []Verb{
	{Name: "team", Classes: []Class{ClassOrchestrator, ClassWorker}},
	{Name: "check", Classes: []Class{ClassOrchestrator, ClassWorker}},
	{Name: "lease", Classes: []Class{ClassOrchestrator, ClassWorker}},
	{Name: "report", Classes: []Class{ClassWorker}},
	{Name: "files", Classes: []Class{ClassOrchestrator}},
	{Name: "show", Classes: []Class{ClassOrchestrator}},
	{Name: "task", Classes: []Class{ClassOrchestrator}},
	{Name: "worker", Classes: []Class{ClassOrchestrator}},
	{Name: "direct", Classes: []Class{ClassOrchestrator}},
	{Name: "inbox", Classes: []Class{ClassOrchestrator}},
	{Name: "peer", Classes: []Class{ClassOrchestrator}},
	{Name: "human", Classes: []Class{ClassOrchestrator}},
	{Name: "lane", Classes: []Class{ClassOrchestrator}},
	{Name: "turn.end", Classes: []Class{ClassOrchestrator}}, // the runner only (task 16's pid gate)
	{Name: "token", Classes: []Class{ClassOrchestrator, ClassWorker}, Local: true},
	{Name: "events.wait", Classes: []Class{ClassOrchestrator}, Local: true},
}

// ClassMay reports whether class c may send verb to Fleet. A local verb is never
// sent, so ClassMay is false for it whatever the class.
func ClassMay(c Class, verb string) bool {
	for _, v := range Verbs {
		if v.Name != verb {
			continue
		}
		if v.Local {
			return false
		}
		for _, k := range v.Classes {
			if k == c {
				return true
			}
		}
		return false
	}
	return false
}

// LocalVerb reports whether verb is answered in the pod.
func LocalVerb(verb string) bool {
	for _, v := range Verbs {
		if v.Name == verb {
			return v.Local
		}
	}
	return false
}
```

**Two rows are decisions, not transcriptions.** `lease` is open to the Orchestrator as well as
workers: §5.2 says "all", and task 24 takes intent leases at plan time from the Orchestrator's
`task` call. `report` is workers only: an Orchestrator has no Orchestrator to report to. The table
is a slice, not a map, so its order is §5.2's and `panacea` usage output (task 18) can print it
as written.

#### The wire — complete source, Fleet copy

```go
// control-plane/internal/fleet/wire/control.go

// Package wire is the panacea control protocol, v1 (ST.1). sandboxd/internal/control/wire.go
// is the same declarations in package control; testdata/panacea-control/v1 pins both.
// Frames are single JSON objects; the transport (task 16's socket, task 19's stream)
// is not this package's.
package wire

import (
	"encoding/json"
	"time"
)

const Version = 1

// MaxRequestID bounds Request.ID; the id is opaque, echoed back, ^[A-Za-z0-9_-]{1,64}$.
// The CLI uses rq-<n> (task 18); sandboxd re-stamps it c-<n> (task 16).
const MaxRequestID = 64

// Request is what a sandbox sends. Args is always a JSON object, {} when empty;
// its shape is the verb handler's (tasks 22 to 31), decoded strictly there.
type Request struct {
	V    int             `json:"v"`
	ID   string          `json:"id"`
	Verb string          `json:"verb"`
	Args json.RawMessage `json:"args"`
}

// Response answers one Request. OK true carries Result (JSON null allowed:
// events.wait's empty answer, task 16); OK false carries Error.
type Response struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Error is the one error shape. Since is set with CodeUnavailable only; RetryAfter
// with CodeRateLimited only (task 28); Details carries typed context (task 24's
// {"from","to"} on CodeConflict), strings only, never free text from a peer.
type Error struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Since      *time.Time        `json:"since,omitempty"`
	RetryAfter *time.Time        `json:"retryAfter,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

// Pushed event names this task owns. turn.begin is task 26's (wire/turn.go);
// snapshot is task 16's. Every pushed frame is one object whose "event" key names it.
const (
	EventNudge = "nudge"
	EventStop  = "stop"
)

// Nudge tells a pod that its team view changed; the pod's delivery rung decides
// what, if anything, to do (task 38). It carries nothing else in v1.
type Nudge struct {
	Event string `json:"event"` // "nudge"
}

// Stop tells an Orchestrator runner (task 27) to end after its current turn.
type Stop struct {
	Event  string `json:"event"`  // "stop"
	Reason string `json:"reason"` // StopRotate | StopAccessRemoved | StopLapsed | StopByOwner
}

const (
	StopRotate        = "rotate"         // task 40: a new session, seeded from typed state
	StopAccessRemoved = "access_removed" // task 11's Revoker
	StopLapsed        = "lapsed"         // the delegation lapsed
	StopByOwner       = "by_owner"       // the owner stopped it from the panel
)
```

```go
// control-plane/internal/fleet/wire/codes.go — the one code list (ST.1)
package wire

const (
	// Dispatch (task 19): the frame, the session, the class table.
	CodeBadRequest     = "bad_request"
	CodeTooLarge       = "too_large"
	CodeStampMismatch  = "stamp_mismatch"
	CodeUnknownSession = "unknown_session"
	CodeUnknownVerb    = "unknown_verb"
	CodeForbiddenClass = "forbidden_class"
	CodeInternal       = "internal"
	// Availability: every verb, when Fleet or the Hub path is down (§5.2).
	CodeUnavailable = "unavailable"
	// Ownership and objects (tasks 19, 22 to 24). One code for "not yours" and
	// "no such object of yours", so existence never leaks.
	CodeAgentNotOwner = "agent_not_owner"
	CodeNotFound      = "not_found"
	CodeConflict      = "conflict"
	CodeTaskHasWorkers = "task_has_workers"
	// Text and globs (task 02, as the Hub's task-01.md:197-199).
	CodeTextRefused     = "text_refused"
	CodeGlobRefused     = "glob_refused"
	CodeBreadthExceeded = "breadth_exceeded"
	CodeBreadthUnknown  = "breadth_unknown"
	// Turns (task 26).
	CodeTurnMismatch = "turn_mismatch"
	// Protocol limits (task 28).
	CodeRoundsExhausted    = "rounds_exhausted"
	CodeRateLimited        = "rate_limited"
	CodeThreadClosed       = "thread_closed"
	CodeKindNotEnabled     = "kind_not_enabled"
	CodePermissionRequired = "permission_required"
	CodeNotParticipant     = "not_participant"
)

// Codes is every code above; a code outside it is a bug the round-trip test finds.
var Codes = []string{CodeBadRequest, CodeTooLarge, CodeStampMismatch, CodeUnknownSession,
	CodeUnknownVerb, CodeForbiddenClass, CodeInternal, CodeUnavailable, CodeAgentNotOwner,
	CodeNotFound, CodeConflict, CodeTaskHasWorkers, CodeTextRefused, CodeGlobRefused,
	CodeBreadthExceeded, CodeBreadthUnknown, CodeTurnMismatch, CodeRoundsExhausted,
	CodeRateLimited, CodeThreadClosed, CodeKindNotEnabled, CodePermissionRequired,
	CodeNotParticipant}

// Refusal is a handler's typed refusal. Dispatch (task 19) turns it into Error;
// any other error becomes CodeInternal with a fixed message, so a Go error string
// (which may hold a path or a peer's text) never reaches a sandbox.
type Refusal struct {
	Code    string
	Message string // one product-written sentence
	Details map[string]string
}

func (r *Refusal) Error() string { return r.Code + ": " + r.Message }

// Refuse is what handlers return: `return nil, wire.Refuse(wire.CodeNotFound, "No such lease.")`.
func Refuse(code, message string) error { return &Refusal{Code: code, Message: message} }

// RefuseWith is Refuse with details.
func RefuseWith(code, message string, details map[string]string) error {
	return &Refusal{Code: code, Message: message, Details: details}
}
```

The sandbox copy carries the same list, exactly. Task 16's `CodeForbidden` (`task-16.md:173-176`),
which only `sandboxd` sends, lives in task 16's `server.go`, not in `wire.go`, so the cross-module
comparison below sees two equal lists.

```go
// control-plane/internal/fleet/wire/decode.go
package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

var (
	ErrVersion = errors.New("wire: v is not 1")
	ErrID      = errors.New("wire: id is not 1-64 of [A-Za-z0-9_-]")
	ErrVerb    = errors.New("wire: verb is empty")
	ErrArgs    = errors.New("wire: args is not a JSON object")
	ErrShape   = errors.New("wire: ok and result/error disagree")
)

// strict decodes exactly one JSON value into dst, refusing unknown fields (G2)
// and trailing data.
func strict(b []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if d.More() {
		return errors.New("wire: trailing data")
	}
	return nil
}

func DecodeRequest(b []byte) (Request, error) {
	var r Request
	if err := strict(b, &r); err != nil {
		return Request{}, err
	}
	switch {
	case r.V != Version:
		return Request{}, ErrVersion
	case !idRe.MatchString(r.ID):
		return Request{}, ErrID
	case r.Verb == "":
		return Request{}, ErrVerb
	case len(bytes.TrimSpace(r.Args)) == 0 || bytes.TrimSpace(r.Args)[0] != '{':
		return Request{}, ErrArgs
	}
	return r, nil
}

func DecodeResponse(b []byte) (Response, error) {
	var r Response
	if err := strict(b, &r); err != nil {
		return Response{}, err
	}
	if r.OK == (r.Error != nil) {
		return Response{}, ErrShape
	}
	return r, nil
}

// EventName reads only the "event" key of a pushed frame, so the caller can pick
// the strict type to decode it into (Nudge, Stop, task 26's TurnBegin, task 16's
// snapshot). It is the one loose decode in the package, and it decodes nothing else.
func EventName(b []byte) (string, error) {
	var e struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(b, &e); err != nil {
		return "", err
	}
	if e.Event == "" {
		return "", errors.New("wire: no event name")
	}
	return e.Event, nil
}

// DecodeEvent decodes a pushed frame strictly into dst after EventName chose it.
func DecodeEvent(b []byte, dst any) error { return strict(b, dst) }
```

`OK(id string, result any) (Response, error)` marshals `result` into `Result`, and
`Fail(id string, err error) Response` turns a `*Refusal` into `Error` and anything else into
`{"code":"internal","message":"Fleet could not answer that."}`; both are six lines and live in
`decode.go`. `Unavailable(id string, since time.Time) Response` is the one constructor that sets
`Since`, so no code path can send `unavailable` without it.

**The sandbox copy** is `sandboxd/internal/control/wire.go`: the four types, the event names,
`Nudge`, `Stop`, the `Stop*` reasons, the codes and `Codes`, and the three decoders, with the
same JSON tags, in `package control`. It omits `Refusal`, `Refuse`, `RefuseWith`, `OK`, `Fail`
and `Unavailable` except as task 16 needs them (`sandboxd` answers `unavailable` itself when the
stream is down, `task-16.md:281`, so it carries `Unavailable`). The copy is written out by
copying the Fleet file and changing the package line; the round-trip test is what proves the two
still agree afterwards.

#### Every verb's `args` and `result`

The `args` column is task 18's argv table (`task-18.md:131-152`), adopted as written; its rows
marked "(09?)" are hereby fixed. The Go type of each `args` and `result` lives with the verb's
handler, in `cp/internal/fleet/verbs` or `…/protocol`, because only Fleet decodes them: the CLI
prints `result` verbatim between task 18's header and footer and never decodes it. The **result**
column names the owning task and its fixture; the shape there is authoritative.

| Verb | `args` | `result` (owner, fixture) |
|---|---|---|
| `team` | `{}` | worker or orchestrator view (task 22, `result-team-worker.json`, `result-team-orchestrator.json`) |
| `check` | `{"path"}` | directives, holders, leases, conflicts on one path (task 22, `result-check-*.json`) |
| `files` | `{}` | the tree at the integration head (task 22) |
| `show` | `{"path"}` | one file at the integration head (task 22) |
| `lease` | `{"op":"request","globs":[…],"mode"?:"intent","task"?}` · `{"op":"release","lease"}` | the lease and any overlaps (task 23, `result-lease-granted.json`, `result-lease-contested.json`) |
| `report` | `{"kind":"done"\|"blocked"\|"assumed","text","on"?:"human"}` | `{"reported":"ev-…"}` (task 24) |
| `task` | `{"op":"create"\|"update"\|"abandon", …}` | the task and its overlaps (task 24, `result-task-create-overlap.json`) |
| `worker` | `{"op":"spawn","task"}` · `{"op":"stop","agent"}` | the worker's `ag-…` and status (task 24) |
| `direct` | `{"to","kind", "globs"? \| "lease"? \| "text"?}` | the `dv-…` issued, or the `es-…` a decide became (task 31) |
| `inbox` | `{}` | quoted items since the last read (task 28, `inbox-*.json`) |
| `peer` | `{"kind", "thread"?, "fields", "text"?}` | the `ms-…` appended, or a templated answer (task 28, `peer-*.json`) |
| `human` | `{"kind":"inform"\|"ask", "question", "options"?}` | the `es-…` (task 30) |
| `lane` | `{"agent"}` | changed paths and line counts, never contents (task 34's sample; TM1-α answers `unavailable` until task 34) |
| `turn.end` | task 26's `TurnEnd` | `{}` or `turn_mismatch` (task 26, `turn-end*.json`) |

**No `args` carries an owner, an org, a class or a caller id.** The caller is `sandboxd`'s stamp
(task 16's `Stamped.Agent`, `.Class`), and Fleet looks the owner up from the agent (task 19). A
handler's strict decode refuses `"owner"` like any unknown key, which task 24's
`TestCreateForOtherOwnerRefused` already relies on.

#### The fixtures this task ships

All in `testdata/panacea-control/v1/`, one JSON value per file, no trailing newline, compact
(`json.Compact` of the file equals the file). Later tasks add their own files beside these.

| File | Holds |
|---|---|
| `request-team.json` | `{"v":1,"id":"rq-00000001","verb":"team","args":{}}` |
| `request-check.json` | `{"v":1,"id":"rq-00000002","verb":"check","args":{"path":"src/export/route.ts"}}` |
| `request-lease-request.json` | `{"v":1,"id":"rq-00000003","verb":"lease","args":{"op":"request","globs":["src/export/**"]}}` |
| `request-report-blocked-human.json` | `{"v":1,"id":"rq-00000004","verb":"report","args":{"kind":"blocked","on":"human","text":"Which region codes are in scope?"}}` |
| `request-direct-pause.json` | `{"v":1,"id":"rq-00000005","verb":"direct","args":{"to":"ag-00000000000000b1","kind":"pause"}}` |
| `request-inbox.json` | `{"v":1,"id":"rq-00000006","verb":"inbox","args":{}}` |
| `response-ok-null.json` | `{"id":"rq-00000001","ok":true,"result":null}` |
| `response-unavailable.json` | `{"id":"rq-00000002","ok":false,"error":{"code":"unavailable","message":"team view unavailable since 2026-09-25T10:00:00Z","since":"2026-09-25T10:00:00Z"}}` |
| `response-conflict.json` | `{"id":"rq-00000003","ok":false,"error":{"code":"conflict","message":"That task cannot move from done.","details":{"from":"done","to":"active"}}}` |
| `response-rate-limited.json` | `{"id":"rq-00000004","ok":false,"error":{"code":"rate_limited","message":"Twenty messages an hour between two orchestrators.","retryAfter":"2026-09-25T11:02:00Z"}}` |
| `event-nudge.json` | `{"event":"nudge"}` |
| `event-stop.json` | `{"event":"stop","reason":"access_removed"}` |
| `bad-unknown-field.json` | `{"v":1,"id":"rq-1","verb":"team","args":{},"owner":"u-x"}`: must be **refused** by both |
| `bad-version.json` | `{"v":2,"id":"rq-1","verb":"team","args":{}}`: refused |
| `bad-args-array.json` | `{"v":1,"id":"rq-1","verb":"team","args":[]}`: refused |

Files beginning `bad-` are the negative set: both modules' tests assert each is refused. Task 18's
`request-<verb>-<case>.json` convention is this table's.

**How the round-trip test classifies a file it did not write.** By its top-level keys, not its
name: `v` is a request, `ok` a response, `event` a pushed event; a file with none of the three is
an `args-`, `result-` or snapshot body that its own task tests, and this test skips it. Two
exceptions, by prefix: `stamped-*` (task 16's sandbox-to-Fleet frame, a request plus two stamped
keys, decoded by task 16's and task 19's own types) and `bad-*` (must be refused). **An event
fixture must decode into a registered type in both modules**, so a task that adds a pushed event
(task 26's `turn.begin`, task 16's snapshot) registers it in each copy, or both modules' tests
fail. Registration is one line in the adding task's own file:

```go
// control-plane/internal/fleet/wire/control.go (the registry)

// eventTypes maps a pushed event's name to a fresh value of its strict type.
// Tasks that add an event register it from their own file with an init, e.g.
// task 26's turn.go: func init() { eventTypes[EventTurnBegin] = func() any { return new(TurnBegin) } }.
var eventTypes = map[string]func() any{
	EventNudge: func() any { return new(Nudge) },
	EventStop:  func() any { return new(Stop) },
}

// DecodePushed decodes any registered pushed frame strictly.
func DecodePushed(b []byte) (any, error) {
	name, err := EventName(b)
	if err != nil {
		return nil, err
	}
	mk, ok := eventTypes[name]
	if !ok {
		return nil, errors.New("wire: unregistered event " + name)
	}
	v := mk()
	return v, DecodeEvent(b, v)
}
```

- [ ] **Step 1: Write the failing tests**

Both modules get the same four tests over the same directory; the Fleet file is shown, and the
sandbox file differs in its package, its import path, and `fixtures = "../../../testdata/…"`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEveryFixtureRoundTrips` | for each non-`bad-`, non-`stamped-` file classified as request, response or event: decode strictly, re-encode with `json.Marshal`, and the bytes equal the file | a field renamed in one copy; an `omitempty` dropped, so `"details":null` appears |
| `TestBadFixturesAreRefused` | each `bad-*.json` fails `DecodeRequest` | `DisallowUnknownFields` omitted (G2), so `"owner"` rides in |
| `TestUnavailableCarriesSince` | `response-unavailable.json`'s `Error.Since` is 10:00Z; `Unavailable("x", t)` marshals with `since` and no `retryAfter` | a constructor that forgets `since`, so the CLI prints "since" with no time |
| `TestResponseShape` | `{"id":"a","ok":true,"error":{…}}` and `{"id":"a","ok":false}` both fail `DecodeResponse` with `ErrShape` | an ok that disagrees with its body accepted |
| `TestRequestIDGrammar` | ids of 0 and 65 characters, and `"a b"`, fail with `ErrID`; 64 passes | the length check off by one; spaces allowed, which breaks task 16's log lines |
| `TestFailHidesGoErrors` | `Fail("a", errors.New("open /home/x/secret: denied"))` is `internal` with the fixed message | the Go error string sent to a sandbox |
| `TestCodesMatchAcrossModules` (Fleet side only) | parses `sandboxd/internal/control/wire.go` with `go/parser`, collects every `Code*` string constant, and compares the set with `wire.Codes`: equal | a code added to one copy only |
| `TestVerbTable` (`model_test.go`) | `ClassMay(ClassWorker, "peer")` false; `ClassMay(ClassOrchestrator, "peer")` true; `ClassMay(ClassMirror, v)` false for every verb; `ClassMay(any, "token")` false; every `Verbs` name unique | a local verb dispatched; the mirror given a verb; a duplicate row that shadows the first |
| `TestTerminalLeaseStates` | exactly `yielded`, `released`, `expired`, `admin-released` are `Terminal()` | a terminal state missed, so task 06's Hub refuses Fleet's next event |
| `TestEventKindsAreSections42` | `EventKinds` has nine distinct values, each `Valid()`; `"turn.begin"` is not an `EventKind` | a pushed wire event confused with a queued model event |

`TestCodesMatchAcrossModules` reads the sibling module's source as text, which is allowed: it is
a file read, not an import, so the two modules still do not depend on each other.

The central test in full:

```go
// control-plane/internal/fleet/wire/control_test.go (excerpt)
package wire

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtures = "../../../../testdata/panacea-control/v1"

func classify(t *testing.T, b []byte) string {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatalf("not a JSON object: %v", err)
	}
	for _, k := range []string{"v", "ok", "event"} {
		if _, ok := top[k]; ok {
			return k
		}
	}
	return ""
}

// Mutant killed: a JSON tag renamed or an omitempty dropped in this copy only. The
// sandbox copy runs the same loop, so a drift fails one module or the other.
func TestEveryFixtureRoundTrips(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(fixtures, "*.json"))
	if len(files) < 12 {
		t.Fatalf("found %d fixtures; the directory moved?", len(files))
	}
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasPrefix(base, "bad-") || strings.HasPrefix(base, "stamped-") {
			continue
		}
		b, _ := os.ReadFile(f)
		var v any
		var err error
		switch classify(t, b) {
		case "v":
			var r Request
			r, err = DecodeRequest(b)
			v = r
		case "ok":
			var r Response
			r, err = DecodeResponse(b)
			v = r
		case "event":
			v, err = DecodePushed(b)
		default:
			continue // an args-, result- or snapshot body: its own task tests it
		}
		if err != nil {
			t.Errorf("%s: %v", base, err)
			continue
		}
		got, _ := json.Marshal(v)
		if !bytes.Equal(got, b) {
			t.Errorf("%s: round trip\n got %s\nwant %s", base, got, b)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd control-plane && go test ./internal/fleet/model/ ./internal/fleet/wire/ -count=1
cd sandboxd && go test ./internal/control/ -count=1
```

Expected: both fail to build, naming the first missing symbol per test file (`undefined:
DecodeRequest`, `undefined: ClassMay`, and in `sandboxd` `undefined: DecodeRequest`). If task 16
has already landed in `sandboxd/internal/control`, the package builds and only these tests fail.

- [ ] **Step 3: Write `model`, then the Fleet wire, then the fixtures, then the sandbox copy**

In that order. Write each fixture with an editor and then run `python -c "import json,sys;b=open(sys.argv[1],'rb').read();assert b==json.dumps(json.loads(b),separators=(',',':'),ensure_ascii=False).encode()" <file>`
**only to check** it is compact (there is no `jq` on this machine); do not generate fixtures from the Go types, or the round trip proves
nothing. The sandbox copy is the Fleet file with `package control`; delete `Refusal`, `Refuse`,
`RefuseWith`, `OK`, `Fail`; keep `Unavailable`; add nothing. Task 16's `CodeForbidden` is task
16's own file.
