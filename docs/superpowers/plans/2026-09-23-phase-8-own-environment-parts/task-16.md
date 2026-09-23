### Task 16: `internal/entra` — one Entra access-token validator for the broker and the Hub

The broker must know **who** is calling before it creates a pod (P8-D4), and ADR-H1 made this
package the Hub's validator too: the Hub is a second binary in the control-plane module and
"`control-plane/cmd/broker` imports `control-plane/internal/entra` … HA-1 … is satisfied by
construction: it is one package, not two to reconcile" (ADR-H1 `:206-229`).

**The interface is the Hub contract's.** The Hub contract (sibling `feat/hub`,
`docs/superpowers/contracts/2026-09-23-hub-contract.md` §6.3, `:417-494`) wrote down the interface
the Hub needs, and said: "Phase 8's task 16 owns this package … If task 16 lands first with a
different shape, the Hub adapts to it in one file … the rules in the table must hold either way".
This part adopts that interface **exactly** — the same `Config` fields, `Claims`, `Validator`,
`New`, the ten sentinels and the check order — so the Hub adapts nothing. What phase 8 adds is
**additive**: an HTTP middleware, a readiness call, a stable key, and one sentinel for a key-set
outage, which is an infrastructure failure rather than a rejection.

**No new dependency.** RS256 verification is `crypto/rsa.VerifyPKCS1v15` over `crypto/sha256`; a
JWKS key is two base64url integers. The Hub contract says the same ("Standard library only",
`:426`). A JOSE library would accept algorithms this package must refuse; not having one removes
the key-confusion class instead of configuring it away. `control-plane/go.mod` is unchanged.

**Files:**
- `control-plane/internal/entra/{entra.go,jwks.go,verify.go,middleware.go}` (new)
- `control-plane/internal/entra/{entra_test.go,jwks_test.go,middleware_test.go,testkey_test.go}` (new)

#### Contract (complete) — the Hub contract's §6.3, verbatim, then phase 8's additions

```go
// Package entra validates Microsoft Entra ID v2 access tokens for a resource API.
// Standard library only: RS256 over crypto/rsa, JWKS over net/http.
package entra

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type Config struct {
	// AuthorityHost is "https://login.microsoftonline.com" in every deployed
	// environment, or devissuer's base URL locally. The tenant id is appended.
	AuthorityHost string
	// TenantIDs are the enrolled tenants. A token from any other is ErrTenant.
	TenantIDs []string
	// Audience is the resource API's client id, a GUID: a v2 token's aud.
	Audience string
	// RequiredScope must appear in scp, e.g. "Hub.Access" or "Sandboxes.Access".
	RequiredScope string
	// AllowedClients is the azp allowlist: the desktop's client id.
	AllowedClients []string
	// ClockSkew defaults to 120 s.
	ClockSkew time.Duration
	// AllowInsecureAuthority permits an http AuthorityHost, and only a loopback one.
	AllowInsecureAuthority bool
	HTTPClient             *http.Client // default: 10 s timeout
	Now                    func() time.Time
}

// Claims is what a validated token says. It holds no token text.
type Claims struct {
	TenantID          string   // tid
	ObjectID          string   // oid
	ClientID          string   // azp
	Name              string   // name
	PreferredUsername string   // preferred_username
	Email             string   // email, when the optional claim is configured
	Scopes            []string // scp, split on spaces
	Roles             []string // roles
	ExpiresAt         time.Time
}

type Validator interface {
	Validate(ctx context.Context, bearer string) (Claims, error)
}

func New(cfg Config) (*JWKSValidator, error) // errors on an unusable Config; fetches nothing

// Every rejection wraps exactly one of these. The Hub maps each to details.reason.
var (
	ErrMalformed   = errors.New("entra: malformed token")      // "malformed"
	ErrSignature   = errors.New("entra: bad signature")         // "signature"
	ErrExpired     = errors.New("entra: token expired")         // "expired"
	ErrNotYetValid = errors.New("entra: token not yet valid")   // "not_yet_valid"
	ErrIssuer      = errors.New("entra: wrong issuer")          // "issuer"
	ErrAudience    = errors.New("entra: wrong audience")        // "audience"
	ErrScope       = errors.New("entra: required scope absent") // "scope"
	ErrClient      = errors.New("entra: client not allowed")    // "client"
	ErrVersion     = errors.New("entra: not a v2 token")        // "version"
	ErrTenant      = errors.New("entra: tenant not enrolled")   // 403 tenant_not_enrolled
)
```

```go
// ── Phase 8's additions. All additive; the Hub may use or ignore each. ──

// JWKSValidator implements Validator. Safe for concurrent use.
type JWKSValidator struct{ /* unexported: config, per-tenant key cache, single-flight fetch */ }

var _ Validator = (*JWKSValidator)(nil)

// ErrKeySetUnavailable: the tenant's JWKS could not be fetched. NOT a
// rejection of the token — nobody could be authenticated — so the broker
// answers 503, not 401. It is the only error Validate returns that is not one
// of the ten above.
var ErrKeySetUnavailable = errors.New("entra: key set unavailable")

// Key is "tid/oid", the stable identity. Never sub (pairwise per app), never
// preferred_username (mutable) — ADR-H1 :366.
func (c Claims) Key() string { return c.TenantID + "/" + c.ObjectID }

// Ready fetches the key set of every enrolled tenant not yet fetched and
// returns the first failure (wrapping ErrKeySetUnavailable). The broker's
// /readyz calls it; the Hub's /readyz may (ADR-H1 :528-530).
func (v *JWKSValidator) Ready(ctx context.Context) error

// Middleware authenticates "Authorization: Bearer <token>", stores the Claims
// in the request context, and REMOVES the Authorization header from the
// request it hands on, so no handler can forward the user's token (ADR-H1
// :543-546: no token is forwarded to another service or into a sandbox).
// A missing header: 401 with "WWW-Authenticate: Bearer". A rejection: 401 with
// "WWW-Authenticate: Bearer error=\"invalid_token\"", except ErrTenant, which
// is 403. ErrKeySetUnavailable: 503 backend_unavailable. Bodies are httpx
// problems with code CodeUnauthenticated (401), "tenant_not_enrolled" (403) or
// httpx.CodeBackendUnavailable (503); details.reason carries the Hub's reason
// string for a rejection.
func Middleware(v Validator, next http.Handler) http.Handler

// ClaimsFrom returns the Claims Middleware stored.
func ClaimsFrom(ctx context.Context) (Claims, bool)

// CodeUnauthenticated is the problem code Middleware writes with its 401.
const CodeUnauthenticated = "unauthenticated"
```

**Check order** — the Hub contract's table (`:483-494`), unchanged: shape (three base64url
segments; `alg == "RS256"` and a `kid`; anything else, `none` and every `HS*` included, is
`ErrMalformed` before claims are read) → tenant (`tid` present and enrolled, else `ErrTenant`) →
signature (the JWKS key whose `kid` matches, from `<AuthorityHost>/<tid>/discovery/v2.0/keys`,
cached 24 h per tenant; an unknown `kid` triggers one refetch at most once per 5 min per tenant;
still unknown, or a bad signature, is `ErrSignature`) → `ver == "2.0"` → `iss ==
<AuthorityHost>/<tid>/v2.0` → `aud == Audience` → `now < exp + skew` (`ErrExpired`) and
`nbf − skew ≤ now` (`ErrNotYetValid`) → `azp ∈ AllowedClients` → `scp` contains `RequiredScope`
(a token with `roles` but no `scp` — app-only — fails here, `ErrScope`) → `oid` present and a GUID
(`ErrMalformed`). JWKS keys are used only if `kty == "RSA"`, `use` absent or `"sig"`, and the
modulus is at least 2048 bits. Error messages never contain the token or a signature segment.

#### Tests to write first

`testkey_test.go` generates one 2048-bit key per test binary (`rsa.GenerateKey` once, in a
`sync.Once`), serves `/{tid}/discovery/v2.0/keys` from an `httptest.Server` that counts requests,
and mints tokens with any header and claims; the authority is that server on loopback with
`AllowInsecureAuthority: true`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAValidTokenYieldsItsClaims` | tid, oid, azp, scopes, name, email, `ExpiresAt`; `Key() == "tid/oid"` | a validator that fails everything |
| `TestNoneIsMalformed` | `alg: none` → `ErrMalformed` | the classic `none` bypass |
| `TestHS256SignedWithThePublicKeyIsMalformed` | an HMAC token keyed with the JWKS modulus → `ErrMalformed`, before any fetch | key confusion |
| `TestAMissingKidIsMalformed` | → `ErrMalformed` | "try every key" |
| `TestATamperedPayloadFailsTheSignature` | one claim byte flipped → `ErrSignature` | a verifier over the header only |
| `TestAnUnknownKidRefetchesOnceThenFails` | first → one extra fetch → `ErrSignature`; a second within 5 min → no fetch | a JWKS endpoint hammered by garbage |
| `TestRotationIsPickedUpOnAnUnknownKid` | the server rotates; a new-kid token validates after one refetch | a cache that never refreshes inside 24 h |
| `TestTheKeySetIsCachedFor24Hours` | two validations → one fetch; `Now` + 25 h → a second | a fetch per request |
| `TestConcurrentMissesFetchOnce` | 20 goroutines, cold cache → one fetch | a thundering herd |
| `TestTheTenantIsCheckedBeforeAnyFetch` | tid not enrolled → `ErrTenant`, zero fetches | JWKS fetched for any tenant a caller names |
| `TestExpiryAndNotBeforeHonourTheSkew` | exp = now − 119 s ok; − 121 s → `ErrExpired`; nbf = now + 121 s → `ErrNotYetValid` | skew on one bound only |
| `TestAVersion1TokenIsRejected` | `ver: "1.0"` → `ErrVersion` | v1 tokens, whose iss and aud differ |
| `TestTheIssuerMustBeTheTokensOwnTenant` | tid A, iss of enrolled tenant B → `ErrIssuer` | a token replayed across tenants |
| `TestTheAudienceIsTheClientIDOnly` | `aud: api://<id>` → `ErrAudience`; the Hub's id at a broker validator → `ErrAudience` | a Hub token driving sandboxes (P8-D4) |
| `TestAnUnlistedClientIsRejected` | azp not allowed → `ErrClient` | a token another app obtained for our API |
| `TestScopeMatchIsExact` | `Sandboxes.AccessAll` → `ErrScope`; `"openid Sandboxes.Access"` → ok | a substring match |
| `TestAppOnlyTokensFailTheScopeCheck` | `roles` but no `scp` → `ErrScope` | a daemon credential accepted as a user |
| `TestAMissingOrNonGUIDOidIsMalformed` | → `ErrMalformed` | a principal with no identity |
| `TestSubIsNeverTheIdentity` | same tid/oid, different sub → equal `Key()` | identity keyed on the pairwise sub |
| `TestAKeySetOutageIsNotARejection` | JWKS server 500 → `ErrKeySetUnavailable`, and `errors.Is` false for all ten rejections | a JWKS outage reported as "your token is invalid" |
| `TestErrorsNeverContainTheToken` | for every failure above, the error text holds neither the token nor its signature segment | a token in a log |
| `TestMiddlewareStatuses` | missing → 401 `Bearer`; rejection → 401 `invalid_token` with `details.reason`; `ErrTenant` → 403 `tenant_not_enrolled`; outage → 503 | a 401 that sends a user to re-sign-in during an Entra outage |
| `TestMiddlewareStripsTheAuthorizationHeader` | the handler sees no `Authorization` and a `ClaimsFrom` value | a handler that forwards the user's token into a pod |
| `TestAnInsecureAuthorityIsRefused` | `http://` without the flag → error; `http://example.com` with it → error | a production validator on plaintext |
| `TestReadyFetchesEveryTenant` | two tenants → two fetches; one failing → `ErrKeySetUnavailable` | a /readyz that says ready with no keys |

#### Steps

- [ ] **Step 1:** Re-read Hub contract §6.3. If it changed after this part was written, the
  contract's words win for everything above the "Phase 8's additions" line, and the change is
  recorded in the commit.
- [ ] **Step 2:** `testkey_test.go`, then the tests; they fail to compile.
- [ ] **Step 3:** `entra.go`, `jwks.go`, `verify.go`, `middleware.go`; all pass under `-race`.
- [ ] **Step 4:** `git diff control-plane/go.mod` — empty. Commit.

#### Acceptance

```bash
go test ./control-plane/internal/entra/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 25
go test ./sandboxd/... ./control-plane/... -race -count=1                                 # ok
git diff --stat control-plane/go.mod control-plane/go.sum                                 # empty
```

#### What this task does not claim

- That KPMG's tenant issues tokens in exactly this shape (v2, `azp`, `scp`); the Hub's H1 has one
  real-tenant acceptance test for that (Hub contract `:520-523`).
- Group-overage handling: nothing here reads `groups` (ADR-H1 §6.6 rejects membership from group
  claims).
