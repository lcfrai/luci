### Task 13: the Hub's auth pipeline — rate limits, token validation, org enrolment, principal resolution (correction A7)

**Lane A. Depends on: 07, 08, 10. About 3 hours. Contract tasks H1/H2.**

Contract §5.1 is the order every `/v1` request passes through. Steps 1–4 need no database and live
in one middleware here; steps 5–6 (`resolve_principal`, then `set_config('hub.org_id', …)`) are
the first two statements of every request's transaction, which task 14 builds — so a write's
principal lookup, its audit lock and its change commit **in one transaction**, as §10.6 draws it.
This task also enrols the configured tenants at start (§5.1, *"org enrolment is configuration in
v1"*) and adds the `jwks` readiness check.

**Why the per-user and per-org limits key on `tid/oid` and `tid`:** §5.1 puts the limits (step 4)
before the transaction (step 5), when the Hub's own `u-…` and `org-…` ids are not yet known. The
token's `(tid, oid)` is the same identity one-to-one (ADR-H1 §5.2's principal rule), so the bucket
is the same bucket.

**Files:**
- `control-plane/internal/hub/server/auth.go` + `auth_test.go` (new)
- `control-plane/internal/hub/server/ratelimit.go` + `ratelimit_test.go` (new)
- `control-plane/internal/hub/server/export_test.go` — a test-only route behind the pipeline (new)
- `control-plane/internal/hub/store/orgs.go` + `orgs_integration_test.go` (new)
- `control-plane/cmd/hub/serve.go` — build the validator, enrol the orgs, wire the middleware (modify)
- `control-plane/go.mod` — `golang.org/x/time` becomes a direct requirement **at the version the graph already holds** (v0.3.0 today, `control-plane/go.mod:57`), by an import and `go mod tidy`, never a named `go get` (modify ⚑)

#### Contract — complete

```go
// control-plane/internal/hub/server/auth.go
package server

// Claims are the validated token's facts that later steps need. The token text
// is not kept anywhere.
type Claims struct {
	TenantID, ObjectID string
	Name, Email        string // DISPLAY ONLY. Email: the email claim, else preferred_username, lowercased (§5.1)
	OrgAdmin           bool   // roles ∋ AuthConfig.OrgAdminRole, read from THIS token (§5.1)
	// BindEmail is the only address an invite may bind to (review I12, correction
	// A15): the email claim alone, lowercased, and only when THIS token shows a
	// member account (acct is the JSON number 0) or a verified email domain owner
	// (xms_edov is the JSON boolean true). Otherwise "". Never preferred_username,
	// never the users row's stored email. Both claims are optional in Entra, so an
	// absent one, or one of another JSON type, fails closed.
	BindEmail          string
}

// bindingClaims reads acct and xms_edov from the payload segment of the bearer
// that Validator.Validate has JUST accepted — never before Validate, and never a
// token Validate refused. It is the seam for correction A15: entra.Claims (phase
// 8's package, task 10) carries neither claim, and task 10's ownership rule says
// the Hub adapts in auth.go alone. When phase 8 adds them (the sibling ask in the
// plan's A15 row), this function's body becomes a read of entra.Claims and nothing
// else changes. It decodes JSON only: no signature code, and nothing is kept.
func bindingClaims(bearer string) (member, emailVerified bool)

// ClaimsFrom returns what the pipeline stored. Every /v1 handler may assume ok.
func ClaimsFrom(ctx context.Context) (Claims, bool)

type Rate struct {
	PerSecond float64
	Burst     int
}

type Limits struct {
	IP, User, UserWrite, Org Rate
	BlobBytesPerMinute       int64
}

type AuthConfig struct {
	Validator    entra.Validator
	OrgAdminRole string // "Hub.OrgAdmin"
	Limits       Limits
	// TrustedHops is how many proxies in front of the Hub append to
	// X-Forwarded-For (PANACEA_APP_HUB_TRUSTED_HOPS, correction A14). 0: the header
	// is ignored and the peer address is the key. N ≥ 1: every X-Forwarded-For
	// line, joined in order, is split on commas, and the key is the N-th entry
	// from the RIGHT — the address the outermost trusted proxy saw. Never the
	// left-most: a proxy appends, so everything left of its entry is whatever the
	// client wrote (review I7). Fewer than N entries, or an entry that is not an
	// IP address, and the peer address is the key.
	TrustedHops  int
	Log          *slog.Logger
	Now          func() time.Time
}

// Authenticate wraps every /v1 route. In order (contract §5.1 steps 2–4; step 1,
// the Origin refusal, is the chassis's, task 08):
//  2. the per-IP limit → 429 rate_limited, scope "ip";
//  3. "Authorization: Bearer <token>" — absent or not Bearer → 401 unauthenticated,
//     reason "missing"; Validator.Validate — each rejection → 401 with its reason
//     (table below); entra.ErrTenant → 403 tenant_not_enrolled, details.tenantId;
//     entra.ErrKeySetUnavailable → 503 unavailable (correction A7);
//  4. the per-user limit (key tid/oid) → 429 "user"; for a non-GET, the
//     user-write limit → 429 "user_write"; the per-org limit (key tid) → 429 "org".
// Then it stores Claims in the context and calls next. It logs tid and the
// reason on a refusal, never the token (contract §11.5).
func Authenticate(cfg AuthConfig, next http.Handler) http.Handler

// BlobBudget is the per-user byte bucket (contract §11.3, "blob_bytes"). The blob
// route (task 21) reserves Content-Length bytes before reading the body.
type BlobBudget interface {
	Reserve(userKey string, n int64) (wait time.Duration, ok bool)
}
```

| `entra` error | `details.reason` | Status, header |
|---|---|---|
| (no or non-Bearer header) | `missing` | 401, `WWW-Authenticate: Bearer` |
| `ErrMalformed` | `malformed` | 401, `Bearer error="invalid_token"` |
| `ErrSignature` | `signature` | 401, `invalid_token` |
| `ErrExpired` | `expired` | 401, `invalid_token` — the one reason the control plane refreshes on (§4.3) |
| `ErrNotYetValid` | `not_yet_valid` | 401, `invalid_token` |
| `ErrIssuer` | `issuer` | 401, `invalid_token` |
| `ErrAudience` | `audience` | 401, `invalid_token` |
| `ErrScope` | `scope` | 401, `Bearer error="insufficient_scope"` |
| `ErrClient` | `client` | 401, `invalid_token` |
| `ErrVersion` | `version` | 401, `invalid_token` |
| `ErrTenant` | — | 403 `tenant_not_enrolled`, `details.tenantId` |
| `ErrKeySetUnavailable` | — | 503 `unavailable` |
| anything else | — | 500 `internal`, logged |

```go
// control-plane/internal/hub/store/orgs.go
package store

// EnsureOrgs calls hub.ensure_org for each configured tenant (contract §10.4), with a
// fresh ids.Wide("org") that is used only if the tenant is new, and returns
// tenant id → org id. `hub serve` calls it once at start, before listening.
func (s *Store) EnsureOrgs(ctx context.Context, tenants []Tenant) (map[string]string, error)

type Tenant struct{ ID, Name string }

// ResolvedPrincipal is hub.resolve_principal's row.
type ResolvedPrincipal struct {
	OrgID, UserID string
	Created       bool
}

// ErrNotEnrolled: resolve_principal returned no row — the token's tenant has no org.
var ErrNotEnrolled = errors.New("store: tenant not enrolled")

// ResolvePrincipal runs hub.resolve_principal on tx with a fresh ids.Wide("u"),
// retrying ONCE with another id on a primary-key collision (contract §3.1:
// a collision costs a retry, never a wrong row). Task 14's transaction calls it first.
func ResolvePrincipal(ctx context.Context, tx pgx.Tx, tenantID, objectID, name, email string) (ResolvedPrincipal, error)
```

**Readiness.** `serve` adds `Extra["jwks"] = validator.Ready` (phase 8 task 16's `JWKSValidator.Ready`):
`/readyz` is not ready until every enrolled tenant's key set has been fetched once (§12.4).

**The limiter map.** One `*rate.Limiter` per key, created on first use; an entry unused for 10
minutes is dropped by a sweep every minute, so a flood of distinct IPs or users cannot grow the
map without bound. Each limit's rate and burst are the configuration's (`PANACEA_APP_HUB_RATE_*`).
`Retry-After` is the reservation's delay, rounded **up** to a whole second, minimum 1.

#### Tests to write first

Unit tests use a fake `entra.Validator` that returns a scripted `Claims` or error and counts calls.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMissingBearerIs401Missing` | no header, `Basic x`, `Bearer` with nothing → 401 `unauthenticated`, `reason: missing`, `WWW-Authenticate: Bearer`; the validator is never called | a validator fed the empty string |
| `TestEachRejectionHasItsReason` | one row per line of the table above | two rejections collapsed into one, so the control plane refreshes when it should re-sign-in |
| `TestTenantNotEnrolledIs403` | `ErrTenant` → 403 `tenant_not_enrolled`, `details.tenantId` = the token's `tid` | a 401 that sends the user round a sign-in loop |
| `TestAKeySetOutageIs503Unavailable` | `ErrKeySetUnavailable` → 503 `unavailable`; never `backend_unavailable` (A7) | "your token is invalid" during an Entra outage |
| `TestIPLimitRunsBeforeValidation` | with `IP 1:2`, the 3rd request in a burst → 429 `scope: ip`, `Retry-After ≥ 1`; the validator was called twice | a flood of bad tokens that costs a signature check each |
| `TestUserAndWriteLimitsAreSeparate` | with `User 100:100` and `UserWrite 1:1`, two POSTs → the second 429 `user_write`; GETs still pass | reads throttled by writes |
| `TestOrgLimit` | two users of one tenant share the `org` bucket; a user of another tenant does not | a per-user limit mislabelled as an org one |
| `TestForwardedForOnlyWhenTrusted` | with `IP 1:2`: (1) `TrustedHops 1`, `X-Forwarded-For: 198.51.100.7, 203.0.113.9` → the key is `203.0.113.9`; (2) three requests from that ingress hop whose left-most value changes each time (`198.51.100.1, 203.0.113.9`, `198.51.100.2, 203.0.113.9`, …) → the 3rd is 429 `scope: ip`: a client-written value never resets the bucket; (3) the same entries split over two `X-Forwarded-For` lines → the same key; (4) `TrustedHops 2`, `198.51.100.7, 203.0.113.9, 10.0.0.1` → `203.0.113.9`; (5) `TrustedHops 1` with no header, or a right-most entry that is not an IP → the peer address; (6) `TrustedHops 0` → the header is ignored and the peer address is the key | the left-most entry as the key (review I7: a made-up value per request is a fresh bucket, and the pre-validation bound is gone behind phase 8's ingress); only the first header line read; the header honoured with `TrustedHops 0` |
| `TestLimiterEntriesAreEvicted` | 1,000 distinct keys; advance 11 min; sweep; the map is empty | unbounded memory under a spray of IPs |
| `TestClaimsReachTheHandler` | through `export_test.go`'s route: `ClaimsFrom` has tid, oid, name, the email rule (email claim, else `preferred_username`, lowercased) and `OrgAdmin` from `roles` | an email taken from `preferred_username` when `email` exists; an org admin cached across tokens |
| `TestBindEmailNeedsAMemberOrAVerifiedEmail` | the fake validator accepts a hand-built bearer whose payload holds the claims under test; `ClaimsFrom(...).BindEmail` is: `acct: 0` + `email` → the email, lowercased; `acct: 1` without `xms_edov` → `""`; `acct: 1` + `xms_edov: true` → the email; `acct` absent → `""`; `acct: "0"` (a string) → `""`; `xms_edov: "true"` (a string) → `""`; `acct: 0`, no `email`, a `preferred_username` → `""`, while `Email` (display) is still the `preferred_username`; a bearer the validator refuses → 401 and its payload is never decoded (a payload that is not JSON gives 401, not 500) | binding by `preferred_username` (review I12); a guest's self-asserted `email` granting a member's invite; an absent optional claim read as `acct == 0`; `bindingClaims` run before `Validate` |
| `TestNoTokenInAnyLogLine` | every test above run with a captured `slog` handler; no line contains the bearer text or its signature segment | a token in a log (T1) |
| `TestADevissuerTokenPassesTheRealValidator` | `devissuer` in-process (task 11), `entra.New` configured as `serve` configures it with `AllowInsecureAuthority` → a minted token for Grace reaches the handler, with `BindEmail` = `grace@example.test` (devissuer's `acct: 0`, task 11) | a pipeline tested only against the fake; a devissuer whose members can never bind |

Integration (`orgs_integration_test.go`, `//go:build integration`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEnsureOrgsIsStableAcrossRestarts` | two tenants → two orgs; a second call → the same two ids; a renamed tenant → the new display name, the same id | a new org per restart, orphaning every project |
| `TestResolvePrincipalRetriesACollision` | a users row pre-inserted with the id the first attempt will use (the id source injected) → the retry succeeds with the second id | a collision surfacing as a 500 |
| `TestResolvePrincipalRefusesAnUnknownTenant` | → `ErrNotEnrolled` | a user created with a NULL org |

#### Steps

- [ ] **Step 1:** Re-read contract §5.1, §6.3, §11.3, §11.5; phase 8 task 16's `Ready` and
  `ErrKeySetUnavailable`.
- [ ] **Step 2:** Record the version the graph already holds: `cd control-plane && go list -m
  golang.org/x/time > $TEMP/xtime.txt` (v0.3.0 today, `control-plane/go.mod:57`). **Never `go get`
  x/time at a named version**: a lane or sibling merged before this task may have raised it, and a
  named pin would lower it and whatever requires it (plan re-review b03 m2; plan.md's "The `go`
  directive"). `go get` also does not make a module direct; importing it does.
- [ ] **Step 3:** Tests; they fail. Then `ratelimit.go` (which imports `golang.org/x/time/rate`),
  `auth.go`, `orgs.go`; then `go mod tidy`. `git diff control-plane/go.mod` shows only the x/time line
  moving from the indirect block to the direct block, **at the version Step 2 recorded**; any other
  line moving is a defect to undo, not to commit. Wire `serve.go`:
  build `entra.New` from `Config` (`AllowInsecureAuthority` = `DevIssuer`), `EnsureOrgs`, wrap `/v1`
  in `Authenticate` (`AuthConfig.TrustedHops` = `Config.TrustedHops`), add the readiness check.
- [ ] **Step 4:** By hand: `devstack.sh up` (task 09 — or its manual equivalent if task 09 has not
  landed), then `curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8750/v1/me` → `401`
  (no `/v1/me` handler exists yet, and the pipeline answers first).
- [ ] **Step 5:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hub/server/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"                # 24 (11 from task 08 + 13)
go test -tags integration ./control-plane/internal/hub/store/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # ≥ 4 (task 08's dbauth test + these 3)
git diff control-plane/go.mod | grep "^[-+]" | grep -v "^[-+][-+]"                                         # only golang.org/x/time moving to the direct block, at $TEMP/xtime.txt's version
```

#### What this task does not claim

- Any authorisation by role or membership: tasks 15 and 16.
- That the limits are right for production traffic; they stop runaway loops (§11.3).
- That per-replica limits add up: with two replicas the effective limit is up to double
  (contract §11.3, accepted).
