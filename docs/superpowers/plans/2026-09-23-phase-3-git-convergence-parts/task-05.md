### Task 05: Git in the sandbox for clones — the session trailer, `core.hooksPath`, and the hook dispatcher

Obligation 5 (survey §2 row 5; phase 7's `7-D3`): every agent commit carries the session that made
it, as `Panacea-Session: <id>`. The only mechanism that respects "never modify, wrap or parse the
CLI" is git's own hook, enabled by environment the control plane already owns (survey §5.2): the
agent runs `git commit`, git runs the hook, the CLI never knows. This task adds the environment,
the variable, and the hook.

**The hazard it must not ship (risk R7).** `core.hooksPath` from the environment outranks the
repository's own configuration, so a repository using husky (`core.hooksPath=.husky/_`) or plain
`.git/hooks` would silently lose **every** hook, not only `prepare-commit-msg`. So the image's
hooks directory holds one dispatcher, linked under every client-side hook name, and each name
runs the repository's own hook of that name first. It is tested in the image before it ships
against husky 9's shape (a stub per hook name in `.husky/_` that sources `h`, which runs the
tracked `.husky/<name>`; the fixture's `h` is a small stand-in, not husky's file, so the real
file's chain stays owner item 5's), on a clone made by task 03's `lanes.Creator` from a project whose
own config carries the setting (review I11: a clone configured by hand proves nothing, because
`git clone` copies neither `.git/config` nor `.git/hooks`).

**Files:**
- `images/sandbox/githooks/dispatch` — the dispatcher (new, LF line endings)
- `.gitattributes` — `images/sandbox/githooks/* text eol=lf` (new file; this checkout has
  `core.autocrlf=true`, and a CRLF `#!/bin/sh` does not run)
- `images/sandbox/Dockerfile` — install the dispatcher and its links (p2)
- `control-plane/internal/backend/backend.go` — `EnvSessionID`, written by `EnvEntries` (p2)
- `control-plane/internal/backend/backend_test.go` — the order pin gains one entry (p2)
- `control-plane/internal/sandbox/manager.go` — `containerEnv` writes the same variable (p2)
- `control-plane/internal/api/sessions.go` — `sessionEnv` gains `core.hooksPath` (the fix's function); `sessionMounts` binds `.git/config` read-only (task-02 §HGH H5)
- `control-plane/internal/api/sessions_sandboxenv_test.go` — the fix's test, extended (the fix's file)
- `control-plane/cmd/api/gitlineendings_test.go` — the fix's resolver test, extended with a lane clone (the fix's file, from `5a4d0d8`)
- `control-plane/internal/api/hooks_docker_test.go` — `//go:build docker`, the real thing (new)
- `docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md` — §2 `EnvSessionID`; §5.1 environment

#### Interfaces — complete

```go
// backend.go — one constant beside the three, and one entry in EnvEntries.
const (
	EnvCommand  = "PANACEA_APP_COMMAND"
	EnvArgsJSON = "PANACEA_APP_ARGS_JSON"
	EnvToken    = "PANACEA_APP_ATTACH_TOKEN"
	// EnvSessionID is the cockpit session this sandbox runs. The image's git hook stamps it
	// on every commit as "Panacea-Session: <id>" (phase 3 task 05). Written from
	// Spec.SessionID, never from Spec.Env, so neither a client nor an agent chooses it.
	EnvSessionID = "PANACEA_APP_SESSION_ID"
)

// Spec.SessionID's comment changes from "for the label only; a backend never reads it back"
// to: "for the label, and exported as EnvSessionID; a backend never reads it back".

// EnvEntries: the three contract variables, then EnvSessionID when Spec.SessionID is not
// empty, then Spec.Env sorted with reserved keys dropped. sandbox.containerEnv produces the
// identical sequence (TestThePodEnvironmentIsExactlySpecEnvEntries pins it).
```

```go
// sessions.go (post-fix: feat/phase-2-cockpit at or after 5a4d0d8) — at 10fa5fd the fix's
// sessionEnv is sessionEnv(id GitIdentity, eol GitLineEndings, git sessionGitFiles, caBundle
// string), and its git block is a `pairs` slice numbered into GIT_CONFIG_KEY_n/VALUE_n, with
// GIT_CONFIG_COUNT = len(pairs). It held five pairs at 5a4d0d8 and six at 57c3bf5; at 10fa5fd
// (re-read 2026-09-24, review I12) it holds eight (safe.directory, user.useConfigOnly,
// gc.worktreePruneExpire, gc.auto, core.autocrlf, core.eol, core.excludesFile,
// core.attributesFile), and the fix is still moving. Phase 3 APPENDS one pair after the last of
// them and never renumbers or retypes the fix's own.
const sessionHooksPath = "/opt/panacea/githooks" // images/sandbox/Dockerfile installs it

	pairs := [][2]string{
		// … the fix's pairs, unchanged, which at 10fa5fd end {"core.excludesFile", …},
		// {"core.attributesFile", …} …
		// Phase 3: the session trailer. The dispatcher there runs the repository's own hook of
		// every name first, so a husky repository keeps its hooks (risk R7).
		{"core.hooksPath", sessionHooksPath},
	}
```

The fix's `core.autocrlf` and `core.eol` pairs are what a lane session's git reads for line
endings, over the clone's local keys (task 03 "How the sandbox's git gets the same answer").
Their values are right for a lane only because the handlers ask `Deps.GitLineEndings` in the
session's worktree path, which for a lane is the clone. This task pins that; it changes no
handler code for it.

```go
// sessions.go — sessionMounts gains one entry (task-02 §HGH H5; review C1). When the worktree's
// host path holds a .git DIRECTORY (every clone, and the project on a primary-worktree
// session), its config is bound read-only over the worktree mount, so the config the control
// plane checks is the config its git reads. A linked worktree's .git is a file: no entry.
	if fi, err := os.Stat(filepath.Join(worktreePath, ".git")); err == nil && fi.IsDir() {
		mounts = append(mounts, backend.Mount{
			HostPath:      filepath.Join(worktreePath, ".git", "config"),
			ContainerPath: sessionWorkDir + "/.git/config",
			ReadOnly:      true,
		})
	}
```

```sh
#!/bin/sh
# images/sandbox/githooks/dispatch — installed as /opt/panacea/githooks/dispatch, and every
# client-side hook name in that directory is a symlink to it. The control plane points git
# here with core.hooksPath in the sandbox's environment, which overrides the repository's own
# core.hooksPath and .git/hooks. So this script:
#   1. runs the repository's own hook of the same name, if it has one, with the same
#      arguments and stdin, and stops with its status if that fails;
#   2. for prepare-commit-msg only, then adds "Panacea-Session: <id>" to the message, unless
#      the message already carries a Panacea-Session trailer (an amend keeps its author's).
# It is advisory (survey §5.3): an agent can unset the variable or bypass hooks, and a
# commit without the trailer renders "no session recorded".
set -u
name=$(basename "$0")
self_dir=/opt/panacea/githooks

# The repository's own hooks: its LOCAL core.hooksPath when set (husky sets .husky/_), else
# <git-common-dir>/hooks. `git config --local` reads .git/config only, so it never returns
# the environment's value, which is this directory (task 01 pins that).
repo_hooks=$(git config --local --get core.hooksPath 2>/dev/null || true)
if [ -n "$repo_hooks" ]; then
	case "$repo_hooks" in
		/*) ;;
		*) repo_hooks="$(git rev-parse --show-toplevel)/$repo_hooks" ;;
	esac
else
	repo_hooks="$(git rev-parse --git-common-dir)/hooks"
fi

# A configured directory the clone does not have runs nothing; say so once per commit, not
# once per hook (review I11: task 03 copies a generated one at create time). This fires only
# when the setting was copied without its folder: deleted in the project after install, refused
# by CopyGeneratedHooks, or deleted in the lane. A lane made before the project's install step
# has no local setting at all, so it is silent here (re-review b03 I-3; task-03's last bullet).
if [ -n "$(git config --local --get core.hooksPath 2>/dev/null)" ] && [ ! -d "$repo_hooks" ] &&
	[ "$name" = prepare-commit-msg ]; then
	echo "panacea: core.hooksPath names $repo_hooks, which this clone does not have; this repository's hooks did not run" >&2
fi

if [ "$repo_hooks" != "$self_dir" ] && [ -x "$repo_hooks/$name" ]; then
	"$repo_hooks/$name" "$@" || exit $?
fi

if [ "$name" = "prepare-commit-msg" ] && [ -n "${PANACEA_APP_SESSION_ID:-}" ]; then
	case "$PANACEA_APP_SESSION_ID" in
		*[!a-z0-9-]*) exit 0 ;; # not an id the control plane minted; stamp nothing
	esac
	git interpret-trailers --in-place --if-exists doNothing \
		--trailer "Panacea-Session: $PANACEA_APP_SESSION_ID" "$1"
fi
exit 0
```

```dockerfile
# images/sandbox/Dockerfile — before `USER dev`, so root owns the directory and the agent
# user cannot rewrite the hook. The sed strips any CR a Windows checkout introduced.
COPY images/sandbox/githooks/dispatch /opt/panacea/githooks/dispatch
RUN sed -i 's/\r$//' /opt/panacea/githooks/dispatch \
 && chmod 0755 /opt/panacea/githooks/dispatch \
 && cd /opt/panacea/githooks \
 && for h in applypatch-msg pre-applypatch post-applypatch pre-commit pre-merge-commit \
             prepare-commit-msg commit-msg post-commit pre-rebase post-checkout post-merge \
             pre-push reference-transaction push-to-checkout pre-auto-gc post-rewrite \
             sendemail-validate post-index-change; do ln -s dispatch "$h"; done
```

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEnvEntriesPutsTheContractFirstAndSortsTheRest` (amended) | order is the three, then `PANACEA_APP_SESSION_ID=<Spec.SessionID>`, then Env sorted | the id written anywhere else in the order |
| `TestEnvEntriesOmitsAnEmptySessionID` | `SessionID == ""` → no `PANACEA_APP_SESSION_ID` entry | an empty trailer value in the hook |
| `TestEnvEntriesIgnoresASessionIDInEnv` | `Env{"PANACEA_APP_SESSION_ID":"s-evil"}` with `SessionID:"s-1"` → exactly one entry, value `s-1` | a client-chosen session id |
| `TestThePodEnvironmentIsExactlySpecEnvEntries` (kata, unchanged) | still passes after `containerEnv` gains the variable | Docker and Kata environments diverging |
| `TestSessionEnvSetsHooksPath` (fix's file, extended) | with `n` the fix's pair count read before the change (8 at `feat/phase-2-cockpit`'s `10fa5fd`, re-read 2026-09-24, review I12): `GIT_CONFIG_COUNT=n+1`, `KEY_n=core.hooksPath`, `VALUE_n=/opt/panacea/githooks`; keys `0` to `n-1` byte-identical to the fix's, `core.autocrlf` and `core.eol` among them | dropping or renumbering one of the fix's settings while adding one |
| `TestALaneSessionSendsTheClonesLineEndings` (fix's file, extended; plan re-review b02 I-1) | a project and a lane clone of it (task 04's route); a `Deps.GitLineEndings` double that answers by path: `{false, lf}` in the clone's `Path`, `{true, crlf}` in `Project.Path`, and fails the test for any other path. A session created on the lane, then restarted: the recorded `backend.Spec.Env` carries `core.autocrlf=false` and `core.eol=lf`, found by **key** among the `GIT_CONFIG_KEY_n`, on both. Red first: a mutant that asks the resolver in `Project.Path` sends `true`/`crlf` | the resolver asked anywhere but the clone, which brings C2's split back inside the sandbox |
| `TestTheResolverReadsALaneCloneAsLF` (cmd/api, the fix's `gitlineendings_test.go`, extended) | production-shaped: a `gitxtest` project with `UseSystemConfig(WindowsInstallerSystem)`, cloned by task 03's `lanes.Creator`. The production runner reads the **process** environment, not `Repo.Env`, so the test also sets it the way the fix's `TestTheRealHostGitAnswersFromEveryScope` does (`t.Setenv` of `GIT_CONFIG_NOSYSTEM=""`, `GIT_CONFIG_SYSTEM` to a file holding `WindowsInstallerSystem`, `GIT_CONFIG_GLOBAL` to a temp file, `GIT_CONFIG_COUNT=""`). The production resolver, as `gitLineEndingsResolverWith(project.NewExecRunner(), "crlf")` (Windows' native, fixed so the test means the same on any host), answers `{"false", "lf"}` in the clone, and `{"true", "crlf"}` in the project (the control: the system file is in force). Red first against a `cloneArgv` without the `--config` flags, which answers `{"true", "crlf"}` in the clone | the double above answering what the real resolver never would |
| `TestSessionCreateAndRestartCarryTheSessionID` | the `backend.Spec` recorded by the fake backend has `SessionID` equal to the new session's id, on create **and** restart | a restart exporting the old sandbox's id |
| `TestSessionMountsBindGitConfigReadOnly` | a worktree with a `.git` directory → the mounts end with `{<wt>/.git/config, /home/dev/work/.git/config, ReadOnly: true}`; a `.git` file (a linked worktree) → no such entry | the H5 mount dropped, or bound read-write |
| `TestSandboxCloneConfigIsReadOnly` (docker) | task-02 §HGH H5, red first without the entry: in the real sandbox, `git config filter.x.clean y` in `/home/dev/work` exits non-zero, and the host's `.git/config` is byte-identical afterwards; `git commit` still succeeds. Then, with the session still live, the host runs `git branch -m` and `git config --remove-section branch.<b>` (task 11's writes, which replace the file by rename) through Exec: both succeed, and the test records whether `git config --list` in the sandbox then shows the new content or the old. A file bind pins an inode, so the old content is possible; the result goes in the task report and the plan's Q7, not asserted (b01 re-review Minor 1) | a nested read-only bind that Docker Desktop ignores (unverified until this runs) |
| `TestSandboxLaneCloneChecksOutLF` (docker; plan re-review b02 I-1) | a `gitxtest` project of LF blobs under `WindowsInstallerSystem`, cloned by `lanes.Creator`, run with the environment below (read from `sessionEnv`, with the lane constant as `eol`): `git status --porcelain` prints nothing, and after `rm a.txt` and `git checkout -- a.txt` the host reads `a.txt` with no `\r`. Red first with `eol` set to `{true, crlf}`, the answer in `Project.Path`: `status` still empty, the checkout writes `\r\n` | the sandbox's git writing CRLF over a lane's LF blobs while every status check reads clean |
| `TestDispatchStampsTheSession` (docker) | a commit in a clone mounted at `/home/dev/work` with the session environment carries `Panacea-Session: s-test1` exactly once (`git log -1 --format=%(trailers:key=Panacea-Session,valueonly)`) | the hook not installed, not executable, or CRLF |
| `TestDispatchSurvivesNoVerify` (docker) | `git commit --no-verify -m x` still carries the trailer | trusting `--no-verify` to skip only what we expect |
| `TestDispatchKeepsAnAmendsSession` (docker) | commit with `s-a`, then `git commit --amend` with `PANACEA_APP_SESSION_ID=s-b` → the trailer is `s-a`, once | a rewritten commit claiming a new author |
| `TestDispatchChainsToHusky` (docker; review I11) | the clone is made by task 03's `lanes.Creator` (its `Mounts` a fake that runs fn: no session is live on the project), never configured by the test: a `gitxtest` project whose **local** config has `core.hooksPath=.husky/_`, whose branch tracks `.husky/pre-commit` (exits 1) and `.husky/commit-msg` (appends `X-Husky: ran`), and whose working tree has an ignored `.husky/_/` in husky 9's stub shape: `.gitignore` `*`; `pre-commit` and `commit-msg` each the one line `. "${0%/*}/h"`; and an `h` that runs `$(dirname "$(dirname "$0")")/$(basename "$0")` with `sh -e` when that file exists (a stand-in: husky's own `h` was not read, re-review b03 Minor 2). The test writes nothing into the clone. In the sandbox `git commit` fails; with the tracked `.husky/pre-commit` changed to exit 0 and committed on the branch in the project (with `-c core.hooksPath=/dev/null`, since the old one still exits 1 there), and a second lane created, the commit has **both** trailers. Red first against a Creator without step 6 | risk R7: a husky repository losing its hooks on a real clone |
| `TestDispatchNamesAMissingHooksDir` (docker; review I11) | a lane whose clone has `core.hooksPath=.husky/_` and no `.husky/_`: `git commit` succeeds with the `Panacea-Session` trailer, and its stderr holds the `panacea: core.hooksPath names` line exactly once | a lane that silently runs none of the repository's hooks |
| `TestSandboxCannotWriteHooksPath` (docker; review I11) | in a lane sandbox, `git config core.hooksPath .husky/_` (the write husky's install step makes) exits non-zero and the host's `.git/config` is byte-identical: the reason task 03 copies the directory rather than leave it to the install step. If it exits 0, H5 is broken: stop and report | the premise of task 03's `CopyGeneratedHooks` going stale unnoticed |
| `TestDispatchChainsToDotGitHooks` (docker) | no local hooksPath; `.git/hooks/prepare-commit-msg` appends `X-Repo: ran` → both lines present, the repo's first | the default hooks directory skipped |
| `TestDispatchRefusesAForgedID` (docker) | `PANACEA_APP_SESSION_ID='s-1 Panacea-Land: w-x'` → no Panacea trailer at all | trailer injection through the environment |
| `TestDispatchCostIsBounded` (docker) | 50 commits with the dispatcher vs 50 with `core.hooksPath=/dev/null`: record both wall-clocks in the task report; fail only if the dispatcher is over 3× | a hook so slow agents notice |

The Docker tests build the image (`docker build -f images/sandbox/Dockerfile -t panacea-app/sandbox:phase3-test .`),
create repositories on the host with `gitxtest`, clone them with `lanes.Creator`, and run
`docker run --rm -v <clone>:/home/dev/work -e …` with **exactly** the environment
`backend.Spec{…}.EnvEntries()` plus `sessionEnv(...)` produce, read from those functions rather
than retyped. `sessionEnv`'s `eol` argument is `GitLineEndings{AutoCRLF: "false", EOL: "lf"}`:
the answer `TestTheResolverReadsALaneCloneAsLF` pins the production resolver to give in a lane
clone. It is written once, as a named constant in the test file, because the resolver lives in
package `main`, which this package cannot import.

#### Steps

- [ ] **Step 1: Preflight.** G1 holds, with the line-ending round in: `grep -c 'func sessionEnv(id GitIdentity, eol GitLineEndings' control-plane/internal/api/sessions.go`
  is 1, and `grep -c '{"core.eol", eol.EOL}' control-plane/internal/api/sessions.go` is 1. Record the
  fix's pair count `n` (`TestSessionEnvSetsHooksPath` uses it). If either grep is 0, stop and
  report (plan "Gate G1").
  G1's contract diff (review I12): `git diff 3303fd6 HEAD -- docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md`,
  and read every hunk in §2 and §5.1 before editing them: the fix branch already edited §2.2
  (`5e32000`) and §2.6 and §5.1 (`2d6d95b`), and phase 2's later rounds add more there.
  Task 01's `TestSandboxGitHonoursEnvHooksPath` passed and recorded the image's git version.
- [ ] **Step 2:** the Go tests; then `EnvSessionID`, `EnvEntries`, `containerEnv`, `sessionEnv`.
  `TestALaneSessionSendsTheClonesLineEndings` and `TestTheResolverReadsALaneCloneAsLF` may pass
  on arrival, since they pin the fix's behaviour: show each red against its named mutant on the
  `$TEMP` copy, and record that in the report.
- [ ] **Step 3:** `dispatch`, `.gitattributes`, the Dockerfile block. Check the committed blob is
  LF: `git ls-files --eol images/sandbox/githooks/dispatch` shows `i/lf`.
- [ ] **Step 4:** the Docker tests (PowerShell tool for `docker`).
- [ ] **Step 5:** contract: §2 lists `EnvSessionID` with the three; §5.1's environment paragraph
  adds `core.hooksPath` and the variable, and says the trailer is advisory.
- [ ] **Step 6:** commit by pathspec.

#### Acceptance

```
go test ./control-plane/... -race -count=1                                   → PASS
go test ./control-plane/internal/api/ -tags docker -run Dispatch -count=1 -v → PASS, 8 tests; the cost numbers in the report
go test ./control-plane/internal/api/ -tags docker -run TestSandboxCloneConfigIsReadOnly -count=1 -v → PASS (task-02 §HGH H5)
go test ./control-plane/internal/api/ -tags docker -run TestSandboxCannotWriteHooksPath -count=1 -v → PASS (review I11)
go test ./control-plane/internal/api/ -tags docker -run TestSandboxLaneCloneChecksOutLF -count=1 -v → PASS (plan re-review b02 I-1)
go test ./control-plane/cmd/api/ -run TestTheResolverReadsALaneCloneAsLF -count=1 -v              → PASS, not SKIP
git ls-files --eol images/sandbox/githooks/dispatch                          → i/lf  w/lf
```

#### What this task does not claim

- **The trailer is advisory** and says so on screen later: `git commit-tree`, `-c core.hooksPath=`,
  or unsetting the variable all skip it. The Files lens renders "no session recorded" for those
  and for commits made outside the cockpit, in the same words, because we cannot tell them apart.
- A commit in a worktree shared by two sessions names the one that ran `git commit`, not whose
  lines are in it (survey §5.3 item 2; 3-D2's notice, task 06).
- **Fresh-clone hooks (review I11).** `git clone` copies neither `.git/config` nor `.git/hooks`,
  so a lane runs the repository's hooks only through what task 03's Create put there: a
  relative local `core.hooksPath` (step 6) and, when the project had generated and ignored it,
  that directory's files as they were at create time. The agent cannot repair a lane from
  inside: the install step's `git config core.hooksPath` fails against the read-only
  `.git/config` (`TestSandboxCannotWriteHooksPath`). The dispatcher's `panacea:` line prints
  only when the clone has the setting but not its folder: the project's `.husky/_` was deleted
  after install (`git clean -fdx`), CopyGeneratedHooks refused it (over 64 files, a link, a
  subfolder, or a live primary-worktree session), or the agent deleted it in the lane. Three
  cases are **silent** and look like a repository with no hooks: a lane made before the
  project's install step ran (the install is what writes the local setting, so there was
  nothing to copy), an absolute hooks path, and hooks only in `.git/hooks` or a
  `core.hooksPath` naming it (the pre-commit framework's shim). Re-creating the lane after the
  install, with no primary-worktree session live, repairs a lane made before install, a deleted
  folder and a live-session refusal. A folder refused for its shape, an absolute path and
  `.git/hooks` reach no lane however it is made (re-review b03 I-3). Whether a copied hook is executable through Docker Desktop's bind
  mount is `TestDispatchChainsToHusky`'s to show (NTFS has no execute bit; unverified).
- Observation, not fixed here: `images/sandbox/entrypoint.sh` is **CRLF in this checkout**
  (`file` reports "with CRLF line terminators"; `core.autocrlf=true`, no `.gitattributes`). Whether
  that has broken an image build depends on where the image was built, which this plan did not
  check. The new `.gitattributes` rule is scoped to `githooks/`; widening it to `*.sh` is a
  one-line change the orchestrator may prefer, and is reported rather than made.
