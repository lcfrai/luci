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
compromise of it is contained to one user's disk and one microVM.

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

import "github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"

// The disk layout, relative to PANACEA_APP_MACHINED_ROOT. P8-D6.
const (
	DirProjects      = "projects"      // project repositories: projects/<name>
	DirLanes         = "lanes"         // phase 3 lane clones: lanes/<projectId>/<worktreeId> (S3-P8.2)
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

// WorktreePath answers GET /v1/worktrees/{worktreeId}/path. MachinePath is
// relative to the root — the form backend.MachineMount takes (task 15).
// GitConfig is MachinePath + "/.git/config" when, read with Lstat (no symlink
// followed), MachinePath/.git is a directory and its config a regular file;
// otherwise "". Task 24b binds it read-only over the sandbox's
// /home/dev/work/.git/config (phase 3 §HGH H5, review I7). A .git directory
// whose config is missing or is not a regular file is 409 conflict with
// details.reason "git_config_not_a_file": a Kubernetes subPath that does not
// exist is created as a directory, and one that is a symlink binds its target
// (Kubernetes' documented behaviour, UNVERIFIED here: nothing runs on a cluster).
type WorktreePath struct {
	MachinePath string `json:"machinePath"`
	GitConfig   string `json:"gitConfig,omitempty"`
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
| `GET /v1/worktrees/{worktreeId}/path` | `200 WorktreePath` | 404 `worktree_not_found` · 409 `conflict`, `details.reason: git_config_not_a_file` |
| `POST /v1/dirs` | `204` | 400 `bad_request` — an escaping or absolute path, naming it |
| `DELETE /v1/conversations/{worktreeId}/{conversationId}` | `204` — removes every `<conversationId>.jsonl` under `conversations/<worktreeId>/` (Claude Code writes them one directory down, e.g. `-home-dev-work/`, per `internal/api/sessions.go:38-43`); `204` also when none existed | 400 if either id fails its pattern (`^w-[a-f0-9]{8}$`, a UUID) |
| `DELETE /v1/conversations/{worktreeId}` | `204` — removes the whole store (worktree removal, OD-10) | 400 |

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
| `TestWorktreePathIsRelativeToTheRoot` | `machinePath == "projects/a"` for the primary worktree, no leading `/` | an absolute path that fails task 15's validation |
| `TestWorktreePathNamesTheGitConfigOnlyWhenItIsAFile` | a clone → `gitConfig == "projects/a/.git/config"`; a worktree whose `.git` is a file → `gitConfig` absent; `.git/config` replaced by a symlink to `../../b/.git/config`, or removed → 409 `git_config_not_a_file` | a subPath bind that follows an agent's symlink, or that Kubernetes creates as a directory (review I7, b06 re-review 1) |
| `TestAnUnknownWorktreeIs404` | → `worktree_not_found` | a 500 |

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
  Scanner over `<root>/projects`; `Path` stripped of the root prefix), `dirs.go`.
- [ ] **Step 3:** Full run; `CGO_ENABLED=0 GOOS=linux go build ./control-plane/cmd/machined`
  succeeds (task 27 builds it this way); commit.

#### Acceptance

```bash
go test ./control-plane/cmd/machined/ ./control-plane/internal/machineapi/ -race -count=1   # ok (catalog tests skip without git)
CGO_ENABLED=0 GOOS=linux go build -o /tmp/machined ./control-plane/cmd/machined            # builds
go test ./sandboxd/... ./control-plane/... -race -count=1                                   # ok
```

#### What this task does not claim

- Phase 3's composite catalog (lanes beside `git worktree list`, its task 11): `machined` serves
  phase 2's Scanner until task 25 switches it to phase 3's enumerator, which is the seam
  S3-P8.2 names.
- That `git` in the sandbox image behaves inside Kata as on Docker — the sandbox-git fix (sibling
  `sandbox-git-fix` worktree, `2d6d95b`) changed the image for the corporate CA; task 27 builds on
  whatever image is current.
