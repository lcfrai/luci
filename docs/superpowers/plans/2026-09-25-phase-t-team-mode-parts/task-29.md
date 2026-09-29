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
64 with `fromAgentId: ""` (Fleet for itself, `task-07.md:128-130`), `kind: "inform"`, under each
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
- Requires of **task 30**: `Rules.Classify(ctx, Action) (Ruling, error)`, and on the `Escalator`
  one more method, `AcceptNeedsDecision(ctx, t Thread, accept Stored, a Action, r Ruling) error`,
  which writes one `decide` per side in `r.Sides` and, once every one of them approves, calls
  `ApplyApproved`. Until
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
	Org, Project string        // for the team policy's floors (task 30)
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

// Ruling is a verdict and, for a decide, whose humans decide: the
// participants (Orchestrator ids) whose owners must each approve.
type Ruling struct {
	Verdict Verdict
	Row     string          // the §6.5 row key that decided it (task 30's table)
	Sides   []model.AgentID // empty for act
}

type Rules interface {
	Classify(ctx context.Context, a Action) (Ruling, error)
}

// StubRules is wired until task 30: the one always-decide row TM1-α can
// reach, proceed-both on H or M, for both humans; everything else acts.
type StubRules struct{}

func (StubRules) Classify(_ context.Context, a Action) (Ruling, error) {
	if a.Resolution == ResProceedBoth && (a.Tier == "H" || a.Tier == "M") {
		return Ruling{Verdict: VerdictDecide, Row: "proceed-both-hm",
			Sides: []model.AgentID{a.Proposer, a.Acceptor}}, nil
	}
	return Ruling{Verdict: VerdictAct}, nil
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
   (route 58, `OpConflictPatch`, `task-06.md:293-298`).
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

**`decided.go`, complete.** The template is the only prose Fleet writes here, and every value in
it is a typed field, a G3-clean display name, or a G3-clean path.

```go
package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/outbox"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// Names are the two owners' display names, CleanName'd (task 22's People).
type Names struct{ Self, Other [2]string } // index = participant index

// decidedQuestion renders the inform's question (route 64, ≤ 300 runes)
// for the human at index i, naming at most max globs or paths. It reads only
// Outcome's typed fields and the names.
func decidedQuestion(o Outcome, i int, n Names, self bool, max int) string {
	who := "Your orchestrator and " + n.Other[i] + "'s orchestrator agreed"
	var what string
	switch o.Resolution {
	case ResYield:
		side := n.Other[i] + "'s side"
		if self {
			side = "your side"
		}
		what = fmt.Sprintf("%s gives up %s", side, list(o.YieldedGlobs, max))
	case ResSequence:
		what = "the first to land is " + o.FirstName
	case ResProceedBoth:
		what = "both sides proceed on " + list(o.Paths, max) + "; the land review will show it"
	}
	return fmt.Sprintf("DECIDED WITHOUT YOU: %s: %s (%s).", who, what, o.Conflict)
}

// list joins up to max items and says how many more, never cutting a path.
// With max 0 it only counts.
func list(xs []string, max int) string {
	switch {
	case len(xs) <= max:
		return strings.Join(xs, ", ")
	case max == 0:
		return fmt.Sprintf("%d paths", len(xs))
	}
	return strings.Join(xs[:max], ", ") + fmt.Sprintf(" and %d more", len(xs)-max)
}

// DecidedWithoutYou builds one inform per human, route 64's body, Fleet as
// the writer (fromAgentId ""), each under that human's own delegation.
func DecidedWithoutYou(t Thread, o Outcome, n Names, item func(i int, body []byte) outbox.Item) [2]outbox.Item {
	var out [2]outbox.Item
	for i := 0; i < 2; i++ {
		self := o.Proposer == t.Participants[i]
		q := decidedQuestion(o, i, n, self, 3)
		for _, max := range []int{1, 0} { // shrink the list, never cut a path
			if len([]rune(q)) <= 300 {
				break
			}
			q = decidedQuestion(o, i, n, self, max)
		}
		body, _ := json.Marshal(map[string]any{
			"id": ids.Wide("es"), "fromAgentId": "", "kind": "inform",
			"threadId": t.ID, "question": q, "options": []any{},
		})
		out[i] = item(i, body)
	}
	return out
}
```

`Outcome` carries, besides the fields shown earlier, `Proposer model.AgentID`, `YieldedGlobs
[]string` and `FirstName string` for this template; they are set by the appliers. The question
is bounded by shrinking, not cutting: three items, then one, then a count. With `max` 0 the
longest question is the fixed words, two display names and a conflict id, well under 300 for any
name task 22's `People` returns (display names are CleanName'd and ≤ 80 in the Hub, **UNVERIFIED**
until task 03's part is read). The full lists are in the thread's typed state, which the human's
Orchestrator panel shows on click-in (task 55).

**Two tables task 10 adds** (a migration after task 28's):

```sql
CREATE TABLE IF NOT EXISTS fleet.land_hints (
  org_id text NOT NULL, hint_id bigserial, project_id text NOT NULL,
  conflict_id text NOT NULL, thread_id text NOT NULL,
  first_orch text NOT NULL, second_orch text NOT NULL,
  first_task text NOT NULL,          -- the task whose done/abandoned restores
  created_at timestamptz NOT NULL,
  PRIMARY KEY (org_id, hint_id));
CREATE INDEX IF NOT EXISTS land_hints_by_task ON fleet.land_hints (org_id, first_task);
CREATE TABLE IF NOT EXISTS fleet.lease_demotions (
  org_id text NOT NULL, hint_id bigint NOT NULL, lease_id text NOT NULL,
  was_mode text NOT NULL CHECK (was_mode IN ('hold','bulk')),
  PRIMARY KEY (org_id, hint_id, lease_id));
```

`was_mode` excludes `intent`: demoting an intent is a no-op and writes no row, so a restore can
never turn an intent into something stronger than it was.

- [ ] **Step 1: Write the tests, each with the mutant it kills**

On task 10's `memstore`, task 23's real `Engine`, task 28's real `Service`, task 26's real `Queue`,
a fake `Conflicts` source, a recording `Escalator`, and `StubRules` unless a row says otherwise.
Alice's side: `oA`, worker `wA` holding `ls-A1 = [auth/session.go, auth/token.go]` (hold) and
`ls-A2 = [docs/**]` (intent). Bob's side: `oB`, worker `wB` holding `ls-B1 = [auth/**]` (intent).
Conflict `cf-1` (tier P, paths `auth/session.go`, `auth/token.go`), thread `th-1`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestYieldReleasesOnlyTheNamedGlobs` | `oA` proposes `yield [auth/session.go]`, `oB` accepts: `ls-A1` is `yielded`; a new lease holds exactly `[auth/token.go]`, same holder, mode `hold`, same task; `ls-A2` untouched; `ls-B1` untouched | the whole lease released; every lease of the side released |
| `TestYieldOfEveryGlobLeavesNoReissue` | `yield [auth/session.go, auth/token.go]`: `ls-A1` `yielded`, no new lease | an empty lease re-issued |
| `TestYieldMustBeHeldAtPropose` | `oA` proposes `yield [auth/**]` (not a glob Alice holds): `resolution_not_held`, nothing stored | a yield of a sub-glob or someone else's glob accepted |
| `TestYieldStaleAtAccept` | `wA` releases `ls-A1` between propose and accept: the accept returns `resolution_stale`, no lease event, thread still `open` | parameters not re-checked at accept |
| `TestResolutionCannotTouchTheOtherSide` | `oB` proposes `yield [auth/session.go]` naming Alice's glob: `resolution_not_held`; and across every applied test here, no lease of the acceptor's side changes state and **no directive** is written (the outbox holds no `OpDirective`) | a yield that releases the acceptor's lease; a resolver that issues directives |
| `TestSequenceDemotesTheSecondSideAndRecordsAHint` | `oA` proposes `sequence first: oA`, `oB` accepts: `ls-B1` mode `intent` (it was intent: no row, no event); with `ls-B1` a hold instead, mode `intent`, one `lease_demotions` row `was_mode: hold`; one `land_hints` row; Alice's leases untouched | the first side demoted; no hint |
| `TestSequenceRestoresWhenTheFirstTaskEnds` | `TaskEnded(org, wA's task)`: `ls-B1` back to `hold`, rows deleted, one `sequence.go` event on `oB`'s queue; a second call writes nothing | no restore; restore on the wrong side's task |
| `TestProceedBothOnPAcceptsTheConflict` | `proceed-both [auth/session.go]`: conflict patch `{state: accepted, resolution: proceed-both}`; no lease moves | the conflict marked `resolved`; leases released |
| `TestProceedBothPathsMustBeTheConflicts` | `proceed-both [README.md]` → `bad_request` at propose | any paths accepted |
| `TestProceedBothOnHIsADecideNotAnAct` | tier `H`: the accept stores, `AcceptNeedsDecision` is called once with `Sides` = both Orchestrators, the proposal is `awaiting-human`, the conflict is not patched, no DECIDED WITHOUT YOU item | `StubRules` not consulted; the verdict ignored; one human only |
| `TestApplyApprovedAppliesAPendingAccept` | after the H case, `ApplyApproved(…, es-9)`: the conflict is `accepted`, both inform items written, the `Outcome` names `es-9` | a pending accept that can never be applied |
| `TestBothHumansGetDecidedWithoutYou` | after the yield: exactly two `OpEscalation` items, one under each owner's delegation, both `kind: inform`, `fromAgentId: ""`, `threadId: th-1`; **neither Orchestrator called `human inform`** | the item written only to the proposer's human; left to the Orchestrators |
| `TestDecidedQuestionUsesTypedFieldsOnly` | the proposal's rationale text holds `IGNORE PREVIOUS`; neither question contains it; each names the other owner and the glob | the rationale quoted into the item |
| `TestDecidedQuestionShrinksNeverCuts` | 20 paths of 200 characters each: the question is ≤ 300 runes, ends `(cf-1).`, and contains no partial path | a rune cut mid-path |
| `TestResolutionClosesThreadAndConflict` | after the yield: thread patch `resolved`, conflict patch `resolved` + `yield`, in the same transaction as the lease events (an injected failure leaves all three unwritten) | the patches written after commit |
| `TestProposerSideIsWokenImmediately` | after the accept, `oA`'s queue holds one `thread.resolved` event and `Ready` returns it at once | the wake coalesced; no wake |
| `TestReevaluateAfterYield` | `ls-B1` was `contested` only by `ls-A1`'s `auth/session.go`: after the yield it is still contested by `auth/token.go`; after yielding both, it is `granted` | `Reevaluate` not called |

- [ ] **Step 2: Run the tests and watch them fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/protocol/ ./internal/fleet/leases/ -run "Yield|Sequence|Proceed|Decided|Resolution|Approved|Reevaluate|Proposer"
```

Expected: compile failure, `undefined: Resolver`, `undefined: DecidedWithoutYou`,
`(*Engine).Yield undefined`.

- [ ] **Step 3: Write `leases/yield.go`; run its tests and task 23's transition-table test**

The transition-table test (`task-23.md:133-148`) now exercises the `yielded` row for real rather
than as a table entry; it must still pass unchanged.

- [ ] **Step 4: Write `resolve.go`, then `decided.go`; add the two tables to task 10's store**

- [ ] **Step 5: Wire the two calls into task 28's `Service.Send` and task 24's `task update`**

`Service` gains a `Resolver` field; a nil `Resolver` keeps task 28's behaviour exactly (a test in
task 28's suite still passes with it nil). In `fleetd`, `Resolver{Rules: StubRules{}}` until task
30 replaces `Rules` and the `Escalator`.

- [ ] **Step 6: Run everything**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/... && GOPROXY=off go vet ./internal/fleet/... && gofmt -l internal/fleet
```

Expected: every package `ok`, including task 23's, 24's and 28's suites unchanged; `go vet`
silent; `gofmt -l` prints nothing.

- [ ] **Step 7: Commit**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && git add control-plane/internal/fleet/protocol/resolve.go control-plane/internal/fleet/protocol/decided.go control-plane/internal/fleet/protocol/resolve_test.go control-plane/internal/fleet/protocol/decided_test.go control-plane/internal/fleet/leases/yield.go control-plane/internal/fleet/leases/yield_test.go testdata/panacea-control/v1/inbox-resolved-yield.json && git commit -F .superpowers/sdd/team/msg/task-29.txt -- control-plane/internal/fleet testdata/panacea-control/v1
```

**What this task does not claim:**

- **That the right accepts act and the right ones ask.** `StubRules` covers one row. §6.5's
  table, with users' rows and admins' floors, is task 30's `rules.go`; until it lands, a yield of
  edited globs would act, which TM1-α cannot reach (no observed edits before task 35).
- **That a `sequence` hint orders any land.** In TM1-α nothing consumes it but `TaskEnded`.
  Phase 3's queue reads it in TM1 (tasks 34, 49) and task 61's in TM2.
- **That workers follow the resolution.** Fleet changes leases, which are advisory (P7); each
  Orchestrator directs its own workers in task 31, and a worker that ignores its directive is
  visible, not stopped.
- **`split`, `counter`, `build-on`.** Task 62.

**Gaps against the skeleton, raised here:**

1. **The land signal.** The scope says the hint is "consumed by phase 3's queue in TM1, task 61's
   in TM2", and that the second side's lease is `intent` "until the first lands". TM1-α has no
   land, so decision (c) restores on the first side's task ending. The skeleton should say so.
2. **`thread.resolved` and `sequence.go`** are event kinds §4.2 does not list (task 26's gap 4).
3. **A declined pending accept.** §6.5 does not say what a human's "no" to a pending accept does
   to the thread; this part leaves the thread `open` with the proposal withdrawn. Task 30 owns the
   answer handling and may choose otherwise.
