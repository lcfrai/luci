### Task 63: Phase 3 delta: merge and files lenses by Hub principal; teammates' lanes from the mirror

**Slice TM2. Depends on: 60, P3-16, P3-19, P3-21. About 3 hours. Seam: ST.8 (consumed and
extended by one read).**

Phase 3's two lenses answer *what lands* (Merge, task 19) and *what is changing, and who is
changing it* (Files, task 16's read model, task 21's lens). Built for one person, they know "who"
only as a worktree name and, on each commit, git's `authorName` (phase 3 `parts/task-18.md:37-39`,
`:92`). On a team project that is not good enough, because **git author is not identity**: a
commit's author is whatever the committer's config says (`04-enterprise-constraints.md:465-469`).
This task makes both lenses name every lane's owner by **Hub principal**, and shows teammates'
lanes, which are not on the viewer's machine, from the lane mirror: **paths and line counts only**.

Every phase 3 path below is **UNVERIFIED until merge**: phase 3's parts exist on
`.claude/worktrees/phase-3`, the code does not (no `changes`, `converge` or lens code yet). This
task starts after phase 3's sync-merge into `feat/team-mode`, and after task 60 (lane bundles in
the mirror).

Three properties, each pinned by a test.

1. **A lane's owner is the principal whose agent record names it.** Fleet maps each lane ref to
   the Hub agent whose `laneRef` it is (`hubapi.Agent.LaneRef`, `task-03.md:560`), and that
   agent's `ownerUserId` to a display name. Nothing reads the git author, the committer, or the
   `<owner-hash>` segment of `refs/panacea/lanes/<owner-hash>/<wt-id>` as identity.
2. **No file content from a teammate's lane is fetched for display.** Fleet's route carries paths,
   statuses and `numstat` line counts; it has no field that can hold a line of a file. The
   desktop never asks the local control plane for a teammate's hunks, and the local diff route
   refuses a worktree that is not on the viewer's machine.
3. **Solo projects are unchanged.** With `team: false` (task 52's `TeamStrip.Team`) no Fleet call
   is made, no owner label is drawn, and phase 3's own tests pass unedited.

Four decisions the scope left open, made here.

**(a) The join is on the desktop, not in phase 3's read models.** Phase 3's `changes.Read` and
the convergence GET stay about the viewer's own clones. Teammates' lanes arrive by one new read,
through the local control plane's `fleetbe` proxy (task 21, task 52's `api/team.go`), and the lens
models merge them in pure functions (`withTeammates`, below). So `changes` and `converge` gain no
team type and no Hub import, and phase 3's tests are untouched except where a lens shows a name.

**(b) Own lanes are "You"; a teammate's are her display name.** The desktop knows its own
principal (task 52's `PersonRow.You`). An own lane's commits keep their rows, but on a team
project the row shows the lane owner's label, not `authorName`; git's string is not rendered at
all there, because rendering it beside the principal invites the reader to trust the wrong one.
Solo projects keep `authorName` (property 3).

**(c) A cross-person mark is a Hub conflict, never a local guess.** Phase 3's Files mark
`conflicts` comes from `merge-tree` over two of the viewer's clones (`parts/task-16.md:45-49`). A
path touched by the viewer and a teammate is marked `conflicts` only when a Hub `cf-…` at tier `M`
(task 60's) lists it; otherwise `both-edit`, which phase 3 already draws as a suspicion. The
desktop never runs or asks for a merge across people.

**(d) Line counts are `numstat` against the lane's merge base with the integration head**, the
same range task 60 bundles. Binary files count as `binary`, not as zero. More than 500 paths in
one lane: the first 500 by path, and `truncated: true`, drawn as *"and N more paths"*.

**Files:**
- `control-plane/internal/fleet/mirror/client.go` (task 59) — `LaneStats` (modify; task 59's file)
- `sandboxd/cmd/panacea-mirror/main.go` (task 59) — the `lane-stats` op (modify)
- `control-plane/internal/fleet/verbs/lanes.go` + `lanes_test.go` — `TeamLanes`, the read (new)
- `control-plane/internal/fleet/server/lanes.go` + test — `GET /v1/projects/{p}/team/lanes` (new)
- `control-plane/internal/api/teamview.go`, `team.go` (task 52) — the ST.8 types below and the
  local `GET /v1/projects/{projectId}/team/lanes`, relayed through `fleetbe` (modify)
- `control-plane/internal/api/changes.go` (phase 3 task 16) — the diff GET refuses a worktree
  that is not a local lane, `404 lane_not_local` (modify; one check)
- `desktop/src/store/types.ts`, `desktop/src/api/team.ts`, `store/slices/teamSlice.ts` (task 52)
  — `TeamLanes`, `teamLanes()`, the slice's `lanes` on the poll tick (modify)
- `desktop/src/components/lens/files/model.ts` (phase 3 task 21) — `withTeammates` (modify)
- `desktop/src/components/lens/files/PathList.tsx`, `PathDiff.tsx` (task 21) — owner labels; a
  teammate's touch renders counts, never a hunk (modify)
- `desktop/src/components/lens/merge/LaneCard.tsx` (task 19) — the owner label; no `authorName`
  on a team project (modify)
- `desktop/src/components/lens/merge/TeammateLanes.tsx` + test — teammates' lanes under the
  queue (new)
- `desktop/src/components/lens/files/model.test.ts`, `PathDiff.test.tsx`, `LaneCard.test.tsx`
  (modify / new)

**Interfaces:**
- Produces: `verbs.TeamLanes`; the Fleet and local `team/lanes` routes; ST.8's `TeamLanes`,
  `TeamLane`, `LaneStat`; `withTeammates`.
- Consumes: task 22's `verbs.Sources` (the agents of the project, their owners' display names);
  task 60's lanes in the mirror and its tier-M `cf-…` records; task 61's `landq` entries (queue
  position, optional); task 52's `PersonRef`, `TeamStrip.Team`, the `fleetbe` relay; phase 3's
  `changes.Changes`, `PathRow`, `Touch` (`parts/task-16.md:33-67`) and `LaneObservation`
  (`parts/task-18.md:43-47`).
- Requires of **task 59's mirror client**: `LaneStats` (below).

```go
// control-plane/internal/fleet/mirror/client.go (modify): one op.

// LaneStat is one path of a lane since its merge base with the integration
// head: git diff --numstat -z --no-renames and --name-status -z over the same
// range, joined by path. No content, ever.
type LaneStat struct {
	Path    string `json:"path"`
	Status  string `json:"status"`  // A | M | D | T
	Added   int    `json:"added"`   // 0 when Binary
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary"`  // numstat's "-\t-"
}

type LaneStats struct {
	MergeBase string     `json:"mergeBase"`
	Tip       string     `json:"tip"`
	Paths     []LaneStat `json:"paths"`     // sorted by Path, at most 500
	Truncated bool       `json:"truncated"`
	More      int        `json:"more"`      // paths beyond the 500
}

// LaneStats runs in the mirror pod under phase 3's §HGH hardening
// (--no-ext-diff, --no-textconv, no attributes-driven drivers). tip must be
// laneRef's sha, else ErrLaneMoved: a count for a tip nobody asked about is
// worse than none.
func (c *Client) LaneStats(ctx context.Context, project, laneRef, tip, branch string) (LaneStats, error)
```
