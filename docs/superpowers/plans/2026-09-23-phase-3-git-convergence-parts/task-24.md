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
`refs/panacea/lanes/…`) is the second half, edit unit U2.

R4.3 names this as the seam phase 8's `machined` reuses (P8-D7: "the machine agent serves the
catalog and the git runner inside the machine pod"). Phase 3 builds the Docker half. P8-D7 is
R4's citation; no document in this tree carries it (grep, 2026-09-24).

**Order.** Deps: 02 (the pinned git, `gitx.MaxSentence` and `Sentence`, which the runner's error
copies), 04 (the lane clone the runner serves, mounted at `sessionWorkDir`). Users: 07 (`status`,
`refs` for the lane head), 09 (`bundle` out for landing, `fetch` in for refresh) and 10 through
09, 11 (`refs` for Elsewhere), 16 (`diff`). Plan order: after 06, before 07.

**Trust note.** Every runner answer is the agent's claim about its own lane. The agent runs as the
same uid as `sandboxd` (`dev`, uid 1001, `images/sandbox/Dockerfile:19` and `:24`; the entrypoint
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
call it makes is `git -C <GitWorktree> --literal-pathspecs …`, with `GIT_OPTIONAL_LOCKS=0` in the
environment on the reads (`status`, `refs`, `bundle`, `diff`) and `--no-auto-maintenance` on
`fetch`. That the lane's own config, hooks and filters run here is by construction: they run as
the agent, in the agent's sandbox, which the agent could already do. None of it reaches the host.
The git is the image's (`images/sandbox/Dockerfile:10-12`, Debian bookworm's package); every
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
	Names      []string `json:"names"`      // for-each-ref patterns; each passes check-ref-format
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
	Refs  []string `json:"refs"`  // full refnames to carry; each passes check-ref-format
	Basis []string `json:"basis"` // shas the host already has; 40 or 64 hex
}

// FetchResponse is read back after the fetch and the deletes. A deleted ref is absent.
type FetchResponse struct {
	Refs map[string]string `json:"refs"` // destination refname → sha
}

type DiffRequest struct {
	Path string `json:"path"` // relative to the lane root; see Validation
}

// ErrorResponse is every non-2xx answer except 401, whose body is phase 2's plain text.
type ErrorResponse struct {
	Code    string `json:"code"`              // bad_request, path_outside_lane, unknown_op,
	                                           // bundle_empty, too_large, git_failed, git_timeout
	GitSays string `json:"gitSays,omitempty"` // git_failed only; see Errors
}
```

**The ops.** `G` below is `git -C <GitWorktree> --literal-pathspecs`.

1. **`status`** (task 07's observer). `G status --porcelain=v2 -z --untracked-files=all`, whose
   stdout is `PorcelainV2Z` byte for byte; the runner does not parse it (task 07 does, on the
   host, as untrusted input). Then **Head**, the same for `status` and `refs`:
   `G symbolic-ref -q HEAD` (exit 0: `Ref` is its trimmed stdout; exit 1: detached, `Ref` ""),
   then `G rev-parse --verify -q HEAD` (exit 0: `Sha`; exit 1: unborn, `Sha` ""). Any other exit
   is `git_failed`.
2. **`refs`** (task 07's lane head, task 11's Elsewhere). `G for-each-ref
   --format=%(refname)%00%(objectname) <Names…>`, one ref per line, split at the NUL (a refname
   holds no NUL and no newline: `check-ref-format` forbids control characters). When
   `MergedInto` is set, also `G for-each-ref --format=%(refname) --no-merged=<MergedInto>
   <Names…>`; a ref is `Merged` when the first listing has it and the second does not. Then Head.
   An empty `Names` is 400: the runner never lists every ref by default.
3. **`bundle`** (task 09's landing, through U2). Keep the `Basis` shas that
   `G cat-file -e <sha>` finds; a sha the lane lacks is dropped, not an error, since
   `--not` over a missing object fails the bundle. Then `G rev-list --count <Refs…> --not
   <kept…>`; `0` is 409 `bundle_empty`, decided before `bundle create` so the answer never rests
   on the wording of git's refusal. Then `G bundle create - <Refs…> --not <kept…>`. Its stdout
   goes through a counting writer into a mode-0600 file in a fresh `os.MkdirTemp("/tmp",
   "gitrunner-")` directory, outside the worktree; past `MaxBundleBytes` the runner kills git and
   answers 413 `too_large`. Spooling first is what lets the cap be a status code rather than a
   truncated 200. The file is then sent as `200`, `Content-Type: application/x-git-bundle`, with
   its `Content-Length`, and the directory is removed whatever happens.
4. **`fetch`** (task 09's refresh, R4.5: the host writes a bundle from the project and the runner
   fetches it here). The body is a bundle, read through `http.MaxBytesReader(MaxBundleBytes)` into
   a mode-0600 file in a fresh `os.MkdirTemp("/tmp", "gitrunner-")` directory. Headers: one
   `X-Panacea-Refspec` per refspec, `[+]<src>:<dst>`, at least one; one `X-Panacea-Delete` per
   ref to delete, any number. The runner runs `G bundle verify -q <file>`, then `G fetch
   --no-auto-maintenance --no-tags --quiet <file> <refspecs…>`, then `G update-ref -d <ref>` for
   each delete, removes the directory, and answers `FetchResponse` from
   `G for-each-ref --format=%(refname)%00%(objectname) <each dst and delete…>`. A failed verify
   is `git_failed` and nothing is fetched. The runner never writes `refs/heads/…`: product-branch
   commits arrive as remote-tracking refs, and what the lane does with them is the agent's.
5. **`diff`** (task 16). `G diff --no-color --no-ext-diff --no-textconv -U3 HEAD -- <Path>`; its
   stdout is the body, `200`, `Content-Type: text/x-diff`. Past `MaxJSONBytes` it is 413
   `too_large`. An unborn HEAD makes git fail, which is `git_failed`.

**Validation**, before the op lock and before any git call in the lane; each failure is 400 `bad_request`
unless named otherwise.
- **Refs.** Every refname the host sends (`Names`, `MergedInto`, `Refs`, both sides of a
  refspec, each delete) begins `refs/` and passes `git check-ref-format <ref>`, run without `-C`
  (it needs no repository).
- **Destinations.** A refspec's `<dst>` and every delete sit under `refs/remotes/origin/` or
  `refs/remotes/upstream/`. A refspec is `[+]<src>:<dst>` with exactly one `:`.
- **Shas** match `^[0-9a-f]{40}$` or `^[0-9a-f]{64}$`.
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
- **Timeouts.** One `context.WithTimeout` per op; `exec.CommandContext` kills git; the answer is
  504 `git_timeout`.
- **Size.** A JSON result is encoded to a buffer and git's stdout is read through the same cap;
  past `MaxJSONBytes` either one is 413 `too_large`. So is a request body past its cap.

**Errors.** A git exit the op does not expect is 422 `git_failed` with `gitSays`: the last stderr
line that starts `fatal:`; else task 02's `Sentence` rule (the last `error:` or `remote:` line,
else the last non-empty line, else `git exited <code>`, `task-02.md:178-181`); capped at 2048
bytes on a UTF-8 boundary (`gitx.MaxSentence`, `task-02.md:163`). The rule is copied, not
imported, for the same reason as the limits, and a golden fixture pins it for both modules.

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
`bundle.request.json`, `fetch.response.json`, `diff.request.json`, `error.git_failed.json`,
`error.path_outside_lane.json`). Each module's test decodes every fixture into its own type with
`DisallowUnknownFields`, re-encodes it, and compares with the fixture's compacted bytes, so a
field renamed on either side fails a test on that side. `sandboxd/internal/gitrunner` reads them
at `../../../testdata/gitrunner`.
