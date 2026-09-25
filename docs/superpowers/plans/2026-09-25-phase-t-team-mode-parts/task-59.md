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

#### `pair.go` (complete surface and rule)

```go
// Pair is two lane tips to merge against each other (tier M, decision (c)).
type Pair struct{ A, B string } // shas the repository holds

// PathLines is one path both sides changed. Lines are [start, end] on the
// merge base's blob, 1-based, inclusive, merged, at most MaxRanges.
type PathLines struct {
	Path       string
	Conflicted bool     // merge-tree reported it conflicted
	BaseBlob   string   // the merge base's blob sha for Path, "" when absent there
	Lines      [][2]int // overlapping hunks of A and B against the base
}

// PairResult is PairForecast's answer. State: "clean" | "conflicts" | "unknown".
type PairResult struct {
	State     string
	MergeBase string
	Paths     []PathLines // every path both sides changed, conflicted or not, sorted
	Code      string      // "" or one of the Code* constants when State is "unknown"
}

const MaxRanges = 20 // route 57's cap (task-06.md:203-205)

// PairForecast, all Class Read or Local in repo, Scope Project:
//   1. merge-base <A> <B>                          → MergeBase; none → unknown, CodeNoMergeBase
//   2. MergeTreeArgs + <A> <B>                     → ParseMergeTree; exit 0 clean, 1 conflicts
//   3. diff --name-only -z --no-renames <base> <A>, and <base> <B>   → the two changed sets
//   4. for each path in both sets (sorted):
//        rev-parse --verify -q <base>:<path>       → BaseBlob ("" on exit 1)
//        diff --unified=0 --no-color --no-ext-diff --no-textconv <base> <X> -- <path>, X = A, B
//        → only the "@@ -s[,c] +…" headers are parsed, into [s, s+c-1] on the base (c = 0 is
//          an insertion after line s, recorded as [s, s]); every other line is skipped
//        → Lines = the overlap-or-adjacency intersection of A's and B's ranges (task 36's
//          rule (a), overlaps), merged, capped at MaxRanges as task 36's mergeRanges does
//   A path with Conflicted false and Lines empty is still listed: both sides changed it,
//   and task 60 decides what that means. No content line is kept, returned or logged.
func PairForecast(ctx context.Context, m gitx.Machine, repo string, p Pair) PairResult
```

`--no-renames` on step 3 keeps a rename from making one path two; `--literal-pathspecs` is already
in §HGH H1's prefix (`task-02.md:481`), so a path such as `app/[slug]/page.tsx` in step 4 is not
read as a glob. Both `--no-ext-diff` and `--no-textconv` matter in a repository whose trees an
attacker writes: a lane's `.gitattributes` can name a diff driver, and while the mirror's config
defines none (so git would fall back to its built-in diff), the two flags make that fact
irrelevant rather than relied on.

#### `ingest.go` (complete surface)

```go
// IngestSpec is one bundle file to take into repo.
type IngestSpec struct {
	File     string   // a bundle the caller spooled, at most MaxBundleBytes (task 24)
	Refs     []string // exactly the refnames the bundle must carry, each refs/heads/…
	Refspecs []string // "+<ref>:<dst>", each dst under DstPrefix
	DstPrefix string  // "refs/panacea/lanes/" (phase 3) or the mirror's two prefixes
}

// ErrNeedBase is Ingest's answer when bundle verify fails for a missing
// prerequisite. Missing is every prerequisite sha the bundle header names
// ("-<sha>" lines) that `cat-file -e <sha>^{commit}` does not find. No ref
// has moved.
type ErrNeedBase struct{ Missing []string }

func (e *ErrNeedBase) Error() string { return "gitforecast: the bundle needs commits the repository lacks" }

// Ingest is FetchLane's steps 3 to 6 (task-24.md:767-781), unchanged in
// order and in every flag: bundle verify; list-heads must equal Refs exactly;
// fetch --atomic --no-auto-maintenance --no-recurse-submodules --no-tags
// --no-write-fetch-head --quiet <file> <refspecs…> with transfer.fsckObjects=true
// in Call.Env; read back with for-each-ref. It answers dst → sha, an
// *ErrNeedBase, or a *BundleRefused carrying git's sentence.
func Ingest(ctx context.Context, git *gitx.Exec, repo string, s IngestSpec) (map[string]string, error)
```

`lanegit.FetchLane` keeps its signature and its steps 1 and 2 (arguments, spooling from the
runner) and calls `Ingest` for the rest; `BundleRefused` moves to `gitforecast` with a `lanegit`
alias, so phase 3's `errors.As` checks still match. The `need-base` split is new: phase 3's step 3
answers a missing prerequisite as a refusal, and it still does for `FetchLane` (it unwraps
`*ErrNeedBase` into `*BundleRefused{"the bundle needs commits the project lacks"}`), because
phase 3 fixed that answer and its callers rely on it. The prerequisite shas come from the bundle
header, which `bundle list-heads` does not print. When `bundle verify` exits non-zero, `Ingest`
reads the header itself (the v2 or v3 signature line, `@` capability lines, `-<sha>` prerequisite
lines, `<sha> <ref>` lines, then a blank line; bounded to the first 64 KiB), keeps each
prerequisite sha of 40 or 64 hex, and asks `cat-file -e` for each; if at least one is missing the
answer is `*ErrNeedBase`, and otherwise it is `*BundleRefused` with git's sentence. git's stderr is
never parsed for meaning. That `bundle verify` exits non-zero for a missing prerequisite is phase 3's
pin (`task-24.md:948`).

#### ST.7 — `control-plane/internal/fleet/mirror/result.go` (complete)

```go
// Package mirror is Fleet's side of the lane mirror (Phase T tasks 59-61):
// ST.7's typed results, their validation, and the client over the broker's
// control relay. Nothing here ever holds file content.
package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/teamtext"
)

// LaneKey names one lane in the mirror: refs/panacea/lanes/<Owner>/<Worktree>.
type LaneKey struct {
	Owner    string `json:"owner"`    // OwnerHash(owner): 16 lower-case hex
	Worktree string `json:"worktree"` // a phase 3 worktree id
}

// OwnerHash is the lane namespace for an owner. Not reversible; Fleet keeps
// the map back to model.Owner in its own state.
func OwnerHash(o model.Owner) string {
	sum := sha256.Sum256([]byte("lane-owner/" + string(o)))
	return hex.EncodeToString(sum[:])[:16]
}

func (k LaneKey) Ref() string { return "refs/panacea/lanes/" + k.Owner + "/" + k.Worktree }

// PairRequest asks tier M for two lanes of different owners.
type PairRequest struct {
	A LaneKey `json:"a"`
	B LaneKey `json:"b"`
}

// PathResult is one path both lanes changed. No field can hold content:
// a validated path, a sha, and integers.
type PathResult struct {
	Path       string   `json:"path"`
	Conflicted bool     `json:"conflicted"`
	BaseBlob   string   `json:"baseBlob"` // "" when the path is new on both sides
	Lines      [][2]int `json:"lines"`    // on the merge base's blob; ≤ 20
}

// Basis is what the answer was computed from, so a stale answer is detectable.
type Basis struct {
	A         string `json:"a"`
	B         string `json:"b"`
	MergeBase string `json:"mergeBase"`
}

// PairResult is ST.7: tier, paths, line ranges, basis shas. Code is from the
// fixed set below, never a sentence.
type PairResult struct {
	A     LaneKey      `json:"a"`
	B     LaneKey      `json:"b"`
	Tier  string       `json:"tier"`  // always "M"
	State string       `json:"state"` // clean | conflicts | unknown
	Paths []PathResult `json:"paths"`
	Basis Basis        `json:"basis"`
	Code  string       `json:"code"`
}

// IngestResult answers one bundle.
type IngestResult struct {
	Lane    LaneKey  `json:"lane"`
	State   string   `json:"state"`   // fetched | need-base | refused
	Tip     string   `json:"tip"`     // the lane ref after the fetch, when fetched
	Missing []string `json:"missing"` // need-base: prerequisite shas, ≤ 16
	Code    string   `json:"code"`
}

const (
	CodeNone           = ""
	CodeUnverified     = "bundle_unverified"  // bundle verify failed, prerequisites present
	CodeFsck           = "fsck_failed"        // the fsck'd fetch refused an object
	CodeHeads          = "heads_mismatch"     // list-heads is not exactly the lane's ref
	CodeTooLarge       = "bundle_too_large"
	CodeNoMergeBase    = "no_merge_base"
	CodeLaneMissing    = "lane_missing"       // a PairRequest named a lane the mirror lacks
	CodeGitFailed      = "git_failed"
)

var (
	shaRE   = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	ownerRE = regexp.MustCompile(`^[0-9a-f]{16}$`)
	wtRE    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	codes   = map[string]bool{CodeNone: true, CodeUnverified: true, CodeFsck: true, CodeHeads: true,
		CodeTooLarge: true, CodeNoMergeBase: true, CodeLaneMissing: true, CodeGitFailed: true}
)

func (k LaneKey) Validate() error {
	if !ownerRE.MatchString(k.Owner) || !wtRE.MatchString(k.Worktree) {
		return fmt.Errorf("mirror: bad lane key")
	}
	return nil
}

// Validate is what Fleet runs on every answer before any field is used. The
// mirror runs code over hostile objects, so its answers are hostile too: an
// answer that fails is dropped whole, and the pair is treated as unknown.
func (r PairResult) Validate(asked PairRequest) error {
	if r.A != asked.A || r.B != asked.B || r.Tier != "M" || !codes[r.Code] {
		return fmt.Errorf("mirror: answer does not match the question")
	}
	switch r.State {
	case "clean", "conflicts":
		if r.Code != CodeNone || !shaRE.MatchString(r.Basis.A) || !shaRE.MatchString(r.Basis.B) ||
			!shaRE.MatchString(r.Basis.MergeBase) {
			return fmt.Errorf("mirror: a %s answer needs three basis shas and no code", r.State)
		}
	case "unknown":
		if r.Code == CodeNone || len(r.Paths) != 0 {
			return fmt.Errorf("mirror: an unknown answer carries a code and no paths")
		}
	default:
		return fmt.Errorf("mirror: state %q", r.State)
	}
	if len(r.Paths) > 500 {
		return fmt.Errorf("mirror: %d paths", len(r.Paths))
	}
	conflicted := false
	for _, p := range r.Paths {
		if err := teamtext.ValidPath(p.Path); err != nil { // task 02: repo-relative, ≤ 200, G3 hygiene
			return fmt.Errorf("mirror: path refused: %w", err)
		}
		if p.BaseBlob != "" && !shaRE.MatchString(p.BaseBlob) {
			return fmt.Errorf("mirror: base blob")
		}
		if len(p.Lines) > 20 {
			return fmt.Errorf("mirror: %d ranges", len(p.Lines))
		}
		for _, l := range p.Lines {
			if l[0] < 1 || l[1] < l[0] || l[1] > 10_000_000 {
				return fmt.Errorf("mirror: range")
			}
		}
		conflicted = conflicted || p.Conflicted
	}
	if (r.State == "conflicts") != conflicted {
		return fmt.Errorf("mirror: state and conflicted paths disagree")
	}
	return nil
}
```

`IngestResult.Validate(asked LaneKey)` is the same discipline: the lane matches, `State` is one of
three, `Tip` is a sha exactly when `fetched`, `Missing` holds only shas and at most 16, `Code` is
from the set. `teamtext.ValidPath` is task 02's path check (`task-02.md:77`, `:847`), which takes
glob metacharacters literally (`:623`). The JSON decoder on every answer uses
`DisallowUnknownFields`, so a field the mirror adds, a place content could hide, is a refusal.
The 500-path cap is this part's: a pair that both changed more than 500 paths is a rewrite, not a
conflict to forecast, and the runner answers it `unknown` with `CodeGitFailed` rather than send it.

#### The runner's wire (complete): one WebSocket per request, through the broker's relay

`panacea-mirror` listens on `:8080` (task 58's `MirrorPort`) and serves exactly `GET /control`
and `GET /healthz`. `/control` needs `Authorization: Bearer <PANACEA_APP_CONTROL_TOKEN>`, compared
in constant time; the broker's relay adds it (task 17, `task-17.md:342`). One upgrade carries one
request and its answer, then the runner closes it, so no state spans requests. Frames, with
`github.com/coder/websocket` (global constraint), read limit 64 KiB for text frames:

```go
// The first text frame of every request.
type Request struct {
	Op    string        `json:"op"`              // ingest | pairs | prune-owner
	Lane  *LaneKey      `json:"lane,omitempty"`  // ingest
	Kind  string        `json:"kind,omitempty"`  // ingest: "lane" | "base"
	Head  string        `json:"head,omitempty"`  // ingest: the one ref the bundle carries, refs/heads/…
	Size  int64         `json:"size,omitempty"`  // ingest: the bundle's bytes, ≤ MaxBundleBytes
	Pairs []PairRequest `json:"pairs,omitempty"` // pairs: ≤ 32 per request
	Owner string        `json:"owner,omitempty"` // prune-owner: an OwnerHash
}
// ingest: then binary frames totalling exactly Size bytes, then the answer
//         (one text frame): IngestResult.
// pairs:  the answer is one text frame: {"results": [PairResult, …]}, one per
//         pair, in order.
// prune-owner: {"deleted": n}.
// Any refusal before work: {"error": "<code>"} and close 1008.
```

**What each op does in the runner**, in `/mirror/lanes.git`, a bare repository `repo.go` creates
on first start with `git init --bare` and three config keys it writes itself and never reads
back from a lane: `gc.auto=0`, `receive.fsckObjects=true`, `transfer.fsckObjects=true`:

- `ingest`, kind `lane`: spool to `/mirror/spool/<16 random hex>.bundle` (`O_EXCL`, `0600`, removed
  by a defer), refusing byte `Size+1` or `MaxBundleBytes+1`; `head` must begin `refs/heads/` and
  pass `check-ref-format`; `gitforecast.Ingest` with `Refs = [head]`, `Refspecs = ["+" + head +
  ":" + lane.Ref()]`, `DstPrefix = "refs/panacea/lanes/<owner>/"`. The bundle is exactly what
  phase 3's runner makes for a lane (one `refs/heads/<branch>` ref, `task-24.md:713-717`), so
  task 60 needs no new bundle shape. `*ErrNeedBase` → `need-base` with `Missing`;
  `*BundleRefused` → `refused` with the code its step maps to; success → `fetched`, `Tip`.
- `ingest`, kind `base`: the same, with `Refs = [head]`, `Refspecs = ["+" + head +
  ":refs/panacea/base/" + <head without refs/heads/>]`, `DstPrefix = "refs/panacea/base/"`; `lane`
  is absent. A base bundle is as hostile as a lane's (a member's clone is agent-writable, phase 3's
  R4.2), so it takes the same path.
- `pairs`: for each pair, both lanes must exist (`rev-parse --verify -q <ref>`), else
  `unknown`/`lane_missing`; then `gitforecast.PairForecast` over the two tips; the result is mapped
  field for field into ST.7 and **validated by the runner with the same `Validate` before it is
  sent**, so a runner bug fails in the mirror, not in Fleet.
- `prune-owner`: `for-each-ref --format=%(refname) refs/panacea/lanes/<owner>/`, then one
  `update-ref --stdin` transaction deleting each; then `git gc --prune=now --quiet`, so the leaver's
  objects that no other ref reaches leave the disk, not just the refs (task 58 decision (f)).

Every git call goes through `gitx.Exec` with `Scope: Project` on `/mirror/lanes.git` and a
`Hardening` the runner builds once: `HooksDir` an empty read-only directory in the image
(`/opt/panacea-mirror/no-hooks`), no `Credential`, no `Proxy`. That `gitx.Exec` accepts a bare
repository as a Project directory is **UNVERIFIED until merge** (phase 3 only ever runs it in a
user's checkout); step 1's preflight reads `harden.go`, and if it refuses one, `repo.go` keeps a
non-bare repository with no working-tree checkout (`core.bare=false`, never `checkout`), which
§HGH covers as written.

`panacea-mirror --version` prints `git --version`'s line from `/etc/panacea-mirror-git-version`
(task 58's image) and its own build commit; `main.go` logs one `slog` JSON line per request with
`op`, `lane` or pair count, `state`, `code` and `ms`, and **never a path**: a path is the lane
owner's, and the mirror's logs are an operator's.

#### `client.go` — Fleet's side (complete surface)

```go
// Dialer opens one control WebSocket to a project's mirror, through the
// broker's GET /v1/mirrors/{projectId}/control (task 58), as Fleet.
type Dialer interface {
	Dial(ctx context.Context, projectID string) (*websocket.Conn, error)
}

type Client struct {
	D       Dialer
	Timeout time.Duration // per request; default 60 s
}

func (c *Client) Ingest(ctx context.Context, projectID string, lane *LaneKey, kind, head string,
	bundle io.Reader, size int64) (IngestResult, error) // lane nil for kind "base"
func (c *Client) Pairs(ctx context.Context, projectID string, pairs []PairRequest) ([]PairResult, error)
func (c *Client) PruneOwner(ctx context.Context, projectID, ownerHash string) (int, error)
```

`Pairs` validates every result against its request (`PairResult.Validate`) and replaces one that
fails with `{State: "unknown", Code: "git_failed"}` for that pair alone, logging `mirror answer
refused` with the pair's lane keys and the validation error, never the answer's bytes. A
`len(results) != len(pairs)` answer is an error for the whole call.

- [ ] **Step 1: Preflight** (after the phase 3 sync-merge)

```bash
cd control-plane
grep -n "func ForecastQueue\|func ParseMergeTree\|var MergeTreeArgs" internal/converge/*.go   # task 07/08 merged
grep -n "func FetchLane\|type BundleRefused\|MaxBundleBytes" internal/lanegit/*.go            # task 24 merged
grep -n "type Scope\|ScopeProject\|ErrLaneDir" internal/gitx/harden.go                         # read: does Project admit a bare repo?
git --version                                                                                  # ≥ task-02's MinVersion; merge-tree --write-tree needs 2.38
```

- [ ] **Step 2: Write the failing tests**

Real git in `t.TempDir()` repositories, as phase 3's git tests do (task 01's behaviour pins); a
helper `twoLanes(t)` builds a bare mirror with a base commit and two lanes of different owners.

| Test | Asserts | Mutant it kills |
|---|---|---|
| gitforecast · `TestQueueTestsPassUnchanged` | phase 3's `forecast_test.go` cases, moved, all pass against `gitforecast.ForecastQueue` | the move changing behaviour |
| converge · `TestForecastCacheSkipsGit` | unchanged, still in `converge` | the cache lost in the move |
| gitforecast · `TestPairConflictHasBaseLines` | both lanes rewrite lines 10-12 of `a.go` → `conflicts`; `a.go` has `Conflicted`, `BaseBlob` = the base's blob, `Lines == [[10,12]]` | ranges on the merged file or on one side, not the base |
| gitforecast · `TestPairCleanOverlapIsListed` | A edits line 3, B edits line 40 of `b.go` → `clean`, `b.go` listed with `Conflicted: false` and `Lines == []` | dropping paths both changed when the text merges (task 60 needs them) |
| gitforecast · `TestPairAdjacentLinesOverlap` | A edits 10, B edits 11 → `Lines == [[10,11]]` | strict intersection, missing task 36's rule (a) |
| gitforecast · `TestPairInsertionIsAPoint` | A inserts after line 5 (`@@ -5,0 …`), B edits line 5 → `[[5,5]]` | a zero-count hunk read as the empty range |
| gitforecast · `TestPairIgnoresDiffDrivers` | both lanes add `.gitattributes` `*.go diff=evil`; the test itself sets `diff.evil.command` in the repository's config to a script that writes a marker file (the worst case: a driver defined); after `PairForecast` the marker does not exist | `diff` run without `--no-ext-diff`/`--no-textconv`, relying on the config having no driver |
| gitforecast · `TestPairNoMergeBase` | two unrelated roots → `unknown`, `no_merge_base` | a merge-tree over unrelated histories reported as conflicts |
| gitforecast · `TestIngestRefusesAnUnverifiedBundle` | a truncated bundle → `*BundleRefused`, no ref under `refs/panacea/` moved | fetching before `bundle verify` |
| gitforecast · `TestIngestReportsNeedBase` | a lane bundle whose prerequisite the mirror lacks → `*ErrNeedBase{Missing: [that sha]}`; then a base bundle, then the lane bundle → fetched | a missing base treated as a hostile bundle, so no team project ever forecasts |
| gitforecast · `TestIngestRefusesACorruptObject` | a bundle holding a tree with a duplicate entry (phase 3's fsck fixture, `task-24.md:952`) → `*BundleRefused` with fsck's sentence, no ref moved | `transfer.fsckObjects` not in `Call.Env` |
| gitforecast · `TestIngestHeadsMustMatch` | a bundle carrying two heads, or `HEAD` → refused | a bundle that writes a second ref |
| lanegit · `TestFetchLaneUnchanged` | phase 3's `FetchLane` tests pass; a missing prerequisite is still `*BundleRefused` for `FetchLane` | the need-base split leaking into phase 3's answer |
| mirror · `TestResultCarriesNoContent` | the golden `testdata/mirror/v1/pair-conflict.json` decodes with `DisallowUnknownFields`, and a copy with an added `"hunk": "…"` field is refused | an answer shape with room for content |
| mirror · `TestValidateRefusesAPathWithANewline` | a `path` of `"a.go\n+secret"` → refused, and `Pairs` turns that one result into `unknown` | a path used to smuggle a line of content |
| mirror · `TestValidateStateMustMatchPaths` | `state: "clean"` with a conflicted path → refused | a runner that says clean over a conflict |
| mirror · `TestValidateNeedsTheAskedPair` | the answer names lanes other than those asked → refused | answers mismatched to questions |
| runner · `TestUnverifiedBundleIsDroppedAndReported` | over a real WebSocket to `serve` with a `twoLanes` repo: a truncated bundle → `{"state":"refused","code":"bundle_unverified"}`, the spool directory is empty after | a refused bundle left on the disk |
| runner · `TestControlNeedsTheBearer` | no token, a wrong token, a right token → 401, 401, 101 | an open control route |
| runner · `TestPruneOwnerRemovesRefsAndObjects` | a leaver's lane holding a unique blob → after `prune-owner`, `cat-file -e <blob>` fails; another owner's lane is untouched | refs deleted and the leaver's objects kept on disk |
| runner · `TestLogsHoldNoPath` | every log line for a pairs request over `secret-plan.md` lacks the string `secret-plan` | a path in the operator's logs |

- [ ] **Step 3: The library.** Move task 08's code into `gitforecast`, add `pair.go` and
  `ingest.go`, write the `converge` aliases and the `lanegit` call. `go test ./internal/gitforecast/
  ./internal/converge/ ./internal/lanegit/ -count=1` → `ok` three times.

- [ ] **Step 4: ST.7 and the client.** `result.go`, `client.go`, the goldens. `go test
  ./internal/fleet/mirror/ -count=1` → `ok`.

- [ ] **Step 5: The runner.** `cmd/panacea-mirror`. `go test ./cmd/panacea-mirror/ -count=1` → `ok`;
  `CGO_ENABLED=0 go build -o ../images/mirror/panacea-mirror ./cmd/panacea-mirror` → a static
  binary for task 58's image.

- [ ] **Step 6: Acceptance**

```bash
cd control-plane
go vet ./internal/gitforecast/ ./internal/converge/ ./internal/lanegit/ ./internal/fleet/mirror/ ./cmd/panacea-mirror/
go test ./internal/gitforecast/ ./internal/converge/ ./internal/lanegit/ ./internal/fleet/mirror/ ./cmd/panacea-mirror/ -count=1 -race
go list -deps ./cmd/panacea-mirror | grep -c "k8s.io\|internal/hub\|internal/api"   # 0: the mirror links no cluster, Hub or API code
```

**Deferred to task 69:** the runner inside the real mirror pod, fed by a real `machined`; its
memory under a large repository's `merge-tree`.

- [ ] **Step 7: Commit** by pathspec, one commit per step 3 to 5.

**What this task does NOT claim:**
- That line ranges on the merge base are what a reviewer would draw. They are hunk headers, the
  same unit as tier H; a rename splits a path in two (`--no-renames`).
- That `gitx.Exec` runs in a bare repository unchanged (**UNVERIFIED until merge**; the fallback is
  in the wire section).
- That fsck catches every hostile object. It catches what git's fsck checks; a parser bug in
  `merge-tree` itself is contained by the pod (task 58), not by this task.
- That the solo control plane's forecast is faster or slower: it is the same code behind aliases.
- That the mirror is fed. Task 60 sends the bundles.

**Gaps found in the skeleton and in sibling parts:**
1. The file map puts the runner at `sbx/cmd/panacea-mirror/main.go`; decision (a) builds it from
   `control-plane/cmd/panacea-mirror`. Task 58's image note is updated to match.
2. The file map has no row for `control-plane/internal/gitforecast/`, nor for the edits to phase
   3's `converge`, `lanegit` and their tests.
3. The skeleton's scope says the mirror fetches the customer's remote; task 58 decision (b) and
   this task's decision (d) replace that with base bundles from `machined`.

