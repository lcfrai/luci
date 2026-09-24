### Task 01: Verify the git and sandbox behaviours the plan rests on, and measure clones on this laptop

Every later task leans on git behaving the way its documentation says. The brief lists what it
took from documentation and never ran (`scope-and-risk-phase-3-git-convergence.md` §6): clone
hard-links, `merge-tree --write-tree`, `update-ref`'s compare-and-swap, `prepare-commit-msg` under
`--no-verify`, `GIT_CONFIG_COUNT` against repository config. This programme's planning documents
have been wrong about their own inputs often enough that the first task turns each assumption into
a test that runs real git, so a later task that relies on one is relying on a pinned fact. Risk
R12 (clones on Windows: hard links, path length, antivirus) is measured here too.

This task writes **tests and a results document only**. It adds no production code except a
one-line `doc.go` so the test package has a home.

**Files:**
- `control-plane/internal/gitx/doc.go` — package comment only (new; task 02 fills the package)
- `control-plane/internal/gitx/behaviour_test.go` — host-git behaviour pins (new)
- `control-plane/internal/gitx/sandbox_docker_test.go` — `//go:build docker`, the sandbox half (new)
- `docs/verification/2026-09-2x-phase-3-git-behaviours.md` — results, numbers, and every
  divergence from the plan, dated the day it is run (new)

**Interfaces:** none produced. The results document is consumed by every later task's preflight.

#### The tests to write first

Each test builds its repositories in `t.TempDir()` with the host's `git` (skip with a clear
message if `git` is not on `PATH`), runs git with `GIT_TERMINAL_PROMPT=0`, `GIT_CONFIG_NOSYSTEM=1`,
`GIT_CONFIG_GLOBAL=<tempdir>/empty` and a fixed identity, so the owner's global config cannot make a
test pass or fail. **Production-shaped tests are the exception** (review C2): they replace
`GIT_CONFIG_NOSYSTEM=1` with `GIT_CONFIG_SYSTEM=<tempdir>/system` holding `core.autocrlf=true`.
That is the setting Git for Windows' installer writes, and this machine's system config holds it
(`file:C:/Program Files/Git/etc/gitconfig true`, 2026-09-23). A hermetic run removes exactly the
setting that breaks a real clone. The "mutant" column names the plan assumption the test exists to
catch being false.

| Test | Asserts | Assumption it would catch being false | Consumed by |
|---|---|---|---|
| `TestBehaviourLocalCloneHardLinksObjects` | after `git clone --local <src> <dst>`, a loose object file and a pack file in `dst/.git/objects` satisfy `os.SameFile` with the same file in `src` | that `--local` shares the project's object files, which is why task 03 does not use it (review I8) | 03 |
| `TestBehaviourNoHardlinksCloneSharesNoObjectFile` | after `git clone --no-hardlinks <src> <dst>`, no file under `dst/.git/objects` satisfies `os.SameFile` with the same relative path in `src` | an agent's write to a clone's pack reaching the project (I8, Q9) | 03 |
| `TestBehaviourLocalCloneHasNoAlternates` | after either clone above, `dst/.git/objects/info/alternates` does not exist | a `gc` in the source cannot break a clone | 03 |
| `TestBehaviourSystemAutocrlfReachesAClone` | **production-shaped**. `src` holds LF blobs `a.txt` and `b.bat` and a `.gitattributes` of `*.bat eol=crlf`. A plain `clone --no-hardlinks` writes `a.txt` as `x\r\ny\r\n`, and its `config --local --get core.autocrlf` exits 1. Adding `--config core.autocrlf=false --config core.eol=lf --config core.checkStat=minimal --config core.trustctime=false` writes `a.txt` as `x\ny\n` and `b.bat` as CRLF, lists all four keys in `config --local`, and leaves `status --porcelain` empty | that a clone inherits no line-ending setting, and that task 03's four keys undo the system one (review C2) | 03 |
| `TestBehaviourCloneSymlinksFollowTheSystemConfig` | **production-shaped**, `src` holding a symlink entry `link` → `t.txt` (mode `120000`, via `update-index --cacheinfo`). System file with `core.symlinks=true`: a plain clone writes no local `core.symlinks`, and `link` is a symlink by `os.Lstat` where the OS lets this user make one (record which). System file without it: the clone writes `core.symlinks=false` locally and `link` is a regular file holding `t.txt`. With `--config core.symlinks=false`: the second result under both files. Probed 2026-09-24, `scratchpad/p3/b08/probe2.out` | that a clone's symlink behaviour is the same on every machine, which is why task 03 writes the key (review Minor 15) | 03 |
| `TestBehaviourEmptyFilterEnvChecksOutTheBlob` | **production-shaped**, a system file defining `filter.lfs` with a smudge that writes a marker, `clean = cat`, `required=true` and no `process` (task 03's `TestCreateWritesBlobBytes` fixture: without the clean, a later host `status` exits 128); `src` holds `a.bin` under `*.bin filter=lfs`. A plain clone writes the smudge's output and the marker. With task 03's NoFilterEnv (`GIT_CONFIG_COUNT=3`: `filter.lfs.smudge=""`, `filter.lfs.process=""`, `filter.lfs.required=false`): `a.bin` equals its blob, no marker, exit 0. Probed 2026-09-24, `probe.out` | that an empty command-scope driver disables a system one (review Minor 13) | 03 |
| `TestBehaviourLongPathNeedsCoreLongpaths` | a tree holding a path that puts the checkout past 260 characters: a plain `clone --no-hardlinks` exits 128, stderr holds `Filename too long`, and `warning: Clone succeeded, but checkout failed.`; with `--config core.longpaths=true` exit 0 and `status --porcelain` empty; record both sentences. Probed 2026-09-24, `probe3.out` | that a deep repository clones under the state directory (review Minor 15) | 03, 04 |
| `TestBehaviourCloneBranchSetsUpstream` | `clone --branch feat/x` checks out `feat/x`; `branch.feat/x.remote` is `origin` and `.merge` is `refs/heads/feat/x` | the agent's `git status` shows ahead/behind of `origin/feat/x` | 03, 09 |
| `TestBehaviourMergeTreeCleanExitsZeroWithATree` | `merge-tree --write-tree --name-only -z A B` exits 0, stdout is exactly a 40-hex tree id and one NUL | forecast parsing (review I2: every phase 3 merge-tree call is `-z`) | 07, 08 |
| `TestBehaviourMergeTreeConflictExitsOneWithPaths` | on a conflict over `docs/résumé.md` and `a b.txt`: exit **1**; NUL-separated fields: a tree id, then each conflicted path once and **unquoted** (the raw UTF-8 bytes, not `"docs/r\303\251sum\303\251.md"`), then an empty field, then informational messages. The same call without `-z` prints `docs/résumé.md` in the quoted form and `a b.txt` bare (git quotes `"`, `\`, control characters and, under `core.quotePath`, bytes ≥ 0x80; not a space): record both verbatim, and assert only the résumé path's quoting (re-review b05 nit) | forecast parsing, that exit 1 is not an error, and why `-z` is required (I2) | 07, 08, 09, 10, 16 |
| `TestBehaviourUpdateRefCompareAndSwapRefuses` | `update-ref refs/heads/b NEW OLD` with `b` at a third commit exits non-zero and leaves `b` unmoved; record stderr **verbatim** in the results doc | A5 and R13 | 09, 10 |
| `TestBehaviourUpdateRefCreateOnlyRefusesAnExistingRef` | `update-ref refs/heads/b NEW 0000000000000000000000000000000000000000` fails when `b` exists | create-only branch creation | 11 |
| `TestBehaviourCommitTreeTrailersReadBack` | a `commit-tree` message ending in `Panacea-Brief: brief:b-42@6` reads back through `log --format=%(trailers:key=Panacea-Brief,valueonly)` as exactly `brief:b-42@6` | the merge link is recoverable | 09, 10 |
| `TestBehaviourMergeTreeWithMergeBaseRevertsAMerge` | `merge-tree --write-tree --name-only -z --merge-base=M T M^1` (git ≥ 2.40; the argument list task 10 runs) yields T's tree minus M's change | revert of a land | 10 |
| `TestBehaviourFixedDateCommitTreeIsDeterministic` | two `commit-tree` calls with the same tree, parents, message, identity and `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE=@0 +0000` return the same id | decision 6: a repeated forecast GET writes nothing new | 08 |
| `TestBehaviourStatusWithoutOptionalLocksLeavesTheIndex` | `git status --porcelain=v2 -z` with `GIT_OPTIONAL_LOCKS=0`, after touching a tracked file's mtime, leaves `.git/index`'s bytes and mtime unchanged | observed claims are a read | 07 |
| `TestBehaviourLocalConfigHooksPathIgnoresEnvironment` | with `core.hooksPath=.husky/_` in `.git/config` and `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/x`, `git config --local --get core.hooksPath` prints `.husky/_` | the dispatcher can find the repository's own hooks | 05 |
| `TestBehaviourPrepareCommitMsgRunsUnderNoVerify` | a `prepare-commit-msg` hook that appends a line runs for `git commit --no-verify -m x` | the session trailer survives `--no-verify` | 05 |
| `TestBehaviourRevListLeftRightCountFormat` | `rev-list --left-right --count A...B` prints `<n>\t<m>\n` | ahead/behind parsing | 08 |
| `TestBehaviourGitVarInACloneMissesTheSourcesLocalIdentity` | with `user.name`/`user.email` set only in the source's `.git/config`, `git var GIT_AUTHOR_IDENT` in the clone does **not** print them | the identity copy of task 03 is needed | 03 |
| `TestBehaviourNoWriteFetchHeadLeavesFetchHeadAlone` | `fetch --no-write-fetch-head <bundle file> +refs/heads/b:refs/panacea/lanes/w`, the bundle made by `bundle create` in a scratch clone (R4.4: the host fetches from a bundle file, never from a clone; review r4-1 M-9), leaves `FETCH_HEAD` absent or unchanged; a plain `fetch origin` writes it | lane syncs never masquerade as the last remote fetch | 07, 08, 12 |
| `TestBehaviourCheckRefFormatBranchExpandsShorthand` (review Minor 6) | after `switch -c feature` then `switch main`: `check-ref-format --branch @{-1}` exits 0 and prints `feature\n`; `check-ref-format 'refs/heads/@{-1}'` exits 1; `update-ref 'refs/heads/@{-1}' HEAD 0000…` exits 128 "refusing to update ref with bad name"; `--branch -x` and `--branch HEAD` exit 128 "fatal: '<name>' is not a valid branch name", while `refs/heads/-x` and `refs/heads/HEAD` exit 0; `refs/heads/a..b` exits 1 with empty stderr. Probed 2026-09-24, git 2.53, `b10-probe.out` and `b10-probe2.out` | that ValidBranchName needs `--branch` and the printed-name equality both | 11 |
| `TestBehaviourRemoveSectionExitCodes` (review Minor 12) | `config --remove-section branch.nope` exits 128, stderr `fatal: no such section: branch.nope`; a present section exits 0; with `.git/config.lock` held, remove-section exits 255 `error: could not lock config file .git/config`, and `branch -D x` exits 0, prints `warning: update of config-file failed` and leaves `branch.x.*` in place. Probed 2026-09-24 | DeleteBranch's OKExit 128 and its warn-and-answer rule | 11 |
| `TestBehaviourBranchRenameMovesACheckoutsHead` (review Minor 16) | with `main` checked out, `branch -m main renamed` exits 0 with no output and `symbolic-ref HEAD` then prints `refs/heads/renamed`. Probed 2026-09-24 | that RenameBranch must refuse `branch_checked_out` | 11 |
| `TestBehaviourFetchStartsAutoMaintenance` (review Minor 8) | under `GIT_TRACE=1`, `fetch --no-tags --quiet <src> +refs/heads/main:refs/x` logs `run_command: git maintenance run --auto --quiet --detach`; with `--no-auto-maintenance` no `maintenance` line, even with `maintenance.auto=true` in the repository's `.git/config`. Probed 2026-09-24 | that every host fetch needs the flag | 07, 09, 11, 12 |
| `TestBehaviourRemoveAllOnAClone` | `os.RemoveAll(clone)` succeeds — **or** fails on read-only object files on Windows; record which, and the fix that makes it succeed (clear `FILE_ATTRIBUTE_READONLY` with `os.Chmod(p, 0o600)` during a walk) | worktree removal | 11 |

`sandbox_docker_test.go` (`//go:build docker`), each against `panacea-app/sandbox:dev` with the
clone bind-mounted at `/home/dev/work` and the fix's environment as `sessionEnv` builds it on
`feat/phase-2-cockpit` (`internal/api/sessions.go:215-229` at `10fa5fd`, re-read 2026-09-24, review I12; at or after `5a4d0d8`): the
`GIT_CONFIG_KEY_n`/`VALUE_n` pairs `safe.directory=/home/dev/work`, `user.useConfigOnly=true`,
`gc.worktreePruneExpire=never`, `gc.auto=0`, `core.autocrlf=<A>`, `core.eol=<E>`,
`core.excludesFile=/dev/null`, `core.attributesFile=/dev/null` (the fix's `sessionGitNoFile`,
what a host whose git reads no global excludes or attributes file sends; `d7e2dea`), in that order,
`GIT_CONFIG_COUNT` their number, and `GIT_AUTHOR_*`/`GIT_COMMITTER_*`. `<A>`/`<E>` are task 03's
step-4 constants, `false`/`lf`, which task 05 sends to every lane session without asking git
(R4.2; its `TestALaneSessionSendsTheClonesLineEndings` pins it). The pairs are written out here, once, as a named
list in the test file: this task precedes gate G1, and `gitx` cannot import `internal/api`'s
unexported `sessionEnv`. Task 05's `TestSandboxLaneCloneChecksOutLF` repeats the line-ending
check with the env read from `sessionEnv` itself, so a retyped list that drifts is caught there
(plan re-review b02 I-1):

| Test | Asserts | Consumed by |
|---|---|---|
| `TestSandboxCloneHasWorkingGit` | the host clones a repository of LF blobs with task 03's argv, **production-shaped** (host `core.autocrlf=true`, plus `core.symlinks=true` and an LFS-shaped `filter.lfs`, run with task 03's NoFilterEnv). The fixture also holds a symlink entry and an LFS pointer under `*.bin filter=lfs`, so the empty `status` below covers review Minors 13 and 15 in the real sandbox. In the sandbox, `git status --porcelain` prints **nothing** (an exit code of 0 is not enough: review C2); then `rm a.txt` and `git checkout -- a.txt`, and the host reads `a.txt`'s bytes: they contain no `\r`. That is the only check here that sees an environment `core.autocrlf=true`, which leaves `status` empty (plan re-review b02 I-1). `git commit --allow-empty -m x` exits 0 and the commit's author is the injected identity. Two controls, run first. (1) C2's mechanism: a plain `clone --local` under the same host config, with the env **minus** its two line-ending pairs (a session without them), makes `status --porcelain` list every LF file as modified; record the count, and if it is clean, record that C2's mechanism did not reproduce. With the resolver's real answer for a plain clone (`true`/`crlf`) this control would come out clean for the wrong reason, which is why it drops the pairs. (2) The I-1 mutant: task 03's clone with `<A>`/`<E>` set to `true`/`crlf` (the answer in `Project.Path`) leaves `status` empty, and the checkout writes `x\r\ny\r\n`; record both | 03, 04, 05 |
| `TestSandboxCloneCannotFetch` | the clone's origin set as task 03's step 4a sets it (`remote.origin.url` = NoFetchURL, `.pushurl` = NoPushURL): `git fetch`, `git pull` and `git push origin HEAD` each exit non-zero, and stderr names NoFetchURL (fetch, pull) or NoPushURL (push); record all three sentences verbatim. Control, run first: with origin left at the Windows path `git clone` wrote, record `git fetch`'s sentence (review Minor 14 predicts SSH to a host named `C`; record what it is) | 03, 04; the sentences also wait in the results document for a later agent-docs task (no phase 3 task writes agent-facing docs) |
| `TestSandboxGitHonoursEnvHooksPath` | with `core.hooksPath` appended as the last `GIT_CONFIG_KEY_n` (after the fix's pairs, as task 05 appends it), pointing at a bind-mounted directory holding a `prepare-commit-msg`, a commit carries the hook's line; record `git --version` from the image | 05 |

Exit codes are captured with `cmd.ProcessState.ExitCode()`, never through a pipe (phase 2's
pipe exit-code trap). Docker is driven with `os/exec` and `docker run --rm`; the test builds no
image and skips with a sentence if `panacea-app/sandbox:dev` is absent.

#### Steps

- [ ] **Step 1: Preflight.** `git --version` on the host (expect ≥ 2.40 for `--merge-base`;
  2.53.0.windows.2 on 2026-09-23). `docker image inspect panacea-app/sandbox:dev` (record whether
  it exists). `ls control-plane/internal/gitx` must fail: another agent creating it first is a stop.
- [ ] **Step 2:** write `doc.go` (`// Package gitx runs git for writes. Task 02 builds it.`) and
  `behaviour_test.go` with a small `run(t, dir, env, args...) (stdout, stderr string, code int)`
  helper local to the test file.
- [ ] **Step 3:** run it: `go test ./control-plane/internal/gitx/ -run Behaviour -count=1 -v`.
  **A failing behaviour test is a finding, not a bug to fix here**: keep the test, invert its
  assertion to what git actually does, and record the divergence and the task it changes in the
  results document.
- [ ] **Step 4:** write `sandbox_docker_test.go` and run it with `-tags docker` (PowerShell tool,
  per phase 2's Docker rule).
- [ ] **Step 5: Measure clones (R12).** On the owner's laptop, for this repository and for
  `~/Projects/Panacea` (the large one), record in the results document, for **both**
  `git clone --local` and `git clone --no-hardlinks` (task 03 uses the second; Q9): wall-clock,
  disk before and after (`Get-PSDrive C` free space, or `du -sh` on the clone **and** the source),
  the size of the source's `.git/objects`, whether the state directory and the projects root are on
  the same volume (hard links need one), the longest path in the checkout plus the clone root's
  length against 260 (task 03 writes `core.longpaths=true` into every clone either way, review
  Minor 15), and whether a symlink the checkout holds is a link or a file. Q9's answer rests on the
  `--no-hardlinks` wall-clock for `~/Projects/Panacea`. No antivirus configuration is changed.
- [ ] **Step 6:** write the results document: one row per test, the recorded sentences verbatim,
  the measurements, and a **Divergences** section naming each later task that must change.
- [ ] **Step 7:** commit `doc.go`, both test files and the document, by pathspec.

#### Acceptance

```
go test ./control-plane/internal/gitx/ -run Behaviour -count=1 -v   → PASS, 26 tests: the table's 26 `TestBehaviour*` rows (or the recorded, inverted form)
go test ./control-plane/internal/gitx/ -tags docker -run Sandbox -count=1 -v → PASS, 3 tests, or SKIP naming the missing image
go vet ./control-plane/...                                           → no output
```

#### What this task does not claim

- It verifies git on **this** laptop and in **this** image. A different git on another machine is
  not covered; task 02's runner checks the host git version at startup instead.
- The clone measurements are one machine, one antivirus configuration, one afternoon.
- It does not verify Claude Code behaviour of any kind; task 17 owns the transcript canary.
