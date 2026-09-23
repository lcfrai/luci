# Phase 3 plan — adversarial review

**Plan:** `docs/superpowers/plans/2026-09-23-phase-3-git-convergence.md` and its 23 part files,
at `034232a` on `feat/phase-3-git`. **Reviewer:** adversarial plan reviewer (OD-1), 2026-09-23.
**Verdict:** Needs fixes. There are 3 Critical, 12 Important and 17 Minor issues. All three
Critical issues come from one blind spot: the plan treats the repositories the sandbox can write
as trusted input to the host's git.

Citations are `file:line` in this worktree. "Plan" means the map file; `task-NN` means
`docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-NN.md`. Every code citation
below was opened in the tree at `034232a` (whose control plane is `3303fd6`'s) or on
`fix/sandbox-git-ca`, unless it is marked **unverified**. Nothing was run except read-only git,
`sed`/`grep` over the tree, and one structural count over this machine's Claude Code transcripts
(line types and version fields only, no content printed).

## Coverage

| Source | Item | Covered by | Verdict |
|---|---|---|---|
| OD-1 | adversarial review before code; UI branches unmerged until the owner looks | plan:8; task 23 step 4 | covered |
| OD-2 | a local clone per worktree, fetched back under `refs/panacea/lanes/…` | tasks 01, 03, 04, 07 | covered. The blast-radius claim is false while objects are hard-linked (**I8**) |
| OD-3/5 | shared stores in the Hub | H-2 seam, local store labelled (task 15, Q2) | covered as a seam |
| OD-4 | Azure DevOps **and** GitHub behind one interface | tasks 13, 14 | covered; host APIs unverified (declared) |
| OD-6 | the user's own tokens, never pooled | task 13 seam; `credstore.GitHelper` (Q1) | shape covered; token safety broken (**C3**) |
| OD-8 | read-only, pinned, canary-tested, degrading adapter | task 17 | degrades, but declines on every real transcript (**I1**); the canary never fails |
| OD-10 | removal deletes the worktree's conversation store | task 11 | covered |
| OD-12 | owner checks batched, unverified until then | task 23 checklist; every UI task's look marked unverified | covered |
| Brief A1-A9 | git is the truth; clean merges only; no force; no credential in sandbox; plumbing CAS; machine seam; archive; rename map; no enforced claims | plan:81-91, tasks 02, 09-12 | covered, except A4 (**C1**, **C3**) and "exact before a later land" (**I3**) |
| Brief §2.1 | create/remove worktree, branch lifecycle, fetch on a slow timer | tasks 04, 11, 12 | covered |
| Brief §2.2 | identity, `core.hooksPath`, session id, chained hook | tasks 03, 05 | covered in code; husky in practice (**I11**); line endings missing (**C2**) |
| Brief §2.3 | observed claims, chained forecast, shared-worktree warning | tasks 06, 07, 08 | covered; path quoting (**I2**); off-branch commits invisible (**I6**) |
| Brief §2.4 | queue, land one/all, conflict in lane, history and two undos | tasks 08-10, 19, 20 | covered; stale `origin` in the resolve prompt (**I4**) |
| Brief §2.5 | push, ahead/behind, fetch time, PR with key as text | tasks 08, 12-14, 20 | covered, except bringing in remote commits (**I5**) |
| Brief §2.6 | DoD mount, merge-link trailer, evaluator, deletion archive, back-links | tasks 09, 11, 15-17, 19 | covered as seams to phase 4; revert keeps evidence settled (**I9**) |
| Brief §2.7 | the two lenses fill task 26's files; `AppShell.tsx` untouched | tasks 19-21; G2 | covered (G2 verified still absent: `desktop/src/components/lens/` holds only `LensPlaceholder.tsx`) |
| Prototype | 14 states, `phase-3-convergence.html:387-402` | 10 Merge states in tasks 19-20, 4 Files states in task 21; owner recipe in task 23 | all 14 mapped |
| Design system | tokens, elevation, radius, motion, heartbeat, tabular-nums, virtualisation | tasks 06, 19-22 design checks and audit tests | compliant. The 600 ms background flash is allowed by `DESIGN-SYSTEM.md:63`; virtualisation over 50 rows is in task 21 |

## Strengths

- **Every git assumption becomes a test before anything relies on it** (task 01), and each later
  task's preflight reads the results document. That is the right answer to this program's history
  of wrong inputs.
- **Landing is correct git.** The land is `merge-tree --write-tree`, then a two-parent
  `commit-tree` with the old tip as parent 1, then a compare-and-swap `update-ref`. It refuses a
  checked-out branch, and `TestLandLosesARaceCleanly` injects the race. The revert is the right
  three-way merge (`--merge-base=<M> <tip> <M^1>`). Fetch-backs use `--no-write-fetch-head`, so
  `FETCH_HEAD` keeps meaning "the last remote fetch".
- **Seams are structural, not promised.** Two lines go into `api.go`, and every later task appends
  a contiguous block to `gitdeps.go` and `gitroutes.go`. The Hub and phase 4 asks are declared as
  Go interfaces with fallbacks, and the gates say what happens when each is absent.
- **Tests name the mutant each one kills,** including the negative cases that usually go missing:
  a GET that writes, an unscripted git call, a token in `%#v`, an argv with `+`, an injected
  trailer, a spinner or interval added.
- **Honest scope.** Every part has a "does not claim" section. Unverified host APIs, the
  unverified 2.1.274 transcript shape and the UI's look are all marked as such.
- **The citations are accurate.** Against the real tree I checked every cited identifier:
  `project.WorktreeID`, `BranchID`, `worktree.Verify`/`ErrNotADir`, `api.Enumerator`,
  `LocalMachineID`, `routes`/`route`/`guarded`, `catalogReadOnlyRoutes` at
  `routes_audit_test.go:32`, `TestEveryGitCommandIsReadOnly` at `project_test.go:429`,
  `writeAtomic` at `filestore.go:460`, `EnvEntries`, `containerEnv`,
  `TestThePodEnvironmentIsExactlySpecEnvEntries`, `sessionConversationOwner`,
  `sessionStoreSegment`, the conversation-path comment at `sessions.go:39-42`,
  `poller.ts:84`, `heartbeat.ts:34`, `onGoodTick`, `PERSIST_VERSION = 1`, `moveWithinVisible` at
  `lib/immutable.ts:45`, `TopBar.tsx:29`, the `AppShell.tsx` placeholders, `EmptyDevelop.tsx:100`,
  `ContextMenu.tsx:103`, and the Dockerfile pin `claude-code@2.1.274`. All exist as described.
  The fix's shapes (`GIT_CONFIG_COUNT` "2", the three-argument `sessionMounts`, `GitIdentity`)
  match too. Only the Minor items above and I12 are wrong.

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
   `bad_request` for an unknown field (`internal/httpx/problem.go:165-173`;
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

## Test strategy, sizing and order

- **Red before green** is explicit in tasks 02 (helper process first), 06 ("run them red") and
  09, and implied elsewhere by "tests first" steps. Every row names its mutant, and the negative
  cases are present.
- **The one systematic gap: the tests are hermetic in exactly the way production is not.**
  - `gitxtest` and task 01 set `GIT_CONFIG_NOSYSTEM=1` and an empty global config, so the
    system `core.autocrlf=true` (C2), GCM as the system `credential.helper` (C3) and
    `filter.lfs` (Minor 13) never reach a test.
  - No test builds a **hostile** repository: agent-written config, hooks or remotes (C1, C3).
- **Fix:** add a `gitxtest.HostileClone` helper that writes each dangerous key and hook with a
  marker file, plus one "production-shaped" run per task that touches a clone (a system config
  file with `core.autocrlf=true` and a `credential.helper`).
- **Sizing:** 1-3 hours per task is plausible for most tasks. Task 09 (land, 20 tests, the
  briefref package, main.go wiring) and task 16 (four parsers and pairwise merge-tree) are the
  largest and likely run over. C1/C3's hardening adds about 2 hours to task 02 and 1 hour each to
  tasks 07, 09 and 12. The C2 fix is about 30 minutes in task 03, plus a test run without
  `GIT_CONFIG_NOSYSTEM`.
- **Order:** sound apart from I7 (11 needs `briefref`; 12 needs a shared lock and the Lander).
  G2 still holds the desktop track (verified absent 2026-09-23).

## The two defects a first implementer is most likely to hit

1. **Every file shows as modified inside the first real sandbox (C2).**
   - **Why it is likely:** the implementer follows task 01 and `gitxtest` exactly, so every test
     runs with `GIT_CONFIG_NOSYSTEM=1`. The suite is green. Task 04's `curl` returns
     `"kind":"clone"`, and task 01's Docker test sees `git status` exit 0.
   - **When it shows up:** the first owner-driven session, or the first agent commit that
     normalises every file to CRLF.
   - **Does the plan prevent it?** No. No part mentions `core.autocrlf`, and the one place that
     notices CRLF (task-05:175-179) scopes a `.gitattributes` rule to `githooks/` only.
2. **Tier 2 back-links never appear (I1).**
   - **Why it is likely:** the implementer writes the synthetic fixture to the shape task 17
     states ("every line carries `version`"). They implement "decline when any line's version is
     not the pin", and every test passes.
   - **When it shows up:** the first real transcript. About a third of its lines have no
     `version` (verified on 76,592 lines here), so the adapter declines it with a format reason.
     The owner sees tier 1 everywhere and cannot tell a broken adapter from a correct
     degradation.
   - **Does the plan prevent it?** No. It states the false shape as verified, and the one test
     that would catch it (`TestCapturedFixtureParses`) is designed to skip.

Runner-up: `merge-tree` without `-z` (I2). It passes every ASCII-named test, then blames the
branch itself for the first conflict in a path with a space or an accent.

## Assessment

The plan is unusually disciplined. The git plumbing is right, the seams are structural, the tests
name their mutants, and almost every citation checked out against the real tree. Its failure is
concentrated in one assumption: **the host's git can trust the repositories the sandbox writes.**

- C1 (host code execution through config and hooks) and C3 (the user's token leaving through
  agent-written remote, proxy and helper settings) each break A4, the phase's central promise.
- C2 (line endings) breaks the first real session on this machine.
- None needs a redesign. Each needs a hardening rule in `gitx`, a config allowlist for clones,
  one literal-path fetch, three config keys at clone time, and hostile-repository tests.

The Important items need a part-file edit each, and two need the owner:

- **I5:** bringing remote commits in. Build it, or record it as Q6 and fix the copy.
- **I8:** hard links versus `--no-hardlinks`, which is OD-2's own wording.

Record both as open questions with the recommendations above, then build on the
recommendations.

**Plan quality: Needs fixes**

## Fix batch b01 (2026-09-24): C1 and C3

Both findings are adopted as written (orchestrator ruling R1). The rule is written once, as
`task-02 §HGH`, and every other part cites it at its call site (R2). Line numbers are for the
tree at this batch's last commit. "parts/" means `2026-09-23-phase-3-git-convergence-parts/`.

### C1: host git runs agent-controlled config, hooks and filters

| Review's fix | Where it is now |
|---|---|
| 1. Command-scope prefix: `core.hooksPath=<owned empty dir>`, `core.fsmonitor=false`, `core.sshCommand=`, `protocol.ext.allow=never`, and `--upload-pack=git-upload-pack` on fetch | parts/task-02.md:328 (H1). It adds `core.askPass=`, `commit.gpgSign`/`tag.gpgSign=false`, `submodule.recurse`/`fetch.recurseSubmodules=false`, the `credential.helper=` reset, `--receive-pack` on push, and `ErrCallShape`, so no caller's `-c` can precede the prefix. The interfaces are at task-02.md:73-113 (`Call.Scope`, `Call.RemoteURL`, `Hardening`, `MachineHardening`, `UntrustedConfig`, `ErrCallShape`). `NewExec(Hardening)` is wired at parts/task-04.md:176 |
| 2. Clone-config allowlist, refusing with `clone_config_untrusted` and the key; `include.*`/`includeIf.*` outside it | parts/task-02.md:369 (H3). `Clone` scope is a closed allowlist; `Project` scope is a refusal list, because the user's own config is not ours to close. The listing is read with `--local --no-includes -z`. The reasons are at parts/task-04.md:83 and parts/task-18.md:162 |
| 3. Land-path writes run no hooks, the user's husky `pre-push` included, and why | parts/task-02.md:385 (H4). It is cited at parts/task-09.md:133, parts/task-10.md:71 and :89, and parts/task-12.md:103 |
| 4. Red-first marker tests: fsmonitor → observer; `reference-transaction` → land; `remote.origin.uploadpack` → origin refresh; `filter.x.clean` → `clone_config_untrusted` | parts/task-02.md:406 (15 §HGH tests, each layer tested alone), Step 2a at :442, mutants at Step 3a :447, and the count at :457. End to end: parts/task-07.md:164 (fsmonitor and filter, observer), parts/task-09.md:197 (hook, land), parts/task-09.md:198 (uploadpack, refresh), and parts/task-11.md:162 (a hostile clone stays removable) |
| Call sites (review "Where": task 07 status, task 09 refresh, task 11 remove question) | parts/task-07.md:77 and :129-130, parts/task-09.md:128-133, parts/task-10.md:71 and :89, parts/task-11.md:40-45, :54 and :70 |
| Residual: phase 2's scanner runs git in the primary worktree's repository | plan:355 (Q6), and parts/task-02.md:469. **Not verified**: it is phase 2's to check |

**Beyond the review (declared, root-cause option taken):** H3 alone has a check-then-call race.
An agent could rewrite `.git/config` between the check and the call. H5
(parts/task-02.md:392) closes it by binding `<path>/.git/config` read-only into the sandbox, which
is built at parts/task-05.md:69 and tested at :149 and :181. It amends the mounts row at plan:195
and the file map. Its cost, that the agent's own `git config` writes fail, is plan:356 (Q7).
**Unverified:** that Docker Desktop honours the nested read-only file bind. Task 05's docker test
is the check.

### C3: the user's token reaches agent-written repository config

| Review's fix | Where it is now |
|---|---|
| 1. The origin refresh fetches the project's literal path, never the clone's `origin` | parts/task-09.md:129-131 (Land step 14), plan:158 (decision 3), and parts/task-02.md:350 (H2, which also names task 07's lane sync). Tested at parts/task-09.md:198: (a) an allowlisted `remote.origin.url` pointed at an `httptest` server is never contacted; (b) a hostile `http.proxy`/`sslVerify`/`uploadpack` is refused `clone_config_untrusted`, neither server is contacted, and no marker is written |
| 2. Command-scope `credential.helper=` reset plus the machine's helpers; `http.sslVerify=true`; proxy from global config or `HTTPS_PROXY` | parts/task-02.md:328 (H1: the reset) and :350 (H2: `Hardening.Credential` and `.Proxy`, read once by `MachineHardening`, task-02.md:99; task-04.md:176). **A correction to the review's fix:** git selects `http.<url>.*` by URL specificity before file order, so a generic `-c http.sslVerify=true` loses to a repository's `http.https://github.com.sslVerify=false`. H2 therefore writes each override twice, generic and at the exact `RemoteURL`, and every `Remote` call must carry `Call.RemoteURL` (parts/task-12.md:68 and :104). The pin is `TestHGHRemoteOverridesBeatAURLScopedKey` (`git config --get-urlmatch`), in the table at task-02.md:406 |
| 2b. The allowlist refuses `credential.*`, `http.*`, `remote.*.pushurl`, `url.*.insteadOf` and `url.*.pushInsteadOf` | parts/task-02.md:369 (H3, both scopes). The cockpit's only project config writes (`branch.<b>.remote`/`.merge` from `push --set-upstream`) are named there |
| 3. `GitHelper.Dir` is a control-plane-owned empty repository | parts/task-13.md:108-112 (the `Dir` comment), Step 3 at :222 (made with `git init` under `OwnedRoot`), and `Scope None`, limited to `OwnedRoot` (task-02.md:79-85 and :113) |
| PushEnv's scoping does not stop a repository proxy | parts/task-13.md:172-174 |
| 4. Red-first tests: proxy and origin `httptest` servers receive nothing after a land; a repo-local helper writes nothing after a push and after `GitHelper.Credential` | parts/task-09.md:198 (land), parts/task-12.md:151-153 (push: repository helper, repository proxy, `pre-push`), and parts/task-13.md:200-202 (`GitHelper`) |

**Not changed:** A4's wording at plan:33-35 gains one sentence pointing at decision 9 (plan:179).
The OD-8 adapter, the contract §2 gate and `desktop/src/store/types.ts` are untouched.
`ConflictReason` lives in the new `api/convergence.ts` (task 18), not in `types.ts`.

### Fix batch b01, round 2 (2026-09-24): the re-review's open issues

Source: `.superpowers/sdd/p3/plan-rereview-b01-1.md`. Line numbers are for the tree at this
round's last commit. Four probes ran on git 2.53.0.windows.2 in `$TEMP` (scripts
`probe-b01*.sh`): a `.git/commondir` redirect, the `credential.helper=` reset against a URL-scoped
helper, a valueless key under `-z`, and what a local fetch runs in its source.

| Re-review item | Where it is now |
|---|---|
| R1 (Critical): `Scope`'s zero value was `Clone` | parts/task-02.md:75 and :86: the zero value is `Unset`, which is `ErrCallShape`. The rule "Scope follows Dir", and `clone` checking its source as `Project`, are at :398. Call sites now name their scope: parts/task-03.md:150, task-08.md:59, :63 and :146, task-11.md:100 and :115, task-14.md:124, task-15.md:110, and task-16.md:64 and :128. Tests: `TestHGHCallShape` (parts/task-02.md:493, `Unset`), `TestHGHProjectAdmitsRealConfig` (task-02.md:487, `vscode-merge-base` and `gh-resolved`). `gitxtest.NewRepo` carries real project keys (task-02.md:269), so a project call site marked `Clone` goes red in its own task |
| R2: `credential.*`/`http.*` refused on every Project call | parts/task-02.md:408: refused on a `Remote` call only. `credential.*` keeps four inert keys admitted there (the helper, cleared by H1's reset, which a probe confirmed for a URL-scoped helper, :358; and `useHttpPath`). The rest stays refused, because the machine helper runs in the project on a `Remote` call and GCM reads its own `credential.trace`/`traceSecrets` from there. No machine helper runs on a `Read` or `Local` call (:358). This repository's config passes (:408, :487). The account change is stated at :414 and in plan Q8 (plan:360) |
| R3: `remote.<name>.proxy` beats H2 | parts/task-02.md:379 (H2 no longer claims it) and :408 (H3 refuses `remote.*.proxy`, `.proxyAuthMethod` and `.vcs` on every Project call). Test: parts/task-12.md:152, `TestPushAndFetchRefuseARepoProxy` case (b) |
| R4: two H2 tests could not fail | parts/task-02.md:491: transport level against a bare TLS server, keyed at exactly `RemoteURL`, with a `--get-urlmatch` precondition. parts/task-12.md:152: keyed at the remote's exact scheme, host and port, with the same precondition |
| R5: other `.git` files steer config discovery | Verified: `.git/commondir` redirected `git config` in a plain repository, and pinning `GIT_DIR` plus `GIT_COMMON_DIR` restored it. parts/task-02.md:447 (the layout pin; `Call.Env` may not override it; the `clone` exception; a `.git`-file project is not pinned, unverified). Test: `TestHGHCommondirCannotRedirect` (:494). The claim is softened at :447 and in plan Q6 (plan:358) |
| Minor 1: task 11 also writes project config | parts/task-02.md:408 names `branch -m` and `config --remove-section`. parts/task-05.md:149 records what the sandbox sees after a host rename under the bind |
| Minor 2: Q7's cost list | plan:359 names `git branch -m` and husky's `prepare`, both unverified. Batch b03 is told in the ledger |
| Minor 3: a valueless key is `key\0` | Verified (`x.flag\0`). parts/task-02.md:392 (the parser) and :488 (`TestHGHConfigParseIsNULSafe` gains a valueless `filter.x.required`) |
| Minor 4: the source side of a local fetch | parts/task-02.md:421. Verified: neither a source-local nor a command-scope `uploadpack.packObjectsHook` ran. Pinned by `TestHGHUploadPackSourceRunsNothing` (:495), a git-behaviour pin |
| Minor 5: `protocol.ext.allow` blocks one transport by name | parts/task-02.md:354: `protocol.allow=never`, with `file` always and `https` on a `Remote` call only. So `gitxtest.HTTPSRemote` (:290) replaces plain `httptest` remotes (parts/task-12.md:149) |

Counts: 35 new tests in task 02, 18 of them §HGH (parts/task-02.md:506 and :524). Mutants added
at Step 3a (task-02.md:514). Decision 9 is restated to match at plan:179.

**Declared rulings (root-cause option taken):**
1. R2 goes beyond the re-review's fix. The re-review said to refuse the credential families on
   `Remote` calls. That still refuses this repository's push (`credential.usehttppath`). So the
   four keys H1 makes inert are admitted, and the rest of the family stays refused.
2. Minor 5 turns https off on `Read` and `Local` calls too. That also closes a lazy fetch from a
   partial-clone promisor remote on a non-network call, which H2 does not harden.
3. The layout pin (R5) is Exec's, not a refusal, because a refusal would have the same
   check-then-call race H5 exists to close.

**Unverified:** GCM reading `credential.trace` from the repository it runs in (from GCM's
documentation). Docker Desktop's nested file bind, and what it shows after a host rename (task 05).
Whether any `.git` file other than `commondir` steers config discovery.

## Fix batch b02 (2026-09-24): C2 and I8

Both findings rewrite task 03's clone step, so one batch does both. C2 is adopted as written
(ruling R1). I8 is the owner's, because OD-2 names hard links: the plan builds the review's
recommendation behind one seam and asks the owner as Q9. Line numbers are for the tree at this
batch's last commit. "parts/" means `2026-09-23-phase-3-git-convergence-parts/`.

### C2: a clone made under the system `core.autocrlf=true` is CRLF on disk

| Review's fix | Where it is now |
|---|---|
| Clone with `-c core.autocrlf=false -c core.eol=lf` **and** write `core.autocrlf=false` into the clone's local config | parts/task-03.md:143-148 (Create step 4). `clone --config` (git's `clone -c`) does both at once: it writes each key into the new repository's local config before the checkout, so the checkout runs under them and the sandbox's git reads the same keys later. The mechanism, and why phase 2's primary-worktree fix takes another route (a lane clone is ours to configure, the user's primary checkout is not), are at parts/task-03.md:167-180 |
| Also write `core.checkStat=minimal` and `core.trustctime=false` | the same argv, parts/task-03.md:144; the reason (task 07's poll re-hashing) at :174-175 |
| A repository whose `.gitattributes` asks for `eol=crlf` keeps it | parts/task-03.md:175-177; asserted through `b.bat` at parts/task-01.md:40 and parts/task-03.md:259 |
| The keys must pass the `Clone` allowlist | parts/task-02.md:415 (§HGH H3 names all four), tested at parts/task-02.md:492 |
| task 01 gains `TestBehaviourSystemAutocrlfReachesAClone`, without `GIT_CONFIG_NOSYSTEM` | parts/task-01.md:28-33: "production-shaped" tests replace `GIT_CONFIG_NOSYSTEM=1` with `GIT_CONFIG_SYSTEM=<tempdir>/system` holding `core.autocrlf=true`. The test is at :40, with its control: a plain clone writes CRLF and no local key |
| `TestSandboxCloneHasWorkingGit` asserts an **empty** `status --porcelain` under host `core.autocrlf=true` | parts/task-01.md:63. A control run (plain `clone --local`) comes first and records whether C2 reproduces in the image |
| task 03's `TestCreateWritesLineEndingConfig` pins the keys | parts/task-03.md:259, production-shaped, red first against a `--local` argv without the `--config` flags. It pins four keys, not three, because `core.eol=lf` persists too |
| (helper, so the tests above cost one call) | parts/task-02.md:286-292: `gitxtest.(*Repo).UseSystemConfig` and `WindowsInstallerSystem` |
| (plan body) | plan:70-75 (the paragraph), plan:496-501 (what b02 verified, and what it did not) |

### I8: hard-linked objects are the project's files, writable from the sandbox

| Review's fix | Where it is now |
|---|---|
| An owner decision, recorded as Q6 | plan:368, recorded as **Q9**, because Q6 to Q8 were taken by fix batch b01. OD-2's row states the departure at plan:131 |
| Task 01 measures `clone --local` against `--no-hardlinks` on `~/Projects/Panacea` | parts/task-01.md:85-92 (Step 5). Q9's answer rests on that wall-clock. **Not measured yet** |
| Use `--no-hardlinks` unless too slow | parts/task-03.md:143-146, with the reason at :186-195. The argv lives in one function, `cloneArgv`, which is the seam Q9's answer changes. The diagram is at plan:50 |
| (tests) | parts/task-01.md:37 (`--local` does share object files, pinned as the reason) and :38 (`--no-hardlinks` shares none); parts/task-03.md:260 (`TestCreateSharesNoObjectFileWithTheProject`, red first against `--local`); `gitxtest.CloneOf` copies too, at parts/task-02.md:284 |
| If hard links stay: `fsck --connectivity-only` before every land, and a "does not claim" residual | Not built. Q9's fallback column (plan:368) names both, for an owner who answers no |
| (knock-on) | parts/task-04.md:169-170 drops the different-volume warning, since no clone needs one volume. parts/task-03.md:299-301 states the per-lane disk cost |

**Verified** (git 2.53.0.windows.2, 2026-09-24, host only; `scratchpad/p3/b02-verify.sh`, output
in `b02-verify.out`, temp directories only). With a test system file holding
`core.autocrlf=true`: a plain clone checked `a.txt` out `x\r\ny\r\n` and had no local
`core.autocrlf`. Task 03's argv checked it out `x\ny\n` and `b.bat` (`eol=crlf`) out CRLF, listed
all four keys locally, and left `status --porcelain` empty. `clone --local` gave the pack, `.idx`,
`.rev`, `info/packs` and `info/commit-graph` the source's file ids with a link count of 2;
`--no-hardlinks` gave every file a new id. Neither clone had `objects/info/alternates`.

**Unverified:** the sandbox's Linux git on a four-key clone, through the bind mount (task-01:63 is
the check). That `checkStat=minimal` and `trustctime=false` stop the re-hash across the two
systems (the probe ran on the host only). What `--no-hardlinks` costs on `~/Projects/Panacea`.

### Fix batch b02, round 2 (2026-09-24): the re-review's open issues

The re-review (`.superpowers/sdd/p3/plan-rereview-b02-1.md`) found I8 resolved and C2 resolved on
the host only. Round 2 fixes its I-1 and M-1. It supersedes round 1's claim, in the C2 table's
first row above, that "the sandbox's git reads the same keys later", and round 1's I8 row naming
`fsck --connectivity-only`. Line numbers are for the tree at this round's last commit.

#### I-1: the sandbox's git reads line endings from the session env, not the clone's config

Facts re-checked: `5a4d0d8` is on `feat/phase-2-cockpit`, not on `fix/sandbox-git-ca`
(`merge-base --is-ancestor` exits 1); that branch merged the fix at `f898f5e`. Its committed
`sessionEnv` (`internal/api/sessions.go:173-187`) is a `pairs` slice: five pairs at `5a4d0d8`,
six now (`gc.auto` was added later). The handlers ask `sessionGitLineEndings(ctx, path)` at
`:543` and `:1142`, and the production resolver is `cmd/api/gitlineendings.go:36-62`.

| Re-review's fix | Where it is now |
|---|---|
| 1. Restate the mechanism, naming `5a4d0d8`: env outranks the clone's keys; they agree because the resolver is asked in the clone; the local keys still serve the host's git and a session without the env | plan:74-81 (the invariant in bold at :79-80); parts/task-03.md:172-173 (the host reads the local keys), :179-202 (the full mechanism, the committed call sites, both probes, and why phase 2 and phase 3 differ on where the answer comes from) |
| 2. A handler-level test, production-shaped under `WindowsInstallerSystem`, red against resolving in `Project.Path` | Split along the package line, because the production resolver lives in package `main`, which `internal/api` cannot import. parts/task-05.md:155 `TestALaneSessionSendsTheClonesLineEndings` (the handler: a lane session's `Spec.Env` carries `false`/`lf` on create and restart, found by key; red against the `Project.Path` mutant). :156 `TestTheResolverReadsALaneCloneAsLF` (cmd/api, production-shaped: the real resolver answers `false`/`lf` in a lane clone and `true`/`crlf` in its project under the installer's system file). Together they pin the composition. Red-first on arrival is covered at :186-188 |
| 3. `TestSandboxCloneHasWorkingGit` gets the env `sessionEnv` produces, plus a checkout-bytes check | parts/task-01.md:57-68 (the six pairs as committed, valued as the resolver answers in the clone) and :72 (`rm a.txt`, `checkout -- a.txt`, no `\r`; control 1 drops the line-ending pairs so it cannot be clean for the wrong reason; control 2 records the `Project.Path` mutant). `gitx` cannot import `internal/api`, so task 01 writes the pairs out once; parts/task-05.md:160 `TestSandboxLaneCloneChecksOutLF` repeats the bytes check with the env read from `sessionEnv`, and :173-176 names where its `eol` comes from |
| 4. One `sessionEnv` across the plan | plan:212 (the fix's pairs, phase 3 appends one); parts/task-05.md:53-74 (the append, not a three-entry map), :154 (`TestSessionEnvSetsHooksPath` asserts `n+1`, not `3`), :180-183 (preflight greps the fix's signature and `core.eol` pair, not `"GIT_CONFIG_COUNT":   "2"`); parts/task-01.md:74 (`core.hooksPath` appended last, not at index 2). G1 names `5a4d0d8`: plan:219-222 and :317 |
| (knock-on) | plan:462: task 05 needs 04 for its lane tests, not only its Docker step (the order already runs 05 after 04). parts/task-05.md:26 and :202-203: the new test file and acceptance lines |

The rest of I12 (plan:11 and :199 still cite `2d6d95b`; tasks 04 and 06 diffing the contract
sections the fix already edited) is left to I12's batch. b02 changed only what describes
`sessionEnv` and what G1 must contain.

#### M-1: Q9's fallback named a check that cannot see I8's damage

plan:377, the "Changes if overruled" column: `fsck --connectivity-only` is gone (git-fsck(1),
checked at `C:/Program Files/Git/mingw64/share/doc/git-doc/git-fsck.html:534`: "in blob objects
will not be detected at all"). It now names a full `git fsck` of the project, slow on a large
repository, and says no check before a land closes the hole, because the files are the
project's from the moment they are written.

**Verified in round 2** (git 2.53.0.windows.2, host only; `scratchpad/p3/b02r2-probe.sh`, output
`b02r2-probe.out`): with a test system file holding `core.autocrlf=true`, the resolver's two calls
answer `true`/`native` in the project and `false`/`lf` in a clone made by task 03's argv. A
checkout under `GIT_CONFIG_*` holding the clone's answer writes `x\ny\n`; under the project's
answer it writes `x\r\ny\r\n`. **Unverified:** everything inside the sandbox image; the re-hash
item above; the `--no-hardlinks` cost.
