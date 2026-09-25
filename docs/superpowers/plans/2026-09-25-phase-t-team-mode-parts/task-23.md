### Task 23: Lease engine: intent, right of way, breadth cap, request and release

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 10 (`Store`, `Tx`), 22 (`TreeSource`, the
read sources); 02 (`teamtext`), 09 (`model.Lease` and its enums), 11 (`outbox.Enqueue`) through them.

A lease is a soft claim with an expiry (§6.3). It is **advisory**: Fleet never refuses a write
(P7); `panacea check` reports the holder, the worker's managed block says to report `blocked`
rather than edit a file someone else holds, and phase 3's reviewed land is the backstop. What a
lease buys is *knowing early*: an overlap is a typed fact the forecaster (task 25) turns into a
conflict and a thread, ideally before any worker exists (task 24's intent at plan time, §8.1).

This task is the engine and the one verb that calls it. It owns four things:

1. **The state machine** of §3.3's lease row, with every transition written to the Hub through the
   outbox **in the same transaction** as the Fleet row (task 11's rule; `task-11.md:184-203`).
   Fleet is authoritative; the Hub only checks shape, owner and `seq` (`task-01.md:341-347`).
2. **The breadth cap** (§6.3 item 1, `07` I2): every glob passes `teamtext.ValidGlob`, which already
   refuses `**`, `*` and `**/*` as `whole_repo` (`task-02.md:36-38`), and then
   `teamtext.CheckSet(globs, tracked)` against the integration head's tracked files, which refuses
   one glob matching more than `teamtext.MaxBreadth` (10%) with `breadth_exceeded`.
3. **Overlap as a typed value**, returned to the caller and handed to the forecaster. The engine
   **raises nothing** itself: turning an overlap into `cf-…` and `th-…` is task 25's, so there is
   one place that decides whether a pair already has an open thread.
4. **Right of way** (§6.3 items 5–6) as a pure function, so task 29's resolutions and task 30's safe
   default read one answer.

**The tree in TM1-α.** Task 22 wires `unavailableTree` in TM1-α, because `machined`'s tree read is
task 34's (TM1). Without a file list the 10% check cannot run. The engine then **fails closed on
the shape it cannot measure**: a glob whose first segment contains a wildcard (`*.go`, `**/x`,
`{a,b}/…`) is refused with `breadth_unknown`; a glob with a literal first directory (`auth/**`,
`src/export/*.ts`) is accepted, recorded with `trackedFiles: 0` (the Hub's "count not supplied",
`task-01.md:285-288`) and flagged `breadthChecked: false` on the lease view. Task 33 records that
TM1-α ran without the 10% cap; from task 34 on the flag is always true. This is a deliberate
weakening for two weeks, stated, not a silent one.

**Files:**
- `control-plane/internal/fleet/leases/leases.go` — `Engine`, `Request`, `Release`, `EndTask`,
  `EndAgent`, `Reevaluate`, the transition table (new)
- `control-plane/internal/fleet/leases/rightofway.go` — `RightOfWay`, `Contested` (new)
- `control-plane/internal/fleet/leases/overlap.go` — `Overlap` and its computation (new)
- `control-plane/internal/fleet/verbs/lease.go` — `panacea lease request|release` (new)
- `control-plane/internal/fleet/leases/{leases,rightofway,overlap}_test.go`,
  `control-plane/internal/fleet/verbs/lease_test.go` (new)
- `testdata/panacea-control/v1/args-lease-request.json`, `result-lease-granted.json`,
  `result-lease-contested.json` (new fixtures in task 09's directory)

**Interfaces:**
- Produces: `leases.Engine` with `New(Deps)`, `Request`, `Release`, `EndTask`, `EndAgent`,
  `Reevaluate`, `Live`; `leases.Overlap`; `leases.RightOfWay`, `leases.Contested`,
  `leases.Side`; `leases.ErrBreadthUnknown`; `leases.HoldTTL` (30 min); the `lease` verb handler.
- Consumes, **written against these names; where the owning task spells one differently, it wins
  and this part is renamed mechanically**:
  - task 09: `model.Lease`, `model.LeaseMode` (`LeaseIntent`, `LeaseHold`, `LeaseBulk`),
    `model.LeaseState` (the seven of §3.3), `model.AgentID`, `model.Owner`, `model.Class`;
  - task 10: `store.Store.Tx(ctx, func(store.Tx) error) error`; on `store.Tx`: `PutLease`,
    `Lease(id)`, `LiveLeases(org, project)`, `AppendOutbox` (`task-11.md:85`);
  - task 11: `outbox.Enqueue(ctx, tx, outbox.Item)`, `outbox.OpLeaseEvent`
    (`task-11.md:120-126`, `:203`);
  - task 22: `verbs.TreeSource`, `verbs.Tree`, `verbs.ErrTreeUnavailable`, `verbs.Activity`
    (nil in TM1-α);
  - task 02: `teamtext.ValidGlob`, `teamtext.CheckSet`, `teamtext.Match`, `teamtext.MaxGlobs`,
    `teamtext.ErrBreadth`, `teamtext.ErrGlob`;
  - task 19: `control.Caller`, `control.Dispatcher.Handle`.
- Required by: 24 (intent at task create), 25 (overlaps), 29 (`yield`, `sequence` re-issue
  leases), 30 (the contested worker), 35 (holds, bulk, per-path expiry extend this engine), 44
  (leavers' leases), 52 (the lease view).

#### `leases.go` (complete public surface)

```go
package leases

const HoldTTL = 30 * time.Minute // §6.3 item 3; task 35 makes it per path

var ErrBreadthUnknown = errors.New("leases: breadth cannot be measured without the tree")

// Tracked returns the tracked files at the integration head, or ErrNoTree.
// It is a func, not verbs.TreeSource, because verbs imports leases (the
// lease verb) and an interface from verbs here would be an import cycle.
// verbs/lease.go adapts task 22's TreeSource to it, mapping
// verbs.ErrTreeUnavailable to ErrNoTree.
type Tracked func(ctx context.Context, owner model.Owner, project string) ([]string, error)

var ErrNoTree = errors.New("leases: integration tree unavailable")

type Deps struct {
	Store   store.Store
	Tracked Tracked          // returns ErrNoTree in TM1-α
	Now     func() time.Time
	IDs     func() string    // ids.Wide("ls") (task 03's grammar)
}

type Engine struct{ d Deps }

func New(d Deps) *Engine

// Req is one request. Holder, Owner, Org, Project come from control.Caller,
// never from args (§3.3 rule 1). Task is the holder's task for an intent.
type Req struct {
	Org, Project string
	Holder       model.AgentID
	Owner        model.Owner
	Class        model.Class
	Task         string // tk-… for intent; "" for a hold
	Globs        []string
	Mode         model.LeaseMode // intent | hold; bulk is refused here (task 35 observes it)
}

// Result is what Request returns: the lease as stored and every overlap it
// has with another owner's live lease. Overlaps is non-nil.
type Result struct {
	Lease    model.Lease
	Overlaps []Overlap
	BreadthChecked bool
}

// Request validates, measures breadth, computes overlaps, and in ONE
// transaction writes the lease in state granted (no overlap) or contested
// (any overlap with another owner) plus its outbox lease events. The Hub
// sees two events, requested then the outcome, seq 1 and 2, so its history
// matches §3.3's states exactly.
func (e *Engine) Request(ctx context.Context, r Req) (Result, error)

// Release is the holder's own release of its own lease; any other caller is
// agent_not_owner. Released is terminal. It then calls Reevaluate.
func (e *Engine) Release(ctx context.Context, org string, holder model.AgentID, id string) error

// EndTask releases every live lease tied to task; EndAgent every lease the
// agent holds. Both are called by task 24 (task abandon/done) and task 20
// (agent end). Idempotent: a second call writes nothing.
func (e *Engine) EndTask(ctx context.Context, org, task string) (int, error)
func (e *Engine) EndAgent(ctx context.Context, org string, agent model.AgentID) (int, error)

// Reevaluate moves every contested lease on project whose overlaps are all
// gone to granted, one event each. Called after any release or expiry.
func (e *Engine) Reevaluate(ctx context.Context, org, project string) (int, error)

// Live returns the project's live leases (requested, granted, contested).
func (e *Engine) Live(ctx context.Context, org, project string) ([]model.Lease, error)
```

**The transition table**, the only moves `leases.go` makes (a test ranges over every pair of the
seven states and asserts that exactly these are accepted):

| From | To | Caused by | Task |
|---|---|---|---|
| (none) | `requested` | `Request` | 23 |
| `requested` | `granted` · `contested` | `Request`, same transaction | 23 |
| `contested` | `granted` | `Reevaluate` | 23 |
| `granted` · `contested` | `released` | `Release`, `EndTask`, `EndAgent` | 23 |
| `granted` · `contested` | `yielded` | an accepted `yield` | 29 |
| `granted` · `contested` | `expired` | a hold's per-path TTL, a lapsed delegation | 35, 11 |
| `granted` · `contested` | `admin-released` | an admin, audited | 44 |

`yielded`, `released`, `expired` and `admin-released` are terminal, which is the Hub's rule too
(`task-01.md:344-346`). The moves tasks 29, 35 and 44 own are in the table so the table test covers
them now; their functions arrive with those tasks.

**The Hub payload** is task 01's §7.16 body exactly (`task-01.md:341-343`), built by one function
so the `seq` rule cannot drift:

```go
func leaseEvent(l model.Lease, seq int, state model.LeaseState, tracked int, at time.Time) outbox.Item
// Payload: {"seq","holderAgentId","mode","globs","trackedFiles","state","expiresAt","at"}
// Owner: l.Owner; Class: the holder's class; Op: outbox.OpLeaseEvent; Subject: l.ID
```

`model.Lease.Seq` is the last `seq` written; the store increments it inside the transaction, so a
retried `Request` cannot write `seq` 2 twice.

#### `overlap.go` and `rightofway.go` (complete)

```go
// Overlap is one other owner's live lease that intersects a request. Only
// DIFFERENT owners overlap: an owner's own agents are its Orchestrator's to
// sequence, and the forecaster ignores same-owner pairs anyway (task 25).
type Overlap struct {
	Lease  string          `json:"lease"`  // the other ls-…
	Holder model.AgentID   `json:"holder"`
	Owner  model.Owner     `json:"owner"`
	Mode   model.LeaseMode `json:"mode"`
	Since  time.Time       `json:"since"`  // the other lease's CreatedAt
	Paths  []string        `json:"paths"`  // tracked paths both sides match, sorted, ≤ 50
	Globs  [2]string       `json:"globs"`  // the first intersecting pair: [mine, theirs]
	Basis  string          `json:"basis"`  // "tree" | "glob"
}

// overlaps compares r's globs with every live lease of another owner.
// Basis "tree": the tracked files matching one of mine AND one of theirs
// (teamtext.Match both ways); empty means no overlap. Basis "glob" (no
// tree): globsMeet below, and Paths holds only literal paths.
// tracked is nil when Deps.Tracked returned ErrNoTree.
func (e *Engine) overlaps(ctx context.Context, r Req, live []model.Lease, tracked []string) []Overlap

// globsMeet is the no-tree test. A literal path meets a glob it Matches. Two
// globs meet when one's literal directory prefix is a prefix of the
// other's (auth/** meets auth/*.go; auth/** does not meet docs/**). It can
// say "meet" for two globs no real file satisfies; it never says "apart" for
// two that share a file. Advisory leases prefer a false alarm to a miss.
func globsMeet(a, b string) bool
```

```go
// Side is one party to an overlap, as right of way sees it.
type Side struct {
	Lease   model.Lease
	Matched int // tracked files its globs match; -1 when the tree is unknown
}

type Reason string

const (
	ReasonNotBulk         Reason = "not-bulk"         // the other side is bulk (§6.3 item 2)
	ReasonEarlierNarrower Reason = "earlier-narrower" // the default (§6.3 item 5)
	ReasonNarrower        Reason = "narrower"         // later but narrower: wins on its paths
	ReasonEarlier         Reason = "earlier"          // equal breadth
)

type Way struct {
	Holder model.AgentID `json:"holder"` // who has right of way on the overlap
	Reason Reason        `json:"reason"`
}

// RightOfWay decides, in this order:
//  1. exactly one side is bulk            -> the other, ReasonNotBulk
//  2. one side is narrower                -> it; ReasonEarlierNarrower if it is also
//                                            earlier, else ReasonNarrower
//  3. equal breadth, or both bulk         -> the earlier CreatedAt, ReasonEarlier;
//                                            an exact tie -> the smaller ls-… id
// "Narrower": with both Matched >= 0, fewer matched files. With either -1, a
// literal path is narrower than any glob, and between globs the one with
// more literal leading segments is narrower; otherwise equal.
func RightOfWay(a, b Side) Way

// Contested names "the contested worker" on path (§6.3 item 6), which the
// safe default pauses (§6.5, task 30):
//  - both have edited path          -> the newer edit
//  - neither has                    -> the side without right of way
//  - exactly one has                -> the side without right of way (see below)
// lastEdit is task 35's per-path edit time; nil in TM1-α, which is "neither".
func Contested(a, b Side, path string,
	lastEdit func(model.AgentID, string) (time.Time, bool)) model.AgentID
```

**An interpretation, stated.** §6.3 item 6 covers "both edited" (newer edit) and "neither" (no
right of way) and is silent on "exactly one". This task gives that case to the side **without right
of way**, because pausing the only worker actually editing a path, for a lease the other side
merely requested, would reward claiming over working. It is one line in `Contested` and one test,
`TestContestedOneEditorIsNotPaused`, so the owner can reverse it cheaply. Recorded in the return.

#### `verbs/lease.go`: the `lease` verb (complete wire)

```go
// LeaseArgs is `panacea lease request|release <globs>`.
//   request: Globs 1..teamtext.MaxGlobs; Mode "" (hold) or "intent" (an
//            Orchestrator only, and only with Task set to its own owner's task)
//   release: Lease set, Globs empty; the caller's own lease only
type LeaseArgs struct {
	Op    string   `json:"op"` // "request" | "release"
	Globs []string `json:"globs,omitempty"`
	Mode  string   `json:"mode,omitempty"`
	Task  string   `json:"task,omitempty"`
	Lease string   `json:"lease,omitempty"`
}

// LeaseResult answers a request. Right of way is computed for each overlap
// so the caller learns at once whether it is the side that must propose.
type LeaseResult struct {
	Lease          LeaseView    `json:"lease"`          // task 22's view type
	BreadthChecked bool         `json:"breadthChecked"` // false in TM1-α (no tree)
	Overlaps       []OverlapView `json:"overlaps"`
}

type OverlapView struct {
	leases.Overlap
	OwnerName string      `json:"ownerName"` // cleaned display name (task 22's out.clean)
	Way       leases.Way  `json:"way"`
}
```

Error codes (ST.1 envelope): `bad_request` (shape, a `bulk` mode, `intent` from a worker, a
`task` that is not the caller's owner's), `glob_refused` and `breadth_exceeded` (task 02's
sentinels, mapped by `errors.Is`), `breadth_unknown` (TM1-α, a leading wildcard with no tree),
`agent_not_owner` (release of another's lease), `not_found` (no such lease). After a granted or
contested `Request`, the handler hands `Result.Overlaps` to task 25's `forecast.OnLease`, which
this task declares as a `func` field in `verbs.Deps` and task 25 fills; nil means no forecaster
(this task's own tests).

- [ ] **Step 1: Write the failing tests first**

Fixtures: a tracked tree of 100 files, 12 under `auth/` (so `auth/**` is 12%, over the cap), 5 under
`auth/session/` (5%); Alice's worker `ag-a1`, Bob's worker `ag-b1`, Alice's second worker `ag-a2`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWholeRepoRefused` | `**`, `*`, `**/*` are `glob_refused`, no row written | skipping `ValidGlob` |
| `TestBreadthCapPerGlob` | `auth/**` (12%) is `breadth_exceeded`; `auth/session/**` (5%) is granted | comparing against 100% or a union |
| `TestBreadthUnknownFailsClosed` | with `unavailableTree`: `**/*.go` is `breadth_unknown`; `auth/session/**` is granted with `breadthChecked:false` and Hub `trackedFiles: 0` | accepting every glob when the tree is down |
| `TestTwentyOneGlobsRefused` | 21 globs are `bad_request` | an off-by-one on `MaxGlobs` |
| `TestOverlapOtherOwnerOnly` | `ag-a2`'s request over `ag-a1`'s lease is granted with no overlap; `ag-b1`'s is contested with one | treating a same-owner pair as a conflict |
| `TestOverlapTreeBasis` | `auth/session/**` vs `auth/session/token.go` overlaps on exactly that path, `basis:"tree"` | a set test that returns every file of either side |
| `TestGlobsMeetConservative` | table: `auth/**`·`auth/*.go` meet; `auth/**`·`docs/**` apart; `a/b.go`·`a/*.go` meet; `a/*.go`·`a/b/*.go` meet | a prefix test on raw strings (`auth` meeting `authz/**`) |
| `TestOneTransactionTwoEvents` | a contested request writes the lease row and outbox rows `seq 1 requested`, `seq 2 contested` in one `Tx`; a failing `AppendOutbox` leaves no lease row | writing the row and the outbox separately |
| `TestTransitionTable` | every (from, to) pair of the 7 states: accepted iff in the table | a release after `yielded` |
| `TestEndTaskReleasesAll` | a task with 3 intent leases: `EndTask` releases 3; a second call releases 0 and enqueues nothing | leases outliving the task (§6.3 item 3) |
| `TestReleaseReevaluates` | Bob's contested lease becomes `granted` when Alice releases; one event | contested forever |
| `TestReleaseOthersRefused` | Bob releasing Alice's lease is `agent_not_owner` | owner taken from args |
| `TestRightOfWayEarlierNarrower` | earlier 5-file vs later 12-file: earlier, `earlier-narrower` | swapping the comparison |
| `TestRightOfWayLaterNarrower` | earlier 12-file vs later 1-file: later, `narrower` | "earlier always wins" |
| `TestRightOfWayBulkNever` | an earlier, narrower `bulk` loses to a later `hold`: `not-bulk` | bulk keeping right of way |
| `TestRightOfWayTie` | equal breadth: earlier; exact tie: smaller id | a map-order winner |
| `TestRightOfWayNoTree` | `a/b.go` beats `a/**`; `a/b/**` beats `a/**` | narrower undefined without a tree |
| `TestContestedNewerEdit` | both edited: the newer is contested | the older |
| `TestContestedOneEditorIsNotPaused` | only `ag-a1` edited, `ag-a1` has right of way: `ag-b1` is contested | pausing the only editor |
| `TestAdvisoryNeverRefusesWrite` | the package exports no function that returns "write denied"; `check` on a contested path still `ok:true` | a lease that blocks (P7) |

```bash
cd control-plane && go test ./internal/fleet/leases/... ./internal/fleet/verbs/... 2>&1 | head -3
```

Expected: `undefined: New` in `leases` (build failure).

- [ ] **Step 2: `rightofway.go` and `overlap.go` first**

Both are pure functions over values, so their tests pass with no store. Write `globsMeet` over
path **segments**, never raw string prefixes: `auth/**` and `authz/**` share the string prefix
`auth` and are apart. The literal prefix of a glob is its leading segments up to the first that
contains any of `*?[{`.

- [ ] **Step 3: `leases.go`**

`Request` in order: shape (`Mode`, `Task` for intent, 1 to `MaxGlobs` globs); `ValidGlob` each;
`Tracked(ctx, r.Owner, r.Project)`, then either `teamtext.CheckSet(r.Globs, tracked)` or, on
`ErrNoTree`, the leading-wildcard refusal; read `LiveLeases` inside the `Tx`; compute overlaps;
choose `granted` or `contested`; `PutLease` with `Seq` 2; `outbox.Enqueue` twice (`requested`
seq 1, then the outcome seq 2). The overlap read and the write share one transaction, so two
concurrent requests on one path cannot both come out `granted`: `pgstore` takes a per-project
advisory transaction lock (`pg_advisory_xact_lock(hashtext(org||project))`) at the top of the
`Tx`; `memstore` a mutex. A test in `storetest.Suite` is **not** added here (task 10 owns the
suite); `TestConcurrentRequestsOneGranted` runs 20 goroutines against `memstore` here, and task 10's
suite is asked, in the return, to carry the same case for `pgstore`.

Expiry of a hold is `Now() + HoldTTL` at request time. An intent's `ExpiresAt` is the zero time,
sent to the Hub as `null`: an intent lives until its task ends, and task 24 ends it.

- [ ] **Step 4: `verbs/lease.go`**

Register `lease` for both classes. The handler builds `leases.Req` from `control.Caller` only,
calls the engine, cleans every string it returns through task 22's `out`, computes `Way` per
overlap with `RightOfWay`, then calls `Deps.OnLease` (task 25) when non-nil. Workers may not ask
for `intent`; that refusal is here, not in the engine, because it is a class rule (task 09's verb
table allows `lease` for both classes).

- [ ] **Step 5: Acceptance**

```bash
cd control-plane && go vet ./internal/fleet/... && go test ./internal/fleet/leases/... ./internal/fleet/verbs/... -count=1 -race
```

Expected: `ok` for both, with `-race` clean on `TestConcurrentRequestsOneGranted`. Then the
advisory guard, which must print nothing (no lease path returns a write refusal):

```bash
cd control-plane && grep -rn "deny\|forbidWrite\|refuseWrite" internal/fleet/leases/*.go
```

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/fleet/leases control-plane/internal/fleet/verbs/lease.go \
  control-plane/internal/fleet/verbs/lease_test.go testdata/panacea-control/v1/*lease*.json
git commit -F <msgfile> -- control-plane/internal/fleet/leases control-plane/internal/fleet/verbs \
  testdata/panacea-control/v1
```

**What this task does NOT claim.**
- Holds from observed edits, `bulk`, or per-path expiry. Those need `machined`'s sweep and are
  task 35's; the table rows they use are here so the table test covers them.
- The 10% cap in TM1-α. Without a tree it is replaced by the leading-wildcard refusal, which is
  weaker: `src/**` in a repo that is all `src/` passes. Task 33 records it; task 34 ends it.
- That an overlap becomes a conflict. That is task 25; this task only returns the typed overlap.
- That `globsMeet` is exact. It may report two globs as meeting when no real file satisfies both.
  It never misses a shared file; with the tree (task 34 on) the `tree` basis is exact.
- That anyone obeys a lease. Leases are advisory (P7); the worker's managed block (task 50) and
  the reviewed land (task 49) are what make a contested edit visible.
