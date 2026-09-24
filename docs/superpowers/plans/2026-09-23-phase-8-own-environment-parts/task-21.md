### Task 21: `machined` git — phase 3's runner, on the machine

Phase 3 keys every git call by machine (its brief A6): `gitx.Machine` has one implementation,
`gitx.Local`, and "Phase 8 adds a Kata machine behind the same four methods: a pod gets a clone,
FetchURL names it, and the control plane fetches from it" (phase 3 plan, sibling `phase-3`
worktree, `2026-09-23-phase-3-git-convergence-parts/task-02.md:146-159`, seam S3-P8.1). On a
cloud machine the project repository and its lane clones share one disk (OD-2, P8-D6), so the
fetch-back is local **to the machine** — git just has to run there. This task gives `machined` one
route that runs **phase 3's own `gitx.Exec`** on the disk; task 25 wraps it as the laptop's
`gitx.Runner`.

**Depends on phase 3 task 02, including its §HGH hardening** (`internal/gitx`: `Call` with
`Scope` and `RemoteURL`, `Result`, `Runner`, `Exec`, `Hardening`, `MachineHardening`,
`UntrustedConfig`, `ErrCallShape`, `Error`, `Class` — sibling `phase-3` worktree, task-02.md
`#### Interfaces — complete`; H1–H5 and the `TestHGH…` tests under `#### Host git hardening
(HGH)`, read at phase 3's `93c1fdf`; that file is still being edited, so sections are cited by
heading, not line). If it has not landed, this task does not start: no local copy of
`gitx` is written, and no second hardening is invented here.

**Files:**
- `control-plane/internal/machineapi/wire.go` — `GitCall`, `GitResult` (modify)
- `control-plane/cmd/machined/git.go`, `git_test.go` (new)
- `control-plane/cmd/machined/server.go` — the route (modify)
- `control-plane/cmd/machined/main.go` — the environment scrub below (modify)

#### Contract (complete)

```go
package machineapi

// GitCall is POST /v1/git's body: gitx.Call on the wire. A non-empty Dir is
// relative to the machine root (/machine). Dir "" is the one exception: it maps
// to machined's own <OwnedRoot>/clone (/tmp/machined-git/clone), never to
// /machine, and it is admitted only with Scope "none".
// Class is "read" | "local" | "remote" (gitx.Read/Local/Remote).
// Scope is "clone" | "project" | "none" (gitx.Clone/Project/None, phase 3 §HGH
// H3); it has no default, and it must follow Dir (below). RemoteURL is
// gitx.Call.RemoteURL: required for Class "remote", an https URL.
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
//   details.reason    "clone_config_untrusted" | "project_config_untrusted"
//   details.key       the refused key, lowercased; never its value
const (
	DetailGitSays  = "gitSays"
	DetailExitCode = "exitCode"
	DetailTimedOut = "timedOut"
	DetailReason   = "reason"
	DetailKey      = "key"
)
```

| Route | Answer | Failures |
|---|---|---|
| `POST /v1/git` | `200 GitResult` | `400 bad_request` (a `Dir` that escapes the root, an unknown `Class` or `Scope`, a `Scope` that does not follow `Dir`, an `Env` key outside the allowlist, credential `Env` on a `clone`-scope call, a `clone` outside its two shapes, empty `Args`, or `gitx.ErrCallShape`; the message names the field or key, never a value) · `409 conflict` with `reason` and `key` (phase 3's H3 refused the repository's config; git never ran) · `422 git_failed` with the three details · `413` above 16 MiB of stdin |

**What `machined` does to every call (review I7).** A lane clone's `.git` is inside that lane's
session mount. So is a project's `.git` whenever a session runs on the project's primary
worktree: its `machinePath` is `projects/<name>` (task 20, `TestWorktreePathIsRelativeToTheRoot`),
and task 24b mounts the worktree's machine path read-write at `/home/dev/work`. The agent can
therefore write any key into either `config`, any file into either `hooks/`, and any
`.gitattributes`. A git that obeyed them would run code the agent chose **in the machine pod**,
which holds the whole disk (every project, lane and conversation, where a session sees only its
own sub-paths) and receives the user's host token. Phase 3 closes the same hole for the laptop's
git with §HGH. `machined` runs §HGH unchanged, and adds the four things below that the machine
needs.

1. **Phase 3's hardening, whole.** The runner is `gitx.NewExec(h)`, with
   `h = gitx.MachineHardening(ctx, "/tmp/machined-git")`. That root is on the machine pod's own
   `emptyDir` `/tmp` (task 19), never on the disk, so no session can reach its `no-hooks`
   directory. Every call then gets:
   - H1's command-scope prefix: hooks, `core.fsmonitor`, `core.sshCommand`, every transport but
     `file` (and `https` on a Remote call), `core.askPass`, gpg signing, submodule recursion,
     every repository credential helper, and `--upload-pack` / `--receive-pack`;
   - H2's machine-owned proxy and TLS on a Remote call;
   - H3's `git config --local --no-includes --list -z` check before git acts. For
     `Scope: Clone` that is a closed allowlist. For `Scope: Project` it is the refusal list,
     which refuses `filter.*`, `diff.*.textconv`, `diff.*.command`, `diff.external`,
     `merge.*.driver`, `include.*`, `includeIf.*` and `extensions.worktreeConfig`, among others;
   - H5, both halves. The layout pin: Exec sets `GIT_DIR`, `GIT_COMMON_DIR` and
     `GIT_WORK_TREE` itself, so a planted `commondir` cannot swap the config that H3 checked.
     And the read-only config (phase-3 `task-02.md:482-487`): H3 reads `.git/config` a few
     milliseconds before git does, so an agent that rewrites it inside that window would defeat
     H3. Phase 3 closes that window by binding `<path>/.git/config` read-only in the sandbox, in
     its task 05's `sessionMounts`, which builds only the laptop's Docker mounts. A cloud session
     gets its mounts from task 24b instead, so **task 24b binds that file read-only** (from
     task 20's `WorktreePath.GitConfig`), and `machined` depends on it: without the bind, a lane
     or primary-worktree agent that loops a `filter.*` write and removal would sometimes pass H3
     with a config git then reads dirty.

   `machined` writes no `-c` of its own. The two it wrote before this batch are H1's first two
   rows. It refuses to start (exit 2, naming the key) if `MachineHardening` finds any system or
   global credential helper, so the only credential a call can carry is the one in its `Env`.
2. **`Scope` follows `Dir`, and `machined` checks it.** A `Dir` under `lanes/` must be
   `"clone"`, a `Dir` equal to `projects/<name>` must be `"project"`, and `Dir: ""` must be
   `"none"`. Any other `Dir` is `400`. **`Dir: ""` never means the disk root `/machine`.**
   `machined` runs a `none`-scope call in `<OwnedRoot>/clone` (`/tmp/machined-git/clone`), an
   empty directory it creates at start beside phase 3's `no-hooks`, as phase 3's `GitHelper`
   runs in `<OwnedRoot>/credential` (phase-3 `task-13.md:108`). It is a subdirectory, not
   `OwnedRoot` itself, because phase 3's `None` rule admits "a `Dir` under
   `Hardening.OwnedRoot`" (phase-3 `task-02.md:452`), and that wording does not settle whether
   the root itself counts. The laptop's `Call` for such a call carries `Dir: ""` (task 25).
   `"none"` is admitted for exactly two shapes: `Args == ["version"]`, and the add-project clone
   below. Any other pairing is `400` before gitx sees the call. The laptop names the scope
   (task 25), but `machined` does not take it on trust, because the scope decides whether a lane
   is read against the closed allowlist and whether the token may enter it.

   **Two `clone` shapes, one per scope.** Any call whose `Args[0]` is `clone` must be one of
   them; any other `clone` is `400`, naming the argument:
   - `Scope: "none"`: the add-project clone below.
   - `Scope: "project"`: **phase 3's lane create** (its `lanes.Creator` step 4, built by the
     unexported `cloneArgv(branch, src, dst)`, phase-3 `task-03.md:157-160`, `:286`; run with
     `Dir` the project and `Scope Project`, `:179-181`). It is admitted when all of these hold:
     `Dir` is `projects/<name>`; `Class` is `"local"`; `Env` is empty (a lane create carries no
     credential); the options before `--` are drawn only from `--no-hardlinks`, `--origin origin`,
     `--branch <b>` with `<b>` not starting with `-`, and `--config <k>=<v>` with `<k>` one of the
     four clone keys H3's Clone allowlist admits (`core.autocrlf`, `core.eol`,
     `core.checkStat`, `core.trustctime`); after `--` come exactly two paths (`/machine` below is
     `machined`'s root, `PANACEA_APP_MACHINED_ROOT`, task 20, and a test's temp root); the first
     equals `/machine/<Dir>`, so H3 checked the very repository being cloned; and the second is
     `/machine/lanes/<a>/<b>`, two path elements, each non-empty, neither `.` nor `..`, holding no
     `/`, `\` or NUL, that does not exist yet, and whose parent, with symlinks resolved, is still
     under `/machine/lanes`. The option list is closed because a clone's `--config` is written
     into the new clone **before its checkout**, and H3 first reads that clone on the next call:
     `--config filter.x.smudge=<cmd>` would run on the machine pod during the clone itself. The
     destination is confined because a clone anywhere else would plant a repository in
     `projects/` (read under the Project rule) or in `/tmp/machined-git` (where `None` calls run
     unchecked). A change to phase 3's `cloneArgv` needs the matching change here first (seam
     S8-P3.3, task 33).
3. **An explicit `Env` allowlist, derived from phase 3's call sites.** Every `Call.Env` phase 3
   writes, found by grepping its parts for `Env`, `GIT_AUTHOR`, `GIT_COMMITTER` and
   `commit-tree` at phase-3 `e816a59` (2026-09-24): task 08's `forecastCommit` (identity and a
   fixed `DATE`, phase-3 `task-08.md:55-57`, because `commit-tree` has no date flag); task 09's
   land step 11 and its sync merge (`task-09.md:97`, `:206`) and task 10's revert
   (`task-10.md:89`), each a `commit-tree` with the user's identity in `GIT_AUTHOR_*` /
   `GIT_COMMITTER_*`; and task 12's `remote.Auth.Env`, which task 13's `PushEnv` fills
   (`task-12.md:34-37`, `task-13.md:168-174`, `:190`). Task 02's Exec adds its own headless
   defaults (`GIT_TERMINAL_PROMPT=0`, `GCM_INTERACTIVE=never`, `GIT_PAGER=cat`,
   `GIT_EDITOR=false`, and `GIT_OPTIONAL_LOCKS=0` on a Read, `task-02.md:150-154`) inside
   `machined`, so none of them needs to cross the wire. Task 05's `sessionEnv` is the sandbox's
   environment, not a `Call.Env`, and never reaches this route. So only these keys are admitted:
   - `GIT_CONFIG_COUNT`, a decimal from 1 to 4;
   - `GIT_CONFIG_KEY_<n>` and `GIT_CONFIG_VALUE_<n>`, both present, for each `n` below the count.
     Every `KEY_<n>` must be `http.https://<host>[:<port>]/.extraHeader` (compared lowercased),
     the form phase 3's `PushEnv` writes (phase 3 task 13, `PushEnv`), and `<host>` must be
     `RemoteURL`'s host;
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

   Any `GIT_CONFIG_*` key is **credential `Env`**. A `clone`-scope call that carries one is `400`,
   so a lane, where every config key is the agent's, never receives the token. A lane needs none:
   phase 3 fetches a lane back into its project by the lane's literal path (H2), a local fetch.
   Credential `Env` is admitted only on a `project`-scope Remote call, where H3's Remote rule also
   refuses every `http.*` key and every `credential.*` key but four, and on the add-project clone.
4. **A scrubbed environment.** Phase 3's Exec starts git with `os.Environ()` plus the call's
   `Env` (its `Call.Env` comment). `machined`'s own environment holds
   `PANACEA_APP_MACHINED_TOKEN`, the bearer that drives every route on this machine. So
   `main.go` reads its configuration, then `os.Unsetenv`s **every** `PANACEA_APP_*` key before it
   listens, and no git, and nothing git starts, inherits one. What stays is Kubernetes'
   (`KUBERNETES_SERVICE_*`, `HOSTNAME`) and the image's (`PATH`, `HOME`). Unsetting in `machined`
   covers every child it starts and asks phase 3 for nothing. **Limit, stated:** a process that
   already runs as the pod's uid can still read `/proc/1/environ`, which keeps the variables
   `machined` was started with. The scrub stops inheritance. Stopping code from running in the
   first place is the job of items 1 to 3.

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

#### Tests to write first (`git_test.go`; real `git`, skipped with a message if absent)

Every test in `git_test.go` is named `TestGit…`, so `-run '^TestGit'` selects exactly them (the
old acceptance's `-run Git` missed every name without "Git" in it). "Git never ran" is observed
with a fake `git` first on `PATH` that writes a marker on any start. A marker script is a
`#!/bin/sh` file that writes a marker file; the owned root is a `t.TempDir()`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGitRunsInTheMachinePath` | `rev-parse --show-toplevel` with `Dir: "projects/a"`, `Scope: "project"` prints `<root>/projects/a` | a Dir resolved against the process's cwd |
| `TestGitDirOutsideTheRootIsRefused` | `Dir: "../x"`, `"/etc"` → 400, git never ran | git run anywhere on the pod |
| `TestGitRefusesGITDIRInEnv` | review I7 (d): `Env: {GIT_DIR: "/machine/projects/b/.git"}` on a `lanes/…` call → 400 naming `GIT_DIR`, git never ran | a `Dir` confinement that `Env` undoes |
| `TestGitEnvIsAnExplicitAllowlist` | table, each → 400 naming the key and git never ran: `LD_PRELOAD`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_EXEC_PATH`, `GIT_SSH_COMMAND`, `GIT_EXTERNAL_DIFF`, `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_GLOBAL`, `GIT_ASKPASS`; `KEY_0=core.sshCommand`; the bare `KEY_0=http.extraHeader`; `COUNT=2` with one pair; `KEY_0` for `https://evil.example/` when `RemoteURL` is `https://github.com/…`; `GIT_TERMINAL_PROMPT=1`; `GIT_AUTHOR_NAME` holding a newline. Admitted (git ran): `GIT_OPTIONAL_LOCKS=0`; one host-scoped header pair; all six identity keys at once, with phase 3 task 08's forecast values | a `^GIT_` prefix match; a header sent to a host the call does not name; an allowlist that refuses phase 3's commit identity, so every forecast and land on a cloud machine fails |
| `TestGitCommitTreeTakesTheCallersIdentity` | a `projects/a` repository, `Scope: "project"`, `Class: "local"`: `commit-tree <its HEAD tree> -m "panacea forecast"` with the forecast's `Env` (`GIT_AUTHOR_NAME`/`GIT_COMMITTER_NAME` `panacea-forecast`, `EMAIL` `forecast@panacea.invalid`, `DATE` `@0 +0000`) → 200, and the id equals the one the same `commit-tree` prints when run directly with that environment on the test's own git; run twice, the same id | the identity keys dropped on the way to git (the id changes with the machine's clock and identity); phase 3's forecast not deterministic on a cloud machine |
| `TestGitExtraHeaderReachesGit` | `ls-remote` with `Scope: "project"`, `Class: "remote"`, against phase 3's `gitxtest.HTTPSRemote` stand-in that records `Authorization`, with `KEY_0=http.<RemoteURL's origin>/.extraHeader`, `VALUE_0=Authorization: Bearer t0k` → the stand-in saw `Bearer t0k` | phase 3's push credential path broken on cloud machines |
| `TestGitSecretsNeverLeaveInLogsOrErrors` | the same value, on a failing call → neither the log nor the problem body contains `t0k` | a host token in a log |
| `TestGitHooksNeverRun` | a clone whose `post-checkout` writes a marker file; `checkout -b x` through the route → no marker | agent-written hooks executed on the machine pod |
| `TestGitLaneFilterNeverRuns` | review I7 (d): a lane clone under `lanes/p-1/w-2` whose `.git/config` sets `filter.x.clean=<marker script>`, `.gitattributes` `* filter=x`, one modified file; `status --porcelain`, `Scope: "clone"` → 409 `conflict`, `reason: clone_config_untrusted`, `key: filter.x.clean`, and no marker | a lane's filter driver run on the machine pod; a lane checked under the Project rule (the `reason` differs) |
| `TestGitPrimaryWorktreeFilterNeverRuns` | the same in `projects/a` with `Scope: "project"` → 409, `reason: project_config_untrusted`, no marker | the review's "`lanes/` only" rule: a session on the primary worktree writes `projects/a/.git` |
| `TestGitIncludedConfigIsRefused` | `projects/a` whose config sets only `include.path=../evil` (a file that sets the filter) → 409, `key: include.path`, no marker | an included file's filter read past the check |
| `TestGitScopeMustFollowDir` | `lanes/…` with `"project"`; `projects/a` with `"clone"`; `projects/a` with `"none"`; `"none"` with `Args: ["status"]`; `Dir: ""` with `"project"`; `Scope: ""` → each 400, git never ran | the laptop's scope taken on trust; `Dir: ""` read as the disk root under a checked scope |
| `TestGitCredentialEnvIsRefusedInALane` | `Scope: "clone"`, `Class: "remote"`, one host-scoped header pair → 400, git never ran | the user's token handed to a git whose config is the agent's |
| `TestGitNeverSeesTheMachinedToken` | `machined` started through the function `main` calls, with `PANACEA_APP_MACHINED_TOKEN` and `PANACEA_APP_LOG_LEVEL` set; one route call to a fake `git` that dumps its environment → no `PANACEA_APP_` key in it | the scrub dropped; a scrub of the token alone; a scrub in a helper `main` never calls |
| `TestGitStartRefusesAMachineCredentialHelper` | a `HOME` whose `.gitconfig` sets `credential.helper=store` → start exits 2 naming `credential.helper` | a machine helper answering for the user's host |
| `TestGitAddProjectCloneHasOneShape` | the exact shape → the fake `git` saw `clone -- <url> /machine/projects/n` with its cwd `<owned root>/clone`, never `/machine`; an existing `projects/n`, a name with `/`, an extra argument, `Class: "read"` → each 400, git never ran | a `none`-scope call used to skip H3; `Dir: ""` resolved to the disk root |
| `TestGitLaneCreateCloneIsAdmitted` | real git: `projects/a` a repository with branch `main`; phase 3's exact argv, `clone --no-hardlinks --config core.autocrlf=false --config core.eol=lf --config core.checkStat=minimal --config core.trustctime=false --branch main --origin origin -- /machine/projects/a /machine/lanes/p-1/w-2` (with the test's root for `/machine`), `Dir: "projects/a"`, `Scope: "project"`, `Class: "local"` → 200, and `lanes/p-1/w-2/.git` is a directory whose `core.eol` is `lf` | the one-shape rule keyed on `Args[0] == "clone"`, which refuses every lane create on a cloud machine |
| `TestGitCloneOutsideItsTwoShapesIsRefused` | table, each `Scope: "project"`, `Dir: "projects/a"` → 400 and git never ran: destination `/machine/projects/b`; destination `/tmp/machined-git/x`; destination `/machine/lanes/p-1` (one element); destination `/machine/lanes/p-1/../../projects/b`; an existing destination; source `/machine/projects/b` (not the call's `Dir`); `--config filter.x.smudge=<marker script>`; `--template=/machine/projects/a`; `--separate-git-dir=/tmp/x`; `Class: "remote"`; one host-scoped header pair in `Env` | a lane create that plants a repository where H3 reads it under the wrong rule or not at all; a clone-time filter run before H3 first reads the clone |
| `TestGitFailureIsGitFailedWithGitsOwnSentence` | `rev-parse nosuchref` → 422, `details.gitSays` begins `fatal:`, `exitCode` non-zero, no `args` in details | a generic 500; argv (which may hold a URL) in the body |
| `TestGitOKExitIsAnAnswer` | a `merge-tree --write-tree` conflict with `OKExit: [1]` → 200, `exitCode: 1` | a forecast conflict reported as a failure |
| `TestGitStdinReachesGit` | `hash-object --stdin` with stdin `x` → the known hash | stdin dropped (commit-tree -F - in phase 3) |
| `TestGitReadCallsTakeNoOptionalLocks` | a `Class: "read"` call's environment includes `GIT_OPTIONAL_LOCKS=0` (via a fake `git` on `PATH` that dumps its env) | a status racing an agent for `index.lock` |
| `TestGitOutputIsCapped` | a fake `git` writing 65 MiB → 422 `output exceeds 64 MiB` | a pod killed by a runaway log |

#### Steps

- [ ] **Step 1:** Confirm phase 3 task 02 has landed **with §HGH**: `git grep -n "type Runner interface\|func MachineHardening\|type UntrustedConfig" -- control-plane/internal/gitx/` returns three lines. If not, stop.
- [ ] **Step 2:** Wire types; tests, each shown red first; the route over `gitx.NewExec(h)`; the scrub in `main.go`; all pass.
- [ ] **Step 3: Mutation check** (rule 7), each on a `$TEMP` copy, each turning the named test red:
  (1) the `Env` allowlist replaced by `^GIT_[A-Z0-9_]+$` → `TestGitEnvIsAnExplicitAllowlist`,
  on its `GIT_EXEC_PATH`, `GIT_SSH_COMMAND` and `GIT_CONFIG_PARAMETERS` rows. Not
  `TestGitRefusesGITDIRInEnv`: `GIT_DIR` is refused twice, by `machined` and then by phase 3's
  H5 (`ErrCallShape`, also a `400`), so no single mutant turns that test red. That is the
  intended depth, and the reason this mutant is checked against the other test;
  (2) the bare `http.extraHeader` admitted → `TestGitEnvIsAnExplicitAllowlist`;
  (3) the header host not compared with `RemoteURL`'s → `TestGitEnvIsAnExplicitAllowlist`;
  (4) credential `Env` admitted on a `clone` call → `TestGitCredentialEnvIsRefusedInALane`;
  (5) the wire `Scope` passed through unchecked → `TestGitScopeMustFollowDir`;
  (6) `"clone"` mapped to `gitx.Project` → `TestGitLaneFilterNeverRuns` (on its `reason`);
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
  on its `filter.x.smudge` row;
  (14) `Dir: ""` resolved to the machine root → `TestGitAddProjectCloneHasOneShape` (the cwd).
  The two named beforehand as likeliest to survive are (6), because `filter.*` is refused under
  both scopes, and (8), because a test of the helper alone passes. Each is killed only by the
  assertion named beside it: the `reason` string, and a start through `main`'s own function.
- [ ] **Step 4:** Full run; commit.

#### Acceptance

```bash
go test ./control-plane/cmd/machined/ -run '^TestGit' -race -count=1 -v > "$TMPDIR/t21.log" 2>&1; echo "exit=$?"   # exit=0; a build failure is non-zero, never a count of 0
grep -c "^--- PASS" "$TMPDIR/t21.log"                                                          # 23 (the 23 TestGit rows above; or SKIP lines without git)
grep -c "^--- SKIP" "$TMPDIR/t21.log"                                                          # 0 on a host with git
go test ./sandboxd/... ./control-plane/... -race -count=1                                       # ok
```

#### What this task does not claim

- That phase 3's `gitx` behaves identically on the machine pod's Linux git and the laptop's
  Windows git (`MinVersion` 2.40 is checked by `gitx.CheckVersion` at `machined` start; the sandbox
  image's Debian git version, trixie's after task 27a, is **UNVERIFIED** against it — task 27's
  `git version ≥ 2.40.0` row checks it at build).
- A clone surface in the cockpit.
- That code the agent chose can never run in the machine pod. The claim is narrower: every path
  that phase 3's §HGH names, and every `Env` path, is closed and tested. A git feature that §HGH
  does not know of is a new finding for phase 3's list, and it would then run with the whole
  disk in reach. The scrub hides `machined`'s token from children but not from `/proc/1/environ`
  (item 4).
- That `objects/info/alternates` is checked. It can lend a lane another repository's objects,
  but not its refs, and H5 pins the layout that holds the refs. **UNVERIFIED** beyond that
  reading of git's documented layout.
