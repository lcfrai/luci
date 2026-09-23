### Task 11: `devissuer`, part 1 — the package, the fictional tenant, discovery, JWKS, and token minting

**Lane B. Depends on: 10 (its tests validate minted tokens with `entra`). About 2.5 hours. Contract task H1.**

A KPMG app registration is an IT request with its own lead time (ADR-H1 §5.2). `devissuer` is a
loopback-only OIDC issuer that serves the same things the control plane and the Hub use from Entra
— discovery, `/authorize`, `/token`, JWKS, and Entra-v2-shaped tokens — so every line of sign-in and
validation code runs unchanged against it. Correction **A5** makes it a **package**
(`internal/devissuer`) with a thin `main`, so tests in `entraauth` (task 30) and the Hub's matrix
test (task 16) can run the real issuer in-process instead of a hand-written fake.

This part: configuration, discovery, the key set, and minting. Task 12: `/authorize` and `/token`.

**Files:**
- `control-plane/internal/devissuer/{config.go,keys.go,tokens.go,discovery.go,issuer.go}` + tests (new)
- `control-plane/cmd/devissuer/main.go` + `main_test.go` (new)
- `control-plane/cmd/devissuer/dev-tenant.json` (new; fictional, no secret)

#### Contract — complete

`control-plane/cmd/devissuer/dev-tenant.json`, committed: fictional people, fictional GUIDs, no
secret of any kind. Every id in it is quoted by `devstack.sh env` (task 09) and by tests.

```json
{
  "tenants": [
    { "id": "11111111-1111-4111-8111-111111111111", "name": "Example Dev Tenant", "enrolled": true,
      "users": [
        { "oid": "aaaaaaaa-0000-4000-8000-000000000001", "name": "Ada Example",   "username": "ada@example.test",   "email": "ada@example.test",   "roles": ["Hub.OrgAdmin"] },
        { "oid": "aaaaaaaa-0000-4000-8000-000000000002", "name": "Grace Example", "username": "grace@example.test", "email": "grace@example.test", "roles": [] },
        { "oid": "aaaaaaaa-0000-4000-8000-000000000003", "name": "Linus Example", "username": "linus@example.test", "email": "linus@example.test", "roles": [] },
        { "oid": "aaaaaaaa-0000-4000-8000-000000000004", "name": "Out Sider",     "username": "out@example.test",   "email": "out@example.test",   "roles": [] }
      ] },
    { "id": "44444444-4444-4444-8444-444444444444", "name": "Second Dev Tenant", "enrolled": true,
      "users": [
        { "oid": "bbbbbbbb-0000-4000-8000-000000000001", "name": "Xavier Other", "username": "xavier@other.test", "email": "xavier@other.test", "roles": [] }
      ] },
    { "id": "66666666-6666-4666-8666-666666666666", "name": "Unenrolled Dev Tenant", "enrolled": false,
      "users": [
        { "oid": "cccccccc-0000-4000-8000-000000000001", "name": "Nobody Enrolled", "username": "nobody@nowhere.test", "email": "nobody@nowhere.test", "roles": [] }
      ] }
  ],
  "clients": [ { "clientId": "22222222-2222-4222-8222-222222222222", "name": "PanaceaApp Desktop (dev)" } ],
  "apis": [
    { "clientId": "33333333-3333-4333-8333-333333333333", "name": "PanaceaApp Hub API (dev)",        "scopes": ["Hub.Access"] },
    { "clientId": "55555555-5555-4555-8555-555555555555", "name": "PanaceaApp Sandbox Broker (dev)", "scopes": ["Sandboxes.Access"] }
  ]
}
```

Three tenants make every column of the authorisation matrix reachable locally: the first is the
org (`V`, `M`, `A`, `O`, `N`); the second is `X`, another **enrolled** org; the third is enrolled
nowhere, for `tenant_not_enrolled`. Fields beyond contract §6.4's schema — tenant `name` and
`enrolled`, client and API `name` — are additive.

```go
// control-plane/internal/devissuer/config.go
package devissuer

type Config struct {
	Tenants []Tenant `json:"tenants"`
	Clients []Client `json:"clients"`
	APIs    []API    `json:"apis"`
}

type Tenant struct {
	ID       string `json:"id"`       // a GUID
	Name     string `json:"name"`
	Enrolled bool   `json:"enrolled"` // listed in PANACEA_APP_HUB_TENANTS by Env
	Users    []User `json:"users"`
}

type User struct {
	OID      string   `json:"oid"` // a GUID, unique across the file
	Name     string   `json:"name"`
	Username string   `json:"username"` // preferred_username
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
}

type Client struct {
	ClientID string `json:"clientId"`
	Name     string `json:"name"`
}

type API struct {
	ClientID string   `json:"clientId"`
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
}

// LoadConfig reads the file with unknown fields refused (so no one adds a
// "secret" field and has it silently carried), and validates: every id a
// GUID; oids unique; ≥ 1 tenant, client and API; exactly one API exposes
// "Hub.Access"; every API has ≥ 1 scope.
func LoadConfig(path string) (Config, error)
```

```go
// control-plane/internal/devissuer/issuer.go
package devissuer

type Options struct {
	Config Config
	// BaseURL is how every party reaches the issuer: "http://127.0.0.1:8740".
	// A tenant's issuer is BaseURL + "/" + tid + "/v2.0" — the entra
	// validator's rule with another host (contract §6.4).
	BaseURL    string
	AccessTTL  time.Duration // default 60 m (PANACEA_APP_DEVISSUER_ACCESS_TTL)
	RefreshTTL time.Duration // default 24 h
	Auto       bool          // PANACEA_APP_DEVISSUER_AUTO (task 12)
	Now        func() time.Time
	Key        *rsa.PrivateKey // nil: a fresh RSA-2048 key at New
}

type Issuer struct{ /* unexported: options, kid, codes, refresh tokens, mutex */ }

func New(o Options) (*Issuer, error)

// Handler serves the paths of contract §6.4. This task: discovery and keys;
// task 12: authorize and token. Anything else is 404.
func (i *Issuer) Handler() http.Handler

// MintAccessToken signs an Entra-v2-shaped access token without a browser —
// for tests (task 16's matrix, task 34's conformance) and nothing else. It
// refuses a tid, oid, client or API not in the configuration.
func (i *Issuer) MintAccessToken(tid, oid, clientID, apiClientID string, scopes []string) (string, error)

// Env is the variables devstack.sh prints (task 09), one "NAME=value" per line,
// in this order: PANACEA_APP_HUB_TENANTS (every enrolled tenant, "<id>=<name>",
// comma-separated), PANACEA_APP_HUB_AUDIENCE, PANACEA_APP_HUB_ALLOWED_CLIENTS,
// PANACEA_APP_HUB_URL=http://127.0.0.1:8750, PANACEA_APP_ENTRA_AUTHORITY (the
// FIRST tenant), PANACEA_APP_ENTRA_CLIENT_ID, PANACEA_APP_HUB_SCOPE.
func (i *Issuer) Env() []string
```

**Discovery** — `GET /<tid>/v2.0/.well-known/openid-configuration`, for any configured tenant
(else 404):

```json
{ "issuer": "<base>/<tid>/v2.0",
  "authorization_endpoint": "<base>/<tid>/oauth2/v2.0/authorize",
  "token_endpoint": "<base>/<tid>/oauth2/v2.0/token",
  "jwks_uri": "<base>/<tid>/discovery/v2.0/keys",
  "response_types_supported": ["code"],
  "response_modes_supported": ["query"],
  "subject_types_supported": ["pairwise"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid", "profile", "email", "offline_access"],
  "token_endpoint_auth_methods_supported": ["none"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "code_challenge_methods_supported": ["S256"] }
```

**Keys** — `GET /<tid>/discovery/v2.0/keys`: `{"keys":[{"kty":"RSA","use":"sig","alg":"RS256",
"kid":"<16 hex>","n":"<base64url modulus>","e":"AQAB"}]}`; `kid` is the first 16 hex characters
of SHA-256 over the modulus's big-endian bytes (contract §6.4).

**Tokens** — JWS compact, header `{"alg":"RS256","typ":"JWT","kid":"<kid>"}`. Claims, all as Entra
v2 names them:

| Claim | Access token | ID token |
|---|---|---|
| `ver` | `"2.0"` | `"2.0"` |
| `iss` | `<base>/<tid>/v2.0` | same |
| `aud` | the **API's** client id | the **desktop client's** id |
| `azp` | the client id | — |
| `scp` | the granted scopes, space-separated, bare names (`Hub.Access`) | — |
| `tid`, `oid`, `name`, `preferred_username`, `email` | the user's | the user's |
| `roles` | the user's, **only when the API is the Hub API** and the list is non-empty | — |
| `sub` | pairwise: base64url(SHA-256(oid + "\|" + aud)), first 43 characters | same rule, with the client id |
| `nonce` | — | the authorize request's |
| `iat`, `nbf` | now | now |
| `exp` | now + `AccessTTL` | now + `AccessTTL` |
| `uti` | 22 random base64url characters | same |

**`cmd/devissuer`**: `devissuer` (serve) and `devissuer env`. Variables, read in one block:
`PANACEA_APP_DEVISSUER_ADDR` (default `127.0.0.1:8740`; a host that is not loopback is refused
at start), `PANACEA_APP_DEVISSUER_CONFIG` (default `control-plane/cmd/devissuer/dev-tenant.json`,
resolved against the working directory), `PANACEA_APP_DEVISSUER_ACCESS_TTL` (default `60m`,
minimum `1m`), `PANACEA_APP_DEVISSUER_AUTO` (default `false`). `BaseURL` is `http://` + `ADDR`.

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheCommittedTenantLoads` | `LoadConfig("../../cmd/devissuer/dev-tenant.json")` succeeds; three tenants, one unenrolled; the Hub API exposes `Hub.Access` | a committed config that the stack cannot start with |
| `TestLoadConfigRefusals` | a non-GUID id, a duplicate oid, an API without scopes, two APIs with `Hub.Access`, an unknown field `"secret"` → an error each | a secret smuggled into a committed file |
| `TestDiscoveryDocument` | the JSON above exactly, for tenant 1; an unknown tenant → 404 | an issuer string that differs from the validator's rule |
| `TestKeysAndKid` | one key; `kid` = the modulus rule; `n` decodes to the key's modulus; `e` = `AQAB` | a kid the validator's refetch logic never matches |
| `TestAMintedAccessTokenValidates` | `entra.New(Config{AuthorityHost: base, TenantIDs: [t1], Audience: hubAPI, RequiredScope: "Hub.Access", AllowedClients: [desktop], AllowInsecureAuthority: true})` accepts a minted token; `Claims` carries tid, oid, name, username, email, `roles` for Ada | an issuer the Hub cannot validate (R9) |
| `TestRolesOnlyForTheHubAPI` | Ada's token for the broker API has no `roles` | the org-admin role leaking to another resource |
| `TestSubIsPairwise` | one oid, two APIs → two different `sub`s; `oid` equal | code that keys on `sub` passing locally and failing on Entra |
| `TestMintRefusesUnknownParties` | an unknown tid, oid, client or API → error | a test minting a principal that does not exist |
| `TestLoopbackOnlyBind` | `ADDR=0.0.0.0:8740` and `ADDR=192.168.1.5:8740` → start refused; `127.0.0.1:0` and `[::1]:0` → start | a token mint reachable from the LAN |
| `TestEnvLines` | the seven lines, in order, with tenant 3 absent from `PANACEA_APP_HUB_TENANTS` | devstack printing the wrong authority or enrolling the unenrolled tenant |

#### Steps

- [ ] **Step 1:** Re-read contract §6.4 and the entra check table (§6.3).
- [ ] **Step 2:** `dev-tenant.json`; tests; they fail.
- [ ] **Step 3:** `config.go`, `keys.go`, `tokens.go`, `discovery.go`, `issuer.go`, `main.go`.
- [ ] **Step 4:** `go run ./control-plane/cmd/devissuer env` prints the seven lines; commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/devissuer/ ./control-plane/cmd/devissuer/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 10
go run ./control-plane/cmd/devissuer env | head -1
# PANACEA_APP_HUB_TENANTS=11111111-1111-4111-8111-111111111111=Example Dev Tenant,44444444-4444-4444-8444-444444444444=Second Dev Tenant
git grep -n -i "secret\|password" -- control-plane/cmd/devissuer/dev-tenant.json                                         # no output
```

#### What this task does not claim

- That Entra's real tokens carry exactly these claims with `profile` and `email` scopes (ADR-H1
  §11); task 43's real-tenant checklist compares one real token's claim names against this table.
- Anything interactive: task 12.
