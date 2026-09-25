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
  - `budget.Gate` with `Admit(ctx, orch, owner, batch, now) (Admitted, error)` and
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
```

