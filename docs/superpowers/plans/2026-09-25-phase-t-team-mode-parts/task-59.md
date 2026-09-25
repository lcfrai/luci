### Task 59: Phase 3 delta — the forecast as a library; the mirror runner and its typed result

**Slice:** TM2. **Focused time:** 3 h. **dependsOn:** 58 (the mirror pod, its `/control` port, its
disk at `/mirror`), P3-08 (the chained forecast, **UNVERIFIED until merge**: phase 3's
`control-plane/internal/converge/forecast.go` is planned, not built, `phase-3 …/task-08.md:15`),
and through P3-08, P3-07 (`ParseMergeTree`, `LaneObservation`), P3-24 (`lanegit.FetchLane`'s host
half) and P3-02 (`gitx.Exec` and its §HGH hardening). Built here after `feat/phase-3-git`'s
sync-merge. **Seams:** defines **ST.7**, the mirror's typed result, consumed by tasks 60 and 61.

Tier M is §8.2's third tier: the exact answer of `git merge-tree --write-tree` for two lanes of
different owners. Phase 3 already runs merge-tree for one person's land queue, in that person's
project repository (the chained forecast, task 08). A team project has no such repository: its
lanes live in the mirror (task 58). This task does two things. It lifts phase 3's forecast out of
`converge` into a package with no dependency beyond `gitx`, so the owner's control plane runs it
for a solo project exactly as before and the mirror runs the same code for a team one. And it
writes the mirror's runner, which takes lane bundles in, checks them as phase 3 checks a lane
bundle, and answers Fleet with **only** a typed result: tiers, paths, line ranges and shas. No
byte of file content leaves the mirror.

**Decisions where the scope is silent or the sources disagree, made here.**

**(a) The runner is built from the `control-plane` module, not `sandboxd`.** The skeleton's file
map puts it at `sbx/cmd/panacea-mirror`. But the two things it must run are control-plane code:
phase 3's §HGH hardening is `gitx.Exec` (`phase-3 …/task-02.md:19`, `:454-512`), and the forecast
is `converge` (`task-08.md:15`). `sandboxd` cannot import either (the skeleton's file map: the
modules never import each other), and a second copy of §HGH is exactly the drift phase 3 forbids
(*"This is the rule's only copy"*, `task-02.md:456-457`). So the runner is
`control-plane/cmd/panacea-mirror`. It runs no harness and no PTY, so it needs nothing of
`sandboxd` (task 58 decision (c)). Gap 1.

**(b) The library is a new package, `control-plane/internal/gitforecast`.** Phase 3's `converge`
also holds the convergence GET and its registry reads (`task-08.md:16`, `:130-188`); a mirror
linking all of that would carry code it never runs. `gitforecast` holds `ForecastQueue`, `Forecast`,
`UndoneLand`, `CarriedUndone`, `ParseMergeTree` and `MergeTreeArgs`, and imports only `gitx`.
`converge` keeps every exported name as a type alias or a one-line wrapper, so **no phase 3
caller or test changes**; `LaneObservation` stays in `converge` (it carries the sweep's state) and
is adapted to `gitforecast.Lane{ID, Tip, Synced, Changed}` in the wrapper. `forecast.go` becomes the
wrapper, and its cache stays in `converge`, where the solo control plane's 2-second reader needs it.

**(c) Tier M between two lanes is new code in the library, beside the queue.** The chained
forecast merges each lane into *the branch as it will be*. Tier M asks a different question: *do
these two lanes, as they are now, conflict with each other?* `PairForecast` runs `merge-tree
--write-tree --name-only -z <tipA> <tipB>` (merge base found by git), then, for each path both
sides changed, `diff --unified=0` of each side against the merge base, and intersects the hunk
headers into line ranges **on the merge base's blob** (§8.2's tier H unit, so H and M ranges
compare). It reads content lines only to skip them; only `@@` headers are parsed.

**(d) The mirror's base arrives by bundle, never by fetch from a remote** (task 58 decision (b)).
When a lane bundle's prerequisite is missing, `bundle verify` fails before any fetch (phase 3's
step 3, `task-24.md:767-768`); the runner then lists the missing shas and answers `need-base`
with them, and Fleet asks the lane owner's `machined` for a base bundle of the default branch
(task 60). The base lands at `refs/panacea/base/<branch>`, through the same verify-and-fsck path.

**(e) Nothing under `refs/panacea/*` is ever pushed.** The mirror has no egress (task 58), so it
cannot push at all. The scope's rule is kept at the other end too: task 61's integration head is
handed to the lane owner's `machined` as a bundle of `refs/heads/<branch>` only, and phase 3's push
(task 13) pushes one named branch. `gitforecast` never names a remote.

**Files:**
- `control-plane/internal/gitforecast/{forecast.go,pair.go,ingest.go}` — the library: the chained
  forecast moved from `converge`, `PairForecast`, `Ingest` (new)
- `control-plane/internal/gitforecast/{forecast,pair,ingest}_test.go` (new; the queue tests move
  here from `converge/forecast_test.go` unchanged)
- `control-plane/internal/converge/forecast.go` — aliases and the `LaneObservation` adapter; the
  cache stays (modify, **phase 3 task 08's file, UNVERIFIED until merge**)
- `control-plane/internal/converge/{mergetree.go}` — `MergeTreeArgs` and `ParseMergeTree` become
  aliases of `gitforecast`'s (modify, phase 3 task 07's file)
- `control-plane/internal/lanegit/bundle.go` — `FetchLane`'s steps 3 to 6 become a call to
  `gitforecast.Ingest` (modify, phase 3 task 24's file, **UNVERIFIED until merge**)
- `control-plane/internal/fleet/mirror/{result.go,client.go}` — ST.7 and Fleet's client (new)
- `control-plane/internal/fleet/mirror/{result,client}_test.go` (new)
- `control-plane/cmd/panacea-mirror/{main.go,serve.go,repo.go}` — the runner (new; gap 1)
- `control-plane/cmd/panacea-mirror/serve_test.go` (new)
- `testdata/mirror/v1/*.json` — ST.7's golden fixtures, read by both packages' tests (new)

**Interfaces:**
- Produces: `gitforecast.{Lane, Forecast, UndoneLand, ForecastQueue, CarriedUndone, MergeTreeArgs,
  ParseMergeTree, Pair, PairResult, PathLines, PairForecast, Ingest, IngestSpec, ErrNeedBase}`;
  ST.7 in `mirror.{LaneKey, OwnerHash, PairRequest, PairResult, PathResult, IngestResult, Code*,
  Validate}`; `mirror.Client` with `Ingest`, `Pairs`, `PruneOwner`, `Base`; the runner's wire
  (below).
- Consumes, **written against these names; where the owning task spells one differently, it wins
  and this part is renamed mechanically**:
  - phase 3 task 08: `ForecastQueue`, `Forecast`, `UndoneLand`, `CarriedUndone`, the fixed-identity
    forecast commit (`task-08.md:24-97`);
  - phase 3 task 07: `LaneObservation`, `MergeTreeArgs`, `ParseMergeTree` (`task-07.md:225-283`);
  - phase 3 task 24: `FetchLane`'s six steps, `BundleRefused`, `MaxBundleBytes`
    (`task-24.md:704-781`);
  - phase 3 task 02: `gitx.Exec`, `gitx.Machine`, `Call`, `Scope`, `Hardening`, the classes `Read`
    and `Local` (`task-02.md:19`, `:77-124`);
  - task 58: `sandbox.MirrorMountPath`, `sandbox.MirrorPort`, the broker's `GET
    /v1/mirrors/{projectId}/control` relay;
  - task 02: `teamtext`'s repo-relative path validation and the 200-character cap;
  - task 09: `model.Owner`.
- Consumed by: task 60 (tier M, the bundle sender), task 61 (the land queue over the mirror),
  task 63 (teammates' lanes, paths and line counts), task 44 (`PruneOwner`).

#### `gitforecast` — the public surface (complete)

```go
// Package gitforecast is phase 3's merge forecast as a library (Phase T task 59).
// It runs git through gitx only, in a repository the caller owns (Scope Project,
// task-02 §HGH): the user's project for a solo project, the mirror's bare
// repository for a team one. It never names a remote.
package gitforecast

import (
	"context"
	"io"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
)

// Lane is one lane as the forecast needs it. Tip is the lane ref's sha.
type Lane struct {
	ID      string   // a phase 3 worktree id, or "<owner-hash>/<wt-id>" in the mirror
	Tip     string   // "" when never synced
	Synced  bool
	Changed []string // committed paths, base..lane, raw UTF-8
}

// Forecast, UndoneLand: moved from converge unchanged (task-08.md:31-58).
type Forecast struct { /* … task 08's fields, byte for byte … */ }
type UndoneLand struct { /* … */ }

// MergeTreeArgs and ParseMergeTree: moved from converge unchanged (task-07.md:268-283).
var MergeTreeArgs = []string{"merge-tree", "--write-tree", "--name-only", "-z"}

func ParseMergeTree(stdout []byte, exitCode int) (tree string, conflicted []string, err error)
func CarriedUndone(ctx context.Context, m gitx.Machine, repo, laneTip, ownID string) ([]UndoneLand, error)

// ForecastQueue is task 08's walk, over Lane instead of LaneObservation. With no
// cache: the caller keeps one if it polls (converge does; the mirror answers
// one request per call).
func ForecastQueue(ctx context.Context, m gitx.Machine, repo, branchTip string, queue []Lane) []Forecast
```

`converge/forecast.go` keeps `func ForecastQueue(ctx, m, projectPath, branchTip string, queue
[]LaneObservation) []Forecast` with its cache, converting each observation to a `Lane` (its
`Changed[i].Path`), and `type Forecast = gitforecast.Forecast`. Every phase 3 test of the queue
moves to `gitforecast/forecast_test.go` with only the `Lane` literal changed; `converge` keeps
`TestForecastCacheSkipsGit` (`task-08.md:280`), because the cache stays there.
