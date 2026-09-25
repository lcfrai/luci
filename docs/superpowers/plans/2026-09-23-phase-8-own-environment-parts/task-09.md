### Task 09: `identity` — our own Keycloak realm, as code (OD-30)

> **Rewritten 2026-09-25 for OD-30** (ruling `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`
> §9). Was: `stacks/identity`, three Entra app registrations on `azuread` (HA-3, IT request Part 1); that
> version is in git history before this commit. **No Entra app registration is made for sign-in.**

Users sign in through a Keycloak realm that brokers Entra with no clicks (OD-30). Until phase 8 that
realm is Panacea's dev realm; phase 8 runs **its own Keycloak, modelled on Panacea's (a pattern, no
source, OD-13)**: realm `panacea-app` at `https://auth.<our domain>/realms/panacea-app`. The move is one
variable, `PANACEA_APP_OIDC_ISSUER`; the person key `(tid, oid)` is Entra's in both realms (mappers M1,
M2, P1, P2), so no Hub row, owner label or LiteLLM user changes. This task writes the realm file and the
Go test that pins it. Keycloak's namespace (task 11), egress (07), database (08) and release and realm
Job (29) are those tasks' rows (ruling §9 table).

**A correction to the ruling's facts.** Ruling §9 says task 09's "Entra admin groups (cluster admins,
Postgres admins) stay". Task 09 never created them: they are owner inputs to tasks 06 and 08
(`var.cluster_admin_group_object_ids`, `var.postgres_admin_group`) and stay there, unchanged. With the
registrations and the *PanaceaApp Users* and *Org Admins* groups deleted (the IdP's `tid` filter admits
the tenant; `hub.org_admin` is a realm role), **`infra/terraform/stacks/identity` has no resource left and
is not written**. P8-D1's `identity` stack and task 30's refusal of it are left for the plan's owner.

**Owner-run, outside this repository** (IT request Part 2, sent when phase 8 provisions):
- IT adds **one redirect URI**, `https://auth.<our domain>/realms/panacea-app/broker/entra/endpoint`, to
  the existing registration (the one Panacea's realm brokers through, client id beginning `37f8bd70-`),
  and **a second client secret** for our Keycloak, so either side rotates without breaking the other.
- The owner stores that secret in our Key Vault as `keycloak-entra-idp-secret` (task 29 mounts it), and
  the Fleet and gateway client secrets as `keycloak-client-panacea-app-fleet` and
  `keycloak-client-panacea-app-gateway` (ruling §6), and Keycloak's bootstrap admin as
  `keycloak-bootstrap-admin` (task 29's realm Job signs in with it). None is Terraform: the registration is IT's, and a
  secret value in state is a usable secret in state (P8-D11).

**Files:**
- `deploy/keycloak/realm/panacea-app.json` (new) — the realm in keycloak-config-cli's format (Apache-2.0)
- `control-plane/internal/realmfile/{doc.go,realm_test.go}` (new) — a test-only package (standard
  library only) that parses the JSON; `doc.go` holds one line, `// Package realmfile pins deploy/keycloak/realm/panacea-app.json.`

#### Contract — `deploy/keycloak/realm/panacea-app.json` (complete)

`$(env:NAME)` is keycloak-config-cli's variable substitution (`IMPORT_VARSUBSTITUTION_ENABLED=true`,
set by task 29's Job; syntax **from training knowledge, UNVERIFIED**). Every environment value comes in
that way; no id, tenant or secret is typed into the file. The Job's variables:
`PANACEA_APP_ENTRA_TENANT_ID`, `PANACEA_APP_ENTRA_IDP_CLIENT_ID` (both from task 29's values),
`KC_ENTRA_IDP_SECRET`, `KC_FLEET_SECRET`, `KC_GATEWAY_SECRET` (all three from Key Vault).

```json
{
  "realm": "panacea-app",
  "enabled": true,
  "displayName": "PanaceaApp",
  "sslRequired": "all",
  "registrationAllowed": false,
  "loginWithEmailAllowed": false,
  "resetPasswordAllowed": false,
  "rememberMe": false,
  "defaultSignatureAlgorithm": "RS256",
  "accessTokenLifespan": 300,
  "ssoSessionIdleTimeout": 86400,
  "ssoSessionMaxLifespan": 86400,
  "offlineSessionIdleTimeout": 2592000,
  "offlineSessionMaxLifespanEnabled": true,
  "offlineSessionMaxLifespan": 5184000,
  "roles": {
    "realm": [
      { "name": "hub.org_admin", "description": "PanaceaApp org administrator (Hub); assigned by the realm admin" }
    ]
  },
  "userProfile": {
    "unmanagedAttributePolicy": "ADMIN_VIEW",
    "attributes": [
      { "name": "username", "permissions": { "view": ["admin", "user"], "edit": ["admin"] } },
      { "name": "email", "permissions": { "view": ["admin", "user"], "edit": ["admin"] } },
      { "name": "firstName", "permissions": { "view": ["admin", "user"], "edit": ["admin"] } },
      { "name": "lastName", "permissions": { "view": ["admin", "user"], "edit": ["admin"] } },
      { "name": "entra_oid", "permissions": { "view": ["admin"], "edit": ["admin"] },
        "validations": { "length": { "max": 36 } } },
      { "name": "entra_tid", "permissions": { "view": ["admin"], "edit": ["admin"] },
        "validations": { "length": { "max": 36 } } }
    ]
  },
  "clientScopes": [
    { "name": "panacea-app-hub", "protocol": "openid-connect",
      "attributes": { "include.in.token.scope": "true", "display.on.consent.screen": "false" },
      "protocolMappers": [ { "name": "audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
        "config": { "included.custom.audience": "panacea-app-hub", "access.token.claim": "true", "id.token.claim": "false" } } ] },
    { "name": "panacea-app-broker", "protocol": "openid-connect",
      "attributes": { "include.in.token.scope": "true", "display.on.consent.screen": "false" },
      "protocolMappers": [ { "name": "audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
        "config": { "included.custom.audience": "panacea-app-broker", "access.token.claim": "true", "id.token.claim": "false" } } ] },
    { "name": "panacea-app-gateway", "protocol": "openid-connect",
      "attributes": { "include.in.token.scope": "true", "display.on.consent.screen": "false" },
      "protocolMappers": [ { "name": "audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
        "config": { "included.custom.audience": "panacea-app-gateway", "access.token.claim": "true", "id.token.claim": "false" } } ] },
    { "name": "panacea-app-fleet", "protocol": "openid-connect",
      "attributes": { "include.in.token.scope": "true", "display.on.consent.screen": "false" },
      "protocolMappers": [ { "name": "audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
        "config": { "included.custom.audience": "panacea-app-fleet", "access.token.claim": "true", "id.token.claim": "false" } } ] }
  ],
  "clients": [
    { "clientId": "panacea-app-desktop", "name": "PanaceaApp Desktop", "enabled": true, "protocol": "openid-connect",
      "publicClient": true, "standardFlowEnabled": true, "implicitFlowEnabled": false,
      "directAccessGrantsEnabled": false, "serviceAccountsEnabled": false, "consentRequired": false,
      "redirectUris": ["http://127.0.0.1/callback"], "webOrigins": [],
      "attributes": { "pkce.code.challenge.method": "S256", "oauth2.device.authorization.grant.enabled": "false",
        "client.offline.session.max.lifespan": "604800", "access.token.lifespan": "600" },
      "authenticationFlowBindingOverrides": { "browser": "panacea-app desktop browser" },
      "defaultClientScopes": ["basic", "profile", "email", "roles", "web-origins", "acr"],
      "optionalClientScopes": ["offline_access", "panacea-app-hub", "panacea-app-broker", "panacea-app-gateway", "panacea-app-fleet"],
      "protocolMappers": [
        { "name": "oid", "protocol": "openid-connect", "protocolMapper": "oidc-usermodel-attribute-mapper",
          "config": { "user.attribute": "entra_oid", "claim.name": "oid", "jsonType.label": "String",
            "access.token.claim": "true", "id.token.claim": "true", "userinfo.token.claim": "false" } },
        { "name": "tid", "protocol": "openid-connect", "protocolMapper": "oidc-usermodel-attribute-mapper",
          "config": { "user.attribute": "entra_tid", "claim.name": "tid", "jsonType.label": "String",
            "access.token.claim": "true", "id.token.claim": "true", "userinfo.token.claim": "false" } },
        { "name": "roles", "protocol": "openid-connect", "protocolMapper": "oidc-usermodel-realm-role-mapper",
          "config": { "claim.name": "roles", "multivalued": "true", "jsonType.label": "String",
            "access.token.claim": "true", "id.token.claim": "false", "userinfo.token.claim": "false" } }
      ] },
    { "clientId": "panacea-app-fleet", "name": "PanaceaApp Fleet", "enabled": true, "protocol": "openid-connect",
      "publicClient": false, "clientAuthenticatorType": "client-secret", "secret": "$(env:KC_FLEET_SECRET)",
      "serviceAccountsEnabled": true, "standardFlowEnabled": false, "implicitFlowEnabled": false,
      "directAccessGrantsEnabled": false, "redirectUris": [], "webOrigins": [],
      "defaultClientScopes": [], "optionalClientScopes": ["panacea-app-hub", "panacea-app-broker"] },
    { "clientId": "panacea-app-gateway", "name": "PanaceaApp Gateway", "enabled": true, "protocol": "openid-connect",
      "publicClient": false, "clientAuthenticatorType": "client-secret", "secret": "$(env:KC_GATEWAY_SECRET)",
      "serviceAccountsEnabled": true, "standardFlowEnabled": false, "implicitFlowEnabled": false,
      "directAccessGrantsEnabled": false, "redirectUris": [], "webOrigins": [],
      "defaultClientScopes": [], "optionalClientScopes": ["panacea-app-hub"] }
  ],
  "identityProviders": [
    { "alias": "entra", "displayName": "Microsoft Entra ID", "providerId": "oidc", "enabled": true,
      "trustEmail": true, "storeToken": false, "addReadTokenRoleOnCreate": false, "linkOnly": false,
      "firstBrokerLoginFlowAlias": "panacea-app first broker login",
      "config": {
        "clientId": "$(env:PANACEA_APP_ENTRA_IDP_CLIENT_ID)",
        "clientSecret": "$(env:KC_ENTRA_IDP_SECRET)",
        "clientAuthMethod": "client_secret_post",
        "authorizationUrl": "https://login.microsoftonline.com/$(env:PANACEA_APP_ENTRA_TENANT_ID)/oauth2/v2.0/authorize",
        "tokenUrl": "https://login.microsoftonline.com/$(env:PANACEA_APP_ENTRA_TENANT_ID)/oauth2/v2.0/token",
        "issuer": "https://login.microsoftonline.com/$(env:PANACEA_APP_ENTRA_TENANT_ID)/v2.0",
        "jwksUrl": "https://login.microsoftonline.com/$(env:PANACEA_APP_ENTRA_TENANT_ID)/discovery/v2.0/keys",
        "useJwksUrl": "true", "validateSignature": "true",
        "defaultScope": "openid profile email", "syncMode": "IMPORT",
        "loginHint": "true", "pkceEnabled": "false",
        "filteredByClaim": "true", "claimFilterName": "tid",
        "claimFilterValue": "$(env:PANACEA_APP_ENTRA_TENANT_ID)"
      } }
  ],
  "identityProviderMappers": [
    { "name": "email", "identityProviderAlias": "entra", "identityProviderMapper": "oidc-user-attribute-idp-mapper",
      "config": { "claim": "email", "user.attribute": "email", "syncMode": "FORCE" } },
    { "name": "firstName", "identityProviderAlias": "entra", "identityProviderMapper": "oidc-user-attribute-idp-mapper",
      "config": { "claim": "given_name", "user.attribute": "firstName", "syncMode": "INHERIT" } },
    { "name": "lastName", "identityProviderAlias": "entra", "identityProviderMapper": "oidc-user-attribute-idp-mapper",
      "config": { "claim": "family_name", "user.attribute": "lastName", "syncMode": "INHERIT" } },
    { "name": "username", "identityProviderAlias": "entra", "identityProviderMapper": "oidc-username-idp-mapper",
      "config": { "template": "${CLAIM.email}", "syncMode": "INHERIT" } },
    { "name": "entra_oid", "identityProviderAlias": "entra", "identityProviderMapper": "oidc-user-attribute-idp-mapper",
      "config": { "claim": "oid", "user.attribute": "entra_oid", "syncMode": "FORCE" } },
    { "name": "entra_tid", "identityProviderAlias": "entra", "identityProviderMapper": "oidc-user-attribute-idp-mapper",
      "config": { "claim": "tid", "user.attribute": "entra_tid", "syncMode": "FORCE" } }
  ],
  "authenticatorConfig": [
    { "alias": "panacea-app entra redirector", "config": { "defaultProvider": "entra" } }
  ],
  "authenticationFlows": [
    { "alias": "panacea-app desktop browser", "providerId": "basic-flow", "topLevel": true, "builtIn": false,
      "authenticationExecutions": [
        { "authenticator": "identity-provider-redirector", "authenticatorConfig": "panacea-app entra redirector",
          "requirement": "REQUIRED", "priority": 10, "authenticatorFlow": false }
      ] },
    { "alias": "panacea-app first broker login", "providerId": "basic-flow", "topLevel": true, "builtIn": false,
      "authenticationExecutions": [
        { "authenticator": "idp-create-user-if-unique", "requirement": "ALTERNATIVE", "priority": 10, "authenticatorFlow": false },
        { "flowAlias": "panacea-app handle existing account", "requirement": "ALTERNATIVE", "priority": 20, "authenticatorFlow": true }
      ] },
    { "alias": "panacea-app handle existing account", "providerId": "basic-flow", "topLevel": false, "builtIn": false,
      "authenticationExecutions": [
        { "authenticator": "idp-auto-link", "requirement": "REQUIRED", "priority": 10, "authenticatorFlow": false }
      ] }
  ]
}
```

Why the settings that are not obvious (ruling §4, §6, §9; Panacea's pattern from
`.superpowers/sdd/identity/facts-panacea-keycloak.md` §3, read-only survey):
- **The IdP's `prompt` is absent** and there is **no review-profile step**: either would put a click in
  front of the person (ruling §3). `loginHint` forwards the desktop's `login_hint`.
- **`FORCE` on M1, M2**: `oid` and `tid` never change, so rewriting them at every login is a no-op for
  a stable person and repairs a user created before the mapper existed.
- **The desktop's browser flow is one Identity Provider Redirector to `entra`** (ruling K1 `:500-505`,
  review-1 I1): no Cookie step and no username/password form, so every interactive sign-in goes through
  Entra and its conditional access, M1/M2 run each time. Refresh and offline grants run no browser flow,
  so silent launches are unaffected.
- **`entra` is the realm's only IdP** (ruling K1's invariant, review-2 I1): the redirector reads a
  client-supplied `kc_idp_hint` before `defaultProvider`, so the hint can only name `entra`, and an empty
  or unknown hint fails closed (UNVERIFIED; task 31 runs ruling §10 check 2). A second IdP, such as a
  customer BYO-Entra one, is an OD-30 ruling change first, never a realm-file edit: a hint could pick
  it, and a user auto-linked through it by email would carry the person's FORCE-written `entra_oid`.
- **`email` is admin-edit and its IdP mapper `FORCE`** (ruling K3, K4, review-1 I3): the Hub binds
  invites by `email` when `email_verified` is true, so a user must not be able to rewrite it.
- **`unmanagedAttributePolicy` is `ADMIN_VIEW`, never `ENABLED`** (ruling K4, review-1 M2): an
  undeclared attribute is never user-editable, so a lapsed declaration cannot make `entra_oid` forgeable.
- **`access.token.lifespan` 600** (ruling K1 `:495`, §6, review-1 M4): bounds how long a revoked session
  stays valid at the offline validators. **`redirectUris` is `/callback` only**, the one path the
  listener serves (ruling K1 `:490`, review-1 M3).
- **`${CLAIM.email}`** is Keycloak's own template, not a `$(env:…)` substitution: the two syntaxes differ
  on purpose.
- **Keycloak's built-in scopes** (`basic`, `profile`, `email`, `roles`, `web-origins`, `acr`,
  `offline_access`) are created with the realm and not declared here. Task 29's Job therefore runs with
  `IMPORT_MANAGED_CLIENT_SCOPE=no-delete`, so a full-managed import cannot remove them (the default's
  behaviour for built-ins is **UNVERIFIED**).
- **The gateway client gets only `panacea-app-hub`**, per ruling §6's table (the Hub is its only
  acceptor), as ruling §6 now says: Fleet A1-A2, the gateway A1 only.

#### Tests to write first (`control-plane/internal/realmfile/realm_test.go`)

The test reads `../../../deploy/keycloak/realm/panacea-app.json` with `encoding/json` into
`map[string]any` and finds clients, scopes, mappers and flows by name, never by position.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheRealmFileParses` | valid JSON; `realm == "panacea-app"`, `sslRequired == "all"`, registration, password reset and email login off | a file the Job refuses at deploy; a self-registration page |
| `TestDesktopIsAPublicPKCEClientOnLoopback` | public, no `secret`; `S256`; `redirectUris == ["http://127.0.0.1/callback"]` exactly; implicit, direct grants, service accounts and device grant off; no consent; `webOrigins` empty; `access.token.lifespan` `"600"` | a secret on a laptop; a `localhost` redirect (facts §8); a `/*` path (review-1 M3); a password grant; 30 minutes of revocation latency (M4) |
| `TestEveryBrowserSignInGoesToEntra` | the desktop's `authenticationFlowBindingOverrides.browser` names `panacea-app desktop browser`; that flow is top-level with exactly one execution, `identity-provider-redirector`, `REQUIRED`, whose config has `defaultProvider` `entra`; no `auth-cookie`, `auth-username-password-form` or OTP execution in it; `identityProviders` has **exactly one** entry, alias `entra`, `enabled` true | a realm password form or SSO cookie that mints a token without Entra (review-1 I1); a second IdP that a client's `kc_idp_hint` could pick (review-2 I1) |
| `TestDesktopScopesAreExactlyTheRulings` | optional scopes `offline_access` + the four audience scopes, and nothing else; no `groups` anywhere | groups in every token; no `offline_access`, so the silent refresh dies overnight (ruling §2) |
| `TestEachAudienceScopeNamesItself` | four scopes, each with one `oidc-audience-mapper` whose `included.custom.audience` is the scope's own name, access token only, `include.in.token.scope` true | a broker token that the Hub also accepts (the confused deputy) |
| `TestPersonClaimsCarryTheEntraIds` | desktop mappers: `entra_oid` → `oid`, `entra_tid` → `tid` (access + id token), realm roles → `roles` (multivalued, access only) | a person keyed on Keycloak's `sub` |
| `TestIdPMappersForceTheEntraIds` | `oid` → `entra_oid`, `tid` → `entra_tid`, and `email` → `email`, each `syncMode` `FORCE` | a user who never gets `entra_oid` (`IMPORT` writes only at creation); a stale or self-edited `email` surviving a login (review-1 I3) |
| `TestOnlyAnAdminCanEditTheEntraIds` | user profile: `entra_oid`, `entra_tid` view and edit `["admin"]`; `email` edit `["admin"]`; `unmanagedAttributePolicy` present and not `ENABLED` | a person rewriting their own `oid` or `email` in the account console, becoming someone else or binding a colleague's invite (review-1 I3, M2) |
| `TestTheIdPIsPinnedToOneTenant` | alias `entra`; all four URLs tenant-specific through `$(env:PANACEA_APP_ENTRA_TENANT_ID)`; `client_secret_post`; `tid` claim filter on; `trustEmail` true; `storeToken` false; `loginHint` true; no `prompt` key; its first-login flow exists | any Entra tenant signing in; an Entra token stored in the realm; a click |
| `TestFirstLoginHasNoReviewStep` | the flow holds exactly `idp-create-user-if-unique` and the sub-flow with `idp-auto-link`; no `idp-review-profile` anywhere | a profile-review page at first sign-in |
| `TestServiceClientsCarryNoPerson` | Fleet and gateway: confidential, service accounts on, every browser flow off, no redirects, no protocol mappers; Fleet's optional scopes hub + broker, the gateway's hub only | a service token carrying `oid`; a gateway token the broker could accept |
| `TestNothingSecretIsTyped` | every `secret` and `clientSecret` matches `^\$\(env:[A-Z_]+\)$`; no GUID anywhere in the file | a secret or a tenant id committed to the repository |
| `TestLeaversLapseWithinAWeek` | realm `offlineSessionMaxLifespanEnabled` true, `offlineSessionMaxLifespan` 5184000; desktop `client.offline.session.max.lifespan` 604800; `ssoSessionMaxLifespan` 86400 | a person disabled in Entra keeping cloud access for 60 days (ruling §6) |
| `TestOrgAdminIsARealmRole` | realm role `hub.org_admin` exists | no org admin able to erase (ADR-H1 G6) |

#### Steps

- [ ] **Step 1:** Re-read ruling §4, §6 and §9; if they changed, they win over this part.
- [ ] **Step 2:** `doc.go` and the tests; `go test` fails (no realm file).
- [ ] **Step 3:** The realm file until the fourteen tests pass.
- [ ] **Step 4:** Commit.

#### Acceptance

```bash
go test ./control-plane/internal/realmfile/ -count=1 -v 2>&1 | grep -c '^--- PASS'   # 14
go vet ./control-plane/internal/realmfile/
```

#### What this task does not claim

- That keycloak-config-cli reads this file as written: the `$(env:…)` substitution, the `userProfile`
  key and the `authenticatorFlow` spelling are **from training knowledge, UNVERIFIED** until task 29's
  Job applies it to a Keycloak 26 in a cluster (task 31). The test pins the settings, not the format.
- That `http://127.0.0.1/callback` matches any loopback port in Keycloak (the same assumption as ruling
  §10's K1, U-3, UNVERIFIED), that keycloak-config-cli resolves `authenticationFlowBindingOverrides` by
  flow **alias** (a realm export holds the flow's id; UNVERIFIED), and that `unmanagedAttributePolicy`
  sits in `userProfile` as written (UNVERIFIED): task 31's run against the cluster settles all three.
- That the account console refuses a user's own edit of `entra_oid` or `email` at runtime: ruling
  §10's check 3, run against our realm when task 31 applies this file. This test pins the permissions only.
- That a **first-time user** is still created with no click, and gets `email`, `email_verified`,
  `entra_oid` and `entra_tid`, while `username`, `email`, `firstName` and `lastName` are admin-edit only
  (ruling U-16, review-2 I2: `idp-create-user-if-unique` writes through the user profile in a user
  context; UNVERIFIED). Task 31 runs ruling §10 check 4 against our realm: a user absent from it signs
  in and the token is read. If it fails, the owner picks the fallback; nothing is loosened here first.
- That an empty or unknown `kc_idp_hint` fails closed at runtime: ruling §10 check 2, run by task 31.
  The test pins one IdP only.
- That IT has added the redirect URI and the second secret, or that the owner has stored the secrets:
  both are owner-run (above) and task 31 checks them.
