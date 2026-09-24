### Task 05: The connection — a write-only token, an address, and an honest status

**Half:** 7a. **Hours:** 2. **Depends on:** 02, 03. **Elsewhere:** Hub task C1's `credstore`
(or the three-method subset below, if C1 has not landed).

Plan decision 7P-1: the control plane authenticates to Panacea with the user's own `pnc_` token,
kept in Windows Credential Manager, pasted once. This task is the paste, the store, and the status
the lens draws. **The token enters through one route and never leaves the control plane:** no
response carries it, no log line prints it, it never reaches the Hub or a sandbox.

**Files:**
- `control-plane/internal/deploy/connection/connection.go` (new) — `Secrets`, `Config`, `Manager`, `Status`, `State`
- `control-plane/internal/deploy/connection/current.go` (new) — `Current`, the delegating `deploy.Deployer`
- `control-plane/internal/deploy/connection/connection_test.go` (new)
- `control-plane/internal/api/deploy_connection.go` (new) — the four routes
- `control-plane/internal/api/deploy_errors.go` (new) — `writeDeployError`, the one mapping from `deploy.Error` and `hubclient.Error` to problems
- `control-plane/internal/api/deploy_connection_test.go` (new)
- `control-plane/internal/api/api.go` (modified) — `Deps.DeployConnection`; `a.registerDeployConnection(guarded)` in `New`
- `control-plane/internal/api/routes_audit_test.go` (modified) — one `catalogReadOnlyRoutes` line
- `control-plane/internal/httpx/problem.go` (modified) — the codes below
- `control-plane/internal/httpx/origin.go` (modified) — `PUT` in `corsMethods`, `Idempotency-Key` in `corsHeaders`, and the comment above them (review C1; a declared amendment to phase 2 and Hub C3, below)
- `control-plane/internal/httpx/origin_test.go` (modified) — the exact-value row updated; two new preflight rows
- `control-plane/cmd/api/config.go`, `config_test.go`, `main.go` (modified) — three variables, the wiring

#### Interfaces — complete

```go
// Package connection holds the signed-in user's own Panacea credential and address, and
// reports honestly whether they work. It is the only writer of the credential store's
// PanaceaApp/panacea/* rows (Hub contract §9.1).
package connection

// Secrets is the subset of credstore.Secrets (Hub contract §9.1) this package uses. It is
// declared here so phase 7 compiles before Hub task C1 lands; credstore.Secrets satisfies it.
type Secrets interface {
	Get(target string) ([]byte, error)
	Put(target, userName string, blob []byte) error
	Delete(target string) error
}

// Targets. The token row is the one the Hub contract reserves; the origin row is non-secret
// and holds {"origin","savedAt"} as JSON.
const (
	targetOrigin    = "PanaceaApp/panacea/current"
	targetTokenBase = "PanaceaApp/panacea/" // + lowercased host
)

type State string

const (
	NotConnected     State = "not_connected"     // no address, or no token
	Connected        State = "connected"         // the token works and the account is approved
	AwaitingApproval State = "awaiting_approval" // the token works; Panacea says approved is not true
	TokenRejected    State = "token_rejected"    // Panacea answered 401 to this token
	Unreachable      State = "unreachable"       // the last check could not reach Panacea
)

type Status struct {
	State        State
	Origin       string // "" when none
	OriginLocked bool   // PANACEA_APP_PANACEA_URL is set: the connect form shows it read-only
	APIVersion   string // from GET /api/v1/version; "" when never reached
	Account      *deploy.Account
	Message      string    // Panacea's sentence for TokenRejected; "" otherwise
	CheckedAt    time.Time // zero = never checked
}

type Config struct {
	Secrets    Secrets
	IsNotFound func(error) bool // errors.Is(err, credstore.ErrNotFound), wired in main.go
	// LockedOrigin is PANACEA_APP_PANACEA_URL after ParseOrigin; "" = the user chooses.
	LockedOrigin string
	CAFile       string
	Timeout      time.Duration
	UserAgent    string
	Now          func() time.Time
	Log          *slog.Logger
	// NewClient builds a Deployer for an origin. main.go passes a closure over panacea.New;
	// tests pass one over deploytest.Fake or panaceatest.Server.
	NewClient func(origin string, tokens panacea.TokenSource) (deploy.Deployer, error)
	// Version is panacea.Version, injectable for tests.
	Version func(ctx context.Context, origin string) (string, error)
}

type Manager struct{ /* mu, cfg, status, current *Current */ }

// New loads the stored origin (or the locked one) and builds the current Deployer. It makes
// NO network call: startup never blocks on Panacea. Status starts as NotConnected or, when a
// token is stored, as Connected-unchecked (CheckedAt zero) until Check runs.
func New(cfg Config) (*Manager, error)

// PanaceaToken implements panacea.TokenSource. deploy.ErrNotConnected when none is stored.
func (m *Manager) PanaceaToken(ctx context.Context) (string, error)

// Current is the Deployer every other phase 7 package holds. It is stable for the process's
// life; Connect and Disconnect swap what it delegates to.
func (m *Manager) Current() *Current

// Connect proves origin and token before storing either:
//   1. ParseOrigin (or the locked origin; an origin that differs from a locked one is refused).
//   2. Version(origin) — KindRejected (incl. ErrSignInRedirect) or KindUnavailable stop here.
//   3. Whoami with the candidate token, through a throwaway client — 401 stops here.
//   4. Put the token row, then the origin row (the origin is written LAST, so a crash between
//      the two leaves no origin pointing at a missing token).
//   5. Swap Current; Status becomes Connected or AwaitingApproval.
// Nothing is stored unless step 3 answered 200.
func (m *Manager) Connect(ctx context.Context, origin, token string) (Status, error)

// Disconnect deletes the token row (absent is not an error) and swaps Current to not-connected.
// The origin row is kept, so reconnecting asks only for a token.
func (m *Manager) Disconnect(ctx context.Context) error

// Check re-runs Version and Whoami with the stored token and updates Status. Called at start
// (in a goroutine), by POST …/check, and by the watcher (task 07) after it observes
// KindUnauthorized or recovers from KindUnavailable.
func (m *Manager) Check(ctx context.Context) Status

// Observe lets the watcher report what it saw without a second request: KindUnauthorized sets
// TokenRejected with the error's Message; KindUnavailable sets Unreachable; nil after either
// schedules a Check.
func (m *Manager) Observe(err error)

func (m *Manager) Status() Status
```

```go
// Current is a deploy.Deployer that delegates to whatever client Connect installed, or returns
// deploy.ErrNotConnected from every method when none is.
type Current struct{ /* atomic.Pointer[deploy.Deployer] */ }

var _ deploy.Deployer = (*Current)(nil)
```

#### Control-plane routes — complete wire

Canonical `/v1/…`; `/api/v1/…` via the existing alias (`api.go:292`).

```
GET /v1/deploy/connection
  200 {
    "state": "not_connected" | "connected" | "awaiting_approval" | "token_rejected" | "unreachable",
    "origin": "https://dev.panacea.kpmg" | null,
    "originLocked": false,
    "apiVersion": "v1.163.0" | null,
    "account": { "username": "dmackie1", "email": "dmackie1@kpmg.com.au", "admin": false } | null,
    "message": "" ,
    "checkedAt": "2026-09-23T10:02:11.123Z" | null
  }
  — never contains the token, a prefix of it, or its length.

PUT /v1/deploy/connection
  request { "origin": "https://dev.panacea.kpmg", "token": "pnc_…" }   ("origin" optional when originLocked)
  200 the GET body, after the checks
  400 bad_request               — "token" missing or empty; "origin" missing while not locked;
                                  a token not starting "pnc_": "A Panacea API key starts with pnc_. Copy it from the portal: Account → API keys."
  422 deploy_rejected           — ParseOrigin's sentence, or "That address answered, but not as the Panacea API.", or ErrSignInRedirect's
  409 deploy_token_rejected     — Panacea's 401 sentence; nothing was stored
  503 upstream_unavailable      — details.upstream "panacea"; nothing was stored

POST /v1/deploy/connection/check
  request none → 200 the GET body

DELETE /v1/deploy/connection
  204
```

`PUT` is `http.MaxBytesReader`-bounded by `httpx.DecodeJSON` (1 MiB) and decoded into a struct
whose `Token` field has a `String()` that redacts, so a `%v` in any later log line cannot print it.

#### The preflight must allow `PUT` and `Idempotency-Key` (review C1; a declared amendment)

This task sends phase 7's first `PUT`. Every Tauri build is cross-origin to `127.0.0.1:8730`
(phase 2 task 09, decision 1), so the webview preflights every `PUT` and every request that
carries `Idempotency-Key`. `origin.go:231-237` answers that preflight from two constants
(`origin.go:168-172`), and today they list neither: `corsMethods = "GET, POST, PATCH, DELETE,
OPTIONS"`, `corsHeaders = "Content-Type"`. Unchanged, they refuse every one of these at the
preflight, before the request is sent:
- Connect, from this task (`PUT /v1/deploy/connection`)
- Bind (`PUT …/deploy/binding`, task 06)
- the three governance `PUT`s (task 11)
- Ship (task 09), and Rollback and Rebuild (task 10), which send `Idempotency-Key`

The Go tests call handlers directly and the vitest tests mock `fetch`, so without the rows below
the whole plan stays green while the lens cannot connect or ship.

```go
// control-plane/internal/httpx/origin.go — the constants after this task.
// If Hub task 37 (C3) has already landed, corsHeaders is "Content-Type, Authorization" today:
	corsMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsHeaders = "Content-Type, Authorization, Idempotency-Key"
// If it has not:
	corsMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsHeaders = "Content-Type, Idempotency-Key"
```

Rewrite the comment above them (`origin.go:164-167`). It says the desktop *"sends no custom
header"*, and after this task that is false. The new comment says:
- Methods is every verb the control plane routes, `PUT` included (phase 7's writes).
- Headers is `Content-Type`, plus `Idempotency-Key`, which ship, rollback and rebuild send
  (tasks 09 and 10), plus `Authorization` once Hub C3's local key is in.

`corsMaxAge`, the origin check, the `Vary` header and the missing
`Access-Control-Allow-Credentials` stay exactly as they are.

**Declared amendment to phase 2.** Phase 2 fixed both values:
- task 09 (`phase-2-parts/task-09.md:5`, `:642-643`, `:952-953`)
- task 24's live check, which expects them (`phase-2-parts/task-24.md:1299`)

Phase 7 widens both lists and narrows nothing. Phase 2's exact-value test row
(`origin_test.go:171-172`) is updated to the new values in this task.

**Declared amendment to Hub C3.** Hub task 37 writes both constants as literals
(`phase-h-hub-parts/task-37.md:154-159`: methods without `PUT`, headers `"Content-Type,
Authorization"`). Whichever of the two tasks lands second **adds its entry to the list it
finds**. It must not paste its own literal. Pasting compiles, and it silently drops the other
stream's entry (the silent-override class). If that happens, the two membership rows below go
red, whichever order the streams land in.

#### Problem codes — added to `httpx`'s constant block (`problem.go:45-60`)

```go
	CodeUpstreamUnavailable      = "upstream_unavailable"       // 503 a remote the control plane depends on is unreachable; details.upstream (ruling R4) — add ONLY if phase 5 or Hub C2 has not
	CodeDeployNotConnected       = "deploy_not_connected"       // 409 no Panacea address or token is stored
	CodeDeployTokenRejected      = "deploy_token_rejected"      // 409 Panacea answered 401 to the stored token
	CodeDeployForbidden          = "deploy_forbidden"           // 403 Panacea refused the act for this user
	CodeDeployNotFound           = "deploy_not_found"           // 404 Panacea has no such project, run or deployment
	CodeDeployConflict           = "deploy_conflict"            // 409 Panacea refused because of its current state
	CodeDeployRejected           = "deploy_rejected"            // 422 Panacea refused the request, or the address is not Panacea
	CodeDeployNotBound           = "deploy_not_bound"           // 409 the local project has no Panacea project (task 06)
	CodeDeployEnvironmentInvalid = "deploy_environment_invalid" // 409 Panacea's environment name for the branch breaks the Hub's §7.9 rule (task 09 step 4; review b02 replaced deploy_environment_unknown)
	CodeDeployEnvironmentUnreadable = "deploy_environment_unreadable" // 409 Panacea did not name the branch's environment: EnvironmentFor answered KindNotFound or KindRejected, e.Message verbatim (task 09 step 4; b02 round 2, R2 — never deploy_not_found, which would blame a project that exists)
	CodeDeployPrecondition       = "deploy_precondition"        // 409 Ship's local precondition failed; details.reason (task 09)
```

Why `409` and not `401` for a rejected Panacea token: the desktop's request to the control plane
was fine. A `401` on this surface already means *"the local caller key is wrong"* (Hub contract
§9.4, `local_key_required`) or *"sign in to the Hub"* (`signed_out`); a third meaning would make the
status useless to the desktop, which switches on the **code** anyway (`problems.ts:37-41`).

#### `writeDeployError` — the one mapping (`deploy_errors.go`)

```go
// writeDeployError answers err as a problem. It is the ONLY place a deploy.Error or a
// hubclient.Error becomes a status code, so the table below is testable in one file.
//
//   deploy.KindNotConnected → 409 deploy_not_connected, "Connect to Panacea first."
//   deploy.KindUnavailable  → 503 upstream_unavailable, details {"upstream":"panacea"},
//                             "Panacea could not be reached, so nothing was changed." (+ "retryAfter" when set)
//   deploy.KindUnauthorized → 409 deploy_token_rejected, e.Message
//   deploy.KindForbidden    → 403 deploy_forbidden,      e.Message
//   deploy.KindNotFound     → 404 deploy_not_found,      e.Message
//   deploy.KindConflict     → 409 deploy_conflict,       e.Message
//   deploy.KindRejected     → 422 deploy_rejected,       e.Message
//   *hubclient.Error        → the Hub's status and problem, UNCHANGED (Hub contract §4.3)
//   hubclient.ErrUnavailable→ 503 upstream_unavailable, details {"upstream":"hub"}
//   entraauth.ErrSignedOut  → 401 signed_out (the Hub's C1 code)
//   anything else           → a.internal(…)
// Every deploy_* problem carries details {"upstream":"panacea","status":"<upstream status>"}.
func (a *API) writeDeployError(w http.ResponseWriter, r *http.Request, op string, err error)
```

The Hub rows compile only once Hub C2's `hubclient` exists; until then they are behind the
`hubclient` interface's fake, which already returns `*hubclient.Error` (Hub contract §9.2).

#### Configuration (`config.go`)

| Variable | Parse | Error, at start |
|---|---|---|
| `PANACEA_APP_PANACEA_URL` | `panacea.ParseOrigin` | its sentence, prefixed with the variable's name |
| `PANACEA_APP_PANACEA_CA_FILE` | `os.Stat` + one PEM block parses | *"PANACEA_APP_PANACEA_CA_FILE: no certificate in <path>"* |
| `PANACEA_APP_PANACEA_TIMEOUT` | `parseDuration` (`config.go:317`) | the existing helper's sentence |

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestConnectStoresNothingOn401` | a server answering `/me` 401 → `Connect` returns `KindUnauthorized`; `Secrets` holds no row | storing a token that does not work, so the next start shows "connected" |
| `TestConnectStoresNothingOnBadOrigin` | `Version` fails → no row, and `Whoami` was never called (the token never left) | sending the token to an address that is not Panacea — a paste into the wrong field would leak it |
| `TestOriginWrittenAfterToken` | a `Secrets` fake that fails the second `Put` → no origin row exists | an origin pointing at a missing token |
| `TestLockedOriginRefusesAnother` | `LockedOrigin` set, `Connect` with a different origin → 400 | a user override of an administrator's address |
| `TestAwaitingApproval` | `/me` with `approved: false` → token stored, `State == AwaitingApproval` | treating a pending account as connected |
| `TestNewMakesNoNetworkCall` | `New` with a stored token against a server that fails the test on any request | a startup that hangs on Panacea |
| `TestCurrentWhenDisconnected` | every method of `Current` that can fail (all 24 but `SameEnvironment`, which answers exact equality when disconnected; review b02 added `EnvironmentFor` to the 24; by reflection over `deploy.Deployer`) returns `deploy.ErrNotConnected` | one delegating method forgotten |
| `TestObserveUnauthorized` | `Observe(&deploy.Error{Kind: KindUnauthorized, Message: "api key revoked or expired"})` → status `TokenRejected` with that message | the lens keeping "connected" while every call 401s |
| `TestConnectionGETNeverHasToken` | after `PUT` with `pnc_abcdef`, the `GET` body contains neither `pnc_` nor `abcdef` | an `Account`-style struct growing a token field |
| `TestPutRejectsNonPncToken` | `"token": "eyJhbGciOi…"` → 400 with the sentence; nothing stored | storing a pasted JWT |
| `TestWriteDeployErrorTable` | one sub-test per row of `writeDeployError` | a 401 on this surface for a Panacea token |
| `TestConfigPanaceaURL` | `https://dev.panacea.kpmg/api` exits with the path sentence; `http://…` exits | the doubled `/api` (measured 404) |
| `TestRoutesAudit` | the existing §10a audit passes with `GET /v1/deploy/connection` in `catalogReadOnlyRoutes` | — (the audit is phase 2's) |
| `TestPreflightAllowsPut` (`origin_test.go`) | `OPTIONS` from `http://tauri.localhost` with `Access-Control-Request-Method: PUT` → 204, and `PUT` is one of the comma-separated entries of `Access-Control-Allow-Methods` | `corsMethods` unchanged: every phase 7 `PUT` is refused at the preflight |
| `TestPreflightAllowsIdempotencyKey` (`origin_test.go`) | `OPTIONS` with `Access-Control-Request-Method: POST` and `Access-Control-Request-Headers: content-type,idempotency-key` (the browser's lower-cased form) → 204, and `Idempotency-Key` is one of the entries of `Access-Control-Allow-Headers`, compared case-insensitively | `corsHeaders` unchanged: Ship, Rollback and Rebuild are refused at the preflight |
| `TestMiddlewareAnswersPreflightBeforeTheMux` (existing, `origin_test.go:151-182`) | its rows at `:171-172` expect the new constants exactly | — (phase 2's row; it stays exact, so a verb or header nobody asked for is still a diff) |

`catalogReadOnlyRoutes` line: `"GET /v1/deploy/connection": "Returns Manager.Status(), a cached
struct; it performs no network call and writes nothing. The token is never in it."`

The two new preflight rows test **membership, not string equality**, on purpose. Hub C3 adds
`Authorization` to the same constant. A row that pinned the whole string would be rewritten by
whichever stream lands second, and that rewrite would carry the other stream's entry away with it.
A membership row does not change when Hub C3 lands, so it keeps `PUT` and `Idempotency-Key`
guarded. **Mutants** (on a `$TEMP` copy): restore `corsMethods` to phase 2's value, and
`TestPreflightAllowsPut` goes red; restore `corsHeaders`, and `TestPreflightAllowsIdempotencyKey`
goes red.

#### Steps

- [ ] Tests first; they fail to compile.
- [ ] The two preflight rows first, against today's `origin.go`: both go red (`PUT` and
  `Idempotency-Key` are missing). Then change the two constants and their comment (above) and the
  exact row at `origin_test.go:171-172`; all three go green. Commit this change by itself, so that
  the Hub stream can see the amendment as one diff.
- [ ] `connection.go`, `current.go`; then the codes in `problem.go`; then `deploy_errors.go` and
  `deploy_connection.go`; register through `routes`; one `Deps` field `DeployConnection
  *connection.Manager` — **nil disables every phase 7 route with 503 `deploy_not_connected`**
  rather than failing `api.New`, so phase 2's wiring tests are unchanged.
- [ ] `config.go` and `main.go`: `connection.New` with `credstore.WinCred()` on Windows and
  `credstore.Memory()` elsewhere (Hub contract §9.1, `PANACEA_APP_CREDSTORE`); `go Check(ctx)`
  after the listener is up.
- [ ] Run the control-plane suite; commit by pathspec.

#### Acceptance

```
go test ./control-plane/... -race -count=1
→ ok, including internal/api's routes audit

grep -rnE '["`]pnc_["`]|pnc_[0-9a-fA-F]{8}' control-plane/internal --include=*.go | grep -v _test.go
→ exactly one line: deploy_connection.go's prefix check, strings.HasPrefix(token, "pnc_")
  (review I1. The pattern guards two things: a real key — Panacea mints pnc_<32 hex>,
  handlers/api_keys.go:360-368 at b33d15acb — pasted into any non-test source, and a second
  place that tests the prefix. It does not match the lines that only name the prefix: the
  TokenSource comment (t02:25), panaceatest's "pnc_test" (Server.Token, t03:291, a non-test
  file) and the refusal sentence "A Panacea API key starts with pnc_." (:155). A bare 'pnc_'
  prints all three. The quote class is ["`], so a second check written with a raw string,
  strings.HasPrefix(token, `pnc_`), prints as well; the "-only form missed it — plan re-review
  b06 M-6, planted and run.)

grep -n 'corsMethods =\|corsHeaders =' control-plane/internal/httpx/origin.go
→ corsMethods lists PUT; corsHeaders lists Idempotency-Key (and Authorization, if Hub C3 is in)
```

#### What this task does NOT claim

- That Credential Manager holds a `pnc_` token across a restart on this laptop — that is Hub C1's
  `WinCred`, run on Windows CI; this task uses its interface.
- That the §6.6 exposure is closed. Another OS user reaching `127.0.0.1:8730` is stopped by Hub
  task C3's local caller key, not here.
- Anything visual. Task 14 draws these states.
- That WebView2 accepts the widened preflight. The rows prove what the middleware answers, not
  what the browser does with that answer. The real window first sees it at `LIVE-CHECK.md`
  steps 2 and 7 (task 20, OD-12).
