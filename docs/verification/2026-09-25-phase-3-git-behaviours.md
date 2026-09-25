# Phase 3 git behaviours, verified — 2026-09-25

Plan task 01 (`docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-01.md`).
Every later phase 3 task's preflight reads this file. Each row below is a test that ran real git
on the owner's laptop, or in the sandbox image, on 2026-09-25. A row's sentence in quotes is
git's own output, copied from the `-v` log.

**Where it ran.** Host: Windows 11 Enterprise 10.0.26200, `git version 2.53.0.windows.2`,
`go1.26.2` (module `go 1.23`). Sandbox: `panacea-app/sandbox:dev`
(`sha256:469a07f6c43d…`, built 2026-09-23T06:52Z), whose git is **2.39.5**, with no `ssh`, no
`git-lfs` and no `/etc/gitconfig`. The host's system file,
`C:/Program Files/Git/etc/gitconfig`, holds `core.autocrlf=true`, `core.symlinks=true`,
`credential.helper=manager`, `diff.astextplain.textconv` and `init.defaultbranch=master`.

**How.** `control-plane/internal/gitx/behaviour_test.go` (26 pins) and
`sandbox_docker_test.go` (`//go:build docker`, 3 pins). Every host call runs under
`GIT_TERMINAL_PROMPT=0`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL=<tmp>/empty`, a temporary
`HOME`, no inherited `GIT_*`, `EMAIL` or `XDG_CONFIG_HOME`, and a fixed identity. The
production-shaped pins (review C2) replace `GIT_CONFIG_NOSYSTEM` with a system file of their
own that holds `core.autocrlf=true`. Exit codes come from `ProcessState.ExitCode()`, never
through a pipe.

```
go test ./control-plane/internal/gitx/ -run Behaviour -count=1 -v              → PASS, 26 tests
go test ./control-plane/internal/gitx/ -tags docker -run Sandbox -count=1 -v   → PASS, 3 tests
go vet ./control-plane/...                                                     → no output
```

**Cost.** One git process takes about 2 to 3 s of wall-clock on this laptop (almost all of it
outside git), so every pin calls `t.Parallel()`. The full behaviour run takes about 2.5 to
3 minutes and the sandbox run about 3 minutes.

## Host pins

| # | Test | What git did (2026-09-25) | Consumed by |
|---|---|---|---|
| 1 | `TestBehaviourLocalCloneHardLinksObjects` | `clone --local`: the loose object and the pack are the source's own files (`os.SameFile`). | 03 |
| 2 | `TestBehaviourNoHardlinksCloneSharesNoObjectFile` | `clone --no-hardlinks`: 7 object files compared with the source, none shared. | 03 |
| 3 | `TestBehaviourLocalCloneHasNoAlternates` | Neither clone writes `.git/objects/info/alternates`. | 03 |
| 4 | `TestBehaviourSystemAutocrlfReachesAClone` | Production-shaped. A plain `clone --no-hardlinks` writes `a.txt` as `x\r\ny\r\n`, and `config --local --get core.autocrlf` exits 1. Task 03's four keys write `a.txt` as `x\ny\n` and `b.bat` (`eol=crlf`) as CRLF, all four keys land in `config --local`, and `status --porcelain` is empty. | 03 |
| 5 | `TestBehaviourCloneSymlinksFollowTheSystemConfig` | This user **can** create symlinks (Developer Mode or the privilege). System `core.symlinks=true`: no local `core.symlinks`, and `link` is a real symlink. System file without it: the clone writes `core.symlinks=false` locally and `link` is a file holding `t.txt`. `--config core.symlinks=false`: a file holding `t.txt` under both. | 03 |
| 6 | `TestBehaviourEmptyFilterEnvChecksOutTheBlob` | Production-shaped with a system `filter.lfs` (smudge writes a marker, `clean = cat`, `required = true`). A plain clone writes `SMUDGED` and the marker. With task 03's NoFilterEnv, `a.bin` equals its blob, there is no marker, and a later production-shaped `status` exits 0. | 03 |
| 7 | `TestBehaviourLongPathNeedsCoreLongpaths` | A 354-character path (clone root 93). Without `core.longpaths`: exit 128, "fatal: cannot create directory at 'aaaa…/aaaa…/aaaa…': Filename too long", then "warning: Clone succeeded, but checkout failed." With `--config core.longpaths=true`: exit 0, empty status. | 03, 04 |
| 8 | `TestBehaviourCloneBranchSetsUpstream` | `clone --branch feat/x` checks out `feat/x`; `branch.feat/x.remote` is `origin`, `.merge` is `refs/heads/feat/x`. | 03, 09 |
| 9 | `TestBehaviourMergeTreeCleanExitsZeroWithATree` | Exit 0; stdout is exactly a 40-hex tree id and one NUL. | 07, 08 |
| 10 | `TestBehaviourMergeTreeConflictExitsOneWithPaths` | Exit **1**. `-z` stdout, verbatim: `<tree>\0a b.txt\0docs/résumé.md\0\0` then, per message, `1\0<path>\0<type>\0<text>\n\0` (types `Auto-merging` and `CONFLICT (contents)`). Paths are raw UTF-8, each once. Without `-z`: `a b.txt` bare, and `"docs/r\303\251sum\303\251.md"` quoted. | 07, 08, 09, 10, 16 |
| 11 | `TestBehaviourUpdateRefCompareAndSwapRefuses` | Exit 128, `b` unmoved: "fatal: update_ref failed for ref 'refs/heads/b': cannot lock ref 'refs/heads/b': is at 72e9981e… but expected 356eb9d9…" | 09, 10 |
| 12 | `TestBehaviourUpdateRefCreateOnlyRefusesAnExistingRef` | Exit 128: "fatal: update_ref failed for ref 'refs/heads/b': cannot lock ref 'refs/heads/b': reference already exists" | 11 |
| 13 | `TestBehaviourCommitTreeTrailersReadBack` | `log -1 --format=%(trailers:key=Panacea-Brief,valueonly)` prints `"brief:b-42@6\n\n"`: the value, the trailer's newline, then the format's terminator. A parser trims trailing newlines. | 09, 10 |
| 14 | `TestBehaviourMergeTreeWithMergeBaseRevertsAMerge` | `merge-tree --write-tree --name-only -z --merge-base=M T M^1` exits 0 with T's tree minus the merge's `f.txt`, every other entry byte-identical (`ls-tree -r`). | 10 |
| 15 | `TestBehaviourFixedDateCommitTreeIsDeterministic` | Two `commit-tree` calls, two parents, `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE=@0 +0000`: one id. | 08 |
| 16 | `TestBehaviourStatusWithoutOptionalLocksLeavesTheIndex` | After a tracked file's mtime moves, `status --porcelain=v2 -z` under `GIT_OPTIONAL_LOCKS=0` leaves `.git/index`'s bytes and mtime alone. Control added: a plain `status` then **does** rewrite the index. | 07 |
| 17 | `TestBehaviourLocalConfigHooksPathIgnoresEnvironment` | With `core.hooksPath=.husky/_` in `.git/config` and the env pair `core.hooksPath=/x`: `config --local --get` prints `.husky/_`. Control added: `config --get` prints `/x`, so the env wins for git's own use. | 05 |
| 18 | `TestBehaviourPrepareCommitMsgRunsUnderNoVerify` | `commit --no-verify`: `prepare-commit-msg` ran and its line is in the message, while `pre-commit` and `commit-msg` (both `exit 1`) were skipped. | 05 |
| 19 | `TestBehaviourRevListLeftRightCountFormat` | `rev-list --left-right --count A...B`, A one ahead and B two: `"1\t2\n"`. | 08 |
| 20 | `TestBehaviourGitVarInACloneMissesTheSourcesLocalIdentity` | In the source, `git var GIT_AUTHOR_IDENT` prints its local identity; in the clone it does not. **Finding:** with no identity anywhere, the clone's `git var` exited **0** and printed `Dylan <au-dev-dmackie1@kpmg.com.au> …`, which is Git for Windows' fallback: the Windows display name and the account's UPN. No config supplied it. | 03 (see Divergences) |
| 21 | `TestBehaviourNoWriteFetchHeadLeavesFetchHeadAlone` | `fetch --no-auto-maintenance --no-write-fetch-head <bundle> +refs/heads/b:refs/panacea/lanes/w` (the bundle from `bundle create` in a scratch clone): no `FETCH_HEAD` in a fresh clone, and `FETCH_HEAD`'s bytes and mtime unchanged after a `fetch origin` wrote one. The lane ref lands. | 07, 08, 12 |
| 22 | `TestBehaviourCheckRefFormatBranchExpandsShorthand` | `--branch @{-1}`: exit 0, `feature\n`. `refs/heads/@{-1}`: exit 1, no stderr. `update-ref refs/heads/@{-1} HEAD 0000…`: exit 128, "fatal: update_ref failed for ref 'refs/heads/@{-1}': refusing to update ref with bad name 'refs/heads/@{-1}'". `--branch -x` and `--branch HEAD`: exit 128, "fatal: '-x' is not a valid branch name" and "fatal: 'HEAD' is not a valid branch name". `refs/heads/-x`, `refs/heads/HEAD`: exit 0. `refs/heads/a..b`: exit 1, empty stderr. | 11 |
| 23 | `TestBehaviourRemoveSectionExitCodes` | `--remove-section branch.nope`: exit 128, "fatal: no such section: branch.nope". A present section: exit 0. With `.git/config.lock` held: exit **255**, "error: could not lock config file .git/config". `branch -D x` then exits 0, stdout "Deleted branch x (was 9cf81c2).", stderr **two** lines: "error: could not lock config file .git/config" and "warning: update of config-file failed"; `[branch "x"]` stays in `.git/config` and `refs/heads/x` is gone. | 11 |
| 24 | `TestBehaviourBranchRenameMovesACheckoutsHead` | `branch -m main renamed` with `main` checked out: exit 0, no output; `symbolic-ref HEAD` prints `refs/heads/renamed`. | 11 |
| 25 | `TestBehaviourFetchStartsAutoMaintenance` | Under `GIT_TRACE=1`, `fetch --no-tags --quiet <src> +refs/heads/main:refs/x` logs `run_command: git maintenance run --auto --quiet --detach`. With `--no-auto-maintenance` and `maintenance.auto=true` in `.git/config`: no `maintenance run` line (the trace does echo the argv's `--no-auto-maintenance`). | 07, 09, 11, 12 |
| 26 | `TestBehaviourRemoveAllOnAClone` | Git wrote the clone's pack `-rw-rw-rw-` (not read-only). The pin then sets it `0o444` (FILE_ATTRIBUTE_READONLY) and checks that took. `os.RemoveAll(clone)` on `go1.26.2` succeeded **first time**, read-only pack and all; the chmod walk was not needed. | 11 |

## Sandbox pins (`-tags docker`)

Each is one `docker run --rm` of `panacea-app/sandbox:dev`, container `pa-p3-t01-*` (no
`panacea-app.managed` label, so no control plane's reaper counts it), the clone bind-mounted at
`/home/dev/work`, `--entrypoint /bin/sh`. The environment is sessionEnv's eight pairs as
`feat/phase-2-cockpit` builds them (`control-plane/internal/api/sessions.go:426-442` at
`31576b7`, which contains `5a4d0d8`, re-read 2026-09-25): `safe.directory=/home/dev/work`,
`user.useConfigOnly=true`, `gc.worktreePruneExpire=never`, `gc.auto=0`, `core.autocrlf=false`,
`core.eol=lf`, `core.excludesFile=/dev/null`, `core.attributesFile=/dev/null`, then
`GIT_AUTHOR_*`/`GIT_COMMITTER_*`. The host side is production-shaped: system
`core.autocrlf=true`, `core.symlinks=true` and an LFS-shaped `filter.lfs`, cloning with task 03's
step-4 argv and NoFilterEnv. The fixture holds LF blobs, a symlink entry and an LFS pointer under
`*.bin filter=lfs`. No control plane was started, so the Docker lock was not taken.

| Test | What happened | Consumed by |
|---|---|---|
| `TestSandboxCloneHasWorkingGit` | Control 1 (C2's mechanism: plain `clone --local`, session without the two line-ending pairs): `status --porcelain` exit 0 listing **4** entries, ` M .gitattributes`, ` M a.txt`, ` M sub/b.txt`, ` M t.txt`. **C2 reproduces.** (`a.bin` is `-text`; the host's real symlink was not listed.) Control 2 (the I-1 mutant, `true`/`crlf` in the argv and the session): status empty, `a.txt` on disk `x\r\ny\r\n`. Task 03's clone with `false`/`lf`: status prints nothing; `rm a.txt && git checkout -- a.txt` in the sandbox writes no `\r`; `commit --allow-empty` exits 0 with author and committer the injected identity. | 03, 04, 05 |
| `TestSandboxCloneCannotFetch` | Control, origin left at the Windows path clone wrote (`C:\Users\AU-DEV~1\…\src`): `git fetch` exit 128, "error: cannot run ssh: No such file or directory" / "fatal: unable to fork". With step 4a's URLs: `git fetch` exit 128 and `git pull` exit **1**, both "fatal: '/panacea-app/no-remote/the-cockpit-fetches-origin-and-upstream-for-this-worktree' does not appear to be a git repository" / "fatal: Could not read from remote repository." / "Please make sure you have the correct access rights" / "and the repository exists."; `git push origin HEAD` exit 128, the same four lines naming `/panacea-app/no-remote/the-cockpit-lands-this-worktree-and-pushes-the-branch`. | 03, 04; the sentences wait here for a later agent-docs task |
| `TestSandboxGitHonoursEnvHooksPath` | `core.hooksPath=/opt/panacea/githooks` as `GIT_CONFIG_KEY_8`, the directory bind-mounted read-only (it lists `-rwxrwxrwx 1 root root 58 … prepare-commit-msg`): the commit carries `Panacea-Session: s-sandbox`. `git --version` in the image: `git version 2.39.5`. | 05 |

## Divergences: what a later task must change or know

No pin was inverted: all 29 held as the part states them. These are the facts the pins found
beyond that, each with the task it touches.

1. **Host git invents an identity on Windows (tasks 02, 03; 08, 09, 10).** With no
   `user.name`/`user.email` in any config and no `GIT_AUTHOR_*`/`EMAIL`, Git for Windows
   2.53 does not fail: `git var GIT_AUTHOR_IDENT` exits 0 with
   `Dylan <au-dev-dmackie1@kpmg.com.au> <time> +1000` (the Windows display name and the
   account's UPN), and `commit` exits 0 as that identity. With `-c user.useConfigOnly=true`,
   `var` exits 128: "fatal: no email was given and auto-detection is disabled". (Row 20, and a
   probe on 2026-09-25, `scratchpad/p3/t01-identprobe.out`.) So task 03 step 5
   (`var GIT_AUTHOR_IDENT` in the project, copied into the clone when ParseIdent accepts it)
   never sees "no identity" on this laptop, and copies an invented one into the lane's config.
   Its `TestCreateWithoutAnIdentityStillCreates` passes only because its fixture sets
   `user.useConfigOnly`. **Change:** task 03 step 5 runs `var` with
   `-c user.useConfigOnly=true`, or task 02's hardening sets `user.useConfigOnly=true` for
   every host call, so a host `commit-tree` (tasks 08, 09, 10) cannot author as an invented
   identity either. Phase 2's host identity resolver may have the same gap (not checked here).
2. **`branch -D` with the config locked prints two stderr lines (task 11).** "error: could not
   lock config file .git/config", then "warning: update of config-file failed", with exit 0.
   DeleteBranch's warn-and-answer rule must look for the warning line, not treat stderr as
   that one line.
3. **`git pull` exits 1, not 128 (task 03's origin, later agent docs).** `fetch` and `push`
   exit 128 with the NoFetchURL/NoPushURL sentence; `pull` prints the same sentence and exits 1.
4. **A Windows-path origin fails in the sandbox as "cannot run ssh" (task 03, review
   Minor 14).** Git does read `C:\…` as an SSH address, as Minor 14 predicted, but the image has
   no `ssh`, so the sentence is "error: cannot run ssh: No such file or directory" / "fatal:
   unable to fork". Step 4a's reason stands; the sentence would change if `ssh` joined the image.
5. **The sandbox's git is 2.39.5 (tasks 05, 24).** The host needs ≥ 2.40 for
   `merge-tree --merge-base` (row 14); the sandbox has neither the flag nor the need, since
   every merge-tree runs on the host. Anything task 24's runner runs in the sandbox is limited to
   2.39 (`GIT_CONFIG_COUNT` needs 2.31, which it has).
6. **`os.RemoveAll` needs no read-only fix on `go1.26.2` (task 11).** Git for Windows wrote the
   pack `-rw-rw-rw-`, and even a pack forced read-only was removed first time. The chmod walk
   the part names is a fallback, not a requirement, on this toolchain. An older Go is not
   covered.
7. **merge-tree `-z` messages end in a newline before their NUL (tasks 07, 08).** Each
   informational record is `<count>\0<path>…\0<type>\0<text>\n\0`. A parser that takes the
   paths up to the empty field (row 10) never reads them; one that shows messages trims the
   `\n`.
8. **The trailer read-back has two trailing newlines (tasks 09, 10).** Row 13: trim, do not
   compare the raw stdout.
9. **Docker Desktop bind mounts show every file as `-rwxrwxrwx root` (task 05).** The env
   hooks directory's `prepare-commit-msg` ran without the test setting a mode, because this
   mount makes every file executable. A Linux host or a Kata sandboxd will not, so task 05's
   dispatcher must carry its own executable bit.
10. **C2 in the sandbox covers text files only.** Control 1 listed the four text files; the
    LFS pointer (`-text`) and the host's real symlink were not listed. The symlink shape is
    task 03's `core.symlinks=false` either way.
