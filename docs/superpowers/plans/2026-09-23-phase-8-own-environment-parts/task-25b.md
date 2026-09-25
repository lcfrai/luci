### Task 25b: lane git on a cloud machine: phase 3's `Lanes.For` over the broker, the runner-only pod, and the bundle relay

HELD until phase 3's task 24 (the runner, `lanegit`) is sync-merged into feat/env, phase 3 answers S8-P3.4 (`lanegit.Bundles`) and S8-P3.5 (the pool reaches a lane's own machine), and tasks 25 and 21b land (`.superpowers/sdd/env/held.md`): the build stream skips this task until then.

Split out of task 25 by the env R4 rulings of 2026-09-25 (`.superpowers/sdd/env/r4-env-rulings.md`,
"What each task changes", task 25). Task 25 builds **project** git on a cloud machine. This task
builds the laptop's half of **lane** git, which never runs in the machine pod (ruling 2):
- **Step 1 of phase 3's pool, a live session.** The `Runner` for a lane with a live session is
  that session's pod's `sandboxd` `/git/v1/`, dialled through the broker's new sandbox git route
  (ruling 1): `POST /v1/sandboxes/{id}/git/v1/{op}`, `Authorization: Bearer <entra>` and
  `Panacea-Attach-Token: <token>`. The request names no directory.
- **Step 2, a runner-only pod.** Phase 3's pool asks this machine's `remotebe.Backend` for a pod
  whose one mount is the lane at `/home/dev/work` (ruling 2; phase-3 `task-24.md:678-684`). Its id
  is in phase 3's `HeldSet`, so the reaper keeps it (`task-24.md:661-668`, `:690-695`).
- **Bundles.** The cloud `lanegit.Bundles` (seam S8-P3.4, ruling 3) over `machined`'s bundle
  routes (task 21b), and the relay: lane work streams from the runner into `POST /v1/bundles`
  through an `io.Pipe`; a refresh bundle streams from `GET /v1/bundles/{id}` into `Runner.Fetch`.
  Nothing touches the laptop's disk.

No call here reads a lane's `.git`, and no lane git request reaches `machined` (ruling 1).

**Two places the rulings are silent, and the smallest thing consistent with R4.**
(Phase 3 citations say `phase-3`; bare `task-NN.md` is this plan's part.)
- **Phase 3's pool serves one backend.** `Lanes` holds one `Backend` and one `Dial`
  (phase-3 `task-24.md:621-636`), and phase 3's callers hold one `*lanegit.Lanes` (phase-3
  `task-07.md:70`, `task-09.md:92`, `task-11.md:160`, `task-16.md:177`). Step 2 calls
  `backend.WaitReady` on the sandbox's `Endpoint` (phase-3 `task-24.md:683`), and remotebe's
  `Endpoint` is `remote:<id>`, never dialled (`task-22.md:13-17`). Step 2 is `ErrNoRunner` "on
  katabe" (phase-3 `task-24.md:684`), and remotebe's `Kind()` is `KindKata`
  (`task-22.md:128-130`). `NewRunner` dials `"http://" + endpoint` (phase-3
  `task-24.md:574-579`). So this task asks phase 3 for **S8-P3.5** (below): the pool reaches a
  lane on the lane's own machine. This task does not start without it.
- **The agent proxy caps request bodies at 16 MiB** (`task-19.md:91-94`), and ruling 3 sends a
  bundle of up to `MaxBundleBytes` (1 GiB) through it to `POST /v1/bundles`. Task 19 owns the
  exception: on that one path the body streams through a `GitMaxBundleBytes` cap with
  `GitBundleTimeout` + 5 s as its deadline (`task-19.md:96-103`, tests `:225-227`), and `/tmp`
  has `sizeLimit: 3Gi` (`:26`, `:229-231`). This task edits neither.

**Depends on** task 25 (`machines.Remote`; `machineapi.MachineRoot` is task 20's), task 21b (the
bundle routes), task 19 (the proxy's bundle cap and `/tmp`'s `sizeLimit`), task 18 (the sandbox
git route and the runner-only admission, `task-18.md:127-133`, `:155`), task 15
(`ValidateSpec` refuses a `projects/` mount), task 23 (`machines.Set`, and the per-machine reaper
that honours `Held`), task 24b (`machines.Agent.WorktreePath`), phase 3 task 24 (`lanegit`), and
phase 3's **acceptance of S8-P3.4 and S8-P3.5**. The plan holds 25b out of the build stream until
phase 3 answers (plan, "Order", "Held"; P3A-5); Step 1 checks.

**Files:**
- `control-plane/internal/backend/remotebe/lanegit.go`, `lanegit_test.go` — the runner-only spec
  translation, `Client.Raw`, `Client.GitTransport`, `DialLane` (new)
- `control-plane/internal/backend/remotebe/remote.go` — `Create` runs the translation and
  `LaneCheck` first (modify)
- the `machines` file where task 23 builds a machine's `remotebe` backend — sets `LaneCheck` to
  `Agent.WorktreePath` (modify; the file name is task 23's, UNVERIFIED until it lands)
- `control-plane/internal/machines/bundles.go`, `bundles_test.go` — `Bundles`, the cloud
  `lanegit.Bundles` (new)
- `control-plane/internal/machineapi/bundles.go` — the bundle wire, if task 21b has not already
  added it (new; task 21b owns it, and its spelling wins)
- `control-plane/cmd/api/main.go` — `laneGit.Machines` (modify, where phase 3 builds `laneGit`,
  phase-3 `task-24.md:841-850`)
- `control-plane/cmd/api/lanegit_cloud_test.go` — the pool against a fake broker (new)

#### Contract (complete)

S8-P3.5, the shape this plan asks of phase 3's `lanegit` (phase 3's spelling wins; if it answers
with another, Step 1 adapts the wiring and leaves remotebe and `machines` alone):

```go
package lanegit

// Machine is how the pool reaches the lanes of one machine (S8-P3.5).
type Machine struct {
	Backend backend.Backend
	Dial    func(endpoint, token string) Runner // nil means the pool's Dial, then NewRunner
	Bundles Bundles                             // S8-P3.4; the local machine's is the OwnedRoot files
}

// Lanes gains one field:
//
//	// Machines answers the machine a lane is on, by lane.MachineID (phase-3 task-03.md:67).
//	// nil means every lane is on {Backend, Dial, the local Bundles}: phase 3 exactly. For
//	// calls it with lane.MachineID; Live and Drop find the lane with Store.ByID first. A pool
//	// entry remembers the Machine it created on, so the Idle timer and Drop destroy there.
//	Machines func(ctx context.Context, machineID string) (Machine, error)
//
// and one method, for the callers of FetchLane, ArchiveLane and SendBranch (tasks 07, 09, 11):
//
//	// Bundles answers the Bundles of the machine lane is on.
//	func (p *Lanes) Bundles(ctx context.Context, lane lanes.Lane) (Bundles, error)
//
// Step 2 changes in two places. Neither changes dockerbe or katabe:
//   - readiness: when the Machine's Backend implements backend.ReadyWaiter (task 22), step 2
//     calls its WaitReady(ctx, sb.ID, ReadyTimeout), not backend.WaitReady on sb.Endpoint;
//   - ErrNoRunner is decided from Create's answer, errors.Is(err, backend.ErrUnsupported),
//     never from Kind(): remotebe's Kind is KindKata (task-22.md:128-130) and it can serve step 2.

// NewRunnerTransport is NewRunner with the http.Client's Transport set to rt. nil rt is
// NewRunner. Every check NewRunner makes (phase-3 task-24.md:562-579) is unchanged.
func NewRunnerTransport(endpoint, token string, rt http.RoundTripper) Runner
```

How phase 3 decides `ErrNoRunner` today is **UNVERIFIED**: its plan says only "on katabe"
(phase-3 `task-24.md:684`). If it already decides from `Create`'s answer, that half of the ask is
answered.

remotebe's additions (`lanegit.go`):

```go
package remotebe

// RunnerWorkDir is phase 3's sessionWorkDir and sandboxd's -git-worktree default
// (phase-3 task-24.md:680-683).
const RunnerWorkDir = "/home/dev/work"

// SandboxHost is the host DialLane gives NewRunnerTransport. .invalid never resolves (RFC 2606),
// so a request that bypassed GitTransport fails closed.
const SandboxHost = "sandbox.invalid"

// laneMount maps phase 3's runner-only spec onto this backend's machine. Mounts that are exactly
// one {HostPath: <lane path>, ContainerPath: RunnerWorkDir, ReadOnly: false}, with MachineMounts
// empty, become Mounts nil and MachineMounts [{MachinePath: rel, ContainerPath: RunnerWorkDir}],
// where rel is machineapi.LaneMachinePath(HostPath) (task-20.md:132-144): the one parser, which
// takes either separator, since a Windows laptop may hold "\machine\lanes\…" (P3A-1,
// UNVERIFIED). No Mounts: the spec is returned unchanged, rel "". Any other Mounts:
// ErrUnsupported naming the field, before any request. Syntax only; it touches no disk.
func laneMount(s backend.Spec) (spec backend.Spec, rel string, err error)

// LaneCheck is the machine agent's worktree path route (task 20, task 21's lane_creating step),
// which machines sets to its Agent.WorktreePath (task-24b.md:83) where it builds this machine's
// backend (task 23). remotebe cannot import machines (machines imports remotebe), so it is a func.
type LaneCheck func(ctx context.Context, worktreeID, lane string) (machineapi.WorktreePath, error)

// Create runs laneMount; when it mapped a lane, it calls LaneCheck(ctx,
// project.WorktreeID(HostPath), rel) before anything else: the id phase 3's lane store minted
// from the same Path (task-03.md:62; env ruling 1). So ruling 1's Lstat walk and the
// lane_creating refusal run at pod create, for a runner-only pod as for a session (24b step 3).
// Its refusal is returned and no broker request is sent. A nil LaneCheck refuses every lane
// mount with ErrUnsupported (fail closed). Then backend.ValidateSpec (task 15's projects/
// refusal), then the request, as today (task-22.md:124-126).

// Raw sends body to path with the Entra bearer and returns the response whatever its status,
// for the caller to read and close. Only a TokenSource failure or a network error is an error:
// ErrUnavailable, wrapping ErrSignedOut when that was the cause, as Do maps them
// (task-22.md:159). The sandbox git route and the bundle routes use it: their non-2xx bodies
// are the runner's or machined's, which Do would re-map (ruling 1 passes them through).
func (c *Client) Raw(ctx context.Context, method, path string, hdr http.Header, body io.Reader) (*http.Response, error)

// GitTransport takes the request NewRunnerTransport built, POST http://sandbox.invalid/git/v1/<op>
// with "Authorization: Bearer <attach token>", and sends it through Raw as
//
//	POST /v1/sandboxes/<sandboxID>/git/v1/<op>
//	Panacea-Attach-Token: <attach token>   (brokerapi.HeaderAttachToken, task-18.md:34)
//	Content-Type: as received
//
// with the request's context, body and ContentLength. <op> is one segment of [a-z-]. A method
// other than POST, another host or path, or no bearer is an error before any request. The
// answer comes back as received: status, headers, body (phase 3's client decodes it).
func (c *Client) GitTransport(sandboxID string) http.RoundTripper

// DialLane is Machine.Dial for a cloud machine. An endpoint "remote:<id>" (task 22's opaque
// endpoint) answers lanegit.NewRunnerTransport(SandboxHost, token, c.GitTransport(id)). Any
// other endpoint answers a Runner whose transport refuses every op: a cloud lane never dials a
// pod IP.
func DialLane(c *Client) func(endpoint, token string) lanegit.Runner
```

The bundle wire, which task 21b serves (task 21b owns `machineapi`'s bundle file; its spelling
wins):

```go
package machineapi

const (
	PathBundles = "/v1/bundles"               // POST: body a bundle, or ?reserve=1 and no body
	BundleDir   = "/tmp/machined-git/bundles" // machined's owned root + "/bundles" (ruling 3)
)

// BundleID is POST /v1/bundles's 201. ID matches ^[0-9a-f]{16}$. GET and DELETE are
// PathBundles + "/" + ID.
type BundleID struct {
	ID string `json:"id"`
}
```

The cloud `Bundles` (`internal/machines/bundles.go`). Ruling 3 names `Put(io.Reader)`,
`Reserve()`, `Open(id)`, `Remove(id)` and `Path(id)`; the four that cross the network also take a
`ctx`, the one change made here.

```go
package machines

// Bundles is lanegit.Bundles (S8-P3.4) on one cloud machine: machined's bundle routes
// (task 21b) through the broker's agent proxy, /v1/machines/<MachineID>/agent/v1/bundles…
// (task-19.md:89), sent with remotebe's Raw. Every id it takes or answers is checked against
// ^[0-9a-f]{16}$: an answered id is machined's claim, and every id lands in a path.
type Bundles struct {
	Client    *remotebe.Client
	MachineID string // ^m-[0-9a-f]{8}$
}

var _ lanegit.Bundles = Bundles{}

// ErrBundleBusy: machined already holds two live bundles (429 bundle_busy). Nothing here waits
// or retries.
var ErrBundleBusy = errors.New("machines: the machine already holds two bundles")

// BundleError is every other problem on a bundle route: 413 too_large, 504 git_timeout, 404 for
// an id that is not live, 409 machine_asleep from the proxy. Error() is "<status> <code>".
type BundleError struct {
	Status int
	Code   string
}

func (e *BundleError) Error() string

// Put streams r as the body of POST …/v1/bundles, chunked (ContentLength -1), never read ahead,
// and answers the 201's id. On any failure it closes r when r is an io.Closer, so a relay's
// io.PipeWriter (the runner's Bundle) unblocks with io.ErrClosedPipe instead of leaking.
func (b Bundles) Put(ctx context.Context, r io.Reader) (string, error)

// Reserve is POST …/v1/bundles?reserve=1 with no body: an id and no file, for SendBranch's
// bundle create into Path(id) through POST /v1/git (ruling 3, project to lane).
func (b Bundles) Reserve(ctx context.Context) (string, error)

// Open is GET …/v1/bundles/{id}, streamed, for the caller to close. It is Runner.Fetch's body,
// unread here (phase-3 task-24.md:570).
func (b Bundles) Open(ctx context.Context, id string) (io.ReadCloser, error)

// Remove is DELETE …/v1/bundles/{id}: nil on 204, which machined also answers for an id already
// gone (ruling 3).
func (b Bundles) Remove(ctx context.Context, id string) error

// Path is machineapi.BundleDir + "/" + id + ".bundle": a path on the MACHINE, the one argument
// machined's POST /v1/git admits under /tmp/machined-git/ while the id is live (ruling 3). A
// malformed id answers "". The laptop never opens it.
func (b Bundles) Path(id string) string
```

The relay, as phase 3's `FetchLane` spools after S8-P3.4 (ruling 3, lane to project). It is phase
3's code; this is what `Bundles` must make work:

```go
pr, pw := io.Pipe()
go func() { _, err := runner.Bundle(ctx, req, pw); pw.CloseWithError(err) }()
id, err := bundles.Put(ctx, pr) // ctx: BundleTimeout + 5 s, one deadline over both legs
if err != nil { return err }
defer bundles.Remove(context.WithoutCancel(ctx), id) // every path, success included
// bundle verify, bundle list-heads and fetch on bundles.Path(id): Scope Project, POST /v1/git
```

The wiring in `cmd/api/main.go`, beside phase 3's `laneGit` (phase-3 `task-24.md:850`):

```go
laneGit.Machines = func(ctx context.Context, id string) (lanegit.Machine, error) {
	if id == "" || id == machines.LocalID {
		return lanegit.Machine{Backend: be, Bundles: localBundles}, nil // phase 3's own (S8-P3.4)
	}
	b, err := set.Backend(ctx, id) // task 23: ErrUnknownMachine for an id that is not this user's
	if err != nil {
		return lanegit.Machine{}, err
	}
	c := set.Cloud() // non-nil: Backend answers a cloud id only when a broker is configured
	return lanegit.Machine{Backend: b, Dial: remotebe.DialLane(c),
		Bundles: machines.Bundles{Client: c, MachineID: id}}, nil
}
```

The one `HeldSet` (phase-3 `task-24.md:845`) holds runner-only ids on every machine: ids are
`lg-` + 16 random hex, so they do not collide across machines.

#### Asks of phase 3, recorded here (task 33 lists the phase's asks)

| Ask | What | Why | Plan proceeds on |
|---|---|---|---|
| **S8-P3.4** `lanegit.Bundles` (ruling 3) | `FetchLane`, `ArchiveLane` and `SendBranch` take `gitx.Runner` and a `Bundles` (`Put`, `Reserve`, `Open`, `Remove`, `Path`), four of them with `ctx` | today they take `*gitx.Exec` and write `<OwnedRoot>/bundles` on the laptop (phase-3 `task-24.md:704-745`), the wrong disk for a cloud lane | nothing: without it this task does not start |
| **S8-P3.5** the pool reaches a lane's own machine (new; the rulings are silent) | `Lanes.Machines`, `Lanes.Bundles`, step 2's `ReadyWaiter` and `ErrUnsupported` rules, `NewRunnerTransport` (above) | one `Backend` and one `Dial` per pool, a readiness poll of an undialled endpoint, a Kind-based `ErrNoRunner`, and a runner that dials `http://<endpoint>` | nothing: without it this task does not start |

#### Tests to write first

`remotebe/lanegit_test.go`, an `httptest.NewTLSServer` playing the broker that records every
request, and phase 3's `lanegit.NewRunnerTransport`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLaneMountMapsOneLane` | `{HostPath: "/machine/lanes/p-1/app-0a1b2c", ContainerPath: "/home/dev/work"}` → `MachineMounts == [{MachinePath: "lanes/p-1/app-0a1b2c", ContainerPath: "/home/dev/work"}]`, `Mounts == nil`; the same lane as `\machine\lanes\p-1\app-0a1b2c` → the same `MachineMounts`; no `Mounts` → the spec unchanged | a runner-only pod refused, a Windows laptop's lane path refused (a parser of laneMount's own), or a session spec rewritten |
| `TestRunnerOnlyCreateChecksTheLaneFirst` | a fake `LaneCheck` records its calls: it is called once, with `project.WorktreeID(HostPath)` and `lanes/p-1/app-0a1b2c`, before any broker request; answering `409 lane_path_invalid` (a symlinked `lanes/p-1`) or `409 lane_creating` → that error and the broker got no request; a nil `LaneCheck` → `ErrUnsupported`, no request | a runner-only pod on a symlinked or still-creating lane, because only a session create checked it (ruling 1) |
| `TestLaneMountRefusesEverythingElse` | `/machine/projects/app`, `/machine/lanes/p`, `/machine/lanes/p/d/x`, `/machine/lanes/../projects/app`, `/machine/lanes/p/..`, `C:\x`, `/machine`, ContainerPath `/other`, `ReadOnly: true`, two mounts, `Mounts` with `MachineMounts` → `ErrUnsupported`, and the broker got no request | a runner-only pod that mounts the project, another lane or the disk root (R4.1) |
| `TestCreateSendsTheRunnerOnlySpec` | phase 3's step 2 spec (`lg-…`, `sleep infinity`, no `SessionID`) → the broker's create body has `machine: "m-0123abcd"`, the one lane mount at `/home/dev/work`, and no other mount | a conversation store or a `projects/` mount on a runner-only pod |
| `TestGitTransportUsesTheSandboxRoute` | a runner `Status` → `POST /v1/sandboxes/lg-00…01/git/v1/status`, `Authorization: Bearer <entra>`, `Panacea-Attach-Token: <token>`, no query, the body phase 3 sent | the attach token sent as the broker bearer (broker 401), or a request to the opaque endpoint |
| `TestGitTransportPassesTheAnswerThrough` | the broker answers 409 with a runner `ErrorResponse` `bundle_empty` → `errors.Is(err, lanegit.ErrBundleEmpty)`; a 200 body arrives byte for byte | a re-mapped runner answer (ruling 1) |
| `TestGitTransportRefusesOtherRequests` | `GET`, another host, a path outside `/git/v1/<op>`, an `op` with `/` or `..`, no bearer → an error, no request | the transport as an open proxy to any broker route |
| `TestDialLaneRefusesAPodEndpoint` | `DialLane(c)("10.0.0.5:8080", t)` → every op fails, no request anywhere | a cloud lane dialled at a pod IP |
| `TestSignedOutIsNotAGitAnswer` | a failing `TokenSource` → the op's error wraps `ErrSignedOut`; not a runner `Code` | "git said" shown when the user is signed out |

`machines/bundles_test.go`, the same fake broker playing the agent proxy:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestPutStreams` | a reader that yields 1 MiB and then blocks until the server has read that 1 MiB → `Put` completes; the request had no `Content-Length` | a bundle buffered in memory before sending |
| `TestPutFailureUnblocksTheWriter` | server answers 413 `too_large` mid-body → `*BundleError{413, "too_large"}`; the `io.PipeWriter` goroutine returns `io.ErrClosedPipe` | a leaked relay goroutine holding a runner stream |
| `TestBusyIsErrBundleBusy` | 429 `bundle_busy` → `errors.Is(err, ErrBundleBusy)` | a third bundle queued on the machine |
| `TestReserveOpenRemovePaths` | `Reserve` → `POST …/agent/v1/bundles?reserve=1`, empty body; `Open` → `GET …/{id}`; `Remove` → `DELETE …/{id}`, nil on 204 | a route the proxy does not forward |
| `TestIdsAreChecked` | an answered id `../x` or 15 hex → error; `Open`, `Remove` with a bad id → error, no request; `Path("bad") == ""`; `Path("0123456789abcdef") == "/tmp/machined-git/bundles/0123456789abcdef.bundle"` | a machine-supplied id used as a path |

`cmd/api/lanegit_cloud_test.go`, phase 3's real pool with `Machines` set, a fake broker, phase 3's
in-memory lane store and registry, and task 23's reaper:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCloudLaneWithASessionUsesItsPod` | a live session on lane `w-0a1b2c3d` (`SandboxID`, `AttachToken`) → `For` creates nothing; `Runner.Refs` reaches that sandbox's git route | a second pod on a lane that has one |
| `TestCloudLaneWithoutASessionGetsARunnerOnlyPod` | no session → one `Create` with the runner-only spec, `WaitReady` through `ReadyWaiter` (the broker's `GET`), never a `/healthz` dial; the id is in the `HeldSet`; after `Idle`, `DELETE` and the id leaves | `ErrNoRunner` on a cloud lane, or a health poll of `remote:<id>` |
| `TestTheReaperKeepsAHeldCloudRunner` | task 23's sweep over `m-0123abcd` lists the held `lg-…` pod with no session record past `createGrace` → not destroyed; once released and not held → destroyed | a runner-only pod reaped mid-fetch |
| `TestEachLaneUsesItsOwnMachine` | lanes on `m-1…` and `m-2…` → each `Create` goes to its own machine; `Bundles(lane)` answers each machine's | lane work sent to another of the user's machines |
| `TestLocalLanesAreUnchanged` | a lane with `MachineID "local"` → phase 3's backend, `NewRunner` and local bundles; phase 3's own pool tests pass | phase 3's local path bent by the cloud arm |
| `TestNoLaneGitReachesMachined` | across `For`, `Status`, `Refs`, `Bundle` (relayed into `Put`) and `Fetch` (from `Open`), the fake broker saw no `…/agent/…` path except `…/agent/v1/bundles…`, and no `…/agent/v1/git` whose body `dir` is under `lanes/` | lane git run in the machine pod (ruling 2) |

#### Steps

- [ ] **Step 1:** Confirm the dependencies: `git grep -n "type Bundles interface" -- control-plane/internal/lanegit/` (S8-P3.4), `git grep -n -e "Machines func" -e "func NewRunnerTransport" -- control-plane/internal/lanegit/` (S8-P3.5), `git grep -n "type Remote struct" -- control-plane/internal/machines/` (task 25), `git grep -n "func LaneMachinePath" -- control-plane/internal/machineapi/` (task 20), `git grep -n "/git/v1/" -- control-plane/cmd/ control-plane/internal/brokerapi/` (task 18's route), `git grep -n "PathBundles" -- control-plane/internal/machineapi/` (task 21b) and `git grep -n "TestAgentProxyStreamsABundleUpToItsCap" -- control-plane/` (task 19's bundle cap). If S8-P3.4 or S8-P3.5 is absent, stop and report which one is unanswered. If phase 3 answered with another spelling, adapt the wiring only.
- [ ] **Step 2:** `lanegit_test.go`, red; `laneMount`, `LaneCheck`, `Raw`, `GitTransport`, `DialLane`; `Create` calls `laneMount`, then `LaneCheck`; `machines` sets `LaneCheck` to its `Agent.WorktreePath`. Green.
- [ ] **Step 3:** `bundles_test.go`, red; `machines.Bundles` and the `machineapi` wire if task 21b lacks it. Green.
- [ ] **Step 4:** `lanegit_cloud_test.go`, red; `laneGit.Machines` in `main.go`. Green. If `TestTheReaperKeepsAHeldCloudRunner` is red, task 23's per-machine sweep skips `Held`: report it, and do not patch the reaper here.
- [ ] **Step 5: Mutation check** (rule 7), each on a `$TEMP` copy, each verified applied and
  turning the named test red:
  (1) `laneMount` accepts any `HostPath` under `/machine/` → `TestLaneMountRefusesEverythingElse`;
  (2) `Create` skips `LaneCheck` → `TestRunnerOnlyCreateChecksTheLaneFirst`;
  (3) a nil `LaneCheck` read as "no check" → the same test, on its nil row;
  (4) `GitTransport` sends the attach token as the broker bearer → `TestGitTransportUsesTheSandboxRoute`;
  (5) `Machines` answers a cloud lane's machine with `NewRunner` over `machined`'s proxy → `TestNoLaneGitReachesMachined`;
  (6) `Put` reads the reader into a buffer first → `TestPutStreams`.
  The two named beforehand as likeliest to survive are (3), because every other row sets a
  `LaneCheck`, and (5), because a `…/agent/v1/git` call with a `lanes/` `dir` is the only thing
  that shows it. Each is killed only by the row named beside it.
- [ ] **Step 6:** Full run, including phase 3's suites; commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/backend/remotebe/ ./control-plane/internal/machines/ ./control-plane/cmd/api/ -race -count=1   # ok
go test ./sandboxd/... ./control-plane/... -race -count=1                                                                      # ok
git grep -n "git/v1" -- control-plane/internal/machines/          # no output: machines never speaks the lane runner route
git grep -n "HostPath" -- control-plane/internal/backend/remotebe/ # only lanegit.go and its test: laneMount is the one reader
```

#### What this task does not claim

- Ruling 2's Kata probes: that a subPath confines the runner inside the Kata guest, kubelet's
  subPath symlink safety under Kata, that a runner-only pod fits beside the machine pod, and its
  cold start. All UNVERIFIED; task 32 probes them.
- That phase 3 accepts S8-P3.4 or S8-P3.5. The shapes above are this plan's asks (UNVERIFIED
  until phase 3 answers).
- The runner-only pod's size: task 18's runner-only admission sets it (ruling 2, "the smallest
  resources the broker admits"); remotebe sends none, as for sessions (`task-17.md:40`).
- A cockpit sentence for a broker failure before the proxy (`sandbox_not_found`,
  `sandbox_not_running`): phase 3's client reads a non-runner body as "the runner did not
  answer" (phase-3 `task-24.md:581-583`), so the sentence is phase 3's generic one.
- A broker-side splice of lane bundles: ruling 3 defers it. The bytes cross the laptop twice.
- That any of this has run against a cloud machine: the owner's batched hands-on session (OD-12),
  listed in task 32.
