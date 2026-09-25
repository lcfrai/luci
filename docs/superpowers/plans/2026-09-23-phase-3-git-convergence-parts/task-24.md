### Task 24: the sandboxd git runner: lane git inside the sandbox, lane work as a bundle (ruling R4)

Ruling R4 (`.superpowers/sdd/p3/orchestrator-rulings.md`) is structural: host git never executes
with an agent-writable directory as its repository, `--git-dir`, `-C`, working tree or alternate
(R4.2). A lane clone is such a directory from the moment a sandbox mounts it. So git work on a
lane runs **inside the sandbox**, through `sandboxd`, our own daemon there (R4.3). `sandboxd` is
not the agent CLI, so the "never parse the CLI or its PTY" rule does not apply to it.

This task builds that runner: a git RPC on `sandboxd`'s existing authenticated HTTP listener. It
answers with structured data. Lane work leaves the sandbox as a `git bundle`, which is data too
(R4.4). Updates go the other way through the same runner: the host writes a bundle from the
project and the runner fetches it inside the sandbox (R4.5, task 09's refresh). This half of the
part is the sandbox side. The host side (the client, host-side `bundle verify` and the fetch into
`refs/panacea/lanes/…`) is the second half, edit unit U2. "The runner" in this part is always
`sandboxd/internal/gitrunner`, never phase 2's `control-plane/internal/project/gitrunner.go`
(the host's read-only launcher) or task 02's `gitx.Exec`.

R4.3 names this as the seam phase 8's `machined` reuses (P8-D7: "the machine agent serves the
catalog and the git runner inside the machine pod"). Phase 3 builds the Docker half. P8-D7 is
not in this tree; it is in the env stream's worktree (`.claude/worktrees/env/docs/superpowers/
plans/2026-09-23-phase-8-own-environment.md:136`, checked 2026-09-25), and it says something else:
`machined` "reuses … phase 3's local git runner unchanged, on Linux paths, with phase 3's §HGH
hardening". After R4 that describes nothing phase 3 builds: `gitx.Exec` refuses lane
directories (`ErrLaneDir`), `Scope Clone` is gone, and `sandboxd/internal/gitrunner` cannot be
imported by `control-plane/cmd/machined` (Go's internal rule, and separate modules). **The
contract phase 8 must serve instead** is recorded as cross-stream dependency S3-P8.1 (plan):
- the `/git/v1/` wire of this part, with bearer auth, the five ops, the error envelope and the
  four limits;
- `testdata/gitrunner/*.json` as the shared goldens, which are **the only shared artifact**:
  the op code lives in `sandboxd/internal/gitrunner` and is not importable from
  `control-plane`, so `machined` either vendors `sandboxd` as a module or writes its own
  server against the goldens; phase 8 chooses;
- bundle crossing (R4.4): lane work reaches the host only as a bundle the host verifies;
- on the host, the Kata half is a `Lanes.For` that answers a `Runner` pointed at `machined`,
  not a `gitx.Machine`, which lane git no longer goes through.

**Order.** Deps: 02 (the pinned git, `gitx.MaxSentence` and `Sentence`, of which the runner's
error rule is a variant), 04 (the lane clone the runner serves, mounted at `sessionWorkDir`).
Users:
- 07 (`status`, `refs` for the lane head);
- 08 (`LaneGit.For`, the runner Observe reads through, `task-08.md:171`, `:185`);
- 09 (`bundle` out for landing, `fetch` in for refresh), and 10 through 09's refresh;
- 11 (`refs` for Elsewhere, and `Lanes.Drop` before removal);
- 12, through `AfterFetch`, which is 09's refresh;
- 14 (its tests, through the `Lanes.Dial` seam);
- 16 (`diff`).

Plan order: after 04, first in lane (b), before 07 (plan "Order"). It does not depend on lane
(a)'s 05 or 06.

**Trust note.** Every runner answer is the agent's claim about its own lane. The agent runs as the
same uid as `sandboxd` (`dev`, uid 1001: `images/sandbox/Dockerfile`'s `useradd -m -u 1001` and
`USER dev` lines, cited by content because task 05 inserts above `USER dev`; the entrypoint
`exec`s `sandboxd`, `images/sandbox/entrypoint.sh:13`), so it can read `sandboxd`'s token from
the process environment (`PANACEA_APP_ATTACH_TOKEN`, `sandboxd/cmd/sandboxd/main.go:44`). That is
true since phase 2 and this task does not change it. It follows that:
- the runner's validation and limits protect the runner and the lane from a malformed host
  request, and bound an honest answer; they protect the host from nothing;
- the host parses every answer as untrusted data, under the same limits, and derives no
  permission from it: a `status` or `refs` answer is a reading (task 07's "these are readings");
- the one thing the host trusts is a bundle it verified itself, with `git bundle verify` in the
  project under §HGH, before it fetches from it (U2).

**Files:**
- `sandboxd/internal/gitrunner/runner.go` — `Runner`, `New`, `ServeHTTP`: routing, POST-only,
  the op mutex, the per-op timeout, the error envelope (new)
- `sandboxd/internal/gitrunner/ops.go` — the five ops and the `git` argv each one runs (new)
- `sandboxd/internal/gitrunner/wire.go` — the request and response types, the JSON shapes (new)
- `sandboxd/internal/gitrunner/limits.go` — `MaxJSONBytes`, `MaxBundleBytes`, the timeouts (new)
- `sandboxd/internal/gitrunner/*_test.go` — the tests below (new)
- `sandboxd/internal/server/server.go` (p2) — `Config.GitWorktree`; one
  `s.mux.HandleFunc("/git/v1/", s.git)` beside `/attach` at `server.go:85`; `s.git` checks
  `s.authorised(r)` (`server.go:105-116`) first and answers 401 without it, exactly as
  `s.attach` does at `server.go:166-171`
- `sandboxd/internal/server/server_test.go` (p2) — test 6's cases (auth on `/git/v1/`, and the
  route absent without `GitWorktree`), appended; no existing case changes
- `sandboxd/cmd/sandboxd/main.go` (p2) — flag `-git-worktree`, default `/home/dev/work`, beside
  the flags at `main.go:40-48`; `serve_posix.go:11-18` and `serve_other.go:13` (p2) pass it
  through `newServer` into `server.Config`
- `testdata/gitrunner/*.json` — the wire golden fixtures, at the repo root (new; `go.work`
  already lists `./sandboxd` and `./control-plane`)

Stdlib only (`net/http`, `os/exec`, `encoding/json`, `context`, `sync`). No new dependency, so
`sandboxd/vendor` (committed; `images/sandbox/Dockerfile:8` builds with `-mod=vendor`) needs no
re-vendoring. The entrypoint passes no `-git-worktree`, and needs none: the default is the
control plane's `sessionWorkDir` (`control-plane/internal/api/sessions.go:33` in this tree;
`:35` at `feat/phase-2-cockpit`, which is not an ancestor of this branch; the value is the same).

#### Interfaces — complete

**The endpoint.** `POST /git/v1/<op>`, `<op>` one of `status`, `refs`, `bundle`, `fetch`, `diff`.
Another method is 405; another op is 404 `unknown_op`. Every op takes JSON in, except `fetch`,
whose body is a bundle. Every op answers JSON, except `bundle` (a bundle) and `diff` (bytes).
**The request never names a directory.** The runner has one, `Config.GitWorktree`, and every git
call it makes is `G …` (defined under "The ops"), with `GIT_OPTIONAL_LOCKS=0` in the environment
on the reads (`status`, `refs`, `bundle`, `diff`) and `--no-auto-maintenance` on `fetch`.
- **`-c safe.directory=<GitWorktree>`** is what lets git open the lane at all. The lane is a bind
  mount of a host-created directory, whose owner is not `dev`, and git refuses it: `fatal:
  detected dubious ownership in repository at '/home/dev/work'`, exit 128, measured with this
  image and mount; `-c safe.directory=/home/dev/work` gave exit 0
  (`scope-and-risk-phase-3-git-convergence.md:373-375`). A session sandbox also carries it in
  `GIT_CONFIG_*` (`control-plane/internal/api/sessions.go:118-143`), but a runner-only sandbox
  (U2's pool) carries no session env. So the runner sets it itself, on the command line, which
  git counts as protected configuration. Both sandbox kinds then behave the same.
- **`-c core.hooksPath=/dev/null` and `-c core.fsmonitor=false`**, unconditionally. `/dev/null`
  is not a directory, so no hook path under it can exist, and `dev` cannot create one in `/dev`;
  no image change is needed. The runner's own writes are remote-tracking refs (fetched or Set),
  deletes of them, and the repair's removal of unreadable ones, and no hook is owed on them.
  Without these, a lane-local `core.hooksPath` would run `reference-transaction` on the
  runner's `fetch` and `update-ref`, and a lane
  `core.fsmonitor` would run on `status`, on every sweep and refresh. In a session sandbox that
  is only what the agent can already do. In a **runner-only** sandbox it is not: the control
  plane starts that one after the session has ended, with no session record, and nothing in the
  cockpit shows it, so planted code would keep running after "stop". That breaks "stopped means
  stopped", though never the host, which trusts only a bundle it verified itself (U2).
- **Residual, recorded:** lane-defined clean and smudge filter drivers (`filter.<name>.clean`
  named by a lane `.gitattributes`) still run on `status` in a runner-only sandbox. `diff` sets
  `--no-ext-diff --no-textconv`; `status` has no equivalent flag. Non-claims, U2.
- **`-c core.alternateRefsCommand=true`** (review r4-2 M-7). A `fetch`'s connectivity check
  lists the tips of every repository in `objects/info/alternates`, which the lane writes. By
  default git runs `for-each-ref` there, but a lane `core.alternateRefsCommand` would run
  through the shell instead: planted code on the runner's `fetch`. With the value `true`, the
  shell runs `true <alternate path>`: no output, so no alternate tip is taken as known, and the
  check only walks more. This is reasoned from `transport.c` and `connected.c`, not run. Test 8
  proves it red first. If test 8 is green without this `-c`, the fetch never runs the command:
  drop the `-c` and record that finding here.

None of the lane's config reaches the host. The git is the image's
(`images/sandbox/Dockerfile`, the `apt-get install … git` line, Debian bookworm's package); every
flag above predates its release series (checked against the git docs, not run in the image).

```go
// wire.go — the JSON shapes. Requests are decoded with DisallowUnknownFields; a body past
// MaxJSONBytes, an unknown field or trailing data is 400 bad_request.

// Head is where the lane's HEAD points. Ref is "" when HEAD is detached; Sha is "" when
// the branch is unborn.
type Head struct {
	Ref string `json:"ref"`
	Sha string `json:"sha"`
}

type StatusRequest struct{}
type StatusResponse struct {
	PorcelainV2Z []byte `json:"porcelainV2Z"` // raw git bytes; base64 in JSON, as encoding/json does []byte
	Head         Head   `json:"head"`
}

type RefsRequest struct {
	// Names are for-each-ref prefixes, never globs: a full refname, or a hierarchy with no
	// trailing slash such as "refs/heads", which for-each-ref matches "from the beginning up to a
	// slash", so it lists refs/heads/a and refs/heads/b/c. Each passes plain check-ref-format,
	// which "refs/heads" does and "refs/heads/*" does not (both exit codes probed, git 2.39.5 and
	// 2.53; review r4-3 C-1).
	Names      []string `json:"names"`
	MergedInto string   `json:"mergedInto"` // a refname; "" leaves every Merged false
}
type Ref struct {
	Name   string `json:"name"`
	Sha    string `json:"sha"`
	Merged bool   `json:"merged"` // reachable from MergedInto
}
type RefsResponse struct {
	Refs []Ref `json:"refs"`
	Head Head  `json:"head"`
}

type BundleRequest struct {
	Refs  []string `json:"refs"`  // full refnames, each passing check-ref-format, or exactly
	                               // "HEAD", matched literally (Validation; review r4-3 M-1);
	                               // only refs the basis does not reach (the host splits first,
	                               // "The split" below; review r4-4 C-1)
	Basis []string `json:"basis"` // shas the host already has; 40 or 64 hex
}

// FetchRequest travels as headers, since the body is the bundle: one X-Panacea-Refspec per
// Refspecs entry, one X-Panacea-Set per Sets entry, one X-Panacea-Delete per Deletes entry. The
// validation goldens and fetch.request.json spell it as this JSON.
type FetchRequest struct {
	Refspecs []string `json:"refspecs"` // [+]<src>:<dst>, at least one
	Sets     []string `json:"sets"`     // "<dst> <sha>": a ref the bundle does not carry, because
	                                     // the lane already holds its commit (review r4-4 C-1)
	Deletes  []string `json:"deletes"`
}

// FetchResponse is read back after the fetch, the Sets and the deletes. A deleted ref is absent.
type FetchResponse struct {
	Refs map[string]string `json:"refs"` // destination refname → sha, each refspec's and Set's
}

type DiffRequest struct {
	Path string `json:"path"` // relative to the lane root; see Validation
}

// ErrorResponse is every non-2xx answer except 401, whose body is phase 2's plain text.
type ErrorResponse struct {
	Code    string   `json:"code"`              // bad_request, path_outside_lane, unknown_op,
	                                             // bundle_empty, too_large, git_failed, git_timeout,
	                                             // lane_broken, prerequisite_missing
	GitSays string   `json:"gitSays,omitempty"` // git_failed and lane_broken; see Errors
	Ref     string   `json:"ref,omitempty"`     // lane_broken only: the ref git called a bad object,
	                                             // or the repair's symlink
	Missing []string `json:"missing,omitempty"` // prerequisite_missing only: the shas the lane
	                                             // lacks, sorted (review r4-4 I-1)
}
```

**The ops.** `G` below is `git -c safe.directory=<GitWorktree> -c core.hooksPath=/dev/null -c
core.fsmonitor=false -c core.alternateRefsCommand=true -C <GitWorktree> --literal-pathspecs`
(the reasons are under "The endpoint"; the fourth `-c` under its residual note).

1. **`status`** (task 07's observer). `G status --porcelain=v2 -z --untracked-files=all`, whose
   stdout is `PorcelainV2Z` byte for byte; the runner does not parse it (task 07 does, on the
   host, as untrusted input). Then **Head**, the same for `status` and `refs`:
   `G symbolic-ref -q HEAD` (exit 0: `Ref` is its trimmed stdout; exit 1: detached, `Ref` ""),
   then `G rev-parse --verify -q HEAD` (exit 0: `Sha`; exit 1: unborn, `Sha` ""). Any other exit
   is `git_failed`.
2. **`refs`** (task 07's lane head, task 11's Elsewhere). `G for-each-ref
   --format=%(refname)%00%(objectname) <Names…>`, one ref per line, split at the NUL (a refname
   holds no NUL and no newline: `check-ref-format` forbids control characters). When
   `MergedInto` is set, also `G for-each-ref --format=%(refname) --merged=<MergedInto>
   <Names…>`; a ref is `Merged` only when this second listing has it. `Merged` is positive
   evidence: a ref naming a missing object is in neither `--merged` nor `--no-merged` (task 11
   probed it, git 2.53, both exit 0), so "absent from `--no-merged`" would call it `Merged` and
   task 11 would drop it from Elsewhere (review r4-2 I-3). Then Head.
   An empty `Names` is 400: the runner never lists every ref by default. Task 11 lists a
   lane's branches with `Names: [refs/heads]`, a prefix; `refs/heads/*` is 400 (Validation),
   and runner test 11 pins both (review r4-3 C-1).
3. **`bundle`** (task 09's landing and task 11's archive, through U2). An empty `Refs` is 400
   `bad_request`, as `refs` refuses an empty `Names`, before any git call (review r4-2 M-3).
   Keep the `Basis` shas that
   `G cat-file -e <sha>` finds; a sha the lane lacks is dropped, not an error, since
   `--not` over a missing object fails the bundle. Then `G rev-list --count <Refs…> --not
   <kept…>`; `0` is 409 `bundle_empty`, decided before `bundle create` so the answer never rests
   on the wording of git's refusal. Then `G bundle create - <Refs…> --not <kept…>`. Its stdout
   goes through a counting writer into a mode-0600 file in a fresh `os.MkdirTemp("/tmp",
   "gitrunner-")` directory, outside the worktree; past `MaxBundleBytes` the runner kills git and
   answers 413 `too_large`. Spooling first is what lets the cap be a status code rather than a
   truncated 200. The file is then sent as `200`, `Content-Type: application/x-git-bundle`, with
   its `Content-Length`, and the directory is removed whatever happens. `bundle create` leaves
   out, with exit 0 and no warning, every ref whose tip `--not <kept…>` reaches (review r4-4
   Probe 1; fix round 5 re-ran it on both gits: `HEAD` at C3, `x` at C4 and `y` at C2, `--not
   C2`, lists `HEAD` and `x`). So the host asks only for refs the basis does not reach (U2's
   split), and a ref the bundle still lacks is the host's `list-heads` refusal.
4. **`fetch`** (task 09's refresh, R4.5: the host writes a bundle from the project and the runner
   fetches it here). The body is a bundle, read through `http.MaxBytesReader(MaxBundleBytes)` into
   a mode-0600 file in a fresh `os.MkdirTemp("/tmp", "gitrunner-")` directory. Headers are
   `FetchRequest`'s. In order:
   1. **Repair: stale locks, then refs naming a missing object** (review r4-3 I-2, r4-5 M-2).
      First, step 2's walk (below) runs for lock files alone: a regular file whose name ends
      `.lock` and whose mtime is older than `BundleTimeout` is removed with `os.Remove` (a
      symlink is left to step 2). The runner holds the op semaphore, so no lock of its own is
      live, and an agent's live ref lock is milliseconds old. A stale `<dst>.lock`, left by a
      runner op killed while it held the lock (see Timeouts), otherwise fails every later fetch
      into that `<dst>`, and this step's own `update-ref -d` of that ref: `error: cannot lock ref
      '<dst>': … File exists`, exit 1, while verify exits 0 (r4-5 Probe 7, 2.39.5). Then `G
      for-each-ref
      --format=%(objectname)%00%(symref)%00%(refname) refs/remotes/origin/ refs/remotes/upstream/`,
      the runner's own namespace (Validation limits every `<dst>`, Set and delete to it); the shas
      of the rows whose `%(symref)` is empty go on stdin to `G cat-file --batch-check`, and each
      ref whose line ends ` missing` is `G update-ref --no-deref -d <ref>`. This step deletes no
      symref: once its target is gone it dangles, which git skips, and the fetch rewrites the
      target (step 2 drops a symref only where it is a `<dst>`). On git 2.39.5 one such ref
      anywhere makes `bundle verify` and `fetch` exit 128, `fatal: bad object <ref>`, even for a
      bundle with no prerequisite; after the repair both exit 0, and `refs/remotes/origin/HEAD`
      resolves again (fix round 4 probe, both gits).
   2. **Repair: unreadable ref files** (review r4-4 I-2). `D` = `G rev-parse --absolute-git-dir`.
      `D/refs` and `D/refs/remotes` each `Lstat` as a directory or are absent, and
      `filepath.WalkDir`, which follows no symlink, walks `D/refs/remotes/origin` and
      `D/refs/remotes/upstream`. An entry that is neither a directory nor a regular file (a
      symlink) is 409 `lane_broken`, `Ref` its refname, and nothing is fetched. A regular file is
      unreadable when its name does not end `.lock` (a live lock may be the agent's git, and
      step 1 has removed the stale ones), step 1's
      listing omits its refname (git skips an empty or garbled file with `warning: ignoring
      broken ref`), its bytes do not begin `ref: ` (a dangling symref, which git skips silently
      and which breaks neither verify nor fetch), and `G rev-parse -q --verify <refname>` still
      exits 1 (a ref the agent wrote since the listing resolves, and stays). It is removed with
      `os.Remove`. `update-ref -d` cannot remove it (exit 1, `reference broken`; r4-4 Probe 3).
      One such file anywhere in the namespace, a `<dst>` or not, fails verify and fetch of any
      bundle: 2.39.5 exits 128, `fatal: bad object refs/remotes/origin/other`; 2.53 exits 1,
      `did not send all necessary objects` or `some prerequisite commits exist in the object
      store, but are not connected`, naming no ref, so a rule keyed on git's words would miss it.
      With the file removed both exit 0 on both gits (fix round 5 probe). A garbled
      `packed-refs` is not repaired: the recorded residual. A file whose bytes begin `ref: ` and
      whose refname is a refspec's or a Set's `<dst>` is `G update-ref --no-deref -d <dst>` (exit
      0, dangling or not): `fetch` writes through a symref `<dst>`, so an agent's
      `refs/remotes/upstream/<b>` → `refs/heads/y` moved `y`, and one → `refs/heads/nothing`
      created it (fix round 5 probe, both gits). Symrefs are loose files only, so the walk sees
      every one. When this step removed a file with `os.Remove`, step 1's listing and deletes
      run again (not its lock sweep) before step 3 (review r4-5 M-1). A removed loose file can
      uncover a `packed-refs` line that names a missing object, which the first listing could
      not see: git read the broken loose file for that refname, warned `ignoring broken ref` and
      omitted it. Without the second run verify and fetch still fail (2.39.5 `fatal: bad object
      <ref>`, 128; 2.53 `not connected`, 1), and the runner would answer `lane_broken` for a ref
      inside its own namespace. The second listing names it ` missing`, and `update-ref
      --no-deref -d` exits 0 and drops the packed line; verify and fetch then exit 0 and
      `refs/remotes/origin/HEAD` resolves (r4-5 Probe 3, both gits).
   3. **Prerequisites** (review r4-4 I-1). The runner reads the bundle's header itself, up to its
      first empty line and at most `MaxJSONBytes`: a first line `# v2 git bundle` or `# v3 git
      bundle`, `@` capability lines skipped, each `-<sha> …` line a prerequisite (40 or 64 hex).
      The prerequisites and every Set's sha go on stdin to `G cat-file --batch-check`; a line other
      than `<sha> commit <size>` is missing (both gits answer `<sha> missing`, exit 0; probed). Any
      missing is 409 **`prerequisite_missing`**, `Missing` those shas, and nothing is fetched. A
      header the runner cannot read is left to verify, which refuses it (`git_failed`). The code is
      the runner's decision, not git's wording: `bundle verify -q` exits 1 and prints nothing on a
      missing prerequisite (r4-4 Probe 2 D), as it does on many other failures.
   4. `G bundle verify -q <file>`, then `G fetch --no-auto-maintenance --no-recurse-submodules
      --no-tags --quiet <file> <refspecs…>` (no recursion: `fetch.recurseSubmodules` defaults to
      on-demand, and a bundle that moves a gitlink would start a fetch in a submodule; review
      r4-2 M-2).
   5. `G update-ref --no-deref <dst> <sha>` for each Set, then `G update-ref --no-deref -d <ref>`
      for each delete. `--no-deref` on both: a plain `update-ref -d` of a symref deletes its
      target (fix round 5 probe, both gits: `refs/remotes/upstream/c` → `refs/heads/y`, and `-d`
      removed `refs/heads/y`; with `--no-deref` only the symref went, and a `--no-deref` set
      replaced the symref and left `y` alone).
   6. Remove the directory, and answer `FetchResponse` from `G for-each-ref
      --format=%(refname)%00%(objectname) <each refspec dst, Set dst and delete…>`.

   A failed verify or fetch is `git_failed` and nothing is fetched, except that one whose
   `fatal:` line is `fatal: bad object <ref>`, `<ref>` being `HEAD` or beginning `refs/`, is 409
   **`lane_broken`** with `Ref` = `<ref>`, whatever git's exit code. After steps 1 and 2, and
   step 1's second run when step 2 removed a file, such a `<ref>` is outside the runner's
   namespace (`refs/heads/bad` in the probe), or inside it only through a garbled `packed-refs`,
   which no step repairs (review r4-5 M-1): the agent's to fix, and no bundle the host sends can
   mend it. The runner deletes only what it owns, so it never repairs one outside.
   The runner never writes `refs/heads/…`: product-branch commits arrive as remote-tracking
   refs, step 2 drops a symref at a `<dst>` before the fetch, `--no-deref` keeps one from
   redirecting a Set or delete, and what the lane does with them is the agent's.
5. **`diff`** (task 16). `G diff --no-color --no-ext-diff --no-textconv -U3 HEAD -- <Path>`; its
   stdout is the body, `200`, `Content-Type: text/x-diff`. Past `MaxJSONBytes` it is 413
   `too_large`. An unborn HEAD makes git fail, which is `git_failed`.

**Validation**, before the op lock and before any git call in the lane; each failure is 400 `bad_request`
unless named otherwise.
- **Refs.** Every refname the host sends (`Names`, `MergedInto`, `Refs`, both sides of a
  refspec, each Set's `<dst>`, each delete) begins `refs/` and passes plain `git check-ref-format <ref>` (no
  `--refspec-pattern`, so a `*` is refused), run without `-C` (it needs no repository). A
  `Names` entry is a prefix (`RefsRequest`), and `refs/heads` passes. One exception: a
  `BundleRequest.Refs` entry may be exactly `HEAD`, which task 11's archive carries for a
  detached HEAD (`ArchiveLane`, U2). `HEAD` is matched literally and never given to
  `check-ref-format`, which refuses it (exit 1, git 2.39.5 and 2.53; review r4-3 M-1). `HEAD`
  is refused everywhere else, and `bundle` is read-only in the lane, so admitting it moves
  nothing there.
- **Shared goldens** (review r4-3 C-1). Each rule on this list has cases in
  `testdata/gitrunner/validation/*.json`, one file per case: `{"op", "request", "code"}`, where
  `request` is the op's JSON body, or for `fetch` its `FetchRequest`, and `code` is
  `""` (admitted) or the 400 code. Runner test 7 runs every case through the runner's
  Validation, and `lanegittest.Fake` runs the same Validation before its git (U2). So a
  request the runner refuses fails the host test that sends it, not only task 23.
- **Destinations.** A refspec's `<dst>`, each Set's `<dst>` and every delete sit under
  `refs/remotes/origin/` or `refs/remotes/upstream/`, and each `<dst>` appears at most once
  across one request's refspecs, Sets and deletes. A refspec is `[+]<src>:<dst>` with exactly
  one `:`; a Set is `<dst> SP <sha>` with exactly one space (review r4-4 C-1).
- **Shas** (`Basis`, and each Set's) match `^[0-9a-f]{40}$` or `^[0-9a-f]{64}$`.
- **Path** (`diff`): 400 **`path_outside_lane`** when it is empty, is absolute, holds a `..`
  segment, a `\` or a NUL, starts with `:`, or when `filepath.Clean(filepath.Join(GitWorktree,
  p))` is not strictly under `GitWorktree + "/"`. `--literal-pathspecs` already turns off
  pathspec magic; the `:` rule refuses it before git sees it.

```go
// limits.go — the contract's numbers. U2's host client declares the same four values. Both
// modules' tests compare their constants with testdata/gitrunner/limits.json, because the host
// cannot import this package: Go's internal rule forbids it from outside sandboxd/, and
// control-plane/go.mod requires no sandboxd module.
const (
	MaxJSONBytes   = 8 << 20           // a JSON request, a JSON result, diff's bytes, git's stderr
	MaxBundleBytes = 1 << 30           // a bundle, in (fetch) and out (bundle)
	ReadTimeout    = 30 * time.Second  // status, refs, diff: the whole op
	BundleTimeout  = 120 * time.Second // bundle, fetch: the whole op, body read included
)
```

- **One op at a time per sandbox.** The runner holds a one-slot semaphore (a mutex whose wait
  can end) for the whole op, taken after validation. A waiting op gives up with its own timeout.
  It orders only the runner's calls, so its `fetch` and `update-ref` never interleave with its
  reads; the agent's own git is not ordered by it, which is why the reads set
  `GIT_OPTIONAL_LOCKS=0`.
- **Timeouts.** One `context.WithTimeout` per op, and git runs under `exec.CommandContext` with
  `cmd.Cancel` sending SIGTERM and `cmd.WaitDelay = KillDelay`, after which Go sends SIGKILL;
  the answer is 504 `git_timeout`. `CommandContext`'s default Cancel is SIGKILL, and git removes
  its lock files on SIGTERM, SIGINT and SIGHUP but cannot on SIGKILL, so a killed `fetch`, Set
  or step 1 delete would leave its `<dst>.lock` (review r4-5 M-2; from git's tempfile design,
  not probed). `KillDelay = 2 * time.Second` is the runner's own (`runner.go`), not a contract
  limit: 2 s keeps the 504 inside the host client's deadline, the op's timeout plus 5 s. A lock
  that SIGKILL still leaves (git past `KillDelay`, or `sandboxd` itself killed) is fetch step
  1's to sweep.
- **Size.** A JSON result is encoded to a buffer and git's stdout is read through the same cap;
  past `MaxJSONBytes` either one is 413 `too_large`. So is a request body past its cap.

**Errors.** A git exit the op does not expect is 422 `git_failed` with `gitSays`: the last stderr
line that starts `fatal:`; else the last `error:` or `remote:` line, else the last non-empty
line, else `git exited <code>`; capped at 2048 bytes on a UTF-8 boundary (`gitx.MaxSentence`,
`task-02.md:194-195`). This is a **variant** of task 02's `Sentence` (`task-02.md:210-213`),
not a copy: `Sentence` takes the last line with any of the three prefixes, so on `fatal: a`
then `error: b` it answers `error: b` where the runner answers `fatal: a`. The runner prefers
`fatal:` because git's `fatal:` is the cause and a later `error:` is usually cleanup. It is
written in `sandboxd`, not imported, for the same reason as the limits, and the
`error.git_failed.json` golden pins the variant, in the runner's tests only (test 7); the host
passes `gitSays` through as received, capped, and never recomputes it. A `lane_broken` answer
(409, `fetch` only) carries the same `gitSays` and its `Ref`, capped at `MaxSentence`, and
`error.lane_broken.json` pins it; the repair's symlink answer carries `Ref` and no `gitSays`,
since no git ran. A `prerequisite_missing` answer (409, `fetch` only) carries `Missing` and no
`gitSays`, and `error.prerequisite_missing.json` pins it (review r4-4 I-1).

**Wiring (p2 files).** `server.Config` gains `GitWorktree string`. `New` mounts
`s.mux.HandleFunc("/git/v1/", s.git)` beside `/attach` (`server.go:85`) only when `GitWorktree`
is set, so phase 2's tests, which build `Config` without it, are unchanged:

```go
func (s *Server) git(w http.ResponseWriter, r *http.Request) {
	if !s.authorised(r) {
		slog.Warn("rejected unauthenticated git call", "remote", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.runner.ServeHTTP(w, r) // *gitrunner.Runner, built in New from cfg.GitWorktree
}
```

`AllowUnauthenticated` opens `/git/v1/` exactly as it opens `/attach`: local development only.
`main.go` adds `gitWorktree := flag.String("git-worktree", "/home/dev/work", "the lane clone the
git runner serves")` beside the flags at `main.go:40-48` and passes `*gitWorktree` through
`newServer` (`serve_posix.go:11`, and `serve_other.go:13`'s signature) into the `Config`.

**Wire golden fixtures**, `testdata/gitrunner/` at the repo root: `limits.json`, and one file
per shape (`status.response.json`, `refs.request.json`, `refs.response.json`,
`bundle.request.json`, `fetch.request.json`, `fetch.response.json`, `diff.request.json`,
`error.git_failed.json`, `error.path_outside_lane.json`, `error.lane_broken.json`,
`error.prerequisite_missing.json`), and `validation/`, the Validation
cases above. Each module's test decodes every fixture into its own type with
`DisallowUnknownFields`, re-encodes it, and compares with the fixture's compacted bytes, so a
field renamed on either side fails a test on that side. `sandboxd/internal/gitrunner` reads them
at `../../../testdata/gitrunner`.

#### The tests to write first: the runner

`gitrunner` carries `//go:build linux || darwin`, as `server.go:1` does, so its tests run where
`sandboxd` runs: CI's `ubuntu-latest` job (`azure-pipelines.yml:6`, `:14`, `go test
./sandboxd/... -race`). They use a real `git` on a `t.TempDir()` lane. The runner takes its temp
root and limits from an unexported `newWithLimits`, which `New` calls with `/tmp` and the
constants, and it records every argv and directory it executes, for the tests to assert on. Each
test is red before its code exists.
1. **A path outside the lane is refused before git runs.** `diff` with `""`, `/etc/passwd`,
   `../x`, `a/../../x`, `a\b`, `:(top)x`, `a` NUL `b` and `.`: each is 400 `path_outside_lane`,
   and the recorder holds no call.
2. **The request never names a directory.** A body with `dir`, `cwd`, `gitDir` or `worktree` is
   400 (unknown field). Across a run of all five ops, every recorded argv after `git` begins
   `-c safe.directory=<GitWorktree> -c core.hooksPath=/dev/null -c core.fsmonitor=false -c
   core.alternateRefsCommand=true -C <GitWorktree> --literal-pathspecs`, except
   `check-ref-format`, which has no `-C` at all.
3. **`fetch` refuses a bad bundle.** A truncated bundle and random bytes are each 422
   `git_failed`; the lane's refs are byte-identical before and after, and the temp directory is
   gone. A destination `refs/heads/main`, `refs/panacea/lanes/x` or `refs/remotes/origin` (no
   trailing name) is 400 and fetches nothing.
4. **A bundle round-trips, and "nothing new" says so.** `bundle` with a basis the lane has and
   one it lacks answers a bundle that `git bundle verify` passes in a second repository holding
   the basis. With the lane's tip as the basis, the answer is 409 `bundle_empty`.
5. **The caps are status codes.** With `MaxBundleBytes` set to 1 KiB by `newWithLimits`,
   `bundle` is 413 `too_large` and `fetch` of a 2 KiB body is 413; with `ReadTimeout` at 1 ms
   and a second op holding the semaphore, the waiting op is 504 `git_timeout`.
6. **Auth, in `server_test.go` (p2).** `/git/v1/status` without a bearer is 401 and the runner
   is not reached; with the token it is 200. Without `GitWorktree` the route is 404.
7. **The fixtures.** Every `testdata/gitrunner/*.json` round-trips through its type;
   `limits.json` equals the four constants; `error.git_failed.json`'s `gitSays` is the last
   `fatal:` line of a stderr that also holds `error:` lines, and a 3000-byte `fatal:` line is
   cut to 2048 bytes on a rune boundary. Every `validation/*.json` case gets its `code` from
   the runner's Validation, with no git call on a refusal; the cases hold at least
   `refs/heads` (admitted), `refs/heads/*`, `HEAD` as a `Names` entry and as a refspec side
   (each 400), and `HEAD` as a `BundleRequest.Refs` entry (admitted); a Set `<dst> <sha>`
   under `refs/remotes/upstream/` (admitted), a Set whose `<dst>` is `refs/heads/main`, whose
   sha is 39 hex, or with two spaces, and a `<dst>` both a refspec's and a Set's (each 400).
8. **`TestRunnerRunsNoLaneHook`.** The lane's `.git/config` sets `core.hooksPath=.hooks` and
   `core.fsmonitor=<marker script>`; `.hooks/reference-transaction` writes a second marker. A
   `status`, a `fetch` of a valid bundle and a `fetch` with one `X-Panacea-Delete` leave both
   markers absent, and the refs moved. Red with either `-c` dropped from `G`. A second lane
   case: `objects/info/alternates` names a second repository with one commit, and the lane's
   config sets `core.alternateRefsCommand=<marker script>`. A `fetch` of a valid bundle leaves
   that marker absent. Red with `core.alternateRefsCommand=true` dropped (see the M-7 note).
9. **`refs` takes `Merged` from `--merged`.** A lane with `refs/heads/done` merged into `main`,
   `refs/heads/wip` not, and `refs/heads/bad` a ref file naming a missing object: `refs` with
   `MergedInto: refs/heads/main` answers `done` Merged true, and `wip` and `bad` Merged false.
   Red against the `--no-merged` rule, which calls `bad` Merged (review r4-2 I-3).
10. **`bundle` takes `HEAD` and refuses nothing to carry.** A detached HEAD and `refs/heads/x`
    as `Refs` answer a bundle whose `list-heads` names both. An empty `Refs` is 400 and the
    recorder holds no call (review r4-2 M-3).
11. **`refs` takes a prefix, not a glob** (review r4-3 C-1). A lane with `refs/heads/a`,
    `refs/heads/b/c` and `main`: `Names: [refs/heads]` lists all three; `Names:
    [refs/heads/*]` is 400 and the recorder holds only `check-ref-format`.
12. **`fetch` repairs its own refs, and names a broken lane** (review r4-3 I-2). Red first
    without the repair. A lane whose `refs/remotes/origin/main` file names a missing object,
    `refs/remotes/origin/HEAD` a symref to it, and `refs/remotes/origin/other` a second
    missing one: a `fetch` of a whole-history bundle into `origin/main` is 200, answers
    `origin/main` at the bundle's tip, `other` is gone and `origin/HEAD` is still a symref and
    resolves. A lane whose `refs/heads/bad` names a missing object: the same `fetch` is 409
    `lane_broken`, `Ref` `refs/heads/bad`, `refs/heads/bad` untouched, the origin refs
    unmoved. Which step fails depends on the git (verify on 2.39.5, the fetch on 2.53), so
    the test asserts the answer and the refs, not the step. Unreadable files (review r4-4
    I-2), red first without fetch step 2: an empty `refs/remotes/origin/main` (the `<dst>`)
    and a `zzzz` `refs/remotes/upstream/other` (not one) → the same `fetch` is 200,
    `origin/main` at the bundle's tip, `upstream/other` gone, and a dangling loose symref
    `refs/remotes/origin/dang` kept. A symlink `refs/remotes/origin/link` → 409
    `lane_broken`, `Ref` `refs/remotes/origin/link`, the recorder holds no `fetch`. A
    `refs/remotes/upstream/main` symref to `refs/heads/y`, with `upstream/main` a refspec
    `<dst>` → 200, `upstream/main` a plain ref at the tip, `refs/heads/y` unmoved. An
    uncovered packed ref (review r4-5 M-1), red first without step 1's second run: a lane
    cloned with `--no-hardlinks`, so its origin refs are in `packed-refs`, whose
    `refs/remotes/origin/other` line names a missing object, under an empty loose file of that
    name → the same `fetch` is 200, `packed-refs` holds no `origin/other` line, and
    `origin/HEAD` resolves. The mutant answers 409 `lane_broken`, `Ref`
    `refs/remotes/origin/other`, on 2.39.5, and 422 `git_failed` on 2.53.
13. **`fetch` takes Sets** (review r4-4 C-1). A lane at T, with A an ancestor of T: a bundle
    of `refs/heads/main` over basis T, refspec `+refs/heads/main:refs/remotes/origin/main`
    and Set `refs/remotes/upstream/main <A>` → 200, `FetchResponse` holds both, `upstream/main`
    at A. With `upstream/main` a symref to `refs/heads/y`, the Set and a delete of it each
    leave `refs/heads/y` unmoved. Red against a Set or delete without `--no-deref`.
14. **A missing prerequisite has its own code** (review r4-4 I-1). A bundle whose
    prerequisite the lane lacks → 409 `prerequisite_missing`, `Missing` that sha, no ref moved,
    and the recorder holds no `bundle verify`; a Set whose sha the lane lacks → the same, with
    that sha. A truncated header → 422 `git_failed`, from verify.
15. **A killed op leaves no lock behind** (review r4-5 M-2). Red first without the sweep: a
    `refs/remotes/origin/main.lock` whose mtime `os.Chtimes` sets `BundleTimeout` plus 1 s ago
    → a `fetch` into `origin/main` is 200 and the lock is gone; the same lock 1 s old → 422
    `git_failed`, `gitSays` git's `cannot lock ref` line, and the lock kept. Red against the
    walk skipping every `.lock` (the first case), and a sweep with no age test (the second).
    Then a stand-in `git`, first on `PATH` by `t.Setenv`, a script the test writes with a
    marker path in it: `trap 'touch <marker>; exit 143' TERM; sleep 30 & wait`. With
    `ReadTimeout` 200 ms by `newWithLimits`, `status` is 504 `git_timeout` and the marker
    exists: red against `CommandContext`'s default Cancel (SIGKILL). A stand-in that ignores
    SIGTERM (`trap '' TERM; sleep 30`) → 504 within `ReadTimeout` + `KillDelay` + 1 s: red
    against a SIGTERM Cancel with no `WaitDelay`, which waits out the 30 s.

#### The host side: `control-plane/internal/lanegit` (edit unit U2)

The host half: a client for the runner, a pool that finds or starts a runner for a lane, and the
two bundle moves. Host git in this half runs only in the **project** or under
`Hardening.OwnedRoot`, never in a lane (R4.2); every call goes through task 02's `gitx.Exec`.

**Files (host side):**
- `control-plane/internal/lanegit/wire.go` — the shapes of `wire.go` above, field for field, and
  the four limits of `limits.go` above, checked against `testdata/gitrunner` (new)
- `control-plane/internal/lanegit/client.go` — `Runner`, `NewRunner`, `NewRunnerTransport` (S8-P3.5), `RunnerError` (new)
- `control-plane/internal/lanegit/pool.go` — `Lanes`, `For`, the runner-only sandbox, `Machine`, `Lanes.Bundles`, `readyWaiter` (S8-P3.5) (new)
- `control-plane/internal/lanegit/bundle.go` — `FetchLane`, `ArchiveLane`, `SendBranch`, `Sent`, the split, `BundleRefused`, `Bundles`, `DirBundles` (S8-P3.4) (new)
- `control-plane/internal/lanegit/lanegittest/fake.go` — `Fake`, a test-only `Runner` (new)
- `control-plane/internal/lanegit/lanegittest/validate.go` — the Fake's copy of the runner's
  Validation, held to `testdata/gitrunner/validation` (new)
- `control-plane/internal/lanegit/*_test.go` — the tests below, and `behaviour_test.go`, the
  pins (new)
- `control-plane/internal/backend/backend.go` (p2) — `GitPath = "/git/v1/"` in the const block
  at `backend.go:40-45`, beside `AttachPath` (`:42`) and `HealthPath` (`:43`)
- `control-plane/internal/reaper/reaper.go` (p2) — `Config` (`reaper.go:120`) gains `Held`; see
  the pool
- `control-plane/cmd/api/main.go` (p2) — one `// ── task 24 ──` block; see Wiring
- `control-plane/internal/api/gitdeps.go` (task 04's) — outside task 24's own block: one case in
  `gitFailed` for a `*lanegit.RunnerError` (see "How a caller answers"), and `bundle_refused`
  in the closed reason list (see "Reason codes")
- `control-plane/cmd/api/lanegit_spy_test.go` — `TestHostNeverRunsGitInALane` (new)

Stdlib and packages already in `control-plane/go.mod` only. No new dependency on either side.

```go
// client.go — the host's view of one sandbox's runner. Every answer is the agent's claim (the
// trust note above): decoded with DisallowUnknownFields under the same limits, checked (refnames
// begin refs/ and match what was asked, shas are 40 or 64 hex), and never a permission.
type Runner interface {
	Status(ctx context.Context) (StatusResponse, error)
	Refs(ctx context.Context, req RefsRequest) (RefsResponse, error)
	// Bundle streams the answer into w and stops at MaxBundleBytes; it returns the byte count.
	Bundle(ctx context.Context, req BundleRequest, w io.Writer) (int64, error)
	Fetch(ctx context.Context, bundle io.Reader, req FetchRequest) (FetchResponse, error)
	Diff(ctx context.Context, req DiffRequest) ([]byte, error)
}

// NewRunner is the HTTP client for one sandbox. endpoint is backend.Sandbox.Endpoint, host:port,
// the value backend.Dial takes (dial.go:38); token is the sandbox's AttachToken. Each op is
// POST "http://" + endpoint + backend.GitPath + op, with "Authorization: Bearer " + token.
// The http.Client follows no redirect (CheckRedirect returns http.ErrUseLastResponse) and its
// context deadline is the op's runner timeout plus 5 s.
func NewRunner(endpoint, token string) Runner

// NewRunnerTransport is NewRunner with the http.Client's Transport set to rt (seam S8-P3.5):
// the request is still POST "http://" + endpoint + backend.GitPath + op with the bearer, and
// rt carries it, so a machine whose sandboxes are reached through a broker (phase 8's
// remotebe.DialLane, env task 25b) rewrites it there. nil rt is NewRunner exactly. Every check
// NewRunner makes (no redirect, the deadline, the decode limits) is unchanged. NewRunner is
// NewRunnerTransport(endpoint, token, nil).
func NewRunnerTransport(endpoint, token string, rt http.RoundTripper) Runner

// RunnerError is every failed op. Code is the runner's ErrorResponse.Code; GitSays is capped at
// gitx.MaxSentence. A runner that does not answer (refused, reset, timed out, a body that is not
// an ErrorResponse) has Status 0, Code "" and GitSays one sentence naming what failed.
type RunnerError struct {
	Status  int
	Code    string
	GitSays string
	Ref     string   // lane_broken only, checked as a refname or "HEAD"
	Missing []string // prerequisite_missing only, each checked as 40 or 64 hex
}

// ErrPrerequisiteMissing is what errors.Is finds on a RunnerError with Code
// "prerequisite_missing" (409): the lane lacks a commit the bundle's basis or a Set named, so
// only a bundle over less basis can refresh it (task 09's Forget; review r4-4 I-1).
var ErrPrerequisiteMissing = errors.New("lanegit: the lane lacks a commit the bundle needs")

// ErrBundleEmpty is what errors.Is finds on a RunnerError with Code "bundle_empty" (409).
var ErrBundleEmpty = errors.New("lanegit: the lane has nothing the basis lacks")

// ErrLaneBroken is what errors.Is finds on a RunnerError with Code "lane_broken" (409): a ref
// outside the runner's namespace names a missing object, or one inside it is a symlink or sits
// in a garbled packed-refs. Fetch's steps 1 and 2, with step 1 run again after step 2 removes
// a file, clear every other broken ref inside it (review r4-5 M-1); no bundle can refresh the
// lane.
var ErrLaneBroken = errors.New("lanegit: a lane ref names a missing object")
```

**How a caller answers a `RunnerError`.** `git_failed` 500, `details.gitSays` = `GitSays`, through
task 04's `gitFailed` (`task-04.md:125-128`), which gains that one case. So decision 8 still has
two codes. `ErrBundleEmpty` is not an error to the user: task 09 reads it as nothing to land.
`ErrLaneBroken` is `git_failed` like any other to a route; task 09's refresh reads it and stops
sending that lane bundles (`Syncer.Broken`, task 07; review r4-3 I-2). `ErrPrerequisiteMissing`
is `git_failed` too, and since the runner sends no `gitSays` for it, the client writes
`GitSays` as `the lane lacks <sha>[, <sha>…]` from `Missing`, so an errs entry or a
`details.gitSays` is never empty. Task 09's refresh reads it, and only it, as "forget this
lane's basis" (review r4-4 I-1).

```go
// pool.go — a Runner for a lane: the live session's sandbox when one is on the lane, else a
// runner-only sandbox this pool starts and destroys.
type Lanes struct {
	Backend      backend.Backend
	Sessions     registry.Store // List, registry.go:51
	Store        lanes.Store    // task 03; For refuses a lane ByID does not answer, so the pool
	                            // never mounts a path the lane index does not hold
	Image        string         // cfg.SandboxImage, as api.Deps.Image
	Resources    backend.Resources
	ReadyTimeout time.Duration  // cfg.ReadyTimeout, for backend.WaitReady (dial.go:75)
	Idle         time.Duration  // 60 s; a field so tests can shorten it
	Log          *slog.Logger
	// Dial builds the Runner for a sandbox. nil means NewRunner. It is THE test seam: tasks 11,
	// 14 and 16's tests and TestHostNeverRunsGitInALane set it to lanegittest.Fake.Dial, and
	// pair it with lanegittest.Backend (below), so For runs its real lookup, pool and release
	// logic with no HTTP and no Docker.
	Dial func(endpoint, token string) Runner
	// LocalBundles is the local machine's bundle store (seam S8-P3.4): main.go sets
	// DirBundles{Dir: <Hardening.OwnedRoot>/bundles}, the directory this task used before the seam.
	LocalBundles Bundles
	// Machines answers the machine a lane is on, by lane.MachineID (task-03.md:68) (seam
	// S8-P3.5). nil means every lane is on {Backend, Dial, LocalBundles}: phase 3 exactly, and
	// phase 3 never sets it. For calls it with lane.MachineID; Drop finds the lane with
	// Store.ByID first. A pool entry remembers the Machine it created on, so the Idle timer and
	// Drop destroy there. Live reads only the registry, as before.
	Machines func(ctx context.Context, machineID string) (Machine, error)
}

// Machine is how the pool reaches the lanes of one machine (seam S8-P3.5, env task 25b).
type Machine struct {
	Backend backend.Backend
	Dial    func(endpoint, token string) Runner // nil means the pool's Dial, then NewRunner
	Bundles Bundles                             // nil means LocalBundles, on machine "local" only
}

// Bundles answers the Bundles of the machine lane is on, for the callers of FetchLane,
// ArchiveLane and SendBranch (tasks 07, 09, 11). Machines nil, or a Machine with nil Bundles
// on MachineID "local" (or ""): LocalBundles. A nil Bundles on any other machine is
// ErrNoBundles, so a cloud lane's bundle is never spooled on the laptop's disk.
func (p *Lanes) Bundles(ctx context.Context, lane lanes.Lane) (Bundles, error)

var ErrNoBundles = errors.New("lanegit: this lane's machine has no bundle store")

// readyWaiter is the pool's readiness seam (S8-P3.5): the method set of phase 8's
// backend.ReadyWaiter (env task 22), declared here so phase 3 needs nothing phase 8 adds. A
// Backend that implements it is asked WaitReady(ctx, sb.ID, ReadyTimeout) in step 2; any other
// is polled with backend.WaitReady(ctx, sb.Endpoint, ReadyTimeout), as before. dockerbe and
// katabe do not implement it; lanegittest.Backend does (it answers nil at once).
type readyWaiter interface {
	WaitReady(ctx context.Context, id string, timeout time.Duration) error
}

// Drop destroys the lane's runner-only sandbox, if the pool holds one, waiting for any caller
// still holding it to release, and refuses For for laneID from then on (ErrLaneDropped). Task
// 11's Remove calls it before RemoveTree, so no bind mount is live on a directory being
// deleted, and no sweep can start a sandbox on a path that is going away. A live session's
// sandbox is not the pool's, and Drop leaves it alone: Remove stops sessions first.
func (p *Lanes) Drop(ctx context.Context, laneID string) error

var ErrLaneDropped = errors.New("lanegit: the lane is being removed")

// Live answers, from the registry alone and without starting anything, whether a session's
// sandbox is on the lane now (For's step 1, less the Backend.Get), and lastExit, the latest
// UpdatedAt (session.go:77) among the lane's StatusExited records (kept, session.go:26-29).
// Task 07's sweep and task 09's refresh use it to leave a quiet sessionless lane alone
// (review r4-1 I-6).
func (p *Lanes) Live(ctx context.Context, laneID string) (live bool, lastExit time.Time, err error)

// ErrNoRunner: no live session is on the lane and this backend cannot start a runner-only
// sandbox (katabe refuses Mounts, backend.go:129-131). It is decided from Create's answer,
// errors.Is(err, backend.ErrUnsupported), and wraps it; never from Kind(), because phase 8's
// remotebe is KindKata and can serve step 2 (seam S8-P3.5). katabe's Create refuses Mounts
// with ErrUnsupported (katabe/kata.go:84-85), so katabe's answer is the same as before.
// The wrap keeps both sentences, fmt.Errorf("%w: %w", ErrNoRunner, err) (env seams review 1,
// M3): every ValidateSpec failure also wraps ErrUnsupported (backend.go:220-251) and dockerbe's
// Create returns it, so a broken Resources, image or WorkDir still reads as itself after
// "no git runner can reach this lane".
var ErrNoRunner = errors.New("lanegit: no git runner can reach this lane")

// For returns a Runner for lane and a release the caller must call exactly once.
func (p *Lanes) For(ctx context.Context, lane lanes.Lane) (Runner, func(), error)

// HeldSet is the ids of the runner-only sandboxes the pool holds, under a mutex. main.go makes
// it before the reaper (main.go:87) and passes its Has as reaper.Config.Held, so rule 4 does not
// destroy them as orphans. The pool adds an id before Backend.Create and drops it after Destroy.
type HeldSet struct{ /* sync.Mutex; map[string]struct{} */ }

func NewHeldSet() *HeldSet
func (h *HeldSet) Has(sandboxID string) bool
```

`Lanes` also carries `Held *HeldSet`.

1. **A live session.** A registry `Session` whose `WorktreeID` (`session.go:48`) is `lane.ID`
   and whose `SandboxID`, `Endpoint` and `AttachToken` (`session.go:71-74`) are set, confirmed
   by `Backend.Get(ctx, s.SandboxID)` (`backend.go:291`) answering a non-empty `Endpoint`. For
   answers `Dial(sb.Endpoint, s.AttachToken)` (nil `Dial` → `NewRunner`, the seam below; review
   r4-2 M-4) and a no-op release. The token is read from
   the registry and never logged, as phase 2 does for `/attach`.
2. **Otherwise, a runner-only sandbox.** `Backend.Create` (`backend.go:290`) with
   `Spec{ID: "lg-" + 16 random hex, Image, Command: "sleep", Args: ["infinity"], AttachToken:
   32 random bytes (crypto/rand) as hex, Resources, WorkDir: "/home/dev/work", Mounts:
   [{HostPath: lane.Path, ContainerPath: "/home/dev/work"}]}` (`Spec` at `backend.go:141-152`;
   `Mounts` is docker only, `:151`). `/home/dev/work` is `sessionWorkDir` and the runner's
   `-git-worktree` default. Then readiness: `WaitReady(ctx, sb.ID, ReadyTimeout)` when the
   Backend implements `readyWaiter`, else `backend.WaitReady(ctx, sb.Endpoint, ReadyTimeout)`
   (S8-P3.5). The token lives only in the pool's memory. A `Create` error that `errors.Is`
   `backend.ErrUnsupported` is `ErrNoRunner` wrapping it with both sentences, `%w: %w` (katabe's
   answer; a dockerbe `ValidateSpec` refusal keeps its own sentence); any other `Create`
   error is returned as itself. With `Machines` set, `Backend`, `Dial` and `Bundles` in steps 1
   to 5 are the lane's Machine's, and `Held` is the one set for every machine (ids are
   `lg-` + 16 random hex, so they do not collide across machines).
3. **One per lane, reused, then destroyed.** The pool keeps one entry per lane with a count of
   holders; two concurrent `For` calls start one sandbox. The last release starts a timer of
   `Idle` (60 s); a `For` inside it reuses the sandbox, and when it fires the pool calls
   `Backend.Destroy`. When a session is live on a lane that also has an idle runner-only
   sandbox, `For` uses the session and destroys the idle one.
4. **The reaper.** The reaper's rule 4 (`reaper.go:363-409`) destroys a managed sandbox with no
   session record once `createGrace` has passed. A runner-only sandbox has no record, so
   `reaper.Config` gains `Held func(sandboxID string) bool`, and rule 4 skips an id for which
   it answers true; `reaperConfigFor` (`main.go:215`) takes it as a fifth argument, and a nil
   `Held` holds nothing, so phase 2's reaper tests are unchanged. After a restart nothing holds
   the old ones, and rule 4 reaps them, which is the crash path it already is.
5. **Drop.** `Drop(laneID)` marks the id dropped under the pool's mutex (a later `For` is
   `ErrLaneDropped`, before any lookup), waits for the entry's holders to release, then
   destroys the runner-only sandbox, if any, without the `Idle` wait. A `Destroy` failure is
   returned, and task 11 stops before `RemoveTree`. The mark lives as long as the pool; lane ids
   are never reused (task 03), so it never needs clearing. This also closes a window: Docker's
   `Binds` creates a missing host path as an empty directory (from memory, not run), so a sweep
   that reached step 2 after `RemoveTree` would leave an orphan directory under `CloneRoot`.

```go
// bundle.go — lane work in, product commits out. All three run git in the PROJECT only, through
// the project machine's gitx.Runner (the host's gitx.Exec in phase 3), with Scope Project. The
// bundle lives in a Bundles (seam S8-P3.4, env R4 ruling 3), on the machine whose git reads
// it; the local one is DirBundles over <Hardening.OwnedRoot>/bundles, exactly the file this
// task used before the seam. Each bundle is removed by a defer on every path, success included.

// Bundles is where a lane bundle is spooled. Every id is ^[0-9a-f]{16}$; an id it answers is
// checked before use, since on a cloud machine it is that machine's claim.
type Bundles interface {
	// Put streams r into a new bundle and answers its id. It never reads r ahead into memory.
	// On any failure it closes r when r is an io.Closer (a relay's *io.PipeReader, so the
	// runner's Bundle unblocks with io.ErrClosedPipe) and leaves no bundle behind.
	Put(ctx context.Context, r io.Reader) (string, error)
	// Reserve answers a new id for a bundle git itself writes at Path(id) (SendBranch).
	Reserve(ctx context.Context) (string, error)
	// Open streams bundle id, for the caller to close. Each Open starts at the first byte.
	Open(ctx context.Context, id string) (io.ReadCloser, error)
	// Remove deletes bundle id; nil when it is already gone.
	Remove(ctx context.Context, id string) error
	// Path is where git on that machine reads or writes bundle id: an argument to a Scope
	// Project call, never opened by the caller. A malformed id answers "".
	Path(id string) string
}

// DirBundles is Bundles on this process's disk: Dir is created mode 0700 on first use; Put and
// Reserve create <Dir>/<16 random hex>.bundle with O_EXCL, mode 0600 (Reserve's stays empty
// until git replaces it); Put removes its file on any failure; Open is os.Open; Remove is
// os.Remove with not-exist as nil; Path is filepath.Join(Dir, id+".bundle").
type DirBundles struct{ Dir string }

var _ Bundles = DirBundles{}

// BundleRefused is every reason FetchLane will not take a bundle. No ref has moved.
type BundleRefused struct{ Why string }

// FetchLane asks runner for a bundle of refs over basis, checks it in the project, and fetches
// it with refspecs. refs is exactly one ref, under refs/heads/ (task 07's one call), so the
// basis either reaches it (ErrBundleEmpty, task 07's step 3b) or the bundle carries it: no
// bundle can leave out a ref FetchLane asked for (review r4-4 C-1). Each refspec is
// +<that ref>:<dst> with dst under refs/panacea/lanes/. It answers the fetched refs,
// ErrBundleEmpty, a *BundleRefused, or a *gitx.Error from a project call.
func FetchLane(ctx context.Context, git gitx.Runner, bundles Bundles, projectPath string,
	runner Runner, refs, basis, refspecs []string) (map[string]string, error)

// ArchiveLane is task 11's archive of off-branch work (review I6), and FetchLane's only sibling.
// refs maps each refname to the sha the runner answered for it (Refs, or Head for "HEAD"); each
// refname begins refs/heads/ or is exactly "HEAD"; each refspec is +<one of refs>:<dst> with
// dst under prefix+"/", and prefix is refs/panacea/archive/offbranch/<lane id>/<unix> (task 11's
// OffBranchPrefix, no trailing slash). basis is shas the project holds, as FetchLane's is: the
// archive lands in the project, so any project-held sha is a valid prerequisite (review r4-3
// I-1). It answers every dst written, a *BundleRefused, or a *gitx.Error; never
// ErrBundleEmpty (review r4-4 C-1).
func ArchiveLane(ctx context.Context, git gitx.Runner, bundles Bundles, projectPath string,
	runner Runner, refs map[string]string, basis []string, prefix string, refspecs []string) (map[string]string, error)

// Sent is SendBranch's answer (review r4-4 C-1). The bundle ID carries Carried, the refs the
// kept basis does not reach. Reached are the others, refname → sha: each an ancestor of a kept
// basis sha, so a lane holding the basis holds it, and task 09 hands each to Runner.Fetch as a Set.
type Sent struct {
	ID      string // in the Bundles SendBranch was given; Open it once per lane (S8-P3.4)
	Carried []string
	Reached map[string]string
}

// SendBranch writes a bundle of refs over basis from the project into bundles, for task 09's
// refresh to hand to Runner.Fetch. The caller calls done, which removes the bundle.
func SendBranch(ctx context.Context, git gitx.Runner, bundles Bundles, projectPath string,
	refs, basis []string) (sent Sent, done func(), err error)
```

**The split** (review r4-4 C-1), shared by `ArchiveLane` and `SendBranch`, in the project
(Class `Read`). `bundle create` leaves out every ref the basis reaches (the `bundle` op), so
neither asks for one. Keep the `basis` shas `cat-file -e <sha>^{commit}` finds. A ref's sha is
SendBranch's `rev-parse --verify -q <ref>` in the project, or ArchiveLane's `refs` value (the
runner's, which is data). A ref is **reached** when `cat-file -e <sha>^{commit}` finds its sha
and `merge-base --is-ancestor <sha> <k>` exits 0 (OKExit 1) for some kept `k`: task 07 step
3b's check (`task-07.md:176-183`). The rest are **carried**. A sha reachable from the kept set
is an ancestor of one of them, so "no ref carried" is exactly the old `rev-list --count … = 0`,
and is SendBranch's `ErrBundleEmpty`, decided before `bundle create`. It costs one `merge-base`
per ref and kept sha: SendBranch's two refs by its basis (a told sha, a claim's two, or a
tip's parents), ArchiveLane's refs by at most three.

**`FetchLane`, in order.** Any failure stops it; the file goes with the defer.
1. **Arguments.** `refs` (exactly one) and `refspecs` are checked as above, and the ref passes
   `check-ref-format` in the project (Class `Read`). A failure is `*BundleRefused` and the runner
   is not called.
2. **Spool** (S8-P3.4's relay). `pr, pw := io.Pipe()`; a goroutine runs `runner.Bundle` into
   a counting writer over `pw` and ends with `pw.CloseWithError(err)`; `id, err :=
   bundles.Put(ctx, pr)` under one deadline, `BundleTimeout` + 5 s, over both legs; then
   `defer bundles.Remove(context.WithoutCancel(ctx), id)`. Byte `MaxBundleBytes + 1` stops the
   copy and is `*BundleRefused{"larger than MaxBundleBytes"}`; the runner's own 413 is the same
   refusal. `ErrBundleEmpty` returns as itself. Steps 3 to 5 name `bundles.Path(id)`; a Path of
   "" is `*BundleRefused` before any git. Locally Put is the same O_EXCL file this step wrote.
3. **`bundle verify <file>`** in the project (Class `Read`). It checks the header and that every
   prerequisite is in the project, so a truncated file or a missing prerequisite fails here.
4. **`bundle list-heads <file>`** (Class `Read`), parsed as `<sha> SP <refname>` lines. The set of
   refnames must equal `refs` exactly: no extra, none missing, no duplicate, no `HEAD`, each under
   `refs/heads/`, each sha 40 or 64 hex. Otherwise `*BundleRefused`.
5. **The fetch** (Class `Local`): `fetch --atomic --no-auto-maintenance --no-recurse-submodules
   --no-tags --no-write-fetch-head --quiet <file> <refspecs…>`. `--no-recurse-submodules`: a lane
   bundle that moves a gitlink must not start a `fetch` in the project's populated submodule, from
   its remote, under the project lock (review r4-2 M-2). `transfer.fsckObjects=true` is given as
   `GIT_CONFIG_COUNT=1`, `GIT_CONFIG_KEY_0`, `GIT_CONFIG_VALUE_0` in `Call.Env`, the way task 13's
   `PushEnv` sets config. Not `-c`: task 02 refuses a caller's `Args[0] == "-c"` with
   `ErrCallShape` (`TestHGHCallShape`). `--atomic` makes a partial failure move no ref. A failed
   fetch, fsck's included, is `*BundleRefused` carrying git's sentence.
6. **Read back** with `for-each-ref --format=%(refname)%00%(objectname)` over each `dst` (Class
   `Read`), which is the answer. Nothing here reads the lane's `.git`.

**`ArchiveLane`, in order** (review r4-2 C-1). The same six steps and one shared body, with
these changes, so neither entry point widens the other's namespace and an archive carries only
what the project lacks:
1. **Arguments.** `prefix` passes `check-ref-format` and begins
   `refs/panacea/archive/offbranch/`, with the lane id and a decimal `<unix>` as its next two
   and last segments. Each ref is `HEAD` or under `refs/heads/`; `HEAD` is matched literally
   and skips `check-ref-format`, which refuses it (exit 1 on both gits; review r4-3 M-1), and
   every other ref passes it; each sha is 40 or 64 hex. Each refspec is
   `+<ref>:<dst>` with `dst` under `prefix + "/"`, so a `dst` under `refs/panacea/lanes/` or any other
   namespace is `*BundleRefused`, and FetchLane still refuses every `dst` outside
   `refs/panacea/lanes/`. Each `basis` sha is 40 or 64 hex.
2. **The split** (above; review r4-4 C-1). No carried ref: no runner call, straight to step 7.
   Otherwise the old steps 2 to 6 run over the carried refs only: `BundleRequest.Refs` is the
   carried refs, `Basis` the kept shas, and the runner drops the ones the lane lacks.
3. **List-heads (step 4).** The set of refnames must equal the carried refs exactly, as for
   FetchLane, but `HEAD` is admitted when they hold it. That is the only place `HEAD` is
   admitted. A runner `ErrBundleEmpty` (the lane's refs moved since Inspect listed them) is
   `*BundleRefused{"the lane's refs moved since they were listed"}`, so task 11 never sees it.
4. **The runner check.** `BundleRequest.Refs` admits exactly `HEAD` (Validation above), so the
   runner answers the archive's bundle rather than 400.
5. **One retry with no basis** (review r4-3 I-1). When `bundle verify` fails with a
   non-empty kept basis, ArchiveLane asks the runner once more for the carried refs with
   `Basis: []` and runs steps 2 to 6 again; a second failure is the answer. Every prerequisite
   is an ancestor of a project-held sha, so this should not happen; the retry keeps a project
   with a missing ancestor object archivable, at the old whole-history cost.
6. **The fetch** keeps `--atomic`, so a failed fetch writes no ref.
7. **The reached refs** (review r4-4 C-1): one `update-ref --stdin` transaction in the project
   (Class `Local`), `start`, an `update <dst> <sha>` per reached ref, `commit`. This is step
   3b's move of a host ref to a commit the project holds and reaches from its own basis, so
   R4.4 is not weakened. If it fails, the refs step 6 wrote are deleted in one more
   (`delete <dst> <sha>` each), so a failed archive still writes no ref, and the answer is
   the first failure; a failed rollback is logged, and its refs are kept, as every archive
   ref is (Q5).

The spy row is task 11's (remove), and it sees only project calls here.

**`SendBranch`, in order.** Each call in the project. The split (above): no carried ref is
`ErrBundleEmpty`. Then `id := bundles.Reserve(ctx)`, `bundle create <bundles.Path(id)>
<carried…> --not <kept…>` (a Path of "" is an error before any git; any failure removes id),
and `Sent{ID, Carried, Reached}`; a ref the kept basis reaches is never asked of `bundle create`, which would
leave it out and so fail the runner's fetch of its refspec (`fatal: couldn't find remote ref`,
exit 128; r4-4 Probe 1 A). `bundle create` is Class `Local` (2 min, `task-02.md:64`, which equals
`BundleTimeout`). It writes only the file, but it packs every object it carries, and
`MaxBundleBytes` is sized for 120 s, not for `Read`'s 10 s (review r4-2 I-2). A file past
`MaxBundleBytes` is deleted and refused, because the runner's `fetch` would 413 it. That check
is `DirBundles.Size(id) (int64, error)`, an `os.Stat`, taken through an optional interface
`interface{ Size(id string) (int64, error) }`, not a sixth `Bundles` method: a `Bundles` without
it (phase 8's, env task 25b) is capped by the runner's own 413 at `Fetch` instead, and the
lane's errs entry names it. Locally the answer is the one this paragraph gave before the seam.

**The Class of each host bundle move.** `bundle create` (SendBranch) and the `fetch`
(FetchLane, ArchiveLane) are `Local`: each moves up to `MaxBundleBytes`. ArchiveLane's
`update-ref --stdin` and its rollback are `Local`, as every host ref write is.
`check-ref-format`, `cat-file -e`, `rev-parse --verify`, `merge-base --is-ancestor`,
`bundle verify`, `bundle list-heads` and the read-back stay `Read`: `list-heads` reads only
the header, and `verify` reads the header and looks up each prerequisite.
`TestBundleMovesUseTheBundleBudget` (below) pins the split.

**Reason codes.** The closed `details.reason` list in task 04's `gitdeps.go` (edit unit U5) gains
`bundle_refused`: 409 `conflict`, `details.why` = `BundleRefused.Why`. A `*gitx.Error` from a
project call stays `git_failed`, and a `RunnerError` is `git_failed` 500 as above.

**Wiring (p2 files).** `api.GitDeps` gains a `// ── task 24 ──` block holding `LaneGit
*lanegit.Lanes` (one block per task, `task-04.md:55`). In `cmd/api/main.go`:

```go
	// just above the reaper (main.go:87), which must know the pool's sandboxes from its first sweep
	laneHeld := lanegit.NewHeldSet()
	rp := reaper.New(reaperConfigFor(cfg, store, be, log, laneHeld.Has))

	// ── phase 3 task 24 ── after task 04's block, before api.New (main.go:132)
	// R4: lane git runs inside the sandbox. The pool finds the runner or starts one.
	laneGit := &lanegit.Lanes{
		Backend: be, Sessions: store, Store: laneStore, Held: laneHeld,
		Image: cfg.SandboxImage, Resources: cfg.Resources(), ReadyTimeout: cfg.ReadyTimeout,
		Idle: 60 * time.Second, Log: log,
		// S8-P3.4: the directory bundle.go used before the seam. hardening is task 04's
		// gitx.MachineHardening value (task-04.md:225). Machines stays nil: phase 8 sets it.
		LocalBundles: lanegit.DirBundles{Dir: filepath.Join(hardening.OwnedRoot, "bundles")},
	}
	defer laneGit.Close(context.Background()) // destroys every runner-only sandbox it holds
	gitDeps.LaneGit = laneGit
```

`Close(ctx) error` joins to `pool.go`. The reaper line is the one edit outside the block, and it
is one argument.

**`lanegittest.Fake`** is a `Runner` for tests. It runs plain `os/exec` git with `-C <Dir>` in a
test clone, the same argv as the runner's ops above, `fetch`'s two repair steps (step 1's
lock sweep and its second run included, review r4-5), its
prerequisite check (`prerequisite_missing`), its Sets and its `lane_broken` rule included
(review r4-4). Before any git it runs the runner's **Validation**
(`lanegittest/validate.go`: the host's copy, since Go's internal rule keeps `control-plane`
from importing the runner's), and a refusal is `RunnerError{Status: 400, Code}`, as the client
decodes the runner's. `TestFakeValidatesAsTheRunner` runs every
`testdata/gitrunner/validation/*.json` case through it, and runner test 7 runs the same files
through the runner, so the two copies cannot drift apart unseen (review r4-3 C-1). It never
goes through `gitx.Exec`, so the spy below sees only production calls. `Fake.BundleOverride []byte`, when set, is what `Bundle`
writes, so a test can hand `FetchLane` a truncated or hostile bundle.

**The seam behind `Lanes.For`.** `lanegittest.Fakes` maps a lane path to its `*Fake`; its
`Dial(endpoint, token string) lanegit.Runner` is what a test puts in `Lanes.Dial`.
`lanegittest.Backend` is a `backend.Backend` whose `Create` answers `Sandbox{Endpoint:
"fake:" + <the Spec's one mount's HostPath>}` and whose `Get`, `Destroy` and `List` keep a map,
so `Fakes.Dial` finds the lane's `Fake` from the endpoint. Tasks 11, 14 and 16's tests and the
spy test below build `Lanes{Dial: fakes.Dial, Backend: lanegittest.Backend{}, …}`: the pool's
real `For`, reuse, release and `Drop`, with no HTTP and no Docker.

**Fixtures.** `lanegit`'s wire test reads `testdata/gitrunner` at `../../../testdata/gitrunner`,
round-trips every fixture through its own types with `DisallowUnknownFields`, and compares
`limits.json` with its four constants: the runner's test 7, from the host side
(`TestWireMatchesTheGoldens`).

#### The tests, by name

**The runner's** (sandboxd, the numbered list above; each red first against the named mutant):

| Test | Asserts | Red against |
|---|---|---|
| `TestRunnerRefusesAPathOutsideTheLane` | test 1: `../x`, `/etc/passwd`, `:(top)x`, `a/../../x`, `a\b` (and the rest of test 1's list) → 400 `path_outside_lane`; the runner's exec recorder holds no call | a runner that passes the path through to `git diff` |
| `TestRunnerNeedsTheToken` | test 6: no bearer, or a wrong one → 401, the runner never reached | a `/git/v1/` route mounted outside `s.git` |
| `TestRunnerRefusesAnUnknownOp` | `POST /git/v1/push`, `/git/v1/`, `/git/v1/status/x` → 404 `unknown_op`; `GET /git/v1/status` → 405; no exec | a router that falls through to a default op |
| `TestRunnerRefusesARefspecOutsideItsNamespace` | test 3's second half: a `dst` of `refs/heads/main`, `refs/panacea/lanes/x`, `refs/remotes/origin`, or two `:` → 400, no exec | a `fetch` that trusts the host's refspec |
| `TestRunnerCapsOutputAndTimesOut` | test 5 | a cap that truncates to a 200 |
| `TestRunnerFetchRepairsUnreadableRefs` | test 12's second half (review r4-4 I-2): empty and garbled ref files in `refs/remotes/{origin,upstream}/` removed, a dangling symref kept, a symlink `lane_broken`, a symref `<dst>` dropped before the fetch | a repair that only lists refs, which git's own listing skips; a fetch through an agent's symref |
| `TestRunnerFetchTakesSets` | test 13 (review r4-4 C-1) | a Set that trusts the lane holds its sha; a Set or delete without `--no-deref` |
| `TestRunnerNamesAMissingPrerequisite` | test 14 (review r4-4 I-1) | a missing prerequisite answered as `git_failed` `git exited 1`, which task 09 cannot tell from any other failure |
| `TestRunnerFetchRepairsAnUncoveredPackedRef` | test 12's packed-ref case (review r4-5 M-1): an empty loose `origin/other` over a `packed-refs` line naming a missing object → 200, the line gone, `origin/HEAD` resolves | step 1 run once, before step 2 uncovers the packed line: `lane_broken` for a ref in the runner's own namespace |
| `TestRunnerLeavesNoLockBehind` | test 15 (review r4-5 M-2): a lock older than `BundleTimeout` swept, a fresh one kept; a timed-out git gets SIGTERM, and one that ignores it is killed after `KillDelay` | a sweep that skips every `.lock`, or one with no age test; `CommandContext`'s default SIGKILL; a Cancel with no `WaitDelay` |
| `TestRunnerFetchStartsNoMaintenance` | the recorded `fetch` argv holds `--no-auto-maintenance`; with `GIT_TRACE` on the runner's git, no `maintenance` or `gc` child starts | a `fetch` that auto-gcs the agent's lane mid-op |
| `TestSandboxRunnerStatusIsPorcelainV2Z` | Docker, behind a `docker` build tag (no test in either module starts Docker today, grep 2026-09-24; this is the first), run by hand with `go test -tags docker ./sandboxd/internal/gitrunner/`: in the built image (`images/sandbox/Dockerfile`, its `apt-get install … git` line, bookworm's git), started as a **runner-only** sandbox is (no session env, `sleep infinity`, `docker exec` of `sandboxd`), with the lane a host-created `t.TempDir()` bind-mounted by `-v <tempdir>:/home/dev/work`, so its owner is not `dev`: `status` on a lane with one change of each kind answers `1 `, `2 `, `u `, `? ` records, NUL-terminated, and `# branch.oid` headers absent | a `G` without `-c safe.directory` (git's `dubious ownership`, exit 128, is 422 `git_failed`); a runner, or a bookworm git, that answers another shape |

**The host's** (`lanegit`, real git through `gitxtest` for the project, `lanegittest.Fake` as the
runner):

| Test | Asserts | Red against |
|---|---|---|
| `TestBundleFetchRefusesABadBundle` | each of: a truncated file; an extra ref beyond the ones asked for; a ref outside `refs/heads/`; a missing prerequisite; one byte over `MaxBundleBytes` (lowered by an unexported option); a corrupt blob caught by fsck → `*BundleRefused`. Every case: `for-each-ref` of the project byte-identical before and after, and `<OwnedRoot>/bundles` empty | a `FetchLane` that fetches without `verify` and `list-heads` |
| `TestBundleRoundTrip` | a lane commit on `refs/heads/b` → `FetchLane` → task 07's `LaneRef(<id>)`, `refs/panacea/lanes/<id>` (`task-07.md:36`; not `…/<id>/b`, which git cannot hold beside it), in the project at the lane's sha; `SendBranch` of a project commit → `Fake.Fetch` → the lane's `refs/remotes/origin/main` at that sha; with nothing new, `ErrBundleEmpty` both ways. Red first (review r4-4 C-1): `SendBranch(refs: [refs/heads/main, refs/remotes/origin/main], basis: [T])` with the project's `origin/main` an ancestor of T → `Sent.Carried` is `[refs/heads/main]`, `Sent.Reached` is `{refs/remotes/origin/main: <its sha>}`, the bundle's `list-heads` names only `refs/heads/main`, and a `Fake.Fetch` of it with that Set lands both | a round trip that loses a ref or a prerequisite; a `SendBranch` that asks `bundle create` for a ref the basis reaches |
| `TestClientMapsTheCodes` | an `httptest` runner answering each 409: `bundle_empty` → `errors.Is(err, ErrBundleEmpty)`; `lane_broken` with `Ref` → `ErrLaneBroken`, `Ref` kept; `prerequisite_missing` with `Missing: [<sha>]` → `ErrPrerequisiteMissing`, `Missing` kept, `GitSays` `the lane lacks <sha>`; a `Missing` entry of 39 hex → a Status-0 `RunnerError`, as any answer that fails the client's checks (review r4-4 I-1) | a client that folds every 409 into one error, so task 09 Forgets on any of them |
| `TestArchiveRoundTrip` | red first (review r4-2 C-1). A lane with a detached HEAD at a commit no branch holds and a second branch `refs/heads/x` → `ArchiveLane(refs: {HEAD: <its sha>, refs/heads/x: <its sha>}, basis: [], prefix: OffBranchPrefix(id, 1700000000), refspecs: +HEAD:<prefix>/HEAD, +refs/heads/x:<prefix>/heads/x)` → both refs in the project at the lane's shas. Refused, as `*BundleRefused` with the project's `for-each-ref` byte-identical: a `dst` under `refs/panacea/lanes/`; a prefix outside `refs/panacea/archive/offbranch/`; and `FetchLane` given `HEAD` or an archive `dst`. With a basis (review r4-3 I-1): the project holds LaneTip, and `refs/heads/y` is one commit past it → `ArchiveLane(refs: {refs/heads/y: …}, basis: [LaneTip, a sha the project lacks], …)` → the Fake records a `BundleRequest.Basis` of `[LaneTip]` only, and the spooled bundle's `rev-list --count` over its heads, less LaneTip, is 1; a Fake whose first bundle fails the project's `verify` → one more `Bundle` with `Basis: []`, and the archive lands. Red first (review r4-4 C-1): a detached HEAD at LaneTip and `refs/heads/x` one past it, basis `[LaneTip]` → the Fake records `BundleRequest.Refs` `[refs/heads/x]` only, the bundle carries one commit, and both `<prefix>/HEAD` (at LaneTip, by the `update-ref --stdin`) and `<prefix>/heads/x` are written; every ref at or behind LaneTip → no `Bundle` call and every archive ref written; the same with a `<prefix>/HEAD.lock` file held in the project, so the `update-ref --stdin` fails → that error, and `<prefix>/heads/x` is gone again; a Fake answering `bundle_empty` → `*BundleRefused`, never `ErrBundleEmpty` | an archive that goes through `FetchLane`'s namespace check, or a `FetchLane` widened to take it; an archive of the whole history on every removal; an archive that asks the runner for a ref the basis reaches, which `bundle create` leaves out |
| `TestFakeValidatesAsTheRunner` | red first (review r4-3 C-1): every `testdata/gitrunner/validation/*.json` case through `lanegittest.Fake` → its `code` (`""` means the op ran), and a refused case runs no git; `Fake.Refs(Names: [refs/heads/*])` → `RunnerError{Status: 400, Code: "bad_request"}` | a Fake that runs whatever the host sends, so task 11's glob passes every host test and fails only task 23 |
| `TestBundleMovesUseTheBundleBudget` | through `gitxtest.Spy` (U3), whose `Spawn.Env` holds `GIT_OPTIONAL_LOCKS=0` on a `Read` call only (`TestExecReadClassSetsNoOptionalLocks`) and no Remote prefix on these: `SendBranch`'s `bundle create`, the `fetch` of `FetchLane` and `ArchiveLane`, and ArchiveLane's `update-ref --stdin` are `Local`; `verify`, `list-heads`, `cat-file -e`, `rev-parse --verify`, `merge-base --is-ancestor` and the read-back are `Read` (review r4-2 I-2; r4-4 C-1's split). Task 02's `TestExecTimeoutIsPerClass` already proves a `Local` call outlives the `Read` budget | a `bundle create` as `Read`, which times out at 10 s on a large refresh |
| `TestPoolStartsOneRunnerForASessionlessLane` | a fake `Backend`: two concurrent `For` on a lane with no session → one `Create`, with `Command "sleep"`, `Args ["infinity"]`, one mount of `lane.Path` at `/home/dev/work`, a 64-hex token; `Held.Has` true; after both releases and `Idle`, one `Destroy` and `Has` false; a lane `ByID` lacks → error, no `Create` | a pool that starts a sandbox per call, or never destroys it |
| `TestPoolUsesTheLiveSession` | a registry `Session` on the lane with `SandboxID`, `Endpoint`, `AttachToken` → no `Create`; the runner's requests carry that token to that endpoint | a pool that ignores the running session |
| `TestPoolDropDestroysAndRefuses` | a runner-only sandbox held by one caller: `Drop` blocks until that release, then one `Destroy` with no `Idle` wait and `Held.Has` false; a later `For` on the id → `ErrLaneDropped` and no `Create`; `Drop` on a lane with a live session → no `Destroy` of the session's sandbox | a `Drop` that waits for `Idle`, or a `For` that restarts a dropped lane |
| `TestPoolLiveReadsOnlyTheRegistry` | a lane with one running and one exited session → `live` true; with only exited ones → `live` false and `lastExit` the later `UpdatedAt`; neither case calls `Backend.Create`, `Get` or `Dial` | a `Live` that starts a sandbox to answer |
| `TestPoolDialSeam` | `Lanes{Dial: fake.Dial, Backend: lanegittest.Backend{}}` → `For` on a sessionless lane answers the Fake, and the Fake saw the pool's endpoint and token; `Dial` nil → the Runner is `NewRunner`'s | a `For` that builds `NewRunner` whatever `Dial` holds |
| `TestWireMatchesTheGoldens` | the fixtures paragraph above | a field renamed on the host side only |

Seams S8-P3.4 and S8-P3.5 (phase 8's asks; env tasks 21b and 25b consume them). Every row above
runs with `Machines` nil and `LocalBundles` a `DirBundles`, so it is phase 3's path unchanged.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFetchLaneSpoolsThroughBundles` | a recording `Bundles` over a `DirBundles`: `TestBundleRoundTrip`'s lane → one `Put`, then `bundle verify`, `list-heads` and `fetch` each name `Path(id)`, then one `Remove(id)`; the refused cases of `TestBundleFetchRefusesABadBundle` → `Remove` too; `Path` answering "" → `*BundleRefused`, no git | a `FetchLane` that still writes `<OwnedRoot>/bundles` itself, the wrong disk for a cloud lane |
| `TestFetchLaneRelayStreams` | a `Bundles` whose `Put` reads 1 MiB and then blocks until the runner has written 2 MiB → both progress; a `Put` that fails mid-body → the runner's `Bundle` returns `io.ErrClosedPipe`, and no goroutine is left (`runtime.NumGoroutine` before and after, settled; no new dependency) | a spool buffered in memory, or a relay goroutine leaked on a failed `Put` |
| `TestSendBranchReservesAndCreatesAtPath` | a recording `Bundles`: `Reserve` once, `bundle create`'s first path argument is `Path(id)`, `Sent.ID == id`; `done()` → `Remove(id)`; two `Open(id)` calls each read the whole bundle; a `Bundles` without `Size` → no size refusal, and one with it past `MaxBundleBytes` → refused and removed | `Sent` holding a laptop `*os.File`, or a second lane of task 09's group reading an exhausted stream |
| `TestDirBundlesIDsAndModes` | `Put` and `Reserve` answer `^[0-9a-f]{16}$`, files mode 0600 in a 0700 `Dir` (Unix only), O_EXCL; `Remove` of a gone id → nil; `Path("../x") == ""` | an id that lands outside `Dir` |
| `TestPoolReachesTheLanesMachine` | `Machines` answers two `lanegittest.Backend`s for `m-1` and `m-2` → each lane's `Create`, `Get`, `Destroy` (after `Idle`, and on `Drop`) go to its own; `Bundles(lane)` answers each machine's; a nil `Bundles` on `m-1` → `ErrNoBundles`, on `local` → `LocalBundles` | lane work sent to another machine, a runner destroyed on the wrong one, or a cloud bundle spooled on the laptop |
| `TestPoolAsksAReadyWaiter` | a Backend that implements `readyWaiter` and answers `Endpoint "remote:lg-…"` → `WaitReady(ctx, sb.ID, ReadyTimeout)` once, and no HTTP request reaches any listener (a counting `httptest` server as the only one); a Backend without it → `backend.WaitReady` on the endpoint, as before | a `/healthz` poll of an endpoint nothing can dial |
| `TestPoolNoRunnerIsFromCreate` | a Backend with `Kind() KindKata` whose `Create` succeeds → a runner-only sandbox; one whose `Create` answers an error wrapping `ErrUnsupported` → `errors.Is(err, ErrNoRunner)` and `errors.Is(err, backend.ErrUnsupported)`; a dockerbe-shaped `Create` answering `ValidateSpec`'s `spec.Resources must be positive …` → both `errors.Is` hold and `err.Error()` still contains that sentence (env seams review 1, M3); any other `Create` error → itself, not `ErrNoRunner` | `ErrNoRunner` decided from `Kind()`, which refuses phase 8's remotebe; a single-`%w` wrap that drops the spec refusal's sentence, so a broken `Resources` on dockerbe reads as "no git runner" |
| `TestNewRunnerTransportCarriesTheRequest` | a recording `RoundTripper` → `POST http://<endpoint>/git/v1/status`, `Authorization: Bearer <token>`, the body `NewRunner` sends; a 302 answer is not followed; `rt` nil → the same as `NewRunner` against an `httptest` server | a transport that bypasses `NewRunner`'s checks |

"A corrupt blob caught by fsck" is built as a commit whose author has no email (fsck's
`missingEmail`, an error), carried in a hand-made bundle. Fsck never reads a blob's bytes (bar
`.gitmodules`), and a blob whose bytes do not match its id fails `index-pack` with fsck off, so
it would not show that fsck is on.

**The spy test** (`cmd/api/lanegit_spy_test.go`, over the `api.Deps` that `serve` builds at
`main.go:132`, with `gitxtest.Spy` from U3 as `gitx.Exec`'s process seam and
`lanegittest.Fake` as every lane's runner):

| Test | Asserts | Red against |
|---|---|---|
| `TestHostNeverRunsGitInALane` | the spy records the argv, `Dir` and env of every process `gitx.Exec` starts through create, sweep, observe, land, refresh, undo, remove and diff. For each, none of: `Dir`; the value after `-C`, `--git-dir`, `--work-tree` (either spelling, `=` or next arg); `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`; nor any other argv element, is a path under `CloneRoot` (`cfg.WorktreesDir`, cleaned, compared with `filepath.Rel`, case-folded on Windows). The exceptions are R4.5's creation window, before this `Create`'s `Store.Add`, and nothing else: (a) the calls whose `Call.NewLane` is the `Path` this `Create` drew (Scope None, U3); (b) exactly one Scope Project call, `Dir` the project, `NewLane` "", whose `Args[0]` is `clone` and whose last argument is that same `Path` (task 03 step 4). Exec admits (b) as it stands, since it parses no positional (`task-02.md:142`); task 02's `TestExecRunsTheCreateCloneInTheProject` pins that. A second `clone`, or one after `Store.Add`, fails the row | task 07's pre-R4 step 1, host `rev-parse --verify --quiet refs/heads/<branch>` in the clone (`task-07.md:117`), planted as a flow; and a `fetch <lane path>` from the project, which runs upload-pack in the lane |

The argv rule is wider than the four flags because a local `fetch <path>` reads the lane's `.git`
through upload-pack, which R4.4 retires. Task 24 lands the harness, the `create` row and the two
planted mutants. The task that builds each other flow (07 sweep and observe, 09 land and
refresh, 10 undo, remove, 16 diff) appends its row, and that row is in its acceptance.

#### Behaviour pins (task 01's style, `control-plane/internal/lanegit/behaviour_test.go`)

Real host git, no `gitx`. Each records git's exact words in a comment, as task 01 does.
1. `bundle create <file> <ref> --not <ref's own sha>` exits 128 and writes no file; its sentence
   is recorded. `SendBranch` and the runner decide emptiness first and never rely on it.
2. `bundle verify <file>` in a repository lacking a prerequisite exits non-zero; its sentence
   is recorded.
3. `fetch --no-auto-maintenance --no-write-fetch-head <file> <refspec>` with `GIT_TRACE=1`: the
   trace names no `upload-pack` and no `maintenance` or `gc`.
4. The same fetch with `transfer.fsckObjects=true` from `Call.Env`'s `GIT_CONFIG_*` refuses the
   `missingEmail` bundle, and without it takes it. Not run here. Bundle fsck arrived in git
   2.48.0 (`RelNotes/2.48.0.adoc`, read, not run), so task 02's `MinVersion` is 2.48.0
   (`task-02.md:222-227`) and `TestCheckVersion` refuses anything older; CI's git must meet it.
   If the pin fails at 2.48.0, `MinVersion` rises to the first release it passes on;
   `FetchLane` does not work around it.
5. `bundle create <file> A B --not <t>`, B an ancestor of t and A not, exits 0 with no warning
   and `list-heads` names only A (review r4-4 C-1; run on 2.39.5 and 2.53 in review r4-4
   Probe 1 and fix round 5). This is what the split exists for; if a later git carried B, the
   split would still be correct, only no longer needed.
6. `fetch <file> +<src>:<dst>` with `<dst>` a symref to `refs/heads/y` moves `y`; `update-ref
   -d <symref>` deletes its target; `update-ref --no-deref -d` deletes only the symref (fix
   round 5, both gits). The runner's step 2 and `--no-deref` exist for these.

#### Steps

- [ ] **Step 1: Preflight.** Tasks 02 and 04 merged: `gitx.Exec`, `gitxtest` with U3's `Spy`,
  `lanes.Store`, `api.GitDeps`, task 04's `main.go` block. `git --version` ≥ 2.48.0.
- [ ] **Step 2: Pins.** Write `behaviour_test.go`; run; paste git's sentences into its comments.
- [ ] **Step 3: sandboxd, red.** The runner's tests; each fails for its named reason.
- [ ] **Step 4: sandboxd, green.** `limits.go`, `wire.go`, `ops.go`, `runner.go`; `server.go`'s
  route and `s.git`; `main.go`'s flag and `newServer`. `go test ./sandboxd/... -race` in WSL or
  CI (the package is `linux || darwin`).
- [ ] **Step 5: lanegit, red.** The host's tests and the fixtures test.
- [ ] **Step 6: lanegit, green.** `wire.go`, `client.go`, `pool.go`, `bundle.go`,
  `lanegittest`; `backend.GitPath`; `reaper.Config.Held` and `reaperConfigFor`; the `main.go`
  block. `go test ./control-plane/... -race`.
- [ ] **Step 6a: Mutants for S8-P3.4 and S8-P3.5**, on a `$TEMP` copy, each verified applied:
  `FetchLane` writes its own file under `OwnedRoot` → `TestFetchLaneSpoolsThroughBundles` red;
  `Put` reads into a `bytes.Buffer` first → `TestFetchLaneRelayStreams` red; step 2 on
  `Kind() == KindKata` → `TestPoolNoRunnerIsFromCreate` red; step 2 ignores `readyWaiter` →
  `TestPoolAsksAReadyWaiter` red; `For` uses `p.Backend` whatever `Machines` answers →
  `TestPoolReachesTheLanesMachine` red. Likeliest to survive: the `Machines` one, since every
  other row runs with `Machines` nil.
- [ ] **Step 7: The spy test, last.** Red with each planted mutant, green without.
- [ ] **Step 8: Commit by pathspec**, the files listed in both halves.

#### What this task does not claim

- **A cloud machine's lanes.** Seams S8-P3.4 (`Bundles`) and S8-P3.5 (`Machine`,
  `Lanes.Machines`, `readyWaiter`, `NewRunnerTransport`) are built and tested here only with
  local and fake implementations. That phase 8's `machines.Bundles`, `remotebe.DialLane` and
  `laneGit.Machines` satisfy them is env task 25b's claim, UNVERIFIED here; phase 3 never sets
  `Machines`. A cloud `FetchLane`'s `fetch` also needs `machined` to admit its
  `transfer.fsckObjects` `Call.Env` (step 5). Env task 21's allowlist (item 3) now admits
  exactly that set, alone, on a `project`-scope `local` `fetch` of a live bundle's path (seam
  S8-P3.3; env seams review 1, I1); UNVERIFIED until env task 21 and 21b are built.
- **The Kata runner.** On katabe a sessionless lane is `ErrNoRunner`. Phase 8's `machined`
  serves the runner in the machine pod, against the contract above (S3-P8.1), not P8-D7's
  "phase 3's local git runner unchanged, with §HGH", which phase 3 no longer builds.
- **That a runner's answer is true.** `status`, `refs` and `diff` are the agent's claim about
  its own lane (the trust note). A host lane or archive ref moves in exactly two ways: to a
  tip a bundle the host verified brought in, or (task 07's `ErrBundleEmpty` branch, and
  ArchiveLane's reached refs, review r4-4 C-1) to a commit the project already holds, below a
  basis sha the project holds, checked in the project with `cat-file -e` and `merge-base
  --is-ancestor`. For task 07 step 3b the basis is host ref values, so there the commit is one
  a host ref reaches. ArchiveLane's basis also holds the lane's claimed `origin/<b>` sha
  (task-11.md:138-142), and `cat-file -e` shows only that the project holds it, not that a
  project ref reaches it (review r4-5 M-3). Either way the second way brings no object in, and
  every object the project holds came from the user or from a bundle the host verified, so
  R4.4's "only a verified bundle brings objects in" is not weakened (review r4-1 I-1).
- **A garbled `packed-refs` in the lane.** The repair clears loose ref files only; a
  `packed-refs` git cannot parse is `lane_broken` or `git_failed`, the agent's to fix (review
  r4-4 I-2, residual). The lock sweep, likewise, covers `refs/remotes/{origin,upstream}/` only:
  a `packed-refs.lock` that a SIGKILL leaves is not swept, since it is the agent's file too;
  the SIGTERM Cancel lets git remove it first (review r4-5 M-2, residual).
- **That a stopped agent runs nothing.** `G`'s `core.hooksPath=/dev/null` and
  `core.fsmonitor=false` stop hooks and fsmonitor in a runner-only sandbox. Lane-defined clean
  filter drivers still run on `status` there (review r4-1 I-5, residual).
- **That the runner protects the host.** Its validation protects the lane and the runner.
- **That the Docker pin runs in CI.** It runs by hand, behind its build tag.
