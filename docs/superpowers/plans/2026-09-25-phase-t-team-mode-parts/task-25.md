### Task 25: Tier-P forecaster; conflict to thread

**Slice:** TM1-α. **Focused time:** 2.5 h. **dependsOn:** 07 (the Hub's thread routes), 24 (task
create's overlap); 23 (`leases.Overlap`), 11 (`outbox`, `HubState`) through them.

Tier P is the cheapest of §8.2's four tiers: **same paths**, a set intersection, run on every
signal. It is the one tier TM1-α runs, and the one that turns task 24's plan-time overlap into the
thing the whole phase exists for: a typed conflict both Orchestrators hear about **at once**, with
a thread already open between them, before any worker has written a line (§6.4, §8.1 item 2).

The forecaster is the **only** writer of conflicts (§3.3 conflict row: *"Fleet's forecaster
only"*). Every path to a `cf-…` goes through `Raise`, so one function decides the three things that
must not be decided twice:

1. **Different owners only.** Two agents of one owner are that owner's Orchestrator's business;
   a same-owner overlap raises nothing (§4.1, OD-16).
2. **One conflict per open pair.** A pair of agents that already has an open conflict (in task 11's
   `HubState`) **or** an open thread on one (task 28's thread store, which is durable and survives a
   restart before the conflict's outbox row is delivered) raises nothing new. The new paths are
   merged into Fleet's working copy of the open conflict, so `panacea check` and `team` show them,
   and are **not** re-sent: route 58 cannot patch a conflict's paths (`task-06.md:210-215`). The
   thread already exists; the Orchestrators see the wider overlap on their next `check`.
3. **Both queues, one call, no coalescing.** `Raise` enqueues one `conflict.opened` event for each
   Orchestrator in the same call. Task 26 makes that kind immediate (`task-26.md:121`, `:174-176`),
   so neither side waits up to 60 seconds for its turn.

**Wave order, and why this task tests against fakes.** Task 25 runs in TM1-α wave 17, and the two
things it calls are built after it: task 26's `turns.Queue.Enqueue` (wave 18) and task 28's
`protocol.OpenForConflict` (wave 19, `task-28.md:89-91`). Both are named in their parts already, so
this task declares each as a one-method interface over exactly that name, tests against a
recording fake, and wires the real ones in `cmd/fleetd` once they land. Nothing here is re-planned
when they arrive.

**Files:**
- `control-plane/internal/fleet/forecast/tierp.go` — `Forecaster`, `OnLease`, `Raise`, the quiet
  list (new)
- `control-plane/internal/fleet/forecast/conflicts.go` — the working copy of open conflicts;
  implements task 22's `verbs.Conflicts` (new)
- `control-plane/internal/fleet/forecast/{tierp,conflicts}_test.go` (new)
- `control-plane/cmd/fleetd/main.go` — wires `Forecaster` into `verbs.Deps.OnLease` (modify, one
  anchored hunk after task 23's engine is constructed)

**Interfaces:**
- Produces: `forecast.New(Deps) *Forecaster`; `(*Forecaster).OnLease`, `(*Forecaster).OnPaths`
  (the entry task 35 calls with observed edits and committed paths), `(*Forecaster).Raise`;
  `forecast.Signal`; `forecast.Open` (the working conflict); `(*Forecaster).Open(ctx, project)`
  satisfying `verbs.Conflicts`; `forecast.Quiet(path) bool`.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 23: `leases.Overlap` (`Holder`, `Owner`, `Paths`, `Globs`, `Basis`), `model.Lease`;
  - task 11: `delegation.HubState.PutConflict(hubapi.Conflict)` (`task-11.md:795-802`) and its
    read of open conflicts; `outbox.Enqueue`, `outbox.OpConflictCreate`, `outbox.OpConflictUpdate`
    (`task-11.md:124-125`);
  - task 06: route 57's body and route 58's `negotiating` + `threadId` patch
    (`task-06.md:198-216`);
  - task 28: `OpenForConflict(ctx, org, project, conflict string, orchA, orchB model.AgentID)
    (string, error)` and `ThreadFor` (`task-28.md:365`);
  - task 26: `Enqueue(ctx, model.Event) error`, `model.EventConflictOpened`;
  - task 10: the agent rows (parent Orchestrator, owner, task) via `store.Store`.
- Required by: 26 (the immediate kind has a producer), 28 (the thread's conflict subject), 29
  and 30 (they close or escalate what this opens), 33 (the scripted conflict), 35 (calls
  `OnPaths`), 36 (tier H runs beside this and reuses `Raise`).

#### `tierp.go` (complete public surface)

```go
package forecast

// The three things Raise calls that are built after it (see above).
type Queue interface {
	Enqueue(ctx context.Context, e model.Event) error // task 26
}
type Threads interface {
	OpenForConflict(ctx context.Context, org, project, conflict string,
		orchA, orchB model.AgentID) (string, error) // task 28
	HasOpenConflictThread(ctx context.Context, org string, a, b model.AgentID) (bool, error)
}

type Deps struct {
	Store   store.Store                // agents: owner, parent, task (task 10)
	Hub     *delegation.HubState       // open conflicts, tasks (task 11)
	Queue   Queue
	Threads Threads
	IDs     func() string              // ids.Wide("cf")
	Now     func() time.Time
}

type Forecaster struct {
	d  Deps
	mu sync.Mutex // serialises Raise per process; the pair check below is the cross-replica guard
}

func New(d Deps) *Forecaster

// Signal is one side of a possible conflict: an agent and the concrete
// paths or globs it has claimed or touched. Head is the basis sha for that
// side, or "" when none is known.
type Signal struct {
	Agent model.AgentID
	Owner model.Owner
	Paths []string // concrete, repo-relative
	Globs []string // intent or hold globs, when no concrete paths are known
	Head  string   // 40 or 64 hex, or ""
}

// OnLease is verbs.Deps.OnLease (task 23): each overlap of a just-granted or
// contested lease becomes one Raise with Tier P. Called after the lease's
// transaction commits, never inside it.
func (f *Forecaster) OnLease(ctx context.Context, org, project string,
	l model.Lease, ov []leases.Overlap) error

// OnPaths is task 35's entry: observed edits and committed paths for one
// agent, intersected with every other owner's live leases and observed
// paths. Present now so task 35 adds a caller, not an API.
func (f *Forecaster) OnPaths(ctx context.Context, org, project string, s Signal) error

// Raise is the one door to a cf-…. It returns ("", nil) when the pair is
// same-owner, already open, or every shared path is Quiet.
func (f *Forecaster) Raise(ctx context.Context, org, project string, tier string,
	a, b Signal, paths []string) (conflictID string, err error)

// Quiet reports a path tier P never raises on (§8.2 "Noise"): a lockfile.
// Generated files and the per-project co-edit rate are task 36's.
func Quiet(path string) bool
```

**The quiet list** is a fixed table of base names, so a reviewer can read all of it: `package-lock.json`,
`npm-shrinkwrap.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lockb`, `go.sum`, `Cargo.lock`,
`poetry.lock`, `Pipfile.lock`, `uv.lock`, `composer.lock`, `Gemfile.lock`, `packages.lock.json`.
A pair whose **only** shared paths are quiet raises nothing; a pair sharing a quiet path and a real
one raises on the real one alone.

**`Raise`, in order.** (1) `a.Owner == b.Owner` → nothing. (2) Drop quiet paths; none left →
nothing. (3) Orchestrators: `orchA`, `orchB` are each side's own Orchestrator (the agent itself if
it is one, else its parent); either missing (a member with no Orchestrator yet) → the conflict is
still raised, and no thread is opened until both exist (task 20 creates one at a member's first
team launch, so the gap is seconds). (4) The pair check: `HubState` has an open conflict whose
`{agentA, agentB}` is `{a, b}` in either order, **or** `HasOpenConflictThread(a, b)` → merge the
paths into the working copy and return. (5) One `Store.Tx`: `outbox.Enqueue` route 57's body with
`agentA = a.Agent` (the side whose signal triggered it, so the write goes under `a`'s owner's
delegation, `task-06.md:198-200`), `tier`, `paths` (≤ 50, sorted, each `{"path", "lines": []}`),
`basis`. (6) `HubState.PutConflict` with state `raised`. (7) `OpenForConflict(…, orchA, orchB)`,
then one `outbox.Enqueue` of route 58: `state: negotiating`, `threadId`. (8) `Enqueue` one
`conflict.opened` event for `orchA` and one for `orchB`, `Ref` the `cf-…`.

`paths` for a glob-only overlap with no tree (task 23's `basis: "glob"`, empty `Paths`) is the
narrower glob's literal directory prefix (`auth/**` against `auth/session/*.go` gives
`auth/session`), because route 57 takes paths, not globs. It is labelled in the working copy as
`approx: true`, so `panacea check` can say *"overlap on auth/session (by plan, not by file)"*.

**`HasOpenConflictThread` is an adapter**, in `cmd/fleetd`, over task 28's durable
`Store.OpenThreads(ctx, orchA)` (`task-28.md:397`): true when one of `orchA`'s open threads has a
`Conflict` (`task-28.md:380`, a `cf-…` id) whose `HubState` record's agent pair is `{a, b}`. It is declared here as one method so this package does not
import `protocol` (task 28 imports nothing of `forecast`, and neither should need the other).

**Basis shas, and a gap.** Route 57 takes `basis.a` and `basis.b` as 40 or 64 hex
(`task-06.md:205`) and does not say either may be absent. A tier-P conflict raised at **plan time**
has no commit on either side, and in TM1-α Fleet has no integration head either (task 22's
`unavailableTree`). Until tasks 01 and 06 make `basis` optional for tier P, the forecaster sends
**git's null object id**, forty `0`s, for a side with no `Head`: valid hex, and the value git itself
uses for "no object". The working copy marks it `basisKnown: false`. This is listed in the return
as a skeleton gap, not decided silently here.

**Two replicas.** `fleetd` runs two replicas (task 32). The in-process mutex serialises `Raise`
within one; across replicas the pair check reads `HubState`, which a second replica only learns
about through the feed. A race can therefore raise **two** conflicts for one pair in the few
seconds before the feed catches up. The durable guard is the thread: this part **asks task 28** to make
`OpenForConflict` return the existing thread for an Orchestrator pair that already has an open
conflict thread over the same agent pair. Task 28 does, and more broadly: one open thread per
Orchestrator pair per subject, a second opener gets the existing id (`task-28.md:53-55`),
and the second
`Raise` finds the first thread through `HasOpenConflictThread` on its step (4) if it runs even
slightly later. The residual (two conflicts, one thread) is harmless to the humans, who see one
thread, and is cleared by task 29's resolution, which closes every open conflict of the pair.
`TestTwoRaisesOneThread` pins the visible half. A Postgres advisory lock on the pair would close
it fully and is **not** added here, because the forecaster otherwise holds no transaction open
across the Hub-bound calls; it is recorded as a follow-up, not a defect.

#### `conflicts.go` (complete)

```go
// Open is Fleet's working copy of one open conflict, for task 22's check and
// team. The Hub record is authoritative; this adds only what Fleet merged or
// knows that the Hub record cannot carry.
type Open struct {
	ID         string
	Tier       string
	State      string // raised | negotiating | accepted | escalated
	A, B       model.AgentID
	Paths      []string // the Hub's, plus paths merged by later signals, sorted
	Approx     bool     // a path is a glob's literal prefix, not a file (no tree)
	BasisKnown bool
	Thread     string   // th-…, "" until opened
}

// Open satisfies verbs.Conflicts (task 22): every open conflict on project,
// sorted by id.
func (f *Forecaster) Open(ctx context.Context, project string) []verbs.Conflict
```

Task 22's `verbs.Conflict` has `ID, Tier, State, A, B, Paths`; `Open` fills exactly those, so
task 22 needs no change. `Approx` and `BasisKnown` stay in `forecast` until task 52's read model
asks for them.

- [ ] **Step 1: Write the failing tests first**

Fixture: Alice (Orchestrator `ag-a0`, worker `ag-a1`), Bob (Orchestrator `ag-b0`, worker `ag-b1`),
Alice's second worker `ag-a2`; task 11's `HubState`, task 08's fake Hub behind a recording outbox,
a recording `Queue`, a fake `Threads` that mints `th-…` ids and records calls.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTwoIntentsOneConflictOneThread` | Bob's intent `auth/**` then Alice's intent `auth/session.go` (through task 24's `task create`): exactly one `conflicts.create` (tier `P`, `agentA: ag-a0`, paths `[{"path":"auth/session.go","lines":[]}]`), one `OpenForConflict(ag-a0, ag-b0)`, one `conflicts.update` to `negotiating` with the `th-…` | raising per overlap without a pair check, or never opening the thread |
| `TestSameOwnerRaisesNothing` | `ag-a1` and `ag-a2` overlapping: zero outbox rows, zero threads, zero events | a forecaster that ignores owner |
| `TestBothQueuesOneCall` | after one `OnLease`, the recording queue holds exactly two `conflict.opened` events, for `ag-a0` and `ag-b0`, both `Ref` the `cf-…`, and they were enqueued inside that one call (no goroutine) | enqueueing only the requester's side, or deferring the other |
| `TestOpenPairRaisesNoSecond` | a second overlap between the same pair on `auth/token.go`: no new `cf-…`; the working copy's `Paths` now holds both files | a second conflict and a second thread for one pair |
| `TestOpenThreadGuardsAfterRestart` | with an empty `HubState` (a restart before the feed caught up) but `HasOpenConflictThread` true: no new conflict | trusting memory alone |
| `TestQuietOnlyRaisesNothing` | a shared `package-lock.json` alone raises nothing; with `src/a.ts` too, the conflict's paths are `["src/a.ts"]` only | lockfile noise at tier P (§8.2) |
| `TestAgentAIsTheTrigger` | when Alice's request triggers, `agentA` is Alice's agent and the outbox item's `Owner` is Alice | writing under the other owner's delegation, which route 57 refuses (`task-06.md:198-200`) |
| `TestNullBasisWhenUnknown` | with no `Head` on either side, `basis` is forty `0`s for both and `BasisKnown` is false | sending `""`, which route 57 refuses |
| `TestGlobPrefixApprox` | `auth/**` vs `auth/session/*.go` with no tree: path `auth/session`, `Approx: true` | sending a glob as a path |
| `TestNoOrchestratorYet` | Bob has no Orchestrator: the conflict is raised, no thread opened, one event for `ag-a0` only | refusing to raise, or opening a one-sided thread |
| `TestPathsCappedAt50` | 60 shared files: 50 paths sent, sorted, and the working copy keeps all 60 | a Hub 422 on a wide overlap |
| `TestTwoRaisesOneThread` | two `Raise` calls for one pair with `HubState` not yet updated between them (the two-replica race): at most one `OpenForConflict` returns a new thread | two threads for one pair |
| `TestConflictsSourceForCheck` | `Open(ctx, project)` returns the working copy as `[]verbs.Conflict` with merged paths; task 22's `check auth/token.go` lists it | task 22 reading only the Hub's paths |

```bash
cd control-plane && go test ./internal/fleet/forecast/... 2>&1 | head -3
```

Expected: `undefined: New` (build failure).

- [ ] **Step 2: `tierp.go`, then `conflicts.go`**

Implement `Raise` in the eight steps above. `OnLease` maps each `leases.Overlap` to two `Signal`s
(the lease's holder and the overlap's holder, each with its globs and, from the tree basis, the
concrete `Paths`) and calls `Raise` with tier `P`. No step holds `f.mu` across a network call:
the lock covers steps (1) to (6) only, and (7) and (8) run after it is released, so a slow Hub
cannot stall every other project's forecasts.

- [ ] **Step 3: Wire it**

In `cmd/fleetd/main.go`, after task 23's `leases.New`, anchored on that line:
`fc := forecast.New(forecast.Deps{…})`; `verbsDeps.OnLease = fc.OnLease`; `verbsDeps.Sources.Conflicts
= fc`. Until tasks 26 and 28 are merged, `Queue` and `Threads` are the logging stubs of their own
parts (`task-28.md:653`), which is TM1-α wave 17's honest state.

- [ ] **Step 4: Acceptance**

```bash
cd control-plane && go vet ./internal/fleet/... ./cmd/fleetd/... && go test ./internal/fleet/forecast/... ./internal/fleet/verbs/... -count=1 -race
```

Expected: `ok` for both, `-race` clean. Then the one-door guard, which must print nothing (no
package but `forecast` enqueues a conflict create):

```bash
cd control-plane && grep -rln "OpConflictCreate" internal/fleet | grep -v "internal/fleet/forecast/" | grep -v "internal/fleet/outbox/"
```

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/fleet/forecast
git commit -F <msgfile> -- control-plane/internal/fleet/forecast control-plane/cmd/fleetd/main.go
```

**What this task does NOT claim.**
- Tiers H, M or S. Tier H and the co-edit rate are task 36, tier M task 60, tier S is TM3.
- Forecasts from observed edits. `OnPaths` exists and is tested with a hand-built `Signal`; its
  real caller is task 35, after `machined`'s sweep (task 34). In TM1-α every conflict comes from
  a lease or an intent.
- That a thread resolves. Opening it is here; the protocol is task 28, resolutions task 29, the
  deadline escalation task 30.
- Exactly one conflict per pair across two replicas. One **thread** per pair is the visible
  guarantee; a duplicate conflict record in a seconds-wide race is possible and stated above.
- That the null basis is what the Hub will accept. It is valid hex under route 57 as written; the
  right fix is an optional `basis` for tier P, asked of tasks 01 and 06 in the return.
- That the file-level answer is precise without a tree. With `basis: "glob"` the path is a
  directory prefix, `Approx: true`; from task 34 on, the tree basis gives files.
