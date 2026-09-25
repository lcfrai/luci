### Task 30: `signin` — sign-in: discovery, PKCE, the one-shot loopback listener, the exchange (correction A6)

**Lane C. Depends on: 04 (`hubclient.ErrSignedOut`, `ErrUnavailable`; review M15), 12, 28. About 3.5 hours. Contract task C1.**

**OD-30 (ruling `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` on `main`, sections
2 and 3) rewrites this part.** The user signs in through a **Keycloak realm that brokers Entra**, with no
clicks: auth code + PKCE against `PANACEA_APP_OIDC_ISSUER`, `kc_idp_hint` skips Keycloak's page,
`login_hint` (the Windows UPN) and the Windows session skip Entra's picker, and no `prompt` is ever
sent. OD-6's other half stands: the **system browser**, and ADR-H1 §5.2's mechanics: the flow and the
tokens live **in the control plane**, the renderer never sees a code or a token. The package is
renamed `entraauth` -> `signin` (ruling section 11): it talks to an issuer, not to Entra.
The **control plane opens the browser itself** (`ShellExecuteW` "open", ruling section 2 step 3), so
a launch-time re-sign-in (task 31) needs no renderer round trip and no click.
The callback lands on a **one-shot listener on `127.0.0.1:0`** (RFC 8252 §7.3), separate from the
control plane's own listener, because an OAuth callback is a side-effecting GET and the main
listener's Origin policy rests on having none (`control-plane/internal/httpx/origin.go:97-116`).
This task is contract §6.1, steps 2–11, as the ruling amends them. Task 31 is what happens after
(the silent refresh at launch, per-audience refresh, sign-out).

**Files:**
- `control-plane/internal/signin/{signin.go,oidc.go,pkce.go,listener.go,flow.go,idtoken.go,browser_windows.go,browser_other.go,upn_windows.go,upn_other.go}` (new)
- `control-plane/internal/signin/{flow_test.go,idtoken_test.go,pkce_test.go,listener_test.go,upn_windows_test.go}` (new)

#### Contract — complete

```go
// control-plane/internal/signin/signin.go

// Package signin signs the user in through the OIDC issuer (a Keycloak realm that
// brokers Entra, OD-30: auth code + PKCE, system browser, loopback redirect, zero
// clicks) and serves single-audience access tokens (contract §6, §9.1, as the OD-30
// ruling amends them). It holds the offline refresh token in credstore.Secrets and
// access tokens in memory only. Nothing here logs a token, a code, a verifier, an
// id_token or the login hint.
package signin

// TokenSource is phase 8's HA-2, exactly: remotebe and hubclient call it. Under
// OD-30 the argument is an audience scope: "panacea-app-hub", "panacea-app-broker",
// "panacea-app-gateway" or "panacea-app-fleet" (ruling section 4, A1-A4).
type TokenSource interface {
	Token(ctx context.Context, scope string) (string, error)
}

// ErrSignedOut is hubclient.ErrSignedOut — one value, so errors.Is holds across
// both packages without hubclient importing this one (plan correction A9).
var ErrSignedOut = hubclient.ErrSignedOut

var (
	ErrAlreadySignedIn   = errors.New("signin: already signed in")             // POST /v1/auth/sign-in → 409 conflict
	ErrIssuerUnavailable = errors.New("signin: the issuer could not be reached") // → 503 upstream_unavailable, upstream "entra" (wire value unchanged, contract §10)
	// ErrTenantNotEnrolled is what AfterSignIn returns when the Hub answered
	// 403 tenant_not_enrolled; the sign-in then fails with that reason.
	ErrTenantNotEnrolled = errors.New("signin: this tenant is not enrolled in the Hub")
)

// AudienceScopes is the authorize request's audience list, in this order (ruling
// section 2 step 4). The code-exchange access token, which carries all four, is
// discarded; each Token call narrows to one (task 31).
var AudienceScopes = []string{"panacea-app-hub", "panacea-app-broker", "panacea-app-gateway", "panacea-app-fleet"}

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
	Issuer      string // PANACEA_APP_OIDC_ISSUER: a realm URL, https://<host>/realms/<realm>, or devissuer's loopback one (task 32 refuses anything else)
	Profile     string // PANACEA_APP_OIDC_PROFILE: "keycloak" (default); "entra" is refused by task 32 until a direct-Entra issuer exists
	ClientID    string // PANACEA_APP_OIDC_CLIENT_ID, default "panacea-app-desktop" (a public client: no secret)
	IdPHint     string // PANACEA_APP_OIDC_IDP_HINT, default "entra"; "" omits kc_idp_hint
	LoginHint   string // task 32: PANACEA_APP_OIDC_LOGIN_HINT, else WindowsUPN(); "" omits login_hint, never guessed
	HubAudience string // PANACEA_APP_HUB_AUDIENCE, default "panacea-app-hub"; one of AudienceScopes
	Secrets     credstore.Secrets
	HTTPClient  *http.Client // default: 10 s timeout
	Now         func() time.Time
	// OpenBrowser opens the authorize URL in the system browser. Default:
	// OpenSystemBrowser. Tests pass a scripted browser. An error is logged and
	// the flow still waits: the renderer's fallback link (task 41) covers it.
	OpenBrowser func(authorizeURL string) error
	// SignInTimeout defaults to 10 minutes (contract §6.1, "failures").
	SignInTimeout time.Duration
	// AfterSignIn is step 11: called once with the fresh Hub token already in
	// the cache (so Token(ctx, HubAudience) answers it). It returns the Hub org's
	// Entra tenant id from GET /v1/me — correction A6 cross-checks it against the
	// id_token's tid — or ErrTenantNotEnrolled, or another error. (Contract §9.1
	// declared it returning error only; the tenant id is A6's addition.)
	AfterSignIn func(ctx context.Context) (hubTenantID string, err error)
	Log         *slog.Logger
}

type Authenticator struct{ /* unexported: config, discovery cache, state, pending flow, token cache */ }

func New(cfg Config) (*Authenticator, error) // validates cfg; reads the account record (task 31); no network

// BeginSignIn is contract §6.1 steps 2–4. It cancels a pending sign-in, opens the
// listener, fetches discovery (cached for the process), builds the authorize URL,
// calls cfg.OpenBrowser with it, and returns it and its expiry (now +
// SignInTimeout); the renderer shows it only as a fallback link. The rest runs in
// the background; Status reports it.
func (a *Authenticator) BeginSignIn(ctx context.Context) (authorizeURL string, expiresAt time.Time, err error)

// OpenSystemBrowser is ShellExecuteW(nil, "open", url, nil, nil, SW_SHOWNORMAL)
// through golang.org/x/sys/windows (browser_windows.go); it refuses any URL
// whose scheme is not https, or http on a loopback host (devissuer). Off Windows
// (browser_other.go) it returns errors.ErrUnsupported.
func OpenSystemBrowser(authorizeURL string) error

// WindowsUPN is windows.GetUserNameEx(windows.NameUserPrincipal, ...), the call
// `whoami /upn` makes, read once per process (sync.Once). Any error (for example
// ERROR_NONE_MAPPED for a local account) → "". Off Windows (upn_other.go) → "".
func WindowsUPN() string

func (a *Authenticator) Cancel()        // closes the listener; state signed_out, reason ""
func (a *Authenticator) Status() Status
func (a *Authenticator) SignOut(ctx context.Context) error                       // task 31
func (a *Authenticator) Token(ctx context.Context, scope string) (string, error) // task 31

var _ TokenSource = (*Authenticator)(nil)
```

**The flow, exactly** (contract §6.1 as the OD-30 ruling, sections 2 and 3, amends it; A6 at step 9):

| Step | What |
|---|---|
| 2 | `verifier` = base64url(32 random bytes) — 43 characters; `challenge` = base64url(SHA-256(verifier)); `state`, `nonce` = base64url(32 random bytes) each. `net.Listen("tcp4", "127.0.0.1:0")` |
| 3 | `GET <Issuer>/.well-known/openid-configuration` (Keycloak's layout), **cached for the process**; its `issuer` must equal `Issuer` byte for byte (OIDC Discovery 1.0 §4.3). A failure or a mismatch → `ErrIssuerUnavailable` |
| 4 | the authorize URL: the discovered `authorization_endpoint` with **exactly** `response_type=code`, `client_id`, `redirect_uri=http://127.0.0.1:<port>/callback`, `scope=openid offline_access panacea-app-hub panacea-app-broker panacea-app-gateway panacea-app-fleet` (`AudienceScopes`, in order), `state`, `nonce`, `code_challenge`, `code_challenge_method=S256`, `kc_idp_hint=<IdPHint>` (omitted when `""`), `login_hint=<LoginHint>` (omitted when `""`). **Nothing else**: no `prompt` (neither Keycloak nor Entra is told to re-ask), no `domain_hint` (Keycloak 25.0.0 cannot forward it), no `response_mode` (query is the code flow's default). Then `OpenBrowser(url)` |
| 7 | the listener serves **only `GET /callback`** (anything else: 404; another method: 405). A `state` that does not match, compared with `subtle.ConstantTimeCompare`, is answered with the **failure page** and **ignored** — the flow goes on, so a racing local page cannot abort it. A matching `error=` → the failure page, `signed_out`, `reason: entra_error` (wire value unchanged); `error` and `error_description` are logged at `warn`, never put in `Status` and **never echoed into HTML**. A matching `code` → the **success page**, then the listener closes. Both pages: `Content-Type: text/html; charset=utf-8`, `Cache-Control: no-store`, `Content-Security-Policy: default-src 'none'`, a static body with no script and no external resource. Success: `200`, whole visible text *"Signed in. You can return to PanaceaApp."* Failure: `400`, *"Sign-in did not complete. Return to PanaceaApp to try again."* |
| 8 | `POST token_endpoint`, form: `grant_type=authorization_code`, `client_id`, `code`, `redirect_uri` (identical), `code_verifier`. A **public client**: no secret; no `Origin` header. A non-200 → `exchange_failed`. The response must carry a `refresh_token` (the offline token) and an `id_token`, else `exchange_failed`. Its `access_token` (every audience) is **discarded, never sent** (ruling section 2 step 7) |
| 9 | **A6:** the `id_token`'s payload, decoded **without** a signature check (it came on the back channel from the endpoint this process called — OIDC Core 1.0 §3.1.3.7 item 6): `iss` = `Issuer`; `aud` = `ClientID` (a string, or an array of exactly that one); `azp` = `ClientID`; `nonce` = step 2's; `exp` + 120 s > now; `tid` and `oid` present GUIDs (the realm's P1/P2 mappers). `name`; `username` = `preferred_username`, else `email`. Any failure → `exchange_failed`, nothing stored |
| 9b | **one narrowing redemption** by `oidc.go`'s `redeem` (the same function task 31's `TokenSource` calls): `grant_type=refresh_token`, `client_id`, `refresh_token`, `scope=openid <HubAudience>`. A non-200 → `exchange_failed`, nothing stored. A `refresh_token` in the answer (rotation) replaces step 8's for step 10 |
| 10 | the generation sequence of task 28's `chunk.go` (review I1), under the OD-30 target `PanaceaApp/oidc/<issuer host>/<realm>/refresh` (task 28, as the ruling amends it: `credstore.OIDCRefreshPrefix(Issuer)`, `OIDCRefreshGenPrefix(Issuer, gen)`, `OIDCAccountTarget(Issuer)`): `gen := credstore.NewGeneration()`; write the latest refresh token with `credstore.PutChunked` under `OIDCRefreshGenPrefix(Issuer, gen)`, **then** the account record `{"tenantId","objectId","name","username","issuer","clientId","refreshGen","refreshChunks","savedAt"}` at `OIDCAccountTarget(Issuer)` — last, so it marks a complete write, and naming `gen`; **only then** `SweepGenerations(OIDCRefreshPrefix(Issuer), gen)`. A failed chunk or record `Put` → `exchange_failed`, a best-effort `DeleteChunked` of `gen`'s prefix, and any earlier record left as it was. Cache step 9b's single-audience access token for `HubAudience` until 60 s before its `exp` |
| 11 | `AfterSignIn`. `ErrTenantNotEnrolled` → delete both entries (the account record **first**, then every generation by `DeleteChunked(OIDCRefreshPrefix(Issuer))`, so a crash in between leaves chunks with no record, which start-up sweeps), `reason: tenant_not_enrolled`. A tenant id **different** from the `id_token`'s → delete both, `reason: exchange_failed` (A6). Any other error (the Hub unreachable) → the sign-in **still completes**: the issuer said yes and the Hub validates every token on every call anyway; the control plane's feed poller calls `/v1/me` on its first good tick (task 36), which binds invites then |
| — | no callback within `SignInTimeout` → listener closed, `reason: timeout` |

#### Tests to write first

`flow_test.go` runs `devissuer` in-process (task 11/12, the `keycloak` profile, its default under
OD-30) on `httptest.NewServer` with `Auto` on, `credstore.Memory()`, `Issuer = <server>/realms/panacea-app-dev`,
`LoginHint: "grace@example.test"`, and a **scripted browser** as `OpenBrowser`: it records the URL,
`GET`s it with a client that does not follow redirects, then `GET`s the `Location` it answers — which
is the listener. No test adds a parameter to the URL: devissuer picks the dev user from the
`login_hint` the control plane sent (ruling section 5), so the hint is under test, not supplied by it.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSignInEndToEnd` | `OpenBrowser` was called once, with the returned URL; `Status` goes `signing_in` → `signed_in`; `Account` is Grace; the store holds the account record, and its `refreshGen` names the **only** generation under `OIDCRefreshPrefix(Issuer)`, which holds exactly `refreshChunks` ≥ 1 chunks; `AfterSignIn` was called once and `Token(ctx, "panacea-app-hub")` inside it returned a token `oidcauth` validates under `ProfileKeycloak` with `Audience: "panacea-app-hub"` | any break in steps 2–11 against the real issuer; a browser the control plane never opens |
| `TestTheAuthorizeURL` | the query's key set is **exactly** step 4's ten keys, each value exact; `scope` is the six names in order; `redirect_uri` is `http://127.0.0.1:<the listener's port>/callback`; the listener is bound to `127.0.0.1`, not `0.0.0.0` | a redirect the realm client rejects; a listener reachable from the LAN; any extra parameter |
| `TestTheIdPHintIsSent` | `kc_idp_hint` = `entra`; with `IdPHint: ""` the key is **absent** (not empty) | **a missing `kc_idp_hint`**: Keycloak's login page, a click, comes back |
| `TestTheLoginHintIsSent` | `login_hint` = `grace@example.test`, and devissuer signed in Grace, not its first user; with `LoginHint: ""` the key is **absent** and devissuer's fallback user signs in | **a missing `login_hint`**: Entra's account picker comes back; a guessed hint |
| `TestNoPromptIsSent` | the query has **no** `prompt` key, and no `domain_hint`, `response_mode`, `max_age` or `acr_values`, at every sign-in including one after `Cancel` | **a `prompt` parameter sent** (`select_account` or `login`): Entra re-asks every time |
| `TestTheExchangeAccessTokenIsNeverUsed` | devissuer marks the code-exchange access token; it never reaches `AfterSignIn`'s `Token` answer, and step 9b's request carries `scope=openid panacea-app-hub` | the all-audience token replayed at the Hub (confused deputy) |
| `TestAWrongStateIsIgnored` | a `GET /callback?code=x&state=wrong` → 400, the failure page, still `signing_in`; the real callback then completes | a local page aborting the user's sign-in |
| `TestTheListenerServesOnlyGetCallback` | `POST /callback` → 405; `GET /` and `GET /favicon.ico` → 404; none ends the flow | a side-effecting path on the callback listener |
| `TestTheLoopbackPages` (`listener_test.go`) | success: `200`, `Content-Type: text/html; charset=utf-8`, `Cache-Control: no-store`, the CSP; the body has no `<script`, no `src=`, no `href=`, and its text is exactly *"Signed in. You can return to PanaceaApp."* Failure (`error=access_denied&error_description=<b>x</b>`): `400`, the failure text, and neither `access_denied` nor `<b>` in the body | a script or remote resource on the page; the IdP's error text echoed into HTML |
| `TestAnIssuerErrorEndsTheSignIn` | the scripted browser posts *Cancel* on devissuer's page → `signed_out`, `reason: entra_error`; `Status` holds no description | the IdP's text shown to the user |
| `TestTheSignInTimesOut` | `SignInTimeout: 200 ms`, no callback → `reason: timeout`; the port no longer accepts connections | a listener left open for ever |
| `TestAFailedExchange` | a code altered before the exchange → `reason: exchange_failed`, nothing stored | — |
| `TestTheIdTokenIsChecked` | a fake token endpoint returning an `id_token` with the wrong `nonce`, `aud`, `azp`, `iss`, an expired `exp`, a missing `oid`, a missing `tid` → `exchange_failed` each, nothing stored | A6's checks missing, so a token for another client is accepted |
| `TestTheDiscoveredIssuerMustMatch` | a discovery document whose `issuer` differs from `Issuer` by a trailing `/` → `ErrIssuerUnavailable`, no listener left open, `OpenBrowser` not called | endpoints taken from a document that names another realm |
| `TestANotEnrolledTenantStoresNothing` | `AfterSignIn` → `ErrTenantNotEnrolled` → store empty, `reason: tenant_not_enrolled` | a refresh token kept for a tenant that can never use the Hub |
| `TestATenantMismatchStoresNothing` | `AfterSignIn` → a different tenant id → store empty, `reason: exchange_failed` | A6's cross-check missing |
| `TestAnUnreachableHubStillSignsIn` | `AfterSignIn` → `hubclient.ErrUnavailable` → `signed_in`, the tokens stored | a sign-in that fails because of an unrelated Hub outage |
| `TestSignInWhenSignedInIsRefused` | → `ErrAlreadySignedIn` | two refresh tokens for one account |
| `TestCancelClosesTheListener` | `Cancel()` → `signed_out`, `reason: ""`; the port is closed | — |
| `TestDiscoveryIsCached` | two sign-ins in one process → one discovery request | a discovery fetch per sign-in |
| `TestNoSecretInAnyLog` | across every test, the captured log holds no code, verifier, access token, refresh token, id_token or login hint | T1; a UPN in the log |

`pkce_test.go`: `TestTheRFC7636Vector` — verifier `dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk` →
challenge `E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM` (RFC 7636 Appendix B); and 1,000 generated
verifiers are 43 characters of the base64url alphabet. `idtoken_test.go` covers A6's parser on
hand-built payloads (the same seven failures, without HTTP). `listener_test.go` also holds
`TestOpenSystemBrowserRefusesOtherSchemes`: `file:`, `javascript:`, `ms-settings:` and `http://evil.example/`
are refused before any shell call (a fake `shellExecute` records none); `https://…` and
`http://127.0.0.1:…` reach it (mutant: an OS handler launched from a URL). `upn_windows_test.go`
(`//go:build windows`): `TestWindowsUPNIsReadOnce` — two calls, one underlying call through a
swappable `getUserNameEx`; an `ERROR_NONE_MAPPED` stub → `""` (mutant: a guessed hint such as
`%USERNAME%@<domain>`).

#### Steps

- [ ] **Step 1:** Re-read contract §6.1, §6.2, §9.1; ADR-H1 §5.2; OIDC Core §3.1.3.7; the OD-30
  ruling sections 2-4 (on `main`).
- [ ] **Step 2:** Tests; they fail. Then the ten source files. The silent refresh at launch,
  per-audience refresh and sign-out are task 31: here `Token` returns only the cached
  `HubAudience` token, and `ErrSignedOut` otherwise.
- [ ] **Step 3:** On the owner's laptop, once: `go test ./control-plane/internal/signin/ -run TestWindowsUPNPrintsTheUPN -v`
  (skipped unless `PANACEA_APP_UPN_PROBE=1`) prints the UPN; compare it to `whoami /upn`. This
  settles ruling U-7; if it prints nothing, record it, `login_hint` is omitted and Entra relies on
  the browser session.
- [ ] **Step 4:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/signin/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 25 on Windows (24 elsewhere): 21 in flow_test/listener_test + TestTheRFC7636Vector + the id_token parser test + TestOpenSystemBrowserRefusesOtherSchemes + TestWindowsUPNIsReadOnce
go list -deps ./control-plane/internal/signin | grep panacea-app
#   …/internal/credstore  …/internal/httpx  …/internal/hub/hubapi  …/internal/hub/hubclient  …/internal/ids  …/internal/signin
go test ./control-plane/internal/signin/ -run 'TestNoPromptIsSent|TestTheIdPHintIsSent|TestTheLoginHintIsSent' -count=1 -v 2>&1 | grep -c "^--- PASS"   # 3
```

#### What this task does not claim

- **The realm's behaviour** — loopback redirects on any port, `kc_idp_hint`, the `loginHint`
  forward, the `oid`/`tid` claims in the `id_token`, offline tokens (ruling U-2, U-3, U-5): task 43's
  check against Panacea's dev realm. **Zero clicks on a KPMG device** (the PRT browser session,
  conditional access, consent; ruling U-6): the owner, the same check.
- That `GetUserNameExW` returns the **Entra** UPN on KPMG's devices (ruling U-7): Step 3, one run.
- Bringing the window forward when `Status` turns `signed_in` (`unminimize`, `setFocus`,
  `requestUserAttention`): the renderer, tasks 40 and 41 (ruling section 3, U-11). The renderer's
  15-second fallback link to `authorizeUrl`: task 41.
- The `oidcauth` validator used in `TestSignInEndToEnd` is env task 17 step 0's, taken through the
  orchestrator's merge (ruling section 5); this task writes no validator.
