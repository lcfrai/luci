### Task 12: `devissuer`, part 2 — `/auth` and `/token`: authorization code with PKCE S256, offline refresh with scope narrowing, client credentials (OD-30)

**Lane B. Depends on: 11. About 4 hours. Contract task H1.**

**OD-30** (main `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`, "the ruling"; Hub
contract §17, which wins over §6.1 and §6.4). Under the default **`keycloak`** profile (task 11) the two
endpoints are the realm's: `<iss>/protocol/openid-connect/auth` and `<iss>/protocol/openid-connect/token`,
with `<iss>` = `<BaseURL>/realms/<Realm>` (task 11). They model what the ruling asks of Panacea's realm
(ruling §2, §10 K1): the public client `panacea-app-desktop`, redirect `http://127.0.0.1:<port>/callback`
only, `kc_idp_hint` **accepted and ignored**, `login_hint` selecting the dev user, `offline_access`
refresh tokens **that are never rotated or spent** (Panacea's `revokeRefreshToken:false`), a narrowing
`scope` on refresh, and the `client_credentials` grant for the confidential clients (ruling §6). Under
`-profile entra` the earlier `/<tid>/oauth2/v2.0/…` endpoints and rules stand unchanged, for the
entra-profile end-to-end row only; the tables below are the `keycloak` profile's.

The two interactive endpoints of contract §6.4. With them, task 30's real sign-in code runs a whole
PKCE flow on a laptop: the browser opens `/auth`, the developer clicks a user, the browser is
sent to the control plane's one-shot listener with a code, and the control plane exchanges it.
`login_hint` + `PANACEA_APP_DEVISSUER_AUTO=true` skip the click, for end-to-end tests. Two test
hooks let task 31 prove its failure paths: revoking a user's refresh tokens, and demanding
interaction.

**Files:**
- `control-plane/internal/devissuer/{authorize.go,token.go,page.go}` + tests (new)
- `control-plane/internal/devissuer/issuer.go` — the routes and the two hooks (modify)

#### Contract — complete

```go
// control-plane/internal/devissuer/issuer.go — added by this task.

// RevokeRefreshTokens ends every offline session of oid, as a realm admin's
// "revoke offline sessions" (or the 7-day offline cap, ruling §6) does: each
// refresh token issued to oid then answers invalid_grant. Both profiles.
func (i *Issuer) RevokeRefreshTokens(oid string)

// RequireInteraction makes oid's refresh grants answer interaction_required
// (a conditional-access sign-in frequency, in Entra's terms) until cleared.
// entra profile only: Keycloak never answers interaction_required on a refresh.
func (i *Issuer) RequireInteraction(oid string, on bool)
```

**`GET <iss>/protocol/openid-connect/auth`** — query parameters exactly as contract §17 §6.1 step 4
sends them: `client_id`, `response_type=code`, `redirect_uri`, `scope`, `state`, `nonce`,
`code_challenge`, `code_challenge_method=S256`, optional `kc_idp_hint` (accepted, ignored: the realm's
browser flow goes to Entra whether or not it is sent, ruling K1), optional `login_hint`. No `prompt`,
no `response_mode`.

Validation, **in this order**. The first two answer a plain 400 HTML page and **never redirect**,
because the redirect target is not yet trusted; the rest redirect to `redirect_uri` with
`?error=…&error_description=…&state=…`, as OAuth 2.0 §4.1.2.1 requires.

| # | Rule | Failure |
|---|---|---|
| 1 | `client_id` is a configured client | 400 page |
| 1b | the client is **public** (`panacea-app-desktop`); a confidential client has no standard flow (ruling §6) | 400 page |
| 2 | `redirect_uri` is exactly `http://127.0.0.1:<port>/callback`, any port, no query, no fragment; never `localhost`, never another path (ruling K1 `redirectUris`, review-1 M3) | 400 page |
| 3 | `response_type` is `code`; `response_mode` is `query` or absent | `unsupported_response_type` |
| 4 | `code_challenge_method` is `S256` and `code_challenge` is 43 base64url characters | `invalid_request` |
| 5 | `scope` includes `openid`; every other scope is `profile`, `email`, `offline_access`, or one of `Config.Audiences` (`panacea-app-hub`, `-broker`, `-gateway`, `-fleet`); **at least one** audience scope | `invalid_scope` |
| 6 | `state` and `nonce` are present | `invalid_request` |

A valid request with `login_hint` naming a user of this tenant **and** `Auto` set redirects at once
with a code. Otherwise the page lists the tenant's users as buttons, plus *Cancel*. The page is
static HTML with `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline';
form-action 'self' <listener origin>`, where `<listener origin>` is the scheme, host and port of
the `redirect_uri` that rule 2 has already accepted (`http://127.0.0.1:53123` for
`redirect_uri=http://127.0.0.1:53123/callback`) — computed per request, never a wildcard, never a port
other than that one. **Why the listener's origin is in `form-action`:** each button is a form
**POST** back to the same path, and the POST answers `302` to the listener, a different origin
(another port). Chromium, and so Edge, the corporate system browser, applies `form-action` to a
redirect that follows a form submission; with `form-action 'self'` alone the click is refused
with a console error and the tab never reaches the listener (review I4). Dropping `form-action`
would fix the click but lets the page post anywhere (`form-action` does not fall back to
`default-src`); `http://127.0.0.1:*` would admit every loopback port. The exact origin admits the
one redirect this page exists to perform. Each button carries the query as hidden fields and
`user=<oid>`. *Cancel* posts `cancel=1` and redirects with `error=access_denied`. A successful POST
redirects `302` to `redirect_uri?code=<code>&state=<state>`. **Unverified here:** Chromium's
redirect behaviour; step 4 is the evidence, and no Go test can stand in for it.

A **code** is 32 random bytes, base64url; single use; lives 5 minutes; bound to the tenant, client,
`redirect_uri`, `code_challenge`, the granted scopes, `nonce`, and the user.

**`POST <iss>/protocol/openid-connect/token`** — `application/x-www-form-urlencoded`. The desktop is
public (no client authentication); a confidential client authenticates with its `devSecret` by
`client_secret_basic` or `client_secret_post` (task 11's discovery document). Three grants:

| Grant | Required | Checks | Answer |
|---|---|---|---|
| `authorization_code` | `client_id`, `code`, `redirect_uri`, `code_verifier` | the code exists, is unused and unexpired, and was issued to this client and this `redirect_uri`; `base64url(SHA-256(code_verifier)) == code_challenge` | the token response below: an access token whose `aud` holds **every** granted audience scope then `"account"` (what Keycloak does, and why the control plane discards it, ruling §2 step 7); an `id_token`; an **offline** refresh token when `offline_access` was granted |
| `refresh_token` | `client_id`, `refresh_token`, `scope` | the refresh token exists, is unexpired, belongs to this client, and its user's offline session is live (`RevokeRefreshTokens`); `scope` is `openid` plus **one** audience scope **that the presented token's own scope holds**, else `invalid_scope` | an access token for **that** audience alone (`aud` = `[<scope>, "account"]`; ruling §2 step 7, U-4). The presented token is **not** spent and keeps working (Panacea's `revokeRefreshToken:false`, facts `:94-96`). A refresh token is also returned, as Keycloak does, and its scope is the **narrowed** one: the adverse case of ruling U-4b, pinned on purpose so that a task 31 which writes it back fails its next `Token(hub)` with `invalid_scope` (ruling U-4b: task 31 keeps the original) |
| `client_credentials` | client authentication, `scope` | the client is confidential with `serviceAccountsEnabled`; the secret matches its `devSecret`; `scope` is **one** of that client's `audiences` (Fleet: hub or broker; the gateway: hub) | `MintServiceToken(<client>, <scope>)` (task 11): no `oid`, no `tid`, no refresh token, no `id_token` |

```json
{ "token_type": "Bearer",
  "scope": "openid profile email offline_access panacea-app-hub",
  "expires_in": 3600,
  "access_token": "<JWS>",
  "refresh_token": "<43 base64url characters>",
  "id_token": "<JWS>" }
```

Errors are `400 {"error":"<code>","error_description":"<one sentence>"}` with these codes:
`invalid_request` (a missing parameter), `invalid_client` (an unknown `client_id`, a wrong secret),
`unauthorized_client` (a public client asking for `client_credentials`),
`invalid_grant` (a bad, expired or mis-bound code or refresh token; a spent code; a wrong or missing
verifier; a mismatched `redirect_uri`; a revoked offline session), `interaction_required` (the hook,
`entra` only), `unsupported_grant_type`, `invalid_scope`. Under `-profile entra` refresh tokens still
rotate and a spent one is refused at once (the earlier rule, contract §6.4).

#### Tests to write first

Each runs the issuer on `httptest.NewServer(issuer.Handler())` with `BaseURL` set to the server's
URL, and an injected clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheUserPageListsTheTenant` | GET with valid parameters → 200 HTML listing the five users of tenant 1 and not tenant 2's; the CSP header above | a page that signs in anyone from any tenant |
| `TestTheFormMayPostOnlyToTheListener` | `redirect_uri=http://127.0.0.1:53123/callback` → the CSP's `form-action` is exactly `'self' http://127.0.0.1:53123`; `redirect_uri=http://127.0.0.1:1/callback` → exactly `'self' http://127.0.0.1:1`; neither contains `*` or another port; `default-src 'none'` is still present | `form-action 'self'` alone (review I4: the redirect to the listener is blocked in Edge); a dropped `form-action`; `http://127.0.0.1:*` |
| `TestAnUntrustedRedirectIsNeverFollowed` | `redirect_uri` of `https://127.0.0.1:1/callback`, `http://evil.example:1/callback`, `http://localhost:1/callback`, `http://127.0.0.1:1/`, `http://127.0.0.1:1/cb`, `http://127.0.0.1:1/callback?x=1`; and `client_id=panacea-app-fleet` → 400 page, **no `Location` header** | an open redirector carrying a code to another origin or path (review-1 M3); a `localhost` redirect the realm never registers; a confidential client given a code |
| `TestAnyLoopbackPortIsAccepted` | `http://127.0.0.1:53123/callback` and `http://127.0.0.1:1/callback` → the page | a registration rule that only matches one port (the rule ruling §2 relies on) |
| `TestKcIdpHintIsIgnored` | the same request with and without `kc_idp_hint=entra`, and with `kc_idp_hint=other` → the same page (or, with `Auto` + `login_hint`, the same redirect) | local code that depends on the hint, when the realm's flow override makes it cosmetic (ruling K1, review-1 I1) |
| `TestS256IsRequired` | `plain`, a missing challenge, a 42-character challenge → redirect with `invalid_request` and the `state` | a flow that works without PKCE locally and fails on the realm |
| `TestScopeRules` | no `openid`, an unknown scope, an Entra `api://…/Hub.Access` scope, no audience scope at all → `invalid_scope` | — |
| `TestAutoWithLoginHintSkipsThePage` | `Auto` + `login_hint=grace@example.test` → 302 with a code; without `Auto` → the page | an end-to-end test that needs a human |
| `TestCancelIsAccessDenied` | POST `cancel=1` → 302 with `error=access_denied` and the `state` | task 30's `entra_error` path untestable |
| `TestTheCodeExchangeIssuesValidTokens` | code (all four audience scopes + `offline_access`) → token response; the access token's `aud` holds all four, so `oidcauth` under `keycloak` with `Audience: panacea-app-hub` **refuses** it (`ErrAudience`, ruling §5); the `id_token`'s `aud` is `panacea-app-desktop` and its `nonce` is the request's; a refresh token is present | a code-exchange token the Hub would accept (the confused deputy, review-1 I2); a missing offline token |
| `TestAWrongVerifierIsInvalidGrant` | a different verifier, or none → `invalid_grant`; the code is then spent | PKCE checked after the code is consumed, allowing a guess per code |
| `TestACodeIsSingleUseAndShortLived` | a second exchange → `invalid_grant`; an exchange after 5 min 1 s → `invalid_grant` | a replayable code |
| `TestTheRedirectURIMustMatch` | the exchange with a different port than the authorize request → `invalid_grant` | a code sent to one listener and redeemed by another |
| `TestAnOfflineTokenIsNeverSpent` | the code's refresh token R, used five times in turn for `panacea-app-hub`, `-broker`, `-gateway`, `-fleet`, `-hub` → five access tokens, each `aud` exactly `[<scope>, "account"]`; R still works after all five | a devissuer that rotates, when the realm does not (`revokeRefreshToken:false`) |
| `TestANarrowedRefreshTokenCannotWiden` | the refresh token returned by R's `panacea-app-broker` grant has `scope` `openid panacea-app-broker`; using it for `panacea-app-hub` → `invalid_scope`; R for `panacea-app-hub` still succeeds | a task 31 that writes back a narrowed grant's token passing locally (ruling U-4b, review-1 I4) |
| `TestClientCredentials` | `panacea-app-fleet` + its `devSecret`, `scope=panacea-app-hub` → a service token with no `oid`, no `tid`, no refresh token; a wrong secret → `invalid_client`; `scope=panacea-app-gateway` → `invalid_scope`; `panacea-app-desktop` → `unauthorized_client` | a public client minting a service token; a service reaching an audience it was not given (ruling §6) |
| `TestTheHooks` | after `RevokeRefreshTokens(grace)` → R answers `invalid_grant` under both profiles; under `-profile entra`, `RequireInteraction(grace, true)` → `interaction_required` | task 31's sign-out path untestable |
| `TestUnknownClientIsInvalidClient` | → `invalid_client` | — |

#### Steps

- [ ] **Step 1:** Re-read contract §6.1 (steps 4–8), §6.4 and **§17** (which wins); the ruling §2, §6
  and §12 U-4/U-4b.
- [ ] **Step 2:** Tests; they fail.
- [ ] **Step 3:** `authorize.go`, `page.go`, `token.go`; register the routes in `issuer.go`.
- [ ] **Step 4:** By hand: `go run ./control-plane/cmd/devissuer`, then open
  `http://127.0.0.1:8740/realms/panacea-app-dev/protocol/openid-connect/auth?client_id=panacea-app-desktop&response_type=code&redirect_uri=http://127.0.0.1:53123/callback&scope=openid%20offline_access%20panacea-app-hub&state=s&nonce=n&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256&kc_idp_hint=entra`
  **in Edge** (the corporate system browser, the one task 30 opens): the page lists Ada, Grace,
  Linus, Out Sider and Vera. **In Edge, clicking Grace lands on the listener URL**
  `http://127.0.0.1:53123/callback?code=…&state=s` — the address bar shows it (the connection fails,
  since nothing listens; the URL is the evidence), and DevTools' console shows **no**
  `Content-Security-Policy` / `form-action` violation. A tab that stays on the page with such a
  console error is review I4 back: stop and fix the CSP. Port 53123, not a low port: Chromium
  refuses its "unsafe" ports (9, among others) with `ERR_UNSAFE_PORT`, which would mask the result.
- [ ] **Step 5:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/devissuer/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 27 (task 11's 9 in this package + 18; its other 2 are in cmd/devissuer; re-count task 11's after its OD-30 amendment)
go vet ./control-plane/internal/devissuer/                                                  # no output
```

#### What this task does not claim

- Entra's behaviour under conditional access, MFA or consent, or the realm's broker hop to Entra —
  none of it is modelled (contract §6.4's list of known differences).
- What the real realm does on a narrowed refresh (ruling U-4, U-4b): hub task 43 settles it against
  Panacea's dev realm; this task models the adverse case on purpose.
- That the page looks like anything: it is a developer tool with no design-system obligations,
  and nothing in the product links to it.
