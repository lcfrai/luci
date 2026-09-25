### Task 37: Budgets: turn budget, human bypass, per-counterpart cap and digest, the spend split

**Slice:** TM1. **Focused time:** 3 h. **dependsOn:** 20 (a fresh gateway key per Orchestrator
turn, with its tags), 26 (the queue, `Ready`, the scheduler's claim), P6-09 (the usage rollup);
10 (`CounterpartTurns`, `AddCounterpartTurn`), 30 (`InformOwnHuman`) and 31 (directives) through
them. **Seam:** S6.2 (phase 6's budgets), consumed.

An Orchestrator reads every peer message against its **owner's** quota (§6.6, P4). Without this
task a teammate's Orchestrator could spend Alice's quota for her, or bury her own message under a
flood of peer events (`07` I3). `05-design.md` §4.4 gives three rules, and each becomes a property
pinned by a test, not by review:

1. **A human's own events skip the budget and the queue.** Her message starts the next turn at
   once; her `pause` is executed by Fleet directly, with no turn at all.
2. **Peer-triggered cost is metered per counterpart and shown.** Past 10 turns an hour from one
   counterpart, that counterpart's events fold into one digest turn every 15 minutes.
3. **When the budget is spent, non-human events queue and the owner is told.** Nothing is dropped.

This task is a **gate** between task 26's `Ready` and its `ClaimTurn`. It never reads event text
(task 26's `Event` has none), never adds text to the turn input (task 26's `Input(n int)` stays
the only input), and never changes a batch's origin: `OriginOf` still computes it from what the
gate admits.

Five decisions the scope left open, made here so nobody re-derives them.

**(a) The human bypass is structural, in three places.** (1) `human.message` becomes an
**immediate** kind: task 26 left it coalescing for up to 60 s and recorded that latency as a gap
(`task-26.md:133-137`); this task closes it by adding the kind to `turns.Immediate`. (2) `Ready`
sorts an own-human `human.message` **before every other event**, so a batch capped at `MaxBatch`
(50) can never leave it behind 50 peer events. (3) The gate admits any batch that holds one,
whatever the budget, the pause or the counterpart caps say. The one delay left is a turn already
in flight: Fleet never interrupts a running turn, so a human message can wait for at most the
current turn's end (task 26's `TurnTimeout`, 10 minutes at worst). That residual is stated, not
hidden, and `TestPeerFloodCannotDelayHuman` bounds it to one turn.

**(b) The turn budget counts begun turns in a sliding hour, per Orchestrator.** Default 30, the
team policy's `orchestratorTurnsPerHour` (`task-01.md:474`, range 1 to 30). The owner may set a
lower number for their own Orchestrator, never a higher one; human-origin turns are **counted**
(so the owner sees them) but **never refused**. The owner's setting is Fleet state in Postgres, not
memory: losing it would silently loosen a budget the owner tightened. It is not a Hub record,
because it is a preference about the owner's own agent, not a decision or an accountability fact;
if the owner later wants it audited, it moves to the Hub (listed in the gaps).

**(c) The per-turn cap is a spend cap on the turn's key, not a token count.** The scope says
"per-turn token cap". Nothing in the pod may count tokens (OD-8; task 27 picks `--output-format
text` for that reason), and the gateway enforces spend per key (`max_budget`, phase 6 task 07's
mint, `task-07.md:840-860` on `feat/phase-6-gateway`), not tokens. So the owner sets
`turnCapCents` (default 50, range 5 to 500), task 20 passes it into the per-turn mint as a
ceiling that can only **tighten** phase 6's `budget.Slice`, and the gateway stops the turn's calls
when it is spent. Tokens are still **reported** per turn, from task 26's `TokenMeter`. This
deviates from the scope's word and says so; the owner-facing line is *"Each coordination turn may
spend up to A$0.50."*

**(d) A turn's counterparts are resolved by Fleet, not read from `Event.Author`.** Task 24 sets
`Author` to an **agent** id for a worker report (`task-24.md:165`) while task 09 types it as an
`Owner` (`task-09.md:248`), so it cannot be trusted to name the other person. The gate asks one
interface, `Counterparts.Of(ctx, e) (model.Owner, bool)`, which `cmd/fleetd` implements over the
records the event refers to: a `peer.message`'s sender (task 28's thread store), a
`conflict.opened`'s or `thread.resolved`'s other side (task 25's working copy, task 29), a
`sequence.go`'s other side. Every other kind has no counterpart. A turn counts **once against
each** distinct counterpart it admitted events from; its tokens are attributed to that
counterpart when there is exactly one, and to `several` otherwise, never split by guesswork.

**(e) The digest is a hold, not a new turn kind.** Once a counterpart has 10 turns in the last
hour (task 10's `CounterpartTurns`), the gate **holds** that counterpart's events: it admits none
of them, and they stay unbatched in the queue, until 15 minutes after that counterpart's last
admitted turn. Then it admits **all** of that counterpart's held events at once, one batch, one
turn: the digest. The turn input is still `Input(n)`; the Orchestrator reads what happened through
`panacea inbox` as always. Nothing is dropped and nothing new is said to the model.

**Files:**
- `control-plane/internal/fleet/budget/budget.go` — `Settings`, `Gate`, `Admit`, the exhaustion
  notice (new)
- `control-plane/internal/fleet/budget/counterpart.go` — `Counterparts`, the hold and the digest
  rule (new)
- `control-plane/internal/fleet/budget/pause.go` — `HumanPause`, `HumanResume`: Fleet executes
  them directly, no turn (new; the skeleton's map lists only the first two files)
- `control-plane/internal/fleet/budget/split.go` — `Split`, the *workers* and *coordination*
  figures for task 56 (new)
- `control-plane/internal/fleet/budget/{budget,counterpart,pause,split}_test.go` (new)
- `control-plane/internal/fleet/server/budget.go` — the four routes below (new)
- `control-plane/internal/fleet/turns/queue.go` — `Immediate` gains `human.message`; `Ready`
  sorts own-human events first (modify, task 26's file, two anchored hunks)
- `control-plane/internal/fleet/turns/scheduler.go` — `Scheduler.Gate`; `TryBegin` calls
  `Gate.Admit` between `Ready` and `ClaimTurn`, and `Gate.Record` after a won claim (modify, one
  anchored hunk)
- `control-plane/internal/fleet/store/migrations/00NN_budget.sql` — numbered after the last Fleet
  migration at merge (new)
- `control-plane/internal/fleet/store/{memstore,pgstore}.go`, `.../storetest/suite.go` — the
  `budget.Store` methods (modify, task 10's files)
- `control-plane/cmd/fleetd/main.go` — builds the gate, the `Counterparts` adapter and the routes
  (modify, one anchored hunk after task 26's scheduler)

**Interfaces:**
- Produces:
  - `budget.Gate` with `Admit(ctx, orch, owner, pending, now) (Admitted, error)` and
    `Record(ctx, Admitted, turnID) error`, consumed by task 26's scheduler;
  - `(*Gate).TurnCapCents(ctx, orch) int`, consumed by task 20's per-turn mint;
  - `budget.HumanPause` / `HumanResume`, consumed by task 52's panel actions through `fleetbe`;
  - `budget.Split`, consumed by task 56's spend line;
  - routes `GET`/`PUT /v1/projects/{p}/orchestrator/budget`, `POST /v1/projects/{p}/pause`,
    `POST /v1/projects/{p}/resume`, `GET /v1/projects/{p}/spend`, each through task 12's
    `Server.Route` under `VerbSessionsOwn` (`task-12.md:520-527`, `:1076`), so each joins Z5.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 26: `turns.Queued`, `turns.Ready`, `turns.Immediate`, `turns.MaxBatch`,
    `turns.TokenMeter.TurnTokens(ctx, turnID)`, `Scheduler.TryBegin` (`task-26.md:180-257`,
    `:456-486`, `:535-578`);
  - task 10: `CounterpartTurns(ctx, org, owner, counterpart, since)` and
    `AddCounterpartTurn(ctx, org, owner, counterpart, at)` (`task-10.md:115-117`, `:147`);
  - task 03: `hubapi.TeamPolicy.OrchestratorTurnsPerHour` (`task-03.md:908`), read through task
    11's `HubState` (its `TeamPolicy(project)` read is **required of task 11**, see the gaps);
  - task 30: `(*escalate.Service).InformOwnHuman(ctx, orch, msg)` (`task-30.md:387`);
  - task 31: `directive.IssueHuman(ctx, owner, worker, kind)` (**required of task 31**, not yet
    written) and task 38's `delivery.Ladder.Pause` (`task-38.md`, this writer's own);
  - phase 6 task 09: a usage sum by **agent** (`usage.Store.Sum` takes `person`, `project` or
    `team` only, `task-09.md:514-517` on `feat/phase-6-gateway`), **required of P6-09**, see the
    gaps.

#### `budget.go` (complete)

```go
// Package budget is §4.4's three rules as a gate between task 26's Ready and
// ClaimTurn. It reads kinds and ids, never text, and adds no text to a turn.
package budget

import (
	"context"
	"errors"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/turns"
)

const (
	DefaultTurnsPerHour = 30              // §4.4; the team policy's ceiling (1–30)
	DefaultTurnCapCents = 50              // decision (c)
	MinTurnCapCents     = 5
	MaxTurnCapCents     = 500
	CounterpartCap      = 10              // turns an hour from one counterpart (§4.4)
	DigestEvery         = 15 * time.Minute
	Window              = time.Hour
	NoticeEvery         = time.Hour       // at most one "budget spent" inform an hour
)

var (
	ErrAboveTeam   = errors.New("budget: turnsPerHour is above the team policy's")
	ErrCapRange    = errors.New("budget: turnCapCents is outside 5 to 500")
	ErrTurnsRange  = errors.New("budget: turnsPerHour is outside 1 to the team policy's")
)

// Settings is an owner's own setting for their Orchestrator: Fleet state.
type Settings struct {
	TurnsPerHour int // 1 .. team policy's orchestratorTurnsPerHour
	TurnCapCents int // 5 .. 500
	UpdatedAt    time.Time
}

// Store is what the gate needs of task 10's fleet store, beside its two
// counterpart methods. Every method is one transaction.
type Store interface {
	Settings(ctx context.Context, orch model.AgentID) (Settings, bool, error)
	PutSettings(ctx context.Context, orch model.AgentID, s Settings) error
	// LogTurn records a begun turn: its origin, counterparts, digest flag.
	LogTurn(ctx context.Context, row TurnLog) error
	// TurnsSince counts begun turns of orch at or after since, all origins.
	TurnsSince(ctx context.Context, orch model.AgentID, since time.Time) (int, error)
	// LastCounterpartTurn is the begin time of orch's last turn that admitted
	// an event from counterpart; ok == false if none in the window.
	LastCounterpartTurn(ctx context.Context, orch model.AgentID,
		counterpart model.Owner) (time.Time, bool, error)
	SetTurnTokens(ctx context.Context, turnID string, in, out int64) error
	TurnLogs(ctx context.Context, orch model.AgentID, from, to time.Time) ([]TurnLog, error)
	Paused(ctx context.Context, owner model.Owner, project string) (bool, error)
	SetPaused(ctx context.Context, owner model.Owner, project string, paused bool) error
	LastNotice(ctx context.Context, orch model.AgentID) (time.Time, bool, error)
	SetLastNotice(ctx context.Context, orch model.AgentID, at time.Time) error
	CounterpartTurns(ctx context.Context, org string, owner, counterpart model.Owner,
		since time.Time) (int, error) // task 10's, unchanged
	AddCounterpartTurn(ctx context.Context, org string, owner, counterpart model.Owner,
		at time.Time) error // task 10's, unchanged
}

// TurnLog is one begun turn. Tokens are nil until the meter answers.
type TurnLog struct {
	Orch         model.AgentID
	TurnID       string        // ot-…
	Origin       turns.Origin
	Counterparts []model.Owner // sorted, distinct; empty for a human-only batch
	Digest       bool
	Began        time.Time
	InTokens     *int64
	OutTokens    *int64
}

// Policy is the team policy's ceiling (task 11's HubState, required there).
type Policy interface {
	TurnsPerHour(ctx context.Context, org, project string) (int, error)
}

// Notifier is task 30's Fleet-written inform to the orchestrator's own human.
type Notifier interface {
	InformOwnHuman(ctx context.Context, orch model.AgentID, msg string) error
}

// NoticeSpent is the one sentence the owner is told. Product text, not an
// agent's; it names no count of turns left and no time (OD-27).
const NoticeSpent = "Your Orchestrator has used this hour's coordination budget. " +
	"Team events are waiting; your own messages still go through."

type Gate struct {
	Store        Store
	Policy       Policy
	Counterparts Counterparts // counterpart.go
	Notify       Notifier
	Org          func(orch model.AgentID) (org, project string) // from task 10's agent row
	Now          func() time.Time
}

// Admitted is the gate's answer: the events that may run now, and why.
type Admitted struct {
	Orch         model.AgentID
	Owner        model.Owner
	Batch        []turns.Queued // nil: no turn now; the rest stay queued
	Counterparts []model.Owner
	Digest       bool
	HumanBypass  bool
}
```

**`Admit`, in order.** It takes the Orchestrator's **pending** events, not `Ready`'s batch,
because a digest admits held events that `Ready`'s cut may have left out. It calls
`turns.Ready(pending, now)` first; `nil` means no turn is due and it returns at once. Otherwise it
is a function of that batch, `pending` and five store reads, and it never writes: `Record` writes
after `ClaimTurn` wins, so a lost claim race counts nothing twice.

1. **Human bypass.** If the batch holds a `human.message` (task 26's `Enqueue` has already
   refused any whose author is not the owner, `task-26.md:439-441`), admit **the whole batch**,
   `HumanBypass: true`. Peer events in it ride along; that is cheaper than a second turn and it
   cannot delay the human, who is already first.
2. **Human pause.** If `Paused(owner, project)`, admit nothing that is not a human event; return
   `Batch: nil`.
3. **Turn budget.** `limit = min(Settings.TurnsPerHour or DefaultTurnsPerHour,
   Policy.TurnsPerHour)`. If `TurnsSince(orch, now-Window) >= limit`, admit nothing; if
   `LastNotice` is older than `NoticeEvery` (or absent), call `Notify.InformOwnHuman(orch,
   NoticeSpent)` and `SetLastNotice`. The events stay queued; the next sweep after the oldest turn
   leaves the window admits them.
4. **Counterpart holds** (`counterpart.go`). Resolve each event's counterpart. For each
   counterpart at or over `CounterpartCap` in the window, **drop its events from this batch**
   unless its digest is due (`now - LastCounterpartTurn >= DigestEvery`); if a digest is due,
   admit **every** held event of that counterpart (from task 26's `PendingEvents`, still unbatched),
   `Digest: true`, and no other over-cap counterpart's.
5. What is left is the batch, cut to `turns.MaxBatch`. Empty means `Batch: nil`.

`Record(ctx, a, turnID)` writes `LogTurn` and one `AddCounterpartTurn` per counterpart, in one
`Store.Tx`. `TurnCapCents(ctx, orch)` returns the owner's setting or the default; task 20 calls it
at each per-turn mint.

A held event keeps its Orchestrator in task 26's `OrchestratorsDue` (it is unbatched and older
than the coalesce window), so `TryBegin` calls `Admit` on every scheduler tick while it is held
and gets `nil`. That costs one indexed read per tick per held Orchestrator and no turn; it is
accepted rather than adding a "held until" column to task 26's prefilter.

#### `counterpart.go` (complete)

```go
package budget

import (
	"context"
	"sort"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/turns"
)

// Counterparts resolves the other person behind an event (decision d).
// cmd/fleetd implements it over task 28's messages, task 25's working copy
// and task 29's threads. ok == false: the event has no counterpart.
type Counterparts interface {
	Of(ctx context.Context, owner model.Owner, e model.Event) (model.Owner, bool)
}

// peerKinds are the kinds that can have a counterpart. A worker report, a
// lease event and an escalation answer are the owner's own business.
var peerKinds = map[model.EventKind]bool{
	model.EventPeerMessage: true, model.EventConflictOpened: true,
	model.EventThreadResolved: true, model.EventSequenceGo: true,
}

// holds splits a batch by counterpart state: kept events, the over-cap
// counterparts whose events were held, and the one digest to run, if due.
func (g *Gate) holds(ctx context.Context, org string, orch model.AgentID, owner model.Owner,
	batch, pending []turns.Queued, now time.Time) (kept []turns.Queued, digest model.Owner,
	cps []model.Owner, err error) {
	over := map[model.Owner]bool{}
	seen := map[model.Owner]bool{}
	cpOf := func(q turns.Queued) (model.Owner, bool) {
		if !peerKinds[q.Event.Kind] {
			return "", false
		}
		return g.Counterparts.Of(ctx, owner, q.Event)
	}
	for _, q := range batch {
		cp, ok := cpOf(q)
		if !ok {
			kept = append(kept, q)
			continue
		}
		if _, known := over[cp]; !known {
			n, err := g.Store.CounterpartTurns(ctx, org, owner, cp, now.Add(-Window))
			if err != nil {
				return nil, "", nil, err
			}
			over[cp] = n >= CounterpartCap
		}
		if !over[cp] {
			kept = append(kept, q)
			seen[cp] = true
			continue
		}
		if digest == "" {
			last, ok, err := g.Store.LastCounterpartTurn(ctx, orch, cp)
			if err != nil {
				return nil, "", nil, err
			}
			if !ok || !now.Before(last.Add(DigestEvery)) {
				digest = cp
			}
		}
	}
	if digest != "" {
		for _, q := range pending { // every held event of that counterpart
			if cp, ok := cpOf(q); ok && cp == digest && q.TurnID == "" && !contains(kept, q) {
				kept = append(kept, q)
			}
		}
		seen[digest] = true
	}
	for cp := range seen {
		cps = append(cps, cp)
	}
	sort.Slice(cps, func(i, j int) bool { return cps[i] < cps[j] })
	return kept, digest, cps, nil
}
```

`contains` compares `Event.ID`. `model.EventSequenceGo` is task 26's `sequence.go` kind
(`task-26.md:126`); if task 09 spells it differently, task 09 wins. The digest turn's size is
bounded by `turns.MaxBatch` in `Admit`'s step 5, so a counterpart who sent 200 events in 15
minutes costs the receiver one turn of 50 and the next digest takes the rest, 15 minutes later.
Task 09's kind list (`task-09.md:213-221`) has no `EventThreadResolved` or `EventSequenceGo`
yet; task 26 already requires the first, and this task requires both (see the gaps).

#### `pause.go` (complete)

```go
package budget

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// Directives is task 31's writer for a human-origin directive: a dv-… Hub
// record (through the outbox) BEFORE delivery, origin "human", no turn.
type Directives interface {
	IssueHuman(ctx context.Context, owner model.Owner, worker model.AgentID,
		kind string) (dv string, err error)
}

// Deliverer is task 38's ladder, rung 4 for pause, rung 1 for resume.
type Deliverer interface {
	Pause(ctx context.Context, worker model.AgentID, dv string) error
	Resume(ctx context.Context, worker model.AgentID, dv string) error
}

// Workers lists the owner's own live workers on project (task 10).
type Workers interface {
	OwnWorkers(ctx context.Context, owner model.Owner, project string) ([]model.AgentID, error)
}

type Pauser struct {
	Store      Store
	Workers    Workers
	Directives Directives
	Deliver    Deliverer
}

// HumanPause is §4.4 rule 1's second half: Fleet executes it directly. It
// sets the owner's pause flag first (so no fleet-origin turn starts), then
// writes one pause dv-… per named worker and delivers it on rung 4. The
// agents argument, when non-empty, must be a subset of OwnWorkers; any
// other id is refused whole (ErrNotOwn), before anything is written.
func (p *Pauser) HumanPause(ctx context.Context, owner model.Owner, project string,
	agents []model.AgentID) ([]string, error)

// HumanResume clears the flag and writes one resume dv-… per worker it paused.
func (p *Pauser) HumanResume(ctx context.Context, owner model.Owner, project string,
	agents []model.AgentID) ([]string, error)
```

`ErrNotOwn` is `errors.New("budget: not the caller's own worker")`, mapped to 403
`agent_not_owner` (task 09's `CodeAgentNotOwner`, `task-09.md:439`) by the route. No Orchestrator turn is started, queued or consumed: the Orchestrator
learns of the pause from its next `panacea team` (its workers read `paused`), which is the same
fact source every other agent uses. The pause flag holds only **fleet-origin** turns; the owner
can still talk to their Orchestrator while everything is paused, and that message is a
human-origin turn (decision a), which is exactly what a person pausing a run to think wants.

#### `split.go` (complete)

```go
package budget

import (
	"context"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// AgentSpend is the usage sum by agent that phase 6 task 09 must add (its
// Store.Sum takes person, project or team only). ok == false: not known.
type AgentSpend interface {
	AgentCents(ctx context.Context, agent model.AgentID, day time.Time) (cents int64, ok bool, err error)
}

// SplitView is what task 56's spend line renders, in words and numbers.
// There is no percentage field on purpose (OD-27).
type SplitView struct {
	Day              string          `json:"day"`              // UTC, "2026-09-25"
	WorkersCents     *int64          `json:"workersCents"`     // null: not known
	CoordinationCents *int64         `json:"coordinationCents"` // the Orchestrator's turn keys
	ByCounterpart    []CounterpartUse `json:"byCounterpart"`
	TurnsPerHour     int             `json:"turnsPerHour"`    // the effective limit
	Spent            bool            `json:"spent"`           // this hour's budget is used
	Paused           bool            `json:"paused"`
}

// CounterpartUse is "coordination with Alice: 41k tokens today" (§4.4).
type CounterpartUse struct {
	UserID       string `json:"userId"`       // the counterpart, or "several"
	Turns        int    `json:"turns"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	Digested     bool   `json:"digested"`     // at least one digest turn today
}

func (g *Gate) Split(ctx context.Context, owner model.Owner, project string,
	orch model.AgentID, workers []model.AgentID, day time.Time) (SplitView, error)
```

`WorkersCents` is the sum over the owner's workers on the project, `CoordinationCents` the
Orchestrator's; either is `null` when phase 6 cannot answer, **never zero** (as task 26's
`tokens` rule). Tokens come from the `TurnLog` rows `Record` wrote and task 26's meter filled
through `SetTurnTokens`; a turn whose meter never answered counts its turn and adds no tokens.
There is deliberately no "turns used this hour" field: beside `TurnsPerHour` it would be an
"N of M", which OD-27 forbids on every team surface. The view says the limit (*"up to 30 turns an
hour"*) and, when it applies, the fact (*"this hour's budget is used"*); nothing in between.

#### The routes, `server/budget.go`

All four are registered with `s.Route(pattern, VerbSessionsOwn, h)`, so each is `member`/`admin`,
own only (`task-12.md:1076`), joins the Z5 matrix, and takes the owner **from the token**, never
the body (global constraint 3).

```jsonc
// GET /v1/projects/{p}/orchestrator/budget → 200
{ "turnsPerHour": 30, "turnCapCents": 50, "teamTurnsPerHour": 30, "spent": false, "paused": false }

// PUT /v1/projects/{p}/orchestrator/budget
{ "turnsPerHour": 12, "turnCapCents": 40 }
// 200: as GET. 422 bad_request, details.field "turnsPerHour" (0, or above teamTurnsPerHour)
//      or "turnCapCents" (outside 5..500). 404 not_found: the caller has no Orchestrator here.

// POST /v1/projects/{p}/pause     { "agents": ["ag-…"] }   // omitted or [] = every own worker
// POST /v1/projects/{p}/resume    { "agents": ["ag-…"] }
// 200 { "directives": ["dv-…"] }. 403 agent_not_owner: an id that is not the caller's own worker;
//      nothing is written.

// GET /v1/projects/{p}/spend?day=2026-09-25 → 200 SplitView (day defaults to today, UTC;
//      more than 31 days back is 422 bad_request, details.field "day")
```

A `PUT` above the team's ceiling is refused, not clamped: a silent clamp would tell the owner a
number that is not the one in force. When a team admin later **lowers** the team ceiling below an
owner's setting, the gate uses the lower (`min`, step 3) and `GET` shows both numbers, so the
owner sees why.

#### Task 26's files (modify: three anchored hunks)

```go
// queue.go — Immediate: human.message joins the two thread kinds (decision a).
func Immediate(k model.EventKind) bool {
	return k == model.EventHumanMessage || k == model.EventPeerMessage ||
		k == model.EventConflictOpened || k == model.EventThreadResolved
}

// queue.go — Ready's sort: own-human events first, then immediate, then oldest.
// (Enqueue has already refused a human.message from anyone but the owner.)
rank := func(q Queued) int {
	switch {
	case q.Event.Kind == model.EventHumanMessage:
		return 0
	case Immediate(q.Event.Kind):
		return 1
	}
	return 2
}
sort.SliceStable(open, func(i, j int) bool {
	if ri, rj := rank(open[i]), rank(open[j]); ri != rj {
		return ri < rj
	}
	return open[i].Event.At.Before(open[j].Event.At)
})

// scheduler.go — TryBegin: the gate replaces the bare Ready call.
adm, err := s.Gate.Admit(ctx, orch, a.Owner, pending, now) // Ready is called inside
if err != nil || adm.Batch == nil {
	return false, err
}
batch := adm.Batch
// … task 26's SessionFor, ClaimRow, ClaimTurn, unchanged …
if ok, err := s.Store.ClaimTurn(ctx, row); err != nil || !ok {
	return false, err
}
if err := s.Gate.Record(ctx, adm, row.TurnID); err != nil {
	s.Log.Warn("budget record", "turn", row.TurnID, "err", err) // the turn still runs
}
```

The `Agent` read moves above `Admit`, because the gate needs the owner. `Scheduler` gains one
field, `Gate interface{ Admit(…) (budget.Admitted, error); Record(…) error }`, declared in
`turns` as an interface so `turns` does not import `budget` (budget imports turns). A `Record`
failure is logged and the turn runs: refusing a won claim would strand its events, and the next
`TurnsSince` read undercounts by one turn at most. Task 26's `End`, on the turn's meter reading,
calls `SetTurnTokens` through the same interface (a third method, `Tokens(ctx, turnID, in, out)`).

#### `00NN_budget.sql` (complete; `NN` is the next Fleet migration number at merge)

```sql
CREATE TABLE fleet.orch_budgets (
  orch_id         text        PRIMARY KEY CHECK (orch_id ~ '^ag-'),
  turns_per_hour  int         NOT NULL CHECK (turns_per_hour BETWEEN 1 AND 30),
  turn_cap_cents  int         NOT NULL CHECK (turn_cap_cents BETWEEN 5 AND 500),
  updated_at      timestamptz NOT NULL
);
CREATE TABLE fleet.turn_log (
  turn_id        text        PRIMARY KEY CHECK (turn_id ~ '^ot-'),
  orch_id        text        NOT NULL,
  origin         text        NOT NULL CHECK (origin IN ('human', 'fleet')),
  counterparts   text[]      NOT NULL DEFAULT '{}',
  digest         boolean     NOT NULL DEFAULT false,
  began_at       timestamptz NOT NULL,
  in_tokens      bigint,
  out_tokens     bigint
);
CREATE INDEX turn_log_by_orch ON fleet.turn_log (orch_id, began_at);
CREATE TABLE fleet.owner_pauses (
  owner       text        NOT NULL,
  project_id  text        NOT NULL,
  paused_at   timestamptz NOT NULL,
  PRIMARY KEY (owner, project_id)
);
CREATE TABLE fleet.budget_notices (
  orch_id    text        PRIMARY KEY,
  last_at    timestamptz NOT NULL
);
```

`turn_log` rows older than 32 days are deleted by a `fleet-turnlog-prune` singleton, hourly (the
spend route reads at most 31 days back). The table holds ids, counts and times, no text. The
`CHECK` on `turns_per_hour` repeats the team policy's range (`task-01.md:482`) so the table
enforces it too; the tighter per-team ceiling is the route's.

- [ ] **Step 1: Write the failing tests first**

Fixture: Alice (owner `u-a`, Orchestrator `ag-a0`, workers `ag-a1`, `ag-a2`), Bob (`u-b`,
`ag-b0`), Carol (`u-c`, `ag-c0`); task 26's `memstore` queue; a fake clock; a fake
`Counterparts` that maps each event's `Ref` to an owner; a recording `Notifier`, `Directives` and
`Deliverer`; team policy ceiling 30.

