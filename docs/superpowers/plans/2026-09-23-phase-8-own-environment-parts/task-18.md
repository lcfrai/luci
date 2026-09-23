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
- `control-plane/cmd/broker/sandboxes.go`, `attach.go`, `machinecheck.go` and their tests (new)
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
| `POST /v1/sandboxes` | `201` + `Sandbox`, **after** the pod answers `/healthz` (katabe's wait, up to `READY_TIMEOUT`) | `invalid_json` 400 · `bad_request` 400 (`backend.ValidateSpec` refused; message names the field) · `image_not_allowed` 400 · `resources_too_large` 400 · `machine_not_found` 404 · `machine_asleep` 409 · `conflict` 409 (id exists for this owner) · `quota_exceeded` 429 (`details.limit`) · `backend_unavailable` 503 (API server, or the pod never became ready: `details.reason = "not_ready"`) |
| `GET /v1/sandboxes` | `200` + `SandboxList`, this owner's only | `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}` | `200` + `Sandbox` | `sandbox_not_found` 404 — also for another owner's id |
| `DELETE /v1/sandboxes/{id}` | `204`, also when it did not exist (katabe's `Destroy` is idempotent, `kata.go:243-257`) | `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}/attach` | `101` WebSocket, then frames both ways | before the upgrade, as HTTP: `bad_request` 400 (no `Panacea-Attach-Token`) · `sandbox_not_found` 404 · `sandbox_not_running` 409 · `attach_rejected` 403 (sandboxd said 401) · `backend_unavailable` 503 |

**Create, in order:** decode → owner from the principal → `Image` "" or equal to the configured
digest, then set to it → resources: 0 means the default (the balloon's size, task 12), above the
max is refused → `Machine` present, and the machine **exists for this owner and is awake**
(`machineChecker`, below) → the owner's pod count < `MAX_SANDBOXES_PER_USER` → build
`backend.Spec` (`Mounts: nil`, `MachineMounts` mapped) → `katabe.New(Config{Manager: base,
Owner: owner, ReadyTimeout})` → `Create` → map errors by `errors.Is` (never by string, as the
control plane does).

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
- [ ] **Step 2:** `machinecheck.go` (interface + a fake for tests), `sandboxes.go`, `attach.go`; the
  routes join `server.go`'s table; update task 17's `TestNoRouteIsASideEffectingGet` allowlist
  with the two read routes.
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
