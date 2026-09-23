### Task 08: `cmd/hub` — config, `serve`, `migrate`, `verify`; the server chassis; the database pool

**Lane A. Depends on: 01, 06 (and 07 for `migrate.Embedded`). About 3 hours. Contract task H0.**

The binary phase 8's chart runs (`["/hub", "serve"]`, `["/hub", "migrate"]`, phase 8 task 27 `:23`),
with every variable of contract §12.2 read in **one** block, the health routes of §12.4, graceful
shutdown, and the chassis every route task plugs into: request ids, the security headers, the
`Origin` refusal, JSON 404/405, request logging and panic recovery. Authentication is task 13; no
`/v1` route exists yet.

**Files:**
- `control-plane/cmd/hub/main.go` — subcommand dispatch (new)
- `control-plane/cmd/hub/config.go` + `config_test.go` — every `PANACEA_APP_HUB_*` variable (new)
- `control-plane/cmd/hub/serve.go` + `serve_test.go` — listener, signals, shutdown (new)
- `control-plane/cmd/hub/verify.go` — the check runner; checks are added by tasks 20 and 27 (new)
- `control-plane/internal/hub/server/server.go` + `server_test.go` — the chassis (new)
- `control-plane/internal/hub/store/store.go` — the pool (new)
- `control-plane/internal/hub/store/dbauth.go` + `dbauth_test.go` — Entra-only Postgres, standard library (new)

#### Contract — complete

```go
// control-plane/cmd/hub/main.go
package main

// version is set at build time: -ldflags "-X main.version=<git sha>" (phase 8 task 27 :39).
var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr)) }

// run: "serve" | "migrate" | "verify" | "version". Anything else prints usage to
// stderr and returns 2. A configuration error returns 2; a runtime failure 1.
func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int
```

```go
// control-plane/cmd/hub/config.go
package main

// Every variable this binary reads, in one block (contract §12.2). No other
// package calls os.Getenv.
const (
	envEnv            = "PANACEA_APP_HUB_ENV"
	envAddr           = "PANACEA_APP_HUB_ADDR"
	envDatabaseURL    = "PANACEA_APP_HUB_DATABASE_URL"
	envMigrateURL     = "PANACEA_APP_HUB_MIGRATE_DATABASE_URL"
	envDBAuth         = "PANACEA_APP_HUB_DB_AUTH"
	envDBMaxConns     = "PANACEA_APP_HUB_DB_MAX_CONNS"
	envAuthorityHost  = "PANACEA_APP_HUB_ENTRA_AUTHORITY_HOST"
	envTenants        = "PANACEA_APP_HUB_TENANTS"
	envAudience       = "PANACEA_APP_HUB_AUDIENCE"
	envRequiredScope  = "PANACEA_APP_HUB_REQUIRED_SCOPE"
	envAllowedClients = "PANACEA_APP_HUB_ALLOWED_CLIENTS"
	envOrgAdminRole   = "PANACEA_APP_HUB_ORG_ADMIN_ROLE"
	envDevIssuer      = "PANACEA_APP_HUB_DEV_ISSUER"
	envReservationTTL = "PANACEA_APP_HUB_RESERVATION_TTL"
	envBlobMaxBytes   = "PANACEA_APP_HUB_BLOB_MAX_BYTES"
	envProjectQuota   = "PANACEA_APP_HUB_PROJECT_QUOTA_BYTES"
	envTrustProxy     = "PANACEA_APP_HUB_TRUST_PROXY"
	envRateIP         = "PANACEA_APP_HUB_RATE_IP"
	envRateUser       = "PANACEA_APP_HUB_RATE_USER"
	envRateUserWrite  = "PANACEA_APP_HUB_RATE_USER_WRITE"
	envRateOrg        = "PANACEA_APP_HUB_RATE_ORG"
	envRateBlobBytes  = "PANACEA_APP_HUB_RATE_BLOB_BYTES"
	envShutdownGrace  = "PANACEA_APP_HUB_SHUTDOWN_GRACE"
	envLogLevel       = "PANACEA_APP_HUB_LOG_LEVEL"
	// Injected by AKS's workload-identity webhook; read only with DB_AUTH=entra (§12.2).
	envAzureClientID      = "AZURE_CLIENT_ID"
	envAzureTenantID      = "AZURE_TENANT_ID"
	envAzureFederatedFile = "AZURE_FEDERATED_TOKEN_FILE"
	envAzureAuthorityHost = "AZURE_AUTHORITY_HOST"
)

type Tenant struct{ ID, Name string } // ID: a GUID, lowercased

type Rate struct {
	PerSecond float64
	Burst     int
}

type Config struct {
	Env            string // "local" | "dev" | "prod"
	Addr           string
	DatabaseURL    string
	MigrateURL     string
	DBAuth         string // "none" | "entra"
	DBMaxConns     int32
	AuthorityHost  string
	Tenants        []Tenant
	Audience       string
	RequiredScope  string
	AllowedClients []string
	OrgAdminRole   string
	DevIssuer      bool
	ReservationTTL time.Duration
	BlobMaxBytes   int64
	ProjectQuota   int64
	TrustProxy     bool
	RateIP, RateUser, RateUserWrite, RateOrg Rate
	RateBlobBytes  int64 // per user per minute
	ShutdownGrace  time.Duration
	LogLevel       slog.Level
	Workload       store.WorkloadIdentity // DB_AUTH=entra only
}

// loadConfig reads and validates for one subcommand. Every default is §12.2's.
// The rules, each a test in config_test.go:
//   - ENV ∈ {local, dev, prod}.
//   - ADDR's host is loopback unless ENV is dev or prod.
//   - DEV_ISSUER=true only when ENV=local.
//   - an http:// AUTHORITY_HOST only with DEV_ISSUER=true, and only a loopback host.
//   - serve: TENANTS (≥ 1 "<guid>=<name>", name 1–200 chars), AUDIENCE (a GUID)
//     and ALLOWED_CLIENTS (≥ 1 GUID) are required. migrate and verify need none of them.
//   - RATE_* are "<per second>:<burst>", both positive.
//   - RESERVATION_TTL in [1m, 24h]; BLOB_MAX_BYTES in [1, 16777216] (the table's CHECK);
//     PROJECT_QUOTA_BYTES ≥ BLOB_MAX_BYTES; DB_MAX_CONNS in [1, 100]; SHUTDOWN_GRACE in [1s, 5m].
//   - DB_AUTH=entra needs ENV dev or prod and all four AZURE_* variables.
func loadConfig(subcommand string, getenv func(string) string) (Config, error)
```

```go
// control-plane/internal/hub/store/store.go
package store

// Store is the Hub's Postgres access: one pool as hub_app. Aggregates add
// methods in their own files (teams.go, briefs.go, …).
type Store struct{ Pool *pgxpool.Pool }

type Options struct {
	URL      string
	MaxConns int32
	DBAuth   string // "none" | "entra"
	Workload WorkloadIdentity
	Log      *slog.Logger
}

// Open builds the pool: RuntimeParams search_path=hub; MaxConns; MaxConnLifetime
// 45 min (ADR-H1 R8, below an Entra token's life); with DBAuth "entra" a
// BeforeConnect hook sets each NEW connection's password from EntraPassword.
// It does not ping; /readyz does.
func Open(ctx context.Context, o Options) (*Store, error)

func (s *Store) Ping(ctx context.Context) error            // SELECT 1
func (s *Store) SchemaVersion(ctx context.Context) (int, error) // migrate.Current
func (s *Store) Close()
```

```go
// control-plane/internal/hub/store/dbauth.go — the ~70-line exchange the contract names (§12.2).
package store

// WorkloadIdentity is what AKS's webhook injects.
type WorkloadIdentity struct {
	ClientID, TenantID, FederatedTokenFile, AuthorityHost string
}

// DatabaseScope is Azure Database for PostgreSQL's Entra resource (contract §12.2).
const DatabaseScope = "https://ossrdbms-aad.database.windows.net/.default"

// EntraPassword returns a token source for new connections: it reads the
// federated token file afresh each time (the webhook rotates it), POSTs
// grant_type=client_credentials, client_id, scope=DatabaseScope,
// client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer and
// client_assertion=<file> to <AuthorityHost><TenantID>/oauth2/v2.0/token, and caches
// the access token until 5 minutes before expires_in. It never logs the token
// or the assertion. Standard library only: no Azure SDK.
func EntraPassword(w WorkloadIdentity, hc *http.Client, now func() time.Time) func(ctx context.Context) (string, error)
```

```go
// control-plane/internal/hub/server/server.go
package server

// Readiness is what /readyz asks. DB is the store; Extra are named checks
// later tasks add (task 13: "jwks").
type Readiness struct {
	DB interface {
		Ping(ctx context.Context) error
		SchemaVersion(ctx context.Context) (int, error)
	}
	WantSchema int
	Extra      map[string]func(ctx context.Context) error
}

type Config struct {
	Version   string
	Log       *slog.Logger
	Readiness Readiness
}

// Server is the Hub's HTTP surface. Route tasks add a register method in their
// own file and one line in New, exactly as control-plane/internal/api does.
type Server struct{ /* unexported: cfg, mux, pathMux, methodsByPath, shuttingDown atomic.Bool */ }

func New(cfg Config) (*Server, error)

// Handler is the whole surface with its middleware, outermost first:
// recover → request id → security headers → Origin refusal → access log → routes.
func (s *Server) Handler() http.Handler

// BeginShutdown makes /readyz answer 503 {"failing":["shutting_down"]} at once.
func (s *Server) BeginShutdown()

// handle registers "METHOD /path" on the mux and records the path for the 405 rule.
func (s *Server) handle(pattern string, h http.HandlerFunc)

// Patterns lists every registered "METHOD /path", in registration order. Task 16's
// matrix meta-test and task 34's conformance harness read it, so neither keeps a
// hand-maintained list of routes.
func (s *Server) Patterns() []string

// WriteJSON is every success body: hubapi.EnsureLists, then v encoded into a
// buffer by json.NewEncoder with SetEscapeHTML(false) — NOT httpx.WriteJSON,
// whose default encoder writes "<", ">", "&", U+2028 and U+2029 as six-byte
// escapes even inside a json.RawMessage, so a revision's content would reach the
// wire up to six times its canonical size and the 8 MiB reply cap would not be a
// bound (task 00, the wire rule). It keeps httpx.WriteJSON's other behaviour:
// encode before touching w, and an encode failure is the constant 500 body;
// Content-Type "application/json; charset=utf-8" and nosniff on every body.
// httpx itself is unchanged: it is shared with the control plane.
func WriteJSON(w http.ResponseWriter, status int, v any)

// WriteProblem is httpx.WriteProblem, and records the code for the access log.
func WriteProblem(w http.ResponseWriter, status int, code, msg string, details map[string]string)

// WriteUnauthenticated is the 401 of contract §4.1: WWW-Authenticate is
// `Bearer error="insufficient_scope"` when reason is hubapi.ReasonScope,
// `Bearer` when it is hubapi.ReasonMissing, else `Bearer error="invalid_token"`.
func WriteUnauthenticated(w http.ResponseWriter, reason string)

// WriteRateLimited is the 429 of contract §11.3, with Retry-After in whole seconds, rounded up.
func WriteRateLimited(w http.ResponseWriter, scope string, wait time.Duration)
```

**The chassis rules, each tested:**
- **Request id.** A fresh `ids.UUID()` per request, in the context and in `Request-Id`.
- **Headers on every response.** `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`
  (contract §1). **No** `Access-Control-*` header, ever (§11.4).
- **Origin.** Any `Origin` header → 403 `forbidden_origin`, before routing (§5.1 step 1).
- **404 and 405 as problems.** Go's `ServeMux` writes plain-text 404/405. The chassis keeps a
  second mux, `pathMux`, holding every registered **path** without its method. A request whose
  method pattern matches nothing is looked up in `pathMux`: no match → 404 `route_not_found`; a
  match → 405 `method_not_allowed` with `Allow` listing the recorded methods for that path.
  `OPTIONS` is never registered, so it is always 405 (§11.4).
- **Access log.** One `slog` JSON line per request: `requestId`, the **route pattern** (never the
  raw path), `status`, `durationMs`, `orgId` and `userId` when task 13 has set them, and the problem
  code. Never a header, a body or a query (§11.5).
- **Recover.** A panic → 500 `internal`, the stack logged under the request id.
- **Health.** `GET /healthz` → `200 {"status":"ok","version":…}`. `GET /readyz` → `200
  {"status":"ready"}` when the database answers `SELECT 1` within 1 s, `SchemaVersion ==
  WantSchema`, every `Extra` check passes and shutdown has not begun; else `503
  {"status":"not_ready","failing":[…]}` naming `database`, `schema`, each failing extra, or
  `shutting_down`. Neither route authenticates, logs at `info`, or names a host, tenant or error.

**`serve`**: load config; open the store; `server.New` with `WantSchema: migrate.Latest()`; listen
on `ADDR`; on `SIGTERM`/`SIGINT`, `BeginShutdown`, then `http.Server.Shutdown` with
`SHUTDOWN_GRACE`. **`migrate`**: open one `pgx.Conn` on `MIGRATE_DATABASE_URL` (with the Entra
password when `DB_AUTH=entra`), `migrate.Apply(…, migrate.Embedded(), …)`, print the applied
versions, exit 0. **`verify`**: connect on `DATABASE_URL` and run each registered check in order,
printing `ok <name>` or `FAULT <name>: <what>` and exiting 1 at the first fault; with no checks
registered it prints `verify: no checks registered` and exits 0 (tasks 20 and 27 register them).

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestConfigDefaultsAreTheContracts` | with only `TENANTS`, `AUDIENCE`, `ALLOWED_CLIENTS` set, every field equals §12.2's default (`127.0.0.1:8750`, `Hub.Access`, `15m`, `20:40`, `5:20`, …) | a default drifted from the table phase 8's chart reads |
| `TestConfigRefusals` | one row per rule in `loadConfig`'s comment → an error naming the variable | each rule's deletion; notably `DEV_ISSUER` outside `local`, and a public `ADDR` in `local` |
| `TestMigrateNeedsNoAuthVariables` | `loadConfig("migrate", …)` with only the migrate URL → ok | the migrate Job failing for want of a tenant list |
| `TestHealthz` | 200, `{"status":"ok","version":"test"}`, `Cache-Control: no-store` | — |
| `TestReadyzNamesEachFailure` | a DB whose ping fails → `failing:["database"]`; version 0 vs 1 → `["schema"]`; an extra `jwks` failing → `["jwks"]`; all good → 200 `ready`; the body never contains an error string | a readiness that says ready before the schema exists |
| `TestShutdownFlipsReadyzFirst` | `BeginShutdown`; `/readyz` → 503 `shutting_down` while `/healthz` stays 200 | a pod that keeps receiving traffic while draining |
| `TestUnknownPathIs404Problem` / `TestWrongMethodIs405ProblemWithAllow` | `GET /nope` → 404 `route_not_found` JSON; with `GET /x` registered, `POST /x` → 405 `method_not_allowed`, `Allow: GET` | Go's plain-text 404/405 reaching a client that parses JSON |
| `TestOptionsIs405` | `OPTIONS /healthz` → 405 | a preflight answered, i.e. CORS by accident |
| `TestOriginIsRefusedBeforeRouting` | `Origin: http://tauri.localhost` on `/healthz` and on `/nope` → 403 `forbidden_origin` | a browser context reaching any route (§11.4) |
| `TestNoCORSHeaderEver` | across every test response above, no header starts `Access-Control-` | an allowlist added later "just for dev" |
| `TestRequestIDOnEveryResponse` | `Request-Id` is a v4 UUID and differs per request | a bug report nobody can find in the log |
| `TestAccessLogShape` | the log line for `GET /readyz?x=secret` has `route: "GET /readyz"` and no `secret` | a query string (with a token someone pasted) in the log |
| `TestPanicIs500Internal` | a registered handler that panics → 500 `internal`, the process lives | one bad request killing a replica |
| `TestSuccessBodiesAreNotHTMLEscaped` | `WriteJSON` of `struct{C json.RawMessage}` holding `{"t":"a<b&&c>d"}` plus a raw U+2028 → the body contains those bytes exactly, and no `<`, `&` or ` `; `Content-Type: application/json; charset=utf-8` and `nosniff` set; a value that fails to encode → the constant 500 body | `WriteJSON` calling `httpx.WriteJSON` or `json.Marshal`, which makes a `<`-dense 64-revision reply up to six times its canonical size and past `hubclient`'s 8 MiB cap (task 00, the wire rule) |
| `TestEntraPasswordExchangeAndCache` | against an `httptest` token endpoint: the form fields exactly as above; the assertion is the file's current contents; a second call within the lifetime makes no request; the token never appears in a log line | a stale assertion after rotation; a token per connection |
| `TestRunDispatch` | `run(["nope"])` → 2 with usage; `run(["version"])` prints `dev` → 0 | — |

#### Steps

- [ ] **Step 1:** Re-read contract §1, §11.4, §11.5, §12.1–§12.4; `control-plane/cmd/api/config.go`
  for the house pattern (one block, `getenv` injected).
- [ ] **Step 2:** Tests; they fail.
- [ ] **Step 3:** `config.go`, `store.go`, `dbauth.go`, `server.go`, `serve.go`, `verify.go`,
  `main.go`. Pass under `-race`.
- [ ] **Step 4:** By hand, against the task-06 container:
  `PANACEA_APP_HUB_MIGRATE_DATABASE_URL=postgres://hub_owner@127.0.0.1:5433/hub?sslmode=disable go run ./control-plane/cmd/hub migrate`
  prints `applied [1]`, and a second run prints `applied []`.
- [ ] **Step 5:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/cmd/hub/ ./control-plane/internal/hub/server/ ./control-plane/internal/hub/store/ -race -count=1   # ok
go vet ./control-plane/cmd/hub/ ./control-plane/internal/hub/...                                                          # no output
go build -o /tmp/hub ./control-plane/cmd/hub && /tmp/hub version                                                           # dev
/tmp/hub nope; echo $?                                                                                                     # usage, then 2
```

#### What this task does not claim

- That the Entra-only Postgres exchange works against Azure: it is tested against a local
  endpoint only (contract §15, *"described, not prototyped"*). Phase 8's first apply checks it.
- Any authenticated route: task 13.
- The import boundary: task 09 writes that test once `devissuer` also exists to be excluded.
