### Task 18: Broker sandboxes — the `backend.Backend` surface over HTTP, and the attach relay

The laptop's control plane already drives sandboxes through seven methods (`backend.go:286-294`,
seam S8.1). This task serves five of them over HTTP from inside the cluster — `Create`, `Get`,
`List`, `Destroy` and `Attach` (`Kind` and `Close` are local to `remotebe`) — each scoped to the
caller through an owner-scoped `katabe.Backend` (task 14's `Config.Owner`). The laptop's
`remotebe` (task 22) is their only client, so the wire types live in `internal/brokerapi` and both
sides import them.

The relay reaches pods through **`proxy.RelayHooked`**, the one relay (seam S8.7,
`control-plane/internal/proxy/proxy.go:38`); no second relay is written. The broker never decodes
a frame: it copies them, as the control plane does (`internal/api/attach.go:212-216`).

**Files:**
- `control-plane/internal/brokerapi/wire.go` — request/response types (new)
- `control-plane/cmd/broker/sandboxes.go`, `attach.go`, `machinecheck.go`, `capacity.go` and their tests (new)
- `control-plane/cmd/broker/server.go` — the five routes join the table (modify)

#### Contract — `brokerapi/wire.go` (complete)

```go
package brokerapi

import "time"

// HeaderAttachToken carries a sandbox's attach token on GET …/attach. The
// broker passes it to sandboxd as the bearer token and never stores it. It is
// a separate header because Authorization carries the caller's Entra token.
const HeaderAttachToken = "Panacea-Attach-Token"

// MachineMount mirrors backend.MachineMount on the wire.
type MachineMount struct {
	MachinePath   string `json:"machinePath"`
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly"`
}

// SandboxSpec is POST /v1/sandboxes' body: backend.Spec, minus Mounts (host
// paths are meaningless here) and with resources flattened. Unknown fields are
// refused (httpx.DecodeJSON).
type SandboxSpec struct {
	ID            string            `json:"id"`        // backend.Spec.ID's pattern
	SessionID     string            `json:"sessionId"` // label only
	Image         string            `json:"image"`     // "" or exactly the broker's configured digest
	Command       string            `json:"command"`
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
	AttachToken   string            `json:"attachToken"` // stored in the pod's Secret (task 14), never logged
	CPUMillis     int               `json:"cpuMillis"`   // 0 = the broker's default
	MemoryMiB     int               `json:"memoryMib"`   // 0 = the broker's default
	WorkDir       string            `json:"workDir"`
	Machine       string            `json:"machine"` // required: every cloud sandbox runs on a machine
	MachineMounts []MachineMount    `json:"machineMounts"`
}

// Sandbox is what the broker reports. It deliberately has no Endpoint: the
// laptop never learns a pod IP; it attaches through the broker.
type Sandbox struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`  // always "kata"
	Ref       string            `json:"ref"`   // the pod name
	State     string            `json:"state"` // backend.State: starting | running | stopped
	Machine   string            `json:"machine"`
	CreatedAt time.Time         `json:"createdAt"`
	Labels    map[string]string `json:"labels"`
}

// SandboxList is GET /v1/sandboxes' body.
type SandboxList struct {
	Sandboxes []Sandbox `json:"sandboxes"`
}
```

#### Contract — the routes

| Route | Success | Failures (problem code, status) |
|---|---|---|
| `POST /v1/sandboxes` | `201` + `Sandbox`, **after** the pod answers `/healthz` (katabe's wait, up to `READY_TIMEOUT`) | `invalid_json` 400 · `bad_request` 400 (`backend.ValidateSpec` refused; message names the field) · `image_not_allowed` 400 · `resources_too_large` 400 · `machine_not_found` 404 · `machine_asleep` 409 · **`machine_full` 409** (only under the machine affinity; `details.sessionsPerMachine`: the machine's other session pods at the refusal, possibly `0` when other users' pods fill the node or it is cordoned; the new pod is already deleted — review I5, b05 re-review) · `conflict` 409 (id exists for this owner) · `quota_exceeded` 429 (`details.limit`) · `backend_unavailable` 503 (API server, or the pod never became ready: `details.reason = "not_ready"`, and the pod is deleted before the answer) |
| `GET /v1/sandboxes` | `200` + `SandboxList`, this owner's only | `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}` | `200` + `Sandbox` | `sandbox_not_found` 404 — also for another owner's id |
| `DELETE /v1/sandboxes/{id}` | `204`, also when it did not exist (katabe's `Destroy` is idempotent, `kata.go:243-257`) | `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}/attach` | `101` WebSocket, then frames both ways | before the upgrade, as HTTP: `bad_request` 400 (no `Panacea-Attach-Token`) · `sandbox_not_found` 404 · `sandbox_not_running` 409 · `attach_rejected` 403 (sandboxd said 401) · `backend_unavailable` 503 |

**Create, in order:** decode → owner from the principal → `Image` "" or equal to the configured
digest, then set to it → resources: 0 means the default (`Cfg`'s `SESSION_*`, the balloon's size,
task 12), and 0 is what every cloud session sends (remotebe never sends a size, task 22); above
the SKU-derived max (task 17) is `resources_too_large`, because such a session fits on no node
beside its machine pod → `Machine` present, and the machine **exists for this owner and is awake**
(`machineChecker`, below) → the owner's pod count < `MAX_SANDBOXES_PER_USER` → build
`backend.Spec` (`Mounts: nil`, `MachineMounts` mapped) → `katabe.New(Config{Manager: base,
Owner: owner, ReadyTimeout})` → `Create` under a context that `watchUnschedulable` (below) may
cancel, **armed only when `Cfg.MachineAffinity` is true** → on failure, **delete the pod this
request created** (below) → answer in the order below, mapping errors by `errors.Is` (never by
string, as the control plane does).

**A full machine is refused in seconds, and no failed create leaves a pod (review I5).** Task 15's
required affinity puts every session on its machine pod's node, beside whatever else runs there
(other users' machines and sessions, the DaemonSets). When that node is full, the session can
never schedule: a new node does not hold the machine pod, so the autoscaler cannot help, and a
balloon helps only if it happens to sit on that node. Without this, the create waited the whole
`READY_TIMEOUT` (120 s), answered a generic 503, and left the Pending pod counting against the
owner's cap: katabe deliberately keeps a pod after a readiness timeout
(`TestCreateTimesOutWithErrNotRunningAndLeavesThePod`, `kata_test.go:379`), which is right for
the laptop's retry and wrong here, where the caller has been told the create failed and nobody
will ever use the pod.
- **The scheduler decides "fits", not the broker.** The review proposed a pre-create sum
  (the machine's pods plus the RuntimeClass overhead, against the node's allocatable). That is
  wrong on two facts: the node is shared, so the machine's own pods are not all that is on it;
  and reading a node's allocatable needs a permission on `nodes`, which task 29 grants to no one
  and tests as absent (least privilege), while the DaemonSets of other namespaces are pods the
  broker cannot list either. The scheduler already counts all of it, Kata overhead
  (`pod.spec.overhead`) included. So the broker creates the pod and watches it (`pods watch`,
  already granted): `PodScheduled` `False` with reason `Unschedulable` **and** an empty
  `status.nominatedNodeName` (no preemption under way, so no balloon on that node is being
  evicted for it), held continuously for `UNSCHEDULABLE_GRACE` (5 s), cancels the create.
  A nominated node means a balloon is being preempted: keep waiting, that is the warm path.
- **The watcher is armed only under the machine affinity (b05 re-review 3).** The verdict rests on
  "a new node does not hold the machine pod", which is true only under task 15's required
  affinity. With `PANACEA_APP_BROKER_MACHINE_AFFINITY=false` (P8-D6's NFS fallback, P03's "if
  false" column), an `Unschedulable` session with no balloon left to preempt is exactly the pod
  the cluster autoscaler adds a node for, and a 5 s refusal would turn away every session past the
  warm capacity while the pool is below `max_count`. So without the affinity the create keeps
  the `READY_TIMEOUT` path, which now deletes its pod, and `machine_full` is never answered.
- **Then** the broker counts the machine's other session pods (labels `panacea-app/component:
  sandbox`, `panacea-app/machine-id: <machine>`, not terminating, and not this request's own pod,
  which it excludes by name because the count runs **before** `deleteIfOurs`), then deletes, then
  answers `409 machine_full`
  with `details.sessionsPerMachine` = that count as a decimal string, and a message naming it.
- **Delete what this request created, decided by identity, not by the error's class (b05
  re-review 1).** The error class cannot tell the cases apart. katabe wraps a client error with
  `%v` and only `backend.ErrUnavailable` with `%w` (`kata.go:105` for the pod create, `:160` for
  every poll `Get`). So a cancel that lands while one of those calls is in flight reaches the
  broker as `ErrUnavailable`, not as `context.Canceled`, whatever error the client returned for
  it. Only a cancel in the `select` at `kata.go:143-146` returns `ctx.Err()`
  itself. The same holds for a caller who leaves during the pod create at `:91-105`, which the API
  server may already have stored. And one error class holds both kinds of pod: a create of an id
  the owner already runs, refused at admission (the `ResourceQuota` is full, or the LimitRange
  refuses the size), is a 403 that katabe also wraps as `ErrUnavailable`, and there the pod named
  `sandbox-<id>` is the owner's running session. The rule:
  - After **every** `Create` error except `ErrAlreadyExists`, the broker runs
    `deleteIfOurs(ctx, pods, "sandbox-"+id, spec.AttachToken)` under a fresh 10 s context, then
    answers.
  - `deleteIfOurs` gets the pod and compares its `backend.EnvToken` env value
    (`manager.go:110` puts the attach token there) with this request's token, using
    `subtle.ConstantTimeCompare`. It deletes the pod only on a match, with
    `Preconditions{UID: pod.UID}`, so a pod that replaced it between the `Get` and the `Delete`
    is not touched.
  - Every create carries a fresh token (`ValidateSpec` refuses an empty one, `backend.go:232`),
    so only this request's pod matches. A missing pod, or a pod with another token, is a no-op.
    That covers the running session behind an admission refusal, and the `ErrUnsupported`
    refusals (`ValidateSpec` and `kata.go:81-89`), which return before any pod exists.
  - Never after `ErrAlreadyExists`, even on a token match. A laptop that re-sends a create whose
    201 it lost carries the same token, and that pod is the session it asked for.
  - The token is compared, never logged (`TestTheAttachTokenIsNeverLogged` covers the new path).
- **Which answer, in this order:** `stop()` reported `fired` → `409 machine_full` (only when the
  watcher was armed, above). Else the request context's `Err()` is set → the caller is gone, and
  the broker logs `create abandoned` and writes nothing. Else map by `errors.Is`. katabe's own
  behaviour is unchanged.
- The prediction the review asked for survives as a **number, not a gate**: P8-D6 states the
  ceiling (about three default sessions per machine on `Standard_D8s_v5`), and task 32's P07
  measures it.

**Attach, in order — the dial comes before the upgrade here**, unlike the control plane's
(`attach.go:190-199` dials after, to avoid wasting sandboxd's replay frame on a client that never
upgrades). The broker is one hop further in: if it upgraded first, a missing sandbox or a rejected
token could only be a close code, and `remotebe` would have to reverse-map close codes to the
sentinel errors the control plane's own `attachCloseStatus` expects. Dialing first lets every
failure be an HTTP status and a problem code that `remotebe` maps one-to-one. The replay frame
waits in the upstream socket until the relay starts, a few milliseconds later.
1. `Panacea-Attach-Token` present, else 400.
2. owner-scoped `katabe.Attach(ctx, id, token)` → `backend.Dial` to `podIP:8080` with the token as
   bearer, `SetReadLimit(MaxMessageBytes)` already applied (`dial.go:33-40`).
3. `websocket.Accept` the caller; **`SetReadLimit(backend.MaxMessageBytes)` immediately** — the
   fourth site of the shared rule (`backend.go:29-37`); the comment there is updated to list five.
4. a keepalive goroutine pings the caller every **30 s** (`Conn.Ping`), so neither Traefik nor the
   Azure load balancer (task 13) sees an idle flow;
5. `proxy.RelayHooked(ctx, caller, upstream, proxy.Hooks{})` until either side closes; then both
   are closed with the other side's status where known, `1001` on broker shutdown.

The caller's `Authorization` header never reaches the pod: `entra.Middleware` removed it
(task 16), and `backend.Dial` sends only the attach token.

```go
// machinecheck.go — the seam task 19 fills.
type machineChecker interface {
	// Awake returns nil if machine exists for owner and its machine pod is
	// running; an error wrapping errMachineNotFound or errMachineAsleep
	// otherwise. Task 19 implements it over the machine's claim and pod.
	Awake(ctx context.Context, owner, machine string) error
}
var (
	errMachineNotFound = errors.New("broker: no such machine for this owner")
	errMachineAsleep   = errors.New("broker: machine is asleep")
)
// serverDeps gains:  Machines machineChecker; Base *sandbox.Manager (unscoped); Cfg Config
```

```go
// capacity.go — review I5. The scheduler is the one authority on whether a
// session fits beside its machine pod; the broker only listens to it.

// watchUnschedulable calls cancel once pod name (katabe's "sandbox-<id>",
// kata_test.go:387) has reported PodScheduled=False, reason Unschedulable, with
// an empty status.nominatedNodeName, continuously for grace. A nominated node,
// a scheduled pod or a deleted pod resets or ends the watch. It returns a func
// that stops the watch and reports whether it fired.
func watchUnschedulable(ctx context.Context, pods corev1client.PodInterface, name string,
	grace time.Duration, cancel context.CancelFunc) (stop func() (fired bool))

// machineSessions counts the non-terminating session pods of machine
// (component=sandbox, machine-id=machine) in the broker's namespace, except
// the pod named except (this request's own: the count runs before its delete).
func machineSessions(ctx context.Context, pods corev1client.PodInterface, machine, except string) (int, error)

// deleteIfOurs deletes pod name only if its backend.EnvToken env value equals
// token (constant-time), with a UID precondition. A missing pod or another
// token is a no-op and returns nil (b05 re-review 1).
func deleteIfOurs(ctx context.Context, pods corev1client.PodInterface, name, token string) error
```

#### Tests to write first

`sandboxes_test.go` — the broker's handler over a fake clientset, a fake `machineChecker`, and a
test validator; a reactor marks created pods Running with an IP that points at an `httptest`
sandboxd stand-in (the pattern `katabe/kata_test.go`'s `runningSandbox` already uses):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCreateReturns201AfterTheSandboxIsReady` | 201, `state: "running"`, no `endpoint` key in the JSON | a pod IP leaked to the laptop |
| `TestCreateRefusesAnImageThatIsNotTheDigest` | `image: "evil/x:latest"` → 400 `image_not_allowed`, no pod | any image the caller names, run in our cluster |
| `TestCreateFillsTheImageWhenEmpty` | `image: ""` → the pod's image is the configured digest | an empty image reaching the API server |
| `TestCreateRefusesOversizedResources` | with the default config, 6001 millicpu → 400 `resources_too_large` and 6000 → 201; 20481 MiB → 400 and 20480 → 201; no pod for either refusal (b05 re-review 2) | a session that fits on no node beside its machine pod, which under the affinity is refused as `machine_full` with advice that cannot help |
| `TestZeroResourcesTakeTheBrokersDefault` | a body with no `resources` → the pod's container requests and limits are `2000m` / `4096Mi`, from `Cfg`, not from `backend.DefaultResources` (a test config of `1500`/`3072` shows which) (b05 re-review 4) | the laptop's size, or a compiled-in default, deciding a cloud session's size |
| `TestCreateRefusesAHostMountField` | a body with `"mounts": [...]` → 400 `invalid_json` (unknown field) | host paths smuggled through |
| `TestCreateNeedsAnAwakeOwnedMachine` | checker says not found → 404 `machine_not_found`; asleep → 409 `machine_asleep`; no pod either way | a session pod that can never schedule (no machine pod to be near) |
| `TestCreateEnforcesThePerUserCap` | owner at 8 pods → 429 `quota_exceeded`, `details.limit = "8"`; another owner at 0 → 201 | a cap counted across all owners |
| `TestAFullMachineIs409MachineFullInSeconds` | affinity on; three running session pods of this owner on machine `m-1`, and beside them, none of which may count (b05 re-review 7): a session pod of this owner on `m-2`, another owner's session pod on its machine `m-3` (machine ids are namespace-unique, task 19, so that owner cannot hold an `m-1`), the `m-1` machine pod itself (component `machine`), and a terminating session pod on `m-1`; the reactor marks the new pod `PodScheduled=False/Unschedulable`, no nomination, never Running; grace 50 ms, `READY_TIMEOUT` 5 s → 409 `machine_full`, `details.sessionsPerMachine = "3"`, answered in under 1 s, and the new pod is gone; a `delete pods` reactor records the one delete of the new pod, whose `DeleteOptions.Preconditions.UID` equals that pod's UID (review I5; b05 re-review 1) | the two-minute hang and generic 503; a Pending pod left holding the owner's cap |
| `TestABriefUnschedulableIsNotFull` | the new pod is `Unschedulable` with no nomination for 20 ms (grace 200 ms), then Running → 201, the pod kept | a watcher that fires on the first `Unschedulable` event and ignores the grace, refusing a session whose room was freeing up (a terminating pod on the node) |
| `TestPreemptionUnderWayIsNotFull` | the new pod is `Unschedulable` **with** `nominatedNodeName` set for 200 ms (grace 50 ms), then Running → 201 | a refusal every time a session preempts a balloon, which is the warm path task 12 builds |
| `TestAReadyTimeoutDeletesThePod` | the pod stays Pending with no scheduling condition; `READY_TIMEOUT` 400 ms → 503 `backend_unavailable`, `details.reason = "not_ready"`, and the pod is gone | katabe's keep-the-pod rule carried into the broker, where the pod is an orphan counting against the cap |
| `TestAConflictNeverDeletesTheExistingPod` | a running pod already has the id → 409 `conflict`, and that pod is still there; once with another token, and once with **the same token** (a laptop re-sending a create whose 201 it lost, b05 re-review 1) | a cleanup after `ErrAlreadyExists`, which the token match alone would not stop: it kills the owner's running session |
| `TestAFullMachineIs409EvenWhenKatabeHidesTheCancel` | affinity on; the new pod is `Unschedulable`, no nomination; a `get pods` reactor holds the first `Get` of the new pod (katabe's first poll, made at once after the create, `kata.go:130`, so it is in flight when the watcher fires) for 3× the grace, returns `errors.New("client rate limiter Wait returned an error: context canceled")` unwrapped, as the real client does when its context is cancelled mid-call, and lets every later `Get` through (so `deleteIfOurs`'s own `Get` works). katabe's error then wraps `ErrUnavailable`, not `context.Canceled`, which a test-only `serverDeps.onCreateError` hook records and the test asserts, so the test proves it took the path it names → 409 `machine_full` and the pod is gone (b05 re-review 1) | the old "`ErrNotRunning` or a context error" cleanup rule, which keeps the pod and answers 503 or a `machine_full` whose "the pod is deleted" is false |
| `TestAnAdmissionRefusalNeverDeletesTheRunningSession` | the owner's running pod `sandbox-x` carries token `tok-A`; a create of `x` with token `tok-B` meets a `create pods` reactor returning `403 Forbidden` (exceeded quota) → 503 `backend_unavailable`, and `sandbox-x` still runs with `tok-A` (b05 re-review 1) | a cleanup keyed on the error class ("every error except `ErrAlreadyExists` and `ErrUnsupported`"), which destroys the owner's running session behind a refused duplicate |
| `TestWithoutAffinityUnschedulableWaitsForTheTimeout` | `Cfg.MachineAffinity` false; the new pod is `Unschedulable` with no nomination for 10× the grace (grace 50 ms, `READY_TIMEOUT` 5 s), then Running → 201, the pod kept (b05 re-review 3) | a watcher armed whatever the affinity, which refuses every session in the NFS fallback that the autoscaler would have placed |
| `TestTheCallerLeavingDeletesThePod` | the client cancels its request while the pod is Pending → the pod is gone within 1 s | an orphan per abandoned create |
| `TestAnotherOwnersSandboxIs404Everywhere` | owner B: `GET`, `DELETE` (→ 204 but the pod survives), `attach` on A's id → 404 / no-op / 404 | the cross-user access the broker exists to prevent |
| `TestListReturnsOnlyTheCallersSandboxes` | A has 2, B has 1 → A lists 2 | `Manager.List` unscoped (`manager.go:240-247`) |
| `TestDeleteIsIdempotent` | two DELETEs → 204, 204 | the reaper's second pass a 500 |
| `TestErrorsAreMappedByIsNotByString` | a fake katabe error wrapping `backend.ErrUnavailable` with an unrelated message → 503 | string matching |
| `TestTheAttachTokenIsNeverLogged` | a create and an attach with token `tok-SECRET` → the captured log has no `tok-SECRET` | a token in a log line |

`attach_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAttachRelaysFramesBothWays` | a frame from the caller reaches the stand-in and back | a relay in one direction |
| `TestAttachCarriesA64KiBFrame` | a 64 KiB stdout frame arrives whole | the read limit left at 32 KiB on the broker's `Accept` (the shared rule's new site) |
| `TestAttachWithoutTheTokenHeaderIs400` | before any upgrade | an upgrade the pod will refuse |
| `TestAttachToAStoppedSandboxIs409BeforeUpgrade` | HTTP 409 `sandbox_not_running`, no 101 | a close code remotebe cannot map |
| `TestARejectedTokenIs403AttachRejected` | the stand-in answers 401 → HTTP 403 `attach_rejected` | a 500 that looks like a broker bug |
| `TestThePodNeverSeesTheEntraToken` | the stand-in records its request headers: `Authorization` is `Bearer <attach token>`, never the Entra token | the user's token forwarded into a sandbox (ADR-H1 `:545`) |
| `TestKeepalivePingsTheCaller` | with a 50 ms test interval, the caller observes pings while idle | idle sockets cut by the edge |
| `TestShutdownClosesRelaysWith1001` | cancelling the server context → the caller sees status 1001 | a silent drop remotebe cannot tell from a crash |

#### Steps

- [ ] **Step 1:** `wire.go`; the tests; they fail to compile.
- [ ] **Step 2:** `machinecheck.go` (interface + a fake for tests), `capacity.go`, `sandboxes.go`,
  `attach.go`; the routes join `server.go`'s table; update task 17's
  `TestNoRouteIsASideEffectingGet` allowlist with the two read routes.
- [ ] **Step 2a (review I5):** mutation check on a `$TEMP` copy, each verified applied and the
  named test red: drop the `nominatedNodeName` condition from `watchUnschedulable` →
  `TestPreemptionUnderWayIsNotFull`; fire on the first `Unschedulable` event, ignoring the grace →
  `TestABriefUnschedulableIsNotFull`; delete the post-failure `Destroy` →
  `TestAReadyTimeoutDeletesThePod`, `TestAFullMachineIs409MachineFullInSeconds`; run the cleanup
  after `ErrAlreadyExists` too → `TestAConflictNeverDeletesTheExistingPod` (its same-token case).
  **b05 re-review:** restore the old rule, cleanup only after `ErrNotRunning` or a context error →
  `TestAFullMachineIs409EvenWhenKatabeHidesTheCancel`; drop the token match from `deleteIfOurs`
  (delete by name) → `TestAnAdmissionRefusalNeverDeletesTheRunningSession`; arm
  `watchUnschedulable` whatever `Cfg.MachineAffinity` is →
  `TestWithoutAffinityUnschedulableWaitsForTheTimeout`; `machineSessions` without the
  `machine-id` selector, without the `component: sandbox` selector (the machine pod counts), or
  without the terminating filter, each → `TestAFullMachineIs409MachineFullInSeconds` (the count leaves `"3"`);
  the default `MAX_CPU_MILLIS` back to `8000` → `TestCreateRefusesOversizedResources`; the size
  taken from `backend.DefaultResources` instead of `Cfg` → `TestZeroResourcesTakeTheBrokersDefault`.
  The two named as likeliest to survive, and closed: `deleteIfOurs` without the UID precondition
  (the fake clientset may ignore preconditions, so only the `delete pods` reactor's recorded
  options catch it) → `TestAFullMachineIs409MachineFullInSeconds`; `machineSessions` ignoring
  `except` (a fake delete removes the pod at once, so this is caught only because the count runs
  before the delete) → the same test (the count becomes `"4"`).
- [ ] **Step 3:** Update the `MaxMessageBytes` comment in `backend.go:29-37` to name five sites.
- [ ] **Step 4:** Full test run under `-race`; commit.

#### Acceptance

```bash
go test ./control-plane/cmd/broker/ ./control-plane/internal/brokerapi/ -race -count=1   # ok
go test ./sandboxd/... ./control-plane/... -race -count=1                                 # ok
grep -n "five" control-plane/internal/backend/backend.go                                  # the updated comment
```

#### What this task does not claim

- That a pod IP is reachable from the broker's pod through Cilium with task 11's policies (task 32).
- That 30 s pings survive the chosen Traefik release's WebSocket handling (task 13's open item).
- That the scheduler writes `nominatedNodeName` no later than it reports `Unschedulable` for a
  pod that will preempt a balloon (training knowledge of kube-scheduler, **UNVERIFIED**). The 5 s
  grace covers a lag between the two; task 32's P06 (d) and P07 exercise both paths on a real
  node, and a false `machine_full` there raises the grace, not the design.
- How many sessions a machine holds. P8-D6 states the arithmetic (about three default sessions on
  `Standard_D8s_v5`); task 32's P07 measures it. Nor the largest session that fits: the
  `6000m` / `20480Mi` max is arithmetic, and P07 rewrites it.
- That client-go's fake clientset honours `DeleteOptions.Preconditions` or passes the context to a
  reactor (training knowledge, **UNVERIFIED**); no test depends on either. The UID precondition is
  pinned by the recorded delete options, and the mid-call cancel is simulated by a reactor that
  returns the real client's error text.
- The exact error the real client-go returns for a cancel mid-request (training knowledge,
  **UNVERIFIED**). Nothing depends on it: katabe's `%v` breaks the `errors.Is` chain whatever it
  is (`kata.go:105`, `:160`, verified), and the token rule deletes this request's pod whatever the
  error says.
