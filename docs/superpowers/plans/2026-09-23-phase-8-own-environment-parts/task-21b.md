### Task 21b: `machined` bundles — lane work in and product commits out, as files in its owned root

Phase 3's ruling R4.4 moves lane work across as a `git bundle`, which is data, verified and
fetched in the project under §HGH. On a cloud machine the project is on the machine disk and the
lane's runner is `sandboxd` in another pod (env ruling 2), so the bundle needs a home the project
calls can name. Env ruling 3 (`.superpowers/sdd/env/r4-env-rulings.md:109-173`) puts it in
`machined`'s owned root, and the laptop relays it. This task builds `machined`'s half:
- **the bundle routes**: `POST /v1/bundles` (a body, or `?reserve=1`), `GET` and `DELETE
  /v1/bundles/{id}`, each behind `machined`'s bearer, reached through the broker's agent proxy
  (`task-19.md:89-94`);
- **the one bundle argument** `POST /v1/git` admits, on four ops and `Scope` `project` only;
- **the limits**, the `429` at two live ids, and **the sweep**.

The laptop's side (`lanegit.Bundles` over these routes, and the relay from the lane's runner) is
task 25b's. Split out of task 21 by env ruling 3's "What each task changes" (`:274-284`).

**Depends on:**
- **task 21** (`POST /v1/git`, `gitx.NewExec(h)` with `OwnedRoot` `/tmp/machined-git`);
- **task 19's `/tmp` `emptyDir` with `sizeLimit: 3Gi`** (ruling 3: two live bundles at
  `MaxBundleBytes` plus clone scratch). Task 19 owns it: its pod row (`task-19.md:26`) and
  `TestMachinePodTmpHasASizeLimit` (`task-19.md:229-231`);
- **task 19's agent proxy streaming a bundle body.** Task 19 owns it (`task-19.md:96-103`): on
  `POST …/agent/v1/bundles` only, the body streams through a `GitMaxBundleBytes` cap with
  `GitBundleTimeout` + 5 s as its deadline; every other path keeps the 16 MiB request cap
  (`task-19.md:93`). Its tests are `TestAgentProxyStreamsABundleUpToItsCap`,
  `TestAgentProxyKeepsTheCapOffTheBundlePath` and `TestAgentProxyBundleDeadline`
  (`task-19.md:225-227`). This task edits neither;
- **phase 3 seam S8-P3.4** (`lanegit.Bundles`, ruling 3 `:166-173`): an ask of phase 3, not yet
  answered. This task's routes stand on their own, but nothing calls them until S8-P3.4 and
  task 25b land. Step 1 records what landed.

**Files:**
- `control-plane/internal/machineapi/wire.go` — `BundleID`, the limits, the codes (modify)
- `control-plane/cmd/machined/bundles.go`, `bundles_test.go` (new) — the store, the routes, the sweep
- `control-plane/cmd/machined/git.go` — the bundle argument admission (modify)
- `control-plane/cmd/machined/server.go`, `main.go` — the routes; start empties the directory
  and starts the sweep (modify)

#### Contract (complete)

```go
package machineapi

import "time"

// BundleID is POST /v1/bundles' 201 answer. ID is 16 lowercase hex from crypto/rand,
// never derived from the time, a counter or the body.
type BundleID struct {
	ID string `json:"id"`
}

// The limits are phase 3's (phase-3 task-24.md:365-370), restated here because
// control-plane cannot import sandboxd's internal gitrunner (task-24.md:363-364).
// They apply to POST /v1/bundles' body and GET's file; the broker applies the same.
const (
	MaxBundleBytes = 1 << 30           // one bundle, either way
	BundleTimeout  = 120 * time.Second // a POST's whole body read
	MaxLiveBundles = 2                 // reserved or stored ids at once (ruling 3)
	BundleSweep    = 60 * time.Second  // how often the sweep runs
	BundleMaxAge   = 2 * BundleTimeout // a live id older than this is removed by the sweep
	BundleDir      = "bundles"         // under machined's OwnedRoot: /tmp/machined-git/bundles
)

// Problem codes of the bundle routes, beside task 21's.
//   too_large         413: the body passed MaxBundleBytes, or GET's file is past it
//   git_timeout       504: the body was still arriving at BundleTimeout
//   bundle_busy       429: MaxLiveBundles ids are live; nothing was stored
//   bundle_not_found  404: no live id, or (GET) a reserved id git has not written
const (
	CodeTooLarge       = "too_large"
	CodeGitTimeout     = "git_timeout"
	CodeBundleBusy     = "bundle_busy"
	CodeBundleNotFound = "bundle_not_found"
)
```

| Route | Answer | Failures |
|---|---|---|
| `POST /v1/bundles` | `201 BundleID`; the file is `<id>.bundle` | `400` (a body with `?reserve=1`) · `413 too_large` · `504 git_timeout` · `429 bundle_busy`; on each, no file and no live id |
| `POST /v1/bundles?reserve=1` | `201 BundleID`; no file | `429 bundle_busy` |
| `GET /v1/bundles/{id}` | `200`, `application/octet-stream`, the file streamed | `400` (`id` not `^[0-9a-f]{16}$`) · `404 bundle_not_found` · `413 too_large` (a file git wrote past the cap) |
| `DELETE /v1/bundles/{id}` | `204`, also for an id that is not live | `400` |
| `POST /v1/git` (task 21's) | unchanged | adds `400` for any argument under `/tmp/machined-git/` but the one below |

**The store (`bundles.go`).** `machined` owns every file (ruling 3, "Who deletes").
- The directory is `<OwnedRoot>/bundles`, made mode `0700` at start and emptied then, so a
  restart forgets every id and leaves no file.
- A live id is an in-memory entry: its creation time, and whether a file is there. Two at most,
  reserved and stored alike; a third `POST` is `429` before any byte is read. The caller is
  told, not queued: task 25b maps it to `ErrBundleBusy`, nothing waits or retries, and that land,
  archive or refresh fails with it, so a third concurrent one on one machine is refused and the
  user runs it again. Whether phase 3's callers then retry is theirs (S8-P3.4, **UNVERIFIED**).
- `POST` with a body opens `<id>.bundle` with `O_CREATE|O_EXCL`, mode `0600` (phase 3's rule,
  `task-24.md:705-708`). An id whose file exists is drawn again, never reused. It copies the
  body through `io.LimitReader(body, MaxBundleBytes+1)` under a `BundleTimeout` deadline, and
  never holds the bundle in memory. Past the cap it is `413`; at the deadline, `504`. Either
  way the partial file is removed and the id is not live.
- `?reserve=1` makes a live id and no file, for SendBranch's `bundle create` (ruling 3,
  "Project to lane"). Git then writes the file, with git's own mode inside the `0700`
  directory.
- The sweep runs every `BundleSweep`: it removes any live id older than `BundleMaxAge`, with its
  file, and any file in the directory that no live id names.

**The argument `POST /v1/git` admits** (ruling 3, `:128-131`). `machined`'s task 21 check refuses
any argument that names the owned root. It admits one exception: an argument **exactly**
`/tmp/machined-git/bundles/<id>.bundle`, where `<id>` is live, on a call whose `Scope` is
`project`, whose `Dir` is `projects/<name>`, and whose `Args` begin `bundle verify`,
`bundle create`, `bundle list-heads` or `fetch`. Phase 3's code passes the path unchanged, so
the laptop names it through `lanegit.Bundles.Path(id)` (S8-P3.4). A `..`, a second path, another
directory under the owned root, or the path on any other op is `400`, git never ran. `bundle
create` needs a reserved id with no file yet; the other three need a stored file.

#### Tests to write first (`bundles_test.go`; real `git` where a row says so, skipped with a message if absent)

Every test is named `TestBundle…`. The limits are fields a test may shrink; the clock and the id
source are injected.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestBundlePostStoresTheBodyAndAnswersAnID` | a body → 201, `id` matches `^[0-9a-f]{16}$`, `<dir>/<id>.bundle` holds the bytes, mode `0600`, the directory `0700`; an id source pinned to an existing file's id → that file's bytes unchanged and a second id answered | a file opened without `O_EXCL`, overwriting a live bundle; a world-readable bundle |
| `TestBundlePostOverTheCapIs413AndLeavesNoFile` | cap 1024: 1024 bytes → 201; 1025 → 413 `too_large`, no file, no live id | an off-by-one cap; a partial file left in the 3 GiB `/tmp` |
| `TestBundlePostSlowBodyIs504` | `BundleTimeout` 50 ms and a body that stalls → 504 `git_timeout`, no file, no live id | a stalled relay that holds a slot and a partial file forever |
| `TestBundleThirdLiveIDIs429` | two live (one stored, one reserved) → a third `POST` 429 `bundle_busy` before its body is read; after one `DELETE` → 201 | a count of files only, so reserved ids let a third bundle in |
| `TestBundleReserveMakesAnIDAndNoFile` | `?reserve=1` → 201 and no file; `?reserve=1` with a body → 400; `GET` of the reserved id → 404 `bundle_not_found` | a reserve that plants an empty file `bundle create` then refuses |
| `TestBundleGetStreamsAndDeleteIsIdempotent` | `GET` → the bytes; `DELETE` → 204, again → 204; `GET` after → 404; `id` `../x` → 400 | a `DELETE` the laptop's deferred retry turns into an error; a path built from an unchecked id |
| `TestBundleArgumentIsExactlyALivePath` | real git, `projects/a`. Admitted: `bundle create <path(reserved)> main`, then `bundle verify`, `bundle list-heads` and `fetch <path> +refs/heads/main:refs/panacea/lanes/x/main` on a stored id → 200. Each 400, git never ran: a path for a dead id; `<dir>/../clone/x`; `/tmp/machined-git/no-hooks/x`; the live path on `status`; the live path with `Scope: "none"`; `bundle create` on a stored id | a prefix check on `/tmp/machined-git/`, which lets git write into `no-hooks` or `clone`; an op list left open |
| `TestBundleSweepRemovesStaleAndStartEmpties` | fake clock: an id aged `BundleMaxAge` + 1 s → gone with its file; a younger one kept; a stray file the sweep finds → gone; a file present at start → gone | a laptop that died mid-land leaking 1 GiB of `/tmp` per bundle |
| `TestBundleRoutesTakeTheBearer` | each route with no bearer, or a wrong one → 401 (task 20's check) | a route registered outside the authentication middleware |
| `TestBundleLandAndRefreshThroughTheRoutes` | real git. Land: a scratch repository bundles one commit; `POST` it; `bundle verify` and `fetch` into `projects/a` through `POST /v1/git`; `DELETE` → `refs/panacea/lanes/x/main` in `projects/a` is that commit. Refresh: `reserve`; `bundle create` from `projects/a`; `GET` → `git bundle verify` in the scratch repository passes; `DELETE` | a break anywhere in ruling 3's two sequences |

#### Steps

- [ ] **Step 1:** Confirm task 21 has landed (`git grep -n "func.*handleGit" -- control-plane/cmd/machined/` answers). Record in the commit message whether phase 3's S8-P3.4 has landed (`git grep -n "type Bundles interface" -- control-plane/internal/lanegit/`); it does not block this task. Confirm task 19 has landed with both of its bundle changes (`git grep -n "TestMachinePodTmpHasASizeLimit\|TestAgentProxyStreamsABundleUpToItsCap" -- control-plane/` prints two lines); if not, stop and report, because cloud lands above 16 MiB would fail with the proxy's `413`.
- [ ] **Step 2:** Wire types; tests, each shown red first; `bundles.go`; the admission in `git.go`; start and the sweep in `main.go`; all pass.
- [ ] **Step 3: Mutation check** (rule 7), each on a `$TEMP` copy, each turning the named test red:
  (1) `O_EXCL` dropped → `TestBundlePostStoresTheBodyAndAnswersAnID`;
  (2) the partial file kept on `413` → `TestBundlePostOverTheCapIs413AndLeavesNoFile`;
  (3) the body deadline removed → `TestBundlePostSlowBodyIs504`;
  (4) `MaxLiveBundles` counting files only → `TestBundleThirdLiveIDIs429`;
  (5) the argument check made a prefix match on `<OwnedRoot>/bundles/` → `TestBundleArgumentIsExactlyALivePath`, on its dead-id and `../clone` rows;
  (6) the op list opened to any subcommand → `TestBundleArgumentIsExactlyALivePath`, on its `status` row;
  (7) the sweep's stray-file pass removed → `TestBundleSweepRemovesStaleAndStartEmpties`.
  The two named beforehand as likeliest to survive are (4), because a reserve alone makes no
  file, and (7), because every id the store made is also swept by age. Each is killed only by
  the row named beside it: one stored and one reserved id, and a file the store never made.
- [ ] **Step 4:** Full run; commit.

#### Acceptance

```bash
go test ./control-plane/cmd/machined/ -run '^TestBundle' -race -count=1 -v > "$TMPDIR/t21b.log" 2>&1; echo "exit=$?"   # exit=0
grep -c "^--- PASS" "$TMPDIR/t21b.log"                                                               # 10 (or SKIP lines without git)
go test ./control-plane/cmd/machined/ -run '^(TestGit|TestLaneFS)' -race -count=1                    # ok: task 21 still green
go test ./control-plane/... -race -count=1                                                           # ok
```

#### What this task does not claim

- The laptop's half: `lanegit.Bundles` over these routes and the relay from the lane's runner
  (task 25b), or phase 3's S8-P3.4 seam (an ask, **UNVERIFIED**).
- That the relay works through the broker today: task 19's proxy caps request bodies at 16 MiB
  (above). Or a broker-side splice, which ruling 3 defers.
- That `/tmp` is node disk and not guest memory under Kata (ruling 3; **UNVERIFIED**, task 32's
  probe). If it is memory, two 1 GiB bundles cannot fit a pod limited to 1 GiB (the plan's P8-D6).
- That a bundle is safe to fetch: that is phase 3's `FetchLane` checks, run by the laptop over
  `POST /v1/git` (phase-3 `task-24.md:748-758` and `:760` on). This task stores bytes and
  names a path.
