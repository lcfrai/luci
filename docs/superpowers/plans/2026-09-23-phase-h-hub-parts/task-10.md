### Task 10: `internal/entra` — the one Entra validator, **phase 8 task 16's**, written here only if it has not landed

**Lane B. Depends on: nothing. About 3 hours, or 0 if phase 8 task 16 has landed. Contract task H1 (HA-1).**

The Hub and phase 8's broker must accept and reject **the same tokens** (HA-1). ADR-H1 §4.1 makes
that true by construction: one package, `control-plane/internal/entra`, in the one module. Phase 8
owns it — its task 16 part (sibling `feat/env`,
`docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-16.md`, read 2026-09-23)
adopts the Hub contract's §6.3 interface **exactly** and adds five things (`JWKSValidator`,
`ErrKeySetUnavailable`, `Claims.Key`, `Ready`, `Middleware`/`ClaimsFrom`/`CodeUnauthenticated`).

**This task exists so the Hub is not blocked on phase 8's schedule.** It is one of two outcomes:

- **Phase 8 task 16 has landed** on `feat/hub`'s base (or in a branch the orchestrator merges
  first): do nothing but Step 1, and record "task 10: satisfied by phase 8 task 16 at `<sha>`" in
  the plan's progress notes.
- **It has not:** implement phase 8 task 16's part **exactly** — its files, its interface, its
  check order, its 25 tests — in this lane. The part gives signatures, not bodies (`feat/env` at
  `aa12b01`), so when phase 8's branch later carries the same files, the two sets of bodies meet
  at one path: an add/add conflict, not a no-op (review M5). **Phase 8 wins it:** the orchestrator
  keeps phase 8's files whole and re-runs this plan's tests against them (plan, "Files that belong
  to a sibling phase").

**Files** (phase 8 task 16's list, unchanged):
- `control-plane/internal/entra/{entra.go,jwks.go,verify.go,middleware.go}` (new)
- `control-plane/internal/entra/{entra_test.go,jwks_test.go,middleware_test.go,testkey_test.go}` (new)

#### Contract — complete (phase 8 task 16's, which is contract §6.3 plus its additions)

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

// ── Phase 8's additions. All additive; the Hub uses JWKSValidator, ErrKeySetUnavailable and Ready. ──

// JWKSValidator implements Validator. Safe for concurrent use.
type JWKSValidator struct{ /* unexported: config, per-tenant key cache, single-flight fetch */ }

var _ Validator = (*JWKSValidator)(nil)

// ErrKeySetUnavailable: the tenant's JWKS could not be fetched. NOT a rejection
// of the token. The Hub answers it 503 unavailable (plan correction A7).
var ErrKeySetUnavailable = errors.New("entra: key set unavailable")

// Key is "tid/oid", the stable identity. Never sub, never preferred_username.
func (c Claims) Key() string { return c.TenantID + "/" + c.ObjectID }

// Ready fetches the key set of every enrolled tenant not yet fetched and returns
// the first failure (wrapping ErrKeySetUnavailable). The Hub's /readyz "jwks" check calls it.
func (v *JWKSValidator) Ready(ctx context.Context) error

// Middleware is the broker's; the Hub does NOT use it (plan correction A7): it
// answers a key-set outage with a control-plane code and runs before the Hub's
// Origin and rate-limit steps can.
func Middleware(v Validator, next http.Handler) http.Handler

func ClaimsFrom(ctx context.Context) (Claims, bool)

const CodeUnauthenticated = "unauthenticated"
```

**Check order, unchanged** (contract §6.3 `:483-494`): shape (three base64url segments; `alg ==
"RS256"` and a `kid`; `none` and every `HS*` are `ErrMalformed` before claims are read) → tenant
(`tid` present and enrolled, else `ErrTenant`, **with no fetch**) → signature (the JWKS key whose
`kid` matches, from `<AuthorityHost>/<tid>/discovery/v2.0/keys`, cached 24 h per tenant; an unknown
`kid` refetches at most once per 5 min per tenant) → `ver == "2.0"` → `iss == <AuthorityHost>/<tid>/v2.0`
→ `aud == Audience` → `exp`/`nbf` with skew → `azp ∈ AllowedClients` → `scp` contains
`RequiredScope` (app-only tokens fail here) → `oid` present and a GUID. JWKS keys are used only if
`kty == "RSA"`, `use` absent or `"sig"`, modulus ≥ 2048 bits. Errors never contain the token.

#### Tests to write first — phase 8 task 16's 25, by name

`testkey_test.go` generates one RSA-2048 key per test binary and serves
`/{tid}/discovery/v2.0/keys` from an `httptest.Server` that counts requests, on loopback with
`AllowInsecureAuthority: true`. The tests, each with the assertion and mutant in phase 8's part:
`TestAValidTokenYieldsItsClaims`, `TestNoneIsMalformed`, `TestHS256SignedWithThePublicKeyIsMalformed`,
`TestAMissingKidIsMalformed`, `TestATamperedPayloadFailsTheSignature`,
`TestAnUnknownKidRefetchesOnceThenFails`, `TestRotationIsPickedUpOnAnUnknownKid`,
`TestTheKeySetIsCachedFor24Hours`, `TestConcurrentMissesFetchOnce`,
`TestTheTenantIsCheckedBeforeAnyFetch`, `TestExpiryAndNotBeforeHonourTheSkew`,
`TestAVersion1TokenIsRejected`, `TestTheIssuerMustBeTheTokensOwnTenant`,
`TestTheAudienceIsTheClientIDOnly`, `TestAnUnlistedClientIsRejected`, `TestScopeMatchIsExact`,
`TestAppOnlyTokensFailTheScopeCheck`, `TestAMissingOrNonGUIDOidIsMalformed`,
`TestSubIsNeverTheIdentity`, `TestAKeySetOutageIsNotARejection`, `TestErrorsNeverContainTheToken`,
`TestMiddlewareStatuses`, `TestMiddlewareStripsTheAuthorizationHeader`,
`TestAnInsecureAuthorityIsRefused`, `TestReadyFetchesEveryTenant`.

**One Hub-side test is added here**, because the Hub depends on it and phase 8's list does not
name it: `TestADevissuerShapedTokenValidates` — a token with `iss =
http://127.0.0.1:<port>/<tid>/v2.0` and the claims `devissuer` mints (task 11) validates with
`AllowInsecureAuthority: true`. It kills a validator the Hub can never use locally.

#### Steps

- [ ] **Step 1: Preflight.** `ls control-plane/internal/entra/` on this lane's base, then
  `git -C /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/env log --oneline -- control-plane/internal/entra`
  (read only). If the package exists with phase 8's interface, stop here: run its tests, add
  `TestADevissuerShapedTokenValidates` only if absent, commit that test alone.
- [ ] **Step 2:** Otherwise re-read phase 8's task 16 part in full; it wins over this summary for
  everything above the "Phase 8's additions" line only where the Hub contract §6.3 agrees with it.
- [ ] **Step 3:** `testkey_test.go`, then the 26 tests; they fail to compile.
- [ ] **Step 4:** `entra.go`, `jwks.go`, `verify.go`, `middleware.go`. All pass under `-race`.
- [ ] **Step 5:** `git diff control-plane/go.mod` is empty. Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/entra/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 26
go list -deps ./control-plane/internal/entra | grep '\.' | grep -v '^vendor/' | grep -v 'control-plane/internal/entra$'
# no output: standard library only (a third-party path has a dot in its first element;
# the standard library's own vendored x/ packages start "vendor/")
git diff --stat control-plane/go.mod control-plane/go.sum                             # empty
```

#### What this task does not claim

- That KPMG's tenant issues tokens in exactly this shape (v2, `azp`, `scp`); task 43's real-tenant
  checklist checks that once the registrations exist.
- Group-overage handling: nothing reads `groups` (ADR-H1 §6.6).
- Ownership: the package stays phase 8's. A later change to it is made in phase 8's plan and the
  Hub adapts in `internal/hub/server/auth.go` alone (contract §6.3).
