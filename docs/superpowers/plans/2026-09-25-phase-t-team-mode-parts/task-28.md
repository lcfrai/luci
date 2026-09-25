### Task 28: Protocol messages: `peer`, `inbox`, the templated `ask`, limits

Orchestrators never address each other's sandboxes (§6.1). They write typed messages into
threads that Fleet holds and the Hub records, and they read what the other side wrote only
through `panacea inbox`, quoted. This task is that channel: the eight message kinds of §6.2, the
schema check (G2) and hygiene (G3) before anything is stored, the fixed envelope (G4) on the way
out, and the limits of §6.4 and §6.6 that keep a thread from ping-ponging or draining a
teammate's quota. Resolutions are task 29's; escalations task 30's. This task stores a `propose`
and an `accept`; it does not act on them.

Four properties, each pinned by a test.

1. **Fleet answers every `ask` itself, at once, from typed card fields** (§6.2, §6.5 row 1). The
   asked Orchestrator takes no turn and its context window is never consulted: *"a context
   window is a transcript under another name"* (`04-enterprise-constraints.md:123-126`, `07`
   I5). The answer is returned **inline** in the asker's `peer` result, so a question costs the
   asker one tool call, not a hop, and the thread records a `templated` answer (task 07's route
   62, `templated: true`, no text).
2. **A free-text `answer` needs the sender's human's standing permission**, default off, and each
   one lands in that human's SINCE YOU LAST LOOKED as an `inform` (task 30 writes it). Without the
   permission it is refused with `permission_required`, and nothing is stored.
3. **Peer text reaches an Orchestrator only inside the `inbox` envelope** (G4): each message names
   its author as an agent, with its owner's display name, its author class (`text·A`), and the
   fixed notice sentence. Typed fields are returned as typed JSON, never folded into prose.
4. **The limits are Fleet's, per thread and per pair**, and a message that breaks one is refused
   with its code before storage: 3 rounds per side, a 30-minute deadline that team policy may
   lengthen and never shorten, 20 messages an hour between any two Orchestrators, and a thread
   that closes `escalated` after two rounds with no new typed content.

Five decisions the scope and §6.2 left open, made here.

**(a) What a round is.** A round is one `propose`, `counter` or `reject` by a side. Those are the
kinds that move a negotiation; `ask`, `answer` and `inform` exchange facts, `accept` ends a
round-trip, and `withdraw` takes something back. A side's fourth counting message is refused
with `rounds_exhausted`, and the thread escalates (below).

**(b) What "new typed content" is.** A `propose` or `counter` carries new typed content iff its
resolution, in canonical form (kind, then parameters sorted, globs sorted and de-duplicated), is
not equal to any resolution already in the thread. A `reject` never carries new typed content.
Two counting messages in a row, from either side, with no new typed content close the thread
`escalated`. So *propose X, reject, propose X again* escalates at the third message, and
*propose X, reject, propose Y, reject* does not.

**(c) `counter`, `split` and `build-on` are schema-checked here and refused with
`kind_not_enabled` until task 62** (TM2) enables them. The eight kinds are all in the wire from
day one so the CLI (task 18) and the Hub (task 07) never change shape; TM1-α's negotiation is
`propose` / `accept` / `reject` over `yield`, `sequence` and `proceed-both` (task 29).

**(d) Who may open a thread.** Fleet opens every conflict thread (task 25). An Orchestrator may
open one with `ask`, `inform` or `propose` by naming `to`, a live Orchestrator of **another**
current member of the same project; a same-owner `to` is `bad_request` (one owner's agents are
its own Orchestrator's to sequence). The subject maps onto task 07's route 60: `conflict` →
`conflict`, `lease` → `lease`, `task` and `path` → `free`. One open thread per pair per subject:
a second opener on the same subject gets the existing thread id and its message is appended
there.

**(e) Negotiation pauses with the Hub.** When task 11's `outbox.Gate` reports the org paused
(10 minutes of Hub failure, `task-11.md` `PauseAfter`), `peer` answers `unavailable` with
`since`, and the deadline sweep does not escalate a paused thread; each paused thread's deadline
moves out by the pause's length when the Gate reopens, so a Hub outage never turns into a
deadline escalation. `inbox` keeps working from Fleet's own state. The extension is Fleet state
only: task 07's route 61 cannot patch a thread's `deadline`, so the Hub keeps the original, and
the thread's escalation record (task 30) states the effective one.

**Files:**
- `control-plane/internal/fleet/protocol/messages.go` — `Kind`, `Resolution`, `Subject`, `Send`
  (the `peer` args), `Validate`: G2 schema and G3 text, per kind (new)
- `control-plane/internal/fleet/protocol/threads.go` — `Thread` (Fleet's working state), `Open`,
  `Append`, one transaction each with their outbox rows (new)
- `control-plane/internal/fleet/protocol/limits.go` — `Limits`, rounds, stale rounds, pair rate,
  the effective deadline, and the deadline sweep (new)
- `control-plane/internal/fleet/protocol/ask.go` — `CardAnswer`, the templated answer (new)
- `control-plane/internal/fleet/protocol/inbox.go` — `Envelope`, `Unread`, the read log (new)
- `control-plane/internal/fleet/verbs/peer.go`, `verbs/inbox.go` — the two handlers over
  `control.Dispatcher` (new)
- `control-plane/internal/fleet/protocol/*_test.go`, `verbs/{peer,inbox}_test.go` (new)
- `testdata/panacea-control/v1/peer-*.json`, `inbox-*.json` — the `peer` and `inbox` args and
  results, in task 09's fixture set (new; task 09 owns the directory)

The file map lists `messages.go`, `inbox.go`, `ask.go`, `limits.go` and `resolve.go` for tasks
28, 29 and 62. `resolve.go` is task 29's. `threads.go` is added here because opening and
appending are neither a message's schema nor a limit.

**Interfaces:**
- Produces:
  - `protocol.Service` with `Send(ctx, c control.Caller, s Send) (SendResult, error)`,
    `Inbox(ctx, c control.Caller) (Envelope, error)`, `SweepDeadlines(ctx) error`, and
    `Thread(ctx, org, id string) (Thread, bool, error)`
  - `protocol.OpenForConflict(ctx, org, project, conflict string, orchA, orchB model.AgentID) (string,
    error)` — task 25 calls it when a conflict opens; it is the same `Open` with a `conflict`
    subject, `orchA` as `openerAgentId` under its owner's delegation (route 60 requires the opener be the delegation owner's Orchestrator)
  - `protocol.Canonical(r Resolution) string` — task 29 reads it to find the proposal being
    accepted; task 62 to decide whether a `counter` is new
  - `protocol.ReadLog` rows: `(orchestrator, id, at)` for every message and card id an `inbox`
    or templated answer delivered — task 31's "ids the session read since its human last spoke"
  - `verbs.Threads` for task 22 (`Party`), implemented over `Thread` state
- Consumes:
  - task 19: `control.Dispatcher.Handle`, `control.Handler`, `control.Caller{Org, Project, Agent,
    Owner, Class}` (as task 22 cites them, `task-22.md:49-52`), dispatch's class check (`peer`
    and `inbox` are orchestrator-only in task 09's verb table)
  - task 09: `model.AgentID`, `model.Owner`, `model.EventPeerMessage`; `wire.Refuse(code, msg)`
  - task 02: `teamtext.New`, `KindMessage` (600), `KindReason` (300), `ValidGlob`, `ValidPath`,
    `MaxGlobs`; `teamtext.Text` as the wire form of every text field
  - task 22: `verbs.Sources` (`Cards`, `Briefs`, `People`, `Store` leases, `Activity`) for the
    templated answer, `task-22.md:76-164`
  - task 23: `leases.Engine.Live` for the lease half of a card
  - task 26: `turns.Queue.Enqueue` for `peer.message`; `turns.Scheduler.RecordIssued` for every
    `ms-…` an Orchestrator sends during a turn
  - task 11: `outbox.Enqueue`, `OpThreadCreate`, `OpThreadPatch`, `OpMessageAppend`,
    `outbox.Gate`; task 07's routes 60–62 are their targets, and their bodies are task 07's
  - `ids.Wide("th")`, `ids.Wide("ms")` (`internal/ids/ids.go:61` on `feat/hub`)
- Requires of **task 30**: a `Permissions.FreeTextAnswers(ctx, owner model.Owner) bool` (the
  standing permission, from the owner's rows) and an `Escalator` with
  `InformOwnHuman(ctx, orch model.AgentID, msg string) error` and `ThreadExhausted(ctx, org,
  thread string, why Exhaustion) error`. This task defines both interfaces and ships
  `DenyAll` and a recording fake; task 30 provides the real ones and wires them.
- Requires of **task 10**: the `protocol.Store` methods below in `memstore`, `pgstore` and
  `storetest.Suite`.

**`messages.go`, the types, complete.** The JSON is the `peer` verb's `args` on the ST.1 wire and,
field for field, task 07's route 62 `fields` (`task-07.md:99-111`), so Fleet forwards typed
fields without re-shaping them.

```go
package protocol

import (
	"encoding/json"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/teamtext"
)

type Kind string

const (
	KindAsk      Kind = "ask"
	KindAnswer   Kind = "answer"
	KindInform   Kind = "inform"
	KindPropose  Kind = "propose"
	KindAccept   Kind = "accept"
	KindReject   Kind = "reject"
	KindCounter  Kind = "counter"  // kind_not_enabled until task 62
	KindWithdraw Kind = "withdraw"
)

type ResolutionKind string

const (
	ResYield       ResolutionKind = "yield"
	ResSequence    ResolutionKind = "sequence"
	ResProceedBoth ResolutionKind = "proceed-both"
	ResSplit       ResolutionKind = "split"    // kind_not_enabled until task 62
	ResBuildOn     ResolutionKind = "build-on" // kind_not_enabled until task 62
)

// Resolution is task 07's Resolution: exactly the fields of its kind.
type Resolution struct {
	Kind  ResolutionKind             `json:"kind"`
	Globs []string                   `json:"globs,omitempty"` // yield: the sender's own
	First model.AgentID              `json:"first,omitempty"` // sequence: a participant
	Paths []string                   `json:"paths,omitempty"` // proceed-both: literal paths
	Sides map[model.AgentID][]string `json:"sides,omitempty"` // split (task 62)
	Base  model.AgentID              `json:"base,omitempty"`  // build-on (task 62)
}

type SubjectKind string

const (
	SubjectTask     SubjectKind = "task"
	SubjectLease    SubjectKind = "lease"
	SubjectPath     SubjectKind = "path"
	SubjectConflict SubjectKind = "conflict"
)

type Subject struct {
	Kind SubjectKind `json:"kind"`
	Ref  string      `json:"ref"` // tk-… | ls-… | a repo-relative path | cf-…
}

// Send is the peer verb's args. Thread is empty only when opening one, and
// then To names the other Orchestrator. The sender comes from the stamp.
type Send struct {
	Kind       Kind          `json:"kind"`
	Thread     string        `json:"thread,omitempty"`
	To         model.AgentID `json:"to,omitempty"`
	Subject    *Subject      `json:"subject,omitempty"`    // ask, inform
	InReplyTo  string        `json:"inReplyTo,omitempty"`  // answer
	Resolution *Resolution   `json:"resolution,omitempty"` // propose, counter
	Proposal   string        `json:"proposal,omitempty"`   // accept, reject, withdraw, counter
	Text       string        `json:"text,omitempty"`
}

// SendResult is the peer verb's result.
type SendResult struct {
	Message  string       `json:"message"` // ms-…
	Thread   string       `json:"thread"`  // th-…
	Rounds   Rounds       `json:"rounds"`
	Deadline time.Time    `json:"deadline"` // the effective one
	State    string       `json:"state"`    // open | escalated (this message closed it)
	Answer   *CardAnswer  `json:"answer,omitempty"` // ask only: Fleet's templated answer
}

type Rounds struct {
	Mine   int `json:"mine"`
	Theirs int `json:"theirs"`
	Cap    int `json:"cap"` // 3
}

// Stored is one message as Fleet holds it: typed fields, and text labelled.
type Stored struct {
	ID        string          `json:"id"`
	Thread    string          `json:"thread"`
	Sender    model.AgentID   `json:"senderAgentId"`
	Owner     model.Owner     `json:"-"`
	Kind      Kind            `json:"kind"`
	Fields    json.RawMessage `json:"fields"`
	Text      *teamtext.Text  `json:"text,omitempty"`
	Templated bool            `json:"templated"`
	Counting  bool            `json:"-"` // a round, decision (a)
	Fresh     bool            `json:"-"` // new typed content, decision (b)
	Seq       int64           `json:"-"`
	At        time.Time       `json:"at"`
}
```
