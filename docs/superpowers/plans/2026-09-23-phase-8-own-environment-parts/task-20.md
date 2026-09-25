### Task 20: `cmd/machined` — the catalog and the stores, where the files are

A cloud machine's projects, worktrees and conversation stores live on its disk (P8-D6), inside a
Kata pod the laptop cannot reach except through the broker. Rather than teach the laptop's
catalog to read a remote filesystem, **the catalog runs on the machine** (P8-D7): `machined` is a
small server in the machine pod that serves phase 2's read-only catalog from phase 2's own code —
`project.Scanner` over `project.ExecRunner` (`control-plane/internal/project/project.go:61-76`,
`gitrunner.go:21-46`) — on Linux paths, plus the three filesystem operations a session needs:
create its sub-path directories before it starts (task 15's rule), and delete a conversation or a
whole store (OD-10). Task 21 adds git.

It runs as uid 1001 in a Kata pod, so everything it creates belongs to the agent user, and a
compromise of it is contained to one user's disk and one microVM. No agent code runs in the
machine pod: lane git is not served here but by `sandboxd` in the pod that mounts the lane (env
R4 rulings, `.superpowers/sdd/env/r4-env-rulings.md` ruling 2), and nothing `machined` does
reads a lane with git.

**R4.1 in this task.** No sandbox mounts `projects/<name>` (phase-3 `orchestrator-rulings.md`
R4.1). A session on a primary or linked worktree runs in that worktree's backing lane (phase-3
`task-05.md:96-112`; env ruling 5), so the worktree path route answers **lane paths only**, and
refuses a primary or linked worktree's id with `worktree_not_a_lane`.

**Files:**
- `control-plane/cmd/machined/{main.go,config.go,server.go,catalog.go,dirs.go}` and tests (new)
- `control-plane/internal/machineapi/wire.go` — the machine agent's wire types (new)

#### Configuration (complete)

| Variable | Default | Meaning |
|---|---|---|
| `PANACEA_APP_MACHINED_ADDR` | `:8081` | pod network only; NetworkPolicy admits the broker alone (task 11) |
| `PANACEA_APP_MACHINED_TOKEN` | — **required**, ≥ 32 characters | the bearer the broker presents (from the machine's Secret, task 19) |
| `PANACEA_APP_MACHINED_ROOT` | — **required**, absolute | the disk's mount point, `/machine` |
| `PANACEA_APP_LOG_LEVEL` | `info` | |

On start it creates `projects/`, `lanes/` and `conversations/` under the root (mode `0700`) and
refuses to start if the root is not writable.

#### Contract — `machineapi/wire.go` (complete)

```go
// Package machineapi is the machine agent's wire contract, shared by
// cmd/machined and the laptop's remote catalog (task 24b).
package machineapi

import (
	"fmt"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// MachineRoot is the disk's mount point in the machine pod: PANACEA_APP_MACHINED_ROOT
// (task 19). A cloud gitx.Machine's CloneRoot is MachineRoot + "/lanes" (env ruling 4).
const MachineRoot = "/machine"

// The disk layout, relative to PANACEA_APP_MACHINED_ROOT. P8-D6.
const (
	DirProjects      = "projects"      // project repositories: projects/<name>
	DirLanes         = "lanes"         // phase 3 lane clones: lanes/<projectId>/<name>-<nonce> (S3-P8.2; phase-3 task-03.md:66)
	DirConversations = "conversations" // conversations/<worktreeId>, mounted at ~/.claude/projects
)

// The catalog envelopes carry phase 2's own types, unchanged, so the laptop can
// pass them through and the JSON the cockpit sees is byte-identical in shape.
type Projects struct {
	Projects []project.Project `json:"projects"`
}
type Branches struct {
	Branches []project.Branch `json:"branches"`
}
type Worktrees struct {
	Worktrees []project.Worktree `json:"worktrees"`
}

// WorktreePath answers GET /v1/worktrees/{worktreeId}/path?lane=<machine path>.
// MachinePath is the lane's path relative to the root, always exactly
// lanes/<projectId>/<dir> (R4.1: never projects/…) — the form
// backend.MachineMount takes (task 15). GitConfig is MachinePath + "/.git/config"
// when, read with Lstat (no symlink followed), MachinePath/.git is a directory
// and its config a regular file; "" when .git is not a directory. Task 24b binds
// it read-only over the sandbox's /home/dev/work/.git/config as hygiene: after
// phase 3's ruling R4 no host git reads a lane's config, so nothing is claimed
// from the bind (phase 3 plan :245, :712; review I7, Q18). A .git directory whose
// config is missing or is not a regular file is 409 conflict with details.reason
// "git_config_not_a_file": a Kubernetes subPath that does not exist is created
// as a directory, and one that is a symlink binds its target (Kubernetes'
// documented behaviour, UNVERIFIED here: nothing runs on a cluster).
type WorktreePath struct {
	MachinePath string `json:"machinePath"`
	GitConfig   string `json:"gitConfig,omitempty"`
}

// The path route's 409 reasons (details.reason).
const (
	ReasonWorktreeNotALane  = "worktree_not_a_lane"   // a primary or linked worktree's id (env ruling 5)
	ReasonLanePathInvalid   = "lane_path_invalid"     // the lane parameter fails LanePathShapeOK or the Lstat walk
	ReasonGitConfigNotAFile = "git_config_not_a_file" // above
)

// NotALane is the details of a worktree_not_a_lane answer: what the laptop needs
// to resolve the backing lane (task 24b) without a second catalog walk. Kind is
// "primary" or "linked"; Branch is project.BranchName of the worktree's BranchID,
// "" when detached; Path is the worktree's root-relative path (projects/<name>…),
// for phase 3's worktree_detached sentence only, never a mount. On the wire the
// four keys sit beside "reason" in httpx.Problem.Details (map[string]string,
// httpx/problem.go:33); the json tags are those keys.
type NotALane struct {
	Kind      string `json:"kind"`
	ProjectID string `json:"projectId"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
}

// LanePathShapeOK reports whether p is exactly lanes/<a>/<b>: three
// slash-separated segments, the first DirLanes, none empty, "." or "..", and no
// backslash, colon or NUL in any. It is the syntactic half of env ruling 1's
// check; machined adds the Lstat half on its own disk.
func LanePathShapeOK(p string) bool {
	seg := strings.Split(p, "/")
	if len(seg) != 3 || seg[0] != DirLanes {
		return false
	}
	for _, s := range seg[1:] {
		if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "\\:\x00") {
			return false
		}
	}
	return true
}

// LaneMachinePath turns a lane's Path as the laptop's lane index holds it
// (phase 3's Lane.Path, "<CloneRoot>/<ProjectID>/<Name>-<Nonce>", task-03.md:66)
// into the root-relative form. A Windows laptop may hold it with backslashes
// (filepath.Join; UNVERIFIED until task 25's Remote exists), so both separators
// are accepted. Syntax only: it never touches a disk.
func LaneMachinePath(lanePath string) (string, error) {
	p := strings.ReplaceAll(lanePath, `\`, "/")
	rel, ok := strings.CutPrefix(p, MachineRoot+"/")
	if !ok || !LanePathShapeOK(rel) {
		return "", fmt.Errorf("machineapi: lane path %q is not %s/lanes/<projectId>/<dir>", lanePath, MachineRoot)
	}
	return rel, nil
}

// EnsureDirs is POST /v1/dirs' body: machine paths to create if absent, owned
// by the agent user, mode 0700. Each obeys backend.MachineMount's path rules.
type EnsureDirs struct {
	Paths []string `json:"paths"`
}
```

#### Contract — the routes (every one but `/healthz` requires `Authorization: Bearer <token>`, compared in constant time)

| Route | Answer | Failures |
|---|---|---|
| `GET /healthz` | `200 {"status":"ok"}` (the readiness probe) | — |
| `GET /v1/projects` | `200 Projects` — `project.Scanner` over `<root>/projects` | 401, 500 `internal` |
| `GET /v1/projects/{projectId}/branches` | `200 Branches` | 404 `project_not_found` |
| `GET /v1/projects/{projectId}/worktrees` | `200 Worktrees` | 404 `project_not_found` |
| `GET /v1/worktrees/{worktreeId}/path?lane=<machine path>` | `200 WorktreePath`, a lane path only (below) | 400 `bad_request` · 404 `worktree_not_found` · 409 `conflict`, `details.reason` one of `worktree_not_a_lane` (details: `NotALane`), `lane_path_invalid`, `git_config_not_a_file` |
| `POST /v1/dirs` | `204` | 400 `bad_request` — an escaping or absolute path, naming it |
| `DELETE /v1/conversations/{worktreeId}/{conversationId}` | `204` — removes every `<conversationId>.jsonl` under `conversations/<worktreeId>/` (Claude Code writes them one directory down, e.g. `-home-dev-work/`, per `internal/api/sessions.go:38-43`); `204` also when none existed | 400 if either id fails its pattern (`^w-[a-f0-9]{8}$`, a UUID) |
| `DELETE /v1/conversations/{worktreeId}` | `204` — removes the whole store (worktree removal, OD-10) | 400 |

**The worktree path route, in order** (env rulings 1 and 5). The laptop's lane index is the
authority for which directory a lane id names (ruling 1): the laptop sends the lane's path, and
`machined` checks it on its disk. `machined` never recomputes a lane id: `project.WorktreeID`
hashes `filepath.Abs` of the path (`project.go:92-98`), so a cloud lane's id minted on a Windows
laptop is not the hash `machined` would compute on Linux.
1. `worktreeId` fails `^w-[0-9a-f]{8}$` → 400 `bad_request`.
2. The Scanner over `<root>/projects` answers `worktreeId` → 409 `conflict`, `details.reason
   "worktree_not_a_lane"`, details `NotALane` (`kind` `primary` when `IsPrimary`, else `linked`).
   Checked before `lane`, so a primary's id is refused whatever `lane` names.
3. `lane` absent → 404 `worktree_not_found` (the id is no worktree this machine lists, and no
   lane is named).
4. `lane` fails `LanePathShapeOK`, or any of `lanes`, `lanes/<p>`, `lanes/<p>/<d>` is not an
   `os.Lstat` directory (a symlink, a file, or absent) → 409 `conflict`, `details.reason
   "lane_path_invalid"`.
5. The `GitConfig` rule above, by `Lstat` only → `200 WorktreePath{MachinePath: lane, …}`.

No step runs git: steps 4 and 5 are `Lstat` calls, and step 2's git runs in `projects/`, which no
sandbox mounts (R4.1). Task 21 adds one step between 3 and 4: 409 `lane_creating` while the
path is in its creating set (env ruling 4).

Problem bodies are `httpx`'s, with `httpx`'s codes. Every `GET` is read-only; a
`TestNoMachinedGetChangesState` audit pins it, and task 19's proxy relies on it.

**The delete rule is OD-8's, applied:** a harness's file may be deleted by the id the control
plane minted; it is never opened, parsed or rewritten here. The conversation id is Claude's own
session id, which the control plane already mints and passes as `--session-id` (seam SX.10).

#### Tests to write first

`catalog_test.go` (a temp dir as root, real `git` if present — skip with a message if not, as
phase 2's project tests do):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestProjectsListsReposUnderProjectsOnly` | a repo in `projects/a` is listed; one in `lanes/` is not | lane clones listed as projects (phase 3 S3-P8.2's warning) |
| `TestTheJSONIsPhase2sShape` | the body decodes into `machineapi.Projects` and each project's keys are exactly `id`, `name`, `path` | a second, drifting project shape |
| `TestALanePathIsRelativeToTheRoot` | a clone at `lanes/p-1a2b3c4d/w-0badc0de-3f9a1c` asked with that `lane` → `machinePath` exactly that, no leading `/` | an absolute path that fails task 15's validation |
| `TestAPrimaryIdIsNotALane` | the primary worktree of `projects/a` (branch `main`) → 409, `reason "worktree_not_a_lane"`, `kind "primary"`, its `projectId`, `branch "main"`, `path "projects/a"`; a linked worktree → `kind "linked"`; detached → `branch ""`; each also with `lane` set to a valid lane → still 409 | a session mounting `projects/<name>` (R4.1), or a primary id that borrows another lane's path |
| `TestAProjectsPathIsNotALane` | `lane=projects/a`, `lane=projects/a/x` → 409 `lane_path_invalid` | a `projects/` mount through the lane parameter (R4.1) |
| `TestADotDotLanePathIsRefused` | `lanes/../projects/a`, `lanes/p/..`, `lanes/p/./d`, `lanes/p/d/..`, `lanes//d`, `lanes/p/d/e`, `lanes\p\d` → 409 `lane_path_invalid`; nothing outside `lanes/` was `Lstat`ed | a lane path that climbs into `projects/` or off the disk |
| `TestASymlinkedLaneSegmentIsRefused` | `lanes/p` a symlink to `../projects`, then `lanes/p/d` a symlink to a real lane → each 409 `lane_path_invalid`; a regular file at `lanes/p/d` → 409 | a subPath that Kubernetes resolves through an agent's symlink (env ruling 2 (b)) |
| `TestAMissingLaneIs409AndAnUnknownIdWithoutALaneIs404` | an absent `lanes/p/d` → 409 `lane_path_invalid`; an id no worktree has, with no `lane` → 404 `worktree_not_found`; a malformed id → 400 | a 500, or a lane directory created by a GET |
| `TestWorktreePathNamesTheGitConfigOnlyWhenItIsAFile` | a lane clone → `gitConfig == "lanes/p-1a2b3c4d/w-0badc0de-3f9a1c/.git/config"`; a lane whose `.git` is a file → `gitConfig` absent; `.git/config` replaced by a symlink to `../../b/.git/config`, or removed → 409 `git_config_not_a_file` | a subPath bind that follows an agent's symlink, or that Kubernetes creates as a directory (review I7, b06 re-review 1) |
| `TestTheLanePathRouteRunsNoGitInALane` | the route run over a spy `project.Runner`: no recorded call has `dir` under `<root>/lanes` and no argv names a lane (`-C`, `--git-dir`, `--work-tree`) | host git in a sandbox-writable directory (R4.2; `rev-parse --git-dir` to find `.git`) |

`machineapi/wire_test.go`: `TestLanePathShapeOK` (the table of `TestADotDotLanePathIsRefused`
false, `lanes/p/d` true; kills a check that admits `..`) and `TestLaneMachinePath`
(`/machine/lanes/p/d` and `\machine\lanes\p\d` → `lanes/p/d`; `/machine/projects/a`,
`/machinex/lanes/p/d`, `C:\machine\lanes\p\d` and `lanes/p/d` → error; kills a prefix check that
admits a sibling root or `projects/`).

`dirs_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEnsureDirsCreatesOwnedPrivateDirs` | the directories exist, mode `0700`, owned by the process uid | root-owned or world-readable conversation stores |
| `TestEnsureDirsRefusesEscapes` | `../etc`, `/etc`, `a/../../b` → 400, nothing created outside root | a path that climbs off the disk |
| `TestForgetDeletesOnlyThatConversation` | two `.jsonl` files in the store's subdirectory; delete one id → only it is gone | Forget wiping the whole store |
| `TestForgetNeverReadsTheFile` | the file is made unreadable (`0000`) — delete still succeeds | an implementation that opens the transcript (OD-8's line) |
| `TestRemovingAWorktreeDeletesItsStore` | `DELETE /v1/conversations/w-…` → the directory is gone | OD-10's store left behind |
| `TestIdsArePatternChecked` | a worktree id `w-../x` and a conversation id `x` → 400 | path injection through an id |

`server_test.go`: `TestEveryRouteButHealthzNeedsTheToken` (401 without, 401 with a wrong one);
`TestTheTokenIsComparedInConstantTime` (the handler uses `subtle.ConstantTimeCompare` — asserted
by a source anchor, since timing is not unit-testable); `TestNoMachinedGetChangesState` (every
`GET` handler run against a root snapshot leaves the tree byte-identical); `TestMissingConfigExits2`.

#### Steps

- [ ] **Step 1:** Wire types; tests; they fail.
- [ ] **Step 2:** `config.go`, `server.go` (auth, routes, problem bodies), `catalog.go` (the
  Scanner over `<root>/projects`; `Path` stripped of the root prefix; the worktree path route in
  the five steps above, its lane half `Lstat` only), `dirs.go`.
- [ ] **Step 3: Mutation check** (rule 7), each on a `$TEMP` copy, each verified applied and
  turning the named test red:
  (1) step 2 moved after step 4 → `TestAPrimaryIdIsNotALane`, on its rows with a valid `lane`;
  (2) step 4's walk `os.Stat` instead of `os.Lstat` → `TestASymlinkedLaneSegmentIsRefused`;
  (3) step 4 checks the leaf only → the same test, on its `lanes/p` row;
  (4) `LanePathShapeOK` without the `..` check → `TestADotDotLanePathIsRefused`;
  (5) the `.git` lookup through `rev-parse --git-dir` in the lane → `TestTheLanePathRouteRunsNoGitInALane`;
  (6) `LaneMachinePath` without the backslash replace → `TestLaneMachinePath`, on `\machine\lanes\p\d`.
  The two named beforehand as likeliest to survive are (3), because only a symlinked middle
  segment tells a leaf check from a walk, and (5), because git's answer for a clean lane is the
  same `.git`, so only the spy `Runner` sees it. Each is killed only by the row named beside it.
- [ ] **Step 4:** Full run; `CGO_ENABLED=0 GOOS=linux go build ./control-plane/cmd/machined`
  succeeds (task 27 builds it this way); commit.

#### Acceptance

```bash
go test ./control-plane/cmd/machined/ ./control-plane/internal/machineapi/ -race -count=1   # ok (catalog tests skip without git)
CGO_ENABLED=0 GOOS=linux go build -o /tmp/machined ./control-plane/cmd/machined            # builds
go test ./sandboxd/... ./control-plane/... -race -count=1                                   # ok
```

#### What this task does not claim

- Phase 3's composite catalog (lanes beside `git worktree list`, its task 11): `machined` keeps
  phase 2's Scanner over `projects/`, and nothing switches it. The composition runs on the
  laptop, from the laptop's lane index (task 25, P3A-2). An enumerator here that listed lanes
  would make step 2 refuse every lane id as `linked`.
- Lane git. `machined` serves no `/git/v1/`: a lane's git runs in `sandboxd` in the pod that
  mounts it, a session or a runner-only pod (env ruling 2; tasks 25b and 18). Lane creation,
  the creating set and `lane_creating` are task 21's (env ruling 4).
- That the backing lane exists: task 24b resolves or creates it before it asks this route.
- **UNVERIFIED on Kata** (task 32 probes; env ruling 2 (a), (b)): that a subPath of the machine
  disk confines a container to that lane inside the guest, and kubelet's subPath symlink
  safety under Kata. The `Lstat` walk runs once, at pod create. It relies on `lanes/<p>` being
  written by `machined` alone and on a lane's own directory being its pod's mount root, which no
  agent can swap for a symlink; both hold only if (a) and (b) do.
- That `git` in the sandbox image behaves inside Kata as on Docker — the sandbox-git fix (sibling
  `sandbox-git-fix` worktree, `2d6d95b`) changed the image for the corporate CA; task 27 builds on
  whatever image is current.
