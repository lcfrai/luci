### Task 16: `internal/entra` — one Entra access-token validator for the broker and the Hub

The broker must know **who** is calling before it creates a pod (P8-D4), and ADR-H1 made this
package the Hub's validator too: the Hub is a second binary in the control-plane module and
"`control-plane/cmd/broker` imports `control-plane/internal/entra` … HA-1 … is satisfied by
construction: it is one package, not two to reconcile" (ADR-H1 §4.1, `67609ab` `:206-229`; the
plan's "How ADR-H1 is cited" pins every ADR line number to that commit).

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
	ErrMalformed   = errors.New("entra: malformed token")       // "malformed"
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
// preferred_username (mutable) — ADR-H1 §5.2 (67609ab :366).
func (c Claims) Key() string { return c.TenantID + "/" + c.ObjectID }

// Ready fetches the key set of every enrolled tenant not yet fetched and
// returns the first failure (wrapping ErrKeySetUnavailable). The broker's
// /readyz calls it; the Hub's /readyz may (ADR-H1 §5.7, 67609ab :528-530).
// Tenants are fetched concurrently; the first failure is in TenantIDs order.
func (v *JWKSValidator) Ready(ctx context.Context) error

// Middleware authenticates "Authorization: Bearer <token>", stores the Claims
// in the request context, and REMOVES the Authorization header from the
// request it hands on, so no handler can forward the user's token (ADR-H1
// §5.8, 67609ab :543-546: no token is forwarded to another service or into a
// sandbox).
//
// No Bearer credential (no Authorization header, another scheme, or an empty
// token): 401, a bare "WWW-Authenticate: Bearer" (no error code, RFC 6750
// §3.1), details.reason "missing". A rejection: 401, `Bearer
// error="invalid_token"` — `insufficient_scope` for ErrScope (Hub contract
// §4.1) — with details.reason; ErrTenant is 403 tenant_not_enrolled with
// details.tenantId when the validator named a GUID tenant (§4.2).
// ErrKeySetUnavailable: 503 backend_unavailable. An error that is none of
// these: 500 internal. Bodies are httpx problems with code
// CodeUnauthenticated (401), tenant_not_enrolled (403),
// httpx.CodeBackendUnavailable (503) or httpx.CodeInternal (500), and never
// hold the token or the error chain.
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
| `TestAnUnusableConfigIsRefused` *(added by the build)* | no authority, a path, query, fragment or userinfo on it; no or non-GUID tenant; no or `api://` audience; no or two-word scope; no or non-GUID client; negative skew → error; upper-case GUIDs and a trailing slash normalised; New fetches nothing; the defaults (10 s client, wall clock) | any one of `New`'s refusals deleted |
| `TestOnlyRSASigningKeysOfAtLeast2048BitsAreUsed` *(added by the build)* | 1024-bit, `use: enc`, `kty: EC`, `alg: PS256` keys → `ErrSignature`; `use` absent and `alg: RS256` accepted; a set of only unusable keys → `ErrKeySetUnavailable` | the key filter stated above, deleted |

#### Steps

- [ ] **Step 1:** Re-read Hub contract §6.3. If it changed after this part was written, the
  contract's words win for everything above the "Phase 8's additions" line, and the change is
  recorded in the commit.
- [ ] **Step 2:** `testkey_test.go`, then the tests; they fail to compile.
- [ ] **Step 3:** `entra.go`, `jwks.go`, `verify.go`, `middleware.go`; all pass under `-race`.
- [ ] **Step 4:** `git diff control-plane/go.mod` — empty. Commit.

#### Acceptance

```bash
go test ./control-plane/internal/entra/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 27 (was 25; see "As built")
go test ./sandboxd/... ./control-plane/... -race -count=1                                 # ok
git diff --stat control-plane/go.mod control-plane/go.sum                                 # empty
```

#### What this task does not claim

- That KPMG's tenant issues tokens in exactly this shape (v2, `azp`, `scp`); the Hub's H1 has one
  real-tenant acceptance test for that (Hub contract `:520-523`).
- Group-overage handling: nothing here reads `groups` (ADR-H1 §6.6 rejects membership from group
  claims).

#### As built (2026-09-25)

Step 1 held: Hub contract §6.3 at `feat/hub` `6704930`, `:417-494`, is the interface above,
unchanged. Departures, each declared in the task report:

1. **27 tests, not 25.** The two rows marked *added by the build*: `New`'s refusals other than the
   insecure authority, and the JWKS key filter, had no test in the table, so a mutant deleting any
   of them survived.
2. **The middleware follows the Hub contract outside §6.3**: §4.1 (`insufficient_scope` for a
   missing scope, still 401) and §4.2 (`reason: missing` for no token; `tenantId` on the 403). The
   Hub must answer the same, and these are its words. One exception: with no Bearer credential
   the challenge is a bare `Bearer` (RFC 6750 §3.1), where §4.1 (`:223`) says every 401 carries
   `error="invalid_token"`; the Hub must call `Middleware` or match that. The Middleware and
   Ready docs in the block above are the source's, byte for byte (fix round 1; they had been
   the same rules in different words).
3. **Rules the part left open, taken on the safe side:**
   - `http` authority: a loopback **IP literal** only; `localhost` is refused (the key set is the
     trust anchor, and a name is one resolver away from elsewhere). The authority is
     `scheme://host` only.
   - Config GUIDs are trimmed and lower-cased. A token's `tid` must equal an enrolled one exactly,
     and `oid` must be a lower-case GUID: Entra writes both lower-case, and a case-folded `oid`
     would split one user into two `Key()`s.
   - Shape (`ErrMalformed`): a token over 32 KiB, a byte outside unpadded base64url, a `crit`
     header, a header or payload that is not a JSON object, a claim of the wrong JSON type (an
     array `aud` included), `exp`/`nbf` outside 0…9999-12-31. At the time step, a missing `exp` or
     `nbf` is `ErrMalformed`.
   - A JWKS key with `alg` present and not `RS256`, padded `n`/`e`, or an exponent over four bytes
     (which `big.Int.Int64` would truncate into a valid-looking one) is skipped; of two keys under
     one `kid`, the first wins. A set with no usable key, or a body over 1 MiB, is
     `ErrKeySetUnavailable`, so `Ready` never counts a tenant fetched without a usable key.
     `Ready` means "fetched once" (ADR-H1 §5.7, `67609ab :528-530`): a set past 24 h whose
     refresh fails still counts as fetched, although `Validate` refuses it. Whether `/readyz`
     should go unready then (an Entra outage would take every broker pod out of rotation) is
     task 17's readiness policy. *[Reworded in fix round 1: it said "never says ready with
     nothing to verify with", which that state contradicts.]*
   - *Fix round 1.* The key set comes only from the authority: `New` sets a no-redirect policy
     on a **copy** of the client, the default one or the caller's (the caller's is not changed),
     so a 3xx is a non-200 answer, `ErrKeySetUnavailable`. A JWKS key is also skipped when its
     exponent is under 3, even or over 2^31-1, or its modulus is even: Go 1.23's
     `VerifyPKCS1v15` checks no exponent (with `e = 1` a "signature" is its own encoding), so
     without this rule what is trusted would depend on the toolchain.
   - A set older than 24 h is not used when its refresh fails (`ErrKeySetUnavailable`). A refetch an
     unknown `kid` caused that fails is `ErrKeySetUnavailable`, and still spends the 5-min budget.
     On a cold cache the fetch that fills it is the one lookup: no immediate second fetch.
   - The fetch runs detached from the caller's context, capped at 10 s by the validator itself
     (whatever `HTTPClient` is configured), so a caller that gives up does not fail the others
     waiting on it, and a server that never answers cannot hold a fetch open.
   - Two `Authorization` headers are `ErrMalformed`; an error that is none of the eleven is 500.
