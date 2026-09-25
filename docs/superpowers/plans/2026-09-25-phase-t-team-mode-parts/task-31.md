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

**Interfaces:**
- Produces:
  - `directive.Service` with `Issue(ctx, c control.Caller, a DirectArgs, approvedBy string)
    (DirectResult, error)`, `Drain(ctx, org string, worker model.AgentID, max int)
    ([]wire.Directive, error)`, `Ack(ctx, org string, worker model.AgentID, ids []string) error`,
    `InForce(ctx, worker model.AgentID) []verbs.PendingDirective`, `PauseForDefault(ctx, worker
    model.AgentID, es string) error`, `Release(ctx, org, es string) error`
  - `directive.Gate` with `Judge(ctx, c control.Caller, a Act) (Judgement, error)`; `Act`,
    `Judgement`, and the `Act.Kind` constants task 24's verbs and task 62's `build-on` pass
  - `wire.Directive` and `wire.Response.Directives` (ST.1, both modules, one fixture)
  - `verbs.Direct` — the `direct` handler, registered for `ClassOrchestrator` only
    (`task-09.md:297`)
- Consumes:
  - task 26: `turns.Scheduler.CurrentTurn(ctx, orch) (TurnRef, bool, error)` and
    `RecordIssued(ctx, orch, id)` (`task-26.md:663-665`); `TurnRef.Origin` (`:194-199`)
  - task 30: `escalate.Service.Decide(ctx, DecideRequest) (string, error)` (`task-30.md:340-352`,
    `:383`), the `free-directive-fleet-turn` and `scope-change` rows (`task-30.md:204-206`), and
    `Pauser` (`:370-372`), which `Service` satisfies
  - task 28: `fleet.message_reads` (`task-28.md:661-663`) for the read list
  - task 24: the worker's `Agent.Task` and the task card's `Scope` (1..20 globs,
    `task-24.md:81`), read through task 22's `Sources.Cards`
  - task 23: `leases.Engine.Live` for a `wait-for` lease's liveness
  - task 19: `control.Caller`, `control.Handler`, `wire.Refuse` with `Details`
  - task 11: `outbox.Enqueue`, `outbox.OpDirective` (`task-11.md:133`), whose target is task 07's
    route 67; task 02: `teamtext.New` with the 600 bound, `teamtext.ValidGlob`
    (`task-02.md:843`)
- Requires of **task 07**: `fleetTurnVerbs` = the five typed kinds (G31-1); a decide's optional
  `evidence` object on route 64 (G31-2); route 67's body carrying task 01's `params`
  (`task-01.md:431-432`), which task 07's shape (`task-07.md:160-166`) omits (G31-5).
- Requires of **task 10**: the tables below in both stores and the suite, and `OutboxMarkSent`
  setting `directive_queue.hub_at = at WHERE outbox_id = id` in the same transaction.
- Requires of **task 30**: `Effect{Kind: "send-held"}` and, in `OnAnswered`, `Held.Release(ctx,
  org, es)` when the chosen option's effect is `send-held` (G31-4). Task 30's hold (option 0) is
  already the default and the safe side.

**The ST.1 shapes, complete.** In `control-plane/internal/fleet/wire/control.go` and, byte for
byte in its fields and tags, `sandboxd/internal/control/wire.go`:

```go
// Response gains Directives, placed after OK so a printed frame shows them first.
type Response struct {
	ID         string          `json:"id"`
	OK         bool            `json:"ok"`
	Directives []Directive     `json:"directives,omitempty"` // task 31: workers only
	Result     json.RawMessage `json:"result,omitempty"`
	Error      *Error          `json:"error,omitempty"`
}

// Directive is one delivery to a worker, from its own Orchestrator. Text is set
// for kind "free" only, and it is the only place directive text crosses the wire.
type Directive struct {
	ID     string    `json:"id"`              // dv-…
	Kind   string    `json:"kind"`            // pause|resume|narrow|wait-for|rebase|free
	Globs  []string  `json:"globs,omitempty"` // narrow
	Lease  string    `json:"lease,omitempty"` // wait-for: ls-…
	Text   string    `json:"text,omitempty"`  // free: ≤ 600, text·A, already cleaned
	Author string    `json:"author"`          // always "agent": an Orchestrator wrote it
	Origin string    `json:"origin"`          // human | fleet
	At     time.Time `json:"at"`
}

// MaxPiggyback bounds one response's directives; the rest wait for the next call.
const MaxPiggyback = 8
```

`8 × 600` bytes of text is well under task 19's frame cap. The fixture `response-piggyback.json`
is `{"id":"c-7","ok":true,"directives":[{"id":"dv-00000000000000d1","kind":"narrow","globs":
["web/auth/**"],"author":"agent","origin":"fleet","at":"2026-09-25T01:00:00Z"}],"result":{…}}`,
and both modules' round-trip tests read it (the plan's rule for a shape both speak).

**`direct`'s args and result, complete** (`verbs/direct.go`; task 09's row, `task-09.md:607`):

```go
package verbs

// DirectArgs is `panacea direct <ag-…> <kind> …`. Exactly one of Globs, Lease
// and Text is set, and only for its kind; any other combination is bad_request.
type DirectArgs struct {
	To    model.AgentID `json:"to"`              // one of the caller's own workers
	Kind  string        `json:"kind"`            // pause|resume|narrow|wait-for|rebase|free
	Globs []string      `json:"globs,omitempty"` // narrow: 1..20, each inside the task scope
	Lease string        `json:"lease,omitempty"` // wait-for: a live ls-… in the project
	Text  string        `json:"text,omitempty"`  // free: 1..600
}

// DirectResult says what Fleet did. Exactly one of Directive and Escalation is set.
type DirectResult struct {
	Directive  string    `json:"directive,omitempty"`  // dv-…: issued, queued for delivery
	Escalation string    `json:"escalation,omitempty"` // es-…: held for the human's decide
	Deadline   *time.Time `json:"deadline,omitempty"`  // the decide's, when held (Go 1.23: no omitzero)
	Origin     string     `json:"origin"`              // the turn origin Fleet judged
}
```

`direct` is registered with `control.OwnArgs("to")` (task 19), so a `to` that is another owner's
agent, or no agent, is `agent_not_owner` before the handler runs. The handler adds the parent
check: `to` must be a worker whose `Parent` is the caller (task 07's route 67 rule,
`task-07.md:169-171`); a worker of the same owner under another Orchestrator is `agent_not_owner`
too, so one of a person's Orchestrators cannot steer another's.

**`taint.go`, the table and the judgement, complete.**

```go
package directive

// Act is one thing an Orchestrator asks Fleet to do that the taint rule governs.
type Act struct {
	Kind   ActKind
	Worker model.AgentID // directives, spawn: the worker concerned
	Task   string        // tk-… the act concerns
	Globs  []string      // narrow, or a scope update's new scope
	Scope  []string      // the task's current scope (task 24's card)
	Text   string        // free only; never a spawn prompt (OD-24)
	Verb   string        // the panacea verb, for the held replay
	Args   json.RawMessage
}

type ActKind string

const (
	ActPause   ActKind = "pause"
	ActResume  ActKind = "resume"
	ActNarrow  ActKind = "narrow"
	ActWaitFor ActKind = "wait-for"
	ActRebase  ActKind = "rebase"
	ActFree    ActKind = "free"
	ActNewTask ActKind = "task-create"
	ActWiden   ActKind = "scope-widen"
	ActAbandon ActKind = "task-abandon"
	ActSpawn   ActKind = "worker-spawn"
	ActBuildOn ActKind = "build-on" // task 62 calls Judge; task 30's locked row decides anyway
)

// fleetTurnPasses is §7.2 item 2's list, and nothing else. A kind absent here
// is a decide in a Fleet-started turn.
var fleetTurnPasses = map[ActKind]bool{
	ActPause: true, ActResume: true, ActNarrow: true, ActWaitFor: true, ActRebase: true,
}

// rowOf is the §6.5 row each held kind is written under (task 30's Table).
var rowOf = map[ActKind]string{
	ActFree: "free-directive-fleet-turn", ActNewTask: "free-directive-fleet-turn",
	ActSpawn: "free-directive-fleet-turn", ActBuildOn: "build-on",
	ActWiden: "scope-change", ActAbandon: "scope-change",
}

// Judgement is the gate's answer. Pass: go ahead now. Held: a decide was
// written and the act waits for it. Never both.
type Judgement struct {
	Pass       bool
	Origin     turns.Origin
	Escalation string     // es-… when held
	Deadline   time.Time  // when held
}

type Gate struct {
	Turns  CurrentTurner // task 26's Scheduler
	Held   *Held         // held.go
	Now    func() time.Time
}

// CurrentTurner is the one method of turns.Scheduler the gate needs.
type CurrentTurner interface {
	CurrentTurn(ctx context.Context, orch model.AgentID) (turns.TurnRef, bool, error)
}

// Judge applies §7.2. The order is fixed: shape rules that hold in every turn
// first, then origin. An error is never a pass.
func (g *Gate) Judge(ctx context.Context, c control.Caller, a Act) (Judgement, error) {
	if a.Kind == ActNarrow {
		for _, glob := range a.Globs {
			if !Within(glob, a.Scope) {
				return Judgement{}, &wire.Refusal{Code: wire.CodeGlobRefused,
					Message: "A narrow must stay inside the task's scope.",
					Details: map[string]string{"reason": "outside_task_scope", "glob": glob}}
			}
		}
	}
	origin := turns.OriginFleet // decision (c): no turn is a Fleet turn
	if ref, ok, err := g.Turns.CurrentTurn(ctx, c.Agent); err != nil {
		return Judgement{}, err
	} else if ok {
		origin = ref.Origin
	}
	if origin == turns.OriginHuman || fleetTurnPasses[a.Kind] {
		return Judgement{Pass: true, Origin: origin}, nil
	}
	if es, ok := approvalFrom(ctx); ok { // only Release sets it; no frame can
		return Judgement{Pass: true, Origin: origin, Escalation: es}, nil
	}
	es, deadline, err := g.Held.Hold(ctx, c, a, rowOf[a.Kind])
	if err != nil {
		return Judgement{}, err
	}
	return Judgement{Origin: origin, Escalation: es, Deadline: deadline}, nil
}

// Within reports whether glob g names only paths inside scope. It is
// deliberately conservative: g must equal a scope glob, or a scope glob must
// be "<dir>/**" and g must start with "<dir>/". A false refusal is possible
// (a narrow the human can issue in their own turn); a false pass is not.
func Within(g string, scope []string) bool {
	for _, s := range scope {
		if g == s {
			return true
		}
		if dir, ok := strings.CutSuffix(s, "/**"); ok && strings.HasPrefix(g, dir+"/") {
			return true
		}
		if s == "**" {
			return true
		}
	}
	return false
}
```

`Within` is not glob containment in general (that needs the two globs' languages), and it does
not claim to be. `web/auth/*.go` inside `web/**` passes; `web/auth/*.go` inside `web/*/*.go`
does not, though it is. The refusal names the glob, so the Orchestrator can restate the narrow
under a scope glob's directory (`web/auth/**`), which passes. A false refusal costs one retry; a
false pass would let a Fleet turn widen a worker. A `narrow` whose glob fails
`teamtext.ValidGlob` is `glob_refused` before `Within` runs.

**`Service.Issue`, the order.** One `Tx` from step 5 to step 7; nothing is written before it.

1. Decode `DirectArgs` strictly; the kind/field combination rule; `Text` through
   `teamtext.New(..., 600)` and `Clean` (task 02) for `free`, refused `text_refused` otherwise.
2. Load `to` (task 10's `Agent`); refuse unless `Class == worker`, `Parent == c.Agent`, and it is
   live. Load its card for `Scope` (task 22's `Sources.Cards`). For `wait-for`, the lease must be
   live in the caller's project (task 23's `Engine.Live`), whoever holds it: waiting for a
   teammate's lease is the point, and the lease id leaks nothing the caller's `team` view lacks.
3. `Gate.Judge`. Held → return `DirectResult{Escalation, Deadline, Origin}`; no `dv-…` exists.
4. Build the read list (decision f) and `approvedBy` (the releasing `es-…`, or `""`).
5. Mint `dv-…` (`ids`, task 09's prefix). `outbox.Enqueue` an `OpDirective` item under the
   owner's `orchestrator`-class delegation, the payload being route 67's body with task 01's
   `params`, `rung: "piggyback"`, `origin`, `read`, `approvedBy`.
6. Insert the `directive_queue` row with `outbox_id` and `hub_at NULL`.
7. Commit. Then `RecordIssued(ctx, c.Agent, dv)` (task 26), outside the `Tx`: a lost call loses
   one id from the turn record's `issued`, never a directive.

A `free` issued in a `human` turn is also what task 55's panel shows "inline, as issued" (§7.2
item 3): the panel reads the `dv-…` from task 52's read model; this task adds no push for it.

**`held.go`, the decide, complete in its words.** `Hold` validates nothing itself (the caller's
verb already did), writes the `held_actions` row and calls task 30's `Decide`:

```go
// holdQuestion is the only prose Fleet writes here. Every value is typed.
// %s: the act's product name; %s: the worker's display label; %d: reads.
const holdQuestion = "Your Orchestrator wants to %s for %s during a turn it started itself, " +
	"after reading %d teammate messages since you last spoke. Send it?"

var holdOptions = []escalate.Option{
	{Label: "Do not send it", Consequence: "Nothing changes. Your Orchestrator is told you said no.",
		Effect: escalate.Effect{Kind: "hold"}},
	{Label: "Send it", Consequence: "Fleet sends it in your name, exactly as shown.",
		Effect: escalate.Effect{Kind: "send-held"}},
}

var actName = map[ActKind]string{
	ActFree: "send a free instruction", ActNewTask: "create a new task",
	ActSpawn: "start a new worker", ActWiden: "widen a task's scope",
	ActAbandon: "abandon a task", ActBuildOn: "build on a teammate's lane",
}
```

The decide carries route 64's `evidence` (gap G31-2): `verb`, `toAgentId`, `task`, `text` for
`free` only, `read` and `readTotal`. `Group` is empty (one decider, `Own`). `Release(org, es)`
loads the held row, deletes it, and re-invokes its verb through the dispatcher's handler table as
the same caller under `withApproval(ctx, es)`, an unexported context key of package
`directive`; the verb's gate call reads it through `approvalFrom` and passes without a second
decide. `approvedBy` is never an argument: no field of any verb's args can set it, and a sandbox
frame cannot set a Go context value. A replay that now fails validation (the worker ended, the lease
lapsed) is reported to the Orchestrator as an `escalation.answered` event with the refusal code,
and nothing is issued. A decide that expires or is answered "Do not send it" deletes the row.
