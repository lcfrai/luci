### Task 25: a cloud machine's project-only `gitx.Machine`, its `MachineFS`, and its composite catalog (seam S3-P8.1)

HELD until phase 3's tasks 02 to 04 and 11 are sync-merged into feat/env, with phase 3's S8-P3.6 (`gitx.MachineFS`, with `Seal`, planned at phase-3 `task-02.md:286-299`) and P3A-1 (`Machine.Join`, `task-02.md:250-254`), and tasks 21 and 24b land (`.superpowers/sdd/env/held.md`): the build stream skips this task until then.

Re-planned onto phase 3's ruling R4 by the env rulings of 2026-09-25
(`.superpowers/sdd/env/r4-env-rulings.md`, "What each task changes", task 25). A cloud machine meets
phase 3 at three seams. This task builds the laptop's half of all three; lane git is task 25b's.
- **`gitx.Machine`** (`ID`, `Git`, `CloneRoot`, `Join`, `FS`; phase-3 `task-02.md:242-260`) runs **project** git
  only (R4.2). `machines.Remote` below is its cloud implementation, over task 21's `POST /v1/git`.
  It lives in phase 8's `machines`, not phase 3's `gitx`, so `gitx` never imports `machineapi`
  (review r4-1 M4). Its `CloneRoot()` is `/machine/lanes` (ruling 4).
- **`gitx.MachineFS`** (seam S8-P3.6, asked of phase 3; ruling 4) is the filesystem half of lane
  create that `Machine` lacks: step 3's `os.Lstat` (phase-3 `task-03.md:153`), the failure path's
  `RemoveTree` (`task-03.md:276-280`) and `CopyGeneratedHooks`' copy (`task-03.md:235-274`).
  `Remote.FS()` is its cloud implementation, over task 21's `/v1/fs/…` routes and
  `POST /v1/lanes/sealed`.
- **The catalog.** Phase 3's `lanes.Catalog` (`task-03.md:433-445`) composes a `Source` (phase 2's
  enumeration) with the lane index (`Store`). The lane index is on the laptop (P3A-2), so for a
  cloud machine the composition runs **on the laptop**: `Source` is task 24b's agent catalog,
  which is `machined`'s phase 2 Scanner, and `Store` is the laptop's index for that machine. Lanes
  are listed from the index. Nothing runs git in `lanes/` (R4.2).

Retired by R4 and absent here: `Scope` `"clone"` on the wire, `clone_config_untrusted`,
`*UntrustedConfig{Scope: Clone}`, a project call whose `Dir` is under `lanes/`, and `FetchURL`
(phase 3 removed it from `Machine`, `task-02.md:559`; the fetch-back reads a bundle, task
25b). Survives: P3A-1 (`Join`), P3A-2, and the `git_failed` and unreachable-agent mappings.

**Correction to the rulings.** They say task 25 "covers … the catalog switch to phase 3's composite
catalog" and the old file list switched `cmd/machined/catalog.go`. `machined` cannot build that
catalog: its `Store` is the laptop's index, which `machined` never sees (P3A-2). So `machined`
keeps phase 2's Scanner over `projects/` (task 20), and this task adds one test there that pins
it never enumerates `lanes/`. The switch is the laptop's.

**Depends on** phase 3 tasks 02–04 and 11 (`Exec` with `Call.NewLane`, the lane store and
`Creator`, `lanes.Catalog`, the machine resolver, clone remove), phase 3's **acceptance of
S8-P3.6** (`gitx.MachineFS`), task 20 (`machineapi.MachineRoot`), task 21 (the create shapes,
the `/v1/fs/…` routes, `sealed`) and task 24b (`machines.Agent`, the agent catalog). If S8-P3.6
is declined, a cloud machine cannot create lanes (ruling 4): Step 1 stops and reports. The plan
holds this task out of the build stream until phase 3's code for it (planned, phase-3
`task-02.md:286-299`) is sync-merged into feat/env ("Order", "Held"; P3A-5). Task
25b depends on this task.

**Files:**
- `control-plane/internal/machines/remote.go`, `remote_test.go` — `machines.Remote` and its `gitx.MachineFS` (new; phase 8's package, so phase 3's `gitx` gains no phase 8 import)
- no `machineapi` file: the `/v1/fs/…` and `sealed` wire is task 21's (`task-21.md:139-186`), and an agent's problem body is task 24b's `machines.AgentError` (`task-24b.md:62-71`) (review-2 M2, M3)
- `control-plane/internal/machines/agent.go` — `*Agent` implements `AgentRoutes` (modify, after 24b)
- `control-plane/internal/machines/source.go` — `CatalogSource`, the agent catalog as a `lanes.Source` (new)
- `control-plane/internal/api/catalog.go` — a cloud `machineId` is served by `lanes.Catalog{Source, Store, FS}` (modify, after 24b)
- wherever phase 3 resolves a `gitx.Machine` by machine id (its task 04 wiring) — one arm for `m-` ids (modify)
- `control-plane/cmd/machined/catalog_test.go` — `TestCatalogScansProjectsOnly` (modify; no production change)

#### Contract (complete)

```go
// package machines, not gitx (review r4-1 M4): phase 3's gitx never imports phase 8's
// machineapi. In the comments below, Machine, MachineFS, Runner, Error, UntrustedConfig,
// ErrLaneDir and ErrCallShape are gitx's, and MachineRoot is machineapi.MachineRoot, task 20's
// one constant (task-20.md:53-55).
package machines

import (
	"context"
	"path"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/machineapi"
)

// AgentRoutes is task 21's routes on one machine's agent, through the broker's agent proxy.
// *Agent implements it; tests use a fake. A problem body the agent answered comes back as
// *AgentError (task 24b's, the one type; review-2 M2); anything else (the broker unreachable, backend.ErrUnavailable) comes back
// as it is.
type AgentRoutes interface {
	Git(ctx context.Context, call machineapi.GitCall) (machineapi.GitResult, error) // POST /v1/git
	Lstat(ctx context.Context, req machineapi.FSPath) (machineapi.FSStat, error)    // POST /v1/fs/lstat
	RemoveTree(ctx context.Context, req machineapi.FSRemoveTree) error                // POST /v1/fs/remove-tree
	CopyHooks(ctx context.Context, req machineapi.FSCopyHooks) (machineapi.FSCopied, error) // POST /v1/fs/copy-hooks
	Seal(ctx context.Context, req machineapi.FSPath) error                            // POST /v1/lanes/sealed
}

// Remote is a cloud machine: seam S3-P8.1's second implementation. Paths it takes and hands
// out are absolute on the MACHINE, under machineapi.MachineRoot, slash-separated, never host
// paths.
type Remote struct {
	MachineID string // "m-…"
	Agent     AgentRoutes
}

var (
	_ gitx.Machine   = Remote{}
	_ gitx.MachineFS = remoteFS{}
)

func (r Remote) ID() string { return r.MachineID }

// CloneRoot is MachineRoot + "/lanes" (machineapi.DirLanes; ruling 4).
func (r Remote) CloneRoot() string { return machineapi.MachineRoot + "/" + machineapi.DirLanes }

// Join is P3A-1's method: machine paths are slash-separated whatever the control plane's OS.
func (r Remote) Join(elem ...string) string { return path.Join(elem...) }

// FS is the machine's MachineFS (S8-P3.6), over the same Agent.
func (r Remote) FS() gitx.MachineFS { return remoteFS{r.Agent} }

// Git runs project git on the machine agent. Before any request it refuses, in this order:
//  1. ErrLaneDir: Dir or NewLane under CloneRoot() on a call that is not ruling 4's creating
//     write (Scope None with Dir == NewLane). Phase 3's Exec guard (task-02.md:139-149),
//     mirrored so the laptop refuses before machined does (R4.2).
//  2. ErrCallShape: Scope Unset; a Dir that is neither "" nor absolute under MachineRoot after
//     path.Clean (a backslash, a drive letter or a ".." that climbs out is refused); Scope
//     Project with Dir not under MachineRoot/projects/; Scope None with Dir "" and NewLane set,
//     or with a Dir that is not its NewLane; NewLane set on any other scope.
// It sends Dir and NewLane relative to MachineRoot ("" stays "": machined's own owned root,
// task 21), Class as "read" | "local" | "remote", Scope as "project" | "none", and RemoteURL,
// Stdin, Env and OKExit unchanged. It maps the answer back:
//   - *AgentError "git_failed" → *Error{Dir, Args from the Call; ExitCode, TimedOut and
//     Stderr = details.gitSays from the details};
//   - "conflict" with details.reason "project_config_untrusted" → *UntrustedConfig{Dir,
//     Key: details.key, Scope: Project}; any other conflict reason stays an *AgentError;
//   - "bad_request" (400) → an error wrapping ErrCallShape;
//   - anything else passes through, never an *Error, so the cockpit never says "git said" for a
//     network failure. Callers that switch on *Error or *UntrustedConfig behave as for Local.
func (r Remote) Git() gitx.Runner

// remoteFS is MachineFS over the agent. Every path is absolute under MachineRoot and sent
// relative. Exists, RemoveTree's path and Seal take a path under CloneRoot(); RemoveTree's root is
// CloneRoot() or under it; CopyGeneratedHooks takes projectTop under MachineRoot/projects/ and
// cloneTop under CloneRoot(). Any other path is ErrCallShape before any request. Problems map as
// Git's do, less the git ones.
type remoteFS struct{ agent AgentRoutes }

func (f remoteFS) Exists(ctx context.Context, p string) (exists, dir bool, err error)
func (f remoteFS) RemoveTree(ctx context.Context, root, p string) error
func (f remoteFS) CopyGeneratedHooks(ctx context.Context, projectTop, cloneTop, rel string) (int, error)
func (f remoteFS) Seal(ctx context.Context, p string) error
```

`gitx.MachineFS` is **phase 3's** interface, the shape this plan asks for in seam **S8-P3.6**
(env ruling 4, "The one S8-P3.6 contract", copied verbatim; task 21 copies the same, at
`task-21.md:192-214`). If phase 3 answers with another spelling, Step 1 adapts `remoteFS` to it
and leaves the routes alone. `lanes.Creator`, `lanes.CopyGeneratedHooks`, task 11's `Remove` and
`lanes.Catalog.Path` take it from the machine instead of calling `os`.

```go
package gitx

import "context"

// MachineFS is the filesystem half of lane create and remove (phase-3
// task-03.md:153-154, :189-190, :227-268). Paths are absolute on that machine.
// The local implementation is os, on the laptop's disk.
type MachineFS interface {
	// Exists is os.Lstat at path: exists is false only for not-exist; dir is true only for a
	// real directory, never a symlink. Step 3 reads exists; Catalog.Path's worktree.Verify reads dir.
	Exists(ctx context.Context, path string) (exists, dir bool, err error)
	// RemoveTree is task 03's RemoveTree(root, path): it refuses a path not under root.
	RemoveTree(ctx context.Context, root, path string) error
	// CopyGeneratedHooks is the copy half of task 03's CopyGeneratedHooks: its Lstat
	// guards, its SameFile check and its 64-file / 1 MiB budget. Its ls-files check and
	// the take-back stay in lanes.CopyGeneratedHooks, over m.Git() and RemoveTree.
	CopyGeneratedHooks(ctx context.Context, projectTop, cloneTop, rel string) (int, error)
	// Seal ends R4.5's window for path: Create calls it after step 6, before Store.Add.
	// Locally it does nothing; on a cloud machine it closes machined's creating-set entry.
	Seal(ctx context.Context, path string) error
}
```

The wire is task 21's and is not declared here (review-2 M3): `GitCall.NewLane`, the
`PathFS…` and `PathLanesSealed` routes, `FSPath`, `FSStat`, `FSRemoveTree` (root "lanes or a
lane"), `FSCopyHooks` and `FSCopied`, all in `machineapi/wire.go` (`task-21.md:139-186`). Paths
are relative to the machine root. An agent's problem body comes back as task 24b's
`*machines.AgentError` (`task-24b.md:62-71`); there is no `machineapi.Problem` (review-2 M2).

The catalog for a cloud machine, on the laptop:

```go
package machines

// CatalogSource is task 24b's agent catalog for one machine as a lanes.Source (phase-3
// task-03.md:433-440): Projects, Branches, Worktrees and Path over the agent's catalog routes.
type CatalogSource struct {
	Agent     *Agent
	MachineID string
}

var _ lanes.Source = CatalogSource{}

// In internal/api/catalog.go, a cloud machineId is served by
//	&lanes.Catalog{Source: CatalogSource{…}, Store: <the lane index for machineId>, FS: remote.FS()}
// Catalog.FS is S8-P3.6's: nil means os, as today.
```

#### Asks of phase 3, recorded here and in task 33 (phase 3's code is not edited silently)

| Ask | What | Why | Plan proceeds on |
|---|---|---|---|
| **P3A-1** Machine paths are not host paths | `Machine` gains `Join(elem ...string) string` (`Local.Join` = `filepath.Join`, `Remote.Join` = `path.Join`), and phase 3 joins every path it derives from `CloneRoot()` through it | the control plane runs on **Windows**; `filepath.Join("/machine/lanes", "p-1")` yields `\machine\lanes\p-1`, which the agent refuses | phase 3 files it (seams review 1 M4): task 02 declares `Join` on `gitx.Machine` with `Local.Join` = `filepath.Join` (phase-3 `task-02.md:250-254`, `:272`), and task 03's step 3 builds `Path` through `m.Join` (phase-3 `task-03.md:155-158`). This task implements `Remote.Join` only and edits no phase 3 interface or call site |
| **P3A-2** Where the lane index lives for a cloud machine | the index stays on the laptop, keyed by `(machineId, projectId)` | a machine is per user, not per laptop | v1 states the single-laptop limitation in the cockpit's machine popover. The same limit holds for sessions (review I6, task 23). A default machine's id recurs if its disk is lost out of band, so the index also holds the machine's `createdAt` and drops a machine's entries when it changes (b09 re-review 4; the broker refuses to delete the default, `409 machine_is_default`) |
| **S8-P3.6** `gitx.MachineFS` (ruling 4; plan P3A-5; task 33) | the interface above; `Creator`, `CopyGeneratedHooks`, task 11's `Remove` and `Catalog.Path` take it from the machine; `Creator` calls `Seal` after step 6 | three create steps and `Catalog.Path`'s `worktree.Verify` touch the **laptop's** disk, which for a cloud machine is the wrong disk | nothing: without it a cloud machine cannot create lanes (ruling 4), so this task does not start |

#### Tests to write first

`remote_test.go`, with a fake `Agent` that records every request:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRemoteSendsDirRelativeToTheMachineRoot` | `Call{Dir: "/machine/projects/app", Scope: Project}` → `GitCall{Dir: "projects/app", Scope: "project"}` | an absolute path the agent refuses, or a host path sent |
| `TestRemoteRefusesADirOutsideTheRoot` | `"C:\\Users\\x"`, `"/etc"`, `"/machine/../etc"`, and `"/machine"` with `Scope Project` → `ErrCallShape`, no request | a laptop path leaking to the machine |
| `TestRemoteRefusesAProjectCallInALane` | `Scope Project`, `Dir: "/machine/lanes/p/app-0a1b2c"` → `ErrLaneDir`, no request | the host running git in a sandbox-writable directory (R4.2) |
| `TestRemoteSendsOnlyTheNewLaneWrites` | phase 3's `TestExecAdmitsTheNewLaneOnly` cases (`task-02.md:654`): `Dir == NewLane == L`, `Scope None` → sent with both relative; `Dir: L2` → `ErrLaneDir`; `NewLane: L` with `Scope Project` → `ErrCallShape` | a NewLane that opens every lane, or any scope |
| `TestRemoteSendsTheCreateCloneAsAProjectCall` | task 03 step 4's call (`Dir` the project, `Scope Project`, `Class Local`, `NewLane ""`, `Env` = `NoFilterEnv`, the clone's destination under `lanes/` as a positional) → sent unchanged | an unmarked clone refused, or a clone marked `NewLane`, which machined refuses (ruling 4) |
| `TestRemoteSendsAnEmptyDirOnlyForScopeNone` | `Scope None, Dir ""` → `Dir ""`; `Scope None, Dir "/machine"` and `Scope Project, Dir ""` → `ErrCallShape` | `/machine` sent to mean machined's owned root, which machined reads as the disk root |
| `TestRemoteRefusesAnUnsetScope` | `Scope Unset` → `ErrCallShape`, no request | a call the machine must guess the scope of |
| `TestRemoteCarriesClassStdinEnvAndOKExit` | each field arrives; `Class` as `"read" | "local" | "remote"`; `Scope` only ever `"project" | "none"` | a Read call taking optional locks on the machine; a retired `"clone"` on the wire |
| `TestGitFailedBecomesAGitxError` | problem `git_failed` → `*Error{ExitCode, Stderr, TimedOut}`, `AsError` finds it, `Sentence()` is `gitSays` | the cockpit's `details.gitSays` empty for cloud machines |
| `TestAnUntrustedConfigBecomesAGitxUntrustedConfig` | `conflict`, `reason: project_config_untrusted`, `key: filter.x.clean` → `*UntrustedConfig{Key: "filter.x.clean", Scope: Project}`; `reason: clone_config_untrusted` → not an `*UntrustedConfig` | a refused filter shown as a network failure; a retired reason read as a scope |
| `TestABadRequestWrapsErrCallShape` and `TestAnUnreachableAgentIsNotAGitError` | `400` → `errors.Is(err, ErrCallShape)`; `backend.ErrUnavailable` → not an `*Error` | "git said" shown for a network failure |
| `TestCloneRootIsMachineLanes` and `TestJoinIsSlashSeparated` | `/machine/lanes`; `Join("/machine/lanes", "p-1") == "/machine/lanes/p-1"` on Windows | P3A-1's backslash path |
| `TestFSSendsRelativePathsUnderLanes` | `Exists`, `RemoveTree`, `Seal` and `CopyGeneratedHooks` send the bodies above; `FSStat` maps to `(exists, dir)`; `FSCopied` to the count | a host path, or a dropped answer |
| `TestFSRefusesAPathOutsideItsRoot` | `Exists("/machine/projects/app")`, `RemoveTree(CloneRoot, "/machine/projects/app")`, `Seal("/machine/lanes/../projects/app")`, and `CopyGeneratedHooks` with `cloneTop` under `projects/` → `ErrCallShape`, no request | a filesystem route aimed at the project or the disk root (R4.1) |
| phase 3's own `Machine` conformance tests, if its task 02 ships any | pass against `Remote` and the fake | a second implementation that diverges from the first |

`internal/api` and `internal/machines`, with a fake agent catalog and phase 3's in-memory lane
store; and `cmd/machined`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCloudCatalogListsLanesFromTheIndex` | the agent gives p's primary and linked; the index for `m-1` holds two lanes of p → p's list is primary, linked, then two with `Kind: "clone"` and `Name` the lane's `Name`; the fake agent received no request naming `lanes/` | lanes missing from the rail, or listed by scanning the machine's disk |
| `TestCloudCatalogIndexIsPerMachine` | lanes indexed for `m-2` are absent from `m-1`'s list | P3A-2's key dropped |
| `TestCloudCatalogPathOfAVanishedLane` | `FS.Exists` answers not-exist → `errors.Is(err, worktree.ErrNotADir)`; the laptop's `os` is never asked | a mount of a missing directory, or `os.Stat("/machine/…")` on the laptop |
| `TestResolverAnswersRemoteForACloudID` | `m-0123abcd` → `machines.Remote{MachineID: "m-0123abcd"}`; `local` → phase 3's `Local`; an unknown id → phase 3's not-found | a cloud id served by the laptop's disk |
| `TestCatalogScansProjectsOnly` (`cmd/machined`) | a root holding `projects/app` (a repo) and `lanes/p/app-0a1b2c` (a clone) → the projects list is `app` only, and a spy `Runner` records no git whose `Dir` is under `lanes/` | `machined` enumerating lanes by running git in them (R4.2) |

#### Steps

- [ ] **Step 1:** Confirm the dependencies: `git grep -n "type Machine interface" -- control-plane/internal/gitx/`, `git grep -n "type MachineFS interface" -- control-plane/internal/gitx/` (S8-P3.6), `git grep -n "NewLane" -- control-plane/internal/gitx/`, `git grep -n "const MachineRoot" -- control-plane/internal/machineapi/` (task 20), phase 3's `lanes.Catalog`, and tasks 21 and 24b landed. If `MachineFS` is absent, stop and report S8-P3.6 unanswered; the plan holds this task out of the stream until then ("Order", "Held"). P3A-1 is phase 3's (the table): confirm `git grep -n "Join(elem" -- control-plane/internal/gitx/` prints, then add `Remote.Join` only. Name the resolver file with `git grep -n "LocalMachineID" -- control-plane/cmd/api/`.
- [ ] **Step 2:** `remote_test.go`, red; `remote.go`; the `machineapi` wire if task 21 lacks it; `machines.Agent`'s five calls. Green.
- [ ] **Step 3:** The catalog tests, red; `CatalogSource`; the cloud branch of `api/catalog.go`; the resolver arm. Green.
- [ ] **Step 4:** `TestCatalogScansProjectsOnly` (green on task 20's code; if it is red, task 20 is wrong, and this task reports it rather than patching `machined`).
- [ ] **Step 5: Mutation check** (rule 7), each on a `$TEMP` copy, each verified applied and
  turning the named test red:
  (1) `Git`'s `ErrLaneDir` guard dropped → `TestRemoteRefusesAProjectCallInALane`;
  (2) the `NewLane` exception widened to any `Dir` under `CloneRoot()` → `TestRemoteSendsOnlyTheNewLaneWrites`, on its `L2` row;
  (3) `Dir` sent absolute → `TestRemoteSendsDirRelativeToTheMachineRoot`;
  (4) `remoteFS` checks no root → `TestFSRefusesAPathOutsideItsRoot`;
  (5) the cloud catalog lists lanes through the agent → `TestCloudCatalogListsLanesFromTheIndex`;
  (6) `Catalog.FS` left nil for a cloud id → `TestCloudCatalogPathOfAVanishedLane`.
  The two named beforehand as likeliest to survive are (2), because every other `NewLane` row
  has `Dir == NewLane`, and (6), because the laptop's `os.Lstat("/machine/…")` also answers
  not-exist, so only the "`os` is never asked" assertion catches it. Each is killed only by the
  row named beside it.
- [ ] **Step 6:** Full run, including phase 3's suites; commit.

#### Acceptance

```bash
go test ./control-plane/internal/gitx/ ./control-plane/internal/machines/ ./control-plane/internal/api/ ./control-plane/cmd/machined/ -race -count=1   # ok
git grep -n -e 'internal/machineapi' -e 'MachineRoot' -- control-plane/internal/gitx/   # no output: phase 3's gitx imports nothing of phase 8's
go test ./sandboxd/... ./control-plane/... -race -count=1                                                                                          # ok
git grep -n -e 'clone_config_untrusted' -e 'Scope.*"clone"' -- control-plane/internal/gitx/ control-plane/internal/machineapi/   # no output: no clone scope on the wire
```

#### What this task does not claim

- Lane git on a cloud machine: `Lanes.For`, the runner-only pod and the bundle relay are task 25b.
- That phase 3's convergence flows have run against a real cloud machine: that is the owner's
  batched hands-on session (OD-12), listed in task 32.
- That phase 3 accepts S8-P3.6. The shape above is this plan's ask (UNVERIFIED until phase 3
  answers).
- Push from a cloud machine to a git host: phase 3's push (its task 13) runs in the project,
  `Scope Project`, `Class Remote`, with `RemoteURL`, through this `Git`, and is unobserved.
  `machined` admits the user's token only as `http.<RemoteURL's origin>/.extraHeader`, the form
  `PushEnv` writes (task 21, review I7). A push from a lane is refused here (`ErrLaneDir`), as
  R4.2 requires.
