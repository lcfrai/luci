# Phase 3 — Git, worktrees, coordination and convergence — implementation plan

> **For agentic workers:** execute task by task, in the order of the task list at the end. Each
> task's full specification is its part file in
> `docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-NN.md`. The part file is
> the contract for that task; this file is the map. Steps use checkbox (`- [ ]`) syntax.

**Status: written 2026-09-23, not yet adversarially reviewed** (OD-1: every plan is reviewed
before code). 23 tasks, each specified in its part file. Inputs: the owner's decisions OD-1–OD-13,
`scope-and-risk-phase-3-git-convergence.md`, `survey-phase-3-git-convergence.md`,
`prototype/phase-3-convergence.html`, the tree at `3303fd6`, and `fix/sandbox-git-ca` at `2d6d95b`
plus its then-uncommitted `LinkedWorktreeNote`. **Re-baselined 2026-09-24 (review I12):** the fix
branch is at `4f6d698`, merged into `feat/phase-2-cockpit` at `f898f5e`, which is the commit
`feat/phase-3-git` forks from; phase 2's later sandbox-git rounds run to `10fa5fd` ("How this plan
supersedes the sandbox git fix" below).

## Goal

Make a Panacea worktree **real git that works inside its sandbox**, and converge several of them
into their product branch with provenance. Concretely, when this plan is done:

1. The cockpit creates and removes worktrees. A worktree is a **local clone** of the project
   repository (OD-2), checked out on the product branch's own name, so an agent inside the sandbox
   has a working git whose world matches the rail. The linked-worktree git failure that phase 2's
   sandbox fix could only *announce* is gone for every worktree the cockpit creates.
2. Every agent commit carries `Panacea-Session: <id>`, added by a git hook the image ships and the
   control plane enables through environment, chained to the repository's own hook.
3. The control plane fetches each clone's work back into the project repository under
   `refs/panacea/lanes/<worktree-id>` and computes, from git alone, what each worktree changed, what
   it has not committed, and whether it will land clean against the branch **as it will be** after
   the worktrees above it in the queue.
4. A human lands worktrees into their branch one at a time, in the order they set, by plumbing with
   compare-and-swap. The merge commit carries the brief key the human saw as a trailer. A land can be
   undone exactly until it is pushed or built upon, and by a revert commit after.
5. The branch pushes from the control plane with the user's own credentials, and a PR opens on
   **Azure DevOps or GitHub** behind one host interface (OD-4). The sandbox never holds a git
   credential, and it cannot obtain one through the host's git either. Every host git call is
   hardened against the config the sandbox writes (decision 9, task-02 §HGH).
6. Each diff hunk links to the commit and session that wrote it (tier 1, always), and, for Claude
   Code, to the tool call that wrote it through a version-pinned, canary-tested transcript adapter
   that degrades to tier 1 (OD-8).
7. The **Merge** and **Files** lenses show all of the above, and nothing a human cannot trace to a
   git command or a recorded fact.

## Architecture

Three places hold git, and the control plane is the only process that runs git in two of them.

```
 project repository (the user's clone,          lane clones, one per cockpit worktree
 under PANACEA_APP_PROJECTS_DIR)                 (under PANACEA_APP_WORKTREES_DIR, default
                                                  statepath.Dir("worktrees"))
 ┌───────────────────────────────┐   git clone --no-hardlinks (objects copied, Q9)
 │ refs/heads/evidence-upload     │ ─────────────────────────────►  ┌──────────────────────────┐
 │ refs/panacea/lanes/<w-id>      │ ◄──── lane sync: git fetch ───  │ refs/heads/evidence-upload│ ◄── agent's git,
 │ refs/panacea/archive/…         │       (control plane, host)     │ refs/remotes/origin/…     │     inside the sandbox,
 │ refs/panacea/undone/<sha>      │ ───── origin refresh ─────────► │ .git is a DIRECTORY       │     bind-mounted at
 │ refs/remotes/origin/*          │       after every land          └──────────────────────────┘     /home/dev/work
 └──────────────┬────────────────┘
                │ fetch / push (control plane, host, the user's credentials)
                ▼
        the git host — GitHub or Azure DevOps (OD-4), PRs through githost.Host
```

**The model (OD-2).** A cockpit worktree is a local clone of the project repository, checked out
on the product branch's own name. The clone's `.git` is a directory inside the clone, so the
bind mount phase 2 already makes (`sessionMounts` in `api/sessions.go`) carries a complete,
working repository into the sandbox. Nothing about the mount changes: its host path becomes the
clone's directory. The clone's `origin` names the project repository by host path, which the
sandbox cannot reach. The agent sees `origin/<branch>` as the control plane last refreshed it and
cannot fetch or push, and that is the design (brief A4).

**A clone shares no file with the project, and holds the blobs' bytes.** Task 03 clones with
`--no-hardlinks`, so no object file in the clone is the project's file (Q9). Its `--config`
writes `core.autocrlf=false`, `core.eol=lf`, `core.checkStat=minimal` and `core.trustctime=false`
into the clone's local config before the checkout, so the working tree holds the blobs' bytes
under the system `core.autocrlf=true` Git for Windows installs (review C2; task 03 "Line
endings"). The host's git reads those local keys. The sandbox's git does not: on a base with
phase 2's `5a4d0d8`, `sessionEnv` sends `core.autocrlf` and `core.eol` as `GIT_CONFIG_*`, and
command scope outranks every config file. The two gits agree only because `Deps.GitLineEndings`
is asked in the session's worktree path, which for a lane is the clone, and there the host's git
reads the clone's local `false` and `lf`. **That is the invariant: the line-ending resolver is
asked in the clone, never in `Project.Path`** (task 05 pins it). The local keys still govern a
sandbox session started without that environment.

**The control plane converges by fetching, never by sharing a filesystem.** A background lane
syncer, on the control plane's own timer and never behind a GET, copies each clone's
`refs/heads/<branch>` into the project repository as `refs/panacea/lanes/<worktree-id>`. Every
convergence fact (changed paths, `merge-tree` forecasts, lands, evidence ancestry) is computed in
the project repository against those refs. Uncommitted paths are read from the clone with
`git status` under `GIT_OPTIONAL_LOCKS=0`, which writes nothing.

**A land is plumbing with compare-and-swap (brief A5).** `merge-tree --write-tree`, then
`commit-tree` with the merge link as trailers, then `update-ref <branch> <new> <old>`. No
worktree and no index is touched, so the product branch must not be checked out anywhere, and a
land refuses when it is.

**git is the truth for every git fact (brief A1).** Phase 3 persists no tip, dirty flag,
ahead/behind or conflict. It persists four things git cannot hold: the lane index (which clone
belongs to which project and branch, and the queue order), the branch rename map (A8), the
evidence records (3-D5), and nothing else. The merge link lives in git as a trailer (3-D6).

**Everything that runs git is keyed by machine (brief A6).** `gitx.Machine` has one
implementation, `gitx.Local`. Phase 8 adds the Kata one: a pod gets a clone and the control plane
fetches from it, which is this same model behind the same interface.

**Packages.** Every git write lives outside `internal/project`, whose read-only whitelist test
stays exactly as it is (task 26 seam S3.1).

| Package | Owns | Task |
|---|---|---|
| `internal/gitx` | the write-capable git runner, its timeout classes, `*gitx.Error` with git's own sentence, the `Machine` seam, trailer formatting | 02 |
| `internal/lanes` | the lane index, clone create and remove, the composite catalog that lists clones beside `git worktree list` | 03, 04, 11 |
| `internal/converge` | lane sync, observed claims, the chained forecast, land, history, undo, revert, branch create/rename/delete | 07–11 |
| `internal/remote` | fetch (on demand and on a slow timer), ahead/behind, push | 12, 13 |
| `internal/credstore` | the credential-store seam the Hub fills, plus a local fallback through the machine's git credential helper | 13 |
| `internal/githost` | remote-URL recognition and the git auth header (13); one host interface with GitHub and Azure DevOps implementations (14) | 13, 14 |
| `internal/briefref` | the two interfaces phase 3 needs from phase 4 and the Hub: resolve a brief key, archive a brief head. Phase 3 never parses a key (contract P5) | 09 |
| `internal/evidence` | evidence records, the store seam the Hub fills, the evaluator over git ancestry | 15 |
| `internal/changes` | the Files read model: changed paths across lanes, per-path hunks, tier-1 back-links | 16 |
| `internal/transcript/claudecode` | the OD-8 adapter: pinned, canary-tested, read-only, degrading to tier 1 | 17 |
| `images/sandbox/githooks/` | the hook dispatcher that stamps `Panacea-Session` and chains to the repository's own hooks | 05 |

The HTTP handlers for all of it live in new files in `internal/api` (`worktrees.go`,
`convergence.go`, `lands.go`, `branches.go`, `remote.go`, `changes.go`, `evidence.go`), and every
route registers through the recorder (`api.go`'s `routes`, seam SX.4).

**The desktop** gains one unpersisted slice (`convergenceSlice`), loaded on the existing poll
tick for the selected branch only while Merge or Files is open (orchestrator ruling R6: no new
interval). It fills `FilesLens.tsx` and `MergeLens.tsx`, which task 26 creates (seam SX.1), and
edits no other lens.

## The decisions this plan rests on

Owner decisions are in `docs/superpowers/planning/2026-09-23-owner-decisions.md`. The brief is
`docs/superpowers/planning/2026-09-23-phases-4-5-7/scope-and-risk-phase-3-git-convergence.md`.

| Id | Decision | Where it lands |
|---|---|---|
| **OD-2** (= 3-D1 b) | A worktree is a local clone, work fetched back under `refs/panacea/lanes/<worktree-id>`. OD-2 says "objects hard-linked"; this plan **copies** them (`clone --no-hardlinks`), because a hard-linked object is the project's own file, writable from the sandbox through the bind mount, and OD-2's "blast radius is its own clone" is false while it is (review I8; open question Q9) | tasks 01, 03, 04, 07 |
| **OD-4** (= 3-D8) | Azure DevOps **and** GitHub, behind one host interface | task 14 |
| **OD-6** | Tokens live in Windows Credential Manager, the store the Hub builds; users are always themselves | task 13 (the seam) |
| **OD-8** (= 3-D7 b) | The Claude Code transcript may be read, read-only, through an adapter pinned to the image's CLI version, canary-tested, degrading to commit level | task 17 |
| **OD-10** | Removing a worktree deletes its conversation store | task 11 |
| **OD-3 / OD-5** | Shared stores live in the Hub. Phase 3's evidence store has the brief's shape and the Hub's location | task 15 (seam), open question Q2 |
| **OD-12** | Owner hands-on checks are batched at the end; until then every owner-observed item is **unverified** | task 23 |
| 3-D2 | Two agents in one worktree: allowed, and the worktree says so, with one click to start in a new worktree instead | tasks 06, 19, 22 |
| 3-D3 | A conflict is resolved in the lane: one click starts a session there with an opening prompt, shown and editable first | task 19 |
| 3-D4 | A lane lands as a merge commit (`--no-ff` in effect: two parents, always) | task 09 |
| 3-D5 | Evidence is recorded by a human, at a commit, with a required *because* | task 15, 20 |
| 3-D6 | Trailers enter history (`Panacea-Session`, `Panacea-Land`, `Panacea-Brief`, `Panacea-Brief-Digest`); `CLAUDE.md`'s managed block and the design handoff bundle do not | tasks 05, 09; seam to phase 4 and 5 |
| 3-D9 | Files shows changed paths only, across all of the branch's worktrees | tasks 16, 21 |
| 3-D10 | Authority is not phase 3; the phase number comes off `TopBar.tsx:29` | task 22 |
| A1–A9 | The brief's assertions, all accepted (OD-1: "every other recommendation on file is accepted") | throughout |
| R2 | `writeAtomic` is lifted into `statepath`, not duplicated | task 03 step 1, if phase 4 has not done it first |
| R3 | Phase 3 claims **no** persist version. Nothing it adds to the desktop is persisted | task 18 |
| R6 | The desktop loads on the existing poll tick; no new `setInterval` | task 18 |

### Decisions this plan makes where nothing on file does

Each is small, stated so a reviewer can overturn it, and none changes what the owner decided.

1. **Queue order lives in the control plane, in the lane index.** The forecast is computed
   against *the branch as it will be after the rows above*, so the server must know the order.
   `PUT …/queue` sets it; the desktop reorders through `moveWithinVisible` (seam S3.4) and sends
   the whole order. This keeps the desktop's persisted state unchanged (R3) and the forecast a
   server fact.
2. **A land clears the landed worktree's `ready` flag.** Ready meant "this work is ready to land";
   once it has landed, new work in that worktree is marked ready again by a person (known defect
   9: nothing infers it).
3. **After every land, the control plane refreshes `origin/<branch>` in every clone of that
   branch.** It writes only a remote-tracking ref, which the agent does not write. This is what
   makes 3-D3's opening prompt ("merge `origin/evidence-upload` into this branch") true. The
   refresh fetches from the project's literal path, taken from the lane index, and never from the
   clone's configured `origin`, which the agent can rewrite (review C3; task-02 §HGH H2).
4. **A land or an evidence record that names a brief key is refused when no resolver is
   configured.** Writing an unverifiable key into immutable history is worse than writing none. A
   land with no key records no brief trailer and renders `none`.
5. **Evidence matches by criterion-text digest, not by key or position.** The positional id breaks
   on insertion (risk R14); a key-based match would need phase 3 to parse keys (P5). The digest is
   supplied by the resolver, which phase 4 owns. This is the brief's §4.2 amendment, realised.
6. **`merge-tree` and forecast `commit-tree` run inside GETs, and that is accepted.** They write
   only content-addressed objects reachable from no ref. Forecast commits use a fixed identity and
   a fixed date, so the same inputs produce the same object ids and a repeated GET writes nothing
   new. No ref, index or worktree changes in any GET. `routes_audit_test.go` records this reason.
7. **A worktree created by the cockpit is identified as a clone on the wire.** `Worktree` gains
   `kind: "primary" | "linked" | "clone"`, a declared amendment to both contracts (task 06). The
   alternative, a lane set in a phase 3 slice, would let the linked-worktree notice render on a
   clone until that slice loaded, which is a false claim on screen.
8. **Two new problem codes, no new kind.** `git_failed` (a git process the control plane ran
   failed: 502 when it talked to a remote, 500 otherwise; `details.gitSays` is git's sentence) and
   `host_failed` (the GitHub or Azure DevOps API refused, 502; `details.hostSays`). Both classify
   as `server-said` through `classify`'s existing catch-all. Every other refusal is `conflict`
   with a `details.reason` from the closed list in task 04's `gitdeps.go`.
9. **Host git never runs what a repository names** (review C1 and C3; task-02 §HGH). The sandbox
   can write a clone's `.git`, and on a primary-worktree session the project's too. So every host
   git call carries a command-scope prefix that disables hooks, fsmonitor, ssh commands, askpass,
   signing and repository credential helpers, and allows only the `file` transport (plus `https`
   on a network call). A config check refuses driver, include, URL-rewrite and per-remote proxy
   keys, and credential and `http` keys on a network call (`clone_config_untrusted`,
   `project_config_untrusted`). Every call names its `Scope`; there is no default. The sandbox
   mounts `.git/config` read-only, and Exec pins `GIT_DIR` and `GIT_COMMON_DIR`, so a
   `.git/commondir` cannot move the config git reads. Network calls take their proxy, TLS and
   helpers from the machine's config, never the repository's.

## How this plan supersedes the sandbox git fix (`fix/sandbox-git-ca`)

The fix is **the base this plan builds on**. Re-read on 2026-09-24 (review I12) with
`git log 3303fd6..fix/sandbox-git-ca`, `git log f898f5e..feat/phase-2-cockpit` and
`git show <ref>:<path>`:

- `fix/sandbox-git-ca` is at `4f6d698`, five commits past `3303fd6`: `2d6d95b` (the fix, based on
  `69bbc02`), `58d607f` (`LinkedWorktreeNote`, now committed), `5e32000` (the contracts swept:
  Go §2.2, UI §6.5 and §7.7; across the fix branch (`2d6d95b`, `5e32000`),
  `git diff --stat 3303fd6...fix/sandbox-git-ca` shows 55 lines in the Go contract, in §2.2,
  §2.6, §4 and §5.1, and 7 in the UI contract, in §6.5 and §7.7), `73dd041` (`run()` in the
  identity test made hermetic) and `4f6d698` (gofmt).
- `feat/phase-2-cockpit` merged it at `f898f5e`, and `feat/phase-3-git` forks from `f898f5e`, so
  those five are in phase 3's base.
- Phase 2's later sandbox-git rounds are on `feat/phase-2-cockpit` only (head `10fa5fd`):
  `5a4d0d8` (line endings from the host; a primary's linked-worktree metadata mounted read-only),
  `d3a2824` (UI §6.5's `LinkedWorktreeNote` row), `57c3bf5` (`gc.auto=0`), and `d7e2dea` with
  `10fa5fd` (the host's global excludes and attributes files, mounted read-only).
- At `10fa5fd` the shapes are `sessionMounts(worktreePath, worktreeAdmin, conversationStore,
  caBundle string, git sessionGitFiles)` and `sessionEnv(id GitIdentity, eol GitLineEndings, git
  sessionGitFiles, caBundle string)` with eight `GIT_CONFIG_*` pairs. `Deps.GitIdentity` and
  `LinkedWorktreeNote`'s condition (`worktree.isPrimary`) and sentence are unchanged.

| The fix does | Phase 3 does | Kept, changed or superseded |
|---|---|---|
| Mounts the worktree at `/home/dev/work`, the conversation store, and the CA bundle read-only; since phase 2's later rounds also a primary's linked-worktree metadata and the host's global excludes and attributes files, each read-only (`sessionMounts(worktreePath, worktreeAdmin, conversationStore, caBundle string, git sessionGitFiles)` at `10fa5fd`) | Leaves all of those mounts as they are, and adds one: when the worktree's `.git` is a directory (every clone, and a primary worktree), `<path>/.git/config` is bound read-only on top (task 05; task-02 §HGH H5; review C1). For a cockpit worktree the worktree mount's host path is the clone, whose `.git` is a directory, so no `.git` common-dir mount is ever added (brief 3-D1 option a is rejected) | **Kept, plus one read-only file.** The mount model the brief calls "linked-worktree mounts" is replaced by clones, not by more directory mounts |
| `sessionEnv(id, eol, git, caBundle)` (as of `10fa5fd` on `feat/phase-2-cockpit`, re-read 2026-09-24, review I12): a `pairs` slice numbered into `GIT_CONFIG_KEY_n`/`VALUE_n` (`safe.directory=/home/dev/work`, `user.useConfigOnly=true`, `gc.worktreePruneExpire=never`, `gc.auto=0`, `core.autocrlf` and `core.eol` from `Deps.GitLineEndings` asked in the session's worktree path, `core.excludesFile` and `core.attributesFile`), `GIT_CONFIG_COUNT` their number (eight at `10fa5fd`; six at `57c3bf5`); `GIT_AUTHOR_*`/`GIT_COMMITTER_*` from the identity; `NODE_EXTRA_CA_CERTS` | Appends one pair, `core.hooksPath=/opt/panacea/githooks`, after the fix's own, which it never renumbers. Relies on the resolver being asked in the clone for a lane session, and pins it (task 05; plan re-review b02 I-1; "A clone shares no file" above). Adds `PANACEA_APP_SESSION_ID`, a reserved variable `EnvEntries` writes from `Spec.SessionID` (task 05) | **Extended.** `safe.directory` is still needed: a clone's bind mount has the same foreign owner |
| `gitIdentityResolver` asks `git var GIT_AUTHOR_IDENT` in the worktree's host path | A clone does not inherit its source's `.git/config`, nor an `includeIf gitdir:` that matched the project's path. So clone creation asks `git var` **in the project repository** and writes the answer into the clone's local `user.name`/`user.email` (task 03). The fix's per-worktree resolution then gives the same answer in the clone that it gives in the project | **Kept, with one addition at clone time** |
| `LinkedWorktreeNote` renders for every worktree with `isPrimary: false`, saying git "arrives in Phase 3" | The condition narrows to `kind === "linked"`: a worktree made with `git worktree add` outside the cockpit, where git is still unavailable. A clone is `kind: "clone"`, `isPrimary: false`, and renders no notice. The sentence is rewritten to name the fix the user can take, and stops promising a phase (task 06) | **Superseded** for clones; **kept and reworded** for linked worktrees |
| Git over HTTPS "is phase 3's" (the fix's commit message) | Push and fetch run on the host, never in the sandbox; the sandbox holds no credential (A4). `NODE_EXTRA_CA_CERTS` does not affect git on the host, which uses whatever trust the host's git is already configured with (its `http.sslBackend` — not read for this plan) | **Answered**: nothing git-over-HTTPS happens in a sandbox |

**Gate G1.** Task 04 and every later task assume the fix is in the branch phase 3 builds on.
Baseline (review I12): `fix/sandbox-git-ca` at `4f6d698`, which phase 3's base already holds
(`git merge-base --is-ancestor 4f6d698 HEAD` exits 0, since `feat/phase-3-git` forks from
`f898f5e`). "The fix" also includes phase 2's later sandbox-git rounds, `5a4d0d8` to `10fa5fd`
on `feat/phase-2-cockpit`, which phase 3's base does **not** hold yet. Tasks 04 and 05 each open
with the same two greps for the line-ending round (`func sessionEnv(id GitIdentity, eol
GitLineEndings` and `{"core.eol", eol.EOL}` in `sessions.go`; both 1 at `10fa5fd`, the first 0 at
`8236ba1`), so the stop fires where it is written (re-review b03 I-4). If they are not in phase
3's branch when task 04 starts, the implementer stops and reports; phase 3 does not re-implement
them. Task 04 needs them itself: it edits session create and restart (its mount guard). Before tasks 04–06 append to a
contract section, each preflight runs `git diff 3303fd6 HEAD -- docs/superpowers/contracts/` and
reads every hunk in the sections it amends (Go §2, §3.1, §3.2, §5.1; UI §2, §6.5, §7.7), because
the fix already edited §2.2, §5.1, §6.5 and §7.7. If the fix merges with a different
shape from the one quoted above (`sessionEnv`'s pairs, `GitIdentity`'s signature,
`LinkedWorktreeNote`'s condition), tasks 04–06
re-read it and adjust their preflight greps, and the change is reported in the task's commit.

## Global constraints

Everything in phase 2's Global Constraints (`2026-09-18-phase-2-backend-and-cockpit.md:42-60`)
still binds. Restated where phase 3 leans on it, and added where phase 3 is the first to need it:

- **Commands.** Go: `go test ./sandboxd/... ./control-plane/... -race -count=1` from the repo
  root (`./...` is invalid at a `go.work` root). Desktop: `cd desktop && npx vitest run` and
  `npx tsc --noEmit`. A test that needs real git uses `t.TempDir()` and the host's git
  (2.53.0.windows.2 on the owner's laptop); a test that needs Docker is behind
  `//go:build docker` and names itself in its task's acceptance section. Every shell command an
  agent runs goes through `~/.claude/tools/claudezsh/claudezsh`.
- **The `go` directive stays `go 1.23`** in `go.work` and both modules. Phase 3 adds **no Go
  module dependency**: HTTP clients are `net/http`, JSON is `encoding/json`, and git is a
  subprocess. `git diff` on the `go.mod` files before every commit.
- **Commit trailer**, exactly: `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.
  Commit by pathspec (`git commit -F <file> -- <paths>`); never `add -A`, stash, reset, checkout,
  rebase, amend or push.
- **Agent CLIs are never modified, wrapped or parsed, and nothing derives meaning from PTY bytes.**
  The one exception is OD-8's transcript adapter (task 17), which reads a file, read-only, and
  which no feature requires. The git hook of task 05 is git's mechanism, enabled by environment;
  the CLI never knows it exists.
- **A sandbox never holds a git credential** (A4). Fetch and push run in the control plane on the
  host. No token is ever written to disk by phase 3, placed on a command line, logged, or
  returned over HTTP.
- **No force-push, ever** (A3). No code path in phase 3 passes `-f`, `--force`,
  `--force-with-lease` or a `+` refspec to `git push`. Task 12 tests the argv.
- **Never pool subscriptions or credentials across users.** Every credential phase 3 uses is the
  signed-in user's own, from the user's own store (OD-6).
- **Loopback only.** Phase 3 adds routes behind the existing origin guard and the loopback bind.
  It adds no listener. Control-plane authentication is the Hub's (critique-2 §7.1 #4 answered by
  OD-6); phase 3 adds a push route behind today's boundary and says so (open question Q5).
- **No GET changes state** (Group C §10a). Every new route registers through the recorder
  (`routes` in `api.go`) and every new GET gets a line in `catalogReadOnlyRoutes`
  (`routes_audit_test.go:32`) with its reason. The lane syncer and the fetch timer run on the
  control plane's own timers, started in `cmd/api/main.go` beside the reaper, never behind a
  request.
- **`internal/project` stays read-only.** `TestEveryGitCommandIsReadOnly`
  (`project_test.go:429`) is not edited. The one change phase 3 makes in that package is a
  struct field (`Worktree.Kind`, task 03) that runs no git.
- **Configuration comes from `PANACEA_APP_*`**, read in `cmd/api/config.go` only (contract §4):
  `PANACEA_APP_WORKTREES_DIR` (task 04), `PANACEA_APP_LANE_SYNC_INTERVAL` (task 07),
  `PANACEA_APP_FETCH_INTERVAL` (task 12). No other package calls `os.Getenv`.
- **git's own sentence is shown verbatim** when git refuses (the prototype's `gitsays` lines),
  capped at 2 KiB, through `details.gitSays`. The control plane writes the sentence around it; it
  never paraphrases git.
- **UI tasks** (18–22) hold to `docs/DESIGN-SYSTEM.md`'s non-negotiables: no indigo/violet, no
  gradient anywhere (phase 2 task 24's `designsystem.test.ts` greps for `gradient(`), no gradient-hero or 3-card grid, no
  blanket glassmorphism; elevation is a 1px ring plus layered shadow (`--e-*` tokens), never a flat
  `1px solid` border; radius from `--r-chip`/`--r-ctl`/`--r-card`/`--r-window` by element size;
  motion only to explain causality, `transform`/`opacity` only, never `transition: all`; one
  global heartbeat (`hooks/heartbeat.ts`, seam SX.8), never a spinner per row; `tabular-nums` on
  every live number; no counts outside the queue and no "2 of 4" (AGENTIC-UX §10). **Visual
  acceptance is the owner's eyes (OD-12), batched at the end; every UI task's report says its look
  is unverified.**

## The HTTP surface phase 3 adds

All under `/v1`, behind the origin guard, registered through the recorder, and served under the
`/api/v1` alias automatically. Each row's complete request and response shapes are in its task's
part file; task 18 restates them as TypeScript.

| Method and pattern | Does | Writes | Task |
|---|---|---|---|
| `POST /v1/projects/{projectId}/worktrees` | create a worktree (a lane clone) on a branch | clone dir, lane index | 04 |
| `DELETE /v1/projects/{projectId}/worktrees/{worktreeId}` | remove a cockpit worktree, after a confirm token | clone dir, lane index, archive ref, conversation store | 11 |
| `POST /v1/projects/{projectId}/branches` | create a product branch | `refs/heads/<name>` | 11 |
| `PATCH /v1/projects/{projectId}/branches/{branchId}` | rename a product branch | `refs/heads/*`, rename map | 11 |
| `DELETE /v1/projects/{projectId}/branches/{branchId}` | delete a product branch, archiving its tip | archive ref, `refs/heads/<name>` | 11 |
| `GET /v1/projects/{projectId}/renames` | the rename map, `old branch id → new branch id` | — | 11 |
| `GET /v1/projects/{projectId}/branches/{branchId}/convergence` | the Merge read model | objects only (decision 6) | 08 |
| `PUT /v1/projects/{projectId}/branches/{branchId}/queue` | set the queue order | lane index | 08 |
| `POST /v1/projects/{projectId}/branches/{branchId}/lands` | land one worktree | merge commit, `refs/heads/<branch>`, ready flag, clones' `origin/<branch>` | 09 |
| `GET /v1/projects/{projectId}/branches/{branchId}/lands` | landed history, first-parent, with undo state | — | 10 |
| `POST /v1/projects/{projectId}/branches/{branchId}/lands/{sha}/undo` | exact undo | `refs/heads/<branch>`, `refs/panacea/undone/<sha>` | 10 |
| `POST /v1/projects/{projectId}/branches/{branchId}/lands/{sha}/revert` | land a revert commit | revert commit, `refs/heads/<branch>` | 10 |
| `POST /v1/projects/{projectId}/fetch` | fetch `origin` now | remote-tracking refs, `FETCH_HEAD` | 12 |
| `POST /v1/projects/{projectId}/branches/{branchId}/push` | push the branch, never forced | the remote; upstream config on first push | 12, 13 |
| `POST /v1/projects/{projectId}/branches/{branchId}/pull-requests` | open (or find) a PR on the host | the host | 14 |
| `POST /v1/projects/{projectId}/evidence` | record evidence at a commit | evidence store | 15 |
| `GET /v1/projects/{projectId}/evidence` | evidence records by criterion digest | — | 15 |
| `GET /v1/projects/{projectId}/evidence/states` | criterion states at a commit, for a brief key the resolver answers | — | 15 |
| `GET /v1/projects/{projectId}/branches/{branchId}/changes` | the Files read model | objects only (decision 6) | 16 |
| `GET /v1/projects/{projectId}/branches/{branchId}/changes/diff` | one path's hunks per worktree, with back-links | — | 16, 17 |
| `GET /v1/sessions/{id}/transcript/spans/{toolUseId}` | the tier-2 span: one tool call and the text before it | — | 17 |

## Seams

### Gates phase 3 waits on

| Gate | What | Blocks | If it has not landed |
|---|---|---|---|
| **G1** | `fix/sandbox-git-ca` at `4f6d698` in phase 3's base (it is, through `f898f5e`), with phase 2's later sandbox-git rounds `5a4d0d8` to `10fa5fd` (on `feat/phase-2-cockpit`, not yet in phase 3's base) | tasks 04–06, and everything that starts a session | stop and report (above) |
| **G2** | Phase 2 task 26 lands: `components/lens/FilesLens.tsx` and `MergeLens.tsx` exist and `AppShell.tsx` is closed (seam SX.1). **Verified absent on 2026-09-23**: `desktop/src/components/lens/` holds only `LensPlaceholder.tsx`, and `AppShell.tsx:44-58` still routes `files` and `merge` to `LensPlaceholder phase={3}` | tasks 19–21 | the desktop half waits. It never edits `AppShell.tsx`: that reopens the four-way collision `dependency-graph-and-order.md` §2.2 counts |
| G3 (soft) | Phase 4's `provenance` slice zero: a `briefref.Resolver` implementation and `DefinitionOfDone` | the brief key on a land, evidence recording, the DoD above the queue | lands record no brief (`none`); evidence recording answers `brief_unresolvable`; the Merge lens renders the DoD slot's honest empty state |
| G4 (soft) | The Hub's credential store (`credstore.Store`) | push and PR with a stored token | push uses the machine's git credential helper; PR creation uses the git-helper fallback (task 13) or says there is no credential |

### What phase 3 asks of the Hub (Phase H) — no Hub contract existed on 2026-09-23

`…/worktrees/hub/docs/superpowers/contracts/` held only the two phase 2 contracts when this plan
was written, so phase 3 **defines the minimal interfaces it needs** and marks each as a Hub ask.
The Hub's contract replaces them by satisfying them; if it cannot, it names the difference and
phase 3 adapts in one file.

| Ask | Interface (complete source in the named part) | Why the Hub |
|---|---|---|
| **H-1** Credential store | `credstore.Store` — `Credential(ctx, credstore.Host) (credstore.Credential, error)`, `ErrNoCredential` (task 13) | OD-6: Windows Credential Manager, the user's own tokens, one store for ADO and GitHub. Phase 7's token joins it |
| **H-2** Evidence store | `evidence.Store` — `Record`, `ByDigest`, `Shared()` (task 15) | OD-3: approvals-like judgements are shared from day one. Phase 3 ships a local file store with `Shared() == false`, and the UI says "recorded on this machine" until the Hub's arrives |
| **H-3** A stable repository identity | `evidence.Record.Repo` is `githost.RepoKey` (`github.com/<owner>/<repo>` or `<org>.visualstudio.com/<project>/<repo>`), never the local `p-…` id, which is a hash of a path on one laptop (task 15) | the Hub keys shared records by repository, and two teammates' project ids never match |
| **H-4** Who is recording | `evidence.Record.RecordedBy` is the git identity today; the Hub's signed-in principal replaces it | OD-6 |

### What phase 3 asks of phase 4 (Discover and `provenance`)

| Ask | Detail |
|---|---|
| **P4-1** Satisfy `briefref.Resolver` (task 09) | `Resolve(ctx, key) (briefref.Revision, bool, error)` with the revision's digest and each criterion's id **and text digest** |
| **P4-2** Add `digest` to each criterion on `provenance.Ref` | the brief's §4.2 amendment and risk R14; phase 4 owns the `Ref` schema. Phase 3's desktop reads it and never computes it |
| **P4-3** Satisfy `briefref.Archiver` (task 11) | `ArchiveHead(ctx, projectID, branchID) error`, called when a product branch is deleted (P13's "may archive"; revisions untouched) |
| **P4-4** Call `evidence.Evaluator` for a subject | the `Ref` served for `DefinitionOfDone` evaluates criterion states at the branch head through phase 3's evaluator (contract §2.3). Phase 4 returns `open` until it does |
| **P4-5** An optional `onRecordEvidence?(criterionId)` prop on `DefinitionOfDone` | lets the Merge lens open its evidence dialog from the criterion row. Until it exists the dialog is reachable from the DoD slot's own control (task 20) |
| **P4-6** Where the managed `CLAUDE.md` block goes in a clone | 3-D6 says never committed. Each clone has its own `.git/info/exclude`; phase 4's Materialiser writes the exclude line there on first materialise. A repository that already tracks `CLAUDE.md` still needs phase 4's other location |
| **P4-7** Serialise the `sessions.go` edits | phase 4's Materialiser call (R1), the resume fix's minted id, and phase 3's environment addition (task 05) all sit in `handleSessionCreate`/`handleSessionRestart`. Task 05's edit is inside `sessionEnv` and `EnvEntries`, not in the handlers' step order, so it does not contend with R1's call site |

### Seams phase 3 leaves for later phases

| Seam | Where | The later phase | Must not |
|---|---|---|---|
| **S3-P8.1** `gitx.Machine` | `internal/gitx/machine.go` | phase 8 adds a Kata machine: a pod gets a clone, `FetchURL` names it, the control plane fetches from it | run git for a cloud machine through `gitx.Local` |
| **S3-P8.2** `lanes.Store` location | `PANACEA_APP_WORKTREES_DIR` | phase 8 puts cloud clones on the pod's volume, behind S3-P8.1 | move local clones under the projects root (the scanner would list each one as a project) |
| **S3-P7.1** Session trailers in a shipped range | `git log --format=%(trailers:key=Panacea-Session)` | phase 7's manifest lists the sessions that produced a release (`7-D3`) | infer a session from a commit's time |
| **S3-P7.2** Merge-link trailers | `Panacea-Brief`/`Panacea-Brief-Digest` on first-parent merges | phase 7 shows "reviewed against rev N at land" | parse the key (P5) |
| **S3-P7.3** Ahead/behind | the convergence read model's `remote` block (task 08) | phase 7b's "2 local commits are not pushed and will not ship" | fetch on its own poll |
| **S3-P7.4** Push | `POST …/push` (task 12) | phase 7b's Ship waits on it (`dependency-graph-and-order.md:275`) | push from a sandbox |
| **S3-P7.5** The evaluator at a commit | `evidence.Evaluator` (task 15) | phase 7 evaluates at `Deployment.Commit` | evaluate a release at the branch head (P12) |
| **S3-P5.1** A worktree created from a design | `POST …/worktrees` | phase 5 creates a worktree like any other and materialises its handoff bundle into it, excluded (3-D6) | commit the bundle |
| **S3-Hx.1** More harness adapters | `internal/transcript/<harness>` | one harness at a time (3-D7 c) | make any feature require an adapter |

## Open questions — recorded, with the recommendation this plan builds on

None blocks a task. Each is built on the recommendation; the owner overrules by saying so, and the
named task changes.

| # | Question | Recommendation, built | Changes if overruled |
|---|---|---|---|
| **Q1** | PR creation before the Hub exists: may the control plane ask the machine's git credential helper for the user's token (`git credential fill`, non-interactive) as a fallback? | **Yes, as a fallback only.** It is the user's own credential for the user's own act, the helper already holds it for `git push`, and it makes PR creation provable on a laptop now. The Hub's store wins whenever it has a token | task 13 drops `credstore.GitHelper`; PR creation waits for H-1 |
| **Q2** | Evidence before the Hub exists: record locally, labelled, or not at all? | **Locally, labelled "recorded on this machine only"**, behind `evidence.Store`, replaced by the Hub's implementation (H-2). No migration is built: local records are a laptop's draft | task 15 ships the store without a local implementation and evidence waits for the Hub |
| **Q3** | Is refusing to land into a checked-out branch acceptable when the user's primary worktree has that branch checked out, which is common? | **Yes** (brief A5). The banner names the checkout and the two ways out | task 09 would need a real `git merge` through that checkout, which touches the user's files |
| **Q4** | Rename a product branch that has cockpit worktrees? | **Refused in v1** with the sentence "Remove or land this branch's worktrees first". Renaming a checked-out branch under a running agent is a change nobody asked that agent for | task 11 renames in every clone too |
| **Q5** | Contract Q6 / risk R6: phase 3 adds push behind a loopback boundary with no authentication | **Accept for now**, as the brief does; the Hub owns control-plane authentication (OD-6). Push only ever uses the signed-in user's own credential, so another local process could push *as the user*, never as anyone else | the Hub's authentication lands before task 12 |
| **Q6** | Review C1's residual: phase 2's scanner and `project.ExecRunner` run host git in the primary worktree's repository, whose `.git` the sandbox can write on a primary-worktree session. task-02 §HGH hardens `gitx.Exec` only. Are phase 2's read-only commands (`worktree list`, `for-each-ref`, `status`) exposed to `core.fsmonitor`, hooks or filters? | **Phase 2 checks, and adopts §HGH's H1 prefix in `ExecRunner` if they are.** `status` honours `core.fsmonitor`, so that exposure is likely. It is **not verified here**, and phase 2's files are not this plan's to edit. H5's read-only `.git/config` mount covers that one file only: a `.git/commondir` the agent writes moves the config git reads (verified on git 2.53.0.windows.2, task-02 §HGH H5). So phase 2's launcher needs H5's layout pin (`GIT_DIR`, `GIT_COMMON_DIR`) as well as the H1 prefix | phase 3 lifts H1 into `project.ExecRunner` itself (one prefix, both launchers) |
| **Q7** | task-02 §HGH H5 makes a checkout's `.git/config` read-only inside the sandbox. An agent's `git config`, `git branch --set-upstream-to` and `git remote add` then fail there. So, by the same reasoning and not verified, do an agent's `git branch -m` (it moves `branch.<old>.*`) and husky's `prepare` script, which runs `git config core.hooksPath` when the agent runs `npm install` (task 05's `TestSandboxCannotWriteHooksPath` pins that write failing; so task 03's Create copies a relative `core.hooksPath` and the generated, ignored directory it names at create time, the directory only while no primary-worktree session is live or starting, review I11 and re-review b03 I-2). Accept that? | **Yes.** It is the only rule that closes H3's check-then-call race. The sandbox holds no git credential (A4), so it never needs a remote, and identity arrives through the session environment. The agent's `git commit`, `branch` and `checkout -b` keep working | H5 is dropped, and H3 becomes a best-effort check with a documented race. Filter drivers in a clone are then a residual host-execution risk |
| **Q8** | task-02 §HGH H1 clears a repository's own credential helper, so a cockpit fetch or push authenticates as the account the machine's helpers (system, then global) answer for that URL, not the one a repo-local `credential.<url>.helper` chooses. On this repository that is the global helper's account, not the repo-local `lcfrai/panaceaapp` one. Accept that? | **Yes.** A repository helper is a program the sandbox may have written, and the machine's config is the only config the agent cannot write. A user who wants the repo-local account moves that key to `--global`; it is URL-scoped, so it still applies to that repository alone | task 12 lets a push run the repository's own helper when no session has ever mounted that repository's `.git`, which needs a per-repository record of mounts |
| **Q9** | OD-2 says a lane clone's objects are hard-linked. A hard link is one file with two names, so through the bind mount an agent can clear a pack's read-only attribute and rewrite the **project's** pack, which makes OD-2's "blast radius is its own clone" false (review I8; verified on git 2.53.0.windows.2: `clone --local` gives the pack, `.idx`, `.rev` and commit-graph a link count of 2). Copy the objects instead? | **Yes: `clone --no-hardlinks`, built** (task 03 step 4, one function `cloneArgv`). The cost is disk and clone time per lane. Task 01's Step 5 measures both on `~/Projects/Panacea`; **not measured yet**. Recommend keeping the copy unless that clone is too slow on the owner's laptop | task 03 goes back to `clone --local`, and **no check closes the hole**. The files an agent rewrites are the project's from the moment it writes them, so a check at land time leaves the user's repository exposed between lands; and `git fsck --connectivity-only` cannot see the damage at all (git-fsck(1), git 2.53: "Corruption in blob objects will not be detected at all"). The nearest mitigation is a full `git fsck` of the project before each land (task 09), which checks every object's hash and is slow on a large repository. This plan's "does not claim" then says plainly: the user's object files are writable from the sandbox, and no check before a land closes that (plan re-review b02 M-1) |

## File map

New unless marked. Phase 2 files phase 3 edits are marked **(p2)**, and each part file names the
exact insertion.

```
control-plane/
  cmd/api/config.go                 (p2) three PANACEA_APP_* rows                          04 07 12
  cmd/api/main.go                   (p2) wire lanes, syncer, fetch timer, deps             04 07 12 13
  internal/gitx/                    runner.go error.go machine.go trailers.go harden.go (§HGH)
                                    gitxtest/repo.go  behaviour_test.go                     01 02
  internal/lanes/                   store.go clone.go catalog.go remove.go                  03 04 11
  internal/converge/                sync.go observe.go forecast.go land.go history.go
                                    undo.go branches.go                                     07-11
  internal/remote/                  fetch.go push.go aheadbehind.go                          08 12 13
  internal/credstore/               credstore.go githelper.go chain.go                       13
  internal/githost/                 remote.go header.go                                      13
                                    githost.go github.go azuredevops.go                      14
  internal/briefref/                briefref.go                                              09
  internal/evidence/                record.go store.go filestore.go evaluate.go              15
  internal/changes/                 changes.go diff.go blame.go                              16
  internal/transcript/claudecode/   adapter.go match.go testdata/                            17
  internal/project/project.go       (p2) Worktree.Kind; parseWorktrees sets it — no git      03
  internal/gitx/ident.go            ParseIdent, the one parser of git's ident line           03
  internal/statepath/               (p2) atomic.go, if R2's lift has not happened            03
  internal/backend/backend.go       (p2) EnvSessionID, written by EnvEntries                 05
  internal/backend/katabe/kata_test.go (p2) the env-parity pin gains one variable            05
  internal/api/api.go               (p2) two lines: Deps.Git, a.registerGit                   04
  internal/api/gitdeps.go           GitDeps (one block per task), reasons, problem helpers   04 onward
  internal/api/gitroutes.go         phase 3's route table (one block per task)               04 onward
  internal/httpx/problem.go         (p2) CodeGitFailed, CodeHostFailed                        04
  internal/api/sessions.go          (p2, post-fix) create and restart admit through
                                    MountGuard (re-review b03 I-2)                           04
                                    sessionEnv gains core.hooksPath;
                                    sessionMounts binds .git/config read-only (§HGH H5)       05
  internal/api/mounts.go            MountGuard: lane hooks copy vs sandbox start             04
  internal/api/catalog.go           (p2) worktreeJSON.Kind                                   04
  internal/api/routes_audit_test.go (p2) one line per new GET                                08 10 11 15 16 17
  internal/api/{worktrees,convergence,lands,branches,remote,pullrequests,
                evidence,changes,transcript}.go  + tests                                     04-17
images/sandbox/
  Dockerfile                        (p2) COPY githooks, chmod                                05
  githooks/dispatch                 the one script; githooks/<hook-name> → dispatch          05
docs/
  verification/2026-09-2x-phase-3-git-behaviours.md                                          01
  superpowers/contracts/2026-09-18-phase-2-go-contract.md  (p2) §3.7 kind; §4 three rows;
                                    §5.1 env; problem codes                                   04 05 06 12 14
  superpowers/contracts/2026-09-18-phase-2-ui-contract.md  (p2) §2 Worktree.kind; §4.2 codes  06 18
desktop/src/
  store/types.ts                    (p2) Worktree.kind                                       06
  components/develop/LinkedWorktreeNote.tsx (+test) (fix) narrowed, reworded, + shared-index  06
  api/client.ts                     (p2) append phase 3's functions and ProblemCodes          18
  api/convergence.ts                the phase 3 wire types                                    18
  store/slices/convergenceSlice.ts  unpersisted                                               18
  store/store.ts                    (p2) compose the slice; partialize UNCHANGED              18
  store/selectors.ts                (p2) append                                               18
  api/poller.ts                     (p2) one call on the existing tick                        18
  components/lens/MergeLens.tsx     (task 26's stub) filled                                   19 20
  components/lens/merge/*           LaneCard, QueueList, LandedList, RemoteBar,
                                    ResolveDialog, EvidenceDialog, merge.css                  19 20
  components/lens/FilesLens.tsx     (task 26's stub) filled                                   21
  components/lens/files/*           PathList, PathDiff, SpanDrawer, files.css                 21
  components/menu/ContextMenu.tsx   (p2) spread phase 3's items; project "New branch" enabled 22
  components/menu/lifecycleItems.ts the worktree and branch verbs                             22
  components/lifecycle/*            forms, the remove question, actions.ts                    22
  components/develop/EmptyDevelop.tsx (p2) "arrives in Phase 3" → a New worktree button       22
  components/develop/SessionPane.tsx  (p2) SharedIndexNote mount (06), its onNewWorktree (22)  06 22
  store/slices/{prefs,ui}Slice.ts   (p2) rebindBranch — keys moved, no persist version        22
  components/palette/commands.ts    (p2) append                                               22
  components/topbar/TopBar.tsx      (p2) line 29's title loses "Phase 3"                      22
  styles/tokens.css                 (p2) --diff-add, --diff-del (check first, rule 2)          21
```

## Task list

Local-only first: nothing before task 13 needs a credential, a network or the Hub. Tasks 01–17
are the Go track; 18–22 the desktop track, which starts when G2 holds and task 18's types match
the Go parts (it codes against the part files' shapes and fakes, not a running server). Each task
is sized for one implementer in one to three hours.

| # | Task | Depends on | Est. |
|---|---|---|---|
| 01 | Verify the git and sandbox behaviours the plan rests on; measure clones on this laptop (R12) | — | 2 h |
| 02 | `gitx`: the write runner, timeout classes, `*gitx.Error`, the `Machine` seam, trailers, a real-git test helper | 01 | 2 h |
| 03 | `lanes`: the lane index, clone create, identity copy, the composite catalog | 02 | 3 h |
| 04 | Create-worktree route, `kind` on the Go wire, config and wiring | 03, G1 | 2 h |
| 05 | Git in the sandbox for clones: `core.hooksPath`, `PANACEA_APP_SESSION_ID`, the hook dispatcher | 02, G1; 04 for its lane tests and Docker step (the order below runs it after 04 anyway) | 3 h |
| 06 | The linked-worktree notice superseded: `kind` in `types.ts`, the notice narrowed, the shared-index note (3-D2) | 04, G1 | 2 h |
| 07 | Lane sync and observed claims | 04 | 3 h |
| 08 | The chained forecast, queue order, remote reading, and the convergence GET | 07 | 3 h |
| 09 | Land, with the merge link, `briefref`, and the clones' origin refresh | 08 | 3 h |
| 10 | Landed history, exact undo, revert | 09 | 2 h |
| 11 | Remove a worktree; create, rename and delete a branch | 07 | 3 h |
| 12 | Fetch (on demand and on a slow timer) and push, with the machine's credential helper | 02, 04 | 2 h |
| 13 | The credential-store seam (H-1), the git-helper fallback, push with a stored token | 12 | 2 h |
| 14 | `githost`: GitHub and Azure DevOps behind one interface; the PR route | 10, 13 | 3 h |
| 15 | Evidence: records, the store seam (H-2), the evaluator over ancestry | 07, 09 | 3 h |
| 16 | Files read model: changed paths across lanes, per-path hunks, tier-1 back-links | 08 | 3 h |
| 17 | The OD-8 transcript adapter: pinned, canary-tested, tier 2, degrading | 16 | 3 h |
| 18 | Desktop: phase 3 wire types, client functions, the convergence slice on the poll tick | G2; parts 04, 08–17 | 3 h |
| 19 | Merge lens, part 1: the queue, forecasts, land, conflict resolution, banners | 18 | 3 h |
| 20 | Merge lens, part 2: remote bar, push, PR, landed history with undo, evidence dialog | 19 | 3 h |
| 21 | Files lens: changed paths, one path's hunks, back-links, the span drawer | 18 | 3 h |
| 22 | Lifecycle verbs: new and remove worktree, branch create/rename/delete, palette; the three "arrives in Phase 3" promises retired | 04, 06, 11, 18 | 3 h |
| 23 | Proof: the end-to-end script on real git and Docker, the owner's batched checklist, the seams doc | all | 2 h |

**Order.** 01 → 02 → 03 → 04 → then three lanes that can run in parallel worktrees:
**(a)** 05 → 06; **(b)** 07 → 08 → 09 → 10, then 15 and 16 → 17; **(c)** 11 (after 07), 12 → 13,
then 14 once 10 has landed (its PR description lists the lands). The desktop track 18 → (19 → 20) ∥ 21 ∥ 22 starts once G2 holds. Task 23 is last.

**Shared-file hazards inside phase 3.** Task 04 adds exactly two lines to `api/api.go`
(`Deps.Git *GitDeps` and `a.registerGit(guarded)`); after that, phase 3's dependencies live in
`api/gitdeps.go` and its routes in `api/gitroutes.go`, both phase 3's own files. Each later task
appends a **contiguous, commented block** (`// ── task NN ──`) to `GitDeps`, to `gitRoutes()`
and to `serve`'s phase 3 wiring in `cmd/api/main.go`, never between another task's lines, so
parallel lanes merge mechanically (`dependency-graph-and-order.md` §4.2's rule for `api.go`,
made structural). `routes_audit_test.go` gets one line per GET, appended.

## What this plan does not claim

- **Nothing here has been run.** The plan was written from reading the tree at `3303fd6`, the fix
  branch at `2d6d95b` and its then-uncommitted notice, and one read-only look at a transcript
  file. The fix's citations were re-read on 2026-09-24 at `4f6d698` and `10fa5fd` (review I12);
  `feat/phase-2-cockpit` is still moving, so G1's preflight re-reads them again.
- **Verified while writing**: the transcript line shape on the host's Claude Code **2.1.267**
  (`version`, `timestamp`, `uuid`; `tool_use` with `input.file_path`, `old_string`, `new_string`
  or `content`; the result line's `toolUseResult.structuredPatch` with `oldStart`, `newStart`,
  `lines`). **Not verified on 2.1.274**, the version the image pins: task 17's canary is what
  checks it. Also verified: the remote URL forms of this programme's own repositories
  (`https://github.com/<owner>/<repo>`, `https://kpmgcomau.visualstudio.com/<project>/_git/<repo>`),
  and that no Hub contract exists yet.
- **Verified in fix batch b02** (git 2.53.0.windows.2, 2026-09-24, host only, with a test system
  file holding `core.autocrlf=true`): a plain clone checks LF blobs out CRLF and writes no local
  `core.autocrlf`; task 03's argv checks them out LF, writes its four keys and leaves `status`
  clean; `clone --local` hard-links the pack files and `--no-hardlinks` does not. Round 2 (same
  git, host only; `scratchpad/p3/b02r2-probe.out`): the production resolver's two `git config`
  calls answer `false`/`lf` in such a clone and `true`/`native` in its project, a checkout under
  the clone's answer as `GIT_CONFIG_*` writes LF, and one under the project's writes CRLF while
  `status` stays empty (the re-review's probe). **Not verified**: the sandbox's Linux git on such
  a clone under the real session env (task 01's and task 05's Docker tests), whether
  `checkStat=minimal` and `trustctime=false` stop the cross-system re-hash, and what
  `--no-hardlinks` costs on `~/Projects/Panacea` (task 01 Step 5, Q9).
- **Every git behaviour the brief took from documentation** is task 01's to verify before any
  task relies on it, and task 01's results document replaces this plan's assumptions wherever they
  differ.
- The Merge and Files lenses' look is **unverified** until the owner sees them (OD-12).

