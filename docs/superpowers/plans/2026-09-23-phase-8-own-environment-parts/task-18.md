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
| `POST /v1/sandboxes` | `201` + `Sandbox`, **after** the pod answers `/healthz` (katabe's wait, up to `READY_TIMEOUT`) | `invalid_json` 400 · `bad_request` 400 (`backend.ValidateSpec` refused; message names the field) · `image_not_allowed` 400 · `resources_too_large` 400 · `machine_not_found` 404 · `machine_asleep` 409 · **`machine_full` 409** (`details.sessionsPerMachine`: the machine's session pods at the refusal; the new pod is already deleted — review I5) · `conflict` 409 (id exists for this owner) · `quota_exceeded` 429 (`details.limit`) · `backend_unavailable` 503 (API server, or the pod never became ready: `details.reason = "not_ready"`, and the pod is deleted before the answer) |
| `GET /v1/sandboxes` | `200` + `SandboxList`, this owner's only | `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}` | `200` + `Sandbox` | `sandbox_not_found` 404 — also for another owner's id |
| `DELETE /v1/sandboxes/{id}` | `204`, also when it did not exist (katabe's `Destroy` is idempotent, `kata.go:243-257`) | `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}/attach` | `101` WebSocket, then frames both ways | before the upgrade, as HTTP: `bad_request` 400 (no `Panacea-Attach-Token`) · `sandbox_not_found` 404 · `sandbox_not_running` 409 · `attach_rejected` 403 (sandboxd said 401) · `backend_unavailable` 503 |

**Create, in order:** decode → owner from the principal → `Image` "" or equal to the configured
digest, then set to it → resources: 0 means the default (the balloon's size, task 12), above the
max is refused → `Machine` present, and the machine **exists for this owner and is awake**
(`machineChecker`, below) → the owner's pod count < `MAX_SANDBOXES_PER_USER` → build
`backend.Spec` (`Mounts: nil`, `MachineMounts` mapped) → `katabe.New(Config{Manager: base,
Owner: owner, ReadyTimeout})` → `Create` under a context that `watchUnschedulable` (below) may
cancel → on failure, **delete the pod this request created** (below) → map errors by `errors.Is`
(never by string, as the control plane does).

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
- **Then** the broker counts the machine's other session pods (labels `panacea-app/component:
  sandbox`, `panacea-app/machine-id: <machine>`, not terminating) and answers `409 machine_full`
  with `details.sessionsPerMachine` = that count as a decimal string, and a message naming it.
- **Delete what this request created.** After a `Create` error that wraps `backend.ErrNotRunning`
  (the readiness timeout) or is this request's context error (the unschedulable cancel, or the
  caller gone), the broker calls the owner-scoped `Destroy(id)` under a fresh 10 s context, then
  answers. Never after `ErrAlreadyExists` (that pod is the owner's running session with this id)
  and never after a refusal that came before `Create`. katabe's own behaviour is unchanged.
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
// (component=sandbox, machine-id=machine) in the broker's namespace.
func machineSessions(ctx context.Context, pods corev1client.PodInterface, machine string) (int, error)
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
| `TestCreateRefusesOversizedResources` | 9000 millicpu → 400 `resources_too_large` | one user taking a whole node |
| `TestCreateRefusesAHostMountField` | a body with `"mounts": [...]` → 400 `invalid_json` (unknown field) | host paths smuggled through |
| `TestCreateNeedsAnAwakeOwnedMachine` | checker says not found → 404 `machine_not_found`; asleep → 409 `machine_asleep`; no pod either way | a session pod that can never schedule (no machine pod to be near) |
| `TestCreateEnforcesThePerUserCap` | owner at 8 pods → 429 `quota_exceeded`, `details.limit = "8"`; another owner at 0 → 201 | a cap counted across all owners |
| `TestAFullMachineIs409MachineFullInSeconds` | three running session pods on machine `m-1`; the reactor marks the new pod `PodScheduled=False/Unschedulable`, no nomination, never Running; grace 50 ms, `READY_TIMEOUT` 5 s → 409 `machine_full`, `details.sessionsPerMachine = "3"`, answered in under 1 s, and the new pod is gone (review I5) | the two-minute hang and generic 503; a Pending pod left holding the owner's cap |
| `TestABriefUnschedulableIsNotFull` | the new pod is `Unschedulable` with no nomination for 20 ms (grace 200 ms), then Running → 201, the pod kept | a watcher that fires on the first `Unschedulable` event and ignores the grace, refusing a session whose room was freeing up (a terminating pod on the node) |
| `TestPreemptionUnderWayIsNotFull` | the new pod is `Unschedulable` **with** `nominatedNodeName` set for 200 ms (grace 50 ms), then Running → 201 | a refusal every time a session preempts a balloon, which is the warm path task 12 builds |
| `TestAReadyTimeoutDeletesThePod` | the pod stays Pending with no scheduling condition; `READY_TIMEOUT` 400 ms → 503 `backend_unavailable`, `details.reason = "not_ready"`, and the pod is gone | katabe's keep-the-pod rule carried into the broker, where the pod is an orphan counting against the cap |
| `TestAConflictNeverDeletesTheExistingPod` | a running pod already has the id → 409 `conflict`, and that pod is still there | a cleanup on every error, killing the owner's running session |
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
  `TestAReadyTimeoutDeletesThePod`, `TestAFullMachineIs409MachineFullInSeconds`; widen the
  cleanup to every `Create` error → `TestAConflictNeverDeletesTheExistingPod`.
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
  `Standard_D8s_v5`); task 32's P07 measures it.
