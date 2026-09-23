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

#### I7. Two dependency edges are missing, and parallel lane (c) cannot compile or pass as ordered

- **Where:** plan:434-435 and plan:448-450 (11 depends on 07; 12 depends on 02 and 04; both run
  in lane (c) alongside lane (b)'s 08-10); task-11:108 and task-11:114 (`briefref.Archiver`);
  task-09:16 (task 09 creates `internal/briefref`); task-12:128-130 and task-12:152 (one mutex per
  project shared with lands; `TestPushAndLandSerialise`); task-09:96 (the Lander serialises per
  (project, branch), a different key).
- **What is wrong:**
  - Task 11 imports a package that task 09 creates. Run first, it either fails to compile or
    creates `briefref` itself and collides with task 09.
  - Task 12's race test needs task 09's `Lander` and a lock both sides take. The plan names two
    lock scopes (per project in task 12, per branch in task 09) and makes no task the owner of
    either.
- **How to fix:**
  - Move `briefref.go` into task 03 or 04. It is 30 lines with no dependencies.
  - Add `gitx.Locks` (per project path, one per process) to task 02, and have `Lander`, `Push`
    and `Fetch` all take it.
  - Or make 12 depend on 09 and 11 depend on 09, and redraw the lanes.

#### I8. Hard-linked objects make the project's object files writable from the sandbox

- **Where:** plan:49 and task-03:143 (`clone --local`, hard links); OD-2's consequence (owner
  decisions:13): "agent blast radius is its own clone".
- **What is wrong:** a hard link is one file with two names. A pack or loose object in the clone
  **is** the project's file. Through the bind mount the agent can clear the read-only attribute
  (task-03:152-155 says `os.Chmod` does exactly that) and write it, and that writes the user's
  repository history. `git gc` in the clone is safe, because it writes new files and unlinks.
  A deliberate or confused in-place write is not safe. Nothing in the plan states this residual,
  and OD-2's blast-radius sentence is false while it stands.
- **How to fix:** this is an owner decision, because OD-2 names hard links. Record it as Q6.
  Recommendation: have task 01 measure `clone --local` against `clone --no-hardlinks` on
  `~/Projects/Panacea`, and use `--no-hardlinks` unless the copy is too slow on the owner's
  laptop. If hard links stay, run `git fsck --connectivity-only` on the lane objects before every
  land, and state the residual in the plan's "does not claim".

#### I9. Evidence stays settled after its land is reverted

- **Where:** task-15:104-111 (a record decides every descendant of its commit).
- **What is wrong:** a revert commit removes a land's changes but keeps its commits as
  ancestors. A criterion settled at a lane commit therefore stays `settled` at the branch head
  after task 10 reverts that land. That is the same false "done" that contract §2.3's three
  surfaces exist to prevent, and phase 7 would carry it into a release manifest (S3-P7.5).
- **How to fix:** give the evaluator the first-parent `Panacea-Reverts` entries between each
  record's commit and the subject (task 10's trailers). A record whose commit lies in a reverted
  land's `^1..^2` range, with the revert an ancestor of the subject, no longer decides; the state
  becomes `open`, with `because: ["the land that carried this evidence was reverted"]`. Add test
  `TestStatesARevertedLandUnsettles`.

#### I10. Hub ask H-3's repository key has two formats

- **Where:** plan:304 (`github.com/<owner>/<repo>` or `<org>.visualstudio.com/<project>/<repo>`)
  versus task-13:136-139 (`github.com/<owner>/<repo>` or `azure-devops/<org>/<project>/<repo>`).
- **Why it matters:** this is the value the Hub keys shared evidence by. It is a cross-team
  contract value, and the plan and its part disagree about it. The part's form is the better
  one, because `dev.azure.com/<org>` and `<org>.visualstudio.com` are one organisation.
- **How to fix:** make plan:304 quote task 13's form exactly.

#### I11. Husky chaining is proven only on a clone configured by hand; a real clone carries no repository hooks

- **Where:** task-05:84-99 (the dispatcher reads the clone's local `core.hooksPath`, else
  `<common-dir>/hooks`); task-05:137 (`TestDispatchChainsToHusky` writes `core.hooksPath` into the
  clone's config itself); task-03:137-147 (clone creation copies identity only).
- **What is wrong:** `git clone` copies neither the source's `.git/config` nor its `.git/hooks`.
  Husky's `.husky/_` is generated and ignored by git. So a fresh lane of a husky or pre-commit
  repository runs **no** repository hooks until the agent runs the project's install step. The
  test passes either way, so R7 stays open in practice while the plan reports it closed. Owner
  checklist item 5 (task-23:77-78) would be the first place this shows up.
- **How to fix:** state fresh-clone semantics in task 05's "does not claim". At clone time, copy
  a **relative** `core.hooksPath` from the project's local config into the clone. Never copy
  `.git/hooks` contents: a host hook's interpreter line names a Windows path. Add test
  `TestCreateCopiesARelativeHooksPath`, and have the Docker husky test build its clone with
  `lanes.Creator` from a project whose config carries the setting.

#### I12. The fix branch has moved since the plan read it

- **Where:** plan:11-12, plan:179-181, task-06:3-4 (the fix at `2d6d95b` "plus its uncommitted
  `LinkedWorktreeNote`").
- **What is true on 2026-09-23:** `fix/sandbox-git-ca` is at `4f6d698`, five commits past
  `3303fd6`. The notice is committed (`58d607f`). The contracts were swept (`5e32000`;
  `git diff --stat 3303fd6...fix/sandbox-git-ca` shows 55 lines changed in the Go contract and 7
  in the UI contract) in the same sections that tasks 04-06 amend (§3.7, §5.1).
  `run()` in the identity test was made hermetic (`73dd041`). The shapes the plan relies on still
  hold: `GIT_CONFIG_COUNT` "2", `sessionMounts(worktreePath, conversationStore, caBundle)`,
  `Deps.GitIdentity func(ctx, worktreePath) (GitIdentity, error)`, and `!worktree.isPrimary`.
- **How to fix:** re-baseline G1 to `4f6d698`. Have tasks 04-06's preflight diff the contract
  sections the fix already edited, before appending to them.

### Minor

Each is small, and each is cheap to fix in the part file named.

1. **task-04:147** says an unknown field is `400 invalid_json`. `httpx.decodeCode` answers
   `bad_request` for an unknown field (`internal/httpx/problem.go:164-172`;
   `problem_test.go:203`). Fix the row.
2. **task-03:22** says `registry` has "two call sites" of `writeAtomic`. There is one
   (`internal/registry/filestore.go:401`; the definition is at `:460`, as cited).
3. **plan:81-84** says phase 3 persists "four things", then lists three (lane index, rename map,
   evidence). Queue order lives in the lane index. Make the count match.
4. **task-05:164** says `PASS, 8 tests`, but the table lists seven Docker tests (`Dispatch*`).
5. **plan:107-108** leaves `pullrequests.go` and `transcript.go` out of the `internal/api` file
   list, while plan:380-381 has them.
6. **task-11:74** uses `check-ref-format --branch`, which **expands** `@{-N}` to the previous
   branch's name. A request named `@{-1}` passes validation and then fails `update-ref` as a 500.
   Use `check-ref-format "refs/heads/<name>"`.
7. **task-03:54** derives a lane id from its path, so removing a lane and re-creating it with the
   same name reuses the id. History's `worktreeName` (task-10:33) and undo's
   `SetWorktreeReady(<Panacea-Land id>, true)` (task-10:69) then act on the new lane for the old
   lane's land. Add a creation nonce to the directory name, or keep a generation in the index.
8. **Lane sync and origin refresh fetch into repositories the control plane does not own**, and
   every `git fetch` runs `maintenance --auto`/`gc --auto` afterwards. A detached `gc` holding
   `packed-refs.lock` makes a concurrent land's `update-ref` fail as `git_failed`. Pass
   `--no-auto-maintenance` (git ≥ 2.29) on both fetches.
9. **task-16:130** reads an untracked file "from disk" on the host. Use `os.Lstat` and refuse a
   symlink or reparse point the agent made. Cap the read before reading, not after: a 2 GB
   untracked file is otherwise read whole to find its first 2,000 lines. Add `--no-textconv` to
   task 16's `diff` and `blame` calls (C1).
10. **desktop `components/develop/sessionVerbs.ts:608,624`** still says "the merge queue in
    Phase 3 owns that". Task 22's retirement grep (`arrives in Phase 3`, task-22:148) misses it.
    Task 19's Not ready button now owns un-marking, so reword it and widen the grep to
    `Phase 3`.
11. **task-17:110-114** matches tool calls "at or before commitAt", which is the **author** date.
    `--amend` and rebase keep the author date, so a later edit is excluded and tier 2 declines.
    Use the committer date (`%cI`).
12. **task-11:105** treats `config --remove-section branch.<name>` on an absent section as fine,
    but git exits non-zero for it. Have task 01 pin the exit code, and have the code match it.
13. **Git LFS repositories:** Git for Windows' system config defines `filter.lfs.*`, and the
    sandbox image (`images/sandbox/Dockerfile:12` installs `git`, not `git-lfs`) does not. LFS
    paths then look modified in the sandbox, which is the same shape as C2. Say so, or add
    `git-lfs` to the image.
14. **The clone's `origin` is a Windows path.** Linux git reads `C:/…` as scp-style SSH to a host
    named `C`, so an agent that tries `git pull` gets a confusing sentence. Once C3's fix stops
    the host from using it, set `remote.origin.pushurl` and the URL to a self-describing
    unreachable value, and record the sandbox's sentence in task 01.
15. **`core.symlinks=false` and `core.longpaths`.** Git for Windows writes `symlinks=false` into a
    clone's local config, so a repository's symlinks are plain files in the sandbox. A clone under
    `%LOCALAPPDATA%\PanaceaApp\state\worktrees\p-…\<name>\` (about 75 characters before the
    repository's own paths; `statepath.go:30-44`) fails with "Filename too long" unless
    `core.longpaths=true`. Task 01 measures path length but no task sets the value. Clone with
    `-c core.longpaths=true` and write it locally. State the symlink behaviour.
16. **task-11:89-93** renames a branch without `CheckedOutAt`. `git branch -m` silently moves the
    user's own checkout's HEAD. Refuse with `branch_checked_out`, as delete does.
17. **Loopback exposure grows.** Q5 names push. Destructive routes (DELETE worktree, DELETE
    branch, lands, undo) and the transcript span GET sit behind the same unauthenticated
    boundary. Q5's sentence should list them.
