# Phase 3 plan — adversarial review

**Plan:** `docs/superpowers/plans/2026-09-23-phase-3-git-convergence.md` and its 23 part files,
at `034232a` on `feat/phase-3-git`. **Reviewer:** adversarial plan reviewer (OD-1), 2026-09-23.
**Status of this file:** in progress; the Critical section is written first so a restart keeps it.

Citations are `file:line` in this worktree. "Plan" means the map file; `task-NN` means
`docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-NN.md`. Every code citation
below was opened in the tree at `034232a` (whose control plane is `3303fd6`'s) or on
`fix/sandbox-git-ca`, unless it is marked **unverified**.

## Issues

### Critical

#### C1. Host-side git runs agent-controlled configuration and hooks: a sandbox escape to the Windows host

- **Where:** plan:73-74 (status read "from the clone"), plan:153-156 (decision 3, origin refresh
  in every clone), task-02:101-104 (`Exec.Run`'s headless defaults, the only hardening any call
  gets), task-07 (observed claims run `git status` in the clone on the host), task-09 (origin
  refresh), task-11 (the remove question reads the clone). Nothing in the 23 parts mentions
  `core.fsmonitor`, `core.hooksPath` for host calls, `remote.*.uploadpack`, or filter drivers
  (grep of the parts directory, 2026-09-23: zero hits).
- **What is wrong:** a lane clone's `.git` directory is inside the bind mount
  (`sessionMounts`, `fix/sandbox-git-ca:control-plane/internal/api/sessions.go`, the worktree
  mount is read-write). So is the **project's** `.git` whenever a session runs on the primary
  worktree, which the sandbox fix makes a supported case. The agent can therefore write any key
  into `<clone>/.git/config` or `<project>/.git/config`, and any file into their `hooks/`.
  The control plane then runs the **host's** git in those repositories:
  - `git status` in the clone (task 07) honours `core.fsmonitor=<program>` and any
    `filter.<driver>.clean` a `.gitattributes` names — both execute a program on the host;
  - `git fetch` in the clone for the origin refresh (decision 3) and in the project for lane
    sync runs the `reference-transaction` hook from whatever `core.hooksPath` or `.git/hooks`
    the repository names, and a fetch honours `remote.<name>.uploadpack`, which for a local
    path is a command run locally;
  - `update-ref` in the project on every land, undo and revert (tasks 09, 10) runs
    `reference-transaction` too.
  Each is code execution on the Windows host, as the user, outside the container. That defeats
  the isolation the product is built on (CLAUDE.md, "Isolation: Kata … from day one"), and it
  is exactly the Kata model phase 8 inherits through `gitx.Machine`.
- **Why it matters:** an agent that has been prompt-injected by a file in the repository reaches
  the user's laptop with the user's credentials and the git credential helper task 12 relies on.
  A4 ("a sandbox never holds a git credential") holds only while the sandbox cannot run code on
  the host. This breaks it.
- **How to fix:** add a hardening rule to task 02 and pin it with tests:
  1. `gitx.Exec` prefixes every host call with command-scope config that neutralises execution
     paths a repository can name: `-c core.hooksPath=<an empty directory the control plane
     owns>`, `-c core.fsmonitor=false`, `-c core.sshCommand=`, `-c protocol.ext.allow=never`,
     and on every fetch an explicit `--upload-pack=git-upload-pack`. Command scope outranks
     repository config (task 01's own `TestBehaviourLocalConfigHooksPathIgnoresEnvironment`
     proves the ordering).
  2. Filter drivers cannot be neutralised by name. Before any host call **in a clone**, read
     `git config --local --list --show-origin` (reading config runs nothing) and refuse the call
     when a key outside a closed allowlist is present (`core.*` keys git itself writes at init,
     `remote.origin.*`, `branch.*.remote`/`.merge`, `user.name`/`user.email`, a relative
     `core.hooksPath`), surfacing a `conflict` with `reason: clone_config_untrusted` and the key.
     `include.*`/`includeIf.*` are outside the allowlist.
  3. State on the land path that control-plane writes run **no** repository hooks, including the
     user's own husky `pre-push` on a cockpit push, and say why (the project `.git` is
     sandbox-writable when a primary-worktree session exists).
  4. Tests, each red first against today's `Exec`: a clone whose config sets
     `core.fsmonitor` to a script that writes a marker file, then task 07's observer runs → no
     marker; a `reference-transaction` hook in the project writing a marker, then a land → no
     marker; `remote.origin.uploadpack` set to a marker-writing script, then the origin refresh →
     no marker; `filter.x.clean` plus a `.gitattributes` line → the observer refuses with
     `clone_config_untrusted`.
  Record the residual in the plan's Q-table: the scanner of phase 2 already runs git in the
  primary worktree's repository, and whether its read-only commands are exposed is phase 2's to
  check (not verified here).

#### C2. Line endings: a clone made by the host's git is CRLF on disk, and the sandbox's Linux git sees every text file as modified

- **Where:** task-03:143 (the clone command), task-01:25-28 (every behaviour test sets
  `GIT_CONFIG_NOSYSTEM=1`), task-01:57 (`TestSandboxCloneHasWorkingGit` asserts only an exit code).
- **What is wrong:** this machine's **system** gitconfig sets `core.autocrlf=true`
  (`git config --show-origin --get-all core.autocrlf` on 2026-09-23:
  `file:C:/Program Files/Git/etc/gitconfig true`; Git for Windows' installer default). A user
  project without a local override is therefore cloned by `lanes.Creator` with CRLF working
  files. `git clone` writes no `core.autocrlf` into the clone's local config, and the sandbox's
  Linux git (2.39 per the fix's `sessionEnv` comment) has no system config saying `true`. Inside
  the sandbox, the index's stat data (written by Windows git) never matches what Linux sees
  through the bind mount, so git re-hashes, and every CRLF file hashes differently from its LF
  blob: `git status` shows the whole tree modified, and an agent's `git add -A && git commit`
  rewrites every text file to CRLF. The host's own `git status` (task 07, autocrlf=true) meanwhile
  says clean, so the Merge lens and the agent disagree about the same directory.
  Task 01 cannot catch it: `GIT_CONFIG_NOSYSTEM=1` removes exactly the setting that causes it, and
  the sandbox test checks `git status` exits 0, which it does.
- **Why it matters:** it is the first thing a user sees in a sandbox on a real project, and the
  failure writes damage into history that lands into the product branch.
- **How to fix:** in task 03, clone with `-c core.autocrlf=false -c core.eol=lf` **and** write
  `core.autocrlf=false` into the clone's local config, so the working tree holds the blobs' bytes
  and both gits agree; also write `core.checkStat=minimal` and `core.trustctime=false`, so the
  two operating systems' stat data stop forcing a full re-hash on every host `status` (the
  poll-driven observer of task 07). A repository whose own `.gitattributes` asks for `eol=crlf`
  keeps that, because attributes outrank `core.autocrlf`. Tests: task 01 gains
  `TestBehaviourSystemAutocrlfReachesAClone` run **without** `GIT_CONFIG_NOSYSTEM` (or with a
  test system file setting `core.autocrlf=true`), asserting what the clone's working bytes are;
  `TestSandboxCloneHasWorkingGit` asserts `git status --porcelain` is **empty** on a fresh clone
  of a repository with LF blobs, under a host config with `core.autocrlf=true`; task 03's
  `TestCreateWritesLineEndingConfig` pins the three keys.
- **Also verified:** `git ls-files --eol images/sandbox/entrypoint.sh` in this worktree prints
  `i/lf w/crlf`: an LF blob checked out CRLF by the system `autocrlf`, the exact mechanism above.

#### C3. The user's token reaches whatever the agent writes into a repository's config (A4 broken)

- **Where:** task-09:128-130 (origin refresh: `fetch … origin …` **in the clone**), task-12:66
  (timer `fetch --prune origin` in the project), task-12:94-101 (push), task-13:106-110
  (`GitHelper.Dir` is "the project's, so a repo-local helper config applies"), task-13:164-168
  (`PushEnv`).
- **What is wrong:** every remote operation takes its URL, proxy, TLS and helper settings from a
  repository config the agent can write (C1's premise).
  1. **The origin refresh follows the clone's `origin`.** The agent sets
     `remote.origin.url=https://github.com/<any>/<repo>`, `http.proxy=http://<its listener>` and
     `http.sslVerify=false` in `<clone>/.git/config`. After the next land, the host's git fetches
     "origin" from GitHub through that proxy with TLS verification off, and the machine's Git
     Credential Manager answers non-interactively with the user's cached token (task 12 relies on
     exactly that). The `Authorization` header goes to the agent's proxy in clear.
  2. **A repo-local `credential.helper`** runs on the host (C1) and, after any successful
     authenticated fetch or push, git calls `store` on **every** configured helper with the
     username and password on stdin. The agent's helper receives the token GCM supplied.
     `GitHelper` deliberately honours repo-local helper config (task-13:108).
  3. `PushEnv` scopes the header to `http.https://<Authority>/.extraHeader`, which stops it going
     to another host, but not through an agent-chosen `http.proxy` with `http.sslVerify=false`.
- **Why it matters:** A4 is the phase's central security promise ("a sandbox never holds a git
  credential", plan:33-34, plan:218-220). Obtaining the token through the host is holding it.
- **How to fix:**
  1. The origin refresh never uses the clone's configured remote. Fetch from the project's path
     given **literally** (from the lane index, never read from the clone):
     `git -C <clone> fetch --no-tags --upload-pack=git-upload-pack <projectPath> +refs/heads/<b>:refs/remotes/origin/<b>`,
     with C1's hardening. A local path needs no credential, proxy or TLS.
  2. Every `gitx.Remote` call and `credential fill` gets command-scope overrides that repository
     config cannot beat: `-c credential.helper=` followed by `-c credential.helper=<the helper
     the machine's global/system config names>` (read once at startup from
     `git config --global/--system --get-all credential.helper`), `-c http.sslVerify=true`, and
     no repository `http.proxy`; say where the proxy comes from instead (global config or
     `HTTPS_PROXY`). C1's allowlist then refuses a project config carrying `credential.*`,
     `http.*`, `remote.*.pushurl`, `url.*.insteadOf` or `url.*.pushInsteadOf` that the cockpit
     did not write, with the key named.
  3. `GitHelper.Dir` is a directory the control plane owns (an empty temp repository), never the
     project, so no repo-local helper config is read.
  4. Tests, red first: a clone whose config names a proxy `httptest` server as `http.proxy`,
     `sslVerify=false`, and an `origin` on an `httptest` smart-HTTP URL → after a land, neither
     test server received a request; a repo-local `credential.helper` that writes its stdin to a
     file → after a push and a `GitHelper.Credential`, the file does not exist.
