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

### Important

#### I1. The OD-8 adapter declines every real transcript: a third of the lines carry no `version`

- **Where:** task-17:20-22 ("every line carries `type`, `version`, `timestamp`, `uuid`, …"),
  task-17:98-104 (decline when "any line's `version` is not PinnedVersion" or "more than 1% of
  lines fail to parse"), task-17:154 (the synthetic fixture is hand-written to that shape).
- **What is wrong:** verified 2026-09-23 with a read-only structural count over this machine's 40
  newest transcripts (76,592 lines; Claude Code 2.1.236, 2.1.260, 2.1.267, 2.1.280; no text
  printed): 25,111 lines have **no `version` field**, of eleven types — `last-prompt`, `mode`,
  `permission-mode`, `atis-latch`, `ai-title`, `queue-operation`, `file-history-snapshot`,
  `file-history-delta`, `cost-state`, `relocated`, `worktree-state`. Only `user`, `assistant`,
  `system` and `attachment` lines carry one. Read literally, both decline rules fire on every
  transcript, so tier 2 never appears. The synthetic fixture is written to the false shape, so
  every task 17 test passes. OD-8's degradation is honoured, but the feature is dead on arrival,
  and nothing in the plan would reveal it until the owner's session.
- **Also:** `TestCapturedFixtureParses` skips until a file exists, so the "canary that fails
  when the format moves" (OD-8, quoted at task-17:6-10) never fails. After a pin bump nothing
  forces a new capture.
- **How to fix:** check `version` only on `user` and `assistant` lines. Skip lines of any other
  `type` without counting them as failures. Correct the verified-shape paragraph, and add the
  eleven version-less types to the synthetic fixture. Name the captured fixture
  `captured-<PinnedVersion>.jsonl`. The test **skips** only while `PinnedVersion == "2.1.274"`
  and the OD-12 waiver constant is set; any other pin with no fixture **fails**. Add
  `TestVersionlessLinesAreIgnored` (a fixture that is 40% `file-history-snapshot` → tier 2 still
  links).

#### I2. `merge-tree` output is parsed without `-z`, so any path git quotes breaks the forecast

- **Where:** task-08:48, task-09:119, task-16:63 (`merge-tree --write-tree --name-only`, no
  `-z`); task-04:65 (`details.paths` "newline-joined").
- **What is wrong:** without `-z`, git C-quotes paths holding a space, a quote, a control
  character or (under the default `core.quotePath=true`) any non-ASCII byte:
  `"docs/r\303\251sum\303\251.md"`. Task 07 parses `diff` and `status` with `-z` (task-07:16), so
  `ConflictsWith` (Paths ∩ a row's `Changed`) never matches such a path, and the lens blames the
  branch itself. Newline-joined `details.paths` also cannot carry a path containing a newline,
  which task 07's own `TestPathsWithNewlinesAndSpaces` treats as a real case.
- **How to fix:** add `-z` to every `merge-tree` call and parse NUL-separated output (task 01
  pins the `-z` form too). Carry paths in `details` as one JSON-encoded array string, or give
  `gitConflict` a `[]string` field. Add a test with a conflicted `é`-named file.

#### I3. "Exact" undo is not exact once another lane has merged the land

- **Where:** task-10:41-45, 64-72 (exact means tip, a land, not pushed); plan:153-156 (decision
  3 puts every land into every clone's `origin/<branch>`); task-19:49-53 (3-D3's prompt tells an
  agent to merge `origin/<branch>`).
- **What is wrong:** the plan's own conflict flow makes lane B merge land A's commit M. Undo of M
  then moves the branch back, but `refs/panacea/lanes/B` still contains M. B's next land brings M
  and A's work back without a word, and the forecast calls it clean. Task 23's steps 8-10 do
  exactly this sequence and assert only the tip.
- **How to fix:** before an exact undo, run
  `for-each-ref --contains <sha> --format=%(refname) refs/panacea/lanes/`. When any lane contains
  M, answer `undo_not_exact` with `why: merged_into_lane` and `details.worktrees`, and offer the
  revert. History's `UndoWhy` gains the same value. Add test `TestUndoRefusesALandALaneHasMerged`.

#### I4. The resolve prompt merges an `origin/<branch>` that is stale for every conflict not caused by a land

- **Where:** plan:153-156 (clones refreshed "after every land" only); task-19:49-53
  (`resolvePrompt`, including the case "conflicts with the branch itself").
- **What is wrong:** the branch also moves by undo (task 10 refreshes), by revert (task 10 does
  **not** say it refreshes), and by a user's own commit or `git pull` in a terminal (nothing
  refreshes). In each case the agent merges an old ref, the conflict survives, and the lane
  stays red after the agent reports success.
- **How to fix:** the syncer already reads the branch tip each sweep. When a branch tip differs
  from what the clones were last told, it refreshes their `origin/<branch>` (C3's
  literal-path fetch). Revert refreshes like land and undo. `ResolveDialog` refreshes the one
  lane before it starts the session. Add test `TestSweepRefreshesClonesAfterADirectCommit`.

#### I5. There is no way to bring the remote's commits in, yet the copy promises one

- **Where:** task-20:78-80 (`push_rejected`: "Fetch, then land or merge them in a worktree");
  task-08:131-151 (`Behind` is shown); task-09 (a land takes only `refs/panacea/lanes/*`).
- **What is wrong:** a clone's `refs/remotes/origin/*` mirrors the project's `refs/heads/*`, not
  the project's `refs/remotes/origin/*`, and the clone's objects stop at clone time. So no lane can
  see or merge a teammate's pushed commits. Landing takes lanes only, so the product branch
  cannot take them either. The only way out is a terminal `git merge` in a checkout of the
  branch, and that checkout switches landing off (A5, Q3). A diverged branch strands the user.
- **How to fix:** have the origin refresh also copy the project's `refs/remotes/origin/<branch>`
  into every clone as `refs/remotes/upstream/<branch>`, so a lane can merge it. Add a queue row
  "Bring in origin/<branch>" when `behind > 0`: the same merge-tree, commit-tree and update-ref
  land, refused on conflict (A2), with resolution in a lane through the refreshed ref. Correct the
  `push_rejected` sentence. Or, if this is judged out of scope, record it as open question Q6 and
  make the copy say "merge it in a terminal checkout, then switch that checkout away".

#### I6. A lane sees only the clone's `refs/heads/<branch>`, so work committed on another branch in the clone is invisible

- **Where:** task-07:69-77 (`SyncLane` reads `refs/heads/<branch>` in the clone only).
- **What is wrong:** agents routinely `git switch -c fix-x` before committing. Those commits never
  reach the lane, the row says "nothing to land", and removing the worktree calls it clean.
  `Inspect` counts unlanded commits from the lane ref (task-11:38), so the confirm question omits
  them and `RemoveTree` deletes them.
- **How to fix:** in `Observe`, read `symbolic-ref -q HEAD` in the clone. When HEAD is not
  `refs/heads/<branch>` (another branch, or detached), `LaneObservation` gains
  `headElsewhere: "<ref or sha>"`, and the card says so. `Inspect` also archives every clone ref
  under `refs/heads/` that is not reachable from the lane (`for-each-ref` in the clone, then a
  fetch into `refs/panacea/archive/lanes/<id>/<unix>/<name>`). Add tests
  `TestObserveReportsAHeadOffTheBranch` and `TestRemoveArchivesOtherClonesBranches`.
