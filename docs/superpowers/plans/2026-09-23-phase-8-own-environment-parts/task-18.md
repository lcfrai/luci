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
- `control-plane/cmd/broker/sandboxgit.go`, `sandboxgit_test.go` — the sandbox git route (env R4
  ruling 1) and the runner-only admission (ruling 2) (new)
- `control-plane/cmd/broker/server.go` — the six routes join the table (modify)
- `control-plane/internal/backend/backend.go` — `KubeLabelControlPlane`, `Spec.ControlPlane` and
  its `ValidateSpec` rule (modify; review I6), and the `MaxMessageBytes` comment at `:29-37`, which
  step 3 updates to name five sites (review m3)
- `control-plane/internal/sandbox/manager.go` and `internal/backend/katabe/kata.go` — the label
  on the pod and its Secret, passed through from `Spec.ControlPlane` (modify; review I6)

#### Contract — `brokerapi/wire.go` (complete)

```go
package brokerapi

import "time"

// HeaderAttachToken carries a sandbox's attach token on GET …/attach. The
// broker passes it to sandboxd as the bearer token and never stores it. It is
// a separate header because Authorization carries the caller's realm token (OD-30).
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
	// ControlPlane is the creating control plane's install id (task 23's
	// machines.InstallID), ^[0-9a-f]{32}$, required. The broker stamps it as the
	// pod's and Secret's backend.KubeLabelControlPlane label, so a laptop's
	// reaper can tell its own sandboxes from another laptop's of the same owner
	// (review I6). It is an identifier, not a secret, and authorises nothing:
	// the owner label still scopes every route.
	ControlPlane string `json:"controlPlane"`
}

// QueryControlPlane is GET /v1/sandboxes' optional filter: ?controlPlane=<id>
// lists only the owner's sandboxes carrying that install id. remotebe always
// sends it (task 22). Without it the answer is every sandbox of the owner,
// which is the seam for ending a session another laptop started (review I6's
// owner question).
const QueryControlPlane = "controlPlane"

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

#### Contract — the install-id label (review I6)

```go
// package backend (backend.go), beside KubeLabelOwner (task 14):
const KubeLabelControlPlane = "panacea-app/control-plane" // the creating control plane's install id; absent on the laptop

// Spec gains one field, after MachineMounts (task 15):
//
//	ControlPlane string // for the label only, like SessionID; a backend never reads it back
//
// ValidateSpec gains, wrapping ErrUnsupported: ControlPlane, when set, must
// match ^[0-9a-f]{32}$. sandbox.Spec gains the same field; katabe passes it
// through, and sandbox.Manager writes it as KubeLabelControlPlane on the pod and
// on its attach Secret when it is non-empty. dockerbe ignores it: the laptop's
// Docker sandboxes keep phase 2's one-control-plane-per-daemon assumption.
```

Why a label and not the owner: the broker scopes by **owner** (`tid/oid`), and one owner can run
two control planes, on a second PC or after a reinstall with a fresh state directory. Each
laptop's reaper destroys any listed sandbox that its registry does not name (rule 4,
`destroyOrphan`, `reaper.go:452-463`). Without this label, the second laptop's reaper would
destroy the first laptop's running cloud sessions within one reap interval.

#### Contract — the routes

| Route | Success | Failures (problem code, status) |
|---|---|---|
| `POST /v1/sandboxes` | `201` + `Sandbox`, **after** the pod answers `/healthz` (katabe's wait, up to `READY_TIMEOUT`) | `invalid_json` 400 · `bad_request` 400 (`backend.ValidateSpec` refused; message names the field) · `image_not_allowed` 400 · `resources_too_large` 400 · `machine_not_found` 404 · `machine_asleep` 409 · **`machine_full` 409** (only under the machine affinity; `details.sessionsPerMachine`: the machine's other session pods at the refusal, possibly `0` when other users' pods fill the node or it is cordoned; the new pod is already deleted — review I5, b05 re-review) · `conflict` 409 (id exists for this owner) · `quota_exceeded` 429 (`details.limit`) · `backend_unavailable` 503 (API server, or the pod never became ready: `details.reason = "not_ready"`, and the pod is deleted before the answer) |
| `GET /v1/sandboxes` | `200` + `SandboxList`, this owner's only; with `?controlPlane=<id>`, only those whose pod carries `panacea-app/control-plane: <id>` (a label selector beside the owner's, so the filter runs in the API server). Each `Sandbox.Labels` carries the pod's labels, the control-plane label among them (katabe copies them, `kata.go:181-186`) | `bad_request` 400 (a `controlPlane` that is not 32 lowercase hex) · `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}` | `200` + `Sandbox` | `sandbox_not_found` 404 — also for another owner's id |
| `DELETE /v1/sandboxes/{id}` | `204`, also when it did not exist (katabe's `Destroy` is idempotent, `kata.go:243-257`) | `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}/attach` | `101` WebSocket, then frames both ways | before the upgrade, as HTTP: `bad_request` 400 (no `Panacea-Attach-Token`) · `sandbox_not_found` 404 · `sandbox_not_running` 409 · `attach_rejected` 403 (sandboxd said 401) · `backend_unavailable` 503 |
| `POST /v1/sandboxes/{id}/git/v1/{op}` (env R4 ruling 1) | the runner's answer as received: status, `Content-Type`, body | before the proxy, as attach's: `bad_request` 400 (no `Panacea-Attach-Token`, or `{op}` not one segment of `^[a-z-]{1,16}$`) · `sandbox_not_found` 404 (another owner's id too) · `sandbox_not_running` 409 · `attach_rejected` 403 (sandboxd said 401) · `backend_unavailable` 503. In the **runner's** envelope (`{"code":…}`, phase-3 `task-24.md:194-202`): `413 too_large` (a request past its cap) · `504 git_timeout` (the deadline, before any header) |

**Sandbox git, in order (env R4 ruling 1, `.superpowers/sdd/env/r4-env-rulings.md`).** Lane git
reaches phase 3's runner, `sandboxd`'s `/git/v1/`, in the one pod that mounts the lane.
1. Steps 1 and 2 of attach below: the token header, then owner-scoped `katabe` lookup, so another
   owner's id is 404 and the pod sees nothing.
2. `POST http://<podIP>:8080/git/v1/<op>` with `Authorization: Bearer <attach token>` and the
   caller's `Content-Type`; no other caller header, the realm token never. The request names no
   directory, and the broker adds none (phase-3 `task-24.md:91-94`).
3. **Caps, per direction** (phase-3 `task-24.md:91-93`, `:365-370`): `fetch` request
   `MaxBundleBytes`, response `MaxJSONBytes`; `bundle` request `MaxJSONBytes`, response
   `MaxBundleBytes`; every other op `MaxJSONBytes` both ways. The values are phase 3's host copies
   (`lanegit`, which declares the same four, `:361`); tests lower them through the handler's
   fields. Both directions stream through `io.Copy` over an `io.LimitReader` of cap + 1, never
   buffered. A request past its cap is `413` in the runner's envelope, so phase 3's client
   decodes a `RunnerError{413, "too_large"}` (`:388`), not "the runner did not answer"
   (`:582-583`). A response past its cap is cut: the status is sent, so the broker stops and
   aborts the stream; phase 3's client stops at the same cap (`:568-569`).
4. Deadline: the op's timeout (`ReadTimeout`, or `BundleTimeout` for `bundle` and `fetch`) plus
   5 s (`:366-369`, `:577-578`). Past it before a header, `504` `{"code":"git_timeout"}`.
5. sandboxd's `401` is `403 attach_rejected`; every other answer passes through unchanged.

**Runner-only admission (env R4 ruling 2).** A create with `SessionID` `""`, `Machine` set, and
`MachineMounts` exactly one `lanes/<p>/<d>` at `/home/dev/work`, not read-only, is a runner-only
pod (phase 3's pool step 2, phase-3 `task-24.md:678-684`, sent by task 25b). It runs the same
image, affinity and fresh attach token as a session. Its resources are two new `Cfg` fields read
where `SESSION_*` is, `PANACEA_APP_BROKER_RUNNER_CPU_MILLIS` and `…_RUNNER_MEMORY_MIB`, default
`500` and `1024`, whatever the body asks: the smallest the broker admits (arithmetic,
**UNVERIFIED**; task 32's P07 measures). It counts against the per-user cap
and `machine_full` like any pod. A `projects/…` mount is `400 bad_request` from `ValidateSpec`
(task 15, R4.1), for sessions and runners alike.

**Create, in order:** decode → owner from the principal → `Image` "" or equal to the configured
digest, then set to it → resources: 0 means the default (`Cfg`'s `SESSION_*`, the balloon's size,
task 12), and 0 is what every cloud session sends (remotebe never sends a size, task 22); above
the SKU-derived max (task 17) is `resources_too_large`, because such a session fits on no node
beside its machine pod → `Machine` present, and the machine **exists for this owner and is awake**
(`machineChecker`, below) → the owner's pod count < `MAX_SANDBOXES_PER_USER` → build
`backend.Spec` (`Mounts: nil`, `MachineMounts` mapped, `ControlPlane` copied; an empty or
malformed `ControlPlane` is `bad_request` naming the field, review I6) → `katabe.New(Config{Manager: base,
Owner: owner, ReadyTimeout})` → `Create` under a context that `watchUnschedulable` (below) may
cancel, **armed only when `Cfg.MachineAffinity` is true** → on failure, **delete the pod this
request created** (below) → on success, **re-count** the owner's session pods by consistent read
(no `resourceVersion`, never an informer cache; pods with a `deletionTimestamp` not counted): over
`MAX_SANDBOXES_PER_USER` → delete this request's pod and its Secret through `deleteIfOurs`
(below; the same identity check) and answer 429 `quota_exceeded`. The count
before the create is only the cheap early refusal: two replicas (task 29) can both pass it, and
the re-count is what keeps the owner at or under the cap (both racers may roll back, which fails
closed; task 19 applies the same rule to machines, review m11) → answer in the order below, mapping errors by `errors.Is` (never by
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
  sandbox`, `panacea-app/machine-id: <machine>`, `panacea-app/owner: <owner>` — a machine id is
  unique per owner only, since task 19's names carry the owner (review m12) — not terminating, and
  not this request's own pod,
  which it excludes by name, `sandbox.PodName(owner, id)` (task 14), because the count runs
  **before** `deleteIfOurs`), then deletes, then
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
  refuses the size), is a 403 that katabe also wraps as `ErrUnavailable` (admission runs before
  the API server's name check, so a refused duplicate is a 403, not a 409), and there the pod
  named `sandbox.PodName(owner, id)` is the owner's running session. The rule:
  - After **every** `Create` error except `ErrAlreadyExists`, the broker runs
    `deleteIfOurs(ctx, pods, secrets, owner, id, spec.AttachToken)` under a fresh 10 s context,
    then answers. The names come from task 14's exported `sandbox.PodName` and
    `sandbox.AttachSecretName`, the same functions the owner view creates with, so the rollback
    names the pod this request made (`sandbox-<owner[:8]>-<id>`), never a bare `sandbox-<id>`
    that no broker pod has (b09 re-review 2).
  - **The identity is the token in the pod's Secret, not in the pod.** Since task 14 the pod's
    `backend.EnvToken` entry is a `secretKeyRef` with an empty `value` (`manager.go:110` holds the
    literal only until then), so the pod spec carries no token to compare. `deleteIfOurs` gets
    the Secret `AttachSecretName(owner, id)` (`secrets get`, task 29) and compares its
    `Data[sandbox.AttachSecretKey]` with this request's token, using
    `subtle.ConstantTimeCompare`. A missing Secret, or another token, is a no-op that returns nil.
    On a match it gets the pod `PodName(owner, id)` and deletes it with
    `Preconditions{UID: pod.UID}` (a missing pod is skipped), then deletes the Secret with
    `Preconditions{UID: secret.UID}`.
  - **Why the Secret proves it.** Task 14 creates the Secret before the pod, a create whose
    Secret exists fails there with `AlreadyExists` and leaves it alone, and an ambiguous pod
    create keeps the Secret. So while the Secret holds this request's token, no other request
    can have created that pod. Every create carries a fresh token (`ValidateSpec` refuses an
    empty one, `backend.go:232`), so only this request matches. That covers the running session
    behind an admission refusal (its Secret holds the old token), and the `ErrUnsupported`
    refusals (`ValidateSpec` and `kata.go:81-89`), which return before any Secret exists.
  - **A late-stored pod.** If the API server stores an ambiguous pod create after the rollback
    has deleted the Secret, that pod can never start (its `secretKeyRef` names a missing
    Secret), and it carries the control-plane label, so the laptop's reaper, which lists by
    `?controlPlane=` and destroys what its registry does not hold, removes it. The window is the
    API server's own request timeout. UNVERIFIED on a cluster; offline, a fake clientset cannot
    store late.
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

The caller's `Authorization` header never reaches the pod: `oidcauth.Middleware` removed it
(task 16, renamed by task 17 step 0), and `backend.Dial` sends only the attach token.

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

// watchUnschedulable calls cancel once pod name (sandbox.PodName(owner, id),
// the owner view's "sandbox-<owner[:8]>-<id>", task 14; b09 re-review 2) has reported PodScheduled=False, reason Unschedulable, with
// an empty status.nominatedNodeName, continuously for grace. A nominated node,
// a scheduled pod or a deleted pod resets or ends the watch. It returns a func
// that stops the watch and reports whether it fired.
func watchUnschedulable(ctx context.Context, pods corev1client.PodInterface, name string,
	grace time.Duration, cancel context.CancelFunc) (stop func() (fired bool))

// machineSessions counts the non-terminating session pods of owner's machine
// (component=sandbox, machine-id=machine, owner=owner) in the broker's
// namespace, except the pod named except (this request's own: the count runs
// before its delete).
func machineSessions(ctx context.Context, pods corev1client.PodInterface, owner, machine, except string) (int, error)

// deleteIfOurs deletes owner's sandbox id only if the Secret
// sandbox.AttachSecretName(owner, id) holds token under sandbox.AttachSecretKey
// (constant-time): then the pod sandbox.PodName(owner, id), if present, and the
// Secret, each with a UID precondition. A missing Secret or another token is a
// no-op and returns nil (b05 re-review 1; b09 re-review 2: the pod spec holds
// no token since task 14).
func deleteIfOurs(ctx context.Context, pods corev1client.PodInterface,
	secrets corev1client.SecretInterface, owner, id, token string) error
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
| `TestTwoReplicasNeverTakeAnOwnerPastTheCap` | owner at 7 pods; two broker servers on **one** fake clientset each create a session, with a `PrependReactor` on pod `create` holding both at a barrier until both have made the early count → at most one 201, the other 429; afterwards the owner has at most 8 non-terminating session pods, and a rolled-back request's pod and Secret are deleted by UID through `deleteIfOurs` (review m11; b09 re-review 2) | a count-then-create cap, which two replicas overrun together |
| `TestAFullMachineIs409MachineFullInSeconds` | affinity on; three running session pods of this owner on machine `m-1`, and beside them, none of which may count (b05 re-review 7): a session pod of this owner on `m-2`, another owner's session pod on its machine `m-3`, a third owner's session pod on **its own** `m-1` (machine ids are unique per owner only, since task 19's names carry the owner, review m12), the `m-1` machine pod itself (component `machine`), and a terminating session pod on `m-1`; the reactor marks the new pod `PodScheduled=False/Unschedulable`, no nomination, never Running; grace 50 ms, `READY_TIMEOUT` 5 s → 409 `machine_full`, `details.sessionsPerMachine = "3"`, answered in under 1 s, and the new pod and its Secret are gone (the pod was created through the owner view, so its token lives only in the Secret, b09 re-review 2); a `delete pods` reactor records the one delete of the new pod, whose `DeleteOptions.Preconditions.UID` equals that pod's UID (review I5; b05 re-review 1) | the two-minute hang and generic 503; a Pending pod left holding the owner's cap |
| `TestABriefUnschedulableIsNotFull` | the new pod is `Unschedulable` with no nomination for 20 ms (grace 200 ms), then Running → 201, the pod kept | a watcher that fires on the first `Unschedulable` event and ignores the grace, refusing a session whose room was freeing up (a terminating pod on the node) |
| `TestPreemptionUnderWayIsNotFull` | the new pod is `Unschedulable` **with** `nominatedNodeName` set for 200 ms (grace 50 ms), then Running → 201 | a refusal every time a session preempts a balloon, which is the warm path task 12 builds |
| `TestAReadyTimeoutDeletesThePod` | the pod stays Pending with no scheduling condition; `READY_TIMEOUT` 400 ms → 503 `backend_unavailable`, `details.reason = "not_ready"`, and the pod is gone | katabe's keep-the-pod rule carried into the broker, where the pod is an orphan counting against the cap |
| `TestAConflictNeverDeletesTheExistingPod` | a running session already has the id, created through the owner view (so its Secret exists and the new create fails there with `AlreadyExists`) → 409 `conflict`, and that pod and its Secret are still there; once with another token, and once with **the same token** (a laptop re-sending a create whose 201 it lost, b05 re-review 1) | a cleanup after `ErrAlreadyExists`, which the token match alone would not stop: it kills the owner's running session |
| `TestAFullMachineIs409EvenWhenKatabeHidesTheCancel` | affinity on; the new pod is `Unschedulable`, no nomination; a `get pods` reactor holds the first `Get` of the new pod (katabe's first poll, made at once after the create, `kata.go:130`, so it is in flight when the watcher fires) for 3× the grace, returns `errors.New("client rate limiter Wait returned an error: context canceled")` unwrapped, as the real client does when its context is cancelled mid-call, and lets every later `Get` through (so `deleteIfOurs`'s own `Get` works). katabe's error then wraps `ErrUnavailable`, not `context.Canceled`, which a test-only `serverDeps.onCreateError` hook records and the test asserts, so the test proves it took the path it names → 409 `machine_full` and the pod is gone (b05 re-review 1) | the old "`ErrNotRunning` or a context error" cleanup rule, which keeps the pod and answers 503 or a `machine_full` whose "the pod is deleted" is false |
| `TestAnAdmissionRefusalNeverDeletesTheRunningSession` | the owner's running session `x` is created **through the owner view** (`katabe.Config{Owner: o}`), so its pod is `sandbox.PodName(o, "x")`, its token env value is empty, and its Secret `AttachSecretName(o, "x")` holds `tok-A`; a create of `x` with token `tok-B` meets a `create secrets` reactor returning `403 Forbidden` (exceeded quota: admission answers before the name check) → 503 `backend_unavailable`, and the pod still runs and its Secret still holds `tok-A` (b05 re-review 1; b09 re-review 2) | a cleanup keyed on the error class ("every error except `ErrAlreadyExists` and `ErrUnsupported`"), which destroys the owner's running session behind a refused duplicate |
| `TestDeleteIfOursReadsTheSecret` | pods and Secrets made through the owner view: (a) Secret holds `tok-A`, call with `tok-A` → the pod and the Secret are deleted, and a `delete` reactor records `Preconditions.UID` equal to each object's UID; (b) with `tok-B` → both kept; (c) no Secret, pod present → no-op, nil; (d) Secret holds `tok-A`, no pod → the Secret is deleted, nil (b09 re-review 2) | the env-value match, which under task 14 compares `""` with the token and never deletes, so every failed, `machine_full` or over-cap create keeps its pod |
| `TestWithoutAffinityUnschedulableWaitsForTheTimeout` | `Cfg.MachineAffinity` false; the new pod is `Unschedulable` with no nomination for 10× the grace (grace 50 ms, `READY_TIMEOUT` 5 s), then Running → 201, the pod kept (b05 re-review 3) | a watcher armed whatever the affinity, which refuses every session in the NFS fallback that the autoscaler would have placed |
| `TestTheCallerLeavingDeletesThePod` | the client cancels its request while the pod is Pending → the pod is gone within 1 s | an orphan per abandoned create |
| `TestAnotherOwnersSandboxIs404Everywhere` | owner B: `GET`, `DELETE` (→ 204 but the pod survives), `attach` on A's id → 404 / no-op / 404 | the cross-user access the broker exists to prevent |
| `TestListReturnsOnlyTheCallersSandboxes` | A has 2, B has 1 → A lists 2 | `Manager.List` unscoped (`manager.go:240-247`) |
| `TestCreateStampsTheControlPlane` | `controlPlane: "<32 hex>"` → the pod and its Secret carry `panacea-app/control-plane: <id>`, and the answer's `labels` include it (review I6) | an install id the reaper can never find |
| `TestCreateWithoutAControlPlaneIs400` | `""`, `"ABC"` and 31 hex characters → 400 `bad_request` naming `controlPlane`, no pod | an unlabelled cloud sandbox that every laptop of the owner may treat as its own |
| `TestListFiltersByControlPlane` | owner A has two pods with id `X` and one with `Y`; owner B has one with `X` → `?controlPlane=X` lists A's two; no query lists A's three; `?controlPlane=zz` → 400 | the filter dropped, or applied across owners |
| `TestDeleteIsIdempotent` | two DELETEs → 204, 204 | the reaper's second pass a 500 |
| `TestErrorsAreMappedByIsNotByString` | a fake katabe error wrapping `backend.ErrUnavailable` with an unrelated message → 503 | string matching |
| `TestTheAttachTokenIsNeverLogged` | a create and an attach with token `tok-SECRET` → the captured log has no `tok-SECRET` | a token in a log line |

And beside them (review I6): `backend_test.go` gains `TestValidateSpecRefusesAMalformedControlPlane`
(`"ABC"` → `ErrUnsupported`; `""` and 32 hex characters pass), and `internal/sandbox` gains
`TestManagerWritesTheControlPlaneLabel` (the pod and the Secret carry it; with `""`, neither has
the key).

`attach_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAttachRelaysFramesBothWays` | a frame from the caller reaches the stand-in and back | a relay in one direction |
| `TestAttachCarriesA64KiBFrame` | a 64 KiB stdout frame arrives whole | the read limit left at 32 KiB on the broker's `Accept` (the shared rule's new site) |
| `TestAttachWithoutTheTokenHeaderIs400` | before any upgrade | an upgrade the pod will refuse |
| `TestAttachToAStoppedSandboxIs409BeforeUpgrade` | HTTP 409 `sandbox_not_running`, no 101 | a close code remotebe cannot map |
| `TestARejectedTokenIs403AttachRejected` | the stand-in answers 401 → HTTP 403 `attach_rejected` | a 500 that looks like a broker bug |
| `TestThePodNeverSeesTheRealmToken` | the stand-in records its request headers: `Authorization` is `Bearer <attach token>`, never the caller's realm token | the user's token forwarded into a sandbox (ADR-H1 §5.8, `67609ab` `:545`) |
| `TestKeepalivePingsTheCaller` | with a 50 ms test interval, the caller observes pings while idle | idle sockets cut by the edge |
| `TestShutdownClosesRelaysWith1001` | cancelling the server context → the caller sees status 1001 | a silent drop remotebe cannot tell from a crash |

`sandboxgit_test.go` (a stand-in sandboxd on `httptest`, recording each request; caps and
timeouts lowered through the handler's fields):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSandboxGitRelaysByteForByte` | a `fetch` with a 64 KiB random body: the stand-in saw `POST /git/v1/fetch`, those bytes, the `Content-Type`, and `Authorization: Bearer <attach token>` only; its `422 {"code":"git_failed","gitSays":"fatal: x"}` answer and a `bundle`'s 64 KiB body reach the caller unchanged, status and bytes | a re-mapped runner error; a body rewritten or truncated; the realm token forwarded |
| `TestSandboxGitAnotherOwnersIdIs404` | owner B on A's running id → `404 sandbox_not_found`; the stand-in saw nothing | the owner filter skipped on the git route |
| `TestSandboxGitRefusesBeforeTheProxy` | no token header → 400; `{op}` `../healthz` or `a/b` → 400; a stopped sandbox → `409 sandbox_not_running`; the stand-in saw nothing | a path smuggled into the pod; a proxy to a pod that is not running |
| `TestSandboxGitRejectedTokenIs403` | the stand-in answers 401 → `403 attach_rejected` | sandboxd's 401 passed through, where it reads as the broker's own 401 |
| `TestSandboxGitCapsEachDirection` | cap `C`: a `fetch` body of `C` → proxied, `C + 1` → `413 {"code":"too_large"}`, `application/json`; a `status` body past the JSON cap → the same 413; a `bundle` answer of `C + 1` → the caller reads at most `C` bytes, then an error; a `fetch` answer past the JSON cap → cut the same way | the bundle cap on the wrong direction; a 413 in the httpx envelope, which phase 3's client reads as "did not answer" |
| `TestSandboxGitDeadlineIsTheOpsTimeoutPlusFive` | with `ReadTimeout` 50 ms and the 5 s slack lowered to 50 ms, a stand-in that never answers `status` → `504 {"code":"git_timeout"}` between 100 ms and 1 s; `fetch` waits `BundleTimeout` + slack | one deadline for every op; no deadline |
| `TestCreateAdmitsARunnerOnlySpecAtRunnerSize` | `SessionID ""`, `Machine` set, one `lanes/p-1/d-0a1b2c` mount at `/home/dev/work` → 201, the pod's requests and limits are `500m` / `1024Mi` though the body asked more; the same with a second (`conversations/…`) mount → the session size; a `projects/app` mount → `400 bad_request` naming R4.1 | a runner-only pod sized as a session, taking a session's place on the machine; detection keyed on `SessionID` alone |

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
  (delete by name) → `TestAnAdmissionRefusalNeverDeletesTheRunningSession`,
  `TestDeleteIfOursReadsTheSecret` (b); **b09 re-review 2:** match the pod's env value instead of
  the Secret → `TestDeleteIfOursReadsTheSecret` (a), `TestAFullMachineIs409MachineFullInSeconds`;
  name the rollback's pod `"sandbox-"+id` → `TestAFullMachineIs409MachineFullInSeconds`; skip
  the Secret's delete → `TestDeleteIfOursReadsTheSecret` (a, d); arm
  `watchUnschedulable` whatever `Cfg.MachineAffinity` is →
  `TestWithoutAffinityUnschedulableWaitsForTheTimeout`; `machineSessions` without the
  `machine-id` selector, without the `owner` selector (the third owner's `m-1` pod counts),
  without the `component: sandbox` selector (the machine pod counts), or without the terminating
  filter, each → `TestAFullMachineIs409MachineFullInSeconds` (the count leaves `"3"`);
  the default `MAX_CPU_MILLIS` back to `8000` → `TestCreateRefusesOversizedResources`; the size
  taken from `backend.DefaultResources` instead of `Cfg` → `TestZeroResourcesTakeTheBrokersDefault`.
  The two named as likeliest to survive, and closed: `deleteIfOurs` without the UID precondition
  (the fake clientset may ignore preconditions, so only the `delete pods` reactor's recorded
  options catch it) → `TestAFullMachineIs409MachineFullInSeconds`; `machineSessions` ignoring
  `except` (a fake delete removes the pod at once, so this is caught only because the count runs
  before the delete) → the same test (the count becomes `"4"`).
- [ ] **Step 2b (review I6):** mutation check, the same way: `controlPlane` optional on create →
  `TestCreateWithoutAControlPlaneIs400`; the `?controlPlane` selector dropped →
  `TestListFiltersByControlPlane`; the label written on the Secret only, not the pod →
  `TestCreateStampsTheControlPlane`.
- [ ] **Step 2c (env R4 review 1, C2):** `sandboxgit.go` after its tests, each shown red first.
  Mutation check, the same way: the owner-scoped lookup replaced by an unscoped one →
  `TestSandboxGitAnotherOwnersIdIs404`; sandboxd's 401 passed through →
  `TestSandboxGitRejectedTokenIs403`; the bundle cap applied to `fetch`'s response instead of its
  request → `TestSandboxGitCapsEachDirection`; the request 413 written with `httpx` →
  the same test (its content check); one deadline for every op →
  `TestSandboxGitDeadlineIsTheOpsTimeoutPlusFive`; runner-only detection on `SessionID ""` alone →
  `TestCreateAdmitsARunnerOnlySpecAtRunnerSize`. The two named beforehand as likeliest to survive:
  a body buffered with `io.ReadAll` before the cap (every test body fits in memory; nothing but a
  heap profile sees it, so it is closed by review of `sandboxgit.go`, which must hold no
  `ReadAll`: `git grep -n ReadAll -- control-plane/cmd/broker/sandboxgit.go` prints nothing), and
  the `{op}` segment check (the pod's own mux would 404 `../healthz` anyway), killed only by the
  stand-in's "saw nothing" assertion.
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
- That the install id separates two laptops for anything but listing. `GET`, `DELETE` and
  `attach` stay scoped by owner alone. A laptop only ever addresses the ids its own registry
  holds, and an attach still needs the sandbox's token, which only the creating laptop has. The
  unfiltered list plus `DELETE` is the seam for ending a session that another laptop started
  (review I6's owner question).
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
