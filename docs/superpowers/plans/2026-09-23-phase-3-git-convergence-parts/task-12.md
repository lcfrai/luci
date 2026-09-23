### Task 12: Fetch, on demand and on a slow timer, and push — with the machine's own credential helper

Obligation 10 (survey §2): get a cockpit branch onto a remote a pipeline can build from. Phase 7b's
Ship waits on it (`dependency-graph-and-order.md:275`). Push and fetch run **on the machine, in the
control plane, with the user's own credentials** — never in a sandbox, which holds none (A4). In
this task "the user's own credentials" means whatever the machine's git credential helper already
has (Git Credential Manager on the owner's laptop), asked **non-interactively**: phase 2's
`GCM_INTERACTIVE=never` and `GIT_TERMINAL_PROMPT=0` are in every `gitx` call, so a push with nothing
cached fails with git's own sentence rather than a hidden window (the prototype's `m-push` state).
Task 13 adds a token from the Hub's store in front of it; this task is local-only and needs no Hub.

**Never forced** (A3): no argv this task builds contains `-f`, `--force`, `--force-with-lease`, or
a `+` refspec, and a test asserts the exact argv. A rejected push (the remote moved) is a refusal
with git's sentence, and the way out is a fetch and a land, not an overwrite.

**Fetch is not free and never runs on the 2 s poll** (survey §7.5): it runs when the human asks
and on a slow timer, owned by the control plane. Its time is `FETCH_HEAD`'s mtime (task 08).

**Files:**
- `control-plane/internal/remote/fetch.go` + test — `Auth`, `Fetcher` (new)
- `control-plane/internal/remote/push.go` + test — `Push`, `PushResult` (new)
- `control-plane/internal/remote/state.go` — `ReadState` fills `FetchError` from the Fetcher (task 08's file)
- `control-plane/internal/api/remote.go` + test — POST fetch, POST push (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go` — `// ── task 12 ──`
- `control-plane/cmd/api/config.go`, `main.go` — `PANACEA_APP_FETCH_INTERVAL`; start/stop the Fetcher

#### Interfaces — complete

```go
package remote

// Auth is how one remote call authenticates. The zero value means "the machine's own git
// credential helper, non-interactively". Task 13 fills Env with an http.extraHeader built
// from the user's stored token; Env values are secrets (gitx.Call's rule: never logged).
type Auth struct {
	Env map[string]string
}

// Fetcher fetches origin. One per process.
type Fetcher struct {
	Machine gitx.Machine
	// Projects lists the project paths the timer fetches: projects with at least one cockpit
	// worktree. A project the user never converges in is never fetched behind their back.
	Projects func(ctx context.Context) ([]string, error)
	// AuthFor is task 13's hook; nil means the zero Auth for every project.
	AuthFor  func(ctx context.Context, projectPath string) Auth
	Interval time.Duration // PANACEA_APP_FETCH_INTERVAL; default 10m; 0 disables the timer; minimum 1m otherwise
	Log      *slog.Logger
	// unexported: per-project mutex, last error per project
}

const (
	DefaultFetchInterval = 10 * time.Minute
	MinFetchInterval     = 1 * time.Minute
)

// FetchResult is a fetch that succeeded.
type FetchResult struct {
	Remote    string `json:"remote"`    // "origin"
	FetchedAt string `json:"fetchedAt"` // FETCH_HEAD's mtime after the fetch
}

// ErrNoRemote is a project with no origin.
var ErrNoRemote = errors.New("remote: the project has no origin")

// Fetch runs `fetch --prune origin` (gitx.Remote) in the project. It records the failure's
// sentence as the project's last error (ReadState's FetchError) and clears it on success.
// Hardened per task-02 §HGH: Scope Project, and RemoteURL is `config --get remote.origin.url`,
// read just before the call (H2).
func (f *Fetcher) Fetch(ctx context.Context, projectPath string) (FetchResult, error)

// LastError is the last failed fetch's sentence for a project, or "".
func (f *Fetcher) LastError(projectPath string) string

// Run fetches every listed project each Interval until ctx is done; a no-op when Interval is 0.
func (f *Fetcher) Run(ctx context.Context)
```

```go
package remote

// PushResult is a push that the remote accepted.
type PushResult struct {
	Branch      string `json:"branch"`
	Remote      string `json:"remote"`
	Upstream    string `json:"upstream"`    // after the push, e.g. "origin/evidence-upload"
	SetUpstream bool   `json:"setUpstream"` // this push created or moved the upstream
	Pushed      string `json:"pushed"`      // the sha the remote now has
	UpToDate    bool   `json:"upToDate"`    // nothing was sent
}

// ErrPushRejected is a push the remote refused because it has commits the branch lacks.
// The *gitx.Error is wrapped, so the handler can put its Sentence in details.gitSays.
var ErrPushRejected = errors.New("remote: the remote has commits this branch does not")

// Push publishes refs/heads/<branch>. The remote is branch.<branch>.remote when set, else
// origin (ErrNoRemote when neither exists). The argv is exactly one of:
//   push --porcelain <remote> refs/heads/<branch>:refs/heads/<branch>
//   push --porcelain --set-upstream <remote> refs/heads/<branch>:refs/heads/<branch>
// the second when the branch has no upstream, or its upstream's merge ref is not
// refs/heads/<branch> (a branch the cockpit renamed; task 11's RemoteKeepsOldName). Nothing
// else is ever passed. --porcelain's "!" line is ErrPushRejected; "=" is UpToDate.
// Those are the Args Push gives Exec. Exec adds task-02 §HGH's prefix and --receive-pack itself.
// The call uses Scope Project, and RemoteURL is `config --get remote.<remote>.url` (H2). No
// hook runs, including the user's pre-push (H4).
func Push(ctx context.Context, m gitx.Machine, projectPath, branch string, auth Auth) (PushResult, error)
```

```go
// gitdeps.go
	// ── task 12 ──
	Fetcher *remote.Fetcher
	// PushAuth is task 13's hook; nil means the machine's credential helper.
	PushAuth func(ctx context.Context, projectPath string) remote.Auth
```

**Wire:**

```
POST /v1/projects/{projectId}/fetch
  request  {}  (an empty object; httpx.DecodeJSON refuses unknown fields)
  200 { "remote": "origin", "fetchedAt": "2026-09-23T05:41:10Z" }
  409 conflict reason no_remote
  502 git_failed  details.gitSays — e.g. "fatal: could not read Username for 'https://github.com': terminal prompts disabled"

POST /v1/projects/{projectId}/branches/{branchId}/push
  request  {}
  200 PushResult
  409 conflict reason no_remote | push_rejected (details.gitSays)
  502 git_failed details.gitSays
```

Both serialise per project with each other and with lands on that project's branches (one
`sync.Mutex` per project path, shared through `GitDeps`), because a push reading a branch a land
is moving would push whichever tip it happened to see.

#### The tests to write first

Real git against a **bare repository on disk** as the remote, and, for the credential cases, a
`net/http/httptest` server that answers git's smart-HTTP discovery with 401.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestPushArgvIsExactlyOneOfTwo` | Script runner: first push → `push --porcelain --set-upstream origin refs/heads/b:refs/heads/b`; second → the same without `--set-upstream`; no argv element matches `^-f$|^--force|^\+` | any force, ever (A3) |
| `TestPushSetsTheUpstreamOnFirstPush` | bare remote; after push, `rev-parse --abbrev-ref b@{upstream}` is `origin/b`, `SetUpstream: true` | Ship's "never pushed" state never clearing |
| `TestPushUpToDate` | pushing twice → the second `UpToDate: true` | a no-op reported as a push |
| `TestPushRejectedIsARefusal` | the bare remote has a commit the branch lacks → `ErrPushRejected` wrapping git's `! … [rejected]` line; the remote unchanged | a rejection shown as a crash, or retried with force |
| `TestPushAfterARenameMovesTheUpstream` | upstream `refs/heads/old`, local name `new` → `--set-upstream` push of `new` | pushing a renamed branch over its old name |
| `TestPushWithNoCredentialSaysSo` | remote is the 401 httptest URL (every remote in these tests is a `gitxtest.HTTPSRemote`, because a Remote call allows https only, task-02 §HGH H1), isolated config with no helper → `*gitx.Error` whose Sentence contains `terminal prompts disabled` (task 01 records the exact words); returns within `RemoteTimeout` | a push that hangs on a prompt |
| `TestPushWithNoOrigin` | no remote → `ErrNoRemote` | a 500 for a local-only project |
| `TestPushNeverRunsARepoCredentialHelper` | task-02 §HGH H1/H2 and review C3, red first: the project's config names `credential.helper=<script writing its stdin to a marker>` beside a machine helper (`Hardening.Credential`) that answers; the remote is the 401 httptest URL → H3 admits `credential.helper` (it is inert, task-02 §HGH H3), the marker does not exist after the push, and the machine helper's username reached the server. Red first with H1's `credential.helper=` reset dropped: the marker appears | a push that lets a repository helper see the user's token |
| `TestPushAndFetchRefuseARepoProxy` | review C3 and b01 re-review R3, red first. The remote `U` is a `gitxtest.HTTPSRemote`; `P` is a second listener that counts connections. Two project configs: (a) `http.<U's scheme://host:port>/.proxy=<P>`, keyed at `U`'s exact scheme, host and port; (b) `remote.origin.proxy=<P>`. **Precondition**, raw git with no Exec: for (a), `config --get-urlmatch http.proxy U` → P, so the key matches. Through Push and Fetch: (a) and (b) are each refused `project_config_untrusted`, naming `http.<…>.proxy` and `remote.origin.proxy`, and `P` saw no connection. With the H3 check mutated away, (a) still sends nothing to `P` (H2), and (b) reaches `P` (so H3 is the only layer for (b), as task-02 §HGH H2 states) | a generic-only H2 override; a Project list that admits a per-remote proxy |
| `TestPushRunsNoPrePushHook` | task-02 §HGH H4: the project's `.git/hooks/pre-push` writes a marker → a push succeeds and no marker exists | a cockpit push that runs the repository's hooks |
| `TestFetchWritesFetchHeadAndPrunes` | a branch deleted on the remote disappears from `refs/remotes/origin/*`; `FetchedAt` is FETCH_HEAD's new mtime | stale remote branches forever |
| `TestFetchRecordsAndClearsTheLastError` | a failing fetch → `LastError` is git's sentence; a later success → `""`; `ReadState` reports the same through `FetchError` | a stale "fetched 15:36" with no hint it is failing |
| `TestFetchTimerOnlyTouchesListedProjects` | `Projects` returns one of two repos → only that one's FETCH_HEAD changes over two intervals (shortened) | fetching every repository under the root |
| `TestFetchTimerOffAtZero` | `Interval: 0` → `Run` returns immediately, no git call | a timer the user turned off still running |
| `TestFetchRouteNoRemote` | → 409 `no_remote` | a git error for a normal state |
| `TestPushRouteMapsRejection` | → 409 `push_rejected` with `details.gitSays` | a rejection surfacing as `git_failed` |
| `TestPushAndLandSerialise` | a push and a land on one project concurrently under `-race` → the push publishes either the old or the new tip, never a torn state, and the result names which | pushing a tip mid-land |
| `TestLoadConfigFetchInterval` (cmd/api) | default 10m; `0` accepted (off); `30s` refused naming the 1m minimum | a fetch loop hammering a corporate proxy |

#### Steps

- [ ] **Step 1: Preflight.** Task 02 merged; task 01's sandbox test recorded that fetch fails
  *inside* a sandbox (the design), which this task never attempts.
- [ ] **Step 2:** tests; `fetch.go`, `push.go`; the `FetchError` fill in `ReadState`.
- [ ] **Step 3:** handlers, routes, deps, config row, `main.go` start/stop; the Go contract's §4
  table gains `PANACEA_APP_FETCH_INTERVAL` (default `10m`, `0` off, minimum `1m`).
- [ ] **Step 4:** run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/remote/ ./control-plane/internal/api/ ./control-plane/cmd/api/ -race -count=1 → PASS
```

Manual, on the owner's laptop, recorded or marked not run: a push of a throwaway branch to
`github.com/lcfrai/panaceaapp` through GCM; a push with GCM's cache cleared, whose 502 carries git's
sentence verbatim.

#### What this task does not claim

- **Whether GCM holds a credential for a given host is the machine's business**; this task reports
  git's sentence when it does not. Task 13's store is what makes that a sign-in the cockpit owns.
- A push is not a PR (task 14) and not a deploy (phase 7).
- Control-plane authentication does not exist (open question Q5): another local process could
  call this route and push **as the user**. It could not push as anyone else.
