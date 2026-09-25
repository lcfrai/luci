### Task 10: `internal/oidcauth` — the one validator, **env task 17 step 0's**, taken unchanged (OD-30)

**Lane B. Depends on: env task 17 step 0 (through the orchestrator's merge) and task 11. About 1 hour.
Contract task H1 (HA-1).** Ruling: main `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`
§5 (`:222-302`); it supersedes this part's earlier "write phase 8 task 16 here if it has not landed".

The Hub and the broker accept and reject **the same tokens** (HA-1): one package, in the one module.
Phase 8 task 16 BUILT it as `control-plane/internal/entra` on `feat/env` (`669204b` .. `1772f25`), with
Entra-only rules that refuse every Keycloak token. **Env task 17's step 0** renames it by `git mv` to
`control-plane/internal/oidcauth` and adds the `keycloak` claims profile, committed before any broker code.
**This task writes no validator** and copies nothing: it takes that commit unchanged through the
orchestrator's merge. Task 13 waits for this task and inherits the wait.

**Files:** `control-plane/internal/devissuer/oidcauth_test.go` (new, package `devissuer_test`). Nothing
under `control-plane/internal/oidcauth/` is written by this lane.

#### Contract — the exported API the Hub relies on (ruling §5, complete)

```go
package oidcauth

type Profile string

const (
	ProfileKeycloak Profile = "keycloak"
	ProfileEntra    Profile = "entra"
)

type Config struct {
	Profile        Profile  // required; New refuses "". Callers default it from PANACEA_APP_OIDC_PROFILE.
	Issuer         string   // keycloak: exact iss; JWKS at Issuer + "/protocol/openid-connect/certs".
	AuthorityHost  string   // entra only, unchanged.
	TenantIDs      []string // both: enrolled tids, GUIDs. keycloak: checked AFTER the signature.
	Audience       string   // keycloak: a client-id string, aud must CONTAIN it. entra: GUID, unchanged.
	RequiredScope  string   // keycloak: in `scope`. entra: in `scp`, unchanged.
	AllowedClients []string // person clients (azp). keycloak: strings. entra: GUIDs, unchanged.
	ServiceClients []string // keycloak only: azp values accepted with NO oid and NO tid (ruling §6).
	ClockSkew              time.Duration // defaults to 120 s
	AllowInsecureAuthority bool          // also permits a loopback http Issuer
	HTTPClient             *http.Client  // default: 10 s timeout
	Now                    func() time.Time
}

// Claims is what a validated token says. It holds no token text.
type Claims struct {
	TenantID, ObjectID, ClientID       string // tid, oid, azp
	Name, PreferredUsername, Email     string
	Scopes, Roles                      []string // keycloak: `scope` split on spaces; flat `roles`
	ExpiresAt                          time.Time
	Issuer, Subject                    string // iss; sub (logged, never a key)
	Service                            bool   // an azp in ServiceClients, no oid, no tid
}

func (c Claims) Key() string // person: "tid/oid" (unchanged). service: "client/" + ClientID.

var ErrType = errors.New("oidcauth: typ is not Bearer") // keycloak profile only
// Unchanged from task 16, prefix renamed: ErrMalformed, ErrSignature, ErrExpired, ErrNotYetValid,
// ErrIssuer, ErrAudience, ErrScope, ErrClient, ErrVersion (never under keycloak), ErrTenant,
// ErrKeySetUnavailable; New, JWKSValidator, Validator, Ready, Middleware, ClaimsFrom, CodeUnauthenticated.
```

**`keycloak` order** (ruling §5 `:266-271`): size and alg → signature against the issuer's JWKS → exp/nbf →
`iss` equals `Issuer` → `typ == "Bearer"` → `aud` contains `Audience` → `azp` in `AllowedClients` or
`ServiceClients` → `RequiredScope` in `scope` → person: `oid` and `tid` GUIDs, `tid` in `TenantIDs`;
service: both absent, `Service = true`. Hub mapping: `ErrType` → `ReasonMalformed` (hub `hubapi/codes.go:57-67`).

#### Tests to write first

`TestADevissuerKeycloakTokenValidates` (in `oidcauth_test.go`): start `devissuer` in process (task 11,
default profile), mint a person token for audience scope `panacea-app-hub`, and validate it with
`oidcauth.New(Config{Profile: ProfileKeycloak, Issuer: <devissuer>/realms/panacea-app-dev, Audience:
"panacea-app-hub", RequiredScope: "panacea-app-hub", AllowedClients: ["panacea-app-desktop"], TenantIDs:
[dev tid], AllowInsecureAuthority: true})`. Asserts `Key() == "<tid>/<oid>"` of the dev user and
`Service == false`. **Kills** a `devissuer` whose token the Hub can never validate locally (a wrong `iss`
path, a string-only `aud`, `scp` in place of `scope`, or no `typ`). A second row mints the Fleet
client-credentials token and asserts `Key() == "client/panacea-app-fleet"` when Fleet is in `ServiceClients`.

#### Steps

- [ ] **Step 1: Preflight.** `ls control-plane/internal/oidcauth/` on this lane's base. If absent, stop and
  report "task 10 waits for env 17 step 0"; never write the package here.
- [ ] **Step 2:** run its suite unchanged; record "task 10: env 17 step 0 at `<sha>`" in the plan's notes.
- [ ] **Step 3:** write the test above; it passes against task 11's `devissuer`. Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/oidcauth/ -race -count=1 2>&1 | tail -1         # ok
go test ./control-plane/internal/devissuer/ -run TestADevissuerKeycloakTokenValidates -race -count=1 -v \
  2>&1 | grep -c "^--- PASS"                                                     # 1
git diff --stat <env 17 step 0 sha> -- control-plane/internal/oidcauth           # empty
git diff --stat control-plane/go.mod control-plane/go.sum                        # empty
```

#### What this task does not claim

- That Panacea's live realm issues exactly this shape; ruling §10's check and task 43 settle it (U-2..U-4).
- Group-overage handling: the desktop does not request `groups` (ruling §4; ADR-H1 §6.6).
- Ownership: the package stays env's. A change is made in phase 8's plan; the Hub adapts in
  `internal/hub/server/auth.go` alone.
