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
  `MaxBundleBytes` plus clone scratch). Task 19's pod row (`task-19.md:26`) gives `emptyDir`
  `/tmp` with **no** `sizeLimit` today, so task 19 must gain it before this task ships;
- **task 19's agent proxy streaming a bundle body.** Ruling 3 says the proxy reaches these
  routes "streamed and unbuffered", but `task-19.md:92-93` caps **request** bodies at 16 MiB;
  only responses stream. `GET` works as written. `POST /v1/bundles` needs task 19 to admit up
  to `MaxBundleBytes` on that one path, streamed, with `BundleTimeout` + 5 s as its deadline
  (ruling 3's relay deadline, `:155-158`). Until it does, a land from a cloud lane above 16 MiB
  fails with the proxy's `413`;
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
