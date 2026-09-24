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

## Fix batch b03 (2026-09-24): I11 and I12

Lane (a) setup: clone creation, and the fix's baseline. Runs after b02, because both edit task 03's
Create. Line numbers are for the tree at this batch's last commit. "parts/" means
`2026-09-23-phase-3-git-convergence-parts/`.

### I11: a real clone carries no repository hooks

The review's fix is adopted, and **extended on the facts**. Copying `core.hooksPath` alone still
runs nothing on a husky 9 project: husky's `.husky/_` is generated and ignored, so a clone never
has it, and the agent's install step cannot make it inside a lane, because its
`git config core.hooksPath` write fails against the read-only `.git/config` that task-02 §HGH H5
binds (the plan's own Q7 already said so). So Create also copies the generated directory the
copied setting names, under guards. What husky 9 does after that failed write (stop before it
creates `.husky/_`) is from memory of its source, which was not read for this batch: **unverified**.
The design does not depend on it, since the directory is copied either way.

| Review's fix | Where it is now |
|---|---|
| At clone time, copy a **relative** `core.hooksPath` from the project's local config | parts/task-03.md:152-158 (Create step 6; step 7 is `Store.Add`, and the cleanup and Scope lines at :160-164 now say 4-7 and name step 6's calls). The rule is `CopyableHooksPath`, parts/task-03.md:167-173: H3's Clone rule for the key (relative, no `..`), plus no `\`, because the sandbox's Linux git reads `\` as part of a file name |
| Never copy `.git/hooks` contents | parts/task-03.md:155-156, asserted in `TestCreateCopiesARelativeHooksPath` (parts/task-03.md:322: the clone's `.git/hooks` has no `pre-commit`) |
| (extension) the generated directory the setting names | `CopyGeneratedHooks`, parts/task-03.md:175-194: only an untracked directory, reached through real directories only (no symlink or junction, so nothing outside the project's tree or inside `.git` is read), flat, regular files, at most 64 files and 1 MiB; the host never runs them (H1). Tests at parts/task-03.md:324-325 |
| Add `TestCreateCopiesARelativeHooksPath` | parts/task-03.md:322, with the refusal table (`/abs`, `C:/`, `C:hooks`, `\hooks`, `~/`, `../x`, `a/../../x`, `.githooks\x`); `TestCopyableHooksPath` :323; `TestCreateHooksPathPassesTheCloneCheck` :326 (a copied value never makes H3 refuse the lane) |
| The Docker husky test builds its clone with `lanes.Creator` from a project whose config carries the setting | parts/task-05.md:176 (`TestDispatchChainsToHusky`: the test writes nothing into the clone; red first against a Creator without step 6); the hazard paragraph says so at parts/task-05.md:13-16 |
| State fresh-clone semantics in task 05's "does not claim" | parts/task-05.md:232-242; task 03's side at parts/task-03.md:368-373 |
| (extension) the dispatcher names the loss it can see (round 2: a lane made before install, an absolute path and `.git/hooks` stay silent, and the plan says so; re-review b03 I-3) | the dispatcher prints one `panacea: core.hooksPath names …` line per commit when the configured directory is missing, parts/task-05.md:123-129, tested by `TestDispatchNamesAMissingHooksDir` at :177 |
| (extension) the premise is pinned | `TestSandboxCannotWriteHooksPath`, parts/task-05.md:178 and its acceptance line :219 |
| Prove it in task 23 | parts/task-23.md:27-32 (the e2e project laid out as husky's install step leaves one), :38 (step 3: every sandbox commit carries `X-Husky: ran`), :41 (step 6: HGH H1; round 2 replaced the trailer check with a host-only `reference-transaction` marker, see below), :81-85 (owner item 5 runs the real install step first) |
| Restatements | plan:399 (Q7 cites the pinning test and the copy); parts/task-02.md:415 (H3's Clone row names Create as a writer of the key) |

Also fixed on the way: task 05's acceptance said `-run Dispatch` runs 8 tests when the table named
7. With `TestDispatchNamesAMissingHooksDir` it names 8, so parts/task-05.md:217 is now true.

### I12: the fix branch has moved since the plan read it

Re-read on 2026-09-24. The review's facts hold, with one correction and one addition:

- **Correction.** The fix branch's contract edits (`2d6d95b`, `5e32000`; `git diff 3303fd6...fix/sandbox-git-ca`, 55 Go lines
  and 7 UI lines, as the review says) touch Go §2.2 (`5e32000`: §2.2, UI §6.5, §7.7; `2d6d95b`: §2.6, §4, §5.1), §2.6, §4 and §5.1 and UI §6.5 and §7.7,
  found by mapping each hunk to its heading. It does not touch §3.7.
- **Addition.** Phase 3's branch forks from `f898f5e`, `feat/phase-2-cockpit`'s merge of the fix,
  so `4f6d698` is already in phase 3's base. Phase 2's later sandbox-git rounds (`5a4d0d8` to
  `10fa5fd`) are not in it, and they changed two shapes the plan quoted: `sessionMounts` now takes
  five parameters (`worktreeAdmin`, `git sessionGitFiles`), and `sessionEnv` takes four with
  eight `GIT_CONFIG_*` pairs (`core.excludesFile` and `core.attributesFile` added by `d7e2dea`).
  `Deps.GitIdentity` and `LinkedWorktreeNote`'s condition and sentence are unchanged.

| Review's fix | Where it is now |
|---|---|
| Re-cite plan:11-12 | plan:11-15 |
| Re-cite plan:179-181 (now plan:208-225) | plan:208-225: the five commits on the fix branch, the sweep's sections, the fork point, phase 2's later rounds, and the shapes at `10fa5fd`; the table's two rows at plan:230-231 quote those shapes (five-argument `sessionMounts`; eight pairs, six at `57c3bf5`) |
| Re-baseline G1 to `4f6d698` | plan:236-248 (the gate) and plan:341 (the gates table); plan:520-523 ("does not claim") |
| Tasks 04-06's preflight diffs the contract sections the fix already edited | parts/task-04.md:234-235 (Go §3.1, §3.2); parts/task-05.md:198-200 (Go §2, §5.1); parts/task-06.md:141-143 (UI §2, §6.5, §7.7) |
| Re-cite task-06:3-4 | parts/task-06.md:3-10: `58d607f`, in the base through `f898f5e`, unchanged at `4f6d698` and `10fa5fd` |
| (addition) task 06 amends the rows the fix wrote | UI §6.5's `LinkedWorktreeNote.tsx` row and §7.7's linked-worktree row still say `isPrimary === false`; task 06 now amends both and adds a `SharedIndexNote` row: parts/task-06.md:24 (Files) and :145-148 (Step 3) |
| (restatement) task 05's `sessionEnv` quote | parts/task-05.md:55-62 and :66-67 (four parameters, eight pairs at `10fa5fd`); :166 (`n` is 8 at `10fa5fd`, was "6 at the time of writing") |
| (restatement; b02 round 2's M-A overlaps) the `sessions.go` line citations and task 01's written-out pairs | parts/task-03.md:218-223 (`:215-229`, `:651`, `:1255`, `:250` at `10fa5fd`, where they said `:173-187`, `:543`, `:1142`, `:208`); parts/task-01.md:59-62 (the cite, and the two pairs `d7e2dea` added, `core.excludesFile` and `core.attributesFile`, each `/dev/null` for a host that reads no such file) |

**Unverified.** `feat/phase-2-cockpit` is still moving (its sandbox-git round is in progress
tonight), so the shapes above are true at `10fa5fd` only. G1's preflight re-reads them, which is
why it stays a gate rather than a quotation.

### Fix batch b03, round 2 (2026-09-24): the re-review's open issues

Source: `.superpowers/sdd/p3/plan-rereview-b03-1.md` (local). Every issue is fixed; none is
deferred. Line numbers are for the tree at this round's last commit.

| Re-review issue | Where it is now |
|---|---|
| **I-1** `.git/hooks` copied; a hooks folder tracked on the lane's branch overwritten | parts/task-03.md:198-201 (no `.git` segment, case-folded, trailing dots and spaces trimmed) and :202-205 (`os.Lstat(<cloneTop>/<rel>)` must report not-exist, which replaces the project-index `ls-files` guard: wrong index). Rows `.git/hooks`, `.GIT/hooks`, `.git./hooks` and tracked-on-the-lane-branch-only in `TestCopyGeneratedHooksRefuses`, parts/task-03.md:356 |
| **I-2** check-then-use on a folder the sandbox can write (A4) | Root cause: nothing stopped a sandbox mounting the project's tree mid-copy, and session create had no lock to share (at `8236ba1` only restart's per-id `sessionIDSet`, sessions.go:913-938). New `lanes.MountGuard`, parts/task-03.md:137-147; step 6 runs the copy inside `WhileUnmounted`, :170-173 (ran false or Mounts nil copies nothing, not an error). `api.MountGuard` orders session create's and restart's `StatusStarting` write (`Store.Create` at `10fa5fd`:689, `Store.Update` :1316, each before `Backend.Create` :729, :1340) against the copy, and fails closed on a registry error: parts/task-04.md:26-27 (Files), :62-89 (type), :234-243 (wiring), :265-269 (tests). Open-then-`SameFile` and `io.LimitReader`: parts/task-03.md:210-215, test :357. Zero files with a live session: :358. Plan file map :438-442, Q7 :403 |
| **I-3** the silent case was claimed to print | parts/task-05.md:125-129 (the dispatcher's comment) and :241-251 ("does not claim"); parts/task-03.md:401-414. Each lists what prints and the three silent cases (made before install, absolute path, `.git/hooks`), and which of them re-creating the lane repairs. Review row :701 corrected |
| **I-4** G1's stop at task 04 checked nothing | parts/task-04.md:11-18 and :274 use task 05's two greps. Verified: both give 1 at `10fa5fd`; the signature grep gives 0 at `8236ba1`. Plan G1, plan:241-246, says both tasks run them, and why task 04 needs the round (it now edits session create and restart) |
| Minor 1: task 23 step 6 could not fail | parts/task-23.md:27-42: a tracked `.husky/reference-transaction` writes a marker in a host-only directory, and a control `update-ref` before step 1 shows the marker appears when host git runs the hook; step 6, :53, asserts it is absent |
| Minor 2: not husky's real layout | parts/task-05.md:13-16 and :180 and parts/task-23.md:27-35 use husky 9's stub shape (`. "${0%/*}/h"`) with a stand-in `h`, and say the real `h` is owner item 5's |
| Minor 3: an unignored untracked folder gets committed | parts/task-03.md:216-219 (the clone's `ls-files --others --exclude-standard` must print nothing, else the copy is removed); row in :356 |
| Minor 4: `5e32000` over-credited | plan:213-216; parts/task-05.md:203-204; review :713-715 |
| Minor 5: task 06 Step 3 misses two phrases | parts/task-06.md:148-154 (contract line 1770 at `10fa5fd`: "arrives in Phase 3" and round 1's "not primary" clause, both rewritten, with the grep that finds them) |

Also on the way: `TestCreateCopiesAGeneratedHooksDir` (parts/task-03.md:355) and
`TestDispatchChainsToHusky` (parts/task-05.md:180) now give `Creator.Mounts` a fake that runs
fn, since nil copies nothing.

**Unverified.** Whether a container can make a Win32-followable link on Docker Desktop's bind
mount is still not run. The design no longer depends on it: a live or starting primary-worktree
session stops the copy. The restart line number (`:1316`) is `10fa5fd`'s; task 04 re-locates both
call sites by grep.

## Fix batch b04 (2026-09-24): I6 and I7

Lane observation and task ordering. Line numbers are for the tree at this batch's last commit.
"parts/" means `2026-09-23-phase-3-git-convergence-parts/`.

### I7: briefref and the lock get owners; no dependency edge changes

The review's first option is adopted: `briefref` moves to task 03, and `gitx.Locks` is added to
task 02. **The orchestrator's dependsOn needs no new edge.** 11 still depends on 07 (which
follows 03), and 12 on 02 and 04. Lane (c) needs only 07 from lane (b) before task 14
(corrected in round 2: task 11 waits on 07).

Two changes go past the review, on the facts:
- **The lock is keyed by project ID, not by path.** Every caller takes the ID from the one
  catalog entry, which has one spelling (`project.go:29`, `:102`); a path handed in by each caller
  could be spelled several ways on Windows (re-review b03 N-1: this machine's `%TEMP%` is 8.3).
  The ID does not normalise 8.3 or junction spellings itself (`project.go:86-97`: absolute,
  clean, lowercase only), so two catalog entries for one repository still get two locks, and
  compare-and-swap still refuses the double land (corrected in round 2).
- **The instance needs an owner too.** Task 04 builds the only one, in `GitDeps.Locks`.
  Undo and revert (task 10) and the three branch verbs (task 11) take it as well, because they
  move `refs/heads/*` or `branch.<name>.*`, which a land or a push reads.

| Review's fix | Where it is now |
|---|---|
| Move `briefref.go` into task 03 or 04 | parts/task-03.md:28-29 (Files), :338-374 (the interfaces, moved verbatim, with the reason), :420 (Step 3), :428 (`go vet`); removed from parts/task-09.md:15-23, which now consumes it (:22); plan:122 (package table), :430 (file map), :496 (task table row 03), :502 (row 09); plan:372, :374 (P4-1, P4-3 name the declaring task); parts/task-15.md:180 |
| Add `gitx.Locks` (per project, one per process) to task 02 | parts/task-02.md:22-23 (Files), :226-250 (type, key, lock order), :364-366 (three tests), :548 (Step 3), acceptance 35 → 38 tests; plan:116 (package table), :421 (file map), :495 (row 02), :98-100 (the rule, beside the land paragraph) |
| `Lander`, `Push` and `Fetch` all take it | Lander: parts/task-09.md:62-66 (field), :79 (step 0), :166 (`TestLandWaitsOnTheProjectLock`, through the handler too), :173-179 (preflight, wiring). Fetcher and Push: parts/task-12.md:40-53 (`ProjectRef`, `Locks`), :79, :116, :143-150 (the paragraph replacing "one `sync.Mutex` per project path"), :175 (`TestPushAndFetchWaitOnTheProjectLock`, which replaces `TestPushAndLandSerialise`, the test that needed task 09's `Lander`) |
| (extension) one instance, and every branch mover | parts/task-04.md:67-71 (`GitDeps.Locks`), :248 (main.go builds it), :271 (`TestServeWiresGit` asserts it); parts/task-10.md:71-72, :90-91, :149; parts/task-11.md:129-132 and the two signatures after it, :230 (`TestBranchVerbsWaitOnTheProjectLock`), :235-236 (preflight) |
| Or add the edges 11→09 and 12→09 and redraw the lanes | Not taken. plan:521-524 says why the lanes stay as drawn |

### I6: a clone HEAD off the lane's branch is reported, and its work archived before removal

The review's fix is adopted, with **one correction on the facts**. The review archives into
`refs/panacea/archive/lanes/<id>/<unix>/<name>`. Git cannot write that: step 1 already writes
`refs/panacea/archive/lanes/<id>/<unix>`, which is a ref, and git refuses a ref beneath another
ref. The probe below shows git's refusal. So off-branch work goes to a sibling namespace,
`refs/panacea/archive/offbranch/<id>/<unix>/heads/<name>`, and a detached HEAD goes to
`…/HEAD`. Contract Q5's erasure scope, `refs/panacea/archive/*`, still covers both.

A second addition: a clone with a broken or hostile ref must stay removable. So `Inspect` runs
the archive itself, for real, and a failure becomes `Unarchivable`. Its presence is in the token
and its sentence is in the question, so the human confirms the loss knowingly. (Round 1 probed
with `fetch --dry-run` and fetched again at removal; round 2 replaced that, because a dry run
cannot see a ref-write failure. See "Fix batch b04, round 2" below.)

| Review's fix | Where it is now |
|---|---|
| `Observe` reads `symbolic-ref -q HEAD`; `LaneObservation` gains `headElsewhere` | parts/task-07.md:117-121 (field), :135-137 (the calls, Scope Clone); plan:91-93 |
| The card says so | parts/task-19.md:45-48 (`headElsewhereLine`), :159; the wire at parts/task-08.md:185 and parts/task-18.md:45 |
| `Inspect` archives every clone ref under `refs/heads/` the lane does not reach | parts/task-11.md:42-51 (`Elsewhere`, `Unarchivable`), :57-58 (token), :63-64 (`NothingToLose`), :70-79 (the listing and the dry-run probe), :87-100 (`ArchivedElsewhere`, `OffBranchPrefix`, `OffBranchRefspecs`), :104-108 (Remove step 1a), wire :176-181, intro :12-14, "does not claim" (stash, tags and reflog are not archived) |
| `TestObserveReportsAHeadOffTheBranch` | parts/task-07.md:177 |
| `TestRemoveArchivesOtherClonesBranches` | parts/task-11.md:212; also `TestRemoveIgnoresBranchesTheLaneContains` :213, `TestRemoveAnUnarchivableCloneIsAskedNotStuck` :214, and the hostile-clone row extended to the wildcard archive |
| Restatements | parts/task-22.md:48-49 (`RemovalQuestion.elsewhere`, `.unarchivable`), :106-111 (four headings), :121 (the test, "names all four losses"); parts/task-23.md:65 (step 18) |

**Verified on this machine** (git 2.53.0.windows.2, isolated config, script
`scratchpad/p3/b04-probe.sh`):
- `update-ref refs/panacea/archive/lanes/w1/100/fix` beside `…/100` fails: exit 128, "cannot
  lock ref … 'refs/panacea/archive/lanes/w1/100' exists".
- `for-each-ref --no-merged=refs/heads/main refs/heads/` lists `fix-x` and omits `old`, a branch
  `main` contains.
- `symbolic-ref -q HEAD` exits 1 when HEAD is detached.
- `fetch --dry-run` with the named refspecs exits 0 and writes no ref.
- The real fetch writes `…/offbranch/w1/100/heads/fix-x` and `…/HEAD`, and `HEAD` names the
  detached commit.

**Unverified.**
- Which call fails first for a loose ref that names a missing object: the listing, or the
  archive. Since round 2 the first ends in `Unlisted` (and a wildcard archive), the second in
  `Unarchivable`; either way the human is asked.

### Fix batch b04, round 2: re-review b04-1's five issues

Source: `.superpowers/sdd/p3/plan-rereview-b04-1.md`. The dependency graph is unchanged by this
round: **dependsOn stays 11←07, 12←02,04, 15←07,09**, and no new edge is added.

| Re-review issue | Fix | Where it is now |
|---|---|---|
| 1 (Important): a dry run cannot see a ref-write failure, so step 1a failed on every retry | Root-cause option taken: `Inspect` runs the real named-refspec archive fetch into `OffBranchPrefix(id, now)` and sets `Unarchivable` from its failure; `Remove` step 1a runs no git and reports `st.ArchivedAt`; the 500 `git_failed` path is gone. The token hashes whether `Unarchivable` is set, not git's wording. One `now` per request feeds both `Inspect` and `Remove`. The D/F case is in the unarchivable test | parts/task-11.md:51-61 (`ArchivedAt`, `Unarchivable`, why not a dry run), :67-75 (token), :82-102 (`Inspect`, now with `now`), :108 (`ArchivedElsewhere`), :112 (`OffBranchPrefix`), :126-134 (step 1a, one `now`), :203-204 (wire: `confirm_stale` replaces the 500), :219-221 (handler), :238 (test: D/F pair, the same token a second apart, a fault in `Inspect`'s archive), :282-287 ("does not claim": repeated archives; the wording) |
| 2 (Important): a failed listing was not in `RemovalState` | `Unlisted` (git's sentence) added, and put in `Confirm`, `NothingToLose`, the archive condition, `OffBranchRefspecs`' wildcard and `details.unlisted` | parts/task-11.md:47-50 (field), :68 (token), :79-80 (`NothingToLose`), :92-94 (set by the listing; archive condition), :120-121 (wildcard), :201 (wire), :239 (`TestRemoveWithAnUnlistedCloneArchivesTheWildcard`, a listing-fault Runner, red first); restated in parts/task-22.md:49 (`RemovalQuestion.unlisted`), :111-114 (the fourth heading), :124 (the test) |
| 3 (Minor): no test proves the Lander and the Fetcher share `Git.Locks` | Tasks 09 and 12 each extend task 04's `TestServeWiresGit`, red first, with a same-pointer assertion | parts/task-09.md:19 (Files), :168 (test row), :187 (acceptance now runs `./control-plane/cmd/api/`); parts/task-12.md:26 (Files), :150-151 (the paragraph), :178 (test row) |
| 4 (Minor): "lane (c) needs nothing from lane (b)" was false | Now "needs only 07 from lane (b) before 14" | plan:521-522; this file's round-1 text at :773-774 |
| 5 (Minor): the lock-key reason overstated what the ID normalises | Reworded: the ID helps because every caller takes it from one catalog entry; `hashPath` only does Abs, Clean and lowercase, so two catalog entries for one repository still get two locks, and compare-and-swap still refuses the double land | parts/task-02.md:236-243 (checked against `project.go:86-102`); this file's round-1 text at :777-782 |

**Verified on this machine** (git 2.53.0.windows.2, isolated config;
`scratchpad/p3/b04r2-probe.sh`, output in `b04r2-probe.out`), on the re-reviewer's D/F clone:
- The real `--quiet` fetch exits 1 with "error: some local refs could not be updated; try running
  'git remote prune ../clone' …". The sentence is the same for prefixes `…/100` and `…/101`, and
  again on a same-prefix retry, so it does not name the prefix under `--quiet`.
- Without `--quiet`, git prints one line per ref naming `…/102/heads/d`, so the prefix does reach
  git's output there. That, and a timeout's duration, is why the token hashes only presence.
- Each failed attempt leaves `…/<unix>/heads/d/e` written and `…/heads/d` not. "It only adds refs"
  (parts/task-11.md:98) holds.

**Unverified.**
- Issue 2's real-world trigger (a listing that fails while the wildcard archive succeeds) is
  still unproven. The interface now covers it either way.
- The path-length and held-lock variants of issue 1. They are ref-write failures like D/F, so the
  real archive sees them, but no probe was run.

## Fix batch b05 (2026-09-24): I2 and I5

The forecast and land pipeline. Line numbers are for the tree at this batch's last commit.
"parts/" means `2026-09-23-phase-3-git-convergence-parts/`.

### I2: every merge-tree call is `-z`, parsed once, and `details.paths` is a JSON array

Adopted as the review writes it, in its first form for the paths: one JSON-encoded array
string. Phase 2's problem envelope keeps `details` a `map[string]string`
(`httpx.WriteProblem`; `api/sessions.go:579`), so a `[]string` field would change a phase 2
contract. **One parser, not four:** the review names four call sites. Each now calls
`converge.MergeTreeArgs` and `converge.ParseMergeTree`, declared once in task 07's `parse.go`
beside the other `-z` parsers.

**Probed** on git 2.53.0.windows.2 (`scratchpad/p3/b05-mt-probe.sh`, output in `.out`). Without
`-z` the conflicted path prints as `"docs/r\303\251sum\303\251.md"`. With `-z` the output is: a
tree id, NUL, then each path raw and NUL-terminated, then an empty field, then the messages. A
clean merge prints the tree id and one NUL and exits 0. The `--merge-base` form takes `-z` too.

| Review's fix | Where it is now |
|---|---|
| Add `-z` to every `merge-tree` call | parts/task-08.md:48-49 (forecast), parts/task-09.md:91-92 (land step 9), parts/task-10.md:86-88 (revert), parts/task-16.md:63-66 (pairwise marks); parts/task-09.md:202-203 (the new bring-in, I5) |
| Parse NUL-separated output | parts/task-07.md:16 (Files), :141-155 (`MergeTreeArgs`, `ParseMergeTree`: shape checks against the exit code, sorted, de-duplicated), :198-199 (two tests) |
| Task 01 pins the `-z` form | parts/task-01.md:42-43 (clean and conflict pins over `docs/résumé.md` and `a b.txt`, both forms recorded verbatim), :47 (the revert pin runs task 10's argument list) |
| `details.paths` as one JSON array | parts/task-04.md:104, :130-139 (`detailList`, and why), :278 (`TestDetailListRoundTripsAnyPath`); parts/task-11.md:200 (confirm_required); parts/task-18.md:178-181 (desktop `detailList`, never a `\n` split), :285 (its test); parts/task-22.md:46 |
| Test with a conflicted `é`-named file | parts/task-07.md:198 (built with plumbing, because NTFS forbids `\n` in a name), parts/task-08.md:230 (the row above is blamed, not the branch), parts/task-09.md:234, parts/task-10.md:148, parts/task-16.md:188 |
| (extension) diff and blame | parts/task-16.md:141-143: task 16's parsers take the path from the request and never read one back from a C-quoted `diff --git`, `---`/`+++` or porcelain `filename` line |

Not changed, on purpose: `details.commits` and `details.elsewhere` keep their line formats (they
hold shas and ref names, and `check-ref-format` refuses a control character). `details.sessions`
holds user titles joined by `\t` and `\n`. Whether phase 2 lets a title hold a newline is
**unverified**, and it is outside I2's scope, so it is deferred.

### I5: every clone gets `upstream/<branch>`, and the lens can bring the remote's commits in

The review's first option is adopted: the root cause, not the copy. It is recorded as **Q10**
(plan:415), built on that recommendation. Overruling it gives the review's terminal-checkout
copy. The origin refresh stays C3's literal-path fetch (b01), made **one** function,
`Lander.RefreshClones`, which land, undo, bring-in and a project fetch all call.

**Probed** on git 2.53.0.windows.2 (`scratchpad/p3/b05-up-probe.sh`, output in `.out`):
- `rev-parse --verify -q --symbolic-full-name <b>@{upstream}` exits 1 silently when no upstream
  is set, and prints `refs/remotes/origin/main` when one is.
- The two-refspec fetch from the literal project path copies the project's origin ref to the
  clone's `refs/remotes/upstream/main`, and writes no remote into the clone's config.
- A fetch whose source ref is missing exits 128, so the upstream refspec is conditional.
- `update-ref -d` of an absent ref exits 0.
- `git merge upstream/main` in the clone exits 0.

| Review's fix | Where it is now |
|---|---|
| The origin refresh also copies `refs/remotes/origin/<b>` as `refs/remotes/upstream/<b>` | parts/task-09.md:101 (step 14), :107-122 (`RefreshClones`: `<branch>@{upstream}` read once, the refspec added only when it exists, a stale copy deleted, no credential and no URL reach the clone, A4), :124-129 (`RefreshProject`, lock-free because `gitx.Locks` is not re-entrant), :247-249 (three tests); parts/task-10.md:70 (undo calls it); plan:179-184 (decision 3), :57-59 (diagram) |
| (extension) a fetch refreshes the clones | parts/task-12.md:58-63 (`Fetcher.AfterFetch`, a func because `converge` imports `remote`), :177 (the lock is still held), :195; parts/task-14.md:20, :169: task 14, the first task after both 09 and 12, wires it, so plan:531-535 stays true (and now says so) and **no new edge** appears; plan:496-498 (task 09 is 4 h, and where it splits), :510 (row 09) |
| A "Bring in origin/<branch>" row when `behind > 0` | parts/task-08.md:107-129 (`Convergence.BringIn`, forecast against the tip alone, never chained into the queue), :195, :228-229; parts/task-18.md:61-67, :78-82, :203; parts/task-19.md:19, :64-75, :97-100, :112-115, :145-151, :185-188 (the row, no Undo, no force, never part of Land all, a conflict resolved in a picked lane) |
| The same merge-tree, commit-tree and update-ref land, refused on conflict (A2) | parts/task-09.md:16-18 (Files), :164-218 (`BringIn`: compare-and-swap on both the tip and the upstream the human saw, fast-forward when only behind, merge-tree `-z` otherwise, no trailer, so undo refuses it as `not_a_land`), :250-254 (five tests); parts/task-04.md:101-117 (`upstream_moved`, and the routes each reason names); parts/task-18.md:167 |
| Resolution in a lane through the refreshed ref | parts/task-19.md:71-75 (`bringInResolvePrompt` names `upstream/<branch>`, not `origin/<branch>`, which in a clone is the project's branch), :187 |
| Correct the `push_rejected` sentence | parts/task-20.md:79-83 (it names fetch and bring-in, both of which exist), :28-29, :120 |

Two choices past the review, on the facts:
- **A bring-in fast-forwards when the branch is only behind.** A land never does (3-D4). A
  bring-in records no lane's work, and a merge commit there would leave the branch ahead of
  origin for no reason.
- **`upstream_moved` is a new reason**, not an overload of `branch_moved`. The bring-in
  compare-and-swaps two refs, and the banner must say which one moved.

**Unverified / deferred.**
- Task 23's end-to-end script does not exercise a bring-in yet. Deferred to the proof batch.
- The I4 batch still owns refreshing a lane just before `ResolveDialog` starts. Until then, a
  bring-in conflict resolves against the upstream as of the last land, undo, bring-in or fetch.
