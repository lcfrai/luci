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

```go
// control-plane/internal/fleet/verbs/lanes.go
package verbs

// TeamLanes is GET /v1/projects/{p}/team/lanes (VerbTeamRead). Every live
// worker of the project with a lane, the caller's own included (flagged You),
// so the desktop can label its own lanes from the same source.
type TeamLanes struct {
	V         int        `json:"v"` // 1
	ProjectID string     `json:"projectId"`
	Branch    string     `json:"branch"` // the integration branch the counts are against
	Lanes     []TeamLane `json:"lanes"`  // own first, then by owner display name, then worktree id
	ReadAt    string     `json:"readAt"`
}

type TeamLane struct {
	Owner      Person            `json:"owner"`      // from the agent record's owner, never git
	You        bool              `json:"you"`
	AgentID    model.AgentID     `json:"agentId"`
	WorktreeID string            `json:"worktreeId"` // LaneRef's last segment (phase 3 lane id); the owner-hash segment is never read
	LaneRef    string            `json:"laneRef"`
	Tip        string            `json:"tip"`        // the mirror's, as of ReadAt
	MergeBase  string            `json:"mergeBase"`
	Paths      []mirror.LaneStat `json:"paths"`      // [] for the caller's own: the desktop has better
	Added      int               `json:"added"`      // totals over Paths, binaries excluded
	Deleted    int               `json:"deleted"`
	Truncated  bool              `json:"truncated"`
	More       int               `json:"more"`
	Conflicts  []LaneConflict    `json:"conflicts"`  // tier-M cf-… this lane shares with one of the caller's
	Queued     *LaneQueued       `json:"queued"`     // task 61's entry, when live; null otherwise
	Unread     string            `json:"unread"`     // "" | "mirror_unavailable" | "lane_moved": counts withheld
}

type Person struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"` // People.DisplayName, teamtext.CleanName'd; "a teammate" if unknown
}

type LaneConflict struct {
	ID    string   `json:"id"`    // cf-…
	Tier  string   `json:"tier"`  // "M" only in this read
	Paths []string `json:"paths"`
	With  model.AgentID `json:"with"` // the caller's agent on the other side
}

type LaneQueued struct {
	Entry string `json:"entry"` // lq-…
	State string `json:"state"` // task 61's State
}

// LanesSource is what the read needs beyond task 22's Sources.
type LanesSource interface {
	LaneRef(ctx context.Context, org string, agent model.AgentID) (string, error) // "" when none
	Stats(ctx context.Context, project, laneRef, branch string) (mirror.LaneStats, error) // cached by tip
	Queued(ctx context.Context, org, project string, agent model.AgentID) (*LaneQueued, error)
}

// Lanes builds TeamLanes for principal p. Owner is Sources.People.DisplayName
// of the agent's Owner. The caller's own lanes carry no Paths (the desktop
// reads its own lanes through phase 3), so no stats call is made for them.
func Lanes(ctx context.Context, src Sources, ls LanesSource, org, project, callerUserID string) (TeamLanes, error)
```

`Stats` caches one `LaneStats` per `(project, laneRef, tip)` for 10 minutes, so a poll every 5 s
(task 52's tick) costs one mirror call per lane per new commit, not per poll. The mirror being
down is `Unread: "mirror_unavailable"` on each teammate lane, not a failed read: owners and
queue state still render.

`model.Agent` has no lane ref today (`task-09.md:131-145`); the Hub's agent record does
(`task-03.md:560`). `LaneRef` reads it through task 11's rebuilt cache of Hub agents. If task 11
does not keep `laneRef`, this task adds the field to Fleet's cache, not to `model.Agent` (gap 1).

**The local route** (`api/team.go`, task 52's pattern): `GET
/v1/projects/{projectId}/team/lanes`, the signed-in user's token through `fleetbe`, the body
relayed as ST.8's `TeamLanes`, byte for byte. The desktop draws *"You"* from the `you` flag, as
task 52's `PersonRow.You` does; `labelFor` (`task-52.md:741`) is for texts and is not used here. A solo project (`TeamStrip.Team == false`)
answers `{"v":1,"projectId":…,"lanes":[]}` without calling Fleet.

**The refusal on the local diff route** (phase 3 task 16's `GET …/changes/diff`): a
`worktreeId` that is not a lane of this machine's project (phase 3's `lanes.Store`) is `404
lane_not_local`, before any git runs. Phase 3 already refuses an unknown id; this names the case a
desktop bug would hit, so the test can pin it.

**The desktop half — complete types and the join.**

```ts
// desktop/src/store/types.ts (modify): the team block, field for field with verbs.TeamLanes.
export interface LaneStat { path: string; status: "A" | "M" | "D" | "T"; added: number; deleted: number; binary: boolean }
export interface TeamLaneConflict { id: string; tier: "M"; paths: string[]; with: string }
export interface TeamLane {
  owner: PersonRef; you: boolean; agentId: string; worktreeId: WorktreeId; laneRef: string;
  tip: string; mergeBase: string; paths: LaneStat[]; added: number; deleted: number;
  truncated: boolean; more: number; conflicts: TeamLaneConflict[];
  queued: { entry: string; state: string } | null;
  unread: "" | "mirror_unavailable" | "lane_moved";
}
export interface TeamLanes { v: 1; projectId: ProjectId; branch: string; lanes: TeamLane[]; readAt: string }
```

```ts
// desktop/src/components/lens/files/model.ts (modify)

/** A teammate's touch: counts only. `kind` tells PathDiff never to ask for hunks. */
export interface RemoteTouch {
  kind: "remote"; worktreeId: WorktreeId; owner: PersonRef;
  status: LaneStat["status"]; added: number; deleted: number; binary: boolean;
}
export interface LocalTouch extends Touch { kind: "local"; owner: PersonRef | null } // null: solo
export type AnyTouch = LocalTouch | RemoteTouch;
export interface TeamPathRow extends Omit<PathRow, "touches"> { touches: AnyTouch[] }

/**
 * withTeammates joins phase 3's Changes (the viewer's own clones) with TeamLanes.
 * Own touches get the viewer's PersonRef; every teammate lane path becomes a RemoteTouch
 * on its row (a new row when only teammates touch it). A path the viewer and a teammate
 * both touch is marked "conflicts" iff a TeamLaneConflict lists it, else "both-edit"
 * (decision c); phase 3's own "conflicts" between two of the viewer's clones is kept.
 * Pure; sorted by path; lanes with unread !== "" contribute no paths.
 */
export function withTeammates(changes: Changes, team: TeamLanes | null, me: PersonRef): TeamPathRow[];
```

**What the lenses draw.**

- **Files, the path list** (`PathList.tsx`): each touch shows its owner's name before the worktree
  name (*"Alice Chen · auth-refresh"*, *"You · export"*), ids in Commit Mono (OD-28), counts as
  `+12 −3` in `tabular-nums`. A row only teammates touch is shown, marked neither `conflicts` nor
  `both-edit` unless two of them overlap (the same rule over their touches).
- **Files, one path** (`PathDiff.tsx`): a `RemoteTouch` renders one line, *"Alice Chen's lane
  changes this file: 12 lines added, 3 removed. Its content stays in her lane."*, and no hunk
  area. `PathDiff` calls phase 3's diff client for `kind: "local"` touches only.
- **Merge, a lane card** (`LaneCard.tsx`): on a team project, the owner label (*"You"*) heads the
  card and commit rows show subject, sha and time, **not** `authorName`. Solo: unchanged.
- **Merge, teammates' lanes** (`TeammateLanes.tsx`, under phase 3's queue): one row per teammate
  lane: owner, worktree id, `N paths · +A −D`, *"queued"* with its state when `queued` is set,
  and each tier-M conflict with one of the viewer's lanes as *"conflicts with your <wt> on 2
  files"*. No action on another person's lane: rows are read-only (OD-16).

- [ ] **Step 1: Write the tests, each with the mutant it kills**

Go tests on task 22's fake `Sources` and a fake `LanesSource`; the mirror half on real git in task
59's harness; desktop tests with Vitest and Testing Library, a spy on the phase 3 diff client.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSameGitAuthorDifferentPrincipalsShowTwoNames` (Go) | two teammate lanes whose commits both carry author `Panacea Agent <agent@x>`, agents owned by Alice and Priya: two `TeamLane`s, owners *Alice Chen* and *Priya Rao* | owner derived from git author or committer |
| `TestOwnerHashIsNotIdentity` (Go) | a lane ref whose `<owner-hash>` segment is Priya's hash, on an agent owned by Alice: owner Alice | the ref segment parsed as the owner |
| `TestOwnLanesCarryNoStats` (Go) | the caller's two lanes: `you: true`, `paths: []`, and the fake `Stats` saw no call for them | teammates' path fetch applied to own lanes (double counting) |
| `TestTeamLanesHasNoContentField` (Go) | reflection over `TeamLanes`, `TeamLane`, `mirror.LaneStat`: every field is in the allowlist (path, status, counts, ids, shas, names, states); a `string` field named like `line`, `text`, `hunk`, `patch`, `content` fails | a content field added later |
| `TestMirrorDownIsUnreadNotAnError` (Go) | `Stats` errors for one lane: 200, that lane `unread: "mirror_unavailable"`, `paths: []`, owner and `queued` still set | one lane failing the whole read |
| `TestMirrorLaneStatsNumstat` (sandboxd, real git) | a lane adding 12 lines to `a.go`, deleting `b.txt`, adding a PNG: `a.go M 12 0`, `b.txt D 0 n`, `x.png A binary`; the op's output bytes contain none of the files' lines | content leaking through the op; binary as zero |
| `TestDiffRouteRefusesATeammatesLane` (Go, phase 3's handler) | `GET …/changes/diff?worktreeId=<a teammate's wt>` → 404 `lane_not_local`, and task 24's spy saw no git | a teammate's id served by a local lookup |
| `withTeammates sets owners and marks by Hub conflict only` (TS) | own `a.go` + Alice's `a.go`, no conflict → `both-edit`; with a tier-M conflict listing `a.go` → `conflicts`; Alice-only `c.go` → a row with one remote touch | a local "conflicts" guessed across people |
| `PathDiff never fetches a teammate's hunks` (TS) | selecting a path whose touches are one local and one remote: the diff client is called once, for the local worktree; the remote renders the counts sentence | fetching remote content for display |
| `LaneCard hides authorName on a team project` (TS) | team: no element contains the commit's `authorName`; the owner label reads *You*; solo: `authorName` renders as before | git author shown as identity |
| `TeammateLanes rows are read-only` (TS) | no button, link or menu in a teammate row; `queued` renders its state; counts use `tabular-nums` | an action on another person's lane |

- [ ] **Step 2: Run the tests and watch them fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/verbs/ ./internal/api/ -run "Lanes|GitAuthor|OwnerHash|Teammate|NoContent|Unread"
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/desktop && npm test -- lens/files lens/merge
```

Expected: `undefined: TeamLanes`, `undefined: Lanes`; Vitest fails to import `withTeammates`.

- [ ] **Step 3: `LaneStats` in the mirror (op and client); its real-git test.**
- [ ] **Step 4: `verbs/lanes.go`, the Fleet route, the local relay, the diff refusal.**
- [ ] **Step 5: The TS types, `teamLanes()`, the slice; `withTeammates`; then the four views.**
  Read `docs/DESIGN-SYSTEM.md` and `docs/AGENTIC-UX.md` first; the approved prototype (task 51)
  is the visual reference for owner labels. No percentages, bars or projections (OD-27).
- [ ] **Step 6: Run everything**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./... && GOPROXY=off go vet ./... && gofmt -l internal
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/sandboxd && GOFLAGS=-mod=vendor go test ./cmd/panacea-mirror/... && GOFLAGS=-mod=vendor go vet ./...
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/desktop && npm run typecheck && npm test
```

Expected: every package `ok`; phase 3's lens tests pass unedited except `LaneCard`'s team case;
typecheck clean.

- [ ] **Step 7: Commit**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && git add control-plane/internal/fleet/verbs/lanes.go control-plane/internal/fleet/verbs/lanes_test.go control-plane/internal/fleet/server/lanes.go control-plane/internal/fleet/server/lanes_test.go desktop/src/components/lens/merge/TeammateLanes.tsx desktop/src/components/lens/merge/TeammateLanes.test.tsx && git commit -F .superpowers/sdd/team/msg/task-63.txt -- control-plane sandboxd/cmd/panacea-mirror desktop/src
```

Visual acceptance is the owner's eyes (`CLAUDE.md` working agreements); the two-person check that
a teammate's lane shows her name and counts on the real environment is task 69's.

**What this task does not claim:**

- **That the Hub principal proves who wrote the code.** It proves whose agent published the lane.
  A person can still paste someone else's code; signed commits are not proposed for v1 (`04`
  §7.3).
- **Line counts as a size of risk.** `+12 −3` is `numstat`; a one-line change can matter more.
- **Phase 3's landed history** (task 20) by principal. Lands through task 61 carry
  `Panacea-Owner`, and history could read it; that is not in this task's scope (gap 2).

**Gaps against the skeleton, raised here:**

1. **Fleet does not keep a lane ref per agent.** `model.Agent` has none (`task-09.md:131-145`);
   the Hub record does. This task reads it through task 11's cache and adds it there if absent.
2. **History by principal.** The scope names tasks 16, 19 and 21, not 20; landed history keeps
   `authorName` until a task reads task 61's `Panacea-Owner` trailer.
3. **The design names no route for teammates' lanes.** `GET /v1/projects/{p}/team/lanes` is this
   part's; task 01's contract is not involved (Fleet state and mirror facts, not Hub records).
