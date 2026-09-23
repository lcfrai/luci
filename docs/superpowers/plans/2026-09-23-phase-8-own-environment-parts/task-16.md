### Task 16: `internal/entra` — one Entra access-token validator for the broker and the Hub

The broker must know **who** is calling before it creates a pod (P8-D4), and ADR-H1 made this
package the Hub's validator too: the Hub is a second binary in the control-plane module and
"`control-plane/cmd/broker` imports `control-plane/internal/entra` … HA-1 … is satisfied by
construction: it is one package, not two to reconcile" (ADR-H1 `:206-229`). ADR-H1 also fixed the
rules (`:354-366`); this task implements **exactly** that table, parameterised so each server
passes its own audience, scope and allowed clients. If the Hub's task H1 reaches this package
first, it writes it "to phase 8's interface" (ADR-H1 `:557`) — the interface below.

**No new dependency.** RS256 verification is `crypto/rsa.VerifyPKCS1v15` over `crypto/sha256`,
and a JWKS key is two base64url integers; the whole verifier is standard library. The ADR's
devissuer and token exchange are standard library for the same reason (ADR-H1 `:377-380`,
`:532-535`). A JOSE library would accept algorithms this package must refuse; not having one
removes the key-confusion class instead of configuring it away. `control-plane/go.mod` is
unchanged.

**Files:**
- `control-plane/internal/entra/{entra.go,jwks.go,verify.go,middleware.go}` (new)
- `control-plane/internal/entra/{entra_test.go,jwks_test.go,middleware_test.go}` and `testkey_test.go` — an in-test RSA key and JWKS server (new)

#### Contract (complete)

```go
// Package entra validates Microsoft Entra ID v2.0 access tokens for our two
// resource APIs, the broker and the Hub. It is the only place either server
// decides who a caller is. Rules: ADR-H1 §5.2's validation table.
package entra

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Principal is who a validated token speaks for. (TenantID, ObjectID) is the
// identity; every other field is for display and audit only.
type Principal struct {
	TenantID          string   // tid
	ObjectID          string   // oid
	ClientID          string   // azp — the app that obtained the token
	Name              string   // name; display only
	PreferredUsername string   // preferred_username; display only, mutable, never a key
	Scopes            []string // scp, split on single spaces
	Roles             []string // roles
}

// Key is "tid/oid", the stable identity string. Never sub (pairwise per app),
// never preferred_username (mutable).
func (p Principal) Key() string { return p.TenantID + "/" + p.ObjectID }

// Config is one server's acceptance policy.
type Config struct {
	// Authority is the issuer base: "https://login.microsoftonline.com" for
	// Entra. A tenant's issuer is Authority+"/"+tid+"/v2.0" and its JWKS is
	// Authority+"/"+tid+"/discovery/v2.0/keys".
	Authority string
	// Tenants is the set of enrolled tenant ids. A token whose tid is absent is
	// ErrTenantNotEnrolled.
	Tenants []string
	// Audience is the API's client id — a v2 token's aud is the GUID, not the
	// api:// URI.
	Audience string
	// RequiredScope must appear, exactly, as one element of scp.
	RequiredScope string
	// AllowedClients are the accepted azp values (the desktop's client id).
	AllowedClients []string
	// ClockSkew applies to exp and nbf. Zero means 120 s.
	ClockSkew time.Duration
	// JWKSMaxAge is how long a fetched key set is trusted. Zero means 24 h.
	JWKSMaxAge time.Duration
	// UnknownKidRefetch is the minimum gap between refetches triggered by an
	// unknown kid, per tenant. Zero means 5 minutes.
	UnknownKidRefetch time.Duration
	// HTTPClient fetches key sets. nil means a client with a 10 s timeout.
	HTTPClient *http.Client
	// Now is the clock. nil means time.Now.
	Now func() time.Time
	// AllowInsecureAuthority permits an http:// Authority. Only the Hub's local
	// devissuer on loopback needs it (ADR-H1 :389-390); NewValidator refuses an
	// http:// authority whose host is not 127.0.0.1 or localhost even with it.
	AllowInsecureAuthority bool
}

// Validator is safe for concurrent use.
type Validator struct{ /* unexported: config, per-tenant key cache, single-flight fetch */ }

// NewValidator checks cfg (non-empty Authority, Tenants, Audience,
// RequiredScope and AllowedClients; https unless allowed) and returns a
// Validator. It fetches nothing.
func NewValidator(cfg Config) (*Validator, error)

// Validate verifies raw (a JWS compact string, no "Bearer " prefix) and returns
// its Principal, or an error wrapping exactly one of the sentinels below. Error
// messages never contain the token or any claim value longer than an id.
func (v *Validator) Validate(ctx context.Context, raw string) (Principal, error)

// Ready fetches the key set of every enrolled tenant not yet fetched, and
// reports the first failure. The Hub's /readyz and the broker's call it.
func (v *Validator) Ready(ctx context.Context) error

// Middleware authenticates "Authorization: Bearer <token>" and stores the
// Principal in the request context. On failure it answers 401 with
// "WWW-Authenticate: Bearer error=\"invalid_token\"" and an httpx problem body
// {"error":"unauthenticated","message":…}; a missing header gets
// "WWW-Authenticate: Bearer" with no error attribute (RFC 6750 §3.1). It never
// passes the request on, and it removes the Authorization header from the
// request it hands to next, so no handler can forward the token (ADR-H1 :545).
func (v *Validator) Middleware(next http.Handler) http.Handler

// PrincipalFrom returns the Principal Middleware stored.
func PrincipalFrom(ctx context.Context) (Principal, bool)

// CodeUnauthenticated is the problem code Middleware writes with its 401. It is
// not in httpx's set because the control plane itself never authenticates a
// caller this way; the broker and the Hub both use this constant.
const CodeUnauthenticated = "unauthenticated"

var (
	ErrMalformed         = errors.New("entra: not a well-formed JWS")
	ErrAlgorithm         = errors.New("entra: algorithm is not RS256")
	ErrNoKeyID           = errors.New("entra: header has no kid")
	ErrUnknownKey        = errors.New("entra: kid is not in the tenant's key set")
	ErrSignature         = errors.New("entra: signature does not verify")
	ErrVersion           = errors.New("entra: not a v2.0 access token")
	ErrExpired           = errors.New("entra: expired or not yet valid")
	ErrTenantNotEnrolled = errors.New("entra: tenant is not enrolled")
	ErrIssuer            = errors.New("entra: issuer does not match the token's tenant")
	ErrAudience          = errors.New("entra: audience is not this API")
	ErrAppOnly           = errors.New("entra: app-only tokens are not accepted")
	ErrScope             = errors.New("entra: required scope is missing")
	ErrClient            = errors.New("entra: issued to a client that is not allowed")
	ErrNoPrincipal       = errors.New("entra: tid or oid is missing")
	ErrKeySetUnavailable = errors.New("entra: the tenant's key set could not be fetched")
)
```

**Order of checks** (each failure returns immediately, so a cheap check never waits on a fetch):
split into three base64url parts → header JSON: `alg == "RS256"` (anything else, including
`none` and every `HS*`, is `ErrAlgorithm`) and `kid` present → payload JSON (unverified, only to
read `tid`) → `tid` enrolled → key set for that tenant (cache; on an unknown `kid`, refetch if the
last refetch for that tenant is older than `UnknownKidRefetch`, single-flight) → RS256 signature
over `header.payload` → `ver == "2.0"` → `exp`/`nbf` with skew → `iss ==
Authority+"/"+tid+"/v2.0"` → `aud == Audience` → app-only (`idtyp == "app"`, or no `scp`) →
`scp` contains `RequiredScope` → `azp ∈ AllowedClients` → `tid`, `oid` non-empty. JWKS keys are
used only if `kty == "RSA"` and `use` is absent or `"sig"`; a key with `n` shorter than 2048 bits
is skipped.

#### Tests to write first

`testkey_test.go` generates one 2048-bit key per test binary (`rsa.GenerateKey`, once, in a
`sync.Once`), serves `/{tid}/discovery/v2.0/keys` from an `httptest.Server` that counts requests,
and mints tokens with any header and claims. The authority is that server with
`AllowInsecureAuthority: true` (loopback).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAValidTokenYieldsItsPrincipal` | tid, oid, azp, scopes, name; `Key() == "tid/oid"` | a validator that fails everything |
| `TestNoneIsRejected` | `alg: none`, no signature → `ErrAlgorithm` | the classic `none` bypass |
| `TestHS256SignedWithThePublicKeyIsRejected` | a token HMAC-signed with the JWKS modulus bytes → `ErrAlgorithm` | key confusion (an HMAC check keyed by the public key) |
| `TestAMissingKidIsRejected` | → `ErrNoKeyID` | "try every key", which lets a stale key verify forever |
| `TestATamperedPayloadFailsTheSignature` | flip one claim byte → `ErrSignature` | a verifier over the header only |
| `TestAnUnknownKidRefetchesOnceThenRejects` | first unknown kid → one extra fetch → `ErrUnknownKey`; a second within 5 min → no fetch | a JWKS endpoint hammered by garbage tokens |
| `TestRotationIsPickedUpOnAnUnknownKid` | the server rotates to a new key; a token with the new kid validates after one refetch | a cache that never refreshes before 24 h |
| `TestTheKeySetIsCachedFor24Hours` | two validations → one fetch; `Now` + 25 h → a second fetch | a fetch per request |
| `TestConcurrentMissesFetchOnce` | 20 goroutines, cold cache → one fetch | a thundering herd |
| `TestExpiryHonoursTheSkew` | exp = now − 119 s accepted; now − 121 s → `ErrExpired`; nbf = now + 121 s → `ErrExpired` | skew applied only to exp, or not at all |
| `TestAVersion1TokenIsRejected` | `ver: "1.0"` → `ErrVersion` | v1 tokens, whose iss and aud differ |
| `TestAnUnenrolledTenantIsRejectedBeforeAnyFetch` | tid not in Tenants → `ErrTenantNotEnrolled`, zero fetches | fetching JWKS for any tenant a caller names |
| `TestTheIssuerMustBeTheTokensOwnTenant` | tid A (enrolled), iss for tenant B (enrolled) → `ErrIssuer` | a token minted in one tenant replayed as another |
| `TestTheAudienceIsTheClientIDOnly` | aud = `api://<id>` → `ErrAudience`; aud = the Hub's id at the broker → `ErrAudience` | a Hub token driving sandboxes (P8-D4's separate audiences) |
| `TestScopeMatchIsExact` | scp `Sandboxes.AccessAll` → `ErrScope`; `"openid Sandboxes.Access"` → ok | a substring match |
| `TestAppOnlyTokensAreRejected` | `idtyp: app` with roles, no scp → `ErrAppOnly` | a daemon credential accepted as a user |
| `TestAnUnlistedClientIsRejected` | azp not in AllowedClients → `ErrClient` | a token some other app obtained for our API |
| `TestSubIsNeverTheIdentity` | two tokens, same tid/oid, different sub → equal `Key()` | identity keyed on the pairwise sub |
| `TestErrorsNeverContainTheToken` | for every failure above, `err.Error()` does not contain the raw token or its signature segment | a token written to a log by an error message |
| `TestMiddlewareAnswers401WithTheChallenge` | bad token → 401, `WWW-Authenticate: Bearer error="invalid_token"`, problem code `unauthenticated`; missing header → `WWW-Authenticate: Bearer` | a 403, or a handler reached |
| `TestMiddlewareStripsTheAuthorizationHeader` | the handler sees no `Authorization` header and a Principal in context | a handler that forwards the user's token into a pod |
| `TestAnInsecureAuthorityIsRefused` | `http://` without the flag → error; `http://example.com` with it → error | a production validator pointed at plaintext |
| `TestReadyFetchesEveryTenant` | two tenants → two fetches; one failing → `ErrKeySetUnavailable` | a /readyz that says ready with no keys |

#### Steps

- [ ] **Step 1:** Re-read the Hub contract, if it now exists, for any change to ADR-H1's table; if it
  differs, the contract wins and the difference is recorded in the commit.
- [ ] **Step 2:** `testkey_test.go`, then the tests; they fail to compile.
- [ ] **Step 3:** `entra.go` (types, config checks), `jwks.go` (cache, single-flight, rotation),
  `verify.go` (the check order), `middleware.go`. All tests pass under `-race`.
- [ ] **Step 4:** `git diff control-plane/go.mod` — empty. Commit.

#### Acceptance

```bash
go test ./control-plane/internal/entra/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 23
go test ./sandboxd/... ./control-plane/... -race -count=1                                 # ok
git diff --stat control-plane/go.mod control-plane/go.sum                                 # empty
```

#### What this task does not claim

- That KPMG's tenant issues tokens in exactly this shape (v2, `azp`, `scp`, `idtyp`) — ADR-H1's R2
  and the first real sign-in are where that is proven; the devissuer mimics the shape.
- Group-overage handling: no rule here reads `groups` (ADR-H1 §6.6 rejects membership from group
  claims).
