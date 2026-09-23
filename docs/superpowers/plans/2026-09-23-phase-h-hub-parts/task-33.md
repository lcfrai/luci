### Task 33: `hubclient` over HTTP — the transport, its retry, idempotency and refresh rules, and the relay rule

**Lane C. Depends on: 04 (lane A). About 3 hours. Contract task C2.**

Every Hub-backed control-plane handler — phase 4's, 5's, 7's, 3's — calls the Hub through this
client and answers the desktop through **one** function, `hubclient.WriteError`, so a teammate's
`revision_conflict` reads the same in both processes and a Hub outage is one well-known 503
(contract §4.3, §5.1). What the client does for every call, so no handler has to (§9.2): take the
user's token, send a fresh `Idempotency-Key` on every `POST` **and reuse it across its own
retries**, retry transport failures and 502/503/504 twice, refresh and retry once on a 401
`expired`, and time out each attempt at 10 s. This task is the transport and the relay; task 34 is
the methods.

**Files:**
- `control-plane/internal/hub/hubclient/client.go` — `Client`, `New`, `Options` (replaces task 04's stub) (new)
- `control-plane/internal/hub/hubclient/transport.go` + `transport_test.go` (new)
- `control-plane/internal/hub/hubclient/relay.go` + `relay_test.go` (new)

#### Contract — complete

```go
// control-plane/internal/hub/hubclient/client.go
package hubclient

type Client struct{ /* unexported: base, scope, tokens, http, now, sleep, log */ }

// Options are for tests; New's zero Options are production's.
type Options struct {
	AttemptTimeout time.Duration          // default 10 s
	Backoff        []time.Duration        // default [250 ms, 1 s] — so at most three attempts
	Sleep          func(d time.Duration)  // default time.Sleep
	Log            *slog.Logger
}

func New(baseURL, scope string, tokens TokenSource, httpClient *http.Client) *Client
func NewWithOptions(baseURL, scope string, tokens TokenSource, httpClient *http.Client, o Options) *Client

// ErrTokenUnavailable wraps a TokenSource failure that is not ErrSignedOut —
// Entra unreachable, in practice. The relay answers it 503 upstream_unavailable,
// details.upstream "entra".
var ErrTokenUnavailable = errors.New("hubclient: no token: Entra could not be reached")
```

**One call, in order:**

1. `tokens.Token(ctx, scope)`: `ErrSignedOut` passes through unchanged; any other error →
   `fmt.Errorf("%w: %w", ErrTokenUnavailable, err)`.
2. Headers: `Authorization: Bearer <token>`, `Accept: application/json`, and for a JSON body
   `Content-Type: application/json; charset=utf-8`. **Never** an `Origin` header (the Hub refuses
   one, §11.4). A `POST` carries `Idempotency-Key: <ids.UUID()>`, minted **once per call** and
   sent unchanged on every retry of that call.
3. Each attempt has its own `AttemptTimeout` deadline. A transport error, a timeout, a TLS failure,
   or a 502/503/504 → retry after `Backoff[i]`, at most `len(Backoff)` times; then `ErrUnavailable`
   wrapping the last cause. **Every attempt sends the whole body**: a JSON body is a `[]byte`
   re-sent as is; a blob body is the caller's `io.ReadSeeker` (HA5-1, task 00), sought to 0
   before each attempt — a failed `Seek` ends the call with that error, never an attempt with a
   short body.
4. A 401 whose `details.reason` is `expired` → `Invalidate(scope)` if `tokens` is an
   `Invalidator` (task 31), a fresh `Token`, **one** retry; a second 401 → `*Error` as answered.
5. Any other non-2xx with a problem body → `*Error{Status, Problem}`; a 500 or 501 → `ErrUnavailable`
   (not retried: a Hub bug is not transient). A non-2xx without a parseable problem → `*Error` with
   code `internal` and the status.
6. A 2xx JSON body over **8 MiB** → `ErrUnavailable` (a Hub that answers that is broken). The cap
   is derived, not picked (task 00, *Sizes*): the two largest bodies the Hub sends are `/v1/refs`
   (100 keys × a 64 KiB revision ≈ 6.3 MiB) and route 26's reply (64 revisions echoed with their
   content ≈ 4.1 MiB). The old 1 MiB cap reported a **committed** 16-revision batch as
   `ErrUnavailable`. Blob bodies stream (task 34) and are bounded by the blob limit. A 429's
   `Retry-After` is kept in `Error.RetryAfter` (task 04).
7. Every error that leaves the client names the method, the route **pattern** and the Hub's
   `Request-Id` when there was one — never the token, never a request body.

```go
// control-plane/internal/hub/hubclient/relay.go
package hubclient

// WriteError is the relay rule (contract §4.3), and the only way a Hub-backed
// control-plane handler answers a Hub failure:
//
//	*Error, 4xx except 401     → the same status, code, message and details; Retry-After copied on a 429
//	*Error, 401                → 401 signed_out, details.reason = the Hub's reason
//	ErrSignedOut               → 401 signed_out, details.reason = "signed_out"
//	ErrUnavailable             → 503 upstream_unavailable, details.upstream = "hub"
//	ErrTokenUnavailable        → 503 upstream_unavailable, details.upstream = "entra"
//	ErrNotConfigured           → 503 hub_not_configured
//	anything else              → 500 internal; the error is logged, not sent
//
// signed_out's message is the control plane's own sentence: "You're signed out
// of PanaceaApp. Sign in to see your team's shared work." The desktop renders it
// verbatim (desktop/src/api/problems.ts classifies it server-said) and its rail
// footer offers the sign-in (task 41).
func WriteError(w http.ResponseWriter, log *slog.Logger, err error)

// Relay is the same rule as a value (HA4-5, task 00), for a handler that answers
// in its own shape — a batch result, a stream's last frame. It returns the status
// and problem WriteError would write; ok is false for "anything else" (a non-Hub
// error), which the caller answers itself (WriteError: 500 internal, logged). A
// 429's Retry-After is (*Error).RetryAfter, read with errors.As.
func Relay(err error) (status int, p httpx.Problem, ok bool)
```

`WriteError` is `Relay` plus `httpx.WriteProblem` (and the `Retry-After` copy), so the two can
never disagree, and the body is byte-for-byte the control plane's problem shape (§4.1).

#### Tests to write first

Against an `httptest.Server` playing the Hub, with a scripted `TokenSource` that counts calls and
implements `Invalidator`, and a recording `Sleep`.

`relay_test.go` — **one test per row of contract §4.3**, plus the rows this plan adds:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRelayA4xxUnchanged` | the Hub's 409 `revision_conflict` with `details.headRevision` → the desktop gets 409, the same code, message and details | a relay that rewrites a teammate's conflict into a generic error |
| `TestRelayA429KeepsRetryAfter` | 429 with `Retry-After: 3` → the same | a desktop that retries a rate limit at once |
| `TestRelayA401IsSignedOut` | a 401 `reason: audience` → 401 `signed_out`, `details.reason: audience` | a Hub rejection shown as the user's password being wrong |
| `TestRelaySignedOut` | `ErrSignedOut` → 401 `signed_out`, reason `signed_out` | — |
| `TestRelayUnavailableIsUpstreamHub` | `ErrUnavailable` → 503 `upstream_unavailable`, `upstream: hub` (ruling R4's code, a third upstream name) | a new desktop `ProblemKind` for one more outage (contract §5.1) |
| `TestRelayTokenUnavailableIsUpstreamEntra` | → 503, `upstream: entra` | an Entra outage reported as the Hub's |
| `TestRelayNotConfigured` | → 503 `hub_not_configured` | — |
| `TestRelayAnythingElseIs500` | `io.ErrUnexpectedEOF` → 500 `internal`, the cause logged and absent from the body | a Go error chain in `message` |
| `TestRelayAgreesWithWriteError` | for every error above, `Relay`'s `(status, p)` equals what `WriteError` writes, byte for byte after marshalling; `io.ErrUnexpectedEOF` → `ok == false` | phase 4's `discoverapi.relay` and the control plane's disagreeing about one Hub failure (HA4-5) |

`transport_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAnExpiredTokenRefreshesOnce` | 401 `expired` then 200 → success; `Invalidate` called once; two `Token` calls | the contract's *"refreshes once and retries"* skipped |
| `TestASecondExpiredIs401` | 401 `expired` twice → `*Error` 401; exactly one retry | a refresh loop against a Hub that disagrees about the clock |
| `TestRetriesTransientFailuresTwice` | 503, 502, then 200 → success; sleeps recorded `[250ms, 1s]` | no retry on a rolling Hub deploy (two replicas, §12.3) |
| `TestGivesUpAfterThreeAttempts` | 503 × 3 → `ErrUnavailable`; three requests | an unbounded retry |
| `TestNeverRetriesA4xx` | 409 → one request | a duplicate write on a conflict |
| `TestA500IsNotRetried` | 500 → `ErrUnavailable` after one request | a Hub bug hammered three times per call |
| `TestTheIdempotencyKeyIsReusedAcrossRetries` | 503 then 201 on a `POST` → both requests carry the **same** key | a retried reserve creating two releases (§1) |
| `TestEachCallGetsAFreshKey` | two `POST` calls → two different keys, each a v4 UUID | one key replaying the first call's answer to the second |
| `TestAGetCarriesNoKey` | → no `Idempotency-Key` | — |
| `TestEachAttemptTimesOut` | `AttemptTimeout: 50ms`, a Hub that stalls → retried, then `ErrUnavailable` in < 1 s total | a control plane hung on a hung Hub |
| `TestNoOriginHeaderIsSent` | the fake Hub records none | a 403 `forbidden_origin` from every call |
| `TestTokenErrorsMap` | `ErrSignedOut` passes through `errors.Is`; any other → `errors.Is(err, ErrTokenUnavailable)` | an Entra outage signing the user out |
| `TestAnOversizeBodyIsUnavailable` | a 6.5 MiB 200 body → decoded; an 8 MiB + 1 200 body → `ErrUnavailable` | unbounded memory on a broken Hub, or a committed batch or a 100-key `/v1/refs` read reported as an outage (task 00) |
| `TestARetryRewindsASeekableBody` | a call whose body is an `io.ReadSeeker` (the shape task 34's `PutBlob` passes) gets 503 after the fake Hub read half of it → the second attempt's body is the whole body, byte-equal; a reader whose `Seek` fails → that error after one attempt | a retried upload sending a short body the Hub then refuses `digest_mismatch` (HA5-1) |
| `TestA429KeepsRetryAfterOnTheError` | 429 with `Retry-After: 3` → `errors.As` gives `*Error` with `RetryAfter: "3"` | `WriteError` asked to copy a header the error never carried |
| `TestErrorsNeverHoldTheToken` | every error above, formatted `%+v`, lacks the bearer | T1 |

#### Steps

- [ ] **Step 1:** Re-read contract §4.3, §9.2 ("what the client does for every call"), §1
  (idempotency).
- [ ] **Step 2:** Tests; they fail. Then `client.go` (deleting task 04's stub `Client`),
  `transport.go`, `relay.go`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubclient/ -race -count=1 -v -run 'TestRelay|TestAn|TestASecond|TestRetries|TestGivesUp|TestNever|TestA500|TestTheIdempotency|TestEachCall|TestAGet|TestEachAttempt|TestNoOrigin|TestTokenErrors|TestErrorsNever' 2>&1 | grep -c "^--- PASS"
# 22 (8 relay + 14 transport; no task 04 or 05 test name matches these prefixes)
go list -deps ./control-plane/internal/hub/hubclient | grep panacea-app      # hubapi, hubclient, httpx, ids — nothing else (A9)
```

#### What this task does not claim

- Any `Hub` method over HTTP: task 34. Until it lands, `Client`'s methods still answer
  `ErrNotConfigured`.
- The digest re-check: task 34, because it is per method.
