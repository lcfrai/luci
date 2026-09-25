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
