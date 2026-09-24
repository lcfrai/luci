### Task 02: The Panacea transport — address, bearer, errors, TLS

**Half:** 7a. **Hours:** 2. **Depends on:** 01.

The only code in the repository that speaks HTTP to Panacea. Tasks 03 and 04 add methods on top of
the one `do` this task writes; none of them opens a connection of its own. Its rules come from what
Panacea's source says it does (plan §3.3, 7P-10 to 7P-12) and from `panacea-mcp`'s proven behaviour
(`services/panacea-mcp/src/index.ts:175-203`), ported as behaviour, not code.

**Files:**
- `control-plane/internal/deploy/panacea/client.go` (new) — `Config`, `TokenSource`, `Client`, `New`, `ParseOrigin`, `Version`, `do`
- `control-plane/internal/deploy/panacea/errors.go` (new) — `mapStatus`, `readProblem`
- `control-plane/internal/deploy/panacea/client_test.go` (new)
- `control-plane/internal/deploy/panacea/errors_test.go` (new)
- `control-plane/internal/deploy/panacea/cmd/versioncheck/main.go` (new) — prints `Version` for an origin; written and vetted here, run only by the owner (`LIVE-CHECK.md` step 1, task 20)

#### Interfaces — complete

```go
// Package panacea implements deploy.Deployer over Panacea's panacea-api (OD-13: phase 7 is a
// client of its existing API). It is the ONLY package that knows a Panacea route, envelope or
// field spelling. Every DTO in wire.go names the Panacea file and line it mirrors.
package panacea

// TokenSource yields the signed-in user's own pnc_ token. connection.Manager (task 05)
// implements it over the credential store. deploy.ErrNotConnected when none is stored.
type TokenSource interface {
	PanaceaToken(ctx context.Context) (string, error)
}

type Config struct {
	// Origin is scheme://host[:port], no path — ParseOrigin's output. Every Panacea route
	// already begins /api/v1/ (routes.go), so the client appends paths verbatim.
	Origin string
	Tokens TokenSource
	// CAFile, when set, is a PEM bundle ADDED to the system roots. Empty uses the platform
	// verifier alone. There is no option to skip verification.
	CAFile string
	// Timeout bounds each attempt. Zero means 15s: POST …/deploy itself waits up to 15 s inside
	// Panacea (handlers/projects.go:2985), so a lower value would abandon deploys that succeed.
	Timeout time.Duration
	// HTTPClient replaces the built client. Tests only.
	HTTPClient *http.Client
	Log        *slog.Logger // nil = slog.Default()
	UserAgent  string       // "PanaceaApp/<version>"
}

type Client struct{ /* cfg, http *http.Client, log, sleep func(ctx, time.Duration) error (ctx-aware; tests record it, review M3) */ }

// New validates cfg and builds the client. Errors are sentences a person can act on.
func New(cfg Config) (*Client, error)

// ParseOrigin returns the canonical origin ("https://dev.panacea.kpmg", lowercased host, no
// trailing slash, default port dropped). Refused, each with its own sentence:
//   - a scheme other than https  → "The Panacea address must start with https://."
//   - a path, query or fragment   → "Give the address without a path: https://dev.panacea.kpmg, not https://dev.panacea.kpmg/api."
//   - userinfo                    → "The address must not contain a user name or password."
//   - no host                     → "That is not a web address."
func ParseOrigin(s string) (string, error)

// Version calls GET <origin>/api/v1/version, unauthenticated (routes.go:630), and returns the
// "version" string. It exists before any Client does: the connect form proves an address is a
// Panacea API with it before accepting it. A redirect is NOT followed and is reported as
// ErrSignInRedirect — the api.dev host 307s to /oauth2/sign_in (orchestrator-rulings.md).
// It makes one attempt and never retries: the connect form shows its answer at once.
func Version(ctx context.Context, hc *http.Client, origin string) (string, error)

// ErrSignInRedirect: the address answered with a redirect, which Panacea's API never does and
// its SSO proxy always does. Its sentence names the likely fix.
var ErrSignInRedirect = &deploy.Error{
	Kind:    deploy.KindRejected,
	Op:      "version",
	Message: "That address sends you to a sign-in page, so it is the portal, not the API. Try https://dev.panacea.kpmg.",
}

// BuildHTTPClient is the one place TLS is configured:
//   the transport is http.DefaultTransport.(*http.Transport).Clone(), never a fresh
//   &http.Transport{} (whose Proxy is nil). The clone keeps Proxy: http.ProxyFromEnvironment
//   (HTTPS_PROXY, NO_PROXY), ForceAttemptHTTP2 (which keeps HTTP/2 once TLSClientConfig is
//   set), and the idle-connection and handshake timeouts. Only TLSClientConfig is replaced;
//   RootCAs nil when caFile is "" (the platform verifier; on Windows, the Windows store);
//   otherwise x509.SystemCertPool() plus caFile's certificates;
//   MinVersion TLS 1.2; CheckRedirect returns http.ErrUseLastResponse (never follow).
//   Go reads the proxy from the environment only, never the Windows system or PAC proxy.
//   Whether the owner's network needs one for Panacea is unverified: LIVE-CHECK.md step 1
//   answers it, and the fix is HTTPS_PROXY, not code (review M2).
func BuildHTTPClient(caFile string, timeout time.Duration) (*http.Client, error)
```

```go
// do is the one request path. Unexported; tasks 03 and 04 call it.
//
//   - Authorization: Bearer <token from Tokens>; Accept: application/json; User-Agent;
//     X-Request-Id: ids.UUID() (Panacea echoes it as X-Client-Request-Id, middleware/requestid.go:28-32).
//   - GET only: a transport failure, 502, 503 or 504 is retried twice, after 250 ms and 1 s.
//     POST, PUT and DELETE are NEVER retried: a retried deploy is a second deploy.
//     Retry-After is honoured inside do up to 2 s; a longer one is returned, and no caller
//     waits on it yet (review M3; b07 round 2, M-3). A 503 that carries it waits
//     max(the fixed delay, Retry-After) when Retry-After is at most 2 s; a longer one ends the
//     retries at once and returns the KindUnavailable with RetryAfter set, so do never blocks
//     the watcher's loop on it; the watcher's failure rule (t07:259-266) ends the cycle. A 429
//     is not retried at all; it returns with RetryAfter set. The watcher's next cycle then comes
//     3 s or 15 s later whatever RetryAfter says, so a longer Retry-After is overridden by the
//     cadence. That caller-side wait is its own open item, seam S7-R1 (plan §10.2), not t07's
//     per-cycle deadline (M14, which bounds a cycle and does not back off).
//   - A 2xx body is read up to 8 MiB into out (json.Unmarshal); an unreadable 2xx body is
//     KindUnavailable with Op set, never a zero value handed back as success.
//   - A non-2xx is readProblem + mapStatus. Its sentence reaches Error.Message by the table
//     below; the full body goes to the log at Debug and never into an error string.
//   - One log line per failed call: op, method, path, status, duration, Panacea's X-Request-Id.
//     Never the Authorization header, never a request body.
func (c *Client) do(ctx context.Context, op, method, path string, in, out any, okStatus ...int) error
```

#### The status table — `errors.go`, exactly

Panacea's error bodies are `{"error":"<sentence>"}` from three writers: `writeError`
(`handlers/helpers.go:41-43`), `writeAPIKeyError` (`middleware/api_key.go:396-400`), and `http.Error`
with a JSON literal (`middleware/auth.go:250`, `middleware/rbac.go:398-400`) — the last arrives as
`text/plain; charset=utf-8`, so **`readProblem` parses the body as JSON whatever `Content-Type`
says**, takes `error`, falls back to `message`, trims to 500 bytes on a rune boundary, and returns
`""` for a body that is not that JSON (an ingress HTML page).

| Panacea answered | `deploy.Error.Kind` | `Message` | `RetryAfter` |
|---|---|---|---|
| dial, TLS, timeout, reset | `KindUnavailable` | `""` | 0 |
| 3xx | `KindUnavailable` (from `Version`: `ErrSignInRedirect`) | `""` | 0 |
| 401 | `KindUnauthorized` | the sentence (e.g. *"api key revoked or expired"*) | 0 |
| 403 | `KindForbidden` | the sentence | 0 |
| 404 | `KindNotFound` | the sentence | 0 |
| 409 | `KindConflict` | the sentence | 0 |
| 402 | `KindConflict` | the sentence — a refusal, never an outage: `POST …/deploy` answers 402 from its two gates, the budget (*"new deploys are paused: <reason>"*, `handlers/projects.go:2856`, reason built at `handlers/budget_resolve.go:229-245`) and the risk assessment (*"production deploy blocked by the project's risk assessment: …"*, `projects.go:2916-2918`); both sentences are written for people and carry no `err.Error()` (review I2, fix batch b05) | 0 |
| 400, 413, 422 | `KindRejected` | the sentence | 0 |
| 429 | `KindUnavailable` | `""` | `Retry-After` seconds |
| 500–599 | `KindUnavailable` | **`""` — never the sentence**: Panacea appends `err.Error()` to them (e.g. `projects.go:1529`) | `Retry-After` on 503 |
| any other | `KindUnavailable` | `""` | 0 |

`Status` is always the upstream status (0 for transport). `Cause` carries the Go error for the log.

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestParseOriginTable` | the four refusals above each yield their own sentence; `HTTPS://Dev.Panacea.KPMG/` → `https://dev.panacea.kpmg`; `:443` dropped | accepting `https://dev.panacea.kpmg/api`, which would produce `/api/api/v1/…` — measured 404 (orchestrator-rulings.md) |
| `TestBearerOnEveryRequest` | an `httptest` server records `Authorization: Bearer pnc_test`; the request has no `Cookie` and no `Origin` | a client that forgets the header on one method |
| `TestTokenNeverInErrors` | a 401 whose body echoes the token back; the returned error's `Error()` and `Message` do not contain `pnc_test`, and the captured log does not either | echoing an upstream body that quotes the credential |
| `TestNotConnectedShortCircuits` | `Tokens` returns `deploy.ErrNotConnected` → `do` returns it and the server saw **no** request | sending an unauthenticated request that Panacea would answer 401, turning "not connected" into "token rejected" |
| `TestStatusTable` | one sub-test per row of the table, including `text/plain` bodies and an HTML 502 | mapping 401 to forbidden (the lens would say "ask for access" to someone whose token expired) |
| `TestPaymentRequiredIsConflict` | a 402 `{"error":"new deploys are paused: team \"alpha\" has reached its monthly budget; …"}` → `KindConflict`, `Status == 402`, `Message` equal to the sentence | the "any other" row swallowing 402, so a paused budget reads as *Panacea is down* and task 09 never abandons the reservation (review I2) |
| `TestFiveHundredSentenceNeverRendered` | a 500 `{"error":"failed to list deployments: dial tcp 10.0.3.7:6443"}` → `Message == ""`, and the log holds the sentence | leaking a cluster address into the UI |
| `TestGetRetriedPostNot` | a server failing the first two attempts with 503: GET succeeds on the third; POST returns `KindUnavailable` after exactly **one** attempt | retrying a deploy |
| `TestGetRetryHonoursRetryAfter` | `do` takes its sleep through an unexported `c.sleep` the test records. A GET answered 503 `Retry-After: 1`, then 200 → one sleep of 1 s (not 250 ms). A GET answered 503 `Retry-After: 30` → one attempt, `KindUnavailable`, `RetryAfter == 30s`, no sleep. A GET answered 429 `Retry-After: 5` → one attempt, `RetryAfter == 5s` | a retry on the fixed 250 ms that ignores the server's `Retry-After`, or a `do` that sleeps 30 s inside the watcher's cycle (review M3) |
| `TestRedirectNotFollowed` | a 307 to `/oauth2/sign_in` → `Version` returns `ErrSignInRedirect`; the server saw one request | following the redirect and reading the sign-in HTML as a failed JSON decode |
| `TestCAFileAddsRoot` | `httptest.NewTLSServer`; its certificate written to a temp PEM; `BuildHTTPClient(pem, …)` reaches it; `BuildHTTPClient("", …)` fails with an `x509` unknown-authority error | a CA option that is read and never applied — phase 2's silent-override class |
| `TestProxyFromEnvironmentHonoured` | the test re-runs its own binary (`os.Args[0] -test.run=TestHelperProxyChild`) with `HTTPS_PROXY=http://<an httptest recorder>` and `NO_PROXY=""` and `P7_PROXY_CHILD=1`, because `ProxyFromEnvironment` reads the environment once per process (`sync.Once` in `net/http`). The acceptance `-run` regex below matches the helper too; without `P7_PROXY_CHILD=1` it calls `t.Skip` at once. The child calls `Version` for `https://panacea.invalid` through `BuildHTTPClient("", 5s)`. `Version` does not retry, so the recorder sees exactly one request, `CONNECT panacea.invalid:443`, and answers 502; the child exits 0 on that proxy error and 1 on any other. In-process, the transport has `ForceAttemptHTTP2 == true`, `TLSClientConfig.MinVersion == tls.VersionTLS12`, and `IdleConnTimeout`, `TLSHandshakeTimeout`, `MaxIdleConns` and `ExpectContinueTimeout` equal to `http.DefaultTransport`'s | a fresh `&http.Transport{}`: its nil `Proxy` dials `panacea.invalid` directly, the recorder sees nothing, and on a corporate network every call fails before it leaves the machine (review M2); or a hand-built transport that sets `Proxy` and `ForceAttemptHTTP2` but drops the idle and handshake defaults (b07's named survivor) |
| `TestTimeoutPerAttempt` | a server that sleeps 200 ms, `Timeout: 50ms` → `KindUnavailable` in under 1 s total for a POST | a timeout on the whole retry loop instead of each attempt, or none |
| `TestVersionDecodes` | `{"version":"v1.163.0"}` → `"v1.163.0"`; `{"version":""}` and `{}` → an error: *"That address answered, but not as the Panacea API."* | accepting any 200 JSON as a Panacea API |

#### Steps

- [ ] Write the two test files; run `go test ./control-plane/internal/deploy/panacea/ -race
  -count=1`; they fail to compile.
- [ ] Write `client.go` and `errors.go` as declared. `do` builds the URL as `c.cfg.Origin + path`;
  `path` always begins `/api/v1/` and every path segment taken from a caller is escaped with
  `url.PathEscape` (a branch name contains `/`).
- [ ] Run the tests green.
- [ ] Write `cmd/versioncheck/main.go`: a 20-line `main` that takes the origin as its one
  argument and calls `Version` through `BuildHTTPClient("", 15s)`. It prints the version, or the
  error and exit status 1. **Do not run it.** Type-check it with `go vet` (Acceptance). This task
  makes no network call: an implementing agent never calls a real Panacea, and OD-12 batches every
  live item into the owner's session (review I9). 7P-10 (Go through the Windows store trusts
  KPMG's interception root) is settled when the owner runs this command at `LIVE-CHECK.md` step 1
  (task 20). If that run fails with an `x509` error, the fix is `PANACEA_APP_PANACEA_CA_FILE`,
  not a skip flag.
- [ ] Commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/panacea/ -race -count=1 -run 'Origin|Bearer|Token|Status|Retried|RetryAfter|Redirect|CAFile|Proxy|Timeout|Version|NotConnected|FiveHundred'
→ ok

go vet ./control-plane/internal/deploy/panacea/cmd/versioncheck
→ (nothing; it compiles and is not run: its live run is LIVE-CHECK.md step 1, task 20)

grep -rn 'InsecureSkipVerify' control-plane/internal/deploy/
→ (nothing)
```

#### What this task does NOT claim

- That any authenticated route works. No token is used in this task.
- That the Windows store trusts the interception root on **any** KPMG laptop. This task makes no
  live call. The owner's `LIVE-CHECK.md` step 1 (task 20) settles it for the one laptop it runs
  on.
- Anything about the response shapes of routes other than `/api/v1/version`. Task 03.
