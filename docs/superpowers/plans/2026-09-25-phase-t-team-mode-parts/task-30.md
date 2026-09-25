### Task 30: Escalations: inform and decide, the §6.5 rows, Fleet-set default and deadline

AGENTIC-UX asks for authority *"legislated in advance, not vetoed in real time"*
(`docs/AGENTIC-UX.md:37-38`, via `05-design.md` §6.5). This task is the legislation and its
enforcement. It writes every `es-…` record, `inform` or `decide`, to exactly one human: the
Orchestrator's own, or, when Fleet writes for itself, each human a negotiation concerns. It holds
§6.5's table **as data**, keyed on a typed action, and applies it at the verb, so an Orchestrator
cannot skip a row by not asking. And it is the only place a decide's **default and deadline** are
chosen, by Fleet, whatever the Orchestrator asked for (`07-critique-security.md` I8).

Five properties, each pinned by a test.

1. **Fleet sets every decide's default and deadline.** The default is always the safe option,
   and it is an option **Fleet writes**, not one of the Orchestrator's; the deadline is at least
   30 minutes (team policy may lengthen it). An Orchestrator that asks for *"default: continue,
   deadline: 60 s"* gets Fleet's hold option as the default and a 30-minute deadline.
2. **The rows are data, and they only tighten.** A team admin's floors (task 01 §7.24,
   `escalationFloors`, `task-01.md:471-480`) may turn an *act* row into a *decide*; nothing may
   turn a *decide* into an *act*. The always-decide rows (`build-on`, `proceed-both` on H or M,
   free text in a Fleet-started turn, rounds or deadline exhausted) are marked locked in the
   table, and a floor or user row naming them is refused (OD-24).
3. **An accept on a decide row is the escalation.** Task 29's `Resolve` asks `Rules.Classify`
   before applying anything; a `decide` there writes the decide here and applies nothing until
   every deciding human approves (task 29's `ApplyApproved`).
4. **Exhausted negotiation reaches both humans from Fleet.** On rounds, stale rounds or the
   deadline (task 28's `ThreadExhausted`), Fleet writes a decide to **each** human with the
   thread's typed state, so neither depends on the other side's agent-written summary.
5. **An unanswered decide pauses exactly the contested worker, and deletes nothing.** At the
   deadline the safe default applies: task 23's `Contested` names one worker, which gets a
   `pause`; no lease is released, no lane touched, no work reverted. *"Pausing costs time, never
   work"* (§6.5).

Four decisions the scope left open, made here.

**(a) Fleet's hold option is option 0 of every decide.** `human ask` carries the Orchestrator's
own options (at most 3, each a label ≤ 80 and a consequence ≤ 200, `text·A`); Fleet prepends its
own option 0, **"Hold: nothing changes until you choose"**, and makes it the default. For a
thread decide Fleet writes every option itself (below). So "the safe default" never depends on
Fleet judging an agent's words, which it must not do (OD-8's spirit, §6.1).

**(b) The deciding humans are named by the row.** §6.5's rows are written from one human's view
("yield globs *my* worker has edited"). Each row names whose humans decide: the side whose work
the action would discard or delay, or both for the two always-both rows (`proceed-both` on H or
M, exhaustion). `Ruling.Sides` (task 29) carries them.

**(c) A thread decide's options each map to a typed effect Fleet can apply alone.** An option
that changes only the chooser's own side applies as soon as that human chooses it (`my side
yields`, `my side goes second`). An option that changes both sides (`both proceed`) applies only
when both humans choose it. Option 0 (hold) is always the default. When one human's choice
resolves the conflict, the other's decide is answered by Fleet's inform (*"resolved by
<name>'s choice"*); the Hub keeps it unanswered, and a later answer to it is recorded and ignored.

**(d) User rows have no Hub home yet.** §6.5 says each user sets, per action class, what their
Orchestrator may agree to; neither §3.3 nor task 01 has a record for it. This task reads user
rows through a `RulesSource` whose TM1-α implementation returns none, so every user gets the
defaults plus the team's floors. The standing permission for free-text answers is one user row
(`free-text-answers`, default off) and is therefore off for everyone until the record exists.
Recorded as a gap.

**Files:**
- `control-plane/internal/fleet/escalate/rules.go` — the §6.5 table as data, `Rules` (task 29's
  interface), floors and user rows, the lock (new)
- `control-plane/internal/fleet/escalate/escalate.go` — `Service`: `Inform`, `Ask`, `Decide`,
  and task 28's and 29's `Escalator` and `Permissions` methods; `OnAnswered`; `SweepDeadlines`
  (new)
- `control-plane/internal/fleet/escalate/options.go` — a thread decide's options and their
  typed effects; the contested worker (new)
- `control-plane/internal/fleet/verbs/human.go` — `human inform` and `human ask` (new)
- `control-plane/internal/fleet/escalate/{rules,escalate,options}_test.go`,
  `verbs/human_test.go` (new)
- `testdata/panacea-control/v1/human-inform.json`, `human-ask.json`, `human-ask-result.json`
  (new, in task 09's set)

`options.go` is not in the file map; it keeps the one piece with real branching, "what does each
choice do", out of the service.

**Interfaces:**
- Produces:
  - `escalate.Rules`, satisfying task 29's `protocol.Rules` (`Classify(ctx, Action) (Ruling,
    error)`), replacing `StubRules` in `fleetd`
  - `escalate.Service`, satisfying task 28's `protocol.Escalator` (`InformOwnHuman`,
    `ThreadExhausted`), task 29's `AcceptNeedsDecision`, and task 28's `protocol.Permissions`
    (`FreeTextAnswers`)
  - `escalate.Service.Decide(ctx, DecideRequest) (string, error)` — the one way any task writes
    a decide; task 31's `direct` calls it for the taint rule's decides
  - `escalate.Row` keys, the table's vocabulary, used by task 52's read model and task 67's admin
    view
- Consumes:
  - task 28: `protocol.Thread`, `Stored`, `Exhaustion`, `Service.Thread`; task 29:
    `protocol.Action`, `Ruling`, `Verdict`, `Resolver.ApplyApproved`, `Engine.Yield`/`Demote`
    through the resolver's appliers (an option's effect is applied as a resolution)
  - task 23: `leases.Contested`, `leases.RightOfWay`, `leases.Side`, `Engine.Live`
  - task 26: `turns.Queue.Enqueue` with `escalation.answered`; `turns.Scheduler.CurrentTurn`
    (a `human ask`'s origin); `RecordIssued` for each `es-…`
  - task 11: `outbox.Enqueue`, `OpEscalation`; task 07's route 64 body (`task-07.md:127-142`)
    and route 65's answer shape (`:144-151`), read from task 08's feed kind
    `escalation.answered` (**UNVERIFIED** that task 08 names it so; the feed row is task 01's
    event of that name)
  - task 01 §7.24's team policy: `escalationFloors`, `messageBudgets.threadDeadlineMinutes`
  - task 02: `teamtext.New` with `KindQuestion` (300), `KindOptionLabel` (80),
    `KindConsequence` (200)
- Requires of **task 31**: `Pauser.PauseForDefault(ctx, worker model.AgentID, es string) error`,
  a `pause` directive written by Fleet from the worker's own Orchestrator (route 67 requires
  `fromAgentId` be the worker's parent), `origin: "fleet"`, `approvedBy: es`. Until task 31
  lands, `fleetd` wires a `Pauser` that records the pause in `fleet.default_pauses` and task 31's
  piggyback delivers it; the tests here use a recording fake.
- Requires of **task 10**: `fleet.open_escalations` (below).

**The ST.1 shapes of `human inform` and `human ask`, complete.**

```go
package escalate

import "time"

// InformArgs is `panacea human inform <text>`. To the caller's own human.
type InformArgs struct {
	Text   string `json:"text"`             // ≤ 300 (route 64's question), text·A
	Thread string `json:"thread,omitempty"` // th-… the caller is party to, or ""
}

// AskArgs is `panacea human ask`. Default and Deadline are accepted so an
// Orchestrator's request decodes, and then IGNORED: Fleet sets both.
type AskArgs struct {
	Question string      `json:"question"` // ≤ 300
	Options  []AskOption `json:"options"`  // 1..3; Fleet prepends option 0
	Thread   string      `json:"thread,omitempty"`
	Default  *int        `json:"default,omitempty"`  // ignored, recorded in the audit line
	Deadline string      `json:"deadline,omitempty"` // ignored, recorded in the audit line
}

type AskOption struct {
	Label       string `json:"label"`       // ≤ 80
	Consequence string `json:"consequence"` // ≤ 200
}

// AskResult tells the Orchestrator what Fleet actually wrote.
type AskResult struct {
	Escalation string    `json:"escalation"` // es-…
	Default    int       `json:"default"`    // always 0
	Deadline   time.Time `json:"deadline"`   // ≥ now + 30 min
	Overridden []string  `json:"overridden,omitempty"` // "default", "deadline"
}

// HoldLabel and HoldConsequence are option 0 of every Orchestrator ask.
const (
	HoldLabel       = "Hold: nothing changes until you choose"
	HoldConsequence = "Your orchestrator waits; no worker is paused and nothing is undone."
)
```

`Overridden` tells the Orchestrator plainly that its default or deadline was not used, so its
next turn's reasoning starts from the truth. Accepting the two fields and ignoring them, rather
than refusing them, is deliberate: a refusal teaches a prompt-injected Orchestrator which fields
matter; an override is recorded in the audit line (`requestedDefault`, `requestedDeadline`) where
an auditor sees the attempt.

**`rules.go`, the table, complete.** One row per §6.5 line that is keyed on a typed action. The
row that is not (*"a peer asks why"*) is task 28's templated answer, not a verdict.

```go
package escalate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/protocol"
)

type Who int // whose human decides, relative to the Action

const (
	Proposer Who = iota
	Acceptor
	Second   // sequence: the side that waits
	Builder  // build-on: the side that builds on the other's lane
	Both
	Own      // an Orchestrator's own human (direct, scope change)
)

type Row struct {
	Key     string
	Verdict protocol.Verdict
	Who     Who
	Locked  bool // always-decide: no source may change it (OD-24)
	Why     string
}

// Table is §6.5 (05-design.md:722-735). Order is irrelevant; keys are unique.
var Table = []Row{
	{"yield-unedited", protocol.VerdictAct, Proposer, false, "nothing is lost"},
	{"yield-edited", protocol.VerdictDecide, Proposer, false, "work would be discarded"},
	{"accept-split", protocol.VerdictAct, Acceptor, false, "my side keeps every edited path"},
	{"accept-split-moves-edited", protocol.VerdictDecide, Acceptor, false, "a split can move my edited work"},
	{"proceed-both-ps", protocol.VerdictAct, Both, false, "the land will show it"},
	{"proceed-both-hm", protocol.VerdictDecide, Both, true, "a clean merge can hide a semantic conflict"},
	{"sequence-short", protocol.VerdictAct, Second, false, "a wait under an hour"},
	{"sequence-long", protocol.VerdictDecide, Second, false, "a long wait is a scheduling choice"},
	{"build-on", protocol.VerdictDecide, Builder, true, "it runs their code in my sandbox"},
	{"scope-change", protocol.VerdictDecide, Own, false, "it changes what I asked for"},
	{"exhausted", protocol.VerdictDecide, Both, true, "negotiation failed"},
	{"free-directive-fleet-turn", protocol.VerdictDecide, Own, true, "the injection fence"},
}

// Floorable are the four act rows a team admin may floor (task-01.md:471-480).
var Floorable = map[string]bool{
	"yield-unedited": true, "accept-split": true, "proceed-both-ps": true, "sequence-short": true,
}

var ErrLoosen = errors.New("escalate: a rule may only add asking")

// Sources are the floors (team policy) and each user's own rows.
type Sources interface {
	Floors(ctx context.Context, org, project string) ([]string, error)
	UserRows(ctx context.Context, owner model.Owner) ([]string, error) // none in TM1-α
	OwnerOf(ctx context.Context, orch model.AgentID) (model.Owner, error)
}

type Rules struct{ Src Sources }

// KeyOf maps a typed action to its row. Unknown forecast wait counts as
// short: TM1-α has no forecast (decision recorded under the gaps).
func KeyOf(a protocol.Action) string {
	switch a.Resolution {
	case protocol.ResYield:
		if len(a.EditedByProposer) > 0 {
			return "yield-edited"
		}
		return "yield-unedited"
	case protocol.ResSequence:
		if a.ForecastWait >= time.Hour {
			return "sequence-long"
		}
		return "sequence-short"
	case protocol.ResProceedBoth:
		if a.Tier == "H" || a.Tier == "M" {
			return "proceed-both-hm"
		}
		return "proceed-both-ps"
	case protocol.ResBuildOn:
		return "build-on"
	case protocol.ResSplit:
		return "accept-split" // task 62 adds the moves-edited test
	}
	return ""
}

// Classify is task 29's Rules. Order: the row; a locked row is final; a
// team floor or a deciding side's own user row turns act into decide.
// Nothing turns decide into act: there is no code path that could.
func (r Rules) Classify(ctx context.Context, a protocol.Action) (protocol.Ruling, error) {
	key := KeyOf(a)
	row, ok := rowByKey[key]
	if !ok {
		return protocol.Ruling{}, fmt.Errorf("escalate: no row for %q", a.Resolution)
	}
	sides := sidesFor(row.Who, a)
	if row.Verdict == protocol.VerdictDecide {
		return protocol.Ruling{Verdict: protocol.VerdictDecide, Row: key, Sides: sides}, nil
	}
	floors, err := r.Src.Floors(ctx, a.Org, a.Project)
	if err != nil {
		return decideOnError(key, sides), nil // cannot read the floors: ask
	}
	if contains(floors, key) {
		return protocol.Ruling{Verdict: protocol.VerdictDecide, Row: key, Sides: sides}, nil
	}
	var asking []model.AgentID
	for _, s := range sides {
		owner, err := r.Src.OwnerOf(ctx, s)
		if err != nil {
			return decideOnError(key, sides), nil
		}
		rows, err := r.Src.UserRows(ctx, owner)
		if err != nil {
			return decideOnError(key, sides), nil
		}
		if contains(rows, key) {
			asking = append(asking, s) // a user tightens for their own side only
		}
	}
	if len(asking) > 0 {
		return protocol.Ruling{Verdict: protocol.VerdictDecide, Row: key, Sides: asking}, nil
	}
	return protocol.Ruling{Verdict: protocol.VerdictAct, Row: key}, nil
}

// decideOnError: a rule source that cannot be read never loosens; it asks.
func decideOnError(key string, sides []model.AgentID) protocol.Ruling {
	return protocol.Ruling{Verdict: protocol.VerdictDecide, Row: key, Sides: sides}
}
```

`rowByKey` is built from `Table` at `init`, which panics on a duplicate key (a programming error
caught by the first test run, never at run time in a released build). `sidesFor` maps `Who` to
Orchestrator ids: `Proposer` → `a.Proposer`; `Acceptor` → `a.Acceptor`; `Second` → the
participant that is not `a.First`; `Builder` → the participant that is not the base; `Both` →
both; `Own` → the caller (the directive and scope rows, where `Action.Proposer` is set to the
calling Orchestrator by task 31). `contains` is a linear scan.

**Validating a floor list** (the Hub already refuses non-floorable keys, task 01 §7.24; Fleet
checks again because it must not trust a stale cache): `ValidateFloors(keys []string) error`
returns `ErrLoosen` naming the first key that is not in `Floorable`, including every locked and
every decide row (flooring a decide row is meaningless, and accepting it would hide a typo). A bad
cached floor list is treated as "every floorable row floored" until the next policy read, which
only adds asking.

**`escalate.go`, the surface, complete.**

```go
package escalate

import (
	"context"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/protocol"
)

const (
	MinDecideDeadline = 30 * time.Minute // §6.5; the Hub refuses less (task-07.md:140-142)
	MaxDecideDeadline = 24 * time.Hour
	MaxAskOptions     = 3                // + Fleet's option 0 = route 64's maximum of 4
)

// Effect is what choosing an option does, typed. Fleet applies it; the
// option's label only describes it.
type Effect struct {
	Kind  string   // "hold" | "apply-proposal" | "decline-proposal" | "yield-mine" |
	               // "go-second" | "proceed-both" | "none"
	Globs []string // yield-mine
	Both  bool     // true: applies only when every decider chose it
}

// DecideRequest is the one way to write a decide. Owner is the human asked;
// From is the Orchestrator asking, or "" when Fleet writes for itself.
type DecideRequest struct {
	Org, Project string
	Owner        model.Owner
	From         model.AgentID
	Thread       string
	Conflict     string
	Question     string   // ≤ 300; Fleet's template, or an Orchestrator's text·A
	Options      []Option // option 0 is always the hold/default Fleet writes
	Group        string   // decides that must agree share a group id (Both effects)
	Row          string   // the §6.5 row key, for the audit and the cockpit
}

type Option struct {
	Label, Consequence string
	Effect             Effect
}

type Service struct {
	Store    Store
	Rules    Rules
	Resolver *protocol.Resolver
	Queue    Enqueuer // task 26's Queue
	Pauser   Pauser   // task 31
	Outbox   func(org string, owner model.Owner, body []byte) outboxItem
	Policy   func(ctx context.Context, org, project string) (deadline time.Duration)
	Now      func() time.Time
}

type Pauser interface {
	PauseForDefault(ctx context.Context, worker model.AgentID, es string) error
}

// Inform writes an inform to the caller's own human. No options, no default.
func (s *Service) Inform(ctx context.Context, c Caller, a InformArgs) (string, error)

// Ask writes a decide from the caller to its own human: option 0 Fleet's
// hold, then the caller's options; default 0; deadline Fleet's.
func (s *Service) Ask(ctx context.Context, c Caller, a AskArgs) (AskResult, error)

// Decide is the shared writer: bounds, G3, default 0, deadline, the
// open-escalation row and the outbox item, in one transaction.
func (s *Service) Decide(ctx context.Context, r DecideRequest) (string, error)

// The four methods tasks 28 and 29 call through their interfaces.
func (s *Service) FreeTextAnswers(ctx context.Context, owner model.Owner) bool
func (s *Service) InformOwnHuman(ctx context.Context, orch model.AgentID, msg string) error
func (s *Service) ThreadExhausted(ctx context.Context, org, thread string, why protocol.Exhaustion) error
func (s *Service) AcceptNeedsDecision(ctx context.Context, t protocol.Thread, accept protocol.Stored,
	a protocol.Action, r protocol.Ruling) error

// OnAnswered is task 11's feed consumer's callback for escalation.answered.
func (s *Service) OnAnswered(ctx context.Context, org, es string, choice *int, at time.Time) error

// SweepDeadlines applies the safe default to every open decide past its
// deadline. Task 12's deadline-sweeper singleton runs it every 5 s.
func (s *Service) SweepDeadlines(ctx context.Context) error
```

`Caller` is task 19's `control.Caller`; `Store`, `Enqueuer` and `outboxItem` are narrow local
interfaces over task 10's store, task 26's `Queue` and task 11's `outbox.Item`, each a few lines.
`FreeTextAnswers` is `contains(UserRows(owner), "free-text-answers")` minus a team floor of the
same name if present, so it is false for everyone in TM1-α (decision d).

**The deadline, one function.** `deadline = now + clamp(policy.threadDeadlineMinutes, 30 min,
24 h)`, where an absent policy is 30 minutes. An Orchestrator's requested deadline is never read.
A decide written for a thread uses the thread's **effective** deadline plus 30 minutes, so the
humans always have at least 30 minutes after negotiation stops.

**`options.go`: what each decide offers, and what each choice does.** Every label below is
product text; `<other>` is the other owner's CleanName'd display name, `<paths>` task 29's
shrinking `list`. The question is Fleet's template from typed state, never a message's text.

| Decide | Asked | Option 0 (default) | Option 1 | Option 2 | Option 3 |
|---|---|---|---|---|---|
| **Accept on a decide row** (`AcceptNeedsDecision`) | each side in `Ruling.Sides` | *Hold: the agreement waits* · `hold` | *Apply what the orchestrators agreed* · `apply-proposal`, `Both` when `len(Sides) == 2` | *Do not apply it* · `decline-proposal` | — |
| **Exhausted** (`ThreadExhausted`) | both | *Hold: nothing changes; at the deadline the contested worker pauses* · `hold` | *My side gives up <paths>* · `yield-mine` (the chooser's own globs on the conflict's paths) | *My side goes second* · `go-second` | *Both proceed; the land review decides* · `proceed-both`, `Both` (not offered on tier H or M, where it is exactly the decision that failed) |
| **Orchestrator ask** (`Ask`) | the caller's own | Fleet's *Hold* · `hold` | the caller's options, each `none` | … | … |

A choice applies as follows, in `OnAnswered`, each in one transaction with its outbox items:

- `hold`: nothing. The decide stays open until its deadline.
- `apply-proposal`: record the approval; when every decide of the `Group` has it, call task 29's
  `Resolver.ApplyApproved` with the approving `es-…` ids.
- `decline-proposal`: task 28's `withdraw` of the pending proposal, written by Fleet; the thread
  stays `open`; the other decide of the group is answered by an inform.
- `yield-mine`, `go-second`: applied at once as a resolution on the chooser's own side only
  (task 29's appliers with the chooser as proposer); the conflict closes; the other human's
  decide gets an inform, *"Resolved by <name>'s choice"*.
- `proceed-both`: applied when both decides of the group chose it.
- `none` (an Orchestrator's own option): nothing but the `escalation.answered` event.

Every answer enqueues `escalation.answered` (task 26) for the asking Orchestrator, or for each
participant's Orchestrator when Fleet asked, so the next turn sees the answer through `panacea
inbox`'s escalation list. An answer after the deadline (task 07 accepts it, `task-07.md:148-150`)
is applied the same way when its effect is still possible, and otherwise recorded with an inform
saying what already happened.

**The contested worker, one function.** For a conflict with paths `P` between workers `a` and `b`:
`Contested(a, b, p, lastEdit)` (task 23, `task-23.md:225-232`) for every `p` in `P`; the worker named
for more paths is the contested one; a tie goes to the side **without** right of way on the whole
conflict (`RightOfWay(a, b)`). One worker, always. `lastEdit` is nil in TM1-α, so the answer is
the side without right of way, which is task 23's rule for "neither has edited".

**`SweepDeadlines`**: for each open decide with `deadline <= now` and no answer: `hold` is the
default, so the safe default is applied **once** per group, not per decide: for a thread decide,
`Pauser.PauseForDefault(contested, es)`; for an Orchestrator's own ask, nothing; then an inform to
each asked human, *"The deadline passed. <worker> is paused on <paths>; nothing was undone."*, and
the group is marked `defaulted`. A later answer can still resume or resolve (above).

**The table task 10 adds:**

```sql
CREATE TABLE IF NOT EXISTS fleet.open_escalations (
  org_id text NOT NULL, es_id text NOT NULL, grp text NOT NULL,
  owner_tid text NOT NULL, owner_oid text NOT NULL, from_agent text NOT NULL DEFAULT '',
  thread_id text NOT NULL DEFAULT '', conflict_id text NOT NULL DEFAULT '',
  proposal_id text NOT NULL DEFAULT '', row_key text NOT NULL,
  effects jsonb NOT NULL,             -- option index -> Effect
  deadline timestamptz NOT NULL,
  state text NOT NULL CHECK (state IN ('open','approved','declined','applied','defaulted','closed')),
  PRIMARY KEY (org_id, es_id));
CREATE INDEX IF NOT EXISTS open_escalations_due ON fleet.open_escalations (deadline) WHERE state = 'open';
```

No question, label or answer text is stored here: those are the Hub's `es-…` record. This row is
the typed effect of each option, which is Fleet working state.

- [ ] **Step 1: Write the tests, each with the mutant it kills**

On task 10's `memstore`, tasks 23, 26, 28 and 29's real packages, a fake `Sources` (floors and
user rows set per test), a recording `Pauser`, and a fixed clock. The cast is task 29's: `oA`/`wA`
(Alice), `oB`/`wB` (Bob), conflict `cf-1`, thread `th-1`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestOrchestratorDefaultAndDeadlineAreOverridden` | `human ask` with `default: 1` and `deadline: now+60s`: the outbox body has `default: 0`, option 0 is `HoldLabel`, `deadline` = now + 30 min; the result's `overridden` is `["default","deadline"]`; the audit line records both requested values | the requested default or deadline used |
| `TestAskHasAtMostFourOptions` | four caller options → `bad_request`; three → four written, Fleet's first | Fleet's option appended last, or a fifth option |
| `TestAskGoesToTheCallersOwnHumanOnly` | the outbox item is under Alice's delegation with `ownerUserId` hers; there is no field in `AskArgs` that names another owner (decode of `"owner"` fails) | an owner taken from args |
| `TestAskTextBounds` | question 301 runes → `text_refused`; a label with `ESC[` → refused; a consequence of 201 → refused | bounds not applied to options |
| `TestTableKeysUniqueAndLockedRowsDecide` | every key unique; every `Locked` row is `decide` | a locked `act` row |
| `TestFloorTurnsActIntoDecide` | floors `["yield-unedited"]`: a yield of unedited globs → decide, `Sides == [proposer]` | floors ignored |
| `TestNothingLoosens` | for every decide row, with floors = every key and user rows = every key: still decide; `ValidateFloors(["proceed-both-hm"])` and `(["yield-edited"])` → `ErrLoosen` | any code path from decide to act; a decide row accepted as a floor |
| `TestUserRowTightensOnlyTheirOwnSide` | Bob's user rows hold `sequence-short`: a sequence where Bob waits → decide, `Sides == [oB]`; one where Alice waits → act | a user row that makes a teammate's agent ask |
| `TestUnreadableRulesAsk` | `Floors` returns an error: an act row → decide | an error treated as "no floors" |
| `TestProceedBothOnHIsLockedDecideForBoth` | tier H: decide, both sides, whatever the floors and user rows | the H/M row as a floorable act |
| `TestAcceptOnDecideRowBecomesTheEscalation` | `oB` accepts `oA`'s proceed-both on tier H: two decides in one group, one per owner, `row: proceed-both-hm`; the conflict is untouched; `oA`'s and `oB`'s queues have no `thread.resolved` | the accept applied first and escalated after |
| `TestApplyOnlyWhenEveryDeciderApproves` | Alice chooses option 1: nothing applied; Bob chooses option 1: `ApplyApproved` called once with both `es-…` ids | the first approval applies |
| `TestDeclineWithdrawsTheProposal` | Alice chooses option 2: the proposal is withdrawn, the thread is `open`, Bob's decide gets an inform | a decline that closes the thread |
| `TestExhaustionWritesADecideToBothHumans` | `ThreadExhausted(rounds)`: two decides, each with four options (three on tier H: no `proceed-both`), each question built from typed state only (a rationale containing `IGNORE PREVIOUS` in the thread appears in neither) | one decide; a decide from the Orchestrator's summary |
| `TestOwnSideChoiceResolvesAtOnce` | after exhaustion, Bob chooses `go-second`: task 29's sequence is applied with Alice first; Alice's decide gets *"Resolved by Bob Nguyen's choice"* | `go-second` waiting for Alice |
| `TestExpiredDecidePausesExactlyTheContestedWorker` | no answer by the deadline: `PauseForDefault` called **once**, for the worker `Contested` names (`wB`, without right of way), with the group's first `es-…`; no lease released, no lease event, no conflict patch; both humans get the *"nothing was undone"* inform | both workers paused; a pause per decide; a lease released as the "default" |
| `TestExpiredOrchestratorAskPausesNothing` | an Orchestrator's own ask expires: no pause, one `escalation.answered` event with `defaulted` | the thread default applied to a free ask |
| `TestLateAnswerStillApplies` | Bob answers `go-second` after the default paused `wB`: the sequence applies and the inform says `wB` was already paused | a late answer dropped |
| `TestFreeTextAnswersOffInTM1Alpha` | `FreeTextAnswers` is false for every owner with the TM1-α `Sources` | a default of true |
| `TestInformHasNoOptions` | `human inform`: `kind: inform`, `options: []`, no `default`, no `deadline` in the body | an inform written as a decide |

- [ ] **Step 2: Run the tests and watch them fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/escalate/... ./internal/fleet/verbs/ -run "Ask|Inform|Floor|Loosen|Table|Decide|Exhaust|Expired|Late|FreeText|Accept|Apply|Decline|OwnSide|Unreadable|UserRow|ProceedBoth"
```

Expected: compile failure, `undefined: escalate.Service`, `undefined: Table`.

- [ ] **Step 3: Write `rules.go`; its tests pass with a fake `Sources` and no store**

- [ ] **Step 4: Write `options.go`, `escalate.go` and `verbs/human.go`; add the table to task
  10's store**

- [ ] **Step 5: Wire into `fleetd`**

Replace `StubRules` (task 29) with `escalate.Rules`; replace task 28's `DenyAll` and logging
`Escalator` with `escalate.Service`; register `human` with task 19's dispatcher for the
`orchestrator` class; run `SweepDeadlines` on the deadline-sweeper singleton; register
`OnAnswered` with task 11's feed consumer for `escalation.answered`.

- [ ] **Step 6: Run everything**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/... ./cmd/fleetd/... && GOPROXY=off go vet ./internal/fleet/... ./cmd/fleetd/... && gofmt -l internal/fleet cmd/fleetd
```

Expected: every package `ok`, tasks 28 and 29's suites unchanged (their fakes still pass, and
their tests that used `StubRules` still use it); `go vet` silent; `gofmt -l` prints nothing.

- [ ] **Step 7: Commit**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && git add control-plane/internal/fleet/escalate control-plane/internal/fleet/verbs/human.go control-plane/internal/fleet/verbs/human_test.go testdata/panacea-control/v1/human-*.json && git commit -F .superpowers/sdd/team/msg/task-30.txt -- control-plane/internal/fleet testdata/panacea-control/v1 control-plane/cmd/fleetd
```

**What this task does not claim:**

- **That the humans see anything.** The records are written; the Desk's WAITING and RETURN
  items are tasks 52 to 55, behind the owner-reviewed prototype (task 51).
- **That a paused worker stops.** `PauseForDefault` writes a directive; its delivery (piggyback,
  then the ladder) is tasks 31 and 38. A worker that ignores a pause is visible, not stopped (P7).
- **That the §6.5 defaults are right for every team.** They are the design's; floors let a team
  ask more; nothing lets it ask less.
- **The forecast wait for `sequence`.** Unknown in TM1-α, and counted as short (below).
- **That a real Hub accepts these bodies.** Route 64's shape is task 07's; delivery is task 33's.

**Gaps against the skeleton, raised here:**

1. **User rows have no home.** §6.5 has users set their own rows; no Hub record carries them
   (decision d). Until one exists, every user has the defaults plus the team's floors, and the
   free-text-answer permission is off for everyone. The Hub needs a per-member "my
   Orchestrator's rules" record (tighten-only keys plus `free-text-answers`), with its route and
   Z5 row; the desktop needs the setting (task 55's panel is the natural place).
2. **An unknown forecast wait counts as a short wait**, so in TM1-α every `sequence` acts and
   informs. The safer reading (unknown is long, so decide) would make the scripted conflict of
   task 33 wait for a human every time. Owner call.
3. **Answers after the default.** Task 07 accepts a late answer; this part applies it when still
   possible. §6.5 is silent.
4. **Thread decides and `peer` limits.** An exhausted thread is `escalated`, so task 28 refuses
   further messages on it; the humans' choices are applied by Fleet, not by reopening the thread.

