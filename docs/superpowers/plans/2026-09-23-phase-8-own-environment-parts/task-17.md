### Task 17: `cmd/broker` — configuration, listener, authentication, health

The broker is `katabe` moved into the cluster (OQ-1, P8-D4): the only process with Kubernetes
credentials for sandboxes, reached from laptops through the edge, trusting nothing but a valid
Entra token. This task builds its skeleton — everything but the sandbox and machine routes (tasks
18, 19) — so that those tasks add handlers to a server that already authenticates, scopes and
shuts down correctly.

It is a new `main` package in the existing module, as the Hub is (ADR-H1 §4.1): it imports
`internal/entra`, `internal/sandbox`, `internal/backend/katabe`, `internal/httpx` and the
new `internal/brokerapi`. It does **not** import `internal/api`, `internal/registry` or anything of
the laptop's control plane: the broker holds no session registry; the laptop's registry stays the
one durable home of attach tokens (`backend.go:281-285`), and the broker only passes them through.

**Why the broker listens on all interfaces.** The rule "the control plane listens on loopback
only" is about the laptop, where a non-loopback bind is a shell on the LAN. The broker is a server
inside a pod: its listener is the pod IP, reachable only as NetworkPolicy allows (task 29: from
the edge's pods, on 8080), and it refuses every request that is not authenticated.

**Files:**
- `control-plane/cmd/broker/{main.go,config.go,server.go}` and `{config_test.go,server_test.go}` (new)
- `control-plane/internal/brokerapi/codes.go` — the broker's problem codes (new; task 18 adds the wire types beside it)

#### Contract — configuration (complete; every variable read in `config.go` and nowhere else)

| Variable | Default | Meaning |
|---|---|---|
| `PANACEA_APP_BROKER_ADDR` | `:8080` | listen address (in-cluster) |
| `PANACEA_APP_BROKER_NAMESPACE` | `panacea-sandboxes` | where every pod, Secret and claim lives (P8-D5) |
| `PANACEA_APP_RUNTIME_CLASS` | `kata-vm-isolation` | P8-D15 — the same variable name `cmd/api` reads (task 14) |
| `PANACEA_APP_BROKER_SESSION_PRIORITY_CLASS` | `panacea-session` | task 11 |
| `PANACEA_APP_BROKER_MACHINE_PRIORITY_CLASS` | `panacea-machine` | task 11 |
| `PANACEA_APP_BROKER_ENTRA_AUTHORITY` | `https://login.microsoftonline.com` | `entra.Config.AuthorityHost` |
| `PANACEA_APP_BROKER_ENTRA_TENANTS` | — **required** | comma-separated enrolled tenant ids |
| `PANACEA_APP_BROKER_ENTRA_AUDIENCE` | — **required** | the broker API's client id (task 09 output) |
| `PANACEA_APP_BROKER_ENTRA_SCOPE` | `Sandboxes.Access` | `entra.Config.RequiredScope` |
| `PANACEA_APP_BROKER_ALLOWED_CLIENTS` | — **required** | comma-separated; the desktop's client id |
| `PANACEA_APP_BROKER_SANDBOX_IMAGE` | — **required** | the only session image, **by digest** |
| `PANACEA_APP_BROKER_MACHINE_IMAGE` | — **required** | the only machine image, by digest |
| `PANACEA_APP_BROKER_SESSION_CPU_MILLIS` | `2000` | a session's size when the caller sends 0 — the same value as the balloon (task 12) and `backend.DefaultResources` (`backend.go:110`) |
| `PANACEA_APP_BROKER_SESSION_MEMORY_MIB` | `4096` | ditto |
| `PANACEA_APP_BROKER_MAX_CPU_MILLIS` | `8000` | largest session a caller may request |
| `PANACEA_APP_BROKER_MAX_MEMORY_MIB` | `32768` | ditto |
| `PANACEA_APP_BROKER_MAX_SANDBOXES_PER_USER` | `8` | concurrent session pods per owner |
| `PANACEA_APP_BROKER_MAX_MACHINES_PER_USER` | `2` | machines per owner (task 19) |
| `PANACEA_APP_BROKER_MACHINE_DISK_GIB` | `64` | a new machine's disk (task 19) |
| `PANACEA_APP_BROKER_MACHINE_STORAGE_CLASS` | `panacea-machine-disk` | task 11 |
| `PANACEA_APP_BROKER_MACHINE_AFFINITY` | `true` | `sandbox.Config.MachineAffinity` (task 15); `false` only with RWX storage |
| `PANACEA_APP_BROKER_MACHINE_IDLE_SLEEP` | `30m` | a machine with no session pods for this long is put to sleep (task 19); `0` disables |
| `PANACEA_APP_BROKER_READY_TIMEOUT` | `120s` | a pod's time to answer `/healthz` (Kata cold start) |
| `PANACEA_APP_BROKER_SHUTDOWN_GRACE` | `20s` | drain time on SIGTERM |
| `PANACEA_APP_LOG_LEVEL` | `info` | as `cmd/api` |

Validation, each failure exiting 2 with one sentence naming the variable: every required value
present; both images match `@sha256:[a-f0-9]{64}$` (a tag is refused — P8-D8, OD-8's pin); the
runtime class a DNS-1123 subdomain; durations parse; limits positive; the authority `https` unless
its host is loopback (a local run against the Hub's devissuer).

```go
package main

// Config is the broker's whole configuration, parsed once.
type Config struct {
	Addr                  string
	Namespace             string
	RuntimeClass          string
	SessionPriorityClass  string
	MachinePriorityClass  string
	Entra                 entra.Config // AuthorityHost, TenantIDs, Audience, RequiredScope, AllowedClients
	SandboxImage          string
	MachineImage          string
	SessionCPUMillis      int
	SessionMemoryMiB      int
	MaxCPUMillis          int
	MaxMemoryMiB          int
	MaxSandboxesPerUser   int
	MaxMachinesPerUser    int
	MachineDiskGiB        int
	MachineStorageClass   string
	MachineAffinity       bool
	MachineIdleSleep      time.Duration
	ReadyTimeout          time.Duration
	ShutdownGrace         time.Duration
	LogLevel              slog.Level
}

func loadConfig(getenv func(string) string) (Config, error)
```

#### Contract — `internal/brokerapi/codes.go` (complete)

```go
// Package brokerapi is the broker's wire contract: the problem codes here, and
// (task 18) the request and response types. Both the broker and remotebe import
// it, so the two sides cannot spell a field differently.
package brokerapi

// Problem codes the broker writes, beside httpx's shared ones (invalid_json,
// bad_request, forbidden_origin, conflict, internal, backend_unavailable,
// sandbox_not_running) and entra.CodeUnauthenticated.
const (
	CodeSandboxNotFound   = "sandbox_not_found"   // 404 — also for another owner's sandbox: existence is not disclosed
	CodeMachineNotFound   = "machine_not_found"   // 404 — likewise
	CodeImageNotAllowed   = "image_not_allowed"   // 400 — a spec named an image other than the configured digest
	CodeResourcesTooLarge = "resources_too_large" // 400 — above PANACEA_APP_BROKER_MAX_*
	CodeQuotaExceeded     = "quota_exceeded"      // 429 — the owner is at MAX_SANDBOXES / MAX_MACHINES
	CodeMachineAsleep     = "machine_asleep"      // 409 — a session create on a sleeping machine; wake it first
	CodeMachineBusy       = "machine_busy"        // 409 — sleep or delete while sessions run
	CodeAttachRejected    = "attach_rejected"     // 403 — sandboxd refused the attach token (backend.ErrUnauthorized)
)
```

#### Contract — the server skeleton

```go
// newServer wires the routes behind, in order: a recover handler (500 internal,
// the panic logged, never echoed), a request logger (method, route pattern,
// status, duration, owner hash — never a header value), an Origin refusal (any
// request carrying an Origin header gets 403 forbidden_origin: only the control
// plane calls the broker, never a browser — the Hub's rule, ADR-H1 :436-437),
// then entra's Middleware for everything under /v1/.
func newServer(cfg Config, v *entra.JWKSValidator, deps serverDeps) http.Handler

// serverDeps is what handlers need; tasks 18 and 19 add their fields here.
type serverDeps struct {
	Pods    kubernetes.Interface // in-cluster client
	Log     *slog.Logger
	Now     func() time.Time
}

// ownerOf is the only place a request becomes an owner label value.
func ownerOf(r *http.Request) (owner string, c entra.Claims, ok bool)
// = sandbox.OwnerLabelValue(c.TenantID, c.ObjectID)
```

Routes this task registers (tasks 18–19 add theirs through the same table):

| Route | Auth | Answer |
|---|---|---|
| `GET /healthz` | none | `200 {"status":"ok"}` — the process is up |
| `GET /readyz` | none | `200 {"status":"ready"}` when `Validator.Ready` succeeds and a `pods list limit=1` in the namespace succeeds within 1 s; else `503 {"status":"not_ready"}` with no detail |
| `GET /v1/whoami` | Entra | `200 {"tenantId":"…","objectId":"…","owner":"<32 hex>"}` — the smoke test's first authenticated call (task 31) |

`http.Server`: `ReadHeaderTimeout: 10s`, `IdleTimeout: 120s`, no `WriteTimeout` (the attach
route is a long-lived WebSocket), `MaxHeaderBytes: 64 << 10`. SIGTERM → stop accepting, wait up
to `ShutdownGrace` for in-flight requests; open WebSocket relays are closed with status 1001
(going away) so the laptop's `remotebe` reconnects.

main builds the Kubernetes client from `rest.InClusterConfig()` **only** — there is no kubeconfig
variable, by design (S8.2 on the broker side too).

#### Tests to write first

`config_test.go`: `TestEveryRequiredVariableIsNamedWhenMissing` (five required, each missing in
turn names itself — kills a generic "config invalid"); `TestImagesMustBeDigests` (a `:tag` is
refused — kills OD-8's pin defeated by a tag); `TestDefaultsAreTheDocumentedOnes` (every default
in the table — kills drift between this part and the code); `TestAPlaintextAuthorityIsRefusedOffLoopback`.

`server_test.go` (an `httptest.Server`, a fake clientset, an `entra.Validator` against a test JWKS
server as in task 16):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestHealthzNeedsNoToken` | 200 with no `Authorization` | a liveness probe that fails because it is unauthenticated |
| `TestReadyzIsNotReadyWithoutKeys` | JWKS server down → 503, body `{"status":"not_ready"}` only | a ready broker that cannot authenticate anyone |
| `TestV1RoutesNeedAToken` | `GET /v1/whoami` with none → 401 + `WWW-Authenticate` | an unauthenticated route under /v1 |
| `TestWhoamiReportsTheOwnerHash` | the owner equals `sandbox.OwnerLabelValue(tid, oid)` | two derivations of the owner that can differ |
| `TestARequestWithAnOriginIsRefused` | `Origin: http://localhost:1420` → 403 `forbidden_origin`, even with a valid token | a browser page driving the broker with a stolen token |
| `TestThePanicHandlerNeverEchoes` | a handler that panics with a secret string → 500 `internal`, body without the string | a panic message (with a token in it) sent to the caller |
| `TestTheLogNeverHoldsHeaderValues` | a request with `Authorization` and `Panacea-Attach-Token` → the captured log contains neither value | a token in the log |
| `TestNoRouteIsASideEffectingGet` | every registered `GET` is in {`/healthz`, `/readyz`, `/v1/whoami`, and the read routes tasks 18–19 declare} — the phase 2 audit rule (`internal/api/routes_audit_test.go:133`) applied to the broker | a GET that creates a pod |

#### Steps

- [ ] **Step 1:** Config tests, then `config.go`.
- [ ] **Step 2:** Server tests, then `server.go` and `main.go`. `go build ./control-plane/cmd/broker`
  produces a binary; running it with no environment exits 2 naming the first required variable.
- [ ] **Step 3:** Full test run; commit.

#### Acceptance

```bash
go test ./control-plane/cmd/broker/ ./control-plane/internal/brokerapi/ -race -count=1   # ok
go vet ./control-plane/...
go build -o /tmp/broker ./control-plane/cmd/broker && /tmp/broker; echo "exit=$?"
# PANACEA_APP_BROKER_ENTRA_TENANTS is required: …
# exit=2
```

#### What this task does not claim

- That the broker can reach the API server from its pod — needs the chart's Role (task 29) and a
  cluster (task 31).
- Rate limiting per user beyond the concurrency caps: the edge has none either; phase 9.
