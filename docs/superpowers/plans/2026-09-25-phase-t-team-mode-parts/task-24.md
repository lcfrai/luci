### Task 24: Task, worker and report verbs; intent leases at plan time

**Slice:** TM1-α. **Focused time:** 2.5 h. **dependsOn:** 23 (the lease engine, `RequestIn`); 20
(`sessions.Launch`, `Stop`), 22 (the read sources), 26 (`turns.Queue.Enqueue`) through it.

These are the verbs by which an Orchestrator plans and runs its own human's work, and by which a
worker tells its own Orchestrator how it went. Two rules of the design meet here:

1. **Avoid first, at plan time** (§8.1 item 2, §6.3 item 1). `task create` takes `intent` leases on
   the task's scope **in the same transaction** that writes the task record to the outbox. An
   overlap with a teammate's live lease is therefore a typed fact before `worker spawn` is ever
   called, and task 25 turns it into a conflict and a thread while splitting or sequencing the work
   still costs nothing.
2. **Only the owner's own agents write the owner's rows** (§3.3 rules 1 and 5). Every verb takes
   the owner from `control.Caller`, never from args. Dispatch (task 19) already refuses an arg that
   names another owner's object; each handler checks again, because a handler is the last place
   that knows what the object is.

**Who holds an intent lease.** At plan time no worker exists, so the task's `intent` leases are
held by **the Orchestrator** with `Task` set to the `tk-…`. They are not moved when a worker
spawns: the worker's own claims arrive as `hold` leases, by request now and from observed edits in
task 35. Right of way reads `CreatedAt`, so a task planned first keeps its standing however late
its worker starts. The leases end when the task ends (`done` or `abandoned`), through task 23's
`EndTask`.

**Which delegation a spawned worker runs under.** A delegation is created only by the user's own
desktop (task 06 route 46 is desktop only), so Fleet cannot mint a worker-class one for a worker
its Orchestrator spawns. The spawned worker therefore runs under **its Orchestrator's**
`orchestrator`-class delegation: task 06's route 53 accepts an agent start from Fleet under either
class, and the Orchestrator's route set is every route (`task-01.md:271`). Its outbox items carry
`Class: orchestrator`. The worker still cannot reach the cross-user channel, because that is
refused by **task 19's verb table** (a worker calling `peer` is refused), not by its delegation.
This is recorded in the return: a reviewer should know the Hub-side route set does not fence a
spawned worker.

**Files:**
- `control-plane/internal/fleet/verbs/task.go` — `task create|update|abandon` (new)
- `control-plane/internal/fleet/verbs/worker.go` — `worker spawn|stop` (new)
- `control-plane/internal/fleet/verbs/report.go` — `report done|blocked|assumed` (new)
- `control-plane/internal/fleet/verbs/{task,worker,report}_test.go` (new)
- `testdata/panacea-control/v1/args-task-create.json`, `result-task-create-overlap.json`,
  `args-worker-spawn.json`, `args-report-blocked-human.json` (new fixtures)

**Interfaces:**
- Produces: the `task`, `worker` and `report` handlers, registered by `verbs.Register`;
  `verbs.TaskArgs`, `verbs.TaskResult`, `verbs.WorkerArgs`, `verbs.WorkerResult`,
  `verbs.ReportArgs`; `verbs.Report` (the Fleet-state row `panacea inbox` reads, task 28).
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 23: `leases.Engine.Validate`, `RequestIn`, `EndTask`; `leases.Overlap`, `leases.Way`;
  - task 20: `sessions.Launch(ctx, sessions.LaunchReq) (model.Agent, error)` and
    `sessions.Stop(ctx, org string, agent model.AgentID, exit string) error`;
  - task 26: `turns.Queue.Enqueue(ctx, model.Event) error` and the kinds `worker.done`,
    `worker.blocked`, `worker.assumed` (`task-26.md:113-120`);
  - task 11: `outbox.Enqueue`, `outbox.OpTaskCreate`, `outbox.OpTaskUpdate`
    (`task-11.md:120-122`);
  - task 06: route 50 and 51's bodies (`task-06.md:121-147`), including `titleSource` and
    `trackedFiles`;
  - task 02: `teamtext.Bound`, `teamtext.Clean`, `teamtext.KindTitle`, `teamtext.KindWhy`,
    `teamtext.KindMessage`, `teamtext.MaxGlobs`;
  - task 22: `verbs.Cards` (this task adds `Put` to the in-memory card set; see Step 3);
  - task 25: `Deps.OnLease`, called with the overlaps a `task create` produced.
- Required by: 25 (the overlap from a create), 26 (report events), 28 (`panacea inbox` reads
  reports), 29–31 (resolutions and directives act on these tasks and workers), 33 (the scripted
  conflict is two `task create` calls), 41 (the instructions name these verbs).

#### The wire (complete)

```go
// TaskArgs is `panacea task create|update|abandon`. Orchestrators only.
//   create:  Title, BriefRef, Why, Scope required; TitleSource optional
//   update:  Task required; any of State, Scope, Why
//   abandon: Task required, nothing else
type TaskArgs struct {
	Op          string   `json:"op"`                    // "create" | "update" | "abandon"
	Task        string   `json:"task,omitempty"`        // tk-…, update and abandon
	Title       string   `json:"title,omitempty"`       // ≤ 140
	TitleSource string   `json:"titleSource,omitempty"` // hm-…: the owner's own words (text·H)
	BriefRef    string   `json:"briefRef,omitempty"`    // brief key + "#" + section id
	Why         string   `json:"why,omitempty"`         // ≤ 200, text·A
	Scope       []string `json:"scope,omitempty"`       // 1..20 globs
	State       string   `json:"state,omitempty"`       // update: active | blocked | done
}

type TaskResult struct {
	Task           string            `json:"task"`           // tk-…
	State          string            `json:"state"`
	Revision       int               `json:"revision"`
	Leases         []LeaseView       `json:"leases"`         // the task's live intent leases
	Overlaps       []OverlapView     `json:"overlaps"`       // task 23's view, with Way
	BreadthChecked bool              `json:"breadthChecked"`
}

// WorkerArgs is `panacea worker spawn|stop`. Orchestrators only.
type WorkerArgs struct {
	Op      string `json:"op"`                // "spawn" | "stop"
	Task    string `json:"task,omitempty"`    // spawn: the caller's owner's task, not terminal
	Harness string `json:"harness,omitempty"` // spawn: a catalogue id the class policy allows
	Brief   string `json:"brief,omitempty"`   // spawn: the worker's opening prompt, ≤ 2000, text·A
	Agent   string `json:"agent,omitempty"`   // stop: ag-… of the caller's owner's worker
}

type WorkerResult struct {
	Agent  string `json:"agent"`  // ag-…
	Status string `json:"status"` // starting on spawn; ended on stop
}

// ReportArgs is `panacea report done|blocked|assumed <text>`. Workers only.
type ReportArgs struct {
	Kind string `json:"kind"`         // "done" | "blocked" | "assumed"
	Text string `json:"text"`         // ≤ 600, text·A
	On   string `json:"on,omitempty"` // "human", with blocked only
}

// Report is Fleet state, not a Hub record (§3.3 lists none): a worker's word
// to its own Orchestrator, read by `panacea inbox` (task 28) and deleted 24 h
// after the turn that consumed it (task 26's EndTurn).
type Report struct {
	ID           string        // ids.New("rp"), Fleet-local
	Org          string
	Orchestrator model.AgentID // the worker's parent, same owner
	Worker       model.AgentID
	Kind         string
	Text         teamtext.Text // Author agent, AuthorAgent = Worker
	On           string
	At           time.Time
}
```

**A spawned worker's opening prompt is private too (OD-24).** `Brief` goes to `sessions.Launch`
as the harness argument and nowhere else: it is not a card field, not a Hub field and not a
`Report`. The same marker-grep test as task 22 covers it. It is `text·A` because the
Orchestrator wrote it, and it is `teamtext.Clean`ed, because it is the one place an Orchestrator's
words become a worker's instructions; the taint rule (task 31) governs *whether* a Fleet-origin turn
may spawn at all, and this task does not pre-empt it.

**Error codes** (ST.1 envelope): `bad_request`; `text_refused`, `glob_refused`, `breadth_exceeded`,
`breadth_unknown` (tasks 02, 23); `agent_not_owner` (a task or worker of another owner, whatever
the caller's role); `not_found`; `conflict` with `details {from,to}` (a task transition task 06
refuses: `planned → active | abandoned`, `active → blocked | done | abandoned`,
`blocked → active | abandoned`, `task-06.md:144-146`); `task_has_workers` (abandon or done while a
live worker is on the task: stop it first, so no worker keeps editing under a released lease).

- [ ] **Step 1: Write the failing tests first**

Fixture: Alice (Orchestrator `ag-a0`), Bob (Orchestrator `ag-b0`, worker `ag-b1` on `tk-b` with an
intent lease on `auth/**`), the tracked tree of task 23's tests, task 20's fake sessions, task 08's
fake Hub behind the outbox, and a recording `turns.Queue`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCreateTakesIntentInSameTx` | after `task create` with scope `["auth/session/**"]`: one outbox `tasks.create` and two `leases.events` rows, all in one `Tx`; a failing `AppendOutbox` on the lease leaves **no** task row either | writing the task, then leasing in a second transaction |
| `TestCreateOverlapBeforeSpawn` | Alice's create over Bob's intent returns one overlap with `way.holder: ag-b0`, and `OnLease` was called with it; `sessions.Launch` was **never** called | the overlap found only when a worker edits (§8.1 item 2) |
| `TestCreateRefusedGlobWritesNothing` | a scope holding `**` gives `glob_refused` and zero outbox rows, zero leases | validating globs after the task is enqueued |
| `TestCreateForOtherOwnerRefused` | the args carry no owner field (strict decode refuses `"owner"`), and a `task update` on Bob's `tk-b` from `ag-a0` is `agent_not_owner` | trusting an owner in args, or checking ownership only in dispatch |
| `TestWorkerCannotCreateTask` | a worker-class caller gets the class refusal for `task`, `worker` | a verb table that lets a worker plan |
| `TestTitleSourceMakesHuman` | with `titleSource: hm-1` whose trimmed body equals the title, the outbox payload carries `titleSource` and the card's `Title.Author` is `human`; a different body gives `agent` | labelling every title `text·H`, or trusting the claim without the match |
| `TestCreateSendsTrackedFiles` | with a tree: `trackedFiles: 100`; without: `trackedFiles: 0` and `breadthChecked:false` | a hard-coded count the Hub then checks against |
| `TestUpdateScopeReissuesIntents` | changing scope from `["a/**"]` to `["b/**"]` releases the `a/**` intent and requests `b/**`, with the `revision` sent as card revision + 1 | stale intents surviving a re-plan |
| `TestTransitionsMatchHub` | every (from, to) pair: accepted iff task 06 accepts it | a local state machine that disagrees with the Hub, which then dead-letters the write |
| `TestAbandonWithLiveWorkerRefused` | `abandon` while `ag-a1` runs on the task is `task_has_workers`; after `worker stop`, it succeeds and `EndTask` released every intent | releasing leases under a live worker |
| `TestSpawnActivatesTask` | the first `worker spawn` on a `planned` task moves it to `active` (one `tasks.update`) and launches under the Orchestrator's delegation with `Class: orchestrator` | a worker started under a worker delegation Fleet cannot have |
| `TestSpawnOtherOwnersTaskRefused` | `ag-a0` spawning on `tk-b` is `agent_not_owner`; no launch | spawning onto a teammate's task |
| `TestStopOnlyOwn` | `ag-a0` stopping `ag-b1` is `agent_not_owner`; stopping its own `ag-a1` ends it and calls `EndAgent` | OD-16's "no user steers another user's agent", broken |
| `TestReportGoesToOwnOrchestrator` | `report done "tests pass"` from `ag-a1` enqueues `worker.done` for `ag-a0`, `Author: ag-a1`, and **nothing** for `ag-b0` | reports broadcast to the project |
| `TestBlockedOnHumanSetsWaitingOn` | `report blocked --on human` sets `waitingOn: human`; plain `blocked` leaves it empty | inferring `human` from the word "blocked" (phase 2 refuses to guess) |
| `TestOnHumanOnlyWithBlocked` | `report done --on human` is `bad_request` | a second source of `waitingOn: human` |
| `TestReportTextRefused` | a report holding `U+202E` is `text_refused`; nothing enqueued | agent text reaching an Orchestrator uncleaned |
| `TestSpawnBriefNeverStored` | a spawn with brief `PROMPT-MARKER-9`: the marker is absent from every outbox payload, card, `Report` and `team` result | the spawn prompt copied into the card |

```bash
cd control-plane && go test ./internal/fleet/verbs/... -run "Task|Worker|Report|Spawn|Create|Stop|Blocked|Abandon|Update|Title" 2>&1 | head -3
```

Expected: `undefined: TaskArgs` (build failure).

- [ ] **Step 2: `task.go`**

`create` in order, refusing before anything is written: decode strictly; `Bound` and `Clean` the
title (`KindTitle`) and why (`KindWhy`); check `BriefRef` against task 06's grammar (a brief key,
`#`, then `^[a-z0-9][a-z0-9-]{0,63}$`); 1 to `MaxGlobs` scope globs; `leases.Engine.Validate` for
the breadth. If `TitleSource` is set, read that `hm-…` from task 11's in-memory Hub state and use
it only if its owner is the caller's owner and its trimmed body equals the title; otherwise send no
`titleSource`, so the Hub labels the title `agent`. Then one `Store.Tx`: `outbox.Enqueue` the route
50 body (`id` a new `tk-…`, `orchestratorAgentId` the caller, `trackedFiles` from `Validate`), then
`RequestIn` with `Mode: intent`, `Task: tk-…`, `Holder: c.Agent`. After commit: put the card in
the in-memory set (it is Hub-owned; the rebuild of task 11 would restore it after a restart from
the Hub's copy once delivered), and call `Deps.OnLease(overlaps)`.

`update` computes `revision = card.Revision + 1` locally. Fleet is the only writer of a team task
(route 51 is Fleet·O, owner only), so a 409 `revision_conflict` from the Hub means Fleet's own
state is wrong: the outbox classifies it permanent (task 11) and the operator alert fires; it is
not retried with a fresh revision, which would hide the bug.

- [ ] **Step 3: `worker.go`**

`spawn`: the task must be the caller's owner's and not terminal; `sessions.Launch` with `Class:
worker`, `Parent: c.Agent`, `Task`, `Harness`, the brief as the harness's opening prompt, and the
**Orchestrator's own delegation** (task 20 reads it from the Orchestrator's agent row); on a
`planned` task, one `tasks.update` to `active` in the same `Tx` as the agent row task 20 writes.
`stop`: the agent must be a worker whose parent's owner is the caller's owner; `sessions.Stop(...,
"stopped")`, then `leases.Engine.EndAgent`.

- [ ] **Step 4: `report.go`**

Workers only. `Bound(KindMessage)` and `Clean` the text; build a `teamtext.Text` with `Author:
agent`, `AuthorAgent: c.Agent`; in one `Tx` put the `Report` and, for `blocked` with `On: human`,
the agent's `waitingOn` (task 10's agent status row); then `turns.Queue.Enqueue` the matching kind
for the worker's parent. The `waitingOn: human` flag clears on the worker's next `panacea` call
that is not `report`, or on a `resume` directive (task 31); nothing else sets it.

- [ ] **Step 5: Acceptance**

```bash
cd control-plane && go vet ./internal/fleet/... && go test ./internal/fleet/verbs/... ./internal/fleet/leases/... -count=1 -race
```

Expected: `ok` for both. Then the owner guard, which must print nothing (no handler reads an
owner out of args):

```bash
cd control-plane && grep -n "args\.Owner\|a\.Owner\|\"owner\"" internal/fleet/verbs/task.go internal/fleet/verbs/worker.go internal/fleet/verbs/report.go
```

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/fleet/verbs/task*.go control-plane/internal/fleet/verbs/worker*.go \
  control-plane/internal/fleet/verbs/report*.go testdata/panacea-control/v1/*task*.json \
  testdata/panacea-control/v1/*worker*.json testdata/panacea-control/v1/*report*.json
git commit -F <msgfile> -- control-plane/internal/fleet/verbs testdata/panacea-control/v1
```

**What this task does NOT claim.**
- That the Orchestrator plans well. Whether it reads `panacea team` first and plans around live
  leases is its instructions' job (task 41) and G9's (task 42).
- That a conflict is raised. This task hands the overlap to task 25; with `OnLease` nil the
  overlap is returned to the Orchestrator and nothing else happens.
- That a spawned worker is fenced by the Hub's route set. It runs under its Orchestrator's
  delegation; the fence against the cross-user channel is task 19's verb table.
- Where `Report` rows live in Postgres. Task 10 owns the schema; this part requires a `reports`
  table (id, org, orchestrator, worker, kind, text JSON, on, at, consumed_turn) and says so in the
  return.
- That a spawn obeys the taint rule. Whether a Fleet-origin turn may spawn is task 31's check.
