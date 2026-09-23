# Phase 3 — Git, worktrees, coordination and convergence — implementation plan

> **For agentic workers:** execute task by task, in the order of the task list at the end. Each
> task's full specification is its part file in
> `docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-NN.md`. The part file is
> the contract for that task; this file is the map. Steps use checkbox (`- [ ]`) syntax.

**Status: DRAFT, being written.** This line is removed when the task list is complete.

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
   credential.
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
 ┌───────────────────────────────┐   git clone --local (objects hard-linked)
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
| `internal/githost` | one host interface, GitHub and Azure DevOps implementations, remote-URL recognition | 14 |
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
| **OD-2** (= 3-D1 b) | A worktree is a local clone, objects hard-linked, work fetched back under `refs/panacea/lanes/<worktree-id>` | tasks 01, 03, 04, 07 |
| **OD-4** (= 3-D8) | Azure DevOps **and** GitHub, behind one host interface | task 14 |
| **OD-6** | Tokens live in Windows Credential Manager, the store the Hub builds; users are always themselves | task 13 (the seam) |
| **OD-8** (= 3-D7 b) | The Claude Code transcript may be read, read-only, through an adapter pinned to the image's CLI version, canary-tested, degrading to commit level | task 17 |
| **OD-10** | Removing a worktree deletes its conversation store | task 11 |
| **OD-3 / OD-5** | Shared stores live in the Hub. Phase 3's evidence store has the brief's shape and the Hub's location | task 15 (seam), open question Q2 |
| **OD-12** | Owner hands-on checks are batched at the end; until then every owner-observed item is **unverified** | task 23 |
| 3-D2 | Two agents in one worktree: allowed, and the worktree says so, with one click to start in a new worktree instead | tasks 06, 19 |
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
   makes 3-D3's opening prompt ("merge `origin/evidence-upload` into this branch") true.
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
8. **Two new problem codes, no new kind.** `git_failed` (a git process the control plane ran for
   a remote failed; `details.gitSays` is git's sentence) and `host_failed` (the GitHub or Azure
   DevOps API refused; `details.hostSays`). Both classify as `server-said` through `classify`'s
   existing catch-all. Every other refusal is `conflict` with a `details.reason` from a closed list
   (task 08's table).

## How this plan supersedes the sandbox git fix (`fix/sandbox-git-ca`)

The fix (`2d6d95b` on `fix/sandbox-git-ca`, based on `69bbc02`, plus the uncommitted
`LinkedWorktreeNote` in its worktree) is **the base this plan builds on**. Read on 2026-09-23 with
`git log 3303fd6..fix/sandbox-git-ca` and `git show fix/sandbox-git-ca:<path>`.

| The fix does | Phase 3 does | Kept, changed or superseded |
|---|---|---|
| Mounts the worktree at `/home/dev/work`, the conversation store, and the CA bundle read-only (`sessionMounts(worktreePath, conversationStore, caBundle)`) | Leaves all three mounts exactly as they are. For a cockpit worktree the worktree mount's host path is the clone, whose `.git` is a directory, so no `.git` common-dir mount is ever added (brief 3-D1 option a is rejected) | **Kept.** The mount model the brief calls "linked-worktree mounts" is replaced by clones, not by more mounts |
| `sessionEnv`: `GIT_CONFIG_COUNT=2` with `safe.directory=/home/dev/work` and `user.useConfigOnly=true`; `GIT_AUTHOR_*`/`GIT_COMMITTER_*` from the identity; `NODE_EXTRA_CA_CERTS` | Adds a third entry, `core.hooksPath=/opt/panacea/githooks`, so the count becomes 3. Adds `PANACEA_APP_SESSION_ID`, a reserved variable `EnvEntries` writes from `Spec.SessionID` (task 05) | **Extended.** `safe.directory` is still needed: a clone's bind mount has the same foreign owner |
| `gitIdentityResolver` asks `git var GIT_AUTHOR_IDENT` in the worktree's host path | A clone does not inherit its source's `.git/config`, nor an `includeIf gitdir:` that matched the project's path. So clone creation asks `git var` **in the project repository** and writes the answer into the clone's local `user.name`/`user.email` (task 03). The fix's per-worktree resolution then gives the same answer in the clone that it gives in the project | **Kept, with one addition at clone time** |
| `LinkedWorktreeNote` renders for every worktree with `isPrimary: false`, saying git "arrives in Phase 3" | The condition narrows to `kind === "linked"`: a worktree made with `git worktree add` outside the cockpit, where git is still unavailable. A clone is `kind: "clone"`, `isPrimary: false`, and renders no notice. The sentence is rewritten to name the fix the user can take, and stops promising a phase (task 06) | **Superseded** for clones; **kept and reworded** for linked worktrees |
| Git over HTTPS "is phase 3's" (the fix's commit message) | Push and fetch run on the host, never in the sandbox; the sandbox holds no credential (A4). `NODE_EXTRA_CA_CERTS` does not affect git on the host, which uses whatever trust the host's git is already configured with (its `http.sslBackend` — not read for this plan) | **Answered**: nothing git-over-HTTPS happens in a sandbox |

**Gate G1.** Task 04 and every later task assume the fix is merged into the branch phase 3 builds
on. If the fix is still unmerged when task 04 starts, the implementer stops and reports; phase 3
does not re-implement it. If the fix merges with a different shape from the one quoted above
(`sessionEnv`'s count, `GitIdentity`'s signature, `LinkedWorktreeNote`'s condition), tasks 04–06
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
  struct field (`Worktree.Kind`, task 04) that runs no git.
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
| `GET /v1/projects/{projectId}/branches/{branchId}/changes` | the Files read model | objects only (decision 6) | 16 |
| `GET /v1/projects/{projectId}/branches/{branchId}/changes/diff` | one path's hunks per worktree, with back-links | — | 16, 17 |
| `GET /v1/sessions/{id}/transcript/spans/{toolUseId}` | the tier-2 span: one tool call and the text before it | — | 17 |

## Seams

### Gates phase 3 waits on

| Gate | What | Blocks | If it has not landed |
|---|---|---|---|
| **G1** | `fix/sandbox-git-ca` merged into phase 3's base | tasks 04–06, and everything that starts a session | stop and report (above) |
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

## File map

New unless marked. Phase 2 files phase 3 edits are marked **(p2)**, and each part file names the
exact insertion.

```
control-plane/
  cmd/api/config.go                 (p2) three PANACEA_APP_* rows                          04 07 12
  cmd/api/main.go                   (p2) wire lanes, syncer, fetch timer, deps             04 07 12 13
  internal/gitx/                    runner.go error.go machine.go trailers.go
                                    gitxtest/repo.go  behaviour_test.go                     01 02
  internal/lanes/                   store.go clone.go catalog.go remove.go                  03 04 11
  internal/converge/                sync.go observe.go forecast.go land.go history.go
                                    undo.go branches.go                                     07-11
  internal/remote/                  fetch.go push.go aheadbehind.go                          08 12 13
  internal/credstore/               credstore.go githelper.go chain.go                       13
  internal/githost/                 githost.go remote.go github.go azuredevops.go            14
  internal/briefref/                briefref.go                                              09
  internal/evidence/                record.go store.go filestore.go evaluate.go              15
  internal/changes/                 changes.go diff.go blame.go                              16
  internal/transcript/claudecode/   adapter.go match.go testdata/                            17
  internal/project/project.go       (p2) Worktree.Kind; parseWorktrees sets it — no git      04
  internal/statepath/               (p2) atomic.go, if R2's lift has not happened            03
  internal/backend/backend.go       (p2) EnvSessionID, written by EnvEntries                 05
  internal/backend/katabe/kata_test.go (p2) the env-parity pin gains one variable            05
  internal/api/api.go               (p2) Deps: Lanes, Converge, Remote, Hosts, Evidence…     04 onward
  internal/api/sessions.go          (p2, post-fix) sessionEnv gains core.hooksPath           05
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
| 05 | Git in the sandbox for clones: `core.hooksPath`, `PANACEA_APP_SESSION_ID`, the hook dispatcher | 02, G1; 04 for its Docker step | 3 h |
| 06 | The linked-worktree notice superseded: `kind` in `types.ts`, the notice narrowed, the shared-index note (3-D2) | 04, G1 | 2 h |
| 07 | Lane sync and observed claims | 04 | 3 h |
| 08 | The chained forecast, queue order, remote reading, and the convergence GET | 07 | 3 h |
| 09 | Land, with the merge link, `briefref`, and the clones' origin refresh | 08 | 3 h |
| 10 | Landed history, exact undo, revert | 09 | 2 h |
| 11 | Remove a worktree; create, rename and delete a branch | 07 | 3 h |
| 12 | Fetch (on demand and on a slow timer) and push, with the machine's credential helper | 02, 04 | 2 h |
| 13 | The credential-store seam (H-1), the git-helper fallback, push with a stored token | 12 | 2 h |
| 14 | `githost`: GitHub and Azure DevOps behind one interface; the PR route | 13 | 3 h |
| 15 | Evidence: records, the store seam (H-2), the evaluator over ancestry | 07, 09 | 3 h |
| 16 | Files read model: changed paths across lanes, per-path hunks, tier-1 back-links | 08 | 3 h |
| 17 | The OD-8 transcript adapter: pinned, canary-tested, tier 2, degrading | 16 | 3 h |
| 18 | Desktop: phase 3 wire types, client functions, the convergence slice on the poll tick | G2; parts 04, 08–17 | 3 h |
| 19 | Merge lens, part 1: the queue, forecasts, land, conflict resolution, banners | 18 | 3 h |
| 20 | Merge lens, part 2: remote bar, push, PR, landed history with undo, evidence dialog | 19 | 3 h |
| 21 | Files lens: changed paths, one path's hunks, back-links, the span drawer | 18 | 3 h |
| 22 | Lifecycle verbs: new and remove worktree, branch create/rename/delete, palette, Authority's title | 18 | 2 h |
| 23 | Proof: the end-to-end script on real git and Docker, the owner's batched checklist, the seams doc | all | 2 h |

**Order.** 01 → 02 → 03 → 04 → then three lanes that can run in parallel worktrees:
**(a)** 05 → 06; **(b)** 07 → 08 → 09 → 10, then 15 and 16 → 17; **(c)** 11 (after 07), 12 → 13 →
14. The desktop track 18 → (19 → 20) ∥ 21 ∥ 22 starts once G2 holds. Task 23 is last.

**Shared-file hazards inside phase 3.** `api/api.go`'s `Deps` and `cmd/api/main.go` are touched by
tasks 04, 07, 08, 09, 12, 13, 14, 15, 16 and 17. Each task adds a **contiguous, commented block**
of its own (`// ── phase 3 task NN ──`) at the foot of `Deps` and at the foot of `serve`'s
wiring, never between another task's lines, so parallel lanes merge mechanically
(`dependency-graph-and-order.md` §4.2's rule for `api.go`). `routes_audit_test.go` gets one line
per GET, appended.

## What this plan does not claim

- **Nothing here has been run.** The plan was written from reading the tree at `3303fd6`, the fix
  branch at `2d6d95b` and its uncommitted notice, and one read-only look at a transcript file.
- **Verified while writing**: the transcript line shape on the host's Claude Code **2.1.267**
  (`version`, `timestamp`, `uuid`; `tool_use` with `input.file_path`, `old_string`, `new_string`
  or `content`; the result line's `toolUseResult.structuredPatch` with `oldStart`, `newStart`,
  `lines`). **Not verified on 2.1.274**, the version the image pins: task 17's canary is what
  checks it. Also verified: the remote URL forms of this programme's own repositories
  (`https://github.com/<owner>/<repo>`, `https://kpmgcomau.visualstudio.com/<project>/_git/<repo>`),
  and that no Hub contract exists yet.
- **Every git behaviour the brief took from documentation** is task 01's to verify before any
  task relies on it, and task 01's results document replaces this plan's assumptions wherever they
  differ.
- The Merge and Files lenses' look is **unverified** until the owner sees them (OD-12).

