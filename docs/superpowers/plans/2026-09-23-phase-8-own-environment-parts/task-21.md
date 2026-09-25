### Task 21: `machined` git — the project's git on the machine, and lane creation from it

HELD until phase 3's tasks 02 (§HGH, `gitx`) and 03 (`lanes`) are sync-merged into feat/env, with phase 3's S8-P3.6 (`gitx.MachineFS`, with `Seal`), planned at phase-3 `task-02.md:286-299` and `task-03.md:293-303` (`.superpowers/sdd/env/held.md`): the build stream skips this task until then.

Phase 3's ruling R4 (sibling `phase-3` worktree, `.superpowers/sdd/p3/orchestrator-rulings.md:3-67`,
R4 rules 1-6) splits git by the repository it runs in. This task applies it to a cloud machine,
where the project repositories and their lane clones share one disk (OD-2, P8-D6), as the env
re-plan rulings order (`.superpowers/sdd/env/r4-env-rulings.md`, rulings 2 and 4, "Task 21"):
- **Project calls** have `projects/<name>` as their repository. No sandbox mounts it (R4.1), so
  §HGH hardens these calls, and only these (R4.2). `machined` serves them on `POST /v1/git`,
  which runs **phase 3's own `gitx.Exec`** on the disk; task 25 wraps it as the laptop's
  project-only `gitx.Machine`, whose `CloneRoot()` is `/machine/lanes`.
- **Lane creation** is the one host git that names a lane (R4.5): phase 3's `lanes.Creator`
  clones the lane *from the project*, then writes its config in the new clone before any
  sandbox mounts it (phase-3 `task-03.md:148-199`). `machined` admits exactly those calls, and
  holds each new lane in an in-memory **creating set** until the laptop seals it. It also serves
  the three filesystem steps of a create that would otherwise touch the laptop's disk, as seam
  S8-P3.6's `gitx.MachineFS` (ruling 4).
- **Lane git never runs in the machine pod** (ruling 2, which amends P8-D7). `machined` serves no
  `/git/v1/`. Lane git runs in `sandboxd`, in the one pod that mounts the lane: the live
  session's, or a runner-only pod (task 25b). The machine pod runs no agent-defined filter,
  hook or config, because the only git it runs reads a project or a lane no sandbox has mounted.
- **Bundles** (lane work in, product commits out) live in `machined`'s owned root and are task
  21b's: the bundle routes, and the one bundle argument `POST /v1/git` admits (ruling 3).

**Retired by R4, and not built here.** Every `Scope: "clone"` path (the wire value,
`clone_config_untrusted`, H3's Clone allowlist, credential `Env` "in a lane"); every lane `Dir`
on `POST /v1/git` outside a create's window; the read-only `.git/config` bind as a defence
(after R4 it is hygiene, task 24b); a session on the primary worktree writing
`projects/<name>/.git` (it runs in its backing lane, env ruling 5); the lane runner route and the
vendor-or-reimplement question (ruling 2: the cloud runner *is* `sandboxd`'s); and the tests
`TestGitLaneFilterNeverRuns` and `TestGitCredentialEnvIsRefusedInALane`, with their mutants. The
review's four open items are closed by env rulings 1 and 2 (the route, and what the runner
reaches), 3 (bundles, task 21b) and 4 (the create shape, below).

**Depends on, all in phase 3 (sibling `phase-3` worktree, parts read at `339b607`):**
- **task 02 with §HGH** (`internal/gitx`: `Call` with `Scope`, `RemoteURL` and `NewLane`,
  `Result`, `Runner`, `Exec`, `Hardening` with `LaneRoot`, `MachineHardening`, `UntrustedConfig`,
  `ErrCallShape`, `ErrLaneDir`, `Error`, `Class`, `Machine`; `task-02.md:85-96`, `:242-250`,
  `:529-534`);
- **task 03** (`lanes`: `NoFetchURL`, `NoPushURL`, `NoFilterEnv`, `CopyableHooksPath`,
  `CopyGeneratedHooks`, `RemoveTree`, and `Create`'s steps; `task-03.md:148-268`);
- **seam S8-P3.6's `gitx.MachineFS`** (ruling 4): an ask of phase 3, not yet answered. Without
  it a cloud machine cannot create lanes, so it is a blocker, not a fallback.

If any has not landed, this task does not start: no local copy of `gitx` or `lanes` is written,
no second hardening is invented, and no `MachineFS` of this task's own shape is built. Phase 3's
task 24 (the runner) is **not** a dependency: `machined` serves no runner (ruling 2).

**Held out of the env build stream** (plan, "Order", "Held"): the stream skips this task, and
its dependants 21b, 25 and 25b, until Step 1's three greps pass on the env branch. S8-P3.6 is
filed with phase 3 by the orchestrator or the owner (env ruling 4); this part does not file it.

**Files:**
- `control-plane/internal/machineapi/wire.go` — `GitCall`, `GitResult`, the filesystem and seal
  bodies (modify)
- `control-plane/cmd/machined/git.go`, `git_test.go` (new) — `POST /v1/git`
- `control-plane/cmd/machined/lanefs.go`, `lanefs_test.go` (new) — the creating set, `MachineFS`'
  three routes and `POST /v1/lanes/sealed`
- `control-plane/cmd/machined/server.go` — the routes (modify)
- `control-plane/cmd/machined/catalog.go` — task 20's worktree path route answers
  `409 lane_creating` for a lane in the creating set (modify)
- `control-plane/cmd/machined/main.go` — the environment scrub below (modify)

#### Contract (complete)

```go
package machineapi

import "time"

// GitCall is POST /v1/git's body: gitx.Call on the wire. A non-empty Dir, and
// NewLane, are slash-separated and relative to the machine root (/machine).
// Dir "" is the one exception: it maps to machined's own <OwnedRoot>/clone
// (/tmp/machined-git/clone), never to /machine, and only with Scope "none".
// Class is "read" | "local" | "remote" (gitx.Read/Local/Remote).
// Scope is "project" | "none" (gitx.Project/None, phase 3 §HGH H3). It has no
// default, and it must follow Dir (below). "clone" is not a scope: R4.2 deleted
// it, and it is 400 like any unknown value. RemoteURL is gitx.Call.RemoteURL:
// required for Class "remote", an https URL.
// NewLane is gitx.Call.NewLane (phase-3 task-02.md:90-96): "" on every call but
// a lane create's writes in its new clone (task-03.md:169-181), where it equals
// Dir and names a lane in machined's creating set.
// Stdin is base64 in JSON ([]byte). Env is an explicit allowlist (below), not a
// prefix match; values may be secrets (phase 3 task 13's PushEnv passes a
// host-scoped http.<url>.extraHeader through GIT_CONFIG_COUNT/KEY_n/VALUE_n) and
// are never logged, echoed or stored.
type GitCall struct {
	Dir       string            `json:"dir"`
	Args      []string          `json:"args"`
	Class     string            `json:"class"`
	Scope     string            `json:"scope"`
	RemoteURL string            `json:"remoteUrl,omitempty"`
	NewLane   string            `json:"newLane,omitempty"`
	Stdin     []byte            `json:"stdin,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	OKExit    []int             `json:"okExit,omitempty"`
}

// GitResult is a call that exited 0 or with a code in OKExit (gitx.Result).
type GitResult struct {
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// A call that did not answer is a problem body with code "git_failed" (phase 3's
// httpx.CodeGitFailed, its task 04) and these details, from which the laptop
// rebuilds a *gitx.Error (task 25):
//   details.gitSays   gitx.Error.Sentence()
//   details.exitCode  decimal, "-1" if git did not start or timed out
//   details.timedOut  "true" | "false"
// Args and Dir are NOT in the details: the caller already has them, and Args may
// name a URL with credentials in it.
//
// A call phase 3's H3 check refused (*gitx.UntrustedConfig) never started git. It
// is a problem body with code "conflict" (409) and these details, from which the
// laptop rebuilds the *gitx.UntrustedConfig (task 25), exactly as phase 3's API
// maps it on the laptop:
//   details.reason    "project_config_untrusted" (the only scope H3 checks)
//   details.key       the refused key, lowercased; never its value
// The lane routes' state refusals are "conflict" (409) with details.reason:
//   "lane_path_invalid"  a segment of lanes/<p>/<d> is not a directory by Lstat
//                        (a symlink included), ruling 1's reason
//   "lane_not_creating"  the lane is not in the creating set; nothing ran
//   "lane_creating"      task 20's path route: the lane is still being created
const (
	DetailGitSays  = "gitSays"
	DetailExitCode = "exitCode"
	DetailTimedOut = "timedOut"
	DetailReason   = "reason"
	DetailKey      = "key"

	ReasonLanePathInvalid = "lane_path_invalid"
	ReasonLaneNotCreating = "lane_not_creating"
	ReasonLaneCreating    = "lane_creating"
)

// S8-P3.6's wire (env ruling 4, "The one S8-P3.6 contract"; task 25's
// machines.Remote.FS() is its client). Every path is relative to the root.
const (
	PathFSLstat      = "/v1/fs/lstat"
	PathFSRemoveTree = "/v1/fs/remove-tree"
	PathFSCopyHooks  = "/v1/fs/copy-hooks"
	PathLanesSealed  = "/v1/lanes/sealed"
)

// FSPath is the body of POST /v1/fs/lstat and POST /v1/lanes/sealed. Path is a
// lane, lanes/<projectId>/<dir>.
type FSPath struct {
	Path string `json:"path"`
}

// FSStat answers POST /v1/fs/lstat, read with Lstat (phase-3 task-03.md:152-154,
// Create's step 3): Exists is false only for not-exist, so a symlink counts; Dir
// is true only for a real directory, never a symlink (Catalog.Path's
// worktree.Verify reads it).
type FSStat struct {
	Exists bool `json:"exists"`
	Dir    bool `json:"dir"`
}

// FSRemoveTree is POST /v1/fs/remove-tree's body: task 03's RemoveTree(root,
// path). Root is "lanes" or a lane; Path is under Root, and is a whole lane or a
// path below a lane in the creating set.
type FSRemoveTree struct {
	Root string `json:"root"`
	Path string `json:"path"`
}

// FSCopyHooks is POST /v1/fs/copy-hooks' body, MachineFS.CopyGeneratedHooks on
// the wire. ProjectTop is projects/<name>, CloneTop is a lane in the creating
// set, and Rel is a value lanes.CopyableHooksPath accepts.
type FSCopyHooks struct {
	ProjectTop string `json:"projectTop"`
	CloneTop   string `json:"cloneTop"`
	Rel        string `json:"rel"`
}

// FSCopied answers it: the files copied, 0 when a guard refused (not an error).
type FSCopied struct {
	Copied int `json:"copied"`
}

// CreatingTTL is how long a lane stays in the creating set with no seal (ruling 4).
const CreatingTTL = 5 * time.Minute
```

**The ask of phase 3, seam S8-P3.6** (env ruling 4, "The one S8-P3.6 contract", copied
verbatim; task 25 copies the same). Phase 3 owns the final shape; this one is **UNVERIFIED**
until phase 3 answers, and step 1 greps for what landed. `machined` serves it, and the laptop's
cloud implementation over these routes is task 25's.

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

| Route | Answer | Failures |
|---|---|---|
| `POST /v1/git` | `200 GitResult` | `400 bad_request` (a `Dir` that escapes the root or is none of the three forms in item 2, an unknown `Class` or `Scope`, `"clone"` included, a `Scope` that does not follow `Dir`, a `NewLane` that is not `Dir`, an `Env` key outside the allowlist, a `clone` outside its two shapes, a `NewLane` call outside its shapes, an argument that names a lane or the owned root (but the create clone's destination, and task 21b's one bundle path), empty `Args`, or `gitx.ErrCallShape` / `gitx.ErrLaneDir`; the message names the field, key or argument index, never a value) · `409 conflict` `project_config_untrusted` with `key` (H3 refused; git never ran) · `409 conflict` `lane_path_invalid` or `lane_not_creating` (git never ran) · `422 git_failed` with the three details · `413` above 16 MiB of stdin |
| `POST /v1/fs/lstat` | `200 FSStat` | `400` (not `lanes/<p>/<d>`) · `409 lane_path_invalid` |
| `POST /v1/fs/remove-tree` | `204`, also when nothing was there | `400` (a `Root` that is neither `lanes` nor a lane, or a `Path` not under `Root`) · `409 lane_path_invalid` · `409 lane_not_creating` (a path below a lane not in the set) |
| `POST /v1/fs/copy-hooks` | `200 FSCopied` | `400` · `409 lane_path_invalid` · `409 lane_not_creating` |
| `POST /v1/lanes/sealed` | `204`, idempotent | `400` (not `lanes/<p>/<d>`) |
| `GET /v1/worktrees/{worktreeId}/path` (task 20's) | unchanged | adds `409 lane_creating` while the lane is in the set |

**What `machined` does to every call (review I7, restated for R4).** No sandbox mounts
`projects/<name>` (R4.1; env ruling 5 runs a primary-worktree session in its backing lane), and
no lane git runs here (ruling 2). So this route takes three kinds of call, and no other:
- **project calls**: `Dir` `projects/<name>`, `Scope` `project`;
- **the add-project clone**: `Dir` `""`, `Scope` `none` (below);
- **a lane create's calls** (ruling 4), the only git that names a lane, inside R4.5's window
  before any pod mounts it (item 2).

The agent still reaches a project's *content*: a land fetches the lane's commits into it (task
21b), so tracked files and `.gitattributes` there are agent-written. Its `.git/config` and
`.git/hooks` are the user's, and may name drivers (Git LFS). The machine pod holds the whole disk
(every project, lane and conversation) and receives the user's host token, so a git that ran a
driver here would run it beside all of that. Phase 3 closes this for the laptop's git with §HGH.
`machined` runs §HGH unchanged, and adds the four things below.

1. **Phase 3's hardening, whole.** The runner is `gitx.NewExec(h)`, with
   `h = gitx.MachineHardening(ctx, "/tmp/machined-git")` and then `h.LaneRoot = <root>/lanes`,
   as phase 3's task 04 sets `LaneRoot` to its Machine's `CloneRoot()` (phase-3
   `task-02.md:115`, `:120-121`). Phase 3's own `ErrLaneDir` then refuses any call that runs in a
   lane but a create's `NewLane` call (`task-02.md:139-149`, `:523-524`), under `machined`'s own
   check (item 2). The owned root is on the machine pod's `emptyDir` `/tmp` (task 19), never on
   the disk, so no pod can reach its `no-hooks` directory. Every call then gets:
   - H1's command-scope prefix: hooks, `core.fsmonitor`, `core.sshCommand`, every transport but
     `file` (and `https` on a Remote call), `core.askPass`, gpg signing, submodule recursion,
     every repository credential helper, and `--upload-pack` / `--receive-pack`;
   - H2's machine-owned proxy and TLS on a Remote call;
   - H3's `git config --local --no-includes --list -z` check before git acts, on every
     `Scope: Project` call: the refusal list, which refuses `filter.*`, `diff.*.textconv`,
     `diff.*.command`, `diff.external`, `merge.*.driver`, `include.*`, `includeIf.*` and
     `extensions.worktreeConfig`, among others. `Scope: None` has no check (`task-02.md:534`).
     `machined` admits it for `version`, for the add-project clone (no repository exists yet),
     and for a create's `NewLane` writes, whose config at that moment holds only the six keys
     the clone's `--config` wrote and the closed writes before it (item 2);
   - H5's layout pin (`task-02.md:562-570`): Exec sets `GIT_DIR`, `GIT_COMMON_DIR` and
     `GIT_WORK_TREE` itself, so a planted `commondir` cannot swap the config that H3 checked.
     H5's read-only `.git/config` bind is hygiene (`task-02.md:556-560`), and `machined` does
     not depend on it: no agent can write a config this route reads, because no pod mounts a
     project (R4.1), and no pod mounts a lane in the creating set (task 20's path route answers
     `409 lane_creating` for it, below).

   `machined` writes no `-c` of its own. The two it wrote before this batch are H1's first two
   rows. It refuses to start (exit 2, naming the key) if `MachineHardening` finds any system or
   global credential helper, so the only credential a call can carry is the one in its `Env`.
2. **`Scope` follows `Dir`, and `machined` checks it before gitx sees the call.** `Dir` has
   three forms. Any other `Dir`, `Scope` `""`, `"clone"` or any unknown value is `400`:
   - **`projects/<name>`** (the name rule below): `Scope` must be `"project"`, `NewLane` `""`.
   - **`""`**: `Scope` must be `"none"`. **`Dir: ""` never means the disk root `/machine`.**
     `machined` runs it in `<OwnedRoot>/clone` (`/tmp/machined-git/clone`), an empty directory it
     creates at start beside phase 3's `no-hooks`, as phase 3's `GitHelper` runs in
     `<OwnedRoot>/credential` (phase-3 `task-13.md:108`). It is a subdirectory, not `OwnedRoot`
     itself, because phase 3's `None` rule admits "a `Dir` under `Hardening.OwnedRoot`"
     (`task-02.md:523`, `:534`), which does not settle whether the root itself counts. It is
     admitted for exactly two shapes: `Args == ["version"]`, and the add-project clone below.
   - **`lanes/<p>/<d>`**: task 20's lane-path check first (three segments, no `..`, each an
     `Lstat` directory; otherwise `409 lane_path_invalid`). Then `Scope` must be `"none"`,
     `NewLane` must equal `Dir`, and the path must be in the creating set (otherwise
     `409 lane_not_creating`, git never ran), and `Args` must be one of the create writes below.

   The laptop names the scope (task 25), but `machined` does not take it on trust, because the
   scope decides whether H3 runs.

   **Two `clone` shapes, one per scope.** Any call whose `Args[0]` is `clone` must be one of
   them; any other `clone` is `400`, naming the argument:
   - `Scope: "none"`: the add-project clone below.
   - `Scope: "project"`: **phase 3's lane create**, its `lanes.Creator` step 4 (phase-3
     `task-03.md:161-168`, built by the unexported `cloneArgv`, `:392`), exactly as env ruling 4
     states it. It is admitted when all of these hold:
     - `Dir` is `projects/<name>`, `Class` is `"local"` and `NewLane` is `""`;
     - `Env` **equals `lanes.NoFilterEnv`**, key for key and value for value, imported from
       phase 3 and never copied (`GIT_CONFIG_COUNT=3`, `filter.lfs.smudge=""`,
       `filter.lfs.process=""`, `filter.lfs.required=false`; `task-03.md:212-218`);
     - the options before `--` are exactly this set, each once, in any order: `--no-hardlinks`,
       `--branch <b>` with `<b>` not starting with `-`, `--origin origin`, and `--config` for
       exactly `core.autocrlf=false`, `core.eol=lf`, `core.checkStat=minimal`,
       `core.trustctime=false`, `core.longpaths=true` and `core.symlinks=false`;
     - after `--` come exactly two paths (`/machine` is `machined`'s root,
       `PANACEA_APP_MACHINED_ROOT`, task 20, and a test's temp root). The first equals
       `/machine/<Dir>`, so H3 checked the very repository being cloned. The second is
       `/machine/lanes/<a>/<b>`: two path elements, each non-empty, neither `.` nor `..`,
       holding no `/`, `\` or NUL; it does not exist yet; and its parent, with symlinks
       resolved, is still under `/machine/lanes`.

     The option list is closed because a clone's `--config` is written into the new clone
     **before its checkout**: `--config filter.x.smudge=<cmd>` would run on the machine pod
     during the clone itself. The destination is confined because a clone anywhere else would
     plant a repository in `projects/` (read under the Project rule) or in `/tmp/machined-git`
     (where `None` calls run unchecked). Once admitted, and **before git starts**, the
     destination joins the creating set (below). A change to phase 3's `cloneArgv` needs the
     matching change here first (seam S8-P3.3, task 33).

   **A create's writes in its new lane** (R4.5's window; `task-03.md:169-181`, `:191-198`).
   `Dir == NewLane`, a lane in the creating set, `Scope` `"none"`, `Env` empty, and `Args`
   exactly one of these; any other is `400`:
   - `config remote.origin.url <v>` with `v == lanes.NoFetchURL`, and
     `config remote.origin.pushurl <v>` with `v == lanes.NoPushURL` (`:207-210`);
   - `config user.name <v>` and `config user.email <v>`, `<v>` non-empty, holding no NUL or
     newline and not starting with `-` (step 5, `:174-177`);
   - `config core.hooksPath <v>` with `lanes.CopyableHooksPath(v)` (step 6, `:178-181`,
     `:220-225`);
   - `ls-files --others --exclude-standard -- <rel>` with `lanes.CopyableHooksPath(rel)`
     (CopyGeneratedHooks' check, `:253-256`).

   `None` skips H3, so this list *is* the check: the lane's config then holds only the clone's six
   keys and these values, none of which names a program, and H1 overrides `core.hooksPath` on
   every call anyway.

   **The creating set.** An in-memory map from a lane's relative path to a deadline. The entry
   is made when its clone is admitted, with no deadline while git runs, and gets
   `CreatingTTL` from the clone's exit, success or failure (so Create's failure path can still
   remove it). It closes on `POST /v1/lanes/sealed`, on its deadline, or on a whole-lane
   `remove-tree`. A restart empties it: an in-flight create's next write is
   `409 lane_not_creating`, and Create's failure path removes the directory, which needs no
   entry. After an entry closes, `machined` never runs git in that path again (R4.5).
3. **An explicit `Env` allowlist, derived from phase 3's call sites.** Every `Call.Env` phase 3
   writes, found by grepping its parts for `Env`, `GIT_CONFIG`, `GIT_AUTHOR`, `GIT_COMMITTER` and
   `commit-tree` at phase-3 `94f954d` (2026-09-26; the grep at `e816a59` had no `GIT_CONFIG`
   and missed task 24, seams review 1 I1): task 08's `forecastCommit` (identity and a
   fixed `DATE`, phase-3 `task-08.md:83-85`, because `commit-tree` has no date flag); task 09's
   land step 11 and its sync merge (`task-09.md:121`, `:366`) and task 10's revert
   (`task-10.md:132`), each a `commit-tree` with the user's identity in `GIT_AUTHOR_*` /
   `GIT_COMMITTER_*`; task 12's `remote.Auth.Env`, which task 13's `PushEnv` fills
   (`task-12.md:34-37`, `task-13.md:168-174`, `:190`); task 03's `NoFilterEnv` on the lane
   clone (`task-03.md:220-226`); and task 24's `FetchLane`/`ArchiveLane` step 5 fetch, which
   sets `transfer.fsckObjects=true` as `GIT_CONFIG_COUNT=1`, `KEY_0`, `VALUE_0`
   (`task-24.md:860-862`). Task 02's Exec adds its own headless
   defaults (`GIT_TERMINAL_PROMPT=0`, `GCM_INTERACTIVE=never`, `GIT_PAGER=cat`,
   `GIT_EDITOR=false`, and `GIT_OPTIONAL_LOCKS=0` on a Read, `task-02.md:188-190`) inside
   `machined`, so none of them needs to cross the wire. Task 05's `sessionEnv` is the sandbox's
   environment, not a `Call.Env`, and never reaches this route. So only these keys are admitted:
   - `GIT_CONFIG_COUNT`, a decimal from 1 to 4;
   - `GIT_CONFIG_KEY_<n>` and `GIT_CONFIG_VALUE_<n>`, both present, for each `n` below the count.
     Every `KEY_<n>` must be `http.https://<host>[:<port>]/.extraHeader` (compared lowercased),
     the form phase 3's `PushEnv` writes (phase 3 task 13, `PushEnv`), and `<host>` must be
     `RemoteURL`'s host. A second `GIT_CONFIG_*` set admitted is exactly
     `lanes.NoFilterEnv`, and only on the lane-create clone (item 2), where no other key may join
     it. The third is exactly `{GIT_CONFIG_COUNT=1, GIT_CONFIG_KEY_0=transfer.fsckObjects,
     GIT_CONFIG_VALUE_0=true}`, alone, and only on a `project`-scope, `local`-class call whose
     `Args` begin `fetch` and whose file argument is a live bundle's path (task 21b's one
     admitted path under the owned root). Before task 21b there is no live bundle, so this set
     is `400` on every call;
   - the six commit identity keys, `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_AUTHOR_DATE`,
     `GIT_COMMITTER_NAME`, `GIT_COMMITTER_EMAIL` and `GIT_COMMITTER_DATE`. Each is data that
     names no program and no path, so any value is admitted but one holding a NUL or a newline;
   - `GIT_TERMINAL_PROMPT` = `0`, and `GIT_OPTIONAL_LOCKS` = `0`.

   Everything else is `400`, naming the key. That includes `GIT_DIR`, `GIT_WORK_TREE`,
   `GIT_COMMON_DIR`, `GIT_EXEC_PATH`, `GIT_SSH_COMMAND`, `GIT_EXTERNAL_DIFF`,
   `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_GLOBAL`, `GIT_ASKPASS` and `LD_PRELOAD`. The review also
   allowed the bare `http.extraHeader`. It is refused here, for three reasons: it sends the header
   to every host a call reaches, phase 3 never writes it, and phase 3's own
   `TestPushEnvScopesTheHeaderToTheHost` treats it as the bug. A new `Call.Env` key in phase 3
   needs a matching row here before it can run on a cloud machine; task 33 records that
   dependency as seam S8-P3.3.

   A header pair is **credential `Env`**. It is admitted only on a `project`-scope Remote call,
   where H3's Remote rule also refuses every `http.*` key and every `credential.*` key but four,
   and on the add-project clone. A lane create carries none: its clone's `Env` is exactly
   `NoFilterEnv` and its writes' `Env` is empty, so a lane never receives the token. A lane needs
   none: its work reaches the project as a bundle (task 21b), a local fetch of a file.
4. **A scrubbed environment.** Phase 3's Exec starts git with `os.Environ()` plus the call's
   `Env` (its `Call.Env` comment). `machined`'s own environment holds
   `PANACEA_APP_MACHINED_TOKEN`, the bearer that drives every route on this machine. So
   `main.go` reads its configuration, then `os.Unsetenv`s **every** `PANACEA_APP_*` key before it
   listens, and no git, and nothing git starts, inherits one. What stays is Kubernetes'
   (`KUBERNETES_SERVICE_*`, `HOSTNAME`) and the image's (`PATH`, `HOME`). Unsetting in `machined`
   covers every child it starts and asks phase 3 for nothing. **Limit, stated:** a process that
   already runs as the pod's uid can still read `/proc/1/environ`, which keeps the variables
   `machined` was started with. The scrub stops inheritance. Stopping code from running in the
   first place is the job of items 1 to 3, and of ruling 2: no lane git, so no agent-defined
   filter, hook or config, ever runs in this pod.

The binary is fixed (`git` on the image's `PATH`), and nothing but git ever runs.

Responses are capped at 64 MiB of stdout; a call that would exceed it fails `git_failed` with
`gitSays: "output exceeds 64 MiB"`, so a runaway `git log` cannot exhaust the pod.

**Adding a project to a cloud machine** is this route with one fixed shape: `Scope: "none"`,
`Dir: ""` (`machined` runs it in `<OwnedRoot>/clone`, `/tmp/machined-git/clone`, item 2, a
`Dir` under `OwnedRoot`, where phase 3 admits `Scope None`), `Class: "remote"`, `RemoteURL` the https URL, and `Args` exactly
`["clone", "--", <RemoteURL>, "/machine/projects/<name>"]`. `<name>` must match
`^[a-z0-9][a-z0-9._-]{0,62}$`, and `projects/<name>` must not exist yet. The user's host token
arrives in `Env`, host-scoped to `RemoteURL` (from phase 3's credential store, which the Hub's C1
builds, ADR-H1 §5.2, `67609ab` `:344-352`). H3 has nothing to read, because no repository exists yet. The next
call in the new repository is a `project`-scope call, and H3 checks it then. Any other
`none`-scope call is `400`. The only other `clone` admitted is phase 3's lane create, a
`project`-scope call (item 2). No clone-specific route exists; the cockpit surface
that starts it belongs to whichever phase owns "add project" (recorded as seam S8-P3.2, task 33).

**The lane filesystem routes (`lanefs.go`, S8-P3.6's cloud half).** Each takes `machined`'s
bearer and paths relative to the root, and serves them through phase 3's **local** `MachineFS`
(the `os` one) on `/machine/<path>`, so every guard is phase 3's code, not a copy. `machined`
adds only the confinement, and runs no git:
- **`lstat`.** `Path` must pass task 20's `LanePathShapeOK`, else `400`. Each of `lanes` and
  `lanes/<p>` that exists must be a real directory by `Lstat`, else `409 lane_path_invalid`; an
  absent `lanes/<p>` answers `exists: false, dir: false`. The leaf is `Lstat`ed, so anything
  there, a symlink too, is `exists: true`, and `dir` is true only for a real directory. It needs
  no set entry: Create's step 3 runs before the clone.
- **`remove-tree`.** `Root` is `lanes` or a lane, and `Path` is under `Root`, else `400`. `Path`
  is a whole lane (the shape and parent checks of `lstat`), or a
  path below a lane in the creating set (CopyGeneratedHooks' take-back, `task-03.md:253-256`),
  else `409 lane_not_creating`. A whole lane is admitted in or out of the set, because phase 3
  task 11's Remove deletes a sealed lane with `RemoveTree` after its step 2a destroyed any
  runner-only sandbox (phase-3 `task-11.md:195-201`); env ruling 4 confines the route to
  `lanes/` and is silent on that caller. It closes the lane's entry. Nothing there is `204`. It
  must never follow a symlink, neither to delete nor to clear a mode.
- **`copy-hooks`.** `ProjectTop` is `projects/<name>` and a real directory, `CloneTop` is a lane
  in the creating set (else `409 lane_not_creating`), and `Rel` passes
  `lanes.CopyableHooksPath`, else `400`. The copy is phase 3's `CopyGeneratedHooks` half, with
  all its guards (`:227-252`).
- **`sealed`.** `Path` must pass the shape check. It closes the entry, and is `204` whether or
  not one was open. Phase 3's Create sends it through `MachineFS.Seal` after step 6 and
  before `Store.Add` (the seam's fourth method, above).

**The path route (`catalog.go`).** Task 20's `GET /v1/worktrees/{worktreeId}/path` answers
`409 conflict`, `details.reason "lane_creating"`, when the lane it resolved is in the creating
set, as a step between its shape check and its answer. So no pod mounts a lane while `machined`
may still run git in it, and a sealed lane is never run in again.

#### Tests to write first (`git_test.go`; real `git`, skipped with a message if absent)

Every test in `git_test.go` is named `TestGit…`, so `-run '^TestGit'` selects exactly them (the
old acceptance's `-run Git` missed every name without "Git" in it). "Git never ran" is observed
with a fake `git` first on `PATH` that writes a marker on any start. A marker script is a
`#!/bin/sh` file that writes a marker file; the owned root is a `t.TempDir()`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGitRunsInTheMachinePath` | `rev-parse --show-toplevel` with `Dir: "projects/a"`, `Scope: "project"` prints `<root>/projects/a` | a Dir resolved against the process's cwd |
| `TestGitDirOutsideTheRootIsRefused` | `Dir: "../x"`, `"/etc"` → 400, git never ran | git run anywhere on the pod |
| `TestGitRefusesGITDIRInEnv` | review I7 (d): `Env: {GIT_DIR: "/machine/projects/b/.git"}` on a `projects/a` call → 400 naming `GIT_DIR`, git never ran | a `Dir` confinement that `Env` undoes |
| `TestGitEnvIsAnExplicitAllowlist` | table, each → 400 naming the key and git never ran: `LD_PRELOAD`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_EXEC_PATH`, `GIT_SSH_COMMAND`, `GIT_EXTERNAL_DIFF`, `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_GLOBAL`, `GIT_ASKPASS`; `KEY_0=core.sshCommand`; the bare `KEY_0=http.extraHeader`; `COUNT=2` with one pair; `KEY_0` for `https://evil.example/` when `RemoteURL` is `https://github.com/…`; `GIT_TERMINAL_PROMPT=1`; `GIT_AUTHOR_NAME` holding a newline; the fsck set (`COUNT=1`, `KEY_0=transfer.fsckObjects`, `VALUE_0=true`) on a `status` call and on a `fetch` whose file is not a live bundle's path (seams review 1 I1; the admitted case is task 21b's `TestBundleFetchFsckEnvIsAdmittedOnlyThere`). Admitted (git ran): `GIT_OPTIONAL_LOCKS=0`; one host-scoped header pair; all six identity keys at once, with phase 3 task 08's forecast values | a `^GIT_` prefix match; a header sent to a host the call does not name; an allowlist that refuses phase 3's commit identity, so every forecast and land on a cloud machine fails |
| `TestGitCommitTreeTakesTheCallersIdentity` | a `projects/a` repository, `Scope: "project"`, `Class: "local"`: `commit-tree <its HEAD tree> -m "panacea forecast"` with the forecast's `Env` (`GIT_AUTHOR_NAME`/`GIT_COMMITTER_NAME` `panacea-forecast`, `EMAIL` `forecast@panacea.invalid`, `DATE` `@0 +0000`) → 200, and the id equals the one the same `commit-tree` prints when run directly with that environment on the test's own git; run twice, the same id | the identity keys dropped on the way to git (the id changes with the machine's clock and identity); phase 3's forecast not deterministic on a cloud machine |
| `TestGitExtraHeaderReachesGit` | `ls-remote` with `Scope: "project"`, `Class: "remote"`, against phase 3's `gitxtest.HTTPSRemote` stand-in that records `Authorization`, with `KEY_0=http.<RemoteURL's origin>/.extraHeader`, `VALUE_0=Authorization: Bearer t0k` → the stand-in saw `Bearer t0k` | phase 3's push credential path broken on cloud machines |
| `TestGitSecretsNeverLeaveInLogsOrErrors` | the same value, on a failing call → neither the log nor the problem body contains `t0k` | a host token in a log |
| `TestGitHooksNeverRun` | `projects/a` whose `.git/hooks/post-checkout` writes a marker file; `checkout -b x` through the route, `Scope: "project"` → no marker | a project's hooks executed on the machine pod |
| `TestGitProjectFilterNeverRuns` | `projects/a` whose `.git/config` sets `filter.x.clean=<marker script>`, with a landed `.gitattributes` `* filter=x` and one modified file; `status --porcelain`, `Scope: "project"` → 409 `conflict`, `reason: project_config_untrusted`, `key: filter.x.clean`, and no marker | a project call run without H3, so an agent-landed attribute drives a driver on the machine pod |
| `TestGitIncludedConfigIsRefused` | `projects/a` whose config sets only `include.path=../evil` (a file that sets the filter) → 409, `key: include.path`, no marker | an included file's filter read past the check |
| `TestGitScopeMustFollowDir` | each → 400, git never ran: `projects/a` with `"clone"` (R4.2 deleted it); `projects/a` with `"none"`; `projects/a` with `NewLane: "projects/a"`; `Dir: ""` with `"none"` and `Args: ["status"]`; `Dir: ""` with `"project"`; `Scope: ""`; a creating lane with `"project"`; `conversations/w-1` with `"project"` | the laptop's scope taken on trust; `"clone"` read as `Project`; `Dir: ""` read as the disk root under a checked scope |
| `TestGitHardeningNamesTheLaneRoot` | the `Hardening` `machined` builds for a root `R` has `LaneRoot == R/lanes` and `OwnedRoot` the owned root, read from the function `main` calls. Not observable through the route: `machined`'s own checks refuse every lane call first, which is the intended depth | `LaneRoot` left `""`, so phase 3's own lane guard is off on the one host that holds every lane |
| `TestGitNeverSeesTheMachinedToken` | `machined` started through the function `main` calls, with `PANACEA_APP_MACHINED_TOKEN` and `PANACEA_APP_LOG_LEVEL` set; one route call to a fake `git` that dumps its environment → no `PANACEA_APP_` key in it | the scrub dropped; a scrub of the token alone; a scrub in a helper `main` never calls |
| `TestGitStartRefusesAMachineCredentialHelper` | a `HOME` whose `.gitconfig` sets `credential.helper=store` → start exits 2 naming `credential.helper` | a machine helper answering for the user's host |
| `TestGitAddProjectCloneHasOneShape` | the exact shape → the fake `git` saw `clone -- <url> /machine/projects/n` with its cwd `<owned root>/clone`, never `/machine`; an existing `projects/n`, a name with `/`, an extra argument, `Class: "read"` → each 400, git never ran | a `none`-scope call used to skip H3; `Dir: ""` resolved to the disk root |
| `TestGitLaneCreateCloneIsAdmitted` | real git: `projects/a` a repository with branch `main`; phase 3's exact argv, `clone --no-hardlinks --config core.autocrlf=false --config core.eol=lf --config core.checkStat=minimal --config core.trustctime=false --config core.longpaths=true --config core.symlinks=false --branch main --origin origin -- /machine/projects/a /machine/lanes/p-1/ui-0a1b2c` (with the test's root for `/machine`), `Dir: "projects/a"`, `Scope: "project"`, `Class: "local"`, `Env` `lanes.NoFilterEnv` → 200; `lanes/p-1/ui-0a1b2c/.git` is a directory whose `core.eol` is `lf` and `core.symlinks` `false`; and the path is in the creating set (task 20's path route answers `409 lane_creating`) | the one-shape rule keyed on `Args[0] == "clone"`, which refuses every lane create on a cloud machine; the four-key shape of the pre-R4 plan; an admitted clone that makes no set entry, so its writes are refused |
| `TestGitCloneOutsideItsTwoShapesIsRefused` | table, each `Scope: "project"`, `Dir: "projects/a"` → 400 and git never ran: destination `/machine/projects/b`; destination `/tmp/machined-git/x`; destination `/machine/lanes/p-1` (one element); destination `/machine/lanes/p-1/../../projects/b`; an existing destination; source `/machine/projects/b` (not the call's `Dir`); `--config filter.x.smudge=<marker script>`; `--template=/machine/projects/a`; `--separate-git-dir=/tmp/x`; the pre-R4 four `--config` keys only; `core.symlinks=true`; `core.eol=lf` twice; a seventh `--config core.fsmonitor=<marker script>`; `Class: "remote"`; `Env` empty; `Env` `NoFilterEnv` plus one host-scoped header pair; `NoFilterEnv` with `filter.lfs.required=true`; `NewLane` set to the destination | a lane create that plants a repository where H3 reads it under the wrong rule or not at all; a clone-time filter run before H3 first reads the clone; the user's token in a lane create |
| `TestGitNewLaneWritesAreAClosedList` | a creating lane `L` (made by an admitted clone). Admitted, each 200 and the value then read back: `config remote.origin.url <NoFetchURL>`, `config remote.origin.pushurl <NoPushURL>`, `config user.name "A B"`, `config user.email a@b.invalid`, `config core.hooksPath .husky/_`, `ls-files --others --exclude-standard -- .husky/_`. Refused, each 400 and git never ran: `config remote.origin.url https://x.invalid/r`, `config filter.x.smudge <marker script>`, `config core.fsmonitor x`, `config core.hooksPath ../x`, `config user.name -x`, `config user.email` with a newline, `status`, `ls-files -- ../x`, any admitted shape with a non-empty `Env`, `Scope: "project"` | a `NewLane` that opens any git in a lane, where `None` skips H3; a create write that plants a driver the sandbox then runs |
| `TestGitLaneDirOutsideTheCreatingSetIsRefused` | each 409 `lane_not_creating`, git never ran: an admitted write shape on a lane never cloned by this `machined`; on `L` after `sealed`; on `L` after its deadline (fake clock); on `L` after a restart. `NewLane` ≠ `Dir` → 400. A creating lane whose `lanes/p-1` was swapped for a symlink → 409 `lane_path_invalid` | a creating set that never closes, so the host runs git in a lane an agent has mounted (R4.2, R4.5) |
| `TestGitServesPhase3sCreateSequence` | real git: phase 3's Create call sequence for `ui` on `projects/a` (a husky project with `core.hooksPath .husky/_`), replayed as wire calls in `task-03.md:148-198`'s order: `rev-parse --verify --quiet`; `fs/lstat`; the clone; step 4a's two writes; `var GIT_AUTHOR_IDENT`; step 5's two writes; `config --local --get core.hooksPath`; its write; `fs/copy-hooks`; `ls-files`; `sealed` → each 200 or 204; the lane's origin is `NoFetchURL`, its `.husky/_` files match the project's, and task 20's path route then answers the lane path | one shape that refuses a step of phase 3's real create, which only a full sequence shows |
| `TestGitFailureIsGitFailedWithGitsOwnSentence` | `rev-parse nosuchref` → 422, `details.gitSays` begins `fatal:`, `exitCode` non-zero, no `args` in details | a generic 500; argv (which may hold a URL) in the body |
| `TestGitOKExitIsAnAnswer` | a `merge-tree --write-tree` conflict with `OKExit: [1]` → 200, `exitCode: 1` | a forecast conflict reported as a failure |
| `TestGitStdinReachesGit` | `hash-object --stdin` with stdin `x` → the known hash | stdin dropped (commit-tree -F - in phase 3) |
| `TestGitReadCallsTakeNoOptionalLocks` | a `Class: "read"` call's environment includes `GIT_OPTIONAL_LOCKS=0` (via a fake `git` on `PATH` that dumps its env) | a status racing an agent for `index.lock` |
| `TestGitOutputIsCapped` | a fake `git` writing 65 MiB → 422 `output exceeds 64 MiB` | a pod killed by a runaway log |

**`lanefs_test.go`** (every test is named `TestLaneFS…`; a creating lane `L` is made by an
admitted clone through `POST /v1/git`, with real `git` unless the row names a fake one; no
route in `lanefs.go` starts git, and each test's route calls are made with a marker `git` first
on `PATH` that must stay silent):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLaneFSExistsIsLstat` | `POST /v1/fs/lstat`: absent leaf → `{exists: false, dir: false}`; a directory → `{true, true}`; a dangling symlink → `{true, false}`; a symlink to a directory → `{true, false}`; a regular file → `{true, false}`; an absent `lanes/p-1` → `{false, false}`; `lanes/p-1` a symlink → 409 `lane_path_invalid`; `projects/a`, `lanes/p-1`, `lanes/p-1/d/e`, `lanes/../x` → 400 | an `os.Stat` exists, so Create's step 3 takes a planted link's path as free; `dir` from `os.Stat`, so `Catalog.Path` verifies a link to a directory as a lane; a shape check that admits `projects/` |
| `TestLaneFSRemoveTreeTakesAWholeLaneOrBelowACreatingOne` | `POST /v1/fs/remove-tree`: a whole sealed lane with `root: "lanes"` → 204 and gone; `<L>/.husky/_` below a creating `L` with `root: <L>` → 204; the same below a sealed lane → 409 `lane_not_creating`; `path: projects/a` → 400; `root: projects`, and a `path` not under its `root` → 400; nothing there → 204; after a whole-lane removal of a creating `L`, its entry is gone | task 11's Remove refused on a cloud machine; a take-back that reaches into a lane an agent has mounted; a route that deletes a project; a `root` taken on trust |
| `TestLaneFSRemoveTreeNeverFollowsASymlink` | a lane holding `x → ../../../projects/a` and `f → ../../../projects/a/README` (mode `0400`) → 204; `projects/a` and its `README` intact, still mode `0400` | a delete, or a read-only-clearing walk, that follows an agent's link out of `lanes/`. If phase 3's local `RemoveTree` fails it, the fix is phase 3's (step 1) |
| `TestLaneFSCopyHooksOnlyIntoACreatingLane` | a project with `.husky/_/pre-commit` and a creating `L` → `copied: 1`, the bytes equal; a sealed lane → 409 `lane_not_creating`; `Rel` `../x` or `/x` → 400; `Project` `lanes/p-1/d` → 400 | a copy into a live lane; a copy source outside `projects/` |
| `TestLaneFSSealClosesTheWindow` | a creating `L`: `sealed` → 204, again → 204; then an admitted write shape → 409 `lane_not_creating`, and task 20's path route answers the lane path | a seal that leaves the window open; a seal the laptop's retry turns into an error |
| `TestLaneFSCreatingEntryExpires` | fake clock and a fake `git` that blocks: while the clone runs, past `CreatingTTL` → still creating; after it exits, `CreatingTTL` + 1 s → a write is 409 and the path route answers | a deadline counted from admission, which fails a large clone; an entry that outlives a laptop that died mid-create |
| `TestLaneFSPathRouteIsLaneCreatingWhileInTheSet` | a creating `L` → task 20's route answers 409 `conflict`, `reason: lane_creating`; a sealed lane → 200 | a pod mounting a lane while `machined` may still run git in it |

#### Steps

- [ ] **Step 1: Dependencies, all phase 3's.** Each command must print what it says, or stop and report which is missing (this is a dependency wait on phase 3, not a re-plan):
  - task 02 **with §HGH**: `git grep -n "type Runner interface\|func MachineHardening\|type UntrustedConfig\|LaneRoot " -- control-plane/internal/gitx/` prints at least four lines, `NewLane` among `Call`'s fields;
  - task 03: `git grep -n "NoFilterEnv\|NoFetchURL\|NoPushURL\|func CopyableHooksPath\|func RemoveTree" -- control-plane/internal/lanes/` prints all five names;
  - seam S8-P3.6: `git grep -n "type MachineFS interface" -- control-plane/internal/gitx/` prints one line, and its methods and local implementation are the ones above. Record the names that landed in the commit message; where they differ from this part's, phase 3's win, and the routes keep their confinement.
- [ ] **Step 2:** Wire types; tests, each shown red first; `git.go` over `gitx.NewExec(h)` with `h.LaneRoot` set; `lanefs.go` over phase 3's local `MachineFS`; `catalog.go`'s `lane_creating` step; the scrub in `main.go`; all pass.
- [ ] **Step 3: Mutation check** (rule 7), each on a `$TEMP` copy, each turning the named test red:
  (1) the `Env` allowlist replaced by `^GIT_[A-Z0-9_]+$` → `TestGitEnvIsAnExplicitAllowlist`,
  on its `GIT_EXEC_PATH`, `GIT_SSH_COMMAND` and `GIT_CONFIG_PARAMETERS` rows. Not
  `TestGitRefusesGITDIRInEnv`: `GIT_DIR` is refused twice, by `machined` and then by phase 3's
  H5 (`ErrCallShape`, also a `400`), so no single mutant turns that test red. That is the
  intended depth, and the reason this mutant is checked against the other test;
  (2) the bare `http.extraHeader` admitted → `TestGitEnvIsAnExplicitAllowlist`;
  (3) the header host not compared with `RemoteURL`'s → `TestGitEnvIsAnExplicitAllowlist`;
  (4) the create clone's `Env` accepted as any superset of `NoFilterEnv` →
  `TestGitCloneOutsideItsTwoShapesIsRefused`, on its `NoFilterEnv`-plus-header row;
  (5) the wire `Scope` passed through unchecked → `TestGitScopeMustFollowDir`;
  (6) `"clone"` mapped to `gitx.Project` → `TestGitScopeMustFollowDir`, on its `"clone"` row;
  (7) only `PANACEA_APP_MACHINED_TOKEN` unset → `TestGitNeverSeesTheMachinedToken`;
  (8) the scrub moved into a helper that `main` does not call → `TestGitNeverSeesTheMachinedToken`;
  (9) the identity keys dropped from the allowlist → `TestGitEnvIsAnExplicitAllowlist` (its
  admitted row) and `TestGitCommitTreeTakesTheCallersIdentity`;
  (10) the identity keys admitted but not passed to git → `TestGitCommitTreeTakesTheCallersIdentity`;
  (11) the clone rule keyed on `Args[0] == "clone"` alone (every clone but the add-project one
  refused) → `TestGitLaneCreateCloneIsAdmitted`;
  (12) the lane-create destination check dropped → `TestGitCloneOutsideItsTwoShapesIsRefused`,
  on its `projects/b` and `/tmp/machined-git/x` rows;
  (13) the lane-create option list opened to any `--config` → `TestGitCloneOutsideItsTwoShapesIsRefused`,
  on its `filter.x.smudge` and `core.fsmonitor` rows;
  (14) the six `--config` keys checked by key, not value → `TestGitCloneOutsideItsTwoShapesIsRefused`,
  on its `core.symlinks=true` row;
  (15) `Dir: ""` resolved to the machine root → `TestGitAddProjectCloneHasOneShape` (the cwd);
  (16) the creating-set check dropped (any `lanes/` `Dir` with `NewLane == Dir` runs) →
  `TestGitLaneDirOutsideTheCreatingSetIsRefused`, on its sealed row;
  (17) the `NewLane` write list opened to any `config` → `TestGitNewLaneWritesAreAClosedList`,
  on its `filter.x.smudge` row;
  (18) `sealed` answering 204 without closing the entry → `TestLaneFSSealClosesTheWindow`;
  (19) the deadline counted from the clone's admission → `TestLaneFSCreatingEntryExpires`;
  (20) `h.LaneRoot` left `""` → `TestGitHardeningNamesTheLaneRoot`;
  (21) `remove-tree` below a lane admitted without the set → `TestLaneFSRemoveTreeTakesAWholeLaneOrBelowACreatingOne`;
  (22) `lstat` by `os.Stat` → `TestLaneFSExistsIsLstat`, on its dangling-symlink and link-to-directory rows.
  The two named beforehand as likeliest to survive are (8), because a test of the helper alone
  passes, and (20), because no route call can observe it: `machined`'s own checks refuse every
  lane call before phase 3's guard sees one. Each is killed only by the assertion named beside
  it: a start through `main`'s own function, and the `Hardening` read from that same function.
- [ ] **Step 4:** Full run; commit.

#### Acceptance

```bash
go test ./control-plane/cmd/machined/ -run '^(TestGit|TestLaneFS)' -race -count=1 -v > "$TMPDIR/t21.log" 2>&1; echo "exit=$?"   # exit=0; a build failure is non-zero, never a count of 0
grep -c "^--- PASS" "$TMPDIR/t21.log"                                                          # 32 (25 TestGit rows and 7 TestLaneFS rows above; or SKIP lines without git)
grep -c "^--- SKIP" "$TMPDIR/t21.log"                                                          # 0 on a host with git
grep -rn "gitrunner\|/git/v1/" control-plane/cmd/machined/ ; echo "grep=$?"                     # grep=1: machined serves no lane runner (ruling 2)
go test ./sandboxd/... ./control-plane/... -race -count=1                                       # ok
```

#### What this task does not claim

- That phase 3's `gitx` behaves identically on the machine pod's Linux git and the laptop's
  Windows git (`MinVersion` 2.40 is checked by `gitx.CheckVersion` at `machined` start; the sandbox
  image's Debian git version, trixie's after task 27a, is **UNVERIFIED** against it — task 27's
  `git version ≥ 2.40.0` row checks it at build).
- A clone surface in the cockpit.
- That code the agent chose can never run in the machine pod. The claim is narrower: no lane git
  runs here (ruling 2), and for the project content an agent lands, every path that phase 3's
  §HGH names, and every `Env` path, is closed and tested. A git feature that §HGH does not know
  of is a new finding for phase 3's list, and it would then run with the whole disk in reach. The
  scrub hides `machined`'s token from children but not from `/proc/1/environ` (item 4).
- That `objects/info/alternates` is checked. It can lend a project another repository's objects,
  but not its refs, and H5 pins the layout that holds the refs. **UNVERIFIED** beyond that
  reading of git's documented layout.
- That phase 3 accepts seam S8-P3.6 as shaped here, `Seal` and `dir` included (**UNVERIFIED**; step 1
  greps). Or that phase 3's local `RemoveTree` follows no symlink on Linux: its doc says it
  clears read-only attributes during the walk (phase-3 `task-03.md:264-268`), and
  `TestLaneFSRemoveTreeNeverFollowsASymlink` is the first place that is checked.
- That the creating set survives a restart. It does not; the create in flight fails, and cleans
  up through `remove-tree`.
- Bundles: the routes, the bundle argument on `POST /v1/git` and the sweep are task 21b's.
