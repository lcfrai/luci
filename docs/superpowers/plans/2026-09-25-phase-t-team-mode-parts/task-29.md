### Task 29: Resolutions `yield`, `sequence`, `proceed-both`; DECIDED WITHOUT YOU

A negotiation can decide exactly five things (§6.2's resolutions table), and TM1-α builds three of
them: `yield`, `sequence` and `proceed-both`. `split`, `counter` and `build-on` are task 62's
(TM2). When every party has accepted a proposal, **Fleet applies its typed parameters and nothing
else**. The Orchestrators' prose never has an effect (§6.1, "only typed fields have effects").
Then each Orchestrator directs its **own** workers to match (task 31); Fleet moves no worker, and
neither side can move the other's. Finally, and whether or not either Orchestrator says a word to
its human, **Fleet itself writes a DECIDED WITHOUT YOU item to both humans** from the thread's
typed state, so a conflict cannot be closed in silence by an agent that simply does not inform
(§6.2, `07-critique-security.md` I1).

Four properties, each pinned by a test.

1. **A resolution changes only what its parameters name.** `yield` releases the proposer's own
   leases on exactly the named globs; `sequence` demotes the second side's overlapping leases to
   `intent` and records one land-order hint; `proceed-both` marks the conflict `accepted` on
   exactly the named paths. Nothing else in the project moves.
2. **No resolution touches the other side.** `yield` is a promise the proposer makes about its
   own side, so it can only name globs its own side holds; an accept can never yield the
   acceptor's leases. `sequence` changes the second side's lease **mode**, which both sides
   agreed to by accepting; it issues no directive to anyone.
3. **An accept that §6.5 says needs a human does not apply.** It becomes a pending decision (task
   30 writes the `decide`, and task 30's rules decide which accepts those are). Here that is one
   hook, `Rules.Classify`, and until task 30 lands its stub already sends `proceed-both` on tier H
   or M to a decide, which is the one always-decide row TM1-α could reach.
4. **Both humans hear of every applied resolution**, from Fleet, with the typed state, in the same
   transaction that applies it.

Four decisions §6.2 left open, made here.

**(a) "Every party has accepted" is the other side's `accept`.** A thread has two participants
(task 07's route 60: "exactly 2 Orchestrators, different owners"), and proposing is accepting
one's own proposal. So the one `accept` that task 28 stores, on a live proposal by the other side,
is the moment of resolution. `Resolve` is called from task 28's `Service.Send` step 9, after the
accept commits and with the thread already updated.

**(b) A yield names whole globs its side holds.** Leases are glob lists, and "`auth/**` minus
`auth/session.go`" is not a glob. So every yielded glob must equal, as a string, a glob of a live
lease held by the proposer's side (its own workers, and its own task intents). This is checked
**at propose time** (`CheckProposal`, called from task 28's step 4) with `resolution_not_held`,
and **again at accept time**, since leases move between the two; a proposal whose globs are no
longer held when accepted is `resolution_stale` and applies nothing. A lease left with some globs
is re-issued: the old lease goes to `yielded`, a new one with the remaining globs (same holder,
mode, task) is `granted` or `contested` by task 23's own rules.

**(c) "Until the first lands" in TM1-α is "until the first side's task is done".** Phase 3's
land queue is TM1's (task 34 onward) and task 61's in TM2. Until then the only land-shaped fact
Fleet has is the first side's task reaching `done` (task 24's `task update`). So the hint is a
Fleet row `(project, conflict, first, second)`; when the first side's task is `done` or
`abandoned`, the second side's demoted leases return to their original mode. In TM1 the trigger
becomes phase 3's land event; the row and the restore are unchanged.

**(d) DECIDED WITHOUT YOU is an `inform` escalation written by Fleet.** It is task 07's route
64 with `fromAgentId: ""` (Fleet for itself, `task-07.md:123-125`), `kind: "inform"`, under each
owner's own delegation, one per human. Its `question` is a product template filled from typed
fields only (the resolution kind, the other owner's display name, globs or paths, the conflict
id), never from a message's text. §6.5's "act; Fleet informs" rows are this same item.

**Files:**
- `control-plane/internal/fleet/protocol/resolve.go` — `Resolver`: `CheckProposal`, `Resolve`,
  `ApplyApproved`, `TaskEnded`; the three appliers (new)
- `control-plane/internal/fleet/protocol/decided.go` — `Decided`: the two inform items, the
  template (new)
- `control-plane/internal/fleet/leases/yield.go` — `Engine.Yield`, `Engine.Demote`,
  `Engine.Restore`: the `yielded` transition task 23's table assigns to task 29 (new, in task 23's
  package)
- `control-plane/internal/fleet/protocol/{resolve,decided}_test.go`,
  `control-plane/internal/fleet/leases/yield_test.go` (new)
- `control-plane/internal/fleet/protocol/service.go` — task 28's `Service.Send`: the
  `CheckProposal` call in step 4 and the `Resolve` call in step 9; `SendResult` gains
  `Outcome *Outcome` (`json:"outcome,omitempty"`), so the acceptor sees what was applied in its
  own `peer` result (modified; task 28's file)
- `control-plane/internal/fleet/protocol/inbox.go` — task 28's `Envelope` gains `Resolved
  []Outcome` (`json:"resolved"`): every outcome on the caller's threads since its last `inbox`,
  so the proposer's side learns it the same way it learns everything else (modified; task 28's
  file, plus one fixture `inbox-resolved-yield.json`)
- `control-plane/internal/fleet/verbs/task.go` — task 24's `task update` path calls
  `Resolver.TaskEnded` on `done` and `abandoned` (modified; task 24's file, one call)

`decided.go` and `yield.go` are not in the file map; the map's `resolve.go` row covers the
resolutions, and the lease moves belong in the lease engine, not beside the protocol, so that task
23's transition-table test covers them.

**Interfaces:**
- Produces:
  - `protocol.Resolver` with `CheckProposal(ctx, t Thread, sender model.AgentID, r Resolution)
    error`, `Resolve(ctx, t Thread, accept Stored) (Outcome, error)`, `ApplyApproved(ctx, org,
    thread, proposal, escalation string) (Outcome, error)` (task 30 calls it when a human approves
    a pending accept), `TaskEnded(ctx, org, task string) error`
  - `protocol.Action`, `protocol.Verdict`, and the `Rules` interface task 30 implements
  - `protocol.Outcome` — what was applied, for task 31's instructions and task 52's read model
  - `leases.Engine.Yield`, `Demote`, `Restore`
  - `protocol.DecidedWithoutYou(t Thread, o Outcome, names Names) [2]outbox.Item`
- Consumes:
  - task 28: `Thread`, `Stored`, `Resolution` and its kinds, `Canonical`, `protocol.Store`, the
    `Escalator` interface (extended below)
  - task 23: `leases.Engine.Live`, `Reevaluate`, the transition table (`granted`·`contested` →
    `yielded`, task 29's row), `leaseEvent` for every Hub lease event
  - task 22: `verbs.Conflicts.Open` for the conflict's tier and paths; `verbs.People.DisplayName`
  - task 26: `turns.Queue.Enqueue` for the other side's wake after a resolution
  - task 11: `outbox.OpConflictPatch`, `OpThreadPatch`, `OpEscalation`, `OpLeaseEvent`; task
    06's route 58 and task 07's routes 61 and 64 are their targets
- Requires of **task 30**: `Rules.Classify(ctx, Action) (Verdict, error)`, and on the `Escalator`
  one more method, `AcceptNeedsDecision(ctx, t Thread, accept Stored, a Action) error`. Until
  task 30 lands, `StubRules` (below) is wired.
- Requires of **task 10**: `fleet.land_hints` and `fleet.lease_demotions` (below), and the
  pending-accept marker on a thread's live proposal.

**`resolve.go`, the types, complete.**

```go
package protocol

import (
	"context"
	"errors"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

var (
	ErrNotHeld = errors.New("protocol: resolution_not_held")  // at propose
	ErrStale   = errors.New("protocol: resolution_stale")     // at accept
	ErrTier    = errors.New("protocol: proceed-both paths not in the conflict")
)

// Action is what an accept would do, typed, as task 30's rules read it.
type Action struct {
	Resolution ResolutionKind
	Tier       string          // the conflict's P | H | M | S; "" for a free thread
	Proposer   model.AgentID   // the side that proposed
	Acceptor   model.AgentID
	Globs      []string        // yield
	First      model.AgentID   // sequence
	Paths      []string        // proceed-both
	EditedByProposer []string  // globs the proposer's side has edited (TM1; empty in α)
	ForecastWait time.Duration // sequence: forecast wait of the second side; 0 if unknown
}

type Verdict string

const (
	VerdictAct    Verdict = "act"    // apply; Fleet informs both
	VerdictDecide Verdict = "decide" // do not apply; task 30 asks the human(s)
)

type Rules interface {
	Classify(ctx context.Context, a Action) (Verdict, error)
}

// StubRules is wired until task 30: the one always-decide row TM1-α can
// reach, proceed-both on H or M; everything else acts.
type StubRules struct{}

func (StubRules) Classify(_ context.Context, a Action) (Verdict, error) {
	if a.Resolution == ResProceedBoth && (a.Tier == "H" || a.Tier == "M") {
		return VerdictDecide, nil
	}
	return VerdictAct, nil
}

// Outcome is what Resolve did.
type Outcome struct {
	Thread     string
	Conflict   string
	Resolution ResolutionKind
	Applied    bool            // false: pending a human decision
	Yielded    []string        // ls-… moved to yielded
	Reissued   []string        // ls-… granted/contested with the remaining globs
	Demoted    []string        // ls-… moved to intent (sequence)
	First      model.AgentID   // sequence
	Paths      []string        // proceed-both
	At         time.Time
}
```

**`Resolve`, the order.** One transaction from step 3 to step 6; the verdict is read before it.

1. Find the accepted proposal in the thread (`accept.Fields.proposal`), its sender (the
   proposer) and its `Resolution`. The accept's sender is the acceptor.
2. Build the `Action` (tier from `verbs.Conflicts.Open`, `""` for a free thread) and call
   `Rules.Classify`. **`decide`**: mark the proposal `awaiting-human` in the thread's live set,
   call `Escalator.AcceptNeedsDecision`, return `Outcome{Applied: false}`. Nothing else moves.
3. Re-check the parameters against current state (`resolution_stale` on any miss): `yield`'s
   globs still held by the proposer's side; `sequence`'s `first` still a participant; the
   conflict still open.
4. Apply, per kind (below), writing every lease event through task 23's `leaseEvent`.
5. Close: thread → `resolved` (route 61, `OpThreadPatch`); conflict → `resolved` with the
   resolution for `yield` and `sequence`, `accepted` with `proceed-both` for `proceed-both`
   (route 58, `OpConflictPatch`, `task-06.md:210-215`).
6. Write `DecidedWithoutYou`'s two items (`OpEscalation`), and a `thread.resolved` event (task
   26's kind, immediate) for the proposer's Orchestrator, which must now direct its workers; the
   acceptor is mid-turn and sees the `Outcome` in its `peer` result.
7. After commit, `Engine.Reevaluate(org, project)`: a lease that was contested only by what was
   just yielded becomes `granted`.

`ApplyApproved(org, thread, proposal, es)` is steps 3 to 7 for a proposal marked
`awaiting-human`, called by task 30 once the human (or, for a two-human decide, both) chose the
applying option; the approving `es-…` id goes into the conflict patch's audit metadata through
task 07, and into the `Outcome`. A human who declines leaves the thread `open` with the proposal
withdrawn; the rounds already spent stay spent.

**The three appliers.**

| Kind | Leases | Other Fleet state | Hub |
|---|---|---|---|
| `yield` | for each live lease L of the proposer's side with a yielded glob: `Yield(L, globs)` → L `yielded`; if globs remain, a new lease with them | none | L's `yielded` event, the new lease's `requested` and outcome |
| `sequence` | every live lease of the **second** side whose globs overlap the conflict's paths (task 02's `Match`): `Demote(L)` → mode `intent`; one row in `fleet.lease_demotions` with the old mode | a row in `fleet.land_hints (org, project, conflict, first, second, created_at)` | a mode-change lease event per demoted lease; the conflict patch |
| `proceed-both` | none | none | the conflict patch, `accepted`, `proceed-both`; `paths` must be a non-empty subset of the conflict's paths, else `ErrTier`-class `bad_request` at propose |

"The proposer's side" and "the second side" are the Orchestrator and every worker whose
`parentAgentId` is that Orchestrator (task 20's agent records). The first side is `First`; the
second is the other participant's side.

**`TaskEnded(org, task)`** looks up `land_hints` where the task belongs to the first side's
agent that was party to the conflict; for each, `Restore` every row of `lease_demotions` for that
hint (mode back to the stored one, one lease event each), deletes the rows, and enqueues a
`sequence.go` event (task 26's kind) for the second side's Orchestrator so it can tell its
workers the way is clear. Idempotent: a second call finds no rows.

**`leases/yield.go`, the surface, complete.**

```go
package leases

// Yield moves l to yielded and, when globs of l remain, requests a new
// lease with them for the same holder, mode and task, in one transaction.
// It fails with ErrNotHeld-class agent_not_owner unless every yielded glob
// is, as a string, one of l.Globs.
func (e *Engine) Yield(ctx context.Context, org string, l model.Lease, globs []string) (reissued string, err error)

// Demote sets l's mode to intent (a hold or bulk becomes intent) and writes
// a lease event; state is unchanged. Returns the previous mode.
func (e *Engine) Demote(ctx context.Context, org string, l model.Lease) (was model.LeaseMode, err error)

// Restore sets l's mode back to was, if l is still live; a lease that ended
// meanwhile is skipped, not an error.
func (e *Engine) Restore(ctx context.Context, org string, id string, was model.LeaseMode) error
```

A mode change is not a state change, so task 23's transition table is untouched by `Demote` and
`Restore`; the lease event carries the new `mode` with the unchanged `state`, which task 01's
§7.16 lease-event body already has fields for (`task-23.md:155`, `"mode"`). The Hub's lease
history therefore shows the demotion.
