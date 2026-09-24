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
- `control-plane/internal/converge/land.go` + `land_test.go` — `Land`, `LandRequest`, `LandResult`, refusals, `RefreshClones`, `RefreshProject`, `RefreshBranch` (review I4) (new)
- `control-plane/internal/converge/bringin.go` + `bringin_test.go` — `BringIn` (review I5) (new)
- `control-plane/internal/api/lands.go` + test — `POST …/lands` and `POST …/bring-in` (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go` — `// ── task 09 ──` blocks
- `control-plane/cmd/api/main.go` — wire `Lander`; task 04's `TestServeWiresGit`, in the test file task 04 put it in, gains one assertion (re-review b04-1 issue 3)

**Depends on:** task 08, and task 24 (`lanegit.Lanes` with `Live` and the `Dial` seam,
`Runner.Fetch`, `SendBranch`, `BundleRefused`, `lanegittest.Fake` and `lanegittest.Backend`):
RefreshClones sends a bundle, one per distinct told sha, from the project into each live
lane's runner (R4.5), and no host git runs in a lane (R4.2). `Lander.LaneGit` is set from
`GitDeps.LaneGit` where step 3 builds the `Lander`. No new module dependency: `sandboxd/vendor/`
is untouched.

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
	// BroughtBack is step 7a's answer: each undone land (task 10) this land put back on the
	// branch, because the lane had merged it (re-review b06-1 issue 1b). The desktop's toast
	// names them, and this land's own undo is exact, so the user can take it straight back.
	BroughtBack []UndoneLand `json:"broughtBack"`
}
// Every list and map in LandResult, BringInResult, UndoResult, RevertResult (task 10) and the
// refresh route's answer is
// written as [] or {} when empty, never null (re-review b06-1 issue 3): RefreshClones and
// CarriedUndone make both with make(), before any fetch or parse, and each handler test reads
// the raw body. Go's encoding/json writes a nil slice or map as null, and the desktop indexes
// into refreshErrors.

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
	LaneGit  *lanegit.Lanes    // task 24's pool, GitDeps.LaneGit: RefreshLanes' runner per lane (R4.3)
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
//  4. Syncer.SyncLane                                             (git_failed; a *BundleRefused
//     → bundle_refused, details.why)
//  5. BriefKey: Resolver nil → brief_unresolvable/no_resolver; !ok → /missing;
//     rev.Digest != BriefDigest → /digest_mismatch
//  6. old = rev-parse refs/heads/<branch>; old != ExpectedTip     (branch_moved, details.tip = old)
//  7. laneTip = rev-parse LaneRef; laneTip an ancestor of old     (nothing_to_land)
//  7a. BroughtBack = CarriedUndone(ProjectPath, laneTip, id) (task 08) (git_failed). Not a
//     refusal: the forecast row already named these lands, and the land that brings them back
//     is itself exactly undoable. Refusing until acknowledged is the owner's to choose (re-review
//     b06-1 issue 1b, plan Q11); this step is the seam.
//  8. Identity(ProjectPath)                                       (no_git_identity)
//  9. MergeTreeArgs + old laneTip, OKExit 1 → ParseMergeTree     (merge_conflict, details.paths =
//     detailList(conflicted); task 07, task 04; review I2 — never the newline form)
// 10. msg = gitx.Message("Land <name> into <branch>",
//                        "From the cockpit worktree <name> (<worktree id>).",
//                        [Panacea-Land: <id>] + [Panacea-Brief, Panacea-Brief-Digest] when a key)
//     — a value Message refuses is bad_request, still before any write
// 11. merge = commit-tree <tree> -p old -p laneTip -F -   (stdin msg; identity in GIT_AUTHOR_*/COMMITTER_*)
// 12. update-ref -m "panacea: land <worktree id>" refs/heads/<branch> merge old
//     — refused: re-read the tip; moved → branch_moved (details.tip, details.gitSays); else git_failed
// 13. Flags.SetWorktreeReady(id, false); a failure is logged and ReadyCleared = false
// 14. l.RefreshClones(ctx, ProjectPath, Branch) → Refreshed, RefreshErrors (below; both made
//     empty, never nil, re-review b06-1 issue 3). Each
//     failure goes to RefreshErrors, never fails the land (plan decision 3)
// Hardened per task-02 §HGH: steps 6-12 run with Scope Project (a refusal there is
// project_config_untrusted, before any ref moves). The lane is read by task 07's SyncLane
// (runner and bundle, step 4) and refreshed by RefreshClones (a bundle into each lane's runner,
// step 14); every host call is Scope Project, and none runs in a lane (R4.2); no hook runs (H4).
func (l *Lander) Land(ctx context.Context, req LandRequest) (LandResult, error)

// RefreshClones tells every clone of the branch what the project holds. It is the ONE origin
// refresh: Land's step 14, Undo and Revert (task 10), BringIn (below), RefreshProject, the
// refresh route (below) and the lane syncer's tip-change sweep (task 07, through
// Syncer.Refresher; review I4) call it. The caller holds Locks.Lock. RefreshClones is
// RefreshLanes(ctx, projectPath, branch, every lane of the branch); the sweep calls RefreshLanes
// with only its stale lanes (task 07; re-review b06-1 issue 2), and an id that is not a lane of
// the branch is skipped. First, once, in the project (Scope Project):
//   up  = rev-parse --verify -q --symbolic-full-name <branch>@{upstream}, OKExit 1
//         (exit 1 or not under refs/remotes/ → up = "")
// Deferral (review r4-1 I-6). For each lane asked for, live = l.LaneGit.Live(ctx, id) (task
// 24; no sandbox is started to answer it). A lane with no live session is not fetched: it is
// l.Syncer.Defer(id, tip), in neither refreshed nor errs, and costs no container start under
// the project lock. The sweep refreshes it once a session is live on it again (task 07, Run
// step b), within one Interval of the session starting; that lag is the recorded residual.
// ForceRefreshLanes is RefreshLanes with no deferral. Its one caller is RefreshBranch, the
// refresh route, which ResolveDialog (task 19) calls just before it starts a session on the
// lane: the one moment a sessionless lane must be current. It starts a runner-only sandbox
// when it must. Land, Undo, Revert, BringIn, RefreshProject and the sweep all defer.
// Then, in the project (Scope Project), task 24's bundle out (R4.5), grouped by what each
// remaining lane already holds (review r4-1 I-8). told is task 07's unexported told map, read
// under the Syncer's lock (Syncer and Lander share package converge). refs is
// [refs/heads/<branch>] + [<up> when up != ""]. For each distinct told sha t:
//   bundle_t, done_t = lanegit.SendBranch(refs, basis: [t])
//   — `bundle create` into <Hardening.OwnedRoot>/bundles, shared by every lane whose told is t.
//   One basis per bundle is the point: `bundle create … --not A B`, A an ancestor of B, has only
//   B as its prerequisite, and a lane holding A would fail verify and pull the whole history.
//   On ErrBundleEmpty (the branch went back to a commit t already contains, as task 10's Undo
//   does): SendBranch(refs, basis: the tip's parent shas, from `rev-parse <tip>^@` in the
//   project, Read), a one-commit bundle every such
//   lane can take, since it holds t and so the tip and its parents. A root-commit tip has no
//   parents, and gets the no-basis bundle.
// Lanes with no told at all share one SendBranch(refs, basis: []), the whole history. That is
// the only no-basis bundle. A lane whose Fetch fails on a missing prerequisite (its clone lost
// objects told says it has) is an errs entry, and its told is cleared, so the sweep's retry
// sends it the no-basis bundle; it is never retried inside this call.
// Then, for each lane asked for and not deferred, r, release = l.LaneGit.For(lane), and one
// runner call (task 24), with its group's bundle:
//   r.Fetch(bundle, refspecs: +refs/heads/<branch>:refs/remotes/origin/<branch>
//                             [+<up>:refs/remotes/upstream/<branch>   — only when up != ""],
//           deletes: [refs/remotes/upstream/<branch>]                 — only when up == "")
//   The delete (a no-op when absent) keeps a clone from offering a remote branch the project
//   no longer tracks. The bundle file is rewound for each lane of its group.
// done_t() deletes each bundle file after the last lane of its group reads it. When no asked
// id is a lane of the branch, or every one is deferred, nothing is sent.
// No host git runs in a lane, and the runner is told no path of the project's (R4.2):
// sandboxd fetches from the bundle file it is streamed. refs/remotes/upstream/<branch> is
// review I5: a clone's origin/* mirrors the project's refs/heads/*, so without it no lane can
// see, merge or resolve against a teammate's pushed commits. It carries objects and a ref, no
// credential and no URL (brief A4): the project's remote is never named to the clone.
// --no-auto-maintenance (review Minor 8) is now the runner's flag on its fetch (task 24,
// TestRunnerFetchStartsNoMaintenance): a plain fetch starts a detached `maintenance run --auto`
// in the clone, whose gc holds packed-refs.lock while the next refresh's delete (above) or a
// land's step-14 refresh runs, and fails it.
// After each lane's Fetch succeeds:
//   got = FetchResponse.Refs["refs/remotes/origin/<branch>"]   (the runner's read-back)
// then l.Syncer.Told(id, got), so the next sweep knows what this clone holds (task 07; review
// I4). got is what the fetch delivered, not a tip read before it, so a branch that moves, or
// moves away and back, while the fetches run leaves told unequal to the tip and costs one more
// refresh, never a missed one (re-review b06-1 issue 5). A Fetch error, a runner that does not
// answer (RunnerError, Status 0), a For error (ErrNoRunner included) or a read-back missing
// that ref is that lane's errs entry. refreshed and errs are made empty before SendBranch,
// never nil (issue 3). A SendBranch failure is every asked lane's errs entry.
func (l *Lander) RefreshClones(ctx context.Context, projectPath, branch string) (refreshed []string, errs map[string]string)
func (l *Lander) RefreshLanes(ctx context.Context, projectPath, branch string, ids []string) (refreshed []string, errs map[string]string)
func (l *Lander) ForceRefreshLanes(ctx context.Context, projectPath, branch string, ids []string) (refreshed []string, errs map[string]string)

// RefreshProject runs RefreshClones for every branch that has a cockpit worktree in the
// project. Like RefreshClones it takes no lock: its one caller is task 12's Fetcher.AfterFetch,
// which runs inside Fetch's Locks.Lock(ctx, projectID) (gitx.Locks is not re-entrant). So a
// fetch that moves refs/remotes/origin/<branch> reaches every clone as upstream/<branch>
// before any land can read it. Errors are logged per clone; it returns nothing.
func (l *Lander) RefreshProject(ctx context.Context, projectPath, projectID string)
```

```go
// gitdeps.go
	// ── task 09 ──
	Lander   *converge.Lander
	Resolver briefref.Resolver // nil until phase 4; Lander holds the same value
	// and, where step 3 builds the Lander (review I4): Syncer.Refresher = Lander and
	// Syncer.Locks = Locks, so task 07's sweep refreshes on a tip change; and Lander.LaneGit =
	// GitDeps.LaneGit (task 24), the runners RefreshLanes fetches through. Leaving any unset
	// is the silent-no-op failure this program keeps paying for; TestMainWiresTheSweepRefresh
	// pins all three.
```

```go
// RefreshBranch is the refresh route's verb (review I4): Locks.Lock(ctx, projectID), then
// ForceRefreshLanes(ctx, projectPath, branch, every lane of the branch), so a deferred lane
// is refreshed too (review r4-1 I-6). ResolveDialog (task 19) calls it before it starts a
// session, so the prompt's origin/<branch> and upstream/<branch> are the branch as it is now,
// not as the last land or sweep left them. It writes only the clones' remote-tracking refs.
func (l *Lander) RefreshBranch(ctx context.Context, projectPath, projectID, branch string) (refreshed []string, errs map[string]string, err error)
```

**Wire: `POST /v1/projects/{projectId}/branches/{branchId}/refresh`** (review I4)

```
request  {}   (no body is also accepted)
200 { "refreshed": ["w-4a77b1c9"], "refreshErrors": { "w-5e6f7a8b": "fatal: …" } }
200 { "refreshed": ["w-4a77b1c9", "w-5e6f7a8b"], "refreshErrors": {} }   (nothing failed: {}, never null)
200 { "refreshed": [], "refreshErrors": {} }                            (a branch with no lanes)
404 not_found for an unknown project or branch; 500 git_failed with gitSays when the project's
own rev-parse fails. A clone's failure is a refreshErrors entry, never a failed request.
```

Audit line: *"Tells each of this branch's worktrees what the project's branch and its upstream
hold now, with a bundle from the project into each lane's runner, which fetches it into the
clone's origin/ and upstream/ refs. Writes those refs only."*

**Wire: `POST /v1/projects/{projectId}/branches/{branchId}/lands`**

```json
// request
{ "worktreeId": "w-4a77b1c9", "expectedTip": "a90d3f4c…",
  "briefKey": "brief:b-42@6", "briefDigest": "sha256:…" }       // the last two: both or neither

// 201 Created
{ "merge": "d5f0a17…", "before": "a90d3f4c…", "branch": "evidence-upload", "worktreeId": "w-4a77b1c9",
  "briefKey": "brief:b-42@6", "readyCleared": true,
  "refreshed": ["w-4a77b1c9", "w-5e6f7a8b"], "refreshErrors": {}, "broughtBack": [] }
```

| Refusal | Status, code | `details` |
|---|---|---|
| missing `worktreeId` or `expectedTip`; one of key/digest without the other; a key or digest `gitx.Message` refuses | 400 `bad_request` | — |
| worktree not a cockpit worktree of this branch | 400 `bad_request` | — |
| unknown project | 404 `project_not_found` | `projectId` |
| every `*Refusal` | 409 `conflict` | `reason` plus the table in `Land`'s comment |
| step 4's sync got a `*lanegit.BundleRefused` from task 24's `FetchLane` (a truncated, oversized, extra-ref or fsck-failing bundle); no ref moved | 409 `conflict` | `reason: bundle_refused`, `why` (task 24) |
| sync, merge-tree, commit-tree or a non-race update-ref failure; a lane runner that does not answer (`RunnerError`, Status 0) or fails an op | 500 `git_failed` | `gitSays` |

"Land all in order" is the desktop calling this once per queued worktree, top down, passing the
previous response's `merge` as the next `expectedTip`, and stopping at the first refusal. There is
no batch route: every land is its own compare-and-swap, which is what makes a partial run honest.

#### Bring in origin/<branch> (review I5)

A land takes lanes only, and a clone's objects stop at clone time, so without this verb a
teammate's pushed commits reach the product branch only through a terminal `git merge` in a
checkout of it, and that checkout switches landing off (A5, Q3). A push is then rejected
forever. Task 08 shows the row when `remote.behind > 0`; the plan's Q10 records the choice.

```go
package converge

// BringInRequest brings the project's remote-tracking branch into the product branch.
type BringInRequest struct {
	ProjectID        string
	ProjectPath      string
	Branch           string
	ExpectedTip      string // Convergence.Tip the human saw; required
	ExpectedUpstream string // Convergence.BringIn.Tip the human saw; required
}

// BringInResult is a bring-in that happened.
type BringInResult struct {
	Tip           string            `json:"tip"`    // the branch's new tip
	Before        string            `json:"before"` // ExpectedTip
	Mode          string            `json:"mode"`   // "fast-forward" | "merge"
	Refreshed     []string          `json:"refreshed"`
	RefreshErrors map[string]string `json:"refreshErrors"`
}

// BringIn, holding Locks.Lock(ctx, req.ProjectID) for the whole verb. Steps 1-5 write nothing.
//  1. CheckedOutAt(branch) == ""                               (branch_checked_out, details.path)
//  2. up = rev-parse --verify -q --symbolic-full-name <branch>@{upstream}, OKExit 1
//     exit 1, or not under refs/remotes/                       (no_remote)
//  3. old = rev-parse refs/heads/<branch>; old != ExpectedTip  (branch_moved, details.tip)
//  4. upTip = rev-parse <up>; upTip != ExpectedUpstream        (upstream_moved, details.upstream)
//  5. upTip an ancestor of old                                 (nothing_to_land)
//  6. old an ancestor of upTip → Mode "fast-forward":
//       update-ref -m "panacea: bring in <up>" refs/heads/<branch> upTip old
//     else Mode "merge": Identity(ProjectPath) (no_git_identity);
//       MergeTreeArgs + old upTip, OKExit 1 → ParseMergeTree (merge_conflict,
//       details.paths = detailList — resolved in a lane that merges origin/<branch>, then
//       upstream/<branch>, so it holds both sides whatever its age; task 19's prompt);
//       name = up without "refs/remotes/", e.g. "origin/evidence-upload" (never a literal origin);
//       msg = gitx.Message("Bring in <name>", "The commits <name> has and <branch> lacked,
//             as of the last fetch.", [Panacea-Bring-In: <upTip>]) — the sha, not <name>,
//             because Message's trailer charset refuses some branch names and a sha never;
//       merge = commit-tree <tree> -p old -p upTip -F -; update-ref … refs/heads/<branch> merge old
//     A refused update-ref is Land's step 12: branch_moved, else git_failed.
//  7. l.RefreshClones(ctx, ProjectPath, Branch)
// A fast-forward is right here and wrong for a land (3-D4): no lane's work is recorded, and the
// result is exactly <name>, neither ahead nor behind. No ready flag moves and no
// Panacea-Land trailer is written, so undo and revert refuse it as not_a_land (task 10). A merge
// carries Panacea-Bring-In instead, so task 10's history files it as "bring-in", not "direct"
// (re-review b05 issue 5): the cockpit made that commit, not someone outside it.
// Hardened per task-02 §HGH: 1-6 Scope Project; 7 is RefreshClones, a bundle from the project
// into each lane's runner (task 24), so every host call is Scope Project and none runs in a
// lane (R4.2); no hook or merge driver runs.
func (l *Lander) BringIn(ctx context.Context, req BringInRequest) (BringInResult, error)
```

**Wire: `POST /v1/projects/{projectId}/branches/{branchId}/bring-in`** — request
`{ "expectedTip": "a90d3f4c…", "expectedUpstream": "5c21e0b…" }`; `201` is `BringInResult`;
400 `bad_request` for a missing field; every `*Refusal` is 409 with its reason;
git failures are `git_failed`, as the land route.

#### The tests to write first

Real repositories: a project, two clones made by `lanes.Creator`, agent commits made in the clones,
a `Syncer` over them, a fake `WorktreeFlagStore`, and a fake `Resolver`. Every lane's runner is
task 24's `lanegittest.Fake` over its clone, as task 24's spy test wires it (R4.3); a "Runner
wrapper" or "recording Runner" below is task 02's `gitx.Runner`, on project calls only.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLandMakesATwoParentMergeWithTheLink` | the new tip has parents `[old, laneTip]`, its tree is merge-tree's, and `%(trailers)` reads `Panacea-Land: <id>`, `Panacea-Brief: brief:b-42@6`, `Panacea-Brief-Digest: sha256:x` | a squash or a fast-forward (3-D4) |
| `TestLandAlwaysMakesAMergeEvenWhenFastForwardable` | the lane already contains the tip → still two parents | a fast-forward that loses the land's record |
| `TestLandLosesARaceCleanly` | a Runner wrapper commits to the branch between steps 11 and 12 → `branch_moved`, the branch is at the injected commit, not the merge | `update-ref` without its old value |
| `TestLandRefusesAStaleExpectedTip` | `ExpectedTip` ≠ tip → `branch_moved` with `details.tip`, nothing written (loose-object count unchanged) | landing against a view the human did not see |
| `TestLandRefusesACheckedOutBranch` | the project's primary has the branch checked out → `branch_checked_out`, `details.path` is its path | moving a ref under a user's checkout |
| `TestLandConflictWritesNothing` | both lanes edit a line; land the first, then the second → `merge_conflict`, `details.paths`, tip unchanged | a conflicted product branch (A2) |
| `TestLandConflictNamesANonASCIIPath` (review I2) | the same with the file `docs/résumé.md` → `json.Unmarshal(details.paths)` is `["docs/résumé.md"]` (raw UTF-8, not `"docs/r\303\251sum\303\251.md"`) | `merge-tree` without `-z`, or a newline-joined detail |
| `TestLandRefusesNotReady` | ready false → `not_ready` | landing work nobody marked ready (defect 9) |
| `TestLandRefusesAnEmptyLane` | no commits → `nothing_to_land` | an empty merge commit |
| `TestLandSyncsTheLaneFirst` | commit in the clone with no sweep since → the merge's second parent is that commit | landing a stale lane ref |
| `TestLandNamesTheUndoneLandsItBringsBack` (re-review b06-1 issue 1b) | land A → M; B's clone merges M and commits; A's land undone exactly; land B → 201, `broughtBack == [{sha: M, worktreeId: A}]`, and B's merge has M as an ancestor; a land of a lane that never merged M → the raw body contains `"broughtBack":[]` and `"refreshErrors":{}` | a land that puts undone work back on the branch without saying so, or a `null` the desktop indexes into |
| `TestLandWithoutAResolverRefusesAKey` | `Resolver == nil`, key set → `brief_unresolvable`/`no_resolver` | an unverified key in history (decision 4) |
| `TestLandWithoutAKeyWritesNoBriefTrailer` | no key → `Panacea-Land` only | an empty `Panacea-Brief:` trailer |
| `TestLandRefusesAMissingOrMismatchedRevision` | resolver `ok == false` → `/missing`; wrong digest → `/digest_mismatch` | recording a key that names different text |
| `TestLandCopiesTheKeyVerbatim` | key `brief:b-42@6` → exactly that string in the trailer | normalising a key (P5) |
| `TestLandRefusesAnInjectedKey` | key containing `\nPanacea-Land: w-x` → 400 before any git write | a forged second trailer |
| `TestLandCommitsAsTheUser` | author and committer of the merge equal `Identity`'s answer; no identity → `no_git_identity`, nothing written | a merge by `dev@container` or by nobody |
| `TestLandClearsReady` | after the land, the flag store holds `ready:false` for the worktree | a landed lane sitting in the queue (decision 2) |
| `TestLandTellsEveryClone` | after landing clone A, clone B's `refs/remotes/origin/<b>` equals the merge | 3-D3's prompt naming a stale `origin/<b>` |
| `TestLandRefreshFailureDoesNotFailTheLand` | clone B's directory removed, so B's `Fake.Fetch` fails → 201, `refreshErrors[B]` holds the runner's `gitSays` | failing a land that happened |
| `TestRefreshGivesEveryCloneTheUpstream` (review I5) | a bare "remote", the project fetched after a teammate's push → after a land, clone B's `refs/remotes/upstream/<b>` equals the project's `refs/remotes/origin/<b>`, and `git merge upstream/<b>` in B exits 0; B's `config --get-regexp ^remote\.` is unchanged | a lane that cannot see a teammate's commits |
| `TestRefreshWithoutAnUpstreamStillRefreshesOrigin` | no upstream configured → clone B's `origin/<b>` refreshed, no `upstream/<b>`, no RefreshError; then unset the upstream after one refresh → the next refresh deletes B's `upstream/<b>` | a fetch of a missing source ref (exit 128) failing every refresh of a never-pushed branch |
| `TestRefreshStartsNoMaintenance` (review Minor 8; the no-maintenance proof moved to task 24's `TestRunnerFetchStartsNoMaintenance`, since the fetch now runs in the runner) | `RefreshClones` with upstream set, then unset → each lane's recorded `Fake.Fetch` request is exactly refspecs `+refs/heads/<b>:refs/remotes/origin/<b>` and `+<up>:refs/remotes/upstream/<b>`, deletes `[]`; then refspec `+refs/heads/<b>:refs/remotes/origin/<b>` only, deletes `[refs/remotes/upstream/<b>]`; the bundle's `list-heads` are exactly the refs asked for | a refresh that asks the runner for another namespace, or a fetch path that bypasses the runner's maintenance-free fetch |
| `TestRefreshProjectTakesNoLock` | called while the test holds `Locks.Lock(ctx, p)` → returns (a deadlock fails the test's 5 s deadline) | AfterFetch deadlocking inside Fetch |
| `TestSweepRefreshesClonesAfterADirectCommit` (review I4) | real git, a real `Syncer` with this `Lander` as `Refresher`: two lanes; a commit made with plain `git commit` in a checkout of the project's branch, as a user's terminal would; one `Run` sweep → both clones' `refs/remotes/origin/<b>` equal the new tip, and a second sweep makes no `Fake.Fetch` call and no `SendBranch` (a recording Fake and Runner) | a resolve prompt that merges an origin/<branch> the user's own commit has left behind |
| `TestRefreshClonesRecordsWhatItSent` (review I4) | after a land, `Syncer`'s told for both lanes is the merge; with clone B's directory removed (B's `Fake.Fetch` fails), B's told is unchanged and A's is the merge | a clone recorded fresh after its fetch failed, which the sweep then never retries |
| `TestRefreshRecordsWhatTheCloneReceived` (re-review b06-1 issue 5) | real git, a Runner wrapper that moves the project's `refs/heads/<b>` from T1 to T0 just before `SendBranch`'s `bundle create` and back to T1 just after it → the bundle carries T0, A's `Fake.Fetch` answers `refs["refs/remotes/origin/<b>"] == T0`, `Told(A, T0)`, A's `origin/<b>` is T0, and the next sweep refreshes A to T1; a Fake whose `Fetch` answers a `refs` map without that ref → A's errs entry, told unchanged; `RefreshLanes(…, [A])` touches no other clone; `RefreshLanes(…, ["w-gone"])` for an id that is not a lane of the branch → no `SendBranch`, no fetch, `refreshed == []`, `errs == {}` | a told taken from a tip read before the fetch, which the away-and-back case leaves stale for good |
| `TestSweepDoesNotHammerAFailingClone` (re-review b06-1 issue 2) | real git, a real `Syncer` with this `Lander` as `Refresher`, a recording `Fake` per lane and a fake `Now`: lanes A and B; B's runner does not answer (a `Fake` wrapper whose every op returns `RunnerError{Status: 0}`, as a dead `sandboxd` does); a direct commit; the first sweep calls `Fetch` on A's and B's runner once each, B fails, and B's error is logged once; three more sweeps inside the backoff with the tip unchanged → no `Fetch` on A or B and no `SendBranch`, no second log line; a second direct commit → exactly one `Fetch` on each of A and B. The same with B's clone directory removed instead, so B's `Fake.Fetch` answers a failed git | one broken clone, which an agent can make on purpose, re-fetching every clone of the branch under the project lock every five seconds |
| `TestRefreshRunsNoHostGitInTheLane` (R4.2) | red first: planted as the pre-R4 flow, a host `fetch <ProjectPath> …` run through `gitx.Exec` with `Dir` = clone B fails it. `gitxtest.Spy` (task 02) on the `Lander`'s `gitx.Exec` through a land, `RefreshClones` with and without an upstream, and `RefreshBranch` → no recorded process has `Dir`, `-C`, `--git-dir`, `--work-tree`, `GIT_DIR`, `GIT_WORK_TREE` or any argv element under the clone root (task 24's `TestHostNeverRunsGitInALane` rule), and every lane was reached only through its `Fake` | a refresh that runs host git in a lane, where the agent's config, hooks and upload-pack run on the host |
| `TestRefreshSendsOneBundlePerTold` (R4.5; review r4-1 I-8) | three live lanes of `b`, two told T1 and one told T0, T0 an ancestor of T1; one `RefreshClones` → the spy records exactly two `bundle create`, in the project, with refs `refs/heads/<b>` [+ `<up>`], one `--not T1` and one `--not T0`, never both in one call; the two T1 lanes' `Fake.Fetch` got the same bytes; every lane is refreshed and `<OwnedRoot>/bundles` is empty after. A lane with no told → one `bundle create` with no `--not`, the only one. A lane whose clone lacks its told commit (pruned in the test) → an `errs` entry, its told cleared, no second `bundle create` in this call. An Undo back to T0 while every lane's told is T1 (`ErrBundleEmpty`) → one `bundle create … --not <T0's parents>`, every lane at T0 | a bundle per lane; a union basis that makes the T0 lane fail verify; a whole-history bundle after an undo |
| `TestRefreshDefersASessionlessLane` (review r4-1 I-6) | lanes A (live session) and B (none), `lanegittest.Backend` counting `Create`; a land → A refreshed, B in neither `refreshed` nor `errs`, `Create` never called, `Syncer`'s told for B unchanged and B marked deferred; `POST …/refresh` (`RefreshBranch`) → B refreshed through one runner-only sandbox | a land that starts a container per sessionless lane under the project lock; a refresh route that skips B |
| `TestRefreshRouteRefreshesTheBranchsClones` (review I4) | a direct commit, then `POST …/refresh` → 200, `refreshed` names both lanes, both clones' `origin/<b>` equal the tip, and the raw body contains `"refreshErrors":{}` (re-review b06-1 issue 3); a branch with no lanes → the raw body contains `"refreshed":[]` and `"refreshErrors":{}`; with the test holding `Locks.Lock(ctx, p)` the request stays unanswered until release; an unknown branch → 404 | a ResolveDialog whose refresh skips the lock or lies about which clones it reached |
| `TestMainWiresTheSweepRefresh` (review I4; `cmd/api`) | step 3's wiring function, called as `main.go` calls it → `Syncer.Refresher` is the built `Lander`, `Syncer.Locks` is `GitDeps.Locks`, and `Lander.LaneGit` is `GitDeps.LaneGit` (task 24) | a sweep refresh that ships switched off because one assignment was left out |
| `TestBringInFastForwardsWhenOnlyBehind` | branch 0 ahead, 2 behind → `Mode "fast-forward"`, tip == the upstream tip, no new commit object | a needless merge commit that makes the branch ahead of origin |
| `TestBringInMergesWhenDiverged` | 1 ahead, 1 behind → `Mode "merge"`, parents `[old, upTip]`, no `Panacea-Land` trailer, `%(trailers)` reads `Panacea-Bring-In: <upTip>` (re-review b05 issue 5); undo of it → `undo_not_exact/not_a_land` | a bring-in undo treats as a land |
| `TestBringInConflictWritesNothing` | the teammate and the branch edit one line of `docs/résumé.md` → `merge_conflict`, `json.Unmarshal(details.paths) == ["docs/résumé.md"]`, tip unchanged, no ref moved | a conflicted product branch (A2) |
| `TestBringInRefusesAMovedUpstream` | a fetch moves the upstream after the GET → `upstream_moved`, `details.upstream` the new sha, tip unchanged | bringing in commits the human never saw forecast |
| `TestBringInRefusals` | table: checked out → `branch_checked_out`; no upstream → `no_remote`; upstream an ancestor → `nothing_to_land`; stale `expectedTip` → `branch_moved` | a refusal that surfaces as 500 |
| `TestLandSerialisesPerBranch` | two concurrent lands of different lanes on one branch under `-race` → one succeeds, the other gets `branch_moved` or lands after it; never both on the same `old` | a double land on one tip |
| `TestLandWaitsOnTheProjectLock` (review I7) | the test holds `Locks.Lock(ctx, p)`: a land on branch `a` and one on branch `b` of `p` have run no git call 50 ms later (a recording Runner); release → both land. Through the handler: holding `Deps.Git.Locks` for `p` keeps `POST …/lands` unanswered until release. A `nil` `Locks` → 500 `internal`, nothing run | a per-branch lock task 12's push does not share; a Lander given a lock of its own |
| `TestServeWiresGit` (cmd/api; task 04's test, extended; re-review b04-1 issue 3) | red first against a `main.go` that gives the `Lander` `&gitx.Locks{}`: `Git.Lander.Locks` is the same pointer as `Git.Locks` | a Lander given a lock of its own, which every handler test (each builds its own Deps) would miss |
| `TestLandRouteMapsEveryRefusal` | table test over the handler: each `*Refusal` → 409 with its reason; each `*gitx.Error` → `git_failed`; a `*gitx.UntrustedConfig` → 409 `project_config_untrusted` with `details.key` | a refusal that surfaces as 500 |
| `TestLandRunsNoProjectHook` | task-02 §HGH, red first against an unhardened `Exec`: the project's `.git/hooks/reference-transaction` writes a marker → a land succeeds and no marker exists | a land that honours the repository's hooks |
| `TestLandRefreshNeverUsesTheClonesOrigin` | review C3, red first; R4.4-4.5. (a) Clone B's config sets `remote.origin.url` to an `httptest` smart-HTTP server → after a land, B's `refs/remotes/origin/<b>` equals the merge and the server received no request: B's runner fetched the bundle file, and no refspec or argument named a remote. (b) B's config also sets `http.proxy` to a second `httptest` server, `http.sslVerify=false` and `remote.origin.uploadpack=<marker script>` → neither server received a request, no marker exists, `refreshErrors` is `{}`, and the spy shows no host process read B's config (the lane's config is the agent's own, inside its sandbox, R4.2) | a refresh that fetches `origin`, or names a remote to the runner |

#### Steps

- [ ] **Step 1: Preflight.** Task 08 merged; `briefref` (task 03) and `gitx.Locks` (task 02) and
  `GitDeps.Locks` (task 04) exist. Task 01's `TestBehaviourUpdateRefCompareAndSwapRefuses`
  recorded the refusal's stderr; the race test asserts on the **re-read tip**, not on that text.
- [ ] **Step 2:** the tests; `land.go`, then `bringin.go`; the three handlers (land, bring-in, and review I4's refresh), routes and deps block.
- [ ] **Step 3:** wire `Lander` in `main.go` with `Locks: gitDeps.Locks`, `Identity` adapting
  `api.Deps.GitIdentity` (the fix's `func(ctx, path) (api.GitIdentity, error)`) to `gitx.Ident`,
  and `Resolver: nil`; then `syncer.Refresher = lander` and `syncer.Locks = gitDeps.Locks`
  (review I4). Put these assignments in one function `main.go` calls, so
  `TestMainWiresTheSweepRefresh` calls the same code, not a copy of it.
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
  pushed, built upon, or merged into another lane (task 10; review I3).
- The merge link is forgeable by anyone who can commit (survey §7.2); it is a record of what the
  cockpit did, not a signature.
- The sweep refresh (task 07) keys on the branch tip only. A terminal `git fetch` in the project
  moves `<branch>@{upstream}` without moving the branch, so the clones' `upstream/<branch>`
  catches up at the next cockpit refresh: a land, an undo, a revert, a cockpit fetch, or the
  refresh route `ResolveDialog` calls before a session. No resolve prompt names a stale upstream.
