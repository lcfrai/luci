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
test pass or fail. The "mutant" column names the plan assumption the test exists to catch being
false.

| Test | Asserts | Assumption it would catch being false | Consumed by |
|---|---|---|---|
| `TestBehaviourLocalCloneHardLinksObjects` | after `git clone --local <src> <dst>`, a loose object file and a pack file in `dst/.git/objects` satisfy `os.SameFile` with the same file in `src` | clones are cheap (R12) | 03 |
| `TestBehaviourLocalCloneHasNoAlternates` | `dst/.git/objects/info/alternates` does not exist | a `gc` in the source cannot break a clone | 03 |
| `TestBehaviourCloneBranchSetsUpstream` | `clone --branch feat/x` checks out `feat/x`; `branch.feat/x.remote` is `origin` and `.merge` is `refs/heads/feat/x` | the agent's `git status` shows ahead/behind of `origin/feat/x` | 03, 09 |
| `TestBehaviourMergeTreeCleanExitsZeroWithATree` | `merge-tree --write-tree --name-only A B` exits 0, stdout is exactly one line, a 40-hex tree id | forecast parsing | 08 |
| `TestBehaviourMergeTreeConflictExitsOneWithPaths` | on a conflict: exit **1**, line 1 a tree id, then each conflicted path once, then an empty line, then informational messages | forecast parsing, and that exit 1 is not an error | 08, 09 |
| `TestBehaviourUpdateRefCompareAndSwapRefuses` | `update-ref refs/heads/b NEW OLD` with `b` at a third commit exits non-zero and leaves `b` unmoved; record stderr **verbatim** in the results doc | A5 and R13 | 09, 10 |
| `TestBehaviourUpdateRefCreateOnlyRefusesAnExistingRef` | `update-ref refs/heads/b NEW 0000000000000000000000000000000000000000` fails when `b` exists | create-only branch creation | 11 |
| `TestBehaviourCommitTreeTrailersReadBack` | a `commit-tree` message ending in `Panacea-Brief: brief:b-42@6` reads back through `log --format=%(trailers:key=Panacea-Brief,valueonly)` as exactly `brief:b-42@6` | the merge link is recoverable | 09, 10 |
| `TestBehaviourMergeTreeWithMergeBaseRevertsAMerge` | `merge-tree --write-tree --merge-base=M T M^1` (git ≥ 2.40) yields T's tree minus M's change | revert of a land | 10 |
| `TestBehaviourFixedDateCommitTreeIsDeterministic` | two `commit-tree` calls with the same tree, parents, message, identity and `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE=@0 +0000` return the same id | decision 6: a repeated forecast GET writes nothing new | 08 |
| `TestBehaviourStatusWithoutOptionalLocksLeavesTheIndex` | `git status --porcelain=v2 -z` with `GIT_OPTIONAL_LOCKS=0`, after touching a tracked file's mtime, leaves `.git/index`'s bytes and mtime unchanged | observed claims are a read | 07 |
| `TestBehaviourLocalConfigHooksPathIgnoresEnvironment` | with `core.hooksPath=.husky/_` in `.git/config` and `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/x`, `git config --local --get core.hooksPath` prints `.husky/_` | the dispatcher can find the repository's own hooks | 05 |
| `TestBehaviourPrepareCommitMsgRunsUnderNoVerify` | a `prepare-commit-msg` hook that appends a line runs for `git commit --no-verify -m x` | the session trailer survives `--no-verify` | 05 |
| `TestBehaviourRevListLeftRightCountFormat` | `rev-list --left-right --count A...B` prints `<n>\t<m>\n` | ahead/behind parsing | 08 |
| `TestBehaviourGitVarInACloneMissesTheSourcesLocalIdentity` | with `user.name`/`user.email` set only in the source's `.git/config`, `git var GIT_AUTHOR_IDENT` in the clone does **not** print them | the identity copy of task 03 is needed | 03 |
| `TestBehaviourNoWriteFetchHeadLeavesFetchHeadAlone` | `fetch --no-write-fetch-head <clone> +refs/heads/b:refs/panacea/lanes/w` leaves `FETCH_HEAD` absent or unchanged; a plain `fetch origin` writes it | lane syncs never masquerade as the last remote fetch | 07, 08, 12 |
| `TestBehaviourRemoveAllOnAClone` | `os.RemoveAll(clone)` succeeds — **or** fails on read-only object files on Windows; record which, and the fix that makes it succeed (clear `FILE_ATTRIBUTE_READONLY` with `os.Chmod(p, 0o600)` during a walk) | worktree removal | 11 |

`sandbox_docker_test.go` (`//go:build docker`), each against `panacea-app/sandbox:dev` with the
clone bind-mounted at `/home/dev/work` and the fix's environment (`GIT_CONFIG_COUNT=2`,
`safe.directory=/home/dev/work`, `user.useConfigOnly=true`, `GIT_AUTHOR_*`/`GIT_COMMITTER_*`):

| Test | Asserts | Consumed by |
|---|---|---|
| `TestSandboxCloneHasWorkingGit` | `git status` exits 0; `git commit --allow-empty -m x` exits 0 and the commit's author is the injected identity | 03, 04 |
| `TestSandboxCloneCannotFetch` | `git fetch` exits non-zero; record the sentence verbatim | 04, and the agent-facing docs of task 23 |
| `TestSandboxGitHonoursEnvHooksPath` | with `GIT_CONFIG_KEY_2=core.hooksPath` pointing at a bind-mounted directory holding a `prepare-commit-msg`, a commit carries the hook's line; record `git --version` from the image | 05 |

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
  `~/Projects/Panacea` (the large one), record in the results document: wall-clock of
  `git clone --local`, disk before and after (`Get-PSDrive C` free space, or `du -sh` on the clone
  **and** the source), whether the state directory and the projects root are on the same volume
  (hard links need one), the longest path in the checkout plus the clone root's length against
  260, and whether `core.longpaths` is set. No antivirus configuration is changed.
- [ ] **Step 6:** write the results document: one row per test, the recorded sentences verbatim,
  the measurements, and a **Divergences** section naming each later task that must change.
- [ ] **Step 7:** commit `doc.go`, both test files and the document, by pathspec.

#### Acceptance

```
go test ./control-plane/internal/gitx/ -run Behaviour -count=1 -v   → PASS, 17 tests (or the recorded, inverted form)
go test ./control-plane/internal/gitx/ -tags docker -run Sandbox -count=1 -v → PASS, 3 tests, or SKIP naming the missing image
go vet ./control-plane/...                                           → no output
```

#### What this task does not claim

- It verifies git on **this** laptop and in **this** image. A different git on another machine is
  not covered; task 02's runner checks the host git version at startup instead.
- The clone measurements are one machine, one antivirus configuration, one afternoon.
- It does not verify Claude Code behaviour of any kind; task 17 owns the transcript canary.
