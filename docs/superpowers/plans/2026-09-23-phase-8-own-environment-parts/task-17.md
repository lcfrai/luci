### Task 17: `cmd/broker` — configuration, listener, authentication, health

The broker is `katabe` moved into the cluster (OQ-1, P8-D4): the only process with Kubernetes
credentials for sandboxes, reached from laptops through the edge, trusting nothing but a valid
realm access token (a Keycloak realm brokering Entra, OD-30). This task builds its skeleton — everything but the sandbox and machine routes (tasks
18, 19) — so that those tasks add handlers to a server that already authenticates, scopes and
shuts down correctly.

It is a new `main` package in the existing module, as the Hub is (ADR-H1 §4.1): it imports
`internal/oidcauth` (step 0), `internal/sandbox`, `internal/backend/katabe`, `internal/httpx` and the
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
- step 0 only: `control-plane/internal/entra/*` -> `control-plane/internal/oidcauth/*` (below)

#### Step 0 (OD-30, 2026-09-25): `internal/entra` becomes `internal/oidcauth`, before any broker code

Users sign in through a Keycloak realm brokering Entra (OD-30; ruling
`docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` §4-§6). Task 16 BUILT
`control-plane/internal/entra` (`669204b` .. `1772f25`) with no importer yet, and its Entra-only rules
(tenant first, `ver`, `iss .../<tid>/v2.0`, GUID `aud`/`azp`, `scp`, `/discovery/v2.0/keys`) refuse
every realm token. Step 0 is **one commit, made before step 1**; hub task 10 takes that commit
unchanged through the orchestrator's merge, never a copy (HA-1: one package, not two).

**Files (step 0):**
- `git mv control-plane/internal/entra control-plane/internal/oidcauth` — all eight files, history kept; `package entra` -> `package oidcauth`; the package comment names both profiles; each error text's `entra:` prefix becomes `oidcauth:` (the `errors.Is` identities are unchanged)
- `oidcauth/entra.go` — `Profile`, `Config`, `Claims`, `ErrType` and `New`'s refusals (below)
- `oidcauth/keycloak.go` (new) — the `keycloak` payload (`aud` as a string or an array) and its checks
- `oidcauth/verify.go` — `Validate` dispatches on the profile; the `entra` path is task 16's, unchanged
- `oidcauth/jwks.go` — the key-set URL and cache key per profile (keycloak: one set, keyed by `Issuer`)
- `oidcauth/middleware.go` — `ErrType` maps to reason `malformed` (hub `codes.go:57-67`, ruling §11); its three problem texts name no IdP: "This request needs an access token." (`middleware.go:68`), "Sign-in cannot be checked right now: the issuer's signing keys are unreachable." (`:107`), "Your organisation's tenant is not enrolled." (`:117`); no test asserts these texts (grep, 2026-09-25)
- `oidcauth/{entra,jwks,middleware,testkey}_test.go` — `Profile: ProfileEntra` in every existing `Config` literal, no assertion changed; a Keycloak token builder; the certs path row
- `oidcauth/keycloak_test.go` (new)

**Contract — `oidcauth` after step 0** (complete for what changes; everything else as task 16 built it):

```go
package oidcauth

type Profile string

const (
	ProfileKeycloak Profile = "keycloak" // a Keycloak realm; PANACEA_APP_OIDC_PROFILE's default
	ProfileEntra    Profile = "entra"    // Entra ID v2 directly: task 16's rules, unchanged
)

type Config struct {
	Profile        Profile  // required; New refuses "" and any other value
	Issuer         string   // keycloak: the exact iss; JWKS at Issuer + "/protocol/openid-connect/certs"
	AuthorityHost  string   // entra only, unchanged: the tenant id is appended
	TenantIDs      []string // both: enrolled tids, GUIDs. keycloak: checked AFTER the signature
	Audience       string   // keycloak: a client-id string aud must CONTAIN. entra: a GUID aud equals
	RequiredScope  string   // keycloak: a field of scope. entra: a field of scp
	AllowedClients []string // person clients (azp). keycloak: strings. entra: GUIDs
	ServiceClients []string // keycloak only: azp values accepted with NO oid and NO tid (ruling §6)

	ClockSkew              time.Duration // default 120 s
	AllowInsecureAuthority bool          // permits an http AuthorityHost or Issuer, loopback host only
	HTTPClient             *http.Client  // default: 10 s timeout
	Now                    func() time.Time
}

// Claims is what a validated token says. It holds no token text.
type Claims struct {
	TenantID          string   // tid; "" on a service token
	ObjectID          string   // oid; "" on a service token
	ClientID          string   // azp
	Name              string   // name
	PreferredUsername string   // preferred_username
	Email             string   // email
	Scopes            []string // entra: scp; keycloak: scope; split on spaces
	Roles             []string // roles (keycloak: the realm roles, mapper P3)
	ExpiresAt         time.Time
	Issuer            string // iss
	Subject           string // sub: logged, never a key
	Service           bool   // keycloak: azp is in ServiceClients
}

// Key is "tid/oid" for a person (unchanged) and "client/" + ClientID for a service.
func (c Claims) Key() string

var ErrType = errors.New("oidcauth: typ is not Bearer") // keycloak only; reason "malformed"
```

`Validator`, `JWKSValidator`, `New`, `Validate`, `Ready`, `Middleware`, `ClaimsFrom`,
`CodeUnauthenticated` and task 16's eleven errors keep their signatures. `ErrVersion` is never
returned under `keycloak`.

**The `keycloak` profile, in order** (the first failure is returned; ruling §5):

| # | Check | Failure |
|---|---|---|
| 1 | size, three segments, `alg` RS256, a `kid`, no `crit` (task 16's `parseToken`) | `ErrMalformed` |
| 2 | RS256 signature against the set at `Issuer + "/protocol/openid-connect/certs"`, cached and refreshed as task 16's | `ErrSignature`; `ErrKeySetUnavailable` if unfetchable |
| 3 | `exp` and `nbf` present and within `ClockSkew` | `ErrMalformed`, `ErrExpired`, `ErrNotYetValid` |
| 4 | `iss == Issuer`, exact | `ErrIssuer` |
| 5 | `typ == "Bearer"` (refuses id and refresh tokens) | `ErrType` |
| 6 | `aud`, a string or an array, contains `Audience` (Keycloak's `account` beside it is ignored) | `ErrAudience` |
| 7 | `azp` in `AllowedClients` (person) or `ServiceClients` (service) | `ErrClient` |
| 8 | the space-separated fields of `scope` contain `RequiredScope` | `ErrScope` |
| 9 | person: `oid` a GUID, `tid` a GUID in `TenantIDs`. Service: `oid` and `tid` both absent, `Service = true` | `ErrMalformed` (oid); `ErrTenant` (tid); a service token carrying either: `ErrClient` |

The tenant is checked after the signature because a realm has one key set, not one per tenant: an
unsigned token must not learn which tenants are enrolled. Row 9's `ErrClient` for a service token that
carries a person is this part's choice (the ruling says only "refused").

`New` refuses, each error naming the field: `Profile` empty or unknown; under `keycloak`, an empty
`Issuer`, one ending in `/`, or a non-https one (http only with `AllowInsecureAuthority` and a loopback
host), and an empty `Audience`; `ServiceClients` set under `entra`; a client id in both lists;
`TenantIDs` empty or holding a non-GUID (both profiles).

**Tests to write first (step 0)** — the whole task 16 suite passes with `Profile: ProfileEntra` added and
no assertion changed (kills any regression in the `entra` profile). Then:

| Test (`keycloak_test.go` unless named) | Asserts | Mutant it kills |
|---|---|---|
| `TestKeycloakAcceptsAGoodPersonToken` | tid, oid, azp, scope, roles, iss, sub come back; `Service` false; `Key() == tid+"/"+oid` | a claim dropped, or a key on `sub` |
| `TestKeycloakRefusesAnotherIssuer` | iss `.../realms/other` -> `ErrIssuer` | a prefix or host-only issuer compare |
| `TestKeycloakRefusesANonBearerTyp` | typ `ID`, and no typ -> `ErrType` | an id token accepted as an access token |
| `TestKeycloakAudienceIsContained` | aud `"panacea-app-broker"` and `["account","panacea-app-broker"]` pass; `["account","panacea-app-hub"]` and none -> `ErrAudience` | an equality compare that refuses arrays; a Hub token replayed at the broker |
| `TestKeycloakRefusesAnUnlistedClient` | azp `panacea-app-other` -> `ErrClient` | the azp check skipped |
| `TestKeycloakScopeIsReadFromScope` | `scope` without `panacea-app-broker` -> `ErrScope`, even with an `scp` that holds it | Entra's `scp` read under keycloak |
| `TestKeycloakPersonNeedsAnOid` | no oid -> `ErrMalformed` | a person keyed `"tid/"` |
| `TestKeycloakTenantAfterSignature` | an unenrolled tid, well signed -> `ErrTenant`; same tid, bad signature -> `ErrSignature` | the entra order copied: a tenant read from an unverified token |
| `TestKeycloakAcceptsAServiceToken` | azp `panacea-app-fleet` in `ServiceClients`, no oid or tid -> `Service`, `Key() == "client/panacea-app-fleet"` | a service token refused, or keyed as a person |
| `TestKeycloakServiceTokenWithAPersonIsRefused` | the same carrying `oid` -> `ErrClient` | a service client presenting a person |
| `TestNewRefusesAnUnusableConfig` (table) | each refusal above errors, naming its field | a validator built on an unusable config |
| `TestProfilesDoNotCross` (`entra_test.go`) | a realm token under `ProfileEntra` is refused; an Entra token under `ProfileKeycloak` is `ErrIssuer` (one key served at both paths) | one profile's rules applied to the other's tokens |
| `TestKeySetPathPerProfile` (`jwks_test.go`) | keycloak fetches `/protocol/openid-connect/certs` under `Issuer`; entra still `<host>/<tid>/discovery/v2.0/keys` | a keycloak validator fetching Entra's path |

#### Contract — configuration (complete; every variable read in `config.go` and nowhere else)

| Variable | Default | Meaning |
|---|---|---|
| `PANACEA_APP_BROKER_ADDR` | `:8080` | listen address (in-cluster) |
| `PANACEA_APP_BROKER_NAMESPACE` | `panacea-sandboxes` | where every pod, Secret and claim lives (P8-D5) |
| `PANACEA_APP_RUNTIME_CLASS` | `kata-vm-isolation` | P8-D15 — the same variable name `cmd/api` reads (task 14) |
| `PANACEA_APP_BROKER_SESSION_PRIORITY_CLASS` | `panacea-session` | task 11 |
| `PANACEA_APP_BROKER_MACHINE_PRIORITY_CLASS` | `panacea-machine` | task 11 |
| `PANACEA_APP_OIDC_ISSUER` | — **required** | the realm URL, `oidcauth.Config.Issuer` (ruling §2: Panacea's dev realm until phase 8, then ours, task 09); under `entra` it is read as `AuthorityHost` |
| `PANACEA_APP_OIDC_PROFILE` | `keycloak` | `oidcauth.Config.Profile`; `keycloak` or `entra` |
| `PANACEA_APP_BROKER_TENANTS` | — **required** | comma-separated enrolled tenant ids (`tid`, GUIDs) |
| `PANACEA_APP_BROKER_AUDIENCE` | `panacea-app-broker` | `oidcauth.Config.Audience`, and `RequiredScope` too: the audience scope carries the same name (ruling §4) |
| `PANACEA_APP_BROKER_ALLOWED_CLIENTS` | `panacea-app-desktop` | comma-separated person clients (`azp`) |
| `PANACEA_APP_BROKER_SERVICE_CLIENTS` | `panacea-app-fleet` | comma-separated service clients (`oidcauth.Config.ServiceClients`, ruling §6) |
| `PANACEA_APP_BROKER_SANDBOX_IMAGE` | — **required** | the only session image, **by digest** |
| `PANACEA_APP_BROKER_MACHINE_IMAGE` | — **required** | the only machine image, by digest |
| `PANACEA_APP_BROKER_SESSION_CPU_MILLIS` | `2000` | a session's size when the caller sends 0, which is **every cloud session**: remotebe never sends a size (task 22, b05 re-review 4), so this is the one place a cloud session's size is set. The same value as the balloon (task 12) and the LimitRange default (task 11); it equals `backend.DefaultResources` (`backend.go:110`) today by choice, not by coupling |
| `PANACEA_APP_BROKER_SESSION_MEMORY_MIB` | `4096` | ditto |
| `PANACEA_APP_BROKER_MAX_CPU_MILLIS` | `6000` | the largest session a caller may request, **derived from the sandbox SKU** (b05 re-review 2): the largest session that fits beside a `500m` machine pod, the DaemonSets and the Kata overhead on an **empty** `Standard_D8s_v5` node, with margin. Allocatable a little under 8 vCPU (P8-D6) less those leaves about 6.5, so `6000` (arithmetic, **UNVERIFIED**). A request above it could fit on no node, and under the affinity it would be refused as `machine_full`, whose advice cannot help. Task 32's P07 measures the real figure and rewrites this value, the LimitRange `max` (task 11) and task 29's `limits.*` together |
| `PANACEA_APP_BROKER_MAX_MEMORY_MIB` | `20480` | ditto for memory: a 32 GiB node's allocatable is below 32 GiB (AKS reserves memory for the kubelet and eviction), less the machine pod's `1Gi`, the DaemonSets and the Kata overhead. `20480` leaves margin (arithmetic, **UNVERIFIED**, P07) |
| `PANACEA_APP_BROKER_MAX_SANDBOXES_PER_USER` | `8` | concurrent session pods per owner |
| `PANACEA_APP_BROKER_MAX_MACHINES_PER_USER` | `2` | machines per owner (task 19) |
| `PANACEA_APP_BROKER_MACHINE_DISK_GIB` | `64` | a new machine's disk (task 19) |
| `PANACEA_APP_BROKER_MACHINE_STORAGE_CLASS` | `panacea-machine-disk` | task 11 |
| `PANACEA_APP_BROKER_MACHINE_AFFINITY` | `true` | `sandbox.Config.MachineAffinity` (task 15); `false` only with RWX storage |
| `PANACEA_APP_BROKER_MACHINE_IDLE_SLEEP` | `30m` | a machine with no session pods for this long is put to sleep (task 19); `0` disables |
| `PANACEA_APP_BROKER_LEASE_NAMESPACE` | — **required** unless `MACHINE_IDLE_SLEEP` is `0` | the namespace of the sleeper's `Lease` `broker-sleeper`: the broker's own, which task 29 sets from the downward API `metadata.namespace` (task 19, review m11) |
| `PANACEA_APP_BROKER_POD_NAME` | — **required** unless `MACHINE_IDLE_SLEEP` is `0` | the Lease holder's identity, one per replica: the downward API `metadata.name` (task 29) |
| `PANACEA_APP_BROKER_READY_TIMEOUT` | `120s` | a pod's time to answer `/healthz` (Kata cold start) |
| `PANACEA_APP_BROKER_UNSCHEDULABLE_GRACE` | `5s` | how long a new session pod may stay `Unschedulable` with no preemption under way before the broker deletes it and answers `409 machine_full` (task 18, review I5); used only when `MACHINE_AFFINITY` is `true` (b05 re-review 3); must be below `READY_TIMEOUT` |
| `PANACEA_APP_BROKER_SHUTDOWN_GRACE` | `20s` | drain time on SIGTERM |
| `PANACEA_APP_LOG_LEVEL` | `info` | as `cmd/api` |

Validation, each failure exiting 2 with one sentence naming the variable: every required value
present; both images match `@sha256:[a-f0-9]{64}$` (a tag is refused — P8-D8, OD-8's pin); the
runtime class a DNS-1123 subdomain; durations parse; limits positive; each `SESSION_*` at or
below its `MAX_*` (else the default session itself is `resources_too_large`); `UNSCHEDULABLE_GRACE`
positive and below `READY_TIMEOUT` (else the timeout answers first and a full machine reads as
`not_ready`); the issuer `https` unless
its host is loopback (a local run against the Hub's devissuer); the profile `keycloak` or `entra`.

```go
package main

// Config is the broker's whole configuration, parsed once.
type Config struct {
	Addr                  string
	Namespace             string
	RuntimeClass          string
	SessionPriorityClass  string
	MachinePriorityClass  string
	OIDC                  oidcauth.Config // Profile, Issuer, TenantIDs, Audience, RequiredScope, AllowedClients, ServiceClients
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
	UnschedulableGrace    time.Duration
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
// sandbox_not_running) and oidcauth.CodeUnauthenticated.
const (
	CodeSandboxNotFound   = "sandbox_not_found"   // 404 — also for another owner's sandbox: existence is not disclosed
	CodeMachineNotFound   = "machine_not_found"   // 404 — likewise
	CodeImageNotAllowed   = "image_not_allowed"   // 400 — a spec named an image other than the configured digest
	CodeResourcesTooLarge = "resources_too_large" // 400 — above PANACEA_APP_BROKER_MAX_*
	CodeQuotaExceeded     = "quota_exceeded"      // 429 — the owner is at MAX_SANDBOXES / MAX_MACHINES
	CodeMachineAsleep     = "machine_asleep"      // 409 — a session create on a sleeping machine; wake it first
	CodeMachineBusy       = "machine_busy"        // 409 — sleep or delete while sessions run
	CodeMachineIsDefault  = "machine_is_default"  // 409 — DELETE of the default machine, whose derived id would recur on an empty disk (task 19, b09 re-review 4)
	CodeMachineFull       = "machine_full"        // 409 — the machine pod's node has no room for this session; details.sessionsPerMachine (task 18, review I5)
	CodeAttachRejected    = "attach_rejected"     // 403 — sandboxd refused the attach token (backend.ErrUnauthorized)
	CodeServiceNeedsDelegation = "service_needs_delegation" // 403 — a service token (Fleet) on an owner-scoped route: it owns nothing until phase T task 17's delegation amendment names the person (OD-30, ruling §6)
)
```

#### Contract — the server skeleton

```go
// newServer wires the routes behind, in order: a recover handler (500 internal,
// the panic logged, never echoed), a request logger (method, route pattern,
// status, duration, owner hash — never a header value), an Origin refusal (any
// request carrying an Origin header gets 403 forbidden_origin: only the control
// plane calls the broker, never a browser — the Hub's rule, ADR-H1 §5.4,
// 67609ab :436-437),
// then oidcauth's Middleware for everything under /v1/.
func newServer(cfg Config, v *oidcauth.JWKSValidator, deps serverDeps) http.Handler

// serverDeps is what handlers need; tasks 18 and 19 add their fields here.
type serverDeps struct {
	Pods    kubernetes.Interface // in-cluster client
	Log     *slog.Logger
	Now     func() time.Time
}

// ownerOf is the only place a request becomes an owner label value.
func ownerOf(r *http.Request) (owner string, c oidcauth.Claims, ok bool)
// = sandbox.OwnerLabelValue(c.TenantID, c.ObjectID) for a person.
// A service token (c.Service) has no owner: ok is false, and every
// owner-scoped route answers 403 service_needs_delegation (never the hash of
// empty ids, which every service call would share).
```

Routes this task registers (tasks 18–19 add theirs through the same table):

| Route | Auth | Answer |
|---|---|---|
| `GET /healthz` | none | `200 {"status":"ok"}` — the process is up |
| `GET /readyz` | none | `200 {"status":"ready"}` when `Validator.Ready` succeeds and a `pods list limit=1` in the namespace succeeds within 1 s; else `503 {"status":"not_ready"}` with no detail |
| `GET /v1/whoami` | realm token | person: `200 {"tenantId":"…","objectId":"…","owner":"<32 hex>"}` — the smoke test's first authenticated call (task 31); service: `200 {"clientId":"panacea-app-fleet","service":true}`, no owner |

`http.Server`: `ReadHeaderTimeout: 10s`, `IdleTimeout: 120s`, no `WriteTimeout` (the attach
route is a long-lived WebSocket), `MaxHeaderBytes: 64 << 10`. SIGTERM → stop accepting, wait up
to `ShutdownGrace` for in-flight requests; open WebSocket relays are closed with status 1001
(going away) so the laptop's `remotebe` reconnects.

main builds the Kubernetes client from `rest.InClusterConfig()` **only** — there is no kubeconfig
variable, by design (S8.2 on the broker side too).

#### Tests to write first

`config_test.go`: `TestEveryRequiredVariableIsNamedWhenMissing` (four required: `OIDC_ISSUER`,
`BROKER_TENANTS`, both images; each missing in
turn names itself — kills a generic "config invalid"); `TestImagesMustBeDigests` (a `:tag` is
refused — kills OD-8's pin defeated by a tag); `TestDefaultsAreTheDocumentedOnes` (every default
in the table — kills drift between this part and the code); `TestAPlaintextIssuerIsRefusedOffLoopback`;
`TestTheScopeIsTheAudience` (`RequiredScope == Audience` after load — kills a broker requiring a
scope the realm never issues);
`TestUnschedulableGraceMustBeBelowTheReadyTimeout` (`UNSCHEDULABLE_GRACE=120s` with the default
`READY_TIMEOUT` exits 2 naming both — kills a full machine reported as `not_ready` after two
minutes, review I5); `TestTheLeaseVariablesAreRequiredOnlyWhenTheSleeperRuns` (with the default
`MACHINE_IDLE_SLEEP`, a missing `LEASE_NAMESPACE` or `POD_NAME` exits 2 naming it; with `0`, neither
is asked for — kills a sleeper that starts with no Lease identity and so runs on both replicas,
review m11).

`server_test.go` (an `httptest.Server`, a fake clientset, an `oidcauth.Validator` under the
`keycloak` profile against a test JWKS server, with step 0's token builder):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestHealthzNeedsNoToken` | 200 with no `Authorization` | a liveness probe that fails because it is unauthenticated |
| `TestReadyzIsNotReadyWithoutKeys` | JWKS server down → 503, body `{"status":"not_ready"}` only | a ready broker that cannot authenticate anyone |
| `TestV1RoutesNeedAToken` | `GET /v1/whoami` with none → 401 + `WWW-Authenticate` | an unauthenticated route under /v1 |
| `TestWhoamiReportsTheOwnerHash` | the owner equals `sandbox.OwnerLabelValue(tid, oid)` | two derivations of the owner that can differ |
| `TestAServiceTokenHasNoOwner` | a `panacea-app-fleet` service token: whoami has no `owner`; `ownerOf` is not ok | every Fleet call owning sandboxes under `OwnerLabelValue("", "")` |
| `TestAHubTokenIsRefused` | a token whose `aud` is `panacea-app-hub` only → 401, reason `audience` | a token for the Hub replayed at the broker |
| `TestARequestWithAnOriginIsRefused` | `Origin: http://localhost:1420` → 403 `forbidden_origin`, even with a valid token | a browser page driving the broker with a stolen token |
| `TestThePanicHandlerNeverEchoes` | a handler that panics with a secret string → 500 `internal`, body without the string | a panic message (with a token in it) sent to the caller |
| `TestTheLogNeverHoldsHeaderValues` | a request with `Authorization` and `Panacea-Attach-Token` → the captured log contains neither value | a token in the log |
| `TestNoRouteIsASideEffectingGet` | every registered `GET` is in {`/healthz`, `/readyz`, `/v1/whoami`, and the read routes tasks 18–19 declare} — the phase 2 audit rule (`internal/api/routes_audit_test.go:133`) applied to the broker | a GET that creates a pod |

#### Steps

- [ ] **Step 0 (OD-30):** `git mv` the package; add `Profile: ProfileEntra` to every existing
  `Config` literal and run task 16's suite green with no assertion changed; write step 0's tests
  (red), then `keycloak.go`, the `verify.go` dispatch, `jwks.go`'s per-profile path and `New`'s
  refusals (green). **Commit alone**, message `feat(oidcauth): task 17 step 0, keycloak profile (OD-30)`,
  before any file of step 1 exists; report the hash, which hub task 10 waits for.
- [ ] **Step 1:** Config tests, then `config.go`.
- [ ] **Step 2:** Server tests, then `server.go` and `main.go`. `go build ./control-plane/cmd/broker`
  produces a binary; running it with no environment exits 2 naming the first required variable.
- [ ] **Step 3:** Full test run; commit.

#### Acceptance

```bash
# step 0
go test ./control-plane/internal/oidcauth/ -race -count=1                  # ok
test ! -e control-plane/internal/entra && echo gone                        # gone
grep -rn '"entra:' control-plane/internal/oidcauth; echo "left=$?"         # left=1 (none)
grep -rln 'control-plane/internal/entra"' control-plane; echo "users=$?"   # users=1 (none)
git log --follow --oneline -- control-plane/internal/oidcauth/verify.go | tail -1   # a task 16 commit
# steps 1-3
go test ./control-plane/cmd/broker/ ./control-plane/internal/brokerapi/ -race -count=1   # ok
go vet ./control-plane/...
go build -o /tmp/broker ./control-plane/cmd/broker && /tmp/broker; echo "exit=$?"
# PANACEA_APP_OIDC_ISSUER is required: …
# exit=2
```

#### What this task does not claim

- That the broker can reach the API server from its pod — needs the chart's Role (task 29) and a
  cluster (task 31).
- Rate limiting per user beyond the concurrency caps: the edge has none either; phase 9.
- That a real realm token passes: step 0's tests mint their own. Panacea's realm carries `oid`,
  `tid` and the `panacea-app-broker` audience only after its K1-K4 update (ruling §10); our realm
  after task 09. The owner's check in ruling §10 is the first real token.
- That Fleet may act for anyone: a service token is authenticated here and owns nothing; the
  delegation that names the person is phase T task 17's amendment.
