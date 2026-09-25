### Task 25: a cloud machine's project-only `gitx.Machine` and its lane runners (seam S3-P8.1)

After phase 3's ruling R4 (P8-D7; review Q18) phase 3 has two git seams, and a cloud machine
needs both (sibling `phase-3` worktree, plan `2026-09-23-phase-3-git-convergence.md:120-128`,
`:436`, read at `9935278`):
- `gitx.Machine` — `ID()`, `Git() Runner`, `CloneRoot()`, … — runs **project** git only, under
  §HGH. `gitx.Remote` below is its cloud implementation, over task 21's `POST /v1/git`.
- `lanegit.Lanes.For` answers a `lanegit.Runner` for a lane (phase-3 `task-24.md:565-579`,
  `:654-659`). Its Kata half answers a `Runner` pointed at `machined`'s `/git/v1/` runner through
  the broker, where phase 3's katabe answers `ErrNoRunner`. Lane work crosses as a bundle the host
  verifies and fetches in the project (R4.4); no call here reads a lane's `.git`.

**Re-plan required before this task starts (review Q18).** The contract and tests below predate
R4. Where they and this block disagree, this block holds. Retired: `Scope` `"clone"` on the wire,
`clone_config_untrusted`, `*UntrustedConfig{Scope: Clone}`, a `Remote.Git` call whose `Dir` is
under `lanes/`, and `FetchURL` as "the clone's path" (the fetch-back reads a bundle file, never a
lane). To add: the Kata `Lanes.For` and its `Runner` over the broker, and the bundle hand-off of
task 21's open item 3 (a lane bundle the laptop receives must reach `machined`'s owned
`bundles` directory before a project call verifies and fetches it, and a refresh bundle the
project writes there must reach the runner). Survives: P3A-1 (`Join`), P3A-2, and the `git_failed`
and unreachable-agent mappings.

**Depends on** phase 3 tasks 02–04 and 11 (the runner, the lane store, clone create/remove, the
composite catalog) and on tasks 21 and 24b here. If any is missing, this task does not start.

**Files:**
- `control-plane/internal/gitx/remote.go`, `remote_test.go` — `gitx.Remote` (new file in phase 3's package: the seam's own home)
- `control-plane/internal/machines/agent.go` — `Agent` implements `gitx.AgentGit` (modify)
- `control-plane/cmd/machined/catalog.go` — switch from phase 2's Scanner to phase 3's composite catalog over `<root>/projects` and `<root>/lanes` (S3-P8.2) (modify)
- wherever phase 3 resolves a `gitx.Machine` by machine id (its task 04 wiring) — one arm for cloud ids (modify)

#### Contract (complete)

```go
package gitx

import (
	"context"
	"path"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/machineapi"
)

// AgentGit is the one call a cloud machine's Runner needs: task 21's
// POST /v1/git on that machine's agent, through the broker. machines.Agent
// implements it; gitx does not import machines, so there is no cycle.
type AgentGit interface {
	Git(ctx context.Context, call machineapi.GitCall) (machineapi.GitResult, error)
}

// MachineRoot is where a cloud machine's disk is mounted in its machine pod
// (PANACEA_APP_MACHINED_ROOT, task 20). Paths a Remote hands out are absolute
// on the MACHINE and slash-separated — never host paths.
const MachineRoot = "/machine"

// Remote is a cloud machine: seam S3-P8.1's second implementation.
type Remote struct {
	MachineID string   // "m-…"
	Agent     AgentGit
}

var _ Machine = Remote{}

func (r Remote) ID() string { return r.MachineID }

// Git runs on the machine agent. Call.Dir must be absolute under MachineRoot
// (slash-separated); it is sent relative. The one exception is a Scope None
// call, whose Call.Dir must be "" and is sent as "": machined runs it in its own
// <OwnedRoot>/clone, never in MachineRoot itself (task 21 item 2); a Scope None
// call with any other Dir, or any other scope with Dir "", is ErrCallShape here,
// before any request. Call.Scope and Call.RemoteURL are
// sent as GitCall.Scope ("clone" | "project" | "none") and RemoteURL; an Unset
// scope is ErrCallShape here, before any request. A problem "git_failed" becomes a
// *Error with ExitCode, Stderr (= details.gitSays) and TimedOut from the
// details, Args and Dir from the Call. A problem "conflict" with details.reason
// clone_config_untrusted | project_config_untrusted becomes
// *UntrustedConfig{Dir, Key: details.key, Scope}. A 400 wraps ErrCallShape. So
// callers that switch on *Error or *UntrustedConfig, and the cockpit's
// details.gitSays, behave exactly as for Local (task 21, review I7).
func (r Remote) Git() Runner

// CloneRoot is MachineRoot + "/lanes" (machineapi.DirLanes).
func (r Remote) CloneRoot() string { return MachineRoot + "/" + machineapi.DirLanes }

// FetchURL is the clone's path, unchanged: the project repository and the
// clone are on the same disk, so the fetch is local to the machine — the same
// answer Local gives, for the same reason.
func (r Remote) FetchURL(clonePath string) string { return clonePath }

// Join is P3A-1's method (below): machine paths are slash-separated whatever
// the control plane's OS. Local.Join is filepath.Join; Remote.Join is path.Join.
func (r Remote) Join(elem ...string) string { return path.Join(elem...) }
```

#### Two asks of phase 3, recorded here and in task 33 (phase 3's code is not edited silently)

| Ask | What | Why | Plan proceeds on |
|---|---|---|---|
| **P3A-1** Machine paths are not host paths | Every path phase 3 derives from `CloneRoot()` for a non-local machine is joined with `path.Join` (slash), never `filepath.Join` — or `Machine` gains `Join(elem ...string) string` | the control plane runs on **Windows**; `filepath.Join("/machine/lanes", "p-1")` yields `\machine\lanes\p-1`, which the machine agent refuses | phase 3 adds `Join` to `Machine` (`Local.Join` = `filepath.Join`, `Remote.Join` = `path.Join`) in its task 02 before it lands; if it has already landed, this task adds the method to both implementations and to the interface in one commit and names every call site it changed |
| **P3A-2** Where the lane index lives for a cloud machine | Phase 3's lane store (its task 03) persists on the laptop under `statepath`. For a cloud machine it must be **keyed by machine id** | a machine is per user, not per laptop: a second laptop signed in as the same user sees the machine's clones through the catalog but not their lane metadata | v1 keeps the index on the laptop, keyed by `(machineId, projectId)`, and states the single-laptop limitation in the cockpit's machine popover; a machine-side index served by `machined` is later. The same limit holds for **sessions** (review I6): a cloud sandbox carries its creating control plane's install id, and no other laptop lists, attaches to, reaps or ends it (task 23). A default machine's id is derived from its owner (task 19), so it recurs if its disk is lost out of band; the index also holds the machine's `createdAt` and drops a machine's entries when it changes (b09 re-review 4; the broker refuses to delete the default, `409 machine_is_default`) |

#### Tests to write first (`remote_test.go`, a fake `AgentGit`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRemoteSendsDirRelativeToTheMachineRoot` | `Call{Dir: "/machine/lanes/p-1/w-2"}` → `GitCall.Dir == "lanes/p-1/w-2"` | an absolute path the agent refuses, or a host path sent |
| `TestRemoteRefusesADirOutsideTheRoot` | `Dir: "C:\\Users\\x"` or `"/etc"` → error, no call | a laptop path leaking to the machine |
| `TestRemoteCarriesClassStdinEnvAndOKExit` | each field arrives; `Class` as `"read" | "local" | "remote"`; `Scope` as `"clone" | "project" | "none"`; `RemoteURL` | a Read call taking optional locks on the machine; a lane call sent as `project` |
| `TestRemoteRefusesAnUnsetScope` | `Scope: Unset` → `ErrCallShape`, no call | a call the machine must guess the scope of |
| `TestRemoteSendsAnEmptyDirOnlyForScopeNone` | `Scope: None, Dir: ""` → `GitCall.Dir == ""`, `Scope == "none"`; `Scope: None, Dir: "/machine"` and `Scope: Project, Dir: ""` → `ErrCallShape`, no call | the laptop sending `/machine` to mean `machined`'s owned root, which `machined` reads as the disk root |
| `TestGitFailedBecomesAGitxError` | problem `git_failed` with details → `*Error{ExitCode, Stderr, TimedOut}`, `AsError` finds it, `Sentence()` is `gitSays` | the cockpit's `details.gitSays` empty for cloud machines |
| `TestAnUntrustedConfigBecomesAGitxUntrustedConfig` | problem `conflict`, `reason: clone_config_untrusted`, `key: filter.x.clean` → `*UntrustedConfig{Key: "filter.x.clean", Scope: Clone}`; the `project_…` reason → `Scope: Project` | a refused filter shown as a network failure, or as "git said" |
| `TestAnUnreachableAgentIsNotAGitError` | the fake returns `backend.ErrUnavailable` → the error is not an `*Error` | "git said" shown for a network failure |
| `TestFetchURLIsThePath` and `TestCloneRootIsMachineLanes` | as the contract | a URL scheme invented for a same-disk fetch |
| phase 3's own `Machine` conformance tests, if its task 02 ships any, run against `Remote` with the fake | pass | a second implementation that diverges from the first |

And in `cmd/machined`: `TestCatalogListsLanesBesideWorktrees` — phase 3's composite catalog on the
machine lists a lane clone under `lanes/` as a worktree of its project (S3-P8.2).

#### Steps

- [ ] **Step 1:** Confirm the re-plan above (review Q18) has landed in this part; if this part still carries the "Re-plan required" block, stop. Confirm the dependencies: `git grep -n "type Machine interface" -- control-plane/internal/gitx/` and `git grep -n "type Runner interface" -- control-plane/internal/lanegit/`, phase 3's lane store and composite catalog exist, tasks 21 and 24b landed. Resolve P3A-1 as the table says.
- [ ] **Step 2:** Tests; `remote.go`; `machines.Agent.Git`.
- [ ] **Step 3:** The resolver arm for cloud ids; `machined`'s catalog switch.
- [ ] **Step 4:** Full run, including phase 3's suites; commit.

#### Acceptance

```bash
go test ./control-plane/internal/gitx/ ./control-plane/internal/machines/ ./control-plane/cmd/machined/ -race -count=1   # ok
go test ./sandboxd/... ./control-plane/... -race -count=1                                                               # ok
```

#### What this task does not claim

- That phase 3's convergence flows have run against a real cloud machine: that is the owner's
  batched hands-on session (OD-12), listed in task 32.
- Push from a cloud machine to a git host: it is phase 3's push (its task 13) with the user's
  token in `Env`, through the same route, and it is unobserved. It runs in the project
  repository, `Scope: Project`, `Class: Remote`, with `RemoteURL`. `machined` admits the token
  there only as `http.<RemoteURL's origin>/.extraHeader`, the form `PushEnv` writes, and refuses
  it on every `clone`-scope call (task 21, review I7). A phase 3 push from a lane clone would
  therefore be a `400`. Phase 3's task 12 runs its fetch and push with `Scope Project` (its
  contract comments, read at `93c1fdf`), which is what this plan relies on.
