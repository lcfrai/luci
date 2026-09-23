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
runs the repository's own hook of that name first. It is tested against a real husky layout in
the image before it ships.

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
// sessions.go (post-fix) — sessionEnv's git block becomes three entries.
const sessionHooksPath = "/opt/panacea/githooks" // images/sandbox/Dockerfile installs it

	env := map[string]string{
		"GIT_CONFIG_COUNT":   "3",
		"GIT_CONFIG_KEY_0":   "safe.directory",
		"GIT_CONFIG_VALUE_0": sessionWorkDir,
		"GIT_CONFIG_KEY_1":   "user.useConfigOnly",
		"GIT_CONFIG_VALUE_1": "true",
		// Phase 3: the session trailer. The dispatcher there runs the repository's own hook of
		// every name first, so a husky repository keeps its hooks (risk R7).
		"GIT_CONFIG_KEY_2":   "core.hooksPath",
		"GIT_CONFIG_VALUE_2": sessionHooksPath,
	}
```

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
| `TestSessionEnvSetsHooksPath` (fix's file, extended) | `GIT_CONFIG_COUNT=3`, `KEY_2=core.hooksPath`, `VALUE_2=/opt/panacea/githooks`; keys 0 and 1 unchanged | dropping the fix's two settings while adding a third |
| `TestSessionCreateAndRestartCarryTheSessionID` | the `backend.Spec` recorded by the fake backend has `SessionID` equal to the new session's id, on create **and** restart | a restart exporting the old sandbox's id |
| `TestSessionMountsBindGitConfigReadOnly` | a worktree with a `.git` directory → the mounts end with `{<wt>/.git/config, /home/dev/work/.git/config, ReadOnly: true}`; a `.git` file (a linked worktree) → no such entry | the H5 mount dropped, or bound read-write |
| `TestSandboxCloneConfigIsReadOnly` (docker) | task-02 §HGH H5, red first without the entry: in the real sandbox, `git config filter.x.clean y` in `/home/dev/work` exits non-zero, and the host's `.git/config` is byte-identical afterwards; `git commit` still succeeds | a nested read-only bind that Docker Desktop ignores (unverified until this runs) |
| `TestDispatchStampsTheSession` (docker) | a commit in a clone mounted at `/home/dev/work` with the session environment carries `Panacea-Session: s-test1` exactly once (`git log -1 --format=%(trailers:key=Panacea-Session,valueonly)`) | the hook not installed, not executable, or CRLF |
| `TestDispatchSurvivesNoVerify` (docker) | `git commit --no-verify -m x` still carries the trailer | trusting `--no-verify` to skip only what we expect |
| `TestDispatchKeepsAnAmendsSession` (docker) | commit with `s-a`, then `git commit --amend` with `PANACEA_APP_SESSION_ID=s-b` → the trailer is `s-a`, once | a rewritten commit claiming a new author |
| `TestDispatchChainsToHusky` (docker) | a clone whose `.git/config` has `core.hooksPath=.husky/_` and whose `.husky/_/pre-commit` exits 1 → `git commit` fails; with it exiting 0 and `.husky/_/commit-msg` appending `X-Husky: ran`, the commit has **both** trailers | risk R7: a husky repository losing its hooks |
| `TestDispatchChainsToDotGitHooks` (docker) | no local hooksPath; `.git/hooks/prepare-commit-msg` appends `X-Repo: ran` → both lines present, the repo's first | the default hooks directory skipped |
| `TestDispatchRefusesAForgedID` (docker) | `PANACEA_APP_SESSION_ID='s-1 Panacea-Land: w-x'` → no Panacea trailer at all | trailer injection through the environment |
| `TestDispatchCostIsBounded` (docker) | 50 commits with the dispatcher vs 50 with `core.hooksPath=/dev/null`: record both wall-clocks in the task report; fail only if the dispatcher is over 3× | a hook so slow agents notice |

The Docker tests build the image (`docker build -f images/sandbox/Dockerfile -t panacea-app/sandbox:phase3-test .`),
create repositories on the host with `gitxtest`, clone them with `lanes.Creator`, and run
`docker run --rm -v <clone>:/home/dev/work -e …` with **exactly** the environment
`backend.Spec{…}.EnvEntries()` plus `sessionEnv(...)` produce, read from those functions rather
than retyped.

#### Steps

- [ ] **Step 1: Preflight.** G1 holds: `grep -c '"GIT_CONFIG_COUNT":   "2"' control-plane/internal/api/sessions.go` is 1.
  Task 01's `TestSandboxGitHonoursEnvHooksPath` passed and recorded the image's git version.
- [ ] **Step 2:** the Go tests; then `EnvSessionID`, `EnvEntries`, `containerEnv`, `sessionEnv`.
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
git ls-files --eol images/sandbox/githooks/dispatch                          → i/lf  w/lf
```

#### What this task does not claim

- **The trailer is advisory** and says so on screen later: `git commit-tree`, `-c core.hooksPath=`,
  or unsetting the variable all skip it. The Files lens renders "no session recorded" for those
  and for commits made outside the cockpit, in the same words, because we cannot tell them apart.
- A commit in a worktree shared by two sessions names the one that ran `git commit`, not whose
  lines are in it (survey §5.3 item 2; 3-D2's notice, task 06).
- Observation, not fixed here: `images/sandbox/entrypoint.sh` is **CRLF in this checkout**
  (`file` reports "with CRLF line terminators"; `core.autocrlf=true`, no `.gitattributes`). Whether
  that has broken an image build depends on where the image was built, which this plan did not
  check. The new `.gitattributes` rule is scoped to `githooks/`; widening it to `*.sh` is a
  one-line change the orchestrator may prefer, and is reported rather than made.
