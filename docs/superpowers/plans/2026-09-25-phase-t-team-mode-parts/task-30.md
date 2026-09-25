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
