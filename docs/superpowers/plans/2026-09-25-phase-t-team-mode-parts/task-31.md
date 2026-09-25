### Task 31: Directives and the taint rule; piggyback delivery

A directive is how an Orchestrator steers **its own** workers: `pause`, `resume`, `narrow` to
globs, `wait-for` a lease, `rebase` onto upstream, or `free` text (§3.3's directive row,
`05-design.md:309`). This task builds the verb that issues one, the rule that decides whether it
may be issued, and rung 1 of delivery. Three properties carry the injection fence (§7.2):

1. **Authority follows turn origin, checked in Fleet's handler, not in a prompt.** Every
   Orchestrator on a team project is tainted from its first turn (§7.2 item 1). In a turn Fleet
   started, only the five typed kinds pass, and `narrow` only inside the task's scope. `free`, a
   new task, a wider scope, abandoning the task, a new worker, or `build-on` becomes a **decide
   written by the verb itself**, showing the human what the session read since they last spoke
   (§7.2 item 2). In a turn the human started, `free` passes and is shown in the panel as issued
   (§7.2 item 3).
2. **The Hub has it first.** Every directive is a `dv-…` Hub record (task 07's route 67) before
   any worker can see it. Delivery waits on the outbox row's `sent` state, in the same transaction
   that marks it sent.
3. **Piggyback is the only way directive text reaches a worker** (§4.3 rung 1,
   `05-design.md:453-457`). Every `panacea` call a worker makes returns its pending directives
   first, in the response, as tool output. No PTY write, no socket post and no file carries the
   text; rungs 2 and 3 (task 38) carry only a fixed template telling the worker to call `panacea`.

**Decisions made here.**

**(a) The five typed kinds pass in a Fleet-started turn, not three.** Design §7.2 item 2
(`05-design.md:807-809`), the skeleton's scope, task 01's contract §7.20 (`task-01.md:440`: "The
typed five (`pause`, `resume`, `narrow`, `wait-for`, `rebase`) pass") and task 03's
`DirectiveKind.Typed()` (`task-03.md:341-343`) all say five. Task 07's Hub copy of the rule,
`fleetTurnVerbs` (`task-07.md:205-209`), lists three and its table test expects `resume` and
`rebase` from a Fleet turn to be 403 (`task-07.md:292-293`). Fleet follows the design. **Raised as
gap G31-1**: until task 07 widens its set to `Typed()`, a Fleet-turn `resume` or `rebase` goes
dead in the outbox with 403 `forbidden`, and this task's acceptance step 4 fails loudly on it.

**(b) `narrow` never widens, in any turn.** A `narrow` whose globs are not each inside the
worker's task scope is refused `glob_refused` with `details.reason = "outside_task_scope"`,
whatever the origin (the skeleton's test: "a `narrow` outside scope is refused"). Widening is a
different verb, `task update` with a wider `scope` (task 24), and **that** is what the gate turns
into a `scope-change` decide in a Fleet turn. So "a wider scope" and "out-of-scope paths" in §7.2
reach the human through the verb that means them, and `narrow` keeps one meaning.

**(c) No current turn is a Fleet turn.** A `direct` that arrives while the caller has no claimed
turn (task 26's `CurrentTurn` returns `ok=false`) is judged as `fleet` origin: fail closed. The
one human-driven case outside a turn, DIVE (§7.2 item 3), is task 40's: it will register an
attached session as a `human` turn through the same `CurrentTurn`, and nothing here changes.

**(d) A held action is replayed, not remembered as prose.** When the gate turns a call into a
decide, Fleet keeps the **validated** verb and args in `fleet.held_actions`, keyed by the
decide's `es-…`, as Fleet working state (a fact about now, deleted when answered or expired;
Global constraint 2). Option 0, the default, is "Do not send it"; option 1 is "Send it". When the
owner chooses option 1, Fleet re-runs the held verb as the same Orchestrator with
`approvedBy: es-…`, and the verb validates again from scratch, because the world moved. Only a
**decide the owner answered with option 1** releases a held action; the Hub's `TaintAllows`
proves only that a decide was answered (`task-07.md:232-237`), so the choice check is Fleet's.

**(e) What the decide shows lives in the Hub.** What the human was shown is an accountability
fact, so it is a Hub record (Global constraint 1). Route 64's body (`task-07.md:127-133`) has no
field for it, so this task **requires of task 07** an optional `evidence` object on a decide:
`{"verb", "toAgentId"?, "task"?, "text"?, "read": [≤ 100 ids], "readTotal"}`. `text` is the
`free` directive's own text (≤ 600, `text·A`), which the `dv-…` would carry anyway; **a held
spawn's opening prompt is never in it** (OD-24: opening prompts are private; `task-24.md:130-135`),
and the owner reads it in their own panel from Fleet's held row. **Raised as gap G31-2.**

**(f) "Since its human last spoke" is a timestamp task 26 already sees.** When task 26 claims a
batch whose origin is `human`, it stamps `fleet.orch_sessions.human_spoke_at` with the newest
`human.message` event time in the batch (one line in task 26's claim path, below). The read list
is task 28's `fleet.message_reads` rows for the Orchestrator with `at > human_spoke_at`, newest
first, capped at 100 as route 67 requires, with the full count in `readTotal`. An Orchestrator
whose human never spoke reads from its session's start: every row task 28 still holds (7 days).

**(g) Delivered means written to the stream, at least once.** Dispatch drains a worker's pending
directives before its handler runs, so a refused call still delivers them, and marks them
delivered only after the response frame's write returns `nil`. A frame lost in flight leaves them
pending for the next call; each carries its `dv-…`, so a repeat is recognisable. A worker that
ends drops its undelivered rows in the same `Tx` as task 10's `Tx.EndAgent`.

**(h) `check`'s own `directives` key shows what is in force, not what is new.** Task 22 put a
`directives` key first in `CheckResult` (`task-22.md:267-276`), fed by `Directives.Pending`. With
piggyback at the response level, a pending directive is drained before `check` runs, so `Pending`
would always be empty there. This task implements task 22's interface as **in force**: the
latest of `pause`/`resume` if it is `pause`, the latest `narrow` with its globs, and each
`wait-for` whose lease is live, with no text. The hook runs `check` before every edit
(`task-18.md:62-67`), so a paused or narrowed worker is reminded on every edit. **Raised as gap
G31-3**: task 22's method is renamed `InForce`, its comment changed; its key and order are not.

**Files:**
- `control-plane/internal/fleet/directive/directive.go` — `Service`: `Issue`, `Drain`, `Ack`,
  `InForce`, `PauseForDefault` (task 30's `Pauser`), `Release` (new)
- `control-plane/internal/fleet/directive/taint.go` — `Gate.Judge`, the kind table, `Within`
  (new)
- `control-plane/internal/fleet/directive/held.go` — `Hold`, the decide's template and evidence,
  the held-action replay (new; not in the file map, it keeps the replay out of the gate)
- `control-plane/internal/fleet/verbs/direct.go` — the `direct` verb (new)
- `control-plane/internal/fleet/directive/{directive,taint,held}_test.go`,
  `control-plane/internal/fleet/verbs/direct_test.go` (new)
- `control-plane/internal/fleet/store/migrations/00NN_directives.sql` — `fleet.directive_queue`,
  `fleet.held_actions`, `orch_sessions.human_spoke_at` (new, numbered after task 30's)
- `control-plane/internal/fleet/store/{store.go,memstore.go,pgstore.go,storetest/suite.go}` — the
  `directive.Store` methods, and `OutboxMarkSent` stamping `directive_queue.hub_at` in the same
  statement's transaction (modified; task 10's files)
- `control-plane/internal/fleet/wire/control.go` and `sandboxd/internal/control/wire.go` —
  `Response.Directives` and `wire.Directive` (modified; task 09's files, one copy per module)
- `testdata/panacea-control/v1/response-piggyback.json`, `request-direct-narrow.json`,
  `request-direct-free.json`, `result-direct-issued.json`, `result-direct-held.json` (new, in task
  09's set)
- `control-plane/internal/fleet/control/{dispatch.go,streams.go}` — the drain before the handler,
  the ack after the write (modified; task 19's files)
- `sandboxd/internal/panaceacli/envelope.go` — prints `directives` above the result (modified; task
  18's file)
- `control-plane/internal/fleet/verbs/{task.go,worker.go}` — one `Gate.Judge` call each in `task
  create`, `task update` (a wider scope), `task abandon` and `worker spawn` (modified; task 24's
  files, `task-24.md:37-38`)
- `control-plane/internal/fleet/verbs/sources.go` — `Directives.Pending` → `InForce` (modified;
  task 22's file, gap G31-3)
- `control-plane/internal/fleet/turns/scheduler.go` — the `human_spoke_at` stamp on a `human`
  claim (modified; task 26's file, `task-26.md:68`, one statement)
- `control-plane/internal/fleet/escalate/{options.go,escalate.go}` — `Effect.Kind "send-held"`;
  `OnAnswered` calls `Held.Release` for it (modified; task 30's files, gap G31-4)
