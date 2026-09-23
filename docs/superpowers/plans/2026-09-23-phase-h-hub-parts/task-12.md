### Task 12: `devissuer`, part 2 — `/authorize` and `/token`: authorization code with PKCE S256, rotating refresh

**Lane B. Depends on: 11. About 3 hours. Contract task H1.**

The two interactive endpoints of contract §6.4. With them, task 30's real sign-in code runs a whole
PKCE flow on a laptop: the browser opens `/authorize`, the developer clicks a user, the browser is
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

// RevokeRefreshTokens makes every refresh token issued to oid answer invalid_grant.
func (i *Issuer) RevokeRefreshTokens(oid string)

// RequireInteraction makes oid's refresh grants answer interaction_required
// (a conditional-access sign-in frequency, in Entra's terms) until cleared.
func (i *Issuer) RequireInteraction(oid string, on bool)
```

**`GET /<tid>/oauth2/v2.0/authorize`** — query parameters exactly as contract §6.1 step 4 sends
them: `client_id`, `response_type=code`, `response_mode=query`, `redirect_uri`, `scope`, `state`,
`nonce`, `code_challenge`, `code_challenge_method=S256`, `prompt=select_account`, optional
`login_hint`.

Validation, **in this order**. The first two answer a plain 400 HTML page and **never redirect**,
because the redirect target is not yet trusted; the rest redirect to `redirect_uri` with
`?error=…&error_description=…&state=…`, as OAuth 2.0 §4.1.2.1 requires.

| # | Rule | Failure |
|---|---|---|
| 1 | `client_id` is a configured client | 400 page |
| 2 | `redirect_uri` is `http://127.0.0.1:<port>` or `http://localhost:<port>`, any port, path `/` or empty, no query, no fragment | 400 page |
| 3 | `response_type` is `code`; `response_mode` is `query` or absent | `unsupported_response_type` |
| 4 | `code_challenge_method` is `S256` and `code_challenge` is 43 base64url characters | `invalid_request` |
| 5 | `scope` includes `openid`; every other scope is `profile`, `email`, `offline_access`, or `api://<api clientId>/<scope>` for a configured API scope; **at least one** API scope | `invalid_scope` |
| 6 | `state` and `nonce` are present | `invalid_request` |

A valid request with `login_hint` naming a user of this tenant **and** `Auto` set redirects at once
with a code. Otherwise the page lists the tenant's users as buttons, plus *Cancel*. The page is
static HTML with `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline';
form-action 'self'`; each button is a form **POST** back to the same path carrying the query as
hidden fields and `user=<oid>`. *Cancel* posts `cancel=1` and redirects with
`error=access_denied`. A successful POST redirects `302` to `redirect_uri?code=<code>&state=<state>`.

A **code** is 32 random bytes, base64url; single use; lives 5 minutes; bound to the tenant, client,
`redirect_uri`, `code_challenge`, the granted scopes, `nonce`, and the user.

**`POST /<tid>/oauth2/v2.0/token`** — `application/x-www-form-urlencoded`, no client
authentication (a public client). Two grants:

| Grant | Required | Checks | Answer |
|---|---|---|---|
| `authorization_code` | `client_id`, `code`, `redirect_uri`, `code_verifier`; `scope` optional | the code exists, is unused and unexpired, and was issued to this client and this `redirect_uri`; `base64url(SHA-256(code_verifier)) == code_challenge` | the token response below, access token for the **first** API scope granted, with an `id_token` |
| `refresh_token` | `client_id`, `refresh_token`, `scope` | the refresh token exists, is unused, is unexpired, and belongs to this client; the user is not revoked and does not require interaction; `scope` names **one** configured API scope, plus optionally `offline_access` and `openid` | a new access token for **that** API (a different resource from the first grant is allowed: one refresh token serves every consented API, contract §6.2); a **new** refresh token, and the old one is spent; an `id_token` only when `openid` is in `scope` |

```json
{ "token_type": "Bearer",
  "scope": "api://33333333-3333-4333-8333-333333333333/Hub.Access openid profile email offline_access",
  "expires_in": 3600,
  "ext_expires_in": 3600,
  "access_token": "<JWS>",
  "refresh_token": "<43 base64url characters>",
  "id_token": "<JWS>" }
```

Errors are `400 {"error":"<code>","error_description":"<one sentence>"}` with these codes:
`invalid_request` (a missing parameter), `invalid_client` (an unknown `client_id`),
`invalid_grant` (a bad, spent, expired or mis-bound code or refresh token; a wrong or missing
verifier; a mismatched `redirect_uri`; a revoked user), `interaction_required` (the hook),
`unsupported_grant_type`, `invalid_scope`. **Stricter than Entra, on purpose, and listed in
contract §6.4:** a spent refresh token is refused at once, where Entra keeps honouring an old one
for a while.

#### Tests to write first

Each runs the issuer on `httptest.NewServer(issuer.Handler())` with `BaseURL` set to the server's
URL, and an injected clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheUserPageListsTheTenant` | GET with valid parameters → 200 HTML listing the four users of tenant 1 and not tenant 2's; the CSP header above | a page that signs in anyone from any tenant |
| `TestAnUntrustedRedirectIsNeverFollowed` | `redirect_uri` of `https://127.0.0.1:1/`, `http://evil.example:1/`, `http://127.0.0.1:1/cb`, `http://127.0.0.1:1/?x=1` → 400 page, **no `Location` header** | an open redirector carrying a code to another origin |
| `TestAnyLoopbackPortIsAccepted` | `http://127.0.0.1:53123/` and `http://localhost:1/` → the page | a registration rule that only matches one port (the rule contract §6.1 relies on) |
| `TestS256IsRequired` | `plain`, a missing challenge, a 42-character challenge → redirect with `invalid_request` and the `state` | a flow that works without PKCE locally and fails on Entra |
| `TestScopeRules` | no `openid`, an unknown API scope, no API scope at all → `invalid_scope` | — |
| `TestAutoWithLoginHintSkipsThePage` | `Auto` + `login_hint=grace@example.test` → 302 with a code; without `Auto` → the page | an end-to-end test that needs a human |
| `TestCancelIsAccessDenied` | POST `cancel=1` → 302 with `error=access_denied` and the `state` | task 30's `entra_error` path untestable |
| `TestTheCodeExchangeIssuesValidTokens` | code → token response; `entra` validates the access token for the Hub API; the `id_token`'s `aud` is the client and its `nonce` is the request's | a token the Hub refuses |
| `TestAWrongVerifierIsInvalidGrant` | a different verifier, or none → `invalid_grant`; the code is then spent | PKCE checked after the code is consumed, allowing a guess per code |
| `TestACodeIsSingleUseAndShortLived` | a second exchange → `invalid_grant`; an exchange after 5 min 1 s → `invalid_grant` | a replayable code |
| `TestTheRedirectURIMustMatch` | the exchange with a different port than the authorize request → `invalid_grant` | a code sent to one listener and redeemed by another |
| `TestRefreshRotates` | refresh → a new refresh token; the old one → `invalid_grant` | a refresh token that never rotates, so task 31's write-before-return is never exercised |
| `TestOneRefreshTokenServesAnotherResource` | refresh with `scope=api://55555555-…/Sandboxes.Access offline_access` → an access token whose `aud` is the broker and which carries no `roles` | phase 8's broker needing a second sign-in |
| `TestTheHooks` | after `RevokeRefreshTokens(grace)` → `invalid_grant`; with `RequireInteraction(grace, true)` → `interaction_required` | task 31's `reauth_required` path untestable |
| `TestUnknownClientIsInvalidClient` | → `invalid_client` | — |

#### Steps

- [ ] **Step 1:** Re-read contract §6.1 (steps 4–8) and §6.4.
- [ ] **Step 2:** Tests; they fail.
- [ ] **Step 3:** `authorize.go`, `page.go`, `token.go`; register the routes in `issuer.go`.
- [ ] **Step 4:** By hand: `go run ./control-plane/cmd/devissuer`, then open
  `http://127.0.0.1:8740/11111111-1111-4111-8111-111111111111/oauth2/v2.0/authorize?client_id=22222222-2222-4222-8222-222222222222&response_type=code&redirect_uri=http://127.0.0.1:9/&scope=openid%20api://33333333-3333-4333-8333-333333333333/Hub.Access&state=s&nonce=n&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256`
  in a browser: the page lists Ada, Grace, Linus and Out Sider; clicking one lands on
  `http://127.0.0.1:9/?code=…&state=s` (the connection fails; the URL is the evidence).
- [ ] **Step 5:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/devissuer/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 25 (10 from task 11 + 15)
go vet ./control-plane/internal/devissuer/                                                  # no output
```

#### What this task does not claim

- Entra's behaviour under conditional access, MFA or consent — none of it is modelled (contract
  §6.4's list of known differences).
- That the page looks like anything: it is a developer tool with no design-system obligations,
  and nothing in the product links to it.
