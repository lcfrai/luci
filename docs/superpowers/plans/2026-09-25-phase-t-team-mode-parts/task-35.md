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
- `control-plane/internal/api/presence.go` — the local control plane's `PUT /api/team/presence`
  and its 30 s relay to Fleet with the user's own token (new; the desktop's one call is below)
- `desktop/src/api/client.ts` — `putPresence(projectIds)` on project open and on the one global
  heartbeat tick (modified; one function)

**Interfaces:**
- Produces:
  - `activity.Table` with `Fold(ctx, machine string, page machineapi.ActivityPage) ([]Edit,
    error)`, `Of(ctx, agent model.AgentID) (verbs.Observed, bool)`, `LastEdit(agent
    model.AgentID, path string) (time.Time, bool)`
  - `activity.Edit{Org, Project, Agent, Owner, Path, At, Commit bool}`
  - `leases.Engine.Observe(ctx, edits []activity.Edit) (ObserveResult, error)`, `Bulk`, `ExpireDue(ctx,
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
  `task-20.md:538-540`).
- Requires of **task 44**: the Hub route that records an admin's release and its feed kind
  `lease.admin_released` (G35-3).
