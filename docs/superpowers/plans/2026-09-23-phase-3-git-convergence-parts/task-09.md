### Task 09: Land — plumbing with compare-and-swap, the merge link as trailers, and every clone told

The one write that converges. A land takes one ready worktree and makes it part of its product
branch **without touching any checkout or index** (brief A5): `merge-tree --write-tree` for the
tree, `commit-tree` for a two-parent merge commit whose message carries the merge link (3-D4, 3-D6),
and `update-ref <branch> <new> <old>`, which git refuses if the branch moved since the land read
it (risk R13). The product branch only ever receives clean merges (A2): a conflict is refused and
resolved in the lane (3-D3, task 19's one click).

The merge link is the brief key the human **saw** above the queue, carried in the request and
verified by phase 4's resolver before anything is written (survey §7.2). It is copied into the
commit byte for byte and never parsed (contract P5). Without a resolver a land records no brief
(plan decision 4) and renders `none`.

**Files:**
- `control-plane/internal/converge/land.go` + `land_test.go` — `Land`, `LandRequest`, `LandResult`, refusals (new)
- `control-plane/internal/api/lands.go` + test — `POST …/lands` (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go` — `// ── task 09 ──` blocks
- `control-plane/cmd/api/main.go` — wire `Lander`; task 04's `TestServeWiresGit`, in the test file task 04 put it in, gains one assertion (re-review b04-1 issue 3)

#### Interfaces — complete

`briefref.Resolver` and `briefref.Revision` are task 03's (review I7 moved the package there, so
task 11 can import it in a parallel lane). This task consumes them and declares nothing in it.

```go
package converge

// LandRequest is one land.
type LandRequest struct {
	ProjectID   string
	ProjectPath string
	BranchID    string
	Branch      string
	WorktreeID  string
	ExpectedTip string // the Convergence.Tip the human saw; required
	BriefKey    string // "" or the key the DefinitionOfDone above the queue resolved
	BriefDigest string // required with BriefKey, absent without it
}

// LandResult is a land that happened.
type LandResult struct {
	Merge         string            `json:"merge"`  // the new tip
	Before        string            `json:"before"` // the tip it replaced (ExpectedTip)
	Branch        string            `json:"branch"`
	WorktreeID    string            `json:"worktreeId"`
	BriefKey      string            `json:"briefKey"` // "" when none was recorded
	ReadyCleared  bool              `json:"readyCleared"`
	Refreshed     []string          `json:"refreshed"`     // clones whose origin/<branch> now names Merge
	RefreshErrors map[string]string `json:"refreshErrors"` // worktree id → git's sentence; non-fatal
}

// Refusal is a land that did not happen and wrote nothing. Reason is one of gitdeps.go's
// constants; Details goes to the problem body's details beside it.
type Refusal struct {
	Reason  string
	Message string
	Details map[string]string
}

func (r *Refusal) Error() string { return r.Message }

// Lander lands. One per process. Land, Undo and Revert (task 10) each hold
// Locks.Lock(ctx, req.ProjectID) for the whole verb, so they serialise per project with each
// other and with task 11's branch verbs and task 12's Fetch and Push (review I7: one scope).
type Lander struct {
	Locks    *gitx.Locks // GitDeps.Locks, task 04's one value; nil is a wiring error, refused
	Machine  gitx.Machine
	Syncer   *Syncer
	Lanes    lanes.Store
	Flags    registry.WorktreeFlagStore
	Resolver briefref.Resolver // nil until phase 4 provides one
	// Identity is api.Deps.GitIdentity (the sandbox fix's resolver), asked for the PROJECT's
	// path: the land is the user's act, committed as the user.
	Identity func(ctx context.Context, path string) (gitx.Ident, error)
}

// Land, in this order. Every step before 10 writes nothing; a *Refusal from any of them
// leaves the repository exactly as it was.
//  0. unlock, err := Locks.Lock(ctx, req.ProjectID); defer unlock()  (ctx ended → its error)
//  1. the lane belongs to this project and branch                 (else bad_request)
//  2. it is marked ready                                          (not_ready)
//  3. CheckedOutAt(branch) == ""                                  (branch_checked_out, details.path)
//  4. Syncer.SyncLane                                             (git_failed)
//  5. BriefKey: Resolver nil → brief_unresolvable/no_resolver; !ok → /missing;
//     rev.Digest != BriefDigest → /digest_mismatch
//  6. old = rev-parse refs/heads/<branch>; old != ExpectedTip     (branch_moved, details.tip = old)
//  7. laneTip = rev-parse LaneRef; laneTip an ancestor of old     (nothing_to_land)
//  8. Identity(ProjectPath)                                       (no_git_identity)
//  9. merge-tree --write-tree --name-only old laneTip, OKExit 1   (merge_conflict, details.paths)
// 10. msg = gitx.Message("Land <name> into <branch>",
//                        "From the cockpit worktree <name> (<worktree id>).",
//                        [Panacea-Land: <id>] + [Panacea-Brief, Panacea-Brief-Digest] when a key)
//     — a value Message refuses is bad_request, still before any write
// 11. merge = commit-tree <tree> -p old -p laneTip -F -   (stdin msg; identity in GIT_AUTHOR_*/COMMITTER_*)
// 12. update-ref -m "panacea: land <worktree id>" refs/heads/<branch> merge old
//     — refused: re-read the tip; moved → branch_moved (details.tip, details.gitSays); else git_failed
// 13. Flags.SetWorktreeReady(id, false); a failure is logged and ReadyCleared = false
// 14. for every lane of the branch: in its clone,
//     fetch --no-tags --quiet <ProjectPath> +refs/heads/<branch>:refs/remotes/origin/<branch>  (Local)
//     — <ProjectPath> is the literal path from the lane index, never the clone's configured
//     origin (task-02 §HGH H2; review C3). Each failure goes to RefreshErrors, never fails the
//     land (plan decision 3)
// Hardened per task-02 §HGH: steps 6-12 run with Scope Project (a refusal there is
// project_config_untrusted, before any ref moves), step 14 with Scope Clone, and no hook runs (H4).
func (l *Lander) Land(ctx context.Context, req LandRequest) (LandResult, error)
```

```go
// gitdeps.go
	// ── task 09 ──
	Lander   *converge.Lander
	Resolver briefref.Resolver // nil until phase 4; Lander holds the same value
```

**Wire: `POST /v1/projects/{projectId}/branches/{branchId}/lands`**

```json
// request
{ "worktreeId": "w-4a77b1c9", "expectedTip": "a90d3f4c…",
  "briefKey": "brief:b-42@6", "briefDigest": "sha256:…" }       // the last two: both or neither

// 201 Created
{ "merge": "d5f0a17…", "before": "a90d3f4c…", "branch": "evidence-upload", "worktreeId": "w-4a77b1c9",
  "briefKey": "brief:b-42@6", "readyCleared": true,
  "refreshed": ["w-4a77b1c9", "w-5e6f7a8b"], "refreshErrors": {} }
```

| Refusal | Status, code | `details` |
|---|---|---|
| missing `worktreeId` or `expectedTip`; one of key/digest without the other; a key or digest `gitx.Message` refuses | 400 `bad_request` | — |
| worktree not a cockpit worktree of this branch | 400 `bad_request` | — |
| unknown project | 404 `project_not_found` | `projectId` |
| every `*Refusal` | 409 `conflict` | `reason` plus the table in `Land`'s comment |
| sync, merge-tree, commit-tree or a non-race update-ref failure | 500 `git_failed` | `gitSays` |

"Land all in order" is the desktop calling this once per queued worktree, top down, passing the
previous response's `merge` as the next `expectedTip`, and stopping at the first refusal. There is
no batch route: every land is its own compare-and-swap, which is what makes a partial run honest.

#### The tests to write first

Real repositories: a project, two clones made by `lanes.Creator`, agent commits made in the clones,
a `Syncer` over them, a fake `WorktreeFlagStore`, and a fake `Resolver`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLandMakesATwoParentMergeWithTheLink` | the new tip has parents `[old, laneTip]`, its tree is merge-tree's, and `%(trailers)` reads `Panacea-Land: <id>`, `Panacea-Brief: brief:b-42@6`, `Panacea-Brief-Digest: sha256:x` | a squash or a fast-forward (3-D4) |
| `TestLandAlwaysMakesAMergeEvenWhenFastForwardable` | the lane already contains the tip → still two parents | a fast-forward that loses the land's record |
| `TestLandLosesARaceCleanly` | a Runner wrapper commits to the branch between steps 11 and 12 → `branch_moved`, the branch is at the injected commit, not the merge | `update-ref` without its old value |
| `TestLandRefusesAStaleExpectedTip` | `ExpectedTip` ≠ tip → `branch_moved` with `details.tip`, nothing written (loose-object count unchanged) | landing against a view the human did not see |
| `TestLandRefusesACheckedOutBranch` | the project's primary has the branch checked out → `branch_checked_out`, `details.path` is its path | moving a ref under a user's checkout |
| `TestLandConflictWritesNothing` | both lanes edit a line; land the first, then the second → `merge_conflict`, `details.paths`, tip unchanged | a conflicted product branch (A2) |
| `TestLandRefusesNotReady` | ready false → `not_ready` | landing work nobody marked ready (defect 9) |
| `TestLandRefusesAnEmptyLane` | no commits → `nothing_to_land` | an empty merge commit |
| `TestLandSyncsTheLaneFirst` | commit in the clone with no sweep since → the merge's second parent is that commit | landing a stale lane ref |
| `TestLandWithoutAResolverRefusesAKey` | `Resolver == nil`, key set → `brief_unresolvable`/`no_resolver` | an unverified key in history (decision 4) |
| `TestLandWithoutAKeyWritesNoBriefTrailer` | no key → `Panacea-Land` only | an empty `Panacea-Brief:` trailer |
| `TestLandRefusesAMissingOrMismatchedRevision` | resolver `ok == false` → `/missing`; wrong digest → `/digest_mismatch` | recording a key that names different text |
| `TestLandCopiesTheKeyVerbatim` | key `brief:b-42@6` → exactly that string in the trailer | normalising a key (P5) |
| `TestLandRefusesAnInjectedKey` | key containing `\nPanacea-Land: w-x` → 400 before any git write | a forged second trailer |
| `TestLandCommitsAsTheUser` | author and committer of the merge equal `Identity`'s answer; no identity → `no_git_identity`, nothing written | a merge by `dev@container` or by nobody |
| `TestLandClearsReady` | after the land, the flag store holds `ready:false` for the worktree | a landed lane sitting in the queue (decision 2) |
| `TestLandTellsEveryClone` | after landing clone A, clone B's `refs/remotes/origin/<b>` equals the merge | 3-D3's prompt naming a stale `origin/<b>` |
| `TestLandRefreshFailureDoesNotFailTheLand` | clone B's directory removed → 201, `refreshErrors[B]` holds git's sentence | failing a land that happened |
| `TestLandSerialisesPerBranch` | two concurrent lands of different lanes on one branch under `-race` → one succeeds, the other gets `branch_moved` or lands after it; never both on the same `old` | a double land on one tip |
| `TestLandWaitsOnTheProjectLock` (review I7) | the test holds `Locks.Lock(ctx, p)`: a land on branch `a` and one on branch `b` of `p` have run no git call 50 ms later (a recording Runner); release → both land. Through the handler: holding `Deps.Git.Locks` for `p` keeps `POST …/lands` unanswered until release. A `nil` `Locks` → 500 `internal`, nothing run | a per-branch lock task 12's push does not share; a Lander given a lock of its own |
| `TestServeWiresGit` (cmd/api; task 04's test, extended; re-review b04-1 issue 3) | red first against a `main.go` that gives the `Lander` `&gitx.Locks{}`: `Git.Lander.Locks` is the same pointer as `Git.Locks` | a Lander given a lock of its own, which every handler test (each builds its own Deps) would miss |
| `TestLandRouteMapsEveryRefusal` | table test over the handler: each `*Refusal` → 409 with its reason; each `*gitx.Error` → `git_failed`; a `*gitx.UntrustedConfig` → 409 `project_config_untrusted` with `details.key` | a refusal that surfaces as 500 |
| `TestLandRunsNoProjectHook` | task-02 §HGH, red first against an unhardened `Exec`: the project's `.git/hooks/reference-transaction` writes a marker → a land succeeds and no marker exists | a land that honours the repository's hooks |
| `TestLandRefreshNeverUsesTheClonesOrigin` | task-02 §HGH H2 and review C3, red first. (a) Clone B's config sets only `remote.origin.url` (an allowlisted key) to an `httptest` smart-HTTP server → after a land, B's `refs/remotes/origin/<b>` equals the merge and the server received no request. (b) B's config also sets `http.proxy` to a second `httptest` server, `http.sslVerify=false` and `remote.origin.uploadpack=<marker script>` → neither server received a request, no marker exists, and `refreshErrors[B]` names `clone_config_untrusted` | a refresh that fetches `origin` |

#### Steps

- [ ] **Step 1: Preflight.** Task 08 merged; `briefref` (task 03) and `gitx.Locks` (task 02) and
  `GitDeps.Locks` (task 04) exist. Task 01's `TestBehaviourUpdateRefCompareAndSwapRefuses`
  recorded the refusal's stderr; the race test asserts on the **re-read tip**, not on that text.
- [ ] **Step 2:** the tests; `land.go`; the handler, routes and deps block.
- [ ] **Step 3:** wire `Lander` in `main.go` with `Locks: gitDeps.Locks`, `Identity` adapting
  `api.Deps.GitIdentity` (the fix's `func(ctx, path) (api.GitIdentity, error)`) to `gitx.Ident`,
  and `Resolver: nil`.
- [ ] **Step 4:** run and commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/converge/ ./control-plane/internal/api/ ./control-plane/cmd/api/ -race -count=1 → PASS
git log --first-parent -1 --format="%P%n%(trailers)" refs/heads/<b>   (manual, after a real land) → two parents; Panacea-Land
```

#### What this task does not claim

- Brief keys reach git only once phase 4 provides a `briefref.Resolver` (gate G3). Until then every
  land renders `none` — correctly: no brief was shown.
- A land moves a **local** branch. Nothing is pushed (task 12), and the undo is exact until it is
  (task 10).
- The merge link is forgeable by anyone who can commit (survey §7.2); it is a record of what the
  cockpit did, not a signature.
