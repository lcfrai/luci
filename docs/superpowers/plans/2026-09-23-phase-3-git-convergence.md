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

