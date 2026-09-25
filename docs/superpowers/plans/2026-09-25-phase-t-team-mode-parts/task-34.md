### Task 34: Phase 3 delta — `machined`'s activity sweep (ST.6)

**Slice:** TM1, its first task; it starts only if task 33's verdict is PASS. **Focused time:** 3 h.
**dependsOn:** 17 (the broker's second caller, under a delegation), P3-07 (lane observation and
the `-z` parsers), P8-20 and P8-21 (`machined`, and its git through phase 3's §HGH). **Seam
defined:** ST.6, the activity sample; consumed by tasks 35, 36 and 60.

**Agents see facts, not words** (global constraint 6): what a teammate's agent is changing reaches
anyone else as git's own reading of its worktree, taken **outside the agent's VM**, never as the
agent's account of itself. This task builds that reading. `machined` runs in the machine pod, a
separate Kata pod that holds the disk (P8-D6, `.claude/worktrees/env/…/task-20.md:3-13`); the
agent's session pod mounts only its own worktree. Every 5 to 10 seconds, for each live worktree, it
takes one **sample**: the dirty paths, their line counts, the dirty hunks as line ranges on the
blob at `HEAD`, and the last commit's sha, subject and paths. Nothing it samples is what the agent
*read*: git does not know, and §3.3's activity row forbids "files read".

**Built after the sync-merge of `feat/phase-3-git` and `feat/env`.** None of the extended code
exists: phase 3 has no `converge` or `gitx` package yet (`task-07.md:19-26` and `task-02.md:19` of
`.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/`, plan
only), and `control-plane/cmd/` on `feat/env` holds `api` only (read 2026-09-25). Every path below
is **UNVERIFIED until merge**.

**Why the hardening is this task's first concern.** Phase 3 lets the agent write its clone's
`.git/config` and `.git/hooks` (R4.2), and `machined` holds the whole disk and the user's host
token (P8-21, `.claude/worktrees/env/…/task-21.md:89-98`). A `git status` that obeyed the clone's
`core.fsmonitor`, a `post-index-change` hook, a `diff.external` or a filter driver would run the
agent's code in the machine pod, every five seconds. So every call goes through phase 3's
`gitx.Exec` with §HGH's H1 prefix and H3's `Clone` refusal list, exactly as P8-21 runs every
other clone call (`task-21.md:100-105`), and adds the four switches a *read* needs: `--no-ext-diff`,
`--no-textconv`, `GIT_OPTIONAL_LOCKS=0` (status never rewrites the agent's index, so it runs no
index hook and races no agent `git add`), and `-c core.untrackedCache=false`. Command-line `-c`
outranks every config file, including one reached through `include.path`.

**Decisions made here.**

**(a) The library is phase 3's, the loop is `machined`'s.** `converge.Sample` (in `claims.go`, the
skeleton's name) is a pure function over a `gitx.Runner` and a clone directory, so the laptop's
control plane can run the same code for a solo project on a local lane, and phase 3 task 07's
`Observe` can later share its parsers. `machined` owns the timer, the ring buffer and the route.

**(b) "Live" is `machined`'s own observation.** A worktree whose last sample differed from the one
before, within the last 10 minutes, is **live** and swept every 5 s plus up to 5 s of jitter; any
other is swept every 60 s. No broker call tells `machined` which sessions run, so there is no new
broker-to-machine API, and a worktree whose session crashed is still read.

**(c) Pull, not push.** `machined` accepts ingress from the broker alone (P8-D12) and has no route
into the cluster, so it cannot send. It keeps the last two minutes of samples in a ring buffer and
answers `GET /v1/activity?project=&since=`. Fleet polls it through the broker **under the
project's delegation** (task 17), so the broker's delegation check is what keeps Fleet to team
projects: a solo project's activity is readable only with its owner's own token, which is how "to
the owner's control plane for a solo one" holds without `machined` knowing which projects are team.

**(d) Paths are refused at the source and counted.** Every path goes through `teamtext.ValidPath`
and `teamtext.Clean` (task 02: bidi, C0/C1, ANSI, tag block, secret shape, the 200-character cap,
`task-02.md:767-776`, `:845-847`) in `machined`, before it enters a sample. A refused path is left
out and counted in `dropped`, never escaped and passed on. Fleet checks again on receipt (task 35).

**(e) Errors are codes.** A sample's `readError` is one of five product codes, never git's text,
which can quote a path the hygiene would have refused.

**Files:**
- `control-plane/internal/machineapi/activity.go` — ST.6: `ActivitySample`, `DirtyPath`, `Hunk`,
  `LastCommit`, `ActivityPage`, the bounds and the error codes (new, in P8-20's package)
- `control-plane/internal/converge/claims.go` — `Sample`, the four reads, the hunk-header parser
  (new, in P3-07's package; the skeleton's name for it)
- `control-plane/internal/converge/claims_test.go` — real `git`, skipped with a message if absent
  (new)
- `control-plane/cmd/machined/sweep.go`, `sweep_test.go` — `Sweeper`: the cadence, the ring buffer,
  `GET /v1/activity` (new, in P8-20's command)
- `control-plane/cmd/machined/server.go` — the route (modified; P8-20's file)
- `control-plane/cmd/broker/machines.go` — the relay `GET /v1/machines/{m}/activity`, owner-scoped
  or delegation-scoped by project (modified; P8-18's machine routes, **UNVERIFIED** file name until
  merge)
- `testdata/activity/v1/{sample-dirty.json,sample-unborn.json,sample-untrusted.json}` — ST.6's
  fixtures, read by `machined`'s, the broker's and task 35's tests (new)

**Interfaces:**
- Produces:
  - ST.6: `machineapi.ActivitySample` and `ActivityPage` (below, complete)
  - `converge.Sample(ctx, r gitx.Runner, dir string, at time.Time) machineapi.ActivitySample`
  - `machined`'s `GET /v1/activity?project=<pj-…>&since=<seq>` → `200 ActivityPage`
  - the broker's `GET /v1/machines/{machineId}/activity?project=<pj-…>&since=<seq>` → the same body
- Consumes:
  - P3-02: `gitx.Runner`, `gitx.Call{Dir, Args, Class, Scope, Env}`, `gitx.Read`, `gitx.Clone`,
    `*gitx.UntrustedConfig`, `gitx.MachineHardening` (as P8-21 names them,
    `.claude/worktrees/env/…/task-21.md:31-52`, `:100-105`)
  - P3-07: `converge.ParseStatus` over `PorcelainV2Z` and the `PathChange.Status` vocabulary
    (`task-07.md:217-223` of phase 3)
  - P8-20: `machineapi.DirLanes`, the `lanes/<projectId>/<worktreeId>` layout (`task-20.md:40-45`),
    the bearer check on every route
  - task 02: `teamtext.ValidPath`, `teamtext.Clean`, `teamtext.MaxPath` (200)
  - task 17: the broker's delegation check, which yields the delegation's project
- Consumed by: task 35 (`fleet/activity` polls and folds samples), task 36 (hunks and base blobs),
  task 60 (lane bundles use the same cadence), and the laptop's control plane for a solo project
  (a later task; this one does not wire it).

#### Contract — `machineapi/activity.go` (complete; ST.6)

```go
package machineapi

import "time"

// The bounds a sample never exceeds. A sample that would is cut, oldest-changed
// paths last, and says Truncated.
const (
	MaxDirtyPaths   = 500
	MaxHunkPaths    = 50  // paths whose hunks are read in one sweep
	MaxRangesPerPath = 64
	MaxSubjectRunes = 140 // teamtext's title bound
	MaxLastCommitPaths = 200
	RingWindow      = 2 * time.Minute
)

// ReadError codes. Never git's text.
const (
	ReadErrUntrustedConfig = "clone_config_untrusted" // §HGH H3 refused the clone's config; git never ran
	ReadErrNotARepo        = "not_a_repository"
	ReadErrTimeout         = "timeout"                 // any one call over 5 s
	ReadErrGitFailed       = "git_failed"
	ReadErrUnreadable      = "unreadable"              // the directory could not be listed
)

// ActivitySample is one sweep of one worktree. Every path in it passed
// teamtext.ValidPath and teamtext.Clean in machined; a refused one is counted
// in Dropped and appears nowhere. It holds no file contents and no "read" set.
type ActivitySample struct {
	Seq        int64       `json:"seq"`        // per machine, strictly increasing across restarts (below)
	ProjectID  string      `json:"projectId"`  // pj-…, from the lane's path
	WorktreeID string      `json:"worktreeId"` // w-…
	At         time.Time   `json:"at"`         // when the sweep began, UTC
	Head       string      `json:"head"`       // 40-hex; "" on an unborn branch
	Dirty      []DirtyPath `json:"dirty"`      // sorted by path; never nil
	Hunks      []Hunk      `json:"hunks"`      // sorted by path; never nil
	LastCommit *LastCommit `json:"lastCommit"` // nil on an unborn branch
	Dropped    int         `json:"dropped"`    // paths refused by hygiene or the cap
	Truncated  bool        `json:"truncated"`  // a Max* bound cut something
	ReadError  string      `json:"readError,omitempty"` // a ReadErr* code; then Dirty, Hunks are empty
}

// DirtyPath is one path that differs from HEAD, staged or not, or is untracked.
type DirtyPath struct {
	Path    string `json:"path"`
	Status  string `json:"status"`  // P3-07's vocabulary: added|modified|deleted|renamed|typechange|untracked|unmerged
	From    string `json:"from,omitempty"` // a rename's old path, also cleaned
	Added   int    `json:"added"`   // numstat; -1 for a binary file
	Deleted int    `json:"deleted"` // numstat; -1 for a binary file
}

// Hunk is one path's dirty line ranges, on the blob that HEAD holds for it.
// Task 22's verbs.Hunk has the same three fields and tags (task-22.md:149-153).
type Hunk struct {
	Path   string   `json:"path"`
	Base   string   `json:"base"`   // HEAD's blob for Path, 40-hex; "" for a path HEAD lacks
	Ranges [][2]int `json:"ranges"` // [start, count] from "@@ -start,count"; count 0 is a pure insertion after start
}

// LastCommit is HEAD's commit.
type LastCommit struct {
	SHA     string    `json:"sha"`
	Subject string    `json:"subject"` // cleaned and cut to MaxSubjectRunes; an agent wrote it (text·A where Fleet wraps it)
	Paths   []string  `json:"paths"`   // changed against its first parent, or all paths for a root commit
	At      time.Time `json:"at"`      // committer date
}

// ActivityPage answers GET /v1/activity. Samples are oldest first. Next is the
// Seq to ask for next time. Gap is true when since was older than the ring
// holds, so the caller knows it missed samples and rebuilds from this page.
type ActivityPage struct {
	Samples []ActivitySample `json:"samples"`
	Next    int64            `json:"next"`
	Gap     bool             `json:"gap"`
}
```

**`Seq` across a restart.** `machined` seeds its counter with the start time in milliseconds times
1,000, then increments by one per sample, so a restarted `machined` never repeats a `Seq` a poller
has seen, without writing state to disk. A poller that sees `Gap` rebuilds that worktree's view
from the page, which is the "rebuilt within one sweep after a restart" task 35 needs. A clock
that steps back across a restart could repeat a range; the page's `Gap` does not catch that, and
it is listed under "does not claim".

#### Contract — `converge/claims.go` (complete in its surface)

```go
package converge

// ReadEnv is added to every call Sample makes, on top of §HGH's own
// environment: status must not take the index lock or rewrite the index.
var ReadEnv = map[string]string{"GIT_OPTIONAL_LOCKS": "0"}

// ReadFlags go after "git" and before the subcommand, after §HGH's prefix,
// so they win over any repository or included config.
var ReadFlags = []string{"-c", "core.untrackedCache=false", "-c", "core.quotePath=false"}

// Sample reads one clone. It never returns an error: a failure is ReadError,
// so one broken clone does not blank the machine's page. Every call is
// gitx.Call{Dir: dir, Class: gitx.Read, Scope: gitx.Clone}, 5 s each.
func Sample(ctx context.Context, r gitx.Runner, dir string, at time.Time) machineapi.ActivitySample

// ParseHunkHeaders reads "@@ -a[,b] +c[,d] @@" lines from `diff -U0` output
// and returns [a, b] pairs (b defaults to 1). It reads nothing else in the
// patch: no path, no content. A malformed header is an error, never a guess.
func ParseHunkHeaders(patch []byte) ([][2]int, error)
```

`Sample` makes these calls, in this order, and stops at the first failure with its code:

| # | Arguments after `git` and the prefixes | Reads | Parsed by |
|---|---|---|---|
| 1 | `rev-parse --verify -q HEAD` (`OKExit` 1) | `Head`; exit 1 is an unborn branch | trimmed 40-hex, else `git_failed` |
| 2 | `status --porcelain=v2 -z --untracked-files=normal` | the dirty set | P3-07's `ParseStatus` (untrusted input) |
| 3 | `diff --numstat -z --no-ext-diff --no-textconv HEAD` (skipped when unborn) | `Added`, `Deleted`; `-` is binary, -1 | a `-z` numstat parser here, with a rename's two paths |
| 4 | `ls-tree -z HEAD -- <path>…` for the dirty paths, in batches of 100 (`--literal-pathspecs` is in §HGH's prefix, `task-02.md:589`) | `Hunk.Base` | the mode, type, sha, path fields |
| 5 | `diff -U0 --no-ext-diff --no-textconv --no-color HEAD -- <path>` for each of the first `MaxHunkPaths` modified text paths | `Hunk.Ranges` | `ParseHunkHeaders` |
| 6 | `log -1 -z --format=%H%x00%cI%x00%s HEAD` then `diff-tree -r -z --no-commit-id --name-only --root HEAD` (skipped when unborn) | `LastCommit` | NUL fields |

One `diff -U0` per path is deliberate: parsing paths out of a multi-file patch means undoing git's
quoting, and a per-path call with a literal pathspec reads only the `@@` lines. Up to 50 calls of
a few milliseconds each every 5 s is the cost; a clone with more modified paths reads the first
50 in path order and sets `Truncated`. Every path from calls 2, 3 and 6 goes through
`teamtext.ValidPath` then `teamtext.Clean`; a refused one increments `Dropped` and is removed from
`Dirty`, `Hunks` and `LastCommit.Paths` alike. The subject goes through `teamtext.Clean` and is cut
at 140 runes; a subject `Clean` refuses becomes `""` and counts as one drop.

A `*gitx.UntrustedConfig` from §HGH H3 on any call (a filter driver, a `diff.*.command`, or any
other key H3's `Clone` row refuses) ends the sample with `clone_config_untrusted`: git never ran,
and the clone is reported unreadable rather than read with the agent's program. That lets an agent
hide its own worktree from the sweep; it cannot hide that it did, because the sample says so.

#### `machined`'s sweeper and route

`Sweeper` lists the clones under `lanes/<projectId>/<worktreeId>` (P8-20's layout) every 60 s and
keeps a `nextAt` per worktree: live ones every 5 s plus a uniform jitter of 0 to 5 s, idle ones
every 60 s (decision b). At most four samples run at once, so a machine with twenty lanes cannot
spend more than four git processes on this. Each sample is appended to a ring of `RingWindow`;
older samples are dropped. `GET /v1/activity?project=<pj-…>&since=<seq>` (bearer as every P8-20
route) returns the ring's samples for that project with `Seq > since`, at most 500, oldest first;
`project` is required, so no caller receives every project's activity by omission. `400
bad_request` for a missing or malformed `project` or `since`.
