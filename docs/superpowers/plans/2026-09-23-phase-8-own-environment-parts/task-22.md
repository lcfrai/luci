### Task 22: `remotebe` — `backend.Backend` over the broker

The laptop's control plane drives every sandbox through `backend.Backend`'s seven methods
(`control-plane/internal/backend/backend.go:286-294`, seam S8.1). `remotebe` is the third
implementation, beside `dockerbe` and `katabe`, and the interface **does not change**. It calls the
broker (tasks 18–19) as the signed-in user, with a token from the Hub's token source (HA-2 —
ADR-H1 names it `entraauth.TokenSource.Token(ctx, scope)`, built in the Hub's client track C1,
`:334-342`, `:564`). Each `remotebe.Backend` is bound to **one cloud machine**; task 23's
`machines.Set` holds one per machine.

Two things phase 2's code assumes that are false for a remote sandbox, and how this task handles
each without changing the seven methods:
1. **`Endpoint` is dialable.** `watchSessionReady` polls `http://<endpoint>/healthz` from the
   laptop (`internal/api/sessions.go:569-610`), and `attach.go:182` treats an empty `Endpoint` as
   "not running". A pod IP is unreachable from a laptop. So `remotebe` reports an **opaque,
   non-empty** `Endpoint` (`remote:<id>`, never dialled) and implements a new **optional**
   interface, `backend.ReadyWaiter`, which task 23 teaches `watchSessionReady` to prefer.
2. **Readiness is the backend's.** The broker's `Create` already waits for `/healthz`
   (task 18), so `remotebe`'s `WaitReady` is a confirmation poll of `GET /v1/sandboxes/{id}`, not
   a health probe.

**Files:**
- `control-plane/internal/backend/remotebe/{remote.go,errors.go,attach.go}` and `remote_test.go`, `attach_test.go` (new)
- `control-plane/internal/backend/ready.go` — `ReadyWaiter`, `ErrQuotaExceeded`, `ErrMachineFull` and `MachineFullError` (new file, so `backend.go` — edited by tasks 14 and 15 and by phase 3 — is not touched again)

#### Contract (complete)

```go
package backend

// ReadyWaiter is an OPTIONAL capability. A backend whose sandboxes are not
// reachable from the control plane (remotebe) implements it, and the control
// plane's readiness watch calls it instead of polling Endpoint's /healthz.
// dockerbe and katabe do not implement it, and nothing about them changes.
// It is not part of Backend, so seam S8.1's seven methods stay seven.
type ReadyWaiter interface {
	WaitReady(ctx context.Context, id string, timeout time.Duration) error
}

// ErrQuotaExceeded: the caller is at a per-user limit the backend enforces
// (the broker's MAX_SANDBOXES_PER_USER). The API maps it to 429 (task 24).
var ErrQuotaExceeded = errors.New("backend: per-user limit reached")

// ErrMachineFull: the node holding the machine pod has no room for another
// session (the broker's 409 machine_full, review I5). A cloud machine is
// bounded by one node (P8-D6), so this is not "unavailable" and a retry does
// not help until a session ends. The API maps it to 409 machine_full (task 24).
var ErrMachineFull = errors.New("backend: machine is full")

// MachineFullError is what remotebe returns for machine_full: errors.Is(err,
// ErrMachineFull) holds, and SessionsPerMachine carries the broker's
// details.sessionsPerMachine (0 if the broker sent none or a non-number).
type MachineFullError struct{ SessionsPerMachine int }

func (e *MachineFullError) Error() string        // "backend: machine is full at N sessions"
func (e *MachineFullError) Is(target error) bool // target == ErrMachineFull
```

```go
package remotebe

// TokenSource is the one method remotebe needs from the Hub's
// entraauth.TokenSource (ADR-H1 §5.2). Declared here so remotebe compiles and
// is tested before the Hub's C1 lands; cmd/api adapts C1's type to it.
type TokenSource interface {
	Token(ctx context.Context, scope string) (string, error)
}

// ErrSignedOut is what a TokenSource returns when nobody is signed in. cmd/api's
// adapter maps entraauth's own signed-out error to it.
var ErrSignedOut = errors.New("remotebe: no user is signed in")

// SignedOut is the TokenSource used until the Hub's C1 lands: it always
// returns ErrSignedOut, so every cloud call is ErrUnavailable with that cause.
type SignedOut struct{}

func (SignedOut) Token(context.Context, string) (string, error) { return "", ErrSignedOut }

type Config struct {
	// BaseURL is the broker, e.g. https://broker.<zone>. https only.
	BaseURL string
	// Scope is the broker API's scope, api://<client id>/Sandboxes.Access
	// (task 09's broker_api_scope output).
	Scope  string
	Tokens TokenSource
	// HTTPClient: nil means one with a 150 s timeout (the broker's create waits
	// up to its READY_TIMEOUT, 120 s) and http.ProxyFromEnvironment.
	HTTPClient *http.Client
	Log        *slog.Logger
}

// Client is the authenticated core every broker call goes through — the
// Backend below and task 23's machine set alike. Per call it asks Tokens for
// Scope, sets the bearer, and turns a problem body into the sentinel table
// below. It never logs a token or a request body.
type Client struct{ /* unexported */ }

// NewClient refuses a non-https BaseURL (loopback excepted, for tests), an
// empty Scope, or a nil Tokens.
func NewClient(cfg Config) (*Client, error)

// Do sends in (JSON, or nothing when nil) and decodes a 2xx body into out
// (skipped when nil). A non-2xx answer is an error wrapping the mapped sentinel.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error

// Stream sends body as-is and returns the response for the caller to read and
// close; a non-2xx answer is decoded and mapped as in Do. Task 24 and 25 use it
// for the machine agent (/v1/machines/{id}/agent/...).
func (c *Client) Stream(ctx context.Context, method, path string, contentType string, body io.Reader) (*http.Response, error)

// Backend is one cloud machine's sandboxes.
type Backend struct{ /* unexported */ }

var (
	_ backend.Backend     = (*Backend)(nil)
	_ backend.ReadyWaiter = (*Backend)(nil)
)

// NewBackend binds c to one machine; every Create is on it and List is
// filtered to it. machine must match ^m-[0-9a-f]{8}$.
func NewBackend(c *Client, machine string) (*Backend, error)

// Kind is KindKata: a remote sandbox IS a Kata pod, and the persisted Kind
// string set stays the two phase 2 pinned (seam S8.1's second test).
func (b *Backend) Kind() backend.Kind

// Endpoint on every Sandbox this backend returns is RemoteEndpoint(id): opaque,
// non-empty, never dialled.
func RemoteEndpoint(id string) string { return "remote:" + id }
```

**The seven methods:**

| Method | Broker call | Notes |
|---|---|---|
| `Create` | `POST /v1/sandboxes` | refuses `spec.Mounts` with `ErrUnsupported` **before any request** (host paths never leave the laptop); sets `Machine` to the backend's; sends `MachineMounts`, `WorkDir`, argv, env, attach token. **Never a size** (b05 re-review 4): the body's `resources` stays zero, so the broker's default applies. A cloud session's size is server policy, because it is what the balloons, the LimitRange default and P8-D6's ceiling are all sized to. The laptop's `spec.Resources` (`sessionResources()`, `sessions.go:332-337`, which is `PANACEA_APP_SESSION_*` or `backend.DefaultResources` `{2000, 4096}`, never 0) is ignored on this backend |
| `Get` / `List` | `GET /v1/sandboxes/{id}` / `GET /v1/sandboxes` | `brokerapi.Sandbox` → `backend.Sandbox{Kind: KindKata, Endpoint: RemoteEndpoint(id), …}`; List filters to this backend's machine |
| `Attach` | `GET /v1/sandboxes/{id}/attach` (WSS) | headers `Authorization: Bearer <entra>` and `Panacea-Attach-Token: <token>`; `SetReadLimit(backend.MaxMessageBytes)` right after the dial — **the fifth site** of the shared rule; an HTTP refusal is decoded and mapped as below, so `attachCloseStatus` (`attach.go:330-343`) sees the same sentinels as for Docker |
| `Destroy` | `DELETE /v1/sandboxes/{id}` | 204 → nil |
| `Close` | — | releases idle connections |
| `WaitReady` | `GET /v1/sandboxes/{id}` every 250 ms | nil at `running`; `ErrNotRunning` at `stopped`; `ErrNotRunning` at the timeout |

**Error mapping** (`errors.go`, by problem code, never by message):

| Broker answer | Sentinel |
|---|---|
| 404 `sandbox_not_found`, 404 `machine_not_found` | `ErrNotFound` |
| 409 `sandbox_not_running`, 409 `machine_asleep` | `ErrNotRunning` |
| 409 `conflict` | `ErrAlreadyExists` |
| 400 `bad_request`, `invalid_json`, `image_not_allowed`, `resources_too_large` | `ErrUnsupported` (message carried) |
| 403 `attach_rejected` | `ErrUnauthorized` |
| 429 `quota_exceeded` | `ErrQuotaExceeded` |
| 409 `machine_full` | `*backend.MachineFullError{SessionsPerMachine: <details.sessionsPerMachine>}` (`errors.Is(err, ErrMachineFull)`; review I5) |
| 401 `unauthenticated`, 503, any 5xx, a network error, a `TokenSource` error | `ErrUnavailable` (wrapping `ErrSignedOut` when that was the cause) |

#### Tests to write first

`remote_test.go` — an `httptest.NewTLSServer` playing the broker (it records every request), a
fake `TokenSource` that counts calls:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestBackendSatisfiesTheSeamAndReadyWaiter` | compile-time assertions; `Kind() == KindKata` | a third Kind string, which the phase 2 seam test forbids |
| `TestCreateSendsTheSpecAsTheUser` | one POST; `Authorization: Bearer <fake token>`; body's `machine`, `machineMounts`, `workDir`, `attachToken` match the spec | a spec field dropped on the wire |
| `TestCreateLeavesTheSizeToTheBroker` | a spec with `Resources{CPUMillis: 6000, MemoryMiB: 16384}` → the recorded body has no `resources` or a zero one; the same with `backend.DefaultResources` | the laptop's `PANACEA_APP_SESSION_*` or its 2 CPU / 4 GiB default deciding a cloud session's size, so changing the broker's default, the balloons and the LimitRange together changes no real session (b05 re-review 4) |
| `TestCreateRefusesHostMountsWithoutARequest` | `spec.Mounts` set → `ErrUnsupported`, broker saw zero requests | a laptop path sent to the cluster |
| `TestEveryCallAsksForTheBrokerScope` | the token source was asked for `cfg.Scope` on each call | a Hub token sent to the broker (wrong audience — it would 401 anyway, but the laptop must not even ask for it) |
| `TestEndpointIsOpaqueAndNonEmpty` | Get's `Endpoint == "remote:<id>"` | an empty Endpoint the attach handler reads as "not running" (`attach.go:182`) |
| `TestErrorsMapByCode` | every row of the mapping table, driven by the stand-in | string matching; a quota error shown as "unavailable" |
| `TestMachineFullCarriesTheCount` | a 409 `machine_full` with `details.sessionsPerMachine: "3"` → `errors.Is(err, backend.ErrMachineFull)`, `errors.As` gives `SessionsPerMachine == 3`, and **not** `ErrNotRunning` or `ErrAlreadyExists`; a body with no details → `SessionsPerMachine == 0` (review I5) | a full machine read as the 409s beside it (`conflict`, `machine_asleep`), so the cockpit says "already exists" or "not running" |
| `TestSignedOutMakesNoRequest` | `SignedOut{}` → `ErrUnavailable` wrapping `ErrSignedOut`; zero requests | an unauthenticated request to the broker |
| `TestListReturnsOnlyThisMachinesSandboxes` | two machines' sandboxes in the stand-in's list → only this one's | the reaper on machine A destroying machine B's sandboxes |
| `TestWaitReadyPollsUntilRunning` | starting, starting, running → nil after the third GET | a WaitReady that returns before the pod is up |
| `TestWaitReadyStopsAtStopped` | → `ErrNotRunning` immediately | a full timeout waited on a dead pod |
| `TestAPlainHTTPBrokerIsRefused` | `NewClient` with `http://broker.example` → error | a user token sent in the clear |
| `TestStreamMapsRefusalsAndPassesBodiesThrough` | a 409 `machine_asleep` → `ErrNotRunning`; a 200 streams a 32 MiB body without buffering it (the stand-in writes slowly; the first bytes arrive before the last are written) | a Stream that buffers a git log in memory |
| `TestTheTokenNeverReachesTheLog` | a captured logger across a failing call has no token bytes | a bearer in a log |

`attach_test.go`: `TestAttachSendsBothCredentialsInTheirOwnHeaders`;
`TestAttachCarriesA64KiBFrame` (kills the read limit left at 32 KiB);
`TestAttachRefusalsMapToSentinels` (404/409/403 from the stand-in before upgrade → `ErrNotFound`,
`ErrNotRunning`, `ErrUnauthorized`); `TestAttachRelaysFramesBothWays`.

#### Steps

- [ ] **Step 1:** Check the Hub contract for `entraauth.TokenSource`'s exact signature and its
  signed-out error; if it differs from `Token(ctx, scope) (string, error)`, keep remotebe's
  interface and write the adapter in task 23.
- [ ] **Step 2:** `backend/ready.go`; tests; `remotebe`; all pass under `-race`.
- [ ] **Step 3:** `git diff control-plane/go.mod` empty; full run; commit.

#### Acceptance

```bash
go test ./control-plane/internal/backend/... -race -count=1        # ok, remotebe included
go test ./sandboxd/... ./control-plane/... -race -count=1           # ok
git diff --stat control-plane/go.mod                                 # empty
```

#### What this task does not claim

- That the laptop reaches the broker directly. On the KPMG network outbound HTTPS may need a
  proxy; the transport honours `HTTPS_PROXY` (`http.ProxyFromEnvironment`) and **Go does not read
  the Windows system proxy or a PAC file**. Whether a WebSocket upgrade survives the corporate
  proxy's TLS inspection is **UNVERIFIED** and is an owner hands-on item (OD-12, task 32).
- Anything about cloud machines in the cockpit: nothing constructs a `remotebe` until task 23.
