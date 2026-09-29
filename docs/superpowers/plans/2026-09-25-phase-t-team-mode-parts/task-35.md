### Task 35: Activity and presence; holds from observed edits, bulk, per-path expiry, releases

**Slice:** TM1. **Focused time:** 3 h. **dependsOn:** 23 (the lease engine), 34 (ST.6 samples).
**Seam consumed:** ST.6 (`machineapi.ActivitySample`, `task-34.md`).

TM1-α's leases are only the ones agents ask for. §6.3 says that is not enough: a worker that edits
without asking must still be visible, and a claim must follow the work, not the request. This task
turns `machined`'s samples into three things Fleet keeps. **Activity**: what each worker is
changing now, in memory, 45-second TTL, the only state besides presence that is not in Postgres
(global constraint 1). **Holds**: on the first observed edit to a path, the worker gets a `hold`
lease on that path **whether or not it asked**; a hold lapses 30 minutes after the last edit **to
that path**. **Bulk**: more than 25 files in one minute becomes one flagged `bulk` lease with no
right of way, reported to the owner's Orchestrator. And **presence**: whether each person's cockpit
is online and which project it has open, from the desktop.

Every lease still ends when its task or agent ends (task 23's `EndTask`, `EndAgent`), when its
delegation lapses (task 11) or when its owner leaves (task 44); this task adds the observed side of
that and a test that all four hold together, not new release paths for them.

**Decisions made here.**

**(a) An observed hold is one lease per path.** Leases are glob lists and expiry is per lease
(`model.Lease.ExpiresAt`, `task-09.md:185-199`). "A hold lapses 30 minutes after the last edit to
that path" is exact only if the path *is* the lease, so an observed edit to an unleased path makes
a lease whose one glob is that literal path. A path already inside one of the worker's live leases
(its own intent, or a hold it asked for) makes no new lease; the edit is recorded on that lease's
path table (task 10's `TouchLeasePath`, `task-10.md` `Tx` interface).

**(b) An asked-for hold with globs expires by its paths.** Its `ExpiresAt` becomes `max(requested
at, the newest edit to any path it matches) + 30 min`, recomputed on each touch. An edit to a path
its globs do not match never moves it, which is the skeleton's "editing another path does not
extend a hold". An `intent` never expires by time (task 23: zero `ExpiresAt`); it ends with its task.

**(c) An edit is a change between two samples.** A path counts as edited at a sample's `At` when it
is in `Dirty` and was not in the worker's previous sample, or its `Added`/`Deleted` counts or its
hunk ranges changed; and every path of a **new** `LastCommit` counts as edited at that commit's
time. A dirty path that sits unchanged is not re-edited every sweep, so it expires.

**(d) After a restart, the first page is a baseline.** A page with `Gap` (task 34), or the first
page since start, sets each worktree's previous sample without counting edits, except that a dirty
path no live lease covers gets a hold at once, expiring 30 minutes from now. Holds live in Postgres
and survive the restart; the bulk window and the activity table are rebuilt from the page, which
holds two minutes of samples: "rebuilt within one sweep".

**(e) Bulk is observed, never requested, and its globs are collapsed.** Task 23 refuses `bulk` on
`Request` (`task-23.md:106`). When one worker's edits reach 26 distinct paths within a sliding 60
seconds, Fleet writes one `bulk` lease for that worker whose globs are those paths collapsed to at
most `teamtext.MaxGlobs` (20) directory globs (below), skipping task 23's breadth cap, because
right of way already ranks bulk last (`ReasonNotBulk`, `task-23.md:224`, `:236`). The worker's
per-path holds from that window are released into it in the same transaction, so a formatter run
leaves one lease, not forty. It expires like an asked-for hold (decision b).

**(f) An admin's release is the admin's own Hub write.** §6.3 lets a team admin release a stale
lease, audited. Fleet writes nothing in an admin's name (global constraint 3, task 14's pattern,
`task-14.md:40-61`): the admin's client posts the release **to the Hub**, which audits it with the
admin as actor and puts `lease.admin_released` on Fleet's feed; Fleet's feed consumer moves the
lease to `admin-released` (task 23's table row, `task-23.md:164`) and reevaluates. The Hub route is
task 44's (task 23 assigns the transition there); this task builds Fleet's half. **Gap G35-3.**

**Files:**
- `control-plane/internal/fleet/activity/activity.go` — `Table` (the 45 s activity table), `Fold`
  (a page into edits), `Of` (task 22's `Activity`), `LastEdit` (task 23's `Contested` input) (new)
- `control-plane/internal/fleet/activity/poller.go` — `Poller`: one broker poll per live
  (machine, team project) pair, every 5 s, under that project's delegation (new; not in the file
  map, it keeps the network out of the table)
- `control-plane/internal/fleet/activity/presence.go` — `Presence`, the 90 s TTL, `POST
  /v1/presence` (new)
- `control-plane/internal/fleet/leases/observed.go` — `Engine.Observe` (holds from edits),
  `Engine.Bulk`, `Engine.ExpireDue`, `Engine.AdminReleased`, `Collapse` (new, in task 23's
  package, so its transition-table test covers them)
- `control-plane/internal/fleet/activity/{activity,poller,presence}_test.go`,
  `control-plane/internal/fleet/leases/observed_test.go` (new)
- `control-plane/internal/fleet/model/event.go` — `EventLeaseBulk EventKind = "lease.bulk"` and
  its place in the kinds list (modified; task 09's file)
- `control-plane/internal/fleet/verbs/sources.go` — `Sources.Activity` wired (task 22 left it nil,
  `task-22.md:71`) (modified; one line in `fleetd`'s wiring, none in task 22's logic)
- `control-plane/cmd/fleetd/main.go` — start the poller on every replica, the expiry sweep on the
  deadline-sweeper singleton, the presence route (modified; task 12's file)
- `control-plane/internal/api/presence.go` — the local control plane's `PUT /v1/team/presence`
  and its 30 s relay to Fleet with the user's own token (new; the desktop's one call is below)
- `desktop/src/api/client.ts` — `putPresence(projectIds)` on project open and on the one global
  heartbeat tick (modified; one function)

**Interfaces:**
- Produces:
  - `activity.Table` with `Fold(ctx, machine string, page machineapi.ActivityPage) ([]Edit,
    error)`, `Of(ctx, agent model.AgentID) (verbs.Observed, bool)`, `LastEdit(agent
    model.AgentID, path string) (time.Time, bool)`
  - `leases.Edit{Org, Project, Agent, Owner, Path, At, Commit bool}` (aliased as `activity.Edit`)
  - `leases.Engine.Observe(ctx, edits []leases.Edit) (ObserveResult, error)`, `Bulk`, `ExpireDue(ctx,
    now) (int, error)`, `AdminReleased(ctx, org, lease string, at time.Time) error`, `Collapse(paths
    []string, max int) []string`
  - `activity.Presence` with `Put(owner model.Owner, projects []string, at time.Time)` and
    `Online(owner) (projects []string, ok bool)`; Fleet route `POST /v1/presence`
  - `model.EventLeaseBulk`
- Consumes:
  - task 34: `machineapi.ActivityPage`, `ActivitySample`, `Gap`, the broker's `GET
    /v1/machines/{m}/activity`
  - task 23: `Engine.RequestIn` (holds are requests with the observed flag below), `Live`,
    `Reevaluate`, `leaseEvent`, the table rows `expired` and `admin-released`, `HoldTTL`
  - task 10: `Tx.TouchLeasePath`, `Tx.LeasePathEdits`, `Tx.PutLease`
  - task 22: `verbs.Activity`, `verbs.Observed`, `verbs.Hunk` (`task-22.md:136-153`)
  - task 26: `turns.Queue.Enqueue` for `lease.bulk` and `lease.expiring` (to the owner's
    Orchestrator)
  - task 11: the feed consumer's hook for `lease.admin_released`; task 12: `Server.Route`,
    `Singleton`
  - task 02: `teamtext.ValidPath`, `teamtext.MaxGlobs`, `teamtext.Match`
- Requires of **task 20**: a way to map a sample's `(projectId, worktreeId)` to the worker `ag-…`
  on it, and each live worker's machine id: `sessions.Lanes(ctx, org, project) ([]Lane, error)`
  with `Lane{Agent, Owner, Machine, Worktree}` (G35-1; it follows task 20's own gap S20-3,
  `task-20.md:650-652`).
- Requires of **task 44**: the Hub route that records an admin's release and its feed kind
  `lease.admin_released` (G35-3).

#### `activity/activity.go` (complete in its surface)

```go
package activity

const (
	TTL          = 45 * time.Second // an entry with no sample for this long reads as absent
	RecentWindow = 10 * time.Minute // Observed.Files: paths edited this recently
	BulkWindow   = time.Minute
	BulkOver     = 25               // more than this many distinct paths in BulkWindow is bulk
)

// Edit is leases.Edit (below): it lives in package leases because verbs
// imports leases and this package imports verbs, so an activity.Edit that
// leases used would be an import cycle.
type Edit = leases.Edit

// Lanes is task 20's sessions.Lanes (G35-1).
type Lanes interface {
	Lanes(ctx context.Context, org, project string) ([]Lane, error)
}
type Lane struct {
	Agent    model.AgentID
	Owner    model.Owner
	Machine  string // the broker's machine id
	Worktree string // w-…
}

// Table is in memory only (global constraint 1). One per replica: every
// replica polls (poller.go) so every replica can answer team and check.
type Table struct {
	mu    sync.Mutex
	now   func() time.Time
	lanes Lanes
	last  map[laneKey]entry // the previous sample per worktree
	recent map[model.AgentID]map[string]time.Time // path -> last edit, pruned at RecentWindow
	window map[model.AgentID][]stamp                 // the bulk window's (path, at)
}

// Fold turns one page from one machine into edits, per decision (c), and
// updates the table. A sample for a worktree no Lane names is ignored (it is
// not a team worker's). Every path is checked again with teamtext.ValidPath;
// a refused one is dropped here too (two layers: machined, then Fleet).
// A page with Gap, or the first since start, is a baseline (decision d): it
// returns only the "uncovered dirty path" edits, marked At = now.
func (t *Table) Fold(ctx context.Context, org, project, machine string, page machineapi.ActivityPage) ([]Edit, error)

// Of is task 22's verbs.Activity. ok is false when the agent's last sample is
// older than TTL. LastCommitSubj is teamtext.Text with Author agent and
// AuthorAgent the worker, so its class travels with it to Orchestrators only.
func (t *Table) Of(ctx context.Context, agent model.AgentID) (verbs.Observed, bool)

// LastEdit is task 23's Contested input (task-23.md:251-253).
func (t *Table) LastEdit(agent model.AgentID, path string) (time.Time, bool)

// BulkDue returns, and clears, each agent whose window holds more than
// BulkOver distinct paths, with those paths.
func (t *Table) BulkDue() map[model.AgentID][]string
```

#### `leases/observed.go` (complete in its surface)

```go
package leases

// Edit is one observed edit, attributed to the worker whose worktree it is in.
// Owner comes from the Lane (task 20), never from the sample.
type Edit struct {
	Org, Project string
	Agent        model.AgentID
	Owner        model.Owner
	Path         string
	At           time.Time
	Commit       bool // the path is in a new LastCommit, not only dirty
}

// ObserveResult is what one fold changed, for the tests and the logs.
type ObserveResult struct {
	NewHolds []model.Lease // one per newly held path
	Touched  []string      // ls-… whose ExpiresAt moved
}

// Observe applies edits in one Tx per project: for each edit, the worker's
// live lease whose globs Match the path is touched (TouchLeasePath, then
// ExpiresAt = max(created, newest touch) + HoldTTL for a hold or bulk; an
// intent is untouched); if none matches, a hold on exactly that path is
// requested through RequestIn with Mode hold, which writes it granted or
// contested with its Hub events, like any request. The breadth check does
// not apply to a one-path hold (a literal path is never broad).
func (e *Engine) Observe(ctx context.Context, edits []Edit) (ObserveResult, error)

// Bulk writes one bulk lease for agent over Collapse(paths, teamtext.MaxGlobs)
// and releases, in the same Tx, the agent's holds from the window that the
// bulk lease's globs now cover; then enqueues lease.bulk for the owner's
// Orchestrator with the lease id and the path count (no path list: the
// Orchestrator reads it with check or team).
func (e *Engine) Bulk(ctx context.Context, org, project string, agent model.AgentID, paths []string) (model.Lease, error)

// ExpireDue moves every hold or bulk lease with ExpiresAt <= now to expired,
// one Hub event each, then Reevaluate per project; lease.expiring is enqueued
// for the holder's Orchestrator five minutes before (once per lease).
func (e *Engine) ExpireDue(ctx context.Context, now time.Time) (int, error)

// AdminReleased applies the Hub's lease.admin_released (decision f). A lease
// already terminal is left alone; the Hub's event is the audit, so Fleet
// writes no second lease event for it.
func (e *Engine) AdminReleased(ctx context.Context, org, lease string, at time.Time) error

// Collapse reduces literal paths to at most max globs: group by the deepest
// directory holding every path in the group, starting two segments deep and
// widening one segment at a time until there are at most max groups; a group
// of one path stays literal; a group becomes "<dir>/**". Never returns "**".
func Collapse(paths []string, max int) []string
```

**`Collapse` cannot always stay under 20 without reaching the root.** Twenty-one top-level
directories each with one edited file already need 21 globs; then `Collapse` keeps the 20 largest
groups and drops the rest, and the bulk lease's Hub event carries the real number of edited paths
as `trackedFiles`, so the record shows more paths than the globs cover. A bulk
lease is advisory and ranked last, so an under-covering bulk lease loses nothing a hold would have
protected: the paths it misses keep their own per-path holds.

#### The poller, and who writes

Every replica runs a `Poller`: every 5 s, for each team project with a live worker, it asks task
20's `Lanes` for the (machine, worktree, agent) triples and polls each machine once through the
broker, under that project's `worker`-class delegation of the machine's owner (task 17), with
`since` = the last `Next` it saw for that machine. Every replica folds into its own `Table`, so
`team` and `check` answer on any replica. **Only the replica holding the `fleet-activity`
singleton** (task 12's advisory lock, a fourth name beside its three) passes the edits to
`Engine.Observe` and runs `BulkDue` and `Bulk`, so two replicas never write two holds for one edit.
`ExpireDue` runs on the deadline-sweeper singleton every 30 s. A poll that fails leaves the table
as it was; after `TTL` with no sample, `Of` answers absent and `team` shows the worker's activity
as unknown, never as idle.

#### `activity/presence.go` (complete in its surface) and the route

```go
package activity

const PresenceTTL = 90 * time.Second // three missed 30 s beats

// Presence is in memory only. A person is online while their desktop's last
// beat is under PresenceTTL old; projects are the ones that beat named.
type Presence struct {
	mu   sync.Mutex
	now  func() time.Time
	seen map[model.Owner]beat
}

func (p *Presence) Put(owner model.Owner, projects []string, at time.Time)
func (p *Presence) Online(owner model.Owner) (projects []string, ok bool)
```

`POST /v1/presence` on Fleet, through task 12's `Route` with verb `presence.put` (any role that
may read the project, §3.4), body `{"projects": ["pj-…"]}`, at most 20 ids, each a project the
principal is a member of (task 12's `Membership`, cached ≤ 60 s), else 403 naming the id; `204`.
The owner is the token's principal, never a body field (global constraint 3). The local control
plane's `PUT /v1/team/presence` takes the same body from the desktop and relays it every 30 s
while the desktop keeps calling; the desktop calls it when the open project changes and on the one
global heartbeat clock's 30 s tick (`CLAUDE.md` frontend rules), never on a timer of its own.
Presence is shown by task 53's strip; this task only keeps it.

- [ ] **Step 1: Write the tests first, each with the mutant it kills**

All on task 10's `memstore`, a fake broker serving ST.6 fixtures (`testdata/activity/v1/`), a
fake `Lanes`, a fake queue and a fixed clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFormatterRunIsOneBulkLease` (the skeleton's) | 40 files dirty across two samples 10 s apart → exactly one live lease for the worker, mode `bulk`, ≤ 20 globs; no per-path hold left live; `RightOfWay` against a later one-path hold → the hold, `not-bulk`; one `lease.bulk` event queued for the owner's Orchestrator | per-path holds kept beside the bulk lease; bulk given right of way |
| `TestEditingAnotherPathDoesNotExtendAHold` (the skeleton's) | a hold on `a.go` edited at t0; `b.go` edited at t0+20 min → `a.go`'s hold `ExpiresAt == t0+30 min`, and at t0+30 min `ExpireDue` expires it | expiry measured from the worker's last edit anywhere |
| `TestFirstEditMakesAHoldUnasked` | a worker with no lease edits `x.go` → one granted hold on `["x.go"]` with two Hub events (requested, granted) | holds only for requested paths |
| `TestEditInsideOwnIntentMakesNoHold` | the worker's task intent covers `web/**`; an edit to `web/a.go` → no new lease; the path's edit time recorded | a second lease beside the intent |
| `TestUnchangedDirtyPathExpires` | a dirty path identical in every sample for 31 min → its hold expires | every sweep counted as an edit |
| `TestCommitCountsAsEdit` | a new `LastCommit` naming `y.go` → `y.go` edited at the commit's time | commits invisible once the tree is clean |
| `TestRestartBaselineRebuildsInOneSweep` | a fresh `Table` and one page with `Gap` → `Of` answers for every worker in it; no hold's `ExpiresAt` moved; an uncovered dirty path got a hold expiring now+30 min | a restart re-editing every dirty path, or losing activity until the next change |
| `TestOnlyTheSingletonWrites` | two `Table`s fold the same page; only the one with the lock calls `Observe` → one hold | duplicate holds from two replicas |
| `TestUnknownWorktreeIgnored` | a sample for a worktree no `Lane` names → no edit, no entry | a solo lane's activity entering Fleet |
| `TestFleetRechecksPaths` | a fixture sample with a bidi path (a `machined` that skipped hygiene) → dropped here too | trusting the source's hygiene alone |
| `TestActivityAbsentAfterTTL` | no sample for 46 s → `Of` ok=false | a stale picture read as current |
| `TestEveryEndReleases` | a worker with a hold, a bulk lease and an intent: `EndAgent` releases hold and bulk, `EndTask` the intent; a lapsed delegation (task 11's path) expires the rest; a `lease.admin_released` feed item moves a live lease to `admin-released` with no second Hub event | one end path forgetting observed leases |
| `TestPresenceTTLAndOwner` | a beat at t0 → online until t0+90 s; a body naming a project the principal is not in → 403; the owner is the token's | presence set from a body field; no expiry |
| `TestCollapseNeverRoot` | 40 paths under `src/` and `test/` → `["src/**", "test/**"]`-shaped, never `**`, ≤ 20 | a bulk lease on the whole repository |

Run: `cd control-plane && go test ./internal/fleet/activity/ ./internal/fleet/leases/ -run
'Formatter|AnotherPath|FirstEdit|OwnIntent|Unchanged|Commit|Restart|Singleton|Unknown|Recheck|TTL|EveryEnd|Presence|Collapse'`.
Expected: compile failure (`undefined: Table`, `undefined: Edit`).

- [ ] **Step 2:** `model.EventLeaseBulk` and its place in task 09's kinds list; task 26's queue
  treats it as non-immediate (it coalesces), which needs no change there.
- [ ] **Step 3:** `leases/observed.go`: `Edit`, `Observe`, `Bulk`, `ExpireDue`, `AdminReleased`,
  `Collapse`. Task 23's transition-table test still passes: every move here is a row it already
  lists (`task-23.md:156-164`).
- [ ] **Step 4:** `activity/activity.go` and `poller.go`; wire `Sources.Activity` and task 23's
  `Contested` `lastEdit` to `Table.LastEdit`.
- [ ] **Step 5:** `presence.go`, the Fleet route, the local relay, the desktop call.
- [ ] **Step 6: Acceptance.**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && go vet ./... && go test ./internal/fleet/... ./internal/api/... -count=1 -race
```
Expected: `ok` for every package, `activity` and `leases` among them.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/desktop && npm run typecheck && npm test
```
Expected: both pass; `putPresence` has one test that it sends the open project ids only.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && grep -rn "pgx\|INSERT\|CREATE TABLE" control-plane/internal/fleet/activity/
```
Expected: no output: activity and presence are in memory only (global constraint 1).

- [ ] **Step 7: Commit** after `git add` of the new files: `git commit -F
  .superpowers/sdd/team/msg/t35.txt -- control-plane/internal/fleet control-plane/internal/api
  control-plane/cmd/fleetd desktop/src/api/client.ts`.

**What this task does not claim:**

- **That observed holds are timely.** They lag a sweep (5 to 10 s) plus a poll (5 s), and a
  guest write's visibility to `machined` is **UNVERIFIED** (task 34, task 47). Two workers can
  both edit a path inside that window; the second one's hold is `contested`, which is what the land
  review and task 36's tier H then see.
- **That a lease stops an edit.** Leases stay advisory (P7).
- **That bulk is always a formatter.** Any 26 paths in a minute is bulk; a real refactor is
  flagged the same way, which ranks it last on right of way, and the owner's Orchestrator is told.
- **The admin's Hub route and view**: task 44 and the cockpit tasks. **Tier-H forecasts** from the
  hunks: task 36.

**Gaps against the skeleton and sibling parts, raised here:**

- **G35-1.** Fleet cannot map a sample's `(projectId, worktreeId)` to a worker, or a worker to its
  machine, without task 20 recording both; task 20's own gap S20-3 says the worker's worktree is
  not wired in TM1-α (`task-20.md:650-652`).
- **G35-2.** The skeleton lists no event kind for bulk; this task adds `lease.bulk` to task 09's
  model, and task 41's instructions must say what an Orchestrator does with it.
- **G35-3.** The skeleton gives the admin's release to task 35, task 23 gives the transition to
  task 44 (`task-23.md:164`); this task builds Fleet's half and names the Hub route as task 44's.
- **G35-4.** The desktop's presence call is a desktop change in a task the skeleton lists as
  Fleet-only; it is one function on the global heartbeat, and task 52's client should absorb it.
