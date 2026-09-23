### Task 30: `entraauth` — sign-in: discovery, PKCE, the one-shot loopback listener, the exchange (correction A6)

**Lane C. Depends on: 12, 28. About 3 hours. Contract task C1.**

`OD-6`: the desktop signs the user in with Entra, OAuth 2.0 authorization code + PKCE, in the
**system browser**. ADR-H1 §5.2 puts the flow and the tokens **in the control plane**: the renderer
asks for an authorize URL, opens it with `tauri-plugin-opener`, and never sees a code or a token.
The callback lands on a **one-shot listener on `127.0.0.1:0`** (RFC 8252 §7.3), separate from the
control plane's own listener, because an OAuth callback is a side-effecting GET and the main
listener's Origin policy rests on having none (`control-plane/internal/httpx/origin.go:97-116`).
This task is contract §6.1, steps 2–11. Task 31 is what happens after (refresh, sign-out,
start-up).

**Files:**
- `control-plane/internal/entraauth/{entraauth.go,oidc.go,pkce.go,listener.go,signin.go,idtoken.go}` (new)
- `control-plane/internal/entraauth/{signin_test.go,idtoken_test.go,pkce_test.go}` (new)

#### Contract — complete

```go
// control-plane/internal/entraauth/entraauth.go

// Package entraauth signs the user in to Entra (auth code + PKCE, system browser,
// loopback redirect) and serves access tokens for the resources the desktop client
// is consented for (contract §6, §9.1). It holds the refresh token in
// credstore.Secrets and access tokens in memory only. Nothing here logs a token,
// a code, a verifier or an id_token.
package entraauth

// TokenSource is phase 8's HA-2, exactly: remotebe and hubclient call it.
type TokenSource interface {
	Token(ctx context.Context, scope string) (string, error)
}

// ErrSignedOut is hubclient.ErrSignedOut — one value, so errors.Is holds across
// both packages without hubclient importing this one (plan correction A9).
var ErrSignedOut = hubclient.ErrSignedOut

var (
	ErrAlreadySignedIn  = errors.New("entraauth: already signed in")        // POST /v1/auth/sign-in → 409 conflict
	ErrEntraUnavailable = errors.New("entraauth: Entra could not be reached") // → 503 upstream_unavailable, upstream "entra"
	// ErrTenantNotEnrolled is what AfterSignIn returns when the Hub answered
	// 403 tenant_not_enrolled; the sign-in then fails with that reason.
	ErrTenantNotEnrolled = errors.New("entraauth: this tenant is not enrolled in the Hub")
)

type State string

const (
	SignedOut State = "signed_out"
	SigningIn State = "signing_in"
	SignedIn  State = "signed_in"
)

type Account struct{ TenantID, ObjectID, Name, Username string }

type Status struct {
	State   State
	Reason  string   // "" | "timeout" | "entra_error" | "exchange_failed" | "reauth_required" | "tenant_not_enrolled"
	Account *Account // nil unless SignedIn
}

type Config struct {
	Authority    string // PANACEA_APP_ENTRA_AUTHORITY: https://login.microsoftonline.com/<tenant>, or devissuer's
	ClientID     string // PANACEA_APP_ENTRA_CLIENT_ID
	HubScope     string // PANACEA_APP_HUB_SCOPE, e.g. "api://<hub-client-id>/Hub.Access"
	RedirectHost string // "127.0.0.1" (default) or "localhost" (contract §6.1)
	Secrets      credstore.Secrets
	HTTPClient   *http.Client // default: 10 s timeout
	Now          func() time.Time
	// SignInTimeout defaults to 10 minutes (contract §6.1, "failures").
	SignInTimeout time.Duration
	// AfterSignIn is step 11: called once with the fresh Hub token already in
	// the cache (so Token(ctx, HubScope) answers it). It returns the Hub org's
	// Entra tenant id from GET /v1/me — correction A6 cross-checks it against the
	// id_token's tid — or ErrTenantNotEnrolled, or another error. (Contract §9.1
	// declared it returning error only; the tenant id is A6's addition.)
	AfterSignIn func(ctx context.Context) (hubTenantID string, err error)
	Log         *slog.Logger
}

type Authenticator struct{ /* unexported: config, discovery cache, state, pending flow, token cache */ }

func New(cfg Config) (*Authenticator, error) // validates cfg; reads the account record (task 31); no network

// BeginSignIn is contract §6.1 steps 2–4. It cancels a pending sign-in, opens the
// listener, fetches discovery (cached 24 h), and returns the authorize URL and
// its expiry (now + SignInTimeout). The rest runs in the background; Status
// reports it.
func (a *Authenticator) BeginSignIn(ctx context.Context) (authorizeURL string, expiresAt time.Time, err error)

func (a *Authenticator) Cancel()        // closes the listener; state signed_out, reason ""
func (a *Authenticator) Status() Status
func (a *Authenticator) SignOut(ctx context.Context) error                       // task 31
func (a *Authenticator) Token(ctx context.Context, scope string) (string, error) // task 31

var _ TokenSource = (*Authenticator)(nil)
```

**The flow, exactly** (contract §6.1, with correction A6 at step 9):

| Step | What |
|---|---|
| 2 | `verifier` = base64url(32 random bytes) — 43 characters; `challenge` = base64url(SHA-256(verifier)); `state`, `nonce` = base64url(32 random bytes) each. `net.Listen("tcp4", "127.0.0.1:0")` |
| 3 | `GET <Authority>/v2.0/.well-known/openid-configuration`, cached 24 h; a failure → `ErrEntraUnavailable` |
| 4 | the authorize URL: the discovered `authorization_endpoint` with `client_id`, `response_type=code`, `response_mode=query`, `redirect_uri=http://<RedirectHost>:<port>/`, `scope=openid profile offline_access email <HubScope>`, `state`, `nonce`, `code_challenge`, `code_challenge_method=S256`, `prompt=select_account` |
| 7 | the listener serves **only `GET /`** (anything else: 404; another method: 405). A `state` that does not match, compared with `subtle.ConstantTimeCompare`, is answered 400 and **ignored** — the flow goes on, so a racing local page cannot abort it. A matching `error=` → `signed_out`, `reason: entra_error`; `error_description` is logged at `warn`, never put in `Status`. A matching `code` → `200 text/html` with `Content-Security-Policy: default-src 'none'`, no script: *"You're signed in to PanaceaApp. You can close this tab."* — then the listener closes |
| 8 | `POST token_endpoint`, form: `grant_type=authorization_code`, `client_id`, `code`, `redirect_uri` (identical), `code_verifier`, `scope` (identical). No secret; no `Origin` header. A non-200 → `exchange_failed` |
| 9 | **A6:** the `id_token`'s payload, decoded **without** a signature check (it came on the back channel from the endpoint this process called — OIDC Core 1.0 §3.1.3.7 item 6): `iss` = the discovered `issuer`; `aud` = `ClientID`; `nonce` = step 2's; `exp` + 120 s > now; `tid` and `oid` present GUIDs. Any failure → `exchange_failed`, nothing stored |
| 10 | write the refresh token with `credstore.PutChunked` under `EntraRefreshPrefix(tid, ClientID)`, **then** the account record `{"tenantId","objectId","name","username","authority","refreshChunks","savedAt"}` at `EntraAccountTarget(tid, ClientID)` — last, so it marks a complete write. Cache the access token for `HubScope` with its expiry (`expires_in`) |
| 11 | `AfterSignIn`. `ErrTenantNotEnrolled` → delete both entries, `reason: tenant_not_enrolled`. A tenant id **different** from the `id_token`'s → delete both, `reason: exchange_failed` (A6). Any other error (the Hub unreachable) → the sign-in **still completes**: Entra said yes and the Hub validates every token on every call anyway; the control plane's feed poller calls `/v1/me` on its first good tick (task 36), which binds invites then |
| — | no callback within `SignInTimeout` → listener closed, `reason: timeout` |

#### Tests to write first

`signin_test.go` runs `devissuer` in-process (task 11/12) on `httptest.NewServer` with `Auto` on,
`credstore.Memory()`, and a **scripted browser**: take the authorize URL, append
`&login_hint=grace@example.test`, `GET` it with a client that does not follow redirects, then `GET`
the `Location` it answers — which is the listener.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSignInEndToEnd` | `Status` goes `signing_in` → `signed_in`; `Account` is Grace; the store holds the account record and ≥ 1 refresh chunk; `AfterSignIn` was called once and `Token(ctx, HubScope)` inside it returned a token `entra` validates | any break in steps 2–11 against the real issuer |
| `TestTheAuthorizeURL` | every parameter of step 4 exactly; `redirect_uri` is `http://127.0.0.1:<the listener's port>/`; the listener is bound to `127.0.0.1`, not `0.0.0.0` | a redirect Entra rejects; a listener reachable from the LAN |
| `TestRedirectHostLocalhost` | `RedirectHost: "localhost"` → `redirect_uri=http://localhost:<port>/`, listener still on `127.0.0.1` | the registration fallback of contract §6.5 |
| `TestAWrongStateIsIgnored` | a `GET /?code=x&state=wrong` → 400, still `signing_in`; the real callback then completes | a local page aborting the user's sign-in |
| `TestTheListenerServesOnlyGetRoot` | `POST /` → 405; `GET /favicon.ico` → 404; neither ends the flow | a side-effecting path on the callback listener |
| `TestAnEntraErrorEndsTheSignIn` | the scripted browser posts *Cancel* on devissuer's page → `signed_out`, `reason: entra_error`; `Status` holds no description | Entra's text shown to the user |
| `TestTheSignInTimesOut` | `SignInTimeout: 200 ms`, no callback → `reason: timeout`; the port no longer accepts connections | a listener left open for ever |
| `TestAFailedExchange` | a code altered before the exchange → `reason: exchange_failed`, nothing stored | — |
| `TestTheIdTokenIsChecked` | a fake token endpoint returning an `id_token` with the wrong `nonce`, `aud`, `iss`, an expired `exp`, a missing `oid` → `exchange_failed` each, nothing stored | A6's checks missing, so a token for another client is accepted |
| `TestANotEnrolledTenantStoresNothing` | `AfterSignIn` → `ErrTenantNotEnrolled` → store empty, `reason: tenant_not_enrolled` | a refresh token kept for a tenant that can never use the Hub |
| `TestATenantMismatchStoresNothing` | `AfterSignIn` → a different tenant id → store empty, `reason: exchange_failed` | A6's cross-check missing |
| `TestAnUnreachableHubStillSignsIn` | `AfterSignIn` → `hubclient.ErrUnavailable` → `signed_in`, the tokens stored | a sign-in that fails because of an unrelated Hub outage |
| `TestSignInWhenSignedInIsRefused` | → `ErrAlreadySignedIn` | two refresh tokens for one account |
| `TestCancelClosesTheListener` | `Cancel()` → `signed_out`, `reason: ""`; the port is closed | — |
| `TestDiscoveryIsCached` | two sign-ins within 24 h → one discovery request | a discovery fetch per sign-in |
| `TestNoSecretInAnyLog` | across every test, the captured log holds no code, verifier, access token, refresh token or id_token | T1 |

`pkce_test.go`: `TestTheRFC7636Vector` — verifier `dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk` →
challenge `E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM` (RFC 7636 Appendix B); and 1,000 generated
verifiers are 43 characters of the base64url alphabet. `idtoken_test.go` covers A6's parser on
hand-built payloads (the same five failures, without HTTP).

#### Steps

- [ ] **Step 1:** Re-read contract §6.1, §6.2, §9.1; ADR-H1 §5.2; OIDC Core §3.1.3.7.
- [ ] **Step 2:** Tests; they fail. Then the six source files. Token refresh and sign-out are
  task 31: `Token` returns only the cached access token here, and `ErrSignedOut` otherwise.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/entraauth/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 18 (16 + TestTheRFC7636Vector + the id_token parser test)
go list -deps ./control-plane/internal/entraauth | grep panacea-app
#   …/internal/credstore  …/internal/entraauth  …/internal/httpx  …/internal/hub/hubapi  …/internal/hub/hubclient  …/internal/ids
```

#### What this task does not claim

- **Entra's behaviour** — loopback redirects on any port, the claims in a v2 `id_token` with
  `profile` and `email`, conditional access on a public client (ADR-H1 R2): task 43's real-tenant
  checklist.
- That the system browser opens: the renderer does that (task 41) with `opener:default`, which is
  verified to allow `https` URLs (plan §"Where this plan corrects the contract").
