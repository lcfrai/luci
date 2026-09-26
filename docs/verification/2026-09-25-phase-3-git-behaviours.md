# Phase 3 git behaviours, verified — 2026-09-25

Plan task 01 (`docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-01.md`).
Every later phase 3 task's preflight reads this file. Each row below is a test that ran real git
on the owner's laptop, or in the sandbox image, on 2026-09-25. A row's sentence in quotes is
git's own output, copied from the `-v` log.

**Where it ran.** Host: Windows 11 Enterprise 10.0.26200, `git version 2.53.0.windows.2`,
`go1.26.2` (module `go 1.23`). Sandbox: `panacea-app/sandbox:dev`
(`sha256:469a07f6c43d…`, built 2026-09-23T06:52Z), whose git is **2.39.5**, with no `ssh`, no
`git-lfs` and no `/etc/gitconfig`. Between 19:17 and 19:21 (between two mutation runs) the same tag's `.Id` came to read
`sha256:5d9a580e035b…`, an OCI image index (`docker image inspect --format "{{json .Descriptor}}"`),
with the same `Created` to the nanosecond (`2026-09-23T06:52:05.425529555Z`). The mutation runs
after that time used it. That it is the same build is inferred from `Created`, not proven. The host's system file,
`C:/Program Files/Git/etc/gitconfig`, holds nine keys (`git config --system --list
--show-origin`, 2026-09-26): `diff.astextplain.textconv=astextplain`, `http.sslbackend=schannel`,
`core.autocrlf=true`, `core.fscache=true`, `core.symlinks=true`, `pull.rebase=false`,
`credential.helper=manager`, `credential.https://dev.azure.com.usehttppath=true` and
`init.defaultbranch=master`. No pin reads this file: the production-shaped pins write a system
file of their own.

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
| 4 | `TestBehaviourSystemAutocrlfReachesAClone` | Production-shaped. A plain `clone --no-hardlinks` writes `a.txt` as `x\r\ny\r\n`, and `config --local --get core.autocrlf` exits 1. Task 03's four keys write `a.txt` as `x\ny\n` and `b.bat` (`eol=crlf`) as CRLF, all four keys land in `config --local`, and `status --porcelain` is empty. **Not pinned:** what `core.checkStat=minimal` and `core.trustctime=false` do. Dropping both leaves every pin green (divergence 15). | 03 |
| 5 | `TestBehaviourCloneSymlinksFollowTheSystemConfig` | This user **can** create symlinks (Developer Mode or the privilege). System `core.symlinks=true`: no local `core.symlinks`, and `link` is a real symlink. System file without it: the clone writes `core.symlinks=false` locally and `link` is a file holding `t.txt`. `--config core.symlinks=false`: a file holding `t.txt` under both. | 03 |
| 6 | `TestBehaviourEmptyFilterEnvChecksOutTheBlob` | Production-shaped with a system `filter.lfs` (smudge writes a marker, `clean = cat`, `required = true`). A plain clone writes `SMUDGED` and the marker. With task 03's NoFilterEnv, `a.bin` equals its blob, there is no marker, and a later production-shaped `status` exits 0. **Added after the mutation check** (M6 survived, see below): the same with git-lfs's real shape, a system `filter.lfs.process` as well. A plain clone runs the process driver and exits 128 ("fatal: the remote end hung up unexpectedly", then "warning: Clone succeeded, but checkout failed."). NoFilterEnv: exit 0, the blob, no driver run. `process=` and `required=false` alone: the same. `smudge=` and `process=` without `required=false`: exit 128, "fatal: a.bin: smudge filter lfs failed". | 03 |
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
| 20 | `TestBehaviourGitVarInACloneMissesTheSourcesLocalIdentity` | In the source, `git var GIT_AUTHOR_IDENT` prints its local identity; in the clone it does not. **Finding:** with no identity anywhere, the clone's `git var` exited **0** and printed `<display name> <account UPN> …`, which is Git for Windows' fallback: the Windows display name and the account's UPN (the real values are in the `-v` log, not repeated here). No config supplied it. | 03 (see Divergences) |
| 21 | `TestBehaviourNoWriteFetchHeadLeavesFetchHeadAlone` | `fetch --no-auto-maintenance --no-write-fetch-head <bundle> +refs/heads/b:refs/panacea/lanes/w` (the bundle from `bundle create` in a scratch clone): no `FETCH_HEAD` in a fresh clone, and `FETCH_HEAD`'s bytes and mtime unchanged after a `fetch origin` wrote one. The lane ref lands. | 07, 08, 12 |
| 22 | `TestBehaviourCheckRefFormatBranchExpandsShorthand` | `--branch @{-1}`: exit 0, `feature\n`. `refs/heads/@{-1}`: exit 1, no stderr. `update-ref refs/heads/@{-1} HEAD 0000…`: exit 128, "fatal: update_ref failed for ref 'refs/heads/@{-1}': refusing to update ref with bad name 'refs/heads/@{-1}'". `--branch -x` and `--branch HEAD`: exit 128, "fatal: '-x' is not a valid branch name" and "fatal: 'HEAD' is not a valid branch name". `refs/heads/-x`, `refs/heads/HEAD`: exit 0. `refs/heads/a..b`: exit 1, empty stderr. | 11 |
| 23 | `TestBehaviourRemoveSectionExitCodes` | `--remove-section branch.nope`: exit 128, "fatal: no such section: branch.nope". A present section: exit 0. With `.git/config.lock` held: exit **255**, "error: could not lock config file .git/config". `branch -D x` then exits 0, stdout "Deleted branch x (was 9cf81c2).", stderr **two** lines: "error: could not lock config file .git/config" and "warning: update of config-file failed"; `[branch "x"]` stays in `.git/config` and `refs/heads/x` is gone. | 11 |
| 24 | `TestBehaviourBranchRenameMovesACheckoutsHead` | `branch -m main renamed` with `main` checked out: exit 0, no output; `symbolic-ref HEAD` prints `refs/heads/renamed`. | 11 |
| 25 | `TestBehaviourFetchStartsAutoMaintenance` | Under `GIT_TRACE=1`, `fetch --no-tags --quiet <src> +refs/heads/main:refs/x` logs `run_command: git maintenance run --auto --quiet --detach`. With `--no-auto-maintenance` and `maintenance.auto=true` in `.git/config`: no `maintenance run` line (the trace does echo the argv's `--no-auto-maintenance`). | 07, 09, 11, 12 |
| 26 | `TestBehaviourRemoveAllOnAClone` | Git wrote the clone's pack `-rw-rw-rw-` (not read-only). The pin then sets it `0o444` (FILE_ATTRIBUTE_READONLY) and checks that took. `os.RemoveAll(clone)` on `go1.26.2` succeeded **first time**, read-only pack and all; the chmod walk was not needed. **Changed after the mutation check** (M26 survived, see below): the pin now asserts that first call, so it goes red on a toolchain where it fails; the walk runs only on that red, to say whether the plan's fix repairs it. | 03, 11 |

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
| `TestSandboxCloneHasWorkingGit` | Control 1 (C2's mechanism: plain `clone --local`, session without the two line-ending pairs): `status --porcelain` exit 0 listing **4** entries, ` M .gitattributes`, ` M a.txt`, ` M sub/b.txt`, ` M t.txt`. **C2 reproduces.** (`a.bin` is `-text`; the host's real symlink was not listed.) Control 2 (the I-1 mutant, `true`/`crlf` in the argv and the session): status empty, `a.txt` on disk `x\r\ny\r\n`. Task 03's clone with `false`/`lf`: status prints nothing; `rm a.txt && git checkout -- a.txt` in the sandbox writes no `\r`; the sandbox sees `link` as a regular file holding `t.txt` (added after the mutation check: D6 survived, see below); `commit --allow-empty` exits 0 with author and committer the injected identity. | 03, 04, 05 |
| `TestSandboxCloneCannotFetch` | Control, origin left at the Windows path clone wrote (`C:\Users\AU-DEV~1\…\src`): `git fetch` exit 128, "error: cannot run ssh: No such file or directory" / "fatal: unable to fork". With step 4a's URLs: `git fetch` exit 128 and `git pull` exit **1**, both "fatal: '/panacea-app/no-remote/the-cockpit-fetches-origin-and-upstream-for-this-worktree' does not appear to be a git repository" / "fatal: Could not read from remote repository." / "Please make sure you have the correct access rights" / "and the repository exists."; `git push origin HEAD` exit 128, the same four lines naming `/panacea-app/no-remote/the-cockpit-lands-this-worktree-and-pushes-the-branch`. | 03, 04; the sentences wait here for a later agent-docs task |
| `TestSandboxGitHonoursEnvHooksPath` | `core.hooksPath=/opt/panacea/githooks` as `GIT_CONFIG_KEY_8`, the directory bind-mounted read-only (it lists `-rwxrwxrwx 1 root root 58 … prepare-commit-msg`): the commit carries `Panacea-Session: s-sandbox`. `git --version` in the image: `git version 2.39.5`. | 05 |

## R12: clones on this laptop (Step 5)

2026-09-25, 19:07 to 19:12 +1000, `git version 2.53.0.windows.2`, antivirus as configured (not
changed). Each clone ran task 03's six `--config` keys and NoFilterEnv, with the owner's own
global config, from the source's Windows path into
`%TEMP%\claude\…\scratchpad\p3\r12\` (the same volume, `C:`, as `~/Projects` and
`%LOCALAPPDATA%\PanaceaApp\state\worktrees`, so hard links were possible). Sizes come from a
directory walk that opens no file (`scratchpad/p3/r12size`). A first round used `du -sk` and
was stopped: `du` over the fresh `--no-hardlinks` clone's 20 MB of objects took 6.5 minutes,
and so did the next `du`, because every newly written file is scanned when it is first opened.
The raw logs are `scratchpad/p3/r12/r12-round2.log` and `r12.log`.

| Source | Objects in the source | Mode | Exit | Wall-clock | Clone on disk (objects) | Packs that are the source's file | First `status` |
|---|---|---|---|---|---|---|---|
| PanaceaApp (419 files, `0881a85`) | 3,520 loose (7.0 MiB) + 2 packs (9.5 MiB) | `--local` | 0 | **26.2 s** | 24.0 MiB (16.6 MiB) | 2 of 2 | empty, 4.1 s |
| PanaceaApp | same | `--no-hardlinks` | 0 | **41.8 s** | 24.1 MiB (16.7 MiB) | 0 of 2 | empty, 3.4 s |
| Panacea (3,586 files, `219531165`) | 1,128 loose (39 MiB) + 11 packs (163 MiB) | `--local` | 0 | **32.5 s** | 267.6 MiB (202.4 MiB) | 11 of 11 | empty, 1.8 s |
| Panacea | same | `--no-hardlinks` | 0 | **32.9 s** | 267.7 MiB (202.4 MiB) | 0 of 11 | empty, 2.2 s |

- **Q9's answer.** `--no-hardlinks` on `~/Projects/Panacea` cost **32.9 s against 32.5 s** for
  `--local`: the copy adds no measurable time, because the time goes on files, not bytes, and
  Panacea's objects are 11 large packs. It adds about **202 MiB of disk per lane** (the objects),
  where `--local` adds only the checkout. On PanaceaApp, whose objects are mostly loose, the copy
  cost 15.6 s more (41.8 s against 26.2 s): 3,520 small files each copied and scanned. A first
  round measured 22.7 s and 34.5 s for the same pair. The recommendation stands: keep the copy.
- **Disk, by free space.** `C:` free space moved by +5 MiB, +81 MiB, −31 MiB and +345 MiB across
  the four clones: other agents write to `C:` at the same time, so the delta is noise and the
  walk's sizes above are the measure. Free space was about 48 GiB throughout.
- **Path length.** The longest tracked path is 91 characters in PanaceaApp and 119 in Panacea.
  Under this scratch root (158 to 168 characters) the longest checkout path was 260 and 285, and
  both clones exited 0 only because `core.longpaths=true` was set. Under the production root
  (task 03: 77 characters to the project id's folder, plus up to 48 for the name and nonce) the
  longest Panacea path is 125 + 1 + 119 = **245**, inside 260 but only by 15; a repository with a
  deeper tree passes it, which is what `core.longpaths=true` is for.
- **Symlinks.** Neither repository tracks a symlink (0 entries of mode `120000`), so neither
  checkout holds one. Rows 5 and `TestSandboxCloneHasWorkingGit` cover the shape: a regular file
  under task 03's `core.symlinks=false`.
- **Antivirus.** Not changed. Its cost shows as per-file time: a git process costs 2 to 3 s, and
  a first open of a new file is slow enough that `du` over 3,574 files took 6.5 minutes.

## Divergences: what a later task must change or know

No pin was inverted: all 29 held as the part states them. These are the facts the pins found
beyond that, each with the task it touches. Divergence 14 is about running the pins, not
about git's answers. Divergence 15 comes from a probe, not a pin. Only task 02's part reads
this section in its preflight. So each divergence that changes another task is also carried
to that task in the stream's `carried/` files (local and git-ignored, `.superpowers/sdd/p3/carried/`;
fix round 1). D5, D7 and D10 are not carried, and `carried/INDEX.md` gives the reasons. It also
gives them for each task a carried divergence names but is not carried to (D1 for tasks 08
and 10, D8 for task 09, D15 for task 07).

1. **Host git invents an identity on Windows (tasks 02, 03; 08, 09, 10).** With no
   `user.name`/`user.email` in any config and no `GIT_AUTHOR_*`/`EMAIL`, Git for Windows
   2.53 does not fail: `git var GIT_AUTHOR_IDENT` exits 0 with
   `<display name> <account UPN> <time> +1000` (the Windows display name and the account's
   UPN), and `commit` exits 0 as that identity. With `-c user.useConfigOnly=true`,
   `var` exits 128: "fatal: no email was given and auto-detection is disabled". (Row 20, and a
   probe on 2026-09-25, `scratchpad/p3/t01-identprobe.out`.) So task 03 step 5
   (`var GIT_AUTHOR_IDENT` in the project, copied into the clone when ParseIdent accepts it)
   never sees "no identity" on this laptop, and copies an invented one into the lane's config.
   Its `TestCreateWithoutAnIdentityStillCreates` passes only because its fixture sets
   `user.useConfigOnly`. **Change:** task 03 step 5 runs `var` with
   `-c user.useConfigOnly=true`, or task 02's hardening sets `user.useConfigOnly=true` for
   every host call, so a host `commit-tree` (tasks 08, 09, 10) cannot author as an invented
   identity either.
   **Phase 2's resolver, checked 2026-09-26 (task 01 fix round 1).** At `31576b7` and at the
   phase-2 tip `87551a9`, `control-plane/cmd/api/gitidentity.go:18-27` and `:38` read
   `git var GIT_AUTHOR_IDENT` in hostGit's own directory, never a worktree (X-1). They do not
   set `user.useConfigOnly`, and on purpose: the comment says the derived identity is the one
   the owner's own commits carry. `api.Deps.GitIdentity` takes no path
   (`internal/api/api.go:149`). So the resolver has this shape by design, and every session is
   sent that identity as `GIT_AUTHOR_*`. The change above would make a lane's local identity
   disagree with it. Which rule decides who "the user" is, the same across phases, is the
   orchestrator's call. The recommended rule is phase 2's. Then task 03's step 5 keeps the
   project's `git var` answer, or is dropped as redundant: the resolver no longer reads the
   clone, so task-03:14-16 is stale. Carried to task 03 with the correction.
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
6. **`os.RemoveAll` needs no read-only fix on `go1.26.2` (tasks 03, 11).** Git for Windows wrote
   the pack `-rw-rw-rw-`, and even a pack forced read-only was removed first time. The reason is
   Go's, not git's. On go1.26.2 Windows builds `GOROOT/src/os/removeall_at.go`
   (`//go:build unix || wasip1 || windows`). `os.Remove` runs only on the top-level path (`:29`).
   Each child is deleted by `removeAllFrom` (`:56`), through `removefileat` (`:68`) or
   `removedirat` (`:152`), which call `windows.Deleteat` (`os/root_windows.go:346-351`). That
   passes `FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE`
   (`internal/syscall/windows/at_windows.go:290-303`), and its fallback, `deleteatFallback`,
   clears the attribute (`:327-344`). *Corrected in fix round 1:* this entry first cited
   `os/file_windows.go:247-249`, which is `os.Remove`'s own clear-and-retry, the top-level path
   only. On go1.23.12 (the module cache's toolchain) Windows builds `removeall_noat.go`
   (`//go:build !unix`). It deletes each child with `os.Remove` (`:30`, `:108`, `:127`, `:134`),
   whose clear-and-retry is at `os/file_windows.go:205-206` there. So go1.23 has the same
   outcome by another path (read in source; see the run note below). The chmod walk the part
   names is a fallback, not a requirement, on these toolchains. Task 03's `RemoveTree` comment
   ("os.RemoveAll fails on them") is wrong here, and its `TestRemoveTreeDeletesReadOnlyObjects`
   cannot go red against a plain `os.RemoveAll`. Task 03 either drops the walk and leans on
   this pin, or keeps it and states the red it shows (a Go change, not git). Go older than 1.23
   is not covered; the module's floor is `go 1.23`.
7. **merge-tree `-z` messages end in a newline before their NUL (tasks 07, 08).** Each
   informational record is `<count>\0<path>…\0<type>\0<text>\n\0`. A parser that takes the
   paths up to the empty field (row 10) never reads them; one that shows messages trims the
   `\n`.
8. **The trailer read-back has two trailing newlines (tasks 09, 10; 05, 08).** Row 13: trim, do not
   compare the raw stdout. Task 02's `TrailerFormat` (`separator=%x1f`) prints the value with
   no newline of its own. Two parts read `valueonly` without it (fix round 1 probe,
   `scratchpad/p3/t01f1-trailer-probe.out`):
   - task-08:62's `for-each-ref` format printed `<sha> w-c\n\n<sha> w-a\n\n`, a blank line after
     every record. That is b06-2 issue 2, already carried to task 08.
   - task-05:222's `log -1` printed `w-a\n\n`.
9. **Docker Desktop bind mounts show every file as `-rwxrwxrwx root` (task 05).** The env
   hooks directory's `prepare-commit-msg` ran without the test setting a mode, because this
   mount makes every file executable. A Linux host or a Kata sandboxd will not, so task 05's
   dispatcher must carry its own executable bit.
10. **C2 in the sandbox covers text files only.** Control 1 listed the four text files; the
    LFS pointer (`-text`) and the host's real symlink were not listed. The symlink shape is
    task 03's `core.symlinks=false` either way.
11. **NoFilterEnv's empty `smudge` is redundant (task 03, review Minor 13).** Git skips
    `smudge` and `clean` whenever `filter.<d>.process` is set, and an empty `process` runs
    nothing, so `filter.lfs.process=` alone turns every LFS driver off for the call. The
    load-bearing pair is `process=` and `required=false`: without `required=false` a
    `required = true` driver that runs nothing is a failure, and the checkout dies ("fatal:
    a.bin: smudge filter lfs failed"). Keep all three keys (the empty smudge costs nothing);
    task 03's test should not claim the smudge key is what does it. The part's row 6 fixture
    (no `process`) did not model git-lfs's real config; the pin now does.
12. **The sandbox's empty status does not tell a symlink from a file (task 03, review
    Minor 15).** With `core.symlinks=true` in the argv and a user who may make symlinks, the
    host writes a real symlink, Docker Desktop shows it to the sandbox as a Linux symlink, and
    status is still empty. The part said the empty status covers Minor 15 in the real sandbox;
    it does not. The pin now asks the shape in the sandbox. The reason task 03 writes the key is
    that the answer otherwise depends on the machine (row 5), not that the sandbox breaks.
13. **A plain clone's CRLF checkout stays dirty with the pairs too (task 05).** Control 1 listed
    the same four files with the two line-ending pairs restored (mutant D4). The session pairs
    cannot repair a checkout that task 03's argv did not make LF; they only keep the sandbox's
    reading of an LF checkout consistent. In this image they change nothing at all: `false` and
    `lf` are what Linux git uses when no config sets them, and the image has no
    `/etc/gitconfig`. They guard against an image or a mounted config that does set them, which
    no pin here models.
14. **Under load, `git.exe` can fail to start, and the exit code is then Windows' (task 02).**
    An acceptance run at 21:11 to 21:17 on 2026-09-25, on the pins as committed at `2bb826e`,
    failed 12 of 26 (`.superpowers/sdd/p3/t01-acc-behaviour.out`, git-ignored). The load at
    21:17 was not recorded; at 21:52 the machine held 13 other agents' `go.exe` and about 600
    processes. Single pins took up to 106 s in that run. Each of the 12 had one git call read `exit 3221225794`
    with empty stdout and stderr: that is `0xC0000142`, `STATUS_DLL_INIT_FAILED`, the Windows
    loader failing before git's `main` ran, not an answer from git. One of them, row 25, first
    failed inside a git that had started: `fetch` exited 1 with "error: <src> did not send all
    necessary objects", a child of it presumably having failed the same way (inferred; the trace
    does not say). The same pins passed in full at 21:29 (the race run) and
    at 21:48 to 21:59 (26 of 26, 244.7 s). The sandbox run at 22:23 failed as a whole the
    same way: the test binary itself never started (`exit status 0xc0000142`, no test ran).
    At `e7d2281` the reruns passed one `go test` at a time: sandbox 3 of 3 at 22:37 to 22:44
    (177.6 s), behaviour 26 of 26 at 22:47 to 22:51 (156.0 s), and the `-race` suite over
    `./sandboxd/... ./control-plane/...` at 23:08 to 23:26 (gitx 346.8 s). **Change:** task 02's runner must not read
    3221225794 as git's exit code. An `OKExit` list, or a caller's "128 means no such ref",
    would otherwise file a git that never ran as an answer. Its `*Error` should name it
    ("git could not start"). Retrying that one code is safe, because no git code ran; a git
    that started and then failed, like the `fetch` above, cannot be told from a real failure
    and is not retried. For these pins: an `exit 3221225794` in this package's log is the
    machine, not a finding. Rerun with fewer processes live. *Since fix round 1 (2026-09-26)*
    the pin helper says so itself. Every git call, and the sandbox half's `docker run` and
    `docker image inspect`, goes through `runProgram`. On that code, `runProgram` stops the pin
    with a sentence that starts with the program's own name, for git "git could not start
    (exit 3221225794 = 0xC0000142, STATUS_DLL_INIT_FAILED): the machine's load, not a finding
    about git". `TestPinHelperNamesAProgramThatNeverStarted` pins this with a
    `cmd /c exit -1073741502` fixture. That test went red before the check existed. It also
    checks that other codes pass through untouched: 0, 1, 128 and 255, and three NTSTATUS codes
    (fix round 2). Those are `0xC0000005` and `0xC0000409`, a program that started and crashed,
    one either side of `0xC0000142`, and `0xC0000135`, the loader's "DLL not found". And it
    checks that the helper fails the pin rather than skipping it. The helper does not retry.
    `TestPinHelpersStartEveryProgramThroughRunProgram` reads this package's test sources and
    fails on any other `exec.Command` site, in either build tag: before the fix, `needSandbox`'s
    own inspect would have read a docker.exe that never started as "the image is absent" and
    skipped the whole sandbox half.
    A test binary that itself fails to start ("exit status 0xc0000142" from `go test`, with no
    test run) is outside any helper's reach.
15. **No pin shows what `core.checkStat=minimal` and `core.trustctime=false` do. Probes show
    that without them every poll re-reads the whole tree until the sandbox runs a lock-taking
    git, so task-03:313-314 holds (task 03; task 07 knows).** Added in fix round 1 (review M-2,
    2026-09-26). The conclusion was corrected in fix round 2 (re-review 1 I-A): round 1's probe
    ran a plain `status`, not task 07's poll.
    - The reviewer's mutant S1 dropped both keys from `task03Argv`, and all 3 sandbox pins stayed
      green (`scratchpad/p3/t01rv-S1.out`). A pin sees only what `status` prints, and a re-hash
      prints nothing.
    - **Probe A, fix round 1** (`scratchpad/p3/t01f1-stat-probe.sh`, output `.out`, host git
      2.53.0.windows.2, image `5d9a580e…`). The host is production-shaped. Two lanes were cloned
      from one 41-file source: "with", by task 03's step-4 argv, and "without", the same minus
      the two keys. Each lane got three sandbox `git status --porcelain` runs, each in a new
      container with sessionEnv's eight pairs. The host hashed `.git/index` after each run and ran
      no git in either lane.
      - Every status printed nothing and exited 0.
      - Host git wrote `dev: 0 ino: 0` and a Windows ctime for every entry. The sandbox's `stat`
        of `f1.txt` saw `dev 67` and a 64-bit inode, the same in all three containers.
      - "with": the index was never rewritten (one hash from clone to run 3).
      - "without": run 1 rewrote the index (`636b74d1…` → `38658cd5…`), and its entries took
        the sandbox's `dev: 67`, a 32-bit inode and the new ctime. Runs 2 and 3 left it alone.
    - **What probe A did not run: task 07's poll.** A plain `status` takes the optional index
      lock and writes the refreshed stat data back. That write is the only reason its runs 2 and
      3 were cheap. Task 07's poll is task 24's runner op
      `G status --porcelain=v2 -z --untracked-files=all` under `GIT_OPTIONAL_LOCKS=0`
      (task-24:95, :212; plan `:103`, "which writes nothing"). Row 16 pins that no-write rule on
      the host. The re-reviewer's probe (`scratchpad/p3/t01rr1-optlock-probe.out`) ran the poll
      three times without the keys: host git's `dev: 0 ino: 0` stayed in the index each time,
      and a plain `status` afterwards still found them stale and rewrote it.
    - **Probe B, fix round 2, counts the re-reads** (`scratchpad/p3/t01f2-count-probe.sh`, output
      `.out`, rc=0, same host git and image). Every tracked path has `filter=count`. Only the
      sandbox defines that driver (a ninth session pair, `filter.count.clean`), and its command
      logs the path and passes the bytes through. Git runs a clean filter exactly when it hashes a
      working-tree file, so the log's length is the number of files one git call re-read. The
      source holds 42 tracked files: an 8 MiB `big.bin`, 40 small files and `.gitattributes`.
      The lanes use the same two argvs as probe A, and each run is a new `--rm` container.
      - "without": polls 1 to 3 each re-read **all 42** files. The index stayed `2a380d1d…`,
        still holding `dev: 0 ino: 0`. The plain `status` of run 4 re-read 42 and rewrote the
        index (`a29ee1b2…`, `dev: 67`, a ctime equal to the mtime). The two polls after it
        re-read 0.
      - "with": polls 7 to 9 each re-read **16** files, never `big.bin`. The plain `status` of
        run 10 re-read 16 and rewrote the index with the same entries (hash `23820900…`
        unchanged, new file mtime). The poll after it re-read 0.
      - The 16 are exactly the entries whose mtime falls in the same whole second as the index
        file's (11:24:05; `scratchpad/p3/t01f2-with-mtimes.out`). That is git's racy-clean
        rule: an entry the index cannot prove older than itself is re-checked by content. This
        sandbox git applies it to whole seconds. That is inferred from which 16 were re-read:
        `f31.txt`, at `:05.027`, is older than the index's `:05.285` and was still re-read.
      - This round's first probe (`scratchpad/p3/t01f2-reread-probe.out`) timed the same shape
        with a 32 MiB file. A cold mount cache made the "with" lane's first poll slow too
        (22.0 s), so timing could not tell re-reads from cache misses. Probe B counts instead.
    - So:
      - task-03:309 ("the stat data Windows wrote never matches through the bind mount") holds.
      - task-03:313-314 ("a full re-hash on every `status` (task 07's poll)") **holds**. Without
        the keys, every poll re-reads the whole tree. That lasts until a lock-taking git (the
        agent's own `status`, `add` or `commit`) stores the sandbox's stat data. Fix round 1's
        "one re-hash" was true only of a lock-taking status. Keep the keys, for the reason the
        plan gives.
      - New, and true with the keys too: each poll re-reads the clone's racily clean entries
        until a lock-taking git runs. Those are the files checkout wrote in the index's own last
        second, so the cost is bounded by one second of checkout, not by the tree. Carried to task
        03 as a "know" in t01-D15.
    - Unmeasured: a Kata or virtiofs mount, whose inode or uid numbering may not be stable; a
      large repository's time; and whether Claude Code runs a lock-taking git soon after a
      session starts. Carried to task 03 as t01-D15.

## Mutation check

Run on copies under `scratchpad/p3/t01-mut-*`, built from `git archive HEAD` (plus the
uncommitted test file where a round needed it). `scratchpad/p3/t01-mutants.py` applies each
mutant as an exact, once-only replacement and stops if an anchor does not match exactly once,
so no mutant that failed to apply can pass as a survivor. Each mutant makes one plan assumption
false, or removes one thing task 03 or 05 does.

**Round 1** (copies from `4612f00`). M1 to M26, one per behaviour pin, in one copy (each edits
only its own pin's body); D1 to D3 in a second copy (each a different sandbox pin); D4, D5, D6
alone. Named before running as the two most likely survivors: M26 and D4.

| Result | Mutants |
|---|---|
| Killed, 24 of 26 | M1 to M5, M7 to M25: all 24 pins failed, each on its own mutant's change (`scratchpad/p3/t01-mut-b.reasons`) |
| Killed | D1 (`true`/`crlf` in argv and session: "the sandbox's checkout of a.txt wrote "x\r\ny\r\n"; want no \r"), D2 (no NoFetchURL: fetch and pull named ssh, not the no-remote path), D3 (no env `core.hooksPath`: no hook line), D5 (no NoFilterEnv on the host: status ` M a.bin`) |
| **Survived: M26** | the walk's chmod mode `0o600` → `0o400`. Predicted. The first `os.RemoveAll` succeeds, so the walk never ran. Fixed in `2bb826e`: the pin asserts the first call (row 26) |
| **Survived: M6** | NoFilterEnv's `smudge` key renamed. Not predicted. The fixture had no `process` driver. Fixed in `abb8842`: the git-lfs-shaped half (row 6, divergence 11) |
| **Survived: D4** | control 1 keeps the two line-ending pairs. Predicted, and equivalent: the pairs are the image's defaults (divergence 13). Control 1 is record-only by the part |
| **Survived: D6** | `core.symlinks=true` in the sandbox pin's argv. Not predicted. Fixed in `9674b53`: the pin asks the link's shape in the sandbox (divergence 12) |

**Round 2** (after `abb8842`). M6r2, the smudge key renamed again on the new two-half pin:
**survived**, and is equivalent: the pin itself records that `process=` and `required=false`
alone turn the driver off (divergence 11). M6b, M6c and D6r2 were started four `go test` runs at
once; three test binaries died at start with `exit status 0xc0000142` (Windows could not
initialise the process) before any test ran, so they were **not results** and were rerun in
round 3. Run `go test` processes one at a time on this laptop.

**Round 3** (copies from `9674b53` plus the new RemoveAll pin, one run at a time). Named
before running as the two most likely survivors: M26 on the new pin, and M26c on the old one.

| Mutant | Pin | Result |
|---|---|---|
| M26c: the first `os.RemoveAll(dst)` → `os.Remove(dst)` | row 26 **before** `2bb826e` | **survived**, as predicted: "The directory is not empty", then the walk and a second RemoveAll, then PASS. The old pin let a failing first call through |
| M26c | row 26 **after** `2bb826e` | killed: "os.RemoveAll on a clone with a read-only pack failed first time … tasks 03 and 11 need the read-only walk" |
| M26d: the fixture's `0o444` → `0o644` | row 26 after | killed: "fixture: the pack is not read-only" |
| M26: the walk's mode `0o600` → `0o400` | row 26 after | **survived**, as predicted, and equivalent here: the walk runs only after the pin has already gone red |
| M6b: NoFilterEnv's `process` key renamed | row 6 | killed: "NoFilterEnv: the system process driver ran", exit 128 |
| M6c: NoFilterEnv's `required` key renamed | row 6 | killed: the NoFilterEnv clone over the `required = true` smudge exits 128 |
| D6r2: `core.symlinks=true` in the argv | `TestSandboxCloneHasWorkingGit` after `9674b53` | killed: "the sandbox sees link as "symlink\n"; want file:t.txt, task 03's core.symlinks=false shape" |
| none (control) | rows 6 and 26 | both PASS on the unmutated copy |

Survivors against the pins as committed, all declared and all equivalent on this laptop and
image: D4, M6r2, and M26 against the new row 26. Round 1's M6, M26 and D6 each led to a stronger
pin, and the mutants run against those pins (M6b, M6c, M26c, M26d, D6r2) are killed. M26c's
survival on the old pin is the evidence that the old pin was weaker, not a survivor of this one.

**Round 4, fix round 1** (2026-09-26, the pin helper of divergence 14). Copies
`scratchpad/p3/t01f1-mut-*` from `25107e8` and `t01f1-mut3-*` from `2666901`, archived with
`core.autocrlf=false` so the anchors match; `scratchpad/p3/t01f1-mut.py` applies each mutant once
or refuses. Each copy ran `go test -run TestPinHelper` and `go vet -tags docker` (every vet
empty, so each mutant compiled in both build tags), one `go` process at a time. Named before
running as the two most likely survivors: MH6 (the check asked for no program name) and MH4
(only a source scan can see it).

| Mutant | Result |
|---|---|
| MH1: `runProgram` no longer calls `requireStarted` | killed: the helper said "", want "could not start" |
| MH2: the status compared is `0xC0000135` | killed: the helper said "" |
| MH3: every non-zero code is called "could not start" | killed: `cmd /c exit 1`, `128` and `255` each got the fatal |
| MH4: `inSandbox` starts docker with its own `exec.Command` | killed by the source scan: 2 sites, `sandbox_docker_test.go:135` |
| MH5: the sentence drops "not" before "a finding" | killed: want a sentence holding "not a finding" |
| MH6: the sentence names git whatever program it was | **survived** at `25107e8`, as predicted. Fixed in `2666901`: the pin asks the sentence to start "cmd could not start". Killed at `2666901` |
| MH7: `needSandbox` inspects with its own `exec.Command` | killed by the source scan: 2 sites, `sandbox_docker_test.go:87` |
| none (control), at both commits | both pins PASS |

No survivor remains against the helper pins as committed. The review's S1 (the stat keys
dropped) is answered by divergence 15's probe, not by a pin.
