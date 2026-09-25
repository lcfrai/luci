### Task 64: "Move my lanes to the cloud"

**Slice TM2. Depends on: 21, P3-24. About 2.5 hours. Seam: none new. Not costed in
`05-design.md` §12; this plan places it in TM2 and says so.**

When a project is shared, every new session launches in Fleet, and a session still running on
the laptop keeps working, labelled *outside the team* (`05-design.md:1044-1045`, task 21). Work
the owner already has in laptop lanes stays there. §10.1 item 4 (`:1046-1048`): *"Unpushed laptop
lanes come along on request. 'Move my 2 lanes to the cloud' exports each as a bundle (phase 3
ruling R4) into the owner's cloud machine, where Fleet takes them over. Nothing moves without
that click."* This task is that request.

Three properties, each pinned by a test.

1. **Nothing moves without the call.** No timer, share event, sign-in or launch moves a lane. The
   only code path that exports a lane to the cloud is `POST …/team/lanes-move`, and the desktop
   calls it only from the button.
2. **A lane already on the machine is not duplicated.** The machine's lane index is read first;
   a lane with the same worktree id is updated fast-forward only, or skipped, never imported a
   second time.
3. **Lane work crosses only as a verified bundle** (phase 3 ruling R4). The laptop's host git
   reads the lane from the host-private project repository, never from the lane clone; the
   machine verifies the bundle before it creates anything. No host path, config key or URL
   crosses (`04-enterprise-constraints.md:460-463`).

Five decisions the scope left open, made here.

**(a) What "unpushed" means.** A laptop lane is a candidate when its synced tip (phase 3 task
07's `SyncLane`, which already brings the lane into the project repository as a bundle through
task 24's runner) has commits the product branch's upstream does not reach. A lane with nothing
ahead is not offered: its work is already on the remote, and the cloud machine fetches it with the
project's read-only credential (task 48).

**(b) Only committed work moves.** A bundle carries commits. A lane with uncommitted changes is
listed with the count (*"3 files not committed stay on this laptop"*) and moves only if the owner
ticks *Move the committed work* for it (`leaveUncommitted: true`); otherwise it is skipped with
`lane_uncommitted`. Nothing on the laptop is discarded either way.

**(c) A lane with a live local session is not moved.** Two agents on two copies of one lane
would diverge at once. The candidate list says *"a session is running in it"* and the move
skips it with `lane_busy`; the owner stops the session and asks again.

**(d) The laptop copy is kept, and frozen.** After a lane moves, phase 3's lane index marks it
`movedAt` and `movedTo` (the machine id), and a new local session in that lane is refused with
`lane_moved_to_cloud` and a link to the cloud lane. The laptop copy is not deleted: deleting an
owner's work is a destructive action this task does not take on her behalf. She may remove the
worktree later with phase 3 task 11's remove, which already asks.

**(e) "Fleet takes it over" is the next launch.** The moved lane becomes a lane on the owner's
cloud machine like any other: `machined`'s sweep sees it (task 34), its first commit there is
exported to the mirror (task 60), and the desktop offers **Continue in the cloud** per moved
lane, which is task 21's team launch with that `worktreeId`, creating the agent record (task 06)
whose `laneRef` names it. Until that launch there is no agent on it, so no lease and no
presence; the lane appears in teammates' views only once an agent is running on it (task 63).

**Why the duplicate check needs a marker.** Phase 3's lane id is `project.WorktreeID(Path)`, `w-`
plus 8 hex, **new per creation** (phase 3 `parts/task-03.md:61-69`), and a cloud clone has a
different path. So the id cannot say "this lane was already moved". Each lane record gains
`MovedFrom` (on the cloud lane: the laptop lane's id) and `MovedTo` (on the laptop lane: the
cloud lane's id), and the check is `MovedFrom == laptop id` among the cloud project's lanes.

**Files:**
- `control-plane/internal/api/lanesmove.go` + `lanesmove_test.go` — `LanesMover`, the two routes
  (new; the file map's row)
- `control-plane/internal/lanes/lanes.go`, `create.go` (phase 3 task 03) — `Lane.MovedFrom`,
  `MovedTo`, `MovedAt`; `CreateRequest.StartAt`, `MovedFrom`; `Store.MarkMoved` (modify;
  **UNVERIFIED until merge**)
- `control-plane/internal/api/sessions.go` (phase 2) or phase 3 task 05's lane session path —
  a local session in a moved lane → `409 lane_moved_to_cloud` (modify; one check, the site named
  at build time since phase 3's session path is unbuilt)
- `control-plane/cmd/machined/bundlein.go` (phase 8 task 21, planned) — `POST /project/v1/bundle-in`:
  a bundle into the machine's project, verified and fetched under §HGH (new on the sibling's
  code after its merge; **UNVERIFIED**)
- `control-plane/internal/api/routes_audit_test.go` (phase 3) — the two routes' rows (modify)
- `desktop/src/components/team/MoveLanes.tsx` + `MoveLanes.test.tsx` (new); `desktop/src/api/team.ts`,
  `store/types.ts` (task 52) — `laneMoveCandidates`, `moveLanes` (modify)

**Interfaces:**
- Produces: `api.LanesMover`; `GET` and `POST /v1/projects/{projectId}/team/lanes-move`;
  `lanes.Lane.MovedFrom`/`MovedTo`/`MovedAt`; `machined`'s `bundle-in`.
- Consumes: phase 3 task 07's `Syncer.SyncLane`; task 24's `lanegit.Lanes.Live` (a live session
  in the lane) and `Runner.Fetch` (a fast-forward into an existing cloud lane, R4.5); phase 3
  task 03's `lanes.Creator` and `Store`; phase 8's cloud `gitx.Machine` (S3-P8.1: project git on
  `machined` through the broker) and its lanes catalog; task 52's `TeamStrip.Team`.

```go
// control-plane/internal/api/lanesmove.go
package api

// MoveCandidate is one laptop lane the owner could move.
type MoveCandidate struct {
	WorktreeID  string `json:"worktreeId"`  // the laptop lane's w-…
	Name        string `json:"name"`
	Branch      string `json:"branch"`
	Tip         string `json:"tip"`          // after SyncLane
	Ahead       int    `json:"ahead"`        // commits the branch's upstream does not reach; ≥ 1
	Uncommitted int    `json:"uncommitted"`  // paths changed and not committed
	LiveSession bool   `json:"liveSession"`
	OnMachine   string `json:"onMachine"`    // "none" | "same" | "behind" | "diverged"
	CloudLaneID string `json:"cloudLaneId"`  // "" when none
}

type MoveCandidates struct {
	ProjectID string          `json:"projectId"`
	MachineID string          `json:"machineId"` // the owner's cloud machine
	Lanes     []MoveCandidate `json:"lanes"`     // [] never null; Ahead ≥ 1 only (decision a)
}

type MoveRequest struct {
	WorktreeIDs      []string `json:"worktreeIds"`      // 1..20 laptop lane ids, each a candidate
	LeaveUncommitted []string `json:"leaveUncommitted"` // ids whose committed work alone moves (decision b)
}

type MoveOutcome struct {
	WorktreeID  string `json:"worktreeId"`
	CloudLaneID string `json:"cloudLaneId,omitempty"`
	Result      string `json:"result"` // "moved" | "updated" | "skipped"
	Reason      string `json:"reason,omitempty"` // skipped: a code below
	Tip         string `json:"tip,omitempty"`
}

type MoveResult struct {
	Outcomes []MoveOutcome `json:"outcomes"` // one per requested id, in request order
}

const (
	CodeNotTeam          = "not_a_team_project"
	CodeLaneBusy         = "lane_busy"           // a live local session (decision c)
	CodeLaneBusyInCloud  = "lane_busy_in_cloud"  // "behind", and a cloud session is live in it
	CodeLaneUncommitted  = "lane_uncommitted"    // decision b, not ticked
	CodeLaneDiverged     = "lane_diverged"       // the cloud copy has commits the laptop lacks
	CodeAlreadyMoved     = "already_moved"       // "same": nothing to do
	CodeNothingAhead     = "nothing_ahead"
	CodeLaneMovedToCloud = "lane_moved_to_cloud" // a local launch into a moved lane
	CodeNoCloudMachine   = "no_cloud_machine"
)

// LanesMover is wired in cmd/api when the Hub, fleetbe and a cloud backend are
// all configured; otherwise both routes answer 404, as phase 2 does for any
// unwired route.
type LanesMover struct {
	Local    LocalLanes // the laptop: phase 3's Syncer, lanes.Store, lanegit.Lanes, the local Machine
	Cloud    CloudLanes // the owner's cloud machine through S8.1's backend
	IsTeam   func(ctx context.Context, projectID string) (bool, error) // task 52's TeamStrip.Team
	Now      func() time.Time
}

type LocalLanes interface {
	Candidates(ctx context.Context, projectID string) ([]lanes.Lane, error)
	Sync(ctx context.Context, lane lanes.Lane) (tip string, err error)                  // task 07's SyncLane
	Ahead(ctx context.Context, lane lanes.Lane, tip string) (ahead, uncommitted int, err error)
	Live(ctx context.Context, lane lanes.Lane) (bool, error)                            // task 24's Lanes.Live
	// Bundle runs host git in the laptop's PROJECT repository (§HGH, Scope Project),
	// never in the lane clone: git bundle create - <lane ref> ^<each basis sha>.
	Bundle(ctx context.Context, lane lanes.Lane, basis []string, w io.Writer) (int64, error)
	MarkMoved(ctx context.Context, laneID, cloudLaneID string, at time.Time) error
}

type CloudLanes interface {
	MachineID(ctx context.Context) (string, error)                                    // no_cloud_machine
	ByMovedFrom(ctx context.Context, projectID, laptopLaneID string) (lanes.Lane, bool, error)
	Basis(ctx context.Context, projectID, branch string) ([]string, error)             // shas its project holds
	Relation(ctx context.Context, projectID, cloudTip, laptopTip string) (string, error) // same | behind | diverged
	BundleIn(ctx context.Context, projectID, ref string, bundle io.Reader) (tip string, err error)
	Create(ctx context.Context, req lanes.CreateRequest) (lanes.Lane, error)          // phase 3's Creator on the cloud Machine
	FastForward(ctx context.Context, cloudLane lanes.Lane, bundle io.Reader, tip string) error // task 24's Runner.Fetch, R4.5
	Live(ctx context.Context, cloudLane lanes.Lane) (bool, error)
}

// Candidates is GET …/team/lanes-move. It reads; it moves nothing.
func (m *LanesMover) Candidates(ctx context.Context, projectID string) (MoveCandidates, error)

// Move is POST …/team/lanes-move, the owner's click, and the ONLY caller of
// Local.Bundle and Cloud.BundleIn in the module (TestOnlyMoveExportsALane).
func (m *LanesMover) Move(ctx context.Context, projectID string, req MoveRequest) (MoveResult, error)
```

```go
// control-plane/internal/lanes (phase 3 task 03, modify): the fields added.
type Lane struct {
	// … phase 3's fields, unchanged …
	MovedFrom string    `json:"movedFrom,omitempty"` // cloud lane: the laptop lane id it came from
	MovedTo   string    `json:"movedTo,omitempty"`   // laptop lane: the cloud lane id
	MovedAt   time.Time `json:"movedAt,omitempty"`
}

type CreateRequest struct {
	// … phase 3's fields, unchanged …
	StartAt   string // "" = the branch tip (phase 3's behaviour); else a sha the project holds
	MovedFrom string
}

// MarkMoved sets MovedTo and MovedAt on one lane, through WriteAtomic like every mutation.
// Store gains: MarkMoved(id, movedTo string, at time.Time) error
```

**`Move`, per lane, in request order.** Each lane is independent: one lane's failure is its
outcome, never the whole call's (the answer is 200 with every outcome).

1. `IsTeam` false → the whole call `409 not_a_team_project`. `Cloud.MachineID` fails →
   `409 no_cloud_machine`.
2. The id is a laptop lane of this project with `MovedTo == ""`; else `skipped`,
   `already_moved` (moved before) or `404`-class `bad_request` (unknown id).
3. `Local.Live` → `lane_busy`. `Local.Sync` → tip. `Local.Ahead` → 0 → `nothing_ahead`;
   uncommitted > 0 and the id not in `LeaveUncommitted` → `lane_uncommitted`.
4. `Cloud.ByMovedFrom`. Found: `Relation(cloudTip, tip)` → `same` → `skipped`, `already_moved`,
   and `Local.MarkMoved` if the laptop lane was not marked (a crash between steps 6 and 7 last
   time); `diverged` → `lane_diverged`; `behind` and `Cloud.Live` → `lane_busy_in_cloud`;
   `behind` → `Local.Bundle(basis = [cloudTip])` into `Cloud.FastForward` → `updated`.
5. Not found: `Cloud.Basis(branch)` → `Local.Bundle(basis)` → `Cloud.BundleIn(ref
   refs/panacea/moved/<laptop id>)` → the tip it reports must equal `tip`, else `git_failed`.
6. `Cloud.Create{Branch, Name: the laptop lane's name, or phase 3's derived one if taken,
   StartAt: tip, MovedFrom: laptop id}` → the cloud lane.
7. `Local.MarkMoved(laptop id, cloud lane id, now)` → `moved`.

Step 4's recovery makes the call idempotent: asking twice, or after a crash, never creates a
second cloud lane. `BundleIn` on `machined` (phase 8 task 21's project half, planned): the body
is a bundle of at most task 24's bundle limit, written to the project's private temp directory,
`git bundle verify`, then fetched into the one ref named under `refs/panacea/moved/`, all as
project calls under §HGH with P8-D7's `Env` allowlist; the ref name is checked by
`check-ref-format` and the prefix, never taken as a path.
