### Task 11: `devissuer`, part 1 — the package, the fictional tenant, discovery, JWKS, and token minting

**Lane B. Depends on: 10 (its tests validate minted tokens with `oidcauth`). About 3 hours. Contract task H1.**

**Amended by OD-30** (main `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` §2 `:101-106`,
§4, §5 `:296-302`, §6): users sign in through a Keycloak realm that brokers Entra. `devissuer` is a
loopback-only OIDC issuer that serves what the control plane and the Hub use from **that realm** —
Keycloak's path layout, discovery, `/auth`, `/token`, `/certs`, and **`keycloak`-profile** tokens — so
every line of sign-in and validation code runs byte for byte as it will against Panacea's dev realm.
`-profile entra` keeps this part's earlier Entra-v2 shape for the entra-profile end-to-end row only.
Correction **A5** makes it a **package**
(`internal/devissuer`) with a thin `main`, so tests in `signin` (task 30) and the Hub's matrix
test (task 16b) can run the real issuer in-process instead of a hand-written fake.

This part: configuration, discovery, the key set, and minting. Task 12: `/authorize` and `/token`.

**Files:**
- `control-plane/internal/devissuer/{config.go,keys.go,tokens.go,discovery.go,issuer.go}` + tests (new)
- `control-plane/cmd/devissuer/main.go` + `main_test.go` (new)
- `control-plane/cmd/devissuer/dev-tenant.json` (new; fictional; its only credential values are the
  two `dev-` service-client values below, usable on loopback alone)

#### Contract — complete

`control-plane/cmd/devissuer/dev-tenant.json`, committed: fictional people, fictional GUIDs, one realm.
No real credential of any kind: ruling §6 `:320-322` has the client-credentials grant accept "the dev
secret from `dev-tenant.json`", so each service client carries a `devSecret` that `LoadConfig` requires to
start `dev-` and to be at most 32 characters (a pasted real Keycloak secret fails). Every id in it is quoted
by `devstack.sh env` (task 09) and by tests.

```json
{
  "realm": "panacea-app-dev",
  "tenants": [
    { "id": "11111111-1111-4111-8111-111111111111", "name": "Example Dev Tenant", "enrolled": true,
      "users": [
        { "oid": "aaaaaaaa-0000-4000-8000-000000000001", "name": "Ada Example",   "username": "ada@example.test",   "email": "ada@example.test",   "roles": ["hub.org_admin"] },
        { "oid": "aaaaaaaa-0000-4000-8000-000000000002", "name": "Grace Example", "username": "grace@example.test", "email": "grace@example.test", "roles": [] },
        { "oid": "aaaaaaaa-0000-4000-8000-000000000003", "name": "Linus Example", "username": "linus@example.test", "email": "linus@example.test", "roles": [] },
        { "oid": "aaaaaaaa-0000-4000-8000-000000000004", "name": "Out Sider",     "username": "out@example.test",   "email": "out@example.test",   "roles": [] },
        { "oid": "aaaaaaaa-0000-4000-8000-000000000005", "name": "Vera Example",  "username": "vera@example.test",  "email": "vera@example.test",  "roles": [] }
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
  "clients": [
    { "clientId": "panacea-app-desktop", "name": "PanaceaApp Desktop (dev)", "public": true },
    { "clientId": "panacea-app-fleet",   "name": "Fleet (dev)",              "public": false, "devSecret": "dev-fleet-loopback-only",
      "audiences": ["panacea-app-hub", "panacea-app-broker"] },
    { "clientId": "panacea-app-gateway", "name": "Gateway (dev)",            "public": false, "devSecret": "dev-gateway-loopback-only",
      "audiences": ["panacea-app-hub"] }
  ],
  "audiences": ["panacea-app-hub", "panacea-app-broker", "panacea-app-gateway", "panacea-app-fleet"],
  "entra": {
    "clientId": "22222222-2222-4222-8222-222222222222",
    "apis": [
      { "clientId": "33333333-3333-4333-8333-333333333333", "name": "PanaceaApp Hub API (dev)",        "scopes": ["Hub.Access"] },
      { "clientId": "55555555-5555-4555-8555-555555555555", "name": "PanaceaApp Sandbox Broker (dev)", "scopes": ["Sandboxes.Access"] }
    ]
  }
}
```

`audiences` are ruling §4's four audience scopes (A1-A4): each is a scope name the desktop may request
and the one `aud` value it adds. The service clients and their allowed audiences are ruling §6's table
(`:312-315`). `entra` is read **only** under `-profile entra` (the earlier revision's client and APIs,
unchanged), so the entra-profile end-to-end row still has GUID audiences and `Hub.Access`; under
`keycloak` it is ignored. Under `entra`, a user's `hub.org_admin` is emitted as `Hub.OrgAdmin`.

Three tenants and five first-tenant users make every column of the authorisation matrix
(contract §5.3) reachable locally, each by a different person: **Ada** is `O` (an org admin in no
team), **Grace** `A`, **Linus** `M`, **Vera** `V`, **Out Sider** `N`; the second tenant's
**Xavier** is `X`, another **enrolled** org; the third tenant is enrolled nowhere, for
`tenant_not_enrolled` (under `keycloak`, refused **after** the signature, ruling §5 `:266-269`).
Fields beyond contract §6.4's schema — `realm`, tenant `name` and `enrolled`, client `name`, `public`,
`devSecret`, `audiences`, `entra`, user `guest` and `emailVerified` (correction A15) — are additive.
`guest` and `emailVerified` shape the `entra` profile's `acct`/`xms_edov` only: a realm token carries
neither, and Panacea's IdP sets `trustEmail:true` (ruling §4 `:188`), so under `keycloak` every user's
`email_verified` is `true`. The committed file sets neither field; a test that needs a guest copies the
loaded `Config` and flips one user (`Options.Config` is a value).

```go
// control-plane/internal/devissuer/config.go
package devissuer

type Config struct {
	Realm     string   `json:"realm"` // the path segment: /realms/<Realm>
	Tenants   []Tenant `json:"tenants"`
	Clients   []Client `json:"clients"`
	Audiences []string `json:"audiences"` // the audience scopes (ruling §4 A1-A4)
	Entra     Entra    `json:"entra"`     // read only under ProfileEntra
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
	Roles    []string `json:"roles"` // realm role names, e.g. "hub.org_admin"
	// The two claims the entra profile's invite binding reads (review I12, A15).
	// Both default to false when omitted. Ignored under ProfileKeycloak.
	Guest         bool `json:"guest"`         // entra acct: 1 when true, else 0
	EmailVerified bool `json:"emailVerified"` // entra xms_edov, a JSON boolean
}

type Client struct {
	ClientID  string   `json:"clientId"` // a Keycloak client id string
	Name      string   `json:"name"`
	Public    bool     `json:"public"`    // true: PKCE, no secret (the desktop)
	DevSecret string   `json:"devSecret"` // confidential only; "dev-" prefix, ≤ 32 chars
	Audiences []string `json:"audiences"` // confidential only: client-credentials audiences
}

type Entra struct { // the earlier revision's shape, for ProfileEntra only
	ClientID string `json:"clientId"` // the desktop's GUID
	APIs     []API  `json:"apis"`
}

type API struct {
	ClientID string   `json:"clientId"`
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
}

// LoadConfig reads the file with unknown fields refused (so no one adds a
// field and has it silently carried), and validates: Realm non-empty and
// [a-z0-9-] only; every tenant id and oid a GUID; oids unique; ≥ 1 tenant;
// exactly one public client, "panacea-app-desktop"; a public client has no
// DevSecret and no Audiences; a confidential client has a DevSecret that
// starts "dev-" and is ≤ 32 characters and ≥ 1 Audience; every Audience
// named anywhere is in Audiences; Audiences contains "panacea-app-hub".
// Entra, when present, is validated as the earlier revision did (GUIDs,
// exactly one API exposing "Hub.Access"); New refuses ProfileEntra without it.
func LoadConfig(path string) (Config, error)
```

```go
// control-plane/internal/devissuer/issuer.go
package devissuer

type Profile string

const (
	ProfileKeycloak Profile = "keycloak" // the default (ruling §5 :296)
	ProfileEntra    Profile = "entra"    // the entra-profile end-to-end row only
)

type Options struct {
	Config  Config
	Profile Profile // "" means ProfileKeycloak
	// BaseURL is how every party reaches the issuer: "http://127.0.0.1:8740".
	// keycloak: the one issuer is BaseURL + "/realms/" + Config.Realm.
	// entra: a tenant's issuer is BaseURL + "/" + tid + "/v2.0" (contract §6.4).
	BaseURL    string
	AccessTTL  time.Duration // default 60 m (PANACEA_APP_DEVISSUER_ACCESS_TTL)
	RefreshTTL time.Duration // default 24 h
	Auto       bool          // PANACEA_APP_DEVISSUER_AUTO (task 12)
	Now        func() time.Time
	Key        *rsa.PrivateKey // nil: a fresh RSA-2048 key at New
}

type Issuer struct{ /* unexported: options, kid, codes, refresh tokens, mutex */ }

func New(o Options) (*Issuer, error)

// Handler serves the profile's paths (the table below). This task: discovery
// and keys; task 12: authorize and token. Anything else is 404.
func (i *Issuer) Handler() http.Handler

// MintAccessToken signs a person's access token in the issuer's profile
// without a browser — for tests (task 16b's matrix, task 34's conformance)
// and nothing else. keycloak: audience is an audience scope ("panacea-app-hub")
// and scopes are extra scope names; entra: audience is an API's GUID and
// scopes its scope names, as before. It refuses a tid, oid, client or
// audience not in the configuration, and a confidential clientID.
func (i *Issuer) MintAccessToken(tid, oid, clientID, audience string, scopes []string) (string, error)

// MintServiceToken signs a client-credentials access token (ruling §6): no
// oid, no tid, azp = clientID, aud = [audience]. keycloak only; it refuses a
// public client and an audience not in that client's Audiences.
func (i *Issuer) MintServiceToken(clientID, audience string) (string, error)

// Env is the variables devstack.sh prints (task 09), one "NAME=value" per line,
// in this order: PANACEA_APP_HUB_TENANTS (every enrolled tenant, "<id>=<name>",
// comma-separated), PANACEA_APP_HUB_AUDIENCE=panacea-app-hub,
// PANACEA_APP_HUB_ALLOWED_CLIENTS=panacea-app-desktop,
// PANACEA_APP_HUB_URL=http://127.0.0.1:8750,
// PANACEA_APP_OIDC_ISSUER=<BaseURL>/realms/<Realm>, PANACEA_APP_OIDC_PROFILE=keycloak,
// PANACEA_APP_OIDC_CLIENT_ID=panacea-app-desktop (ruling §2 :89-99; these
// replace PANACEA_APP_ENTRA_AUTHORITY, _ENTRA_CLIENT_ID and PANACEA_APP_HUB_SCOPE).
// Under ProfileEntra: the same seven names, with HUB_AUDIENCE and
// ALLOWED_CLIENTS the Entra GUIDs, OIDC_ISSUER = BaseURL (the authority
// host) and OIDC_PROFILE=entra.
func (i *Issuer) Env() []string
```

**Paths**, per profile (ruling §2 `:103-106`). `<iss>` is `<base>/realms/<realm>` under `keycloak`:

| Path | `keycloak` (default) | `entra` |
|---|---|---|
| discovery | `/realms/<realm>/.well-known/openid-configuration` | `/<tid>/v2.0/.well-known/openid-configuration` |
| authorize (task 12) | `/realms/<realm>/protocol/openid-connect/auth` | `/<tid>/oauth2/v2.0/authorize` |
| token (task 12) | `/realms/<realm>/protocol/openid-connect/token` | `/<tid>/oauth2/v2.0/token` |
| keys | `/realms/<realm>/protocol/openid-connect/certs` | `/<tid>/discovery/v2.0/keys` |

Any other realm, or under `entra` any unconfigured tenant, is 404.

**Discovery** (`keycloak`):

```json
{ "issuer": "<iss>",
  "authorization_endpoint": "<iss>/protocol/openid-connect/auth",
  "token_endpoint": "<iss>/protocol/openid-connect/token",
  "jwks_uri": "<iss>/protocol/openid-connect/certs",
  "response_types_supported": ["code"],
  "response_modes_supported": ["query"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid", "profile", "email", "offline_access",
                       "panacea-app-hub", "panacea-app-broker", "panacea-app-gateway", "panacea-app-fleet"],
  "token_endpoint_auth_methods_supported": ["none", "client_secret_basic", "client_secret_post"],
  "grant_types_supported": ["authorization_code", "refresh_token", "client_credentials"],
  "code_challenge_methods_supported": ["S256"] }
```

`scopes_supported` lists the four from `Config.Audiences`, in file order. Under `entra`: the earlier
revision's document, unchanged (`issuer` `<base>/<tid>/v2.0`, `subject_types_supported` `["pairwise"]`,
`token_endpoint_auth_methods_supported` `["none"]`, grants `authorization_code` and `refresh_token`,
scopes `openid profile email offline_access`, the entra paths above).

**Keys** — the profile's keys path: `{"keys":[{"kty":"RSA","use":"sig","alg":"RS256",
"kid":"<16 hex>","n":"<base64url modulus>","e":"AQAB"}]}`; `kid` is the first 16 hex characters
of SHA-256 over the modulus's big-endian bytes (contract §6.4).

**Tokens** — JWS compact, header `{"alg":"RS256","typ":"JWT","kid":"<kid>"}`. **`keycloak` claims**
(ruling §4 `:176-191`; a person's token from `MintAccessToken` or task 12's grants, a service token from
`MintServiceToken`):

| Claim | Person access token | Service access token | ID token |
|---|---|---|---|
| `iss` | `<iss>` | same | same |
| `typ` | `"Bearer"` | `"Bearer"` | `"ID"` |
| `aud` | a JSON **array**: the one requested audience scope, then `"account"` (Keycloak's own; the validator's rule is *contains*) | `[<audience>]` | the string `"panacea-app-desktop"` |
| `azp` | the client id (`panacea-app-desktop`) | the service client id | `panacea-app-desktop` |
| `scope` | space-separated: `openid profile email`, then `offline_access` if granted, then the audience scope | the audience scope | — |
| `oid`, `tid` | the user's Entra ids from `dev-tenant.json` (mappers P1, P2) | **absent** | the user's |
| `roles` | a flat array: the user's realm roles (P3; every audience, as P3 sits on the client), omitted when empty | absent | — |
| `email`, `email_verified` | the user's; `email_verified` the JSON boolean `true` (`trustEmail`) | absent | the user's |
| `name`, `preferred_username` | the user's; `preferred_username` = the email | absent; `preferred_username` = `service-account-<client>` | the user's |
| `sub` | the Keycloak user id: SHA-256(`Realm` + "\|" + oid), first 16 bytes as a lowercase UUID string | SHA-256(`Realm` + "\|" + client) likewise | same as the access token |
| `nonce` | — | — | the authorize request's |
| `iat`, `nbf`, `exp` | now, now, now + `AccessTTL` | same | same |
| `jti` | a random UUID string | same | same |

`sub` is stable across audiences but changes with `Realm`, as Keycloak's does when a user is re-created in
another realm (ruling §4 `:166-167`): code that keys on `sub` breaks locally, not first in phase 8. No
`ver`, `scp`, `acct`, `xms_edov` or `groups` is emitted under `keycloak`.

**`entra` claims** (`-profile entra` only), all as Entra v2 names them, the earlier revision unchanged:

| Claim | Access token | ID token |
|---|---|---|
| `ver` | `"2.0"` | `"2.0"` |
| `iss` | `<base>/<tid>/v2.0` | same |
| `aud` | the **API's** client id | the **desktop client's** id |
| `azp` | the client id | — |
| `scp` | the granted scopes, space-separated, bare names (`Hub.Access`) | — |
| `tid`, `oid`, `name`, `preferred_username`, `email` | the user's | the user's |
| `acct` | a JSON number: `1` for a `guest` user, else `0` (Entra's optional claim: 0 member, 1 guest) | same |
| `xms_edov` | a JSON boolean: the user's `emailVerified` (Entra's optional claim, *email domain owner verified*) | same |
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
One flag, `-profile keycloak|entra` (default `keycloak`; any other value refused at start), sets
`Options.Profile` for both `devissuer` and `devissuer env`.

**Left to task 12 by this ruling** (its part is amended separately; ruling §5 `:296-302`): `kc_idp_hint`
accepted and ignored; `login_hint` selects the dev user; `offline_access` refresh tokens with a narrowing
`scope` on refresh; the `client_credentials` grant, which checks a confidential client's `devSecret` and
answers `MintServiceToken`'s token.

#### Tests to write first

Thirteen tests in two packages (plan re-review b05 m4). **Eleven** live in
`control-plane/internal/devissuer/`: every row below except the last two. **Two** —
`TestLoopbackOnlyBind` and `TestEnvLines` — test `main` and live in `control-plane/cmd/devissuer/main_test.go`.
Every row runs under the default `keycloak` profile unless it says `entra`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheCommittedTenantLoads` | `LoadConfig("../../cmd/devissuer/dev-tenant.json")` succeeds; realm `panacea-app-dev`; three tenants, one unenrolled; one public client `panacea-app-desktop`; four audiences | a committed config that the stack cannot start with |
| `TestLoadConfigRefusals` | a non-GUID tid or oid, a duplicate oid, two public clients, a public client with a `devSecret`, a `devSecret` not starting `dev-` or over 32 characters, a service audience not in `audiences`, an unknown field `"secret"` → an error each | a real credential smuggled into a committed file; a public client given a secret |
| `TestDiscoveryDocument` | the keycloak JSON above exactly, at `/realms/panacea-app-dev/...`; another realm → 404; the entra discovery path → 404 | an `issuer` that differs from `PANACEA_APP_OIDC_ISSUER`; a control plane hard-coding Entra paths passing locally |
| `TestKeysAndKid` | at `/realms/<realm>/protocol/openid-connect/certs`: one key; `kid` = the modulus rule; `n` decodes to the key's modulus; `e` = `AQAB` | a kid the validator's refetch logic never matches |
| `TestAMintedAccessTokenValidates` | `oidcauth.New(Config{Profile: ProfileKeycloak, Issuer: iss, TenantIDs: [t1], Audience: "panacea-app-hub", RequiredScope: "panacea-app-hub", AllowedClients: ["panacea-app-desktop"], AllowInsecureAuthority: true})` accepts Ada's token; `Claims` carries tid, oid, name, username, email, `roles` `["hub.org_admin"]`; `Key()` = `t1/<oid>` | an issuer the Hub cannot validate (R9): a wrong `iss` path, a string `aud`, `scp` for `scope`, no `typ` |
| `TestAServiceTokenValidatesAsAService` | `MintServiceToken("panacea-app-fleet", "panacea-app-hub")` validates with `ServiceClients: ["panacea-app-fleet"]`: `Service`, `Key()` = `client/panacea-app-fleet`; its payload has no `oid`, no `tid`; the desktop client, and the gateway for `panacea-app-broker`, → error | a service token carrying a person (ruling §6); a public client minting one |
| `TestOneAudiencePerToken` | Ada's `panacea-app-broker` token: `aud` contains it and not `panacea-app-hub`; the Hub's validator refuses it `ErrAudience` | a devissuer emitting all four audiences, which hides a confused-deputy bug locally (ruling §4 `:215-217`) |
| `TestTheRealmClaims` | decoding Grace's payloads: access `typ` `"Bearer"`, `aud` a JSON array, `email_verified` the JSON boolean `true`, `preferred_username` = the email, and **no** `ver`, `scp`, `acct`, `xms_edov`, `groups`; ID token `typ` `"ID"`, `aud` the string `panacea-app-desktop`, no `roles` | Hub code that reads an Entra-only claim passing locally and failing on the dev realm |
| `TestSubIsTheRealmsUserID` | one oid, two audiences → the same `sub`, a UUID, not the oid; the same `Config` with `Realm` `"other"` → a different `sub`, the same `oid` | code that keys on `sub` passing locally and breaking at phase 8's realm move |
| `TestMintRefusesUnknownParties` | an unknown tid, oid, client or audience, and a confidential client passed to `MintAccessToken` → error | a test minting a principal that does not exist |
| `TestTheEntraProfileIsUnchanged` | under `entra`: the earlier discovery document for tenant 1; Grace `"acct": 0`, `"xms_edov": false` (a JSON number and boolean), Out Sider flipped `guest`/`emailVerified` → `1`, `true`; `Hub.OrgAdmin` only for the Hub API; pairwise `sub`; a minted token validates under `oidcauth` `Profile: ProfileEntra`, `AuthorityHost: base` | the entra-profile end-to-end row losing its issuer |
| `TestLoopbackOnlyBind` | `ADDR=0.0.0.0:8740` and `ADDR=192.168.1.5:8740` → start refused; `127.0.0.1:0` and `[::1]:0` → start; `-profile other` → refused | a token mint reachable from the LAN |
| `TestEnvLines` | the seven keycloak lines, in order, with tenant 3 absent from `PANACEA_APP_HUB_TENANTS` and `PANACEA_APP_OIDC_ISSUER=http://127.0.0.1:8740/realms/panacea-app-dev`; `-profile entra` prints `PANACEA_APP_OIDC_PROFILE=entra` | devstack printing the wrong issuer or enrolling the unenrolled tenant |

#### Steps

- [ ] **Step 1:** Re-read ruling §4-§6 and `oidcauth`'s `keycloak` order (task 10's part).
- [ ] **Step 2:** `dev-tenant.json`; tests; they fail.
- [ ] **Step 3:** `config.go`, `keys.go`, `tokens.go`, `discovery.go`, `issuer.go`, `main.go`.
- [ ] **Step 4:** `go run ./control-plane/cmd/devissuer env` prints the seven lines; commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/devissuer/ ./control-plane/cmd/devissuer/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 13
go run ./control-plane/cmd/devissuer env | sed -n 5p
# PANACEA_APP_OIDC_ISSUER=http://127.0.0.1:8740/realms/panacea-app-dev
git grep -n -i "password" -- control-plane/cmd/devissuer/dev-tenant.json                                                 # no output
grep -o '"devSecret": "[^"]*"' control-plane/cmd/devissuer/dev-tenant.json | grep -vc '"dev-'                         # 0
```

#### What this task does not claim

- That Panacea's dev realm issues exactly these claims, `account` in `aud`, or one audience per refreshed
  token (ruling §12 U-2, U-4); task 43's check against the dev realm compares one real token's claim
  names against the keycloak table.
- That Entra's real tokens match the `entra` table: nothing in v1 runs against direct Entra.
- Anything interactive, the refresh grant or the client-credentials grant: task 12.
