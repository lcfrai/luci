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
   `-C <GitWorktree> --literal-pathspecs`, except `check-ref-format`, which has no `-C` at all.
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
   cut to 2048 bytes on a rune boundary.

#### The host side: `control-plane/internal/lanegit` (edit unit U2)

The host half: a client for the runner, a pool that finds or starts a runner for a lane, and the
two bundle moves. Host git in this half runs only in the **project** or under
`Hardening.OwnedRoot`, never in a lane (R4.2); every call goes through task 02's `gitx.Exec`.

**Files (host side):**
- `control-plane/internal/lanegit/wire.go` — the shapes of `wire.go` above, field for field, and
  the four limits of `limits.go` above, checked against `testdata/gitrunner` (new)
- `control-plane/internal/lanegit/client.go` — `Runner`, `NewRunner`, `RunnerError` (new)
- `control-plane/internal/lanegit/pool.go` — `Lanes`, `For`, the runner-only sandbox (new)
- `control-plane/internal/lanegit/bundle.go` — `FetchLane`, `SendBranch`, `BundleRefused` (new)
- `control-plane/internal/lanegit/lanegittest/fake.go` — `Fake`, a test-only `Runner` (new)
- `control-plane/internal/lanegit/*_test.go` — the tests below (new)
- `control-plane/internal/backend/backend.go` (p2) — `GitPath = "/git/v1/"` in the const block
  at `backend.go:40-45`, beside `AttachPath` (`:42`) and `HealthPath` (`:43`)
- `control-plane/internal/reaper/reaper.go` (p2) — `Config` (`reaper.go:120`) gains `Held`; see
  the pool
- `control-plane/cmd/api/main.go` (p2) — one `// ── task 24 ──` block; see Wiring
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
	Fetch(ctx context.Context, bundle io.Reader, refspecs, deletes []string) (FetchResponse, error)
	Diff(ctx context.Context, req DiffRequest) ([]byte, error)
}

// NewRunner is the HTTP client for one sandbox. endpoint is backend.Sandbox.Endpoint, host:port,
// the value backend.Dial takes (dial.go:38); token is the sandbox's AttachToken. Each op is
// POST "http://" + endpoint + backend.GitPath + op, with "Authorization: Bearer " + token.
// The http.Client follows no redirect (CheckRedirect returns http.ErrUseLastResponse) and its
// context deadline is the op's runner timeout plus 5 s.
func NewRunner(endpoint, token string) Runner

// RunnerError is every failed op. Code is the runner's ErrorResponse.Code; GitSays is capped at
// gitx.MaxSentence. A runner that does not answer (refused, reset, timed out, a body that is not
// an ErrorResponse) has Status 0, Code "" and GitSays one sentence naming what failed.
type RunnerError struct {
	Status  int
	Code    string
	GitSays string
}

// ErrBundleEmpty is what errors.Is finds on a RunnerError with Code "bundle_empty" (409).
var ErrBundleEmpty = errors.New("lanegit: the lane has nothing the basis lacks")
```

**How a caller answers a `RunnerError`.** `git_failed` 500, `details.gitSays` = `GitSays`, through
task 04's `gitFailed` (`task-04.md:143-146`), which gains that one case. So decision 8 still has
two codes. `ErrBundleEmpty` is not an error to the user: task 09 reads it as nothing to land.

```go
// pool.go — a Runner for a lane: the live session's sandbox when one is on the lane, else a
// runner-only sandbox this pool starts and destroys.
type Lanes struct {
	Backend      backend.Backend
	Sessions     registry.Store // List, registry.go:51
	Image        string         // cfg.SandboxImage, as api.Deps.Image
	Resources    backend.Resources
	ReadyTimeout time.Duration  // cfg.ReadyTimeout, for backend.WaitReady (dial.go:75)
	Idle         time.Duration  // 60 s; a field so tests can shorten it
	Log          *slog.Logger
}

// ErrNoRunner: no live session is on the lane and this backend cannot start a runner-only
// sandbox (katabe refuses Mounts, backend.go:129-131). Phase 8's machined is that half (P8-D7).
var ErrNoRunner = errors.New("lanegit: no git runner can reach this lane")

// For returns a Runner for lane and a release the caller must call exactly once.
func (p *Lanes) For(ctx context.Context, lane lanes.Lane) (Runner, func(), error)

// Holds reports whether id is a runner-only sandbox this pool holds. reaper.Config.Held is
// this method, so the reaper's rule 4 does not destroy it as an orphan.
func (p *Lanes) Holds(sandboxID string) bool
```

1. **A live session.** A registry `Session` whose `WorktreeID` (`session.go:48`) is `lane.ID`
   and whose `SandboxID`, `Endpoint` and `AttachToken` (`session.go:71-74`) are set, confirmed
   by `Backend.Get(ctx, s.SandboxID)` (`backend.go:291`) answering a non-empty `Endpoint`. For
   answers `NewRunner(sb.Endpoint, s.AttachToken)` and a no-op release. The token is read from
   the registry and never logged, as phase 2 does for `/attach`.
2. **Otherwise, a runner-only sandbox.** `Backend.Create` (`backend.go:290`) with
   `Spec{ID: "lg-" + 16 random hex, Image, Command: "sleep", Args: ["infinity"], AttachToken:
   32 random bytes (crypto/rand) as hex, Resources, WorkDir: "/home/dev/work", Mounts:
   [{HostPath: lane.Path, ContainerPath: "/home/dev/work"}]}` (`Spec` at `backend.go:141-152`;
   `Mounts` is docker only, `:151`). `/home/dev/work` is `sessionWorkDir` and the runner's
   `-git-worktree` default. Then `backend.WaitReady(ctx, sb.Endpoint, ReadyTimeout)`. The token
   lives only in the pool's memory. On katabe this step is `ErrNoRunner`.
3. **One per lane, reused, then destroyed.** The pool keeps one entry per lane with a count of
   holders; two concurrent `For` calls start one sandbox. The last release starts a timer of
   `Idle` (60 s); a `For` inside it reuses the sandbox, and when it fires the pool calls
   `Backend.Destroy`. When a session is live on a lane that also has an idle runner-only
   sandbox, `For` uses the session and destroys the idle one.
4. **The reaper.** The reaper's rule 4 (`reaper.go:363-409`) destroys a managed sandbox with no
   session record once `createGrace` has passed. A runner-only sandbox has no record, so
   `reaper.Config` gains `Held func(sandboxID string) bool`, and rule 4 skips an id for which
   it answers true. After a restart nothing holds the old ones, and rule 4 reaps them, which is
   the crash path it already is.
